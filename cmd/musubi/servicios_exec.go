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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// tiempoDeEnumeracion acota lo que puede tardar UNA fuente. Es corto a propósito: el intervalo de
// latido más chico de la flota son 10 s, y el inventario no puede comerse el latido.
//
// Es `var` y no `const` para que la guarda del timeout pueda clavarlo en milisegundos: probarlo
// con los 5 s de verdad costaría 5 s de reloj por caso, y una prueba lenta es una prueba que
// alguien termina salteando.
var tiempoDeEnumeracion = 5 * time.Second

// ────────────────────────────────────────────────────────────────────────────────────────────
// «SE PASÓ DEL PRESUPUESTO» Y «EL COMANDO FALLÓ» CONTESTABAN LO MISMO
//
// En Windows, matar un proceso es `TerminateProcess(h, 1)`, así que el que mata el contexto sale
// con código 1 — el MISMO que un comando que falló de verdad. Medido el 2026-09-20 con una réplica
// exacta de esta función:
//
//	matado por el timeout     -> exit status 1
//	fallo real del comando    -> exit status 1
//
// Idénticos. Y eso viaja: `salidaDeComando` lo envuelve como `powershell: exit status 1`, el
// agente lo manda en `servicios_error` y el cerebro se lo muestra a quien mire la flota. En
// `davantis-1` apareció justamente así el 2026-09-20, con la máquina cargada, y el mensaje mandaba
// a buscar un PowerShell roto que estaba perfecto: `Get-CimInstance Win32_Service` tarda ~2,1 s en
// reposo y ~3,5 s bajo carga, o sea que el presupuesto se cruza por ocupación, no por avería.
//
// Son dos causas con dos arreglos opuestos —bajar la carga o la cadencia, contra reparar la
// herramienta—, así que la distinción no es cosmética: es a dónde va a mirar la persona.
//
// SE PREGUNTA POR `ctx.Err()` Y NO POR EL CÓDIGO DE SALIDA, porque el código de salida es
// exactamente lo que no alcanza para distinguirlos.
var ejecutarParaEnumerar = func(nombre string, args ...string) ([]byte, error) {
	return correrConPresupuesto(nil, nombre, args...)
}

// ejecutarComoParaEnumerar corre el comando con la identidad del dueño (identidad_servicios.go).
//
// SI NO HAY QUE BAJAR, DELEGA EN ejecutarParaEnumerar, y no es por prolijidad: las pruebas de
// parseo apuntan esa `var` a un doble, y un camino que la saltara dejaría de pasar por sus dobles
// —el agente de todas las máquinas menos una iría por acá con la identidad en cero—.
//
// Si hay que bajar, es el MISMO cuerpo con el mismo presupuesto: sólo cambian el entorno y la
// credencial del hijo. El exec y la shell de las personas NO pasan por acá, y siguen como root.
var ejecutarComoParaEnumerar = func(id identidadDeServicios, nombre string, args ...string) ([]byte, error) {
	if !id.Bajar {
		return ejecutarParaEnumerar(nombre, args...)
	}
	return correrConPresupuesto(func(cmd *exec.Cmd) {
		cmd.Env = entornoPara(id, os.Environ(), esDeUid)
		cmd.SysProcAttr = atributosPara(id)
	}, nombre, args...)
}

// correrConPresupuesto es el único lugar que lanza un enumerador: el timeout y la distinción de
// abajo valen igual para el hijo que hereda al agente y para el que baja al dueño.
func correrConPresupuesto(preparar func(*exec.Cmd), nombre string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tiempoDeEnumeracion)
	defer cancel()
	cmd := exec.CommandContext(ctx, nombre, args...)
	if preparar != nil {
		preparar(cmd)
	}
	// Output y no CombinedOutput: el stderr de systemctl trae avisos de units que no existen, y
	// mezclarlo con el stdout ensucia el parseo con texto que no tiene la forma esperada.
	b, err := cmd.Output()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("no terminó en %s y hubo que matarlo: %w", tiempoDeEnumeracion, ctx.Err())
	}
	return b, err
}
