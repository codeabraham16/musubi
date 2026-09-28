package recalleval

import (
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// TestConfigsNoDivergenDeProduccion es el trinquete que faltaba: exige que lo que el banco mide
// tenga los MISMOS valores que config.Default() declara.
//
// El defecto que fija: las configs vivían escritas a mano en un _test.go con VectorFloor y
// MMRLambda en el cero de Go, contra 0.30 y 0.75 de producción. El gate de CI defendía un ranker
// sin piso de coseno y con MMR apagado — o sea, uno que nadie corre. Nada avisaba porque nada
// comparaba los dos lados. Esto lo compara.
//
// Sabotaje: el brazo con alcance lo pierde y mide el turno federado de antes.
// arnes: archivo="internal/recalleval/configs.go"
// arnes: de="\tc.Opts = memory.OpcionesDeRecallDelTurno(m, alcance)\n"
// arnes: a="\tc.Opts = memory.OpcionesDeRecallDelTurno(m, memory.AlcanceDelTurno{})\n"
func TestConfigsNoDivergenDeProduccion(t *testing.T) {
	m := config.Default().Memory
	o := OptsDeProduccion()

	casos := []struct {
		campo       string
		banco, prod any
	}{
		{"Stemming", o.Stemming, m.RecallStemming},
		{"Cooccurrence", o.Cooccurrence, m.RecallCooccurrence},
		{"GraphCentrality", o.GraphCentrality, m.RecallGraphCentrality},
		{"VectorFloor", o.VectorFloor, m.VectorFloor},
		{"MMRLambda", o.MMRLambda, m.MMRLambda},
		{"CandidatePool", o.CandidatePool, m.CandidatePool},
		{"TokenBudget", o.TokenBudget, m.RecallTokenBudget},
	}
	for _, c := range casos {
		if c.banco != c.prod {
			t.Errorf("%s: el banco mide %v y producción declara %v — el gate defendería un ranker que nadie corre",
				c.campo, c.banco, c.prod)
		}
	}

	// El brazo de producción tiene que llevar TODO lo declarado, MMR incluido: es el único que
	// responde "cómo se comporta el sistema tal como está configurado".
	if p := ConfigProduccion(); p.Opts.MMRLambda != m.MMRLambda || p.Opts.VectorFloor != m.VectorFloor {
		t.Errorf("ConfigProduccion no lleva los defaults: MMRLambda=%v (esperaba %v), VectorFloor=%v (esperaba %v)",
			p.Opts.MMRLambda, m.MMRLambda, p.Opts.VectorFloor, m.VectorFloor)
	}
	// Y el léxico tiene que ser léxico DE VERDAD: sin señal vectorial, o el contraste no significa nada.
	if l := ConfigLexica(); l.UseVector {
		t.Error("ConfigLexica enciende la señal vectorial: entonces no es la línea base de nada")
	}
	// El híbrido sí conserva el piso de producción: es el punto del brazo.
	if h := ConfigHibrida(); !h.UseVector || h.Opts.VectorFloor != m.VectorFloor {
		t.Errorf("ConfigHibrida: UseVector=%v VectorFloor=%v (esperaba true y %v)",
			h.UseVector, h.Opts.VectorFloor, m.VectorFloor)
	}

	// EL BRAZO DEL TURNO, con las mismas filas. Sus opciones salen de la fuente única del hook
	// (TestConfigDelBancoEsLaDelHook), pero esa fuente también podría divergir del yaml: acá se ve.
	// CandidatePool y TokenBudget no tienen fila a propósito: el hook nunca los tomó de memory.* (el
	// pool es el del motor y el presupuesto es loop.recall_budget).
	tu := ConfigTurno().Opts
	for _, c := range []struct {
		campo       string
		banco, prod any
	}{
		{"Stemming", tu.Stemming, m.RecallStemming},
		{"Cooccurrence", tu.Cooccurrence, m.RecallCooccurrence},
		{"GraphCentrality", tu.GraphCentrality, m.RecallGraphCentrality},
		{"VectorFloor", tu.VectorFloor, m.VectorFloor},
		{"MMRLambda", tu.MMRLambda, m.MMRLambda},
	} {
		if c.banco != c.prod {
			t.Errorf("turno · %s: el banco mide %v y producción declara %v", c.campo, c.banco, c.prod)
		}
	}

	// EL ALCANCE DEL TURNO, que sale de loop.* y no de memory.*: con el modo y el tope de fábrica, el
	// brazo que mide la mezcla de proyectos corre acotado al propio y con el tope del yaml. Si esto
	// divergiera, el banco de la mezcla mediría el turno federado de antes y diría que no cambió nada.
	l := config.Default().Loop
	al := ConfigTurnoConAlcance(m, memory.AlcanceDelTurnoSegun(l.RecallOtrosProyectos, l.RecallOtrosMax, "propio")).Opts
	for _, c := range []struct {
		campo       string
		banco, prod any
	}{
		{"TopeOtrosProyectos", al.TopeOtrosProyectos, l.RecallOtrosMax},
		{"ProjectScope", al.ProjectScope, "propio"},
		{"Federate", al.Federate, false},
		{"Stemming", al.Stemming, m.RecallStemming},
	} {
		if c.banco != c.prod {
			t.Errorf("turno con alcance · %s: el banco mide %v y producción declara %v", c.campo, c.banco, c.prod)
		}
	}
}
