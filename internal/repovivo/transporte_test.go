package repovivo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// repoConRemoto arma un repo "en vivo" (repoEnVivo) con un remoto bare local llamado "central".
func repoConRemoto(t *testing.T) (f *fotografo, dir, remoto, bare string) {
	t.Helper()
	dir = repoEnVivo(t)
	bare = t.TempDir()
	runGit(t, "", "init", "--bare", "-q", bare)
	runGit(t, dir, "remote", "add", "central", bare)
	var err error
	f, err = nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatalf("nuevoFotografo: %v", err)
	}
	return f, dir, "central", bare
}

// shaEnBare lee el sha de una ref en el repo bare; "" si no existe.
func shaEnBare(t *testing.T, bare, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "--git-dir="+bare, "rev-parse", "--verify", "--quiet", ref)
	out, _ := cmd.Output() // exit 1 + salida vacía si la ref no existe
	return strings.TrimSpace(string(out))
}

// fotoTras cambia un archivo del working tree y devuelve el sha de la foto resultante.
func fotoTras(t *testing.T, f *fotografo, dir, archivo, contenido string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, archivo), []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatalf("tomarFoto: %v", err)
	}
	return sha
}

const refTest = "refs/vivo/test/slug"

func TestTransportePrimerPushCreaLaRef(t *testing.T) {
	f, _, remoto, bare := repoConRemoto(t)
	tr, err := nuevoTransporte(f.git, remoto, refTest)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.publicar(context.Background(), sha); err != nil {
		t.Fatalf("publicar: %v", err)
	}
	if got := shaEnBare(t, bare, refTest); got != sha {
		t.Errorf("ref remota = %q, esperaba la foto %q", got, sha)
	}
	if tr.ultimoSha != sha {
		t.Errorf("ultimoSha = %q, esperaba %q", tr.ultimoSha, sha)
	}
}

func TestTransportePisaLaFotoAnterior(t *testing.T) {
	f, dir, remoto, bare := repoConRemoto(t)
	tr, _ := nuevoTransporte(f.git, remoto, refTest)
	sha1 := fotoTras(t, f, dir, "a.txt", "uno\n")
	if err := tr.publicar(context.Background(), sha1); err != nil {
		t.Fatal(err)
	}
	sha2 := fotoTras(t, f, dir, "a.txt", "dos\n")
	if sha1 == sha2 {
		t.Fatal("las dos fotos salieron iguales; el test necesita shas distintos")
	}
	if err := tr.publicar(context.Background(), sha2); err != nil {
		t.Fatalf("segundo push: %v", err)
	}
	if got := shaEnBare(t, bare, refTest); got != sha2 {
		t.Errorf("ref remota = %q, esperaba la foto nueva %q (no pisó la anterior)", got, sha2)
	}
}

// TestTransporteLeaseViejoNoPisa cubre R5a a nivel de la pieza: con un lease que ya no refleja
// el remoto (porque otro lo avanzó), pushConLease es rechazado y el remoto NO se mueve.
func TestTransporteLeaseViejoNoPisa(t *testing.T) {
	f, dir, remoto, bare := repoConRemoto(t)
	tr, _ := nuevoTransporte(f.git, remoto, refTest)
	sha1 := fotoTras(t, f, dir, "a.txt", "uno\n")
	if err := tr.publicar(context.Background(), sha1); err != nil {
		t.Fatal(err)
	}
	// avance AJENO: otro empuja sha2 a la ref, por fuera del transporte
	sha2 := fotoTras(t, f, dir, "a.txt", "dos\n")
	runGit(t, dir, "push", "--force", remoto, sha2+":"+refTest)

	sha3 := fotoTras(t, f, dir, "a.txt", "tres\n")
	if err := tr.pushConLease(context.Background(), sha3+":"+refTest, sha1); err == nil {
		t.Error("el push con lease viejo (sha1) debía ser rechazado")
	}
	if got := shaEnBare(t, bare, refTest); got != sha2 {
		t.Errorf("el remoto se movió con un lease viejo: = %q, esperaba %q", got, sha2)
	}
}

// TestTransportePublicarSeEnteraYReintenta cubre R5b: tras el rechazo por lease viejo, publicar
// lee el sha real (ls-remote) y reintenta con él, publicando la foto nueva.
func TestTransportePublicarSeEnteraYReintenta(t *testing.T) {
	f, dir, remoto, bare := repoConRemoto(t)
	tr, _ := nuevoTransporte(f.git, remoto, refTest)
	sha1 := fotoTras(t, f, dir, "a.txt", "uno\n")
	if err := tr.publicar(context.Background(), sha1); err != nil {
		t.Fatal(err)
	}
	sha2 := fotoTras(t, f, dir, "a.txt", "dos\n")
	runGit(t, dir, "push", "--force", remoto, sha2+":"+refTest) // avance ajeno
	sha3 := fotoTras(t, f, dir, "a.txt", "tres\n")

	if err := tr.publicar(context.Background(), sha3); err != nil {
		t.Fatalf("publicar debía enterarse y reintentar: %v", err)
	}
	if got := shaEnBare(t, bare, refTest); got != sha3 {
		t.Errorf("tras el reintento el remoto = %q, esperaba %q", got, sha3)
	}
	if tr.ultimoSha != sha3 {
		t.Errorf("ultimoSha = %q, esperaba %q", tr.ultimoSha, sha3)
	}
}

func TestTransporteRefFueraDeVivoRechazada(t *testing.T) {
	g := nuevoGitRunner(t.TempDir())
	for _, ref := range []string{"refs/heads/main", "refs/tags/x", "vivo/sin-prefijo", ""} {
		if _, err := nuevoTransporte(g, "origin", ref); err == nil {
			t.Errorf("ref %q fuera de refs/vivo/ debía ser rechazada", ref)
		}
	}
	if _, err := nuevoTransporte(g, "origin", "refs/vivo/n/s"); err != nil {
		t.Errorf("ref válida bajo refs/vivo/ fue rechazada: %v", err)
	}
}

func TestTransporteRemotoInvalidoRechazado(t *testing.T) {
	g := nuevoGitRunner(t.TempDir())
	for _, r := range []string{"--upload-pack=x", "-foo", ""} {
		if _, err := nuevoTransporte(g, r, "refs/vivo/n/s"); err == nil {
			t.Errorf("remoto %q debía ser rechazado", r)
		}
	}
}

// TestTransporteNoDisparaPrePush cubre R3 del lado del push: el push lleva --no-verify, así que el
// hook pre-push del repo del usuario (p. ej. el de migraciones de Altura) NO corre. Plantamos un
// pre-push que deja rastro y aborta (exit 1): si corriera, el push se cancelaría y la ref no se
// crearía. El sabotaje —quitar --no-verify— pone este test en rojo, que es lo que lo hace medir algo.
func TestTransporteNoDisparaPrePush(t *testing.T) {
	f, dir, remoto, bare := repoConRemoto(t)
	rastro := filepath.Join(dir, "PREPUSH-RASTRO.txt")
	hook := filepath.Join(dir, ".git", "hooks", "pre-push")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho corrio >> \""+rastro+"\"\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tr, err := nuevoTransporte(f.git, remoto, refTest)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := f.tomarFoto(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.publicar(context.Background(), sha); err != nil {
		t.Fatalf("publicar debía ignorar el pre-push (--no-verify): %v", err)
	}
	if _, err := os.Stat(rastro); err == nil {
		t.Error("el pre-push corrió: el push debe llevar --no-verify (R3)")
	}
	if got := shaEnBare(t, bare, refTest); got != sha {
		t.Errorf("ref remota = %q, esperaba la foto %q (el push no se completó)", got, sha)
	}
}
