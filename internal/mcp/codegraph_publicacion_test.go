package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// sinVariablesDeGit saca del proceso las variables que git exporta al correr un hook (pre-push corre
// las pruebas): con GIT_DIR puesto, `git -C <temp>` miraría el repo de Musubi y no el de la prueba.
// El código de producción hereda el entorno, así que no alcanza con limpiarlo en los comandos de acá.
func sinVariablesDeGit(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		clave, valor, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(clave, "GIT_") {
			_ = os.Unsetenv(clave)
			t.Cleanup(func() { _ = os.Setenv(clave, valor) })
		}
	}
}

// gitDePrueba corre git en dir con fechas fijas: la guarda compara fechas de COMMIT, así que la
// prueba no puede depender del reloj.
func gitDePrueba(t *testing.T, dir, fecha string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=prueba", "GIT_AUTHOR_EMAIL=p@x", "GIT_COMMITTER_NAME=prueba", "GIT_COMMITTER_EMAIL=p@x",
		"GIT_AUTHOR_DATE="+fecha, "GIT_COMMITTER_DATE="+fecha)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// repoConRamaDeTrabajo arma lo que tenía la laptop el 2026-09-24: una rama de trabajo que NO está en
// origin/main. Devuelve el dir, el commit de main y el de la rama.
func repoConRamaDeTrabajo(t *testing.T) (dir, enMain, enRama string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	sinVariablesDeGit(t)
	dir = t.TempDir()
	gitDePrueba(t, dir, "2026-09-12T16:04:09Z", "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	gitDePrueba(t, dir, "2026-09-12T16:04:09Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-12T16:04:09Z", "commit", "-q", "-m", "base")
	enMain = gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	// El remoto sin red: lo que importa es la referencia origin/main, que es lo que mira la guarda.
	gitDePrueba(t, dir, "", "update-ref", "refs/remotes/origin/main", enMain)
	gitDePrueba(t, dir, "2026-09-13T10:00:00Z", "checkout", "-q", "-b", "fix/rama-de-trabajo")
	writeFile(t, filepath.Join(dir, "b.go"), "package a\n")
	gitDePrueba(t, dir, "2026-09-13T10:00:00Z", "add", ".")
	gitDePrueba(t, dir, "2026-09-13T10:00:00Z", "commit", "-q", "-m", "trabajo")
	enRama = gitDePrueba(t, dir, "", "rev-parse", "HEAD")
	return dir, enMain, enRama
}

func servidorSobreElArbol(t *testing.T, dir string) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	return NewMcpServer(engine, dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
}

// LA MITAD DEL CLIENTE: un árbol que no está en la rama principal remota no se publica. Es el caso
// medido: la laptop publicó una rama de trabajo del 12/09 encima del grafo al día.
//
// Sabotaje que la pone roja: tratar «no es ancestro» como publicable.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="\tcase 1:\n\t\treturn pub, false,"
// arnes: a="\tcase 1:\n\t\treturn pub, true,"
func TestElGrafoDeUnaRamaDeTrabajoNoSePublica(t *testing.T) {
	dir, enMain, enRama := repoConRamaDeTrabajo(t)
	s := servidorSobreElArbol(t, dir)

	if err := s.engine.SetMeta(memory.MetaCodegraphHead, enRama[:7]); err != nil {
		t.Fatal(err)
	}
	pub, publicable, motivo := s.origenDelGrafo()
	if publicable {
		t.Fatalf("el índice es de una rama que no está en origin/main y se iba a publicar (pub=%+v)", pub)
	}
	if !strings.Contains(motivo, "origin/main") {
		t.Errorf("el motivo tiene que decir contra qué se comparó, dijo %q", motivo)
	}

	// El mismo servidor, con el índice de main: se publica, con el commit COMPLETO y su fecha de commit.
	if err := s.engine.SetMeta(memory.MetaCodegraphHead, enMain[:7]); err != nil {
		t.Fatal(err)
	}
	pub, publicable, _ = s.origenDelGrafo()
	if !publicable {
		t.Fatal("el índice de un commit de origin/main tenía que publicarse")
	}
	if pub.Head != enMain {
		t.Errorf("head = %q, esperaba el commit completo %q", pub.Head, enMain)
	}
	if want := time.Date(2026, 9, 12, 16, 4, 9, 0, time.UTC); !pub.En.Equal(want) {
		t.Errorf("la fecha tiene que ser la de COMMIT (%v), dio %v", want, pub.En)
	}
}

// Ante la duda se publica y decide el central: sin rama principal conocida no hay contra qué
// comparar, y callar el push dejaría al central atrás sin que nadie lo vea.
func TestSinRamaPrincipalConocidaSePublica(t *testing.T) {
	dir, _, enRama := repoConRamaDeTrabajo(t)
	gitDePrueba(t, dir, "", "update-ref", "-d", "refs/remotes/origin/main")
	s := servidorSobreElArbol(t, dir)
	if err := s.engine.SetMeta(memory.MetaCodegraphHead, enRama[:7]); err != nil {
		t.Fatal(err)
	}
	if pub, publicable, _ := s.origenDelGrafo(); !publicable || pub.Head != enRama {
		t.Fatalf("sin origin/main se tenía que publicar con su commit: publicable=%v pub=%+v", publicable, pub)
	}
}

// Una rama de trabajo no sale a la red: preguntar es local, y el push —el grafo entero— no se manda.
// La tool lo dice con un motivo, no con un federated:false mudo.
func TestUnaRamaDeTrabajoNoMandaElGrafoPorLaRed(t *testing.T) {
	dir, _, _ := repoConRamaDeTrabajo(t)
	central := nuevoCentralQueCuentaPushes(t)
	s := servidorFederado(t, dir, central)
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")

	res := mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	if n := central.pushes.Load(); n != 0 {
		t.Fatalf("el grafo de una rama de trabajo salió al central (%d pushes): %v", n, res)
	}
	var cuerpo map[string]interface{}
	_ = json.Unmarshal([]byte(textOf(t, res)), &cuerpo)
	if cuerpo["federated"] != false || !strings.Contains(fmt.Sprint(cuerpo["federated_motivo"]), "origin/main") {
		t.Errorf("la tool tiene que decir federated=false CON el motivo: %v", cuerpo)
	}
}

// centralQueRechazaPorViejo contesta como un central con la guarda y un grafo más nuevo publicado.
func centralQueRechazaPorViejo(t *testing.T) (url string, pushes *atomic.Int64) {
	t.Helper()
	pushes = &atomic.Int64{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		pushes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","error":{"code":-32006,"message":"el grafo empujado es de un árbol más viejo que el publicado"}}`))
	}))
	t.Cleanup(ts.Close)
	return ts.URL, pushes
}

// Un rechazo por viejo NO se reintenta en cada tick: la foto es vieja y va a seguir siéndolo, y cada
// reintento es el grafo entero por la red. Se marca como empujada, igual que un push aceptado.
//
// Sabotaje que la pone roja: no marcar la generación tras el rechazo.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if merr := s.engine.MarcarGrafoEmpujado(foto.Generacion, time.Now()); merr != nil {"
// arnes: a="if merr := error(nil); merr != nil {"
func TestUnRechazoPorViejoNoSeReintentaEnCadaTick(t *testing.T) {
	url, pushes := centralQueRechazaPorViejo(t)
	dir := proyectoGoSinIndexar(t)
	s := servidorSobreElArbol(t, dir)
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 200})

	for i := 0; i < 3; i++ {
		s.reindexCodeGraphOnce(context.Background())
	}
	if n := pushes.Load(); n != 1 {
		t.Fatalf("el central rechazó por viejo y el cliente reintentó el grafo entero: %d pushes en 3 ticks (esperaba 1)", n)
	}
	if m := s.ultimoMotivoDelPush(); !strings.Contains(m, "más viejo") {
		t.Errorf("el motivo del rechazo tiene que quedar a la vista, quedó %q", m)
	}
}

// LA MITAD DEL CENTRAL, por la tool: un push de un árbol más viejo que el publicado se rechaza con
// codeGrafoViejo y el grafo queda como estaba.
//
// Sabotaje que la pone roja: que el receptor vuelva al reemplazo sin guarda.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if err := s.engine.ReplaceProjectGraphPublicado(origin, pub, args.Nodes, args.Edges); err != nil {"
// arnes: a="if err := s.engine.ReplaceProjectGraphFrom(origin, args.Nodes, args.Edges); pub.Vacia() && err != nil {"
func TestElCentralNoAceptaUnGrafoMasViejo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	p := &Principal{Name: "davantis-2", Role: RoleWriter, ProjectID: "musubi"}
	ctx := withPrincipal(context.Background(), p)
	empujar := func(nombre, head, fecha string) *RpcError {
		args, _ := json.Marshal(map[string]interface{}{
			"nodes":   []memory.GraphNode{{Key: nombre + ".go#func:" + nombre, Kind: "func", Name: nombre, Path: nombre + ".go"}},
			"edges":   []memory.GraphEdge{},
			"head":    head,
			"head_at": fecha,
		})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: args})
		_, rpcErr := s.handleToolsCall(ctx, params)
		return rpcErr
	}
	if e := empujar("AlDia", "fc3c297aaaa", "2026-09-23T17:00:00Z"); e != nil {
		t.Fatalf("publicar el grafo al día: %+v", e)
	}
	e := empujar("Rancio", "8de3b00bbbb", "2026-09-12T16:04:09Z")
	if e == nil || e.Code != codeGrafoViejo {
		t.Fatalf("un árbol más viejo tenía que rechazarse con codeGrafoViejo, dio %+v", e)
	}
	own := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: "musubi"})
	if n, _ := s.engine.AllGraphNodesCtx(own); len(n) != 1 || n[0].Name != "AlDia" {
		t.Fatalf("el rechazo tocó el grafo publicado: %+v", n)
	}
	if e := empujar("X", "abc", "ayer"); e == nil || e.Code != codeInvalidParams {
		t.Errorf("un head_at ilegible es un pedido mal formado, dio %+v", e)
	}
}

// El push lleva de qué árbol es cuando lo sabe, y es idéntico al de antes cuando no.
func TestPushGraphDeMandaElCommitYSuFecha(t *testing.T) {
	var cuerpo atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		cuerpo.Store(string(b))
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	defer ts.Close()
	c := newTestSyncClient(t, ts.URL)

	pub := memory.PublicacionDelGrafo{Head: "fc3c297aaaa", En: time.Date(2026, 9, 23, 17, 0, 0, 0, time.UTC)}
	if err := c.PushGraphDe(pub, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	got := cuerpo.Load().(string)
	if !strings.Contains(got, `"head":"fc3c297aaaa"`) || !strings.Contains(got, `"head_at":"2026-09-23T17:00:00Z"`) {
		t.Errorf("el push no dijo de qué árbol es: %s", got)
	}
	if err := c.PushGraph(nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := cuerpo.Load().(string); strings.Contains(got, `"head"`) {
		t.Errorf("sin publicación el push tiene que ser el de antes, sin head: %s", got)
	}
}

// De punta a punta: el índice de un árbol de origin/main sale con SU commit completo y su fecha, que
// es lo que el central necesita para no dejar que otro árbol más viejo lo pise después.
//
// Sabotaje que la pone roja: averiguar el commit y no ponerlo en la publicación.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="pub = memory.PublicacionDelGrafo{Head: completo, En: en}"
// arnes: a="pub = memory.PublicacionDelGrafo{Head: completo[:0], En: en}"
func TestElPushDesdeMainLlevaSuCommit(t *testing.T) {
	dir, enMain, _ := repoConRamaDeTrabajo(t)
	gitDePrueba(t, dir, "", "checkout", "-q", "main")
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")

	var cuerpo atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		cuerpo.Store(string(b))
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	defer ts.Close()
	s := servidorSobreElArbol(t, dir)
	s.SetSyncClient(newTestSyncClient(t, ts.URL), config.SyncConfig{BatchSize: 200})

	mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{"mode": "incremental"})
	got, _ := cuerpo.Load().(string)
	if !strings.Contains(got, `"head":"`+enMain+`"`) || !strings.Contains(got, `"head_at":"2026-09-12T16:04:09Z"`) {
		t.Fatalf("el push desde main no llevó su commit y su fecha:\n%.400s", got)
	}
}
