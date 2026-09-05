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

// TestOperationalStatsSnapshotLocalAge valida el gauge del snapshot LOCAL, que responde una
// pregunta distinta de la de arriba: «¿se tomó un backup?», no «¿salió de la máquina?».
//
// EL CASO QUE JUSTIFICA QUE SEAN DOS Y NO UNA está en el medio de esta prueba: con la marca
// off-host presente y la local ausente —o al revés— los dos gauges tienen que discrepar. Si
// alguna vez alguien los unifica, esto se pone rojo. En modo local-only (BACKUP_REMOTE vacío,
// que es la decisión vigente) el de off-host vale -1 PARA SIEMPRE, así que sin el local el único
// trabajo programado del servidor no tiene ninguna señal de que siga corriendo.
func TestOperationalStatsSnapshotLocalAge(t *testing.T) {
	e := newTestEngine(t)

	st, err := e.OperationalStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.BackupLocalAgeSec != -1 {
		t.Errorf("sin marca de snapshot esperaba -1, obtuve %d", st.BackupLocalAgeSec)
	}

	backups := filepath.Join(filepath.Dir(e.path), "backups")
	if err := os.MkdirAll(backups, 0755); err != nil {
		t.Fatal(err)
	}

	// Sólo la marca LOCAL: es exactamente el modo local-only de producción.
	if err := os.WriteFile(filepath.Join(backups, snapshotMarkerName), []byte("2026-09-03T03:30:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	st, err = e.OperationalStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.BackupLocalAgeSec < 0 {
		t.Errorf("con marca de snapshot esperaba antigüedad >= 0, obtuve %d", st.BackupLocalAgeSec)
	}
	if st.BackupOffhostAgeSec != -1 {
		t.Errorf("la marca local NO puede mover el gauge de off-host: son dos preguntas distintas (obtuve %d)", st.BackupOffhostAgeSec)
	}

	// Y al revés: la de off-host tampoco puede mover la local.
	if err := os.Remove(filepath.Join(backups, snapshotMarkerName)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backups, offhostMarkerName), []byte("2026-09-03T03:31:00Z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	st, err = e.OperationalStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.BackupOffhostAgeSec < 0 {
		t.Errorf("con marca off-host esperaba antigüedad >= 0, obtuve %d", st.BackupOffhostAgeSec)
	}
	if st.BackupLocalAgeSec != -1 {
		t.Errorf("la marca off-host NO puede mover el gauge local: un backup que salió sin haberse tomado es imposible, y confundirlos deja el timer sin vigilancia (obtuve %d)", st.BackupLocalAgeSec)
	}
}
