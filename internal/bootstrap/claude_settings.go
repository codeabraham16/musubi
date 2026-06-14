// Package bootstrap implementa la inyección de Musubi en un proyecto.
// Este archivo gestiona la inyección de hooks SessionStart en .claude/settings.json.
package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// HookCommand describe un hook tipo command de Claude Code.
type HookCommand struct {
	Type    string `json:"type"`              // "command"
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // segundos; 0 = sin límite explícito
}

// sessionStartEntry representa una entrada en el array hooks.SessionStart.
// Cada entrada tiene un matcher opcional y una lista de hooks.
type sessionStartEntry struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookCommand `json:"hooks"`
}

// MergeClaudeSettings inserta (de forma idempotente) un hook SessionStart en el
// contenido de un .claude/settings.json. Preserva otros hooks, matchers y claves
// de nivel superior. matcher define cuándo dispara (ej. "startup"). No duplica
// el hook de Musubi si su Command ya está presente en alguna entrada existente.
func MergeClaudeSettings(existing []byte, matcher string, hook HookCommand) ([]byte, error) {
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

	// Paso 3: extraer el array SessionStart.
	var sessionStart []sessionStartEntry
	if raw, ok := hooksMap["SessionStart"]; ok {
		if err := json.Unmarshal(raw, &sessionStart); err != nil {
			return nil, fmt.Errorf("error al parsear hooks.SessionStart: %w", err)
		}
	}

	// Paso 4: idempotencia — si el Command ya está presente en cualquier entrada,
	// devolver el contenido existente sin modificar.
	for _, entrada := range sessionStart {
		for _, h := range entrada.Hooks {
			if h.Command == hook.Command {
				// El hook ya existe; no duplicar.
				return existing, nil
			}
		}
	}

	// Paso 5: buscar una entrada con el mismo matcher y agregar el hook a ella.
	// Si no existe, crear una nueva entrada.
	encontrado := false
	for i, entrada := range sessionStart {
		if entrada.Matcher == matcher {
			sessionStart[i].Hooks = append(sessionStart[i].Hooks, hook)
			encontrado = true
			break
		}
	}
	if !encontrado {
		sessionStart = append(sessionStart, sessionStartEntry{
			Matcher: matcher,
			Hooks:   []HookCommand{hook},
		})
	}

	// Paso 6: re-serializar SessionStart → hooksMap → root.
	ssBytes, err := json.Marshal(sessionStart)
	if err != nil {
		return nil, fmt.Errorf("error al serializar SessionStart: %w", err)
	}
	hooksMap["SessionStart"] = ssBytes

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
