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

	// LOS DOS READMEs. Antes esta guarda sólo miraba el español, y el inglés se fue a la deriva
	// sin que nadie se enterara: llegó a decir 27 cuando había 66, con una tabla que se había
	// quedado en 8 dominios de 16. Una guarda que cubre un solo idioma enseña que el otro no
	// importa.
	for _, doc := range []struct{ archivo, palabra string }{
		{"../../README.md", "herramientas"},
		{"../../README.en.md", "tools"},
	} {
		b, err := os.ReadFile(doc.archivo)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", doc.archivo, err)
		}
		re := regexp.MustCompile(`\*\*(\d+)\s+` + doc.palabra + `\*\*|(\d+)\s+` + doc.palabra)
		matches := re.FindAllStringSubmatch(string(b), -1)
		if len(matches) == 0 {
			t.Errorf("%s no menciona el conteo de herramientas", doc.archivo)
			continue
		}
		for _, m := range matches {
			crudo := m[1]
			if crudo == "" {
				crudo = m[2]
			}
			got, _ := strconv.Atoi(crudo)
			if got != want {
				t.Errorf("%s dice %d %s pero el registro tiene %d — actualizá el README (o el registro cambió sin querer)",
					doc.archivo, got, doc.palabra, want)
			}
		}
	}
}
