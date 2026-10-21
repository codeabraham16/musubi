package repovivo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// gitRunner ejecuta git en un directorio de trabajo, de forma model-free y determinista,
// con un entorno EXTRA configurable. A diferencia del GitRunner de internal/codeintel —que
// sólo hace `diff` y es de solo lectura— la zona en vivo necesita escribir (add -A /
// write-tree / commit-tree / push) y, sobre todo, poder apuntar GIT_INDEX_FILE a un índice
// temporal: así la foto viva nunca toca el índice ni el HEAD del checkout compartido (el
// invariante R2 del spike). Es un value type: conEnv devuelve una copia, nunca muta el
// original, para que dos fotos concurrentes no se pisen el entorno.
type gitRunner struct {
	dir     string   // directorio del checkout (se pasa como cmd.Dir, no como -C)
	env     []string // entradas "CLAVE=valor" que se SUMAN al entorno del proceso
	timeout time.Duration
}

// nuevoGitRunner crea un runner sobre el checkout dado con un timeout por defecto. El
// timeout acota cada invocación: un git colgado (lock remoto, red muerta) no debe trabar
// el observador, que corre en su propia goroutine atada al ctx del daemon.
func nuevoGitRunner(dir string) gitRunner {
	return gitRunner{dir: dir, timeout: 15 * time.Second}
}

// conEnv devuelve una COPIA del runner con las entradas de entorno dadas añadidas (p.ej.
// "GIT_INDEX_FILE=/ruta/al/índice-temporal"). Copia el slice en vez de hacer append sobre
// el del receptor: con un value receiver, un append podría compartir el array subyacente y
// una foto concurrente vería el GIT_INDEX_FILE de la otra. El original queda intacto.
func (g gitRunner) conEnv(kv ...string) gitRunner {
	nuevo := make([]string, 0, len(g.env)+len(kv))
	nuevo = append(nuevo, g.env...)
	nuevo = append(nuevo, kv...)
	g.env = nuevo
	return g
}

// entorno arma el entorno del proceso git: el del propio proceso (os.Environ) + el extra de g.env,
// con las defensas de NO interactividad aplicadas (ver entornoNoInteractivo). El observador corre en
// el daemon, sin un humano delante, así que git NUNCA debe abrir un prompt.
func (g gitRunner) entorno() []string {
	return entornoDeterminista(entornoNoInteractivo(append(os.Environ(), g.env...)))
}

// entornoNoInteractivo toma un entorno base y devuelve uno NUEVO con las defensas que impiden que git
// se cuelgue pidiendo credenciales (no muta el base). GIT_TERMINAL_PROMPT=0 hace fallar rápido el
// prompt de credencial de git-remote-http —en vez de colgar la invocación hasta el timeout y, peor,
// escribir en la terminal del usuario— y va al FINAL para ganarle a un valor heredado. Para el
// transporte por ssh, un GIT_SSH_COMMAND con BatchMode apaga los prompts de passphrase y de host-key,
// pero sólo se agrega si el base no trae ya el del usuario (que puede llevar su propio -i/-o). Está
// separada de entorno() a propósito: la prueba la ejerce con un base CONTROLADO, porque el entorno
// real del proceso suele traer estas variables (el harness de desarrollo las inyecta) y mediría nada.
func entornoNoInteractivo(base []string) []string {
	env := append([]string{}, base...)
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	if !tieneClaveEnv(env, "GIT_SSH_COMMAND") {
		env = append(env, "GIT_SSH_COMMAND=ssh -oBatchMode=yes")
	}
	return env
}

// entornoDeterminista neutraliza las variables de git que harían interpretar un pathspec según el
// entorno heredado. GIT_LITERAL_PATHSPECS=1 (que el daemon puede heredar del proceso) apaga TODA la
// magia de pathspec —incluido el ":(top,glob,icase)" con que la foto ancla la exclusión de secretos—,
// y entonces `git rm --cached --ignore-unmatch` no matchea NADA y CALLA: la foto se llevaría los
// secretos sin un solo error. Fijarla a 0 al FINAL le gana a un valor heredado. Va separada para que
// la prueba la ejerza con un base CONTROLADO (el entorno real del proceso puede no traer la variable).
func entornoDeterminista(base []string) []string {
	return append(append([]string{}, base...), "GIT_LITERAL_PATHSPECS=0")
}

// tieneClaveEnv informa si env ya trae una entrada para la variable `clave` (comparando contra la
// forma "CLAVE="). Sirve para no pisar un GIT_SSH_COMMAND que el usuario haya puesto a mano.
func tieneClaveEnv(env []string, clave string) bool {
	prefijo := clave + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefijo) {
			return true
		}
	}
	return false
}

// run ejecuta `git <args...>` en g.dir, con el entorno extra de g.env y el stdin dado (nil
// = sin stdin). Devuelve el stdout TAL CUAL (el llamador recorta si quiere) y, si git salió
// con error, un error que incluye el stderr para diagnóstico —el transporte lo inspecciona
// para distinguir el rechazo de --force-with-lease de una falla real—. El timeout de g corta
// la invocación. NUNCA se usa porcelain (add/commit del usuario): sólo plumbing.
func (g gitRunner) run(ctx context.Context, stdin []byte, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := g.timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.dir
	cmd.Env = g.entorno()
	// WaitDelay acota cuánto espera cmd.Wait TRAS matarse git (por el timeout de arriba o el ctx
	// cancelado): si git lanzó un nieto —git-remote-http al pushear, o un filtro clean/smudge— que
	// sobrevive al SIGKILL reteniendo el write-end del pipe de stderr, SIN esto cmd.Run() no vuelve
	// nunca y el observador queda mudo, sin error ni tick siguiente. Con él, Wait fuerza el cierre del
	// pipe y vuelve, así el timeout vuelve a acotar DE VERDAD la invocación (el nieto queda huérfano,
	// pero eso es cosa del SO, no nuestra: lo que importa es no trabar la goroutine del observador).
	cmd.WaitDelay = 2 * time.Second
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s falló: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// rechazarFlags protege los pocos valores de la zona en vivo que vienen DE AFUERA —el
// nombre del remoto (de la config del usuario) y el de la ref (derivado del host y del
// checkout)— de que git los lea como OPCIONES. Sin esto, un remoto como "--upload-pack=<cmd>"
// o un ref "--output=<ruta>" se interpretan como flags y ejecutan/escriben lo que no deben.
// Es la misma defensa que internal/codeintel/git.go aplica al ref de diff (auditoría #7).
// Un valor vacío no empieza con '-', así que no lo rechaza acá: lo valida quien lo exige.
func rechazarFlags(vals ...string) error {
	for _, v := range vals {
		if strings.HasPrefix(v, "-") {
			return fmt.Errorf("valor inválido %q: no puede empezar con '-'", v)
		}
	}
	return nil
}
