package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestTeamModeAutoShared valida C5.2: con team mode ON, una captura SIN scope explícito se persiste
// como 'shared' (se encola en el outbox → fluye al central); con team mode OFF queda 'local' (no se
// encola). Un scope explícito 'local' se respeta aún en team mode (escape hatch). Señal observable:
// el outbox, que solo encola observaciones shared.
func TestTeamModeAutoShared(t *testing.T) {
	newServer := func(teamMode bool) (*McpServer, *memory.DbEngine) {
		engine, err := memory.NewDbEngine(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
			WithMemory(config.MemoryConfig{TeamMode: teamMode}))
		return s, engine
	}
	save := func(t *testing.T, s *McpServer, content, scope string) {
		m := map[string]any{"topic_key": "t/x", "content": content}
		if scope != "" {
			m["scope"] = scope
		}
		args, _ := json.Marshal(m)
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_save_observation", Arguments: args})
		if _, rpcErr := s.handleToolsCall(context.Background(), params); rpcErr != nil {
			t.Fatalf("save: %+v", rpcErr)
		}
	}

	// Team mode ON, sin scope ⇒ shared ⇒ encolado en el outbox.
	s, e := newServer(true)
	defer e.Close()
	save(t, s, "decision de equipo alpha", "")
	if p, _, _, _ := e.OutboxStats(); p != 1 {
		t.Errorf("team mode ON, captura sin scope: outbox pending = %d, esperaba 1 (shared)", p)
	}

	// Team mode OFF, sin scope ⇒ local ⇒ NO encolado (comportamiento histórico).
	s2, e2 := newServer(false)
	defer e2.Close()
	save(t, s2, "nota local beta", "")
	if p, _, _, _ := e2.OutboxStats(); p != 0 {
		t.Errorf("team mode OFF, captura sin scope: outbox pending = %d, esperaba 0 (local)", p)
	}

	// Team mode ON pero scope EXPLÍCITO 'local' ⇒ respetado (escape hatch), NO encolado.
	s3, e3 := newServer(true)
	defer e3.Close()
	save(t, s3, "tanteo privado gamma", "local")
	if p, _, _, _ := e3.OutboxStats(); p != 0 {
		t.Errorf("team mode ON + scope explícito local: outbox pending = %d, esperaba 0 (escape hatch)", p)
	}
}
