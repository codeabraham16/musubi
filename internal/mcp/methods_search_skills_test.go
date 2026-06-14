package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// catalogJSON construye un JSON de catálogo con las entradas dadas como fragmento JSON.
func catalogJSON(entriesJSON string) string {
	return fmt.Sprintf(`{"catalog_version":1,"entries":[%s]}`, entriesJSON)
}

// entradaGoGin es una entrada válida que aplica a proyectos Go con gin.
const entradaGoGin = `{
	"id":"go-gin",
	"name":"Go — Gin",
	"description":"Framework web Gin para Go",
	"stacks":["Go"],
	"deps":["github.com/gin-gonic/gin"],
	"triggers":["*.go"],
	"capabilities":["go"],
	"tags":["http"],
	"rules_url":"https://example.com/go-gin.md",
	"excerpt":"Usá gin.Context para manejar requests.",
	"source":"musubi-catalog-v1"
}`

// entradaRust es una entrada que NO aplica a un proyecto Go puro.
const entradaRust = `{
	"id":"rust-axum",
	"name":"Rust — Axum",
	"description":"Framework web Axum para Rust",
	"stacks":["Rust"],
	"deps":["axum"],
	"triggers":["*.rs"],
	"capabilities":[],
	"tags":["http"],
	"rules_url":"https://example.com/rust-axum.md",
	"excerpt":"Usá Router::new() de axum.",
	"source":"musubi-catalog-v1"
}`

// newServerConCatalog construye un McpServer apuntando al servidor httptest dado.
func newServerConCatalog(t *testing.T, catalogURL string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })

	root := t.TempDir()
	// Crear go.mod para que DetectStack identifique ecosistema Go.
	gomod := "module ejemplo.com/test\n\ngo 1.26.4\n\nrequire github.com/gin-gonic/gin v1.9.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}
	// Crear un archivo .go para que el trigger *.go coincida.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear main.go: %v", err)
	}

	cfg := config.SourcingConfig{
		Enabled:       true,
		CatalogURL:    catalogURL,
		MaxCandidates: 20,
		CacheSeconds:  3600,
	}
	return NewMcpServer(engine, root, embedding.NoopProvider{}, WithSourcing(cfg))
}

// TestSearchSkillsRetornaCandidatoAplicable verifica que un candidato válido para
// un proyecto Go con gin aparece en la respuesta.
func TestSearchSkillsRetornaCandidatoAplicable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, catalogJSON(entradaGoGin))
	}))
	defer srv.Close()

	s := newServerConCatalog(t, srv.URL)
	res, rpcErr := call(t, s, "musubi_search_skills", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}

	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}
	if !strings.Contains(resp.Content[0].Text, "go-gin") {
		t.Errorf("esperaba candidato go-gin en la respuesta, obtuve: %s", resp.Content[0].Text)
	}
	// Debe incluir evidencia de aplicabilidad.
	if !strings.Contains(resp.Content[0].Text, "Evidence") && !strings.Contains(resp.Content[0].Text, "evidence") {
		t.Errorf("esperaba campo evidence en la respuesta, obtuve: %s", resp.Content[0].Text)
	}
}

// TestSearchSkillsExcluyeNoAplicables verifica que entradas no aplicables (Rust en proyecto Go)
// no aparecen en la respuesta.
func TestSearchSkillsExcluyeNoAplicables(t *testing.T) {
	// Catálogo con dos entradas: go-gin (aplicable) y rust-axum (no aplicable).
	body := catalogJSON(entradaGoGin + "," + entradaRust)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	s := newServerConCatalog(t, srv.URL)
	res, rpcErr := call(t, s, "musubi_search_skills", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}

	text := res.(CallToolResponse).Content[0].Text
	if strings.Contains(text, "rust-axum") {
		t.Errorf("rust-axum no debe aparecer para proyecto Go, obtuve: %s", text)
	}
	if !strings.Contains(text, "go-gin") {
		t.Errorf("go-gin debe aparecer para proyecto Go, obtuve: %s", text)
	}
}

// TestSearchSkillsCatalogCaidoDevuelveTextGracioso verifica que si el catálogo no
// está disponible, la respuesta es un texto de degradación (no un RpcError).
func TestSearchSkillsCatalogCaidoDevuelveTextGracioso(t *testing.T) {
	// URL inválida que va a fallar la conexión.
	s := newServerConCatalog(t, "http://127.0.0.1:1") // puerto sin listener

	res, rpcErr := call(t, s, "musubi_search_skills", map[string]interface{}{})
	// Debe devolver nil RpcError (degradación graciosa).
	if rpcErr != nil {
		t.Fatalf("esperaba degradación graciosa (no RpcError), obtuve: %+v", rpcErr)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada: %+v", res)
	}
	// El texto debe guiar al fallback.
	text := strings.ToLower(resp.Content[0].Text)
	if !strings.Contains(text, "catálogo") && !strings.Contains(text, "catalogo") && !strings.Contains(text, "catalog") {
		t.Errorf("el mensaje de degradación debe mencionar el catálogo, obtuve: %s", resp.Content[0].Text)
	}
}

// TestSearchSkillsSourcingDeshabilitado verifica que cuando sourcing.Enabled=false
// la herramienta devuelve un mensaje descriptivo sin contactar el catálogo.
func TestSearchSkillsSourcingDeshabilitado(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	cfg := config.SourcingConfig{
		Enabled:    false,
		CatalogURL: "http://127.0.0.1:1", // no debe contactarse
	}
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithSourcing(cfg))

	res, rpcErr := call(t, s, "musubi_search_skills", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("esperaba respuesta text (no RpcError) cuando sourcing deshabilitado: %+v", rpcErr)
	}
	text := strings.ToLower(res.(CallToolResponse).Content[0].Text)
	if !strings.Contains(text, "deshabilitado") && !strings.Contains(text, "disabled") {
		t.Errorf("esperaba mensaje de sourcing deshabilitado, obtuve: %s", res.(CallToolResponse).Content[0].Text)
	}
}

// TestSearchSkillsMaxCandidatesRespetado verifica que el cap MaxCandidates se aplica.
func TestSearchSkillsMaxCandidatesRespetado(t *testing.T) {
	// Construir catálogo con 5 entradas Go aplicables.
	var entries []string
	for i := 0; i < 5; i++ {
		entries = append(entries, fmt.Sprintf(`{
			"id":"go-skill-%d",
			"name":"Go Skill %d",
			"description":"desc",
			"stacks":["Go"],
			"deps":[],
			"triggers":["*.go"],
			"capabilities":["go"],
			"tags":[],
			"rules_url":"https://example.com/%d.md",
			"excerpt":"regla",
			"source":"test"
		}`, i, i, i))
	}
	body := fmt.Sprintf(`{"catalog_version":1,"entries":[%s]}`, strings.Join(entries, ","))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module test\n\ngo 1.26.4\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear main.go: %v", err)
	}

	cfg := config.SourcingConfig{
		Enabled:       true,
		CatalogURL:    srv.URL,
		MaxCandidates: 3, // cap de 3 sobre 5 disponibles
		CacheSeconds:  3600,
	}
	s := NewMcpServer(engine, root, embedding.NoopProvider{}, WithSourcing(cfg))

	res, rpcErr := call(t, s, "musubi_search_skills", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}

	// Parsear el JSON resultado para contar candidatos.
	text := res.(CallToolResponse).Content[0].Text
	var candidatos []interface{}
	if err := json.Unmarshal([]byte(text), &candidatos); err != nil {
		t.Fatalf("respuesta no es JSON de slice: %v\nTexto: %s", err, text)
	}
	if len(candidatos) > 3 {
		t.Errorf("esperaba máximo 3 candidatos (MaxCandidates=3), obtuve %d", len(candidatos))
	}
}

// TestSearchSkillsFiltroStack verifica que el parámetro stack filtra los resultados.
func TestSearchSkillsFiltroStack(t *testing.T) {
	body := catalogJSON(entradaGoGin + "," + entradaRust)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer srv.Close()

	// Usar un proyecto que tenga Go detectado; pedir filtro stack="Rust".
	// El resultado debería estar vacío porque el proyecto Go no pasa la gate para Rust.
	s := newServerConCatalog(t, srv.URL)
	res, rpcErr := call(t, s, "musubi_search_skills", map[string]interface{}{"stack": "Rust"})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}
	text := res.(CallToolResponse).Content[0].Text
	if strings.Contains(text, "go-gin") {
		t.Errorf("go-gin no debe aparecer cuando stack=Rust, obtuve: %s", text)
	}
}
