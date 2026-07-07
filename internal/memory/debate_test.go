package memory

import "testing"

// Escenario (a): ciclo completo 3 agentes × 2 rondas con quórum → gana la mayoría y cierra.
func TestDebateFullCycleMajorityWins(t *testing.T) {
	e := newTestEngine(t)
	d, err := e.OpenDebate("¿monolito o microservicios?", 2, 2)
	if err != nil {
		t.Fatalf("OpenDebate: %v", err)
	}
	if d.Status != DebateOpen || d.CurrentRound != 1 {
		t.Fatalf("debate recién abierto debe estar open en ronda 1, obtuve %+v", d)
	}

	// Ronda 1: tres posturas.
	for _, a := range []string{"ana", "beto", "caro"} {
		if err := e.PostPosture(d.ID, a, "postura R1 de "+a); err != nil {
			t.Fatalf("post R1 %s: %v", a, err)
		}
	}
	// Avanzar: devuelve las 3 posturas de R1 y pasa a la ronda 2.
	round, prev, err := e.AdvanceDebate(d.ID)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if round != 2 {
		t.Errorf("advance debe llevar a la ronda 2, obtuve %d", round)
	}
	if len(prev) != 3 {
		t.Fatalf("advance debe devolver las 3 posturas previas (crítica cruzada), obtuve %d", len(prev))
	}
	if prev[0].Agent != "ana" || prev[0].Round != 1 {
		t.Errorf("posturas previas deben ordenarse por agente y ser de la ronda 1, obtuve %+v", prev[0])
	}

	// Ronda 2: posturas revisadas.
	for _, a := range []string{"ana", "beto", "caro"} {
		if err := e.PostPosture(d.ID, a, "postura R2 de "+a); err != nil {
			t.Fatalf("post R2 %s: %v", a, err)
		}
	}
	// Votos: mayoría a "monolito".
	e.CastVote(d.ID, "ana", "monolito")
	e.CastVote(d.ID, "beto", "monolito")
	e.CastVote(d.ID, "caro", "microservicios")

	res, dd, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if !res.Decided || res.Winner != "monolito" {
		t.Errorf("debe ganar 'monolito' (2-1, quórum 2), obtuve %+v", res)
	}
	if dd.Status != DebateClosed || dd.Winner != "monolito" {
		t.Errorf("el debate debe quedar closed con winner, obtuve %+v", dd)
	}
	if res.TotalVotes != 3 {
		t.Errorf("total de votos debe ser 3, obtuve %d", res.TotalVotes)
	}
}

// Escenario (b): empate en la cima → no_consensus, el debate sigue open.
func TestDebateTieNoConsensus(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 0)
	e.CastVote(d.ID, "ana", "A")
	e.CastVote(d.ID, "beto", "B")
	e.CastVote(d.ID, "caro", "C")
	res, dd, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if res.Decided || res.Winner != "" {
		t.Errorf("un empate 1-1-1 no debe decidir, obtuve %+v", res)
	}
	if res.Reason == "" {
		t.Error("un no_consensus debe explicar la razón")
	}
	if dd.Status != DebateOpen {
		t.Errorf("sin consenso el debate debe seguir open, está %q", dd.Status)
	}
}

// Escenario (c): el más votado no alcanza el quórum → no_consensus.
func TestDebateBelowQuorum(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 3) // exige 3 votos para ganar
	e.CastVote(d.ID, "ana", "A")
	e.CastVote(d.ID, "beto", "A")
	e.CastVote(d.ID, "caro", "B") // A=2, B=1; máximo 2 < quórum 3
	res, dd, _ := e.TallyDebate(d.ID)
	if res.Decided {
		t.Errorf("el máximo (2) no alcanza el quórum (3): no debe decidir, obtuve %+v", res)
	}
	if dd.Status != DebateOpen {
		t.Errorf("sin quórum el debate sigue open, está %q", dd.Status)
	}
}

// Escenario (d): re-post y re-vote del mismo agente son idempotentes (sin duplicados).
func TestDebatePostAndVoteIdempotent(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 0)
	if err := e.PostPosture(d.ID, "ana", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := e.PostPosture(d.ID, "ana", "v2 corregida"); err != nil {
		t.Fatal(err)
	}
	_, postures, _, err := e.DebateStatus(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(postures) != 1 || postures[0].Stance != "v2 corregida" {
		t.Errorf("re-post debe reemplazar, no duplicar; obtuve %+v", postures)
	}
	e.CastVote(d.ID, "ana", "A")
	e.CastVote(d.ID, "ana", "B")
	_, _, votes, _ := e.DebateStatus(d.ID)
	if len(votes) != 1 || votes[0].Choice != "B" {
		t.Errorf("re-vote debe reemplazar, no duplicar; obtuve %+v", votes)
	}
}

// Escenario (e): tally sobre un debate ya cerrado es idempotente (no recuenta ni re-cierra).
func TestDebateTallyIdempotentOnClosed(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 0)
	e.CastVote(d.ID, "ana", "A")
	e.CastVote(d.ID, "beto", "A")
	res1, _, _ := e.TallyDebate(d.ID)
	if !res1.Decided || res1.Winner != "A" {
		t.Fatalf("primer tally debe cerrar con winner A, obtuve %+v", res1)
	}
	// Un voto tardío tras el cierre NO debe cambiar el ganador (el debate está closed).
	if err := e.CastVote(d.ID, "caro", "B"); err == nil {
		t.Error("votar en un debate cerrado debe fallar")
	}
	res2, dd, _ := e.TallyDebate(d.ID)
	if !res2.Decided || res2.Winner != "A" || dd.Status != DebateClosed {
		t.Errorf("un segundo tally debe devolver el mismo winner sin cambios, obtuve %+v / %+v", res2, dd)
	}
}

// Escenario (f): guardas — debate inexistente y post sobre debate cerrado.
func TestDebateGuards(t *testing.T) {
	e := newTestEngine(t)
	if err := e.PostPosture("no-existe", "ana", "x"); err == nil {
		t.Error("postear en un debate inexistente debe fallar")
	}
	if _, _, err := e.AdvanceDebate("no-existe"); err == nil {
		t.Error("avanzar un debate inexistente debe fallar")
	}
	d, _ := e.OpenDebate("tema", 1, 0)
	e.CastVote(d.ID, "ana", "A")
	e.TallyDebate(d.ID) // cierra (1 voto, sin quórum, máximo estricto)
	if err := e.PostPosture(d.ID, "beto", "tarde"); err == nil {
		t.Error("postear en un debate cerrado debe fallar")
	}
}

// Escenario (g): persistencia append-only — las posturas de rondas anteriores no se borran.
func TestDebatePosturesPersistAcrossRounds(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 3, 0)
	e.PostPosture(d.ID, "ana", "R1")
	e.AdvanceDebate(d.ID)
	e.PostPosture(d.ID, "ana", "R2")
	e.AdvanceDebate(d.ID)
	e.PostPosture(d.ID, "ana", "R3")
	_, postures, _, err := e.DebateStatus(d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(postures) != 3 {
		t.Fatalf("las 3 posturas (una por ronda) deben persistir, obtuve %d", len(postures))
	}
	// Ordenadas por ronda asc.
	for i, want := range []int{1, 2, 3} {
		if postures[i].Round != want {
			t.Errorf("postura %d debe ser de la ronda %d, obtuve %d", i, want, postures[i].Round)
		}
	}
}

// Escenario (h): un solo choice con máximo estricto gana (sin quórum).
func TestDebateSingleChoiceWins(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 0)
	e.CastVote(d.ID, "ana", "A")
	e.CastVote(d.ID, "beto", "A")
	res, _, _ := e.TallyDebate(d.ID)
	if !res.Decided || res.Winner != "A" {
		t.Errorf("un único choice votado debe ganar sin quórum, obtuve %+v", res)
	}
}

// Escenario (i): sin votos → no_consensus.
func TestDebateNoVotesNoConsensus(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 0)
	res, dd, _ := e.TallyDebate(d.ID)
	if res.Decided {
		t.Errorf("sin votos no debe haber ganador, obtuve %+v", res)
	}
	if dd.Status != DebateOpen {
		t.Error("sin votos el debate sigue open")
	}
}
