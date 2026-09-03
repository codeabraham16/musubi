package mcp

// consentimiento_flota.go es LA TABLA DEL EJE DE CONSENTIMIENTO, en un solo lugar (A75).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL EJE EXISTÍA COMPLETO Y GATEABA UNA SOLA PUERTA DE LAS TRES
//
// `internal/fleet/consentimiento.go` tiene los cuatro grados, la acumulación por el más
// restrictivo y la degradación de `pide` cuando no hay a quién preguntarle. Lo consultaba
// EXCLUSIVAMENTE el camino de pantalla. O sea: en una máquina marcada `pide`, mirar la pantalla
// preguntaba, y abrir una TERMINAL en la misma máquina no preguntaba nada. Una terminal es más
// invasiva que mirar, no menos — quien tiene un prompt puede leer los mismos archivos, y además
// escribirlos.
//
// El agujero no era de la tabla: era de las llamadas que faltaban. Y esa forma —una regla bien
// escrita que la mitad de las superficies no consulta— es cómo este repo se rompe siempre. Así
// que la tabla se aplica desde UNA función y las tres puertas la llaman: pantalla, shell y exec.
// Copiarla en cada tool habría dejado tres lugares donde recordar que `prohibido` gana, y la
// copia que se queda vieja es siempre la de la puerta que se usa menos.

import (
	"fmt"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// modalidadDeAcceso es lo ÚNICO que cambia entre las tres puertas: cómo se nombra lo que se está
// pidiendo. La decisión —abrir, avisar o negar— es la misma para las tres, y por eso no vive acá.
//
// Los textos son datos y no un `switch` sobre la modalidad, para que agregar una cuarta puerta
// —transferencia de archivos, que es la que MeshCentral tiene y Musubi todavía no— sea una
// variable más y no una rama nueva en el medio de la decisión.
type modalidadDeAcceso struct {
	// rechazo abre el mensaje de `prohibido`: «no se abre la pantalla de %q: …».
	rechazo string
	// aviso completa el texto que se le dibuja a quien está en la máquina:
	// «Musubi: %s <aviso> esta máquina.». Tiene que decir QUÉ está pasando, porque un aviso que
	// no distingue una pantalla de una terminal no le sirve para decidir nada a quien lo lee.
	aviso string
	// endurece explica POR QUÉ un `pide` terminó en `prohibido` en ESTA puerta, y es el campo que
	// evita el peor error de diagnóstico del eje: alguien mirando un «prohibido» sobre una máquina
	// cuya configuración dice `pide`, sin saber cuál de las dos razones fue. En pantalla y en
	// shell es que la máquina no tiene con qué preguntar; en exec es estructural.
	endurece string
}

// sinInterlocutor es el `endurece` de las dos puertas que SÍ saben preguntar cuando la máquina no
// tiene con qué. Es una sola frase compartida porque el diagnóstico es el mismo, y dos copias se
// habrían separado a la primera corrección.
const sinInterlocutor = "si además figura como que no puede preguntar, un `pide` se endurece a " +
	"prohibido a propósito — quien escribió `pide` pidió que nadie entre sin permiso, y si el " +
	"permiso no se puede pedir, no se entra."

var (
	accesoPantalla = modalidadDeAcceso{
		rechazo:  "no se abre la pantalla de",
		aviso:    "está abriendo una sesión de pantalla en",
		endurece: sinInterlocutor,
	}
	accesoShell = modalidadDeAcceso{
		rechazo:  "no se abre una shell en",
		aviso:    "está abriendo una terminal en",
		endurece: sinInterlocutor,
	}
	accesoExec = modalidadDeAcceso{
		rechazo: "no se ejecuta en",
		aviso:   "está ejecutando un comando en",
		// EN exec, UN `pide` SIEMPRE TERMINA EN `prohibido`, y hay que decírselo a quien lo lee:
		// si no, va a buscar durante media hora la capacidad de preguntar de una máquina que
		// quizás la tenga perfectamente. Ver consentimientoParaExec.
		endurece: "en un exec, un `pide` SIEMPRE se endurece a prohibido: un comando es de una " +
			"sola vez y no hay un «después» donde volver a buscar el sí, así que no hay a quién " +
			"preguntarle. `pide` es la política de un escritorio con alguien trabajando enfrente; " +
			"LOS SERVIDORES VAN EN `libre`, y un servidor headless en `pide` deja de aceptar exec " +
			"por diseño. Se arregla con musubi_fleet_consent, que además devuelve el grado EFECTIVO.",
	}
)

// aplicarConsentimiento aplica la tabla del eje: niega, avisa, o deja seguir.
//
// SE LLAMA DESPUÉS DE LA COMPUERTA DE CAPACIDADES Y ANTES DE ENCOLAR NADA, y las dos mitades de
// esa frase son decisiones:
//
//   - DESPUÉS de la capacidad, porque quien no puede tocar una máquina no tiene por qué
//     enterarse de su política de consentimiento —ni de que existe—;
//   - ANTES de encolar, porque el daño de mirar esto tarde no es fallar: es haber puesto ya un
//     comando en la cola de una máquina donde el dueño dijo que no se entra.
//
// `pide` NO SE RESUELVE ACÁ, y es deliberado: cada modalidad lo resuelve distinto. La pantalla y
// la shell parten la apertura en dos pedidos (preguntar, y volver a buscar el sí); el exec no
// tiene un «después» donde volver, así que lo resuelve endureciéndolo antes de llegar hasta acá.
// Meter las tres formas en esta función la volvería un `switch` sobre quién la llamó.
//
// Devuelve un error sólo cuando el acceso está BLOQUEADO. El resto de los grados no cortan nada:
// avisar es una obligación con quien usa la máquina, no un permiso que se pide.
func (s *McpServer) aplicarConsentimiento(d fleet.Device, p *Principal,
	consent fleet.Consentimiento, m modalidadDeAcceso) *RpcError {

	switch {
	case consent.Bloquea():
		// Es un eje SEPARADO del permiso, así que el error lo dice con todas las letras: la
		// capacidad puede estar perfectamente concedida y el acceso igual no abrirse.
		// Confundirlos manda a alguien a revisar `principals.yaml` buscando un permiso que ya
		// está.
		return rpcErrorf(codeUnauthorized,
			"%s %q: %v. El grado configurado en esta máquina es %q; %s",
			m.rechazo, d.Name, fleet.ErrConsentimientoProhibido, d.Consentimiento, m.endurece)
	case consent.AvisaAlUsuario() && !d.PuedePreguntar:
		// SE ABRE, Y SE DICE QUE EL AVISO NO SE PUDO ENTREGAR. Prometer una notificación que el
		// agente de ESTA máquina no sabe dar sería exactamente lo que este eje viene a evitar:
		// una configuración que se ve puesta y no lo está. Bloquear tampoco: `avisa` no bloquea,
		// y hacerlo cerraría el acceso por una capacidad que esa máquina puede no tener nunca
		// —un servidor sin escritorio— por razones que no son de seguridad.
		s.avisarUnaVezPorDevice(d.ID, d.Name, consent, m)
	case consent.AvisaAlUsuario():
		// EL AGENTE SABE AVISAR: se le encola el aviso. Se hace ACÁ y no después de abrir, y el
		// orden importa: el aviso dice «alguien está por entrar», y entregarlo después de que la
		// terminal ya está abierta lo convierte en la noticia de algo que ya pasó. El agente lo
		// recoge en su próximo latido —hasta 30 s— y esa demora es el precio de no ponerlo a
		// escuchar un puerto, que es la superficie que este track evita desde S2.
		s.encolarAvisoAlUsuario(d, p, m)
	}
	return nil
}

// consentimientoParaExec es el grado que rige un exec, y la línea del medio es toda la decisión.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EN UN exec, `pide` SE ENDURECE A `prohibido` PORQUE NO HAY DÓNDE VOLVER
//
// Un `pide` se resuelve en DOS pedidos: el primero pregunta y devuelve una espera, el segundo
// recoge el sí. Eso funciona para una pantalla y para una shell porque lo que se concede es una
// SESIÓN —algo que dura y a lo que se vuelve—. Un exec es de una sola vez: partirlo en dos
// significaría dejar un comando esperando en la base con su argv adentro, que alguien tiene que
// volver a buscar, y con un permiso que vence antes que el comando. Eso no es un exec: es una
// política con otro nombre, y las políticas ya existen (S10) y encolan por otro camino.
//
// Así que se reusa la decisión que el dominio ya tomó para el caso «no hay a quién preguntarle»
// (fleet.Consentimiento.AplicarACapacidadDePreguntar): degrada a `prohibido` y no a `libre`. La
// salida cómoda —correr igual, «nadie puede negarse, entonces adelante»— es exactamente al revés
// de lo que alguien quiso decir al escribir `pide`.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA CONSECUENCIA, DICHA: UN SERVIDOR HEADLESS EN `pide` DEJA DE ACEPTAR exec
//
// Y es a propósito. `pide` es la política de un ESCRITORIO con una persona real trabajando, no
// la de un servidor: LOS SERVIDORES VAN EN `libre`, que es el grado que dice «no hay nadie
// enfrente a quien deberle un aviso». Poner `pide` en un servidor ya cerraba su pantalla —el
// agente no tiene dónde dibujar el diálogo, así que ConsentimientoEfectivo lo endurecía solo— y
// desde A75 también cierra su exec. Es un error de configuración VISIBLE, que es la clase buena:
// se arregla poniendo el grado que corresponde con `musubi_fleet_consent`, y la respuesta de esa
// tool dice el grado EFECTIVO justamente para que se vea al configurarlo y no cuando algo no anda.
func consentimientoParaExec(d fleet.Device) fleet.Consentimiento {
	return d.ConsentimientoEfectivo().AplicarACapacidadDePreguntar(false)
}

// avisarUnaVezPorDevice deja constancia de que se debía un aviso a quien usa la máquina y no se
// pudo entregar, porque su agente todavía no sabe notificar.
//
// UNA VEZ POR MÁQUINA, POR MODALIDAD Y POR VIDA DEL PROCESO. La modalidad entra en la clave
// porque las tres se arreglan igual pero no dicen lo mismo: enterarse de que las pantallas de una
// máquina se abren sin aviso no dice nada sobre sus terminales, y con una clave sola la segunda
// se perdería para siempre detrás de la primera.
//
// Esto NO reemplaza al aviso: cuando el agente sepa notificar, este camino desaparece y la
// notificación viaja de verdad. Mientras tanto, la ausencia se ve en el log del cerebro en vez de
// ser silenciosa — que es la diferencia entre una función a medio hacer y una que miente.
func (s *McpServer) avisarUnaVezPorDevice(deviceID, nombre string, c fleet.Consentimiento, m modalidadDeAcceso) {
	// Se reusa `avisosDados`, que es exactamente para esto: un aviso de CONFIGURACIÓN que no es
	// un evento sino un ESTADO. La clave lleva prefijo para no chocar con los del empuje.
	clave := "consentimiento_sin_aviso\x00" + m.aviso + "\x00" + deviceID
	if _, ya := s.avisosDados.LoadOrStore(clave, true); ya {
		return
	}
	logx.Warn("flota: se entró a una máquina y el aviso al usuario NO se pudo entregar",
		"device", nombre, "consentimiento", string(c), "modalidad", m.aviso,
		"motivo", "el agente de esta máquina no declara saber notificar (devices.puede_preguntar = 0)")
}

// encolarAvisoAlUsuario le manda al agente el aviso que `avisa` promete.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL TEXTO NOMBRA A QUIEN ENTRA Y QUÉ VIENE A HACER, Y ESO ES EL AVISO
//
// «Alguien está usando tu máquina» no le sirve a nadie. Lo que convierte esto en información son
// dos cosas: QUIÉN —un aviso sin nombre no se puede accionar, no hay a quién preguntarle— y QUÉ,
// porque una pantalla mirada y una terminal abierta no son el mismo riesgo ni piden la misma
// reacción de quien está sentado ahí.
//
// El nombre del principal es entrada de configuración (sale de principals.yaml, no de la red),
// pero igual se acota: termina interpolado en un diálogo del escritorio de otra persona.
//
// BEST-EFFORT A PROPÓSITO. Si encolar falla, el acceso sigue: `avisa` NO bloquea —ése es el grado
// siguiente— y convertir un fallo de la cola en un acceso denegado le daría a `avisa` la
// semántica de `pide` sin que nadie lo decidiera. Lo que no puede pasar es que falle callado, y
// por eso queda la línea.
func (s *McpServer) encolarAvisoAlUsuario(d fleet.Device, p *Principal, m modalidadDeAcceso) {
	quien := nombrePrincipal(p)
	if quien == "" {
		quien = "un operador"
	}
	texto := fmt.Sprintf("Musubi: %s %s esta máquina.", fleet.RecortarRunas(quien, 64), m.aviso)
	// EL AVISO VIAJA COMO `musubi:avisar` PARA LAS TRES MODALIDADES, y eso deja UNA cosa sin
	// resolver que conviene decir acá antes de que la encuentre alguien: fleet.TipoDeArgv
	// clasifica esa operación como `canal_pantalla`, así que en la cronología estas filas piden
	// `screen:view` para verse, incluso cuando el aviso lo generó una shell o un exec.
	//
	// SE DEJA ASÍ A PROPÓSITO Y NO ES GRATIS. Separar la operación por modalidad
	// (`musubi:avisar-shell`) arreglaría la clasificación y rompería algo peor: el agente ya
	// desplegado sólo entiende `musubi:avisar`, así que toda la flota que no se actualice dejaría
	// de entregar el aviso — y dejaría de entregarlo EN SILENCIO, que es el modo de fallo que
	// este eje entero viene a cerrar. La clasificación se arregla el día que el argv pueda llevar
	// la modalidad sin que el texto la absorba.
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: quien,
		Origen:  fleet.OrigenPersona,
		Argv:    []string{comandoAviso, texto},
		Timeout: fleet.ComandoTimeoutDefault,
	}); err != nil {
		logx.Warn("flota: no se pudo encolar el aviso al usuario; el acceso sigue igual",
			"device", d.Name, "modalidad", m.aviso, "error", err)
	}
}
