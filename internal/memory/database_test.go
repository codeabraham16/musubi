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

func TestForeignKeysEnabled(t *testing.T) {
	e := newTestEngine(t)
	var fk int
	if err := e.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys error: %v", err)
	}
	if fk != 1 {
		t.Errorf("esperaba foreign_keys=1 (para que el CASCADE de embeddings funcione), obtuve %d", fk)
	}
}

func TestEmbeddingsCascadeOnDelete(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "t", "contenido con embedding", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`INSERT INTO embeddings(observation_id, vector) VALUES('o1', ?)`, []byte{0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`DELETE FROM observations WHERE id='o1'`); err != nil {
		t.Fatal(err)
	}
	var n int
	e.db.QueryRow(`SELECT COUNT(*) FROM embeddings WHERE observation_id='o1'`).Scan(&n)
	if n != 0 {
		t.Errorf("el embedding debió borrarse en cascada al borrar la observación, quedaron %d", n)
	}
}
