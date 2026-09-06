package mcp

import (
	"os"
	"testing"
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
func TestMain(m *testing.M) {
	_ = os.Unsetenv("MUSUBI_TOOLS_ALL")
	os.Exit(m.Run())
}
