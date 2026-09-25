package main

// flota_aviso_test.go — lo que el aviso de máquinas sin latir promete, medido contra las reglas
// que de verdad existen en deploy/musubi-alerts-flota.yml.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestElAvisoDeSinLatirNombraLasAlertasQueDeVerdadSuenan — un dato de la UI tiene que salir de un dato.
//
// El aviso decía «Pasados 5 minutos avisa MaquinaCaida por Telegram», y era cierto sólo a veces.
// «No late» es `musubi_fleet_device_up == 0`, y deploy/musubi-alerts-flota.yml lo parte en DOS reglas
// mutuamente excluyentes según la máquina siga o no en la red: con la máquina viva y el agente
// muerto —gio, tres días «caída» contestando el ping— suena `AgenteCaidoConMaquinaViva`, con otro
// runbook. Y el canal lo decide el Alertmanager de cada uno. Lo encontró la revisión del punto 32:
// ninguna prueba miraba el texto.
//
// Así que el texto se mide contra las reglas: toda alerta que el aviso nombra existe, nombra TODAS
// las que disparan por `musubi_fleet_device_up == 0`, y si promete un plazo, es el `for:` de ellas.
//
// Sabotaje que la hace fallar: nombrar sólo una de las dos.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="—o <code>AgenteCaidoConMaquinaViva</code> si la máquina sigue en la red—"
// arnes: a="—o la otra si la máquina sigue en la red—"
// Sabotaje que la hace fallar: prometer otro plazo que el de las reglas.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="a los 5 minutos avisa"
// arnes: a="a los 2 minutos avisa"
// Sabotaje que la hace fallar: nombrar una alerta que no existe.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="avisa <code>MaquinaCaida</code>"
// arnes: a="avisa <code>MaquinaCaida</code> o <code>MaquinaApagada</code>"
func TestElAvisoDeSinLatirNombraLasAlertasQueDeVerdadSuenan(t *testing.T) {
	pagina := sinComentariosDeHTMLyJS(string(assetsFS(t, "assets/flota.html")))
	i := strings.Index(pagina, "if (d.sin_latir && d.sin_latir.length) {")
	if i < 0 {
		t.Fatal("no encontré el aviso de máquinas sin latir en flota.html: esta prueba no está mirando nada")
	}
	bloque := pagina[i:]
	if j := strings.Index(bloque, "\n  }\n"); j > 0 {
		bloque = bloque[:j]
	}

	ruta := filepath.Join("..", "..", "deploy", "musubi-alerts-flota.yml")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v", ruta, err)
	}
	var doc struct {
		Groups []struct {
			Rules []struct {
				Alert string `yaml:"alert"`
				Expr  string `yaml:"expr"`
				For   string `yaml:"for"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(crudo, &doc); err != nil {
		t.Fatalf("%s no es YAML válido: %v", ruta, err)
	}
	existen := map[string]bool{}
	noLate := map[string]string{} // alerta → su `for:`
	for _, g := range doc.Groups {
		for _, r := range g.Rules {
			if r.Alert == "" {
				continue
			}
			existen[r.Alert] = true
			// LA QUE DISPARA POR «NO LATE» EMPIEZA POR ESO. Las demás lo usan para CALLARSE
			// (`unless … (musubi_fleet_device_up == 0)`), y ésas no avisan de una máquina caída.
			if strings.HasPrefix(strings.Join(strings.Fields(r.Expr), " "), "musubi_fleet_device_up == 0") {
				noLate[r.Alert] = r.For
			}
		}
	}
	if len(noLate) == 0 {
		t.Fatalf("ninguna regla de %s dispara por `musubi_fleet_device_up == 0`: o cambió la forma de "+
			"MaquinaCaida y AgenteCaidoConMaquinaViva, o este detector dejó de verlas", ruta)
	}

	nombradas := map[string]bool{}
	for _, m := range regexp.MustCompile(`<code>([A-Za-z0-9]+)</code>`).FindAllStringSubmatch(bloque, -1) {
		nombradas[m[1]] = true
		if !existen[m[1]] {
			t.Errorf("el aviso de máquinas sin latir nombra <code>%s</code>, y en %s no hay ninguna alerta "+
				"con ese nombre: le promete al que lo lee un aviso que no va a llegar", m[1], ruta)
		}
	}
	for nombre := range noLate {
		if !nombradas[nombre] {
			t.Errorf("%s también dispara cuando una máquina no late, y el aviso no la nombra: el que "+
				"espera la alerta que el panel le dijo recibe otra, con otro runbook", nombre)
		}
	}
	if m := regexp.MustCompile(`(\d+) minutos`).FindStringSubmatch(bloque); m != nil {
		for nombre, plazo := range noLate {
			if plazo != m[1]+"m" {
				t.Errorf("el aviso promete que avisan a los %s minutos, y %s tiene `for: %s`", m[1], nombre, plazo)
			}
		}
	}
}
