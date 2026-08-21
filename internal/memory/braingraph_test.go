package memory

import (
	"context"
	"testing"
	"time"
)

// insBrainObs inserta una observación con control total de saliencia (importance,
// access_count, created_at) para los tests del grafo neuronal.
func insBrainObs(t *testing.T, e *DbEngine, id, topic string, importance float64, access int, created string) {
	t.Helper()
	_, err := e.db.Exec(`INSERT INTO observations
		(id, topic_key, content, gist, created_at, last_accessed, importance, access_count, archived)
		VALUES (?,?,?,?,?,?,?,?,0)`,
		id, topic, "contenido de "+id, "gist de "+id, created, created, importance, access)
	if err != nil {
		t.Fatalf("insertar obs %s: %v", id, err)
	}
}

func insBrainRel(t *testing.T, e *DbEngine, src, tgt, rel string, conf float64) {
	t.Helper()
	_, err := e.db.Exec(`INSERT INTO observation_relations
		(id, source_id, target_id, relation, confidence, status)
		VALUES (?,?,?,?,?, 'resolved')`,
		src+"->"+tgt, src, tgt, rel, conf)
	if err != nil {
		t.Fatalf("insertar rel %s->%s: %v", src, tgt, err)
	}
}

// TestBrainGraphSalienceAndCap: las neuronas se ordenan por saliencia y se capan a limit,
// exponiendo el total real y truncated (R2).
func TestBrainGraphSalienceAndCap(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
	old := now.Add(-400 * 24 * time.Hour).Format(sqliteTimeLayout)

	// alta saliencia: importante y reciente. baja: vieja y sin importancia.
	insBrainObs(t, e, "hi", "a/uno", 3.0, 5, recent)
	insBrainObs(t, e, "mid", "a/dos", 1.0, 0, recent)
	insBrainObs(t, e, "lo", "b/tres", 1.0, 0, old)

	g, err := e.brainGraphAt(context.Background(), now, 2)
	if err != nil {
		t.Fatal(err)
	}
	if g.TotalNeurons != 3 {
		t.Fatalf("total esperado 3, obtuve %d", g.TotalNeurons)
	}
	if !g.Truncated {
		t.Error("con 3 neuronas y limit 2 debe marcar truncated")
	}
	if len(g.Neurons) != 2 {
		t.Fatalf("esperaba 2 neuronas tras el cap, obtuve %d", len(g.Neurons))
	}
	if g.Neurons[0].ID != "hi" {
		t.Errorf("la neurona más saliente debe ser 'hi', obtuve %q", g.Neurons[0].ID)
	}
	// 'lo' (vieja, sin importancia) debe quedar fuera del top-2.
	for _, n := range g.Neurons {
		if n.ID == "lo" {
			t.Error("la neurona vieja/sin-peso no debía entrar en el top-2")
		}
	}
	// domain derivado del topic_key.
	if g.Neurons[0].Domain != "a" {
		t.Errorf("domain esperado 'a', obtuve %q", g.Neurons[0].Domain)
	}
}

// TestBrainGraphSynapsesNoDangling: solo se devuelven sinapsis con AMBOS extremos entre
// las neuronas incluidas (R3), incluso si otras relaciones apuntan fuera del set.
func TestBrainGraphSynapsesNoDangling(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
	for _, id := range []string{"n1", "n2", "n3"} {
		insBrainObs(t, e, id, "a/"+id, 1.0, 0, ts)
	}
	insBrainRel(t, e, "n1", "n2", "related", 0.9)      // ambos incluidos
	insBrainRel(t, e, "n2", "n3", "conflicts_with", 1) // ambos incluidos
	insBrainRel(t, e, "n1", "ghost", "related", 0.5)   // target inexistente → colgante

	g, err := e.brainGraphAt(context.Background(), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Synapses) != 2 {
		t.Fatalf("esperaba 2 sinapsis (sin la colgante), obtuve %d: %+v", len(g.Synapses), g.Synapses)
	}
	for _, s := range g.Synapses {
		if s.Target == "ghost" || s.Source == "ghost" {
			t.Error("no debía incluirse la sinapsis colgante hacia 'ghost'")
		}
	}
}

// marcarNoVisible saca una observación del universo visible por el eje pedido, para poder
// probar que el DENOMINADOR de las sinapsis vive en el mismo universo que el numerador.
func marcarNoVisible(t *testing.T, e *DbEngine, id, eje string) {
	t.Helper()
	var q string
	switch eje {
	case "superseded":
		q = `UPDATE observations SET superseded_by = 'otra' WHERE id = ?`
	case "quarantined":
		q = `UPDATE observations SET quarantined = 1 WHERE id = ?`
	case "archived":
		q = `UPDATE observations SET archived = 1 WHERE id = ?`
	default:
		t.Fatalf("eje desconocido: %s", eje)
	}
	if _, err := e.db.Exec(q, id); err != nil {
		t.Fatalf("marcar %s como %s: %v", id, eje, err)
	}
}

// TestBrainGraphSinapsisDeclaranSuTotal es el test que faltaba y por cuya ausencia el bug
// vivió: el único test de sinapsis corría con limit=100 sobre 3 neuronas, así que la rama
// que descarta aristas por el cap JAMÁS se ejecutaba y nada podía ponerse rojo.
//
// INVARIANTE: cuando el cap deja una sinapsis afuera, el grafo lo DECLARA — TotalSynapses
// cuenta la perdida y SynapsesTruncated es true. Sin esto, el consumidor mide len(Synapses)
// y publica ese número como si fuera el universo.
func TestBrainGraphSinapsisDeclaranSuTotal(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)

	// Saliencia decreciente para que el top-2 sea determinista: n1, n2.
	insBrainObs(t, e, "n1", "a/n1", 3.0, 0, ts)
	insBrainObs(t, e, "n2", "a/n2", 2.0, 0, ts)
	insBrainObs(t, e, "n3", "a/n3", 1.0, 0, ts)
	insBrainRel(t, e, "n1", "n2", "related", 0.9) // sobrevive al cap
	insBrainRel(t, e, "n2", "n3", "related", 0.9) // n3 queda fuera del top-2 → se pierde

	g, err := e.brainGraphAt(context.Background(), now, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Synapses) != 1 {
		t.Fatalf("con el cap en 2 sólo n1↔n2 es dibujable, obtuve %d: %+v", len(g.Synapses), g.Synapses)
	}
	if g.TotalSynapses != 2 {
		t.Errorf("TotalSynapses debe contar la que el cap dejó afuera: esperaba 2, obtuve %d", g.TotalSynapses)
	}
	if !g.SynapsesTruncated {
		t.Error("si se dibuja menos de lo que hay, SynapsesTruncated tiene que ser true")
	}

	// Sin cap no se pierde nada, y entonces el total DEBE coincidir con lo dibujado: si no,
	// el campo estaría contando otra cosa (por ejemplo las filas crudas de la tabla).
	full, err := e.brainGraphAt(context.Background(), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if full.TotalSynapses != len(full.Synapses) {
		t.Errorf("sin truncado el total debe igualar lo dibujado: total=%d dibujadas=%d", full.TotalSynapses, len(full.Synapses))
	}
	if full.SynapsesTruncated {
		t.Error("sin cap no puede marcar SynapsesTruncated")
	}
}

// TestBrainGraphTotalSinapsisIgnoraNoVisibles fija el invariante que impide "arreglar" esto
// con un COUNT(*) sobre observation_relations.
//
// INVARIANTE: el denominador pertenece al MISMO universo que el numerador. Una relación con
// un extremo archivado, superado o en cuarentena no es una sinapsis que "no entró": es una
// que no existe para este grafo. Si TotalSynapses contara las filas crudas, "486/3620"
// mentiría para el otro lado — y en el cerebro central expondría además la cardinalidad de
// otros tenants, porque brainSynapses lee la tabla sin scopeClause.
func TestBrainGraphTotalSinapsisIgnoraNoVisibles(t *testing.T) {
	for _, eje := range []string{"superseded", "quarantined", "archived"} {
		t.Run(eje, func(t *testing.T) {
			e := newTestEngine(t)
			now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
			ts := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
			insBrainObs(t, e, "v1", "a/v1", 3.0, 0, ts)
			insBrainObs(t, e, "v2", "a/v2", 2.0, 0, ts)
			insBrainObs(t, e, "oculta", "a/oculta", 1.0, 0, ts)
			insBrainRel(t, e, "v1", "v2", "related", 0.9)     // visible↔visible: cuenta
			insBrainRel(t, e, "v1", "oculta", "related", 0.9) // toca una oculta: NO cuenta
			marcarNoVisible(t, e, "oculta", eje)

			g, err := e.brainGraphAt(context.Background(), now, 100)
			if err != nil {
				t.Fatal(err)
			}
			if g.TotalNeurons != 2 {
				t.Fatalf("la observación %s no debe ser neurona: esperaba 2, obtuve %d", eje, g.TotalNeurons)
			}
			if g.TotalSynapses != 1 {
				t.Errorf("TotalSynapses debe ignorar la relación hacia la %s: esperaba 1, obtuve %d", eje, g.TotalSynapses)
			}
			if g.SynapsesTruncated {
				t.Error("una relación a memoria no visible no es truncado de render: no debe marcar SynapsesTruncated")
			}
		})
	}
}

// TestBrainGraphEmpty: una memoria vacía devuelve slices no-nil (JSON [] y no null) y
// no crashea (escenario 'vacío').
func TestBrainGraphEmpty(t *testing.T) {
	e := newTestEngine(t)
	g, err := e.BrainGraph(0)
	if err != nil {
		t.Fatal(err)
	}
	if g.Neurons == nil || g.Synapses == nil {
		t.Error("neurons/synapses deben ser slices no-nil aun con memoria vacía")
	}
	if g.TotalNeurons != 0 || g.Truncated {
		t.Errorf("memoria vacía: total 0 y no truncated, obtuve total=%d truncated=%v", g.TotalNeurons, g.Truncated)
	}
}
