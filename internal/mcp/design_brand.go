package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// design_brand.go — Musubi Renaissance · CAPA 3 (marca por proyecto), lado ESTRUCTURADO (F2).
//
// Una marca puede guardarse en la obs 'diseno/marca' de dos formas: como PROSA (reglas de identidad, sin
// valores) o como un DOC JSON con los tokens concretos por rol. Cuando hay tokens, el `emit` del brief
// sale RELLENO (los hex/tamaños reales de ESA marca), no sólo con los nombres — así el agente compone con
// los valores correctos y no inventa. El vocabulario de roles (bg/ink/accent/…) es UNIVERSAL (CAPA 2);
// sólo los VALORES cambian por marca (CAPA 3). Model-free: es un resolvedor determinista, sin LLM.

// brandTokens es la forma estructurada de una marca. `Palette` mapea ROL → hex; los roles son el
// vocabulario semántico universal, no colores literales.
type brandTokens struct {
	Name      string            `json:"name,omitempty"`
	Palette   map[string]string `json:"palette,omitempty"` // bg/surface/raised/line/ink/muted/faint/accent/accent2/ok/warn/danger → hex
	Radius    map[string]int    `json:"radius,omitempty"`  // surface / pill
	Type      *brandType        `json:"type,omitempty"`
	Elevation string            `json:"elevation,omitempty"` // flat | raised
	Identity  string            `json:"identity,omitempty"`  // reglas de identidad no-color (prosa)
}

type brandType struct {
	Scale    []int             `json:"scale,omitempty"`
	Families map[string]string `json:"families,omitempty"`
}

// musubiBrandTokens son los tokens REALES de Musubi (body-rs/src/ui.rs), estructurados: la fuente de la
// marca por default cuando no hay un doc 'diseno/marca' en el tenant musubi.
var musubiBrandTokens = &brandTokens{
	Name: "Musubi",
	Palette: map[string]string{
		"bg": "#0C1020", "surface": "#121734", "raised": "#182042", "line": "#2A335C",
		"ink": "#E9ECF7", "muted": "#98A0C0", "faint": "#5A6390",
		"accent": "#6366F1", "accent2": "#22D3EE",
		"ok": "#34D399", "warn": "#FBBF24", "danger": "#FB7185",
	},
	Radius:    map[string]int{"surface": 8, "pill": 4},
	Elevation: "flat",
	Type:      &brandType{Scale: []int{11, 12, 13, 15, 18, 24, 30}},
	Identity:  designBrand,
}

// parseBrandTokens intenta leer el contenido de una obs 'diseno/marca' como un doc JSON de tokens.
// Devuelve nil si no es JSON o si no trae una paleta útil (⇒ el caller lo trata como marca en prosa).
func parseBrandTokens(content string) *brandTokens {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") {
		return nil
	}
	var bt brandTokens
	if err := json.Unmarshal([]byte(content), &bt); err != nil {
		return nil
	}
	if len(bt.Palette) == 0 {
		return nil // sin paleta no hay nada que rellenar; que degrade a emit genérico
	}
	return &bt
}

// brandRoleOrder fija el ORDEN canónico de los roles (los map de Go iteran al azar; la salida del emit
// debe ser determinista) y el nombre del rol en cada dialecto.
var brandRoleOrder = []struct{ key, cssVar, painter string }{
	{"bg", "--bg", "fondo"},
	{"surface", "--surface", "superficie"},
	{"raised", "--raised", "elevada"},
	{"line", "--line", "borde"},
	{"ink", "--ink", "INK"},
	{"muted", "--muted", "MUTED"},
	{"faint", "--faint", "FAINT"},
	{"accent", "--accent", "CORD(acento)"},
	{"accent2", "--accent-2", "BRAIN"},
	{"ok", "--ok", "BODY(ok)"},
	{"warn", "--warn", "WARN"},
	{"danger", "--danger", "NO(peligro)"},
}

// render materializa los tokens de la marca al DIALECTO del target: painter (tabla rol=hex), web/html/any
// (variables CSS :root rellenas). Vacío si no hay tokens (⇒ el emit queda con la guía genérica). Es la
// pieza "una fuente → N targets" (F2), en Go puro y determinista.
func (bt *brandTokens) render(target string) string {
	if bt == nil || len(bt.Palette) == 0 {
		return ""
	}
	if target == "painter" {
		var parts []string
		for _, r := range brandRoleOrder {
			if hex, ok := bt.Palette[r.key]; ok {
				parts = append(parts, r.painter+"="+hex)
			}
		}
		s := "Paleta de la marca (usá ESTOS hex por rol): " + strings.Join(parts, " · ")
		if rad, ok := bt.Radius["surface"]; ok {
			s += fmt.Sprintf(" · radio=%d", rad)
		}
		if bt.Elevation != "" {
			s += " · elevación=" + bt.Elevation
		}
		return s
	}
	// web / html / any → variables CSS
	var b strings.Builder
	b.WriteString(":root {\n")
	for _, r := range brandRoleOrder {
		if hex, ok := bt.Palette[r.key]; ok {
			fmt.Fprintf(&b, "  %s: %s;\n", r.cssVar, hex)
		}
	}
	if rad, ok := bt.Radius["surface"]; ok {
		fmt.Fprintf(&b, "  --radius: %dpx;\n", rad)
	}
	if radp, ok := bt.Radius["pill"]; ok {
		fmt.Fprintf(&b, "  --radius-pill: %dpx;\n", radp)
	}
	b.WriteString("}")
	if bt.Type != nil && len(bt.Type.Scale) > 0 {
		b.WriteString("\nEscala tipográfica (px): ")
		for i, sz := range bt.Type.Scale {
			if i > 0 {
				b.WriteString("/")
			}
			fmt.Fprintf(&b, "%d", sz)
		}
	}
	if bt.Elevation == "flat" {
		b.WriteString("\nElevación PLANA: profundidad por capas de fondo + hairline, NUNCA sombra ni glow.")
	}
	return b.String()
}
