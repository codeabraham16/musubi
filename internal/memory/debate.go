package memory

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// debate.go implementa el subsistema de DEBATE MULTI-AGENTE (multi-agent debate / Society of
// Minds) como andamiaje EJECUTABLE y DETERMINISTA, self-contained y MODEL-FREE. Igual que la
// pizarra (work.go) y el motor DAG (workflow.go), Musubi NO razona: los agentes (LLM) producen
// las posturas, las críticas y los votos; Musubi estructura las rondas, PERSISTE las posturas
// atribuidas (crítica cruzada reproducible) y CUENTA los votos (tally por mayoría/quórum). El
// juicio semántico —elegir o sintetizar— se queda 100% en el LLM; cuando el tally no converge,
// el resultado es 'no_consensus' y decide el humano/LLM (o se difiere a musubi_judge).
//
// Ciclo de vida: open → (post×N, advance)×R → vote → tally → closed(winner) | sigue open.

// Estados de un debate.
const (
	DebateOpen   = "open"
	DebateClosed = "closed"
)

// Debate es una sesión de debate.
type Debate struct {
	ID           string `json:"id"`
	Topic        string `json:"topic"`
	Rounds       int    `json:"rounds"`        // tope de rondas
	CurrentRound int    `json:"current_round"` // ronda activa (1..rounds)
	Quorum       int    `json:"quorum"`        // mínimo de votos que un ganador debe alcanzar (0 = sin piso)
	Status       string `json:"status"`        // open | closed
	Winner       string `json:"winner,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	ClosedAt     string `json:"closed_at,omitempty"`
}

// DebatePosture es la postura de un agente en una ronda.
type DebatePosture struct {
	Round     int    `json:"round"`
	Agent     string `json:"agent"`
	Stance    string `json:"stance"`
	CreatedAt string `json:"created_at,omitempty"`
}

// DebateVote es el voto vigente de un agente.
type DebateVote struct {
	Agent     string `json:"agent"`
	Choice    string `json:"choice"`
	CreatedAt string `json:"created_at,omitempty"`
}

// VoteCount es el conteo de votos de un choice (para el tally).
type VoteCount struct {
	Choice string `json:"choice"`
	Count  int    `json:"count"`
}

// TallyResult es el resultado del recuento determinista.
type TallyResult struct {
	Counts     []VoteCount `json:"counts"`           // por choice, mayor primero (desempate por choice asc)
	Winner     string      `json:"winner,omitempty"` // vacío si no hubo consenso
	Decided    bool        `json:"decided"`          // true si hay ganador (el debate quedó closed)
	Reason     string      `json:"reason,omitempty"` // por qué no hubo consenso (empate / bajo quórum / sin votos)
	TotalVotes int         `json:"total_votes"`
}

// OpenDebate crea un debate 'open' con current_round=1. rounds se clampa a >=1; quorum es el
// mínimo de votos de un choice ganador (0 = sin piso, gana la mayoría estricta).
func (e *DbEngine) OpenDebate(topic string, rounds, quorum int) (Debate, error) {
	if topic == "" {
		return Debate{}, fmt.Errorf("open requiere 'topic'")
	}
	if rounds < 1 {
		rounds = 1
	}
	if quorum < 0 {
		quorum = 0
	}
	d := Debate{ID: uuid.NewString(), Topic: topic, Rounds: rounds, CurrentRound: 1, Quorum: quorum, Status: DebateOpen}
	if _, err := e.db.Exec(`
		INSERT INTO debates (id, topic, rounds, current_round, quorum, status, created_at)
		VALUES (?, ?, ?, 1, ?, ?, datetime('now'))`,
		d.ID, d.Topic, d.Rounds, d.Quorum, d.Status,
	); err != nil {
		return Debate{}, fmt.Errorf("error al abrir el debate: %w", err)
	}
	return d, nil
}

// getDebate lee un debate por id (error claro si no existe).
func (e *DbEngine) getDebate(debateID string) (Debate, error) {
	var d Debate
	var winner, createdAt, closedAt sql.NullString
	err := e.db.QueryRow(`
		SELECT id, topic, rounds, current_round, quorum, status, winner, created_at, closed_at
		FROM debates WHERE id=?`, debateID).
		Scan(&d.ID, &d.Topic, &d.Rounds, &d.CurrentRound, &d.Quorum, &d.Status, &winner, &createdAt, &closedAt)
	if err == sql.ErrNoRows {
		return Debate{}, fmt.Errorf("el debate %q no existe", debateID)
	}
	if err != nil {
		return Debate{}, fmt.Errorf("error al leer el debate: %w", err)
	}
	d.Winner, d.CreatedAt, d.ClosedAt = winner.String, createdAt.String, closedAt.String
	return d, nil
}

// PostPosture registra (o actualiza) la postura de un agente en la ronda ACTUAL. Idempotente por
// (debate_id, ronda actual, agent): re-postear reemplaza la postura de ese agente en esa ronda.
// Solo sobre un debate 'open'.
func (e *DbEngine) PostPosture(debateID, agent, stance string) error {
	if agent == "" || stance == "" {
		return fmt.Errorf("post requiere 'agent' y 'stance'")
	}
	d, err := e.getDebate(debateID)
	if err != nil {
		return err
	}
	if d.Status != DebateOpen {
		return fmt.Errorf("no se puede postear en un debate %q (solo 'open')", d.Status)
	}
	if _, err := e.db.Exec(`
		INSERT INTO debate_postures (debate_id, round, agent, stance, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))
		ON CONFLICT(debate_id, round, agent) DO UPDATE SET
			stance=excluded.stance, created_at=excluded.created_at`,
		debateID, d.CurrentRound, agent, stance,
	); err != nil {
		return fmt.Errorf("error al registrar la postura: %w", err)
	}
	return nil
}

// posturesForRound devuelve las posturas de una ronda, ordenadas por agente (determinista).
func (e *DbEngine) posturesForRound(debateID string, round int) ([]DebatePosture, error) {
	rows, err := e.db.Query(`
		SELECT round, agent, stance, COALESCE(created_at,'')
		FROM debate_postures WHERE debate_id=? AND round=?
		ORDER BY agent ASC`, debateID, round)
	if err != nil {
		return nil, fmt.Errorf("error al leer posturas: %w", err)
	}
	defer rows.Close()
	out := []DebatePosture{}
	for rows.Next() {
		var p DebatePosture
		if err := rows.Scan(&p.Round, &p.Agent, &p.Stance, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear postura: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AdvanceDebate cierra la ronda actual y avanza a la siguiente (hasta 'rounds'; en la última es
// no-op). Devuelve la nueva ronda activa y las posturas de la ronda que se acaba de cerrar, para
// que el orquestador se las pase a los agentes como material de CRÍTICA CRUZADA. Todas las
// posturas persisten (append-only por ronda; nunca se borran). Solo sobre un debate 'open'.
func (e *DbEngine) AdvanceDebate(debateID string) (int, []DebatePosture, error) {
	d, err := e.getDebate(debateID)
	if err != nil {
		return 0, nil, err
	}
	if d.Status != DebateOpen {
		return 0, nil, fmt.Errorf("no se puede avanzar un debate %q (solo 'open')", d.Status)
	}
	prev, err := e.posturesForRound(debateID, d.CurrentRound)
	if err != nil {
		return 0, nil, err
	}
	newRound := d.CurrentRound
	if d.CurrentRound < d.Rounds {
		newRound = d.CurrentRound + 1
		if _, err := e.db.Exec(`UPDATE debates SET current_round=? WHERE id=? AND status=?`,
			newRound, debateID, DebateOpen); err != nil {
			return 0, nil, fmt.Errorf("error al avanzar de ronda: %w", err)
		}
	}
	return newRound, prev, nil
}

// CastVote registra (o actualiza) el voto de un agente. Idempotente por (debate_id, agent):
// re-votar reemplaza el voto vigente. Solo sobre un debate 'open'.
func (e *DbEngine) CastVote(debateID, agent, choice string) error {
	if agent == "" || choice == "" {
		return fmt.Errorf("vote requiere 'agent' y 'choice'")
	}
	d, err := e.getDebate(debateID)
	if err != nil {
		return err
	}
	if d.Status != DebateOpen {
		return fmt.Errorf("no se puede votar en un debate %q (solo 'open')", d.Status)
	}
	if _, err := e.db.Exec(`
		INSERT INTO debate_votes (debate_id, agent, choice, created_at)
		VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(debate_id, agent) DO UPDATE SET
			choice=excluded.choice, created_at=excluded.created_at`,
		debateID, agent, choice,
	); err != nil {
		return fmt.Errorf("error al registrar el voto: %w", err)
	}
	return nil
}

// voteCounts cuenta los votos por choice, mayor primero (desempate determinista por choice asc).
func (e *DbEngine) voteCounts(debateID string) ([]VoteCount, int, error) {
	rows, err := e.db.Query(`
		SELECT choice, COUNT(*) AS c FROM debate_votes WHERE debate_id=?
		GROUP BY choice ORDER BY c DESC, choice ASC`, debateID)
	if err != nil {
		return nil, 0, fmt.Errorf("error al contar votos: %w", err)
	}
	defer rows.Close()
	var counts []VoteCount
	total := 0
	for rows.Next() {
		var vc VoteCount
		if err := rows.Scan(&vc.Choice, &vc.Count); err != nil {
			return nil, 0, fmt.Errorf("error al escanear conteo: %w", err)
		}
		counts = append(counts, vc)
		total += vc.Count
	}
	return counts, total, rows.Err()
}

// TallyDebate cuenta los votos de forma 100% determinista y, si hay ganador, cierra el debate.
// Ganador = el choice con el MÁXIMO ESTRICTO de votos (sin empate en la cima) que además alcanza
// el quórum (si quorum>0). Empate, bajo quórum o sin votos ⇒ no_consensus y el debate sigue
// 'open'. Idempotente: sobre un debate ya cerrado devuelve su ganador sin re-contar ni re-cerrar.
func (e *DbEngine) TallyDebate(debateID string) (TallyResult, Debate, error) {
	d, err := e.getDebate(debateID)
	if err != nil {
		return TallyResult{}, Debate{}, err
	}

	counts, total, err := e.voteCounts(debateID)
	if err != nil {
		return TallyResult{}, Debate{}, err
	}
	res := TallyResult{Counts: counts, TotalVotes: total}

	// Debate ya cerrado: idempotente, devolver el ganador persistido sin re-cerrar.
	if d.Status == DebateClosed {
		res.Winner, res.Decided = d.Winner, true
		return res, d, nil
	}

	// Determinar ganador model-free: máximo estricto + quórum.
	switch {
	case len(counts) == 0:
		res.Reason = "sin votos"
	case len(counts) >= 2 && counts[1].Count == counts[0].Count:
		res.Reason = "empate en la cima (no hay máximo estricto)"
	case d.Quorum > 0 && counts[0].Count < d.Quorum:
		res.Reason = fmt.Sprintf("el más votado (%d) no alcanza el quórum (%d)", counts[0].Count, d.Quorum)
	default:
		res.Winner, res.Decided = counts[0].Choice, true
	}

	if !res.Decided {
		return res, d, nil // no_consensus: el debate sigue open
	}

	// Cierre atómico guardado por status='open' (idempotente ante un tally concurrente).
	upd, err := e.db.Exec(`UPDATE debates SET status=?, winner=?, closed_at=datetime('now')
		WHERE id=? AND status=?`, DebateClosed, res.Winner, debateID, DebateOpen)
	if err != nil {
		return TallyResult{}, Debate{}, fmt.Errorf("error al cerrar el debate: %w", err)
	}
	if n, _ := upd.RowsAffected(); n == 0 {
		// Otro tally lo cerró primero: re-leer el ganador persistido (fuente de verdad).
		d, err = e.getDebate(debateID)
		if err != nil {
			return TallyResult{}, Debate{}, err
		}
		res.Winner, res.Decided = d.Winner, true
		return res, d, nil
	}
	// Reflejar el cierre en el struct devuelto.
	d.Status, d.Winner = DebateClosed, res.Winner
	return res, d, nil
}

// DebateStatus devuelve el estado completo del debate: la sesión, todas las posturas (todas las
// rondas, ordenadas por ronda y agente) y los votos vigentes.
func (e *DbEngine) DebateStatus(debateID string) (Debate, []DebatePosture, []DebateVote, error) {
	d, err := e.getDebate(debateID)
	if err != nil {
		return Debate{}, nil, nil, err
	}

	prows, err := e.db.Query(`
		SELECT round, agent, stance, COALESCE(created_at,'')
		FROM debate_postures WHERE debate_id=?
		ORDER BY round ASC, agent ASC`, debateID)
	if err != nil {
		return Debate{}, nil, nil, fmt.Errorf("error al leer posturas: %w", err)
	}
	defer prows.Close()
	postures := []DebatePosture{}
	for prows.Next() {
		var p DebatePosture
		if err := prows.Scan(&p.Round, &p.Agent, &p.Stance, &p.CreatedAt); err != nil {
			return Debate{}, nil, nil, fmt.Errorf("error al escanear postura: %w", err)
		}
		postures = append(postures, p)
	}
	if err := prows.Err(); err != nil {
		return Debate{}, nil, nil, err
	}

	vrows, err := e.db.Query(`
		SELECT agent, choice, COALESCE(created_at,'')
		FROM debate_votes WHERE debate_id=?
		ORDER BY agent ASC`, debateID)
	if err != nil {
		return Debate{}, nil, nil, fmt.Errorf("error al leer votos: %w", err)
	}
	defer vrows.Close()
	votes := []DebateVote{}
	for vrows.Next() {
		var v DebateVote
		if err := vrows.Scan(&v.Agent, &v.Choice, &v.CreatedAt); err != nil {
			return Debate{}, nil, nil, fmt.Errorf("error al escanear voto: %w", err)
		}
		votes = append(votes, v)
	}
	if err := vrows.Err(); err != nil {
		return Debate{}, nil, nil, err
	}

	return d, postures, votes, nil
}
