package main

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"musubi/internal/testbudget"
)

// TestMain existe SÓLO para el guard del presupuesto, y existe acá porque la enumeración lo
// nombró.
//
// internal/mcp e internal/memory son los dos paquetes que ya se comían el default de Go (10 min
// POR PAQUETE) y por eso fueron los primeros en llevar el guard. cmd/musubi es el tercero, y
// hasta ahora no lo llevaba porque nadie lo había contado: con 13.092 líneas de código de test y
// 118,2 s medidos bajo -race (2026-09-10, corriendo casi solo; dentro de `./...` se pelea los
// núcleos y tarda más) está en la misma pendiente que los otros dos, un crecimiento antes del
// «panic: test timed out» ilegible.
//
// No lo elegí yo: lo eligió el umbral de presupuesto-de-pruebas.env. El día que otro paquete lo
// pase, TestLosPaquetesCarosLlamanAlGuard lo va a nombrar igual que nombró a éste.
func TestMain(m *testing.M) {
	// flag.Parse() antes de leer -test.timeout: testing.Init() ya registró el flag, pero quien
	// lo PARSEA es m.Run(), y para entonces ya sería tarde para avisar.
	flag.Parse()
	if motivo := testbudget.ExigirTimeoutSuficiente("."); motivo != "" {
		fmt.Fprintln(os.Stderr, "musubi/cmd/musubi: presupuesto de pruebas insuficiente —", motivo)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// Y esto exige que lo de arriba HAYA PASADO DE VERDAD en esta corrida.
//
// El TestMain corre antes de que exista un *testing.T: si alguien le borra la llamada, o le
// borra el flag.Parse() que la hace posible (sin él el flag vale 0, que significa «sin límite»,
// y el guard queda mudo), la suite pasa igual y nadie se entera. Esto lo convierte en rojo.
func TestElGuardDelPresupuestoCorrioEnTestMain(t *testing.T) {
	if err := testbudget.ErrorSiElGuardNoCorrio(os.Args); err != nil {
		t.Fatal(err)
	}
}
