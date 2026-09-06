package mcp

// Test de golden snapshot del catálogo tools/list. Congela la salida JSON exacta
// (nombres, descripciones, schemas y ORDEN) para que cualquier refactor del
// registro de tools sea provablemente byte-idéntico. Para regenerar tras un
// cambio intencional de tools: go test ./internal/mcp -run TestToolsListGolden -update

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// normalizeEOL quita los CR para que la comparación sea robusta al fin de línea del
// working tree (git autocrlf en Windows deja CRLF aunque el repo guarde LF).
func normalizeEOL(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

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
	if !bytes.Equal(normalizeEOL(got), normalizeEOL(want)) {
		t.Errorf("la salida de tools/list cambió respecto del golden.\n" +
			"Si el cambio es intencional, regenerá con:\n" +
			"  go test ./internal/mcp -run TestToolsListGolden -update")
	}
}

// TestToolsListGoldenCompleto congela el catálogo COMPLETO, tools dormidas incluidas.
//
// 🔴 POR QUÉ HACEN FALTA DOS GOLDEN Y NO UNO. El golden de arriba captura
// `handleToolsList()`, que FILTRA las tools dormidas. Son nueve. Eso significa que la red
// de seguridad de la regla 5 del repo —«al cambiar una tool hay que regenerar el golden o
// el build queda verde y mal»— es estructuralmente CIEGA para esas nueve: se le puede
// cambiar el contrato a una tool dormida, agregarle campos obligatorios incluso, y el
// golden no se mueve un byte.
//
// Se descubrió midiendo, no leyendo: `musubi_debate` ganó dos campos OBLIGATORIOS
// (model, evidence) y un tercero opcional (gated_choice), se corrió el golden con -update
// y el archivo quedó idéntico. Un guardián que no puede ponerse rojo se ve igual que uno
// que funciona — que es justo el defecto que este repo persigue en todos lados.
//
// Dormir una tool es una decisión sobre su VISIBILIDAD; no debería ser también una
// decisión sobre si su contrato está protegido.
func TestToolsListGoldenCompleto(t *testing.T) {
	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	s := NewMcpServer(nil, "", nil)
	got, err := json.MarshalIndent(s.handleToolsList(), "", "  ")
	if err != nil {
		t.Fatalf("marshal tools/list completo: %v", err)
	}

	golden := filepath.Join("testdata", "toolslist-completo.golden.json")
	if *updateGolden {
		if wErr := os.WriteFile(golden, got, 0o644); wErr != nil {
			t.Fatalf("escribir golden completo: %v", wErr)
		}
		t.Logf("golden completo regenerado: %s", golden)
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("leer golden completo (%s): %v — corré con -update para generarlo", golden, err)
	}
	if !bytes.Equal(normalizeEOL(got), normalizeEOL(want)) {
		t.Errorf("la salida de tools/list COMPLETO (dormidas incluidas) cambió respecto del golden.\n" +
			"Si el cambio es intencional, regenerá con:\n" +
			"  go test ./internal/mcp -run TestToolsListGolden -update")
	}
}

// TestElGoldenCompletoVeMasQueElVisible es la prueba de que el golden nuevo NO es una copia
// del otro: si algún día dejaran de existir tools dormidas, este test avisa que el segundo
// golden pasó a ser redundante en vez de dejarlo ahí dando una falsa sensación de cobertura.
func TestElGoldenCompletoVeMasQueElVisible(t *testing.T) {
	// handleToolsList devuelve interface{}, así que se cuenta sobre el JSON: es la misma
	// superficie que congela el golden, que es justo lo que se quiere comparar.
	cuantas := func() int {
		b, err := json.Marshal(NewMcpServer(nil, "", nil).handleToolsList())
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return bytes.Count(b, []byte(`"name":`))
	}
	visible := cuantas()
	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	completo := cuantas()
	if completo <= visible {
		t.Errorf("el catálogo completo (%d) tiene que ver MÁS que el visible (%d): si son iguales, "+
			"o no quedan tools dormidas o MUSUBI_TOOLS_ALL dejó de funcionar, y en los dos casos "+
			"este golden dejó de cubrir lo que dice cubrir", completo, visible)
	}
}
