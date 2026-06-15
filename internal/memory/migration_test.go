package memory

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"

	_ "modernc.org/sqlite"
)

// TestMigrationAddsColumnsAndBackfills simula una base con el esquema VIEJO
// (observations de 4 columnas) y verifica que NewDbEngine agregue las columnas
// de eficiencia sin perder datos y backfillee gist/tokens.
func TestMigrationAddsColumnsAndBackfills(t *testing.T) {
	root := t.TempDir()
	dbDir := filepath.Join(root, config.DirName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, config.DBFile)

	// Esquema viejo: observations con 4 columnas + una fila.
	old, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(`CREATE TABLE observations (
		id TEXT PRIMARY KEY,
		topic_key TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := old.Exec(
		`INSERT INTO observations (id, topic_key, content) VALUES (?,?,?)`,
		"legacy1", "t", "contenido viejo que debe backfillearse",
	); err != nil {
		t.Fatal(err)
	}
	old.Close()

	// Abrir con el motor nuevo: migra + backfillea.
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer e.Close()

	cols, err := e.observationColumns()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"gist", "content_hash", "tokens", "last_accessed", "access_count", "importance"} {
		if !cols[c] {
			t.Errorf("columna %q no fue agregada por la migración", c)
		}
	}

	// La fila vieja sobrevive y su gist/tokens fueron backfilleados.
	var gist string
	var tokens int
	if err := e.db.QueryRow(`SELECT gist, tokens FROM observations WHERE id=?`, "legacy1").
		Scan(&gist, &tokens); err != nil {
		t.Fatalf("query error: %v", err)
	}
	if gist == "" {
		t.Error("gist no fue backfilleado")
	}
	if tokens == 0 {
		t.Error("tokens no fue backfilleado")
	}
}
