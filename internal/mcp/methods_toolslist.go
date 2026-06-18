package mcp

// Definición del handshake MCP (initialize) y el catálogo de tools (tools/list).
// Separado de methods.go (dispatcher + handlers) por ser un bloque grande y mecánico.

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
					"max_tokens": {Type: "integer", Description: "Techo opcional de tokens a hidratar; recorta para no desbordar el contexto (0 = sin límite)"},
				},
				Required: []string{"ids"},
			},
		},
		{
			Name:        "musubi_maintain",
			Description: "Auto-mantenimiento de la memoria (model-free): fusiona observaciones casi-duplicadas y archiva las memorias frías de baja saliencia para mantener el recall filoso. No recibe parámetros (usa la config de maintenance). Devuelve un resumen.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "musubi_save_fact",
			Description: "Guarda un HECHO estructurado como tripleta (subject, predicate, object) en el grafo de conocimiento. Las entidades se deduplican por nombre. Recuperar hechos cuesta muchísimos menos tokens que recuperar prosa: registrá hechos atómicos (ej. 'auth' 'usa' 'JWT').",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"subject":   {Type: "string", Description: "Entidad sujeto (ej. 'auth')"},
					"predicate": {Type: "string", Description: "Relación (ej. 'usa', 'depende_de')"},
					"object":    {Type: "string", Description: "Entidad objeto (ej. 'JWT')"},
				},
				Required: []string{"subject", "predicate", "object"},
			},
		},
		{
			Name:        "musubi_recall_facts",
			Description: "Recupera HECHOS del grafo alrededor de una entidad, recorriendo hasta max_hops saltos. Devuelve tripletas compactas (no prosa), ideal para reconstruir contexto con muy pocos tokens.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"entity":   {Type: "string", Description: "Entidad desde la que recorrer el grafo"},
					"max_hops": {Type: "number", Description: "Profundidad del recorrido (opcional; usa el default de la config)"},
				},
				Required: []string{"entity"},
			},
		},
		{
			Name:        "musubi_entity_context",
			Description: "Ensambla TODO el contexto de una entidad en una sola consulta barata en tokens: sus HECHOS del grafo + los GISTS de las observaciones (prosa) que la mencionan. Es el puente grafo<->prosa; para el contenido completo de una observación usá musubi_memory_expand.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"entity":   {Type: "string", Description: "Entidad cuyo contexto se quiere reconstruir"},
					"max_hops": {Type: "number", Description: "Profundidad del grafo (opcional; usa el default de la config)"},
				},
				Required: []string{"entity"},
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
		{
			Name:        "musubi_conflicts",
			Description: "Lista las relaciones semánticas entre observaciones que esperan tu veredicto (status pending). Úsalo para revisar posibles contradicciones y luego resolvé con musubi_judge.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "musubi_judge",
			Description: "Emite el veredicto de una relación entre dos observaciones (resolución de conflictos model-free). relation ∈ {related, compatible, scoped, conflicts_with, supersedes, not_conflict}. Si es 'supersedes', la observación target queda oculta del recall.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"relation_id": {Type: "string", Description: "ID de la relación a juzgar (de musubi_conflicts o de la respuesta de save)"},
					"relation":    {Type: "string", Description: "Veredicto: related | compatible | scoped | conflicts_with | supersedes | not_conflict"},
					"reason":      {Type: "string", Description: "Justificación breve (opcional)"},
				},
				Required: []string{"relation_id", "relation"},
			},
		},
		{
			Name:        "musubi_doctor",
			Description: "Diagnostica y repara la base de memoria (integridad SQLite, índice FTS, digests, relaciones huérfanas, esquema). Sin args: diagnóstico completo. Con 'check': corre ese check. Con 'repair: true' y 'check': repara (mode plan|dry-run|apply; default dry-run). 'apply' hace un backup previo.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"check":  {Type: "string", Description: "Código del check (ej. fts_consistency, missing_digests, orphan_relations). Opcional."},
					"repair": {Type: "boolean", Description: "Si es true, repara el check indicado (requiere 'check')."},
					"mode":   {Type: "string", Description: "Modo de reparación: plan | dry-run | apply (default dry-run)."},
				},
			},
		},
		{
			Name:        "musubi_phase",
			Description: "Pipeline por fases del loop dirigido (model-free): Musubi secuencia la tarea por fases (explore→plan→code→verify) y te recuerda la fase actual en cada turno. action ∈ {status, start, advance, set, clear}. 'start' requiere 'task'; 'set' requiere 'phase'. Llamá action=advance al terminar cada fase.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"action": {Type: "string", Description: "status | start | advance | set | clear (default status)"},
					"task":   {Type: "string", Description: "Nombre de la tarea (requerido para start)"},
					"phase":  {Type: "string", Description: "Fase destino dentro de la secuencia (requerido para set)"},
				},
			},
		},
		{
			Name:        "musubi_work",
			Description: "Pizarra compartida para orquestar SUB-AGENTES en paralelo (model-free). Protocolo: 1) el agente principal descompone la tarea y postea las unidades con action=plan; 2) lanza N sub-agentes con el Task tool, pasándoles mcpServers:[musubi]; cada sub-agente hace action=claim (toma una unidad atómicamente, sin colisiones), la ejecuta y action=complete con su resultado; 3) el principal monitorea con action=status y consolida los resultados cuando todas están done. action ∈ {plan, claim, complete, status, clear}.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"action": {Type: "string", Description: "plan | claim | complete | status | clear"},
					"batch":  {Type: "string", Description: "ID del batch (plan: opcional, se genera; claim/status/clear: el batch objetivo; claim vacío toma de cualquiera)"},
					"units":  {Type: "array", Description: "Para plan: lista de unidades [{title, spec}] a postear"},
					"agent":  {Type: "string", Description: "Para claim: etiqueta del sub-agente que reclama"},
					"id":     {Type: "string", Description: "Para complete: ID de la unidad a cerrar"},
					"result": {Type: "string", Description: "Para complete: resultado/resumen producido por el sub-agente"},
					"status": {Type: "string", Description: "Para complete: done | failed (default done)"},
				},
			},
		},
		{
			Name:        "musubi_workflow",
			Description: "Motor de orquestación DAG (model-free). Musubi NO ejecuta los steps: define el grafo, persiste el estado del run en SQLite (resumible entre sesiones) y devuelve qué step(s) están listos; VOS ejecutás y reportás. Protocolo: action=start (run_id + workflow id de .musubi/workflows/<id>.yaml, o definition YAML inline) → devuelve los steps ready; ejecutás un step y hacés action=complete (run_id, step, result) → devuelve los nuevos ready; action=next para reconsultar; action=status para el estado completo. Un step queda listo solo cuando todas sus dependencias (needs) están done. action ∈ {start, next, complete, status}.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"action":     {Type: "string", Description: "start | next | complete | status"},
					"workflow":   {Type: "string", Description: "Para start: id del workflow en .musubi/workflows/<id>.yaml"},
					"definition": {Type: "string", Description: "Para start: definición YAML inline (alternativa a 'workflow')"},
					"run_id":     {Type: "string", Description: "Identificador del run (lo elegís vos; persiste y permite resume)"},
					"step":       {Type: "string", Description: "Para complete: id del step que terminaste"},
					"result":     {Type: "string", Description: "Para complete: resultado/resumen del step"},
					"status":     {Type: "string", Description: "Para complete: done | failed (default done)"},
				},
			},
		},
		{
			Name:        "musubi_tokens",
			Description: "Ledger de tokens de la sesión (model-free): cuántos tokens inyectó Musubi en el contexto (priming de arranque + recall por turno + hidratación), por superficie. action ∈ {status, reset}. Útil para medir y controlar el gasto real de la memoria.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"action": {Type: "string", Description: "status | reset (default status)"},
				},
			},
		},
		{
			Name:        "musubi_save_code",
			Description: "Memoria de CÓDIGO: guardá un gist (titular) + símbolos clave de un archivo que acabás de leer, para no tener que re-leerlo entero después (el mayor costo en tokens de una sesión es re-leer archivos). Musubi calcula un fingerprint del contenido para saber si el gist sigue fresco. Llamala tras leer un archivo grande. Requiere path y gist; symbols opcional.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path":    {Type: "string", Description: "Ruta del archivo (relativa a la raíz del proyecto o absoluta)"},
					"gist":    {Type: "string", Description: "Resumen corto de qué hace el archivo"},
					"symbols": {Type: "string", Description: "Símbolos clave y sus líneas, p.ej. 'Load() L10; parse() L42' (opcional, para lecturas dirigidas luego)"},
				},
				Required: []string{"path", "gist"},
			},
		},
		{
			Name:        "musubi_recall_code",
			Description: "Recuerda el gist + símbolos de un archivo ya leído (memoria de código), para evitar re-leerlo. Devuelve fresh=true si el archivo no cambió desde que se guardó el gist, o fresh=false si conviene re-leerlo (o leer solo los símbolos que necesitás). Llamala ANTES de leer un archivo grande.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"path": {Type: "string", Description: "Ruta del archivo a recordar"},
				},
				Required: []string{"path"},
			},
		},
	}
	return map[string]interface{}{"tools": tools}
}
