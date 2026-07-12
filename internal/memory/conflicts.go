package memory

import (
	"context"
	"fmt"
	"strings"
)

// conflicts.go implementa la detección de relaciones semánticas entre observaciones, 100%
// model-free. Al guardar una observación busca candidatas, mide su parecido y emite un veredicto
// HEURÍSTICO determinista. Usa DOS señales:
//
//   - lex: similitud LÉXICA (Jaccard de trigramas). Ve las palabras.
//   - cos: similitud SEMÁNTICA (coseno de los embeddings estáticos). Ve el significado.
//     Es OPCIONAL: sin embedder, o sin vector de la procedencia actual, el veredicto cae al
//     camino léxico histórico (bit-idéntico).
//
// EL AND-GATE Y POR QUÉ NO ES UN OR
//
// Los embeddings estáticos NO evalúan predicados: miden DE QUÉ se habla, no QUÉ se afirma.
// "usamos NordVPN" y "ya NO usamos NordVPN" tienen coseno ALTO. Si el coseno pudiera auto-resolver
// solo, un supersede automático OCULTARÍA memoria contradictoria EN SILENCIO — el peor modo de
// falla posible para una memoria.
//
// Por eso auto-resolver exige las DOS señales altas (lex Y cos); el coseno sólo CORROBORA, nunca
// decide solo. Como el auto-resolve conserva la condición léxica de siempre y le SUMA una, las
// auto-supresiones son por construcción un SUBCONJUNTO de las de antes: agregar el coseno NO PUEDE
// crear una supresión nueva. El coseno sólo puede (a) hacer VISIBLE como `pending` un duplicado que
// hoy es invisible, o (b) DEGRADAR a `pending` una auto-resolución que no corrobora.
//
// Tabla de verdad (cosFloor <= cos < cosAuto se lee "coseno medio"):
//
//	                   | cos ausente | cos bajo | cos medio | cos alto
//	lex >= auto        | auto (hist) | pending  | pending   | AUTO      <- única celda que auto-resuelve
//	floor <= lex < auto| pending     | pending  | pending   | pending
//	lex < floor        | ignorar     | ignorar  | pending * | pending * <- (*) el falso negativo que se cierra:
//	                                                                       el duplicado dicho con OTRAS palabras
//
// Lo que no se puede deducir sin LLM (contradicción, negación, supersesión real) es justamente lo
// que queda `pending` para que lo juzgue el agente vía musubi_judge. El dedup semántico NUNCA
// fusiona ni borra por su cuenta.

// Umbrales calibrados para Jaccard de trigramas de caracteres (corre bajo: dos
// frases casi idénticas dan ~0.8; relacionadas pero distintas ~0.35; ajenas ~0).
const (
	defaultSimilarityFloor      = 0.3
	defaultAutoResolveThreshold = 0.7
	defaultConflictCandidates   = 10
	// Umbrales de COSENO, MEDIDOS sobre 77.028 pares reales (393 observaciones), no estimados:
	//   - casi-duplicados (Jaccard >= 0.7): coseno ~0.991
	//   - NO relacionados (Jaccard < 0.3):  coseno p50=0.601, p99=0.786, MÁXIMO 0.884
	// La línea de base es ALTÍSIMA (~0.60): texto del mismo dominio comparte vocabulario y el
	// mean-pooling lo amplifica. Por eso los umbrales viven arriba de 0.85 y no donde uno esperaría.
	//
	// OJO: esta escala NO es la de memory.vector_floor (0.30). Allá se compara QUERY vs documento;
	// acá documento vs DOCUMENTO. Reusar el 0.30 del recall marcaría casi TODO como duplicado
	// (con coseno >= 0.75 ya aparecen 2.661 pares no relacionados; con >= 0.90, cero).
	defaultCosineFloor         = 0.85 // piso para que una candidata SEMÁNTICA entre como pending
	defaultCosineAutoThreshold = 0.90 // coseno mínimo para CORROBORAR una auto-resolución
)

// ConflictOptions parametriza la detección. Los ceros usan los defaults.
type ConflictOptions struct {
	SimilarityFloor      float64 // piso léxico (Jaccard) para considerar dos observaciones relacionadas
	AutoResolveThreshold float64 // léxico a partir del cual se auto-resuelve (supersede/related)
	CandidatePool        int     // candidatas a evaluar
	// CosineFloor <= 0 apaga el coseno ⇒ dedup léxico histórico (rollback). No se normaliza en
	// withDefaults justamente para que un 0 EXPLÍCITO se respete; el default lo pone la config.
	CosineFloor         float64
	CosineAutoThreshold float64
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
	if o.CosineAutoThreshold <= 0 {
		o.CosineAutoThreshold = defaultCosineAutoThreshold
	}
	return o
}

// cosineEnabled indica si el coseno participa del veredicto. CosineFloor <= 0 lo apaga.
func (o ConflictOptions) cosineEnabled() bool { return o.CosineFloor > 0 }

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

	// Vector de la observación fuente, con la procedencia ACTUAL. Ausente (embedder apagado, o sin
	// backfill) ⇒ srcVec == nil ⇒ pool y veredicto caen al camino léxico histórico.
	var srcVec []float32
	if opts.cosineEnabled() {
		srcVec, err = e.observationVector(obsID)
		if err != nil {
			return nil, err
		}
	}

	cands, err := e.conflictCandidates(src, srcVec, opts.CandidatePool)
	if err != nil {
		return nil, err
	}

	// Coseno contra CADA candidata del pool (no sólo contra las que trajo el vecino más cercano):
	// una candidata que vino por FTS puede no estar en el top-N vectorial, y sin su coseno no se
	// podría aplicar el AND-gate para DEGRADARLA. Ausente del mapa = sin vector = camino histórico.
	var cosines map[string]float64
	if srcVec != nil {
		ids := make([]string, 0, len(cands))
		for _, c := range cands {
			ids = append(ids, c.id)
		}
		cosines, err = e.candidateCosines(srcVec, ids)
		if err != nil {
			return nil, err
		}
	}

	var out []ObsRelation
	for _, c := range cands {
		lex := Similarity(src.content, c.content)
		var cos *float64
		if v, ok := cosines[c.id]; ok {
			cos = &v
		}
		if !relevantPair(lex, cos, opts) {
			continue
		}

		rel := decideRelation(src, c, lex, cos, opts)
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

// relevantPair decide si el par merece siquiera una relación. Con coseno: entra si CUALQUIERA de
// las dos señales pasa su piso — es la unión lo que hace VISIBLE al duplicado semántico (lex bajo,
// cos alto), que con el filtro léxico solo se descartaba en silencio. Sin coseno: filtro histórico.
func relevantPair(lex float64, cos *float64, opts ConflictOptions) bool {
	if lex >= opts.SimilarityFloor {
		return true
	}
	return cos != nil && *cos >= opts.CosineFloor
}

// pendingRel arma la relación que delega el juicio al agente (musubi_judge). La confianza es la
// señal MÁS FUERTE de las dos: es la que motiva mirar el par.
func pendingRel(src, cand obsRow, lex float64, cos *float64) ObsRelation {
	conf := lex
	if cos != nil && *cos > conf {
		conf = *cos
	}
	return ObsRelation{
		SourceID:   src.id,
		TargetID:   cand.id,
		Relation:   RelPending,
		Confidence: conf,
		Status:     RelStatusPending,
	}
}

// decideRelation aplica la heurística determinista a un par (src=recién guardada, candidato).
//
// cos == nil (sin embedder / sin vector de la procedencia actual) ⇒ camino LÉXICO histórico,
// bit-idéntico al de siempre. Con coseno, rige el AND-GATE: auto-resolver exige lex Y cos altos.
// Ver la tabla de verdad en el encabezado del archivo y el porqué (los estáticos no evalúan
// predicados: "usamos X" y "ya NO usamos X" tienen coseno alto).
func decideRelation(src, cand obsRow, lex float64, cos *float64, opts ConflictOptions) ObsRelation {
	autoThreshold := opts.AutoResolveThreshold

	if cos != nil {
		// AND-gate. El coseno CORROBORA; nunca decide solo.
		if lex >= autoThreshold && *cos < opts.CosineAutoThreshold {
			// Léxicamente casi idénticas pero el coseno NO corrobora: no auto-resolver. Degradar a
			// pending es el lado seguro del error — un pending de más cuesta la atención del agente;
			// una auto-supresión de más cuesta MEMORIA PERDIDA.
			return pendingRel(src, cand, lex, cos)
		}
		if lex < autoThreshold {
			// Incluye el caso nuevo (lex < floor pero cos >= cosFloor): el duplicado semántico dicho
			// con otras palabras, que hasta ahora era INVISIBLE. Lo juzga el agente.
			return pendingRel(src, cand, lex, cos)
		}
		// lex >= auto Y cos >= cosAuto: las dos señales corroboran ⇒ sigue al veredicto de siempre.
	}

	switch {
	case lex >= autoThreshold && src.topicKey == cand.topicKey && src.createdAt > cand.createdAt:
		// Mismo tema + casi duplicado + la recién guardada es ESTRICTAMENTE más
		// nueva: reemplaza a la anterior. Solo auto-supersede en esta dirección, así
		// nunca ocultamos la observación recién guardada ni contenido más nuevo.
		return ObsRelation{
			SourceID:   src.id,
			TargetID:   cand.id,
			Relation:   RelSupersedes,
			Confidence: lex,
			Status:     RelStatusResolved,
			ResolvedBy: "heuristic",
			Reason:     "mismo topic_key y similitud alta; la observación más reciente reemplaza a la anterior",
		}
	case lex >= autoThreshold && src.topicKey == cand.topicKey:
		// Mismo tema + alta similitud, pero la candidata es igual o más nueva: no
		// auto-ocultar nada; que el agente decida.
		return pendingRel(src, cand, lex, cos)
	case lex >= autoThreshold:
		// Alto solape pero distinto tema: relacionadas, sin contradicción deducible.
		return ObsRelation{
			SourceID:   src.id,
			TargetID:   cand.id,
			Relation:   RelRelated,
			Confidence: lex,
			Status:     RelStatusResolved,
			ResolvedBy: "heuristic",
			Reason:     "alto solape léxico; relacionadas",
		}
	default:
		// Similitud media: podría ser contradicción o complemento. No es deducible
		// sin entender el texto -> lo juzga el agente.
		return pendingRel(src, cand, lex, cos)
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

// observationVector trae el vector de una observación con la procedencia (model_id) ACTUAL.
// Devuelve nil SIN error si no hay: sin embedder, sin backfill, o el vector es de otro modelo. Ese
// filtro por model_id no es un detalle — comparar por coseno vectores de tablas distintas da números
// sin sentido; es el contrato de procedencia (F2.2) que el checksum del model_id (N1) volvió fiable.
func (e *DbEngine) observationVector(obsID string) ([]float32, error) {
	if e.vectorModelID == "" {
		return nil, nil
	}
	var b []byte
	err := e.db.QueryRow(
		`SELECT vector FROM embeddings WHERE observation_id = ? AND model_id = ?`,
		obsID, e.vectorModelID,
	).Scan(&b)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("error al leer el vector de %q: %w", obsID, err)
	}
	v, err := BytesToFloat32(b)
	if err != nil {
		return nil, nil // vector ilegible: degradar al camino léxico, no romper el save
	}
	return v, nil
}

// candidateCosines computa el coseno de srcVec contra CADA id del pool (los que tengan vector de la
// procedencia actual). Se traen los vectores y se computa exacto, en vez de reusar el ranking de
// SearchObservations, porque ése devuelve sólo el top-N: una candidata que entró por FTS podría no
// estar ahí, y sin su coseno no se podría aplicar el AND-gate para DEGRADARLA. El pool es chico
// (CandidatePool), así que el costo es despreciable. Un id ausente del mapa = sin vector.
func (e *DbEngine) candidateCosines(srcVec []float32, ids []string) (map[string]float64, error) {
	out := make(map[string]float64, len(ids))
	if len(ids) == 0 || srcVec == nil {
		return out, nil
	}
	// Trocear el IN(...) por el tope de parámetros de SQLite (el pool ya es chico, pero el
	// troceo lo hace robusto si CandidatePool se sube por config).
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids))
		batch := ids[start:end]
		ph := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]interface{}, 0, len(batch)+1)
		for _, id := range batch {
			args = append(args, id)
		}
		args = append(args, e.vectorModelID)
		rows, err := e.db.Query(
			`SELECT observation_id, vector FROM embeddings WHERE observation_id IN (`+ph+`) AND model_id = ?`, args...)
		if err != nil {
			return nil, fmt.Errorf("error al leer los vectores de las candidatas: %w", err)
		}
		for rows.Next() {
			var id string
			var b []byte
			if err := rows.Scan(&id, &b); err != nil {
				rows.Close()
				return nil, fmt.Errorf("error al escanear el vector de una candidata: %w", err)
			}
			v, err := BytesToFloat32(b)
			if err != nil {
				continue // vector ilegible ⇒ esa candidata cae al camino léxico
			}
			c, err := CosineSimilarity(srcVec, v)
			if err != nil {
				continue // dimensión incompatible ⇒ idem
			}
			out[id] = float64(c)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("error al iterar los vectores de las candidatas: %w", err)
		}
		rows.Close()
	}
	return out, nil
}

// conflictCandidates arma el pool de candidatas: la UNIÓN del pool léxico (FTS, como siempre) y el
// pool SEMÁNTICO (vecinos por coseno de srcVec). La unión es lo que hace VISIBLE al duplicado
// escrito con otras palabras: con sólo FTS nunca entraba al pool y por lo tanto nunca se detectaba
// (falso negativo silencioso). Con srcVec == nil devuelve exactamente el pool léxico de siempre.
// Excluye la propia observación, las archivadas y las superseded en ambos caminos.
func (e *DbEngine) conflictCandidates(src obsRow, srcVec []float32, limit int) ([]obsRow, error) {
	out, err := e.lexicalConflictCandidates(src, limit)
	if err != nil {
		return nil, err
	}
	if srcVec == nil {
		return out, nil
	}

	// Pool semántico. SearchObservations ya re-filtra archived/superseded contra SQLite.
	hits, err := e.SearchObservations(context.Background(), srcVec, limit)
	if err != nil {
		return nil, fmt.Errorf("error al buscar candidatas semánticas: %w", err)
	}
	have := make(map[string]bool, len(out)+1)
	have[src.id] = true // nunca la propia
	for _, c := range out {
		have[c.id] = true
	}
	var missing []string
	for _, h := range hits {
		if !have[h.ID] {
			missing = append(missing, h.ID)
			have[h.ID] = true
		}
	}
	for _, id := range missing {
		r, ok, err := e.loadObsRow(id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// lexicalConflictCandidates es el pool por FTS de siempre: excluye la propia, las archivadas y las
// ya superseded.
func (e *DbEngine) lexicalConflictCandidates(src obsRow, limit int) ([]obsRow, error) {
	ftsQuery := buildFTSQueryRanked(src.content)
	if ftsQuery == "" {
		return nil, nil
	}
	rows, err := e.db.Query(`
		SELECT o.id, o.topic_key, o.content, COALESCE(o.created_at,'')
		FROM observations_fts f
		JOIN observations o ON f.id = o.id
		WHERE observations_fts MATCH ?
		  AND o.id != ?
		  AND `+visibleObsPredicate+`
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar candidatas de conflicto: %w", err)
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
	// La superseded deja de ser elegible: sacarla del índice vectorial (el re-filtro
	// SQL ya la excluiría, pero esto evita que ocupe un slot de candidato).
	if e.index != nil {
		e.index.Remove(targetID)
	}
	return nil
}
