package mcp

// Registro de tools map-based: ÚNICA fuente de verdad del catálogo MCP.
//
// Antes, agregar una tool exigía tocar tres lugares desincronizables (el schema en
// la lista de tools/list, un `case` en el switch de tools/call, y un conteo manual
// en los tests). Ahora cada tool es una sola `toolEntry` que liga su schema y su
// handler; tools/list itera el registro (orden determinista) y tools/call resuelve
// por mapa en O(1). Agregar una tool = una entrada acá. El conteo es derivado.

import (
	"context"
	"encoding/json"
)

// toolHandler es la firma uniforme de todo handler de tool. Recibe el contexto del
// request (con su timeout) y los argumentos crudos; devuelve el resultado o un error
// JSON-RPC.
type toolHandler func(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError)

// toolEntry liga el schema público de una tool (lo que ve tools/list) con su handler
// (lo que ejecuta tools/call). Es la unidad atómica del registro.
//
// readOnly marca las tools que NO mutan estado (ni DB, ni índice, ni ledger, ni
// bumpAccess): el dispatch las corre bajo RLock (concurrentes entre sí). El default es
// false = se asume que muta y corre bajo Lock exclusivo (fail-safe: una tool nueva es
// segura por defecto; recién marcás readOnly tras VERIFICAR que es pura lectura).
type toolEntry struct {
	Tool
	handler  toolHandler
	readOnly bool
}

// noCtx adapta un handler que no usa el contexto del request a la firma uniforme
// toolHandler, sin tocar el cuerpo del handler.
func noCtx(h func(json.RawMessage) (interface{}, *RpcError)) toolHandler {
	return func(_ context.Context, raw json.RawMessage) (interface{}, *RpcError) {
		return h(raw)
	}
}

// handleInitialize responde el handshake MCP (initialize).
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

// handleToolsList construye la respuesta de tools/list iterando el registro en orden.
func (s *McpServer) handleToolsList() interface{} {
	tools := make([]Tool, 0, len(s.tools))
	for i := range s.tools {
		tools = append(tools, s.tools[i].Tool)
	}
	return map[string]interface{}{"tools": tools}
}

// buildRegistry devuelve el catálogo ordenado de tools. El ORDEN define la salida de
// tools/list (congelado por TestToolsListGolden). Para agregar una tool: agregá su
// toolEntry acá (schema + handler) y nada más.
func (s *McpServer) buildRegistry() []toolEntry {
	return []toolEntry{
		{
			Tool: Tool{
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
			handler: noCtx(s.countingSave(s.toolSaveObservation)),
		},
		{
			Tool: Tool{
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
			handler: s.toolRecall,
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolMemoryExpand),
		},
		{
			Tool: Tool{
				Name:        "musubi_maintain",
				Description: "Auto-mantenimiento de la memoria (model-free): fusiona observaciones casi-duplicadas y archiva las memorias frías de baja saliencia para mantener el recall filoso. Throttled: si el último mantenimiento fue hace poco devuelve un no-op (skipped) en vez de re-correr el ciclo (consolidación + VACUUM). Pasá force=true para ignorar el throttle. Devuelve un resumen.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"force": {Type: "boolean", Description: "Si es true, corre el ciclo aunque el throttle indique que no toca todavía."},
					},
				},
			},
			handler: noCtx(s.toolMaintain),
		},
		{
			Tool: Tool{
				Name:        "musubi_save_fact",
				Description: "Guarda un HECHO estructurado como tripleta (subject, predicate, object) en el grafo de conocimiento. Las entidades se deduplican por nombre. Recuperar hechos cuesta muchísimos menos tokens que recuperar prosa: registrá hechos atómicos (ej. 'auth' 'usa' 'JWT'). El grafo es BI-TEMPORAL: para un predicado FUNCIONAL (single-valued: trabaja_en, estado_actual, vive_en…) guardar un nuevo objeto INVALIDA automáticamente el anterior (model-free, por cardinalidad) en vez de acumular contradicciones; el hecho viejo no se borra, se cierra su ventana (consultable con recall_facts as_of). Re-afirmar un hecho invalidado lo revive.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"subject":    {Type: "string", Description: "Entidad sujeto (ej. 'auth')"},
						"predicate":  {Type: "string", Description: "Relación (ej. 'usa', 'depende_de')"},
						"object":     {Type: "string", Description: "Entidad objeto (ej. 'JWT')"},
						"valid_from": {Type: "string", Description: "Opcional: marca ISO desde la cual el hecho es verdad (ej. '2026-01-15'). Ausente/ inválido → ahora. No se infieren fechas de texto libre."},
					},
					Required: []string{"subject", "predicate", "object"},
				},
			},
			handler: noCtx(s.countingSave(s.toolSaveFact)),
		},
		{
			Tool: Tool{
				Name:        "musubi_recall_facts",
				Description: "Recupera HECHOS del grafo alrededor de una entidad, recorriendo hasta max_hops saltos. Devuelve tripletas compactas (no prosa), ideal para reconstruir contexto con muy pocos tokens. Por defecto devuelve sólo la VERDAD ACTUAL (los hechos invalidados por cardinalidad quedan fuera); pasá as_of para una consulta point-in-time (qué era verdad en ese momento).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"entity":   {Type: "string", Description: "Entidad desde la que recorrer el grafo"},
						"max_hops": {Type: "number", Description: "Profundidad del recorrido (opcional; usa el default de la config)"},
						"as_of":    {Type: "string", Description: "Opcional: marca ISO para consulta point-in-time (devuelve los hechos válidos en ese instante). Inválido → verdad actual."},
					},
					Required: []string{"entity"},
				},
			},
			handler:  noCtx(s.toolRecallFacts),
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler:  noCtx(s.toolEntityContext),
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler:  s.toolSearchSemantic,
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler:  s.toolSearchKeyword,
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolLogError),
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolResolveTelemetry),
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolResolveSkills),
		},
		{
			Tool: Tool{
				Name:        "musubi_detect_stack",
				Description: "Detecta el stack/ecosistema del proyecto actual (lenguajes y frameworks) inspeccionando archivos de manifiesto. No recibe parámetros. Devuelve JSON para que el agente investigue mejores prácticas oficiales y luego guarde skills con musubi_save_skill.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			handler:  noCtx(s.toolDetectStack),
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolSaveSkill),
		},
		{
			Tool: Tool{
				Name:        "musubi_search_skills",
				Description: "Busca skills aplicables al proyecto actual desde el catálogo remoto. Filtra por ecosistema, dependencias, triggers y capabilities, y EXCLUYE las skills que el usuario ya rechazó (aprende de musubi_log_skill_decision). Devuelve candidatos con evidencia de aplicabilidad.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {Type: "string", Description: "Texto libre para filtrar candidatos por nombre o descripción (opcional)"},
						"stack": {Type: "string", Description: "Filtrar por ecosistema específico (ej. 'Go', 'Node.js') (opcional)"},
						"limit": {Type: "number", Description: "Cantidad máxima de resultados (usa MaxCandidates de la config por defecto)"},
					},
				},
			},
			handler:  noCtx(s.toolSearchSkills),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_discover_skills",
				Description: "Descubre Agent Skills (SKILL.md) de la comunidad, filtradas por el stack del proyecto. Si no se pasa 'query', la deriva del stack detectado. Lee un catálogo curado y estático por default (cero rate limit) y cae a la API del marketplace en vivo si el catálogo no está disponible. Devuelve candidatos con su 'githubUrl' para que el usuario los REVISE e instale: Musubi NO instala ni ejecuta nada (contenido no confiable). Opt-in (sourcing.marketplace_enabled); degrada con gracia si está deshabilitado o la red cae.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {Type: "string", Description: "Texto de búsqueda (opcional; por defecto se deriva del stack del proyecto)"},
						"limit": {Type: "number", Description: "Cantidad máxima de resultados (1-100, por defecto 20)"},
					},
				},
			},
			handler:  noCtx(s.toolDiscoverSkills),
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolLogSkillDecision),
		},
		{
			Tool: Tool{
				Name:        "musubi_conflicts",
				Description: "Lista las relaciones semánticas entre observaciones que esperan tu veredicto (status pending). Úsalo para revisar posibles contradicciones y luego resolvé con musubi_judge.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			handler:  noCtx(s.toolConflicts),
			readOnly: true,
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolJudge),
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolDoctor),
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolPhase),
		},
		{
			Tool: Tool{
				Name:        "musubi_work",
				Description: "Pizarra compartida para orquestar SUB-AGENTES en paralelo (model-free). Protocolo: 1) el agente principal descompone la tarea y postea las unidades con action=plan; 2) lanza N sub-agentes con el Task tool, pasándoles mcpServers:[musubi]; cada sub-agente hace action=claim (toma una unidad atómicamente y con un LEASE, sin colisiones), la ejecuta y action=complete con su resultado; 3) el principal monitorea con action=status y consolida los resultados cuando todas están done. El claim devuelve la unidad con su fencing_token; mientras trabaja, el sub-agente DEBE renovar el lease con action=heartbeat (id + agent + fencing_token) para no perder la unidad — si un agente crashea y no renueva, su unidad se recicla automáticamente al vencer el lease (semántica at-least-once: el trabajo debe ser idempotente). action=savings estima los tokens ahorrados por delegar vs. hacerlo inline (estimación model-free configurable). action ∈ {plan, claim, heartbeat, complete, status, savings, clear}.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action":        {Type: "string", Description: "plan | claim | heartbeat | complete | status | savings | clear"},
						"batch":         {Type: "string", Description: "ID del batch (plan: opcional, se genera; claim/status/clear: el batch objetivo; claim vacío toma de cualquiera)"},
						"units":         {Type: "array", Description: "Para plan: lista de unidades [{title, spec}] a postear"},
						"agent":         {Type: "string", Description: "Para claim/heartbeat/complete: etiqueta del sub-agente (dueño del lease)"},
						"id":            {Type: "string", Description: "Para heartbeat/complete: ID de la unidad"},
						"result":        {Type: "string", Description: "Para complete: resultado/resumen producido por el sub-agente"},
						"status":        {Type: "string", Description: "Para complete: done | failed (default done)"},
						"fencing_token": {Type: "number", Description: "Para heartbeat/complete: el fencing_token que devolvió el claim (defiende contra un agente expropiado que revive)"},
					},
				},
			},
			handler: noCtx(s.toolWork),
		},
		{
			Tool: Tool{
				Name:        "musubi_workflow",
				Description: "Motor de orquestación DAG (model-free). Musubi NO ejecuta los steps: define el grafo, persiste el estado del run en SQLite (resumible entre sesiones) y devuelve qué step(s) están listos; VOS ejecutás y reportás. Protocolo: action=start (run_id + workflow id de .musubi/workflows/<id>.yaml, o definition YAML inline) → devuelve los steps ready; ejecutás un step y hacés action=complete (run_id, step, result) → devuelve los nuevos ready; action=next para reconsultar; action=status para el estado completo; action=resume para retomar un run en otra sesión (estado + ready). Un step queda listo cuando todas sus dependencias (needs) están done o skipped. Control de flujo: un step puede llevar `when` (expresión, ej. `step.build.status == done and step.test.result contains ok`); si es falsa el step se salta (gate/if_then/switch). Un step con `repeat_while` (+ `max_iterations`) se re-ejecuta como loop mientras la condición sea verdadera. Cada avance se registra en un JOURNAL append-only (auditoría/observabilidad): action=journal (run_id) devuelve la traza de eventos del run; action=otel (run_id) exporta el run como una traza OpenTelemetry (OTLP/JSON, run=trace, step=span) lista para un collector; complete acepta un idempotency_key opcional (reintentar con la misma clave es un no-op seguro). SAGA (compensación LIFO): un step puede declarar `compensate` (directiva de cómo deshacerlo); action=rollback (run_id) inicia la saga y devuelve el plan de compensación en orden inverso (LIFO) de los steps completados con compensación; ejecutás cada undo y reportás con action=compensated (run_id, step) → devuelve el plan restante; al vaciarse, el run queda `compensated`. action=validate valida una definición sin correrla; action=list lista los runs. action ∈ {start, next, complete, status, resume, validate, list, journal, otel, rollback, compensated}.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action":          {Type: "string", Description: "start | next | complete | status | resume | validate | list | journal | otel | rollback | compensated"},
						"workflow":        {Type: "string", Description: "Para start: id del workflow en .musubi/workflows/<id>.yaml"},
						"definition":      {Type: "string", Description: "Para start: definición YAML inline (alternativa a 'workflow')"},
						"run_id":          {Type: "string", Description: "Identificador del run (lo elegís vos; persiste y permite resume)"},
						"step":            {Type: "string", Description: "Para complete: id del step que terminaste"},
						"result":          {Type: "string", Description: "Para complete: resultado/resumen del step"},
						"status":          {Type: "string", Description: "Para complete: done | failed (default done)"},
						"idempotency_key": {Type: "string", Description: "Para complete (opcional): clave de idempotencia; reintentar con la misma clave es un no-op seguro"},
					},
				},
			},
			handler: noCtx(s.toolWorkflow),
		},
		{
			Tool: Tool{
				Name:        "musubi_tokens",
				Description: "Ledger de tokens de la sesión (model-free): cuántos tokens inyectó Musubi en el contexto, por superficie (arranque, por turno, PreToolUse, hidratación) y contra el presupuesto blando de sesión. Devuelve total, desglose ordenado por gasto, y —si hay presupuesto— restante, % usado y estado (ok | watch | over). action ∈ {status, reset}. Útil para medir y controlar el gasto real de la memoria.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action": {Type: "string", Description: "status | reset (default status)"},
					},
				},
			},
			handler: noCtx(s.toolTokens),
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.countingSave(s.toolSaveCode)),
		},
		{
			Tool: Tool{
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
			handler: noCtx(s.toolRecallCode),
		},
		{
			Tool: Tool{
				Name:        "musubi_insights",
				Description: "Resumen de observabilidad activa (model-free): estado de la memoria (observaciones totales/activas/archivadas), hotspots de archivos con más errores no resueltos, decisiones de skills (aceptadas/rechazadas por su decisión más reciente), último mantenimiento y salud del ciclo. Sin parámetros. Útil para ver de un vistazo qué aprendió Musubi del proyecto.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			handler:  noCtx(s.toolInsights),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_sdd",
				Description: "Flujo SDD GUIADO (Spec-Driven Development) sobre el motor DAG (model-free). Genera por vos el workflow canónico de un cambio —proposal→spec→design→tasks→implement→verify→archive— sin escribir YAML, y te guía fase por fase: en cada una devuelve su directiva y la ruta de su plantilla (.musubi/templates/sdd/). Al cerrar una fase (action=complete) persiste su CONTRATO DE RESULTADO (summary + artifacts + risks + next_recommended) en memoria bajo sdd/<change>/<phase>; las fases siguientes (sobre todo implement) recuperan esos artefactos por referencia barata con musubi_recall en vez de releer archivos. El run es resumible entre sesiones. Protocolo: action=start (change) → fase activa + directiva; ejecutás la fase y action=complete (change, phase, summary[, artifacts, risks, next_recommended]) → siguiente fase; action=next/status para reconsultar. action ∈ {start, next, complete, status}.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action":           {Type: "string", Description: "start | next | complete | status"},
						"change":           {Type: "string", Description: "Nombre del cambio/feature (identifica el flujo; se normaliza a slug)"},
						"phase":            {Type: "string", Description: "Para complete: fase que cerrás (proposal|spec|design|tasks|implement|verify|archive)"},
						"summary":          {Type: "string", Description: "Para complete: resumen ejecutivo del resultado de la fase (obligatorio)"},
						"artifacts":        {Type: "array", Description: "Para complete: artefactos producidos (rutas/ids)", Items: &Property{Type: "string"}},
						"risks":            {Type: "array", Description: "Para complete: riesgos detectados", Items: &Property{Type: "string"}},
						"next_recommended": {Type: "string", Description: "Para complete: siguiente paso recomendado"},
						"status":           {Type: "string", Description: "Para complete: done | failed (default done)"},
					},
				},
			},
			handler: noCtx(s.toolSDD),
		},
		{
			Tool: Tool{
				Name:        "musubi_author_skill",
				Description: "Sistema de creación AVANZADO de skills (model-free): valida la calidad de una skill contra las best-practices de Agent Skills (description como disparador en 3ª persona ≤1024 chars, name sin reservadas, triggers acotados, rules concisas con ejemplo, sin anti-patrones) y devuelve un REPORTE SCOREADO (score 0-100 + errores que bloquean + warnings + fixes accionables) SIN guardar (save=false, default) para iterar. Con save=true guarda la skill SOLO si pasa el gate. Recomendá derivar las rules de FUENTES CONFIABLES: doc oficial del stack + repos reputados (anthropics/skills, awesome-cursorrules, Gentleman-Skills); pasá source_url y reporta su tier de confiabilidad (official>curated>community). Usalo para crear skills eficientes y útiles antes de musubi_save_skill.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"name":         {Type: "string", Description: "Nombre slug de la skill (minúsculas, números y guiones; gerundio recomendado, ej. 'processing-go-files')"},
						"description":  {Type: "string", Description: "Description en TERCERA persona que dice QUÉ hace + CUÁNDO usarla (cláusula 'Use when …'); es el disparador. ≤1024 chars."},
						"triggers":     {Type: "array", Description: "Globs acotados que activan la skill (ej. '*.go'); evitá '*' (dispara siempre)", Items: &Property{Type: "string"}},
						"capabilities": {Type: "array", Description: "Herramientas requeridas en PATH (opcional)", Items: &Property{Type: "string"}},
						"rules":        {Type: "string", Description: "Reglas concisas y accionables, idealmente con un ejemplo (bloque de código). Derivalas de doc oficial del stack."},
						"source_url":   {Type: "string", Description: "URL de la fuente de la que derivaste la skill (para el tier de confiabilidad; opcional)"},
						"save":         {Type: "boolean", Description: "Si es true, guarda la skill si pasa el gate; si es false (default) solo devuelve el reporte de calidad"},
					},
					Required: []string{"name", "rules"},
				},
			},
			handler: noCtx(s.toolAuthorSkill),
		},
		{
			Tool: Tool{
				Name:        "musubi_detect_changes",
				Description: "Inteligencia de cambios de código (model-free): corre `git diff` y, para cada archivo tocado, RE-DERIVA sus símbolos del contenido ACTUAL (go/ast para .go; escáner liviano para ts/js/py) — nunca de datos guardados, así el diff y los símbolos nunca se desalinean. Devuelve, por archivo: change_type, los símbolos afectados por los hunks, si su gist de memoria de código quedó stale (fingerprint), y qué observaciones/decisiones lo referencian. Es la forma de acotar QUÉ verificar y QUÉ decisión quedó potencialmente obsoleta tras un cambio (útil en la fase verify de SDD). Solo-lectura. ref opcional (base de comparación; default working tree vs HEAD); staged opcional (compara el índice).",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"ref":    {Type: "string", Description: "Base de comparación del diff (ej. 'HEAD~1', 'main'); opcional, default working tree vs HEAD"},
						"staged": {Type: "boolean", Description: "Si es true, compara el índice (git diff --staged); opcional"},
					},
				},
			},
			handler:  s.toolDetectChanges,
			readOnly: true,
		},
	}
}
