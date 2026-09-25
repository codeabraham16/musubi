package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/bootstrap"
	"musubi/internal/config"
)

// agente_plugin.go — MUSUBI INSTALADO UNA VEZ, ACTIVO EN TODOS LOS PROYECTOS.
//
// Hasta acá Musubi se conectaba al agente REPO POR REPO: `musubi setup` escribía el `.mcp.json`,
// los hooks en `.claude/settings.json` y las skills en `.claude/skills/` de cada proyecto, y en
// ~/.claude no había nada. Abrir el agente en un proyecto sin setup era trabajar sin Musubi. La
// dirección del usuario (2026-09-25) fue explícita: que esto sea una configuración de Musubi y no
// de cada repo, por defecto.
//
// `musubi agente instalar` escribe un PLUGIN de Claude Code en ~/.claude/skills/musubi/. Claude
// Code carga solo cualquier plugin de esa carpeta en la sesión siguiente (musubi@skills-dir), sin
// marketplace. El plugin trae las tres cosas que setup escribía en cada repo:
//
//   - el servidor MCP (`musubi daemon`), que desde raiz.go sabe en qué proyecto está —o que no hay
//     ninguno— y desde agente.go le habla al agente al conectarse;
//   - los mismos cinco hooks que escribe setup (ganchosDelAgente, atados a setup por una prueba);
//   - las skills cognitivas de Musubi, que el agente ve como musubi:<nombre>.
//
// Medido con `claude -p --plugin-dir` el 2026-09-25 (CLI 2.1.223 / VS Code 2.1.281): las tools
// llegan como mcp__plugin_musubi_musubi__<tool>, las instrucciones y los hooks del plugin llegan
// igual que los de un repo, y el servidor y los hooks reciben CLAUDE_PLUGIN_ROOT y
// CLAUDE_PROJECT_DIR (la carpeta de la sesión).
//
// CONVIVE CON EL CABLEADO VIEJO SIN DUPLICAR: en un proyecto que ya conecta Musubi por su
// `.mcp.json` el servidor del plugin atiende inerte, y un hook del plugin se calla si el proyecto
// ya tiene ese mismo hook. Lo que hoy funciona repo por repo sigue igual; el plugin cubre el resto.

// nombrePlugin es el nombre del plugin, y por lo tanto el prefijo de todo lo que expone.
const nombrePlugin = "musubi"

// marcaDelPlugin prueba que la carpeta la escribió Musubi. Sin ella, `instalar` no pisa y `quitar`
// no borra: la carpeta ~/.claude/skills/musubi podría ser una skill que alguien hizo a mano.
const marcaDelPlugin = ".musubi-plugin"

// ganchosDelAgente son los hooks que el agente tiene que correr, los MISMOS que escribe setup en
// .claude/settings.json de cada repo. Lo fija TestElPluginLlevaLosMismosGanchosQueSetup: si uno de
// los dos lados cambia, la prueba se pone roja.
var ganchosDelAgente = []struct{ evento, matcher, sub string }{
	{"SessionStart", "startup", "detect --hook-mode"},
	{"UserPromptSubmit", "", "turn --hook-mode"},
	{"PreToolUse", "Read", "precheck --hook-mode"},
	{"PreToolUse", matcherEdicion, "precheck --hook-mode"},
	{"Stop", "", "capture --hook-mode"},
}

// dirDelPlugin es donde se instala: ~/.claude/skills/musubi.
func dirDelPlugin() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "skills", nombrePlugin), nil
}

// runAgente implementa `musubi agente <instalar|estado|quitar> [--dir RUTA]`.
func runAgente(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: musubi agente <instalar|estado|quitar> [--dir RUTA]")
		os.Exit(2)
	}
	accion := args[0]
	fs := flag.NewFlagSet("agente "+accion, flag.ExitOnError)
	dirF := fs.String("dir", "", "carpeta del plugin (default: ~/.claude/skills/musubi)")
	_ = fs.Parse(args[1:])
	dir := *dirF
	if dir == "" {
		d, err := dirDelPlugin()
		if err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: no sé cuál es la carpeta personal: %v\n", err)
			os.Exit(1)
		}
		dir = d
	}
	switch accion {
	case "instalar":
		exe, err := os.Executable()
		if err == nil {
			exe, err = filepath.EvalSymlinks(exe)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: no sé dónde está este binario: %v\n", err)
			os.Exit(1)
		}
		if err := instalarPlugin(dir, exe, version); err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Plugin de Musubi instalado en %s.\n", dir)
		fmt.Println("Claude Code lo carga solo en la próxima sesión (musubi@skills-dir); en una sesión abierta, /reload-plugins.")
		fmt.Println("Donde un repo ya conecta Musubi por su .mcp.json, el plugin se hace a un lado y ese repo sigue como estaba.")
	case "estado":
		fmt.Println(estadoDelPlugin(dir, version))
	case "quitar":
		if err := quitarPlugin(dir); err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Plugin de Musubi quitado de %s.\n", dir)
	default:
		fmt.Fprintf(os.Stderr, "musubi agente: acción desconocida %q (instalar, estado o quitar)\n", accion)
		os.Exit(2)
	}
}

// instalarPlugin escribe (o reescribe) el plugin en dir, apuntando al binario exe.
//
// Es idempotente: reinstalar sobre un plugin de Musubi lo actualiza (otra versión, otro binario).
// Sobre una carpeta que no es de Musubi y no está vacía, se niega.
func instalarPlugin(dir, exe, ver string) error {
	if entradas, err := os.ReadDir(dir); err == nil && len(entradas) > 0 && !esPluginDeMusubi(dir) {
		return fmt.Errorf("%s ya existe y no es un plugin de Musubi (le falta %s): no lo piso", dir, marcaDelPlugin)
	}
	for _, sub := range []string{".claude-plugin", "hooks", "skills"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}

	if ver == "" {
		ver = "0.0.0"
	}
	manifiesto := map[string]interface{}{
		"name":        nombrePlugin,
		"version":     ver,
		"description": "Musubi: la memoria persistente del proyecto y su mapa de código, activa en todos los proyectos.",
		"skills":      []string{"./skills/"},
		"mcpServers":  "./.mcp.json",
		"hooks":       "./hooks/hooks.json",
	}
	if err := escribirJSON(filepath.Join(dir, ".claude-plugin", "plugin.json"), manifiesto); err != nil {
		return err
	}

	mcp := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			nombrePlugin: map[string]interface{}{"command": exe, "args": []string{"daemon"}},
		},
	}
	if err := escribirJSON(filepath.Join(dir, ".mcp.json"), mcp); err != nil {
		return err
	}

	ganchos, err := ganchosDelPlugin(exe)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks", "hooks.json"), ganchos, 0o644); err != nil {
		return err
	}

	// Las skills cognitivas, con el MISMO escritor que el export a .claude/skills: una editada a
	// mano se preserva igual que allá.
	for _, sk := range cognitiveSkills(nil) {
		if _, err := escribirSkillMD(filepath.Join(dir, "skills"), sk); err != nil {
			return fmt.Errorf("escribir la skill %s: %w", sk.Name, err)
		}
	}

	return os.WriteFile(filepath.Join(dir, marcaDelPlugin), []byte("version: "+ver+"\nbinario: "+exe+"\n"), 0o644)
}

// ganchosDelPlugin arma hooks/hooks.json con el mismo formato que la clave "hooks" de un
// settings.json, usando el mismo fusionador que setup (bootstrap.MergeClaudeSettings).
func ganchosDelPlugin(exe string) ([]byte, error) {
	var doc []byte
	for _, g := range ganchosDelAgente {
		var err error
		doc, err = bootstrap.MergeClaudeSettings(doc, g.evento, g.matcher, bootstrap.HookCommand{
			Type:    "command",
			Command: quoteExe(exe) + " " + g.sub,
			Timeout: 10,
		})
		if err != nil {
			return nil, err
		}
	}
	return doc, nil
}

// escribirJSON escribe v indentado, con salto final.
func escribirJSON(ruta string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ruta, append(b, '\n'), 0o644)
}

// esPluginDeMusubi dice si dir lo escribió Musubi.
func esPluginDeMusubi(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, marcaDelPlugin))
	return err == nil
}

// estadoDelPlugin dice si el plugin está, de qué versión, y si coincide con este binario.
func estadoDelPlugin(dir, ver string) string {
	crudo, err := os.ReadFile(filepath.Join(dir, marcaDelPlugin))
	if err != nil {
		if _, errDir := os.Stat(dir); errDir == nil {
			return fmt.Sprintf("NO INSTALADO: %s existe pero no es un plugin de Musubi.", dir)
		}
		return fmt.Sprintf("NO INSTALADO: no hay plugin en %s. Instalalo con: musubi agente instalar", dir)
	}
	instalada := ""
	for _, l := range strings.Split(string(crudo), "\n") {
		if v, ok := strings.CutPrefix(l, "version: "); ok {
			instalada = strings.TrimSpace(v)
		}
	}
	if instalada != ver {
		return fmt.Sprintf("DESACTUALIZADO: en %s está la versión %s y este binario es %s. Reinstalá con: musubi agente instalar", dir, instalada, ver)
	}
	return fmt.Sprintf("INSTALADO: %s, versión %s.", dir, instalada)
}

// quitarPlugin borra el plugin, SÓLO si lo escribió Musubi.
func quitarPlugin(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	if !esPluginDeMusubi(dir) {
		return fmt.Errorf("%s no es un plugin de Musubi (le falta %s): no lo borro", dir, marcaDelPlugin)
	}
	return os.RemoveAll(dir)
}

// ── Convivencia con el cableado por repo ──────────────────────────────────────────────────────

// corriendoComoPlugin dice si este proceso lo lanzó el plugin: Claude Code le pasa
// CLAUDE_PLUGIN_ROOT a los servidores y hooks de un plugin, y a ningún otro.
func corriendoComoPlugin() bool { return os.Getenv("CLAUDE_PLUGIN_ROOT") != "" }

// carpetaDeLaSesion es donde Claude Code lee el `.mcp.json` y el `.claude/settings.json` del
// proyecto: CLAUDE_PROJECT_DIR, o el cwd.
func carpetaDeLaSesion() string {
	if d := os.Getenv("CLAUDE_PROJECT_DIR"); d != "" {
		return d
	}
	d, _ := os.Getwd()
	return d
}

// elProyectoYaConectaMusubi dice si el `.mcp.json` del proyecto ya declara el servidor musubi.
// Ahí el servidor del plugin sería un segundo Musubi contra la misma memoria —dos catálogos, dos
// juegos de instrucciones—, así que se hace a un lado.
func elProyectoYaConectaMusubi(dir string) bool {
	crudo, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		return false
	}
	var doc struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(crudo, &doc) != nil {
		return false
	}
	_, ok := doc.McpServers[nombrePlugin]
	return ok
}

// elProyectoYaTieneElGancho dice si el proyecto ya corre este hook de Musubi desde su propio
// settings. Claude Code corre los hooks del proyecto Y los del plugin: sin esto, el turno
// inyectaría todo dos veces y la captura correría dos veces.
func elProyectoYaTieneElGancho(dir, sub string) bool {
	for _, f := range []string{config.ClaudeSettingsFile, "settings.local.json"} {
		crudo, err := os.ReadFile(filepath.Join(dir, config.ClaudeDir, f))
		if err != nil {
			continue
		}
		if strings.Contains(string(crudo), sub+" --hook-mode") && strings.Contains(string(crudo), "musubi") {
			return true
		}
	}
	return false
}

// elPluginCedeElGancho es la pregunta que se hacen los cuatro hooks al arrancar.
func elPluginCedeElGancho(sub string) bool {
	return corriendoComoPlugin() && elProyectoYaTieneElGancho(carpetaDeLaSesion(), sub)
}
