package transcripts

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Eventos de hook de Claude Code que se nombran en más de un lugar.
const (
	EventoTurno    = "UserPromptSubmit"
	EventoArranque = "SessionStart"
)

// PrefijoDeCorreccion es el comienzo de la línea con que el hook del turno avisa que corrigió un
// tipeo de la consulta («busqué «información» por «infromacion»»). Es un CONTRATO con el
// corrector del frente búsqueda: esa línea repite a propósito un pedazo del prompt, y el detector
// de eco (`uso-agente --contexto`, M7) la saltea. Si el corrector cambia la redacción, se cambia
// acá y el medidor la sigue.
const PrefijoDeCorreccion = "busqué «"

// reBloqueMusubi encuentra el encabezado de cada bloque que un hook de Musubi inyecta.
var reBloqueMusubi = regexp.MustCompile(`\[Musubi — ([^\]]+)\]`)

// reIDMemoria encuentra los ids de memoria que un bloque le ofrece al agente (`[id:abc]`).
var reIDMemoria = regexp.MustCompile(`\[id:([^\]\s]+)\]`)

// BloquesDeMusubi devuelve el nombre de cada bloque «[Musubi — X]» de un texto, en orden.
func BloquesDeMusubi(texto string) []string {
	var out []string
	for _, m := range reBloqueMusubi.FindAllStringSubmatch(texto, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// IDsDeMemoria devuelve los ids de memoria de un texto, en orden y con repetidos.
func IDsDeMemoria(texto string) []string {
	var out []string
	for _, m := range reIDMemoria.FindAllStringSubmatch(texto, -1) {
		out = append(out, m[1])
	}
	return out
}

// Inyeccion es lo que un hook agregó al contexto (un adjunto `hook_additional_context`).
type Inyeccion struct {
	Evento    string
	Timestamp string
	Texto     string // los textos del adjunto, unidos con salto de línea
}

// DeMusubi dice si la inyección trae algún bloque de Musubi.
func (in Inyeccion) DeMusubi() bool { return reBloqueMusubi.MatchString(in.Texto) }

// Tiene dice si la inyección trae el bloque «[Musubi — nombre]».
func (in Inyeccion) Tiene(nombre string) bool {
	return strings.Contains(in.Texto, "[Musubi — "+nombre+"]")
}

// Llamada es un tool_use del asistente, contado una vez por su id.
type Llamada struct {
	ID        string
	Nombre    string
	Timestamp string
	// Entrada se guarda SÓLO para las tools MCP (`mcp__…`): las de Write o Edit traen archivos
	// enteros, y una sesión larga tiene miles.
	Entrada json.RawMessage
}

// Turno es un tramo del hilo de una sesión: lo abre un prompt —o el principio del archivo, o una
// compactación— y dura hasta el próximo. Lo que un hook inyectó y las tools que el agente llamó
// en el medio son de ese turno.
type Turno struct {
	Origen    Origen
	Prompt    string // "" en un tramo de apertura
	Timestamp string // el del registro que abrió el tramo
	// Ventana cuenta las ventanas de contexto: 0 al abrir el archivo y +1 en cada compactación.
	// Todo lo de una ventana convive en el contexto del agente; lo de antes lo tapó un resumen.
	Ventana int
	// Compactacion dice que este tramo lo abrió un `compact_boundary`.
	Compactacion bool
	// Encolado dice que el prompt llegó mientras el agente trabajaba (un `queued_command`).
	Encolado bool

	Inyecciones []Inyeccion
	Llamadas    []Llamada
}

// Sesion es un transcript leído por turnos.
type Sesion struct {
	Turnos []Turno
	Lectura
}

// LeerSesion lee un transcript entero por turnos, con lo reescrito leído una sola vez.
func LeerSesion(ruta string) (Sesion, error) {
	var s Sesion
	llamadas := map[string]bool{}
	abrir := func(t Turno) {
		if n := len(s.Turnos); n > 0 {
			t.Ventana = s.Turnos[n-1].Ventana
		}
		if t.Compactacion {
			t.Ventana++
		}
		s.Turnos = append(s.Turnos, t)
	}
	actual := func(ts string) *Turno {
		if len(s.Turnos) == 0 {
			abrir(Turno{Origen: OrigenApertura, Timestamp: ts})
		}
		return &s.Turnos[len(s.Turnos)-1]
	}
	lec, err := Leer(ruta, func(reg *Registro) {
		switch reg.Type {
		case "system":
			if reg.Subtype == SubtipoCompactacion {
				abrir(Turno{Origen: OrigenApertura, Timestamp: reg.Timestamp, Compactacion: true})
			}
		case "attachment":
			a := reg.Attachment
			if a == nil {
				return
			}
			switch a.Type {
			case AdjuntoEncolado:
				// Un prompt que llegó con el agente trabajando: abre su propio turno, y el hook
				// que dispara es de él, no del turno que estaba en curso.
				texto, bloques := DecodificarContenido(a.Prompt)
				pseudo := &Registro{IsMeta: a.IsMeta}
				if EsPrompt(pseudo, texto, bloques) {
					abrir(Turno{Origen: ClasificarPrompt(pseudo, texto), Prompt: texto, Timestamp: reg.Timestamp,
						Encolado: true})
				}
			case AdjuntoHook:
				t := actual(reg.Timestamp)
				t.Inyecciones = append(t.Inyecciones, Inyeccion{Evento: a.HookEvent, Timestamp: reg.Timestamp,
					Texto: strings.Join(TextosDelAdjunto(a.Content), "\n")})
			}
		case "user":
			if reg.Message == nil {
				return
			}
			texto, bloques := DecodificarContenido(reg.Message.Content)
			if EsPrompt(reg, texto, bloques) {
				abrir(Turno{Origen: ClasificarPrompt(reg, texto), Prompt: texto, Timestamp: reg.Timestamp})
			}
		case "assistant":
			if reg.Message == nil {
				return
			}
			_, bloques := DecodificarContenido(reg.Message.Content)
			for _, b := range bloques {
				if b.Type != "tool_use" {
					continue
				}
				if b.ID != "" {
					if llamadas[b.ID] {
						continue // la misma llamada, escrita otra vez con otro uuid
					}
					llamadas[b.ID] = true
				}
				ll := Llamada{ID: b.ID, Nombre: b.Name, Timestamp: reg.Timestamp}
				if strings.HasPrefix(b.Name, "mcp__") {
					ll.Entrada = b.Input
				}
				t := actual(reg.Timestamp)
				t.Llamadas = append(t.Llamadas, ll)
			}
		}
	})
	s.Lectura = lec
	return s, err
}
