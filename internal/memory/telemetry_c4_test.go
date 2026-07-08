package memory

import "testing"

func TestResolveTelemetryLogAndGet(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveTelemetryLog("a.go", "undefined: X", "import Y"); err != nil {
		t.Fatal(err)
	}
	logs, err := e.GetUnresolvedTelemetryLogs()
	if err != nil || len(logs) != 1 {
		t.Fatalf("setup: err=%v len=%d", err, len(logs))
	}
	id := logs[0].ID

	got, found, err := e.ResolveTelemetryLogAndGet(id)
	if err != nil || !found {
		t.Fatalf("resolve: err=%v found=%v", err, found)
	}
	if got.FilePath != "a.go" || got.ErrorMessage != "undefined: X" || got.SuggestedPatch != "import Y" || !got.Resolved {
		t.Fatalf("fila inesperada: %+v", got)
	}
	if logs2, _ := e.GetUnresolvedTelemetryLogs(); len(logs2) != 0 {
		t.Fatalf("debía quedar resuelto; quedan %d pendientes", len(logs2))
	}

	// id inexistente ⇒ found=false, sin error.
	if _, found2, err := e.ResolveTelemetryLogAndGet(999999); err != nil || found2 {
		t.Fatalf("id inexistente: err=%v found=%v", err, found2)
	}
}
