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
	for _, c := range []string{"gist", "content_hash", "tokens", "last_accessed", "access_count", "importance", "archived"} {
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

// TestRecomputeTokensWhenEstimatorChanges verifica que, si la versión del
// estimador de tokens cambió, NewDbEngine recompute la columna `tokens` de TODAS
// las filas (no solo las sin gist) y deje la versión actualizada.
func TestRecomputeTokensWhenEstimatorChanges(t *testing.T) {
	root := t.TempDir()

	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	content := "func add(a, b int) int { return a + b }" // código: estimador ≠ runas/4
	if err := e.SaveObservation("c1", "t", content, nil); err != nil {
		t.Fatal(err)
	}
	// Simular que la fila quedó con tokens de un estimador viejo y forzar versión vieja.
	if _, err := e.db.Exec(`UPDATE observations SET tokens=1 WHERE id=?`, "c1"); err != nil {
		t.Fatal(err)
	}
	if err := e.SetMeta(metaTokenEstimatorVersion, "estimador-viejo"); err != nil {
		t.Fatal(err)
	}
	e.Close()

	// Reabrir: debe detectar el cambio de versión y recomputar.
	e2, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("reabrir error: %v", err)
	}
	defer e2.Close()

	var tokens int
	if err := e2.db.QueryRow(`SELECT tokens FROM observations WHERE id=?`, "c1").Scan(&tokens); err != nil {
		t.Fatal(err)
	}
	if want := EstimateTokens(content); tokens != want {
		t.Errorf("tokens=%d no fue recomputado al estimador actual (%d)", tokens, want)
	}
	if v, _, _ := e2.GetMeta(metaTokenEstimatorVersion); v != tokenEstimatorVersion {
		t.Errorf("la versión del estimador debió actualizarse a %q, quedó %q", tokenEstimatorVersion, v)
	}
}
