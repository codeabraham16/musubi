package skillsource

import (
	"context"
	"encoding/json"
	"fmt"
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

	cat, err := HarvestMarketplace(context.Background(), fetch, []string{"go", "python", "fail", "  "}, 50, 1)
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
	if _, err := HarvestMarketplace(context.Background(), nil, []string{"go"}, 50, 0); err == nil {
		t.Error("esperaba error con fetch nulo")
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
