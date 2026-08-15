package memory

// StorageBackend es el contrato COMPLETO que un backend de memoria de Musubi debe
// cumplir para servir a la aplicación (el servidor MCP y la CLI). Es el seam de
// extensibilidad de Track 3: hoy *DbEngine (SQLite local-first, puro Go, model-free)
// es la implementación de referencia; un backend alternativo —por ejemplo el modo
// servicio de Track 4— implementa esta misma interfaz sin que los consumidores cambien.
//
// La interfaz se compone de interfaces de rol chicas (idioma Go: "interfaces chicas,
// compuestas"), de modo que un consumidor pueda depender solo del subconjunto que usa
// y los tests puedan falsear un único rol. El conjunto de métodos refleja exactamente
// lo que internal/mcp y cmd/musubi consumen — ni más (sin ceremonia) ni menos.
//
// Las firmas reflejan las de *DbEngine tal cual (incluido qué métodos toman context):
// esto es un seam, no una reescritura — no cambia ningún comportamiento.

import "context"

// ObservationStore — persistencia y búsqueda de observaciones (prosa + embeddings).
type ObservationStore interface {
	SaveObservation(id, topicKey, content string, embedding []float32) error
	SaveObservationWithImportance(id, topicKey, content string, importance float64, embedding []float32) error
	SaveObservationTyped(id, topicKey, content string, importance float64, memType, scope string, embedding []float32) error
	SaveObservationDeduped(topicKey, content string, importance float64, embedding []float32) (string, bool, error)
	SaveObservationDedupedTyped(topicKey, content string, importance float64, memType, scope string, embedding []float32) (string, bool, error)
	// Variantes *From: guardan con el project_id de ORIGEN explícito (atribución
	// multi-tenant, Track 16 F1). origin == "" ⇒ project_id del engine.
	SaveObservationTypedFrom(originProjectID, author, id, topicKey, content string, importance float64, memType, scope string, embedding []float32) error
	SaveObservationDedupedTypedFrom(originProjectID, author, topicKey, content string, importance float64, memType, scope string, embedding []float32) (string, bool, error)
	// Variantes *WithOrigins: además ATAN la observación a archivos del proyecto (ruta +
	// fingerprint al guardar). El recall re-deriva del disco y MARCA la observación si el
	// estado cambió — así una nota con una línea vencida deja de servirse como verdad
	// vigente sin que ningún agente tenga que notarlo. originPaths vacío ⇒ idéntico a las
	// variantes de arriba.
	SaveObservationTypedWithOrigins(originProjectID, author, id, topicKey, content string, importance float64, memType, scope string, originPaths []string, embedding []float32) error
	SaveObservationDedupedTypedFromWithOrigins(originProjectID, author, topicKey, content string, importance float64, memType, scope string, originPaths []string, embedding []float32) (string, bool, error)
	SearchObservations(ctx context.Context, queryEmbedding []float32, limit int) ([]SearchResult, error)
	SearchObservationsFTS(ctx context.Context, queryText string, limit int) ([]Observation, error)
	GetObservationsBudget(ids []string, budget int) ([]Observation, int, error)
	// GetObservationsBudgetCtx hidrata por id respetando el ctx (deadline + ProjectScope de
	// aislamiento multi-tenant, Track 17). El MCP la usa para acotar la expansión a la credencial.
	GetObservationsBudgetCtx(ctx context.Context, ids []string, budget int) ([]Observation, int, error)
	// HydrateForGroundingCtx es GetObservationsBudgetCtx SIN contabilizar el acceso. La usa el
	// grounding de musubi_ask, donde Recall ya bumpeó esos mismos ids: fundamentar una pregunta es
	// UN uso, y contarlo dos veces realimentaría el ranker con su propia salida (invariante N4).
	HydrateForGroundingCtx(ctx context.Context, ids []string, budget int) ([]Observation, int, error)
	// PromoteObservation marca una observación como 'shared' (memoria híbrida local+central).
	PromoteObservation(id string) error
	// PromoteObservationCtx es PromoteObservation acotada al proyecto de la credencial (aislamiento #11).
	PromoteObservationCtx(ctx context.Context, id string) error

	// --- Cuarentena de escritura y procedencia (Murallas 2+3 · F4) ---

	// ProposeObservation escribe una observación EN CUARENTENA con procedencia de modelo. NO
	// recibe procedencia ni bandera de cuarentena a propósito: las escribe ella, así el sello
	// es POR DÓNDE ENTRÓ y no lo que el caller haya dicho ser.
	ProposeObservation(originProjectID, author, topicKey, content, model string, confidence float64, memType string, embedding []float32) (string, error)
	// CorroborateObservationCtx saca una observación de cuarentena, acotada al proyecto de la
	// credencial. Es la ÚNICA salida, y CONSERVA el sello de procedencia.
	CorroborateObservationCtx(ctx context.Context, id string) error
	// IsQuarantined indica si una observación está en cuarentena.
	IsQuarantined(id string) (bool, error)
	// ObservationStamp devuelve el sello de una observación: procedencia, confianza y cuarentena.
	ObservationStamp(id string) (provenance string, confidence float64, quarantined bool, err error)
}

// RecallEngine — recall por presupuesto de tokens (model-free, híbrido FTS + ranking).
type RecallEngine interface {
	Recall(ctx context.Context, query string, opts RecallOptions) (RecallResult, error)
}

// GraphStore — grafo de conocimiento: hechos (tripletas) y contexto de entidad.
type GraphStore interface {
	SaveFact(subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error)
	// SaveFactFrom atribuye el hecho a originProjectID y acota la invalidación por cardinalidad
	// a ese proyecto (multi-tenant, Track 17); UNIQUE por (from_id, predicate, to_id, project_id).
	SaveFactFrom(originProjectID, subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error)
	// SaveFactFromSourced sella además la PROCEDENCIA (source) de la arista para poder auditar
	// y excluir hechos derivados por un LLM (pilar Cognición, F0). source vacío → "agent".
	SaveFactFromSourced(originProjectID, subject, predicate, object, validFrom, source string, singleValued []string) (SaveFactResult, error)
	// ResolveEntityName canonicaliza un nombre a una entidad existente suficientemente similar
	// (Jaccard de trigramas >= threshold), para que las propuestas LLM no fragmenten el grafo
	// (pilar Cognición, F4). threshold<=0 ⇒ no-op. Devuelve (canonical, matched).
	ResolveEntityName(name string, threshold float64) (string, bool, error)
	RecallFacts(entity string, maxHops, maxFacts int, asOf, rank string) (GraphResult, error)
	// RecallFactsCtx acota el traversal a las aristas visibles al proyecto del contexto (Track 17).
	RecallFactsCtx(ctx context.Context, entity string, maxHops, maxFacts int, asOf, rank string) (GraphResult, error)
	FactPath(from, to string, maxHops int, asOf string) (GraphResult, error)
	// FactPathCtx acota el camino a las aristas visibles al proyecto del contexto (Track 17).
	FactPathCtx(ctx context.Context, from, to string, maxHops int, asOf string) (GraphResult, error)
	EntityContext(entity string, maxHops, maxFacts, maxObs int) (EntityContextResult, error)
	// EntityContextCtx acota hechos y observaciones al proyecto del contexto (Track 17).
	EntityContextCtx(ctx context.Context, entity string, maxHops, maxFacts, maxObs int) (EntityContextResult, error)
}

// RelationStore — relaciones semánticas entre observaciones (resolución de conflictos).
type RelationStore interface {
	UpsertObsRelation(r ObsRelation) (string, error)
	PendingObsRelations() ([]ObsRelation, error)
	// PendingObsRelationsCtx acota al proyecto de la credencial (ctx, Track 17 — aislamiento).
	PendingObsRelationsCtx(ctx context.Context) ([]ObsRelation, error)
	// PendingObsRelationsQueryCtx es la misma lectura con filtros, orden y tope: existe porque la
	// cola llegó a las centenas y traerla entera para leer un contador cuesta 77 KB por consulta.
	PendingObsRelationsQueryCtx(ctx context.Context, q PendingQuery) (PendingPage, error)
	ResolveObsRelation(id, relation, resolvedBy, reason string) error
	// ResolveObsRelationCtx acota el veredicto al proyecto de la credencial (aislamiento #11).
	ResolveObsRelationCtx(ctx context.Context, id, relation, resolvedBy, reason string) error
}

// ConflictDetector — deducción model-free de relaciones al guardar una observación.
type ConflictDetector interface {
	DetectRelations(obsID string, opts ConflictOptions) ([]ObsRelation, error)
	// BandNeighbors — SOLO LECTURA: las memorias de la banda ciega [BandFloor, CosineFloor), donde
	// viven las CONTRADICCIONES. Se le MUESTRAN al agente; NO se encolan ni se persisten (band.go).
	BandNeighbors(obsID string, opts ConflictOptions) ([]BandNeighbor, int, error)
	// SaveShadowVerdict registra las DOS lecturas de un par (la model-free y la del motor) en el
	// libro de evidencia. La del motor se descarta: nada del camino de decisión lee esa tabla.
	SaveShadowVerdict(v ShadowVerdict) error
	// ShadowPairTexts trae los contenidos de las dos puntas SIN exigir visibilidad (el target de
	// un supersedes está supersedido por construcción). Lo usa el worker del modo sombra.
	ShadowPairTexts(srcID, tgtID string) (string, string, error)
	// ShadowAgreementByRelation resume el acuerdo entre el detector y el motor, por tipo.
	ShadowAgreementByRelation() ([]ShadowAgreement, error)
}

// CodeMemoryStore — memoria de código (gist + símbolos por archivo, para no re-leer).
type CodeMemoryStore interface {
	SaveCodeMemory(cm CodeMemory) error
	// SaveCodeMemoryFrom atribuye al project_id de origen (multi-tenant, Track 17); UPSERT por (path, project_id).
	SaveCodeMemoryFrom(originProjectID string, cm CodeMemory) error
	GetCodeMemory(path string) (CodeMemory, bool, error)
	// GetCodeMemoryCtx acota al proyecto de la credencial (ctx, Track 17 — aislamiento).
	GetCodeMemoryCtx(ctx context.Context, path string) (CodeMemory, bool, error)
	// AllCodeMemoryCtx / ReplaceProjectCodeMemoryFrom son las dos mitades de la federación de
	// gists: la primera los vuelca para empujarlos con el grafo, la segunda los recibe en el
	// central. Espejan a AllGraphNodesCtx / ReplaceProjectGraphFrom del CodeGraphStore, que ya
	// existían — los gists eran la mitad que faltaba, y por eso el central tenía miles de nodos
	// y cero titulares.
	AllCodeMemoryCtx(ctx context.Context) ([]CodeMemory, error)
	ReplaceProjectCodeMemoryFrom(originProjectID string, gists []CodeMemory) error
}

// CodeGraphStore — grafo de código derivado del AST (Track 20 · F1): nodos + aristas tipadas,
// scopeados por project_id, con invalidación por src_fingerprint. Distinto de GraphStore (que
// es el grafo de HECHOS/memoria). Las tools de consulta y el hook que responde son F2.
type CodeGraphStore interface {
	// UpsertPackageGraphFrom persiste el grafo de un paquete atribuido al project_id de origen,
	// borrando por src_path (delete-by-source) y reinsertando con el fingerprint del snapshot.
	UpsertPackageGraphFrom(originProjectID string, files []string, nodes []GraphNode, edges []GraphEdge) error
	// GetGraphNodeCtx lee un nodo acotado al proyecto de la credencial (prefiere el propio sobre '').
	GetGraphNodeCtx(ctx context.Context, nodeKey string) (GraphNode, bool, error)
	// GraphOutEdgesCtx devuelve las aristas salientes de un nodo, scopeadas al proyecto.
	GraphOutEdgesCtx(ctx context.Context, fromKey string) ([]GraphEdge, error)
	// GraphInEdgesCtx devuelve las aristas entrantes a un nodo (sus callers), scopeadas (F2).
	GraphInEdgesCtx(ctx context.Context, toKey string) ([]GraphEdge, error)
	// GraphImpactCtx devuelve el cierre transitivo de callers (BFS acotado), scopeado (F2).
	GraphImpactCtx(ctx context.Context, key string, maxDepth, maxNodes int) ([]string, error)
	// GraphStatsCtx devuelve conteo de nodos y aristas por kind, scopeado (F2).
	GraphStatsCtx(ctx context.Context) (int, map[string]int, error)
	// GraphTopByDegreeCtx devuelve los N nodos con mayor grado CALLS (god-nodes), scopeado (F2).
	GraphTopByDegreeCtx(ctx context.Context, n int) ([]GraphDegree, error)
	// GraphEntryPointsCtx devuelve funcs/métodos sin callers internos (entry points), scopeado (F2).
	GraphEntryPointsCtx(ctx context.Context, limit int) ([]string, error)
	// ListGraphNodesForFileCtx devuelve los símbolos de un archivo, scopeado (F2).
	ListGraphNodesForFileCtx(ctx context.Context, path string) ([]GraphNode, error)
	// ListGraphFuncsInDirsCtx devuelve las funcs top-level de un conjunto de directorios (no
	// recursivo). Es lo que permite resolver una llamada cross-paquete en el refresco incremental:
	// de un import path se conoce el DIRECTORIO, nunca el archivo donde vive el símbolo (F8-A).
	ListGraphFuncsInDirsCtx(ctx context.Context, dirs []string) ([]GraphNode, error)
	// GraphFileFingerprintsCtx devuelve path → src_fingerprint de los archivos del grafo, scopeado
	// (base de la reconciliación incremental y de la frescura, F5).
	GraphFileFingerprintsCtx(ctx context.Context) (map[string]string, error)
	// PruneGraphFilesFrom borra nodos/aristas de los archivos dados (fantasmas), scopeado por
	// project_id de origen (F5).
	PruneGraphFilesFrom(originProjectID string, paths []string) (int, error)
	// ReplaceProjectGraphFrom reemplaza el grafo COMPLETO de un proyecto (recepción de la
	// federación push-on-index, F6): borra todo lo del origin_project_id y reinserta el set empujado.
	ReplaceProjectGraphFrom(originProjectID string, nodes []GraphNode, edges []GraphEdge) error
	// AllGraphNodesCtx / AllGraphEdgesCtx vuelcan el grafo completo del proyecto (scopeado por la
	// credencial) para serializarlo en el push-on-index de la federación (F6).
	AllGraphNodesCtx(ctx context.Context) ([]GraphNode, error)
	AllGraphEdgesCtx(ctx context.Context) ([]GraphEdge, error)
}

// MetaStore — almacén clave/valor + gates de throttling por intervalo.
type MetaStore interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	MetaDue(key string, intervalHours float64) (bool, error)
	MarkMetaNow(key string) error
	MaintenanceDue(intervalHours float64) (bool, error)
	MarkMaintenanceNow() error
}

// TelemetryStore — logs de errores de compilación/test para el bucle de telemetría.
type TelemetryStore interface {
	SaveTelemetryLog(filePath, errorMessage, suggestedPatch string) error
	// SaveTelemetryLogFrom atribuye el log al project_id de origen (multi-tenant, Track 18).
	SaveTelemetryLogFrom(originProjectID, filePath, errorMessage, suggestedPatch string) error
	GetUnresolvedTelemetryLogs() ([]TelemetryLog, error)
	GetUnresolvedTelemetryLogsForFiles(files []string) ([]TelemetryLog, error)
	// Variantes ctx-aware acotadas al proyecto de la credencial (Track 19): resolve_skills exponía
	// telemetría cross-tenant por colisión de basename.
	GetUnresolvedTelemetryLogsForFilesCtx(ctx context.Context, files []string) ([]TelemetryLog, error)
	ResolveTelemetryLog(id int) error
	// ResolveTelemetryLogAndGet resuelve el log y devuelve su contenido (para capturar el par
	// error→fix como memoria, C4). found=false si el id no existe.
	ResolveTelemetryLogAndGet(id int) (TelemetryLog, bool, error)
	// ResolveTelemetryLogAndGetCtx acota la resolución/lectura al proyecto de la credencial
	// (ctx, Track 18): un tenant no resuelve ni lee el log crudo de otro. found=false si no visible.
	ResolveTelemetryLogAndGetCtx(ctx context.Context, id int) (TelemetryLog, bool, error)
}

// SkillDecisionStore — log persistente de decisiones de skills (aceptada/rechazada).
type SkillDecisionStore interface {
	SaveSkillDecision(skillID, name, decision, reason string) error
	// SaveSkillDecisionFrom atribuye la decisión al project_id de origen (multi-tenant, Track 18).
	SaveSkillDecisionFrom(originProjectID, skillID, name, decision, reason string) error
	GetSkillDecisions() ([]SkillDecision, error)
	// GetSkillDecisionsCtx acota al proyecto de la credencial (ctx, Track 18 — aislamiento).
	GetSkillDecisionsCtx(ctx context.Context) ([]SkillDecision, error)
}

// WorkStore — pizarra compartida de unidades de trabajo (orquestación multi-agente).
type WorkStore interface {
	CreateWorkBatch(batchID string, specs []WorkUnitSpec) (WorkBatch, error)
	ClaimWorkUnit(batchID, agent string, ttlSeconds, maxAttempts int) (WorkUnit, bool, error)
	HeartbeatWorkUnit(id, owner string, fencingToken int64, ttlSeconds int) (bool, error)
	CompleteWorkUnit(id, result, status, agent string, fencingToken int64) error
	// CompleteWorkUnitConEfecto cierra declarando qué hizo el agente (report|apply); el efecto
	// se contrasta con la autonomía declarada de la unidad (L1/L2/L3).
	CompleteWorkUnitConEfecto(id, result, status, agent string, fencingToken int64, effect string) error
	// ApproveWorkUnit firma el intento en curso de una unidad L2 (maker/checker).
	ApproveWorkUnit(id, reviewer string) error
	WorkBatchStatus(batchID string) (WorkBatch, error)
	ActiveBatch() (WorkBatch, bool, error)
	ClearWorkBatch(batchID string) error
	BidWorkUnit(unitID, agent string, bid float64, note string) error
	AwardWorkUnit(unitID string, ttlSeconds int) (WorkUnit, WorkBid, bool, error)
	WorkUnitBids(unitID string) ([]WorkBid, error)
}

// DebateStore — subsistema de debate multi-agente (Society of Minds) model-free: rondas de
// posturas atribuidas + tally determinista por mayoría/quórum.
type DebateStore interface {
	OpenDebate(topic string, rounds, quorum int) (Debate, error)
	PostPosture(debateID, agent, stance string) error
	AdvanceDebate(debateID string) (int, []DebatePosture, error)
	CastVote(debateID, agent, choice string) error
	TallyDebate(debateID string) (TallyResult, Debate, error)
	DebateStatus(debateID string) (Debate, []DebatePosture, []DebateVote, error)
}

// WorkflowStore — motor de orquestación DAG persistente (resumible entre sesiones).
type WorkflowStore interface {
	StartWorkflowRun(runID string, def WorkflowDef) (WorkflowRun, error)
	WorkflowRunStatus(runID string) (WorkflowRun, bool, error)
	WorkflowReady(runID string) ([]string, error)
	CompleteWorkflowStep(runID, stepID, result, stepStatus, idempotencyKey string) (WorkflowRun, error)
	WorkflowJournal(runID string) ([]RunEvent, error)
	WorkflowTraceOTLP(runID string) (string, error)
	WorkflowRollback(runID string) ([]CompensationStep, WorkflowRun, error)
	AbortWorkflowRun(runID, reason string) (WorkflowRun, error)
	CompleteCompensation(runID, stepID string) ([]CompensationStep, WorkflowRun, error)
	ProvideWorkflowInput(runID, stepID, input, status string) (WorkflowRun, error)
	WorkflowAwaiting(runID string) ([]AwaitingStep, error)
	VerifyWorkflowStep(runID, stepID string, pass bool, reflection, targetDigest string) (WorkflowRun, []string, error)
	WorkflowListRuns() ([]WorkflowRunSummary, error)
}

// LedgerStore — ledger de tokens de la sesión (gasto por superficie).
type LedgerStore interface {
	LedgerStatus() (TokenLedger, error)
	LedgerAdd(sessionID, surface string, tokens int) (TokenLedger, error)
	LedgerReset() error
}

// PhaseStore — pipeline por fases del loop dirigido (explore→plan→code→verify).
type PhaseStore interface {
	PhaseStatus() (PhaseState, bool, error)
	StartPhase(task string, phases []string) (PhaseState, error)
	AdvancePhase(phases []string) (PhaseState, bool, error)
	SetPhase(phase string, phases []string) (PhaseState, error)
	ClearPhase() error
}

// Maintainer — ciclo de mantenimiento (consolidar → olvidar → purgar → compactar).
type Maintainer interface {
	Maintain(opts MaintenanceOptions) (MaintenanceReport, error)
}

// Doctor — diagnóstico y reparación de la base de memoria.
type Doctor interface {
	Diagnose() (DiagnoseReport, error)
	DiagnoseQuick() (DiagnoseReport, error)
	RunCheck(code string) (CheckResult, error)
	Repair(code, mode string) (RepairResult, error)
	AutoHeal() (DiagnoseReport, error)
}

// Calibrator — calibración (opt-in) del estimador de tokens.
type Calibrator interface {
	SampleContents(limit int) ([]string, error)
	SaveDivisors(prose, code, jsn float64) error
	RecomputeTokens() error
}

// Insighter — resumen agregado de observabilidad activa (estado de la memoria).
type Insighter interface {
	Insights() (InsightsReport, error)
	// InsightsCtx acota los counts de observations al proyecto del contexto (Track 17, parcial).
	InsightsCtx(ctx context.Context) (InsightsReport, error)
}

// OutboxStore — outbox durable del sync SALIENTE del cerebro híbrido (F2): encolado
// transaccional de las observaciones 'shared' + claim/lease/backoff/dead-letter del drain
// offline-first. enqueueOutboxTx es interno (corre dentro de la tx del save/promote), así que
// no forma parte del contrato público; acá van los métodos que consume el drain (internal/mcp).
type OutboxStore interface {
	BackfillOutbox() (int, error)
	ClaimOutboxBatch(limit, leaseSeconds int) ([]OutboxItem, error)
	MarkOutboxSent(obsID string) error
	MarkOutboxRetry(obsID string, backoffSeconds int, errMsg string) error
	MarkOutboxDead(obsID, errMsg string) error
	OutboxStats() (pending, sent, dead int, err error)
	// ListSharedForPull sirve el sync ENTRANTE (C5.3): lista la memoria 'shared' del proyecto del
	// ctx (aislamiento T17-19) con rowid > afterRowID, paginada. La corre el central en un pull.
	ListSharedForPull(ctx context.Context, afterRowID int64, limit int) ([]SharedObs, error)
	// IngestShared persiste una obs 'shared' bajada del central (sync ENTRANTE C5.3) SIN encolarla
	// en el outbox local (anti-loop). UPSERT idempotente por id. Devuelve si insertó una fila nueva.
	IngestShared(o SharedObs) (inserted bool, err error)
	OutboxHealth() (OutboxHealthReport, error)
	RequeueDeadOutbox() (int, error)
}

// StorageBackend es la unión de todos los roles: el contrato que un backend completo
// debe satisfacer. Embebe io.Closer-equivalente vía Close.
type StorageBackend interface {
	ObservationStore
	RecallEngine
	GraphStore
	RelationStore
	ConflictDetector
	CodeMemoryStore
	CodeGraphStore
	MetaStore
	TelemetryStore
	SkillDecisionStore
	WorkStore
	DebateStore
	WorkflowStore
	LedgerStore
	PhaseStore
	Maintainer
	Doctor
	Calibrator
	Insighter
	OutboxStore

	// Close libera los recursos del backend (espera trabajo en background y cierra
	// la conexión subyacente).
	Close() error
}

// Aserción en tiempo de compilación: *DbEngine (el backend SQLite de referencia)
// satisface el contrato completo. Si se agrega un método al contrato que DbEngine no
// implementa —o cambia una firma— esto rompe la compilación de inmediato.
var _ StorageBackend = (*DbEngine)(nil)
