package repovivo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"musubi/internal/guiones"
)

// repoJuguete monta un repo git real y mínimo en un dir temporal: un commit inicial para
// que existan HEAD y .git/index. No toca ningún repo de verdad.
func repoJuguete(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := guiones.Herramienta(t, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v falló: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	git("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "a.txt")
	git("commit", "-qm", "inicial")
	return dir
}

func sha256Archivo(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leyendo %s: %v", ruta, err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// TestGitRunnerPasaStdin verifica que run pasa el stdin al proceso y captura el stdout: el
// sha de hash-object por --stdin debe coincidir con el del MISMO contenido escrito a archivo.
// Si el stdin no llegara, hash-object leería vacío y el sha no coincidiría.
func TestGitRunnerPasaStdin(t *testing.T) {
	dir := repoJuguete(t)
	g := nuevoGitRunner(dir)
	ctx := context.Background()
	contenido := []byte("contenido para el blob\n")

	shaStdin, err := g.run(ctx, contenido, "hash-object", "--stdin")
	if err != nil {
		t.Fatalf("hash-object --stdin: %v", err)
	}
	ruta := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(ruta, contenido, 0o644); err != nil {
		t.Fatal(err)
	}
	shaArchivo, err := g.run(ctx, nil, "hash-object", ruta)
	if err != nil {
		t.Fatalf("hash-object archivo: %v", err)
	}
	if strings.TrimSpace(shaStdin) == "" {
		t.Fatal("hash-object --stdin devolvió vacío: ¿el stdin no llegó?")
	}
	if strings.TrimSpace(shaStdin) != strings.TrimSpace(shaArchivo) {
		t.Errorf("el stdin no llegó intacto: stdin=%q archivo=%q", shaStdin, shaArchivo)
	}
}

// TestGitRunnerEnvApuntaIndiceTemporal verifica el invariante R2 vía el env: con
// GIT_INDEX_FILE apuntando a un índice temporal, un `add -A` captura el working tree SIN
// tocar el índice real del checkout. Prueba las dos caras: (a) el índice real no cambia y el
// archivo nuevo sigue pendiente ahí; (b) el índice temporal se crea de verdad (el env llegó).
func TestGitRunnerEnvApuntaIndiceTemporal(t *testing.T) {
	dir := repoJuguete(t)
	g := nuevoGitRunner(dir)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(dir, "nuevo.txt"), []byte("en vivo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idxReal := filepath.Join(dir, ".git", "index")
	antes := sha256Archivo(t, idxReal)

	idxTemp := filepath.Join(dir, ".git", "foto-test.index")
	gf := g.conEnv("GIT_INDEX_FILE=" + idxTemp)
	if _, err := gf.run(ctx, nil, "add", "-A"); err != nil {
		t.Fatalf("add -A sobre índice temporal: %v", err)
	}
	tree, err := gf.run(ctx, nil, "write-tree")
	if err != nil {
		t.Fatalf("write-tree: %v", err)
	}
	if strings.TrimSpace(tree) == "" {
		t.Error("write-tree no devolvió un árbol")
	}

	if despues := sha256Archivo(t, idxReal); antes != despues {
		t.Errorf("GIT_INDEX_FILE no se respetó: el índice real cambió (antes %s, después %s)", antes, despues)
	}
	if _, err := os.Stat(idxTemp); err != nil {
		t.Errorf("no se creó el índice temporal %s: ¿el env no llegó? %v", idxTemp, err)
	}
	st, err := g.run(ctx, nil, "status", "--porcelain")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(st, "nuevo.txt") {
		t.Errorf("el add -A tocó el índice real: status no muestra nuevo.txt como pendiente:\n%s", st)
	}
}

// TestGitRunnerDevuelveStdoutYErrorConStderr verifica que run devuelve el stdout en el
// camino feliz y, al fallar, un error que (a) nombra el comando, (b) envuelve el exit code y
// (c) adjunta el stderr de git. Para (c) sin depender del locale, el caso de error pasa una
// ruta inexistente con un marcador único: git la ecoa en su stderr y debe aparecer en el error.
func TestGitRunnerDevuelveStdoutYErrorConStderr(t *testing.T) {
	dir := repoJuguete(t)
	g := nuevoGitRunner(dir)
	ctx := context.Background()

	head, err := g.run(ctx, nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	if len(strings.TrimSpace(head)) < 40 {
		t.Errorf("rev-parse HEAD devolvió %q, esperaba un sha", head)
	}

	marcador := filepath.Join(dir, "archivo-inexistente-marcador-xyz")
	_, err = g.run(ctx, nil, "hash-object", marcador)
	if err == nil {
		t.Fatal("hash-object de un archivo inexistente debía fallar")
	}
	if !strings.Contains(err.Error(), "hash-object") {
		t.Errorf("el error no nombra el comando: %v", err)
	}
	if !strings.Contains(err.Error(), "marcador-xyz") {
		t.Errorf("el error no adjunta el stderr de git (no aparece la ruta que falló): %v", err)
	}
}

// TestRechazarFlags fija la guarda anti-flag: los valores de afuera (remoto, ref) que empiezan
// con '-' se rechazan; los normales y el vacío pasan. Es la defensa contra un remoto o ref que
// git leería como opción (p.ej. "--upload-pack=<cmd>").
func TestRechazarFlags(t *testing.T) {
	casos := []struct {
		nombre  string
		vals    []string
		rechaza bool
	}{
		{"remoto normal", []string{"origin"}, false},
		{"remoto forgejo", []string{"forgejo"}, false},
		{"ref del repo vivo", []string{"refs/vivo/nodo/slug"}, false},
		{"vacío pasa", []string{""}, false},
		{"flag corto", []string{"-x"}, true},
		{"flag largo malicioso", []string{"--upload-pack=touch /tmp/x"}, true},
		{"uno bueno y uno malo", []string{"origin", "--output=/etc/x"}, true},
	}
	for _, c := range casos {
		err := rechazarFlags(c.vals...)
		if (err != nil) != c.rechaza {
			t.Errorf("%s: rechazarFlags(%q) err=%v, esperaba rechazo=%v", c.nombre, c.vals, err, c.rechaza)
		}
	}
}

// TestEntornoNoInteractivo fija las defensas de NO interactividad (B4b) sobre un entorno base
// CONTROLADO —no el real del proceso, que suele traer estas variables inyectadas por el harness de
// desarrollo y volvería vacua la prueba (ya pasó: con el ambiente que las trae, hasta quitar el fix
// seguía verde)—. Verifica las tres caras: (a) un GIT_TERMINAL_PROMPT heredado se fuerza a 0; (b) si
// falta GIT_SSH_COMMAND, se agrega uno con BatchMode; (c) si el usuario ya puso el suyo, se respeta.
func TestEntornoNoInteractivo(t *testing.T) {
	// valor efectivo de una clave: el ÚLTIMO gana en exec, así que recorremos y nos quedamos con él.
	ultimo := func(env []string, clave string) string {
		pref := clave + "="
		v := ""
		for _, e := range env {
			if strings.HasPrefix(e, pref) {
				v = e
			}
		}
		return v
	}

	// (a) un GIT_TERMINAL_PROMPT=1 heredado debe quedar forzado a 0.
	env := entornoNoInteractivo([]string{"GIT_TERMINAL_PROMPT=1", "PATH=/x"})
	if got := ultimo(env, "GIT_TERMINAL_PROMPT"); got != "GIT_TERMINAL_PROMPT=0" {
		t.Errorf("(a) no forzó GIT_TERMINAL_PROMPT a 0 sobre un valor heredado: efectivo = %q", got)
	}
	// (b) sin GIT_SSH_COMMAND en el base, se agrega uno con BatchMode.
	if got := ultimo(env, "GIT_SSH_COMMAND"); !strings.Contains(got, "BatchMode") {
		t.Errorf("(b) no agregó un GIT_SSH_COMMAND con BatchMode cuando faltaba: efectivo = %q", got)
	}
	// (c) con el GIT_SSH_COMMAND del usuario en el base, se respeta (no se pisa con el default).
	conSSH := entornoNoInteractivo([]string{"GIT_SSH_COMMAND=ssh -i /mi/clave"})
	if got := ultimo(conSSH, "GIT_SSH_COMMAND"); got != "GIT_SSH_COMMAND=ssh -i /mi/clave" {
		t.Errorf("(c) pisó el GIT_SSH_COMMAND del usuario: efectivo = %q", got)
	}
}

// TestEntornoDeterministaNeutralizaLiteralPathspecs fija N2 sobre un base CONTROLADO: un
// GIT_LITERAL_PATHSPECS=1 heredado debe quedar forzado a 0 (gana el último en exec), porque con la
// variable en 1 el pathspec mágico de la exclusión de secretos se vuelve texto literal y la foto se
// llevaría los secretos sin error. Base controlado y no el real del proceso: el entorno de desarrollo
// puede no traer la variable y la prueba mediría nada.
func TestEntornoDeterministaNeutralizaLiteralPathspecs(t *testing.T) {
	ultimo := func(env []string, clave string) string {
		pref := clave + "="
		v := ""
		for _, e := range env {
			if strings.HasPrefix(e, pref) {
				v = e
			}
		}
		return v
	}
	// (a) un GIT_LITERAL_PATHSPECS=1 heredado debe quedar forzado a 0.
	env := entornoDeterminista([]string{"GIT_LITERAL_PATHSPECS=1", "PATH=/x"})
	if got := ultimo(env, "GIT_LITERAL_PATHSPECS"); got != "GIT_LITERAL_PATHSPECS=0" {
		t.Errorf("(a) no forzó GIT_LITERAL_PATHSPECS a 0 sobre un valor heredado: efectivo = %q", got)
	}
	// (b) si no venía en el base, igual queda fijada en 0.
	env2 := entornoDeterminista([]string{"PATH=/x"})
	if got := ultimo(env2, "GIT_LITERAL_PATHSPECS"); got != "GIT_LITERAL_PATHSPECS=0" {
		t.Errorf("(b) no fijó GIT_LITERAL_PATHSPECS=0 cuando faltaba: efectivo = %q", got)
	}
	// (c) no muta el base recibido.
	base := []string{"PATH=/x"}
	_ = entornoDeterminista(base)
	if len(base) != 1 || base[0] != "PATH=/x" {
		t.Errorf("(c) entornoDeterminista mutó el base: %v", base)
	}
}

// TestGitRunnerWaitDelayNoSeCuelgaConNietoVivo mide B4a: cuando git lanza un nieto que sobrevive al
// SIGKILL del timeout reteniendo el pipe de stderr, run() DEBE volver igual (gracias a cmd.WaitDelay)
// en vez de colgarse hasta que el nieto muera por su cuenta. Se simula el nieto con un filtro clean
// que duerme 10 s: git queda bloqueado esperándolo, el timeout de 1 s mata a git, y el filtro —ya
// huérfano— sigue con el write-end del pipe abierto. Sin WaitDelay, cmd.Wait no vería EOF hasta los
// 10 s y el observador quedaría mudo todo ese rato. Unix-only: usa `sleep`.
func TestGitRunnerWaitDelayNoSeCuelgaConNietoVivo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa un filtro clean con `sleep` para simular el nieto que retiene el pipe")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := guiones.Herramienta(t, "git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	// Un filtro clean que duerme: git lo spawnea como nieto al stagear y le hereda el stderr (= el
	// pipe que run captura). Mientras duerme, el write-end de ese pipe sigue abierto.
	git("config", "filter.dormilon.clean", "sleep 10")
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("* filter=dormilon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hola\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := gitRunner{dir: dir, timeout: 1 * time.Second} // timeout corto: git muere al segundo
	hecho := make(chan struct{})
	go func() {
		_, _ = g.run(context.Background(), nil, "add", "-A")
		close(hecho)
	}()
	select {
	case <-hecho:
		// volvió: WaitDelay cerró el pipe pese al nieto dormido. OK.
	case <-time.After(5 * time.Second):
		t.Fatal("run() no volvió en 5 s: el nieto (filtro clean) retiene el pipe y sin cmd.WaitDelay cmd.Wait no vuelve")
	}
}
