package mcp

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/logx"
)

func servidorConRegistro(t *testing.T) *httptest.Server {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	reg := &PrincipalRegistry{principals: []Principal{
		{Name: "buena", Role: RoleWriter, Read: ReadAll, Write: WriteOwn, hash: hashToken("token-bueno")},
	}}
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	t.Cleanup(ts.Close)
	return ts
}

func llamarMCP(t *testing.T, url, bearer string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+mcpHTTPPath,
		bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// EL CABO A88, EN UNA PRUEBA. El candado es POR IP: un cliente roto que comparte máquina con una
// persona le agotaba los intentos y la dejaba afuera con su credencial válida en la mano. Pasó de
// verdad el 2026-09-05 y costó horas de diagnóstico.
//
// Sabotaje que la hace fallar: mover el chequeo de limiter.locked() de vuelta ARRIBA de resolver el
// token en HTTPHandler.
func TestUnaCredencialValidaEntraAunqueLaIPEsteCastigada(t *testing.T) {
	ts := servidorConRegistro(t)

	// El vecino ruidoso gasta sus intentos. Los primeros cuatro dan 401: todavía queda margen.
	for i := 0; i < 4; i++ {
		if got := llamarMCP(t, ts.URL, "token-que-no-existe"); got != http.StatusUnauthorized {
			t.Fatalf("intento %d: se esperaba 401 mientras quedan intentos, obtuve %d", i+1, got)
		}
	}
	// El QUINTO arma el castigo y ya lo cobra: newAuthLimiter(5, ...) bloquea al llegar a cinco,
	// no al superarlos. Vale dejarlo escrito, porque «cinco intentos» se lee fácil como «cinco 401
	// y el sexto castigado», y no es así.
	if got := llamarMCP(t, ts.URL, "token-que-no-existe"); got != http.StatusTooManyRequests {
		t.Fatalf("el quinto fallo ya castiga la IP: se esperaba 429, obtuve %d", got)
	}
	// Y ACÁ ESTÁ LO QUE SE ARREGLÓ: la persona, misma IP, credencial buena, entra.
	if got := llamarMCP(t, ts.URL, "token-bueno"); got != http.StatusOK {
		t.Fatalf("una credencial VÁLIDA no puede pagar por el vecino: se esperaba 200, obtuve %d", got)
	}
}

// El acierto además destraba la IP, que es lo que hace que el sistema se recupere solo.
func TestUnAuthBuenoLevantaElCastigoDeLaIP(t *testing.T) {
	ts := servidorConRegistro(t)
	for i := 0; i < 4; i++ {
		llamarMCP(t, ts.URL, "malo")
	}
	if got := llamarMCP(t, ts.URL, "malo"); got != http.StatusTooManyRequests {
		t.Fatalf("se esperaba 429 en el quinto, obtuve %d", got)
	}
	if got := llamarMCP(t, ts.URL, "token-bueno"); got != http.StatusOK {
		t.Fatalf("se esperaba 200, obtuve %d", got)
	}
	// Tras el acierto el contador quedó en cero: vuelve a haber cinco intentos antes del castigo.
	if got := llamarMCP(t, ts.URL, "malo"); got != http.StatusUnauthorized {
		t.Fatalf("después de un auth bueno el contador se resetea: se esperaba 401, obtuve %d", got)
	}
}

// La segunda mitad de A88: el sistema tiene que poder decir QUIÉN falló y POR QUÉ. Sin esto, una
// tormenta de 401 es indiagnosticable desde adentro — que fue exactamente lo que pasó.
func TestElRechazoDeAuthQuedaAtribuidoEnElLog(t *testing.T) {
	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	defer restaurar()

	ts := servidorConRegistro(t)
	llamarMCP(t, ts.URL, "") // sin cabecera Authorization

	linea := log.String()
	if !strings.Contains(linea, motivoSinCredencial) {
		t.Fatalf("el log tiene que distinguir «sin credencial» de «credencial desconocida»; dijo: %s", linea)
	}
	if !strings.Contains(linea, "127.0.0.1") {
		t.Fatalf("el log tiene que nombrar la IP de origen, que es TODO el punto del cabo; dijo: %s", linea)
	}
	if strings.Contains(linea, "token-bueno") {
		t.Fatalf("el log jamás puede llevar una credencial adentro; dijo: %s", linea)
	}
}

func TestElMotivoDistingueCredencialDesconocidaDeCredencialAusente(t *testing.T) {
	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	defer restaurar()

	ts := servidorConRegistro(t)
	llamarMCP(t, ts.URL, "un-token-cualquiera")

	if s := log.String(); !strings.Contains(s, motivoCredencialDesconocida) {
		t.Fatalf("una credencial presentada y rechazada no es lo mismo que ninguna credencial; dijo: %s", s)
	}
}

// El freno del registro: una IP en bucle no puede escribir el journal a la velocidad que quiera.
func TestElRegistroDeAuthFrenaPeroCuentaLoQueCalla(t *testing.T) {
	r := nuevoRegistroDeAuth(time.Minute, 16)
	t0 := time.Unix(1788600000, 0)

	if !r.fallo(t0, "10.0.0.1", motivoSinCredencial, "/mcp", "") {
		t.Fatal("la primera falla de una IP se escribe siempre: avisar temprano es el punto")
	}
	for i := 1; i <= 4; i++ {
		if r.fallo(t0.Add(time.Duration(i)*time.Second), "10.0.0.1", motivoSinCredencial, "/mcp", "") {
			t.Fatalf("la falla %d cayó dentro de la ventana y no debía escribirse", i)
		}
	}
	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	defer restaurar()
	if !r.fallo(t0.Add(2*time.Minute), "10.0.0.1", motivoSinCredencial, "/mcp", "") {
		t.Fatal("pasada la ventana tiene que volver a escribir")
	}
	if s := log.String(); !strings.Contains(s, "callados_desde_el_ultimo_aviso") || !strings.Contains(s, "4") {
		t.Fatalf("el freno no puede esconder el volumen: la línea debe decir cuántas se callaron; dijo: %s", s)
	}
}

// Otra IP no comparte el freno: si lo compartiera, el ruidoso taparía al que recién empieza a
// fallar, que es justo el que hay que ver.
func TestElFrenoDelRegistroEsPorIP(t *testing.T) {
	r := nuevoRegistroDeAuth(time.Minute, 16)
	t0 := time.Unix(1788600000, 0)
	r.fallo(t0, "10.0.0.1", motivoSinCredencial, "/mcp", "")
	if !r.fallo(t0, "10.0.0.2", motivoSinCredencial, "/mcp", "") {
		t.Fatal("una IP distinta tiene su propia ventana")
	}
}
