package cognition

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// Invariantes del spec «El juez se puede medir» (specs/juez-medible/) que viven en este paquete.

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

var candidatosDePrueba = []Candidato{
	{ID: "a", Gist: "el candado no cruza la red"},
	{ID: "b", Gist: "el bump de accesos es atómico"},
	{ID: "c", Gist: "el juez ordena, no descarta"},
}

// J2 — El juez NO cachea. Dos llamadas iguales golpean el motor dos veces.
//
// Es lo que habilita al banco a medir: con un caché adentro, N queries repetidas darían una sola
// llamada y el banco estaría midiendo el caché en vez del juez.
func TestJ2ElJuezNoCachea(t *testing.T) {
	m := &motorGrabador{respuesta: `["c","a","b"]`}
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

// J8 — Una respuesta imparseable es un ERROR, no un orden vacío.
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
// está en J8: si no hay un array de strings recuperable, es error.
func TestElParseoToleraEnvoltorios(t *testing.T) {
	casos := map[string]string{
		"prosa antes y después": `Claro, acá va: ["c","a","b"] — espero que sirva.`,
		"envuelto en un objeto": `{"orden": ["c","a","b"]}`,
		"con saltos de línea":   "```json\n[\"c\",\"a\",\"b\"]\n```",
	}
	for nombre, resp := range casos {
		t.Run(nombre, func(t *testing.T) {
			got := ParsearOrdenDeIDs(resp)
			if strings.Join(got, ",") != "c,a,b" {
				t.Fatalf("esperaba [c a b], obtuve %v", got)
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
	m := &motorGrabador{respuesta: `["a"]`}
	if _, err := Rerank(context.Background(), m, "consulta", nil); err == nil {
		t.Fatal("esperaba error con la lista de candidatos vacía")
	}
	if n := m.llamadas.Load(); n != 0 {
		t.Fatalf("no debería haber llamado al motor, hubo %d llamadas", n)
	}
}

// El prompt lleva TODOS los candidatos con su id y su gist: si el juez no ve una memoria, no la
// puede rankear, y el llamador la mandaría al final creyendo que el juez la despriorizó.
func TestPromptJuezLlevaTodosLosCandidatos(t *testing.T) {
	system, user := PromptJuez("mi consulta", candidatosDePrueba)
	if !strings.Contains(system, "array JSON") {
		t.Errorf("el system no pide el formato de salida: %q", system)
	}
	if !strings.Contains(user, "mi consulta") {
		t.Errorf("el user no lleva la consulta: %q", user)
	}
	for _, c := range candidatosDePrueba {
		if !strings.Contains(user, "["+c.ID+"] "+c.Gist) {
			t.Errorf("el user no lleva al candidato %q con su gist:\n%s", c.ID, user)
		}
	}
}
