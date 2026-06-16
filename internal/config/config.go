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
	// AutoIntervalHours es cada cuántas horas corre el auto-mantenimiento al
	// arrancar el daemon (0 = desactivado; el mantenimiento manual sigue disponible).
	AutoIntervalHours float64 `yaml:"auto_interval_hours"`
}

// GraphConfig controla la memoria estructurada en grafo (hechos/tripletas).
type GraphConfig struct {
	// MaxHops es la profundidad por defecto del recorrido BFS en musubi_recall_facts.
	MaxHops int `yaml:"max_hops"`
	// MaxFacts es el tope de hechos devueltos por musubi_recall_facts.
	MaxFacts int `yaml:"max_facts"`
	// MaxObservations es el tope de observaciones (gists) que ensambla
	// musubi_entity_context al unir grafo + prosa.
	MaxObservations int `yaml:"max_observations"`
}

// StartupConfig controla el comportamiento del arranque de sesión (hook
// SessionStart): el priming de memoria y la re-generación de skills cuando el
// stack del proyecto cambia.
type StartupConfig struct {
	// PrimeMemory inyecta un recall compacto del contexto del proyecto al
	// arrancar cada sesión (default true).
	PrimeMemory bool `yaml:"prime_memory"`
	// RecallBudget es el techo de tokens del priming de memoria (default 300).
	RecallBudget int `yaml:"recall_budget"`
	// AutoRegen re-dispara la generación de skills cuando el stack crece respecto
	// de la huella guardada (default true). Si es false, la generación es one-shot.
	AutoRegen bool `yaml:"auto_regen"`
	// CognitiveBootstrap inyecta el bloque de skills cognitivas (analizar/deducir/
	// planear + perfil) al arrancar, hasta que el proyecto tenga perfil (default true).
	CognitiveBootstrap bool `yaml:"cognitive_bootstrap"`
}

// LoopConfig controla el loop de trabajo dirigido: la inyección de contexto por
// turno (hook UserPromptSubmit). Extiende el priming de arranque a cada prompt.
type LoopConfig struct {
	// PerTurnRecall inyecta, antes de cada prompt, un recall acotado relevante a lo
	// que el usuario acaba de pedir (default true).
	PerTurnRecall bool `yaml:"per_turn_recall"`
	// RecallBudget es el techo de tokens del recall por turno (default 250).
	RecallBudget int `yaml:"recall_budget"`
	// SurfaceConflicts agrega, cuando hay relaciones de memoria sin resolver, una
	// línea que invita a resolverlas con musubi_conflicts/musubi_judge (default true).
	SurfaceConflicts bool `yaml:"surface_conflicts"`
	// CaptureReminder recuerda persistir aprendizajes cuando pasaron varios turnos
	// sin guardar nada en memoria (default true). Cierra el loop: contexto antes,
	// captura después.
	CaptureReminder bool `yaml:"capture_reminder"`
	// ReminderAfterTurns es la cantidad de turnos sin guardar tras la cual se inyecta
	// el recordatorio de captura (default 5).
	ReminderAfterTurns int `yaml:"reminder_after_turns"`
	// DeltaInjection inyecta por turno SOLO la memoria nueva o modificada respecto
	// de lo ya inyectado en la sesión (en vez de re-inyectar todo cada turno).
	// Ahorra tokens y evita churnear el contexto (cache-considerate) (default true).
	DeltaInjection bool `yaml:"delta_injection"`
}

// PipelineConfig controla el pipeline por fases del loop dirigido: Musubi mantiene
// el estado de la fase actual de la tarea (explorar→planear→codear→verificar) y se
// lo recuerda a Claude en cada turno. Determinista y model-free: Claude hace el
// trabajo, Musubi secuencia.
type PipelineConfig struct {
	// Enabled activa el recordatorio de fase por turno y la herramienta musubi_phase
	// (default true). Sin una tarea activa no inyecta nada.
	Enabled bool `yaml:"enabled"`
	// Phases es la secuencia de fases por defecto al iniciar una tarea.
	Phases []string `yaml:"phases"`
}

// MultiAgentConfig controla la pizarra compartida del multi-agente (musubi_work
// + recordatorio de batch por turno).
type MultiAgentConfig struct {
	// Enabled activa el recordatorio de batch por turno (default true). La tool
	// musubi_work siempre está disponible.
	Enabled bool `yaml:"enabled"`
	// MaxBatchUnits es el tope de unidades por batch, como cota de seguridad
	// (default 50).
	MaxBatchUnits int `yaml:"max_batch_units"`
}

// ConflictConfig controla la detección de relaciones semánticas entre
// observaciones (resolución de conflictos model-free).
type ConflictConfig struct {
	// Enabled activa la detección al guardar observaciones (default true).
	Enabled bool `yaml:"enabled"`
	// SimilarityFloor es el piso (Jaccard de trigramas) para considerar dos
	// observaciones relacionadas (default 0.3).
	SimilarityFloor float64 `yaml:"similarity_floor"`
	// AutoResolveThreshold es la similitud a partir de la cual se auto-resuelve
	// (supersede/related) sin preguntar al agente (default 0.7).
	AutoResolveThreshold float64 `yaml:"auto_resolve_threshold"`
	// CandidatePool es la cantidad de candidatas por FTS a evaluar (default 10).
	CandidatePool int `yaml:"candidate_pool"`
}

// UpdateConfig controla el chequeo de nuevas versiones del binario al arrancar.
type UpdateConfig struct {
	// CheckIntervalHours es cada cuántas horas el daemon chequea si hay una
	// versión nueva y avisa por stderr (default 24). Un valor negativo lo desactiva.
	CheckIntervalHours float64 `yaml:"check_interval_hours"`
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
	// Update configura el chequeo de nuevas versiones del binario.
	Update UpdateConfig `yaml:"update,omitempty"`
	// Startup configura el priming de memoria y la re-generación de skills al arrancar.
	Startup StartupConfig `yaml:"startup,omitempty"`
	// Conflicts configura la detección de relaciones semánticas entre observaciones.
	Conflicts ConflictConfig `yaml:"conflicts,omitempty"`
	// Loop configura el loop de trabajo dirigido (inyección de contexto por turno).
	Loop LoopConfig `yaml:"loop,omitempty"`
	// Pipeline configura el pipeline por fases del loop dirigido.
	Pipeline PipelineConfig `yaml:"pipeline,omitempty"`
	// MultiAgent configura la pizarra compartida del multi-agente.
	MultiAgent MultiAgentConfig `yaml:"multiagent,omitempty"`
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
			AutoIntervalHours: 24,
		},
		Graph: GraphConfig{
			MaxHops:         2,
			MaxFacts:        50,
			MaxObservations: 5,
		},
		Update: UpdateConfig{
			CheckIntervalHours: 24,
		},
		Startup: StartupConfig{
			PrimeMemory:        true,
			RecallBudget:       300,
			AutoRegen:          true,
			CognitiveBootstrap: true,
		},
		Conflicts: ConflictConfig{
			Enabled:              true,
			SimilarityFloor:      0.3,
			AutoResolveThreshold: 0.7,
			CandidatePool:        10,
		},
		Loop: LoopConfig{
			PerTurnRecall:      true,
			RecallBudget:       250,
			SurfaceConflicts:   true,
			CaptureReminder:    true,
			ReminderAfterTurns: 5,
			DeltaInjection:     true,
		},
		Pipeline: PipelineConfig{
			Enabled: true,
			Phases:  []string{"explore", "plan", "code", "verify"},
		},
		MultiAgent: MultiAgentConfig{
			Enabled:       true,
			MaxBatchUnits: 50,
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

	cfg.applyDefaults(presentBlocks(data))
	return cfg, nil
}

// presentBlocks devuelve el conjunto de claves top-level presentes en el YAML.
// Permite distinguir "bloque ausente" de "bloque presente con enabled:false",
// que con un bool puro es indistinguible por su cero-valor.
func presentBlocks(data []byte) map[string]bool {
	present := map[string]bool{}
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err == nil {
		for k := range raw {
			present[k] = true
		}
	}
	return present
}

// applyDefaults rellena campos vacíos con sus valores por defecto. present indica
// qué bloques top-level estaban en el YAML: un bloque ausente toma el default
// completo; uno presente conserva sus bool (incluido enabled:false) y solo rellena
// los numéricos en cero.
func (c *Config) applyDefaults(present map[string]bool) {
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

	// Sourcing: ausente -> default completo (Enabled true); presente -> respetar
	// Enabled y rellenar numéricos.
	if !present["sourcing"] {
		c.Sourcing = d.Sourcing
	} else {
		if c.Sourcing.CatalogURL == "" {
			c.Sourcing.CatalogURL = d.Sourcing.CatalogURL
		}
		if c.Sourcing.MaxCandidates == 0 {
			c.Sourcing.MaxCandidates = d.Sourcing.MaxCandidates
		}
		if c.Sourcing.CacheSeconds == 0 {
			c.Sourcing.CacheSeconds = d.Sourcing.CacheSeconds
		}
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

	// Maintenance: ausente -> default completo; presente -> rellenar numéricos y
	// respetar auto_interval_hours tal cual (0 = desactivado explícito).
	if !present["maintenance"] {
		c.Maintenance = d.Maintenance
	} else {
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
	}

	// Defaults de Graph.
	if c.Graph.MaxHops == 0 {
		c.Graph.MaxHops = d.Graph.MaxHops
	}
	if c.Graph.MaxFacts == 0 {
		c.Graph.MaxFacts = d.Graph.MaxFacts
	}
	if c.Graph.MaxObservations == 0 {
		c.Graph.MaxObservations = d.Graph.MaxObservations
	}

	// Default de Update: 0 (ausente) -> 24h. Un valor negativo desactiva el chequeo.
	if c.Update.CheckIntervalHours == 0 {
		c.Update.CheckIntervalHours = d.Update.CheckIntervalHours
	}

	// Startup: ausente -> default completo; presente -> respetar los bool tal cual
	// y rellenar recall_budget.
	if !present["startup"] {
		c.Startup = d.Startup
	} else if c.Startup.RecallBudget == 0 {
		c.Startup.RecallBudget = d.Startup.RecallBudget
	}

	// Conflicts: ausente -> default completo (Enabled true); presente -> respetar
	// enabled (incluido false) y rellenar numéricos.
	if !present["conflicts"] {
		c.Conflicts = d.Conflicts
	} else {
		if c.Conflicts.SimilarityFloor == 0 {
			c.Conflicts.SimilarityFloor = d.Conflicts.SimilarityFloor
		}
		if c.Conflicts.AutoResolveThreshold == 0 {
			c.Conflicts.AutoResolveThreshold = d.Conflicts.AutoResolveThreshold
		}
		if c.Conflicts.CandidatePool == 0 {
			c.Conflicts.CandidatePool = d.Conflicts.CandidatePool
		}
	}

	// Loop: ausente -> default completo; presente -> respetar bool y rellenar numéricos.
	if !present["loop"] {
		c.Loop = d.Loop
	} else {
		if c.Loop.RecallBudget == 0 {
			c.Loop.RecallBudget = d.Loop.RecallBudget
		}
		if c.Loop.ReminderAfterTurns == 0 {
			c.Loop.ReminderAfterTurns = d.Loop.ReminderAfterTurns
		}
	}

	// Pipeline: ausente -> default completo; presente -> respetar enabled y
	// completar las fases.
	if !present["pipeline"] {
		c.Pipeline = d.Pipeline
	} else if len(c.Pipeline.Phases) == 0 {
		c.Pipeline.Phases = d.Pipeline.Phases
	}

	// MultiAgent: ausente -> default completo; presente -> respetar enabled y
	// completar el tope.
	if !present["multiagent"] {
		c.MultiAgent = d.MultiAgent
	} else if c.MultiAgent.MaxBatchUnits == 0 {
		c.MultiAgent.MaxBatchUnits = d.MultiAgent.MaxBatchUnits
	}
}
