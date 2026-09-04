package memory

// devices.go es la PERSISTENCIA del registro de flota (track «Control de flota», slice S1).
// El dominio —qué es un dispositivo, qué puede su tier, cuándo está en línea— vive en
// internal/fleet y no sabe que esta base existe. Acá sólo se traduce ese dominio a filas.
//
// La tabla la crea la migración 29. Lo que esa migración NO tiene (columna `online`, token crudo,
// borrado físico) está explicado ahí y sostenido acá: no hay un solo método en este archivo que
// escriba un estado de conexión ni que haga DELETE.

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"

	"github.com/google/uuid"
)

// ErrDeviceDuplicado se devuelve cuando ya hay un dispositivo con ese nombre en el proyecto.
// Se distingue del resto porque el llamador tiene una acción concreta: elegir otro nombre o
// revocar el anterior.
var ErrDeviceDuplicado = errors.New("ya existe un dispositivo con ese nombre en el proyecto")

// columnasDevice es la lista de columnas en el orden que espera escanearDevice. Una sola copia:
// que el SELECT y el Scan se desincronicen es el bug clásico de esta capa.
const columnasDevice = `id, name, project_id, tier, caps, os, arch, address, agent_version, tags, enrolled_at, last_seen, revoked, last_sample, rustdesk_id, rustdesk_id_previo, rustdesk_id_cambiado, consentimiento, puede_preguntar, requiere_aprobacion`

// AltaDevice registra un dispositivo y devuelve la fila creada, con el id que asignó el CEREBRO.
//
// El id NO lo elige el cliente (invariante A1): se genera acá. Si `d.ID` viene con algo, se
// IGNORA — un dispositivo que puede declarar su propia identidad puede afirmar ser otro, y toda
// la telemetría, el exec y la auditoría quedarían atribuidos a la máquina equivocada. Es el mismo
// invariante que principals.go ya sostiene para las personas.
//
// `token` es la credencial CRUDA: se hashea acá y no se guarda ni se devuelve. Quien la generó
// (fleet.NuevoToken) es responsable de entregarla una sola vez. Un token vacío es legítimo para
// Tier B, que se alcanza con las llaves del cerebro y no tiene credencial propia.
func (e *DbEngine) AltaDevice(d fleet.Device, token string) (fleet.Device, error) {
	if err := fleet.ValidarAlta(d); err != nil {
		return fleet.Device{}, err
	}
	d.ID = uuid.NewString()
	d.Name = strings.TrimSpace(d.Name)
	d.ProjectID = strings.TrimSpace(d.ProjectID)
	d.Revoked = false
	d.LastSeen = time.Time{} // nunca latió: lo dice la ausencia, no una columna `online`
	if d.EnrolledAt.IsZero() {
		d.EnrolledAt = time.Now().UTC()
	}

	hash := ""
	if strings.TrimSpace(token) != "" {
		hash = fleet.HashToken(token)
	}

	_, err := e.db.Exec(
		// El INSERT nombra sus columnas EXPLÍCITAMENTE y no reusa columnasDevice.
		//
		// Antes las compartía, y agregar dos columnas al SELECT rompió el alta con un
		// «16 values for 18 columns» — un error que no aparece al compilar, sólo al dar de alta.
		// Son dos listas con propósitos distintos: el SELECT tiene que traer TODO lo que se
		// escanea, y el INSERT sólo lo que el alta decide. Las columnas con default (la muestra,
		// el id de RustDesk y su procedencia) no se escriben acá porque el alta no las conoce.
		`INSERT INTO devices (id, name, project_id, tier, caps, os, arch, address, agent_version, tags, enrolled_at, last_seen, revoked, last_sample, rustdesk_id, token_sha256)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, 0, '', '', ?)`,
		d.ID, d.Name, d.ProjectID, string(d.Tier), fleet.CapsComoTexto(d.Caps),
		d.OS, d.Arch, d.Address, d.AgentVer, strings.Join(d.Tags, ","),
		d.EnrolledAt.UTC().Format(time.RFC3339), hash,
	)
	if err != nil {
		// El índice único (project_id, name) es quien decide, no una consulta previa: entre un
		// SELECT y un INSERT hay una carrera, y la base no la tiene.
		if esViolacionDeUnicidad(err) {
			return fleet.Device{}, fmt.Errorf("%w: %q en %q", ErrDeviceDuplicado, d.Name, d.ProjectID)
		}
		return fleet.Device{}, fmt.Errorf("error al dar de alta el dispositivo %q: %w", d.Name, err)
	}
	return d, nil
}

// DevicePorToken resuelve la identidad de un dispositivo a partir de su credencial. Es el camino
// que hace cierto el invariante A1: el id sale de buscar el hash, nunca de lo que el cliente diga.
//
// SOBRE LA GUARDA DE TOKEN VACIO, y que la sostiene de verdad. Hoy NO es lo unico que separa a
// una peticion sin credencial de autenticarse: los Tier B se guardan con token_sha256 = cadena
// vacia, y HashToken("") es e3b0c442..., asi que el SELECT no los encontraria igual. Medido, no
// supuesto.
//
// Existe porque es la segunda linea de una defensa cuya PRIMERA linea esta en otro metodo: si
// alguien "simplifica" AltaDevice para hashear siempre (en vez de guardar la cadena vacia cuando
// no hay credencial), TODOS los devices sin token compartirian el hash del vacio y una peticion
// sin credencial se autenticaria como uno de ellos: un bypass completo. Las dos mitades estan a
// 80 lineas de distancia y nada en el compilador las ata, asi que la guarda se queda y la prueba
// TestTokenVacioNoAutenticaNiConUnaFilaQueHasheoElVacio simula exactamente ese error.
//
// Un dispositivo REVOCADO no resuelve (A9): la fila sigue existiendo para la auditoría, pero deja
// de autenticar en el acto.
func (e *DbEngine) DevicePorToken(token string) (fleet.Device, bool, error) {
	if strings.TrimSpace(token) == "" {
		return fleet.Device{}, false, nil
	}
	row := e.db.QueryRow(
		`SELECT `+columnasDevice+` FROM devices WHERE token_sha256 = ? AND revoked = 0`,
		fleet.HashToken(token),
	)
	return escanearUnDevice(row)
}

// DevicePorNombre busca por la clave humana (proyecto + nombre). Devuelve también los revocados:
// el llamador que administra necesita verlos para poder decir «ése ya lo diste de baja».
func (e *DbEngine) DevicePorNombre(projectID, name string) (fleet.Device, bool, error) {
	row := e.db.QueryRow(
		`SELECT `+columnasDevice+` FROM devices WHERE project_id = ? AND name = ?`,
		strings.TrimSpace(projectID), strings.TrimSpace(name),
	)
	return escanearUnDevice(row)
}

// DevicePorID resuelve una máquina por su id interno.
//
// Lo necesita el relay de shell (S5b): de la máquina, una sesión sólo guarda el id, y la
// concesión se re-evalúa contra el device en CADA request del stream — así revocar a alguien le
// corta el prompt abierto, no sólo el próximo.
//
// NO lleva projectID en la firma a propósito, y por eso el llamador tiene que compuertar después:
// el id es interno y ya viene de una fila que se leyó bajo el proyecto correcto. Meter el
// proyecto acá daría la falsa impresión de que esta función aísla, y no es su trabajo.
func (e *DbEngine) DevicePorID(id string) (fleet.Device, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return fleet.Device{}, false, nil
	}
	row := e.db.QueryRow(`SELECT `+columnasDevice+` FROM devices WHERE id = ?`, id)
	return escanearUnDevice(row)
}

// ListarDevices devuelve la flota de UN proyecto. El aislamiento es el mismo eje que aísla la
// memoria (invariante A7): los dispositivos del proyecto A no aparecen listando el B.
//
// SOBRE LA GUARDA DE projectID VACIO. No es lo que impide devolver «todos» —el WHERE ya lo
// impide: con projectID vacio el SELECT filtra por project_id = cadena vacia y no matchea ninguna
// fila legitima, porque el alta exige proyecto—. Lo que evita es lo OTRO: si alguna fila HUERFANA
// llegara a existir (un backfill, una reparacion a mano, una migracion futura que afloje el NOT
// NULL), un llamador que se olvido de pasar el proyecto recibiria justo esas: las que no
// pertenecen a nadie y por eso mismo no deberia ver. Devolver [] es la lectura fail-closed.
// Lo fija TestListarSinProyectoNoDevuelveLasFilasHuerfanas.
func (e *DbEngine) ListarDevices(projectID string, incluirRevocados bool) ([]fleet.Device, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, nil
	}
	q := `SELECT ` + columnasDevice + ` FROM devices WHERE project_id = ?`
	if !incluirRevocados {
		q += ` AND revoked = 0`
	}
	q += ` ORDER BY name`

	rows, err := e.db.Query(q, projectID)
	if err != nil {
		return nil, fmt.Errorf("error al listar los dispositivos de %q: %w", projectID, err)
	}
	defer rows.Close()

	var out []fleet.Device
	for rows.Next() {
		d, err := escanearDevice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al recorrer los dispositivos de %q: %w", projectID, err)
	}
	return out, nil
}

// LatirDevice estampa la última señal de vida y, si viene, la última muestra. Devuelve si
// actualizó.
//
// NO es un error que no actualice (invariante A10). Un dispositivo revocado o borrado que sigue
// latiendo es lo NORMAL —el agente todavía no se enteró—, y convertir eso en un error haría que
// cada máquina dada de baja produzca una cascada de ruido en los logs del cerebro hasta que
// alguien la apague. El llamador recibe `false` y decide: el camino sensato es responder al
// agente que se dé por revocado.
//
// `muestra` es el JSON de la telemetría del host (S4), o vacío. VACÍO NO BORRA LA ANTERIOR, y
// esa asimetría importa: un agente en un OS sin colector, o con el colector roto, late igual
// (está vivo, que es lo que el latido afirma) y no tiene por qué hacer desaparecer la última
// medición buena que sí se tomó. Estar viva y saber medirse son cosas distintas.
//
// Se escribe en el MISMO UPDATE que last_seen: la telemetría no agrega ni una escritura.
func (e *DbEngine) LatirDevice(id string, ahora time.Time, muestra string) (bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil
	}
	return latirDeviceCon(e.db, id, ahora, muestra)
}

// latirDeviceCon es el UPDATE del latido, y existe para que la MISMA sentencia corra en
// autocommit (LatirDevice, que la usan la sonda y el exec) y adentro de una transacción ajena
// (LatirYTomarComandos, ver latido.go).
//
// LA SENTENCIA VIVE EN UN SOLO LUGAR A PROPÓSITO. Copiarla en el camino transaccional habría
// dejado dos definiciones de qué significa «latir», y la primera vez que alguien agregue una
// columna al latido va a tocar una sola — la máquina quedaría con la mitad del latido escrito
// según por qué puerta entró, y nada lo diría.
//
// Asume `id` ya recortado y no vacío: la guarda es del llamador, que es quien puede devolver
// `false` sin tocar la base.
func latirDeviceCon(x execQuerier, id string, ahora time.Time, muestra string) (bool, error) {
	// El COALESCE-por-parámetro: cuando `muestra` es vacío la columna se reasigna a sí misma.
	// Una sola sentencia para los dos casos, en vez de dos caminos que se pueden desincronizar.
	res, err := x.Exec(
		`UPDATE devices SET last_seen = ?, last_sample = CASE WHEN ? = '' THEN last_sample ELSE ? END
		 WHERE id = ? AND revoked = 0`,
		ahora.UTC().Format(time.RFC3339), muestra, muestra, id,
	)
	if err != nil {
		return false, fmt.Errorf("error al registrar el latido de %q: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado del latido de %q: %w", id, err)
	}
	return n > 0, nil
}

// RenombrarDevice cambia el NOMBRE de una máquina conservando su id, y con él todo su historial
// (A64).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ HACÍA FALTA UNA OPERACIÓN Y NO ALCANZABA CON «dar de baja y volver a enrolar»
//
// El id es lo que referencian la bitácora de comandos, las dos clases de sesión y el inventario
// de servicios. Volver a enrolar da un id NUEVO: la máquina aparece vacía, sin nada de lo que le
// pasó, y lo viejo queda colgando de un device revocado con otro nombre. O sea que el único
// camino que existía convertía un cambio de nombre en una pérdida de auditoría.
//
// EL NOMBRE ES ÚNICO POR PROYECTO Y LO IMPONE LA BASE (idx_devices_nombre). Un choque no se
// chequea con un SELECT previo —entre el SELECT y el UPDATE hay una carrera y la base no la
// tiene—: se deja fallar y se traduce el error, que es la misma regla que usa el alta.
//
// NO TOCA NADA MÁS, y eso es deliberado: las concesiones de `principals.yaml` y los alcances de
// las políticas nombran máquinas por NOMBRE, y arreglarlos desde acá sería que el cerebro edite
// la credencial de alguien. Quién decide eso es una persona; lo que hace este motor es cambiar el
// nombre, y lo que hace la superficie de arriba es DECIR qué quedó apuntando a un nombre que ya
// no existe.
func (e *DbEngine) RenombrarDevice(projectID, viejo, nuevo string) (fleet.Device, error) {
	projectID, viejo, nuevo = strings.TrimSpace(projectID), strings.TrimSpace(viejo), strings.TrimSpace(nuevo)
	if projectID == "" || viejo == "" || nuevo == "" {
		return fleet.Device{}, fmt.Errorf("renombrar necesita proyecto, nombre viejo y nombre nuevo")
	}
	if viejo == nuevo {
		return fleet.Device{}, fmt.Errorf("el nombre nuevo es el mismo que el viejo")
	}
	res, err := e.db.Exec(`UPDATE devices SET name = ? WHERE project_id = ? AND name = ?`,
		nuevo, projectID, viejo)
	if err != nil {
		// El único choque posible es el nombre ya tomado. Se traduce a algo legible: un
		// "UNIQUE constraint failed" en la respuesta de una tool no le dice nada a nadie.
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return fleet.Device{}, fmt.Errorf("ya hay una máquina llamada %q en el proyecto %q", nuevo, projectID)
		}
		return fleet.Device{}, fmt.Errorf("error al renombrar %q: %w", viejo, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fleet.Device{}, fmt.Errorf("no hay ninguna máquina %q en el proyecto %q", viejo, projectID)
	}
	d, _, err := e.DevicePorNombre(projectID, nuevo)
	return d, err
}

// RevocarDevice da de baja un dispositivo: deja de autenticar en el acto y la fila QUEDA.
//
// Es una bandera y no un DELETE (invariante A9). Borrar la fila perdería a quién pertenecían la
// telemetría y las sesiones que ya ocurrieron — justo lo que hace falta mirar después de un
// incidente, que es cuando uno revoca.
//
// La credencial se BORRA de la fila. La bandera ya alcanza para que DevicePorToken no resuelva,
// pero un hash que sobrevive a la baja es material que no hace falta conservar, y además libera
// el índice único por si el mismo token se re-emite. La identidad, el proyecto y las fechas —lo
// que la auditoría necesita— siguen intactos.
//
// Y ARRASTRA SUS SERVICIOS (S12), en la MISMA transacción. Sin esto, los servicios de una máquina
// revocada seguirían apareciendo en el listado del proyecto como si nada, y el hueco pasa
// desapercibido justo hasta un incidente — que es cuando alguien revoca y después mira.
//
// Se eligió esto sobre las otras dos opciones: un JOIN a `devices` al LEER contradiría el patrón
// denormalizado de todo el resto de la flota, y «no hacer nada» deja un servicio visible de una
// máquina que ya no existe, que es exactamente lo que no querés ver después de un incidente. Las
// filas QUEDAN revocadas, igual que la del device: la auditoría necesita saber qué corría ahí.
func (e *DbEngine) RevocarDevice(projectID, name string) (bool, error) {
	projectID, name = strings.TrimSpace(projectID), strings.TrimSpace(name)

	tx, err := e.db.Begin()
	if err != nil {
		return false, fmt.Errorf("error al abrir la transacción para revocar %q: %w", name, err)
	}
	defer tx.Rollback()

	// El id se lee ADENTRO de la transacción y con el mismo WHERE que el UPDATE: es lo que ata
	// la baja de la máquina con la de sus servicios. Resolverlo afuera dejaría una ventana en la
	// que otra baja concurrente cambia la fila entre las dos sentencias.
	var id string
	err = tx.QueryRow(
		`SELECT id FROM devices WHERE project_id = ? AND name = ? AND revoked = 0`, projectID, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // no hay una máquina activa con ese nombre: el llamador lo traduce
	}
	if err != nil {
		return false, fmt.Errorf("error al buscar el dispositivo %q para revocarlo: %w", name, err)
	}

	if _, err := tx.Exec(
		`UPDATE devices SET revoked = 1, token_sha256 = '' WHERE id = ?`, id); err != nil {
		return false, fmt.Errorf("error al revocar el dispositivo %q: %w", name, err)
	}
	if _, err := tx.Exec(
		`UPDATE services SET revoked = 1 WHERE device_id = ? AND revoked = 0`, id); err != nil {
		return false, fmt.Errorf("error al revocar los servicios de %q: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("error al confirmar la baja de %q: %w", name, err)
	}
	return true, nil
}

// ActualizarAutoreporte guarda lo que una máquina sabe de SÍ MISMA: la versión del agente que
// corre y la dirección por la que se la alcanza.
//
// Es la ÚNICA escritura que un dispositivo puede hacer sobre el registro, y está acotada por
// construcción: se identifica por `id` —que el llamador derivó del TOKEN, nunca del cuerpo— así
// que ninguna máquina puede tocar la fila de otra. Campos vacíos no pisan lo que había: un agente
// viejo que no reporta versión no debe borrar la que quedó registrada.
//
// Silenciosa a propósito: es telemetría de conveniencia, no un dato del que dependa nada. Si
// falla, el latido sigue valiendo.
func (e *DbEngine) ActualizarAutoreporte(id, version, direccion string) error {
	id = strings.TrimSpace(id)
	version, direccion = strings.TrimSpace(version), strings.TrimSpace(direccion)
	if id == "" || (version == "" && direccion == "") {
		return nil
	}
	_, err := e.db.Exec(
		`UPDATE devices
		    SET agent_version = CASE WHEN ? = '' THEN agent_version ELSE ? END,
		        address       = CASE WHEN ? = '' THEN address       ELSE ? END
		  WHERE id = ? AND revoked = 0`,
		version, version, direccion, direccion, id,
	)
	if err != nil {
		return fmt.Errorf("error al registrar el autorreporte de %q: %w", id, err)
	}
	return nil
}

// ProyectosConDevices lista los proyectos que tienen al menos una máquina ACTIVA, ordenados.
//
// La usa el export a Prometheus de un principal federado (read=all), que no tiene un proyecto
// propio al que acotarse. `tope` acota el barrido: un scrape corre cada 15 s y no puede
// convertirse en un escaneo sin fin. Se pide uno de más a propósito, para que el llamador pueda
// DISTINGUIR "son exactamente `tope`" de "hay más" y decirlo en vez de truncar en silencio.
func (e *DbEngine) ProyectosConDevices(tope int) ([]string, error) {
	if tope <= 0 {
		return nil, nil
	}
	rows, err := e.db.Query(
		`SELECT DISTINCT project_id FROM devices WHERE revoked = 0 AND project_id <> '' ORDER BY project_id LIMIT ?`, tope)
	if err != nil {
		return nil, fmt.Errorf("error al listar los proyectos con dispositivos: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("error al escanear un proyecto: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al recorrer los proyectos con dispositivos: %w", err)
	}
	return out, nil
}

// ── Escaneo ─────────────────────────────────────────────────────────────────────────────────

// escaneable abstrae *sql.Row y *sql.Rows para no escribir el Scan dos veces.
type escaneable interface {
	Scan(dest ...any) error
}

func escanearUnDevice(row *sql.Row) (fleet.Device, bool, error) {
	d, err := escanearDevice(row)
	if errors.Is(err, sql.ErrNoRows) {
		return fleet.Device{}, false, nil
	}
	if err != nil {
		return fleet.Device{}, false, err
	}
	return d, true, nil
}

// escanearDevice traduce una fila al dominio.
//
// Las columnas de texto se leen TOLERANDO basura (fleet.CapsDesdeTexto descarta lo que no
// conoce, un tier ilegible queda como está y el dominio lo trata como sin capacidades). El
// criterio es que una fila rara no pueda impedir LISTAR la flota: quedarse sin inventario porque
// un campo no parsea es peor que mostrar ese dispositivo con menos permisos de los que tenía —
// y menos permisos es, además, el lado seguro del error.
func escanearDevice(row escaneable) (fleet.Device, error) {
	var (
		d                fleet.Device
		tier, caps, tags string
		enrolled         string
		lastSeen         sql.NullString
		revoked          int
		muestra          string
		cambiado         string
		consent          string
		puedePreguntar   int
		requiereAprob    int
	)
	if err := row.Scan(
		&d.ID, &d.Name, &d.ProjectID, &tier, &caps,
		&d.OS, &d.Arch, &d.Address, &d.AgentVer, &tags,
		&enrolled, &lastSeen, &revoked, &muestra, &d.RustdeskID, &d.RustdeskIDPrevio, &cambiado,
		&consent, &puedePreguntar, &requiereAprob,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fleet.Device{}, err // lo traduce escanearUnDevice
		}
		return fleet.Device{}, fmt.Errorf("error al escanear un dispositivo: %w", err)
	}
	d.Tier = fleet.Tier(tier)
	d.Caps = fleet.CapsDesdeTexto(caps)
	// SE GUARDA CRUDO, se resuelve al usarlo. Un valor ilegible en la columna no se corrige acá:
	// el dominio lo trata como el default, y corregirlo en la lectura escondería que alguien
	// escribió algo que no se entiende.
	d.Consentimiento = fleet.Consentimiento(consent)
	d.PuedePreguntar = puedePreguntar != 0
	d.RequiereAprobacion = requiereAprob != 0
	if tags != "" {
		d.Tags = strings.Split(tags, ",")
	}
	if t, ok := parseObsTime(enrolled); ok {
		d.EnrolledAt = t
	}
	// El cero de time.Time significa «nunca cambió», que es lo que le pasa a la enorme mayoría.
	if t, ok := parseObsTime(cambiado); ok {
		d.RustdeskIDCambiado = t
	}
	// last_seen NULL = nunca latió. Queda en el cero de time.Time, que es lo que
	// fleet.Device.EnLinea espera para responder «no está en línea».
	if lastSeen.Valid {
		if t, ok := parseObsTime(lastSeen.String); ok {
			d.LastSeen = t
		}
	}
	d.Revoked = revoked != 0
	// Una muestra guardada ilegible se trata como AUSENTE, no como error: igual que las caps,
	// una fila rara no puede impedir listar la flota. Perder una medición vieja es barato;
	// quedarse sin inventario, no.
	if m, err := fleet.MuestraDesdeTexto(muestra); err == nil {
		d.UltimaMuestra = m
	}
	return d, nil
}

// esViolacionDeUnicidad reconoce el error de índice único de SQLite por su texto, que es lo único
// que expone el driver modernc sin envolverlo en un tipo propio.
func esViolacionDeUnicidad(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique constraint") || strings.Contains(s, "constraint failed")
}

// FijarConsentimiento escribe la POLÍTICA de consentimiento de una máquina.
//
// Es su propio método y no un `ActualizarDevice` genérico a propósito. Un update genérico sobre
// la fila del dispositivo es una puerta por la que, con el tiempo, se terminan pisando el tier,
// las capacidades o el project_id — los tres campos que sostienen el aislamiento. Un método por
// campo mutable cuesta más de escribir y no admite ese accidente.
//
// NO VALIDA EL GRADO ACÁ: la validación es del dominio y la hace el llamador, que es quien puede
// devolver un error útil. Guardar un valor ilegible tampoco abre nada —el dominio lo resuelve al
// default— así que el peor caso de un bypass es una fila rara, no una puerta abierta.
func (e *DbEngine) FijarConsentimiento(deviceID string, c fleet.Consentimiento) (bool, error) {
	res, err := e.db.Exec(
		`UPDATE devices SET consentimiento = ? WHERE id = ? AND revoked = 0`, string(c), deviceID)
	if err != nil {
		return false, fmt.Errorf("error al fijar el consentimiento: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado: %w", err)
	}
	return n > 0, nil
}

// FijarCapacidadDePreguntar guarda lo que el AGENTE reporta: si en esa máquina hay dónde dibujar
// un diálogo y quién lo conteste.
//
// Va aparte de FijarConsentimiento porque son hechos de dueños distintos —uno es política de
// quien administra, el otro es una medición del agente— y mezclarlos en un método dejaría que un
// latido pise la política, o que un administrador afirme una capacidad que nadie midió.
func (e *DbEngine) FijarCapacidadDePreguntar(deviceID string, puede bool) error {
	v := 0
	if puede {
		v = 1
	}
	if _, err := e.db.Exec(
		`UPDATE devices SET puede_preguntar = ? WHERE id = ? AND revoked = 0`, v, deviceID); err != nil {
		return fmt.Errorf("error al fijar la capacidad de preguntar: %w", err)
	}
	return nil
}
