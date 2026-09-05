package mcp

// Tests del cuerpo comprimido en /mcp (readRequestBody).
//
// El contexto es una falla que estuvo viva sin que nadie la viera: la federación del grafo de código
// manda el grafo entero en UN POST, ese cuerpo pasó los 4 MiB del central, y como el push es
// best-effort el index local seguía devolviendo verde con un `federated:false` al costado. El grafo
// del central quedó congelado. Lo que hay que proteger acá son DOS cosas a la vez, y tiran para
// lados opuestos: que un cuerpo grande legítimo AHORA ENTRE (G1/G4), y que aceptar gzip NO haya
// abierto una bomba de descompresión (G3, el test que importa).

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// postGzip manda un POST a /mcp con el cuerpo ya comprimido y el header puesto.
func postGzip(t *testing.T, baseURL string, cuerpo []byte) (*http.Response, JsonRpcResponse) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(cuerpo); err != nil {
		t.Fatalf("comprimir: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("cerrar gzip: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+mcpHTTPPath, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("construir request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST gzip /mcp: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var jr JsonRpcResponse
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &jr)
	}
	return resp, jr
}

// G1 — un cuerpo gzip válido se descomprime y se despacha igual que uno en claro.
func TestBodyGzipSeDespachaIgual(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, jr := postGzip(t, ts.URL, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, esperaba 200", resp.StatusCode)
	}
	if jr.Error != nil {
		t.Fatalf("el server rechazó un gzip válido: %+v", jr.Error)
	}
	m, ok := jr.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result no es objeto: %T", jr.Result)
	}
	if tools, ok := m["tools"].([]interface{}); !ok || len(tools) != toolsExpuestas() {
		t.Fatalf("por gzip esperaba las mismas %d tools, obtuve %v (%d)", toolsExpuestas(), ok, len(tools))
	}
}

// G2 — EL CUERPO QUE ANTES NO ENTRABA, AHORA ENTRA. Es la regresión concreta: un payload que en
// claro se pasa de los 4 MiB del cable, comprimido pasa sin tocar ese tope.
//
// El relleno es JSON repetitivo a propósito, porque así es el grafo real (miles de nodos con las
// mismas claves); un relleno aleatorio no comprimiría y el test mediría otra cosa.
func TestBodyGrandeEntraComprimidoYNoEnClaro(t *testing.T) {
	ts := newHTTPTestServer(t)

	relleno := strings.Repeat(`{"k":"path/al/simbolo#func:Nombre","kind":"func","line":42},`, 90_000)
	grande := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/list","_pad":[%s{"k":"fin"}]}`, relleno)
	if len(grande) <= maxRequestBody {
		t.Fatalf("el payload de prueba (%d bytes) no supera el tope de %d; el test no probaría nada", len(grande), maxRequestBody)
	}

	// En claro: rechazado, y el mensaje tiene que NOMBRAR el tope y sugerir gzip.
	_, jrClaro := postMCP(t, ts.URL, grande)
	if jrClaro.Error == nil {
		t.Fatalf("un body de %d bytes en claro debería superar el tope de %d y fue aceptado", len(grande), maxRequestBody)
	}
	if !strings.Contains(jrClaro.Error.Message, "gzip") || !strings.Contains(jrClaro.Error.Message, "tope") {
		t.Errorf("el error no orienta al que lo lee: %q", jrClaro.Error.Message)
	}

	// Comprimido: aceptado y despachado.
	resp, jrGzip := postGzip(t, ts.URL, []byte(grande))
	if resp.StatusCode != http.StatusOK || jrGzip.Error != nil {
		t.Fatalf("el mismo body comprimido debería entrar; status=%d error=%+v", resp.StatusCode, jrGzip.Error)
	}
}

// G3 — EL TEST QUE IMPORTA: la bomba de descompresión se corta en maxDecodedBody.
//
// Sin este tope, aceptar gzip sería cambiar un techo de 4 MiB de RAM por uno de gigabytes en un
// central always-on. Se manda un gzip chico en el cable que expande MUY por encima del tope y se
// exige que el server lo rechace nombrando el límite, en vez de intentar materializarlo.
func TestBombaDeDescompresionSeCorta(t *testing.T) {
	ts := newHTTPTestServer(t)

	// Ceros: comprimen ~1000:1, así que unos pocos KB en el cable pasan holgados el tope del
	// cable y aun así desbordan el de descompresión.
	bomba := make([]byte, maxDecodedBody+(1<<20))
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(bomba); err != nil {
		t.Fatalf("comprimir la bomba: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("cerrar la bomba: %v", err)
	}
	if buf.Len() > maxRequestBody {
		t.Fatalf("la bomba pesa %d en el cable y la cortaría el otro tope; el test no probaría el de descompresión", buf.Len())
	}

	req, err := http.NewRequest(http.MethodPost, ts.URL+mcpHTTPPath, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("construir request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST bomba: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var jr JsonRpcResponse
	_ = json.Unmarshal(raw, &jr)

	if jr.Error == nil {
		t.Fatalf("la bomba (%d bytes descomprimidos, %d en el cable) fue aceptada", len(bomba), buf.Len())
	}
	if !strings.Contains(jr.Error.Message, "descomprimido") {
		t.Errorf("rechazada, pero por otro motivo que el tope de descompresión: %q", jr.Error.Message)
	}
}

// G4 — el header puesto sobre un cuerpo que NO es gzip da su propio error, distinto del de tamaño.
// Los tres casos colapsaban en un mismo "error leyendo el body", y eso es lo que hizo que la falla
// del grafo tardara en encontrarse: desde el cliente era indistinguible de un bug de serialización.
func TestGzipInvalidoTieneSuPropioError(t *testing.T) {
	ts := newHTTPTestServer(t)
	req, err := http.NewRequest(http.MethodPost, ts.URL+mcpHTTPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("construir request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip") // miente
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var jr JsonRpcResponse
	_ = json.Unmarshal(raw, &jr)

	if jr.Error == nil {
		t.Fatal("un body que dice ser gzip y no lo es fue aceptado")
	}
	if !strings.Contains(jr.Error.Message, "gzip válido") {
		t.Errorf("el error no identifica el problema como gzip inválido: %q", jr.Error.Message)
	}
}

// G5 — sin el header, nada cambia. Guarda de compatibilidad: el camino en claro es el que usan
// todos los clientes de hoy y no debe haberse movido.
func TestSinHeaderElCaminoEnClaroSigueIgual(t *testing.T) {
	ts := newHTTPTestServer(t)
	resp, jr := postMCP(t, ts.URL, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if resp.StatusCode != http.StatusOK || jr.Error != nil {
		t.Fatalf("el POST en claro de siempre dejó de andar: status=%d error=%+v", resp.StatusCode, jr.Error)
	}
}

// nodosDePrueba fabrica n nodos con la forma del grafo real: claves largas y repetitivas.
func nodosDePrueba(n int) []memory.GraphNode {
	out := make([]memory.GraphNode, n)
	for i := range out {
		p := fmt.Sprintf("internal/paquete%03d/archivo_con_nombre_largo_%04d.go", i%200, i)
		out[i] = memory.GraphNode{
			Key:            p + fmt.Sprintf("#func:FuncionConNombreRazonablementeLargo%d", i),
			Kind:           "func",
			Name:           fmt.Sprintf("FuncionConNombreRazonablementeLargo%d", i),
			Path:           p,
			StartLine:      i,
			EndLine:        i + 20,
			SrcFingerprint: "1ef56c0623a94b1d8f0e7c2a5b3d9e4f6a8c0b2d4e6f8a0c2e4b6d8f0a2c4e6b",
		}
	}
	return out
}

// clienteContra arma un SyncClient apuntado a un handler de prueba.
func clienteContra(t *testing.T, h http.Handler) *SyncClient {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	c, err := NewSyncClient(config.SyncConfig{CentralURL: ts.URL, AllowInsecureToken: true, RequestTimeoutSeconds: 30})
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	return c
}

// G6 — el cliente comprime por encima del umbral, y lo que manda se puede volver a leer entero.
// No alcanza con mirar el header: lo que importa es que el central reciba los MISMOS nodos.
func TestPushGraphComprimeYLlegaCompleto(t *testing.T) {
	const cuantos = 8000
	var gotEnc string
	var gotNodos int
	c := clienteContra(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEnc = r.Header.Get("Content-Encoding")
		var lector io.Reader = r.Body
		if gotEnc == "gzip" {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Errorf("el cliente dijo gzip y no lo era: %v", err)
				return
			}
			defer zr.Close()
			lector = zr
		}
		var req struct {
			Params struct {
				Arguments struct {
					Nodes []memory.GraphNode `json:"nodes"`
				} `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(lector).Decode(&req); err != nil {
			t.Errorf("decodificar el push: %v", err)
			return
		}
		gotNodos = len(req.Params.Arguments.Nodes)
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`)
	}))

	nodos := nodosDePrueba(cuantos)
	if crudo, _ := json.Marshal(nodos); len(crudo) <= umbralCompresionPush {
		t.Fatalf("los %d nodos de prueba pesan %d, por debajo del umbral %d: el test no probaría la compresión", cuantos, len(crudo), umbralCompresionPush)
	}
	if err := c.PushGraph(nodos, nil, nil); err != nil {
		t.Fatalf("PushGraph: %v", err)
	}
	if gotEnc != "gzip" {
		t.Errorf("Content-Encoding = %q, esperaba gzip por encima del umbral", gotEnc)
	}
	if gotNodos != cuantos {
		t.Errorf("llegaron %d nodos de %d: la compresión perdió datos", gotNodos, cuantos)
	}
}

// G8 — END-TO-END: el cliente real contra el servidor real, con un grafo del PORTE del de
// producción. Es el test que reproduce la falla original de punta a punta.
//
// El tamaño no es arbitrario: el 2026-08-14 el push del repo de Musubi pesaba 4.958.147 bytes
// crudos (5.194 nodos, 11.225 aristas, 113 gists) contra un tope de 4 MiB, y el central lo
// rechazaba con -32700. Acá se fabrica un grafo que supera ese tope y se exige que cruce entero.
func TestPushDelPorteDeProduccionCruzaEntero(t *testing.T) {
	// EL PLAZO SE ESCALA BAJO `-race`, Y NO ES UN PARCHE (A53). Comprimir y serializar 5,2 MB con
	// el detector encima tarda más de 90 s —medido: 93,04 s, con cero `DATA RACE` reportados—, así
	// que con 60 s el test moría por `context deadline exceeded` y parecía una carrera. Este test
	// no mide latencia: mide que un grafo del porte del de producción cruce ENTERO. Achicar el
	// grafo no era opción, porque su razón de ser es superar `maxRequestBody`.
	plazo := 60
	if corriendoBajoDetector {
		plazo = 300
	}
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{
		reqTimeout: time.Duration(plazo) * time.Second, loopbackOnly: true}))
	t.Cleanup(ts.Close)

	c, err := NewSyncClient(config.SyncConfig{CentralURL: ts.URL, AllowInsecureToken: true, RequestTimeoutSeconds: plazo})
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}

	nodos := nodosDePrueba(14_000)
	crudo, _ := json.Marshal(nodos)
	if len(crudo) <= maxRequestBody {
		t.Fatalf("el grafo de prueba pesa %d, no llega al tope de %d: no reproduciría la falla", len(crudo), maxRequestBody)
	}

	if err := c.PushGraph(nodos, nil, nil); err != nil {
		t.Fatalf("un grafo de %d bytes crudos debería federar comprimido, y falló: %v", len(crudo), err)
	}
}

// G7 — GUARDA DE COMPATIBILIDAD: por debajo del umbral NO se comprime.
//
// Es la mitad menos vistosa del cambio y la que más fácil se rompe después. Un central viejo no
// entiende Content-Encoding: gzip; comprimir siempre convertiría los pushes chicos —que hoy
// funcionan— en errores de parseo. Si alguien "simplifica" esto sacando el umbral, falla acá.
func TestPushGraphChicoNoSeComprime(t *testing.T) {
	var gotEnc = "(no vino)"
	c := clienteContra(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("Content-Encoding"); v != "" {
			gotEnc = v
		} else {
			gotEnc = ""
		}
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`)
	}))
	if err := c.PushGraph(nodosDePrueba(10), nil, nil); err != nil {
		t.Fatalf("PushGraph: %v", err)
	}
	if gotEnc != "" {
		t.Errorf("un push chico salió con Content-Encoding %q; rompería contra un central viejo", gotEnc)
	}
}
