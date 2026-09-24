package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// escribirCfg deja un config.yaml y devuelve el directorio del proyecto.
func escribirCfg(t *testing.T, contenido string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".musubi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if contenido != "" {
		if err := os.WriteFile(filepath.Join(dir, ".musubi", "config.yaml"), []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func cargar(t *testing.T, dir string) config.Config {
	t.Helper()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("el config que quedó no parsea: %v", err)
	}
	return cfg
}

// EL DEFECTO QUE ESTO ARREGLA (medido 2026-09-23): `provision` escribía sólo el bloque `sync:`, así
// que una máquina recién dada de alta SUBÍA Y NO BAJABA — RunInboundScheduler se apaga solo sin
// team_mode y el pull nunca arranca. Es el motivo por el que «un empleado nuevo hereda el cerebro»
// no funcionaba: no faltaba una función, faltaba una clave que el alta jamás escribía.
func TestProvisionEnciendeLaBajada(t *testing.T) {
	dir := escribirCfg(t, "version: \"1.0\"\nmode: local\n")
	if r := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false); r.Status != StatusDone {
		t.Fatalf("estado %q: %s", r.Status, r.Detail)
	}
	cfg := cargar(t, dir)
	if !cfg.Sync.Enabled {
		t.Error("la subida tiene que quedar encendida")
	}
	if !cfg.Memory.TeamMode {
		t.Error("la BAJADA tiene que quedar encendida: sin team_mode la máquina sube y no baja")
	}
}

// El bloque `memory:` NO se puede reemplazar entero como el de `sync:`: ahí viven otras claves del
// proyecto. Pisarlas sería borrar configuración que nadie pidió tocar.
func TestProvisionNoPisaLasOtrasClavesDeMemory(t *testing.T) {
	dir := escribirCfg(t, `version: "1.0"
memory:
  # un comentario que el usuario escribió
  recall_token_budget: 777
  team_mode: false
  brevity_mode: "lite"
`)
	if r := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false); r.Status != StatusDone {
		t.Fatalf("estado %q: %s", r.Status, r.Detail)
	}
	cfg := cargar(t, dir)
	if !cfg.Memory.TeamMode {
		t.Error("team_mode tiene que quedar en true")
	}
	if cfg.Memory.RecallTokenBudget != 777 {
		t.Errorf("se perdió recall_token_budget: %d (quería 777)", cfg.Memory.RecallTokenBudget)
	}
	if cfg.Memory.BrevityMode != "lite" {
		t.Errorf("se perdió brevity_mode: %q (quería \"lite\")", cfg.Memory.BrevityMode)
	}
	crudo, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if !strings.Contains(string(crudo), "un comentario que el usuario escribió") {
		t.Error("se perdió el comentario del usuario dentro del bloque memory:")
	}
	if strings.Count(string(crudo), "\nmemory:")+strings.Count(string(crudo), "memory:\n") > 2 {
		t.Errorf("clave memory: duplicada — el YAML dejaría de parsear:\n%s", crudo)
	}
}

// Una máquina que YA tenía sync saliente y la bajada apagada: se INFORMA, no se toca. La primera
// versión lo reparaba sola, y la revisión adversarial mostró dos costos: este camino —que antes jamás
// escribía el archivo— pasaba a reescribirlo (y con el editor por regex, a romperlo), y un
// team_mode: false en una máquina ya configurada puede ser una decisión —una consola de alcance
// acotado— que darlo vuelta convierte en «todo lo que se guarde sube al central».
func TestProvisionInformaElSuboYNoBajoSinTocarElArchivo(t *testing.T) {
	cfg := `version: "1.0"
sync:
  enabled: true
  central_url: https://cerebro:10000
  auth_token_env: MUSUBI_TOKEN
memory:
  team_mode: false
`
	dir := escribirCfg(t, cfg)
	r := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false)
	if r.Status != StatusTodo {
		t.Fatalf("tenía que informarlo como pendiente; estado %q: %s", r.Status, r.Detail)
	}
	if !strings.Contains(r.Detail, "NO BAJA") || !strings.Contains(r.Detail, "suba al central") {
		t.Errorf("el aviso tiene que decir que no baja Y lo que cuesta encenderlo: %s", r.Detail)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml")); string(got) != cfg {
		t.Fatalf("en una máquina ya configurada el archivo NO se toca:\n%s", got)
	}
}

// Y con las dos mitades ya puestas no toca nada: idempotente.
func TestProvisionNoTocaLoQueYaEstaCompleto(t *testing.T) {
	dir := escribirCfg(t, `version: "1.0"
sync:
  enabled: true
  central_url: https://cerebro:10000
memory:
  team_mode: true
`)
	antes, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	r := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false)
	if r.Status != StatusOK {
		t.Fatalf("estado %q: %s", r.Status, r.Detail)
	}
	despues, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if string(antes) != string(despues) {
		t.Errorf("no tenía que tocar el archivo:\n--- antes ---\n%s\n--- después ---\n%s", antes, despues)
	}
}

// LA SANGRÍA NO SE PUEDE FIJAR, Y ESTO LO ENCONTRÓ UNA PRUEBA EXISTENTE: `config.Default().Marshal()`
// sale de yaml.Marshal, que indenta con CUATRO espacios, mientras que un config escrito a mano usa
// dos. La primera versión insertaba `  team_mode: true` fijo y dejaba el config del proyecto sin
// parsear —para el parser es un dedent a mitad del mapa—, o sea que el paso reportaba ✓ y rompía la
// configuración entera. La sangría se deduce de los hermanos del bloque.
func TestProvisionRespetaLaSangriaDelBloque(t *testing.T) {
	for _, caso := range []struct{ nombre, sangria string }{
		{"dos espacios", "  "},
		{"cuatro espacios", "    "},
		// Sin caso de tabulador a proposito: YAML PROHIBE el tabulador en la sangria, asi que
		// un config indentado con tabs ya no parsea ANTES de que provision lo toque. Probarlo
		// mediria el parser, no este arreglo.
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			s := caso.sangria
			dir := escribirCfg(t, "version: \"1.0\"\nmemory:\n"+s+"recall_token_budget: 400\n"+s+"gist_max_tokens: 24\n")
			if r := ensureSyncConfig(dir, "https://cerebro:10000", "MUSUBI_TOKEN", false); r.Status != StatusDone {
				t.Fatalf("estado %q: %s", r.Status, r.Detail)
			}
			cfg := cargar(t, dir) // si la sangría quedó mal, esto falla al parsear
			if !cfg.Memory.TeamMode {
				t.Error("team_mode tiene que quedar en true")
			}
			if cfg.Memory.RecallTokenBudget != 400 || cfg.Memory.GistMaxTokens != 24 {
				t.Errorf("se perdieron claves hermanas: budget=%d gist=%d", cfg.Memory.RecallTokenBudget, cfg.Memory.GistMaxTokens)
			}
		})
	}
}
