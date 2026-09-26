package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"musubi/internal/memory"
)

// tareas.go — MUSUBI LE DEJA TRABAJO AL AGENTE, Y EL AGENTE LO HACE SOLO.
//
// El pedido del usuario (2026-09-25): que Musubi cree y ejecute tareas, y que el agente las llame
// solo, aprovechando el plugin para lo que Musubi, sin modelo, no puede hacer. El caso que lo motivó
// está medido: 125 propuestas en cuarentena en la memoria de este repo y ninguna corroborada nunca
// (ver internal/memory/tareas_del_sistema.go).
//
// El circuito tiene tres piezas, y ninguna necesita a la persona:
//
//   - el PRODUCTOR, acá: en el hook del turno y sin modelo, postea en el tablero (lote musubi/…) el
//     trabajo que ninguna unidad cubre todavía;
//   - el AVISO, acá también: un bloque que le pide al agente delegarlo YA a un subagente;
//   - el SUBAGENTE musubi-tareas, que trae el plugin (subagente_tareas.go): reclama las unidades,
//     verifica contra el repo y da el veredicto con las tools de Musubi, en segundo plano.
//
// CUÁNDO NO SE DISPARA, Y POR QUÉ:
//
//   - Dentro de un subagente: el trabajo lo delega la sesión principal.
//   - Si el modo de permisos no deja trabajar al subagente sin preguntar. Un plugin NO puede darse
//     permisos —Claude Code sólo le acepta los ajustes `agent` y `subagentStatusLine`, medido en el
//     binario 2.1.223—, así que en modo normal cada veredicto le pediría confirmación a la persona
//     desde un trabajo de fondo: peor que no hacerlo. Corre en auto (decide el clasificador) y en
//     bypassPermissions.
//   - Si el subagente no está instalado: sin quién lo haga, el aviso mandaría al agente a un callejón.
//   - En un turno que no escribió la persona: una notificación de una tarea de fondo o un mensaje de
//     otra sesión (prefijosDeTurnoAjeno).
//
// POSTEAR ES POR PROYECTO, AVISAR ES POR SESIÓN. El trabajo nuevo entra al tablero a lo sumo cada
// intervaloDeTareas por proyecto, y eso es lo que acota el costo. El aviso sale una vez por intervalo
// en CADA sesión. La primera versión avisaba una vez por proyecto, y el 2026-09-26 el aviso se gastó
// dos veces seguidas en la misma sesión equivocada: una que había arrancado antes de instalar el
// plugin (la delegación falló con «Agent type 'musubi:musubi-tareas' not found») y que además lo
// recibió en un turno de notificación. Las sesiones que sí podían delegar se quedaron dos horas sin
// aviso cada vez. Que dos sesiones deleguen a la vez no duplica trabajo: cada subagente reclama
// unidades distintas del tablero.

const (
	// intervaloDeTareas es cada cuánto, como mucho, un proyecto postea lo nuevo en el tablero, y cada
	// cuánto, como mucho, una sesión recibe el aviso.
	intervaloDeTareas = 2 * time.Hour
	// propuestasPorUnidad es cuántas propuestas lleva una unidad. Orientativo: un tema no se parte.
	propuestasPorUnidad = 5
	// topeDeUnidadesAbiertas acota lo que espera en el tablero. Sin tope, cada ronda postearía más de
	// lo que una corrida del subagente alcanza a hacer, y la cola crecería sola.
	topeDeUnidadesAbiertas = 4
	// metaRondaDeTareas guarda cuándo corrió la última ronda en este proyecto.
	metaRondaDeTareas = "tareas_ultima_ronda"
	// metaAvisoDeTareas guarda, por sesión, cuándo recibió el último aviso.
	metaAvisoDeTareas = "tareas_aviso_por_sesion"
	// prefijoDeIDs abre el renglón de la spec que dice qué propuestas cubre la unidad. Es lo que el
	// productor lee para no volver a postear una propuesta ya cubierta.
	prefijoDeIDs = "ids: "
)

// modosQueCorrenSolos son los modos de permisos de Claude Code en que un subagente de fondo trabaja
// sin pedirle confirmación a la persona. Son nombres del formato de otro programa: se clavan.
var modosQueCorrenSolos = map[string]bool{"auto": true, "bypassPermissions": true}

// prefijosDeTurnoAjeno son los comienzos del texto de un turno que NO escribió la persona. Son
// hechos del formato de Claude Code, medidos el 2026-09-26 en los transcripts de esta máquina: 1186
// turnos que empiezan con `<task-notification>` (terminó una tarea de fondo) y 193 con `Another
// Claude session` (un mensaje de otra sesión). Se clavan: si Claude Code los cambia, el aviso vuelve
// a salir también en esos turnos, que es como era antes, no un daño.
var prefijosDeTurnoAjeno = []string{"<task-notification>", "Another Claude session"}

// esTurnoDeLaPersona dice si el turno lo escribió la persona.
//
// En un turno ajeno el agente está en medio de otra cosa —procesando lo que terminó una tarea suya—
// y el aviso compite con eso: el 2026-09-26 el primero cayó en uno así y el agente siguió de largo.
func esTurnoDeLaPersona(prompt string) bool {
	p := strings.TrimSpace(prompt)
	for _, pre := range prefijosDeTurnoAjeno {
		if strings.HasPrefix(p, pre) {
			return false
		}
	}
	return true
}

// almacenDeTareas es lo que el productor necesita de la memoria. *memory.DbEngine lo satisface.
type almacenDeTareas interface {
	PropuestasEnCuarentena() ([]memory.PropuestaEnCuarentena, error)
	WorkBatchStatus(batchID string) (memory.WorkBatch, error)
	SumarAlLote(batchID string, specs []memory.WorkUnitSpec) (memory.WorkBatch, error)
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
}

// tareasDelTurno es lo que el hook del turno necesita para dejarle trabajo al agente. nil ⇒ el turno
// no postea ni avisa nada.
type tareasDelTurno struct {
	store     almacenDeTareas
	subagente string // cómo lo nombra el agente en la tool Agent: «musubi:musubi-tareas»
	ahora     time.Time
}

// rondaDeTareas es lo que dejó una ronda del productor.
type rondaDeTareas struct {
	postadas    int // unidades nuevas en esta ronda
	reclamables int // unidades que el subagente puede tomar ya
	propuestas  int // propuestas vivas en cuarentena
}

// buildTurnTareas postea lo nuevo si le toca al proyecto, y devuelve el aviso para el agente si le
// toca a esta sesión y hay algo que delegar; si no, "".
func buildTurnTareas(t *tareasDelTurno, in turnInput) string {
	if t == nil || t.store == nil || t.subagente == "" {
		return ""
	}
	if in.AgentID != "" || !modosQueCorrenSolos[in.PermissionMode] {
		return ""
	}
	// Un turno ajeno no gasta nada: ni la ronda del proyecto ni el aviso de la sesión.
	if !esTurnoDeLaPersona(in.Prompt) {
		return ""
	}
	if rondaVencida(t.store, t.ahora) {
		// La marca va ANTES de la ronda: si la ronda falla, se reintenta en el próximo intervalo y no
		// en cada turno, que pagaría el error una y otra vez dentro del techo de 10 s del hook.
		_ = t.store.SetMeta(metaRondaDeTareas, t.ahora.UTC().Format(time.RFC3339))
		if _, err := correrRondaDeTareas(t.store, t.ahora); err != nil {
			fmt.Fprintf(os.Stderr, "musubi turn: la ronda de tareas falló: %v\n", err)
		}
	}
	r, err := contarTareas(t.store, t.ahora)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi turn: no pude contar las tareas: %v\n", err)
		return ""
	}
	if r.reclamables == 0 {
		return ""
	}
	if !avisoVencidoEnLaSesion(t.store, in.SessionID, t.ahora) {
		return ""
	}
	return avisoDeTareas(r, t.subagente)
}

// avisoVencidoEnLaSesion dice si a esta sesión le toca el aviso —nunca lo recibió, o lo recibió hace
// al menos intervaloDeTareas— y, si le toca, anota que lo recibe ahora.
func avisoVencidoEnLaSesion(store metaStore, sessionID string, ahora time.Time) bool {
	m := leerMarcasPorSesion(store, metaAvisoDeTareas)
	if v, ok := m.Valor[sessionID]; ok {
		if ultimo, err := time.Parse(time.RFC3339, v); err == nil && ahora.Sub(ultimo) < intervaloDeTareas {
			return false
		}
	}
	guardarMarcaDeSesion(store, metaAvisoDeTareas, m, sessionID, ahora.UTC().Format(time.RFC3339))
	return true
}

// contarTareas cuenta, sin postear nada, las propuestas vivas y las unidades reclamables.
func contarTareas(store almacenDeTareas, ahora time.Time) (rondaDeTareas, error) {
	var r rondaDeTareas
	props, err := store.PropuestasEnCuarentena()
	if err != nil {
		return r, err
	}
	lote, err := store.WorkBatchStatus(memory.LoteCuarentena)
	if err != nil {
		return r, err
	}
	r.propuestas = len(props)
	r.reclamables = unidadesReclamables(lote, ahora)
	return r, nil
}

// rondaVencida dice si ya pasó el intervalo desde la última ronda del proyecto. Una marca ilegible
// cuenta como vencida: a lo sumo una ronda de más.
func rondaVencida(store almacenDeTareas, ahora time.Time) bool {
	v, ok, err := store.GetMeta(metaRondaDeTareas)
	if err != nil || !ok {
		return true
	}
	ultima, err := time.Parse(time.RFC3339, strings.TrimSpace(v))
	if err != nil {
		return true
	}
	return ahora.Sub(ultima) >= intervaloDeTareas
}

// correrRondaDeTareas postea las propuestas que ninguna unidad cubre, sin pasar el tope de unidades
// abiertas, y cuenta lo que queda para el subagente.
func correrRondaDeTareas(store almacenDeTareas, ahora time.Time) (rondaDeTareas, error) {
	var r rondaDeTareas
	props, err := store.PropuestasEnCuarentena()
	if err != nil {
		return r, err
	}
	r.propuestas = len(props)
	lote, err := store.WorkBatchStatus(memory.LoteCuarentena)
	if err != nil {
		return r, err
	}
	cubiertas := map[string]bool{}
	for _, u := range lote.Units {
		for _, id := range idsDeLaUnidad(u.Spec) {
			cubiertas[id] = true
		}
	}
	r.reclamables = unidadesReclamables(lote, ahora)
	if hueco := topeDeUnidadesAbiertas - r.reclamables; hueco > 0 {
		if specs := unidadesDeCuarentena(props, cubiertas, hueco); len(specs) > 0 {
			nuevo, err := store.SumarAlLote(memory.LoteCuarentena, specs)
			if err != nil {
				return r, err
			}
			r.postadas = len(specs)
			r.reclamables = unidadesReclamables(nuevo, ahora)
		}
	}
	return r, nil
}

// unidadesReclamables cuenta las unidades que el subagente puede tomar: abiertas, o reclamadas con
// el lease vencido (quien la tenía se fue). Es la misma regla que ClaimWorkUnit.
func unidadesReclamables(lote memory.WorkBatch, ahora time.Time) int {
	n := 0
	for _, u := range lote.Units {
		switch u.Status {
		case memory.WorkOpen:
			n++
		case memory.WorkClaimed:
			vence, err := time.Parse("2006-01-02 15:04:05", u.LeaseExpiresAt)
			if u.LeaseExpiresAt == "" || (err == nil && vence.Before(ahora.UTC())) {
				n++
			}
		}
	}
	return n
}

// temaDePropuestas es un tema con sus propuestas sin cubrir, de la más vieja a la más nueva.
type temaDePropuestas struct {
	tema string
	ids  []string
}

// unidadesDeCuarentena arma hasta `tope` unidades con las propuestas que ninguna unidad cubre.
//
// UN TEMA NO SE PARTE: la serie de estados de un mismo trabajo se juzga junta, porque el veredicto
// de la vieja depende de la nueva —la nueva la reemplaza—. Una propuesta sin tema va sola. Los temas
// entran en el orden de su propuesta más vieja, así lo que más esperó sale primero.
func unidadesDeCuarentena(props []memory.PropuestaEnCuarentena, cubiertas map[string]bool, tope int) []memory.WorkUnitSpec {
	var temas []*temaDePropuestas
	porTema := map[string]*temaDePropuestas{}
	for _, p := range props {
		if cubiertas[p.ID] {
			continue
		}
		if p.TopicKey == "" {
			temas = append(temas, &temaDePropuestas{ids: []string{p.ID}})
			continue
		}
		t := porTema[p.TopicKey]
		if t == nil {
			t = &temaDePropuestas{tema: p.TopicKey}
			porTema[p.TopicKey] = t
			temas = append(temas, t)
		}
		t.ids = append(t.ids, p.ID)
	}
	var specs []memory.WorkUnitSpec
	var actual []*temaDePropuestas
	cantidad := 0
	cerrar := func() {
		if len(actual) > 0 {
			specs = append(specs, specDeCuarentena(actual))
		}
		actual, cantidad = nil, 0
	}
	for _, t := range temas {
		if len(specs) == tope {
			break
		}
		if cantidad > 0 && cantidad+len(t.ids) > propuestasPorUnidad {
			cerrar()
			if len(specs) == tope {
				break
			}
		}
		actual = append(actual, t)
		cantidad += len(t.ids)
	}
	if len(specs) < tope {
		cerrar()
	}
	return specs
}

// specDeCuarentena escribe la unidad: los temas con sus ids en orden, y el renglón de ids que lee el
// productor. El PROCEDIMIENTO no va acá: vive en el subagente, que es quien lo sigue.
func specDeCuarentena(temas []*temaDePropuestas) memory.WorkUnitSpec {
	var b strings.Builder
	b.WriteString("Propuestas de memoria en cuarentena que esperan veredicto, por tema y de la más vieja a la más nueva:\n")
	var todos []string
	for _, t := range temas {
		nombre := "sin tema"
		if t.tema != "" {
			nombre = "tema «" + t.tema + "»"
		}
		fmt.Fprintf(&b, "- %s: %s\n", nombre, strings.Join(t.ids, ", "))
		todos = append(todos, t.ids...)
	}
	b.WriteString(prefijoDeIDs + strings.Join(todos, " "))
	return memory.WorkUnitSpec{
		Title: fmt.Sprintf("cuarentena: %d propuesta(s) en %d tema(s)", len(todos), len(temas)),
		Spec:  b.String(),
	}
}

// idsDeLaUnidad lee el renglón de ids de una unidad.
func idsDeLaUnidad(spec string) []string {
	for _, l := range strings.Split(spec, "\n") {
		if resto, ok := strings.CutPrefix(l, prefijoDeIDs); ok {
			return strings.Fields(resto)
		}
	}
	return nil
}

// avisoDeTareas es el bloque que le pide al agente delegar. Nombra el subagente exacto y la tool,
// porque es la forma que se sigue: un aviso atado a una acción concreta, con el nombre justo.
func avisoDeTareas(r rondaDeTareas, subagente string) string {
	return fmt.Sprintf("[Musubi — tareas] Musubi te dejó %d tarea(s) en su tablero que necesitan criterio y "+
		"que Musubi, sin modelo, no puede hacer solo: hay %d propuesta(s) de memoria en cuarentena esperando "+
		"veredicto, y hasta que alguien se lo dé el recall no las ve. Delegalas YA con la tool Agent: "+
		"subagent_type %q, prompt «Hacé las tareas pendientes de Musubi.». Corre en segundo plano y no frena "+
		"lo que te pidieron: seguí con tu trabajo, no las hagas vos y no esperes su resultado.",
		r.reclamables, r.propuestas, subagente)
}

// subagenteDeTareasDisponible dice cómo nombra el agente al subagente de tareas, o "" si no está.
//
// Corriendo como plugin, es el del plugin que lanzó este hook. En un repo cableado a mano los hooks
// del plugin ceden y corren los del proyecto (agente_plugin.go), así que es ESTE proceso el que tiene
// que saber del plugin: se busca en la carpeta donde lo instala `musubi agente instalar`.
func subagenteDeTareasDisponible() string {
	raiz := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if raiz == "" {
		d, err := dirDelPlugin()
		if err != nil {
			return ""
		}
		raiz = d
	}
	if _, err := os.Stat(rutaDelSubagenteDeTareas(raiz)); err != nil {
		return ""
	}
	nombre, err := nombreDelPluginEn(raiz)
	if err != nil || nombre == "" {
		return ""
	}
	return nombre + ":" + subagenteDeTareas
}

// ── musubi tareas ─────────────────────────────────────────────────────────────────────────────

func runTareas(args []string) {
	os.Exit(tareasCmd(args, os.Stdout, os.Stderr))
}

// tareasCmd muestra el tablero de las tareas del sistema en el proyecto: qué hay, qué se hizo y qué
// dijo el subagente de cada unidad. Es la medición de CONSUMO del circuito: que el hook avise no dice
// nada; lo que importa es cuántas unidades se cerraron y con qué veredictos.
func tareasCmd(args []string, out, errOut io.Writer) int {
	fl := flag.NewFlagSet("tareas", flag.ContinueOnError)
	fl.SetOutput(errOut)
	comoJSON := fl.Bool("json", false, "emitir el tablero como JSON")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	r := raizDelProceso()
	if r.Dir == "" || r.Activar {
		fmt.Fprintf(errOut, "musubi tareas: sin memoria de Musubi en esta carpeta (%s)\n", r.Motivo)
		return 1
	}
	engine, err := memory.NewDbEngine(r.Dir)
	if err != nil {
		fmt.Fprintf(errOut, "musubi tareas: no pude abrir la memoria: %v\n", err)
		return 1
	}
	defer engine.Close()
	props, err := engine.PropuestasEnCuarentena()
	if err != nil {
		fmt.Fprintf(errOut, "musubi tareas: %v\n", err)
		return 1
	}
	lote, err := engine.WorkBatchStatus(memory.LoteCuarentena)
	if err != nil {
		fmt.Fprintf(errOut, "musubi tareas: %v\n", err)
		return 1
	}
	if *comoJSON {
		b, _ := json.MarshalIndent(map[string]interface{}{
			"proyecto":                 r.Dir,
			"propuestas_en_cuarentena": len(props),
			"lote":                     lote,
		}, "", "  ")
		fmt.Fprintln(out, string(b))
		return 0
	}
	imprimirTareas(out, r.Dir, len(props), lote)
	return 0
}

func imprimirTareas(w io.Writer, dir string, propuestas int, lote memory.WorkBatch) {
	fmt.Fprintf(w, "Tareas que Musubi le deja al agente — %s\n", dir)
	fmt.Fprintf(w, "  cuarentena: %d propuesta(s) vivas esperando veredicto\n", propuestas)
	if lote.Total == 0 {
		fmt.Fprintln(w, "  tablero   : sin unidades todavía (se postean en el hook del turno, en modo auto)")
		return
	}
	fmt.Fprintf(w, "  tablero   : %d unidad(es) · %d abiertas · %d en curso · %d hechas · %d fallidas\n",
		lote.Total, lote.Open, lote.Claimed, lote.Done, lote.Failed)
	for _, u := range lote.Units {
		if u.Status != memory.WorkDone && u.Status != memory.WorkFailed {
			continue
		}
		fmt.Fprintf(w, "\n  ── %s [%s]\n", u.Title, u.Status)
		for _, l := range strings.Split(strings.TrimSpace(u.Result), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				fmt.Fprintf(w, "     %s\n", l)
			}
		}
	}
}

// rutaDelSubagenteDeTareas es el archivo del subagente dentro del plugin.
func rutaDelSubagenteDeTareas(raizPlugin string) string {
	return filepath.Join(raizPlugin, dirAgentesDelPlugin, subagenteDeTareas+".md")
}
