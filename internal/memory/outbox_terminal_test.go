package memory

import (
	"strings"
	"testing"
)

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

// PurgeOutboxPending es la limpieza del residuo INMORTAL: filas 'pending' que un binario viejo dejó
// en un nodo terminal (las "571 filas muertas" del cerebro real). Las borra sin tocar las
// observaciones — sólo descarta el intento de envío.
func TestPurgeOutboxPendingLimpiaHuerfanas(t *testing.T) {
	e := newTestEngine(t) // cliente: encola por default
	for _, id := range []string{"obs-1", "obs-2", "obs-3"} {
		if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", id, "t/a", "memoria "+id, 0.5, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	if n := pendientesOutbox(t, e); n != 3 {
		t.Fatalf("preparación: esperaba 3 filas, hay %d", n)
	}

	n, err := e.PurgeOutboxPending()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("PurgeOutboxPending devolvió %d, esperaba 3", n)
	}
	if r := pendientesOutbox(t, e); r != 0 {
		t.Errorf("tras purgar quedan %d filas, esperaba 0", r)
	}

	// El contenido NO se toca: purgar el outbox descarta el envío, no la memoria.
	var c int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id IN ('obs-1','obs-2','obs-3')`).Scan(&c); err != nil {
		t.Fatal(err)
	}
	if c != 3 {
		t.Errorf("purgar el outbox borró observaciones: quedan %d de 3", c)
	}
}

// checkOutboxStall es el detector del stall SILENCIOSO (el que dejó 650 filas 9 días sin avisar).
// Vacío/fresco ⇒ ok; una 'shared' vieja SIN un solo intento de envío ⇒ warning que lo distingue.
func TestCheckOutboxStallDetectaElStallSilencioso(t *testing.T) {
	e := newTestEngine(t)

	if r := checkOutboxStall(e); r.Status != "ok" {
		t.Errorf("outbox vacío: status=%q msg=%q, esperaba ok", r.Status, r.Message)
	}

	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "reciente", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if r := checkOutboxStall(e); r.Status != "ok" {
		t.Errorf("pendiente fresca: status=%q, esperaba ok (drena en segundos)", r.Status)
	}

	// Envejecer la fila más allá del umbral, sin last_error (nunca se intentó enviar) ⇒ stall silencioso.
	if _, err := e.db.Exec(`UPDATE outbox SET created_at = datetime('now','-9 days'), last_error = NULL`); err != nil {
		t.Fatal(err)
	}
	r := checkOutboxStall(e)
	if r.Status != "warning" {
		t.Fatalf("pendiente vieja sin intento: status=%q, esperaba warning", r.Status)
	}
	if !strings.Contains(r.Message, "SIN UN SOLO intento") {
		t.Errorf("el mensaje no marca el stall silencioso: %q", r.Message)
	}
}
