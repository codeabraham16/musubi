package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// proponer mete una propuesta en cuarentena y devuelve su id.
func proponer(t *testing.T, e *memory.DbEngine, tema, contenido string) string {
	t.Helper()
	id, err := e.ProposeObservation("", "", tema, contenido, "modelo-de-prueba", 0.7, "", nil)
	if err != nil {
		t.Fatalf("proponer: %v", err)
	}
	return id
}

// loteDeCuarentena lee el lote del sistema.
func loteDeCuarentena(t *testing.T, e *memory.DbEngine) memory.WorkBatch {
	t.Helper()
	b, err := e.WorkBatchStatus(memory.LoteCuarentena)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// LAS PROPUESTAS SE POSTEAN POR TEMA, UNA SOLA VEZ, Y SIN PASAR EL TOPE DE UNIDADES ABIERTAS.
//
// Por tema porque la serie de estados de un trabajo se juzga junta: la vieja se descarta por la
// nueva, y partida en dos unidades eso no se puede decidir. Una sola vez porque el hook corre en cada
// ronda y el tablero no es una cola de repeticiones. Con tope porque sin él cada ronda postearía más
// de lo que una corrida del subagente alcanza a hacer.
//
// Sabotaje que la hace fallar: no leer qué propuestas ya cubre una unidad.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\t\t\tcubiertas[id] = true\n"
// arnes: a="\t\t\t_ = id\n"
//
// Sabotaje que la hace fallar: postear sin mirar el tope.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\tif hueco := topeDeUnidadesAbiertas - r.reclamables; hueco > 0 {\n"
// arnes: a="\tif hueco := 100; hueco > 0 {\n"
//
// Sabotaje que la hace fallar: juntar temas sin mirar cuántas propuestas lleva la unidad.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\t\tif cantidad > 0 && cantidad+len(t.ids) > propuestasPorUnidad {\n"
// arnes: a="\t\tif false {\n"
func TestLasTareasSePosteanPorTemaUnaVezYConTope(t *testing.T) {
	e := memtest.NuevoEngine(t, t.TempDir())
	ahora := time.Now()
	a1 := proponer(t, e, "t/a", "estado 1 de a")
	b1 := proponer(t, e, "t/b", "estado 1 de b")
	a2 := proponer(t, e, "t/a", "estado 2 de a")
	b2 := proponer(t, e, "t/b", "estado 2 de b")
	a3 := proponer(t, e, "t/a", "estado 3 de a")
	b3 := proponer(t, e, "t/b", "estado 3 de b")
	c1 := proponer(t, e, "t/c", "una sola de c")

	r, err := correrRondaDeTareas(e, ahora)
	if err != nil {
		t.Fatal(err)
	}
	lote := loteDeCuarentena(t, e)
	if r.postadas != 2 || lote.Total != 2 {
		t.Fatalf("la primera ronda posteó %d unidad(es) (lote: %d); quería 2: el tema a entero, y b con c", r.postadas, lote.Total)
	}
	if got := idsDeLaUnidad(lote.Units[0].Spec); !igualesIDs(got, []string{a1, a2, a3}) {
		t.Errorf("la primera unidad cubre %v; quería el tema a entero, de la más vieja a la más nueva", got)
	}
	if got := idsDeLaUnidad(lote.Units[1].Spec); !igualesIDs(got, []string{b1, b2, b3, c1}) {
		t.Errorf("la segunda unidad cubre %v; quería el tema b entero y después c", got)
	}
	if r.propuestas != 7 || r.reclamables != 2 {
		t.Errorf("la ronda dice %d propuestas y %d reclamables; quería 7 y 2", r.propuestas, r.reclamables)
	}

	// Otra ronda sin nada nuevo no postea nada.
	if r, _ := correrRondaDeTareas(e, ahora); r.postadas != 0 || loteDeCuarentena(t, e).Total != 2 {
		t.Errorf("una ronda sin propuestas nuevas posteó %d unidad(es)", r.postadas)
	}

	// Una propuesta nueva de un tema ya cubierto va en una unidad nueva, sola.
	a4 := proponer(t, e, "t/a", "estado 4 de a")
	if r, _ := correrRondaDeTareas(e, ahora); r.postadas != 1 {
		t.Fatalf("la propuesta nueva generó %d unidad(es); quería 1", r.postadas)
	}
	lote = loteDeCuarentena(t, e)
	if got := idsDeLaUnidad(lote.Units[2].Spec); !igualesIDs(got, []string{a4}) {
		t.Errorf("la unidad nueva cubre %v; quería sólo la propuesta nueva", got)
	}

	// Con el tablero cerca del tope, se completa hasta el tope y el resto espera.
	for i := 0; i < 6; i++ {
		proponer(t, e, "t/otro-"+string(rune('a'+i)), strings.Repeat("x", 10))
	}
	proponer(t, e, "t/mas", "otra más")
	r, _ = correrRondaDeTareas(e, ahora)
	if lote := loteDeCuarentena(t, e); lote.Open != topeDeUnidadesAbiertas {
		t.Errorf("con 3 abiertas y propuestas para más, el tablero quedó con %d abiertas; quería el tope (%d)", lote.Open, topeDeUnidadesAbiertas)
	}
	if r.reclamables != topeDeUnidadesAbiertas {
		t.Errorf("la ronda dice %d reclamables; quería %d", r.reclamables, topeDeUnidadesAbiertas)
	}
}

func igualesIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// UNA UNIDAD RECLAMADA NO CUENTA COMO PENDIENTE, SALVO QUE A SU DUEÑO SE LE HAYA VENCIDO EL LEASE.
//
// Es la misma regla con que ClaimWorkUnit decide qué se puede reclamar. Contar la reclamada como
// pendiente avisaría de trabajo que otro subagente ya está haciendo; no contar la del lease vencido
// dejaría colgado lo que un subagente muerto soltó.
//
// Sabotaje que la hace fallar: no contar la reclamada con el lease vencido.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\t\t\tif u.LeaseExpiresAt == \"\" || (err == nil && vence.Before(ahora.UTC())) {\n"
// arnes: a="\t\t\tif false && (u.LeaseExpiresAt == \"\" || (err == nil && vence.Before(ahora.UTC()))) {\n"
// arnes: colision_ok="TestUnaUnidadReclamadaCuentaSoloConElLeaseVencido"
//
// Sabotaje que la hace fallar: contar como pendiente la que alguien está haciendo.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\t\t\tif u.LeaseExpiresAt == \"\" || (err == nil && vence.Before(ahora.UTC())) {\n"
// arnes: a="\t\t\tif true || (u.LeaseExpiresAt == \"\" || (err == nil && vence.Before(ahora.UTC()))) {\n"
// arnes: colision_ok="TestUnaUnidadReclamadaCuentaSoloConElLeaseVencido"
func TestUnaUnidadReclamadaCuentaSoloConElLeaseVencido(t *testing.T) {
	e := memtest.NuevoEngine(t, t.TempDir())
	proponer(t, e, "t/a", "una")
	proponer(t, e, "t/b", strings.Repeat("y", 10))
	for i := 0; i < 5; i++ {
		proponer(t, e, "t/a", "más de a")
	}
	ahora := time.Now()
	if r, _ := correrRondaDeTareas(e, ahora); r.reclamables != 2 {
		t.Fatalf("control: %d reclamables; quería 2", r.reclamables)
	}
	if _, ok, err := e.ClaimWorkUnit(memory.LoteCuarentena, agenteEnElTablero, 300, 5); !ok || err != nil {
		t.Fatalf("reclamar: ok=%v err=%v", ok, err)
	}
	if n := unidadesReclamables(loteDeCuarentena(t, e), ahora); n != 1 {
		t.Errorf("con una reclamada y su lease vigente, %d reclamables; quería 1", n)
	}
	if n := unidadesReclamables(loteDeCuarentena(t, e), ahora.Add(10*time.Minute)); n != 2 {
		t.Errorf("con el lease vencido, %d reclamables; quería 2 (la soltó quien la tenía)", n)
	}
}

// storeConTareas es el motor de prueba con dos propuestas en cuarentena.
func storeConTareas(t *testing.T) *memory.DbEngine {
	t.Helper()
	e := memtest.NuevoEngine(t, t.TempDir())
	proponer(t, e, "t/a", "estado viejo")
	proponer(t, e, "t/a", "estado nuevo")
	return e
}

// EL AVISO SALE SÓLO DONDE EL SUBAGENTE PUEDE TRABAJAR SOLO, Y NO MÁS DE UNA VEZ POR INTERVALO.
//
// Cada condición tiene su costo si falta: en modo normal un subagente de fondo le pediría a la
// persona confirmar cada veredicto; dentro de un subagente el aviso llegaría a quien no delega; sin
// el subagente instalado el agente iría a un callejón; y sin el intervalo cada turno de cada sesión
// lanzaría otro subagente. Y donde no hay aviso, tampoco se postea: un hook que no puede lanzar a
// nadie no tiene por qué llenar el tablero.
//
// Sabotaje que la hace fallar: avisar en cualquier modo de permisos.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\tif in.AgentID != \"\" || !modosQueCorrenSolos[in.PermissionMode] {\n"
// arnes: a="\tif in.AgentID != \"\" {\n"
// arnes: colision_ok="TestElAvisoDeTareasSaleSoloDondeElSubagentePuedeTrabajarSolo"
//
// Sabotaje que la hace fallar: avisar también dentro de un subagente.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\tif in.AgentID != \"\" || !modosQueCorrenSolos[in.PermissionMode] {\n"
// arnes: a="\tif !modosQueCorrenSolos[in.PermissionMode] {\n"
// arnes: colision_ok="TestElAvisoDeTareasSaleSoloDondeElSubagentePuedeTrabajarSolo"
//
// Sabotaje que la hace fallar: avisar en cada turno de la sesión.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\t\tif ultimo, err := time.Parse(time.RFC3339, v); err == nil && ahora.Sub(ultimo) < intervaloDeTareas {\n"
// arnes: a="\t\tif ultimo, err := time.Parse(time.RFC3339, v); err == nil && false && ahora.Sub(ultimo) < intervaloDeTareas {\n"
//
// Sabotaje que la hace fallar: avisar aunque no haya nada que reclamar.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\tif r.reclamables == 0 {\n"
// arnes: a="\tif false {\n"
func TestElAvisoDeTareasSaleSoloDondeElSubagentePuedeTrabajarSolo(t *testing.T) {
	const sub = "musubi:musubi-tareas"
	ahora := time.Now()
	aviso := func(e *memory.DbEngine, subagente, modo, agente string, cuando time.Time) string {
		return buildTurnTareas(&tareasDelTurno{store: e, subagente: subagente, ahora: cuando},
			turnInput{Prompt: "x", SessionID: "s", PermissionMode: modo, AgentID: agente})
	}

	// Donde el subagente no puede trabajar solo, ni aviso ni unidades.
	for caso, args := range map[string][3]string{
		"modo normal":         {sub, "default", ""},
		"aceptar ediciones":   {sub, "acceptEdits", ""},
		"sin modo":            {sub, "", ""},
		"dentro de subagente": {sub, "auto", "agente-123"},
		"sin subagente":       {"", "auto", ""},
	} {
		e := storeConTareas(t)
		if out := aviso(e, args[0], args[1], args[2], ahora); out != "" {
			t.Errorf("%s: avisó %q", caso, out)
		}
		if lote := loteDeCuarentena(t, e); lote.Total != 0 {
			t.Errorf("%s: posteó %d unidad(es) sin poder lanzar a nadie", caso, lote.Total)
		}
	}

	for _, modo := range []string{"auto", "bypassPermissions"} {
		e := storeConTareas(t)
		out := aviso(e, sub, modo, "", ahora)
		if !strings.Contains(out, `subagent_type "musubi:musubi-tareas"`) || !strings.Contains(out, "tool Agent") {
			t.Fatalf("en %s el aviso no nombra la tool y el subagente exactos: %q", modo, out)
		}
		if lote := loteDeCuarentena(t, e); lote.Total != 1 {
			t.Errorf("en %s el lote tiene %d unidad(es); quería 1", modo, lote.Total)
		}
		// Mismo intervalo: nada. Pasado el intervalo, con la unidad todavía abierta: otra vez.
		if out := aviso(e, sub, modo, "", ahora.Add(time.Minute)); out != "" {
			t.Errorf("en %s avisó dos veces dentro del intervalo: %q", modo, out)
		}
		if out := aviso(e, sub, modo, "", ahora.Add(intervaloDeTareas+time.Minute)); out == "" {
			t.Errorf("en %s no volvió a avisar pasado el intervalo, con la unidad abierta", modo)
		}
		if lote := loteDeCuarentena(t, e); lote.Total != 1 {
			t.Errorf("en %s el segundo aviso reposteó: el lote tiene %d unidad(es)", modo, lote.Total)
		}
	}

	// Con todo hecho, no hay nada que delegar.
	e := storeConTareas(t)
	if out := aviso(e, sub, "auto", "", ahora); out == "" {
		t.Fatal("control: el primer aviso no salió")
	}
	u, ok, _ := e.ClaimWorkUnit(memory.LoteCuarentena, agenteEnElTablero, 300, 5)
	if !ok {
		t.Fatal("no se pudo reclamar la unidad")
	}
	if err := e.CompleteWorkUnitConEfecto(u.ID, "hecha", memory.WorkDone, agenteEnElTablero, u.FencingToken, memory.EffectReport); err != nil {
		t.Fatal(err)
	}
	if out := aviso(e, sub, "auto", "", ahora.Add(intervaloDeTareas+time.Minute)); out != "" {
		t.Errorf("con todas las unidades hechas avisó igual: %q", out)
	}
}

// reToolDeMusubi reconoce el nombre de una tool de Musubi en un texto.
var reToolDeMusubi = regexp.MustCompile(`musubi_[a-z0-9_]+`)

// EL PLUGIN TRAE EL SUBAGENTE, CON LAS TOOLS QUE SU PROTOCOLO NOMBRA, Y EL HOOK LO ENCUENTRA.
//
// Un protocolo que nombra una tool que el frontmatter no le da manda al subagente a una tool que no
// tiene: se DERIVA del texto. Cada tool de Musubi va con los dos nombres (el del servidor del plugin
// y el de un repo cableado a mano), y el subagente corre en segundo plano sí o sí.
//
// Sabotaje que la hace fallar: no escribir el subagente al instalar.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\tif err := escribirSubagenteDeTareas(dir, nombrePlugin, nombrePlugin); err != nil {\n"
// arnes: a="\tif err := error(nil); err != nil {\n"
//
// Sabotaje que la hace fallar: darle sólo el nombre de las tools del plugin.
// arnes: archivo="cmd/musubi/subagente_tareas.go"
// arnes: de="\t\tout = append(out, \"mcp__plugin_\"+plugin+\"_\"+servidor+\"__\"+t, \"mcp__musubi__\"+t)\n"
// arnes: a="\t\tout = append(out, \"mcp__plugin_\"+plugin+\"_\"+servidor+\"__\"+t)\n"
//
// Sabotaje que la hace fallar: que el protocolo nombre una tool que el subagente no tiene.
// arnes: archivo="cmd/musubi/subagente_tareas.go"
// arnes: de="var toolsDeMusubiDelSubagente = []string{\"musubi_work\", \"musubi_memory_expand\", \"musubi_corroborate\", \"musubi_discard_proposal\", \"musubi_recall\"}\n"
// arnes: a="var toolsDeMusubiDelSubagente = []string{\"musubi_work\", \"musubi_memory_expand\", \"musubi_corroborate\", \"musubi_recall\"}\n"
func TestElPluginTraeElSubagenteDeTareas(t *testing.T) {
	home := t.TempDir()
	plugin := filepath.Join(home, ".claude", "skills", "musubi")
	if err := instalarPlugin(plugin, "musubi", "0.0.0-prueba"); err != nil {
		t.Fatal(err)
	}
	crudo, err := os.ReadFile(filepath.Join(plugin, "agents", "musubi-tareas.md"))
	if err != nil {
		t.Fatalf("el plugin no trae el subagente: %v", err)
	}
	texto := string(crudo)
	partes := strings.SplitN(texto, "---\n", 3)
	if len(partes) != 3 || partes[0] != "" {
		t.Fatalf("el subagente no abre con un frontmatter:\n%.300s", texto)
	}
	campos := map[string]string{}
	for _, l := range strings.Split(partes[1], "\n") {
		if k, v, ok := strings.Cut(l, ": "); ok {
			campos[k] = v
		}
	}
	if campos["name"] != "musubi-tareas" || campos["background"] != "true" {
		t.Errorf("frontmatter: name=%q background=%q; quería musubi-tareas y true", campos["name"], campos["background"])
	}
	tools := map[string]bool{}
	for _, tl := range strings.Split(campos["tools"], ", ") {
		tools[tl] = true
	}
	nombradas := map[string]bool{}
	for _, n := range reToolDeMusubi.FindAllString(partes[2], -1) {
		nombradas[n] = true
	}
	if len(nombradas) == 0 {
		t.Fatal("el protocolo no nombra ninguna tool de Musubi: la derivación no mide nada")
	}
	var faltan []string
	for n := range nombradas {
		for _, completo := range []string{"mcp__plugin_musubi_musubi__" + n, "mcp__musubi__" + n} {
			if !tools[completo] {
				faltan = append(faltan, completo)
			}
		}
	}
	sort.Strings(faltan)
	if len(faltan) > 0 {
		t.Errorf("el protocolo nombra tools que el subagente no tiene: %v", faltan)
	}
	if !strings.Contains(partes[2], memory.LoteCuarentena) {
		t.Errorf("el protocolo no nombra el lote %s", memory.LoteCuarentena)
	}

	// El hook lo encuentra: como plugin, y desde un repo cableado a mano (carpeta de siempre).
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "") // la carpeta de siempre, no la de la máquina que corre la prueba
	t.Setenv("CLAUDE_PLUGIN_ROOT", plugin)
	if got := subagenteDeTareasDisponible(); got != "musubi:musubi-tareas" {
		t.Errorf("como plugin, el subagente es %q; quería musubi:musubi-tareas", got)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	if got := subagenteDeTareasDisponible(); got != "musubi:musubi-tareas" {
		t.Errorf("desde un repo cableado, el subagente es %q; quería musubi:musubi-tareas", got)
	}
	if err := os.Remove(filepath.Join(plugin, "agents", "musubi-tareas.md")); err != nil {
		t.Fatal(err)
	}
	if got := subagenteDeTareasDisponible(); got != "" {
		t.Errorf("sin el archivo del subagente, el hook lo da por disponible: %q", got)
	}
}

// EL HOOK REAL POSTEA Y AVISA, Y `musubi tareas` MUESTRA LO QUE HIZO EL SUBAGENTE.
//
// Se mide sobre el proceso: el cableado vive en runTurn, y una prueba sobre buildTurnTareas mediría
// el ayudante y no el hook.
//
// Sabotaje que la hace fallar: que el hook del turno no pase las tareas.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\ttareas := &tareasDelTurno{store: engine, subagente: subagenteDeTareasDisponible(), ahora: time.Now()}\n"
// arnes: a="\tvar tareas *tareasDelTurno\n"
func TestElHookDelTurnoLeDejaLasTareasAlAgente(t *testing.T) {
	home := t.TempDir()
	repo := proyectoConMemoria(t, home)
	plugin := filepath.Join(home, ".claude", "skills", "musubi")
	if err := instalarPlugin(plugin, "musubi", "0.0.0-prueba"); err != nil {
		t.Fatal(err)
	}
	e, err := memory.NewDbEngine(repo)
	if err != nil {
		t.Fatal(err)
	}
	vieja := proponer(t, e, "t/estado", "estado viejo del trabajo")
	nueva := proponer(t, e, "t/estado", "estado nuevo del trabajo")
	e.Close()

	evento := `{"session_id":"s","prompt":"seguimos con lo de ayer","permission_mode":"auto"}`
	out := correrMusubiCon(t, repo, home, evento, []string{"CLAUDE_PLUGIN_ROOT=" + plugin}, "turn", "--hook-mode")
	if !strings.Contains(out, "[Musubi — tareas]") || !strings.Contains(out, "musubi:musubi-tareas") {
		t.Fatalf("el hook no avisó las tareas:\n%s", out)
	}

	// Lo que hace el subagente, a mano: reclamar, descartar la vieja, corroborar la nueva, cerrar.
	e, err = memory.NewDbEngine(repo)
	if err != nil {
		t.Fatal(err)
	}
	u, ok, err := e.ClaimWorkUnit(memory.LoteCuarentena, agenteEnElTablero, 300, 5)
	if !ok || err != nil {
		t.Fatalf("el hook no dejó una unidad reclamable: ok=%v err=%v", ok, err)
	}
	if got := idsDeLaUnidad(u.Spec); !igualesIDs(got, []string{vieja, nueva}) {
		t.Fatalf("la unidad cubre %v; quería el tema entero", got)
	}
	if err := e.DescartarPropuesta(vieja, nueva); err != nil {
		t.Fatal(err)
	}
	if err := e.CorroborateObservation(nueva); err != nil {
		t.Fatal(err)
	}
	resultado := "[" + vieja[:8] + "] DESCARTADA — la reemplaza el estado nuevo\n[" + nueva[:8] + "] CORROBORADA — se comprueba en el repo"
	if err := e.CompleteWorkUnitConEfecto(u.ID, resultado, memory.WorkDone, agenteEnElTablero, u.FencingToken, memory.EffectApply); err != nil {
		t.Fatal(err)
	}
	e.Close()

	t.Setenv("MUSUBI_HOME", repo)
	var tabla, errOut bytes.Buffer
	if code := tareasCmd(nil, &tabla, &errOut); code != 0 {
		t.Fatalf("musubi tareas salió %d: %s", code, errOut.String())
	}
	for _, quiere := range []string{"0 propuesta(s) vivas", "1 hechas", "DESCARTADA", "CORROBORADA"} {
		if !strings.Contains(tabla.String(), quiere) {
			t.Errorf("musubi tareas no dice %q:\n%s", quiere, tabla.String())
		}
	}
}

// UN TURNO AJENO NO AVISA NI GASTA NADA; CADA SESIÓN RECIBE SU AVISO; POSTEAR SIGUE SIENDO POR PROYECTO.
//
// Medido el 2026-09-26: el primer aviso cayó en una notificación de una tarea de fondo, en una
// sesión que no tenía el subagente, y el segundo otra vez en esa sesión. Con el aviso por proyecto,
// las sesiones que sí podían delegar se quedaban dos horas sin él cada vez. Ahora un turno ajeno
// no toca el intervalo, y el aviso se cuenta por sesión; que dos sesiones deleguen a la vez no
// duplica trabajo, porque cada subagente reclama unidades distintas. Lo que sigue siendo por
// proyecto es el posteo: el trabajo NUEVO entra al tablero a lo sumo cada intervaloDeTareas.
//
// Los prefijos de los turnos ajenos se escriben literales: son del formato de Claude Code.
//
// Sabotaje que la hace fallar: avisar también en un turno que no escribió la persona.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\tif !esTurnoDeLaPersona(in.Prompt) {\n"
// arnes: a="\tif false && !esTurnoDeLaPersona(in.Prompt) {\n"
//
// Sabotaje que la hace fallar: un solo aviso por proyecto, no por sesión.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\tif !avisoVencidoEnLaSesion(t.store, in.SessionID, t.ahora) {\n"
// arnes: a="\tif !avisoVencidoEnLaSesion(t.store, \"proyecto\", t.ahora) {\n"
//
// Sabotaje que la hace fallar: postear en cada turno.
// arnes: archivo="cmd/musubi/tareas.go"
// arnes: de="\treturn ahora.Sub(ultima) >= intervaloDeTareas\n"
// arnes: a="\treturn ahora.Sub(ultima) >= 0\n"
func TestElAvisoEsPorSesionYSoloEnTurnosDeLaPersona(t *testing.T) {
	const sub = "musubi:musubi-tareas"
	ahora := time.Now()
	e := storeConTareas(t)
	turno := func(sesion, prompt string, cuando time.Time) string {
		return buildTurnTareas(&tareasDelTurno{store: e, subagente: sub, ahora: cuando},
			turnInput{Prompt: prompt, SessionID: sesion, PermissionMode: "auto"})
	}

	// Los turnos ajenos no avisan, no postean y no gastan el intervalo.
	for _, ajeno := range []string{
		"<task-notification>\n<task-id>b1</task-id>\n<status>completed</status>",
		"Another Claude session sent you a message: ¿seguís con lo tuyo?",
	} {
		if out := turno("a", ajeno, ahora); out != "" {
			t.Errorf("un turno ajeno (%.30q) avisó: %q", ajeno, out)
		}
	}
	if lote := loteDeCuarentena(t, e); lote.Total != 0 {
		t.Fatalf("un turno ajeno posteó %d unidad(es)", lote.Total)
	}
	// El primer turno de la persona, justo después, sí: el intervalo no se gastó.
	if out := turno("a", "seguimos", ahora); out == "" {
		t.Fatal("el turno de la persona después de dos ajenos no avisó: el intervalo se gastó en turnos ajenos")
	}

	// Otra sesión, en el mismo momento, también recibe el suyo, y no se postea de nuevo.
	if out := turno("b", "hola", ahora.Add(time.Minute)); out == "" {
		t.Error("la segunda sesión no recibió el aviso: sigue siendo uno por proyecto")
	}
	if lote := loteDeCuarentena(t, e); lote.Total != 1 {
		t.Errorf("con dos sesiones el lote tiene %d unidad(es); quería 1", lote.Total)
	}
	// La misma sesión, dentro del intervalo, no.
	if out := turno("a", "otra cosa", ahora.Add(2*time.Minute)); out != "" {
		t.Errorf("la sesión «a» recibió dos avisos dentro del intervalo: %q", out)
	}

	// Una propuesta nueva entra al tablero recién cuando vence el intervalo del PROYECTO, aunque
	// otra sesión nueva hable antes.
	proponer(t, e, "t/nuevo", "un tema nuevo")
	turno("c", "hola desde otra", ahora.Add(3*time.Minute))
	if lote := loteDeCuarentena(t, e); lote.Total != 1 {
		t.Errorf("la propuesta nueva se posteó antes de que venciera el intervalo del proyecto (lote: %d)", lote.Total)
	}
	turno("c", "más tarde", ahora.Add(intervaloDeTareas+time.Minute))
	if lote := loteDeCuarentena(t, e); lote.Total != 2 {
		t.Errorf("vencido el intervalo, la propuesta nueva no entró al tablero (lote: %d)", lote.Total)
	}
}
