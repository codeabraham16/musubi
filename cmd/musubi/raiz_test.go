package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/guiones"
)

// arbolDePrueba arma, bajo un home de prueba, los casos que la regla de raiz.go tiene que distinguir.
// Devuelve el home.
func arbolDePrueba(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	escribir := func(rel, contenido string) {
		p := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// El home es un repo git (dotfiles) y tiene la sombra de A96: ninguna de las dos lo vuelve proyecto.
	mk(".git")
	escribir(".musubi/config.yaml", "sombra: true\n")
	mk("suelta/adentro")

	// Un proyecto con memoria, abierto desde una subcarpeta y desde un worktree ANIDADO.
	mk("proyecto/.git/worktrees/hermano")
	escribir("proyecto/.musubi/config.yaml", "project_id: proyecto\n")
	mk("proyecto/sub/dir")
	escribir("proyecto/.claude/worktrees/anidado/.git", "gitdir: ../../../.git/worktrees/anidado\n")

	// Un worktree HERMANO, afuera del repo: apunta al .git del proyecto.
	escribir("proyecto/.git/worktrees/hermano/commondir", "../..\n")
	escribir("hermano/.git", "gitdir: "+filepath.Join(home, "proyecto", ".git", "worktrees", "hermano")+"\n")

	// Un repo sin memoria, y uno que sólo trae el config.example.yaml versionado.
	mk("repo-nuevo/.git")
	mk("repo-nuevo/pkg")
	mk("repo-ejemplo/.git")
	escribir("repo-ejemplo/.musubi/config.example.yaml", "# ejemplo\n")
	return home
}

// LA REGLA DE LA RAÍZ: DÓNDE TRABAJA UN PROCESO QUE ARRANCÓ SOLO, Y DÓNDE NO HAY PROYECTO.
//
// Cada caso es una situación real de una sesión que arranca sola cuando Musubi está registrado a
// nivel usuario: en el home, en una subcarpeta, en un worktree adentro y afuera del repo, en un repo
// que nunca vio a Musubi. El home es un repo git con la sombra de A96 adentro, a propósito: es el
// peor caso, y ninguna de las dos cosas lo puede volver proyecto.
//
// Sabotaje que la hace fallar: aceptar la memoria del home al subir.
// arnes: archivo="cmd/musubi/raiz.go"
// arnes: de="\t\tif !mismoDir(d, home) && tieneMemoria(d) {"
// arnes: a="\t\tif tieneMemoria(d) {"
//
// Sabotaje que la hace fallar: aceptar un repo git en el home.
// arnes: archivo="cmd/musubi/raiz.go"
// arnes: de="\tif mismoDir(top, home) {"
// arnes: a="\tif false {"
//
// Sabotaje que la hace fallar: contar como memoria un `.musubi/` sin config.yaml.
// arnes: archivo="cmd/musubi/raiz.go"
// arnes: de="\tst, err := os.Stat(filepath.Join(dir, config.DirName, config.ConfigFile))\n\treturn err == nil && !st.IsDir()"
// arnes: a="\t_, err := os.Stat(filepath.Join(dir, config.DirName))\n\treturn err == nil"
//
// Sabotaje que la hace fallar: no subir a buscar la memoria del proyecto.
// arnes: archivo="cmd/musubi/raiz.go"
// arnes: de="\t\tpadre := filepath.Dir(d)\n\t\tif padre == d {\n\t\t\tbreak\n\t\t}\n\t\td = padre\n\t}\n\ttop, ok := raizGit(base)"
// arnes: a="\t\tbreak\n\t}\n\ttop, ok := raizGit(base)"
//
// Sabotaje que la hace fallar: que el worktree hermano no use la memoria del principal.
// arnes: archivo="cmd/musubi/raiz.go"
// arnes: de="\tif principal, ok := principalDelWorktree(top); ok && "
// arnes: a="\tif principal, ok := principalDelWorktree(top); false && ok && "
func TestLaRaizDeUnProcesoQueArrancoSolo(t *testing.T) {
	home := arbolDePrueba(t)
	en := func(rel string) string { return filepath.Join(home, filepath.FromSlash(rel)) }
	casos := []struct {
		nombre, base string
		dir          string // "" ⇒ inerte
		activar      bool
	}{
		{"el home no es un proyecto, aunque sea un repo y tenga la sombra", "", "", false},
		{"una carpeta suelta del home queda inerte", "suelta/adentro", "", false},
		{"una subcarpeta usa la memoria del proyecto", "proyecto/sub/dir", "proyecto", false},
		{"un worktree anidado usa la memoria del proyecto", "proyecto/.claude/worktrees/anidado", "proyecto", false},
		{"un worktree hermano usa la memoria del principal", "hermano", "proyecto", false},
		{"un repo sin memoria se activa en su raíz", "repo-nuevo/pkg", "repo-nuevo", true},
		{"un config.example.yaml no es memoria", "repo-ejemplo", "repo-ejemplo", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			r := resolverRaiz(en(c.base), home)
			quiere := ""
			if c.dir != "" {
				quiere = en(c.dir)
			}
			if !mismoDir(r.Dir, quiere) && !(r.Dir == "" && quiere == "") {
				t.Errorf("raíz = %q (%s), quería %q", r.Dir, r.Motivo, quiere)
			}
			if r.Activar != c.activar {
				t.Errorf("activar = %v (%s), quería %v", r.Activar, r.Motivo, c.activar)
			}
		})
	}
}

// TestHelperMusubi no es un test: es el proceso hijo que corre el `main` de verdad con los
// argumentos de GO_MUSUBI_ARGS (un array JSON: el entorno no admite un separador NUL).
func TestHelperMusubi(t *testing.T) {
	crudo := os.Getenv("GO_MUSUBI_ARGS")
	if crudo == "" {
		t.Skip("helper, no es un test")
	}
	var args []string
	if err := json.Unmarshal([]byte(crudo), &args); err != nil {
		t.Fatalf("GO_MUSUBI_ARGS no es un array JSON: %v", err)
	}
	// Recién acá se muda a la carpeta de la prueba: el TestMain de este binario ya corrió la guarda
	// del presupuesto parado en el paquete, que es donde encuentra su política (ver correrMusubiCon).
	if d := os.Getenv("GO_MUSUBI_CWD"); d != "" {
		if err := os.Chdir(d); err != nil {
			t.Fatalf("no pude pararme en %s: %v", d, err)
		}
	}
	os.Args = append([]string{"musubi"}, args...)
	main()
	os.Exit(0)
}

// correrMusubi corre `musubi <args…>` como proceso, parado en dir, con ese home y sin MUSUBI_HOME.
// HTTPS_PROXY a un puerto cerrado: si algo intenta salir a la red (el chequeo de versión del
// daemon), falla en el acto y en local.
func correrMusubi(t *testing.T, dir, home, stdin string, args ...string) string {
	t.Helper()
	return correrMusubiCon(t, dir, home, stdin, nil, args...)
}

// correrMusubiCon es correrMusubi con variables de entorno extra (p. ej. CLAUDE_PLUGIN_ROOT para
// correr como lo lanza el plugin). Las que decide la prueba se sacan del entorno heredado.
func correrMusubiCon(t *testing.T, dir, home, stdin string, extra []string, args ...string) string {
	t.Helper()
	// EL HIJO ARRANCA PARADO EN EL PAQUETE Y SE MUDA DESPUÉS (GO_MUSUBI_CWD). Es este mismo binario
	// de prueba, y su TestMain corre la guarda del presupuesto, que bajo -race lee la política desde
	// el go.mod del cwd: arrancado en la carpeta de descarte, la guarda lo mataba antes de correr
	// nada («no se encontró go.mod»). -test.timeout=0 porque el hijo no hereda el -timeout del padre
	// y la guarda no aceptaría el default; el límite de verdad lo pone la corrida del padre.
	cmd := guiones.Herramienta(t, os.Args[0], "-test.run=^TestHelperMusubi$", "-test.timeout=0")
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "MUSUBI_HOME=") || strings.HasPrefix(kv, "CLAUDE_PROJECT_DIR=") || strings.HasPrefix(kv, "HOME=") ||
			strings.HasPrefix(kv, "CLAUDE_PLUGIN_ROOT=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, extra...)
	argsJSON, _ := json.Marshal(args)
	cmd.Env = append(env, "GO_MUSUBI_ARGS="+string(argsJSON), "HOME="+home, "USERPROFILE="+home,
		"CLAUDE_PROJECT_DIR="+dir, "HTTPS_PROXY=http://127.0.0.1:9", "HTTP_PROXY=http://127.0.0.1:9")
	cmd.Env = append(cmd.Env, "GO_MUSUBI_CWD="+dir)
	cmd.Stdin = strings.NewReader(stdin)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("musubi %v terminó mal: %v\n  stdout=%.300q\n  stderr=%s", args, err, out, errBuf.String())
	}
	return string(out)
}

// respuestasJSONRPC separa la salida de un daemon por id.
func respuestasJSONRPC(t *testing.T, out string) map[float64]map[string]interface{} {
	t.Helper()
	res := map[float64]map[string]interface{}{}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" {
			continue
		}
		var r map[string]interface{}
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("salida que no es JSON-RPC: %v\n  %.200s", err, l)
		}
		id, _ := r["id"].(float64)
		res[id] = r
	}
	return res
}

const handshakeYLista = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
`

// DONDE NO HAY PROYECTO, NINGÚN PROCESO CREA MEMORIA, Y EL DAEMON HABLA INERTE.
//
// Se mide sobre los procesos reales: el defecto vivía en los puntos de entrada (workspaceDir +
// ensureWorkspace / NewDbEngine), no en una función que se pueda llamar suelta. En una carpeta suelta
// del home corren el daemon y los cuatro hooks, y después se mira el disco: no puede haber aparecido
// un `.musubi` en ningún lado.
//
// Sabotaje que la hace fallar: que el daemon vuelva a crear memoria donde lo arranquen.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tr := raizDelProceso()\n\tif r.Dir == \"\" {\n\t\tfmt.Fprintf(os.Stderr, \"musubi: sin proyecto en esta carpeta"
// arnes: a="\tr := raizResuelta{Dir: workspaceDir(), Activar: true}\n\tif r.Dir == \"\" {\n\t\tfmt.Fprintf(os.Stderr, \"musubi: sin proyecto en esta carpeta"
//
// Sabotaje que la hace fallar: que el hook del turno vuelva a abrir (y crear) la memoria donde esté.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tr := raizDelProceso()\n\tif r.Dir == \"\" || r.Activar || elPluginCedeElGancho(\"turn\") {\n\t\treturn\n\t}\n\troot := r.Dir\n\tcfg, _ := config.Load(root)"
// arnes: a="\troot := workspaceDir()\n\tcfg, _ := config.Load(root)"
// arnes: colision_ok="TestElPluginSeHaceAUnLadoDondeElProyectoYaConectaMusubi"
//
// Sabotaje que la hace fallar: que el hook de arranque vuelva a trabajar donde lo arranquen.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="\t\tr := raizDelProceso()"
// arnes: a="\t\tr := raizResuelta{Dir: workspaceDir()}"
//
// Sabotaje que la hace fallar: que el hook de captura vuelva a crear memoria donde lo arranquen.
// arnes: archivo="cmd/musubi/capture.go"
// arnes: de="\t\tr := raizDelProceso()"
// arnes: a="\t\tr := raizResuelta{Dir: workspaceDir()}"
func TestSinProyectoNadaCreaMemoria(t *testing.T) {
	home := t.TempDir()
	suelta := filepath.Join(home, "Descargas")
	if err := os.MkdirAll(suelta, 0o755); err != nil {
		t.Fatal(err)
	}

	res := respuestasJSONRPC(t, correrMusubi(t, suelta, home, handshakeYLista, "daemon"))
	init, _ := res[1]["result"].(map[string]interface{})
	if texto, _ := init["instructions"].(string); !strings.Contains(texto, "no está activo") {
		t.Errorf("el daemon inerte no le dijo al agente que Musubi no está activo acá: instructions=%q", texto)
	}
	lista, _ := res[2]["result"].(map[string]interface{})
	if tools, _ := lista["tools"].([]interface{}); len(tools) != 0 {
		t.Errorf("el daemon inerte ofreció %d tools: sin proyecto no tiene ninguna que funcione", len(tools))
	}

	evento := `{"session_id":"s","prompt":"hola","tool_name":"Read","tool_input":{"file_path":"x.go"}}`
	for _, hook := range [][]string{{"turn", "--hook-mode"}, {"detect", "--hook-mode"}, {"precheck", "--hook-mode"}, {"capture", "--hook-mode"}} {
		correrMusubi(t, suelta, home, evento, hook...)
	}

	for _, d := range []string{suelta, home} {
		if _, err := os.Stat(filepath.Join(d, config.DirName)); err == nil {
			t.Errorf("apareció %s: un proceso creó memoria donde no hay proyecto", filepath.Join(d, config.DirName))
		}
	}
}

// EN UN REPO QUE NUNCA VIO A MUSUBI, EL DAEMON LO ACTIVA SOLO Y SIN ENSUCIAR EL REPO.
//
// Es el «por defecto»: el agente tiene memoria en cualquier proyecto sin que nadie corra setup. La
// memoria se crea en la RAÍZ del repo aunque la sesión arranque en una subcarpeta, y trae su propio
// `.gitignore` para que activarse solo no deje archivos nuevos en el `git status` de nadie.
//
// Sabotaje que la hace fallar: activar sin el `.gitignore` propio.
// arnes: archivo="cmd/musubi/raiz.go"
// arnes: de="\tif os.IsNotExist(errAntes) {"
// arnes: a="\tif false && os.IsNotExist(errAntes) {"
func TestUnRepoNuevoSeActivaSoloYSinEnsuciar(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "proyectos", "nuevo")
	sub := filepath.Join(repo, "internal")
	for _, d := range []string{filepath.Join(repo, ".git"), sub} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	res := respuestasJSONRPC(t, correrMusubi(t, sub, home, handshakeYLista, "daemon"))
	lista, _ := res[2]["result"].(map[string]interface{})
	if tools, _ := lista["tools"].([]interface{}); len(tools) == 0 {
		t.Errorf("el daemon no ofreció tools en un repo git: no se activó")
	}
	if !tieneMemoria(repo) {
		t.Fatalf("el daemon no creó la memoria en la raíz del repo (%s)", repo)
	}
	if _, err := os.Stat(filepath.Join(sub, config.DirName)); err == nil {
		t.Errorf("la memoria se creó en la subcarpeta de la sesión y no en la raíz del repo")
	}
	ign, err := os.ReadFile(filepath.Join(repo, config.DirName, ".gitignore"))
	if err != nil || !strings.Contains(string(ign), "\n*\n") {
		t.Errorf("la memoria activada sola no trae su .gitignore que la ignora entera (%v): %q", err, ign)
	}
}
