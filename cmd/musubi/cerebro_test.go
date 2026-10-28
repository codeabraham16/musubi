package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/guiones"
)

// `musubi cerebro` existe porque el cliente MCP-sobre-HTTP de Claude Code NO manda los `headers`
// declarados en el .mcp.json (bug anthropics/claude-code#48514): la credencial nunca llega. Acá el
// header lo pone Musubi, así que no hay nada que el cliente pueda omitir. Estos tests fijan las
// tres cosas que hacen que el canal funcione o muera en el handshake.

// stubCerebro imita el /mcp del central: captura el Authorization que recibe y responde eco.
func stubCerebro(t *testing.T, capturado *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*capturado = r.Header.Get("Authorization")
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if len(req.ID) == 0 {
			req.ID = json.RawMessage("null")
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"ok":true}}`))
	}))
}

// corre `musubi cerebro` como subproceso real (es un comando de stdio: se testea por sus pipes) y
// devuelve las líneas que escribió en stdout.
func correrCerebro(t *testing.T, url, token string, entrada string) []string {
	t.Helper()
	// -run con un test inexistente: el binario de test se re-ejecuta a sí mismo como helper.
	cmd := guiones.Herramienta(t, os.Args[0], "-test.run=TestHelperCerebro")
	cmd.Env = append(os.Environ(),
		"GO_QUIERO_SER_CEREBRO=1",
		"MUSUBI_CENTRAL_URL="+url,
		"MANDO_MUSUBI_TOKEN="+token,
	)
	cmd.Stdin = strings.NewReader(entrada)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("musubi cerebro falló: %v (stdout=%q)", err, out)
	}
	var lineas []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			lineas = append(lineas, l)
		}
	}
	return lineas
}

// TestHelperCerebro no es un test: es el proceso hijo que corre runCerebro.
func TestHelperCerebro(t *testing.T) {
	if os.Getenv("GO_QUIERO_SER_CEREBRO") != "1" {
		t.Skip("helper, no es un test")
	}
	runCerebro(nil)
	os.Exit(0)
}

// LA RAZÓN DE SER DEL COMANDO: el Authorization llega al cerebro. Si esto se rompe, el canal entero
// deja de tener sentido — es exactamente lo que el cliente HTTP de Claude Code no hace.
func TestCerebroMandaElTokenAlCentral(t *testing.T) {
	var auth string
	srv := stubCerebro(t, &auth)
	defer srv.Close()

	lineas := correrCerebro(t, srv.URL, "tok-secreto", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n")

	if auth != "Bearer tok-secreto" {
		t.Errorf("el cerebro recibió Authorization=%q, esperaba 'Bearer tok-secreto'", auth)
	}
	if len(lineas) != 1 || !strings.Contains(lineas[0], `"id":1`) {
		t.Errorf("esperaba 1 respuesta con id=1, obtuve %v", lineas)
	}
}

// Una NOTIFICACIÓN (sin "id") NO lleva respuesta. Si respondiéramos igual, el cliente vería una
// respuesta huérfana y cerraría la sesión: el canal moriría justo después del initialize, que es
// cuando el cliente manda notifications/initialized.
func TestCerebroNoRespondeNotificaciones(t *testing.T) {
	var auth string
	srv := stubCerebro(t, &auth)
	defer srv.Close()

	entrada := `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	lineas := correrCerebro(t, srv.URL, "tok", entrada)

	if len(lineas) != 2 {
		t.Fatalf("esperaba 2 respuestas (la notificación NO responde), obtuve %d: %v", len(lineas), lineas)
	}
	if !strings.Contains(lineas[0], `"id":1`) || !strings.Contains(lineas[1], `"id":2`) {
		t.Errorf("las respuestas no corresponden a los ids 1 y 2: %v", lineas)
	}
}

// EL BOM: un productor que escribe UTF-8 "con firma" (PowerShell, por caso) antepone \xef\xbb\xbf
// al stream. Esa marca INVISIBLE rompe el parseo de la PRIMERA línea — que es justo el `initialize`.
// Encontrado en vivo: el canal contestaba tools/list pero no el handshake, y el cliente moría.
func TestCerebroToleraElBOM(t *testing.T) {
	var auth string
	srv := stubCerebro(t, &auth)
	defer srv.Close()

	conBOM := "\xef\xbb\xbf" + `{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n"
	lineas := correrCerebro(t, srv.URL, "tok", conBOM)

	if len(lineas) != 1 || !strings.Contains(lineas[0], `"id":1`) {
		t.Errorf("una línea con BOM debe procesarse igual; obtuve %v", lineas)
	}
}

// …y el defecto MÁS GRAVE que destapó el BOM: si el JSON no parsea, NO es una notificación. Antes
// se trataban igual, así que una línea corrupta DESAPARECÍA EN SILENCIO y el cliente esperaba para
// siempre una respuesta que nunca iba a llegar. Un ilegible se responde con error de parseo.
func TestCerebroNoSeTragaUnJSONIlegible(t *testing.T) {
	var auth string
	srv := stubCerebro(t, &auth)
	defer srv.Close()

	lineas := correrCerebro(t, srv.URL, "tok", "{esto no es json}\n")
	if len(lineas) != 1 {
		t.Fatalf("un JSON ilegible debe RESPONDER un error, no desaparecer; obtuve %v", lineas)
	}
	var r struct {
		Error *struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lineas[0]), &r); err != nil || r.Error == nil {
		t.Fatalf("esperaba un error JSON-RPC, obtuve %v", lineas[0])
	}
	if r.Error.Code != -32700 {
		t.Errorf("un JSON ilegible es un parse error (-32700), obtuve %d", r.Error.Code)
	}
}

// Si el cerebro no contesta, el error vuelve como un error JSON-RPC CON EL MISMO ID. Sin esto el
// cliente se queda esperando para siempre una respuesta que nunca llega.
func TestCerebroDevuelveErrorConElMismoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	lineas := correrCerebro(t, srv.URL, "tok", `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`+"\n")
	if len(lineas) != 1 {
		t.Fatalf("esperaba 1 respuesta de error, obtuve %v", lineas)
	}
	var r struct {
		ID    int `json:"id"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(lineas[0]), &r); err != nil {
		t.Fatalf("respuesta no parseable: %v", lineas[0])
	}
	if r.ID != 7 {
		t.Errorf("el error volvió con id=%d, debe volver con el id del pedido (7)", r.ID)
	}
	if r.Error == nil {
		t.Fatal("esperaba un error JSON-RPC")
	}
}

// EL ARREGLO: el `initialize` del MCP se responde LOCALMENTE, sin discar el central. Antes se
// reenviaba, y un dial en frío que pasara de los 5 s dejaba al cliente MCP marcando este server
// `failed` para TODA la sesión aunque el central estuviera vivo (medido 2026-10-28). Acá se apunta
// a una IP que NO rutea (el central caído): si el handshake se respondiera reenviando, esto pagaría
// el dial y volvería un error; como se responde local, vuelve un result válido y al instante.
func TestCerebroSaludaInicializacionSinDiscar(t *testing.T) {
	entrada := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

	arranque := time.Now()
	lineas := correrCerebro(t, "http://"+ipQueNoResponde, "tok", entrada)
	tardo := time.Since(arranque)

	// Exactamente UNA línea: el result del initialize. La notificación no responde.
	if len(lineas) != 1 {
		t.Fatalf("esperaba 1 respuesta (el saludo; la notificación no responde), obtuve %d: %v", len(lineas), lineas)
	}
	// Y DEBE ser un result con el handshake, NO un error: un reenvío a la IP muerta sólo podría
	// devolver un error -32603, nunca un protocolVersion.
	var r struct {
		ID     int `json:"id"`
		Result *struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      *struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal([]byte(lineas[0]), &r); err != nil {
		t.Fatalf("respuesta no parseable: %v", lineas[0])
	}
	if len(r.Error) != 0 {
		t.Fatalf("el initialize se respondió con un error en vez de local: %v", lineas[0])
	}
	if r.Result == nil || r.Result.ProtocolVersion == "" {
		t.Fatalf("el saludo local debe traer un result con protocolVersion; obtuve %v", lineas[0])
	}
	if r.ID != 1 {
		t.Errorf("el saludo debe ecoar el id del pedido (1), obtuve %d", r.ID)
	}
	if r.Result.ServerInfo == nil || r.Result.ServerInfo.Name != "musubi-cerebro" {
		t.Errorf("el serverInfo.name debe ser 'musubi-cerebro', obtuve %v", lineas[0])
	}
	// Prueba de que NO discó: un reenvío a la IP muerta bloquearía los 5 s del dial. El saludo local
	// no toca la red, así que el ciclo entero (incluido el spawn del subproceso) entra MUY por debajo.
	if tardo > 4*time.Second {
		t.Errorf("tardó %v: con la IP muerta eso es haber discado; el saludo local no toca la red", tardo.Round(time.Millisecond))
	}
}

// LA CONTRACARA: el handshake no sólo se responde local, tampoco se REENVÍA. Un stub que cuenta los
// pedidos comprueba que al central sólo le llega el tools/list — ni el initialize ni la notificación,
// que son los dos dials en frío de más que producían el incidente #275.
func TestCerebroNoReenviaElHandshakeAlCentral(t *testing.T) {
	var mu sync.Mutex
	var metodos []string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		metodos = append(metodos, req.Method)
		auth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if len(req.ID) == 0 {
			req.ID = json.RawMessage("null")
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"ok":true}}`))
	}))
	defer srv.Close()

	entrada := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	lineas := correrCerebro(t, srv.URL, "tok-h", entrada)

	// El cliente ve 2 respuestas: el saludo local (id 1) y el tools/list del central (id 2).
	if len(lineas) != 2 {
		t.Fatalf("esperaba 2 respuestas (saludo local + tools/list), obtuve %d: %v", len(lineas), lineas)
	}
	// Pero el CENTRAL debió recibir SOLO el tools/list.
	mu.Lock()
	defer mu.Unlock()
	if len(metodos) != 1 || metodos[0] != "tools/list" {
		t.Fatalf("el central debió recibir SOLO tools/list; recibió %v", metodos)
	}
	if auth != "Bearer tok-h" {
		t.Errorf("el tools/list reenviado debe llevar el Bearer; auth=%q", auth)
	}
}

// EL ATAJO DEL HANDSHAKE RESPETA LA DISTINCIÓN REQUEST/NOTIFICACIÓN. El initialize es SIEMPRE un
// request (lleva id) y la notificación de cierre NUNCA lo lleva. Un mensaje malformado que invierta
// eso —un initialize SIN id, o un notifications/initialized CON id— NO debe tomar el atajo local:
// cae al reenvío de siempre, para no contradecir la regla JSON-RPC que el propio archivo fija
// (cerebro.go: una notificación no se responde). Un cliente MCP conforme jamás manda estos casos,
// pero el atajo no puede violar esa regla por un caso de borde. Los dos van a un stub que cuenta:
// si alguno se atajara local, el central NO lo vería y la cuenta fallaría.
func TestCerebroElAtajoRespetaRequestVsNotificacion(t *testing.T) {
	var mu sync.Mutex
	var metodos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		mu.Lock()
		metodos = append(metodos, req.Method)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if len(req.ID) == 0 {
			req.ID = json.RawMessage("null")
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + string(req.ID) + `,"result":{"ok":true}}`))
	}))
	defer srv.Close()

	// initialize SIN id (malformado) y notifications/initialized CON id (malformado): ninguno toma
	// el atajo local, los dos se reenvían.
	entrada := `{"jsonrpc":"2.0","method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":9,"method":"notifications/initialized"}` + "\n"
	lineas := correrCerebro(t, srv.URL, "tok", entrada)

	mu.Lock()
	defer mu.Unlock()
	// Los dos malformados llegaron al central (no se atajaron local).
	if len(metodos) != 2 || metodos[0] != "initialize" || metodos[1] != "notifications/initialized" {
		t.Fatalf("un handshake malformado NO debe tomar el atajo local; el central recibió %v", metodos)
	}
	// Y la respuesta sigue la regla JSON-RPC: el initialize SIN id no se responde; el
	// notifications/initialized CON id sí, porque trae id. Queda exactamente 1 línea, la del id 9.
	if len(lineas) != 1 || !strings.Contains(lineas[0], `"id":9`) {
		t.Fatalf("esperaba 1 respuesta, la del mensaje con id=9; obtuve %v", lineas)
	}
}
