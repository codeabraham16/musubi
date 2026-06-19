package mcp

// Demuestra el seam de StorageBackend: el servidor MCP corre contra un backend FALSO
// (sin SQLite), probando que internal/mcp depende del contrato memory.StorageBackend
// y no del *memory.DbEngine concreto. Este desacople es la testabilidad que habilita
// T3.2 — un handler se puede probar en aislamiento con un fake que solo implementa el
// método bajo prueba.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// fakeBackend embebe memory.StorageBackend (nil): satisface el contrato completo en
// tiempo de compilación, pero solo los métodos que sobreescribimos están implementados;
// cualquier otro paniquea si se llama (lo que mantiene el test honesto sobre qué usa
// el handler).
type fakeBackend struct {
	memory.StorageBackend
	pending    []memory.ObsRelation
	pendingErr error
	calls      int
}

func (f *fakeBackend) PendingObsRelations() ([]memory.ObsRelation, error) {
	f.calls++
	return f.pending, f.pendingErr
}

func TestStorageBackendSeam_ConflictsViaFake(t *testing.T) {
	fake := &fakeBackend{
		pending: []memory.ObsRelation{
			{ID: "rel-123", SourceID: "a", TargetID: "b", Relation: "pending", Status: "pending"},
		},
	}

	// El servidor MCP se construye con el fake en lugar de un *DbEngine real.
	s := NewMcpServer(fake, "", nil)

	res, rpcErr := s.handleToolsCall(context.Background(), json.RawMessage(`{"name":"musubi_conflicts","arguments":{}}`))
	if rpcErr != nil {
		t.Fatalf("handleToolsCall devolvió error: %+v", rpcErr)
	}
	if fake.calls != 1 {
		t.Fatalf("esperaba 1 llamada a PendingObsRelations vía el seam, hubo %d", fake.calls)
	}

	// El resultado del handler debe reflejar el dato que devolvió el fake.
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %#v", res)
	}
	text := resp.Content[0].Text
	if !strings.Contains(text, "rel-123") || !strings.Contains(text, `"count": 1`) {
		t.Fatalf("el resultado no refleja el dato del fake backend:\n%s", text)
	}
}

// Aserción extra en tiempo de compilación: el fake satisface el contrato completo,
// confirmando que StorageBackend es implementable por backends ajenos a *DbEngine.
var _ memory.StorageBackend = (*fakeBackend)(nil)
