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
