package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/skillsource"
)

// respMarketplaceCLI arma el sobre JSON del endpoint de búsqueda del marketplace.
func respMarketplaceCLI(skillsJSON string) string {
	return fmt.Sprintf(`{"success":true,"data":{"skills":[%s],"pagination":{"total":1}}}`, skillsJSON)
}

const skillCLI = `{
	"id":"acme-go-skill-md","name":"go-skill","author":"acme",
	"description":"Una skill Go.","githubUrl":"https://github.com/acme/go-skill",
	"skillUrl":"https://skillsmp.com/skills/acme-go","stars":7,"updatedAt":"1781667763"
}`

// TestRunHarvestEscribeCatalogo: harvest contra un marketplace de prueba escribe un
// catálogo válido con la skill cosechada y el timestamp Generated seteado.
func TestRunHarvestEscribeCatalogo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, respMarketplaceCLI(skillCLI))
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "marketplace-index.json")
	err := runHarvest([]string{
		"--url", srv.URL,
		"--seeds", "Go,Python",
		"--top", "10",
		"--out", out,
		"--api-key-env", "__NO_EXISTE__", // sin key → tier anónimo (aviso, no error)
	})
	if err != nil {
		t.Fatalf("runHarvest falló: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no se escribió el catálogo: %v", err)
	}
	var cat skillsource.MarketplaceCatalog
	if err := json.Unmarshal(data, &cat); err != nil {
		t.Fatalf("el catálogo escrito no parsea: %v", err)
	}
	if len(cat.Skills) != 1 || cat.Skills[0].ID != "acme-go-skill-md" {
		t.Errorf("esperaba 1 skill cosechada (dedup entre seeds), obtuve %+v", cat.Skills)
	}
	if cat.Generated == "" {
		t.Error("esperaba Generated seteado por el CLI")
	}
}

// TestRunHarvestFlagsInvalidos: flags mal formados devuelven error (no os.Exit).
func TestRunHarvestFlagsInvalidos(t *testing.T) {
	casos := [][]string{
		{"--top", "abc"},       // no numérico
		{"--top", "-3"},        // no positivo
		{"--min-stars", "-1"},  // negativo
		{"--flag-desconocido"}, // flag inexistente
		{"--seeds", ""},        // seeds vacías → sin nada para cosechar
	}
	for _, args := range casos {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			if err := runHarvest(args); err == nil {
				t.Errorf("esperaba error para %v, obtuve nil", args)
			}
		})
	}
}
