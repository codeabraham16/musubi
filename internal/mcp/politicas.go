package mcp

// politicas.go es AUTO-HEAL: ejecución remota sin una persona detrás. Track «Control de flota», S10.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ES LO MÁS PELIGROSO DEL TRACK ENTERO, Y LO QUE LO HACE DEFENDIBLE CABE EN UNA FRASE:
//
//	UNA POLÍTICA NO TIENE AUTORIDAD PROPIA.
//
// Nombra un principal de principals.yaml y actúa con la suya: la misma compuerta de tres lados de
// S3, la misma allowlist de S10, la misma bitácora que las personas. No hay un segundo camino a
// la ejecución remota — hay el mismo camino, recorrido por un temporizador en vez de por alguien.
//
// La alternativa (un daemon que ejecuta «porque es el daemon») habría sido más corta de escribir
// y sería exactamente el puente de privilegio que el track viene esquivando desde el proposal:
// bastaría con poder editar el archivo de configuración del cerebro para tener root en 40
// máquinas, sin figurar en ninguna concesión y sin dejar un nombre en la auditoría.
//
// Y una consecuencia que conviene ver de frente: si alguien REVOCA al principal de una política,
// la política se apaga sola en el próximo tick. No hay que acordarse de apagarla en dos lugares.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// aplicarPoliticas evalúa todas las políticas contra todas las máquinas de un proyecto y devuelve
// cuántas acciones se dispararon.
func (s *McpServer) aplicarPoliticas(proyecto string, ahora time.Time) int {
	if len(s.politicas) == 0 {
		return 0 // I15: sin sección, no existe
	}
	// Se RE-LEE la lista después del sondeo: las muestras que acaban de entrar son justamente las
	// que hay que juzgar. Evaluar sobre la lista previa al barrido significaría reaccionar siempre
	// con un tick de atraso, y con un tick de atraso el cooldown y la condición se desincronizan.
	devices, err := s.engine.ListarDevices(proyecto, false)
	if err != nil {
		logx.Error("políticas: no se pudieron listar los dispositivos", "proyecto", proyecto, "error", err)
		return 0
	}
	// LA VENTANA DE MANTENIMIENTO FRENA LAS POLÍTICAS, Y ES LA MITAD QUE UN SILENCE NO PUEDE
	// (Ola 1 del plan empresa).
	//
	// Un `amtool silence` calla el aviso y no toca esto: las políticas no leen alertas, leen la
	// muestra y actúan solas. Sin esta guarda, un reinicio planificado de postgres dispara
	// `servicio_caido`, el auto-heal lo levanta EN MITAD DEL MANTENIMIENTO, y el silence sólo
	// garantiza que nadie se entere — la automatización actuando con el canal que lo contaría
	// apagado.
	//
	// Se consulta UNA vez por proyecto y no una por (política × máquina): son las mismas ventanas
	// para todas las políticas del barrido, y preguntarlo adentro del bucle sería una consulta por
	// combinación.
	//
	// La lectura, y qué se hace si falla, es ventanasParaPoliticas: la MISMA que usa el inventario
	// para publicar `inerte_por: "mantenimiento"` (A131).
	enMantenimiento := s.ventanasParaPoliticas(ahora, "proyecto", proyecto)

	acciones := 0
	for _, pol := range s.politicas {
		for _, d := range devices {
			if enMantenimiento[d.ID] {
				// Se CUENTA, con resultado propio. Un salteo silencioso se ve igual que una
				// política que nunca tuvo que actuar, y la pregunta «¿el auto-heal no actuó
				// porque no hizo falta, o porque estaba en mantenimiento?» se contesta acá o no
				// se contesta.
				s.metrics.contarPolitica(pol.Nombre, "mantenimiento")
				continue
			}
			if s.evaluarPolitica(pol, d, ahora) {
				acciones++
			}
		}
	}
	return acciones
}

// ventanasParaPoliticas dice qué máquinas están AHORA en una ventana de mantenimiento, para decidir
// políticas. La leen los dos lados de la decisión: el barrido que actúa (aplicarPoliticas) y el
// inventario que contesta si actuaría (toolFleetList → politicasSobre → porQueNoActuaria).
//
// Si la consulta falla NO se saltea nada: un error leyendo las ventanas no puede convertirse en
// «no hay mantenimiento» (dispararía en medio de uno) ni en «hay mantenimiento en todas»
// (apagaría el auto-heal de la flota entera). Se sigue con el comportamiento de siempre y se dice,
// que es el sesgo con el que ya se equivocaba antes de que esto existiera.
//
// ES UNA SOLA FUNCIÓN PARA QUE EL SESGO SEA UNO SOLO. Si el inventario leyera las ventanas por su
// cuenta y ante el error eligiera el otro lado —«en la duda, inerte», que suena prudente—, el día
// que la base no contesta diría `puede_actuar: false` de una política que el barrido está
// ejecutando: la misma contradicción entre indicador y acción que A131 vino a cerrar, sólo que
// escondida en la rama de error, que es la que nadie prueba.
func (s *McpServer) ventanasParaPoliticas(ahora time.Time, contexto ...any) map[string]bool {
	en, err := s.engine.DevicesEnMantenimiento(ahora)
	if err != nil {
		logx.Error("políticas: no se pudieron leer las ventanas de mantenimiento; se evalúa como si no hubiera ninguna",
			append(contexto, "error", err)...)
		return nil
	}
	return en
}

// evaluarPolitica decide y, si corresponde, actúa sobre UNA máquina. Devuelve si actuó.
//
// El orden de las guardas es de más barato a más caro, pero sobre todo es de más específico a más
// general: primero lo que descarta la mayoría sin tocar nada.
func (s *McpServer) evaluarPolitica(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {
	if !pol.Alcanza(d.Name) {
		return false
	}
	if d.Revoked {
		return false
	}
	// LAS POLÍTICAS DE SERVICIO SE DECIDEN POR OTRO CAMINO, Y SE BIFURCA ACÁ ARRIBA.
	//
	// Las guardas de abajo son de la MUESTRA del host: que exista, que la máquina esté en línea,
	// que no sea rancia. Una política de servicio no las necesita —no mira la muestra— y
	// aplicárselas la volvería inútil justo cuando más importa: una máquina cuyo colector murió
	// sigue reportando su inventario de servicios, y ahí es donde uno quiere que la política actúe.
	//
	// Lo que sí comparte es TODO lo de después: el cooldown, la compuerta del principal, la
	// allowlist y la bitácora. Bifurcar sólo la decisión y no la acción es lo que impide que el
	// plano de actuar tenga dos caminos con reglas distintas.
	if pol.EsDeServicio() {
		return s.evaluarPoliticaDeServicio(pol, d, ahora)
	}

	// I13 — NO SE ACTÚA SOBRE UNA MUESTRA RANCIA.
	//
	// Es la guarda que más veces va a evitar un desastre y la más fácil de olvidar. Si la máquina
	// dejó de reportar, el último dato que tenemos puede ser de hace horas: el disco pudo haberse
	// vaciado hace veinte minutos, o el proceso que consumía la RAM pudo haber muerto. Actuar con
	// eso no es reaccionar tarde — es reaccionar a algo que ya no está pasando.
	//
	// El umbral es el MISMO que decide «en línea», y por tier (I2): si la máquina figura caída, su
	// muestra es rancia por definición.
	umbral := s.umbralEnLinea(d)
	if d.UltimaMuestra == nil || !d.EnLinea(ahora, umbral) {
		return false
	}
	if ahora.Sub(d.UltimaMuestra.Tomada) > umbral {
		// Late pero dejó de MEDIR: el agente vive y el colector murió. Es el fallo silencioso que
		// `up` no detecta, y una política que no lo mirara actuaría eternamente sobre la última
		// muestra buena — que, siendo la última buena, siempre cruza el umbral.
		return false
	}
	valor, dispara := pol.Dispara(d.UltimaMuestra)
	if !dispara {
		return false
	}
	// El valor de una métrica del host SIEMPRE se midió si la política disparó: `Dispara` no
	// devuelve true sin número. Se envuelve para compartir la firma con el camino de servicios,
	// donde el «no medido» sí existe.
	medido := valor
	// I14 — cooldown por (política × máquina × alcance), contado desde el DISPARO y no desde el
	// resultado: lo que hay que espaciar es la decisión de actuar, y el comando puede tardar.
	return s.actuarSiCorresponde(pol, d, &medido, ahora)
}

// evaluarPoliticaDeServicio decide una política que mira un SERVICIO adentro de la máquina.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// NO PIDE QUE LA MÁQUINA ESTÉ EN LÍNEA, Y ESA AUSENCIA ES DELIBERADA
//
// Las guardas de la muestra —que exista, que la máquina lata, que el dato no sea rancio— son de
// la TELEMETRÍA. Un servicio no se mide con la muestra: se mide con el inventario, que tiene su
// propia frescura y su propio umbral. Copiar las guardas de la muestra acá volvería la política
// inútil justo donde más sirve — una máquina cuyo colector murió sigue mandando su inventario, y
// ahí es donde uno quiere que algo actúe.
//
// La frescura del inventario SÍ se exige, y la decide el dominio: ver DisparaSobreServicio.
func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {
	servicios, err := s.engine.ServiciosDeDevice(d.ID)
	if err != nil {
		// No se puede leer el inventario: no se actúa. Es la misma regla que la muestra ausente —
		// no saber no es una razón para tocar una máquina.
		return false
	}
	umbral := fleet.UmbralInventario
	for _, sv := range servicios {
		if sv.Nombre != pol.Servicio {
			continue
		}
		fresco := !sv.UltimoReporte.IsZero() && ahora.Sub(sv.UltimoReporte) <= umbral
		valor, dispara := pol.DisparaSobreServicio(sv, fresco)
		if !dispara {
			return false
		}
		v := 0.0
		if valor != nil {
			v = *valor
		}
		return s.actuarSiCorresponde(pol, d, &v, ahora)
	}
	// EL SERVICIO NO ESTÁ EN EL INVENTARIO DE ESA MÁQUINA, y eso NO dispara. Podría tentar
	// tratarlo como «caído» —no está, algo pasó— pero es exactamente al revés: la ausencia
	// significa que la máquina no lo enumera, no que se cayó. Una política que reinicia lo que no
	// existe es la que se lleva puesto un host donde alguien escribió mal el nombre.
	return false
}

// actuarSiCorresponde es TODO lo que las dos clases de política comparten: el cooldown, la
// compuerta del principal, la allowlist, la ejecución y la bitácora.
//
// Está separado para que bifurcar la DECISIÓN no bifurque la ACCIÓN. Dos caminos con reglas
// distintas para ejecutar es cómo una de las dos se queda sin una guarda el día que se toca la
// otra — y acá las guardas son las que impiden que el cerebro corra un comando que nadie autorizó.
func (s *McpServer) actuarSiCorresponde(pol fleet.Politica, d fleet.Device, valor *float64, ahora time.Time) bool {
	// I14 — cooldown por (política × máquina), contado desde el DISPARO y no desde el resultado:
	// lo que hay que espaciar es la decisión de actuar, y el comando puede tardar.
	clave := pol.ClaveDeCooldown(d.ID)
	if previo, hay := s.ultimoDisparo.Load(clave); hay {
		if t, ok := previo.(time.Time); ok && ahora.Sub(t) < pol.CooldownEfectivo() {
			return false
		}
	}

	// ── QUÉ COMPUERTAS HAY SE DECIDE EN UNA SOLA FUNCIÓN, Y EL INVENTARIO LLAMA A LA MISMA (A131) ──
	//
	// Acá queda sólo lo que la decisión PRODUCE: los avisos, las métricas y el aviso al usuario de
	// la máquina. Qué principal, qué compuertas y en qué orden lo contesta autoridadDePolitica, que
	// es también lo que lee `puede_actuar` en el inventario. Hasta A131 el indicador era una copia
	// a mano de esta cadena: A91 le agregó aquí la tercera compuerta y la copia se quedó con dos,
	// así que una máquina en `pide` o `prohibido` figuraba «puede actuar» y la política no actuaba
	// nunca. Ver el doc de autoridadDePolitica.
	pr, freno, consent := s.autoridadDePolitica(pol, d)
	if freno == frenoSinRegistro {
		// Sin registro de principals no hay a quién nombrar. Con políticas configuradas el
		// arranque ya lo rechaza (vincularRegistroDeFlota); acá sólo se cierra el camino.
		return false
	}

	// LOS AVISOS SON ESTADOS, NO EVENTOS: cada uno se da UNA vez mientras su compuerta sea la que
	// frena, y se rearma en cuanto deja de serlo. Un WARN por tick son 288 líneas idénticas por
	// día, que es exactamente cómo se entierra la línea que sí importa — el mismo criterio con el
	// que el resto del scheduler sólo se anuncia cuando hubo trabajo. Lo destapó el e2e: 17 avisos
	// idénticos en un minuto.
	//
	// Van con avisoMientras y no con avisarUnaVez + un Delete suelto por la misma razón que la
	// escribió: el rearme en una rama aparte se puede borrar sin que nada se ponga rojo.
	par := pol.Nombre + "\x00" + d.ID

	// SIN PRINCIPAL: se revocó, venció o se le cambió el nombre entre el arranque y ahora. Se dice
	// fuerte: una política que dejó de poder actuar es una alarma apagada, y las alarmas apagadas
	// en silencio son la razón por la que existe la mitad de este slice. La clave es por POLÍTICA
	// y no por máquina: el principal falta para todas a la vez.
	//
	// El MOTIVO se distingue: «revocado» y «vencido» apagan la política igual, pero mandan a dos
	// lugares distintos a arreglarla. Decir «ya no está en principals.yaml» de alguien que está
	// ahí escrito manda a buscar donde no está el problema — por eso el segundo lookup es el de
	// diagnóstico, que sí ve la credencial muerta.
	s.avisoMientras("sin_principal:"+pol.Nombre, freno == frenoSinPrincipal, func() {
		nota := "el principal ya no está en principals.yaml; la política quedó inerte"
		if p, hay := s.buscarPrincipal.porNombreAunqueVencida(pol.Principal); hay && p.Vencida(ahoraParaVencimiento()) {
			nota = "la credencial VENCIÓ el " + p.Expires.UTC().Format(time.RFC3339) +
				"; sigue escrita en principals.yaml pero ya no ejecuta nada: renovále el `expires:`"
		}
		logx.Warn("política sin principal: no actúa (no se repite este aviso hasta que se resuelva)",
			"politica", pol.Nombre, "principal", pol.Principal, "device", d.Name,
			"nota", nota)
	})
	// Por máquina: un rechazo de compuerta también es un estado que dura hasta que alguien edite
	// el registro o la máquina, no un evento. El texto separa los dos lados que PuedeSobreDevice
	// junta, porque mandan a arreglar en lugares distintos: el alta de la máquina o principals.yaml.
	s.avisoMientras("compuerta:"+par, freno == frenoSinExec, func() {
		porque := "el principal no tiene `exec` sobre esa máquina (su proyecto o su concesión no la alcanzan)"
		if !d.Permite(fleet.CapExec) {
			porque = "la máquina no admite `exec` (su tier o sus caps no lo incluyen, o está revocada)"
		}
		logx.Warn("política rechazada por la compuerta: "+porque,
			"politica", pol.Nombre, "principal", pol.Principal, "device", d.Name)
	})
	s.avisoMientras("allowlist:"+par, freno == frenoAllowlist, func() {
		permitidos, _ := comandosPermitidos(pr, d)
		logx.Warn("política rechazada por la allowlist del principal",
			"politica", pol.Nombre, "principal", pol.Principal, "device", d.Name,
			"pidio", pol.Hacer[0], "permitidos", permitidos)
	})
	// El candado del dueño es su decisión y no una falla, así que se dice UNA vez: dura hasta que
	// alguien cambie el grado o saque la máquina de la política. `pide` y `prohibido` comparten la
	// clave porque son el mismo eje; lo que cambia es qué hacer para destrabarla.
	s.avisoMientras("consentimiento:"+par, freno == frenoConsentimientoProhibido || freno == frenoConsentimientoPide, func() {
		if freno == frenoConsentimientoPide {
			logx.Warn("política frenada porque la máquina exige que su usuario ACEPTE, y un barrido no puede esperar",
				"politica", pol.Nombre, "device", d.Name, "grado", string(consent),
				"nota", "mismo criterio que A86 para `exec`; si esta máquina tiene que automatizarse, "+
					"bajala a `avisa`, que sí notifica y no bloquea")
			return
		}
		logx.Warn("política frenada por el consentimiento de la máquina: no actúa",
			"politica", pol.Nombre, "device", d.Name, "grado", string(consent),
			"nota", "es la decisión del dueño de esa máquina, no un error de configuración; "+
				"para automatizarla hay que bajarle el grado o sacarla del alcance de la política")
	})

	// LA SEÑAL, EN CAMBIO, SE CUENTA SIEMPRE: de ella viven PoliticaSinPermiso y
	// PoliticaFrenadaPorConsentimiento. Lo que se acota es el ruido, no la señal.
	//
	// Los resultados van LITERALES en cada rama, y no derivados del freno, a propósito:
	// TestSeSiembranTodosLosResultadosQueSeEmiten lee del AST los literales que recibe
	// `contarPolitica` para exigir que cada uno esté sembrado, y con una variable no podría.
	switch freno {
	case sinFreno:
	case frenoSinPrincipal:
		s.metrics.contarPolitica(pol.Nombre, "sin_principal")
		return false
	case frenoSinExec, frenoAllowlist:
		s.metrics.contarPolitica(pol.Nombre, "rechazada")
		return false
	case frenoConsentimientoProhibido:
		s.metrics.contarPolitica(pol.Nombre, "consentimiento_prohibido")
		return false
	case frenoConsentimientoPide:
		// Se cuenta aparte de `prohibido` justamente para poder medir cuánto costaría la puerta
		// que queda abierta (ver autoridadDePolitica).
		s.metrics.contarPolitica(pol.Nombre, "consentimiento_pide")
		return false
	default:
		// UN FRENO QUE ESTE SWITCH NO SABE NOMBRAR FRENA IGUAL. Si autoridadDePolitica gana una
		// compuerta y nadie la agrega acá, la política tiene que quedarse quieta y decirlo, no
		// caer al final del switch y actuar: el lado seguro de «no sé qué me frenó» es no hacer.
		logx.Error("política frenada por una compuerta que actuarSiCorresponde no sabe nombrar: no actúa",
			"politica", pol.Nombre, "device", d.Name, "freno", string(freno))
		s.metrics.contarPolitica(pol.Nombre, "rechazada")
		return false
	}

	// Y ACÁ SE REUSA EL ENCOLADOR ÚNICO, que es todo el punto de A83: había DOS copias del bloque
	// de aviso, se agregó un tercer camino, nadie se acordó de copiarlo, y el eje quedó escrito y
	// sin efecto. Escribir una cuarta copia acá habría dejado la causa intacta para el quinto
	// camino. Se agrega una frase a `avisoPolitica` y se llama a la función.
	//
	// `AvisaAlUsuario()` es true para `pide` también (es `nivel >= avisa`), y eso ya no puede
	// mandar un aviso y ejecutar igual: `pide` salió arriba con su freno, así que acá sólo llegan
	// `libre` y `avisa`.
	if consent.AvisaAlUsuario() {
		s.encolarAvisoDeAcceso(d, pr, avisoPolitica)
	}

	// El cooldown se marca ANTES de ejecutar. Si se marcara después, un comando lento (o un
	// cerebro que se cae a mitad) dejaría la puerta abierta para que el próximo tick dispare otra
	// vez: la tormenta que el cooldown viene a evitar empieza justo cuando algo va mal.
	s.ultimoDisparo.Store(clave, ahora)
	// ...y HACIA EL DISCO, para que sobreviva un reinicio (A24). El mapa en memoria sigue siendo
	// el camino caliente —una lectura por par y por tick contra la base serían 200 consultas para
	// un dato que sólo cambia cuando algo dispara—, y la tabla es su respaldo durable.
	//
	// Un fallo al persistir NO cancela la acción: el comando ya está decidido y auditado, y el
	// costo del fallo es un cooldown que no sobrevive al próximo reinicio. Cancelar la acción por
	// no poder anotar el cooldown sería dejar el problema sin atender para proteger la anotación.
	if err := s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora); err != nil {
		logx.Error("política: no se pudo persistir el cooldown (sobrevive en memoria, no a un reinicio)",
			"politica", pol.Nombre, "device", d.Name, "error", err)
	}

	logx.Info("política dispara",
		"politica", pol.Nombre, "device", d.Name, "principal", pol.Principal,
		"condicion", pol.Umbral(), "medido", medidoParaLog(valor), "hacer", pol.Hacer)

	if err := s.correrAccionDePolitica(pol, pr, d, ahora); err != nil {
		logx.Error("política: la acción falló", "politica", pol.Nombre, "device", d.Name, "error", err)
		s.metrics.contarPolitica(pol.Nombre, "error")
		return false
	}
	s.metrics.contarPolitica(pol.Nombre, "ok")
	return true
}

// frenoDePolitica nombra la compuerta que frena a una política sobre una máquina.
//
// Es un conjunto CERRADO —las constantes de abajo— y viaja tal cual al inventario como
// `inerte_por`, así que quien mira el panel sabe cuál de las compuertas la dejó inerte en vez de
// leer una lista fija de causas posibles. La lista fija que había nombraba dos y ya eran tres.
type frenoDePolitica string

const (
	// sinFreno: ninguna compuerta frena. Si la condición se cumple, la política actúa.
	sinFreno frenoDePolitica = ""
	// frenoSinRegistro: no hay registro de principals, así que no hay a quién nombrar.
	frenoSinRegistro frenoDePolitica = "sin_registro"
	// frenoSinPrincipal: el principal no está en el registro vigente, o su credencial venció.
	frenoSinPrincipal frenoDePolitica = "sin_principal"
	// frenoSinExec: la compuerta de tres lados (tenencia, concesión, aparato) dice que no.
	frenoSinExec frenoDePolitica = "sin_exec"
	// frenoAllowlist: el comando de la política no pasa la allowlist de su principal.
	frenoAllowlist frenoDePolitica = "allowlist"
	// frenoConsentimientoProhibido: el candado del dueño de la máquina.
	frenoConsentimientoProhibido frenoDePolitica = "consentimiento_prohibido"
	// frenoConsentimientoPide: la máquina exige que su usuario acepte, y un barrido no puede esperar.
	frenoConsentimientoPide frenoDePolitica = "consentimiento_pide"
	// frenoMantenimiento: la máquina está en una ventana de mantenimiento. NO lo devuelve
	// autoridadDePolitica —no es autoridad sino una pausa, y el barrido la mira antes de llegar
	// ahí—; lo antepone porQueNoActuaria, en el mismo orden en que la frena aplicarPoliticas.
	frenoMantenimiento frenoDePolitica = "mantenimiento"
)

// autoridadDePolitica es LA cadena de AUTORIDAD de una política sobre una máquina, entera y en
// orden: el principal vigente, `exec` sobre esa máquina, la allowlist y el eje de consentimiento.
// Devuelve con qué principal actuaría, qué compuerta la frena (sinFreno si ninguna) y el grado
// efectivo de consentimiento, que quien actúa necesita para decidir si avisa.
//
// LO QUE NO ESTÁ ACÁ, Y POR QUÉ. La ventana de mantenimiento también frena a una política, pero no
// es autoridad: es una pausa que el barrido consulta UNA vez por proyecto, en aplicarPoliticas,
// antes de llegar a esta función (preguntarla acá sería una consulta por política × máquina). El
// inventario la antepone en el mismo orden en porQueNoActuaria, así que `puede_actuar` sí la ve.
// Lo que queda fuera de los dos es la CONDICIÓN y lo que la rodea —la muestra fresca, el
// cooldown—, porque el indicador contesta justamente «si la condición se cumpliera».
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ES UNA FUNCIÓN Y NO DOS COPIAS (A131)
//
// La decisión se consulta desde dos lados: actuarSiCorresponde, que actúa, y el `puede_actuar`
// del inventario, que le contesta a un operador «si la condición se cumpliera ahora, ¿pasaría
// algo?». El indicador se presentaba como «deliberadamente la MISMA cadena de guardas» y era una
// COPIA escrita a mano de dos compuertas. A91 le agregó a la acción la tercera —el eje de
// consentimiento— y la copia no se enteró: con la máquina en `pide` o en `prohibido`, el
// inventario decía `puede_actuar: true` y la política no actuaba nunca. Es la alarma apagada que
// A23 vino a cerrar, reabierta por el mismo mecanismo de siempre: la guarda en N−1 de N caminos.
//
// Medido en la auditoría A131: 0 de 1 pares (política × máquina) expuestos. La única política en
// producción es `vaciar-journal` sobre `musubi-server`, que no declara grado y resuelve a
// `avisa`. El defecto era latente y se volvía vivo con el primer `pide` o `prohibido`.
//
// Con una sola función, divergir deja de ser algo que se pueda escribir: una cuarta compuerta
// agregada acá aparece sola en el indicador. Lo que una sola función NO puede ver es un defecto
// adentro de ella, porque engaña a los dos lados por igual, y hay dos clases:
//
//   - una compuerta mal escrita (sin la tenencia, digamos) la caza la tabla de
//     TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice, que compara cada fila contra
//     un hecho escrito y no sólo contra el otro lado;
//   - un principal que ENVEJECE (una copia cacheada que sobrevive a la edición del registro) la
//     tabla NO la ve: arma un servidor nuevo por fila, así que ninguna copia llega a envejecer.
//     Lo caza TestCadaFormaDeQuitarleAutoridadAlPrincipalApagaLaPolitica, que sobre UN servidor
//     dispara, edita el registro y vuelve a disparar. Medido con la mutación P1-m5 portada acá:
//     la tabla queda en verde y esa prueba cae en los pasos 1, 3 y 5.
//
// NO TIENE EFECTOS: ni avisos, ni métricas, ni encolados. El inventario la llama por cada fila
// que dibuja, y un indicador que al consultarse dejara un aviso o sumara una métrica estaría
// contando miradas como si fueran disparos.
func (s *McpServer) autoridadDePolitica(pol fleet.Politica, d fleet.Device) (*Principal, frenoDePolitica, fleet.Consentimiento) {
	// El principal se resuelve AHORA, en cada evaluación, contra el snapshot vigente del registro
	// (que se recarga en caliente cada 10 s). Resolverlo una vez al arranque habría dejado a las
	// políticas actuando en nombre de credenciales ya revocadas. `porNombre` excluye a las
	// vencidas: una credencial muerta no actúa por este camino.
	if s.buscarPrincipal == nil {
		return nil, frenoSinRegistro, ""
	}
	pr, existe := s.buscarPrincipal.porNombre(pol.Principal)
	if !existe {
		return nil, frenoSinPrincipal, ""
	}

	// LAS TRES COMPUERTAS, LAS MISMAS QUE PARA UNA PERSONA. No hay atajo por ser automático.
	//
	// Eran DOS —capacidad y allowlist— y el comentario decía «las mismas que para una persona»
	// mientras una persona ya pasaba TRES. Esa tercera es el eje de consentimiento, y su ausencia
	// acá es A91: medido el 2026-09-05 corriendo el barrido real, una máquina en `prohibido`
	// recibía el comando igual, y bajo `avisa` no se encolaba ningún aviso. El eje se describe en
	// todas las superficies como «el candado del dueño de la máquina, que no se abre NUNCA» y
	// cerraba los tres caminos con una persona detrás y ninguno cuando la orden la dispara un
	// temporizador. Decisión de gio (2026-09-05): el eje GOBIERNA al auto-heal.
	//
	// La capacidad se pregunta ENTERA a PuedeSobreDevice y no por partes: sus tres lados
	// (tenencia, concesión, aparato) son necesarios, y una compuerta armada acá con dos de ellos
	// dejaría a la política poder más que su principal en el tercero.
	if !PuedeSobreDevice(pr, d, fleet.CapExec) {
		return pr, frenoSinExec, ""
	}
	if !argvPermitido(pr, d, pol.Hacer) {
		return pr, frenoAllowlist, ""
	}

	// ── TERCERA COMPUERTA · EL EJE DE CONSENTIMIENTO (A91) ─────────────────────────────────────
	//
	// EL ORDEN DEL SWITCH NO ES ESTÉTICO Y ES EL DEFECTO DE A83/A85: `AvisaAlUsuario()` es true
	// para `pide` TAMBIÉN —es `nivel >= avisa`—, así que preguntar por `avisa` primero manda un
	// aviso y ejecuta igual en una máquina que exigía una respuesta. `pide` va antes.
	//
	// `ConsentimientoEfectivo()` ya endurece `pide` a `prohibido` cuando la máquina no sabe
	// preguntar, así que acá `pide` sólo llega desde una máquina que SÍ podría — y aun así frena.
	switch consent := d.ConsentimientoEfectivo(); {
	case consent.Bloquea():
		// El candado del dueño. Es su decisión y no una falla.
		return pr, frenoConsentimientoProhibido, consent
	case consent == fleet.ConsentimientoPide:
		// MISMA RAZÓN QUE A86 EN `exec`, Y VALE MÁS ACÁ: `pide` promete que su usuario ACEPTE antes
		// de que pase algo, y un barrido por temporizador no tiene dónde esperar esa respuesta. Un
		// `exec` a mano al menos tiene una persona del otro lado que puede reintentar; una política
		// corre sola, así que actuar sería romper la promesa sin nadie que lo note.
		//
		// LA PUERTA QUE QUEDA ABIERTA, dicha para que no se descubra desplegando: se podría encolar
		// un `musubi:preguntar` y actuar en el tick siguiente si la respuesta llegó. Eso exige
		// guardar el estado de una aprobación por (política × máquina) y expirarlo, que es un
		// mecanismo entero y no una rama de un switch. No se hizo, y se cuenta aparte de
		// `prohibido` justamente para poder medir cuánto costaría.
		return pr, frenoConsentimientoPide, consent
	default:
		return pr, sinFreno, consent
	}
}

// correrAccionDePolitica encola (Tier A) o ejecuta (Tier B) la acción.
//
// I16 — VA A LA MISMA BITÁCORA QUE LAS PERSONAS, con el nombre del principal de la política. Un
// operador ve `auto-heal` en la misma tabla y con las mismas columnas que ve `gio`. Un segundo
// registro de auditoría «para lo automático» es cómo se llega a auditar sólo la mitad de lo que
// pasa — y la mitad automática es justo la que nadie miró ejecutarse.
func (s *McpServer) correrAccionDePolitica(pol fleet.Politica, pr *Principal, d fleet.Device, ahora time.Time) error {
	// EL BARRIDO TAMBIÉN ESCRIBE BAJO CANDADO, Y ANTES NO LO HACÍA NADIE POR ÉL.
	//
	// Mientras `musubi_fleet_exec` no declaraba `lockSelf`, el despacho serializaba las escrituras
	// de la tool y este camino —un temporizador— escribía por afuera. Ahora que la tool acota sus
	// propios tramos, dejar éste sin candado sería la guarda a medias: la mitad automática, que es
	// justo la que nadie mira ejecutarse, quedaría como el único escritor sin serializar.
	//
	// Es seguro: RunFlotaScheduler no toma dispatchMu (scheduler_flota.go:273).
	var cmd fleet.Comando
	var err error
	s.withWriteLock(func() {
		cmd, err = s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: d.ProjectID, Principal: pr.Name,
			// LO ÚNICO AUTOMÁTICO DE TODO EL CANAL. Va a la misma bitácora que una persona (I16) y
			// ahora además se puede DISTINGUIR al leer: cuarenta reinicios seguidos son un relato
			// distinto según si los pidió alguien o los disparó una regla.
			Origen: fleet.OrigenPolitica,
			Argv:   pol.Hacer, Timeout: fleet.ComandoTimeoutDefault,
		})
	})
	if err != nil {
		return err
	}
	// Tier B no tiene agente que levante la cola: el cerebro sale a ejecutar, igual que en la
	// tool. Se hace SÍNCRONO dentro del barrido —el timeout del comando lo acota— porque el
	// paralelismo del barrido ya está acotado y esto no es un camino de request.
	if d.Tier == fleet.TierProtocolo {
		return s.correrPorSSH(d, cmd, fleet.ComandoTimeoutDefault, ahora)
	}
	// Tier A: queda encolado y el agente lo levanta en su próximo latido. NO se espera el
	// resultado: el barrido no es una request y nadie está del otro lado esperando una respuesta.
	// El resultado aparece en la bitácora cuando llegue.
	return nil
}

// avisarUnaVez ejecuta `emitir` sólo la primera vez que se ve esa clave.
//
// Existe porque los fallos de configuración de una política NO son eventos: son ESTADOS que duran
// hasta que alguien edita un archivo. Repetirlos en cada tick los convierte en el ruido que
// entierra a la línea que sí importa — el mismo criterio con el que el resto del scheduler sólo
// se anuncia cuando hubo trabajo. La clave se borra cuando la condición se resuelve, así que una
// recaída vuelve a avisar.
func (s *McpServer) avisarUnaVez(clave string, emitir func()) {
	if _, yaEstaba := s.avisosDados.LoadOrStore(clave, true); yaEstaba {
		return
	}
	emitir()
}

// avisoMientras es avisarUnaVez CON SU REARME PEGADO, y existe porque el rearme se puede borrar.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL REARME NO PUEDE VIVIR EN UNA RAMA `else`
//
// La forma anterior era `if condicion { avisarUnaVez(k, ...) } else { avisosDados.Delete(k) }`,
// repetida en cinco lugares. Un saboteador borró las dos ramas `else` del empuje y TODA la suite
// quedó en verde: un techo que se cruza y se arregla dejaba su aviso mudo para siempre, así que
// el PRÓXIMO corte del mismo techo pasaba en silencio — y eso no se ve, porque lo que falta es
// una línea de log que nadie está esperando.
//
// Acá el rearme no es una rama que se pueda borrar: es la mitad `else` de una sola función, y
// borrarla se lleva puesto el aviso, que sí tiene guarda. Un solo lugar donde equivocarse en vez
// de uno por condición, que además es la forma del defecto «la guarda está en N-1 de N caminos».
//
// `emitir` puede ser nil cuando sólo interesa el rearme (un camino que ya salió por otro lado).
func (s *McpServer) avisoMientras(clave string, activo bool, emitir func()) {
	if !activo {
		s.avisosDados.Delete(clave)
		return
	}
	if emitir == nil {
		return
	}
	s.avisarUnaVez(clave, emitir)
}

// cargarCooldowns siembra el mapa en memoria con lo que haya en la base (A24).
//
// SIN ESTO, EL COOLDOWN ES UNA GARANTÍA QUE DURA LO QUE DURE EL PROCESO. Y el reinicio no es un
// evento raro que ocurra en momentos tranquilos: es lo primero que alguien hace cuando algo va
// mal, que es exactamente cuando las políticas están disparando. El caso concreto: la política
// vacía un journal, el operador reinicia el cerebro treinta segundos después para tocar otra
// cosa, y la política vuelve a vaciarlo porque la muestra vieja todavía cruza el umbral.
//
// Best-effort: si la base no contesta se arranca con los cooldowns vacíos y se dice. Negarse a
// arrancar por esto sería peor — el cerebro entero abajo para proteger un espaciado.
func (s *McpServer) cargarCooldowns() {
	if len(s.politicas) == 0 {
		return
	}
	porPolitica, err := s.engine.CooldownsDePoliticas()
	if err != nil {
		logx.Warn("políticas: no se pudo cargar el cooldown persistido; se arranca sin él (una política podría actuar antes de tiempo, una vez)", "error", err)
		return
	}
	// LA CLAVE SE REARMA CON LA MISMA FUNCIÓN DEL DOMINIO y no concatenando a mano. La base
	// devuelve `device_id\x00alcance`; si acá se compusiera la clave por separado, el día que
	// ClaveDeCooldown cambie de forma los cooldowns persistidos dejarían de encontrarse — en
	// silencio, y sólo después de un reinicio, que es cuando menos se mira.
	//
	// SE SIEMBRA SÓLO LO QUE CORRESPONDE A LA POLÍTICA TAL COMO ESTÁ CONFIGURADA HOY. Una fila
	// cuyo alcance ya no coincide (alguien le cambió el `servicio` a la política) se ignora, y
	// eso es correcto: es otra cosa la que se está vigilando ahora, y su enfriamiento no se
	// hereda del anterior.
	n := 0
	for _, pol := range s.politicas {
		esperada := strings.TrimSpace(pol.Servicio)
		for compuesta, cuando := range porPolitica[pol.Nombre] {
			deviceID, alcance, _ := strings.Cut(compuesta, "\x00")
			if alcance != esperada {
				continue
			}
			s.ultimoDisparo.Store(pol.ClaveDeCooldown(deviceID), cuando)
			n++
		}
	}
	if n > 0 {
		logx.Info("políticas: cooldowns recuperados del reinicio", "pares", n)
	}
}

// ── Lo que el inventario tiene que poder mostrar (S9b · A23) ────────────────────────────────

// politicasSobre describe las políticas que aplican a una máquina, para el inventario y el panel.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO ES PARTE DE LA SEGURIDAD Y NO DE LA COSMÉTICA
//
// S10 dejó al cerebro ejecutando comandos en máquinas ajenas sin una persona detrás, y eso NO SE
// VEÍA EN NINGÚN LADO salvo hurgando la bitácora después del hecho. Una máquina con auto-heal
// encima era indistinguible de una sin él. Quien mira el inventario tiene que poder saber que ahí
// hay algo que actúa solo, qué haría, y con la autoridad de quién.
//
// EL CAMPO QUE MÁS IMPORTA ES `puede_actuar`. Una política mal configurada —su principal perdió la
// concesión, el comando se cayó de la allowlist, o la máquina pasó a `pide` o `prohibido`— se ve
// EXACTAMENTE IGUAL que una que funciona: las dos figuran en la lista y ninguna hace nada visible
// hasta que la condición se cumple. Es una alarma apagada, y la única forma de que alguien lo
// note antes del incidente es decirlo acá. `inerte_por` dice cuál de esas compuertas es. Una
// máquina en una ventana de mantenimiento cuenta igual: la política no actúa mientras dure, y una
// ventana olvidada es exactamente una alarma apagada con el panel en verde.
//
// QUIÉN VE QUÉ. El detalle exige `exec` sobre esa máquina, la misma regla que la bitácora: saber
// qué comando corre en un servidor es casi tan revelador como poder correrlo. Pero el CONTEO se
// muestra a cualquiera que vea la máquina — que exista algo automático encima no es un secreto, y
// ocultarlo del todo dejaría a quien sólo tiene `metrics` viendo cambiar una máquina sin ninguna
// pista de por qué.
//
// `enMantenimiento` lo trae el llamador, leído con ventanasParaPoliticas UNA vez para todo el
// inventario y no una por máquina.
// ────────────────────────────────────────────────────────────────────────────────────────────
func (s *McpServer) politicasSobre(p *Principal, d fleet.Device, enMantenimiento bool) (detalle []map[string]interface{}, total int) {
	if len(s.politicas) == 0 {
		return nil, 0
	}
	verDetalle := PuedeSobreDevice(p, d, fleet.CapExec)
	for _, pol := range s.politicas {
		if !pol.Alcanza(d.Name) {
			continue
		}
		total++
		if !verDetalle {
			continue
		}
		// puede_actuar es la decisión REAL, evaluada ahora contra las ventanas de mantenimiento, el
		// registro vigente y el grado de la máquina: la misma función que decide en
		// actuarSiCorresponde, no una copia de sus compuertas. La copia que había se quedó con dos
		// de tres (A131). Lo único que no mira es la condición, que es lo que el campo supone.
		freno := s.porQueNoActuaria(pol, d, enMantenimiento)
		fila := map[string]interface{}{
			"nombre":       pol.Nombre,
			"principal":    pol.Principal,
			"condicion":    pol.Umbral(),
			"hacer":        pol.Hacer,
			"cooldown_min": int(pol.CooldownEfectivo().Minutes()),
			"puede_actuar": freno == sinFreno,
		}
		// inerte_por dice CUÁL compuerta la frena, y sólo viaja cuando alguna frena: un
		// `inerte_por: ""` en cada política sana sería ruido que entrena a ignorar la columna.
		if freno != sinFreno {
			fila["inerte_por"] = string(freno)
		}
		// El último disparo viaja como null cuando nunca actuó: «todavía no» y «actuó hace mucho»
		// son cosas distintas, y una fecha inventada las confundiría — el mismo criterio que
		// gobierna exit_code y los porcentajes en todo el track.
		fila["ultimo_disparo"] = nil
		if v, hay := s.ultimoDisparo.Load(pol.ClaveDeCooldown(d.ID)); hay {
			if t, ok := v.(time.Time); ok {
				fila["ultimo_disparo"] = t.UTC().Format(time.RFC3339)
			}
		}
		detalle = append(detalle, fila)
	}
	return detalle, total
}

// porQueNoActuaria contesta «si la condición se cumpliera ahora mismo, ¿pasaría algo?»: devuelve
// la compuerta que frenaría a la política sobre esa máquina, o sinFreno si ninguna. Es lo que el
// inventario publica como `puede_actuar` e `inerte_por`, y lo ÚNICO que lo calcula.
//
// Un indicador que dijera «sí» donde la política dice «no» sería peor que no tenerlo, porque
// enseñaría a confiar en él. Por eso no evalúa compuertas propias: antepone la ventana de
// mantenimiento —en el mismo orden que aplicarPoliticas, que la mira antes que nada— y el resto
// se lo pregunta a autoridadDePolitica, que es la que decide en actuarSiCorresponde.
//
// ERA UNA COPIA DE LAS COMPUERTAS Y SE QUEDÓ CON DOS DE TRES (A131). Decía ser «deliberadamente
// la MISMA cadena de guardas» que la acción, y A91 le agregó a la acción el eje de
// consentimiento sin tocarla. Ahora no hay cadena que copiar: la decisión es una sola función, y
// el indicador descarta lo que no necesita —el principal y el grado— de la misma respuesta.
//
// Y LA VENTANA ERA EL HERMANO QUE QUEDABA AFUERA: con una abierta y la condición cumplida, el
// inventario decía `puede_actuar: true` sin `inerte_por`, y el barrido contaba `mantenimiento` y
// no actuaba. Medido en la revisión de A131 con una sonda sobre esta rama. Exposición en
// producción: el contador `mantenimiento` de `vaciar-journal` dio 0 como máximo en 30 días
// (auditoría A131), o sea que ningún barrido encontró una máquina en ventana en ese lapso.
//
// Existía también politicaPuedeActuar, un envoltorio de una línea sobre ésta que después de A131
// sólo llamaban las pruebas; se borró y las pruebas preguntan acá, que es lo que corre.
func (s *McpServer) porQueNoActuaria(pol fleet.Politica, d fleet.Device, enMantenimiento bool) frenoDePolitica {
	if enMantenimiento {
		return frenoMantenimiento
	}
	_, freno, _ := s.autoridadDePolitica(pol, d)
	return freno
}

// medidoParaLog traduce el «no sé» del dominio al del log.
//
// Un nil no puede salir como 0 en la línea que deja constancia de por qué el cerebro ejecutó algo:
// «se reinició 0 veces» y «esta plataforma no sabe contarlos» explican la misma acción de dos
// maneras opuestas, y la línea del disparo es lo primero que alguien lee cuando pregunta por qué
// se tocó una máquina.
func medidoParaLog(v *float64) any {
	if v == nil {
		return "no medido"
	}
	return *v
}
