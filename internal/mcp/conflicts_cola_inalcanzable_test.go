package mcp

// Superficie MCP del orden FIFO de musubi_conflicts. La semántica del orden se prueba en
// internal/memory/pending_fifo_test.go, donde se pueden envejecer las filas; acá se protege lo que
// sólo se ve desde la tool: que `oldest` sea aceptado, que el error nombre las tres opciones, y que
// el aviso de cola LLEGUE AL AGENTE.
//
// Esa última parte es la que importa. El problema original no fue que faltara un orden: fue que un
// adjudicador con tope corrió semanas sobre las mismas relaciones sin enterarse de que había una
// cola de julio detrás. Un orden nuevo que el consumidor no sabe que existe no arregla nada.

import (
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
)

// respConflictos vive en conflicts_triage_test.go; acá se leen los campos nuevos del mapa crudo.
func campoDeConflictos(t *testing.T, s *McpServer, args map[string]interface{}, campo string) (string, bool) {
	t.Helper()
	res, e := call(t, s, "musubi_conflicts", args)
	if e != nil {
		t.Fatalf("musubi_conflicts %v: %+v", args, e)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(textOf(t, res)), &m); err != nil {
		t.Fatalf("respuesta no parseable: %v", err)
	}
	v, ok := m[campo].(string)
	return v, ok
}

// M1 — order=oldest es un valor VÁLIDO y no rompe nada más de la respuesta.
func TestOrderOldestEsAceptado(t *testing.T) {
	s := pendientesEn(t, 4)
	r := pedirConflictos(t, s, map[string]interface{}{"order": "oldest"})
	if r.Count != 4 {
		t.Errorf("count = %d, esperaba 4", r.Count)
	}
	if len(r.Relations) != 4 {
		t.Errorf("trajo %d relaciones, esperaba 4", len(r.Relations))
	}
}

// M2 — un order inventado sigue siendo error, y el mensaje nombra LAS TRES opciones. Si alguien
// suma un orden y se olvida del mensaje, el que se equivoca queda sin saber qué puede pedir.
func TestOrderInvalidoNombraLasTresOpciones(t *testing.T) {
	s := NewMcpServer(nil, t.TempDir(), embedding.NoopProvider{})
	_, rpcErr := call(t, s, "musubi_conflicts", map[string]interface{}{"order": "por_gravedad"})
	if rpcErr == nil {
		t.Fatal("un order inventado fue aceptado")
	}
	for _, opt := range []string{"recent", "confidence", "oldest"} {
		if !contieneTexto(rpcErr.Message, opt) {
			t.Errorf("el error no menciona %q: %q", opt, rpcErr.Message)
		}
	}
}

// M3 — EL AVISO LLEGA AL AGENTE. Con la lista truncada en un orden estable, la respuesta trae
// `oldest_pending_at` y un `tail_hint` que nombra la salida. Con FIFO no viene, porque la más vieja
// ya está en la página.
func TestElAvisoDeColaLlegaEnLaRespuesta(t *testing.T) {
	s := pendientesEn(t, 5)

	trunc := pedirConflictos(t, s, map[string]interface{}{"limit": 2})
	if !trunc.Truncated {
		t.Fatal("con limit=2 sobre 5 esperaba truncated:true")
	}
	if _, ok := campoDeConflictos(t, s, map[string]interface{}{"limit": 2}, "oldest_pending_at"); !ok {
		t.Error("la lista se truncó en un orden estable y la respuesta no dice cuán vieja es la más vieja")
	}
	hint, _ := campoDeConflictos(t, s, map[string]interface{}{"limit": 2}, "tail_hint")
	if !contieneTexto(hint, "oldest") {
		t.Errorf("el tail_hint no nombra la salida: %q", hint)
	}

	if _, ok := campoDeConflictos(t, s, map[string]interface{}{"limit": 2, "order": "oldest"}, "oldest_pending_at"); ok {
		t.Error("con order=oldest el aviso sobra: la más vieja ya viene en la página")
	}
	// Sin tope tampoco hay cola oculta.
	if _, ok := campoDeConflictos(t, s, map[string]interface{}{}, "oldest_pending_at"); ok {
		t.Error("sin límite no hay nada oculto y el aviso no corresponde")
	}
}

func contieneTexto(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
