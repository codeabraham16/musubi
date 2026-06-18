package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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
	CountObservations() (int, error)
	PhaseStatus() (memory.PhaseState, bool, error)
	ActiveBatch() (memory.WorkBatch, bool, error)
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error)
}

// Claves de meta del loop dirigido para el recordatorio de captura.
const (
	metaLoopObsSeen = "loop_obs_seen"         // conteo de observaciones del turno previo
	metaLoopTurns   = "loop_turns_since_save" // turnos consecutivos sin guardar nada
)

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
func turnOutput(store turnStore, loopCfg config.LoopConfig, pipeCfg config.PipelineConfig, maCfg config.MultiAgentConfig, stdin io.Reader) string {
	if store == nil {
		return ""
	}
	in := readTurnInput(stdin)
	prompt := in.Prompt
	if prompt == "" {
		return ""
	}

	var blocks []string
	if pipeCfg.Enabled {
		blocks = append(blocks, buildTurnPhase(store))
	}
	if maCfg.Enabled {
		blocks = append(blocks, buildTurnBatch(store))
	}
	if loopCfg.PerTurnRecall {
		blocks = append(blocks, buildTurnRecall(store, in.SessionID, prompt, loopCfg.RecallBudget, loopCfg.DeltaInjection))
	}
	// SurfaceConflicts es independiente del recall: si hay relaciones sin resolver,
	// conviene avisarlas aunque el recall por turno esté apagado.
	if loopCfg.SurfaceConflicts {
		blocks = append(blocks, buildTurnConflicts(store))
	}
	if loopCfg.CaptureReminder {
		blocks = append(blocks, buildCaptureReminder(store, loopCfg))
	}
	return assembleHookContext("UserPromptSubmit", blocks...)
}

// buildTurnPhase inyecta la fase activa del pipeline y su directiva. Devuelve ""
// si no hay una tarea en curso.
func buildTurnPhase(store turnStore) string {
	st, ok, err := store.PhaseStatus()
	if err != nil || !ok {
		return ""
	}
	return fmt.Sprintf("[Musubi — fase] Tarea «%s» — fase %s (%d/%d). %s",
		st.Task, st.Phase, st.Index+1, st.Total, memory.PhaseDirective(st.Phase))
}

// buildTurnBatch inyecta el estado de un batch de trabajo en curso, para que el
// agente principal recuerde monitorearlo y consolidar. Devuelve "" si no hay batch
// activo.
func buildTurnBatch(store turnStore) string {
	b, ok, err := store.ActiveBatch()
	if err != nil || !ok {
		return ""
	}
	return fmt.Sprintf("[Musubi — multi-agente] Batch activo «%s»: %d/%d unidades done (%d open, %d en curso). Monitoreá con musubi_work action=status batch=%s y consolidá los resultados cuando estén todas.",
		b.BatchID, b.Done, b.Total, b.Open, b.Claimed, b.BatchID)
}

// buildCaptureReminder cierra el loop: cuando pasaron varios turnos sin que se
// guardara nada en memoria, recuerda persistir lo aprendido. Es model-free: usa el
// conteo de observaciones como señal de "se guardó algo" entre turnos.
func buildCaptureReminder(store turnStore, cfg config.LoopConfig) string {
	current, err := store.CountObservations()
	if err != nil {
		return ""
	}
	prev, hasPrev := readIntMeta(store, metaLoopObsSeen)
	turns, _ := readIntMeta(store, metaLoopTurns)

	// La línea base del próximo turno es el conteo de este turno.
	_ = store.SetMeta(metaLoopObsSeen, strconv.Itoa(current))

	// Primer turno observado: solo fijar la base, sin recordar.
	if !hasPrev {
		_ = store.SetMeta(metaLoopTurns, "0")
		return ""
	}
	// Se guardó algo desde el turno previo: reiniciar el contador.
	if current > prev {
		_ = store.SetMeta(metaLoopTurns, "0")
		return ""
	}

	turns++
	threshold := cfg.ReminderAfterTurns
	if threshold <= 0 {
		threshold = 5
	}
	if turns >= threshold {
		_ = store.SetMeta(metaLoopTurns, "0") // reiniciar para no repetir cada turno
		return fmt.Sprintf("[Musubi — captura] Van %d turnos sin guardar nada en memoria. Si tomaste una decisión, arreglaste un bug o aprendiste algo no obvio, persistilo con musubi_save_observation (o musubi_save_fact).", turns)
	}
	_ = store.SetMeta(metaLoopTurns, strconv.Itoa(turns))
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
func buildTurnRecall(store turnStore, sessionID, prompt string, budget int, deltaEnabled bool) string {
	res, err := store.Recall(context.Background(), prompt, memory.RecallOptions{TokenBudget: budget, NoBump: true})
	if err != nil || res.Count == 0 {
		return ""
	}

	items := res.Items
	var updated []bool
	used := res.UsedTokens

	if deltaEnabled {
		seen := loadDeltaState(store, sessionID)
		var keep []memory.RecallItem
		used = 0
		for _, it := range res.Items {
			prev, known := seen[it.ID]
			switch {
			case !known:
				keep = append(keep, it)
				updated = append(updated, false)
				used += memory.EstimateTokens(it.Gist)
			case prev != it.ContentHash:
				keep = append(keep, it)
				updated = append(updated, true)
				used += memory.EstimateTokens(it.Gist)
			}
			seen[it.ID] = it.ContentHash
		}
		saveDeltaState(store, sessionID, seen)
		items = keep
	}

	if len(items) == 0 {
		return "" // nada nuevo este turno: no re-inyectar (preserva contexto/caché)
	}

	// Contabilizar lo inyectado (session_id resetea el ledger por sesión).
	_, _ = store.LedgerAdd(sessionID, "turn_recall", used)
	header := "[Musubi — memoria relevante] Lo que Musubi ya sabe sobre lo que pediste (gists; usá musubi_memory_expand con el id para el detalle completo):"
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
		if it.TopicKey != "" {
			fmt.Fprintf(&b, "- (%s) %s%s [id:%s]\n", it.TopicKey, it.Gist, suffix, it.ID)
		} else {
			fmt.Fprintf(&b, "- %s%s [id:%s]\n", it.Gist, suffix, it.ID)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// buildTurnConflicts agrega una línea compacta cuando hay relaciones de memoria
// sin resolver, invitando a resolverlas. Devuelve "" si no hay pendientes.
func buildTurnConflicts(store turnStore) string {
	pending, err := store.PendingObsRelations()
	if err != nil || len(pending) == 0 {
		return ""
	}
	return fmt.Sprintf("[Musubi — conflictos] Hay %d relación(es) de memoria sin resolver. Revisalas con musubi_conflicts y resolvé cada una con musubi_judge.", len(pending))
}

// formatGists arma un bloque con un encabezado y la lista de gists de un recall.
// Compartido por el priming de arranque y la inyección por turno.
func formatGists(header string, res memory.RecallResult) string {
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	for _, it := range res.Items {
		if it.TopicKey != "" {
			fmt.Fprintf(&b, "- (%s) %s [id:%s]\n", it.TopicKey, it.Gist, it.ID)
		} else {
			fmt.Fprintf(&b, "- %s [id:%s]\n", it.Gist, it.ID)
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

	out := turnOutput(engine, cfg.Loop, cfg.Pipeline, cfg.MultiAgent, os.Stdin)
	if out != "" {
		fmt.Println(out)
	}
}
