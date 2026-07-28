package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// TestMigrationV14SweepsOrphanRelations: una base en v13 con una relación LEGACY huérfana (to_id
// apuntando a una entidad inexistente — posible en datos anteriores al CASCADE) migra a v14 SIN
// brickear el arranque. El rebuild corre con foreign_keys=ON; sin el barrido previo de huérfanas, el
// INSERT..SELECT fallaría por la FK y NewDbEngine devolvería error (la base no abriría). Guard de la
// auditoría v0.98.0. La relación VÁLIDA se preserva; la huérfana se descarta.
func TestMigrationV14SweepsOrphanRelations(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(db, migrationsUpTo(13)); err != nil {
		t.Fatalf("migrar a v13: %v", err)
	}
	// Una sola entidad; la tabla relations pre-v14 no tiene FK, así que se puede sembrar una huérfana.
	if _, err := db.Exec(`INSERT INTO entities (id, name, norm) VALUES (1,'Ana',?)`, normalizeForSim("Ana")); err != nil {
		t.Fatal(err)
	}
	// Relación VÁLIDA (1→1) y relación HUÉRFANA (to_id=999 inexistente).
	if _, err := db.Exec(`INSERT INTO relations (from_id, predicate, to_id, valid_from) VALUES (1,'sabe',1,'2020-01-01 00:00:00')`); err != nil {
		t.Fatalf("seed relación válida: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO relations (from_id, predicate, to_id, valid_from) VALUES (1,'sabe',999,'2020-01-01 00:00:00')`); err != nil {
		t.Fatalf("seed relación huérfana: %v", err)
	}
	db.Close()

	// v14 corre acá bajo FK on (DSN). Sin el guard, esto devolvería error y la base no abriría.
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine debía migrar v14 sin brickear ante una relación huérfana: %v", err)
	}
	defer e.Close()

	if v, _ := e.schemaVersion(); v != latestSchemaVersion() {
		t.Fatalf("tras migrar user_version=%d, esperaba %d", v, latestSchemaVersion())
	}
	// La válida sobrevive; la huérfana se barrió.
	var count int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM relations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("esperaba 1 relación (válida preservada, huérfana barrida), obtuve %d", count)
	}
	var orphans int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM relations WHERE to_id=999`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Errorf("la relación huérfana debió barrerse, quedan %d", orphans)
	}
}
