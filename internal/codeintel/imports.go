package codeintel

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// Import es una dependencia declarada por un archivo, con la línea (1-based) donde aparece.
type Import struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

var (
	reJSImportFrom = regexp.MustCompile(`^\s*import\s+.*\bfrom\s+['"]([^'"]+)['"]`)
	reJSImportBare = regexp.MustCompile(`^\s*import\s+['"]([^'"]+)['"]`)
	reJSRequire    = regexp.MustCompile(`\brequire\(\s*['"]([^'"]+)['"]\s*\)`)
	rePyImport     = regexp.MustCompile(`^\s*(?:from\s+([\w.]+)\s+import|import\s+([\w.]+))`)
)

// ExtractImports devuelve los imports declarados en el contenido, derivados del estado
// actual. Misma política de degradación que ExtractSymbols: extensión no soportada o
// parseo fallido → lista vacía, sin pánico.
func ExtractImports(path, content string) []Import {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return goImports(content)
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return braceImports(content)
	case ".py":
		return pyImports(content)
	default:
		return nil
	}
}

func goImports(content string) []Import {
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, "", content, parser.ImportsOnly|parser.SkipObjectResolution)
	if file == nil {
		return nil
	}
	var out []Import
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		p := strings.Trim(imp.Path.Value, `"`)
		out = append(out, Import{Path: p, Line: fset.Position(imp.Pos()).Line})
	}
	return out
}

func braceImports(content string) []Import {
	var out []Import
	for i, line := range strings.Split(content, "\n") {
		switch {
		case reJSImportFrom.MatchString(line):
			out = append(out, Import{Path: reJSImportFrom.FindStringSubmatch(line)[1], Line: i + 1})
		case reJSImportBare.MatchString(line):
			out = append(out, Import{Path: reJSImportBare.FindStringSubmatch(line)[1], Line: i + 1})
		default:
			if m := reJSRequire.FindStringSubmatch(line); m != nil {
				out = append(out, Import{Path: m[1], Line: i + 1})
			}
		}
	}
	return out
}

func pyImports(content string) []Import {
	var out []Import
	for i, line := range strings.Split(content, "\n") {
		if m := rePyImport.FindStringSubmatch(line); m != nil {
			p := m[1]
			if p == "" {
				p = m[2]
			}
			out = append(out, Import{Path: p, Line: i + 1})
		}
	}
	return out
}
