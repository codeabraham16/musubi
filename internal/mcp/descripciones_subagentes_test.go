package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDescripcionesNoMandanAUnaToolQueNoExiste: musubi_work y musubi_debate le decían al agente
// que lanzara los sub-agentes con el «Task tool + mcpServers:[musubi]». Esa tool hoy se llama
// Agent y nunca tuvo el parámetro. Se barre el catálogo COMPLETO, dormidas incluidas, porque
// el texto vivía en dos descripciones y en dos skills a la vez.
func TestDescripcionesNoMandanAUnaToolQueNoExiste(t *testing.T) {
	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	s := NewMcpServer(nil, "", nil)
	b, err := json.Marshal(s.handleToolsList())
	if err != nil {
		t.Fatalf("marshal tools/list completo: %v", err)
	}
	catalogo := string(b)
	for _, viejo := range []string{"Task tool", "mcpServers:[musubi]"} {
		if strings.Contains(catalogo, viejo) {
			t.Errorf("el catálogo de tools todavía dice %q: el agente no tiene esa tool ni ese parámetro", viejo)
		}
	}
}
