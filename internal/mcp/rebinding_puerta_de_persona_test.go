package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// laMarcaDeQueNoEsDePersona es lo que tiene que decir una puerta que NO se envuelve con
// `puertaDePersona`. Va pegada a la línea, no en una lista central: una lista de excepciones
// envejece lejos del código que describe y nadie la revisa.
const laMarcaDeQueNoEsDePersona = "no es de persona:"

// ─────────────────────────────────────────────────────────────────────────────────────────────
// LA DEFENSA ANTI DNS-REBINDING CUBRE TODAS LAS PUERTAS DE PERSONA, NO UNA
// ─────────────────────────────────────────────────────────────────────────────────────────────

// TestNingunaPuertaDePersonaContestaAUnHostAjenoSinCredencial — la guarda del comportamiento.
//
// EL DEFECTO, MEDIDO EL 2026-09-20. La compuerta vivía adentro de la clausura de `/mcp`. Con el
// handler real en confianza local (sin credencial) y `Host: evil.example.com`:
//
//	POST /mcp         → 403   ← la única protegida
//	GET  /metrics     → 200   telemetría de la flota
//	GET  /api/actores → 200   el censo de qué tools invoca esa persona
//	GET  /api/stream  → 200   el feed EN VIVO
//	POST /api/flota   → 202   una ESCRITURA aceptada
//
// O sea DNS-rebinding clásico contra el musubi local de cualquiera: una página maliciosa lee el
// feed en vivo de cada tool que esa persona invoca, y escribe en él. Es el defecto dominante de
// este repo —la guarda en N−1 de N caminos— en su forma más cara.
//
// LA PROPIEDAD QUE AFIRMA ES UNA SOLA Y NO ENUMERA FORMAS: sin credencial, ninguna puerta de
// persona le contesta 2xx a un pedido con Host ajeno.
//
// Sabotaje que la hace fallar: sacarle el envoltorio a una puerta cualquiera.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\tmux.Handle(\"/api/flota\", puertaDePersona(opt, s.handlerFlota(opt)))"
// arnes: a="\tmux.HandleFunc(\"/api/flota\", s.handlerFlota(opt))"
func TestNingunaPuertaDePersonaContestaAUnHostAjenoSinCredencial(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	defer ts.Close()

	puertas := []struct {
		metodo, ruta, cuerpo string
	}{
		{"POST", "/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`},
		{"GET", "/metrics", ""},
		{"GET", "/api/actores", ""},
		{"POST", "/api/flota", `[{"at":"2026-09-20T23:00:00Z","tool":"inyectado","outcome":"ok","ms":1}]`},
		{"POST", shellOutPath, `{}`},
		{"POST", shellInPath, `{}`},
		{"POST", shellClosePath, `{}`},
	}

	ejercidas := 0
	for _, p := range puertas {
		codigo := pedir(t, ts.URL+p.ruta, p.metodo, p.cuerpo, "evil.example.com", "http://evil.example.com", "")
		ejercidas++
		if codigo >= 200 && codigo < 300 {
			t.Errorf("%s %s CONTESTÓ %d a un pedido con Host ajeno y SIN credencial.\n"+
				"  Eso es DNS-rebinding: una página cualquiera, desde el navegador de la persona, entra\n"+
				"  a su musubi local. El navegador no puede mentir el Host, y por eso el Host decide.\n"+
				"  Arreglo: registrala con `puertaDePersona(opt, <handler>)`. Si de verdad NO es una\n"+
				"  puerta de persona, decilo al lado con una línea que empiece por «%s» y el motivo.",
				p.metodo, p.ruta, codigo, laMarcaDeQueNoEsDePersona)
		}
	}

	// ── CONTROL: QUE TODO DÉ 403 NO PUEDE LEERSE COMO VERDE ──────────────────────────────
	// Sin esto, romper el servidor entero —o devolver 403 siempre— haría pasar la prueba. Las
	// mismas puertas, con Host loopback, tienen que CONTESTAR.
	vivas := 0
	for _, p := range puertas {
		if codigo := pedir(t, ts.URL+p.ruta, p.metodo, p.cuerpo, "", "", ""); codigo != http.StatusForbidden {
			vivas++
		}
	}
	if vivas == 0 {
		t.Fatal("con Host loopback NINGUNA puerta contestó algo distinto de 403: el servidor está roto " +
			"o la compuerta se comió todo, y entonces el cero de arriba no significa «cubierto», " +
			"significa «no medí nada»")
	}
	if ejercidas != len(puertas) {
		t.Fatalf("se ejercieron %d de %d puertas", ejercidas, len(puertas))
	}
	t.Logf("%d puertas ejercidas con Host ajeno; %d vivas con Host loopback", ejercidas, vivas)
}

// TestConCredencialElHostAjenoPasa — el otro lado, y es el que destraba el paso 3 de A129.
//
// El cerebro sirve detrás de un proxy TLS propio (`tailscale serve`) que PRESERVA el Host del
// tailnet. Con credencial, la compuerta no corre: el navegador de la víctima no puede fabricar un
// bearer, así que la defensa de Host no está comprando nada y sí rompía al proxy con un 403.
//
// Sabotaje: volver a colgar la compuerta de algo que no sea la credencial.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="func (o httpOptions) confianzaLocal() bool { return o.registry == nil && o.token == \"\" }"
// arnes: a="func (o httpOptions) confianzaLocal() bool { return true }"
func TestConCredencialElHostAjenoPasa(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	const tok = "bearer-de-una-persona"
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, token: tok}))
	defer ts.Close()

	// Con el bearer bueno y Host ajeno: el proxy pasando.
	if c := pedir(t, ts.URL+"/mcp", "POST", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		"musubi-server.tail89e295.ts.net:10000", "", tok); c != http.StatusOK {
		t.Errorf("con credencial y Host del tailnet, /mcp dio %d y tiene que dar 200: así entra el proxy TLS", c)
	}
	// SIN el bearer y Host ajeno: 401, no 403. La credencial es el gate, y el 401 es lo que el
	// verificador puede sondear sin tener token.
	if c := pedir(t, ts.URL+"/mcp", "POST", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		"musubi-server.tail89e295.ts.net:10000", "", ""); c != http.StatusUnauthorized {
		t.Errorf("sin credencial y Host del tailnet, /mcp dio %d y tiene que dar 401: un 403 ahí es la "+
			"compuerta comiéndose al proxy, que es el fallo medido en producción el 2026-09-20", c)
	}
}

// TestNingunaPuertaNaceSinDecirDeQuienEs — la guarda ESTRUCTURAL, y es la que hace que el camino
// malo deje de ser representable.
//
// La de arriba prueba comportamiento sobre las puertas que YO enumeré. Ésta pregunta otra cosa:
// ¿alguien registró una puerta nueva sin pasar por `puertaDePersona` y sin declarar que no lo es?
// Sin ella, la próxima puerta nace desprotegida y ninguna prueba de comportamiento la ve, porque
// nadie la agregó a la tabla.
//
// Sabotaje: registrar una puerta sin envoltorio ni marca.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t// no es de persona: readiness sin auth, la frontera es el tailnet."
// arnes: a="\t// readiness sin auth."
func TestNingunaPuertaNaceSinDecirDeQuienEs(t *testing.T) {
	ruta := filepath.Join("http.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, ruta, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("no pude parsear %s: %v", ruta, err)
	}

	var sinDeclarar []string
	personas, dispositivos := 0, 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "mux" {
			return true
		}
		linea := fset.Position(call.Pos()).Line
		if envuelveEnPuertaDePersona(call) {
			personas++
			return true
		}
		if motivoDeDispositivoCerca(f, fset, linea) {
			dispositivos++
			return true
		}
		sinDeclarar = append(sinDeclarar, fset.Position(call.Pos()).String())
		return true
	})

	// ── CONTROLES QUE NO PUEDEN ENMUDECER ────────────────────────────────────────────────
	if personas == 0 {
		t.Fatal("no se reconoció NINGUNA puerta envuelta en `puertaDePersona`: el reconocedor se rompió, " +
			"y entonces el cero de abajo no significa «todas declaran», significa «no estoy mirando»")
	}
	if dispositivos == 0 {
		t.Fatalf("no se reconoció NINGUNA puerta de dispositivo y hay varias (/healthz, /readyz, los tres "+
			"de /fleet/ y los dos del shell del agente): o se rompió el reconocedor de la marca «%s», "+
			"o alguien las envolvió y dejó a la flota afuera", laMarcaDeQueNoEsDePersona)
	}

	for _, d := range sinDeclarar {
		t.Errorf("la puerta registrada en %s no dice de quién es.\n"+
			"  Si la usa una PERSONA, registrala con `puertaDePersona(opt, <handler>)`: así nace con la\n"+
			"  defensa anti DNS-rebinding puesta, en vez de depender de que alguien se acuerde.\n"+
			"  Si la usa un DISPOSITIVO (un agente con su token, que llega por el proxy con el Host del\n"+
			"  tailnet), decilo al lado con una línea que empiece por «%s» y el motivo — envolverla\n"+
			"  dejaría a la flota incomunicada.", d, laMarcaDeQueNoEsDePersona)
	}
	t.Logf("%d puertas de persona, %d de dispositivo declaradas", personas, dispositivos)
}

// envuelveEnPuertaDePersona mira si ALGUNO de los argumentos del registro es una llamada a
// `puertaDePersona`. Se mira el argumento y no el texto de la línea porque el registro puede
// ocupar decenas de líneas (los literales de /metrics y /api/stream lo hacen).
func envuelveEnPuertaDePersona(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		c, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "puertaDePersona" {
			return true
		}
	}
	return false
}

// motivoDeDispositivoCerca mira las líneas de comentario INMEDIATAMENTE anteriores al registro. No
// se busca en todo el archivo a propósito: un comentario en otra función no declara nada sobre
// ésta, y esa confusión —el texto que está donde no decide— ya costó guardas verdes en este repo.
func motivoDeDispositivoCerca(f *ast.File, fset *token.FileSet, linea int) bool {
	for _, g := range f.Comments {
		if fset.Position(g.End()).Line != linea-1 {
			continue
		}
		texto := g.Text()
		i := strings.Index(texto, laMarcaDeQueNoEsDePersona)
		if i < 0 {
			continue
		}
		motivo := strings.TrimSpace(texto[i+len(laMarcaDeQueNoEsDePersona):])
		if len(strings.Fields(motivo)) >= 6 {
			return true
		}
	}
	return false
}

// pedir manda un request y devuelve el código. Host vacío ⇒ el de httptest (loopback).
func pedir(t *testing.T, url, metodo, cuerpo, host, origin, bearer string) int {
	t.Helper()
	req, err := http.NewRequest(metodo, url, strings.NewReader(cuerpo))
	if err != nil {
		t.Fatalf("%s %s: %v", metodo, url, err)
	}
	if host != "" {
		req.Host = host
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Content-Type", "application/json")
	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", metodo, url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
