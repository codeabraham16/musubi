package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/config"
)

// CheckConfigQueGobierna contesta «¿cuál config.yaml manda acá?» — la pregunta que el sistema no
// sabía contestar y que costó tres hipótesis falsas el 2026-09-05 (cabo A96, salida a).
//
// SIEMPRE dice la ruta absoluta del que gobierna, incluso en `ok`: la mitad del valor de este check
// es responder la pregunta, no sólo avisar de un problema.
//
// Y SE PONE AMARILLO SÓLO SI EL OTRO CAMBIARÍA ALGO. Que exista un `~/.musubi/config.yaml` es
// normal y avisarlo siempre sería gritar lobo —el doctor marca TODA la corrida como `issues` si un
// check no es ok, así que un aviso permanente apagaría el canal entero, que es el defecto que este
// repo persigue—. Se compara el bloque `sync`, que es donde difirieron de verdad: uno lo tenía
// apagado y el otro encendido apuntando al central, y leer el equivocado invirtió la conclusión.
// ConfigQueGobiernaCheckCode es el código del check, para poder pedirlo con `doctor --check`.
const ConfigQueGobiernaCheckCode = "config_que_gobierna"

func CheckConfigQueGobierna(projectPath string) CheckResult {
	const code = ConfigQueGobiernaCheckCode

	ruta := config.ConfigPath(projectPath)
	if abs, err := filepath.Abs(ruta); err == nil {
		ruta = abs
	}
	if _, err := os.Stat(ruta); err != nil {
		// Sin config propia se corre con defaults. No es un error, pero decirlo importa: explica
		// por qué un valor que alguien puso en OTRO archivo no tiene efecto acá.
		return CheckResult{Code: code, Status: "ok",
			Message: fmt.Sprintf("no hay config propia en %s: este proceso corre con los valores por defecto", ruta)}
	}

	sombra := config.ConfigSombra(projectPath)
	if sombra == "" {
		return CheckResult{Code: code, Status: "ok",
			Message: fmt.Sprintf("gobierna %s, y es el único config.yaml en juego", ruta)}
	}

	dif := diferenciasDeSync(projectPath, filepath.Dir(filepath.Dir(sombra)))
	if len(dif) == 0 {
		return CheckResult{Code: code, Status: "ok",
			Message: fmt.Sprintf("gobierna %s; también existe %s pero su bloque sync no difiere", ruta, sombra)}
	}
	return CheckResult{Code: code, Status: "warning",
		Message: fmt.Sprintf(
			"gobierna %s, pero también existe %s y su bloque sync DIFIERE (%s). "+
				"Quien diagnostique abriendo el segundo va a leer lo contrario de lo que hace este proceso: "+
				"el que manda es el del directorio de trabajo",
			ruta, sombra, strings.Join(dif, "; ")),
	}
}

// diferenciasDeSync compara los campos del bloque `sync` que cambian el comportamiento observable.
// El token NO se compara ni se nombra: `auth_token_env` es el NOMBRE de una variable, nunca su valor.
func diferenciasDeSync(dirGobierna, dirSombra string) []string {
	a, err := config.Load(dirGobierna)
	if err != nil {
		return nil
	}
	b, err := config.Load(dirSombra)
	if err != nil {
		return nil
	}
	var dif []string
	if a.Sync.Enabled != b.Sync.Enabled {
		dif = append(dif, fmt.Sprintf("sync.enabled: %v acá vs %v allá", a.Sync.Enabled, b.Sync.Enabled))
	}
	if a.Sync.CentralURL != b.Sync.CentralURL {
		dif = append(dif, fmt.Sprintf("sync.central_url: %q acá vs %q allá", a.Sync.CentralURL, b.Sync.CentralURL))
	}
	if a.Sync.AuthTokenEnv != b.Sync.AuthTokenEnv {
		dif = append(dif, fmt.Sprintf("sync.auth_token_env: %q acá vs %q allá", a.Sync.AuthTokenEnv, b.Sync.AuthTokenEnv))
	}
	return dif
}
