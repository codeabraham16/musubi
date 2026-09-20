package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// servicios_timeout_test.go — que «se pasó del presupuesto» no se lea como «el comando falló».
//
// EL DEFECTO, MEDIDO EL 2026-09-20 EN `davantis-1`.
//
// La flota mostraba `servicios_error: "powershell: exit status 1"`, que se lee como una herramienta
// rota. PowerShell estaba perfecto: corriendo a mano, el mismo `Get-CimInstance Win32_Service`
// devolvía 300 servicios con exit 0 en ~2,1 s. Lo que pasaba es que la máquina estaba cargada y la
// enumeración se comía los 5 s de `tiempoDeEnumeracion` — bajo carga moderada ya tardaba ~3,5 s.
//
// Y el mensaje NO PODÍA decirlo. En Windows, matar un proceso es `TerminateProcess(h, 1)`, así que
// el proceso que mata el contexto sale con el MISMO código que uno que falló de verdad:
//
//	matado por el timeout   -> exit status 1
//	fallo real del comando  -> exit status 1
//
// Las dos causas tienen arreglos OPUESTOS —bajar la carga o la cadencia, contra reparar la
// herramienta—, así que un mensaje que las mezcla manda a la persona al lugar equivocado. Es la
// misma forma que «no vino nadie» y «vino y no imprime» contestando igual.
//
// LA ASERCIÓN QUE IMPORTA ES QUE LOS DOS MENSAJES DIFIERAN. Comprobar sólo que el del timeout
// nombre el presupuesto dejaría pasar una versión que se lo agregue a los dos.
//
// Sabotaje que la hace fallar: preguntar por `context.Canceled` en vez de `DeadlineExceeded`, que
// nunca se cumple porque el único cancel es el `defer`.
// arnes: archivo="cmd/musubi/servicios_exec.go"
// arnes: de="errors.Is(ctx.Err(), context.DeadlineExceeded)"
// arnes: a="errors.Is(ctx.Err(), context.Canceled)"
func TestElTimeoutDeEnumeracionNoSeConfundeConUnFalloDelComando(t *testing.T) {
	original := tiempoDeEnumeracion
	t.Cleanup(func() { tiempoDeEnumeracion = original })
	const presupuestoCorto = 300 * time.Millisecond
	tiempoDeEnumeracion = presupuestoCorto

	// (A) el ayudante no termina dentro del presupuesto: lo mata el contexto.
	t.Setenv("GO_AYUDANTE_SERVICIOS", "colgarse")
	_, errTimeout := ejecutarParaEnumerar(os.Args[0], "-test.run=^TestAyudanteDeEnumeracion$")
	if errTimeout == nil {
		t.Fatal("el ayudante duraba más que el presupuesto y aun así no hubo error:\n" +
			"o el timeout dejó de aplicarse, o el ayudante dejó de colgarse")
	}

	// (B) el ayudante falla rápido y de verdad.
	//
	// EL PRESUPUESTO DE ACÁ ES HOLGADO A PROPÓSITO, y la primera versión de esta prueba no lo era:
	// reusaba los 300 ms de (A) y el caso «fallo real» TAMBIÉN moría por timeout, porque arrancar el
	// binario de prueba como subproceso ya cuesta cientos de ms en Windows. O sea que la prueba medía
	// el MISMO desenlace dos veces y habría pasado con el arreglo puesto y sin poner. Con 30 s, lo
	// único que puede hacer fallar a (B) es el comando.
	tiempoDeEnumeracion = 30 * time.Second
	t.Setenv("GO_AYUDANTE_SERVICIOS", "fallar")
	_, errFallo := ejecutarParaEnumerar(os.Args[0], "-test.run=^TestAyudanteDeEnumeracion$")
	if errFallo == nil {
		t.Fatal("el ayudante salió con código 3 y no se reportó error")
	}
	var salida *exec.ExitError
	if !errors.As(errFallo, &salida) {
		t.Fatalf("el fallo real tendría que llegar como *exec.ExitError y llegó como %T: %v", errFallo, errFallo)
	}

	// LO QUE SE CUSTODIA: que no se puedan confundir.
	if errTimeout.Error() == errFallo.Error() {
		t.Fatalf("el timeout y el fallo real contestan LO MISMO (%q).\n"+
			"En Windows los dos salen con código 1, así que quien lea `servicios_error` no puede\n"+
			"saber si la máquina estaba ocupada o si la herramienta está rota — y el arreglo de\n"+
			"cada caso es el contrario del otro", errTimeout.Error())
	}
	if !errors.Is(errTimeout, context.DeadlineExceeded) {
		t.Errorf("el error del timeout no envuelve context.DeadlineExceeded: %v", errTimeout)
	}
	if errors.Is(errFallo, context.DeadlineExceeded) {
		t.Errorf("un fallo real se está reportando como vencimiento del presupuesto: %v", errFallo)
	}
	// Y que el mensaje diga CUÁNTO era el presupuesto: sin el número no se sabe si subirlo.
	if !strings.Contains(errTimeout.Error(), presupuestoCorto.String()) {
		t.Errorf("el mensaje del timeout no nombra el presupuesto (%s): %v",
			presupuestoCorto, errTimeout)
	}
}

// TestAyudanteDeEnumeracion no es un test: es el proceso hijo que se cuelga o falla a pedido.
func TestAyudanteDeEnumeracion(t *testing.T) {
	switch os.Getenv("GO_AYUDANTE_SERVICIOS") {
	case "colgarse":
		// Holgado contra el presupuesto de la prueba (300 ms) pero corto en absoluto: si alguna vez
		// el contexto dejara de matarlo, la prueba falla en segundos en vez de colgar la suite.
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "fallar":
		// SALE CON 1 Y NO CON OTRO CÓDIGO, y es lo que hace que esta prueba sirva. En Windows el
		// proceso que mata el contexto sale con 1, así que un ayudante que fallara con 3 daría dos
		// mensajes distintos POR EL CÓDIGO y la aserción de «contestan lo mismo» pasaría aunque el
		// arreglo no estuviera. Con 1 la prueba reproduce el caso real —PowerShell falla con 1— y
		// la única cosa capaz de distinguirlos es preguntar por `ctx.Err()`.
		os.Exit(1)
	default:
		t.Skip("ayudante, no es un test")
	}
}
