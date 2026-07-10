package memory

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOperationalStatsBackupAge valida el gauge de staleness del backup (Track 18): -1 cuando no
// hay marca (instancia local / backup nunca exitoso), y una antigüedad >= 0 cuando la marca existe.
// Ese -1 es la señal que la alerta MusubiBackupOffhostStale usa para paginar "DR apagado".
func TestOperationalStatsBackupAge(t *testing.T) {
	e := newTestEngine(t)

	st, err := e.OperationalStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.BackupOffhostAgeSec != -1 {
		t.Errorf("sin marca de backup esperaba -1, obtuve %d", st.BackupOffhostAgeSec)
	}

	backups := filepath.Join(filepath.Dir(e.path), "backups")
	if err := os.MkdirAll(backups, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backups, offhostMarkerName), []byte("2026-07-10T04:00:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	st, err = e.OperationalStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.BackupOffhostAgeSec < 0 {
		t.Errorf("con marca de backup esperaba antigüedad >= 0, obtuve %d", st.BackupOffhostAgeSec)
	}
}
