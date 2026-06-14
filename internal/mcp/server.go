package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/skills"
)

// Códigos de error JSON-RPC 2.0 estándar usados por el servidor.
const (
	codeParseError     = -32700
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

type McpServer struct {
	engine      *memory.DbEngine
	resolver    *skills.Resolver
	embedder    embedding.Provider
	// projectPath es la raíz del proyecto (== MUSUBI_HOME).
	// La usan los handlers de detect_stack y save_skill para resolver rutas.
	projectPath string
	out         io.Writer
}

// NewMcpServer construye el servidor MCP. embedder genera embeddings a partir de
// texto; usá embedding.NoopProvider{} para desactivar la búsqueda semántica.
func NewMcpServer(engine *memory.DbEngine, projectPath string, embedder embedding.Provider) *McpServer {
	if embedder == nil {
		embedder = embedding.NoopProvider{}
	}
	return &McpServer{
		engine:      engine,
		resolver:    skills.NewResolver(projectPath),
		embedder:    embedder,
		projectPath: projectPath,
		out:         os.Stdout,
	}
}

// Start arranca el servidor sobre stdin/stdout (modo daemon).
func (s *McpServer) Start() {
	s.Serve(os.Stdin, os.Stdout)
}

// Serve procesa pedidos JSON-RPC línea a línea desde in y escribe respuestas en out.
// El loop es de un solo goroutine; *sql.DB es seguro pero las peticiones se
// atienden de forma secuencial.
func (s *McpServer) Serve(in io.Reader, out io.Writer) {
	s.out = out
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadBytes('\n')

		if len(bytes.TrimSpace(line)) > 0 {
			var req JsonRpcRequest
			if jerr := json.Unmarshal(line, &req); jerr != nil {
				s.send(JsonRpcResponse{JsonRpc: "2.0", Error: rpcErrorf(codeParseError, "Parse error")})
			} else {
				s.handleRequest(req)
			}
		}

		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "musubi: error leyendo entrada: %v\n", err)
			}
			return
		}
	}
}

func (s *McpServer) handleRequest(req JsonRpcRequest) {
	switch req.Method {
	case "initialize":
		s.sendResult(req.ID, s.handleInitialize())
	case "tools/list":
		s.sendResult(req.ID, s.handleToolsList())
	case "tools/call":
		result, rpcErr := s.handleToolsCall(req.Params)
		if rpcErr != nil {
			s.sendError(req.ID, rpcErr)
			return
		}
		s.sendResult(req.ID, result)
	default:
		// Las notificaciones (sin id) no requieren respuesta.
		if req.ID != nil {
			s.sendError(req.ID, rpcErrorf(codeMethodNotFound, "Method not found: %s", req.Method))
		}
	}
}

func (s *McpServer) sendResult(id interface{}, result interface{}) {
	s.send(JsonRpcResponse{JsonRpc: "2.0", ID: id, Result: result})
}

func (s *McpServer) sendError(id interface{}, rpcErr *RpcError) {
	s.send(JsonRpcResponse{JsonRpc: "2.0", ID: id, Error: rpcErr})
}

// send serializa y emite una respuesta, reportando fallos de marshal a stderr
// (nunca a stdout, que es el canal JSON-RPC).
func (s *McpServer) send(res JsonRpcResponse) {
	data, err := json.Marshal(res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi: error serializando respuesta: %v\n", err)
		return
	}
	fmt.Fprintf(s.out, "%s\n", data)
}
