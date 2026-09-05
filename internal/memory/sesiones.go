package memory

// sesiones.go es la persistencia de la BITÁCORA DE SESIONES DE PANTALLA (S6).
//
// Ninguna función de este archivo recibe, devuelve ni guarda una contraseña. Es la garantía G1 y
// se sostiene por construcción: no hay parámetro donde ponerla.

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"

	"github.com/google/uuid"
)

const columnasSesion = `id, device_id, project_id, principal, estado, creada, vence, cerrada, error, consentimiento`

// AbrirSesionPantalla registra que alguien pidió mirar una pantalla.
//
// Se escribe ANTES de que la contraseña llegue a la máquina (G7, misma regla que F1 de S5): si el
// agente nunca la aplica, el PEDIDO queda igual. Que alguien haya intentado mirar una pantalla es
// información de auditoría tanto como que lo haya logrado.
func (e *DbEngine) AbrirSesionPantalla(s fleet.SesionPantalla) (fleet.SesionPantalla, error) {
	if strings.TrimSpace(s.DeviceID) == "" || strings.TrimSpace(s.ProjectID) == "" {
		return fleet.SesionPantalla{}, fmt.Errorf("una sesión necesita dispositivo y proyecto")
	}
	s.ID = uuid.NewString()
	// EL ESTADO INICIAL LO DECIDE EL LLAMADOR, PERO SÓLO ENTRE DOS.
	//
	// Antes se pisaba con `solicitada` siempre, y estaba bien mientras ése era el único comienzo
	// posible. A57 agregó el otro: un `pide` nace en `esperando_permiso`, sin contraseña acuñada,
	// hasta que la persona conteste.
	//
	// La lista es CERRADA a propósito. Sin ella, un llamador podría abrir una sesión directamente
	// en `activa` —o en `sin_permiso`— y la bitácora registraría un acceso que nunca pasó por la
	// compuerta. Que los estados de llegada sólo se alcancen por transición es lo que hace que la
	// bitácora signifique algo.
	if s.Estado != fleet.SesionEsperandoPermiso {
		s.Estado = fleet.SesionSolicitada
	}
	if s.Creada.IsZero() {
		s.Creada = time.Now().UTC()
	}
	if s.Vence.IsZero() {
		s.Vence = s.Creada.Add(fleet.SesionDuracionDefault)
	}
	_, err := e.db.Exec(
		`INSERT INTO screen_sessions (id, device_id, project_id, principal, estado, creada, vence)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.DeviceID, s.ProjectID, s.Principal, string(s.Estado),
		s.Creada.UTC().Format(time.RFC3339), s.Vence.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fleet.SesionPantalla{}, fmt.Errorf("error al abrir la sesión de pantalla: %w", err)
	}
	return s, nil
}

// MarcarSesion cambia el estado de una sesión. `deviceID` es la GUARDA, igual que en los
// comandos: la sesión tiene que ser de esa máquina, y el llamador derivó ese id del TOKEN.
func (e *DbEngine) MarcarSesion(deviceID, sesionID string, estado fleet.EstadoSesion, errMsg string, ahora time.Time) error {
	deviceID, sesionID = strings.TrimSpace(deviceID), strings.TrimSpace(sesionID)
	if deviceID == "" || sesionID == "" {
		return ErrComandoAjeno
	}
	var dueno string
	err := e.db.QueryRow(`SELECT device_id FROM screen_sessions WHERE id = ?`, sesionID).Scan(&dueno)
	if err == sql.ErrNoRows {
		return ErrComandoAjeno
	}
	if err != nil {
		return fmt.Errorf("error al buscar la sesión %q: %w", sesionID, err)
	}
	if dueno != deviceID {
		return ErrComandoAjeno
	}
	var cerrada any
	if estado != fleet.SesionActiva {
		cerrada = ahora.UTC().Format(time.RFC3339)
	}
	_, err = e.db.Exec(
		`UPDATE screen_sessions SET estado = ?, error = ?, cerrada = ? WHERE id = ?`,
		string(estado), errMsg, cerrada, sesionID)
	if err != nil {
		return fmt.Errorf("error al marcar la sesión %q: %w", sesionID, err)
	}
	return nil
}

// ResponderConsentimiento registra CÓMO contestó el usuario de la máquina (A57) y cierra —o
// habilita— la sesión según eso.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA GUARDA ES LA MISMA QUE MarcarSesion, Y POR EL MISMO MOTIVO
//
// `deviceID` sale del TOKEN del agente, y la sesión tiene que ser SUYA. Sin esa comprobación,
// una máquina comprometida podría contestar «concedida» a la pregunta que se le hizo al usuario
// de OTRA — y el permiso de entrar a una pantalla ajena se conseguiría sin tocar esa máquina.
//
// SÓLO SE PUEDE CONTESTAR UNA VEZ, y esa condición está en el WHERE. Una sesión que ya salió de
// `esperando_permiso` no vuelve: sin eso, un agente podría mandar «negada» y después «concedida»
// —o repetir la respuesta después de que la sesión venció— y la bitácora registraría la última,
// que es exactamente la que un atacante elegiría.
func (e *DbEngine) ResponderConsentimiento(deviceID, sesionID string, r fleet.RespuestaAviso,
	ahora time.Time) error {

	deviceID, sesionID = strings.TrimSpace(deviceID), strings.TrimSpace(sesionID)
	if deviceID == "" || sesionID == "" || !r.Valida() {
		return ErrComandoAjeno
	}
	estado := fleet.SesionSinPermiso
	var cerrada any = ahora.UTC().Format(time.RFC3339)
	if r.Concede() {
		// CONCEDIDA NO ES ACTIVA: la sesión pasa a `solicitada`, que es donde estaría si nunca
		// hubiera hecho falta preguntar. Recién ahí se acuña la contraseña y se le manda a la
		// máquina — o sea que el permiso y la credencial siguen siendo dos pasos, y entre los dos
		// vuelve a pasar por la compuerta de capacidades.
		estado, cerrada = fleet.SesionSolicitada, nil
	}
	res, err := e.db.Exec(
		`UPDATE screen_sessions SET estado = ?, consentimiento = ?, cerrada = ?
		  WHERE id = ? AND device_id = ? AND estado = ?`,
		string(estado), string(r), cerrada, sesionID, deviceID, string(fleet.SesionEsperandoPermiso))
	if err != nil {
		return fmt.Errorf("error al registrar el consentimiento de %q: %w", sesionID, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		// La sesión no es de esta máquina, no existe, o ya se contestó. Las tres dan el MISMO
		// error, por el mismo motivo que motivoRechazo en la puerta del dispositivo: distinguirlas
		// convertiría esto en un oráculo de qué sesiones existen en otras máquinas.
		return ErrComandoAjeno
	}
	return nil
}

// SesionesDePantalla devuelve la bitácora de un proyecto, lo más reciente primero.
//
// El estado VENCIDO se deriva al leer, no se persiste: una columna que alguien tiene que ir a
// actualizar miente en cuanto nadie la actualiza — la misma lección que el `online` de S1.
func (e *DbEngine) SesionesDePantalla(projectID, deviceID string, tope int, ahora time.Time) ([]fleet.SesionPantalla, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || tope <= 0 {
		return nil, nil
	}
	q := `SELECT ` + columnasSesion + ` FROM screen_sessions WHERE project_id = ?`
	args := []any{projectID}
	if d := strings.TrimSpace(deviceID); d != "" {
		q += ` AND device_id = ?`
		args = append(args, d)
	}
	q += ` ORDER BY creada DESC LIMIT ?`
	args = append(args, tope)

	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al leer las sesiones de %q: %w", projectID, err)
	}
	defer rows.Close()
	var out []fleet.SesionPantalla
	for rows.Next() {
		s, err := escanearSesion(rows)
		if err != nil {
			return nil, err
		}
		// Derivado, no guardado: una sesión que pasó su ventana está vencida aunque nadie haya
		// venido a marcarla.
		//
		// Vale también para las SOLICITADAS, y ese caso importa: una sesión cuya contraseña
		// nunca llegó a la máquina —porque se apagó, porque el agente murió— pasada su ventana
		// está tan vencida como una que sí se usó. Dejarla en `solicitada` para siempre haría
		// que la bitácora se llene de sesiones que parecen pendientes y no lo están.
		if (s.Estado == fleet.SesionActiva || s.Estado == fleet.SesionSolicitada) && s.Vencida(ahora) {
			s.Estado = fleet.SesionVencida
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al recorrer las sesiones de %q: %w", projectID, err)
	}
	return out, nil
}

// GuardarRustdeskID registra el identificador público del cliente RustDesk de una máquina. Lo
// reporta el propio agente, y sólo puede tocar SU fila (misma disciplina que el autorreporte).
func (e *DbEngine) GuardarRustdeskID(deviceID, rid string) error {
	deviceID, rid = strings.TrimSpace(deviceID), strings.TrimSpace(rid)
	if deviceID == "" || rid == "" {
		return nil
	}
	if len(rid) > 32 {
		rid = rid[:32]
	}
	// EL CAMBIO SE ANOTA (S6b · A13). Un id que se mueve solo tiene dos explicaciones —se
	// reinstaló la máquina, o alguien está mintiendo— y las dos ameritan que quede escrito.
	//
	// Va en el MISMO UPDATE que el valor nuevo, con un CASE, y no en dos sentencias: entre un
	// SELECT y un UPDATE separados cabe otro latido de la misma máquina, y entonces el «previo»
	// que se guardaría sería el que acaba de escribir el otro. Con un solo UPDATE, SQLite lo
	// resuelve por fila y no hay hueco.
	//
	// Sólo se anota cuando el valor CAMBIA de uno no vacío a otro distinto: el primer reporte de
	// una máquina recién enrolada no es un cambio, es el estreno.
	_, err := e.db.Exec(`
		UPDATE devices SET
			rustdesk_id_previo = CASE
				WHEN rustdesk_id <> '' AND rustdesk_id <> ? THEN rustdesk_id
				ELSE rustdesk_id_previo END,
			rustdesk_id_cambiado = CASE
				WHEN rustdesk_id <> '' AND rustdesk_id <> ? THEN ?
				ELSE rustdesk_id_cambiado END,
			rustdesk_id = ?
		 WHERE id = ? AND revoked = 0`,
		rid, rid, time.Now().UTC().Format(time.RFC3339), rid, deviceID)
	if err != nil {
		return fmt.Errorf("error al registrar el id de RustDesk de %q: %w", deviceID, err)
	}
	return nil
}

// QuienMasDiceSer devuelve los NOMBRES de las otras máquinas que reportan el mismo rustdesk_id, y
// cuántas de ellas quedan fuera del alcance de quien pregunta.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ LA COLISIÓN ES LA GUARDA QUE SIRVE, Y NO INTERROGAR AL RELAY
//
// El plan original decía «verificar el rustdesk_id contra el relay». No es viable ni serviría:
// hbbs (el relay OSS de RustDesk) no expone ninguna API para eso —habría que hablarle su
// protobuf, que es reimplementar medio cliente— y aunque la expusiera, sólo diría qué CONEXIÓN
// reclama ese id ahora mismo. No diría cuál de nuestras máquinas es.
//
// La colisión sí ataca el caso real, y ataca DOS a la vez:
//
//   - EL MALICIOSO: una máquina comprometida declara el id de otra para que un operador abra la
//     pantalla equivocada. Al declararlo, COLISIONA con la verdadera. Es su firma.
//   - EL BENIGNO Y MUCHO MÁS FRECUENTE: dos máquinas clonadas de la misma imagen. RustDesk deriva
//     su id de características de la máquina, así que los clones nacen con el mismo. Y ahí
//     conectarse ya era una moneda al aire, sólo que en silencio.
//
// SE MIRA GLOBALMENTE Y SE INFORMA CON CUIDADO. Acotar la consulta al proyecto de quien pregunta
// dejaría pasar el caso peor —dos tenants distintos con el mismo id, donde un operador aterriza
// en la máquina de otra empresa— pero nombrar una máquina ajena rompería el aislamiento. Así que
// se cuentan aparte: los nombres del propio alcance, y un CONTEO de las de afuera. Alcanza para
// decir «este id es ambiguo, no te fíes» sin decir de quién.
// ────────────────────────────────────────────────────────────────────────────────────────────
func (e *DbEngine) QuienMasDiceSer(deviceID, rid, projectID string) (nombres []string, fuera int, err error) {
	rid = strings.TrimSpace(rid)
	if rid == "" {
		return nil, 0, nil // sin id no hay colisión posible
	}
	rows, err := e.db.Query(
		`SELECT name, project_id FROM devices
		  WHERE rustdesk_id = ? AND id <> ? AND revoked = 0`,
		rid, strings.TrimSpace(deviceID))
	if err != nil {
		return nil, 0, fmt.Errorf("error al buscar colisiones de rustdesk_id: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var nombre, proy string
		if err := rows.Scan(&nombre, &proy); err != nil {
			return nil, 0, fmt.Errorf("error al escanear una colisión: %w", err)
		}
		if proy == strings.TrimSpace(projectID) {
			nombres = append(nombres, nombre)
		} else {
			fuera++
		}
	}
	return nombres, fuera, rows.Err()
}

func escanearSesion(row escaneable) (fleet.SesionPantalla, error) {
	var (
		s                     fleet.SesionPantalla
		estado, creada, vence string
		cerrada               sql.NullString
		consentimiento        string
	)
	if err := row.Scan(&s.ID, &s.DeviceID, &s.ProjectID, &s.Principal, &estado, &creada, &vence,
		&cerrada, &s.Error, &consentimiento); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fleet.SesionPantalla{}, err
		}
		return fleet.SesionPantalla{}, fmt.Errorf("error al escanear una sesión: %w", err)
	}
	s.Estado = fleet.EstadoSesion(estado)
	// Vacío es el valor de casi todas las filas y significa «no hizo falta preguntar». No se
	// traduce a nada: interpretarlo acá obligaría a des-interpretarlo en cada lectura.
	s.Consentimiento = fleet.RespuestaAviso(consentimiento)
	if t, ok := parseObsTime(creada); ok {
		s.Creada = t
	}
	if t, ok := parseObsTime(vence); ok {
		s.Vence = t
	}
	if cerrada.Valid {
		if t, ok := parseObsTime(cerrada.String); ok {
			s.Cerrada = t
		}
	}
	return s, nil
}
