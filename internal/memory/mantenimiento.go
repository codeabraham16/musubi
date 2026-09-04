package memory

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"musubi/internal/fleet"
)

// AbrirMantenimiento declara una ventana. Append-only: no actualiza nada.
func (e *DbEngine) AbrirMantenimiento(m fleet.Mantenimiento) (fleet.Mantenimiento, error) {
	if err := fleet.ValidarMantenimiento(m); err != nil {
		return fleet.Mantenimiento{}, err
	}
	// El id lo asigna el cerebro y se ignora lo que traiga el llamador, por la misma razón que en
	// AltaDevice: un id elegido por quien pide es un id que se puede pisar.
	m.ID = uuid.NewString()
	m.Creado = time.Now().UTC()
	if _, err := e.db.Exec(
		`INSERT INTO device_maintenance (id, device_id, project_id, principal, desde, hasta, motivo, cancelada, creado)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		m.ID, m.DeviceID, m.ProjectID, m.Principal,
		m.Desde.UTC().Format(time.RFC3339), m.Hasta.UTC().Format(time.RFC3339),
		strings.TrimSpace(m.Motivo), m.Creado.Format(time.RFC3339),
	); err != nil {
		return fleet.Mantenimiento{}, fmt.Errorf("error al abrir la ventana de mantenimiento: %w", err)
	}
	return m, nil
}

// CancelarMantenimiento retira una ventana. Marca la fila, no la borra: la cronología se
// construye sólo sobre tablas append-only, y «lo cancelaron a los diez minutos» explica el
// comportamiento de esa máquina mejor que la ausencia de toda fila.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DUEÑO DE LA FILA VA EN EL WHERE, NO EN LA COMPUERTA DE ARRIBA
//
// Esta función tomaba sólo el id y escribía `WHERE id = ? AND cancelada = 0`. El id de una
// ventana es global, y su único llamador autoriza contra una máquina QUE NOMBRA QUIEN LLAMA:
// alcanzaba con tener `metrics` sobre una máquina propia y conocer un id ajeno para cancelar la
// ventana de otro cliente. Media compuerta —mira una máquina, escribe otra— es la misma forma
// que ya nos costó `renderAprobaciones`.
//
// Y no es una fuga de lectura. Una ventana no sólo calla alertas: FRENA EL AUTO-HEAL. Cancelar
// la de otro devuelve la automatización sobre una máquina que alguien puso a resguardo a
// propósito, en mitad de su mantenimiento, y del otro lado eso se ve como un servicio que se
// levanta solo sin que nadie lo haya pedido.
//
// La reparación no es agregar la comprobación en el llamador: es que el dueño viaje EN EL
// UPDATE. Un WHERE que no puede alcanzar filas ajenas no depende de que el próximo camino se
// acuerde de comprobar, y `RowsAffected() == 0` deja el fallo cerrado sin caso especial.
func (e *DbEngine) CancelarMantenimiento(deviceID, projectID, id string) (bool, error) {
	res, err := e.db.Exec(
		`UPDATE device_maintenance SET cancelada = 1
		  WHERE id = ? AND device_id = ? AND project_id = ? AND cancelada = 0`,
		id, deviceID, projectID)
	if err != nil {
		return false, fmt.Errorf("error al cancelar la ventana %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado de la cancelación: %w", err)
	}
	return n > 0, nil
}

// DevicesEnMantenimiento devuelve los ids de máquina con una ventana activa AHORA.
//
// Devuelve un conjunto y no una lista de ventanas porque los dos llamadores —el scheduler y el
// exportador— preguntan lo mismo: «¿ésta está en ventana?». Traer las ventanas enteras los
// obligaría a repetir la comparación de bordes, y dos copias de una comparación de bordes se
// separan.
func (e *DbEngine) DevicesEnMantenimiento(ahora time.Time) (map[string]bool, error) {
	t := ahora.UTC().Format(time.RFC3339)
	rows, err := e.db.Query(
		`SELECT DISTINCT device_id FROM device_maintenance
		  WHERE cancelada = 0 AND desde <= ? AND hasta > ?`, t, t)
	if err != nil {
		return nil, fmt.Errorf("error al listar las máquinas en mantenimiento: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("error al leer una fila de mantenimiento: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// MantenimientosDeDevice trae las ventanas de una máquina, la más reciente primero.
func (e *DbEngine) MantenimientosDeDevice(deviceID string, tope int) ([]fleet.Mantenimiento, error) {
	if tope <= 0 {
		tope = 50
	}
	rows, err := e.db.Query(
		`SELECT id, device_id, project_id, principal, desde, hasta, motivo, cancelada, creado
		   FROM device_maintenance WHERE device_id = ? ORDER BY desde DESC LIMIT ?`, deviceID, tope)
	if err != nil {
		return nil, fmt.Errorf("error al listar las ventanas de %q: %w", deviceID, err)
	}
	defer rows.Close()
	var out []fleet.Mantenimiento
	for rows.Next() {
		var m fleet.Mantenimiento
		var desde, hasta, creado string
		var cancelada int
		if err := rows.Scan(&m.ID, &m.DeviceID, &m.ProjectID, &m.Principal, &desde, &hasta, &m.Motivo, &cancelada, &creado); err != nil {
			return nil, fmt.Errorf("error al leer una ventana: %w", err)
		}
		m.Cancelada = cancelada != 0
		m.Desde, _ = time.Parse(time.RFC3339, desde)
		m.Hasta, _ = time.Parse(time.RFC3339, hasta)
		m.Creado, _ = time.Parse(time.RFC3339, creado)
		out = append(out, m)
	}
	return out, rows.Err()
}
