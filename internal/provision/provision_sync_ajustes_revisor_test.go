package provision

import (
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// Un bloque `sync:` deshabilitado pero CON ajustes del usuario (tls_server_name para discar la IP del
// tailnet, flota_vivo: false como opt-out de telemetría, un batch_size a mano) no es el default del
// init: reemplazarlo entero los borra, y validarEdicion no lo ve porque copia TODO Sync del editado
// al original antes de comparar.
func TestProvisionNoBorraAjustesDelSyncDeshabilitado(t *testing.T) {
	orig := `version: "1.0"
sync:
  enabled: false
  central_url: https://100.79.126.62:10000
  tls_server_name: musubi-server.tail89e295.ts.net
  flota_vivo: false
  batch_size: 7
memory:
  recall_token_budget: 400
`
	dir := escribirCfg(t, orig)
	r := ensureSyncConfig(dir, "https://100.79.126.62:10000", "MUSUBI_TOKEN", false)
	got, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if r.Status != StatusDone {
		if string(got) != orig {
			t.Fatalf("al negarse no se escribe; estado %q y quedó:\n%s", r.Status, got)
		}
		return // negarse con la instrucción manual también es aceptable
	}
	cfg, err := config.Parse(got)
	if err != nil {
		t.Fatalf("no parsea: %v\n%s", err, got)
	}
	if cfg.Sync.TLSServerName != "musubi-server.tail89e295.ts.net" {
		t.Errorf("se borró tls_server_name (%q): con central_url por IP el handshake TLS falla y el outbox queda pending para siempre:\n%s", cfg.Sync.TLSServerName, got)
	}
	if cfg.Sync.FlotaVivoActivo() {
		t.Errorf("se borró flota_vivo: false: la telemetría quedó ENCENDIDA sin que nadie lo pidiera:\n%s", got)
	}
	if cfg.Sync.BatchSize != 7 {
		t.Errorf("se borró batch_size: %d (quería 7)", cfg.Sync.BatchSize)
	}
}
