package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// ANTES DE COMPACTAR, MUSUBI LE DICE AL RESUMEN QUÉ TIENE QUE CONSERVAR, Y LO DICE POR EL CANAL QUE
// LLEGA.
//
// En PreCompact, la salida estándar de un hook que termina bien se agrega a las instrucciones del
// resumen; un envelope JSON con additionalContext, en cambio, Claude Code lo descarta entero (así
// estuvo este hook tres semanas en verde sin hacer nada). Por eso se mide la salida del proceso: que
// traiga las instrucciones, que sea texto y no un envelope, y que el hook del plugin se calle cuando
// el proyecto ya lo corre desde su propio settings (dos copias sólo agrandan el pedido del resumen).
//
// Sabotaje que la hace fallar: que el hook no imprima las instrucciones.
// arnes: archivo="cmd/musubi/precompact.go"
// arnes: de="\tfmt.Print(instruccionesDeCompactacion)\n"
// arnes: a="\tfmt.Print(\"\")\n"
//
// Sabotaje que la hace fallar: que el plugin repita las instrucciones que ya da el proyecto.
// arnes: archivo="cmd/musubi/precompact.go"
// arnes: de="\tif elPluginCedeElGancho(\"precompact\") {\n"
// arnes: a="\tif false && elPluginCedeElGancho(\"precompact\") {\n"
func TestElResumenRecibeLasInstruccionesDeMusubi(t *testing.T) {
	home := t.TempDir()
	payload := `{"hook_event_name":"PreCompact","trigger":"auto","custom_instructions":""}`

	// Fuera del plugin (la variable vacía por si la corrida hereda la de una sesión).
	out := correrMusubiCon(t, home, home, payload, []string{"CLAUDE_PLUGIN_ROOT="}, "precompact", "--hook-mode")
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("PreCompact no acepta un envelope JSON: Claude Code lo descarta. Salió:\n%s", out)
	}
	for _, clave := range []string{"TEXTUALES las reglas, decisiones y pedidos", "el próximo paso", "[id:…]", "No copies salidas largas"} {
		if !strings.Contains(out, clave) {
			t.Errorf("las instrucciones del resumen no dicen %q:\n%s", clave, out)
		}
	}

	// Como plugin, en un proyecto que ya corre el hook desde su settings: se calla.
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repo, config.ClaudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	propio := `{"hooks":{"PreCompact":[{"hooks":[{"type":"command","command":"/usr/local/bin/musubi precompact --hook-mode"}]}]}}`
	if err := os.WriteFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile), []byte(propio), 0o644); err != nil {
		t.Fatal(err)
	}
	comoPlugin := []string{"CLAUDE_PLUGIN_ROOT=" + filepath.Join(home, "plugin"), "CLAUDE_PROJECT_DIR=" + repo}
	if out := correrMusubiCon(t, repo, home, payload, comoPlugin, "precompact", "--hook-mode"); strings.TrimSpace(out) != "" {
		t.Errorf("el plugin repitió las instrucciones en un proyecto que ya las da:\n%s", out)
	}

	// Como plugin, en un proyecto sin el hook propio: las da el plugin.
	sinHook := filepath.Join(home, "otro")
	if err := os.MkdirAll(sinHook, 0o755); err != nil {
		t.Fatal(err)
	}
	comoPlugin[1] = "CLAUDE_PROJECT_DIR=" + sinHook
	if out := correrMusubiCon(t, sinHook, home, payload, comoPlugin, "precompact", "--hook-mode"); !strings.Contains(out, "Instrucciones de Musubi") {
		t.Errorf("como plugin, en un proyecto sin hook propio, no dio las instrucciones:\n%s", out)
	}
}

// `musubi setup` ESCRIBE EL HOOK PreCompact, igual que los otros: sin él, los repos cableados a mano
// se quedarían sin las instrucciones del resumen (y el plugin, que corre los mismos hooks que setup,
// tampoco las tendría: lo cuida TestElPluginLlevaLosMismosGanchosQueSetup).
//
// Sabotaje que la hace fallar: que setup no registre el hook.
// arnes: archivo="cmd/musubi/setup.go"
// arnes: de="\t\tif err := writePreCompactHook(root, exePath); err != nil {\n"
// arnes: a="\t\tif err := error(nil); err != nil {\n"
func TestSetupRegistraElHookDelResumen(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	correrMusubi(t, repo, home, "", "setup", "--agent", "claude")
	crudo, err := os.ReadFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile))
	if err != nil {
		t.Fatalf("setup no dejó .claude/settings.json: %v", err)
	}
	var hay bool
	for _, g := range ganchosDeUnSettings(t, crudo) {
		if strings.HasPrefix(g, "PreCompact||") && strings.HasSuffix(g, "precompact --hook-mode") {
			hay = true
		}
	}
	if !hay {
		t.Errorf("setup no registró el hook PreCompact:\n%s", crudo)
	}
}
