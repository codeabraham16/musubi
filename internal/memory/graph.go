package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

// graph.go implementa una memoria estructurada en GRAFO (entidades + relaciones).
// Es MODEL-FREE desde el server: el agente (que ya es un LLM) aporta los hechos
// como tripletas sujeto-predicado-objeto; el server solo deduplica, almacena y
// recorre. Recuperar HECHOS (no prosa) es mucho más barato en tokens.

const (
	defaultMaxHops  = 2
	defaultMaxFacts = 50
)

// Fact es una tripleta sujeto-predicado-objeto.
type Fact struct {
	Subject   string `json:"subject"`
	Predicate string `json:"predicate"`
	Object    string `json:"object"`
}

// SaveFactResult indica si la relación guardada era nueva.
type SaveFactResult struct {
	Created bool `json:"created"`
}

// GraphResult es el resultado de un recall de hechos alrededor de una entidad.
type GraphResult struct {
	Entity string `json:"entity"`
	Hops   int    `json:"hops"`
	Count  int    `json:"count"`
	Facts  []Fact `json:"facts"`
}

// SaveFact guarda una tripleta (subject, predicate, object). Las entidades se
// deduplican por nombre normalizado (case-insensitive); la relación se deduplica
// por (sujeto, predicado, objeto).
func (e *DbEngine) SaveFact(subject, predicate, object string) (SaveFactResult, error) {
	if strings.TrimSpace(subject) == "" || strings.TrimSpace(predicate) == "" || strings.TrimSpace(object) == "" {
		return SaveFactResult{}, fmt.Errorf("subject, predicate y object son obligatorios")
	}

	tx, err := e.db.Begin()
	if err != nil {
		return SaveFactResult{}, fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	fromID, err := upsertEntity(tx, subject)
	if err != nil {
		return SaveFactResult{}, err
	}
	toID, err := upsertEntity(tx, object)
	if err != nil {
		return SaveFactResult{}, err
	}

	res, err := tx.Exec(
		`INSERT INTO relations (from_id, predicate, to_id) VALUES (?, ?, ?)
		 ON CONFLICT(from_id, predicate, to_id) DO NOTHING`,
		fromID, predicate, toID,
	)
	if err != nil {
		return SaveFactResult{}, fmt.Errorf("error al guardar relación: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return SaveFactResult{}, fmt.Errorf("error al contar filas afectadas: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return SaveFactResult{}, fmt.Errorf("error al commitear hecho: %w", err)
	}
	return SaveFactResult{Created: affected > 0}, nil
}

// upsertEntity devuelve el id de la entidad, creándola si no existe (dedup por
// nombre normalizado).
func upsertEntity(tx *sql.Tx, name string) (int64, error) {
	norm := normalizeForSim(name)
	if _, err := tx.Exec(
		`INSERT INTO entities (name, norm) VALUES (?, ?) ON CONFLICT(norm) DO NOTHING`,
		strings.TrimSpace(name), norm,
	); err != nil {
		return 0, fmt.Errorf("error al guardar entidad: %w", err)
	}
	var id int64
	if err := tx.QueryRow(`SELECT id FROM entities WHERE norm = ?`, norm).Scan(&id); err != nil {
		return 0, fmt.Errorf("error al resolver entidad: %w", err)
	}
	return id, nil
}

// RecallFacts recorre el grafo en anchura (BFS) desde entity hasta maxHops saltos,
// devolviendo hasta maxFacts tripletas. Cero LLM; cero prosa.
func (e *DbEngine) RecallFacts(entity string, maxHops, maxFacts int) (GraphResult, error) {
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}
	if maxFacts <= 0 {
		maxFacts = defaultMaxFacts
	}

	result := GraphResult{Entity: entity, Hops: maxHops, Facts: []Fact{}}

	var startID int64
	err := e.db.QueryRow(`SELECT id FROM entities WHERE norm = ?`, normalizeForSim(entity)).Scan(&startID)
	if err == sql.ErrNoRows {
		return result, nil
	}
	if err != nil {
		return GraphResult{}, fmt.Errorf("error al buscar entidad: %w", err)
	}

	visited := map[int64]bool{startID: true}
	includedRel := map[int64]bool{}
	frontier := []int64{startID}

	for hop := 0; hop < maxHops && len(frontier) > 0 && len(result.Facts) < maxFacts; hop++ {
		next, done, err := e.expandFrontier(frontier, visited, includedRel, &result, maxFacts)
		if err != nil {
			return GraphResult{}, err
		}
		if done {
			break
		}
		frontier = next
	}

	result.Count = len(result.Facts)
	return result, nil
}

// expandFrontier consulta las relaciones que tocan la frontera, agrega hechos
// nuevos (hasta maxFacts) y devuelve la siguiente frontera. done=true si se
// alcanzó el tope de hechos.
func (e *DbEngine) expandFrontier(frontier []int64, visited, includedRel map[int64]bool, result *GraphResult, maxFacts int) ([]int64, bool, error) {
	placeholders := make([]string, len(frontier))
	args := make([]interface{}, 0, len(frontier)*2)
	for i, id := range frontier {
		placeholders[i] = "?"
		args = append(args, id)
	}
	in := strings.Join(placeholders, ",")
	// frontier aparece dos veces (from_id IN ... OR to_id IN ...).
	args = append(args, args...)

	rows, err := e.db.Query(`
		SELECT r.id, ef.name, r.predicate, et.name, r.from_id, r.to_id
		FROM relations r
		JOIN entities ef ON r.from_id = ef.id
		JOIN entities et ON r.to_id = et.id
		WHERE r.from_id IN (`+in+`) OR r.to_id IN (`+in+`)
		ORDER BY r.id
	`, args...)
	if err != nil {
		return nil, false, fmt.Errorf("error al expandir grafo: %w", err)
	}
	defer rows.Close()

	var next []int64
	for rows.Next() {
		var (
			relID           int64
			subj, pred, obj string
			fromID, toID    int64
		)
		if err := rows.Scan(&relID, &subj, &pred, &obj, &fromID, &toID); err != nil {
			return nil, false, fmt.Errorf("error al escanear relación: %w", err)
		}
		if includedRel[relID] {
			continue
		}
		includedRel[relID] = true
		result.Facts = append(result.Facts, Fact{Subject: subj, Predicate: pred, Object: obj})

		for _, nid := range []int64{fromID, toID} {
			if !visited[nid] {
				visited[nid] = true
				next = append(next, nid)
			}
		}

		if len(result.Facts) >= maxFacts {
			return next, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("error al iterar relaciones del grafo: %w", err)
	}
	return next, false, nil
}
