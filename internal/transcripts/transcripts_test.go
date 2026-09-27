package transcripts

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// LAS PRUEBAS LEEN FIXTURES, NUNCA LOS TRANSCRIPTS REALES: cambian a cada minuto y traen lo que
// una persona le dijo al agente. Los tipos de registro y de adjunto van LITERALES —son un hecho del
// formato de Claude Code— y no se derivan de las constantes que la prueba custodia.

func fxEscribir(t *testing.T, registros ...map[string]any) string {
	t.Helper()
	var b bytes.Buffer
	for _, r := range registros {
		linea, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(linea)
		b.WriteByte('\n')
	}
	ruta := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(ruta, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func fxUsuario(uuid, ts string, contenido any) map[string]any {
	return map[string]any{"type": "user", "uuid": uuid, "timestamp": ts,
		"message": map[string]any{"role": "user", "content": contenido}}
}

func fxHookDe(uuid, ts, evento, texto string) map[string]any {
	return map[string]any{"type": "attachment", "uuid": uuid, "timestamp": ts,
		"attachment": map[string]any{"type": "hook_additional_context", "hookName": evento,
			"hookEvent": evento, "content": []string{texto}}}
}

func fxBorde(uuid, ts string) map[string]any {
	return map[string]any{"type": "system", "subtype": "compact_boundary", "uuid": uuid, "timestamp": ts,
		"compactMetadata": map[string]any{"trigger": "manual"}}
}

// Sabotaje que la hace fallar: no reconocer el aviso de una tarea de fondo.
// arnes: archivo="internal/transcripts/prompts.go"
// arnes: de="var prefijosDeSistema = []string{\"<task-notification\", "
// arnes: a="var prefijosDeSistema = []string{\"<task-notificacion\", "
//
// Sabotaje que la hace fallar: no saltear los espacios del comienzo.
// arnes: archivo="internal/transcripts/prompts.go"
// arnes: de="\tp := strings.TrimLeftFunc(prompt, unicode.IsSpace)\n"
// arnes: a="\tp := prompt\n"
func TestEsDeSistemaReconoceLosAvisos(t *testing.T) {
	// Los cuatro comienzos medidos en los transcripts de esta máquina, y los parecidos que NO son
	// avisos: un pedido que nombra la etiqueta, y un prompt pegado que empieza con otra etiqueta.
	casos := map[string]bool{
		"<task-notification>\n<task-id>b1</task-id>\n<summary>Monitor event</summary>": true,
		"  \n<command-name>/compact</command-name>":                                    true,
		"<local-command-stdout>Compacted</local-command-stdout>":                       true,
		"<system-reminder>ojo</system-reminder>":                                       true,
		"sigue con el <task-notification> que llegó":                                   false,
		"<mission>armá el banco</mission>":                                             false,
		"":                                                                             false,
	}
	for prompt, quiere := range casos {
		if got := EsDeSistema(prompt); got != quiere {
			t.Errorf("EsDeSistema(%q) = %v, quería %v", prompt, got, quiere)
		}
	}
}

// Sabotaje que la hace fallar: una palabra de contenido entra a la lista.
// arnes: archivo="internal/transcripts/prompts.go"
// arnes: de="\tsigue segui sigamos seguimos siga sigo continua"
// arnes: a="\tglobal sigue segui sigamos seguimos siga sigo continua"
//
// Sabotaje que la hace fallar: comparar sin plegar los acentos.
// arnes: archivo="internal/transcripts/prompts.go"
// arnes: de="\t\t\tout = append(out, plegarAcentos.Replace(b.String()))\n"
// arnes: a="\t\t\tout = append(out, b.String())\n"
func TestLaListaDeContinuacionNoSeComeElContenido(t *testing.T) {
	// Prompts reales del dueño, cortos: los de arriba dicen qué hacer y tienen que buscar memoria;
	// los de abajo sólo piden seguir. Un falso «continuación» le quita la memoria a un pedido de
	// verdad, así que la tabla carga más filas de ese lado.
	casos := map[string]bool{
		"hazlo global":                 false,
		"bajalo":                       false,
		"deploy central":               false,
		"mira en git":                  false,
		"sigue con el TLS del cerebro": false,
		"revisa el commit 3fd81c33":    false,
		"":                             false, // una imagen sola: trae otra cosa
		"go":                           true,
		"continua":                     true,
		"continúa":                     true,
		"si hazlo":                     true,
		"listo.":                       true,
		"Sigue":                        true,
		"dale, seguí":                  true,
		"ok 1":                         true,
	}
	for prompt, quiere := range casos {
		if got := EsPedidoDeContinuacion(prompt); got != quiere {
			t.Errorf("EsPedidoDeContinuacion(%q) = %v, quería %v", prompt, got, quiere)
		}
	}
}

// Sabotaje que la hace fallar: no abrir una ventana nueva al compactar.
// arnes: archivo="internal/transcripts/turnos.go"
// arnes: de="\t\tif t.Compactacion {\n\t\t\tt.Ventana++\n"
// arnes: a="\t\tif false && t.Compactacion {\n\t\t\tt.Ventana++\n"
//
// Sabotaje que la hace fallar: que el resumen de la compactación pase por un pedido humano.
// arnes: archivo="internal/transcripts/prompts.go"
// arnes: de="\tcase reg.IsCompactSummary,\n"
// arnes: a="\tcase false,\n"
func TestLeerSesionPartePorTurnosYVentanas(t *testing.T) {
	// La forma medida: el hook llega DESPUÉS de su prompt, la compactación deja un borde, el resumen
	// y el `/compact`, y después se reescriben registros con el mismo uuid.
	p1 := fxUsuario("u1", "2026-09-20T10:00:00.000Z", "armá el banco con prompts reales")
	h1 := fxHookDe("h1", "2026-09-20T10:00:01.000Z", "UserPromptSubmit", "[Musubi — memoria relevante]\n- x [id:a1]")
	llamada := map[string]any{"type": "assistant", "uuid": "a1", "timestamp": "2026-09-20T10:00:02.000Z",
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_1", "name": "mcp__musubi__musubi_memory_expand",
				"input": map[string]any{"ids": []string{"a1"}}},
			map[string]any{"type": "tool_use", "id": "toolu_2", "name": "Write",
				"input": map[string]any{"content": "un archivo entero"}}}}}
	// El resumen de una versión nueva que ya no empieza como los viejos: sólo lo marca el campo.
	resumen := fxUsuario("u2", "2026-09-20T11:00:01.000Z", "Summary: 1. Primary Request and Intent")
	resumen["isCompactSummary"] = true
	ruta := fxEscribir(t,
		p1, h1, llamada,
		p1, h1, // Claude Code reescribe lo que ya estaba
		fxUsuario("u1b", "2026-09-20T10:30:00.000Z", "<task-notification>\n<summary>listo</summary>"),
		fxBorde("b1", "2026-09-20T11:00:00.000Z"),
		resumen,
		fxUsuario("u3", "2026-09-20T11:00:02.000Z", "<command-name>/compact</command-name>"),
		fxHookDe("h2", "2026-09-20T11:00:03.000Z", "SessionStart", "[Musubi — memoria]\n- y [id:a2]"),
		fxUsuario("u4", "2026-09-20T11:05:00.000Z", []any{map[string]any{"type": "text", "text": "sigue"}}),
	)

	s, err := LeerSesion(ruta)
	if err != nil {
		t.Fatal(err)
	}
	type fila struct {
		Origen  Origen
		Ventana int
		Iny     int
	}
	var got []fila
	for _, tu := range s.Turnos {
		got = append(got, fila{tu.Origen, tu.Ventana, len(tu.Inyecciones)})
	}
	quiero := []fila{
		{OrigenHumano, 0, 1},   // el pedido, con su hook (una vez, aunque se reescribió)
		{OrigenSistema, 0, 0},  // el aviso de la tarea de fondo
		{OrigenApertura, 1, 0}, // la compactación abre la ventana 1
		{OrigenInterno, 1, 0},  // el resumen no es un pedido
		{OrigenSistema, 1, 1},  // el /compact, y el SessionStart que llega detrás
		{OrigenHumano, 1, 0},
	}
	if !reflect.DeepEqual(got, quiero) {
		t.Fatalf("turnos (origen, ventana, inyecciones) = %v\nquería %v", got, quiero)
	}
	if ll := s.Turnos[0].Llamadas; len(ll) != 2 || string(ll[0].Entrada) == "" || ll[1].Entrada != nil {
		t.Errorf("llamadas del primer turno = %+v: quería las 2, con la entrada sólo en la tool MCP", ll)
	}
	if ids := IDsDeMemoria(s.Turnos[4].Inyecciones[0].Texto); !reflect.DeepEqual(ids, []string{"a2"}) {
		t.Errorf("ids del arranque = %v, quería [a2]", ids)
	}
}

func fxEncolado(uuid, ts string, prompt any, meta bool) map[string]any {
	a := map[string]any{"type": "queued_command", "prompt": prompt, "commandMode": "prompt", "timestamp": ts}
	if meta {
		a["isMeta"] = true
	}
	return map[string]any{"type": "attachment", "uuid": uuid, "timestamp": ts, "attachment": a}
}

// Sabotaje que la hace fallar: no reconocer el prompt que llega con el agente trabajando.
// arnes: archivo="internal/transcripts/turnos.go"
// arnes: de="\t\t\tcase AdjuntoEncolado:\n"
// arnes: a="\t\t\tcase AdjuntoEncolado + \"-no\":\n"
func TestUnPromptEncoladoAbreSuTurno(t *testing.T) {
	// Lo que el dueño escribe mientras el agente trabaja, y el aviso de una tarea de fondo que
	// termina en el medio, llegan como adjunto `queued_command` y no como registro `user`. El hook
	// del turno corre para ellos: su memoria es de ELLOS, no del pedido que estaba en curso. Las dos
	// formas del `prompt` (string y lista de bloques) aparecen en los reales; uno marcado meta no es
	// un prompt.
	ruta := fxEscribir(t,
		fxUsuario("u1", "2026-09-20T10:00:00.000Z", "armá el banco con prompts reales"),
		fxHookDe("h1", "2026-09-20T10:00:01.000Z", "UserPromptSubmit", "[Musubi — memoria relevante]\n- x [id:a1]"),
		fxEncolado("q1", "2026-09-20T10:01:00.000Z", "<task-notification>\n<summary>listo</summary>", false),
		fxHookDe("h2", "2026-09-20T10:01:01.000Z", "UserPromptSubmit", "[Musubi — memoria relevante]\n- t [id:t1]"),
		fxEncolado("q2", "2026-09-20T10:02:00.000Z", []any{map[string]any{"type": "text", "text": "revisá también el vector"}}, false),
		fxHookDe("h3", "2026-09-20T10:02:01.000Z", "UserPromptSubmit", "[Musubi — memoria relevante]\n- v [id:v1]"),
		fxEncolado("q3", "2026-09-20T10:03:00.000Z", "interno", true),
	)
	s, err := LeerSesion(ruta)
	if err != nil {
		t.Fatal(err)
	}
	type fila struct {
		Origen   Origen
		Encolado bool
		IDs      string
	}
	var got []fila
	for _, tu := range s.Turnos {
		f := fila{Origen: tu.Origen, Encolado: tu.Encolado}
		for _, in := range tu.Inyecciones {
			for _, id := range IDsDeMemoria(in.Texto) {
				f.IDs += id + " "
			}
		}
		got = append(got, f)
	}
	quiero := []fila{
		{OrigenHumano, false, "a1 "},
		{OrigenSistema, true, "t1 "},
		{OrigenHumano, true, "v1 "},
	}
	if !reflect.DeepEqual(got, quiero) {
		t.Errorf("turnos (origen, encolado, ids) = %v\nquería %v: la memoria de un prompt encolado se le "+
			"anotó al turno en curso", got, quiero)
	}
}
