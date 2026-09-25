package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// TestElAdjudicadorSoloPuedeLeerYJuzgar — el adjudicador B1 corre solo, de noche, sin nadie mirando.
//
// Es Claude Code sin interfaz con `--permission-mode dontAsk`: todo lo que no esté en
// `--allowedTools` se niega sin preguntar. Así que la lista ES la frontera de lo que puede hacerle a
// la memoria, y tiene que ser exactamente leer la cola, leer las observaciones y juzgar. Una
// herramienta de más (guardar, borrar, archivar, `Bash`) y un prompt confundido deja de ser un
// veredicto equivocado —que se re-juzga— para ser memoria escrita o borrada por nadie.
//
// Se mira la lista DEFINIDA, que la invocación use ESA lista y no otra, y el modo. Las tres cosas,
// porque cualquiera de las tres sola se puede esquivar con las otras dos.
//
// Sabotaje que la hace fallar: darle una herramienta que escribe memoria.
// arnes: archivo="deploy/adjudicador/b1-adjudicar.sh"
// arnes: de="mcp__${SERVIDOR}__musubi_judge\""
// arnes: a="mcp__${SERVIDOR}__musubi_judge,mcp__${SERVIDOR}__musubi_save_observation\""
//
// Sabotaje que la hace fallar: pasarle a la invocación algo más que la lista.
// arnes: archivo="deploy/adjudicador/b1-adjudicar.sh"
// arnes: de="--allowedTools \"$HERRAMIENTAS\""
// arnes: a="--allowedTools \"$HERRAMIENTAS,Bash\""
//
// Sabotaje que la hace fallar: sacarle el modo que niega lo que no está en la lista.
// arnes: archivo="deploy/adjudicador/b1-adjudicar.sh"
// arnes: de="--permission-mode dontAsk"
// arnes: a="--permission-mode bypassPermissions"
func TestElAdjudicadorSoloPuedeLeerYJuzgar(t *testing.T) {
	codigo := leerDeploy(t, "adjudicador", "b1-adjudicar.sh")

	defs := regexp.MustCompile(`(?m)^HERRAMIENTAS="([^"]*)"$`).FindAllStringSubmatch(codigo, -1)
	if len(defs) != 1 {
		t.Fatalf("tiene que haber exactamente UNA definición de HERRAMIENTAS en el código; hay %d", len(defs))
	}
	var tiene []string
	for _, h := range strings.Split(defs[0][1], ",") {
		tiene = append(tiene, strings.TrimSpace(h))
	}
	sort.Strings(tiene)
	quiere := []string{
		"mcp__${SERVIDOR}__musubi_conflicts",
		"mcp__${SERVIDOR}__musubi_judge",
		"mcp__${SERVIDOR}__musubi_memory_expand",
	}
	if strings.Join(tiene, ",") != strings.Join(quiere, ",") {
		t.Errorf("el adjudicador puede usar %v y tiene que poder usar EXACTAMENTE %v: leer la cola, leer las "+
			"observaciones y juzgar. Cualquier otra herramienta le deja escribir o borrar memoria sin nadie mirando", tiene, quiere)
	}

	invs := regexp.MustCompile(`--allowedTools "([^"]*)"`).FindAllStringSubmatch(codigo, -1)
	if len(invs) != 1 || invs[0][1] != "$HERRAMIENTAS" {
		t.Errorf("la invocación de claude tiene que pasar --allowedTools \"$HERRAMIENTAS\" y nada más; encontré %v", invs)
	}

	modos := regexp.MustCompile(`--permission-mode (\S+)`).FindAllStringSubmatch(codigo, -1)
	if len(modos) != 1 || modos[0][1] != "dontAsk" {
		t.Errorf("el modo tiene que ser dontAsk (niega sin preguntar lo que no está en la lista); encontré %v", modos)
	}
	if strings.Contains(codigo, "--dangerously-skip-permissions") {
		t.Error("el adjudicador corre sin nadie mirando: --dangerously-skip-permissions le abre todas las herramientas")
	}
}

// TestElAdjudicadorLocalFijaSuMemoria — el mcp.json que escribe el instalador fija MUSUBI_HOME.
//
// Sin esa variable, `musubi daemon` toma el workspace del directorio desde el que arranca, y el timer
// arranca en $HOME: ahí `ensureWorkspace` planta un config y una memoria NUEVOS. El adjudicador
// juzgaría una cola vacía de una memoria que nadie usa, todo en verde, mientras la de verdad se
// llena. Es la sombra de A96, que ya pasó una vez.
//
// No se busca el texto: se CORRE el trozo de python que escribe el archivo y se lee lo que escribió.
//
// Sabotaje que la hace fallar: no pasarle MUSUBI_HOME al daemon.
// arnes: archivo="deploy/adjudicador/instalar-adjudicador-local.sh"
// arnes: de="\"env\": {\"MUSUBI_HOME\": repo},"
// arnes: a="\"env\": {},"
func TestElAdjudicadorLocalFijaSuMemoria(t *testing.T) {
	compuerta := guiones.Exigir(t, "corre el bloque del instalador del adjudicador local que escribe el "+
		"mcp.json, que es de una máquina Linux con systemd de usuario", "bash", "python3")

	// El bloque entero, TAL CUAL: la invocación de python3 con sus argumentos y el heredoc hasta su
	// cierre. Correr sólo el python con argumentos armados acá probaría una copia de la invocación,
	// no la invocación.
	codigo := leerDeploy(t, "adjudicador", "instalar-adjudicador-local.sh")
	lineas := strings.Split(codigo, "\n")
	inicio, fin := -1, -1
	for i, l := range lineas {
		if inicio < 0 && strings.HasPrefix(l, "python3 - \"$DESTINO/mcp.json\"") && strings.HasSuffix(l, "<<'PY'") {
			inicio = i
			continue
		}
		if inicio >= 0 && l == "PY" {
			fin = i
			break
		}
	}
	if inicio < 0 || fin < 0 {
		t.Fatalf("no encuentro en el instalador el bloque de python que escribe el mcp.json")
	}
	bloque := strings.Join(lineas[inicio:fin+1], "\n") + "\n"
	if !strings.Contains(bloque, "mcpServers") {
		t.Fatalf("el bloque extraído no es el que escribe el mcp.json:\n%s", bloque)
	}

	dir := t.TempDir()
	destino := filepath.Join(dir, "mcp.json")
	const binario, repo, servidor = "/opt/musubi/bin/musubi", "/home/alguien/musubi", "musubi"
	cmd := compuerta.Comando("bash", "-c", "set -euo pipefail\n"+bloque)
	cmd.Env = append(os.Environ(), "DESTINO="+dir, "BIN="+binario, "REPO="+repo, "SERVIDOR="+servidor)
	if salida, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("el bloque del instalador falló: %v\n%s", err, salida)
	}
	crudo, err := os.ReadFile(destino)
	if err != nil {
		t.Fatalf("el instalador no escribió el mcp.json: %v", err)
	}
	var cfg struct {
		McpServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(crudo, &cfg); err != nil {
		t.Fatalf("el mcp.json no es JSON: %v\n%s", err, crudo)
	}
	s, ok := cfg.McpServers[servidor]
	if !ok {
		t.Fatalf("el mcp.json no declara el servidor %q, que es el que nombran las herramientas permitidas:\n%s", servidor, crudo)
	}
	if s.Env["MUSUBI_HOME"] != repo {
		t.Errorf("el daemon del adjudicador no tiene MUSUBI_HOME=%s (tiene %q): arrancando desde $HOME juzgaría una "+
			"memoria sombra vacía, en verde, mientras la de verdad se llena", repo, s.Env["MUSUBI_HOME"])
	}
	if s.Command != binario || strings.Join(s.Args, " ") != "daemon" {
		t.Errorf("el mcp.json tiene que lanzar `%s daemon`; lanza %q %v", binario, s.Command, s.Args)
	}
}

// TestElTimerDelAdjudicadorLocalRecuperaLaCorridaPerdida — una laptop se apaga.
//
// Los tres disparos del día caen a horas fijas, y una laptop apagada a esa hora se los pierde para
// siempre salvo con `Persistent=true`, que corre la corrida perdida al prender. Sin eso, una
// máquina que se usa de día y se apaga de noche podría no adjudicar nunca.
//
// Sabotaje que la hace fallar: sacarle Persistent al timer.
// arnes: archivo="deploy/adjudicador/instalar-adjudicador-local.sh"
// arnes: de="Persistent=true\n"
// arnes: a="Persistent=false\n"
func TestElTimerDelAdjudicadorLocalRecuperaLaCorridaPerdida(t *testing.T) {
	codigo := leerDeploy(t, "adjudicador", "instalar-adjudicador-local.sh")
	bloque := regexp.MustCompile(`(?s)musubi-adjudicador\.timer" <<'EOF'\n(.*?)\nEOF\n`).FindStringSubmatch(codigo)
	if bloque == nil {
		t.Fatalf("no encuentro el timer que escribe el instalador")
	}
	if !regexp.MustCompile(`(?m)^Persistent=true$`).MatchString(bloque[1]) {
		t.Errorf("el timer del adjudicador local no tiene Persistent=true: una laptop apagada a la hora del "+
			"disparo pierde la corrida para siempre\n%s", bloque[1])
	}
}
