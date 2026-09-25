package memory

import "testing"

// EL ECO DEL SYNC — el anti-loop de IngestShared se verificaba en UN SOLO INSTANTE.
//
// TestIngestSharedNoLoop comprueba que al ingerir no queda fila de outbox, y eso es cierto. Lo que
// nunca preguntó es qué pasa en la PRÓXIMA APERTURA DE LA BASE: BackfillOutbox corre ahí (no sólo
// al arrancar el servicio) y siembra una fila 'pending' por cada observación 'shared' que no tenga
// fila. Lo bajado del central es 'shared' y no tiene fila. Así que la red de seguridad del backfill
// deshacía el anti-loop del ingest, una apertura después.
//
// El daño no es sólo tráfico. IngestShared REDACTA el contenido otra vez y recalcula gist, hash y
// tokens; si esa copia difiere de la del central, el eco la sube y PISA el original. Otro nodo baja
// esa versión pisada, la vuelve a redactar, y la degradación avanza sola.
//
// Medido en la base de esta máquina el 2026-09-24: 943 de 3.307 filas del outbox (28,5%) pertenecen
// a observaciones con autor ajeno —gio 617, davantis 260, davantis-2 66— que esta máquina no pudo
// escribir. Bajaron y volvieron a subir.

// TestBackfillNoResucitaElEspejo es la mitad que faltaba: el anti-loop tiene que sobrevivir a la
// apertura de la base, no sólo al ingest.
func TestBackfillNoResucitaElEspejo(t *testing.T) {
	e := newTestEngine(t)

	bajada := SharedObs{
		ID: "del-central-1", TopicKey: "t/x", Content: "esto lo escribió otra máquina",
		Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme",
	}
	if _, err := e.IngestShared(bajada); err != nil {
		t.Fatal(err)
	}

	// Precondición: el ingest la sella como espejo (esto ya lo cubre TestIngestSharedNoLoop).
	var status string
	if err := e.db.QueryRow(`SELECT status FROM outbox WHERE obs_id='del-central-1'`).Scan(&status); err != nil {
		t.Fatalf("precondición: la obs bajada debía quedar sellada: %v", err)
	}
	if status != outboxEspejo {
		t.Fatalf("precondición: esperaba status %q tras el ingest, hay %q", outboxEspejo, status)
	}

	// Y acá está el agujero: la siguiente apertura de la base corre el backfill.
	if _, err := e.BackfillOutbox(); err != nil {
		t.Fatalf("BackfillOutbox: %v", err)
	}

	// La observación bajada NO puede quedar reclamable: el central ya la tiene.
	items, err := e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatalf("ClaimOutboxBatch: %v", err)
	}
	for _, it := range items {
		if it.ObsID == "del-central-1" {
			t.Errorf("ECO: el backfill puso a la cola una observación que BAJÓ del central (autor %q)", bajada.Author)
		}
	}
}

// TestEspejoNoTapaUnaEdicionLocal es la contracara, y sin ella el arreglo sería una mordaza: marcar
// lo bajado como 'espejo' no debe impedir que una edición LOCAL posterior de esa misma observación
// viaje al central. El espejo dice «el central tiene exactamente esto», no «no le cuentes nunca
// más nada de esta fila».
func TestEspejoNoTapaUnaEdicionLocal(t *testing.T) {
	e := newTestEngine(t)

	bajada := SharedObs{
		ID: "del-central-2", TopicKey: "t/x", Content: "version del central",
		Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme",
	}
	if _, err := e.IngestShared(bajada); err != nil {
		t.Fatal(err)
	}
	if _, err := e.BackfillOutbox(); err != nil {
		t.Fatal(err)
	}

	// Una edición local de verdad: cambia el contenido, así que cambia el content_hash.
	nuevo := "version editada aca, que el central todavia no tiene"
	if _, err := e.db.Exec(
		`UPDATE observations SET content = ?, content_hash = ? WHERE id = ?`,
		nuevo, ContentHash(nuevo), "del-central-2"); err != nil {
		t.Fatal(err)
	}
	tx, err := e.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := enqueueOutboxTx(tx, "del-central-2"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	items, err := e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatalf("ClaimOutboxBatch: %v", err)
	}
	for _, it := range items {
		if it.ObsID == "del-central-2" {
			return // la edición local sí viaja: es lo que queremos
		}
	}
	t.Error("MORDAZA: una edición LOCAL de una observación bajada no llegó a la cola de envío")
}

// TestEspejoNoPisaUnaPendienteLocal cubre la guarda del ON CONFLICT: si esta máquina tenía una
// intención de envío SIN SALIR todavía ('pending'), el sello de espejo no la puede matar. Sellarla
// sería descartar un envío local en silencio, que es peor que el eco que vino a arreglar.
func TestEspejoNoPisaUnaPendienteLocal(t *testing.T) {
	e := newTestEngine(t)

	// Nace local, se promueve a shared: queda 'pending' esperando el drain.
	if err := e.SaveObservation("mia-1", "t/x", "esto lo escribi yo y todavia no salio", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.PromoteObservation("mia-1"); err != nil {
		t.Fatal(err)
	}
	var antes string
	if err := e.db.QueryRow(`SELECT status FROM outbox WHERE obs_id='mia-1'`).Scan(&antes); err != nil {
		t.Fatalf("precondición: la promovida debía quedar encolada: %v", err)
	}
	if antes != outboxPending {
		t.Fatalf("precondición: esperaba %q, hay %q", outboxPending, antes)
	}

	// Y ahora el central nos devuelve esa misma id (la subió otra máquina, o es un rebote viejo).
	if _, err := e.IngestShared(SharedObs{
		ID: "mia-1", TopicKey: "t/x", Content: "version del central",
		Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme",
	}); err != nil {
		t.Fatal(err)
	}

	var despues string
	if err := e.db.QueryRow(`SELECT status FROM outbox WHERE obs_id='mia-1'`).Scan(&despues); err != nil {
		t.Fatal(err)
	}
	if despues != outboxPending {
		t.Errorf("ENVÍO PERDIDO: el sello de espejo pisó una fila local sin enviar (%q -> %q)", antes, despues)
	}
}

// TestCuotaDesalojaElEspejo es la otra mitad que el estado nuevo podía romper sin avisar: la cuota
// sólo desaloja lo que está sincronizado, y lo medía con `status != 'sent'`. Con el sello nuevo eso
// volvía INDESALOJABLE todo lo bajado del central —la mayoría del corpus en un nodo de equipo— y la
// cuota se quedaba sin nada que liberar mientras la base crecía.
func TestCuotaDesalojaElEspejo(t *testing.T) {
	e := newTestEngine(t)

	// Techo 1, dos activas frías; la más fría bajó del central (sellada espejo).
	seedQuotaObs(t, e, "bajada", "p", 500, 1.0, 0)
	seedQuotaObs(t, e, "local", "p", 200, 1.0, 0)
	if _, err := e.db.Exec(
		`INSERT INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at)
		 VALUES ('bajada', 'h', 'espejo', 0, datetime('now'), datetime('now'), datetime('now'))`,
	); err != nil {
		t.Fatal(err)
	}

	if _, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 1, HalfLifeDays: 30, MinAgeDays: 14}); err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if !isArchived(t, e, "bajada") {
		t.Error("lo bajado del central SÍ se puede desalojar: el central lo tiene y se vuelve a bajar")
	}
}
