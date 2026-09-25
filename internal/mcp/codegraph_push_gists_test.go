package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// La federación del grafo llevaba nodos y aristas pero NO los gists. Medido en el cerebro central
// el 2026-08-12: 4.862 nodos y 10.100 aristas federados contra CERO filas en code_memory. El
// agujero importa porque `musubi_recall_code` contra el central es la única vía al gist en un
// proyecto sin hooks — y contra un central vacío no devuelve nada.
//
// El punto delicado no es agregar el campo: es que "no mandé gists" y "no tengo gists" son cosas
// distintas en un protocolo de REEMPLAZO. Por eso el receptor lo lee como puntero.

func gistsDe(t *testing.T, s *McpServer, proyecto string) []memory.CodeMemory {
	t.Helper()
	ctx := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: proyecto})
	g, err := s.engine.AllCodeMemoryCtx(ctx)
	if err != nil {
		t.Fatalf("AllCodeMemoryCtx(%s): %v", proyecto, err)
	}
	return g
}

func pushear(t *testing.T, s *McpServer, args map[string]interface{}) {
	t.Helper()
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: raw})
	p := &Principal{Name: "davantis-musubi", Role: RoleWriter, ProjectID: "musubi"}
	if _, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params); rpcErr != nil {
		t.Fatalf("push falló: %+v", rpcErr)
	}
}

var gistEjemplo = []memory.CodeMemory{
	{Path: "internal/memory/outbox.go", Gist: "encolado y drenaje del outbox", Symbols: "BackfillOutbox,PurgeOutboxPending", Fingerprint: "f1", Tokens: 24},
	{Path: "internal/mcp/registry.go", Gist: "catálogo de tools MCP", Symbols: "toolsAllEnabled", Fingerprint: "f2", Tokens: 18},
}

// Los gists federados se persisten bajo el proyecto del PRINCIPAL, con la misma guarda de tenant
// que ya tenía el grafo: un write=own no puede plantarlos en otro proyecto aunque lo declare.
func TestGistsFederadosSePersistenYRespetanElTenant(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pushear(t, s, map[string]interface{}{
		"nodes":      []memory.GraphNode{},
		"edges":      []memory.GraphEdge{},
		"gists":      gistEjemplo,
		"project_id": "ajeno", // declarado a propósito: un write=own debe ignorarlo
	})

	got := gistsDe(t, s, "musubi")
	if len(got) != 2 {
		t.Fatalf("esperaba 2 gists bajo 'musubi', hay %d", len(got))
	}
	if got[0].Gist == "" || got[0].Symbols == "" || got[0].Fingerprint == "" {
		t.Errorf("el gist llegó incompleto: %+v", got[0])
	}
	if n := len(gistsDe(t, s, "ajeno")); n != 0 {
		t.Errorf("el project_id declarado debía ignorarse: 'ajeno' tiene %d gists", n)
	}
}

// EL TEST DE COMPATIBILIDAD. Un cliente VIEJO —anterior a que el push llevara gists— no manda la
// clave. Si el receptor leyera eso como "reemplazá por vacío", ese cliente le borraría los gists
// al central cada vez que indexa. Es un escenario real: el mismo proyecto empujado desde dos
// máquinas con binarios distintos.
func TestPushSinLaClaveGistsNoBorraLosGuardados(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pushear(t, s, map[string]interface{}{
		"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{}, "gists": gistEjemplo,
	})
	if n := len(gistsDe(t, s, "musubi")); n != 2 {
		t.Fatalf("preparación: esperaba 2 gists, hay %d", n)
	}

	// Push de un cliente viejo: nodos y aristas, sin la clave 'gists'.
	pushear(t, s, map[string]interface{}{
		"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{},
	})
	if n := len(gistsDe(t, s, "musubi")); n != 2 {
		t.Errorf("un cliente viejo borró los gists del central: quedan %d de 2", n)
	}
}

// La contracara: mandar la clave VACÍA sí es una instrucción explícita de borrado. Sin esto no
// habría forma de federar "este proyecto ya no tiene gists".
func TestPushConGistsVaciosSiLosReemplaza(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pushear(t, s, map[string]interface{}{
		"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{}, "gists": gistEjemplo,
	})
	pushear(t, s, map[string]interface{}{
		"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{}, "gists": []memory.CodeMemory{},
	})
	if n := len(gistsDe(t, s, "musubi")); n != 0 {
		t.Errorf("una lista vacía explícita debe reemplazar: quedan %d gists", n)
	}
}

// Re-empujar el mismo set no duplica: el push es idempotente, igual que con nodos y aristas.
func TestPushDeGistsEsIdempotente(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	for i := 0; i < 3; i++ {
		pushear(t, s, map[string]interface{}{
			"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{}, "gists": gistEjemplo,
		})
	}
	if n := len(gistsDe(t, s, "musubi")); n != 2 {
		t.Errorf("tres pushes idénticos dejaron %d gists, esperaba 2", n)
	}
}

// Y del lado del emisor: la clave 'gists' viaja en el payload. Si no viaja, todo lo de arriba es
// correcto y no sirve para nada, porque el central nunca recibiría un gist.
func TestPushGraphMandaLaClaveGistsEnElPayload(t *testing.T) {
	var recibido map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Arguments map[string]interface{} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		recibido = req.Params.Arguments
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	defer ts.Close()

	client := newTestSyncClient(t, ts.URL)
	if err := client.PushGraph(nil, nil, gistEjemplo); err != nil {
		t.Fatalf("PushGraph: %v", err)
	}
	g, ok := recibido["gists"].([]interface{})
	if !ok {
		t.Fatalf("el payload no lleva la clave 'gists'; llegó: %v", recibido)
	}
	if len(g) != 2 {
		t.Errorf("llegaron %d gists, esperaba 2", len(g))
	}

	// Y SIN gists —nil o una lista vacía, que es lo que da una foto de una base sin gists— la clave
	// NO viaja. Hasta el 2026-09-24 esta prueba exigía lo contrario (mandar `[]` para poder vaciar
	// los del central), y eso era justo lo que hacía que una máquina sin gists le borrara al central
	// los de las otras en cada push. El dueño eligió que el daemon ya no pueda vaciarlos por push.
	for _, sinGists := range [][]memory.CodeMemory{nil, {}} {
		recibido = nil
		if err := client.PushGraph(nil, nil, sinGists); err != nil {
			t.Fatalf("PushGraph(%#v): %v", sinGists, err)
		}
		if v, presente := recibido["gists"]; presente {
			t.Errorf("sin gists el emisor mandó la clave (%#v): el central la leería como «borrá los míos»", v)
		}
	}
}
