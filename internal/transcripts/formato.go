// Package transcripts lee, en SÓLO LECTURA, los transcripts de Claude Code (`~/.claude/projects`).
//
// ES EL ÚNICO LECTOR DEL REPO Y EL ÚNICO CLASIFICADOR DE PROMPTS. Lo usan `musubi uso-agente` (qué
// tools y skills de Musubi consume el agente), `musubi uso-agente --contexto` (si el contexto se
// repite, se pierde o descarrila) y, detrás, el hook del turno, el banco de búsqueda con forma de
// prompt y la métrica de proyecto. Antes de este paquete el lector vivía adentro de `cmd/musubi`,
// y cada consumidor nuevo iba a copiarse su propio criterio de «qué es un prompt humano»: dos
// criterios que miden lo mismo terminan dando dos números.
//
// EL FORMATO DE LOS TRANSCRIPTS NO ESTÁ DOCUMENTADO, y lo que sigue se midió leyendo los `.jsonl`
// de esta máquina (2026-09-25 y 2026-09-26, Claude Code 2.1.2xx):
//
//   - Cada línea es un registro con `type`, `uuid`, `timestamp` (siempre UTC con `Z`) y, según el
//     tipo, `message` o `attachment`.
//   - LOS REGISTROS SE REESCRIBEN AL REANUDAR UNA SESIÓN: 50.285 uuid repetidos DENTRO del mismo
//     archivo, ninguno entre archivos distintos (0 de 451.748). Sin deduplicar por `uuid`, cada
//     reanudación vuelve a contar el pasado entero. Los repetidos llegan en tramos seguidos (23
//     tramos en la sesión que más se reanudó), y cada tramo es una reanudación.
//   - Lo que inyecta un hook llega como `attachment` de tipo `hook_additional_context`, con el
//     evento en `hookEvent` y los bloques «[Musubi — X]» adentro de `content`. Llega DESPUÉS del
//     prompt que lo disparó. El mismo texto puede aparecer DENTRO de un `tool_result` —por
//     ejemplo, cuando el agente lee `detect.go`— y eso NO es una inyección: el agente leyó código,
//     nadie le habló.
//   - UN PROMPT NO SIEMPRE ES UN REGISTRO `user`. Lo que llega mientras el agente trabaja —el dueño
//     escribe, o termina una tarea de fondo— se entrega adentro del turno como un adjunto
//     `queued_command`, con el texto en `prompt`, y el hook del turno corre igual. Medido el
//     2026-09-26 desde el 09-14: 35 pedidos y 117 avisos llegaron así, 68 dispararon el hook, y
//     sólo 5 aparecen además como registro `user`. Sin contarlos como prompts, la memoria que
//     trajeron se le anota al turno anterior: eran los 45 turnos con dos o más bloques de memoria.
//   - Una compactación deja un registro `system` con `subtype` `compact_boundary`; después viene
//     el resumen (`isCompactSummary`), el `<command-name>/compact` y su `<local-command-stdout>`.
//   - Las carpetas de proyecto empiezan con `-` (`-home-davantis-musubi`), así que un glob
//     `*/*.jsonl` da cero. Se camina el árbol.
//   - Los subagentes viven bajo una carpeta `subagents/`; los de un workflow, además, en
//     `subagents/workflows/wf_*/`, junto a un `journal.jsonl` que es la bitácora del workflow y NO
//     un transcript (112 medidos): se excluye por nombre.
//   - Cada carpeta de proyecto es la ruta de trabajo con todo carácter que no sea letra o dígito
//     ASCII cambiado por `-` (`/home/davantis/.cache/x` → `-home-davantis--cache-x`). Por eso las
//     carpetas de experimento se reconocen por el nombre: ver ExclusionesPorDefecto.
package transcripts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Tipos de registro y de adjunto de Claude Code que este paquete lee. Son strings del formato de
// OTRO programa y por eso van con nombre: si Claude Code los renombra, se cambian acá y en ningún
// otro lado.
const (
	AdjuntoHook           = "hook_additional_context"
	AdjuntoInstrucciones  = "mcp_instructions_delta"
	AdjuntoDiferidasDelta = "deferred_tools_delta"
	AdjuntoDiferidasLista = "deferred_tools_record"
	AdjuntoListadoSkills  = "skill_listing"
	AdjuntoEncolado       = "queued_command"

	// SubtipoCompactacion marca, en un registro `system`, el borde de una compactación.
	SubtipoCompactacion = "compact_boundary"

	// JournalDeWorkflow es la bitácora de un workflow: vive entre los transcripts y no es uno.
	JournalDeWorkflow = "journal.jsonl"
)

// Registro es una línea de un transcript, con sólo los campos que alguien de este repo lee.
// `content` y `toolUseResult` quedan crudos porque su forma depende del tipo de registro.
type Registro struct {
	Type             string          `json:"type"`
	Subtype          string          `json:"subtype"`
	UUID             string          `json:"uuid"`
	Timestamp        string          `json:"timestamp"`
	SessionID        string          `json:"sessionId"`
	IsMeta           bool            `json:"isMeta"`
	IsSidechain      bool            `json:"isSidechain"`
	IsCompactSummary bool            `json:"isCompactSummary"`
	Message          *Mensaje        `json:"message"`
	Attachment       *Adjunto        `json:"attachment"`
	ToolUseResult    json.RawMessage `json:"toolUseResult"`
}

// Mensaje es el `message` de un registro `user` o `assistant`.
type Mensaje struct {
	Content json.RawMessage `json:"content"`
}

// Bloque es un elemento de `message.content`: texto, tool_use, tool_result o, adentro del
// resultado de un ToolSearch, un tool_reference.
type Bloque struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	ToolName  string          `json:"tool_name"`
}

// Adjunto es el `attachment` de un registro `attachment`.
type Adjunto struct {
	Type       string          `json:"type"`
	HookEvent  string          `json:"hookEvent"`
	Content    json.RawMessage `json:"content"`
	AddedNames []string        `json:"addedNames"`
	Names      []string        `json:"names"`
	Entries    []struct {
		Name string `json:"name"`
	} `json:"entries"`
	// Prompt e IsMeta son de un `queued_command`: el prompt que llegó mientras el agente trabajaba,
	// como string o como lista de bloques, igual que el `content` de un registro `user`.
	Prompt json.RawMessage `json:"prompt"`
	IsMeta bool            `json:"isMeta"`
}

// DecodificarContenido lee un `content` que puede ser un string o una lista de bloques —las dos
// formas aparecen, en el prompt y en un tool_result—. Devuelve el texto (los bloques `text` unidos)
// y los bloques, si los había.
func DecodificarContenido(raw json.RawMessage) (string, []Bloque) {
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
		var bloques []Bloque
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

// TextosDelAdjunto lee el `content` de un adjunto de hook: una lista de strings (así llegaron los
// 2.542 medidos) o, por las dudas, un string suelto.
func TextosDelAdjunto(raw json.RawMessage) []string {
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

// EsPrompt dice si un registro `user` abre un turno: lo escribió una persona (o el orquestador, en
// un subagente, o Claude Code mismo), no es meta y no trae el resultado de una tool. QUIÉN lo
// escribió lo dice ClasificarPrompt.
func EsPrompt(reg *Registro, texto string, bloques []Bloque) bool {
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

// Lectura es lo que la pasada por un archivo dejó además de los registros.
type Lectura struct {
	// Ilegibles son las líneas que no eran JSON: la última línea de un transcript que se está
	// escribiendo puede estar cortada, y eso se CUENTA y se informa en vez de tirarlo en silencio.
	Ilegibles int
	// Reanudaciones tiene, por cada tramo de registros reescritos, el timestamp del primer registro
	// NUEVO que vino después: el momento en que la sesión siguió. Un tramo reescrito al final del
	// archivo, sin nada nuevo detrás, no deja fecha y no se anota.
	Reanudaciones []string
}

// Leer pasa cada registro de un transcript a fn, en orden y UNA SOLA VEZ por uuid: lo que la
// reanudación reescribe ya se leyó. Un registro sin uuid pasa siempre.
func Leer(ruta string, fn func(*Registro)) (Lectura, error) {
	var lec Lectura
	f, err := os.Open(ruta)
	if err != nil {
		return lec, err
	}
	defer f.Close()
	vistos := map[string]bool{}
	enReescritura := false
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		// ReadBytes y no un Scanner: hay líneas de decenas de MB (un tool_result con un archivo
		// entero) y el Scanner corta en 64 KB por defecto.
		linea, rerr := r.ReadBytes('\n')
		if len(bytes.TrimSpace(linea)) > 0 {
			var reg Registro
			if json.Unmarshal(linea, &reg) != nil {
				lec.Ilegibles++
			} else if reg.UUID != "" && vistos[reg.UUID] {
				enReescritura = true // reescrito al reanudar la sesión: ya se leyó
			} else {
				if reg.UUID != "" {
					vistos[reg.UUID] = true
					if enReescritura && reg.Timestamp != "" {
						lec.Reanudaciones = append(lec.Reanudaciones, reg.Timestamp)
						enReescritura = false
					}
				}
				fn(&reg)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return lec, fmt.Errorf("leer %s: %w", ruta, rerr)
		}
	}
	return lec, nil
}
