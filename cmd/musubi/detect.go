package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/memory"
)

// startupStore abstrae lo que el hook necesita del motor de memoria: leer/guardar
// la huella del stack (meta) y traer el contexto de priming. *memory.DbEngine lo
// satisface. Se inyecta para poder testear el hook de forma determinista y para
// degradar con gracia si la DB no abre (store == nil).
type startupStore interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	PrimeContext(budget int) (memory.RecallResult, error)
	TopicExists(topicKey string) (bool, error)
}

// detectOutput implementa la lógica central del comando 'musubi detect'.
//
// Modos:
//   - hookMode=false: devuelve el JSON indentado del slice de StackResult.
//   - hookMode=true: abre la memoria (best-effort), carga config y delega en
//     buildHookOutput, que decide qué generación de skills hace falta (completa,
//     incremental por delta de stack, o ninguna) e inyecta el priming de memoria.
func detectOutput(root string, hookMode bool) (string, error) {
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
	cfg, _ := config.Load(root)
	var store startupStore
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi detect: memoria no disponible para el arranque: %v\n", err)
	} else {
		defer engine.Close()
		store = engine
	}
	return buildHookOutput(root, store, cfg.Startup)
}

// buildHookOutput arma el additionalContext del SessionStart combinando dos
// partes (cualquiera puede estar vacía):
//  1. Priming de memoria: contexto que Musubi "recuerda" del proyecto.
//  2. Generación de skills: instrucciones para que el agente genere skills, ya
//     sea completa (primera vez) o incremental (delta del stack).
//
// Si ambas partes quedan vacías, devuelve "" (hook silencioso e idempotente).
func buildHookOutput(root string, store startupStore, cfg config.StartupConfig) (string, error) {
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
		priming = buildPrimingContext(store, cfg.RecallBudget)
	}
	cognitive := ""
	if store != nil && cfg.CognitiveBootstrap && bootstrappingAutoconocimiento(store) {
		cognitive = buildCognitiveContext()
	}

	return assembleHookContext("SessionStart", priming, cognitive, generation), nil
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
func buildPrimingContext(store startupStore, budget int) string {
	res, err := store.PrimeContext(budget)
	if err != nil || res.Count == 0 {
		return ""
	}
	header := "[Musubi — memoria] Contexto que Musubi recuerda de este proyecto (gists; usá musubi_memory_expand con el id para el detalle completo):"
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

	out, err := detectOutput(root, hookMode)
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
