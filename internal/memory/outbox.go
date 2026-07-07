package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

// outbox.go implementa el OUTBOX DURABLE del cerebro híbrido F2: el registro persistente
// de las observaciones 'shared' que hay que empujar al cerebro central, con la maquinaria
// de claim/lease/backoff/dead-letter del drain offline-first. El outbox es el patrón
// TRANSACCIONAL canónico: el enqueue ocurre en la MISMA tx que promueve/guarda a 'shared'
// (durable y atómico con el cambio de estado), y el drain (en internal/mcp) lo consume con
// reintentos. El outbox NO copia el contenido: guarda sólo obs_id + metadatos; el payload se
// reconstruye con un JOIN a observations al drenar, así siempre entrega el contenido fresco.
// Ver migración v11 (outbox) y las decisiones D1-D9 del design de F2.

// Estados canónicos de una fila del outbox.
//
//	pending -> encolada, lista para reclamar cuando next_attempt_at <= now
//	claimed -> reclamada por un ciclo de drain, con lease en next_attempt_at (futuro)
//	sent    -> entregada al central con éxito (no se re-entrega)
//	dead    -> dead-letter: fallo permanente o tope de reintentos (no se reintenta)
const (
	outboxPending = "pending"
	outboxClaimed = "claimed"
	outboxSent    = "sent"
	outboxDead    = "dead"
)

// OutboxItem es una unidad de entrega ya lista para empujar al central: el obs_id (que
// también es el id JSON-RPC, para idempotencia end-to-end) más el payload reconstruido
// desde observations al reclamar el batch. Attempts es el contador de intentos de entrega
// YA fallidos de esta fila (no viaja en el payload): lo usa el drain para decidir el backoff
// y el corte a dead-letter (attempts+1 >= max_attempts) sin un round-trip extra a la DB.
type OutboxItem struct {
	ObsID      string
	TopicKey   string
	Content    string
	Importance float64
	MemType    string
	ProjectID  string
	Attempts   int
}

// enqueueOutboxTx encola (o re-encola) la observación obsID en el outbox, DENTRO de la tx
// del caller (misma atomicidad que el cambio de scope). Un único statement INSERT..SELECT..
// ON CONFLICT parametrizado por obs_id:
//   - Si la observación NO es 'shared', el SELECT no produce fila → no-op (barato para el
//     caso común 'local': el enqueue es incondicional a nivel engine, ver D6).
//   - Si es 'shared' y no había fila → INSERT pending.
//   - Si ya había fila con el MISMO content_hash → no-op (idempotencia, R3).
//   - Si el content_hash CAMBIÓ → vuelve a pending con attempts reseteado (re-sync, R3).
//
// El WHERE del ON CONFLICT usa `IS NOT` (no `!=`) para tratar NULL correctamente.
func enqueueOutboxTx(tx *sql.Tx, obsID string) error {
	_, err := tx.Exec(`
		INSERT INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at)
		SELECT id, content_hash, 'pending', 0, datetime('now'), datetime('now'), datetime('now')
		FROM observations WHERE id = ? AND scope = 'shared'
		ON CONFLICT(obs_id) DO UPDATE SET
			status = 'pending', attempts = 0, next_attempt_at = datetime('now'),
			enqueued_hash = excluded.enqueued_hash, last_error = NULL, updated_at = datetime('now')
		WHERE outbox.enqueued_hash IS NOT excluded.enqueued_hash`, obsID)
	if err != nil {
		return fmt.Errorf("error al encolar en outbox: %w", err)
	}
	return nil
}

// BackfillOutbox siembra idempotentemente una fila pending por cada observación 'shared'
// que todavía no tiene fila de outbox. Es la red de seguridad para las 'shared' creadas en
// F1 antes de que existiera el outbox (R4), y para las promovidas mientras el sync estaba
// apagado. Devuelve cuántas filas sembró. Idempotente: un segundo llamado no duplica (el
// NOT EXISTS filtra las ya sembradas). No re-encola las 'sent'/'dead' (sólo siembra faltantes).
func (e *DbEngine) BackfillOutbox() (int, error) {
	res, err := e.db.Exec(`
		INSERT INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at)
		SELECT o.id, o.content_hash, 'pending', 0, datetime('now'), datetime('now'), datetime('now')
		FROM observations o
		WHERE o.scope = 'shared'
		  AND NOT EXISTS (SELECT 1 FROM outbox b WHERE b.obs_id = o.id)`)
	if err != nil {
		return 0, fmt.Errorf("error al sembrar el outbox: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer filas sembradas en el outbox: %w", err)
	}
	return int(n), nil
}

// ClaimOutboxBatch reclama atómicamente hasta limit filas vencidas y devuelve sus payloads
// listos para push. El claim es un único UPDATE..RETURNING (SQLite 3.35+, igual que
// AwardWorkUnit): marca 'claimed' y posterga next_attempt_at leaseSeconds al futuro (el
// lease), de modo que otro ciclo/proceso no reclame las mismas filas dentro de la ventana
// (R5) y que un claim colgado (crash del drain) se auto-recupere al vencer el lease (D4).
// Claimable = status IN (pending, claimed) AND next_attempt_at <= now. Tras el claim se
// cargan los payloads con un SELECT a observations por los obs_ids devueltos (D2).
func (e *DbEngine) ClaimOutboxBatch(limit, leaseSeconds int) ([]OutboxItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 60
	}
	rows, err := e.db.Query(`
		UPDATE outbox
		SET status = 'claimed',
		    next_attempt_at = datetime('now', '+' || ? || ' seconds'),
		    updated_at = datetime('now')
		WHERE id IN (
			SELECT id FROM outbox
			WHERE status IN ('pending','claimed') AND next_attempt_at <= datetime('now')
			ORDER BY next_attempt_at
			LIMIT ?
		)
		RETURNING obs_id, attempts`, leaseSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("error al reclamar batch del outbox: %w", err)
	}
	var ids []string
	attemptsByID := map[string]int{}
	for rows.Next() {
		var id string
		var attempts int
		if err := rows.Scan(&id, &attempts); err != nil {
			rows.Close()
			return nil, fmt.Errorf("error al escanear obs_id reclamado: %w", err)
		}
		ids = append(ids, id)
		attemptsByID[id] = attempts
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("error al iterar el batch reclamado: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}
	return e.loadOutboxPayloads(ids, attemptsByID)
}

// loadOutboxPayloads reconstruye el payload de cada obs_id reclamado desde observations. Si
// una observación fue borrada tras el claim (raro), simplemente no aparece en el resultado
// (su fila de outbox seguirá 'claimed' hasta que venza el lease y se re-evalúe). El orden de
// salida respeta el de ids para entregar aproximadamente FIFO.
func (e *DbEngine) loadOutboxPayloads(ids []string, attemptsByID map[string]int) ([]OutboxItem, error) {
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `SELECT id, topic_key, content, COALESCE(importance, 1.0), COALESCE(mem_type, ''), COALESCE(project_id, '')
		FROM observations WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al cargar payloads del outbox: %w", err)
	}
	defer rows.Close()
	byID := make(map[string]OutboxItem, len(ids))
	for rows.Next() {
		var it OutboxItem
		if err := rows.Scan(&it.ObsID, &it.TopicKey, &it.Content, &it.Importance, &it.MemType, &it.ProjectID); err != nil {
			return nil, fmt.Errorf("error al escanear payload del outbox: %w", err)
		}
		it.Attempts = attemptsByID[it.ObsID]
		byID[it.ObsID] = it
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar payloads del outbox: %w", err)
	}
	items := make([]OutboxItem, 0, len(byID))
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			items = append(items, it)
		}
	}
	return items, nil
}

// MarkOutboxSent marca la fila 'sent' tras una entrega exitosa (R13). No se re-entrega.
func (e *DbEngine) MarkOutboxSent(obsID string) error {
	if _, err := e.db.Exec(`
		UPDATE outbox SET status = 'sent', last_error = NULL, updated_at = datetime('now')
		WHERE obs_id = ?`, obsID); err != nil {
		return fmt.Errorf("error al marcar outbox como enviado: %w", err)
	}
	return nil
}

// MarkOutboxRetry devuelve la fila a 'pending' tras un fallo transitorio (R11): incrementa
// attempts y posterga next_attempt_at backoffSeconds al futuro (backoff), guardando el error.
func (e *DbEngine) MarkOutboxRetry(obsID string, backoffSeconds int, errMsg string) error {
	if backoffSeconds < 0 {
		backoffSeconds = 0
	}
	if _, err := e.db.Exec(`
		UPDATE outbox
		SET status = 'pending',
		    attempts = attempts + 1,
		    next_attempt_at = datetime('now', '+' || ? || ' seconds'),
		    last_error = ?,
		    updated_at = datetime('now')
		WHERE obs_id = ?`, backoffSeconds, errMsg, obsID); err != nil {
		return fmt.Errorf("error al reprogramar reintento en outbox: %w", err)
	}
	return nil
}

// MarkOutboxDead manda la fila a dead-letter (R12): fallo permanente o tope de reintentos.
// No se reintenta automáticamente; queda como registro de auditoría con last_error.
func (e *DbEngine) MarkOutboxDead(obsID, errMsg string) error {
	if _, err := e.db.Exec(`
		UPDATE outbox SET status = 'dead', last_error = ?, updated_at = datetime('now')
		WHERE obs_id = ?`, errMsg, obsID); err != nil {
		return fmt.Errorf("error al marcar outbox como dead: %w", err)
	}
	return nil
}

// OutboxStats devuelve el conteo por estado relevante (pending incluye claimed, que es una
// pending en vuelo). Para tests y observabilidad.
func (e *DbEngine) OutboxStats() (pending, sent, dead int, err error) {
	rows, qerr := e.db.Query(`SELECT status, COUNT(*) FROM outbox GROUP BY status`)
	if qerr != nil {
		return 0, 0, 0, fmt.Errorf("error al consultar estadísticas del outbox: %w", qerr)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if serr := rows.Scan(&status, &n); serr != nil {
			return 0, 0, 0, fmt.Errorf("error al escanear estadísticas del outbox: %w", serr)
		}
		switch status {
		case outboxPending, outboxClaimed:
			pending += n
		case outboxSent:
			sent += n
		case outboxDead:
			dead += n
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return 0, 0, 0, fmt.Errorf("error al iterar estadísticas del outbox: %w", rerr)
	}
	return pending, sent, dead, nil
}

// OutboxHealthReport es el estado del sync saliente para observabilidad (musubi_sync_status):
// counts por estado, antigüedad de la observación pendiente más vieja, y el último error visto.
type OutboxHealthReport struct {
	Pending             int    `json:"pending"`
	Sent                int    `json:"sent"`
	Dead                int    `json:"dead"`
	OldestPendingAgeSec int64  `json:"oldest_pending_age_seconds"`
	LastError           string `json:"last_error"`
}

// OutboxHealth resume la salud del outbox del cerebro híbrido para el tool musubi_sync_status.
// pending incluye claimed (una pendiente en vuelo). OldestPendingAgeSec son segundos desde la
// fila pending/claimed más vieja (0 si no hay). LastError es el más reciente entre las filas NO
// enviadas (pending/claimed/dead), por updated_at. Con outbox vacío devuelve ceros sin error.
func (e *DbEngine) OutboxHealth() (OutboxHealthReport, error) {
	var h OutboxHealthReport
	p, s, d, err := e.OutboxStats()
	if err != nil {
		return h, err
	}
	h.Pending, h.Sent, h.Dead = p, s, d

	var age sql.NullInt64
	if err := e.db.QueryRow(
		`SELECT CAST(strftime('%s','now') - strftime('%s', MIN(created_at)) AS INTEGER)
		 FROM outbox WHERE status IN (?, ?)`, outboxPending, outboxClaimed,
	).Scan(&age); err != nil && err != sql.ErrNoRows {
		return h, fmt.Errorf("error al calcular antigüedad del outbox: %w", err)
	}
	if age.Valid && age.Int64 > 0 {
		h.OldestPendingAgeSec = age.Int64
	}

	var last sql.NullString
	if err := e.db.QueryRow(
		`SELECT last_error FROM outbox
		 WHERE last_error IS NOT NULL AND status != ?
		 ORDER BY updated_at DESC LIMIT 1`, outboxSent,
	).Scan(&last); err != nil && err != sql.ErrNoRows {
		return h, fmt.Errorf("error al leer el último error del outbox: %w", err)
	}
	if last.Valid {
		h.LastError = last.String
	}
	return h, nil
}

// RequeueDeadOutbox devuelve TODAS las filas dead-letter a 'pending' (attempts=0, listas para
// drenar), limpiando last_error. Es la red de seguridad manual (musubi_sync_requeue) tras un
// corte del central o de la VPN. Idempotente: sin filas dead devuelve 0 sin error.
func (e *DbEngine) RequeueDeadOutbox() (int, error) {
	res, err := e.db.Exec(`
		UPDATE outbox
		SET status = 'pending', attempts = 0, next_attempt_at = datetime('now'),
		    last_error = NULL, updated_at = datetime('now')
		WHERE status = 'dead'`)
	if err != nil {
		return 0, fmt.Errorf("error al re-encolar dead-letter del outbox: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer filas re-encoladas: %w", err)
	}
	return int(n), nil
}
