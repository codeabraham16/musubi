package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// defaultCatalogURL es la URL del catálogo de skills por defecto alojado en este repositorio.
const defaultCatalogURL = "https://raw.githubusercontent.com/codeabraham16/musubi-skills/main/index.json"

// EmbeddingConfig describe cómo se generan los embeddings para la búsqueda semántica.
type EmbeddingConfig struct {
	Provider   string `yaml:"provider"`   // none | ollama
	Model      string `yaml:"model"`      // ej. nomic-embed-text
	BaseURL    string `yaml:"base_url"`   // ej. http://localhost:11434
	Dimensions int    `yaml:"dimensions"` // dimensión del vector que produce el modelo
}

// SourcingConfig controla la obtención automática de skills desde un catálogo remoto.
type SourcingConfig struct {
	// Enabled activa o desactiva el sourcing de skills desde el catálogo.
	Enabled bool `yaml:"enabled"`
	// CatalogURL es la URL del índice de catálogo de skills en formato JSON.
	CatalogURL string `yaml:"catalog_url"`
	// MaxCandidates limita la cantidad máxima de skills candidatas retornadas por musubi_search_skills.
	MaxCandidates int `yaml:"max_candidates"`
	// CacheSeconds es la duración (en segundos) del caché en memoria de la respuesta del catálogo.
	CacheSeconds int `yaml:"cache_seconds"`
}

// MemoryConfig controla el recall por presupuesto de tokens (memoria eficiente).
type MemoryConfig struct {
	// RecallTokenBudget es el techo de tokens por defecto de musubi_recall.
	RecallTokenBudget int `yaml:"recall_token_budget"`
	// GistMaxTokens es el tope de tokens de un gist (titular extractivo).
	GistMaxTokens int `yaml:"gist_max_tokens"`
	// CandidatePool es la cantidad de candidatos a rankear antes de empaquetar.
	CandidatePool int `yaml:"candidate_pool"`
}

// MaintenanceConfig controla el auto-mantenimiento de la memoria (consolidación
// de casi-duplicados y olvido por saliencia).
type MaintenanceConfig struct {
	// DedupThreshold es la similitud mínima (0..1) para fusionar casi-duplicados.
	DedupThreshold float64 `yaml:"dedup_threshold"`
	// DecayHalfLifeDays es la vida media de la recencia en el cálculo de saliencia.
	DecayHalfLifeDays float64 `yaml:"decay_half_life_days"`
	// DecayMinSalience es el umbral por debajo del cual una memoria fría se archiva.
	DecayMinSalience float64 `yaml:"decay_min_salience"`
	// DecayMinAgeDays es la edad mínima para que una memoria pueda archivarse.
	DecayMinAgeDays float64 `yaml:"decay_min_age_days"`
}

// GraphConfig controla la memoria estructurada en grafo (hechos/tripletas).
type GraphConfig struct {
	// MaxHops es la profundidad por defecto del recorrido BFS en musubi_recall_facts.
	MaxHops int `yaml:"max_hops"`
	// MaxFacts es el tope de hechos devueltos por musubi_recall_facts.
	MaxFacts int `yaml:"max_facts"`
}

// Config es la configuración del workspace (.musubi/config.yaml).
type Config struct {
	Version           string          `yaml:"version"`
	Mode              string          `yaml:"mode"`
	SkillsAutoResolve bool            `yaml:"skills_auto_resolve"`
	Embedding         EmbeddingConfig `yaml:"embedding"`
	// Sourcing configura el comportamiento de sourcing de skills desde catálogos remotos.
	Sourcing SourcingConfig `yaml:"sourcing,omitempty"`
	// Memory configura el recall por presupuesto de tokens.
	Memory MemoryConfig `yaml:"memory,omitempty"`
	// Maintenance configura el auto-mantenimiento (consolidación + olvido).
	Maintenance MaintenanceConfig `yaml:"maintenance,omitempty"`
	// Graph configura la memoria estructurada en grafo.
	Graph GraphConfig `yaml:"graph,omitempty"`
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
		Sourcing: SourcingConfig{
			Enabled:       true,
			CatalogURL:    defaultCatalogURL,
			MaxCandidates: 20,
			CacheSeconds:  3600,
		},
		Memory: MemoryConfig{
			RecallTokenBudget: 400,
			GistMaxTokens:     24,
			CandidatePool:     50,
		},
		Maintenance: MaintenanceConfig{
			DedupThreshold:    0.85,
			DecayHalfLifeDays: 30,
			DecayMinSalience:  0.2,
			DecayMinAgeDays:   14,
		},
		Graph: GraphConfig{
			MaxHops:  2,
			MaxFacts: 50,
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

	// Aplicar defaults de Sourcing.
	// Si CatalogURL y MaxCandidates son cero-valor, el bloque sourcing estaba ausente
	// en el YAML: activar Enabled por defecto también.
	bloqueSourcingAusente := c.Sourcing.CatalogURL == "" && c.Sourcing.MaxCandidates == 0
	if c.Sourcing.CatalogURL == "" {
		c.Sourcing.CatalogURL = d.Sourcing.CatalogURL
	}
	if c.Sourcing.MaxCandidates == 0 {
		c.Sourcing.MaxCandidates = d.Sourcing.MaxCandidates
	}
	if c.Sourcing.CacheSeconds == 0 {
		c.Sourcing.CacheSeconds = d.Sourcing.CacheSeconds
	}
	// Solo forzar Enabled=true cuando el bloque completo estaba ausente.
	// Si el usuario escribió enabled: false explícitamente, lo respetamos.
	if bloqueSourcingAusente {
		c.Sourcing.Enabled = true
	}

	// Defaults de Memory.
	if c.Memory.RecallTokenBudget == 0 {
		c.Memory.RecallTokenBudget = d.Memory.RecallTokenBudget
	}
	if c.Memory.GistMaxTokens == 0 {
		c.Memory.GistMaxTokens = d.Memory.GistMaxTokens
	}
	if c.Memory.CandidatePool == 0 {
		c.Memory.CandidatePool = d.Memory.CandidatePool
	}

	// Defaults de Maintenance.
	if c.Maintenance.DedupThreshold == 0 {
		c.Maintenance.DedupThreshold = d.Maintenance.DedupThreshold
	}
	if c.Maintenance.DecayHalfLifeDays == 0 {
		c.Maintenance.DecayHalfLifeDays = d.Maintenance.DecayHalfLifeDays
	}
	if c.Maintenance.DecayMinSalience == 0 {
		c.Maintenance.DecayMinSalience = d.Maintenance.DecayMinSalience
	}
	if c.Maintenance.DecayMinAgeDays == 0 {
		c.Maintenance.DecayMinAgeDays = d.Maintenance.DecayMinAgeDays
	}

	// Defaults de Graph.
	if c.Graph.MaxHops == 0 {
		c.Graph.MaxHops = d.Graph.MaxHops
	}
	if c.Graph.MaxFacts == 0 {
		c.Graph.MaxFacts = d.Graph.MaxFacts
	}
}
