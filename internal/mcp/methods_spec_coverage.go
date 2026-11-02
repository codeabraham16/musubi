// methods_spec_coverage.go es la 9ª tool del grafo de specs: musubi_spec_coverage, el cross-link
// spec↔código de F3-B (Track 20). Es la PRIMERA spec-tool que lee tablas scopeadas por tenant
// (code_graph_nodes/_edges vía los ...Ctx de s.engine), a diferencia de las otras ocho, que sólo
// tocan el filesystem. Por eso su aislamiento se prueba con un test dedicado (D8), no con el barrido
// genérico: el scope lo da s.scopedCtx(ctx), pero hay que PROBARLO —es la clase de fuga cross-tenant
// que el repo ya arregló tres veces (read_surface_class_test.go)—.
//
// Read-only y model-free: reusa la maquinaria existente (GraphImpactCtx de musubi_impact,
// ListGraphNodesForFileCtx, el índice de specs de F1) sin tabla nueva ni migración. Degrada SIN
// error (D3): grafo no indexado ⇒ aviso sin blast (R4); ancla rancia ⇒ marcada (R5); ancla que no
// resuelve a un nodo ⇒ no_resuelto (R6). Los tres estados son ortogonales y no se colapsan: hacerlo
// mentiría —"sin impacto" se leería igual tanto para lo que no tiene callers como para lo que el
// grafo no miró—.

package mcp

import (
	"context"
	"encoding/json"
	"path"
	"sort"
	"strings"

	"musubi/internal/specgraph"
)

// --- DTOs (tags json explícitos en camelCase, musubi-rules #6; listas make([]T,0,n), go-hygiene #6) ---

// specCoverageAnchorOut es una ancla de una spec resuelta (o no) contra el grafo de código.
type specCoverageAnchorOut struct {
	Ref         string   `json:"ref"`
	Path        string   `json:"path"`
	Symbol      string   `json:"symbol,omitempty"`
	Staleness   string   `json:"staleness"`  // none | changed | missing (R5)
	Resolution  string   `json:"resolution"` // resolved | no_resuelto | graph_not_indexed (R4/R6/D3)
	NodeKeys    []string `json:"nodeKeys"`
	BlastRadius []string `json:"blastRadius"`
	BlastCount  int      `json:"blastCount"`
}

// specCoverageForwardOut es la respuesta spec→código.
type specCoverageForwardOut struct {
	Direction   string                  `json:"direction"` // "spec_to_code"
	SpecID      string                  `json:"specId"`
	Anchors     []specCoverageAnchorOut `json:"anchors"`
	BlastRadius []string                `json:"blastRadius"` // unión dedup de todas las anclas, ordenada (D2)
	BlastCount  int                     `json:"blastCount"`
	Warning     string                  `json:"warning,omitempty"`
}

// specCoverageSpecOut es una spec que cubre el símbolo/módulo consultado.
type specCoverageSpecOut struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Title   string   `json:"title,omitempty"`
	Path    string   `json:"path"`
	Anchors []string `json:"anchors"`
}

// specCoverageInverseOut es la respuesta código→spec.
type specCoverageInverseOut struct {
	Direction string                `json:"direction"` // "code_to_spec"
	Symbol    string                `json:"symbol,omitempty"`
	Module    string                `json:"module,omitempty"`
	Specs     []specCoverageSpecOut `json:"specs"`
}

// toolSpecCoverage cruza las specs con el grafo de código en DOS direcciones. Exactamente UNA
// dirección por llamada (D4/R3): cero deja la intención sin definir, dos la vuelve ambigua, y un
// resultado parcial enseñaría a no confiar en el error.
func (s *McpServer) toolSpecCoverage(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		SpecID   string `json:"spec_id"`
		Symbol   string `json:"symbol"`
		Module   string `json:"module"`
		MaxDepth int    `json:"max_depth"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	specID := strings.TrimSpace(args.SpecID)
	symbol := strings.TrimSpace(args.Symbol)
	module := strings.TrimSpace(args.Module)

	dirs := 0
	for _, v := range []string{specID, symbol, module} {
		if v != "" {
			dirs++
		}
	}
	if dirs != 1 {
		return nil, rpcErrorf(codeInvalidParams,
			"exactamente una dirección por llamada: pasá spec_id (spec→código) O symbol/module (código→spec), no %d", dirs)
	}

	if specID != "" {
		return s.specCoverageForward(s.scopedCtx(ctx), specID, args.MaxDepth)
	}
	return s.specCoverageInverse(symbol, module)
}

// specCoverageForward resuelve las anclas de una spec contra el grafo de código scopeado y mide su
// blast radius (callers transitivos). Degradación tri-estado ortogonal (D3): grafo no indexado
// (GraphStatsCtx nodes==0) ⇒ graph_not_indexed + warning, sin blast (R4); grafo indexado pero el
// ancla no matchea ningún nodo ⇒ no_resuelto (R6); la ranciedad changed/missing viene de StalenessOf
// (disco vs baseline), independiente del grafo (R5).
func (s *McpServer) specCoverageForward(scoped context.Context, specID string, maxDepth int) (interface{}, *RpcError) {
	node, ok := s.specIndex.Graph().Nodes[specID]
	if !ok {
		// R7: un spec_id inexistente es un error de uso, nunca un resultado vacío que se lea "sin cobertura".
		return nil, rpcErrorf(codeInvalidParams, "no existe una spec con id %q", specID)
	}

	graphNodes, _, err := s.engine.GraphStatsCtx(scoped)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer el grafo de código: %v", err)
	}
	graphIndexed := graphNodes > 0

	// R5: la ranciedad surface-only de F2, indexada por ref. StalenessOf sólo devuelve las rancias;
	// su StaleAnchor.Path ES la ref del ancla.
	staleByRef := map[string]string{}
	for _, st := range specgraph.StalenessOf(s.projectPath, node.Frontmatter) {
		staleByRef[st.Path] = st.Reason
	}

	anchors := specgraph.Anchors(node.Frontmatter)
	out := specCoverageForwardOut{
		Direction: "spec_to_code",
		SpecID:    specID,
		Anchors:   make([]specCoverageAnchorOut, 0, len(anchors)),
	}
	blastUnion := map[string]bool{} // unión dedup a nivel spec (D2)

	for _, a := range anchors {
		ao := specCoverageAnchorOut{
			Ref:         a.Ref,
			Path:        a.Path,
			Symbol:      a.Symbol,
			Staleness:   "none",
			NodeKeys:    make([]string, 0),
			BlastRadius: make([]string, 0),
		}
		if r, isStale := staleByRef[a.Ref]; isStale {
			ao.Staleness = r // "changed" | "missing"
		}

		switch {
		case !graphIndexed:
			// R4: sin grafo no se resuelve ni se mide impacto; el ancla se reporta igual (con su staleness).
			ao.Resolution = "graph_not_indexed"
		default:
			// D2: el ancla no lleva kind, el node_key sí. En vez de adivinar el kind (falla para
			// métodos y homónimos cross-lenguaje), se listan los nodos reales del archivo y se filtra
			// por nombre. Símbolo "" (ancla de archivo entero) ⇒ TODOS los nodos del archivo.
			fileNodes, ferr := s.engine.ListGraphNodesForFileCtx(scoped, a.Path)
			if ferr != nil {
				return nil, rpcErrorf(codeInternalError, "error al resolver el ancla %q: %v", a.Ref, ferr)
			}
			matched := make([]string, 0, len(fileNodes))
			for _, fn := range fileNodes {
				if a.Symbol == "" || fn.Name == a.Symbol {
					matched = append(matched, fn.Key)
				}
			}
			if len(matched) == 0 {
				// R6: el archivo existe en disco pero no hay un nodo que matchee —archivo no indexado,
				// lenguaje sin extractor, o grafo viejo—. DISTINTO de graph_not_indexed y de missing.
				ao.Resolution = "no_resuelto"
				break
			}
			ao.Resolution = "resolved"
			sort.Strings(matched)
			ao.NodeKeys = matched
			// Blast radius del ancla = unión dedup de los callers transitivos de cada node_key homónimo.
			anchorBlast := map[string]bool{}
			for _, nk := range matched {
				callers, cerr := s.engine.GraphImpactCtx(scoped, nk, maxDepth, 200)
				if cerr != nil {
					return nil, rpcErrorf(codeInternalError, "error al calcular el impacto de %q: %v", nk, cerr)
				}
				for _, c := range callers {
					anchorBlast[c] = true
					blastUnion[c] = true
				}
			}
			ao.BlastRadius = sortedStringSet(anchorBlast) // D10: ordenado por node_key
			ao.BlastCount = len(ao.BlastRadius)
		}
		out.Anchors = append(out.Anchors, ao)
	}

	// R8: anclas por ref asc; unión de blast por node_key asc. Determinista y reproducible (D10),
	// a diferencia de musubi_impact, que hereda el orden de descubrimiento del BFS.
	sort.Slice(out.Anchors, func(i, j int) bool { return out.Anchors[i].Ref < out.Anchors[j].Ref })
	out.BlastRadius = sortedStringSet(blastUnion)
	out.BlastCount = len(out.BlastRadius)
	if !graphIndexed {
		out.Warning = "grafo no indexado; corré musubi_codegraph_index"
	}
	return jsonResult(out)
}

// specCoverageInverse devuelve las specs cuyas anclas cubren un símbolo o caen bajo un módulo.
// Lectura pura del índice de specs (FS-nativo de F1); no toca el grafo de código. Un match vacío NO
// es un error: es "sin cobertura", y la query echoed en la respuesta lo distingue de un error (R7).
func (s *McpServer) specCoverageInverse(symbol, module string) (interface{}, *RpcError) {
	// D5: symbol puede venir como `ruta#símbolo` (forma del ancla) o como un node_key
	// `path#kind:name` (lo que el usuario copia de impact/code_graph). symbolNameFromKey normaliza las
	// dos a (path, name): descarta el `kind:` cuando está; para `ruta#símbolo` el name ES el símbolo.
	var wantPath, wantSym string
	if symbol != "" {
		wantPath, wantSym = symbolNameFromKey(symbol)
		if wantPath == "" {
			// symbol sin '#': una ruta de archivo a secas (equivalente a un ancla de archivo entero).
			wantPath, wantSym = wantSym, ""
		}
	}

	out := specCoverageInverseOut{Direction: "code_to_spec", Specs: make([]specCoverageSpecOut, 0)}
	if symbol != "" {
		out.Symbol = symbol
	} else {
		out.Module = module
	}

	for _, node := range s.specIndex.Graph().Nodes {
		matched := make([]string, 0)
		for _, a := range specgraph.Anchors(node.Frontmatter) {
			hit := false
			if symbol != "" {
				// Matchea la ruta y, si se pidió un símbolo, también el símbolo. Un ancla de archivo
				// entero (Symbol=="") cubre cualquier símbolo de ese archivo.
				hit = a.Path == wantPath && (wantSym == "" || a.Symbol == "" || a.Symbol == wantSym)
			} else {
				hit = underModule(a.Path, module)
			}
			if hit {
				matched = append(matched, a.Ref)
			}
		}
		if len(matched) > 0 {
			sort.Strings(matched) // R8: anclas por ref asc
			out.Specs = append(out.Specs, specCoverageSpecOut{
				ID: node.ID, Type: node.Type, Title: node.Title, Path: node.Path, Anchors: matched,
			})
		}
	}
	// R8: specs por id asc (el mapa de nodos no tiene orden estable).
	sort.Slice(out.Specs, func(i, j int) bool { return out.Specs[i].ID < out.Specs[j].ID })
	return jsonResult(out)
}

// coverageOverlay computa la cobertura de specs por MÓDULO para el flag opt-in de musubi_map
// (R11/D7). El universo de módulos es el de walkSourceTree (los directorios de código del indexador,
// con sus mismas exclusiones). Un módulo está cubierto si alguna ancla de alguna spec apunta a un
// archivo DENTRO de él. Es ancla-based y no "hay un SPEC.md en el dir" (D7): Musubi centraliza sus
// specs en specs/<change>/, así que el modelo co-locado de thinkrail daría casi todo descubierto acá
// y sería inútil. Surface-only: sólo lee rutas, no muta nada (R12).
func (s *McpServer) coverageOverlay() map[string]interface{} {
	dirs, _, _ := s.walkSourceTree() // claves con "/", relativas a projectPath (filepath.ToSlash)

	// path.Dir usa "/" —igual que las claves de walkSourceTree— y un archivo en la raíz mapea a ".",
	// que es como el árbol clavea los .go de la raíz. Se marca sólo el dir inmediato del archivo: cada
	// paquete es su propio módulo, así que un ancla a internal/foo/sub/x.go cubre internal/foo/sub, no
	// internal/foo.
	covered := map[string]bool{}
	for _, node := range s.specIndex.Graph().Nodes {
		for _, a := range specgraph.Anchors(node.Frontmatter) {
			covered[path.Dir(a.Path)] = true
		}
	}

	withSpec := 0
	without := make([]string, 0)
	for d := range dirs {
		if covered[d] {
			withSpec++
		} else {
			without = append(without, d)
		}
	}
	sort.Strings(without) // determinista
	return map[string]interface{}{
		"modules_total": len(dirs),
		"with_spec":     withSpec,
		"without_spec":  without,
	}
}

// sortedStringSet devuelve las claves de un set como lista ordenada asc (nunca nil). Es el orden
// determinista que exige R8/D10: el blast radius se ordena por node_key para ser reproducible, a
// diferencia de musubi_impact, que hereda el orden de descubrimiento del BFS (no determinista entre
// corridas de SQLite, porque callersOfCtx no tiene ORDER BY).
func sortedStringSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// underModule indica si la ruta de un archivo cae bajo un módulo (prefijo de directorio). El módulo
// "internal/foo" cubre "internal/foo/bar.go" pero NO "internal/foobar/x.go": se compara contra el
// borde de segmento ("internal/foo/"), no un prefijo de string crudo.
func underModule(filePath, module string) bool {
	module = strings.TrimSuffix(strings.TrimSpace(module), "/")
	if module == "" {
		return false
	}
	return filePath == module || strings.HasPrefix(filePath, module+"/")
}
