package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/config"
	"musubi/internal/detector"
)

// detectOutput implementa la lógica central del comando 'musubi detect'.
// Es una función pura-ish (sin side effects sobre stdout) que facilita los tests.
//
// Modos:
//   - hookMode=false: devuelve el JSON indentado del slice de StackResult.
//   - hookMode=true:
//   - Si el sentinel existe → devuelve "" (operación silenciosa idempotente).
//   - Si el sentinel no existe → devuelve el JSON hookSpecificOutput con
//     instrucciones para que Claude detecte el stack y guarde las skills.
func detectOutput(root string, hookMode bool) (string, error) {
	skillsDir := filepath.Join(root, config.DirName, config.SkillsDir)
	sentinelPath := filepath.Join(skillsDir, config.SentinelFile)

	if hookMode {
		// Verificar presencia del sentinel.
		if _, err := os.Stat(sentinelPath); err == nil {
			// Sentinel presente → no-op silencioso.
			return "", nil
		}

		// Sentinel ausente: emitir el JSON de hookSpecificOutput con instrucciones.
		// Intentar detectar el stack para incluirlo en el contexto (best-effort).
		resultados, _ := detector.DetectStack(root)
		stackResumen := resumirStack(resultados)

		// Construir el texto de additionalContext (en español, por convención).
		contexto := buildAdditionalContext(stackResumen)

		envelope := map[string]interface{}{
			"hookSpecificOutput": map[string]interface{}{
				"hookEventName":     "SessionStart",
				"additionalContext": contexto,
			},
		}
		datos, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return "", fmt.Errorf("error al serializar hookSpecificOutput: %w", err)
		}
		return string(datos), nil
	}

	// Modo normal: JSON del slice de StackResult.
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
			resumen += " ("
			for j, f := range r.Frameworks {
				if j > 0 {
					resumen += ", "
				}
				resumen += f
			}
			resumen += ")"
		}
	}
	return resumen
}

// buildAdditionalContext construye el texto de instrucciones que Claude Code
// recibirá al inicio de la sesión cuando el sentinel no está presente.
// Las instrucciones deben llevar a Claude a:
//  1. Llamar musubi_detect_stack para obtener el stack del proyecto.
//  2. Investigar documentación OFICIAL del stack detectado.
//  3. Confirmar las reglas propuestas con el usuario.
//  4. Guardar cada skill aprobada con musubi_save_skill (que crea el sentinel).
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
