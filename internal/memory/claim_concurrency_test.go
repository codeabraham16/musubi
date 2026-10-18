package memory

import (
	"database/sql"
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
// veinte unidades, y cualquier error la ponía en rojo. A lo que parece, medía la disponibilidad de
// la máquina, igual que la del ledger (A134) y la de los dos escritores (A142, txlock_test.go): el
// 2026-09-21, en el runner de Windows, siete de las ocho goroutines volvieron con SQLITE_BUSY (5) y
// la prueba tardó 39 s. Y no tenía sabotajes: nunca se había comprobado que cayera por el doble
// reclamo que nombra.
//
// Ahora el orden lo fija la prueba, con una COMPUERTA. A es otro escritor, que toma el lock y no lo
// suelta. Mientras tanto b1 y b2 reclaman desde dos goroutines, y ninguno puede terminar: ni bien,
// porque no tiene el lock, ni mal, porque lo espera. A confirma, y recién ahí terminan los dos. La
// compuerta se pasa dos veces, con los mismos dos agentes, y lo que cambia es lo que escribe A.
// Que sean los mismos importa: en la segunda, b1 y b2 ya tienen una cada uno, y un reclamo que
// vuelva a llevarse la suya, o que informe la que ya tenía en vez de la que escribió, se delata.
//
// En la primera, A no cambia nada: con las veinte libres, b1 y b2 se llevan una cada uno, y no la
// misma. Sola no alcanza: caza el doble reclamo nada más que si los dos eligen la misma unidad.
//
// En la segunda, A reclama para sí las dieciocho que quedan libres —con dueño y lease, como
// AwardWorkUnit— y AGREGA una unidad al lote, como SumarAlLote, en la misma transacción. Cuando
// A confirma, la nueva es la única libre: uno de los dos se la lleva y el otro vuelve sin nada.
// Ésta mira la garantía de frente, porque desde afuera nadie ve la nueva hasta que A confirma, y
// una elección hecha afuera mientras A escribe no puede dar con ella, elija como elija. Si eligió
// una de las dieciocho y la actualiza por id, se la pisa a A; si vuelve a mirar antes de escribir,
// o no eligió ninguna, la nueva queda sin dueño. Las dos salidas son rojas. Pasa la elección de
// afuera que, cuando pierde, vuelve a elegir —adentro del lock o afuera otra vez—, y ésa ya no es
// un defecto si no se rinde mientras quede una libre. La que se rinde después de perder dos veces
// también pasa, porque acá hay una sola libre en disputa: ver lo que no mira. Y lo que A cuenta
// al reclamar —tienen que quedar dieciocho libres— caza además un reclamo de la primera que se
// lleve más de una, o que informe una que no dejó reclamada.
//
// Al final A devuelve las dieciocho, y el lote se vacía en secuencia: cada una de las veintiuna
// sale una sola vez por el camino público, y un reclamo más vuelve sin nada. La segunda compuerta
// dice que se alcanza la última unidad; el vaciado, que se alcanzan todas.
//
// La ventana de 300 ms sólo decide bajo sabotaje, y no en todos. Un sabotaje que elige afuera
// podría salir verde la vez que las goroutines eligieran su unidad después de que A confirma, y el
// que le saca la espera al reclamo, la vez que llegaran después y no chocaran entre ellas; los que
// cambian la elección adentro del lock caen a cualquier hora. Con el DSN sano queda un solo reloj,
// los 5 s del `busy_timeout`, y no hay cola sin orden: A suelta el lock una vez por compuerta, y
// cada reclamo es un SELECT corto —el de las unidades agotadas, que acá no encuentra ninguna— y un
// UPDATE corto. Las carreras de memoria entre los reclamos en vuelo las ve `go test -race`, que
// corren las pruebas automáticas.
//
// Lo que no mira: el orden en que salen las unidades (sin el `ORDER BY seq` sigue verde); la rama
// sin lote, que no recorre, ni un reclamo que cruce de lote, porque hay uno solo; que una unidad
// terminada o fallida vuelva a ser elegible, o que la del lease nulo deje de serlo, que miran
// TestElDeadLetterPersisteLaHistoria y TestUnaUnidadReclamadaSinLeaseNoQuedaTrabadaParaSiempre;
// lo que pasa sólo con tres reclamos en vuelo, porque pone dos, o con dos libres en disputa,
// porque pone una, como una elección que va afuera sólo cuando hay otros dos en vuelo, que la
// versión de ocho goroutines cazaba, o un reclamo que se rinde tras perder dos veces aunque
// quede una libre; y un lease de 0 s, que cae sólo si cambia el segundo entre un reclamo y otro
// que lo mire.
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
// compuerta b1 y b2 casi siempre eligen unidades distintas y pasan; en la segunda eligen entre las
// dieciocho que veían libres mientras A las reclamaba, y le pisan a A las que eligieron.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY random() LIMIT 1`, batchID, WorkOpen, WorkClaimed).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: el SELECT aparte, corrido según el último byte del nombre del
// agente → b1 y b2 eligen unidades distintas y pasan la primera compuerta; en la segunda eligen
// cada uno la suya entre las que veían libres, y le pisan a A las dos.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1 OFFSET ?`, batchID, WorkOpen, WorkClaimed, int(agent[len(agent)-1])%2).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: el SELECT aparte, corrido según el turno de llegada, que cuenta el
// proceso → b1 y b2 llegan en turnos seguidos, eligen unidades distintas y pasan la primera
// compuerta; en la segunda eligen cada uno la suya entre las que veían libres, y le pisan a A las
// dos. El turno se cuenta con un canal y no con sync/atomic, que work.go no importa; y como una
// directiva es un solo reemplazo, el `de` llega hasta el final de la función, donde lo declara.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)\n\t}\n\tu, err := scanWorkUnit(row)\n\tif err == sql.ErrNoRows {\n\t\treturn WorkUnit{}, false, nil\n\t}\n\tif err != nil {\n\t\treturn WorkUnit{}, false, fmt.Errorf(\"error al reclamar unidad: %w\", err)\n\t}\n\treturn u, true, nil\n}\n"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\tturno := <-turnoDelReclamo + 1\n\t\t\t\tturnoDelReclamo <- turno\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1 OFFSET ?`, batchID, WorkOpen, WorkClaimed, turno%2).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())\n\t}\n\tu, err := scanWorkUnit(row)\n\tif err == sql.ErrNoRows {\n\t\treturn WorkUnit{}, false, nil\n\t}\n\tif err != nil {\n\t\treturn WorkUnit{}, false, fmt.Errorf(\"error al reclamar unidad: %w\", err)\n\t}\n\treturn u, true, nil\n}\n\nvar turnoDelReclamo = func() chan int { c := make(chan int, 1); c <- 0; return c }()\n"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: el SELECT aparte, corrido según cuántos reclamos hay en vuelo →
// b1 y b2 están en vuelo a la vez, eligen unidades distintas y pasan la primera compuerta; en la
// segunda eligen cada uno la suya entre las que veían libres, y le pisan a A las dos. La cuenta
// es otro canal, por lo mismo que el turno, y baja apenas vuelve el UPDATE.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)\n\t}\n\tu, err := scanWorkUnit(row)\n\tif err == sql.ErrNoRows {\n\t\treturn WorkUnit{}, false, nil\n\t}\n\tif err != nil {\n\t\treturn WorkUnit{}, false, fmt.Errorf(\"error al reclamar unidad: %w\", err)\n\t}\n\treturn u, true, nil\n}\n"
// arnes: a="WHERE id = ?\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\tenVuelo := <-enVueloDelReclamo\n\t\t\t\tenVueloDelReclamo <- enVuelo + 1\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1 OFFSET ?`, batchID, WorkOpen, WorkClaimed, enVuelo).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}())\n\t}\n\tu, err := scanWorkUnit(row)\n\tenVueloDelReclamo <- <-enVueloDelReclamo - 1\n\tif err == sql.ErrNoRows {\n\t\treturn WorkUnit{}, false, nil\n\t}\n\tif err != nil {\n\t\treturn WorkUnit{}, false, fmt.Errorf(\"error al reclamar unidad: %w\", err)\n\t}\n\treturn u, true, nil\n}\n\nvar enVueloDelReclamo = func() chan int { c := make(chan int, 1); c <- 0; return c }()\n"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: el SELECT aparte al azar, pero volviendo a mirar en el UPDATE si
// la elegida sigue libre → nunca se lleva una unidad ajena; en la segunda compuerta b1 y b2 eligen
// entre las dieciocho que veían libres, al escribir ya son de A, y la nueva queda sin dueño. A
// veces cae antes: si en la primera eligen la misma, el que pierde vuelve sin nada.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, batchID, WorkOpen, WorkClaimed)"
// arnes: a="WHERE id = ? AND `+eligible+`\n\t\t\tRETURNING `+workUnitCols,\n\t\t\tWorkClaimed, agent, agent, ttlSeconds, entrada, func() (elegida string) {\n\t\t\t\t_ = e.db.QueryRow(`SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY random() LIMIT 1`, batchID, WorkOpen, WorkClaimed).Scan(&elegida)\n\t\t\t\treturn elegida\n\t\t\t}(), WorkOpen, WorkClaimed)"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: elegir adentro del lock, pero la segunda elegible y no la primera
// → con veinte libres pasa la primera compuerta; en la segunda la nueva es la única libre, y no
// se la lleva nadie.
// arnes: archivo="internal/memory/work.go"
// arnes: de="`+eligible+` ORDER BY seq LIMIT 1)"
// arnes: a="`+eligible+` ORDER BY seq LIMIT 1 OFFSET 1)"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: que el filtro de elegibles deje pasar las unidades del propio
// agente → en la segunda compuerta b1 y b2 vuelven a llevarse cada uno la que tiene desde la
// primera.
// arnes: archivo="internal/memory/work.go"
// arnes: de="WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)"
// arnes: a="WHERE batch_id=? AND (`+eligible+` OR claimed_by = '`+agent+`') ORDER BY seq LIMIT 1)"
// arnes: colision_ok="TestClaimWorkUnitConcurrentNoDoubleClaim"
//
// Sabotaje que la hace fallar: que la unidad seq 10 no sea elegible nunca → pasa las dos
// compuertas, porque A se la lleva y la devuelve sin reclamarla por el camino público; el vaciado
// vuelve sin nada con veinte de las veintiuna.
// arnes: archivo="internal/memory/work.go"
// arnes: de="`+eligible+` ORDER BY seq LIMIT 1)"
// arnes: a="`+eligible+` AND seq <> 10 ORDER BY seq LIMIT 1)"
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

	// La compuerta: A toma el lock con la escritura que le toca, los agentes reclaman detrás de él
	// por el camino público —que es como reclaman los agentes de verdad—, A confirma y la compuerta
	// devuelve los reclamos en el orden en que terminaron. Cada Fatalf queda acá adentro y nombra
	// la compuerta, para que la línea del rojo diga cuál salida fue.
	compuerta := func(cual string, escribir func(tx *sql.Tx) error, agentes ...string) []reclamo {
		// A toma el lock escribiendo, y no sólo con el Begin: así la compuerta no depende de
		// `_txlock=immediate`, que custodia X1.
		tx, err := e.db.Begin()
		if err != nil {
			t.Fatalf("%s: Begin de A: %v", cual, err)
		}
		defer tx.Rollback() // en cualquier salida suelta el lock, y los reclamos dejan de esperar
		if err := escribir(tx); err != nil {
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

	// PRIMERA COMPUERTA: las veinte libres. A escribe sin cambiar nada, y b1 y b2 se llevan una
	// cada uno, y no la misma.
	primera := compuerta("primera compuerta", func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE work_units SET updated_at = updated_at WHERE batch_id = ?`, batch.BatchID)
		return err
	}, "b1", "b2")
	for _, r := range primera {
		if !r.ok {
			t.Fatalf("primera compuerta: %s no reclamó nada, con las %d unidades del lote libres al empezar. O el otro reclamo se llevó más de una —todas, si la sentencia no se limita a una unidad—, o se eligió fuera del lock y, cuando los dos eligieron la misma, el que perdió volvió a mirar pero no volvió a elegir, o la elección no da con las libres", r.agente, units)
		}
		if otro, ya := reclamadaPor[r.u.ID]; ya {
			t.Fatalf("primera compuerta: DOBLE RECLAMO: %s y %s se llevaron la misma unidad (seq %d). O se eligió fuera del lock —los dos la vieron libre mientras A escribía—, o el filtro de elegibles deja pasar una unidad reclamada con el lease vigente, o el primero escribió un lease que ya estaba vencido, o un reclamo informó una unidad que no es la que escribió", otro, r.agente, r.u.Seq)
		}
		reclamadaPor[r.u.ID] = r.agente
	}

	// SEGUNDA COMPUERTA: A reclama para sí las dieciocho libres y agrega una unidad al lote, como
	// SumarAlLote, en la misma transacción. Cuando confirma, la nueva es la única libre: uno de
	// los dos se la lleva y el otro vuelve sin nada. A deja cada reclamo como lo deja
	// AwardWorkUnit: estado, dueño, lease, latido, intento, token y la hora del cambio, y sin
	// bitácora. Una reclamada sin dueño, o con el latido más nuevo que la hora del cambio, es un
	// estado que la API no produce, y un filtro que la rescatara pondría esta prueba en rojo sin
	// ser un defecto. Una reclamada sin bitácora, en cambio, la deja AwardWorkUnit, y un filtro
	// que la rescate sí es un defecto: se lleva una adjudicada con el lease vigente. La nueva se
	// busca por su título, que no se repite en el lote, y no por su seq, que se repetiría si
	// CreateWorkBatch numerara desde 1.
	var robadas int64
	var nueva string
	segunda := compuerta("segunda compuerta", func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE work_units SET status = ?, owner_id = 'a', claimed_by = 'a', lease_expires_at = datetime('now', '+1 hour'), heartbeat_at = datetime('now'), attempts = attempts + 1, fencing_token = fencing_token + 1, updated_at = datetime('now') WHERE batch_id = ? AND status = ?`,
			WorkClaimed, batch.BatchID, WorkOpen)
		if err != nil {
			return err
		}
		if robadas, err = res.RowsAffected(); err != nil {
			return err
		}
		if err := insertarUnidades(tx, batch.BatchID, units, []WorkUnitSpec{{Title: "nueva", Spec: "hacer algo"}}); err != nil {
			return err
		}
		return tx.QueryRow(`SELECT id FROM work_units WHERE batch_id = ? AND title = ?`, batch.BatchID, "nueva").Scan(&nueva)
	}, "b1", "b2")
	if robadas != units-2 {
		t.Fatalf("segunda compuerta: A encontró %d unidades libres y tenían que quedar %d. Si son menos, algún reclamo de la primera se llevó más de una; si son más, alguno informó una unidad que no dejó reclamada", robadas, units-2)
	}
	var ganadores []reclamo
	for _, r := range segunda {
		if !r.ok {
			continue
		}
		if r.u.ID != nueva {
			if duenio, ya := reclamadaPor[r.u.ID]; ya {
				t.Fatalf("segunda compuerta: RE-RECLAMO: %s se llevó la unidad seq %d, que %s tiene desde la primera compuerta. O el filtro de elegibles deja pasar una unidad reclamada con el lease vigente, o el reclamo de la primera escribió un lease que ya estaba vencido, o este reclamo informó una unidad que no es la que escribió", r.agente, r.u.Seq, duenio)
			}
			// Éste no nombra un lease vencido, como los demás: el de las dieciocho lo escribe A,
			// con una hora, y no pasa por el reclamo.
			t.Fatalf("segunda compuerta: ROBO: %s se llevó la unidad seq %d, que A había reclamado para sí, y ahora la tienen los dos: un doble reclamo. O se eligió fuera del lock —entre las que se veían libres mientras A escribía— y se la actualizó por id sin volver a mirar, o el filtro de elegibles deja pasar una unidad reclamada con el lease vigente, o el reclamo informó una unidad que no es la que escribió", r.agente, r.u.Seq)
		}
		ganadores = append(ganadores, r)
	}
	switch len(ganadores) {
	case 0:
		t.Fatalf("segunda compuerta: NADIE se llevó la unidad que agregó A (seq %d), la única libre cuando A soltó el lock. O se eligió fuera del lock —entre lo que se veía libre mientras A escribía, donde la nueva todavía no estaba— y al escribir no quedaba nada que llevarse, o la elección no da con la única libre", units)
	case 2:
		t.Fatalf("segunda compuerta: DOBLE RECLAMO de la unidad que agregó A (seq %d): se la llevaron %s y %s. O el filtro de elegibles deja pasar una unidad reclamada con el lease vigente, o el primero escribió un lease que ya estaba vencido, o uno de los dos informó una unidad que no es la que escribió", units, ganadores[0].agente, ganadores[1].agente)
	}
	reclamadaPor[nueva] = ganadores[0].agente

	// A devuelve las dieciocho como las deja ReopenWorkUnit: libres, sin dueño, sin lease y con los
	// intentos en cero.
	res, err := e.db.Exec(`UPDATE work_units SET status = ?, owner_id = NULL, claimed_by = NULL, lease_expires_at = NULL, attempts = 0 WHERE batch_id = ? AND owner_id = 'a'`,
		WorkOpen, batch.BatchID)
	if err != nil {
		t.Fatalf("la devolución de A: %v", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != units-2 {
		t.Fatalf("la devolución de A: devolvió %d unidades y se había llevado %d (err=%v)", n, units-2, err)
	}

	// EL VACIADO: las dieciocho que devolvió A salen en secuencia, cada una una sola vez. Con las
	// de b1 y b2 y la nueva, son las veintiuna del lote.
	for len(reclamadaPor) < units+1 {
		u, ok, err := e.ClaimWorkUnit(batch.BatchID, "c", 300, 5)
		if err != nil {
			t.Fatalf("vaciado: el reclamo en secuencia falló: %v", err)
		}
		if !ok {
			t.Fatalf("vaciado: el reclamo en secuencia volvió sin nada, con %d de las %d unidades reclamadas: alguna libre no se alcanza", len(reclamadaPor), units+1)
		}
		if otro, ya := reclamadaPor[u.ID]; ya {
			t.Fatalf("vaciado: la unidad seq %d salió dos veces, para %s y ahora para c. O el filtro de elegibles deja pasar una unidad reclamada con el lease vigente, o el reclamo anterior escribió un lease que ya estaba vencido, o el reclamo informó una unidad que no es la que escribió", u.Seq, otro)
		}
		reclamadaPor[u.ID] = "c"
	}

	// Ya no queda ninguna: un reclamo más vuelve sin unidad y sin error.
	if u, ok, err := e.ClaimWorkUnit(batch.BatchID, "c", 300, 5); err != nil || ok {
		t.Fatalf("con las %d unidades reclamadas, un reclamo más tenía que volver sin nada y sin error: ok=%v seq=%d err=%v", units+1, ok, u.Seq, err)
	}
}
