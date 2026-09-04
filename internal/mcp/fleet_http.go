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
	"strings"
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

// El CONTRATO del latido —qué contesta el cerebro y qué lee el agente— vive en
// internal/fleet/protocolo.go, en UN solo tipo que usan los dos lados. Estaba duplicado acá y en
// cmd/musubi, y esa duplicación es lo que dejó `token_nuevo` y `servicios` sin receptor: encoding/
// json descarta los campos que el otro lado no declara, sin error y sin log. El porqué largo está
// allá.

// maxComandosPorLatido acota cuántos se entregan de una. Diez alcanzan para cualquier ráfaga
// real y evitan que una cola acumulada por una máquina que estuvo caída le caiga encima toda
// junta al volver.
// EL NÚMERO VIVE EN EL DOMINIO (`fleet.ComandosPorEntregaMax`) y acá sólo se usa: la derivación
// de `perdido` depende de él, así que dos definiciones se desincronizarían y volverían la cota
// incorrecta EN SILENCIO — subir el tope acá sin tocar la otra haría que un comando vivo se
// dibuje muerto.
const maxComandosPorLatido = fleet.ComandosPorEntregaMax

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
		// RESUELVE POR CUALQUIERA DE LOS DOS TOKENS mientras hay una rotación abierta (Ola 2).
		// El agente se entera del nuevo en la RESPUESTA de un latido, o sea después de haber
		// usado el viejo: sin solapamiento quedaría afuera entre que lo recibe y lo guarda.
		d, conElNuevo, ok, err := s.engine.DevicePorTokenConRotacion(token)
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
			escribirLatido(w, http.StatusUnauthorized, fleet.RespuestaLatido{OK: false, Motivo: motivoRechazo})
			return
		}
		limiter.reset(ip)

		// La telemetría (S4). Se lee DESPUÉS de autenticar, nunca antes: leer el cuerpo de un
		// desconocido es trabajo gratis para quien lo mande.
		// LA ROTACIÓN SE COMPLETA ACÁ, y no al emitir el token: llegar con el nuevo es la ÚNICA
		// prueba de que el agente lo guardó y lo puede usar. Emitirlo y darlo por cerrado sería
		// creerle al remitente en vez de al receptor — el mismo error que cerró A78.
		//
		// Best-effort: si falla, el viejo sigue valiendo y el próximo latido reintenta. Lo que NO
		// puede pasar es que el latido se rechace por esto.
		if conElNuevo {
			if err := s.engine.CompletarRotacion(d.ID); err != nil {
				logx.Warn("flota: no se pudo completar la rotación del token; el viejo sigue valiendo y se reintenta en el próximo latido",
					"device", d.Name, "error", err)
			} else {
				// Y el secreto se olvida: completada la rotación no tiene ninguna razón para
				// seguir existiendo en memoria.
				s.olvidarRotacion(d.ID)
				logx.Info("flota: rotación de token completada; el token anterior dejó de valer", "device", d.Name)
			}
		}
		muestraJSON, notaMuestra, notaServicios := s.leerCuerpoDelLatido(r, d)

		// UNA SOLA TRANSACCIÓN PARA LAS DOS ESCRITURAS QUE SIEMPRE OCURREN: la señal de vida y
		// el paso por la cola (que vence lo viejo y marca entregado lo que se lleva). Eran dos
		// —`LatirDevice` y `TomarComandos`— y a 2000 máquinas cada 30 s eso es un fsync de más
		// 67 veces por segundo, todos los segundos. El por qué completo está en
		// internal/memory/latido.go; lo que importa acá es que el TOPE de escrituras por latido
		// es una propiedad de ESTA función, y lo fija una prueba (latido_una_tx_test.go).
		//
		// `actualizado == false` significa que la fila ya no está activa. Es una carrera real y
		// benigna: entre el DevicePorToken de arriba y este UPDATE, un admin pudo revocar. Se
		// trata igual que un token inválido —el agente tiene que dejar de latir— y NO como un
		// error del servidor. En ese caso tampoco se entrega nada de la cola, que es lo que uno
		// espera de una máquina que acaba de ser dada de baja.
		//
		// La cola es la de ESTA máquina: `d` salió de resolver el token, así que un agente no
		// puede pedir la de otro (F5). Un fallo de la cola sigue sin tumbar el latido — eso lo
		// garantiza el motor, no este handler.
		actualizado, pendientes, err := s.engine.LatirYTomarComandos(
			d.ID, time.Now(), muestraJSON, maxComandosPorLatido)
		if err != nil {
			http.Error(w, "device registry unavailable", http.StatusServiceUnavailable)
			return
		}
		if !actualizado {
			w.Header().Set("WWW-Authenticate", "Bearer")
			escribirLatido(w, http.StatusUnauthorized, fleet.RespuestaLatido{OK: false, Motivo: motivoRechazo})
			return
		}

		resp := fleet.RespuestaLatido{OK: true, Device: d.Name, Project: d.ProjectID,
			Muestra: notaMuestra, Servicios: notaServicios}
		// EL TOKEN NUEVO VIAJA MIENTRAS EL AGENTE SIGA LATIENDO CON EL VIEJO, y deja de viajar en
		// cuanto late con el nuevo (ahí la rotación ya se completó unas líneas más arriba).
		//
		// Repetirlo en cada latido no es un descuido: el agente puede fallar al guardarlo —disco
		// lleno, permisos— y un token que se manda una sola vez convierte ese fallo en una
		// rotación que nadie puede completar y nadie sabe por qué. Repetido, el próximo latido lo
		// vuelve a traer; y como el viejo sigue valiendo, la máquina nunca queda afuera.
		if !conElNuevo {
			if tok, hay := s.tokenDeRotacionPendiente(d.ID); hay {
				resp.TokenNuevo = tok
			}
		}
		for _, c := range pendientes {
			resp.Comandos = append(resp.Comandos, fleet.ComandoParaElAgente{
				ID: c.ID, Argv: c.Argv, TimeoutSeg: int(c.Timeout.Seconds()),
			})
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
	//
	// Y NO SE REESCRIBE SI DICE LO MISMO QUE YA ESTÁ GUARDADO. El agente manda su versión en
	// TODOS los latidos, así que este UPDATE ocurría 67 veces por segundo en una flota de 2000
	// máquinas para no cambiar NADA: la versión de un agente cambia cuando alguien lo actualiza,
	// o sea nunca, y la dirección cuando se muda de red. Un UPDATE que reasigna una fila a sí
	// misma cuesta exactamente lo mismo que uno que la cambia —página sucia, frame de WAL y
	// fsync— y no hay nada del otro lado que lo compense.
	//
	// LA COMPARACIÓN ES CONTRA `d`, que es la fila que se leyó al resolver el token unas líneas
	// más arriba: no cuesta ni una lectura de más. Se compara YA RECORTADO Y SIN ESPACIOS, que es
	// la forma exacta en la que ActualizarAutoreporte lo iba a guardar; comparar el crudo haría
	// que un agente que manda su versión con un espacio al final reescriba en cada latido.
	//
	// El sesgo del error es el seguro: si la comparación se equivoca, escribe de más (una
	// escritura idéntica, inofensiva), nunca de menos.
	version := strings.TrimSpace(recortar(cuerpo.Version, 64))
	direccion := strings.TrimSpace(recortar(cuerpo.Direccion, 128))
	if (version != "" && version != d.AgentVer) || (direccion != "" && direccion != d.Address) {
		_ = s.engine.ActualizarAutoreporte(d.ID, version, direccion)
	}
	// Mismo criterio para el id de RustDesk, que también viene en cada latido de una máquina con
	// escritorio remoto. Saltearlo cuando no cambió no pierde nada: el UPDATE de
	// GuardarRustdeskID anota el «previo» sólo cuando el valor es DISTINTO, así que un reporte
	// idéntico ya no cambiaba ninguna de las tres columnas.
	//
	// El 32 es el mismo recorte que aplica el motor. Si divergiera, el peor caso es volver a
	// escribir un valor que ya estaba — nunca saltear un cambio real.
	if rid := strings.TrimSpace(recortar(cuerpo.RustdeskID, 32)); rid != "" && rid != d.RustdeskID {
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
		// TAMPOCO SE REESCRIBE SI NO CAMBIÓ, por lo mismo que el autorreporte: el campo viene en
		// todos los latidos y su valor se mueve cuando alguien instala o saca un escritorio, o
		// sea casi nunca. La distinción que importa —«no opinó» contra «opinó que no»— la sigue
		// haciendo el `nil` de arriba, que es la guarda de A57 y no se toca: acá adentro ya hay
		// una opinión, y lo único que se decide es si hace falta guardarla.
		//
		// El aviso queda AFUERA de esta guarda a propósito. No es una escritura de la base, es
		// una decisión sobre qué se le dice a quien mira los logs, y ya tiene su propio
		// «una vez por máquina» (avisarUnaVez). Meterlo adentro haría que el aviso dependa de si
		// la columna ya estaba en ese valor, que es una relación que nadie querría explicar.
		if *cuerpo.PuedePreguntar != d.PuedePreguntar {
			_ = s.engine.FijarCapacidadDePreguntar(d.ID, *cuerpo.PuedePreguntar)
		}
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
	// AUSENTE Y VACÍO NO SON LO MISMO, y toda A78 vive en esa distinción.
	//
	// `nil` es «el bloque no vino»: el latido de siempre, sin novedad de inventario. No hay nada
	// que hacer y no hay nada que decir.
	//
	// Una lista NO nil y de largo cero es lo contrario de un silencio: la máquina mandó el bloque
	// para decir que no corre nada. El agente manda el inventario COMPLETO o no lo manda —si una
	// fuente falla, aborta el lote a propósito y no manda el campo—, así que un `[]` que llega
	// acá enumeró bien y no encontró nada. Es un hecho, y se guarda como tal.
	//
	// En la práctica «cero servicios» en una máquina real no existe: ni systemd ni el SCM tienen
	// cero. Así que esto casi siempre significa que el enumerador se rompió sin dar error — y por
	// eso mismo tiene que PODAR: el inventario viejo queda revocado, la máquina cae en
	// `MaquinaSinInventario` a los 15 minutos, y alguien mira. La alternativa era la de antes:
	// un panel con servicios fantasma que nadie desmiente nunca.
	if reportes == nil {
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
	nuevos, actualizados := 0, 0
	if len(reportes) > 0 {
		var err error
		nuevos, actualizados, err = s.engine.ReportarServicios(d.ID, ahora, reportes)
		if err != nil {
			return "descartados: el registro no pudo guardarlos"
		}
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
	// LA AUTORIZACIÓN PARA VACIAR SE GANA ACÁ Y EN NINGÚN OTRO LADO: sólo cuando la lista llegó
	// vacía DE ORIGEN. Un lote que traía reportes y se quedó sin ninguno válido al filtrarlo no
	// es una máquina diciendo «no corre nada»: es un lote roto, y ésos no podan —que es la guarda
	// que existía desde antes y sigue entera.
	vacioAfirma := len(reportes) == 0
	podados, _ := s.engine.PodarServiciosAusentes(d.ID, vivos, vacioAfirma)
	if vacioAfirma {
		return fmt.Sprintf("inventario VACÍO reportado: %d servicio(s) dado(s) de baja. La máquina dice que no corre nada; en un sistema real eso casi siempre es un enumerador roto.", podados)
	}
	return fmt.Sprintf("guardados: %d nuevo(s), %d actualizado(s), %d dado(s) de baja por ausencia",
		nuevos, actualizados, podados)
}

func escribirLatido(w http.ResponseWriter, code int, resp fleet.RespuestaLatido) {
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
// Es un ALIAS del tipo del contrato (internal/fleet/protocolo.go) y no una copia: era idéntico
// byte a byte al del agente, y el porqué de que eso sea peligroso está allá.
type cuerpoResultado = fleet.ResultadoDeComando

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
			escribirLatido(w, http.StatusUnauthorized, fleet.RespuestaLatido{OK: false, Motivo: motivoRechazo})
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
		// Y si era un PEDIDO DE PERMISO, su respuesta saca la sesión de `esperando_permiso`.
		s.registrarRespuestaDePermiso(d.ID, cuerpo)

		// F3 — la guarda: el comando tiene que pertenecer a la máquina del TOKEN. Un rechazo acá
		// es un intento de escribir en la bitácora de otro, así que gasta cuota del limiter.
		if err := s.engine.GuardarResultado(d.ID, cuerpo.ComandoID, cuerpo.ExitCode,
			cuerpo.Stdout, cuerpo.Stderr, cuerpo.Error, time.Now()); err != nil {
			limiter.fail(ip, time.Now())
			escribirLatido(w, http.StatusForbidden, fleet.RespuestaLatido{OK: false, Motivo: "ese comando no es de esta máquina"})
			return
		}
		escribirLatido(w, http.StatusOK, fleet.RespuestaLatido{OK: true, Device: d.Name})
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

// registrarRespuestaDePermiso recoge lo que contestó el usuario de la máquina (A57).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA SESIÓN SALE DEL COMANDO, NO DEL CUERPO
//
// El agente contesta con el `command_id`, y de ahí se saca la sesión. Es la misma garantía que
// usa el resultado de pantalla: sin ella, un agente podría nombrar la sesión de OTRA máquina y
// contestarle «concedida» a una pregunta que se le hizo a otra persona. La comprobación final
// —que la sesión sea de este device— vive una capa más abajo, en ResponderConsentimiento.
//
// UNA RESPUESTA QUE NO SE ENTIENDE NO SE DEGRADA A «NEGADA» EN SILENCIO. Parece seguro y esconde
// que hay un agente hablando un protocolo que este cerebro no conoce; se registra `no_se_pudo`,
// que es la categoría honesta —la máquina no pudo darnos una respuesta— y deja rastro.
func (s *McpServer) registrarRespuestaDePermiso(deviceID string, cuerpo cuerpoResultado) {
	cmd, existe, err := s.engine.ComandoPorID(cuerpo.ComandoID)
	if err != nil || !existe || cmd.DeviceID != deviceID || len(cmd.Argv) < 2 ||
		cmd.Argv[0] != comandoPreguntar {
		return
	}
	r := fleet.RespuestaNoSePudo
	if cuerpo.Error == "" {
		if v, ok := strings.CutPrefix(strings.TrimSpace(cuerpo.Stdout), prefijoRespuestaPermiso); ok {
			if cand := fleet.RespuestaAviso(strings.TrimSpace(v)); cand.Valida() {
				r = cand
			}
		}
	}
	if err := s.engine.ResponderConsentimiento(deviceID, cmd.Argv[1], r, time.Now()); err != nil {
		// Incluye la sesión ajena, la inexistente y la YA CONTESTADA: las tres dan el mismo error
		// una capa más abajo, a propósito. Acá se logea porque un agente que insiste en contestar
		// sesiones que no son suyas es una señal, no un detalle.
		logx.Warn("flota: se descartó una respuesta de permiso", "device_id", deviceID,
			"comando", cuerpo.ComandoID, "error", err)
	}
}

// prefijoRespuestaPermiso sale del CONTRATO, no de un literal repetido acá: el agente y el cerebro
// tenían el mismo valor declarado dos veces, y el porqué de por qué eso era grave —y asimétrico—
// está en internal/fleet/protocolo.go.
const prefijoRespuestaPermiso = fleet.PrefijoRespuestaPermiso

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
