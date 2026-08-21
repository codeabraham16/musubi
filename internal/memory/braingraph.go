package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// braingraph.go expone la memoria como un GRAFO NEURONAL para el dashboard-cerebro:
// las observaciones activas son NEURONAS y las observation_relations son SINAPSIS. Es
// read-only, model-free y de una sola pasada; deriva todo de SQLite sin LLM. La saliencia
// (para el cap top-N y el glow del render) se computa en Go —no en SQL— para no depender
// de funciones math de SQLite (exp/log) que el driver puede no traer.

// BrainNeuron es una observación activa vista como neurona del cerebro. domain es el
// prefijo temático (antes del primer '/'); heat = access_count; age_days/recency_days
// alimentan el tamaño y el glow del render.
type BrainNeuron struct {
	ID          string  `json:"id"`
	Topic       string  `json:"topic"`
	Domain      string  `json:"domain"`
	MemType     string  `json:"mem_type,omitempty"`
	Importance  float64 `json:"importance"`
	Heat        int     `json:"heat"`
	AgeDays     float64 `json:"age_days"`
	RecencyDays float64 `json:"recency_days"`
	Gist        string  `json:"gist,omitempty"`
	salience    float64 // interno: para ordenar y capar
}

// BrainSynapse es una relación semántica entre dos neuronas (observation_relations):
// su tipo (related/compatible/scoped/conflicts_with/supersedes/not_conflict), su
// confianza y su estado (pending/resolved).
type BrainSynapse struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status,omitempty"`
}

// BrainGraph es el grafo neuronal para el render: las neuronas incluidas (top-N por
// saliencia), sus sinapsis (solo las que conectan neuronas incluidas) y —para CADA una de
// las dos colecciones— su total real y si se recortó.
//
// Las sinapsis necesitan su propio par de campos, y no alcanza con `Truncated`: una sinapsis
// se pierde en cuanto UNO de sus extremos queda fuera del top-N, así que el recorte de
// aristas es muchísimo más agresivo que el de nodos y no se deduce del de neuronas. Medido
// en el cerebro central: 3660 neuronas capadas a 300 dejaban 486 sinapsis visibles, y el
// JSON no traía forma alguna de saber cuántas habían quedado afuera.
//
// OJO CON EL DENOMINADOR: TotalSynapses NO es `COUNT(*) FROM observation_relations`. Ese
// número incluye relaciones que tocan observaciones archivadas, superadas o en cuarentena
// —que el grafo excluye a propósito y no se dibujarían ni sin tope—, así que publicarlo
// cambiaría una mentira por otra. Y en el cerebro central sería peor: brainSynapses lee la
// tabla SIN scopeClause, y hoy no filtra memoria ajena sólo porque el gate es el set de
// neuronas visibles del scope; un COUNT crudo expondría la cardinalidad de otros tenants.
// El gemelo honesto de TotalNeurons es "relaciones con los DOS extremos visibles".
type BrainGraph struct {
	Neurons           []BrainNeuron  `json:"neurons"`
	Synapses          []BrainSynapse `json:"synapses"`
	TotalNeurons      int            `json:"total_neurons"`
	Truncated         bool           `json:"truncated"`
	TotalSynapses     int            `json:"total_synapses"`
	SynapsesTruncated bool           `json:"synapses_truncated"`
}

// newBrainGraph es el ÚNICO constructor de BrainGraph, y los totales entran como parámetros
// POSICIONALES a propósito: un literal con campos nombrados deja en cero y en silencio el
// total que uno se olvida de poner, que es exactamente cómo nació el bug que este archivo
// arregla. Así, agregar una colección recortada sin su total no compila.
func newBrainGraph(neurons []BrainNeuron, totalNeurons int, neuronsTruncated bool,
	synapses []BrainSynapse, totalSynapses int, synapsesTruncated bool) BrainGraph {
	if neurons == nil {
		neurons = []BrainNeuron{}
	}
	if synapses == nil {
		synapses = []BrainSynapse{}
	}
	return BrainGraph{
		Neurons: neurons, Synapses: synapses,
		TotalNeurons: totalNeurons, Truncated: neuronsTruncated,
		TotalSynapses: totalSynapses, SynapsesTruncated: synapsesTruncated,
	}
}

// defaultBrainNeuronLimit es el tope de neuronas del render por defecto: suficiente para
// una silueta densa sin castigar el force-sim del navegador (O(n^2) por frame).
const defaultBrainNeuronLimit = 300

// BrainGraph arma el grafo neuronal read-only. limit<=0 usa el default. Las neuronas se
// ordenan por saliencia = importance*exp(-ageDays/30) + ln(1+heat) y se capan a limit;
// las sinapsis se filtran a las que tienen ambos extremos entre las incluidas.
// BrainGraph arma el grafo neuronal FEDERADO (sin scope): todo el espacio. Lo usa
// `musubi export` (local monoproyecto).
func (e *DbEngine) BrainGraph(limit int) (BrainGraph, error) {
	return e.brainGraphAt(context.Background(), time.Now().UTC(), limit)
}

// BrainGraphCtx arma el grafo neuronal SCOPEADO al proyecto de la credencial (ctx):
// para servir el grafo por MCP sin filtrar memoria de otros tenants (central).
func (e *DbEngine) BrainGraphCtx(ctx context.Context, limit int) (BrainGraph, error) {
	return e.brainGraphAt(ctx, time.Now().UTC(), limit)
}

func (e *DbEngine) brainGraphAt(ctx context.Context, now time.Time, limit int) (BrainGraph, error) {
	if limit <= 0 {
		limit = defaultBrainNeuronLimit
	}

	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	// Usa el predicado CANÓNICO de visibilidad, no un `archived = 0` propio. Antes de F4 esta
	// query filtraba sólo por archived, y eso la dejaba fuera de sincronía con todo el resto
	// del recall por dos motivos:
	//
	//  1. Las observaciones en CUARENTENA (F4) se dibujarían como neuronas. Este grafo alimenta
	//     el dashboard, o sea la cara visual del cerebro: texto de un LLM sin corroborar
	//     apareciendo ahí es exactamente el daño que la Muralla 2 existe para evitar.
	//  2. Las REEMPLAZADAS (superseded_by) también se dibujaban. Eso ya estaba mal antes de esta
	//     fase —una nota superada no es memoria viva— y se corrige de paso.
	//
	// El punto 2 CAMBIA lo que muestra el dashboard: las notas superadas dejan de aparecer.
	// Es intencional y está decidido, no un efecto colateral que se coló.
	rows, err := e.db.QueryContext(ctx, `
		SELECT id, topic_key, COALESCE(mem_type,''), COALESCE(importance,1.0),
		       COALESCE(access_count,0), COALESCE(created_at,''), COALESCE(last_accessed,''),
		       COALESCE(NULLIF(gist,''), substr(content,1,120))
		FROM observations
		WHERE `+visibleObsPredicate+clause, args...)
	if err != nil {
		return BrainGraph{}, fmt.Errorf("brain: neuronas: %w", err)
	}
	var neurons []BrainNeuron
	for rows.Next() {
		var n BrainNeuron
		var created, last string
		if err := rows.Scan(&n.ID, &n.Topic, &n.MemType, &n.Importance, &n.Heat, &created, &last, &n.Gist); err != nil {
			rows.Close()
			return BrainGraph{}, fmt.Errorf("brain: escanear neurona: %w", err)
		}
		n.Domain = domainOf(n.Topic)
		n.AgeDays = daysSince(now, created)
		n.RecencyDays = daysSince(now, mostRecent(created, last))
		n.salience = n.Importance*math.Exp(-n.AgeDays/30.0) + math.Log(1+float64(n.Heat))
		neurons = append(neurons, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return BrainGraph{}, fmt.Errorf("brain: iterar neuronas: %w", err)
	}
	rows.Close()

	// El set VISIBLE se toma ACÁ, con todas las neuronas todavía en la mano y ANTES del cap:
	// es el universo contra el que se cuentan las sinapsis. Tomarlo después del recorte haría
	// colapsar el total a la cantidad dibujada y el bug volvería disfrazado de arreglo.
	visible := make(map[string]bool, len(neurons))
	for _, n := range neurons {
		visible[n.ID] = true
	}

	sort.SliceStable(neurons, func(i, j int) bool { return neurons[i].salience > neurons[j].salience })
	neurons, total, truncated := capped(neurons, limit)

	included := make(map[string]bool, len(neurons))
	for _, n := range neurons {
		included[n.ID] = true
	}

	synapses, totalSynapses, err := e.brainSynapses(visible, included)
	if err != nil {
		return BrainGraph{}, err
	}

	return newBrainGraph(neurons, total, truncated, synapses, totalSynapses, len(synapses) < totalSynapses), nil
}

// brainSynapses lee todas las relaciones y hace DOS cosas en la misma pasada:
//   - devuelve las que conectan dos neuronas INCLUIDAS (las dibujables, sin colgantes);
//   - cuenta las que conectan dos neuronas VISIBLES, que es el universo contra el cual
//     declarar el truncado.
//
// Los dos sets son distintos y el orden importa: `visible` son todas las observaciones que
// pasaron el predicado de visibilidad (≈ TotalNeurons), `included` son sólo las que además
// entraron en el top-N. Contar sobre `visible` y no sobre las filas crudas es lo que hace
// que el denominador pertenezca al mismo universo que el numerador: una relación con un
// extremo archivado, superado o en cuarentena no es una sinapsis que "no entró", es una que
// no existe para este grafo. Filtrar en Go evita un IN(...) gigante y su troceo.
func (e *DbEngine) brainSynapses(visible, included map[string]bool) ([]BrainSynapse, int, error) {
	rows, err := e.db.Query(`
		SELECT source_id, target_id, relation, COALESCE(confidence,0), COALESCE(status,'')
		FROM observation_relations`)
	if err != nil {
		return nil, 0, fmt.Errorf("brain: sinapsis: %w", err)
	}
	defer rows.Close()

	var out []BrainSynapse
	total := 0
	for rows.Next() {
		var s BrainSynapse
		if err := rows.Scan(&s.Source, &s.Target, &s.Relation, &s.Confidence, &s.Status); err != nil {
			return nil, 0, fmt.Errorf("brain: escanear sinapsis: %w", err)
		}
		if !visible[s.Source] || !visible[s.Target] {
			continue
		}
		total++
		if included[s.Source] && included[s.Target] {
			out = append(out, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("brain: iterar sinapsis: %w", err)
	}
	return out, total, nil
}

// domainOf deriva el dominio de un topic_key: el prefijo antes del primer '/' (o el
// topic entero si no tiene). "audit/token-system" -> "audit"; "overview" -> "overview".
func domainOf(topic string) string {
	if i := strings.IndexByte(topic, '/'); i >= 0 {
		return topic[:i]
	}
	return topic
}

// daysSince devuelve los días entre ts (formato SQLite o RFC3339) y now. Un ts vacío o
// no parseable devuelve 0 (se trata como "reciente": no penaliza ni infla la saliencia).
func daysSince(now time.Time, ts string) float64 {
	t, ok := parseObsTime(ts)
	if !ok {
		return 0
	}
	d := now.Sub(t).Hours() / 24.0
	if d < 0 {
		return 0
	}
	return d
}

// mostRecent devuelve el más nuevo de dos timestamps (los vacíos/ilegibles pierden).
func mostRecent(a, b string) string {
	ta, oka := parseObsTime(a)
	tb, okb := parseObsTime(b)
	switch {
	case oka && okb:
		if tb.After(ta) {
			return b
		}
		return a
	case okb:
		return b
	default:
		return a
	}
}

// parseObsTime parsea un timestamp de observación tolerando el layout de SQLite y RFC3339.
func parseObsTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{sqliteTimeLayout, time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
