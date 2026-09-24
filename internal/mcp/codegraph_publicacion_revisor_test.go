package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// centralDeVerdad reenvía cada push a un McpServer central REAL (la guarda de verdad, no un stub
// que contesta fijo), atribuido a un writer del proyecto "musubi".
func centralDeVerdad(t *testing.T) (*McpServer, string) {
	t.Helper()
	central := newTestServer(t, embedding.NoopProvider{})
	p := &Principal{Name: "davantis-2", Role: RoleWriter, ProjectID: "musubi"}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req struct {
			ID     interface{}     `json:"id"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(b, &req)
		res, rpcErr := central.handleToolsCall(withPrincipal(context.Background(), p), req.Params)
		resp := okResponse(req.ID, res)
		if rpcErr != nil {
			resp = errResponse(req.ID, rpcErr)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(ts.Close)
	return central, ts.URL
}

func nombresEnElCentral(t *testing.T, central *McpServer) map[string]bool {
	t.Helper()
	own := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: "musubi"})
	ns, err := central.engine.AllGraphNodesCtx(own)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]bool{}
	for _, n := range ns {
		m[n.Name] = true
	}
	return m
}

// repoSoloMain: un repo con un commit en main que además es origin/main.
func repoSoloMain(t *testing.T) (dir, enMain string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	dir = t.TempDir()
	gitDePrueba(t, dir, "2026-09-12T16:04:09Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nfunc EnMain() {}\n")
	gitDePrueba(t, dir, "2026-09-12T16:04:09Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-12T16:04:09Z", "commit", "-q", "-m", "base")
	enMain = gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	gitDePrueba(t, dir, "", "update-ref", "refs/remotes/origin/main", enMain)
	return dir, enMain
}

// HALLAZGO 1: trabajo SIN COMMITEAR sobre un HEAD que está en origin/main se publica etiquetado
// como el commit de main, y el central lo acepta por «mismo head».
//
// Sabotaje que la pone roja: no mirar el árbol sucio.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if sucio := s.primerArchivoIndexableSinCommitear(); sucio != \"\" {"
// arnes: a="if sucio := s.primerArchivoIndexableSinCommitear(); sucio != \"\" && false {"
func TestRevisorUnArbolSucioSobreMainSePublicaComoMain(t *testing.T) {
	dir, enMain := repoSoloMain(t)
	central, url := centralDeVerdad(t)

	a := servidorSobreElArbol(t, dir)
	a.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	mustCall(t, a, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if pub, _ := central.engine.PublicacionDelGrafoDe("musubi"); pub.Head != enMain {
		t.Fatalf("precondición: el central tenía que publicar main limpio, tiene %+v", pub)
	}

	gitDePrueba(t, dir, "", "checkout", "-q", "-b", "fix/wip")
	writeFile(t, filepath.Join(dir, "wip.go"), "package a\n\nfunc BorradorSinCommitear() {}\n")
	b := servidorSobreElArbol(t, dir)
	b.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	res := mustCall(t, b, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})

	if nombresEnElCentral(t, central)["BorradorSinCommitear"] {
		pub, _ := central.engine.PublicacionDelGrafoDe("musubi")
		t.Fatalf("el central publicó trabajo SIN COMMITEAR de fix/wip etiquetado como %s (pub=%+v)\nrespuesta: %s",
			enMain[:7], pub, textOf(t, res))
	}
}

// HALLAZGO 2: un índice con UN directorio fallido no re-sella el head, el sello queda en el commit
// de main anterior, y el grafo de la RAMA DE TRABAJO sale publicado con ese sello.
//
// Sabotaje que la pone roja: no comparar el sello con el HEAD de ahora.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="rc == 0 && head != \"\" && head != completo {"
// arnes: a="rc == 0 && head != \"\" && head != completo && false {"
func TestRevisorUnIndiceConFallidosPublicaLaRamaComoMain(t *testing.T) {
	dir, enMain := repoSoloMain(t)
	gitDePrueba(t, dir, "2026-09-13T10:00:00Z", "checkout", "-q", "-b", "fix/rama")
	writeFile(t, filepath.Join(dir, "b.go"), "package a\n\nfunc SoloEnLaRama() {}\n")
	gitDePrueba(t, dir, "2026-09-13T10:00:00Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-13T10:00:00Z", "commit", "-q", "-m", "trabajo")
	gitDePrueba(t, dir, "", "checkout", "-q", "main")

	central, url := centralDeVerdad(t)
	s := servidorSobreElArbol(t, dir)
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	ctx := context.Background()

	s.reindexCodeGraphOnce(ctx) // main: sella enMain y publica
	if pub, _ := central.engine.PublicacionDelGrafoDe("musubi"); pub.Head != enMain {
		t.Fatalf("precondición: el central tenía que publicar main, tiene %+v", pub)
	}

	// La terminal pasa a la rama de trabajo, y en ese tick UN directorio falla (acá, la guarda de
	// contención con un nodo de otro repo; en producción vale igual un SQLITE_BUSY).
	gitDePrueba(t, dir, "", "checkout", "-q", "fix/rama")
	ajeno := t.TempDir()
	writeFile(t, filepath.Join(ajeno, "x.go"), "package x\n")
	abs := filepath.ToSlash(filepath.Join(ajeno, "x.go"))
	if err := s.engine.UpsertPackageGraphFrom("", []string{abs},
		[]memory.GraphNode{{Key: abs + "#file", Kind: "file", Name: "x.go", Path: abs}}, nil); err != nil {
		t.Fatal(err)
	}
	s.reindexCodeGraphOnce(ctx)

	sello, _, _ := s.engine.GetMeta(memory.MetaCodegraphHead)
	if nombresEnElCentral(t, central)["SoloEnLaRama"] {
		pub, _ := central.engine.PublicacionDelGrafoDe("musubi")
		t.Fatalf("el grafo de fix/rama se publicó en el central como %s (sello local=%s, pub=%+v)", enMain[:7], sello, pub)
	}
}

// HALLAZGO 3: la fecha de commit no ordena árboles cuando main integra con MERGE COMMIT.
//
// Sabotaje que la pone roja: volver a «es ancestro» en lugar de «está en la línea de primer padre».
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="s.gitDelArbol(\"rev-list\", \"--first-parent\", principal)"
// arnes: a="s.gitDelArbol(\"rev-list\", principal)"
func TestRevisorUnCommitMergeadoMasNuevoPisaAMainSinSusCambios(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	dir := t.TempDir()
	gitDePrueba(t, dir, "2026-09-10T10:00:00Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n\nfunc EnMain() {}\n")
	gitDePrueba(t, dir, "2026-09-10T10:00:00Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-10T10:00:00Z", "commit", "-q", "-m", "base")
	gitDePrueba(t, dir, "2026-09-15T10:00:00Z", "checkout", "-q", "-b", "feat")
	writeFile(t, filepath.Join(dir, "c.go"), "package a\n\nfunc DeLaRama() {}\n")
	gitDePrueba(t, dir, "2026-09-15T10:00:00Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-15T10:00:00Z", "commit", "-q", "-m", "feat")
	enRama := gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	gitDePrueba(t, dir, "", "checkout", "-q", "main")
	writeFile(t, filepath.Join(dir, "m2.go"), "package a\n\nfunc DeMain2() {}\n")
	gitDePrueba(t, dir, "2026-09-14T10:00:00Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-14T10:00:00Z", "commit", "-q", "-m", "m2")
	enM2 := gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	gitDePrueba(t, dir, "2026-09-16T10:00:00Z", "merge", "-q", "--no-ff", "-m", "merge feat", "feat")
	gitDePrueba(t, dir, "", "update-ref", "refs/remotes/origin/main", gitDePrueba(t, dir, "", "rev-parse", "HEAD"))

	central, url := centralDeVerdad(t)
	gitDePrueba(t, dir, "", "checkout", "-q", "--detach", enM2) // máquina X: main sin el merge
	x := servidorSobreElArbol(t, dir)
	x.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	mustCall(t, x, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if !nombresEnElCentral(t, central)["DeMain2"] {
		t.Fatal("precondición: X tenía que publicar M2")
	}
	gitDePrueba(t, dir, "", "checkout", "-q", "feat") // máquina Y: sentada en la rama ya mergeada
	y := servidorSobreElArbol(t, dir)
	y.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})
	mustCall(t, y, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if !nombresEnElCentral(t, central)["DeMain2"] {
		pub, _ := central.engine.PublicacionDelGrafoDe("musubi")
		t.Fatalf("el central retrocedió: perdió DeMain2 (de %s) al aceptar %s, que no lo tiene (pub=%+v)", enM2[:7], enRama[:7], pub)
	}
}
