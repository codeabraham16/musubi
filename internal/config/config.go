package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EmbeddingConfig describe cómo se generan los embeddings para la búsqueda semántica.
type EmbeddingConfig struct {
	Provider   string `yaml:"provider"`   // none | ollama
	Model      string `yaml:"model"`      // ej. nomic-embed-text
	BaseURL    string `yaml:"base_url"`   // ej. http://localhost:11434
	Dimensions int    `yaml:"dimensions"` // dimensión del vector que produce el modelo
}

// Config es la configuración del workspace (.musubi/config.yaml).
type Config struct {
	Version           string          `yaml:"version"`
	Mode              string          `yaml:"mode"`
	SkillsAutoResolve bool            `yaml:"skills_auto_resolve"`
	Embedding         EmbeddingConfig `yaml:"embedding"`
}

// Default devuelve la configuración por defecto (local-first, embeddings desactivados).
func Default() Config {
	return Config{
		Version:           "1.0",
		Mode:              "local",
		SkillsAutoResolve: true,
		Embedding: EmbeddingConfig{
			Provider:   "none",
			Model:      "nomic-embed-text",
			BaseURL:    "http://localhost:11434",
			Dimensions: 768,
		},
	}
}

// Marshal serializa la configuración a YAML (usado por `musubi init`).
func (c Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

// Load lee projectPath/.musubi/config.yaml aplicando defaults para campos ausentes.
// Si el archivo no existe, devuelve la configuración por defecto sin error.
func Load(projectPath string) (Config, error) {
	cfg := Default()
	path := filepath.Join(projectPath, DirName, ConfigFile)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("error al leer %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("error al parsear config.yaml: %w", err)
	}

	cfg.applyDefaults()
	return cfg, nil
}

// applyDefaults rellena campos vacíos con sus valores por defecto.
func (c *Config) applyDefaults() {
	d := Default()
	if c.Embedding.Provider == "" {
		c.Embedding.Provider = d.Embedding.Provider
	}
	if c.Embedding.Model == "" {
		c.Embedding.Model = d.Embedding.Model
	}
	if c.Embedding.BaseURL == "" {
		c.Embedding.BaseURL = d.Embedding.BaseURL
	}
	if c.Embedding.Dimensions == 0 {
		c.Embedding.Dimensions = d.Embedding.Dimensions
	}
}
