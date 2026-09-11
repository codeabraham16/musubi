package recalleval

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// TestDistribucionDelPisoFixtureReal muestra QUÉ FRACCIÓN del pool vectorial deja pasar cada piso
// de coseno, preguntándole al MOTOR (SearchObservations), que es lo único que decide de verdad.
//
// SE LE PREGUNTA AL MOTOR Y NO SE RECALCULA POR FUERA, y no es un detalle de estilo: la primera
// versión de este test embebía y comparaba por su cuenta, y dio un máximo de 0.4866 cuando el motor
// devuelve 0.7602. Con el número de afuera la conclusión se daba vuelta —"con piso 0.60 el pool
// tiene que estar vacío"— y era falsa. Cualquier medición sobre el ranking tiene que salir del
// mismo camino que el ranking usa.
func TestDistribucionDelPisoFixtureReal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirPotion := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirPotion == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB y/o MUSUBI_POTION_DIR")
	}
	prov, err := embedding.NewStaticProvider(dirPotion)
	if err != nil {
		t.Fatalf("NewStaticProvider: %v", err)
	}
	embed := func(s string) ([]float32, error) { return prov.Embed(context.Background(), s) }
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	eng, err := SeedEngine(t.TempDir(), fx, embed)
	if err != nil {
		t.Fatalf("SeedEngine: %v", err)
	}
	defer eng.Close()

	var sims []float64
	for _, q := range fx.Queries {
		qv, err := embed(q.Text)
		if err != nil {
			t.Fatalf("embed consulta: %v", err)
		}
		res, err := eng.SearchObservations(context.Background(), qv, 50)
		if err != nil {
			t.Fatalf("SearchObservations: %v", err)
		}
		for _, r := range res {
			sims = append(sims, float64(r.Similarity))
		}
	}
	sort.Float64s(sims)
	n := len(sims)
	if n == 0 {
		t.Fatal("el pool vectorial vino vacío: el fixture no tiene vectores")
	}
	pct := func(q float64) float64 {
		i := int(q * float64(n))
		if i >= n {
			i = n - 1
		}
		return sims[i]
	}
	t.Logf("similitud del pool vectorial (top-50 por consulta, n=%d): min %.4f · p50 %.4f · p90 %.4f · p99 %.4f · max %.4f",
		n, sims[0], pct(0.50), pct(0.90), pct(0.99), sims[n-1])
	for _, piso := range []float64{0.30, 0.40, 0.50, 0.60, 0.70} {
		pasan := 0
		for _, x := range sims {
			if x >= piso {
				pasan++
			}
		}
		t.Logf("  piso %.2f -> %4d de %d candidatas entran al RRF (%.1f%%)",
			piso, pasan, n, float64(pasan)/float64(n)*100)
	}
}

// TestPisoImposibleDebeIgualarAlLexico es un CONTROL DE INSTRUMENTO, no una medición de calidad.
//
// Con un piso por encima del coseno máximo observado, augmentWithVectorPool descarta todos los
// vecinos, vecRank queda vacío y la función devuelve (cands, nil, nil): el ranking tiene que ser
// BIT-IDÉNTICO al del brazo léxico. Si no lo es, entonces el brazo "híbrido" del banco difiere del
// léxico por algo MÁS que la señal vectorial, y todo delta que se le atribuya a los vectores está
// contaminado por esa otra cosa.
//
// Se agrega porque el barrido dio hybrid-floor-0.60 con MRR 0.438 contra 0.424 del léxico, mientras
// que el máximo coseno consulta-documento medido sobre 165.835 pares fue 0.4866. Los dos números no
// pueden ser ciertos a la vez.
func TestPisoImposibleDebeIgualarAlLexico(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirPotion := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirPotion == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB y/o MUSUBI_POTION_DIR")
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

	imposible := memory.RecallOptions{Stemming: true, Cooccurrence: true, GraphCentrality: true}
	imposible.VectorFloor = 0.99 // por encima de cualquier coseno consulta-documento observado

	ks := []int{1, 5, 10}
	scores, err := Run(context.Background(), t.TempDir(), fx, embed, []Config{
		lexicalConfig,
		{Name: "hybrid-floor-0.99", Opts: imposible, UseVector: true},
	}, ks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Logf("\n%s", FormatReport(scores, ks))

	var lex, hyb Scores
	for _, s := range scores {
		if s.Config == lexicalConfig.Name {
			lex = s
		} else {
			hyb = s
		}
	}
	if lex.MRR != hyb.MRR || lex.RecallAtK[10] != hyb.RecallAtK[10] {
		t.Errorf("EL BRAZO HÍBRIDO DIFIERE DEL LÉXICO CON EL POOL VECTORIAL VACÍO: "+
			"MRR %.6f vs %.6f · R@10 %.6f vs %.6f. El delta que el banco le atribuye a los vectores "+
			"incluye algo más que los vectores.", lex.MRR, hyb.MRR, lex.RecallAtK[10], hyb.RecallAtK[10])
	}
}

// TestFixtureTraeLasAristasDelGrafo fija que el banco ejercite la QUINTA señal RRF.
//
// El defecto que cierra: SeedEngine nunca creaba observation_relations, así que buildObsGraph
// cargaba un grafo vacío, graphRank salía vacío en TODA medición, y una config con
// GraphCentrality:true era bit-idéntica a una con false. El gate de CI defendía una señal que no
// ejercitaba nunca — cualquier cambio que rompiera la centralidad habría pasado en verde.
func TestFixtureTraeLasAristasDelGrafo(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	if ruta == "" {
		t.Skip("falta MUSUBI_FIXTURE_DB")
	}
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	if len(fx.Relaciones) == 0 {
		t.Fatal("el fixture no trajo ni una arista: la quinta señal RRF sigue siendo un no-op en el banco")
	}
	t.Logf("aristas en el fixture: %d (sobre %d docs)", len(fx.Relaciones), len(fx.Docs))

	// Las dos puntas de cada arista tienen que estar en el corpus, o se siembra una relación
	// huérfana — justo lo que el check orphan_relations del doctor existe para encontrar.
	vivos := make(map[string]bool, len(fx.Docs))
	for _, d := range fx.Docs {
		vivos[d.ID] = true
	}
	for _, r := range fx.Relaciones {
		if !vivos[r.Source] || !vivos[r.Target] {
			t.Errorf("arista huérfana en el fixture: %s -> %s", r.Source, r.Target)
			break
		}
		if r.Source == r.Target {
			t.Errorf("self-loop en el fixture: %s", r.Source)
			break
		}
	}

	// Y que al sembrarlas el grafo REALMENTE tenga nodos: sin esto el arreglo sería cosmético.
	eng, err := SeedEngine(t.TempDir(), fx, nil)
	if err != nil {
		t.Fatalf("SeedEngine: %v", err)
	}
	defer eng.Close()
	rels, err := eng.AllObsRelations()
	if err != nil {
		t.Fatalf("AllObsRelations: %v", err)
	}
	if len(rels) == 0 {
		t.Error("SeedEngine no sembró ninguna arista: el grafo del banco sigue vacío")
	}
	t.Logf("aristas sembradas en el motor: %d", len(rels))
}

// baseConTextoLargoYCuarentena siembra una base donde el gist SÍ recorta (texto largo) y donde hay
// una observación en cuarentena, que son las dos cosas que baseDePrueba no puede producir.
//
// POR QUÉ NO ALCANZA baseDePrueba, medido: su texto de semilla da un gist IDÉNTICO al content
// (Gist con tope de 24 tokens no recorta 84 caracteres). Una prueba de "content vs gist" escrita
// con esa semilla queda VERDE con el arreglo sacado, porque las dos ramas devuelven lo mismo. Es
// exactamente el modo de falla que esta rama ya cometió dos veces: el fixture llega arreglado por
// otra capa y la prueba no prueba nada.
func baseConTextoLargoYCuarentena(t *testing.T) (ruta string, idCuarentenada string) {
	t.Helper()
	dir := t.TempDir()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	largo := strings.Repeat("una oración larga que obliga al gist a recortar de verdad y no a copiar. ", 40)
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("t/largo#%d", i)
		if err := eng.SaveObservation(id, "t/largo", fmt.Sprintf("nota %d: %s", i, largo), nil); err != nil {
			t.Fatalf("SaveObservation: %v", err)
		}
	}
	idCuarentenada = "t/largo#3"
	eng.Close() // cerrar antes de tocarla por fuera: single-writer

	ruta = filepath.Join(dir, ".musubi", "memory.db")
	// Se cuarentena por SQL directo porque no hay API pública para hacerlo, y porque el caso REAL
	// que esto representa —una fila marcada como no confiable— llega por el camino de cuarentena
	// del motor, no por un save.
	db, err := sql.Open("sqlite", "file:"+ruta)
	if err != nil {
		t.Fatalf("abrir para cuarentenar: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`UPDATE observations SET quarantined = 1 WHERE id = ?`, idCuarentenada); err != nil {
		t.Fatalf("cuarentenar: %v", err)
	}
	return ruta, idCuarentenada
}

// TestFixtureUsaContentYNoGist fija que el default del fixture sea el texto COMPLETO.
//
// El defecto que cierra: los docs salían del gist (82 caracteres de promedio en la base real)
// mientras el backfill embebe el content (2.790). El banco vectorizaba el 2,9% del texto de
// producción, y eso dio vuelta la conclusión entera sobre la capa vectorial cuando se corrigió.
func TestFixtureUsaContentYNoGist(t *testing.T) {
	ruta, _ := baseConTextoLargoYCuarentena(t)

	porDefecto, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{MinPorTopico: 2})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	conGist, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{MinPorTopico: 2, TextoDelDoc: DocDesdeGist})
	if err != nil {
		t.Fatalf("FixtureDesdeDB(gist): %v", err)
	}
	if len(porDefecto.Docs) == 0 || len(conGist.Docs) == 0 {
		t.Fatal("el fixture salió vacío: la semilla no sirve para esta prueba")
	}
	// LA GUARDA DE LA GUARDA: si la semilla no hace que el gist recorte, esta prueba no puede
	// distinguir nada y hay que decirlo en vez de pasar en verde por el motivo equivocado.
	if len(conGist.Docs[0].Content) >= len(porDefecto.Docs[0].Content) {
		t.Fatalf("la semilla no hace recortar al gist (gist %d chars, content %d): la prueba sería vacua",
			len(conGist.Docs[0].Content), len(porDefecto.Docs[0].Content))
	}
	t.Logf("gist %d chars vs content %d chars", len(conGist.Docs[0].Content), len(porDefecto.Docs[0].Content))
}

// TestFixtureExcluyeCuarentena fija que el corpus del banco sea el MISMO universo que el recall
// puede devolver. Sin esto el banco mide contra un corpus que producción no tiene.
func TestFixtureExcluyeCuarentena(t *testing.T) {
	ruta, idQuar := baseConTextoLargoYCuarentena(t)
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{MinPorTopico: 2})
	if err != nil {
		t.Fatalf("FixtureDesdeDB: %v", err)
	}
	for _, d := range fx.Docs {
		if d.ID == idQuar {
			t.Fatalf("la observación en cuarentena %s entró al corpus del banco: el recall nunca la devuelve", idQuar)
		}
	}
	if len(fx.Docs) != 3 {
		t.Errorf("esperaba 3 docs visibles (4 menos la cuarentenada), obtuve %d", len(fx.Docs))
	}
}
