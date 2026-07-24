package memory

import (
	"context"
	"fmt"
)

// codegraph_query.go añade las LECTURAS de consulta del grafo de código (Track 20 · F2):
// callers (aristas entrantes), impacto transitivo, estadísticas, god-nodes por grado, entry
// points y símbolos de un archivo. Todo scopeado por proyecto (patrón de F1) y model-free: el
// motor sólo recorre/cuenta; el juicio y la anotación de staleness viven en la capa MCP.

// GraphDegree es un nodo con su grado (para los god-nodes del panorama).
type GraphDegree struct {
	Key    string `json:"key"`
	Degree int    `json:"degree"`
}

// GraphInEdgesCtx devuelve las aristas ENTRANTES a un nodo (sus callers, con el kind), scopeadas
// al proyecto de la credencial. Simétrica de GraphOutEdgesCtx.
func (e *DbEngine) GraphInEdgesCtx(ctx context.Context, toKey string) ([]GraphEdge, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	q := `SELECT from_key, to_key, kind, confidence, provenance, src_path, src_fingerprint
	      FROM code_graph_edges WHERE to_key=?` + clause + ` ORDER BY kind, from_key`
	rows, err := e.db.QueryContext(ctx, q, append([]interface{}{toKey}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("error al leer aristas entrantes: %w", err)
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

// callersOfCtx devuelve los from_key de las aristas CALLS entrantes a toKey (quién lo llama
// directo), scopeado. Primitiva del impacto transitivo.
func (e *DbEngine) callersOfCtx(ctx context.Context, toKey string) ([]string, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	q := `SELECT DISTINCT from_key FROM code_graph_edges WHERE to_key=? AND kind='CALLS'` + clause
	rows, err := e.db.QueryContext(ctx, q, append([]interface{}{toKey}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// GraphImpactCtx devuelve el cierre transitivo de CALLERS de un nodo ("qué se rompe si cambio
// X") por BFS model-free sobre aristas CALLS entrantes, acotado por maxDepth y maxNodes para no
// explotar. No incluye el propio origen. El orden es de descubrimiento (más cercano primero).
func (e *DbEngine) GraphImpactCtx(ctx context.Context, key string, maxDepth, maxNodes int) ([]string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	if maxNodes <= 0 {
		maxNodes = 200
	}
	visited := map[string]bool{key: true}
	var order []string
	frontier := []string{key}
	for depth := 0; depth < maxDepth && len(frontier) > 0 && len(order) < maxNodes; depth++ {
		var next []string
		for _, k := range frontier {
			callers, err := e.callersOfCtx(ctx, k)
			if err != nil {
				return nil, err
			}
			for _, c := range callers {
				if visited[c] {
					continue
				}
				visited[c] = true
				order = append(order, c)
				next = append(next, c)
				if len(order) >= maxNodes {
					return order, nil
				}
			}
		}
		frontier = next
	}
	return order, nil
}

// GraphStatsCtx devuelve el conteo de nodos y de aristas por kind, scopeado.
func (e *DbEngine) GraphStatsCtx(ctx context.Context) (int, map[string]int, error) {
	sc := projectScopeFrom(ctx)
	nClause, nArgs := sc.scopeClause("")
	var nodes int
	if err := e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM code_graph_nodes WHERE 1=1`+nClause, nArgs...).Scan(&nodes); err != nil {
		return 0, nil, fmt.Errorf("error al contar nodos: %w", err)
	}
	eClause, eArgs := sc.scopeClause("")
	rows, err := e.db.QueryContext(ctx, `SELECT kind, COUNT(*) FROM code_graph_edges WHERE 1=1`+eClause+` GROUP BY kind`, eArgs...)
	if err != nil {
		return 0, nil, fmt.Errorf("error al contar aristas: %w", err)
	}
	defer rows.Close()
	byKind := map[string]int{}
	for rows.Next() {
		var k string
		var c int
		if err := rows.Scan(&k, &c); err != nil {
			return 0, nil, err
		}
		byKind[k] = c
	}
	return nodes, byKind, rows.Err()
}

// GraphTopByDegreeCtx devuelve los N nodos con más aristas CALLS incidentes (grado = callers +
// callees): los "god-nodes" del panorama. Scopeado.
func (e *DbEngine) GraphTopByDegreeCtx(ctx context.Context, n int) ([]GraphDegree, error) {
	if n <= 0 {
		n = 10
	}
	sc := projectScopeFrom(ctx)
	c1, a1 := sc.scopeClause("")
	c2, a2 := sc.scopeClause("")
	q := `SELECT k, SUM(c) AS deg FROM (
	        SELECT from_key AS k, COUNT(*) AS c FROM code_graph_edges WHERE kind='CALLS'` + c1 + ` GROUP BY from_key
	        UNION ALL
	        SELECT to_key AS k, COUNT(*) AS c FROM code_graph_edges WHERE kind='CALLS'` + c2 + ` GROUP BY to_key
	      ) GROUP BY k ORDER BY deg DESC, k ASC LIMIT ?`
	args := append(append(append([]interface{}{}, a1...), a2...), n)
	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al calcular god-nodes: %w", err)
	}
	defer rows.Close()
	var out []GraphDegree
	for rows.Next() {
		var d GraphDegree
		if err := rows.Scan(&d.Key, &d.Degree); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GraphEntryPointsCtx devuelve funcs/métodos que NADIE llama internamente (posibles puntos de
// entrada: main, handlers, exports usados desde afuera, tests), acotado a `limit`. Se computa
// como la diferencia de conjuntos en Go (funcs − destinos de CALLS) para no complicar el SQL con
// scope en dos tablas.
func (e *DbEngine) GraphEntryPointsCtx(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 25
	}
	sc := projectScopeFrom(ctx)
	// Funcs/métodos del proyecto.
	nClause, nArgs := sc.scopeClause("")
	rows, err := e.db.QueryContext(ctx,
		`SELECT node_key FROM code_graph_nodes WHERE kind IN ('func','method')`+nClause+` ORDER BY node_key`, nArgs...)
	if err != nil {
		return nil, fmt.Errorf("error al listar funcs: %w", err)
	}
	defer rows.Close()
	var funcs []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		funcs = append(funcs, k)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Destinos de CALLS (los que SÍ son llamados).
	eClause, eArgs := sc.scopeClause("")
	crows, err := e.db.QueryContext(ctx,
		`SELECT DISTINCT to_key FROM code_graph_edges WHERE kind='CALLS'`+eClause, eArgs...)
	if err != nil {
		return nil, fmt.Errorf("error al listar llamados: %w", err)
	}
	defer crows.Close()
	called := map[string]bool{}
	for crows.Next() {
		var k string
		if err := crows.Scan(&k); err != nil {
			return nil, err
		}
		called[k] = true
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range funcs {
		if !called[f] {
			out = append(out, f)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ListGraphNodesForFileCtx devuelve los nodos (símbolos) contenidos en un archivo, scopeado.
func (e *DbEngine) ListGraphNodesForFileCtx(ctx context.Context, path string) ([]GraphNode, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	q := `SELECT node_key, kind, name, path, start_line, end_line, external, src_fingerprint
	      FROM code_graph_nodes WHERE path=? AND kind!='file'` + clause + ` ORDER BY start_line`
	rows, err := e.db.QueryContext(ctx, q, append([]interface{}{path}, args...)...)
	if err != nil {
		return nil, fmt.Errorf("error al listar nodos del archivo: %w", err)
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
