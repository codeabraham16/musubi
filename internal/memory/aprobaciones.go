package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"musubi/internal/fleet"
)

const columnasAprobacion = `id, device_id, project_id, solicitante, capacidad, motivo, estado, aprobador, nota, creada, vence, resuelta, usada`

// FijarAprobacion enciende o apaga los cuatro ojos en una máquina.
//
// Es su propio método y no un update genérico, por lo mismo que FijarConsentimiento: un update
// genérico sobre devices es una superficie por donde cualquier camino puede escribir cualquier
// columna, y ésta decide si hace falta una segunda persona.
func (e *DbEngine) FijarAprobacion(deviceID string, requiere bool) (bool, error) {
	v := 0
	if requiere {
		v = 1
	}
	res, err := e.db.Exec(`UPDATE devices SET requiere_aprobacion = ? WHERE id = ?`, v, deviceID)
	if err != nil {
		return false, fmt.Errorf("error al fijar la aprobación de %q: %w", deviceID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado: %w", err)
	}
	return n > 0, nil
}

// AbrirSolicitudDeAprobacion registra un pedido. Append-only: no actualiza nada.
func (e *DbEngine) AbrirSolicitudDeAprobacion(s fleet.SolicitudDeAprobacion) (fleet.SolicitudDeAprobacion, error) {
	if s.Creada.IsZero() {
		s.Creada = time.Now().UTC()
	}
	if s.Vence.IsZero() {
		s.Vence = s.Creada.Add(fleet.VentanaDeAprobacion)
	}
	if s.Estado == "" {
		s.Estado = fleet.AprobacionPendiente
	}
	if err := fleet.ValidarSolicitud(s); err != nil {
		return fleet.SolicitudDeAprobacion{}, err
	}
	// El id lo asigna el cerebro, igual que en AltaDevice y AbrirMantenimiento: un id elegido por
	// quien pide es un id que se puede pisar — y acá pisarlo sería reusar la aprobación de otro.
	s.ID = uuid.NewString()
	if _, err := e.db.Exec(
		`INSERT INTO fleet_approvals (`+columnasAprobacion+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, '', '', ?, ?, '', '')`,
		s.ID, s.DeviceID, s.ProjectID, s.Solicitante, string(s.Capacidad),
		fleet.RecortarRunas(strings.TrimSpace(s.Motivo), 500), string(s.Estado),
		s.Creada.UTC().Format(time.RFC3339), s.Vence.UTC().Format(time.RFC3339),
	); err != nil {
		return fleet.SolicitudDeAprobacion{}, fmt.Errorf("error al abrir la solicitud de aprobación: %w", err)
	}
	return s, nil
}

// SolicitudDeAprobacionPorID trae una solicitud.
func (e *DbEngine) SolicitudDeAprobacionPorID(id string) (fleet.SolicitudDeAprobacion, bool, error) {
	row := e.db.QueryRow(`SELECT `+columnasAprobacion+` FROM fleet_approvals WHERE id = ?`, id)
	s, err := escanearAprobacion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return fleet.SolicitudDeAprobacion{}, false, nil
	}
	if err != nil {
		return fleet.SolicitudDeAprobacion{}, false, err
	}
	return s, true, nil
}

// AprobacionVigenteDe busca el pedido VIGENTE de este principal sobre esta máquina y capacidad.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA CONSULTA ACOTA POR SOLICITANTE, Y ESO NO ES UNA OPTIMIZACIÓN
//
// La aprobación se le dio a QUIEN pidió, para hacer ESO, en ESA máquina. Sin el filtro por
// solicitante, la aprobación que alguien le dio a una persona la usaría cualquier otra — y el
// que aprobó nombró a alguien justamente para que su «sí» fuera sobre esa persona. Es la misma
// regla que sesionEsperandoDe sostiene del lado del consentimiento.
//
// Y ACOTA POR CAPACIDAD por lo mismo: aprobar que alguien MIRE una pantalla no es aprobar que
// abra una shell. Sin el filtro, el permiso más barato de conseguir habilitaría el más caro.
//
// Las vencidas se excluyen ACÁ y no en el llamador: un permiso vencido que llega al camino de
// acceso es un permiso que alguien tiene que acordarse de mirar, y ese olvido se ve idéntico a
// que el control funcione.
//
// EL ORDEN ES POR QUÉ ESTADO MANDA, NO POR FECHA, y ésa es la parte que se hizo mal primero.
//
// Con `ORDER BY creada DESC` ganaba la fila MÁS NUEVA, y eso deja tapar un «no». Puede haber más
// de una fila viva para el mismo (máquina, solicitante, capacidad): la puerta lee y después
// inserta, sin índice único, así que dos llamadas simultáneas abren dos solicitudes. Si a una la
// niegan y la otra queda pendiente, la pendiente —más nueva— ganaba y la negativa desaparecía.
//
// La precedencia es fail-closed: **negada gana siempre**, después concedida, y última pendiente.
// Un «no» no lo puede tapar nada.
//
// NO SE PONE UN ÍNDICE ÚNICO, y no es olvido: una fila vencida sigue diciendo `pendiente` —nadie
// la marca al vencer, se filtra por `vence`—, así que un único sobre (device, solicitante,
// capacidad) bloquearía TODAS las solicitudes futuras después de la primera. La duplicación es
// benigna con esta precedencia: dos concedidas exigieron dos segundas personas de verdad, y
// gastarlas sigue siendo de a una porque el `WHERE` de ConsumirAprobacion no admite carreras.
//
// Y UNA NEGADA CUENTA COMO VIGENTE, que es la parte que parece de más y no lo es. Si un «no»
// desapareciera de esta consulta, el siguiente intento abriría una solicitud nueva y el control
// se degradaría a «pedir hasta que alguien diga que sí» — que es exactamente cómo el cansancio
// vence a los cuatro ojos en cualquier organización. El «no» dura su ventana. Las `usada` sí
// salen: ésa ya abrió su sesión, y la próxima sesión necesita su propio permiso.
func (e *DbEngine) AprobacionVigenteDe(deviceID, solicitante string, cap fleet.Cap, ahora time.Time) (fleet.SolicitudDeAprobacion, bool, error) {
	t := ahora.UTC().Format(time.RFC3339)
	row := e.db.QueryRow(
		`SELECT `+columnasAprobacion+` FROM fleet_approvals
		  WHERE device_id = ? AND solicitante = ? AND capacidad = ?
		    AND estado IN ('pendiente', 'concedida', 'negada') AND vence > ?
		  ORDER BY CASE estado WHEN 'negada' THEN 0 WHEN 'concedida' THEN 1 ELSE 2 END,
		           creada DESC
		  LIMIT 1`,
		deviceID, solicitante, string(cap), t)
	s, err := escanearAprobacion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return fleet.SolicitudDeAprobacion{}, false, nil
	}
	if err != nil {
		return fleet.SolicitudDeAprobacion{}, false, err
	}
	return s, true, nil
}

// ResolverAprobacion registra el sí o el no de la segunda persona.
//
// El WHERE exige `estado = 'pendiente'` y la vigencia, así que la BASE decide y no una consulta
// previa: entre leer y escribir hay una carrera, y sin esto dos aprobadores simultáneos —o uno
// que llega justo cuando vence— dejarían la última escritura ganando en silencio.
func (e *DbEngine) ResolverAprobacion(id, aprobador, nota string, concede bool, ahora time.Time) (bool, error) {
	estado := fleet.AprobacionNegada
	if concede {
		estado = fleet.AprobacionConcedida
	}
	t := ahora.UTC().Format(time.RFC3339)
	res, err := e.db.Exec(
		`UPDATE fleet_approvals SET estado = ?, aprobador = ?, nota = ?, resuelta = ?
		  WHERE id = ? AND estado = 'pendiente' AND vence > ?`,
		string(estado), aprobador, fleet.RecortarRunas(strings.TrimSpace(nota), 500), t, id, t)
	if err != nil {
		return false, fmt.Errorf("error al resolver la solicitud %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado: %w", err)
	}
	return n > 0, nil
}

// ConsumirAprobacion gasta el permiso. Devuelve false si ya estaba gastado, vencido o sin conceder.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL UN SOLO USO LO GARANTIZA EL `WHERE`, NO EL LLAMADOR
//
// Un permiso reusable no es cuatro ojos: es una llave que la segunda persona entregó una vez y
// que después abre siempre. Comprobar el estado en Go y actualizar después dejaría la ventana
// clásica —dos sesiones simultáneas leen «concedida» y las dos abren—, y esa ventana no se ve en
// ninguna prueba secuencial.
//
// Por eso la condición viaja EN el UPDATE y el resultado es RowsAffected: la base es la única
// que puede decidirlo sin carrera, y el llamador tiene que negarse si le devuelve false.
func (e *DbEngine) ConsumirAprobacion(id string, ahora time.Time) (bool, error) {
	t := ahora.UTC().Format(time.RFC3339)
	res, err := e.db.Exec(
		`UPDATE fleet_approvals SET estado = 'usada', usada = ?
		  WHERE id = ? AND estado = 'concedida' AND vence > ?`, t, id, t)
	if err != nil {
		return false, fmt.Errorf("error al consumir la aprobación %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado: %w", err)
	}
	return n > 0, nil
}

// AprobacionesPendientes lista lo que está esperando una segunda persona.
//
// Es lo que mira quien puede aprobar —no hay notificación: el permiso no viaja a nadie— y lo que
// cuenta el exportador para la alerta de «hay alguien esperando». Las vencidas NO entran: una
// solicitud que ya no sirve en una lista de pendientes es ruido que enseña a no mirar la lista.
func (e *DbEngine) AprobacionesPendientes(projectID string, ahora time.Time, tope int) ([]fleet.SolicitudDeAprobacion, error) {
	if tope <= 0 {
		tope = 50
	}
	t := ahora.UTC().Format(time.RFC3339)
	args := []interface{}{t}
	q := `SELECT ` + columnasAprobacion + ` FROM fleet_approvals
	       WHERE estado = 'pendiente' AND vence > ?`
	if strings.TrimSpace(projectID) != "" {
		q += ` AND project_id = ?`
		args = append(args, projectID)
	}
	q += ` ORDER BY creada ASC LIMIT ?`
	args = append(args, tope)

	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar las aprobaciones pendientes: %w", err)
	}
	defer rows.Close()
	var out []fleet.SolicitudDeAprobacion
	for rows.Next() {
		s, err := escanearAprobacion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func escanearAprobacion(row escaneable) (fleet.SolicitudDeAprobacion, error) {
	var (
		s                              fleet.SolicitudDeAprobacion
		cap, estado                    string
		creada, vence, resuelta, usada string
	)
	if err := row.Scan(
		&s.ID, &s.DeviceID, &s.ProjectID, &s.Solicitante, &cap, &s.Motivo,
		&estado, &s.Aprobador, &s.Nota, &creada, &vence, &resuelta, &usada,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fleet.SolicitudDeAprobacion{}, err
		}
		return fleet.SolicitudDeAprobacion{}, fmt.Errorf("error al escanear una solicitud de aprobación: %w", err)
	}
	s.Capacidad = fleet.Cap(cap)
	s.Estado = fleet.EstadoAprobacion(estado)
	if t, ok := parseObsTime(creada); ok {
		s.Creada = t
	}
	if t, ok := parseObsTime(vence); ok {
		s.Vence = t
	}
	if t, ok := parseObsTime(resuelta); ok {
		s.Resuelta = t
	}
	if t, ok := parseObsTime(usada); ok {
		s.Usada = t
	}
	return s, nil
}
