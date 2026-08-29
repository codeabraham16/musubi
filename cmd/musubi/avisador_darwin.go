//go:build darwin

package main

// El avisador en macOS. Ver avisador.go para el porqué.

import (
	"context"
	"os/exec"
	"strings"

	"musubi/internal/fleet"
)

// detectarAvisador en macOS.
//
// NO SE MIRAN DISPLAY NI WAYLAND_DISPLAY: en macOS no existen, y un chequeo copiado de Linux
// diría que ninguna Mac puede avisar. Lo que hay es `osascript`, que está en todo sistema, y una
// sesión de ventanas — que se detecta preguntándole a launchd por la sesión de Aqua.
func detectarAvisador() fleet.CapacidadDeAvisar {
	if _, err := exec.LookPath("osascript"); err != nil {
		return fleet.CapacidadDeAvisar{Motivo: "no hay osascript en el PATH"}
	}
	// `launchctl managername` dice `Aqua` en una sesión gráfica y `Background`/`System` en un
	// demonio. Es lo que separa al agente corriendo en la sesión de alguien del que corre como
	// LaunchDaemon —que no puede dibujar nada aunque la Mac tenga a alguien sentado adelante—.
	salida, err := exec.Command("launchctl", "managername").Output()
	if err != nil {
		return fleet.CapacidadDeAvisar{Motivo: "no se pudo determinar el tipo de sesión " +
			"(launchctl managername falló): " + err.Error()}
	}
	if m := strings.TrimSpace(string(salida)); m != "Aqua" {
		return fleet.CapacidadDeAvisar{Motivo: "el agente corre en una sesión " + m +
			", no en Aqua: no hay escritorio al que dibujarle. Instalar algo no lo arregla"}
	}
	return fleet.CapacidadDeAvisar{Puede: true, Herramienta: "osascript"}
}

// escaparAppleScript tapa las comillas y las barras del texto.
//
// NO ES COSMÉTICO: el texto lo arma el cerebro y se INTERPOLA en un programa que osascript va a
// ejecutar. Una comilla sin escapar cierra la cadena y lo que sigue se ejecuta como AppleScript
// — inyección de código en la máquina de otro, por el camino que existe para pedirle permiso.
func escaparAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

func correrAvisador(ctx context.Context, _ fleet.CapacidadDeAvisar, texto string) error {
	guion := `display notification "` + escaparAppleScript(texto) + `" with title "Musubi"`
	return exec.CommandContext(ctx, "osascript", "-e", guion).Run()
}

func correrPregunta(ctx context.Context, _ fleet.CapacidadDeAvisar, texto string) fleet.RespuestaAviso {
	// `display dialog` devuelve error si el usuario cancela, y también si vence el `giving up`.
	// El plazo lo acota el ctx del llamador; acá no se declara uno propio para no tener dos
	// relojes distintos diciendo cosas distintas sobre el mismo vencimiento.
	guion := `display dialog "` + escaparAppleScript(texto) + `" with title "Musubi" ` +
		`buttons {"No permitir", "Permitir"} default button "No permitir" with icon caution`
	salida, err := exec.CommandContext(ctx, "osascript", "-e", guion).Output()
	if err != nil {
		return fleet.RespuestaNegada
	}
	// EL BOTÓN SE MIRA, no el código de salida. A diferencia de zenity, `display dialog` sale con
	// cero para CUALQUIER botón: quedarse con el exit code concedería acceso también cuando la
	// persona apretó «No permitir».
	if strings.Contains(string(salida), "Permitir") && !strings.Contains(string(salida), "No permitir") {
		return fleet.RespuestaConcedida
	}
	return fleet.RespuestaNegada
}
