package repovivo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// secretosPorDefecto son los patrones que la foto EXCLUYE siempre, aunque el repo los
// trackee y aunque NO estén en .gitignore (exclusión dura, R6). La lista del proyecto
// (repo_vivo.secretos_extra) se suma a ésta. Son nombres/patrones de archivo —no regex— que
// pathspecsDeSecreto convierte en pathspecs anclados a la raíz del repo y válidos en CUALQUIER nivel.
// Se mantiene conservadora: sólo material claramente sensible, para no borrar de la foto
// archivos legítimos (por eso no van, p.ej., los *.crt, que son certificados públicos).
var secretosPorDefecto = []string{
	"*.env", ".env", ".env.*",
	"*.pem", "*.key", "*.pfx", "*.p12",
	"*.keystore", "*.jks",
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
}

// mensajeFoto es el cuerpo del commit huérfano de cada foto. La fecha y la autoría van en los
// metadatos del commit (identidad fija de la foto, ver tomarFoto), no en el mensaje.
const mensajeFoto = "foto viva del working tree (zona en vivo)"

// errOperacionEnVuelo señala que la foto PAUSÓ porque había una operación git a medio camino
// (R7). El observador lo trata como "saltear este tick", no como falla: no es un error que
// amerite ruido, es el comportamiento correcto.
var errOperacionEnVuelo = errors.New("operación git en vuelo: foto pausada")

// fotografo agrupa lo estable de un checkout: su runner, el git-dir real (resuelto una vez) y
// la lista de secretos a excluir. Un fotografo es de un solo checkout; la identidad y el
// transporte (ref, remoto, lease) los agregan las piezas de más arriba.
type fotografo struct {
	git      gitRunner
	gitDir   string // absoluto, resuelto una vez al construir
	secretos []string
}

// nuevoFotografo resuelve el git-dir del checkout y arma el fotografo. Si dir no es un repo
// git, rev-parse falla y se devuelve el error: ésa es la detección de "hay repo" (no asume
// que exista un `.git` físico, así anda también en un worktree, donde `.git` es un archivo).
func nuevoFotografo(dir string, secretosExtra []string) (*fotografo, error) {
	g := nuevoGitRunner(dir)
	salida, err := g.run(context.Background(), nil, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, fmt.Errorf("%s no parece un repo git: %w", dir, err)
	}
	secretos := make([]string, 0, 2*(len(secretosPorDefecto)+len(secretosExtra)))
	for _, p := range secretosPorDefecto {
		secretos = append(secretos, pathspecsDeSecreto(p)...)
	}
	for _, p := range secretosExtra {
		secretos = append(secretos, pathspecsDeSecreto(p)...)
	}
	return &fotografo{
		git:      g,
		gitDir:   strings.TrimSpace(salida),
		secretos: secretos,
	}, nil
}

// pathspecsDeSecreto convierte un patrón de secreto en los pathspecs de git que lo hacen valer
// SIEMPRE desde la raíz del repo (no desde el cwd del fotógrafo). Sin esto, un patrón literal como
// "id_rsa", ".env" o ".env.*" sólo matchea en la raíz —los secretos anidados (.ssh/id_rsa,
// apps/web/.env.local) se colaban a la foto— y, si el fotógrafo corre desde un subdirectorio, el
// rm --cached ni siquiera alcanzaba la raíz. La magia ":(top,glob,icase)" ancla a la raíz, habilita
// "**/" (cualquier nivel) y vuelve el match insensible a mayúsculas (Windows). Devuelve DOS pathspecs
// por patrón —el ítem y, por si fuera un directorio, su contenido—. Un patrón con su propia magia
// (":(") se respeta tal cual; uno con ruta ("config/x") se ancla a la raíz; un nombre suelto matchea
// en CUALQUIER nivel.
func pathspecsDeSecreto(p string) []string {
	if strings.HasPrefix(p, ":(") {
		return []string{p} // ya trae su propia magia: se respeta tal cual
	}
	// El `\` es AMBIGUO: en Windows separa rutas (config\secrets.yml), pero en un patrón glob ESCAPA
	// el carácter siguiente (otro\[2\].txt = el literal "otro[2].txt"). No sabemos cuál quiso el usuario,
	// así que generamos DOS lecturas y sobre-excluimos, que es lo seguro: la lectura "ruta" con \ → /
	// —config\secrets.yml ⇒ config/secrets.yml—, y SÓLO si el patrón traía `\`, también la lectura con
	// el `\` intacto, para que el glob honre el escape y "otro\[2\].txt" no termine convertido en la
	// clase de caracteres "otro/[2/].txt", que ya no matchea el literal. Cada forma se normaliza (sin
	// "./" inicial, sin "/" final); los pathspecs duplicados exactos se descartan.
	formas := []string{strings.ReplaceAll(p, "\\", "/")}
	if strings.Contains(p, "\\") {
		formas = append(formas, p)
	}
	// icase para que valga en sistemas de archivos insensibles a mayúsculas (Windows): .ENV e ID_RSA
	// se excluyen igual que .env. Emitimos DOS pathspecs por forma: el ítem en sí y, por si fuera un
	// directorio, todo lo que cuelga de él. Un patrón que trae ruta ("config/x") se ancla a la raíz;
	// un nombre suelto ("id_rsa", "*.pem") matchea en CUALQUIER nivel del árbol.
	const magia = ":(top,glob,icase)"
	var out []string
	visto := map[string]bool{}
	agregar := func(s string) {
		if !visto[s] {
			visto[s] = true
			out = append(out, s)
		}
	}
	for _, q := range formas {
		q = strings.TrimPrefix(q, "./")
		q = strings.Trim(q, "/")
		if q == "" || q == "." {
			continue
		}
		if strings.Contains(q, "/") {
			agregar(magia + q)
			agregar(magia + q + "/**")
		} else {
			agregar(magia + "**/" + q)
			agregar(magia + "**/" + q + "/**")
		}
	}
	return out
}

// operacionGitEnVuelo decide si hay una operación git a medio camino —rebase, merge,
// cherry-pick, revert, bisect— o un lock del índice. En ese estado la foto PAUSA (R7): un
// `add -A` en pleno rebase capturaría un working tree a mitad de reescritura. Las rutas se
// arman sobre el git-dir real, así el chequeo vale también dentro de un worktree.
func operacionGitEnVuelo(gitDir string) bool {
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		if info, err := os.Stat(filepath.Join(gitDir, d)); err == nil && info.IsDir() {
			return true
		}
	}
	for _, f := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "index.lock"} {
		if _, err := os.Stat(filepath.Join(gitDir, f)); err == nil {
			return true
		}
	}
	return false
}

// sembrarDesdeIndiceReal copia el índice del checkout (src) al índice temporal de la foto (dst), para
// que la captura arranque viendo TODO lo que el usuario ya tiene trackeado y STAGED. Es lectura pura
// del índice del usuario: no lo modifica (R2). git escribe su índice con un rename atómico, así que
// leerlo entero da una versión consistente (la vieja o la nueva, nunca a medias). Si el índice real no
// existe (repo recién init, nada staged ni commiteado), no hay nada que sembrar y el índice temporal
// queda vacío, que es lo correcto: add -A capturará el working tree completo.
func sembrarDesdeIndiceReal(dst, src string) error {
	datos, err := os.ReadFile(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(dst, datos, 0o600)
}

// escribirTree captura TODO el working tree en un índice TEMPORAL y devuelve el sha del árbol,
// SIN crear ningún commit. Es la mitad barata de la foto (la que el observador corre cada tick
// para decidir si algo cambió). Invariantes del spike:
//   - R1 captura TODO el working tree (tracked modificado, staged, y sin trackear).
//   - R2 NO toca el índice ni el HEAD del usuario: usa un GIT_INDEX_FILE temporal.
//   - R4 índice temporal único por invocación: dos capturas concurrentes no se pisan.
//   - R6 excluye los secretos aunque el repo los trackee o no estén en .gitignore.
//   - R7 si hay una operación git en vuelo, devuelve errOperacionEnVuelo sin tocar nada.
//
// El sha del árbol es función SÓLO del contenido: dos capturas de un working tree idéntico dan
// el mismo sha, y eso es lo que deja al observador saltear el commit+push cuando nada cambió.
func (f *fotografo) escribirTree(ctx context.Context) (string, error) {
	if operacionGitEnVuelo(f.gitDir) {
		return "", errOperacionEnVuelo
	}

	// Índice temporal ÚNICO por invocación, dentro del git-dir (R4). Descartable: lo borra
	// el defer pase lo que pase, así no quedan índices colgados ni en el camino de error.
	tmp, err := os.MkdirTemp(f.gitDir, "foto-viva.")
	if err != nil {
		return "", fmt.Errorf("creando índice temporal: %w", err)
	}
	defer os.RemoveAll(tmp)
	gf := f.git.conEnv("GIT_INDEX_FILE=" + filepath.Join(tmp, "index"))

	// (R1) Sembramos el índice temporal desde el ÍNDICE REAL del usuario (no desde HEAD) ANTES del
	// add -A. El índice real trae TODO lo que el usuario ya conoce —lo de HEAD y, sobre todo, lo
	// STAGED que todavía no commiteó—; sembrar sólo desde HEAD perdía en silencio un archivo recién
	// `git add`-eado que encima matchea .gitignore: ausente del índice sembrado, el add -A lo trata
	// como ignorado-sin-trackear y lo DROPEA (y el dedupe por árbol hace que ningún tick posterior lo
	// recupere). Con el índice real sembrado, add -A ve tracked lo que el usuario stageó y lo captura
	// con su contenido sin commitear. Las reglas de ignore no aplican a lo ya trackeado en el índice.
	if err := sembrarDesdeIndiceReal(filepath.Join(tmp, "index"), filepath.Join(f.gitDir, "index")); err != nil {
		return "", fmt.Errorf("sembrando el índice temporal: %w", err)
	}

	// (R1) captura TODO el working tree en el índice temporal (sobre lo sembrado desde el índice real).
	if _, err := gf.run(ctx, nil, "add", "-A"); err != nil {
		return "", fmt.Errorf("add -A: %w", err)
	}
	// (R6) exclusión DURA: saca los secretos del índice temporal. --cached no toca el working
	// tree (y el índice es descartable igual); --sparse para alcanzar también las entradas marcadas
	// skip-worktree o fuera del cono de sparse-checkout —sin él `git rm --cached` las SALTEA y un
	// secreto trackeado así SOBREVIVE y entra a la foto, agravado porque sembrarDesdeIndiceReal copia
	// las banderas skip-worktree del índice real; requiere git ≥ 2.34, y en git más viejo el flag es
	// desconocido y el rm falla, lo que PAUSA la foto (fail-closed: nunca una foto con secretos)—;
	// --ignore-unmatch para no fallar si un patrón no matchea nada; el `--` sella los patrones como
	// pathspecs, nunca como flags.
	rmArgs := append([]string{"rm", "-q", "--cached", "--sparse", "--ignore-unmatch", "-r", "--"}, f.secretos...)
	if _, err := gf.run(ctx, nil, rmArgs...); err != nil {
		// Un error acá (fuera del "no matcheó nada", que --ignore-unmatch ya silencia) dejaría
		// secretos dentro de la foto: preferimos NO tomarla a tomarla con secretos.
		return "", fmt.Errorf("excluyendo secretos: %w", err)
	}
	tree, err := gf.run(ctx, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write-tree: %w", err)
	}
	return strings.TrimSpace(tree), nil
}

// commitDeTree envuelve un árbol ya escrito en un commit HUÉRFANO (sin padre) y devuelve su sha.
// Es la mitad cara-en-red de la foto, separada a propósito: sólo se corre cuando el árbol cambió.
//   - R3 NO dispara hooks: commit-tree no los corre (a diferencia de `git commit`).
//
// Huérfano (sin -p): la foto se PISA, no acumula historia ni se ata al HEAD del usuario. Identidad
// fija por env: no depende de que el usuario tenga user.name/email configurados, y deja clara la
// autoría. La fecha queda real (el dedupe por árbol de más arriba ya evita el commit espurio).
func (f *fotografo) commitDeTree(ctx context.Context, tree string) (string, error) {
	if err := rechazarFlags(tree); err != nil {
		return "", err
	}
	commit, err := f.git.conEnv(
		"GIT_AUTHOR_NAME=Musubi repo-vivo",
		"GIT_AUTHOR_EMAIL=repo-vivo@musubi.local",
		"GIT_COMMITTER_NAME=Musubi repo-vivo",
		"GIT_COMMITTER_EMAIL=repo-vivo@musubi.local",
	).run(ctx, []byte(mensajeFoto), "commit-tree", tree)
	if err != nil {
		return "", fmt.Errorf("commit-tree: %w", err)
	}
	return strings.TrimSpace(commit), nil
}

// tomarFoto es la foto completa: escribe el árbol del working tree y lo envuelve en un commit
// huérfano, devolviendo su sha. No crea ni mueve ninguna ref: el commit queda suelto y lo publica
// el transporte. Es la composición de escribirTree + commitDeTree (ver sus invariantes).
func (f *fotografo) tomarFoto(ctx context.Context) (string, error) {
	tree, err := f.escribirTree(ctx)
	if err != nil {
		return "", err
	}
	return f.commitDeTree(ctx, tree)
}
