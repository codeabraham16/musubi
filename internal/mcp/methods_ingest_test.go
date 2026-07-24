package mcp

import (
	"testing"

	"musubi/internal/memory"
)

// TestIngestToolSiempreRegistrada: musubi_ingest_url se registra tanto en el daemon local como en el
// central. La seguridad en infra compartida la da la guarda SSRF del handler (RestrictToPublic), no
// esconder la tool.
func TestIngestToolSiempreRegistrada(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	s := NewMcpServer(engine, t.TempDir(), nil)
	if _, ok := s.toolIndex["musubi_ingest_url"]; !ok {
		t.Fatal("musubi_ingest_url debe estar registrada (local y central)")
	}
}
