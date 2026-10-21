package repovivo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// repoEnVivo monta un repo con un working tree "en vivo": las tres clases de cambio a la vez,
// un hook post-commit que deja rastro si corre, un secreto YA trackeado (clave.pem) y un
// secreto nuevo sin trackear (secreto.env). Devuelve el dir del checkout.
func repoEnVivo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v falló: %v\n%s", args, err, out)
		}
	}
	escribir := func(nombre, contenido string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, nombre), []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	git("config", "commit.gpgsign", "false")

	hook := filepath.Join(dir, ".git", "hooks", "post-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho corrio >> \""+dir+"/HOOK-RASTRO.txt\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	escribir("a.txt", "v1\n")
	escribir("b.txt", "comun\n")
	escribir("clave.pem", "CLAVE PRIVADA TRACKEADA\n") // secreto ya trackeado
	git("add", "a.txt", "b.txt", "clave.pem")
	git("commit", "-qm", "inicial")                      // este commit SÍ dispara el hook...
	_ = os.Remove(filepath.Join(dir, "HOOK-RASTRO.txt")) // ...borramos su rastro: cuenta la FOTO

	// working tree en vivo: las tres clases de cambio
	escribir("a.txt", "v2-SIN-COMMIT\n") // (1) tracked modificado, no staged
	escribir("c.txt", "nuevo-staged\n")  // (2) nuevo y staged
	git("add", "c.txt")
	escribir("d.txt", "nuevo-suelto\n")             // (3) nuevo y sin trackear
	escribir("secreto.env", "TOKEN=xoxb-secreto\n") // secreto nuevo, NO en .gitignore
	return dir
}

func snapshotUsuario(t *testing.T, dir string) string {
	t.Helper()
	head := gitSalida(t, dir, "rev-parse", "HEAD")
	st := gitSalida(t, dir, "status", "--porcelain=v2")
	idx := sha256Archivo(t, filepath.Join(dir, ".git", "index"))
	h := sha256.Sum256([]byte(head + "|" + st))
	return hex.EncodeToString(h[:]) + "|" + idx
}

func gitSalida(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

func contenidoDeCommit(t *testing.T, dir, sha string) string {
	t.Helper()
	nombres := strings.Fields(gitSalida(t, dir, "ls-tree", "-r", "--name-only", sha))
	// Fields parte por espacios; los nombres del juguete no llevan espacios. Ordenamos para
	// comparar estable.
	return strings.Join(nombres, ",")
}

// TestFotoCapturaWorkingTreeSinSecretos cubre R1 y R6: la foto tiene las tres clases de cambio
// con la versión sin commitear, y NO tiene ni el secreto trackeado (clave.pem) ni el nuevo
// (secreto.env).
func TestFotoCapturaWorkingTreeSinSecretos(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	if got := contenidoDeCommit(t, dir, sha); got != "a.txt,b.txt,c.txt,d.txt" {
		t.Errorf("contenido de la foto = %q; esperaba a.txt,b.txt,c.txt,d.txt (sin secretos)", got)
	}
	if av := gitSalida(t, dir, "show", sha+":a.txt"); av != "v2-SIN-COMMIT" {
		t.Errorf("a.txt en la foto = %q; esperaba la versión sin commitear 'v2-SIN-COMMIT'", av)
	}
}

// TestFotoNoTocaEstadoDelUsuario cubre R2: HEAD, índice y status quedan idénticos.
func TestFotoNoTocaEstadoDelUsuario(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	antes := snapshotUsuario(t, dir)
	if _, err := f.tomarFoto(context.Background()); err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	if despues := snapshotUsuario(t, dir); antes != despues {
		t.Errorf("la foto tocó el estado del usuario:\n antes=%s\n desp.=%s", antes, despues)
	}
}

// TestFotoNoDisparaHooks cubre R3: commit-tree no corre el post-commit.
func TestFotoNoDisparaHooks(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	if _, err := f.tomarFoto(context.Background()); err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "HOOK-RASTRO.txt")); err == nil {
		t.Error("el post-commit corrió: la foto NO debe disparar hooks (R3)")
	}
}

// TestFotoEsHuerfana: el commit no tiene padre. Con -p HEAD la foto acumularía historia y se
// ataría al HEAD del usuario; huérfana, se pisa limpio en cada tick.
func TestFotoEsHuerfana(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	cuerpo := gitSalida(t, dir, "cat-file", "-p", sha)
	if strings.Contains(cuerpo, "\nparent ") {
		t.Errorf("la foto tiene padre, debía ser huérfana:\n%s", cuerpo)
	}
}

// TestFotoConcurrenteNoSeCorrompe cubre R4: dos fotos a la vez sobre el mismo checkout dan dos
// árboles completos y no dejan índices temporales colgados. Correr con -race lo refuerza.
func TestFotoConcurrenteNoSeCorrompe(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	var wg sync.WaitGroup
	shas := make([]string, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			shas[i], errs[i] = f.tomarFoto(context.Background())
		}(i)
	}
	wg.Wait()
	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Fatalf("foto concurrente %d falló: %v", i, errs[i])
		}
		if got := contenidoDeCommit(t, dir, shas[i]); got != "a.txt,b.txt,c.txt,d.txt" {
			t.Errorf("foto concurrente %d: árbol = %q, incompleto/corrupto", i, got)
		}
	}
	colgados, _ := filepath.Glob(filepath.Join(f.gitDir, "foto-viva.*"))
	if len(colgados) != 0 {
		t.Errorf("quedaron índices temporales colgados: %v", colgados)
	}
}

// TestFotoPausaConOperacionEnVuelo cubre R7: con un rebase simulado, tomarFoto pausa y no deja
// commit; retirado el rebase, vuelve a tomar foto.
func TestFotoPausaConOperacionEnVuelo(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	rebase := filepath.Join(f.gitDir, "rebase-merge")
	if err := os.Mkdir(rebase, 0o755); err != nil {
		t.Fatal(err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != errOperacionEnVuelo {
		t.Errorf("con rebase en curso esperaba errOperacionEnVuelo, obtuve sha=%q err=%v", sha, err)
	}
	if sha != "" {
		t.Errorf("pausada, no debía devolver sha, obtuve %q", sha)
	}
	if err := os.RemoveAll(rebase); err != nil {
		t.Fatal(err)
	}
	if _, err := f.tomarFoto(context.Background()); err != nil {
		t.Errorf("sin rebase, tomarFoto debía andar: %v", err)
	}
}

// repoConArbol monta un repo git vacío (sin "working tree en vivo" prearmado) y devuelve el dir
// y dos ayudantes: git(args…) corre git en el repo y escribir(ruta, contenido) escribe un archivo
// creando los directorios intermedios —para plantar secretos y cambios en SUBDIRECTORIOS, que es
// lo que repoEnVivo no permite—.
func repoConArbol(t *testing.T) (string, func(...string), func(string, string)) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v falló: %v\n%s", args, err, out)
		}
	}
	escribir := func(nombre, contenido string) {
		t.Helper()
		ruta := filepath.Join(dir, nombre)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	git("config", "commit.gpgsign", "false")
	return dir, git, escribir
}

// TestFotoCapturaTrackedAunqueGitignorado cubre R1 en su borde más filoso: un archivo TRACKEADO
// que además matchea .gitignore y tiene cambios sin commitear DEBE entrar a la foto con su versión
// sin commitear. Sin sembrar el índice temporal desde HEAD, git lo trata como ignorado-sin-trackear
// y lo DROPEA, perdiéndolo en silencio (el dedupe por árbol impide que un tick posterior lo recupere).
func TestFotoCapturaTrackedAunqueGitignorado(t *testing.T) {
	dir, git, escribir := repoConArbol(t)
	escribir("app.txt", "app\n")
	escribir("config.local", "v1\n")
	git("add", "app.txt", "config.local")
	git("commit", "-qm", "inicial")
	// config.local pasa a estar ignorado DESPUÉS de trackearse, y se modifica sin commitear.
	escribir(".gitignore", "config.local\n")
	escribir("config.local", "v2-SIN-COMMIT\n")

	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	if !strings.Contains(arbol, ",config.local,") {
		t.Errorf("config.local (tracked + gitignored + modificado) falta en la foto; árbol = %s", arbol)
	}
	if v := gitSalida(t, dir, "show", sha+":config.local"); v != "v2-SIN-COMMIT" {
		t.Errorf("config.local en la foto = %q; esperaba la versión sin commitear 'v2-SIN-COMMIT'", v)
	}
}

// TestFotoExcluyeSecretosEnSubdirectorios cubre R6 en profundidad: los secretos en CUALQUIER
// subdirectorio quedan fuera de la foto, aunque no estén en .gitignore, mientras que los archivos
// legítimos anidados SÍ entran. Los pathspecs literales anclados a la raíz (id_rsa, .env.*) dejaban
// pasar los anidados. Este repo no tiene commits: ejerce además el camino de HEAD sin nacer.
func TestFotoExcluyeSecretosEnSubdirectorios(t *testing.T) {
	dir, _, escribir := repoConArbol(t)
	// legítimos (uno anidado): deben ENTRAR
	escribir("app.txt", "app\n")
	escribir("deploy/README.md", "doc\n")
	// secretos anidados, NINGUNO en .gitignore: deben QUEDAR FUERA
	escribir(".ssh/id_rsa", "PRIV\n")
	escribir("deploy/keys/id_ed25519", "PRIV\n")
	escribir("apps/web/.env.local", "TOKEN=secreto\n")
	escribir("infra/prod.pem", "CERT\n")

	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	for _, secreto := range []string{".ssh/id_rsa", "deploy/keys/id_ed25519", "apps/web/.env.local", "infra/prod.pem"} {
		if strings.Contains(arbol, ","+secreto+",") {
			t.Errorf("secreto anidado %q entró a la foto; árbol = %s", secreto, arbol)
		}
	}
	for _, legit := range []string{"app.txt", "deploy/README.md"} {
		if !strings.Contains(arbol, ","+legit+",") {
			t.Errorf("archivo legítimo %q falta en la foto; árbol = %s", legit, arbol)
		}
	}
}

// TestFotoCapturaStagedAunGitignoradoYSinHEAD cubre R1 en el borde que el sembrado desde HEAD perdía:
// un archivo recién `git add`-eado (STAGED, en el índice real, pero AÚN NO en HEAD) que además matchea
// .gitignore. Sembrando el índice temporal desde HEAD ese archivo no estaba, y el add -A lo trataba
// como ignorado-sin-trackear y lo DROPEABA en silencio. Sembrando desde el índice REAL del usuario,
// entra con su contenido sin commitear. Es alcanzable sin -f: basta `git add` ANTES de que exista la
// regla en .gitignore (exactamente lo que pasa al agregar una regla a posteriori).
func TestFotoCapturaStagedAunGitignoradoYSinHEAD(t *testing.T) {
	dir, git, escribir := repoConArbol(t)
	escribir("app.txt", "app\n")
	git("add", "app.txt")
	git("commit", "-qm", "inicial") // HEAD = {app.txt}
	// data.bin: staged (en el índice real) pero NO commiteado -> no está en HEAD; y luego gitignored.
	escribir("data.bin", "contenido-staged\n")
	git("add", "data.bin")
	escribir(".gitignore", "data.bin\n") // ahora matchea .gitignore, pero YA está staged

	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	if !strings.Contains(arbol, ",data.bin,") {
		t.Errorf("data.bin (staged + gitignored + no en HEAD) se perdió de la foto; árbol = %s", arbol)
	}
	if v := gitSalida(t, dir, "show", sha+":data.bin"); v != "contenido-staged" {
		t.Errorf("data.bin en la foto = %q; esperaba 'contenido-staged'", v)
	}
}

// TestFotoExcluyeSecretosAunConLiteralPathspecsHeredado cubre N2: si el daemon hereda
// GIT_LITERAL_PATHSPECS=1, el ":(top,glob,icase)" con que la foto ancla la exclusión se trataría como
// texto literal y `git rm --cached --ignore-unmatch` no matchearía NADA, llevándose los secretos sin un
// error. entornoDeterminista fija la variable a 0, así que la exclusión vale igual. Sin t.Parallel:
// t.Setenv lo prohíbe, y el env es estado global del proceso.
func TestFotoExcluyeSecretosAunConLiteralPathspecsHeredado(t *testing.T) {
	t.Setenv("GIT_LITERAL_PATHSPECS", "1")
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	for _, secreto := range []string{"clave.pem", "secreto.env"} {
		if strings.Contains(arbol, ","+secreto+",") {
			t.Errorf("con GIT_LITERAL_PATHSPECS=1 heredado, el secreto %q entró a la foto; árbol = %s", secreto, arbol)
		}
	}
}

// TestFotoExcluyeSecretosEnMayusculas cubre B1-case: con :(icase) en el pathspec, un secreto escrito en
// mayúsculas (PROD.PEM, .ENV, ID_RSA) queda fuera igual que su forma en minúscula. En davantis-1
// (Windows, filesystem insensible) esto es lo que evita que .ENV se publique; se mide también en Linux,
// donde el filesystem es sensible pero el pathspec :(icase) matchea igual.
func TestFotoExcluyeSecretosEnMayusculas(t *testing.T) {
	dir, _, escribir := repoConArbol(t)
	escribir("app.txt", "app\n") // legítimo
	escribir("PROD.PEM", "CERT\n")
	escribir(".ENV", "TOKEN=x\n")
	escribir("sub/ID_RSA", "PRIV\n")

	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	for _, secreto := range []string{"PROD.PEM", ".ENV", "sub/ID_RSA"} {
		if strings.Contains(arbol, ","+secreto+",") {
			t.Errorf("secreto en mayúsculas %q entró a la foto; árbol = %s", secreto, arbol)
		}
	}
	if !strings.Contains(arbol, ",app.txt,") {
		t.Errorf("app.txt legítimo falta; árbol = %s", arbol)
	}
}

// TestFotoExcluyeSecretosExtraEnSubdirectorios cubre N3: un patrón de secretos_extra escrito como
// directorio ("secretos/") excluye su contenido en CUALQUIER nivel, no sólo en la raíz —que es lo que
// promete config.example.yaml—. Antes, "secretos/" se anclaba a la raíz y dejaba pasar sub/secretos/*.
func TestFotoExcluyeSecretosExtraEnSubdirectorios(t *testing.T) {
	dir, _, escribir := repoConArbol(t)
	escribir("app.txt", "app\n")
	escribir("secretos/raiz.txt", "x\n")
	escribir("sub/secretos/anidado.txt", "y\n")

	f, err := nuevoFotografo(dir, []string{"secretos/"})
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	for _, secreto := range []string{"secretos/raiz.txt", "sub/secretos/anidado.txt"} {
		if strings.Contains(arbol, ","+secreto+",") {
			t.Errorf("el dir de secretos_extra no se excluyó en todos los niveles: %q entró; árbol = %s", secreto, arbol)
		}
	}
	if !strings.Contains(arbol, ",app.txt,") {
		t.Errorf("app.txt legítimo falta; árbol = %s", arbol)
	}
}

// TestPathspecsDeSecretoNormaliza fija N3/B1 a nivel de unidad: la forma de los pathspecs que arma
// pathspecsDeSecreto. Cada patrón lleva magia :(top,glob,icase) y emite DOS pathspecs (el ítem y su
// contenido si fuera dir); un nombre suelto va a CUALQUIER nivel (**/) y uno con ruta se ancla a la
// raíz; se normalizan \, ./ inicial y / final; la magia propia :( se respeta; lo vacío no produce nada.
func TestPathspecsDeSecretoNormaliza(t *testing.T) {
	casos := []struct {
		nombre   string
		in       string
		esperado []string
	}{
		{"nombre suelto", "id_rsa", []string{":(top,glob,icase)**/id_rsa", ":(top,glob,icase)**/id_rsa/**"}},
		{"glob suelto", "*.pem", []string{":(top,glob,icase)**/*.pem", ":(top,glob,icase)**/*.pem/**"}},
		{"dir con barra final va a cualquier nivel", "secretos/", []string{":(top,glob,icase)**/secretos", ":(top,glob,icase)**/secretos/**"}},
		{"con ./ inicial", "./secreto.json", []string{":(top,glob,icase)**/secreto.json", ":(top,glob,icase)**/secreto.json/**"}},
		{"backslash: ruta normalizada Y escape de glob", "config\\secrets.yml", []string{":(top,glob,icase)config/secrets.yml", ":(top,glob,icase)config/secrets.yml/**", ":(top,glob,icase)**/config\\secrets.yml", ":(top,glob,icase)**/config\\secrets.yml/**"}},
		{"corchetes escapados: emite el escape de glob intacto (JS2)", "otro\\[2\\].txt", []string{":(top,glob,icase)otro/[2/].txt", ":(top,glob,icase)otro/[2/].txt/**", ":(top,glob,icase)**/otro\\[2\\].txt", ":(top,glob,icase)**/otro\\[2\\].txt/**"}},
		{"ruta se ancla a la raíz", "config/x.yml", []string{":(top,glob,icase)config/x.yml", ":(top,glob,icase)config/x.yml/**"}},
		{"magia propia se respeta", ":(exclude)foo", []string{":(exclude)foo"}},
		{"vacío no produce nada", "", nil},
		{"solo barras no produce nada", "/", nil},
		{"punto solo no produce nada", ".", nil},
	}
	for _, c := range casos {
		if got := pathspecsDeSecreto(c.in); !reflect.DeepEqual(got, c.esperado) {
			t.Errorf("%s: pathspecsDeSecreto(%q) = %v; esperaba %v", c.nombre, c.in, got, c.esperado)
		}
	}
}

// TestFotoExcluyeSecretoTrackeadoConSkipWorktree cubre JS1: un secreto YA trackeado al que el usuario le
// puso la bandera skip-worktree sobrevivía a `git rm --cached` —git saltea esas entradas SIN --sparse— y
// entraba a la foto, agravado porque sembrarDesdeIndiceReal copia la bandera del índice real. Con --sparse
// en el rm el secreto queda fuera. Es alcanzable: skip-worktree es de uso común para no tocar un archivo
// trackeado localmente (un .env de ejemplo que se personaliza, un binario grande).
func TestFotoExcluyeSecretoTrackeadoConSkipWorktree(t *testing.T) {
	dir, git, escribir := repoConArbol(t)
	escribir("app.txt", "app\n")
	escribir("deploy.pem", "SECRETO-SKIPWORKTREE\n") // secreto trackeado
	git("add", "app.txt", "deploy.pem")
	git("commit", "-qm", "inicial")
	git("update-index", "--skip-worktree", "deploy.pem") // el usuario lo marca skip-worktree

	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	if strings.Contains(arbol, ",deploy.pem,") {
		if v := gitSalida(t, dir, "show", sha+":deploy.pem"); v == "SECRETO-SKIPWORKTREE" {
			t.Errorf("el secreto trackeado con skip-worktree entró a la foto CON su contenido; árbol = %s", arbol)
		} else {
			t.Errorf("deploy.pem (skip-worktree) entró a la foto; árbol = %s", arbol)
		}
	}
	if !strings.Contains(arbol, ",app.txt,") {
		t.Errorf("app.txt legítimo falta; árbol = %s", arbol)
	}
}

// TestFotoExcluyeSecretoExtraConCorchetesEscapados cubre JS2: un patrón de secretos_extra que usa `\` para
// ESCAPAR corchetes de glob (otro\[2\].txt, para el literal otro[2].txt) debía excluir ese archivo. La
// normalización \ → / sola lo rompía (otro/[2/].txt, una clase de glob que no matchea el literal);
// emitiendo TAMBIÉN la lectura con `\` intacto, el glob honra el escape y el secreto queda afuera.
func TestFotoExcluyeSecretoExtraConCorchetesEscapados(t *testing.T) {
	dir, _, escribir := repoConArbol(t)
	escribir("app.txt", "app\n")
	escribir("otro[2].txt", "SECRETO\n") // nombre con corchetes LITERALES

	f, err := nuevoFotografo(dir, []string{`otro\[2\].txt`})
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	arbol := "," + contenidoDeCommit(t, dir, sha) + ","
	if strings.Contains(arbol, ",otro[2].txt,") {
		t.Errorf("el secreto con corchetes escapados entró a la foto; árbol = %s", arbol)
	}
	if !strings.Contains(arbol, ",app.txt,") {
		t.Errorf("app.txt legítimo falta; árbol = %s", arbol)
	}
}
