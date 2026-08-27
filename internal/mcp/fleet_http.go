package mcp

// fleet_http.go es LA PUERTA DEL DISPOSITIVO: el endpoint por el que una máquina de la flota dice
// «sigo viva». Track «Control de flota», slice S2.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ES UNA PUERTA APARTE Y NO UN PRINCIPAL MÁS
//
// La vía cómoda sería darle a cada máquina una línea en principals.yaml y que late por /mcp como
// todo el mundo: cero código nuevo. Sería el error del track.
//
// Un agente corre en CADA máquina de la flota — la de un cliente, un portátil que viaja, un
// Windows con antivirus ajeno. Es la superficie más expuesta del sistema, y la que más
// probablemente se comprometa. Si su credencial abriera /mcp, entonces robar UNA máquina
// cualquiera entregaría musubi_recall sobre la memoria de toda la empresa: el plano de monitoreo
// se convertiría en el plano de exfiltración.
//
// Por eso las credenciales viven en DOS ALMACENES DISTINTOS:
//
//	personas      -> principals.yaml  -> autentican en /mcp
//	dispositivos  -> tabla `devices`  -> autentican acá, y sólo acá
//
// La separación es ESTRUCTURAL, no una promesa: el handler de /mcp resuelve contra
// PrincipalRegistry y no tiene forma de llegar a la tabla `devices`; este handler resuelve contra
// la tabla y no mira el registro. Ninguno de los dos puede autenticar la credencial del otro
// aunque quisiera.
//
// Las pruebas B1 y B2 existen igual, y no son ceremonia: «estructural hoy» y «estructural dentro
// de un año» son cosas distintas. Unificar los dos lookups «para simplificar» es exactamente el
// refactor que alguien va a proponer, y esas dos pruebas son las que lo van a frenar.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	jsonpkg "encoding/json"
	"io"
	"net/http"
	"time"

	"musubi/internal/fleet"
)

// fleetHeartbeatPath es la ruta del latido. Bajo /fleet/ para que la separación se vea en el
// mapa de rutas y no sólo en este comentario.
const fleetHeartbeatPath = "/fleet/heartbeat"

// cuerpoLatido es lo ÚNICO que un dispositivo puede mandar. Tiene un solo campo, y esa pobreza
// es el invariante B4/D5: no hay dónde poner un `device_id`, un `name` ni un `project`. La
// identidad sale del token y de ningún otro lado, así que una máquina no puede reportar las
// métricas de otra ni aunque quiera.
type cuerpoLatido struct {
	Muestra *fleet.Muestra `json:"muestra"`
	// Version y Direccion son lo que la máquina sabe de SÍ MISMA y el cerebro no puede
	// averiguar solo: qué build del agente corre y por qué dirección se la alcanza.
	//
	// Que el device escriba en su propia fila NO rompe B4/D5. El invariante es que no puede
	// decir QUIÉN ES —eso sale del token—, no que no pueda decir CÓMO ESTÁ. Sin campos de
	// identidad acá, la única fila que estos valores pueden tocar es la del token presentado.
	Version   string `json:"version"`
	Direccion string `json:"direccion"`
	// RustdeskID es el identificador PÚBLICO del cliente de pantalla (S6). No es un secreto: sin
	// la contraseña de sesión no sirve para entrar, y sin él quien mira no sabe a qué conectarse.
	RustdeskID string `json:"rustdesk_id"`
}

// respuestaLatido es lo que ve el agente. Deliberadamente pobre: no devuelve nada que no le
// pertenezca a esa máquina.
type respuestaLatido struct {
	OK      bool   `json:"ok"`
	Device  string `json:"device,omitempty"`
	Project string `json:"project,omitempty"`
	// Comandos son los pedidos de ejecución que le tocan a ESTA máquina (S5). Viajan de vuelta
	// en la respuesta del latido, por el canal que el agente ya abre él mismo: poner al agente a
	// escuchar un puerto sería la superficie que este track viene evitando desde S2, y sería
	// inútil la mitad de las veces porque esa máquina está detrás de un NAT.
	Comandos []comandoParaElAgente `json:"comandos,omitempty"`
	// Muestra dice qué pasó con la telemetría: "guardada", "descartada: <razón>" o vacío si el
	// agente no mandó ninguna. El agente lo imprime, así que un colector roto o una capacidad
	// que falta se ven DESDE LA MÁQUINA en vez de desaparecer en silencio en el cerebro.
	Muestra string `json:"muestra,omitempty"`
	// Motivo viaja SÓLO en el 401 y es el mismo texto para todos los rechazos (B3).
	Motivo string `json:"motivo,omitempty"`
}

// comandoParaElAgente es lo MÍNIMO que el agente necesita para ejecutar. No viaja quién lo pidió
// ni por qué: el agente no tiene nada que hacer con esa información, y todo lo que viaja a la
// máquina más expuesta de la flota es superficie.
type comandoParaElAgente struct {
	ID         string   `json:"id"`
	Argv       []string `json:"argv"`
	TimeoutSeg int      `json:"timeout_seg"`
}

// maxComandosPorLatido acota cuántos se entregan de una. Diez alcanzan para cualquier ráfaga
// real y evitan que una cola acumulada por una máquina que estuvo caída le caiga encima toda
// junta al volver.
const maxComandosPorLatido = 10

// motivoRechazo es el ÚNICO texto de rechazo, y es el mismo para un token desconocido, uno
// revocado y uno con formato raro.
//
// No es pereza: distinguir «no existe» de «revocado» convierte el endpoint en un ORÁCULO. Quien
// prueba credenciales aprendería cuáles existieron alguna vez, que es justo lo que no se le
// quiere decir. El agente legítimo no necesita el detalle — para él las tres respuestas
// significan lo mismo: «dejá de latir y avisá».
const motivoRechazo = "credencial de dispositivo inválida o revocada: dejá de latir y pedí un alta nueva"

// handlerLatido devuelve el handler de POST /fleet/heartbeat.
//
// El `limiter` es el MISMO que protege /mcp, compartido a propósito: una puerta nueva sin lockout
// es un oráculo de fuerza bruta con la tabla de dispositivos entera detrás, y dos limitadores
// separados dejarían que un atacante gaste su cuota en una puerta y siga entero en la otra.
func (s *McpServer) handlerLatido(limiter *authLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ip := clientIP(r)
		if limiter.locked(ip, time.Now()) {
			http.Error(w, "too many failed auth attempts", http.StatusTooManyRequests)
			return
		}

		// La identidad sale del TOKEN y de ningún otro lado (invariante A1 de S1). No se lee el
		// cuerpo del request: no hay ningún campo que el dispositivo pueda mandar para decir
		// quién es. Esa ausencia es el invariante B4 — un cuerpo con `device_id` no tendría dónde
		// aterrizar aunque lo mandaran.
		token := bearerToken(r.Header.Get("Authorization"))
		d, ok, err := s.engine.DevicePorToken(token)
		if err != nil {
			// Un fallo de la base NO es un rechazo de credencial: no gasta la cuota del limiter
			// (si no, una base caída bloquearía por IP a toda la flota legítima) y se responde
			// 503, que es lo que el agente debe reintentar.
			http.Error(w, "device registry unavailable", http.StatusServiceUnavailable)
			return
		}
		if !ok {
			limiter.fail(ip, time.Now())
			w.Header().Set("WWW-Authenticate", "Bearer")
			escribirLatido(w, http.StatusUnauthorized, respuestaLatido{OK: false, Motivo: motivoRechazo})
			return
		}
		limiter.reset(ip)

		// La telemetría (S4). Se lee DESPUÉS de autenticar, nunca antes: leer el cuerpo de un
		// desconocido es trabajo gratis para quien lo mande.
		muestraJSON, notaMuestra := s.leerMuestraDelLatido(r, d)

		// LatirDevice devuelve (false, nil) si la fila ya no está activa. Es una carrera real y
		// benigna: entre el DevicePorToken de arriba y este UPDATE, un admin pudo revocar. Se
		// trata igual que un token inválido — el agente tiene que dejar de latir — y NO como un
		// error del servidor.
		actualizado, err := s.engine.LatirDevice(d.ID, time.Now(), muestraJSON)
		if err != nil {
			http.Error(w, "device registry unavailable", http.StatusServiceUnavailable)
			return
		}
		if !actualizado {
			w.Header().Set("WWW-Authenticate", "Bearer")
			escribirLatido(w, http.StatusUnauthorized, respuestaLatido{OK: false, Motivo: motivoRechazo})
			return
		}

		// La cola de ESTA máquina. `d` salió de resolver el token, así que un agente no puede
		// pedir la cola de otro (F5). Un fallo acá NO tumba el latido: seguir viva es lo que el
		// latido afirma, y quedarse sin comandos un ciclo es recuperable.
		resp := respuestaLatido{OK: true, Device: d.Name, Project: d.ProjectID, Muestra: notaMuestra}
		if pendientes, err := s.engine.TomarComandos(d.ID, time.Now(), maxComandosPorLatido); err == nil {
			for _, c := range pendientes {
				resp.Comandos = append(resp.Comandos, comandoParaElAgente{
					ID: c.ID, Argv: c.Argv, TimeoutSeg: int(c.Timeout.Seconds()),
				})
			}
		}
		escribirLatido(w, http.StatusOK, resp)
	}
}

// leerMuestraDelLatido extrae la telemetría del cuerpo, si vino. Devuelve el JSON a guardar
// (vacío = no tocar la columna) y una nota legible para el agente.
//
// NUNCA DEVUELVE ERROR, y es el invariante D7: un cuerpo roto, una muestra absurda o una
// capacidad que falta descartan la MEDICIÓN, no el LATIDO. Estar viva y saber medirse son cosas
// distintas, y un agente con el colector roto no debe desaparecer del inventario — es
// precisamente cuando más querés verlo.
func (s *McpServer) leerMuestraDelLatido(r *http.Request, d fleet.Device) (json string, nota string) {
	if r.Body == nil || r.ContentLength == 0 {
		return "", ""
	}
	// D6 — el cuerpo está ACOTADO. Un agente corre en la superficie más expuesta de la flota;
	// un cuerpo sin tope es un DoS con forma de telemetría. El techo general del transporte
	// (4 MiB) es absurdamente alto para esta puerta: una muestra son ~300 bytes.
	crudo, err := io.ReadAll(io.LimitReader(r.Body, fleet.MuestraMaxBytes+1))
	if err != nil {
		return "", "descartada: no se pudo leer el cuerpo"
	}
	if len(crudo) > fleet.MuestraMaxBytes {
		return "", "descartada: cuerpo demasiado grande"
	}

	var cuerpo cuerpoLatido
	if err := jsonpkg.Unmarshal(crudo, &cuerpo); err != nil {
		return "", "descartada: JSON inválido"
	}
	// EL AUTORREPORTE VA ANTES DEL CORTE POR «no vino muestra», y el orden es el invariante.
	//
	// Se escribió al revés la primera vez y las pruebas lo agarraron: un agente en un OS sin
	// colector manda `{"version":"..."}` SIN muestra, salía por el return de abajo y nunca se
	// identificaba. Justo la máquina de la que menos se sabe era la que se quedaba anónima.
	//
	// Tampoco depende de `metrics`: qué build corre una máquina y por dónde se la alcanza es
	// INVENTARIO, no telemetría. Best-effort — si falla, el latido sigue valiendo.
	if cuerpo.Version != "" || cuerpo.Direccion != "" {
		_ = s.engine.ActualizarAutoreporte(d.ID, recortar(cuerpo.Version, 64), recortar(cuerpo.Direccion, 128))
	}
	if cuerpo.RustdeskID != "" {
		_ = s.engine.GuardarRustdeskID(d.ID, cuerpo.RustdeskID)
	}

	if cuerpo.Muestra == nil {
		return "", ""
	}

	// D8 — LA CAPACIDAD NO ES DECORATIVA. Una máquina a la que no se le concedió `metrics`
	// late (sigue viva) pero su medición se descarta. Sin esto, conceder capacidades sería un
	// gesto sin efecto y el inventario diría una cosa mientras la base guarda otra.
	if !d.Permite(fleet.CapMetrics) {
		return "", "descartada: esta máquina no tiene concedida la capacidad `metrics`"
	}

	// El agente es un cliente y su muestra es entrada NO CONFIABLE, aunque su credencial sea
	// válida: una máquina comprometida puede reportar 900 % de CPU para ensuciar un panel o
	// disparar alertas. No se corrige el valor —eso escondería el problema—, se rechaza entera.
	if err := cuerpo.Muestra.Valida(); err != nil {
		return "", "descartada: " + err.Error()
	}
	texto, err := cuerpo.Muestra.Serializar()
	if err != nil {
		return "", "descartada: no se pudo serializar"
	}
	return texto, "guardada"
}

func escribirLatido(w http.ResponseWriter, code int, resp respuestaLatido) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = jsonpkg.NewEncoder(w).Encode(resp)
}

// recortar acota un texto que viene del device. El cuerpo ya está acotado en bytes, pero un
// `agent_version` de 4 KiB seguiría ensuciando el inventario y las etiquetas de Prometheus: un
// campo que se muestra en una tabla tiene que tener un tamaño de tabla.
func recortar(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// fleetResultPath es la ruta por la que el agente reporta cómo salió un comando.
const fleetResultPath = "/fleet/result"

// cuerpoResultado es lo que manda el agente al terminar. NO trae identidad, igual que el latido:
// el `command_id` se verifica CONTRA la máquina del token, así que nombrar un comando ajeno no
// alcanza para escribirlo (F3).
type cuerpoResultado struct {
	ComandoID string `json:"command_id"`
	ExitCode  *int   `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error"`
}

// resultadoMaxBytes acota el cuerpo del reporte: dos salidas de 64 KiB más el sobre.
const resultadoMaxBytes = 2*fleet.SalidaMaxBytes + (8 << 10)

// handlerResultado recibe el resultado de un comando.
//
// Misma puerta y mismo almacén de credenciales que el latido: es el agente el que reporta, y su
// token no abre /mcp. La guarda que importa está una capa abajo, en GuardarResultado: el comando
// tiene que ser de ESTA máquina.
func (s *McpServer) handlerResultado(limiter *authLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ip := clientIP(r)
		if limiter.locked(ip, time.Now()) {
			http.Error(w, "too many failed auth attempts", http.StatusTooManyRequests)
			return
		}
		d, ok, err := s.engine.DevicePorToken(bearerToken(r.Header.Get("Authorization")))
		if err != nil {
			http.Error(w, "device registry unavailable", http.StatusServiceUnavailable)
			return
		}
		if !ok {
			limiter.fail(ip, time.Now())
			w.Header().Set("WWW-Authenticate", "Bearer")
			escribirLatido(w, http.StatusUnauthorized, respuestaLatido{OK: false, Motivo: motivoRechazo})
			return
		}
		limiter.reset(ip)

		crudo, err := io.ReadAll(io.LimitReader(r.Body, resultadoMaxBytes+1))
		if err != nil || len(crudo) > resultadoMaxBytes {
			http.Error(w, "cuerpo demasiado grande", http.StatusRequestEntityTooLarge)
			return
		}
		var cuerpo cuerpoResultado
		if err := jsonpkg.Unmarshal(crudo, &cuerpo); err != nil {
			http.Error(w, "cuerpo inválido", http.StatusBadRequest)
			return
		}

		// Si el comando era una operación de PANTALLA, su resultado también cierra el estado de
		// la sesión. Sin esto la sesión queda en `solicitada` para siempre y la bitácora no
		// distingue «se aplicó» de «la máquina no pudo» — que es justo lo que se va a mirar
		// cuando alguien diga «no me deja entrar».
		s.marcarSesionSiEsDePantalla(d.ID, cuerpo)

		// F3 — la guarda: el comando tiene que pertenecer a la máquina del TOKEN. Un rechazo acá
		// es un intento de escribir en la bitácora de otro, así que gasta cuota del limiter.
		if err := s.engine.GuardarResultado(d.ID, cuerpo.ComandoID, cuerpo.ExitCode,
			cuerpo.Stdout, cuerpo.Stderr, cuerpo.Error, time.Now()); err != nil {
			limiter.fail(ip, time.Now())
			escribirLatido(w, http.StatusForbidden, respuestaLatido{OK: false, Motivo: "ese comando no es de esta máquina"})
			return
		}
		escribirLatido(w, http.StatusOK, respuestaLatido{OK: true, Device: d.Name})
	}
}

// marcarSesionSiEsDePantalla cierra el estado de una sesión cuando el resultado que llega
// corresponde a una operación `musubi:pantalla`.
//
// El id de la sesión sale del ARGV DEL COMANDO GUARDADO, no del cuerpo que mandó el agente: si
// saliera del cuerpo, una máquina podría marcar como activa la sesión de otra. Es la misma
// disciplina que el resto del track — el dato de autoridad se lee de donde ya está verificado.
//
// Best-effort y silenciosa: el resultado del comando se guarda igual aunque esto falle. La
// bitácora de comandos es la fuente; ésta es la vista cómoda.
func (s *McpServer) marcarSesionSiEsDePantalla(deviceID string, cuerpo cuerpoResultado) {
	cmd, existe, err := s.engine.ComandoPorID(cuerpo.ComandoID)
	if err != nil || !existe || cmd.DeviceID != deviceID || !EsComandoDePantalla(cmd.Argv) || len(cmd.Argv) < 2 {
		return
	}
	estado, motivo := fleet.SesionActiva, ""
	if cuerpo.Error != "" || (cuerpo.ExitCode != nil && *cuerpo.ExitCode != 0) {
		estado, motivo = fleet.SesionFallida, cuerpo.Error
	}
	_ = s.engine.MarcarSesion(deviceID, cmd.Argv[1], estado, motivo, time.Now())
}
