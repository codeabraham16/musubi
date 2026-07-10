package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestCheckOffhostBackupDeadMansSwitch valida el dead-man's-switch del backup off-host (DR, Track
// 17): marca ausente ⇒ ok (instancia local / no configurado, sin falso positivo); marca fresca ⇒
// ok; marca más vieja que el umbral ⇒ warning (el timer dejó de shipear).
func TestCheckOffhostBackupDeadMansSwitch(t *testing.T) {
	e := newTestEngine(t)
	backups := filepath.Join(filepath.Dir(e.path), "backups")
	if err := os.MkdirAll(backups, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backups, offhostMarkerName)

	// (1) Sin marca ⇒ ok (no alarma en máquina de desarrollo).
	if r := checkOffhostBackup(e); r.Status != "ok" {
		t.Errorf("sin marca esperaba ok, obtuve %s — %s", r.Status, r.Message)
	}

	// (2) Marca fresca ⇒ ok.
	if err := os.WriteFile(marker, []byte("2026-07-10T04:00:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if r := checkOffhostBackup(e); r.Status != "ok" {
		t.Errorf("marca fresca esperaba ok, obtuve %s — %s", r.Status, r.Message)
	}

	// (3) Marca vieja (mtime hace 72h > 48h) ⇒ warning (dead-man's-switch dispara).
	old := time.Now().Add(-72 * time.Hour)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	if r := checkOffhostBackup(e); r.Status != "warning" {
		t.Errorf("marca obsoleta esperaba warning, obtuve %s — %s", r.Status, r.Message)
	}
}
