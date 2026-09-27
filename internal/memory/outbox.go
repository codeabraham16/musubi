package memory

import (
	"context"
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
//	espejo  -> BAJÓ del central: ya está allá, no hay nada que enviar (ver abajo)
//
// 'espejo' NO es un estado de envío: es el sello de que esta fila entró por el sync ENTRANTE. Existe
// porque el anti-loop de IngestShared —no encolar lo bajado— se verificaba en un solo instante y
// BackfillOutbox lo deshacía en la apertura siguiente: siembra una 'pending' por cada 'shared' SIN
// fila, y lo bajado del central es exactamente eso. Dejando el sello, el `NOT EXISTS` del backfill ya
// no la ve y el `status IN ('pending','claimed')` del claim tampoco.
//
// Es un estado terminal como 'sent', y por el mismo motivo: el central tiene el contenido. La
// diferencia es de dirección, y se guarda aparte para poder MEDIR el eco en vez de confiar en que no
// volvió. No es una mordaza: si después se edita la observación acá, el content_hash cambia y el
// ON CONFLICT de enqueueOutboxTx la devuelve a 'pending' como a cualquier otra.
const (
	outboxPending = "pending"
	outboxClaimed = "claimed"
	outboxSent    = "sent"
	outboxDead    = "dead"
	outboxEspejo  = "espejo"
)

// OutboxItem es una unidad de entrega ya lista para empujar al central: el obs_id (que
// también es el id JSON-RPC, para idempotencia end-to-end) más el payload reconstruido
// desde observations al reclamar el batch. Attempts es el contador de intentos de entrega
// YA fallidos de esta fila (no viaja en el payload): lo usa el drain para decidir el backoff y
// para la observabilidad, SIN UN ROUND-TRIP EXTRA a la DB. No corta nada: una fila muere sólo por
// un fallo PERMANENTE, y eso lo decide el código de error. La regla está escrita una sola vez, en
// config.SyncConfig.MaxAttempts.
//
// Hash es el content_hash de ESTE Content, leído en el mismo SELECT que el payload, y lo que el
// drain le devuelve a las marcas (MarkOutboxSent/Retry/Dead). No viaja: dice QUÉ versión salió, y
// con eso una marca no confunde la versión que empujó con una edición local que llegó mientras
// viajaba. Se lee junto con el contenido y no de enqueued_hash porque tiene que nombrar lo que se
// empujó: el claim y la carga del payload son dos sentencias, y una edición entre las dos cambia el
// contenido que sale.
//
// Reclamo es el next_attempt_at que escribió EL claim que trajo este ítem, tal cual quedó en la base:
// el fin de su lease. Tampoco viaja. Sirve de ficha del claim para ReclamoVigente: si la fila ya no
// tiene esa marca, la reclamó otro drainer (venció el lease y la volvió a tomar) o se editó, y el
// payload que este ítem lleva ya no es el que hay que subir.
type OutboxItem struct {
	ObsID      string
	TopicKey   string
	Content    string
	Importance float64
	MemType    string
	ProjectID  string
	Attempts   int
	Hash       string
	Reclamo    string
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
		    next_attempt_at = `+leaseMs+`,
		    updated_at = datetime('now')
		WHERE id IN (
			SELECT id FROM outbox
			WHERE `+porSubir+`
			ORDER BY next_attempt_at
			LIMIT ?
		)
		RETURNING obs_id, attempts, CAST(next_attempt_at AS TEXT)`, leaseSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("error al reclamar batch del outbox: %w", err)
	}
	var ids []string
	attemptsByID := map[string]int{}
	reclamoByID := map[string]string{}
	for rows.Next() {
		var id, reclamo string
		var attempts int
		if err := rows.Scan(&id, &attempts, &reclamo); err != nil {
			rows.Close()
			return nil, fmt.Errorf("error al escanear obs_id reclamado: %w", err)
		}
		ids = append(ids, id)
		attemptsByID[id] = attempts
		reclamoByID[id] = reclamo
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("error al iterar el batch reclamado: %w", err)
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, nil
	}
	items, err := e.loadOutboxPayloads(ids, attemptsByID)
	for i := range items {
		items[i].Reclamo = reclamoByID[items[i].ObsID]
	}
	return items, err
}

// ahoraMs es «ahora» CON MILISEGUNDOS. El lease del claim se escribe y se compara con esto, y no con
// datetime('now'), porque datetime trunca al segundo: un lease de N segundos escrito así vencía entre
// N-1 y N segundos después del claim, según en qué fracción del segundo cayera. Con lease_seconds=1
// eso es un lease de CERO a un segundo, y una fila reclamada a las hh:mm:ss.95 volvía a ser
// reclamable 50 ms después, con su push todavía en vuelo: el envío doble que este lease existe para
// impedir. Con milisegundos el lease dura lo que dice.
//
// Convive con los valores de segundo pelado que escriben el enqueue, el backfill y el reintento: el
// texto «AAAA-MM-DD hh:mm:ss» es prefijo de «AAAA-MM-DD hh:mm:ss.mmm», así que compara como el
// primer milisegundo de ese segundo, que es lo que significaba.
const ahoraMs = `strftime('%Y-%m-%d %H:%M:%f', 'now')`

// leaseMs es el fin del lease de un claim: ahoraMs más los segundos del parámetro.
const leaseMs = `strftime('%Y-%m-%d %H:%M:%f', 'now', '+' || ? || ' seconds')`

// porSubir es lo que un claim puede tomar: pendiente o reclamada con el lease vencido, y ya en hora.
// Es UNA sola condición para el claim y para el sondeo (HayOutboxPorSubir) a propósito: si el sondeo
// mirara otra cosa, podría ver trabajo que el claim no toma —y drenar en vacío cada dos segundos— o
// no ver el que sí toma.
const porSubir = `status IN ('pending','claimed') AND next_attempt_at <= ` + ahoraMs

// ReclamoVigente dice si la fila de obsID sigue reclamada por EL claim que trajo el ítem (reclamo es
// su OutboxItem.Reclamo) y con el mismo contenido (hash). El drain la pregunta justo antes de cada
// push, y si la respuesta es no, ese payload no sale.
//
// Existe por la carrera de dos drainers sobre la misma base (varios daemons por base en davantis-1):
// el claim carga el payload de todo el sublote al principio, y si mientras se empujan los primeros
// se edita uno de los de atrás, la edición deja la fila 'pending' y otro daemon la reclama y sube v2.
// Sin esta pregunta el primero seguía con su lista y subía v1 DESPUÉS: el central quedaba con v1, la
// fila local 'sent' por v2, y la bajada siguiente le pisaba v2 también acá. Lo mismo si el lease
// venció y otro drainer ya la tomó: sin mirar, salía dos veces.
//
// Es una LECTURA: no toma el candado de escritura, que con seis daemons por base es lo que se paga.
// Lo que queda entre esta lectura y el POST son microsegundos, y lo que ya salió no lo detiene nada
// del lado del cliente (ver el comentario de drainOutboxOnce).
//
// Un ítem sin Reclamo (armado a mano, no por un claim) no tiene con qué compararse y se da por
// vigente: es el comportamiento de antes de esta pregunta.
func (e *DbEngine) ReclamoVigente(obsID, hash, reclamo string) (bool, error) {
	if reclamo == "" {
		return true, nil
	}
	var vigente bool
	if err := e.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM outbox
		WHERE obs_id = ? AND status = 'claimed' AND next_attempt_at = ? AND ? = `+hashActual+`)`,
		obsID, reclamo, hash).Scan(&vigente); err != nil {
		return false, fmt.Errorf("error al verificar el claim de %s: %w", obsID, err)
	}
	return vigente, nil
}

// HayOutboxPorSubir es el SONDEO del drain: si hay alguna fila que un claim tomaría ahora mismo. Lo
// corre RunOutboxScheduler cada pocos segundos entre ticks para que una nota escrita por OTRO proceso
// sobre la misma base —la captura, los hooks, otro daemon— no espere al tick siguiente.
//
// Es una lectura sobre idx_outbox_claim (status, next_attempt_at) y nada más: no abre transacción,
// así que no toma el candado de escritura, que en esta base comparten varios daemons; y no sale a la
// red. Si hay algo, el que sale es el drain de siempre.
func (e *DbEngine) HayOutboxPorSubir() (bool, error) {
	var hay bool
	fila := e.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM outbox WHERE ` + porSubir + `)`)
	if err := fila.Scan(&hay); err != nil {
		return false, fmt.Errorf("error al sondear el outbox: %w", err)
	}
	return hay, nil
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
	q := `SELECT id, topic_key, content, COALESCE(importance, 1.0), COALESCE(mem_type, ''), COALESCE(project_id, ''),
			COALESCE(content_hash, '')
		FROM observations WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al cargar payloads del outbox: %w", err)
	}
	defer rows.Close()
	byID := make(map[string]OutboxItem, len(ids))
	for rows.Next() {
		var it OutboxItem
		if err := rows.Scan(&it.ObsID, &it.TopicKey, &it.Content, &it.Importance, &it.MemType, &it.ProjectID, &it.Hash); err != nil {
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

// hashActual es el content_hash que la observación tiene AHORA, leído igual que OutboxItem.Hash
// (un NULL vale la cadena vacía). Las tres marcas comparan contra esto el hash de lo que salió, y
// NO contra outbox.enqueued_hash.
//
// Las dos columnas pueden no coincidir en una fila en vuelo, y con la marca contra enqueued_hash
// esa fila no se cerraba NUNCA: el push salía bien, la marca no aplicaba, vencía el lease y se
// volvía a empujar cada LeaseSeconds para siempre. Y se veía sana: last_error vacío, sent_at al
// día y un rebote por vuelta. Medido en la revisión: 4 saves al central en 4 ticks, donde main
// cerraba la fila con el primer 200 porque marcaba sin mirar. Se llega ahí por dos caminos de
// producción: un binario anterior a este arreglo que baja otra versión sobre una edición pendiente
// (su UPSERT pisa el contenido y su sello no toca la 'pending'), y una 'pending' que la retención
// dejó huérfana —el outbox no tiene FK— y que el central re-entrega con otro contenido.
//
// Contra lo que hay, la entrega de lo que la base tiene ahora cierra la fila aunque lo encolado
// diga otra cosa, y una edición que llegó mientras el push viajaba —que cambia content_hash— sigue
// sin darse por entregada.
const hashActual = `(SELECT COALESCE(o.content_hash, '') FROM observations o WHERE o.id = outbox.obs_id)`

// MarkOutboxSent marca la fila 'sent' tras una entrega exitosa (R13). No se re-entrega.
//
// hash es el de lo que SALIÓ (OutboxItem.Hash). La fila queda 'sent' sólo si eso es lo que la
// observación tiene ahora (hashActual): una edición local que llegó mientras el push viajaba ya
// cambió el contenido y devolvió la fila a 'pending' (enqueueOutboxTx), y marcarla 'sent' daba por
// entregada una versión que nunca salió —medido: claim de v1, edición v2, 200 de v1, y la fila en
// 'sent' con 0 filas por reenviar—. Así queda 'pending' y la versión nueva sale en el próximo tick.
//
// Al quedar 'sent' re-sella enqueued_hash con lo que salió, que es contra lo que enqueueOutboxTx
// mide la próxima edición: si quedaba el de una deriva, re-guardar el mismo contenido la volvía a
// encolar y la subía otra vez. Y borra last_error sólo entonces: si lo que salió era otra versión,
// el error es de la que sigue pendiente —con dos drainers, el intento fallido de v2 llega antes que
// el 200 tardío de v1— y es lo que musubi_sync_status muestra de lo que está trabado.
//
// Deja escrito además QUÉ salió y CUÁNDO (migración v58): sent_hash es el hash que esta máquina
// entregó por última vez y sent_at la hora de esa entrega. «Enviadas en 24 h» se cuenta con sent_at
// y no con el estado, porque el estado lo mueven otros caminos (una edición local la devuelve a
// 'pending'); la hora de la última salida no la escribe nadie más que esta marca.
//
// ESO SE ANOTA AUNQUE LA FILA NO QUEDE 'sent', y es a propósito: la versión vieja SÍ salió, y el
// central la va a devolver en la bajada. IngestShared distingue ese rebote propio de un choque con
// otra máquina comparando contra sent_hash; si la entrega en vuelo no se anotara, el rebote más
// común —el de la versión que viajaba mientras se editaba— se contaría como choque.
//
// Cierra también una fila 'pending' con el MISMO contenido que salió, y es a propósito: esa versión
// ya está en el central. Se llega ahí sin error de nadie: al drainer se le venció el lease con el
// push en vuelo, otro la reclamó y su intento falló (MarkOutboxRetry la dejó 'pending' con backoff),
// y recién después volvió este 200. Dejarla abierta la subía otra vez igual: un save de más en el
// central, un embedding de más y un eco para todas las máquinas.
func (e *DbEngine) MarkOutboxSent(obsID, hash string) error {
	// Fencing por estado (auditoría #13c): una marca sólo aplica a una fila NO terminal (pending/claimed),
	// nunca a una 'sent'/'dead'. Sin esto, con dos drainers solapados por vencimiento de lease, un ciclo
	// rezagado podía re-marcar una fila que otro ya resolvió (p. ej. un MarkRetry tardío revivía un 'sent'
	// a 'pending' = phantom pending). Excluir los estados terminales corta esa resurrección.
	if _, err := e.db.Exec(`
		UPDATE outbox SET
			status = CASE WHEN ? = `+hashActual+` THEN 'sent' ELSE status END,
			enqueued_hash = CASE WHEN ? = `+hashActual+` THEN ? ELSE enqueued_hash END,
			last_error = CASE WHEN ? = `+hashActual+` THEN NULL ELSE last_error END,
			updated_at = datetime('now'),
			sent_hash = ?, sent_at = datetime('now')
		WHERE obs_id = ? AND status IN ('pending','claimed')`, hash, hash, hash, hash, hash, obsID); err != nil {
		return fmt.Errorf("error al marcar outbox como enviado: %w", err)
	}
	return nil
}

// MarkOutboxRetry devuelve la fila a 'pending' tras un fallo transitorio (R11): incrementa
// attempts y posterga next_attempt_at backoffSeconds al futuro (backoff), guardando el error.
//
// Sólo si lo que falló (hash) es lo que la observación tiene ahora (hashActual): una edición que
// llegó mientras viajaba ya dejó la fila 'pending' para salir YA, y el backoff y el intento fallido
// son de la versión vieja.
func (e *DbEngine) MarkOutboxRetry(obsID, hash string, backoffSeconds int, errMsg string) error {
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
		WHERE obs_id = ? AND status IN ('pending','claimed') AND ? = `+hashActual, backoffSeconds, errMsg, obsID, hash); err != nil {
		return fmt.Errorf("error al reprogramar reintento en outbox: %w", err)
	}
	return nil
}

// MarkOutboxDead manda la fila a dead-letter (R12): fallo permanente o tope de reintentos.
// No se reintenta automáticamente; queda como registro de auditoría con last_error.
//
// Sólo si lo rechazado (hash) es lo que la observación tiene ahora (hashActual): el central rechazó
// la versión que viajaba, no la edición que llegó después, y matarla acá la dejaba sin salir nunca.
func (e *DbEngine) MarkOutboxDead(obsID, hash, errMsg string) error {
	if _, err := e.db.Exec(`
		UPDATE outbox SET status = 'dead', last_error = ?, updated_at = datetime('now')
		WHERE obs_id = ? AND status IN ('pending','claimed') AND ? = `+hashActual, errMsg, obsID, hash); err != nil {
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
	Pending int `json:"pending"`
	Sent    int `json:"sent"`
	Dead    int `json:"dead"`
	// Espejo son las que BAJARON del central y por eso no se envían. Se reporta porque un arreglo
	// que sólo deja de hacer algo es invisible: sin este número no hay manera de distinguir «el eco
	// se frenó» de «el sello no se está poniendo». Es la cuenta de lo que NO se re-subió.
	Espejo              int    `json:"espejo"`
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

	// El espejo va aparte y no por OutboxStats: ese devuelve los tres contadores de ENVÍO y los
	// firma su nombre. Meterle un cuarto valor cambiaría la firma de todos sus callers para que
	// ninguno lo use más que éste.
	if err := e.db.QueryRow(
		`SELECT COUNT(*) FROM outbox WHERE status = ?`, outboxEspejo,
	).Scan(&h.Espejo); err != nil && err != sql.ErrNoRows {
		return h, fmt.Errorf("error al contar las filas espejo del outbox: %w", err)
	}

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

// OutboxHealthCtx es OutboxHealth ACOTADO al proyecto del ctx, para musubi_sync_status.
//
// El outbox no tiene project_id: el proyecto de cada fila es el de su observación, así que el
// recorte va por JOIN, con la misma scopeClause que el resto de las lecturas. Sin él, en un nodo
// que sirve a credenciales de varios proyectos, una credencial leía los conteos del outbox ajeno y
// el TEXTO de su último error, que puede nombrar la nota rechazada. Hoy es latente —el central es
// nodo terminal y su outbox está vacío— pero la tool ya se declara aislada.
//
// Sin recorte (el daemon local, sin credencial, o un admin federado) delega en OutboxHealth tal
// cual: el JOIN dejaría afuera las filas cuya observación se borró, y eso le cambiaría los números
// a quien hoy los mira.
func (e *DbEngine) OutboxHealthCtx(ctx context.Context) (OutboxHealthReport, error) {
	scope, args := projectScopeFrom(ctx).scopeClause("o")
	if scope == "" {
		return e.OutboxHealth()
	}
	var h OutboxHealthReport
	const desde = ` FROM outbox b JOIN observations o ON o.id = b.obs_id WHERE 1 = 1`
	rows, err := e.db.QueryContext(ctx, `SELECT b.status, COUNT(*)`+desde+scope+` GROUP BY b.status`, args...)
	if err != nil {
		return h, fmt.Errorf("error al consultar el outbox del proyecto: %w", err)
	}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return h, fmt.Errorf("error al escanear el outbox del proyecto: %w", err)
		}
		switch status {
		case outboxPending, outboxClaimed:
			h.Pending += n
		case outboxSent:
			h.Sent += n
		case outboxDead:
			h.Dead += n
		case outboxEspejo:
			h.Espejo += n
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return h, fmt.Errorf("error al recorrer el outbox del proyecto: %w", err)
	}
	rows.Close()

	var age sql.NullInt64
	if err := e.db.QueryRowContext(ctx,
		`SELECT CAST(strftime('%s','now') - strftime('%s', MIN(b.created_at)) AS INTEGER)`+desde+
			` AND b.status IN (?, ?)`+scope, append([]interface{}{outboxPending, outboxClaimed}, args...)...,
	).Scan(&age); err != nil && err != sql.ErrNoRows {
		return h, fmt.Errorf("error al calcular la antigüedad del outbox del proyecto: %w", err)
	}
	if age.Valid && age.Int64 > 0 {
		h.OldestPendingAgeSec = age.Int64
	}

	var last sql.NullString
	if err := e.db.QueryRowContext(ctx,
		`SELECT b.last_error`+desde+` AND b.last_error IS NOT NULL AND b.status != ?`+scope+
			` ORDER BY b.updated_at DESC LIMIT 1`, append([]interface{}{outboxSent}, args...)...,
	).Scan(&last); err != nil && err != sql.ErrNoRows {
		return h, fmt.Errorf("error al leer el último error del outbox del proyecto: %w", err)
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

// PurgeOutboxPending BORRA las filas pending/claimed del outbox y devuelve cuántas. Es la limpieza
// del NODO TERMINAL: un cerebro que SIRVE sin sync saliente (central_url vacío / sync.enabled=false)
// no tiene upstream a dónde empujar, así que una fila 'pending' ahí es basura INMORTAL — encolada
// por un binario viejo (antes del gate SetOutboxEnabled) o por una config que después apagó el sync.
// A diferencia de RequeueDeadOutbox (que REACTIVA para reintentar), esto DESCARTA: sólo lo llama el
// arranque cuando la config confirma que el destino no existe. NO toca observations — el contenido
// queda intacto; se descarta únicamente el intento de envío. Idempotente: sin filas devuelve 0.
func (e *DbEngine) PurgeOutboxPending() (int, error) {
	res, err := e.db.Exec(`DELETE FROM outbox WHERE status IN (?, ?)`, outboxPending, outboxClaimed)
	if err != nil {
		return 0, fmt.Errorf("error al purgar pendientes del outbox: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al contar filas purgadas del outbox: %w", err)
	}
	return int(n), nil
}
