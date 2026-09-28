package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"musubi/internal/bootstrap"
	"musubi/internal/config"
	"musubi/internal/mcp"
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

// dirSkillsDelPlugin es la carpeta de skills dentro del plugin. La nombran el manifiesto, el
// instalador que escribe los SKILL.md y el servidor que se los nombra al agente.
const dirSkillsDelPlugin = "skills"

// marcaDelPlugin prueba que la carpeta la escribió Musubi. Sin ella, `instalar` no pisa y `quitar`
// no borra: la carpeta ~/.claude/skills/musubi podría ser una skill que alguien hizo a mano.
const marcaDelPlugin = ".musubi-plugin"

// ganchosDelAgente son los hooks que el agente tiene que correr, los MISMOS que escribe setup en
// .claude/settings.json de cada repo. Lo fija TestElPluginLlevaLosMismosGanchosQueSetup: si uno de
// los dos lados cambia, la prueba se pone roja.
//
// El arranque va con tres matchers y el mismo comando: «compact» devuelve la memoria que el resumen
// perdió (detect_compactar.go) y «clear» es una sesión nueva, que va por el arranque. «resume» y
// «fork» no van.
var ganchosDelAgente = []struct{ evento, matcher, sub string }{
	{"SessionStart", "startup", "detect --hook-mode"},
	{"SessionStart", "compact", "detect --hook-mode"},
	{"SessionStart", "clear", "detect --hook-mode"},
	{"UserPromptSubmit", "", "turn --hook-mode"},
	{"PreToolUse", "Read", "precheck --hook-mode"},
	{"PreToolUse", matcherEdicion, "precheck --hook-mode"},
	{"Stop", "", "capture --hook-mode"},
	{"PreCompact", "", "precompact --hook-mode"},
}

// dirDelPlugin es donde se instala: <config de Claude Code>/skills/musubi.
func dirDelPlugin() (string, error) {
	base, err := dirConfigDeClaude()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "skills", nombrePlugin), nil
}

// dirConfigDeClaude es la carpeta de configuración de Claude Code: CLAUDE_CONFIG_DIR si está puesta
// (así la reubica Claude Code), y si no ~/.claude. La leen el plugin y sus permisos: con dos
// derivaciones distintas, con la variable puesta el plugin iba a un lado y los permisos a otro.
func dirConfigDeClaude() (string, error) {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// runAgente implementa `musubi agente <instalar|estado|quitar> [--dir RUTA]`.
func runAgente(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "uso: musubi agente <instalar|estado|quitar> [--dir RUTA] [--settings RUTA] [--sin-permisos] [--flota-sin-preguntar] [--compactar-en TOKENS]")
		os.Exit(2)
	}
	accion := args[0]
	fs := flag.NewFlagSet("agente "+accion, flag.ExitOnError)
	dirF := fs.String("dir", "", "carpeta del plugin (default: ~/.claude/skills/musubi)")
	settingsF := fs.String("settings", "", "settings.json de Claude Code donde van los permisos y el ahorro (default: ~/.claude/settings.json)")
	sinPermisos := fs.Bool("sin-permisos", false, "instalar sin tocar los permisos de Claude Code")
	flotaSinPreguntar := fs.Bool("flota-sin-preguntar", false,
		"permitir también, sin preguntar, las tools que actúan sobre otras máquinas de la flota o sobre credenciales")
	compactarEn := fs.Int("compactar-en", ventanaDeCompactacionPorDefecto,
		"resumir la conversación al llegar a estos tokens (autoCompactWindow); 0 no pone ninguna")
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
	settings := *settingsF
	if settings == "" {
		s, err := settingsPorDefecto()
		if err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: no sé dónde está el settings.json de Claude Code: %v\n", err)
			os.Exit(1)
		}
		settings = s
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
		if !*sinPermisos {
			p, err := instalarPermisos(dir, settings, *flotaSinPreguntar)
			if err != nil {
				fmt.Fprintf(os.Stderr, "musubi agente: el plugin quedó instalado, pero no pude poner los permisos: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Permisos en %s: Musubi permitido (%d regla(s) puestas por Musubi)", settings, len(p.Allow))
			if len(p.Ask) > 0 {
				fmt.Printf("; %d tool(s) que actúan sobre otras máquinas o credenciales siguen preguntando (--flota-sin-preguntar las permite)", len(p.Ask)/len(servidoresDeMusubi(nombrePlugin)))
			}
			fmt.Println(".")
		}
		a, propia, err := instalarAhorro(dir, settings, *compactarEn)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "musubi agente: el plugin quedó instalado, pero no pude poner el ahorro de tokens: %v\n", err)
			os.Exit(1)
		case propia != "":
			fmt.Printf("Ahorro: respeté tu ventana de compactación (%s) en %s.\n", propia, settings)
		case a.Ventana > 0:
			fmt.Printf("Ahorro: la conversación se resume al llegar a %s tokens (autoCompactWindow en %s).\n", enMiles(a.Ventana), settings)
		}
		fmt.Println("Claude Code lo carga solo en la próxima sesión (musubi@skills-dir); en una sesión abierta, /reload-plugins.")
		fmt.Println("Donde un repo ya conecta Musubi por su .mcp.json, el plugin se hace a un lado y ese repo sigue como estaba.")
	case "estado":
		fmt.Println(estadoDelPlugin(dir, version))
		if p, ok := leerPermisosAnotados(dir); ok {
			fmt.Printf("Permisos: %d permitidas y %d en «preguntar», puestas por Musubi en %s.\n", len(p.Allow), len(p.Ask), p.Settings)
		} else {
			fmt.Println("Permisos: Musubi no puso ninguno (cada llamada puede pedir confirmación).")
		}
		fmt.Println(estadoDelAhorro(dir, settings))
		fmt.Println(estadoDeLaCompactacion(carpetaDeLaSesion(), dir))
	case "quitar":
		if err := quitarPermisos(dir); err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: no pude sacar los permisos que había puesto: %v\n", err)
			os.Exit(1)
		}
		if err := quitarAhorro(dir); err != nil {
			fmt.Fprintf(os.Stderr, "musubi agente: no pude sacar el ahorro de tokens que había puesto: %v\n", err)
			os.Exit(1)
		}
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
	for _, sub := range []string{".claude-plugin", "hooks", dirSkillsDelPlugin} {
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
		"skills":      []string{"./" + dirSkillsDelPlugin + "/"},
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
		if _, err := escribirSkillMD(filepath.Join(dir, dirSkillsDelPlugin), sk); err != nil {
			return fmt.Errorf("escribir la skill %s: %w", sk.Name, err)
		}
	}

	// El subagente que hace las tareas que Musubi deja en su tablero (subagente_tareas.go). El
	// servidor se llama como el plugin: es la clave que escribe el .mcp.json de arriba.
	if err := escribirSubagenteDeTareas(dir, nombrePlugin, nombrePlugin); err != nil {
		return fmt.Errorf("escribir el subagente de tareas: %w", err)
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

// skillsDelPluginParaElMapa es la option con que el servidor del plugin le nombra al agente las
// skills que el plugin trae. Si este proceso no es el del plugin, o su manifiesto no se lee, es una
// option que no hace nada (WithSkillsDelPlugin sin carpeta): así va siempre dentro de la llamada.
//
// EL PREFIJO SALE DEL MANIFIESTO INSTALADO, no de nombrePlugin: Claude Code nombra las skills de un
// plugin con el `name` de su plugin.json (medido: un plugin en la carpeta `plug` con name
// `pruebagentes` expuso `pruebagentes:<agente>`), así que es el manifiesto el que dice cómo las ve
// el agente. Sin manifiesto legible no se nombra ninguna: un prefijo adivinado es un callejón.
func skillsDelPluginParaElMapa() mcp.Option {
	ninguna := mcp.WithSkillsDelPlugin("", "", nil)
	raiz := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if raiz == "" {
		return ninguna
	}
	nombre, err := nombreDelPluginEn(raiz)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi: %v: el mapa no nombra las skills del plugin\n", err)
		return ninguna
	}
	return mcp.WithSkillsDelPlugin(filepath.Join(raiz, dirSkillsDelPlugin), nombre, cognitiveSkills(nil))
}

// nombreDelPluginEn lee el `name` del plugin.json de un plugin: el prefijo con que Claude Code
// nombra sus skills y sus subagentes.
func nombreDelPluginEn(raiz string) (string, error) {
	crudo, err := os.ReadFile(filepath.Join(raiz, ".claude-plugin", "plugin.json"))
	if err != nil {
		return "", fmt.Errorf("no leo el manifiesto del plugin: %w", err)
	}
	var manifiesto struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(crudo, &manifiesto); err != nil {
		return "", fmt.Errorf("el manifiesto del plugin no se entiende: %w", err)
	}
	return manifiesto.Name, nil
}

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

// elPluginCedeElGancho es la pregunta que se hacen los hooks al arrancar. El de arranque no la usa:
// hace la suya, que mira el matcher (elPluginCedeElArranque).
func elPluginCedeElGancho(sub string) bool {
	return corriendoComoPlugin() && elProyectoYaTieneElGancho(carpetaDeLaSesion(), sub)
}

// ── El arranque cede según la fuente ─────────────────────────────────────────────────────────

// elPluginCedeElArranque es la pregunta del hook de arranque del plugin, y a diferencia de la de los
// demás hooks mira el MATCHER. Mirar sólo si el proyecto nombra `detect --hook-mode` alcanzaba
// mientras el arranque tenía un solo matcher; con tres, un repo con el settings viejo (sólo
// «startup») y el plugin nuevo haría que el plugin cediera también la compactación, que el repo no
// corre: nadie devolvería la memoria. El plugin cede sólo si el proyecto ya corre este hook PARA ESTA
// MISMA FUENTE, porque entonces Claude Code corre los dos.
func elPluginCedeElArranque(fuente string) bool {
	return corriendoComoPlugin() && elProyectoYaTieneElArranque(carpetaDeLaSesion(), fuente)
}

// elProyectoYaTieneElArranque dice si el proyecto corre el hook de arranque de Musubi para esta
// fuente desde cualquiera de sus dos settings: Claude Code junta los hooks de los dos.
func elProyectoYaTieneElArranque(dir, fuente string) bool {
	for _, f := range []string{config.ClaudeSettingsFile, "settings.local.json"} {
		crudo, err := os.ReadFile(filepath.Join(dir, config.ClaudeDir, f))
		if err != nil {
			continue
		}
		if arranqueDeMusubiCubre(crudo, fuente) {
			return true
		}
	}
	return false
}

// arranqueDeMusubiCubre dice si un settings —o el hooks.json de un plugin, que tiene la misma
// forma— corre el hook de arranque de Musubi para la fuente. La fuente vacía es la de un evento sin
// source, que va por el arranque: cuenta como «startup». Un JSON que no se entiende no cubre nada.
func arranqueDeMusubiCubre(crudo []byte, fuente string) bool {
	if !strings.Contains(strings.ToLower(string(crudo)), "musubi") {
		return false
	}
	if fuente == "" {
		fuente = fuenteStartup
	}
	var doc struct {
		Hooks struct {
			SessionStart []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"SessionStart"`
		} `json:"hooks"`
	}
	if json.Unmarshal(crudo, &doc) != nil {
		return false
	}
	for _, e := range doc.Hooks.SessionStart {
		if !matcherCubre(e.Matcher, fuente) {
			continue
		}
		for _, h := range e.Hooks {
			if strings.Contains(h.Command, "detect --hook-mode") {
				return true
			}
		}
	}
	return false
}

// matcherSimple es la forma de matcher que Claude Code compara por igualdad, o por lista con «|».
var matcherSimple = regexp.MustCompile(`^[a-zA-Z0-9_|]+$`)

// matcherCubre dice si un matcher de hook dispara para el valor, con la regla de Claude Code: vacío
// o «*» dispara para todo; letras, dígitos, «_» y «|» son uno o varios valores exactos; cualquier
// otra cosa es una expresión regular sin anclar, y una que no compila no dispara.
func matcherCubre(matcher, valor string) bool {
	if matcher == "" || matcher == "*" {
		return true
	}
	if matcherSimple.MatchString(matcher) {
		for _, m := range strings.Split(matcher, "|") {
			if strings.TrimSpace(m) == valor {
				return true
			}
		}
		return false
	}
	re, err := regexp.Compile(matcher)
	if err != nil {
		return false
	}
	return re.MatchString(valor)
}

// elPluginEnCubre dice si el plugin de Musubi en dir corre el hook de arranque para la fuente. Una
// carpeta que no es un plugin de Musubi no cubre nada.
func elPluginEnCubre(dir, fuente string) bool {
	if dir == "" || !esPluginDeMusubi(dir) {
		return false
	}
	crudo, err := os.ReadFile(filepath.Join(dir, "hooks", "hooks.json"))
	if err != nil {
		return false
	}
	return arranqueDeMusubiCubre(crudo, fuente)
}

// dirDelPluginInstalado es el plugin que Claude Code carga: el que lanzó este proceso, si lo lanzó
// uno, y si no el de la carpeta de configuración de Claude Code.
func dirDelPluginInstalado() string {
	if d := os.Getenv("CLAUDE_PLUGIN_ROOT"); d != "" {
		return d
	}
	d, _ := dirDelPlugin()
	return d
}

// compactCubierto dice si algo ata el hook de arranque de Musubi a la compactación: el settings de
// alguna de las carpetas dadas, o el plugin instalado. No mira el settings del usuario
// (~/.claude/settings.json) ni si el plugin está deshabilitado en enabledPlugins.
func compactCubierto(dirs ...string) bool {
	for _, d := range dirs {
		if d != "" && elProyectoYaTieneElArranque(d, fuenteCompact) {
			return true
		}
	}
	return elPluginEnCubre(dirDelPluginInstalado(), fuenteCompact)
}

// estadoDeLaCompactacion es la línea de `musubi agente estado` sobre la compactación en el repo:
// si su settings o el plugin en pluginDir devuelven la memoria después de compactar.
func estadoDeLaCompactacion(repo, pluginDir string) string {
	switch {
	case elProyectoYaTieneElArranque(repo, fuenteCompact):
		return fmt.Sprintf("Compactación: cubierta en %s (SessionStart «compact» en su .claude/settings): tras compactar, Musubi devuelve la memoria que el resumen perdió.", repo)
	case elPluginEnCubre(pluginDir, fuenteCompact):
		return "Compactación: cubierta por el plugin (SessionStart «compact» en su hooks/hooks.json): tras compactar, Musubi devuelve la memoria que el resumen perdió."
	default:
		return fmt.Sprintf("Compactación: SIN CUBRIR en %s: ni su .claude/settings ni el plugin tienen el hook SessionStart «compact», y después de compactar la memoria que el resumen pierde no vuelve. "+
			"Lo instala `musubi setup` en el repo, o `musubi agente instalar` para el plugin.", repo)
	}
}
