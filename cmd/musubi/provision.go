package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/mcp"
	"musubi/internal/provision"
)

// runProvision implementa `musubi provision`: lleva esta máquina a estar UNIDA al cerebro
// central (Fase 1 del track PC-provisioning). Diagnostica la red (preflight VPN-agnóstico),
// asegura Tailscale + la apertura del tailnet, cablea el .mcp.json (local + cerebro) y hace
// el self-check. Idempotente. El secreto va por ${MUSUBI_TOKEN}, nunca al archivo.
func runProvision(args []string) {
	opts := provision.Options{
		ProjectDir: ".",
		TokenEnv:   "MUSUBI_TOKEN",
	}

	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "--brain" || strings.HasPrefix(a, "--brain="):
			opts.Brain = flagValue(a, "--brain", val)
		case a == "--project" || strings.HasPrefix(a, "--project="):
			opts.ProjectDir = flagValue(a, "--project", val)
		case a == "--token-env" || strings.HasPrefix(a, "--token-env="):
			opts.TokenEnv = flagValue(a, "--token-env", val)
		case a == "--authkey" || strings.HasPrefix(a, "--authkey="):
			opts.AuthKey = flagValue(a, "--authkey", val)
		case a == "--skills":
			opts.Skills = true
		case a == "--dry-run":
			opts.DryRun = true
		case a == "--yes" || a == "-y":
			opts.Yes = true
		case a == "-h" || a == "--help":
			printProvisionUsage()
			return
		default:
			fmt.Fprintf(os.Stderr, "provision: opción desconocida %q\n", a)
			printProvisionUsage()
			os.Exit(1)
		}
	}

	// --brain es REQUERIDO: apunta a TU cerebro central. No hay default (antes apuntaba a la
	// infra del autor, lo que rompía la adopción por terceros — Track 16 F4). Para setear solo
	// este proyecto localmente, sin cerebro central, está `musubi setup`.
	if strings.TrimSpace(opts.Brain) == "" {
		fmt.Fprintln(os.Stderr, "provision: falta --brain <ip:port> — la dirección de TU cerebro central de Musubi.")
		fmt.Fprintln(os.Stderr, "  Si solo querés setear este proyecto localmente (sin cerebro central), usá 'musubi setup'.")
		printProvisionUsage()
		os.Exit(1)
	}

	if strings.TrimSpace(opts.ProjectDir) == "" {
		opts.ProjectDir = "."
	}
	if abs, err := filepath.Abs(opts.ProjectDir); err == nil {
		opts.ProjectDir = abs
	}
	exe, err := os.Executable()
	if err != nil {
		exe = "musubi"
	}

	rep, err := provision.Run(opts, provision.RealDeps(exe))
	if err != nil {
		fmt.Fprintf(os.Stderr, "provision: %v\n", err)
		os.Exit(1)
	}

	// P2: dejar el proyecto SETEADO como Musubi (workspace + skills + hooks, incl. la captura
	// automática C1/C3), reusando los helpers de setup. Best-effort: no revierte la conexión.
	if opts.DryRun {
		rep.Steps = append(rep.Steps, provision.StepResult{
			Name: "setup-local", Status: provision.StatusTodo,
			Detail: "setearía el proyecto (workspace + skills + templates + 4 hooks)",
		})
	} else {
		rep.Steps = append(rep.Steps, injectLocalSetup(opts.ProjectDir, exe)...)
	}

	// B2: el arsenal del cerebro central. Va DESPUÉS de injectLocalSetup a propósito — las
	// skills cognitivas que escribe setup son las locales del proyecto, y el arsenal no las
	// pisa (G10). Invertir el orden haría que setup sobrescribiera lo recién adoptado.
	rep.Steps = append(rep.Steps, arsenalStep(opts))

	printProvisionReport(rep, opts)
}

// arsenalStep baja el ARSENAL del cerebro central al proyecto (spec «arsenal-arranque», B2).
//
// Sin --skills NO se llama al central: sólo se deja un paso que dice cómo pedirlo. Que el paso
// informativo costara una llamada de red haría que `provision` —cuyo trabajo es unir una
// máquina— dependiera de que el arsenal esté sano.
func arsenalStep(opts provision.Options) provision.StepResult {
	if !opts.Skills {
		return provision.StepResult{
			Name: "arsenal", Status: provision.StatusTodo,
			Detail: "no se tocó; corré 'musubi provision --skills' para instalar las skills del cerebro central",
		}
	}

	rep, err := mcp.InstallArsenalInto(opts.ProjectDir, opts.DryRun)
	if err != nil {
		return provision.StepResult{Name: "arsenal", Status: provision.StatusTodo, Detail: err.Error()}
	}

	var partes []string
	if n := len(rep.Instaladas); n > 0 {
		verbo := "instaladas"
		if opts.DryRun {
			verbo = "se instalarían"
		}
		partes = append(partes, fmt.Sprintf("%s %d (%s)", verbo, n, strings.Join(rep.Instaladas, ", ")))
	}
	if n := len(rep.Salteadas); n > 0 {
		partes = append(partes, fmt.Sprintf("%d ya estaban en el proyecto (no se pisan)", n))
	}
	if n := len(rep.Fallidas); n > 0 {
		partes = append(partes, fmt.Sprintf("%d RECHAZADAS por el gate: %s", n, strings.Join(rep.Fallidas, ", ")))
	}

	switch {
	case len(rep.Fallidas) > 0:
		return provision.StepResult{Name: "arsenal", Status: provision.StatusError, Detail: strings.Join(partes, "; ")}
	case len(partes) == 0:
		return provision.StepResult{Name: "arsenal", Status: provision.StatusOK, Detail: "el arsenal del central está vacío"}
	case opts.DryRun:
		return provision.StepResult{Name: "arsenal", Status: provision.StatusTodo, Detail: strings.Join(partes, "; ")}
	case len(rep.Instaladas) == 0:
		return provision.StepResult{Name: "arsenal", Status: provision.StatusOK, Detail: strings.Join(partes, "; ")}
	default:
		return provision.StepResult{Name: "arsenal", Status: provision.StatusDone, Detail: strings.Join(partes, "; ")}
	}
}

// injectLocalSetup deja el proyecto seteado como Musubi reusando los helpers de setup.go:
// workspace, skills cognitivas, templates SDD y los 4 hooks (SessionStart/UserPromptSubmit/
// PreToolUse/Stop). Best-effort: cada paso reporta done/todo; un fallo no aborta ni revierte la
// conexión al cerebro ya lograda. Si el workspace no se puede crear, se omiten los dependientes.
func injectLocalSetup(projectDir, exePath string) []provision.StepResult {
	var steps []provision.StepResult
	add := func(name string, err error, okDetail string) {
		if err != nil {
			steps = append(steps, provision.StepResult{Name: name, Status: provision.StatusTodo, Detail: err.Error()})
			return
		}
		steps = append(steps, provision.StepResult{Name: name, Status: provision.StatusDone, Detail: okDetail})
	}

	if err := ensureWorkspace(projectDir); err != nil {
		add("workspace", err, "")
		return steps // sin workspace no tiene sentido seguir con skills/hooks
	}
	add("workspace", nil, ".musubi/ (config + memoria) listo")

	_, skillsErr := writeCognitiveSkills(projectDir)
	add("skills", skillsErr, "skills cognitivas en .musubi/skills/")
	// Y al formato que el agente lee: sin este paso el arsenal queda escrito y sin aplicar.
	_, exportErr := exportarSkillsAlAgente(projectDir)
	add("skills-agente", exportErr, "skills para el agente en "+dirSkillsAgente+"/")
	add("sdd-templates", writeSddTemplates(projectDir), "templates SDD en .musubi/templates/sdd/")
	add("hook-sessionstart", writeClaudeHook(projectDir, exePath), "SessionStart (priming + descubrimiento)")
	add("hook-turn", writeTurnHook(projectDir, exePath), "UserPromptSubmit (contexto por turno)")
	add("hook-precheck", writeCodeMemoryHook(projectDir, exePath), "PreToolUse(Read) (memoria de código)")
	add("hook-stop", writeCaptureHook(projectDir, exePath), "Stop (captura de commits)")
	return steps
}

// flagValue devuelve el valor de un flag pasado como "--x=valor" o "--x valor".
func flagValue(arg, name string, next func() string) string {
	if strings.HasPrefix(arg, name+"=") {
		return strings.TrimPrefix(arg, name+"=")
	}
	return next()
}

func printProvisionUsage() {
	fmt.Println(cBold("Uso:") + " musubi provision [opciones]")
	fmt.Println("  Une esta máquina al cerebro central de Musubi (red + .mcp.json + verificación).")
	fmt.Println()
	fmt.Println("  --brain <ip:port>    cerebro central (REQUERIDO; ip:port de TU servidor Musubi)")
	fmt.Println("  --project <dir>      proyecto cuyo .mcp.json se cablea (default: actual)")
	fmt.Println("  --token-env <VAR>    env var con el token (default MUSUBI_TOKEN)")
	fmt.Println("  --authkey <key>      auth key de Tailscale para unir sin navegador")
	fmt.Println("  --skills             instala en el proyecto el arsenal de skills del cerebro central")
	fmt.Println("  --dry-run            diagnostica y muestra el plan sin mutar nada")
}

// printProvisionReport imprime el diagnóstico de red, cada paso y el veredicto.
func printProvisionReport(rep provision.Report, opts provision.Options) {
	fmt.Println(cCyan("musubi provision — unir esta máquina al cerebro"))
	fmt.Printf("  cerebro %s · proyecto %s\n", opts.Brain, opts.ProjectDir)
	if opts.DryRun {
		fmt.Println("  " + cYellow("(dry-run: no se muta nada)"))
	}
	fmt.Println()

	fmt.Printf("%s %s\n", cBold("Red:"), rep.Mode)
	fmt.Println("  " + cDim(rep.Guidance))
	fmt.Println()

	for _, s := range rep.Steps {
		fmt.Printf("  %s %-10s %s\n", stepMark(s.Status), s.Name, cDim(s.Detail))
	}
	fmt.Println()

	switch {
	case rep.Connected:
		fmt.Println(cGreen("✅ Máquina unida al cerebro. Este proyecto ya lo usa."))
	case opts.DryRun:
		fmt.Println(cYellow("Plan mostrado. Corré sin --dry-run para aplicarlo."))
	default:
		fmt.Println(cYellow("Falta algún paso (ver arriba). Resolvé los pasos '!' y volvé a correr."))
	}
}

// stepMark traduce un estado a un símbolo coloreado.
func stepMark(status string) string {
	switch status {
	case provision.StatusOK, provision.StatusDone:
		return cGreen("✓")
	case provision.StatusTodo, provision.StatusError:
		return cYellow("!")
	default: // skipped
		return cDim("·")
	}
}
