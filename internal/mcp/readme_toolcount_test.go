package mcp

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"musubi/internal/embedding"
)

// TestReadmeToolCountMatchesRegistry mata la clase de drift "el README miente sobre cuántas tools
// hay" (auditoría v0.98.0: decía 27, había 43). Cada aparición de "<N> herramientas" en el README
// DEBE igualar el conteo real del registro. Si agregás/quitás una tool, este test te obliga a
// actualizar el README en el mismo commit.
func TestReadmeToolCountMatchesRegistry(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	want := len(s.tools)

	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("no se pudo leer README.md: %v", err)
	}

	re := regexp.MustCompile(`(\d+)\s+herramientas`)
	matches := re.FindAllStringSubmatch(string(b), -1)
	if len(matches) == 0 {
		t.Fatal("el README no menciona el conteo de herramientas; esperaba al menos una vez \"<N> herramientas\"")
	}
	for _, m := range matches {
		got, _ := strconv.Atoi(m[1])
		if got != want {
			t.Errorf("el README dice %d herramientas pero el registro tiene %d — actualizá el README (o el registro cambió sin querer)", got, want)
		}
	}
}
