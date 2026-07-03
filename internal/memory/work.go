package memory

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// work.go implementa la PIZARRA COMPARTIDA del multi-agente: una cola de unidades
// de trabajo agrupadas en batches. El agente principal descompone una tarea y
// postea las unidades; cada sub-agente reclama una (claim ATÓMICO, sin colisiones),
// la ejecuta y deja el resultado. El principal consolida. Es 100% model-free: la
// inteligencia (descomponer, trabajar, consolidar) es de Claude; Musubi solo
// garantiza coordinación determinista.
//
// LEASE/TTL (v0.59+): cada claim toma un LEASE con vencimiento (patrón visibility
// timeout de SQS / lease de Chubby). Si el dueño no lo renueva (heartbeat) dentro
// de la ventana, la unidad se vuelve reclamable de nuevo — así un agente que crashea
// no bloquea la unidad para siempre. El reciclado es LAZY: ocurre en el próximo claim,
// sin goroutine barredora. Un FENCING TOKEN monótono defiende del "worker zombie"
// (un agente lento que revive tras ser expropiado y quiere escribir con un token
// viejo: su UPDATE afecta 0 filas). Semántica at-least-once → el trabajo debe ser
// idempotente.

// Estados de una unidad de trabajo.
const (
	WorkOpen    = "open"    // disponible para reclamar
	WorkClaimed = "claimed" // tomada por un agente, en curso (con lease)
	WorkDone    = "done"    // completada con éxito
	WorkFailed  = "failed"  // completada con fallo (o dead-letter por reintentos agotados)
)

// defaultLeaseTTLSeconds y defaultMaxAttempts son los fallbacks si el llamador pasa
// valores no positivos (espejan los defaults de config.MultiAgentConfig).
const (
	defaultLeaseTTLSeconds = 300
	defaultMaxAttempts     = 5
)

// WorkUnit es una unidad de trabajo de la pizarra.
type WorkUnit struct {
	ID        string `json:"id"`
	BatchID   string `json:"batch_id"`
	Seq       int    `json:"seq"`
	Title     string `json:"title"`
	Spec      string `json:"spec"`
	Status    string `json:"status"`
	ClaimedBy string `json:"claimed_by,omitempty"`
	Result    string `json:"result,omitempty"`
	// Campos de lease (v0.59+).
	OwnerID        string `json:"owner_id,omitempty"`         // dueño canónico del lease
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"` // vencimiento (UTC ISO)
	Attempts       int    `json:"attempts,omitempty"`         // reclamos acumulados
	FencingToken   int64  `json:"fencing_token,omitempty"`    // token monótono anti-zombie
}

// WorkUnitSpec describe una unidad a crear.
type WorkUnitSpec struct {
	Title string `json:"title"`
	Spec  string `json:"spec"`
}

// WorkBatch es el estado consolidado de un batch.
type WorkBatch struct {
	BatchID string     `json:"batch_id"`
	Total   int        `json:"total"`
	Open    int        `json:"open"`
	Claimed int        `json:"claimed"`
	Done    int        `json:"done"`
	Failed  int        `json:"failed"`
	Units   []WorkUnit `json:"units"`
}

// workUnitCols es la lista de columnas leídas al escanear una WorkUnit completa
// (mismo orden que scanWorkUnit). Incluye los campos de lease.
const workUnitCols = `id, batch_id, seq, COALESCE(title,''), COALESCE(spec,''), status, ` +
	`COALESCE(claimed_by,''), COALESCE(result,''), COALESCE(owner_id,''), ` +
	`COALESCE(lease_expires_at,''), COALESCE(attempts,0), COALESCE(fencing_token,0)`

// scanWorkUnit escanea una fila con las columnas de workUnitCols en una WorkUnit.
func scanWorkUnit(s interface{ Scan(...interface{}) error }) (WorkUnit, error) {
	var u WorkUnit
	err := s.Scan(&u.ID, &u.BatchID, &u.Seq, &u.Title, &u.Spec, &u.Status,
		&u.ClaimedBy, &u.Result, &u.OwnerID, &u.LeaseExpiresAt, &u.Attempts, &u.FencingToken)
	return u, err
}

// CreateWorkBatch postea un batch de unidades. Si batchID == "" genera uno.
func (e *DbEngine) CreateWorkBatch(batchID string, specs []WorkUnitSpec) (WorkBatch, error) {
	if len(specs) == 0 {
		return WorkBatch{}, fmt.Errorf("un batch necesita al menos una unidad")
	}
	if batchID == "" {
		batchID = uuid.NewString()
	}
	// Transaccional: o se postean todas las unidades o ninguna (sin huérfanas si
	// falla a mitad de camino).
	tx, err := e.db.Begin()
	if err != nil {
		return WorkBatch{}, fmt.Errorf("error al iniciar la transacción del batch: %w", err)
	}
	for i, s := range specs {
		if _, err := tx.Exec(
			`INSERT INTO work_units (id, batch_id, seq, title, spec, status) VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), batchID, i, s.Title, s.Spec, WorkOpen,
		); err != nil {
			tx.Rollback()
			return WorkBatch{}, fmt.Errorf("error al crear unidad %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return WorkBatch{}, fmt.Errorf("error al confirmar el batch: %w", err)
	}
	return e.WorkBatchStatus(batchID)
}

// ClaimWorkUnit reclama atómicamente la próxima unidad reclamable y le asigna un
// lease con vida ttlSeconds. Reclamable = una unidad `open` O una `claimed` cuyo lease
// venció (huérfana). Si batchID != "" se limita a ese batch. ok=false si no hay ninguna
// disponible. El UPDATE...RETURNING en una sola sentencia es atómico: dos claims
// concurrentes nunca toman la misma unidad.
//
// Antes del claim, dead-letterea (status=failed) las huérfanas cuyo `attempts` ya
// alcanzó maxAttempts, para no reciclar indefinidamente una unidad que siempre falla.
// Al reclamar incrementa `attempts` y `fencing_token`, y devuelve la unidad con el
// FencingToken vigente (el dueño lo usa en heartbeat/complete).
func (e *DbEngine) ClaimWorkUnit(batchID, agent string, ttlSeconds, maxAttempts int) (WorkUnit, bool, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = defaultLeaseTTLSeconds
	}
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}

	// Paso 1: dead-letter de huérfanas agotadas. Va ANTES del claim; como work_units
	// vive en una base SQLite single-writer, no hay interleaving entre ambos statements.
	deadLetter := `
		UPDATE work_units
		   SET status=?, result=COALESCE(NULLIF(result,''), ?), updated_at=datetime('now')
		 WHERE status=?
		   AND lease_expires_at IS NOT NULL AND lease_expires_at < datetime('now')
		   AND attempts >= ?`
	dlArgs := []interface{}{WorkFailed, "lease agotado: superó el máximo de reintentos", WorkClaimed, maxAttempts}
	if batchID != "" {
		deadLetter += ` AND batch_id=?`
		dlArgs = append(dlArgs, batchID)
	}
	if _, err := e.db.Exec(deadLetter, dlArgs...); err != nil {
		return WorkUnit{}, false, fmt.Errorf("error al dead-letter huérfanas: %w", err)
	}

	// Paso 2: claim atómico. Elegible = open OR huérfana (claimed con lease vencido).
	leaseExpr := `datetime('now','+' || ? || ' seconds')`
	eligible := `(status=? OR (status=? AND lease_expires_at IS NOT NULL AND lease_expires_at < datetime('now')))`
	setClause := `status=?, owner_id=?, claimed_by=?, lease_expires_at=` + leaseExpr +
		`, heartbeat_at=datetime('now'), attempts=attempts+1, fencing_token=fencing_token+1, updated_at=datetime('now')`

	var row *sql.Row
	if batchID == "" {
		row = e.db.QueryRow(`
			UPDATE work_units SET `+setClause+`
			WHERE id = (SELECT id FROM work_units WHERE `+eligible+` ORDER BY created_at, seq, rowid LIMIT 1)
			RETURNING `+workUnitCols,
			WorkClaimed, agent, agent, ttlSeconds, WorkOpen, WorkClaimed)
	} else {
		row = e.db.QueryRow(`
			UPDATE work_units SET `+setClause+`
			WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND `+eligible+` ORDER BY seq LIMIT 1)
			RETURNING `+workUnitCols,
			WorkClaimed, agent, agent, ttlSeconds, batchID, WorkOpen, WorkClaimed)
	}
	u, err := scanWorkUnit(row)
	if err == sql.ErrNoRows {
		return WorkUnit{}, false, nil
	}
	if err != nil {
		return WorkUnit{}, false, fmt.Errorf("error al reclamar unidad: %w", err)
	}
	return u, true, nil
}

// HeartbeatWorkUnit renueva el lease de una unidad reclamada, extendiendo su
// vencimiento a now + ttlSeconds. Solo tiene efecto si la unidad sigue `claimed` y
// `owner` es su dueño actual; si se pasa fencingToken > 0, además debe coincidir con
// el token vigente. ok=false (sin error) significa que fuiste EXPROPIADO o ya no sos
// el dueño: el agente debe DETENER el trabajo (otro lo tomó).
func (e *DbEngine) HeartbeatWorkUnit(id, owner string, fencingToken int64, ttlSeconds int) (bool, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = defaultLeaseTTLSeconds
	}
	query := `UPDATE work_units
	             SET lease_expires_at=datetime('now','+' || ? || ' seconds'),
	                 heartbeat_at=datetime('now'), updated_at=datetime('now')
	           WHERE id=? AND status=? AND owner_id=?`
	args := []interface{}{ttlSeconds, id, WorkClaimed, owner}
	if fencingToken > 0 {
		query += ` AND fencing_token=?`
		args = append(args, fencingToken)
	}
	res, err := e.db.Exec(query, args...)
	if err != nil {
		return false, fmt.Errorf("error al renovar el lease: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al verificar el heartbeat: %w", err)
	}
	return n > 0, nil
}

// CompleteWorkUnit cierra una unidad con su resultado. status debe ser done o
// failed (vacío = done). Si agent != "", exige además que sea el DUEÑO actual
// (owner_id), de modo que un agente expropiado no cierre trabajo que ya no es suyo.
// Si fencingToken > 0, exige que coincida con el token vigente: esto defiende del
// "worker zombie" incluso cuando dos agentes comparten el mismo id (owner_id no los
// distingue, pero el token monótono sí).
func (e *DbEngine) CompleteWorkUnit(id, result, status, agent string, fencingToken int64) error {
	if status == "" {
		status = WorkDone
	}
	if status != WorkDone && status != WorkFailed {
		return fmt.Errorf("status de cierre inválido %q (usá done|failed)", status)
	}
	// Guarda de estado: solo una unidad RECLAMADA puede cerrarse. Evita cerrar una
	// open nunca reclamada y re-cerrar/sobrescribir una ya done/failed.
	query := `UPDATE work_units SET status=?, result=?, updated_at=datetime('now') WHERE id=? AND status=?`
	args := []interface{}{status, result, id, WorkClaimed}
	if agent != "" {
		query += ` AND owner_id=?`
		args = append(args, agent)
	}
	if fencingToken > 0 {
		query += ` AND fencing_token=?`
		args = append(args, fencingToken)
	}
	res, err := e.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error al completar unidad: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al verificar unidad completada: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("la unidad %q no existe, no está reclamada, fue expropiada (lease vencido y retomada por otro), o el fencing token no coincide", id)
	}
	return nil
}

// WorkBatchStatus devuelve el estado consolidado de un batch (conteos + unidades).
func (e *DbEngine) WorkBatchStatus(batchID string) (WorkBatch, error) {
	rows, err := e.db.Query(`SELECT `+workUnitCols+` FROM work_units WHERE batch_id=? ORDER BY seq`, batchID)
	if err != nil {
		return WorkBatch{}, fmt.Errorf("error al leer el batch: %w", err)
	}
	defer rows.Close()

	b := WorkBatch{BatchID: batchID, Units: []WorkUnit{}}
	for rows.Next() {
		u, err := scanWorkUnit(rows)
		if err != nil {
			return WorkBatch{}, fmt.Errorf("error al escanear unidad: %w", err)
		}
		b.Units = append(b.Units, u)
		b.Total++
		switch u.Status {
		case WorkOpen:
			b.Open++
		case WorkClaimed:
			b.Claimed++
		case WorkDone:
			b.Done++
		case WorkFailed:
			b.Failed++
		}
	}
	if err := rows.Err(); err != nil {
		return WorkBatch{}, fmt.Errorf("error al iterar unidades del batch: %w", err)
	}
	return b, nil
}

// ClearWorkBatch elimina todas las unidades de un batch.
func (e *DbEngine) ClearWorkBatch(batchID string) error {
	if _, err := e.db.Exec(`DELETE FROM work_units WHERE batch_id=?`, batchID); err != nil {
		return fmt.Errorf("error al limpiar el batch: %w", err)
	}
	return nil
}

// ActiveBatch devuelve el batch con trabajo pendiente (open/claimed) más reciente,
// para el recordatorio por turno. ok=false si no hay ninguno en curso.
func (e *DbEngine) ActiveBatch() (WorkBatch, bool, error) {
	var batchID string
	err := e.db.QueryRow(`
		SELECT batch_id FROM work_units
		WHERE status IN (?, ?)
		ORDER BY created_at DESC, rowid DESC LIMIT 1`, WorkOpen, WorkClaimed).Scan(&batchID)
	if err == sql.ErrNoRows {
		return WorkBatch{}, false, nil
	}
	if err != nil {
		return WorkBatch{}, false, fmt.Errorf("error al buscar batch activo: %w", err)
	}
	b, err := e.WorkBatchStatus(batchID)
	if err != nil {
		return WorkBatch{}, false, err
	}
	return b, true, nil
}
