package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"musubi/internal/transcripts"
)

// Las pruebas de `uso-agente --contexto` usan los fixtures de uso_agente_test.go: la misma forma
// medida en los transcripts reales, escrita literal.

func fxBordeDeCompactacion(uuid, ts string) map[string]any {
	return map[string]any{"type": "system", "subtype": "compact_boundary", "uuid": uuid, "timestamp": ts,
		"compactMetadata": map[string]any{"trigger": "manual"}}
}

func medirContextoFixture(t *testing.T, raiz string) *MedicionContexto {
	t.Helper()
	inf, err := medirContexto(raiz, transcripts.Ventana{}, nil)
	if err != nil {
		t.Fatalf("medirContexto: %v", err)
	}
	if inf.Principal.MedicionContexto == nil {
		t.Fatalf("el hilo principal quedó «%s» con transcripts en disco", inf.Principal.Estado)
	}
	return inf.Principal.MedicionContexto
}

// Sabotaje que la hace fallar: no recordar los uuid ya leídos, así lo reescrito al reanudar se
// vuelve a contar.
// arnes: archivo="internal/transcripts/formato.go"
// arnes: de="\t\t\t\t\tvistos[reg.UUID] = true\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: que el clasificador por largo no mire el largo.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\treturn n > 0 && n <= 3\n"
// arnes: a="\treturn n > 0 && n <= 0\n"
func TestMedirContextoDeduplicaYCuenta(t *testing.T) {
	// Un «sigue» con su bloque de memoria, reescrito entero al reanudar, y una compactación que no
	// sigue ningún arranque de Musubi: M1 tiene que dar 1/1 (no 2/2) y M2, 0/1.
	raiz := t.TempDir()
	dia := "2026-09-20"
	pedido := fxPrompt("u-p1", fxTS(dia, 1), "sigue")
	memoria := fxHook("u-h1", fxTS(dia, 2), "UserPromptSubmit",
		"[Musubi — memoria relevante] Contexto de fondo.\n- (crm) algo al azar [id:7]\n- otra [id:8]")
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		pedido, memoria,
		pedido, memoria, // la reanudación
		fxBordeDeCompactacion("u-b1", fxTS(dia, 3)),
		fxPrompt("u-p2", fxTS(dia, 4), "armá el banco con los prompts reales del dueño"),
	)

	m := medirContextoFixture(t, raiz)
	if got := m.M1.PorLista; got != (ConteoDeTurnos{Turnos: 1, ConMemoria: 1, IDs: 2}) {
		t.Errorf("M1 por lista = %+v, quería 1 turno de continuación con memoria y 2 ids: lo reescrito al "+
			"reanudar se contó otra vez", got)
	}
	if got := m.M1.PorLargo; got != (ConteoDeTurnos{Turnos: 1, ConMemoria: 1, IDs: 2}) {
		t.Errorf("M1 por largo = %+v, quería el mismo «sigue» (1 palabra)", got)
	}
	if got := m.M1.Sustantivos; got.Turnos != 1 || got.ConMemoria != 0 {
		t.Errorf("sustantivos = %+v, quería 1 sin memoria", got)
	}
	if m.M2 != (MedidaM2{Compactaciones: 1}) {
		t.Errorf("M2 = %+v, quería 1 compactación y ningún arranque detrás", m.M2)
	}
	if m.Reanudaciones.Resume != 1 || m.Reanudaciones.Fork != motivoForkSinMedir {
		t.Errorf("reanudaciones = %+v, quería 1 resume y el fork sin medir", m.Reanudaciones)
	}
}

// Sabotaje que la hace fallar: no vaciar la ventana al compactar.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\tenLaVentana = map[string]bool{}\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: no reconocer el arranque de Musubi después de compactar.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\tif in.Evento == transcripts.EventoArranque && esperandoArranque {\n"
// arnes: a="\t\t\tif false && in.Evento == transcripts.EventoArranque && esperandoArranque {\n"
//
// Sabotaje que la hace fallar: olvidar lo que se inyectó en las ventanas anteriores.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\t\t\tantes[id] = true\n"
// arnes: a="\t\t\t\tantes[id] = false\n"
func TestMedirContextoVentanasCompactacionYArranque(t *testing.T) {
	//	ventana 0  pedido → [1 2] · «sigue» → [2 3]          M3: el 2 se repite
	//	compacta   SessionStart de Musubi → [1]             M2 1/1 · M5: el 1 vuelve
	//	           «go» → [4]
	//	compacta   pedido sin arranque                      M2 1/2
	raiz := t.TempDir()
	dia := "2026-09-20"
	mem := func(uuid string, seg int, evento string, ids ...string) map[string]any {
		texto := "[Musubi — memoria relevante] Contexto de fondo."
		if evento == "SessionStart" {
			texto = "[Musubi — memoria] Contexto de fondo."
		}
		for _, id := range ids {
			texto += "\n- gist [id:" + id + "]"
		}
		return fxHook(uuid, fxTS(dia, seg), evento, texto)
	}
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("p1", fxTS(dia, 1), "armá el deploy del central con la receta"),
		mem("h1", 2, "UserPromptSubmit", "1", "2"),
		fxPrompt("p2", fxTS(dia, 3), "sigue"),
		mem("h2", 4, "UserPromptSubmit", "2", "3"),
		fxBordeDeCompactacion("b1", fxTS(dia, 5)),
		fxPrompt("c1", fxTS(dia, 6), "<command-name>/compact</command-name>"),
		mem("h3", 7, "SessionStart", "1"),
		fxPrompt("p3", fxTS(dia, 8), "go"),
		mem("h4", 9, "UserPromptSubmit", "4"),
		fxBordeDeCompactacion("b2", fxTS(dia, 10)),
		fxPrompt("p4", fxTS(dia, 11), "revisá los números del banco de prompts"),
	)

	m := medirContextoFixture(t, raiz)
	if m.M3 != (MedidaM3{IDs: 6, Repetidos: 1}) {
		t.Errorf("M3 = %+v, quería 6 ids y 1 repetido (el 2, en la misma ventana); el 1 del arranque está "+
			"en OTRA ventana y no se repite", m.M3)
	}
	if m.M2 != (MedidaM2{Compactaciones: 2, ConArranque: 1}) {
		t.Errorf("M2 = %+v, quería 2 compactaciones y 1 seguida del arranque de Musubi", m.M2)
	}
	if m.M5 != (MedidaM5{IDs: 2, Reinyectados: 1, VentanasConReinyeccion: 1}) {
		t.Errorf("M5 = %+v, quería 2 ids tras compactar (1 y 4), 1 que vuelve de la ventana anterior", m.M5)
	}
	if m.M1.PorLista != (ConteoDeTurnos{Turnos: 2, ConMemoria: 2, IDs: 3}) {
		t.Errorf("M1 por lista = %+v, quería «sigue» y «go», los dos con memoria (3 ids)", m.M1.PorLista)
	}
}

// Sabotaje que la hace fallar: que un aviso del sistema cuente como pedido humano.
// arnes: archivo="internal/transcripts/prompts.go"
// arnes: de="\tcase EsDeSistema(p):\n"
// arnes: a="\tcase false && EsDeSistema(p):\n"
//
// Sabotaje que la hace fallar: contar como eco la línea con que el corrector avisa.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\tif strings.HasPrefix(strings.TrimSpace(l), transcripts.PrefijoDeCorreccion) {\n"
// arnes: a="\t\tif false && strings.HasPrefix(strings.TrimSpace(l), transcripts.PrefijoDeCorreccion) {\n"
//
// Sabotaje que la hace fallar: no buscar el eco.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\tif strings.Contains(cuerpo, p) {\n"
// arnes: a="\t\tif false && strings.Contains(cuerpo, p) {\n"
func TestMedirContextoAvisosDelSistemaYEco(t *testing.T) {
	// Un aviso de tarea de fondo trae memoria: es M1s, no un pedido. Después de compactar, un bloque
	// de Musubi que repite el pedido del dueño es eco (M7); la línea de corrección de tipeo, que
	// repite un pedazo del prompt a propósito, no lo es.
	raiz := t.TempDir()
	dia := "2026-09-20"
	pedido := "sigue con el certificado TLS del cerebro central, fijate el nombre"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("p0", fxTS(dia, 1), "<task-notification>\n<summary>Monitor event: build notification</summary>"),
		fxHook("h0", fxTS(dia, 2), "UserPromptSubmit", "[Musubi — memoria relevante] Contexto.\n- architecture/notifications [id:9]"),
		fxPrompt("p1", fxTS(dia, 3), pedido),
		fxHook("h1", fxTS(dia, 4), "UserPromptSubmit", "[Musubi — memoria relevante] Contexto.\n- tls [id:5]"),
		fxBordeDeCompactacion("b1", fxTS(dia, 5)),
		fxHook("h2", fxTS(dia, 6), "SessionStart", "[Musubi — hilo] Tus últimos pedidos:\n- "+pedido),
		fxPrompt("p2", fxTS(dia, 7), "ahora la parte del cliente"),
		fxHook("h3", fxTS(dia, 8), "UserPromptSubmit",
			"[Musubi — memoria relevante] Contexto.\n"+transcripts.PrefijoDeCorreccion+pedido+"» por «certifcado»\n- tls [id:5]"),
	)

	m := medirContextoFixture(t, raiz)
	if m.M1s != (ConteoDeTurnos{Turnos: 1, ConMemoria: 1, IDs: 1}) {
		t.Errorf("M1s = %+v, quería el aviso de la tarea de fondo con su memoria", m.M1s)
	}
	if n := m.M1.Sustantivos.Turnos + m.M1.PorLista.Turnos; n != 2 {
		t.Errorf("turnos humanos = %d, quería 2: el aviso del sistema se contó como pedido", n)
	}
	if m.M7 != (MedidaM7{Bloques: 2, ConEco: 1}) {
		t.Errorf("M7 = %+v, quería 2 bloques tras compactar y 1 con eco (el que repite el pedido; la "+
			"línea de corrección no cuenta)", m.M7)
	}
}

// Sabotaje que la hace fallar: decir 0 en vez de «sin medir».
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\tif m.Transcripts == 0 {\n"
// arnes: a="\tif false {\n"
//
// Sabotaje que la hace fallar: medir también las sesiones hijas.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\tif a.Subagente {\n\t\t\tinf.SubagentesNoMedidos++\n\t\t\treturn nil\n"
// arnes: a="\t\tif a.Subagente {\n\t\t\tinf.SubagentesNoMedidos++\n"
func TestMedirContextoSinSesionesPrincipalesDiceSinMedir(t *testing.T) {
	// Sólo un subagente en la ventana: las hijas no se miden, así que el alcance principal no tiene
	// nada que decir, y lo dice como «sin medir», sin una sola clave numérica en el JSON.
	raiz := t.TempDir()
	escribirTranscript(t, raiz, "-home-x/s1/subagents/agent-a1.jsonl",
		fxPrompt("s-p1", fxTS("2026-09-20", 1), "sigue"),
		fxHook("s-h1", fxTS("2026-09-20", 2), "UserPromptSubmit", "[Musubi — memoria relevante]\n- x [id:1]"),
	)
	var js, errOut bytes.Buffer
	if code := usoAgente([]string{"--dir", raiz, "--contexto", "--json"}, &js, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	var inf map[string]json.RawMessage
	if err := json.Unmarshal(js.Bytes(), &inf); err != nil {
		t.Fatalf("el --json no es JSON: %v\n%s", err, js.String())
	}
	var principal map[string]any
	if err := json.Unmarshal(inf["principal"], &principal); err != nil {
		t.Fatalf("principal no es un objeto: %v", err)
	}
	if principal["estado"] != "sin medir" {
		t.Errorf("principal.estado = %v, quería «sin medir»: sólo hay una sesión hija", principal["estado"])
	}
	for _, clave := range []string{"transcripts", "m1_continuacion_con_memoria", "m2_compactacion_con_arranque"} {
		if v, esta := principal[clave]; esta {
			t.Errorf("principal sin medir trae %q = %v: un cero que se lee como «medí y no hubo»", clave, v)
		}
	}
	if string(inf["subagentes_no_medidos"]) != "1" {
		t.Errorf("subagentes_no_medidos = %s, quería 1", inf["subagentes_no_medidos"])
	}
}
