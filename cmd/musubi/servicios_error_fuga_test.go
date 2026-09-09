package main

import (
	"runtime"
	"strings"
	"testing"
)

// EL MOTIVO QUE VIAJA AL CEREBRO NO PUEDE LLEVAR EL stderr DEL COMANDO.
//
// POR QUÉ, Y DE DÓNDE SALE LA PREGUNTA. `servicios_error` viaja en el latido, se guarda en la fila
// de la máquina y se lee con `musubi_fleet_list`. Es TEXTO LIBRE que produce la máquina, así que la
// pregunta correcta —que planteó la sesión `musubi-89` el 2026-09-09 al revisar el campo— no es si
// puede suplantar identidad (no puede: la fila la elige el TOKEN, no el cuerpo) sino si puede
// FILTRAR datos: una ruta con el nombre del usuario, un share de red, el volcado de un perfil.
//
// HOY NO PUEDE, Y ES POR CONSTRUCCIÓN, NO POR SUERTE:
//
//   - `ejecutarParaEnumerar` usa `exec.Output()` y NO `CombinedOutput()` (servicios_exec.go), con
//     su motivo escrito: el stderr de `systemctl` trae avisos de units inexistentes que ensucian
//     el parseo.
//   - Go pone el stderr de `Output()` en `ExitError.Stderr`, un campo aparte. El `Error()` de un
//     `*exec.ExitError` devuelve sólo `"exit status N"`, así que un `%w` NO lo arrastra.
//   - Los nombres de comando son literales del repo: `systemctl`, `powershell`, `podman`, `docker`.
//
// Medido en el log de `test-cross (windows-latest)`: el cuerpo del latido salió con
// `"servicios_error":"powershell: exit status 1"`. Nombre y código, nada más.
//
// LO QUE FALTABA ES ESTO: la propiedad se cumple y NADA la sostenía. Cambiar `Output()` por
// `CombinedOutput()` —o meter `ExitError.Stderr` en el mensaje «para que ayude más», que es una
// tentación razonable— abriría la fuga sin que nadie se entere, porque el mensaje seguiría
// viéndose útil. Es la misma forma que veníamos cerrando todo el día: una promesa cierta que
// ninguna guarda custodia.
//
// SE EJERCITA EL COMPORTAMIENTO, no el texto del archivo: se corre un comando REAL que escribe un
// centinela en stderr y sale distinto de cero, y se exige que el centinela no aparezca en el error.
// Un `grep` buscando "CombinedOutput" pasaría igual el día que la fuga entre por el otro camino.
//
// SABOTAJES CORRIDOS, Y UNO SALIÓ FALSO — QUEDA ESCRITO PORQUE ENSEÑA DÓNDE ESTÁ EL RIESGO:
//
//   - ROJO: agregar el `Stderr` del `ExitError` al mensaje de `salidaDeComando`. Ése es el camino
//     real, y es el tentador: el mensaje se ve más útil y nadie mira que ahora cruza la red.
//
//   - VERDE, Y ESTÁ BIEN QUE LO SEA: cambiar `Output()` por `CombinedOutput()`. Yo lo di por
//     rojo al escribir esto y me equivoqué. Con `CombinedOutput` el stderr va a los BYTES DE
//     SALIDA, no al error —`ExitError.Stderr` queda vacío— y `salidaDeComando` descarta esos
//     bytes cuando hay error (`return "", ...`). Así que ese cambio no filtra nada acá.
//
//     Sí rompe otra cosa, y por eso `servicios_exec.go` lo prohíbe con su propio motivo: mezcla
//     el stderr de `systemctl` con el stdout y ensucia el PARSEO. Es un defecto de corrección,
//     no de fuga, y lo custodia otra guarda. Confundir los dos habría hecho que esta prueba
//     reclamara una propiedad que no es la suya — que es cómo nace una guarda que pasa por el
//     motivo equivocado.
func TestElMotivoDeEnumeracionNoArrastraElStderrDelComando(t *testing.T) {
	// El centinela imita lo que de verdad preocupa: una ruta con el nombre de un usuario.
	const centinela = `C:\Users\usuario-privado\perfil`

	var nombre string
	var args []string
	if runtime.GOOS == "windows" {
		nombre, args = "cmd", []string{"/c", "echo " + centinela + " 1>&2 & exit 3"}
	} else {
		nombre, args = "sh", []string{"-c", "echo '" + centinela + "' >&2; exit 3"}
	}
	// LA DIFERENCIA DE SHELL NO ES UNA EXCEPCIÓN DE COBERTURA: el comportamiento que se comprueba
	// es el mismo en las dos ramas y las dos corren. Lo que cambia es cómo se le pide a cada
	// sistema que escriba en stderr, que es una diferencia real del sistema y no del código.

	salida, err := salidaDeComando(nombre, args...)
	if err == nil {
		t.Fatalf("el comando tenía que fallar para poder mirar el error; devolvió %q sin error", salida)
	}

	if strings.Contains(err.Error(), centinela) {
		t.Errorf("el error de enumeración ARRASTRA el stderr del comando, y ese texto viaja al "+
			"cerebro en `servicios_error`, se guarda en la fila de la máquina y se lee con "+
			"`musubi_fleet_list`.\n\n"+
			"error: %v\n\n"+
			"Un stderr puede traer rutas con el nombre del usuario, shares de red o volcados de "+
			"perfil. El campo existe para decir POR QUÉ falló la enumeración —`exit status 1` "+
			"alcanza para eso— y no para mandar la salida cruda. Si hace falta más detalle, va al "+
			"log LOCAL del agente, que no cruza la red.\n"+
			"La propiedad se sostiene con `exec.Output()` (no `CombinedOutput`) y con no meter "+
			"`ExitError.Stderr` en el mensaje.", err)
	}

	// Y LA OTRA MITAD: que el error siga sirviendo. Una guarda que sólo exige «no filtres» se
	// satisface con un mensaje vacío, y ahí el campo dejaría de valer para lo que existe.
	if !strings.Contains(err.Error(), nombre) {
		t.Errorf("el error no nombra el comando que falló (%q): %v — sin eso el motivo no dice "+
			"cuál de las fuentes se rompió, que es la mitad accionable", nombre, err)
	}
}
