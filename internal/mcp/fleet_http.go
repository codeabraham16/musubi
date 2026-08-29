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
	"fmt"
	"io"
	"net/http"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// fleetHeartbeatPath es la ruta del latido. Bajo /fleet/ para que la separación se vea en el
// mapa de rutas y no sólo en este comentario.
const fleetHeartbeatPath = "/fleet/heartbeat"

// cuerpoLatido es lo ÚNICO que un dispositivo puede mandar. Tiene un solo campo, y esa pobreza
// es el invariante B4/D5: no hay dónde poner un `device_id`, un `name` ni un `project`. La
// identidad sale del token y de ningún otro lado, así que una máquina no puede reportar las
// métricas de otra ni aunque quiera.
type cuerpoLatido struct {
	// Muestra viaja como RawMessage y NO como *fleet.Muestra para poder pesarla CRUDA: el techo
	// de la telemetría es suyo (fleet.MuestraMaxBytes ≈ 4 KiB) y tiene que seguir siendo suyo
	// aunque el cuerpo entero haya crecido para hacerle lugar al inventario de servicios. Con un
	// solo techo compartido, una muestra de 100 KiB entraría por la puerta que se abrió para las
	// units, y el tope de la telemetría se habría aflojado sin que nadie lo decidiera.
	Muestra jsonpkg.RawMessage `json:"muestra"`
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
	// Servicios es QUÉ CORRE ADENTRO de esta máquina (S12): sus units, sus contenedores.
	//
	// No rompe B4/D5 por la misma razón que `version` y `direccion`: un fleet.ReporteServicio no
	// tiene NINGÚN campo de identidad —ni device, ni project, ni id— así que lo único que estas
	// filas pueden tocar es el inventario de la máquina del token presentado. Y los tags están en
	// castellano a propósito: `nombre`, no `name`.
	Servicios []fleet.ReporteServicio `json:"servicios,omitempty"`
	// PuedePreguntar es una CAPACIDAD MEDIDA por el agente (A57): si en esta máquina hay dónde
	// dibujar un diálogo Y con qué. No es configuración — un servidor sin escritorio no tiene
	// dónde, y afirmarlo desde un archivo haría que un `pide` prometa un permiso que nunca se va
	// a pedir.
	//
	// PUNTERO Y NO bool, y ésa es la diferencia que importa: un agente VIEJO no manda el campo, y
	// con un bool pelado eso sería indistinguible de un agente nuevo que midió y dijo que no. El
	// nil se saltea y conserva lo que hubiera; el `false` explícito SÍ escribe. Sin esto, la
	// primera flota con agentes mezclados vería a los viejos «declarando» que no pueden preguntar
	// cuando en realidad no opinaron.
	PuedePreguntar *bool `json:"puede_preguntar,omitempty"`
	// MotivoNoPreguntar dice POR QUÉ no puede, cuando no puede. Sin él, un `pide` endurecido a
	// `prohibido` en toda la flota es un cero sin explicación, y las tres causas posibles —no hay
	// escritorio, falta un paquete, el agente corre como servicio— se arreglan distinto.
	MotivoNoPreguntar string `json:"motivo_no_preguntar,omitempty"`
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
	// Servicios dice qué pasó con el inventario, por el MISMO motivo que `Muestra`: un bloque
	// descartado en silencio es indistinguible de uno que nunca se mandó, y quien puede arreglarlo
	// —el que administra ESA máquina— es justamente el que no ve los logs del cerebro. Vacío = el
	// agente no mandó ninguno.
	Servicios string `json:"servicios,omitempty"`
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
		muestraJSON, notaMuestra, notaServicios := s.leerCuerpoDelLatido(r, d)

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
		resp := respuestaLatido{OK: true, Device: d.Name, Project: d.ProjectID,
			Muestra: notaMuestra, Servicios: notaServicios}
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

// leerCuerpoDelLatido extrae del cuerpo lo que la máquina reporta de SÍ MISMA: el autorreporte,
// el inventario de servicios y la telemetría. Devuelve el JSON de la muestra a guardar (vacío = no
// tocar la columna) y una nota legible para el agente por cada uno de los dos bloques.
//
// NUNCA DEVUELVE ERROR, y es el invariante D7: un cuerpo roto, una muestra absurda o una
// capacidad que falta descartan la MEDICIÓN, no el LATIDO. Estar viva y saber medirse son cosas
// distintas, y un agente con el colector roto no debe desaparecer del inventario — es
// precisamente cuando más querés verlo.
func (s *McpServer) leerCuerpoDelLatido(r *http.Request, d fleet.Device) (json, notaMuestra, notaServicios string) {
	if r.Body == nil || r.ContentLength == 0 {
		return "", "", ""
	}
	// D6 — el cuerpo está ACOTADO. Un agente corre en la superficie más expuesta de la flota;
	// un cuerpo sin tope es un DoS con forma de telemetría. El techo general del transporte
	// (4 MiB) es absurdamente alto para esta puerta: una muestra son ~300 bytes.
	//
	// El techo lo fija latidoMaxBytes y no MuestraMaxBytes desde que el cuerpo también lleva el
	// inventario de servicios: dejarlo en el de la muestra habría hecho que una máquina con 40
	// units mande un cuerpo sobrado y pierda TAMBIÉN su telemetría, que es la parte que sí
	// entraba. Los dos techos siguen existiendo por separado y cada uno acota lo suyo.
	crudo, err := io.ReadAll(io.LimitReader(r.Body, latidoMaxBytes+1))
	if err != nil {
		return "", "descartada: no se pudo leer el cuerpo", ""
	}
	if len(crudo) > latidoMaxBytes {
		return "", "descartada: cuerpo demasiado grande", ""
	}

	var cuerpo cuerpoLatido
	if err := jsonpkg.Unmarshal(crudo, &cuerpo); err != nil {
		return "", "descartada: JSON inválido", ""
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
	// LA CAPACIDAD DE PREGUNTAR (A57), y el nil se saltea a propósito.
	//
	// Un agente VIEJO no manda el campo. Escribir `false` en ese caso sería afirmar «esta máquina
	// midió y no puede» cuando la verdad es «esta máquina no opinó» — y como `puede_preguntar`
	// endurece un `pide` a `prohibido`, esa afirmación cerraría el acceso por pantalla a una
	// máquina que quizás sí puede, sin que nada lo dijera. El puntero es lo que hace posible la
	// distinción; sin él, una flota con agentes mezclados se rompe callada.
	//
	// El `false` explícito SÍ se escribe: una máquina que perdió su escritorio tiene que dejar de
	// declarar que puede.
	if cuerpo.PuedePreguntar != nil {
		_ = s.engine.FijarCapacidadDePreguntar(d.ID, *cuerpo.PuedePreguntar)
		if !*cuerpo.PuedePreguntar && cuerpo.MotivoNoPreguntar != "" {
			// UNA VEZ POR MÁQUINA Y NO POR LATIDO. Es un ESTADO —el agente corre como servicio,
			// falta zenity— que dura hasta que alguien cambie algo, y un aviso cada 30 s deja de
			// leerse, que es lo mismo que no avisar.
			s.avisarUnaVez("no_puede_preguntar\x00"+d.ID, func() {
				logx.Info("flota: esta máquina no puede pedirle permiso a nadie; un `pide` se le "+
					"endurece a `prohibido`",
					"device", d.Name, "motivo", recortar(cuerpo.MotivoNoPreguntar, fleet.AvisoTextoMax))
			})
		} else if *cuerpo.PuedePreguntar {
			s.avisosDados.Delete("no_puede_preguntar\x00" + d.ID)
		}
	}
	// EL INVENTARIO DE SERVICIOS VA ANTES DEL CORTE POR «no vino muestra», por el mismo motivo
	// que el autorreporte: una máquina en un OS sin colector puede saber perfectamente qué corre
	// adentro suyo, y salir por el `return` de abajo la dejaría sin inventario para siempre.
	notaServicios = s.guardarServiciosDelLatido(d, cuerpo.Servicios)

	if len(cuerpo.Muestra) == 0 || string(cuerpo.Muestra) == "null" {
		return "", "", notaServicios
	}
	// EL TECHO DE LA MUESTRA ES SUYO Y SE MIDE SOBRE EL JSON CRUDO. Medirlo después de
	// deserializar no serviría de nada: los campos que la struct no conoce se pierden en el
	// camino, así que un cuerpo con 4 MiB de basura adentro de `muestra` volvería a pesar 300
	// bytes justo antes de que alguien lo mire.
	if len(cuerpo.Muestra) > fleet.MuestraMaxBytes {
		return "", "descartada: cuerpo demasiado grande", notaServicios
	}

	// D8 — LA CAPACIDAD NO ES DECORATIVA. Una máquina a la que no se le concedió `metrics`
	// late (sigue viva) pero su medición se descarta. Sin esto, conceder capacidades sería un
	// gesto sin efecto y el inventario diría una cosa mientras la base guarda otra.
	if !d.Permite(fleet.CapMetrics) {
		return "", "descartada: esta máquina no tiene concedida la capacidad `metrics`", notaServicios
	}

	var m fleet.Muestra
	if err := jsonpkg.Unmarshal(cuerpo.Muestra, &m); err != nil {
		return "", "descartada: JSON inválido", notaServicios
	}
	// El agente es un cliente y su muestra es entrada NO CONFIABLE, aunque su credencial sea
	// válida: una máquina comprometida puede reportar 900 % de CPU para ensuciar un panel o
	// disparar alertas. No se corrige el valor —eso escondería el problema—, se rechaza entera.
	if err := m.Valida(); err != nil {
		return "", "descartada: " + err.Error(), notaServicios
	}
	texto, err := m.Serializar()
	if err != nil {
		return "", "descartada: no se pudo serializar", notaServicios
	}
	return texto, "guardada", notaServicios
}

// latidoMaxBytes es el techo del CUERPO ENTERO del latido.
//
// Es la muestra (~300 B) más el inventario: fleet.ServiciosPorLatido entradas de a lo sumo
// fleet.SaludMaxBytes cada una, más el sobre. Sigue siendo ridículamente chico comparado con el
// techo general del transporte (4 MiB), que es justamente el punto: esta puerta la abre la
// superficie más expuesta de la flota.
const latidoMaxBytes = fleet.MuestraMaxBytes + fleet.ServiciosPorLatido*fleet.SaludMaxBytes + (8 << 10)

// guardarServiciosDelLatido registra QUÉ CORRE adentro de la máquina (S12). Devuelve la NOTA que
// va de vuelta al agente.
//
// NUNCA DEVUELVE ERROR, y es el mismo invariante D7 que gobierna la muestra: un bloque de
// servicios roto, demasiado largo o sin la capacidad concedida descarta EL INVENTARIO, no el
// LATIDO. Estar viva y saber enumerarse son cosas distintas — y una máquina que no puede
// enumerar sus units es precisamente cuando más querés verla en la lista.
//
// PERO NO SE DESCARTA EN SILENCIO, y por eso hay nota. Un bloque que desaparece sin decir nada se
// ve, DESDE LA MÁQUINA, idéntico a uno que nunca se mandó; y quien puede arreglarlo —el que
// administra ESA máquina— es justamente el que no lee los logs del cerebro. Es la misma decisión
// que ya toma la nota de la muestra, por el mismo motivo.
//
// La asimetría con la muestra: si el bloque se pasa del techo se descarta ENTERO en vez de
// truncarse. Un inventario a medias haría que la poda por ausencia diera de baja los servicios
// que quedaron afuera del corte, que es peor que no actualizar nada.
func (s *McpServer) guardarServiciosDelLatido(d fleet.Device, reportes []fleet.ReporteServicio) string {
	if len(reportes) == 0 {
		return ""
	}
	if len(reportes) > fleet.ServiciosPorLatido {
		return fmt.Sprintf("descartados: %d servicios superan el techo de %d por latido. Reportá menos servicios por vez.",
			len(reportes), fleet.ServiciosPorLatido)
	}
	// D8 — la capacidad no es decorativa, y se reusa `metrics` a propósito: qué corre en una
	// máquina es telemetría del host, del mismo peso que su uso de CPU. Inventar una Cap nueva
	// obligaría a tocar la matriz por tier, la lista de capsQuePuede —cuyo orden dibuja la
	// columna «admite / puedo» del panel— y seis bucles exhaustivos en tres paquetes.
	if !d.Permite(fleet.CapMetrics) {
		return "descartados: esta máquina no tiene concedida la capacidad `metrics`"
	}

	ahora := time.Now()
	nuevos, actualizados, err := s.engine.ReportarServicios(d.ID, ahora, reportes)
	if err != nil {
		return "descartados: el registro no pudo guardarlos"
	}
	// La poda por AUSENCIA: lo que la máquina dejó de reportar se da de baja. `vivos` sale de lo
	// que vino en ESTE latido, ya recortado igual que al guardarlo, para que los nombres coincidan
	// con las filas que se acaban de escribir. Una lista vacía no poda nada (lo garantiza el
	// almacén), así que un bloque entero de reportes inválidos no vacía el inventario.
	//
	// Y LO QUE SE DECLARÓ A MANO NO SE PODA, lo garantiza también el almacén (`declared = 0` en el
	// UPDATE). Es la guarda sin la cual esta línea era una mina con temporizador: la tool de
	// declarar existe para lo que NINGÚN enumerador ve —un Tier B que no enumera, un bot, un
	// puente—, así que el día que el agente aprenda a enumerar sus units, este latido habría
	// borrado de un saque todo lo declarado en toda la flota, y sin vuelta atrás visible.
	vivos := make([]string, 0, len(reportes))
	for _, r := range reportes {
		if r = fleet.RecortarReporte(r); fleet.NombreDeServicioValido(r.Nombre) {
			vivos = append(vivos, r.Nombre)
		}
	}
	podados, _ := s.engine.PodarServiciosAusentes(d.ID, vivos)
	return fmt.Sprintf("guardados: %d nuevo(s), %d actualizado(s), %d dado(s) de baja por ausencia",
		nuevos, actualizados, podados)
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

// ── La puerta del RENDIMIENTO: salud para lo que ninguna máquina enumera (fase 4) ────────────

// fleetSaludPath es la ruta por la que un colector le pone salud a un servicio DECLARADO.
const fleetSaludPath = "/fleet/service-health"

// cuerpoSalud es lo que manda un colector. NO trae identidad, igual que el latido y el resultado:
// la máquina sale del TOKEN. Que no tenga POR DÓNDE pasarla es la garantía, no la disciplina.
type cuerpoSalud struct {
	Servicios []fleet.ReporteServicio `json:"servicios"`
}

// saludMaxBytes acota el cuerpo. Es chico a propósito: por acá entra un puñado de servicios
// DECLARADOS, no un inventario de 54 units como el del latido.
const saludMaxBytes = 64 << 10

// respuestaSalud es lo que contesta la puerta. Los `desconocidos` viajan porque el error más
// probable de este camino es apuntar a un nombre que nadie declaró —un typo, un servicio que se
// dio de baja— y su síntoma, sin esto, sería un panel que nunca cambia y un colector convencido
// de que está reportando.
type respuestaSalud struct {
	OK           bool     `json:"ok"`
	Device       string   `json:"device,omitempty"`
	Actualizados int      `json:"actualizados"`
	Desconocidos []string `json:"desconocidos,omitempty"`
	Motivo       string   `json:"motivo,omitempty"`
}

// handlerSaludDeServicios recibe salud (y rendimiento) para servicios de ESTA máquina.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ES UNA PUERTA APARTE Y NO UN CAMPO MÁS DEL LATIDO
//
// El latido hace DOS cosas que acá serían un bug:
//
//  1. PODA POR AUSENCIA. Un colector que manda un solo servicio por el camino del latido borraría
//     los otros 53 de esa máquina. La poda es correcta para un inventario —«esto es TODO lo que
//     corre acá»— y es una afirmación que un colector de un bot no está en condiciones de hacer.
//  2. ESTAMPA SEÑAL DE VIDA. Si este reporte marcara viva a la máquina, un host cuyo AGENTE murió
//     pero cuyo colector sigue corriendo figuraría sano — y el colector es justamente lo que menos
//     se cae, porque es un cron de un minuto.
//
// Misma puerta y mismo almacén de credenciales que el latido: el token del dispositivo, que no
// abre /mcp. No amplía lo que ese token puede hacer: por el latido ya podía escribir la salud de
// los servicios de su propia máquina. Lo que hace es dejarlo escribir SIN afirmar las otras dos
// cosas.
func (s *McpServer) handlerSaludDeServicios(limiter *authLimiter) http.HandlerFunc {
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
			escribirSalud(w, http.StatusUnauthorized, respuestaSalud{OK: false, Motivo: motivoRechazo})
			return
		}
		limiter.reset(ip)

		crudo, err := io.ReadAll(io.LimitReader(r.Body, saludMaxBytes+1))
		if err != nil || len(crudo) > saludMaxBytes {
			http.Error(w, "cuerpo demasiado grande", http.StatusRequestEntityTooLarge)
			return
		}
		var cuerpo cuerpoSalud
		if err := jsonpkg.Unmarshal(crudo, &cuerpo); err != nil {
			escribirSalud(w, http.StatusBadRequest, respuestaSalud{OK: false, Motivo: "cuerpo inválido"})
			return
		}
		if len(cuerpo.Servicios) > fleet.ServiciosPorLatido {
			escribirSalud(w, http.StatusRequestEntityTooLarge, respuestaSalud{OK: false,
				Motivo: "demasiados servicios en un reporte"})
			return
		}

		actualizados, desconocidos, err := s.engine.ReportarSaludDeServicios(d.ID, time.Now(), cuerpo.Servicios)
		if err != nil {
			escribirSalud(w, http.StatusServiceUnavailable, respuestaSalud{OK: false, Motivo: "no se pudo guardar"})
			return
		}
		// UN NOMBRE DESCONOCIDO NO ES UN ERROR HTTP. El reporte llegó y se aplicó lo que se pudo;
		// devolver 4xx haría que un colector con un typo en UN servicio deje de reportar los otros
		// —o peor, que su cron loguee un error rojo cada minuto y alguien lo silencie—. Se contesta
		// 200 con la lista, que es la información que hace falta para arreglarlo.
		escribirSalud(w, http.StatusOK, respuestaSalud{
			OK: true, Device: d.Name, Actualizados: actualizados, Desconocidos: desconocidos})
	}
}

func escribirSalud(w http.ResponseWriter, code int, r respuestaSalud) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = jsonpkg.NewEncoder(w).Encode(r)
}
