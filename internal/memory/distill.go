package memory

import "fmt"

// distill.go expone las consultas del DESTILADOR del acervo (pilar 'Musubi Renaissance'): recorrer los
// blobs crudos ingeridos que TODAVÍA no produjeron tarjetas curadas, para que un pase offline (con motor
// de cognición) los destile de a tandas. La capa de memoria se queda AGNÓSTICA del dominio: no sabe de
// "diseño" ni de "tarjetas", sólo de "observaciones con tal prefijo de topic que no son destino de tal
// relación". El mcp le pasa los términos concretos (prefijo `ingested/`, relación `derived_from`).

// ObsLite es una proyección liviana de una observación (id + topic + contenido), sin ranking ni metadata
// de scope. La usan los pases internos en masa —el destilador el primero— que necesitan el texto crudo y
// nada más.
type ObsLite struct {
	ID       string
	TopicKey string
	Content  string
}

// ObservationsMissingRelation devuelve hasta limit observaciones VISIBLES de projectID cuyo topic_key
// empieza con topicPrefix y que NO son DESTINO (target) de ninguna relación `relation`. Es la consulta
// del destilador: los blobs `ingested/*` a los que todavía no les apunta una arista `derived_from` (o
// sea, de los que aún no salió ninguna tarjeta). Orden FIFO (created_at ASC) para drenar el backlog de a
// tandas sin repetir. Model-free (query keyed, sin LLM ni embeddings).
func (e *DbEngine) ObservationsMissingRelation(projectID, topicPrefix, relation string, limit int) ([]ObsLite, error) {
	if limit <= 0 {
		limit = 8
	}
	// target_id es NOT NULL (FK), así que el NOT IN no corre riesgo del gotcha del NULL en SQL. El
	// visibleObsPredicate va con columnas desnudas: acá hay una sola tabla `observations` en el scope
	// externo, así que resuelven a ella sin ambigüedad.
	q := `SELECT id, topic_key, content FROM observations
	      WHERE project_id = ? AND topic_key LIKE ? AND ` + visibleObsPredicate + `
	        AND id NOT IN (SELECT target_id FROM observation_relations WHERE relation = ?)
	      ORDER BY created_at ASC, rowid ASC LIMIT ?`
	rows, err := e.db.Query(q, projectID, topicPrefix+"%", relation, limit)
	if err != nil {
		return nil, fmt.Errorf("observaciones sin relación %q (prefijo %q): %w", relation, topicPrefix, err)
	}
	defer rows.Close()
	var out []ObsLite
	for rows.Next() {
		var o ObsLite
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content); err != nil {
			return nil, fmt.Errorf("escanear observación sin destilar: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterar observaciones sin destilar: %w", err)
	}
	return out, nil
}

// CountObservationsMissingRelation cuenta lo mismo que ObservationsMissingRelation sin traer las filas:
// cuántos blobs quedan por destilar, para reportar el progreso del backlog sin pagar el contenido.
func (e *DbEngine) CountObservationsMissingRelation(projectID, topicPrefix, relation string) (int, error) {
	q := `SELECT COUNT(*) FROM observations
	      WHERE project_id = ? AND topic_key LIKE ? AND ` + visibleObsPredicate + `
	        AND id NOT IN (SELECT target_id FROM observation_relations WHERE relation = ?)`
	var n int
	if err := e.db.QueryRow(q, projectID, topicPrefix+"%", relation).Scan(&n); err != nil {
		return 0, fmt.Errorf("contar observaciones sin relación %q (prefijo %q): %w", relation, topicPrefix, err)
	}
	return n, nil
}
