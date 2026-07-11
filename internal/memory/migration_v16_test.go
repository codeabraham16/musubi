package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// TestMigrationV16AddsAuthorPreservingData prueba que la migración v16 (C5.1) agrega la columna
// author a observations SIN perder datos: las filas legacy quedan con author vacío (sin atribución,
// backward-compat R3.6). Siembra una base pre-v16 (v15), inserta una observación sin author,
// migra y verifica columna + default + preservación.
func TestMigrationV16AddsAuthorPreservingData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// Migrar hasta v15: observations AÚN sin columna author.
	if err := applyMigrations(db, migrationsUpTo(15)); err != nil {
		t.Fatalf("migrar hasta v15: %v", err)
	}
	// Sembrar una observación SIN author (esquema pre-v16). id/topic_key/content son los únicos
	// NOT NULL sin default de la tabla baseline; el resto es aditivo/nullable.
	mustExec(t, db, `INSERT INTO observations (id, topic_key, content) VALUES (?,?,?)`,
		"o-legacy", "t/x", "contenido viejo")

	// Aplicar v16.
	if err := applyMigrations(db, migrationsUpTo(16)); err != nil {
		t.Fatalf("aplicar v16: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 16 {
		t.Fatalf("user_version=%d tras v16, esperaba 16", v)
	}

	// La fila legacy sobrevive con author vacío (default): backward-compat, sin atribución.
	var author, content string
	if err := db.QueryRow(`SELECT author, content FROM observations WHERE id='o-legacy'`).Scan(&author, &content); err != nil {
		t.Fatalf("leer observación tras v16: %v", err)
	}
	if author != "" || content != "contenido viejo" {
		t.Errorf("observación legacy: author=%q content=%q, esperaba \"\" y \"contenido viejo\"", author, content)
	}
	db.Close()
}
