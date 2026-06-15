package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/skills"
	"musubi/internal/skillsource"

	"gopkg.in/yaml.v3"
)

const (
	defaultLimit = 5
	maxLimit     = 100
)

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Items       *Property `json:"items,omitempty"`
}

func (s *McpServer) handleInitialize() interface{} {
	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    "musubi-core",
			"version": "1.0.0",
		},
	}
}

func (s *McpServer) handleToolsList() interface{} {
	tools := []Tool{
		{
			Name:        "musubi_save_observation",
			Description: "Guarda una observación persistente o lección de aprendizaje. Si hay un proveedor de embeddings configurado, indexa el contenido para búsqueda semántica.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"topic_key":  {Type: "string", Description: "Clave de agrupación temática (ej. architecture/auth)"},
					"content":    {Type: "string", Description: "Contenido completo de la observación o lección"},
					"id":         {Type: "string", Description: "Identificador único opcional; si se omite se genera un UUID y se deduplica por contenido"},
					"importance": {Type: "number", Description: "Peso opcional (>0, default 1.0) que prioriza la observación en el recall"},
				},
				Required: []string{"topic_key", "content"},
			},
		},
		{
			Name:        "musubi_recall",
			Description: "Recall por PRESUPUESTO de tokens (model-free). Devuelve los GISTS más útiles para la consulta que entren en token_budget, rankeados por relevancia + recencia + frecuencia + importancia. Para traer el contenido completo de un item, usá musubi_memory_expand con su id. Es la forma eficiente de recuperar memoria.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query":        {Type: "string", Description: "Texto de la consulta"},
					"token_budget": {Type: "number", Description: "Techo de tokens del resultado (opcional; usa el default de la config)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "musubi_memory_expand",
			Description: "Hidrata el contenido completo de observaciones por id (hidratación perezosa tras un musubi_recall). Solo traé lo que realmente necesitás para ahorrar tokens.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"ids": {
						Type:        "array",
						Description: "Lista de ids de observaciones a expandir",
						Items:       &Property{Type: "string", Description: "id de observación"},
					},
				},
				Required: []string{"ids"},
			},
		},
		{
			Name:        "musubi_search_semantic",
			Description: "Busca observaciones por similitud semántica. Recibe TEXTO; el servidor genera el embedding. Requiere un proveedor de embeddings configurado.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {Type: "string", Description: "Texto de consulta a buscar semánticamente"},
					"limit": {Type: "number", Description: "Número máximo de resultados (por defecto 5)"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "musubi_search_keyword",
			Description: "Busca observaciones por texto completo (FTS5 de SQLite). Funciona siempre, sin necesidad de embeddings.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query_text": {Type: "string", Description: "Texto o palabras clave a buscar"},
					"limit":      {Type: "number", Description: "Número máximo de resultados (por defecto 5)"},
				},
				Required: []string{"query_text"},
			},
		},
		{
			Name:        "musubi_log_error",
			Description: "Registra un error de compilación o testeo capturado para el bucle de telemetría.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"file_path":       {Type: "string", Description: "Ruta del archivo que causó el error"},
					"error_message":   {Type: "string", Description: "Detalle completo del compilador o linter"},
					"suggested_patch": {Type: "string", Description: "Parche correctivo sugerido (opcional)"},
				},
				Required: []string{"file_path", "error_message"},
			},
		},
		{
			Name:        "musubi_resolve_telemetry",
			Description: "Marca un log de telemetría como resuelto una vez corregido el error.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"id": {Type: "number", Description: "ID del log de telemetría a marcar como resuelto"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        "musubi_resolve_skills",
			Description: "Resuelve dinámicamente las reglas y skills activas según los archivos modificados, junto con la telemetría sin resolver.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"modified_files": {
						Type:        "array",
						Description: "Listado de archivos que van a ser modificados o leídos",
						Items:       &Property{Type: "string", Description: "Ruta o nombre del archivo"},
					},
				},
				Required: []string{"modified_files"},
			},
		},
		{
			Name:        "musubi_detect_stack",
			Description: "Detecta el stack/ecosistema del proyecto actual (lenguajes y frameworks) inspeccionando archivos de manifiesto. No recibe parámetros. Devuelve JSON para que el agente investigue mejores prácticas oficiales y luego guarde skills con musubi_save_skill.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "musubi_save_skill",
			Description: "Guarda una skill generada como {name}.yaml en .musubi/skills/ y crea el sentinel para no re-generar. IMPORTANTE: usar solo después de confirmar las reglas con el usuario. Por defecto NO sobrescribe skills existentes (pasa overwrite=true para forzar).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"name":         {Type: "string", Description: "Nombre slug de la skill (solo letras minúsculas, números y guiones)"},
					"description":  {Type: "string", Description: "Descripción breve de la skill (opcional)"},
					"triggers":     {Type: "array", Description: "Globs que activan la skill (ej. '*.go')", Items: &Property{Type: "string"}},
					"capabilities": {Type: "array", Description: "Herramientas requeridas en PATH (opcional)", Items: &Property{Type: "string"}},
					"rules":        {Type: "string", Description: "Reglas de la skill en texto plano (mínimo 20 caracteres)"},
					"overwrite":    {Type: "boolean", Description: "Si es true, sobrescribe una skill existente (por defecto false)"},
				},
				Required: []string{"name", "triggers", "rules"},
			},
		},
		{
			Name:        "musubi_search_skills",
			Description: "Busca skills aplicables al proyecto actual desde el catálogo remoto. Filtra por ecosistema, dependencias, triggers y capabilities. Devuelve candidatos con evidencia de aplicabilidad.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"query": {Type: "string", Description: "Texto libre para filtrar candidatos por nombre o descripción (opcional)"},
					"stack": {Type: "string", Description: "Filtrar por ecosistema específico (ej. 'Go', 'Node.js') (opcional)"},
					"limit": {Type: "number", Description: "Cantidad máxima de resultados (usa MaxCandidates de la config por defecto)"},
				},
			},
		},
		{
			Name:        "musubi_log_skill_decision",
			Description: "Registra una decisión de skill (aceptada o rechazada) en el log persistente de SQLite. Útil para auditar qué skills se adoptaron y por qué.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"skill_id": {Type: "string", Description: "Identificador slug de la skill (ej. 'go-gin')"},
					"name":     {Type: "string", Description: "Nombre legible de la skill (opcional si se provee skill_id)"},
					"decision": {Type: "string", Description: "Decisión tomada: 'accepted' o 'rejected'"},
					"reason":   {Type: "string", Description: "Justificación de la decisión (opcional)"},
				},
				Required: []string{"skill_id", "decision"},
			},
		},
	}
	return map[string]interface{}{"tools": tools}
}

type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type CallToolResponse struct {
	Content []TextContent `json:"content"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func textResult(text string) interface{} {
	return CallToolResponse{Content: []TextContent{{Type: "text", Text: text}}}
}

func jsonResult(v interface{}) (interface{}, *RpcError) {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al serializar resultado: %v", err)
	}
	return textResult(string(jsonBytes)), nil
}

// clampLimit normaliza el límite recibido a un rango razonable.
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func (s *McpServer) handleToolsCall(params json.RawMessage) (interface{}, *RpcError) {
	var callReq CallToolRequest
	if err := json.Unmarshal(params, &callReq); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid params: %v", err)
	}

	switch callReq.Name {
	case "musubi_save_observation":
		return s.toolSaveObservation(callReq.Arguments)
	case "musubi_search_semantic":
		return s.toolSearchSemantic(callReq.Arguments)
	case "musubi_search_keyword":
		return s.toolSearchKeyword(callReq.Arguments)
	case "musubi_recall":
		return s.toolRecall(callReq.Arguments)
	case "musubi_memory_expand":
		return s.toolMemoryExpand(callReq.Arguments)
	case "musubi_log_error":
		return s.toolLogError(callReq.Arguments)
	case "musubi_resolve_telemetry":
		return s.toolResolveTelemetry(callReq.Arguments)
	case "musubi_resolve_skills":
		return s.toolResolveSkills(callReq.Arguments)
	case "musubi_detect_stack":
		return s.toolDetectStack(callReq.Arguments)
	case "musubi_save_skill":
		return s.toolSaveSkill(callReq.Arguments)
	case "musubi_search_skills":
		return s.toolSearchSkills(callReq.Arguments)
	case "musubi_log_skill_decision":
		return s.toolLogSkillDecision(callReq.Arguments)
	default:
		return nil, rpcErrorf(codeMethodNotFound, "Tool not found: %s", callReq.Name)
	}
}

func (s *McpServer) toolSaveObservation(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID         string  `json:"id"`
		TopicKey   string  `json:"topic_key"`
		Content    string  `json:"content"`
		Importance float64 `json:"importance"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.TopicKey) == "" {
		return nil, rpcErrorf(codeInvalidParams, "topic_key es obligatorio")
	}
	if strings.TrimSpace(args.Content) == "" {
		return nil, rpcErrorf(codeInvalidParams, "content es obligatorio")
	}
	importance := args.Importance
	if importance <= 0 {
		importance = 1.0
	}

	var emb []float32
	if embedding.Enabled(s.embedder) {
		vec, err := s.embedder.Embed(context.Background(), args.Content)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al generar embedding: %v", err)
		}
		emb = vec
	}

	// Sin id explícito: deduplicar por contenido y autogenerar UUID.
	if strings.TrimSpace(args.ID) == "" {
		id, deduped, err := s.engine.SaveObservationDeduped(args.TopicKey, args.Content, importance, emb)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "error al guardar observación: %v", err)
		}
		if deduped {
			return textResult("Observación ya existente, no se duplicó (id: " + id + ")."), nil
		}
		return textResult("Observación guardada con éxito (id: " + id + ")."), nil
	}

	// Con id explícito: upsert por id.
	if err := s.engine.SaveObservationWithImportance(args.ID, args.TopicKey, args.Content, importance, emb); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar observación: %v", err)
	}
	return textResult("Observación guardada con éxito (id: " + args.ID + ")."), nil
}

// maxRecallBudget acota el presupuesto pedido por el cliente a un rango sano.
const maxRecallBudget = 8000

func (s *McpServer) toolRecall(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query       string `json:"query"`
		TokenBudget int    `json:"token_budget"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query es obligatorio")
	}

	opts := memory.RecallOptions{
		TokenBudget:   s.memory.RecallTokenBudget,
		CandidatePool: s.memory.CandidatePool,
		GistMaxTokens: s.memory.GistMaxTokens,
	}
	if args.TokenBudget > 0 {
		opts.TokenBudget = args.TokenBudget
		if opts.TokenBudget > maxRecallBudget {
			opts.TokenBudget = maxRecallBudget
		}
	}

	res, err := s.engine.Recall(args.Query, opts)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en recall: %v", err)
	}
	return jsonResult(res)
}

func (s *McpServer) toolMemoryExpand(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if len(args.IDs) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "ids no puede estar vacío")
	}

	res, err := s.engine.GetObservations(args.IDs)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al expandir memorias: %v", err)
	}
	return jsonResult(res)
}

func (s *McpServer) toolSearchSemantic(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Query) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query es obligatorio")
	}
	if !embedding.Enabled(s.embedder) {
		return nil, rpcErrorf(codeInvalidParams, "búsqueda semántica no disponible: no hay proveedor de embeddings configurado. Usá musubi_search_keyword o configurá embedding.provider en .musubi/config.yaml")
	}

	vec, err := s.embedder.Embed(context.Background(), args.Query)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al generar embedding de la consulta: %v", err)
	}

	results, err := s.engine.SearchObservations(vec, clampLimit(args.Limit))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en búsqueda semántica: %v", err)
	}
	return jsonResult(results)
}

func (s *McpServer) toolSearchKeyword(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		QueryText string `json:"query_text"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.QueryText) == "" {
		return nil, rpcErrorf(codeInvalidParams, "query_text es obligatorio")
	}

	results, err := s.engine.SearchObservationsFTS(args.QueryText, clampLimit(args.Limit))
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en búsqueda por palabra clave: %v", err)
	}
	return jsonResult(results)
}

func (s *McpServer) toolLogError(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		FilePath       string `json:"file_path"`
		ErrorMessage   string `json:"error_message"`
		SuggestedPatch string `json:"suggested_patch"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.FilePath) == "" {
		return nil, rpcErrorf(codeInvalidParams, "file_path es obligatorio")
	}
	if strings.TrimSpace(args.ErrorMessage) == "" {
		return nil, rpcErrorf(codeInvalidParams, "error_message es obligatorio")
	}

	if err := s.engine.SaveTelemetryLog(args.FilePath, args.ErrorMessage, args.SuggestedPatch); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar log de telemetría: %v", err)
	}
	return textResult("Log de telemetría guardado con éxito."), nil
}

func (s *McpServer) toolResolveTelemetry(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if args.ID <= 0 {
		return nil, rpcErrorf(codeInvalidParams, "id debe ser un entero positivo")
	}

	if err := s.engine.ResolveTelemetryLog(args.ID); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al resolver telemetría: %v", err)
	}
	return textResult("Log de telemetría marcado como resuelto."), nil
}

// slugRegex valida que el nombre de una skill sea un slug seguro para usar como nombre de archivo.
var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)

// toolDetectStack detecta el ecosistema del proyecto usando el projectPath del servidor.
// No requiere parámetros; devuelve el slice []StackResult serializado como JSON.
func (s *McpServer) toolDetectStack(raw json.RawMessage) (interface{}, *RpcError) {
	results, err := detector.DetectStack(s.projectPath)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al detectar stack: %v", err)
	}
	return jsonResult(results)
}

// argsGuardarSkill contiene los parámetros del tool musubi_save_skill.
type argsGuardarSkill struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Triggers     []string `json:"triggers"`
	Capabilities []string `json:"capabilities"`
	Rules        string   `json:"rules"`
	Overwrite    bool     `json:"overwrite"`
}

// toolSaveSkill valida los argumentos y guarda la skill como YAML en .musubi/skills/.
// También escribe el sentinel de manera best-effort.
func (s *McpServer) toolSaveSkill(raw json.RawMessage) (interface{}, *RpcError) {
	var args argsGuardarSkill
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}

	// Validar nombre: no vacío y slug-safe (previene path traversal).
	if strings.TrimSpace(args.Name) == "" {
		return nil, rpcErrorf(codeInvalidParams, "name es obligatorio")
	}
	if !slugRegex.MatchString(args.Name) {
		return nil, rpcErrorf(codeInvalidParams, "name debe ser un slug válido (solo letras minúsculas, números y guiones, ej. 'mi-skill'): %q", args.Name)
	}

	// Validar triggers: al menos uno y cada uno debe ser un glob válido.
	if len(args.Triggers) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "triggers no puede estar vacío")
	}
	for _, t := range args.Triggers {
		if _, err := filepath.Match(t, ""); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "trigger inválido %q: %v", t, err)
		}
	}

	// Validar rules: no vacío y mínimo 20 caracteres.
	if len(strings.TrimSpace(args.Rules)) < 20 {
		return nil, rpcErrorf(codeInvalidParams, "rules debe tener al menos 20 caracteres (actual: %d)", len(strings.TrimSpace(args.Rules)))
	}

	// Construir ruta y aplicar defensa de path traversal adicional.
	skillsDir := filepath.Join(s.projectPath, config.DirName, config.SkillsDir)
	skillPath := filepath.Join(skillsDir, args.Name+".yaml")
	// Verificar que la ruta resultante está bajo el directorio de skills (cinturón y tirantes).
	if !strings.HasPrefix(filepath.Clean(skillPath), filepath.Clean(skillsDir)) {
		return nil, rpcErrorf(codeInvalidParams, "nombre de skill no permitido: %q", args.Name)
	}

	// Puerta de sobrescritura: rechazar si el archivo existe y overwrite=false.
	if _, err := os.Stat(skillPath); err == nil && !args.Overwrite {
		return nil, rpcErrorf(codeInvalidParams, "la skill %q ya existe; pasa overwrite=true para reemplazarla", args.Name)
	}

	// Construir la skill con campos de procedencia.
	sk := skills.Skill{
		Name:         args.Name,
		Description:  args.Description,
		Triggers:     args.Triggers,
		Capabilities: args.Capabilities,
		Rules:        args.Rules,
		GeneratedBy:  "auto-discovery",
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// Serializar a YAML.
	data, err := yaml.Marshal(sk)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al serializar skill: %v", err)
	}

	// Crear el directorio de skills si no existe.
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al crear directorio de skills: %v", err)
	}

	// Escribir el archivo YAML de la skill.
	if err := os.WriteFile(skillPath, data, 0644); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al escribir skill: %v", err)
	}

	// Escribir el sentinel (best-effort: fallo no cancela el guardado de la skill).
	sentinelPath := filepath.Join(skillsDir, config.SentinelFile)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(sentinelPath, []byte(timestamp), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "musubi: advertencia: no se pudo escribir el sentinel: %v\n", err)
	}

	return textResult(fmt.Sprintf("skill %q guardada en %s", args.Name, skillPath)), nil
}

// toolSearchSkills busca skills aplicables al proyecto desde el catálogo remoto.
// Inputs opcionales: query (string), stack (string), limit (int).
// Degradación graciosa: catálogo caído → textResult con guía, sin RpcError.
func (s *McpServer) toolSearchSkills(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Query string `json:"query"`
		Stack string `json:"stack"`
		Limit int    `json:"limit"`
	}
	if raw != nil {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	// Si el sourcing está deshabilitado, guiar al fallback sin tocar el catálogo.
	if !s.sourcing.Enabled {
		return textResult("El sourcing de skills está deshabilitado en la configuración de Musubi. " +
			"Para buscar skills manualmente, investigá la documentación oficial del stack detectado " +
			"y usá musubi_save_skill para guardar las reglas."), nil
	}

	// Obtener el catálogo con timeout vía contexto (5 segundos).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cat, err := skillsource.FetchCatalog(ctx, s.sourcing.CatalogURL)
	if err != nil {
		// Degradación graciosa: catálogo inaccesible → texto explicativo, no RpcError.
		return textResult(fmt.Sprintf(
			"El catálogo de skills no está disponible en este momento (%v). "+
				"Podés buscar skills manualmente en la documentación oficial del stack detectado "+
				"y guardarlas con musubi_save_skill.", err)), nil
	}

	// Detectar stack y dependencias del proyecto actual.
	stacks, _ := detector.DetectStack(s.projectPath)
	deps, _ := detector.ExtractDeps(s.projectPath)

	// Determinar límite efectivo.
	maxCands := s.sourcing.MaxCandidates
	if args.Limit > 0 && args.Limit < maxCands {
		maxCands = args.Limit
	}

	// Filtrar candidatos aplicables.
	candidatos := skillsource.FilterCatalog(cat, s.projectPath, deps, stacks, maxCands)

	// Filtro adicional en memoria por stack (si se especificó).
	if args.Stack != "" {
		filtrados := candidatos[:0]
		for _, c := range candidatos {
			for _, st := range c.Entry.Stacks {
				if strings.EqualFold(st, args.Stack) {
					filtrados = append(filtrados, c)
					break
				}
			}
		}
		candidatos = filtrados
	}

	// Filtro adicional por query (nombre o descripción).
	if args.Query != "" {
		q := strings.ToLower(args.Query)
		filtrados := candidatos[:0]
		for _, c := range candidatos {
			if strings.Contains(strings.ToLower(c.Entry.Name), q) ||
				strings.Contains(strings.ToLower(c.Entry.Description), q) {
				filtrados = append(filtrados, c)
			}
		}
		candidatos = filtrados
	}

	return jsonResult(candidatos)
}

// toolLogSkillDecision registra una decisión de skill (accepted/rejected) en SQLite.
// Inputs: skill_id (requerido), decision (requerido, "accepted"|"rejected"),
// name (opcional), reason (opcional).
func (s *McpServer) toolLogSkillDecision(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		SkillID  string `json:"skill_id"`
		Name     string `json:"name"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}

	// Validar skill_id: requerido y debe ser slug válido.
	if strings.TrimSpace(args.SkillID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "skill_id es obligatorio")
	}
	if !slugRegex.MatchString(args.SkillID) {
		return nil, rpcErrorf(codeInvalidParams, "skill_id debe ser un slug válido (solo letras minúsculas, números y guiones): %q", args.SkillID)
	}

	// Validar decision: debe ser "accepted" o "rejected".
	if args.Decision != "accepted" && args.Decision != "rejected" {
		return nil, rpcErrorf(codeInvalidParams, "decision debe ser 'accepted' o 'rejected', se recibió: %q", args.Decision)
	}

	if err := s.engine.SaveSkillDecision(args.SkillID, args.Name, args.Decision, args.Reason); err != nil {
		return nil, rpcErrorf(codeInternalError, "error al guardar decisión de skill: %v", err)
	}
	return textResult(fmt.Sprintf("Decisión '%s' para skill '%s' registrada con éxito.", args.Decision, args.SkillID)), nil
}

func (s *McpServer) toolResolveSkills(raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ModifiedFiles []string `json:"modified_files"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if len(args.ModifiedFiles) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "modified_files no puede estar vacío")
	}

	activeSkills, err := s.resolver.ResolveSkills(args.ModifiedFiles)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al resolver skills: %v", err)
	}

	telemetryLogs, err := s.engine.GetUnresolvedTelemetryLogs()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al obtener telemetría: %v", err)
	}

	return jsonResult(map[string]interface{}{
		"active_skills":  activeSkills,
		"telemetry_logs": telemetryLogs,
	})
}
