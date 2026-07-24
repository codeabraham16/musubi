package mcp

import (
	"testing"

	"musubi/internal/memory"
)

// TestIngestToolSoloEnLocal verifica el gate de seguridad: musubi_ingest_url se registra en el
// daemon local (WithLocalTools) y NO en el central (sin la opción).
func TestIngestToolSoloEnLocal(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	local := NewMcpServer(engine, t.TempDir(), nil, WithLocalTools())
	if _, ok := local.toolIndex["musubi_ingest_url"]; !ok {
		t.Fatal("el daemon local DEBE exponer musubi_ingest_url")
	}

	central := NewMcpServer(engine, t.TempDir(), nil)
	if _, ok := central.toolIndex["musubi_ingest_url"]; ok {
		t.Fatal("el central NO debe exponer musubi_ingest_url (superficie SSRF)")
	}
}
