package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// puertaDeCuatroOjos es la compuerta que aplican `shell` y `screen` sobre una máquina marcada.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL CONTRATO DE LOS DOS NIL
//
// Devuelve (nil, nil) cuando hay que SEGUIR. Cualquier otra combinación es la respuesta que el
// llamador tiene que devolver tal cual. Se eligió así —y no un booleano `seguir`— porque un
// booleano invita a ignorarlo: `if !ok` se olvida, y olvidarlo abre la sesión sin aprobación.
// Con dos valores que hay que reenviar, el camino de la distracción no compila.
func (s *McpServer) puertaDeCuatroOjos(d fleet.Device, p *Principal, proyecto string,
	cap fleet.Cap, ahora time.Time) (interface{}, *RpcError) {

	if !d.RequiereAprobacion {
		return nil, nil
	}
	quien := nombrePrincipal(p)
	sol, hay, err := s.engine.AprobacionVigenteDe(d.ID, quien, cap, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	if hay {
		switch sol.Estado {
		case fleet.AprobacionPendiente:
			return jsonResult(map[string]interface{}{
				"solicitud": sol.ID, "device": d.Name, "estado": string(sol.Estado),
				"vence": sol.Vence.UTC().Format(time.RFC3339),
				"aviso": "esta máquina exige cuatro ojos y tu solicitud sigue esperando. NO se le " +
					"avisa a nadie: la aprobación no viaja, se consulta con musubi_fleet_approvals. " +
					"Avisale vos a alguien que también tenga `" + string(cap) + "` sobre esta máquina, " +
					"y que corra musubi_fleet_approve con este id.",
			})
		case fleet.AprobacionNegada:
			return nil, rpcErrorf(codeUnauthorized, "%s", explicarNegada(d, sol))
		case fleet.AprobacionConcedida:
			// SE GASTA ACÁ Y NO DESPUÉS DE ABRIR LA SESIÓN. Si el consumo fuera después, una
			// sesión que falla al abrirse dejaría el permiso vivo, y el segundo intento —o el de
			// otra ventana en paralelo— lo reusaría. Un permiso de un solo uso que sobrevive al
			// primer uso no es de un solo uso.
			gastada, err := s.engine.ConsumirAprobacion(sol.ID, ahora)
			if err != nil {
				return nil, rpcErrorf(codeInternalError, "%v", err)
			}
			if !gastada {
				// La base dijo que no: se usó entre que la leímos y la quisimos gastar, o venció
				// justo. Es la carrera que el WHERE existe para perder de este lado.
				return nil, rpcErrorf(codeUnauthorized,
					"la aprobación %s ya no sirve: la usó otra sesión o venció mientras se abría ésta. "+
						"Pedí otra — es de un solo uso a propósito.", sol.ID)
			}
			logx.Info("sesión abierta con aprobación de cuatro ojos",
				"device", d.Name, "capacidad", string(cap),
				"solicitante", quien, "aprobador", sol.Aprobador, "solicitud", sol.ID)
			return nil, nil
		}
	}

	// Nada pedido todavía: se abre la solicitud. NO se acuña ni se abre nada más — el llamador
	// devuelve esto y se corta acá.
	nueva, err := s.engine.AbrirSolicitudDeAprobacion(fleet.SolicitudDeAprobacion{
		DeviceID: d.ID, ProjectID: proyecto, Solicitante: quien, Capacidad: cap,
		Creada: ahora, Vence: ahora.Add(fleet.VentanaDeAprobacion),
	})
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	logx.Info("cuatro ojos: solicitud abierta",
		"device", d.Name, "capacidad", string(cap), "solicitante", quien, "solicitud", nueva.ID)
	return jsonResult(map[string]interface{}{
		"solicitud": nueva.ID, "device": d.Name, "estado": string(nueva.Estado),
		"vence": nueva.Vence.UTC().Format(time.RFC3339),
		"aviso": "esta máquina exige la aprobación de una segunda persona antes de abrir una sesión " +
			"de `" + string(cap) + "`. Se registró tu solicitud y vale " + fleet.VentanaDeAprobacion.String() +
			". Quien apruebe tiene que tener `" + string(cap) + "` sobre ESTA máquina y no podés ser vos: " +
			"eso son dos ojos, no cuatro. Cuando te la aprueben, volvé a pedir la sesión.",
	})
}

func explicarNegada(d fleet.Device, sol fleet.SolicitudDeAprobacion) string {
	base := fmt.Sprintf("no se abre la sesión en %q: %s no aprobó tu solicitud de `%s`",
		d.Name, sol.Aprobador, sol.Capacidad)
	if n := strings.TrimSpace(sol.Nota); n != "" {
		base += fmt.Sprintf(" — dijo: %q", fleet.RecortarRunas(n, 300))
	}
	// EL «NO» DURA SU VENTANA, y decirlo es parte del control. Si volver a pedir en el acto
	// funcionara, cuatro ojos se degradaría a «pedir hasta que alguien diga que sí».
	return base + ". Un «no» vale hasta que venza la solicitud (" +
		sol.Vence.UTC().Format(time.RFC3339) + "): volver a pedirlo en el acto no es una segunda " +
		"opinión, es insistir. Si cambió algo, hablalo con quien te lo negó."
}

// toolFleetApprove es la segunda persona contestando.
func (s *McpServer) toolFleetApprove(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Solicitud string `json:"solicitud"`
		// Aprobar es un PUNTERO para poder distinguir «no lo dijo» de «dijo que no».
		//
		// Con un bool común, un llamador que se olvida el campo estaría NEGANDO sin saberlo. El
		// sesgo del error importa poco acá —negar es el lado seguro— pero el ruido no: una
		// negativa accidental gasta la ventana de otro y hay que esperarla. Se exige decirlo.
		Aprobar *bool  `json:"aprobar"`
		Nota    string `json:"nota"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	id := strings.TrimSpace(args.Solicitud)
	if id == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `solicitud`: el id que devolvió el intento de abrir la sesión")
	}
	if args.Aprobar == nil {
		return nil, rpcErrorf(codeInvalidParams,
			"falta `aprobar`: decilo explícitamente (true o false). No hay default a propósito — "+
				"un campo omitido que significara «no» negaría por distracción y gastaría la ventana de otro.")
	}
	ahora := time.Now().UTC()

	sol, existe, err := s.engine.SolicitudDeAprobacionPorID(id)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	var d fleet.Device
	if existe {
		var hayDev bool
		d, hayDev, err = s.engine.DevicePorID(sol.DeviceID)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		existe = hayDev
	}
	// INEXISTENTE Y SIN CAPACIDAD DAN LA MISMA RESPUESTA, igual que en exec, shell y pantalla.
	// Distinguirlas convertiría esta tool en un oráculo: probando ids se sabría qué máquinas hay
	// y quién está pidiendo entrar a ellas, que es justo lo que no puede filtrar un control de
	// acceso. Y la capacidad se comprueba ANTES que la identidad del que aprueba, para que
	// alguien sin `shell` sobre esa máquina no se entere de que la solicitud existe.
	if !existe || !PuedeSobreDevice(p, d, sol.Capacidad) {
		return nil, rpcErrorf(codeUnauthorized,
			"no podés resolver la solicitud %q: o no existe, o tu credencial no tiene `%s` sobre esa máquina. "+
				"Aprobar exige LA MISMA capacidad que la sesión que se pide —no `admin`—: la barra es «podrías "+
				"haberlo hecho vos», así que aprobar no te concede nada que no tuvieras.", id, sol.Capacidad)
	}

	quien := nombrePrincipal(p)
	// ════════════════════════════════════════════════════════════════════════════════════════
	// LA COMPROBACIÓN QUE ES TODO EL CONTROL
	//
	// Sin esto queda una tabla, una tool, un estado «concedida» en la bitácora con nombre y hora
	// — y una sola persona abriendo la sesión. El control se ve entero y no existe. Es el único
	// falso verde que esta feature no puede permitirse, y por eso tiene su sabotaje obligatorio.
	if quien == sol.Solicitante {
		return nil, rpcErrorf(codeUnauthorized, "%v", fleet.ErrSeAprueboSolo)
	}

	ok, err := s.engine.ResolverAprobacion(id, quien, args.Nota, *args.Aprobar, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if !ok {
		return nil, rpcErrorf(codeInvalidParams,
			"la solicitud %q ya no está pendiente: alguien la resolvió antes, ya se usó, o venció. "+
				"Su estado era %q y vencía %s.", id, sol.Estado, sol.Vence.UTC().Format(time.RFC3339))
	}
	decision := "negada"
	if *args.Aprobar {
		decision = "concedida"
	}
	logx.Info("cuatro ojos: solicitud resuelta",
		"solicitud", id, "device", d.Name, "capacidad", string(sol.Capacidad),
		"solicitante", sol.Solicitante, "aprobador", quien, "decision", decision)
	return jsonResult(map[string]interface{}{
		"solicitud": id, "device": d.Name, "capacidad": string(sol.Capacidad),
		"solicitante": sol.Solicitante, "aprobador": quien, "estado": decision,
		"vence": sol.Vence.UTC().Format(time.RFC3339),
		"nota": "la aprobación es de UN SOLO USO y vence con la solicitud. " +
			"No se le avisa a nadie: quien pidió la sesión la va a encontrar cuando vuelva a pedirla.",
	})
}

// toolFleetApprovals lista lo que está esperando una segunda persona.
//
// EXISTE PORQUE LA APROBACIÓN NO VIAJA. No hay notificación al que puede aprobar —mandarla
// exigiría saber a quién, y «quién tiene esta capacidad sobre esta máquina» es una consulta sobre
// principals.yaml que este track deliberadamente no invierte—. Sin esta lista, la única forma de
// enterarse sería que el solicitante avise por otro canal, y una solicitud que nadie mira vence
// sola: el control se convertiría en una negación con demora.
func (s *McpServer) toolFleetApprovals(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project string `json:"project"`
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
	ahora := time.Now().UTC()
	pendientes, err := s.engine.AprobacionesPendientes(proyecto, ahora, 50)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	out := make([]map[string]interface{}, 0, len(pendientes))
	ocultas := 0
	for _, sol := range pendientes {
		d, hay, err := s.engine.DevicePorID(sol.DeviceID)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		// SE FILTRA POR LO QUE PODRÍAS APROBAR, no por lo que podrías leer. Una solicitud que no
		// vas a poder resolver en esta lista es una fila que invita a intentarlo y falla — y de
		// paso te cuenta quién está pidiendo entrar a una máquina que no manejás.
		if !hay || !PuedeSobreDevice(p, d, sol.Capacidad) {
			ocultas++
			continue
		}
		out = append(out, map[string]interface{}{
			"solicitud": sol.ID, "device": d.Name, "capacidad": string(sol.Capacidad),
			"solicitante": sol.Solicitante, "motivo": sol.Motivo,
			"creada": sol.Creada.UTC().Format(time.RFC3339),
			"vence":  sol.Vence.UTC().Format(time.RFC3339),
			// Se dice acá y no sólo en el error: quien lee la lista tiene que saber que la suya
			// propia no la puede aprobar ANTES de intentarlo.
			"podes_aprobarla": sol.Solicitante != nombrePrincipal(p),
		})
	}
	res := map[string]interface{}{"pendientes": out, "proyecto": proyecto}
	// LO QUE LA LISTA NO CONTIENE VIAJA EN LA RESPUESTA, misma regla que la cronología: una lista
	// vacía significa «ninguna que YO pueda aprobar», no «no hay nadie esperando».
	if ocultas > 0 {
		res["fuera_de_tu_alcance"] = ocultas
		res["nota"] = fmt.Sprintf("hay %d solicitud(es) más en este proyecto que no podés aprobar "+
			"porque tu credencial no tiene esa capacidad sobre esas máquinas.", ocultas)
	}
	return jsonResult(res)
}

// toolFleetRequireApproval enciende o apaga los cuatro ojos en una máquina.
func (s *McpServer) toolFleetRequireApproval(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized,
			"musubi_fleet_require_approval decide si esta máquina necesita una segunda persona: requiere un principal admin. "+
				"Tener `shell` sobre ella no alcanza — si quien entra pudiera apagar el control, no habría control.")
	}
	var args struct {
		Device   string `json:"device"`
		Project  string `json:"project"`
		Requerir *bool  `json:"requerir"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`")
	}
	if args.Requerir == nil {
		return nil, rpcErrorf(codeInvalidParams,
			"falta `requerir`: decilo explícitamente (true o false). Un campo omitido que significara "+
				"`false` APAGARÍA el control por distracción, que es la dirección peligrosa del error.")
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
	if _, err := s.engine.FijarAprobacion(d.ID, *args.Requerir); err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	logx.Info("cuatro ojos: marca cambiada",
		"device", d.Name, "requiere", *args.Requerir, "principal", nombrePrincipal(p))

	res := map[string]interface{}{
		"device": d.Name, "project_id": d.ProjectID, "requiere_aprobacion": *args.Requerir,
	}
	if *args.Requerir {
		// SE DICE AL ENCENDERLO, no cuando alguien no puede entrar. Con un solo principal esto
		// deja la máquina inaccesible por `shell` y `screen`, y descubrirlo en la urgencia —que
		// es cuando se usa una shell— sería el peor momento.
		res["ojo"] = "a partir de ahora `shell` y `screen` sobre esta máquina exigen que OTRO principal, " +
			"con la misma capacidad sobre ella, apruebe cada sesión. Si en la práctica hay una sola " +
			"persona con esa capacidad, esta máquina queda sin acceso interactivo: cuatro ojos con un " +
			"solo par no es un control lento, es un candado. `metrics` y `exec` no se tocan."
	}
	return jsonResult(res)
}
