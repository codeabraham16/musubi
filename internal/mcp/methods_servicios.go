package mcp

// methods_servicios.go es QUÉ CORRE ADENTRO de las máquinas de la flota, visto por una PERSONA
// (slice S12). Dos tools: una para mirar y otra para declarar lo que ninguna máquina enumera sola.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE REUSA `metrics` Y NO SE INVENTA UNA Cap NUEVA, y conviene tener escrito por qué
//
// Qué corre en una máquina es TELEMETRÍA DEL HOST, del mismo peso que su uso de CPU: quien puede
// ver que un servidor está al 95 % puede ver que el postgres se está reiniciando. No hay una
// tercera clase de secreto acá.
//
// Y el costo de la Cap nueva sería alto y silencioso: obligaría a tocar la matriz `capsPorTier`,
// la lista `todas` de capsQuePuede —cuyo ORDEN dibuja la columna «admite / puedo» del panel— y
// seis bucles exhaustivos repartidos en tres paquetes. Peor: el barrido de aislamiento le da al
// atacante `metrics`, `exec` y `screen` sobre "*" a propósito, así que una Cap nueva haría que
// ese barrido pase probando la COMPUERTA DE CAPACIDADES en vez de la TENENCIA — verde por el
// motivo equivocado, que es peor que rojo.
//
// ACCIONAR sobre un servicio (reiniciarlo) sigue siendo `exec`, con su allowlist por argv encima
// y su línea en la bitácora. Leer y tocar siguen siendo dos permisos distintos.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// toolFleetServices lista QUÉ CORRE en las máquinas del proyecto. readOnly: lee la tabla
// `services` y no escribe nada — quien ESCRIBE es el latido, por la otra puerta.
//
// La compuerta es POR MÁQUINA, no por proyecto: cada servicio se filtra con PuedeSobreDevice
// sobre SU device. Filtrar sólo por proyecto dejaría ver el inventario de una máquina sobre la
// que esta credencial no tiene nada concedido, que es justo lo que la sección `fleet:` decide.
func (s *McpServer) toolFleetServices(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project          string `json:"project"`
		Device           string `json:"device"`
		IncluirRevocados bool   `json:"incluir_revocados"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar de qué proyecto listar los servicios: declaralo en `project`")
	}

	// Las máquinas primero: hacen falta para compuertar (PuedeSobreDevice pide el Device entero)
	// y para traducir el id interno de cada servicio a un nombre que una persona reconozca.
	devices, err := s.engine.ListarDevices(proyecto, args.IncluirRevocados)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	porID := make(map[string]fleet.Device, len(devices))
	filtroID := ""
	filtro := strings.TrimSpace(args.Device)
	for _, d := range devices {
		porID[d.ID] = d
		if filtro != "" && d.Name == filtro {
			filtroID = d.ID
		}
	}
	if filtro != "" && filtroID == "" {
		// Se responde VACÍO y no un error: «no existe» y «no la podés ver» tienen que verse
		// igual desde afuera, o el filtro se vuelve un oráculo de qué máquinas hay en el proyecto.
		//
		// Y ESO NO ALCANZABA. La respuesta de acá salía SIN `sin_permiso`, y la del camino normal
		// —máquina que existe y no podés ver— salía CON él. Dos formas distintas es un oráculo
		// igual, sólo que más sutil: preguntando por un nombre alcanzaba para saber si estaba.
		// Verificado por el verificador adversarial con dos llamadas: `{"services":[]}` contra
		// `{"services":[],"sin_permiso":2}`. Ahora las dos salen por la misma puerta.
		return jsonResult(respuestaDeServicios(proyecto, nil, 0, 0, filtro != ""))
	}

	servicios, err := s.engine.ListarServicios(proyecto, filtroID, args.IncluirRevocados)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	ahora := time.Now()
	filas := make([]map[string]interface{}, 0, len(servicios))
	sinPermiso, huerfanos := 0, 0
	for _, sv := range servicios {
		d, hay := porID[sv.DeviceID]
		if !hay {
			// Un servicio cuyo device no está en el listado del proyecto. Sin foreign keys esto
			// es representable (una fila reparada a mano, un backfill), y no hay contra qué
			// compuertar: se cuenta y no se muestra. Fail-closed.
			huerfanos++
			continue
		}
		if !PuedeSobreDevice(p, d, fleet.CapMetrics) {
			sinPermiso++
			continue
		}
		filas = append(filas, filaDeServicio(sv, d, ahora))
	}

	return jsonResult(respuestaDeServicios(proyecto, filas, sinPermiso, huerfanos, filtro != ""))
}

// respuestaDeServicios arma la respuesta del listado, y decide si los contadores salen.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ LOS CONTADORES SE CALLAN CUANDO HAY FILTRO POR MÁQUINA
//
// Sin filtro, `sin_permiso` es información honesta y necesaria: una lista corta sin explicación
// se lee como «no hay más servicios», que es distinto de «no podés verlos». El número es
// agregado, no nombra nada, y no revela existencia.
//
// CON un filtro por nombre de máquina, ese mismo número se vuelve el oráculo. Preguntás por
// `pc-gio`: si te vuelve `sin_permiso: 2` la máquina existe y no la ves; si te vuelve la
// respuesta pelada, no existe. Alcanzaba con probar nombres para mapear la flota del vecino.
// Por eso con filtro salen las dos por la misma puerta y sin contadores — se pierde una pista
// que quien filtró por una máquina suya no necesita (si tenés `metrics` sobre ella, el contador
// era cero de todos modos).
func respuestaDeServicios(proyecto string, filas []map[string]interface{}, sinPermiso, huerfanos int, conFiltro bool) map[string]interface{} {
	if filas == nil {
		filas = []map[string]interface{}{}
	}
	res := map[string]interface{}{"project_id": proyecto, "services": filas}
	if conFiltro {
		return res
	}
	if sinPermiso > 0 {
		res["sin_permiso"] = sinPermiso
	}
	if huerfanos > 0 {
		res["sin_maquina"] = huerfanos
	}
	return res
}

// toolFleetServiceDeclare da de alta a mano un servicio que ninguna máquina enumera sola: un
// Tier B, un bot, un puente. ADMIN, como el alta y la baja de una máquina.
//
// Es admin y no `metrics` porque escribe en el inventario del plano de control: quien puede
// declarar servicios puede llenar el panel de otro con filas inventadas.
func (s *McpServer) toolFleetServiceDeclare(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_fleet_service_declare escribe en el inventario del plano de control: requiere un principal admin")
	}
	var args struct {
		Device  string `json:"device"`
		Nombre  string `json:"nombre"`
		Clase   string `json:"clase"`
		Project string `json:"project"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	if strings.TrimSpace(args.Device) == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`: el nombre de la máquina en la que corre el servicio")
	}
	if strings.TrimSpace(args.Nombre) == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `nombre`: la unit, el servicio o el contenedor (por ejemplo \"postgresql.service\")")
	}
	// El proyecto lo fija la CREDENCIAL, igual que en el enrolado: un principal acotado declara
	// en SU tenant aunque diga otro.
	proyecto, ok := writeOriginFor(p, args.Project)
	if !ok || strings.TrimSpace(proyecto) == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto del servicio: declaralo en `project`")
	}

	// La máquina se resuelve POR NOMBRE DENTRO DE ESE PROYECTO, y de ELLA sale el project_id del
	// servicio. Una máquina de otro tenant simplemente no aparece — y el rechazo es EL MISMO que
	// el de una que no existe, palabra por palabra, para que no se pueda usar esta tool como
	// oráculo de qué máquinas tiene el vecino. Es el mismo texto que musubi_fleet_revoke.
	d, existe, err := s.engine.DevicePorNombre(proyecto, args.Device)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if !existe || d.Revoked {
		return nil, rpcErrorf(codeInvalidParams, "no hay un dispositivo activo llamado %q en el proyecto %q",
			strings.TrimSpace(args.Device), proyecto)
	}

	sv, err := s.engine.AltaServicio(fleet.Servicio{
		Nombre: args.Nombre, DeviceID: d.ID, Clase: args.Clase,
	})
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	return jsonResult(map[string]interface{}{
		"declared":   true,
		"service_id": sv.ID,
		"nombre":     sv.Nombre,
		"device":     d.Name,
		"project_id": sv.ProjectID,
		"clase":      sv.Clase,
		"estado":     string(fleet.EstadoDesconocido),
		"aviso": "declarado y todavía sin medir: `desconocido` NO es `detenido`. El estado aparece cuando la máquina lo reporte, " +
			"o se queda así si nadie lo mide. Lo declarado a mano NO lo poda el latido: lo saca una persona.",
	})
}

// filaDeServicio arma lo que ve una persona. `estado` y `fresco` se DERIVAN al servir, igual que
// `online` en el inventario de máquinas: no hay columna que se pueda quedar vieja.
//
// Lo desconocido viaja como null —pid, reinicios y `desde` son punteros— y `estado` es
// "desconocido" cuando no hay salud, JAMÁS "detenido".
// filaDeServicio arma lo que ve el operador.
//
// NO recibe el umbral del dispositivo a propósito. Lo recibía, y ése fue el defecto: la frescura
// de un SERVICIO se mide contra el ritmo del INVENTARIO (fleet.UmbralInventario), no contra el
// del latido. Un parámetro que se ignora es peor que no tenerlo — el próximo lo pasa y espera
// que importe.
func filaDeServicio(sv fleet.Servicio, d fleet.Device, ahora time.Time) map[string]interface{} {
	fila := map[string]interface{}{
		"nombre": sv.Nombre,
		"device": d.Name,
		"clase":  sv.Clase,
		"estado": string(sv.EstadoActual()),
		// `fresco` es lo que separa «corriendo» de «lo último que supimos es que corría, hace dos
		// días». Sin él, un servicio muerto con su última salud buena se lee como sano para
		// siempre — el mismo modo de falla que el `online` derivado cierra para las máquinas.
		// La frescura de un SERVICIO no se mide con el umbral del DISPOSITIVO. El latido va cada
		// pocos segundos; el inventario, cada `fleet.InventarioCada`. Medirlo con el del
		// dispositivo dejaba todo servicio en `fresco: false` para siempre — y un false
		// permanente no es una alarma, es ruido que enseña a ignorar la columna.
		"fresco":   sv.Fresco(ahora, fleet.UmbralInventario),
		"revocado": sv.Revocado,
		// `declarado` se muestra porque cambia QUÉ le va a pasar a esta fila: lo que puso una
		// persona no lo poda el latido de la máquina, y lo que enumeró la máquina desaparece solo
		// cuando deja de reportarse. Sin la columna, las dos filas se ven igual y esperan cosas
		// distintas.
		"declarado": sv.Declarado,
		// nulls, no ceros: los tres son punteros y un servicio detenido no tiene pid.
		"pid":       nil,
		"reinicios": nil,
		"desde":     nil,
		"detalle":   "",
	}
	if sv.UltimoReporte.IsZero() {
		// DECLARADO Y TODAVÍA SIN MUESTRAS: un estado legítimo y distinto de «caído». Viaja como
		// null y no como una fecha del año 1, que un panel dibujaría como «hace 2025 años».
		fila["ultimo_reporte"] = nil
		fila["antiguedad_s"] = nil
	} else {
		fila["ultimo_reporte"] = sv.UltimoReporte.UTC().Format(time.RFC3339)
		fila["antiguedad_s"] = int(ahora.Sub(sv.UltimoReporte).Seconds())
	}
	if sv.Salud != nil {
		fila["pid"] = sv.Salud.PID
		fila["reinicios"] = sv.Salud.Reinicios
		fila["detalle"] = sv.Salud.Detalle
		if sv.Salud.Desde != nil {
			fila["desde"] = sv.Salud.Desde.UTC().Format(time.RFC3339)
		}
	}
	return fila
}
