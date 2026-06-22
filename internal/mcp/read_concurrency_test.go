package mcp

// Tests de la clasificación read/write del dispatch (Track 4 / T4.5). La corrección de
// la concurrencia depende de que SOLO tools verificadas como pura-lectura estén
// marcadas readOnly: marcar una tool que muta (bumpAccess, ledger, DB) sería un bug de
// read-modify-write bajo concurrencia que -race NO detecta. Este test congela la
// clasificación.

import (
	"context"
	"sync"
	"testing"

	"musubi/internal/embedding"
)

func TestToolReadOnlyClassification(t *testing.T) {
	s := NewMcpServer(nil, "", nil)

	// El conjunto EXACTO de tools de solo-lectura (verificadas: sin DB write, sin
	// bumpAccess, sin LedgerAdd ni en el handler ni en el método del motor).
	wantReadOnly := map[string]bool{
		"musubi_search_semantic": true,
		"musubi_search_keyword":  true,
		"musubi_recall_facts":    true,
		"musubi_entity_context":  true,
		"musubi_conflicts":       true,
		"musubi_detect_stack":    true,
		"musubi_search_skills":   true,
		"musubi_discover_skills": true,
		"musubi_insights":        true,
	}
	for i := range s.tools {
		name := s.tools[i].Name
		if s.tools[i].readOnly != wantReadOnly[name] {
			t.Errorf("tool %q readOnly=%v, esperaba %v", name, s.tools[i].readOnly, wantReadOnly[name])
		}
	}

	// Guard de regresión: estas tools MUTAN estado y NUNCA deben marcarse readOnly
	// (recall/memory_expand hacen bumpAccess; recall_code hace LedgerAdd).
	mustWrite := []string{
		"musubi_recall", "musubi_memory_expand", "musubi_recall_code",
		"musubi_save_observation", "musubi_maintain", "musubi_judge", "musubi_tokens",
		"musubi_save_fact", "musubi_work", "musubi_workflow", "musubi_phase",
	}
	for _, name := range mustWrite {
		if s.toolReadOnly[name] {
			t.Errorf("tool %q marcada readOnly pero MUTA estado (riesgo lost-update RMW)", name)
		}
	}
}

// TestConcurrentReadDispatch dispara muchas tools de solo-lectura en paralelo vía
// Dispatch: deben correr concurrentes (RLock) sin deadlock ni error. Bajo -race (CI)
// también valida que no haya carreras de memoria en el camino de lectura.
func TestConcurrentReadDispatch(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// Semilla mínima para que las búsquedas tengan algo que mirar.
	s.Dispatch(context.Background(), mkReq(0, "tools/call", callReqJSON("musubi_save_observation", map[string]interface{}{
		"topic_key": "ro/seed", "content": "semilla para concurrencia de lectura",
	})))

	reads := []JsonRpcRequest{
		mkReq(1, "tools/call", callReqJSON("musubi_search_keyword", map[string]interface{}{"query_text": "semilla"})),
		mkReq(2, "tools/call", callReqJSON("musubi_recall_facts", map[string]interface{}{"entity": "x"})),
		mkReq(3, "tools/call", callReqJSON("musubi_conflicts", map[string]interface{}{})),
		mkReq(4, "tools/list", nil),
	}

	var wg sync.WaitGroup
	for w := 0; w < 32; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			req := reads[w%len(reads)]
			if resp, ok := s.Dispatch(context.Background(), req); !ok || resp.JsonRpc != "2.0" {
				t.Errorf("dispatch de lectura concurrente falló: ok=%v", ok)
			}
		}(w)
	}
	wg.Wait()
}
