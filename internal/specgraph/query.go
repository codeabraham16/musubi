// query.go porta core/query.ts: grepSpecs (búsqueda paginada con filtros de
// frontmatter y presupuesto de bytes) y graphSlice (recorte del grafo en tres
// direcciones). Model-free: substring o regex, nada de semántica (eso es F3).

package specgraph

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DefaultGrepLimit es el tope de matches por defecto, igual que thinkrail.
const DefaultGrepLimit = 200

// SpecContentEntry es una entrada con su contenido crudo (para grep sobre el
// cuerpo, no sólo el frontmatter).
type SpecContentEntry struct {
	Path        string
	Content     string
	Frontmatter Frontmatter
}

// SpecFilters acota el grep por campos de frontmatter. "" = filtro ausente.
type SpecFilters struct {
	Type      string
	Tag       string
	Parent    string
	DependsOn string
}

// GrepOptions son los parámetros de grepSpecs. IgnoreCase nil ⇒ true (igual que
// el `ignoreCase ?? true` de thinkrail). Limit<=0 ⇒ DefaultGrepLimit.
// MaxBytes<=0 ⇒ sin tope de bytes.
type GrepOptions struct {
	SpecFilters
	Pattern    string
	Regex      bool
	IgnoreCase *bool
	Limit      int
	Offset     int
	MaxBytes   int
}

type GrepMatch struct {
	Path    string
	Line    int
	Snippet string
}

type GrepResult struct {
	Matches    []GrepMatch
	Truncated  bool
	NextOffset int
}

// FormatGrepMatch rinde un match como "ruta:línea: snippet". El tamaño de esta
// cadena +1 es lo que se contabiliza contra MaxBytes.
func FormatGrepMatch(m GrepMatch) string {
	return fmt.Sprintf("%s:%d: %s", m.Path, m.Line, m.Snippet)
}

func matchesFilters(fm Frontmatter, f SpecFilters) bool {
	if f.Type != "" {
		if t, _ := Scalar(fm, FieldType); t != f.Type {
			return false
		}
	}
	if f.Parent != "" {
		if p, _ := Scalar(fm, FieldParent); p != f.Parent {
			return false
		}
	}
	if f.Tag != "" && !contains(List(fm, FieldTags), f.Tag) {
		return false
	}
	if f.DependsOn != "" && !contains(List(fm, FieldDependsOn), f.DependsOn) {
		return false
	}
	return true
}

func buildMatcher(opts GrepOptions) (func(string) bool, error) {
	ignoreCase := opts.IgnoreCase == nil || *opts.IgnoreCase
	if opts.Regex {
		pat := opts.Pattern
		if ignoreCase {
			pat = "(?i)" + pat
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, fmt.Errorf("patrón regex inválido: %w", err)
		}
		return re.MatchString, nil
	}
	if ignoreCase {
		needle := strings.ToLower(opts.Pattern)
		return func(line string) bool { return strings.Contains(strings.ToLower(line), needle) }, nil
	}
	return func(line string) bool { return strings.Contains(line, opts.Pattern) }, nil
}

// GrepSpecs busca el patrón línea por línea sobre las entradas que pasan los
// filtros, con paginado (offset/limit) y presupuesto de bytes. Al truncar
// devuelve Truncated=true y NextOffset apuntando al PRÓXIMO match no emitido.
// Garantiza al menos un match aunque exceda MaxBytes por sí solo (si no, el
// paginado no avanzaría).
func GrepSpecs(entries []SpecContentEntry, opts GrepOptions) (GrepResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultGrepLimit
	}
	offset := opts.Offset
	if offset < 0 {
		return GrepResult{}, errors.New("invalid match offset")
	}
	matcher, err := buildMatcher(opts)
	if err != nil {
		return GrepResult{}, err
	}

	matches := make([]GrepMatch, 0)
	skipped := 0
	byteCount := 0
	for _, entry := range entries {
		if !matchesFilters(entry.Frontmatter, opts.SpecFilters) {
			continue
		}
		lines := strings.Split(entry.Content, "\n")
		for i, raw := range lines {
			if i == 0 {
				raw = strings.TrimPrefix(raw, bom)
			}
			line := strings.TrimSuffix(raw, "\r")
			if !matcher(line) {
				continue
			}
			if skipped < offset {
				skipped++
				continue
			}
			match := GrepMatch{Path: entry.Path, Line: i + 1, Snippet: strings.TrimSpace(line)}
			size := len(FormatGrepMatch(match)) + 1
			if len(matches) >= limit || (opts.MaxBytes > 0 && len(matches) > 0 && byteCount+size > opts.MaxBytes) {
				return GrepResult{Matches: matches, Truncated: true, NextOffset: offset + len(matches)}, nil
			}
			matches = append(matches, match)
			byteCount += size
		}
	}
	return GrepResult{Matches: matches, Truncated: false}, nil
}

// SliceDirections son las tres direcciones válidas de graphSlice.
var SliceDirections = []string{"subtree", "ancestors", "neighbors"}

type SliceOptions struct {
	Root      string
	Depth     int
	Direction string
	Edge      string
}

type GraphSliceResult struct {
	Root      string
	Direction string
	Nodes     []*SpecNode
	Edges     []SpecEdge
	Missing   []string
}

// GraphSlice recorta el grafo desde Root hasta Depth saltos en una dirección:
//   - subtree: hijos (aristas parent inversas).
//   - ancestors: padres (aristas parent hacia adelante).
//   - neighbors: vecinos por Edge (depends-on por defecto), hacia adelante e
//     inversa.
//
// Cada arista se emite una sola vez (dedup por from\x00kind\x00to). Missing son
// los targets referenciados que no existen como nodo.
func GraphSlice(graph *SpecGraph, opts SliceOptions) GraphSliceResult {
	depth := opts.Depth
	if depth <= 0 {
		depth = 1
	}

	// Conjuntos ordenados: un map de Go no preserva orden como el Set de JS, y
	// el orden de included decide el de Nodes (root primero, luego BFS).
	includedOrder := []string{opts.Root}
	included := map[string]bool{opts.Root: true}
	edges := make([]SpecEdge, 0)
	seenEdges := make(map[string]bool)
	missingOrder := make([]string, 0)
	missing := make(map[string]bool)

	type step struct {
		to   string
		edge SpecEdge
	}
	walk := func(next func(id string) []step) {
		frontier := []string{opts.Root}
		for d := 0; d < depth; d++ {
			nextFrontier := make([]string, 0)
			for _, id := range frontier {
				for _, s := range next(id) {
					key := s.edge.From + nulSep + s.edge.Kind + nulSep + s.edge.To
					if !seenEdges[key] {
						seenEdges[key] = true
						edges = append(edges, s.edge)
					}
					if _, ok := graph.Nodes[s.to]; !ok && !missing[s.to] {
						missing[s.to] = true
						missingOrder = append(missingOrder, s.to)
					}
					if !included[s.to] {
						included[s.to] = true
						includedOrder = append(includedOrder, s.to)
						nextFrontier = append(nextFrontier, s.to)
					}
				}
			}
			if len(nextFrontier) == 0 {
				break
			}
			frontier = nextFrontier
		}
	}

	switch opts.Direction {
	case "subtree":
		walk(func(id string) []step {
			out := make([]step, 0)
			for _, child := range graph.Reverse[FieldParent][id] {
				out = append(out, step{to: child, edge: SpecEdge{From: child, To: id, Kind: FieldParent}})
			}
			return out
		})
	case "ancestors":
		walk(func(id string) []step {
			out := make([]step, 0)
			for _, parent := range graph.Forward[FieldParent][id] {
				out = append(out, step{to: parent, edge: SpecEdge{From: id, To: parent, Kind: FieldParent}})
			}
			return out
		})
	default: // neighbors
		edge := opts.Edge
		if edge == "" {
			edge = FieldDependsOn
		}
		walk(func(id string) []step {
			out := make([]step, 0)
			for _, to := range graph.Forward[edge][id] {
				out = append(out, step{to: to, edge: SpecEdge{From: id, To: to, Kind: edge}})
			}
			for _, from := range graph.Reverse[edge][id] {
				out = append(out, step{to: from, edge: SpecEdge{From: from, To: id, Kind: edge}})
			}
			return out
		})
	}

	nodes := make([]*SpecNode, 0, len(includedOrder))
	for _, id := range includedOrder {
		if node, ok := graph.Nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return GraphSliceResult{
		Root:      opts.Root,
		Direction: opts.Direction,
		Nodes:     nodes,
		Edges:     edges,
		Missing:   missingOrder,
	}
}
