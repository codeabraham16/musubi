package memory

// servicios.go es la PERSISTENCIA del inventario de SERVICIOS de la flota (slice S12): qué corre
// adentro de cada máquina. El dominio —qué es un servicio, qué significa `desconocido`, cuándo un
// reporte es viejo— vive en internal/fleet y no sabe que esta base existe.
//
// La tabla la crea la migración 36. Lo que esa migración NO tiene —columna de estado, serie
// temporal, foreign key— está explicado ahí y se sostiene acá: no hay un solo método en este
// archivo que escriba un booleano de salud ni que haga DELETE.
//
// LA REGLA QUE GOBIERNA TODO EL ARCHIVO: la escritura del AGENTE lleva SIEMPRE `AND device_id =
// ?`, con el id derivado del TOKEN. Sin esa guarda, cualquier máquina de la flota puede reportar
// que el postgres de producción está caído — y eso no es un error de datos, es uno de seguridad.

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"

	"github.com/google/uuid"
)

// columnasServicio es la lista de columnas en el orden que espera escanearServicio. Una sola
// copia: que el SELECT y el Scan se desincronicen es el bug clásico de esta capa.
const columnasServicio = `id, name, project_id, device_id, kind, registered_at, last_report, last_health, revoked, declared`

// AltaServicio registra un servicio A MANO: un Tier B que nadie enumera solo, un bot, un puente.
//
// EL project_id SALE DEL DEVICE, no del pedido. Se resuelve la máquina por su id y se copia su
// proyecto, IGNORANDO lo que declare el llamador. Sin foreign keys, un servicio atribuido a un
// proyecto distinto del de su máquina es perfectamente representable — y esa desalineación es una
// fuga de tenant con la forma exacta de A6: la fila aparecería listando el proyecto equivocado.
//
// Un device REVOCADO no admite servicios nuevos: la fila sigue existiendo para la auditoría, pero
// dejó de ser una máquina sobre la que se opera.
//
// LA FILA NACE `declared = 1`, y eso es lo que la pone fuera del alcance de la poda por ausencia:
// lo que una persona declaró lo saca una persona, nunca el silencio de un latido. Ver la
// migración 37 y PodarServiciosAusentes.
//
// Y SI LA FILA YA EXISTE PERO ESTÁ REVOCADA, SE REVIVE en vez de rebotar. Sin eso no había salida:
// la fila revocada sigue ocupando el único (project_id, device_id, name), así que redeclarar
// chocaba contra el índice con un «ya existe un servicio con ese nombre» que, desde donde mira
// quien opera, es MENTIRA —el listado por defecto no lo muestra— y encima no decía qué hacer.
// Revivir acá no contradice al agente: un REPORTE sigue sin resucitar nada (ver ReportarServicios),
// porque volver al inventario tiene que ser la decisión de alguien, y esto es exactamente eso.
func (e *DbEngine) AltaServicio(s fleet.Servicio) (fleet.Servicio, error) {
	s.DeviceID = strings.TrimSpace(s.DeviceID)
	d, existe, err := e.DevicePorID(s.DeviceID)
	if err != nil {
		return fleet.Servicio{}, err
	}
	// El MISMO mensaje para «no existe» y «está revocado», por el mismo motivo que motivoRechazo
	// en la puerta del dispositivo: distinguirlos convierte esto en un oráculo de qué máquinas
	// existen en otro tenant. El llamador de arriba ya acotó la búsqueda a un proyecto.
	if !existe || d.Revoked {
		return fleet.Servicio{}, fmt.Errorf("no hay una máquina activa con ese identificador: dala de alta con musubi_fleet_enroll, o revisá el nombre")
	}
	// Acá está la copia que sostiene el invariante: el proyecto es el del DEVICE, punto.
	s.ProjectID = d.ProjectID
	s.Nombre = strings.TrimSpace(s.Nombre)
	s.Clase = strings.ToLower(strings.TrimSpace(s.Clase))
	if err := fleet.ValidarAltaServicio(s); err != nil {
		return fleet.Servicio{}, err
	}
	// El id lo asigna el CEREBRO, igual que el de un Device (A1): si viene con algo, se ignora.
	s.ID = uuid.NewString()
	s.Revocado = false
	s.UltimoReporte = time.Time{} // nunca reportó: DECLARADO y todavía sin muestras
	s.Salud = nil
	if s.Registrado.IsZero() {
		s.Registrado = time.Now().UTC()
	}

	s.Declarado = true

	_, err = e.db.Exec(
		`INSERT INTO services (id, name, project_id, device_id, kind, registered_at, last_report, last_health, revoked, declared)
		 VALUES (?, ?, ?, ?, ?, ?, NULL, '', 0, 1)`,
		s.ID, s.Nombre, s.ProjectID, s.DeviceID, s.Clase, s.Registrado.UTC().Format(time.RFC3339),
	)
	if err != nil {
		if esViolacionDeUnicidad(err) {
			return e.revivirServicioDeclarado(s, d)
		}
		return fleet.Servicio{}, fmt.Errorf("error al dar de alta el servicio %q: %w", s.Nombre, err)
	}
	return s, nil
}

// revivirServicioDeclarado resuelve el choque contra el índice único mirando POR QUÉ chocó.
//
// Si la fila que ocupa el nombre está REVOCADA, esta alta la trae de vuelta: es la única salida
// que tiene quien declaró un servicio, lo perdió (por una poda o por una baja) y lo quiere
// devuelta. Vuelve como NACE una declaración —sin último reporte y sin salud—, porque la salud que
// tenía cuando la revocaron es de otra época y mostrarla como presente sería mentir.
//
// Si la fila está VIVA, sí es un duplicado de verdad, y el mensaje dice qué hacer en vez de sólo
// qué pasó.
func (e *DbEngine) revivirServicioDeclarado(s fleet.Servicio, d fleet.Device) (fleet.Servicio, error) {
	res, err := e.db.Exec(
		`UPDATE services SET revoked = 0, declared = 1, kind = ?, registered_at = ?, last_report = NULL, last_health = ''
		  WHERE project_id = ? AND device_id = ? AND name = ? AND revoked = 1`,
		s.Clase, s.Registrado.UTC().Format(time.RFC3339), s.ProjectID, s.DeviceID, s.Nombre)
	if err != nil {
		return fleet.Servicio{}, fmt.Errorf("error al reactivar el servicio %q: %w", s.Nombre, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		// El mensaje NO manda a «darlo de baja y volver a declararlo»: no hay ninguna tool que dé
		// de baja UN servicio, y mandar a alguien a hacer algo que no se puede es peor que no
		// decir nada. Se dice lo único que sí puede hacer: mirarlo.
		return fleet.Servicio{}, fmt.Errorf("%w: %q en %q. Ya está en el inventario y ACTIVO, así que no hace falta "+
			"declararlo de nuevo: miralo con musubi_fleet_services (device=%q). Si no te aparece en el listado, "+
			"estás mirando otro proyecto — revisá el `project` del pedido",
			fleet.ErrServicioDuplicado, s.Nombre, d.Name, d.Name)
	}
	// El id es el de la fila que ya estaba: se conserva a propósito, porque perderlo rompería
	// cualquier referencia vieja a un servicio que —para quien opera— es el mismo de siempre.
	var id string
	if err := e.db.QueryRow(
		`SELECT id FROM services WHERE project_id = ? AND device_id = ? AND name = ?`,
		s.ProjectID, s.DeviceID, s.Nombre).Scan(&id); err == nil {
		s.ID = id
	}
	s.Declarado = true
	s.Revocado = false
	return s, nil
}

// ReportarServicios es la escritura del AGENTE: lo que una máquina dice de lo que corre adentro
// suyo. Devuelve cuántos se crearon y cuántos se actualizaron.
//
// `deviceID` viene del TOKEN, nunca del cuerpo, y TODO lo que se toca acá lleva
// `AND device_id = ?`. Es la guarda que impide que una máquina comprometida reporte que el
// postgres de OTRA está caído: un error de seguridad, no de datos.
//
// Es un UPSERT por (device_id, name): lo que no existía se crea —el agente descubre servicios
// solo— y lo que existía se actualiza. Un servicio que nunca nadie declaró a mano aparece por
// este camino, y eso es lo que se quiere: el inventario a mano es para lo que NO se enumera.
func (e *DbEngine) ReportarServicios(deviceID string, ahora time.Time, reportes []fleet.ReporteServicio) (int, int, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || len(reportes) == 0 {
		return 0, 0, nil
	}
	d, existe, err := e.DevicePorID(deviceID)
	if err != nil {
		return 0, 0, err
	}
	if !existe || d.Revoked {
		// No es un error del servidor: es una máquina revocada que todavía no se enteró, igual
		// que en LatirDevice. El llamador ya decidió qué hacer con el latido.
		return 0, 0, nil
	}

	tx, err := e.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("error al abrir la transacción del reporte de servicios: %w", err)
	}
	defer tx.Rollback()

	nuevos, actualizados := 0, 0
	for _, r := range reportes {
		r = fleet.RecortarReporte(r)
		if !fleet.NombreDeServicioValido(r.Nombre) {
			continue // uno inválido se saltea; no tumba a los demás
		}
		// UN NOMBRE VÁLIDO CON UNA SALUD QUE NO SE PUEDE INTERPRETAR NO SE DESCARTA: se guarda
		// el SERVICIO con la salud VACÍA. Es la asimetría D7 un nivel más abajo — «el servicio
		// existe» y «la máquina supo medirlo» son cosas distintas, igual que «la máquina está
		// viva» y «la máquina supo medirse».
		//
		// Importa de verdad y no es una sutileza: `systemctl list-units` da los nombres y
		// `systemctl show` puede fallar por permisos en la misma corrida. Descartando el reporte
		// entero, esa máquina no tendría inventario NUNCA y nadie sabría por qué; guardándolo, el
		// servicio aparece como `desconocido`, que es exactamente lo que es.
		salud := ""
		if err := r.Salud.Valida(); err == nil {
			if txt, err := r.Salud.Serializar(); err == nil {
				salud = txt
			}
		}

		// LA ESCRITURA, con el mismo CASE que LatirDevice: una salud vacía NO borra la anterior.
		// Ésa es la otra mitad — el servicio que hoy no se pudo medir conserva la última medición
		// buena y avanza su `last_report`, en vez de perder las dos cosas a la vez.
		//
		// Y `revoked = 0`: UN REPORTE RESUCITA LO QUE LA PODA SE LLEVÓ, pero sólo eso.
		//
		// La versión anterior no resucitaba nada, y el efecto en producción fue que el inventario
		// resultó ser un TRINQUETE: sólo podía achicarse. Un `podman ps` que falló UNA vez —o un
		// arranque en el que el runtime todavía no estaba— mandaba un inventario sin los 18
		// contenedores, la poda por ausencia los daba de baja, y a partir de ahí la máquina los
		// reportaba en cada latido para siempre sin que ninguno volviera. No había error en
		// ningún lado: 18 filas revocadas, 18 reportes descartados en silencio.
		//
		// La asimetría era el error: podar por ausencia y NO despodar por presencia. Si la fila
		// está acá porque la máquina la reporta (`declared = 0`) y se fue porque la máquina dejó
		// de reportarla, entonces que la reporte de nuevo es exactamente la condición que la
		// trajo la primera vez.
		//
		// Lo que NO resucita es la fila que puso una PERSONA (`declared = 1`): ésa se da de alta
		// de nuevo por `fleet_service_declare`, que es una decisión de alguien. Ahí sí vale el
		// «que vuelva a aparecer tiene que ser una decisión» — el error era aplicárselo también a
		// la mitad que nadie decidió.
		res, err := tx.Exec(
			`UPDATE services SET last_report = ?,
			        last_health = CASE WHEN ? = '' THEN last_health ELSE ? END,
			        kind        = CASE WHEN ? = '' THEN kind        ELSE ? END,
			        revoked     = 0
			  WHERE name = ? AND device_id = ? AND (revoked = 0 OR declared = 0)`,
			ahora.UTC().Format(time.RFC3339), salud, salud, r.Clase, r.Clase, r.Nombre, deviceID)
		if err != nil {
			return 0, 0, fmt.Errorf("error al reportar el servicio %q: %w", r.Nombre, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, 0, fmt.Errorf("error al leer el resultado del reporte de %q: %w", r.Nombre, err)
		}
		if n > 0 {
			actualizados++
			continue
		}
		// No existía (o está revocado y no se resucita por un reporte): se crea. El project_id
		// sale del device, igual que en el alta a mano — el reporte no tiene por dónde declararlo.
		if _, err := tx.Exec(
			// `declared = 0`: esta fila la trajo la MÁQUINA, así que la poda por ausencia puede
			// sacarla. Es la mitad simétrica del `declared = 1` del alta a mano.
			`INSERT INTO services (id, name, project_id, device_id, kind, registered_at, last_report, last_health, revoked, declared)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0)`,
			uuid.NewString(), r.Nombre, d.ProjectID, deviceID, r.Clase,
			ahora.UTC().Format(time.RFC3339), ahora.UTC().Format(time.RFC3339), salud,
		); err != nil {
			if esViolacionDeUnicidad(err) {
				// La fila existe, revocada, y la puso una PERSONA (si fuera de la máquina, el
				// UPDATE de arriba ya la habría resucitado y no estaríamos acá). Que vuelva a
				// aparecer en el inventario es una decisión de alguien, no el efecto de que la
				// máquina siga reportándola.
				continue
			}
			return 0, 0, fmt.Errorf("error al registrar el servicio %q: %w", r.Nombre, err)
		}
		nuevos++
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("error al confirmar el reporte de servicios: %w", err)
	}
	return nuevos, actualizados, nil
}

// ReportarSaludDeServicios actualiza la salud de servicios QUE YA EXISTEN en esta máquina, sin
// crear ninguno y sin podar nada. Devuelve cuántos se actualizaron y los nombres que no existen.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ NO CREA, Y NO ES PRUDENCIA SINO UN BUG EVITADO
//
// El camino del latido crea con `declared = 0` —esa fila la trajo la máquina, así que la poda por
// ausencia puede sacarla—. Si este camino creara igual, el bot nacería con `declared = 0` y **el
// siguiente latido del agente lo podaría**: el agente enumera systemd y contenedores, y el bot no
// está en ninguno de los dos. El colector lo recrearía un minuto después, el agente lo podaría de
// nuevo, y el servicio aparecería y desaparecería del panel para siempre sin que nada dijera por
// qué.
//
// Un bot entra al inventario por `musubi_fleet_service_declare`, que lo marca `declared = 1` y lo
// hace inmune a esa poda. Esta función sólo le pone salud a lo que alguien ya decidió que existe.
//
// TAMPOCO ESTAMPA SEÑAL DE VIDA, y eso es lo otro que la separa del latido: si un reporte de salud
// marcara viva a la máquina, un host cuyo AGENTE murió pero cuyo colector sigue corriendo figuraría
// sano. La vida de la máquina la afirma quien la mide; ésta afirma otra cosa.
func (e *DbEngine) ReportarSaludDeServicios(deviceID string, ahora time.Time,
	reportes []fleet.ReporteServicio) (actualizados int, desconocidos []string, err error) {

	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || len(reportes) == 0 {
		return 0, nil, nil
	}
	d, existe, err := e.DevicePorID(deviceID)
	if err != nil {
		return 0, nil, err
	}
	if !existe || d.Revoked {
		return 0, nil, nil
	}

	tx, err := e.db.Begin()
	if err != nil {
		return 0, nil, fmt.Errorf("error al abrir la transacción del reporte de salud: %w", err)
	}
	defer tx.Rollback()

	for _, r := range reportes {
		r = fleet.RecortarReporte(r)
		if !fleet.NombreDeServicioValido(r.Nombre) {
			continue
		}
		// LA SALUD SE VALIDA Y SE DESCARTA ENTERA SI NO SE PUEDE CREER. Acá NO vale la asimetría
		// del latido —guardar el servicio con la salud vacía—, porque el servicio ya existe: no
		// hay nada que crear, y pisar una salud buena con una vacía sería perder el último dato
		// bueno por culpa de un reporte roto.
		if err := r.Salud.Valida(); err != nil {
			desconocidos = append(desconocidos, r.Nombre+": "+err.Error())
			continue
		}
		salud, err := r.Salud.Serializar()
		if err != nil {
			return 0, nil, fmt.Errorf("error al serializar la salud de %q: %w", r.Nombre, err)
		}
		// `revoked = 0` NO se toca: un servicio dado de baja no vuelve por un reporte de salud.
		// Es la misma regla que el latido aplica a lo declarado, y por el mismo motivo — que algo
		// vuelva al inventario es una decisión de alguien.
		res, err := tx.Exec(
			`UPDATE services SET last_report = ?, last_health = ?
			  WHERE name = ? AND device_id = ? AND revoked = 0`,
			ahora.UTC().Format(time.RFC3339), salud, r.Nombre, deviceID)
		if err != nil {
			return 0, nil, fmt.Errorf("error al reportar la salud de %q: %w", r.Nombre, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, nil, fmt.Errorf("error al leer el resultado de %q: %w", r.Nombre, err)
		}
		if n == 0 {
			// SE DICE CUÁL, y no se traga en silencio. Un colector apuntando a un nombre que no
			// existe es el error más probable de este camino —un typo, un servicio que nadie
			// declaró— y su síntoma sin esto sería un panel que nunca cambia.
			desconocidos = append(desconocidos, r.Nombre)
			continue
		}
		actualizados++
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, fmt.Errorf("error al confirmar el reporte de salud: %w", err)
	}
	return actualizados, desconocidos, nil
}

// ListarServicios devuelve los servicios de UN proyecto, opcionalmente los de UNA máquina.
//
// SOBRE LA GUARDA DE projectID VACÍO: no es lo que impide devolver «todos» —el WHERE ya lo
// impide—, es lo que evita que un llamador que se olvidó el proyecto reciba justo las filas
// HUÉRFANAS: las que no pertenecen a nadie y por eso mismo no debería ver. Es la misma guarda que
// ListarDevices, y la fija una prueba que INSERTA una huérfana a mano para que tenga algo que
// tapar.
//
// El aislamiento va por el project_id DENORMALIZADO de la fila, NUNCA por un JOIN a `devices`:
// es lo que ya hacen device_commands y screen_sessions, y un JOIN ataría cada lectura a que la
// fila de la máquina siga existiendo con el mismo proyecto.
func (e *DbEngine) ListarServicios(projectID, deviceID string, incluirRevocados bool) ([]fleet.Servicio, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, nil
	}
	q := `SELECT ` + columnasServicio + ` FROM services WHERE project_id = ?`
	args := []any{projectID}
	if deviceID = strings.TrimSpace(deviceID); deviceID != "" {
		q += ` AND device_id = ?`
		args = append(args, deviceID)
	}
	if !incluirRevocados {
		q += ` AND revoked = 0`
	}
	q += ` ORDER BY device_id, name`
	return e.consultarServicios(q, args...)
}

// ServiciosDeDevice devuelve los servicios ACTIVOS de una máquina, sin pasar por el proyecto.
//
// NO lleva projectID en la firma a propósito, igual que DevicePorID: el id es interno y ya viene
// de una fila que se leyó bajo el proyecto correcto. Meter el proyecto acá daría la falsa
// impresión de que esta función aísla, y no es su trabajo — el llamador compuertea después.
func (e *DbEngine) ServiciosDeDevice(deviceID string) ([]fleet.Servicio, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, nil
	}
	return e.consultarServicios(
		`SELECT `+columnasServicio+` FROM services WHERE device_id = ? AND revoked = 0 ORDER BY name`, deviceID)
}

// RevocarServiciosDeDevice saca del inventario los servicios de una máquina dada de baja.
//
// Las filas QUEDAN, como la del device: perder a qué máquina pertenecía un servicio es justo lo
// que no querés después de un incidente, que es cuando uno revoca.
func (e *DbEngine) RevocarServiciosDeDevice(deviceID string) (int64, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, nil
	}
	res, err := e.db.Exec(`UPDATE services SET revoked = 1 WHERE device_id = ? AND revoked = 0`, deviceID)
	if err != nil {
		return 0, fmt.Errorf("error al revocar los servicios de %q: %w", deviceID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer el resultado de revocar los servicios de %q: %w", deviceID, err)
	}
	return n, nil
}

// PodarServiciosAusentes marca revocados los servicios de una máquina que ya NO figuran en su
// reporte: lo que se desinstaló, lo que se renombró.
//
// UNA LISTA VACÍA NO PODA NADA **SALVO QUE EL LLAMADOR AFIRME QUE ES UN HECHO**, y las dos mitades
// de esa frase costaron A78.
//
// La regla original era «vacía no poda, punto», calcada de PodarEstadoDePoliticas, y protegía un
// caso real: «este device no reportó ningún servicio» es también lo que se ve cuando el agente
// arrancó a medias, cuando systemd no contestó, o cuando el lote entero de reportes era inválido.
// Vaciar el inventario por eso es irreversible; conservarlo cuesta unas filas.
//
// Lo que faltaba es que hay DOS maneras de llegar acá con la lista vacía y sólo una es un
// accidente. La otra es que la máquina lo haya DICHO —el bloque `servicios` vino, y vino `[]`—,
// y ésa es una afirmación: «acá no corre nada de lo que mirás». Tratarla como al accidente deja
// el inventario del cerebro congelado con servicios que ya no existen, envejeciendo, sin que nada
// se ponga en rojo. Un panel que muestra fantasmas es peor que uno vacío.
//
// Por eso la autorización viaja en un parámetro y no se deduce acá: quién sabe distinguir el
// hecho del accidente es quien leyó el JSON, no el almacén. `vacioAfirma` en false conserva la
// guarda vieja entera.
//
// Y LO DECLARADO A MANO NO SE PODA NUNCA (`AND declared = 0`), que es la otra mitad de lo mismo.
// La poda razona «la máquina dejó de reportarlo, así que ya no está», y esa inferencia NO VALE
// sobre una fila que la máquina no reportó jamás: musubi_fleet_service_declare existe justamente
// para lo que ningún enumerador ve —un Tier B que no enumera, un bot, un puente—, de modo que sin
// esta guarda el primer latido con inventario borraría todo lo declarado en toda la flota a la
// vez. Lo declarado sale del inventario cuando lo saca una persona.
func (e *DbEngine) PodarServiciosAusentes(deviceID string, vivos []string, vacioAfirma bool) (int64, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return 0, nil
	}
	if len(vivos) == 0 && !vacioAfirma {
		return 0, nil
	}
	// La vacía autorizada se escribe SIN la cláusula `NOT IN`, y conviene decir por qué no es sólo
	// estilo. SQLite acepta `NOT IN ()` con la lista vacía —se midió: podá igual, es una extensión
	// suya— pero la mayoría de los motores lo rechazan como error de sintaxis. Apoyar «vaciar el
	// inventario de una máquina» en una particularidad del dialecto es dejar una mina para el día
	// que el almacén no sea SQLite: no fallaría acá, fallaría allá, y el síntoma sería una poda
	// que no ocurre. Escrito así, la intención está en el código y no en el motor.
	//
	// Es el mismo UPDATE con las mismas dos guardas (`revoked = 0`, `declared = 0`): lo único que
	// cambia es que no sobrevive nadie.
	consulta := `UPDATE services SET revoked = 1 WHERE device_id = ? AND revoked = 0 AND declared = 0`
	args := make([]any, 0, len(vivos)+1)
	args = append(args, deviceID)
	if len(vivos) > 0 {
		marcas := strings.TrimSuffix(strings.Repeat("?,", len(vivos)), ",")
		consulta += ` AND name NOT IN (` + marcas + `)`
		for _, v := range vivos {
			args = append(args, v)
		}
	}
	res, err := e.db.Exec(consulta, args...)
	if err != nil {
		return 0, fmt.Errorf("error al podar los servicios ausentes de %q: %w", deviceID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer el resultado de la poda de servicios de %q: %w", deviceID, err)
	}
	return n, nil
}

// ── Escaneo ─────────────────────────────────────────────────────────────────────────────────

func (e *DbEngine) consultarServicios(q string, args ...any) ([]fleet.Servicio, error) {
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar los servicios: %w", err)
	}
	defer rows.Close()
	var out []fleet.Servicio
	for rows.Next() {
		s, err := escanearServicio(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al recorrer los servicios: %w", err)
	}
	return out, nil
}

// escanearServicio traduce una fila al dominio.
//
// Una SALUD ILEGIBLE se trata como AUSENTE, no como error, igual que una muestra ilegible en
// escanearDevice: perder una medición vieja es barato, quedarse sin inventario porque un campo no
// parsea es el fallo caro. Y como el estado se deriva, el servicio aparece «desconocido», que es
// exactamente lo que es.
func escanearServicio(row escaneable) (fleet.Servicio, error) {
	var (
		s              fleet.Servicio
		registrado     string
		ultimoReporte  sql.NullString
		saludGuardada  string
		revocado       int
		declarado      int
		claseGuardada  string
		nombreGuardado string
	)
	if err := row.Scan(
		&s.ID, &nombreGuardado, &s.ProjectID, &s.DeviceID, &claseGuardada,
		&registrado, &ultimoReporte, &saludGuardada, &revocado, &declarado,
	); err != nil {
		return fleet.Servicio{}, fmt.Errorf("error al escanear un servicio: %w", err)
	}
	s.Nombre, s.Clase = nombreGuardado, claseGuardada
	if t, ok := parseObsTime(registrado); ok {
		s.Registrado = t
	}
	// last_report NULL = nunca reportó. Queda en el cero de time.Time, que es lo que
	// fleet.Servicio.Fresco espera para responder «sin noticias».
	if ultimoReporte.Valid {
		if t, ok := parseObsTime(ultimoReporte.String); ok {
			s.UltimoReporte = t
		}
	}
	if salud, err := fleet.SaludDesdeTexto(saludGuardada); err == nil {
		s.Salud = salud
	}
	s.Revocado = revocado != 0
	s.Declarado = declarado != 0
	return s, nil
}
