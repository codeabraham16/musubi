package mcp

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/codeintel"
	"musubi/internal/logx"
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
		if ent.IsDir() || !codeintel.IndexableForGraph(ent.Name()) {
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
// que tiene fs — como gistStale). Los nodos sin archivo (paquetes externos) nunca son stale. Un
// archivo AUSENTE o ilegible cuenta como stale (nodo fantasma): mostrar código borrado como
// fresco era un bug de correctitud — impact/callers apuntarían a símbolos que ya no existen.
func (s *McpServer) cgStale(n memory.GraphNode) bool {
	if n.Path == "" {
		return false
	}
	// En el central COMPARTIDO (forceRedact) el grafo es FEDERADO: los nodos vienen de otros proyectos y
	// sus archivos NO están en el disco del central, así que fingerprintear contra s.projectPath daría
	// "fantasma"/stale para TODO y la cabina mostraría el código federado como podrido (auditoría #3).
	// No se puede verificar frescura de un árbol remoto ⇒ no marcarlo stale.
	if s.forceRedact {
		return false
	}
	cur, err := memory.FileFingerprint(s.projectPath, n.Path)
	if err != nil || cur == "" {
		return true
	}
	return cur != n.SrcFingerprint
}

func (s *McpServer) cgView(n memory.GraphNode) cgNodeView {
	return cgNodeView{Key: n.Key, Kind: n.Kind, Name: n.Name, Path: n.Path, Line: n.StartLine, Stale: s.cgStale(n)}
}

// walkSourceTree recorre el proyecto UNA vez (WalkDir desde projectPath) y devuelve el set de
// directorios con archivos INDEXABLES (ver codeintel.IndexableForGraph: siempre .go, y TS/JS/Py sólo
// si el binario se compiló con -tags treesitter) y el set de file-keys normalizados (con "/") de cada
// uno en disco. Salta directorios ocultos (.git/.musubi), vendor, testdata, node_modules y salidas
// generadas (dist/coverage) — evita indexar bundles minificados. Es la fuente común del índice full
// (usa los dirs) y del incremental (compara los file-keys contra el grafo).
func (s *McpServer) walkSourceTree() (dirs map[string]bool, fileKeys map[string]bool) {
	dirs = map[string]bool{}
	fileKeys = map[string]bool{}
	_ = filepath.WalkDir(s.projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort: saltar lo ilegible sin abortar el índice
		}
		if d.IsDir() {
			if p != s.projectPath {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" ||
					name == "node_modules" || name == "dist" || name == "coverage" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if codeintel.IndexableForGraph(d.Name()) {
			if rel, rerr := filepath.Rel(s.projectPath, filepath.Dir(p)); rerr == nil {
				dirs[filepath.ToSlash(rel)] = true
			}
			fileKeys[memory.NormalizeCodePath(s.projectPath, p)] = true
		}
		return nil
	})
	return dirs, fileKeys
}

// dirExists indica si un directorio del paquete (relativo con "/", o absoluto) existe en disco.
func (s *McpServer) dirExists(dir string) bool {
	p := dir
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.projectPath, dir)
	}
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// graphSize devuelve el total de nodos y aristas del grafo scopeado (para el resumen del índice).
func (s *McpServer) graphSize(scoped context.Context) (nodes, edges int) {
	nodes, byKind, _ := s.engine.GraphStatsCtx(scoped)
	for _, c := range byKind {
		edges += c
	}
	return nodes, edges
}

// indexAllPackages puebla el repo ENTERO: refresca el grafo de cada directorio con .go. Devuelve
// un resumen {packages, nodes, edges}.
func (s *McpServer) indexAllPackages(ctx context.Context) (map[string]interface{}, error) {
	dirs, _ := s.walkSourceTree()
	pkgs := 0
	for dir := range dirs {
		if err := s.refreshCodeGraphForPackage(ctx, dir); err == nil {
			pkgs++
		}
	}
	nodes, edges := s.graphSize(s.scopedCtx(ctx))
	return map[string]interface{}{"packages": pkgs, "nodes": nodes, "edges": edges}, nil
}

// indexIncremental reconcilia el grafo con el working tree sin re-derivar todo (Track 20 · F5):
// compara el src_fingerprint guardado por archivo contra el actual del disco y sólo re-deriva los
// paquetes con archivos MODIFICADOS o NUEVOS, PODA los FANTASMA (en el grafo, ausentes en disco)
// y salta los sin cambio. El dir de un fantasma que aún existe se re-deriva para limpiar aristas
// colgantes que lo referenciaban. Devuelve {packages, pruned, skipped, nodes, edges}.
func (s *McpServer) indexIncremental(ctx context.Context) (map[string]interface{}, error) {
	scoped := s.scopedCtx(ctx)
	stored, err := s.engine.GraphFileFingerprintsCtx(scoped)
	if err != nil {
		return nil, err
	}
	_, diskFiles := s.walkSourceTree()

	dirtyDirs := map[string]bool{}
	var ghostPaths []string
	skipped := 0
	for path, fp := range stored {
		if !diskFiles[path] {
			ghostPaths = append(ghostPaths, path)
			if dir := packageDirOf(path); s.dirExists(dir) {
				dirtyDirs[dir] = true // re-derivar para soltar aristas que apuntaban al fantasma
			}
			continue
		}
		cur, ferr := memory.FileFingerprint(s.projectPath, path)
		if ferr == nil && cur == fp {
			skipped++
			continue
		}
		dirtyDirs[packageDirOf(path)] = true
	}
	for path := range diskFiles {
		if _, ok := stored[path]; !ok {
			dirtyDirs[packageDirOf(path)] = true // archivo nuevo
		}
	}

	pruned := 0
	if len(ghostPaths) > 0 {
		// Poda scopeada por credencial (como el refresh): sin proyecto atribuible, no borramos.
		if origin, ok := writeOriginFor(principalFrom(ctx), ""); ok {
			if n, perr := s.engine.PruneGraphFilesFrom(origin, ghostPaths); perr == nil {
				pruned = n
			}
		}
	}
	refreshed := 0
	for dir := range dirtyDirs {
		if err := s.refreshCodeGraphForPackage(ctx, dir); err == nil {
			refreshed++
		}
	}
	nodes, edges := s.graphSize(scoped)
	return map[string]interface{}{
		"mode": "incremental", "packages": refreshed, "pruned": pruned,
		"skipped": skipped, "nodes": nodes, "edges": edges,
	}, nil
}

func (s *McpServer) toolCodegraphIndex(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(raw, &args) // best-effort: sin/mal 'mode' ⇒ full (no rompe callers)
	var (
		res map[string]interface{}
		err error
	)
	if strings.EqualFold(strings.TrimSpace(args.Mode), "incremental") {
		res, err = s.indexIncremental(ctx)
	} else {
		res, err = s.indexAllPackages(ctx)
	}
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al indexar el grafo de código: %v", err)
	}
	// Federación (Track 20 · F6): tras indexar, empujar el grafo al central. BEST-EFFORT — si el
	// gate está apagado no hace nada, y un fallo del push jamás rompe el index (el grafo local ya
	// quedó bien). Sólo reportamos `federated` cuando de verdad se intentó (gate encendido).
	if attempted, ok := s.pushCodeGraphToCentral(ctx); attempted {
		res["federated"] = ok
	}
	return jsonResult(res)
}

// pushCodeGraphToCentral empuja el grafo local completo al cerebro central tras indexar (Track 20 ·
// F6), BEST-EFFORT. Gateado por sync configurado (s.syncClient) Y team mode (el proyecto participa
// del cerebro compartido) — igual que el drain entrante. Cualquier fallo se loguea y se traga: no
// rompe el index. Devuelve (attempted, ok): attempted=false cuando el gate está apagado (no-op sin
// red); ok=true sólo si el push se concretó.
func (s *McpServer) pushCodeGraphToCentral(ctx context.Context) (attempted, ok bool) {
	if s.syncClient == nil || !s.memory.TeamMode {
		return false, false // no-op: sin sync o proyecto local (R6/E4)
	}
	scoped := s.scopedCtx(ctx)
	nodes, err := s.engine.AllGraphNodesCtx(scoped)
	if err != nil {
		logx.Error("federación del grafo: no se pudieron leer los nodos locales", "error", err)
		return true, false
	}
	edges, err := s.engine.AllGraphEdgesCtx(scoped)
	if err != nil {
		logx.Error("federación del grafo: no se pudieron leer las aristas locales", "error", err)
		return true, false
	}
	// Los GISTS viajan con el grafo. Sin esto el central se quedaba con la estructura y sin los
	// titulares: 4.862 nodos federados contra 0 filas en code_memory, medido el 2026-08-12.
	//
	// Si leerlos falla se ABORTA el push entero, y es a propósito: el protocolo es de REEMPLAZO,
	// así que mandar la lista vacía no significa "no pude leerlos" sino "borrá todos los míos".
	// Ante una lectura fallida, no federar nada es lo único seguro — el grafo local ya quedó bien
	// y el próximo index reintenta.
	gists, err := s.engine.AllCodeMemoryCtx(scoped)
	if err != nil {
		logx.Error("federación del grafo: no se pudieron leer los gists locales (se aborta el push para no borrar los del central)", "error", err)
		return true, false
	}
	if err := s.syncClient.PushGraph(nodes, edges, gists); err != nil {
		logx.Error("federación del grafo: el push al central falló (best-effort, no rompe el index)", "error", err)
		return true, false
	}
	logx.Info("federación del grafo: empujado al central", "nodes", len(nodes), "edges", len(edges), "gists", len(gists))
	return true, true
}

// toolCodegraphPush RECIBE el grafo de código federado de un proyecto (Track 20 · F6): el daemon
// local, tras indexar, empuja su grafo entero y el central lo REEMPLAZA scopeado por el project_id
// del PRINCIPAL del token. La atribución la fija writeOriginFor —la misma guarda que save_observation—:
// un write=own IGNORA cualquier project_id del payload y usa el suyo (no puede plantar el grafo en
// otro tenant); sólo un write=any puede declarar otro proyecto. Es una tool WRITE (canCall exige
// autoridad de escritura). Model-free: sólo persiste lo derivado.
func (s *McpServer) toolCodegraphPush(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Nodes     []memory.GraphNode `json:"nodes"`
		Edges     []memory.GraphEdge `json:"edges"`
		ProjectID string             `json:"project_id"`
		// Gists es PUNTERO a propósito: acá la ausencia y el vacío significan cosas distintas.
		// nil ⇒ el emisor no habla de gists (un cliente viejo, anterior a que el push los
		// llevara) ⇒ no se toca lo que ya haya en el central. Lista vacía ⇒ el emisor SÍ habla
		// y dice que no tiene ninguno ⇒ se reemplaza por vacío. Sin esta distinción, un cliente
		// viejo empujando el mismo proyecto desde otra máquina le borraría los gists a uno nuevo.
		Gists *[]memory.CodeMemory `json:"gists"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	origin, ok := writeOriginFor(principalFrom(ctx), args.ProjectID)
	if !ok {
		return nil, rpcErrorf(codeUnauthorized, "no se pudo atribuir el grafo a un proyecto (declará project_id o usá una credencial con proyecto)")
	}
	if err := s.engine.ReplaceProjectGraphFrom(origin, args.Nodes, args.Edges); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al persistir el grafo federado: %v", err)
	}
	res := map[string]interface{}{"nodes": len(args.Nodes), "edges": len(args.Edges)}
	if args.Gists != nil {
		if err := s.engine.ReplaceProjectCodeMemoryFrom(origin, *args.Gists); err != nil {
			return nil, rpcErrorf(codeInternalError, "error al persistir los gists federados: %v", err)
		}
		res["gists"] = len(*args.Gists)
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

// graphFreshness cuenta, sobre los archivos presentes en el grafo scopeado, cuántos están STALE
// (fingerprint del disco distinto al guardado) y cuántos son FANTASMA (ausentes/ilegibles en
// disco). Es la señal de "conviene re-indexar" que expone map (Track 20 · F5), a granularidad de
// archivo (barata: una pasada de stat sobre los paths del grafo).
func (s *McpServer) graphFreshness(scoped context.Context) (stale, ghosts int) {
	// Igual que cgStale: en el central compartido el grafo es federado y sus archivos no están en disco,
	// así que la frescura por fingerprint no aplica (todo saldría fantasma). No inventar podredumbre. #3
	if s.forceRedact {
		return 0, 0
	}
	stored, err := s.engine.GraphFileFingerprintsCtx(scoped)
	if err != nil {
		return 0, 0
	}
	for path, fp := range stored {
		cur, ferr := memory.FileFingerprint(s.projectPath, path)
		if ferr != nil || cur == "" {
			ghosts++
			continue
		}
		if cur != fp {
			stale++
		}
	}
	return stale, ghosts
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
	stale, ghosts := s.graphFreshness(scoped)
	return jsonResult(map[string]interface{}{
		"nodes": nodes, "edges": byKind, "god_nodes": god, "entry_points": entry,
		"stale": stale, "ghosts": ghosts,
	})
}

// graphVizEngine es el sub-contrato para los grafos renderizables completos. Lo
// cumple *memory.DbEngine; se accede por type-assert para no ensuciar la interfaz
// StorageBackend ni sus fakes.
type graphVizEngine interface {
	BrainGraphCtx(ctx context.Context, limit int) (memory.BrainGraph, error)
	CodeGraphViz(ctx context.Context, limit int) (memory.CodeGraphViz, error)
}

func graphLimit(raw json.RawMessage, def int) int {
	if len(raw) > 0 {
		var args struct {
			Limit int `json:"limit"`
		}
		if json.Unmarshal(raw, &args) == nil && args.Limit > 0 {
			return args.Limit
		}
	}
	return def
}

// toolCodeGraphViz devuelve el grafo de código COMPLETO renderizable (nodos +
// aristas, top-N por grado) en una sola llamada — lo que dibuja el Grafo del
// cuerpo/dashboard. Scopeado por tenant (ctx). Read-only.
func (s *McpServer) toolCodeGraphViz(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	eng, ok := s.engine.(graphVizEngine)
	if !ok {
		return nil, rpcErrorf(codeInternalError, "este backend no expone el grafo renderizable")
	}
	g, err := eng.CodeGraphViz(s.scopedCtx(ctx), graphLimit(raw, 400))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "grafo de código: %v", err)
	}
	return jsonResult(g)
}

// toolBrainGraph devuelve el grafo de MEMORIA renderizable (neuronas + sinapsis,
// top-N por relevancia) en una sola llamada — el gemelo de memoria de
// toolCodeGraphViz. Read-only.
func (s *McpServer) toolBrainGraph(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	eng, ok := s.engine.(graphVizEngine)
	if !ok {
		return nil, rpcErrorf(codeInternalError, "este backend no expone el grafo renderizable")
	}
	g, err := eng.BrainGraphCtx(s.scopedCtx(ctx), graphLimit(raw, 300))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "grafo neuronal: %v", err)
	}
	return jsonResult(g)
}

// ---- F3: soldar el grafo de código a la memoria (Track 20 · F3) ----

// symbolNameFromKey parsea un node_key "path#kind:name" y devuelve (path, name). El name de un
// método viene calificado ("Recv.Metodo"). Si no tiene el separador, devuelve ("", key).
func symbolNameFromKey(key string) (path, name string) {
	i := strings.Index(key, "#")
	if i < 0 {
		return "", key
	}
	path = key[:i]
	rest := key[i+1:] // kind:name
	if j := strings.Index(rest, ":"); j >= 0 {
		return path, rest[j+1:]
	}
	return path, rest
}

// explainedBy DERIVA el weld código→memoria: busca en las observaciones (FTS, ya scopeada por la
// credencial) el nombre del símbolo y su archivo, y devuelve los topic_keys deduplicados de las
// decisiones/gotchas que lo mencionan. No persiste ninguna arista (respeta el invariante de F1:
// las aristas sólo se derivan). El agente expande el detalle con recall/memory_expand.
func (s *McpServer) explainedBy(ctx context.Context, path, name string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	terms := []string{}
	if strings.TrimSpace(name) != "" {
		terms = append(terms, name)
	}
	if strings.TrimSpace(path) != "" {
		terms = append(terms, path, filepath.Base(path))
	}
	seen := map[string]bool{}
	out := []string{}
	for _, term := range terms {
		obs, err := s.engine.SearchObservationsFTS(ctx, term, limit)
		if err != nil {
			continue
		}
		for _, o := range obs {
			ref := o.TopicKey
			if ref == "" {
				ref = o.ID
			}
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			out = append(out, ref)
		}
	}
	return out
}

// toolCodeContext es el PUENTE código↔memoria (Track 20 · F3): dado un símbolo, devuelve su
// estructura (nodo + callees/callers) Y las decisiones/gotchas que lo explican (`explained_by`,
// derivado por FTS). Es el análogo de musubi_entity_context para el mundo del código: responde
// "qué es esto, a qué llama, quién lo llama, y POR QUÉ es así / qué cuidado tiene".
func (s *McpServer) toolCodeContext(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Symbol string `json:"symbol"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Symbol) == "" {
		return nil, rpcErrorf(codeInvalidParams, "se requiere 'symbol' (node_key path#kind:name)")
	}
	scoped := s.scopedCtx(ctx)
	path, name := symbolNameFromKey(args.Symbol)

	resp := map[string]interface{}{"symbol": args.Symbol}
	n, found, err := s.engine.GetGraphNodeCtx(scoped, args.Symbol)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer el nodo: %v", err)
	}
	resp["found"] = found
	if found {
		resp["node"] = s.cgView(n)
		if n.Path != "" {
			path = n.Path
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
		resp["callees"] = callees
		resp["callers"] = callers
	}
	// El weld: decisiones/gotchas que explican este símbolo (derivado, scopeado).
	resp["explained_by"] = s.explainedBy(scoped, path, name, 5)
	return jsonResult(resp)
}
