package cognition

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"musubi/internal/memory"
)

// EL DECIDIDOR DE SKILLS, como unidad reusable. Hermano de Rerank (ver rerank.go) y calcado de él
// a propósito: mismo contrato (una copia del prompt, un parseo, sin caché/timeout/gate adentro),
// porque ese molde ya está probado y medido.
//
// POR QUÉ EXISTE. Musubi OFRECE un catálogo de skills y DELEGA la elección al agente; medido, eso
// da 0 cargas en 402 turnos. Cuando un texto EXIGE una skill puntual, la adherencia salta a 7/7.
// El decididor convierte "catálogo ofrecido" en "orden tomada": dado el texto de la tarea, el motor
// elige A LO SUMO UNA skill que la tarea exija, o ninguna. El servidor la entrega como exigencia
// imperativa montada sobre la respuesta del recall (ver internal/mcp/exigirSkillSiCorresponde).
//
// Acá vive SÓLO el juicio puro: prompt + llamada + parseo + validación contra el catálogo. La
// disciplina de cuándo llamarlo (guard, gate, freno, caché, best-effort) es del llamador, igual
// que en Rerank.

// SkillN1 es lo mínimo que el motor necesita ver de una skill para elegirla: su nombre, qué hace y
// cuándo aplica. Es un tipo propio y no skills.Skill a propósito —el mismo motivo que Candidato no
// es memory.RecallItem—: el motor no tiene por qué ver `rules`, capabilities ni procedencia, y
// atarlo al tipo del arsenal lo acoplaría a una capa que no le corresponde. El "N1" es por
// nivel-1: el catálogo sin cuerpo (ver internal/skills/niveles.go).
type SkillN1 struct {
	Name        string
	Description string
	Cuando      string
}

// Eleccion es lo que el motor decidió. Skill == "" significa NINGUNA: una respuesta válida y
// frecuente (la mayoría de las tareas no exige una skill). Lleva tags JSON porque se deserializa
// directo de la respuesta del motor, que contesta {"skill":...,"razon":...}.
type Eleccion struct {
	Skill string `json:"skill"`
	Razon string `json:"razon"`
}

// sistemaDecididor es el prompt de sistema. Es la ÚNICA copia: si un banco o una prueba tuviera la
// suya, "hay un solo decididor" sería una afirmación de comentario y no un invariante —el mismo
// razonamiento que sistemaJuez—.
const sistemaDecididor = "Sos un selector de skills. Dada una TAREA y un CATÁLOGO de skills disponibles " +
	"(cada una con su nombre, qué hace y, si está, cuándo aplica), elegí A LO SUMO UNA skill que la tarea " +
	"EXIJA de verdad, o NINGUNA si ninguna aplica con claridad. Preferí ninguna antes que forzar una que " +
	"no corresponde. Devolvé SÓLO un objeto JSON, sin prosa ni explicación: " +
	"{\"skill\":\"<el nombre EXACTO del catálogo, o cadena vacía si ninguna>\",\"razon\":\"<una frase breve>\"}. " +
	"No inventes skills que no estén en el catálogo."

// PromptDecididor arma el par (system, user) que se le manda al motor. Expuesto para que una prueba
// pueda comparar byte a byte producción contra lo que arma este paquete —ídem PromptJuez—.
func PromptDecididor(query string, cat []SkillN1) (system, user string) {
	var b strings.Builder
	for _, s := range cat {
		// EL CATÁLOGO VA SANEADO, y no es cosmético: es una LISTA, una skill por línea. Una
		// descripción con un salto de línea fabricaría skills que no existen, o inyectaría una
		// instrucción con el aspecto de un ítem de la lista. Ver internal/memory/linea_ajena.go.
		fmt.Fprintf(&b, "- %s: %s", s.Name, memory.EnUnaLinea(s.Description, 400))
		if c := memory.EnUnaLinea(s.Cuando, 400); c != "" {
			fmt.Fprintf(&b, " (cuándo: %s)", c)
		}
		b.WriteByte('\n')
	}
	return sistemaDecididor, "TAREA: " + query + "\n\nCATÁLOGO:\n" + b.String()
}

// ElegirSkill le pide al motor que elija la skill que la tarea exige, y valida la respuesta contra
// el catálogo que vio.
//
// LO QUE DELIBERADAMENTE NO HACE, y es del llamador (idéntico a Rerank):
//   - NO cachea: el caché estira la suscripción y protege el rate-limit, pero adentro falsearía
//     cualquier medición que necesite una llamada por tarea.
//   - NO fija timeout: el backstop es política del llamador.
//   - NO gatea: decidir CUÁNDO vale la pena gastar una llamada es del llamador (el gate conservador
//     vive en el servidor).
//
// Devuelve error si el catálogo está vacío (no hay nada que elegir), si el motor falla, o si la
// respuesta no trae un objeto JSON parseable. Una skill que el motor haya INVENTADO (no está en el
// catálogo) NO es error: se descarta a "ninguna" (Eleccion{}), porque exigir cargar algo que no
// existe manda al agente a buscar un archivo que no está —peor que no exigir nada—.
func ElegirSkill(ctx context.Context, p Provider, query string, cat []SkillN1) (Eleccion, error) {
	if len(cat) == 0 {
		return Eleccion{}, fmt.Errorf("elegir_skill: catálogo vacío, no hay nada que elegir")
	}
	system, user := PromptDecididor(query, cat)
	respuesta, err := p.Ask(ctx, system, user)
	if err != nil {
		return Eleccion{}, fmt.Errorf("elegir_skill: el motor falló: %w", err)
	}
	elec, ok := ParsearEleccion(respuesta)
	if !ok {
		return Eleccion{}, fmt.Errorf("elegir_skill: no pude parsear un objeto JSON de la respuesta del motor")
	}
	// R8 — ELECCIÓN VÁLIDA. El motor sólo puede exigir una skill que EXISTE en el catálogo que vio.
	// Una alucinación se descarta a "ninguna": si devolviéramos la skill inventada, el agente
	// recibiría la orden de cargar algo inexistente.
	if elec.Skill != "" && !enCatalogo(elec.Skill, cat) {
		return Eleccion{}, nil
	}
	return elec, nil
}

// ParsearEleccion extrae el objeto JSON de la respuesta del motor. Tolera prosa alrededor (toma del
// primer '{' al último '}') porque los modelos la agregan aunque se les pida que no —misma
// tolerancia deliberada que ParsearOrdenDeIDs—. Devuelve ok=false si no hay objeto parseable.
func ParsearEleccion(respuesta string) (Eleccion, bool) {
	i := strings.IndexByte(respuesta, '{')
	j := strings.LastIndexByte(respuesta, '}')
	if i < 0 || j <= i {
		return Eleccion{}, false
	}
	var e Eleccion
	if err := json.Unmarshal([]byte(respuesta[i:j+1]), &e); err != nil {
		return Eleccion{}, false
	}
	e.Skill = strings.TrimSpace(e.Skill)
	e.Razon = strings.TrimSpace(e.Razon)
	return e, true
}

// enCatalogo indica si name es el nombre exacto de alguna skill del catálogo. Igualdad exacta: la
// misma regla que matchAlcance, y por el mismo motivo —un "casi" no distingue un acierto de una
// alucinación—.
func enCatalogo(name string, cat []SkillN1) bool {
	for _, s := range cat {
		if s.Name == name {
			return true
		}
	}
	return false
}
