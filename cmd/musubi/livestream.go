package main

// livestream.go conecta el panel local con el FEED EN VIVO del cerebro central.
//
// POR QUÉ HAY UN RELAY EN EL MEDIO Y NO SE CONECTA EL NAVEGADOR DIRECTO. Tres razones, y cada una
// alcanzaría sola:
//
//   1. EL TOKEN. Conectar el navegador al central exige que el bearer llegue al navegador. Ahí
//      queda —en el JS, en el historial si va por query string, en cualquier extensión que lea la
//      página— y con ese token se puede llamar a todo el cerebro, no sólo al feed. Con el relay,
//      el token no sale nunca del proceso del panel.
//   2. EL ORIGEN. La página se sirve desde loopback y el central vive en el tailnet: sin relay
//      hace falta CORS en el cerebro, o sea abrirle una puerta al navegador de cualquiera.
//   3. LO LOCAL Y LO CENTRAL EN LA MISMA VISTA. El panel ya muestra la memoria local; el relay
//      deja que el mismo riel muestre las dos procedencias sin que el front hable con dos hosts.
//
// LO QUE EL RELAY NO HACE: no interpreta los eventos. Reenvía los frames tal como llegan. Si algún
// día el cerebro agrega un campo, aparece en el panel sin tocar este archivo — y, más importante,
// no hay acá una segunda copia de la clasificación que se pueda desincronizar de la del cerebro.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"musubi/internal/logx"
)

// relayRing es cuánto guarda el relay para el navegador que abre la página después. El cerebro
// manda SU backlog al conectarse el relay, no al conectarse cada pestaña, así que sin este ring
// una pestaña abierta cinco minutos tarde arrancaría vacía.
const relayRing = 200

// relaySubBuf es el buffer por pestaña. Mismo criterio que en el cerebro: una pestaña lenta se
// queda sin eventos antes de frenar a las demás.
const relaySubBuf = 256

// Backoff de reconexión. Arranca corto (una caída suele ser un reinicio del cerebro, que vuelve en
// segundos) y crece hasta un techo para no martillar un tailnet caído.
const (
	relayBackoffMin = 1 * time.Second
	relayBackoffMax = 30 * time.Second
)

// frame es un evento SSE ya serializado: nombre y payload crudo. El relay no lo abre.
type frame struct {
	evento string
	data   []byte
}

// estadoEnlace es lo que el panel necesita saber del vínculo con el cerebro.
//
// EXISTE PORQUE UN FEED CAÍDO SE VE IGUAL QUE UN FEED TRANQUILO. Con ~23 eventos de trabajo por
// hora, "hace veinte minutos que no pasa nada" es un estado NORMAL — y también es exactamente lo
// que se ve cuando el enlace se cortó. Sin este evento, el panel no puede distinguirlos, y la
// primera vez que el cerebro se reinicie alguien va a mirar una pantalla quieta creyendo que está
// mirando la verdad.
type estadoEnlace struct {
	Estado  string `json:"estado"`            // "conectado" | "conectando" | "caido"
	Destino string `json:"destino,omitempty"` // host del cerebro, para poder decir a dónde
	Detalle string `json:"detalle,omitempty"` // el error, cuando lo hay
	Desde   string `json:"desde,omitempty"`   // cuándo se estableció el estado actual
}

// relayVivo mantiene UNA conexión al cerebro y la reparte entre las pestañas abiertas.
type relayVivo struct {
	url    string // ya resuelto a .../api/stream
	base   string // la raíz del cerebro, sin ruta: la necesita el censo de actores (actores.go)
	token  string
	client *http.Client

	mu     sync.Mutex
	subs   map[int64]chan frame
	next   int64
	ring   []frame
	desde  int
	llenó  bool
	enlace estadoEnlace
}

// nuevoRelay arma EL RIEL. Siempre devuelve uno: desde que existe el spool local (livelocal.go)
// hay eventos que mostrar aunque no haya central, y devolver nil dejaría el riel local sin por
// dónde salir.
//
// LO QUE NO CAMBIA: sin URL o sin token NO se intenta conectar al central. Un relay que reintenta
// para siempre contra nada sería un motor prendido calentando el aire — por eso `run` sale de
// inmediato cuando no hay a dónde ir, y el estado del enlace lo dice con una frase accionable en
// vez de dejarlo adivinar. Ver R4.
func nuevoRelay(base, token string) *relayVivo {
	base = strings.TrimSpace(base)
	token = strings.TrimSpace(token)
	sinCentral := base == "" || token == ""
	if sinCentral {
		return &relayVivo{
			subs:   make(map[int64]chan frame),
			ring:   make([]frame, relayRing),
			enlace: estadoEnlace{Estado: "apagado", Detalle: "sin enlace al cerebro central: sólo se ve lo de esta máquina"},
		}
	}
	return &relayVivo{
		url:   strings.TrimRight(base, "/") + "/api/stream",
		base:  strings.TrimRight(base, "/"),
		token: token,
		// Sin Timeout global: este cliente abre una conexión que POR DISEÑO no termina nunca, y
		// un http.Client.Timeout cubre el cuerpo entero, así que la cortaría a mano. Los timeouts
		// que sí corresponden —establecer la conexión y recibir las cabeceras— van en el
		// Transport, que es donde acotan lo que hay que acotar.
		client: &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				ResponseHeaderTimeout: 15 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		subs:   make(map[int64]chan frame),
		ring:   make([]frame, relayRing),
		enlace: estadoEnlace{Estado: "conectando"},
	}
}

// host devuelve el destino en forma corta, para mostrarlo sin exponer la ruta completa.
func (r *relayVivo) host() string {
	s := strings.TrimPrefix(strings.TrimPrefix(r.url, "http://"), "https://")
	if i := strings.Index(s, "/"); i > 0 {
		return s[:i]
	}
	return s
}

// run mantiene la conexión hasta que ctx se cancela. Bloquea: se lanza en una goroutine.
func (r *relayVivo) run(ctx context.Context) {
	if r.url == "" {
		return // no hay central configurado: no se reintenta contra nada
	}
	espera := relayBackoffMin
	for ctx.Err() == nil {
		r.marcar(estadoEnlace{Estado: "conectando", Destino: r.host()})
		err := r.conectar(ctx)
		if ctx.Err() != nil {
			return
		}
		detalle := "el cerebro cerró el stream"
		if err != nil {
			detalle = err.Error()
		}
		r.marcar(estadoEnlace{Estado: "caido", Destino: r.host(), Detalle: detalle})
		logx.Warn("panel en vivo: enlace con el cerebro caído; reintentando", "destino", r.host(), "espera", espera, "detalle", detalle)

		select {
		case <-ctx.Done():
			return
		case <-time.After(espera):
		}
		if espera *= 2; espera > relayBackoffMax {
			espera = relayBackoffMax
		}
	}
}

// conectar abre el stream y bombea frames hasta que se corta. Devuelve el motivo del corte.
func (r *relayVivo) conectar(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("el cerebro rechazó la credencial (401): revisá el token")
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("el cerebro no tiene /api/stream (404): está corriendo una versión anterior al feed en vivo")
		}
		return fmt.Errorf("el cerebro respondió %d", resp.StatusCode)
	}
	r.marcar(estadoEnlace{Estado: "conectado", Destino: r.host()})

	// El frame de backlog lleva hasta 200 eventos en una sola línea `data:`. Con el buffer default
	// del scanner (64 KB) esa línea se cortaría y el stream moriría en el primer frame, que es
	// justo el que hace que el panel no arranque en blanco.
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)

	var evento string
	var data []byte
	for sc.Scan() {
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, ":"): // latido del cerebro: prueba de vida, no se reenvía
		case strings.HasPrefix(l, "event: "):
			evento = strings.TrimPrefix(l, "event: ")
		case strings.HasPrefix(l, "data: "):
			data = []byte(strings.TrimPrefix(l, "data: "))
		case l == "":
			if evento != "" {
				r.publicar(frame{evento: evento, data: data})
				evento, data = "", nil
			}
		}
	}
	return sc.Err()
}

// marcar fija el estado del enlace y se lo cuenta a las pestañas.
func (r *relayVivo) marcar(e estadoEnlace) {
	e.Desde = time.Now().Format(time.RFC3339)
	r.mu.Lock()
	r.enlace = e
	r.mu.Unlock()
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	// El estado NO va al ring: el ring es historia de trabajo, y una pestaña que abre tiene que
	// recibir el estado ACTUAL (se lo manda suscribirse), no la crónica de las caídas de anoche.
	r.repartir(frame{evento: "enlace", data: b}, false)
}

// publicar mete un frame en el ring y lo reparte.
func (r *relayVivo) publicar(f frame) { r.repartir(f, true) }

func (r *relayVivo) repartir(f frame, alRing bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if alRing && f.evento == "uso" { // sólo las invocaciones son historia; el backlog ya lo es
		r.ring[r.desde] = f
		r.desde = (r.desde + 1) % len(r.ring)
		if r.desde == 0 {
			r.llenó = true
		}
	}
	for _, ch := range r.subs {
		select {
		case ch <- f:
		default: // pestaña que no lee: se le descarta, nunca se frena al relay
		}
	}
}

// suscribir registra una pestaña y le devuelve su estado inicial (enlace + historia) y su canal.
func (r *relayVivo) suscribir() (int64, <-chan frame, []frame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.next
	r.next++
	ch := make(chan frame, relaySubBuf)
	r.subs[id] = ch

	inicial := make([]frame, 0, relayRing+1)
	if b, err := json.Marshal(r.enlace); err == nil {
		inicial = append(inicial, frame{evento: "enlace", data: b})
	}
	n := len(r.ring)
	ini, cant := 0, r.desde
	if r.llenó {
		ini, cant = r.desde, n
	}
	previos := make([]json.RawMessage, 0, cant)
	for i := 0; i < cant; i++ {
		previos = append(previos, json.RawMessage(r.ring[(ini+i)%n].data))
	}
	if b, err := json.Marshal(previos); err == nil {
		inicial = append(inicial, frame{evento: "backlog", data: b})
	}
	return id, ch, inicial
}

func (r *relayVivo) desuscribir(id int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.subs[id]; ok {
		delete(r.subs, id)
		close(ch)
	}
}

// handlerStream es el /api/stream del panel LOCAL. Sin auth: la frontera es loopback, igual que
// para el resto del panel (que ya sirve la memoria entera sin credencial).
func (r *relayVivo) handlerStream() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rc := http.NewResponseController(w)
		_ = rc.SetWriteDeadline(time.Time{})
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		id, ch, inicial := r.suscribir()
		defer r.desuscribir(id)

		escribir := func(f frame) bool {
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", f.evento, f.data); err != nil {
				return false
			}
			return rc.Flush() == nil
		}
		for _, f := range inicial {
			if !escribir(f) {
				return
			}
		}
		hb := time.NewTicker(20 * time.Second)
		defer hb.Stop()
		for {
			select {
			case <-req.Context().Done():
				return
			case f, ok := <-ch:
				if !ok || !escribir(f) {
					return
				}
			case <-hb.C:
				if _, err := fmt.Fprint(w, ": latido\n\n"); err != nil {
					return
				}
				if rc.Flush() != nil {
					return
				}
			}
		}
	}
}

// explicarSinCentral fija el motivo concreto de que no haya central, para que el panel lo muestre.
//
// POR QUE EXISTE (era handlerStreamApagado, R4). Antes, sin central no habia riel y una ruta
// aparte servia la explicacion. Ahora el riel existe siempre —porque lo LOCAL hay que verlo aunque
// el central falte—, asi que la explicacion viaja por el mismo canal, en el evento `enlace`.
// Lo que NO cambia es el invariante: el panel dice que falta, con una frase accionable, en vez de
// dejarlo adivinar. Un 404 o un "apagado" a secas obligan a mirar la consola del server para saber
// si falta la URL, falta el token, o el panel es viejo.
func (r *relayVivo) explicarSinCentral(motivo string) {
	if r == nil || r.url != "" {
		return // hay central: el estado lo maneja run()
	}
	r.marcar(estadoEnlace{Estado: "apagado", Detalle: motivo})
}
