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
	// MinAlwaysBecauseChars es el piso de una justificación de trigger "*". No es un capricho de
	// longitud: es lo mínimo para que la frase diga ALGO. "porque sí" no es una declaración de
	// alcance, y un campo que se satisface con dos palabras no cambia ninguna conducta.
	MinAlwaysBecauseChars = 20
)

// Penalizaciones del score (base 100, piso 0). Un error pesa mucho más que un warning.
const (
	scoreBase         = 100
	penaltyPerError   = 34
	penaltyPerWarning = 12
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

	// R3b: el alcance declarado tiene que estar en el vocabulario CERRADO.
	//
	// Es ERROR y no warning porque el modo de falla es invisible: un `applies_to: [phase:planing]`
	// con typo no rompe nada, no avisa nada, y produce una skill que simplemente no se activa
	// nunca. Indistinguible de una skill que no aplica — y por eso nadie lo encontraría.
	for _, a := range s.AppliesTo {
		if !AlcanceValido(a) {
			r.Errors = append(r.Errors, QualityIssue{
				Code:    "applies_to_desconocido",
				Message: "applies_to declara un alcance que no existe: " + a,
				Fix:     "usá uno de: " + strings.Join(VocabularioDeAlcance(), ", "),
			})
		}
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
	// R7: la skill se activa siempre sin declarar por qué.
	if WildcardUnjustified(s) {
		r.Warnings = append(r.Warnings, QualityIssue{
			Code:    "triggers_over_broad",
			Message: "la skill se activa SIEMPRE ('*' entre sus triggers) y compite por contexto en cada tarea",
			Fix:     "acotá los triggers a donde aplica de verdad (ej. '*.go', 'Dockerfile'); si de verdad aplica siempre —porque se activa por TIPO DE TAREA y no por archivo— declaralo en 'always_because'",
		})
	}
	// R7b: el '*' convive con triggers específicos. Es peor que el caso de arriba y por eso se
	// nombra aparte: los específicos hacen PARECER que la skill está acotada cuando el '*' ya la
	// activó en todo. Un 'always_because' justifica el '*', no vuelve honesto al adorno.
	if anyWildcard(s.Triggers) && !allWildcard(s.Triggers) {
		r.Warnings = append(r.Warnings, QualityIssue{
			Code:    "triggers_wildcard_masks_specific",
			Message: "hay un '*' mezclado con triggers específicos: los específicos no significan nada, el '*' ya activa la skill en todo",
			Fix:     "quitá el '*' si la skill es acotada, o quitá los triggers específicos si de verdad aplica siempre",
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

// WildcardUnjustified indica si la skill se activa SIEMPRE sin declarar por qué.
//
// Es la ÚNICA definición de «'*' sin declarar» del sistema: la consumen el score de calidad —donde
// es un warning— y la puerta de promoción al arsenal —donde bloquea—. La asimetría es deliberada:
// en tu proyecto el alcance es tu problema; en el arsenal compartido es el de todos. Pero la
// DEFINICIÓN tiene que ser una sola, o la advertencia local y el rechazo del central terminarían
// diciendo cosas distintas de la misma skill.
func WildcardUnjustified(s Skill) bool {
	if !anyWildcard(s.Triggers) {
		return false
	}
	return len([]rune(strings.TrimSpace(s.AlwaysBecause))) < MinAlwaysBecauseChars
}

// anyWildcard indica si ALGUNO de los triggers es "*".
//
// Alguno, no todos: un solo "*" vuelve decorativos a los demás, porque la skill ya se activa en
// todo. La versión anterior exigía que TODOS lo fueran, así que ["*", "*.go"] —el caso que miente
// sobre su alcance— se le escapaba entero.
func anyWildcard(triggers []string) bool {
	for _, t := range triggers {
		if strings.TrimSpace(t) == "*" {
			return true
		}
	}
	return false
}

// allWildcard indica si hay triggers y TODOS son "*". Sigue existiendo para distinguir el caso
// honesto (sólo "*") del enmascarado ("*" mezclado con específicos).
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

// containsAny indica si s contiene alguna de las frases, RESPETANDO LÍMITES DE PALABRA.
//
// Antes era un strings.Contains pelado y aprobaba de más. Caso real medido: `adversarial-review`
// pasaba el check desc_no_trigger porque la lista incluye "al " y su descripción dice «revisión
// advers-AL- estilo debate». Un token de tres caracteres buscado ADENTRO de las palabras aprueba
// casi cualquier texto en castellano, y el warning que existe justamente para avisar que la skill
// no dice cuándo usarse quedaba mudo en la skill que más lo necesitaba.
//
// El límite se chequea sólo al INICIO de la frase: las de la lista ya terminan en espacio o son
// frases completas, así que exigirlo también al final rechazaría "use when" seguido de coma.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		for i := 0; ; {
			j := strings.Index(s[i:], sub)
			if j < 0 {
				break
			}
			inicio := i + j
			if inicio == 0 || !esLetraODigito(s[inicio-1]) {
				return true
			}
			i = inicio + 1
			if i >= len(s) {
				break
			}
		}
	}
	return false
}

// esLetraODigito trabaja sobre bytes ASCII a propósito: alcanza para decidir si el carácter previo
// pega la frase adentro de una palabra, y evita arrastrar unicode a un chequeo heurístico.
func esLetraODigito(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
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
