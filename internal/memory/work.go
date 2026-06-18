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

// Estados de una unidad de trabajo.
const (
	WorkOpen    = "open"    // disponible para reclamar
	WorkClaimed = "claimed" // tomada por un agente, en curso
	WorkDone    = "done"    // completada con éxito
	WorkFailed  = "failed"  // completada con fallo
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

// ClaimWorkUnit reclama atómicamente la próxima unidad open. Si batchID != "" se
// limita a ese batch; si está vacío, toma de cualquier batch. ok=false si no hay
// ninguna disponible. El UPDATE...RETURNING en una sola sentencia es atómico:
// dos claims concurrentes nunca toman la misma unidad.
func (e *DbEngine) ClaimWorkUnit(batchID, agent string) (WorkUnit, bool, error) {
	var (
		row  *sql.Row
		cols = `id, batch_id, seq, COALESCE(title,''), COALESCE(spec,''), status, COALESCE(claimed_by,''), COALESCE(result,'')`
	)
	if batchID == "" {
		row = e.db.QueryRow(`
			UPDATE work_units SET status=?, claimed_by=?, updated_at=CURRENT_TIMESTAMP
			WHERE id = (SELECT id FROM work_units WHERE status=? ORDER BY created_at, seq, rowid LIMIT 1)
			RETURNING `+cols, WorkClaimed, agent, WorkOpen)
	} else {
		row = e.db.QueryRow(`
			UPDATE work_units SET status=?, claimed_by=?, updated_at=CURRENT_TIMESTAMP
			WHERE id = (SELECT id FROM work_units WHERE batch_id=? AND status=? ORDER BY seq LIMIT 1)
			RETURNING `+cols, WorkClaimed, agent, batchID, WorkOpen)
	}
	var u WorkUnit
	err := row.Scan(&u.ID, &u.BatchID, &u.Seq, &u.Title, &u.Spec, &u.Status, &u.ClaimedBy, &u.Result)
	if err == sql.ErrNoRows {
		return WorkUnit{}, false, nil
	}
	if err != nil {
		return WorkUnit{}, false, fmt.Errorf("error al reclamar unidad: %w", err)
	}
	return u, true, nil
}

// CompleteWorkUnit cierra una unidad con su resultado. status debe ser done o
// failed (vacío = done). Si agent != "", exige además que sea quien la reclamó
// (claimed_by), de modo que un agente no cierre la unidad de otro.
func (e *DbEngine) CompleteWorkUnit(id, result, status, agent string) error {
	if status == "" {
		status = WorkDone
	}
	if status != WorkDone && status != WorkFailed {
		return fmt.Errorf("status de cierre inválido %q (usá done|failed)", status)
	}
	// Guarda de estado: solo una unidad RECLAMADA puede cerrarse. Evita cerrar una
	// open nunca reclamada y re-cerrar/sobrescribir una ya done/failed.
	query := `UPDATE work_units SET status=?, result=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND status=?`
	args := []interface{}{status, result, id, WorkClaimed}
	if agent != "" {
		query += ` AND claimed_by=?`
		args = append(args, agent)
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
		return fmt.Errorf("la unidad %q no existe, no está reclamada, o fue reclamada por otro agente", id)
	}
	return nil
}

// WorkBatchStatus devuelve el estado consolidado de un batch (conteos + unidades).
func (e *DbEngine) WorkBatchStatus(batchID string) (WorkBatch, error) {
	rows, err := e.db.Query(`
		SELECT id, batch_id, seq, COALESCE(title,''), COALESCE(spec,''), status,
		       COALESCE(claimed_by,''), COALESCE(result,'')
		FROM work_units WHERE batch_id=? ORDER BY seq`, batchID)
	if err != nil {
		return WorkBatch{}, fmt.Errorf("error al leer el batch: %w", err)
	}
	defer rows.Close()

	b := WorkBatch{BatchID: batchID, Units: []WorkUnit{}}
	for rows.Next() {
		var u WorkUnit
		if err := rows.Scan(&u.ID, &u.BatchID, &u.Seq, &u.Title, &u.Spec, &u.Status,
			&u.ClaimedBy, &u.Result); err != nil {
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
