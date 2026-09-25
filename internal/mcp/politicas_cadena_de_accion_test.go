package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// A131 · T3 (revisión 2) — A LA CADENA QUE HACE ACTUAR UNA POLÍTICA SE ENTRA POR UN SOLO LUGAR, Y ESE
// LUGAR DECIDE EL ALCANCE ANTES.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ HACE FALTA
//
// Desde T3, evaluarPolitica no mira el alcance: lo decide aplicarPoliticas antes de llamarla, y antes
// incluso que la ventana de mantenimiento, porque es el ÚNICO lugar del barrido que lo decide (ver el
// porqué ahí). Lo que la hace segura es, entonces, que nadie más la llame. Hoy es así: su único
// llamador de producción es aplicarPoliticas, y TestUnaPoliticaActuaYFiguraSoloSobreLasMaquinasQueNombra
// mide que ése decide el alcance bien, en cada condición. Pero esa tabla mide al barrido, no a la
// función: la revisión 2 de T3 lo señaló —si aparece otro llamador, una tool de «probar política» por
// ejemplo, actúa fuera del alcance y nada se pone rojo—.
//
// Y LA OTRA SALIDA SE DESCARTÓ A PROPÓSITO: repetir pol.Alcanza adentro de evaluarPolitica. Es el mismo
// predicado y es barato, pero dos lecturas del alcance en la misma cadena se tapan entre sí. Medido en
// esta revisión, con -run de TestUnaPoliticaActuaYFiguraSoloSobreLasMaquinasQueNombra y el alcance
// repetido adentro de evaluarPolitica: el glob propio que esa tabla siembra en el barrido la deja en
// VERDE (sin la repetición cae en la acción, en el contador y en la ventana), y P2-m5 sólo se sigue
// viendo por el contador `mantenimiento` del segundo barrido —ya no en la acción ni en el `ok`—. O sea
// que la defensa de más apaga la alarma de la defensa que ya estaba. Lo que queda es que la cadena
// tenga una sola entrada, y eso sí se puede exigir.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Los eslabones de la cadena que termina en ejecutar —evaluarPolitica, evaluarPoliticaDeServicio,
// actuarSiCorresponde y correrAccionDePolitica— y quién puede nombrar a cada uno, leído del código de
// producción de internal/mcp (AST, archivos de `git ls-files`): una llamada o un valor de método desde
// cualquier otra función es una entrada nueva a la cadena, y la guarda falla diciendo dónde. Quien la
// agregue tiene que decidir el alcance antes con pol.Alcanza —el mismo predicado, no una copia— y
// sumar su prueba de comportamiento; recién entonces se agrega acá, con su porqué.
//
// PISO: cada eslabón existe, y cada llamador permitido lo nombra de verdad. Una entrada de la tabla que
// ya no se usa es una afirmación rancia —dice que alguien llama cuando nadie llama— y falla igual.
//
// Sabotaje: aparece un segundo llamador de evaluarPolitica, una tool de «probar política» que evalúa
// sobre la máquina que le digan, sin pasar por el alcance. El arreglo reescribe la llamada del barrido
// con una variable intermedia: sigue siendo el mismo llamador.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="// medidoParaLog traduce el «no sé» del dominio al del log.\n"
// arnes: a="// probarPolitica evalúa una política sobre la máquina que le digan.\nfunc (s *McpServer) probarPolitica(pol fleet.Politica, d fleet.Device) bool {\n\treturn s.evaluarPolitica(pol, d, time.Now())\n}\n\n// medidoParaLog traduce el «no sé» del dominio al del log.\n"
// arnes: arreglo_de="\t\t\tif s.evaluarPolitica(pol, d, ahora) {\n\t\t\t\tacciones++\n\t\t\t}\n"
// arnes: arreglo_a="\t\t\tactuo := s.evaluarPolitica(pol, d, ahora)\n\t\t\tif actuo {\n\t\t\t\tacciones++\n\t\t\t}\n"
func TestLaCadenaDeAccionDeUnaPoliticaTieneUnaSolaEntrada(t *testing.T) {
	// Quién puede nombrar a cada eslabón. La entrada es aplicarPoliticas, y es la que decide el alcance.
	llamadores := map[string][]string{
		"evaluarPolitica":           {"aplicarPoliticas"},
		"evaluarPoliticaDeServicio": {"evaluarPolitica"},
		"actuarSiCorresponde":       {"evaluarPolitica", "evaluarPoliticaDeServicio"},
		"correrAccionDePolitica":    {"actuarSiCorresponde"},
	}

	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()
	declarados := map[string]bool{}
	vistos := map[[2]string]bool{} // (eslabón, llamador)
	archivos := 0
	for _, rel := range archivosGo(t, raiz) {
		if strings.HasSuffix(rel, "_test.go") || path.Dir(rel) != "internal/mcp" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("no pude leer %s: %v — no medí nada", rel, err)
		}
		f, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v — no medí nada", rel, err)
		}
		archivos++
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if _, esEslabon := llamadores[fn.Name.Name]; esEslabon && fn.Recv != nil {
				declarados[fn.Name.Name] = true
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				permitidos, esEslabon := llamadores[sel.Sel.Name]
				if !esEslabon {
					return true
				}
				vistos[[2]string{sel.Sel.Name, fn.Name.Name}] = true
				if !slices.Contains(permitidos, fn.Name.Name) {
					t.Errorf("%s: %s nombra a %s, y a ese eslabón sólo se llega desde %s. Es una entrada nueva a la "+
						"cadena que termina en ejecutar un comando en una máquina, y evaluarPolitica NO mira el alcance: "+
						"lo decide aplicarPoliticas antes de llamarla. Si esta entrada hace falta, que decida el alcance "+
						"con pol.Alcanza antes de llamar, que tenga su prueba de comportamiento, y recién entonces sumala "+
						"a esta tabla", fset.Position(sel.Pos()), fn.Name.Name, sel.Sel.Name, strings.Join(permitidos, " o "))
				}
				return true
			})
		}
	}
	if archivos == 0 {
		t.Fatal("PISO: no encontré ni un archivo de producción de internal/mcp — no medí nada")
	}

	// PISO: cada eslabón existe, y cada llamador permitido lo nombra de verdad.
	eslabones := make([]string, 0, len(llamadores))
	for e := range llamadores {
		eslabones = append(eslabones, e)
	}
	sort.Strings(eslabones)
	for _, e := range eslabones {
		if !declarados[e] {
			t.Errorf("PISO: el eslabón %s no está declarado como método en internal/mcp: la cadena cambió de forma y esta "+
				"tabla describe una que ya no existe", e)
		}
		for _, quien := range llamadores[e] {
			if !vistos[[2]string{e, quien}] {
				t.Errorf("PISO: la tabla dice que %s llama a %s, y no lo llama: una afirmación rancia sobre quién entra a "+
					"la cadena es peor que ninguna", quien, e)
			}
		}
	}
}
