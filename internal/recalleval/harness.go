package recalleval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"musubi/internal/logx"
	"os"
	"sort"
	"strings"

	"musubi/internal/cognition"
	"musubi/internal/memory"
)

// Doc es un documento del corpus de evaluación: una observación con id estable.
type Doc struct {
	ID      string `json:"id"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

// Query es una consulta etiquetada: el texto que busca el agente y el conjunto de docs
// que un humano considera RELEVANTES (la verdad de referencia contra la que se mide).
type Query struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	Relevant []string `json:"relevant"`
	// Note documenta POR QUÉ estos docs son relevantes (p. ej. "hueco de vocabulario
	// deploy↔despliegue": el léxico no debería encontrarlo, la semántica sí). Solo
	// informativo; no afecta el cálculo.
	Note string `json:"note,omitempty"`
}

// Fixture es el corpus + queries etiquetadas. Vive versionado en testdata/ para que la
// evaluación sea reproducible y revisable en el diff.
type Fixture struct {
	Docs    []Doc   `json:"docs"`
	Queries []Query `json:"queries"`
	// Relaciones son las aristas del grafo de observaciones (las "sinapsis" que DetectRelations
	// va tejiendo). Sin ellas la QUINTA SEÑAL RRF —la centralidad de grafo— es un NO-OP en toda
	// medición: buildObsGraph carga un grafo vacío, graphRank sale vacío, y una config con
	// GraphCentrality:true queda bit-idéntica a una con false.
	//
	// O sea que el banco defendía una señal que nunca ejercitaba. Peor: cualquier cambio que
	// rompiera la centralidad habría pasado el gate en verde, porque el gate no la tocaba.
	Relaciones []Relacion `json:"relaciones,omitempty"`
}

// Relacion es una arista entre dos observaciones del fixture. Es no dirigida a los efectos de la
// centralidad (buildObsGraph agrega las dos direcciones), pero se guarda con su origen y destino
// porque así vive en la base.
type Relacion struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// EmbedFunc genera el vector de un texto (el StaticProvider real, o uno sintético en
// tests). nil ⇒ evaluación 100% léxica (los docs se siembran sin vector).
type EmbedFunc func(text string) ([]float32, error)

// evalSeedCreatedAt es el created_at CONSTANTE con el que SeedEngine siembra TODOS los docs, para
// que la evaluación sea REPRODUCIBLE. El created_at real (datetime('now') al guardar) haría que la
// señal de recencia dependa del instante del seed: si el loop cruza un borde de segundo, unos docs
// quedan "más nuevos" y el ranking cambia entre corridas (el flake que veía CI en Windows). Con un
// valor fijo, todos los docs son equidistantes de `now` ⇒ recencia/frecuencia son un término
// CONSTANTE que no altera el orden relativo, y el ranking queda determinado sólo por la señal de
// recuperación (léxico + co-ocurrencia + grafo + importancia). Fecha absoluta a propósito (no
// relativa a now): así el factor de edad es idéntico para todos, corra el test el día que corra.
const evalSeedCreatedAt = "2020-01-01 00:00:00"

// ModeloDelBanco es la procedencia (embeddings.model_id) con la que SeedEngine estampa los vectores
// cuando siembra con un embebedor. Va con nombre, como la estampa el backfill de producción, y no
// vacía: un motor sin embebedor (vectorModelID "", el hook de hoy) no tiene que verlos. Con ” los
// veía, y el banco medía MMR donde el hook no lo corre (ver Config.SinEmbebedor).
const ModeloDelBanco = "recalleval:banco"

// Config es una variante de recall a evaluar. UseVector activa el recall híbrido
// (rellena QueryVector con embed(query)); requiere un EmbedFunc no-nil en Run.
type Config struct {
	Name      string
	Opts      memory.RecallOptions
	UseVector bool
	// Juez, si no es nil, somete los primeros JuezTopK resultados al JUEZ DE PERTINENCIA antes de
	// medir — el mismo camino que corre en producción cuando `cognition.read_time_rerank` está
	// encendido. Nil ⇒ el brazo no existe y el resultado es bit-idéntico al recall model-free.
	//
	// EL JUEZ ES EL DE VERDAD, no una copia: se llama a cognition.Rerank, que es la misma unidad que
	// usa el servidor. Reimplementar acá el prompt haría que este banco midiera una IMITACIÓN, y un
	// número con aspecto de autoridad sobre algo que no corre en producción es peor que no medir.
	Juez cognition.Provider
	// PoolDelTurno, si es true, RESPETA el CandidatePool de Opts en vez de subirlo al corpus entero.
	// Es el modo que corre el ranker del hook: el hook rankea 50 candidatos, no miles, y un banco con
	// pool = corpus mide otra cosa. Pesa sobre todo en el brazo híbrido, porque augmentWithVectorPool
	// usa ese mismo límite: con pool = corpus el vector trae hasta 3.155 vecinos, y en el hook trae
	// 50. Las métricas @k se miden entonces DENTRO de lo que ese pool deja entrar: un relevante que
	// no entró al pool cuenta como no encontrado, igual que en el turno. En false, el histórico:
	// pool = corpus, y el recorte lo hacen las @k.
	PoolDelTurno bool
	// SinEmbebedor modela el PROCESO del brazo, no sus opciones: un proceso que no construyó
	// embebedor deja el motor sin procedencia de vectores (vectorModelID ""), y ese motor no ve los
	// vectores estampados con nombre. Es el hook de hoy. runTurn no construye el embebedor cuando la
	// tabla estática está presente (embedderCaroDeConstruir, cmd/musubi/embed.go), así que nunca llama
	// SetVectorModelID, y vectorsFor filtra `model_id = ''`, que en una base real no tiene ni un vector
	// (los estampa el backfill, con el nombre del modelo). Resultado: MMR queda inerte en cada turno
	// aunque MMRLambda valga 0,75.
	//
	// POR QUÉ NO ALCANZA CON COPIAR LAS OPCIONES. SeedEngine estampaba los vectores con model_id ''
	// y el motor del banco los leía con '', así que un brazo con las opciones del hook diversificaba
	// y el hook no. Medido sobre la base de davantis-1: el léxico del banco daba 0,357 / 0,211 (MRR /
	// R@10) y el hook, 0,395 / 0,355. Ahora SeedEngine estampa con ModeloDelBanco, como el backfill,
	// y rankedIDs corre cada recall de este brazo con el motor en "" y lo devuelve como estaba.
	//
	// Contradice a UseVector (sin embebedor no hay con qué embeber la consulta): rankedIDs lo rechaza.
	SinEmbebedor bool
	// JuezTopK es cuántos resultados del tope ven al juez. 0 ⇒ cognition.DefaultTopK, la MISMA
	// constante que aplica el servidor — no una copia: si el banco juzgara más candidatos que
	// producción, mediría una configuración que nadie va a correr.
	JuezTopK int
	// CorregirTipeo pasa la consulta por el CORRECTOR DE TIPEO del motor (CorregirConsulta) antes de
	// embeberla y de buscarla, que es lo que hacen el hook y las tools con
	// memory.recall_typo_correction encendido. Es un eje del experimento: un brazo lo prende o lo
	// apaga para medir el corrector contra el mismo ranker sin él.
	//
	// EL ALCANCE ES EL DEL HOOK: memory.RecallOptions.AlcanceDelCorrector de Opts, la misma regla que
	// corre buildTurnRecall. Sin tope es el filtro duro del recall del brazo; con tope («aparte»), el
	// vocabulario de todo el acervo. Y el juez, si hay, ve la consulta como llegó, igual que en
	// musubi_recall: juzga pertinencia contra lo que la persona preguntó.
	//
	// Un motor sin corrector no puede correr este brazo, y rankedIDs lo corta: seguir sin corregir
	// mediría el ranker sin corrector bajo el nombre del brazo que corrige.
	CorregirTipeo bool
}

// Scores son las métricas agregadas (promedio sobre queries con ≥1 relevante) de una
// configuración. RecallAtK y NDCGAtK están indexados por k.
type Scores struct {
	Config    string          `json:"config"`
	Queries   int             `json:"queries"`
	MRR       float64         `json:"mrr"`
	RecallAtK map[int]float64 `json:"recall_at_k"`
	NDCGAtK   map[int]float64 `json:"ndcg_at_k"`
	// PorConsulta guarda el resultado de CADA consulta, no sólo el promedio.
	//
	// POR QUÉ HACE FALTA: dos configuraciones pueden tener el mismo MRR promedio y haber cambiado
	// de lugar la mitad del corpus. El promedio no distingue "mejoró parejo" de "mejoró mucho en
	// tres consultas y empeoró en veinte", y esas dos cosas se deciden distinto. Sin el detalle por
	// consulta no se puede comparar de a pares, que es la única forma de ver si un cambio mueve el
	// sistema o mueve el ruido.
	PorConsulta map[string]MetricasConsulta `json:"por_consulta,omitempty"`
}

// MetricasConsulta es el resultado de UNA consulta bajo UNA configuración.
type MetricasConsulta struct {
	RR        float64         `json:"rr"`
	RecallAtK map[int]float64 `json:"recall_at_k"`
	NDCGAtK   map[int]float64 `json:"ndcg_at_k"`
}

// LoadFixture lee un fixture JSON del disco.
func LoadFixture(path string) (*Fixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fx Fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		return nil, fmt.Errorf("fixture %s: %w", path, err)
	}
	if len(fx.Docs) == 0 || len(fx.Queries) == 0 {
		return nil, fmt.Errorf("fixture %s: necesita al menos 1 doc y 1 query", path)
	}
	return &fx, nil
}

// SeedEngine crea un motor de memoria en dir y guarda todos los docs del fixture. Si
// embed no es nil, cada doc lleva su embedding (activa la señal vectorial) estampado con
// ModeloDelBanco, y el motor queda leyendo con esa procedencia, como un proceso con embebedor;
// si es nil, se siembra 100% léxico. El caller es dueño del engine (debe Close()).
func SeedEngine(dir string, fx *Fixture, embed EmbedFunc) (*memory.DbEngine, error) {
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		return nil, err
	}
	if embed != nil {
		eng.SetVectorModelID(ModeloDelBanco)
	}
	var omitidos []string
	for _, d := range fx.Docs {
		var vec []float32
		if embed != nil {
			vec, err = embed(d.Content)
			if err != nil {
				eng.Close()
				return nil, fmt.Errorf("embed doc %s: %w", d.ID, err)
			}
		}
		if err := eng.SaveObservation(d.ID, d.Topic, d.Content, vec); err != nil {
			// UNA GUARDA DE ESCRITURA NO PUEDE VOLVER INSEMBRABLE AL HISTÓRICO. saveObservation
			// rechaza el contenido que se comió el cierre de su propia llamada (`</content>` con el
			// sobre adentro), y hace bien: existe para que no entre MÁS de eso. Pero acá no está
			// entrando nada nuevo — se está re-sembrando en una base descartable lo que YA vive en
			// la memoria real. Medido: 66 observaciones del acervo están en ese estado y son
			// irreparables (reescribir el texto cambiaría el content_hash, que es la clave del
			// dedup y viaja en el sync), así que abortar acá dejaría el fixture por `content`
			// permanentemente inconstruible.
			//
			// Se saltean y SE CUENTAN. El conteo va al log al final: un banco que recorta su corpus
			// en silencio informa "medí todo" cuando midió menos, que es la falla que este repo ya
			// tiene nombrada. Cualquier OTRO error sí aborta: un disco lleno no es un dato sucio.
			if errors.Is(err, memory.ErrPayloadInvalido) {
				omitidos = append(omitidos, d.ID)
				continue
			}
			eng.Close()
			return nil, fmt.Errorf("guardar doc %s: %w", d.ID, err)
		}
		// Seeding DETERMINISTA en el tiempo: fijar el created_at a un valor constante para que la
		// recencia no dependa del instante del guardado (ver evalSeedCreatedAt). Sin esto la
		// evaluación flakea cuando el loop de seed cruza un borde de segundo.
		if err := eng.SetObservationCreatedAt(d.ID, evalSeedCreatedAt); err != nil {
			eng.Close()
			return nil, fmt.Errorf("fijar created_at doc %s: %w", d.ID, err)
		}
	}
	// LAS ARISTAS, DESPUÉS DE LOS DOCS. Van al final a propósito: observation_relations referencia
	// observations, y sembrar una arista hacia un doc que el motor rechazó (ver omitidos) dejaría
	// una relación huérfana — justo lo que el check orphan_relations del doctor existe para
	// encontrar. Se saltean las aristas cuyas puntas no hayan entrado.
	sembrados := make(map[string]bool, len(fx.Docs))
	for _, d := range fx.Docs {
		sembrados[d.ID] = true
	}
	for _, o := range omitidos {
		delete(sembrados, o)
	}
	var aristasOmitidas int
	for i, r := range fx.Relaciones {
		if !sembrados[r.Source] || !sembrados[r.Target] {
			aristasOmitidas++
			continue
		}
		if _, err := eng.UpsertObsRelation(memory.ObsRelation{
			ID:       fmt.Sprintf("rel-eval-%d", i),
			SourceID: r.Source,
			TargetID: r.Target,
			Relation: "related",
			Status:   "resolved",
		}); err != nil {
			eng.Close()
			return nil, fmt.Errorf("sembrar relación %s->%s: %w", r.Source, r.Target, err)
		}
	}
	if aristasOmitidas > 0 {
		logx.Warn("el banco sembró menos aristas de las que pidió: alguna punta no entró al corpus",
			"omitidas", aristasOmitidas, "de", len(fx.Relaciones))
	}

	if len(omitidos) > 0 {
		logx.Warn("el banco sembró MENOS corpus del que pidió: hay observaciones que el motor no acepta re-guardar",
			"omitidas", len(omitidos), "de", len(fx.Docs), "primera", omitidos[0],
			"motivo", "content con el sobre de su propia llamada adentro (irreparable: reescribirlo cambia el content_hash)")
	}
	return eng, nil
}

// recuperador es lo único que Evaluate y rankedIDs necesitan del motor (*memory.DbEngine lo
// cumple). Existe para que una prueba pueda ver las opciones con las que el banco llama a Recall
// (TestElBancoCorreElPoolDelTurno) sin sembrar un corpus del tamaño del pool.
type recuperador interface {
	Recall(ctx context.Context, query string, opts memory.RecallOptions) (memory.RecallResult, error)
}

// motorConProcedencia es el estado del motor que un brazo SinEmbebedor necesita poner como el del
// hook: con qué procedencia lee los vectores. *memory.DbEngine lo cumple.
type motorConProcedencia interface {
	VectorModelID() string
	SetVectorModelID(string)
}

// motorQueCorrige es el corrector de tipeo que un brazo CorregirTipeo necesita del motor.
// *memory.DbEngine lo cumple.
type motorQueCorrige interface {
	CorregirConsulta(ctx context.Context, q string, alcance memory.ProjectScope) (string, []memory.Correccion)
}

// rankedIDs corre un recall y devuelve solo los ids en orden de score (mejor primero).
// Fuerza un presupuesto de tokens enorme para que el ranking NO se recorte por presupuesto: el
// harness mide CALIDAD DE ORDEN, no empaquetado. Salvo en el modo PoolDelTurno, también sube el
// pool al corpus entero. NoBump evita que un recall contamine las stats de acceso del siguiente
// (reproducibilidad). En un brazo SinEmbebedor corre el recall con el motor sin procedencia de
// vectores y lo devuelve como estaba; en un brazo CorregirTipeo, embebe y busca la consulta corregida.
func rankedIDs(ctx context.Context, eng recuperador, query string, cfg Config, embed EmbedFunc, pool int) ([]string, error) {
	opts := cfg.Opts
	opts.NoBump = true
	opts.TokenBudget = 1 << 30
	if !cfg.PoolDelTurno && opts.CandidatePool < pool {
		opts.CandidatePool = pool
	}
	if cfg.SinEmbebedor {
		if cfg.UseVector {
			return nil, fmt.Errorf("config %q: UseVector y SinEmbebedor se contradicen, sin embebedor no hay con qué embeber la consulta", cfg.Name)
		}
		// Un motor que no deja fijar la procedencia no puede correr en el estado del hook, y seguir
		// igual mediría el motor con embebedor sin decirlo: se corta.
		m, ok := eng.(motorConProcedencia)
		if !ok {
			return nil, fmt.Errorf("config %q modela un motor sin embebedor y este motor (%T) no deja fijar la procedencia de sus vectores", cfg.Name, eng)
		}
		antes := m.VectorModelID()
		m.SetVectorModelID("")
		defer m.SetVectorModelID(antes)
	}
	// El corrector va ANTES de embeber, como en el hook: lo que se embebe y se busca es la consulta
	// corregida. query sigue siendo lo que tipeó la persona, y es lo que ve el juez.
	consulta := query
	if cfg.CorregirTipeo {
		c, ok := eng.(motorQueCorrige)
		if !ok {
			return nil, fmt.Errorf("config %q corrige el tipeo y este motor (%T) no tiene corrector", cfg.Name, eng)
		}
		consulta, _ = c.CorregirConsulta(ctx, query, opts.AlcanceDelCorrector())
	}
	if cfg.UseVector {
		if embed == nil {
			return nil, fmt.Errorf("config %q usa vector pero no se pasó EmbedFunc", cfg.Name)
		}
		vec, err := embed(consulta)
		if err != nil {
			return nil, fmt.Errorf("embed query %q: %w", consulta, err)
		}
		opts.QueryVector = vec
	}
	res, err := eng.Recall(ctx, consulta, opts)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(res.Items))
	for i, it := range res.Items {
		ids[i] = it.ID
	}
	if cfg.Juez == nil {
		return ids, nil
	}

	// EL BRAZO DEL JUEZ. Espeja lo que hace el servidor: somete sólo la cabeza del ranking y deja
	// la cola intacta. A diferencia de producción NO hay caché — una respuesta memoizada mediría el
	// caché en vez del juez— y el error NO se traga: en producción degradar en silencio protege al
	// usuario, pero acá un juez roto que devuelve el orden model-free daría «el juez no aporta
	// nada», que es una conclusión falsa con cara de medición.
	topK := cfg.JuezTopK
	if topK <= 0 {
		topK = cognition.DefaultTopK
	}
	n := len(ids)
	if n > topK {
		n = topK
	}
	if n < 2 {
		return ids, nil // igual que producción: con menos de 2 no hay nada que ordenar
	}
	cands := make([]cognition.Candidato, n)
	for i, it := range res.Items[:n] {
		cands[i] = cognition.Candidato{ID: it.ID, Gist: it.Gist}
	}
	orden, err := cognition.Rerank(ctx, cfg.Juez, query, cands)
	if err != nil {
		return nil, fmt.Errorf("config %q: el juez falló en la query %q: %w", cfg.Name, query, err)
	}
	return append(cognition.ReordenarIDs(ids[:n], orden), ids[n:]...), nil
}

// Evaluate corre todas las queries del fixture bajo una configuración y agrega las
// métricas en los k pedidos. Omite del promedio las queries sin relevantes.
func Evaluate(ctx context.Context, eng recuperador, fx *Fixture, cfg Config, embed EmbedFunc, ks []int) (Scores, error) {
	// pool = corpus entero: el recorte lo hacen las métricas @k, no el pool. En el modo PoolDelTurno
	// rankedIDs lo ignora y corre con el pool de las opciones, que es el del hook.
	pool := len(fx.Docs)
	recallByK := make(map[int][]float64, len(ks))
	ndcgByK := make(map[int][]float64, len(ks))
	porConsulta := make(map[string]MetricasConsulta, len(fx.Queries))
	var rr []float64
	for _, q := range fx.Queries {
		if len(q.Relevant) == 0 {
			continue
		}
		relevant := make(map[string]bool, len(q.Relevant))
		for _, id := range q.Relevant {
			relevant[id] = true
		}
		ranked, err := rankedIDs(ctx, eng, q.Text, cfg, embed, pool)
		if err != nil {
			return Scores{}, fmt.Errorf("query %s: %w", q.ID, err)
		}
		rr = append(rr, ReciprocalRank(ranked, relevant))
		mc := MetricasConsulta{
			RR:        ReciprocalRank(ranked, relevant),
			RecallAtK: make(map[int]float64, len(ks)),
			NDCGAtK:   make(map[int]float64, len(ks)),
		}
		for _, k := range ks {
			r := RecallAtK(ranked, relevant, k)
			n := NDCGAtK(ranked, relevant, k)
			recallByK[k] = append(recallByK[k], r)
			ndcgByK[k] = append(ndcgByK[k], n)
			mc.RecallAtK[k] = r
			mc.NDCGAtK[k] = n
		}
		porConsulta[q.ID] = mc
	}
	s := Scores{
		Config:      cfg.Name,
		Queries:     len(rr),
		MRR:         mean(rr),
		RecallAtK:   make(map[int]float64, len(ks)),
		NDCGAtK:     make(map[int]float64, len(ks)),
		PorConsulta: porConsulta,
	}
	for _, k := range ks {
		s.RecallAtK[k] = mean(recallByK[k])
		s.NDCGAtK[k] = mean(ndcgByK[k])
	}
	return s, nil
}

// Run es el orquestador: siembra un engine en dir con el fixture (embeddings si embed
// no es nil) y evalúa cada configuración sobre el MISMO corpus, devolviendo un Scores por
// config. Comparar léxico vs híbrido con el mismo embed aísla el aporte de la semántica.
func Run(ctx context.Context, dir string, fx *Fixture, embed EmbedFunc, configs []Config, ks []int) ([]Scores, error) {
	eng, err := SeedEngine(dir, fx, embed)
	if err != nil {
		return nil, err
	}
	defer eng.Close()
	out := make([]Scores, 0, len(configs))
	for _, cfg := range configs {
		s, err := Evaluate(ctx, eng, fx, cfg, embed, ks)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// FormatReport arma una tabla legible de los Scores (una fila por config), ordenada por
// los k ascendentes. Es la salida que un humano lee para decidir si la semántica gana.
func FormatReport(scores []Scores, ks []int) string {
	sorted := append([]int(nil), ks...)
	sort.Ints(sorted)
	var b strings.Builder
	fmt.Fprintf(&b, "%-22s %7s", "config", "MRR")
	for _, k := range sorted {
		fmt.Fprintf(&b, "  R@%-4d nDCG@%-2d", k, k)
	}
	b.WriteByte('\n')
	for _, s := range scores {
		fmt.Fprintf(&b, "%-22s %7.3f", s.Config, s.MRR)
		for _, k := range sorted {
			fmt.Fprintf(&b, "  %5.3f  %6.3f", s.RecallAtK[k], s.NDCGAtK[k])
		}
		b.WriteByte('\n')
	}
	return b.String()
}
