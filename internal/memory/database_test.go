package memory

import (
	"strings"
	"testing"
)

// TestNewDbEngineEnablesWAL verifica que la base se abre en modo WAL con
// busy_timeout, necesario para el acceso concurrente de sub-agentes a la pizarra
// compartida (work_units).
func TestNewDbEngineEnablesWAL(t *testing.T) {
	e := newTestEngine(t)

	var mode string
	if err := e.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode error: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("esperaba journal_mode=wal, obtuve %q", mode)
	}

	var timeout int
	if err := e.db.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout error: %v", err)
	}
	if timeout <= 0 {
		t.Errorf("esperaba busy_timeout > 0, obtuve %d", timeout)
	}
}
