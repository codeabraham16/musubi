package publico

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/recalleval"
)

// LA MEDICIÓN DE LONGMEMEVAL, OPT-IN. No corre en CI ni en una suite normal: necesita el dataset
// (S pesa 278 MB y M 2,7 GB; se bajan aparte de Hugging Face) y, para los brazos híbridos, la tabla
// de POTION.
//
//	MUSUBI_LONGMEMEVAL       ruta a longmemeval_s.json o longmemeval_m.json, que comparten formato
//	                         (sin ella, la prueba se saltea)
//	MUSUBI_LONGMEMEVAL_N     tope de preguntas a LEER, en el orden del archivo (0 = todas). Es para
//	                         probar el camino: un prefijo del archivo no es una muestra, porque el
//	                         dataset viene agrupado por tipo de pregunta.
//	MUSUBI_LONGMEMEVAL_MODO  "user" (el del paper, por defecto) o "full" (con el asistente)
//	MUSUBI_LONGMEMEVAL_SOLO_BM25  "1": sólo la réplica del BM25 del paper, sin sembrar ninguna base.
//	                         Es para validar la réplica contra M, donde cada pregunta trae ~500
//	                         sesiones y sembrarlas es lo caro.
//	MUSUBI_LONGMEMEVAL_ABLACION  "1": suma los brazos exploratorios de brazosDeAblacion, que
//	                         apagan señales de la fusión RRF del léxico. No son configs de producción.
//	MUSUBI_POTION_DIR        tabla de POTION: agrega los brazos híbridos (embebedor local, sin red)
//	MUSUBI_LONGMEMEVAL_OUT   prefijo de salida: <prefijo>.json (agregados) y <prefijo>.jsonl (por
//	                         pregunta). Fuera del repo: son sesiones de chat de un dataset ajeno.
//
// DOS PROMEDIOS, PORQUE EL PAPER TIENE DOS. run_retrieval.py saca del promedio las abstenciones Y
// las preguntas sin evidencia del lado del usuario (líneas 396-402); print_retrieval_metrics.py,
// que imprime exactamente las cuatro columnas de la tabla (recall_all@5, ndcg_any@5, recall_all@10,
// ndcg_any@10), saca SÓLO las abstenciones. En una pregunta sin oro, recall_all vale 1 para
// cualquier recuperador (`all([])`) y el nDCG vale 0, así que los dos promedios dan números
// distintos con el mismo ranking. Cuál de los dos es la tabla no lo dice el paper: lo decide la
// réplica del BM25, que tiene que caer sobre uno.
func TestLongMemEval(t *testing.T) {
	ruta := os.Getenv("MUSUBI_LONGMEMEVAL")
	if ruta == "" {
		t.Skip("falta MUSUBI_LONGMEMEVAL: la medición contra LongMemEval es opt-in")
	}
	tope := 0
	if s := os.Getenv("MUSUBI_LONGMEMEVAL_N"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 {
			t.Fatalf("MUSUBI_LONGMEMEVAL_N=%q no es un entero ≥ 0", s)
		}
		tope = n
	}
	modo := ModoUsuario
	if s := os.Getenv("MUSUBI_LONGMEMEVAL_MODO"); s != "" {
		modo = Modo(s)
		if modo != ModoUsuario && modo != ModoCompleto {
			t.Fatalf("MUSUBI_LONGMEMEVAL_MODO=%q: los modos son %q y %q", s, ModoUsuario, ModoCompleto)
		}
	}

	cfgs := []recalleval.Config{recalleval.ConfigLexica(), recalleval.ConfigTurno()}
	soloBM25 := os.Getenv("MUSUBI_LONGMEMEVAL_SOLO_BM25") == "1"
	if soloBM25 {
		if os.Getenv("MUSUBI_POTION_DIR") != "" {
			t.Fatal("MUSUBI_LONGMEMEVAL_SOLO_BM25 y MUSUBI_POTION_DIR juntas se contradicen: los brazos híbridos son de Musubi")
		}
		cfgs = nil
	}
	var embed recalleval.EmbedFunc
	if dir := os.Getenv("MUSUBI_POTION_DIR"); dir != "" {
		prov, err := embedding.NewStaticProvider(dir)
		if err != nil {
			t.Fatalf("NewStaticProvider(%s): %v", dir, err)
		}
		embed = embebedorConCache(func(s string) ([]float32, error) { return prov.Embed(context.Background(), s) })
		cfgs = append(cfgs, recalleval.ConfigHibrida(), recalleval.ConfigProduccion())
	}
	if os.Getenv("MUSUBI_LONGMEMEVAL_ABLACION") == "1" {
		if soloBM25 {
			t.Fatal("MUSUBI_LONGMEMEVAL_ABLACION y MUSUBI_LONGMEMEVAL_SOLO_BM25 juntas se contradicen: la ablación es de Musubi")
		}
		cfgs = append(cfgs, brazosDeAblacion()...)
	}
	brazos := []string{"bm25-paper"}
	for _, c := range cfgs {
		brazos = append(brazos, c.Name)
	}

	var jsonl *bufio.Writer
	prefijo := os.Getenv("MUSUBI_LONGMEMEVAL_OUT")
	if prefijo != "" {
		f, err := os.Create(prefijo + ".jsonl")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		jsonl = bufio.NewWriter(f)
		defer jsonl.Flush()
	}

	f, err := os.Open(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	m := newMedicion(brazos)
	inicio := time.Now()
	errTope := errors.New("tope de preguntas")
	err = LeerLongMemEval(bufio.NewReaderSize(f, 1<<20), func(p Pregunta) error {
		if tope > 0 && m.leidas >= tope {
			return errTope
		}
		m.leidas++
		motivo := Exclusion(p)
		m.porMotivo[motivo]++
		if motivo == "abstencion" {
			return nil
		}
		c := CorpusDe(p, modo)
		if c.Desparejas {
			m.desparejas++
		}
		oroDocs := 0
		for _, e := range c.Etiquetas {
			if EsOro(e) {
				oroDocs++
			}
		}
		if motivo == "" && oroDocs == 0 {
			m.evaluadasSinOro++
		}

		rankings := map[string][]int{}
		// El BM25 del paper, sobre el mismo corpus y la misma consulta: `entry['question']` a secas.
		toks := make([][]string, len(c.Textos))
		for i, s := range c.Textos {
			toks[i] = Tokens(s)
		}
		punt := NuevoBM25Okapi(toks).Puntajes(Tokens(p.Question))
		rankings["bm25-paper"] = RankingBM25(punt)
		for _, k := range []int{5, 10} {
			if EmpateEnCorte(rankings["bm25-paper"], punt, k) {
				m.empatesBM25[k]++
			}
		}

		if len(cfgs) > 0 {
			r, omitidos, err := RankingsMusubi(context.Background(), p.Question, c, cfgs, embed)
			if err != nil {
				return fmt.Errorf("pregunta %s: %w", p.QuestionID, err)
			}
			if omitidos > 0 {
				m.omitidos += omitidos
				m.preguntasConOmitidos++
			}
			for k, v := range r {
				rankings[k] = v
			}
		}

		linea := map[string]any{"qid": p.QuestionID, "tipo": p.QuestionType, "motivo": motivo, "docs": len(c.IDs), "oro": oroDocs}
		porBrazo := map[string]map[string]float64{}
		for _, b := range brazos {
			mt := metricasDe(rankings[b], c)
			porBrazo[b] = mt
			m.sumar(b, p.QuestionType, motivo == "", mt)
		}
		linea["brazos"] = porBrazo
		if jsonl != nil {
			b, _ := json.Marshal(linea)
			jsonl.Write(append(b, '\n'))
		}
		if m.leidas%50 == 0 {
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			t.Logf("… %d preguntas leídas en %s (heap %d MiB)", m.leidas, time.Since(inicio).Round(time.Second), ms.HeapAlloc>>20)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errTope) {
		t.Fatal(err)
	}
	if jsonl != nil {
		// bufio guarda el primer error de escritura y lo devuelve acá: sin este chequeo, un disco
		// lleno dejaría un .jsonl trunco con un .json de agregados que parece completo.
		if err := jsonl.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	m.duracion = time.Since(inicio)
	m.dataset = filepath.Base(ruta)
	m.modo = string(modo)
	m.embebedor = embed != nil

	t.Log("\n" + m.informe())
	if prefijo != "" {
		b, err := json.MarshalIndent(m.resumen(), "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(prefijo+".json", b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestBrazosDeAblacion fija lo que hace legible a la ablación: cada brazo es ConfigLexica con
// otros pesos y NADA MÁS, y apaga exactamente las señales que dice su nombre. Si un brazo saliera
// de otra config, su distancia con el léxico mezclaría los pesos con esa otra diferencia.
//
// Sabotaje: un brazo de la ablación sale de otra config y ya no difiere del léxico sólo en los pesos.
// arnes: archivo="internal/recalleval/publico/medicion_test.go"
// arnes: de="\t\tc := recalleval.ConfigLexica()\n\t\tc.Name = nombre\n"
// arnes: a="\t\tc := recalleval.ConfigTurno()\n\t\tc.Name = nombre\n"
func TestBrazosDeAblacion(t *testing.T) {
	apagadas := map[string][]string{
		"abl-solo-lexico": {"Recencia", "Frecuencia", "Vector", "Grafo", "Coocurrencia", "Importancia"},
		"abl-sin-planas":  {"Recencia", "Frecuencia", "Importancia"},
		"abl-sin-grafo":   {"Grafo"},
		"abl-sin-cooc":    {"Coocurrencia"},
	}
	brazos := brazosDeAblacion()
	if len(brazos) != len(apagadas) {
		t.Fatalf("hay %d brazos de ablación y %d esperados", len(brazos), len(apagadas))
	}
	lexica := recalleval.ConfigLexica()
	for _, c := range brazos {
		campos, ok := apagadas[c.Name]
		if !ok {
			t.Errorf("brazo %q sin expectativa", c.Name)
			continue
		}
		if c.Opts.Pesos == nil {
			t.Errorf("%s: sin pesos, correría con los uniformes y mediría el léxico", c.Name)
			continue
		}
		esperado := memory.PesosUniformes()
		ve := reflect.ValueOf(&esperado).Elem()
		for _, campo := range campos {
			ve.FieldByName(campo).SetFloat(0)
		}
		if *c.Opts.Pesos != esperado {
			t.Errorf("%s: pesos %+v, se esperaban %+v", c.Name, *c.Opts.Pesos, esperado)
		}
		resto := c
		resto.Name = lexica.Name
		resto.Opts.Pesos = lexica.Opts.Pesos
		if !reflect.DeepEqual(resto, lexica) {
			t.Errorf("%s difiere de ConfigLexica en algo más que el nombre y los pesos:\n%+v\n%+v", c.Name, resto, lexica)
		}
	}
}

// brazosDeAblacion son los brazos EXPLORATORIOS de MUSUBI_LONGMEMEVAL_ABLACION: no son configs de
// producción. El léxico de Musubi no es un BM25 solo: es una fusión RRF de siete señales con peso 1
// (memory.PesosUniformes). Cada brazo es ConfigLexica con algunas de esas señales en peso 0, y
// NADA MÁS distinto —lo fija TestBrazosDeAblacion—: si difiriera en otra opción, la distancia con
// el léxico ya no se le podría atribuir a las señales apagadas.
//
//   - abl-solo-lexico: sólo el match léxico, sin fusión.
//   - abl-sin-planas: sin las tres señales que en una base sembrada por el banco no miran la
//     consulta (recencia con created_at constante, frecuencia con NoBump, importancia sin tocar).
//   - abl-sin-grafo y abl-sin-cooc: sin la centralidad de grafo y sin la expansión por
//     co-ocurrencia, de a una, para saber cuál de las dos mueve el resultado.
func brazosDeAblacion() []recalleval.Config {
	brazo := func(nombre string, apagar func(*memory.PesosRRF)) recalleval.Config {
		c := recalleval.ConfigLexica()
		c.Name = nombre
		p := memory.PesosUniformes()
		apagar(&p)
		c.Opts.Pesos = &p
		return c
	}
	return []recalleval.Config{
		brazo("abl-solo-lexico", func(p *memory.PesosRRF) { *p = memory.PesosRRF{Lexico: 1} }),
		brazo("abl-sin-planas", func(p *memory.PesosRRF) { p.Recencia, p.Frecuencia, p.Importancia = 0, 0, 0 }),
		brazo("abl-sin-grafo", func(p *memory.PesosRRF) { p.Grafo = 0 }),
		brazo("abl-sin-cooc", func(p *memory.PesosRRF) { p.Coocurrencia = 0 }),
	}
}

// embebedorConCache memoiza por el hash del texto: los pajares salen de un fondo común de sesiones
// y se repiten entre preguntas. Contado sobre los `haystack_session_ids` el 2026-09-30: en S, 19.829
// ids distintos en 25.112 lugares (se repite una de cada cinco); en M, 52.476 en 250.948 (sólo una
// de cada cinco es nueva). El vector es función pura del texto (POTION es estático), así que
// memoizar no cambia ningún número.
func embebedorConCache(embed func(string) ([]float32, error)) recalleval.EmbedFunc {
	var mu sync.Mutex
	cache := map[[32]byte][]float32{}
	return func(s string) ([]float32, error) {
		h := sha256.Sum256([]byte(s))
		mu.Lock()
		v, ok := cache[h]
		mu.Unlock()
		if ok {
			return v, nil
		}
		v, err := embed(s)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		cache[h] = v
		mu.Unlock()
		return v, nil
	}
}

var ksMedidos = []int{1, 3, 5, 10, 30, 50}

// metricasDe calcula, para un ranking, las métricas del paper y las de recalleval.
func metricasDe(ranking []int, c Corpus) map[string]float64 {
	mt := map[string]float64{"devueltos": float64(len(ranking))}
	ids := make([]string, len(ranking))
	for i, pos := range ranking {
		ids[i] = c.IDs[pos]
	}
	rel := map[string]bool{}
	for i, e := range c.Etiquetas {
		if EsOro(e) {
			rel[c.IDs[i]] = true
		}
	}
	for _, k := range ksMedidos {
		anyK, allK := RecallAnyAll(ranking, c.Etiquetas, k)
		mt[fmt.Sprintf("recall_any@%d", k)] = anyK
		mt[fmt.Sprintf("recall_all@%d", k)] = allK
		mt[fmt.Sprintf("ndcg_any@%d", k)] = NDCGPaper(ranking, c.Etiquetas, k)
		mt[fmt.Sprintf("rv_recall@%d", k)] = recalleval.RecallAtK(ids, rel, k)
		mt[fmt.Sprintf("rv_ndcg@%d", k)] = recalleval.NDCGAtK(ids, rel, k)
	}
	mt["rv_mrr"] = recalleval.ReciprocalRank(ids, rel)
	return mt
}

type acumulador struct {
	N     int                `json:"n"`
	Sumas map[string]float64 `json:"-"`
}

func (a *acumulador) sumar(mt map[string]float64) {
	if a.Sumas == nil {
		a.Sumas = map[string]float64{}
	}
	a.N++
	for k, v := range mt {
		a.Sumas[k] += v
	}
}

func (a *acumulador) media(k string) float64 {
	if a == nil || a.N == 0 {
		return 0
	}
	return a.Sumas[k] / float64(a.N)
}

type medicion struct {
	brazos               []string
	leidas               int
	porMotivo            map[string]int
	desparejas           int
	evaluadasSinOro      int
	omitidos             int
	preguntasConOmitidos int
	empatesBM25          map[int]int
	// sinAbs es el promedio de print_retrieval_metrics.py (fuera sólo las abstenciones); estricto,
	// el de run_retrieval.py (fuera también las que no tienen evidencia del usuario).
	sinAbs, estricto map[string]*acumulador
	porTipo          map[string]map[string]*acumulador // tipo → brazo → acumulador (estricto)
	duracion         time.Duration
	dataset          string
	modo             string
	embebedor        bool
}

func newMedicion(brazos []string) *medicion {
	m := &medicion{brazos: brazos, porMotivo: map[string]int{}, empatesBM25: map[int]int{},
		sinAbs: map[string]*acumulador{}, estricto: map[string]*acumulador{}, porTipo: map[string]map[string]*acumulador{}}
	for _, b := range brazos {
		m.sinAbs[b], m.estricto[b] = &acumulador{}, &acumulador{}
	}
	return m
}

func (m *medicion) sumar(brazo, tipo string, estricta bool, mt map[string]float64) {
	m.sinAbs[brazo].sumar(mt)
	if !estricta {
		return
	}
	m.estricto[brazo].sumar(mt)
	if m.porTipo[tipo] == nil {
		m.porTipo[tipo] = map[string]*acumulador{}
	}
	if m.porTipo[tipo][brazo] == nil {
		m.porTipo[tipo][brazo] = &acumulador{}
	}
	m.porTipo[tipo][brazo].sumar(mt)
}

var columnasPaper = []string{"recall_all@5", "ndcg_any@5", "recall_all@10", "ndcg_any@10"}
var columnasExtra = []string{"recall_any@5", "recall_any@10", "recall_all@30", "rv_recall@10", "rv_ndcg@10", "rv_mrr", "devueltos"}

func (m *medicion) informe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s · modo %s · embebedor local %v · %s\n", m.dataset, m.modo, m.embebedor, m.duracion.Round(time.Second))
	fmt.Fprintf(&b, "leídas %d · excluidas por abstención %d · sin evidencia del usuario %d · evaluadas (estricto) %d\n",
		m.leidas, m.porMotivo["abstencion"], m.porMotivo["sin_oro_de_usuario"], m.porMotivo[""])
	fmt.Fprintf(&b, "anomalías: evaluadas sin oro %d · pajares desparejos %d · docs que la base no aceptó %d (en %d preguntas) · empates BM25 en el corte @5 %d, @10 %d\n",
		m.evaluadasSinOro, m.desparejas, m.omitidos, m.preguntasConOmitidos, m.empatesBM25[5], m.empatesBM25[10])
	tabla := func(titulo string, acc map[string]*acumulador) {
		fmt.Fprintf(&b, "\n%s\n| brazo | n | %s | %s |\n", titulo, strings.Join(columnasPaper, " | "), strings.Join(columnasExtra, " | "))
		for _, br := range m.brazos {
			a := acc[br]
			fmt.Fprintf(&b, "| %s | %d |", br, a.N)
			for _, c := range append(append([]string{}, columnasPaper...), columnasExtra...) {
				fmt.Fprintf(&b, " %.4f |", a.media(c))
			}
			b.WriteString("\n")
		}
	}
	tabla("PROMEDIO ESTRICTO (run_retrieval.py: fuera abstenciones y sin evidencia del usuario)", m.estricto)
	tabla("PROMEDIO SIN ABSTENCIONES (print_retrieval_metrics.py)", m.sinAbs)

	tipos := make([]string, 0, len(m.porTipo))
	for t := range m.porTipo {
		tipos = append(tipos, t)
	}
	sort.Strings(tipos)
	fmt.Fprintf(&b, "\nPOR TIPO (estricto): recall_all@5 / recall_all@10 / ndcg_any@10\n| tipo | n | %s |\n", strings.Join(m.brazos, " | "))
	for _, tp := range tipos {
		n := 0
		if a := m.porTipo[tp][m.brazos[0]]; a != nil {
			n = a.N
		}
		fmt.Fprintf(&b, "| %s | %d |", tp, n)
		for _, br := range m.brazos {
			a := m.porTipo[tp][br]
			fmt.Fprintf(&b, " %.3f / %.3f / %.3f |", a.media("recall_all@5"), a.media("recall_all@10"), a.media("ndcg_any@10"))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (m *medicion) resumen() map[string]any {
	medias := func(acc map[string]*acumulador) map[string]any {
		out := map[string]any{}
		for _, br := range m.brazos {
			a := acc[br]
			if a == nil {
				continue
			}
			fila := map[string]float64{"n": float64(a.N)}
			for k := range a.Sumas {
				fila[k] = a.media(k)
			}
			out[br] = fila
		}
		return out
	}
	porTipo := map[string]any{}
	for tp, acc := range m.porTipo {
		porTipo[tp] = medias(acc)
	}
	return map[string]any{
		"dataset": m.dataset, "modo": m.modo, "embebedor_local": m.embebedor, "duracion_s": m.duracion.Seconds(),
		"leidas": m.leidas, "por_motivo": m.porMotivo, "evaluadas_sin_oro": m.evaluadasSinOro,
		"desparejas": m.desparejas, "docs_omitidos": m.omitidos, "preguntas_con_omitidos": m.preguntasConOmitidos,
		"empates_bm25_en_corte": m.empatesBM25,
		"estricto":              medias(m.estricto), "sin_abstenciones": medias(m.sinAbs), "por_tipo_estricto": porTipo,
	}
}
