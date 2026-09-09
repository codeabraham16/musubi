package memory

import (
	"strings"
	"testing"
)

// Banco de la compuerta y de la procedencia del panel (F3). Cada test declara UN
// invariante, nombrado E<n>, con su sabotaje verificado en rojo.
//
// El invariante más importante de todos es E4: si la compuerta se aplicara también al
// rechazo, lo construido sería una máquina de aprobar por incapacidad. Todo lo demás de
// esta fase es inútil si ése falla.

const (
	elVeredictoAprobatorio = "real"
	elVeredictoQueRechaza  = "no_real"
)

// panelDe abre un debate con compuerta sobre el veredicto aprobatorio y hace votar a los
// agentes dados, cada uno con su modelo y su clase de evidencia.
func panelConCompuerta(t *testing.T, quorum int, votos [][3]string) (*DbEngine, Debate) {
	t.Helper()
	e := newTestEngine(t)
	d, err := e.OpenDebate("¿el cambio sobrevive?", 1, quorum, elVeredictoAprobatorio)
	if err != nil {
		t.Fatalf("OpenDebate: %v", err)
	}
	for _, v := range votos {
		// v = {agente, choice, evidencia}
		if err := e.CastVote(d.ID, v[0], v[1], "modelo-de-"+v[0], v[2]); err != nil {
			t.Fatalf("CastVote(%v): %v", v, err)
		}
	}
	return e, d
}

// --- E1: model y evidence nacen obligatorios ------------------------------------------

func TestE1ModelYEvidenceSonObligatorios(t *testing.T) {
	e := newTestEngine(t)
	d, err := e.OpenDebate("tema", 1, 0, "")
	if err != nil {
		t.Fatalf("OpenDebate: %v", err)
	}
	casos := []struct {
		nombre           string
		model, evidencia string
	}{
		{"sin modelo", "", EvidenciaInferida},
		{"evidencia vacía", "m", ""},
		{"evidencia inventada", "m", "capaz"},
		{"evidencia con otro vocabulario del paquete", "m", EvidenciaGlob},
	}
	for _, c := range casos {
		if err := e.PostPosture(d.ID, "ana", "postura", c.model, c.evidencia); err == nil {
			t.Errorf("post con %s tendría que fallar: un campo de auditoría opcional se deja vacío justo cuando importa", c.nombre)
		}
		if err := e.CastVote(d.ID, "ana", elVeredictoAprobatorio, c.model, c.evidencia); err == nil {
			t.Errorf("vote con %s tendría que fallar", c.nombre)
		}
	}
}

// --- E2: lo declarado se persiste y se puede leer después ------------------------------

func TestE2ElModeloYLaEvidenciaSobrevivenYSeLeen(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 0, "")
	if err := e.PostPosture(d.ID, "seguridad", "refuto", "opus", EvidenciaDeterministica); err != nil {
		t.Fatalf("post: %v", err)
	}
	if err := e.CastVote(d.ID, "seguridad", elVeredictoQueRechaza, "opus", EvidenciaDeterministica); err != nil {
		t.Fatalf("vote: %v", err)
	}

	_, posturas, votos, err := e.DebateStatus(d.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	// «Este cambio lo revisaron tres modelos distintos» tiene que ser comprobable A
	// POSTERIORI. Si no se lee de vuelta, la columna existe y la afirmación sigue siendo
	// indemostrable, que es exactamente el estado del que se venía.
	if len(posturas) != 1 || posturas[0].Model != "opus" || posturas[0].Evidence != EvidenciaDeterministica {
		t.Errorf("la postura tiene que devolver su modelo y su evidencia, obtuve %+v", posturas)
	}
	if len(votos) != 1 || votos[0].Model != "opus" || votos[0].Evidence != EvidenciaDeterministica {
		t.Errorf("el voto tiene que devolver su modelo y su evidencia, obtuve %+v", votos)
	}
}

// --- E3: la compuerta frena una aprobación sin nadie que haya verificado ---------------

func TestE3AprobarSinEvidenciaNoCierra(t *testing.T) {
	e, d := panelConCompuerta(t, 2, [][3]string{
		{"correctitud", elVeredictoAprobatorio, EvidenciaInferida},
		{"seguridad", elVeredictoAprobatorio, EvidenciaNinguna},
	})
	res, deb, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if res.Decided {
		t.Error("dos opiniones sin una sola comprobación no son un aprobado: la compuerta tenía que frenarlo")
	}
	if !res.Gated {
		t.Error("el resultado tiene que DECIR que lo frenó la compuerta: «no hubo consenso» y «hubo consenso y no alcanzó» piden acciones distintas")
	}
	if !strings.Contains(res.Reason, EvidenciaDeterministica) {
		t.Errorf("el motivo tiene que nombrar qué falta; obtuve %q", res.Reason)
	}
	// E8: frenado no es cerrado. El debate sigue vivo para poder ir a correr las pruebas.
	if deb.Status != DebateOpen {
		t.Errorf("un debate frenado por la compuerta sigue open, obtuve %q", deb.Status)
	}
}

// --- E4 🔴: rechazar NUNCA lleva compuerta ---------------------------------------------

func TestE4RechazarNoLlevaCompuertaNunca(t *testing.T) {
	// Un panel que no pudo verificar NADA —la peor evidencia posible en los dos votos—
	// tiene que poder rechazar igual.
	e, d := panelConCompuerta(t, 2, [][3]string{
		{"correctitud", elVeredictoQueRechaza, EvidenciaNinguna},
		{"seguridad", elVeredictoQueRechaza, EvidenciaNinguna},
	})
	res, deb, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if !res.Decided || res.Winner != elVeredictoQueRechaza {
		t.Fatalf("el rechazo tiene que poder cerrar sin ninguna evidencia: si esto falla, lo construido es una MÁQUINA DE APROBAR POR INCAPACIDAD. res=%+v", res)
	}
	if res.Gated {
		t.Error("el rechazo no se gatea: marcarlo como frenado sería la misma trampa por otra puerta")
	}
	if deb.Status != DebateClosed {
		t.Errorf("el debate tenía que cerrar, obtuve %q", deb.Status)
	}
}

// --- E5: un solo voto determinístico desarma la compuerta ------------------------------

func TestE5UnSoloVotoDeterministicoAlcanza(t *testing.T) {
	e, d := panelConCompuerta(t, 2, [][3]string{
		{"correctitud", elVeredictoAprobatorio, EvidenciaDeterministica},
		{"seguridad", elVeredictoAprobatorio, EvidenciaNinguna},
	})
	res, _, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	// Es un PISO, no una mayoría paralela. Pedir que la mayoría traiga evidencia sería un
	// segundo quórum encubierto, y el quórum está documentado como «mínimo de votos».
	if !res.Decided || res.Winner != elVeredictoAprobatorio {
		t.Errorf("alcanza con UNO que haya verificado; res=%+v", res)
	}
	if res.Gated {
		t.Error("con evidencia determinística la compuerta no interviene")
	}
}

// --- E6: la compuerta no reemplaza al quórum -------------------------------------------

func TestE6LaCompuertaNoReemplazaAlQuorum(t *testing.T) {
	// Un solo voto, determinístico, con quórum 2: la evidencia está, el quórum no.
	e, d := panelConCompuerta(t, 2, [][3]string{
		{"correctitud", elVeredictoAprobatorio, EvidenciaDeterministica},
	})
	res, _, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if res.Decided {
		t.Error("la evidencia no compra el quórum: siguen siendo dos requisitos")
	}
	if res.Gated {
		t.Error("acá lo que faltó fue quórum, no evidencia: decir «compuerta» mandaría a correr pruebas que ya se corrieron")
	}
	if !strings.Contains(res.Reason, "quórum") {
		t.Errorf("el motivo tiene que ser el quórum; obtuve %q", res.Reason)
	}
}

// --- E7: sin compuerta declarada, nada cambia ------------------------------------------

func TestE7SinCompuertaElComportamientoEsElDeSiempre(t *testing.T) {
	e := newTestEngine(t)
	d, _ := e.OpenDebate("tema", 1, 2, "") // sin gated_choice
	for _, a := range []string{"ana", "beto"} {
		if err := e.CastVote(d.ID, a, elVeredictoAprobatorio, "m", EvidenciaNinguna); err != nil {
			t.Fatalf("vote: %v", err)
		}
	}
	res, deb, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if !res.Decided || res.Winner != elVeredictoAprobatorio || deb.Status != DebateClosed {
		t.Errorf("sin compuerta el aprobado cierra como siempre; res=%+v status=%q", res, deb.Status)
	}
	if res.Gated {
		t.Error("sin compuerta declarada no puede haber nada frenado")
	}
}

// --- E8: frenado se puede destrabar corriendo las pruebas ------------------------------

func TestE8ElDebateFrenadoSeDestrabaConEvidencia(t *testing.T) {
	e, d := panelConCompuerta(t, 2, [][3]string{
		{"correctitud", elVeredictoAprobatorio, EvidenciaInferida},
		{"seguridad", elVeredictoAprobatorio, EvidenciaInferida},
	})
	if res, _, _ := e.TallyDebate(d.ID); res.Decided {
		t.Fatal("precondición: el primer tally tenía que quedar frenado")
	}
	// El lente va, corre las comprobaciones y re-vota declarándolo. Re-votar reemplaza.
	if err := e.CastVote(d.ID, "correctitud", elVeredictoAprobatorio, "modelo-de-correctitud", EvidenciaDeterministica); err != nil {
		t.Fatalf("re-voto: %v", err)
	}
	res, deb, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally 2: %v", err)
	}
	if !res.Decided || deb.Status != DebateClosed {
		t.Errorf("con la evidencia declarada el debate tiene que poder cerrar; res=%+v", res)
	}
}

// --- E9: la compuerta mira el choice GATEADO, no cualquiera ----------------------------

func TestE9SoloSeGateaElChoiceDeclarado(t *testing.T) {
	e := newTestEngine(t)
	// Se gatea un choice que NADIE vota: el ganador es otro y no lo toca la compuerta.
	d, _ := e.OpenDebate("tema", 1, 0, "un_choice_que_nadie_vota")
	for _, a := range []string{"ana", "beto"} {
		if err := e.CastVote(d.ID, a, elVeredictoAprobatorio, "m", EvidenciaNinguna); err != nil {
			t.Fatalf("vote: %v", err)
		}
	}
	res, _, err := e.TallyDebate(d.ID)
	if err != nil {
		t.Fatalf("tally: %v", err)
	}
	if !res.Decided || res.Gated {
		t.Errorf("la compuerta sólo alcanza al choice declarado; res=%+v", res)
	}
}
