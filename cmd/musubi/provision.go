package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/provision"
)

// runProvision implementa `musubi provision`: lleva esta máquina a estar UNIDA al cerebro
// central (Fase 1 del track PC-provisioning). Diagnostica la red (preflight VPN-agnóstico),
// asegura Tailscale + la apertura del tailnet, cablea el .mcp.json (local + cerebro) y hace
// el self-check. Idempotente. El secreto va por ${MUSUBI_TOKEN}, nunca al archivo.
func runProvision(args []string) {
	opts := provision.Options{
		Brain:      provision.DefaultBrain,
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
	printProvisionReport(rep, opts)
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
	fmt.Println("  --brain <ip:port>    cerebro (default " + provision.DefaultBrain + ")")
	fmt.Println("  --project <dir>      proyecto cuyo .mcp.json se cablea (default: actual)")
	fmt.Println("  --token-env <VAR>    env var con el token (default MUSUBI_TOKEN)")
	fmt.Println("  --authkey <key>      auth key de Tailscale para unir sin navegador")
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
