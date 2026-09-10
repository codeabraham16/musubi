package mcp

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"musubi/internal/testbudget"
)

// TestMain fija el entorno del paquete antes de correr un solo test.
//
// 🔴 POR QUÉ ESTO ESTÁ ACÁ. `MUSUBI_TOOLS_ALL=1` es un interruptor DOCUMENTADO —devuelve al
// catálogo las nueve tools dormidas sin recompilar (ver registry.go)— y media docena de tests de
// este paquete afirman sobre la FORMA de ese catálogo: cuántas tools lista, el golden byte a byte,
// que las dormidas no aparezcan.
//
// Heredando la variable del entorno, todos esos tests se ponen ROJOS en la máquina de cualquiera
// que la tenga puesta, y siguen VERDES en CI, que no la tiene. Es el peor rojo posible: el que
// sólo ve quien trabaja, y que por eso se aprende a ignorar. Medido: con la variable puesta caían
// seis tests de este paquete, ninguno por un defecto del código.
//
// Se limpia acá y no test por test porque el nivel correcto es el PAQUETE: lo que se hereda no es
// un dato de un caso, es la configuración global sobre la que casi todos estos casos afirman. Los
// tests que necesitan el catálogo COMPLETO lo piden explícitamente con t.Setenv, que restaura al
// terminar.
//
// 🔴 Y POR QUÉ ADEMÁS MIRA EL -timeout. Este es el paquete más caro del repo bajo `-race`, y ya
// no entra en el default de Go de 10 min por paquete. Sin esta guarda, `go test -race ./...` a
// secas quema diez minutos y muere con «panic: test timed out after 10m0s» y el stack del test
// que le tocó estar corriendo — un mensaje que acusa a un test inocente y no nombra el flag que
// falta. Con la guarda falla en el primer segundo diciendo qué correr. El techo lo saca de
// presupuesto-de-pruebas.env, no de un número tipeado acá.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("MUSUBI_TOOLS_ALL")
	// flag.Parse() antes de leer -test.timeout: testing.Init() ya registró el flag, pero quien
	// lo PARSEA es m.Run(), y para entonces ya sería tarde para avisar.
	flag.Parse()
	if motivo := testbudget.ExigirTimeoutSuficiente("."); motivo != "" {
		fmt.Fprintln(os.Stderr, "musubi/internal/mcp: presupuesto de pruebas insuficiente —", motivo)
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
