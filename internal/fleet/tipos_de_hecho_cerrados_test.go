package fleet

import (
	"go/ast"
	"testing"
)

// TiposDeHecho ES EL ENUM ENTERO, Y SE CIERRA CONTRA SU FUENTE: cada constante de tipo TipoDeHecho
// declarada en el paquete —en cualquier archivo, con cualquier build tag— está en la lista, y la
// lista no inventa ninguna.
//
// La lista la recorren tres guardas: la de conformidad (capacidad y plano por tipo, acá al lado),
// la de bordes de los lectores de ventana (internal/memory) y el inventario de la cronología en
// mcp. Las tres heredan lo que la lista olvide. Y ya olvidó una: HechoCanalExec faltó, y la prueba
// de conformidad recorrió 6 de 7 sin poder ver el séptimo. Escrita a mano, la lista es una copia
// del bloque const; esta prueba es lo que la ata al original.
//
// Sabotaje: sacar HechoCanalExec de la lista, que es el olvido que ya pasó → las guardas que la
// recorren dejan de ver el tipo y ésta lo nombra. Pisa la línea que sabotea
// TestCadaTipoDeHechoMostrableTieneCapacidadYPlano (un tipo en la lista sin capacidad): son dos
// guardas sobre la misma línea, con motivos distintos, medidos los dos.
// arnes: colision_ok="TestCadaTipoDeHechoMostrableTieneCapacidadYPlano"
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tHechoCanalPantalla, HechoCanalShell, HechoCanalExec, HechoSinClasificar,"
// arnes: a="\tHechoCanalPantalla, HechoCanalShell, HechoSinClasificar,"
// Sabotaje: declarar un tipo nuevo sin sumarlo a la lista → queda fuera de las tres guardas.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tHechoSinClasificar TipoDeHecho = \"sin_clasificar\"\n)"
// arnes: a="\tHechoSinClasificar TipoDeHecho = \"sin_clasificar\"\n\tHechoNuevoSinLista TipoDeHecho = \"nuevo_sin_lista\"\n)"
func TestTiposDeHechoEsElEnumEntero(t *testing.T) {
	declarados := tiposDeHechoDeclarados(t)
	// EL PISO: si el barrido no encontrara nada, la comparación de abajo no mediría nada.
	if len(declarados) < 7 {
		t.Fatalf("el barrido del paquete encontró %d constantes TipoDeHecho (%v); el enum tenía 7 "+
			"cuando se escribió esta prueba: el barrido está roto", len(declarados), declarados)
	}

	enLista := map[string]bool{}
	for _, tipo := range TiposDeHecho {
		if enLista[string(tipo)] {
			t.Errorf("TiposDeHecho repite %q", tipo)
		}
		enLista[string(tipo)] = true
	}
	for valor, nombre := range declarados {
		if !enLista[valor] {
			t.Errorf("%s (%q) está declarado en el paquete y falta en TiposDeHecho: las guardas que "+
				"recorren la lista no lo ven", nombre, valor)
		}
	}
	for valor := range enLista {
		if _, ok := declarados[valor]; !ok {
			t.Errorf("TiposDeHecho tiene %q y ninguna constante del paquete lo declara", valor)
		}
	}
}

// tiposDeHechoDeclarados devuelve valor → nombre de cada constante de tipo TipoDeHecho que
// declaran los archivos de producción del paquete: con el tipo escrito (`X TipoDeHecho = "x"`) o
// convertido (`X = TipoDeHecho("x")`).
//
// El barrido es el de constantesDeclaradas (origen_cerrado_test.go), que desde A131·T9 cierra
// también el enum de OrigenComando: una sola copia del recorrido, para que las dos listas no se
// aten a su fuente con dos reglas distintas.
func tiposDeHechoDeclarados(t *testing.T) map[string]string {
	t.Helper()
	return constantesDeclaradas(t, "TipoDeHecho")
}

func esIdent(e ast.Expr, nombre string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == nombre
}
