package mcp

// La bajada viaja comprimida (ola 2, frente sync): el central comprime la respuesta de /mcp cuando
// el pedido nombra gzip y el cuerpo pasa de 1 KiB, y el cliente del sync la pide así A MANO para
// poder medir los dos tamaños.
//
// Se protegen tres cosas, y las dos primeras tiran para lados opuestos: que la página grande viaje
// comprimida y llegue ENTERA (el ahorro); que a quien no la pidió le llegue en claro, byte a byte
// como hoy (ningún cliente del central se rompe), y que sync_viajes cuente el cable ANTES de
// descomprimir y no después (el instrumento de la ola no miente). Y una cuarta, que es la frontera
// del cambio: comprime respuestas enteras, nunca el stream del panel.

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// notasDelCentral es cuántas notas siembra centralConNotas: menos de una página (50), así que la
// primera página las trae todas.
const notasDelCentral = 40

// pedidoDePull es el tools/call de musubi_sync_pull que manda el cliente del sync, con el cursor.
const pedidoDePull = `{"jsonrpc":"2.0","id":"pull","method":"tools/call","params":{"name":"musubi_sync_pull","arguments":{"after_rowid":%d,"limit":50}}}`

// centralConNotas arma un central REAL —su handler HTTP, el mismo que sirve producción— con
// notasDelCentral notas shared de prosa parecida a la del central. Devuelve el server y el handler,
// para poder montar encima el central de hoy (ver TestLaBajadaComprimidaTraeLoMismoYMideElCable).
func centralConNotas(t *testing.T) (*httptest.Server, http.Handler) {
	t.Helper()
	eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	for i := 0; i < notasDelCentral; i++ {
		contenido := fmt.Sprintf("Nota %d del central. El cursor de la bajada vuelve a cero cuando cambia el alcance, "+
			"y la página %d se pide otra vez desde el principio. Medido el día %d sobre %d filas, con %d páginas "+
			"vacías en el medio y un tick de %d s.", i, i%7, 10+i%20, 100+i*13, i%11, 30+i%4)
		if err := eng.SaveObservationTyped(fmt.Sprintf("nota-%03d", i), fmt.Sprintf("sync/tema-%d", i%5), contenido,
			1, "semantic", memory.ScopeShared, nil); err != nil {
			t.Fatal(err)
		}
	}
	h := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{}).HTTPHandler(httpOptions{reqTimeout: 30 * time.Second})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, h
}

// postComoViaja manda un POST a /mcp con el Accept-Encoding que se le pida —o ninguno, con ""— y
// devuelve la respuesta con el cuerpo TAL COMO VIAJÓ.
//
// El transporte va con DisableCompression, y sin eso la prueba no probaría nada: el de Go agrega
// `Accept-Encoding: gzip` por su cuenta cuando el pedido no lo trae, y después descomprime y borra
// el Content-Encoding sin avisar. Con el transporte por defecto, «sin la cabecera» mandaría la
// cabecera, y la respuesta se vería siempre en claro, comprimiera el central o no.
func postComoViaja(t *testing.T, url, cuerpo, acepta string) (*http.Response, []byte) {
	t.Helper()
	cli := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{DisableCompression: true}}
	defer cli.CloseIdleConnections()
	req, err := http.NewRequest(http.MethodPost, url+mcpHTTPPath, strings.NewReader(cuerpo))
	if err != nil {
		t.Fatalf("armar el pedido: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if acepta != "" {
		req.Header.Set("Accept-Encoding", acepta)
	}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("leer la respuesta: %v", err)
	}
	return resp, b
}

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("la respuesta dice gzip y no lo es: %v", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("descomprimir la respuesta: %v", err)
	}
	return out
}

// TestRespuestaGzipSoloSiSeAcepta: la página de la bajada sale comprimida SÓLO si el pedido nombra
// gzip, y descomprimida es la misma página, byte a byte. Sin la cabecera, con «gzip;q=0» o con un
// «*» a secas, sale en claro. Y una respuesta chica —la página vacía de un tick sin novedades— no se
// comprime aunque se pida.
//
// Lo que importa de verdad es la mitad negativa: el /mcp del central tiene más clientes que el sync
// (el canal de las sesiones, el panel, los bots del server), y uno que no pidió gzip y lo recibe no
// tiene forma de leerlo.
//
// Sabotaje: no comprimir nunca.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="if grande && aceptaGzip("
// arnes: a="if false && grande && aceptaGzip("
//
// Sabotaje: comprimirle también a quien no lo pidió.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="aceptaGzip(r.Header.Get(\"Accept-Encoding\")) {"
// arnes: a="aceptaGzip(r.Header.Get(\"Accept-Encoding\")) || true {"
//
// Sabotaje: leer «gzip;q=0» como un sí.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="; err != nil || q <= 0 {"
// arnes: a="; false && (err != nil || q <= 0) {"
//
// Sabotaje: comprimir también las respuestas chicas.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="grande := len(data) > umbralCompresionRespuesta"
// arnes: a="grande := len(data) >= 0"
func TestRespuestaGzipSoloSiSeAcepta(t *testing.T) {
	ts, _ := centralConNotas(t)
	pagina := fmt.Sprintf(pedidoDePull, 0)

	// Sin la cabecera: en claro. Es también la referencia de lo que dice la página.
	resp, claro := postComoViaja(t, ts.URL, pagina, "")
	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("SIN Accept-Encoding la respuesta vino con Content-Encoding %q: un cliente que no sabe descomprimir recibiría bytes ilegibles", ce)
	}
	var jr JsonRpcResponse
	if err := json.Unmarshal(claro, &jr); err != nil || jr.Error != nil {
		t.Fatalf("la página en claro no es JSON-RPC válido (err=%v, rpc=%+v)", err, jr.Error)
	}
	if len(claro) <= umbralCompresionRespuesta {
		t.Fatalf("precondición: la página pesa %d B, no pasa el umbral de %d; la prueba no probaría la compresión", len(claro), umbralCompresionRespuesta)
	}

	// Con la cabecera: comprimida, más chica, y descomprimida es LA MISMA página.
	resp, cable := postComoViaja(t, ts.URL, pagina, "gzip")
	if ce := resp.Header.Get("Content-Encoding"); ce != "gzip" {
		t.Fatalf("con Accept-Encoding: gzip y %d B de página, Content-Encoding = %q; esperaba gzip", len(claro), ce)
	}
	if !strings.Contains(resp.Header.Get("Vary"), "Accept-Encoding") {
		t.Errorf("la respuesta comprimida no declara Vary: Accept-Encoding (vino %q)", resp.Header.Get("Vary"))
	}
	if len(cable) >= len(claro) {
		t.Errorf("comprimida pesa %d B y en claro %d B: no se ahorró nada", len(cable), len(claro))
	}
	if got := gunzip(t, cable); !bytes.Equal(got, claro) {
		t.Errorf("descomprimida, la página no es la misma que en claro (%d B contra %d B)", len(got), len(claro))
	}
	t.Logf("página de %d notas: %d B en claro, %d B comprimida (%.1f %%)", notasDelCentral, len(claro), len(cable),
		100*float64(len(cable))/float64(len(claro)))

	// La forma de los navegadores y de las bibliotecas HTTP: varias codificaciones con pesos.
	if resp, _ := postComoViaja(t, ts.URL, pagina, "br;q=1.0, gzip;q=0.8, deflate"); resp.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("con gzip entre varias codificaciones, Content-Encoding = %q; esperaba gzip", resp.Header.Get("Content-Encoding"))
	}

	// «gzip;q=0» es un NO explícito, y un «*» a secas no nombra gzip: los dos, en claro.
	for _, acepta := range []string{"gzip;q=0, identity", "*"} {
		resp, b := postComoViaja(t, ts.URL, pagina, acepta)
		if ce := resp.Header.Get("Content-Encoding"); ce != "" || !bytes.Equal(b, claro) {
			t.Errorf("con Accept-Encoding %q la respuesta vino con Content-Encoding %q (%d B; en claro son %d): quien no pidió gzip lo recibió",
				acepta, ce, len(b), len(claro))
		}
	}

	// La página vacía de un tick sin novedades no pasa el umbral: en claro aunque se pida gzip.
	resp, chica := postComoViaja(t, ts.URL, fmt.Sprintf(pedidoDePull, int64(1)<<40), "gzip")
	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		t.Errorf("una respuesta de %d B salió con Content-Encoding %q: debajo del umbral de %d gzip no ahorra nada", len(chica), ce, umbralCompresionRespuesta)
	}
	if !bytes.Contains(chica, []byte(`\"items\":[]`)) {
		t.Errorf("precondición: la respuesta chica tenía que ser una página vacía, vino %q", chica)
	}
}

// clienteDelSyncContra arma el cliente del sync, el de los daemons, apuntado a url.
func clienteDelSyncContra(t *testing.T, url string) *SyncClient {
	t.Helper()
	c, err := NewSyncClient(config.SyncConfig{CentralURL: url, AllowInsecureToken: true, RequestTimeoutSeconds: 30})
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	return c
}

// TestLaBajadaComprimidaTraeLoMismoYMideElCable: SyncClient.Pull contra el central que comprime trae
// los MISMOS ítems que contra el central de hoy, que contesta en claro; y el tráfico se cuenta del
// lado correcto de la descompresión: comprimida, cable < crudos, y los crudos son exactamente los de
// la misma página en claro. Contra el central de hoy, cable = crudos. Y el drain lo anota así en
// sync_viajes, que es de donde sale la métrica (1) de la ola.
//
// La trampa que se custodia es del transporte de Go: si el pedido no nombra Accept-Encoding, lo
// agrega él y descomprime sin avisar, y el contador vería el cuerpo ya descomprimido como si fuera
// el cable. La página llegaría igual y todo se vería bien, salvo el número.
//
// Sabotaje: no pedir gzip a mano (lo pide el transporte, y descomprime sin avisar).
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="req.Header.Set(\"Accept-Encoding\", \"gzip\")"
// arnes: a="req.Header.Del(\"Accept-Encoding\")"
//
// Sabotaje: contar como cable lo ya descomprimido.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="c.trafico.bajadaCable.Add(enCable)"
// arnes: a="c.trafico.bajadaCable.Add(n)"
func TestLaBajadaComprimidaTraeLoMismoYMideElCable(t *testing.T) {
	ts, h := centralConNotas(t)
	// EL CENTRAL DE HOY es el mismo handler sin ver el Accept-Encoding: el binario desplegado escribe
	// siempre en claro, que es exactamente lo que hace writeHTTPJSON cuando el pedido no nombra gzip.
	hoy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("Accept-Encoding")
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(hoy.Close)

	nuevo, viejo := clienteDelSyncContra(t, ts.URL), clienteDelSyncContra(t, hoy.URL)
	items, next, alcance, err := nuevo.Pull(0, 50)
	if err != nil {
		t.Fatalf("Pull contra el central que comprime: %v", err)
	}
	itemsHoy, nextHoy, alcanceHoy, err := viejo.Pull(0, 50)
	if err != nil {
		t.Fatalf("Pull contra el central de hoy: %v", err)
	}
	if len(items) != notasDelCentral {
		t.Fatalf("precondición: la página tenía que traer las %d notas, trajo %d", notasDelCentral, len(items))
	}
	if !reflect.DeepEqual(items, itemsHoy) || next != nextHoy || alcance != alcanceHoy {
		t.Errorf("comprimida, la página trajo otra cosa que en claro: %d ítems (cursor %d, alcance %q) contra %d (cursor %d, alcance %q)",
			len(items), next, alcance, len(itemsHoy), nextHoy, alcanceHoy)
	}

	cable, crudos := nuevo.trafico.bajadaCable.Load(), nuevo.trafico.bajadaCrudos.Load()
	cableHoy, crudosHoy := viejo.trafico.bajadaCable.Load(), viejo.trafico.bajadaCrudos.Load()
	if cableHoy <= 0 || cableHoy != crudosHoy {
		t.Errorf("contra el central de hoy la página viaja en claro: cable y crudos tienen que dar lo mismo, dieron %d y %d", cableHoy, crudosHoy)
	}
	if cable <= 0 || cable >= crudos {
		t.Errorf("CABLE MAL MEDIDO: la página comprimida contó %d B de cable y %d B crudos; el cable tiene que ser lo que viajó, antes de descomprimir", cable, crudos)
	}
	if crudos != crudosHoy {
		t.Errorf("los crudos de la página comprimida (%d B) no son los de la misma página en claro (%d B)", crudos, crudosHoy)
	}
	// Y el cable es EXACTAMENTE lo que viajó: el cuerpo comprimido que sirve el central para el mismo
	// pedido, byte a byte (gzip es determinista para la misma entrada).
	if _, viajo := postComoViaja(t, ts.URL, fmt.Sprintf(pedidoDePull, 0), "gzip"); int64(len(viajo)) != cable {
		t.Errorf("el cable contó %d B y el central sirvió %d B comprimidos para la misma página", cable, len(viajo))
	}
	t.Logf("cliente: %d B en el cable, %d B crudos (%.1f %%)", cable, crudos, 100*float64(cable)/float64(max(crudos, 1)))

	// El drain lo anota así en sync_viajes.
	s, _ := serverQueAnotaViajes(t, ts.URL, true)
	s.drainInboundOnce(context.Background())
	r, err := s.engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b := r.BajadaHoy; b.Filas != notasDelCentral || b.BytesCable <= 0 || b.BytesCable >= b.BytesCrudos {
		t.Errorf("sync_viajes de la bajada = %+v; esperaba %d filas con el cable por debajo de los crudos", b, notasDelCentral)
	}
}

// TestElStreamDelPanelNoSeComprime: el feed en vivo (/api/stream, SSE) sale en claro aunque el
// cliente nombre gzip, y su primer frame se lee sin esperar nada más. Un stream comprimido no se
// ve hasta que el compresor suelta un bloque: con el tráfico de trabajo real (~23 eventos por hora)
// el panel «en vivo» se quedaría mudo. Por eso la compresión vive en writeHTTPJSON, que sólo
// escribe respuestas enteras de /mcp, y no en un middleware sobre el mux.
//
// Sabotaje: que el stream declare gzip.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="w.Header().Set(\"X-Accel-Buffering\", \"no\") // que ningún proxy intermedio lo bufferee"
// arnes: a="w.Header().Set(\"X-Accel-Buffering\", \"no\") // que ningún proxy intermedio lo bufferee\n\t\tw.Header().Set(\"Content-Encoding\", \"gzip\")"
func TestElStreamDelPanelNoSeComprime(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/stream", nil)
	// Nombrado a mano, igual que en el sync: así el transporte no descomprime ni borra la cabecera.
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	defer resp.Body.Close()
	if ce := resp.Header.Get("Content-Encoding"); ce != "" {
		t.Fatalf("el stream del panel salió con Content-Encoding %q: un stream comprimido no llega hasta que el compresor suelta un bloque", ce)
	}
	linea, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || linea != "event: backlog\n" {
		t.Fatalf("el primer frame del stream no se lee en claro: %q (err=%v)", linea, err)
	}
}

// cuerpoPropio es un texto de ~8 KiB que sólo puede ser de la llamada (g, i): si una respuesta se
// mezcla con otra, descomprimida ya no es la suya.
func cuerpoPropio(g, i int) string {
	var b strings.Builder
	for k := 0; b.Len() < 8<<10; k++ {
		fmt.Fprintf(&b, "respuesta %d de la goroutina %d, línea %d: %x. ", i, g, k, (g*7919+i*104729+k*31)%65521)
	}
	return b.String()
}

// TestLasRespuestasComprimidasNoSeMezclan: el central comprime respuestas CONCURRENTES —el sync de
// varias máquinas, las sesiones de `musubi cerebro` y el adjudicador le pegan a la vez— reciclando
// los compresores (escritoresGzip), y cada respuesta tiene que llevar SU cuerpo. El pool comparte el
// compresor y nunca el destino. Si un refactor comparte lo que no debe, en serie todo sigue andando
// y en producción dos principals reciben cuerpos cruzados: memoria de un tenant en la respuesta de
// otro. Medido en la revisión con un buffer compartido: 1.652 de 1.920 respuestas salieron con un
// cuerpo que no era el suyo, y las pruebas en serie seguían verdes.
//
// No necesita -race para morder (esta PC no tiene cgo): cada respuesta, descomprimida si viajó
// comprimida, se compara con su propio JSON. Un pánico adentro del compresor también cuenta como
// respuesta rota. Una que salió en claro porque comprimir falló NO está rota —es la salida que
// promete comprimirGzip— mientras lleve su cuerpo.
//
// Sabotaje: devolver el compresor al pool antes de usarlo.
// arnes: archivo="internal/mcp/gzip.go"
// arnes: de="defer escritoresGzip.Put(zw)"
// arnes: a="escritoresGzip.Put(zw)"
func TestLasRespuestasComprimidasNoSeMezclan(t *testing.T) {
	const goroutinas, llamadas = 32, 60
	var rotas, comprimidas atomic.Int64
	var primera sync.Once
	var ejemplo string
	responder := func(g, i int) {
		defer func() {
			if p := recover(); p != nil {
				rotas.Add(1)
				primera.Do(func() { ejemplo = fmt.Sprintf("la llamada %d-%d entró en pánico: %v", g, i, p) })
			}
		}()
		resp := JsonRpcResponse{JsonRpc: "2.0", ID: fmt.Sprintf("%d-%d", g, i), Result: textResult(cuerpoPropio(g, i))}
		propio, err := json.Marshal(resp)
		if err != nil {
			panic(err)
		}
		req := httptest.NewRequest(http.MethodPost, mcpHTTPPath, nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		writeHTTPJSON(rec, req, resp)
		llego := rec.Body.Bytes()
		if rec.Header().Get("Content-Encoding") == "gzip" {
			comprimidas.Add(1)
			zr, zerr := gzip.NewReader(bytes.NewReader(llego))
			if zerr == nil {
				llego, zerr = io.ReadAll(zr)
			}
			err = zerr
		}
		if err != nil || !bytes.Equal(llego, propio) {
			rotas.Add(1)
			primera.Do(func() {
				ejemplo = fmt.Sprintf("la llamada %d-%d leyó %d B (Content-Encoding %q, err=%v) y su JSON son %d B",
					g, i, len(llego), rec.Header().Get("Content-Encoding"), err, len(propio))
			})
		}
	}
	var wg sync.WaitGroup
	for g := 0; g < goroutinas; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < llamadas; i++ {
				responder(g, i)
			}
		}(g)
	}
	wg.Wait()
	total := goroutinas * llamadas
	if n := rotas.Load(); n > 0 {
		t.Errorf("%d de %d respuestas escritas en paralelo llegaron con un cuerpo ajeno o roto (%d salieron comprimidas); por ejemplo, %s",
			n, total, comprimidas.Load(), ejemplo)
	}
	if comprimidas.Load() == 0 {
		t.Fatalf("precondición: ninguna de las %d respuestas salió comprimida; sin compresión, la prueba no mira el pool", total)
	}
}

// centralQueContesta sirve siempre el mismo cuerpo en /mcp, con el Content-Encoding que se le diga
// ("" = ninguno): un central de mentira para ver qué hace el Pull con lo que el real no manda.
func centralQueContesta(t *testing.T, contentEncoding string, cuerpo []byte) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if contentEncoding != "" {
			w.Header().Set("Content-Encoding", contentEncoding)
		}
		_, _ = w.Write(cuerpo)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

// paginaConUnaNota arma la respuesta de musubi_sync_pull —la misma forma que la de toolSyncPull— con
// una sola nota de `peso` bytes de contenido y el cursor en 8.
func paginaConUnaNota(t *testing.T, peso int) []byte {
	t.Helper()
	contenido := strings.Repeat("la bajada se lee acotada. ", peso/26+1)[:peso]
	nota := memory.SharedObs{RowID: 8, ID: "nota-grande", TopicKey: "sync/tope", Content: contenido, Importance: 1, MemType: "semantic"}
	pl, err := json.Marshal(map[string]interface{}{"items": []memory.SharedObs{nota}, "next_cursor": 8, "alcance": ""})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(JsonRpcResponse{JsonRpc: "2.0", ID: "pull", Result: textResult(string(pl))})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestLaBajadaNoDescomprimeSinTope: el Pull lee la página ACOTADA. Una página comprimida que en claro
// pasa del tope se corta ahí y vuelve como un fallo permanente que nombra el tope, sin avanzar el
// cursor ni contar nada para sync_viajes; en claro, lo mismo. Y el tope de producción no corta una
// página real: la misma página de 1 MiB, con el tope de NewSyncClient, llega entera.
//
// Lo que se custodia es la memoria del daemon. Sin tope, ~64 KB de cable se expanden a 64 MiB y el
// decoder los junta enteros (448 MiB reservados, medido en la revisión); con 1 MB serían ~7 GiB. El
// tope de la prueba es de 64 KiB y la página de 1 MiB para no fabricar los 64 MiB de verdad: lo que
// importa es que el lector corta, no dónde.
//
// Sabotaje: descomprimir sin tope.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="return io.LimitReader(claro, tope+1), nil"
// arnes: a="return claro, nil"
func TestLaBajadaNoDescomprimeSinTope(t *testing.T) {
	const peso, tope = 1 << 20, 64 << 10
	pagina := paginaConUnaNota(t, peso)
	var comprimida bytes.Buffer
	zw := gzip.NewWriter(&comprimida)
	if _, err := zw.Write(pagina); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	bomba := centralQueContesta(t, "gzip", comprimida.Bytes())

	// El tope de producción deja pasar la página entera: es una página válida, y 1 MiB está lejos de 64.
	items, next, _, err := clienteDelSyncContra(t, bomba).Pull(7, 50)
	if err != nil || len(items) != 1 || len(items[0].Content) != peso || next != 8 {
		t.Fatalf("precondición: con el tope de producción la página de %d B tenía que llegar entera; err=%v, %d ítems, cursor %d", peso, err, len(items), next)
	}

	for _, caso := range []struct{ forma, url string }{
		{"comprimida", bomba},
		{"en claro", centralQueContesta(t, "", pagina)},
	} {
		c := clienteDelSyncContra(t, caso.url)
		c.topePagina = tope
		items, next, _, err := c.Pull(7, 50)
		if err == nil {
			t.Errorf("%s: una página de %d B en claro pasó con un tope de %d B (trajo %d ítems): se leyó sin tope", caso.forma, len(pagina), tope, len(items))
			continue
		}
		if !errors.Is(err, errPermanent) || !strings.Contains(err.Error(), bytesLegibles(tope)) {
			t.Errorf("%s: el error no es el del tope (permanente, nombrando %s): %v", caso.forma, bytesLegibles(tope), err)
		}
		if items != nil || next != 7 {
			t.Errorf("%s: una página cortada devolvió %d ítems y el cursor %d; esperaba nada y el 7 de antes", caso.forma, len(items), next)
		}
		if f := c.foto().bajada; f != (memory.Viaje{}) {
			t.Errorf("%s: una página cortada contó para sync_viajes: %+v", caso.forma, f)
		}
	}
	t.Logf("la página: %d B en claro, %d B comprimida", len(pagina), comprimida.Len())
}
