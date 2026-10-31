package motor

import (
	"encoding/json"
	"testing"
)

// EL MODELO DEL TRANSCRIPT. Errarle no da un error: da un turno al que le falta el
// razonamiento. Ver el comentario de ModeloResuelto — con un alias el CLI descarta los
// bloques de thinking firmados en silencio, así que el campo tiene que resolverse ACÁ y no
// confiar en que el cliente mandó el id completo.
func TestElPedidoResuelveElModeloDelTranscript(t *testing.T) {
	casos := []struct {
		nombre           string
		modelo, resuelto string
		quiero           string
	}{
		{
			// EL CASO QUE IMPORTA: el cliente pidió un alias y el proxy lo resolvió. Al
			// transcript tiene que ir el resuelto, no lo que pidió el cliente.
			nombre: "el resuelto le gana al alias",
			modelo: "sonnet", resuelto: "claude-sonnet-5",
			quiero: "claude-sonnet-5",
		},
		{
			// Sin resolución, lo que pidió el cliente es lo mejor que hay. Es correcto
			// cuando ya mandó el id completo.
			nombre: "sin resuelto cae en lo que pidió el cliente",
			modelo: "claude-opus-5", resuelto: "",
			quiero: "claude-opus-5",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			p := Pedido{Modelo: c.modelo, ModeloResuelto: c.resuelto}
			if got := p.modeloParaElTranscript(); got != c.quiero {
				t.Errorf("got %q, want %q", got, c.quiero)
			}
		})
	}
}

// EL stop_reason DEL ASISTENTE sale de acá. Un turno que pidió una tool y se declara
// "end_turn" le dice al modelo que la conversación había terminado cuando en realidad
// estaba esperando un resultado.
func TestUnMensajeSabeSiPidioUnaTool(t *testing.T) {
	bloque := func(s string) []json.RawMessage { return []json.RawMessage{json.RawMessage(s)} }

	casos := []struct {
		nombre string
		cont   []json.RawMessage
		quiero bool
	}{
		{"texto solo", bloque(`{"type":"text","text":"hola"}`), false},
		{"tool_use", bloque(`{"type":"tool_use","id":"t1","name":"Read","input":{}}`), true},
		// Las server tools (web_search y compañía) también dejan el turno esperando un
		// resultado, así que cuentan igual.
		{"server_tool_use", bloque(`{"type":"server_tool_use","id":"t1","name":"web_search"}`), true},
		{
			"el tool_use detrás de un texto igual cuenta",
			[]json.RawMessage{
				json.RawMessage(`{"type":"text","text":"ya voy"}`),
				json.RawMessage(`{"type":"tool_use","id":"t1","name":"Read","input":{}}`),
			},
			true,
		},
		// Un bloque ilegible no puede hacer explotar la serialización del transcript: se
		// ignora y el turno sale como si no hubiera pedido nada.
		{"un bloque roto no panica", bloque(`{roto`), false},
		{"sin contenido", nil, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			m := Mensaje{Rol: "assistant", Contenido: c.cont}
			if got := m.tieneToolUse(); got != c.quiero {
				t.Errorf("got %v, want %v", got, c.quiero)
			}
		})
	}
}

// Los dos constructores de conveniencia tienen que producir bloques que la API acepte: son
// lo que usan los tests y el camino de un pedido de una sola línea.
func TestLosConstructoresDeTextoArmanBloquesValidos(t *testing.T) {
	for _, m := range []Mensaje{TextoDeUsuario("hola"), TextoDeAsistente("chau")} {
		if len(m.Contenido) != 1 {
			t.Fatalf("%s: %d bloques, want 1", m.Rol, len(m.Contenido))
		}
		var b struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(m.Contenido[0], &b); err != nil {
			t.Fatalf("%s: el bloque no es JSON: %v", m.Rol, err)
		}
		if b.Type != "text" || b.Text == "" {
			t.Errorf("%s: bloque %+v", m.Rol, b)
		}
	}
	if TextoDeUsuario("x").Rol != "user" || TextoDeAsistente("x").Rol != "assistant" {
		t.Error("los roles se cruzaron")
	}
}
