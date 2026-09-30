package cognition

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"musubi/internal/memory"
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

// rotuloDelJuez es el id con el que el juez ve al candidato i (base 0): id-1, id-2, … id-N.
//
// EL JUEZ NO VE LOS IDS REALES. Un id real es un UUID de 36 caracteres, y los hex alternados
// tokenizan a 1,6 caracteres por token: ~25 tokens por candidato en el prompt y otros ~24 en la
// respuesta, que tiene que COPIARLOS todos en orden. Con 12 candidatos, la salida sola eran ~280
// tokens de hex, y la salida es lo caro: se genera token a token. Un rótulo cuesta 4 o 5. Medido
// en specs/juez-ids-cortos/medicion.md, donde está también el otro costo: un modelo chico copió
// mal un UUID (se comió el primer carácter), y ese id se ignoró por inventado.
//
// POR QUÉ «id-» Y NO UN RÓTULO MÁS CORTO. El primero fue c1..cN, y un modelo chico contestó en 2
// de 4 consultas con la FORMA del ejemplo del system prompt («id-7», «id-1234») en vez de copiar
// los rótulos que tenía a la vista: nada se reconoció y el juez degradó. El system prompt habla de
// «ids» y su ejemplo tiene esta forma, así que el rótulo la toma: lo que el modelo tiende a
// escribir pasa a ser un rótulo válido. El ejemplo sigue con LETRAS, a propósito: copiarlo tal
// cual no nombra a ningún candidato y es error (I4), no un orden inventado que parece sano.
//
// Es un string con prefijo y no un número pelado: un array de números no es el []string que se
// parsea, y un número se confunde con los que traen los gists.
//
// La usan el prompt y el parseo, así que un corrimiento acá no rompe el viaje de ida y vuelta:
// lo caza una prueba con rótulos LITERALES, no una que los derive de esta función.
func rotuloDelJuez(i int) string { return "id-" + strconv.Itoa(i+1) }

// PromptJuez arma el par (system, user) que se le manda al motor. Cada candidato va rotulado con
// rotuloDelJuez, en el orden en que llega.
//
// Está expuesto para que una prueba pueda comparar, byte a byte, lo que produce el camino de
// producción contra lo que produce este paquete. Sin esa comparación posible, «hay un solo juez»
// sería una afirmación de comentario y no un invariante.
func PromptJuez(query string, cands []Candidato) (system, user string) {
	var b strings.Builder
	for i, c := range cands {
		// EL GIST VA SANEADO, y acá no es cosmético. Este prompt es una LISTA de candidatos, uno
		// por línea, y el juez los ordena por rótulo. Un gist con un salto de línea fabrica
		// renglones nuevos: candidatos que no existen, o instrucciones con el aspecto de la lista
		// que el servidor armó. Ver internal/memory/linea_ajena.go.
		fmt.Fprintf(&b, "[%s] %s\n", rotuloDelJuez(i), memory.EnUnaLinea(c.Gist, 400))
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
// Los ids que devuelve son los REALES: el juez contesta en rótulos (ver rotuloDelJuez) y la
// traducción la hace ParsearOrdenDeIDs. Hacia afuera no hay rótulos.
//
// Devuelve error si el motor falla o si la respuesta no trae ningún rótulo de estos candidatos. Un
// error explícito y no una lista vacía: reordenar contra una lista vacía se vería como «el juez no
// cambió nada», que es indistinguible de un juez sano y por lo tanto el peor modo de falla posible.
func Rerank(ctx context.Context, p Provider, query string, cands []Candidato) ([]string, error) {
	if len(cands) == 0 {
		return nil, fmt.Errorf("rerank: sin candidatos que juzgar")
	}
	system, user := PromptJuez(query, cands)
	respuesta, err := p.Ask(ctx, system, user)
	if err != nil {
		return nil, fmt.Errorf("rerank: el motor falló: %w", err)
	}
	orden := ParsearOrdenDeIDs(respuesta, cands)
	if len(orden) == 0 {
		return nil, fmt.Errorf("rerank: la respuesta del motor no trae un array con rótulos de los candidatos (id-1..id-%d)", len(cands))
	}
	return orden, nil
}

// ParsearOrdenDeIDs extrae el array JSON de rótulos de la respuesta del juez y lo traduce a los
// ids REALES de cands. Tolera prosa alrededor (toma del primer '[' al último ']') porque los
// modelos la agregan aunque se les pida que no.
//
// Lo que no es un rótulo de ESTOS candidatos se ignora: uno fuera de rango, basura, un id real
// (el juez nunca los vio, así que si aparece uno lo escribió otra cosa) o el ejemplo del system
// prompt copiado. Un candidato que aparece dos veces cuenta en su primera aparición: el orden que
// sale de acá también va al caché de producción, y un orden con repetidos ahí sería basura
// guardada. Lo que el juez no nombró no se agrega: eso es de ReordenarIDs.
//
// LA TRADUCCIÓN ESTÁ ACÁ ADENTRO A PROPÓSITO, y por eso la firma pide los candidatos. Con un paso
// aparte, el día que un llamador se olvidara de traducir, ReordenarIDs recibiría rótulos, los
// descartaría todos como inventados y devolvería el orden model-free SIN error: el juez parecería
// no cambiar nada, que es el modo de falla que Rerank existe para no tener.
//
// Devuelve nil si no hay un array de strings parseable o si ninguno es un rótulo conocido.
func ParsearOrdenDeIDs(respuesta string, cands []Candidato) []string {
	i := strings.IndexByte(respuesta, '[')
	j := strings.LastIndexByte(respuesta, ']')
	if i < 0 || j <= i {
		return nil
	}
	var rotulos []string
	if err := json.Unmarshal([]byte(respuesta[i:j+1]), &rotulos); err != nil {
		return nil
	}
	real := make(map[string]string, len(cands))
	for k, c := range cands {
		real[rotuloDelJuez(k)] = c.ID
	}
	var out []string
	visto := make(map[string]bool, len(cands))
	for _, r := range rotulos {
		id, ok := real[r]
		if !ok || visto[id] {
			continue
		}
		visto[id] = true
		out = append(out, id)
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
