package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/memory"
)

// readSessionID extrae session_id del JSON del evento de hook (stdin). Tolera
// entrada vacía o inválida devolviendo "".
func readSessionID(stdin io.Reader) string {
	var in struct {
		SessionID string `json:"session_id"`
	}
	_ = json.NewDecoder(stdin).Decode(&in)
	return strings.TrimSpace(in.SessionID)
}

// startupStore abstrae lo que el hook necesita del motor de memoria: leer/guardar
// la huella del stack (meta) y traer el contexto de priming. *memory.DbEngine lo
// satisface. Se inyecta para poder testear el hook de forma determinista y para
// degradar con gracia si la DB no abre (store == nil).
type startupStore interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	PrimeContext(budget int) (memory.RecallResult, error)
	TopicExists(topicKey string) (bool, error)
	LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error)
}

// detectOutput implementa la lógica central del comando 'musubi detect'.
//
// Modos:
//   - hookMode=false: devuelve el JSON indentado del slice de StackResult.
//   - hookMode=true: abre la memoria (best-effort), carga config y delega en
//     buildHookOutput, que decide qué generación de skills hace falta (completa,
//     incremental por delta de stack, o ninguna) e inyecta el priming de memoria.
func detectOutput(root string, hookMode bool, sessionID string) (string, error) {
	if !hookMode {
		resultados, err := detector.DetectStack(root)
		if err != nil {
			return "", fmt.Errorf("error al detectar stack: %w", err)
		}
		datos, err := json.MarshalIndent(resultados, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error al serializar resultados: %w", err)
		}
		return string(datos), nil
	}

	// Modo hook: cargar config y abrir memoria (best-effort; si falla, store nil
	// y se cae al flujo basado solo en el sentinel).
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi detect: config no disponible, usando defaults: %v\n", err)
		cfg = config.Default()
	}
	var store startupStore
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi detect: memoria no disponible para el arranque: %v\n", err)
	} else {
		defer engine.Close()
		store = engine
	}
	return buildHookOutput(root, store, cfg.Startup, sessionID)
}

// buildHookOutput arma el additionalContext del SessionStart combinando dos
// partes (cualquiera puede estar vacía):
//  1. Priming de memoria: contexto que Musubi "recuerda" del proyecto.
//  2. Generación de skills: instrucciones para que el agente genere skills, ya
//     sea completa (primera vez) o incremental (delta del stack).
//
// Si ambas partes quedan vacías, devuelve "" (hook silencioso e idempotente).
func buildHookOutput(root string, store startupStore, cfg config.StartupConfig, sessionID string) (string, error) {
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	sentinelPath := filepath.Join(skillsDir, config.SentinelFile)
	_, sentinelErr := os.Stat(sentinelPath)
	sentinelExists := sentinelErr == nil

	current, _ := detector.DetectStack(root)

	// SessionStart (arranque o compactación) = contexto fresco: limpiar el estado
	// de inyección diferencial para que la memoria relevante se re-inyecte por turno.
	if store != nil {
		_ = store.SetMeta(metaDeltaInjected, "")
		_ = store.SetMeta(metaDeltaSession, "")
	}

	generation := decideGeneration(root, store, cfg, current, sentinelExists)
	priming := ""
	if store != nil && cfg.PrimeMemory {
		priming = buildPrimingContext(store, cfg.RecallBudget, sessionID)
	}
	cognitive := ""
	if store != nil && cfg.CognitiveBootstrap && bootstrappingAutoconocimiento(store) {
		cognitive = buildCognitiveContext()
	}
	health := buildHealthContext(store)

	// El bloque de captura proactiva viaja SOLO si el hook ya tiene algo que decir: no debe
	// romper el "hook silencioso cuando no hay nada" (la brevedad es sagrada; ver los tests de
	// silencio). El recordatorio por turno cubre la captura en sesiones sin priming.
	capture := ""
	if store != nil && (priming != "" || cognitive != "" || health != "" || generation != "") {
		capture = buildCapturePriming()
	}

	// Contabilizar TODAS las superficies de arranque (no solo el priming): el bloque
	// cognitivo y el de generación de skills son los que más tokens inyectan y antes
	// eran invisibles en el ledger. Medirlos es el cimiento para optimizarlos.
	return assembleAccounted(store, "SessionStart", sessionID, []accountedBlock{
		{surface: "startup_priming", text: priming},
		{surface: "startup_capture", text: capture},
		{surface: "startup_health", text: health},
		{surface: "startup_cognitive", text: cognitive},
		{surface: "startup_skillgen", text: generation},
	}), nil
}

// buildCapturePriming instruye al agente a CAPTURAR SOLO (sin que el usuario se lo pida) los
// aprendizajes durables mientras trabaja — la mitad de escritura del cerebro, que hoy depende
// de que el agente se acuerde. La extracción la hace el agente (que es el LLM), como el
// auto-memory de Claude Code: Musubi solo lo dirige. Es la Fase 1 (captura proactiva) del track
// de captura automática. Texto estático y conciso (se contabiliza en el ledger como una
// superficie más del arranque). La captura es LOCAL (scope default); compartir al cerebro es una
// fase posterior detrás de la redacción de secretos.
func buildCapturePriming() string {
	return `[Musubi — captura proactiva] Mientras trabajás, guardá VOS SOLO (sin que te lo pidan) lo que valga la pena recordar, con las tools de Musubi:
- Decisiones (el porqué de lo elegido) y gotchas/aprendizajes no obvios → musubi_save_observation
- Hechos estables (convenciones, versiones, rutas, comandos) → musubi_save_fact ; gists de archivos que leíste → musubi_save_code
- Estado del trabajo (qué se hizo, qué falta) al cerrar un avance → musubi_save_observation
Guardá solo lo REUSABLE / NO OBVIO; nada de trivialidades (saludos, "ok", estado transitorio). No dupliques: recuperá antes con musubi_recall.
Aclaración: "shared" en Musubi = scope shared de la MEMORIA (que la vean otras máquinas/el equipo), NO un tag ni un commit de git.`
}

// ledgerStore es lo mínimo que la contabilidad holística necesita del motor:
// imputar tokens a una superficie de la sesión. Lo satisfacen tanto startupStore
// (arranque) como turnStore (por turno), de modo que el mismo helper de ensamblado
// sirva a ambos hooks.
type ledgerStore interface {
	LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error)
}

// accountedBlock asocia un bloque de contexto inyectado con la superficie del
// ledger a la que se imputa su costo en tokens.
type accountedBlock struct {
	surface string
	text    string
}

// assembleAccounted contabiliza en el ledger CADA bloque no vacío —estimando los
// tokens de su texto FINAL (headers, ids y formato incluidos, que es la huella real
// que entra al contexto)— y luego ensambla los no vacíos en el envelope del evento.
// Centralizar la contabilidad acá garantiza que ninguna superficie inyectada quede
// sin medir: antes cada builder contabilizaba (o no) por su cuenta y la mayoría no
// lo hacía, así que el ledger sub-reportaba el gasto real de Musubi. Si store es nil
// (memoria no disponible), solo ensambla sin contabilizar.
func assembleAccounted(store ledgerStore, eventName, sessionID string, blocks []accountedBlock) string {
	texts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b.text) == "" {
			continue
		}
		if store != nil {
			_, _ = store.LedgerAdd(sessionID, b.surface, memory.EstimateTokens(b.text))
		}
		texts = append(texts, b.text)
	}
	return assembleHookContext(eventName, texts...)
}

// buildHealthContext surfacea (T5.4) los problemas que la auto-curación NO pudo reparar
// sola, leyendo el último DiagnoseReport persistido por AutoHeal (MetaLastHealth). Si la
// base está sana o no hay reporte, devuelve "" (silencioso). Es una lectura barata de
// meta: no re-diagnostica.
func buildHealthContext(store startupStore) string {
	if store == nil {
		return ""
	}
	raw, ok, err := store.GetMeta(memory.MetaLastHealth)
	if err != nil || !ok || strings.TrimSpace(raw) == "" {
		return ""
	}
	var rep memory.DiagnoseReport
	if jerr := json.Unmarshal([]byte(raw), &rep); jerr != nil {
		return ""
	}
	if rep.Status == "ok" {
		return ""
	}
	var problemas []string
	for _, c := range rep.Checks {
		if c.Status != "ok" {
			problemas = append(problemas, fmt.Sprintf("%s — %s", c.Code, c.Message))
		}
	}
	if len(problemas) == 0 {
		return ""
	}
	return "[Musubi — salud] El auto-mantenimiento detectó problemas en la base de memoria que no se auto-reparan:\n- " +
		strings.Join(problemas, "\n- ") +
		"\nCorré musubi_doctor para el detalle; los reparables se arreglan con musubi_doctor {check, repair:true, mode:\"apply\"}."
}

// bootstrappingAutoconocimiento indica si el proyecto AÚN no tiene perfil: en ese
// caso el hook inyecta el bloque cognitivo con fuerza. Una vez que existe el
// perfil (project/profile), deja de inyectarlo y se apoya en el priming.
func bootstrappingAutoconocimiento(store startupStore) bool {
	profiled, err := store.TopicExists(profileTopicKey)
	if err != nil {
		return false // ante la duda, no molestar
	}
	return !profiled
}

// decideGeneration devuelve el bloque de instrucciones de generación de skills, o
// "" si no corresponde generar nada. Maneja: primera vez (completa), delta de
// stack (incremental), migración de proyectos viejos (backfill sin re-generar) y
// el fallback sin DB (comportamiento histórico basado en el sentinel).
func decideGeneration(root string, store startupStore, cfg config.StartupConfig, current []detector.StackResult, sentinelExists bool) string {
	currentFP := detector.StackFingerprint(current)
	stackResumen := resumirStack(current)

	// Sin DB: comportamiento histórico (sentinel presente → nada; ausente → full).
	if store == nil {
		if sentinelExists {
			return ""
		}
		return buildAdditionalContext(stackResumen)
	}

	storedFP, hasFP, _ := store.GetMeta(memory.MetaStackFingerprint)

	// Primera vez: nunca se generó (ni sentinel ni huella) → generación completa.
	if !sentinelExists && !hasFP {
		return buildAdditionalContext(stackResumen)
	}

	// Migración: proyecto viejo con sentinel pero sin huella → backfillear la
	// huella actual y no molestar (asumimos que las skills cubren el stack actual).
	if sentinelExists && !hasFP {
		_ = store.SetMeta(memory.MetaStackFingerprint, currentFP)
		return ""
	}

	// Ya hay huella: detectar drift del stack.
	delta := detector.StackDelta(storedFP, current)
	if len(delta) > 0 && cfg.AutoRegen {
		return buildDeltaContext(delta)
	}
	return ""
}

// resumirStack devuelve un resumen legible del stack detectado.
func resumirStack(resultados []detector.StackResult) string {
	if len(resultados) == 0 {
		return "stack desconocido"
	}
	resumen := ""
	for i, r := range resultados {
		if i > 0 {
			resumen += ", "
		}
		resumen += r.Ecosystem
		if len(r.Frameworks) > 0 {
			resumen += " (" + strings.Join(r.Frameworks, ", ") + ")"
		}
	}
	return resumen
}

// assembleHookContext combina los bloques no vacíos (en orden) en el envelope
// JSON de hookSpecificOutput para el evento eventName (ej. "SessionStart" o
// "UserPromptSubmit"). Devuelve "" si no hay nada que inyectar.
func assembleHookContext(eventName string, bloques ...string) string {
	var partes []string
	for _, b := range bloques {
		if strings.TrimSpace(b) != "" {
			partes = append(partes, b)
		}
	}
	if len(partes) == 0 {
		return ""
	}
	contexto := strings.Join(partes, "\n\n")

	envelope := map[string]interface{}{
		"hookSpecificOutput": map[string]interface{}{
			"hookEventName":     eventName,
			"additionalContext": contexto,
		},
	}
	datos, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi detect: error al serializar hookSpecificOutput: %v\n", err)
		return ""
	}
	return string(datos)
}

// buildPrimingContext arma el bloque de "memoria recordada" del proyecto a partir
// de un recall por presupuesto de tokens. Devuelve "" si no hay memoria.
func buildPrimingContext(store startupStore, budget int, sessionID string) string {
	res, err := store.PrimeContext(budget)
	if err != nil || res.Count == 0 {
		return ""
	}
	// La contabilidad del priming la hace assembleAccounted sobre el texto final
	// (header incluido); acá solo construimos el bloque y sembramos el delta.

	// SEMBRAR el delta con lo que el priming ya inyectó: así el recall por turno
	// no repite estos mismos gists en la sesión (evita la doble inyección
	// priming↔turno). El estado se reinicia por sesión, igual que el delta.
	seed := make(map[string]string, len(res.Items))
	for _, it := range res.Items {
		seed[it.ID] = it.ContentHash
	}
	saveDeltaState(store, sessionID, seed)

	header := "[Musubi — memoria] Contexto de fondo que Musubi recuerda de este proyecto. La edad va en cada línea (· hace Xd/m/a): puede estar DESACTUALIZADO — verificá contra el código/estado actual antes de darlo por cierto, sobre todo lo viejo. (gists; expandí con musubi_memory_expand):"
	return formatGists(header, res)
}

// buildCognitiveContext arma el bloque de autoconocimiento que activa las skills
// cognitivas: que el agente analice el proyecto, deduzca convenciones, planee y
// arme el PERFIL. Se inyecta mientras el proyecto no tenga perfil; una vez creado,
// el priming lo lleva barato y este bloque deja de aparecer.
func buildCognitiveContext() string {
	return fmt.Sprintf(`[Musubi — autoconocimiento] Este proyecto todavía no tiene perfil. ANTES de avanzar con lo que pida el usuario, dedicá un momento a conocerlo usando las skills cognitivas de .musubi/skills/:

- analyze-project: mapeá estructura, manifests y entrypoints (usá musubi_detect_stack) y capturá hallazgos no obvios con musubi_save_observation.
- deduce-conventions: deducí convenciones (naming, tests, manejo de errores) del código existente y guardalas como hechos con musubi_save_fact.
- plan-ahead: recuperá contexto con musubi_recall / musubi_recall_facts antes de actuar.
- project-profile: consolidá un perfil conciso del proyecto con musubi_save_observation usando el topic_key exacto '%s' (propósito, stack, arquitectura, convenciones, decisiones).

Cuando el perfil exista, Musubi te lo recordará automáticamente al arrancar y este paso dejará de aparecer. No dupliques lo que ya esté en memoria: recuperá primero.`, profileTopicKey)
}

// buildAdditionalContext construye las instrucciones de generación COMPLETA de
// skills (primera vez en el proyecto).
func buildAdditionalContext(stackResumen string) string {
	return fmt.Sprintf(`[Musubi — auto-descubrimiento de skills] Stack detectado: %s.

Por favor realizá los siguientes pasos ANTES de responder al usuario:

1. Llamar a la herramienta musubi_detect_stack para obtener el análisis completo del stack del proyecto (ecosistemas, frameworks, manifests).

1.5. Llamar a musubi_search_skills (sin parámetros) para obtener candidatos del catálogo ya filtrados por relevancia técnica (triggers, deps y capabilities del proyecto). Tu trabajo es evaluar VALOR, no relevancia. Ordená los candidatos por valor para este proyecto, descartá los que sean redundantes con skills existentes. Podés complementar con búsqueda web solo para llenar gaps. Fetchá rules_url únicamente de las skills que vayas a guardar. Opcionalmente registrá decisiones con musubi_log_skill_decision.

2. Investigar la documentación OFICIAL del stack detectado:
   - Go → pkg.go.dev
   - Node.js / React / Next.js → react.dev, nextjs.org
   - Python → docs.python.org, fastapi.tiangolo.com, docs.djangoproject.com
   - Rust → doc.rust-lang.org
   - Preferir SIEMPRE documentación oficial sobre blogs y foros.

3. Sintetizar entre 2 y 5 reglas concretas y prácticas para el proyecto, usando la documentación oficial como fuente. Incluir la URL fuente como comentario en el campo "rules".

4. CONFIRMAR con el usuario las reglas propuestas ANTES de guardar. Mostrar un borrador y esperar aprobación.

5. Por cada skill aprobada por el usuario, llamar a musubi_save_skill con los campos name, triggers, rules y description. Esto creará el archivo .musubi/skills/{name}.yaml y el sentinel que evita repetir este flujo en sesiones futuras.

IMPORTANTE: No omitir el paso de confirmación con el usuario. Las skills son persistentes y afectan el comportamiento del agente en todas las sesiones futuras.`, stackResumen)
}

// buildDeltaContext construye las instrucciones de generación INCREMENTAL cuando
// el stack creció respecto de la huella guardada (ej. se agregó React).
func buildDeltaContext(delta []string) string {
	nuevo := strings.Join(delta, ", ")
	return fmt.Sprintf(`[Musubi — stack actualizado] Detecté que el stack del proyecto creció desde la última vez que generamos skills. Novedades: %s.

Por favor, ANTES de responder al usuario, generá skills SOLO para lo nuevo:

1. Llamar a musubi_search_skills para obtener candidatos del catálogo relevantes a las novedades del stack.

2. Investigar la documentación OFICIAL de lo nuevo y sintetizar entre 1 y 3 reglas concretas para esa parte del stack.

3. CONFIRMAR con el usuario las reglas propuestas ANTES de guardar.

4. Por cada skill aprobada, llamar a musubi_save_skill (esto actualiza la huella del stack y evita volver a pedir esto). No re-generes skills que ya existan para el resto del stack.`, nuevo)
}

// runDetect implementa el comando 'musubi detect [--hook-mode]'.
// Lee el root del workspace y llama a detectOutput; imprime en stdout y sale.
// En hook-mode, la salida va al canal de hooks de Claude Code (stdout del proceso).
// Los errores no fatales se loguean a stderr para no contaminar el canal de hooks.
func runDetect() {
	root := workspaceDir()

	hookMode := false
	for _, arg := range os.Args[2:] {
		if arg == "--hook-mode" {
			hookMode = true
		}
	}

	// En hook-mode, Claude Code envía el JSON del evento por stdin; extraemos el
	// session_id para contabilizar el ledger por sesión. Tolera ausencia (== "").
	sessionID := ""
	if hookMode {
		sessionID = readSessionID(os.Stdin)
	}

	out, err := detectOutput(root, hookMode, sessionID)
	if err != nil {
		// En hook-mode, un error no debe romper la sesión: loguear a stderr y salir 0.
		fmt.Fprintf(os.Stderr, "musubi detect: %v\n", err)
		if hookMode {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if out != "" {
		fmt.Println(out)
	}
}
