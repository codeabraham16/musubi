package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/memory"
)

func escribirConfig(t *testing.T, dir, cuerpo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".musubi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".musubi", "config.yaml"), []byte(cuerpo), 0o644); err != nil {
		t.Fatal(err)
	}
}

// EL CASO DEL CABO A96, TAL CUAL PASÓ: dos config.yaml, el del home con el sync apagado y el del
// proyecto con el sync encendido. Leer el del home invirtió la conclusión del diagnóstico.
//
// Sabotaje que la hace fallar: comparar sólo la existencia de los dos archivos y no su contenido.
func TestElDoctorSePoneAmarilloSiElOtroConfigDiceLoContrario(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	escribirConfig(t, home, "sync:\n  enabled: false\n")

	proyecto := t.TempDir()
	escribirConfig(t, proyecto, "sync:\n  enabled: true\n  central_url: http://127.0.0.1:7717\n  auth_token_env: MUSUBI_TOKEN\n")

	c := memory.CheckConfigQueGobierna(proyecto)

	if c.Status != "warning" {
		t.Fatalf("con los dos configs diciendo cosas distintas tiene que avisar; dio %q: %s", c.Status, c.Message)
	}
	if !strings.Contains(c.Message, "sync.enabled") {
		t.Fatalf("el aviso tiene que DECIR en qué difieren, no sólo que difieren: %s", c.Message)
	}
	if !strings.Contains(c.Message, proyecto) {
		t.Fatalf("tiene que nombrar la ruta del que gobierna: %s", c.Message)
	}
}

// Que existan dos configs es NORMAL. Si no difieren en nada que importe, el check queda en ok — si
// no, el doctor marcaría toda la corrida como `issues` para siempre y se apagaría el canal, que es
// el defecto que este repo persigue.
func TestElDoctorNoGritaLoboSiLosDosConfigsCoinciden(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	escribirConfig(t, home, "sync:\n  enabled: false\n")

	proyecto := t.TempDir()
	escribirConfig(t, proyecto, "sync:\n  enabled: false\n")

	c := memory.CheckConfigQueGobierna(proyecto)
	if c.Status != "ok" {
		t.Fatalf("sin diferencias no hay nada que avisar; dio %q: %s", c.Status, c.Message)
	}
}

// La otra mitad del valor: incluso en verde tiene que CONTESTAR la pregunta.
func TestElDoctorSiempreDiceCualConfigGobierna(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	proyecto := t.TempDir()
	escribirConfig(t, proyecto, "sync:\n  enabled: true\n")

	c := memory.CheckConfigQueGobierna(proyecto)
	if c.Status != "ok" {
		t.Fatalf("sin sombra no hay aviso; dio %q", c.Status)
	}
	if !strings.Contains(c.Message, filepath.Join(proyecto, ".musubi", "config.yaml")) {
		t.Fatalf("aun en ok tiene que decir la ruta absoluta del que manda: %s", c.Message)
	}
}

func TestElDoctorDiceCuandoSeCorreConDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c := memory.CheckConfigQueGobierna(t.TempDir()) // sin .musubi/config.yaml
	if c.Status != "ok" || !strings.Contains(c.Message, "valores por defecto") {
		t.Fatalf("tiene que explicar que corre con defaults: %q / %s", c.Status, c.Message)
	}
}
