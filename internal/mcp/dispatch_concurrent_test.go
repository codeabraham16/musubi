package mcp

// Prueba de concurrencia del seam Dispatch (Track 4 / T4.1). Dispara muchos requests
// JSON-RPC en paralelo contra UN servidor + motor compartidos, mezclando lecturas y
// escrituras. Bajo `go test -race` (CI Linux) detecta cualquier carrera de datos; el
// objetivo es probar que Dispatch es seguro para usarse concurrentemente —el
// prerequisito para los transportes de red de Track 4— sin compartir estado mutable.
//
// Las escrituras (musubi_save_observation) ejercitan el camino más riesgoso: Add al
// índice IVF + posible rebuild en background (spawnBackground), serializado por el
// guard rebuilding.CompareAndSwap; las lecturas (search_semantic/keyword, tools/list)
// ejercitan el RLock del índice y el dispatch por mapa.

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"musubi/internal/embedding"
)

func mkReq(id int, method string, params interface{}) JsonRpcRequest {
	var raw json.RawMessage
	if params != nil {
		b, _ := json.Marshal(params)
		raw = b
	}
	return JsonRpcRequest{JsonRpc: "2.0", ID: id, Method: method, Params: raw}
}

func callReqJSON(name string, args map[string]interface{}) interface{} {
	ab, _ := json.Marshal(args)
	pb, _ := json.Marshal(CallToolRequest{Name: name, Arguments: ab})
	return json.RawMessage(pb)
}

func TestDispatchConcurrentSafe(t *testing.T) {
	s := newTestServer(t, fakeEmbedder{vec: []float32{0.11, 0.22, 0.33, 0.44}})

	// Semillas iniciales para que recall/search tengan datos.
	for i := 0; i < 20; i++ {
		req := mkReq(i, "tools/call", callReqJSON("musubi_save_observation", map[string]interface{}{
			"topic_key": fmt.Sprintf("seed/%d", i%4),
			"content":   fmt.Sprintf("observación semilla número %d sobre arquitectura y memoria", i),
		}))
		if resp, ok := s.Dispatch(context.Background(), req); !ok || resp.Error != nil {
			t.Fatalf("seed %d falló: ok=%v err=%+v", i, ok, resp.Error)
		}
	}

	// Mezcla de operaciones concurrentes: cada índice elige una variante determinista.
	ops := []func(id int) JsonRpcRequest{
		func(id int) JsonRpcRequest { return mkReq(id, "tools/list", nil) },
		func(id int) JsonRpcRequest {
			return mkReq(id, "tools/call", nil) // params nil → invalid params (camino de error), no debe paniquear
		},
		func(id int) JsonRpcRequest {
			return mkReq(id, "tools/call", callReqJSON("musubi_save_observation", map[string]interface{}{
				"topic_key": fmt.Sprintf("conc/%d", id%5),
				"content":   fmt.Sprintf("escritura concurrente %d con contenido suficiente para indexar", id),
			}))
		},
		func(id int) JsonRpcRequest {
			return mkReq(id, "tools/call", callReqJSON("musubi_search_semantic", map[string]interface{}{
				"query": "arquitectura memoria", "limit": 5,
			}))
		},
		func(id int) JsonRpcRequest {
			return mkReq(id, "tools/call", callReqJSON("musubi_search_keyword", map[string]interface{}{
				"query_text": "arquitectura", "limit": 5,
			}))
		},
		func(id int) JsonRpcRequest {
			return mkReq(id, "tools/call", callReqJSON("musubi_recall", map[string]interface{}{
				"query": "memoria", "token_budget": 500,
			}))
		},
		func(id int) JsonRpcRequest {
			return mkReq(id, "tools/call", callReqJSON("musubi_tokens", map[string]interface{}{"action": "status"}))
		},
	}

	const workers = 64
	var wg sync.WaitGroup
	var mu sync.Mutex
	var badResp int
	var toolsListOK int

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 6; i++ {
				id := w*100 + i
				req := ops[(w+i)%len(ops)](id)
				resp, ok := s.Dispatch(context.Background(), req)
				if !ok || resp.JsonRpc != "2.0" {
					mu.Lock()
					badResp++
					mu.Unlock()
					continue
				}
				// tools/list debe devolver siempre el catálogo completo, incluso bajo carga.
				if req.Method == "tools/list" {
					if m, isMap := resp.Result.(map[string]interface{}); isMap {
						if tools, isSlice := m["tools"].([]Tool); isSlice && len(tools) == 30 {
							mu.Lock()
							toolsListOK++
							mu.Unlock()
						}
					}
				}
			}
		}(w)
	}
	wg.Wait()

	if badResp != 0 {
		t.Errorf("%d respuestas malformadas bajo concurrencia (esperaba 0)", badResp)
	}
	if toolsListOK == 0 {
		t.Error("ninguna respuesta tools/list válida observada bajo concurrencia")
	}
}

// Sanity: el embedder noop también sirve para el test (sin búsqueda semántica real).
var _ embedding.Provider = embedding.NoopProvider{}
