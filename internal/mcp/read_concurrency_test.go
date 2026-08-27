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
		// El motor de diseño: arma un brief leyendo el acervo `musubi-design` con SearchObservations/
		// FTS (búsquedas puras, sin bumpAccess ni ledger). No muta nada; por eso es readOnly y la
		// puede llamar una cabina (F1 · Lienzo como capacidad del cerebro).
		"musubi_design": true,
		// Inventario de la flota (track «Control de flota», S2): lee la tabla `devices` y no
		// escribe nada. El campo `online` se CALCULA al servir (no hay columna ni UPDATE), así
		// que listar la flota no muta ni una fila. readOnly ⇒ la puede llamar una cabina, que es
		// justo el caso de uso: el panel que muestra las máquinas no escribe en ninguna.
		"musubi_fleet_list": true,
		// Telemetría de la flota (S4): lee la columna `last_sample` de `devices` y deriva los
		// porcentajes al servir. No escribe: la muestra la estampa el LATIDO, por la otra puerta.
		"musubi_fleet_metrics": true,
		// La bitácora de ejecución remota (S5): lee device_commands y no escribe. Quien ESCRIBE
		// es musubi_fleet_exec (que encola) y el agente por la otra puerta (que reporta).
		"musubi_fleet_log": true,
		// La bitácora de sesiones de pantalla (S6): lee screen_sessions y no escribe. Quien
		// ESCRIBE es musubi_fleet_screen (que abre la sesión) y el agente por la otra puerta.
		"musubi_fleet_sessions": true,
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
		// El destilador y el afilador (Musubi Renaissance) escriben tarjetas + aristas en el acervo:
		// jamás readOnly. Son lockSelf (I/O externa) pero eso es concurrencia, no autorización.
		"musubi_distill", "musubi_sharpen",
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
