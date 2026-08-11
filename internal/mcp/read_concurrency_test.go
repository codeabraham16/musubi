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
		"musubi_detect_changes":  true,
		"musubi_sync_status":     true,
		"musubi_sync_pull":       true,
		"musubi_code_graph":      true,
		"musubi_impact":          true,
		"musubi_map":             true,
		"musubi_code_context":    true,
		"musubi_code_graph_viz":  true,
		"musubi_brain_graph":     true,
		"musubi_whoami":          true,
		// Contadores en memoria del proceso; no toca la DB ni bumpea nada (F5, invariante D8).
		"musubi_cognition_stats": true,
		// Lee el ledger de uso y no escribe nada: sin bumpAccess, sin ledger de tokens (F0).
		"musubi_tool_usage": true,
		// Agrega ledger, conflictos, memoria y grafo para puntuar la instalación: cinco lecturas
		// y ninguna escritura — no bumpea acceso ni marca nada (F3 · madurez medida).
		"musubi_readiness": true,
		// Lee .musubi/skills/*.yaml del disco: no toca la DB, no bumpea, no escribe (F5.1).
		"musubi_list_skills": true,
		// Lee los contadores del arsenal y el disco (§7 «Forja global»). Los conteos que las
		// otras tools provocan no se escriben en el handler: van al buffer del ledger y bajan
		// desde otra goroutine, justamente para no escribir con dispatchMu tomado.
		"musubi_skill_usage": true,
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
		// Federación del arsenal: promote escribe en el CENTRAL, install escribe en el disco
		// del proyecto. Marcarlas readOnly las metería en la clase de lectura aislada, que es
		// justo lo que no son.
		"musubi_promote_skill", "musubi_install_skill",
		"musubi_save_fact", "musubi_work", "musubi_workflow", "musubi_phase",
		// musubi_sdd hace un RMW del blob del run (CompleteWorkflowStep) + persiste el
		// artefacto: su corrección depende del Lock exclusivo de dispatchMu. Marcarla readOnly
		// la pondría bajo RLock y reintroduciría el lost-update del complete (auditoría #5).
		"musubi_sdd",
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
