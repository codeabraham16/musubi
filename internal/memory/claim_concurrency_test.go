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
// runner de Windows, siete de los ocho reclamos volvieron con SQLITE_BUSY (5) y la prueba tardó
// 39 s. Y nadie la había visto nunca en rojo por el doble reclamo que nombra.
//
// Ahora el orden lo fija la prueba. A es otro escritor, que toma el lock y no lo suelta. Mientras
// tanto b1 y b2 reclaman desde dos goroutines, y ninguno puede terminar: ni bien, porque no tiene
// el lock, ni mal, porque lo espera. A confirma, y recién ahí terminan los dos, cada uno con su
// unidad. La ventana de 300 ms sólo decide bajo sabotaje, y ahí decide por reloj: si las goroutines
// eligieran su unidad después de la ventana, el primer sabotaje podría salir verde esa vez. Con el
// DSN sano queda un solo reloj, los 5 s del `busy_timeout`, y ya no hay cola sin orden: A suelta el
// lock una sola vez y cada reclamo es una sentencia corta.
//
// Sabotaje que la hace fallar: elegir la unidad con un SELECT aparte, antes del UPDATE, y
// actualizarla por id sin volver a mirar si sigue libre → b1 y b2 eligen mientras A tiene el lock,
// los dos ven libre la misma, y al soltarlo se la llevan los dos.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1`, batchID, WorkOpen, WorkClaimed).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())"
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

	// A toma el lock de escritura y no lo suelta. Lo toma con una escritura que no cambia nada, y
	// no sólo con el Begin: así la compuerta no depende de `_txlock=immediate`, que custodia X1.
	tx, err := e.db.Begin()
	if err != nil {
		t.Fatalf("Begin de A: %v", err)
	}
	defer tx.Rollback() // en cualquier salida suelta el lock, y los reclamos dejan de esperar
	if _, err := tx.Exec(`UPDATE work_units SET updated_at = updated_at WHERE batch_id = ?`, batch.BatchID); err != nil {
		t.Fatalf("la escritura de A: %v", err)
	}

	// b1 y b2 reclaman por el camino público, que es como reclaman los agentes de verdad.
	type reclamo struct {
		agente string
		u      WorkUnit
		ok     bool
		err    error
	}
	hecho := make(chan reclamo, 2)
	inicio := time.Now()
	for _, agente := range []string{"b1", "b2"} {
		go func(agente string) {
			u, ok, err := e.ClaimWorkUnit(batch.BatchID, agente, 300, 5)
			hecho <- reclamo{agente, u, ok, err}
		}(agente)
	}

	// MIENTRAS A NO SUELTE EL LOCK, NINGÚN RECLAMO PUEDE TERMINAR. Cada salida anticipada tiene su
	// propia línea, porque cada una señala una cosa distinta.
	const ventana = 300 * time.Millisecond
	select {
	case r := <-hecho:
		if esBaseBloqueada(r.err) {
			t.Fatalf("%s se rindió con SQLITE_BUSY a los %v, con el lock de A tomado: no esperó al otro escritor. O al DSN le falta el `busy_timeout`, o vale menos que la ventana — %v", r.agente, time.Since(inicio).Round(time.Millisecond), r.err)
		}
		if r.err != nil {
			t.Fatalf("%s falló con el lock de A tomado, y no por el lock: %v", r.agente, r.err)
		}
		t.Fatalf("%s terminó con el lock de A tomado (ok=%v): reclamar es escribir, así que o A no tenía el lock o el reclamo dejó de escribir", r.agente, r.ok)
	case <-time.After(ventana):
	}

	// A confirma, y con eso suelta el lock.
	if err := tx.Commit(); err != nil {
		t.Fatalf("A no pudo confirmar: %v", err)
	}

	// Los dos, que esperaban, terminan con una unidad cada uno, y no con la misma.
	reclamadaPor := map[string]string{} // id de la unidad → agente que la reclamó
	for i := 0; i < 2; i++ {
		select {
		case r := <-hecho:
			if r.err != nil {
				t.Fatalf("%s no sobrevivió a la espera: %v", r.agente, r.err)
			}
			if !r.ok {
				t.Fatalf("%s no reclamó nada, con las %d unidades del lote libres al empezar", r.agente, units)
			}
			if otro, ya := reclamadaPor[r.u.ID]; ya {
				t.Fatalf("DOBLE RECLAMO: %s y %s se llevaron la misma unidad (seq %d). O se eligió fuera del lock —los dos la vieron libre mientras A escribía—, o el filtro de elegibles deja pasar una unidad reclamada con el lease vigente", otro, r.agente, r.u.Seq)
			}
			reclamadaPor[r.u.ID] = r.agente
		case <-time.After(10 * time.Second):
			t.Fatal("un reclamo siguió esperando 10 s después de que A soltó el lock")
		}
	}

	// El resto del lote se vacía en secuencia: cada unidad sale una sola vez, y al final no queda
	// ninguna.
	for {
		u, ok, err := e.ClaimWorkUnit(batch.BatchID, "c", 300, 5)
		if err != nil {
			t.Fatalf("el reclamo en secuencia falló: %v", err)
		}
		if !ok {
			break
		}
		if otro, ya := reclamadaPor[u.ID]; ya {
			t.Fatalf("la unidad seq %d salió dos veces: para %s y ahora para c", u.Seq, otro)
		}
		reclamadaPor[u.ID] = "c"
	}
	if len(reclamadaPor) != units {
		t.Errorf("se reclamaron %d unidades distintas y el lote tiene %d", len(reclamadaPor), units)
	}
}
