package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// uso_agente.go implementa `musubi uso-agente`: un medidor de SÓLO LECTURA sobre los transcripts de
// Claude Code, para saber si el agente USA de verdad las tools y las skills de Musubi.
//
// MIDE CONSUMO, NO EJECUCIÓN. El ledger de la base (`tool_invocations`) dice cuántas veces corrió
// una tool, pero no quién la pidió ni por qué: medido el 2026-09-25, `principal` y `project_id`
// estaban vacíos en el 100 % de sus 1.002 filas. Y lo que se crea solo casi nadie lo usa: la
// pregunta que importa no es «¿el hook se inyectó?» sino «¿el agente hizo algo con eso?». Eso vive
// en un solo lugar, el transcript, que tiene el aviso, la búsqueda de la tool y la llamada, en
// orden.
//
// LO QUE ESTE COMANDO DISTINGUE, PORQUE CADA CERO PIDE UNA ACCIÓN DISTINTA:
//
//   - «0 llamadas» con las tools A LA VISTA es que el agente no las quiso;
//   - «0 llamadas» con las tools fuera de la vista es que no las podía ver;
//   - «sin medir» es que no hay un solo transcript en la ventana, y NO SE ESCRIBE COMO 0. Un cero
//     que significa «no sé» se lee igual que «medí y no hubo», y es la forma en que este repo más
//     veces se engañó a sí mismo.
//
// EL FORMATO DE LOS TRANSCRIPTS NO ESTÁ DOCUMENTADO, y lo que sigue se midió leyendo los 3.652
// `.jsonl` de esta máquina (2026-09-25, Claude Code 2.1.2xx):
//
//   - Cada línea es un registro con `type`, `uuid`, `timestamp` (siempre UTC con `Z`) y, según el
//     tipo, `message` o `attachment`.
//   - LOS REGISTROS SE REESCRIBEN AL REANUDAR UNA SESIÓN: 50.285 uuid repetidos DENTRO del mismo
//     archivo, ninguno entre archivos distintos (0 de 451.748). Sin deduplicar por `uuid` y cada
//     `tool_use` por su `id`, las llamadas del hilo principal a Musubi daban 1.243 en vez de 791.
//   - Lo que inyecta un hook llega como `attachment` de tipo `hook_additional_context`, con los
//     bloques «[Musubi — X]» adentro de `content`. El mismo texto puede aparecer DENTRO de un
//     `tool_result` —por ejemplo, cuando el agente lee `detect.go`— y eso NO es una inyección: el
//     agente leyó código, nadie le habló (629 encabezados así, ver `usuario`).
//   - Las carpetas de proyecto empiezan con `-` (`-home-davantis-musubi`), así que un glob
//     `*/*.jsonl` da cero. Se camina el árbol.
//   - Los subagentes viven bajo una carpeta `subagents/`; los de un workflow, además, en
//     `subagents/workflows/wf_*/`, junto a un `journal.jsonl` que es la bitácora del workflow y NO
//     un transcript (112 medidos): se excluye por nombre.
//   - Cada carpeta de proyecto es la ruta de trabajo con todo carácter que no sea letra o dígito
//     ASCII cambiado por `-` (`/home/davantis/.cache/x` → `-home-davantis--cache-x`). Por eso las
//     carpetas de experimento se reconocen por el nombre: ver exclusionesPorDefecto.

// Tipos de adjunto de Claude Code que este comando lee. Son strings del formato de OTRO programa y
// por eso van con nombre: si Claude Code los renombra, se cambian acá y en ningún otro lado.
const (
	adjuntoHook           = "hook_additional_context"
	adjuntoInstrucciones  = "mcp_instructions_delta"
	adjuntoDiferidasDelta = "deferred_tools_delta"
	adjuntoDiferidasLista = "deferred_tools_record"
	adjuntoListadoSkills  = "skill_listing"

	// journalDeWorkflow es la bitácora de un workflow: vive entre los transcripts y no es uno.
	journalDeWorkflow = "journal.jsonl"

	estadoMedido   = "medido"
	estadoSinMedir = "sin medir"
)

// reBloqueMusubi encuentra el encabezado de cada bloque que un hook de Musubi inyecta.
var reBloqueMusubi = regexp.MustCompile(`\[Musubi — ([^\]]+)\]`)

// reNombreTool encuentra los nombres pelados de tools de Musubi en un texto (`musubi_recall`).
//
// SIN `\b` Y SIN GUIONES BAJOS DOBLES, a propósito: en «mcp__musubi__musubi_recall» un `\b` no
// separa (el `_` es carácter de palabra) y un `[a-z_]+` se come «musubi__musubi_recall» entero, que
// no es el nombre de ninguna tool. Pedir segmentos separados por UN guion bajo hace que el primer
// intento falle en el `__` y el siguiente encuentre el nombre de verdad.
var reNombreTool = regexp.MustCompile(`musubi_[a-z0-9]+(?:_[a-z0-9]+)*`)

// InformeUso es lo que el comando mide, entero. Es también la forma del `--json`, y por eso sus
// claves son estables: los mapas salen ordenados y un alcance sin medir no trae números.
type InformeUso struct {
	Dir               string   `json:"dir"`
	Desde             string   `json:"desde,omitempty"`
	Hasta             string   `json:"hasta,omitempty"`
	Archivos          int      `json:"archivos_jsonl"`
	JournalExcluidos  int      `json:"journal_excluidos"`
	SalteadosPorFecha int      `json:"salteados_por_fecha"`
	LineasIlegibles   int      `json:"lineas_ilegibles"`
	SkillsDeMusubi    []string `json:"skills_de_musubi"`
	// Exclusiones son los patrones con que se descartaron carpetas de proyecto enteras, y
	// CarpetasExcluidas cuántas cayeron. Viajan en el informe porque un número medido sin decir qué
	// quedó afuera no se puede comparar con otro.
	Exclusiones       []string `json:"exclusiones"`
	CarpetasExcluidas int      `json:"carpetas_excluidas"`

	Principal  AlcanceUso `json:"principal"`
	Subagentes AlcanceUso `json:"subagentes"`
}

// AlcanceUso es la medición de las sesiones principales o la de los subagentes.
//
// EL PUNTERO NIL ES «SIN MEDIR», Y ESO ES LO QUE LO HACE IRREPRESENTABLE COMO CERO. Con los números
// adentro de un struct por valor, «sin medir» saldría en el JSON con `"llamadas": 0` al lado del
// estado, y quien consume el JSON lee el número, no el estado. Embebido como puntero, un alcance
// sin medir no tiene NINGUNA clave numérica: no hay cero que leer mal.
type AlcanceUso struct {
	Estado string `json:"estado"`
	Motivo string `json:"motivo,omitempty"`
	*MedicionUso
}

// MedicionUso son los números de un alcance medido.
type MedicionUso struct {
	Transcripts        int `json:"transcripts"`
	ConToolsALaVista   int `json:"con_tools_de_musubi_a_la_vista"`
	ConInstrucciones   int `json:"con_instrucciones_de_musubi"`
	ConSkillsEnListado int `json:"con_skills_de_musubi_en_el_listado"`

	Llamadas            int            `json:"llamadas_a_tools_de_musubi"`
	SinToolSearchPrevio int            `json:"llamadas_sin_toolsearch_previo"`
	Atribucion          AtribucionUso  `json:"atribucion"`
	PorTool             map[string]int `json:"llamadas_por_tool"`

	InvocacionesSkill int            `json:"invocaciones_de_skill"`
	SkillsDeMusubi    int            `json:"invocaciones_de_skills_de_musubi"`
	PorSkill          map[string]int `json:"invocaciones_por_skill"`

	Bloques map[string]int `json:"bloques_de_hook"`
}

// AtribucionUso reparte cada llamada a una tool de Musubi según QUIÉN SE LA NOMBRÓ al agente.
//
// Las categorías se preguntan en este orden y cada llamada cae en UNA: el hook del mismo turno, el
// prompt del turno, un hook de antes en la sesión, nadie. La tercera no estaba en el pedido y se
// agrega porque sin ella se esconde un número grande: medido el 2026-09-25, 212 de 791 llamadas del
// hilo principal las había nombrado un hook de un turno ANTERIOR. Sin la categoría, esas 212 caían
// en «nadie la nombró» y el agente parecía mucho más espontáneo de lo que es.
type AtribucionUso struct {
	HookMismoTurno int `json:"hook_mismo_turno"`
	Prompt         int `json:"prompt_la_nombraba"`
	HookAntes      int `json:"hook_antes_en_la_sesion"`
	Nadie          int `json:"nadie_la_nombro"`
}

func nuevaMedicionUso() *MedicionUso {
	return &MedicionUso{PorTool: map[string]int{}, PorSkill: map[string]int{}, Bloques: map[string]int{}}
}

// alcanceUso decide si un alcance se midió. Es el ÚNICO lugar que lo decide: la tabla y el JSON
// leen el resultado, así que no pueden contradecirse.
func alcanceUso(m *MedicionUso, quien string) AlcanceUso {
	if m.Transcripts == 0 {
		return AlcanceUso{Estado: estadoSinMedir,
			Motivo: "ningún transcript de " + quien + " tiene registros en la ventana"}
	}
	return AlcanceUso{Estado: estadoMedido, MedicionUso: m}
}

// ventanaUso es el intervalo de días pedido. `hasta` es EXCLUSIVO: el día siguiente al pedido a las
// 00:00, así `--hasta 2026-09-20` incluye todo el 20.
//
// LOS DÍAS SON UTC, como los timestamps de los transcripts. Interpretarlos en la hora local haría
// que la misma línea de comandos midiera ventanas distintas en davantis-1 y en la laptop.
type ventanaUso struct {
	desde, hasta       time.Time
	hayDesde, hayHasta bool
}

func parsearVentanaUso(desde, hasta string) (ventanaUso, error) {
	var v ventanaUso
	if desde != "" {
		t, err := time.Parse("2006-01-02", desde)
		if err != nil {
			return v, fmt.Errorf("--desde %q no es una fecha AAAA-MM-DD", desde)
		}
		v.desde, v.hayDesde = t, true
	}
	if hasta != "" {
		t, err := time.Parse("2006-01-02", hasta)
		if err != nil {
			return v, fmt.Errorf("--hasta %q no es una fecha AAAA-MM-DD", hasta)
		}
		v.hasta, v.hayHasta = t.AddDate(0, 0, 1), true
	}
	if v.hayDesde && v.hayHasta && !v.desde.Before(v.hasta) {
		return v, fmt.Errorf("--hasta %s es anterior a --desde %s: la ventana queda vacía", hasta, desde)
	}
	return v, nil
}

// ubicar dice si un registro con ese timestamp CUENTA (cae adentro de la ventana) y si ya quedó
// DESPUÉS de ella. Un registro de antes de la ventana no cuenta pero sí alimenta el estado —una
// tool cargada ayer sigue cargada hoy—; uno de después no hace nada.
func (v ventanaUso) ubicar(ts string) (cuenta, despues bool) {
	if !v.hayDesde && !v.hayHasta {
		return true, false
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return false, false
	}
	if v.hayHasta && !t.Before(v.hasta) {
		return false, true
	}
	if v.hayDesde && t.Before(v.desde) {
		return false, false
	}
	return true, false
}

// esServidorMusubi dice si un nombre de servidor MCP es de Musubi, en las dos formas en que Claude
// Code lo escribe: el que va en el nombre de una tool (`musubi`, `plugin_<plugin>_musubi`) y el que
// va en el adjunto de instrucciones (`musubi`, `plugin:<plugin>:musubi`).
//
// NO SE ATA AL PREFIJO `mcp__musubi__`: el día que Musubi se instale como plugin sus tools se van a
// llamar `mcp__plugin_<algo>_musubi__…`, y un medidor clavado al nombre de hoy diría «el agente
// dejó de usar Musubi» justo el día de la migración. También cuenta `musubi-cerebro`, el canal
// stdio al cerebro central (`musubi cerebro`): sirve las mismas tools, y medido el 2026-09-25 tenía
// 23 llamadas que un prefijo exacto habría tirado.
func esServidorMusubi(servidor string) bool {
	s := strings.ToLower(strings.TrimSpace(servidor))
	if strings.HasPrefix(s, "plugin_") || strings.HasPrefix(s, "plugin:") {
		// El nombre del plugin puede traer `_`; el del servidor es lo que sigue al último separador.
		s = s[strings.LastIndexAny(s, "_:")+1:]
	}
	return s == "musubi" || strings.HasPrefix(s, "musubi-")
}

// toolDeMusubi devuelve el nombre pelado de una tool (`musubi_recall`) si la sirve un servidor de
// Musubi. Lo que decide es el SERVIDOR, no que el nombre empiece con `musubi_`: otro servidor
// puede tener una tool con ese nombre, y contarla sería medir a Musubi con el uso de otro.
func toolDeMusubi(nombre string) (string, bool) {
	resto, ok := strings.CutPrefix(nombre, "mcp__")
	if !ok {
		return "", false
	}
	servidor, tool, ok := strings.Cut(resto, "__")
	if !ok || tool == "" || !esServidorMusubi(servidor) {
		return "", false
	}
	return tool, true
}

// nombreDeSkill le saca a un nombre de skill el prefijo de su origen (`anthropic-skills:pdf`,
// `musubi:plan-ahead`): lo que identifica a la skill es lo que sigue a los dos puntos.
func nombreDeSkill(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "/")
	return s[strings.LastIndex(s, ":")+1:]
}

// skillsDeMusubi son las skills que Musubi escribe, DERIVADAS de las cognitivas de cognitive.go y
// no copiadas a mano: una lista escrita acá se despegaría el día que se agregue la décima, y el
// medidor diría «0 invocaciones» de una skill que no sabe que existe.
func skillsDeMusubi() []string {
	var out []string
	for _, sk := range cognitiveSkills(nil) {
		out = append(out, sk.Name)
	}
	sort.Strings(out)
	return out
}

// registroTranscript es una línea de un transcript, con sólo los campos que este comando lee.
// `content` y `toolUseResult` quedan crudos porque su forma depende del tipo de registro.
type registroTranscript struct {
	Type          string             `json:"type"`
	UUID          string             `json:"uuid"`
	Timestamp     string             `json:"timestamp"`
	IsMeta        bool               `json:"isMeta"`
	Message       *mensajeTranscript `json:"message"`
	Attachment    *adjuntoTranscript `json:"attachment"`
	ToolUseResult json.RawMessage    `json:"toolUseResult"`
}

type mensajeTranscript struct {
	Content json.RawMessage `json:"content"`
}

// bloqueMensaje es un elemento de `message.content`: texto, tool_use, tool_result o, adentro del
// resultado de un ToolSearch, un tool_reference.
type bloqueMensaje struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	ToolName  string          `json:"tool_name"`
}

type adjuntoTranscript struct {
	Type       string          `json:"type"`
	Content    json.RawMessage `json:"content"`
	AddedNames []string        `json:"addedNames"`
	Names      []string        `json:"names"`
	Entries    []struct {
		Name string `json:"name"`
	} `json:"entries"`
}

// decodificarContenido lee un `content` que puede ser un string o una lista de bloques —las dos
// formas aparecen, en el prompt y en un tool_result—. Devuelve el texto (los bloques `text` unidos)
// y los bloques, si los había.
func decodificarContenido(raw json.RawMessage) (string, []bloqueMensaje) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "", nil
	}
	switch raw[0] {
	case '"':
		var s string
		_ = json.Unmarshal(raw, &s)
		return s, nil
	case '[':
		var bloques []bloqueMensaje
		if json.Unmarshal(raw, &bloques) != nil {
			return "", nil
		}
		var partes []string
		for _, b := range bloques {
			if b.Type == "text" {
				partes = append(partes, b.Text)
			}
		}
		return strings.Join(partes, "\n"), bloques
	}
	return "", nil
}

// textosDelAdjunto lee el `content` de un adjunto de hook: una lista de strings (así llegaron los
// 2.542 medidos) o, por las dudas, un string suelto.
func textosDelAdjunto(raw json.RawMessage) []string {
	var lista []string
	if json.Unmarshal(raw, &lista) == nil {
		return lista
	}
	var uno string
	if json.Unmarshal(raw, &uno) == nil {
		return []string{uno}
	}
	return nil
}

// sumarBloques cuenta cada encabezado «[Musubi — X]» de un texto.
func sumarBloques(bloques map[string]int, texto string) {
	for _, m := range reBloqueMusubi.FindAllStringSubmatch(texto, -1) {
		bloques[strings.TrimSpace(m[1])]++
	}
}

// esPrompt dice si un registro `user` abre un turno: lo escribió una persona (o el orquestador, en
// un subagente), no es meta y no trae el resultado de una tool.
func esPrompt(reg *registroTranscript, texto string, bloques []bloqueMensaje) bool {
	if reg.IsMeta {
		return false
	}
	if bloques == nil {
		return strings.TrimSpace(texto) != ""
	}
	hayContenido := false
	for _, b := range bloques {
		switch b.Type {
		case "tool_result":
			return false
		case "text", "image":
			hayContenido = true
		}
	}
	return hayContenido
}

// lectorTranscript recorre UN transcript en orden y lleva el estado que hace falta para atribuir
// cada llamada: qué cargó ToolSearch, qué nombró cada hook y qué nombró el prompt del turno.
type lectorTranscript struct {
	m      *MedicionUso
	v      ventanaUso
	skills map[string]bool

	vistos          map[string]bool // uuid de registros ya procesados
	llamadasVistas  map[string]bool // id de tool_use ya contados
	busquedas       map[string]bool // id de ToolSearch esperando su resultado
	cargadas        map[string]bool // nombres completos que un ToolSearch devolvió
	nombradasTurno  map[string]bool // tools que un hook nombró en el turno en curso
	nombradasSesion map[string]bool // tools que algún hook nombró en la sesión
	nombradasPrompt map[string]bool // tools que el prompt del turno nombra

	enVentana, aLaVista, instrucciones, skillsEnListado bool
}

func nuevoLectorTranscript(m *MedicionUso, v ventanaUso, skills map[string]bool) *lectorTranscript {
	return &lectorTranscript{
		m: m, v: v, skills: skills,
		vistos: map[string]bool{}, llamadasVistas: map[string]bool{}, busquedas: map[string]bool{},
		cargadas: map[string]bool{}, nombradasTurno: map[string]bool{}, nombradasSesion: map[string]bool{},
		nombradasPrompt: map[string]bool{},
	}
}

func (l *lectorTranscript) registro(reg *registroTranscript) {
	if reg.UUID != "" {
		if l.vistos[reg.UUID] {
			return // reescrito al reanudar la sesión: ya se contó
		}
		l.vistos[reg.UUID] = true
	}
	cuenta, despues := l.v.ubicar(reg.Timestamp)
	if despues {
		return
	}
	if cuenta {
		l.enVentana = true
	}
	switch reg.Type {
	case "attachment":
		l.adjunto(reg.Attachment, cuenta)
	case "user":
		l.usuario(reg)
	case "assistant":
		l.asistente(reg, cuenta)
	}
}

func (l *lectorTranscript) adjunto(a *adjuntoTranscript, cuenta bool) {
	if a == nil {
		return
	}
	switch a.Type {
	case adjuntoHook:
		for _, texto := range textosDelAdjunto(a.Content) {
			for _, n := range reNombreTool.FindAllString(texto, -1) {
				l.nombradasTurno[n] = true
				l.nombradasSesion[n] = true
			}
			if cuenta {
				sumarBloques(l.m.Bloques, texto)
			}
		}
	case adjuntoInstrucciones:
		for _, n := range a.AddedNames {
			if esServidorMusubi(n) {
				l.instrucciones = true
			}
		}
	case adjuntoDiferidasDelta:
		for _, n := range a.AddedNames {
			if _, ok := toolDeMusubi(n); ok {
				l.aLaVista = true
			}
		}
	case adjuntoDiferidasLista:
		for _, e := range a.Entries {
			if _, ok := toolDeMusubi(e.Name); ok {
				l.aLaVista = true
			}
		}
	case adjuntoListadoSkills:
		for _, n := range a.Names {
			if l.skills[nombreDeSkill(n)] {
				l.skillsEnListado = true
			}
		}
	}
}

func (l *lectorTranscript) usuario(reg *registroTranscript) {
	if reg.Message == nil {
		return
	}
	texto, bloques := decodificarContenido(reg.Message.Content)
	if esPrompt(reg, texto, bloques) {
		// Turno nuevo: lo que un hook nombró en el anterior ya no es «de este turno».
		l.nombradasTurno = map[string]bool{}
		l.nombradasPrompt = map[string]bool{}
		for _, n := range reNombreTool.FindAllString(texto, -1) {
			l.nombradasPrompt[n] = true
		}
		return
	}
	for _, b := range bloques {
		if b.Type != "tool_result" {
			continue
		}
		// El TEXTO de un tool_result no se mira: si trae «[Musubi — …]» es porque el agente leyó
		// código o una salida que lo contiene, no porque un hook le haya hablado. Medido el
		// 2026-09-25: 629 encabezados en 206 tool_result (leer `detect.go`, un grep, un informe),
		// contra 2.846 en adjuntos de hook; contarlos inflaba las inyecciones un 22 %. De un
		// resultado sólo importa si cerró un ToolSearch.
		if !l.busquedas[b.ToolUseID] {
			continue
		}
		delete(l.busquedas, b.ToolUseID)
		for _, n := range toolsCargadas(b.Content, reg.ToolUseResult) {
			l.cargadas[n] = true
		}
	}
}

// toolsCargadas lee qué tools devolvió un ToolSearch. Vienen de dos lados según la versión: bloques
// `tool_reference` en el contenido del resultado y `toolUseResult.matches` en el registro. Se unen.
func toolsCargadas(contenido, resultado json.RawMessage) []string {
	var out []string
	_, bloques := decodificarContenido(contenido)
	for _, b := range bloques {
		if b.Type == "tool_reference" && b.ToolName != "" {
			out = append(out, b.ToolName)
		}
	}
	var r struct {
		Matches []string `json:"matches"`
	}
	if len(resultado) > 0 && json.Unmarshal(resultado, &r) == nil {
		out = append(out, r.Matches...)
	}
	return out
}

func (l *lectorTranscript) asistente(reg *registroTranscript, cuenta bool) {
	if reg.Message == nil {
		return
	}
	_, bloques := decodificarContenido(reg.Message.Content)
	for _, b := range bloques {
		if b.Type != "tool_use" {
			continue
		}
		if b.ID != "" {
			if l.llamadasVistas[b.ID] {
				continue // la misma llamada, escrita otra vez con otro uuid
			}
			l.llamadasVistas[b.ID] = true
		}
		switch b.Name {
		case "ToolSearch":
			l.busquedas[b.ID] = true
		case "Skill":
			if cuenta {
				l.invocarSkill(b.Input)
			}
		default:
			if pelada, ok := toolDeMusubi(b.Name); ok {
				l.aLaVista = true // si la llamó, la veía
				if cuenta {
					l.llamar(b.Name, pelada)
				}
			}
		}
	}
}

func (l *lectorTranscript) invocarSkill(input json.RawMessage) {
	var in struct {
		Skill string `json:"skill"`
	}
	_ = json.Unmarshal(input, &in)
	nombre := strings.TrimSpace(in.Skill)
	if nombre == "" {
		nombre = "(sin nombre)"
	}
	l.m.InvocacionesSkill++
	l.m.PorSkill[nombre]++
	if l.skills[nombreDeSkill(nombre)] {
		l.m.SkillsDeMusubi++
	}
}

func (l *lectorTranscript) llamar(completo, pelada string) {
	l.m.Llamadas++
	l.m.PorTool[pelada]++
	if !l.cargadas[completo] {
		l.m.SinToolSearchPrevio++
	}
	switch {
	case l.nombradasTurno[pelada]:
		l.m.Atribucion.HookMismoTurno++
	case l.nombradasPrompt[pelada]:
		l.m.Atribucion.Prompt++
	case l.nombradasSesion[pelada]:
		l.m.Atribucion.HookAntes++
	default:
		l.m.Atribucion.Nadie++
	}
}

// cerrar vuelca lo que el transcript dejó sobre su alcance. Un transcript sin registros en la
// ventana no existe para la medición: ni suma a «medidos» ni a ninguno de sus porcentajes.
func (l *lectorTranscript) cerrar() {
	if !l.enVentana {
		return
	}
	l.m.Transcripts++
	if l.aLaVista {
		l.m.ConToolsALaVista++
	}
	if l.instrucciones {
		l.m.ConInstrucciones++
	}
	if l.skillsEnListado {
		l.m.ConSkillsEnListado++
	}
}

// esDeSubagente dice si un transcript es de un subagente: vive bajo una carpeta `subagents/`.
func esDeSubagente(rel string) bool {
	for _, parte := range strings.Split(filepath.ToSlash(rel), "/") {
		if parte == "subagents" {
			return true
		}
	}
	return false
}

// carpetaDeProyecto es el nombre de la carpeta donde Claude Code guarda los transcripts de una ruta
// de trabajo: cada carácter que no es letra ni dígito ASCII pasa a `-`. Es un hecho del formato de
// OTRO programa, medido en las 49 carpetas de esta máquina: `/home/davantis/.cache/x` da
// `-home-davantis--cache-x`, y el `_` de `wf_31193ce6` también pasa a `-`.
func carpetaDeProyecto(ruta string) string {
	var b strings.Builder
	for _, r := range ruta {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return b.String()
}

// exclusionesPorDefecto son las carpetas de proyecto de la carpeta TEMPORAL del sistema: la de
// `os.TempDir()` y todas las que cuelgan de ella.
//
// UNA SESIÓN QUE TRABAJA EN UNA CARPETA TEMPORAL ES UN EXPERIMENTO, no trabajo: una prueba de
// conducta con `claude -p` en un mktemp, que por diseño arranca vacía. Medida junto a las reales las
// contamina, y en la dirección que más engaña: el 2026-09-25, las invocaciones de skills y las
// llamadas sin ToolSearch que el medidor contaba venían TODAS de carpetas de experimento, y en los
// proyectos reales eran cero.
//
// SE DERIVA DE `os.TempDir()` y no se escribe `-tmp-*`: en Windows la temporal es
// `C:\Users\…\Temp` y en macOS cuelga de `/var/folders`, y un patrón clavado al Linux de hoy no
// excluiría nada allá. Se prueba también la ruta con los enlaces resueltos, porque la sesión guarda
// la carpeta como la ve el proceso: en macOS `/var` es un enlace a `/private/var`.
//
// Los experimentos que corren FUERA de la temporal no se pueden reconocer desde acá: van con
// `--excluir`.
func exclusionesPorDefecto() []string {
	var out []string
	visto := map[string]bool{}
	tmp := filepath.Clean(os.TempDir())
	rutas := []string{tmp}
	if real, err := filepath.EvalSymlinks(tmp); err == nil {
		rutas = append(rutas, real)
	}
	for _, r := range rutas {
		base := carpetaDeProyecto(r)
		for _, patron := range []string{base, base + "-*"} {
			if !visto[patron] {
				visto[patron] = true
				out = append(out, patron)
			}
		}
	}
	return out
}

// excluida dice si una carpeta de proyecto cae en alguna exclusión. Los patrones ya se validaron al
// leer los argumentos: acá un error de patrón no puede pasar, y si pasara no excluye.
func excluida(nombre string, exclusiones []string) bool {
	for _, patron := range exclusiones {
		if ok, _ := path.Match(patron, nombre); ok {
			return true
		}
	}
	return false
}

// listaDePatrones es un flag que se puede repetir: `--excluir a --excluir b`.
type listaDePatrones []string

func (l *listaDePatrones) String() string { return strings.Join(*l, ",") }

func (l *listaDePatrones) Set(v string) error {
	if _, err := path.Match(v, ""); err != nil {
		return fmt.Errorf("%q no es un patrón válido: %v", v, err)
	}
	*l = append(*l, v)
	return nil
}

// medirUsoAgente camina `dir`, lee cada transcript y arma el informe. No escribe nada.
//
// `exclusiones` se comparan contra el nombre de cada carpeta de proyecto —las hijas DIRECTAS de
// `dir`—, y una que cae se saltea entera y se cuenta. Sólo las hijas directas: más abajo están las
// carpetas de cada sesión, que se llaman con un uuid, y un patrón pensado para proyectos no tiene
// nada que decir de ellas.
func medirUsoAgente(dir string, v ventanaUso, exclusiones []string) (InformeUso, error) {
	inf := InformeUso{Dir: dir, SkillsDeMusubi: skillsDeMusubi(), Exclusiones: exclusiones}
	if inf.Exclusiones == nil {
		inf.Exclusiones = []string{}
	}
	if v.hayDesde {
		inf.Desde = v.desde.Format("2006-01-02")
	}
	if v.hayHasta {
		inf.Hasta = v.hasta.AddDate(0, 0, -1).Format("2006-01-02")
	}
	skills := map[string]bool{}
	for _, s := range inf.SkillsDeMusubi {
		skills[s] = true
	}
	principal, subagentes := nuevaMedicionUso(), nuevaMedicionUso()

	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return inf, fmt.Errorf("no puedo leer la carpeta de transcripts %s: %v", dir, err)
	}
	err := filepath.WalkDir(dir, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// La carpeta pedida no se excluye a sí misma. Con `--dir .` su nombre es «.», que un
			// `--excluir '*'` alcanza, y el informe diría que no hubo nada que medir.
			if ruta == dir {
				return nil
			}
			if filepath.Dir(ruta) == filepath.Clean(dir) && excluida(d.Name(), exclusiones) {
				inf.CarpetasExcluidas++
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if d.Name() == journalDeWorkflow {
			inf.JournalExcluidos++
			return nil
		}
		inf.Archivos++
		// UN ARCHIVO QUE NO CAMBIÓ DESDE ANTES DE LA VENTANA NO PUEDE TENER REGISTROS ADENTRO DE ELLA:
		// cada registro se escribe en o después de su timestamp, y una reescritura al reanudar
		// también mueve el mtime. Saltearlo no cambia el resultado y ahorra leer gigas.
		if v.hayDesde {
			if info, ierr := d.Info(); ierr == nil && info.ModTime().Before(v.desde) {
				inf.SalteadosPorFecha++
				return nil
			}
		}
		rel, _ := filepath.Rel(dir, ruta)
		m := principal
		if esDeSubagente(rel) {
			m = subagentes
		}
		ilegibles, lerr := leerTranscript(ruta, nuevoLectorTranscript(m, v, skills))
		inf.LineasIlegibles += ilegibles
		return lerr
	})
	if err != nil {
		return inf, err
	}
	inf.Principal = alcanceUso(principal, "sesión principal")
	inf.Subagentes = alcanceUso(subagentes, "subagente")
	return inf, nil
}

// leerTranscript pasa cada línea de un archivo por el lector. Devuelve cuántas no eran JSON: la
// última línea de un transcript que se está escribiendo puede estar cortada, y eso se CUENTA y se
// informa en vez de tirarlo en silencio.
func leerTranscript(ruta string, l *lectorTranscript) (int, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	ilegibles := 0
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		// ReadBytes y no un Scanner: hay líneas de decenas de MB (un tool_result con un archivo
		// entero) y el Scanner corta en 64 KB por defecto.
		linea, rerr := r.ReadBytes('\n')
		if len(bytes.TrimSpace(linea)) > 0 {
			var reg registroTranscript
			if json.Unmarshal(linea, &reg) != nil {
				ilegibles++
			} else {
				l.registro(&reg)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return ilegibles, fmt.Errorf("leer %s: %w", ruta, rerr)
		}
	}
	l.cerrar()
	return ilegibles, nil
}

// dirTranscriptsPorDefecto es donde Claude Code guarda los transcripts: `$CLAUDE_CONFIG_DIR/projects`
// si la variable está puesta (así lo reubica Claude Code), y si no `~/.claude/projects`.
func dirTranscriptsPorDefecto() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "projects"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

func runUsoAgente(args []string) {
	os.Exit(usoAgente(args, os.Stdout, os.Stderr))
}

// usoAgente es el comando entero con sus salidas inyectadas, para probarlo sin procesos. Devuelve
// el código de salida: 0 medido o «sin medir» (las dos son respuestas), 1 si no pudo leer, 2 si los
// argumentos no sirven.
func usoAgente(args []string, out, errOut io.Writer) int {
	fl := flag.NewFlagSet("uso-agente", flag.ContinueOnError)
	fl.SetOutput(errOut)
	desde := fl.String("desde", "", "primer día a medir, AAAA-MM-DD (UTC, inclusive)")
	hasta := fl.String("hasta", "", "último día a medir, AAAA-MM-DD (UTC, inclusive)")
	dir := fl.String("dir", "", "carpeta de transcripts (default: ~/.claude/projects)")
	comoJSON := fl.Bool("json", false, "emitir el informe como JSON")
	var excluir listaDePatrones
	fl.Var(&excluir, "excluir", "patrón (glob) de carpetas de proyecto a no medir; se puede repetir")
	conTemporales := fl.Bool("incluir-temporales", false,
		"medir también las carpetas de la temporal del sistema, que por defecto se excluyen (son experimentos)")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	if fl.NArg() > 0 {
		fmt.Fprintf(errOut, "musubi uso-agente: argumento de más: %q\n", fl.Arg(0))
		return 2
	}
	v, err := parsearVentanaUso(*desde, *hasta)
	if err != nil {
		fmt.Fprintln(errOut, "musubi uso-agente:", err)
		return 2
	}
	if *dir == "" {
		if *dir, err = dirTranscriptsPorDefecto(); err != nil {
			fmt.Fprintln(errOut, "musubi uso-agente: no sé dónde están los transcripts:", err)
			return 1
		}
	}
	exclusiones := []string(excluir)
	if !*conTemporales {
		exclusiones = append(exclusionesPorDefecto(), exclusiones...)
	}
	inf, err := medirUsoAgente(*dir, v, exclusiones)
	if err != nil {
		fmt.Fprintf(errOut, "musubi uso-agente: %v — sin medir\n", err)
		return 1
	}
	if *comoJSON {
		b, err := json.MarshalIndent(inf, "", "  ")
		if err != nil {
			fmt.Fprintln(errOut, "musubi uso-agente: no pude serializar el informe:", err)
			return 1
		}
		fmt.Fprintln(out, string(b))
		return 0
	}
	imprimirUsoAgente(out, inf)
	return 0
}

// imprimirUsoAgente escribe la tabla legible.
func imprimirUsoAgente(w io.Writer, inf InformeUso) {
	fmt.Fprintln(w, "Uso de Musubi por el agente — lo que CONSUMIÓ, leído de los transcripts de Claude Code")
	fmt.Fprintf(w, "  carpeta  : %s\n", inf.Dir)
	desde, hasta := inf.Desde, inf.Hasta
	if desde == "" {
		desde = "el principio"
	}
	if hasta == "" {
		hasta = "hoy"
	}
	fmt.Fprintf(w, "  ventana  : %s → %s (días UTC, por el timestamp de cada registro)\n", desde, hasta)
	fmt.Fprintf(w, "  archivos : %d .jsonl · %d journal.jsonl excluidos (no son transcripts) · "+
		"%d salteados (sin cambios desde antes de la ventana) · %d línea(s) ilegible(s)\n",
		inf.Archivos, inf.JournalExcluidos, inf.SalteadosPorFecha, inf.LineasIlegibles)
	if len(inf.Exclusiones) == 0 {
		fmt.Fprintf(w, "  excluidas: ninguna carpeta (se midió todo lo que hay en la carpeta)\n")
	} else {
		fmt.Fprintf(w, "  excluidas: %d carpeta(s) de proyecto por %s (--incluir-temporales mide las temporales)\n",
			inf.CarpetasExcluidas, strings.Join(inf.Exclusiones, " "))
	}
	fmt.Fprintf(w, "  skills de Musubi (derivadas de cognitive.go): %s\n", strings.Join(inf.SkillsDeMusubi, ", "))
	imprimirAlcanceUso(w, "Sesiones PRINCIPALES", inf.Principal)
	imprimirAlcanceUso(w, "SUBAGENTES", inf.Subagentes)
}

func imprimirAlcanceUso(w io.Writer, titulo string, a AlcanceUso) {
	fmt.Fprintf(w, "\n── %s\n", titulo)
	if a.MedicionUso == nil {
		fmt.Fprintf(w, "  %s — %s\n", a.Estado, a.Motivo)
		return
	}
	m := a.MedicionUso
	fila := func(etiqueta string, n int, nota string) {
		fmt.Fprintf(w, "  %-46s %6d  %s\n", etiqueta, n, nota)
	}
	fila("1. transcripts medidos", m.Transcripts, "")
	fila("   con tools de Musubi a la vista", m.ConToolsALaVista, "(anunciadas como diferidas, o llamadas)")
	fila("2. recibieron las instrucciones de Musubi", m.ConInstrucciones, "(adjunto de instrucciones MCP)")
	fila("3. llamadas a tools de Musubi", m.Llamadas, "(deduplicadas por uuid y por id de tool_use)")
	fila("   a. sin un ToolSearch previo que la cargara", m.SinToolSearchPrevio, "(llegó cargada)")
	fila("   b. un hook la nombró en el mismo turno", m.Atribucion.HookMismoTurno, porciento(m.Atribucion.HookMismoTurno, m.Llamadas))
	fila("      el prompt del turno la nombraba", m.Atribucion.Prompt, porciento(m.Atribucion.Prompt, m.Llamadas))
	fila("      un hook la nombró antes en la sesión", m.Atribucion.HookAntes, porciento(m.Atribucion.HookAntes, m.Llamadas))
	fila("      nadie la nombró", m.Atribucion.Nadie, porciento(m.Atribucion.Nadie, m.Llamadas))
	if len(m.PorTool) > 0 {
		fmt.Fprintf(w, "      las más llamadas: %s\n", masFrecuentes(m.PorTool, 8))
	}
	fila("4. invocaciones de la tool Skill", m.InvocacionesSkill, "")
	fila("   de skills de Musubi", m.SkillsDeMusubi, fmt.Sprintf("(skills de Musubi en el listado de %d transcript(s))", m.ConSkillsEnListado))
	if len(m.PorSkill) > 0 {
		fmt.Fprintf(w, "      por skill: %s\n", masFrecuentes(m.PorSkill, 0))
	}
	total := 0
	for _, n := range m.Bloques {
		total += n
	}
	fila("5. bloques «[Musubi — X]» inyectados por hook", total, "(adjuntos de hook; el texto de un tool_result no cuenta)")
	for _, par := range ordenarPorCuenta(m.Bloques) {
		fmt.Fprintf(w, "        %-44s %6d\n", par.clave, par.n)
	}
}

func porciento(n, total int) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("(%.1f %%)", 100*float64(n)/float64(total))
}

type claveCuenta struct {
	clave string
	n     int
}

// ordenarPorCuenta ordena de mayor a menor y, a igual cuenta, por nombre: la salida no depende del
// orden de un mapa.
func ordenarPorCuenta(m map[string]int) []claveCuenta {
	out := make([]claveCuenta, 0, len(m))
	for k, n := range m {
		out = append(out, claveCuenta{k, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].clave < out[j].clave
	})
	return out
}

// masFrecuentes resume un mapa en una línea; `tope` 0 es sin tope.
func masFrecuentes(m map[string]int, tope int) string {
	pares := ordenarPorCuenta(m)
	if tope > 0 && len(pares) > tope {
		pares = pares[:tope]
	}
	partes := make([]string, 0, len(pares))
	for _, p := range pares {
		partes = append(partes, fmt.Sprintf("%s %d", p.clave, p.n))
	}
	return strings.Join(partes, " · ")
}
