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
	// G8 — sólo las máquinas sobre las que tenés `screen`. Saber quién mira la pantalla de un
	// servidor es información sensible por sí sola.
	nombrePorID := make(map[string]string, len(devices))
	for _, d := range devices {
		if PuedeSobreDevice(p, d, fleet.CapScreen) {
			nombrePorID[d.ID] = d.Name
		}
	}

	ahora := time.Now()
	crudas, err := s.engine.SesionesDePantalla(proyecto, strings.TrimSpace(args.Device), tope*4, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	filas := make([]map[string]interface{}, 0, tope)
	ocultos := 0
	for _, ses := range crudas {
		nombre, puede := nombrePorID[ses.DeviceID]
		if !puede {
			ocultos++
			continue
		}
		if len(filas) >= tope {
			continue
		}
		fila := map[string]interface{}{
			"session_id": ses.ID,
			"device":     nombre,
			"principal":  ses.Principal,
			"estado":     string(ses.Estado),
			"creada":     ses.Creada.UTC().Format(time.RFC3339),
			"vence":      ses.Vence.UTC().Format(time.RFC3339),
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
