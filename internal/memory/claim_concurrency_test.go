package memory

import (
	"fmt"
	"testing"
	"time"
)

// TestClaimWorkUnitConcurrentNoDoubleClaim — DOS RECLAMOS EN VUELO NO SE LLEVAN LA MISMA UNIDAD.
//
// La garantía es de ClaimWorkUnit (work.go): la unidad se elige ADENTRO del UPDATE...RETURNING, y
// esa sentencia corre con el lock de escritura tomado, así que el segundo reclamo ya ve la unidad
// que se llevó el primero. Tiene dos mitades, y cada una se rompe por su lado: que el reclamo
// ESPERE al otro escritor en vez de fallar —el `busy_timeout` del DSN— y que no REPITA unidad.
//
// ES DETERMINISTA A PROPÓSITO (A143). La versión anterior eran ocho goroutines en bucle reclamando
// veinte unidades, y cualquier error la ponía en rojo. Medía la disponibilidad de la máquina, igual
// que la del ledger (A134) y la de los dos escritores (A142, txlock_test.go): el 2026-09-21, en el
// runner de Windows, siete de las ocho goroutines volvieron con SQLITE_BUSY (5) y la prueba tardó
// 39 s. Y no tenía sabotajes: nunca se había comprobado que cayera por el doble reclamo que nombra.
//
// Ahora el orden lo fija la prueba, con una COMPUERTA. A es otro escritor, que toma el lock y no lo
// suelta. Mientras tanto dos reclamos esperan en dos goroutines, y ninguno puede terminar: ni bien,
// porque no tiene el lock, ni mal, porque lo espera. A confirma, y recién ahí terminan los dos. La
// compuerta se pasa dos veces. La primera, con las veinte unidades libres y dos agentes distintos,
// que se llevan una cada uno. Después el lote se vacía en secuencia hasta que queda UNA libre, y la
// segunda compuerta la disputan dos reclamos con el MISMO nombre: uno se la lleva y el otro vuelve
// sin nada.
//
// La segunda compuerta existe porque la primera sola caza el doble reclamo nada más que si los dos
// eligen la misma unidad. Una elección hecha fuera del lock que no tome la primera por orden —al
// azar, o corrida según el agente— deja a cada uno con una distinta, y la primera pasa en verde.
// Con una sola libre no hay otra que elegir, y el mismo nombre cierra lo que queda: una elección que
// dependa del agente elige lo mismo para los dos. La primera tampoco sobra: con dos nombres
// distintos, es la única que ve a un reclamo llevarse una unidad que otro agente tiene con el lease
// vigente.
//
// La ventana de 300 ms sólo decide bajo sabotaje, y ahí decide por reloj, en todos los sabotajes:
// si las goroutines llegaran a elegir su unidad, o a chocar con el lock de A, después de la
// ventana, el sabotaje podría salir verde esa vez. Con el DSN sano queda un solo reloj, los 5 s del
// `busy_timeout`, y ya no hay cola sin orden: A suelta el lock una vez por compuerta, y cada reclamo
// es un SELECT corto —el de las unidades agotadas, que acá no encuentra ninguna— y un UPDATE corto.
// Las carreras de memoria entre los reclamos en vuelo las ve `go test -race`, que corren las
// pruebas automáticas.
//
// Sabotaje que la hace fallar: elegir la unidad con un SELECT aparte, antes del UPDATE, y
// actualizarla por id sin volver a mirar si sigue libre → en la primera compuerta b1 y b2 eligen
// mientras A tiene el lock, los dos ven libre la misma, y al soltarlo se la llevan los dos.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1`, batchID, WorkOpen, WorkClaimed).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: el mismo SELECT aparte, pero eligiendo al azar → en la primera
// compuerta b1 y b2 casi siempre eligen unidades distintas y pasan; en la segunda hay una sola para
// elegir, y los dos reclamos de d se la llevan.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY random() LIMIT 1`, batchID, WorkOpen, WorkClaimed).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: el SELECT aparte, corrido según el último byte del nombre del
// agente → b1 y b2 eligen unidades distintas y pasan la primera compuerta; en la segunda, los dos
// reclamos se llaman d, eligen la misma, y se la llevan los dos. Es el que custodia que la segunda
// compuerta use un solo nombre: con dos distintos, uno de ellos no elegiría nada y pasaría en verde.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1 OFFSET ?`, batchID, WorkOpen, WorkClaimed, int(agent[len(agent)-1])%2).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: sacar `busy_timeout(5000)` del DSN → el reclamo choca con el lock
// de A y vuelve al instante con «database is locked». Pisa el literal que también sabotean X1 y
// SA5, y las tres lo declaran: son guardas distintas sobre la misma línea —ésta, el reclamo, que
// escribe sin abrir transacción; X1, el alta; SA5, el ledger del motor liviano—.
// arnes: archivo="internal/memory/database.go"
// arnes: de="_pragma=busy_timeout(5000)&"
// arnes: a=""
// arnes: colision_ok="TestX1DosEscritoresConcurrentesNoSeMatan TestSinArranqueElLedgerEsperaAlOtroEscritor"
func TestClaimWorkUnitConcurrentNoDoubleClaim(t *testing.T) {
	e := newTestEngine(t)

	const units = 20
	specs := make([]WorkUnitSpec, units)
	for i := range specs {
		specs[i] = WorkUnitSpec{Title: fmt.Sprintf("u%d", i), Spec: "hacer algo"}
	}
	batch, err := e.CreateWorkBatch("b", specs)
	if err != nil {
		t.Fatalf("CreateWorkBatch: %v", err)
	}
	// Que no quede trabajo de fondo del arranque: el único otro escritor tiene que ser A.
	e.bgWG.Wait()

	type reclamo struct {
		agente string
		u      WorkUnit
		ok     bool
		err    error
	}
	const ventana = 300 * time.Millisecond

	// La compuerta: A toma el lock de escritura, los agentes reclaman detrás de él por el camino
	// público —que es como reclaman los agentes de verdad—, A confirma y la compuerta devuelve los
	// reclamos en el orden en que terminaron. Cada Fatalf queda acá adentro y nombra la compuerta,
	// para que la línea del rojo diga cuál de las salidas fue.
	compuerta := func(cual string, agentes ...string) []reclamo {
		// A toma el lock con una escritura que no cambia nada, y no sólo con el Begin: así la
		// compuerta no depende de `_txlock=immediate`, que custodia X1.
		tx, err := e.db.Begin()
		if err != nil {
			t.Fatalf("%s: Begin de A: %v", cual, err)
		}
		defer tx.Rollback() // en cualquier salida suelta el lock, y los reclamos dejan de esperar
		if _, err := tx.Exec(`UPDATE work_units SET updated_at = updated_at WHERE batch_id = ?`, batch.BatchID); err != nil {
			t.Fatalf("%s: la escritura de A: %v", cual, err)
		}

		hecho := make(chan reclamo, len(agentes))
		inicio := time.Now()
		for _, agente := range agentes {
			go func(agente string) {
				u, ok, err := e.ClaimWorkUnit(batch.BatchID, agente, 300, 5)
				hecho <- reclamo{agente, u, ok, err}
			}(agente)
		}

		// MIENTRAS A NO SUELTE EL LOCK, NINGÚN RECLAMO PUEDE TERMINAR. Cada salida anticipada tiene
		// su propia línea, porque cada una señala una cosa distinta.
		select {
		case r := <-hecho:
			if esBaseBloqueada(r.err) {
				t.Fatalf("%s: %s se rindió con SQLITE_BUSY a los %v, con el lock de A tomado: no esperó al otro escritor. O al DSN le falta el `busy_timeout`, o vale menos que la ventana, o el camino del reclamo abre su transacción como lectora y la sube a escritora, un BUSY que `busy_timeout` no espera — %v", cual, r.agente, time.Since(inicio).Round(time.Millisecond), r.err)
			}
			if r.err != nil {
				t.Fatalf("%s: %s falló con el lock de A tomado, y no por el lock: %v", cual, r.agente, r.err)
			}
			t.Fatalf("%s: %s terminó con el lock de A tomado (ok=%v): reclamar es escribir, así que o A no tenía el lock o el reclamo dejó de escribir", cual, r.agente, r.ok)
		case <-time.After(ventana):
		}

		// A confirma, y con eso suelta el lock.
		if err := tx.Commit(); err != nil {
			t.Fatalf("%s: A no pudo confirmar: %v", cual, err)
		}

		var terminados []reclamo
		for range agentes {
			select {
			case r := <-hecho:
				if r.err != nil {
					t.Fatalf("%s: %s no sobrevivió a la espera: %v", cual, r.agente, r.err)
				}
				terminados = append(terminados, r)
			case <-time.After(10 * time.Second):
				t.Fatalf("%s: un reclamo siguió esperando 10 s después de que A soltó el lock", cual)
			}
		}
		return terminados
	}

	reclamadaPor := map[string]string{} // id de la unidad → agente que la reclamó

	// PRIMERA COMPUERTA: las veinte libres y dos agentes distintos. Cada uno se lleva una, y no la
	// misma.
	for _, r := range compuerta("primera compuerta", "b1", "b2") {
		if !r.ok {
			t.Fatalf("primera compuerta: %s no reclamó nada, con las %d unidades del lote libres al empezar", r.agente, units)
		}
		if otro, ya := reclamadaPor[r.u.ID]; ya {
			t.Fatalf("DOBLE RECLAMO: %s y %s se llevaron la misma unidad (seq %d). O se eligió fuera del lock —los dos la vieron libre mientras A escribía—, o el filtro de elegibles deja pasar una unidad reclamada con el lease vigente", otro, r.agente, r.u.Seq)
		}
		reclamadaPor[r.u.ID] = r.agente
	}

	// El lote se vacía en secuencia hasta que queda UNA libre. Cada reclamo sale bien y con una
	// unidad que nadie se llevó antes.
	for len(reclamadaPor) < units-1 {
		u, ok, err := e.ClaimWorkUnit(batch.BatchID, "c", 300, 5)
		if err != nil {
			t.Fatalf("el reclamo en secuencia falló: %v", err)
		}
		if !ok {
			t.Fatalf("el reclamo en secuencia volvió sin nada, con %d de las %d unidades reclamadas", len(reclamadaPor), units)
		}
		if otro, ya := reclamadaPor[u.ID]; ya {
			t.Fatalf("la unidad seq %d salió dos veces: para %s y ahora para c", u.Seq, otro)
		}
		reclamadaPor[u.ID] = "c"
	}

	// SEGUNDA COMPUERTA: una sola libre y dos reclamos con el MISMO nombre. Uno se la lleva y el
	// otro vuelve sin nada.
	var ganadores []reclamo
	for _, r := range compuerta("segunda compuerta", "d", "d") {
		if r.ok {
			ganadores = append(ganadores, r)
		}
	}
	switch {
	case len(ganadores) == 2 && ganadores[0].u.ID == ganadores[1].u.ID:
		t.Fatalf("DOBLE RECLAMO de la última unidad libre (seq %d): los dos reclamos de d se la llevaron. Se eligió fuera del lock: con una sola libre cualquier elección la ve a ella, y los dos la vieron libre mientras A escribía", ganadores[0].u.Seq)
	case len(ganadores) == 2:
		t.Fatalf("con una sola unidad libre, los dos reclamos de d se llevaron una cada uno (seq %d y %d): el filtro de elegibles deja pasar una unidad reclamada con el lease vigente", ganadores[0].u.Seq, ganadores[1].u.Seq)
	case len(ganadores) == 0:
		t.Fatal("ninguno de los dos reclamos de d se llevó la última unidad libre: o el reclamo no la ve, o la elección depende del agente y a d no le toca ninguna")
	}
	if otro, ya := reclamadaPor[ganadores[0].u.ID]; ya {
		t.Fatalf("la unidad seq %d salió dos veces: para %s y ahora para d, que tenía que llevarse la última libre", ganadores[0].u.Seq, otro)
	}

	// Ya no queda ninguna: un reclamo más vuelve sin unidad y sin error.
	if u, ok, err := e.ClaimWorkUnit(batch.BatchID, "c", 300, 5); err != nil || ok {
		t.Fatalf("con las %d unidades reclamadas, un reclamo más tenía que volver sin nada y sin error: ok=%v seq=%d err=%v", units, ok, u.Seq, err)
	}
}
