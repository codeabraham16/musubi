package mcp

// Tests estructurales del registro de tools. Garantizan que la lista de schemas
// (tools/list) y el mapa de dispatch (tools/call) sean SIEMPRE el mismo conjunto:
// imposible publicar una tool sin handler, o tener un handler huérfano. Esto es lo
// que reemplaza al viejo "mantené sincronizados el switch + la lista + el conteo".

import "testing"

func TestRegistryConsistency(t *testing.T) {
	s := NewMcpServer(nil, "", nil)

	if len(s.tools) == 0 {
		t.Fatal("el registro de tools está vacío")
	}
	if len(s.tools) != len(s.toolIndex) {
		t.Fatalf("desincronización: %d tools en la lista vs %d en el índice de dispatch", len(s.tools), len(s.toolIndex))
	}

	seen := make(map[string]bool, len(s.tools))
	for i := range s.tools {
		entry := s.tools[i]
		name := entry.Name

		if name == "" {
			t.Fatalf("tool en posición %d sin nombre", i)
		}
		if seen[name] {
			t.Fatalf("nombre de tool duplicado en el registro: %q", name)
		}
		seen[name] = true

		if entry.handler == nil {
			t.Fatalf("tool %q sin handler", name)
		}
		// El schema mínimo de MCP exige type=object.
		if entry.InputSchema.Type != "object" {
			t.Fatalf("tool %q con InputSchema.Type=%q (esperaba object)", name, entry.InputSchema.Type)
		}

		h, ok := s.toolIndex[name]
		if !ok {
			t.Fatalf("tool %q está en la lista pero no en el índice de dispatch", name)
		}
		if h == nil {
			t.Fatalf("tool %q mapea a un handler nil en el índice", name)
		}
	}

	// Dirección inversa: ninguna clave del índice puede faltar en la lista.
	for name := range s.toolIndex {
		if !seen[name] {
			t.Fatalf("handler huérfano en el índice (no está en tools/list): %q", name)
		}
	}
}

// TestRegistryDispatchResolves verifica que cada tool publicada en tools/list
// resuelve a un handler vía el camino real de dispatch (handleToolsCall), o sea que
// NINGUNA tool del catálogo puede devolver "Tool not found". No invoca el handler
// (no requiere DB): comprueba el ruteo, no la lógica.
func TestRegistryDispatchResolves(t *testing.T) {
	s := NewMcpServer(nil, "", nil)
	for i := range s.tools {
		name := s.tools[i].Name
		if _, ok := s.toolIndex[name]; !ok {
			t.Errorf("la tool publicada %q no resuelve en el dispatch", name)
		}
	}
}
