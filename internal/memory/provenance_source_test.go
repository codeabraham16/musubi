package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// TestMigrationV20AddsRelationSourcePreservingData prueba que la migración v20 (pilar Cognición · F0)
// agrega la columna `source` a relations SIN perder aristas: las filas legacy quedan con
// source='agent' (backward-compat). Siembra una base pre-v20 (v19), inserta una arista, migra y
// verifica columna + default + preservación + user_version.
func TestMigrationV20AddsRelationSourcePreservingData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Migrar hasta v19: relations AÚN sin columna source.
	if err := applyMigrations(db, migrationsUpTo(19)); err != nil {
		t.Fatalf("migrar hasta v19: %v", err)
	}
	// Sembrar dos entidades + una arista (esquema pre-v20).
	mustExec(t, db, `INSERT INTO entities (id, name, norm) VALUES (1,'A','a'),(2,'B','b')`)
	mustExec(t, db, `INSERT INTO relations (from_id, predicate, to_id, project_id, valid_from) VALUES (1,'rel',2,'','2026-01-01 00:00:00')`)

	// Aplicar v20.
	if err := applyMigrations(db, migrationsUpTo(20)); err != nil {
		t.Fatalf("aplicar v20: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 20 {
		t.Fatalf("user_version=%d tras v20, esperaba 20", v)
	}

	// La arista legacy sobrevive con source='agent' (default).
	var source, predicate string
	if err := db.QueryRow(`SELECT source, predicate FROM relations WHERE from_id=1 AND to_id=2`).Scan(&source, &predicate); err != nil {
		t.Fatalf("leer arista tras v20: %v", err)
	}
	if source != "agent" || predicate != "rel" {
		t.Errorf("arista legacy: source=%q predicate=%q, esperaba \"agent\" y \"rel\"", source, predicate)
	}
}

// TestSaveFactFromSourcedSealsProvenance prueba el sellado de procedencia: SaveFactFrom default a
// 'agent'; SaveFactFromSourced sella el origen explícito; re-afirmar NO pisa la procedencia del
// primer afirmante (invariante de auditoría); source vacío → 'agent'.
func TestSaveFactFromSourcedSealsProvenance(t *testing.T) {
	e := newTestEngine(t)

	if _, err := e.SaveFactFrom("", "musubi", "corre_en", "server", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := factSource(t, e, "musubi", "corre_en", "server"); got != "agent" {
		t.Errorf("SaveFactFrom: source=%q, esperaba agent", got)
	}

	if _, err := e.SaveFactFromSourced("", "claude", "es_un", "llm", "", "llm-extract:potion-x", nil); err != nil {
		t.Fatal(err)
	}
	if got := factSource(t, e, "claude", "es_un", "llm"); got != "llm-extract:potion-x" {
		t.Errorf("SaveFactFromSourced: source=%q, esperaba llm-extract:potion-x", got)
	}

	// F1 (corroboración por precedencia): un 'agent' que re-afirma una propuesta LLM la PROMUEVE a
	// autoritativa (agent-wins). La procedencia sube hacia lo autoritativo, no se preserva la propuesta.
	if _, err := e.SaveFactFromSourced("", "claude", "es_un", "llm", "", "agent", nil); err != nil {
		t.Fatal(err)
	}
	if got := factSource(t, e, "claude", "es_un", "llm"); got != "agent" {
		t.Errorf("corroboración: source=%q, esperaba agent (promovido)", got)
	}

	// source vacío se normaliza a 'agent'.
	if _, err := e.SaveFactFromSourced("", "foo", "p", "bar", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if got := factSource(t, e, "foo", "p", "bar"); got != "agent" {
		t.Errorf("source vacío: %q, esperaba agent", got)
	}
}

// factSource lee la procedencia de la arista (subject, predicate, object), resolviendo las
// entidades por su forma normalizada.
func factSource(t *testing.T, e *DbEngine, subject, predicate, object string) string {
	t.Helper()
	var source string
	err := e.db.QueryRow(`
		SELECT r.source FROM relations r
		JOIN entities fe ON fe.id = r.from_id
		JOIN entities te ON te.id = r.to_id
		WHERE fe.norm=? AND r.predicate=? AND te.norm=?`,
		normalizeForSim(subject), predicate, normalizeForSim(object)).Scan(&source)
	if err != nil {
		t.Fatalf("factSource(%s,%s,%s): %v", subject, predicate, object, err)
	}
	return source
}
