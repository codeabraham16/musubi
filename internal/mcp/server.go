// Package mcp implementa el servidor MCP (Model Context Protocol) de Musubi:
// un loop JSON-RPC 2.0 sobre stdin/stdout que expone las herramientas de memoria,
// orquestación y skills. Coordina y persiste; el agente ejecuta.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"musubi/internal/codeintel"
	"musubi/internal/cognition"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/logx"
	"musubi/internal/memory"
	"musubi/internal/skills"
)

// Códigos de error JSON-RPC 2.0 estándar usados por el servidor.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	// codeUnauthorized (rango server-error de JSON-RPC) = el principal autenticado no
	// tiene permiso para invocar la tool (autorización por rol, Track 16 F1 16.1c).
	codeUnauthorized = -32001
	// codeQuotaExceeded (rango server-error) = el principal superó su cuota de llamadas por
	// ventana (Track 16 F3.2). La credencial es VÁLIDA; excedió el límite de uso. Reintentar
	// tras la ventana.
	codeQuotaExceeded = -32002
	// codeMotorQuota (rango server-error) = el principal agotó su PRESUPUESTO DEL MOTOR de
	// cognición. Código propio y no codeQuotaExceeded porque el remedio es distinto: la cuota
	// general se libera en segundos y ésta en la hora, y quien la recibe puede querer seguir con
	// las tools model-free en vez de esperar.
	codeMotorQuota = -32003
)

type JsonRpcRequest struct {
	JsonRpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JsonRpcResponse struct {
	JsonRpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RpcError   `json:"error,omitempty"`
}

type RpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func rpcErrorf(code int, format string, args ...interface{}) *RpcError {
	return &RpcError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Option es una función de configuración funcional para McpServer.
// Se usa en NewMcpServer para configuración aditiva sin romper callers existentes.
type Option func(*McpServer)

// WithSpoolLocal enciende el vertedero del feed en `dir`, para que un panel de ESTA máquina
// pueda ver lo que hace un daemon stdio. Se usa en `musubi daemon`, no en `musubi serve`:
// el central ya reparte por HTTP y escribir además a disco serían ~100.000 líneas diarias
// para nadie.
func WithSpoolLocal(dir string) Option {
	return func(s *McpServer) {
		s.spool = nuevoSpool(dir, os.Getpid(), spoolTope)
		s.origenFeed = "local"
	}
}

// CloseSpool borra el archivo de este proceso. Va en un defer del daemon: la salida limpia no
// tiene que dejarle basura al lector, aunque el lector igual pode lo que quede de una muerte
// de golpe.
func (s *McpServer) CloseSpool() { s.spool.cerrar() }

// WithSourcing devuelve un Option que configura el campo sourcing del servidor.
func WithSourcing(c config.SourcingConfig) Option {
	return func(s *McpServer) { s.sourcing = c }
}

// WithMemory devuelve un Option que configura los parámetros del recall eficiente.
func WithMemory(c config.MemoryConfig) Option {
	return func(s *McpServer) { s.memory = c }
}

// WithMaintenance devuelve un Option que configura el auto-mantenimiento.
func WithMaintenance(c config.MaintenanceConfig) Option {
	return func(s *McpServer) { s.maintenance = c }
}

// WithGraph devuelve un Option que configura la memoria en grafo.
func WithGraph(c config.GraphConfig) Option {
	return func(s *McpServer) { s.graph = c }
}

// WithConflicts devuelve un Option que configura la detección de relaciones
// semánticas entre observaciones (resolución de conflictos model-free).
func WithConflicts(c config.ConflictConfig) Option {
	return func(s *McpServer) { s.conflicts = c }
}

// WithPipeline devuelve un Option que configura el pipeline por fases del loop
// dirigido (musubi_phase + recordatorio de fase por turno).
func WithPipeline(c config.PipelineConfig) Option {
	return func(s *McpServer) { s.pipeline = c }
}

// WithMultiAgent devuelve un Option que configura la pizarra compartida del
// multi-agente (musubi_work).
func WithMultiAgent(c config.MultiAgentConfig) Option {
	return func(s *McpServer) { s.multiagent = c }
}

type McpServer struct {
	engine   memory.StorageBackend
	resolver *skills.Resolver
	embedder embedding.Provider
	// cognition es el motor del 3er pilar (Cognición LLM); NoopProvider ⇒ pilar apagado (default).
	cognition cognition.Provider
	// cognitionCfg trae las guardas de CALIDAD del 3er pilar (F3): el vocabulario controlado de
	// predicados para propuestas (AllowedPredicates) y el TTL de cuarentena (ProposalTTLHours).
	// Zero-value ⇒ enum allow-all + sin barrido ⇒ comportamiento bit-idéntico.
	cognitionCfg config.CognitionConfig
	// ledger es el LEDGER DE USO (F0): amortigua las invocaciones y las baja por lote. nil ⇒
	// no se registra nada y el servidor se comporta como antes de la fase.
	ledger *usageLedger
	// live es el FEED EN VIVO (livefeed.go): reparte cada invocacion a los paneles conectados
	// en el instante en que termina. Va SIEMPRE construido y no detras de una option: sin
	// suscriptores publish() es tomar un mutex y escribir una posicion de un array, y una
	// telemetria que hay que acordarse de encender termina apagada (la misma leccion que dejo
	// el ledger de uso, cuyos contadores en memoria murieron dos meses sin que nadie lo notara).
	live *liveFeed
	// ejesVec cachea los 19 vectores de la taxonomía de diseño (ejes_diseno.go). Son constantes del
	// binario: recalcularlos por pedido serían 19 llamadas al embebedor para obtener siempre lo
	// mismo, contra un embebedor que se comparte con recall y save. nil ⇒ todavía no se calcularon
	// o no hay embebedor, y el motor de diseño cae al camino por similitud.
	ejesVec map[string][]float32
	ejesMu  sync.Mutex

	// spool saca el feed a disco para los daemons que NO sirven HTTP. nil ⇒ apagado, que es
	// lo correcto en el central: ahí ya hay suscriptores por HTTP y escribir además a disco
	// serían ~100.000 líneas diarias para nadie. Ver spool.go.
	spool *spoolLocal
	// origenFeed viaja en cada evento. Ver LiveEvent.Origen.
	origenFeed string
	// ledgerRetentionDays es la ventana que conserva el ledger; la purga cuelga del
	// mantenimiento que ya existe.
	ledgerRetentionDays int
	// projectPath es la raíz del proyecto (== MUSUBI_HOME).
	// La usan los handlers de detect_stack y save_skill para resolver rutas.
	projectPath string
	// sourcing contiene la configuración de sourcing de skills desde catálogo remoto.
	sourcing config.SourcingConfig
	// sourceCache cachea las respuestas de red del sourcing (catálogo y marketplace) con
	// TTL = sourcing.CacheSeconds, para no repetir el mismo GET en ventanas cortas.
	sourceCache *sourcingCache
	// memory contiene los parámetros del recall por presupuesto de tokens.
	memory config.MemoryConfig
	// maintenance contiene los parámetros del auto-mantenimiento (consolidar + olvidar).
	maintenance config.MaintenanceConfig
	// graph contiene los parámetros de la memoria estructurada en grafo.
	graph config.GraphConfig
	// conflicts contiene los parámetros de la detección de relaciones semánticas.
	conflicts config.ConflictConfig
	// shadow es el worker del MODO SOMBRA, o nil cuando está apagado (que es el default). nil no
	// es un caso especial que haya que recordar: encolarSombra chequea por nil y no hace nada, así
	// que el camino de guardado es idéntico con o sin sombra.
	shadow *shadowWorker
	// pipeline contiene los parámetros del pipeline por fases del loop dirigido.
	pipeline config.PipelineConfig
	// multiagent contiene los parámetros de la pizarra compartida del multi-agente.
	multiagent config.MultiAgentConfig
	// forceRedact fuerza la redacción de secretos en TODO ingest (Track 16 F1 16.1d): el
	// central es infra compartida, así que redacta independientemente del scope declarado por
	// el cliente (cierra el hueco scope=local → secreto crudo). Lo enciende ListenAndServeHTTP
	// cuando el bind es no-loopback (o service.force_redact). En stdio local queda false.
	forceRedact bool
	// gitRunner obtiene el diff para musubi_detect_changes; nil → GitRunner real sobre
	// projectPath. Los tests lo inyectan (codeintel.FakeRunner) para no depender de git.
	gitRunner codeintel.Runner
	// tools es el catálogo ordenado de tools (fuente de tools/list); toolIndex es
	// el mapa nombre→handler para el dispatch O(1) de tools/call. Ambos se construyen
	// una vez en NewMcpServer desde buildRegistry.
	tools     []toolEntry
	toolIndex map[string]toolHandler
	// toolReadOnly[name]=true si la tool no muta estado. Decide la AUTORIZACIÓN (un reader
	// sólo puede llamar tools de lectura) y es el DEFAULT del candado de despacho.
	toolReadOnly map[string]bool
	// toolLock[name] pisa ese default SÓLO para la concurrencia. Ausente ⇒ lockFromReadOnly,
	// el comportamiento histórico. Ver lockClass en registry.go.
	toolLock map[string]lockClass
	// dispatchMu hace seguro el dispatch concurrente (transporte HTTP): las tools que
	// mutan toman Lock (serializadas, RMW-safe); las de solo-lectura toman RLock
	// (concurrentes entre sí). En stdio (un goroutine) está siempre libre, costo nulo.
	//
	// NINGÚN CANDADO DE ACÁ PUEDE CRUZAR UNA LLAMADA DE RED. Un handler que hace I/O externa
	// (motor LLM, embedder) se declara lockSelf y acota su sección crítica con withReadLock:
	// con el candado tomado, una llamada de 120 s deja al servidor entero sin atender.
	dispatchMu sync.RWMutex
	// saveCount cuenta saves desde el último disparo; al cruzar maintenance.AutoAfterSaves
	// dispara un mantenimiento async (T5.3). maintBusy garantiza un solo ciclo en vuelo.
	saveCount atomic.Int64
	maintBusy atomic.Bool
	// syncClient empuja las filas del outbox al cerebro central (F2); nil ⇒ sync desactivado
	// (el drain no arranca). syncCfg trae los parámetros del drain (batch/lease/backoff/tope).
	// Ambos los fija el entrypoint (SetSyncClient) cuando sync.enabled && central_url != "".
	syncClient *SyncClient
	syncCfg    config.SyncConfig
	// metrics son los contadores/histogramas expuestos en /metrics (Track 16 F3.1). Compartido
	// por ambos transportes: el middleware HTTP registra el resultado de cada request y
	// handleToolsCall registra la latencia/resultado de cada tools/call. Lock-free (atomic).
	metrics *serverMetrics
	// quota limita las llamadas por-principal por ventana (Track 16 F3.2); nil ⇒ sin cuota.
	// Solo aplica cuando hay un principal autenticado (serve); en stdio local no hay cuota.
	quota *quotaLimiter
	// motorQuota es el FRENO DE GASTO del motor de cognición: cuenta, por principal y por hora, las
	// llamadas que EFECTIVAMENTE llegan al modelo. Es un limitador aparte de `quota` y no un ajuste
	// suyo porque miden cosas distintas: `quota` protege al daemon de un cliente desbocado (600/min,
	// calibrado para tools gratis) y éste protege la SUSCRIPCIÓN. nil ⇒ sin freno.
	motorQuota *quotaLimiter
	// principalsFile es la ruta del registro de identidades que el server usa para autenticar.
	// La fija ListenAndServeHTTP (serve/HTTP); las tools admin (musubi_token_*) la mutan para dar
	// de alta/baja miembros por la red, sin SSH ni CLI. Vacía en stdio local/tests ⇒ default.
	principalsFile string
}

// WithQuota devuelve un Option que activa la cuota de uso por-principal: máximo perMinute
// llamadas a tools/call por principal por minuto. perMinute<=0 ⇒ sin cuota (default).
func WithQuota(perMinute int) Option {
	return func(s *McpServer) { s.quota = newQuotaLimiter(perMinute, time.Minute) }
}

// WithMotorQuota activa el freno de gasto del motor: máximo perHour llamadas AL MODELO por
// principal por hora. perHour<=0 ⇒ sin freno.
//
// La ventana es de una HORA y no de un minuto porque el gasto no se acumula igual que las llamadas
// baratas: nadie hace 60 preguntas razonadas en un minuto, pero un bucle sí — y contra un bucle lo
// que importa es el techo de la hora, no la ráfaga del segundo.
func WithMotorQuota(perHour int) Option {
	return func(s *McpServer) { s.motorQuota = newQuotaLimiter(perHour, time.Hour) }
}

// WithCognition inyecta el motor del 3er pilar (Cognición). Aditivo; c==nil se ignora (queda el
// NoopProvider por default ⇒ pilar apagado). F0 sólo cablea el enchufe: ninguna ruta lo invoca aún.
func WithCognition(c cognition.Provider) Option {
	return func(s *McpServer) {
		if c != nil {
			s.cognition = c
		}
	}
}

// WithCognitionConfig inyecta las guardas de calidad del 3er pilar (F3): enum de predicados para
// propuestas + TTL de cuarentena. Aditivo; zero-value ⇒ allow-all + sin barrido (bit-idéntico).
func WithCognitionConfig(c config.CognitionConfig) Option {
	return func(s *McpServer) { s.cognitionCfg = c }
}

// WithUsageLedger enciende el LEDGER DE USO (F0 · track «Potencia medida»): persiste una fila por
// invocación de tool para poder responder cuáles se usan de verdad. Sin esta option el servidor no
// registra nada y se comporta exactamente como antes.
//
// El sink es el motor de memoria. Se pasa como interfaz para que un test pueda inyectar uno que
// falla y verificar que un ledger roto NO tumba una tool (invariante L2).
func WithUsageLedger(sink ledgerSink, cfg config.UsageLedgerConfig) Option {
	return func(s *McpServer) {
		if sink == nil || !cfg.EnabledOn() {
			return
		}
		s.ledger = newUsageLedger(sink, time.Duration(cfg.EffectiveFlushSeconds())*time.Second)
		s.ledgerRetentionDays = cfg.EffectiveRetentionDays()
		s.ledger.start()
	}
}

// CloseLedger baja lo que quede en el buffer y detiene la goroutine de flush. Lo llama el
// entrypoint al terminar; es idempotente y seguro sobre un servidor sin ledger.
func (s *McpServer) CloseLedger() { s.ledger.close() }

// SetSyncClient inyecta el cliente de sync saliente y su config en el servidor, habilitando
// el drain del outbox (RunOutboxScheduler). Lo llama el entrypoint (serve/daemon) tras
// construir el SyncClient desde cfg.Sync. Sin llamarlo, el server no sincroniza (syncClient nil).
func (s *McpServer) SetSyncClient(client *SyncClient, cfg config.SyncConfig) {
	s.syncClient = client
	s.syncCfg = cfg
}

// defaultScope es el scope que recibe una captura SIN scope explícito (C5.2): 'shared' cuando el
// proyecto está en team mode (la memoria fluye al cerebro central vía outbox), o 'local' si no
// (comportamiento histórico). Un scope explícito del cliente siempre se respeta (no pasa por acá).
func (s *McpServer) defaultScope() string {
	if s.memory.TeamMode {
		return memory.ScopeShared
	}
	return memory.ScopeLocal
}

// NewMcpServer construye el servidor MCP. embedder genera embeddings a partir de
// texto; usá embedding.NoopProvider{} para desactivar la búsqueda semántica.
// opts son opciones funcionales aditivas (ej. WithSourcing); los callers existentes
// de 3 argumentos compilan sin cambios.
func NewMcpServer(engine memory.StorageBackend, projectPath string, embedder embedding.Provider, opts ...Option) *McpServer {
	if embedder == nil {
		embedder = embedding.NoopProvider{}
	}
	s := &McpServer{
		engine:      engine,
		resolver:    skills.NewResolver(projectPath),
		embedder:    embedder,
		cognition:   cognition.NoopProvider{},
		projectPath: projectPath,
		sourcing:    config.Default().Sourcing,
		memory:      config.Default().Memory,
		maintenance: config.Default().Maintenance,
		graph:       config.Default().Graph,
		conflicts:   config.Default().Conflicts,
		pipeline:    config.Default().Pipeline,
		multiagent:  config.Default().MultiAgent,
		metrics:     &serverMetrics{},
		live:        newLiveFeed(),
	}
	for _, opt := range opts {
		opt(s)
	}
	// El caché de sourcing se crea tras aplicar las options: su TTL sale de la config
	// (WithSourcing) que recién quedó fijada arriba.
	s.sourceCache = newSourcingCache(s.sourcing.CacheSeconds)
	// El worker de sombra se CONSTRUYE acá aunque su bucle arranque después (RunShadowWorker):
	// crearlo en el arranque del bucle dejaría que un save temprano leyera s.shadow mientras otra
	// goroutine lo escribe. Construirlo es sólo un canal; el gasto está en el bucle, no acá.
	// Se exige motor real: sin cognición no hay segunda lectura que comparar, y encolar trabajo
	// para un motor apagado sería llenar una cola que nadie vacía.
	if s.conflicts.Shadow.Enabled && cognition.Enabled(s.cognition) {
		s.shadow = newShadowWorker(engine, s.cognition, s.conflicts.Shadow.Queue)
	}
	// Construir el registro de tools una vez (los handlers leen la config de s en
	// tiempo de llamada, así que el orden respecto de las opciones no importa).
	s.tools = s.buildRegistry()
	s.toolIndex = make(map[string]toolHandler, len(s.tools))
	s.toolReadOnly = make(map[string]bool, len(s.tools))
	// toolLock sólo guarda las clases DISTINTAS del cero: un miss devuelve lockFromReadOnly, que es
	// el default correcto. Así el mapa queda del tamaño de lo que realmente se declaró.
	s.toolLock = make(map[string]lockClass)
	for i := range s.tools {
		s.toolIndex[s.tools[i].Name] = s.tools[i].handler
		if s.tools[i].readOnly {
			s.toolReadOnly[s.tools[i].Name] = true
		}
		if s.tools[i].lock != lockFromReadOnly {
			s.toolLock[s.tools[i].Name] = s.tools[i].lock
		}
	}
	return s
}

// Start arranca el servidor sobre stdin/stdout (modo daemon).
func (s *McpServer) Start() {
	s.Serve(os.Stdin, os.Stdout)
}

// Serve procesa pedidos JSON-RPC línea a línea desde in y escribe respuestas en out.
// Es el transporte stdio (modo daemon): un solo goroutine, peticiones secuenciales.
// Cada respuesta se escribe en el out local — Serve no comparte estado mutable, así
// que Dispatch es seguro para usar concurrentemente desde otros transportes.
func (s *McpServer) Serve(in io.Reader, out io.Writer) {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')

		if len(bytes.TrimSpace(line)) > 0 {
			var req JsonRpcRequest
			if jerr := json.Unmarshal(line, &req); jerr != nil {
				writeResponse(out, JsonRpcResponse{JsonRpc: "2.0", Error: rpcErrorf(codeParseError, "Parse error")})
			} else {
				reqCtx, reqCancel := context.WithTimeout(context.Background(), 60*time.Second)
				if resp, ok := s.Dispatch(reqCtx, req); ok {
					writeResponse(out, resp)
				}
				reqCancel()
			}
		}

		if err != nil {
			if err != io.EOF {
				logx.Error("error leyendo entrada JSON-RPC", "error", err)
			}
			return
		}
	}
}

// Dispatch procesa un request JSON-RPC y DEVUELVE la respuesta (sin escribir a ningún
// writer). El segundo valor es false para notificaciones (sin id), que por spec no
// reciben respuesta. Al no tocar estado mutable compartido y leer solo campos fijados
// en NewMcpServer (toolIndex, engine, embedder), Dispatch es seguro para llamarse
// concurrentemente: cada transporte (stdio, HTTP) serializa su propia escritura.
func (s *McpServer) Dispatch(ctx context.Context, req JsonRpcRequest) (JsonRpcResponse, bool) {
	// Per JSON-RPC 2.0, una notificación (sin id) NUNCA recibe respuesta, ni
	// siquiera para métodos conocidos.
	if req.ID == nil {
		return JsonRpcResponse{}, false
	}
	if req.JsonRpc != "2.0" {
		return errResponse(req.ID, rpcErrorf(codeInvalidRequest, "jsonrpc field must be \"2.0\"")), true
	}
	// Recover de cualquier panic en handlers o en la capa de memoria/embedder,
	// para que un crash interno no mate el servidor sino que devuelva un error al cliente.
	resp := errResponse(req.ID, rpcErrorf(codeInternalError, "error interno inesperado"))
	func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Error("panic en handler", "method", req.Method, "panic", r)
				// resp ya quedó con el error interno por defecto.
			}
		}()
		switch req.Method {
		case "initialize":
			resp = okResponse(req.ID, s.handleInitialize())
		case "tools/list":
			resp = okResponse(req.ID, s.handleToolsList())
		case "tools/call":
			result, rpcErr := s.handleToolsCall(ctx, req.Params)
			if rpcErr != nil {
				resp = errResponse(req.ID, rpcErr)
			} else {
				resp = okResponse(req.ID, result)
			}
		default:
			resp = errResponse(req.ID, rpcErrorf(codeMethodNotFound, "Method not found: %s", req.Method))
		}
	}()
	return resp, true
}

func okResponse(id interface{}, result interface{}) JsonRpcResponse {
	return JsonRpcResponse{JsonRpc: "2.0", ID: id, Result: result}
}

func errResponse(id interface{}, rpcErr *RpcError) JsonRpcResponse {
	return JsonRpcResponse{JsonRpc: "2.0", ID: id, Error: rpcErr}
}

// writeResponse serializa y emite una respuesta al writer dado, reportando fallos de
// marshal a stderr (nunca a stdout, que es el canal JSON-RPC). Es stateless: el writer
// lo provee el transporte que llama, no un campo compartido del servidor.
func writeResponse(out io.Writer, res JsonRpcResponse) {
	data, err := json.Marshal(res)
	if err != nil {
		logx.Error("error serializando respuesta JSON-RPC", "error", err)
		return
	}
	fmt.Fprintf(out, "%s\n", data)
}
