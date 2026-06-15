package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0644); err != nil {
		t.Fatalf("write error: %v", err)
	}
	return root
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cfg.Embedding.Provider != "none" {
		t.Errorf("esperaba provider 'none' por defecto, obtuve %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Dimensions != 768 {
		t.Errorf("esperaba dimensions 768 por defecto, obtuve %d", cfg.Embedding.Dimensions)
	}
}

func TestLoadParsesEmbeddingBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\nskills_auto_resolve: true\nembedding:\n  provider: ollama\n  model: nomic-embed-text\n  base_url: http://localhost:11434\n  dimensions: 768\n")

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Embedding.Provider != "ollama" {
		t.Errorf("esperaba ollama, obtuve %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Model != "nomic-embed-text" {
		t.Errorf("modelo inesperado: %q", cfg.Embedding.Model)
	}
}

func TestLoadAppliesDefaultsForAbsentKeys(t *testing.T) {
	// Config legacy sin bloque embedding: deben aplicarse defaults.
	root := writeConfig(t, "version: \"1.0\"\nmode: local\nskills_auto_resolve: true\n")

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Embedding.Provider != "none" {
		t.Errorf("esperaba provider 'none', obtuve %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.BaseURL == "" || cfg.Embedding.Dimensions == 0 {
		t.Errorf("defaults no aplicados: %+v", cfg.Embedding)
	}
}

func TestLoadMemoryDefaults(t *testing.T) {
	// Config legacy sin bloque memory: deben aplicarse los defaults.
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Memory.RecallTokenBudget != 400 {
		t.Errorf("esperaba recall_token_budget 400 por defecto, obtuve %d", cfg.Memory.RecallTokenBudget)
	}
	if cfg.Memory.GistMaxTokens != 24 {
		t.Errorf("esperaba gist_max_tokens 24 por defecto, obtuve %d", cfg.Memory.GistMaxTokens)
	}
	if cfg.Memory.CandidatePool != 50 {
		t.Errorf("esperaba candidate_pool 50 por defecto, obtuve %d", cfg.Memory.CandidatePool)
	}
}

func TestLoadParsesMemoryBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmemory:\n  recall_token_budget: 800\n  gist_max_tokens: 32\n  candidate_pool: 100\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Memory.RecallTokenBudget != 800 || cfg.Memory.GistMaxTokens != 32 || cfg.Memory.CandidatePool != 100 {
		t.Errorf("bloque memory no parseado: %+v", cfg.Memory)
	}
}

func TestLoadMaintenanceDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Maintenance.DedupThreshold != 0.85 {
		t.Errorf("esperaba dedup_threshold 0.85 por defecto, obtuve %v", cfg.Maintenance.DedupThreshold)
	}
	if cfg.Maintenance.DecayHalfLifeDays != 30 || cfg.Maintenance.DecayMinSalience != 0.2 || cfg.Maintenance.DecayMinAgeDays != 14 {
		t.Errorf("defaults de maintenance no aplicados: %+v", cfg.Maintenance)
	}
	if cfg.Maintenance.AutoIntervalHours != 24 {
		t.Errorf("esperaba auto_interval_hours 24 por defecto, obtuve %v", cfg.Maintenance.AutoIntervalHours)
	}
}

func TestLoadMaintenanceAutoDisableRespected(t *testing.T) {
	// Bloque presente con auto_interval_hours: 0 -> se respeta (desactivado).
	root := writeConfig(t, "version: \"1.0\"\nmaintenance:\n  dedup_threshold: 0.85\n  auto_interval_hours: 0\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Maintenance.AutoIntervalHours != 0 {
		t.Errorf("auto_interval_hours: 0 explícito debería respetarse, obtuve %v", cfg.Maintenance.AutoIntervalHours)
	}
}

func TestLoadGraphDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Graph.MaxHops != 2 || cfg.Graph.MaxFacts != 50 || cfg.Graph.MaxObservations != 5 {
		t.Errorf("defaults de graph no aplicados: %+v", cfg.Graph)
	}
}

func TestLoadParsesGraphBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\ngraph:\n  max_hops: 3\n  max_facts: 100\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Graph.MaxHops != 3 || cfg.Graph.MaxFacts != 100 {
		t.Errorf("bloque graph no parseado: %+v", cfg.Graph)
	}
}

func TestLoadParsesMaintenanceBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmaintenance:\n  dedup_threshold: 0.9\n  decay_half_life_days: 60\n  decay_min_salience: 0.1\n  decay_min_age_days: 7\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Maintenance.DedupThreshold != 0.9 || cfg.Maintenance.DecayHalfLifeDays != 60 ||
		cfg.Maintenance.DecayMinSalience != 0.1 || cfg.Maintenance.DecayMinAgeDays != 7 {
		t.Errorf("bloque maintenance no parseado: %+v", cfg.Maintenance)
	}
}
