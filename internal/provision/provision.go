package provision

import (
	"fmt"
	"os"
	"strings"
)

// Prober sondea el alcance de red desde el proceso musubi. Lo implementa netProber (real,
// probe.go) y los fakes en tests.
type Prober interface {
	PublicReachable() bool
	TailnetReachable(brain string) bool
}

// Verifier hace el self-check contra el cerebro: alcance (/readyz) y autenticación (tools/list).
type Verifier interface {
	Reach(brain string) (bool, string)
	Auth(brain, token string) (bool, string)
}

// NetworkConfigurator asegura el estado de red del cliente: Tailscale y la apertura del tailnet.
type NetworkConfigurator interface {
	// TailscaleState informa si el binario está y si el equipo está unido a la malla.
	TailscaleState() (present, joined bool)
	// EnsureTailnetAllowed aplica (idempotente) la apertura del rango del tailnet en el
	// firewall / allowlist. Con dryRun no muta: solo reporta el plan.
	EnsureTailnetAllowed(dryRun bool) StepResult
	// JoinTailscale une la máquina a la malla usando authKey (no interactivo).
	JoinTailscale(authKey string) StepResult
}

// Options son los parámetros del subcomando (ver contrato CLI en la spec).
type Options struct {
	Brain      string // ip:port del cerebro
	ProjectDir string // dir cuyo .mcp.json se cablea
	TokenEnv   string // nombre de la env var con el token
	AuthKey    string // opcional: auth key de Tailscale
	DryRun     bool
	Yes        bool
}

// Deps agrupa las dependencias inyectables (reales en cmd, fakes en tests).
type Deps struct {
	Prober
	Verifier
	NetworkConfigurator
	ExePath string // ruta del binario musubi actual (para la entrada local del .mcp.json)
}

// RealDeps arma las dependencias reales (sondas de red, verificador HTTP y configurador de
// red por-OS). Es el puente para cmd/musubi, que no puede construir los tipos no exportados.
func RealDeps(exePath string) Deps {
	return Deps{
		Prober:              netProber{},
		Verifier:            httpVerifier{},
		NetworkConfigurator: osNetwork{},
		ExePath:             exePath,
	}
}

// Estados posibles de un StepResult.
const (
	StatusOK      = "ok"      // ya estaba bien
	StatusDone    = "done"    // se hizo un cambio
	StatusSkipped = "skipped" // no aplicaba
	StatusTodo    = "todo"    // falta una acción del usuario
	StatusError   = "error"   // falló
)

// StepResult es el resultado de un paso del provisioning, para el reporte.
type StepResult struct {
	Name   string
	Status string
	Detail string
}

// Report es el resultado completo de una corrida de provision.
type Report struct {
	Mode      NetworkMode
	Guidance  string
	Steps     []StepResult
	Connected bool // ¿verificó reach + auth contra el cerebro?
}

// Run ejecuta el provisioning core (unir al cerebro), idempotente. Orquesta: preflight →
// gate por modo → Tailscale → firewall → cableado del .mcp.json → self-check → veredicto.
// No lanza error salvo fallo duro (p.ej. no poder escribir el .mcp.json); los bloqueos de
// red se reportan como pasos "todo" con guía, no como error.
func Run(opts Options, deps Deps) (Report, error) {
	var rep Report

	// 1. Preflight VPN-agnóstico (antes de mutar nada).
	reach, mode := Preflight(deps.Prober, opts.Brain)
	rep.Mode = mode
	rep.Guidance = Guidance(mode, opts.Brain)
	rep.Steps = append(rep.Steps, StepResult{
		Name:   "preflight",
		Status: map[bool]string{true: StatusOK, false: StatusTodo}[mode.ok()],
		Detail: fmt.Sprintf("modo=%s (público=%v, tailnet=%v)", mode, reach.PublicOK, reach.TailnetOK),
	})

	// Gate: si el cerebro NO es alcanzable (Tunneled/Isolated) y no es dry-run, cortar ANTES
	// de mutar y devolver la guía. En dry-run seguimos para mostrar el plan (los pasos no mutan).
	if !mode.ok() {
		rep.Steps = append(rep.Steps, StepResult{Name: "red", Status: StatusTodo, Detail: rep.Guidance})
		if !opts.DryRun {
			return rep, nil
		}
	}

	// 2. Tailscale: detectar + (instruir | unir con authkey).
	rep.Steps = append(rep.Steps, tailscaleStep(deps.NetworkConfigurator, opts))

	// 3. Apertura del tailnet en el firewall / allowlist (idempotente).
	rep.Steps = append(rep.Steps, deps.EnsureTailnetAllowed(opts.DryRun))

	// 4. Cablear el .mcp.json (local + cerebro), preservando lo existente.
	wr, err := wireMCPJSON(opts.ProjectDir, opts.Brain, opts.TokenEnv, deps.ExePath, opts.DryRun)
	if err != nil {
		return rep, err
	}
	rep.Steps = append(rep.Steps, mcpStep(wr, opts.DryRun))

	// 5. Config de sync: dejar el bloque sync: en .musubi/config.yaml para que el daemon LOCAL
	// suba solo la memoria 'shared' al cerebro (sin esto, conecta pero el auto-sync queda apagado).
	rep.Steps = append(rep.Steps, ensureSyncConfig(opts.ProjectDir, opts.Brain, opts.TokenEnv, opts.DryRun))

	// 6. Self-check reach + auth (solo si el cerebro es alcanzable y no es dry-run).
	if mode.ok() && !opts.DryRun {
		rep.Connected = selfCheck(&rep, deps.Verifier, opts)
	}

	return rep, nil
}

// tailscaleStep decide el paso de Tailscale según el estado y las opciones.
func tailscaleStep(nc NetworkConfigurator, opts Options) StepResult {
	present, joined := nc.TailscaleState()
	switch {
	case !present:
		return StepResult{Name: "tailscale", Status: StatusTodo, Detail: "Tailscale no está instalado: instalalo y unite a la malla (P3 lo automatiza)."}
	case joined:
		return StepResult{Name: "tailscale", Status: StatusOK, Detail: "instalado y unido a la malla"}
	case opts.AuthKey != "" && !opts.DryRun:
		return nc.JoinTailscale(opts.AuthKey)
	default:
		return StepResult{Name: "tailscale", Status: StatusTodo, Detail: "instalado pero sin unir: corré 'tailscale up' (o pasá --authkey para unir sin navegador)."}
	}
}

// mcpStep traduce el resultado del cableado a un StepResult.
func mcpStep(wr wireResult, dryRun bool) StepResult {
	switch {
	case dryRun && wr.changed:
		return StepResult{Name: "mcp.json", Status: StatusTodo, Detail: "cablearía musubi + musubi-cerebro en " + wr.path}
	case !wr.changed:
		return StepResult{Name: "mcp.json", Status: StatusOK, Detail: "ya cableado (sin cambios): " + wr.path}
	default:
		return StepResult{Name: "mcp.json", Status: StatusDone, Detail: "cableado musubi + musubi-cerebro en " + wr.path}
	}
}

// selfCheck corre reach + auth y agrega los pasos al reporte. Devuelve si quedó "conectado"
// (reach + auth OK). Sin token, auth se omite (no es error) y no se declara conectado.
func selfCheck(rep *Report, v Verifier, opts Options) bool {
	reachOK, reachDetail := v.Reach(opts.Brain)
	rep.Steps = append(rep.Steps, StepResult{
		Name:   "reach",
		Status: map[bool]string{true: StatusOK, false: StatusError}[reachOK],
		Detail: reachDetail,
	})
	if !reachOK {
		return false
	}

	token := strings.TrimSpace(os.Getenv(opts.TokenEnv))
	if token == "" {
		rep.Steps = append(rep.Steps, StepResult{
			Name:   "auth",
			Status: StatusSkipped,
			Detail: fmt.Sprintf("sin token en $%s: el .mcp.json usa ${%s}; seteá la env var para verificar auth.", opts.TokenEnv, opts.TokenEnv),
		})
		return false
	}

	authOK, authDetail := v.Auth(opts.Brain, token)
	rep.Steps = append(rep.Steps, StepResult{
		Name:   "auth",
		Status: map[bool]string{true: StatusOK, false: StatusError}[authOK],
		Detail: authDetail,
	})
	return authOK
}
