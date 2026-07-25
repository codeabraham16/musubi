package memory

import (
	"context"
	"math"
	"sort"
	"strings"
)

// codegraph_viz.go expone el GRAFO DE CÓDIGO (Track 20) como un grafo renderizable para la
// "lente código" del dashboard-cerebro — el análogo de braingraph.go para el mundo del código.
// Es read-only, model-free y de una sola pasada: vuelca los nodos top-N por CENTRALIDAD (grado)
// y las aristas cuyos dos extremos sobreviven al cap, scopeado por la credencial (a diferencia de
// BrainGraph, que no scopea: acá SÍ, para no cruzar tenants en el cerebro central). El módulo
// (paquete/dir) alimenta el color y el grado el tamaño; el porqué (EXPLICADO_POR) se resuelve
// aparte, on-demand, con ExplainedBy.

// CodeVizNode es un nodo del grafo de código proyectado para viz: sin cuerpos, con el módulo
// (color) y el grado/centralidad (tamaño) ya computados.
type CodeVizNode struct {
	Key      string `json:"key"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
	Module   string `json:"module"`
	Line     int    `json:"line,omitempty"`
	Degree   int    `json:"degree"`
	External bool   `json:"external,omitempty"`
}

// CodeVizEdge es una arista tipada (CALLS/IMPORTS/CONTAINS) entre dos nodos incluidos.
type CodeVizEdge struct {
	Source     string  `json:"source"`
	Target     string  `json:"target"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
}

// CodeGraphViz es el grafo de código para el render: nodos incluidos (top-N por grado), sus
// aristas (solo entre nodos incluidos) y el total real (para señalar truncamiento en la UI).
type CodeGraphViz struct {
	Nodes      []CodeVizNode `json:"nodes"`
	Edges      []CodeVizEdge `json:"edges"`
	TotalNodes int           `json:"total_nodes"`
	Truncated  bool          `json:"truncated"`
}

// defaultCodeVizLimit es el tope de nodos del render por defecto: denso pero sin ahogar el
// force-sim del navegador (O(n^2) por frame), mismo criterio que defaultBrainNeuronLimit.
const defaultCodeVizLimit = 400

// CodeExplain es una memoria (decisión/gotcha) que EXPLICA un símbolo de código (weld F3),
// derivada por FTS al vuelo — no es una arista persistida.
type CodeExplain struct {
	TopicKey string `json:"topic_key"`
	Gist     string `json:"gist"`
}

// CodeGraphViz arma el grafo de código renderizable. limit<=0 usa el default. Los nodos se
// ordenan por grado (callers+callees+imports+contains) desc y se capan a limit; las aristas se
// filtran a las que tienen ambos extremos entre los incluidos (sin colgantes), como brainSynapses.
func (e *DbEngine) CodeGraphViz(ctx context.Context, limit int) (CodeGraphViz, error) {
	if limit <= 0 {
		limit = defaultCodeVizLimit
	}
	nodes, err := e.listAllGraphNodes(ctx)
	if err != nil {
		return CodeGraphViz{}, err
	}
	edges, err := e.listAllGraphEdges(ctx)
	if err != nil {
		return CodeGraphViz{}, err
	}

	// Centralidad PONDERADA por nodo. CALLS y CONTAINS pesan pleno (estructura interna del
	// código); IMPORTS aporta poco (0.25) para que los paquetes externos (fmt, testing, strings),
	// que reciben cientos de imports, NO eclipsen a los símbolos propios del proyecto — el mismo
	// criterio que los god-nodes de musubi_map (grado CALLS). Se computa en Go, sin más SQL.
	deg := make(map[string]float64, len(nodes))
	for _, ed := range edges {
		w := 1.0
		if ed.Kind == "IMPORTS" {
			w = 0.25
		}
		deg[ed.FromKey] += w
		deg[ed.ToKey] += w
	}

	viz := make([]CodeVizNode, 0, len(nodes))
	for _, n := range nodes {
		viz = append(viz, CodeVizNode{
			Key: n.Key, Kind: n.Kind, Name: n.Name, Path: n.Path,
			Module: codeModuleOf(n), Line: n.StartLine, Degree: int(math.Round(deg[n.Key])), External: n.External,
		})
	}
	total := len(viz)
	sort.SliceStable(viz, func(i, j int) bool {
		if viz[i].Degree != viz[j].Degree {
			return viz[i].Degree > viz[j].Degree
		}
		return viz[i].Key < viz[j].Key
	})
	truncated := false
	if len(viz) > limit {
		viz = viz[:limit]
		truncated = true
	}

	included := make(map[string]bool, len(viz))
	for _, n := range viz {
		included[n.Key] = true
	}

	var out []CodeVizEdge
	seen := make(map[string]bool, len(edges))
	for _, ed := range edges {
		if !included[ed.FromKey] || !included[ed.ToKey] {
			continue
		}
		id := ed.Kind + "\x00" + ed.FromKey + "\x00" + ed.ToKey
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, CodeVizEdge{Source: ed.FromKey, Target: ed.ToKey, Kind: ed.Kind, Confidence: ed.Confidence})
	}

	if viz == nil {
		viz = []CodeVizNode{}
	}
	if out == nil {
		out = []CodeVizEdge{}
	}
	return CodeGraphViz{Nodes: viz, Edges: out, TotalNodes: total, Truncated: truncated}, nil
}

// codeModuleOf deriva el MÓDULO de un nodo para colorear: el directorio del paquete (parte del
// path hasta el último '/', o "." en la raíz) para los internos; los paquetes externos se agrupan
// bajo "(externo)" para no fragmentar la paleta en una entrada por dependencia.
func codeModuleOf(n GraphNode) string {
	if n.External || strings.HasPrefix(n.Key, "pkg:") {
		return "(externo)"
	}
	if n.Path == "" {
		return "."
	}
	if i := strings.LastIndexByte(n.Path, '/'); i >= 0 {
		return n.Path[:i]
	}
	return "."
}

// listAllGraphNodes vuelca TODOS los nodos del grafo de código, scopeado por la credencial (ctx).
// Es el análogo "grafo completo" que faltaba (las otras lecturas son per-símbolo/agregadas).
func (e *DbEngine) listAllGraphNodes(ctx context.Context) ([]GraphNode, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	q := `SELECT node_key, kind, name, path, start_line, end_line, external, COALESCE(src_fingerprint,'')
	      FROM code_graph_nodes WHERE 1=1` + clause + ` ORDER BY node_key`
	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GraphNode
	for rows.Next() {
		var n GraphNode
		var ext int
		if err := rows.Scan(&n.Key, &n.Kind, &n.Name, &n.Path, &n.StartLine, &n.EndLine, &ext, &n.SrcFingerprint); err != nil {
			return nil, err
		}
		n.External = ext != 0
		out = append(out, n)
	}
	return out, rows.Err()
}

// listAllGraphEdges vuelca TODAS las aristas del grafo de código, scopeado por la credencial (ctx).
func (e *DbEngine) listAllGraphEdges(ctx context.Context) ([]GraphEdge, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	q := `SELECT from_key, to_key, kind, COALESCE(confidence,0), COALESCE(provenance,''), src_path, COALESCE(src_fingerprint,'')
	      FROM code_graph_edges WHERE 1=1` + clause + ` ORDER BY kind, from_key, to_key`
	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GraphEdge
	for rows.Next() {
		var ed GraphEdge
		if err := rows.Scan(&ed.FromKey, &ed.ToKey, &ed.Kind, &ed.Confidence, &ed.Provenance, &ed.SrcPath, &ed.SrcFingerprint); err != nil {
			return nil, err
		}
		out = append(out, ed)
	}
	return out, rows.Err()
}

// ExplainedBy deriva el weld código→memoria de UN símbolo (Track 20 · F3): busca por FTS las
// decisiones/gotchas que mencionan su nombre o su archivo y devuelve sus topic_keys + un gist
// corto. On-demand (para el hover del dashboard); no persiste ninguna arista. Scopeado por ctx.
func (e *DbEngine) ExplainedBy(ctx context.Context, path, name string, limit int) ([]CodeExplain, error) {
	if limit <= 0 {
		limit = 5
	}
	var terms []string
	if strings.TrimSpace(name) != "" {
		terms = append(terms, name)
	}
	if p := strings.TrimSpace(path); p != "" {
		terms = append(terms, p)
		if i := strings.LastIndexByte(p, '/'); i >= 0 && i+1 < len(p) {
			terms = append(terms, p[i+1:]) // basename
		}
	}
	seen := map[string]bool{}
	out := []CodeExplain{}
	for _, term := range terms {
		obs, err := e.SearchObservationsFTS(ctx, term, limit)
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
			out = append(out, CodeExplain{TopicKey: ref, Gist: shortSnippet(o.Content, 90)})
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// shortSnippet recorta un texto a ~maxRunes runas para el tooltip (best-effort, sin cortar a la
// mitad de una palabra si puede evitarlo).
func shortSnippet(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	cut := string(r[:maxRunes])
	if i := strings.LastIndexByte(cut, ' '); i > maxRunes/2 {
		cut = cut[:i]
	}
	return cut + "…"
}
