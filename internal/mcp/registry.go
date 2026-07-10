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
						"mem_type":   {Type: "string", Description: "Tipo de memoria opcional: 'semantic' (hechos/conocimiento estable), 'episodic' (eventos puntuales, se olvidan antes) o 'procedural' (cómo hacer algo, más durable). Modula el olvido. Vacío/desconocido = sin tipo (olvido neutro)."},
					},
					Required: []string{"topic_key", "content"},
				},
			},
			handler: s.countingSaveCtx(s.toolSaveObservation),
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
			handler: s.toolMemoryExpand,
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
			handler: s.countingSaveCtx(s.toolSaveFact),
		},
		{
			Tool: Tool{
				Name:        "musubi_recall_facts",
				Description: "Recupera HECHOS del grafo alrededor de una entidad. Devuelve tripletas compactas (no prosa), ideal para reconstruir contexto con muy pocos tokens. Por defecto devuelve sólo la VERDAD ACTUAL (los hechos invalidados por cardinalidad quedan fuera); pasá as_of para una consulta point-in-time. rank elige el ranking: por defecto BFS hasta max_hops; rank='pagerank' hace recall ASOCIATIVO (Personalized PageRank) que prioriza los hechos más relevantes multi-hop a la entidad.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"entity":   {Type: "string", Description: "Entidad desde la que recorrer el grafo"},
						"max_hops": {Type: "number", Description: "Profundidad del recorrido BFS (opcional; usa el default de la config). Ignorado con rank='pagerank'."},
						"as_of":    {Type: "string", Description: "Opcional: marca ISO para consulta point-in-time (devuelve los hechos válidos en ese instante). Inválido → verdad actual."},
						"rank":     {Type: "string", Description: "Opcional: '' o 'bfs' (default, recorrido en anchura) | 'pagerank' (recall asociativo: rankea por relevancia multi-hop a la entidad). Compone con as_of (PageRank point-in-time)."},
						"to":       {Type: "string", Description: "Opcional: si se indica, devuelve el CAMINO más corto (cadena de hechos) entre entity y esta entidad ('¿cómo se conectan?') en vez de la vecindad. Compone con as_of (camino point-in-time)."},
					},
					Required: []string{"entity"},
				},
			},
			handler:  s.toolRecallFacts,
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
			handler:  s.toolEntityContext,
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_search_semantic",
				Description: "Busca observaciones por similitud semántica. Recibe TEXTO; el servidor genera el embedding. Requiere un proveedor de embeddings configurado. Devuelve gists (titulares) por id; hidratá el contenido completo con musubi_recall o musubi_memory_expand.",
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
				Description: "Busca observaciones por texto completo (FTS5 de SQLite). Funciona siempre, sin necesidad de embeddings. Devuelve gists (titulares) por id; hidratá el contenido completo con musubi_recall o musubi_memory_expand.",
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
			handler:  s.toolConflicts,
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
				Description: "Pizarra compartida para orquestar SUB-AGENTES en paralelo (model-free). Protocolo: 1) el principal descompone y postea unidades con action=plan; 2) lanza N sub-agentes (Task tool + mcpServers:[musubi]); cada uno hace action=claim (toma una unidad atómicamente con un LEASE), la ejecuta y action=complete con su result; 3) el principal monitorea con action=status y consolida al estar todas done. El claim devuelve la unidad + fencing_token; el sub-agente DEBE renovar con action=heartbeat (id, agent, fencing_token) o la unidad se recicla al vencer el lease (at-least-once: el trabajo debe ser idempotente). action=savings estima los tokens ahorrados por delegar. CONTRACT-NET (cuando los agentes difieren en aptitud): action=bid (id, agent, bid=score MAYOR es mejor, note opcional) → el principal ve action=bids y adjudica con action=award (id) al mejor; el ganador queda claimed (con lease/fencing). action ∈ {plan, claim, heartbeat, complete, status, savings, clear, bid, award, bids}.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action":        {Type: "string", Description: "plan | claim | heartbeat | complete | status | savings | clear | bid | award | bids"},
						"batch":         {Type: "string", Description: "ID del batch (plan: opcional, se genera; claim/status/clear: el batch objetivo; claim vacío toma de cualquiera)"},
						"units":         {Type: "array", Description: "Para plan: lista de unidades [{title, spec}] a postear"},
						"agent":         {Type: "string", Description: "Para claim/heartbeat/complete: etiqueta del sub-agente (dueño del lease)"},
						"id":            {Type: "string", Description: "Para heartbeat/complete/bid/award/bids: ID de la unidad"},
						"result":        {Type: "string", Description: "Para complete: resultado/resumen producido por el sub-agente"},
						"status":        {Type: "string", Description: "Para complete: done | failed (default done)"},
						"fencing_token": {Type: "number", Description: "Para heartbeat/complete: el fencing_token que devolvió el claim/award (defiende contra un agente expropiado que revive)"},
						"bid":           {Type: "number", Description: "Para bid: score de la oferta del agente (MAYOR = mejor aptitud/confianza para la unidad)"},
						"note":          {Type: "string", Description: "Para bid: nota opcional que justifica la oferta"},
					},
				},
			},
			handler: noCtx(s.toolWork),
		},
		{
			Tool: Tool{
				Name:        "musubi_debate",
				Description: "Debate multi-agente (Society of Minds), andamiaje EJECUTABLE y DETERMINISTA model-free: Musubi NO razona — estructura las rondas, PERSISTE las posturas atribuidas y CUENTA los votos; los sub-agentes (LLM) producen posturas, críticas y votos. Protocolo: 1) action=open (topic, rounds, quorum opcional) → debate_id; 2) lanzás N sub-agentes (Task tool + mcpServers:[musubi]); cada uno postea con action=post (id, agent, stance); 3) action=advance (id) cierra la ronda y devuelve las posturas previas ('previous_postures') que pasás como material de CRÍTICA a la ronda siguiente; repetís post→advance hasta agotar rondas; 4) cada agente action=vote (id, agent, choice); 5) action=tally (id) recuenta DETERMINISTA: gana el choice con máximo ESTRICTO que alcance el quórum → cierra con ese winner; empate/bajo quórum/sin votos ⇒ no_consensus (sigue open: advance+re-votar, o deferí a musubi_judge). action=status (id) da el estado completo. El juicio SEMÁNTICO se queda en el LLM. action ∈ {open, post, advance, vote, tally, status}.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action": {Type: "string", Description: "open | post | advance | vote | tally | status"},
						"id":     {Type: "string", Description: "Para post/advance/vote/tally/status: ID del debate (lo devuelve open)"},
						"topic":  {Type: "string", Description: "Para open: la pregunta/tema del debate"},
						"rounds": {Type: "number", Description: "Para open: tope de rondas de debate (>=1)"},
						"quorum": {Type: "number", Description: "Para open (opcional): mínimo de votos que el ganador debe alcanzar (0 = sin piso, gana la mayoría estricta)"},
						"agent":  {Type: "string", Description: "Para post/vote: etiqueta del sub-agente participante"},
						"stance": {Type: "string", Description: "Para post: la postura/argumento del agente en la ronda actual (puede criticar las posturas previas)"},
						"choice": {Type: "string", Description: "Para vote: la opción por la que vota el agente (una etiqueta consensuada, ej. el nombre de una postura ganadora)"},
					},
				},
			},
			handler: noCtx(s.toolDebate),
		},
		{
			Tool: Tool{
				Name:        "musubi_workflow",
				Description: "Motor de orquestación DAG model-free: Musubi NO ejecuta los steps — define el grafo, persiste el run en SQLite (resumible entre sesiones) y devuelve qué steps están ready; VOS ejecutás y reportás. Loop: action=start (run_id + `workflow` id de .musubi/workflows/<id>.yaml, o `definition` YAML inline) → ready; ejecutás y action=complete (run_id, step, result[, status done|failed, idempotency_key]) → nuevos ready; action=next reconsulta; action=status estado completo; action=resume retoma en otra sesión. Un step queda ready cuando sus `needs` están done/skipped; puede llevar `when` (gate condicional, se salta si es falsa) y `repeat_while`+`max_iterations` (loop). Features avanzadas activadas por el YAML del step, que la RESPUESTA te va guiando cuando el run se pausa: SAGA (`compensate` → action=rollback devuelve el plan LIFO; reportás cada undo con action=compensated); HITL (`await` pausa el run en waiting_input → action=provide con input y status); verificación Reflexion (`verify`: complete deja el step en `verifying` hasta action=verify con verdict pass|fail — fail reabre para reintentar hasta max_iterations); auditoría con action=journal (traza append-only) y action=otel (export OTLP). Si un step failed bloquea todo progreso, el run pasa a `failed`; action=abort lo cierra a `aborted` (ambos aún compensables con rollback). action=validate valida sin correr; action=list lista runs. action ∈ {start, next, complete, status, resume, validate, list, journal, otel, rollback, abort, compensated, provide, verify}. Las respuestas incrementales omiten `definition` (inmutable tras start); usá status/resume para el snapshot completo.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"action":          {Type: "string", Description: "start | next | complete | status | resume | validate | list | journal | otel | rollback | abort | compensated | provide | verify"},
						"workflow":        {Type: "string", Description: "Para start: id del workflow en .musubi/workflows/<id>.yaml"},
						"definition":      {Type: "string", Description: "Para start: definición YAML inline (alternativa a 'workflow')"},
						"run_id":          {Type: "string", Description: "Identificador del run (lo elegís vos; persiste y permite resume)"},
						"step":            {Type: "string", Description: "Para complete: id del step que terminaste"},
						"result":          {Type: "string", Description: "Para complete: resultado/resumen del step"},
						"status":          {Type: "string", Description: "Para complete: done | failed (default done)"},
						"idempotency_key": {Type: "string", Description: "Para complete (opcional): clave de idempotencia; reintentar con la misma clave es un no-op seguro"},
						"input":           {Type: "string", Description: "Para provide (HITL): la decisión/dato del humano que resuelve el gate en espera"},
						"verdict":         {Type: "string", Description: "Para verify: pass (la verificación pasó → done) | fail (falló → reflexión + reintento). La reflexión va en 'result'"},
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
			handler: s.countingSaveCtx(s.toolSaveCode),
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
			handler: s.toolRecallCode,
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
			handler:  s.toolInsights,
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_sdd",
				Description: "Flujo SDD guiado (Spec-Driven Development) sobre el motor DAG, model-free. Arma el workflow canónico de un cambio —proposal→spec→design→tasks→implement→verify→archive— sin escribir YAML y te guía fase por fase (devuelve la directiva + la plantilla en .musubi/templates/sdd/). Al cerrar una fase con action=complete persiste su CONTRATO (summary + artifacts + risks + next_recommended) en memoria bajo sdd/<change>/<phase>, que las fases siguientes recuperan con musubi_recall en vez de releer archivos. Resumible entre sesiones. Protocolo: action=start (change) → fase activa; action=complete (change, phase, summary[, artifacts, risks, next_recommended]) → siguiente fase; action=next/status reconsulta. action ∈ {start, next, complete, status}.",
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
				Description: "Creación AVANZADA de skills (model-free): valida una skill contra las best-practices de Agent Skills (description disparadora en 3ª persona ≤1024 chars, name sin reservadas, triggers acotados, rules concisas con ejemplo, sin anti-patrones) y devuelve un REPORTE SCOREADO (0-100 + errores que bloquean + warnings + fixes) SIN guardar (save=false, default) para iterar; con save=true guarda SOLO si pasa el gate. Derivá las rules de FUENTES CONFIABLES (doc oficial + repos reputados: anthropics/skills, awesome-cursorrules); pasá source_url y su tier (official>curated>community). Usalo antes de musubi_save_skill.",
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
		{
			Tool: Tool{
				Name:        "musubi_promote",
				Description: "Promueve una observación LOCAL (privada del proyecto) a SHARED, marcándola como candidata a la memoria central del cerebro híbrido. Es idempotente: promover una ya compartida es un no-op exitoso. Requiere el id de la observación (el que devuelve musubi_save_observation o un recall). No sincroniza nada por sí solo: sólo cambia el scope.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {Type: "string", Description: "ID de la observación a promover a 'shared'"},
					},
					Required: []string{"id"},
				},
			},
			handler: noCtx(s.countingSave(s.toolPromote)),
		},
		{
			Tool: Tool{
				Name:        "musubi_sync_status",
				Description: "Salud del sync saliente del cerebro híbrido (F2): cuántas observaciones 'shared' están pendientes de enviar al cerebro central, cuántas ya se enviaron, cuántas quedaron en dead-letter, la antigüedad de la más vieja pendiente y el último error. Read-only, sin parámetros.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			handler:  noCtx(s.toolSyncStatus),
			readOnly: true,
		},
		{
			Tool: Tool{
				Name:        "musubi_sync_requeue",
				Description: "Reintenta el envío de las observaciones que quedaron en dead-letter del sync saliente (F2): las devuelve a la cola de envío al cerebro central. Útil tras un corte del central o de la VPN. Sin parámetros; devuelve cuántas re-encoló.",
				InputSchema: InputSchema{
					Type:       "object",
					Properties: map[string]Property{},
				},
			},
			handler: noCtx(s.toolSyncRequeue),
		},
	}
}
