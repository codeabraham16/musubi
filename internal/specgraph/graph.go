// graph.go porta core/graph.ts: a partir de las specs detectadas construye el
// grafo — árbol por `parent` + DAG por `depends-on`/`references`/`implements`,
// con adyacencia hacia adelante e inversa, y detección de `id` duplicados.
// La identidad de un nodo es su `id`, no su ruta.

package specgraph

// SpecNode es una spec en el grafo. Title queda "" si la spec no lo declara
// (Scalar nunca devuelve "" para una clave presente no vacía).
type SpecNode struct {
	ID          string
	Type        string
	Title       string
	Path        string
	Frontmatter Frontmatter
}

// SpecEdge es una arista dirigida from→to de una clase de enlace.
type SpecEdge struct {
	From string
	To   string
	Kind string
}

// SpecFileEntry es la entrada cruda al grafo: una ruta y su frontmatter ya
// parseado. El índice (store.go) produce estas entradas.
type SpecFileEntry struct {
	Path        string
	Frontmatter Frontmatter
}

// adjacency mapea clase-de-enlace → (id origen → ids destino).
type adjacency map[string]map[string][]string

// SpecGraph es el grafo derivado. NodeOrder preserva el orden de primer
// avistaje de cada id: un map de Go no tiene orden como el Map de JS que itera
// thinkrail, y sin ese orden las aristas y las listas de adyacencia saldrían en
// orden aleatorio (rompiendo tests y la salida de graph/validate).
type SpecGraph struct {
	Nodes        map[string]*SpecNode
	NodeOrder    []string
	Edges        []SpecEdge
	Forward      adjacency
	Reverse      adjacency
	DuplicateIds map[string][]string
}

func emptyAdjacency() adjacency {
	adj := make(adjacency, len(LinkKinds))
	for _, kind := range LinkKinds {
		adj[kind] = make(map[string][]string)
	}
	return adj
}

// linkTargets devuelve los targets de un nodo para una clase de enlace: 0/1
// para los campos de enlace único (parent), la lista completa para los de lista.
func linkTargets(fm Frontmatter, kind string) []string {
	if contains(SingleLinkFields, kind) {
		if target, ok := Scalar(fm, kind); ok {
			return []string{target}
		}
		return nil
	}
	return List(fm, kind)
}

func adjPush(m map[string][]string, key, value string) {
	m[key] = append(m[key], value)
}

// BuildGraph construye el grafo a partir de las entradas. Una entrada sin id o
// sin type se saltea (no es spec). Ante ids duplicados, el PRIMER archivo gana
// como nodo canónico; todas las rutas quedan registradas en DuplicateIds.
func BuildGraph(entries []SpecFileEntry) *SpecGraph {
	nodes := make(map[string]*SpecNode, len(entries))
	nodeOrder := make([]string, 0, len(entries))
	pathsByID := make(map[string][]string, len(entries))
	pathsOrder := make([]string, 0, len(entries))

	for _, entry := range entries {
		id, okID := Scalar(entry.Frontmatter, FieldID)
		typ, okType := Scalar(entry.Frontmatter, FieldType)
		if !okID || !okType {
			continue
		}
		if _, seen := pathsByID[id]; !seen {
			pathsOrder = append(pathsOrder, id)
		}
		pathsByID[id] = append(pathsByID[id], entry.Path)
		if _, exists := nodes[id]; !exists {
			title, _ := Scalar(entry.Frontmatter, FieldTitle)
			nodes[id] = &SpecNode{
				ID:          id,
				Type:        typ,
				Title:       title,
				Path:        entry.Path,
				Frontmatter: entry.Frontmatter,
			}
			nodeOrder = append(nodeOrder, id)
		}
	}

	forward := emptyAdjacency()
	reverse := emptyAdjacency()
	edges := make([]SpecEdge, 0)

	for _, id := range nodeOrder {
		node := nodes[id]
		for _, kind := range LinkKinds {
			for _, to := range linkTargets(node.Frontmatter, kind) {
				edges = append(edges, SpecEdge{From: node.ID, To: to, Kind: kind})
				adjPush(forward[kind], node.ID, to)
				adjPush(reverse[kind], to, node.ID)
			}
		}
	}

	duplicateIDs := make(map[string][]string)
	for _, id := range pathsOrder {
		if paths := pathsByID[id]; len(paths) > 1 {
			duplicateIDs[id] = paths
		}
	}

	return &SpecGraph{
		Nodes:        nodes,
		NodeOrder:    nodeOrder,
		Edges:        edges,
		Forward:      forward,
		Reverse:      reverse,
		DuplicateIds: duplicateIDs,
	}
}
