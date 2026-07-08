package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

func TestResolveTelemetryCapturesErrorFix(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	resolve := func(id int) *RpcError {
		raw, _ := json.Marshal(map[string]int{"id": id})
		_, rpcErr := s.toolResolveTelemetry(raw)
		return rpcErr
	}

	// Con parche → captura el par error→fix.
	if err := engine.SaveTelemetryLog("a.go", "undefined: X", "agregar import Y"); err != nil {
		t.Fatal(err)
	}
	logs, _ := engine.GetUnresolvedTelemetryLogs()
	if rpcErr := resolve(logs[0].ID); rpcErr != nil {
		t.Fatalf("resolve con patch: %+v", rpcErr)
	}
	res, _ := engine.SearchObservationsFTS(context.Background(), "Arreglado", 10)
	if len(res) == 0 {
		t.Fatal("esperaba capturar el par error→fix como memoria")
	}

	// Sin parche → NO captura (anti-ruido).
	if err := engine.SaveTelemetryLog("b.go", "problema distinto zzz", ""); err != nil {
		t.Fatal(err)
	}
	logs2, _ := engine.GetUnresolvedTelemetryLogs()
	if rpcErr := resolve(logs2[0].ID); rpcErr != nil {
		t.Fatalf("resolve sin patch: %+v", rpcErr)
	}
	if res2, _ := engine.SearchObservationsFTS(context.Background(), "zzz", 10); len(res2) != 0 {
		t.Fatalf("sin parche no debía capturar; encontró %d", len(res2))
	}

	// Id inexistente → error (comportamiento preservado).
	if rpcErr := resolve(999999); rpcErr == nil {
		t.Fatal("esperaba error por id inexistente")
	}
}
