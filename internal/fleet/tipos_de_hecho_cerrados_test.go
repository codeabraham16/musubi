package fleet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
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
func tiposDeHechoDeclarados(t *testing.T) map[string]string {
	t.Helper()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				tipado := esIdent(vs.Type, "TipoDeHecho")
				for i, nombre := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					v := vs.Values[i]
					if c, ok := v.(*ast.CallExpr); ok && esIdent(c.Fun, "TipoDeHecho") && len(c.Args) == 1 {
						v = c.Args[0]
					} else if !tipado {
						continue
					}
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("%s: %s es un TipoDeHecho con un valor que no es un literal; ampliá el barrido",
							fset.Position(vs.Pos()), nombre.Name)
					}
					valor, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					out[valor] = nombre.Name
				}
			}
		}
	}
	return out
}

func esIdent(e ast.Expr, nombre string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == nombre
}
