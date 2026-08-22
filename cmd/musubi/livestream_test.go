package main

// Invariantes del RELAY del riel en vivo (livestream.go). El relay existe para que el token del
// cerebro no llegue nunca al navegador, así que la mitad de estos tests son sobre eso.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// cerebroFalso levanta un /api/stream de mentira que emite lo que se le pase y registra cómo lo
// llamaron. Devuelve la URL base y un puntero a lo que vio del pedido.
//
// El handler corre en la goroutine del servidor y el test lee desde la suya, así que lo que vio
// va bajo mutex y sólo se lee por esperar(), que primero bloquea hasta que el pedido llegó de
// verdad. Sin las dos cosas el test es una carrera: ver el defecto que esto arregla en R1.
type pedidoVisto struct {
	mu    sync.Mutex
	llego chan struct{}
	una   sync.Once
	auth  string
	query string
	ruta  string
}

// esperar bloquea hasta que el relay haya llamado al cerebro falso, y recién ahí devuelve lo que
// se vio del pedido. Falla el test si no llega en el plazo, en vez de devolver campos vacíos que
// se leerían como "el relay mandó mal el token".
func (p *pedidoVisto) esperar(t *testing.T, plazo time.Duration) pedidoVisto {
	t.Helper()
	select {
	case <-p.llego:
	case <-time.After(plazo):
		t.Fatalf("el relay no llamó al cerebro en %v", plazo)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return pedidoVisto{auth: p.auth, query: p.query, ruta: p.ruta}
}

func cerebroFalso(t *testing.T, emitir func(w http.ResponseWriter, rc *http.ResponseController)) (string, *pedidoVisto) {
	t.Helper()
	visto := &pedidoVisto{llego: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		visto.mu.Lock()
		visto.auth = r.Header.Get("Authorization")
		visto.query = r.URL.RawQuery
		visto.ruta = r.URL.Path
		visto.mu.Unlock()
		visto.una.Do(func() { close(visto.llego) })
		rc := http.NewResponseController(w)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		emitir(w, rc)
		<-r.Context().Done()
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts.URL, visto
}

// leerFrames junta frames SSE del cuerpo hasta juntar n o agotar el plazo.
func leerFrames(t *testing.T, url string, n int, plazo time.Duration) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), plazo)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	var acum strings.Builder
	for {
		k, err := resp.Body.Read(buf)
		if k > 0 {
			acum.Write(buf[:k])
			if strings.Count(acum.String(), "\n\n") >= n {
				break
			}
		}
		if err != nil {
			break
		}
	}
	var out []string
	for _, f := range strings.Split(acum.String(), "\n\n") {
		if strings.TrimSpace(f) != "" {
			out = append(out, f)
		}
	}
	return out
}

// R1 · EL TOKEN VIAJA EN EL HEADER Y NUNCA EN LA URL.
//
// Es la razón de existir del relay. Un token en la query string queda escrito en los logs de
// acceso del servidor, en el historial del navegador y en cualquier proxy del camino — y con ese
// token se llama a TODO el cerebro, no sólo al feed. El test mira las dos cosas por separado:
// que el header esté, y que la query esté vacía.
func TestRelayMandaElTokenPorHeaderYNoPorURL(t *testing.T) {
	base, visto := cerebroFalso(t, func(w http.ResponseWriter, rc *http.ResponseController) {
		fmt.Fprint(w, "event: backlog\ndata: []\n\n")
		_ = rc.Flush()
	})
	r := nuevoRelay(base, "tok-secreto")
	if r == nil {
		t.Fatal("nuevoRelay devolvió nil con URL y token válidos")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.run(ctx)

	ts := httptest.NewServer(r.handlerStream())
	defer ts.Close()

	// OJO con lo que se espera. Un leerFrames(...,2,...) acá NO sirve: suscribir() sintetiza el
	// "enlace" y el "backlog" en el momento, sin haber hablado con nadie, así que los 2 frames
	// llegan al instante aunque el relay no haya conectado jamás (medido: 2 frames en 0 s con
	// run() sin arrancar). Hay que esperar el pedido del lado del cerebro, que es lo único que
	// prueba que el token viajó.
	p := visto.esperar(t, 5*time.Second)

	if p.auth != "Bearer tok-secreto" {
		t.Fatalf("Authorization = %q, esperaba el bearer", p.auth)
	}
	if strings.Contains(p.query, "tok-secreto") || strings.Contains(p.ruta, "tok-secreto") {
		t.Fatalf("el token se filtró a la URL (ruta=%q query=%q): quedaría en logs e historial", p.ruta, p.query)
	}
}

// R2 · lo que emite el cerebro llega al navegador tal cual.
func TestRelayReenviaLosEventos(t *testing.T) {
	base, _ := cerebroFalso(t, func(w http.ResponseWriter, rc *http.ResponseController) {
		fmt.Fprint(w, "event: backlog\ndata: []\n\n")
		fmt.Fprint(w, "event: uso\ndata: {\"tool\":\"musubi_recall\",\"kind\":\"trabajo\"}\n\n")
		_ = rc.Flush()
	})
	r := nuevoRelay(base, "tok")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.run(ctx)

	ts := httptest.NewServer(r.handlerStream())
	defer ts.Close()

	// El primer intento puede llegar antes de que el relay haya conectado; se reintenta hasta que
	// el evento aparezca o se agote el plazo. Sin esto el test sería intermitente por carrera.
	fin := time.Now().Add(6 * time.Second)
	for time.Now().Before(fin) {
		todo := strings.Join(leerFrames(t, ts.URL, 3, 2*time.Second), "\n")
		if strings.Contains(todo, "musubi_recall") {
			return
		}
	}
	t.Fatal("el evento del cerebro nunca llegó al navegador")
}

// R3 · el estado del enlace se dice, no se deja adivinar.
//
// Con ~23 eventos de trabajo por hora, "hace veinte minutos que no pasa nada" es un estado NORMAL
// y se ve EXACTAMENTE igual que un enlace cortado. Sin este evento el panel no puede distinguirlos.
func TestRelayAvisaCuandoElCerebroRechazaLaCredencial(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	r := nuevoRelay(ts.URL, "tok-malo")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.run(ctx)

	panel := httptest.NewServer(r.handlerStream())
	defer panel.Close()

	fin := time.Now().Add(6 * time.Second)
	for time.Now().Before(fin) {
		todo := strings.Join(leerFrames(t, panel.URL, 2, 2*time.Second), "\n")
		if strings.Contains(todo, `"estado":"caido"`) && strings.Contains(todo, "401") {
			return
		}
	}
	t.Fatal("el panel nunca se enteró de que la credencial fue rechazada")
}

// R4 · sin URL o sin token NO hay relay, y el panel dice qué falta.
//
// Devolver 404 sería peor: el front no podría distinguir "este panel no tiene feed" de "el panel
// es viejo" ni de "la ruta está mal", y mostraría un error técnico donde va una frase accionable.
func TestSinEnlaceElRielExplicaPorQue(t *testing.T) {
	if nuevoRelay("", "tok") != nil {
		t.Error("sin URL no debería haber relay")
	}
	if nuevoRelay("http://x", "") != nil {
		t.Error("sin token no debería haber relay")
	}
	if nuevoRelay("   ", "  ") != nil {
		t.Error("espacios en blanco no son una URL ni un token")
	}

	ts := httptest.NewServer(handlerStreamApagado("falta MUSUBI_CENTRAL_URL"))
	defer ts.Close()
	todo := strings.Join(leerFrames(t, ts.URL, 2, 4*time.Second), "\n")
	if !strings.Contains(todo, `"estado":"apagado"`) {
		t.Fatalf("el stream apagado no declara su estado: %s", todo)
	}
	if !strings.Contains(todo, "MUSUBI_CENTRAL_URL") {
		t.Fatalf("el stream apagado no dice qué falta: %s", todo)
	}
}

// R5 · motivoSinRelay nombra LO QUE FALTA, no "apagado" a secas: son dos causas con dos arreglos.
func TestMotivoSinRelayNombraLoQueFalta(t *testing.T) {
	t.Setenv("MUSUBI_TEST_TOKEN_RIEL", "")
	if m := motivoSinRelay("", "MUSUBI_TEST_TOKEN_RIEL"); !strings.Contains(m, "MUSUBI_CENTRAL_URL") || !strings.Contains(m, "MUSUBI_TEST_TOKEN_RIEL") {
		t.Fatalf("con las dos cosas ausentes el motivo tiene que nombrarlas: %q", m)
	}
	t.Setenv("MUSUBI_TEST_TOKEN_RIEL", "hay-token")
	m := motivoSinRelay("", "MUSUBI_TEST_TOKEN_RIEL")
	if !strings.Contains(m, "MUSUBI_CENTRAL_URL") || strings.Contains(m, "MUSUBI_TEST_TOKEN_RIEL") {
		t.Fatalf("con el token puesto sólo falta la URL: %q", m)
	}
}

// R6 · una pestaña que deja de leer no frena al relay ni a las demás.
func TestRelayNoSeFrenaConUnaPestanaMuerta(t *testing.T) {
	r := nuevoRelay("http://x", "tok")
	_, _, _ = r.suscribir() // se suscribe y nunca lee

	listo := make(chan struct{})
	go func() {
		for i := 0; i < relaySubBuf*4; i++ {
			r.publicar(frame{evento: "uso", data: []byte("{}")})
		}
		close(listo)
	}()
	select {
	case <-listo:
	case <-time.After(5 * time.Second):
		t.Fatal("el relay se bloqueó con una pestaña que no lee")
	}
}

// R7 · el relay tampoco acumula pestañas muertas. Mismo razonamiento que en el feed del cerebro:
// lo que se degrada en silencio es lo que no se arregla nunca.
func TestRelayNoAcumulaPestanasMuertas(t *testing.T) {
	r := nuevoRelay("http://x", "tok")
	ids := make([]int64, 0, 4)
	for i := 0; i < 4; i++ {
		id, _, _ := r.suscribir()
		ids = append(ids, id)
	}
	r.mu.Lock()
	vivas := len(r.subs)
	r.mu.Unlock()
	if vivas != 4 {
		t.Fatalf("suscriptores = %d, esperaba 4", vivas)
	}
	for _, id := range ids {
		r.desuscribir(id)
	}
	r.mu.Lock()
	quedan := len(r.subs)
	r.mu.Unlock()
	if quedan != 0 {
		t.Fatalf("quedaron %d pestañas muertas en el relay", quedan)
	}
	r.desuscribir(ids[0]) // idempotente: no puede entrar en pánico
}
