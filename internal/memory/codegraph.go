package memory

import (
	"context"
	"database/sql"
	"fmt"
)

// codegraph.go persiste el GRAFO DE CÓDIGO derivado (Track 20 · F1): nodos (archivos,
// símbolos, paquetes) y aristas tipadas (IMPORTS/CONTAINS/CALLS) scopeados por project_id.
// El grafo NACE derivado del AST en internal/codeintel; acá sólo se persiste y se lee
// scopeado. Es model-free: el motor no razona, sólo guarda y compara. El src_fingerprint de
// cada fila lo estampa la capa MCP (que tiene fs); el motor lo persiste y lo devuelve para que
// MCP detecte staleness comparándolo con el fingerprint actual, igual que con code_memory.
//
// El paquete memory NO importa codeintel (queda desacoplado, como con CodeMemory): define sus
// propios structs de fila y la capa MCP convierte codeintel.Node/Edge → GraphNode/GraphEdge.

// GraphNode es una fila de code_graph_nodes.
type GraphNode struct {
	Key            string `json:"key"`
	Kind           string `json:"kind"`
	Name           string `json:"name"`
	Path           string `json:"path"`
	StartLine      int    `json:"start_line"`
	EndLine        int    `json:"end_line"`
	External       bool   `json:"external"`
	SrcFingerprint string `json:"src_fingerprint"`
}

// GraphEdge es una fila de code_graph_edges. SrcPath es el archivo que la POSEE: el refresco
// borra por src_path y reinserta, de modo que el grafo nunca quede con aristas stale.
type GraphEdge struct {
	FromKey        string  `json:"from_key"`
	ToKey          string  `json:"to_key"`
	Kind           string  `json:"kind"`
	Confidence     float64 `json:"confidence"`
	Provenance     string  `json:"provenance"`
	SrcPath        string  `json:"src_path"`
	SrcFingerprint string  `json:"src_fingerprint"`
}

// UpsertPackageGraph persiste el grafo de un paquete atribuido al project_id del engine.
func (e *DbEngine) UpsertPackageGraph(files []string, nodes []GraphNode, edges []GraphEdge) error {
	return e.UpsertPackageGraphFrom("", files, nodes, edges)
}

// UpsertPackageGraphFrom persiste el grafo de un paquete con el project_id de ORIGEN explícito
// (atribución multi-tenant). En UNA transacción: borra los nodos/aristas de los `files` dados
// (delete-by-source) y reinserta los derivados con su src_fingerprint. Así el refresco es local
// a los archivos cambiados y nunca deja filas stale. Los nodos package externos (path='') NO se
// borran (son compartidos entre archivos): se reinsertan por upsert. origin == "" ⇒ project_id
// del engine.
func (e *DbEngine) UpsertPackageGraphFrom(originProjectID string, files []string, nodes []GraphNode, edges []GraphEdge) error {
	projectID := originProjectID
	if projectID == "" {
		projectID = e.projectID
	}
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción del grafo de código: %w", err)
	}
	defer tx.Rollback()

	for _, f := range files {
		if _, err := tx.Exec(`DELETE FROM code_graph_nodes WHERE project_id=? AND path=?`, projectID, f); err != nil {
			return fmt.Errorf("error al limpiar nodos de %s: %w", f, err)
		}
		if _, err := tx.Exec(`DELETE FROM code_graph_edges WHERE project_id=? AND src_path=?`, projectID, f); err != nil {
			return fmt.Errorf("error al limpiar aristas de %s: %w", f, err)
		}
	}
	for _, n := range nodes {
		if _, err := tx.Exec(
			`INSERT INTO code_graph_nodes (project_id, node_key, kind, name, path, start_line, end_line, external, src_fingerprint, updated_at)
			 VALUES (?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
			 ON CONFLICT(project_id, node_key) DO UPDATE SET
			   kind=excluded.kind, name=excluded.name, path=excluded.path,
			   start_line=excluded.start_line, end_line=excluded.end_line,
			   external=excluded.external, src_fingerprint=excluded.src_fingerprint,
			   updated_at=CURRENT_TIMESTAMP`,
			projectID, n.Key, n.Kind, n.Name, n.Path, n.StartLine, n.EndLine, cgBoolToInt(n.External), n.SrcFingerprint,
		); err != nil {
			return fmt.Errorf("error al guardar nodo %s: %w", n.Key, err)
		}
	}
	for _, ed := range edges {
		if _, err := tx.Exec(
			`INSERT INTO code_graph_edges (project_id, from_key, to_key, kind, confidence, provenance, src_path, src_fingerprint, updated_at)
			 VALUES (?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
			 ON CONFLICT(project_id, from_key, to_key, kind) DO UPDATE SET
			   confidence=excluded.confidence, provenance=excluded.provenance,
			   src_path=excluded.src_path, src_fingerprint=excluded.src_fingerprint,
			   updated_at=CURRENT_TIMESTAMP`,
			projectID, ed.FromKey, ed.ToKey, ed.Kind, ed.Confidence, ed.Provenance, ed.SrcPath, ed.SrcFingerprint,
		); err != nil {
			return fmt.Errorf("error al guardar arista %s→%s: %w", ed.FromKey, ed.ToKey, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el grafo de código: %w", err)
	}
	return nil
}

// GetGraphNodeCtx devuelve un nodo por su clave, acotado al proyecto de la credencial (Track
// 17/18): con scope, sólo el nodo del proyecto pedido o el sin atribuir (''), PREFIRIENDO el
// del proyecto. Ausencia de scope / Federate ⇒ federado (la primera fila de ese node_key).
func (e *DbEngine) GetGraphNodeCtx(ctx context.Context, nodeKey string) (GraphNode, bool, error) {
	sc := projectScopeFrom(ctx)
	var row *sql.Row
	if sc.Federate || sc.ProjectID == "" {
		row = e.db.QueryRowContext(ctx,
			`SELECT node_key, kind, name, path, start_line, end_line, external, src_fingerprint
			 FROM code_graph_nodes WHERE node_key=? LIMIT 1`, nodeKey)
	} else {
		row = e.db.QueryRowContext(ctx,
			`SELECT node_key, kind, name, path, start_line, end_line, external, src_fingerprint
			 FROM code_graph_nodes WHERE node_key=? AND (project_id=? OR project_id='')
			 ORDER BY (project_id=?) DESC LIMIT 1`, nodeKey, sc.ProjectID, sc.ProjectID)
	}
	var n GraphNode
	var ext int
	err := row.Scan(&n.Key, &n.Kind, &n.Name, &n.Path, &n.StartLine, &n.EndLine, &ext, &n.SrcFingerprint)
	if err == sql.ErrNoRows {
		return GraphNode{}, false, nil
	}
	if err != nil {
		return GraphNode{}, false, fmt.Errorf("error al leer nodo del grafo: %w", err)
	}
	n.External = ext != 0
	return n, true, nil
}

// GraphOutEdgesCtx devuelve las aristas SALIENTES de un nodo, acotadas al proyecto de la
// credencial conservando las filas sin atribuir (mismo criterio que el recall). Federate / sin
// scope ⇒ sin filtro. Es la primitiva de recorrido mínima de F1; la superficie rica es F2.
func (e *DbEngine) GraphOutEdgesCtx(ctx context.Context, fromKey string) ([]GraphEdge, error) {
	sc := projectScopeFrom(ctx)
	clause, args := sc.scopeClause("")
	q := `SELECT from_key, to_key, kind, confidence, provenance, src_path, src_fingerprint
	      FROM code_graph_edges WHERE from_key=?` + clause + ` ORDER BY kind, to_key`
	qargs := append([]interface{}{fromKey}, args...)
	rows, err := e.db.QueryContext(ctx, q, qargs...)
	if err != nil {
		return nil, fmt.Errorf("error al leer aristas del grafo: %w", err)
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

// cgBoolToInt mapea el flag external a 0/1 para la columna INTEGER.
func cgBoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
