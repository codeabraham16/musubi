package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"musubi/internal/logx"
	"musubi/internal/motor"
)

// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PUERTA DEL MOTOR, Y POR QUÉ COMPARTE LA RUTA CON EL FEED EN VIVO.
//
// El cliente tiene la ruta HARDCODEADA: `fetch(`${proxyUrl}/api/stream`)`, en
// pi-agent-core/src/proxy.ts:147. Lo único configurable es `proxyUrl`, o sea el HOST. Así que
// el servidor NO puede elegir otro camino, y en Musubi `/api/stream` ya estaba tomada desde
// antes por el feed de eventos en vivo del panel.
//
// La salida es despachar por MÉTODO sobre la misma ruta: GET sigue siendo el feed, POST es un
// turno. No es un parche: son dos operaciones distintas sobre el mismo recurso —leer el flujo
// de lo que pasa, o pedir que pase algo— que es exactamente para lo que existen los métodos.
// Lo que sí hay que cuidar es que el `Allow` diga la verdad en cada estado, porque un 405 sin
// `Allow` deja al cliente sin saber si se equivocó de método o de servidor.
// ────────────────────────────────────────────────────────────────────────────────────────────

// topeDelPedidoDeTurno acota el cuerpo de un turno.
//
// Es generoso porque lo que viaja es un TRANSCRIPT COMPLETO: en la segunda vuelta de un agent
// loop con imágenes pegadas, el cuerpo lleva la conversación entera más cada captura en
// base64. Un tope apretado acá no se ve como un límite sino como un agente que deja de
// funcionar cuando la charla se pone larga. Y uno infinito deja que un cliente roto se coma
// la memoria del proceso, que es la razón por la que el tope existe.
const topeDelPedidoDeTurno = 32 << 20

// latidoDelTurno es cada cuánto se manda un comentario SSE mientras el turno piensa.
//
// Viaja como comentario (`: latido`) y no como un evento, y eso es lo que lo hace seguro: el
// parser del cliente descarta toda línea que no empiece con "data: " (proxy.ts:177), así que
// un latido no puede confundirse con un evento del protocolo ni cuando el cliente cambie.
const latidoDelTurno = 20 * time.Second

// plazoDeEscrituraDelTurno acota cada escritura al cable, una por una.
//
// 🔴 NO CONTRADICE AL `SetWriteDeadline(time.Time{})` DE `servirTurno`, aunque lo parezca: son
// dos relojes distintos, y confundirlos es lo que dejó este agujero abierto. El del server es
// ABSOLUTO —se arma una vez, al aceptar el request, y vence aunque el stream esté saliendo
// perfecto—, y por eso un turno con thinking largo moría a los 90 s: ése hay que limpiarlo. Éste
// es RODANTE: se renueva antes de cada escritura, así que no le pone techo al turno y sí se lo
// pone a una escritura que no avanza.
//
// SIN ÉL, UN CLIENTE QUE ABRE EL POST Y NO LEE RETIENE EL TURNO ENTERO. Con los buffers TCP
// llenos el Write bloquea sin plazo; la goroutine del bombeo deja de recibir del canal, que va
// sin buffer; la del motor se queda dentro de `emitir`; y `Turno` no llega ni a sus defers, así
// que el directorio del espejo, su listener y su token siguen vivos. Medido con un plazo de
// turno de 3 s: a los 6,5 s quedaban `{watchCtx=1, Turno=2, Serve=1}` goroutines y un
// `musubi-mcp-*` en disco, y lo único que los liberaba era el cancel del ctx — que un cliente
// que no lee pero tampoco cierra nunca dispara.
//
// La escala es la del `ReadTimeout` del server ([http.go:968](http.go#L968)): es lo que esta
// casa ya declara como «conexión colgada». Un chunk de SSE son bytes, no megas; que no entre al
// socket en todo ese rato significa que del otro lado no hay nadie leyendo.
const plazoDeEscrituraDelTurno = 30 * time.Second

// servirTurno atiende el `POST /api/stream`: traduce el pedido del cliente, corre el motor y
// bombea los eventos del protocolo por SSE.
func (s *McpServer) servirTurno(opt httpOptions, w http.ResponseWriter, r *http.Request) {
	if s.motor == nil {
		// EL MISMO 405 QUE ANTES DE QUE ESTO EXISTIERA, y el `Allow` dice sólo GET porque es
		// la verdad: sin motor configurado, POST no está disponible. Un 503 sería tentador,
		// pero diría «está pero caído» cuando en realidad nadie lo pidió.
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// AUTH IGUAL QUE EL GET DE ESTA MISMA RUTA. El bloque va copiado del feed a propósito:
	// servir un turno gasta la suscripción o los tokens del usuario, así que es la operación
	// MÁS cara del servidor. Una puerta más floja acá que en el feed de lectura sería al revés
	// de como tiene que ser.
	if opt.registry != nil {
		if _, ok := autenticarPersona(opt, w, r); !ok {
			return
		}
	} else if opt.token != "" && !validBearer(r.Header.Get("Authorization"), opt.token) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	pedido, err := leerPedidoDeTurno(w, r)
	if err != nil {
		// 🔴 EL ERROR ANTES DE LAS CABECERAS VA COMO JSON CON LA CLAVE `error`, porque es lo
		// que el cliente parsea: `const body = await response.json(); throw new Error(
		// body.error ?? ...)` en proxy.ts:163. Un `http.Error` de texto pelado le deja un
		// mensaje genérico y el motivo real se pierde justo cuando era el dato útil.
		errorDeTurnoEnJSON(w, http.StatusBadRequest, err)
		return
	}

	rc := http.NewResponseController(w)
	// 🔴 SIN ESTO EL TURNO MUERE A LOS 90 s. El WriteTimeout del server está calibrado para
	// las respuestas cortas de /mcp (timeout + 30 s), y un turno con thinking largo lo pasa
	// sin problema. El síntoma no sería un error claro: el cliente ve el stream cortado a
	// mitad de una frase. Misma maniobra que el feed en vivo, por el mismo motivo.
	//
	// ⚠️ LIMPIAR EL ABSOLUTO NO ES QUEDARSE SIN PLAZO, y la diferencia costó un agujero: el
	// bombeo arma uno RODANTE antes de cada escritura (`plazoDeEscrituraDelTurno`). Lo que se
	// borra acá es el techo del turno entero; lo que se pone allá es el techo de UNA escritura.
	// Sin el segundo, un cliente que abre el POST y no lee retiene el turno para siempre.
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // que ningún proxy intermedio lo bufferee
	w.WriteHeader(http.StatusOK)
	// 🔴 LAS CABECERAS SE BAJAN ACÁ, ANTES DEL PRIMER EVENTO, y cuesta una línea pero se paga
	// sola. `WriteHeader` sólo las pone en el buffer del server: sin este Flush no salen al
	// cable hasta que algo más escriba. Y del otro lado el cliente hace
	// `await fetch(...)` y recién entonces mira `response.ok` (proxy.ts:154), así que se
	// queda bloqueado todo lo que tarde el primer token — que con thinking largo son
	// decenas de segundos en los que no puede distinguir «pensando» de «no conectó».
	//
	// Medido: sin esta línea, un turno cuyo motor tarda en emitir deja al cliente esperando
	// hasta el primer LATIDO (20 s), que era lo único que destrababa el buffer.
	_ = rc.Flush()

	bombearTurno(r.Context(), s.motor, pedido, w, rc, latidoDelTurno)
}

// leerPedidoDeTurno decodifica el cuerpo y lo traduce al Pedido del motor.
func leerPedidoDeTurno(w http.ResponseWriter, r *http.Request) (motor.Pedido, error) {
	cuerpo := http.MaxBytesReader(w, r.Body, topeDelPedidoDeTurno)
	var crudo motor.PedidoProxy
	dec := json.NewDecoder(cuerpo)
	// ⚠️ ACÁ NO VA `DisallowUnknownFields`, Y ES AL REVÉS DE LO QUE HACE EL RESTO DEL REPO.
	// `/api/flota` lo usa porque Musubi escribe los dos lados de ese mensaje, así que un campo
	// desconocido ahí es un error de programación y conviene que grite. Este cuerpo lo escribe
	// un cliente AJENO que evoluciona solo: manda `temperature`, `samplingParams`,
	// `cacheRetention`, `transport`, `metadata` y el objeto `model` completo con tarifas y
	// modalidades. Rechazar lo que no entendemos haría fallar TODOS los pedidos reales, y
	// volvería a fallar con cada versión nueva del cliente. Se leen los campos que este
	// servidor puede honrar y el resto se ignora, que es lo único compatible hacia adelante.
	if err := dec.Decode(&crudo); err != nil {
		return motor.Pedido{}, fmt.Errorf("el cuerpo del pedido no se pudo leer: %w", err)
	}
	return motor.PedidoDesdeProxy(crudo)
}

// errorDeTurnoEnJSON contesta un error con la forma que el cliente sabe leer.
func errorDeTurnoEnJSON(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// El error de serialización se ignora: ya estamos en el camino de error y el status ya
	// salió, así que no hay nada mejor que hacer con él que no sea empeorar la respuesta.
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// bombearTurno corre el motor y escribe sus eventos como SSE.
//
// 🔴 EL MOTOR CORRE EN SU PROPIA GOROUTINE Y LOS EVENTOS PASAN POR UN CANAL, y no es un
// adorno de concurrencia: un `http.ResponseWriter` NO es seguro para uso concurrente. El
// latido lo manda esta goroutine, que es la que espera en el select; si el motor escribiera
// directo desde la suya, las dos tocarían el mismo writer y sería una carrera de datos — del
// tipo que no se cae en los tests y corrompe el stream en producción.
//
// El canal va SIN BUFFER a propósito: `emitir` no vuelve hasta que esta goroutine recibió el
// evento, así que cuando `Turno` retorna ya se escribieron todos. Eso es lo que hace que el
// `done` del protocolo sea realmente el último byte del stream.
//
// `latido` entra por parámetro en vez de leer la constante para que un test pueda medir el
// camino del latido sin esperar veinte segundos. Una variable de paquete que el test baje
// serviría igual hasta el día que dos tests corran en paralelo y se pisen el valor.
func bombearTurno(ctx context.Context, m motor.Motor, p motor.Pedido, w http.ResponseWriter, rc *http.ResponseController, latido time.Duration) {
	eventos := make(chan motor.Evento)
	// Buffer 1 para que la goroutine del motor pueda terminar aunque esta ya se haya ido:
	// sin él quedaría bloqueada escribiendo el resultado y se filtraría una goroutine por
	// cada cliente que se desconecta a mitad de un turno.
	hecho := make(chan error, 1)

	go func() {
		// 🔴 UN PANIC DEL MOTOR SE LLEVARÍA EL PROCESO ENTERO, no el turno. Esta goroutine NO
		// es la del handler, así que el `recover` de `net/http` no la cubre: medido con dos
		// brazos en la misma corrida, `net/http.(*conn).serve.func1` aparece en la pila de un
		// panic del handler y está AUSENTE en la de esta. Sin este defer, un defecto en
		// cualquier turno mata al `musubi serve` completo, con los demás turnos en vuelo
		// adentro y sus hijos `claude` quedando huérfanos, gastando cuota que ya nadie lee.
		//
		// El panic se degrada al error de UN turno, que el select de abajo ya sabe nombrar por
		// el camino de `eventoDeCierreAnomalo`. Y el send de acá NO duplica al de abajo:
		// `hecho <- m.Turno(...)` evalúa la llamada ANTES de mandar, así que si el motor
		// paniqueó el send de abajo nunca ocurrió. Un solo send por camino, que es justo lo
		// que el buffer de 1 aguanta.
		//
		// El stack va al log porque antes de este defer el panic imprimía su traza al morir, y
		// un arreglo que cambie «crash con traza» por «log sin traza» deja el defecto más
		// difícil de encontrar que estaba. Es lo mismo que hace el recuperador de `net/http`.
		//
		// Al cliente va un mensaje GENÉRICO, y el valor del panic se queda en el log — igual
		// que el recover de `Dispatch` (server.go:712). Un valor de panic arrastra lo que
		// tenga a mano: una ruta del disco, un fragmento del prompt, el pedazo de un token. El
		// detalle sirve para arreglarlo y el lugar para eso es el log del server, no un evento
		// que termina pintado en la UI y guardado en el transcript del otro lado.
		defer func() {
			if r := recover(); r != nil {
				logx.Error("panic en el motor del turno", "panic", r, "stack", string(debug.Stack()))
				hecho <- errors.New("defecto interno del motor")
			}
		}()

		hecho <- m.Turno(ctx, p, func(ev motor.Evento) error {
			select {
			case eventos <- ev:
				return nil
			case <-ctx.Done():
				// Devolver el error del ctx es lo que CORTA el turno desde adentro: el motor
				// lo propaga, mata el proceso hijo o cierra el cuerpo del request, y no
				// queda nadie generando tokens que ya nadie va a leer.
				return ctx.Err()
			}
		})
	}()

	// El plazo va ANTES de cada escritura y no una vez al principio: es rodante a propósito
	// (ver `plazoDeEscrituraDelTurno`). Vive en un ayudante porque hay DOS sitios que escriben
	// al cable —los eventos y el latido— y dos copias de una línea así derivan sin que nadie
	// lo note; el que se olvide deja ese camino sin plazo, que es el agujero entero.
	plazoDeEscritura := func() {
		_ = rc.SetWriteDeadline(time.Now().Add(plazoDeEscrituraDelTurno))
	}

	escribir := func(ev motor.Evento) bool {
		b, err := json.Marshal(ev)
		if err != nil {
			// Un evento que no serializa es un defecto de este paquete, no del turno. Se
			// saltea en vez de cortar el stream: perder un delta es menos malo que perder
			// la respuesta entera.
			return true
		}
		plazoDeEscritura()
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	hb := time.NewTicker(latido)
	defer hb.Stop()

	for {
		select {
		case ev := <-eventos:
			if !escribir(ev) {
				// El cliente se fue. No hay a quién avisarle, así que sólo se vuelve: el
				// ctx del request ya está cancelado y el motor lo va a ver en su `emitir`.
				return
			}

		case err := <-hecho:
			if err == nil {
				// El turno cerró bien y el traductor ya emitió su `done`. No hay nada que
				// agregar, y agregar algo acá sería un segundo evento terminal.
				return
			}
			// 🔴 EL CIERRE ANÓMALO SE NOMBRA, porque el cliente NO TIENE OTRA FORMA DE
			// SABERLO. El status 200 ya salió con las cabeceras, así que no se puede cambiar;
			// y un stream que se corta sin un evento terminal lo deja con el fallback
			// genérico de desconexión, que no dice nada de lo que pasó. Este evento es la
			// única traza del motivo real que llega al otro lado.
			escribir(eventoDeCierreAnomalo(ctx, err))
			return

		case <-hb.C:
			plazoDeEscritura()
			if _, err := fmt.Fprint(w, ": latido\n\n"); err != nil {
				return
			}
			if rc.Flush() != nil {
				return
			}

		case <-ctx.Done():
			// El cliente cortó. El motor lo ve en su propio select y se va solo; la
			// goroutine no queda colgada porque `hecho` tiene buffer.
			return
		}
	}
}

// eventoDeCierreAnomalo arma el evento terminal de un turno que no llegó a cerrar.
//
// Distingue el CANCEL del llamador de una falla real, y la diferencia importa del lado del
// cliente: `aborted` es «vos lo cortaste» y no merece un cartel de error, mientras que
// `error` es «se rompió». Mezclarlos hace que cada Esc del usuario se vea como una falla del
// servidor.
func eventoDeCierreAnomalo(ctx context.Context, err error) motor.Evento {
	razon := motor.ReasonError
	if errors.Is(ctx.Err(), context.Canceled) {
		razon = motor.ReasonAborted
	}
	// El usage va en cero y no nil: el `error` del protocolo lo declara OBLIGATORIO
	// (proxy.ts:44), así que omitirlo rompe el tipo del otro lado. Cero es honesto —este
	// camino se toma cuando el turno no llegó a reportar consumo— y es la misma decisión que
	// toma el traductor cuando el proveedor corta sin un `message_delta`.
	uso := motor.Usage{}
	return motor.Evento{
		Type:         motor.EvError,
		Reason:       razon,
		ErrorMessage: err.Error(),
		Usage:        &uso,
	}
}
