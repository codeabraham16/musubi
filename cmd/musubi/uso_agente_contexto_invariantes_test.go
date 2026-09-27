package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/transcripts"
)

// Las pruebas de este archivo salen de la revisión adversarial de `uso-agente --contexto`
// (2026-09-26): cada una custodia una regla que el instrumento de antes/después no puede perder sin
// que CI se entere, porque un «después» medido con otra regla no es comparable con el «antes».

// Sabotaje que la hace fallar: volver a publicar un número de resume.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\tReanudaciones: Reanudaciones{Resume: motivoResumeSinMedir, Fork: motivoForkSinMedir}}\n"
// arnes: a="\t\tReanudaciones: Reanudaciones{Resume: \"0\", Fork: motivoForkSinMedir}}\n"
func TestMedirContextoResumeSaleSinMedir(t *testing.T) {
	// Las dos formas medidas en los reales. Un tramo reescrito pegado a una compactación, sin hueco
	// de tiempo: así son los 67 tramos de la historia de Musubi y Altura, y no hubo un solo /resume.
	// Y una sesión retomada tras el corte del 09-26, que no dejó tramo. El informe no puede decir
	// «resume 1» por la primera ni «resume 0» por la segunda: no lo mide, y lo dice.
	raiz := t.TempDir()
	dia := "2026-09-20"
	p1 := fxPrompt("p1", fxTS(dia, 1), "armá el deploy del central con la receta")
	h1 := fxHook("h1", fxTS(dia, 2), "UserPromptSubmit", "[Musubi — memoria relevante] Contexto.\n- a [id:1]")
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		p1, h1,
		p1, h1, // el tramo reescrito, sin hueco
		map[string]any{"type": "system", "subtype": "stop_hook_summary", "uuid": "s1", "timestamp": fxTS(dia, 3)},
		fxBordeDeCompactacion("b1", fxTS(dia, 4)),
	)
	escribirTranscript(t, raiz, "-home-x/s2.jsonl",
		fxPrompt("q1", "2026-09-26T20:50:00.000Z", "armá el deploy del central con la receta"),
		fxPrompt("q2", "2026-09-26T22:12:00.000Z", "seguimos con el banco después del corte"),
	)

	var js, tabla, errOut bytes.Buffer
	if code := usoAgente([]string{"--dir", raiz, "--contexto", "--json"}, &js, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	var inf struct {
		Principal struct {
			Reanudaciones map[string]json.RawMessage `json:"reanudaciones"`
		} `json:"principal"`
	}
	if err := json.Unmarshal(js.Bytes(), &inf); err != nil {
		t.Fatalf("el --json no es JSON: %v", err)
	}
	for _, clave := range []string{"resume", "fork"} {
		var motivo string
		if json.Unmarshal(inf.Principal.Reanudaciones[clave], &motivo) != nil || !strings.HasPrefix(motivo, "sin medir") {
			t.Errorf("reanudaciones.%s = %s: quería «sin medir» con el motivo, no un número que el transcript "+
				"no sostiene", clave, inf.Principal.Reanudaciones[clave])
		}
	}
	if code := usoAgente([]string{"--dir", raiz, "--contexto"}, &tabla, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	for _, l := range strings.Split(tabla.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "resume") && !strings.Contains(l, "sin medir") {
			t.Errorf("la tabla dice %q: quería «sin medir»", l)
		}
	}
}

// Sabotaje que la hace fallar: que el aviso de tareas vuelva a tener su propia tabla de prefijos.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\treturn !transcripts.EsDeSistema(prompt)\n"
// arnes: a="\treturn !strings.HasPrefix(strings.TrimSpace(prompt), \"<task-notification>\")\n"
func TestUnSoloClasificadorDePromptHumano(t *testing.T) {
	// El hook del turno decide dos veces si un prompt lo escribió la persona: para el recall
	// (EsDeSistema, que también usa el medidor) y para el aviso de tareas. Eran dos tablas y
	// discrepaban en estos cinco; ninguno lo escribe la persona.
	for _, c := range []struct {
		prompt  string
		sistema bool
	}{
		{"<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>", true},
		{"<cross-session-message from=\"otra\">hola</cross-session-message>", true},
		{"<agent-message>informe del subagente</agent-message>", true},
		{"Another Claude session sent a message: hola", true},
		{"<command-name>/model</command-name>", true},
		{"<local-command-stdout>Set model</local-command-stdout>", true},
		{"<system-reminder>x</system-reminder>", true},
		{"armá el deploy del central con la receta", false},
		{"sigue", false},
	} {
		if got := transcripts.EsDeSistema(c.prompt); got != c.sistema {
			t.Errorf("transcripts.EsDeSistema(%.40q) = %v, quería %v", c.prompt, got, c.sistema)
		}
		if ajeno := !esTurnoDeLaPersona(c.prompt); ajeno != c.sistema {
			t.Errorf("esTurnoDeLaPersona(%.40q) dice ajeno=%v, y EsDeSistema dice %v: dos criterios de «lo "+
				"escribió la persona» en el mismo hook", c.prompt, ajeno, c.sistema)
		}
	}
}

// Sabotaje que la hace fallar: contar en M1s los avisos que el hook no ve.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\t\tif transcripts.LoVeElHook(t.Prompt) {\n"
// arnes: a="\t\t\t\tif true || transcripts.LoVeElHook(t.Prompt) {\n"
func TestMedirContextoM1sSoloCuentaLoQueVeElHook(t *testing.T) {
	// Un slash-command deja dos registros (`<command-name>` y `<local-command-stdout>`) que el hook
	// del turno nunca ve: 0 de 296 y 1 de 283 con un bloque detrás en los reales. No pueden recibir
	// memoria, así que no van al denominador; se informan aparte.
	raiz := t.TempDir()
	dia := "2026-09-20"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("p0", fxTS(dia, 1), "<task-notification>\n<summary>Monitor event</summary>"),
		fxHook("h0", fxTS(dia, 2), "UserPromptSubmit", "[Musubi — memoria relevante] Contexto.\n- n [id:9]"),
		fxPrompt("c1", fxTS(dia, 3), "<command-name>/model</command-name>"),
		fxPrompt("c2", fxTS(dia, 4), "<local-command-stdout>Set model</local-command-stdout>"),
	)
	m := medirContextoFixture(t, raiz)
	if m.M1s != (ConteoDeTurnos{Turnos: 1, ConMemoria: 1, IDs: 1}) || m.AvisosQueElHookNoVe != 2 {
		t.Errorf("M1s = %+v y fuera del hook %d: quería el aviso de la tarea (1/1) y los 2 registros del "+
			"slash-command aparte", m.M1s, m.AvisosQueElHookNoVe)
	}
	if n := m.PromptsPorOrigen[string(transcripts.OrigenSistema)]; n != 3 {
		t.Errorf("prompts del sistema = %d, quería 3: el conteo por origen no cambia", n)
	}
}
