package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// codegraph_recibo_test.go custodia el RECIBO del mapa de código (punto 47A): el central contesta
// lo que GUARDÓ —no el eco de lo que recibió— y quién publicó; el emisor lo compara con lo que mandó;
// dos índices distintos del mismo commit se avisan; y una máquina sin gists no borra los del central.

// pushDeLaTool llama a musubi_codegraph_push como lo haría el daemon de `quien`, y devuelve el JSON
// del resultado.
func pushDeLaTool(t *testing.T, s *McpServer, quien string, args map[string]interface{}) map[string]interface{} {
	t.Helper()
	raw, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_codegraph_push", Arguments: raw})
	ctx := withPrincipal(context.Background(), &Principal{Name: quien, Role: RoleWriter, ProjectID: "musubi"})
	res, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr != nil {
		t.Fatalf("push de %s: %+v", quien, rpcErr)
	}
	var cuerpo map[string]interface{}
	if err := json.Unmarshal([]byte(textOf(t, res)), &cuerpo); err != nil {
		t.Fatalf("el resultado del push no es JSON: %v", err)
	}
	return cuerpo
}

// resultadoDelPush arma la respuesta de un central de mentira con `texto` adentro de content[0].
func resultadoDelPush(w http.ResponseWriter, texto string) {
	cuerpo, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": "codegraph-push",
		"result": map[string]interface{}{"content": []map[string]string{{"type": "text", "text": texto}}}})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(cuerpo)
}

// El central contesta lo que GUARDÓ, no el largo de lo que recibió: un nodo con la clave repetida
// cuenta una vez y un gist sin contenido no cuenta. Y dice quién publicó, sacado de la credencial.
// Las claves de siempre (`nodes`, `edges`, `gists`) siguen siendo el eco, por compatibilidad.
//
// Sabotaje que la pone roja: volver al eco (contestar lo recibido como guardado).
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="guardados := map[string]interface{}{\"nodes\": c.Nodes, \"edges\": c.Edges}"
// arnes: a="guardados := map[string]interface{}{\"nodes\": len(args.Nodes), \"edges\": c.Edges}"
//
// Sabotaje que la pone roja: no atribuir la publicación a la credencial.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="pub.Por = authorFrom(principalFrom(ctx))"
// arnes: a="pub.Por = \"\""
func TestElCentralDevuelveLoQueGuardoYQuienLoEmpujo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	a := memory.GraphNode{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go"}
	b := memory.GraphNode{Key: "b.go#func:B", Kind: "func", Name: "B", Path: "b.go"}
	res := pushDeLaTool(t, s, "davantis-2", map[string]interface{}{
		"nodes":   []memory.GraphNode{a, a, b},
		"edges":   []memory.GraphEdge{{FromKey: a.Key, ToKey: b.Key, Kind: "CALLS"}},
		"gists":   []memory.CodeMemory{{Path: "a.go", Gist: "el gist de a"}, {Path: "b.go", Gist: ""}},
		"head":    "eb2cdb7aaaa",
		"head_at": "2026-09-24T22:22:54Z",
		"huella":  "h-pc",
	})
	if res["nodes"] != float64(3) || res["gists"] != float64(2) {
		t.Errorf("`nodes` y `gists` tienen que seguir siendo el eco de lo recibido (3 y 2): %v", res)
	}
	guardados, _ := res["guardados"].(map[string]interface{})
	if guardados["nodes"] != float64(2) || guardados["edges"] != float64(1) || guardados["gists"] != float64(1) {
		t.Errorf("`guardados` tiene que ser lo que quedó en la base (2 nodos, 1 arista, 1 gist), dio %v", res["guardados"])
	}
	publicado, _ := res["publicado"].(map[string]interface{})
	if publicado["por"] != "davantis-2" || publicado["huella"] != "h-pc" || publicado["head"] != "eb2cdb7aaaa" {
		t.Errorf("`publicado` tiene que decir el commit, quién lo empujó y la huella, dio %v", res["publicado"])
	}
	if pub, _ := s.engine.PublicacionDelGrafoDe("musubi"); pub.Por != "davantis-2" {
		t.Errorf("la publicación guardada no recuerda la credencial que la empujó: %+v", pub)
	}
}

// El mismo commit con OTRO contenido se acepta —no hay forma de saber cuál de los dos lo describe
// bien— pero con un aviso que dice quién tenía lo anterior y cuánto tenía. Es el caso medido el
// 2026-09-24: la laptop y esta PC con eb2cdb7, 12.053 nodos contra 13.329, y el mapa del central
// yendo y viniendo según cuál empujó última, sin que nadie lo viera.
//
// Sabotaje que la pone roja: no comparar las huellas.
// arnes: archivo="internal/memory/codegraph_publicacion.go"
// arnes: de="nueva.Huella != \"\" && vigente.Huella != \"\" && nueva.Huella != vigente.Huella {"
// arnes: a="nueva.Huella != \"\" && vigente.Huella != \"\" && nueva.Huella != vigente.Huella && false {"
//
// Sabotaje que la pone roja: calcular el aviso y no contestarlo.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="\t\tres[\"aviso\"] = aviso\n"
// arnes: a="\n"
func TestElMismoCommitConOtraHuellaSeAceptaConAviso(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	a := memory.GraphNode{Key: "a.go#func:A", Kind: "func", Name: "A", Path: "a.go"}
	b := memory.GraphNode{Key: "boceto-g.js#func:B", Kind: "func", Name: "B", Path: "boceto-g.js"}
	empujar := func(quien, huella string, nodos ...memory.GraphNode) map[string]interface{} {
		return pushDeLaTool(t, s, quien, map[string]interface{}{"nodes": nodos, "edges": []memory.GraphEdge{},
			"head": "eb2cdb7aaaa", "head_at": "2026-09-24T22:22:54Z", "huella": huella})
	}
	if r := empujar("davantis-mando-admin", "h-pc", a, b); r["aviso"] != nil {
		t.Errorf("la primera publicación no tiene nada que avisar: %v", r["aviso"])
	}
	if r := empujar("davantis-mando-admin", "h-pc", a, b); r["aviso"] != nil {
		t.Errorf("re-empujar el mismo contenido no tiene nada que avisar: %v", r["aviso"])
	}

	r := empujar("davantis-2", "h-laptop", a)
	aviso := fmt.Sprint(r["aviso"])
	for _, debe := range []string{"eb2cdb7aaaa", "lo publicó davantis-mando-admin", "tenía 2 nodos", "trae 1"} {
		if !strings.Contains(aviso, debe) {
			t.Errorf("el aviso tiene que decir %q, dijo %q", debe, aviso)
		}
	}
	// Se aceptó: gana el último, y la publicación ahora es suya.
	if c, _ := s.engine.ConteoDelGrafoDe("musubi"); c.Nodes != 1 {
		t.Errorf("el push con aviso tenía que reemplazar el grafo, quedaron %d nodos", c.Nodes)
	}
	if pub, _ := s.engine.PublicacionDelGrafoDe("musubi"); pub.Por != "davantis-2" || pub.Huella != "h-laptop" {
		t.Errorf("la publicación no pasó a la laptop: %+v", pub)
	}
}

// El emisor LEE el recibo: si el central guardó otra cantidad que la mandada, es un error con los
// números. Si el central no manda `guardados` —uno anterior al recibo, o los stubs que contestan
// `result:{}`— no compara: la ausencia de recibo no es un recibo malo.
//
// Sabotaje que la pone roja: no leer el recibo.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="len(edges), len(gists)); falta {"
// arnes: a="len(edges), len(gists)); falta && false {"
//
// Sabotaje que la pone roja: que el recibo no compare los gists.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\tif g.Gists != nil && *g.Gists != gists {"
// arnes: a="\tif false && g.Gists != nil && *g.Gists != gists {"
func TestPushGraphDeDetectaUnReciboQueNoCuadra(t *testing.T) {
	var texto atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resultadoDelPush(w, texto.Load().(string))
	}))
	t.Cleanup(ts.Close)
	c := newTestSyncClient(t, ts.URL)
	nodos := []memory.GraphNode{{Key: "a"}, {Key: "b"}, {Key: "c"}}
	aristas := []memory.GraphEdge{{FromKey: "a", ToKey: "b", Kind: "CALLS"}}
	un := []memory.CodeMemory{{Path: "a.go", Gist: "g"}}

	casos := []struct {
		nombre, texto string
		gists         []memory.CodeMemory
		falta         string // "" = tiene que cuadrar
	}{
		{"cuadra", `{"nodes":3,"edges":1,"guardados":{"nodes":3,"edges":1}}`, nil, ""},
		{"central anterior al recibo", `{"nodes":3,"edges":1}`, nil, ""},
		{"le falta un nodo", `{"nodes":3,"edges":1,"guardados":{"nodes":2,"edges":1}}`, nil, "2 de 3 nodos"},
		{"le falta el gist", `{"nodes":3,"edges":1,"gists":1,"guardados":{"nodes":3,"edges":1,"gists":0}}`, un, "0 de 1 gists"},
	}
	for _, caso := range casos {
		texto.Store(caso.texto)
		err := c.PushGraphDe(memory.PublicacionDelGrafo{}, nodos, aristas, caso.gists)
		switch {
		case caso.falta == "" && err != nil:
			t.Errorf("%s: tenía que cuadrar y dio %v", caso.nombre, err)
		case caso.falta != "" && !errors.Is(err, errReciboNoCuadra):
			t.Errorf("%s: el recibo no cuadra y el push se dio por bueno (err=%v)", caso.nombre, err)
		case caso.falta != "" && (!strings.Contains(err.Error(), caso.falta) || !errors.Is(err, errPermanent)):
			t.Errorf("%s: el error tiene que ser permanente y decir %q, dijo %v", caso.nombre, caso.falta, err)
		}
	}
}

// De punta a punta por el daemon: un recibo que no cuadra deja federated:false CON los números y NO
// se reintenta en cada tick —la misma foto daría el mismo recibo, y cada reintento es el grafo entero
// por la red—. Se marca como empujada, igual que un rechazo por viejo.
//
// Sabotaje que la pone roja: no marcar la generación cuando el recibo no cuadra.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if rerr := s.engine.MarcarGrafoEmpujado(foto.Generacion, time.Now()); rerr != nil {"
// arnes: a="if rerr := error(nil); rerr != nil {"
func TestUnReciboQueNoCuadraSeDiceYNoSeReintenta(t *testing.T) {
	var pushes atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushes.Add(1)
		var req struct {
			Params struct {
				Arguments struct {
					Nodes []memory.GraphNode `json:"nodes"`
					Edges []memory.GraphEdge `json:"edges"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		n, e := len(req.Params.Arguments.Nodes), len(req.Params.Arguments.Edges)
		resultadoDelPush(w, fmt.Sprintf(`{"nodes":%d,"edges":%d,"guardados":{"nodes":%d,"edges":%d}}`, n, e, n-1, e))
	}))
	t.Cleanup(ts.Close)
	s := servidorSobreElArbol(t, proyectoGoSinIndexar(t))
	s.SetSyncClient(newTestSyncClient(t, ts.URL), config.SyncConfig{BatchSize: 200})

	for i := 0; i < 3; i++ {
		s.reindexCodeGraphOnce(context.Background())
	}
	if n := pushes.Load(); n != 1 {
		t.Fatalf("el recibo no cuadró y el cliente reintentó la misma foto: %d pushes en 3 ticks (esperaba 1)", n)
	}
	if m := s.ultimoMotivoDelPush(); !strings.Contains(m, "guardó") || !strings.Contains(m, "nodos") {
		t.Errorf("el motivo tiene que decir cuánto guardó el central, quedó %q", m)
	}
	if est, _ := s.engine.EstadoDelPushDelGrafo(); est.Empujada != est.Generacion {
		t.Errorf("la foto con recibo malo tenía que quedar marcada como empujada: %+v", est)
	}
}

// UNA MÁQUINA SIN GISTS NO BORRA LOS DEL CENTRAL. Medido el 2026-09-24: la laptop, con
// `code_memory` vacía, mandaba `"gists": []` en cada push, y para el receptor eso es «reemplazá los
// míos por nada»: el central quedaba en cero gists hasta que esta PC volvía a empujar. Contra un
// central REAL, con el daemon real: la foto de una base sin gists no habla de gists. Y de paso, el
// recibo contra un central real cuadra (ningún «no cuadra» falso).
//
// Sabotaje que la pone roja: volver a mandar la lista vacía.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="`json:\"gists,omitempty\"`"
// arnes: a="`json:\"gists\"`"
func TestUnaMaquinaSinGistsNoBorraLosDelCentral(t *testing.T) {
	central := newTestServer(t, embedding.NoopProvider{})
	if err := central.engine.ReplaceProjectCodeMemoryFrom("", gistEjemplo); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(central.HTTPHandler(httpOptions{reqTimeout: 30 * time.Second}))
	t.Cleanup(ts.Close)

	laptop := servidorSobreElArbol(t, proyectoGoSinIndexar(t))
	laptop.SetSyncClient(newTestSyncClient(t, ts.URL), config.SyncConfig{BatchSize: 200})
	laptop.reindexCodeGraphOnce(context.Background())

	c, err := central.engine.ConteoDelGrafoDe("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Nodes == 0 {
		t.Fatalf("andamio: el push de la laptop no llegó al central (motivo %q)", laptop.ultimoMotivoDelPush())
	}
	if c.Gists != len(gistEjemplo) {
		t.Errorf("una máquina sin gists le dejó al central %d de %d gists", c.Gists, len(gistEjemplo))
	}
	if m := laptop.ultimoMotivoDelPush(); m != "" {
		t.Errorf("contra un central real el push tenía que cuadrar, y dijo %q", m)
	}
}

// El push lleva la huella del CONTENIDO que manda: la misma con otro orden, otra si falta un nodo.
//
// Sabotaje que la pone roja: calcular la huella sin los nodos.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="memory.HuellaDelGrafo(nodes, edges)"
// arnes: a="memory.HuellaDelGrafo(nil, edges)"
func TestPushGraphDeMandaLaHuellaDelContenido(t *testing.T) {
	var huella atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					Huella string `json:"huella"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		huella.Store(req.Params.Arguments.Huella)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	t.Cleanup(ts.Close)
	c := newTestSyncClient(t, ts.URL)
	a, b := memory.GraphNode{Key: "a.go#func:A"}, memory.GraphNode{Key: "b.go#func:B"}
	llama := []memory.GraphEdge{{FromKey: a.Key, ToKey: b.Key, Kind: "CALLS"}}
	mandada := func(nodos ...memory.GraphNode) string {
		if err := c.PushGraph(nodos, llama, nil); err != nil {
			t.Fatal(err)
		}
		return huella.Load().(string)
	}
	h := mandada(a, b)
	if h == "" || h != memory.HuellaDelGrafo([]memory.GraphNode{a, b}, llama) {
		t.Fatalf("el push no llevó la huella de lo que mandó: %q", h)
	}
	if otra := mandada(b, a); otra != h {
		t.Errorf("el mismo contenido en otro orden salió con otra huella")
	}
	if otra := mandada(a); otra == h {
		t.Errorf("sin un nodo el push salió con la misma huella")
	}
}
