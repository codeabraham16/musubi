package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// El bloque `sync:` se reemplaza CONSERVANDO lo que el alta no escribe: el alta sale «hecho» y los
// ajustes del usuario siguen ahí, con la sangría y el fin de línea del archivo (acá, cuatro espacios
// y CRLF, como lo deja un editor de Windows).
//
// Sabotaje que la pone roja: tirar todos los hijos del bloque viejo.
// arnes: archivo="internal/provision/syncconfig.go"
// arnes: de="\t\tif bytes.Equal(ind, sangria) && esClaveDelAlta(t) {"
// arnes: a="\t\tif bytes.Equal(ind, sangria) || esClaveDelAlta(t) || true {"
func TestProvisionConservaLosAjustesDelSyncViejo(t *testing.T) {
	orig := strings.ReplaceAll(`version: "1.0"
sync:
    enabled: false
    central_url: https://100.79.126.62:10000
    # para discar la IP del tailnet
    tls_server_name: musubi-server.tail89e295.ts.net
    flota_vivo: false
    batch_size: 7
memory:
    recall_token_budget: 400
`, "\n", "\r\n")
	dir := escribirCfg(t, orig)
	r := ensureSyncConfig(dir, "https://100.79.126.62:10000", "MUSUBI_TOKEN", false)
	got, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if r.Status != StatusDone {
		t.Fatalf("el alta tenía que hacerse conservando los ajustes; estado %q: %s\n%s", r.Status, r.Detail, got)
	}
	cfg, err := config.Parse(got)
	if err != nil {
		t.Fatalf("no parsea: %v\n%s", err, got)
	}
	if !cfg.Sync.Enabled || cfg.Sync.CentralURL != "https://100.79.126.62:10000" || !cfg.Memory.TeamMode {
		t.Errorf("el alta no quedó hecha: %+v / team_mode=%v\n%s", cfg.Sync, cfg.Memory.TeamMode, got)
	}
	if cfg.Sync.TLSServerName != "musubi-server.tail89e295.ts.net" || cfg.Sync.FlotaVivoActivo() || cfg.Sync.BatchSize != 7 {
		t.Errorf("se perdieron ajustes del usuario (tls=%q flota_vivo=%v batch=%d):\n%s",
			cfg.Sync.TLSServerName, cfg.Sync.FlotaVivoActivo(), cfg.Sync.BatchSize, got)
	}
	if !strings.Contains(string(got), "# para discar la IP del tailnet") {
		t.Errorf("se perdió el comentario del usuario dentro del bloque:\n%s", got)
	}
	if strings.Count(string(got), "enabled:") != 1 {
		t.Errorf("la clave enabled quedó duplicada:\n%s", got)
	}
	if strings.Contains(strings.ReplaceAll(string(got), "\r\n", ""), "\n") {
		t.Errorf("el archivo era CRLF y quedaron líneas con LF suelto:\n%q", got)
	}
}
