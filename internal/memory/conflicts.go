package memory

import (
	"fmt"
)

// conflicts.go implementa la detección de relaciones semánticas entre
// observaciones, 100% model-free. Al guardar una observación, busca candidatas
// por FTS, mide la similitud léxica (Jaccard de trigramas) y emite un veredicto
// HEURÍSTICO determinista:
//   - mismo topic_key + similitud alta + la nueva es más reciente  -> supersedes (auto)
//   - similitud alta cross-topic                                   -> related   (auto)
//   - similitud media                                              -> pending   (lo juzga el agente)
//   - similitud por debajo del piso                                -> se ignora
//
// Lo que NO se puede deducir sin LLM (contradicción semántica con negación) es
// justamente lo que queda "pending" para que lo juzgue el agente vía musubi_judge.

// Umbrales calibrados para Jaccard de trigramas de caracteres (corre bajo: dos
// frases casi idénticas dan ~0.8; relacionadas pero distintas ~0.35; ajenas ~0).
const (
	defaultSimilarityFloor      = 0.3
	defaultAutoResolveThreshold = 0.7
	defaultConflictCandidates   = 10
)

// ConflictOptions parametriza la detección. Los ceros usan los defaults.
type ConflictOptions struct {
	SimilarityFloor      float64 // piso para considerar dos observaciones relacionadas
	AutoResolveThreshold float64 // a partir de acá se auto-resuelve (supersede/related)
	CandidatePool        int     // candidatas a evaluar por FTS
}

func (o ConflictOptions) withDefaults() ConflictOptions {
	if o.SimilarityFloor <= 0 {
		o.SimilarityFloor = defaultSimilarityFloor
	}
	if o.AutoResolveThreshold <= 0 {
		o.AutoResolveThreshold = defaultAutoResolveThreshold
	}
	if o.CandidatePool <= 0 {
		o.CandidatePool = defaultConflictCandidates
	}
	return o
}

type obsRow struct {
	id        string
	topicKey  string
	content   string
	createdAt string
}

// DetectRelations evalúa la observación obsID contra sus candidatas y persiste las
// relaciones detectadas (auto-resueltas o pendientes). Devuelve las relaciones
// creadas/actualizadas para que la capa MCP pueda surfacearlas al agente.
func (e *DbEngine) DetectRelations(obsID string, opts ConflictOptions) ([]ObsRelation, error) {
	opts = opts.withDefaults()

	src, ok, err := e.loadObsRow(obsID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	cands, err := e.conflictCandidates(src, opts.CandidatePool)
	if err != nil {
		return nil, err
	}

	var out []ObsRelation
	for _, c := range cands {
		sim := Similarity(src.content, c.content)
		if sim < opts.SimilarityFloor {
			continue
		}

		rel := decideRelation(src, c, sim, opts.AutoResolveThreshold)
		id, err := e.UpsertObsRelation(rel)
		if err != nil {
			return nil, err
		}
		rel.ID = id
		// Efecto del supersede auto-resuelto: ocultar la observación obsoleta del
		// recall marcando superseded_by (reversible, auditable).
		if rel.Relation == RelSupersedes && rel.Status == RelStatusResolved {
			if err := e.markSuperseded(rel.TargetID, rel.SourceID); err != nil {
				return nil, err
			}
		}
		out = append(out, rel)
	}
	return out, nil
}

// decideRelation aplica la heurística determinista a un par (src=recién guardada,
// candidato). src es la observación sobre la que corre DetectRelations.
func decideRelation(src, cand obsRow, sim, autoThreshold float64) ObsRelation {
	switch {
	case sim >= autoThreshold && src.topicKey == cand.topicKey && src.createdAt > cand.createdAt:
		// Mismo tema + casi duplicado + la recién guardada es ESTRICTAMENTE más
		// nueva: reemplaza a la anterior. Solo auto-supersede en esta dirección, así
		// nunca ocultamos la observación recién guardada ni contenido más nuevo.
		return ObsRelation{
			SourceID:   src.id,
			TargetID:   cand.id,
			Relation:   RelSupersedes,
			Confidence: sim,
			Status:     RelStatusResolved,
			ResolvedBy: "heuristic",
			Reason:     "mismo topic_key y similitud alta; la observación más reciente reemplaza a la anterior",
		}
	case sim >= autoThreshold && src.topicKey == cand.topicKey:
		// Mismo tema + alta similitud, pero la candidata es igual o más nueva: no
		// auto-ocultar nada; que el agente decida.
		return ObsRelation{
			SourceID:   src.id,
			TargetID:   cand.id,
			Relation:   RelPending,
			Confidence: sim,
			Status:     RelStatusPending,
		}
	case sim >= autoThreshold:
		// Alto solape pero distinto tema: relacionadas, sin contradicción deducible.
		return ObsRelation{
			SourceID:   src.id,
			TargetID:   cand.id,
			Relation:   RelRelated,
			Confidence: sim,
			Status:     RelStatusResolved,
			ResolvedBy: "heuristic",
			Reason:     "alto solape léxico; relacionadas",
		}
	default:
		// Similitud media: podría ser contradicción o complemento. No es deducible
		// sin entender el texto -> lo juzga el agente.
		return ObsRelation{
			SourceID:   src.id,
			TargetID:   cand.id,
			Relation:   RelPending,
			Confidence: sim,
			Status:     RelStatusPending,
		}
	}
}

// loadObsRow trae los campos de una observación no archivada por id.
func (e *DbEngine) loadObsRow(id string) (obsRow, bool, error) {
	var r obsRow
	err := e.db.QueryRow(
		`SELECT id, topic_key, content, COALESCE(created_at,'') FROM observations WHERE id=? AND archived=0`,
		id,
	).Scan(&r.id, &r.topicKey, &r.content, &r.createdAt)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return obsRow{}, false, nil
		}
		return obsRow{}, false, fmt.Errorf("error al cargar observación %q: %w", id, err)
	}
	return r, true, nil
}

// conflictCandidates busca observaciones candidatas por FTS, excluyendo la propia,
// las archivadas y las ya superseded.
func (e *DbEngine) conflictCandidates(src obsRow, limit int) ([]obsRow, error) {
	ftsQuery := buildFTSQuery(src.content)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, o.content, COALESCE(o.created_at,'')
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ?
		  AND o.id != ?
		  AND o.archived = 0
		  AND o.superseded_by IS NULL
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, src.id, limit)
	if err != nil {
		return nil, fmt.Errorf("error al buscar candidatas de conflicto: %w", err)
	}
	defer rows.Close()
	var out []obsRow
	for rows.Next() {
		var r obsRow
		if err := rows.Scan(&r.id, &r.topicKey, &r.content, &r.createdAt); err != nil {
			return nil, fmt.Errorf("error al escanear candidata: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// markSuperseded marca una observación como reemplazada por otra (la oculta del
// recall sin borrarla).
func (e *DbEngine) markSuperseded(targetID, bySourceID string) error {
	if _, err := e.db.Exec(
		`UPDATE observations SET superseded_by=? WHERE id=?`, bySourceID, targetID,
	); err != nil {
		return fmt.Errorf("error al marcar superseded %q: %w", targetID, err)
	}
	return nil
}
