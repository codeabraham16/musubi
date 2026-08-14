package mcp

// Tests del eje DORMANT del registro (ver toolEntry.dormant).
//
// La tesis que hay que proteger es una sola y es contraintuitiva: dormir una tool le saca el lugar
// en el catálogo SIN sacarle la capacidad. Si algún refactor futuro convierte "dormida" en
// "borrada", D2 lo caza — y ese es el test que importa, porque es la diferencia entre archivar
// trabajo y perderlo.

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// dormidas son las que este commit durmió, con el motivo medido al lado. La lista está acá y no
// derivada del registro a propósito: si alguien duerme una tool nueva, tiene que venir a declararla
// y a decir por qué.
var dormidas = map[string]string{
	"musubi_save_fact":         "cero invocaciones; el grafo de hechos se llena por propose_facts",
	"musubi_log_error":         "cero invocaciones; telemetry_logs tiene 1 fila en el repo más usado",
	"musubi_resolve_telemetry": "por arrastre: sin log_error no hay nada que resolver",
	"musubi_debate":            "cero invocaciones; debates/debate_postures/debate_votes en cero",
	"musubi_promote":           "con team_mode:true save_observation ya escribe 'shared' de entrada",
	"musubi_workflow":          "cero invocaciones; los 68 workflow_runs son 'sdd-*', los crea musubi_sdd",

	// Segunda tanda (2026-08-14). Cruce de los DOS ledgers a 90 días —central y local— contra
	// tools/list: 13 tools en cero. Sólo estas TRES se durmieron, y el criterio que las separó de
	// las otras diez fue buscarle el CONSUMIDOR a cada una, no el conteo. Cinco de las trece las
	// llama musubi-body en código vivo (token_revoke, author_skill, search_skills,
	// log_skill_decision, code_graph_viz): dormirlas habría roto el panel de identidades, la Forja
	// y la vista del Grafo. Su cero mide falta de USO, no de cableado.
	"musubi_resolve_skills":  "cero en 90 días; el harness ya lee .claude/skills/*/SKILL.md nativo, que musubi setup escribe junto al .musubi/skills/*.yaml",
	"musubi_detect_stack":    "cero en 90 días; el hook de SessionStart ya corre `musubi detect --hook-mode` antes del primer turno",
	"musubi_discover_skills": "cero en 90 días; ya era opt-in (sourcing.marketplace_enabled) y la solapa Comunidad del cuerpo usa search_skills",
}

// toolsExpuestas es cuántas tools debe listar tools/list con la config por defecto: el registro
// menos las dormidas. Se DERIVA en vez de escribirse a mano para matar una clase de drift que ya
// existía — tres tests distintos tenían el número 59 hardcodeado, así que dormir una sola tool
// rompía los tres en lugares que no hablan del catálogo.
func toolsExpuestas() int {
	s := NewMcpServer(nil, "", nil)
	n := 0
	for i := range s.tools {
		if !s.tools[i].dormant {
			n++
		}
	}
	return n
}

// nombresExpuestos devuelve los nombres que salen por tools/list.
func nombresExpuestos(t *testing.T, s *McpServer) map[string]bool {
	t.Helper()
	res, ok := s.handleToolsList().(map[string]interface{})
	if !ok {
		t.Fatalf("tools/list no devolvió un mapa: %T", s.handleToolsList())
	}
	tools, ok := res["tools"].([]Tool)
	if !ok {
		t.Fatalf("tools/list no devolvió []Tool: %T", res["tools"])
	}
	out := make(map[string]bool, len(tools))
	for _, tl := range tools {
		out[tl.Name] = true
	}
	return out
}

// D1 — las dormidas NO aparecen en el catálogo. Es el efecto buscado.
func TestDormidasNoAparecenEnElCatalogo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	expuestas := nombresExpuestos(t, s)
	for nombre, motivo := range dormidas {
		if expuestas[nombre] {
			t.Errorf("%s sigue en tools/list y está declarada dormida (%s)", nombre, motivo)
		}
	}
}

// D2 — EL TEST QUE IMPORTA: una dormida sigue respondiendo por tools/call. Dormir no es borrar.
//
// Se llama con argumentos VACÍOS y se acepta cualquier error de parámetros: lo que se afirma no es
// que la llamada tenga éxito, sino que la tool EXISTE en el índice de despacho. La señal de
// fracaso es exactamente una — "tool desconocida" —, que es lo que devolvería el dispatcher si
// dormir la hubiera sacado del registro.
func TestDormidaSigueSiendoDespachable(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	for nombre := range dormidas {
		if _, ok := s.toolIndex[nombre]; !ok {
			t.Errorf("%s desapareció del índice de despacho: dormir le sacó la capacidad, no sólo el lugar", nombre)
			continue
		}
		_, rpcErr := call(t, s, nombre, map[string]interface{}{})
		if rpcErr != nil && rpcErr.Code == codeMethodNotFound {
			t.Errorf("%s responde 'tool desconocida' (%s): está dormida, no retirada", nombre, rpcErr.Message)
		}
	}
}

// D3 — MUSUBI_TOOLS_ALL devuelve el catálogo entero. Es la salida de emergencia que hace que
// dormir no vuelva a una tool indescubrible.
func TestToolsAllDevuelveElCatalogoEntero(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sinFlag := len(nombresExpuestos(t, s))

	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	conFlag := nombresExpuestos(t, s)

	if len(conFlag) != len(s.tools) {
		t.Errorf("con MUSUBI_TOOLS_ALL esperaba las %d del registro, obtuve %d", len(s.tools), len(conFlag))
	}
	for nombre := range dormidas {
		if !conFlag[nombre] {
			t.Errorf("%s sigue oculta con MUSUBI_TOOLS_ALL=1", nombre)
		}
	}
	if len(conFlag) <= sinFlag {
		t.Errorf("el flag no agregó nada: %d sin flag, %d con flag", sinFlag, len(conFlag))
	}
}

// D4 — el catálogo se achica DE VERDAD, medido en bytes y no en cantidad de tools.
//
// Contar tools no alcanza: la motivación es el peso del catálogo, y las tools no pesan igual
// (musubi_workflow sola son 3.582 caracteres, el 6,3 % del total). Este test ata el cambio a su
// razón de ser; si alguien duerme seis tools irrelevantes y el peso no baja, falla.
func TestDormirAchicaElCatalogoDeVerdad(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	pesar := func() int {
		b, err := json.Marshal(s.handleToolsList())
		if err != nil {
			t.Fatalf("marshal tools/list: %v", err)
		}
		return len(b)
	}

	conDormidas := pesar()
	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	completo := pesar()

	ahorro := completo - conDormidas
	// El umbral es deliberadamente flojo (el texto de las descripciones se edita seguido); lo que
	// congela es el ORDEN DE MAGNITUD del ahorro medido al dormirlas: 8.176 caracteres.
	const minimoEsperado = 6000
	if ahorro < minimoEsperado {
		t.Errorf("dormir las %d tools ahorró %d caracteres del catálogo, esperaba al menos %d (completo=%d, con dormidas=%d)",
			len(dormidas), ahorro, minimoEsperado, completo, conDormidas)
	}
	t.Logf("catálogo: %d caracteres completo, %d con las dormidas fuera (−%d, −%.1f %%)",
		completo, conDormidas, ahorro, 100*float64(ahorro)/float64(completo))
}
