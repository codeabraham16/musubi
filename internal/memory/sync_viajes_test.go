package memory

import (
	"context"
	"errors"
	"testing"
)

// EL CONTADOR DE ENVIADAS CUENTA LO QUE SALIÓ DE ACÁ (migración v58).
//
// Medido el 2026-09-26 en davantis-1: de 312 filas 'sent', 308 estaban retiradas en el central y 0
// visibles. El sello 'espejo' del pull pisaba a toda 'sent' que rebotaba, así que el contador se
// quedaba sólo con lo que el central ya no devolvía. Estas pruebas fijan las tres piezas: el
// rebote no le cambia el estado a lo enviado, la entrega deja escrito qué salió y cuándo, y los
// viajes se suman por día sin pisarse.

// sentDe lee lo que la fila de outbox dice de la última entrega.
func sentDe(t *testing.T, e *DbEngine, id string) (status string, sentHash, sentAt, updatedAt *string) {
	t.Helper()
	if err := e.db.QueryRow(`SELECT status, sent_hash, sent_at, updated_at FROM outbox WHERE obs_id = ?`, id).
		Scan(&status, &sentHash, &sentAt, &updatedAt); err != nil {
		t.Fatalf("leer la fila de outbox de %s: %v", id, err)
	}
	return
}

func textoONulo(s *string) string {
	if s == nil {
		return "<NULL>"
	}
	return *s
}

// TestElRebotePreservaElEnviado: una nota propia ya entregada vuelve en la bajada y sigue 'sent',
// con su hora de entrega y su hash intactos; una nota ajena bajada queda 'espejo' como siempre.
//
// Sabotaje: volver al sello incondicional, que convierte la 'sent' en 'espejo'.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="status = CASE WHEN outbox.status = 'sent'"
// arnes: a="status = CASE WHEN 0"
func TestElRebotePreservaElEnviado(t *testing.T) {
	e := newTestEngine(t)

	const propio = "esto lo escribi aca y ya salio al central"
	if err := e.SaveObservationTyped("mia-1", "t/x", propio, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ClaimOutboxBatch(50, 60); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxSent("mia-1"); err != nil {
		t.Fatal(err)
	}
	// La entrega se lleva a un pasado fijo: así un «datetime('now')» del rebote se vería distinto.
	if _, err := e.db.Exec(`UPDATE outbox SET sent_at = '2026-01-02 03:04:05', updated_at = '2026-01-02 03:04:05' WHERE obs_id = 'mia-1'`); err != nil {
		t.Fatal(err)
	}
	_, hashAntes, _, _ := sentDe(t, e, "mia-1")

	// El central la devuelve: es el rebote de lo que esta máquina subió.
	if _, err := e.IngestShared(SharedObs{
		ID: "mia-1", TopicKey: "t/x", Content: propio,
		Importance: 1, MemType: "semantic", Author: "davantis-mando-admin", ProjectID: "acme",
	}); err != nil {
		t.Fatal(err)
	}
	status, hash, sentAt, updatedAt := sentDe(t, e, "mia-1")
	if status != outboxSent {
		t.Errorf("CONTADOR ROTO: la nota propia enviada pasó de 'sent' a %q al rebotar en la bajada", status)
	}
	// Las horas se comparan en SQL: el driver devuelve una columna DATETIME reformateada, y lo que
	// importa es el valor guardado, no cómo lo imprime Go.
	var intactas int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE obs_id = 'mia-1'
		AND sent_at = '2026-01-02 03:04:05' AND updated_at = '2026-01-02 03:04:05'`).Scan(&intactas); err != nil {
		t.Fatal(err)
	}
	if intactas != 1 {
		t.Errorf("el rebote tocó la hora de entrega o el updated_at de una 'sent': sent_at=%v updated_at=%v", textoONulo(sentAt), textoONulo(updatedAt))
	}
	if hash == nil || hashAntes == nil || *hash != *hashAntes {
		t.Errorf("el rebote cambió el sent_hash: antes %v, después %v", textoONulo(hashAntes), textoONulo(hash))
	}

	// La ajena sigue sellándose espejo: el arreglo no puede convertir lo bajado en «enviado».
	if _, err := e.IngestShared(SharedObs{
		ID: "ajena-1", TopicKey: "t/x", Content: "esto lo escribio otra maquina",
		Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme",
	}); err != nil {
		t.Fatal(err)
	}
	if st, _, sa, _ := sentDe(t, e, "ajena-1"); st != outboxEspejo || sa != nil {
		t.Errorf("una nota ajena bajada quedó status=%q sent_at=%v; esperaba 'espejo' sin hora de entrega", st, textoONulo(sa))
	}
}

// TestLaEntregaDejaQueSalioYCuando: MarkOutboxSent graba el hash que se entregó y la hora. Sin
// sent_at no hay «enviadas en 24 h», y sin sent_hash el PR siguiente no puede separar un rebote
// propio de un choque con otra máquina.
//
// Sabotaje: que la marca deje las dos columnas como estaban (NULL).
// arnes: archivo="internal/memory/outbox.go"
// arnes: de="sent_hash = enqueued_hash, sent_at = datetime('now')"
// arnes: a="sent_hash = sent_hash, sent_at = sent_at"
func TestLaEntregaDejaQueSalioYCuando(t *testing.T) {
	e := newTestEngine(t)
	const contenido = "una nota compartida que sale en este tick"
	if err := e.SaveObservationTyped("sale-1", "t/x", contenido, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ClaimOutboxBatch(50, 60); err != nil {
		t.Fatal(err)
	}
	if err := e.MarkOutboxSent("sale-1"); err != nil {
		t.Fatal(err)
	}

	var hashObs string
	if err := e.db.QueryRow(`SELECT content_hash FROM observations WHERE id = 'sale-1'`).Scan(&hashObs); err != nil {
		t.Fatal(err)
	}
	status, sentHash, sentAt, _ := sentDe(t, e, "sale-1")
	if status != outboxSent {
		t.Fatalf("precondición: esperaba 'sent', hay %q", status)
	}
	if sentHash == nil || *sentHash != hashObs {
		t.Errorf("sent_hash=%v, esperaba el hash del contenido entregado (%s)", textoONulo(sentHash), hashObs)
	}
	var reciente int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE obs_id = 'sale-1' AND sent_at >= datetime('now','-1 minute')`).Scan(&reciente); err != nil {
		t.Fatal(err)
	}
	if reciente != 1 {
		t.Errorf("sent_at=%v no es la hora de la entrega", textoONulo(sentAt))
	}

	r, err := e.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.EnviadasDia != 1 || r.EnviadasSemana != 1 {
		t.Errorf("enviadas 24 h / 7 d = %d / %d, esperaba 1 / 1", r.EnviadasDia, r.EnviadasSemana)
	}
}

// TestLosViajesSeSumanPorDiaYSentido: dos ticks del mismo día se SUMAN en una fila, y la subida y
// la bajada van por separado. Dos procesos registrando a la vez tienen que acumular, no pisarse.
//
// Sabotaje: que el UPSERT pise las filas en vez de sumarlas.
// arnes: archivo="internal/memory/sync_viajes.go"
// arnes: de="filas        = sync_viajes.filas        + excluded.filas,"
// arnes: a="filas        = excluded.filas,"
func TestLosViajesSeSumanPorDiaYSentido(t *testing.T) {
	e := newTestEngine(t)
	if err := e.RegistrarViaje(ViajeSubida, Viaje{Filas: 2, Posts: 2, BytesCable: 600, BytesCrudos: 600}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegistrarViaje(ViajeSubida, Viaje{Filas: 3, Posts: 4, BytesCable: 900, BytesCrudos: 900}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegistrarViaje(ViajeBajada, Viaje{Filas: 50, Posts: 1, BytesCable: 150000, BytesCrudos: 150000}); err != nil {
		t.Fatal(err)
	}
	if err := e.RegistrarViaje("de-costado", Viaje{Filas: 1}); err == nil {
		t.Error("aceptó un sentido que no es subida ni bajada")
	}

	var filasTabla int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM sync_viajes`).Scan(&filasTabla); err != nil {
		t.Fatal(err)
	}
	if filasTabla != 2 {
		t.Errorf("sync_viajes tiene %d filas; esperaba 2 (una por sentido en el día)", filasTabla)
	}

	r, err := e.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := r.SubidaHoy; got.Filas != 5 || got.Posts != 6 || got.BytesCable != 1500 || got.BytesCrudos != 1500 {
		t.Errorf("subida de hoy = %+v; esperaba filas=5 posts=6 bytes=1500", got)
	}
	if r.Subida7d != r.SubidaHoy {
		t.Errorf("con un solo día, 7 d (%+v) tenía que ser igual a hoy (%+v)", r.Subida7d, r.SubidaHoy)
	}
	if got := r.BajadaHoy; got.Filas != 50 || got.Posts != 1 || got.BytesCable != 150000 {
		t.Errorf("bajada de hoy = %+v; esperaba filas=50 posts=1 bytes=150000", got)
	}
}

// TestLoQueNoViajaSeCuentaUnaSolaVez: las tres categorías de «no viaja» son disjuntas. Una
// propuesta corroborada de un LLM que sigue local cuenta como tal y NO como local a secas, una en
// cuarentena no cuenta como local, y lo archivado o compartido no cuenta.
//
// Sabotaje: que las locales dejen de excluir lo escrito por un LLM.
// arnes: archivo="internal/memory/sync_viajes.go"
// arnes: de="AND NOT (`+esLLM+`) THEN"
// arnes: a="AND (1 OR `+esLLM+`) THEN"
func TestLoQueNoViajaSeCuentaUnaSolaVez(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"local-1", "local-2", "archivada"} {
		if err := e.SaveObservationTyped(id, "t/x", "nota local "+id, 1, "semantic", ScopeLocal, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = 'archivada'`); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("compartida", "t/x", "nota compartida", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ProposeObservation("", "", "t/x", "propuesta que nadie corroboro", "modelo-x", 0.5, "semantic", nil); err != nil {
		t.Fatal(err)
	}
	corroborada, err := e.ProposeObservation("", "", "t/x", "propuesta ya corroborada", "modelo-x", 0.5, "semantic", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.CorroborateObservation(corroborada); err != nil {
		t.Fatal(err)
	}

	r, err := e.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := NoViajan{Locales: 2, EnCuarentena: 1, LLMCorroboradasLocales: 1}
	if r.NoViajan != want {
		t.Errorf("no viajan = %+v; esperaba %+v", r.NoViajan, want)
	}
}

// TestUnBinarioAnteriorLeeLaBaseDelContador: la v58 es readCompatible, así que un binario que
// conoce hasta la v57 abre la base migrada como LEGIBLE (no la puede escribir). Es el contrato que
// deja convivir, durante el despliegue, a los procesos viejos que todavía no se cerraron.
//
// Sabotaje: declarar la migración no-compatible, que sube el piso de lectura por encima del
// binario anterior y lo deja sin poder leer.
// arnes: archivo="internal/memory/migrations.go"
// arnes: de="name:           \"contador_honesto_del_sync\",\n\t\t\treadCompatible: true,"
// arnes: a="name:           \"contador_honesto_del_sync\",\n\t\t\treadCompatible: false,"
func TestUnBinarioAnteriorLeeLaBaseDelContador(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	if err := eng.RegistrarViaje(ViajeSubida, Viaje{Filas: 1, Posts: 1, BytesCable: 10, BytesCrudos: 10}); err != nil {
		t.Fatal(err)
	}
	eng.Close()

	var nueva migration
	var anteriores []migration
	for _, m := range schemaMigrations() {
		if m.name == "contador_honesto_del_sync" {
			nueva = m
			continue
		}
		anteriores = append(anteriores, m)
	}
	if nueva.version == 0 {
		t.Fatal("no encontré la migración contador_honesto_del_sync: la prueba no mira lo que cree")
	}
	for _, m := range anteriores {
		if m.version > nueva.version {
			t.Fatalf("hay una migración posterior (v%d %s): esta prueba asume que la del contador es la última y hay que revisarla", m.version, m.name)
		}
	}

	err = applyMigrations(abrirCruda(t, dir), anteriores)
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("el binario anterior no vio la base como más nueva: %v", err)
	}
	if !errors.Is(err, ErrEsquemaLegible) {
		t.Errorf("un binario v%d no puede LEER la base migrada por la v%d: los procesos viejos de la PC se quedarían sin memoria hasta cerrarlos\n  %v",
			nueva.version-1, nueva.version, err)
	}
}
