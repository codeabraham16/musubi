package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"musubi/internal/config"
)

// ganchosDeUnSettings lee la clave "hooks" de un settings.json (o de un hooks.json de plugin, que
// tiene la misma forma) y devuelve cada gancho como «evento|matcher|subcomando», sin el binario: lo
// que tiene que coincidir entre setup y el plugin es QUÉ se corre y CUÁNDO, no desde qué ruta.
func ganchosDeUnSettings(t *testing.T, crudo []byte) []string {
	t.Helper()
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(crudo, &doc); err != nil {
		t.Fatalf("no es un settings de hooks: %v\n%s", err, crudo)
	}
	var out []string
	for evento, grupos := range doc.Hooks {
		for _, g := range grupos {
			for _, h := range g.Hooks {
				sub := h.Command
				if strings.HasPrefix(sub, `"`) {
					if i := strings.Index(sub[1:], `"`); i >= 0 {
						sub = strings.TrimSpace(sub[i+2:])
					}
				} else if i := strings.Index(sub, " "); i >= 0 {
					sub = strings.TrimSpace(sub[i+1:])
				}
				out = append(out, evento+"|"+g.Matcher+"|"+sub)
			}
		}
	}
	sort.Strings(out)
	return out
}

// EL PLUGIN CORRE LOS MISMOS HOOKS QUE `musubi setup` ESCRIBE EN CADA REPO.
//
// La lista del plugin (ganchosDelAgente) y la de setup (cuatro escritores sueltos en setup.go y en
// provision.go) son dos lugares que describen lo mismo, y dos listas así se separan: un hook nuevo
// entra por setup y el plugin no lo corre, o al revés. La fuente de verdad acá es setup DE PUNTA A
// PUNTA —se corre el comando real en una carpeta de descarte y se lee lo que dejó—, no una tercera
// lista escrita en la prueba.
//
// Sabotaje que la hace fallar: que el plugin deje de correr la captura del Stop.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t{\"Stop\", \"\", \"capture --hook-mode\"},\n"
// arnes: a=""
func TestElPluginLlevaLosMismosGanchosQueSetup(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	correrMusubi(t, repo, home, "", "setup", "--agent", "claude")
	deSetup, err := os.ReadFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile))
	if err != nil {
		t.Fatalf("setup no dejó .claude/settings.json: %v", err)
	}
	delPlugin, err := ganchosDelPlugin("/opt/musubi/musubi")
	if err != nil {
		t.Fatal(err)
	}

	quiere, tiene := ganchosDeUnSettings(t, deSetup), ganchosDeUnSettings(t, delPlugin)
	if len(quiere) == 0 {
		t.Fatal("setup no escribió ningún hook: la prueba no estaría comparando nada")
	}
	if strings.Join(quiere, "\n") != strings.Join(tiene, "\n") {
		t.Errorf("el plugin y setup no corren los mismos hooks.\nsetup:\n  %s\nplugin:\n  %s",
			strings.Join(quiere, "\n  "), strings.Join(tiene, "\n  "))
	}
}

// INSTALAR, CONSULTAR Y QUITAR: Y NUNCA PISAR NI BORRAR LO QUE NO ES DE MUSUBI.
//
// La carpeta es ~/.claude/skills/musubi, donde alguien podría tener una skill propia con ese nombre.
// La marca .musubi-plugin es lo único que la vuelve de Musubi: sin ella no se pisa ni se borra.
//
// Sabotaje que la hace fallar: que quitar borre sin mirar la marca.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\tif !esPluginDeMusubi(dir) {\n\t\treturn fmt.Errorf(\"%s no es un plugin de Musubi (le falta %s): no lo borro\""
// arnes: a="\tif false {\n\t\treturn fmt.Errorf(\"%s no es un plugin de Musubi (le falta %s): no lo borro\""
//
// Sabotaje que la hace fallar: que instalar pise una carpeta ajena.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="len(entradas) > 0 && !esPluginDeMusubi(dir) {"
// arnes: a="len(entradas) > 0 && false {"
func TestInstalarConsultarYQuitarElPlugin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "musubi")
	exe := "/opt/musubi/musubi"
	if err := instalarPlugin(dir, exe, "1.2.3"); err != nil {
		t.Fatalf("instalar: %v", err)
	}

	var manifiesto map[string]interface{}
	crudo, err := os.ReadFile(filepath.Join(dir, ".claude-plugin", "plugin.json"))
	if err != nil || json.Unmarshal(crudo, &manifiesto) != nil {
		t.Fatalf("plugin.json ilegible: %v\n%s", err, crudo)
	}
	if manifiesto["name"] != "musubi" || manifiesto["mcpServers"] != "./.mcp.json" || manifiesto["hooks"] != "./hooks/hooks.json" {
		t.Errorf("plugin.json no declara nombre, servidor y hooks como Claude Code los lee: %s", crudo)
	}
	var mcp struct {
		McpServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	crudo, _ = os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if json.Unmarshal(crudo, &mcp) != nil || mcp.McpServers["musubi"].Command != exe || strings.Join(mcp.McpServers["musubi"].Args, " ") != "daemon" {
		t.Errorf("el .mcp.json del plugin no lanza `%s daemon`: %s", exe, crudo)
	}
	for _, sk := range cognitiveSkills(nil) {
		if _, err := os.Stat(filepath.Join(dir, "skills", sk.Name, "SKILL.md")); err != nil {
			t.Errorf("el plugin no trae la skill %s: %v", sk.Name, err)
		}
	}
	if e := estadoDelPlugin(dir, "1.2.3"); !strings.HasPrefix(e, "INSTALADO") {
		t.Errorf("estado con la misma versión: %q", e)
	}
	if e := estadoDelPlugin(dir, "1.2.4"); !strings.HasPrefix(e, "DESACTUALIZADO") {
		t.Errorf("estado con otra versión: %q", e)
	}

	if err := quitarPlugin(dir); err != nil {
		t.Fatalf("quitar: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("quitar no borró el plugin")
	}

	// Una carpeta ajena con ese nombre: ni se pisa ni se borra.
	ajena := filepath.Join(t.TempDir(), "musubi")
	if err := os.MkdirAll(ajena, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ajena, "SKILL.md"), []byte("---\nname: musubi\n---\nmía\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := instalarPlugin(ajena, exe, "1.2.3"); err == nil {
		t.Error("instalar pisó una carpeta que no es de Musubi")
	}
	if err := quitarPlugin(ajena); err == nil {
		t.Error("quitar aceptó borrar una carpeta que no es de Musubi")
	}
	if _, err := os.Stat(filepath.Join(ajena, "SKILL.md")); err != nil {
		t.Errorf("la skill ajena desapareció: %v", err)
	}
}

// proyectoConMemoria arma un repo git con memoria de Musubi y el aviso de durable en el turno 1,
// para que el hook del turno tenga algo que decir y su silencio signifique algo.
func proyectoConMemoria(t *testing.T, home string) string {
	t.Helper()
	repo := filepath.Join(home, "proyecto")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Loop.DurableNudgeAfterTurns = 1
	cfg.Update.CheckIntervalHours = -1
	contenido, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, config.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, config.DirName, config.ConfigFile), contenido, 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// DONDE EL PROYECTO YA CONECTA MUSUBI, EL PLUGIN SE HACE A UN LADO.
//
// Claude Code corre los hooks del proyecto Y los del plugin, y levanta los dos servidores: sin esta
// regla, en este repo y en Altura —que se cablearon repo por repo— el turno inyectaría todo dos
// veces, la captura correría dos veces, y el agente vería dos catálogos de Musubi contra la misma
// memoria. Cada mitad tiene su control: sin el cableado del proyecto, el plugin SÍ trabaja.
//
// Sabotaje que la hace fallar: que el servidor del plugin no se haga a un lado.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tif corriendoComoPlugin() && elProyectoYaConectaMusubi(carpetaDeLaSesion()) {"
// arnes: a="\tif false {"
//
// Sabotaje que la hace fallar: que el hook del turno del plugin no ceda.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de=" || elPluginCedeElGancho(\"turn\") {"
// arnes: a=" {"
// arnes: colision_ok="TestSinProyectoNadaCreaMemoria"
func TestElPluginSeHaceAUnLadoDondeElProyectoYaConectaMusubi(t *testing.T) {
	home := t.TempDir()
	repo := proyectoConMemoria(t, home)
	comoPlugin := []string{"CLAUDE_PLUGIN_ROOT=" + filepath.Join(home, ".claude", "skills", "musubi")}
	evento := `{"session_id":"s","prompt":"armá el plan del cambio de sincronización"}`

	// CONTROL: sin cableado propio, el plugin trabaja.
	res := respuestasJSONRPC(t, correrMusubiCon(t, repo, home, handshakeYLista, comoPlugin, "daemon"))
	lista, _ := res[2]["result"].(map[string]interface{})
	if tools, _ := lista["tools"].([]interface{}); len(tools) == 0 {
		t.Fatal("control: sin .mcp.json propio, el servidor del plugin tiene que ofrecer las tools")
	}
	if out := correrMusubiCon(t, repo, home, evento, comoPlugin, "turn", "--hook-mode"); !strings.Contains(out, "bajá lo durable") {
		t.Fatalf("control: sin hooks propios, el turno del plugin tiene que avisar; salió %q", out)
	}

	// Con el cableado del proyecto: el plugin se calla.
	if err := os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte(`{"mcpServers":{"musubi":{"command":"musubi","args":["daemon"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, config.ClaudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	settings := `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"musubi turn --hook-mode","timeout":10}]}]}}`
	if err := os.WriteFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	res = respuestasJSONRPC(t, correrMusubiCon(t, repo, home, handshakeYLista, comoPlugin, "daemon"))
	init, _ := res[1]["result"].(map[string]interface{})
	if _, habla := init["instructions"]; habla {
		t.Errorf("el servidor del plugin le habló al agente en un proyecto que ya conecta Musubi: %v", init["instructions"])
	}
	lista, _ = res[2]["result"].(map[string]interface{})
	if tools, _ := lista["tools"].([]interface{}); len(tools) != 0 {
		t.Errorf("el servidor del plugin ofreció %d tools en un proyecto que ya conecta Musubi: dos catálogos", len(tools))
	}
	// Otra sesión, para que el durable del turno 1 vuelva a tocar si el hook no cediera.
	if out := correrMusubiCon(t, repo, home, strings.Replace(evento, `"s"`, `"s2"`, 1), comoPlugin, "turn", "--hook-mode"); out != "" {
		t.Errorf("el turno del plugin inyectó en un proyecto que ya corre ese hook: %q", out)
	}
}

// EL DAEMON DEL PLUGIN LE NOMBRA AL AGENTE LAS SKILLS DEL PLUGIN, CON EL NOMBRE DEL MANIFIESTO.
//
// En un repo sin setup, las skills del plugin son las únicas que el agente tiene, y antes el mapa
// salía vacío ahí: preguntaba sólo por las exportadas del proyecto. Se mide sobre el proceso, con el
// plugin instalado por instalarPlugin, porque el cableado vive en runDaemon: las pruebas de
// internal/mcp fijan qué hace la option y ésta que el daemon la pase, con las cinco skills reales
// que cubren cada alcance y dentro del tope del cliente.
//
// «musubi:» se escribe literal: es cómo Claude Code llama a las skills del plugin (medido el
// 2026-09-25). Y el prefijo sale del manifiesto instalado: con otro `name`, otro prefijo.
//
// Sabotaje que la hace fallar: que el daemon no pase las skills del plugin.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="skillsDelPluginParaElMapa(), mcp.WithVersion(version))"
// arnes: a="mcp.WithVersion(version))"
//
// Sabotaje que la hace fallar: tomar el prefijo de la constante y no del manifiesto.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="dirSkillsDelPlugin), nombre, cognitiveSkills(nil))\n"
// arnes: a="dirSkillsDelPlugin), nombrePlugin, cognitiveSkills(nil))\n"
func TestElDaemonDelPluginNombraLasSkillsDelPlugin(t *testing.T) {
	home := t.TempDir()
	repo := proyectoConMemoria(t, home)
	plugin := filepath.Join(home, ".claude", "skills", "musubi")
	if err := instalarPlugin(plugin, "musubi", "0.0.0-prueba"); err != nil {
		t.Fatalf("instalar el plugin: %v", err)
	}
	instrucciones := func(extra []string) string {
		t.Helper()
		res := respuestasJSONRPC(t, correrMusubiCon(t, repo, home, handshakeYLista, extra, "daemon"))
		init, _ := res[1]["result"].(map[string]interface{})
		texto, _ := init["instructions"].(string)
		if texto == "" {
			t.Fatalf("el daemon no le habló al agente: %v", res[1])
		}
		return texto
	}
	cinco := []string{"adversarial-review", "audit-structure-flow", "orchestrate-multiagent", "plan-ahead", "sdd-flow"}

	// CONTROL: el mismo repo sin plugin no tiene skills exportadas, y no hay mapa.
	if texto := instrucciones(nil); strings.Contains(texto, "plan-ahead") {
		t.Fatalf("control: sin plugin y sin skills exportadas el mapa nombra plan-ahead:\n%s", texto)
	}

	texto := instrucciones([]string{"CLAUDE_PLUGIN_ROOT=" + plugin})
	for _, skill := range cinco {
		if !strings.Contains(texto, "musubi:"+skill) {
			t.Errorf("el mapa del plugin no nombra musubi:%s:\n%s", skill, texto)
		}
	}
	if n := len(utf16.Encode([]rune(texto))); n > 2048 {
		t.Errorf("las instrucciones miden %d unidades UTF-16 y Claude Code corta en 2048", n)
	}

	// Con otro nombre en el manifiesto, Claude Code las llama de otra forma, y el mapa también.
	manifiesto := filepath.Join(plugin, ".claude-plugin", "plugin.json")
	crudo, err := os.ReadFile(manifiesto)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifiesto, []byte(strings.Replace(string(crudo), `"name": "musubi"`, `"name": "otro"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if texto := instrucciones([]string{"CLAUDE_PLUGIN_ROOT=" + plugin}); !strings.Contains(texto, "otro:plan-ahead") {
		t.Errorf("con el plugin llamado «otro» el mapa no nombra otro:plan-ahead:\n%s", texto)
	}
}
