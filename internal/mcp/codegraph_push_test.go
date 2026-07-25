package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestCodegraphPushAttributesToPrincipal valida el invariante crítico de la federación (Track 20 ·
// F6, E2/R2/R3): un push write=own se atribuye SIEMPRE al proyecto del PRINCIPAL, IGNORANDO el
// project_id declarado en el payload — un tenant no puede plantar su grafo en otro.
func TestCodegraphPushAttributesToPrincipal(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	nodes := []memory.GraphNode{{Key: "x.go#func:X", Kind: "func", Name: "X", Path: "x.go", SrcFingerprint: "1"}}
	args, _ := json.Marshal(map[string]interface{}{
		"nodes":      nodes,
		"edges":      []memory.GraphEdge{},
		"project_id": "ajeno", // declara OTRO proyecto: debe ignorarse (write=own)
	})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: args})

	p := &Principal{Name: "davantis-musubi", Role: RoleWriter, ProjectID: "musubi"} // write=own (por rol)
	ctx := withPrincipal(context.Background(), p)
	if _, rpcErr := s.handleToolsCall(ctx, params); rpcErr != nil {
		t.Fatalf("push falló: %+v", rpcErr)
	}

	// Quedó bajo "musubi" (el principal), NO bajo "ajeno" (lo declarado, ignorado).
	own := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: "musubi"})
	if n, _ := s.engine.AllGraphNodesCtx(own); len(n) != 1 || n[0].Name != "X" {
		t.Errorf("el grafo debería estar bajo el proyecto del principal (musubi), ve %+v", n)
	}
	ajeno := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: "ajeno"})
	if n, _ := s.engine.AllGraphNodesCtx(ajeno); len(n) != 0 {
		t.Errorf("el project_id declarado debía ignorarse: 'ajeno' no debería tener nodos, ve %+v", n)
	}
}

// TestCodegraphPushRejectsNonWriter: una credencial write=none (cabina de solo lectura) NO puede
// federar grafo — el push es una tool WRITE (D3), así que canCall la rechaza con codeUnauthorized.
func TestCodegraphPushRejectsNonWriter(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	args, _ := json.Marshal(map[string]interface{}{"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{}})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: args})

	cabina := &Principal{Name: "crm-cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone}
	ctx := withPrincipal(context.Background(), cabina)
	_, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr == nil || rpcErr.Code != codeUnauthorized {
		t.Errorf("cabina write=none debería ser rechazada con codeUnauthorized, got %+v", rpcErr)
	}
}

// TestCodegraphPushNoOpWithoutSync: sin syncClient, la federación es un no-op total (R6/E4) — ni se
// intenta el push (no hay red).
func TestCodegraphPushNoOpWithoutSync(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{}) // no se llama a SetSyncClient
	if attempted, ok := s.pushCodeGraphToCentral(context.Background()); attempted || ok {
		t.Errorf("sin syncClient el push debe ser no-op, got attempted=%v ok=%v", attempted, ok)
	}
}

// TestCodegraphPushBestEffortSwallowsFailure: con sync + team mode pero el central caído (500), el
// push se INTENTA y FALLA, pero el error se traga sin romper nada (R5/E3) — el índice nunca se
// rompería por un fallo de federación.
func TestCodegraphPushBestEffortSwallowsFailure(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // central caído: todo push falla
	}))
	defer stub.Close()

	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	if err := engine.ReplaceProjectGraphFrom("", []memory.GraphNode{{Key: "x.go#func:X", Kind: "func", Name: "X", Path: "x.go", SrcFingerprint: "1"}}, nil); err != nil {
		t.Fatal(err)
	}
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 200})

	attempted, ok := s.pushCodeGraphToCentral(context.Background())
	if !attempted {
		t.Error("con sync + team mode el push debería intentarse (attempted=true)")
	}
	if ok {
		t.Error("con el central caído (500), ok debería ser false")
	}
	// El punto de R5/E3: llegamos hasta acá — el fallo del push no entró en pánico ni propagó.
}
