package memory

import (
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

// BrainGraph es el grafo neuronal completo para el render: las neuronas incluidas (top-N
// por saliencia), sus sinapsis (solo las que conectan neuronas incluidas) y el total real
// (para señalar truncamiento en la UI).
type BrainGraph struct {
	Neurons      []BrainNeuron  `json:"neurons"`
	Synapses     []BrainSynapse `json:"synapses"`
	TotalNeurons int            `json:"total_neurons"`
	Truncated    bool           `json:"truncated"`
}

// defaultBrainNeuronLimit es el tope de neuronas del render por defecto: suficiente para
// una silueta densa sin castigar el force-sim del navegador (O(n^2) por frame).
const defaultBrainNeuronLimit = 300

// BrainGraph arma el grafo neuronal read-only. limit<=0 usa el default. Las neuronas se
// ordenan por saliencia = importance*exp(-ageDays/30) + ln(1+heat) y se capan a limit;
// las sinapsis se filtran a las que tienen ambos extremos entre las incluidas.
func (e *DbEngine) BrainGraph(limit int) (BrainGraph, error) {
	return e.brainGraphAt(time.Now().UTC(), limit)
}

func (e *DbEngine) brainGraphAt(now time.Time, limit int) (BrainGraph, error) {
	if limit <= 0 {
		limit = defaultBrainNeuronLimit
	}

	rows, err := e.db.Query(`
		SELECT id, topic_key, COALESCE(mem_type,''), COALESCE(importance,1.0),
		       COALESCE(access_count,0), COALESCE(created_at,''), COALESCE(last_accessed,''),
		       COALESCE(NULLIF(gist,''), substr(content,1,120))
		FROM observations
		WHERE archived = 0`)
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

	total := len(neurons)
	sort.SliceStable(neurons, func(i, j int) bool { return neurons[i].salience > neurons[j].salience })
	truncated := false
	if len(neurons) > limit {
		neurons = neurons[:limit]
		truncated = true
	}

	included := make(map[string]bool, len(neurons))
	for _, n := range neurons {
		included[n.ID] = true
	}

	synapses, err := e.brainSynapses(included)
	if err != nil {
		return BrainGraph{}, err
	}

	if neurons == nil {
		neurons = []BrainNeuron{}
	}
	if synapses == nil {
		synapses = []BrainSynapse{}
	}
	return BrainGraph{Neurons: neurons, Synapses: synapses, TotalNeurons: total, Truncated: truncated}, nil
}

// brainSynapses lee todas las relaciones y devuelve solo las que conectan dos neuronas
// incluidas (sin aristas colgantes). Filtrar en Go evita un IN(...) gigante y el troceo.
func (e *DbEngine) brainSynapses(included map[string]bool) ([]BrainSynapse, error) {
	rows, err := e.db.Query(`
		SELECT source_id, target_id, relation, COALESCE(confidence,0), COALESCE(status,'')
		FROM observation_relations`)
	if err != nil {
		return nil, fmt.Errorf("brain: sinapsis: %w", err)
	}
	defer rows.Close()

	var out []BrainSynapse
	for rows.Next() {
		var s BrainSynapse
		if err := rows.Scan(&s.Source, &s.Target, &s.Relation, &s.Confidence, &s.Status); err != nil {
			return nil, fmt.Errorf("brain: escanear sinapsis: %w", err)
		}
		if included[s.Source] && included[s.Target] {
			out = append(out, s)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("brain: iterar sinapsis: %w", err)
	}
	return out, nil
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
