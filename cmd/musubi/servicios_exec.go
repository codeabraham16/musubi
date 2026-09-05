package main

// ejecutarParaEnumerar es la ÚNICA puerta por la que los enumeradores hablan con el sistema.
//
// Está aparte y es `var` por dos motivos. Uno: las pruebas la apuntan a un doble y ejercitan el
// PARSEO —que es donde están los errores de verdad— sin necesitar systemd, ni el SCM de Windows,
// ni launchd. Dos: acá vive el timeout, en un solo lugar. Un `systemctl` colgado no puede colgar
// el latido, porque una máquina que deja de latir por no poder listar sus units figura muerta por
// un motivo que no tiene nada que ver con estar muerta.

import (
	"context"
	"os/exec"
	"time"
)

// tiempoDeEnumeracion acota lo que puede tardar UNA fuente. Es corto a propósito: el intervalo de
// latido más chico de la flota son 10 s, y el inventario no puede comerse el latido.
const tiempoDeEnumeracion = 5 * time.Second

var ejecutarParaEnumerar = func(nombre string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tiempoDeEnumeracion)
	defer cancel()
	// Output y no CombinedOutput: el stderr de systemctl trae avisos de units que no existen, y
	// mezclarlo con el stdout ensucia el parseo con texto que no tiene la forma esperada.
	return exec.CommandContext(ctx, nombre, args...).Output()
}
