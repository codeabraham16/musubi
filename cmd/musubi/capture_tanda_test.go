package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
	"musubi/internal/memory"
)

// capture_tanda_test.go custodia la TANDA CONGELADA de la captura (ver captureCommitsKeyed): el
// progreso dentro de una tanda vive en `<cursor>:hecho`, y la base salta al objetivo SÓLO al
// terminarla. Las pruebas con repos git reales son las de la revisión adversarial que refutó la
// primera versión —un cursor de un solo SHA sobre un historial con merges ciclaba para siempre—,
// y son el criterio de aceptación del rediseño.

const (
	claveHecho    = metaCaptureLastCommit + sufijoTandaHecho
	claveObjetivo = metaCaptureLastCommit + sufijoTandaObjetivo
)

// EL DEFECTO ORIGINAL (medido 2026-09-23): el avance se guardaba UNA vez, al final del bucle. El hook
// tiene 10 s; con 546 commits pendientes cada corrida moría a mitad y la siguiente arrancaba por el
// mismo commit. Veinte días con progreso cero. Lo ya procesado tiene que quedar aunque la corrida
// muera, y la corrida siguiente tiene que seguir de ahí, no repetirlo.
func TestCaptureProgresoDurableSobreviveAlCorte(t *testing.T) {
	store := &recordingStore{fallaTrasNGuardados: 2}
	commits := []commit{
		{SHA: "c1", Subject: "feat: el primero de la tanda"},
		{SHA: "c2", Subject: "fix: el segundo de la tanda"},
		{SHA: "c3", Subject: "feat: el tercero, que ya no entra"},
	}
	g := &fakeGit{head: "h5", commits: commits}
	if _, err := captureCommits(store, g, nil, nil, memory.ScopeLocal); err == nil {
		t.Fatal("la corrida tenía que cortarse: el store falla al tercer guardado")
	}
	if got := store.meta[claveHecho]; got != "c2" {
		t.Fatalf("lo ya procesado tiene que quedar: :hecho quería %q, obtuve %q", "c2", got)
	}
	if got := store.meta[metaCaptureLastCommit]; got != "" {
		t.Fatalf("a mitad de tanda la base NO se mueve: quedó en %q", got)
	}
	// La corrida siguiente sigue desde c3: guarda uno solo y cierra la tanda.
	store.fallaTrasNGuardados = 0
	n, err := captureCommits(store, g, nil, nil, memory.ScopeLocal)
	if err != nil || n != 1 {
		t.Fatalf("la corrida siguiente tenía que capturar SÓLO el que faltaba; n=%d err=%v", n, err)
	}
	if got := store.meta[metaCaptureLastCommit]; got != "h5" {
		t.Fatalf("terminada la tanda, la base salta al objetivo: %q", got)
	}
}

// Cortar por el tope no salta la base al objetivo —eso se tragaría lo que quedó sin mirar— y la
// corrida siguiente, con la MISMA lista, sigue desde el último hecho.
func TestCaptureTopePorCorridaNoSaltaAlObjetivo(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "c3", commits: []commit{
		{SHA: "c1", Subject: "feat: uno de la tanda larga"},
		{SHA: "c2", Subject: "feat: dos de la tanda larga"},
		{SHA: "c3", Subject: "feat: tres de la tanda larga"},
	}}
	if n, err := captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 2); err != nil || n != 2 {
		t.Fatalf("con tope 2 esperaba 2 guardados; n=%d err=%v", n, err)
	}
	if store.meta[metaCaptureLastCommit] != "" || store.meta[claveHecho] != "c2" || store.meta[claveObjetivo] != "c3" {
		t.Fatalf("cortar por tope deja la base quieta, :hecho=c2 y :objetivo=c3; quedó %v", store.meta)
	}
	if n, err := captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 2); err != nil || n != 1 {
		t.Fatalf("la corrida siguiente tenía que capturar el que quedó; n=%d err=%v", n, err)
	}
	if store.meta[metaCaptureLastCommit] != "c3" || store.meta[claveObjetivo] != "" || store.meta[claveHecho] != "" {
		t.Fatalf("tanda completa: base=objetivo y la tanda se limpia; quedó %v", store.meta)
	}
}

// El objetivo queda FIJO durante la tanda aunque el HEAD se mueva: si se recalculara con cada HEAD
// nuevo, la lista cambiaría entre corridas y «el último hecho» dejaría de ser una posición en ella.
// Los commits nuevos van a la tanda siguiente.
func TestCaptureElObjetivoQuedaFijoDuranteLaTanda(t *testing.T) {
	store := &recordingStore{meta: map[string]string{metaCaptureLastCommit: "base"}}
	g := &fakeGit{head: "h1", commits: []commit{
		{SHA: "c1", Subject: "feat: uno de la primera tanda"},
		{SHA: "c2", Subject: "feat: dos de la primera tanda"},
	}}
	_, _ = captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 1)
	g.head = "h2" // llegó un commit nuevo a mitad de la tanda
	_, _ = captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 1)
	if g.hastaWith != "h1" {
		t.Fatalf("la tanda en curso tiene que seguir hacia su objetivo fijo h1, no hacia %q", g.hastaWith)
	}
	if store.meta[metaCaptureLastCommit] != "h1" {
		t.Fatalf("terminada la tanda, la base queda en SU objetivo (h1), no en el HEAD nuevo: %q", store.meta[metaCaptureLastCommit])
	}
	// La tanda siguiente va de h1 al HEAD nuevo.
	g.commits = []commit{{SHA: "c3", Subject: "feat: el commit que llegó a mitad"}}
	_, _ = captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 1)
	if g.sinceWith != "h1" || g.hastaWith != "h2" || store.meta[metaCaptureLastCommit] != "h2" {
		t.Fatalf("la tanda siguiente tenía que ser h1..h2; fue %q..%q y la base quedó en %q", g.sinceWith, g.hastaWith, store.meta[metaCaptureLastCommit])
	}
}

// Sin SHA no hay dónde guardar el progreso: el tope no corta, o la corrida siguiente repetiría el
// mismo tramo para siempre.
func TestCaptureTopeNoSeAplicaSinProgreso(t *testing.T) {
	store := &recordingStore{}
	g := &fakeGit{head: "hx", commits: []commit{
		{Subject: "feat: uno sin sha en el registro"},
		{Subject: "feat: dos sin sha en el registro"},
		{Subject: "feat: tres sin sha en el registro"},
	}}
	n, err := captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 1)
	if err != nil || n != 3 {
		t.Fatalf("sin SHA el tope no puede cortar: esperaba los 3, obtuve %d (err %v)", n, err)
	}
	if got := store.meta[metaCaptureLastCommit]; got != "hx" {
		t.Fatalf("consumido el rango entero, la base salta al objetivo: %q", got)
	}
}

// Si el objetivo desaparece (rebase + gc), la tanda se abandona y la corrida siguiente arranca una
// nueva hacia el HEAD de ese momento: no queda trabada apuntando a un commit que ya no existe.
func TestCaptureObjetivoPerdidoAbandonaLaTanda(t *testing.T) {
	store := &recordingStore{meta: map[string]string{claveObjetivo: "borrado", claveHecho: "x"}}
	g := &gitQueFallaCon{fakeGit: fakeGit{head: "h9"}, hastaQueFalla: "borrado"}
	if _, err := captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 25); err == nil {
		t.Fatal("con el objetivo perdido la corrida tenía que avisar")
	}
	if store.meta[claveObjetivo] != "" || store.meta[claveHecho] != "" {
		t.Fatalf("la tanda perdida se abandona; quedó %v", store.meta)
	}
}

// gitQueFallaCon falla cuando se le pide un rango hacia un `hasta` que ya no existe.
type gitQueFallaCon struct {
	fakeGit
	hastaQueFalla string
	falla         error // si no es nil, todo rango falla con esto (un error que NO es «objetivo perdido»)
}

func (g *gitQueFallaCon) CommitsEntre(desde, hasta string) ([]commit, error) {
	if hasta == g.hastaQueFalla {
		return nil, fmt.Errorf("%w: fatal: bad revision '%s'", errObjetivoPerdido, hasta)
	}
	if g.falla != nil {
		return nil, g.falla
	}
	return g.fakeGit.CommitsEntre(desde, hasta)
}

// Un error del rango que NO es «el objetivo ya no existe» —un timeout de git, un lock— conserva la
// tanda con su progreso: tirarla obligaba a repasarla desde el principio, y la primera versión caía
// además a capturar sólo el HEAD, dando por capturado todo lo del medio.
//
// Sabotaje que la pone roja: abandonar la tanda ante cualquier error.
// arnes: archivo="cmd/musubi/capture.go"
// arnes: de="\t\tif errors.Is(err, errObjetivoPerdido) {\n\t\t\t_ = store.SetMeta(objKey, \"\")"
// arnes: a="\t\tif errors.Is(err, errObjetivoPerdido) || true {\n\t\t\t_ = store.SetMeta(objKey, \"\")"
func TestCaptureUnErrorTransitorioConservaLaTanda(t *testing.T) {
	store := &recordingStore{meta: map[string]string{claveObjetivo: "h9", claveHecho: "c3"}}
	g := &gitQueFallaCon{fakeGit: fakeGit{head: "h9"}, falla: fmt.Errorf("git log: signal: killed (timeout)")}
	if _, err := captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, 25); err == nil {
		t.Fatal("el error del rango tenía que avisarse")
	}
	if store.meta[claveObjetivo] != "h9" || store.meta[claveHecho] != "c3" {
		t.Fatalf("un error transitorio tiró la tanda y su progreso: %v", store.meta)
	}
}

// ─── Con repos git de verdad ────────────────────────────────────────────────────────────────────

// repoDePrueba arma un repo git temporal con identidad y fechas fijadas por el caller.
func repoDePrueba(t *testing.T) (dir string, git func(fecha string, args ...string) string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	dir = t.TempDir()
	base := append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(dir, "sin-config"), "GIT_CONFIG_NOSYSTEM=1")
	git = func(fecha string, args ...string) string {
		t.Helper()
		cmd := guiones.Herramienta(t, "git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(base, "GIT_COMMITTER_DATE="+fecha, "GIT_AUTHOR_DATE="+fecha)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	return dir, git
}

// commitDe escribe un archivo con el nombre del commit y lo commitea.
func commitDe(t *testing.T, dir string, git func(string, ...string) string, fecha, nombre string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nombre+".txt"), []byte(nombre), 0o644); err != nil {
		t.Fatal(err)
	}
	git(fecha, "add", ".")
	git(fecha, "commit", "-qm", "feat: commit "+nombre+" de la historia")
}

// corridasHastaAlDia repite la captura con el tope del hook hasta que la base llega al HEAD, y
// devuelve cuántas corridas hicieron falta (falla si no converge).
func corridasHastaAlDia(t *testing.T, store *recordingStore, dir string, tope, maximo int) int {
	t.Helper()
	g := realGit{dir: dir}
	head, err := g.Head()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= maximo; i++ {
		if _, err := captureCommitsKeyed(store, g, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, tope); err != nil {
			t.Fatalf("corrida %d: %v", i, err)
		}
		if store.meta[metaCaptureLastCommit] == head {
			return i
		}
	}
	t.Fatalf("%d corridas y la base no llegó al HEAD: %v", maximo, store.meta)
	return 0
}

// LA PRUEBA QUE REFUTÓ LA PRIMERA VERSIÓN: dos ramas de 30 commits con fechas intercaladas,
// mergeadas. Con un cursor de un solo SHA, «cursor..HEAD» volvía a meter lo ya procesado de la otra
// rama y la captura ciclaba A25 ↔ B25 sin llegar nunca al HEAD. Con la tanda congelada tiene que
// converger y capturar los 61 commits.
func TestCaptureRamasIntercaladasConvergen(t *testing.T) {
	dir, git := repoDePrueba(t)
	seg := 0
	fecha := func() string { seg++; return fmt.Sprintf("2026-09-01T10:%02d:%02d+00:00", seg/60, seg%60) }
	git(fecha(), "init", "-q", "-b", "main")
	commitDe(t, dir, git, fecha(), "base")
	baseSHA := git(fecha(), "rev-parse", "HEAD")
	git(fecha(), "branch", "b")
	for i := 1; i <= 30; i++ {
		git(fecha(), "checkout", "-q", "main")
		commitDe(t, dir, git, fecha(), fmt.Sprintf("A%02d", i))
		git(fecha(), "checkout", "-q", "b")
		commitDe(t, dir, git, fecha(), fmt.Sprintf("B%02d", i))
	}
	git(fecha(), "checkout", "-q", "main")
	git(fecha(), "merge", "-q", "--no-ff", "b", "-m", "merge b")
	commitDe(t, dir, git, fecha(), "after")

	store := &recordingStore{meta: map[string]string{metaCaptureLastCommit: baseSHA}}
	n := corridasHastaAlDia(t, store, dir, maxCommitsPorCorrida, 10)
	if len(store.byID) != 61 {
		t.Fatalf("convergió en %d corridas pero capturó %d commits; quería los 61", n, len(store.byID))
	}
}

// Con merges y commits del MISMO segundo, el orden por fecha de git log puede poner un hijo antes
// que su padre; con el cursor de un solo SHA, cortar justo ahí dejaba al padre afuera para siempre.
// Con la tanda congelada (y --topo-order) se capturan los seis.
func TestCaptureOrdenNoTopologicoNoPierdeCommits(t *testing.T) {
	dir, git := repoDePrueba(t)
	const mismoSegundo = "2026-09-01T10:00:00"
	git(mismoSegundo, "init", "-q", "-b", "main")
	commitDe(t, dir, git, mismoSegundo, "base")
	base := git(mismoSegundo, "rev-parse", "HEAD")
	commitDe(t, dir, git, mismoSegundo, "X")
	git(mismoSegundo, "checkout", "-qb", "b")
	for _, n := range []string{"D1", "D2", "D3"} {
		commitDe(t, dir, git, mismoSegundo, n)
	}
	git(mismoSegundo, "checkout", "-q", "main")
	commitDe(t, dir, git, mismoSegundo, "C1")
	git(mismoSegundo, "merge", "-q", "--no-ff", "b", "-m", "merge b")
	commitDe(t, dir, git, mismoSegundo, "after")

	store := &recordingStore{meta: map[string]string{metaCaptureLastCommit: base}}
	corridasHastaAlDia(t, store, dir, 2, 10)
	if len(store.byID) != 6 {
		t.Fatalf("esperaba los 6 commits (X, D1, D2, D3, C1, after); capturó %d", len(store.byID))
	}
}

// Contra la historia REAL de este repo, desde el cursor huérfano que la trabó (e928e67), con el tope
// del hook. Es la simulación que mostró el ciclo de la primera versión. Depende del repo real, así
// que sólo corre con MUSUBI_REPO_REAL apuntando a un clon con historia completa:
//
//	MUSUBI_REPO_REAL=C:/Proyectos/Musubi go test ./cmd/musubi/ -run TestCaptureHistoriaRealConverge -count=1 -v
func TestCaptureHistoriaRealConverge(t *testing.T) {
	dir := os.Getenv("MUSUBI_REPO_REAL")
	if dir == "" {
		t.Skip("MUSUBI_REPO_REAL vacío: prueba contra la historia real, sólo a mano")
	}
	store := &recordingStore{meta: map[string]string{metaCaptureLastCommit: "e928e673eb1b074797e07f663931c6632ead4141"}}
	n := corridasHastaAlDia(t, store, dir, maxCommitsPorCorrida, 100)
	t.Logf("desde el cursor huérfano e928e67: convergió en %d corridas, %d commits capturados", n, len(store.byID))
}
