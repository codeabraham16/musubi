package memory

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"musubi/internal/testbudget"
)

// TestMain existe SÓLO para el guard del presupuesto, y existe acá por el hermano.
//
// internal/mcp es el paquete más caro bajo `-race` y por eso fue el que se cayó primero; pero
// internal/memory es el segundo, y con 460 s medidos ya usa más de tres cuartos del default de
// Go (10 min POR PAQUETE). Poner la guarda sólo en mcp sería el defecto dominante de este repo
// —una guarda presente en N-1 de N caminos—: el día que memory crezca un 30 % se caería con el
// mismo «panic: test timed out» ilegible, y la lección estaría aprendida de un lado y no del
// hermano.
//
// Los DOS paquetes que pasan la mitad del techo lo miran. Los demás están un orden de magnitud
// más abajo y no lo necesitan; cuando alguno suba, el guard de CI (internal/testbudget, que
// juzga la corrida REAL) es el que lo va a nombrar — este de acá sólo mejora el mensaje.
func TestMain(m *testing.M) {
	// flag.Parse() antes de leer -test.timeout: testing.Init() ya registró el flag, pero quien
	// lo PARSEA es m.Run(), y para entonces ya sería tarde para avisar.
	flag.Parse()
	if motivo := testbudget.ExigirTimeoutSuficiente("."); motivo != "" {
		fmt.Fprintln(os.Stderr, "musubi/internal/memory: presupuesto de pruebas insuficiente —", motivo)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
