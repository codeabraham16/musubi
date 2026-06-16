// Package bootstrap implementa la inyección de Musubi en un proyecto.
// Este archivo gestiona la inyección de hooks de ciclo de vida (SessionStart,
// UserPromptSubmit, …) en .claude/settings.json.
package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// HookCommand describe un hook tipo command de Claude Code.
type HookCommand struct {
	Type    string `json:"type"` // "command"
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // segundos; 0 = sin límite explícito
}

// hookEntry representa una entrada en el array de un evento de hooks (ej.
// hooks.SessionStart). Cada entrada tiene un matcher opcional y una lista de hooks.
type hookEntry struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookCommand `json:"hooks"`
}

// MergeClaudeSettings inserta (de forma idempotente) un hook del evento dado
// (ej. "SessionStart", "UserPromptSubmit") en el contenido de un
// .claude/settings.json. Preserva otros eventos, hooks, matchers y claves de
// nivel superior. matcher define cuándo dispara (ej. "startup"; "" para eventos
// sin matcher como UserPromptSubmit). No duplica el hook de Musubi si su Command
// ya está presente en alguna entrada del mismo evento.
func MergeClaudeSettings(existing []byte, event, matcher string, hook HookCommand) ([]byte, error) {
	// Paso 1: parsear el root en un mapa de RawMessage para preservar claves desconocidas.
	root := map[string]json.RawMessage{}
	if len(bytes.TrimSpace(existing)) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("error al parsear settings.json existente: %w", err)
		}
	}

	// Paso 2: extraer el mapa de hooks de nivel superior.
	hooksMap := map[string]json.RawMessage{}
	if raw, ok := root["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooksMap); err != nil {
			return nil, fmt.Errorf("error al parsear hooks: %w", err)
		}
	}

	// Paso 3: extraer el array del evento.
	var entries []hookEntry
	if raw, ok := hooksMap[event]; ok {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, fmt.Errorf("error al parsear hooks.%s: %w", event, err)
		}
	}

	// Paso 4: dedup por FIRMA (el subcomando, ignorando la ruta del ejecutable).
	// - Si ya existe un hook con la misma firma y Command idéntico -> idempotente.
	// - Si existe con la misma firma pero distinta ruta -> reemplazar (evita dejar
	//   un hook duplicado apuntando a un binario viejo al re-instalar/mover).
	sig := hookSignature(hook.Command)
	for i := range entries {
		for j := range entries[i].Hooks {
			if hookSignature(entries[i].Hooks[j].Command) != sig {
				continue
			}
			if entries[i].Hooks[j].Command == hook.Command {
				return existing, nil // idéntico: nada que hacer
			}
			entries[i].Hooks[j] = hook // misma firma, otra ruta: reemplazar
			return reserialize(root, hooksMap, event, entries)
		}
	}

	// Paso 5: buscar una entrada con el mismo matcher y agregar el hook a ella.
	// Si no existe, crear una nueva entrada.
	encontrado := false
	for i, entrada := range entries {
		if entrada.Matcher == matcher {
			entries[i].Hooks = append(entries[i].Hooks, hook)
			encontrado = true
			break
		}
	}
	if !encontrado {
		entries = append(entries, hookEntry{
			Matcher: matcher,
			Hooks:   []HookCommand{hook},
		})
	}

	// Paso 6: re-serializar evento → hooksMap → root.
	return reserialize(root, hooksMap, event, entries)
}

// reserialize vuelca las entradas del evento al hooksMap y al root, devolviendo el
// settings.json indentado.
func reserialize(root, hooksMap map[string]json.RawMessage, event string, entries []hookEntry) ([]byte, error) {
	evBytes, err := json.Marshal(entries)
	if err != nil {
		return nil, fmt.Errorf("error al serializar hooks.%s: %w", event, err)
	}
	hooksMap[event] = evBytes

	hooksBytes, err := json.Marshal(hooksMap)
	if err != nil {
		return nil, fmt.Errorf("error al serializar hooks: %w", err)
	}
	root["hooks"] = hooksBytes

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error al serializar settings.json: %w", err)
	}
	return out, nil
}

// hookSignature devuelve la parte estable de un Command de hook: el subcomando tras
// la ruta del ejecutable (citada o no). Permite deduplicar/reemplazar el hook de
// Musubi aunque cambie la ruta del binario.
func hookSignature(cmd string) string {
	s := strings.TrimSpace(cmd)
	if strings.HasPrefix(s, "\"") {
		if i := strings.Index(s[1:], "\""); i >= 0 {
			return strings.TrimSpace(s[i+2:]) // tras la comilla de cierre
		}
		return s
	}
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}
