package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTodoAcuneDeCredencialGastaLaAprobacion exige que la función que ACUÑA una credencial de
// sesión sea la misma que CONSUME el permiso de cuatro ojos.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL DEFECTO QUE LA TRAJO, Y POR QUÉ NO SE PODÍA VER
//
// `toolFleetShell` gastaba la aprobación en el LLAMADOR, justo antes de registrar la sesión.
// Correcto para el camino normal y ciego para el otro: con consentimiento `pide`, esa función
// devuelve antes de llegar ahí, y cuando la persona acepta, la shell se abre desde
// `pedirPermisoParaShell`, que llama a `abrirShellConSesion` directo. En una máquina con
// `RequiereAprobacion` Y `pide`, el permiso se comprobaba en la puerta y NO SE CONSUMÍA NUNCA:
// uno que el propio código llama «de un solo uso» quedaba vigente hasta vencer y abría todas las
// sesiones que entraran en esa ventana.
//
// EL GEMELO YA ESTABA ARREGLADO Y ESCRITO. `entregarPantalla` gasta adentro, «en el único lugar
// que acuña una credencial de pantalla», y dice textual que entre la puerta y ese punto «puede
// haber pasado un diálogo de `pide` entero, que es donde la primera versión perdía la
// aprobación». La lección estaba aprendida de un lado y no del hermano, a treinta líneas.
//
// Y NADA PODÍA VERLO: `RequiereAprobacion` no aparecía en NI UNA prueba del árbol —medido sobre
// `git ls-files '*_test.go'`—, así que el control de cuatro ojos no estaba ejercitado de punta a
// punta. `consentimiento_matriz_test.go` recorre los grados de consentimiento con los cuatro ojos
// fijos en falso: generaliza sobre un eje de una matriz de dos, que es la misma frase que ese
// archivo ya tiene escrita sobre un bug anterior suyo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ SE DERIVA DEL AST Y NO SE ENUMERAN LOS CAMINOS
//
// Enumerar los caminos no converge: esta guarda existe porque alguien enumeró dos y aparecieron
// tres. Lo que se fija es la PROPIEDAD: la función que llama a la primitiva de acuñe tiene que
// llamar también a `gastarAprobacion`. Un camino nuevo que acuñe sin gastar se pone rojo solo,
// sin que nadie tenga que acordarse de agregarlo a una lista.
//
// LAS PRIMITIVAS SON UN HECHO DEL MUNDO Y VAN ESCRITAS A MANO: cuáles llamadas crean una
// credencial de sesión no se puede derivar del código sin volver espejo a la guarda. Derivarlas
// de «las funciones que ya llaman a gastarAprobacion» la dejaría aceptando cualquier cosa.
//
// Sabotaje que la hace fallar: sacar el `gastarAprobacion` de `abrirShellConSesion` → el acuñe de
// la shell queda sin gasto y esta guarda lo nombra.
// arnes: archivo="internal/mcp/methods_shell.go"
// arnes: de="\tif e := s.gastarAprobacion(d, p, fleet.CapShell, time.Now().UTC()); e != nil {\n\t\treturn nil, e\n\t}\n\n\tcanal, err := s.abrirCanalShell(d, filas, columnas)"
// arnes: a="\tcanal, err := s.abrirCanalShell(d, filas, columnas)"
// arnes: prueba="TestTodoAcuneDeCredencialGastaLaAprobacion"
func TestTodoAcuneDeCredencialGastaLaAprobacion(t *testing.T) {
	// LO QUE ACUÑA UNA CREDENCIAL DE SESIÓN. Hecho del mundo, escrito a mano.
	acunan := map[string]string{
		"abrirCanalShell":   "abre el canal de la terminal",
		"NuevaPassPantalla": "acuña la contraseña de pantalla",
	}
	const gasta = "gastarAprobacion"

	fset := token.NewFileSet()
	dir := "."
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// EL CONTROL DE QUE SE MIRÓ ALGO: si el barrido no encuentra NINGÚN acuñe, esta guarda pasaría
	// en verde sobre un paquete vacío — que es el cero que significa «no sé», no «está bien».
	sitios := 0
	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, nombre), nil, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v — no medí nada", nombre, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			llamadas := map[string]token.Pos{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := c.Fun.(type) {
				case *ast.Ident:
					llamadas[fun.Name] = c.Pos()
				case *ast.SelectorExpr:
					llamadas[fun.Sel.Name] = c.Pos()
				}
				return true
			})
			for prim, queHace := range acunan {
				pos, acuna := llamadas[prim]
				if !acuna {
					continue
				}
				sitios++
				if _, gastada := llamadas[gasta]; !gastada {
					t.Errorf("%s · %s llama a %s() —%s— y NO llama a %s().\n"+
						"  Un camino que acuña una credencial sin consumir el permiso de cuatro ojos deja\n"+
						"  vigente un permiso que el propio código llama «de un solo uso»: sirve otra vez,\n"+
						"  y otra, hasta que vence. Comprobarlo en la puerta NO alcanza — entre la puerta y\n"+
						"  el acuñe puede pasar un diálogo de `pide` entero.\n"+
						"  El gasto va ADENTRO de la función que acuña, no en sus llamadores: dos lugares\n"+
						"  donde gastar es un lugar donde olvidarse. Ver `entregarPantalla`, que ya lo hace.",
						fset.Position(pos), fn.Name.Name, prim, queHace, gasta)
				}
			}
		}
	}

	if sitios == 0 {
		t.Fatalf("no encontré NI UN acuñe de credencial en %s: o el barrido está roto o las "+
			"primitivas cambiaron de nombre. Un verde de cero sitios se lee igual que un verde de "+
			"todos, y esta guarda no mediría nada.", dir)
	}
	t.Logf("%d sitio/s de acuñe revisados, todos gastan la aprobación", sitios)
}
