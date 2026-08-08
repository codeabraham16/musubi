package cognition

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// EL JUEZ DE PERTINENCIA, como unidad reusable.
//
// POR QUÉ VIVE ACÁ Y NO EN EL SERVIDOR. Antes el juez estaba adentro de `rerankIfEnabled`, en
// internal/mcp, atado al McpServer, a memory.RecallResult y a un caché global de paquete. Eso
// dejaba al banco de evaluación (internal/recalleval) sin forma de llamarlo: su única salida era
// escribir SU PROPIA versión del prompt y del parseo.
//
// Y ése era el problema real: un banco que mide una IMITACIÓN del juez es peor que no tener banco.
// Devuelve un número con aspecto de autoridad sobre algo que no es lo que corre en producción, y el
// día que el prompt cambie el número sigue igual de convincente y ya es falso.
//
// Acá hay UNA definición del prompt y UNA del parseo. Producción y banco llaman a la misma.

// DefaultTopK es cuántos candidatos del tope se someten a juicio cuando nadie lo fija. Acotado a
// propósito: el juez es caro (latencia + cuota), y sólo mira la cabeza del ranking.
//
// Vive acá y no en cada llamador porque si el banco de evaluación juzgara más candidatos que el
// servidor, estaría midiendo una configuración que nadie va a correr — y el número saldría bien
// sin que nadie note que responde otra pregunta.
const DefaultTopK = 12

// Candidato es lo mínimo que el juez necesita ver de una memoria: su identidad y su titular.
//
// Es un tipo propio y no memory.RecallItem a propósito: el juez no tiene por qué saber de
// importancia, decaimiento ni procedencia, y atarlo al tipo del libro mayor lo volvería a acoplar a
// una capa que no le corresponde.
type Candidato struct {
	ID   string
	Gist string
}

// sistemaJuez es el prompt de sistema del juez. Es la ÚNICA copia: si el banco tuviera la suya,
// medir dejaría de significar algo.
const sistemaJuez = "Sos un juez de pertinencia. Dada una CONSULTA y MEMORIAS candidatas, devolvé SÓLO un array JSON " +
	"con TODOS los ids ordenados de MÁS a MENOS relevante para la consulta. Nada de prosa ni explicación, " +
	"sólo el JSON. Ejemplo de formato: [\"id-a\",\"id-b\",\"id-c\"]."

// PromptJuez arma el par (system, user) que se le manda al motor.
//
// Está expuesto para que una prueba pueda comparar, byte a byte, lo que produce el camino de
// producción contra lo que produce este paquete. Sin esa comparación posible, «hay un solo juez»
// sería una afirmación de comentario y no un invariante.
func PromptJuez(query string, cands []Candidato) (system, user string) {
	var b strings.Builder
	for _, c := range cands {
		fmt.Fprintf(&b, "[%s] %s\n", c.ID, c.Gist)
	}
	return sistemaJuez, "CONSULTA: " + query + "\n\nMEMORIAS:\n" + b.String()
}

// Rerank le pide al motor que ordene los candidatos por pertinencia y devuelve los ids en el orden
// que dictó.
//
// LO QUE DELIBERADAMENTE **NO** HACE, y es del llamador:
//   - NO cachea. En producción el caché estira la suscripción y protege el rate-limit compartido;
//     adentro del juez falsearía toda medición del banco, que necesita una llamada por query.
//   - NO fija timeout. El backstop es política del llamador (producción usa uno; el banco, otro).
//   - NO recorta el top-K. Cuántos candidatos se someten a juicio lo decide quien llama.
//
// Devuelve error si el motor falla o si la respuesta no trae un array de ids parseable. Un error
// explícito y no una lista vacía: reordenar contra una lista vacía se vería como «el juez no cambió
// nada», que es indistinguible de un juez sano y por lo tanto el peor modo de falla posible.
func Rerank(ctx context.Context, p Provider, query string, cands []Candidato) ([]string, error) {
	if len(cands) == 0 {
		return nil, fmt.Errorf("rerank: sin candidatos que juzgar")
	}
	system, user := PromptJuez(query, cands)
	respuesta, err := p.Ask(ctx, system, user)
	if err != nil {
		return nil, fmt.Errorf("rerank: el motor falló: %w", err)
	}
	orden := ParsearOrdenDeIDs(respuesta)
	if len(orden) == 0 {
		return nil, fmt.Errorf("rerank: no pude parsear un array de ids de la respuesta del motor")
	}
	return orden, nil
}

// ParsearOrdenDeIDs extrae el array JSON de ids de la respuesta del juez. Tolera prosa alrededor
// (toma del primer '[' al último ']') porque los modelos la agregan aunque se les pida que no.
// Devuelve nil si no hay un array de strings parseable.
func ParsearOrdenDeIDs(respuesta string) []string {
	i := strings.IndexByte(respuesta, '[')
	j := strings.LastIndexByte(respuesta, ']')
	if i < 0 || j <= i {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(respuesta[i:j+1]), &ids); err != nil {
		return nil
	}
	out := ids[:0]
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			out = append(out, id)
		}
	}
	return out
}

// ReordenarIDs aplica el orden que dictó el juez sobre una lista de ids.
//
// EL JUEZ REORDENA, NUNCA DESCARTA. Los ids que el juez inventó se ignoran, y los que no mencionó
// quedan AL FINAL en su orden original. Un juez capaz de hacer desaparecer una memoria del recall
// sería peor que no tener juez: la memoria seguiría en la base y el usuario no la vería nunca.
func ReordenarIDs(ids []string, orden []string) []string {
	presente := make(map[string]bool, len(ids))
	for _, id := range ids {
		presente[id] = true
	}
	out := make([]string, 0, len(ids))
	puesto := make(map[string]bool, len(ids))
	for _, id := range orden {
		if presente[id] && !puesto[id] {
			out = append(out, id)
			puesto[id] = true
		}
	}
	for _, id := range ids { // los que el juez no mencionó, en su orden original
		if !puesto[id] {
			out = append(out, id)
		}
	}
	return out
}
