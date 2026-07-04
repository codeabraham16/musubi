package memory

import "strings"

// memtype.go clasifica cada observación por TIPO de memoria (estilo LangMem), un enum
// model-free que el agente (que ya es un LLM) declara al guardar: el server nunca infiere el
// tipo del contenido. El tipo modula el OLVIDO: un evento puntual (episódico) se enfría más
// rápido que un hecho estable (semántico) o un procedimiento (procedural). Es un eje
// ORTOGONAL a la recencia (esa curva es de B3): acá sólo se pondera la saliencia por tipo.

// Tipos de memoria canónicos.
const (
	MemSemantic   = "semantic"   // conocimiento estable: hechos, definiciones, decisiones
	MemEpisodic   = "episodic"   // eventos puntuales: "hoy pasó X", experiencias fechadas
	MemProcedural = "procedural" // cómo hacer algo: recetas, pasos, convenciones operativas
)

// Pesos de saliencia por tipo. Modulan el olvido (salience *= peso): <1 se archiva antes,
// >1 es más durable. Los valores fijan un ORDEN relativo defendible (episódico se enfría
// antes; procedural persiste), no números sagrados; son consts para mantener el olvido
// determinista y model-free (mismo criterio que pprDamping), configurables a futuro. El tipo
// vacío/desconocido pesa 1.0 → una observación sin tipo decae EXACTAMENTE como antes de B2
// (equivalencia backward-compat).
const (
	weightEpisodic   = 0.6
	weightSemantic   = 1.0
	weightProcedural = 1.5
	weightUntyped    = 1.0
)

// normalizeMemType mapea una entrada libre al enum canónico (case-insensitive, trim). Vacío o
// no reconocido → "" (sin tipo): no inventamos un tipo por defecto para no sesgar el olvido de
// memorias que el agente no quiso clasificar.
func normalizeMemType(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case MemSemantic:
		return MemSemantic
	case MemEpisodic:
		return MemEpisodic
	case MemProcedural:
		return MemProcedural
	default:
		return ""
	}
}

// memTypeSalienceWeight devuelve el peso de saliencia del tipo. "" o desconocido → 1.0
// (neutro): garantiza que las observaciones sin tipo se comporten como antes de B2.
func memTypeSalienceWeight(memType string) float64 {
	switch normalizeMemType(memType) {
	case MemEpisodic:
		return weightEpisodic
	case MemProcedural:
		return weightProcedural
	case MemSemantic:
		return weightSemantic
	default:
		return weightUntyped
	}
}
