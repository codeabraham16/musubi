package memory

import "testing"

// UNA EDICIÓN LOCAL NO SE PIERDE POR EL PUSH NI POR EL PULL (ola 2, frente sync).
//
// Dos pérdidas, medidas en main antes de este arreglo, y ninguna con un error a la vista:
//   - Por el PUSH: claim de v1, edición local v2 mientras v1 viaja, 200 del central por v1.
//     MarkOutboxSent miraba sólo el estado y dejaba la fila 'sent' con v2 adentro: 0 filas por
//     reenviar, y v2 no salía nunca.
//   - Por el PULL: v2 'pending' y baja otra versión de la misma id. El UPSERT de IngestShared le
//     pisaba el contenido, y como el payload del envío se arma desde observations, el push siguiente
//     subía la versión del central. El sello de #656 salvaba el estado de la fila, no la edición.
//
// Las pruebas de acá fijan el arreglo en el engine; el caso entero —dos procesos sobre una misma
// base contra un central que devuelve lo que se le sube— está en internal/mcp
// (TestUnaEdicionEnVueloLlegaAlCentral).

const (
	versionQueViaja = "la nota tal como salio al central"
	versionEditada  = "la nota editada aca mientras la otra viajaba"
)

// reclamada guarda contenido como 'shared' y lo reclama, como hace el drain al empezar un tick.
func reclamada(t *testing.T, e *DbEngine, id, contenido string) OutboxItem {
	t.Helper()
	if err := e.SaveObservationTyped(id, "t/x", contenido, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	return reclamarUna(t, e, id)
}

// reclamarUna reclama y devuelve el ítem de id; falla si el claim no lo trae.
func reclamarUna(t *testing.T, e *DbEngine, id string) OutboxItem {
	t.Helper()
	items, err := e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ObsID == id {
			return it
		}
	}
	t.Fatalf("el claim no trajo %s (trajo %+v)", id, items)
	return OutboxItem{}
}

// editar guarda una versión nueva de id por el camino de siempre (re-encola si cambió el hash).
func editar(t *testing.T, e *DbEngine, id, contenido string) {
	t.Helper()
	if err := e.SaveObservationTyped(id, "t/x", contenido, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
}

// contenidoDe lee el contenido guardado de id.
func contenidoDe(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var c string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id = ?`, id).Scan(&c); err != nil {
		t.Fatal(err)
	}
	return c
}

// TestEdicionEnVueloNoQuedaEnviada: claim de v1, edición v2 mientras viaja, el central acepta v1.
// La fila queda 'pending' y el próximo claim trae v2; recién la entrega de v2 la deja 'sent'.
//
// Acá la fila ya no está 'claimed' cuando vuelve el 200 —la edición la devolvió a la cola—, así que
// la protegen dos cosas a la vez: el estado y el hash. La guarda del hash solo está en
// TestUnaEntregaTardiaNoCierraElReclamoDeOtro, y la del estado solo en TestUnaReclamadaNoLaPisaElPull.
//
// Sabotaje: que el claim no traiga el hash de lo que empuja (ninguna marca vuelve a aplicar).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="COALESCE(content_hash, \x27\x27)"
// arnes: a="\x27\x27"
func TestEdicionEnVueloNoQuedaEnviada(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "vuela-1", versionQueViaja)
	if v1.Content != versionQueViaja || v1.Hash != ContentHash(versionQueViaja) {
		t.Fatalf("el claim tenía que traer v1 con el hash de ESE contenido, trajo %q / %q", v1.Content, v1.Hash)
	}

	// La edición llega mientras v1 viaja, y después el central contesta 200 por v1.
	editar(t, e, "vuela-1", versionEditada)
	if err := e.MarkOutboxSent("vuela-1", v1.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "vuela-1"); st != outboxPending {
		t.Fatalf("EDICIÓN PERDIDA: la entrega de v1 dejó la fila %q con v2 adentro; esperaba %q", st, outboxPending)
	}

	v2 := reclamarUna(t, e, "vuela-1")
	if v2.Content != versionEditada || v2.Hash != ContentHash(versionEditada) {
		t.Fatalf("el tick siguiente tenía que llevar v2 con su hash, lleva %q / %q", v2.Content, v2.Hash)
	}
	if err := e.MarkOutboxSent("vuela-1", v2.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "vuela-1"); st != outboxSent {
		t.Errorf("la entrega de v2 tenía que dejarla 'sent', quedó %q", st)
	}
	if _, sentHash, _, _ := sentDe(t, e, "vuela-1"); sentHash == nil || *sentHash != v2.Hash {
		t.Errorf("sent_hash=%v, esperaba el de v2 (%s)", textoONulo(sentHash), v2.Hash)
	}
}

// TestUnReintentoViejoNoFrenaLaEdicion: v1 falla por algo transitorio DESPUÉS de que llegó v2. El
// backoff y el intento fallido son de v1: v2 no los hereda y sale ya, en el próximo claim.
//
// Sabotaje: reprogramar sin mirar qué versión falló.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="AND ? = `+hashActual, backoffSeconds, errMsg, obsID, hash)"
// arnes: a="AND (1 OR ? = `+hashActual+`)`, backoffSeconds, errMsg, obsID, hash)"
func TestUnReintentoViejoNoFrenaLaEdicion(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "vuela-2", versionQueViaja)
	editar(t, e, "vuela-2", versionEditada)

	if err := e.MarkOutboxRetry("vuela-2", v1.Hash, 3600, "timeout de v1"); err != nil {
		t.Fatal(err)
	}
	if st, attempts, _ := outboxRow(t, e, "vuela-2"); st != outboxPending || attempts != 0 {
		t.Errorf("v2 heredó el fallo de v1: status=%q attempts=%d; esperaba pending/0", st, attempts)
	}
	if v2 := reclamarUna(t, e, "vuela-2"); v2.Content != versionEditada {
		t.Errorf("el claim trajo %q; esperaba v2", v2.Content)
	}
}

// TestUnRechazoViejoNoMataLaEdicion: el central rechaza v1 para siempre DESPUÉS de que llegó v2. Lo
// rechazado es v1; v2 no va a dead-letter y sale en el próximo claim.
//
// Sabotaje: mandar a dead-letter sin mirar qué versión rechazó el central.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="AND ? = `+hashActual, errMsg, obsID, hash)"
// arnes: a="AND (1 OR ? = `+hashActual+`)`, errMsg, obsID, hash)"
func TestUnRechazoViejoNoMataLaEdicion(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "vuela-3", versionQueViaja)
	editar(t, e, "vuela-3", versionEditada)

	if err := e.MarkOutboxDead("vuela-3", v1.Hash, "el central rechazó v1"); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "vuela-3"); st != outboxPending {
		t.Fatalf("v2 murió por el rechazo de v1: status=%q; esperaba %q", st, outboxPending)
	}
	if v2 := reclamarUna(t, e, "vuela-3"); v2.Content != versionEditada {
		t.Errorf("el claim trajo %q; esperaba v2", v2.Content)
	}
}

// TestElReboteDeLoQueViajabaNoEsChoque: v1 salió mientras se editaba v2, y el central la devuelve.
// Es el rebote más común —la versión que viajaba vuelve en la bajada— y NO es un choque con otra
// máquina. Para distinguirlo, la entrega de v1 tiene que quedar anotada en sent_hash aunque la fila
// siga 'pending' por v2.
//
// Sabotaje: anotar la entrega sólo si la fila queda 'sent' (la variante que no anota lo que viajaba).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="WHERE obs_id = ? AND status IN ('pending','claimed')`, hash, hash, hash, hash, hash, hash, obsID)"
// arnes: a="WHERE obs_id = ? AND status IN ('pending','claimed') AND ? = `+hashActual, hash, hash, hash, hash, hash, hash, obsID, hash)"
func TestElReboteDeLoQueViajabaNoEsChoque(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "vuela-4", versionQueViaja)
	editar(t, e, "vuela-4", versionEditada)
	if err := e.MarkOutboxSent("vuela-4", v1.Hash); err != nil {
		t.Fatal(err)
	}

	ing, err := e.IngestShared(SharedObs{
		ID: "vuela-4", TopicKey: "t/x", Content: versionQueViaja,
		Importance: 1, MemType: "semantic", Author: "davantis-mando-admin", ProjectID: "acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ing != (Ingesta{Rebote: true}) {
		t.Errorf("el rebote de la versión que viajaba se contó %+v; esperaba sólo Rebote", ing)
	}
	if c := contenidoDe(t, e, "vuela-4"); c != versionEditada {
		t.Errorf("EDICIÓN PERDIDA: el rebote de v1 pisó v2, quedó %q", c)
	}
}

// TestPullNoPisaUnaEdicionPendiente: v1 enviada, edición local v2 'pending' —con otro tema, otra
// importancia, otro tipo y su vector—, y la bajada devuelve v1. No se pisa NADA de la edición: el
// contenido, los metadatos y el vector siguen siendo los de v2, el claim trae v2 entera, y se cuenta
// como rebote.
//
// Sabotaje: que la conservación deje de mirar las filas 'pending' (el pull pisa como antes la
// edición que espera salir). Las otras dos mitades de la condición tienen su propia guarda:
// TestUnaReclamadaNoLaPisaElPull y TestUnaHuerfanaRecibeLoQueBaja.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="(estado == outboxPending ||"
// arnes: a="(false ||"
//
// Sabotaje: la variante que conserva el contenido pero toma tema, importancia y tipo de lo que bajó.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="\t\tif entregado.Valid"
// arnes: a="\t\tif _, err := tx.Exec(`UPDATE observations SET topic_key = ?, importance = ?, mem_type = CASE WHEN ? != \x27\x27 THEN ? ELSE mem_type END WHERE id = ?`, o.TopicKey, o.Importance, memType, memType, o.ID); err != nil {\n\t\t\treturn Ingesta{}, err\n\t\t}\n\t\tif err := tx.Commit(); err != nil {\n\t\t\treturn Ingesta{}, err\n\t\t}\n\t\tif entregado.Valid"
//
// Sabotaje: no reconocer el rebote (todo lo conservado se cuenta como choque).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="entregado.Valid && entregado.String == hash"
// arnes: a="entregado.Valid && entregado.String == \"\""
func TestPullNoPisaUnaEdicionPendiente(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("mia-2", "t/a", versionQueViaja, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	v1 := reclamarUna(t, e, "mia-2")
	if err := e.MarkOutboxSent("mia-2", v1.Hash); err != nil {
		t.Fatal(err)
	}
	vector := []float32{0, 1, 0}
	if err := e.SaveObservationTyped("mia-2", "t/b", versionEditada, 3, "procedural", ScopeShared, vector); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "mia-2"); st != outboxPending {
		t.Fatalf("precondición: la edición tenía que dejar la fila pending, quedó %q", st)
	}

	// El central devuelve v1: el rebote de lo que esta máquina subió.
	ing, err := e.IngestShared(SharedObs{
		ID: "mia-2", TopicKey: "t/a", Content: versionQueViaja,
		Importance: 1, MemType: "semantic", Author: "davantis-mando-admin", ProjectID: "acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ing != (Ingesta{Rebote: true}) {
		t.Errorf("IngestShared devolvió %+v; esperaba sólo Rebote (lo que bajó es lo que salió de acá)", ing)
	}

	var contenido, tema, tipo string
	var importancia float64
	if err := e.db.QueryRow(`SELECT content, topic_key, importance, COALESCE(mem_type,'') FROM observations WHERE id = 'mia-2'`).
		Scan(&contenido, &tema, &importancia, &tipo); err != nil {
		t.Fatal(err)
	}
	if contenido != versionEditada {
		t.Fatalf("EDICIÓN PERDIDA: el pull pisó el contenido local, quedó %q", contenido)
	}
	if tema != "t/b" || importancia != 3 || tipo != "procedural" {
		t.Errorf("el pull mezcló la edición con lo que bajó: tema=%q importancia=%v tipo=%q; esperaba t/b, 3, procedural", tema, importancia, tipo)
	}
	if v := vectorGuardado(t, e, "mia-2"); len(v) != 3 || v[1] != 1 {
		t.Errorf("el pull tocó el vector de la edición local: %v", v)
	}
	v2 := reclamarUna(t, e, "mia-2")
	if v2.Content != versionEditada || v2.TopicKey != "t/b" || v2.Importance != 3 {
		t.Errorf("el próximo push lleva %+v; esperaba v2 entera", v2)
	}
}

// TestUnChoqueSeCuentaAparte: lo que baja distinto de la edición local Y de lo que esta máquina
// entregó es un choque —otra máquina editó la misma nota—, y se conserva la local igual. También
// lo es cuando la nota nunca salió de acá (sent_hash vacío).
//
// Sabotaje: contar el choque como rebote.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="return Ingesta{Choque: true}, nil"
// arnes: a="return Ingesta{Rebote: true}, nil"
func TestUnChoqueSeCuentaAparte(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "mia-3", versionQueViaja)
	if err := e.MarkOutboxSent("mia-3", v1.Hash); err != nil {
		t.Fatal(err)
	}
	editar(t, e, "mia-3", versionEditada)

	ajena := SharedObs{ID: "mia-3", TopicKey: "t/x", Content: "la version que escribio gio", Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme"}
	ing, err := e.IngestShared(ajena)
	if err != nil {
		t.Fatal(err)
	}
	if ing != (Ingesta{Choque: true}) {
		t.Errorf("una versión ajena contra una edición local se contó %+v; esperaba sólo Choque", ing)
	}
	if c := contenidoDe(t, e, "mia-3"); c != versionEditada {
		t.Errorf("EDICIÓN PERDIDA: el choque pisó la edición local, quedó %q", c)
	}

	// Nunca salió de acá: no hay entrega con la que confundirla.
	if err := e.SaveObservationTyped("nunca-1", "t/x", "escrita aca, sin salir todavia", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	ajena.ID = "nunca-1"
	if ing, err := e.IngestShared(ajena); err != nil || ing != (Ingesta{Choque: true}) {
		t.Errorf("una versión ajena contra una nota que nunca salió dio %+v (err %v); esperaba sólo Choque", ing, err)
	}
}

// TestLaMismaVersionNoEsChoque: la misma nota capturada en dos máquinas —id determinístico, mismo
// contenido— baja idéntica mientras la de acá todavía no salió. No hay edición que conservar ni nada
// que contar: acá está lo mismo que bajó.
//
// Y la fila sigue 'pending' y sale: es una intención de envío local, y el sello de espejo no la
// puede matar (#656). Es ESTA prueba la que custodia el WHERE del sello: con otro contenido, una
// pendiente ni llega al sello —la corta antes la rama Ingesta{Rebote/Choque}—, así que el WHERE
// sólo decide sobre una pendiente con el mismo contenido que baja.
//
// Sabotaje: conservar toda fila en vuelo, sin comparar el contenido.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="if enVuelo && otroContenido {"
// arnes: a="if enVuelo && (otroContenido || true) {"
//
// Sabotaje: que el sello de espejo vuelva a pisar las filas 'pending'/'claimed'.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="WHERE outbox.status NOT IN ('pending','claimed')`,"
// arnes: a="WHERE 1 = 1`,"
func TestLaMismaVersionNoEsChoque(t *testing.T) {
	e := newTestEngine(t)
	const commit = "feat: la misma captura en las dos maquinas"
	if err := e.SaveObservationTyped("commit-1", "git-commit", commit, 1, "episodic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	ing, err := e.IngestShared(SharedObs{
		ID: "commit-1", TopicKey: "git-commit", Content: commit,
		Importance: 1, MemType: "episodic", Author: "davantis-2", ProjectID: "acme",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ing != (Ingesta{}) {
		t.Errorf("la misma versión bajada contra una pendiente idéntica dio %+v; esperaba ni rebote ni choque", ing)
	}
	if st, _, _ := outboxRow(t, e, "commit-1"); st != outboxPending {
		t.Errorf("ENVÍO PERDIDO: el sello de espejo pisó una pendiente local con el mismo contenido (quedó %q)", st)
	}
	if it := reclamarUna(t, e, "commit-1"); it.Content != commit {
		t.Errorf("el push iba a subir %q en vez de lo que se capturó acá", it.Content)
	}
}

// dejarHuerfana guarda id como una 'shared' que nunca salió, la archiva hace más que la ventana y deja
// que la retención la purgue con su fila de outbox todavía 'pending' (el outbox no tiene FK).
// Devuelve la versión que el central re-entrega de esa id, sin ingerirla.
func dejarHuerfana(t *testing.T, e *DbEngine, id string) SharedObs {
	t.Helper()
	if err := e.SaveObservationTyped(id, "t/x", "la version de aca, que nunca salio", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1, archived_at = datetime('now', '-40 day') WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if n, err := e.PurgeArchived(30); err != nil || n != 1 {
		t.Fatalf("la retención tenía que purgar %s: n=%d err=%v", id, n, err)
	}
	return SharedObs{ID: id, TopicKey: "t/x", Content: "la version que el central re-entrega de " + id,
		Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme"}
}

// huerfanaReentregada deja una fila con DERIVA —el hash encolado ya no es el de la observación— por
// el camino de producción que encontró la revisión: una huérfana (dejarHuerfana) que el central
// re-entrega con otro contenido. IngestShared la inserta y el sello respeta la 'pending': queda
// encolado el hash de la versión purgada y en la observación el de la que bajó. Devuelve lo que
// bajó.
func huerfanaReentregada(t *testing.T, e *DbEngine, id string) SharedObs {
	t.Helper()
	bajada := dejarHuerfana(t, e, id)
	if _, err := e.IngestShared(bajada); err != nil {
		t.Fatal(err)
	}
	if st, _, h := outboxRow(t, e, id); st != outboxPending || h == ContentHash(bajada.Content) {
		t.Fatalf("precondición: la fila de %s tenía que seguir 'pending' con el hash de la versión purgada; quedó %q", id, st)
	}
	return bajada
}

// TestUnaFilaConDerivaSeCierraConLaEntrega: una fila en vuelo con deriva se cierra con la primera
// entrega de lo que la base tiene, como en main, en vez de re-empujarse cada lease para siempre. Las
// tres marcas comparan contra el content_hash de la observación (hashActual), no contra lo encolado.
// La otra fuente de deriva —un binario anterior que baja otra versión encima de una edición
// pendiente— deja la fila igual que la huérfana de acá. Una fila vieja sin content_hash también se
// cierra: hashActual lee el NULL como la cadena vacía, igual que el payload.
//
// Y mientras dura, que baje justo lo que ya hay no es rebote ni choque: la conservación compara el
// contenido de acá, no el hash encolado, que en esta fila no dice qué hay.
//
// Sabotaje: no re-sellar enqueued_hash al quedar 'sent' (re-guardar lo mismo la vuelve a encolar).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="enqueued_hash = CASE WHEN ? = `+hashActual+` THEN ? ELSE enqueued_hash END,"
// arnes: a="enqueued_hash = CASE WHEN ? = `+hashActual+` THEN COALESCE(enqueued_hash, ?) ELSE enqueued_hash END,"
//
// Sabotaje: conservar comparando contra el hash encolado, como antes de este arreglo.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="otroContenido := previo != clean"
// arnes: a="var encoladoAhora string\n\t_ = tx.QueryRow(`SELECT COALESCE(enqueued_hash, \x27\x27) FROM outbox WHERE obs_id = ?`, o.ID).Scan(&encoladoAhora)\n\totroContenido := encoladoAhora != hash"
func TestUnaFilaConDerivaSeCierraConLaEntrega(t *testing.T) {
	e := newTestEngine(t)
	bajada := huerfanaReentregada(t, e, "deriva-1")

	// El central la vuelve a entregar tal cual mientras la fila sigue con deriva.
	ing, err := e.IngestShared(bajada)
	if err != nil {
		t.Fatal(err)
	}
	if ing != (Ingesta{}) {
		t.Errorf("la re-entrega de lo que ya está acá se contó %+v; esperaba ni rebote ni choque", ing)
	}

	it := reclamarUna(t, e, "deriva-1")
	if it.Content != bajada.Content || it.Hash != ContentHash(bajada.Content) {
		t.Fatalf("el claim tenía que llevar lo que la base tiene, con su hash; lleva %q / %q", it.Content, it.Hash)
	}
	if err := e.MarkOutboxSent("deriva-1", it.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "deriva-1"); st != outboxSent {
		t.Fatalf("DERIVA SIN FIN: la entrega de lo que la base tiene dejó la fila %q; esperaba %q (si no, vuelve a salir en cada lease)", st, outboxSent)
	}
	// La marca re-selló el hash: re-guardar el mismo contenido no la vuelve a poner a la cola.
	if err := e.SaveObservationTyped("deriva-1", bajada.TopicKey, bajada.Content, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "deriva-1"); st != outboxSent {
		t.Errorf("re-guardar el mismo contenido la volvió a encolar (%q): la entrega no re-selló enqueued_hash", st)
	}

	// Las otras dos marcas también aplican sobre una fila con deriva.
	huerfanaReentregada(t, e, "deriva-2")
	it = reclamarUna(t, e, "deriva-2")
	if err := e.MarkOutboxRetry("deriva-2", it.Hash, 3600, "timeout del central"); err != nil {
		t.Fatal(err)
	}
	if st, attempts, _ := outboxRow(t, e, "deriva-2"); st != outboxPending || attempts != 1 {
		t.Errorf("el reintento no aplicó sobre la fila con deriva: status=%q attempts=%d; esperaba pending/1 (sin él, sale cada lease y sin backoff)", st, attempts)
	}
	huerfanaReentregada(t, e, "deriva-3")
	it = reclamarUna(t, e, "deriva-3")
	if err := e.MarkOutboxDead("deriva-3", it.Hash, "el central la rechaza para siempre"); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "deriva-3"); st != outboxDead {
		t.Errorf("el rechazo no aplicó sobre la fila con deriva: status=%q; esperaba %q (sin él, sale cada lease y la rechazan cada vez)", st, outboxDead)
	}

	// Y una fila vieja sin content_hash, de las que el doctor cuenta para reparar: el ítem sale con
	// Hash vacío, y la marca la cierra porque hashActual lee el NULL igual que el payload.
	if err := e.SaveObservationTyped("sin-hash", "t/x", "una nota de antes de los digests", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET content_hash = NULL WHERE id = 'sin-hash'`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE outbox SET enqueued_hash = NULL WHERE obs_id = 'sin-hash'`); err != nil {
		t.Fatal(err)
	}
	it = reclamarUna(t, e, "sin-hash")
	if err := e.MarkOutboxSent("sin-hash", it.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "sin-hash"); st != outboxSent {
		t.Errorf("DERIVA SIN FIN: una fila sin content_hash quedó %q tras entregarse; esperaba %q", st, outboxSent)
	}
}

// TestUnExitoViejoNoBorraElErrorNuevo: dos drainers sobre la misma base, como los varios daemons por
// base de davantis-1. A lleva v1; se edita v2; B reclama v2 y falla por algo transitorio; recién ahí
// le llega a A el 200 de v1. La fila sigue 'pending' por v2, y el error de v2 —lo que
// musubi_sync_status muestra de lo que está trabado— sigue ahí: el 200 era de otra versión.
//
// Sabotaje: que la entrega de otra versión borre el último error, como antes.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="last_error = CASE WHEN ? = `+hashActual+` THEN NULL ELSE last_error END,"
// arnes: a="last_error = CASE WHEN ? IS NULL THEN NULL ELSE NULL END,"
func TestUnExitoViejoNoBorraElErrorNuevo(t *testing.T) {
	e := newTestEngine(t)
	a := reclamada(t, e, "dos-drainers", versionQueViaja)
	editar(t, e, "dos-drainers", versionEditada)
	b := reclamarUna(t, e, "dos-drainers")
	if b.Content != versionEditada {
		t.Fatalf("precondición: B tenía que reclamar v2, reclamó %q", b.Content)
	}
	if err := e.MarkOutboxRetry("dos-drainers", b.Hash, 30, "timeout de v2"); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxSent("dos-drainers", a.Hash); err != nil {
		t.Fatal(err)
	}

	if st, attempts, _ := outboxRow(t, e, "dos-drainers"); st != outboxPending || attempts != 1 {
		t.Errorf("la fila tenía que seguir pendiente por v2 con su intento fallido; quedó %q con attempts=%d", st, attempts)
	}
	h, err := e.OutboxHealth()
	if err != nil {
		t.Fatal(err)
	}
	if h.LastError != "timeout de v2" {
		t.Errorf("el 200 tardío de v1 borró el error de v2, que sigue sin salir: last_error=%q", h.LastError)
	}
}

// TestUnaEntregaTardiaNoCierraElReclamoDeOtro: dos drainers. A lleva v1; se edita v2; B reclama v2 y
// todavía no volvió. Recién ahí le llega a A el 200 de v1: la fila está 'claimed' —por B— pero con
// otro contenido, y no se cierra. Si se cerrara y B fallara, el reintento de v2 ya no encontraría una
// fila abierta que reprogramar: v2 no saldría nunca.
//
// Sabotaje: marcar 'sent' sin mirar qué versión salió.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="status = CASE WHEN status = 'claimed' AND ? = `+hashActual+` THEN 'sent' ELSE status END,"
// arnes: a="status = CASE WHEN status = 'claimed' AND (1 OR ? = `+hashActual+`) THEN 'sent' ELSE status END,"
// arnes: colision_ok="TestUnaReclamadaNoLaPisaElPull"
func TestUnaEntregaTardiaNoCierraElReclamoDeOtro(t *testing.T) {
	e := newTestEngine(t)
	a := reclamada(t, e, "tardia-1", versionQueViaja)
	editar(t, e, "tardia-1", versionEditada)
	b := reclamarUna(t, e, "tardia-1")
	if b.Content != versionEditada {
		t.Fatalf("precondición: B tenía que reclamar v2, reclamó %q", b.Content)
	}
	if err := e.MarkOutboxSent("tardia-1", a.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "tardia-1"); st != outboxClaimed {
		t.Fatalf("el 200 tardío de v1 cerró la fila que B tiene reclamada con v2: quedó %q", st)
	}
	if err := e.MarkOutboxRetry("tardia-1", b.Hash, 0, "timeout de v2"); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "tardia-1"); st != outboxPending {
		t.Errorf("el fallo de v2 tenía que dejarla en la cola, quedó %q", st)
	}
}

// TestUnaReclamadaNoLaPisaElPull: una edición local que un drain ya reclamó ('claimed') tampoco la
// pisa el pull. Es un drainer que se llevó v2 y todavía no la empujó, o que murió antes de hacerlo y
// la fila espera a que venza su lease: ni el rebote de v1 ni una versión ajena reemplazan a v2, y lo
// que sale es v2.
//
// Y sale DESPUÉS de la ajena. La versión de gio llegó al central más tarde que el push de v2 que ya
// estaba en vuelo; si el 200 de ese push cerrara la fila, quedaría v2 acá y la de gio allá para
// siempre. El choque devuelve la reclamada a la cola, la marca no cierra una 'pending', y v2 vuelve a
// subir: gana la última.
//
// Sabotaje: que la conservación deje de mirar las filas 'claimed'.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="|| estado == outboxClaimed)"
// arnes: a="|| false)"
//
// Sabotaje: que el choque sobre la reclamada no la devuelva a la cola.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="if err := volverASubirTrasUnChoque(tx, o.ID); err != nil {"
// arnes: a="if err := error(nil); err != nil {"
//
// Sabotaje: que la marca de entrega cierre también una 'pending' con el mismo contenido.
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="status = CASE WHEN status = 'claimed' AND ? = `+hashActual+` THEN 'sent' ELSE status END,"
// arnes: a="status = CASE WHEN ? = `+hashActual+` THEN 'sent' ELSE status END,"
// arnes: colision_ok="TestUnaEntregaTardiaNoCierraElReclamoDeOtro"
func TestUnaReclamadaNoLaPisaElPull(t *testing.T) {
	e := newTestEngine(t)
	v1 := reclamada(t, e, "reclamada-1", versionQueViaja)
	if err := e.MarkOutboxSent("reclamada-1", v1.Hash); err != nil {
		t.Fatal(err)
	}
	editar(t, e, "reclamada-1", versionEditada)
	v2 := reclamarUna(t, e, "reclamada-1") // un drain se lleva v2
	if st, _, _ := outboxRow(t, e, "reclamada-1"); st != outboxClaimed {
		t.Fatalf("precondición: la fila tenía que quedar 'claimed', quedó %q", st)
	}

	rebote := SharedObs{ID: "reclamada-1", TopicKey: "t/x", Content: versionQueViaja,
		Importance: 1, MemType: "semantic", Author: "davantis-mando-admin", ProjectID: "acme"}
	if ing, err := e.IngestShared(rebote); err != nil || ing != (Ingesta{Rebote: true}) {
		t.Errorf("el rebote de v1 sobre la reclamada dio %+v (err %v); esperaba sólo Rebote", ing, err)
	}
	ajena := rebote
	ajena.Content, ajena.Author = "la version que escribio gio", "gio"
	if ing, err := e.IngestShared(ajena); err != nil || ing != (Ingesta{Choque: true}) {
		t.Errorf("una versión ajena sobre la reclamada dio %+v (err %v); esperaba sólo Choque", ing, err)
	}
	if c := contenidoDe(t, e, "reclamada-1"); c != versionEditada {
		t.Fatalf("EDICIÓN PERDIDA: el pull pisó la versión que el drain se llevó, quedó %q", c)
	}
	if err := e.MarkOutboxSent("reclamada-1", v2.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "reclamada-1"); st != outboxPending {
		t.Fatalf("DIVERGENCIA: el 200 de v2 cerró la fila después del choque —acá v2, en el central la de gio—; quedó %q", st)
	}
	otra := reclamarUna(t, e, "reclamada-1")
	if otra.Content != versionEditada {
		t.Fatalf("lo que vuelve a subir tras el choque tenía que ser v2, es %q", otra.Content)
	}
	if err := e.MarkOutboxSent("reclamada-1", otra.Hash); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := outboxRow(t, e, "reclamada-1"); st != outboxSent {
		t.Errorf("la entrega de la segunda subida tenía que dejarla 'sent', quedó %q", st)
	}
}

// TestUnaHuerfanaRecibeLoQueBaja: una fila de outbox 'pending' cuya observación ya no existe —la
// purgó la retención; el outbox no tiene FK— no es una edición local, porque acá no hay nada que
// conservar. Lo que baja se inserta como siempre y no se cuenta rebote ni choque. Sin mirar si la
// observación existe, «acá no hay nada» era otro contenido que el que baja: se contaba un choque y la
// nota del central no entraba nunca.
//
// La fila de outbox sigue 'pending' con el hash de la versión purgada, y sale UNA vez con lo que
// bajó (la cierra la marca contra hashActual; ver TestUnaHuerfanaReentregadaSaleUnaSolaVez en
// internal/mcp), como en main. Sellarla 'espejo' acá ahorraría ese push, y queda para otro PR.
//
// Sabotaje: conservar aunque la observación no exista.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="enVuelo := existia && ("
// arnes: a="enVuelo := ("
func TestUnaHuerfanaRecibeLoQueBaja(t *testing.T) {
	e := newTestEngine(t)
	bajada := dejarHuerfana(t, e, "huerfana-1")
	ing, err := e.IngestShared(bajada)
	if err != nil {
		t.Fatal(err)
	}
	if ing != (Ingesta{Insertada: true}) {
		t.Errorf("la re-entrega de una huérfana dio %+v; esperaba sólo Insertada (acá no había nada que conservar)", ing)
	}
	if c := contenidoDe(t, e, "huerfana-1"); c != bajada.Content {
		t.Errorf("la nota del central no entró: quedó %q", c)
	}
}
