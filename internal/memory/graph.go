package memory

import (
	"context"
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

// SaveFact guarda una tripleta en el espacio FEDERADO (project_id=''): el comportamiento
// histórico (stdio local, admin). Es un fino wrapper sobre SaveFactFrom.
func (e *DbEngine) SaveFact(subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error) {
	return e.SaveFactFrom("", subject, predicate, object, validFrom, singleValued)
}

// SaveFactFrom guarda una tripleta (subject, predicate, object) ATRIBUIDA a originProjectID
// (aislamiento multi-tenant, Track 17). Las entidades se deduplican por nombre normalizado
// (case-insensitive) y son GLOBALES (los nodos se comparten; sólo las aristas se atribuyen);
// la relación se deduplica por (sujeto, predicado, objeto, project_id), así que el mismo triple
// puede coexistir en varios proyectos. Si el predicado es single-valued (∈ singleValued,
// case-insensitive) invalida los hechos vivos del mismo (sujeto, predicado) con OTRO objeto
// DENTRO DEL MISMO PROYECTO (cardinalidad ESTRICTA por proyecto): un save en el proyecto A nunca
// cierra la ventana de un hecho vivo de B ni de los legacy ''. validFrom (ISO opcional) marca
// desde cuándo el hecho es verdad; ausente/ inválido → ahora. Re-afirmar un triplete invalidado
// lo revive. originProjectID '' ⇒ espacio federado histórico (admin/stdio); en ese caso la
// cardinalidad se acota a '' y el comportamiento es bit-idéntico al previo a v14.
func (e *DbEngine) SaveFactFrom(originProjectID, subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error) {
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

	// ¿Existía ya el triplete exacto EN ESTE PROYECTO antes de este guardado? Determina Created
	// (fila nueva) vs. revivencia/no-op. El mismo triple en otro proyecto es una fila distinta.
	var dummy int64
	errSel := tx.QueryRow(
		`SELECT id FROM relations WHERE from_id=? AND predicate=? AND to_id=? AND project_id=?`,
		fromID, predicate, toID, originProjectID,
	).Scan(&dummy)
	if errSel != nil && errSel != sql.ErrNoRows {
		return SaveFactResult{}, fmt.Errorf("error al verificar existencia del hecho: %w", errSel)
	}
	created := errSel == sql.ErrNoRows

	// UPSERT: inserta el hecho vivo, o REVIVE uno invalidado (limpia la ventana de
	// invalidación y actualiza valid_from). vf NULL → datetime('now'). El ON CONFLICT incluye
	// project_id: el upsert dedup-a POR PROYECTO.
	vf := parseTimestamp(validFrom)
	if _, err := tx.Exec(`
		INSERT INTO relations (from_id, predicate, to_id, project_id, valid_from, created_at)
		VALUES (?, ?, ?, ?, COALESCE(?, datetime('now')), datetime('now'))
		ON CONFLICT(from_id, predicate, to_id, project_id) DO UPDATE SET
			valid_from = COALESCE(excluded.valid_from, datetime('now')),
			valid_to = NULL, invalidated_at = NULL, superseded_by = NULL`,
		fromID, predicate, toID, originProjectID, vf,
	); err != nil {
		return SaveFactResult{}, fmt.Errorf("error al guardar relación: %w", err)
	}

	var newID int64
	if err := tx.QueryRow(
		`SELECT id FROM relations WHERE from_id=? AND predicate=? AND to_id=? AND project_id=?`,
		fromID, predicate, toID, originProjectID,
	).Scan(&newID); err != nil {
		return SaveFactResult{}, fmt.Errorf("error al resolver el id del hecho: %w", err)
	}

	// Invalidación por cardinalidad: si el predicado es funcional, cerrar la ventana de los otros
	// objetos vivos del mismo (sujeto, predicado) DENTRO DE ESTE PROYECTO. El AND project_id=? es
	// la garantía de aislamiento de ESCRITURA: un save nunca invalida hechos de otro proyecto (ni
	// los legacy ''). Nunca se borran.
	invalidated := 0
	if isSingleValued(predicate, singleValued) {
		res, err := tx.Exec(`
			UPDATE relations
			   SET valid_to = datetime('now'), invalidated_at = datetime('now'), superseded_by = ?
			 WHERE from_id = ? AND predicate = ? AND to_id != ? AND invalidated_at IS NULL AND project_id = ?`,
			newID, fromID, predicate, toID, originProjectID,
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

// RecallFacts recupera hasta maxFacts tripletas alrededor de entity. Por defecto sólo hechos
// VIGENTES (invalidated_at IS NULL); con asOf (ISO) devuelve los que eran válidos en ese
// instante (point-in-time); asOf inválido → verdad actual. El modo de ranking lo elige rank:
//   - "" o "bfs": recorrido en anchura (BFS) hasta maxHops saltos (comportamiento histórico).
//   - "pagerank": recall asociativo por Personalized PageRank personalizado a entity; rankea
//     los hechos por relevancia asociativa multi-hop y devuelve los maxFacts de mayor score.
//     maxHops se ignora en este modo (el damping ya maneja los saltos). Ver pagerank.go.
//
// Cero LLM; cero prosa. El filtro temporal es común a ambos modos, así que pagerank + asOf da
// PageRank point-in-time.
func (e *DbEngine) RecallFacts(entity string, maxHops, maxFacts int, asOf, rank string) (GraphResult, error) {
	return e.RecallFactsCtx(context.Background(), entity, maxHops, maxFacts, asOf, rank)
}

// liveFactFilter arma el fragmento WHERE (referenciando el alias r) que selecciona las
// relaciones VISIBLES a esta consulta: el predicado bi-temporal "vivo" (verdad actual por
// defecto; point-in-time con asOf) Y el scope multi-tenant del proyecto (Track 17). Plegar
// AMBOS en un solo par (filtro, args) hace que TODA superficie de traversal del grafo —BFS
// (expandFrontier), recall asociativo (buildFactGraph/recallFactsPageRank) y caminos
// (pathNeighbors)— quede scopeada IDÉNTICAMENTE, porque todas interpolan este fragmento. Ctx
// federado (admin/stdio, o ProjectID vacío) ⇒ sin cláusula de proyecto: histórico bit-a-bit.
// Los placeholders de proyecto van DESPUÉS de los temporales, igual que los args, así que el
// orden es consistente en todos los call sites (que anteponen los args de la frontera/nodo).
func liveFactFilter(ctx context.Context, asOf string) (string, []interface{}) {
	filter := "r.invalidated_at IS NULL"
	var args []interface{}
	if af := parseTimestamp(asOf); af != nil {
		filter = "r.valid_from <= ? AND (r.valid_to IS NULL OR r.valid_to > ?)"
		args = []interface{}{af, af}
	}
	if scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("r"); scopeSQL != "" {
		filter += scopeSQL // " AND (r.project_id = ? OR r.project_id IS NULL OR r.project_id = '')"
		args = append(args, scopeArgs...)
	}
	return filter, args
}

// RecallFactsCtx es RecallFacts scopeada al proyecto del contexto (Track 17): sólo recorre las
// aristas visibles al proyecto del principal (las propias + las sin atribuir). Las ENTIDADES son
// globales, así que la semilla se resuelve igual; lo que se acota es el traversal de relaciones.
func (e *DbEngine) RecallFactsCtx(ctx context.Context, entity string, maxHops, maxFacts int, asOf, rank string) (GraphResult, error) {
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}
	if maxFacts <= 0 {
		maxFacts = defaultMaxFacts
	}

	// Filtro combinado temporal + scope de proyecto; común a BFS y a pagerank.
	liveFilter, filterArgs := liveFactFilter(ctx, asOf)

	result := GraphResult{Entity: entity, Hops: maxHops, Facts: []Fact{}}

	var startID int64
	err := e.db.QueryRow(`SELECT id FROM entities WHERE norm = ?`, normalizeForSim(entity)).Scan(&startID)
	if err == sql.ErrNoRows {
		return result, nil
	}
	if err != nil {
		return GraphResult{}, fmt.Errorf("error al buscar entidad: %w", err)
	}

	// Modo asociativo: delegar a PageRank tras resolver la semilla (branch temprano; el
	// camino BFS de abajo queda intacto para rank "" / "bfs").
	if rank == "pagerank" {
		res, err := e.recallFactsPageRank(startID, maxFacts, liveFilter, filterArgs)
		if err != nil {
			return GraphResult{}, err
		}
		res.Entity = entity
		res.Hops = maxHops
		return res, nil
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
