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
	// projectPath es la raíz del proyecto (== MUSUBI_HOME).
	// La usan los handlers de detect_stack y save_skill para resolver rutas.
	projectPath string
	// sourcing contiene la configuración de sourcing de skills desde catálogo remoto.
	sourcing config.SourcingConfig
	// memory contiene los parámetros del recall por presupuesto de tokens.
	memory config.MemoryConfig
	// maintenance contiene los parámetros del auto-mantenimiento (consolidar + olvidar).
	maintenance config.MaintenanceConfig
	// graph contiene los parámetros de la memoria estructurada en grafo.
	graph config.GraphConfig
	// conflicts contiene los parámetros de la detección de relaciones semánticas.
	conflicts config.ConflictConfig
	// pipeline contiene los parámetros del pipeline por fases del loop dirigido.
	pipeline config.PipelineConfig
	// multiagent contiene los parámetros de la pizarra compartida del multi-agente.
	multiagent config.MultiAgentConfig
	// tools es el catálogo ordenado de tools (fuente de tools/list); toolIndex es
	// el mapa nombre→handler para el dispatch O(1) de tools/call. Ambos se construyen
	// una vez en NewMcpServer desde buildRegistry.
	tools     []toolEntry
	toolIndex map[string]toolHandler
	// toolReadOnly[name]=true si la tool no muta estado: corre bajo RLock (concurrente
	// con otras lecturas). Las demás corren bajo Lock exclusivo.
	toolReadOnly map[string]bool
	// dispatchMu hace seguro el dispatch concurrente (transporte HTTP): las tools que
	// mutan toman Lock (serializadas, RMW-safe); las de solo-lectura toman RLock
	// (concurrentes entre sí). En stdio (un goroutine) está siempre libre, costo nulo.
	dispatchMu sync.RWMutex
	// saveCount cuenta saves desde el último disparo; al cruzar maintenance.AutoAfterSaves
	// dispara un mantenimiento async (T5.3). maintBusy garantiza un solo ciclo en vuelo.
	saveCount atomic.Int64
	maintBusy atomic.Bool
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
		projectPath: projectPath,
		sourcing:    config.Default().Sourcing,
		memory:      config.Default().Memory,
		maintenance: config.Default().Maintenance,
		graph:       config.Default().Graph,
		conflicts:   config.Default().Conflicts,
		pipeline:    config.Default().Pipeline,
		multiagent:  config.Default().MultiAgent,
	}
	for _, opt := range opts {
		opt(s)
	}
	// Construir el registro de tools una vez (los handlers leen la config de s en
	// tiempo de llamada, así que el orden respecto de las opciones no importa).
	s.tools = s.buildRegistry()
	s.toolIndex = make(map[string]toolHandler, len(s.tools))
	s.toolReadOnly = make(map[string]bool, len(s.tools))
	for i := range s.tools {
		s.toolIndex[s.tools[i].Name] = s.tools[i].handler
		if s.tools[i].readOnly {
			s.toolReadOnly[s.tools[i].Name] = true
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
