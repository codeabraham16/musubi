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
	switch consent := d.ConsentimientoEfectivo(); {
	case consent.Bloquea():
		return nil, rpcErrorf(codeUnauthorized,
			"no se abre la pantalla de %q: %v. "+
				"El grado configurado en esta máquina es %q; si además figura como que no puede preguntar, "+
				"un `pide` se endurece a prohibido a propósito — quien escribió `pide` pidió que nadie entre "+
				"sin permiso, y si el permiso no se puede pedir, no se entra.",
			nombre, fleet.ErrConsentimientoProhibido, d.Consentimiento)
	case consent.AvisaAlUsuario() && !d.PuedePreguntar:
		// SE ABRE, Y SE DICE QUE EL AVISO NO SE PUDO ENTREGAR. Prometer una notificación que
		// ningún agente sabe dar todavía sería exactamente lo que este eje viene a evitar: una
		// configuración que se ve puesta y no lo está. Bloquear tampoco: `avisa` no bloquea, y
		// hacerlo cerraría el acceso a toda la flota por una capacidad que nadie desplegó aún.
		s.avisarUnaVezPorDevice(d.ID, nombre, consent)
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

	// G7 — la sesión se registra ANTES de acuñar nada. Que alguien haya INTENTADO mirar una
	// pantalla es información de auditoría tanto como que lo haya logrado.
	ses, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
		DeviceID: d.ID, ProjectID: proyecto, Principal: nombrePrincipal(p),
		Creada: ahora, Vence: ahora.Add(ttl),
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
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
		// La contraseña va en el argv, así que este comando NUNCA debe llegar a la bitácora
		// legible: se poda abajo. Ver el comentario de `ocultarArgvDePantalla`.
		Argv:    []string{comandoPantalla, ses.ID, pass, ttl.String()},
		Timeout: fleet.ComandoTimeoutDefault,
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	return jsonResult(map[string]interface{}{
		"session_id":  ses.ID,
		"device":      d.Name,
		"rustdesk_id": d.RustdeskID,
		"password":    pass, // UNA sola vez: Musubi no la guarda y no hay forma de recuperarla
		"vence":       ses.Vence.Format(time.RFC3339),
		"minutos":     int(ttl.Minutes()),
		"aviso":       "la contraseña se muestra UNA vez y no se guarda en ningún lado. Vence sola en la máquina aunque el cerebro se caiga. Si la perdés, pedí otra sesión.",
	})
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
const comandoPantalla = "musubi:pantalla"

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
	if !EsComandoDePantalla(argv) {
		return argv
	}
	// Se conserva el id de sesión (sirve para cruzar con la bitácora de pantalla) y se tapa el
	// resto. El largo del resultado no depende del secreto.
	//
	// El marcador va SIN ángulos: `encoding/json` escapa `<` y `>` por default (protección
	// anti-XSS heredada), así que un `<oculto>` sale como `\u003coculto\u003e` y una bitácora
	// leída en crudo se vuelve ilegible justo en la línea que más se mira.
	id := ""
	if len(argv) > 1 {
		id = argv[1]
	}
	return []string{comandoPantalla, id, "[oculto]"}
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
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	tope := bitacoraTopeDefault
	if args.Limite > 0 {
		tope = args.Limite
	}
	if tope > bitacoraTopeMax {
		tope = bitacoraTopeMax
	}

	devices, err := s.engine.ListarDevices(proyecto, true)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
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
	porID := make(map[string]fleet.Device, len(devices))
	for _, d := range devices {
		porID[d.ID] = d
	}

	ahora := time.Now()
	// Se piden de más y se recorta después de compuertar: el tope es de lo que VAS A VER, no de
	// lo que se leyó. Sin el margen, una credencial acotada recibiría una lista corta que se lee
	// como «no hay más sesiones» cuando lo que pasa es que las siguientes no las puede ver.
	crudas, err := s.engine.SesionesVivas(proyecto, strings.TrimSpace(args.Device), tope*4, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
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
	res := map[string]interface{}{"project_id": proyecto, "total": len(filas), "sesiones": filas}
	if ocultos > 0 {
		res["sin_permiso"] = ocultos
	}
	return jsonResult(res)
}

// avisarUnaVezPorDevice deja constancia de que se debía un aviso al usuario de la máquina y no se
// pudo entregar, porque su agente todavía no sabe notificar.
//
// UNA VEZ POR MÁQUINA Y POR VIDA DEL PROCESO: una sesión de pantalla se abre a mano, no en un
// lazo, pero un operador que abre veinte en una tarde no necesita veinte líneas iguales. Lo que
// tiene que quedar es que la deuda existe.
//
// Esto NO reemplaza al aviso: cuando el agente sepa notificar, este camino desaparece y la
// notificación viaja de verdad. Mientras tanto, la ausencia se ve en el log del cerebro en vez de
// ser silenciosa — que es la diferencia entre una función a medio hacer y una que miente.
func (s *McpServer) avisarUnaVezPorDevice(deviceID, nombre string, c fleet.Consentimiento) {
	// Se reusa `avisosDados`, que es exactamente para esto: un aviso de CONFIGURACIÓN que no es
	// un evento sino un ESTADO. La clave lleva prefijo para no chocar con los del empuje.
	clave := "consentimiento_sin_aviso\x00" + deviceID
	if _, ya := s.avisosDados.LoadOrStore(clave, true); ya {
		return
	}
	logx.Warn("flota: se abrió una pantalla y el aviso al usuario NO se pudo entregar",
		"device", nombre, "consentimiento", string(c),
		"motivo", "el agente de esta máquina no declara saber notificar (devices.puede_preguntar = 0)")
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
