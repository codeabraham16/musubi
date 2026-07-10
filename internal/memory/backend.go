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
	SaveObservationTypedFrom(originProjectID, id, topicKey, content string, importance float64, memType, scope string, embedding []float32) error
	SaveObservationDedupedTypedFrom(originProjectID, topicKey, content string, importance float64, memType, scope string, embedding []float32) (string, bool, error)
	SearchObservations(ctx context.Context, queryEmbedding []float32, limit int) ([]SearchResult, error)
	SearchObservationsFTS(ctx context.Context, queryText string, limit int) ([]Observation, error)
	GetObservationsBudget(ids []string, budget int) ([]Observation, int, error)
	// GetObservationsBudgetCtx hidrata por id respetando el ctx (deadline + ProjectScope de
	// aislamiento multi-tenant, Track 17). El MCP la usa para acotar la expansión a la credencial.
	GetObservationsBudgetCtx(ctx context.Context, ids []string, budget int) ([]Observation, int, error)
	// PromoteObservation marca una observación como 'shared' (memoria híbrida local+central).
	PromoteObservation(id string) error
}

// RecallEngine — recall por presupuesto de tokens (model-free, híbrido FTS + ranking).
type RecallEngine interface {
	Recall(ctx context.Context, query string, opts RecallOptions) (RecallResult, error)
}

// GraphStore — grafo de conocimiento: hechos (tripletas) y contexto de entidad.
type GraphStore interface {
	SaveFact(subject, predicate, object, validFrom string, singleValued []string) (SaveFactResult, error)
	RecallFacts(entity string, maxHops, maxFacts int, asOf, rank string) (GraphResult, error)
	FactPath(from, to string, maxHops int, asOf string) (GraphResult, error)
	EntityContext(entity string, maxHops, maxFacts, maxObs int) (EntityContextResult, error)
}

// RelationStore — relaciones semánticas entre observaciones (resolución de conflictos).
type RelationStore interface {
	UpsertObsRelation(r ObsRelation) (string, error)
	PendingObsRelations() ([]ObsRelation, error)
	// PendingObsRelationsCtx acota al proyecto de la credencial (ctx, Track 17 — aislamiento).
	PendingObsRelationsCtx(ctx context.Context) ([]ObsRelation, error)
	ResolveObsRelation(id, relation, resolvedBy, reason string) error
}

// ConflictDetector — deducción model-free de relaciones al guardar una observación.
type ConflictDetector interface {
	DetectRelations(obsID string, opts ConflictOptions) ([]ObsRelation, error)
}

// CodeMemoryStore — memoria de código (gist + símbolos por archivo, para no re-leer).
type CodeMemoryStore interface {
	SaveCodeMemory(cm CodeMemory) error
	// SaveCodeMemoryFrom atribuye al project_id de origen (multi-tenant, Track 17); UPSERT por (path, project_id).
	SaveCodeMemoryFrom(originProjectID string, cm CodeMemory) error
	GetCodeMemory(path string) (CodeMemory, bool, error)
	// GetCodeMemoryCtx acota al proyecto de la credencial (ctx, Track 17 — aislamiento).
	GetCodeMemoryCtx(ctx context.Context, path string) (CodeMemory, bool, error)
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
	GetUnresolvedTelemetryLogs() ([]TelemetryLog, error)
	GetUnresolvedTelemetryLogsForFiles(files []string) ([]TelemetryLog, error)
	ResolveTelemetryLog(id int) error
	// ResolveTelemetryLogAndGet resuelve el log y devuelve su contenido (para capturar el par
	// error→fix como memoria, C4). found=false si el id no existe.
	ResolveTelemetryLogAndGet(id int) (TelemetryLog, bool, error)
}

// SkillDecisionStore — log persistente de decisiones de skills (aceptada/rechazada).
type SkillDecisionStore interface {
	SaveSkillDecision(skillID, name, decision, reason string) error
	GetSkillDecisions() ([]SkillDecision, error)
}

// WorkStore — pizarra compartida de unidades de trabajo (orquestación multi-agente).
type WorkStore interface {
	CreateWorkBatch(batchID string, specs []WorkUnitSpec) (WorkBatch, error)
	ClaimWorkUnit(batchID, agent string, ttlSeconds, maxAttempts int) (WorkUnit, bool, error)
	HeartbeatWorkUnit(id, owner string, fencingToken int64, ttlSeconds int) (bool, error)
	CompleteWorkUnit(id, result, status, agent string, fencingToken int64) error
	WorkBatchStatus(batchID string) (WorkBatch, error)
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
	VerifyWorkflowStep(runID, stepID string, pass bool, reflection string) (WorkflowRun, []string, error)
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
