package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/transcripts"
)

// Las pruebas de este archivo salen de la revisión adversarial de `uso-agente --contexto`
// (2026-09-26): cada una custodia una regla que el instrumento de antes/después no puede perder sin
// que CI se entere, porque un «después» medido con otra regla no es comparable con el «antes».

func medirContextoDesde(t *testing.T, raiz, desde string) *MedicionContexto {
	t.Helper()
	v, err := transcripts.ParsearVentana(desde, "")
	if err != nil {
		t.Fatal(err)
	}
	inf, err := medirContexto(raiz, v, nil)
	if err != nil || inf.Principal.MedicionContexto == nil {
		t.Fatalf("medirContexto: %v, estado %q", err, inf.Principal.Estado)
	}
	return inf.Principal.MedicionContexto
}

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
// arnes: a="\treturn !strings.HasPrefix(strings.TrimSpace(prompt), \"<task-notification>\") || transcripts.EsDeSistema(\"\")\n"
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

// Sabotaje que la hace fallar: que el hook del turno cambie el título del bloque y el medidor no se
// entere (M1 y M1s caerían a 0 en silencio, y el «después» se leería como un éxito).
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\theader := encabezadoDeMemoria(\"[Musubi — memoria relevante] Contexto de fondo"
// arnes: a="\theader := encabezadoDeMemoria(\"[Musubi — memoria pertinente] Contexto de fondo"
func TestMedirContextoReconoceElBloqueRealDelHook(t *testing.T) {
	// El medidor reconoce el recall del turno por su título. En vez de copiar el literal, esta prueba
	// le pasa la salida REAL del hook: si el hook cambia el título o el formato de los ids, falla acá.
	store := newFakeTurnStore()
	store.recall = memory.RecallResult{Count: 1, Items: []memory.RecallItem{
		{ID: "0199abcd-0001", TopicKey: "deploy/central", Gist: "la receta del deploy del central", ContentHash: "h"}}}
	loop := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250}
	pedido := "armá el deploy del central con la receta"
	in := `{"prompt":"` + pedido + `","session_id":"s-medir"}`
	_, bloque := hookAdditionalContext(t, turnOutput(store, loop, pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in)))
	if bloque == "" {
		t.Fatal("el hook del turno no inyectó nada con un recall de un item")
	}
	raiz := t.TempDir()
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("p1", fxTS("2026-09-20", 1), pedido),
		fxHook("h1", fxTS("2026-09-20", 2), "UserPromptSubmit", bloque),
	)
	m := medirContextoFixture(t, raiz)
	if m.M1.Sustantivos != (ConteoDeTurnos{Turnos: 1, ConMemoria: 1, IDs: 1}) {
		t.Errorf("sustantivos = %+v con la salida real del hook: quería 1 turno con memoria y 1 id — el medidor "+
			"no reconoce el bloque que el hook produce:\n%s", m.M1.Sustantivos, bloque)
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

// Sabotaje que la hace fallar: que lo inyectado ANTES de --desde no alimente la ventana (M3).
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\t\tenLaVentana[id] = true\n"
// arnes: a="\t\t\t\tif cuenta {\n\t\t\t\t\tenLaVentana[id] = true\n\t\t\t\t}\n"
//
// Sabotaje que la hace fallar: contar en el denominador de M3 los ids de antes de --desde.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\t\tif cuenta {\n\t\t\t\t\tm.M3.IDs++\n"
// arnes: a="\t\t\t\tif true {\n\t\t\t\t\tm.M3.IDs++\n"
//
// Sabotaje que la hace fallar: que el arranque cuente aunque llegue después del próximo pedido (M2).
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\tesperandoArranque = false // llegó el pedido siguiente y el arranque no habló\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: que M1 por largo cuente también los avisos del sistema cortos.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\t\t\tsumarTurno(&m.M1s)\n\t\t\t\t} else {\n"
// arnes: a="\t\t\t\t\tsumarTurno(&m.M1s)\n\t\t\t\t\tif esPromptCorto(t.Prompt) {\n\t\t\t\t\t\tsumarTurno(&m.M1.PorLargo)\n\t\t\t\t\t}\n\t\t\t\t} else {\n"
//
// Sabotaje que la hace fallar: contar en M2, M3 y M7 los hooks que no son de Musubi.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\tif !in.DeMusubi() || (in.Evento != transcripts.EventoTurno && in.Evento != transcripts.EventoArranque) {\n"
// arnes: a="\t\t\tif false || (in.Evento != transcripts.EventoTurno && in.Evento != transcripts.EventoArranque) {\n"
//
// Sabotaje que la hace fallar: que el pedido del turno cuente como eco de sí mismo (M7).
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\tmemoria, idsMemoria := false, 0\n"
// arnes: a="\t\tif t.Origen == transcripts.OrigenHumano {\n\t\t\tif p := []rune(normalizarParaEco(t.Prompt)); len(p) >= minimoDeEco {\n\t\t\t\tif len(p) > largoDeEco {\n\t\t\t\t\tp = p[:largoDeEco]\n\t\t\t\t}\n\t\t\t\tpedidos = append(pedidos, string(p))\n\t\t\t}\n\t\t}\n\t\tmemoria, idsMemoria := false, 0\n"
//
// Sabotaje que la hace fallar: buscar el eco sin pasar a minúsculas (M7).
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\treturn strings.Join(strings.Fields(strings.ToLower(s)), \" \")\n"
// arnes: a="\treturn strings.Join(strings.Fields(s), \" \")\n"
func TestMedirContextoInvariantes(t *testing.T) {
	dia := "2026-09-20"
	mem := func(uuid string, seg int, evento, titulo string, ids ...string) map[string]any {
		texto := titulo + " Contexto de fondo."
		for _, id := range ids {
			texto += "\n- gist [id:" + id + "]"
		}
		return fxHook(uuid, fxTS(dia, seg), evento, texto)
	}
	const recall, arranque = "[Musubi — memoria relevante]", "[Musubi — memoria]"

	t.Run("M3 arrastra la ventana de antes de --desde", func(t *testing.T) {
		// Lo inyectado ayer sigue en el contexto hoy: el 7 se repite aunque su primera vez sea de
		// antes de la ventana, y el 7 de ayer no entra al denominador. Es la regla que lleva M3 de
		// 44 a 173 en los reales.
		raiz := t.TempDir()
		escribirTranscript(t, raiz, "-home-x/s1.jsonl",
			fxPrompt("p1", fxTS("2026-09-20", 1), "armá el deploy del central con la receta"),
			fxHook("h1", fxTS("2026-09-20", 2), "UserPromptSubmit", recall+" Contexto.\n- a [id:7]"),
			fxPrompt("p2", fxTS("2026-09-21", 1), "revisá la receta del deploy otra vez"),
			fxHook("h2", fxTS("2026-09-21", 2), "UserPromptSubmit", recall+" Contexto.\n- a [id:7]\n- b [id:8]"),
		)
		if m := medirContextoDesde(t, raiz, "2026-09-21"); m.M3 != (MedidaM3{IDs: 2, Repetidos: 1}) {
			t.Errorf("M3 desde el 21 = %+v, quería 2 ids y 1 repetido (el 7, que ya estaba desde el 20)", m.M3)
		}
	})

	t.Run("M2 corta en el próximo pedido", func(t *testing.T) {
		// Un arranque que llega DESPUÉS del pedido siguiente no es el arranque de la compactación.
		raiz := t.TempDir()
		escribirTranscript(t, raiz, "-home-x/s1.jsonl",
			fxPrompt("p1", fxTS(dia, 1), "armá el deploy del central con la receta"),
			fxBordeDeCompactacion("b1", fxTS(dia, 2)),
			fxPrompt("p2", fxTS(dia, 3), "revisá los números del banco de prompts"),
			mem("h1", 4, "SessionStart", arranque, "3"),
		)
		if m := medirContextoFixture(t, raiz); m.M2 != (MedidaM2{Compactaciones: 1}) {
			t.Errorf("M2 = %+v, quería 1 compactación y ningún arranque: el de Musubi llegó tras el pedido", m.M2)
		}
	})

	t.Run("M1 por largo es sólo de la persona", func(t *testing.T) {
		raiz := t.TempDir()
		escribirTranscript(t, raiz, "-home-x/s1.jsonl",
			fxPrompt("p1", fxTS(dia, 1), "<task-notification>listo</task-notification>"),
			mem("h1", 2, "UserPromptSubmit", recall, "4"),
		)
		m := medirContextoFixture(t, raiz)
		if m.M1.PorLargo != (ConteoDeTurnos{}) || m.M1s.Turnos != 1 {
			t.Errorf("M1 por largo = %+v y M1s = %+v: un aviso corto es M1s, no continuación", m.M1.PorLargo, m.M1s)
		}
	})

	t.Run("sólo cuentan los bloques de Musubi", func(t *testing.T) {
		// El SessionStart de otro plugin tras compactar no es un arranque de Musubi, y sus ids no
		// son memoria de Musubi.
		raiz := t.TempDir()
		escribirTranscript(t, raiz, "-home-x/s1.jsonl",
			fxPrompt("p1", fxTS(dia, 1), "armá el deploy del central con la receta"),
			fxBordeDeCompactacion("b1", fxTS(dia, 2)),
			mem("h1", 3, "SessionStart", "[OtroPlugin — arranque]", "9"),
		)
		m := medirContextoFixture(t, raiz)
		if m.M2 != (MedidaM2{Compactaciones: 1}) || m.M3.IDs != 0 || m.M7.Bloques != 0 {
			t.Errorf("M2 = %+v, M3 = %+v, M7 = %+v: se contó el bloque de otro plugin", m.M2, m.M3, m.M7)
		}
	})

	t.Run("el eco es de un pedido viejo", func(t *testing.T) {
		// El bloque de este mismo turno puede nombrar lo que se acaba de pedir: eso no es eco.
		raiz := t.TempDir()
		pedido := "revisá la receta del deploy del central otra vez"
		escribirTranscript(t, raiz, "-home-x/s1.jsonl",
			fxPrompt("p1", fxTS(dia, 1), "go"),
			fxBordeDeCompactacion("b1", fxTS(dia, 2)),
			fxPrompt("p2", fxTS(dia, 3), pedido),
			fxHook("h1", fxTS(dia, 4), "UserPromptSubmit", recall+" Contexto.\n- "+pedido+" [id:5]"),
		)
		if m := medirContextoFixture(t, raiz); m.M7 != (MedidaM7{Bloques: 1}) {
			t.Errorf("M7 = %+v, quería 1 bloque sin eco: el pedido del turno contó como eco de sí mismo", m.M7)
		}
	})

	t.Run("el eco no mira mayúsculas", func(t *testing.T) {
		raiz := t.TempDir()
		escribirTranscript(t, raiz, "-home-x/s1.jsonl",
			fxPrompt("p1", fxTS(dia, 1), "Armá el DEPLOY del Central con la Receta nueva"),
			fxBordeDeCompactacion("b1", fxTS(dia, 2)),
			fxHook("h1", fxTS(dia, 3), "SessionStart",
				"[Musubi — hilo] Tus últimos pedidos:\n- armá el deploy del central con la receta nueva"),
		)
		if m := medirContextoFixture(t, raiz); m.M7 != (MedidaM7{Bloques: 1, ConEco: 1}) {
			t.Errorf("M7 = %+v, quería 1 bloque con eco: el pedido en mayúsculas se repitió en minúsculas", m.M7)
		}
	})
}
