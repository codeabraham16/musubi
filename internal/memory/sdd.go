package memory

import (
	"fmt"
	"strings"
	"unicode"
)

// sdd.go construye el FLUJO SDD GUIADO (O1) sobre el motor DAG model-free de
// workflow.go. SDD (Spec-Driven Development) no es un tipo de step nuevo: es un
// workflow CANÓNICO —proposal→spec→design→tasks→implement→verify→archive— que
// Musubi genera por vos a partir del nombre de un cambio, sin que escribas YAML.
//
// El diferencial es la FUSIÓN memoria↔orquestación: cada fase, al cerrarse,
// persiste su CONTRATO DE RESULTADO (summary/artifacts/risks/next) como una
// observación bajo topic_key sdd/<change>/<phase>. Las fases siguientes recuperan
// esos artefactos por referencia (~300 tokens con musubi_recall) en vez de releer
// archivos (3.000–15.000 tokens). Musubi secuencia y recuerda; el agente ejecuta.

// SDDPhases es la secuencia canónica del pipeline SDD (fuente de verdad del orden).
var SDDPhases = []string{"proposal", "spec", "design", "tasks", "implement", "verify", "archive"}

// sddPhaseTitles da el título legible de cada fase.
var sddPhaseTitles = map[string]string{
	"proposal":  "Propuesta — intención, alcance y rollback",
	"spec":      "Especificación — requisitos y escenarios",
	"design":    "Diseño — decisiones y rationale",
	"tasks":     "Tareas — checklist atómica por fase",
	"implement": "Implementación — código según spec/design",
	"verify":    "Verificación — tests/build contra la spec",
	"archive":   "Archivo — fusión de delta specs y cierre",
}

// sddTemplatePhases son las fases que tienen una plantilla de artefacto en
// .musubi/templates/sdd/<phase>.md (las demás son acción, no documento).
var sddTemplatePhases = map[string]bool{
	"proposal": true, "spec": true, "design": true, "tasks": true,
}

// SDDRunID deriva el run_id determinista de un cambio (resumible y único por cambio).
func SDDRunID(change string) string { return "sdd-" + SlugifyChange(change) }

// SDDTopicKey es la clave de memoria del artefacto de una fase: sdd/<change>/<phase>.
// Es el contrato de handoff: las fases siguientes recuperan por este prefijo.
func SDDTopicKey(change, phase string) string {
	return "sdd/" + SlugifyChange(change) + "/" + phase
}

// SDDTemplatePath devuelve la ruta relativa de la plantilla de una fase (y true) si
// la fase es documental; false para fases de acción (implement/verify/archive).
func SDDTemplatePath(phase string) (string, bool) {
	if !sddTemplatePhases[phase] {
		return "", false
	}
	return ".musubi/templates/sdd/" + phase + ".md", true
}

// SDDWorkflowDef construye el workflow canónico SDD para un cambio: una cadena
// lineal de fases donde cada una depende de la anterior. Reusa el motor DAG, así
// que el run es resumible entre sesiones y compactaciones como cualquier workflow.
func SDDWorkflowDef(change string) WorkflowDef {
	steps := make([]WorkflowStep, 0, len(SDDPhases))
	prev := ""
	for _, p := range SDDPhases {
		s := WorkflowStep{ID: p, Title: sddPhaseTitles[p]}
		if prev != "" {
			s.Needs = []string{prev}
		}
		steps = append(steps, s)
		prev = p
	}
	return WorkflowDef{
		ID:            SDDRunID(change),
		Name:          "SDD: " + change,
		SchemaVersion: "1.0",
		Steps:         steps,
	}
}

// SDDPhaseDirective devuelve la guía concreta de qué hacer en una fase del flujo
// SDD, con el nombre del cambio interpolado para los hints de recall. Nunca vacía.
func SDDPhaseDirective(phase, change string) string {
	slug := SlugifyChange(change)
	switch phase {
	case "proposal":
		return "Fase PROPOSAL. Redactá la propuesta: intención, alcance y estrategia de rollback. " +
			"Plantilla: .musubi/templates/sdd/proposal.md. Al cerrar pasá summary + artifacts + risks."
	case "spec":
		return "Fase SPEC. Escribí requisitos verificables y escenarios Given/When/Then " +
			"(vocabulario RFC 2119: DEBE/DEBERÍA/PUEDE). Plantilla: .musubi/templates/sdd/spec.md."
	case "design":
		return "Fase DESIGN. Documentá las decisiones de arquitectura con su rationale y las " +
			"alternativas descartadas. Plantilla: .musubi/templates/sdd/design.md."
	case "tasks":
		return "Fase TASKS. Descomponé en una checklist numerada de tareas atómicas por fase. " +
			"Plantilla: .musubi/templates/sdd/tasks.md."
	case "implement":
		return fmt.Sprintf("Fase IMPLEMENT. Recuperá los artefactos previos con "+
			"musubi_recall query='sdd/%s' (no releas archivos ya gisteados: usá musubi_recall_code). "+
			"Implementá según spec/design siguiendo las convenciones (musubi_resolve_skills). "+
			"Guardá decisiones no obvias con musubi_save_observation.", slug)
	case "verify":
		return "Fase VERIFY. Verificá contra la spec: corré build/tests. Registrá los fallos con " +
			"musubi_log_error. Si todo pasa, cerrá la fase para pasar a archive."
	case "archive":
		return fmt.Sprintf("Fase ARCHIVE. Fusioná los delta specs en las specs principales y archivá. "+
			"Guardá el aprendizaje final con musubi_save_observation topic_key='sdd/%s/archive'.", slug)
	default:
		return "Trabajá en esta fase y cerrala con musubi_sdd action=complete."
	}
}

// sddRoles da, por fase, el ROL especializado que el agente debe encarnar (la
// "biblioteca de roles" del pilar de orquestación: análogo a los sub-agentes de un
// orquestador SDD, pero contextual —se surface con la fase activa— en vez de 9
// archivos estáticos). Complementa a la directiva: la directiva dice QUÉ hacer, el
// rol dice DESDE QUÉ MENTALIDAD hacerlo.
var sddRoles = map[string]string{
	"proposal":  "Proponente: pensás en intención y valor antes que en código. Delimitás alcance, declarás lo que queda fuera y una estrategia de rollback. Escéptico del scope creep.",
	"spec":      "Especificador: convertís la intención en requisitos verificables y atómicos, con escenarios Given/When/Then observables. Nada ambiguo; vocabulario RFC 2119.",
	"design":    "Diseñador: decidís la arquitectura y JUSTIFICÁS cada decisión con su rationale y las alternativas descartadas. Priorizás la opción más simple que cumple la spec.",
	"tasks":     "Planificador: descomponés el diseño en una checklist de tareas atómicas, ordenadas y sin solapamiento, cada una cerrable de forma independiente.",
	"implement": "Implementador: seguís spec y design al pie de la letra, respetás las convenciones del proyecto y recuperás los artefactos previos por memoria en vez de releer. No inventás alcance nuevo.",
	"verify":    "Verificador: sos ADVERSARIAL. Corré tests/build y buscás activamente en qué NO cumple la implementación la spec. Ante la duda, falla. Registrá cada hallazgo.",
	"archive":   "Archivador: consolidás los delta specs en las specs principales, dejás la memoria coherente y destilás el aprendizaje reutilizable del cambio.",
}

// SDDRole devuelve el rol especializado de una fase (o "" si la fase no tiene rol).
func SDDRole(phase string) string { return sddRoles[phase] }

// SDDContract es el CONTRATO DE RESULTADO de una fase SDD: lo que el agente reporta
// al cerrarla. Summary es obligatorio; el resto es opcional. Se persiste en memoria
// como el handoff para las fases siguientes.
type SDDContract struct {
	Summary         string   `json:"summary"`
	Artifacts       []string `json:"artifacts,omitempty"`
	Risks           []string `json:"risks,omitempty"`
	NextRecommended string   `json:"next_recommended,omitempty"`
}

// Memo formatea el contrato como el cuerpo de la observación de handoff (Markdown).
func (c SDDContract) Memo(change, phase string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## SDD %s — %s\n\n%s\n", phase, change, strings.TrimSpace(c.Summary))
	if len(c.Artifacts) > 0 {
		b.WriteString("\n**Artefactos:**\n")
		for _, a := range c.Artifacts {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	if len(c.Risks) > 0 {
		b.WriteString("\n**Riesgos:**\n")
		for _, r := range c.Risks {
			fmt.Fprintf(&b, "- %s\n", r)
		}
	}
	if strings.TrimSpace(c.NextRecommended) != "" {
		fmt.Fprintf(&b, "\n**Siguiente recomendado:** %s\n", strings.TrimSpace(c.NextRecommended))
	}
	return b.String()
}

// SlugifyChange normaliza el nombre de un cambio a un slug estable para run_id y
// topic_key: minúsculas, letras/dígitos preservados (incl. unicode), cualquier otra
// cosa colapsa a un único guion, sin guiones en los extremos.
func SlugifyChange(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
