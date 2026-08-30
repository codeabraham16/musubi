package mcp

// methods_shell.go abre y audita las sesiones de shell interactiva. Track «Control de flota», S5b.
//
// La compuerta de esta tool es la MISMA de S3 más el cuarto lado de S5b: `shell` es una capacidad
// aparte, y no se implica de `exec` ni de ninguna otra cosa. El porqué está en fleet.CapShell y
// cabe en una frase: quien obtiene un prompt corre lo que quiera, así que gatearlo con `exec`
// convertiría la allowlist por comando de S10 en decoración.

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// toolFleetShell abre una sesión interactiva y devuelve cómo hablarle.
func (s *McpServer) toolFleetShell(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Device   string `json:"device"`
		Project  string `json:"project"`
		Filas    int    `json:"filas"`
		Columnas int    `json:"columnas"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`: en qué máquina abrir la shell")
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}

	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	// Inexistente y sin permiso dan LA MISMA respuesta, igual que en exec: distinguirlas
	// convertiría la tool en un oráculo de qué máquinas existen en un proyecto que no ves.
	//
	// Y el mensaje NOMBRA la capacidad `shell` explícitamente, porque el error más probable acá
	// es que alguien tenga `exec` y crea que le alcanza. Que no le alcance es el diseño, y decirlo
	// mal mandaría a esa persona a revisar la concesión equivocada.
	if !existe || !PuedeSobreDevice(p, d, fleet.CapShell) {
		return nil, rpcErrorf(codeUnauthorized,
			"no podés abrir una shell en %q: o no existe en el proyecto %q, o tu credencial no tiene la capacidad `shell` sobre esa máquina. OJO: `shell` NO se deriva de `exec` — una shell interactiva se saltea cualquier allowlist de comandos, así que se concede aparte (sección `fleet:` de principals.yaml).",
			nombre, proyecto)
	}
	if err := fleet.ValidarAperturaShell(d); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}

	ahora := time.Now()
	// T7 — UNA SOLA SESIÓN VIVA POR (persona, máquina). No es una limitación técnica: dos prompts
	// simultáneos de la misma persona en la misma máquina son, casi siempre, una sesión olvidada
	// más una nueva — y la olvidada es la peligrosa. Se devuelve la que ya está en vez de abrir
	// otra, para que quien perdió su terminal pueda volver a ella y cerrarla.
	if previa, hay, err := s.engine.SesionShellAbiertaDe(nombrePrincipal(p), d.ID, ahora); err == nil && hay {
		return jsonResult(respuestaShell(previa, d, "ya tenías una sesión abierta en esta máquina; se devuelve ésa. Cerrala si querés una nueva."))
	}

	// LA BITÁCORA SE ESCRIBE ANTES DE CONECTAR — misma regla que F1 de S5 y G7 de S6. Si el SSH
	// nunca prende, el PEDIDO queda registrado igual: que alguien haya intentado abrir una shell
	// en un servidor es información de auditoría tanto como que lo haya logrado.
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID: d.ID, ProjectID: proyecto, Principal: nombrePrincipal(p),
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	canal, err := s.abrirCanalShell(d, args.Filas, args.Columnas)
	if err != nil {
		// El fallo también se audita, en la misma fila.
		s.cerrarShell(ses.ID, fleet.ShellFallida, err.Error(), time.Now())
		return nil, rpcErrorf(codeInternalError, "no se pudo abrir la shell en %q: %v", d.Name, err)
	}
	s.shells.guardar(ses.ID, canal)

	// Tier A: hay que ir a avisarle. Si el aviso no se puede encolar, la sesión no va a
	// engancharse nunca y se cierra ACÁ en vez de dejar a alguien esperando un prompt que no
	// viene.
	if d.Tier == fleet.TierAgente {
		if err := s.avisarAlAgenteDeLaShell(d, ses, args.Filas, args.Columnas); err != nil {
			s.cerrarShell(ses.ID, fleet.ShellFallida, "no se pudo avisarle al agente: "+err.Error(), time.Now())
			return nil, rpcErrorf(codeInternalError, "no se pudo avisarle al agente de %q: %v", d.Name, err)
		}
	}

	// Si la shell remota muere sola (alguien teclea `exit`, se cae la red), la fila se cierra sin
	// que nadie tenga que preguntar. Sin esto, la bitácora quedaría con sesiones «activas» que
	// terminaron hace horas.
	go func() {
		<-canal.Terminado()
		s.cerrarShell(ses.ID, fleet.ShellCerrada, "la shell remota terminó", time.Now())
	}()

	logx.Info("shell interactiva abierta",
		"sesion", ses.ID, "device", d.Name, "principal", ses.Principal, "vence", ses.Vence.Format(time.RFC3339))
	return jsonResult(respuestaShell(ses, d, ""))
}

// abrirCanalShell elige el transporte según el tier.
//
// Hoy sólo Tier B. Tier A necesita que el AGENTE abra un pty local y lo relaye, que es otro
// problema entero (ioctls sobre /dev/ptmx sin cgo, y ConPTY en Windows) — S5c. Se dice acá, con
// nombre, en vez de fallar con un error genérico que mandaría a alguien a revisar la red.
func (s *McpServer) abrirCanalShell(d fleet.Device, filas, columnas int) (fleet.CanalInteractivo, error) {
	switch d.Tier {
	case fleet.TierProtocolo:
		return fleet.AbrirShellPorSSH(d.Address, filas, columnas)
	case fleet.TierAgente:
		// A un Tier A NO LE ENTRA NADIE: está detrás de un NAT, sin puertos abiertos, y sólo sabe
		// SALIR hacia el cerebro — que es toda su razón de ser. Así que no se abre nada: se deja
		// un punto de encuentro y se le avisa por la cola de comandos, la misma por la que le
		// llegan los exec y las sesiones de pantalla.
		return fleet.NuevoCanalAgente(), nil
	default:
		return nil, errShellTierNoSoportado(d.Tier)
	}
}

// comandoShell es la operación interna que le dice al agente que lo llamaron.
//
// Viaja por el MISMO canal que los comandos y las sesiones de pantalla, y por eso hereda la
// guarda que S6 puso ahí: `musubi:*` no se puede encolar con `musubi_fleet_exec` (si no, alguien
// con `exec` se fabricaría una sesión de shell sin tener `shell`, que es justo la separación que
// S5b vino a establecer).
const comandoShell = fleet.OpShell

// avisarAlAgenteDeLaShell encola el pedido para que el agente se conecte.
//
// LATENCIA REAL Y DECLARADA: el agente se entera en su PRÓXIMO LATIDO, así que abrir una shell en
// un Tier A demora hasta un intervalo de latido (30 s por defecto). No es un bug, es la
// consecuencia de que la máquina no acepte conexiones entrantes. Quien quiera un prompt más
// rápido baja el `--interval` de su agente y paga más tráfico.
func (s *McpServer) avisarAlAgenteDeLaShell(d fleet.Device, ses fleet.SesionShell, filas, columnas int) error {
	_, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: ses.ProjectID, Principal: ses.Principal,
		// El canal lo abre el cerebro, pero lo PIDIÓ una persona: sin ella no hay sesión.
		Origen:  fleet.OrigenPersona,
		Argv:    []string{comandoShell, ses.ID, strconv.Itoa(filas), strconv.Itoa(columnas)},
		Timeout: fleet.ComandoTimeoutDefault,
	})
	return err
}

func errShellTierNoSoportado(t fleet.Tier) error {
	return &erroresShell{fmt: "la shell interactiva todavía sólo funciona en Tier B (por SSH); esta máquina es tier %s. Tier A necesita que el agente abra un pty propio, y eso llega en otro slice.", arg: string(t)}
}

type erroresShell struct {
	fmt string
	arg string
}

func (e *erroresShell) Error() string { return strings.Replace(e.fmt, "%s", e.arg, 1) }

// respuestaShell arma lo que ve quien abre. Incluye las rutas para que el cliente no tenga que
// saberse la forma de la API: si mañana cambian, el cliente viejo se entera por acá.
func respuestaShell(ses fleet.SesionShell, d fleet.Device, nota string) map[string]interface{} {
	out := map[string]interface{}{
		"session_id":          ses.ID,
		"device":              d.Name,
		"project_id":          ses.ProjectID,
		"estado":              string(ses.Estado),
		"vence":               ses.Vence.UTC().Format(time.RFC3339),
		"inactividad_max_seg": int(fleet.ShellInactividadMax.Seconds()),
		"ruta_salida":         shellOutPath,
		"ruta_entrada":        shellInPath,
		"ruta_cierre":         shellClosePath,
		// EL ID NO ES UNA CREDENCIAL, y se dice acá para que nadie lo trate como una: cada
		// request del stream lleva el bearer de la persona y se vuelve a autorizar entero.
		"nota_seguridad": "el session_id NO es un token: cada request al stream exige tu bearer y se re-autoriza (incluida tu concesión `shell`, que puede revocarse a mitad de sesión).",
	}
	if nota != "" {
		out["nota"] = nota
	}
	// La demora de un Tier A se DICE. Un prompt que tarda medio minuto sin explicación se lee
	// como un cuelgue, y quien lo sufre corta y vuelve a intentar — abriendo otra sesión.
	if d.Tier == fleet.TierAgente {
		out["nota_demora"] = "esta máquina tiene agente y no acepta conexiones entrantes: se entera de que la llamaron en su próximo latido, así que el prompt puede tardar hasta un intervalo (30 s por defecto)."
	}
	return out
}

// toolFleetShellLog devuelve la bitácora de sesiones: quién tuvo un prompt, dónde y por cuánto.
//
// Exige `shell` sobre la máquina para verla, por el mismo criterio con el que la bitácora de
// comandos exige `exec`: saber quién entró a un servidor y cuándo es casi tan revelador como
// poder entrar.
func (s *McpServer) toolFleetShellLog(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
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
	nombrePorID := make(map[string]string, len(devices))
	for _, d := range devices {
		if PuedeSobreDevice(p, d, fleet.CapShell) {
			nombrePorID[d.ID] = d.Name
		}
	}

	crudas, err := s.engine.BitacoraDeShell(proyecto, strings.TrimSpace(args.Device), tope*4)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	ahora := time.Now()
	filas := make([]map[string]interface{}, 0, tope)
	ocultas := 0
	for _, ses := range crudas {
		nombre, puede := nombrePorID[ses.DeviceID]
		if !puede {
			ocultas++
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
		}
		// El estado se DERIVA: una fila que dice «activa» y venció hace una hora mentiría.
		if vencida, motivo := ses.Vencida(ahora); vencida && ses.Cerrada.IsZero() {
			fila["estado"] = string(fleet.ShellVencida)
			fila["motivo"] = motivo
		}
		if !ses.Cerrada.IsZero() {
			fila["cerrada"] = ses.Cerrada.UTC().Format(time.RFC3339)
			fila["duracion_seg"] = int(ses.Cerrada.Sub(ses.Creada).Seconds())
		}
		if ses.Error != "" {
			fila["motivo"] = ses.Error
		}
		filas = append(filas, fila)
	}
	res := map[string]interface{}{"project_id": proyecto, "total": len(filas), "sesiones": filas}
	if ocultas > 0 {
		res["sin_permiso"] = ocultas
	}
	// Se dice lo que NO se guarda, en la respuesta misma: alguien que audita tiene que saber que
	// acá no va a encontrar lo que se tecleó, y por qué.
	res["nota"] = "la bitácora registra QUE hubo acceso (quién, dónde, cuándo, cuánto). El CONTENIDO de la sesión no se guarda: grabar lo que alguien teclea es una decisión legal que nadie tomó."
	return jsonResult(res)
}
