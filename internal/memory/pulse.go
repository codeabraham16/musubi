package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// pulse.go parte en dos lo que el dashboard pedía como un bloque único, y esa división es lo
// que permite BORRAR el tope del grafo.
//
// EL PROBLEMA, MEDIDO. El front pedía /api/snapshot entero cada 5 segundos: 631 KB con el
// tope en 300 neuronas, y 2,3 MB si se lo saca. Por un túnel SSH eso son 31 s y 117 s
// respectivamente — o sea que a los 5 segundos ya hay seis pedidos apilados peleándose el
// ancho de banda y ninguno termina. El síntoma que se ve es un grafo apagado: la actividad
// (el brillo) sólo se enciende al diffear un snapshot nuevo contra el anterior, así que si
// nunca llega uno completo, todo decae a color de reposo. Subir el tope sin tocar esto
// empeora el problema en vez de mostrar más.
//
// LA DIVISIÓN. El GRAFO cambia cuando nace una memoria o una relación —cada tanto—, mientras
// que el PULSO (qué se calentó, cuántas hay, cómo está la salud) cambia todo el tiempo y pesa
// unos pocos KB. Entonces:
//
//	GraphVersion  huella barata del grafo; si no cambió, el cliente no vuelve a bajarlo
//	BrainPulse    lo chico y frecuente, con los deltas de actividad desde `since`
//
// Así el sondeo de 5 s deja de arrastrar el grafo, y el grafo puede crecer con la memoria en
// vez de estar capado para caber en un intervalo de sondeo.
//
// Es todo agregación SQL determinista: model-free, read-only, sin LLM en el medio.

// GraphVersion es la HUELLA del grafo neuronal: cambia si nace o desaparece una neurona o una
// sinapsis. No incluye el calor ni la recencia a propósito — esos se mueven con cada recall y
// harían re-bajar el grafo entero por un cambio que el pulso ya transporta en unos bytes.
type GraphVersion struct {
	Neurons  int    `json:"neurons"`
	Synapses int    `json:"synapses"`
	LastObs  string `json:"last_obs,omitempty"`
	LastRel  string `json:"last_rel,omitempty"`
}

// Tag es la huella en una línea, para comparar y para el ETag HTTP.
func (v GraphVersion) Tag() string {
	return fmt.Sprintf("n%d-s%d-%s-%s", v.Neurons, v.Synapses, compactTS(v.LastObs), compactTS(v.LastRel))
}

// compactTS deja un timestamp en algo corto y comparable; vacío o ilegible da "0".
func compactTS(ts string) string {
	t, ok := parseObsTime(ts)
	if !ok {
		return "0"
	}
	return fmt.Sprintf("%d", t.Unix())
}

// TouchedNeuron es una neurona YA CONOCIDA cuyo calor o recencia se movió desde el último
// pulso: es actividad de RECALL. Alcanza con el id y dos números porque el cliente ya tiene
// el resto del nodo.
type TouchedNeuron struct {
	ID          string  `json:"id"`
	Heat        int     `json:"heat"`
	RecencyDays float64 `json:"recency_days"`
}

// PulseCounts son los tamaños reales del acervo. Van SIEMPRE, aunque el grafo esté truncado:
// es la cifra que el HUD tiene que poder mostrar como denominador honesto.
type PulseCounts struct {
	Neurons  int `json:"neurons"`
	Synapses int `json:"synapses"`
	Domains  int `json:"domains"`
}

// BrainPulse es el latido: lo que cambia seguido y pesa poco. NO lleva el grafo, y no lleva las
// hojas de topic (2.809 en el cerebro central, 278 KB, que el front nunca leyó).
//
// NewNeurons va con el nodo ENTERO y no sólo su id, y esa decisión es la que hace que el grafo
// sin tope sea sostenible: cualquier memoria nueva cambia la huella del grafo, así que si el
// cliente tuviera que re-bajarlo para incorporarla, volveríamos a arrastrar cientos de KB —
// ahora cada vez que alguien guarda algo, en vez de cada 5 segundos. Con el nodo completo acá,
// el cliente lo AGREGA a lo que ya tiene y la re-bajada queda para el caso raro en que
// desaparecen nodos (archivar, superseder, cuarentena), que se detecta comparando `Counts`.
type BrainPulse struct {
	Now          string          `json:"now"`
	GraphVersion string          `json:"graph_version"`
	Counts       PulseCounts     `json:"counts"`
	Domains      []DomainCount   `json:"domains"`
	Touched      []TouchedNeuron `json:"touched"`
	NewNeurons   []BrainNeuron   `json:"new_neurons"`
	NewSynapses  []BrainSynapse  `json:"new_synapses"`
}

// BrainGraphVersion calcula la huella del grafo con dos agregaciones. Barata a propósito: la
// corre cada sondeo, así que no puede costar lo que cuesta armar el grafo.
func (e *DbEngine) BrainGraphVersion(ctx context.Context) (GraphVersion, error) {
	var v GraphVersion
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(created_at),'') FROM observations WHERE `+visibleObsPredicate+clause,
		args...,
	).Scan(&v.Neurons, &v.LastObs); err != nil {
		return v, fmt.Errorf("pulse: versión de neuronas: %w", err)
	}
	// Las relaciones no tienen scope propio (ver brainSynapses): se cuentan todas y el gate
	// real de visibilidad lo pone el set de neuronas. Acá alcanza para detectar CAMBIO.
	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(COALESCE(updated_at, created_at)),'') FROM observation_relations`,
	).Scan(&v.Synapses, &v.LastRel); err != nil {
		return v, fmt.Errorf("pulse: versión de sinapsis: %w", err)
	}
	return v, nil
}

// BrainPulseSince arma el latido: los tamaños reales, los dominios sin sus hojas, y los deltas
// de actividad desde `since`. Un `since` cero devuelve los deltas vacíos —no el histórico
// entero—: el primer pulso de un cliente sirve para fijar el reloj, no para pintar actividad
// vieja como si acabara de pasar.
func (e *DbEngine) BrainPulseSince(ctx context.Context, since time.Time, now time.Time) (BrainPulse, error) {
	var p BrainPulse
	p.Now = now.UTC().Format(time.RFC3339Nano)

	ver, err := e.BrainGraphVersion(ctx)
	if err != nil {
		return p, err
	}
	p.GraphVersion = ver.Tag()
	p.Counts.Neurons = ver.Neurons

	doms, err := e.TopicDomainCounts()
	if err != nil {
		return p, fmt.Errorf("pulse: dominios: %w", err)
	}
	if doms == nil {
		doms = []DomainCount{}
	}
	p.Domains = doms
	p.Counts.Domains = len(doms)

	// Las sinapsis del contador son las VISIBLES (los dos extremos dibujables), el mismo
	// denominador que declara BrainGraph. Contarlas de otra forma haría que el HUD muestre un
	// total y el grafo otro, que es justo el defecto que se arregló en el render.
	visibles, err := e.visibleObsIDs(ctx)
	if err != nil {
		return p, err
	}
	p.Counts.Synapses, err = e.countVisibleSynapses(visibles)
	if err != nil {
		return p, err
	}

	p.Touched = []TouchedNeuron{}
	p.NewNeurons = []BrainNeuron{}
	p.NewSynapses = []BrainSynapse{}
	if since.IsZero() {
		return p, nil
	}

	ts := since.UTC().Format(sqliteTimeLayout)
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	// Una sola query para las dos clases de actividad; el reparto se hace por created_at, que
	// es lo que distingue «nació» (escritura) de «se recordó» (recall). Se piden los campos
	// completos porque las nacidas viajan enteras: ver el comentario de BrainPulse.
	q := `SELECT id, topic_key, COALESCE(mem_type,''), COALESCE(importance,1.0),
	             COALESCE(access_count,0), COALESCE(created_at,''), COALESCE(last_accessed,''),
	             COALESCE(NULLIF(gist,''), substr(content,1,120))
	      FROM observations
	      WHERE ` + visibleObsPredicate + clause + `
	        AND (COALESCE(last_accessed,'') > ? OR COALESCE(created_at,'') > ?)`
	rows, err := e.db.QueryContext(ctx, q, append(args, ts, ts)...)
	if err != nil {
		return p, fmt.Errorf("pulse: neuronas tocadas: %w", err)
	}
	for rows.Next() {
		var n BrainNeuron
		var created, last string
		if err := rows.Scan(&n.ID, &n.Topic, &n.MemType, &n.Importance, &n.Heat, &created, &last, &n.Gist); err != nil {
			rows.Close()
			return p, fmt.Errorf("pulse: escanear tocada: %w", err)
		}
		n.Domain = domainOf(n.Topic)
		n.AgeDays = daysSince(now, created)
		n.RecencyDays = daysSince(now, mostRecent(created, last))
		if created > ts {
			p.NewNeurons = append(p.NewNeurons, n)
		} else {
			p.Touched = append(p.Touched, TouchedNeuron{ID: n.ID, Heat: n.Heat, RecencyDays: n.RecencyDays})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return p, fmt.Errorf("pulse: iterar tocadas: %w", err)
	}
	rows.Close()

	srows, err := e.db.QueryContext(ctx,
		`SELECT source_id, target_id, relation, COALESCE(confidence,0), COALESCE(status,'')
		 FROM observation_relations WHERE COALESCE(created_at,'') > ?`, ts)
	if err != nil {
		return p, fmt.Errorf("pulse: sinapsis nuevas: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var s BrainSynapse
		if err := srows.Scan(&s.Source, &s.Target, &s.Relation, &s.Confidence, &s.Status); err != nil {
			return p, fmt.Errorf("pulse: escanear sinapsis nueva: %w", err)
		}
		if visibles[s.Source] && visibles[s.Target] {
			p.NewSynapses = append(p.NewSynapses, s)
		}
	}
	return p, srows.Err()
}

// visibleObsIDs devuelve el set de observaciones dibujables del scope. Es el mismo universo
// que usa BrainGraph para contar sus sinapsis; compartirlo es lo que evita que el pulso y el
// grafo afirmen dos poblaciones distintas.
func (e *DbEngine) visibleObsIDs(ctx context.Context) (map[string]bool, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	rows, err := e.db.QueryContext(ctx, `SELECT id FROM observations WHERE `+visibleObsPredicate+clause, args...)
	if err != nil {
		return nil, fmt.Errorf("pulse: ids visibles: %w", err)
	}
	defer rows.Close()
	out := make(map[string]bool, 4096)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("pulse: escanear id visible: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// countVisibleSynapses cuenta las relaciones con los DOS extremos visibles, en Go y de una
// pasada, por el mismo motivo que brainSynapses: evita un IN(...) gigante y su troceo.
func (e *DbEngine) countVisibleSynapses(visible map[string]bool) (int, error) {
	rows, err := e.db.Query(`SELECT source_id, target_id FROM observation_relations`)
	if err != nil {
		return 0, fmt.Errorf("pulse: contar sinapsis: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var s, t string
		if err := rows.Scan(&s, &t); err != nil {
			return 0, fmt.Errorf("pulse: escanear sinapsis: %w", err)
		}
		if visible[s] && visible[t] {
			n++
		}
	}
	return n, rows.Err()
}

// ParsePulseSince lee el `since` que manda el cliente (RFC3339). Vacío o ilegible ⇒ cero, que
// significa «primer pulso»: se devuelven los contadores pero ningún delta.
func ParsePulseSince(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, sqliteTimeLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
