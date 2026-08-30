package memory

// cronologia.go es la lectura ÚNICA de la línea de tiempo de una máquina: qué le hizo Musubi
// dentro de una ventana, cruzando los tres registros append-only que existen. Fase 5 · S11.
//
// El dominio —qué es un hecho, quién puede verlo y qué NO contiene esta lista— vive en
// internal/fleet/cronologia.go. Acá sólo se traduce de filas a hechos.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA VENTANA VA EN EL `WHERE`, Y ÉSA ES LA DECISIÓN QUE HACE QUE ESTO SIRVA
//
// La forma cómoda sería reusar las tres bitácoras existentes —que reciben un tope y no una
// ventana—, pedir de más y filtrar por fecha en Go. Es exactamente cómo se fabrica una respuesta
// que miente: pedir las 200 últimas filas de una máquina ocupada y filtrar «el martes» devuelve
// vacío si en los últimos días hubo más de 200 hechos, y ese vacío se lee como «el martes no pasó
// nada».
//
// Con la ventana en el WHERE, el tope corta lo que sobra DENTRO de la ventana pedida, que es un
// corte que se puede declarar (`truncado`) en vez de uno que se disimula.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA COMPARACIÓN DE FECHAS ES LÉXICA, Y ESO EXIGE QUE LO GUARDADO SEA CANÓNICO
//
// Las fechas se guardan como texto RFC3339. Comparar texto sólo funciona si todas las filas están
// en el mismo huso: `2026-08-29T22:00:00Z` y `2026-08-29T18:00:00-04:00` son el MISMO instante y
// se ordenan al revés como texto.
//
// Dos consultas que ya existían dependían de eso sin decirlo —el vencimiento de la cola
// (TomarComandos) y la poda de salidas—, y funcionaban porque en producción nadie pasa una fecha
// propia: los tres caminos reales dejan el campo en cero y el motor pone `time.Now().UTC()`. O
// sea que la garantía era una COSTUMBRE, no una regla.
//
// Al escribir esto se la convirtió en regla: los INSERT de comandos, sesiones, dispositivos y
// servicios normalizan con `.UTC()` antes de formatear. No cambia ningún comportamiento —Go
// compara instantes, no textos, al leer— y hace que la comparación léxica sea cierta por
// construcción y no por suerte.

import (
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// CronologiaDeDevice devuelve los hechos de UNA máquina dentro de la ventana, del más nuevo al
// más viejo. `truncado` dice si el tope cortó algo QUE ESTABA ADENTRO de la ventana.
//
// Se pide por deviceID y no por nombre: el nombre lo resuelve la compuerta, que ya tuvo que ir a
// buscar el device para decidir si esta credencial lo alcanza.
//
// EL TOPE SE APLICA POR FUENTE Y DESPUÉS AL TOTAL, y no es redundante: sin el tope por fuente,
// una máquina con miles de comandos y tres sesiones de pantalla devolvería sólo comandos —el
// ORDER BY global dejaría las sesiones fuera del corte aunque sean lo más reciente de su clase—.
// El resultado se leería como «acá no entró nadie», que es distinto de «entró, y no entró en la
// lista».
func (e *DbEngine) CronologiaDeDevice(projectID, deviceID string, v fleet.Ventana, tope int, ahora time.Time) ([]fleet.Hecho, bool, error) {
	projectID = strings.TrimSpace(projectID)
	deviceID = strings.TrimSpace(deviceID)
	if projectID == "" || deviceID == "" || tope <= 0 {
		return nil, false, nil
	}
	if err := v.Valida(); err != nil {
		return nil, false, err
	}
	// A LA GRANULARIDAD DE LA TABLA ANTES DE FORMATEAR. `Format(time.RFC3339)` tira la fracción
	// de segundo sin avisar, así que una punta con fracción se convierte en OTRA punta: la
	// consulta aplicaría un borde distinto del pedido. Normalizar acá lo hace explícito y, sobre
	// todo, hace que una ventana que termina «ahora» incluya lo que acaba de pasar.
	v = v.Normalizada()
	desde := v.Desde.UTC().Format(time.RFC3339)
	hasta := v.Hasta.UTC().Format(time.RFC3339)

	// El nombre de la máquina se resuelve UNA vez. Se pide CON revocadas: la cronología de una
	// máquina dada de baja sigue siendo un hecho de la auditoría, y perder su nombre justo ahí es
	// perderlo cuando más se necesita (mismo criterio que SesionesVivas).
	devices, err := e.ListarDevices(projectID, true)
	if err != nil {
		return nil, false, err
	}
	nombre := ""
	for _, d := range devices {
		if d.ID == deviceID {
			nombre = d.Name
			break
		}
	}

	var out []fleet.Hecho
	truncado := false

	// `>= desde` y `< hasta`: la ventana es semiabierta, igual que fleet.Ventana.Contiene. Dos
	// ventanas consecutivas no pueden contar dos veces el hecho del borde.
	comandos, err := e.hechosDeComandos(projectID, deviceID, desde, hasta, tope, nombre, ahora)
	if err != nil {
		return nil, false, err
	}
	if len(comandos) >= tope {
		truncado = true
	}
	out = append(out, comandos...)

	pantallas, err := e.hechosDePantalla(projectID, deviceID, desde, hasta, tope, nombre, ahora)
	if err != nil {
		return nil, false, err
	}
	if len(pantallas) >= tope {
		truncado = true
	}
	out = append(out, pantallas...)

	shells, err := e.hechosDeShell(projectID, deviceID, desde, hasta, tope, nombre)
	if err != nil {
		return nil, false, err
	}
	if len(shells) >= tope {
		truncado = true
	}
	out = append(out, shells...)

	fleet.OrdenarHechos(out)
	if len(out) > tope {
		out = out[:tope]
		truncado = true
	}
	return out, truncado, nil
}

func (e *DbEngine) hechosDeComandos(projectID, deviceID, desde, hasta string, tope int, nombre string, ahora time.Time) ([]fleet.Hecho, error) {
	rows, err := e.db.Query(
		`SELECT `+columnasComando+` FROM device_commands
		  WHERE project_id = ? AND device_id = ? AND creado >= ? AND creado < ?
		  ORDER BY creado DESC LIMIT ?`,
		projectID, deviceID, desde, hasta, tope)
	if err != nil {
		return nil, fmt.Errorf("error al leer los comandos de la cronología: %w", err)
	}
	defer rows.Close()
	var out []fleet.Hecho
	for rows.Next() {
		c, err := escanearComando(rows)
		if err != nil {
			return nil, err
		}
		// El vencimiento se DERIVA al leer, igual que el de las sesiones de pantalla. Sin esto,
		// un comando de una máquina cuyo agente no volvió figura `pendiente` para siempre —
		// medido: 50 comandos de 10 horas con una vida máxima de 15 minutos.
		c.Estado = c.EstadoActual(ahora)
		out = append(out, fleet.HechoDeComando(c, nombre))
	}
	return out, rows.Err()
}

func (e *DbEngine) hechosDePantalla(projectID, deviceID, desde, hasta string, tope int, nombre string, ahora time.Time) ([]fleet.Hecho, error) {
	rows, err := e.db.Query(
		`SELECT `+columnasSesion+` FROM screen_sessions
		  WHERE project_id = ? AND device_id = ? AND creada >= ? AND creada < ?
		  ORDER BY creada DESC LIMIT ?`,
		projectID, deviceID, desde, hasta, tope)
	if err != nil {
		return nil, fmt.Errorf("error al leer las sesiones de pantalla de la cronología: %w", err)
	}
	defer rows.Close()
	var out []fleet.Hecho
	for rows.Next() {
		s, err := escanearSesion(rows)
		if err != nil {
			return nil, err
		}
		// El vencimiento se DERIVA al leer, igual que en SesionesDePantalla. Sin esto la
		// cronología mostraría como `activa` una sesión que terminó hace un mes, que es la misma
		// mentira que el panel evita — y acá pesa más, porque una línea de tiempo se lee como el
		// relato de lo que pasó.
		if (s.Estado == fleet.SesionActiva || s.Estado == fleet.SesionSolicitada) && s.Vencida(ahora) {
			s.Estado = fleet.SesionVencida
		}
		out = append(out, fleet.HechoDeSesionPantalla(s, nombre))
	}
	return out, rows.Err()
}

func (e *DbEngine) hechosDeShell(projectID, deviceID, desde, hasta string, tope int, nombre string) ([]fleet.Hecho, error) {
	rows, err := e.db.Query(
		`SELECT `+columnasShell+` FROM shell_sessions
		  WHERE project_id = ? AND device_id = ? AND creada >= ? AND creada < ?
		  ORDER BY creada DESC LIMIT ?`,
		projectID, deviceID, desde, hasta, tope)
	if err != nil {
		return nil, fmt.Errorf("error al leer las sesiones de shell de la cronología: %w", err)
	}
	defer rows.Close()
	var out []fleet.Hecho
	for rows.Next() {
		s, err := escanearSesionShell(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, fleet.HechoDeSesionShell(s, nombre))
	}
	return out, rows.Err()
}
