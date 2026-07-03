// Package codeintel extrae ESTRUCTURA de código (símbolos, imports) y parsea diffs
// de git, todo en Go puro y model-free. Es el núcleo de la detección de cambios: su
// principio rector es DERIVAR del estado ACTUAL del archivo, nunca de datos guardados,
// de modo que los símbolos y los rangos de un diff vivan siempre en el mismo sistema de
// coordenadas (el estado nuevo) y no se desalineen. No depende del motor de memoria ni
// de la base de datos: recibe contenido y devuelve estructura.
package codeintel

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// Kinds de símbolo reconocidos.
const (
	KindFunc   = "func"
	KindMethod = "method"
	KindClass  = "class"
	KindType   = "type"
	KindConst  = "const"
	KindVar    = "var"
	KindDef    = "def"
)

// Symbol es una declaración top-level con su rango de líneas (1-based, inclusivo),
// derivada del contenido actual del archivo.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// ExtractSymbols despacha por extensión y devuelve los símbolos top-level del contenido.
// Nunca entra en pánico; si la extensión no está soportada o el parseo falla del todo,
// devuelve una lista vacía (degradación a granularidad de archivo).
func ExtractSymbols(path, content string) []Symbol {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return extractGo(content)
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return extractBrace(content)
	case ".py":
		return extractPy(content)
	default:
		return nil
	}
}

// extractGo usa go/ast en modo tolerante: aun si el archivo no compila, parser.ParseFile
// devuelve un AST parcial con las declaraciones que sí pudo resolver.
func extractGo(content string) []Symbol {
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", content, parser.SkipObjectResolution)
	if file == nil {
		return nil
	}
	var out []Symbol
	lineOf := func(p token.Pos) int {
		if !p.IsValid() {
			return 0
		}
		return fset.Position(p).Line
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := KindFunc
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = KindMethod
			}
			out = appendSym(out, d.Name.Name, kind, lineOf(d.Pos()), lineOf(d.End()))
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					out = appendSym(out, s.Name.Name, KindType, lineOf(d.Pos()), lineOf(s.End()))
				case *ast.ValueSpec:
					kind := KindVar
					if d.Tok == token.CONST {
						kind = KindConst
					}
					for _, name := range s.Names {
						out = appendSym(out, name.Name, kind, lineOf(name.Pos()), lineOf(s.End()))
					}
				}
			}
		}
	}
	return out
}

// Regex ancladas para lenguajes de llaves (JS/TS): solo construcciones inequívocas.
var (
	reJSFunc  = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+(\w+)`)
	reJSClass = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+(\w+)`)
	reJSArrow = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*(?:async\s*)?\([^)]*\)\s*(?::[^=>]+)?=>`)
)

// extractBrace reconoce funciones, clases y const-arrow top-level; estima EndLine por
// balance de llaves desde el inicio del símbolo. Es aproximado (no lexea strings), acotado
// a construcciones no ambiguas: ante la duda, omite el símbolo en vez de inventarlo.
func extractBrace(content string) []Symbol {
	lines := strings.Split(content, "\n")
	var out []Symbol
	for i, line := range lines {
		var name, kind string
		switch {
		case reJSClass.MatchString(line):
			name, kind = reJSClass.FindStringSubmatch(line)[1], KindClass
		case reJSFunc.MatchString(line):
			name, kind = reJSFunc.FindStringSubmatch(line)[1], KindFunc
		case reJSArrow.MatchString(line):
			name, kind = reJSArrow.FindStringSubmatch(line)[1], KindFunc
		default:
			continue
		}
		out = appendSym(out, name, kind, i+1, braceBlockEnd(lines, i))
	}
	return out
}

// braceBlockEnd devuelve la línea (1-based) donde se cierra el bloque de llaves abierto
// en/después de la línea start; si no hay llaves, el símbolo ocupa una sola línea.
func braceBlockEnd(lines []string, start int) int {
	depth, seen := 0, false
	for i := start; i < len(lines); i++ {
		for _, r := range lines[i] {
			switch r {
			case '{':
				depth++
				seen = true
			case '}':
				depth--
			}
		}
		if seen && depth <= 0 {
			return i + 1
		}
	}
	return start + 1
}

var (
	rePyDef   = regexp.MustCompile(`^(\s*)def\s+(\w+)`)
	rePyClass = regexp.MustCompile(`^(\s*)class\s+(\w+)`)
)

// extractPy reconoce def/class y estima EndLine por des-indentación: el bloque termina
// en la última línea no vacía antes de una línea con indentación menor o igual a la del
// encabezado.
func extractPy(content string) []Symbol {
	lines := strings.Split(content, "\n")
	var out []Symbol
	for i, line := range lines {
		var indent, name, kind string
		switch {
		case rePyClass.MatchString(line):
			m := rePyClass.FindStringSubmatch(line)
			indent, name, kind = m[1], m[2], KindClass
		case rePyDef.MatchString(line):
			m := rePyDef.FindStringSubmatch(line)
			indent, name, kind = m[1], m[2], KindDef
		default:
			continue
		}
		out = appendSym(out, name, kind, i+1, pyBlockEnd(lines, i, len(indent)))
	}
	return out
}

// pyBlockEnd devuelve la última línea (1-based) del cuerpo de un símbolo Python cuyo
// encabezado está en la línea header con headerIndent espacios de sangría.
func pyBlockEnd(lines []string, header, headerIndent int) int {
	end := header + 1
	for i := header + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if leadingSpaces(lines[i]) <= headerIndent {
			break
		}
		end = i + 1
	}
	return end
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
		} else if r == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}

// FormatSymbols serializa símbolos al formato compacto de la memoria de código
// ("Load L10; Validate L14"), para el campo Symbols de code_memory.
func FormatSymbols(syms []Symbol) string {
	if len(syms) == 0 {
		return ""
	}
	parts := make([]string, 0, len(syms))
	for _, s := range syms {
		parts = append(parts, fmt.Sprintf("%s L%d", s.Name, s.StartLine))
	}
	return strings.Join(parts, "; ")
}

// appendSym agrega un símbolo saneando el rango (descarta los sin nombre o sin línea).
func appendSym(out []Symbol, name, kind string, start, end int) []Symbol {
	if name == "" || start <= 0 {
		return out
	}
	if end < start {
		end = start
	}
	return append(out, Symbol{Name: name, Kind: kind, StartLine: start, EndLine: end})
}
