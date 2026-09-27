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
