// Package skillsource — test de validación estructural del catálogo seed.
// Carga catalog/index.json desde disco y verifica que cumple el esquema Catalog.
package skillsource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

// slugRE valida el formato de id de una entrada del catálogo.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ecosistemasConocidos son los valores exactos que devuelve detector.DetectStack.
var ecosistemasConocidos = map[string]bool{
	"Go":     true,
	"Node.js": true,
	"Python": true,
	"Rust":   true,
	"Docker": true,
	"Java":   true,
	"Ruby":   true,
	"PHP":    true,
	".NET":   true,
	"Dart":   true,
	"Elixir": true,
	"C/C++":  true,
}

// catalogPath devuelve la ruta absoluta a catalog/index.json relativa a la raíz del repositorio.
// Usa runtime.Caller para ubicar este archivo y sube dos niveles (skillsource → internal → repo root).
func catalogPath(t *testing.T) string {
	t.Helper()
	_, archivoActual, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller no pudo determinar la ruta del archivo")
	}
	// archivoActual = .../internal/skillsource/catalog_data_test.go
	// subir: skillsource → internal → repo root
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(archivoActual)))
	return filepath.Join(repoRoot, "catalog", "index.json")
}

// TestCatalogData valida que catalog/index.json cumple el esquema Catalog y las reglas de calidad.
func TestCatalogData(t *testing.T) {
	ruta := catalogPath(t)

	data, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer catalog/index.json (%s): %v", ruta, err)
	}

	var cat Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("catalog/index.json no es JSON válido: %v", err)
	}

	// catalog_version debe ser no-cero.
	if cat.CatalogVersion == 0 {
		t.Error("catalog_version debe ser > 0")
	}

	// Mínimo 10 entradas.
	if len(cat.Entries) < 10 {
		t.Errorf("se esperan >= 10 entradas en el catálogo, se encontraron %d", len(cat.Entries))
	}

	ids := make(map[string]bool)

	for i, e := range cat.Entries {
		prefijo := func(campo string) string {
			return "entrada[" + e.ID + "]." + campo
		}

		// id: requerido, formato slug, único.
		if e.ID == "" {
			t.Errorf("entrada[%d].id está vacío", i)
			continue
		}
		if !slugRE.MatchString(e.ID) {
			t.Errorf("%s no cumple el formato slug [a-z0-9][a-z0-9-]{0,63}: %q", prefijo("id"), e.ID)
		}
		if ids[e.ID] {
			t.Errorf("%s duplicado", prefijo("id"))
		}
		ids[e.ID] = true

		// name: requerido.
		if e.Name == "" {
			t.Errorf("%s está vacío", prefijo("name"))
		}

		// stacks: al menos uno, valores conocidos.
		if len(e.Stacks) == 0 {
			t.Errorf("%s debe tener al menos un elemento", prefijo("stacks"))
		}
		for _, s := range e.Stacks {
			if !ecosistemasConocidos[s] {
				t.Errorf("%s valor desconocido %q (no coincide con ningún Ecosystem de detector.DetectStack)", prefijo("stacks"), s)
			}
		}

		// triggers: al menos uno.
		if len(e.Triggers) == 0 {
			t.Errorf("%s debe tener al menos un elemento", prefijo("triggers"))
		}

		// rules_url: requerido.
		if e.RulesURL == "" {
			t.Errorf("%s está vacío", prefijo("rules_url"))
		}

		// excerpt: requerido.
		if e.Excerpt == "" {
			t.Errorf("%s está vacío", prefijo("excerpt"))
		}
	}
}
