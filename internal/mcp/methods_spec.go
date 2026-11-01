// methods_spec.go son las 8 tools MCP del grafo de specs (spec-graph F1 + recall
// F3-A). La capa de tools es de Musubi (frontera D1): el core portado vive en
// internal/specgraph y acá se re-escribe la capa que thinkrail tenía acoplada a
// pi/typebox, con LA convención de Musubi — schema JSON como el resto de
// registry.go, RpcError, candados de dispatch, y los 3 goldens.
//
// Clasificación (D3/D8): grep/get/graph/validate/recall son readOnly por su EFECTO
// sobre el repo (no mutan archivos), aunque por dentro toquen el cache del índice
// —ese cache lo protege el sync.Mutex interno del Index, no el candado del
// dispatch—. recall además embebe el query (I/O de red), por lo que es lockSelf;
// pero eso es concurrencia, no alcance: su EFECTO sobre el repo sigue siendo nulo
// y actuaAfuera() lo deriva false por readOnly (permisos.go sólo mira flota/
// credenciales, no el embebedor). create/update/delete son write (candado
// exclusivo). Las 8 actúan SÓLO sobre archivos locales (recall LEE specs del disco;
// el embebedor es opcional y no escribe) ⇒ actuaAfuera=false, fijado por el tripwire.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/specgraph"
)

// grepResponseByteBudget acota el texto de una página de grep aunque el limit no
// se alcance: 200 matches de líneas patológicamente largas podrían desbordar la
// respuesta. GrepSpecs garantiza ≥1 match aun si una sola línea excede esto, así
// que el paginado siempre avanza (no se cuelga en un match impagable).
const grepResponseByteBudget = 48_000

// specGrepLimitMax es el tope duro del parámetro limit (1..200, contrato de la
// tool). GrepSpecs ya trata limit<=0 como su default (200); acá sólo recortamos
// por arriba para que un pedido de 10_000 no intente formatear 10_000 líneas.
const specGrepLimitMax = 200

// --- DTOs de salida (json tags explícitos, musubi-rules #6) ---
//
// Los tipos del core (GrepMatch, DanglingLink, …) NO llevan json tags: son un
// port fiel y el lado TS no tiene equivalente. Para que la salida MCP use claves
// en minúscula —como todo el resto del catálogo— se mapean acá a DTOs tagueados,
// en vez de contaminar el core con tags de presentación (frontera D1).

type specForwardLinkOut struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Path    string `json:"path,omitempty"`
	Missing bool   `json:"missing,omitempty"`
}

type specReverseLinkOut struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	Path string `json:"path,omitempty"`
}

type specNodeOut struct {
	ID          string                `json:"id"`
	Type        string                `json:"type"`
	Title       string                `json:"title,omitempty"`
	Path        string                `json:"path"`
	Frontmatter specgraph.Frontmatter `json:"frontmatter"`
	Forward     []specForwardLinkOut  `json:"forward"`
	Reverse     []specReverseLinkOut  `json:"reverse"`
	// Stale (F2): anclas de esta spec cuyo código cambió o desapareció. omitempty ⇒ ausente
	// cuando no hay nada rancio (surface-only: la marca es un campo EXTRA, no cambia el nodo).
	// StaleWarning es el mismo dato en prosa, para que el agente lo lea sin parsear Stale.
	Stale        []specStaleAnchorOut `json:"stale,omitempty"`
	StaleWarning string               `json:"staleWarning,omitempty"`
}

type specDanglingOut struct {
	From     string `json:"from"`
	FromPath string `json:"fromPath"`
	Kind     string `json:"kind"`
	Target   string `json:"target"`
}

type specDuplicateOut struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
}

type specCycleOut struct {
	IDs []string `json:"ids"`
}

type specUnknownTypeOut struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type specRejectOut struct {
	Rel    string `json:"rel"`
	Reason string `json:"reason"`
}

// specStaleAnchorOut es la marca de rancedumbre de un ancla en la salida de get (por nodo, así que
// sin id). specStaleAnchorReport es lo mismo para validate, que recorre TODO el grafo y necesita
// el id de la spec dueña del ancla. Las dos espejan specgraph.StaleAnchor con tags en minúscula.
type specStaleAnchorOut struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type specStaleAnchorReport struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type specGrepOut struct {
	Matches    []string `json:"matches"`
	Truncated  bool     `json:"truncated"`
	NextOffset int      `json:"nextOffset,omitempty"`
}

// specHitOut es una spec rankeada por recall semántico. Espeja specgraph.SpecHit con tags en
// minúscula (musubi-rules #6). Score ∈ [0,1] sin importar el modo (coseno o keyword), así que el
// agente lo lee igual degrade o no.
type specHitOut struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	Title   string  `json:"title,omitempty"`
	Path    string  `json:"path"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet,omitempty"`
}

// specRecallOut espeja RecallResult (y specGrepOut): paginado estable con Truncated + NextOffset.
type specRecallOut struct {
	Hits       []specHitOut `json:"hits"`
	Truncated  bool         `json:"truncated"`
	NextOffset int          `json:"nextOffset,omitempty"`
}

type specValidateOut struct {
	Valid         bool                 `json:"valid"`
	DanglingLinks []specDanglingOut    `json:"danglingLinks"`
	DuplicateIDs  []specDuplicateOut   `json:"duplicateIds"`
	ParentCycles  []specCycleOut       `json:"parentCycles"`
	UnknownTypes  []specUnknownTypeOut `json:"unknownTypes"`
	RejectedSpecs []specRejectOut      `json:"rejectedSpecs"`
	// StaleAnchors (F2): anclas rancias de TODO el grafo, cada una con el id de su spec. Se
	// reporta en su propia sección y NO voltea `valid` (D8): `valid` es integridad ESTRUCTURAL
	// (dangling/duplicate/cycles/unknown-type/rejected); un ancla rancia es deriva de CÓDIGO, no
	// un defecto del grafo de specs. [] nunca null (go-hygiene #6).
	StaleAnchors []specStaleAnchorReport `json:"staleAnchors"`
}

// specGraphSliceOut y sus hijos taguean la salida de toolSpecGraph. GraphSliceResult,
// SpecNode y SpecEdge del core tampoco llevan json tags (mismo motivo: port fiel),
// así que marshalarlos directo emitiría {"Root":…,"Nodes":[{"ID":…}]} en PascalCase
// —el resto del catálogo MCP va en minúscula—. El nodo hijo es más chico que
// specNodeOut (sin forward/reverse): un recorte de grafo no trae los enlaces nodo a
// nodo, sólo las aristas agregadas.
type specGraphSliceOut struct {
	Root      string             `json:"root"`
	Direction string             `json:"direction"`
	Nodes     []specGraphNodeOut `json:"nodes"`
	Edges     []specGraphEdgeOut `json:"edges"`
	Missing   []string           `json:"missing"`
}

type specGraphNodeOut struct {
	ID          string                `json:"id"`
	Type        string                `json:"type"`
	Title       string                `json:"title,omitempty"`
	Path        string                `json:"path"`
	Frontmatter specgraph.Frontmatter `json:"frontmatter"`
}

type specGraphEdgeOut struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

func toSpecGraphSliceOut(r specgraph.GraphSliceResult) specGraphSliceOut {
	out := specGraphSliceOut{
		Root:      r.Root,
		Direction: r.Direction,
		Nodes:     make([]specGraphNodeOut, 0, len(r.Nodes)),
		Edges:     make([]specGraphEdgeOut, 0, len(r.Edges)),
		Missing:   append(make([]string, 0, len(r.Missing)), r.Missing...),
	}
	for _, n := range r.Nodes {
		out.Nodes = append(out.Nodes, specGraphNodeOut{
			ID: n.ID, Type: n.Type, Title: n.Title, Path: n.Path, Frontmatter: n.Frontmatter,
		})
	}
	for _, e := range r.Edges {
		out.Edges = append(out.Edges, specGraphEdgeOut{From: e.From, To: e.To, Kind: e.Kind})
	}
	return out
}

// --- Tools de lectura ---

// toolSpecGrep busca un patrón sobre el cuerpo de las specs, con filtros de
// frontmatter y paginado. Reusa el truncado de GrepSpecs (puede ser grande).
func (s *McpServer) toolSpecGrep(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Pattern    string `json:"pattern"`
		Regex      bool   `json:"regex"`
		IgnoreCase *bool  `json:"ignoreCase"`
		Type       string `json:"type"`
		Tag        string `json:"tag"`
		Parent     string `json:"parent"`
		DependsOn  string `json:"dependsOn"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.Pattern) == "" {
		return nil, rpcErrorf(codeInvalidParams, "pattern es obligatorio")
	}
	limit := args.Limit
	if limit > specGrepLimitMax {
		limit = specGrepLimitMax
	}
	result, err := specgraph.GrepSpecs(s.specIndex.ContentEntries(), specgraph.GrepOptions{
		SpecFilters: specgraph.SpecFilters{
			Type:      args.Type,
			Tag:       args.Tag,
			Parent:    args.Parent,
			DependsOn: args.DependsOn,
		},
		Pattern:    args.Pattern,
		Regex:      args.Regex,
		IgnoreCase: args.IgnoreCase,
		Limit:      limit,
		Offset:     args.Offset,
		MaxBytes:   grepResponseByteBudget,
	})
	if err != nil {
		// patrón regex inválido u offset negativo: es culpa del llamador.
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	matches := make([]string, 0, len(result.Matches))
	for _, m := range result.Matches {
		matches = append(matches, specgraph.FormatGrepMatch(m))
	}
	return jsonResult(specGrepOut{
		Matches:    matches,
		Truncated:  result.Truncated,
		NextOffset: result.NextOffset,
	})
}

// toolSpecRecall rankea las specs por SIGNIFICADO (coseno sobre embeddings del cuerpo), el
// complemento semántico del grep LITERAL. Es ctx-aware y lock=lockSelf: el embed del query es una
// llamada de RED (techo 30 s) y NO debe correr bajo el candado del despacho.
//
// Model-free con degradación (R5): el embebedor viaja como una CLAUSURA definida acá, no como una
// interfaz que specgraph llamaría. Es a propósito —y es lo que mantiene honesto al lockSelf—: la
// guarda candado_no_cruza_la_red reconoce el borde `Embed` por nombre simple y SÓLO dentro de
// internal/mcp. Con `s.embedder.Embed` LÉXICAMENTE en este handler, la guarda ve que llega al borde
// y valida el lockSelf; si el embed viviera en specgraph, el borde quedaría invisible y el lockSelf
// figuraría muerto. Sin embebedor la clausura es nil y Recall rankea TODO por keyword sin tocar red.
func (s *McpServer) toolSpecRecall(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query  string `json:"query"`
		Type   string `json:"type"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query es obligatorio")
	}
	limit := args.Limit
	if limit > specGrepLimitMax {
		limit = specGrepLimitMax
	}

	// El embebedor viaja como clausura, nil cuando no hay proveedor configurado (embed==nil ⇒ keyword
	// para todo, R5). El techo de 30 s cubre el embed del query Y los de cada spec: Recall threadea
	// este ctx a cada embed(ctx,·), así que un embebedor colgado no cuelga el servidor. NO hay
	// withReadLock: el handler sólo toca specIndex/specRecaller (cada uno con su propio mutex) y
	// campos inmutables post-construcción; nunca toca el motor (memory.db), D6.
	var embed specgraph.EmbedFunc
	if embedding.Enabled(s.embedder) {
		embCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		embed = func(ctx context.Context, text string) ([]float32, error) {
			return s.embedder.Embed(ctx, text)
		}
		ctx = embCtx
	}

	result, err := s.specRecaller.Recall(ctx, s.specIndex.ContentEntries(), specgraph.RecallOptions{
		Query:  args.Query,
		Type:   args.Type,
		Limit:  limit,
		Offset: args.Offset,
	}, embed)
	if err != nil {
		// El único error de Recall es offset negativo: culpa del llamador (espeja grep).
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	// El DTO con tags json se arma en toSpecRecallOut (misma frontera D1 que
	// toSpecGraphSliceOut): el core no tiene tags y sin el mapeo la salida saldría
	// en PascalCase. Tenerlo en un helper deja el sabotaje de R14 en una sola línea.
	return jsonResult(toSpecRecallOut(result))
}

// toSpecRecallOut mapea el resultado del core (sin json tags) al DTO de salida con
// claves en minúscula (musubi-rules #6). make([]…,0,…) y NO var: una página sin hits
// serializa `"hits":[]`, no `null` (go-hygiene #6) — un cliente que itera la lista no
// debe tragarse un null.
func toSpecRecallOut(result specgraph.RecallResult) specRecallOut {
	hits := make([]specHitOut, 0, len(result.Hits))
	for _, h := range result.Hits {
		hits = append(hits, specHitOut{
			ID: h.ID, Type: h.Type, Title: h.Title, Path: h.Path, Score: h.Score, Snippet: h.Snippet,
		})
	}
	return specRecallOut{
		Hits:       hits,
		Truncated:  result.Truncated,
		NextOffset: result.NextOffset,
	}
}

// toolSpecGet devuelve un nodo por id con sus enlaces hacia adelante (los que la
// spec declara) e inversos (quién la apunta). Sin cuerpo: para eso está grep.
func (s *McpServer) toolSpecGet(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID string `json:"id"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.ID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "id es obligatorio")
	}
	graph := s.specIndex.Graph()
	node, ok := graph.Nodes[args.ID]
	if !ok {
		return nil, rpcErrorf(codeInvalidParams, "no existe una spec con id %q", args.ID)
	}

	forward := make([]specForwardLinkOut, 0)
	reverse := make([]specReverseLinkOut, 0)
	for _, kind := range specgraph.LinkKinds {
		for _, target := range graph.Forward[kind][args.ID] {
			link := specForwardLinkOut{Kind: kind, Target: target}
			if dst, exists := graph.Nodes[target]; exists {
				link.Path = dst.Path
			} else {
				link.Missing = true
			}
			forward = append(forward, link)
		}
		for _, from := range graph.Reverse[kind][args.ID] {
			link := specReverseLinkOut{Kind: kind, From: from}
			// El origen de una arista inversa SIEMPRE es un nodo real (la
			// adyacencia inversa se puebla desde los campos de enlace de los
			// nodos), así que su path está garantizado.
			if src, exists := graph.Nodes[from]; exists {
				link.Path = src.Path
			}
			reverse = append(reverse, link)
		}
	}

	out := specNodeOut{
		ID:          node.ID,
		Type:        node.Type,
		Title:       node.Title,
		Path:        node.Path,
		Frontmatter: node.Frontmatter,
		Forward:     forward,
		Reverse:     reverse,
	}
	// Rancedumbre (R4/D5): re-deriva las anclas de esta spec desde el disco. Es surface-only —un
	// campo extra, nunca cambia el nodo (R5)— y para una spec sin anclas StalenessOf no lee nada.
	// Sin marcas, Stale/StaleWarning quedan ausentes (omitempty), así que el get de siempre no cambia.
	if stale := specgraph.StalenessOf(s.projectPath, node.Frontmatter); len(stale) > 0 {
		staleOut := make([]specStaleAnchorOut, 0, len(stale))
		for _, st := range stale {
			staleOut = append(staleOut, specStaleAnchorOut{Path: st.Path, Reason: st.Reason})
		}
		out.Stale = staleOut
		out.StaleWarning = specgraph.StaleWarning(stale)
	}
	return jsonResult(out)
}

// toolSpecGraph recorta el grafo desde un nodo raíz en una dirección
// (subtree/ancestors/neighbors) hasta cierta profundidad.
func (s *McpServer) toolSpecGraph(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Root      string `json:"root"`
		Direction string `json:"direction"`
		Depth     int    `json:"depth"`
		Edge      string `json:"edge"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.Root) == "" {
		return nil, rpcErrorf(codeInvalidParams, "root es obligatorio")
	}
	if !slices.Contains(specgraph.SliceDirections, args.Direction) {
		return nil, rpcErrorf(codeInvalidParams,
			"direction inválida %q: usá una de %s",
			args.Direction, strings.Join(specgraph.SliceDirections, ", "))
	}
	result := specgraph.GraphSlice(s.specIndex.Graph(), specgraph.SliceOptions{
		Root:      args.Root,
		Depth:     args.Depth,
		Direction: args.Direction,
		Edge:      args.Edge,
	})
	return jsonResult(toSpecGraphSliceOut(result))
}

// toolSpecValidate COMPONE (D9) los 3 chequeos fieles de thinkrail con los 2
// deltas de Musubi: type fuera del vocabulario, y archivos SPEC.md que no son
// spec. El core queda limpio; la política vive acá.
func (s *McpServer) toolSpecValidate(raw json.RawMessage) (interface{}, *RpcError) {
	graph := s.specIndex.Graph()
	report := specgraph.ValidateGraph(graph)
	unknown := specgraph.UnknownTypes(graph)
	rejected := s.specIndex.RejectedSpecs()

	out := specValidateOut{
		DanglingLinks: make([]specDanglingOut, 0, len(report.DanglingLinks)),
		DuplicateIDs:  make([]specDuplicateOut, 0, len(report.DuplicateIDs)),
		ParentCycles:  make([]specCycleOut, 0, len(report.ParentCycles)),
		UnknownTypes:  make([]specUnknownTypeOut, 0, len(unknown)),
		RejectedSpecs: make([]specRejectOut, 0, len(rejected)),
		StaleAnchors:  make([]specStaleAnchorReport, 0),
	}
	for _, d := range report.DanglingLinks {
		out.DanglingLinks = append(out.DanglingLinks, specDanglingOut{
			From: d.From, FromPath: d.FromPath, Kind: d.Kind, Target: d.Target,
		})
	}
	for _, d := range report.DuplicateIDs {
		out.DuplicateIDs = append(out.DuplicateIDs, specDuplicateOut{ID: d.ID, Paths: d.Paths})
	}
	for _, c := range report.ParentCycles {
		out.ParentCycles = append(out.ParentCycles, specCycleOut{IDs: c.IDs})
	}
	for _, u := range unknown {
		out.UnknownTypes = append(out.UnknownTypes, specUnknownTypeOut{ID: u.ID, Path: u.Path, Type: u.Type})
	}
	for _, r := range rejected {
		out.RejectedSpecs = append(out.RejectedSpecs, specRejectOut{Rel: r.Rel, Reason: r.Reason})
	}
	// Rancedumbre (R4/R8/D5/D8): se recorre el grafo y se re-deriva cada spec con anclas. Se
	// reporta en su propia sección; NO entra en `valid` (abajo), que sigue siendo integridad
	// estructural. graph.Nodes es un map (orden no determinista), así que se ordena por (id, path,
	// reason) para una salida estable y testeable.
	for _, node := range graph.Nodes {
		for _, st := range specgraph.StalenessOf(s.projectPath, node.Frontmatter) {
			out.StaleAnchors = append(out.StaleAnchors, specStaleAnchorReport{ID: node.ID, Path: st.Path, Reason: st.Reason})
		}
	}
	slices.SortFunc(out.StaleAnchors, func(a, b specStaleAnchorReport) int {
		if a.ID != b.ID {
			return strings.Compare(a.ID, b.ID)
		}
		if a.Path != b.Path {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Reason, b.Reason)
	})
	// Válido = los 3 chequeos del core limpios Y los 2 deltas de Musubi vacíos. La rancedumbre NO
	// cuenta (D8): un `validate` gateado en CI no debe llorar por deriva de código.
	out.Valid = specgraph.IsValid(report) && len(unknown) == 0 && len(rejected) == 0
	return jsonResult(out)
}

// --- Tools de escritura (candado exclusivo del dispatch) ---

// toolSpecCreate crea una spec nueva. Falla si la ruta no resuelve, el archivo ya
// existe, o el id ya está en uso. El cuerpo son headings por type; la identidad y
// el resto del frontmatter se arman en FieldOrder.
func (s *McpServer) toolSpecCreate(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Path       string   `json:"path"`
		ID         string   `json:"id"`
		Type       string   `json:"type"`
		Title      string   `json:"title"`
		Status     string   `json:"status"`
		Parent     string   `json:"parent"`
		DependsOn  []string `json:"dependsOn"`
		References []string `json:"references"`
		Implements []string `json:"implements"`
		Covers     []string `json:"covers"`
		Tags       []string `json:"tags"`
		Anchors    []string `json:"anchors"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.Path) == "" {
		return nil, rpcErrorf(codeInvalidParams, "path es obligatorio")
	}
	if strings.TrimSpace(args.ID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "id es obligatorio")
	}
	if strings.TrimSpace(args.Title) == "" {
		return nil, rpcErrorf(codeInvalidParams, "title es obligatorio")
	}
	if !slices.Contains(specgraph.SpecTypes, args.Type) {
		return nil, rpcErrorf(codeInvalidParams,
			"type inválido %q: usá uno de %s",
			args.Type, strings.Join(specgraph.SpecTypes, ", "))
	}

	rel, _, err := specgraph.ResolveSpecPath(s.projectPath, args.Path)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	// Toda operación de disco pasa por os.Root, que confina lectura y escritura a la raíz del
	// proyecto EN USE-TIME —no sólo en el resolve de ResolveSpecPath—. Cierra el TOCTOU por el que
	// un proceso co-residente que planta un junction entre el resolve y la escritura lograba que
	// create escribiera FUERA de la raíz (R11/R15; spec.md: "sin leer ni escribir fuera de la
	// raíz"). Medido: 133/2400 escapes con os.WriteFile directo, 0/2400 con os.Root.
	root, oErr := os.OpenRoot(s.projectPath)
	if oErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo abrir la raíz del proyecto: %v", oErr)
	}
	defer root.Close()
	// El archivo NO debe existir todavía (create, no overwrite). root.Stat no escapa la raíz.
	if _, serr := root.Stat(rel); serr == nil {
		return nil, rpcErrorf(codeInvalidParams, "ya existe un archivo en %s", rel)
	} else if !os.IsNotExist(serr) {
		return nil, rpcErrorf(codeInternalError, "no se pudo verificar %s: %v", rel, serr)
	}
	// El id tiene que ser único en el repo: dos specs con el mismo id colisionan
	// en el grafo (gana la primera del walk) y rompen la navegación.
	if existing, ok := s.specIndex.PathForID(args.ID); ok {
		return nil, rpcErrorf(codeInvalidParams, "el id %q ya está en uso en %s", args.ID, existing)
	}

	fm := specgraph.Frontmatter{
		specgraph.FieldID:    args.ID,
		specgraph.FieldType:  args.Type,
		specgraph.FieldTitle: args.Title,
	}
	if args.Status != "" {
		fm[specgraph.FieldStatus] = args.Status
	}
	if args.Parent != "" {
		fm[specgraph.FieldParent] = args.Parent
	}
	setListField(fm, specgraph.FieldDependsOn, args.DependsOn)
	setListField(fm, specgraph.FieldReferences, args.References)
	setListField(fm, specgraph.FieldImplements, args.Implements)
	setListField(fm, specgraph.FieldCovers, args.Covers)
	setListField(fm, specgraph.FieldTags, args.Tags)
	// anchors (F2): se validan y se les calcula la huella ANTES de escribir el archivo. Un ancla
	// que no resuelve (ruta inexistente, directorio, símbolo ausente, lenguaje sin extractor, o el
	// tope de 10) es un error de creación —no una spec que nacería con un ancla rota (R2)—.
	// BuildAnchorEntries lee archivos de CÓDIGO del disco (diagnóstico), no el SPEC.md que aún no existe.
	if len(args.Anchors) > 0 {
		entries, aErr := specgraph.BuildAnchorEntries(s.projectPath, args.Anchors)
		if aErr != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", aErr)
		}
		fm[specgraph.FieldAnchors] = entries
	}

	content := specgraph.SerializeFrontmatter(fm) + "\n" + specBodyForType(args.Type, args.Title)
	// Guarda de round-trip (R15): SerializeFrontmatter emite los escalares SIN
	// comillas, así que un id que YAML re-lee como otra cosa —"01"/"1.0" como el
	// número 1, "true" como bool, una fecha, "null"— se escribiría bien pero al
	// re-indexar el grafo vería un id distinto (o ninguno), y la compuerta de id
	// duplicado de arriba, que compara el string crudo, quedaría ciega. Antes de
	// escribir esa bomba se re-parsea el contenido y se exige que el id sobreviva.
	reparsed := specgraph.ParseFile(content)
	if gotID, ok := specgraph.Scalar(reparsed.Frontmatter, specgraph.FieldID); !ok || gotID != args.ID {
		return nil, rpcErrorf(codeInvalidParams,
			"el id %q no sobrevive el round-trip YAML (se re-parsea como %q): evitá ids que YAML lea como número, fecha, booleano o null",
			args.ID, gotID)
	}
	if mkErr := root.MkdirAll(filepath.Dir(rel), 0o755); mkErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo crear el directorio de %s: %v", rel, mkErr)
	}
	// O_EXCL ata la verificación de no-existencia a la escritura: aunque algo gane la carrera
	// entre el root.Stat de arriba y esta línea, el create atómico falla en vez de sobrescribir.
	f, wErr := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if wErr != nil {
		if os.IsExist(wErr) {
			return nil, rpcErrorf(codeInvalidParams, "ya existe un archivo en %s", rel)
		}
		return nil, rpcErrorf(codeInternalError, "no se pudo escribir %s: %v", rel, wErr)
	}
	if _, wErr := f.Write([]byte(content)); wErr != nil {
		f.Close()
		return nil, rpcErrorf(codeInternalError, "no se pudo escribir %s: %v", rel, wErr)
	}
	if cErr := f.Close(); cErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo cerrar %s: %v", rel, cErr)
	}
	return textResult(fmt.Sprintf("Created %s (%s)", rel, args.ID)), nil
}

// toolSpecUpdate aplica una edición lossless al frontmatter de una spec: la prosa
// queda byte a byte idéntica. Las 4 guardas (no renombrar id, no set de
// list-field, no remove de id/type, no des-espec-ear) las impone el core.
func (s *McpServer) toolSpecUpdate(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID         string              `json:"id"`
		Set        map[string]string   `json:"set"`
		Remove     []string            `json:"remove"`
		AddList    map[string][]string `json:"addList"`
		RemoveList map[string][]string `json:"removeList"`
		Anchors    []string            `json:"anchors"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.ID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "id es obligatorio")
	}
	record, ok := s.specIndex.RecordForID(args.ID)
	if !ok {
		return nil, rpcErrorf(codeInvalidParams, "no existe una spec con id %q", args.ID)
	}
	// Se relee del disco en vez de usar el contenido cacheado: garantiza editar
	// exactamente los bytes vigentes aunque el mtime no haya movido lo que el
	// scan revalida. El candado exclusivo del dispatch serializa las escrituras
	// ENTRE TOOLS; os.Root agrega la contención en use-time frente a un proceso
	// co-residente (mismo TOCTOU que create: record.Rel se resolvió en el scan).
	root, oErr := os.OpenRoot(s.projectPath)
	if oErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo abrir la raíz del proyecto: %v", oErr)
	}
	defer root.Close()
	original, rErr := root.ReadFile(record.Rel)
	if rErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo leer %s: %v", record.Rel, rErr)
	}
	// anchors (F2) se gestionan por una vía anchor-aware, NO por el addList genérico: sumarlas con
	// addList las metería sin digest y SIN validar existencia/símbolo/tope, violando R2. Para
	// AGREGAR o re-vouchear se usa el parámetro anchors (valida + calcula huella + mergea por ref);
	// para QUITAR, removeList con el string exacto `ruta#símbolo=digest` que muestran get/validate.
	if _, ok := args.AddList[specgraph.FieldAnchors]; ok {
		return nil, rpcErrorf(codeInvalidParams,
			"no agregues anchors con addList (quedarían sin digest ni validar): usá el parámetro anchors")
	}
	var setList map[string][]string
	if len(args.Anchors) > 0 {
		// El parámetro anchors re-vouchea SÓLO las refs que trae: se les calcula la huella fresca y
		// se mergean con las que ya tenía la spec (dedup por ref, digest nuevo gana). Un update que
		// NO pasa anchors (p. ej. set status) ni entra acá, así que jamás re-vouchea de rebote (D7).
		fresh, aErr := specgraph.BuildAnchorEntries(s.projectPath, args.Anchors)
		if aErr != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", aErr)
		}
		existing := specgraph.List(specgraph.ParseFile(string(original)).Frontmatter, specgraph.FieldAnchors)
		final, mErr := specgraph.MergeAnchorEntries(existing, fresh)
		if mErr != nil {
			return nil, rpcErrorf(codeInvalidParams, "%v", mErr)
		}
		setList = map[string][]string{specgraph.FieldAnchors: final}
	}
	updated, uErr := specgraph.UpdateFrontmatterText(string(original), specgraph.FrontmatterEdit{
		Set:        args.Set,
		Remove:     args.Remove,
		AddList:    args.AddList,
		RemoveList: args.RemoveList,
		SetList:    setList,
	})
	if uErr != nil {
		// Las guardas del core (renombrar id, set de list-field, remove de
		// id/type, des-espec-ear) son culpa del llamador.
		return nil, rpcErrorf(codeInvalidParams, "%v", uErr)
	}
	if wErr := root.WriteFile(record.Rel, []byte(updated), 0o644); wErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo escribir %s: %v", record.Rel, wErr)
	}
	return textResult(fmt.Sprintf("Updated frontmatter of %s", record.Rel)), nil
}

// toolSpecDelete borra una spec. Avisa de correr validate porque los enlaces que
// otras specs tenían hacia ésta quedan colgantes (F1 no los repara).
func (s *McpServer) toolSpecDelete(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID string `json:"id"`
	}
	if rerr := parseSpecArgs(raw, &args); rerr != nil {
		return nil, rerr
	}
	if strings.TrimSpace(args.ID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "id es obligatorio")
	}
	record, ok := s.specIndex.RecordForID(args.ID)
	if !ok {
		return nil, rpcErrorf(codeInvalidParams, "no existe una spec con id %q", args.ID)
	}
	// os.Root confina el borrado a la raíz en use-time: record.Abs se resolvió en el scan,
	// así que un junction plantado después no puede desviar el Remove fuera del proyecto.
	root, oErr := os.OpenRoot(s.projectPath)
	if oErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo abrir la raíz del proyecto: %v", oErr)
	}
	defer root.Close()
	if rErr := root.Remove(record.Rel); rErr != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo borrar %s: %v", record.Rel, rErr)
	}
	return textResult(fmt.Sprintf(
		"Deleted %s. Corré musubi_spec_validate para ver los enlaces que quedaron colgantes.",
		record.Rel)), nil
}

// --- Helpers ---

// parseSpecArgs decodifica los argumentos JSON de una tool de spec. Un raw vacío
// (sin argumentos) deja la estructura en su cero, que cada handler valida.
func parseSpecArgs(raw json.RawMessage, dst interface{}) *RpcError {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	return nil
}

// setListField agrega un campo lista al frontmatter sólo si trae items: una lista
// vacía no debe emitir una clave vacía (SerializeFrontmatter ya descarta vacíos,
// pero no agregar la clave es más explícito).
func setListField(fm specgraph.Frontmatter, field string, items []string) {
	if len(items) > 0 {
		fm[field] = items
	}
}

// specBodyForType arma el cuerpo inicial de una spec nueva: el título como H1 y
// un esqueleto de headings H2 propio del type. El agente llena las secciones; la
// prosa es suya desde el minuto cero (las tools nunca la vuelven a tocar).
func specBodyForType(specType, title string) string {
	var sections []string
	switch specType {
	case "goal-and-requirements":
		sections = []string{"Goals", "Requirements", "Non-goals"}
	case "architecture-design":
		sections = []string{"Context", "Components", "Decisions"}
	case "module-design":
		sections = []string{"Responsibility", "Interface", "Dependencies"}
	case "submodule-design":
		sections = []string{"Responsibility", "Interface"}
	case "task-spec":
		sections = []string{"Objective", "Acceptance criteria"}
	}
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(title)
	sb.WriteString("\n")
	for _, h := range sections {
		sb.WriteString("\n## ")
		sb.WriteString(h)
		sb.WriteString("\n")
	}
	return sb.String()
}

// specToolEntries son las 8 toolEntry del grafo de specs, en orden FIJO (lo
// congela TestToolsListGolden). grep/recall/get/graph/validate: readOnly (su
// efecto sobre el repo es nulo; el cache del índice lo protege su propio mutex,
// D3). create/update/delete: write (candado exclusivo). Casi todas dejan lock en
// su cero: no hay red ni LLM en el camino. La EXCEPCIÓN es recall, que embebe el
// query (llamada de red, techo 30 s) cuando hay embebedor: lleva lock=lockSelf
// —el despacho NO toma candado y el handler no sostiene ninguno durante el
// embed—, y la guarda candado_no_cruza_la_red lo exige y lo verifica.
func (s *McpServer) specToolEntries() []toolEntry {
	return []toolEntry{
		{
			Tool: Tool{
				Name:        "musubi_spec_grep",
				Description: "Busca un patrón (substring o regex) sobre el cuerpo de las specs del repo, con filtros de frontmatter (type/tag/parent/dependsOn) y paginado. Devuelve páginas 'path:line: snippet'. ignoreCase por defecto true. Model-free.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"pattern":    {Type: "string", Description: "El patrón a buscar. Substring por defecto; regex si regex=true."},
						"regex":      {Type: "boolean", Description: "Interpretar pattern como expresión regular (default false)."},
						"ignoreCase": {Type: "boolean", Description: "Ignorar mayúsculas/minúsculas (default true)."},
						"type":       {Type: "string", Description: "Filtrar por el campo type del frontmatter."},
						"tag":        {Type: "string", Description: "Filtrar por una tag presente en el frontmatter."},
						"parent":     {Type: "string", Description: "Filtrar por el campo parent del frontmatter."},
						"dependsOn":  {Type: "string", Description: "Filtrar por un target presente en depends-on."},
						"limit":      {Type: "integer", Description: "Máximo de matches por página (1..200, default 200)."},
						"offset":     {Type: "integer", Description: "Cuántos matches saltear para paginar (>=0). Usá el nextOffset devuelto."},
					},
					Required: []string{"pattern"},
				},
			},
			handler:  noCtx(s.toolSpecGrep),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_recall",
				Description: "Rankea las specs del repo por SIGNIFICADO (similitud semántica del cuerpo), el complemento del grep LITERAL de musubi_spec_grep: encontrá la spec que habla de un tema aunque no use esas palabras. Con proveedor de embeddings rankea por coseno; sin él —o si el embed falla— cae a un ranking por keyword determinista y NUNCA falla por eso (model-free). Filtro por type y paginado. Devuelve hits con id/type/title/path/score (0..1)/snippet, ordenados por score.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query":  {Type: "string", Description: "Texto de consulta. Se embebe y se rankea por similitud; sin embebedor se rankea por solapamiento de términos."},
						"type":   {Type: "string", Description: "Filtrar por el campo type del frontmatter (opcional)."},
						"limit":  {Type: "integer", Description: "Máximo de hits por página (1..200, default 20)."},
						"offset": {Type: "integer", Description: "Cuántos hits saltear para paginar (>=0). Usá el nextOffset devuelto."},
					},
					Required: []string{"query"},
				},
			},
			// handler DIRECTO (ctx-aware), no noCtx: necesita el ctx para el techo de 30 s del embed.
			handler:  s.toolSpecRecall,
			readOnly: true,
			// lockSelf: el handler embebe el query (red, 30 s) y no sostiene candado durante el embed.
			// readOnly a secas tomaría el candado COMPARTIDO del despacho, que bloquea a los escritores
			// (la captura de memoria escribe) durante esos 30 s. La guarda candado_no_cruza_la_red lo exige.
			lock: lockSelf,
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_get",
				Description: "Devuelve una spec por id: su frontmatter (id/type/title/path y demás campos) y sus enlaces hacia adelante (los que declara) e inversos (quién la apunta). Sin el cuerpo en prosa — para eso está musubi_spec_grep.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {Type: "string", Description: "El id de la spec a traer."},
					},
					Required: []string{"id"},
				},
			},
			handler:  noCtx(s.toolSpecGet),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_graph",
				Description: "Recorta el grafo de specs desde un nodo raíz: subtree (descendientes por parent), ancestors (ascendientes por parent) o neighbors (vecinos por una clase de enlace, depends-on por defecto). Devuelve nodes + edges + missing (targets referenciados que no existen).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"root":      {Type: "string", Description: "El id del nodo raíz del recorte."},
						"direction": {Type: "string", Description: "Dirección del recorte: subtree | ancestors | neighbors."},
						"depth":     {Type: "integer", Description: "Profundidad en saltos (default 1)."},
						"edge":      {Type: "string", Description: "Para neighbors: la clase de enlace a seguir (default depends-on)."},
					},
					Required: []string{"root", "direction"},
				},
			},
			handler:  noCtx(s.toolSpecGraph),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_validate",
				Description: "Valida el grafo de specs: enlaces colgantes, ids duplicados, ciclos de parent, type fuera del vocabulario, y archivos llamados SPEC.md que no son spec (id/type ausente o YAML malformado). Devuelve valid=true si no hay nada que reportar.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			handler:  noCtx(s.toolSpecValidate),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_create",
				Description: "Crea una spec nueva en path (root-relative, debe terminar en .md). Arma el frontmatter (id/type/title obligatorios) y un cuerpo con headings por type. Falla si el archivo ya existe, el id ya está en uso, o la ruta no resuelve dentro del repo.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"path":       {Type: "string", Description: "Ruta root-relative del archivo nuevo, terminada en .md (p. ej. internal/foo/SPEC.md)."},
						"id":         {Type: "string", Description: "Identidad de la spec, única en el repo."},
						"type":       {Type: "string", Description: "Uno de: goal-and-requirements, architecture-design, module-design, submodule-design, task-spec."},
						"title":      {Type: "string", Description: "Título legible de la spec."},
						"status":     {Type: "string", Description: "Estado opcional: draft | active | stale | done | deprecated."},
						"parent":     {Type: "string", Description: "id de la spec padre (árbol)."},
						"dependsOn":  {Type: "array", Description: "ids de specs de las que ésta depende.", Items: &Property{Type: "string"}},
						"references": {Type: "array", Description: "ids de specs referenciadas.", Items: &Property{Type: "string"}},
						"implements": {Type: "array", Description: "ids de specs que ésta implementa.", Items: &Property{Type: "string"}},
						"covers":     {Type: "array", Description: "rutas o símbolos que esta spec cubre (no son aristas).", Items: &Property{Type: "string"}},
						"tags":       {Type: "array", Description: "etiquetas libres (no son aristas).", Items: &Property{Type: "string"}},
						"anchors":    {Type: "array", Description: "Código del que habla la spec, como `ruta` o `ruta#símbolo` root-relative (p. ej. internal/foo/bar.go#Tipo.Metodo). Se valida que exista y se guarda una huella para avisar cuando el código cambie o desaparezca. Falla si la ruta o el símbolo no existen.", Items: &Property{Type: "string"}},
					},
					Required: []string{"path", "id", "type", "title"},
				},
			},
			handler: noCtx(s.toolSpecCreate),
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_update",
				Description: "Edita el frontmatter de una spec SIN tocar la prosa (lossless: comentarios, orden de claves, BOM y line-ending intactos). set (escalares), remove, addList y removeList (campos lista). No se puede renombrar el id, hacer set de un campo lista, quitar id/type, ni dejar el archivo sin id+type.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id":         {Type: "string", Description: "El id de la spec a editar."},
						"set":        {Type: "object", Description: "Campos escalares a fijar (clave→valor). No admite id ni campos lista."},
						"remove":     {Type: "array", Description: "Campos a borrar. No admite id ni type.", Items: &Property{Type: "string"}},
						"addList":    {Type: "object", Description: "Items a agregar a un campo lista (clave→items), con union-dedup. No admite anchors: usá el parámetro anchors."},
						"removeList": {Type: "object", Description: "Items a quitar de un campo lista (clave→items). Para quitar un ancla, pasá su string exacto `ruta#símbolo=digest`."},
						"anchors":    {Type: "array", Description: "Refs (`ruta` o `ruta#símbolo`) a anclar o re-vouchear: se validan, se les calcula la huella y se mergean por ref con las existentes (la nueva gana). No re-vouchea las que no nombres.", Items: &Property{Type: "string"}},
					},
					Required: []string{"id"},
				},
			},
			handler: noCtx(s.toolSpecUpdate),
		},
		{
			Tool: Tool{
				Name:        "musubi_spec_delete",
				Description: "Borra la spec con ese id. Avisa de correr musubi_spec_validate: los enlaces que otras specs tenían hacia ésta quedan colgantes (F1 no los repara).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {Type: "string", Description: "El id de la spec a borrar."},
					},
					Required: []string{"id"},
				},
			},
			handler: noCtx(s.toolSpecDelete),
		},
	}
}
