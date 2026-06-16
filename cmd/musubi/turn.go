package main

import (
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
	Recall(query string, opts memory.RecallOptions) (memory.RecallResult, error)
	PendingObsRelations() ([]memory.ObsRelation, error)
	CountObservations() (int, error)
	PhaseStatus() (memory.PhaseState, bool, error)
	ActiveBatch() (memory.WorkBatch, bool, error)
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
}

// Claves de meta del loop dirigido para el recordatorio de captura.
const (
	metaLoopObsSeen = "loop_obs_seen"         // conteo de observaciones del turno previo
	metaLoopTurns   = "loop_turns_since_save" // turnos consecutivos sin guardar nada
)

// turnInput es el subconjunto del JSON de stdin de UserPromptSubmit que usamos.
type turnInput struct {
	Prompt string `json:"prompt"`
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
	prompt := readPrompt(stdin)
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
		blocks = append(blocks, buildTurnRecall(store, prompt, loopCfg.RecallBudget))
		if loopCfg.SurfaceConflicts {
			blocks = append(blocks, buildTurnConflicts(store))
		}
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

// readPrompt extrae y normaliza el prompt del JSON de stdin. Tolera entrada
// inválida o vacía devolviendo "".
func readPrompt(stdin io.Reader) string {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return ""
	}
	var in turnInput
	if err := json.Unmarshal(data, &in); err != nil {
		return ""
	}
	return strings.TrimSpace(in.Prompt)
}

// buildTurnRecall hace un recall read-only acotado al prompt y formatea los gists.
// Devuelve "" si no hay memoria relevante.
func buildTurnRecall(store turnStore, prompt string, budget int) string {
	res, err := store.Recall(prompt, memory.RecallOptions{TokenBudget: budget, NoBump: true})
	if err != nil || res.Count == 0 {
		return ""
	}
	header := "[Musubi — memoria relevante] Lo que Musubi ya sabe sobre lo que pediste (gists; usá musubi_memory_expand con el id para el detalle completo):"
	return formatGists(header, res)
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
