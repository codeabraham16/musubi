package recalleval

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"musubi/internal/cognition"
)

// MEDICIÓN DE CIERRE DE F2: los dos brazos —model-free y model-free+juez— sobre el MISMO corpus y
// contra el MOTOR DE VERDAD. Es lo único que responde la pregunta que abrió el track: ¿el juez de
// pertinencia aporta, y cuánto?
//
// No es un gate y nunca corre en CI: se saltea sin las env vars, porque gastar cuota de una
// suscripción en cada push sería una forma cara de no aprender nada.
//
// POR QUÉ EL PROVIDER SE ARMA CRUDO (NewOpenAICompatProvider) Y NO CON cognition.NewProvider:
//   - el CACHÉ mediría el caché. Dos brazos sobre las mismas queries con caché encendido devuelven
//     la segunda corrida gratis y hacen parecer estable algo que no se volvió a preguntar;
//   - el PORTERO tapa secretos en el prompt, que es una propiedad de seguridad y no de ranking.
//
// Lo que sí es el de producción es el JUEZ: el harness llama a cognition.Rerank, la misma unidad que
// usa el servidor, con el mismo top-K por default.
func TestMedicionJuezReal(t *testing.T) {
	rutaDB := os.Getenv("MUSUBI_FIXTURE_DB")
	endpoint := os.Getenv("MUSUBI_JUEZ_ENDPOINT")
	if rutaDB == "" || endpoint == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB y/o MUSUBI_JUEZ_ENDPOINT: se saltea la medición con motor real")
	}
	// SIN EMBEDDER NO HAY MEDICIÓN. La base tiene que ser el recall híbrido, y el híbrido sin
	// vectores es el léxico con otro nombre. Antes que dar un número contra la base equivocada, este
	// test se saltea y dice por qué.
	embed, urlEmbed, modeloEmbed, _, hayEmbedder := embedderOllamaDesdeEnv()
	if !hayEmbedder {
		t.Skip("falta MUSUBI_OLLAMA_URL: sin vectores reales la base sería léxica, y el juez quedaría medido contra una configuración que no corre en producción")
	}
	modelo := os.Getenv("MUSUBI_JUEZ_MODEL")
	if modelo == "" {
		modelo = "sonnet"
	}
	// La clave se lee de la env var NOMBRADA en auth_token_env de la config del servidor; nunca se
	// escribe en el yaml ni en el repo. Vacía es válido para backends locales sin auth.
	clave := os.Getenv("LITELLM_MASTER_KEY")

	fx, err := FixtureDesdeDB(rutaDB, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB(%s): %v", rutaDB, err)
	}

	// EL COSTO, DICHO ANTES DE GASTARLO: una llamada al juez por consulta con ≥2 candidatos, más los
	// embeddings. Run SIEMBRA UNA SOLA VEZ y evalúa los dos brazos sobre el mismo engine, así que el
	// corpus se embebe una vez y sólo las consultas se repiten por brazo: docs + 2×consultas. Si
	// estos números sorprenden, hay que parar acá y no después de haber gastado.
	t.Logf("fixture: %d docs · %d consultas ⇒ hasta %d llamadas al juez (%s vía %s) y ~%d embeddings (%s vía %s)",
		len(fx.Docs), len(fx.Queries), len(fx.Queries), modelo, endpoint,
		len(fx.Docs)+2*len(fx.Queries), modeloEmbed, urlEmbed)

	juez := cognition.NewOpenAICompatProvider(endpoint, modelo, clave, 120*time.Second)
	base, conJuez := configsDelJuez(juez)

	ks := []int{1, 5, 10}
	arranque := time.Now()
	scores, err := Run(context.Background(), t.TempDir(), fx, embed, []Config{base, conJuez}, ks)
	if err != nil {
		// El harness ABORTA ante un juez roto en vez de degradar como producción: un juez que falla
		// y devuelve el orden model-free daría «el juez no aporta nada», que es una conclusión falsa
		// con cara de medición.
		t.Fatalf("Run: %v", err)
	}
	t.Logf("\n%s", FormatReport(scores, ks))
	t.Logf("%s", formatearDelta(scores[0], scores[1], ks))
	t.Logf("tardó %s en total (los dos brazos)", time.Since(arranque).Round(time.Second))
	t.Log("OJO: los ABSOLUTOS están subestimados por el etiquetado por topic_key. Lo que decide es el DELTA.")
	t.Logf("BASE de este delta: %q (vector ENCENDIDO) — que es lo que corre en el cerebro central.", base.Name)
}

// configsDelJuez arma los DOS brazos de la medición: la base, y la base con el juez encima.
//
// LA BASE ES `hybrid`, NO `lexical`, Y ESE ES EL PUNTO DE ESTA FUNCIÓN. El cerebro central corre
// búsqueda híbrida —vector + señales model-free— desde el 2026-07-28. Medir el juez contra el brazo
// léxico le regala en el delta todo lo que ya aportaba el vector: la aritmética sale impecable, el
// número sale grande, y responde una pregunta que nadie hizo. Es medir bien en el sitio equivocado,
// y el resultado sale verde igual.
//
// Los dos brazos comparten Opts y UseVector: lo ÚNICO que los separa es el juez. Si divergieran en
// algo más, el delta dejaría de ser atribuible al juez y la medición mentiría sin avisar.
func configsDelJuez(juez cognition.Provider) (base, conJuez Config) {
	base = hybridConfig
	conJuez = Config{
		Name:      base.Name + "+juez",
		Opts:      base.Opts,
		UseVector: base.UseVector,
		Juez:      juez,
	}
	return base, conJuez
}

// ★ LOS DOS BRAZOS SÓLO PUEDEN DIFERIR EN EL JUEZ, Y LA BASE TIENE QUE SER LA DE PRODUCCIÓN.
//
// Corre en CI (no necesita red ni cuota) justamente porque el defecto que previene es invisible: un
// brazo base equivocado no rompe nada, no falla, no avisa — sólo devuelve un delta inflado que
// después alguien publica como si midiera al juez. Que este invariante viva en un test barato es lo
// que evita que vuelva a pasar en silencio.
func TestElJuezSeMideSobreLaBaseDeProduccion(t *testing.T) {
	juez := &juezInvertido{}
	base, conJuez := configsDelJuez(juez)

	if !base.UseVector {
		t.Error("la base del delta tiene que ser el recall HÍBRIDO: el central corre con vector, y medir contra el léxico le acredita al juez lo que ya aportaba el embedding")
	}
	if conJuez.UseVector != base.UseVector {
		t.Errorf("los brazos difieren en UseVector (base=%v, conJuez=%v): el delta ya no es atribuible al juez",
			base.UseVector, conJuez.UseVector)
	}
	// DeepEqual y no `!=`: RecallOptions lleva QueryVector ([]float32) y no es comparable. Además
	// conviene que sea profundo — si un brazo llegara con vector precargado y el otro no, el delta
	// tampoco sería del juez.
	if !reflect.DeepEqual(conJuez.Opts, base.Opts) {
		t.Errorf("los brazos difieren en las señales model-free:\n  base    %+v\n  conJuez %+v", base.Opts, conJuez.Opts)
	}
	if base.Juez != nil {
		t.Error("la base no puede llevar juez: sería medir al juez contra sí mismo")
	}
	if conJuez.Juez == nil {
		t.Error("el brazo con juez quedó sin juez: mediría dos veces la base y daría delta cero")
	}
	if conJuez.Name == base.Name {
		t.Errorf("los dos brazos se llaman igual (%q): FormatReport los mezcla y el delta se lee al revés", base.Name)
	}
}

// formatearDelta es el número que responde la pregunta de F2: cuánto se movió cada métrica al meter
// el juez. Se imprime aparte de la tabla porque restar dos filas a ojo es exactamente donde alguien
// se equivoca de signo y saca la conclusión al revés.
func formatearDelta(base, conJuez Scores, ks []int) string {
	s := fmt.Sprintf("DELTA (%s − %s)\n  MRR %+0.3f", conJuez.Config, base.Config, conJuez.MRR-base.MRR)
	for _, k := range ks {
		s += fmt.Sprintf("   R@%d %+0.3f   nDCG@%d %+0.3f",
			k, conJuez.RecallAtK[k]-base.RecallAtK[k], k, conJuez.NDCGAtK[k]-base.NDCGAtK[k])
	}
	return s
}
