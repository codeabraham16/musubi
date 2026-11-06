package memory

import (
	"fmt"
	"hash/fnv"
	"time"
)

// EL BRAZO DEL A/B DEL DECIDIDOR DE SKILLS (spec skills-decididor, R9).
//
// POR QUÉ VIVE EN memory Y NO EN cognition (donde lo puso la checklist). cognition importa memory
// (ver rerank.go). El registrador del eje `loaded` —que estampa el brazo— vive en memory
// (skillusage.go). Si BrazoAB viviera en cognition, memory tendría que importar cognition y sería
// un ciclo. memory es el piso común: lo importan tanto cognition como mcp, sin ciclo.

// Brazos del experimento. On = grupo de tratamiento (el decididor actúa); Off = control (no actúa).
const (
	BrazoOn  = "on"
	BrazoOff = "off"
)

// BrazoAB asigna una cohorte a un brazo del A/B, DETERMINISTA y con CROSSOVER POR DÍA:
// hash(cohorte|fecha-UTC) % 2.
//
// POR QUÉ LA COHORTE ES EL project_id (tenant) Y NO EL PRINCIPAL CRUDO. El tratamiento (la
// exigencia) nace en el CENTRAL, pero el resultado que mide el A/B (el eje `loaded`) se registra en
// la máquina LOCAL que cargó la skill. Para que las dos puntas caigan en el MISMO brazo sin un
// protocolo que las coordine, la asignación depende sólo de algo que las dos conocen igual: el
// tenant. project_id viaja idéntico por el sync; el principal crudo, no.
//
// POR QUÉ CROSSOVER POR DÍA. Cada cohorte es su propio control entre días (paira la varianza
// entre-máquinas) y dentro de un día su asignación es estable (mata la contaminación de un azar
// por-tarea que mezclaría tratamiento y control en la misma sesión). La fecha va en UTC para que
// dos nodos en husos distintos crucen el mismo día.
//
// Es PURO y model-free: mismo (cohorte, fecha) ⇒ mismo brazo, siempre, sin red ni estado.
func BrazoAB(cohorte string, fecha time.Time) string {
	h := fnv.New64a()
	// El '|' delimita cohorte de fecha. La fecha tiene formato fijo YYYY-MM-DD (10 chars, sin '|'),
	// así que no hay ambigüedad de dónde termina una y empieza la otra.
	fmt.Fprintf(h, "%s|%s", cohorte, fecha.UTC().Format("2006-01-02"))
	if h.Sum64()%2 == 0 {
		return BrazoOff
	}
	return BrazoOn
}
