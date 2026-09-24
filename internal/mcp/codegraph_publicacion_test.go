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
// Sabotaje que la pone roja: no mirar si el commit está en la línea principal.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="rc == 0 && !contieneLinea(cadena, completo) {"
// arnes: a="rc == 0 && !contieneLinea(cadena, completo) && false {"
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

	// El mismo servidor, de vuelta en main con su índice: se publica, con el commit COMPLETO y su
	// fecha de commit. El checkout va antes: un sello que no es el HEAD de ahora no se publica.
	gitDePrueba(t, dir, "", "checkout", "-q", "main")
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

// Un push SIN commit sobre uno publicado con commit —un binario anterior a la guarda— se IGNORA con un
// resultado, no con un error: el binario viejo no conoce -32006, lo tomaría como falla transitoria y
// re-empujaría el grafo entero en cada tick logueando que el central falló. Con un resultado marca su
// generación y se calla. Y no toca nada: ni el grafo ni los gists.
//
// Sabotaje que la pone roja: contestarle al binario viejo con el error de rechazo.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="\t\tif errors.Is(err, memory.ErrGrafoIgnorado) {"
// arnes: a="\t\tif errors.Is(err, memory.ErrGrafoIgnorado) && false {"
func TestUnBinarioViejoRecibeUnResultadoYNoUnError(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	p := &Principal{Name: "davantis-2", Role: RoleWriter, ProjectID: "musubi"}
	ctx := withPrincipal(context.Background(), p)
	empujar := func(args map[string]interface{}) (interface{}, *RpcError) {
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: raw})
		return s.handleToolsCall(ctx, params)
	}
	nodo := func(n string) []memory.GraphNode {
		return []memory.GraphNode{{Key: n + ".go#func:" + n, Kind: "func", Name: n, Path: n + ".go"}}
	}
	if _, e := empujar(map[string]interface{}{"nodes": nodo("AlDia"), "edges": []memory.GraphEdge{},
		"gists": []memory.CodeMemory{{Path: "AlDia.go", Gist: "el gist al día"}},
		"head":  "fc3c297aaaa", "head_at": "2026-09-23T17:00:00Z"}); e != nil {
		t.Fatalf("publicar el grafo al día: %+v", e)
	}
	// Lo que manda un binario viejo: sin head y con sus gists.
	res, e := empujar(map[string]interface{}{"nodes": nodo("Rancio"), "edges": []memory.GraphEdge{},
		"gists": []memory.CodeMemory{{Path: "Rancio.go", Gist: "gist rancio"}}})
	if e != nil {
		t.Fatalf("al binario viejo se le contestó con un error (lo reintentaría cada tick): %+v", e)
	}
	if txt := textOf(t, res); !strings.Contains(txt, `"ignored":true`) || !strings.Contains(txt, "binario") {
		t.Errorf("el resultado tiene que decir que se ignoró y por qué: %s", txt)
	}
	own := memory.WithProjectScope(context.Background(), memory.ProjectScope{ProjectID: "musubi"})
	if n, _ := s.engine.AllGraphNodesCtx(own); len(n) != 1 || n[0].Name != "AlDia" {
		t.Fatalf("el push ignorado tocó el grafo: %+v", n)
	}
	if g := gistsDe(t, s, "musubi"); len(g) != 1 || g[0].Path != "AlDia.go" {
		t.Fatalf("el push ignorado tocó los gists: %+v", g)
	}
}

// El cliente nuevo lee el `ignored` del resultado: no loguea «empujado» y no reintenta.
//
// Sabotaje que la pone roja: no leer el resultado.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\tif motivo, ignorado := pushIgnorado(result); ignorado {"
// arnes: a="\tif motivo, ignorado := pushIgnorado(result); ignorado && false {"
func TestUnPushIgnoradoNoSeReintentaYDiceElMotivo(t *testing.T) {
	var pushes atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		pushes.Add(1)
		texto := `{"nodes":0,"edges":0,"ignored":true,"motivo":"este push no dice de qué commit es"}`
		cuerpo, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": "codegraph-push",
			"result": map[string]interface{}{"content": []map[string]string{{"type": "text", "text": texto}}}})
		_, _ = w.Write(cuerpo)
	}))
	t.Cleanup(ts.Close)
	s := servidorSobreElArbol(t, proyectoGoSinIndexar(t))
	s.SetSyncClient(newTestSyncClient(t, ts.URL), config.SyncConfig{BatchSize: 200})
	for i := 0; i < 3; i++ {
		s.reindexCodeGraphOnce(context.Background())
	}
	if n := pushes.Load(); n != 1 {
		t.Fatalf("el central ignoró el push y el cliente lo reintentó: %d pushes en 3 ticks (esperaba 1)", n)
	}
	if m := s.ultimoMotivoDelPush(); !strings.Contains(m, "commit") {
		t.Errorf("el motivo del ignorado tiene que quedar a la vista, quedó %q", m)
	}
}

// Una fecha de commit en el futuro se topa en la hora del central: sin el tope quedaría publicada y
// bloquearía todo commit real posterior hasta que el reloj la alcanzara.
//
// Sabotaje que la pone roja: sacar el tope.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="en.After(ahora.Add(topeFechaFutura)) {"
// arnes: a="en.After(ahora.Add(topeFechaFutura)) && false {"
func TestUnaFechaDeCommitEnElFuturoSeTopa(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	p := &Principal{Name: "davantis-1", Role: RoleWriter, ProjectID: "musubi"}
	ctx := withPrincipal(context.Background(), p)
	empujar := func(head string, en time.Time) *RpcError {
		args, _ := json.Marshal(map[string]interface{}{"nodes": []memory.GraphNode{}, "edges": []memory.GraphEdge{},
			"head": head, "head_at": en.UTC().Format(time.RFC3339)})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: args})
		_, e := s.handleToolsCall(ctx, params)
		return e
	}
	if e := empujar("adelantado", time.Now().Add(30*24*time.Hour)); e != nil {
		t.Fatalf("un commit con fecha adelantada se tenía que aceptar (topado): %+v", e)
	}
	if e := empujar("real", time.Now().Add(time.Minute)); e != nil {
		t.Fatalf("un commit real posterior quedó bloqueado por la fecha adelantada: %+v", e)
	}
}

// Lo que el grafo NO indexa no frena la publicación: un .md o un respaldo sin trackear no cambian el
// grafo, y en la PC de mando siempre hay alguno.
func TestUnArchivoNoIndexableSinCommitearNoFrenaLaPublicacion(t *testing.T) {
	dir, enMain, _ := repoConRamaDeTrabajo(t)
	gitDePrueba(t, dir, "", "checkout", "-q", "main")
	writeFile(t, filepath.Join(dir, "docs", "notas.md"), "borrador\n")
	writeFile(t, filepath.Join(dir, "config.yaml.antes"), "x: 1\n")
	s := servidorSobreElArbol(t, dir)
	if err := s.engine.SetMeta(memory.MetaCodegraphHead, enMain[:7]); err != nil {
		t.Fatal(err)
	}
	if _, publicable, motivo := s.origenDelGrafo(); !publicable {
		t.Fatalf("un .md y un .yaml sin commitear frenaron la publicación: %s", motivo)
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
