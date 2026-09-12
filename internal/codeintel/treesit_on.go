//go:build treesitter

package codeintel

import (
	"bytes"
	"path/filepath"
	"strings"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// treesit_on.go deriva el grafo de código de lenguajes NO-Go (TS/TSX/JS/Python) vía tree-sitter
// (Track 20 · F4), model-free y SIN CGo (gotreesitter es un runtime tree-sitter 100% Go). Está
// detrás del build tag `treesitter`: el binario por default de Musubi NO lo linkea (queda lean;
// TS/Py siguen solo-símbolos). Para activarlo se compila con:
//
//	-tags 'treesitter grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_javascript grammar_subset_python'
//
// (los grammar_subset_* acotan las gramáticas embebidas a las que usamos: pocos MB, no las 206).

// motorPolyglot es la mitad del sello del derivador que depende del BUILD TAG, y no de ninguna
// dependencia. Ver su gemela en treesit_off.go y la guarda en sello_del_tag_test.go.
const motorPolyglot = "poly-on"

// languageFor devuelve la gramática tree-sitter del archivo, o nil si no lo soportamos.
//
// LA RED EMPIEZA ACÁ Y NO EN `derivePolyglotFile`, Y ESE ERA EL DEFECTO. CARGAR la gramática es una
// llamada a la dependencia —`grammars.TsxLanguage()` y sus hermanas— y puede entrar en pánico por
// las mismas razones que el parseo: es exactamente el escenario «gotreesitter mete pánicos
// incondicionales» que `derivePolyglotFile` cita como su motivo. Sólo que se entra ANTES y AFUERA
// de aquella red, por `polyglotSupported`:
//
//	graph.go:116  IndexableForGraph → polyglotSupported → languageFor   ← el indexador MCP
//	graph.go:374  el filtro de DerivePackage, UNA LÍNEA arriba del llamado protegido
//
// Medido el 2026-09-12 con `case ".tsx": panic(…)`: las DOS guardas del pánico quedaron VERDES y el
// proceso se cayó igual. Lo cazaba `TestIndexableForGraphWithTreesitter`, pero por explotar él
// también, no por afirmar contención — o sea que CI lo DETECTA y nadie lo CONTIENE, y en producción
// se lo lleva el daemon por la goroutine pelada del scheduler del grafo.
//
// Degradar a nil es la misma respuesta honesta que da la red de abajo: el archivo queda como no
// derivable, que es lo que contesta un `languageFor` que no conoce la extensión.
func languageFor(path string) (lang *ts.Language) {
	defer func() {
		if r := recover(); r != nil {
			lang = nil
		}
	}()
	return cargarGramatica(path)
}

// cargarGramatica existe por la prueba: no hay entrada conocida que haga entrar en pánico al
// cargador real, así que la guarda inyecta uno acá. Misma costura que `derivarPolyglotSinRed`.
var cargarGramatica = cargarGramaticaDeVerdad

func cargarGramaticaDeVerdad(path string) *ts.Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts":
		return grammars.TypescriptLanguage()
	case ".tsx":
		return grammars.TsxLanguage()
	case ".js", ".jsx", ".mjs", ".cjs":
		return grammars.JavascriptLanguage()
	case ".py":
		return grammars.PythonLanguage()
	}
	return nil
}

// polyglotSupported indica si el archivo se deriva por tree-sitter (F4).
func polyglotSupported(path string) bool { return languageFor(path) != nil }

// tsKind mapea el Kind language-neutral de gotreesitter a nuestros Kind de símbolo.
func tsKind(k string) string {
	switch strings.ToLower(k) {
	case "function", "func":
		return KindFunc
	case "method":
		return KindMethod
	case "class":
		return KindClass
	case "type", "interface", "type_alias", "enum":
		return KindType
	case "const":
		return KindConst
	case "var", "let", "variable":
		return KindVar
	default:
		return KindDef
	}
}

// defSpan es el rango en bytes de un símbolo (para atribuir cada call-site a su función envolvente).
type defSpan struct {
	start, end uint32
	key        string
}

// derivePolyglotFile deriva nodos+aristas de un archivo TS/JS/Py, en su forma honesta: símbolos
// (CONTAINS), imports (IMPORTS, external = import bare), y CALLS INTRA-archivo (callee resuelto a
// un def del mismo archivo con match único; cross-archivo diferido, como el cross-paquete de Go en
// F1). Degrada a vacío sin pánico si no parsea. DerivePackage deduplica al mergear.
//
// «SIN PÁNICO» ERA UNA PROMESA DEL COMENTARIO Y NO DEL CÓDIGO. Hasta el 2026-09-12 esta función no
// tenía ninguna red: un pánico adentro de la gramática o del motor subía por `DerivePackage` →
// `toolCodegraphIndex` → `reindexCodeGraphOnce` hasta `RunCodeGraphScheduler`, que corre en una
// goroutine PELADA (`go server.RunCodeGraphScheduler(...)`, cmd/musubi/main.go:558). Un pánico ahí
// no degrada un archivo: se lleva el proceso entero —la memoria, el MCP, todo— por un `.js`
// cualquiera de un repo indexado.
//
// La red va ACÁ, en el grano del archivo, y no arriba en el scheduler, por dos razones. Una: es
// donde está la promesa escrita, y una promesa que el código no cumple es peor que no hacerla.
// Dos: acá se sabe QUÉ degradar —este archivo, a vacío— mientras que un recover en el scheduler
// sólo sabe que algo explotó y tiene que abandonar la corrida entera.
//
// Ojo con leer esto como «ya está cubierto»: las OTRAS ocho goroutines que larga `main` siguen sin
// red, y esta función es la única que se la puso. Eso es una decisión de diseño (¿un pánico debe
// matar el daemon para que systemd lo levante limpio, o debe contenerse?) que no se toma acá.
func derivePolyglotFile(path, content string) (nodes []Node, edges []Edge) {
	defer func() {
		if r := recover(); r != nil {
			// Se devuelve VACÍO y no parcial: un grafo a medias es peor que ninguno —reemplaza al
			// anterior y BORRA símbolos que sí existían—. El archivo queda como no derivable, que
			// es lo mismo que contesta un `languageFor` nil.
			//
			// HOY ESTA ASIGNACIÓN ES INALCANZABLE Y CONVIENE DECIRLO EN VEZ DE DEJAR QUE EL
			// COMENTARIO PROMETA DE MÁS. El cuerpo vive en OTRA función, así que los retornos
			// nombrados sólo se escriben en el `return` de abajo: cuando el pánico ocurre YA son
			// nil. Medido el 2026-09-12 borrando esta línea entera —el paquete con tags quedó
			// VERDE—, o sea que ninguna prueba la alcanza ni puede alcanzarla por la costura.
			//
			// SE DEJA A PROPÓSITO, y no es adorno: el día que alguien inline `derivarPolyglotSinRed`
			// acá adentro —por un perfilado, por sacar la indirección «que es sólo para la prueba»—
			// el parcial pasa a ser posible y esta línea es lo único entre eso y un grafo que borra
			// símbolos. Que sea código muerto HOY es una propiedad de la estructura de hoy.
			nodes, edges = nil, nil
		}
	}()
	return derivarPolyglotSinRed(path, content)
}

// derivarPolyglotSinRed es el cuerpo, separado para que la guarda pueda inyectarle un pánico: no
// hay forma de hacer entrar en pánico al parser REAL a pedido, y una guarda que no puede provocar
// el defecto no distingue «hay red» de «no la probé». La indirección es por la prueba y se dice.
var derivarPolyglotSinRed = derivarPolyglotDeVerdad

func derivarPolyglotDeVerdad(path, content string) ([]Node, []Edge) {
	lang := languageFor(path)
	if lang == nil {
		return nil, nil
	}
	src := []byte(content)
	tree, err := ts.NewParser(lang).Parse(src)
	if err != nil || tree == nil {
		return nil, nil
	}

	fileKey := FileKey(path)
	nodes := []Node{{Key: fileKey, Kind: KindFile, Name: filepath.Base(path), Path: path}}
	var edges []Edge

	// Símbolos → nodos + CONTAINS. Tabla nombre→key para resolver CALLS (match único).
	byName := map[string]string{}
	ambiguous := map[string]bool{}
	var spans []defSpan
	for _, d := range ts.ExtractDefinitionSpans(tree) {
		if d.Name == "" {
			continue
		}
		kind := tsKind(d.Kind)
		key := SymbolKey(path, kind, d.Name)
		nodes = append(nodes, Node{
			Key: key, Kind: kind, Name: d.Name, Path: path,
			StartLine: lineAtByte(src, d.StartByte), EndLine: lineAtByte(src, d.EndByte),
		})
		edges = append(edges, Edge{FromKey: fileKey, ToKey: key, Kind: EdgeContains, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
		if _, seen := byName[d.Name]; seen {
			ambiguous[d.Name] = true
		} else {
			byName[d.Name] = key
		}
		spans = append(spans, defSpan{d.StartByte, d.EndByte, key})
	}

	// Imports → nodos package + IMPORTS. Bare = externo; relativo (empieza con ".") = in-project.
	for _, im := range fileImports(path, tree.RootNode(), src, lang) {
		pk := PackageKey(im.path)
		nodes = append(nodes, Node{Key: pk, Kind: KindPackage, Name: im.path, External: im.external})
		edges = append(edges, Edge{FromKey: fileKey, ToKey: pk, Kind: EdgeImports, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
	}

	// CALLS intra-archivo: caller = def que ENVUELVE el call-site; callee = def homónimo del mismo
	// archivo (único). Ambiguo o no resuelto ⇒ se omite (no se inventa).
	for _, c := range ts.ExtractCalls(tree) {
		if ambiguous[c.Name] {
			continue
		}
		target, ok := byName[c.Name]
		if !ok {
			continue
		}
		caller := enclosingDef(spans, c.StartByte)
		if caller == "" || caller == target {
			continue
		}
		edges = append(edges, Edge{FromKey: caller, ToKey: target, Kind: EdgeCalls, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
	}
	return nodes, edges
}

// enclosingDef devuelve la clave del símbolo cuyo rango de bytes CONTIENE `at`, el más chico
// (innermost) — la función/método donde ocurre la llamada. "" si ninguno.
func enclosingDef(spans []defSpan, at uint32) string {
	best := ""
	var bestSize uint32 = ^uint32(0)
	for _, s := range spans {
		if at >= s.start && at < s.end {
			if size := s.end - s.start; size < bestSize {
				bestSize, best = size, s.key
			}
		}
	}
	return best
}

// lineAtByte convierte un offset de bytes a número de línea 1-based.
func lineAtByte(src []byte, off uint32) int {
	if int(off) > len(src) {
		off = uint32(len(src))
	}
	return 1 + bytes.Count(src[:off], []byte{'\n'})
}

// importDecl es un import derivado: su module path y si es externo (no in-project).
type importDecl struct {
	path     string
	external bool
}

// fileImports devuelve los imports del archivo. Python usa el extractor de gotreesitter (que sí
// los da); para TS/TSX/JS/JSX se recorre el árbol, porque gotreesitter NO extrae imports de esos
// lenguajes (devuelve 0): un import ES es `(import_statement ... (string ...))` y el módulo es el
// nodo `string`. Se deduplica por path. (require()/import() dinámico quedan fuera de F4.)
func fileImports(path string, root *ts.Node, src []byte, lang *ts.Language) []importDecl {
	if strings.ToLower(filepath.Ext(path)) == ".py" {
		var out []importDecl
		for _, im := range ts.ExtractImportsFromSource(lang, src) {
			if im.Path != "" {
				out = append(out, importDecl{path: im.Path, external: im.Relative == 0})
			}
		}
		return out
	}
	var out []importDecl
	seen := map[string]bool{}
	var walk func(n *ts.Node)
	walk = func(n *ts.Node) {
		if n == nil {
			return
		}
		if n.Type(lang) == "import_statement" {
			if s := firstNamedChildOfType(n, "string", lang); s != nil {
				p := strings.Trim(s.Text(src), "'\"`")
				if p != "" && !seen[p] {
					seen[p] = true
					out = append(out, importDecl{path: p, external: !strings.HasPrefix(p, ".")})
				}
			}
		}
		for i := 0; i < n.NamedChildCount(); i++ {
			walk(n.NamedChild(i))
		}
	}
	walk(root)
	return out
}

// firstNamedChildOfType devuelve el primer hijo nombrado de n con ese type, o nil.
func firstNamedChildOfType(n *ts.Node, typ string, lang *ts.Language) *ts.Node {
	for i := 0; i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c.Type(lang) == typ {
			return c
		}
	}
	return nil
}

// PolyglotHabilitado dice si ESTE binario linkeó tree-sitter. Ver treesit_off.go por el porqué.
func PolyglotHabilitado() bool { return true }
