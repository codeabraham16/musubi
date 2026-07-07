package provision

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// osNetwork es el NetworkConfigurator real: detecta Tailscale y aplica la apertura del tailnet
// (firewall en Windows / allowlist de subred en Linux). Shellea a las herramientas del sistema;
// por eso NO se testea en unit (se cubre con fakes + la prueba manual en las 2 máquinas).
type osNetwork struct{}

// TailscaleState informa si el binario está y si el equipo está unido a la malla
// (`tailscale status` retorna 0 cuando está unido y online).
func (osNetwork) TailscaleState() (present, joined bool) {
	path := tailscalePath()
	if path == "" {
		return false, false
	}
	return true, exec.Command(path, "status").Run() == nil
}

// JoinTailscale une la máquina a la malla con authKey (no interactivo).
func (osNetwork) JoinTailscale(authKey string) StepResult {
	path := tailscalePath()
	if path == "" {
		return StepResult{Name: "tailscale", Status: StatusTodo, Detail: "Tailscale no está instalado."}
	}
	if err := exec.Command(path, "up", "--authkey", authKey).Run(); err != nil {
		return StepResult{Name: "tailscale", Status: StatusTodo, Detail: "no se pudo unir con --authkey: " + err.Error()}
	}
	return StepResult{Name: "tailscale", Status: StatusDone, Detail: "unido a la malla con authkey"}
}

// EnsureTailnetAllowed abre el rango del tailnet según el SO (idempotente).
func (osNetwork) EnsureTailnetAllowed(dryRun bool) StepResult {
	switch runtime.GOOS {
	case "windows":
		return ensureFirewallWindows(dryRun)
	case "linux":
		return ensureAllowlistLinux(dryRun)
	default:
		return StepResult{Name: "firewall", Status: StatusSkipped, Detail: "apertura del tailnet no implementada en " + runtime.GOOS}
	}
}

func tailscalePath() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if p := `C:\Program Files\Tailscale\tailscale.exe`; fileExists(p) {
			return p
		}
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// firewallRuleNames son las reglas idempotentes que permiten el tailnet pese a una VPN cuyo
// filtro WFP bloquea el tráfico directo (el fix probado en kernelos-pc, PR #120).
var firewallRuleNames = []string{"TS-Allow-Tailnet-Out", "TS-Allow-Tailnet-In"}

func ensureFirewallWindows(dryRun bool) StepResult {
	present := 0
	for _, n := range firewallRuleNames {
		if psRuleExists(n) {
			present++
		}
	}
	if present == len(firewallRuleNames) {
		return StepResult{Name: "firewall", Status: StatusOK, Detail: "reglas TS-Allow-Tailnet-In/Out presentes (" + tailnetCIDR + ")"}
	}
	if dryRun {
		return StepResult{Name: "firewall", Status: StatusTodo, Detail: "crearía reglas TS-Allow-Tailnet-In/Out para " + tailnetCIDR}
	}
	script := `foreach ($r in @(@{Name='TS-Allow-Tailnet-Out';Dir='Outbound'},@{Name='TS-Allow-Tailnet-In';Dir='Inbound'})) {
  Get-NetFirewallRule -DisplayName $r.Name -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue
  New-NetFirewallRule -DisplayName $r.Name -Direction $r.Dir -Action Allow -RemoteAddress 100.64.0.0/10 -Profile Any -Enabled True | Out-Null
}`
	if out, err := runPowerShell(script); err != nil {
		detail := "no se pudieron crear las reglas (¿consola sin admin?): abrí PowerShell como administrador y reintentá."
		if s := strings.TrimSpace(out); s != "" {
			detail += " " + s
		}
		return StepResult{Name: "firewall", Status: StatusTodo, Detail: detail}
	}
	return StepResult{Name: "firewall", Status: StatusDone, Detail: "reglas TS-Allow-Tailnet-In/Out creadas para " + tailnetCIDR}
}

func ensureAllowlistLinux(dryRun bool) StepResult {
	// VPN-agnóstico: solo si hay una CLI conocida que soporte allowlist por subred. En Linux
	// la exclusión suele ser a nivel sistema, así que la ausencia de CLI NO es un problema.
	if _, err := exec.LookPath("nordvpn"); err != nil {
		return StepResult{Name: "allowlist", Status: StatusSkipped, Detail: "sin VPN CLI con allowlist por subred; en Linux la exclusión suele ser system-wide"}
	}
	if dryRun {
		return StepResult{Name: "allowlist", Status: StatusTodo, Detail: "agregaría la subred " + tailnetCIDR + " al allowlist de la VPN"}
	}
	for _, verb := range []string{"allowlist", "whitelist"} { // versiones nuevas vs viejas
		if exec.Command("nordvpn", verb, "add", "subnet", tailnetCIDR).Run() == nil {
			return StepResult{Name: "allowlist", Status: StatusDone, Detail: "subred " + tailnetCIDR + " agregada al " + verb + " de la VPN"}
		}
	}
	return StepResult{Name: "allowlist", Status: StatusTodo, Detail: "no se pudo agregar la subred al allowlist; agregala a mano si el tailnet no responde"}
}

// psRuleExists consulta si una regla de firewall por DisplayName existe (Windows).
func psRuleExists(name string) bool {
	out, err := runPowerShell("if (Get-NetFirewallRule -DisplayName '" + name + "' -ErrorAction SilentlyContinue) { 'yes' } else { 'no' }")
	return err == nil && strings.Contains(out, "yes")
}

// runPowerShell corre un script en Windows PowerShell y devuelve su salida combinada.
func runPowerShell(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
