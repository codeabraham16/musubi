package skills

import (
	"sort"
	"strings"
)

// niveles.go — EL CUERPO DE LA SKILL VIAJA CON LA EVIDENCIA.
//
// POR QUÉ. `musubi_resolve_skills` devolvía los `Skill` completos, `rules` incluido, sin niveles.
// Medido sobre las 11 skills de este repo: las 6 que declaran `triggers: ['*']` matchean CUALQUIER
// archivo, así que inyectaban ~1.750 tokens en cada resolución fueran o no relevantes. Con 100
// skills del mismo perfil eso es un muro, y es lineal: no aparece de golpe.
//
// LA REGLA NO ES UN UMBRAL. Recortar por tamaño ahorraría donde no hay duda y dejaría pasar lo
// dudoso. Se recorta por EVIDENCIA: se clasifica por qué rama del OR dejó entrar a la skill, y sólo
// las que entraron por una razón específica —un glob real, o el alcance que el llamador declaró—
// se llevan el cuerpo. Una skill que entró sólo por su '*' no tiene evidencia de ser relevante: su
// propio autor escribió en `always_because` que no podía atarla a un archivo.
//
// SIGUE SIENDO MODEL-FREE. Clasificar el matcheo es mirar qué condición se cumplió. No hay
// inferencia, no hay puntaje, no hay costo.

// ComoMatcheo dice por qué razón entró una skill en la resolución. El valor viaja en la respuesta:
// quien la lee tiene que poder ver por qué le llegó una skill, sobre todo cuando le llegó sin cuerpo.
type ComoMatcheo string

const (
	// PorAlcance — su `applies_to` coincide con la fase o la tarea que el llamador DECLARÓ. Es la
	// evidencia más fuerte: no se dedujo de un nombre de archivo, alguien la dijo.
	PorAlcance ComoMatcheo = "alcance"
	// PorGlob — alguno de los archivos coincide con un trigger de archivo real (`*.go`).
	PorGlob ComoMatcheo = "glob"
	// PorComodin — entró sólo porque su trigger es '*'. Matchea todo, así que no distingue nada.
	PorComodin ComoMatcheo = "comodin"
)

// evidenciaDe ordena las razones de más a menos específica. Es el orden en que se reparte el
// presupuesto de cuerpos.
func evidenciaDe(m ComoMatcheo) int {
	switch m {
	case PorAlcance:
		return 0
	case PorGlob:
		return 1
	default:
		return 2
	}
}

// SkillResuelta es una skill que matcheó, con la razón por la que matcheó y si su cuerpo viaja.
type SkillResuelta struct {
	Skill
	Matcheo ComoMatcheo
	// ConCuerpo dice si `Rules` se incluye en la respuesta. Falso NO significa que la skill no
	// aplique: significa que el llamador tiene que pedirlo (musubi_list_skills ya lo devuelve).
	ConCuerpo bool
}

// PresupuestoDeCuerpos es el techo en bytes de `rules` que una resolución puede devolver.
//
// 8192 B ≈ 2.048 tokens. La aritmética detrás del número, medida contra el arsenal real: un cambio
// típico en `.go` matchea CON EVIDENCIA `analyze-project` (374) + `deduce-conventions` (364) +
// `go-hygiene` (2.177) + `musubi-rules` (2.148) = 5.063 B, y entra entero. El techo empieza a morder
// alrededor de 1,6× eso — es decir, cuando el arsenal creció de verdad y no antes.
//
// NO ES CONFIG A PROPÓSITO. Sin un arsenal grande, un knob es una decisión que nadie puede tomar con
// datos. El día que haya evidencia de que 8192 queda corto, ese día se convierte en config.
const PresupuestoDeCuerpos = 8192

// SeleccionarCuerpos marca qué skills se llevan su `rules`, sin mutar la entrada.
//
// El orden de la respuesta NO cambia: lo único que se ordena es en qué orden se reparte el
// presupuesto. Devolver las skills en otro orden haría que dos llamadas iguales se vieran distintas
// por un detalle interno.
func SeleccionarCuerpos(res []SkillResuelta, presupuesto int) []SkillResuelta {
	out := make([]SkillResuelta, len(res))
	copy(out, res)

	orden := make([]int, 0, len(out))
	for i := range out {
		out[i].ConCuerpo = false
		if out[i].Matcheo == PorComodin {
			continue // sin evidencia: nunca se lleva el cuerpo, entre en el presupuesto o no
		}
		orden = append(orden, i)
	}

	// Determinista y sin depender del orden de lectura del disco: primero la evidencia más fuerte,
	// y dentro de cada grupo por nombre. `LoadSkills` hereda el orden de os.ReadDir, que es estable
	// pero no es un criterio: si mañana una skill se renombra, no queremos que otra pierda su cuerpo.
	sort.SliceStable(orden, func(a, b int) bool {
		ia, ib := orden[a], orden[b]
		if ea, eb := evidenciaDe(out[ia].Matcheo), evidenciaDe(out[ib].Matcheo); ea != eb {
			return ea < eb
		}
		return out[ia].Name < out[ib].Name
	})

	usado := 0
	for _, i := range orden {
		n := len(out[i].Rules)
		if usado+n > presupuesto {
			// Se saltea y se SIGUE, no se corta: una skill chica detrás de una grande no tiene por
			// qué quedarse afuera por el orden.
			continue
		}
		out[i].ConCuerpo = true
		usado += n
	}
	return out
}

// clasificarMatcheo devuelve la MEJOR razón por la que la skill entró, o false si no entró.
//
// La precedencia (alcance > glob > comodín) importa: una skill con `triggers: ['*', '*.go']` que
// matchea main.go entró por su glob real, y clasificarla por la primera rama evaluada le sacaría el
// cuerpo sin motivo.
func clasificarMatcheo(sk Skill, req ResolveRequest) (ComoMatcheo, bool) {
	if matchAlcance(sk, req) {
		return PorAlcance, true
	}
	var porGlob, porComodin bool
	for _, archivo := range req.ModifiedFiles {
		for _, t := range sk.Triggers {
			if !MatchGlob(t, archivo) {
				continue
			}
			if esComodin(t) {
				porComodin = true
				continue
			}
			porGlob = true
		}
	}
	switch {
	case porGlob:
		return PorGlob, true
	case porComodin:
		return PorComodin, true
	}
	return "", false
}

// esComodin reconoce el trigger que no dice nada. Cubre '*' y sus variantes ('**'), que matchean
// todo igual: lo que define a un comodín no es su forma sino que no distingue ningún archivo.
func esComodin(trigger string) bool {
	t := strings.TrimSpace(trigger)
	return t != "" && strings.Trim(t, "*") == ""
}
