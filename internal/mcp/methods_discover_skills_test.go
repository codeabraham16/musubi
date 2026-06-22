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

// respMarketplace arma el sobre JSON del endpoint de búsqueda del marketplace.
func respMarketplace(skillsJSON string) string {
	return fmt.Sprintf(`{"success":true,"data":{"skills":[%s],"pagination":{"total":1}}}`, skillsJSON)
}

const skillDescubierta = `{
	"id":"acme-go-http-skill-md",
	"name":"go-http-patterns",
	"author":"acme",
	"description":"Patrones HTTP idiomáticos en Go.",
	"githubUrl":"https://github.com/acme/skills/tree/main/go-http",
	"skillUrl":"https://skillsmp.com/skills/acme-go-http",
	"stars":42,
	"updatedAt":"1781667763"
}`

// newServerConMarketplace construye un McpServer Go-detectado apuntando al marketplace dado.
// enabled controla sourcing.marketplace_enabled.
func newServerConMarketplace(t *testing.T, marketURL string, enabled bool) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })

	root := t.TempDir()
	// go.mod para que DetectStack identifique ecosistema Go (deriva la query).
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}

	cfg := config.SourcingConfig{
		Enabled:            true,
		MarketplaceEnabled: enabled,
		MarketplaceURL:     marketURL,
	}
	return NewMcpServer(engine, root, embedding.NoopProvider{}, WithSourcing(cfg))
}

// TestDiscoverSkillsDeshabilitado: con marketplace off, devuelve guía (no error).
func TestDiscoverSkillsDeshabilitado(t *testing.T) {
	s := newServerConMarketplace(t, "https://skillsmp.com", false)
	res, rpcErr := call(t, s, "musubi_discover_skills", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("esperaba texto (no RpcError) con marketplace deshabilitado: %+v", rpcErr)
	}
	resp := res.(CallToolResponse)
	if !strings.Contains(resp.Content[0].Text, "deshabilitado") {
		t.Errorf("esperaba mensaje de deshabilitado, obtuve: %s", resp.Content[0].Text)
	}
}

// TestDiscoverSkillsRetornaResultados: con marketplace on y sin query, deriva la query del
// stack (Go) y devuelve los resultados con su githubUrl y la nota de "revisá antes de instalar".
func TestDiscoverSkillsRetornaResultados(t *testing.T) {
	var gotQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query().Get("q")
		fmt.Fprint(w, respMarketplace(skillDescubierta))
	}))
	defer srv.Close()

	s := newServerConMarketplace(t, srv.URL, true)
	res, rpcErr := call(t, s, "musubi_discover_skills", map[string]interface{}{})
	if rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}
	resp := res.(CallToolResponse)
	txt := resp.Content[0].Text

	// La query se derivó del stack: debe contener "Go".
	if !strings.Contains(gotQ, "Go") {
		t.Errorf("esperaba query derivada del stack con 'Go', el marketplace recibió q=%q", gotQ)
	}
	if !strings.Contains(txt, "go-http-patterns") {
		t.Errorf("esperaba la skill descubierta en la respuesta, obtuve: %s", txt)
	}
	if !strings.Contains(txt, "githubUrl") {
		t.Errorf("esperaba el githubUrl para revisión, obtuve: %s", txt)
	}
	if !strings.Contains(strings.ToLower(txt), "revis") {
		t.Errorf("esperaba la nota de revisar antes de instalar, obtuve: %s", txt)
	}
}

// TestDiscoverSkillsQueryExplicita: un query explícito tiene prioridad sobre el stack.
func TestDiscoverSkillsQueryExplicita(t *testing.T) {
	var gotQ string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query().Get("q")
		fmt.Fprint(w, respMarketplace(skillDescubierta))
	}))
	defer srv.Close()

	s := newServerConMarketplace(t, srv.URL, true)
	if _, rpcErr := call(t, s, "musubi_discover_skills", map[string]interface{}{"query": "kubernetes operator"}); rpcErr != nil {
		t.Fatalf("error inesperado: %+v", rpcErr)
	}
	if gotQ != "kubernetes operator" {
		t.Errorf("esperaba la query explícita, el marketplace recibió q=%q", gotQ)
	}
}

// TestDiscoverSkillsMarketplaceCaido: si el marketplace responde 500, degrada a texto.
func TestDiscoverSkillsMarketplaceCaido(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	s := newServerConMarketplace(t, srv.URL, true)
	res, rpcErr := call(t, s, "musubi_discover_skills", map[string]interface{}{"query": "go"})
	if rpcErr != nil {
		t.Fatalf("esperaba texto (no RpcError) con marketplace caído: %+v", rpcErr)
	}
	resp := res.(CallToolResponse)
	if !strings.Contains(resp.Content[0].Text, "no está disponible") {
		t.Errorf("esperaba mensaje de degradación, obtuve: %s", resp.Content[0].Text)
	}
	// jsonResult no debe haberse usado: verificamos que es texto plano, no JSON con skills.
	if json.Valid([]byte(resp.Content[0].Text)) && strings.Contains(resp.Content[0].Text, "\"skills\"") {
		t.Errorf("no esperaba JSON de skills en degradación")
	}
}
