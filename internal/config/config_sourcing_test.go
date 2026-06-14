package config

import (
	"testing"
)

// TestSourcingConfigDefaultsAplicados verifica que Default() establece valores
// correctos para SourcingConfig cuando no hay config.yaml.
func TestSourcingConfigDefaultsAplicados(t *testing.T) {
	cfg := Default()

	if !cfg.Sourcing.Enabled {
		t.Error("Sourcing.Enabled debería ser true por defecto")
	}
	if cfg.Sourcing.CatalogURL == "" {
		t.Error("Sourcing.CatalogURL no debería estar vacío por defecto")
	}
	if cfg.Sourcing.MaxCandidates != 20 {
		t.Errorf("Sourcing.MaxCandidates: esperaba 20, obtuve %d", cfg.Sourcing.MaxCandidates)
	}
	if cfg.Sourcing.CacheSeconds != 3600 {
		t.Errorf("Sourcing.CacheSeconds: esperaba 3600, obtuve %d", cfg.Sourcing.CacheSeconds)
	}
}

// TestSourcingConfigCompatibilidadHaciaAtras verifica que un config.yaml
// sin bloque sourcing se carga sin error y aplica defaults.
func TestSourcingConfigCompatibilidadHaciaAtras(t *testing.T) {
	// YAML legado sin bloque sourcing (como los escritos antes de skill-sourcing)
	root := writeConfig(t, "version: \"1.0\"\nmode: local\nskills_auto_resolve: true\n")

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error con config legada: %v", err)
	}

	// Los defaults deben aplicarse automáticamente
	if !cfg.Sourcing.Enabled {
		t.Error("Sourcing.Enabled debería ser true (default) al cargar config legada")
	}
	if cfg.Sourcing.CatalogURL == "" {
		t.Error("Sourcing.CatalogURL debería tener valor por defecto al cargar config legada")
	}
	if cfg.Sourcing.MaxCandidates != 20 {
		t.Errorf("Sourcing.MaxCandidates: esperaba 20 (default), obtuve %d", cfg.Sourcing.MaxCandidates)
	}
}

// TestSourcingConfigValoresExplicitosOverrideDefaults verifica que valores
// explícitos en config.yaml sobreescriben los defaults.
func TestSourcingConfigValoresExplicitosOverrideDefaults(t *testing.T) {
	yaml := `version: "1.0"
mode: local
skills_auto_resolve: true
sourcing:
  enabled: true
  catalog_url: "https://example.com/custom-catalog.json"
  max_candidates: 5
  cache_seconds: 1800
`
	root := writeConfig(t, yaml)

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Sourcing.MaxCandidates != 5 {
		t.Errorf("MaxCandidates: esperaba 5, obtuve %d", cfg.Sourcing.MaxCandidates)
	}
	if cfg.Sourcing.CatalogURL != "https://example.com/custom-catalog.json" {
		t.Errorf("CatalogURL: esperaba URL custom, obtuve %q", cfg.Sourcing.CatalogURL)
	}
	if cfg.Sourcing.CacheSeconds != 1800 {
		t.Errorf("CacheSeconds: esperaba 1800, obtuve %d", cfg.Sourcing.CacheSeconds)
	}
}

// TestSourcingConfigOverrideParcialCompleta verifica que un override parcial
// (solo max_candidates) rellena el resto con defaults.
func TestSourcingConfigOverrideParcialCompleta(t *testing.T) {
	yaml := `version: "1.0"
mode: local
skills_auto_resolve: true
sourcing:
  max_candidates: 10
`
	root := writeConfig(t, yaml)

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Sourcing.MaxCandidates != 10 {
		t.Errorf("MaxCandidates: esperaba 10, obtuve %d", cfg.Sourcing.MaxCandidates)
	}
	// El resto debería ser por defecto
	if !cfg.Sourcing.Enabled {
		t.Error("Enabled debería ser true por defecto en override parcial")
	}
	if cfg.Sourcing.CatalogURL == "" {
		t.Error("CatalogURL debería tener valor por defecto en override parcial")
	}
	if cfg.Sourcing.CacheSeconds != 3600 {
		t.Errorf("CacheSeconds: esperaba 3600 (default), obtuve %d", cfg.Sourcing.CacheSeconds)
	}
}

// TestSourcingConfigEnabledFalseExplicito verifica que enabled: false explícito
// se respeta (no se sobreescribe con el default true).
func TestSourcingConfigEnabledFalseExplicito(t *testing.T) {
	yaml := `version: "1.0"
mode: local
sourcing:
  enabled: false
`
	root := writeConfig(t, yaml)

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Sourcing.Enabled {
		t.Error("Sourcing.Enabled debería ser false cuando se establece explícitamente como false")
	}
}
