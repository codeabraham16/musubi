package memory

import (
	"database/sql"
	"fmt"

	"musubi/internal/redact"
)

// Federación de specs (spec-graph F4). Una spec marcada `shared: true` se proyecta al cerebro
// central como una observación DERIVADA que reusa el carril de observaciones (outbox, dead-letter,
// backfill, redacción forzada del central): no hay un segundo mecanismo de sync para specs (R5). La
// proyección NO es fuente de verdad —lo es el SPEC.md en el FS—; si divergen, gana el FS y se
// re-proyecta. Ver specs/spec-graph-f4-federacion/{spec,design}.md.

// specProjectionIDPrefix es el namespace del id determinista de una proyección de spec. El
// reconciliador (ReconcileSpecProjections) enumera las filas por este prefijo, y F5 podrá distinguir
// las specs de las observaciones por él SIN tocar el vocabulario de procedencia (D2/D4).
const specProjectionIDPrefix = "spec-proj:"

// specProjectionID deriva el id DETERMINISTA de la observación proyectada:
// spec-proj:<project_id>:<spec_id> (D2). Es determinista para que un update de la misma spec
// REEMPLACE la proyección por id (idempotente, R6) en vez de acumular duplicados; incluye el
// project_id para que dos repos con el mismo spec_id no colisionen en el central.
func specProjectionID(projectID, specID string) string {
	return specProjectionIDPrefix + projectID + ":" + specID
}

// specProjectionTopicKey es el topic_key de la proyección: spec/<spec_id> (R4). F5 lo usará para
// cruzar una spec con las observaciones que hablan de ella.
func specProjectionTopicKey(specID string) string {
	return "spec/" + specID
}

// ProjectSpecShared proyecta (o actualiza) la observación DERIVADA de una spec marcada `shared: true`
// hacia el carril de federación: UPSERT de la obs con id determinista + bump de sync_seq +
// enqueueOutboxTx, TODO en una transacción. Reusa EXACTAMENTE la cañería de observaciones (outbox,
// dead-letter, backfill, la redacción forzada del central): NO hay un segundo mecanismo de sync para
// specs (R5).
//
// Es un método DEDICADO —no un wrapper de saveObservation— a propósito (D5): una proyección es una
// fila derivada y reconstruible desde el FS, no texto humano, así que NO pasa por las guardas de
// observación humana (el sobre de procedencia, el dedup por content_hash). Su contrato propio: id
// determinista (D2); scope `shared`, mem_type `semantic`, procedencia `human`, confidence 1.0 (D4 —
// marcarla `llm:*` la mandaría a cuarentena y nunca se promovería a shared).
//
// El `content` ya viene renderizado de los campos PARSEADOS por el caller (D3), así que la huella de
// las anclas de F2 —que vive DENTRO del frontmatter— no entra: R8 (el fingerprint no viaja) es una
// garantía ESTRUCTURAL, no un filtro que alguien olvida al tocar el serializador. Igual se redacta en
// el borde (C2), como PromoteObservation: un secreto no debe cruzar a 'shared' por ninguna ruta.
//
// En un nodo TERMINAL (sin destino, e.outboxEnabled=false) el enqueue NO corre —igual que una
// observación shared—: la fila queda escrita (marca) y BackfillOutbox la siembra al ganar destino
// (R11/E5). Gatear SÓLO el enqueue, y no la escritura de la fila, es lo que le da algo que sembrar al
// backfill.
func (e *DbEngine) ProjectSpecShared(specID, projectID, content string, emb []float32) error {
	projectID = e.effectiveProjectID(projectID)
	id := specProjectionID(projectID, specID)

	// Redacción en el borde a 'shared' (C2), idempotente: el hash de algo ya limpio no cambia, así
	// que el outbox no re-encola de más. Gist/hash/tokens se derivan del texto YA limpio para que el
	// payload de sync nunca reconstruya un secreto.
	clean, _ := redact.Redact(content)
	gist := Gist(clean, defaultGistMaxTokens)
	hash := ContentHash(clean)
	tokens := EstimateTokens(clean)

	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción de proyección de spec: %w", err)
	}
	defer tx.Rollback()

	// UPSERT por id determinista: un update de la misma spec REEMPLAZA la fila (R6). El sync_seq se
	// bumpea PLEGADO en el mismo statement (atómico, como IngestShared): una fila jamás queda con
	// sync_seq=0 —que nunca es > cursor ⇒ invisible al pull— y el trigger FTS dispara una sola vez.
	// scope/mem_type/provenance/confidence son LITERALES fijos (el contrato de una proyección) y por
	// eso NO están en el DO UPDATE: nunca cambian. project_id tampoco se pisa: es parte de la clave.
	if _, err = tx.Exec(`INSERT INTO observations
		(id, topic_key, content, gist, content_hash, tokens, importance, mem_type, scope, project_id, provenance, confidence, sync_seq)
		VALUES (?, ?, ?, ?, ?, ?, 1.0, 'semantic', 'shared', ?, 'human', 1.0, (SELECT IFNULL(MAX(sync_seq),0)+1 FROM observations))
		ON CONFLICT(id) DO UPDATE SET
			topic_key=excluded.topic_key,
			content=excluded.content,
			gist=excluded.gist,
			content_hash=excluded.content_hash,
			tokens=excluded.tokens,
			sync_seq=(SELECT IFNULL(MAX(sync_seq),0)+1 FROM observations)`,
		id, specProjectionTopicKey(specID), clean, gist, hash, tokens, projectID); err != nil {
		return fmt.Errorf("error al proyectar la spec %q: %w", specID, err)
	}

	// El embedding es OPCIONAL (model-free): si el caller lo computó, se guarda keyed por la obs, con
	// la procedencia del vector (model_id de este engine) para no mezclar coseno entre modelos. Sin
	// embedder, la proyección igual federa por su gist (R7).
	if len(emb) > 0 {
		vec, err := Float32ToBytes(emb)
		if err != nil {
			return fmt.Errorf("error al serializar el vector de la proyección: %w", err)
		}
		if _, err = tx.Exec(`INSERT OR REPLACE INTO embeddings (observation_id, vector, model_id) VALUES (?, ?, ?)`,
			id, vec, e.vectorModelID); err != nil {
			return fmt.Errorf("error al guardar el embedding de la proyección: %w", err)
		}
	}

	// Encolar en el outbox dentro de la MISMA tx (la fila ya es 'shared', así que el INSERT..SELECT
	// de enqueueOutboxTx sí produce fila). Gateado por e.outboxEnabled: en un nodo terminal no corre
	// (R11) y la fila queda sólo marcada, a la espera del backfill. Idempotente por obs_id.
	if e.outboxEnabled {
		if err := enqueueOutboxTx(tx, id); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear la proyección de spec: %w", err)
	}
	return nil
}

// RetractSpecProjection retira LOCALMENTE la proyección de una spec: hard-delete de la obs derivada +
// su fila de outbox + su embedding. La proyección es reconstruible desde el FS, no fuente de verdad,
// así que borrarla es seguro: deja de empujarse y sale del recall local (R9). Lo llama el hook cuando
// una spec pasa a `shared: false`, se borra, o se le cambia el id (la vieja clave queda huérfana).
//
// Es LOCAL y durable ante el pull INCREMENTAL: des-compartir no empuja nada hacia arriba, así que la
// copia del central queda bajo el cursor y no vuelve (E4). El retiro MESH-WIDE —que el central deje
// de servir la copia a la malla— NO existe en F4 (R10b): el carril es upsert-only y fingir lo
// contrario sería una promesa falsa.
func (e *DbEngine) RetractSpecProjection(specID, projectID string) error {
	projectID = e.effectiveProjectID(projectID)
	id := specProjectionID(projectID, specID)

	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción de retiro de proyección: %w", err)
	}
	defer tx.Rollback()

	if err := deleteProjectionTx(tx, id); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el retiro de proyección: %w", err)
	}
	return nil
}

// deleteProjectionTx hard-deletea una proyección por su id determinista dentro de una tx: obs +
// outbox + embedding. Lo comparten RetractSpecProjection (retiro explícito del hook) y
// ReconcileSpecProjections (barrido de huérfanas de origen local). Sin FK con CASCADE, los tres
// borrados van EXPLÍCITOS o quedan huérfanos —un vector apuntando a una obs que ya no existe, una
// fila de outbox que el loop de sync intentaría empujar para siempre—. El FTS lo mantiene el trigger
// AFTER DELETE de observations. DELETE de una fila ausente es no-op, así que es idempotente.
func deleteProjectionTx(tx *sql.Tx, id string) error {
	if _, err := tx.Exec(`DELETE FROM embeddings WHERE observation_id = ?`, id); err != nil {
		return fmt.Errorf("error al borrar el embedding de la proyección %q: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM outbox WHERE obs_id = ?`, id); err != nil {
		return fmt.Errorf("error al borrar la fila de outbox de la proyección %q: %w", id, err)
	}
	if _, err := tx.Exec(`DELETE FROM observations WHERE id = ?`, id); err != nil {
		return fmt.Errorf("error al borrar la observación proyectada %q: %w", id, err)
	}
	return nil
}

// SpecProjectionInput describe una spec `shared` del FS para el reconciliador: su id, el proyecto al
// que pertenece y el contenido YA renderizado de los campos parseados (D3), para sembrar la
// proyección si falta. El caller (la capa de tools) la arma recorriendo el índice de specs del FS —
// internal/memory NO lee specgraph (frontera «core fiel · política en tools», D6): recibe el
// conjunto ya resuelto, que es la decisión de política «estas specs se federan».
type SpecProjectionInput struct {
	SpecID    string
	ProjectID string
	Content   string
	Embedding []float32
}

// ReconcileSpecProjections hace del conjunto de proyecciones de ORIGEN LOCAL una función pura del
// conjunto de specs `shared` en disco (R10): siembra las que faltan y hard-deletea las locales
// huérfanas (su spec fuente dejó de ser `shared`, desapareció, o cambió de id). Es idempotente y
// corre al abrir, AL LADO de BackfillOutbox; el call-site (capa de tools) le pasa las specs `shared`
// ya leídas del FS.
//
// Una proyección RECIBIDA no es suya para borrar. El discriminador tiene TRES capas, porque ninguna sola
// cubre todas las topologías (D7):
//  1. TENANT: el barrido sólo mira spec-proj:<tenant propio>:… El id lleva el tenant de ORIGEN (D2), así
//     que lo empujado por otra máquina a un central —o bajado por pull— cae fuera y ni se enumera.
//  2. RECEPTOR: en un nodo sin destino (e.outboxEnabled=false: el cerebro central) el barrido NO corre,
//     por si un tenant ajeno colisiona con el propio. Lo empujado entra sin outbox y sin sello espejo,
//     y antes se leía como huérfano local y se borraba en silencio en CADA arranque de `serve`.
//  3. SELLO: una espejo del MISMO tenant (re-onboarding desde cursor 0, E4) se deja; su retiro es
//     mesh-wide (R10b), fuera de F4.
//
// (La premisa vieja «una fila SIN outbox es de origen local» era FALSA para un central que recibió la fila.)
func (e *DbEngine) ReconcileSpecProjections(sharedSpecs []SpecProjectionInput) error {
	// expected: id determinista -> input, para las specs shared del FS. El project_id se resuelve acá
	// con la MISMA regla que ProjectSpecShared (effectiveProjectID), para que las claves coincidan.
	expected := make(map[string]SpecProjectionInput, len(sharedSpecs))
	for _, s := range sharedSpecs {
		expected[specProjectionID(e.effectiveProjectID(s.ProjectID), s.SpecID)] = s
	}

	// DEFENSA 1 — namespace del tenant PROPIO: el barrido SÓLO enumera spec-proj:<tenant propio>:… Una
	// proyección RECIBIDA (empujada por otra máquina a un central, o bajada por pull) lleva el tenant AJENO
	// en su id (D2) y cae FUERA de este prefijo, así que ni se considera huérfana. Es lo que salva al
	// central ENCADENADO: ahí lo empujado entra con outbox 'pending' (no 'espejo'), el filtro del sello no
	// lo protegería, pero el del tenant sí.
	ownPrefix := specProjectionIDPrefix + e.effectiveProjectID("") + ":"

	// Enumerar las proyecciones del tenant propio con su sello de outbox en un solo barrido. LEFT JOIN: una
	// proyección local sin fila de outbox (terminal, no encolada) trae sello '' y es de origen local.
	rows, err := e.db.Query(`SELECT o.id, COALESCE(b.status,'')
		FROM observations o LEFT JOIN outbox b ON b.obs_id = o.id
		WHERE o.id LIKE ? || '%'`, ownPrefix)
	if err != nil {
		return fmt.Errorf("error al enumerar proyecciones de spec: %w", err)
	}
	actualAll := make(map[string]struct{})
	var orphansLocal []string
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			rows.Close()
			return fmt.Errorf("error al leer una proyección de spec: %w", err)
		}
		actualAll[id] = struct{}{}
		if _, ok := expected[id]; ok {
			continue // la proyección sigue justificada por el FS
		}
		// Huérfana del tenant propio. DEFENSA 3 — el sello: una espejo del MISMO tenant (un re-onboarding
		// desde cursor 0 re-ingiere mis propias proyecciones viejas como espejo, E4) NO se toca; su retiro
		// sería mesh-wide (R10b). Las espejo de OTRO tenant ni llegan acá: las excluyó DEFENSA 1.
		if status != outboxEspejo {
			orphansLocal = append(orphansLocal, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("error al recorrer proyecciones de spec: %w", err)
	}
	rows.Close()

	// Barrido de huérfanas locales en una sola tx (atómico; cada DELETE es idempotente de todos modos).
	// DEFENSA 2 — gate del receptor: un nodo SIN destino de sync (e.outboxEnabled=false) es un receptor
	// TERMINAL (el cerebro central), al que otras máquinas le EMPUJAN proyecciones que entran sin outbox y
	// sin sello espejo. Ahí el barrido NO corre: aunque un tenant ajeno COLISIONE con el propio (mismo
	// project_id) y DEFENSA 1 no distinga, la fila empujada no se borra. El retiro local explícito
	// (unshare/delete/cambio-de-id) sigue por RetractSpecProjection, que no pasa por acá; lo único que el
	// terminal resigna es el barrido de huérfanas out-of-band, que recupera apenas gana destino.
	if e.outboxEnabled && len(orphansLocal) > 0 {
		tx, err := e.db.Begin()
		if err != nil {
			return fmt.Errorf("error al iniciar transacción de reconciliación: %w", err)
		}
		defer tx.Rollback()
		for _, id := range orphansLocal {
			if err := deleteProjectionTx(tx, id); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error al commitear la reconciliación: %w", err)
		}
	}

	// Siembra de las shared sin proyección de NINGÚN tipo (una espejo con ese id ya la cubre: no se
	// re-siembra ni se pisa). Cada una en su propia tx vía ProjectSpecShared, que ya gatea el enqueue
	// por e.outboxEnabled (nodo terminal: marca, no encola — R11).
	for id, s := range expected {
		if _, ok := actualAll[id]; ok {
			continue
		}
		if err := e.ProjectSpecShared(s.SpecID, s.ProjectID, s.Content, s.Embedding); err != nil {
			return fmt.Errorf("error al sembrar la proyección de la spec %q: %w", s.SpecID, err)
		}
	}
	return nil
}
