package memory

import (
	"database/sql"
	"testing"
	"time"
)

// EL ENVÍO NO SE DUPLICA NI ESPERA AL TICK (ola 2, frente sync): lo que el engine pone para eso —el
// lease con milisegundos, la pregunta de antes del push y el sondeo sin candado—. El drain entero,
// con dos drainers y un central de juguete, está en internal/mcp (drain_sin_doble_push_test.go).

// esperarFraccion espera a que el reloj de pared esté entre desde y hasta dentro de su segundo. SQLite
// y Go leen el mismo reloj, así que esto ubica el «now» del claim adentro del segundo.
func esperarFraccion(t *testing.T, desde, hasta time.Duration) {
	t.Helper()
	limite := time.Now().Add(3 * time.Second)
	for time.Now().Before(limite) {
		f := time.Duration(time.Now().UnixNano() % int64(time.Second))
		if f >= desde && f < hasta {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("el reloj no pasó por la fracción [%v, %v) en 3 s", desde, hasta)
}

// TestElLeaseDuraLoQueDice: una fila reclamada con un lease de 1 s no se puede volver a reclamar
// medio segundo después, aunque en el medio haya cambiado el segundo. Con el lease escrito con
// datetime('now', '+1 seconds') —que trunca al segundo— un claim a las hh:mm:ss.75 vencía a las
// hh:mm:ss+1, 250 ms después, con el push todavía en vuelo: otro drainer la reclamaba y salía dos
// veces. Con lease_seconds=1 el lease duraba entre cero y un segundo.
//
// El claim se hace pasados los 700 ms del segundo y el segundo intento 400 ms después: ya cambió el
// segundo y el lease sigue vigente. Si el reloj de la máquina se atrasa tanto que el segundo intento
// sale más allá de los 900 ms, la vuelta no mide nada y se repite.
//
// Sabotaje: escribir el lease truncado al segundo, como antes.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="const leaseMs = `strftime('%Y-%m-%d %H:%M:%f', 'now', '+' || ? || ' seconds')`"
// arnes: a="const leaseMs = `datetime('now', '+' || ? || ' seconds')`"
func TestElLeaseDuraLoQueDice(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("lease-1", "t/x", "la nota que viaja", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	for vuelta := 0; vuelta < 3; vuelta++ {
		esperarFraccion(t, 700*time.Millisecond, 750*time.Millisecond)
		primero, err := e.ClaimOutboxBatch(10, 1)
		if err != nil {
			t.Fatal(err)
		}
		reclamado := time.Now()
		if len(primero) != 1 {
			t.Fatalf("el primer claim tenía que traer la nota, trajo %d", len(primero))
		}
		time.Sleep(400 * time.Millisecond)
		segundo, err := e.ClaimOutboxBatch(10, 1)
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(reclamado) > 900*time.Millisecond {
			// La máquina tardó demasiado: el lease pudo vencer de verdad. Se espera a que venza y se repite.
			time.Sleep(time.Second)
			continue
		}
		if len(segundo) != 0 {
			t.Fatalf("ENVÍO DOBLE: %v después del claim con lease de 1 s, otro claim volvió a traer la nota", time.Since(reclamado).Round(time.Millisecond))
		}
		return
	}
	t.Skip("la máquina está tan cargada que en tres vueltas no hubo una medición válida")
}

// TestElSondeoNoTomaElCandadoDeEscritura: el sondeo del drain (HayOutboxPorSubir) contesta aunque
// otro proceso tenga tomado el candado de escritura de la base, y contesta bien: sí con una
// pendiente, no con nada por subir. Corre cada dos segundos en cada daemon —seis por base en
// davantis-1—, así que si tomara el candado competiría con cada guardado de la máquina.
//
// Sabotaje: que el sondeo abra una transacción (con _txlock=immediate, eso toma el candado).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="\tfila := e.db.QueryRow("
// arnes: a="\ttx, terr := e.db.Begin()\n\tif terr != nil {\n\t\treturn false, terr\n\t}\n\tdefer func() { _ = tx.Rollback() }()\n\tfila := tx.QueryRow("
func TestElSondeoNoTomaElCandadoDeEscritura(t *testing.T) {
	e := newTestEngine(t)
	if hay, err := e.HayOutboxPorSubir(); err != nil || hay {
		t.Fatalf("con el outbox vacío el sondeo dio %v (err %v); esperaba que no", hay, err)
	}
	if err := e.SaveObservationTyped("sondeo-1", "t/x", "una nota por subir", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}

	otro, err := sql.Open("sqlite", dsnEscribible(e.path))
	if err != nil {
		t.Fatal(err)
	}
	defer otro.Close()
	tx, err := otro.Begin() // _txlock=immediate: el candado de escritura, tomado por otro proceso
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	type respuesta struct {
		hay bool
		err error
	}
	vino := make(chan respuesta, 1)
	go func() {
		hay, err := e.HayOutboxPorSubir()
		vino <- respuesta{hay, err}
	}()
	select {
	case r := <-vino:
		if r.err != nil || !r.hay {
			t.Fatalf("con una pendiente y el candado ajeno, el sondeo dio %v (err %v); esperaba que sí", r.hay, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EL SONDEO ESPERA EL CANDADO DE ESCRITURA: 2 s sin contestar mientras otro proceso escribe")
	}
	_ = tx.Rollback()

	// Lo que el claim no tomaría tampoco lo ve: reclamada con el lease vigente.
	if _, err := e.ClaimOutboxBatch(10, 60); err != nil {
		t.Fatal(err)
	}
	if hay, err := e.HayOutboxPorSubir(); err != nil || hay {
		t.Errorf("con la única fila reclamada y su lease vigente el sondeo dio %v (err %v); esperaba que no", hay, err)
	}
}

// TestUnReclamoTomadoPorOtroNoEsVigente: la pregunta de antes del push (ReclamoVigente) dice que sí
// sólo a la fila que sigue reclamada por ESE claim y con ESE contenido. Si el lease venció y otro
// drainer la reclamó, o si se editó mientras esperaba su turno en el sublote, dice que no, y ese
// payload no sale.
//
// Sabotaje: que la pregunta no mire de quién es el claim.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="WHERE obs_id = ? AND status = 'claimed' AND next_attempt_at = ? AND ? = `+hashActual+`)`,"
// arnes: a="WHERE obs_id = ? AND status = 'claimed' AND (1 OR next_attempt_at = ?) AND ? = `+hashActual+`)`,"
func TestUnReclamoTomadoPorOtroNoEsVigente(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("vigente-1", "t/x", versionQueViaja, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	items, err := e.ClaimOutboxBatch(10, 1)
	if err != nil || len(items) != 1 {
		t.Fatalf("claim de A: %d ítems, err %v", len(items), err)
	}
	a := items[0]
	if a.Reclamo == "" {
		t.Fatal("el claim no trajo con qué reconocerse (OutboxItem.Reclamo vacío)")
	}
	if ok, err := e.ReclamoVigente(a.ObsID, a.Hash, a.Reclamo); err != nil || !ok {
		t.Fatalf("recién reclamada, la pregunta dio %v (err %v); esperaba que sí", ok, err)
	}

	time.Sleep(1100 * time.Millisecond) // vence el lease de A
	items, err = e.ClaimOutboxBatch(10, 60)
	if err != nil || len(items) != 1 {
		t.Fatalf("claim de B tras vencer el lease: %d ítems, err %v", len(items), err)
	}
	b := items[0]
	if ok, err := e.ReclamoVigente(a.ObsID, a.Hash, a.Reclamo); err != nil || ok {
		t.Errorf("ENVÍO DOBLE: B ya la reclamó y la pregunta de A dio %v (err %v); esperaba que no", ok, err)
	}
	if ok, err := e.ReclamoVigente(b.ObsID, b.Hash, b.Reclamo); err != nil || !ok {
		t.Errorf("la de B dio %v (err %v); esperaba que sí", ok, err)
	}

	editar(t, e, "vigente-1", versionEditada) // se edita mientras B espera su turno
	if ok, err := e.ReclamoVigente(b.ObsID, b.Hash, b.Reclamo); err != nil || ok {
		t.Errorf("editada después del claim, la pregunta de B dio %v (err %v); esperaba que no: su payload es viejo", ok, err)
	}
}

// TestDosEdicionesEnVueloNoLaVuelvenReclamable: dos ediciones mientras v1 viaja. La fila sigue
// retenida para el vuelo de v1 también después de la SEGUNDA: ningún claim la toma y el sondeo no la
// ve, y cuando vuelve el 200 de v1 sale la última. Si la primera edición la dejara 'pending' con el
// lease puesto —retener por el reloj y no por el estado—, la segunda la encontraba 'pending' y la
// ponía en hora, y otro daemon subía v3 a la par de v1: el orden en el central volvía a quedar en
// manos de quién guarda último.
//
// Sabotaje: retener por el reloj y no por el estado (la edición la deja 'pending' con el lease puesto).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="status = CASE WHEN outbox.status = \x27claimed\x27 THEN \x27claimed\x27 ELSE \x27pending\x27 END,"
// arnes: a="status = \x27pending\x27,"
func TestDosEdicionesEnVueloNoLaVuelvenReclamable(t *testing.T) {
	const tercera = "la nota editada otra vez con v1 todavia en vuelo"
	e := newTestEngine(t)
	v1 := reclamada(t, e, "dos-ediciones", versionQueViaja)
	editar(t, e, "dos-ediciones", versionEditada)
	editar(t, e, "dos-ediciones", tercera)

	if hay, err := e.HayOutboxPorSubir(); err != nil || hay {
		t.Errorf("SUBE A LA PAR: con v1 en vuelo y dos ediciones, el sondeo dio %v (err %v); esperaba que no", hay, err)
	}
	items, err := e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("SUBE A LA PAR: con v1 en vuelo y dos ediciones, otro claim se llevó %q; tenía que esperar la vuelta de v1", items[0].Content)
	}

	if err := e.MarkOutboxSent("dos-ediciones", v1.Hash); err != nil {
		t.Fatal(err)
	}
	if it := reclamarUna(t, e, "dos-ediciones"); it.Content != tercera {
		t.Errorf("tras la vuelta de v1 el claim trajo %q; esperaba la última edición", it.Content)
	}
}

// TestUnaEdicionNoEsperaElBackoff: lo que retiene una edición es un vuelo VIGENTE, no el estado de la
// fila ni su turno. v1 falló y espera su backoff; se edita, y v2 sale en el claim siguiente: el
// backoff era de v1 (R3). Y una reclamada cuyo lease ya venció —el drainer que la llevaba murió— no
// tiene vuelo que esperar: la edición sale en el claim siguiente, no un lease más tarde.
//
// Sabotaje: que la edición respete también el turno de una 'pending' (v2 hereda el backoff de v1).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="next_attempt_at = CASE WHEN outbox.status = \x27claimed\x27 THEN"
// arnes: a="next_attempt_at = CASE WHEN outbox.status IN (\x27claimed\x27, \x27pending\x27) THEN"
func TestUnaEdicionNoEsperaElBackoff(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "backoff-1", versionQueViaja)
	if err := e.MarkOutboxRetry("backoff-1", v1.Hash, 3600, "timeout de v1"); err != nil {
		t.Fatal(err)
	}
	if hay, err := e.HayOutboxPorSubir(); err != nil || hay {
		t.Fatalf("precondición: v1 tenía que quedar esperando su backoff; el sondeo dio %v (err %v)", hay, err)
	}
	editar(t, e, "backoff-1", versionEditada)
	items, err := e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Content != versionEditada {
		t.Fatalf("LA EDICIÓN ESPERA EL BACKOFF DE V1: el claim trajo %d ítems; esperaba v2, ya", len(items))
	}

	// La reclamada de un drainer que murió: su lease venció y nadie la va a soltar.
	reclamada(t, e, "muerta-1", versionQueViaja)
	vencerLease(t, e, "muerta-1")
	editar(t, e, "muerta-1", versionEditada)
	items, err = e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ObsID != "muerta-1" || items[0].Content != versionEditada {
		t.Errorf("LA EDICIÓN ESPERA A UN DRAINER MUERTO: con el lease vencido, el claim trajo %d ítems; esperaba la edición de muerta-1", len(items))
	}
}

// TestSoltarReclamoSueltaSoloElPropio: la fila que la pregunta de antes del push rechazó por una
// edición —sigue reclamada por ESTE claim, con el payload viejo en la mano— la suelta el mismo claim,
// y la edición sale en el claim siguiente sin esperar el lease. Pero sólo si sigue siendo de ese
// claim: si el lease venció y otro drainer ya la tomó, soltarla le sacaba el reclamo a ése, y lo que
// lleva podía salir dos veces.
//
// Sabotaje: soltar sin mirar de quién es el reclamo.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="next_attempt_at = ?`, obsID, reclamo)"
// arnes: a="(1 OR next_attempt_at = ?)`, obsID, reclamo)"
func TestSoltarReclamoSueltaSoloElPropio(t *testing.T) {
	e := newTestEngine(t)
	a := reclamada(t, e, "soltar-1", versionQueViaja)
	editar(t, e, "soltar-1", versionEditada) // se edita mientras espera su turno en el sublote
	if ok, err := e.ReclamoVigente(a.ObsID, a.Hash, a.Reclamo); err != nil || ok {
		t.Fatalf("precondición: editada, la pregunta tenía que decir que no; dio %v (err %v)", ok, err)
	}
	if hay, err := e.HayOutboxPorSubir(); err != nil || hay {
		t.Fatalf("precondición: la edición tenía que quedar retenida para este claim; el sondeo dio %v (err %v)", hay, err)
	}
	if err := e.SoltarReclamo(a.ObsID, a.Reclamo); err != nil {
		t.Fatal(err)
	}
	if it := reclamarUna(t, e, "soltar-1"); it.Content != versionEditada {
		t.Errorf("tras soltarla, el claim trajo %q; esperaba la edición", it.Content)
	}

	// A se colgó más que su lease y B la volvió a tomar. B reclama con otro lease para que las dos
	// marcas no coincidan aunque caigan en el mismo milisegundo.
	viejo := reclamada(t, e, "soltar-2", versionQueViaja)
	vencerLease(t, e, "soltar-2")
	items, err := e.ClaimOutboxBatch(50, 30)
	if err != nil || len(items) != 1 {
		t.Fatalf("claim de B tras vencer el lease: %d ítems, err %v", len(items), err)
	}
	b := items[0]
	if err := e.SoltarReclamo(viejo.ObsID, viejo.Reclamo); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.ReclamoVigente(b.ObsID, b.Hash, b.Reclamo); err != nil || !ok {
		t.Errorf("ENVÍO DOBLE: el reclamo vencido de A soltó el de B, que tiene la fila en vuelo (vigente=%v, err %v)", ok, err)
	}
}
