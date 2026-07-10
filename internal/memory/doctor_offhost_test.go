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

// TestCheckOffhostBackupErrorMarker valida el cierre del falso-negativo (Track 18): con la marca
// de error presente, doctor AVISA aunque el backup nunca haya tenido éxito (antes daba 'ok' porque
// no había marca de éxito). Y si el error es más VIEJO que el último éxito (se recuperó) ⇒ ok.
func TestCheckOffhostBackupErrorMarker(t *testing.T) {
	e := newTestEngine(t)
	backups := filepath.Join(filepath.Dir(e.path), "backups")
	if err := os.MkdirAll(backups, 0755); err != nil {
		t.Fatal(err)
	}
	okMarker := filepath.Join(backups, offhostMarkerName)
	errMarker := filepath.Join(backups, offhostErrorMarkerName)

	// (1) Error presente y NINGÚN éxito ⇒ warning ("configurado pero nunca funcionó"). Es el
	// escenario exacto del baseline: BACKUP_REMOTE mal tipeado, el timer falla cada noche.
	if err := os.WriteFile(errMarker, []byte("2026-07-10T03:30:00Z BACKUP_REMOTE vacío\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if r := checkOffhostBackup(e); r.Status != "warning" {
		t.Errorf("error sin éxito previo esperaba warning, obtuve %s — %s", r.Status, r.Message)
	}

	// (2) Éxito MÁS NUEVO que el error (se recuperó) ⇒ ok.
	if err := os.WriteFile(okMarker, []byte("2026-07-10T04:00:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(errMarker, past, past); err != nil {
		t.Fatal(err)
	}
	if r := checkOffhostBackup(e); r.Status != "ok" {
		t.Errorf("éxito posterior al error esperaba ok, obtuve %s — %s", r.Status, r.Message)
	}

	// (3) Error MÁS NUEVO que el último éxito (volvió a fallar) ⇒ warning.
	now := time.Now()
	if err := os.Chtimes(errMarker, now, now); err != nil {
		t.Fatal(err)
	}
	stale := now.Add(-2 * time.Hour)
	if err := os.Chtimes(okMarker, stale, stale); err != nil {
		t.Fatal(err)
	}
	if r := checkOffhostBackup(e); r.Status != "warning" {
		t.Errorf("error posterior al último éxito esperaba warning, obtuve %s — %s", r.Status, r.Message)
	}
}
