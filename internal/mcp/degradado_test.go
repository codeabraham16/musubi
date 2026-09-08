package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// errDePrueba es deliberadamente irrepetible: si alguna de estas afirmaciones pasara por
// casualidad —un mensaje genérico que contenga la palabra «esquema», por ejemplo— no habría forma
// de que contenga esta frase.
var errDePrueba = errors.New("la base está en el esquema v999 pero este binario solo llega a v47")

func servidorDegradadoDePrueba() *McpServer {
	return NewServidorDegradado(t8Dir, "0.0.0-degradado-de-prueba", errDePrueba)
}

// t8Dir es un projectPath cualquiera: el servidor degradado no toca el disco.
const t8Dir = "."

// D1 — EL SERVIDOR DEGRADADO CONTESTA EL HANDSHAKE Y DECLARA POR QUÉ NO PUEDE TRABAJAR.
//
// Es el invariante que define la pieza. Lo que había antes no era un handshake pobre: eran CERO
// BYTES por stdout y exit 1, indistinguible de un musubi no instalado. Este test mide lo que el
// cliente MCP recibe de verdad —la respuesta serializada, no la struct interna—, porque el defecto
// original vivía justamente en el borde entre el proceso y el cliente.
func TestD1ElDegradadoContestaElHandshakeYDeclaraLaCausa(t *testing.T) {
	s := servidorDegradadoDePrueba()

	resp, ok := s.Dispatch(context.Background(), JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "initialize"})
	if !ok {
		t.Fatal("initialize no produjo respuesta: el servidor degradado sigue mudo")
	}
	crudo, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("serializar la respuesta: %v", err)
	}

	var sobre struct {
		Error  *RpcError `json:"error"`
		Result struct {
			ServerInfo map[string]string `json:"serverInfo"`
			Meta       struct {
				Degraded *struct {
					Reason      string `json:"reason"`
					ToolsUsable bool   `json:"tools_usable"`
				} `json:"musubi/degraded"`
			} `json:"_meta"`
		} `json:"result"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		t.Fatalf("parsear la respuesta: %v", err)
	}
	if sobre.Error != nil {
		t.Fatalf("el handshake devolvió error: %+v", sobre.Error)
	}
	if sobre.Result.Meta.Degraded == nil {
		t.Fatal("el handshake no declaró la degradación: el cliente no puede distinguirlo de un servidor sano")
	}
	if !strings.Contains(sobre.Result.Meta.Degraded.Reason, errDePrueba.Error()) {
		t.Errorf("la causa declarada no es la real:\n  dice:    %q\n  esperaba que contuviera: %q",
			sobre.Result.Meta.Degraded.Reason, errDePrueba.Error())
	}
	if sobre.Result.Meta.Degraded.ToolsUsable {
		t.Error("el servidor degradado declaró tools_usable=true")
	}
	// Y sigue diciendo qué binario es: es la mitad de la información que hace falta para arreglarlo.
	if sobre.Result.ServerInfo["version"] == "" {
		t.Error("el handshake degradado no publicó ninguna versión")
	}
}

// D2 — EL DEGRADADO LISTA EL MISMO CATÁLOGO QUE UN SERVIDOR SANO.
//
// Un catálogo vacío sería la misma mentira de antes con otra cara: le enseña al agente que este
// servidor no tiene nada que ofrecer, cuando lo que pasa es que no tiene con qué trabajar. Se
// compara la HUELLA además del conteo: dos listas del mismo largo con descripciones distintas
// pasarían un conteo y no son el mismo catálogo.
func TestD2ElDegradadoListaElMismoCatalogoQueElSano(t *testing.T) {
	sano := newTestServer(t, embedding.NoopProvider{})
	deg := servidorDegradadoDePrueba()

	nSano, shaSano := huellaDelCatalogo(sano.tools)
	nDeg, shaDeg := huellaDelCatalogo(deg.tools)
	if nDeg != nSano || shaDeg != shaSano {
		t.Errorf("el catálogo del degradado no es el del sano:\n  sano      %d tools %s\n  degradado %d tools %s",
			nSano, shaSano, nDeg, shaDeg)
	}

	// Y lo que importa es lo que SALE por tools/list, no el registro interno.
	lista, ok := deg.handleToolsList().(map[string]interface{})["tools"].([]Tool)
	if !ok {
		t.Fatal("tools/list del degradado no devolvió []Tool")
	}
	if len(lista) != nDeg {
		t.Errorf("tools/list degradado sirve %d tools y su registro declara %d", len(lista), nDeg)
	}
	if len(lista) == 0 {
		t.Error("el servidor degradado sirvió un catálogo VACÍO")
	}
}

// D3 — TODA tools/call DEVUELVE UN ERROR EXPLÍCITO QUE NOMBRA LA CAUSA, Y NUNCA DESPACHA.
//
// Las dos mitades importan y son distintas. Si despachara, el handler tocaría un engine nil, el
// recover de Dispatch lo convertiría en «error interno inesperado» y estaríamos de vuelta en un
// mensaje que no dice nada — por eso el test va por Dispatch y no por handleToolsCall: el pánico
// quedaría atrapado ahí, que es exactamente el disfraz que hay que detectar.
func TestD3ElDegradadoRechazaTodaLlamadaNombrandoLaCausa(t *testing.T) {
	s := servidorDegradadoDePrueba()

	// Una tool que existe y una que no: las dos tienen que contestar lo mismo. Que el servidor
	// no pueda trabajar es anterior a si el nombre es válido.
	for _, tool := range []string{"musubi_recall", "musubi_no_existe_esta_tool"} {
		params, _ := json.Marshal(map[string]interface{}{"name": tool, "arguments": map[string]interface{}{}})
		req := JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "tools/call", Params: params}
		resp, ok := s.Dispatch(context.Background(), req)
		if !ok {
			t.Fatalf("%s: tools/call no produjo respuesta", tool)
		}
		if resp.Error == nil {
			t.Fatalf("%s: el degradado devolvió un resultado OK en vez de un error", tool)
		}
		if resp.Error.Code != codeDegraded {
			t.Errorf("%s: código %d, esperaba codeDegraded (%d) — %q",
				tool, resp.Error.Code, codeDegraded, resp.Error.Message)
		}
		if !strings.Contains(resp.Error.Message, errDePrueba.Error()) {
			t.Errorf("%s: el error no nombra la causa:\n  %q", tool, resp.Error.Message)
		}
	}
}

// D4 — UN SERVIDOR SANO NO SE DECLARA DEGRADADO NI DEJA DE DESPACHAR.
//
// Sin este test, «degradar siempre» pasaría D1, D2 y D3 con las mejores notas y rompería el
// producto entero.
func TestD4ElServidorSanoNoSeDeclaraDegradado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	crudo, err := json.Marshal(s.handleInitialize())
	if err != nil {
		t.Fatalf("serializar el handshake: %v", err)
	}
	if strings.Contains(string(crudo), "musubi/degraded") {
		t.Errorf("un servidor sano declaró degradación en el handshake:\n  %s", crudo)
	}

	params, _ := json.Marshal(map[string]interface{}{"name": "musubi_recall", "arguments": map[string]interface{}{"query_text": "x"}})
	resp, ok := s.Dispatch(context.Background(), JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "tools/call", Params: params})
	if !ok {
		t.Fatal("tools/call no produjo respuesta")
	}
	if resp.Error != nil && resp.Error.Code == codeDegraded {
		t.Errorf("un servidor sano rechazó una tool por degradación: %q", resp.Error.Message)
	}
}
