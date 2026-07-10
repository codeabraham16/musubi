package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// TestBackupToProducesRestorableSnapshot es el TEST DE RESTORE en CI (DR, Track 17): prueba que el
// snapshot de BackupTo (VACUUM INTO) no es sólo un archivo que existe, sino una base REALMENTE
// restaurable — se abre como una base nueva, pasa integrity_check, está en el esquema esperado y
// conserva los datos (observación + hecho + memoria de código). Sin esto, "tenemos backups" es una
// afirmación no verificada; con esto, cada corrida de CI ejercita el camino de recuperación.
func TestBackupToProducesRestorableSnapshot(t *testing.T) {
	// Base de origen con datos representativos de las 3 familias de memoria.
	root := t.TempDir()
	src, err := NewDbEngine(root)
	if err != nil {
		t.Fatalf("NewDbEngine origen: %v", err)
	}
	if err := src.SaveObservation("obs-1", "t/dr", "memoria a respaldar y restaurar", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := src.SaveFact("Backup", "protege", "MemoriaCentral", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := src.SaveCodeMemory(CodeMemory{Path: "dr.go", Gist: "gist de dr", Tokens: 3}); err != nil {
		t.Fatal(err)
	}
	wantVer, _ := src.schemaVersion()

	// Snapshot consistente (VACUUM INTO) a un directorio aparte.
	snap, err := src.BackupTo(t.TempDir())
	if err != nil {
		t.Fatalf("BackupTo: %v", err)
	}
	src.Close()

	// RESTORE: colocar el snapshot como la base de un workspace nuevo y abrirlo con el motor.
	restoreRoot := t.TempDir()
	dbPath := filepath.Join(restoreRoot, config.DirName, config.DBFile)
	mkdirForDB(t, dbPath)
	data, err := os.ReadFile(snap)
	if err != nil {
		t.Fatalf("leer snapshot: %v", err)
	}
	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		t.Fatalf("restaurar snapshot: %v", err)
	}

	restored, err := NewDbEngine(restoreRoot)
	if err != nil {
		t.Fatalf("abrir la base restaurada: %v", err)
	}
	t.Cleanup(func() { restored.Close() })

	// (1) Integridad SQLite.
	if r := checkDBIntegrity(restored); r.Status != "ok" {
		t.Errorf("integrity_check tras restore: %s — %s", r.Status, r.Message)
	}
	// (2) Mismo esquema (no degradó ni disparó ErrSchemaTooNew).
	if gotVer, _ := restored.schemaVersion(); gotVer != wantVer {
		t.Errorf("esquema restaurado v%d, esperaba v%d", gotVer, wantVer)
	}
	// (3) Los datos de las 3 familias sobrevivieron.
	if n := countRows(t, restored, "observations"); n != 1 {
		t.Errorf("esperaba 1 observación restaurada, obtuve %d", n)
	}
	facts, err := restored.RecallFacts("Backup", 1, 10, "", "")
	if err != nil || len(facts.Facts) != 1 || facts.Facts[0].Object != "MemoriaCentral" {
		t.Errorf("el hecho no sobrevivió el restore: %+v (err=%v)", facts.Facts, err)
	}
	if cm, ok, err := restored.GetCodeMemoryCtx(context.Background(), "dr.go"); err != nil || !ok || cm.Gist != "gist de dr" {
		t.Errorf("la memoria de código no sobrevivió el restore: ok=%v gist=%q err=%v", ok, cm.Gist, err)
	}
}
