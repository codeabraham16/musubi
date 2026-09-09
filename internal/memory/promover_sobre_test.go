package memory

// PROMOVER ES LA SEGUNDA PUERTA DEL MISMO CUARTO (P1–P4).
//
// La guarda del sobre vive en saveObservation, «donde nace el contenido». Pero promover a 'shared'
// es un UPDATE por id que no pasa por ahí — el mismo argumento por el que la guarda de cuarentena
// tuvo que ponerse en PromoteObservationCtx y no alcanzaba con el predicado de visibilidad.
//
// Los cuatro casos fijan dónde está la línea: se rechaza la decisión NUEVA de compartir contenido
// dañado (P1), sin dejar la fila a medias (P2), sin romper la promoción normal (P3) y sin romper la
// idempotencia documentada de las que ya cruzaron (P4).

import (
	"errors"
	"testing"
)

// yaShared marca una fila como 'shared' por SQL crudo, sin pasar por la promoción. Es como están
// las 73 de producción: cruzaron antes de que la guarda existiera.
func yaShared(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	res, err := e.db.Exec(`UPDATE observations SET scope = ? WHERE id = ?`, ScopeShared, id)
	if err != nil {
		t.Fatalf("marcar %s como shared: %v", id, err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		t.Fatalf("marcar shared no tocó la fila de %s (filas=%d, err=%v)", id, n, err)
	}
}

// P1 — PROMOVER UNA LOCAL CON EL SOBRE SE RECHAZA, Y CON EL ERROR DE LA CLASE.
//
// No alcanza con que falle: tiene que fallar con ErrPayloadInvalido, que es lo que el borde MCP
// traduce a un código JSON-RPC permanente. Un error suelto saldría como «error interno» y le diría
// al que llama que insista.
func TestP1PromoverUnaLocalConElSobreSeRechaza(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	sembrarConSobre(t, e, "sucia", conSobre("una nota que arrastra su sobre", "1.9"), 1.0)

	err := e.PromoteObservation("sucia")
	if err == nil {
		t.Fatal("promover a 'shared' un content con el sobre adentro PASÓ: la guarda de saveObservation no cubre esta puerta")
	}
	if !errors.Is(err, ErrPayloadInvalido) {
		t.Errorf("falló con %v, pero no como pedido inválido: el borde lo emitiría como «error interno» y el que llama insistiría", err)
	}
}

// P2 — Y NO DEJA LA FILA A MEDIAS.
//
// El daño de verdad no es el error: es una fila marcada 'shared' y encolada que el central nunca va
// a aceptar. Se mide el ESTADO después del rechazo, no el error, porque una guarda que rechaza
// después de haber escrito rechaza y ensucia igual.
func TestP2ElRechazoNoDejaLaFilaAMedias(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	sembrarConSobre(t, e, "sucia", conSobre("una nota que arrastra su sobre", "1.9"), 1.0)

	if err := e.PromoteObservation("sucia"); err == nil {
		t.Fatal("precondición: se esperaba el rechazo")
	}

	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id = ?`, "sucia").Scan(&scope); err != nil {
		t.Fatalf("releer el scope: %v", err)
	}
	if scope != ScopeLocal {
		t.Errorf("la fila quedó en scope %q tras el rechazo: dice ser memoria de equipo y nunca va a llegar al equipo", scope)
	}
	pending, sent, dead := 0, 0, 0
	p, s, d, err := e.OutboxStats()
	if err != nil {
		t.Fatalf("OutboxStats: %v", err)
	}
	pending, sent, dead = p, s, d
	if pending+sent+dead != 0 {
		t.Errorf("el outbox tiene %d/%d/%d filas tras un rechazo: se encoló una entrega que el central va a rechazar para siempre", pending, sent, dead)
	}
}

// P3 — UNA OBSERVACIÓN LIMPIA SE SIGUE PROMOVIENDO.
//
// Es la mitad que impide el arreglo fácil: rechazar todo pondría P1 y P2 en verde y dejaría al
// producto sin poder compartir memoria.
func TestP3UnaObservacionLimpiaSeSiguePromoviendo(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	if err := e.SaveObservationTyped("limpia", "t/p3", "una nota perfectamente sana y suficientemente larga", 1.0, "semantic", ScopeLocal, nil); err != nil {
		t.Fatalf("sembrar: %v", err)
	}

	if err := e.PromoteObservation("limpia"); err != nil {
		t.Fatalf("promover una observación limpia falló: %v", err)
	}

	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id = ?`, "limpia").Scan(&scope); err != nil {
		t.Fatalf("releer el scope: %v", err)
	}
	if scope != ScopeShared {
		t.Errorf("la observación limpia quedó en %q en vez de 'shared'", scope)
	}
	// Y encolada: el cambio de scope y la intención de sincronizar son atómicos (F2). Sin este
	// chequeo, una guarda que abortara la transacción entera pasaría el test de scope por el
	// camino equivocado.
	if p, _, _, err := e.OutboxStats(); err != nil || p != 1 {
		t.Errorf("el outbox quedó con %d pendientes (err=%v), esperaba 1: promover sin encolar es 'shared' que nunca viaja", p, err)
	}
}

// P4 — UNA YA-SHARED SIGUE SIENDO UN NO-OP EXITOSO, AUNQUE ARRASTRE EL SOBRE.
//
// Promover una ya-shared es un no-op documentado e idempotente. Hacerlo fallar rompería ese
// contrato para las 73 filas que ya están del otro lado —guardadas antes de que la guarda
// existiera— sin evitar ningún daño, porque el cruce ya ocurrió. Lo que P1 impide es la decisión
// NUEVA de compartir contenido dañado, no el registro de una vieja.
func TestP4UnaYaSharedConElSobreSigueSiendoNoOp(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	sembrarConSobre(t, e, "vieja", conSobre("una que cruzó antes de la guarda", "1.9"), 1.0)
	yaShared(t, e, "vieja")

	if err := e.PromoteObservation("vieja"); err != nil {
		t.Fatalf("promover una ya-shared con el sobre falló: %v — se rompió la idempotencia documentada por una fila que ya cruzó", err)
	}
	var scope string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id = ?`, "vieja").Scan(&scope); err != nil {
		t.Fatalf("releer el scope: %v", err)
	}
	if scope != ScopeShared {
		t.Errorf("la fila quedó en %q: el no-op la degradó", scope)
	}
}
