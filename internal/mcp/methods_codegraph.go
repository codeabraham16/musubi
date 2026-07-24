package mcp

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// methods_codegraph.go dispara el poblado del GRAFO DE CÓDIGO (Track 20 · F1). En F1 NO hay
// tool pública ni hook que responda consultas (eso es F2): el grafo se puebla como EFECTO del
// guardado de un gist de código `.go`, derivándolo del AST y persistiéndolo scopeado por la
// credencial. La derivación es model-free (internal/codeintel, sin fs/db); acá se aportan el
// filesystem, los fingerprints por archivo y la atribución por proyecto.

// moduleImportPath lee la línea `module X` del go.mod del proyecto (model-free). Sirve para
// clasificar imports in-module vs. externos. "" si no lo encuentra (se pierde el flag external,
// no rompe nada).
func (s *McpServer) moduleImportPath() string {
	data, err := os.ReadFile(filepath.Join(s.projectPath, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// refreshCodeGraphForPackage deriva el grafo del paquete (directorio relativo a la raíz, con
// separadores "/") y lo persiste, atribuido al proyecto de la credencial. Es best-effort: si el
// directorio no existe o nada deriva, devuelve nil/err sin efectos, y el llamador (save_code) no
// debe fallar por esto. Deriva del contenido ACTUAL de cada archivo (derivar-no-desfasar) y
// estampa el fingerprint de ese mismo snapshot en cada fila.
func (s *McpServer) refreshCodeGraphForPackage(ctx context.Context, dir string) error {
	absDir := dir
	if !filepath.IsAbs(absDir) {
		absDir = filepath.Join(s.projectPath, dir)
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}

	files := map[string]string{}
	fps := map[string]string{}
	var fileKeys []string
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") {
			continue
		}
		rel := filepath.Join(dir, ent.Name())
		key := memory.NormalizeCodePath(s.projectPath, filepath.Join(s.projectPath, rel))
		content, rerr := s.readProjectFile(rel)
		if rerr != nil {
			continue
		}
		files[key] = content
		if fp, ferr := memory.FileFingerprint(s.projectPath, rel); ferr == nil {
			fps[key] = fp
		}
		fileKeys = append(fileKeys, key)
	}
	if len(files) == 0 {
		return nil
	}

	g := codeintel.DerivePackage(dir, files, s.moduleImportPath())

	nodes := make([]memory.GraphNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, memory.GraphNode{
			Key: n.Key, Kind: n.Kind, Name: n.Name, Path: n.Path,
			StartLine: n.StartLine, EndLine: n.EndLine, External: n.External,
			SrcFingerprint: fps[n.Path],
		})
	}
	edges := make([]memory.GraphEdge, 0, len(g.Edges))
	for _, ed := range g.Edges {
		edges = append(edges, memory.GraphEdge{
			FromKey: ed.FromKey, ToKey: ed.ToKey, Kind: ed.Kind,
			Confidence: ed.Confidence, Provenance: ed.Provenance,
			SrcPath: ed.SrcPath, SrcFingerprint: fps[ed.SrcPath],
		})
	}

	// Atribución por credencial (como save_code): sin proyecto atribuible no persistimos, para
	// no dejar filas sin atribuir que verían todos los tenants.
	origin, ok := writeOriginFor(principalFrom(ctx), "")
	if !ok {
		return nil
	}
	return s.engine.UpsertPackageGraphFrom(origin, fileKeys, nodes, edges)
}

// packageDirOf devuelve el directorio del paquete de una clave de path normalizada (con "/").
// Sin "/" ⇒ "." (archivo en la raíz).
func packageDirOf(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[:i]
	}
	return "."
}

// ---- F2: índice de repo + tools de consulta (Track 20 · F2-A) ----

// cgNodeView es la vista COMPACTA de un nodo para las respuestas de consulta (sin cuerpos):
// la palanca de tokens es navegar estructura sin leer archivos.
type cgNodeView struct {
	Key   string `json:"key"`
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"`
	Path  string `json:"path,omitempty"`
	Line  int    `json:"line,omitempty"`
	Stale bool   `json:"stale,omitempty"`
}

// cgStale compara el fingerprint guardado de un nodo con el ACTUAL del archivo (en la capa MCP,
// que tiene fs — como gistStale). Los nodos sin archivo (paquetes externos) nunca son stale.
func (s *McpServer) cgStale(n memory.GraphNode) bool {
	if n.Path == "" {
		return false
	}
	cur, err := memory.FileFingerprint(s.projectPath, n.Path)
	return err == nil && cur != "" && cur != n.SrcFingerprint
}

func (s *McpServer) cgView(n memory.GraphNode) cgNodeView {
	return cgNodeView{Key: n.Key, Kind: n.Kind, Name: n.Name, Path: n.Path, Line: n.StartLine, Stale: s.cgStale(n)}
}

// indexAllPackages recorre el proyecto (WalkDir desde projectPath), junta los directorios con
// archivos .go y refresca el grafo de cada uno, poblando el repo ENTERO. Salta directorios
// ocultos (.git/.musubi), vendor, testdata y node_modules. Devuelve un resumen.
func (s *McpServer) indexAllPackages(ctx context.Context) (map[string]interface{}, error) {
	dirs := map[string]bool{}
	_ = filepath.WalkDir(s.projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: saltar lo ilegible sin abortar el índice
		}
		if d.IsDir() {
			if p != s.projectPath {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") {
			if rel, rerr := filepath.Rel(s.projectPath, filepath.Dir(p)); rerr == nil {
				dirs[filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})

	pkgs := 0
	for dir := range dirs {
		if err := s.refreshCodeGraphForPackage(ctx, dir); err == nil {
			pkgs++
		}
	}
	scoped := s.scopedCtx(ctx)
	nodes, byKind, _ := s.engine.GraphStatsCtx(scoped)
	edges := 0
	for _, c := range byKind {
		edges += c
	}
	return map[string]interface{}{"packages": pkgs, "nodes": nodes, "edges": edges}, nil
}

func (s *McpServer) toolCodegraphIndex(ctx context.Context, _ json.RawMessage) (interface{}, *RpcError) {
	res, err := s.indexAllPackages(ctx)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al indexar el grafo de código: %v", err)
	}
	return jsonResult(res)
}

func (s *McpServer) toolCodeGraph(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Symbol string `json:"symbol"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	scoped := s.scopedCtx(ctx)

	// Modo símbolo: node_key completo (path#kind:name) → nodo + callees/callers/imports.
	if strings.TrimSpace(args.Symbol) != "" {
		n, ok, err := s.engine.GetGraphNodeCtx(scoped, args.Symbol)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al leer el nodo: %v", err)
		}
		if !ok {
			return jsonResult(map[string]interface{}{"found": false, "key": args.Symbol})
		}
		out, _ := s.engine.GraphOutEdgesCtx(scoped, args.Symbol)
		in, _ := s.engine.GraphInEdgesCtx(scoped, args.Symbol)
		callees, callers := []string{}, []string{}
		for _, e := range out {
			if e.Kind == codeintel.EdgeCalls {
				callees = append(callees, e.ToKey)
			}
		}
		for _, e := range in {
			if e.Kind == codeintel.EdgeCalls {
				callers = append(callers, e.FromKey)
			}
		}
		imports := []string{}
		if n.Path != "" {
			fe, _ := s.engine.GraphOutEdgesCtx(scoped, codeintel.FileKey(n.Path))
			for _, e := range fe {
				if e.Kind == codeintel.EdgeImports {
					imports = append(imports, e.ToKey)
				}
			}
		}
		return jsonResult(map[string]interface{}{
			"found": true, "node": s.cgView(n),
			"callees": callees, "callers": callers, "imports": imports,
		})
	}

	// Modo archivo: path → símbolos contenidos + imports del archivo.
	if strings.TrimSpace(args.Path) != "" {
		key := memory.NormalizeCodePath(s.projectPath, args.Path)
		syms, err := s.engine.ListGraphNodesForFileCtx(scoped, key)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al listar símbolos: %v", err)
		}
		views := make([]cgNodeView, 0, len(syms))
		for _, sy := range syms {
			views = append(views, s.cgView(sy))
		}
		fe, _ := s.engine.GraphOutEdgesCtx(scoped, codeintel.FileKey(key))
		imports := []string{}
		for _, e := range fe {
			if e.Kind == codeintel.EdgeImports {
				imports = append(imports, e.ToKey)
			}
		}
		return jsonResult(map[string]interface{}{"path": key, "symbols": views, "imports": imports})
	}

	return nil, rpcErrorf(codeInvalidParams, "se requiere 'symbol' (node_key path#kind:name) o 'path'")
}

func (s *McpServer) toolImpact(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Symbol   string `json:"symbol"`
		MaxDepth int    `json:"max_depth"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Symbol) == "" {
		return nil, rpcErrorf(codeInvalidParams, "se requiere 'symbol' (node_key)")
	}
	scoped := s.scopedCtx(ctx)
	callers, err := s.engine.GraphImpactCtx(scoped, args.Symbol, args.MaxDepth, 200)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al calcular el impacto: %v", err)
	}
	if callers == nil {
		callers = []string{}
	}
	return jsonResult(map[string]interface{}{"symbol": args.Symbol, "callers": callers, "count": len(callers)})
}

func (s *McpServer) toolMap(ctx context.Context, _ json.RawMessage) (interface{}, *RpcError) {
	scoped := s.scopedCtx(ctx)
	nodes, byKind, err := s.engine.GraphStatsCtx(scoped)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer estadísticas: %v", err)
	}
	god, _ := s.engine.GraphTopByDegreeCtx(scoped, 10)
	if god == nil {
		god = []memory.GraphDegree{}
	}
	entry, _ := s.engine.GraphEntryPointsCtx(scoped, 25)
	if entry == nil {
		entry = []string{}
	}
	return jsonResult(map[string]interface{}{
		"nodes": nodes, "edges": byKind, "god_nodes": god, "entry_points": entry,
	})
}
