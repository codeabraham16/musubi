package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// defaultCatalogURL es la URL del catálogo de skills por defecto alojado en este repositorio.
const defaultCatalogURL = "https://raw.githubusercontent.com/codeabraham16/musubi-skills/main/index.json"

// defaultMarketplaceURL es el host por defecto del marketplace de Agent Skills (SKILL.md)
// que usa el descubrimiento opt-in (musubi_discover_skills).
const defaultMarketplaceURL = "https://skillsmp.com"

// defaultMarketplaceCatalogURL es la URL del catálogo estático cosechado, publicado por el
// cosechador central en el repo musubi-skills. El descubrimiento lee de acá por default
// (cero rate limit); si el archivo aún no existe, cae con gracia a la API en vivo.
const defaultMarketplaceCatalogURL = "https://raw.githubusercontent.com/codeabraham16/musubi-skills/main/marketplace-index.json"

// EmbeddingConfig describe cómo se generan los embeddings para la búsqueda semántica.
type EmbeddingConfig struct {
	Provider   string `yaml:"provider"`    // none | ollama | openai
	Model      string `yaml:"model"`       // ej. nomic-embed-text, text-embedding-3-small
	BaseURL    string `yaml:"base_url"`    // ej. http://localhost:11434, https://api.openai.com/v1
	Dimensions int    `yaml:"dimensions"`  // dimensión del vector que produce el modelo
	APIKeyEnv  string `yaml:"api_key_env"` // nombre de la env var con la API key (openai); el secreto NO se guarda en el yaml
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
	// MarketplaceEnabled activa el DESCUBRIMIENTO de Agent Skills (SKILL.md) desde un
	// marketplace externo (musubi_discover_skills). Default false: es opt-in porque indexa
	// contenido no confiable de GitHub arbitrario. Solo descubre y enlaza, nunca instala.
	MarketplaceEnabled bool `yaml:"marketplace_enabled,omitempty"`
	// MarketplaceURL es el host del marketplace de Agent Skills (ej. https://skillsmp.com).
	MarketplaceURL string `yaml:"marketplace_url,omitempty"`
	// MarketplaceAPIKeyEnv es el NOMBRE de la env var con la API key del marketplace (sube el
	// rate limit). El secreto NO se guarda en el yaml. Vacío => se usa el tier anónimo.
	MarketplaceAPIKeyEnv string `yaml:"marketplace_api_key_env,omitempty"`
	// MarketplaceCatalogURL es la URL del catálogo ESTÁTICO cosechado (marketplace-index.json
	// publicado por el cosechador central). Si está seteada, musubi_discover_skills lee de ahí
	// (cero rate limit) y solo cae a la API en vivo si el catálogo no está disponible. Vacío =>
	// siempre en vivo.
	MarketplaceCatalogURL string `yaml:"marketplace_catalog_url,omitempty"`
}

// MemoryConfig controla el recall por presupuesto de tokens (memoria eficiente).
type MemoryConfig struct {
	// RecallTokenBudget es el techo de tokens por defecto de musubi_recall.
	RecallTokenBudget int `yaml:"recall_token_budget"`
	// GistMaxTokens es el tope de tokens de un gist (titular extractivo).
	GistMaxTokens int `yaml:"gist_max_tokens"`
	// CandidatePool es la cantidad de candidatos a rankear antes de empaquetar.
	CandidatePool int `yaml:"candidate_pool"`
	// SessionTokenBudget es el techo BLANDO de tokens que Musubi inyecta como contexto
	// ambiente en una sesión (suma de todas las superficies del ledger). No recorta nada:
	// el gobernador lo usa para reportar el uso (musubi_tokens) y avisar una vez por sesión
	// cuando se cruza, para que el gasto de contexto sea visible y acotable. 0 = sin techo
	// (default 8000).
	SessionTokenBudget int `yaml:"session_token_budget"`
	// BrevityMode controla la directiva de SALIDA del gobernador (T9.5): pide al agente
	// responder conciso para recortar tokens de RESPUESTA, complementando las superficies
	// que acotan la ENTRADA. Opt-in: "off" (default) no inyecta nada; "lite"/"full"/"ultra"
	// fijan el nivel una vez por sesión; "auto" solo dispara cuando el gasto cruza
	// session_token_budget (mismo umbral que la alerta), atando la brevedad al gobernador.
	// Un valor inválido degrada a "off": un typo nunca activa la directiva.
	BrevityMode string `yaml:"brevity_mode"`
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
	// PurgeArchivedAfterDays borra DEFINITIVAMENTE las observaciones archivadas que no
	// se tocaron en esta cantidad de días (retención dura, acota el crecimiento). El
	// olvido (decay) solo marca archived; esto las elimina de verdad. 0 = nunca purgar.
	PurgeArchivedAfterDays float64 `yaml:"purge_archived_after_days"`
	// Vacuum corre VACUUM tras una purga que borró filas, para reclamar espacio en
	// disco (default true). El checkpoint del WAL y PRAGMA optimize corren siempre.
	Vacuum bool `yaml:"vacuum"`
	// AutoAfterSaves dispara un mantenimiento (best-effort, respetando el throttle) tras
	// esta cantidad de saves en la sesión, para que una sesión intensa no espere al próximo
	// tick del scheduler. 0 = desactivado (default; opt-in consciente).
	AutoAfterSaves int `yaml:"auto_after_saves"`
	// DecayProtectImportance protege del olvido (decay) a las observaciones con importance
	// >= a este valor: conocimiento deliberado (decisiones, arquitectura) no se auto-archiva
	// por más viejo/frío que esté. 0 = sin protección (default; opt-in).
	DecayProtectImportance float64 `yaml:"decay_protect_importance"`
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
	// SingleValuedPredicates lista los predicados FUNCIONALES (single-valued): a lo
	// sumo un objeto vivo por sujeto. Al guardar (S, P, O_new) con P en esta lista, se
	// invalidan los (S, P, O_old) vivos con O_old != O_new (invalidación bi-temporal
	// por cardinalidad, model-free). Los predicados no listados son many-valued (no
	// invalidan). Comparación case-insensitive. Default curado y chico (ES+EN); el
	// usuario puede extenderlo o vaciarlo.
	SingleValuedPredicates []string `yaml:"single_valued_predicates"`
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
	// AvoidedContextTokensPerUnit estima el contexto intermedio (lecturas +
	// razonamiento) que cada unidad delegada mantiene en el sub-agente y que NUNCA
	// entra al contexto del orquestador. Es el motor del ahorro por delegación
	// (default 4000). Model-free: parámetro del estimador, no una medición del
	// sub-agente real.
	AvoidedContextTokensPerUnit int `yaml:"avoided_context_tokens_per_unit"`
	// DelegationOverheadTokens es el costo fijo de lanzar un sub-agente y correr el
	// protocolo de la pizarra por unidad (default 2000). El ahorro neto por unidad es
	// AvoidedContextTokensPerUnit - DelegationOverheadTokens.
	DelegationOverheadTokens int `yaml:"delegation_overhead_tokens"`
	// LeaseTTLSeconds es la vida de un lease de claim (default 300 = 5 min). Si el
	// dueño no renueva su lease (heartbeat) dentro de esta ventana, la unidad se
	// vuelve reclamable por otro agente. El trabajo de un sub-agente puede tardar
	// minutos, por eso el default es mayor que el visibility timeout típico de una cola.
	LeaseTTLSeconds int `yaml:"lease_ttl_seconds"`
	// MaxAttempts es la cantidad de reclamos antes de mandar una unidad a dead-letter
	// (status failed) en vez de reciclarla de nuevo (default 5). Evita el loop
	// crash→reclaim→crash de una unidad que siempre falla.
	MaxAttempts int `yaml:"max_attempts"`
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

// VectorIndexConfig configura el índice vectorial ANN (IVF) para la búsqueda
// semántica a escala. Por debajo de ExactThreshold (o con el índice sin entrenar)
// la búsqueda es el full-scan exacto de siempre; por encima, IVF acota candidatos
// y el ranking final sigue siendo coseno exacto sobre filas re-filtradas en SQLite.
// Bloque YAML: vector_index.
type VectorIndexConfig struct {
	// Enabled activa el índice IVF (default true). false => siempre full-scan exacto.
	Enabled bool `yaml:"enabled"`
	// ExactThreshold es la cantidad de observaciones con embedding a partir de la
	// cual se entrena y usa el índice IVF (default 10000). Debajo, exacto puro.
	ExactThreshold int `yaml:"exact_threshold"`
	// NProbe es la cantidad de celdas (centroides) más cercanas que se sondean por
	// query (default 8). Es el dial directo de recall vs latencia.
	NProbe int `yaml:"nprobe"`
	// NumCentroids fija la cantidad de centroides; 0 => auto = round(sqrt(N)) (default 0).
	NumCentroids int `yaml:"num_centroids"`
	// RebuildEvery es la cantidad de altas/bajas tras la cual se re-entrena el índice
	// (re-k-means) para corregir el drift de centroides (default 5000).
	RebuildEvery int `yaml:"rebuild_every"`
	// RebuildMinHours es el piso temporal entre re-entrenamientos (default 6).
	RebuildMinHours float64 `yaml:"rebuild_min_hours"`
	// KMeansIters son las iteraciones de Lloyd al entrenar centroides (default 10).
	KMeansIters int `yaml:"kmeans_iters"`
	// KMeansSample es el tope de vectores muestreados para entrenar centroides
	// (default 50000); por encima se entrena sobre una muestra y se asigna todo.
	KMeansSample int `yaml:"kmeans_sample"`
}

// ServiceConfig configura el modo servicio (Track 4): exponer el servidor MCP sobre
// HTTP además del stdio por defecto. Está DESACTIVADO por defecto (Enabled=false): un
// workspace existente sin bloque `service:` mantiene intacto el comportamiento
// local-first. Bloque YAML: service.
type ServiceConfig struct {
	// Enabled activa el transporte HTTP (default false). `musubi serve` se niega a
	// arrancar si está en false (salvo override por flag explícito).
	Enabled bool `yaml:"enabled"`
	// Addr es la dirección de escucha (default 127.0.0.1:7717). Por seguridad, en este
	// release SOLO se permite bind a loopback; un addr no-loopback es error de arranque
	// (la autenticación llega en un slice posterior y habilita el bind remoto).
	Addr string `yaml:"addr"`
	// RequestTimeoutSeconds es el timeout por request HTTP (default 60), espejo del
	// deadline de 60s del transporte stdio.
	RequestTimeoutSeconds float64 `yaml:"request_timeout_seconds"`
	// AuthTokenEnv es el nombre de la variable de entorno que contiene el bearer token
	// requerido (patrón de EmbeddingConfig.APIKeyEnv: el secreto NUNCA va en el YAML).
	// Vacío => sin autenticación (solo válido para bind loopback). Un bind no-loopback
	// EXIGE un token (si no, `serve` se niega a arrancar).
	AuthTokenEnv string `yaml:"auth_token_env,omitempty"`
	// TLSCertFile y TLSKeyFile habilitan TLS (HTTPS) cuando AMBOS están seteados. Setear
	// solo uno es error de arranque (no un downgrade silencioso a texto plano).
	TLSCertFile string `yaml:"tls_cert_file,omitempty"`
	TLSKeyFile  string `yaml:"tls_key_file,omitempty"`
	// AllowInsecureToken permite arrancar con un bind no-loopback + token PERO sin TLS
	// (el token viajaría en texto plano). Default false => fail-closed: hay que optar
	// explícitamente (p.ej. cuando un proxy termina TLS por delante).
	AllowInsecureToken bool `yaml:"allow_insecure_token,omitempty"`
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
	// VectorIndex configura el índice vectorial ANN (IVF) para búsqueda semántica a escala.
	VectorIndex VectorIndexConfig `yaml:"vector_index,omitempty"`
	// Service configura el modo servicio (transporte HTTP); desactivado por defecto.
	Service ServiceConfig `yaml:"service,omitempty"`
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
			APIKeyEnv:  "OPENAI_API_KEY",
		},
		Sourcing: SourcingConfig{
			Enabled:               true,
			CatalogURL:            defaultCatalogURL,
			MaxCandidates:         20,
			CacheSeconds:          3600,
			MarketplaceURL:        defaultMarketplaceURL,
			MarketplaceCatalogURL: defaultMarketplaceCatalogURL,
			// MarketplaceEnabled queda en false: el descubrimiento desde el marketplace
			// externo es opt-in (contenido no confiable de GitHub arbitrario).
		},
		Memory: MemoryConfig{
			RecallTokenBudget:  400,
			GistMaxTokens:      24,
			CandidatePool:      50,
			SessionTokenBudget: 8000,
			BrevityMode:        "off",
		},
		Maintenance: MaintenanceConfig{
			DedupThreshold:         0.85,
			DecayHalfLifeDays:      30,
			DecayMinSalience:       0.2,
			DecayMinAgeDays:        14,
			AutoIntervalHours:      24,
			PurgeArchivedAfterDays: 90,
			Vacuum:                 true,
		},
		Graph: GraphConfig{
			MaxHops:         2,
			MaxFacts:        50,
			MaxObservations: 5,
			// Predicados funcionales de dominio general, ES + EN. Curado y chico para
			// minimizar falsos positivos; la invalidación es reversible (re-afirmar revive).
			SingleValuedPredicates: []string{
				"trabaja_en", "works_at",
				"estado_actual", "current_status", "status",
				"vive_en", "lives_in",
				"ubicado_en", "located_in",
				"reporta_a", "reports_to",
				"asignado_a", "assigned_to",
				"pertenece_a", "belongs_to",
				"prioridad", "priority",
				"version_actual", "current_version",
				"responsable", "owner",
			},
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
			Enabled:                     true,
			MaxBatchUnits:               50,
			AvoidedContextTokensPerUnit: 4000,
			DelegationOverheadTokens:    2000,
			LeaseTTLSeconds:             300,
			MaxAttempts:                 5,
		},
		VectorIndex: VectorIndexConfig{
			Enabled:         true,
			ExactThreshold:  10000,
			NProbe:          8,
			NumCentroids:    0,
			RebuildEvery:    5000,
			RebuildMinHours: 6,
			KMeansIters:     10,
			KMeansSample:    50000,
		},
		Service: ServiceConfig{
			Enabled:               false,
			Addr:                  "127.0.0.1:7717",
			RequestTimeoutSeconds: 60,
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

// normalizeBrevityMode acota brevity_mode al conjunto válido {lite,full,ultra,auto};
// cualquier otro valor (incluido vacío o con espacios/mayúsculas) degrada a "off".
func normalizeBrevityMode(m string) string {
	switch strings.ToLower(strings.TrimSpace(m)) {
	case "lite":
		return "lite"
	case "full":
		return "full"
	case "ultra":
		return "ultra"
	case "auto":
		return "auto"
	default:
		return "off"
	}
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
	if c.Embedding.APIKeyEnv == "" {
		c.Embedding.APIKeyEnv = d.Embedding.APIKeyEnv
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
		if c.Sourcing.MarketplaceURL == "" {
			c.Sourcing.MarketplaceURL = d.Sourcing.MarketplaceURL
		}
		if c.Sourcing.MarketplaceCatalogURL == "" {
			c.Sourcing.MarketplaceCatalogURL = d.Sourcing.MarketplaceCatalogURL
		}
	}

	// Memory: ausente -> default completo; presente -> rellenar los numéricos del
	// recall (0 nunca es un valor útil ahí) PERO respetar session_token_budget tal cual
	// (0 = sin techo, opt-out explícito; no se pisa con el default).
	if !present["memory"] {
		c.Memory = d.Memory
	} else {
		if c.Memory.RecallTokenBudget == 0 {
			c.Memory.RecallTokenBudget = d.Memory.RecallTokenBudget
		}
		if c.Memory.GistMaxTokens == 0 {
			c.Memory.GistMaxTokens = d.Memory.GistMaxTokens
		}
		if c.Memory.CandidatePool == 0 {
			c.Memory.CandidatePool = d.Memory.CandidatePool
		}
	}
	// brevity_mode se normaliza siempre (presente o no): un valor desconocido o vacío
	// degrada a "off" para que un typo nunca encienda la directiva de salida.
	c.Memory.BrevityMode = normalizeBrevityMode(c.Memory.BrevityMode)

	// Maintenance: ausente -> default completo; presente -> rellenar numéricos y
	// respetar auto_interval_hours tal cual (0 = desactivado explícito).
	if !present["maintenance"] {
		c.Maintenance = d.Maintenance
		// La purga (PurgeArchivedAfterDays) es hard-delete IRREVERSIBLE: NO se habilita
		// por un upgrade silencioso. Un config sin bloque `maintenance` (minimal a mano,
		// o anterior a la purga) queda con la purga DESACTIVADA; solo se activa cuando el
		// campo está EXPLÍCITO en el yaml (lo escribe `musubi init` con el default 90,
		// visible y editable). Así un upgrade nunca borra memorias sin opt-in del usuario.
		c.Maintenance.PurgeArchivedAfterDays = 0
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
	// nil (ausente) -> default curado; lista vacía explícita ([]) -> se respeta (opt-out
	// total de la invalidación por cardinalidad).
	if c.Graph.SingleValuedPredicates == nil {
		c.Graph.SingleValuedPredicates = d.Graph.SingleValuedPredicates
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
	} else {
		if c.MultiAgent.MaxBatchUnits == 0 {
			c.MultiAgent.MaxBatchUnits = d.MultiAgent.MaxBatchUnits
		}
		if c.MultiAgent.AvoidedContextTokensPerUnit == 0 {
			c.MultiAgent.AvoidedContextTokensPerUnit = d.MultiAgent.AvoidedContextTokensPerUnit
		}
		if c.MultiAgent.DelegationOverheadTokens == 0 {
			c.MultiAgent.DelegationOverheadTokens = d.MultiAgent.DelegationOverheadTokens
		}
		if c.MultiAgent.LeaseTTLSeconds == 0 {
			c.MultiAgent.LeaseTTLSeconds = d.MultiAgent.LeaseTTLSeconds
		}
		if c.MultiAgent.MaxAttempts == 0 {
			c.MultiAgent.MaxAttempts = d.MultiAgent.MaxAttempts
		}
	}

	// VectorIndex: ausente -> default completo (Enabled true); presente -> respetar
	// enabled (incluido false) y rellenar numéricos. NumCentroids 0 = auto (válido).
	if !present["vector_index"] {
		c.VectorIndex = d.VectorIndex
	} else {
		if c.VectorIndex.ExactThreshold == 0 {
			c.VectorIndex.ExactThreshold = d.VectorIndex.ExactThreshold
		}
		if c.VectorIndex.NProbe == 0 {
			c.VectorIndex.NProbe = d.VectorIndex.NProbe
		}
		if c.VectorIndex.RebuildEvery == 0 {
			c.VectorIndex.RebuildEvery = d.VectorIndex.RebuildEvery
		}
		if c.VectorIndex.RebuildMinHours == 0 {
			c.VectorIndex.RebuildMinHours = d.VectorIndex.RebuildMinHours
		}
		if c.VectorIndex.KMeansIters == 0 {
			c.VectorIndex.KMeansIters = d.VectorIndex.KMeansIters
		}
		if c.VectorIndex.KMeansSample == 0 {
			c.VectorIndex.KMeansSample = d.VectorIndex.KMeansSample
		}
	}

	// Service: ausente -> default completo (Enabled false); presente -> respetar
	// enabled (incluido false) y rellenar los campos no fijados.
	if !present["service"] {
		c.Service = d.Service
	} else {
		if c.Service.Addr == "" {
			c.Service.Addr = d.Service.Addr
		}
		if c.Service.RequestTimeoutSeconds == 0 {
			c.Service.RequestTimeoutSeconds = d.Service.RequestTimeoutSeconds
		}
	}
}
