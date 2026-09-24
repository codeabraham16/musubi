package provision

import "testing"

// El dry-run promete «habilitaría el sync en los DOS sentidos» sin probar la edición, y la corrida
// real se niega con los YAML que asegurarTeamMode o validarEdicion rechazan.
func TestProvisionDryRunNoPrometeLoQueLaCorridaRealRechaza(t *testing.T) {
	for _, contenido := range []string{
		`version: "1.0"
memory: {}
`,
		`version: "1.0"
memory:
  "team_mode": false
`,
	} {
		dir := escribirCfg(t, contenido)
		seco := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", true)
		deVerdad := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false)
		if deVerdad.Status == StatusError && seco.Status != StatusError {
			t.Errorf("%q: el dry-run dice %q (%s) y la corrida real se niega (%s)", contenido, seco.Status, seco.Detail, deVerdad.Detail)
		}
	}
}
