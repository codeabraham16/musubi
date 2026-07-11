package memory

import (
	"context"
	"fmt"

	"musubi/internal/redact"
)

// inboundsync.go implementa los PRIMITIVOS del sync ENTRANTE del cerebro híbrido (C5.3a): el espejo
// del outbox (saliente) en sentido de bajada. Un cliente en team mode baja periódicamente la memoria
// 'shared' de su proyecto DESDE el central y la ingiere localmente, para que su recall local
// (rápido, offline) la surfacee sin tocar el hot path del turno (decisión de arquitectura: pull, no
// recall federado en vivo — preserva local-first).
//
// Dos piezas, en los dos extremos del enlace:
//   - ListSharedForPull  → la corre el CENTRAL: devuelve la memoria shared del proyecto de la
//                          credencial (aislamiento T17-19), paginada por un cursor monótono (rowid).
//   - IngestShared       → la corre el CLIENTE: persiste una obs bajada SIN encolarla en el outbox
//                          local (la clave ANTI-LOOP: lo bajado del central no debe re-subirse).

// SharedObs es una observación 'shared' traída del central para el sync ENTRANTE. Lleva el rowid
// del central como cursor de paginación (el cliente guarda el mayor visto para la próxima página).
type SharedObs struct {
	RowID      int64
	ID         string
	TopicKey   string
	Content    string
	Importance float64
	MemType    string
	Author     string
	ProjectID  string
}

// ListSharedForPull devuelve hasta limit observaciones 'shared' visibles con rowid > afterRowID, en
// orden ascendente de rowid (cursor monótono y estable para paginar sin perder ni repetir filas),
// ACOTADAS al proyecto del ctx (aislamiento multi-tenant T17-19: el central sólo entrega la memoria
// del proyecto de la credencial que pide el pull; Federate/vacío ⇒ sin filtro, histórico). afterRowID=0
// trae desde el principio. La corre el central al servir un pull entrante de un cliente.
func (e *DbEngine) ListSharedForPull(ctx context.Context, afterRowID int64, limit int) ([]SharedObs, error) {
	if limit <= 0 {
		limit = 200
	}
	sc := projectScopeFrom(ctx)
	scopeSQL, scopeArgs := sc.scopeClause("")
	q := `SELECT rowid, id, topic_key, content, importance, COALESCE(mem_type,''), COALESCE(author,''), COALESCE(project_id,'')
		FROM observations
		WHERE ` + visibleObsPredicate + ` AND scope = 'shared' AND rowid > ?` + scopeSQL + `
		ORDER BY rowid ASC
		LIMIT ?`
	args := make([]interface{}, 0, len(scopeArgs)+2)
	args = append(args, afterRowID)
	args = append(args, scopeArgs...)
	args = append(args, limit)

	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar shared para pull: %w", err)
	}
	defer rows.Close()
	var out []SharedObs
	for rows.Next() {
		var o SharedObs
		if err := rows.Scan(&o.RowID, &o.ID, &o.TopicKey, &o.Content, &o.Importance, &o.MemType, &o.Author, &o.ProjectID); err != nil {
			return nil, fmt.Errorf("error al escanear shared para pull: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// IngestShared persiste una observación 'shared' bajada del central (sync ENTRANTE) SIN encolarla en
// el outbox local — la clave ANTI-LOOP: lo que bajé del central no debe re-subirse (si pasara por el
// enqueue normal, rebotaría). UPSERT por id (idempotente: re-ingerir la misma no duplica; el UPSERT
// preserva created_at y las stats de acceso). Preserva el project_id y el author de ORIGEN. Redacta
// el contenido por defensa en profundidad (el central ya redacta al ingerir; el borde a shared es
// donde vive la garantía). No indexa vector en este primitivo mínimo (ingest léxico; el FTS lo
// mantienen los triggers AFTER INSERT/UPDATE). Devuelve si insertó una fila nueva (vs. update).
func (e *DbEngine) IngestShared(o SharedObs) (inserted bool, err error) {
	clean, _ := redact.Redact(o.Content)
	gist := Gist(clean, defaultGistMaxTokens)
	hash := ContentHash(clean)
	tokens := EstimateTokens(clean)
	memType := normalizeMemType(o.MemType)

	var before int
	_ = e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id = ?`, o.ID).Scan(&before)

	// UPSERT espejo del de saveObservation, PERO sin enqueueOutboxTx (anti-loop) y forzando
	// scope='shared' (viene del pozo compartido). project_id/author se estampan del ORIGEN y no se
	// pisan en updates (misma disciplina que la atribución local).
	_, err = e.db.Exec(`INSERT INTO observations
		(id, topic_key, content, gist, content_hash, tokens, importance, mem_type, scope, project_id, author)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'shared', ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic_key=excluded.topic_key,
			content=excluded.content,
			gist=excluded.gist,
			content_hash=excluded.content_hash,
			tokens=excluded.tokens,
			importance=excluded.importance,
			mem_type=CASE WHEN excluded.mem_type != '' THEN excluded.mem_type ELSE observations.mem_type END`,
		o.ID, o.TopicKey, clean, gist, hash, tokens, o.Importance, memType, o.ProjectID, o.Author)
	if err != nil {
		return false, fmt.Errorf("error al ingerir observación shared: %w", err)
	}
	return before == 0, nil
}
