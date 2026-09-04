package memory

// shell.go es la persistencia de la BITÁCORA DE SESIONES DE SHELL INTERACTIVA (S5b).
//
// Ninguna función de este archivo recibe, devuelve ni guarda un byte del CONTENIDO de la sesión.
// Es por construcción: no hay parámetro donde ponerlo. Grabar lo que alguien teclea en una
// terminal ajena es una decisión legal antes que técnica, y no se toma de rebote.

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"

	"github.com/google/uuid"
)

const columnasShell = `id, device_id, project_id, principal, estado, creada, vence, ultimo_trafico, cerrada, error, consentimiento`

// AbrirSesionShell registra que alguien pidió un prompt en una máquina ajena.
//
// Se escribe ANTES de intentar la conexión — misma regla que F1 de S5 y G7 de S6. Si el SSH
// nunca prende, el PEDIDO queda igual: que alguien haya intentado abrir una shell en un servidor
// es información de auditoría tanto como que lo haya logrado.
func (e *DbEngine) AbrirSesionShell(s fleet.SesionShell) (fleet.SesionShell, error) {
	if strings.TrimSpace(s.DeviceID) == "" || strings.TrimSpace(s.ProjectID) == "" {
		return fleet.SesionShell{}, fmt.Errorf("una sesión de shell necesita dispositivo y proyecto")
	}
	s.ID = uuid.NewString()
	// EL ESTADO LO PUEDE ELEGIR EL LLAMADOR, y sólo entre los dos que tienen sentido al nacer.
	//
	// `abriendo` es el camino normal; `esperando_permiso` es el de una máquina en `pide`, donde
	// la sesión se registra ANTES de tocar nada —para que el intento quede auditado igual— pero
	// no se abre ningún canal hasta que la persona conteste. Cualquier otro estado inicial sería
	// una fila que dice algo que no pasó, así que se normaliza en vez de confiar.
	if s.Estado != fleet.ShellEsperandoPermiso {
		s.Estado = fleet.ShellAbriendo
	}
	if s.Creada.IsZero() {
		s.Creada = time.Now().UTC()
	}
	if s.Vence.IsZero() {
		s.Vence = s.Creada.Add(fleet.ShellVidaMax)
	}
	// El reloj de INACTIVIDAD arranca al crearse y no en el primer byte. Si arrancara en el
	// primer byte, una sesión que se abre y a la que nadie se conecta jamás quedaría con el
	// campo vacío y sin techo de inactividad: viva hasta el techo de vida, que son dos horas.
	s.UltimoTrafico = s.Creada
	_, err := e.db.Exec(
		`INSERT INTO shell_sessions (id, device_id, project_id, principal, estado, creada, vence, ultimo_trafico, consentimiento)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')`,
		s.ID, s.DeviceID, s.ProjectID, s.Principal, string(s.Estado),
		s.Creada.UTC().Format(time.RFC3339), s.Vence.UTC().Format(time.RFC3339), s.UltimoTrafico.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fleet.SesionShell{}, fmt.Errorf("error al abrir la sesión de shell: %w", err)
	}
	return s, nil
}

// SesionShellPorID lee una sesión. La usa CADA request del stream, no sólo la que abre: una
// sesión que venció a mitad de un `tail -f` tiene que cortarse ahí (T6).
func (e *DbEngine) SesionShellPorID(id string) (fleet.SesionShell, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return fleet.SesionShell{}, false, nil
	}
	row := e.db.QueryRow(`SELECT `+columnasShell+` FROM shell_sessions WHERE id = ?`, id)
	s, err := escanearSesionShell(row)
	if err == sql.ErrNoRows {
		return fleet.SesionShell{}, false, nil
	}
	if err != nil {
		return fleet.SesionShell{}, false, fmt.Errorf("error al leer la sesión de shell: %w", err)
	}
	return s, true, nil
}

// TocarSesionShell mueve el reloj de inactividad y marca la sesión como activa.
//
// Se llama con CADA tramo de bytes, en cualquiera de las dos direcciones: una sesión donde
// `tail -f` escupe líneas está viva aunque nadie teclee, y una donde alguien teclea sin ver
// salida también. Mirar una sola dirección mataría una de las dos.
func (e *DbEngine) TocarSesionShell(id string, ahora time.Time) error {
	_, err := e.db.Exec(
		`UPDATE shell_sessions
		    SET ultimo_trafico = ?, estado = CASE WHEN estado = ? THEN ? ELSE estado END
		  WHERE id = ? AND cerrada IS NULL`,
		ahora.UTC().Format(time.RFC3339), string(fleet.ShellAbriendo), string(fleet.ShellActiva), strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("error al tocar la sesión de shell: %w", err)
	}
	return nil
}

// CerrarSesionShell la da por terminada. Idempotente: cerrar dos veces no cambia la primera
// fecha ni el primer motivo, porque el interesante es el primero — quién la cerró y por qué.
func (e *DbEngine) CerrarSesionShell(id string, estado fleet.EstadoShell, motivo string, ahora time.Time) error {
	_, err := e.db.Exec(
		`UPDATE shell_sessions SET estado = ?, cerrada = ?, error = ?
		  WHERE id = ? AND cerrada IS NULL`,
		string(estado), ahora.UTC().Format(time.RFC3339), motivo, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("error al cerrar la sesión de shell: %w", err)
	}
	return nil
}

// SesionShellAbiertaDe busca una sesión viva de esta persona en esta máquina (T7).
//
// Una sola por par: dos prompts simultáneos de la misma persona en la misma máquina es casi
// siempre una sesión olvidada más una nueva, y la olvidada es la peligrosa.
func (e *DbEngine) SesionShellAbiertaDe(principal, deviceID string, ahora time.Time) (fleet.SesionShell, bool, error) {
	rows, err := e.db.Query(
		`SELECT `+columnasShell+` FROM shell_sessions
		  WHERE device_id = ? AND principal = ? AND cerrada IS NULL
		  ORDER BY creada DESC LIMIT 10`, strings.TrimSpace(deviceID), strings.TrimSpace(principal))
	if err != nil {
		return fleet.SesionShell{}, false, fmt.Errorf("error al buscar sesiones de shell abiertas: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		s, err := escanearSesionShell(rows)
		if err != nil {
			return fleet.SesionShell{}, false, fmt.Errorf("error al escanear una sesión de shell: %w", err)
		}
		// El vencimiento se DERIVA, así que una fila sin cerrar puede estar muerta igual. Se
		// consulta el dominio y no la columna: preguntarle a `estado` daría por viva una sesión
		// que venció hace una hora y nadie fue a marcar.
		if s.Viva(ahora) {
			return s, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return fleet.SesionShell{}, false, fmt.Errorf("error al recorrer las sesiones de shell: %w", err)
	}
	return fleet.SesionShell{}, false, nil
}

// BitacoraDeShell devuelve las sesiones de un proyecto, más recientes primero.
func (e *DbEngine) BitacoraDeShell(projectID, deviceID string, tope int) ([]fleet.SesionShell, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || tope <= 0 {
		return nil, nil
	}
	q := `SELECT ` + columnasShell + ` FROM shell_sessions WHERE project_id = ?`
	args := []interface{}{projectID}
	if d := strings.TrimSpace(deviceID); d != "" {
		q += ` AND device_id = ?`
		args = append(args, d)
	}
	q += ` ORDER BY creada DESC LIMIT ?`
	args = append(args, tope)

	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al leer la bitácora de shell: %w", err)
	}
	defer rows.Close()
	var out []fleet.SesionShell
	for rows.Next() {
		s, err := escanearSesionShell(rows)
		if err != nil {
			return nil, fmt.Errorf("error al escanear una sesión de shell: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CerrarSesionesShellVencidas marca las que algún techo ya mató.
//
// El estado se DERIVA al leer, así que esto no cambia ninguna decisión: existe para que la
// bitácora no quede llena de filas «activas» de hace tres días, que es cómo una tabla de
// auditoría deja de ser legible. Cuelga del barrido de flota.
func (e *DbEngine) CerrarSesionesShellVencidas(ahora time.Time) (int64, error) {
	rows, err := e.db.Query(`SELECT ` + columnasShell + ` FROM shell_sessions WHERE cerrada IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("error al listar sesiones de shell abiertas: %w", err)
	}
	var muertas []fleet.SesionShell
	for rows.Next() {
		s, err := escanearSesionShell(rows)
		if err != nil {
			rows.Close()
			return 0, fmt.Errorf("error al escanear una sesión de shell: %w", err)
		}
		if vencida, _ := s.Vencida(ahora); vencida {
			muertas = append(muertas, s)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var n int64
	for _, s := range muertas {
		_, motivo := s.Vencida(ahora)
		if err := e.CerrarSesionShell(s.ID, fleet.ShellVencida, motivo, ahora); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func escanearSesionShell(row escaneable) (fleet.SesionShell, error) {
	var (
		s                                    fleet.SesionShell
		estado, creada, vence, ultimoTrafico string
		cerrada                              sql.NullString
		consentimiento                       string
	)
	if err := row.Scan(&s.ID, &s.DeviceID, &s.ProjectID, &s.Principal, &estado,
		&creada, &vence, &ultimoTrafico, &cerrada, &s.Error, &consentimiento); err != nil {
		return fleet.SesionShell{}, err
	}
	s.Estado = fleet.EstadoShell(estado)
	// Vacío es el valor de casi todas las filas y significa «no hizo falta preguntar». No se
	// traduce a nada acá, igual que en pantalla: interpretarlo obligaría a des-interpretarlo en
	// cada lectura.
	s.Consentimiento = fleet.RespuestaAviso(consentimiento)
	s.Creada, _ = time.Parse(time.RFC3339, creada)
	s.Vence, _ = time.Parse(time.RFC3339, vence)
	if ultimoTrafico != "" {
		s.UltimoTrafico, _ = time.Parse(time.RFC3339, ultimoTrafico)
	}
	if cerrada.Valid {
		s.Cerrada, _ = time.Parse(time.RFC3339, cerrada.String)
	}
	return s, nil
}

// ResponderConsentimientoDeShell registra CÓMO contestó el usuario de la máquina cuando la shell
// tuvo que pedirle permiso, y habilita o cierra la sesión según eso.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ES EL GEMELO DE ResponderConsentimiento Y COMPARTE SUS DOS GUARDAS, QUE NO SON DECORACIÓN
//
// `deviceID` sale del TOKEN del agente y la sesión tiene que ser SUYA: sin eso, una máquina
// comprometida contestaría «concedida» a la pregunta que se le hizo al usuario de OTRA, y el
// permiso de abrir una shell ajena se conseguiría sin tocar esa máquina.
//
// Y SÓLO SE PUEDE CONTESTAR UNA VEZ: la condición viaja en el WHERE. Sin eso, un agente podría
// mandar «negada» y después «concedida» —o repetir la respuesta después de que la sesión
// venció— y quedaría registrada la última, que es la que un atacante elegiría.
//
// CONCEDIDA NO ABRE NADA: la sesión pasa a `abriendo`, que es donde estaría si nunca hubiera
// hecho falta preguntar. El canal SSH se levanta después, en la segunda llamada de la tool, que
// vuelve a pasar por la compuerta de capacidades. Permiso y acceso siguen siendo dos pasos.
func (e *DbEngine) ResponderConsentimientoDeShell(deviceID, sesionID string, r fleet.RespuestaAviso,
	ahora time.Time) error {

	deviceID, sesionID = strings.TrimSpace(deviceID), strings.TrimSpace(sesionID)
	if deviceID == "" || sesionID == "" || !r.Valida() {
		return ErrComandoAjeno
	}
	estado := fleet.ShellSinPermiso
	var cerrada any = ahora.UTC().Format(time.RFC3339)
	if r.Concede() {
		estado, cerrada = fleet.ShellAbriendo, nil
	}
	res, err := e.db.Exec(
		`UPDATE shell_sessions SET estado = ?, consentimiento = ?, cerrada = ?
		  WHERE id = ? AND device_id = ? AND estado = ?`,
		string(estado), string(r), cerrada, sesionID, deviceID, string(fleet.ShellEsperandoPermiso))
	if err != nil {
		return fmt.Errorf("error al registrar el consentimiento de la shell %q: %w", sesionID, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		// Sesión ajena, inexistente o ya contestada: el MISMO error para las tres, por lo mismo
		// que en pantalla — distinguirlas sería un oráculo de qué sesiones existen en otras
		// máquinas.
		return ErrComandoAjeno
	}
	return nil
}

// SesionShellEsperandoDe busca el pedido de permiso VIGENTE de este principal sobre esta máquina.
//
// SE ACOTA AL PRINCIPAL, y no es una optimización: el permiso se le dio a QUIEN preguntó, y la
// pregunta que se le mostró a la persona lo nombraba. Sin este filtro, un operador aprovecharía
// el «sí» que le dieron a otro. Es la misma regla que sesionEsperandoDe sostiene en pantalla.
func (e *DbEngine) SesionShellEsperandoDe(deviceID, principal string, ahora time.Time) (fleet.SesionShell, bool, error) {
	deviceID, principal = strings.TrimSpace(deviceID), strings.TrimSpace(principal)
	if deviceID == "" || principal == "" {
		return fleet.SesionShell{}, false, nil
	}
	rows, err := e.db.Query(
		`SELECT `+columnasShell+` FROM shell_sessions
		  WHERE device_id = ? AND principal = ?
		    AND estado IN (?, ?, ?)
		  ORDER BY creada DESC LIMIT 10`,
		deviceID, principal,
		string(fleet.ShellEsperandoPermiso), string(fleet.ShellSinPermiso), string(fleet.ShellAbriendo))
	if err != nil {
		return fleet.SesionShell{}, false, fmt.Errorf("error al buscar el permiso de shell: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		s, err := escanearSesionShell(rows)
		if err != nil {
			return fleet.SesionShell{}, false, err
		}
		// UNA ESPERA VENCIDA NO CUENTA: se vuelve a preguntar. Si no, un pedido que nadie
		// contestó hace dos horas bloquearía todos los siguientes con un «ya se preguntó» que no
		// se va a resolver nunca. Misma decisión que en pantalla.
		if s.Estado == fleet.ShellEsperandoPermiso && !s.Vence.IsZero() && ahora.After(s.Vence) {
			continue
		}
		// NO SE FILTRAN LAS `abriendo` SIN CONSENTIMIENTO, y estuvo puesto un rato hasta que el
		// sabotaje mostró que era código muerto: una sesión normal en `abriendo` y sin cerrar la
		// devuelve T7 (SesionShellAbiertaDe) antes de que este camino corra, y una cerrada sale
		// de `abriendo`. La combinación que la guarda cubría no existe.
		//
		// Se saca en vez de dejarla «por las dudas»: una guarda inalcanzable con un comentario
		// que dice qué evita es una promesa que nadie puede comprobar, y este repo ya pagó una
		// —`cerrarSesionesColgadas`, un nombre que sólo existía en un comentario—.
		return s, true, nil
	}
	return fleet.SesionShell{}, false, rows.Err()
}
