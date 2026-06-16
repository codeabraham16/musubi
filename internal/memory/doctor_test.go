package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func statusOf(rep DiagnoseReport, code string) string {
	for _, c := range rep.Checks {
		if c.Code == code {
			return c.Status
		}
	}
	return "missing"
}

func TestDiagnoseDBSanaTodoOK(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "topic/x", "una observación sana", nil); err != nil {
		t.Fatal(err)
	}
	rep, err := e.Diagnose()
	if err != nil {
		t.Fatalf("Diagnose error: %v", err)
	}
	if rep.Status != "ok" {
		t.Errorf("una DB sana debe diagnosticar ok, obtuve %q: %+v", rep.Status, rep.Checks)
	}
	for _, c := range rep.Checks {
		if c.Status != "ok" {
			t.Errorf("check %s no ok: %+v", c.Code, c)
		}
	}
}

func TestFtsConsistencyDetectaYRepara(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "topic/x", "contenido para fts", nil); err != nil {
		t.Fatal(err)
	}
	// Inyectar una fila FTS duplicada (simula el bug de FTS desincronizado).
	if _, err := e.db.Exec(`INSERT INTO observations_fts(id, topic_key, content) VALUES('a','topic/x','contenido para fts')`); err != nil {
		t.Fatal(err)
	}
	rep, _ := e.Diagnose()
	if statusOf(rep, "fts_consistency") == "ok" {
		t.Errorf("se esperaba que fts_consistency detectara la desincronización: %+v", rep.Checks)
	}

	res, err := e.Repair("fts_consistency", "apply")
	if err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	if !res.Applied {
		t.Error("el repair en modo apply debe aplicarse")
	}
	rep2, _ := e.Diagnose()
	if statusOf(rep2, "fts_consistency") != "ok" {
		t.Errorf("tras reparar, fts_consistency debe quedar ok: %+v", rep2.Checks)
	}
}

func TestMissingDigestsDetectaYRepara(t *testing.T) {
	e := newTestEngine(t)
	// Insert crudo sin gist/content_hash (evita saveObservation que los calcula).
	if _, err := e.db.Exec(`INSERT INTO observations(id, topic_key, content) VALUES('raw','t','sin digest')`); err != nil {
		t.Fatal(err)
	}
	rep, _ := e.Diagnose()
	if statusOf(rep, "missing_digests") == "ok" {
		t.Errorf("se esperaba detectar digests faltantes: %+v", rep.Checks)
	}
	if _, err := e.Repair("missing_digests", "apply"); err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	rep2, _ := e.Diagnose()
	if statusOf(rep2, "missing_digests") != "ok" {
		t.Errorf("tras reparar, missing_digests debe quedar ok: %+v", rep2.Checks)
	}
}

func TestOrphanRelationsDetectaYRepara(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.db.Exec(`INSERT INTO observation_relations(id, source_id, target_id, relation, status) VALUES('r1','fantasma','otro','pending','pending')`); err != nil {
		t.Fatal(err)
	}
	rep, _ := e.Diagnose()
	if statusOf(rep, "orphan_relations") == "ok" {
		t.Errorf("se esperaba detectar relaciones huérfanas: %+v", rep.Checks)
	}
	if _, err := e.Repair("orphan_relations", "apply"); err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	rep2, _ := e.Diagnose()
	if statusOf(rep2, "orphan_relations") != "ok" {
		t.Errorf("tras reparar, orphan_relations debe quedar ok: %+v", rep2.Checks)
	}
}

func TestRepairDryRunNoMuta(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.db.Exec(`INSERT INTO observation_relations(id, source_id, target_id, relation, status) VALUES('r1','fantasma','otro','pending','pending')`); err != nil {
		t.Fatal(err)
	}
	res, err := e.Repair("orphan_relations", "dry-run")
	if err != nil {
		t.Fatalf("Repair dry-run error: %v", err)
	}
	if res.Applied {
		t.Error("dry-run no debe aplicar cambios")
	}
	if res.Affected < 1 {
		t.Errorf("dry-run debe reportar las filas que tocaría, obtuve %d", res.Affected)
	}
	// La relación huérfana debe seguir ahí.
	rep, _ := e.Diagnose()
	if statusOf(rep, "orphan_relations") == "ok" {
		t.Error("dry-run no debió eliminar la relación huérfana")
	}
}

func TestRepairApplyCreaBackup(t *testing.T) {
	dir := t.TempDir()
	e, err := NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	if _, err := e.db.Exec(`INSERT INTO observation_relations(id, source_id, target_id, relation, status) VALUES('r1','x','y','pending','pending')`); err != nil {
		t.Fatal(err)
	}
	res, err := e.Repair("orphan_relations", "apply")
	if err != nil {
		t.Fatalf("Repair error: %v", err)
	}
	if res.BackupPath == "" {
		t.Fatal("apply debe crear un backup y reportar su ruta")
	}
	if _, err := os.Stat(res.BackupPath); err != nil {
		t.Errorf("el backup debe existir en disco: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(res.BackupPath), ".musubi/backups/") {
		t.Errorf("el backup debe vivir en .musubi/backups/, obtuve %q", res.BackupPath)
	}
}
