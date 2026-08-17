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

// ProvExtracted es la proveniencia de una arista derivada del código (no inferida por un modelo
// ni provista por el agente). En F1 todo lo emitido es EXTRACTED.
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

// PendingCall es un call-site CALIFICADO hacia otro paquete DEL MISMO MÓDULO (`alias.Func(...)`)
// que la derivación por paquete NO puede convertir en arista: el paquete destino está fuera de su
// alcance, así que no conoce el archivo —y por lo tanto la node_key— donde vive el símbolo.
// Se emite como DATO para que una fase posterior lo resuelva contra un SymbolIndex (ver crosspkg.go).
// Track 20 · F8-A.
// Un pendiente cubre DOS formas de call-site, y el campo Name las distingue:
//   - `alias.Func(...)`      → Name = "Func"
//   - `variable.Metodo(...)` → Name = "Tipo.Metodo", cuando el tipo de `variable` está DECLARADO
//     en la firma o en un `var` (ver tipoCalificado). Es la misma convención con la que el grafo
//     nombra a los métodos, así que el resolver los busca sin ningún caso especial.
type PendingCall struct {
	FromKey    string `json:"from_key"`    // node_key del símbolo que hace la llamada
	ImportPath string `json:"import_path"` // import path YA resuelto desde el alias del propio archivo
	Name       string `json:"name"`        // "Func" o "Tipo.Metodo", sin calificar por paquete
	SrcPath    string `json:"src_path"`    // archivo del CALLER: es quien va a poseer la arista
}

// PackageGraph es el resultado de derivar un paquete (directorio): nodos y aristas ya
// deduplicados y en orden determinista (para golden tests y salidas reproducibles).
// PendingCalls son las llamadas cross-paquete que quedaron SIN resolver a propósito: derivar un
// paquete no alcanza para resolverlas, y la alternativa —recibir un índice como parámetro— es
// imposible en un índice completo (haría falta derivar todo para poder construirlo).
type PackageGraph struct {
	Nodes        []Node        `json:"nodes"`
	Edges        []Edge        `json:"edges"`
	PendingCalls []PendingCall `json:"pending_calls,omitempty"`
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

// IndexableForGraph indica si un archivo debe FEEDEARSE a DerivePackage para poblar el grafo de
// código: siempre los `.go`, y —sólo cuando el binario se compiló con `-tags treesitter`— los
// lenguajes polyglot (TS/TSX/JS/JSX/Py). En el build por default `polyglotSupported` es false, así
// que esto equivale a "sólo .go" y el comportamiento histórico queda idéntico. Es el predicado que
// el indexador (capa MCP) usa para decidir qué archivos recolectar: model-free, sólo mira extensión.
func IndexableForGraph(path string) bool {
	if strings.ToLower(filepath.Ext(path)) == ".go" {
		return true
	}
	return polyglotSupported(path)
}

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
	pkgFuncs := map[string]string{}   // nombre de func top-level → SymbolKey (tabla del paquete)
	pkgMethods := map[string]string{} // "Receptor.Metodo" → SymbolKey (tabla de MÉTODOS del paquete)
	type callSite struct{ caller, callee, src string }
	var calls []callSite
	var methodCalls []callSite // callee ya calificado "Receptor.Metodo"; resuelve contra pkgMethods
	var pending []PendingCall  // cross-paquete in-module: se emiten sin resolver (ver PendingCall)
	pendingSeen := map[string]bool{}

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

		// IMPORTS. De paso se arma la tabla nombre-local→import-path de los imports IN-MODULE, que
		// es lo que después permite leer `alias.Func()` como una llamada cross-paquete. La tabla es
		// POR ARCHIVO a propósito: dos archivos del mismo paquete pueden aliasear distinto el mismo
		// import, y cada uno tiene que resolver por el suyo.
		inModImports := map[string]string{}
		for _, imp := range importsOf(file) {
			pk := PackageKey(imp.Path)
			external := !inModule(imp.Path, modulePath)
			addNode(Node{Key: pk, Kind: KindPackage, Name: imp.Path, External: external})
			addEdge(Edge{FromKey: fileKey, ToKey: pk, Kind: EdgeImports, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
			if external || imp.Alias == "_" || imp.Alias == "." {
				// Externos: fuera de alcance (su grafo no es nuestro). `_` no expone nombres y `.`
				// mete los símbolos sin calificar, que ya no es el caso que este pase resuelve.
				continue
			}
			local := imp.Alias
			if local == "" {
				// Sin alias, el nombre local es el del PAQUETE, que casi siempre coincide con el
				// último segmento del path. Si difiere, el peor caso es no emitir el pendiente —
				// una arista de menos, nunca una inventada.
				local = imp.Path[strings.LastIndex(imp.Path, "/")+1:]
			}
			inModImports[local] = imp.Path
		}

		// Símbolos top-level → nodos + CONTAINS. Los métodos se califican con su receiver.
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				kind := KindFunc
				qual := d.Name.Name
				recvType := receiverTypeName(d.Recv)
				if recvType != "" {
					kind = KindMethod
					qual = recvType + "." + d.Name.Name
				}
				key := SymbolKey(path, kind, qual)
				addNode(Node{Key: key, Kind: kind, Name: qual, Path: path, StartLine: lineOf(d.Pos()), EndLine: lineOf(d.End())})
				addEdge(Edge{FromKey: fileKey, ToKey: key, Kind: EdgeContains, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: path})
				if kind == KindFunc {
					// Las funcs top-level se llaman sin calificar dentro del paquete.
					pkgFuncs[d.Name.Name] = key
				} else {
					// Los métodos se invocan por selector. Se los indexa por "Receptor.Metodo" para
					// poder resolver `recv.Otro()` desde adentro del mismo tipo (Track 20 · F8-C).
					pkgMethods[qual] = key
				}
				// Nombre del receptor —la `s` de `(s *McpServer)`—, que es lo que permite leer
				// `s.Otro()` sin inferir tipos: adentro de este método, `s` ES de tipo recvType.
				recvName := receiverName(d.Recv)
				// Variables cuyo tipo está DECLARADO y es de otro paquete del módulo. Es lo que
				// permite leer `engine.Metodo()` sin inferir: el tipo de `engine` está escrito en
				// la firma. Ver tiposDeclarados.
				varTypes := tiposDeclarados(d, inModImports)
				if d.Body != nil {
					ast.Inspect(d.Body, func(n ast.Node) bool {
						ce, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}
						switch fun := ce.Fun.(type) {
						case *ast.Ident:
							// `Func(...)` — sin calificar: se resuelve en el pase 2 contra el paquete.
							calls = append(calls, callSite{caller: key, callee: fun.Name, src: path})
						case *ast.SelectorExpr:
							// `X.Algo(...)`. X tiene que ser un identificador PELADO: si es una
							// expresión (`s.campo.Metodo()`, `f().Metodo()`) saber a qué tipo
							// pertenece exige inferencia y queda fuera de alcance.
							base, ok := fun.X.(*ast.Ident)
							if !ok {
								return true
							}
							// EL RECEPTOR SE MIRA PRIMERO, y el orden no es capricho: dentro del
							// método el receptor SOMBREA a cualquier import homónimo, así que al
							// revés la llamada se le adjudicaría al paquete equivocado.
							if recvName != "" && base.Name == recvName {
								methodCalls = append(methodCalls, callSite{
									caller: key, callee: recvType + "." + fun.Sel.Name, src: path,
								})
								return true
							}
							// Por el MISMO motivo que el receptor va primero: una variable
							// declarada SOMBREA a un import homónimo dentro de la función. Al
							// revés, `engine.X()` con un paquete llamado `engine` en el árbol se
							// le adjudicaría al paquete y no al tipo.
							ip, name, ok := "", "", false
							if vt, hay := varTypes[base.Name]; hay {
								ip, name, ok = vt.importPath, vt.typeName+"."+fun.Sel.Name, true
							} else if p, hay := inModImports[base.Name]; hay {
								ip, name, ok = p, fun.Sel.Name, true
							}
							if !ok {
								return true
							}
							pc := PendingCall{FromKey: key, ImportPath: ip, Name: name, SrcPath: path}
							id := pc.FromKey + "\x00" + pc.ImportPath + "\x00" + pc.Name
							if !pendingSeen[id] {
								pendingSeen[id] = true
								pending = append(pending, pc)
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

	// Pase 2-bis: MÉTODOS sobre el propio receptor (Track 20 · F8-C). Hasta acá ningún método era
	// DESTINO de una llamada —medido: 2.537 aristas salían de funcs top-level y CERO llegaban a un
	// método—, así que el cierre transitivo de `impact` se cortaba en el primer envoltorio: un
	// `Envoltorio()` que sólo delega en `s.Interno()` no mostraba a quién le pega cambiar `Interno`.
	//
	// El callee ya viene calificado "Receptor.Metodo" del pase 1, y eso NO necesitó inferencia de
	// tipos: adentro de un método, el tipo del receptor está declarado en su propia firma.
	// Lo que sigue fuera de alcance es todo lo demás —`otraVar.Metodo()`, campos, valores de
	// retorno—, porque ahí sí hay que inferir. Igual que arriba: lo que no resuelve se OMITE.
	for _, mc := range methodCalls {
		target, ok := pkgMethods[mc.callee]
		if !ok {
			continue
		}
		addEdge(Edge{FromKey: mc.caller, ToKey: target, Kind: EdgeCalls, Confidence: 1.0, Provenance: ProvExtracted, SrcPath: mc.src})
	}

	// Pase polyglot (Track 20 · F4): TS/JS/Py vía tree-sitter. En el build por default
	// `polyglotSupported` devuelve false y esto es un NO-OP (los no-Go quedan solo-símbolos, sin
	// aristas, como hasta ahora); compilando con `-tags treesitter` se activa la derivación real.
	// Se mergea por los mismos addNode/addEdge (dedup) que el pase Go.
	var polyPaths []string
	for p := range files {
		if polyglotSupported(p) {
			polyPaths = append(polyPaths, p)
		}
	}
	sort.Strings(polyPaths)
	for _, path := range polyPaths {
		pn, pe := derivePolyglotFile(path, files[path])
		for _, n := range pn {
			addNode(n)
		}
		for _, e := range pe {
			addEdge(e)
		}
	}

	return PackageGraph{Nodes: sortedNodes(nodes), Edges: sortEdges(edges), PendingCalls: sortPendingCalls(pending)}
}

// sortPendingCalls ordena los pendientes por (FromKey, ImportPath, Name) para salida determinista,
// igual que sortedNodes/sortEdges: el orden de recorrido del AST no debe filtrarse al resultado.
func sortPendingCalls(p []PendingCall) []PendingCall {
	sort.Slice(p, func(i, j int) bool {
		if p[i].FromKey != p[j].FromKey {
			return p[i].FromKey < p[j].FromKey
		}
		if p[i].ImportPath != p[j].ImportPath {
			return p[i].ImportPath < p[j].ImportPath
		}
		return p[i].Name < p[j].Name
	})
	return p
}

// inModule indica si un import-path pertenece al módulo actual (in-project) y por lo tanto NO
// es externo. modulePath vacío ⇒ todo se considera externo (no rompemos, solo perdemos el flag).
func inModule(importPath, modulePath string) bool {
	if modulePath == "" {
		return false
	}
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}

// receiverName devuelve el NOMBRE de la variable receptora —la `s` de `(s *McpServer)`—, o "" si
// el método declara el receptor sin nombrarlo (`func (*T) M()`, legal cuando no se lo usa) o si no
// hay receptor. Con "" simplemente no se recolectan llamadas a métodos hermanos desde ahí: una
// arista de menos, nunca una inventada.
func receiverName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 || len(recv.List[0].Names) == 0 {
		return ""
	}
	n := recv.List[0].Names[0].Name
	if n == "_" {
		return "" // receptor descartado: no se puede llamar nada a través de él
	}
	return n
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
