package skillsource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/detector"
)

// --- Helpers de fixture ---

// escribirArchivo crea un archivo con el contenido dado.
func escribirArchivo(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatalf("no se pudo crear directorio: %v", err)
	}
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatalf("no se pudo escribir archivo %s: %v", ruta, err)
	}
}

// entradaMinima devuelve una CatalogEntry válida para usar en pruebas.
func entradaMinima(id string) CatalogEntry {
	return CatalogEntry{
		ID:          id,
		Name:        "Skill " + id,
		Description: "Descripción de " + id,
		Stacks:      []string{"Go"},
		Triggers:    []string{"*.go"},
		RulesURL:    "https://example.com/" + id + ".md",
		Source:      "musubi-catalog-v1",
	}
}

// --- Tests de FetchCatalog ---

// TestFetchCatalogExitoso verifica que un servidor que devuelve 200 con JSON válido
// retorna las entradas correctamente y sin error.
func TestFetchCatalogExitoso(t *testing.T) {
	entradas := []CatalogEntry{
		entradaMinima("go-gin"),
		entradaMinima("go-echo"),
	}
	body, err := json.Marshal(Catalog{CatalogVersion: 1, Entries: entradas})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	ctx := context.Background()
	cat, err := FetchCatalog(ctx, srv.URL)
	if err != nil {
		t.Fatalf("FetchCatalog error inesperado: %v", err)
	}
	if len(cat.Entries) != 2 {
		t.Errorf("esperaba 2 entradas, obtuve %d", len(cat.Entries))
	}
	if cat.Entries[0].ID != "go-gin" {
		t.Errorf("primera entrada: esperaba id 'go-gin', obtuve %q", cat.Entries[0].ID)
	}
}

// TestFetchCatalog500 verifica que un servidor que devuelve 500 produce error no fatal.
func TestFetchCatalog500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error interno", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx := context.Background()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic inesperado con servidor 500: %v", r)
			}
		}()
		_, err := FetchCatalog(ctx, srv.URL)
		if err == nil {
			t.Error("FetchCatalog debería devolver error con respuesta 500")
		}
	}()
}

// TestFetchCatalogTimeout verifica que un servidor lento agota el contexto y devuelve error.
func TestFetchCatalogTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simular servidor lento: esperar más que el timeout del contexto
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := FetchCatalog(ctx, srv.URL)
	if err == nil {
		t.Error("FetchCatalog debería devolver error cuando el contexto se agota")
	}
}

// TestFetchCatalogJSONMalformado verifica que un body no-JSON produce error no fatal.
func TestFetchCatalogJSONMalformado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("esto no es json {{{"))
	}))
	defer srv.Close()

	ctx := context.Background()
	_, err := FetchCatalog(ctx, srv.URL)
	if err == nil {
		t.Error("FetchCatalog debería devolver error con body JSON inválido")
	}
}

// TestFetchCatalogVersionDesconocida verifica que una versión de catálogo desconocida
// emite logx.Warn pero aún retorna las entradas válidas (best-effort).
func TestFetchCatalogVersionDesconocida(t *testing.T) {
	entradas := []CatalogEntry{entradaMinima("go-gin")}
	body, err := json.Marshal(map[string]any{
		"catalog_version": 99,
		"entries":         entradas,
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	ctx := context.Background()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic inesperado con versión desconocida: %v", r)
			}
		}()
		cat, err := FetchCatalog(ctx, srv.URL)
		if err != nil {
			t.Errorf("versión desconocida no debería producir error fatal: %v", err)
		}
		if len(cat.Entries) == 0 {
			t.Error("esperaba al menos una entrada con versión desconocida (best-effort)")
		}
	}()
}

// TestFetchCatalogEntradaMalformadaOmitida verifica que una entrada inválida (id vacío)
// no aborta el parseo — las entradas válidas restantes se devuelven.
func TestFetchCatalogEntradaMalformadaOmitida(t *testing.T) {
	body, err := json.Marshal(map[string]any{
		"catalog_version": 1,
		"entries": []any{
			map[string]any{"id": "go-gin", "name": "Gin", "stacks": []string{"Go"}, "rules_url": "https://x.com/go-gin.md", "triggers": []string{"*.go"}, "source": "v1"},
			map[string]any{"id": "", "name": "Entrada Rota", "stacks": []string{"Go"}}, // inválida: sin id
			map[string]any{"id": "go-echo", "name": "Echo", "stacks": []string{"Go"}, "rules_url": "https://x.com/go-echo.md", "triggers": []string{"*.go"}, "source": "v1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	ctx := context.Background()
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic inesperado con entrada inválida: %v", r)
			}
		}()
		cat, err := FetchCatalog(ctx, srv.URL)
		if err != nil {
			t.Errorf("entrada inválida no debería producir error fatal: %v", err)
		}
		if len(cat.Entries) != 2 {
			t.Errorf("esperaba 2 entradas válidas (la inválida omitida), obtuve %d", len(cat.Entries))
		}
	}()
}

// --- Tests de FilterCatalog ---

// TestFilterCatalogSoloAplicables verifica que FilterCatalog devuelve únicamente
// las entradas que pasan el gate de aplicabilidad.
func TestFilterCatalogSoloAplicables(t *testing.T) {
	root := t.TempDir()
	// Crear un archivo .go para que el trigger *.go coincida
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	stacks := []detector.StackResult{
		{Ecosystem: "Go"},
	}
	deps := map[string][]string{"Go": {"github.com/gin-gonic/gin"}}

	entradas := []CatalogEntry{
		{ID: "go-gin", Stacks: []string{"Go"}, Deps: []string{"github.com/gin-gonic/gin"}, Triggers: []string{"*.go"}, RulesURL: "https://x.com/a.md"},
		{ID: "rust-axum", Stacks: []string{"Rust"}, Deps: []string{"axum"}, Triggers: []string{"*.rs"}, RulesURL: "https://x.com/b.md"},
	}
	cat := Catalog{CatalogVersion: 1, Entries: entradas}

	cands := FilterCatalog(cat, root, deps, stacks, 10)
	if len(cands) != 1 {
		t.Errorf("esperaba 1 candidato aplicable, obtuve %d", len(cands))
	}
	if len(cands) > 0 && cands[0].Entry.ID != "go-gin" {
		t.Errorf("esperaba candidato 'go-gin', obtuve %q", cands[0].Entry.ID)
	}
}

// TestFilterCatalogCapRespetado verifica que el límite maxCandidates se respeta.
func TestFilterCatalogCapRespetado(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}}

	// Crear varias entradas aplicables (solo stack + trigger, sin deps requeridas)
	var entradas []CatalogEntry
	for i := 0; i < 5; i++ {
		entradas = append(entradas, CatalogEntry{
			ID:       "go-skill-" + string(rune('a'+i)),
			Stacks:   []string{"Go"},
			Triggers: []string{"*.go"},
			RulesURL: "https://x.com/x.md",
		})
	}
	cat := Catalog{CatalogVersion: 1, Entries: entradas}

	cands := FilterCatalog(cat, root, deps, stacks, 3)
	if len(cands) != 3 {
		t.Errorf("esperaba 3 candidatos (cap=3), obtuve %d", len(cands))
	}
}

// TestFilterCatalogVacioSiNinguno verifica que se retorna slice vacío cuando
// ninguna entrada pasa el gate.
func TestFilterCatalogVacioSiNinguno(t *testing.T) {
	root := t.TempDir()
	// No hay archivos .go ni .rs en root

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}}

	entradas := []CatalogEntry{
		{ID: "rust-axum", Stacks: []string{"Rust"}, Triggers: []string{"*.rs"}, RulesURL: "https://x.com/b.md"},
	}
	cat := Catalog{CatalogVersion: 1, Entries: entradas}

	cands := FilterCatalog(cat, root, deps, stacks, 10)
	if len(cands) != 0 {
		t.Errorf("esperaba 0 candidatos, obtuve %d", len(cands))
	}
}
