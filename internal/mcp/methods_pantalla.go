package mcp

// methods_pantalla.go es el plano visual visto desde las personas (S6).
//
// Musubi NO transporta video: eso va directo entre los dos clientes de RustDesk, P2P o por el
// relay. Lo que hace acá es lo que RustDesk no tiene — decidir QUIÉN puede mirar QUÉ pantalla, y
// dejarlo escrito.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// toolFleetScreen acuña una sesión de pantalla.
//
// La contraseña se genera acá, viaja a la máquina por el canal de comandos y se devuelve UNA
// vez. Musubi no la guarda en ningún lado (G1) — ver internal/memory/sesiones.go, donde no hay
// parámetro donde ponerla.
func (s *McpServer) toolFleetScreen(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Device     string `json:"device"`
		Project    string `json:"project"`
		MinutosTTL int    `json:"minutos"`
		// Motivo sólo se usa si esta máquina exige cuatro ojos: es lo que va a leer quien
		// apruebe. Se acepta siempre para no obligar a saber de antemano si la máquina está
		// marcada — descubrirlo al recibir un error sería un viaje de ida y vuelta de más.
		Motivo string `json:"motivo"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`: qué máquina querés mirar")
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}

	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	// Mismo trato que exec: no se distingue «no existe» de «no podés». La diferencia sería un
	// oráculo de qué máquinas hay en un proyecto que no ves.
	if !existe || !PuedeSobreDevice(p, d, fleet.CapScreen) {
		return nil, rpcErrorf(codeUnauthorized,
			"no podés abrir la pantalla de %q: o no existe en el proyecto %q, o tu credencial no tiene la capacidad `screen` sobre esa máquina. Recordá que un dispositivo de Tier B (por protocolo) NUNCA tiene pantalla: no es política, es que no hay framebuffer.", nombre, proyecto)
	}
	// A18 — UN TIER QUE PUEDE TENER PANTALLA Y UN MOTOR QUE LA ABRA SON DOS COSAS DISTINTAS.
	//
	// Va ANTES de «¿está latiendo?» a propósito: que no haya motor es permanente, y decir «no
	// está latiendo» mandaría a alguien a depurar una sonda que anda bien. Y va ANTES de acuñar
	// —sesión, contraseña y comando— porque el daño de este hueco no era fallar: era ENTREGAR una
	// contraseña de sesión, mostrarla la única vez que se muestra, y dejar en la bitácora que se
	// abrió una pantalla que nunca se iba a abrir. Ver fleet.MotorDePantalla.
	if _, hayMotor := fleet.MotorDePantalla(d.Tier); !hayMotor {
		return nil, rpcErrorf(codeInvalidParams,
			"no se abre la pantalla de %q: es un dispositivo de Tier %s y Musubi todavía no tiene motor para su pantalla. "+
				"La capacidad `screen` está bien concedida —un móvil TIENE framebuffer— pero el motor de Android es scrcpy sobre ADB, "+
				"que es distinto del de RustDesk y no está implementado (ver A18 en specs/control-de-flota/ABIERTO.md). "+
				"Se niega en vez de fallar callado: abrir igual te entregaría una contraseña de sesión, de un solo uso, para una sesión que nadie va a levantar.",
			nombre, d.Tier)
	}
	// EL CONSENTIMIENTO SE MIRA ACÁ, ANTES DE ACUÑAR NADA, y por el mismo motivo que el motor:
	// el daño de mirarlo tarde no es fallar, es ENTREGAR una contraseña de sesión —que se muestra
	// una sola vez— para una sesión que no se tenía que abrir.
	//
	// Va DESPUÉS de la capacidad y no antes: quien no tiene `screen` no puede enterarse de la
	// política de consentimiento de una máquina que no debería ni saber que existe.
	//
	// Es un eje SEPARADO del permiso, así que el error lo dice: la capacidad puede estar
	// perfectamente concedida y la sesión igual no abrirse. Confundirlos mandaría a alguien a
	// revisar `principals.yaml` buscando un permiso que ya está.
	// EL VETO DEL DUEÑO DE LA MÁQUINA VA SOLO Y VA PRIMERO.
	//
	// Estaba en la misma tabla que los avisos y se separó al entrar los cuatro ojos, porque las
	// dos mitades no van en el mismo lugar: `prohibido` no necesita a NADIE para decidir, así que
	// pedirle a un segundo operador que apruebe una sesión que igual no se va a abrir es hacerle
	// perder el tiempo y, de paso, contarle que alguien lo intentó. Los avisos, en cambio, tienen
	// que ir DESPUÉS de la aprobación: ver más abajo.
	if consent := d.ConsentimientoEfectivo(); consent.Bloquea() {
		return nil, rpcErrorf(codeUnauthorized,
			"no se abre la pantalla de %q: %v. "+
				"El grado configurado en esta máquina es %q; si además figura como que no puede preguntar, "+
				"un `pide` se endurece a prohibido a propósito — quien escribió `pide` pidió que nadie entre "+
				"sin permiso, y si el permiso no se puede pedir, no se entra.",
			nombre, fleet.ErrConsentimientoProhibido, d.Consentimiento)
	}

	if !d.EnLinea(time.Now(), s.umbralEnLinea(d)) {
		return nil, rpcErrorf(codeInvalidParams,
			"%q no está latiendo: no hay a quién entregarle la contraseña de sesión. Mirá su estado con musubi_fleet_list.", nombre)
	}

	// A13 — SI EL id DE PANTALLA ES AMBIGUO, NO SE ABRE LA SESIÓN.
	//
	// Ese id lo REPORTA la propia máquina en su latido: es entrada no confiable. Si dos máquinas
	// dicen ser la misma, conectarse es una moneda al aire, y hay dos formas de llegar ahí:
	//
	//   - alguien comprometió una máquina y declaró el id de otra, para que un operador abra la
	//     pantalla equivocada (la colisión es la FIRMA de ese ataque);
	//   - dos máquinas clonadas de la misma imagen — mucho más frecuente, igual de roto, y hasta
	//     hoy invisible.
	//
	// SE NIEGA en vez de avisar. La alternativa amable —abrir igual con una advertencia— entrega
	// una contraseña de sesión y manda a alguien a una pantalla que puede no ser la que cree, que
	// es exactamente el daño que hay que evitar. Y el arreglo (regenerar el id en la máquina
	// duplicada) es el que hace falta de todos modos.
	otras, fuera, err := s.engine.QuienMasDiceSer(d.ID, d.RustdeskID, proyecto)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if len(otras) > 0 || fuera > 0 {
		return nil, rpcErrorf(codeInvalidParams, "%s", explicarColision(d, otras, fuera))
	}

	ttl := fleet.NormalizarDuracion(time.Duration(args.MinutosTTL) * time.Minute)
	ahora := time.Now().UTC()

	// ════════════════════════════════════════════════════════════════════════════════════════
	// `pide`: SE PREGUNTA PRIMERO, Y ESO NO PUEDE SER UNA LLAMADA QUE BLOQUEA (A57)
	//
	// El latido va cada 30 s y el diálogo espera hasta 60: una respuesta tarda hasta minuto y
	// medio en volver. Bloquear acá dejaría al operador mirando una llamada colgada, y —peor—
	// pondría un timeout de red en el camino de una decisión humana, donde el vencimiento
	// significa otra cosa.
	//
	// Así que se parte en dos: este pedido crea la sesión ESPERANDO PERMISO, encola la pregunta
	// y devuelve el id SIN contraseña. El operador vuelve a llamar y recibe la contraseña si le
	// dijeron que sí. Si ya hay una espera en curso para esta máquina y este principal, no se
	// abre otra: se informa la que hay.
	// ════════════════════════════════════════════════════════════════════════════════════════
	// CUATRO OJOS VA ANTES QUE TODO LO QUE MOLESTA A UN HUMANO
	//
	// Es el tercer eje (ver internal/fleet/aprobacion.go) y su lugar en la fila no es un detalle
	// de estilo: si fuera después del aviso, la persona sentada en la máquina recibiría «alguien
	// está por entrar» y no entraría nadie durante media hora, porque la sesión todavía tiene que
	// esperar a un segundo operador. Un aviso de algo que no pasó enseña a ignorar los avisos.
	//
	// Y si fuera después del `pide`, se le gastaría el «sí» a esa persona en una sesión que puede
	// no abrirse nunca: un permiso que se concede y se tira.
	//
	// Va DESPUÉS de la colisión de id y del «¿está latiendo?» a propósito: ésas son máquinas
	// donde la sesión no se puede abrir por razones que ninguna aprobación arregla, y pedirle a
	// alguien que apruebe eso es hacerlo firmar algo que no sirve.
	if resp, rpcErr := s.puertaDeCuatroOjos(d, p, proyecto, fleet.CapScreen, args.Motivo, ahora); rpcErr != nil || resp != nil {
		return resp, rpcErr
	}

	// LOS AVISOS, RECIÉN ACÁ: ya sabemos que esta sesión se va a abrir o va a preguntar.
	switch consent := d.ConsentimientoEfectivo(); {
	case consent.AvisaAlUsuario() && !d.PuedePreguntar:
		// SE ABRE, Y SE DICE QUE EL AVISO NO SE PUDO ENTREGAR. Prometer una notificación que el
		// agente de ESTA máquina no sabe dar sería exactamente lo que este eje viene a evitar:
		// una configuración que se ve puesta y no lo está. Bloquear tampoco: `avisa` no bloquea,
		// y hacerlo cerraría el acceso por una capacidad que esa máquina puede no tener nunca
		// —un servidor sin escritorio— por razones que no son de seguridad.
		s.avisarUnaVezPorDevice(d.ID, nombre, "pantalla", consent)
	case consent.AvisaAlUsuario():
		// EL AGENTE SABE AVISAR: se le encola el aviso (A57). El aviso dice «alguien está por
		// entrar», así que entregarlo después de que la pantalla ya está abierta lo convertiría
		// en una notificación de algo que ya pasó. El agente lo recoge en su próximo latido
		// —hasta 30 s— y esa demora es el precio de no ponerlo a escuchar un puerto.
		s.encolarAvisoDeAcceso(d, p, avisoPantalla)
	}

	if consent := d.ConsentimientoEfectivo(); consent == fleet.ConsentimientoPide {
		return s.pedirPermisoParaPantalla(d, p, proyecto, ttl, ahora)
	}

	// G7 — la sesión se registra ANTES de acuñar nada. Que alguien haya INTENTADO mirar una
	// pantalla es información de auditoría tanto como que lo haya logrado.
	ses, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
		DeviceID: d.ID, ProjectID: proyecto, Principal: nombrePrincipal(p),
		Creada: ahora, Vence: ahora.Add(ttl),
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	return s.entregarPantalla(d, p, proyecto, ses, ttl)
}

// entregarPantalla acuña la contraseña y se la manda a la máquina.
//
// ESTÁ EXTRAÍDA Y NO DUPLICADA porque tiene DOS llamadores: el camino normal y el de `pide`
// cuando el usuario dijo que sí. Copiarla habría dejado dos lugares donde acuñar credenciales y
// dos donde recordar que el argv no puede llegar a la bitácora — y la copia que se queda vieja
// es siempre la del camino que se usa menos, que acá es justo el de mayor autoridad.
func (s *McpServer) entregarPantalla(d fleet.Device, p *Principal, proyecto string,
	ses fleet.SesionPantalla, ttl time.Duration) (interface{}, *RpcError) {
	// EL PERMISO DE CUATRO OJOS SE GASTA ACÁ, en el único lugar que acuña una credencial de
	// pantalla — y por la MISMA razón por la que esta función está extraída y no duplicada. La
	// puerta sólo comprobó; entre aquélla y este punto puede haber pasado un diálogo de `pide`
	// entero, que es donde la primera versión perdía la aprobación. Ver gastarAprobacion.
	if e := s.gastarAprobacion(d, p, fleet.CapScreen, time.Now().UTC()); e != nil {
		return nil, e
	}

	pass, err := fleet.NuevaPassPantalla()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	// La contraseña viaja a la máquina por el canal de comandos de S5, pero NO como un exec: es
	// una operación PROPIA, gateada por `screen` y no por `exec`. Si fuera un exec, abrir una
	// pantalla exigiría el permiso de ejecutar cualquier cosa — y son dos permisos distintos a
	// propósito. El agente reconoce este comando por su primer argumento.
	_, err = s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: proyecto, Principal: nombrePrincipal(p),
		Origen: fleet.OrigenPersona,
		// La contraseña va en el argv, así que este comando NUNCA debe llegar a la bitácora
		// legible: se poda abajo. Ver el comentario de `ocultarArgvDePantalla`.
		Argv:    []string{comandoPantalla, ses.ID, pass, ttl.String()},
		Timeout: fleet.ComandoTimeoutDefault,
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	salida := map[string]interface{}{
		"session_id":  ses.ID,
		"device":      d.Name,
		"rustdesk_id": d.RustdeskID,
		"password":    pass, // UNA sola vez: Musubi no la guarda y no hay forma de recuperarla
		"vence":       ses.Vence.Format(time.RFC3339),
		"minutos":     int(ttl.Minutes()),
		"aviso":       "la contraseña se muestra UNA vez y no se guarda en ningún lado. Vence sola en la máquina aunque el cerebro se caiga. Si la perdés, pedí otra sesión.",
	}
	// CUANDO HUBO QUE PEDIR PERMISO, SE DICE QUE SE CONCEDIÓ. La bitácora ya lo tiene, pero quien
	// abre la pantalla merece saber que del otro lado alguien apretó «permitir»: es la diferencia
	// entre entrar a una máquina y entrar con el consentimiento de quien la está usando.
	if ses.Consentimiento != "" {
		salida["consentimiento"] = string(ses.Consentimiento)
	}
	return jsonResult(salida)
}

// explicarColision arma el mensaje de un rustdesk_id ambiguo.
//
// Dice QUÉ pasa, POR QUÉ importa y CÓMO se arregla — las tres cosas, porque quien recibe esto
// está intentando mirar una pantalla y no tiene por qué saber cómo RustDesk asigna sus ids.
//
// A las máquinas de OTRO alcance se las CUENTA pero no se las nombra: el aislamiento por proyecto
// vale también acá. Alcanza con decir «este id es ambiguo, no te fíes» sin decir de quién.
func explicarColision(d fleet.Device, otras []string, fuera int) string {
	var quienes string
	switch {
	case len(otras) > 0 && fuera > 0:
		quienes = fmt.Sprintf("%s (y %d fuera de tu alcance)", strings.Join(otras, ", "), fuera)
	case len(otras) > 0:
		quienes = strings.Join(otras, ", ")
	default:
		quienes = fmt.Sprintf("%d máquina(s) fuera de tu alcance", fuera)
	}
	return fmt.Sprintf(
		"no se abre la pantalla de %q: su id de RustDesk (%s) lo reclama también %s, así que conectarse sería una moneda al aire. "+
			"Ese id lo reporta la propia máquina, y dos que digan ser la misma significan o bien una imagen clonada (lo más común: RustDesk deriva el id de la máquina, así que los clones nacen iguales), "+
			"o bien que alguien declaró el id de otra para desviar una sesión. "+
			"ARREGLO: regenerá el id en la máquina duplicada (borrar su config de RustDesk y reiniciar el cliente) y esperá un latido. Mientras tanto la pantalla queda cerrada a propósito.",
		d.Name, d.RustdeskID, quienes)
}

// comandoPantalla es el primer argumento que marca a un comando como «aplicar contraseña de
// pantalla». No es un ejecutable: el agente lo intercepta antes de intentar lanzarlo.
//
// El prefijo `musubi:` existe para que no pueda colisionar con un binario real del sistema, y
// para que en cualquier log se lea como lo que es: una operación interna, no un comando del host.
// EL VALOR VIVE EN EL DOMINIO (fleet.OpPantalla): la cronología tiene que clasificar por
// este mismo nombre, y dos literales iguales en dos paquetes es cómo uno se queda viejo.
const comandoPantalla = fleet.OpPantalla

// comandoAviso es la operación con la que el cerebro le habla al usuario de una máquina (A57).
// Igual que la de pantalla: NO es un ejecutable del host y el agente la intercepta antes de
// intentar lanzarla.
const comandoAviso = fleet.OpAvisar

// comandoPreguntar es la operación con la que el cerebro le PIDE PERMISO al usuario de una
// máquina (A57). Distinta de la de avisar porque espera respuesta: el agente contesta por
// /fleet/result y recién ahí la sesión sale de `esperando_permiso`.
const comandoPreguntar = fleet.OpPreguntar

// EsComandoDePantalla dice si un argv es la operación interna de pantalla.
func EsComandoDePantalla(argv []string) bool {
	return len(argv) > 0 && argv[0] == comandoPantalla
}

// ocultarArgvDePantalla reemplaza el argv de un comando de pantalla por una versión sin secreto.
//
// ES OBLIGATORIO EN TODA SUPERFICIE QUE MUESTRE LA BITÁCORA. La contraseña viaja en el argv
// —tiene que llegar a la máquina de alguna forma— y la bitácora de comandos guarda el argv tal
// cual. Sin esta función, `musubi_fleet_log` entregaría contraseñas de sesión a cualquiera que
// pueda leer la bitácora, y la garantía G1 se caería por la puerta de al lado.
func ocultarArgvDePantalla(argv []string) []string {
	return fleet.ArgvDeBitacora(argv)
}

// toolFleetSessions devuelve la bitácora de sesiones de pantalla.
func (s *McpServer) toolFleetSessions(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project string `json:"project"`
		Device  string `json:"device"`
		Limite  int    `json:"limite"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	proyectos, truncado := s.proyectosParaLeer(p, args.Project)
	if len(proyectos) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	tope := bitacoraTopeDefault
	if args.Limite > 0 {
		tope = args.Limite
	}
	if tope > bitacoraTopeMax {
		tope = bitacoraTopeMax
	}

	// G8 — sólo las máquinas sobre las que podés ver ese plano. Saber quién mira la pantalla de un
	// servidor, o quién tuvo un prompt en él, es información sensible por sí sola.
	//
	// LA COMPUERTA ES POR MODALIDAD, y ése es el cambio que trae la vista única. Antes esta tool
	// listaba SÓLO pantallas, así que `screen` alcanzaba para todo lo que devolvía. Ahora también
	// trae shells, y usar `screen` para las dos dejaría ver quién tuvo un prompt en una máquina a
	// alguien que no tiene `shell` sobre ella — una fuga por generalizar la compuerta junto con
	// la consulta.
	puedeVer := func(d fleet.Device, m fleet.Modalidad) bool {
		if m == fleet.ModalidadShell {
			return PuedeSobreDevice(p, d, fleet.CapShell)
		}
		// Mirar la bitácora de pantallas alcanza con poder MIRAR: quien tiene `screen:view` ya
		// puede ver esa pantalla, así que negarle saber quién más la vio no protege nada.
		return PuedeSobreDevice(p, d, fleet.CapScreenView)
	}
	porID := map[string]fleet.Device{}
	ahora := time.Now()
	// Se piden de más y se recorta después de compuertar: el tope es de lo que VAS A VER, no de
	// lo que se leyó. Sin el margen, una credencial acotada recibiría una lista corta que se lee
	// como «no hay más sesiones» cuando lo que pasa es que las siguientes no las puede ver.
	// EL LAZO POR PROYECTO, igual que en las otras tres tools de lectura. Esta se quedó afuera
	// del arreglo original y el síntoma fue exacto: el panel —`read: all` sin proyecto propio—
	// recibía «no se pudo determinar el proyecto» y su columna de sesiones quedaba muda. Tres de
	// cuatro arregladas es el mismo bug con una cuarta parte de la superficie.
	var crudas []fleet.SesionViva
	for _, proyecto := range proyectos {
		devices, err := s.engine.ListarDevices(proyecto, true)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		for _, d := range devices {
			porID[d.ID] = d
		}
		vivas, err := s.engine.SesionesVivas(proyecto, strings.TrimSpace(args.Device), tope*4, ahora)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		crudas = append(crudas, vivas...)
	}
	filas := make([]map[string]interface{}, 0, tope)
	ocultos := 0
	for _, ses := range crudas {
		d, hay := porID[ses.DeviceID]
		if !hay || !puedeVer(d, ses.Modalidad) {
			ocultos++
			continue
		}
		if len(filas) >= tope {
			continue
		}
		fila := map[string]interface{}{
			"session_id": ses.ID,
			// LA MODALIDAD VA PRIMERO ENTRE LOS DATOS porque cambia qué significa todo lo demás:
			// una shell abierta y una pantalla abierta no son el mismo riesgo ni piden lo mismo.
			"modalidad": string(ses.Modalidad),
			"device":    ses.Device,
			"principal": ses.Principal,
			"estado":    string(ses.Estado),
			// `abierta` es DERIVADO y viaja explícito: el estado guardado puede decir `activa`
			// sobre una sesión que ya venció y que nadie vino a marcar, y un panel que dibuja el
			// estado crudo mostraría gente adentro de máquinas de las que ya salió.
			"abierta": ses.Abierta(ahora),
			"creada":  ses.Creada.UTC().Format(time.RFC3339),
			"vence":   ses.Vence.UTC().Format(time.RFC3339),
		}
		if !ses.Cerrada.IsZero() {
			fila["cerrada"] = ses.Cerrada.UTC().Format(time.RFC3339)
		}
		if ses.Error != "" {
			fila["error"] = ses.Error
		}
		filas = append(filas, fila)
	}
	res := map[string]interface{}{"projects": proyectos, "total": len(filas), "sesiones": filas}
	if len(proyectos) == 1 {
		res["project_id"] = proyectos[0]
	}
	if truncado {
		res["proyectos_truncados"] = true
	}
	if ocultos > 0 {
		res["sin_permiso"] = ocultos
	}
	return jsonResult(res)
}

// avisarUnaVezPorDevice deja constancia de que se debía un aviso al usuario de la máquina y no se
// pudo entregar, porque su agente todavía no sabe notificar.
//
// UNA VEZ POR MÁQUINA Y POR OPERACIÓN, y por vida del proceso: una sesión se abre a mano, no en un
// lazo, pero un operador que abre veinte en una tarde no necesita veinte líneas iguales. Lo que
// tiene que quedar es que la deuda existe.
//
// LA CLAVE LLEVA LA OPERACIÓN, y antes no: era una sola por máquina, así que la primera pantalla
// se comía el presupuesto y los `exec` y las shells de esa máquina no dejaban NUNCA una línea. La
// deuda es la misma —el agente no sabe notificar— pero enterarse de que hay shells abriéndose sin
// aviso es otra noticia que enterarse de que hay pantallas.
//
// Y EL TEXTO DECÍA «se abrió una pantalla» EN LOS TRES CAMINOS. Con tres llamadores y un mensaje
// fijo, dos de cada tres líneas del log nombraban una operación que no había pasado — la misma
// clase de defecto que un doc pegado a la declaración equivocada, y en el único lugar donde alguien
// va a mirar cuando `avisa` no avisa.
//
// Esto NO reemplaza al aviso: cuando el agente sepa notificar, este camino desaparece y la
// notificación viaja de verdad. Mientras tanto, la ausencia se ve en el log del cerebro en vez de
// ser silenciosa — que es la diferencia entre una función a medio hacer y una que miente.
func (s *McpServer) avisarUnaVezPorDevice(deviceID, nombre, operacion string, c fleet.Consentimiento) {
	// Se reusa `avisosDados`, que es exactamente para esto: un aviso de CONFIGURACIÓN que no es
	// un evento sino un ESTADO. La clave lleva prefijo para no chocar con los del empuje.
	clave := "consentimiento_sin_aviso\x00" + operacion + "\x00" + deviceID
	if _, ya := s.avisosDados.LoadOrStore(clave, true); ya {
		return
	}
	logx.Warn("flota: el aviso al usuario NO se pudo entregar y la operación siguió igual",
		"device", nombre, "operacion", operacion, "consentimiento", string(c),
		"motivo", "el agente de esta máquina no declara saber notificar (devices.puede_preguntar = 0)")
}

// pedirPermisoParaPantalla es el camino de `pide`: preguntar, y volver sin contraseña (A57).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// TRES SITUACIONES, TRES RESPUESTAS, Y NINGUNA ES UNA ESPERA
//
//  1. NO HAY NADA PEDIDO → se crea la sesión en `esperando_permiso`, se encola la pregunta y se
//     devuelve el id. NO SE ACUÑA CONTRASEÑA: una credencial que existe es una credencial que se
//     puede filtrar, aunque nadie la haya usado, y todavía no se sabe si la respuesta va a ser sí.
//  2. YA HAY UNA ESPERA EN CURSO → se informa, no se pregunta de nuevo. Preguntar dos veces le
//     pone dos ventanas encima a la misma persona por el mismo pedido, que es cómo se le enseña a
//     alguien a apretar «permitir» sin leer.
//  3. YA CONTESTARON → si dijeron que sí, sigue el camino normal y se acuña la contraseña; si no,
//     se niega DICIENDO CUÁL DE LOS TRES «no» fue.
//
// EL PERMISO NO ES LA CREDENCIAL, y por eso la vuelta pasa otra vez por toda la compuerta de
// capacidades de arriba: entre que se concedió el permiso y que se pide la contraseña pueden
// haber revocado la máquina o sacado la capacidad, y el permiso del usuario no vale como
// autorización del sistema.
func (s *McpServer) pedirPermisoParaPantalla(d fleet.Device, p *Principal, proyecto string,
	ttl time.Duration, ahora time.Time) (interface{}, *RpcError) {

	quien := nombrePrincipal(p)
	previa, hay, err := s.sesionEsperandoDe(d, quien, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if hay {
		switch {
		case previa.Estado == fleet.SesionEsperandoPermiso:
			return jsonResult(map[string]interface{}{
				"session_id": previa.ID, "device": d.Name, "estado": string(previa.Estado),
				"aviso": "ya se le preguntó al usuario de esta máquina y todavía no contestó. " +
					"El agente recoge la pregunta en su próximo latido (hasta 30 s) y el diálogo " +
					"espera " + fleet.AvisoTimeout.String() + ". Volvé a pedir la pantalla en un rato.",
			})
		case previa.ConcedeElAcceso():
			// DIJERON QUE SÍ: se sigue por el camino normal. La sesión previa ya quedó en
			// `solicitada`, así que se reusa en vez de abrir otra — abrir una nueva dejaría la
			// concedida colgada y la bitácora con dos filas para un solo permiso.
			return s.entregarPantalla(d, p, proyecto, previa, ttl)
		default:
			return nil, rpcErrorf(codeUnauthorized, "%s", explicarSinPermiso(d, previa))
		}
	}

	// Nada pedido todavía: se pregunta.
	ses, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
		DeviceID: d.ID, ProjectID: proyecto, Principal: quien,
		Estado: fleet.SesionEsperandoPermiso,
		Creada: ahora,
		// LA VENTANA DE LA ESPERA NO ES LA DE LA SESIÓN. Acá `vence` acota cuánto vale el PEDIDO
		// —lo que tarda el latido más el diálogo, con margen—, no cuánto va a durar el acceso.
		// Usar el ttl de la sesión dejaría un pedido de ocho horas esperando una respuesta que
		// venció hace rato.
		Vence: ahora.Add(fleet.VentanaDePermiso),
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	texto := fmt.Sprintf("Musubi: %s pide permiso para ver esta pantalla. ¿Lo permitís?",
		fleet.RecortarRunas(quien, 64))
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: proyecto, Principal: quien,
		Origen:  fleet.OrigenPersona,
		Argv:    []string{comandoPreguntar, ses.ID, texto},
		Timeout: fleet.ComandoTimeoutDefault,
	}); err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	return jsonResult(map[string]interface{}{
		"session_id": ses.ID, "device": d.Name, "estado": string(fleet.SesionEsperandoPermiso),
		"aviso": "esta máquina exige el permiso de quien la está usando. Se le preguntó; el agente " +
			"recoge la pregunta en su próximo latido (hasta 30 s) y el diálogo espera " +
			fleet.AvisoTimeout.String() + ". Volvé a pedir la pantalla en un rato: si dijeron que " +
			"sí vas a recibir la contraseña, y si no, el motivo.",
	})
}

// explicarSinPermiso arma el mensaje de los TRES «no», que se arreglan distinto.
func explicarSinPermiso(d fleet.Device, ses fleet.SesionPantalla) string {
	base := fmt.Sprintf("no se abre la pantalla de %q: ", d.Name)
	switch ses.Consentimiento {
	case fleet.RespuestaNegada:
		return base + "la persona que está usando esa máquina dijo que NO. " +
			"Es el eje funcionando como se configuró; si creés que corresponde igual, hablá con ella."
	case fleet.RespuestaSinRespuesta:
		return base + "se le preguntó y nadie contestó en " + fleet.AvisoTimeout.String() + ". " +
			"El silencio NO es permiso, así que se niega. Si esta máquina está siempre " +
			"desatendida, no debería estar en `pide` — miralo con musubi_fleet_consent."
	case fleet.RespuestaNoSePudo:
		return base + "el agente no tuvo con qué preguntar (no hay escritorio, o le falta la " +
			"herramienta de diálogo). El motivo exacto está en el log del cerebro, en la línea " +
			"«esta máquina no puede pedirle permiso a nadie»."
	default:
		return base + "el pedido de permiso no prosperó."
	}
}

// sesionEsperandoDe busca el pedido de permiso VIGENTE de este principal sobre esta máquina.
//
// SE ACOTA AL PRINCIPAL, y eso no es cosmética: el permiso se le dio a QUIEN preguntó. Sin este
// filtro, un operador aprovecharía el «sí» que la persona le dio a otro — y la pregunta nombra a
// quien entra justamente para que la respuesta sea sobre esa persona.
func (s *McpServer) sesionEsperandoDe(d fleet.Device, quien string, ahora time.Time) (fleet.SesionPantalla, bool, error) {
	sesiones, err := s.engine.SesionesDePantalla(d.ProjectID, d.ID, 20, ahora)
	if err != nil {
		return fleet.SesionPantalla{}, false, err
	}
	for _, ses := range sesiones {
		if ses.Principal != quien || ses.Consentimiento == "" && ses.Estado != fleet.SesionEsperandoPermiso {
			continue
		}
		// UNA ESPERA VENCIDA NO CUENTA: se vuelve a preguntar. Si no, un pedido que nadie
		// contestó hace dos horas bloquearía todos los siguientes con un «ya se preguntó» que
		// nunca se va a resolver.
		if ses.Vencida(ahora) && ses.Estado == fleet.SesionEsperandoPermiso {
			continue
		}
		if ses.Estado == fleet.SesionEsperandoPermiso || ses.Estado == fleet.SesionSinPermiso ||
			(ses.Estado == fleet.SesionSolicitada && ses.Consentimiento != "") {
			return ses, true, nil
		}
	}
	return fleet.SesionPantalla{}, false, nil
}

// LAS TRES FRASES DEL AVISO, JUNTAS Y NO REPARTIDAS POR LOS TRES ARCHIVOS.
//
// Leerlas una debajo de la otra es lo que deja ver si dos se parecen demasiado — y es lo que hace
// evidente que falta una cuando falta. El texto dice QUÉ está pasando y no sólo quién: sin eso,
// quien lo recibe no puede distinguir una sesión de pantalla de una terminal, que son cosas
// distintas para quien está sentado ahí.
//
// «terminal» y no «shell» a propósito: el destinatario no es quien opera. Es la persona en esa
// máquina, que puede no saber qué es una shell y sí qué es que alguien le abra una terminal.
const (
	avisoPantalla = "está abriendo una sesión de pantalla en esta máquina."
	avisoShell    = "está abriendo una terminal en esta máquina."
	avisoExec     = "está ejecutando comandos en esta máquina."
)

// encolarAvisoDeAcceso le manda al agente el aviso que `avisa` promete (A57).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ES UNA SOLA FUNCIÓN PARA LOS TRES CAMINOS, Y ESO ES EL ARREGLO DE A83 — NO UN ORDENAMIENTO
//
// Esto eran DOS copias del mismo bloque, una en pantalla y otra en exec, y A83 fue exactamente lo
// que esa forma produce: al agregar la shell, había que ACORDARSE de copiarlo por tercera vez, y
// nadie se acordó. El eje `avisa` quedó escrito, argumentado y sin efecto en el único camino que
// se saltea cualquier allowlist. Escribir una tercera copia habría dejado la causa intacta para el
// cuarto camino.
//
// Con un solo encolador, sumar un camino es agregar una frase acá arriba y llamar a esta función:
// lo que antes se podía olvidar ahora se ve. Y lo que NO se puede olvidar lo custodia
// TestTodoCaminoQueHonraAvisaLeAvisaAlUsuario, que recorre los tres.
//
// EL TEXTO NOMBRA A QUIEN ENTRA. «Alguien está viendo tu pantalla» no le sirve a nadie: lo que
// convierte esto en información es QUIÉN. Un aviso sin nombre no se puede accionar —no hay a quién
// preguntarle— y se vuelve ruido que la persona aprende a cerrar sin leer. El nombre del principal
// es entrada de configuración (sale de principals.yaml, no de la red), pero igual se acota: termina
// interpolado en un diálogo del escritorio de otra persona.
//
// BEST-EFFORT A PROPÓSITO. Si encolar falla, la operación sigue igual: `avisa` NO bloquea —ése es
// el grado siguiente— y convertir un fallo de la cola en un acceso denegado le daría a `avisa` la
// semántica de `pide` sin que nadie lo decidiera. Lo que no puede pasar es que falle callado, y
// por eso queda la línea, con la operación adentro para que diga cuál fue.
// ════════════════════════════════════════════════════════════════════════════════════════════
func (s *McpServer) encolarAvisoDeAcceso(d fleet.Device, p *Principal, haciendo string) {
	quien := nombrePrincipal(p)
	if quien == "" {
		quien = "un operador"
	}
	texto := fmt.Sprintf("Musubi: %s %s", fleet.RecortarRunas(quien, 64), haciendo)
	if _, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: quien,
		Origen:  fleet.OrigenPersona,
		Argv:    []string{comandoAviso, texto},
		Timeout: fleet.ComandoTimeoutDefault,
	}); err != nil {
		logx.Warn("flota: no se pudo encolar el aviso al usuario; la operación sigue igual",
			"device", d.Name, "haciendo", haciendo, "error", err)
	}
}

// toolFleetConsent fija la política de consentimiento de una máquina.
//
// ES ADMIN, y no `screen`. Quien puede entrar a una máquina no es necesariamente quien decide si
// hay que pedirle permiso a su usuario — al revés: si el que entra pudiera aflojar la política,
// el eje entero sería decoración. Es la misma razón por la que `fleet_service_declare` es admin:
// escribe en el plano de control, no en el de datos.
func (s *McpServer) toolFleetConsent(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized,
			"musubi_fleet_consent escribe la política de acceso de una máquina: requiere un principal admin. "+
				"Tener `screen` sobre ella no alcanza —si quien entra pudiera aflojar la política, el eje no protegería nada.")
	}
	var args struct {
		Device  string `json:"device"`
		Grado   string `json:"grado"`
		Project string `json:"project"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	nombre := strings.TrimSpace(args.Device)
	grado := fleet.Consentimiento(strings.TrimSpace(args.Grado))
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`")
	}
	// SE VALIDA ACÁ Y NO EN EL STORAGE. Un grado que no se entiende no puede guardarse: el
	// dominio lo resolvería al default y quedaría una fila que dice una cosa y significa otra —
	// exactamente la configuración que se ve puesta y no lo está.
	if !grado.Valido() {
		return nil, rpcErrorf(codeInvalidParams,
			"`grado` tiene que ser uno de: libre, avisa, pide, prohibido. Vino %q. "+
				"No se acepta cualquier cosa porque un valor ilegible se resolvería al default y la fila "+
				"diría una cosa mientras el sistema hace otra.", args.Grado)
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if !existe {
		return nil, rpcErrorf(codeInvalidParams, "no hay una máquina %q en el proyecto %q", nombre, proyecto)
	}
	if _, err := s.engine.FijarConsentimiento(d.ID, grado); err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	// SE DEVUELVE EL EFECTIVO Y NO SÓLO LO GUARDADO. Es lo que evita la sorpresa de poner `pide`
	// en un servidor headless y descubrir mucho después que quedó en `prohibido`: la degradación
	// se dice en el momento de configurar, no cuando alguien no puede entrar.
	d.Consentimiento = grado
	efectivo := d.ConsentimientoEfectivo()
	res := map[string]interface{}{
		"device":     d.Name,
		"project_id": d.ProjectID,
		"guardado":   string(grado),
		"efectivo":   string(efectivo),
	}
	if efectivo != grado {
		res["nota"] = "el grado efectivo difiere del guardado porque esta máquina no declara poder " +
			"preguntarle a nadie (su agente no reporta esa capacidad): un `pide` se endurece a `prohibido`."
	}
	return jsonResult(res)
}
