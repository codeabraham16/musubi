package memory

import (
	"context"
	"testing"
	"time"
)

// tocar simula un RECALL: sube el calor y la última lectura, sin crear nada.
func tocar(t *testing.T, e *DbEngine, id string, when time.Time) {
	t.Helper()
	_, err := e.db.Exec(`UPDATE observations SET access_count = access_count + 1, last_accessed = ? WHERE id = ?`,
		when.UTC().Format(sqliteTimeLayout), id)
	if err != nil {
		t.Fatalf("tocar %s: %v", id, err)
	}
}

// TestGraphVersionNoSeMueveConUnRecall es EL invariante que sostiene el diseño sin tope.
//
// El cliente re-baja el grafo entero cuando cambia `graph_version`. Si la huella incluyera el
// calor o la recencia, CADA recall dispararía una re-bajada de cientos de KB — es decir,
// volveríamos al problema que motivó todo esto, sólo que disparado por consultas en vez de por
// el reloj. La huella tiene que moverse cuando cambia la TOPOLOGÍA (nace/desaparece un nodo o
// una arista) y quedarse quieta cuando sólo cambia la temperatura.
func TestGraphVersionNoSeMueveConUnRecall(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-48 * time.Hour).Format(sqliteTimeLayout)
	insBrainObs(t, e, "a", "x/a", 1.0, 0, ts)
	insBrainObs(t, e, "b", "x/b", 1.0, 0, ts)

	v1, err := e.BrainGraphVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}

	tocar(t, e, "a", now)
	v2, err := e.BrainGraphVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Tag() != v2.Tag() {
		t.Errorf("un recall NO debe mover la huella: %q -> %q", v1.Tag(), v2.Tag())
	}

	// Y sí tiene que moverse cuando nace algo, o el cliente nunca se enteraría.
	insBrainObs(t, e, "c", "x/c", 1.0, 0, now.Format(sqliteTimeLayout))
	v3, err := e.BrainGraphVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Tag() == v3.Tag() {
		t.Errorf("nacer una neurona TIENE que mover la huella, pero quedó en %q", v3.Tag())
	}

	insBrainRel(t, e, "a", "b", "related", 0.9)
	v4, err := e.BrainGraphVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v3.Tag() == v4.Tag() {
		t.Errorf("nacer una sinapsis TIENE que mover la huella, pero quedó en %q", v4.Tag())
	}
}

// TestPulseSeparaNacidasDeRecordadas: el nodo nuevo viaja ENTERO (el cliente lo agrega sin
// re-bajar el grafo) y el recordado viaja como id + dos números.
func TestPulseSeparaNacidasDeRecordadas(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	corte := now.Add(-1 * time.Hour)
	viejo := now.Add(-72 * time.Hour).Format(sqliteTimeLayout)

	insBrainObs(t, e, "vieja", "dom/vieja", 2.0, 3, viejo)
	insBrainObs(t, e, "quieta", "dom/quieta", 1.0, 0, viejo)
	insBrainObs(t, e, "nacida", "dom/nacida", 5.0, 0, now.Format(sqliteTimeLayout))
	tocar(t, e, "vieja", now)

	p, err := e.BrainPulseSince(ctx, corte, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.NewNeurons) != 1 || p.NewNeurons[0].ID != "nacida" {
		t.Fatalf("esperaba sólo 'nacida' en new_neurons, obtuve %+v", p.NewNeurons)
	}
	// Entera: sin estos campos el cliente no puede dibujarla y tendría que re-bajar el grafo.
	n := p.NewNeurons[0]
	if n.Topic == "" || n.Domain == "" || n.Importance == 0 {
		t.Errorf("la nacida tiene que viajar completa (topic/domain/importance), obtuve %+v", n)
	}
	if len(p.Touched) != 1 || p.Touched[0].ID != "vieja" {
		t.Fatalf("esperaba sólo 'vieja' en touched, obtuve %+v", p.Touched)
	}
	if p.Touched[0].Heat != 4 {
		t.Errorf("el calor de la tocada debe venir actualizado (3+1=4), obtuve %d", p.Touched[0].Heat)
	}
	// 'quieta' no se tocó: no puede aparecer en ningún lado.
	for _, x := range p.Touched {
		if x.ID == "quieta" {
			t.Error("una neurona que nadie tocó no debe viajar en el pulso")
		}
	}
}

// TestPulsePrimerLatidoNoInventaActividad: sin `since`, el pulso trae los contadores pero
// NINGÚN delta. Si devolviera el histórico, el render pintaría como "acabando de pasar"
// actividad de hace semanas la primera vez que alguien abre la página.
func TestPulsePrimerLatidoNoInventaActividad(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	insBrainObs(t, e, "a", "x/a", 1.0, 7, now.Format(sqliteTimeLayout))

	p, err := e.BrainPulseSince(context.Background(), time.Time{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if p.Counts.Neurons != 1 {
		t.Errorf("los contadores sí van en el primer latido: esperaba 1, obtuve %d", p.Counts.Neurons)
	}
	if len(p.Touched) != 0 || len(p.NewNeurons) != 0 || len(p.NewSynapses) != 0 {
		t.Errorf("el primer latido no debe traer deltas: touched=%d new=%d syn=%d",
			len(p.Touched), len(p.NewNeurons), len(p.NewSynapses))
	}
}

// TestPulseCuentaLoMismoQueElGrafo: los contadores del HUD y los del grafo tienen que salir
// del MISMO universo. Es el mismo invariante que se arregló en el render — un panel que dice
// un total y un dibujo que muestra otro es la falla que este track existe para cerrar.
func TestPulseCuentaLoMismoQueElGrafo(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
	for _, id := range []string{"n1", "n2", "n3", "oculta"} {
		insBrainObs(t, e, id, "d/"+id, 1.0, 0, ts)
	}
	insBrainRel(t, e, "n1", "n2", "related", 0.9)
	insBrainRel(t, e, "n2", "n3", "related", 0.9)
	insBrainRel(t, e, "n1", "oculta", "related", 0.9) // se pierde al ocultar el extremo
	marcarNoVisible(t, e, "oculta", "superseded")

	g, err := e.brainGraphAt(ctx, now, NoLimit)
	if err != nil {
		t.Fatal(err)
	}
	p, err := e.BrainPulseSince(ctx, time.Time{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if p.Counts.Neurons != g.TotalNeurons {
		t.Errorf("neuronas: el pulso dice %d y el grafo %d", p.Counts.Neurons, g.TotalNeurons)
	}
	if p.Counts.Synapses != g.TotalSynapses {
		t.Errorf("sinapsis: el pulso dice %d y el grafo %d", p.Counts.Synapses, g.TotalSynapses)
	}
	if p.Counts.Synapses != 2 {
		t.Errorf("la relación hacia la oculta no cuenta: esperaba 2, obtuve %d", p.Counts.Synapses)
	}
}

// TestNoLimitTraeElGrafoEntero: el valor explícito existe para poder pedir TODO. Antes
// `limit <= 0` caía al default de 300, así que no había forma de expresarlo — el tope no se
// podía sacar ni queriendo, que es exactamente el pedido que originó este cambio.
func TestNoLimitTraeElGrafoEntero(t *testing.T) {
	e := newTestEngine(t)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	ts := now.Add(-24 * time.Hour).Format(sqliteTimeLayout)
	const total = 420 // por encima del default de 300, para que el tope se note si aparece
	for i := 0; i < total; i++ {
		insBrainObs(t, e, string(rune('a'+i%26))+"-"+time.Duration(i).String(), "d/x", 1.0, 0, ts)
	}

	sinTope, err := e.brainGraphAt(context.Background(), now, NoLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(sinTope.Neurons) != total || sinTope.Truncated {
		t.Errorf("NoLimit debe traer las %d: obtuve %d con truncated=%v", total, len(sinTope.Neurons), sinTope.Truncated)
	}

	// Y el cero sigue cayendo al default: las tools MCP no deben cambiar de comportamiento.
	porDefecto, err := e.brainGraphAt(context.Background(), now, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(porDefecto.Neurons) != defaultBrainNeuronLimit || !porDefecto.Truncated {
		t.Errorf("limit=0 debe seguir capando en %d: obtuve %d trunc=%v",
			defaultBrainNeuronLimit, len(porDefecto.Neurons), porDefecto.Truncated)
	}
}
