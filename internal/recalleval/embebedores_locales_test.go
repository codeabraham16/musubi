package recalleval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory/memtest"
)

// instruccionHarrier es la instrucción de CONSULTA que declara la model card de
// microsoft/harrier-oss-v1-270m (el prompt «web_search_query» de su config de sentence-transformers):
// «Each query must come with a one-sentence instruction», y «no need to add instruction for retrieval
// documents». Sin ella, el modelo se mide fuera de las condiciones en que se entrenó.
const instruccionHarrier = "Instruct: Given a web search query, retrieve relevant passages that answer the query\nQuery: "

// varianteConsulta es una forma de embeber las consultas con el MISMO índice de documentos.
type varianteConsulta struct {
	Nombre          string
	PrefijoConsulta string
}

// candidatoEmbebedor es un embebedor a comparar contra POTION.
type candidatoEmbebedor struct {
	Nombre   string
	Modelo   string // tag en Ollama; vacío si no pasa por Ollama
	Dim      int
	Licencia string
	// Fuente dice de dónde sale la regla de prefijos, para que quien lea el número sepa si el
	// modelo se midió como pide su card.
	Fuente     string
	PrefijoDoc string
	Variantes  []varianteConsulta
	// Con Sabotaje en true el embebedor está ROTO a propósito, y la prueba exige que salga peor que
	// POTION. Si no sale, exigirQueElSabotajePierda dice por qué: o el arnés no usa el embebedor, o
	// el fixture no tiene potencia para separar la señal de POTION de la falta de señal.
	Sabotaje bool
	local    func() embedding.Provider // sólo para los que no pasan por Ollama
}

// proveedor arma el embebedor por el MISMO constructor que producción (embedding.NewProvider): con
// Ollama eso trae el portero de privacidad y el troceo, que son parte de lo que cuesta y de lo que
// sale.
func (c candidatoEmbebedor) proveedor(urlOllama string) (embedding.Provider, error) {
	if c.Modelo == "" {
		return c.local(), nil
	}
	return embedding.NewProvider(config.EmbeddingConfig{
		Provider: "ollama", BaseURL: urlOllama, Model: c.Modelo, Dimensions: c.Dim,
	})
}

// catalogoEmbebedores son los candidatos que el arnés sabe medir. Las licencias y las conversiones
// GGUF se verificaron contra las model cards el 2026-09-30 (ver el informe de la unidad).
func catalogoEmbebedores() []candidatoEmbebedor {
	return []candidatoEmbebedor{
		{
			Nombre: "bge-m3", Modelo: "bge-m3:latest", Dim: 1024, Licencia: "MIT",
			Fuente:    "BAAI/bge-m3: la recuperación densa va sin instrucción",
			Variantes: []varianteConsulta{{"bge-m3", ""}},
		},
		{
			Nombre: "granite-97m", Modelo: "hf.co/atfai/granite-embedding-97m-multilingual-r2-GGUF:F16", Dim: 384,
			Licencia:  "Apache-2.0",
			Fuente:    "ibm-granite/granite-embedding-97m-multilingual-r2: sin prefijos (pooling CLS)",
			Variantes: []varianteConsulta{{"granite-97m", ""}},
		},
		{
			Nombre: "granite-311m", Modelo: "hf.co/mykor/granite-embedding-311m-multilingual-r2-GGUF:Q8_0", Dim: 768,
			Licencia:  "Apache-2.0",
			Fuente:    "ibm-granite/granite-embedding-311m-multilingual-r2: sin prefijos (pooling CLS)",
			Variantes: []varianteConsulta{{"granite-311m", ""}},
		},
		{
			Nombre: "harrier-270m", Modelo: "hf.co/mykor/harrier-oss-v1-270m-GGUF:Q8_0", Dim: 640, Licencia: "MIT",
			Fuente: "microsoft/harrier-oss-v1-270m: instrucción en la consulta, documentos sin nada (pooling del último token)",
			Variantes: []varianteConsulta{
				{"harrier-270m+instr", instruccionHarrier}, // lo que pide la card
				{"harrier-270m", ""},                       // lo que Musubi haría hoy: no tiene prefijos
			},
		},
		{
			Nombre: "constante", Dim: 256, Licencia: "-", Sabotaje: true,
			Fuente:    "sabotaje: el mismo vector para cualquier texto",
			Variantes: []varianteConsulta{{"constante", ""}},
			local:     func() embedding.Provider { return embebedorConstante{dim: 256} },
		},
	}
}

// embebedorConstante es el EMBEBEDOR ROTO: devuelve el mismo vector unitario para cualquier texto.
// Su coseno contra cualquier documento es 1, así que la señal vectorial no discrimina nada: el pool
// vectorial entra al RRF en un orden que no dice nada de la consulta.
type embebedorConstante struct{ dim int }

func (c embebedorConstante) Embed(context.Context, string) ([]float32, error) {
	v := make([]float32, c.dim)
	x := float32(1 / math.Sqrt(float64(c.dim)))
	for i := range v {
		v[i] = x
	}
	return v, nil
}
func (c embebedorConstante) Dimensions() int { return c.dim }
func (c embebedorConstante) Name() string    { return "sabotaje:constante" }

// fixtureNombrado es un fixture con el nombre con el que se reporta.
type fixtureNombrado struct {
	Nombre string
	Fx     *Fixture
}

// modeloCargado es una fila de /api/ps de Ollama.
type modeloCargado struct {
	Nombre   string `json:"name"`
	Modelo   string `json:"model"`
	Size     int64  `json:"size"`
	SizeVRAM int64  `json:"size_vram"`
	Contexto int    `json:"context_length"`
}

// medicionEmbebedor es todo lo que se midió de UN embebedor en una corrida.
type medicionEmbebedor struct {
	Nombre        string                `json:"nombre"`
	Modelo        string                `json:"modelo,omitempty"`
	Dim           int                   `json:"dim"`
	Licencia      string                `json:"licencia,omitempty"`
	Fuente        string                `json:"fuente_prefijos,omitempty"`
	EstabaCargado bool                  `json:"estaba_cargado"`
	ConstruirMs   float64               `json:"construir_ms"`       // embedding.NewProvider: con POTION es cargar la tabla
	PrimeraMs     float64               `json:"primera_llamada_ms"` // en frío si el modelo no estaba cargado
	Reembebido    map[string]Reembebido `json:"reembebido"`         // por fixture
	Lotes         ResumenLatencia       `json:"lotes_backfill"`     // las llamadas del backfill, todos los fixtures
	DocUnitario   ResumenLatencia       `json:"doc_de_a_uno"`       // muestra de documentos embebidos de a uno
	Consultas     ResumenLatencia       `json:"consultas"`
	MemAntesMB    int                   `json:"mem_disponible_antes_mb"`
	MemCargadoMB  int                   `json:"mem_disponible_cargado_mb"`
	// RSS de ESTE proceso antes de construir el embebedor y con él cargado. Con POTION la tabla vive
	// acá; con Ollama vive en el runner de Ollama y la cuenta es OllamaPS.
	RSSAntesMB   int             `json:"rss_proceso_antes_mb"`
	RSSCargadoMB int             `json:"rss_proceso_cargado_mb"`
	OllamaPS     []modeloCargado `json:"ollama_ps,omitempty"`
	// La presión de memoria con la que se midió, porque las latencias no se leen sin ella: con la
	// tabla de POTION en swap, cada fila que el embebido toca puede ser un fallo de página, y el
	// costo que se anota es el del swap y no el del modelo. Swap usado de la máquina (SwapTotal −
	// SwapFree) antes y con el embebedor cargado, y el VmSwap de ESTE proceso con él cargado.
	SwapAntesMB     int `json:"swap_usado_antes_mb"`
	SwapCargadoMB   int `json:"swap_usado_cargado_mb"`
	VmSwapCargadoMB int `json:"vmswap_proceso_cargado_mb"`
	// scores NO va al JSON: trae el detalle por consulta, y en el fixture real el id de una consulta
	// es un topic de la memoria de alguien.
	scores map[string]map[string][]Scores // variante → fixture → uno por config
}

// filaPareada es una métrica de un brazo contra su base, en un fixture y una config. La base es
// POTION (Contra "potion": la variante contra el embebedor por defecto) o la config léxica del MISMO
// motor (Contra "lexical": lo que suma el vector).
type filaPareada struct {
	Variante string  `json:"variante"`
	Fixture  string  `json:"fixture"`
	Config   string  `json:"config"`
	Contra   string  `json:"contra"`
	Metrica  string  `json:"metrica"`
	Base     float64 `json:"base"`
	Valor    float64 `json:"valor"`
	Gana     int     `json:"gana"`
	Pierde   int     `json:"pierde"`
	Empata   int     `json:"empata"`
	Delta    float64 `json:"delta"`
	ICLo     float64 `json:"ic95_lo"`
	ICHi     float64 `json:"ic95_hi"`
	P        float64 `json:"p_signo"`
}

// metricasPareadas son las que pide la comparación: MRR, R@5, R@10 y nDCG@10.
var metricasPareadas = []struct {
	metrica string
	k       int
}{{"rr", 0}, {"recall", 5}, {"recall", 10}, {"ndcg", 10}}

const (
	repsBootstrap    = 10000
	semillaBootstrap = 20260930
)

// hibridaSinPiso es ConfigHibrida con el piso de coseno en 0. El 0,30 de producción se calibró con
// la distribución de cosenos de POTION, y cada modelo tiene la suya: un piso que para uno recorta
// ruido, para otro puede dejar pasar todo o no dejar pasar nada. Medir sin piso separa «el modelo es
// peor» de «el piso no le queda».
func hibridaSinPiso() Config {
	c := ConfigHibrida()
	c.Name = "hybrid-piso0"
	c.Opts.VectorFloor = 0
	return c
}

// TestEmbebedoresLocalesVsPotion compara embebedores LOCALES (Ollama) contra POTION, el embebedor
// por defecto, con la config híbrida sobre el dorado y —si hay MUSUBI_FIXTURE_DB— sobre el fixture
// real, de a pares por consulta (CompararPareado) con test de signo e IC95 por bootstrap. Reporta
// además lo que cuesta cada uno en esta máquina: la primera llamada, el tiempo de re-embeber el
// corpus por el backfill de producción, la latencia por documento y por consulta, y la RAM de Ollama.
//
// Es una MEDICIÓN, no un gate: imprime para que decida una persona. Lo único que asevera es que el
// instrumento funciona: un candidato marcado Sabotaje tiene que salir peor que POTION, y el brazo
// léxico tiene que ser idéntico entre motores (si no, algo que no es el embebedor está cambiando).
//
// Entorno (sólo NOMBRES; los valores son de quien corre):
//   - MUSUBI_EMBED_CANDIDATOS: lista separada por comas del catálogo (bge-m3, granite-97m,
//     granite-311m, harrier-270m, constante). Sin ella se saltea.
//   - MUSUBI_POTION_DIR: la tabla POTION, la línea base. Sin ella se saltea.
//   - MUSUBI_OLLAMA_URL: el Ollama LOCAL. Tiene que ser loopback: el corpus real no sale de la máquina.
//   - MUSUBI_FIXTURE_DB: opcional, una COPIA de una memoria real. Nunca la base viva.
//   - MUSUBI_EMBED_SALIDA: opcional, dónde escribir el resumen en JSON (sin texto de la memoria).
//
// Un modelo de Ollama por corrida: la máquina de referencia tiene 7 GB compartidos, y dos modelos
// cargados a la vez miden la presión de memoria, no el modelo.
func TestEmbebedoresLocalesVsPotion(t *testing.T) {
	lista := os.Getenv("MUSUBI_EMBED_CANDIDATOS")
	dirPotion := os.Getenv("MUSUBI_POTION_DIR")
	if lista == "" || dirPotion == "" {
		t.Skip("faltan MUSUBI_EMBED_CANDIDATOS y/o MUSUBI_POTION_DIR: se saltea la comparación de embebedores locales")
	}
	catalogo := map[string]candidatoEmbebedor{}
	var nombres []string
	for _, c := range catalogoEmbebedores() {
		catalogo[c.Nombre] = c
		nombres = append(nombres, c.Nombre)
	}
	var elegidos []candidatoEmbebedor
	deOllama := 0
	for _, n := range strings.Split(lista, ",") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		c, ok := catalogo[n]
		if !ok {
			t.Fatalf("candidato desconocido %q; el catálogo tiene: %s", n, strings.Join(nombres, ", "))
		}
		if c.Modelo != "" {
			deOllama++
		}
		elegidos = append(elegidos, c)
	}
	if deOllama > 1 {
		t.Fatalf("un modelo de Ollama por corrida: corré uno, `ollama stop <modelo>`, y después el siguiente")
	}
	urlOllama := os.Getenv("MUSUBI_OLLAMA_URL")
	if deOllama > 0 {
		if urlOllama == "" {
			t.Fatalf("hay candidatos de Ollama y falta MUSUBI_OLLAMA_URL")
		}
		if err := exigirLoopback(urlOllama); err != nil {
			t.Fatal(err)
		}
	}

	fixtures := []fixtureNombrado{{"dorado", loadGolden(t)}}
	if ruta := os.Getenv("MUSUBI_FIXTURE_DB"); ruta != "" {
		fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
		if err != nil {
			t.Fatalf("FixtureDesdeDB: %v", err)
		}
		fixtures = append(fixtures, fixtureNombrado{"real", fx})
	}
	for _, f := range fixtures {
		car := 0
		for _, d := range f.Fx.Docs {
			car += len(d.Content)
		}
		t.Logf("fixture %s: %d docs (%d caracteres) · %d consultas", f.Nombre, len(f.Fx.Docs), car, len(f.Fx.Queries))
	}

	ctx := context.Background()
	ks := []int{1, 5, 10}
	configs := []Config{ConfigLexica(), ConfigHibrida(), hibridaSinPiso()}

	// 1) POTION primero, y se suelta ANTES de cargar cualquier modelo de Ollama: su tabla son ~500 MB
	// en este proceso, y sumada al modelo mediría la presión de memoria en vez del modelo.
	base := medirPotion(t, ctx, dirPotion, fixtures, configs, ks)
	runtime.GC()
	debug.FreeOSMemory()
	t.Logf("%s", costoLegible(base))

	salida := struct {
		Fecha      string              `json:"fecha"`
		CPU        string              `json:"cpu"`
		Hilos      int                 `json:"hilos"`
		MemTotalMB int                 `json:"mem_total_mb"`
		Fixtures   map[string][2]int   `json:"fixtures"` // docs, consultas
		Potion     medicionEmbebedor   `json:"potion"`
		Candidatos []medicionEmbebedor `json:"candidatos"`
		Filas      []filaPareada       `json:"filas"`
	}{
		Fecha: time.Now().Format(time.RFC3339), CPU: modeloDeCPU(), Hilos: runtime.NumCPU(),
		MemTotalMB: leerMeminfoMB("MemTotal"), Fixtures: map[string][2]int{}, Potion: base,
	}
	for _, f := range fixtures {
		salida.Fixtures[f.Nombre] = [2]int{len(f.Fx.Docs), len(f.Fx.Queries)}
	}
	sumaPotion := compararContraLexico(t, base, fixtures, configs)
	t.Logf("LO QUE SUMA EL VECTOR con potion (la híbrida contra la léxica del mismo motor):\n%s", tablaLegible(sumaPotion))
	salida.Filas = append(salida.Filas, sumaPotion...)

	// 2) Los candidatos, de a uno.
	for _, c := range elegidos {
		rssAntes := leerStatusMB("VmRSS")
		t0 := time.Now()
		prov, err := c.proveedor(urlOllama)
		if err != nil {
			t.Fatalf("%s: construir el embebedor: %v", c.Nombre, err)
		}
		construir := milis(time.Since(t0))
		m := medirEmbebedor(t, ctx, c, prov, urlOllama, fixtures, configs, ks)
		m.ConstruirMs, m.RSSAntesMB = construir, rssAntes
		t.Logf("%s", costoLegible(m))
		exigirLexicoIdentico(t, base, m, fixtures)
		filas := compararContraPotion(t, base, m, fixtures, configs)
		t.Logf("CONTRA POTION:\n%s", tablaLegible(filas))
		if c.Sabotaje {
			exigirQueElSabotajePierda(t, c.Nombre, filas, fixtures)
		}
		suma := compararContraLexico(t, m, fixtures, configs)
		t.Logf("LO QUE SUMA EL VECTOR con %s:\n%s", c.Nombre, tablaLegible(suma))
		salida.Candidatos = append(salida.Candidatos, m)
		salida.Filas = append(salida.Filas, filas...)
		salida.Filas = append(salida.Filas, suma...)
	}

	if ruta := os.Getenv("MUSUBI_EMBED_SALIDA"); ruta != "" {
		b, err := json.MarshalIndent(salida, "", "  ")
		if err != nil {
			t.Fatalf("serializar la salida: %v", err)
		}
		if err := os.WriteFile(ruta, b, 0o600); err != nil {
			t.Fatalf("escribir la salida: %v", err)
		}
	}
}

// medirPotion mide la línea base. Queda en una función aparte para que la tabla (~500 MB) no
// sobreviva en ninguna variable del test cuando empiezan los candidatos.
func medirPotion(t *testing.T, ctx context.Context, dir string, fixtures []fixtureNombrado, configs []Config, ks []int) medicionEmbebedor {
	t.Helper()
	rssAntes := leerStatusMB("VmRSS")
	t0 := time.Now()
	prov, err := embedding.NewProvider(config.EmbeddingConfig{Provider: "static", StaticPath: dir})
	if err != nil {
		t.Fatalf("POTION: %v", err)
	}
	construir := milis(time.Since(t0))
	c := candidatoEmbebedor{
		Nombre: "potion", Dim: prov.Dimensions(), Licencia: "MIT",
		Fuente:    "model2vec: tabla estática, sin prefijos",
		Variantes: []varianteConsulta{{"potion", ""}},
	}
	m := medirEmbebedor(t, ctx, c, prov, "", fixtures, configs, ks)
	m.ConstruirMs, m.RSSAntesMB = construir, rssAntes
	return m
}

// medirEmbebedor corre un embebedor entero: la primera llamada (en frío si el modelo no estaba
// cargado), el re-embebido de cada fixture por el backfill de producción, cada variante de consulta
// con cada config, una muestra de documentos embebidos de a uno y la RAM con el modelo cargado.
func medirEmbebedor(t *testing.T, ctx context.Context, c candidatoEmbebedor, prov embedding.Provider, urlOllama string,
	fixtures []fixtureNombrado, configs []Config, ks []int) medicionEmbebedor {
	t.Helper()
	m := medicionEmbebedor{
		Nombre: c.Nombre, Modelo: c.Modelo, Dim: c.Dim, Licencia: c.Licencia, Fuente: c.Fuente,
		Reembebido: map[string]Reembebido{}, scores: map[string]map[string][]Scores{},
	}
	m.MemAntesMB = leerMeminfoMB("MemAvailable")
	m.SwapAntesMB = swapUsadoMB()
	if c.Modelo != "" {
		ps, err := ollamaPS(ctx, urlOllama)
		if err != nil {
			t.Fatalf("%s: consultar /api/ps: %v", c.Nombre, err)
		}
		for _, x := range ps {
			if x.Nombre == c.Modelo || x.Modelo == c.Modelo {
				m.EstabaCargado = true
			}
		}
	}
	t0 := time.Now()
	v, err := prov.Embed(ctx, "calentamiento del embebedor")
	if err != nil {
		t.Fatalf("%s: la primera llamada falló (¿el modelo está bajado?): %v", c.Nombre, err)
	}
	m.PrimeraMs = milis(time.Since(t0))
	if len(v) != c.Dim {
		t.Fatalf("%s: devolvió vectores de %d dimensiones y el catálogo dice %d", c.Nombre, len(v), c.Dim)
	}

	lotes, consultas, unitario := &Cronometro{}, &Cronometro{}, &Cronometro{}
	for i, f := range fixtures {
		eng, rb, err := SembrarYReembeber(memtest.DirSembrado(t), f.Fx, LoteDeDocumentos(ctx, prov, c.PrefijoDoc, lotes))
		if err != nil {
			t.Fatalf("%s/%s: %v", c.Nombre, f.Nombre, err)
		}
		m.Reembebido[f.Nombre] = rb
		if rb.Embebidos != rb.Sembrados || rb.Sembrados == 0 {
			eng.Close()
			t.Fatalf("%s/%s: re-embebió %d de %d sembrados (fallidos %d, salteados %d): con huecos en el índice la comparación no es pareja",
				c.Nombre, f.Nombre, rb.Embebidos, rb.Sembrados, rb.Fallidos, rb.Salteados)
		}
		if i == len(fixtures)-1 {
			// La RAM se mira con el embebedor recién usado por el backfill, o sea cargado de verdad.
			if c.Modelo != "" {
				if ps, err := ollamaPS(ctx, urlOllama); err == nil {
					m.OllamaPS = ps
				}
			}
			m.MemCargadoMB = leerMeminfoMB("MemAvailable")
			m.RSSCargadoMB = leerStatusMB("VmRSS")
			m.SwapCargadoMB = swapUsadoMB()
			m.VmSwapCargadoMB = leerStatusMB("VmSwap")
		}
		for _, va := range c.Variantes {
			if m.scores[va.Nombre] == nil {
				m.scores[va.Nombre] = map[string][]Scores{}
			}
			q := EmbedDeConsultas(ctx, prov, va.PrefijoConsulta, consultas)
			for _, cfg := range configs {
				s, err := Evaluate(ctx, eng, f.Fx, cfg, q, ks)
				if err != nil {
					eng.Close()
					t.Fatalf("%s/%s/%s: %v", va.Nombre, f.Nombre, cfg.Name, err)
				}
				m.scores[va.Nombre][f.Nombre] = append(m.scores[va.Nombre][f.Nombre], s)
			}
		}
		eng.Close()
	}

	// Documentos de a uno: lo que cuesta embeber UNA observación nueva, que no es lo mismo que el
	// lote del backfill. La muestra sale repartida a lo largo del fixture más grande.
	docs := fixtures[len(fixtures)-1].Fx.Docs
	const muestra = 16
	for i := 0; i < muestra && i < len(docs); i++ {
		d := docs[i*len(docs)/muestra]
		t0 := time.Now()
		if _, err := prov.Embed(ctx, c.PrefijoDoc+d.Content); err != nil {
			t.Fatalf("%s: embeber un documento de a uno: %v", c.Nombre, err)
		}
		unitario.Anotar(time.Since(t0), 1, len(c.PrefijoDoc)+len(d.Content))
	}
	m.Lotes, m.Consultas, m.DocUnitario = lotes.Resumen(), consultas.Resumen(), unitario.Resumen()
	return m
}

// compararContraPotion enfrenta cada variante del candidato con POTION, config híbrida por config
// híbrida, en cada fixture y métrica.
func compararContraPotion(t *testing.T, base, cand medicionEmbebedor, fixtures []fixtureNombrado, configs []Config) []filaPareada {
	t.Helper()
	var filas []filaPareada
	for _, f := range fixtures {
		for i, cfg := range configs {
			if !cfg.UseVector {
				continue // el léxico es el mismo en los dos motores: lo verifica exigirLexicoIdentico
			}
			sp := base.scores["potion"][f.Nombre][i]
			for _, v := range variantesDe(cand) {
				filas = append(filas, pareo(t, v, f.Nombre, cfg.Name, "potion", sp, cand.scores[v][f.Nombre][i])...)
			}
		}
	}
	return filas
}

// compararContraLexico mide lo que SUMA EL VECTOR: la híbrida de cada variante contra la léxica
// del mismo motor, consulta por consulta. Es la escala de lo demás. El control de sabotaje midió
// que en el fixture real la híbrida de POTION apenas se separa de la de un vector constante; esto
// dice cuánto se separa de no usar vector en absoluto, que es el techo de lo que un embebedor
// puede mover en este banco.
func compararContraLexico(t *testing.T, m medicionEmbebedor, fixtures []fixtureNombrado, configs []Config) []filaPareada {
	t.Helper()
	lex := -1
	for i, cfg := range configs {
		if cfg.Name == ConfigLexica().Name {
			lex = i
		}
	}
	if lex < 0 {
		t.Fatalf("la config léxica no está entre las que se corrieron: no hay contra qué medir lo que suma el vector")
	}
	var filas []filaPareada
	for _, f := range fixtures {
		for _, v := range variantesDe(m) {
			sl := m.scores[v][f.Nombre][lex]
			for i, cfg := range configs {
				if cfg.UseVector {
					filas = append(filas, pareo(t, v, f.Nombre, cfg.Name, "lexical", sl, m.scores[v][f.Nombre][i])...)
				}
			}
		}
	}
	return filas
}

// variantesDe devuelve las variantes medidas de un embebedor, ordenadas.
func variantesDe(m medicionEmbebedor) []string {
	vs := make([]string, 0, len(m.scores))
	for v := range m.scores {
		vs = append(vs, v)
	}
	sort.Strings(vs)
	return vs
}

// pareo enfrenta b contra a, consulta por consulta, en las cuatro métricas de metricasPareadas.
func pareo(t *testing.T, variante, fixture, config, contra string, a, b Scores) []filaPareada {
	t.Helper()
	var filas []filaPareada
	for _, mp := range metricasPareadas {
		cmp, err := CompararPareado(a, b, mp.metrica, mp.k)
		if err != nil {
			t.Fatalf("%s/%s/%s contra %s: %v", variante, fixture, config, contra, err)
		}
		lo, hi := cmp.IntervaloBootstrap(repsBootstrap, semillaBootstrap, 0.95)
		filas = append(filas, filaPareada{
			Variante: variante, Fixture: fixture, Config: config, Contra: contra, Metrica: cmp.Metrica,
			Base: agregadoDe(a, mp.metrica, mp.k), Valor: agregadoDe(b, mp.metrica, mp.k),
			Gana: cmp.Gana, Pierde: cmp.Pierde, Empata: cmp.Empata, Delta: cmp.DeltaMedio,
			ICLo: lo, ICHi: hi, P: cmp.PSigno(),
		})
	}
	return filas
}

// agregadoDe es el promedio de una métrica tal como lo guarda Scores.
func agregadoDe(s Scores, metrica string, k int) float64 {
	switch metrica {
	case "rr":
		return s.MRR
	case "recall":
		return s.RecallAtK[k]
	case "ndcg":
		return s.NDCGAtK[k]
	}
	return math.NaN()
}

// exigirLexicoIdentico verifica que el brazo léxico dé EXACTAMENTE lo mismo en el motor de POTION y
// en el del candidato. No usa vectores, así que cualquier diferencia vendría de otra cosa —la
// siembra, el orden, el reloj— y contaminaría el delta que se le atribuye al embebedor.
//
// Cuenta las consultas que difieren y NO las nombra: en el fixture real el id de una consulta es un
// topic de la memoria de alguien, y este mensaje termina en un log.
func exigirLexicoIdentico(t *testing.T, base, cand medicionEmbebedor, fixtures []fixtureNombrado) {
	t.Helper()
	lex := ConfigLexica().Name
	buscar := func(ss []Scores) *Scores {
		for i := range ss {
			if ss[i].Config == lex {
				return &ss[i]
			}
		}
		return nil
	}
	for _, f := range fixtures {
		sp := buscar(base.scores["potion"][f.Nombre])
		for v, porFx := range cand.scores {
			sc := buscar(porFx[f.Nombre])
			if sp == nil || sc == nil || len(sp.PorConsulta) == 0 {
				t.Errorf("%s/%s: falta el brazo léxico de POTION o del candidato: no se pudo verificar que el léxico sea idéntico", v, f.Nombre)
				continue
			}
			distintas := 0
			for id, mp := range sp.PorConsulta {
				mc, ok := sc.PorConsulta[id]
				if !ok || mc.RR != mp.RR || mc.RecallAtK[10] != mp.RecallAtK[10] || mc.NDCGAtK[10] != mp.NDCGAtK[10] {
					distintas++
				}
			}
			if distintas > 0 || len(sc.PorConsulta) != len(sp.PorConsulta) {
				t.Errorf("%s/%s: el brazo LÉXICO cambió entre el motor de POTION y el del candidato en %d de %d consultas (%d contra %d corridas): "+
					"algo que no es el embebedor está moviendo el ranking", v, f.Nombre, distintas, len(sp.PorConsulta),
					len(sc.PorConsulta), len(sp.PorConsulta))
			}
		}
	}
}

// exigirQueElSabotajePierda es el control del instrumento: un embebedor roto tiene que salir PEOR
// que POTION en la config híbrida de producción, con un IC95 del ΔMRR que no toque el cero. Se mira
// en el fixture más grande (el real si está), porque el dorado tiene 12 consultas y ahí un empate no
// prueba nada.
//
// Distingue las dos formas de fallar, porque dicen cosas distintas:
//   - el ranking híbrido es IDÉNTICO al de POTION en todas las consultas: el arnés no está usando
//     el embebedor, y ningún número de esta prueba vale;
//   - el ranking se mueve pero el IC95 toca el cero: el arnés usa el embebedor, y lo que falta es
//     POTENCIA. Un delta de ese tamaño entre dos candidatos tampoco se puede leer como diferencia.
//
// El criterio se fijó antes de correr y no se afloja: con la copia de la memoria del 2026-09-30 (88
// consultas) salió ROJO por la segunda causa, y eso es un resultado de la medición, no del control.
func exigirQueElSabotajePierda(t *testing.T, nombre string, filas []filaPareada, fixtures []fixtureNombrado) {
	t.Helper()
	fx := fixtures[len(fixtures)-1].Nombre
	hib := ConfigHibrida().Name
	vistas := 0
	for _, f := range filas {
		if f.Contra != "potion" || f.Fixture != fx || f.Config != hib || f.Metrica != "rr" {
			continue
		}
		vistas++
		movidas, n := f.Gana+f.Pierde, f.Gana+f.Pierde+f.Empata
		switch {
		case movidas == 0:
			t.Errorf("%s es un embebedor ROTO y en %s/%s dio el MISMO ranking que POTION en las %d consultas: "+
				"el arnés no está usando el embebedor", nombre, fx, hib, n)
		case !(f.Delta < 0 && f.ICHi < 0):
			t.Errorf("%s es un embebedor ROTO y en %s/%s no salió peor que POTION con un IC95 que excluya el cero "+
				"(ΔMRR %+.4f, IC95 [%+.4f, %+.4f], p=%.3f, G%d/P%d/E%d). El arnés SÍ usa el embebedor —movió %d de %d consultas—, "+
				"pero con este fixture no separa la señal de POTION de un vector sin señal: un delta de ese tamaño entre dos "+
				"candidatos tampoco se puede leer como diferencia", nombre, fx, hib, f.Delta, f.ICLo, f.ICHi, f.P,
				f.Gana, f.Pierde, f.Empata, movidas, n)
		}
	}
	if vistas == 0 {
		t.Errorf("%s: no encontré la fila de MRR híbrido en %s para controlar el sabotaje", nombre, fx)
	}
}

// exigirLoopback rechaza un Ollama que no esté en este equipo. El corpus real es la memoria de una
// persona y no sale de la máquina, y la URL la escribe quien corre: la regla va en el código.
// El error no repite la URL: es el valor de una variable de entorno.
func exigirLoopback(crudo string) error {
	u, err := url.Parse(crudo)
	if err != nil {
		return fmt.Errorf("MUSUBI_OLLAMA_URL no es una URL válida")
	}
	host := u.Hostname()
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("MUSUBI_OLLAMA_URL tiene que apuntar a este equipo (loopback): el corpus real no sale de la máquina")
}

// ollamaPS lee los modelos que Ollama tiene cargados, con su memoria.
func ollamaPS(ctx context.Context, base string) ([]modeloCargado, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/api/ps", nil)
	if err != nil {
		return nil, err
	}
	// no habla con el cerebro: consulta el Ollama local (loopback) para ver qué modelos tiene cargados.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("/api/ps devolvió %d", resp.StatusCode)
	}
	var out struct {
		Models []modeloCargado `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Models, nil
}

// leerMeminfoMB devuelve un campo de /proc/meminfo en MB, o -1 si no se pudo leer (no es Linux).
func leerMeminfoMB(campo string) int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return -1
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		partes := strings.Fields(sc.Text())
		if len(partes) >= 2 && partes[0] == campo+":" {
			if kb, err := strconv.Atoi(partes[1]); err == nil {
				return kb / 1024
			}
		}
	}
	return -1
}

// swapUsadoMB es el swap en uso de la máquina (SwapTotal − SwapFree) en MB, o -1 si no se pudo leer.
func swapUsadoMB() int {
	total, libre := leerMeminfoMB("SwapTotal"), leerMeminfoMB("SwapFree")
	if total < 0 || libre < 0 {
		return -1
	}
	return total - libre
}

// leerStatusMB devuelve un campo de /proc/self/status (VmRSS, VmHWM…) en MB, o -1 si no se pudo.
func leerStatusMB(campo string) int {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1
	}
	for _, l := range strings.Split(string(b), "\n") {
		partes := strings.Fields(l)
		if len(partes) >= 2 && partes[0] == campo+":" {
			if kb, err := strconv.Atoi(partes[1]); err == nil {
				return kb / 1024
			}
		}
	}
	return -1
}

// modeloDeCPU es la primera «model name» de /proc/cpuinfo, o vacío.
func modeloDeCPU() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(l, ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func milis(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// costoLegible resume en una línea lo que costó un embebedor.
func costoLegible(m medicionEmbebedor) string {
	var b strings.Builder
	fmt.Fprintf(&b, "COSTO %s (dim %d): construir %.0f ms · primera llamada %.0f ms", m.Nombre, m.Dim, m.ConstruirMs, m.PrimeraMs)
	if m.Modelo != "" {
		if m.EstabaCargado {
			b.WriteString(" (el modelo YA estaba cargado: no es en frío)")
		} else {
			b.WriteString(" (en frío: incluye cargar el modelo)")
		}
	}
	fx := make([]string, 0, len(m.Reembebido))
	for k := range m.Reembebido {
		fx = append(fx, k)
	}
	sort.Strings(fx)
	for _, k := range fx {
		r := m.Reembebido[k]
		fmt.Fprintf(&b, " · re-embeber %s: %d docs en %.1f s (%.1f ms/doc)", k, r.Embebidos, r.Segundos, 1000*r.Segundos/float64(max(1, r.Embebidos)))
	}
	l := m.Lotes
	fmt.Fprintf(&b, " · lotes: %d llamadas, %.2f ms por 1000 caracteres", l.Llamadas, 1000*l.TotalMs/float64(max(1, l.Caracteres)))
	fmt.Fprintf(&b, " · doc de a uno: media %.0f / p50 %.0f / p95 %.0f ms (n=%d)", m.DocUnitario.MediaMs, m.DocUnitario.P50Ms, m.DocUnitario.P95Ms, m.DocUnitario.Llamadas)
	fmt.Fprintf(&b, " · consulta: media %.0f / p50 %.0f / p95 %.0f ms (n=%d)", m.Consultas.MediaMs, m.Consultas.P50Ms, m.Consultas.P95Ms, m.Consultas.Llamadas)
	for _, p := range m.OllamaPS {
		fmt.Fprintf(&b, " · ollama ps: %s %d MB (VRAM %d MB, contexto %d)", p.Nombre, p.Size>>20, p.SizeVRAM>>20, p.Contexto)
	}
	fmt.Fprintf(&b, " · MemAvailable antes %d MB, cargado %d MB · RSS del proceso antes %d MB, cargado %d MB",
		m.MemAntesMB, m.MemCargadoMB, m.RSSAntesMB, m.RSSCargadoMB)
	fmt.Fprintf(&b, " · swap de la máquina antes %d MB, cargado %d MB · VmSwap del proceso cargado %d MB",
		m.SwapAntesMB, m.SwapCargadoMB, m.VmSwapCargadoMB)
	return b.String()
}

// tablaLegible arma una fila por variante, fixture, config y base, con el delta de cada métrica
// contra esa base. ↑/↓ sólo cuando el IC95 no toca el cero; ≈ si lo toca.
func tablaLegible(filas []filaPareada) string {
	type clave struct{ v, f, c, contra string }
	orden := []clave{}
	por := map[clave][]filaPareada{}
	for _, f := range filas {
		k := clave{f.Variante, f.Fixture, f.Config, f.Contra}
		if _, ok := por[k]; !ok {
			orden = append(orden, k)
		}
		por[k] = append(por[k], f)
	}
	var b strings.Builder
	for _, k := range orden {
		fmt.Fprintf(&b, "%-20s %-7s %-13s vs %-8s", k.v, k.f, k.c, k.contra)
		for _, f := range por[k] {
			flecha := "≈"
			switch {
			case f.ICLo > 0:
				flecha = "↑"
			case f.ICHi < 0:
				flecha = "↓"
			}
			fmt.Fprintf(&b, " | %s %.3f vs %.3f Δ%+.3f [%+.3f,%+.3f] p=%.3f %s G%d/P%d/E%d",
				f.Metrica, f.Valor, f.Base, f.Delta, f.ICLo, f.ICHi, f.P, flecha, f.Gana, f.Pierde, f.Empata)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// espiaDeTextos es un Provider que anota cada texto que le piden embeber y devuelve su hashEmbed,
// para que el recall tenga vectores de verdad con los que trabajar.
type espiaDeTextos struct {
	mu     sync.Mutex
	vistos []string
}

func (e *espiaDeTextos) Embed(_ context.Context, texto string) ([]float32, error) {
	e.mu.Lock()
	e.vistos = append(e.vistos, texto)
	e.mu.Unlock()
	return hashEmbed(texto)
}
func (e *espiaDeTextos) Dimensions() int { return 64 }
func (e *espiaDeTextos) Name() string    { return "espia" }

// tomar devuelve lo visto hasta ahora y empieza de cero.
func (e *espiaDeTextos) tomar() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	v := e.vistos
	e.vistos = nil
	return v
}

// TestLosPrefijosVanDondeCorresponden fija lo que hace justa la comparación de embebedores: el
// prefijo de DOCUMENTO llega a cada documento del backfill y a ningún texto de consulta, y el de
// CONSULTA llega a cada consulta y a ningún documento. Si se cruzaran, un modelo como Harrier se
// mediría con la instrucción en el lado equivocado y el número no diría nada de él.
//
// Y que cada consulta se embeba UNA vez aunque Evaluate la corra por cada config: el cronómetro
// tiene que medir embeber, no leer el memo.
//
// Sabotaje: embeber la consulta sin su prefijo.
// arnes: archivo="internal/recalleval/embebedores.go"
// arnes: de="v, err := prov.Embed(ctx, prefijo+texto)"
// arnes: a="v, err := prov.Embed(ctx, texto)"
//
// Sabotaje: mandar el lote de documentos sin su prefijo.
// arnes: archivo="internal/recalleval/embebedores.go"
// arnes: de="\t\t\tcon[i] = prefijo + x\n"
// arnes: a="\t\t\tcon[i] = x\n"
func TestLosPrefijosVanDondeCorresponden(t *testing.T) {
	const prefDoc, prefConsulta = "DOC» ", "CONSULTA» "
	ctx := context.Background()
	fx := loadGolden(t)
	espia := &espiaDeTextos{}

	crono := &Cronometro{}
	eng, rb, err := SembrarYReembeber(memtest.DirSembrado(t), fx, LoteDeDocumentos(ctx, espia, prefDoc, crono))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	if rb.Sembrados == 0 || rb.Embebidos != rb.Sembrados {
		t.Fatalf("el backfill embebió %d de %d sembrados", rb.Embebidos, rb.Sembrados)
	}
	if r := crono.Resumen(); r.Textos != rb.Embebidos || r.Llamadas == 0 {
		t.Errorf("el cronómetro de lotes vio %d textos en %d llamadas; el backfill embebió %d", r.Textos, r.Llamadas, rb.Embebidos)
	}
	docsVistos := espia.tomar()

	q := EmbedDeConsultas(ctx, espia, prefConsulta, nil)
	for _, cfg := range []Config{ConfigHibrida(), hibridaSinPiso()} {
		if _, err := Evaluate(ctx, eng, fx, cfg, q, []int{10}); err != nil {
			t.Fatal(err)
		}
	}
	consultasVistas := espia.tomar()

	vistos := map[string]int{}
	for _, v := range docsVistos {
		if !strings.HasPrefix(v, prefDoc) || strings.HasPrefix(v, prefConsulta) {
			t.Errorf("un texto del backfill llegó sin el prefijo de documento (o con el de consulta): %.40q", v)
		}
		vistos[v]++
	}
	for _, d := range fx.Docs {
		if vistos[prefDoc+d.Content] == 0 {
			t.Errorf("el documento %s no llegó al embebedor con su prefijo de documento", d.ID)
		}
	}
	vistas := map[string]int{}
	for _, v := range consultasVistas {
		if !strings.HasPrefix(v, prefConsulta) || strings.HasPrefix(v, prefDoc) {
			t.Errorf("una consulta llegó sin el prefijo de consulta (o con el de documento): %.40q", v)
		}
		vistas[v]++
	}
	for _, qq := range fx.Queries {
		if len(qq.Relevant) == 0 {
			continue // Evaluate no corre las consultas sin relevantes
		}
		switch n := vistas[prefConsulta+qq.Text]; {
		case n == 0:
			t.Errorf("la consulta %s no llegó al embebedor con su prefijo de consulta", qq.ID)
		case n > 1:
			t.Errorf("la consulta %s se embebió %d veces con dos configs: el memo no está memoizando", qq.ID, n)
		}
	}
}
