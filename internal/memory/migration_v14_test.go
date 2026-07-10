package memory

import (
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// TestMigrationV14RebuildsRelationsPreservingData valida el escenario más delicado de v14: una
// base en v13 con relaciones bi-temporales YA pobladas —incluyendo una auto-referencia
// superseded_by (relations.id → relations.id) y una fila invalidada— se migra al esquema con
// project_id SIN perder datos ni romper referencias. El rebuild de tabla con FKs a entities es
// la parte de mayor riesgo del track; este test es su guard.
func TestMigrationV14RebuildsRelationsPreservingData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// FKs ON como en producción, para ejercer el rebuild bajo enforcement real.
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(db, migrationsUpTo(13)); err != nil {
		t.Fatalf("migrar a v13: %v", err)
	}

	// Entidades para satisfacer las FKs de relations.
	for id, name := range map[int]string{1: "Ana", 2: "Acme", 3: "Globex"} {
		if _, err := db.Exec(`INSERT INTO entities (id, name, norm) VALUES (?,?,?)`, id, name, normalizeForSim(name)); err != nil {
			t.Fatalf("seed entity %s: %v", name, err)
		}
	}
	// Relación 1: Ana-works_at-Acme, INVALIDADA y superseded_by=2 (auto-referencia).
	if _, err := db.Exec(`INSERT INTO relations (id, from_id, predicate, to_id, valid_from, valid_to, invalidated_at, superseded_by)
		VALUES (1, 1, 'works_at', 2, '2020-01-01 00:00:00', '2021-01-01 00:00:00', '2021-01-01 00:00:00', 2)`); err != nil {
		t.Fatalf("seed relation 1: %v", err)
	}
	// Relación 2: Ana-works_at-Globex, VIVA.
	if _, err := db.Exec(`INSERT INTO relations (id, from_id, predicate, to_id, valid_from)
		VALUES (2, 1, 'works_at', 3, '2021-01-01 00:00:00')`); err != nil {
		t.Fatalf("seed relation 2: %v", err)
	}
	db.Close()

	// Abrir con el motor completo: migra a v14 (rebuild de relations).
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine (migra a v14): %v", err)
	}
	defer e.Close()

	if v, _ := e.schemaVersion(); v != 14 {
		t.Fatalf("tras migrar user_version=%d, esperaba 14", v)
	}

	// Las 2 filas sobrevivieron, con project_id='' (federado) y la auto-referencia intacta.
	var count int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM relations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("esperaba 2 relaciones tras el rebuild, obtuve %d", count)
	}
	var pid string
	var superseded sql.NullInt64
	var invalidated sql.NullString
	if err := e.db.QueryRow(`SELECT project_id, superseded_by, invalidated_at FROM relations WHERE id=1`).
		Scan(&pid, &superseded, &invalidated); err != nil {
		t.Fatalf("leer relación 1 migrada: %v", err)
	}
	if pid != "" {
		t.Errorf("relación legacy debe quedar project_id='' (federada), obtuve %q", pid)
	}
	if !superseded.Valid || superseded.Int64 != 2 {
		t.Errorf("superseded_by=2 (auto-referencia) debe sobrevivir el rebuild, obtuve %v", superseded)
	}
	if !invalidated.Valid {
		t.Error("la ventana de invalidación de la relación 1 debe sobrevivir")
	}

	// El nuevo UNIQUE(from_id,predicate,to_id,project_id) permite el MISMO triple en otro proyecto
	// (lo que antes colisionaba). El triple (1,works_at,3) ya existe en '' ⇒ insertarlo en 'crm' OK.
	if _, err := e.db.Exec(`INSERT INTO relations (from_id, predicate, to_id, project_id, valid_from)
		VALUES (1, 'works_at', 3, 'crm', datetime('now'))`); err != nil {
		t.Fatalf("el mismo triple en otro proyecto debe poder coexistir tras v14: %v", err)
	}

	// El índice idx_rel_live se recreó tras el DROP de la tabla vieja.
	var idxName string
	if err := e.db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_rel_live'`).Scan(&idxName); err != nil {
		t.Fatalf("esperaba el índice idx_rel_live recreado: %v", err)
	}
}
