package memory

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"

	_ "modernc.org/sqlite"
)

// TestFreshDBStartsAtLatestVersion verifica que una base nueva quede en la última
// versión de esquema conocida por el binario (no en 0).
func TestFreshDBStartsAtLatestVersion(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer e.Close()

	v, err := e.schemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if want := latestSchemaVersion(); v != want {
		t.Errorf("user_version=%d, esperaba la última versión %d", v, want)
	}
}

// TestReopenIsNoOp verifica que reabrir una base ya migrada no falle ni cambie la
// versión (las migraciones ya aplicadas se saltan por user_version).
func TestReopenIsNoOp(t *testing.T) {
	root := t.TempDir()

	e1, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("primera apertura: %v", err)
	}
	v1, _ := e1.schemaVersion()
	e1.Close()

	e2, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("reapertura: %v", err)
	}
	defer e2.Close()
	v2, _ := e2.schemaVersion()

	if v1 != v2 {
		t.Errorf("la versión cambió al reabrir: %d -> %d", v1, v2)
	}
	if v2 != latestSchemaVersion() {
		t.Errorf("user_version=%d tras reabrir, esperaba %d", v2, latestSchemaVersion())
	}
}

// TestLegacyDBBumpsVersionAndKeepsData verifica que una base preexistente
// (user_version=0, esquema viejo) avance a la última versión SIN perder datos.
func TestLegacyDBBumpsVersionAndKeepsData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)

	// Base "vieja": observations de 4 columnas, sin user_version (queda en 0), 1 fila.
	mkdirForDB(t, dbPath)
	old, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, old, `CREATE TABLE observations (
		id TEXT PRIMARY KEY, topic_key TEXT NOT NULL, content TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
	mustExec(t, old, `INSERT INTO observations (id, topic_key, content) VALUES (?,?,?)`,
		"legacy1", "t", "dato viejo que debe sobrevivir")
	var v0 int
	if err := old.QueryRow(`PRAGMA user_version`).Scan(&v0); err != nil {
		t.Fatal(err)
	}
	if v0 != 0 {
		t.Fatalf("base vieja debería tener user_version=0, tenía %d", v0)
	}
	old.Close()

	// Abrir con el motor nuevo: corre migraciones sobre la base existente.
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine sobre base vieja: %v", err)
	}
	defer e.Close()

	if v, _ := e.schemaVersion(); v != latestSchemaVersion() {
		t.Errorf("user_version=%d tras migrar base vieja, esperaba %d", v, latestSchemaVersion())
	}
	var content string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id=?`, "legacy1").Scan(&content); err != nil {
		t.Fatalf("la fila vieja no sobrevivió a la migración: %v", err)
	}
	if content != "dato viejo que debe sobrevivir" {
		t.Errorf("contenido alterado por la migración: %q", content)
	}
}

// TestApplyMigrationsRunsPendingOnceAndIsIdempotent valida el runner con una
// migración SINTÉTICA por encima de las reales: se aplica una sola vez, sube
// user_version y volver a correr la lista no la re-aplica.
func TestApplyMigrationsRunsPendingOnceAndIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "synthetic.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	calls := 0
	synthetic := []migration{
		{version: 1, name: "baseline", up: func(x execQuerier) error { return initSchemaOn(x) }},
		{version: 2, name: "tabla-foo", up: func(x execQuerier) error {
			calls++
			_, err := x.Exec(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`)
			return err
		}},
	}

	if err := applyMigrations(db, synthetic); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Errorf("user_version=%d, esperaba 2", v)
	}
	// La tabla nueva existe.
	if _, err := db.Exec(`INSERT INTO foo (id) VALUES (1)`); err != nil {
		t.Errorf("la migración v2 no creó la tabla foo: %v", err)
	}

	// Segunda corrida: ninguna migración pendiente, `up` de v2 no vuelve a ejecutarse.
	if err := applyMigrations(db, synthetic); err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	if calls != 1 {
		t.Errorf("la migración v2 se ejecutó %d veces, esperaba 1", calls)
	}
}

// TestApplyMigrationsRollsBackOnFailure verifica que si una migración falla, su
// transacción se revierte y user_version NO avanza (se reintenta en la próxima).
func TestApplyMigrationsRollsBackOnFailure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "rollback.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	bad := []migration{
		{version: 1, name: "ok", up: func(x execQuerier) error {
			_, err := x.Exec(`CREATE TABLE a (id INTEGER PRIMARY KEY)`)
			return err
		}},
		{version: 2, name: "falla", up: func(x execQuerier) error {
			// DDL inválido a propósito.
			_, err := x.Exec(`CREATE TABLE`)
			return err
		}},
	}

	if err := applyMigrations(db, bad); err == nil {
		t.Fatal("esperaba error de la migración v2, no hubo")
	}
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	// v1 aplicó OK (user_version=1); v2 falló y revirtió, así que NO llega a 2.
	if v != 1 {
		t.Errorf("user_version=%d tras fallo en v2, esperaba 1 (v1 commiteada, v2 revertida)", v)
	}
}

// --- helpers ---

func mkdirForDB(t *testing.T, dbPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatal(err)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
