package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// LAS PRUEBAS DE `musubi uso-agente` LEEN FIXTURES, NUNCA LOS TRANSCRIPTS REALES.
//
// Los de verdad cambian a cada minuto —esta misma sesión escribe uno— y traen conversaciones
// enteras: una prueba que los leyera sería lenta, inestable y un lugar por donde se filtra lo que
// una persona le dijo al agente. Cada prueba arma su carpeta en t.TempDir() con la forma que se
// midió en los reales (ver el encabezado de uso_agente.go), y los tipos de registro y de adjunto se
// escriben LITERALES: son un hecho del formato de Claude Code y se clavan, no se derivan de las
// constantes que la prueba custodia.

// fxTS arma un timestamp con la forma de los reales (UTC, milisegundos, `Z`).
func fxTS(dia string, seg int) string { return fmt.Sprintf("%sT10:00:%02d.000Z", dia, seg) }

func fxPrompt(uuid, ts, texto string) map[string]any {
	return map[string]any{"type": "user", "uuid": uuid, "timestamp": ts,
		"message": map[string]any{"role": "user", "content": texto}}
}

func fxHook(uuid, ts, evento string, textos ...string) map[string]any {
	return map[string]any{"type": "attachment", "uuid": uuid, "timestamp": ts,
		"attachment": map[string]any{"type": "hook_additional_context", "hookName": evento,
			"hookEvent": evento, "toolUseID": "hook-" + uuid, "content": textos}}
}

func fxAdjunto(uuid, ts string, adjunto map[string]any) map[string]any {
	return map[string]any{"type": "attachment", "uuid": uuid, "timestamp": ts, "attachment": adjunto}
}

func fxLlamada(uuid, ts, id, nombre string, input map[string]any) map[string]any {
	if input == nil {
		input = map[string]any{}
	}
	return map[string]any{"type": "assistant", "uuid": uuid, "timestamp": ts,
		"message": map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": id, "name": nombre, "input": input}}}}
}

func fxResultado(uuid, ts, toolUseID string, contenido any) map[string]any {
	return map[string]any{"type": "user", "uuid": uuid, "timestamp": ts,
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": toolUseID, "content": contenido}}}}
}

// fxResultadoToolSearch es la respuesta de un ToolSearch tal como la escribe Claude Code: un bloque
// `tool_reference` por tool cargada.
func fxResultadoToolSearch(uuid, ts, toolUseID string, nombres ...string) map[string]any {
	refs := make([]any, 0, len(nombres))
	for _, n := range nombres {
		refs = append(refs, map[string]any{"type": "tool_reference", "tool_name": n})
	}
	return fxResultado(uuid, ts, toolUseID, refs)
}

// escribirTranscript escribe un .jsonl con un registro por línea, en `rel` bajo `raiz`.
//
// LE PONE AL ARCHIVO EL MTIME DE SU ÚLTIMO REGISTRO, como pasa con un transcript real, y no la
// hora de la corrida: el comando saltea los archivos que no cambiaron desde antes de la ventana, y
// con el mtime del reloj la prueba dependería de en qué fecha se corre.
func escribirTranscript(t *testing.T, raiz, rel string, registros ...map[string]any) {
	t.Helper()
	ruta := filepath.Join(raiz, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	var ultimo time.Time
	for _, r := range registros {
		linea, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(linea)
		b.WriteByte('\n')
		if ts, ok := r["timestamp"].(string); ok {
			if cuando, err := time.Parse(time.RFC3339Nano, ts); err == nil && cuando.After(ultimo) {
				ultimo = cuando
			}
		}
	}
	if err := os.WriteFile(ruta, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ultimo.IsZero() {
		if err := os.Chtimes(ruta, ultimo, ultimo); err != nil {
			t.Fatal(err)
		}
	}
}

// medirFixture corre la medición sin ventana. Un alcance «sin medir» vuelve nil, y cada prueba
// decide si eso es un fallo.
func medirFixture(t *testing.T, raiz string) (principal, subagentes *MedicionUso, inf InformeUso) {
	t.Helper()
	inf, err := medirUsoAgente(raiz, ventanaUso{}, nil)
	if err != nil {
		t.Fatalf("medirUsoAgente: %v", err)
	}
	return inf.Principal.MedicionUso, inf.Subagentes.MedicionUso, inf
}

// Sabotaje que la hace fallar: sacar la deduplicación por uuid.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tif l.vistos[reg.UUID] {\n"
// arnes: a="\t\tif false && l.vistos[reg.UUID] {\n"
//
// Sabotaje que la hace fallar: sacar la deduplicación de cada tool_use por su id.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\t\tif l.llamadasVistas[b.ID] {\n"
// arnes: a="\t\t\tif false && l.llamadasVistas[b.ID] {\n"
func TestUsoAgenteNoCuentaDosVecesLoQueLaReanudacionReescribe(t *testing.T) {
	// Medido el 2026-09-25: al reanudar una sesión Claude Code vuelve a escribir registros que ya
	// estaban —50.285 uuid repetidos en el mismo archivo— y sin deduplicar las llamadas del hilo
	// principal daban 1.243 en vez de 791. Las dos formas están acá: el registro reescrito ENTERO
	// (mismo uuid) y la misma llamada bajo un uuid nuevo (mismo id de tool_use). Cada una la ataja
	// una sola de las dos deduplicaciones, así que sacar cualquiera pone la prueba en rojo.
	raiz := t.TempDir()
	dia := "2026-09-20"
	prompt := fxPrompt("u-p1", fxTS(dia, 1), "arrancamos")
	hook := fxHook("u-h1", fxTS(dia, 2), "UserPromptSubmit",
		"[Musubi — memoria relevante]\n- gist [id:7] expandí con musubi_memory_expand")
	llamada := fxLlamada("u-a1", fxTS(dia, 3), "toolu_1", "mcp__musubi__musubi_memory_expand", map[string]any{"ids": []int{7}})
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		prompt, hook, llamada,
		// la reanudación: los mismos registros, otra vez
		prompt, hook, llamada,
		// y la misma llamada con otro uuid
		fxLlamada("u-a1-bis", fxTS(dia, 4), "toolu_1", "mcp__musubi__musubi_memory_expand", nil),
	)

	p, _, _ := medirFixture(t, raiz)
	if p == nil {
		t.Fatal("el hilo principal quedó «sin medir» con un transcript en disco")
	}
	if p.Llamadas != 1 {
		t.Errorf("llamadas a Musubi = %d, quería 1: la misma llamada (mismo id de tool_use) se contó "+
			"más de una vez", p.Llamadas)
	}
	if got := p.Bloques["memoria relevante"]; got != 1 {
		t.Errorf("bloques «memoria relevante» = %d, quería 1: un adjunto de hook reescrito al reanudar "+
			"(mismo uuid) se contó dos veces", got)
	}
	if p.Transcripts != 1 {
		t.Errorf("transcripts = %d, quería 1", p.Transcripts)
	}
}

// Sabotaje que la hace fallar: no excluir el journal.jsonl de los workflows.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tif d.Name() == journalDeWorkflow {\n"
// arnes: a="\t\tif false && d.Name() == journalDeWorkflow {\n"
//
// Sabotaje que la hace fallar: no reconocer la carpeta de subagentes.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tif parte == \"subagents\" {\n"
// arnes: a="\t\tif parte == \"subagentes\" {\n"
func TestUsoAgenteSeparaPrincipalesDeSubagentesYSaltaElJournal(t *testing.T) {
	raiz := t.TempDir()
	dia := "2026-09-20"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("m-p1", fxTS(dia, 1), "buscá lo de ayer"),
		fxLlamada("m-a1", fxTS(dia, 2), "toolu_m1", "mcp__musubi__musubi_recall", map[string]any{"query": "ayer"}),
	)
	// Un subagente común y uno de workflow: los dos son subagentes.
	escribirTranscript(t, raiz, "-home-x/s1/subagents/agent-a1.jsonl",
		fxPrompt("s-p1", fxTS(dia, 3), "Sos un lector. Usá musubi_recall y después musubi_whoami."),
		fxLlamada("s-a1", fxTS(dia, 4), "toolu_s1", "mcp__musubi__musubi_recall", nil),
		fxLlamada("s-a2", fxTS(dia, 5), "toolu_s2", "mcp__musubi__musubi_whoami", nil),
	)
	escribirTranscript(t, raiz, "-home-x/s1/subagents/workflows/wf_1/agent-a2.jsonl",
		fxPrompt("w-p1", fxTS(dia, 6), "revisá la rama"),
		fxLlamada("w-a1", fxTS(dia, 7), "toolu_w1", "mcp__musubi__musubi_judge", nil),
	)
	// La bitácora del workflow: con la forma de un transcript, para que contarla se note.
	escribirTranscript(t, raiz, "-home-x/s1/subagents/workflows/wf_1/journal.jsonl",
		fxPrompt("j-p1", fxTS(dia, 8), "no soy un transcript"),
		fxLlamada("j-a1", fxTS(dia, 9), "toolu_j1", "mcp__musubi__musubi_judge", nil),
	)

	p, s, inf := medirFixture(t, raiz)
	if p == nil || s == nil {
		t.Fatalf("un alcance quedó sin medir: principal=%v subagentes=%v", inf.Principal.Estado, inf.Subagentes.Estado)
	}
	if inf.JournalExcluidos != 1 {
		t.Errorf("journal excluidos = %d, quería 1", inf.JournalExcluidos)
	}
	if p.Transcripts != 1 || p.Llamadas != 1 {
		t.Errorf("principal: %d transcript(s) y %d llamada(s), quería 1 y 1: un subagente se contó como "+
			"sesión principal", p.Transcripts, p.Llamadas)
	}
	if s.Transcripts != 2 || s.Llamadas != 3 {
		t.Errorf("subagentes: %d transcript(s) y %d llamada(s), quería 2 y 3: el journal del workflow "+
			"se contó como transcript, o un subagente se perdió", s.Transcripts, s.Llamadas)
	}
	quiero := AtribucionUso{Prompt: 2, Nadie: 1}
	if s.Atribucion != quiero {
		t.Errorf("atribución de los subagentes = %+v, quería %+v", s.Atribucion, quiero)
	}
}

// Sabotaje que la hace fallar: contar el texto dentro de un tool_result como inyección de hook.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tif !l.busquedas[b.ToolUseID] {\n"
// arnes: a="\t\tif txt, _ := decodificarContenido(b.Content); txt != \"\" {\n\t\t\tsumarBloques(l.m.Bloques, txt)\n\t\t}\n\t\tif !l.busquedas[b.ToolUseID] {\n"
func TestUsoAgenteElTextoDeUnToolResultNoEsUnaInyeccion(t *testing.T) {
	// El agente lee detect.go, un grep devuelve el encabezado, un informe lo cita: el texto
	// «[Musubi — …]» llega DENTRO de un tool_result y nadie se lo inyectó. Medido el 2026-09-25:
	// 629 encabezados así contra 2.846 de hooks de verdad. Van las dos formas de `content` —string y
	// lista de bloques de texto— porque las dos aparecen en los reales.
	raiz := t.TempDir()
	dia := "2026-09-20"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("u-p1", fxTS(dia, 1), "leé detect.go"),
		fxHook("u-h1", fxTS(dia, 2), "UserPromptSubmit", "[Musubi — memoria relevante]\n- nada nuevo"),
		fxLlamada("u-a1", fxTS(dia, 3), "toolu_r1", "Read", map[string]any{"file_path": "cmd/musubi/detect.go"}),
		fxResultado("u-r1", fxTS(dia, 4), "toolu_r1",
			"419\tb.WriteString(\"[Musubi — auto-descubrimiento de skills]\\n\")"),
		fxLlamada("u-a2", fxTS(dia, 5), "toolu_r2", "Grep", map[string]any{"pattern": "Musubi —"}),
		fxResultado("u-r2", fxTS(dia, 6), "toolu_r2",
			[]any{map[string]any{"type": "text", "text": "detect.go:243: [Musubi — salud]"}}),
	)

	p, _, _ := medirFixture(t, raiz)
	if p == nil {
		t.Fatal("el hilo principal quedó «sin medir» con un transcript en disco")
	}
	quiero := map[string]int{"memoria relevante": 1}
	if !reflect.DeepEqual(p.Bloques, quiero) {
		t.Errorf("bloques de hook = %v, quería %v: el texto de un tool_result se contó como inyección", p.Bloques, quiero)
	}
}

// Sabotaje que la hace fallar: no reconocer el servidor de Musubi instalado como plugin.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\tif strings.HasPrefix(s, \"plugin_\") || strings.HasPrefix(s, \"plugin:\") {\n"
// arnes: a="\tif false {\n"
//
// Sabotaje que la hace fallar: decidir por el nombre de la tool y no por el servidor que la sirve.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\tif !ok || tool == \"\" || !esServidorMusubi(servidor) {\n"
// arnes: a="\tif !ok || servidor == \"\" || !strings.HasPrefix(tool, \"musubi_\") {\n"
func TestUsoAgenteReconoceLasToolsDeMusubiComoPlugin(t *testing.T) {
	// El día que Musubi se instale como plugin sus tools se van a llamar
	// `mcp__plugin_<algo>_musubi__…` y sus instrucciones van a venir firmadas `plugin:<algo>:musubi`.
	// Un medidor clavado a `mcp__musubi__` diría «el agente dejó de usar Musubi» justo ese día.
	// Y al revés: lo que decide es el SERVIDOR, así que la tool de otro que por casualidad se llame
	// `musubi_recall` no es de Musubi.
	raiz := t.TempDir()
	dia := "2026-09-20"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxAdjunto("u-i1", fxTS(dia, 1), map[string]any{"type": "mcp_instructions_delta",
			"addedNames": []string{"claude.ai Claude Docs", "plugin:x:musubi"}, "addedBlocks": []string{"## x"}, "removedNames": []string{}}),
		fxAdjunto("u-d1", fxTS(dia, 2), map[string]any{"type": "deferred_tools_delta",
			"addedNames": []string{"mcp__plugin_x_musubi__musubi_recall"}, "removedNames": []string{}}),
		fxPrompt("u-p1", fxTS(dia, 3), "buscá"),
		fxLlamada("u-a1", fxTS(dia, 4), "toolu_1", "mcp__plugin_x_musubi__musubi_recall", nil),
		fxLlamada("u-a2", fxTS(dia, 5), "toolu_2", "mcp__musubi-cerebro__musubi_whoami", nil),
		fxLlamada("u-a3", fxTS(dia, 6), "toolu_3", "mcp__claude_ai_Claude_Docs__batch", nil),
		fxLlamada("u-a4", fxTS(dia, 7), "toolu_4", "mcp__plugin_x_otro__musubi_recall", nil),
	)
	// Un segundo transcript que no ve nada de Musubi: el «a la vista» tiene que distinguirlos.
	escribirTranscript(t, raiz, "-home-x/s2.jsonl",
		fxAdjunto("v-i1", fxTS(dia, 1), map[string]any{"type": "mcp_instructions_delta",
			"addedNames": []string{"claude.ai Claude Docs"}, "addedBlocks": []string{"## x"}, "removedNames": []string{}}),
		fxPrompt("v-p1", fxTS(dia, 2), "hola"),
	)

	p, _, _ := medirFixture(t, raiz)
	if p == nil {
		t.Fatal("el hilo principal quedó «sin medir» con dos transcripts en disco")
	}
	quiero := map[string]int{"musubi_recall": 1, "musubi_whoami": 1}
	if p.Llamadas != 2 || !reflect.DeepEqual(p.PorTool, quiero) {
		t.Errorf("llamadas a Musubi = %d %v, quería 2 %v (la del plugin y la del canal al cerebro; ni la "+
			"de Claude Docs ni la del servidor `otro`)", p.Llamadas, p.PorTool, quiero)
	}
	if p.ConInstrucciones != 1 {
		t.Errorf("transcripts con las instrucciones de Musubi = %d, quería 1: `plugin:x:musubi` es Musubi", p.ConInstrucciones)
	}
	if p.Transcripts != 2 || p.ConToolsALaVista != 1 {
		t.Errorf("transcripts = %d con tools a la vista = %d, quería 2 y 1", p.Transcripts, p.ConToolsALaVista)
	}
}

// Sabotaje que la hace fallar: no vaciar lo que nombró el hook al empezar un turno nuevo.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tl.nombradasTurno = map[string]bool{}\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: no registrar lo que cargó un ToolSearch.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tl.cargadas[n] = true\n"
// arnes: a="\t\tl.cargadas[n] = false\n"
func TestUsoAgenteAtribuyeCadaLlamadaASuOrigen(t *testing.T) {
	// Un transcript con las cuatro atribuciones y las dos formas de llegar a una tool:
	//
	//	turno 0  SessionStart nombra musubi_memory_expand
	//	turno 1  el hook nombra conflicts y judge · ToolSearch carga las dos
	//	         conflicts  → hook del mismo turno, cargada
	//	         memory_expand → hook de ANTES (el de arranque), sin ToolSearch
	//	turno 2  el prompt nombra musubi_recall
	//	         recall  → el prompt, sin ToolSearch
	//	         whoami  → nadie, sin ToolSearch
	//	         judge   → hook de ANTES (turno 1), cargada en el turno 1
	raiz := t.TempDir()
	dia := "2026-09-20"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxHook("u-h0", fxTS(dia, 1), "SessionStart", "[Musubi — memoria]\n- gist [id:3] — expandí con musubi_memory_expand"),
		fxPrompt("u-p1", fxTS(dia, 2), "sigamos"),
		fxHook("u-h1", fxTS(dia, 3), "UserPromptSubmit",
			"[Musubi — conflictos] Hay 2 relación(es). Revisalas con musubi_conflicts y resolvé cada una con musubi_judge."),
		fxLlamada("u-a1", fxTS(dia, 4), "toolu_ts", "ToolSearch",
			map[string]any{"query": "select:mcp__musubi__musubi_conflicts,mcp__musubi__musubi_judge"}),
		fxResultadoToolSearch("u-r1", fxTS(dia, 5), "toolu_ts", "mcp__musubi__musubi_conflicts", "mcp__musubi__musubi_judge"),
		fxLlamada("u-a2", fxTS(dia, 6), "toolu_c", "mcp__musubi__musubi_conflicts", nil),
		fxLlamada("u-a3", fxTS(dia, 7), "toolu_m", "mcp__musubi__musubi_memory_expand", nil),
		fxPrompt("u-p2", fxTS(dia, 8), "ahora usá musubi_recall con «deploy»"),
		fxLlamada("u-a4", fxTS(dia, 9), "toolu_r", "mcp__musubi__musubi_recall", nil),
		fxLlamada("u-a5", fxTS(dia, 10), "toolu_w", "mcp__musubi__musubi_whoami", nil),
		fxLlamada("u-a6", fxTS(dia, 11), "toolu_j", "mcp__musubi__musubi_judge", nil),
	)

	p, _, _ := medirFixture(t, raiz)
	if p == nil {
		t.Fatal("el hilo principal quedó «sin medir» con un transcript en disco")
	}
	if p.Llamadas != 5 {
		t.Fatalf("llamadas a Musubi = %d, quería 5", p.Llamadas)
	}
	quiero := AtribucionUso{HookMismoTurno: 1, Prompt: 1, HookAntes: 2, Nadie: 1}
	if p.Atribucion != quiero {
		t.Errorf("atribución = %+v, quería %+v: lo que un hook nombró en un turno anterior no es «del mismo turno»",
			p.Atribucion, quiero)
	}
	if p.SinToolSearchPrevio != 3 {
		t.Errorf("llamadas sin ToolSearch previo = %d, quería 3 (memory_expand, recall, whoami): conflicts y "+
			"judge las cargó un ToolSearch", p.SinToolSearchPrevio)
	}
}

// Sabotaje que la hace fallar: comparar el nombre de la skill con su prefijo de origen puesto.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\treturn s[strings.LastIndex(s, \":\")+1:]\n"
// arnes: a="\treturn s\n"
func TestUsoAgenteCuentaLasSkillsDeMusubiPorNombreDerivado(t *testing.T) {
	// Qué skills son de Musubi lo dice cognitive.go, no una lista de esta prueba: se pregunta por
	// dos que salen de ahí, una con el prefijo que llevaría como plugin (`musubi:`).
	cognitivas := skillsDeMusubi()
	if len(cognitivas) == 0 {
		t.Fatal("cognitiveSkills no devolvió ninguna skill: no hay contra qué medir")
	}
	primera, ultima := cognitivas[0], cognitivas[len(cognitivas)-1]
	raiz := t.TempDir()
	dia := "2026-09-20"
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxAdjunto("u-l1", fxTS(dia, 1), map[string]any{"type": "skill_listing", "isInitial": true, "skillCount": 3,
			"names": []string{"artifact-design", ultima, "anthropic-skills:pdf"}, "content": "- " + ultima + ": …"}),
		fxPrompt("u-p1", fxTS(dia, 2), "revisá esto"),
		fxLlamada("u-a1", fxTS(dia, 3), "toolu_1", "Skill", map[string]any{"skill": primera}),
		fxLlamada("u-a2", fxTS(dia, 4), "toolu_2", "Skill", map[string]any{"skill": "musubi:" + ultima, "args": "x"}),
		fxLlamada("u-a3", fxTS(dia, 5), "toolu_3", "Skill", map[string]any{"skill": "artifact-design"}),
		fxLlamada("u-a4", fxTS(dia, 6), "toolu_4", "Skill", map[string]any{"skill": "anthropic-skills:pdf"}),
	)
	escribirTranscript(t, raiz, "-home-x/s1/subagents/agent-a1.jsonl",
		fxAdjunto("s-l1", fxTS(dia, 7), map[string]any{"type": "skill_listing", "isInitial": true, "skillCount": 1,
			"names": []string{"artifact-design"}, "content": "- artifact-design: …"}),
		fxPrompt("s-p1", fxTS(dia, 8), "hacé la página"),
	)

	p, s, _ := medirFixture(t, raiz)
	if p == nil || s == nil {
		t.Fatal("un alcance quedó «sin medir» con transcripts en disco")
	}
	if p.InvocacionesSkill != 4 || len(p.PorSkill) != 4 {
		t.Errorf("invocaciones de Skill = %d en %d nombre(s), quería 4 en 4: %v", p.InvocacionesSkill, len(p.PorSkill), p.PorSkill)
	}
	if p.SkillsDeMusubi != 2 {
		t.Errorf("invocaciones de skills de Musubi = %d, quería 2 (%s y musubi:%s): %v",
			p.SkillsDeMusubi, primera, ultima, p.PorSkill)
	}
	if p.ConSkillsEnListado != 1 || s.ConSkillsEnListado != 0 {
		t.Errorf("transcripts con skills de Musubi en el listado: principal %d, subagentes %d; quería 1 y 0",
			p.ConSkillsEnListado, s.ConSkillsEnListado)
	}
}

// Sabotaje que la hace fallar: decir 0 en vez de «sin medir».
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\tif m.Transcripts == 0 {\n"
// arnes: a="\tif false {\n"
//
// Sabotaje que la hace fallar: contar los registros de antes de la ventana.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\tif v.hayDesde && t.Before(v.desde) {\n"
// arnes: a="\tif false && v.hayDesde && t.Before(v.desde) {\n"
func TestUsoAgenteVentanaSinTranscriptsDiceSinMedir(t *testing.T) {
	// «0 llamadas» y «no hay un solo transcript en la ventana» son respuestas opuestas: la primera
	// dice que el agente no usó Musubi, la segunda que nadie miró. Tienen que salir distintas en la
	// tabla y en el JSON, y en el JSON un alcance sin medir no puede traer NI UNA clave numérica —el
	// que consume el JSON lee el número, no el estado—.
	raiz := t.TempDir()
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		fxPrompt("u-p1", fxTS("2026-09-10", 1), "viejo"),
		fxLlamada("u-a1", fxTS("2026-09-10", 2), "toolu_viejo", "mcp__musubi__musubi_recall", nil),
		fxPrompt("u-p2", fxTS("2026-09-20", 1), "nuevo"),
		fxLlamada("u-a2", fxTS("2026-09-20", 2), "toolu_nuevo", "mcp__musubi__musubi_recall", nil),
	)
	escribirTranscript(t, raiz, "-home-x/s1/subagents/agent-a1.jsonl",
		fxPrompt("s-p1", fxTS("2026-09-10", 3), "viejo"),
		fxLlamada("s-a1", fxTS("2026-09-10", 4), "toolu_s", "mcp__musubi__musubi_recall", nil),
	)

	// 1) La ventana deja afuera al subagente y corta al principal.
	var tabla, errOut bytes.Buffer
	if code := usoAgente([]string{"--dir", raiz, "--desde", "2026-09-18"}, &tabla, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	secciones := strings.SplitN(tabla.String(), "── SUBAGENTES", 2)
	if len(secciones) != 2 {
		t.Fatalf("la tabla no tiene la sección de subagentes:\n%s", tabla.String())
	}
	if !strings.Contains(secciones[1], "sin medir") || strings.Contains(secciones[1], "transcripts medidos") {
		t.Errorf("los subagentes no tienen transcripts en la ventana y la tabla no dice «sin medir», o "+
			"imprime números igual:\n%s", secciones[1])
	}
	if strings.Contains(secciones[0], "sin medir") || !strings.Contains(secciones[0], "transcripts medidos") {
		t.Errorf("el hilo principal tiene un registro en la ventana y la tabla no lo mide:\n%s", secciones[0])
	}

	var js bytes.Buffer
	if code := usoAgente([]string{"--dir", raiz, "--desde", "2026-09-18", "--json"}, &js, &errOut); code != 0 {
		t.Fatalf("exit %d con --json, stderr: %s", code, errOut.String())
	}
	// Se lee como lo leería otro programa —un mapa genérico— y no con los tipos de este paquete: la
	// pregunta es qué CLAVES trae el JSON, y un struct propio las rellenaría con ceros.
	var todo map[string]json.RawMessage
	if err := json.Unmarshal(js.Bytes(), &todo); err != nil {
		t.Fatalf("el --json no es JSON: %v\n%s", err, js.String())
	}
	crudo := map[string]map[string]any{}
	for _, k := range []string{"principal", "subagentes"} {
		var m map[string]any
		if err := json.Unmarshal(todo[k], &m); err != nil {
			t.Fatalf("%s no es un objeto en el --json: %v", k, err)
		}
		crudo[k] = m
	}
	if crudo["subagentes"]["estado"] != "sin medir" {
		t.Errorf("subagentes.estado = %v, quería «sin medir»", crudo["subagentes"]["estado"])
	}
	for _, clave := range []string{"transcripts", "llamadas_a_tools_de_musubi", "invocaciones_de_skill"} {
		if v, esta := crudo["subagentes"][clave]; esta {
			t.Errorf("subagentes sin medir trae %q = %v en el JSON: un cero que se lee como «medí y no hubo»", clave, v)
		}
	}
	if crudo["principal"]["estado"] != "medido" || crudo["principal"]["llamadas_a_tools_de_musubi"] != float64(1) {
		t.Errorf("principal = %v, quería medido con 1 llamada (la del 20; la del 10 queda fuera de la ventana)", crudo["principal"])
	}

	// 2) Una ventana donde no cae nada: los dos alcances, sin medir.
	tabla.Reset()
	if code := usoAgente([]string{"--dir", raiz, "--desde", "2026-09-21", "--hasta", "2026-09-22"}, &tabla, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	if n := strings.Count(tabla.String(), "sin medir"); n != 2 || strings.Contains(tabla.String(), "transcripts medidos") {
		t.Errorf("con la ventana vacía la tabla dice «sin medir» %d vez/veces (quería 2, una por alcance) "+
			"o imprime números:\n%s", n, tabla.String())
	}
}

// EL NOMBRE DE UNA CARPETA DE PROYECTO SE FIJA CON LO MEDIDO, NO CON LA FUNCIÓN. Los tres casos son
// carpetas reales de esta máquina (2026-09-25) junto a la ruta de trabajo que las generó: la regla
// es un hecho del formato de Claude Code, y derivar el esperado de carpetaDeProyecto dejaría a la
// prueba midiendo a la función contra sí misma.
//
// Sabotaje que la hace fallar: dejar pasar los caracteres que no son letras ni dígitos.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tb.WriteByte('-')\n"
// arnes: a="\t\tb.WriteRune(r)\n"
func TestUsoAgenteNombraLaCarpetaComoClaudeCode(t *testing.T) {
	medidas := map[string]string{
		"/home/davantis/.cache/musubi-uso-agente/e2e":               "-home-davantis--cache-musubi-uso-agente-e2e",
		"/home/davantis/musubi/.claude/worktrees/wf_31193ce6-417-5": "-home-davantis-musubi--claude-worktrees-wf-31193ce6-417-5",
		"/tmp/claude-1000/musubi-e2e-DH14gM":                        "-tmp-claude-1000-musubi-e2e-DH14gM",
	}
	for ruta, quiere := range medidas {
		if got := carpetaDeProyecto(ruta); got != quiere {
			t.Errorf("carpetaDeProyecto(%q) = %q, Claude Code la llamó %q", ruta, got, quiere)
		}
	}
}

// LAS CARPETAS DE EXPERIMENTO NO SE MIDEN, Y LO QUE QUEDÓ AFUERA SE DICE.
//
// Una prueba de conducta con `claude -p` en un mktemp arranca vacía a propósito, y medida junto a
// las sesiones reales las contamina: el 2026-09-25 las skills invocadas y las llamadas sin
// ToolSearch que el medidor contaba venían todas de carpetas de experimento. Las de la temporal del
// sistema se excluyen solas; las de otro lado, con `--excluir`; y el informe dice cuántas cayeron y
// con qué patrones, porque un número sin eso no se puede comparar con otro.
//
// LA TEMPORAL SE MUEVE A UNA RUTA LITERAL y su nombre de carpeta se escribe literal: ver la prueba
// de arriba. `-tmpxyz` comparte el prefijo sin colgar de la temporal, y tiene que medirse.
//
// Sabotaje que la hace fallar: no excluir ninguna carpeta.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\t\tif filepath.Dir(ruta) == filepath.Clean(dir) && excluida(d.Name(), exclusiones) {\n"
// arnes: a="\t\t\tif false {\n"
//
// Sabotaje que la hace fallar: excluir la temporal pero no lo que cuelga de ella.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\tfor _, patron := range []string{base, base + \"-*\"} {\n"
// arnes: a="\t\tfor _, patron := range []string{base} {\n"
//
// Sabotaje que la hace fallar: no poner las exclusiones por defecto.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\texclusiones = append(exclusionesPorDefecto(), exclusiones...)\n"
// arnes: a="\t\texclusiones = append([]string{}, exclusiones...)\n"
//
// Sabotaje que la hace fallar: que --incluir-temporales no las incluya.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\tif !*conTemporales {\n"
// arnes: a="\tif !*conTemporales || true {\n"
//
// Sabotaje que la hace fallar: que la carpeta pedida pueda excluirse a sí misma.
// arnes: archivo="cmd/musubi/uso_agente.go"
// arnes: de="\t\t\tif ruta == dir {\n"
// arnes: a="\t\t\tif false {\n"
func TestUsoAgenteExcluyeLasCarpetasDeExperimento(t *testing.T) {
	raiz := t.TempDir() // ANTES de mover la temporal: t.TempDir la lee
	tmp, carpetaTmp := "/tmpx", "-tmpx"
	if runtime.GOOS == "windows" {
		tmp, carpetaTmp = `C:\tmpx`, "C--tmpx"
		t.Setenv("TMP", tmp)
		t.Setenv("TEMP", tmp)
	} else {
		t.Setenv("TMPDIR", tmp)
	}
	sesion := func(rel, id string) {
		escribirTranscript(t, raiz, rel,
			fxPrompt("p-"+id, fxTS("2026-09-20", 1), "tarea"),
			fxLlamada("a-"+id, fxTS("2026-09-20", 2), "toolu_"+id, "mcp__musubi__musubi_recall", nil),
		)
	}
	sesion("-home-x/s1.jsonl", "real")
	sesion(carpetaTmp+"-musubi-e2e-AB12/s2.jsonl", "mktemp")   // experimento en la temporal
	sesion(carpetaTmp+"/s3.jsonl", "entemporal")               // sesión abierta EN la temporal
	sesion("-home-x--cache-e2e-real/s4.jsonl", "cache")        // experimento fuera de la temporal
	sesion("-tmpxyz/s5.jsonl", "vecina")                       // no cuelga de la temporal
	sesion("-home-x/s1/subagents/agent-a1.jsonl", "subagente") // lo de adentro de un proyecto medido

	medir := func(args ...string) (principal float64, excluidas float64, patrones []any) {
		t.Helper()
		var js, errOut bytes.Buffer
		if code := usoAgente(append([]string{"--json"}, args...), &js, &errOut); code != 0 {
			t.Fatalf("%v: exit %d, stderr: %s", args, code, errOut.String())
		}
		var inf map[string]any
		if err := json.Unmarshal(js.Bytes(), &inf); err != nil {
			t.Fatalf("el --json no es JSON: %v", err)
		}
		p, _ := inf["principal"].(map[string]any)
		n, _ := p["llamadas_a_tools_de_musubi"].(float64)
		e, _ := inf["carpetas_excluidas"].(float64)
		pat, _ := inf["exclusiones"].([]any)
		return n, e, pat
	}

	// 1) Por defecto: afuera las dos de la temporal; adentro el resto, incluida la vecina. Los
	// patrones se preguntan ANTES que los números: que no haya patrones y que haya patrones que no
	// excluyen son dos defectos distintos, y así cada uno falla con su propio motivo.
	n, e, pat := medir("--dir", raiz)
	if !reflect.DeepEqual(pat, []any{carpetaTmp, carpetaTmp + "-*"}) {
		t.Errorf("el informe dice que excluyó por %v; quería %q y %q", pat, carpetaTmp, carpetaTmp+"-*")
	}
	if n != 3 || e != 2 {
		t.Errorf("por defecto: %v llamadas y %v carpetas excluidas; quería 3 (real, cache, vecina) y 2 (las de la temporal)", n, e)
	}

	// 2) --excluir suma a las de la temporal.
	if n, e, _ := medir("--dir", raiz, "--excluir", "-home-x--cache-*"); n != 2 || e != 3 {
		t.Errorf("con --excluir: %v llamadas y %v excluidas; quería 2 y 3", n, e)
	}

	// 3) --incluir-temporales las mide, y el informe no inventa exclusiones.
	if n, e, pat := medir("--dir", raiz, "--incluir-temporales"); n != 5 || e != 0 || len(pat) != 0 {
		t.Errorf("con --incluir-temporales: %v llamadas, %v excluidas, patrones %v; quería 5, 0 y ninguno", n, e, pat)
	}

	// 4) Las dos juntas: las temporales entran y el patrón propio sigue valiendo.
	if n, e, _ := medir("--dir", raiz, "--incluir-temporales", "--excluir", "-home-x--cache-*"); n != 4 || e != 1 {
		t.Errorf("con las dos: %v llamadas y %v excluidas; quería 4 y 1", n, e)
	}

	// 5) La carpeta pedida no se excluye a sí misma, aunque el patrón la alcance.
	t.Chdir(filepath.Join(raiz, "-home-x"))
	if n, e, _ := medir("--dir", ".", "--excluir", "*"); n != 1 || e != 1 {
		t.Errorf("con --dir . y --excluir '*': %v llamadas y %v excluidas; quería 1 (s1) y 1 (la carpeta de s1, "+
			"con el subagente adentro)", n, e)
	}

	// 6) Un patrón roto se rechaza al leer los argumentos, no se ignora.
	var tabla, errOut bytes.Buffer
	if code := usoAgente([]string{"--dir", raiz, "--excluir", "["}, &tabla, &errOut); code != 2 {
		t.Errorf("un patrón inválido salió con %d; quería 2 (argumentos que no sirven)", code)
	}

	// 7) La tabla lo dice.
	tabla.Reset()
	if code := usoAgente([]string{"--dir", raiz}, &tabla, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(tabla.String(), "excluidas: 2 carpeta(s) de proyecto por "+carpetaTmp+" "+carpetaTmp+"-*") {
		t.Errorf("la tabla no dice qué excluyó:\n%s", tabla.String())
	}
}
