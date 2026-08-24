package main

// actores.go es el CENSO DE ACTORES visto desde el panel: quién llama al cerebro y cuánto.
//
// POR QUÉ EL PANEL LO PROXEA EN VEZ DE QUE EL NAVEGADOR PIDA DIRECTO AL CENTRAL. Tres razones,
// y son las mismas por las que el riel también viaja por acá (livestream.go):
//   1. el bearer. Un fetch desde la página necesitaría el token EN la página, o sea en el DOM,
//      en el historial y en cualquier extensión instalada;
//   2. CORS. El central no publica cabeceras para un origen `127.0.0.1:7777`;
//   3. el panel YA tiene la credencial y la conexión. Duplicarla del otro lado es una segunda
//      copia del secreto para no ganar nada.
//
// POR QUÉ NO HAY CAMINO LOCAL. La tentación es «si no hay central, censar la base local». No
// sirve, y no es una sospecha: medido el 2026-08-24 sobre la base de este repo, `tool_invocations`
// tiene 230.682 filas y las 230.682 con `principal` vacío — en stdio no hay credencial que
// atribuir. Un censo local devolvería la lista vacía SIEMPRE, que es indistinguible de «no hay
// actores». Por eso el estado se declara en vez de servir un vacío que parece un dato.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// censoRespuesta es lo que ve el navegador. `Estado` va SIEMPRE y es lo primero que mira el
// cliente: un censo vacío puede significar cuatro cosas distintas —no hay central, el central es
// viejo, el enlace está caído, o de verdad no llamó nadie— y las cuatro se dibujan igual si lo
// único que viaja es la lista.
type censoRespuesta struct {
	Estado  string          `json:"estado"`            // "vivo" | "apagado" | "viejo" | "caido" | "sin_permiso"
	Detalle string          `json:"detalle,omitempty"` // la frase accionable, cuando el estado no es "vivo"
	Destino string          `json:"destino,omitempty"` // a qué cerebro se le preguntó
	Censo   json.RawMessage `json:"censo,omitempty"`   // el cuerpo tal cual lo devolvió el central
}

// cacheCenso guarda la última respuesta buena. El censo hace COUNT(DISTINCT) sobre una tabla que
// crece ~100.000 filas por día: no es una consulta para atender a cada pestaña que abre. 60 s es
// el orden correcto porque el dato que sirve —el volumen histórico de cada actor— cambia en
// horas, no en segundos; lo que cambia rápido es el riel, y ése ya viaja por su propio canal.
//
// Es la lección de /api/pulse, que corría un Diagnose() entero cada 5 s y terminaba tardando 50.
type cacheCenso struct {
	mu     sync.Mutex
	cuerpo []byte
	dias   int
	hasta  time.Time
}

const ttlCenso = 60 * time.Second

// handlerActores sirve /api/actores del panel. Nunca devuelve 5xx por culpa del central: un
// cerebro apagado o viejo es un ESTADO del sistema que el panel tiene que poder dibujar, no un
// error de la petición del navegador.
func handlerActores(relay *relayVivo, cache *cacheCenso) http.HandlerFunc {
	cli := &http.Client{Timeout: 20 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		responder := func(c censoRespuesta) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(c)
		}

		if relay == nil || relay.base == "" || relay.token == "" {
			responder(censoRespuesta{
				Estado:  "apagado",
				Detalle: "sin enlace al cerebro central: el censo de actores sólo existe ahí, porque en esta máquina las llamadas no llevan credencial",
			})
			return
		}

		dias := 7
		if s := r.URL.Query().Get("days"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 90 {
				dias = n
			}
		}

		cache.mu.Lock()
		if cache.cuerpo != nil && cache.dias == dias && time.Now().Before(cache.hasta) {
			cuerpo := cache.cuerpo
			cache.mu.Unlock()
			responder(censoRespuesta{Estado: "vivo", Destino: relay.host(), Censo: cuerpo})
			return
		}
		cache.mu.Unlock()

		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
			relay.base+"/api/actores?days="+strconv.Itoa(dias), nil)
		if err != nil {
			responder(censoRespuesta{Estado: "caido", Destino: relay.host(), Detalle: err.Error()})
			return
		}
		req.Header.Set("Authorization", "Bearer "+relay.token)

		resp, err := cli.Do(req)
		if err != nil {
			responder(censoRespuesta{Estado: "caido", Destino: relay.host(), Detalle: err.Error()})
			return
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			// Tope de lectura: el cuerpo viene de otra máquina. Sin límite, un central que
			// responda mal puede hacer crecer este proceso sin techo. 4 MB son ~20.000 actores.
			cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			if err != nil {
				responder(censoRespuesta{Estado: "caido", Destino: relay.host(), Detalle: err.Error()})
				return
			}
			cache.mu.Lock()
			cache.cuerpo, cache.dias, cache.hasta = cuerpo, dias, time.Now().Add(ttlCenso)
			cache.mu.Unlock()
			responder(censoRespuesta{Estado: "vivo", Destino: relay.host(), Censo: cuerpo})
		case http.StatusNotFound:
			// Un 404 acá NO es «no hay actores»: es un cerebro anterior al censo. Confundirlos
			// haría que el panel dibuje un sistema desierto sobre un central lleno de trabajo.
			responder(censoRespuesta{
				Estado:  "viejo",
				Destino: relay.host(),
				Detalle: "el cerebro no tiene /api/actores (404): está corriendo una versión anterior al censo",
			})
		case http.StatusUnauthorized, http.StatusForbidden:
			responder(censoRespuesta{
				Estado:  "sin_permiso",
				Destino: relay.host(),
				Detalle: fmt.Sprintf("el cerebro rechazó la credencial (%d): revisá $MUSUBI_TOKEN", resp.StatusCode),
			})
		default:
			// EL CUERPO AJENO NO LLEGA A LA PÁGINA. Viaja el código y nada más.
			//
			// La primera versión de esto reenviaba los primeros 512 bytes del error del central
			// «para que se viera el detalle», y un test lo cazó devolviendo el bearer completo
			// dentro del mensaje. Recortar o filtrar el cuerpo sería defensa en profundidad
			// tapando el invariante: lo que no puede pasar es que texto de otra máquina se
			// renderice acá, y la forma de garantizarlo es no leerlo.
			responder(censoRespuesta{
				Estado:  "caido",
				Destino: relay.host(),
				Detalle: fmt.Sprintf("el cerebro respondió %d al pedir el censo de actores", resp.StatusCode),
			})
		}
	}
}
