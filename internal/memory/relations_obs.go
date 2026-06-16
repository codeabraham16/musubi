package memory

import (
	"fmt"

	"github.com/google/uuid"
)

// relations_obs.go implementa el grafo de RELACIONES SEMÁNTICAS entre
// observaciones (resolución de conflictos model-free). A diferencia del grafo de
// hechos (entities/relations), acá las aristas vinculan dos observaciones y
// expresan cómo se relacionan sus contenidos: una reemplaza a otra, la contradice,
// es compatible, etc. El vocabulario es espejo del de Engram (probado).

// Tipos de relación entre dos observaciones.
const (
	RelPending       = "pending"        // candidato detectado, sin veredicto
	RelRelated       = "related"        // conectadas temáticamente
	RelCompatible    = "compatible"     // no se contradicen
	RelScoped        = "scoped"         // una es más general, otra más específica
	RelConflictsWith = "conflicts_with" // se contradicen directamente
	RelSupersedes    = "supersedes"     // source reemplaza/obsoleta a target
	RelNotConflict   = "not_conflict"   // explícitamente NO en conflicto
)

// Estados de una relación.
const (
	RelStatusPending  = "pending"
	RelStatusResolved = "resolved"
)

// ObsRelation es una arista entre dos observaciones (source -> target).
type ObsRelation struct {
	ID         string  `json:"id"`
	SourceID   string  `json:"source_id"`
	TargetID   string  `json:"target_id"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	ResolvedBy string  `json:"resolved_by,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// UpsertObsRelation inserta o actualiza la relación del par (source, target) y
// devuelve el id CANÓNICO persistido (el existente si el par ya estaba, o el
// nuevo). Es idempotente por par: el mismo par actualiza el veredicto en vez de
// duplicar.
func (e *DbEngine) UpsertObsRelation(r ObsRelation) (string, error) {
	if r.Relation == "" {
		r.Relation = RelPending
	}
	if r.Status == "" {
		r.Status = RelStatusPending
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	_, err := e.db.Exec(`
		INSERT INTO observation_relations (id, source_id, target_id, relation, confidence, status, resolved_by, reason, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source_id, target_id) DO UPDATE SET
			relation=excluded.relation,
			confidence=excluded.confidence,
			status=excluded.status,
			resolved_by=excluded.resolved_by,
			reason=excluded.reason,
			updated_at=CURRENT_TIMESTAMP
	`, r.ID, r.SourceID, r.TargetID, r.Relation, r.Confidence, r.Status, nullable(r.ResolvedBy), nullable(r.Reason))
	if err != nil {
		return "", fmt.Errorf("error al upsertar relación de observaciones: %w", err)
	}
	// El par puede haber existido con otro id (ON CONFLICT no cambia el id): devolver
	// el id realmente persistido.
	var id string
	if err := e.db.QueryRow(
		`SELECT id FROM observation_relations WHERE source_id=? AND target_id=?`,
		r.SourceID, r.TargetID,
	).Scan(&id); err != nil {
		return r.ID, nil // best-effort: si falla la relectura, devolver el tentativo
	}
	return id, nil
}

// PendingObsRelations devuelve las relaciones que aún esperan veredicto.
func (e *DbEngine) PendingObsRelations() ([]ObsRelation, error) {
	return e.queryObsRelations(`WHERE status = ?`, RelStatusPending)
}

// AllObsRelations devuelve todas las relaciones (para inspección/tests).
func (e *DbEngine) AllObsRelations() ([]ObsRelation, error) {
	return e.queryObsRelations(``)
}

// ResolveObsRelation fija el veredicto de una relación por id y la marca resuelta.
// Si el veredicto es supersedes, además oculta la observación target del recall
// (markSuperseded), reflejando que quedó obsoleta.
func (e *DbEngine) ResolveObsRelation(id, relation, resolvedBy, reason string) error {
	var sourceID, targetID string
	row := e.db.QueryRow(`SELECT source_id, target_id FROM observation_relations WHERE id=?`, id)
	if err := row.Scan(&sourceID, &targetID); err != nil {
		return fmt.Errorf("no existe la relación %q: %w", id, err)
	}

	if _, err := e.db.Exec(`
		UPDATE observation_relations
		SET relation=?, status=?, resolved_by=?, reason=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, relation, RelStatusResolved, nullable(resolvedBy), nullable(reason), id); err != nil {
		return fmt.Errorf("error al resolver relación %q: %w", id, err)
	}

	if relation == RelSupersedes {
		if err := e.markSuperseded(targetID, sourceID); err != nil {
			return err
		}
	}
	return nil
}

// queryObsRelations centraliza el SELECT con un filtro WHERE opcional.
func (e *DbEngine) queryObsRelations(where string, args ...interface{}) ([]ObsRelation, error) {
	q := `SELECT id, source_id, target_id, relation, confidence, status,
	             COALESCE(resolved_by,''), COALESCE(reason,'')
	      FROM observation_relations ` + where + ` ORDER BY updated_at DESC`
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar relaciones de observaciones: %w", err)
	}
	defer rows.Close()
	var out []ObsRelation
	for rows.Next() {
		var r ObsRelation
		if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Relation, &r.Confidence,
			&r.Status, &r.ResolvedBy, &r.Reason); err != nil {
			return nil, fmt.Errorf("error al escanear relación: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// nullable convierte "" en NULL para columnas opcionales.
func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
