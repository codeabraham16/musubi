package memory

import "testing"

func TestSaveAndGetUnresolvedTelemetry(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveTelemetryLog("main.go", "undefined: foo", "agregar import"); err != nil {
		t.Fatalf("SaveTelemetryLog error: %v", err)
	}

	logs, err := e.GetUnresolvedTelemetryLogs()
	if err != nil {
		t.Fatalf("GetUnresolvedTelemetryLogs error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("esperaba 1 log, obtuve %d", len(logs))
	}
	if logs[0].FilePath != "main.go" || logs[0].Resolved {
		t.Errorf("log inesperado: %+v", logs[0])
	}
}

func TestResolveTelemetryLog(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveTelemetryLog("a.go", "error A", ""); err != nil {
		t.Fatalf("save error: %v", err)
	}
	logs, err := e.GetUnresolvedTelemetryLogs()
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("setup: esperaba 1 log, obtuve %d", len(logs))
	}

	if err := e.ResolveTelemetryLog(logs[0].ID); err != nil {
		t.Fatalf("ResolveTelemetryLog error: %v", err)
	}

	remaining, err := e.GetUnresolvedTelemetryLogs()
	if err != nil {
		t.Fatalf("get tras resolver error: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("esperaba 0 logs sin resolver, obtuve %d", len(remaining))
	}
}

func TestResolveTelemetryLogMissingID(t *testing.T) {
	e := newTestEngine(t)
	if err := e.ResolveTelemetryLog(9999); err == nil {
		t.Fatal("esperaba error al resolver un id inexistente")
	}
}
