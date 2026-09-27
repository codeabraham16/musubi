package config

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// claveDelTopeDeLaBajada es el nombre PUBLICADO del tope del espaciado de la bajada: sale de
// config.example.yaml, que es lo que el operador tipea, y no de la etiqueta del campo, que es justo
// lo que el sabotaje mueve (ver fleet_techo_yaml_test.go, que explica por qué una prueba con un
// literal de Go no alcanza).
const claveDelTopeDeLaBajada = "inbound_idle_max_seconds"

// TestElTopeDeLaBajadaLlegaDesdeElYAML: el tope que se escribe en `.musubi/config.yaml` llega al
// campo por el parser, y sin la clave —todos los configs que existen— el tope efectivo es 300 s, no
// «sin tope» ni «ritmo fijo». El negativo es la salida para quien quiera la conducta de antes, y
// tiene que llegar tal cual. Y la clave está documentada donde se la va a buscar.
//
// Sabotaje: la etiqueta yaml cambia de nombre y la clave del archivo cae al default en silencio.
// arnes: archivo="internal/config/config.go"
// arnes: de="InboundIdleMaxSeconds int `yaml:\"inbound_idle_max_seconds,omitempty\"`"
// arnes: a="InboundIdleMaxSeconds int `yaml:\"ritmo_saboteado,omitempty\"`"
//
// Sabotaje: la clave desaparece de config.example.yaml.
// arnes: archivo=".musubi/config.example.yaml"
// arnes: de="#   inbound_idle_max_seconds: 300"
// arnes: a="#   inbound_idle_max_saboteado: 300"
func TestElTopeDeLaBajadaLlegaDesdeElYAML(t *testing.T) {
	for _, c := range []struct {
		nombre, yaml string
		esperar      int
	}{
		{"negativo ⇒ ritmo fijo", "sync:\n  " + claveDelTopeDeLaBajada + ": -1\n", -1},
		{"un número", "sync:\n  " + claveDelTopeDeLaBajada + ": 120\n", 120},
		{"la sección sin la clave ⇒ 300", "sync:\n  drain_interval_seconds: 30\n", 300},
		{"sin sección ⇒ 300", "memory:\n  team_mode: true\n", 300},
	} {
		var cfg Config
		if err := yaml.Unmarshal([]byte(c.yaml), &cfg); err != nil {
			t.Fatalf("%s: el YAML no parsea: %v", c.nombre, err)
		}
		if got := cfg.Sync.EffectiveInboundIdleMaxSeconds(); got != c.esperar {
			t.Errorf("%s: se escribió\n%s\ny el tope efectivo de la bajada quedó en %d, esperaba %d", c.nombre, c.yaml, got, c.esperar)
		}
	}

	b, err := os.ReadFile("../../.musubi/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), claveDelTopeDeLaBajada+":") {
		t.Errorf("config.example.yaml no documenta %q: el tope existe en el binario y no donde alguien lo va a buscar", claveDelTopeDeLaBajada)
	}
}
