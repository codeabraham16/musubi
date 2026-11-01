// methods_spec.go son las 7 tools MCP del grafo de specs (spec-graph F1). La capa
// de tools es de Musubi (frontera D1): el core portado vive en internal/specgraph
// y acá se re-escribe la capa que thinkrail tenía acoplada a pi/typebox, con LA
// convención de Musubi — schema JSON como el resto de registry.go, RpcError,
// candados de dispatch, y los 3 goldens.
//
// Clasificación (D3/D8): grep/get/graph/validate son readOnly por su EFECTO sobre
// el repo (no mutan archivos), aunque por dentro toquen el cache del índice —ese
// cache lo protege el sync.Mutex interno del Index, no el candado del dispatch—.
// create/update/delete son write (candado exclusivo). Las 7 actúan SÓLO sobre
// archivos locales ⇒ actuaAfuera=false (permisos.go), fijado por el tripwire.

package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

type specGrepOut struct {
	Matches    []string `json:"matches"`
	Truncated  bool     `json:"truncated"`
	NextOffset int      `json:"nextOffset,omitempty"`
}

type specValidateOut struct {
	Valid         bool                 `json:"valid"`
	DanglingLinks []specDanglingOut    `json:"danglingLinks"`
	DuplicateIDs  []specDuplicateOut   `json:"duplicateIds"`
	ParentCycles  []specCycleOut       `json:"parentCycles"`
	UnknownTypes  []specUnknownTypeOut `json:"unknownTypes"`
	RejectedSpecs []specRejectOut      `json:"rejectedSpecs"`
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

	return jsonResult(specNodeOut{
		ID:          node.ID,
		Type:        node.Type,
		Title:       node.Title,
		Path:        node.Path,
		Frontmatter: node.Frontmatter,
		Forward:     forward,
		Reverse:     reverse,
	})
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
	// Válido = los 3 chequeos del core limpios Y los 2 deltas de Musubi vacíos.
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
	updated, uErr := specgraph.UpdateFrontmatterText(string(original), specgraph.FrontmatterEdit{
		Set:        args.Set,
		Remove:     args.Remove,
		AddList:    args.AddList,
		RemoveList: args.RemoveList,
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

// specToolEntries son las 7 toolEntry del grafo de specs, en orden FIJO (lo
// congela TestToolsListGolden). grep/get/graph/validate: readOnly (su efecto
// sobre el repo es nulo; el cache del índice lo protege su propio mutex, D3).
// create/update/delete: write (candado exclusivo). lock y ro quedan en su cero,
// que deriva del readOnly (sin lockSelf: no hay red ni LLM en el camino, D8).
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
						"addList":    {Type: "object", Description: "Items a agregar a un campo lista (clave→items), con union-dedup."},
						"removeList": {Type: "object", Description: "Items a quitar de un campo lista (clave→items)."},
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
