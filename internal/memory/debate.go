package memory

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"

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

	// GatedChoice es el choice que NO puede ganar sin al menos un voto con evidencia
	// determinística. Vacío = sin compuerta = el comportamiento de siempre.
	//
	// Se declara al ABRIR el debate, antes de saber quién va a votar qué. Declararla
	// después sería mover el arco con la pelota en el aire.
	GatedChoice string `json:"gated_choice,omitempty"`
}

// DebatePosture es la postura de un agente en una ronda.
type DebatePosture struct {
	Round     int    `json:"round"`
	Agent     string `json:"agent"`
	Stance    string `json:"stance"`
	CreatedAt string `json:"created_at,omitempty"`

	// Model es el modelo que produjo esta postura. Sin esta columna, «lo revisaron tres
	// modelos distintos» era inverificable a posteriori: no había dónde leerlo.
	Model string `json:"model"`
	// Evidence es la CLASE de evidencia declarada. Ver las constantes Evidencia*.
	Evidence string `json:"evidence"`
}

// DebateVote es el voto vigente de un agente.
type DebateVote struct {
	Agent     string `json:"agent"`
	Choice    string `json:"choice"`
	CreatedAt string `json:"created_at,omitempty"`

	Model    string `json:"model"`
	Evidence string `json:"evidence"`
}

// Clases de evidencia. Son TRES y no un booleano porque «no verifiqué» y «no pude
// verificar» se declaran igual pero enseñan cosas distintas al que lee el debate.
//
// ⚠️ LIMITE HONESTO: la clase es una DECLARACION, no una prueba. Este mecanismo detecta
// al panel que no verificó nada; no al que miente. Sirve contra la desidia, no contra la
// mala fe, y conviene no confundir las dos cosas.
const (
	// EvidenciaDeterministica: corrió las comprobaciones y tiene la salida real.
	EvidenciaDeterministica = "deterministica"
	// EvidenciaInferida: leyó el código o el diff y razonó, sin ejecutar nada.
	EvidenciaInferida = "inferida"
	// EvidenciaNinguna: no pudo comprobar nada. Es una respuesta VALIDA y hay que poder
	// darla: forzar a declarar algo mejor de lo que se tiene es pedir que se mienta.
	EvidenciaNinguna = "ninguna"
)

// evidenciaValida acepta sólo las tres clases. Una cuarta cadena cualquiera pasaría por
// la compuerta como si fuera evidencia real, así que el conjunto es cerrado.
// OJO: en este mismo paquete hay otra familia Evidencia* (skillusage.go: Alcance, Glob,
// Comodin) que es OTRO vocabulario, de otro subsistema. Por eso el validador lleva
// "clase" en el nombre y no se llama evidenciaValida a secas — que ademas ya existe alla.
func claseEvidenciaValida(e string) bool {
	return e == EvidenciaDeterministica || e == EvidenciaInferida || e == EvidenciaNinguna
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
	// Gated indica que hubo un ganador por conteo y la compuerta lo frenó por falta de
	// evidencia determinística. Va aparte de Reason para que el llamador pueda
	// distinguir «no hubo consenso» de «hubo consenso y no alcanzó»: son dos acciones
	// distintas —seguir debatiendo, o ir a correr las comprobaciones—.
	Gated bool `json:"gated,omitempty"`
}

// reVueltaDeTopic reconoce la convención del bucle de corrección: «... · vuelta k/K · ...».
//
// La convención vive ACÁ, en una sola definición, porque la leen dos consumidores: este motor
// —que la hace cumplir— y `musubi arnes` —que mide cuántas vueltas toma una corrección—. Con
// dos regex separadas, endurecer una dejaría a la otra midiendo la convención vieja, y esa
// desincronización es invisible: las dos siguen andando y contestan cosas distintas.
var reVueltaDeTopic = regexp.MustCompile(`vuelta\s+(\d+)\s*/\s*(\d+)`)

// VueltaDelTopic extrae (vuelta, tope) de un topic que siga la convención del bucle de
// corrección. `ok` es false si el topic no la sigue — un debate suelto, o uno anterior a la
// convención— y en ese caso no hay nada que hacer cumplir.
func VueltaDelTopic(topic string) (vuelta, tope int, ok bool) {
	m := reVueltaDeTopic.FindStringSubmatch(topic)
	if m == nil {
		return 0, 0, false
	}
	k, err1 := strconv.Atoi(m[1])
	t, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return k, t, true
}

// OpenDebate crea un debate 'open' con current_round=1. rounds se clampa a >=1; quorum es el
// mínimo de votos de un choice ganador (0 = sin piso, gana la mayoría estricta).
func (e *DbEngine) OpenDebate(topic string, rounds, quorum int, gatedChoice string) (Debate, error) {
	if topic == "" {
		return Debate{}, fmt.Errorf("open requiere 'topic'")
	}
	// EL TOPE DEL BUCLE DE CORRECCIÓN SE HACE CUMPLIR, NO SE PIDE POR FAVOR.
	//
	// La instrucción decía «K=3 vueltas, y agotarlo es un rechazo», y nada en el código impedía
	// abrir la vuelta K+1: quedaba en manos de quien estaba, justamente, cansado de corregir.
	// El riesgo de un bucle sin salida no es girar para siempre — es que el agente ceda y
	// apruebe para terminar, que es lo mismo que el tope existe para evitar.
	//
	// El motor no necesita estado nuevo para sostenerlo: el topic ya declara la vuelta y su
	// tope, y negarse a abrir falla del lado seguro —un debate que no existe no puede aprobar
	// nada—.
	if k, tope, ok := VueltaDelTopic(topic); ok && k > tope {
		return Debate{}, fmt.Errorf(
			"el bucle de corrección se agotó: la vuelta %d pasa el tope de %d que declara el topic. "+
				"El veredicto es RECHAZADO POR AGOTAMIENTO — no se abre otra vuelta; escalá a una "+
				"persona con el estado completo (hallazgos abiertos y los debates previos)", k, tope)
	}
	if rounds < 1 {
		rounds = 1
	}
	if quorum < 0 {
		quorum = 0
	}
	d := Debate{ID: uuid.NewString(), Topic: topic, Rounds: rounds, CurrentRound: 1, Quorum: quorum, Status: DebateOpen, GatedChoice: gatedChoice}
	if _, err := e.db.Exec(`
		INSERT INTO debates (id, topic, rounds, current_round, quorum, status, gated_choice, created_at)
		VALUES (?, ?, ?, 1, ?, ?, ?, datetime('now'))`,
		d.ID, d.Topic, d.Rounds, d.Quorum, d.Status, d.GatedChoice,
	); err != nil {
		return Debate{}, fmt.Errorf("error al abrir el debate: %w", err)
	}
	return d, nil
}

// getDebate lee un debate por id (error claro si no existe).
func (e *DbEngine) getDebate(debateID string) (Debate, error) {
	var d Debate
	var winner, createdAt, closedAt, gated sql.NullString
	err := e.db.QueryRow(`
		SELECT id, topic, rounds, current_round, quorum, status, winner, created_at, closed_at, COALESCE(gated_choice,'')
		FROM debates WHERE id=?`, debateID).
		Scan(&d.ID, &d.Topic, &d.Rounds, &d.CurrentRound, &d.Quorum, &d.Status, &winner, &createdAt, &closedAt, &gated)
	if err == sql.ErrNoRows {
		return Debate{}, fmt.Errorf("el debate %q no existe", debateID)
	}
	if err != nil {
		return Debate{}, fmt.Errorf("error al leer el debate: %w", err)
	}
	d.Winner, d.CreatedAt, d.ClosedAt, d.GatedChoice = winner.String, createdAt.String, closedAt.String, gated.String
	return d, nil
}

// PostPosture registra (o actualiza) la postura de un agente en la ronda ACTUAL. Idempotente por
// (debate_id, ronda actual, agent): re-postear reemplaza la postura de ese agente en esa ronda.
// Solo sobre un debate 'open'.
func (e *DbEngine) PostPosture(debateID, agent, stance, model, evidence string) error {
	if agent == "" || stance == "" {
		return fmt.Errorf("post requiere 'agent' y 'stance'")
	}
	// model y evidence NACEN OBLIGATORIOS. Se pudo porque las tres tablas estaban en cero
	// filas: sin datos vivos no hace falta default piadoso ni período de gracia. Con datos
	// habría habido que aceptarlos vacíos, y un campo opcional para auditar es un campo
	// que se deja vacío justo cuando importa.
	if model == "" {
		return fmt.Errorf("post requiere 'model': dos jueces del mismo modelo no son dos opiniones, y sin esto no hay dónde comprobarlo")
	}
	if !claseEvidenciaValida(evidence) {
		return fmt.Errorf("post requiere 'evidence' en {%s, %s, %s}, recibí %q",
			EvidenciaDeterministica, EvidenciaInferida, EvidenciaNinguna, evidence)
	}
	d, err := e.getDebate(debateID)
	if err != nil {
		return err
	}
	if d.Status != DebateOpen {
		return fmt.Errorf("no se puede postear en un debate %q (solo 'open')", d.Status)
	}
	if _, err := e.db.Exec(`
		INSERT INTO debate_postures (debate_id, round, agent, stance, model, evidence, created_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(debate_id, round, agent) DO UPDATE SET
			stance=excluded.stance, model=excluded.model, evidence=excluded.evidence, created_at=excluded.created_at`,
		debateID, d.CurrentRound, agent, stance, model, evidence,
	); err != nil {
		return fmt.Errorf("error al registrar la postura: %w", err)
	}
	return nil
}

// posturesForRound devuelve las posturas de una ronda, ordenadas por agente (determinista).
func (e *DbEngine) posturesForRound(debateID string, round int) ([]DebatePosture, error) {
	rows, err := e.db.Query(`
		SELECT round, agent, stance, COALESCE(model,''), COALESCE(evidence,''), COALESCE(created_at,'')
		FROM debate_postures WHERE debate_id=? AND round=?
		ORDER BY agent ASC`, debateID, round)
	if err != nil {
		return nil, fmt.Errorf("error al leer posturas: %w", err)
	}
	defer rows.Close()
	out := []DebatePosture{}
	for rows.Next() {
		var p DebatePosture
		if err := rows.Scan(&p.Round, &p.Agent, &p.Stance, &p.Model, &p.Evidence, &p.CreatedAt); err != nil {
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
func (e *DbEngine) CastVote(debateID, agent, choice, model, evidence string) error {
	if agent == "" || choice == "" {
		return fmt.Errorf("vote requiere 'agent' y 'choice'")
	}
	if model == "" {
		return fmt.Errorf("vote requiere 'model'")
	}
	if !claseEvidenciaValida(evidence) {
		return fmt.Errorf("vote requiere 'evidence' en {%s, %s, %s}, recibí %q",
			EvidenciaDeterministica, EvidenciaInferida, EvidenciaNinguna, evidence)
	}
	d, err := e.getDebate(debateID)
	if err != nil {
		return err
	}
	if d.Status != DebateOpen {
		return fmt.Errorf("no se puede votar en un debate %q (solo 'open')", d.Status)
	}
	if _, err := e.db.Exec(`
		INSERT INTO debate_votes (debate_id, agent, choice, model, evidence, created_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(debate_id, agent) DO UPDATE SET
			choice=excluded.choice, model=excluded.model, evidence=excluded.evidence, created_at=excluded.created_at`,
		debateID, agent, choice, model, evidence,
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

	// ── LA COMPUERTA ──────────────────────────────────────────────────────────────
	//
	// Un veredicto que gana por conteo no cierra si NINGUNO de sus votos declaró haber
	// verificado algo. El tally cuenta filas: sin esto, un lente que corrió los tests pesa
	// igual que uno que leyó el diff y opinó, y que uno que no pudo comprobar nada.
	//
	// 🔴 TRES PROPIEDADES QUE LA SEPARAN DE UN CANDADO, y que hay que sostener juntas:
	//
	//  1. RECHAZAR NUNCA LLEVA COMPUERTA. Sólo se gatea el choice declarado al abrir. Un
	//     panel que no pudo verificar nada tiene que poder rechazar igual — si esto
	//     fallara, lo construido sería una máquina de aprobar por incapacidad.
	//  2. UN SOLO VOTO DETERMINISTICO LA DESARMA. No es una mayoría paralela: es un piso.
	//     Pedir más sería un segundo quórum encubierto.
	//  3. NO REEMPLAZA AL QUORUM. Corre DESPUES: si el quórum no se alcanzó, ya no hay
	//     ganador y la compuerta ni se consulta.
	//
	// Sin gated_choice declarado, todo esto es un no-op y el comportamiento es idéntico al
	// de siempre.
	if res.Decided && d.GatedChoice != "" && res.Winner == d.GatedChoice {
		n, gerr := e.votosDeterministicosPara(debateID, res.Winner)
		if gerr != nil {
			return TallyResult{}, Debate{}, gerr
		}
		if n == 0 {
			res.Winner, res.Decided, res.Gated = "", false, true
			res.Reason = fmt.Sprintf("%q ganó el conteo pero ningún voto declaró evidencia %s: "+
				"un panel que opina sin haber corrido nada es teatro de verificación. Corré las "+
				"comprobaciones y re-votá declarándolo, o dejá que gane otro choice",
				d.GatedChoice, EvidenciaDeterministica)
		}
	}

	if !res.Decided {
		return res, d, nil // no_consensus (o frenado por la compuerta): el debate sigue open
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
		SELECT round, agent, stance, COALESCE(model,''), COALESCE(evidence,''), COALESCE(created_at,'')
		FROM debate_postures WHERE debate_id=?
		ORDER BY round ASC, agent ASC`, debateID)
	if err != nil {
		return Debate{}, nil, nil, fmt.Errorf("error al leer posturas: %w", err)
	}
	defer prows.Close()
	postures := []DebatePosture{}
	for prows.Next() {
		var p DebatePosture
		if err := prows.Scan(&p.Round, &p.Agent, &p.Stance, &p.Model, &p.Evidence, &p.CreatedAt); err != nil {
			return Debate{}, nil, nil, fmt.Errorf("error al escanear postura: %w", err)
		}
		postures = append(postures, p)
	}
	if err := prows.Err(); err != nil {
		return Debate{}, nil, nil, err
	}

	vrows, err := e.db.Query(`
		SELECT agent, choice, COALESCE(model,''), COALESCE(evidence,''), COALESCE(created_at,'')
		FROM debate_votes WHERE debate_id=?
		ORDER BY agent ASC`, debateID)
	if err != nil {
		return Debate{}, nil, nil, fmt.Errorf("error al leer votos: %w", err)
	}
	defer vrows.Close()
	votes := []DebateVote{}
	for vrows.Next() {
		var v DebateVote
		if err := vrows.Scan(&v.Agent, &v.Choice, &v.Model, &v.Evidence, &v.CreatedAt); err != nil {
			return Debate{}, nil, nil, fmt.Errorf("error al escanear voto: %w", err)
		}
		votes = append(votes, v)
	}
	if err := vrows.Err(); err != nil {
		return Debate{}, nil, nil, err
	}

	return d, postures, votes, nil
}

// votosDeterministicosPara cuenta cuántos votos por un choice declararon evidencia
// determinística. Es la única consulta que la compuerta necesita.
func (e *DbEngine) votosDeterministicosPara(debateID, choice string) (int, error) {
	var n int
	err := e.db.QueryRow(`SELECT COUNT(*) FROM debate_votes
		WHERE debate_id=? AND choice=? AND evidence=?`, debateID, choice, EvidenciaDeterministica).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("error al contar votos con evidencia determinística: %w", err)
	}
	return n, nil
}
