package mcp

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// EL VERIFICADOR CORTA SI python3 NO CORRE, NO SÓLO SI NO ESTÁ.
//
// La guarda vieja preguntaba `command -v python3`, y en Windows eso lo contesta el alias de la
// Microsoft Store (`WindowsApps/python3`): se encuentra, y al correrlo imprime cómo instalar Python
// y sale con 49. Medido el 2026-10-01 en davantis-1: la guarda pasaba, cada lectura de JSON volvía
// vacía y el informe salía con ✘ contra un servidor sano.
//
// Se mide CORRIENDO el guion con un `python3` de mentira adelante en el PATH, que hace lo mismo que
// el alias. Y en las dos direcciones: una guarda que cortara siempre pasaría la primera mitad.
//
// Sabotaje que la pone roja: volver a la guarda que sólo lo busca.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de=`if ! python3 -c "import json" >/dev/null 2>&1; then`
// arnes: a=`if ! command -v python3 >/dev/null 2>&1; then`
func TestElVerificadorCortaSiPython3NoCorre(t *testing.T) {
	compuerta := guiones.Unix(t, "corre deploy/verificar-despliegue.sh con un python3 de mentira adelante en el "+
		"PATH; se mide también en macOS porque la guarda vive en el encabezado, que corre en los dos",
		"bash", "git", "python3")

	const aviso = "python3 no corre en esta máquina"

	// La primera sección se DERIVA del guion, como la última en `ultimaSeccionQueDeclaraElGuion`:
	// si cortó en la puerta, el informe no puede haber abierto ninguna.
	titulos := regexp.MustCompile(`titulo "([^"]+)"`).FindStringSubmatch(leerDeploy(t, "verificar-despliegue.sh"))
	if titulos == nil {
		t.Fatal("no se encontró ningún `titulo \"…\"` en verificar-despliegue.sh: cambió la forma de " +
			"declarar una sección y esta prueba no sabría qué buscar para decir «cortó en la puerta»")
	}
	primera := titulos[1]

	correr := func(t *testing.T, path string) (string, error) {
		t.Helper()
		raiz := prepararRepoDePrueba(t)
		cmd := compuerta.Comando("bash", filepath.Join(raiz, "deploy", "verificar-despliegue.sh"))
		cmd.Dir = raiz
		cmd.Env = append(os.Environ(),
			"PATH="+path,
			"MUSUBI_SIN_FETCH=1",
			"PROM_URL=http://127.0.0.1:1",
			"ALERT_URL=http://127.0.0.1:1",
			"MUSUBI_SSH=",
		)
		salida, err := cmd.CombinedOutput()
		return sinColores(string(salida)), err
	}

	t.Run("un python3 que se encuentra y no ejecuta corta en la puerta con 2", func(t *testing.T) {
		stubs := t.TempDir()
		escribirStub(t, stubs, "python3",
			"echo 'Python was not found; run without arguments to install from the Microsoft Store' >&2\nexit 49\n")

		salida, err := correr(t, stubs+string(os.PathListSeparator)+os.Getenv("PATH"))

		if !strings.Contains(salida, aviso) {
			t.Errorf("con un python3 que se encuentra pero no ejecuta, el verificador no dijo %q.\n"+
				"  Es el alias de la Microsoft Store: la guarda que sólo lo BUSCA lo da por bueno, y cada\n"+
				"  lectura de JSON vuelve vacía.\n  Salida:\n%s", aviso, salida)
		}
		if strings.Contains(salida, primera) {
			t.Errorf("el verificador avisó y SIGUIÓ: abrió la sección %q con un python3 que no corre.\n"+
				"  Lo que viene después se lee con un intérprete que no existe.\n  Salida:\n%s", primera, salida)
		}
		var fin *exec.ExitError
		if !errors.As(err, &fin) || fin.ExitCode() != 2 {
			t.Errorf("tiene que salir con 2 (SIN VERIFICAR) y salió con %v: «no pude leer» no es «está bien»\n"+
				"  ni «difiere».\n  Salida:\n%s", err, salida)
		}
	})

	t.Run("con el python3 de verdad no corta", func(t *testing.T) {
		salida, _ := correr(t, os.Getenv("PATH"))
		if strings.Contains(salida, aviso) {
			t.Errorf("con un python3 que corre, el verificador cortó igual: la guarda corta siempre.\n"+
				"  Salida:\n%s", salida)
		}
		if !strings.Contains(salida, primera) {
			t.Errorf("con un python3 que corre, el informe no abrió la primera sección (%q).\n  Salida:\n%s",
				primera, salida)
		}
	})
}
