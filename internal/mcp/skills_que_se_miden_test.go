package mcp

import (
	"errors"
	"strings"
	"testing"
	"time"

	"musubi/internal/memory"
	"musubi/internal/skills"
)

// Invariantes A1–A8 del spec «El arsenal se mide» (specs/skills-que-se-miden/) del lado MCP.
//
// Reusa el arsenal de skills_por_niveles_test.go: por-glob (`*.go`), solo-comodin y otro-comodin,
// que son las tres formas de matchear que importan para contar.

func servidorQueCuenta(t *testing.T, sink ledgerSink) *McpServer {
	t.Helper()
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	s.ledger = newUsageLedger(sink, time.Hour) // intervalo largo: el flush lo hacen los tests
	t.Cleanup(func() { s.ledger.close() })
	escribirSkill(t, root, "por-glob.yaml", skillPorGlob)
	escribirSkill(t, root, "solo-comodin.yaml", skillComodin)
	escribirSkill(t, root, "otro-comodin.yaml", skillOtroComodin)
	return s
}

// buscarConteo devuelve el primer evento que coincide con skill+kind.
func buscarConteo(evs []memory.SkillEvent, skill, kind string) (memory.SkillEvent, bool) {
	for _, e := range evs {
		if e.Skill == skill && e.Kind == kind {
			return e, true
		}
	}
	return memory.SkillEvent{}, false
}

// ★ A1 — RESOLVER CUENTA UNA ACTIVACIÓN POR SKILL MATCHEADA, CON SU EVIDENCIA.
//
// Sin la evidencia el dato no sirve para la lectura que importa —«matcheó siempre por comodín»—,
// que es la que descubre un '*' que merece volverse `applies_to`.
func TestA1CadaActivacionSeCuentaConSuEvidencia(t *testing.T) {
	sink := &sinkEspia{}
	s := servidorQueCuenta(t, sink)

	if _, rerr := call(t, s, "musubi_resolve_skills", map[string]interface{}{
		"modified_files": []string{"main.go"},
	}); rerr != nil {
		t.Fatalf("resolver: %+v", rerr)
	}
	s.ledger.flush()

	evs := sink.conteos()
	glob, ok := buscarConteo(evs, "por-glob", memory.UsoResuelta)
	if !ok {
		t.Fatalf("no se contó la activación de por-glob; conteos: %+v", evs)
	}
	if glob.Evidence != memory.EvidenciaGlob {
		t.Errorf("por-glob: evidencia = %q, esperaba %q", glob.Evidence, memory.EvidenciaGlob)
	}
	comodin, ok := buscarConteo(evs, "solo-comodin", memory.UsoResuelta)
	if !ok {
		t.Fatal("no se contó la activación de solo-comodin: matchea cualquier archivo")
	}
	if comodin.Evidence != memory.EvidenciaComodin {
		t.Errorf("solo-comodin: evidencia = %q, esperaba %q", comodin.Evidence, memory.EvidenciaComodin)
	}
}

// ★ A2 (lado MCP) — SÓLO SE CUENTA `body_sent` PARA QUIEN SE LLEVÓ EL CUERPO.
func TestA2SoloCuentaCuerpoQuienSeLoLlevo(t *testing.T) {
	sink := &sinkEspia{}
	s := servidorQueCuenta(t, sink)

	if _, rerr := call(t, s, "musubi_resolve_skills", map[string]interface{}{
		"modified_files": []string{"main.go"},
	}); rerr != nil {
		t.Fatalf("resolver: %+v", rerr)
	}
	s.ledger.flush()

	evs := sink.conteos()
	if _, ok := buscarConteo(evs, "por-glob", memory.UsoCuerpoEnviado); !ok {
		t.Error("por-glob matcheó por un glob real: su cuerpo viajó y tiene que contarse")
	}
	if _, ok := buscarConteo(evs, "solo-comodin", memory.UsoCuerpoEnviado); ok {
		t.Error("solo-comodin entró por comodín: su cuerpo NO viajó y no puede contarse como enviado")
	}
}

// ★ A3 — PEDIR UNA SKILL POR NOMBRE CUENTA COMO PEDIDO DE CUERPO; mirar la lista entera, no.
func TestA3PedirPorNombreCuentaMirarLaListaNo(t *testing.T) {
	sink := &sinkEspia{}
	s := servidorQueCuenta(t, sink)

	if _, rerr := call(t, s, "musubi_list_skills", map[string]interface{}{"query": "por-glob"}); rerr != nil {
		t.Fatalf("list_skills: %+v", rerr)
	}
	s.ledger.flush()
	if _, ok := buscarConteo(sink.conteos(), "por-glob", memory.UsoCuerpoPedido); !ok {
		t.Errorf("pedir una skill por nombre es el nivel 2 y tiene que contarse; conteos: %+v", sink.conteos())
	}

	sink2 := &sinkEspia{}
	s2 := servidorQueCuenta(t, sink2)
	if _, rerr := call(t, s2, "musubi_list_skills", nil); rerr != nil {
		t.Fatalf("list_skills sin query: %+v", rerr)
	}
	s2.ledger.flush()
	if n := len(sink2.conteos()); n != 0 {
		t.Errorf("mirar el arsenal entero no es pedir una skill; se contaron %d: %+v", n, sink2.conteos())
	}
}

// A4 — UN SINK QUE FALLA NO HACE FALLAR LA HERRAMIENTA.
func TestA4UnSinkRotoNoTumbaLaResolucion(t *testing.T) {
	sink := &sinkEspia{fallaSk: errors.New("base caída simulada")}
	s := servidorQueCuenta(t, sink)

	for i := 0; i < 2; i++ {
		if _, rerr := call(t, s, "musubi_resolve_skills", map[string]interface{}{
			"modified_files": []string{"main.go"},
		}); rerr != nil {
			t.Fatalf("la resolución falló por culpa de la telemetría: %+v", rerr)
		}
		s.ledger.flush() // el error se logea y se sigue
	}
}

// ★ A5 — EL CONTEO NO ESCRIBE EN EL CAMINO CALIENTE.
//
// El handler corre con dispatchMu tomado: escribir a disco ahí alargaría el lock de toda tool, y la
// goroutine de flush no puede tomar dispatchMu sin deadlock.
func TestA5ElConteoNoTocaElDiscoEnElCaminoCaliente(t *testing.T) {
	sink := &sinkEspia{}
	s := servidorQueCuenta(t, sink)

	if _, rerr := call(t, s, "musubi_resolve_skills", map[string]interface{}{
		"modified_files": []string{"main.go"},
	}); rerr != nil {
		t.Fatalf("resolver: %+v", rerr)
	}

	if s.ledger.pendientesSkills() == 0 {
		t.Error("los conteos tienen que quedar en el buffer, no bajar a la base dentro del handler")
	}
	if n := len(sink.conteos()); n != 0 {
		t.Errorf("el sink recibió %d conteos ANTES del flush: se escribió en el camino caliente", n)
	}
}

// A6 — EL BUFFER TIENE TECHO y lo descartado no crece sin límite.
func TestA6ElBufferDeConteosTieneTecho(t *testing.T) {
	sink := &sinkEspia{}
	s := servidorQueCuenta(t, sink)

	exceso := make([]memory.SkillEvent, ledgerBufferCap+500)
	for i := range exceso {
		exceso[i] = memory.SkillEvent{Skill: "x", Evidence: memory.EvidenciaGlob, Kind: memory.UsoResuelta}
	}
	s.ledger.recordSkills(exceso)

	if n := s.ledger.pendientesSkills(); n > ledgerBufferCap {
		t.Errorf("el buffer creció hasta %d, por encima del techo %d", n, ledgerBufferCap)
	}
}

// ★ A8 — UNA SKILL DEL ARSENAL QUE NUNCA MATCHEÓ APARECE CON 0, NO AUSENTE.
//
// «0 activaciones» es la lectura más accionable de las tres, y una fila ausente es indistinguible
// de «no hay dato».
func TestA8LaQueNuncaMatcheoApareceConCero(t *testing.T) {
	sink := &sinkEspia{}
	s := servidorQueCuenta(t, sink)

	out, rerr := call(t, s, "musubi_skill_usage", nil)
	if rerr != nil {
		t.Fatalf("musubi_skill_usage: %+v", rerr)
	}
	txt := out.(CallToolResponse).Content[0].Text

	for _, nombre := range []string{"por-glob", "solo-comodin", "otro-comodin"} {
		if !strings.Contains(txt, nombre) {
			t.Errorf("falta %s: una skill instalada sin actividad tiene que aparecer con 0\n%s", nombre, txt)
		}
	}
	if !strings.Contains(txt, "muerta") {
		t.Errorf("sin conteos, las tres son candidatas «muerta»\n%s", txt)
	}
}

// ★ EL VOCABULARIO DE EVIDENCIA ES EL MISMO DE LOS DOS LADOS.
//
// `memory` no importa `skills` a propósito —la capa de memoria no tiene por qué saber cómo se
// resuelve una skill—, así que los strings están escritos dos veces. Esta prueba es lo que impide
// que se separen: si se separan, los contadores se llenan de evidencias que la lectura nunca cuenta
// y el dato se ve vacío sin que nada falle.
func TestElVocabularioDeEvidenciaNoSeSepara(t *testing.T) {
	pares := []struct {
		enSkills skills.ComoMatcheo
		enMemory string
	}{
		{skills.PorAlcance, memory.EvidenciaAlcance},
		{skills.PorGlob, memory.EvidenciaGlob},
		{skills.PorComodin, memory.EvidenciaComodin},
	}
	for _, p := range pares {
		if string(p.enSkills) != p.enMemory {
			t.Errorf("el vocabulario se separó: skills=%q memory=%q", p.enSkills, p.enMemory)
		}
	}
}
