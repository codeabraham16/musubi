package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	// Provider: none | ollama | openai | static. "none" (o vacío) AUTO-DETECTA (16.2f): si hay
	// una tabla en <workspace>/.musubi/embeddings/<default> (la que baja `musubi embed pull`)
	// el entrypoint enciende "static" solo; si no, queda en recall léxico (degradación elegante).
	Provider   string `yaml:"provider"`    // none | ollama | openai | static
	Model      string `yaml:"model"`       // ej. nomic-embed-text, text-embedding-3-small
	BaseURL    string `yaml:"base_url"`    // ej. http://localhost:11434, https://api.openai.com/v1
	Dimensions int    `yaml:"dimensions"`  // dimensión del vector que produce el modelo
	APIKeyEnv  string `yaml:"api_key_env"` // nombre de la env var con la API key (openai); el secreto NO se guarda en el yaml
	// StaticPath es el directorio con model.safetensors + tokenizer.json (formato
	// model2vec/POTION) para el provider "static": embeddings model-free AT INFERENCE
	// (lookup + mean-pool, sin runtime de modelo ni cgo). Bring-your-own-table.
	StaticPath string `yaml:"static_path,omitempty"`
	// Gateway es el portero de privacidad que se para entre el texto de Musubi y un embedder que
	// habla por un socket (`ollama`, `openai`). Nace ENCENDIDO, como el de la cognición: es una
	// guarda de seguridad y el default seguro es estar protegido.
	//
	// No afecta a `none` ni a `static`: esos no mandan texto a ningún lado, así que no hay frontera
	// que cuidar y el camino sin red queda bit-idéntico.
	Gateway GatewayConfig `yaml:"gateway,omitempty"`
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
	// TeamMode hace que la captura de este proyecto sea CENTRAL por naturaleza (C5.2 del track
	// captura-automática de equipo): con true, una observación guardada SIN scope explícito se
	// persiste como 'shared' (candidata al cerebro central) en vez de 'local' — la pieza que hace
	// fluir la memoria de un proyecto de equipo al central sin pedirlo. Default false (off) ⇒
	// comportamiento histórico (captura local). Un scope explícito ('local'/'shared') siempre se
	// respeta (escape hatch); la redacción de secretos corre igual en el borde a 'shared'.
	TeamMode bool `yaml:"team_mode"`
	// RecallGraphCentrality activa la 5ª señal RRF del recall (B4): centralidad de grafo por
	// Personalized PageRank sobre observation_relations (HippoRAG), que favorece las
	// observaciones más CENTRALES en la telaraña semántica de la memoria. Rerank del pool
	// existente (no incorpora candidatos nuevos), model-free y derivada al vuelo. Default ON;
	// se desactiva con recall_graph_centrality: false. Un bloque `memory` presente pero sin
	// la clave conserva el default ON (ver applyMemoryDefaults).
	RecallGraphCentrality bool `yaml:"recall_graph_centrality"`
	// RecallCooccurrence activa la 6ª señal RRF del recall (Track 14 #2, semántica model-free):
	// expansión por pseudo-relevance feedback (PRF) — trae observaciones con vocabulario distinto
	// pero co-ocurrente en el corpus (puente 'deploy'↔'despliegue'), derivado del corpus, sin
	// embedder externo. Default ON; se desactiva con recall_cooccurrence: false. Un bloque
	// `memory` presente pero sin la clave conserva el default ON (ver applyMemoryDefaults).
	RecallCooccurrence bool `yaml:"recall_cooccurrence"`
	// RecallStemming activa el match por PREFIJO de raíz en el FTS del recall (Track 14 #2, 2ª
	// ola): la query matchea variantes morfológicas de sufijo (deploy/deploys/deployment) sin
	// re-indexar ni dependencia. Default ON; se desactiva con recall_stemming: false. Un bloque
	// `memory` presente pero sin la clave conserva el default ON (ver applyMemoryDefaults).
	RecallStemming bool `yaml:"recall_stemming"`
	// VectorFloor es el piso de coseno (0..1) del pool vectorial del recall híbrido (Q1): los
	// candidatos por similitud con coseno < VectorFloor se descartan ANTES de entrar al ranking,
	// para no inyectar vecinos de baja señal con peso RRF pleno (el defecto era rankear hasta 50
	// vecinos sin umbral, un coseno 0.42 pesando igual que 0.95). Default 0.30 (conservador:
	// descarta la cola claramente irrelevante sin tocar la banda relevante ~0.40-0.50). Un bloque
	// `memory` presente pero sin la clave conserva el default (ver applyMemoryDefaults);
	// `vector_floor: 0` lo desactiva (comportamiento histórico, sin piso).
	VectorFloor float64 `yaml:"vector_floor"`
	// MMRLambda es el dial de DIVERSIDAD del recall (MMR). Pondera relevancia contra REDUNDANCIA al
	// elegir el orden en que se gasta el presupuesto de tokens: una candidata que repite lo que ya
	// se eligió BAJA de posición (nunca se descarta).
	//
	// Por qué hace falta: el ranker optimiza relevancia POR ITEM, pero el presupuesto es del
	// CONJUNTO. Medido en la memoria real, una consulta gastaba un TERCIO del presupuesto en 3
	// cambios contados SIETE VECES cada uno (las 7 fases SDD), enterrando memoria más útil.
	//
	// 1 (o menos) APAGA MMR: sólo relevancia, orden bit-idéntico al histórico. El default sale de
	// MEDIR contra el recall-gate (R@10), no de estimar.
	MMRLambda float64 `yaml:"mmr_lambda"`
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
	// DecayReinforcementK es la fuerza del refuerzo de Ebbinghaus (B3): cada acceso alarga
	// la vida media efectiva de la recencia, de modo que las memorias frecuentemente
	// accedidas ("calientes") se olvidan más lento. 0 = vida media fija (comportamiento
	// histórico). El default activa un refuerzo moderado en el daemon.
	DecayReinforcementK float64 `yaml:"decay_reinforcement_k"`
	// AutoIntervalHours es cada cuántas horas corre el auto-mantenimiento al
	// arrancar el daemon (0 = desactivado; el mantenimiento manual sigue disponible).
	AutoIntervalHours float64 `yaml:"auto_interval_hours"`
	// GraphIndexHours es cada cuántas horas el daemon re-indexa el grafo de código de forma
	// INCREMENTAL (0 = desactivado).
	//
	// POR QUÉ EXISTE: hasta ahora el grafo sólo se indexaba si un agente llamaba
	// musubi_codegraph_index a mano — no hay subcomando CLI, así que ni un hook de git ni un timer
	// podían hacerlo. Medido el 2026-08-15: el grafo del central estaba fechado el día anterior y no
	// contenía los cuatro PRs de esa jornada. Un grafo rancio no falla ruidosamente: contesta, y
	// contesta sobre código que ya no existe.
	//
	// Cuelga del mismo daemon que ya corre el auto-mantenimiento porque el índice incremental es
	// barato cuando no cambió nada: compara el fingerprint de cada archivo contra el guardado y sólo
	// re-deriva los paquetes sucios. Con el árbol quieto, una corrida es leer fingerprints.
	GraphIndexHours float64 `yaml:"graph_index_hours"`
	// PurgeArchivedAfterDays borra DEFINITIVAMENTE las observaciones archivadas que no
	// se tocaron en esta cantidad de días (retención dura, acota el crecimiento). El
	// olvido (decay) solo marca archived; esto las elimina de verdad. 0 = nunca purgar.
	PurgeArchivedAfterDays float64 `yaml:"purge_archived_after_days"`
	// MaxActivePerProject es la CUOTA DE CRECIMIENTO: el techo de observaciones ACTIVAS por
	// tenant (project_id). Cuando un proyecto lo supera, el mantenimiento archiva sus memorias
	// más frías (menor saliencia, reversible) hasta volver bajo el techo — el bound que el
	// olvido por umbral no garantiza en un tenant de alto ingest. Respeta la protección por
	// importancia y nunca evicciona memoria sin sincronizar. 0 = sin cuota. Es lo que acota el
	// crecimiento del cerebro central 24/7; bajalo si querés un techo más ajustado.
	MaxActivePerProject int `yaml:"max_active_per_project"`
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
	// AutoDistillMinutes es cada cuántos minutos el daemon destila una tanda chica del acervo de
	// diseño (blobs `ingested/*` → tarjetas `design-corpus/*`) sin intervención. 0 = desactivado
	// (default; opt-in). Es el "molino continuo" del pilar Musubi Renaissance: material ingerido que
	// se destila solo. Requiere un motor de cognición; sin él, el scheduler es un no-op. La tanda se
	// mantiene chica a propósito — el throughput lo limita el rate-limit del motor, no el código —, así
	// que corre espaciada y va drenando el backlog de a poco sin saturar el endpoint.
	AutoDistillMinutes float64 `yaml:"auto_distill_minutes"`
	// AutoDistillBatch es cuántos blobs destila cada tick del auto-drain (default 3 cuando está activo).
	// Chico y espaciado: dosifica el gasto y no satura el endpoint de cognición.
	AutoDistillBatch int `yaml:"auto_distill_batch"`
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
	// CosineFloor es el piso de COSENO para que una candidata puramente SEMÁNTICA (mismo
	// significado, otras palabras ⇒ Jaccard bajo) entre como `pending` (default 0.85). En 0 el
	// coseno no participa ⇒ comportamiento léxico histórico (interruptor de rollback).
	//
	// OJO — esta escala NO es la de memory.vector_floor: allá se compara QUERY vs documento (sims
	// 0.40-0.50); acá es documento vs DOCUMENTO, y la línea de base es mucho más alta. Medido sobre
	// 77k pares reales: dos observaciones NO relacionadas ya dan ~0.60 de coseno mediano (texto del
	// mismo dominio) y el ruido llega hasta 0.884; los casi-duplicados reales están en ~0.99.
	// Reusar acá el 0.30 del recall marcaría CASI TODO como duplicado.
	CosineFloor float64 `yaml:"cosine_floor"`
	// CosineAutoThreshold es el coseno mínimo para que el coseno CORROBORE una auto-resolución
	// (default 0.90: 0 falsos positivos en 77k pares medidos). El coseno nunca auto-resuelve SOLO:
	// hace falta ADEMÁS similitud léxica alta (AND-gate). Ver conflicts.go.
	CosineAutoThreshold float64 `yaml:"cosine_auto_threshold"`
	// BandFloor es el piso de la BANDA CIEGA: [BandFloor, CosineFloor). Ahí viven las
	// CONTRADICCIONES — "mismo tema, dicho distinto" —, que por debajo del piso del dedup son
	// invisibles. Los vecinos de esta banda se le MUESTRAN al agente al guardar, y NO se encolan:
	// no se crea ninguna relación (mostrar no es encolar).
	//
	// Por qué existe una banda separada: el piso del dedup (0.85) está calibrado sobre DUPLICADOS
	// (los casi-idénticos dan ~0.99). Una contradicción NO es un duplicado — decir lo contrario usa
	// OTRAS palabras — así que vive estructuralmente MÁS ABAJO. Un solo umbral no puede hacer los
	// dos trabajos. MEDIDO: el par contradictorio real (NordVPN↔Tailscale) da coseno 0.806, y el p99
	// de los 94.830 pares reales es 0.803 ⇒ 0.80 deja entrar ~el 1% más similar.
	//
	// OJO: 0.80 sale de UNA medición sobre UNA memoria. Es una heurística calibrada, no una verdad.
	// En 0 (o >= CosineFloor) la banda se APAGA: el save responde exactamente como antes.
	BandFloor float64 `yaml:"band_floor"`
	// LedgerPrefixes son prefijos de topic_key que se tratan como LIBRO MAYOR: se leen y se citan,
	// pero nadie puede pedir un veredicto que los REEMPLACE. Vacío (el default) = nadie, así que
	// una instalación que no lo declara se comporta exactamente como antes.
	//
	// POR QUÉ ES CONFIGURABLE Y NO VA EN EL CÓDIGO. `git-commit` y `sdd/` los conoce el motor
	// porque LOS ESCRIBE ÉL: son artefactos de Musubi. Pero cada equipo inventa géneros propios que
	// tampoco se pueden tachar —correspondencia entre agentes, actas, bitácoras— y esos son
	// convención del DESPLIEGUE, no del producto. Hardcodearlos metería la costumbre de un usuario
	// adentro del motor de todos.
	//
	// EL CASO QUE LO MOTIVÓ, MEDIDO en el cerebro central el 2026-08-17: 465 relaciones pendientes,
	// el 83% apretadas en la franja 0,30-0,35, pegada al piso del detector. No eran contradicciones:
	// eran las 27 notas `terminales/` —despachos entre agentes— pareándose entre sí por la PLANTILLA
	// que comparten (cabeceras, emoji, nombres de destinatario). 27×26/2 = 351, el grueso de la cola.
	// Dos cartas a destinatarios distintos no pueden contradecirse, así que esos pares nunca iban a
	// producir un veredicto — y una cola que no se puede drenar deja de leerse ENTERA, incluida la
	// contradicción real que aparezca mañana.
	LedgerPrefixes []string `yaml:"ledger_prefixes,omitempty"`
	// Shadow enciende el MODO SOMBRA: por cada veredicto del detector, preguntarle también al
	// motor de cognición y guardar las dos lecturas lado a lado. La del motor se descarta.
	Shadow ShadowConfig `yaml:"shadow,omitempty"`
}

// ShadowConfig configura el modo sombra del detector de conflictos.
//
// NO TIENE MUESTREO, y es a propósito. El central emite del orden de 90 veredictos por día; una
// llamada al motor por veredicto es un gasto chico y acotado, mientras que muestrear agregaría un
// generador de azar al camino de guardado —irreproducible en los tests— para ahorrar poco. Si el
// volumen molesta, el interruptor correcto es apagarlo entero, que ya existe.
type ShadowConfig struct {
	// Enabled nace en false: el modo sombra gasta motor y no mejora ninguna respuesta. Se
	// enciende para medir durante un tiempo y se apaga.
	Enabled bool `yaml:"enabled"`
	// Queue es el tope de veredictos esperando al motor (default 64). Lo que no entra se DESCARTA
	// y se cuenta: preferimos perder evidencia a que un pico de guardados haga crecer la cola sin
	// techo, porque la sombra no puede degradar el camino que sí importa.
	Queue int `yaml:"queue"`
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
	// PrincipalsFile es la ruta del registro de identidad por-principal (Track 16 F1
	// 16.1c): un YAML que mapea el SHA-256 de cada token a {name, project_id, role}.
	// Vacío => se prueba el default .musubi/principals.yaml; si tampoco existe, modo
	// legacy (un único bearer, sin roles). El archivo NUNCA contiene el token crudo.
	PrincipalsFile string `yaml:"principals_file,omitempty"`
	// ForceRedact fuerza la redacción de secretos en TODO ingest (Track 16 F1 16.1d),
	// independiente del scope declarado por el cliente. Un bind NO-loopback la enciende
	// SIEMPRE (fail-closed: el central es infra compartida); este flag permite ADEMÁS
	// activarla en un bind loopback. No se puede desactivar en no-loopback.
	ForceRedact bool `yaml:"force_redact,omitempty"`
	// QuotaPerMinute limita las llamadas a tools/call POR PRINCIPAL por minuto (Track 16 F3.2):
	// protege al cerebro central de un principal desbocado. Track 18 la enciende por default:
	// 0 ⇒ default generoso (ver EffectiveQuotaPerMinute); NEGATIVO ⇒ sin límite (opt-out
	// explícito); >0 ⇒ ese límite. Solo aplica cuando hay un principal autenticado (serve con
	// registro); en stdio local no hay cuota. Es por identidad de principal, no por IP.
	QuotaPerMinute int `yaml:"quota_per_minute,omitempty"`
	// StrictTenancy (Track 18) exige, en un bind NO-loopback, un registro de principals con al
	// menos un miembro: rechaza arrancar en "legacy admin-federado" (un único bearer con acceso
	// total a todos los proyectos). Default false (backward-compat); un WARNING de arranque avisa
	// del modo legacy en bind remoto aunque esté apagado.
	StrictTenancy bool `yaml:"strict_tenancy,omitempty"`
}

// defaultServiceQuotaPerMinute es la cuota por-principal aplicada cuando QuotaPerMinute==0
// (Track 18): protege al central por default sin lastimar el uso normal (600/min = 10/s
// sostenidos por principal es holgado para un agente). Un valor negativo la desactiva a conciencia.
const defaultServiceQuotaPerMinute = 600

// EffectiveQuotaPerMinute resuelve la cuota efectiva desde QuotaPerMinute: 0 ⇒ default (protección
// ON, Track 18); <0 ⇒ sin límite (opt-out explícito); >0 ⇒ ese valor. Cablearla en vez de leer
// QuotaPerMinute directo evita cambiar la semántica del cero en YAMLs existentes.
func (c ServiceConfig) EffectiveQuotaPerMinute() int {
	switch {
	case c.QuotaPerMinute == 0:
		return defaultServiceQuotaPerMinute
	case c.QuotaPerMinute < 0:
		return 0
	default:
		return c.QuotaPerMinute
	}
}

// defaultMotorQuotaPerHour es el freno del motor cuando MotorQuotaPerHour==0.
//
// EL NÚMERO SALE DE MEDIR, no de elegirlo lindo: en el cerebro central se hicieron 3 llamadas al
// motor en 30 días. 60 por hora es ~150 veces esa tasa —holgado para una tarde de trabajo intenso, y
// holgado también si el juez read-time se enciende sobre los 14 recalls/mes que hoy ve el central—
// pero acota un BUCLE: un cliente desbocado quema 60 llamadas en una hora, no 36.000.
const defaultMotorQuotaPerHour = 60

// EffectiveMotorQuotaPerHour resuelve el freno del motor: 0 ⇒ default (protección ON); <0 ⇒ sin
// límite (opt-out explícito); >0 ⇒ ese valor. Idéntica a EffectiveQuotaPerMinute a propósito: son
// dos frenos hermanos y una semántica distinta entre ellos sería una trampa.
func (c CognitionConfig) EffectiveMotorQuotaPerHour() int {
	switch {
	case c.MotorQuotaPerHour == 0:
		return defaultMotorQuotaPerHour
	case c.MotorQuotaPerHour < 0:
		return 0
	default:
		return c.MotorQuotaPerHour
	}
}

// SyncConfig configura el sync SALIENTE offline-first del cerebro híbrido (F2): el drain
// del outbox que empuja las observaciones 'shared' al `musubi serve` central por HTTP
// JSON-RPC. Está DESACTIVADO por defecto (Enabled=false): un workspace sin bloque `sync:`
// mantiene intacto el comportamiento local-first (no drena nada). Bloque YAML: sync.
type SyncConfig struct {
	// Enabled activa el drain del outbox (default false). Con false NO se sincroniza nada,
	// aunque el enqueue al outbox sigue ocurriendo (la intención durable se registra igual;
	// habilitar sync después no pierde lo previo).
	Enabled bool `yaml:"enabled"`
	// CentralURL es la base del cerebro central, https://host:port SIN /mcp (el cliente le
	// agrega el path). Vacío ⇒ no se drena aunque enabled sea true.
	CentralURL string `yaml:"central_url"`
	// AuthTokenEnv es el NOMBRE de la env var con el bearer token del central (patrón de
	// EmbeddingConfig.APIKeyEnv: el secreto NUNCA va en el YAML ni se loguea).
	AuthTokenEnv string `yaml:"auth_token_env"`
	// DrainIntervalSeconds es cada cuántos segundos corre el drain (default 30). <=0 desactiva.
	DrainIntervalSeconds int `yaml:"drain_interval_seconds"`
	// BatchSize es el tope de filas reclamadas por tick (default 50).
	BatchSize int `yaml:"batch_size"`
	// MaxAttempts es la cantidad de intentos transitorios antes de mandar la fila a
	// dead-letter (default 5).
	MaxAttempts int `yaml:"max_attempts"`
	// BackoffBaseSeconds es la base del backoff exponencial entre reintentos (default 5).
	BackoffBaseSeconds int `yaml:"backoff_base_seconds"`
	// BackoffMaxSeconds es el tope del backoff (default 300 = 5 min).
	BackoffMaxSeconds int `yaml:"backoff_max_seconds"`
	// LeaseSeconds es la vida del lease de un claim del outbox (default 60): si el drain
	// crashea a mitad de entrega, la fila se re-reclama sola al vencer.
	LeaseSeconds int `yaml:"lease_seconds"`
	// RequestTimeoutSeconds es el timeout por POST al central (default 30).
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds"`
	// AllowInsecureToken permite un CentralURL http:// (token en texto plano). Default false
	// => fail-closed: sólo tailnet/dev con opt-in explícito.
	AllowInsecureToken bool `yaml:"allow_insecure_token"`
}

// HasDestination responde la ÚNICA pregunta que importa para el outbox: ¿este nodo tiene a
// dónde empujar? Existe porque la respuesta se venía calculando de dos maneras distintas, y
// esa discrepancia le costó al cerebro central una métrica de salud entera.
//
// El arranque purgaba las filas huérfanas con `!Enabled || CentralURL == ""` (bien: mide el
// DESTINO), mientras que el gate del encolado miraba sólo `Enabled` (mal: mide la INTENCIÓN).
// Un nodo con `enabled: true` y sin `central_url` caía en la grieta: encolaba sin destino y
// después se purgaba a sí mismo. Con un solo predicado los tres lugares no pueden discrepar.
func (s SyncConfig) HasDestination() bool {
	return s.Enabled && strings.TrimSpace(s.CentralURL) != ""
}

// UpdateConfig controla el chequeo de nuevas versiones del binario al arrancar.
type UpdateConfig struct {
	// CheckIntervalHours es cada cuántas horas el daemon chequea si hay una
	// versión nueva y avisa por stderr (default 24). Un valor negativo lo desactiva.
	CheckIntervalHours float64 `yaml:"check_interval_hours"`
}

// CognitionConfig configura el pilar Cognición (LLM), el 3er pilar de Musubi. OPT-IN: con
// Provider vacío o "none" el pilar nace APAGADO (NoopProvider) y el binario se comporta
// bit-idéntico a un Musubi model-free. El contrato del pilar es que el LLM PROPONE, nunca
// escribe directo al libro mayor durable. El secreto (si el motor lo requiere) se lee de la
// env var NOMBRADA en AuthTokenEnv, nunca de este YAML.
type CognitionConfig struct {
	Provider              string `yaml:"provider,omitempty"`                // "" | none (F1+: motores reales)
	Model                 string `yaml:"model,omitempty"`                   // modelo del motor (para procedencia)
	Endpoint              string `yaml:"endpoint,omitempty"`                // endpoint del motor (tailnet), si aplica
	AuthTokenEnv          string `yaml:"auth_token_env,omitempty"`          // NOMBRE de la env var del secreto
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds,omitempty"` // timeout por llamada (0 => default del motor)
	// AllowedPredicates es el vocabulario controlado de PREDICADOS para las propuestas LLM (F3):
	// musubi_propose_facts rechaza una tripleta cuyo predicado no esté acá (comparación
	// case-insensitive). Vacío ⇒ allow-all (bit-idéntico). No afecta a save_fact autoritativo.
	AllowedPredicates []string `yaml:"allowed_predicates,omitempty"`
	// ProposalTTLHours es el TTL de CUARENTENA de una propuesta LLM no corroborada (F3): el
	// mantenimiento invalida las propuestas ('llm-extract:*') vivas más viejas que esto. 0 ⇒
	// nunca barrer (bit-idéntico). Las corroboradas (source=agent) nunca se barren.
	ProposalTTLHours float64 `yaml:"proposal_ttl_hours,omitempty"`
	// EntityResolutionThreshold es el umbral (0..1) de la resolución de entidades DETERMINISTA
	// para propuestas LLM (F4): al proponer, un subject/object sin match exacto pero con Similarity
	// (Jaccard de trigramas) >= umbral contra una entidad existente se CANONICALIZA a ella, para no
	// fragmentar el grafo con variantes. 0 ⇒ desactivado (bit-idéntico). No afecta a save_fact.
	EntityResolutionThreshold float64 `yaml:"entity_resolution_threshold,omitempty"`
	// ReadTimeRerank activa el juez de pertinencia LLM en el RECALL (F3.5c): tras el ranking
	// model-free, el motor re-ordena los primeros candidatos por relevancia a la consulta. Es el
	// seam de MAYOR riesgo (latencia en el camino caliente + rate-limit), por eso nace APAGADO
	// (false ⇒ bit-idéntico: el recall sigue 100% model-free) y es selectivo/cacheado/best-effort:
	// ante cualquier fallo o timeout se mantiene el orden model-free. Sólo re-ordena, no descarta.
	//
	// Es *bool y no bool desde F5: el dial de potencia necesita distinguir "no lo escribieron"
	// (⇒ lo decide el preset) de "lo escribieron en false" (⇒ el preset NO lo pisa). Con un bool
	// pelado las dos cosas son el cero de Go y lo explícito se perdería. Usar ReadTimeRerankOn().
	ReadTimeRerank *bool `yaml:"read_time_rerank,omitempty"`
	// ReadTimeRerankTopK es cuántos candidatos del tope se someten al juez (el resto queda intacto
	// al final). 0 ⇒ default interno. Acota latencia y costo: el juez nunca ve todo el recall.
	ReadTimeRerankTopK int `yaml:"read_time_rerank_top_k,omitempty"`
	// MotorQuotaPerHour es el FRENO DE GASTO del motor: cuántas llamadas al modelo puede provocar
	// UN principal por hora. Existe porque `quota_per_minute` cuenta todas las tools por igual, y su
	// default (600/min) está calibrado para tools gratis: aplicado al motor no es un límite.
	//
	// Cuenta LLAMADAS, no tokens ni dinero — es lo que el sistema puede saber por sí mismo, sin
	// depender de que un proveedor reporte bien ni de una tabla de precios que envejece.
	//
	// 0 ⇒ default (ver EffectiveMotorQuotaPerHour); NEGATIVO ⇒ sin límite. La MISMA semántica que
	// quota_per_minute, a propósito: dos números que se parecen y significan cosas distintas es la
	// forma más barata de que alguien se apague el freno creyendo que lo apretaba.
	MotorQuotaPerHour int `yaml:"motor_quota_per_hour,omitempty"`
	// Gateway es el portero de privacidad que se para entre la memoria y el motor externo. A
	// diferencia del resto del pilar, nace ENCENDIDO: es una guarda de seguridad, y el default
	// seguro es estar protegido. No rompe la bit-identidad model-free porque sólo actúa cuando la
	// cognición ya está encendida (que sí es opt-in).
	Gateway GatewayConfig `yaml:"gateway,omitempty"`
	// Fleet es la flota ordenada de motores (F2). Vacía ⇒ se usa el motor único de los campos de
	// arriba y no se instancia ningún router: el comportamiento es bit-idéntico al de F1.
	//
	// Con flota, el router prueba los motores EN ORDEN, saltea los que el circuit breaker tiene
	// abiertos, y escala al siguiente cuando uno se niega por política.
	Fleet []FleetEngineConfig `yaml:"fleet,omitempty"`
	// Breaker configura el circuit breaker por motor de la flota.
	Breaker BreakerConfig `yaml:"breaker,omitempty"`
	// Cache es el caché de respuestas del motor (F3): responde sin llamar cuando ya se preguntó
	// lo mismo. Como el portero, sólo actúa cuando la cognición ya está encendida, así que no
	// afecta la bit-identidad del camino model-free.
	Cache CacheConfig `yaml:"cache,omitempty"`
	// Effort es el DIAL DE POTENCIA (F5): un solo parámetro en vez de coordinar a mano las
	// perillas de arriba. "" ⇒ no se aplica ningún preset y todo queda como estaba.
	Effort string `yaml:"effort,omitempty"`
}

// UsageLedgerConfig configura el LEDGER DE USO (F0 · track «Potencia medida»): la historia
// persistente de qué tools se invocaron, cuánto tardaron y cómo terminaron.
//
// NACE ENCENDIDO, y es una decisión: no cambia el comportamiento de ninguna tool, no manda nada
// afuera y no guarda contenido — así que no tiene contraindicación. Y sobre todo, un medidor
// disponible-para-apagar termina apagado, que es literalmente el problema que esta fase vino a
// arreglar (los contadores que ya existían morían en cada reinicio y nadie se enteró en dos meses).
type UsageLedgerConfig struct {
	// Enabled es *bool para distinguir "no lo escribieron" (⇒ encendido) de "lo apagaron a
	// propósito" (false). Con un bool pelado el cero de Go haría que omitir el bloque apagara
	// el ledger, que es el default equivocado. Usar EnabledOn().
	Enabled *bool `yaml:"enabled,omitempty"`
	// FlushIntervalSeconds es cada cuánto baja el buffer a la base. Más chico pierde menos ante
	// una muerte súbita del proceso y escribe más seguido. 0 ⇒ default.
	FlushIntervalSeconds int `yaml:"flush_interval_seconds,omitempty"`
	// RetentionDays es cuánto se conserva. La purga cuelga del mantenimiento que ya existe.
	// 0 ⇒ default. Negativo ⇒ nunca purgar (a tu riesgo).
	RetentionDays int `yaml:"retention_days,omitempty"`
}

// Defaults del ledger de uso.
const (
	defaultLedgerFlushSeconds  = 10
	defaultLedgerRetentionDays = 90
)

// EnabledOn resuelve el *bool: ausente ⇒ ENCENDIDO.
func (c UsageLedgerConfig) EnabledOn() bool { return c.Enabled == nil || *c.Enabled }

// EffectiveFlushSeconds aplica el default sin que el caller tenga que conocerlo.
func (c UsageLedgerConfig) EffectiveFlushSeconds() int {
	if c.FlushIntervalSeconds <= 0 {
		return defaultLedgerFlushSeconds
	}
	return c.FlushIntervalSeconds
}

// EffectiveRetentionDays aplica el default. Un valor negativo se respeta y significa "no purgar".
func (c UsageLedgerConfig) EffectiveRetentionDays() int {
	if c.RetentionDays == 0 {
		return defaultLedgerRetentionDays
	}
	return c.RetentionDays
}

// CacheConfig configura el caché de cognición (F3).
type CacheConfig struct {
	// Enabled: nace ENCENDIDO cuando hay motor real. Ahorrar una llamada idéntica no tiene
	// contraindicación, y el pilar entero ya es opt-in.
	//
	// Es *bool y no bool para distinguir "no lo escribieron" (⇒ default true) de "lo apagaron
	// a propósito" (false). Con un bool pelado, el cero de Go haría que omitir el bloque
	// apagara el caché, que es lo contrario del default que se quiere.
	Enabled *bool `yaml:"enabled,omitempty"`
	// MaxEntries es la cota DURA de entradas. Al llegar se desaloja la usada hace más tiempo,
	// UNA, no todas. 0 ⇒ default interno; negativo con el caché encendido es error de config,
	// no un default silencioso: un caché sin cota es una fuga de memoria con nombre amable.
	MaxEntries int `yaml:"max_entries,omitempty"`
	// TTLSeconds vence las entradas. 0 ⇒ sin vencimiento.
	//
	// El vencimiento importa menos de lo que parece porque el prompt de `musubi_ask` LLEVA
	// ADENTRO la memoria recuperada: si la memoria cambia, cambia el prompt y por lo tanto la
	// clave. El TTL cubre lo que esa propiedad no cubre — que el motor mismo mejore, o que la
	// respuesta dependa de algo que no está en el prompt.
	TTLSeconds int `yaml:"ttl_seconds,omitempty"`
}

// CacheEnabled resuelve el default de Enabled: ausente ⇒ true.
func (c CacheConfig) CacheEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// DefaultCacheMaxEntries es la cota por defecto. 512 es el mismo número que usaba el rerankCache
// que este caché reemplaza, así que no cambia el perfil de memoria de una instalación existente.
const DefaultCacheMaxEntries = 512

// Tiers de confianza de un motor de la flota.
const (
	// TierFree es un motor en el que NO se confía: típicamente un tier gratis que entrena con lo
	// que recibe. Es el DEFAULT a propósito — asumir "no confiable" es la dirección segura, y
	// confiar en algo tiene que declararse.
	TierFree = "free"
	// TierPrivate es un motor de confianza (p. ej. un endpoint propio en loopback o en la tailnet).
	TierPrivate = "private"
)

// FleetEngineConfig es un motor de la flota. Repite los campos del motor único a propósito: cada
// entrada se construye con la MISMA fábrica, así todo lo que vale para un motor vale para todos.
type FleetEngineConfig struct {
	// Name identifica el motor en los logs y en el diagnóstico. Vacío ⇒ se deriva del motor.
	Name                  string `yaml:"name,omitempty"`
	Provider              string `yaml:"provider,omitempty"`
	Model                 string `yaml:"model,omitempty"`
	Endpoint              string `yaml:"endpoint,omitempty"`
	AuthTokenEnv          string `yaml:"auth_token_env,omitempty"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds,omitempty"`
	// Tier es la confianza que se le tiene a este motor: free (default) o private.
	Tier string `yaml:"tier,omitempty"`
	// Gateway fuerza el modo del portero de ESTE motor. Vacío ⇒ se deriva del tier: `free` nace en
	// `refuse` (un secreto no va a un motor en el que no se confía) y `private` en `scrub`.
	Gateway GatewayConfig `yaml:"gateway,omitempty"`
}

// BreakerConfig configura el circuit breaker por motor.
type BreakerConfig struct {
	// Failures son las fallas CONSECUTIVAS que abren el circuito (0 ⇒ default 3). Una respuesta
	// exitosa resetea el contador.
	Failures int `yaml:"failures,omitempty"`
	// CooldownSeconds es cuánto queda el motor fuera de la rotación (0 ⇒ default 60). Vencido,
	// entra UNA sola llamada de prueba (half-open).
	CooldownSeconds int `yaml:"cooldown_seconds,omitempty"`
}

// EffectiveFailures aplica el default sin que el caller tenga que conocerlo.
func (b BreakerConfig) EffectiveFailures() int {
	if b.Failures <= 0 {
		return 3
	}
	return b.Failures
}

// EffectiveCooldown aplica el default sin que el caller tenga que conocerlo.
func (b BreakerConfig) EffectiveCooldown() time.Duration {
	if b.CooldownSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(b.CooldownSeconds) * time.Second
}

// NormalizeTier valida el tier y devuelve el efectivo ("" ⇒ free).
//
// El default es el conservador: un motor sin tier declarado se trata como NO confiable, así que su
// portero nace en `refuse` y un texto con secretos nunca le llega. Equivocarse por omisión no puede
// terminar en "le confié a un servicio que entrena con mis datos".
func NormalizeTier(tier string) (string, error) {
	switch tier {
	case "", TierFree:
		return TierFree, nil
	case TierPrivate:
		return TierPrivate, nil
	default:
		return "", fmt.Errorf("tier desconocido: %q (usá %q o %q)", tier, TierFree, TierPrivate)
	}
}

// DefaultGatewayModeForTier es la regla dura del roadmap hecha estructura: un motor en el que no se
// confía nace RECHAZANDO los textos con secretos, no tapándolos.
//
// Que sea un default y no una imposición es deliberado: tapar antes de mandar a un tier gratis es
// una decisión legítima del dueño de los datos (tapado no hay fuga de credenciales), pero tiene que
// escribirse a mano y el doctor la muestra.
func DefaultGatewayModeForTier(tier string) string {
	if tier == TierPrivate {
		return GatewayModeScrub
	}
	return GatewayModeRefuse
}

// GatewayConfig configura el portero de privacidad de la cognición: qué hacer con los secretos que
// aparecen en el texto que Musubi está por mandarle a un LLM externo.
//
// El default (Mode vacío ⇒ "scrub") protege. Quedarse sin portero exige escribirlo, y avisa.
type GatewayConfig struct {
	// Mode es la política ante un secreto detectado:
	//
	//	scrub  (default) tapa el secreto con un marcador reversible, manda, y repone en la respuesta.
	//	refuse           si hay un secreto, NO manda nada y devuelve error. Para motores en los que
	//	                 no se confía (p.ej. tiers gratis que entrenan con lo que reciben).
	//	off              sin portero. Hay que escribirlo a mano y deja aviso en el log.
	//
	// Un valor desconocido NO cae a un default silencioso: NewProvider devuelve error y el pilar
	// entero queda apagado (model-free). Es falla-cerrado — sin motor no hay frontera que cruzar —
	// y se ve en el log de arranque. Una config mal escrita no puede terminar en "sin protección".
	Mode string `yaml:"mode,omitempty"`
}

// Modos del portero de privacidad. Valen igual para la cognición y para los embeddings.
const (
	GatewayModeScrub  = "scrub"  // default: tapar el secreto antes de que salga
	GatewayModeRefuse = "refuse" // si hay un secreto, no se manda nada
	GatewayModeOff    = "off"    // sin portero (explícito y ruidoso)
)

// NormalizeGatewayMode valida el modo y devuelve el efectivo ("" ⇒ scrub).
//
// Vive en config y no en un pilar porque los DOS pilares que tienen portero —cognición y
// embeddings— la usan. Es la única fuente de verdad sobre qué modos existen: agregar uno no puede
// dejar a uno de los dos desactualizado, ni permitir que signifiquen cosas distintas en cada lado.
func NormalizeGatewayMode(mode string) (string, error) {
	switch mode {
	case "", GatewayModeScrub:
		return GatewayModeScrub, nil
	case GatewayModeRefuse:
		return GatewayModeRefuse, nil
	case GatewayModeOff:
		return GatewayModeOff, nil
	default:
		return "", fmt.Errorf("gateway.mode desconocido: %q (usá %q, %q u %q)",
			mode, GatewayModeScrub, GatewayModeRefuse, GatewayModeOff)
	}
}

// Config es la configuración del workspace (.musubi/config.yaml).
type Config struct {
	Version           string `yaml:"version"`
	Mode              string `yaml:"mode"`
	SkillsAutoResolve bool   `yaml:"skills_auto_resolve"`
	// ProjectID identifica este proyecto en la memoria híbrida local+central (fundación del
	// cerebro híbrido). Opcional: "" => se deriva del basename del directorio del workspace.
	// Se estampa en cada observación (columna project_id) para atribución/filtrado futuro.
	ProjectID string          `yaml:"project_id,omitempty"`
	Embedding EmbeddingConfig `yaml:"embedding"`
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
	// Sync configura el sync saliente offline-first del cerebro híbrido (F2); desactivado por defecto.
	Sync SyncConfig `yaml:"sync,omitempty"`
	// Cognition configura el 3er pilar (Cognición LLM): OPT-IN, apagado por defecto (provider "" => Noop).
	// F0 sólo cablea el andamiaje; no hace ninguna llamada real a un LLM.
	Cognition CognitionConfig `yaml:"cognition,omitempty"`
	// UsageLedger es la historia persistente de invocaciones de tools (F0 · «Potencia medida»).
	// Nace encendido; ver el comentario del tipo para el porqué.
	UsageLedger UsageLedgerConfig `yaml:"usage_ledger,omitempty"`
}

// Default devuelve la configuración por defecto (local-first, embeddings desactivados).
func Default() Config {
	return Config{
		Version:           "1.0",
		Mode:              "local",
		SkillsAutoResolve: true,
		ProjectID:         "", // "" => derivar del basename del workspace (ver resolveProjectID)
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
			RecallTokenBudget:     400,
			GistMaxTokens:         24,
			CandidatePool:         50,
			SessionTokenBudget:    8000,
			BrevityMode:           "off",
			RecallGraphCentrality: true,
			RecallCooccurrence:    true,
			RecallStemming:        true,
			VectorFloor:           0.30,
			MMRLambda:             0.75,
		},
		Maintenance: MaintenanceConfig{
			DedupThreshold:      0.85,
			DecayHalfLifeDays:   30,
			DecayMinSalience:    0.2,
			DecayMinAgeDays:     14,
			DecayReinforcementK: 0.5,
			AutoIntervalHours:   24,
			// 6 h y no 24: el grafo lo consumen musubi_impact y el precheck ANTES de escribir, o
			// sea que una respuesta rancia se paga en una decisión de código y no en una consulta
			// curiosa. Es más barato que el mantenimiento (fingerprints contra disco, sin LLM),
			// así que puede correr más seguido sin que se note.
			GraphIndexHours:        6,
			PurgeArchivedAfterDays: 90,
			MaxActivePerProject:    50000,
			Vacuum:                 true,
			// Auto-drain del acervo APAGADO por default (opt-in): sólo el central con motor de
			// cognición lo enciende. El batch por tick queda sano por si se activa sin fijarlo.
			AutoDistillMinutes: 0,
			AutoDistillBatch:   3,
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
			CosineFloor:          0.85,
			CosineAutoThreshold:  0.90,
			BandFloor:            0.80,
			// Enabled false: la sombra nace apagada. Queue igual trae su default para que
			// encenderla sea una línea (`enabled: true`) y no dos.
			Shadow: ShadowConfig{Queue: 64},
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
		Sync: SyncConfig{
			Enabled:               false,
			DrainIntervalSeconds:  30,
			BatchSize:             50,
			MaxAttempts:           5,
			BackoffBaseSeconds:    5,
			BackoffMaxSeconds:     300,
			LeaseSeconds:          60,
			RequestTimeoutSeconds: 30,
			AllowInsecureToken:    false,
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
	cfg.applyMemoryDefaults(data)
	cfg.applyConflictsDefaults(data)

	// DIAL DE POTENCIA (F5). Se resuelve ACÁ, en el único punto por el que pasa toda config
	// cargada, y no dentro de cognition.NewProvider: el servidor MCP guarda su propia copia de
	// CognitionConfig para decidir el juez del recall, así que resolverlo en la fábrica dejaría a
	// esa copia con el dial sin aplicar. Un dial que rige en la mitad de los consumidores es peor
	// que no tenerlo.
	//
	// Un `effort` mal escrito ROMPE EL ARRANQUE (D2), igual que un gateway.mode desconocido: es
	// preferible no arrancar a arrancar con una potencia distinta de la que se pidió.
	resolved, err := cfg.Cognition.ApplyEffort()
	if err != nil {
		return cfg, err
	}
	cfg.Cognition = resolved
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

// presentBlockKeys devuelve el conjunto de sub-claves presentes bajo un bloque top-level del
// YAML (p.ej. las claves dentro de `memory:`). Permite distinguir "sub-clave ausente" de
// "sub-clave presente con false", que con un bool puro es indistinguible por su cero-valor —
// necesario para los toggles default-ON anidados en un bloque presente.
func presentBlockKeys(data []byte, block string) map[string]bool {
	keys := map[string]bool{}
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return keys
	}
	node, ok := raw[block]
	if !ok || node.Kind != yaml.MappingNode {
		return keys
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keys[node.Content[i].Value] = true
	}
	return keys
}

// applyMemoryDefaults restaura los defaults de los bool default-ON de MemoryConfig cuya clave
// puede faltar dentro de un bloque `memory` presente. La detección de bloque top-level de
// applyDefaults no alcanza: un bloque `memory` presente pero sin `recall_graph_centrality`
// deja el bool en su cero-valor (false), que es indistinguible de un opt-out explícito. Acá se
// mira la presencia de la SUB-CLAVE: ausente ⇒ default (ON); explícita (true/false) ⇒ se
// respeta. Si el bloque `memory` está ausente, applyDefaults ya puso el default completo.
// applyConflictsDefaults resuelve la sub-clave `cosine_floor` con el criterio ausente-vs-explícito:
// ausente ⇒ default (0.85); explícita ⇒ se respeta, INCLUIDO el 0 (que apaga el coseno y devuelve el
// dedup a su comportamiento léxico histórico). Con el `== 0 ⇒ default` de applyDefaults, un
// `cosine_floor: 0` quedaría pisado por el default y el rollback por config no funcionaría.
func (c *Config) applyConflictsDefaults(data []byte) {
	if !presentBlocks(data)["conflicts"] {
		return // bloque ausente ⇒ applyDefaults ya puso el default completo
	}
	keys := presentBlockKeys(data, "conflicts")
	if !keys["cosine_floor"] {
		c.Conflicts.CosineFloor = Default().Conflicts.CosineFloor
	}
	// band_floor: mismo criterio. Un `band_floor: 0` EXPLÍCITO apaga la banda ciega y debe
	// respetarse; con el `== 0 ⇒ default` de applyDefaults quedaría pisado y no habría rollback.
	if !keys["band_floor"] {
		c.Conflicts.BandFloor = Default().Conflicts.BandFloor
	}
}

func (c *Config) applyMemoryDefaults(data []byte) {
	if !presentBlocks(data)["memory"] {
		return
	}
	keys := presentBlockKeys(data, "memory")
	if !keys["recall_graph_centrality"] {
		c.Memory.RecallGraphCentrality = Default().Memory.RecallGraphCentrality
	}
	if !keys["recall_cooccurrence"] {
		c.Memory.RecallCooccurrence = Default().Memory.RecallCooccurrence
	}
	if !keys["recall_stemming"] {
		c.Memory.RecallStemming = Default().Memory.RecallStemming
	}
	if !keys["vector_floor"] {
		c.Memory.VectorFloor = Default().Memory.VectorFloor
	}
	// mmr_lambda: ausente ⇒ default; explícito ⇒ se respeta, INCLUIDO el 1 (que apaga la diversidad
	// y devuelve el ranking histórico). Con el `== 0 ⇒ default` de applyDefaults, un `mmr_lambda: 1`
	// no se distinguiría de "no lo puse" y el rollback por config no funcionaría.
	if !keys["mmr_lambda"] {
		c.Memory.MMRLambda = Default().Memory.MMRLambda
	}
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
		// La cuota de crecimiento archiva memoria (reversible, pero la saca del recall): tampoco
		// se enciende por un upgrade silencioso. Un config sin bloque `maintenance` queda con la
		// cuota DESACTIVADA; sólo se activa con el campo EXPLÍCITO en el yaml (lo escribe
		// `musubi init` con el default 50000, visible y editable).
		c.Maintenance.MaxActivePerProject = 0
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
		if c.Maintenance.DecayReinforcementK == 0 {
			c.Maintenance.DecayReinforcementK = d.Maintenance.DecayReinforcementK
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
		if c.Conflicts.CosineAutoThreshold == 0 {
			c.Conflicts.CosineAutoThreshold = d.Conflicts.CosineAutoThreshold
		}
		// El tope de cola se rellena igual que el resto de los numéricos. `enabled` NO: un false
		// —explícito o por omisión— es la respuesta correcta, porque la sombra nace apagada.
		if c.Conflicts.Shadow.Queue <= 0 {
			c.Conflicts.Shadow.Queue = d.Conflicts.Shadow.Queue
		}
		// CosineFloor NO se rellena acá: un 0 EXPLÍCITO es el interruptor de rollback (apaga el
		// coseno) y hay que respetarlo. Lo resuelve applyConflictsDefaults mirando la presencia de
		// la sub-clave (ausente ⇒ default; explícita, incluido 0 ⇒ se respeta).
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

	// Sync: ausente -> default completo (Enabled false); presente -> respetar los bool
	// (enabled/allow_insecure_token, incluido false) y las cadenas (central_url/auth_token_env,
	// que no tienen default útil) tal cual, rellenando sólo los numéricos en cero.
	if !present["sync"] {
		c.Sync = d.Sync
	} else {
		if c.Sync.DrainIntervalSeconds == 0 {
			c.Sync.DrainIntervalSeconds = d.Sync.DrainIntervalSeconds
		}
		if c.Sync.BatchSize == 0 {
			c.Sync.BatchSize = d.Sync.BatchSize
		}
		if c.Sync.MaxAttempts == 0 {
			c.Sync.MaxAttempts = d.Sync.MaxAttempts
		}
		if c.Sync.BackoffBaseSeconds == 0 {
			c.Sync.BackoffBaseSeconds = d.Sync.BackoffBaseSeconds
		}
		if c.Sync.BackoffMaxSeconds == 0 {
			c.Sync.BackoffMaxSeconds = d.Sync.BackoffMaxSeconds
		}
		if c.Sync.LeaseSeconds == 0 {
			c.Sync.LeaseSeconds = d.Sync.LeaseSeconds
		}
		if c.Sync.RequestTimeoutSeconds == 0 {
			c.Sync.RequestTimeoutSeconds = d.Sync.RequestTimeoutSeconds
		}
	}
}
