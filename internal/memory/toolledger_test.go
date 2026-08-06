package memory

import (
	"context"
	"strings"
	"testing"
	"time"
)

// L4 — el ledger SOBREVIVE AL REINICIO. Es la diferencia entera contra los contadores en memoria
// que ya existían: se escribe con un engine, se cierra, se reabre desde el MISMO directorio y los
// datos siguen ahí. Sin esto no hay fase.
func TestL4ElLedgerSobreviveAlReinicio(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	e1, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}
	if err := e1.RecordToolInvocations(ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: 12 * time.Millisecond},
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: 20 * time.Millisecond},
		{Tool: "musubi_save_observation", Outcome: OutcomeError, Duration: 5 * time.Millisecond},
	}); err != nil {
		t.Fatalf("registrar: %v", err)
	}
	e1.Close()

	// Reinicio: motor nuevo, mismo directorio.
	e2, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer e2.Close()

	filas, err := e2.ToolUsage(ctx, 30)
	if err != nil {
		t.Fatalf("consultar tras reinicio: %v", err)
	}
	if len(filas) != 2 {
		t.Fatalf("FUGA L4: tras reiniciar esperaba 2 tools con historia, obtuve %d: %+v", len(filas), filas)
	}
	if filas[0].Tool != "musubi_recall" || filas[0].Calls != 2 {
		t.Errorf("la más usada debía ser musubi_recall con 2 llamadas, obtuve %+v", filas[0])
	}
	if filas[1].Errors != 1 {
		t.Errorf("save_observation debía tener 1 error, obtuve %+v", filas[1])
	}
}

// El outcome es una taxonomía CERRADA: un valor inventado se normaliza en vez de escribirse.
// Sin esto, un mensaje de error podría entrar como "outcome" y arrastrar contenido adentro.
func TestElOutcomeEsTaxonomiaCerrada(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.RecordToolInvocations(ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: "error: no se pudo leer /home/user/secreto.txt", Duration: time.Millisecond},
	}); err != nil {
		t.Fatalf("registrar: %v", err)
	}
	var outcome string
	if err := e.db.QueryRow(`SELECT outcome FROM tool_invocations LIMIT 1`).Scan(&outcome); err != nil {
		t.Fatal(err)
	}
	if outcome != OutcomeError {
		t.Errorf("un outcome fuera de la taxonomía debe normalizarse a %q, quedó %q", OutcomeError, outcome)
	}
	if strings.Contains(outcome, "secreto") {
		t.Errorf("FUGA: texto libre entró a la columna outcome: %q", outcome)
	}
}

// L6 — la purga borra lo viejo y respeta lo reciente.
func TestL6LaPurgaRespetaLaRetencion(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.RecordToolInvocations(ctx, []ToolInvocation{
		{Tool: "musubi_recall", Outcome: OutcomeOK, Duration: time.Millisecond},
	}); err != nil {
		t.Fatal(err)
	}
	// Una fila vieja, insertada con fecha explícita.
	if _, err := e.db.Exec(
		`INSERT INTO tool_invocations (tool, outcome, duration_us, created_at)
		 VALUES ('musubi_map', 'ok', 1000, datetime('now','-200 days'))`); err != nil {
		t.Fatal(err)
	}

	n, err := e.PurgeToolInvocations(ctx, 90)
	if err != nil {
		t.Fatalf("purgar: %v", err)
	}
	if n != 1 {
		t.Errorf("esperaba purgar exactamente la fila vieja, purgó %d", n)
	}
	var quedan int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM tool_invocations`).Scan(&quedan); err != nil {
		t.Fatal(err)
	}
	if quedan != 1 {
		t.Errorf("la fila reciente debía sobrevivir; quedan %d", quedan)
	}

	// Retención <= 0 significa "no purgar" y no debe borrar nada.
	if n, err := e.PurgeToolInvocations(ctx, 0); err != nil || n != 0 {
		t.Errorf("retención 0 no debe purgar nada, obtuve n=%d err=%v", n, err)
	}
}

// La p95 sale de los datos crudos, así que un outlier se ve y no queda promediado.
func TestLaP95SeCalculaSobreDatosCrudos(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	// 90 rápidas + 10 lentas. La proporción importa: con UNA sola lenta sobre 20 muestras el
	// outlier cae justo en el 5% y la p95 por rango más cercano lo deja AFUERA — sería p100, no
	// p95. Esa fue la primera versión de este test y fallaba por su propia aritmética, no por el
	// código.
	lote := []ToolInvocation{}
	for i := 0; i < 90; i++ {
		lote = append(lote, ToolInvocation{Tool: "musubi_ask", Outcome: OutcomeOK, Duration: 10 * time.Millisecond})
	}
	for i := 0; i < 10; i++ {
		lote = append(lote, ToolInvocation{Tool: "musubi_ask", Outcome: OutcomeOK, Duration: 3 * time.Second})
	}
	if err := e.RecordToolInvocations(ctx, lote); err != nil {
		t.Fatal(err)
	}

	filas, err := e.ToolUsage(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(filas) != 1 {
		t.Fatalf("esperaba 1 tool, obtuve %d", len(filas))
	}
	f := filas[0]
	if f.Calls != 100 {
		t.Fatalf("esperaba 100 llamadas, obtuve %d", f.Calls)
	}
	// La media queda arrastrada por el outlier (~159 ms) pero la p95 lo EXPONE (3000 ms).
	if f.P95Millis < 2900 {
		t.Errorf("la p95 debía mostrar el outlier de 3s, obtuve %.1f ms", f.P95Millis)
	}
	if f.AvgMillis >= f.P95Millis {
		t.Errorf("con un outlier la media (%.1f) debe quedar MUY por debajo de la p95 (%.1f)", f.AvgMillis, f.P95Millis)
	}
	if f.MaxMillis < 2999 {
		t.Errorf("el máximo debía ser ~3000 ms, obtuve %.1f", f.MaxMillis)
	}
}

// Un lote vacío no abre transacción ni falla.
func TestLoteVacioEsNoOp(t *testing.T) {
	e := newTestEngine(t)
	if err := e.RecordToolInvocations(context.Background(), nil); err != nil {
		t.Errorf("un lote vacío debe ser no-op, obtuve %v", err)
	}
}

// El formateador dice algo útil cuando no hay nada, en vez de una tabla vacía.
func TestFormatoSinDatos(t *testing.T) {
	out := FormatToolUsage(nil, 7)
	if !strings.Contains(out, "Sin invocaciones") || !strings.Contains(out, "7") {
		t.Errorf("el vacío debe explicarse y decir la ventana, obtuve %q", out)
	}
}
