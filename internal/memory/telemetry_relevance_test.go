package memory

import "testing"

// TestGetUnresolvedTelemetryLogsForFiles verifica el filtrado por relevancia (T6.2): solo
// devuelve errores no resueltos de los archivos dados (por ruta o por nombre base).
func TestGetUnresolvedTelemetryLogsForFiles(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveTelemetryLog("internal/auth/jwt.go", "undefined: Claims", "import jwt"); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveTelemetryLog("internal/db/store.go", "syntax error", "fix paren"); err != nil {
		t.Fatal(err)
	}

	// Match por ruta completa: solo jwt.go.
	got, err := e.GetUnresolvedTelemetryLogsForFiles([]string{"internal/auth/jwt.go", "noexiste.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].FilePath != "internal/auth/jwt.go" {
		t.Fatalf("esperaba 1 log de jwt.go, obtuve %+v", got)
	}

	// Match por nombre base (rutas con distinto prefijo o separadores).
	gotBase, err := e.GetUnresolvedTelemetryLogsForFiles([]string{`C:\proj\internal\db\store.go`})
	if err != nil {
		t.Fatal(err)
	}
	if len(gotBase) != 1 || gotBase[0].FilePath != "internal/db/store.go" {
		t.Errorf("el match por basename debió traer store.go, obtuve %+v", gotBase)
	}

	// Archivo sin telemetría → vacío.
	if none, _ := e.GetUnresolvedTelemetryLogsForFiles([]string{"otro/archivo.go"}); len(none) != 0 {
		t.Errorf("un archivo sin telemetría no debe devolver nada, obtuve %+v", none)
	}

	// Lista vacía → vacío.
	if none, _ := e.GetUnresolvedTelemetryLogsForFiles(nil); len(none) != 0 {
		t.Errorf("sin archivos no debe devolver nada, obtuve %+v", none)
	}

	// Los resueltos se excluyen.
	logs, _ := e.GetUnresolvedTelemetryLogs()
	var jwtID int
	for _, l := range logs {
		if l.FilePath == "internal/auth/jwt.go" {
			jwtID = l.ID
		}
	}
	if err := e.ResolveTelemetryLog(jwtID); err != nil {
		t.Fatal(err)
	}
	if after, _ := e.GetUnresolvedTelemetryLogsForFiles([]string{"internal/auth/jwt.go"}); len(after) != 0 {
		t.Errorf("un log resuelto no debe aparecer, obtuve %+v", after)
	}
}
