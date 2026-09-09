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

	// SE CUENTA LO QUE tools/list DEVUELVE, no lo que el registro contiene. Son dos números
	// distintos y los dos son ciertos: hay 84 tools registradas y el catálogo lista 75, porque
	// nueve están DORMIDAS (toolEntry.dormant). Una dormida sigue siendo despachable por nombre;
	// lo único que pierde es el lugar en el listado.
	//
	// Acá iba `len(s.tools)`, o sea las registradas. Con eso, un README que decía la verdad
	// —«el servidor expone 75 herramientas»— ponía el test en ROJO, y la única forma de tenerlo
	// verde era escribir 84 y dejar al lector esperando nueve tools que su agente nunca le va a
	// ofrecer. El test empujaba a mentir; ahora afirma el número del que habla el README.
	t.Setenv("MUSUBI_TOOLS_ALL", "") // el catálogo por defecto, sin la salida de emergencia
	listado, ok := s.handleToolsList().(map[string]interface{})
	if !ok {
		t.Fatal("tools/list no devolvió el mapa esperado")
	}
	expuestas, ok := listado["tools"].([]Tool)
	if !ok {
		t.Fatal("tools/list no devolvió una lista de Tool")
	}
	want := len(expuestas)
	// Sin este piso, un registro vacío haría pasar el test con cualquier README que dijera 0.
	if want == 0 {
		t.Fatal("el catálogo salió vacío: el test no estaría verificando nada")
	}

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
