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
// QUIÉNES LO MIRAN NO SE ESCRIBE ACÁ. Esa frase —«los dos paquetes caros»— era todo lo que
// sostenía al hermano, y sacarle esta llamada a cualquiera de los dos dejaba todo en verde.
// Ahora el conjunto lo ENUMERA TestLosPaquetesCarosLlamanAlGuard, del código de test que tiene
// cada paquete contra UMBRAL_GUARDA_LINEAS_TEST; hoy da internal/mcp, internal/memory y
// cmd/musubi, y el que crezca mañana lo nombra sola.
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
