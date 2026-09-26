package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// leerPermisos devuelve las listas allow, ask y deny de un settings.json, y el resto de sus claves.
func leerPermisos(t *testing.T, ruta string) (allow, ask, deny []string, raiz map[string]json.RawMessage) {
	t.Helper()
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(crudo, &raiz); err != nil {
		t.Fatalf("el settings quedó ilegible: %v\n%s", err, crudo)
	}
	var p struct {
		Allow []string `json:"allow"`
		Ask   []string `json:"ask"`
		Deny  []string `json:"deny"`
	}
	if raw, ok := raiz["permissions"]; ok {
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatal(err)
		}
	}
	return p.Allow, p.Ask, p.Deny, raiz
}

func contar(lista []string, r string) int {
	n := 0
	for _, x := range lista {
		if x == r {
			n++
		}
	}
	return n
}

// LOS PERMISOS QUE PONE MUSUBI: TODO MUSUBI PERMITIDO, LO DE AFUERA PREGUNTANDO, Y LO DE LA PERSONA
// INTACTO.
//
// Un plugin no puede darse permisos, así que los pone el instalador en el settings del usuario, y
// ahí conviven con los de la persona. Lo que no puede pasar: pisar un settings ajeno, duplicar una
// regla al reinstalar, dejar reglas viejas al cambiar de opinión, o que `quitar` se lleve una regla
// que la persona ya tenía (aunque sea idéntica a una de Musubi). Las reglas se escriben literales:
// es el formato de Claude Code (`mcp__<servidor>`, `mcp__<servidor>__<tool>`, `Bash(<cmd>:*)`).
//
// Sabotaje que la hace fallar: anotar como de Musubi una regla que la persona ya tenía.
// arnes: archivo="cmd/musubi/permisos_agente.go"
// arnes: de="\t\tif eraNuestra[r] && !presente[r] {\n"
// arnes: a="\t\tif (eraNuestra[r] || quieroSet[r]) && !presente[r] {\n"
//
// Sabotaje que la hace fallar: duplicar las reglas al reinstalar.
// arnes: archivo="cmd/musubi/permisos_agente.go"
// arnes: de="\tfor _, r := range quiero {\n\t\tif presente[r] {\n\t\t\tcontinue\n\t\t}\n"
// arnes: a="\tfor _, r := range quiero {\n\t\tif false && presente[r] {\n\t\t\tcontinue\n\t\t}\n"
//
// Sabotaje que la hace fallar: no sacar lo que Musubi ya no quiere.
// arnes: archivo="cmd/musubi/permisos_agente.go"
// arnes: de="\t\tif eraNuestra[r] && !quieroSet[r] {\n"
// arnes: a="\t\tif false && eraNuestra[r] && !quieroSet[r] {\n"
//
// Sabotaje que la hace fallar: no darle al subagente de tareas su git de lectura.
// arnes: archivo="cmd/musubi/permisos_agente.go"
// arnes: de="\t\tallow = append(allow, \"Bash(\"+c+\":*)\")\n"
// arnes: a="\t\t_ = c\n"
func TestLosPermisosDeMusubiConvivenConLosDeLaPersona(t *testing.T) {
	dir := t.TempDir()
	plugin := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(plugin, 0o755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(dir, "settings.json")
	previo := `{"theme":"dark","permissions":{"allow":["Bash(ls:*)","mcp__musubi"],"deny":["Bash(rm:*)"],"defaultMode":"auto"}}`
	if err := os.WriteFile(settings, []byte(previo), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := instalarPermisos(plugin, settings, false); err != nil {
		t.Fatal(err)
	}
	allow, ask, deny, raiz := leerPermisos(t, settings)
	for _, r := range []string{"mcp__musubi", "mcp__musubi-cerebro", "mcp__plugin_musubi_musubi",
		"Bash(git log:*)", "Bash(git show:*)", "Bash(git grep:*)", "Bash(git branch --contains:*)", "Bash(ls:*)"} {
		if contar(allow, r) != 1 {
			t.Errorf("allow tiene %q %d vez/veces; quería 1:\n%v", r, contar(allow, r), allow)
		}
	}
	for _, srv := range []string{"musubi", "musubi-cerebro", "plugin_musubi_musubi"} {
		if r := "mcp__" + srv + "__musubi_fleet_exec"; contar(ask, r) != 1 {
			t.Errorf("ask no tiene %q: el agente ejecutaría en otra máquina sin preguntar", r)
		}
	}
	if len(deny) != 1 || deny[0] != "Bash(rm:*)" || string(raiz["theme"]) != `"dark"` {
		t.Errorf("se tocó lo ajeno al instalar: deny=%v theme=%s", deny, raiz["theme"])
	}
	var perms map[string]json.RawMessage
	_ = json.Unmarshal(raiz["permissions"], &perms)
	if string(perms["defaultMode"]) != `"auto"` {
		t.Errorf("se perdió el defaultMode de la persona: %s", perms["defaultMode"])
	}

	// Reinstalar no duplica nada.
	if _, err := instalarPermisos(plugin, settings, false); err != nil {
		t.Fatal(err)
	}
	allow2, ask2, _, _ := leerPermisos(t, settings)
	if len(allow2) != len(allow) || len(ask2) != len(ask) {
		t.Errorf("reinstalar cambió las listas: allow %d→%d, ask %d→%d", len(allow), len(allow2), len(ask), len(ask2))
	}

	// Con --flota-sin-preguntar, las de «preguntar» que puso Musubi se van.
	if _, err := instalarPermisos(plugin, settings, true); err != nil {
		t.Fatal(err)
	}
	if _, ask3, _, _ := leerPermisos(t, settings); len(ask3) != 0 {
		t.Errorf("con --flota-sin-preguntar quedaron %d regla(s) en ask: %v", len(ask3), ask3[:1])
	}

	// Quitar deja lo de la persona tal cual, incluida su mcp__musubi.
	if err := quitarPermisos(plugin); err != nil {
		t.Fatal(err)
	}
	allow4, ask4, deny4, _ := leerPermisos(t, settings)
	if strings.Join(allow4, ",") != "Bash(ls:*),mcp__musubi" || len(ask4) != 0 || len(deny4) != 1 {
		t.Errorf("después de quitar: allow=%v ask=%v deny=%v; quería exactamente lo de la persona", allow4, ask4, deny4)
	}
	if _, err := os.Stat(filepath.Join(plugin, marcaDePermisos)); !os.IsNotExist(err) {
		t.Error("quitar dejó la anotación de permisos")
	}
}

// UN SETTINGS QUE NO SE ENTIENDE NO SE TOCA.
//
// Reescribirlo desde cero borraría todo lo que la persona tenía, y un settings roto apaga todo lo
// que ese archivo configura en Claude Code.
//
// Sabotaje que la hace fallar: tratar un settings ilegible como vacío.
// arnes: archivo="cmd/musubi/permisos_agente.go"
// arnes: de="\t\t\treturn nil, nil, fmt.Errorf(\"%s no es JSON válido (no lo toco): %w\", ruta, err)\n"
// arnes: a="\t\t\traiz = map[string]json.RawMessage{}\n"
func TestUnSettingsIlegibleNoSeToca(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings.json")
	roto := "{ \"theme\": \"dark\", // un comentario\n}"
	if err := os.WriteFile(settings, []byte(roto), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := instalarPermisos(dir, settings, false); err == nil {
		t.Error("instalar sobre un settings ilegible no dio error")
	}
	if b, _ := os.ReadFile(settings); string(b) != roto {
		t.Errorf("el settings ilegible quedó cambiado:\n%s", b)
	}
}

// EL PROTOCOLO DEL SUBAGENTE NOMBRA LOS MISMOS COMANDOS DE GIT QUE RECIBEN PERMISO.
//
// Si el protocolo le pidiera un comando que no tiene regla, en modo normal cada verificación pediría
// confirmación desde un trabajo de fondo.
func TestElProtocoloNombraElGitQueTienePermiso(t *testing.T) {
	texto := contenidoDelSubagenteDeTareas("musubi", "musubi")
	allow, _ := reglasDelAgente("musubi", false)
	for _, c := range comandosGitDeLectura {
		if !strings.Contains(texto, c) {
			t.Errorf("el protocolo no nombra %q", c)
		}
		if contar(allow, "Bash("+c+":*)") != 1 {
			t.Errorf("%q no tiene su regla", c)
		}
	}
	if strings.Contains(texto, "{{GIT}}") {
		t.Error("quedó el marcador {{GIT}} sin reemplazar en el protocolo")
	}
}

// `musubi agente instalar` PONE LOS PERMISOS EN EL SETTINGS DEL USUARIO, Y `quitar` LOS SACA.
//
// Se mide sobre el proceso: el cableado de los flags y del settings por defecto vive en runAgente.
//
// Sabotaje que la hace fallar: que instalar no ponga los permisos.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\tif !*sinPermisos {\n"
// arnes: a="\t\tif false && !*sinPermisos {\n"
//
// Sabotaje que la hace fallar: que quitar deje los permisos puestos.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\tif err := quitarPermisos(dir); err != nil {\n"
// arnes: a="\t\tif err := error(nil); err != nil {\n"
func TestElInstaladorPoneYSacaLosPermisos(t *testing.T) {
	home := t.TempDir()
	plugin := filepath.Join(home, ".claude", "skills", "musubi")
	settings := filepath.Join(home, ".claude", "settings.json")
	sinConfig := []string{"CLAUDE_CONFIG_DIR="}

	correrMusubiCon(t, home, home, "", sinConfig, "agente", "instalar", "--dir", plugin)
	allow, ask, _, _ := leerPermisos(t, settings)
	if contar(allow, "mcp__plugin_musubi_musubi") != 1 || contar(ask, "mcp__musubi__musubi_fleet_exec") != 1 {
		t.Fatalf("instalar no dejó los permisos en %s: allow=%v ask=%d", settings, allow, len(ask))
	}
	out := correrMusubiCon(t, home, home, "", sinConfig, "agente", "estado", "--dir", plugin)
	if !strings.Contains(out, "Permisos:") || !strings.Contains(out, settings) {
		t.Errorf("estado no dice dónde están los permisos:\n%s", out)
	}

	correrMusubiCon(t, home, home, "", sinConfig, "agente", "quitar", "--dir", plugin)
	if allow, ask, _, _ := leerPermisos(t, settings); len(allow)+len(ask) != 0 {
		t.Errorf("quitar dejó reglas de Musubi: allow=%v ask=%d", allow, len(ask))
	}
}

// EL PLUGIN Y SUS PERMISOS VAN A LA MISMA CARPETA DE CONFIGURACIÓN.
//
// Con CLAUDE_CONFIG_DIR puesta, Claude Code lee todo de ahí. Si el plugin se derivara de ~/.claude y
// los permisos de la variable, quedarían en dos lugares y uno de los dos no lo leería nadie.
//
// Sabotaje que la hace fallar: que el plugin ignore CLAUDE_CONFIG_DIR.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\tbase, err := dirConfigDeClaude()\n\tif err != nil {\n\t\treturn \"\", err\n\t}\n\treturn filepath.Join(base, \"skills\", nombrePlugin), nil\n"
// arnes: a="\thome, err := os.UserHomeDir()\n\tif err != nil {\n\t\treturn \"\", err\n\t}\n\treturn filepath.Join(home, \".claude\", \"skills\", nombrePlugin), nil\n"
func TestElPluginYSusPermisosVanALaMismaConfig(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	plugin, err := dirDelPlugin()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := settingsPorDefecto()
	if err != nil {
		t.Fatal(err)
	}
	if plugin != filepath.Join(cfg, "skills", "musubi") || settings != filepath.Join(cfg, "settings.json") {
		t.Errorf("con CLAUDE_CONFIG_DIR=%s: plugin en %s y permisos en %s; los dos van ahí", cfg, plugin, settings)
	}
}
