package memory

import "testing"

// El CENTRAL es un nodo TERMINAL: sirve memoria pero no tiene upstream a dónde empujarla. Encolar
// ahí dejaba una fila `pending` INMORTAL por cada observación ingerida — nunca drenaban (el drain
// ni arranca sin sync configurado) y hacían que `sync_status` contra el cerebro reportara miles de
// "pendientes de envío". No era un loop, pero era una señal de salud que MIENTE: medido en el
// cerebro real, 571 filas muertas, y ya mandó a investigar un problema inexistente dos veces.

func pendientesOutbox(t *testing.T, e *DbEngine) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&n); err != nil {
		t.Fatalf("contar outbox: %v", err)
	}
	return n
}

// Con el outbox APAGADO (nodo terminal), un guardado 'shared' persiste la observación pero NO
// encola nada. La memoria se guarda igual: lo que se apaga es la cola de envío, no el guardado.
func TestOutboxApagadoNoEncolaPeroSiGuarda(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(false)

	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "memoria ingerida", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Errorf("outbox apagado: esperaba 0 filas, hay %d (vuelven las filas inmortales)", n)
	}

	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id='obs-1'`).Scan(&scope); err != nil {
		t.Fatalf("la observación NO se guardó: %v", err)
	}
	if scope != ScopeShared {
		t.Errorf("scope = %q, esperaba 'shared' — apagar el outbox no puede cambiar lo que se guarda", scope)
	}
}

// Promover con el outbox apagado tampoco encola (el otro camino que llama a enqueueOutboxTx).
func TestOutboxApagadoNoEncolaAlPromover(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(false)

	if err := e.SaveObservationTyped("obs-1", "t/a", "memoria local", 0.5, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.PromoteObservation("obs-1"); err != nil {
		t.Fatal(err)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Errorf("promover con outbox apagado: esperaba 0 filas, hay %d", n)
	}
}

// Y el default NO cambia: todo CLIENTE encola como siempre. El cero de un bool es false, así que
// si el constructor no lo prendiera explícitamente, ningún cliente sincronizaría nada.
func TestOutboxEncolaPorDefaultEnUnCliente(t *testing.T) {
	e := newTestEngine(t) // sin tocar SetOutboxEnabled: el default de todo cliente

	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "memoria propia", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if n := pendientesOutbox(t, e); n != 1 {
		t.Fatalf("un cliente DEBE encolar sus 'shared': esperaba 1 fila, hay %d", n)
	}
}
