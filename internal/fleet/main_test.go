package fleet

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"musubi/internal/testbudget"
)

// TestMain existe SÓLO para el guard del presupuesto de pruebas.
//
// internal/fleet cruzó UMBRAL_GUARDA_LINEAS_TEST con A131 (las guardas de la cronología que
// recorren sus ejes enteros): TestLosPaquetesCarosLlamanAlGuard enumera los paquetes caros por
// sus líneas de test y exige que cada uno pase por acá, igual que internal/mcp, internal/memory
// y cmd/musubi. Sin esto, `go test -race ./...` con un -timeout corto muere con un «panic: test
// timed out» ilegible en vez de decir qué presupuesto falta.
func TestMain(m *testing.M) {
	// flag.Parse() antes de leer -test.timeout: testing.Init() ya registró el flag, pero quien
	// lo PARSEA es m.Run(), y para entonces ya sería tarde para avisar.
	flag.Parse()
	if motivo := testbudget.ExigirTimeoutSuficiente("."); motivo != "" {
		fmt.Fprintln(os.Stderr, "musubi/internal/fleet: presupuesto de pruebas insuficiente —", motivo)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// Y esto exige que lo de arriba HAYA PASADO DE VERDAD en esta corrida: sin él, borrar la llamada
// o el flag.Parse() (con el que el flag vale 0, «sin límite», y el guard queda mudo) deja la
// suite en verde y nadie se entera.
func TestElGuardDelPresupuestoCorrioEnTestMain(t *testing.T) {
	if err := testbudget.ErrorSiElGuardNoCorrio(os.Args); err != nil {
		t.Fatal(err)
	}
}
