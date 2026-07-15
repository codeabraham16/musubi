package memory

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
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
	// Desincronizar el índice del contenido (external-content): dropear el trigger de UPDATE y
	// cambiar el contenido en observations deja los tokens del índice apuntando al texto VIEJO —
	// justo el desync que el integrity-check rank=1 detecta y applyRebuildFTS repara.
	if _, err := e.db.Exec(`DROP TRIGGER observations_au`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET content='contenido TOTALMENTE distinto ahora' WHERE id='a'`); err != nil {
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
	// El backup (VACUUM INTO) debe ser una base SQLite VÁLIDA y abrible, con el estado
	// PRE-reparación (la relación 'r1' que el apply borró después debe seguir en el snapshot).
	bdb, err := sql.Open("sqlite", res.BackupPath)
	if err != nil {
		t.Fatalf("no se pudo abrir el backup: %v", err)
	}
	defer bdb.Close()
	var integrity string
	if err := bdb.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Errorf("integrity_check del backup = %q (err=%v), esperaba \"ok\"", integrity, err)
	}
	var n int
	if err := bdb.QueryRow(`SELECT COUNT(*) FROM observation_relations WHERE id='r1'`).Scan(&n); err != nil {
		t.Fatalf("no se pudo consultar el backup: %v", err)
	}
	if n != 1 {
		t.Errorf("el backup debe contener el estado pre-reparación (relación r1); filas=%d", n)
	}
}

// TestBackupToCustomDir verifica que BackupTo escribe un snapshot consistente en un
// directorio arbitrario (el que usa `musubi backup --out` para stagear antes de
// shipear off-host), lo crea si falta, y el snapshot es una base válida con los datos.
func TestBackupToCustomDir(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	if _, _, err := e.SaveObservationDedupedTyped("t/x", "un hecho memorable", 0.6, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(t.TempDir(), "staging", "nested") // aún no existe
	dest, err := e.BackupTo(outDir)
	if err != nil {
		t.Fatalf("BackupTo: %v", err)
	}
	if filepath.Dir(dest) != outDir {
		t.Errorf("el snapshot debe vivir en %q, está en %q", outDir, filepath.Dir(dest))
	}

	bdb, err := sql.Open("sqlite", dest)
	if err != nil {
		t.Fatalf("no se pudo abrir el snapshot: %v", err)
	}
	defer bdb.Close()
	var integrity string
	if err := bdb.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Errorf("integrity_check = %q (err=%v), esperaba \"ok\"", integrity, err)
	}
	var n int
	if err := bdb.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n); err != nil {
		t.Fatalf("consulta al snapshot: %v", err)
	}
	if n < 1 {
		t.Errorf("el snapshot debe contener las observaciones; filas=%d", n)
	}
}

// TestRegenGistsEsSeguroEIdempotente pinea el invariante R0: el gist es DERIVADO, así que
// regenerarlo NO puede perder información y correrlo dos veces da lo mismo que correrlo una.
func TestRegenGistsEsSeguroEIdempotente(t *testing.T) {
	e := newTestEngine(t)
	const contenido = "Titulo corto. Detalle que importa. Y mucho mas texto detras que ya no entra en el techo del gist."
	if err := e.SaveObservation("o1", "t/a", contenido, nil); err != nil {
		t.Fatal(err)
	}
	// Simular un gist VIEJO (cortado en la 1ª oración, como lo dejaba el extractor anterior).
	if _, err := e.db.Exec(`UPDATE observations SET gist=? WHERE id='o1'`, "Titulo corto."); err != nil {
		t.Fatal(err)
	}

	n, err := applyRegenGists(e)
	if err != nil {
		t.Fatalf("applyRegenGists: %v", err)
	}
	if n != 1 {
		t.Fatalf("se esperaba 1 gist recalculado, obtuve %d", n)
	}

	var gist, content, hash string
	if err := e.db.QueryRow(`SELECT gist, content, COALESCE(content_hash,'') FROM observations WHERE id='o1'`).
		Scan(&gist, &content, &hash); err != nil {
		t.Fatal(err)
	}
	if gist == "Titulo corto." {
		t.Error("el gist debía recalcularse y usar su presupuesto, no quedarse en la 1ª oración")
	}
	if content != contenido {
		t.Error("la reparación NO debe tocar el content: el gist es DERIVADO")
	}

	// Idempotencia (S.f): correr de nuevo no cambia nada.
	n2, err := applyRegenGists(e)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Errorf("la reparación debe ser IDEMPOTENTE: la 2ª corrida tocó %d gist(s)", n2)
	}
}
