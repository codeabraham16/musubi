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

// languageFor devuelve la gramática tree-sitter del archivo, o nil si no lo soportamos.
func languageFor(path string) *ts.Language {
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
func derivePolyglotFile(path, content string) ([]Node, []Edge) {
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

	// Imports → nodos package + IMPORTS. Relative>0 (import relativo) = in-project; bare = externo.
	for _, im := range ts.ExtractImportsFromSource(lang, src) {
		if im.Path == "" {
			continue
		}
		pk := PackageKey(im.Path)
		nodes = append(nodes, Node{Key: pk, Kind: KindPackage, Name: im.Path, External: im.Relative == 0})
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
