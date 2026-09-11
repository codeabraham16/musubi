package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TODO RESULTADO QUE SE EMITE TIENE QUE SEMBRARSE, Y AL REVÉS.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO, Y LO PREDIJO SU PROPIO COMENTARIO
//
// `sembrarPoliticas` existe porque una serie AUSENTE y una serie en CERO no son lo mismo: la
// alerta que vive de ella usa `increase(...)`, y `increase()` sobre una serie que no existe no
// devuelve nada. Sin sembrar, «no actuó ninguna política» y «el cerebro dejó de exportar» se leen
// igual. Eso ya estaba arreglado y escrito.
//
// Lo que quedó abierto es más fino: `increase()` TAMPOCO PUEDE VER LA SUBIDA DE AUSENTE A 1.
// Necesita dos muestras de una serie que ya exista, así que el PRIMER evento de un resultado sin
// sembrar es invisible. Y el comentario de la siembra decía, textualmente, «sembrar de menos lo
// dejaría abierto para la próxima».
//
// LA PRÓXIMA LLEGÓ. Al 2026-09-11 la regla estaba escrita en TRES lugares y dos estaban mal:
//
//	doc de contarPolitica ......... SIETE resultados (correcto)
//	comentario de sembrarPoliticas  «los CUATRO resultados posibles»
//	el código de la siembra ....... CINCO en una lista a mano
//
// Los dos que faltaban eran `consentimiento_prohibido` y `consentimiento_pide`, los de A91, que
// nacieron después y nadie volvió a esa línea. El costo es una alerta PERDIDA, no falsa:
// `PoliticaFrenadaPorConsentimiento` es `increase(...{result=~"consentimiento_.*"}[24h]) > 0` con
// `for: 6h`, así que el primer bloqueo por consentimiento —que puede ser el único— no la levanta.
// El eje endurecido de A91 se justificó con «el auto-heal deja de actuar y ALGUIEN LO VE»; sin la
// siembra, ese alguien no existía.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ LA GUARDA LEE EL AST Y NO UNA LISTA
//
// Una prueba con los siete nombres escritos a mano sería una CUARTA copia de la misma regla, y por
// lo tanto la próxima en quedar vieja — exactamente el defecto que viene a cerrar. Acá el conjunto
// que se emite se DERIVA: se parsea `internal/mcp`, se buscan las llamadas a `contarPolitica` y se
// lee el literal del segundo argumento. Agregar un resultado nuevo sin sembrarlo pone esto rojo
// sin que nadie tenga que acordarse de nada.
//
// SE EXIGE IGUALDAD EN LOS DOS SENTIDOS, y el segundo también importa: sembrar un resultado que
// nadie emite no rompe ninguna alerta, pero deja una serie en cero para siempre que se lee como
// cobertura. Un eje que nunca se enciende y un eje que no existe se ven igual desde Prometheus.
//
// Sabotaje: sacar cualquiera de los siete de `resultadosDePolitica`, o agregar una llamada a
// `contarPolitica` con un resultado nuevo. Los dos corridos.
func TestSeSiembranTodosLosResultadosQueSeEmiten(t *testing.T) {
	emitidos := map[string]string{} // resultado -> archivo:línea donde se emite
	fset := token.NewFileSet()

	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude leer el paquete: %v", err)
	}
	var conLlamada int
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "contarPolitica" {
				return true
			}
			conLlamada++
			lit, ok := call.Args[1].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				// UN RESULTADO QUE NO ES LITERAL NO SE PUEDE DERIVAR, y callarlo sería un cero
				// que significa «no sé». Se denuncia en vez de ignorarse.
				t.Errorf("%s: `contarPolitica` recibe un resultado que no es un literal de cadena. "+
					"Esta guarda deriva el conjunto emitido del código, y con una variable no puede: "+
					"o lo dejás literal, o esta guarda deja de cubrir ese camino sin decirlo",
					fset.Position(call.Pos()))
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			emitidos[v] = fset.Position(call.Pos()).String()
			return true
		})
	}

	// CONTROL DE «MIRÓ ALGO». Cero llamadas encontradas no es «no se emite nada»: es que el
	// barrido no llegó, o que `contarPolitica` se renombró y esta guarda quedó apuntando al aire.
	if conLlamada == 0 {
		t.Fatal("no encontré NI UNA llamada a `contarPolitica` en internal/mcp. Un cero acá es «no " +
			"pude medir», no «no hay nada que sembrar»: o el barrido no llegó, o la función se " +
			"renombró y esta guarda quedó apuntando al aire")
	}

	sembrados := map[string]bool{}
	for _, r := range resultadosDePolitica {
		sembrados[r] = true
	}

	var sinSembrar []string
	for r, donde := range emitidos {
		if !sembrados[r] {
			sinSembrar = append(sinSembrar, r+" (se emite en "+donde+")")
		}
	}
	sort.Strings(sinSembrar)
	if len(sinSembrar) > 0 {
		t.Errorf("%d resultado(s) se EMITEN y no se SIEMBRAN:\n    %s\n\n"+
			"Su serie no existe hasta el primer evento, y `increase()` no puede ver la subida de "+
			"AUSENTE a 1 —necesita dos muestras de una serie que ya exista—. O sea que el PRIMER "+
			"evento de cada uno es invisible para su alerta, y si es el único, la alerta no suena "+
			"nunca. Agregalos a `resultadosDePolitica`.",
			len(sinSembrar), strings.Join(sinSembrar, "\n    "))
	}

	var sinEmitir []string
	for r := range sembrados {
		if _, ok := emitidos[r]; !ok {
			sinEmitir = append(sinEmitir, r)
		}
	}
	sort.Strings(sinEmitir)
	if len(sinEmitir) > 0 {
		t.Errorf("%d resultado(s) se SIEMBRAN y nadie los emite: %s.\n"+
			"No rompe ninguna alerta, pero deja una serie en cero para siempre que se lee como "+
			"cobertura: un eje que nunca se enciende y un eje que no existe se ven igual desde "+
			"Prometheus. Si el camino que lo emitía se borró, sacalo de `resultadosDePolitica`.",
			len(sinEmitir), strings.Join(sinEmitir, ", "))
	}
}
