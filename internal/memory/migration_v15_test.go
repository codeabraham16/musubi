package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// TestMigrationV15AddsProjectIdPreservingData prueba que la migración v15 (Track 18) agrega
// project_id a telemetry_logs y skill_decisions SIN perder datos, con las filas legacy en el
// espacio federado (project_id vacío). Siembra una base pre-v15 (v14), inserta filas sin él,
// migra y verifica columna + default + preservación.
func TestMigrationV15AddsProjectIdPreservingData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// Migrar hasta v14 (version-explícito, robusto a migraciones futuras): telemetry_logs y
	// skill_decisions AÚN sin project_id.
	if err := applyMigrations(db, migrationsUpTo(14)); err != nil {
		t.Fatalf("migrar hasta v14: %v", err)
	}
	// Sembrar filas SIN project_id (esquema pre-v15).
	mustExec(t, db, `INSERT INTO telemetry_logs (file_path, error_message, suggested_patch, resolved) VALUES (?,?,?,0)`,
		"x.go", "boom", "fix")
	mustExec(t, db, `INSERT INTO skill_decisions (skill_id, name, decision, reason) VALUES (?,?,?,?)`,
		"go-gin", "Go Gin", "accepted", "ok")

	// Aplicar v15.
	if err := applyMigrations(db, migrationsUpTo(15)); err != nil {
		t.Fatalf("aplicar v15: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 15 {
		t.Fatalf("user_version=%d tras v15, esperaba 15", v)
	}

	// Datos preservados + project_id='' (federado) por default en las filas legacy.
	var pid, msg string
	if err := db.QueryRow(`SELECT project_id, error_message FROM telemetry_logs`).Scan(&pid, &msg); err != nil {
		t.Fatalf("leer telemetry_logs tras v15: %v", err)
	}
	if pid != "" || msg != "boom" {
		t.Errorf("telemetry_logs: project_id=%q error_message=%q, esperaba \"\" y \"boom\"", pid, msg)
	}
	var spid, dec string
	if err := db.QueryRow(`SELECT project_id, decision FROM skill_decisions`).Scan(&spid, &dec); err != nil {
		t.Fatalf("leer skill_decisions tras v15: %v", err)
	}
	if spid != "" || dec != "accepted" {
		t.Errorf("skill_decisions: project_id=%q decision=%q, esperaba \"\" y \"accepted\"", spid, dec)
	}
	db.Close()
}
