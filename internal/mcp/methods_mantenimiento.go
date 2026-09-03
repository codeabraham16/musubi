package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// toolFleetMaintenance declara —o cancela— una ventana de mantenimiento sobre una máquina.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTA TOOL NO PIDE `exec` NI ES ADMIN
//
// Declarar una ventana no ejecuta nada ni cambia ninguna autorización: dice «esta máquina va a
// estar rara a propósito». La compuerta correcta es `metrics` sobre ESA máquina, que es la misma
// que hace falta para verla: quien puede mirar su telemetría puede decir que va a estar rara.
//
// Exigir admin la volvería inservible justo donde sirve —el técnico que hace el mantenimiento es
// quien sabe cuándo empieza— y exigir `exec` sería pedir el permiso de otra cosa.
//
// LO QUE SÍ PROTEGE EL TECHO DE 24 h: una ventana silencia alertas Y frena el auto-heal, así que
// una ventana eterna es una máquina ciega con el panel en verde. El techo vive en el dominio
// (fleet.MantenimientoMax) y no acá, para que ningún camino nuevo se lo saltee.
func (s *McpServer) toolFleetMaintenance(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Device  string `json:"device"`
		Project string `json:"project"`
		Minutos int    `json:"minutos"`
		Motivo  string `json:"motivo"`
		Cancel  string `json:"cancelar"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`")
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	// Inexistente y sin permiso dan LA MISMA respuesta, igual que en exec y en shell: distinguirlas
	// convertiría la tool en un oráculo de qué máquinas existen en un proyecto que no ves.
	if !existe || !PuedeSobreDevice(p, d, fleet.CapMetrics) {
		return nil, rpcErrorf(codeUnauthorized,
			"no podés declarar mantenimiento en %q: o no existe en el proyecto %q, o tu credencial no tiene la capacidad `metrics` sobre esa máquina (sección `fleet:` de principals.yaml)", nombre, proyecto)
	}

	ahora := time.Now().UTC()

	if id := strings.TrimSpace(args.Cancel); id != "" {
		// CANCELAR MARCA, NO BORRA. La cronología se construye sólo sobre tablas append-only, y
		// «hubo un mantenimiento y lo cancelaron a los diez minutos» explica el comportamiento de
		// esa máquina mejor que la ausencia de toda fila.
		hubo, err := s.engine.CancelarMantenimiento(id)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		if !hubo {
			return nil, rpcErrorf(codeInvalidParams, "no hay una ventana activa con id %q en esta máquina", id)
		}
		return jsonResult(map[string]interface{}{
			"cancelada": id, "device": d.Name, "project_id": d.ProjectID,
			"nota": "la ventana quedó marcada como cancelada; la fila se conserva porque la cronología se construye sobre tablas que no se editan",
		})
	}

	if args.Minutos <= 0 {
		return nil, rpcErrorf(codeInvalidParams,
			"falta `minutos` (o `cancelar` con el id de una ventana). Una ventana sin largo no existe: el techo es %s",
			fleet.MantenimientoMax)
	}
	m := fleet.Mantenimiento{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: nombrePrincipal(p),
		Desde: ahora, Hasta: ahora.Add(time.Duration(args.Minutos) * time.Minute),
		Motivo: strings.TrimSpace(args.Motivo),
	}
	// La validación del dominio incluye el techo: se deja fallar acá en vez de recortar en
	// silencio, porque recortar le daría a quien pidió 48 h una ventana de 24 sin decírselo.
	creada, err := s.engine.AbrirMantenimiento(m)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	return jsonResult(map[string]interface{}{
		"id": creada.ID, "device": d.Name, "project_id": d.ProjectID,
		"desde": creada.Desde.Format(time.RFC3339), "hasta": creada.Hasta.Format(time.RFC3339),
		"motivo":    creada.Motivo,
		"principal": creada.Principal,
		"nota": "mientras la ventana esté activa, las políticas de auto-heal NO actúan sobre esta máquina y las reglas que miran " +
			"`musubi_fleet_device_maintenance` no alertan. Si nadie la cierra, MantenimientoEterno avisa a las 25 horas.",
	})
}
