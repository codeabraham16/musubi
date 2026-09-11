package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// NINGÚN CAMPO DE MEMORIA AJENA SE INTERPOLA CRUDO EN TEXTO PARA UN MODELO.
//
// El defecto que esto custodia se midió el 2026-09-11: `topic_key` y `gist` se metían tal cual en
// bloques orientados a líneas, y una observación con un salto de línea conseguía una línea propia
// adentro del prompt. Ver internal/memory/linea_ajena.go.
//
// POR QUÉ UNA GUARDA DE AST Y NO CUATRO PRUEBAS, UNA POR SITIO. Arreglar los cuatro sitios que
// había no impide el quinto, y el quinto es el que va a doler — el defecto dominante de este repo
// es una regla puesta en N-1 de N caminos. Esto no enumera sitios: los DERIVA del árbol sintáctico,
// así que un sitio nuevo nace cubierto sin que nadie se acuerde de agregarlo a una lista.
//
// ★ EL LÍMITE, DICHO DE FRENTE: sólo ve la interpolación DIRECTA. Si el campo pasa por una variable
// —`body := it.Gist` y después `Fprintf(..., body)`— este barrido no lo ve, porque seguir el flujo
// de datos en Go pide un análisis que esta guarda no hace. Ese caso existe y es deliberado
// (methods_cognition.go: el cuerpo del RAG es multilínea a propósito y se protege CERCÁNDOLO, no
// colapsándolo). Decirlo acá es parte de la guarda: una cobertura que se cree total es peor que una
// parcial que se conoce.

// camposDeMemoriaAjena son los que escribe quien guarda la observación. El `id` no está: lo genera
// el motor. El `content` tampoco: nunca va a un bloque de líneas —va cercado— y colapsarlo
// destruiría el material del RAG.
var camposDeMemoriaAjena = map[string]bool{"TopicKey": true, "Gist": true}

// funcionesQueFormatean son las que arman texto que después lee un modelo.
var funcionesQueFormatean = map[string]bool{"Fprintf": true, "Sprintf": true, "Printf": true}

// envuelveEnUnaLinea responde si la expresión ya pasó por el saneador.
func envuelveEnUnaLinea(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch f := c.Fun.(type) {
	case *ast.SelectorExpr: // memory.EnUnaLinea(...)
		return f.Sel != nil && f.Sel.Name == "EnUnaLinea"
	case *ast.Ident: // EnUnaLinea(...) dentro del paquete memory
		return f.Name == "EnUnaLinea"
	}
	return false
}

func TestNingunCampoDeMemoriaSeInterpolaCrudo(t *testing.T) {
	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()

	crudos := []string{}
	saneados := 0

	for _, rel := range archivosGo(t, raiz) {
		// Modo 0: SIN comentarios. Ningún comentario puede satisfacer ni disparar esta guarda —
		// que es el defecto que este repo ya pagó siete veces.
		f, err := parser.ParseFile(fset, filepath.Join(raiz, rel), nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || !funcionesQueFormatean[sel.Sel.Name] {
				return true
			}
			for _, arg := range call.Args {
				if envuelveEnUnaLinea(arg) {
					// Sólo cuenta como saneado si lo que envuelve ES un campo de memoria: envolver
					// otra cosa no prueba que este sitio esté cubierto.
					if c := arg.(*ast.CallExpr); len(c.Args) > 0 {
						if s, ok := c.Args[0].(*ast.SelectorExpr); ok && s.Sel != nil && camposDeMemoriaAjena[s.Sel.Name] {
							saneados++
						}
					}
					continue
				}
				s, ok := arg.(*ast.SelectorExpr)
				if !ok || s.Sel == nil || !camposDeMemoriaAjena[s.Sel.Name] {
					continue
				}
				crudos = append(crudos, rel+":"+
					strconv.Itoa(fset.Position(s.Pos()).Line)+" — ."+s.Sel.Name)
			}
			return true
		})
	}

	// CONTROL POSITIVO. Sin esto, un barrido que no encuentre NADA —otra raíz, el parser fallando,
	// el enumerador vacío— daría verde por no haber mirado. «No hay crudos» y «no medí» se
	// escriben igual.
	if saneados == 0 {
		t.Fatal("no encontré NI UNA interpolación saneada con EnUnaLinea en todo el repo. " +
			"Eso no es «está todo bien»: es que esta guarda no miró nada")
	}

	if len(crudos) > 0 {
		t.Errorf("hay %d campo(s) de memoria ajena interpolados CRUDOS en texto que lee un modelo:\n    %s\n\n"+
			"Un `topic_key` o un `gist` con un salto de línea se sale de su renglón y consigue una línea propia "+
			"adentro del prompt — medido el 2026-09-11 con el hook real. Pasalos por memory.EnUnaLinea "+
			"(internal/memory/linea_ajena.go). Si este sitio NO es un bloque de líneas y el multilínea es "+
			"deliberado, el trato correcto es CERCARLO con un nonce, como toolAsk: no lo dejes crudo.",
			len(crudos), strings.Join(crudos, "\n    "))
	}
}

// EL CERCO DEL PROMPT FUNDAMENTADO (musubi_ask) ES OTRO MECANISMO, Y POR ESO SE MIDE APARTE.
//
// Ahí el cuerpo de cada memoria es el CONTENIDO entero, multilínea a propósito: colapsarlo
// destruiría el material del RAG. La protección no es sanear sino CERCAR, y el cerco sólo sirve si
// no se puede cerrar desde adentro.

// EL NONCE ES LA GARANTÍA. Con un delimitador fijo, una observación que lo contenga cierra su
// propio cerco y lo que siga queda con el mismo rango que el prompt del sistema. La salida obvia
// —filtrar el delimitador del cuerpo— no converge y además mutila una nota legítima que lo
// mencione. Ocho bytes aleatorios por llamada lo vuelven irrepresentable: quien escribió la nota no
// los puede adivinar.
func TestElCercoDelPromptNoSePuedeCerrarDesdeAdentro(t *testing.T) {
	a1, c1 := marcasDeCerco()
	a2, c2 := marcasDeCerco()

	if a1 == a2 || c1 == c2 {
		t.Errorf("el cerco NO cambia entre llamadas, así que es adivinable y por lo tanto falsificable:\n  %q / %q", a1, a2)
	}
	// Y LAS DOS MARCAS DE UNA MISMA LLAMADA TIENEN QUE COMPARTIR EL NONCE. Si la de apertura y la
	// de cierre sortearan por separado, el cerco no cerraría nunca y el prompt quedaría con una
	// marca huérfana — un cerco roto se lee igual de bien que uno sano.
	nonce := strings.TrimPrefix(a1, "<<<MEMORIA-")
	if nonce == a1 || !strings.Contains(c1, nonce) {
		t.Errorf("apertura y cierre no comparten el nonce:\n  abre:  %q\n  cierra: %q", a1, c1)
	}
	if len(nonce) < 8 {
		t.Errorf("el nonce es de %d caracteres: demasiado corto para no ser adivinable (%q)", len(nonce), nonce)
	}
}
