package mcp

// Test de golden snapshot del catálogo tools/list. Congela la salida JSON exacta
// (nombres, descripciones, schemas y ORDEN) para que cualquier refactor del
// registro de tools sea provablemente byte-idéntico. Para regenerar tras un
// cambio intencional de tools: go test ./internal/mcp -run TestToolsListGolden -update

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenera los golden files de este paquete")

func TestToolsListGolden(t *testing.T) {
	s := NewMcpServer(nil, "", nil)
	got, err := json.MarshalIndent(s.handleToolsList(), "", "  ")
	if err != nil {
		t.Fatalf("marshal tools/list: %v", err)
	}

	golden := filepath.Join("testdata", "toolslist.golden.json")
	if *updateGolden {
		if mkErr := os.MkdirAll("testdata", 0o755); mkErr != nil {
			t.Fatalf("mkdir testdata: %v", mkErr)
		}
		if wErr := os.WriteFile(golden, got, 0o644); wErr != nil {
			t.Fatalf("escribir golden: %v", wErr)
		}
		t.Logf("golden regenerado: %s", golden)
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("leer golden (%s): %v — corré con -update para generarlo", golden, err)
	}
	if string(got) != string(want) {
		t.Errorf("la salida de tools/list cambió respecto del golden.\n" +
			"Si el cambio es intencional, regenerá con:\n" +
			"  go test ./internal/mcp -run TestToolsListGolden -update")
	}
}
