package skillsource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// stubSkill arma una MarketplaceSkill mínima válida con id y estrellas dados.
func stubSkill(id string, stars int) MarketplaceSkill {
	return MarketplaceSkill{
		ID:        id,
		Name:      id,
		GithubURL: "https://github.com/x/" + id,
		Stars:     stars,
	}
}

// TestHarvestMarketplace cubre dedup (gana la de más estrellas), filtro por min-stars,
// orden por estrellas desc, seeds efectivamente usadas y que una seed que falla se omite
// sin abortar la cosecha.
func TestHarvestMarketplace(t *testing.T) {
	porSeed := map[string][]MarketplaceSkill{
		"go":     {stubSkill("a", 2), stubSkill("b", 1), stubSkill("c", 0)},
		"python": {stubSkill("a", 5), stubSkill("d", 3), stubSkill("e", 0)},
	}
	fetch := func(ctx context.Context, query string, limit int) ([]MarketplaceSkill, error) {
		if query == "fail" {
			return nil, fmt.Errorf("seed caída")
		}
		return porSeed[query], nil
	}

	cat, err := HarvestMarketplace(context.Background(), fetch, []string{"go", "python", "fail", "  "}, 50, 1, 0)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	// min-stars=1 descarta c y e (0 estrellas). Quedan a, b, d.
	ids := make([]string, len(cat.Skills))
	starsByID := map[string]int{}
	for i, s := range cat.Skills {
		ids[i] = s.ID
		starsByID[s.ID] = s.Stars
	}
	if len(cat.Skills) != 3 {
		t.Fatalf("esperaba 3 skills curadas (a,b,d), obtuve %d: %v", len(cat.Skills), ids)
	}
	// dedup: 'a' aparece en ambas seeds; gana la de más estrellas (5).
	if starsByID["a"] != 5 {
		t.Errorf("dedup: 'a' debería tener 5 estrellas (la mayor), obtuve %d", starsByID["a"])
	}
	// orden por estrellas desc: a(5), d(3), b(1).
	if ids[0] != "a" || ids[1] != "d" || ids[2] != "b" {
		t.Errorf("orden esperado [a d b], obtuve %v", ids)
	}
	// la seed "fail" se omite; "  " (vacía) también. Quedan go y python.
	if len(cat.Seeds) != 2 {
		t.Errorf("esperaba 2 seeds usadas (go, python), obtuve %v", cat.Seeds)
	}
	if cat.Version != marketplaceCatalogVersion {
		t.Errorf("Version: %d, esperaba %d", cat.Version, marketplaceCatalogVersion)
	}
	// El núcleo NO setea Generated (lo hace el caller).
	if cat.Generated != "" {
		t.Errorf("HarvestMarketplace no debería setear Generated, obtuve %q", cat.Generated)
	}
}

// TestHarvestMarketplaceFetchNulo verifica el guard de fetch nulo.
func TestHarvestMarketplaceFetchNulo(t *testing.T) {
	if _, err := HarvestMarketplace(context.Background(), nil, []string{"go"}, 50, 0, 3); err == nil {
		t.Error("esperaba error con fetch nulo")
	}
}

// TestHarvestMarketplaceCapPorRepo verifica que maxPerRepo acota cuántas skills aporta un
// mismo repo (las de mayor ranking), para que un monorepo no inunde el catálogo.
func TestHarvestMarketplaceCapPorRepo(t *testing.T) {
	repoSkill := func(id string, stars int, owner, repo string) MarketplaceSkill {
		return MarketplaceSkill{
			ID:        id,
			Name:      id,
			Stars:     stars,
			GithubURL: "https://github.com/" + owner + "/" + repo + "/tree/main/skills/" + id,
		}
	}
	skills := []MarketplaceSkill{
		repoSkill("m1", 100, "mega", "mega"),
		repoSkill("m2", 100, "mega", "mega"),
		repoSkill("m3", 100, "mega", "mega"),
		repoSkill("m4", 100, "mega", "mega"),
		repoSkill("s1", 50, "small", "small"),
	}
	fetch := func(ctx context.Context, query string, limit int) ([]MarketplaceSkill, error) {
		return skills, nil
	}

	cat, err := HarvestMarketplace(context.Background(), fetch, []string{"q"}, 50, 0, 2)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	// mega aporta a lo sumo 2 (de 4); small aporta su 1. Total 3.
	if len(cat.Skills) != 3 {
		t.Fatalf("con cap=2 esperaba 3 skills (2 mega + 1 small), obtuve %d: %v", len(cat.Skills), ids(cat.Skills))
	}
	megaCount := 0
	for _, s := range cat.Skills {
		if repoKey(s.GithubURL) == "mega/mega" {
			megaCount++
		}
	}
	if megaCount != 2 {
		t.Errorf("el repo mega debería estar capeado a 2, obtuve %d", megaCount)
	}

	// Sin cap (0): entran las 5.
	sinCap, _ := HarvestMarketplace(context.Background(), fetch, []string{"q"}, 50, 0, 0)
	if len(sinCap.Skills) != 5 {
		t.Errorf("sin cap esperaba 5, obtuve %d", len(sinCap.Skills))
	}
}

// TestRepoKey verifica la extracción de owner/repo de una URL de GitHub.
func TestRepoKey(t *testing.T) {
	casos := map[string]string{
		"https://github.com/openclaw/openclaw/tree/main/skills/gog": "openclaw/openclaw",
		"https://github.com/a/b":                                    "a/b",
		"https://gitlab.com/a/b":                                    "",
		"no-es-url":                                                 "",
		"https://github.com/soloowner":                              "",
	}
	for url, want := range casos {
		if got := repoKey(url); got != want {
			t.Errorf("repoKey(%q) = %q, esperaba %q", url, got, want)
		}
	}
}

// TestFilterMarketplaceSkills cubre el filtrado local por query (algún término en
// nombre/desc/id), query vacía (todas), preservación del orden y el límite.
func TestFilterMarketplaceSkills(t *testing.T) {
	skills := []MarketplaceSkill{
		{ID: "go-http", Name: "go-http", Description: "patrones HTTP en Go", Stars: 10},
		{ID: "py-flask", Name: "py-flask", Description: "Flask para Python", Stars: 8},
		{ID: "go-test", Name: "golang-testing", Description: "testing", Stars: 5},
	}
	// query "go" matchea go-http (nombre) y go-test (golang-testing contiene "go").
	got := FilterMarketplaceSkills(skills, "go", 10)
	if len(got) != 2 || got[0].ID != "go-http" || got[1].ID != "go-test" {
		t.Errorf("filtro 'go': esperaba [go-http go-test] en orden, obtuve %+v", ids(got))
	}
	// query vacía => todas.
	if len(FilterMarketplaceSkills(skills, "", 10)) != 3 {
		t.Error("query vacía debería devolver todas")
	}
	// límite respetado.
	if len(FilterMarketplaceSkills(skills, "", 1)) != 1 {
		t.Error("el límite debería acotar a 1")
	}
}

func ids(skills []MarketplaceSkill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.ID
	}
	return out
}

// TestFetchMarketplaceCatalog verifica el fetch del catálogo estático y el error no-fatal
// ante HTTP ≠ 200 (para que el caller caiga a live).
func TestFetchMarketplaceCatalog(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"version":1,"seeds":["Go"],"skills":[{"id":"a-skill-md","name":"a","githubUrl":"https://github.com/x/a","stars":3}]}`)
	}))
	defer ok.Close()
	cat, err := FetchMarketplaceCatalog(context.Background(), ok.URL)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cat.Version != 1 || len(cat.Skills) != 1 || cat.Skills[0].ID != "a-skill-md" {
		t.Errorf("catálogo mal parseado: %+v", cat)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bad.Close()
	if _, err := FetchMarketplaceCatalog(context.Background(), bad.URL); err == nil {
		t.Error("esperaba error ante HTTP 404")
	}
}

// TestMarketplaceCatalogEjemploParsea valida que el ejemplo commiteado en testdata parsea
// al esquema y cumple las invariantes (versión, skills con id y githubUrl).
func TestMarketplaceCatalogEjemploParsea(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "marketplace-index.example.json"))
	if err != nil {
		t.Fatalf("no se pudo leer el ejemplo: %v", err)
	}
	var cat MarketplaceCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("el ejemplo no parsea a MarketplaceCatalog: %v", err)
	}
	if cat.Version != marketplaceCatalogVersion {
		t.Errorf("Version del ejemplo: %d, esperaba %d", cat.Version, marketplaceCatalogVersion)
	}
	if len(cat.Skills) == 0 {
		t.Fatal("el ejemplo no tiene skills")
	}
	for _, s := range cat.Skills {
		if s.ID == "" || s.GithubURL == "" {
			t.Errorf("skill incompleta en el ejemplo: %+v", s)
		}
	}
}
