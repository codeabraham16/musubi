package recalleval

import (
	"context"
	"fmt"
	"os"
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

	// EL COSTO, DICHO ANTES DE GASTARLO: una llamada al juez por consulta con ≥2 candidatos. Si este
	// número sorprende, hay que parar acá y no después de haber gastado.
	t.Logf("fixture: %d docs · %d consultas ⇒ hasta %d llamadas al juez (%s vía %s)",
		len(fx.Docs), len(fx.Queries), len(fx.Queries), modelo, endpoint)

	juez := cognition.NewOpenAICompatProvider(endpoint, modelo, clave, 120*time.Second)
	conJuez := Config{
		Name: "lexical+juez",
		Opts: lexicalConfig.Opts, // MISMAS señales model-free: lo único que cambia es el juez
		Juez: juez,
	}

	ks := []int{1, 5, 10}
	arranque := time.Now()
	scores, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{lexicalConfig, conJuez}, ks)
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
