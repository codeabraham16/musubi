package memory

import (
	"context"
	"database/sql"
	"errors"
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
	// RowID es el CURSOR de sync del protocolo entrante. Desde la migración v19 (auditoría #4) lleva el
	// valor de sync_seq (monótono, sube en cada insert/update, estable ante VACUUM), NO el rowid crudo.
	// El nombre del campo se conserva por compat del wire (JSON "rowid") — la semántica es "cursor".
	RowID      int64   `json:"rowid"`
	ID         string  `json:"id"`
	TopicKey   string  `json:"topic_key"`
	Content    string  `json:"content"`
	Importance float64 `json:"importance"`
	MemType    string  `json:"mem_type"`
	Author     string  `json:"author"`
	ProjectID  string  `json:"project_id"`
}

// ListSharedForPull devuelve hasta limit observaciones 'shared' visibles con rowid > afterRowID, en
// orden ascendente de rowid (cursor monótono y estable para paginar sin perder ni repetir filas),
// ACOTADAS al proyecto del ctx (aislamiento multi-tenant T17-19: el central sólo entrega la memoria
// del proyecto de la credencial que pide el pull; Federate/vacío ⇒ sin filtro, histórico). afterRowID=0
// trae desde el principio. La corre el central al servir un pull entrante de un cliente.
//
// LIMITACIÓN VIEJA, CERRADA: este comentario decía que el cursor era por `rowid`, que no cambia en un
// UPDATE. Ya no: la consulta de abajo pagina por `sync_seq`, que sube también al actualizar, y el
// nombre `afterRowID` quedó sólo por compat del wire (ver SharedObs).
//
// 🔴 LIMITACIÓN NUEVA, ABIERTA, Y ES PEOR: EL FILTRO DE PROYECTO NO OCULTA FILAS, LAS SALTA.
//
// El acotamiento por tenant (scopeSQL) entra al MISMO WHERE que `sync_seq > ?`, y el LIMIT se aplica
// DESPUÉS. O sea que el filtro no acorta la página: la corre hacia arriba por encima de las filas
// ajenas. Y `toolSyncPull` (internal/mcp/methods.go) calcula el `next_cursor` con el máximo de las
// filas que DEVOLVIÓ —ya filtradas—, así que todo lo que el filtro descartó queda DEBAJO de ese
// cursor sin haberse entregado. El cliente lo adopta y `AvanzarCursorBajada` (bajada_lease.go) no
// retrocede nunca: su UPSERT lleva `WHERE ... < ...`. Nada lo rebobina —`musubi_sync_requeue` toca
// el dead-letter del OUTBOX, no la bajada—, así que la única salida es editar `meta` a mano.
//
// EL DAÑO SE COBRA CUANDO LA CREDENCIAL SE ENSANCHA. Mientras el token es read:own la fila ajena no
// le corresponde y no falta. Cuando pasa a read:all el filtro desaparece, pero el cursor ya está
// arriba: esa historia no vuelve JAMÁS.
//
// MEDIDO EL 2026-09-24 sobre la base real del central cruzada contra la de davantis-1:
//
//	3.073 filas pullable en el central · 3.009 presentes acá · 64 AUSENTES
//	las 64 con sync_seq <= cursor (10.990) · 0 por encima · mayor seq ausente: 855
//	`altura` 61 de 61 ausentes bajo seq 855, y 0 de 643 por encima. `last-chaos` 1 de 1.
//	(2 de `musubi` se cuentan aparte: pueden ser un borrado en duro local)
//
// La prueba de que es el cursor y no otra cosa es el ENTRELAZADO: seq 709 presente, 711-717 ausentes,
// 718 PRESENTE, 719 y 721 ausentes, 722 presente. `sync_seq` se asigna MAX+1 al insertar, así que ese
// orden es el de llegada: cuando el pull entregó la 718, las 711-717 ya existían y no vinieron.
//
// ⚠️ Y OJO CON CÓMO SE VERIFICA, porque acá me equivoqué antes: comparar el cursor contra
// `max(sync_seq)` del central y verlo al día NO prueba nada. El cursor llegando arriba es
// exactamente el síntoma — avanzó por encima de lo que no entregó. La prueba honesta es cruzar los
// ids del universo pullable del central contra los de la base local.
func (e *DbEngine) ListSharedForPull(ctx context.Context, afterRowID int64, limit int) ([]SharedObs, error) {
	if limit <= 0 {
		limit = 200
	}
	sc := projectScopeFrom(ctx)
	scopeSQL, scopeArgs := sc.scopeClause("")
	// Se pagina por sync_seq (no por rowid): sync_seq SUBE también en un UPDATE (auditoría #4), así que
	// una edición de una obs shared ya sincronizada se re-entrega; y es estable ante VACUUM (rowid no).
	// El valor de sync_seq viaja en el campo RowID (es el cursor del protocolo; ver SharedObs).
	q := `SELECT sync_seq, id, topic_key, content, importance, COALESCE(mem_type,''), COALESCE(author,''), COALESCE(project_id,'')
		FROM observations
		WHERE ` + visibleObsPredicate + ` AND scope = 'shared' AND sync_seq > ?` + scopeSQL + `
		ORDER BY sync_seq ASC
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

	// Una sola transacción para leer el contenido previo, pisarlo e invalidar el vector: el DSN
	// lleva _txlock=immediate, así que el lock de escritura se toma al nacer y nadie puede editar la
	// fila entre la lectura y el UPSERT.
	tx, err := e.db.Begin()
	if err != nil {
		return false, fmt.Errorf("error al abrir la transacción de ingest shared: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var previo string
	existia := true
	switch err := tx.QueryRow(`SELECT content FROM observations WHERE id = ?`, o.ID).Scan(&previo); {
	case errors.Is(err, sql.ErrNoRows):
		existia = false
	case err != nil:
		return false, fmt.Errorf("error al leer el contenido previo de %s: %w", o.ID, err)
	}

	// UPSERT espejo del de saveObservation, PERO sin enqueueOutboxTx (anti-loop) y forzando
	// scope='shared' (viene del pozo compartido). project_id/author se estampan del ORIGEN y no se
	// pisan en updates (misma disciplina que la atribución local).
	//
	// ATOMICIDAD DEL sync_seq (auditoría v0.98.0): el bump va PLEGADO en el mismo statement (subselect
	// MAX+1 tanto en el INSERT como en el DO UPDATE), no en un segundo Exec autocommit. Antes, un
	// crash entre el UPSERT y el bump dejaba la fila con sync_seq=0 — que nunca es > cursor ⇒
	// INVISIBLE para siempre al pull (importa cuando este cliente a su vez sirve pulls: nodo
	// relay/equipo). Plegarlo lo hace atómico (una sola fila jamás queda con sync_seq=0) y de paso
	// dispara el trigger FTS una vez en vez de dos. El subselect en el DO UPDATE ve el sync_seq viejo
	// de la propia fila, así que MAX+1 es un bump monótono estricto. Single-writer ⇒ sin race.
	// ACÁ NO VA LA GUARDA DE `SobreDeLlamadaComido`, Y ES UNA DECISIÓN, NO UN OLVIDO.
	//
	// Esta puerta no CREA memoria: relaya la que otro nodo ya guardó. El cerebro central tiene 73
	// observaciones con el sobre comido de antes de la guarda (medidas el 2026-09-04), así que
	// rechazarlas acá no arreglaría ninguna — rompería el pull del nodo que las tiene, y una
	// sincronización que se corta por una fila vieja pierde TODO lo que venía detrás.
	//
	// La guarda vive donde nace el contenido (saveObservation, ver sobre_de_llamada.go). Si algún
	// día se quiere limpiar el pasado, el lugar es una reparación explícita sobre las filas
	// existentes —que además tiene que devolverle a cada una la importance que declaró— y no un
	// rechazo en el camino de relay.
	_, err = tx.Exec(`INSERT INTO observations
		(id, topic_key, content, gist, content_hash, tokens, importance, mem_type, scope, project_id, author, sync_seq)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'shared', ?, ?, (SELECT IFNULL(MAX(sync_seq),0)+1 FROM observations))
		ON CONFLICT(id) DO UPDATE SET
			topic_key=excluded.topic_key,
			content=excluded.content,
			gist=excluded.gist,
			content_hash=excluded.content_hash,
			tokens=excluded.tokens,
			importance=excluded.importance,
			mem_type=CASE WHEN excluded.mem_type != '' THEN excluded.mem_type ELSE observations.mem_type END,
			sync_seq=(SELECT IFNULL(MAX(sync_seq),0)+1 FROM observations)`,
		o.ID, o.TopicKey, clean, gist, hash, tokens, o.Importance, memType, o.ProjectID, o.Author)
	if err != nil {
		return false, fmt.Errorf("error al ingerir observación shared: %w", err)
	}

	// ⚠️ UNA EDICIÓN QUE BAJA DEL CENTRAL DEJABA EL VECTOR DEL CONTENIDO VIEJO. El central sube el
	// sync_seq en cada update, así que una fila editada vuelve a bajar y este UPSERT le pisa content
	// y content_hash; pero la fila de embeddings seguía con el vector del texto anterior y el
	// model_id ACTUAL, que no entra en stalePredicate: ni el incremental ni el backfill la volvían a
	// ver, y la búsqueda semántica la rankeaba por lo que ya no dice. Si el contenido cambió, el
	// vector se borra en la MISMA transacción y la fila queda pendiente: el relleno que dispara el
	// pull la re-embebe con el texto nuevo. Si no cambió (re-entrega, o sólo cambió la importancia),
	// el vector sigue valiendo y no se toca: re-embeber de más cuesta un pedido por fila y deja la
	// fila fuera del recall semántico hasta que vuelva.
	//
	// Se compara el contenido y no content_hash porque el vector se calcula del contenido: es la
	// entrada exacta, sin depender de que el hash previo de la fila esté cargado.
	// EL SELLO DE ORIGEN, Y ES LA MITAD QUE FALTABA DEL ANTI-LOOP. No encolar no alcanzaba: la
	// ausencia de fila es indistinguible de «una shared vieja que nunca se encoló», que es
	// justamente lo que BackfillOutbox viene a rescatar, y una apertura después la ponía a la cola.
	// Dejar una fila terminal 'espejo' es lo que convierte el silencio en una afirmación: el central
	// ya tiene esto. Va en la MISMA transacción que el UPSERT, porque un crash entre las dos dejaría
	// la observación sin sello y el próximo backfill la volvería a subir.
	//
	// enqueued_hash lleva el hash del contenido YA redactado, o sea lo que esta base tiene ahora.
	// Así, si mañana se edita acá, enqueueOutboxTx compara contra esto, ve el cambio y la encola. Y
	// se REFRESCA en cada re-entrega (DO UPDATE), porque si no una edición que baja del central
	// dejaría el sello apuntando a un contenido que ya no existe y el próximo enqueue subiría de más.
	//
	// Y el DO UPDATE NO pisa una fila 'pending'/'claimed': ésa es una intención de envío LOCAL que
	// todavía no salió, y sellarla como espejo la mataría en silencio. Sobre 'sent'/'dead'/'espejo'
	// sí escribe, que son estados terminales donde el sello sólo agrega información.
	//
	// ⚠️ Lo que este arreglo NO toca: el UPSERT de arriba igual pisa el CONTENIDO local con el del
	// central (último que escribe gana). Eso es el diseño declarado del enlace, no un descuido de
	// acá, y cambiarlo es otra discusión.
	if _, err := tx.Exec(`
		INSERT INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at)
		VALUES (?, ?, 'espejo', 0, datetime('now'), datetime('now'), datetime('now'))
		ON CONFLICT(obs_id) DO UPDATE SET
			status = 'espejo', attempts = 0, last_error = NULL,
			enqueued_hash = excluded.enqueued_hash, updated_at = datetime('now')
		WHERE outbox.status NOT IN ('pending','claimed')`,
		o.ID, hash); err != nil {
		return false, fmt.Errorf("error al sellar como espejo la obs bajada %s: %w", o.ID, err)
	}

	cambio := existia && previo != clean
	if cambio {
		if _, err := tx.Exec(`DELETE FROM embeddings WHERE observation_id = ?`, o.ID); err != nil {
			return false, fmt.Errorf("error al invalidar el vector viejo de %s: %w", o.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("error al commitear el ingest shared de %s: %w", o.ID, err)
	}
	if cambio && e.index != nil {
		// Post-commit, como en saveObservation: el IVF no guarda un candidato muerto. La
		// correctitud ya la da el JOIN contra embeddings; esto es precisión del recall.
		e.index.Remove(o.ID)
	}
	return !existia, nil
}
