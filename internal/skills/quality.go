package skills

import (
	"regexp"
	"strings"
)

// quality.go implementa el VALIDADOR DE CALIDAD model-free de una skill: el núcleo
// del sistema avanzado de creación. Deriva sus reglas de las best-practices oficiales
// de Anthropic Agent Skills y de validadores reputados, y las expresa como checks
// deterministas (sin LLM). Separa ERRORES (bloquean el guardado; alta confianza) de
// WARNINGS (avisan; heurísticos, para no dar falsos positivos duros), y produce un
// score 0-100 con fixes accionables.

// Límites de calidad (Anthropic Agent Skills + eficiencia en tokens).
const (
	// DescMaxChars es el techo oficial de la description de una skill.
	DescMaxChars = 1024
	// RulesMaxChars es el umbral blando de las rules: por encima, la skill pesa
	// demasiado en tokens cada vez que se inyecta (progressive disclosure).
	RulesMaxChars = 5000
)

// Penalizaciones del score (base 100, piso 0). Un error pesa mucho más que un warning.
const (
	scoreBase          = 100
	penaltyPerError    = 34
	penaltyPerWarning  = 12
)

// QualityIssue es un hallazgo del validador: un código estable, el mensaje y cómo
// arreglarlo.
type QualityIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

// QualityReport es el resultado de validar una skill: errores (bloquean), warnings
// (avisan) y un score 0-100.
type QualityReport struct {
	Errors   []QualityIssue `json:"errors"`
	Warnings []QualityIssue `json:"warnings"`
	Score    int            `json:"score"`
}

// OK indica si la skill pasa el gate (sin errores). Los warnings no bloquean.
func (r QualityReport) OK() bool { return len(r.Errors) == 0 }

var (
	// reReservedName detecta las palabras reservadas del formato Agent Skills en el name.
	reReservedName = regexp.MustCompile(`(?i)(anthropic|claude)`)
	// rePerson detecta 1ª/2ª persona en la description (rompe el discovery; debe ir en 3ª).
	rePerson = regexp.MustCompile(`(?i)\b(i|you|your|we|yo|vos|puedo|pod[eé]s|podemos)\b`)
	// reWinPath detecta paths estilo Windows (backslash entre caracteres de palabra).
	reWinPath = regexp.MustCompile(`\w\\\w`)
)

// triggerClauses son señales de que la description dice CUÁNDO usar la skill (su rol
// como disparador). Su ausencia es un warning, no un error.
var triggerClauses = []string{"use when", "use this", "trigger", "when the", "when working", "when editing", "cuando ", "al ", "usá cuando", "usar cuando"}

// exampleMarkers indican presencia de un ejemplo concreto en las rules.
var exampleMarkers = []string{"```", "ejemplo", "example", "e.g.", "p.ej"}

// ValidateSkillQuality corre todos los checks de calidad sobre una skill y devuelve el
// reporte. Es puro y model-free: solo mira los campos de la skill.
func ValidateSkillQuality(s Skill) QualityReport {
	var r QualityReport
	desc := strings.TrimSpace(s.Description)
	descLower := strings.ToLower(desc)
	rules := s.Rules

	// --- ERRORES (bloquean) ---
	// R1: description presente.
	if desc == "" {
		r.Errors = append(r.Errors, QualityIssue{
			Code:    "desc_empty",
			Message: "la description está vacía; es el DISPARADOR de la skill (lo que decide cuándo cargarla)",
			Fix:     "escribí una description en tercera persona que diga QUÉ hace y CUÁNDO usarla (ej. 'Procesa … . Use when …')",
		})
	} else if len([]rune(desc)) > DescMaxChars {
		// R2: description dentro del límite oficial.
		r.Errors = append(r.Errors, QualityIssue{
			Code:    "desc_too_long",
			Message: "la description supera el máximo de 1024 caracteres del formato Agent Skills",
			Fix:     "resumí la description; movés el detalle a las rules (que se cargan bajo demanda)",
		})
	}
	// R3: name sin palabras reservadas.
	if reReservedName.MatchString(s.Name) {
		r.Errors = append(r.Errors, QualityIssue{
			Code:    "name_reserved",
			Message: "el name contiene una palabra reservada ('anthropic' o 'claude')",
			Fix:     "renombrá la skill sin esas palabras (ej. describí la capacidad, no el proveedor)",
		})
	}

	// --- WARNINGS (avisan; heurísticos) ---
	if desc != "" {
		// R4: la description debería declarar cuándo usarla.
		if !containsAny(descLower, triggerClauses) {
			r.Warnings = append(r.Warnings, QualityIssue{
				Code:    "desc_no_trigger",
				Message: "la description no dice CUÁNDO usar la skill; sin eso el agente casi no la dispara",
				Fix:     "agregá una cláusula tipo 'Use when …' / 'Usá cuando …' con los términos que gatillan la skill",
			})
		}
		// R5: tercera persona.
		if rePerson.MatchString(desc) {
			r.Warnings = append(r.Warnings, QualityIssue{
				Code:    "desc_person",
				Message: "la description usa 1ª/2ª persona; se inyecta en el system prompt y debe ir en tercera persona",
				Fix:     "reescribí en tercera persona ('Procesa …', no 'Puedo ayudarte a …' ni 'You can …')",
			})
		}
		// R8a: keyword stuffing.
		if strings.Count(desc, "\"") >= 10 {
			r.Warnings = append(r.Warnings, QualityIssue{
				Code:    "desc_keyword_stuffing",
				Message: "la description parece 'keyword stuffing' (muchas frases entrecomilladas)",
				Fix:     "escribí una description natural con términos clave, sin ametrallar comillas",
			})
		}
	}
	// R6: rules dentro del presupuesto de tokens.
	if len([]rune(rules)) > RulesMaxChars {
		r.Warnings = append(r.Warnings, QualityIssue{
			Code:    "rules_too_long",
			Message: "las rules son muy largas; se inyectan en contexto y cuestan tokens cada vez",
			Fix:     "recortá a lo esencial y accionable; sacá lo obvio (el agente ya es capaz)",
		})
	}
	// R7: triggers no exclusivamente over-broad.
	if allWildcard(s.Triggers) {
		r.Warnings = append(r.Warnings, QualityIssue{
			Code:    "triggers_over_broad",
			Message: "los triggers son todos '*': la skill se activa SIEMPRE y compite por contexto en cada tarea",
			Fix:     "acotá los triggers a los archivos donde la skill aplica de verdad (ej. '*.go', 'Dockerfile')",
		})
	}
	// R8b: paths estilo Windows en las rules.
	if reWinPath.MatchString(rules) {
		r.Warnings = append(r.Warnings, QualityIssue{
			Code:    "rules_windows_paths",
			Message: "las rules usan paths estilo Windows (backslash); rompen en Unix",
			Fix:     "usá siempre '/' en los paths de las rules",
		})
	}
	// R9: al menos un ejemplo concreto.
	if strings.TrimSpace(rules) != "" && !containsAny(strings.ToLower(rules), exampleMarkers) {
		r.Warnings = append(r.Warnings, QualityIssue{
			Code:    "rules_no_example",
			Message: "las rules no incluyen un ejemplo concreto; los ejemplos suben mucho la utilidad de una skill",
			Fix:     "agregá al menos un ejemplo ejecutable (bloque de código) o un patrón input→output",
		})
	}

	r.Score = scoreFor(len(r.Errors), len(r.Warnings))
	return r
}

// scoreFor calcula el score 0-100 a partir de la cantidad de errores y warnings.
func scoreFor(errors, warnings int) int {
	score := scoreBase - errors*penaltyPerError - warnings*penaltyPerWarning
	if score < 0 {
		return 0
	}
	return score
}

// allWildcard indica si hay triggers y TODOS son "*" (over-broad).
func allWildcard(triggers []string) bool {
	if len(triggers) == 0 {
		return false
	}
	for _, t := range triggers {
		if strings.TrimSpace(t) != "*" {
			return false
		}
	}
	return true
}

// containsAny indica si s contiene alguno de los substrings (s ya en minúsculas).
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// SourceTrustTier clasifica la confiabilidad de la FUENTE de una skill a partir de su
// URL, para priorizar fuentes reputadas al derivar skills. No descarga nada: es una
// heurística sobre el host/owner. Tiers: "official" (Anthropic / doc oficial) >
// "curated" (repos de skills reputados y curados) > "community" (otro repo público) >
// "unknown" (sin fuente).
func SourceTrustTier(sourceURL string) string {
	u := strings.ToLower(strings.TrimSpace(sourceURL))
	if u == "" {
		return "unknown"
	}
	switch {
	case strings.Contains(u, "github.com/anthropics/"),
		strings.Contains(u, "anthropic.com"),
		strings.Contains(u, "claude.com"),
		strings.Contains(u, "agentskills.io"):
		return "official"
	case strings.Contains(u, "github.com/patrickjs/awesome-cursorrules"),
		strings.Contains(u, "github.com/gentleman-programming/"),
		strings.Contains(u, "cursor.directory"),
		strings.Contains(u, "musubi-skills"):
		return "curated"
	default:
		return "community"
	}
}
