package memory

import (
	"context"
	"testing"
	"time"
)

// La fecha de una nota viaja con ella: el central la manda en el pull (SharedObs.CreatedAt) y el
// cliente la usa al INSERTAR la fila bajada. Antes, IngestShared no la conocía y la columna tomaba
// su default: una nota de hace tres meses nacía «de hoy» en cada máquina que la bajaba.

// fechaGuardada lee created_at TAL COMO ESTÁ en la base. Adentro de una expresión el driver no la
// convierte (ver TestElDriverConvierteAlLeerYNoAlComparar), y un NULL sale como la palabra NULL.
func fechaGuardada(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(`SELECT COALESCE(created_at, 'NULL') FROM observations WHERE id = ?`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// esLaDeLaBajada dice si la fecha guardada es la del default de la columna: el layout de
// CURRENT_TIMESTAMP y un instante entre antes y después del ingest.
func esLaDeLaBajada(guardada string, antes, despues time.Time) bool {
	f, err := time.Parse(sqliteTimeLayout, guardada)
	if err != nil {
		return false
	}
	return !f.Before(antes.UTC().Truncate(time.Second)) && !f.After(despues.UTC().Add(time.Second))
}

// sembrarEnElCentral guarda una nota shared en e y le fija la fecha de creación.
func sembrarEnElCentral(t *testing.T, e *DbEngine, id, fecha string) {
	t.Helper()
	if err := e.SaveObservationTyped(id, "t/fecha", "la nota "+id+" y su fecha de origen", 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SetObservationCreatedAt(id, fecha); err != nil {
		t.Fatal(err)
	}
}

// TestLaFechaDeOrigenViajaEnElPull: lo que el central lista para el pull lleva la fecha de creación
// de la nota, en el layout en que está guardada.
//
// Sabotaje: el SELECT deja de leer la fecha (viaja vacía, como antes).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="COALESCE(project_id,\x27\x27), COALESCE(created_at,\x27\x27)"
// arnes: a="COALESCE(project_id,\x27\x27), \x27\x27"
// arnes: colision_ok="TestUnaFilaSinFechaNoCortaLaPagina"
func TestLaFechaDeOrigenViajaEnElPull(t *testing.T) {
	central := newTestEngine(t)
	const origen = "2026-06-29 10:11:12"
	sembrarEnElCentral(t, central, "vieja", origen)

	items, err := central.ListSharedForPull(context.Background(), 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("el pull trajo %d filas, esperaba 1", len(items))
	}
	if items[0].CreatedAt != origen {
		t.Errorf("la fecha de origen viajó como %q; esperaba %q", items[0].CreatedAt, origen)
	}
}

// TestUnaFilaSinFechaNoCortaLaPagina: una fila del central con created_at NULL viaja sin fecha y la
// página se lee entera, y la fecha de las demás sale como está guardada, sin la T y la Z con que el
// driver entrega la columna pelada. Una página que no se puede leer deja sin bajar todo lo que venía
// detrás de ella, para siempre, porque el cursor no avanza.
//
// Sabotaje: leer la columna pelada, sin envolverla.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="COALESCE(created_at,\x27\x27)"
// arnes: a="created_at"
// arnes: colision_ok="TestLaFechaDeOrigenViajaEnElPull"
func TestUnaFilaSinFechaNoCortaLaPagina(t *testing.T) {
	central := newTestEngine(t)
	sembrarEnElCentral(t, central, "sin-fecha", "2026-07-01 08:00:00")
	sembrarEnElCentral(t, central, "con-fecha", "2026-07-02 09:30:00")
	if _, err := central.db.Exec(`UPDATE observations SET created_at = NULL WHERE id = 'sin-fecha'`); err != nil {
		t.Fatal(err)
	}

	items, err := central.ListSharedForPull(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("una fila sin fecha cortó la página: %v", err)
	}
	fechas := map[string]string{}
	for _, it := range items {
		fechas[it.ID] = it.CreatedAt
	}
	if len(fechas) != 2 {
		t.Fatalf("el pull trajo %v; esperaba las dos filas", fechas)
	}
	if fechas["sin-fecha"] != "" {
		t.Errorf("la fila sin fecha viajó con %q; esperaba vacía", fechas["sin-fecha"])
	}
	if fechas["con-fecha"] != "2026-07-02 09:30:00" {
		t.Errorf("la fecha viajó como %q; esperaba la forma guardada «2026-07-02 09:30:00»", fechas["con-fecha"])
	}
}

// TestIngestSharedUsaLaFechaDeOrigen: la fila bajada nace con la fecha que trae, normalizada al
// layout de la columna en UTC. La normalización no es cosmética: el SQL compara created_at como
// texto, y la madrugada escrita con la T de RFC3339 ordenaba después que la noche del mismo día.
//
// Sabotaje: el INSERT no escribe la fecha (la fila toma la de la bajada, como antes).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="author, created_at, sync_seq)"
// arnes: a="author, last_accessed, sync_seq)"
//
// Sabotaje: guardar la fecha tal como llega, sin normalizarla.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="return sql.NullString{String: t.Format(sqliteTimeLayout), Valid: true}"
// arnes: a="return sql.NullString{String: cruda, Valid: true}"
func TestIngestSharedUsaLaFechaDeOrigen(t *testing.T) {
	e := newTestEngine(t)
	casos := []struct{ id, llega, guardada string }{
		{"canonica", "2026-06-29 10:11:12", "2026-06-29 10:11:12"},
		{"rfc3339", "2026-06-29T10:11:12Z", "2026-06-29 10:11:12"},
		{"con-zona", "2026-06-29T07:11:12-03:00", "2026-06-29 10:11:12"},
		{"con-fraccion", "2026-06-29T10:11:12.987654Z", "2026-06-29 10:11:12"},
	}
	for _, c := range casos {
		if _, err := e.IngestShared(SharedObs{ID: c.id, TopicKey: "t/fecha", Content: "nota " + c.id, Importance: 1, CreatedAt: c.llega}); err != nil {
			t.Fatal(err)
		}
		if got := fechaGuardada(t, e, c.id); got != c.guardada {
			t.Errorf("%s: llegó %q y se guardó %q; esperaba %q", c.id, c.llega, got, c.guardada)
		}
	}

	// El mismo día, la madrugada en RFC3339 y la noche en el layout de la columna.
	for id, llega := range map[string]string{"madrugada": "2026-06-30T01:00:00Z", "noche": "2026-06-30 22:00:00"} {
		if _, err := e.IngestShared(SharedObs{ID: id, TopicKey: "t/orden", Content: "nota " + id, Importance: 1, CreatedAt: llega}); err != nil {
			t.Fatal(err)
		}
	}
	var ultima string
	if err := e.db.QueryRow(`SELECT id FROM observations WHERE topic_key = 't/orden' ORDER BY created_at DESC LIMIT 1`).Scan(&ultima); err != nil {
		t.Fatal(err)
	}
	if ultima != "noche" {
		t.Errorf("ORDER BY created_at DESC puso primero a %q; esperaba «noche» (22:00 va después de 01:00)", ultima)
	}
}

// TestReingestarNoTocaLaFecha: una fila que ya estaba conserva su fecha aunque vuelva con otra.
// Son los dos casos reales: la nota creada ACÁ que vuelve del central con la hora en que el central
// la recibió, y la que ya había bajado con la fecha de la bajada. La segunda queda mal y es a
// propósito: repararla lo decide el dueño, y si la reparación llega esta prueba cambia con ella.
//
// Sabotaje: el DO UPDATE también pisa la fecha con la que llega.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="sync_seq=(SELECT IFNULL(MAX(sync_seq),0)+1 FROM observations)"
// arnes: a="created_at=excluded.created_at, sync_seq=(SELECT IFNULL(MAX(sync_seq),0)+1 FROM observations)"
func TestReingestarNoTocaLaFecha(t *testing.T) {
	e := newTestEngine(t)

	// Creada acá a las 10:00:00; el central la recibió 36 s después y la devuelve con esa hora.
	const contenido = "la nota propia que va y vuelve"
	if err := e.SaveObservationTyped("propia", "t/fecha", contenido, 1, "semantic", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SetObservationCreatedAt("propia", "2026-09-20 10:00:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.IngestShared(SharedObs{ID: "propia", TopicKey: "t/fecha", Content: contenido, Importance: 1, CreatedAt: "2026-09-20 10:00:36"}); err != nil {
		t.Fatal(err)
	}
	if got := fechaGuardada(t, e, "propia"); got != "2026-09-20 10:00:00" {
		t.Errorf("la nota creada acá quedó con %q; esperaba su propia fecha, 2026-09-20 10:00:00", got)
	}

	// Bajada de un central viejo (sin fecha), y re-entregada después con la de origen.
	antes := time.Now()
	if _, err := e.IngestShared(SharedObs{ID: "bajada", TopicKey: "t/fecha", Content: "bajada sin fecha", Importance: 1}); err != nil {
		t.Fatal(err)
	}
	laDeLaBajada := fechaGuardada(t, e, "bajada")
	if !esLaDeLaBajada(laDeLaBajada, antes, time.Now()) {
		t.Fatalf("sin fecha de origen la fila quedó con %q; esperaba la de la bajada", laDeLaBajada)
	}
	if _, err := e.IngestShared(SharedObs{ID: "bajada", TopicKey: "t/fecha", Content: "bajada sin fecha", Importance: 1, CreatedAt: "2026-07-01 08:00:00"}); err != nil {
		t.Fatal(err)
	}
	if got := fechaGuardada(t, e, "bajada"); got != laDeLaBajada {
		t.Errorf("la re-entrega cambió la fecha de %q a %q; el DO UPDATE no la toca", laDeLaBajada, got)
	}
}

// TestUnaFechaQueNoSirveCaeAlDefault: vacía (el central es viejo y no la manda), ilegible, del
// futuro más allá de la tolerancia o anterior al piso ⇒ la fila toma el default de la columna, la
// fecha de la bajada: la conducta de siempre. Nunca NULL, nunca el texto que llegó.
//
// Sabotaje: aceptar una fecha del futuro.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="case t.After(ahora.Add(toleranciaDeReloj)):"
// arnes: a="case false:"
//
// Sabotaje: guardar lo que llegó cuando no se puede leer.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="case !ok:\n\t\treturn sql.NullString{}"
// arnes: a="case !ok:\n\t\treturn sql.NullString{String: cruda, Valid: true}"
//
// Sabotaje: pasar la fecha sin el default (con la columna en la lista, un NULL se guarda NULL).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="COALESCE(?, CURRENT_TIMESTAMP)"
// arnes: a="?"
// arnes: colision_ok="TestUnClienteNuevoConUnCentralViejo"
//
// Sabotaje: aceptar una fecha anterior al piso.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="case t.Before(pisoDeFechaDeOrigen):"
// arnes: a="case false:"
func TestUnaFechaQueNoSirveCaeAlDefault(t *testing.T) {
	e := newTestEngine(t)
	casos := []struct{ id, llega string }{
		{"ausente", ""},
		{"ilegible", "ayer a la tarde"},
		{"imposible", "2026-13-45 25:61:61"},
		{"epoch", "1695000000"},
		{"futura", time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)},
		{"de-1970", "1970-01-01 00:00:00"},
		{"cero-de-go", "0001-01-01T00:00:00Z"},
	}
	for _, c := range casos {
		antes := time.Now()
		if _, err := e.IngestShared(SharedObs{ID: c.id, TopicKey: "t/fecha", Content: "nota " + c.id, Importance: 1, CreatedAt: c.llega}); err != nil {
			t.Fatal(err)
		}
		if got := fechaGuardada(t, e, c.id); !esLaDeLaBajada(got, antes, time.Now()) {
			t.Errorf("%s: llegó %q y se guardó %q; esperaba la fecha de la bajada", c.id, c.llega, got)
		}
	}
}

// TestLaToleranciaDeReloj: una fecha que viene del futuro por menos de toleranciaDeReloj es desfase
// entre dos relojes y se acepta tal cual; más allá, no. El piso es inclusivo.
//
// Sabotaje: tolerancia cero (un segundo de desfase ya descarta la fecha).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="toleranciaDeReloj = 5 * time.Minute"
// arnes: a="toleranciaDeReloj = 0 * time.Minute"
func TestLaToleranciaDeReloj(t *testing.T) {
	ahora := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	casos := []struct {
		cruda    string
		guardada string // vacía = cae al default
	}{
		{ahora.Add(4*time.Minute + 59*time.Second).Format(time.RFC3339), "2026-09-27 12:04:59"},
		{ahora.Add(5 * time.Minute).Format(time.RFC3339), "2026-09-27 12:05:00"},
		{ahora.Add(5*time.Minute + time.Second).Format(time.RFC3339), ""},
		{"2026-01-01 00:00:00", "2026-01-01 00:00:00"},
		{"2025-12-31 23:59:59", ""},
	}
	for _, c := range casos {
		f := fechaDeOrigen(c.cruda, ahora)
		if f.String != c.guardada || f.Valid != (c.guardada != "") {
			t.Errorf("fechaDeOrigen(%q) = %+v; esperaba %q", c.cruda, f, c.guardada)
		}
	}
}
