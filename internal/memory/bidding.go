package memory

import (
	"database/sql"
	"fmt"
)

// bidding.go implementa el protocolo CONTRACT-NET (Smith, 1980) sobre la pizarra
// multi-agente: en vez de asignar una unidad por CLAIM de orden de llegada (first-come), la
// tarea se anuncia, los agentes OFERTAN (bid = score de aptitud/confianza que produce el
// propio agente — model-free) y el orquestador ADJUDICA (award) a la mejor oferta. Convención:
// MAYOR bid = mejor. La adjudicación REUSA la maquinaria de lease/TTL/fencing de work.go: tras
// el award la unidad queda `claimed` por el ganador y sigue el flujo heartbeat/complete normal.
// Bidding y claim COEXISTEN: el orquestador elige el protocolo por unidad.

// WorkBid es una oferta de un agente por una unidad.
type WorkBid struct {
	UnitID    string  `json:"unit_id"`
	Agent     string  `json:"agent"`
	Bid       float64 `json:"bid"`
	Note      string  `json:"note,omitempty"`
	CreatedAt string  `json:"created_at,omitempty"`
}

// winnerOrder es el criterio determinista de "mejor oferta": mayor bid; a igual bid, la más
// antigua (created_at ASC); a igual instante, por nombre de agente. Compartido por el award y
// el listado para que "el primero" del listado sea siempre el que ganaría.
const winnerOrder = ` ORDER BY bid DESC, created_at ASC, agent ASC `

// BidWorkUnit registra (o actualiza) la oferta de un agente por una unidad. Solo se admite si
// la unidad existe y está OPEN (no se oferta sobre una ya reclamada/cerrada). Un mismo agente
// puede re-ofertar: el UPSERT reemplaza su bid/note y refresca el timestamp (su oferta vigente).
func (e *DbEngine) BidWorkUnit(unitID, agent string, bid float64, note string) error {
	if unitID == "" || agent == "" {
		return fmt.Errorf("bid requiere unit id y agent")
	}
	var status string
	err := e.db.QueryRow(`SELECT status FROM work_units WHERE id=?`, unitID).Scan(&status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("la unidad %q no existe", unitID)
	}
	if err != nil {
		return fmt.Errorf("error al verificar la unidad: %w", err)
	}
	if status != WorkOpen {
		return fmt.Errorf("no se puede ofertar sobre una unidad %q (solo 'open')", status)
	}
	if _, err := e.db.Exec(`
		INSERT INTO work_bids (unit_id, agent, bid, note, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(unit_id, agent) DO UPDATE SET
			bid=excluded.bid, note=excluded.note, created_at=excluded.created_at`,
		unitID, agent, bid, note,
	); err != nil {
		return fmt.Errorf("error al registrar la oferta: %w", err)
	}
	return nil
}

// AwardWorkUnit adjudica la unidad a la MEJOR oferta y la deja `claimed` por el ganador, con un
// lease de ttlSeconds. Atómico: un único UPDATE...RETURNING que asigna el ganador (subselect)
// SOLO si la unidad sigue `open` y tiene al menos una oferta; incrementa attempts y
// fencing_token como un claim normal. ok=false (sin error) si no hay ofertas o la unidad ya no
// está open (adjudicación idempotente: un segundo award no re-asigna). Devuelve la unidad y la
// oferta ganadora.
func (e *DbEngine) AwardWorkUnit(unitID string, ttlSeconds int) (WorkUnit, WorkBid, bool, error) {
	if unitID == "" {
		return WorkUnit{}, WorkBid{}, false, fmt.Errorf("award requiere unit id")
	}
	if ttlSeconds <= 0 {
		ttlSeconds = defaultLeaseTTLSeconds
	}

	winnerSub := `(SELECT agent FROM work_bids WHERE unit_id=?` + winnerOrder + `LIMIT 1)`
	row := e.db.QueryRow(`
		UPDATE work_units SET
			status=?,
			owner_id=`+winnerSub+`,
			claimed_by=`+winnerSub+`,
			lease_expires_at=datetime('now','+' || ? || ' seconds'),
			heartbeat_at=datetime('now'),
			attempts=attempts+1,
			fencing_token=fencing_token+1,
			updated_at=datetime('now')
		WHERE id=? AND status=?
		  AND EXISTS(SELECT 1 FROM work_bids WHERE unit_id=?)
		RETURNING `+workUnitCols,
		WorkClaimed, unitID, unitID, ttlSeconds, unitID, WorkOpen, unitID)
	u, err := scanWorkUnit(row)
	if err == sql.ErrNoRows {
		// Sin ofertas o la unidad ya no está open.
		return WorkUnit{}, WorkBid{}, false, nil
	}
	if err != nil {
		return WorkUnit{}, WorkBid{}, false, fmt.Errorf("error al adjudicar la unidad: %w", err)
	}

	// La oferta ganadora es la del nuevo dueño (owner_id ya es el ganador).
	var wb WorkBid
	wb.UnitID = unitID
	wb.Agent = u.OwnerID
	if err := e.db.QueryRow(
		`SELECT bid, COALESCE(note,''), COALESCE(created_at,'') FROM work_bids WHERE unit_id=? AND agent=?`,
		unitID, u.OwnerID,
	).Scan(&wb.Bid, &wb.Note, &wb.CreatedAt); err != nil {
		return WorkUnit{}, WorkBid{}, false, fmt.Errorf("error al leer la oferta ganadora: %w", err)
	}
	return u, wb, true, nil
}

// WorkUnitBids lista las ofertas de una unidad, la mejor primero (mismo orden que el award).
func (e *DbEngine) WorkUnitBids(unitID string) ([]WorkBid, error) {
	rows, err := e.db.Query(
		`SELECT unit_id, agent, bid, COALESCE(note,''), COALESCE(created_at,'') FROM work_bids WHERE unit_id=?`+winnerOrder,
		unitID)
	if err != nil {
		return nil, fmt.Errorf("error al listar ofertas: %w", err)
	}
	defer rows.Close()
	bids := []WorkBid{}
	for rows.Next() {
		var b WorkBid
		if err := rows.Scan(&b.UnitID, &b.Agent, &b.Bid, &b.Note, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear oferta: %w", err)
		}
		bids = append(bids, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar ofertas: %w", err)
	}
	return bids, nil
}
