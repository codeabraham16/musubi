package skillsource

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// respuestaMarketplace arma el sobre JSON que devuelve el endpoint de búsqueda del
// marketplace, con las skills dadas como fragmento JSON.
func respuestaMarketplace(skillsJSON string) string {
	return fmt.Sprintf(`{"success":true,"data":{"skills":[%s],"pagination":{"total":1}}}`, skillsJSON)
}

const skillGolang = `{
	"id":"sushichan044-dotfiles-golang-data-structures-skill-md",
	"name":"golang-data-structures",
	"author":"sushichan044",
	"description":"Golang data structures. Use when optimizing Go data structures.",
	"githubUrl":"https://github.com/sushichan044/dotfiles/tree/main/.agents/skills/golang-data-structures",
	"skillUrl":"https://skillsmp.com/creators/sushichan044/dotfiles/golang-data-structures",
	"stars":15,
	"updatedAt":"1781667763"
}`

// TestFetchMarketplaceSkillsParsea verifica el parseo del sobre y el mapeo de campos.
func TestFetchMarketplaceSkillsParsea(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, respuestaMarketplace(skillGolang))
	}))
	defer srv.Close()

	skills, err := FetchMarketplaceSkills(context.Background(), srv.URL, "", "golang", 10)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("esperaba 1 skill, obtuve %d", len(skills))
	}
	s := skills[0]
	if s.Name != "golang-data-structures" {
		t.Errorf("Name: %q", s.Name)
	}
	if s.Author != "sushichan044" {
		t.Errorf("Author: %q", s.Author)
	}
	if !strings.HasPrefix(s.GithubURL, "https://github.com/") {
		t.Errorf("GithubURL: %q", s.GithubURL)
	}
	if s.Stars != 15 {
		t.Errorf("Stars: %d", s.Stars)
	}
}

// TestFetchMarketplaceArmaRequest verifica path, query (q, limit acotado, sortBy) y el
// header Authorization cuando hay API key.
func TestFetchMarketplaceArmaRequest(t *testing.T) {
	var gotPath, gotQ, gotLimit, gotSort, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQ = r.URL.Query().Get("q")
		gotLimit = r.URL.Query().Get("limit")
		gotSort = r.URL.Query().Get("sortBy")
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, respuestaMarketplace(skillGolang))
	}))
	defer srv.Close()

	// limit 999 debe acotarse a 100; con API key se envía Authorization: Bearer.
	if _, err := FetchMarketplaceSkills(context.Background(), srv.URL, "sk_live_xyz", "go web", 999); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if gotPath != marketplaceSearchPath {
		t.Errorf("path: %q, esperaba %q", gotPath, marketplaceSearchPath)
	}
	if gotQ != "go web" {
		t.Errorf("q: %q", gotQ)
	}
	if gotLimit != "100" {
		t.Errorf("limit: %q, esperaba 100 (acotado)", gotLimit)
	}
	if gotSort != "stars" {
		t.Errorf("sortBy: %q, esperaba stars", gotSort)
	}
	if gotAuth != "Bearer sk_live_xyz" {
		t.Errorf("Authorization: %q", gotAuth)
	}
}

// TestFetchMarketplaceSinAPIKeySinAuth verifica que sin API key no se manda Authorization
// (se usa el tier anónimo).
func TestFetchMarketplaceSinAPIKeySinAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, respuestaMarketplace(skillGolang))
	}))
	defer srv.Close()

	if _, err := FetchMarketplaceSkills(context.Background(), srv.URL, "", "go", 5); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if gotAuth != "" {
		t.Errorf("no esperaba Authorization sin API key, obtuve: %q", gotAuth)
	}
}

// TestFetchMarketplaceOmiteIncompletas verifica que se descartan skills sin id o sin
// githubUrl (un descubrimiento sin fuente que revisar no sirve).
func TestFetchMarketplaceOmiteIncompletas(t *testing.T) {
	sinGithub := `{"id":"x-skill-md","name":"x","githubUrl":"","stars":1}`
	sinID := `{"id":"","name":"y","githubUrl":"https://github.com/a/b","stars":1}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, respuestaMarketplace(skillGolang+","+sinGithub+","+sinID))
	}))
	defer srv.Close()

	skills, err := FetchMarketplaceSkills(context.Background(), srv.URL, "", "go", 10)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("esperaba 1 skill válida (las incompletas se omiten), obtuve %d", len(skills))
	}
}

// TestFetchMarketplaceErrores verifica la degradación: HTTP ≠ 200, JSON inválido y
// success=false devuelven error (no-fatal) para que el llamador degrade.
func TestFetchMarketplaceErrores(t *testing.T) {
	casos := []struct {
		nombre  string
		handler http.HandlerFunc
	}{
		{"http-500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }},
		{"json-roto", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "{no es json") }},
		{"success-false", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"success":false,"data":{"skills":[]}}`)
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			srv := httptest.NewServer(c.handler)
			defer srv.Close()
			if _, err := FetchMarketplaceSkills(context.Background(), srv.URL, "", "go", 10); err == nil {
				t.Errorf("esperaba error para %s, obtuve nil", c.nombre)
			}
		})
	}
}
