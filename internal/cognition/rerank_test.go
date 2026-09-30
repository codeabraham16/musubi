package cognition

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// Invariantes del spec «El juez se puede medir» (specs/juez-medible/) que viven en este paquete, y
// los del spec «El juez ve ids cortos» (specs/juez-ids-cortos/), I1 a I4. Van con I y no con K
// porque la K de este paquete es la del caché (cache_test.go).

// motorGrabador responde un guion fijo y cuenta las llamadas.
type motorGrabador struct {
	respuesta string
	err       error
	llamadas  atomic.Int64
	system    atomic.Pointer[string]
	user      atomic.Pointer[string]
}

func (m *motorGrabador) Name() string { return "grabador" }

func (m *motorGrabador) Ask(_ context.Context, system, user string) (string, error) {
	m.llamadas.Add(1)
	m.system.Store(&system)
	m.user.Store(&user)
	if m.err != nil {
		return "", m.err
	}
	return m.respuesta, nil
}

// Los ids tienen la forma de los de producción, UUIDs, y no «a», «b», «c»: I1 prueba que ningún id
// real aparece en el prompt, y una letra suelta aparece en cualquier texto.
const (
	idA = "0b6f7c1e-3d2a-4c55-9a10-6e0f5b7d8c21"
	idB = "5f3e9a2b-81c4-4d6e-b7f0-2a9c1d4e6b83"
	idC = "c8d1e4f7-a2b5-4c8e-9f1a-3b6d9e2c5f47"
)

var candidatosDePrueba = []Candidato{
	{ID: idA, Gist: "el candado no cruza la red"},
	{ID: idB, Gist: "el bump de accesos es atómico"},
	{ID: idC, Gist: "el juez ordena, no descarta"},
}

// J2 — El juez NO cachea. Dos llamadas iguales golpean el motor dos veces.
//
// Es lo que habilita al banco a medir: con un caché adentro, N queries repetidas darían una sola
// llamada y el banco estaría midiendo el caché en vez del juez.
func TestJ2ElJuezNoCachea(t *testing.T) {
	m := &motorGrabador{respuesta: `["id-3","id-1","id-2"]`}
	for i := 0; i < 2; i++ {
		if _, err := Rerank(context.Background(), m, "misma consulta", candidatosDePrueba); err != nil {
			t.Fatalf("llamada %d: %v", i, err)
		}
	}
	if n := m.llamadas.Load(); n != 2 {
		t.Fatalf("esperaba 2 llamadas al motor (el juez no debe cachear), hubo %d", n)
	}
}

// J7 — El juez reordena, NUNCA descarta. Los ids inventados se ignoran y los no mencionados quedan
// al final en su orden original. Un juez capaz de hacer desaparecer una memoria del recall sería
// peor que no tener juez: seguiría en la base y el usuario no la vería nunca.
func TestJ7ElJuezReordenaPeroNuncaDescarta(t *testing.T) {
	casos := []struct {
		nombre string
		ids    []string
		orden  []string
		quiero []string
	}{
		{"orden completo", []string{"a", "b", "c"}, []string{"c", "a", "b"}, []string{"c", "a", "b"}},
		{"menciona de menos", []string{"a", "b", "c"}, []string{"c"}, []string{"c", "a", "b"}},
		{"inventa un id", []string{"a", "b"}, []string{"z", "b", "a"}, []string{"b", "a"}},
		{"repite un id", []string{"a", "b"}, []string{"a", "a", "b"}, []string{"a", "b"}},
		{"orden vacío", []string{"a", "b"}, nil, []string{"a", "b"}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := ReordenarIDs(c.ids, c.orden)
			if strings.Join(got, ",") != strings.Join(c.quiero, ",") {
				t.Fatalf("ReordenarIDs(%v, %v) = %v, quería %v", c.ids, c.orden, got, c.quiero)
			}
		})
	}
}

// J8 — Una respuesta imparseable es un ERROR, no un orden vacío. I4 extiende la misma regla a
// un array que se parsea pero no trae ningún rótulo de estos candidatos.
//
// Devolver nil sin error haría que el llamador reordenara contra una lista vacía: el resultado se
// vería idéntico al de un juez sano que no cambió nada. Un fallo indistinguible del éxito es el
// peor modo de falla que puede tener esto.
func TestJ8RespuestaImparseableEsError(t *testing.T) {
	basura := []string{
		"no tengo idea, perdón",
		"",
		"[",
		`["", "  "]`,
	}
	for _, resp := range basura {
		m := &motorGrabador{respuesta: resp}
		orden, err := Rerank(context.Background(), m, "consulta", candidatosDePrueba)
		if err == nil {
			t.Errorf("respuesta %q: esperaba error, obtuve orden=%v", resp, orden)
		}
	}
}

// La tolerancia del parseo es DELIBERADA y tiene su prueba, para que nadie la "arregle" creyendo
// que es laxitud. Los modelos envuelven el array en prosa o en un objeto aunque se les pida que no;
// rescatar los ids correctos de ahí es mejor que descartar una respuesta que sí sirve. El límite
// está en J8: si no hay un array de strings recuperable, es error. I6: con rótulos, igual.
func TestElParseoToleraEnvoltorios(t *testing.T) {
	casos := map[string]string{
		"prosa antes y después": `Claro, acá va: ["id-3","id-1","id-2"] — espero que sirva.`,
		"envuelto en un objeto": `{"orden": ["id-3","id-1","id-2"]}`,
		"con saltos de línea":   "```json\n[\"id-3\",\"id-1\",\"id-2\"]\n```",
	}
	quiero := strings.Join([]string{idC, idA, idB}, ",")
	for nombre, resp := range casos {
		t.Run(nombre, func(t *testing.T) {
			got := ParsearOrdenDeIDs(resp, candidatosDePrueba)
			if strings.Join(got, ",") != quiero {
				t.Fatalf("esperaba [idC idA idB], obtuve %v", got)
			}
		})
	}
}

// Control de J8: el motor caído también es error, y el mensaje conserva la causa para que el
// llamador pueda loguearla.
func TestJ8ElMotorCaidoEsError(t *testing.T) {
	roto := errors.New("connection refused")
	m := &motorGrabador{err: roto}
	if _, err := Rerank(context.Background(), m, "consulta", candidatosDePrueba); !errors.Is(err, roto) {
		t.Fatalf("esperaba que el error del motor se propagara envuelto, obtuve %v", err)
	}
}

// Sin candidatos no se gasta una llamada al motor: no hay nada que ordenar.
func TestRerankSinCandidatosNoLlamaAlMotor(t *testing.T) {
	m := &motorGrabador{respuesta: `["id-1"]`}
	if _, err := Rerank(context.Background(), m, "consulta", nil); err == nil {
		t.Fatal("esperaba error con la lista de candidatos vacía")
	}
	if n := m.llamadas.Load(); n != 0 {
		t.Fatalf("no debería haber llamado al motor, hubo %d llamadas", n)
	}
}

// I1 — El prompt lleva TODOS los candidatos, cada uno con su rótulo y su gist, en el orden en que
// llegan, y NINGÚN id real. Si el juez no ve una memoria, no la puede rankear, y el llamador la
// mandaría al final creyendo que el juez la despriorizó. Y si ve los ids reales, paga ~25 tokens por
// candidato al leerlos y otros ~24 al copiarlos, que es lo que el rótulo vino a sacar.
//
// Los rótulos van LITERALES: derivarlos de rotuloDelJuez haría que la prueba acompañara a la función
// en cualquier cambio.
//
// Sabotaje: el prompt vuelve a mostrar el id real del candidato.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="rotuloDelJuez(i), memory.EnUnaLinea"
// arnes: a="cands[i].ID, memory.EnUnaLinea"
func TestI1ElPromptRotulaYNoMuestraIDsReales(t *testing.T) {
	system, user := PromptJuez("mi consulta", candidatosDePrueba)
	if !strings.Contains(system, "array JSON") {
		t.Errorf("el system no pide el formato de salida: %q", system)
	}
	if !strings.Contains(user, "mi consulta") {
		t.Errorf("el user no lleva la consulta: %q", user)
	}
	quiero := "MEMORIAS:\n" +
		"[id-1] el candado no cruza la red\n" +
		"[id-2] el bump de accesos es atómico\n" +
		"[id-3] el juez ordena, no descarta\n"
	if !strings.HasSuffix(user, quiero) {
		t.Errorf("el user no lleva los candidatos rotulados id-1..id-3 y en orden:\n%s", user)
	}
	for _, c := range candidatosDePrueba {
		if strings.Contains(system+user, c.ID) {
			t.Errorf("el prompt muestra el id real %q: el juez sólo tiene que ver rótulos", c.ID)
		}
	}
}

// I2 — El juez contesta en rótulos y Rerank devuelve los ids REALES, en el orden que dictó. Hacia
// afuera no hay rótulos: producción y el banco reciben lo mismo que antes.
//
// Sabotaje: los rótulos empiezan en id-0.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="strconv.Itoa(i+1)"
// arnes: a="strconv.Itoa(i)"
func TestI2RerankTraduceLosRotulosALosIDsReales(t *testing.T) {
	m := &motorGrabador{respuesta: `["id-3","id-1","id-2"]`}
	orden, err := Rerank(context.Background(), m, "consulta", candidatosDePrueba)
	if err != nil {
		t.Fatalf("Rerank: %v", err)
	}
	if got, quiero := strings.Join(orden, ","), strings.Join([]string{idC, idA, idB}, ","); got != quiero {
		t.Fatalf("Rerank devolvió %v, quería [idC idA idB]", orden)
	}
}

// I1 y I2 con el top-K de producción. Con 12 candidatos (DefaultTopK) los rótulos llegan a dos
// dígitos (id-10..id-12), y las pruebas de arriba usan tres: un rótulo que no sobreviviera a los
// dos dígitos pasaría todas ellas y fallaría justo con el tope que se juzga en producción.
//
// El corte va en la entrada de la función y no en `strconv.Itoa(i+1)`, que ya es el ancla de I2:
// ahí los dos sabotajes se pisarían, y éste es el único que sólo esta prueba ve.
//
// Sabotaje: después de id-9 el rótulo se arma con UN carácter, que deja de ser un dígito (id-:,
// id-;, id-<). Con tres candidatos no cambia nada; con doce, sí.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="func rotuloDelJuez(i int) string {"
// arnes: a=`func rotuloDelJuez(i int) string { if i >= 9 { return "id-" + string(rune('1'+i)) };`
func TestI2ConElTopKDeProduccion(t *testing.T) {
	cands := make([]Candidato, 12)
	for i := range cands {
		n := strconv.Itoa(i + 1)
		cands[i] = Candidato{ID: "0b6f7c1e-3d2a-4c55-9a10-" + strings.Repeat("0", 12-len(n)) + n, Gist: "memoria " + n}
	}
	_, user := PromptJuez("consulta", cands)
	if !strings.HasSuffix(user, "[id-9] memoria 9\n[id-10] memoria 10\n[id-11] memoria 11\n[id-12] memoria 12\n") {
		t.Fatalf("el prompt no rotula id-10..id-12 en orden:\n%s", user)
	}
	m := &motorGrabador{respuesta: `["id-12","id-10","id-1","id-11"]`}
	orden, err := Rerank(context.Background(), m, "consulta", cands)
	if err != nil {
		t.Fatalf("Rerank: %v", err)
	}
	quiero := strings.Join([]string{cands[11].ID, cands[9].ID, cands[0].ID, cands[10].ID}, ",")
	if got := strings.Join(orden, ","); got != quiero {
		t.Fatalf("Rerank devolvió %v, quería los candidatos 12, 10, 1 y 11", orden)
	}
}

// I3 — Lo que no es un rótulo de estos candidatos se ignora, y un candidato repetido cuenta una sola
// vez. Lo que el juez no nombró no lo agrega Rerank: queda al final por ReordenarIDs, cuyo contrato
// (J7) no cambió. Se miran las DOS mitades, porque ReordenarIDs deduplica por su cuenta y taparía un
// Rerank que devuelve repetidos, y ese orden también va al caché de producción.
//
// Sabotaje: un rótulo desconocido pasa tal cual en vez de ignorarse.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="\t\tid, ok := real[r]\n"
// arnes: a="\t\tid, ok := real[r]\n\t\tif !ok {\n\t\t\tid, ok = r, true\n\t\t}\n"
//
// Sabotaje: un candidato repetido vuelve a entrar al orden.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="\t\tvisto[id] = true\n"
// arnes: a=""
func TestI3RotuloDesconocidoORepetidoSeIgnora(t *testing.T) {
	casos := map[string]string{
		"fuera de rango, basura y repetido": `["id-9","id-2","x","id-2","id-1"]`,
		"un id real mezclado con rótulos":   `["id-2","` + idC + `","id-1"]`,
	}
	for nombre, resp := range casos {
		t.Run(nombre, func(t *testing.T) {
			m := &motorGrabador{respuesta: resp}
			orden, err := Rerank(context.Background(), m, "consulta", candidatosDePrueba)
			if err != nil {
				t.Fatalf("Rerank: %v", err)
			}
			if got, quiero := strings.Join(orden, ","), strings.Join([]string{idB, idA}, ","); got != quiero {
				t.Fatalf("Rerank devolvió %v, quería [idB idA]", orden)
			}
			final := ReordenarIDs([]string{idA, idB, idC}, orden)
			if got, quiero := strings.Join(final, ","), strings.Join([]string{idB, idA, idC}, ","); got != quiero {
				t.Fatalf("ReordenarIDs devolvió %v, quería [idB idA idC]: el omitido va al final", final)
			}
		})
	}
}

// I4 — Un array que no trae NINGÚN rótulo de estos candidatos es un error, como J8. Son los dos
// casos que se producen sin mala intención: un doble de prueba o un camino viejo que contesta con
// los ids REALES, y un modelo que copia el EJEMPLO del system prompt. Si alguno diera un orden, el
// llamador reordenaría contra nada y el fallo se vería como «el juez no cambió nada».
//
// El caso del ejemplo no copia el ejemplo a mano: le da al parseo el system prompt entero, así que
// sigue al ejemplo que de verdad se le manda al modelo.
//
// Sabotaje: el parseo acepta también los ids reales.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="\t\treal[rotuloDelJuez(k)] = c.ID\n"
// arnes: a="\t\treal[rotuloDelJuez(k)] = c.ID\n\t\treal[c.ID] = c.ID\n"
//
// Sabotaje: el ejemplo del system prompt usa un rótulo válido.
// arnes: archivo="internal/cognition/rerank.go"
// arnes: de="id-a"
// arnes: a="id-2"
func TestI4SinNingunRotuloConocidoEsError(t *testing.T) {
	system, _ := PromptJuez("consulta", candidatosDePrueba)
	casos := map[string]string{
		"los ids reales, como en el protocolo viejo": `["` + idC + `","` + idA + `","` + idB + `"]`,
		"el ejemplo del system prompt":               system,
		"rótulos fuera de rango":                     `["id-0","id-4","id--1"]`,
		// D5: el rótulo se compara exacto. Aceptar otra grafía se decide con evidencia de un modelo
		// que la escriba (specs/juez-ids-cortos/medicion.md), no por adelantado.
		"otra grafía del rótulo": `["ID-1"," id-2","[id-3]","id1","id-01","3"]`,
		// El rótulo de la primera versión (c1..cN) ya no es de nadie: si el parseo lo siguiera
		// aceptando, habría dos formatos vivos y el prompt enseñaría sólo uno.
		"el rótulo de la primera versión": `["c3","c1","c2"]`,
	}
	for nombre, resp := range casos {
		t.Run(nombre, func(t *testing.T) {
			m := &motorGrabador{respuesta: resp}
			if orden, err := Rerank(context.Background(), m, "consulta", candidatosDePrueba); err == nil {
				t.Fatalf("esperaba error, obtuve orden=%v", orden)
			}
		})
	}
}
