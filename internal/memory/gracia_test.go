package memory

import (
	"fmt"
	"testing"
	"time"
)

// La gracia desde la llegada: una nota bajada del central trae la fecha en que el central la recibió
// (fechaDeOrigen), y el olvido y la cuota cuentan su edad desde que LLEGÓ acá —el sello 'espejo'
// del outbox— si llegó después. Sin esto, el primer mantenimiento de un cliente nuevo archivaba lo
// que acababa de bajar, antes de que nadie lo pudiera usar.

// opcionesDeOlvido son los defaults de producción (config.go), con el refuerzo encendido.
var opcionesDeOlvido = DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14, ReinforcementK: 0.5}

// sembrarBajada baja una nota por el camino real (IngestShared), con la fecha de origen de hace
// `dias` días. Exige que la fecha se haya guardado tal cual y que el sello sea 'espejo': si
// fechaDeOrigen la hubiera cambiado por la de la bajada (piso, reloj), la prueba no probaría nada.
func sembrarBajada(t *testing.T, e *DbEngine, id, proyecto string, dias int, importancia float64) {
	t.Helper()
	origen := time.Now().UTC().AddDate(0, 0, -dias).Format(sqliteTimeLayout)
	if _, err := e.IngestShared(SharedObs{ID: id, TopicKey: "t/gracia", Content: "la nota bajada " + id,
		Importance: importancia, ProjectID: proyecto, CreatedAt: origen}); err != nil {
		t.Fatal(err)
	}
	if got := fechaGuardada(t, e, id); got != origen {
		t.Fatalf("precondición: %s se guardó con %q y no con su fecha de origen %q", id, got, origen)
	}
	var estado string
	if err := e.db.QueryRow(`SELECT status FROM outbox WHERE obs_id = ?`, id).Scan(&estado); err != nil {
		t.Fatalf("precondición: %s no dejó fila en el outbox: %v", id, err)
	}
	if estado != outboxEspejo {
		t.Fatalf("precondición: el sello de %s es %q, no %q", id, estado, outboxEspejo)
	}
}

// moverLlegada corre el sello de llegada de una nota a hace `dias` días.
func moverLlegada(t *testing.T, e *DbEngine, id string, dias int) {
	t.Helper()
	if _, err := e.db.Exec(`UPDATE outbox SET created_at = datetime('now', ?) WHERE obs_id = ?`,
		fmt.Sprintf("-%d days", dias), id); err != nil {
		t.Fatal(err)
	}
}

// sellarEnviada deja a una nota propia con una fila 'sent' recién creada: la que dejó, por ejemplo,
// BackfillOutbox al encolar hoy una nota vieja. Su created_at es de hoy y NO es una llegada.
func sellarEnviada(t *testing.T, e *DbEngine, id string) {
	t.Helper()
	if _, err := e.db.Exec(
		`INSERT OR REPLACE INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at)
		 VALUES (?, 'h', 'sent', 0, datetime('now'), datetime('now'), datetime('now'))`, id); err != nil {
		t.Fatal(err)
	}
}

// envejecerSello deja el sello de una nota como si hubiera llegado hace `dias` días y nadie lo
// hubiera vuelto a tocar: created_at y updated_at, las dos columnas.
func envejecerSello(t *testing.T, e *DbEngine, id string, dias int) {
	t.Helper()
	hace := fmt.Sprintf("-%d days", dias)
	if _, err := e.db.Exec(`UPDATE outbox SET created_at = datetime('now', ?), updated_at = datetime('now', ?) WHERE obs_id = ?`,
		hace, hace, id); err != nil {
		t.Fatal(err)
	}
}

// reentregar vuelve a bajar la misma nota, igual, por el camino real (IngestShared): una re-bajada del
// central, o el rebote de lo que ya estaba. Exige que el UPSERT del sello haya corrido (updated_at de
// hoy): si no, la prueba no distinguiría leer la llegada de created_at o de updated_at.
func reentregar(t *testing.T, e *DbEngine, id, proyecto string, importancia float64) {
	t.Helper()
	if _, err := e.IngestShared(SharedObs{ID: id, TopicKey: "t/gracia", Content: "la nota bajada " + id,
		Importance: importancia, ProjectID: proyecto}); err != nil {
		t.Fatal(err)
	}
	var refrescado bool
	if err := e.db.QueryRow(`SELECT updated_at >= datetime('now','-1 hour') FROM outbox WHERE obs_id = ?`, id).Scan(&refrescado); err != nil {
		t.Fatal(err)
	}
	if !refrescado {
		t.Fatalf("precondición: la re-bajada de %s no refrescó su sello", id)
	}
}

// TestLaNotaBajadaTieneGraciaDesdeQueLlega: una nota que el central recibió hace 60 días y que
// acaba de bajar no se archiva en el primer mantenimiento. Una propia con las MISMAS fechas y la
// misma importancia sí, aunque tenga una fila 'sent' de hoy: sólo el sello 'espejo' es una llegada.
// Saliencia de las dos contando desde el origen: 0,5 × 0,5^(60/30) = 0,125, bajo el umbral de 0,2.
//
// Sabotaje: el olvido no lee el sello (la edad vuelve a contar desde el origen).
// arnes: archivo="internal/memory/decay.go"
// arnes: de="COALESCE(esp.created_at,\x27\x27)"
// arnes: a="\x27\x27"
// arnes: colision_ok="TestUnaRebajadaNoRenuevaLaGracia"
//
// Sabotaje: el olvido le da la gracia a cualquier fila del outbox, también a la 'sent'.
// arnes: archivo="internal/memory/decay.go"
// arnes: de="AND esp.status = \x27espejo\x27"
// arnes: a="AND esp.status IN (\x27espejo\x27, \x27sent\x27)"
//
// Sabotaje: el olvido sólo recorre las notas con sello (JOIN en vez de LEFT JOIN), y la propia ni se mira.
// arnes: archivo="internal/memory/decay.go"
// arnes: de="LEFT JOIN outbox esp"
// arnes: a="JOIN outbox esp"
func TestLaNotaBajadaTieneGraciaDesdeQueLlega(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "", 60, 0.5)

	if err := e.SaveObservation("propia", "t/gracia", "una nota escrita acá hace 60 días", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-60 days'), importance = 0.5 WHERE id = 'propia'`); err != nil {
		t.Fatal(err)
	}
	sellarEnviada(t, e, "propia")

	res, err := e.Decay(opcionesDeOlvido)
	if err != nil {
		t.Fatal(err)
	}
	if isArchived(t, e, "bajada") {
		t.Error("la nota que acaba de bajar se archivó: su edad contó desde el origen y no desde la llegada")
	}
	if !isArchived(t, e, "propia") {
		t.Error("la nota propia de hace 60 días no se archivó: una fila 'sent' de hoy le dio una gracia que no es suya, o el olvido no la recorrió")
	}
	if res.Archived != 1 {
		t.Errorf("se archivaron %d; esperaba 1 (sólo la propia)", res.Archived)
	}
}

// TestLaNotaBajadaSeEnfriaDesdeQueLlego: la llegada no es sólo una ventana de 14 días; la saliencia
// también cuenta desde ahí. Una nota con 200 días de origen que llegó hace 15 ya pasó la edad mínima,
// y su saliencia es 1 × 0,5^(15/30) = 0,71: se queda. Contada desde el origen sería 0,01.
//
// Sabotaje: la gracia es sólo una ventana; pasados 14 días de la llegada, la nota vuelve a su edad de origen.
// arnes: archivo="internal/memory/decay.go"
// arnes: de="return l, true"
// arnes: a="return t, time.Since(l) >= 14*24*time.Hour"
func TestLaNotaBajadaSeEnfriaDesdeQueLlego(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "", 200, 1)
	moverLlegada(t, e, "bajada", 15)

	if _, err := e.Decay(opcionesDeOlvido); err != nil {
		t.Fatal(err)
	}
	if isArchived(t, e, "bajada") {
		t.Error("la nota que llegó hace 15 días se archivó: la saliencia se calculó con la edad de origen")
	}
}

// TestLaNotaBajadaYUsadaCuentaDesdeElUso: la gracia toma el instante MÁS NUEVO, no reemplaza al uso.
// Una nota que llegó hace 60 días y se usó hace 5 tiene 5 días. Si la llegada pisara al uso, tendría
// 60, y con importancia 0,2 y un acceso su saliencia sería 0,2 × 1,69 × 0,5^(60/40,4) = 0,12.
//
// Sabotaje: el sello reemplaza a la fecha de la nota aunque sea anterior.
// arnes: archivo="internal/memory/decay.go"
// arnes: de="if err == nil && l.After(t) {"
// arnes: a="if err == nil {"
func TestLaNotaBajadaYUsadaCuentaDesdeElUso(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "", 90, 0.2)
	moverLlegada(t, e, "bajada", 60)
	if _, err := e.db.Exec(`UPDATE observations SET last_accessed = datetime('now','-5 days'), access_count = 1 WHERE id = 'bajada'`); err != nil {
		t.Fatal(err)
	}

	if _, err := e.Decay(opcionesDeOlvido); err != nil {
		t.Fatal(err)
	}
	if isArchived(t, e, "bajada") {
		t.Error("la nota usada hace 5 días se archivó: la llegada de hace 60 pisó al uso")
	}
}

// TestUnaFechaIlegibleNoSeArchivaPorElSello: una nota cuya fecha no se puede leer nunca se archivó,
// y la gracia no cambia eso aunque el sello sí se lea. En el olvido, la gracia sólo puede archivar MENOS.
//
// Sabotaje: si la fecha de la nota no se lee, usar el sello.
// arnes: archivo="internal/memory/decay.go"
// arnes: de="return time.Time{}, false"
// arnes: a="t = time.Time{}"
func TestUnaFechaIlegibleNoSeArchivaPorElSello(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "", 60, 1)
	if _, err := e.db.Exec(`UPDATE observations SET created_at = 'no-es-una-fecha', last_accessed = NULL WHERE id = 'bajada'`); err != nil {
		t.Fatal(err)
	}
	moverLlegada(t, e, "bajada", 400)

	if _, err := e.Decay(opcionesDeOlvido); err != nil {
		t.Fatal(err)
	}
	if isArchived(t, e, "bajada") {
		t.Error("la nota con la fecha ilegible se archivó por la edad de su sello")
	}
}

// TestLaCuotaNoDesalojaLoQueRecienBajo: la cuota cuenta la edad igual que el olvido. Con techo 1 y
// tres activas en el proyecto, desaloja dos: la propia (100 días) y la enviada (150 días, con una
// fila 'sent' de hoy). La bajada tiene 200 días de origen —la más fría si se contara desde ahí— pero
// acaba de llegar y está en su gracia.
//
// Sin la gracia también desalojaba dos, pero OTRAS: la bajada y la enviada. La cuota saca un
// excedente fijo, así que la gracia no desaloja menos: corre la víctima a la siguiente más fría, que
// acá es la propia, una nota local sin copia en el central. Es lo decidido (ver edadDesde).
//
// Sabotaje: la cuota no lee el sello.
// arnes: archivo="internal/memory/quota.go"
// arnes: de="COALESCE(esp.created_at,\x27\x27)"
// arnes: a="\x27\x27"
// arnes: colision_ok="TestUnaRebajadaNoRenuevaLaGraciaDeLaCuota"
//
// Sabotaje: la cuota le da la gracia también a la fila 'sent'.
// arnes: archivo="internal/memory/quota.go"
// arnes: de="AND esp.status = \x27espejo\x27"
// arnes: a="AND esp.status IN (\x27espejo\x27, \x27sent\x27)"
//
// Sabotaje: la cuota sólo recorre las notas con sello (JOIN en vez de LEFT JOIN): la propia y la enviada dejan de ser candidatas.
// arnes: archivo="internal/memory/quota.go"
// arnes: de="LEFT JOIN outbox esp"
// arnes: a="JOIN outbox esp"
func TestLaCuotaNoDesalojaLoQueRecienBajo(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "p", 200, 1)
	seedQuotaObs(t, e, "propia", "p", 100, 1, 0)
	seedQuotaObs(t, e, "enviada", "p", 150, 1, 0)
	sellarEnviada(t, e, "enviada")

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 1, HalfLifeDays: 30, MinAgeDays: 14, ReinforcementK: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if isArchived(t, e, "bajada") {
		t.Error("la cuota desalojó la nota que acaba de bajar: su edad contó desde el origen")
	}
	for _, id := range []string{"propia", "enviada"} {
		if !isArchived(t, e, id) {
			t.Errorf("%s no se desalojó", id)
		}
	}
	if n != 2 {
		t.Errorf("la cuota desalojó %d; esperaba 2", n)
	}
}

// TestUnaRebajadaNoRenuevaLaGracia: la gracia cuenta desde la PRIMERA llegada. Una nota que llegó hace
// 60 días y que el central vuelve a entregar hoy no vuelve a ser nueva: si cada re-bajada renovara la
// gracia, lo que el central re-entrega no se olvidaría mientras volviera a bajar antes de enfriarse.
// Saliencia desde la llegada: 0,5 × 0,5^(60/30) = 0,125, bajo el umbral de 0,2.
//
// Sabotaje: el olvido lee la llegada de updated_at, que la re-bajada sí refresca.
// arnes: archivo="internal/memory/decay.go"
// arnes: de="COALESCE(esp.created_at,\x27\x27)"
// arnes: a="COALESCE(esp.updated_at,\x27\x27)"
// arnes: colision_ok="TestLaNotaBajadaTieneGraciaDesdeQueLlega"
//
// Sabotaje: el UPSERT del sello renueva created_at en cada re-bajada.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="ELSE \x27espejo\x27 END, attempts = 0, last_error = NULL,"
// arnes: a="ELSE \x27espejo\x27 END, attempts = 0, last_error = NULL, created_at = datetime(\x27now\x27),"
func TestUnaRebajadaNoRenuevaLaGracia(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "", 60, 0.5)
	envejecerSello(t, e, "bajada", 60)
	reentregar(t, e, "bajada", "", 0.5)

	if _, err := e.Decay(opcionesDeOlvido); err != nil {
		t.Fatal(err)
	}
	if !isArchived(t, e, "bajada") {
		t.Error("la nota que llegó hace 60 días no se archivó: volver a bajarla le renovó la gracia")
	}
}

// TestUnaRebajadaNoRenuevaLaGraciaDeLaCuota: lo mismo en la cuota. Con techo 1 y dos activas sale la
// más fría: la bajada que llegó hace 200 días (0,01), y no la propia de 100 (0,1), aunque el central
// la haya vuelto a entregar hoy.
//
// Sabotaje: la cuota lee la llegada de updated_at, que la re-bajada sí refresca.
// arnes: archivo="internal/memory/quota.go"
// arnes: de="COALESCE(esp.created_at,\x27\x27)"
// arnes: a="COALESCE(esp.updated_at,\x27\x27)"
// arnes: colision_ok="TestLaCuotaNoDesalojaLoQueRecienBajo"
func TestUnaRebajadaNoRenuevaLaGraciaDeLaCuota(t *testing.T) {
	e := newTestEngine(t)
	sembrarBajada(t, e, "bajada", "p", 200, 1)
	envejecerSello(t, e, "bajada", 200)
	reentregar(t, e, "bajada", "p", 1)
	seedQuotaObs(t, e, "propia", "p", 100, 1, 0)

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 1, HalfLifeDays: 30, MinAgeDays: 14, ReinforcementK: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if !isArchived(t, e, "bajada") {
		t.Error("la cuota no desalojó a la bajada que llegó hace 200 días: volver a bajarla le renovó la gracia")
	}
	if isArchived(t, e, "propia") {
		t.Error("la cuota desalojó a la propia de 100 días en vez de a la bajada, que es más fría")
	}
	if n != 1 {
		t.Errorf("la cuota desalojó %d; esperaba 1", n)
	}
}
