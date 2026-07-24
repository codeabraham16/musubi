package codeintel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// graph.go DERIVA un grafo de código del AST de Go, model-free (Track 20 · F1). Su
// principio es el mismo que el resto de codeintel: DERIVAR del estado ACTUAL del archivo,
// nunca de datos guardados. Emite NODOS (archivos, símbolos, paquetes importados) y ARISTAS
// tipadas (CONTAINS archivo→símbolo, IMPORTS archivo→paquete, CALLS símbolo→símbolo), cada
// una con su confianza y con el archivo del que se derivó (SrcPath), para que la capa de
// persistencia pueda invalidar por fingerprint. Las aristas NUNCA las provee el llamador: se
// derivan. Solo Go en F1 (los demás lenguajes siguen solo-símbolos; sus aristas son F4).

// Kinds de NODO adicionales a los de símbolo (KindFunc/Method/Type/... en symbols.go).
const (
	KindFile    = "file"
	KindPackage = "package"
)

// Kinds de ARISTA.
const (
	EdgeImports  = "IMPORTS"
	EdgeContains = "CONTAINS"
	EdgeCalls    = "CALLS"
)

// Proveniencia de una arista. En F1 todo lo emitido es EXTRACTED (derivado del código, no
// inferido por un modelo ni provisto por el agente).
const ProvExtracted = "EXTRACTED"

// Node es un vértice del grafo con id estable y re-derivable (ver NodeKey/PackageKey/FileKey).
type Node struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"` // archivo origen ("" para paquete externo)
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	External  bool   `json:"external,omitempty"` // paquete fuera del módulo (stdlib/terceros)
}

// Edge es una arista dirigida y tipada. SrcPath es el archivo que la "posee": el refresco
// borra las aristas por SrcPath y las re-inserta, así el grafo nunca queda con aristas stale.
type Edge struct {
	FromKey    string  `json:"from_key"`
	ToKey      string  `json:"to_key"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
	Provenance string  `json:"provenance"`
	SrcPath    string  `json:"src_path"`
}

// PackageGraph es el resultado de derivar un paquete (directorio): nodos y aristas ya
// deduplicados y en orden determinista (para golden tests y salidas reproducibles).
type PackageGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Import es un import declarado en un archivo Go (Path canónico, Alias si se renombró).
type Import struct {
	Path  string `json:"path"`
	Alias string `json:"alias,omitempty"`
}

// FileKey es la clave estable de un nodo archivo (el propio path).
func FileKey(path string) string { return path }

// SymbolKey es la clave estable de un símbolo dentro de un archivo. `name` ya viene
// calificado por el llamador (p. ej. "Recv.Método" para métodos), de modo que un método y
// una función homónima nunca colisionan.
func SymbolKey(path, kind, name string) string {
	return path + "#" + kind + ":" + name
}

// PackageKey es la clave estable de un nodo paquete importado.
func PackageKey(importPath string) string { return "pkg:" + importPath }

// ExtractImports devuelve los imports declarados en un archivo `.go`. Degrada a lista vacía
// (sin error, sin pánico) si la extensión no es Go o el parseo falla del todo.
func ExtractImports(path, content string) []Import {
	if strings.ToLower(filepath.Ext(path)) != ".go" {
		return nil
	}
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, path, content, parser.SkipObjectResolution|parser.ImportsOnly)
	if file == nil {
		return nil
	}
	return importsOf(file)
}

// importsOf extrae los imports de un *ast.File ya parseado.
func importsOf(file *ast.File) []Import {
	var out []Import
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil || p == "" {
			continue
		}
		imp := Import{Path: p}
		if spec.Name != nil {
			imp.Alias = spec.Name.Name
		}
		out = append(out, imp)
	}
	return out
}

// DerivePackage deriva el grafo de un paquete Go: recibe el directorio, el mapa
// path→contenido de sus archivos, y el path del módulo (de go.mod) para clasificar imports
// in-module vs. externos. La unidad es el PAQUETE porque resolver CALLS intra-paquete exige
// la tabla de símbolos de todos sus archivos. Model-free: un solo parseo por archivo del que
// salen símbolos (con receiver), imports y call-sites. Los archivos no-Go se ignoran; un
// archivo que no parsea se degrada (parcial o vacío) sin pánico.
func DerivePackage(dir string, files map[string]string, modulePath string) PackageGraph {
	// Orden determinista de archivos para salida reproducible.
	paths := make([]string, 0, len(files))
	for p := range files {
		if strings.ToLower(filepath.Ext(p)) == ".go" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)

	nodes := map[string]Node{} // key → nodo (dedup: paquetes aparecen en varios archivos)
	var edges []Edge
	edgeSeen := map[string]bool{}
	pkgFuncs := map[string]string{}       // nombre de func top-level → SymbolKey (tabla del paquete)
	type callSite struct{ caller, callee, src string }
	var calls []callSite

	addNode := func(n Node) {
		if _, ok := nodes[n.Key]; !ok {
			nodes[n.Key] = n
		}
	}
	addEdge := func(e Edge) {
		id := e.Kind + "\x00" + e.FromKey + "\x00" + e.ToKey
		if edgeSeen[id] {
			return
		}
		edgeSeen[id] = true
		edges = append(edges, e)
	}

	// Pase 1: nodos (archivo, símbolos, paquetes), aristas CONTAINS e IMPORTS, y recolección
	// de la tabla de funcs del paquete + los call-sites (para resolver CALLS en el pase 2).
	for _, path := range paths {
		fset := token.NewFileSet()
		file, _ := parser.ParseFile(fset, path, files[path], parser.SkipObjectResolution)
		if file == nil {
			continue // no parseó ni parcialmente: degradación a nada para este archivo
		}
		lineOf := func(p token.Pos) int {
			if !p.IsValid() {
				return 0
			}
			return fset.Position(p).Line
		}

		fileKey := FileKey(path)
		addNode(Node{Key: fileKey, Kind: KindFile, Name: filepath.Base(path), Path: path})

		// IMPORTS.
		for _, imp := range importsOf(file) {
			pk := PackageKey(imp.Path)
			addNode(Node{Key: pk, Kind: KindPackage, Name: imp.Path, External: !inModule(imp.Path, modulePath)})
			addEdge(Edge{FromKey: fileKey, ToKey: pk, Kind: EdgeImports, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
		}

		// Símbolos top-level → nodos + CONTAINS. Los métodos se califican con su receiver.
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				kind := KindFunc
				qual := d.Name.Name
				if recv := receiverTypeName(d.Recv); recv != "" {
					kind = KindMethod
					qual = recv + "." + d.Name.Name
				}
				key := SymbolKey(path, kind, qual)
				addNode(Node{Key: key, Kind: kind, Name: qual, Path: path, StartLine: lineOf(d.Pos()), EndLine: lineOf(d.End())})
				addEdge(Edge{FromKey: fileKey, ToKey: key, Kind: EdgeContains, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
				if kind == KindFunc {
					// Solo las funcs top-level se llaman sin calificar dentro del paquete;
					// los métodos se invocan por selector (x.M) y se difieren.
					pkgFuncs[d.Name.Name] = key
				}
				if d.Body != nil {
					ast.Inspect(d.Body, func(n ast.Node) bool {
						if ce, ok := n.(*ast.CallExpr); ok {
							if id, ok := ce.Fun.(*ast.Ident); ok {
								calls = append(calls, callSite{caller: key, callee: id.Name, src: path})
							}
						}
						return true
					})
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						key := SymbolKey(path, KindType, s.Name.Name)
						addNode(Node{Key: key, Kind: KindType, Name: s.Name.Name, Path: path, StartLine: lineOf(d.Pos()), EndLine: lineOf(s.End())})
						addEdge(Edge{FromKey: fileKey, ToKey: key, Kind: EdgeContains, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
					case *ast.ValueSpec:
						kind := KindVar
						if d.Tok == token.CONST {
							kind = KindConst
						}
						for _, nm := range s.Names {
							if nm.Name == "_" {
								continue
							}
							key := SymbolKey(path, kind, nm.Name)
							addNode(Node{Key: key, Kind: kind, Name: nm.Name, Path: path, StartLine: lineOf(nm.Pos()), EndLine: lineOf(s.End())})
							addEdge(Edge{FromKey: fileKey, ToKey: key, Kind: EdgeContains, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
						}
					}
				}
			}
		}
	}

	// Pase 2: resolver CALLS contra la tabla de funcs del paquete (ya completa). Solo llamadas
	// sin calificar que matchean una func top-level única: confianza 1.0. Lo no resuelto se
	// OMITE (no se inventa). Cross-paquete precisas quedan diferidas (la dependencia ya vive
	// en IMPORTS).
	for _, cs := range calls {
		target, ok := pkgFuncs[cs.callee]
		if !ok {
			continue
		}
		addEdge(Edge{FromKey: cs.caller, ToKey: target, Kind: EdgeCalls, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: cs.src})
	}

	return PackageGraph{Nodes: sortedNodes(nodes), Edges: sortEdges(edges)}
}

// inModule indica si un import-path pertenece al módulo actual (in-project) y por lo tanto NO
// es externo. modulePath vacío ⇒ todo se considera externo (no rompemos, solo perdemos el flag).
func inModule(importPath, modulePath string) bool {
	if modulePath == "" {
		return false
	}
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

// receiverTypeName devuelve el nombre del TIPO receptor de un método (sin puntero ni
// parámetros de tipo), o "" si no hay receiver (es una función).
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	return exprBaseName(recv.List[0].Type)
}

// exprBaseName extrae el nombre base de un tipo receptor: *T→T, T[P]→T, pkg.T→T.
func exprBaseName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.StarExpr:
		return exprBaseName(x.X)
	case *ast.Ident:
		return x.Name
	case *ast.IndexExpr:
		return exprBaseName(x.X)
	case *ast.IndexListExpr:
		return exprBaseName(x.X)
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

// sortedNodes devuelve los nodos ordenados por Key (salida determinista).
func sortedNodes(m map[string]Node) []Node {
	out := make([]Node, 0, len(m))
	for _, n := range m {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// sortEdges ordena las aristas por (Kind, FromKey, ToKey) para salida determinista.
func sortEdges(e []Edge) []Edge {
	sort.Slice(e, func(i, j int) bool {
		if e[i].Kind != e[j].Kind {
			return e[i].Kind < e[j].Kind
		}
		if e[i].FromKey != e[j].FromKey {
			return e[i].FromKey < e[j].FromKey
		}
		return e[i].ToKey < e[j].ToKey
	})
	return e
}
