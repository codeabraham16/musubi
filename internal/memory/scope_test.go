package memory

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// migrationsUpTo devuelve las migraciones reales con versión <= v, para simular una base
// en un esquema anterior y luego migrarla con el binario completo.
func migrationsUpTo(v int) []migration {
	var out []migration
	for _, m := range schemaMigrations() {
		if m.version <= v {
			out = append(out, m)
		}
	}
	return out
}

// TestMigrationV10AddsScopeProjectAndBackfills valida el escenario (a): una base en v9,
// con una fila previa, migrada a v10 gana las columnas scope/project_id; la fila vieja
// queda scope='local' y project_id NULL (backward-compat), y reabrir no falla.
func TestMigrationV10AddsScopeProjectAndBackfills(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)

	// Base a v9 (sin scope/project_id), con una fila previa.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(db, migrationsUpTo(9)); err != nil {
		t.Fatalf("migrar a v9: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 9 {
		t.Fatalf("esperaba user_version=9, obtuve %d", v)
	}
	if _, err := db.Exec(`INSERT INTO observations (id, topic_key, content) VALUES (?,?,?)`,
		"vieja", "t", "fila anterior a scope"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Abrir con el motor completo: migra a v10.
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine (migra a v10): %v", err)
	}
	defer e.Close()

	if v, _ := e.schemaVersion(); v != latestSchemaVersion() {
		t.Errorf("tras migrar user_version=%d, esperaba %d", v, latestSchemaVersion())
	}
	// Las columnas existen y la fila vieja quedó con el default backward-compat.
	var scope string
	var projectID sql.NullString
	if err := e.db.QueryRow(`SELECT scope, project_id FROM observations WHERE id=?`, "vieja").
		Scan(&scope, &projectID); err != nil {
		t.Fatalf("leer scope/project_id de la fila vieja: %v", err)
	}
	if scope != ScopeLocal {
		t.Errorf("fila vieja scope=%q, esperaba 'local'", scope)
	}
	if projectID.Valid {
		t.Errorf("fila vieja project_id=%q, esperaba NULL", projectID.String)
	}

	// El índice idx_obs_project existe.
	var idxName string
	if err := e.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_obs_project'`).Scan(&idxName); err != nil {
		t.Fatalf("esperaba el índice idx_obs_project: %v", err)
	}

	// Reabrir no falla ni cambia la versión.
	e.Close()
	e2, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("reapertura tras v10: %v", err)
	}
	defer e2.Close()
	if v, _ := e2.schemaVersion(); v != latestSchemaVersion() {
		t.Errorf("reapertura user_version=%d, esperaba %d", v, latestSchemaVersion())
	}
}

// scopeAndProject lee scope + project_id de una observación (project_id como NullString).
func scopeAndProject(t *testing.T, e *DbEngine, id string) (string, sql.NullString) {
	t.Helper()
	var scope string
	var pid sql.NullString
	if err := e.db.QueryRow(`SELECT scope, project_id FROM observations WHERE id=?`, id).Scan(&scope, &pid); err != nil {
		t.Fatalf("leer scope/project_id de %q: %v", id, err)
	}
	return scope, pid
}

// TestSaveDefaultsToLocalAndStampsProject valida (b): un save sin scope queda 'local' y
// estampa el project_id inyectado por el engine.
func TestSaveDefaultsToLocalAndStampsProject(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("proyecto-x")
	if err := e.SaveObservation("o1", "t", "sin scope declarado", nil); err != nil {
		t.Fatal(err)
	}
	scope, pid := scopeAndProject(t, e, "o1")
	if scope != ScopeLocal {
		t.Errorf("scope=%q, esperaba 'local' por default", scope)
	}
	if !pid.Valid || pid.String != "proyecto-x" {
		t.Errorf("project_id=%v, esperaba 'proyecto-x' estampado", pid)
	}
}

// TestSaveSharedScope valida (c): un save con scope='shared' persiste 'shared'.
func TestSaveSharedScope(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("proyecto-x")
	if err := e.SaveObservationTyped("o2", "t", "compartible", 1.0, "", ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if scope, _ := scopeAndProject(t, e, "o2"); scope != ScopeShared {
		t.Errorf("scope=%q, esperaba 'shared'", scope)
	}
}

// TestPromoteObservation valida (e): local→shared, idempotencia y error de id inexistente.
func TestPromoteObservation(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("p1", "t", "arranca local", nil); err != nil {
		t.Fatal(err)
	}
	if scope, _ := scopeAndProject(t, e, "p1"); scope != ScopeLocal {
		t.Fatalf("precondición: esperaba local, obtuve %q", scope)
	}
	if err := e.PromoteObservation("p1"); err != nil {
		t.Fatalf("promover local→shared: %v", err)
	}
	if scope, _ := scopeAndProject(t, e, "p1"); scope != ScopeShared {
		t.Errorf("tras promover scope=%q, esperaba 'shared'", scope)
	}
	// Segunda vez: idempotente, sin error.
	if err := e.PromoteObservation("p1"); err != nil {
		t.Errorf("segunda promoción debe ser no-op exitoso, obtuve: %v", err)
	}
	// Id inexistente: error tipado.
	err := e.PromoteObservation("no-existe")
	if err == nil {
		t.Fatal("esperaba error al promover un id inexistente")
	}
	if !errors.Is(err, ErrObservationNotFound) {
		t.Errorf("esperaba ErrObservationNotFound, obtuve: %v", err)
	}
}
