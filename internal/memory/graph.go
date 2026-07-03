package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// graph.go implementa una memoria estructurada en GRAFO (entidades + relaciones).
// Es MODEL-FREE desde el server: el agente (que ya es un LLM) aporta los hechos
// como tripletas sujeto-predicado-objeto; el server solo deduplica, almacena y
// recorre. Recuperar HECHOS (no prosa) es mucho más barato en tokens.
//
// BI-TEMPORAL (v0.59+): cada relación lleva dos ejes de tiempo. El del EVENTO
// (valid_from/valid_to): desde/hasta cuándo el hecho es verdad en el mundo. El de la
// TRANSACCIÓN (invalidated_at/superseded_by): cuándo Musubi supo que dejó de ser la
// verdad corriente y qué hecho lo reemplazó. "Verdad actual" = invalidated_at IS NULL.
// La contradicción se juzga SIN LLM por CARDINALIDAD: para un predicado funcional
// (single-valued, declarado en config) sólo puede haber un objeto vivo por sujeto, así
// que guardar (S,P,O_new) invalida los (S,P,O_old) vivos. Nunca se borra: se cierra la
// ventana, de modo que la historia queda auditable y la consulta point-in-time (as_of)
// es un simple filtro por fecha.

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

// SaveFactResult indica si la relación guardada era nueva y cuántas invalidó por
// cardinalidad.
type SaveFactResult struct {
	Created     bool `json:"created"`
	Invalidated int  `json:"invalidated"`
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
// por (sujeto, predicado, objeto). Si el predicado es single-valued (∈ singleValued,
// comparación case-insensitive), invalida los hechos vivos del mismo (sujeto, predicado)
// con OTRO objeto (cardinalidad). validFrom (ISO opcional) marca desde cuándo el hecho es
// verdad; ausente/ inválido → ahora. Re-afirmar un triplete invalidado lo revive.
func (e *DbEngine) SaveFact(subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error) {
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

	// ¿Existía ya el triplete exacto antes de este guardado? Determina Created (fila
	// nueva) vs. revivencia/no-op.
	var dummy int64
	errSel := tx.QueryRow(
		`SELECT id FROM relations WHERE from_id=? AND predicate=? AND to_id=?`,
		fromID, predicate, toID,
	).Scan(&dummy)
	if errSel != nil && errSel != sql.ErrNoRows {
		return SaveFactResult{}, fmt.Errorf("error al verificar existencia del hecho: %w", errSel)
	}
	created := errSel == sql.ErrNoRows

	// UPSERT: inserta el hecho vivo, o REVIVE uno invalidado (limpia la ventana de
	// invalidación y actualiza valid_from). vf NULL → datetime('now').
	vf := parseTimestamp(validFrom)
	if _, err := tx.Exec(`
		INSERT INTO relations (from_id, predicate, to_id, valid_from, created_at)
		VALUES (?, ?, ?, COALESCE(?, datetime('now')), datetime('now'))
		ON CONFLICT(from_id, predicate, to_id) DO UPDATE SET
			valid_from = COALESCE(excluded.valid_from, datetime('now')),
			valid_to = NULL, invalidated_at = NULL, superseded_by = NULL`,
		fromID, predicate, toID, vf,
	); err != nil {
		return SaveFactResult{}, fmt.Errorf("error al guardar relación: %w", err)
	}

	var newID int64
	if err := tx.QueryRow(
		`SELECT id FROM relations WHERE from_id=? AND predicate=? AND to_id=?`,
		fromID, predicate, toID,
	).Scan(&newID); err != nil {
		return SaveFactResult{}, fmt.Errorf("error al resolver el id del hecho: %w", err)
	}

	// Invalidación por cardinalidad: si el predicado es funcional, cerrar la ventana de
	// los otros objetos vivos del mismo (sujeto, predicado). Nunca se borran.
	invalidated := 0
	if isSingleValued(predicate, singleValued) {
		res, err := tx.Exec(`
			UPDATE relations
			   SET valid_to = datetime('now'), invalidated_at = datetime('now'), superseded_by = ?
			 WHERE from_id = ? AND predicate = ? AND to_id != ? AND invalidated_at IS NULL`,
			newID, fromID, predicate, toID,
		)
		if err != nil {
			return SaveFactResult{}, fmt.Errorf("error al invalidar hechos contradictorios: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return SaveFactResult{}, fmt.Errorf("error al contar invalidados: %w", err)
		}
		invalidated = int(n)
	}

	if err := tx.Commit(); err != nil {
		return SaveFactResult{}, fmt.Errorf("error al commitear hecho: %w", err)
	}
	return SaveFactResult{Created: created, Invalidated: invalidated}, nil
}

// isSingleValued indica si el predicado es funcional (a lo sumo un objeto vivo por
// sujeto). Comparación case-insensitive contra el conjunto declarado en config.
func isSingleValued(predicate string, set []string) bool {
	p := strings.ToLower(strings.TrimSpace(predicate))
	for _, s := range set {
		if strings.ToLower(strings.TrimSpace(s)) == p {
			return true
		}
	}
	return false
}

// parseTimestamp normaliza una marca ISO opcional a 'YYYY-MM-DD HH:MM:SS' (UTC, el
// formato de SQLite). Vacío o inválido → nil (el caller usa datetime('now')); nunca
// falla, para no romper un guardado por un typo de fecha (no inferimos fechas de prosa).
func parseTimestamp(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05")
		}
	}
	return nil
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
// devolviendo hasta maxFacts tripletas. Por defecto sólo hechos VIGENTES
// (invalidated_at IS NULL). Con asOf (ISO) devuelve los que eran válidos en ese
// instante (point-in-time); asOf inválido → verdad actual. Cero LLM; cero prosa.
func (e *DbEngine) RecallFacts(entity string, maxHops, maxFacts int, asOf string) (GraphResult, error) {
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}
	if maxFacts <= 0 {
		maxFacts = defaultMaxFacts
	}

	// Filtro temporal: por defecto "verdad actual"; con asOf válido, point-in-time.
	liveFilter := "r.invalidated_at IS NULL"
	var filterArgs []interface{}
	if af := parseTimestamp(asOf); af != nil {
		liveFilter = "r.valid_from <= ? AND (r.valid_to IS NULL OR r.valid_to > ?)"
		filterArgs = []interface{}{af, af}
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
		next, done, err := e.expandFrontier(frontier, visited, includedRel, &result, maxFacts, liveFilter, filterArgs)
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

// expandFrontier consulta las relaciones que tocan la frontera (respetando el filtro
// temporal en CADA salto), agrega hechos nuevos (hasta maxFacts) y devuelve la
// siguiente frontera. done=true si se alcanzó el tope de hechos.
func (e *DbEngine) expandFrontier(frontier []int64, visited, includedRel map[int64]bool, result *GraphResult, maxFacts int, liveFilter string, filterArgs []interface{}) ([]int64, bool, error) {
	placeholders := make([]string, len(frontier))
	args := make([]interface{}, 0, len(frontier)*2+len(filterArgs))
	for i, id := range frontier {
		placeholders[i] = "?"
		args = append(args, id)
	}
	in := strings.Join(placeholders, ",")
	// frontier aparece dos veces (from_id IN ... OR to_id IN ...).
	args = append(args, args...)
	// El filtro temporal va al final, después de los args de la frontera.
	args = append(args, filterArgs...)

	rows, err := e.db.Query(`
		SELECT r.id, ef.name, r.predicate, et.name, r.from_id, r.to_id
		FROM relations r
		JOIN entities ef ON r.from_id = ef.id
		JOIN entities et ON r.to_id = et.id
		WHERE (r.from_id IN (`+in+`) OR r.to_id IN (`+in+`)) AND (`+liveFilter+`)
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
