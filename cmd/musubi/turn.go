package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// turn.go implementa el comando 'musubi turn --hook-mode': la inyección de
// contexto POR TURNO del loop de trabajo dirigido. Atado al hook UserPromptSubmit
// de Claude Code, lee el prompt del usuario desde stdin y le inyecta a Claude lo
// que Musubi ya sabe sobre lo que acaba de pedir (recall acotado, model-free) más,
// si las hay, las relaciones de memoria sin resolver. Es el cimiento del loop:
// extiende el priming de arranque (SessionStart) a cada turno de la conversación.

// turnStore abstrae lo que la inyección por turno necesita del motor de memoria.
// *memory.DbEngine lo satisface. Se inyecta para testear de forma determinista y
// para degradar con gracia si la DB no abre (store == nil → hook silencioso).
type turnStore interface {
	Recall(ctx context.Context, query string, opts memory.RecallOptions) (memory.RecallResult, error)
	PendingObsRelations() ([]memory.ObsRelation, error)
	CountSavedItems() (int, error)
	PhaseStatus() (memory.PhaseState, bool, error)
	ActiveBatch() (memory.WorkBatch, bool, error)
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error)
	LedgerStatus() (memory.TokenLedger, error)
}

// Claves de meta del loop dirigido para el recordatorio de captura.
const (
	metaLoopObsSeen       = "loop_obs_seen"           // conteo de observaciones del turno previo
	metaLoopTurns         = "loop_turns_since_save"   // turnos consecutivos sin guardar nada
	metaBudgetAlerted     = "loop_budget_alerted"     // sesión ya avisada de exceso de presupuesto
	metaBrevityInjected   = "loop_brevity_injected"   // sesión+modo ya inyectados de la directiva de brevedad
	metaPhaseInjected     = "loop_phase_injected"     // fingerprint de la fase ya inyectada (delta)
	metaConflictsInjected = "loop_conflicts_injected" // cantidad de conflictos ya avisada (delta)
	metaBatchInjected     = "loop_batch_injected"     // fingerprint del batch ya inyectado (delta)
)

// turnSurfaceChanged indica si el payload de una superficie por turno difiere de lo
// último inyectado en la sesión, y persiste el nuevo estado. La primera vez en una
// sesión —o si cambió el session_id— cuenta como cambio (el valor guardado lleva el
// session_id como prefijo, igual que el delta del recall). Es el mismo principio:
// inyectar solo lo que cambió para no repetir el mismo bloque turno a turno.
func turnSurfaceChanged(store turnStore, key, sessionID, payload string) bool {
	want := sessionID + "\x00" + payload
	if prev, ok, _ := store.GetMeta(key); ok && prev == want {
		return false
	}
	_ = store.SetMeta(key, want)
	return true
}

// turnInput es el subconjunto del JSON de stdin de UserPromptSubmit que usamos.
type turnInput struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id"`
}

// turnOutput arma el additionalContext del hook UserPromptSubmit a partir del
// prompt del usuario (leído de stdin). Combina, en orden: la fase activa del
// pipeline, la memoria relevante, los conflictos pendientes y el recordatorio de
// captura. Devuelve "" (hook silencioso) cuando no hay store, el prompt está
// vacío o ningún bloque tiene contenido.
func turnOutput(store turnStore, loopCfg config.LoopConfig, pipeCfg config.PipelineConfig, maCfg config.MultiAgentConfig, memCfg config.MemoryConfig, stdin io.Reader) string {
	if store == nil {
		return ""
	}
	in := readTurnInput(stdin)
	prompt := in.Prompt
	if prompt == "" {
		return ""
	}
	budget := memCfg.SessionTokenBudget
	brevity := memCfg.BrevityMode

	// Cada bloque se etiqueta con su superficie del ledger: assembleAccounted
	// contabiliza TODOS los no vacíos (fase, batch, recall, conflictos, captura),
	// no solo el recall como antes. Así el ledger refleja el gasto real por turno.
	var blocks []accountedBlock
	// Alerta proactiva del gobernador: si el gasto de la sesión ya cruzó el techo
	// blando, avisar UNA vez (no naggear). Va primero por prominencia.
	blocks = append(blocks, accountedBlock{"budget_alert", buildBudgetAlert(store, in.SessionID, budget)})
	// Directiva de brevedad del gobernador (T9.5): recorta tokens de SALIDA. Opt-in;
	// en "auto" solo aparece cuando el gasto ya cruzó el presupuesto, junto a la alerta.
	blocks = append(blocks, accountedBlock{"turn_brevity", buildBrevityNudge(store, in.SessionID, brevity, budget)})
	if pipeCfg.Enabled {
		blocks = append(blocks, accountedBlock{"turn_phase", buildTurnPhase(store, in.SessionID)})
	}
	if maCfg.Enabled {
		blocks = append(blocks, accountedBlock{"turn_batch", buildTurnBatch(store, in.SessionID)})
	}
	if loopCfg.PerTurnRecall {
		blocks = append(blocks, accountedBlock{"turn_recall", buildTurnRecall(store, in.SessionID, prompt, loopCfg.RecallBudget, loopCfg.DeltaInjection, memCfg)})
	}
	// SurfaceConflicts es independiente del recall: si hay relaciones sin resolver,
	// conviene avisarlas aunque el recall por turno esté apagado.
	if loopCfg.SurfaceConflicts {
		blocks = append(blocks, accountedBlock{"turn_conflicts", buildTurnConflicts(store, in.SessionID)})
	}
	if loopCfg.CaptureReminder {
		blocks = append(blocks, accountedBlock{"capture_reminder", buildCaptureReminder(store, in.SessionID, loopCfg)})
	}
	return assembleAccounted(store, "UserPromptSubmit", in.SessionID, blocks)
}

// buildBudgetAlert es la alerta PROACTIVA del gobernador (T9.3): cuando el gasto
// acumulado de la sesión cruza el presupuesto blando, inyecta UNA línea avisando —una
// sola vez por sesión, para no convertir el aviso en ruido—. budget<=0 lo desactiva.
// Lee el ledger ANTES de contabilizar este turno, así que puede atrasarse un turno
// respecto del cruce exacto; alcanza para que el aviso sea oportuno sin ser molesto.
func buildBudgetAlert(store turnStore, sessionID string, budget int) string {
	if budget <= 0 {
		return ""
	}
	l, err := store.LedgerStatus()
	if err != nil || l.Total < budget {
		return ""
	}
	if prev, ok, _ := store.GetMeta(metaBudgetAlerted); ok && prev == sessionID {
		return "" // ya avisado en esta sesión
	}
	_ = store.SetMeta(metaBudgetAlerted, sessionID)
	return fmt.Sprintf("[Musubi — presupuesto] El contexto que Musubi inyectó esta sesión (%d tokens) superó el presupuesto blando (%d). Mirá el desglose por superficie con musubi_tokens; si querés bajar el ruido, ajustá memory.session_token_budget o apagá superficies en loop/startup.", l.Total, budget)
}

// buildBrevityNudge es el escalón de SALIDA del gobernador (T9.5): inyecta UNA vez por
// sesión una directiva para que el agente responda conciso, recortando los tokens de
// RESPUESTA —complementa al resto de superficies, que solo acotan la ENTRADA—. Opt-in
// vía memory.brevity_mode: "off"/"" no hace nada; "lite"/"full"/"ultra" fijan el nivel
// siempre; "auto" solo dispara cuando el gasto de la sesión ya cruzó el presupuesto
// blando (mismo umbral que la alerta), de modo que bajo presupuesto su costo es cero.
func buildBrevityNudge(store turnStore, sessionID, mode string, budget int) string {
	if mode == "" || mode == "off" {
		return ""
	}
	if mode == "auto" {
		if budget <= 0 {
			return ""
		}
		l, err := store.LedgerStatus()
		if err != nil || l.Total < budget {
			return "" // todavía bajo presupuesto: no inyectar (costo cero)
		}
	}
	// Una sola vez por sesión y modo: la directiva persiste en contexto, no hace falta
	// repetirla turno a turno (y reinyectarla solo gastaría tokens). El estado lleva el
	// modo, así que cambiarlo a mitad de sesión vuelve a inyectar.
	want := sessionID + "\x00" + mode
	if prev, ok, _ := store.GetMeta(metaBrevityInjected); ok && prev == want {
		return ""
	}
	_ = store.SetMeta(metaBrevityInjected, want)
	return brevityDirective(mode)
}

// brevityDirective devuelve el texto de la directiva por modo. Mantiene exacto lo que no
// se puede comprimir sin romper precisión (código, rutas, versiones, flags).
func brevityDirective(mode string) string {
	const keep = "Mantené exacto el código, comandos, rutas, nombres de API, versiones y flags."
	switch mode {
	case "lite":
		return "[Musubi — brevedad] Modo conciso (lite): respondé sin relleno ni hedging, manteniendo la gramática. " + keep
	case "ultra":
		return "[Musubi — brevedad] Modo conciso (ultra): máxima compresión, abreviá, solo lo esencial. " + keep
	case "auto":
		return "[Musubi — gobernador] La sesión cruzó el presupuesto de contexto; de acá en más respondé conciso: cortá relleno y hedging y priorizá la sustancia técnica. " + keep
	default: // "full"
		return "[Musubi — brevedad] Modo conciso (full): cortá relleno, cortesías y hedging; priorizá la sustancia técnica (fragmentos OK). " + keep
	}
}

// buildTurnPhase inyecta la fase activa del pipeline y su directiva. Devuelve ""
// si no hay una tarea en curso.
func buildTurnPhase(store turnStore, sessionID string) string {
	st, ok, err := store.PhaseStatus()
	if err != nil || !ok {
		return ""
	}
	// Delta: la directiva de fase solo cambia al avanzar de fase/tarea. Re-inyectarla
	// entera cada turno es el costo que más escala en una sesión larga (medido en
	// footprint_test). Se inyecta completa solo cuando el estado de fase cambia (o
	// arranca la sesión); mientras tanto, silencio: el agente ya la tiene en contexto.
	payload := fmt.Sprintf("%s|%s|%d|%d", st.Task, st.Phase, st.Index, st.Total)
	if !turnSurfaceChanged(store, metaPhaseInjected, sessionID, payload) {
		return ""
	}
	return fmt.Sprintf("[Musubi — fase] Tarea «%s» — fase %s (%d/%d). %s",
		st.Task, st.Phase, st.Index+1, st.Total, memory.PhaseDirective(st.Phase))
}

// buildTurnBatch inyecta el estado de un batch de trabajo en curso, para que el
// agente principal recuerde monitorearlo y consolidar. Devuelve "" si no hay batch
// activo.
func buildTurnBatch(store turnStore, sessionID string) string {
	b, ok, err := store.ActiveBatch()
	if err != nil || !ok {
		return ""
	}
	// Delta: re-inyectar el estado del batch cada turno es gasto repetido (era el único
	// bloque por turno sin guard). Solo emitir cuando el progreso cambió respecto de lo ya
	// inyectado en la sesión; mientras el batch no avanza, silencio.
	fp := fmt.Sprintf("%s|%d|%d|%d|%d", b.BatchID, b.Done, b.Total, b.Open, b.Claimed)
	if !turnSurfaceChanged(store, metaBatchInjected, sessionID, fp) {
		return ""
	}
	return fmt.Sprintf("[Musubi — multi-agente] Batch activo «%s»: %d/%d unidades done (%d open, %d en curso). Monitoreá con musubi_work action=status batch=%s y consolidá los resultados cuando estén todas.",
		b.BatchID, b.Done, b.Total, b.Open, b.Claimed, b.BatchID)
}

// buildCaptureReminder cierra el loop: cuando pasaron varios turnos sin que se
// guardara nada en memoria, recuerda persistir lo aprendido. Es model-free: usa el
// conteo de items guardados en las tres superficies (observaciones + hechos + code)
// como señal de "se guardó algo" entre turnos, para no dar falsos positivos cuando lo
// guardado fue un fact o un snippet y no una observación.
func buildCaptureReminder(store turnStore, sessionID string, cfg config.LoopConfig) string {
	current, err := store.CountSavedItems()
	if err != nil {
		return ""
	}
	// Claves session-scoped: sin el prefijo de sesión, el contador de turnos-sin-guardar
	// SANGRABA entre sesiones (una sesión nueva heredaba el conteo de la anterior y podía
	// disparar el nudge sin actividad propia). El delta ya usa este mismo patrón.
	obsKey := metaLoopObsSeen + ":" + sessionID
	turnsKey := metaLoopTurns + ":" + sessionID
	prev, hasPrev := readIntMeta(store, obsKey)
	turns, _ := readIntMeta(store, turnsKey)

	// La línea base del próximo turno es el conteo de este turno.
	_ = store.SetMeta(obsKey, strconv.Itoa(current))

	// Primer turno observado: solo fijar la base, sin recordar.
	if !hasPrev {
		_ = store.SetMeta(turnsKey, "0")
		return ""
	}
	// Se guardó algo desde el turno previo: reiniciar el contador.
	if current > prev {
		_ = store.SetMeta(turnsKey, "0")
		return ""
	}

	turns++
	threshold := cfg.ReminderAfterTurns
	if threshold <= 0 {
		threshold = 5
	}
	if turns >= threshold {
		_ = store.SetMeta(turnsKey, "0") // reiniciar para no repetir cada turno
		return fmt.Sprintf("[Musubi — captura] Van %d turnos sin guardar nada. Capturá lo durable de lo que venís haciendo — una decisión (el porqué), un gotcha/aprendizaje no obvio, o el estado del trabajo — con musubi_save_observation (hechos estables → musubi_save_fact, gists → musubi_save_code). Solo lo reusable, no trivialidades.", turns)
	}
	_ = store.SetMeta(turnsKey, strconv.Itoa(turns))
	return ""
}

// readIntMeta lee una clave de meta como entero (ok=false si no existe o no parsea).
func readIntMeta(store turnStore, key string) (int, bool) {
	v, ok, err := store.GetMeta(key)
	if err != nil || !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, false
	}
	return n, true
}

// readTurnInput extrae prompt y session_id del JSON de stdin, normalizando el
// prompt. Tolera entrada inválida o vacía devolviendo un turnInput vacío.
func readTurnInput(stdin io.Reader) turnInput {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return turnInput{}
	}
	var in turnInput
	if err := json.Unmarshal(data, &in); err != nil {
		return turnInput{}
	}
	in.Prompt = strings.TrimSpace(in.Prompt)
	return in
}

// Claves de meta del estado de inyección diferencial (delta) por sesión.
const (
	metaDeltaSession  = "loop_delta_session"  // sesión a la que pertenece el estado delta
	metaDeltaInjected = "loop_delta_injected" // JSON {id -> content_hash} ya inyectado
)

// buildTurnRecall hace un recall read-only acotado al prompt y formatea los gists.
// Con deltaEnabled, inyecta SOLO la memoria nueva o modificada respecto de lo ya
// inyectado en la sesión (cache-considerate): si no hay nada nuevo, devuelve ""
// (bloque silencioso). Contabiliza en el ledger solo lo que realmente inyecta.
func buildTurnRecall(store turnStore, sessionID, prompt string, budget int, deltaEnabled bool, memCfg config.MemoryConfig) string {
	// Propagar los toggles semánticos model-free (Stemming/Cooccurrence/GraphCentrality)
	// que la tool musubi_recall ya usa: sin esto, la superficie MÁS caliente (recall por
	// turno) corría léxico puro, ignorando los puentes deploy↔despliegue / morfología /
	// centralidad que el proyecto construyó. Mismos tokens, más relevancia.
	res, err := store.Recall(context.Background(), prompt, memory.RecallOptions{
		TokenBudget:     budget,
		NoBump:          true,
		RankedFTS:       true, // filtrar stopwords: es la superficie más caliente, evita ruido
		Stemming:        memCfg.RecallStemming,
		Cooccurrence:    memCfg.RecallCooccurrence,
		GraphCentrality: memCfg.RecallGraphCentrality,
	})
	if err != nil || res.Count == 0 {
		return ""
	}

	items := res.Items
	var updated []bool

	if deltaEnabled {
		seen := loadDeltaState(store, sessionID)
		var keep []memory.RecallItem
		for _, it := range res.Items {
			prev, known := seen[it.ID]
			switch {
			case !known:
				keep = append(keep, it)
				updated = append(updated, false)
			case prev != it.ContentHash:
				keep = append(keep, it)
				updated = append(updated, true)
			}
			seen[it.ID] = it.ContentHash
		}
		saveDeltaState(store, sessionID, seen)
		items = keep
	}

	if len(items) == 0 {
		return "" // nada nuevo este turno: no re-inyectar (preserva contexto/caché)
	}

	// La contabilidad la hace assembleAccounted sobre el bloque final (header + ids
	// incluidos); acá solo se construye el bloque con la memoria nueva del turno.
	header := "[Musubi — memoria relevante] Contexto de fondo que Musubi recuerda sobre lo que pediste. La edad va en cada línea (· hace Xd/m/a): puede estar DESACTUALIZADO — verificá contra el código/estado actual antes de darlo por cierto, sobre todo lo viejo. (gists; expandí con musubi_memory_expand):"
	return formatDeltaGists(header, items, updated)
}

// metaStore es lo mínimo que necesita el estado del delta: leer/escribir meta.
// Lo satisfacen tanto turnStore (por turno) como startupStore (priming), de modo
// que el priming pueda SEMBRAR el delta y el recall por turno leerlo.
type metaStore interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
}

// loadDeltaState devuelve el conjunto {id -> content_hash} ya inyectado en la
// sesión sessionID. Si el estado pertenece a otra sesión, arranca vacío (reset).
func loadDeltaState(store metaStore, sessionID string) map[string]string {
	prevSession, _, _ := store.GetMeta(metaDeltaSession)
	if prevSession != sessionID {
		return map[string]string{}
	}
	raw, ok, _ := store.GetMeta(metaDeltaInjected)
	m := map[string]string{}
	if ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	return m
}

// saveDeltaState persiste el estado delta para la sesión.
func saveDeltaState(store metaStore, sessionID string, m map[string]string) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = store.SetMeta(metaDeltaInjected, string(data))
	_ = store.SetMeta(metaDeltaSession, sessionID)
}

// formatDeltaGists arma el bloque de gists del turno, marcando como "actualizado"
// los items cuyo content_hash cambió (updated[i] == true). updated puede ser nil.
func formatDeltaGists(header string, items []memory.RecallItem, updated []bool) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for i, it := range items {
		suffix := ""
		if i < len(updated) && updated[i] {
			suffix = " (actualizado)"
		}
		age := gistAge(it.CreatedAt)
		if it.TopicKey != "" {
			fmt.Fprintf(&b, "- (%s) %s%s%s [id:%s]\n", it.TopicKey, it.Gist, age, suffix, it.ID)
		} else {
			fmt.Fprintf(&b, "- %s%s%s [id:%s]\n", it.Gist, age, suffix, it.ID)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildTurnConflicts agrega una línea compacta cuando hay relaciones de memoria
// sin resolver, invitando a resolverlas. Devuelve "" si no hay pendientes.
func buildTurnConflicts(store turnStore, sessionID string) string {
	pending, err := store.PendingObsRelations()
	if err != nil || len(pending) == 0 {
		return ""
	}
	// Delta: avisar solo cuando la cantidad de conflictos cambia (aparecen nuevos o se
	// resuelven), no cada turno. El nudge se ve una vez por cambio en vez de volverse
	// ruido turno a turno.
	if !turnSurfaceChanged(store, metaConflictsInjected, sessionID, strconv.Itoa(len(pending))) {
		return ""
	}
	return fmt.Sprintf("[Musubi — conflictos] Hay %d relación(es) de memoria sin resolver. Revisalas con musubi_conflicts y resolvé cada una con musubi_judge.", len(pending))
}

// gistAge devuelve un sufijo compacto con la EDAD de la memoria (" · hoy", " · hace 3d", " · hace
// 5m", " · hace 2a") para que el agente vea de un vistazo si un gist es fresco o viejo y no trate
// una nota de hace meses como verdad actual. Vacío si la fecha no parsea (degradación segura).
func gistAge(createdAt string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(createdAt))
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 0:
		return ""
	case d < 24*time.Hour:
		return " · hoy"
	case d < 30*24*time.Hour:
		return fmt.Sprintf(" · hace %dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf(" · hace %dm", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf(" · hace %da", int(d.Hours()/24/365))
	}
}

// formatGists arma un bloque con un encabezado y la lista de gists de un recall.
// Compartido por el priming de arranque y la inyección por turno.
func formatGists(header string, res memory.RecallResult) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for _, it := range res.Items {
		age := gistAge(it.CreatedAt)
		if it.TopicKey != "" {
			fmt.Fprintf(&b, "- (%s) %s%s [id:%s]\n", it.TopicKey, it.Gist, age, it.ID)
		} else {
			fmt.Fprintf(&b, "- %s%s [id:%s]\n", it.Gist, age, it.ID)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// runTurn implementa el comando 'musubi turn [--hook-mode]'. Sin --hook-mode es
// un no-op (el comando solo tiene sentido como hook). En hook-mode lee stdin,
// abre la memoria (best-effort) y escribe el envelope en stdout. Los errores no
// fatales van a stderr y el proceso sale 0 para no romper la sesión.
func runTurn() {
	hookMode := false
	for _, arg := range os.Args[2:] {
		if arg == "--hook-mode" {
			hookMode = true
		}
	}
	if !hookMode {
		return
	}

	root := workspaceDir()
	cfg, _ := config.Load(root)

	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi turn: memoria no disponible: %v\n", err)
		os.Exit(0)
	}
	defer engine.Close()

	out := turnOutput(engine, cfg.Loop, cfg.Pipeline, cfg.MultiAgent, cfg.Memory, os.Stdin)
	if out != "" {
		fmt.Println(out)
	}
}
