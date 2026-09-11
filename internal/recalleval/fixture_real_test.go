package recalleval

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// Invariantes del generador de fixtures desde memoria real (specs/fixture-real/).

// baseDePrueba arma una base con topics de tamaños conocidos y devuelve su ruta.
func baseDePrueba(t *testing.T, porTopico map[string]int) string {
	t.Helper()
	dir := t.TempDir()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	for topic, n := range porTopico {
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("%s#%d", topic, i)
			contenido := fmt.Sprintf("nota %d del tema %s con texto suficiente para que el gist tenga algo que decir", i, topic)
			if err := eng.SaveObservation(id, topic, contenido, nil); err != nil {
				t.Fatalf("SaveObservation(%s): %v", id, err)
			}
		}
	}
	if err := eng.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// NewDbEngine crea la base en <dir>/.musubi/memory.db, no en la raíz del dir.
	return filepath.Join(dir, config.DirName, config.DBFile)
}

func consultaPorTopico(fx *Fixture, topic string) *Query {
	for i := range fx.Queries {
		if fx.Queries[i].ID == "topic:"+topic {
			return &fx.Queries[i]
		}
	}
	return nil
}

// K1 — LAS ETIQUETAS SALEN DEL topic_key, NO DEL RANKER.
//
// Es lo único que hace creíble a un fixture automático. Si las etiquetas se derivaran del propio
// ranking —o de un LLM—, el banco mediría si el ranker coincide consigo mismo.
func TestK1LasEtiquetasSalenDelTopic(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/alfa": 4, "tema/beta": 3})
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}

	for topic, n := range map[string]int{"tema/alfa": 4, "tema/beta": 3} {
		q := consultaPorTopico(fx, topic)
		if q == nil {
			t.Fatalf("falta la consulta del topic %q", topic)
		}
		if len(q.Relevant) != n {
			t.Errorf("%s: esperaba %d relevantes, hay %d", topic, n, len(q.Relevant))
		}
		for _, id := range q.Relevant {
			if !strings.HasPrefix(id, topic+"#") {
				t.Errorf("%s: el relevante %q no pertenece al topic", topic, id)
			}
		}
	}
}

// K2 — LOS CAJONES DE SASTRE NO SON CONSULTAS.
//
// Medido en la memoria real de este proyecto: `git-commit` tiene 247 observaciones. No es un tema
// sobre el que alguien pregunte, y una consulta con 247 relevantes sobre 1.210 docs no mide nada.
// Se excluye por PREFIJO (es mecánico) y además por TAMAÑO (por si aparece otro cajón sin nombre).
func TestK2LosCajonesDeSastreNoSonConsultas(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{
		"git-commit":  5,  // excluido por prefijo
		"tema/enorme": 60, // excluido por tamaño (MaxPorTopico default 50)
		"tema/sano":   3,
	})
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	for _, excluido := range []string{"git-commit", "tema/enorme"} {
		if q := consultaPorTopico(fx, excluido); q != nil {
			t.Errorf("%q no debería ser consulta (tiene %d relevantes)", excluido, len(q.Relevant))
		}
	}
	if consultaPorTopico(fx, "tema/sano") == nil {
		t.Error("tema/sano debería ser consulta")
	}
}

// K3 — Un topic con menos de MinPorTopico no es consulta: con 1 relevante las métricas de ORDEN
// casi no informan, y el fixture se llenaría de ruido (834 topics distintos en la memoria real,
// la enorme mayoría con una sola nota).
func TestK3LosTopicsChicosNoSonConsultas(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/solo": 1, "tema/par": 2, "tema/tres": 3})
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{}) // MinPorTopico default = 3
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	if consultaPorTopico(fx, "tema/solo") != nil || consultaPorTopico(fx, "tema/par") != nil {
		t.Error("los topics con menos de 3 observaciones no deberían ser consultas")
	}
	if consultaPorTopico(fx, "tema/tres") == nil {
		t.Error("tema/tres debería ser consulta")
	}
}

// K4 — EL CORPUS ES TODO, aunque el topic no sea consulta.
//
// Las observaciones de los topics excluidos siguen siendo DOCS: son distractores legítimos y sacarlas
// volvería el banco artificialmente fácil — cada consulta competiría contra un corpus recortado a
// medida, que es una forma silenciosa de inflar el resultado.
func TestK4LoExcluidoSigueSiendoCorpus(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"git-commit": 5, "tema/sano": 3})
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	if len(fx.Docs) != 8 {
		t.Fatalf("esperaba los 8 docs en el corpus (5 excluidos como consulta + 3), hay %d", len(fx.Docs))
	}
	var deGitCommit int
	for _, d := range fx.Docs {
		if d.Topic == "git-commit" {
			deGitCommit++
		}
	}
	if deGitCommit != 5 {
		t.Errorf("los docs de un topic excluido deben seguir en el corpus como distractores, hay %d de 5", deGitCommit)
	}
}

// K5 — DETERMINISTA. Dos generaciones de la misma base dan el mismo fixture byte a byte. Un fixture
// que cambia solo convierte cualquier comparación entre corridas en ruido.
func TestK5ElFixtureEsDeterminista(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/a": 3, "tema/b": 4, "tema/c": 5})
	var huellas []string
	for i := 0; i < 2; i++ {
		fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
		if err != nil {
			t.Fatalf("generación %d: %v", i, err)
		}
		b, _ := json.Marshal(fx)
		huellas = append(huellas, fmt.Sprintf("%x", sha256.Sum256(b)))
	}
	if huellas[0] != huellas[1] {
		t.Fatalf("el fixture no es determinista: %s != %s", huellas[0][:16], huellas[1][:16])
	}
}

// K6 — GENERAR NO TOCA LA BASE.
//
// Esto lee la memoria de trabajo de alguien. La apertura es `mode=ro`, y acá se verifica el efecto y
// no la intención: el archivo tiene que quedar byte-idéntico.
func TestK6GenerarNoModificaLaBase(t *testing.T) {
	ruta := baseDePrueba(t, map[string]int{"tema/a": 3})
	antes, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer la base: %v", err)
	}
	if _, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{}); err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	despues, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("releer la base: %v", err)
	}
	if sha256.Sum256(antes) != sha256.Sum256(despues) {
		t.Fatal("generar el fixture MODIFICÓ la base de memoria: tiene que abrirse en sólo lectura")
	}
}

// K7 — La consulta se deriva del topic con una transformación tonta, no del contenido. Cualquier
// cosa más inteligente metería en la consulta información derivada de lo que se está midiendo.
func TestK7LaConsultaSaleDelTopicYNoDelContenido(t *testing.T) {
	casos := map[string]string{
		"roadmap/track-potencia-medida":      "roadmap track potencia medida",
		"cognicion/donde_esta_el_motor":      "cognicion donde esta el motor",
		"server/deploy-cerebro-central":      "server deploy cerebro central",
		"//raro--con__separadores//seguidos": "raro con separadores seguidos",
	}
	for topic, quiero := range casos {
		if got := ConsultaDesdeTopico(topic); got != quiero {
			t.Errorf("ConsultaDesdeTopico(%q) = %q, quería %q", topic, got, quiero)
		}
	}
}

// MEDICIÓN (no es un gate): genera el fixture desde una memoria REAL y corre el banco model-free
// sobre él. Se saltea sin MUSUBI_FIXTURE_DB, porque CI no tiene —ni debe tener— memoria real.
//
// Sirve para ver a qué escala se está midiendo y con qué números arranca el baseline, antes de
// gastar cuota comparando arms con el motor de verdad.
func TestMedicionFixtureReal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	if ruta == "" {
		t.Skip("MUSUBI_FIXTURE_DB no seteado: se saltea la medición sobre memoria real")
	}
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB(%s): %v", ruta, err)
	}
	relevantes := 0
	for _, q := range fx.Queries {
		relevantes += len(q.Relevant)
	}
	t.Logf("fixture real: %d docs · %d consultas · %.1f relevantes por consulta",
		len(fx.Docs), len(fx.Queries), float64(relevantes)/float64(len(fx.Queries)))

	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, nil, []Config{lexicalConfig}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("\n%s", FormatReport(scores, ks))
	t.Log("OJO: los ABSOLUTOS están subestimados — el etiquetado por topic_key cuenta como fallo " +
		"todo lo relevante que viva en otro topic. Lo comparable es el DELTA entre arms.")
}

// TestABLexicoVsHibridoFixtureReal produce EL NÚMERO QUE NO EXISTÍA: el A/B léxico-contra-híbrido
// sobre el corpus REAL, con la tabla POTION real.
//
// Por qué hacía falta otro test y no alcanzaba con los dos que ya había:
//   - TestSemanticVsLexicalReal corre los dos brazos, pero sobre el golden VERSIONADO: 26 docs de
//     84-108 caracteres. Es un gate de no-regresión, no una medida de lo que pasa en un acervo de
//     miles de observaciones en rioplatense.
//   - TestMedicionFixtureReal corre sobre el corpus real, pero UN SOLO BRAZO (lexicalConfig). O sea
//     que medía el piso sin nada con qué compararlo.
//
// El cruce de los dos —corpus real Y los dos brazos— no lo hacía nadie, y es justo el que decide si
// encender la capa vectorial vale la pena.
//
// LO COMPARABLE ES EL DELTA, NO EL ABSOLUTO, por el mismo motivo que ya declara
// TestMedicionFixtureReal: el etiquetado sale del topic_key, así que todo lo relevante que viva en
// otro topic cuenta como fallo en LOS DOS brazos por igual. El sesgo se cancela en la resta.
//
// No tiene gate a propósito: es un INSTRUMENTO DE MEDICIÓN sobre la memoria de trabajo de alguien,
// que cambia entre corridas. Poner un piso acá sería un gate que falla por lo que el usuario
// guardó ayer. El gate vive en TestSemanticVsLexicalReal, sobre el fixture versionado.
func TestABLexicoVsHibridoFixtureReal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirPotion := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirPotion == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB y/o MUSUBI_POTION_DIR: se saltea el A/B sobre memoria real")
	}

	prov, err := embedding.NewStaticProvider(dirPotion)
	if err != nil {
		t.Fatalf("NewStaticProvider(%s): %v", dirPotion, err)
	}
	embed := func(texto string) ([]float32, error) {
		return prov.Embed(context.Background(), texto)
	}

	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB(%s): %v", ruta, err)
	}
	relevantes := 0
	for _, q := range fx.Queries {
		relevantes += len(q.Relevant)
	}
	t.Logf("corpus real: %d docs · %d consultas · %.1f relevantes por consulta · embedder %s (dim %d)",
		len(fx.Docs), len(fx.Queries), float64(relevantes)/float64(len(fx.Queries)),
		prov.Name(), prov.Dimensions())

	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, embed,
		[]Config{lexicalConfig, hybridConfig}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("\n%s", FormatReport(scores, ks))

	var lex, hyb Scores
	for _, s := range scores {
		switch s.Config {
		case lexicalConfig.Name:
			lex = s
		case hybridConfig.Name:
			hyb = s
		}
	}
	t.Logf("DELTA híbrido − léxico sobre el corpus real:")
	for _, k := range ks {
		t.Logf("  R@%-2d  %+.4f   (léxico %.4f → híbrido %.4f)",
			k, hyb.RecallAtK[k]-lex.RecallAtK[k], lex.RecallAtK[k], hyb.RecallAtK[k])
		t.Logf("  nDCG@%-2d %+.4f   (léxico %.4f → híbrido %.4f)",
			k, hyb.NDCGAtK[k]-lex.NDCGAtK[k], lex.NDCGAtK[k], hyb.NDCGAtK[k])
	}
	t.Logf("  MRR   %+.4f   (léxico %.4f → híbrido %.4f)", hyb.MRR-lex.MRR, lex.MRR, hyb.MRR)
}

// TestBarridoVectorFloorFixtureReal barre el PISO DE COSENO sobre el corpus real.
//
// Existe por un resultado concreto: el A/B con hybridConfig tal como está declarado dio el híbrido
// PEOR que el léxico en todas las métricas (MRR 0.424 → 0.370). Y hybridConfig deja VectorFloor en
// el cero de Go, contra el 0.30 que es el default de producción (config.Default().Memory).
//
// Con piso 0, augmentWithVectorPool admite las 50 candidatas que devuelva el coseno sin importar
// cuán flojas sean, y cada una entra al RRF con su 1/(60+rango) como si fuera una señal. O sea que
// el brazo "híbrido" del banco no mide la señal vectorial: mide la señal vectorial SIN SU GUARDA.
//
// El barrido separa las dos hipótesis:
//   - Si el híbrido mejora al subir el piso, el problema era la config del banco.
//   - Si pierde a todo piso, la señal vectorial estática no se gana su lugar en ESTE corpus, y eso
//     es un resultado que hay que saber antes de construir nada encima.
func TestBarridoVectorFloorFixtureReal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirPotion := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirPotion == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB y/o MUSUBI_POTION_DIR: se saltea el barrido")
	}
	prov, err := embedding.NewStaticProvider(dirPotion)
	if err != nil {
		t.Fatalf("NewStaticProvider: %v", err)
	}
	embed := func(texto string) ([]float32, error) { return prov.Embed(context.Background(), texto) }

	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}

	base := memory.RecallOptions{Stemming: true, Cooccurrence: true, GraphCentrality: true}
	configs := []Config{lexicalConfig}
	for _, piso := range []float64{0.0, 0.20, 0.30, 0.40, 0.50, 0.60} {
		o := base
		o.VectorFloor = piso
		configs = append(configs, Config{
			Name:      fmt.Sprintf("hybrid-floor-%.2f", piso),
			Opts:      o,
			UseVector: true,
		})
	}
	// Y el de producción de verdad: piso 0.30 CON MMR encendido en 0.75.
	prod := base
	prod.VectorFloor = 0.30
	prod.MMRLambda = 0.75
	configs = append(configs, Config{Name: "hybrid-PRODUCCION", Opts: prod, UseVector: true})

	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, embed, configs, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("barrido de VectorFloor sobre %d docs / %d consultas:\n%s", len(fx.Docs), len(fx.Queries), FormatReport(scores, ks))
}
