package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── fakes (compartidos por los tests del paquete) ────────────────────────────

type fakeProber struct{ public, tailnet bool }

func (f *fakeProber) PublicReachable() bool        { return f.public }
func (f *fakeProber) TailnetReachable(string) bool { return f.tailnet }

type fakeVerifier struct {
	reach, auth           bool
	reachCalls, authCalls int
}

func (f *fakeVerifier) Reach(string) (bool, string) { f.reachCalls++; return f.reach, "reach-detail" }
func (f *fakeVerifier) Auth(string, string) (bool, string) {
	f.authCalls++
	return f.auth, "auth-detail"
}

type fakeNetwork struct {
	present, joined bool
	ensureCalls     int
	lastEnsureDry   bool
	joinCalls       int
}

func (f *fakeNetwork) TailscaleState() (bool, bool) { return f.present, f.joined }
func (f *fakeNetwork) EnsureTailnetAllowed(dry bool) StepResult {
	f.ensureCalls++
	f.lastEnsureDry = dry
	return StepResult{Name: "firewall", Status: StatusOK, Detail: "regla presente"}
}
func (f *fakeNetwork) JoinTailscale(string) StepResult {
	f.joinCalls++
	return StepResult{Name: "tailscale", Status: StatusDone, Detail: "unido"}
}

func stepByName(rep Report, name string) (StepResult, bool) {
	for _, s := range rep.Steps {
		if s.Name == name {
			return s, true
		}
	}
	return StepResult{}, false
}

// ── escenarios ───────────────────────────────────────────────────────────────

func TestRunCleanWiresAndSelfChecks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	v := &fakeVerifier{reach: true, auth: true}
	nc := &fakeNetwork{present: true, joined: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: v, NetworkConfigurator: nc, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Mode != ModeClean {
		t.Fatalf("modo = %v; quería Clean", rep.Mode)
	}
	if !rep.Connected {
		t.Fatal("esperaba Connected=true")
	}
	if v.reachCalls != 1 || v.authCalls != 1 {
		t.Fatalf("self-check no corrió: reach=%d auth=%d", v.reachCalls, v.authCalls)
	}
	// El .mcp.json debe tener AMBAS entradas, preservando estructura.
	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("no se escribió .mcp.json: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"musubi"`) || !strings.Contains(s, `"musubi-cerebro"`) {
		t.Fatalf(".mcp.json sin ambas entradas:\n%s", s)
	}
	if strings.Contains(s, "secreto") {
		t.Fatal("FUGA: el token no debe aparecer en el .mcp.json")
	}
}

func TestRunTunneledStopsBeforeMutating(t *testing.T) {
	dir := t.TempDir()
	nc := &fakeNetwork{present: true, joined: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	// público OK, tailnet NO = Tunneled.
	deps := Deps{Prober: &fakeProber{public: true, tailnet: false}, Verifier: &fakeVerifier{}, NetworkConfigurator: nc, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Mode != ModeTunneled {
		t.Fatalf("modo = %v; quería Tunneled", rep.Mode)
	}
	if rep.Connected {
		t.Fatal("Tunneled no debería declararse conectado")
	}
	if nc.ensureCalls != 0 {
		t.Fatal("no debería tocar el firewall antes de resolver el bloqueo")
	}
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("no debería escribir el .mcp.json en modo Tunneled")
	}
	if s, ok := stepByName(rep, "red"); !ok || s.Status != StatusTodo {
		t.Fatalf("esperaba un paso 'red' TODO con guía; got %+v", s)
	}
}

func TestRunDryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	v := &fakeVerifier{reach: true, auth: true}
	nc := &fakeNetwork{present: true, joined: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN", DryRun: true}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: v, NetworkConfigurator: nc, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("dry-run no debería escribir el .mcp.json")
	}
	if !nc.lastEnsureDry {
		t.Fatal("EnsureTailnetAllowed debería recibir dryRun=true")
	}
	if v.reachCalls != 0 || v.authCalls != 0 {
		t.Fatal("dry-run no debería correr el self-check")
	}
	if rep.Connected {
		t.Fatal("dry-run no conecta")
	}
}

func TestRunIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	mk := func() (Report, error) {
		return Run(
			Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"},
			Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: &fakeVerifier{reach: true, auth: true}, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"},
		)
	}
	if _, err := mk(); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	rep2, err := mk()
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	s, ok := stepByName(rep2, "mcp.json")
	if !ok || s.Status != StatusOK {
		t.Fatalf("la re-ejecución debería reportar mcp.json sin cambios (ok); got %+v", s)
	}
}

func TestRunEmptyTokenSkipsAuth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "") // explícitamente vacío
	v := &fakeVerifier{reach: true, auth: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: v, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.reachCalls != 1 {
		t.Fatal("reach debería correr aún sin token")
	}
	if v.authCalls != 0 {
		t.Fatal("auth NO debería correr sin token")
	}
	if s, ok := stepByName(rep, "auth"); !ok || s.Status != StatusSkipped {
		t.Fatalf("esperaba auth=skipped; got %+v", s)
	}
	if rep.Connected {
		t.Fatal("sin auth verificada no hay conexión confirmada")
	}
}
