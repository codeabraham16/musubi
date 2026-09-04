package memory

// comandos.go es la persistencia de la BITÁCORA DE EJECUCIÓN REMOTA (S5). El dominio vive en
// internal/fleet; acá sólo se traduce a filas.
//
// La tabla la crea la migración 31, y ahí está explicado por qué la fila se escribe AL ENCOLAR.

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"

	"github.com/google/uuid"
)

// ErrComandoAjeno se devuelve cuando una máquina intenta reportar el resultado de un comando que
// no es suyo. Es un error de SEGURIDAD, no de datos: sin esta guarda cualquier device de la flota
// podría envenenar la bitácora de otro.
var ErrComandoAjeno = errors.New("ese comando no pertenece a este dispositivo")

const columnasComando = `id, device_id, project_id, principal, argv, timeout_seg, estado,
	creado, entregado, terminado, exit_code, stdout, stderr, error, origen, plano`

// EncolarComando registra el pedido y lo deja pendiente. Devuelve el comando con su ID.
//
// ESCRIBIR ACÁ ES EL INVARIANTE (F1): la fila existe desde antes de que nada se ejecute. Si el
// cerebro muere en el próximo milisegundo, el pedido queda auditado igual.
func (e *DbEngine) EncolarComando(c fleet.Comando) (fleet.Comando, error) {
	if err := fleet.ValidarComando(c.Argv, c.Timeout); err != nil {
		return fleet.Comando{}, err
	}
	c.Argv = fleet.LimpiarArgv(c.Argv)
	// ── EL TECHO DE LA COLA, ANTES DE ESCRIBIR NADA ─────────────────────────────────────────
	//
	// Va acá y no en la tool porque ÉSTA es la única puerta de escritura: la comparten el exec,
	// las sesiones de pantalla, la shell y el motor de políticas. Un techo en una sola de esas
	// cuatro es un techo que se esquiva por las otras tres.
	//
	// SE CUENTA SÓLO LO QUE TODAVÍA PODRÍA EJECUTARSE, y ésa es la parte que hay que pensar: si
	// contara todo lo pendiente, una máquina que estuvo caída un día quedaría bloqueada PARA
	// SIEMPRE —sus miles de filas muertas ocupando el cupo— y destrabarla exigiría borrar
	// bitácora, que es justo lo que este repo no hace. Lo vencido no es presión de cola: es
	// historia.
	if c.Creado.IsZero() {
		c.Creado = time.Now().UTC()
	}
	vivos := c.Creado.Add(-fleet.ComandoVidaMax).UTC().Format(time.RFC3339)
	var enCola int
	if err := e.db.QueryRow(
		`SELECT COUNT(*) FROM device_commands WHERE device_id = ? AND estado = ? AND creado >= ?`,
		c.DeviceID, string(fleet.EstadoPendiente), vivos,
	).Scan(&enCola); err != nil {
		return fleet.Comando{}, fmt.Errorf("error al medir la cola de %q: %w", c.DeviceID, err)
	}
	if enCola >= fleet.ColaMaxPorDevice {
		return fleet.Comando{}, fmt.Errorf("%w: %d comandos esperando en %q y el máximo es %d; casi seguro su agente no está levantando la cola",
			fleet.ErrColaLlena, enCola, c.DeviceID, fleet.ColaMaxPorDevice)
	}
	c.ID = uuid.NewString()
	c.Estado = fleet.EstadoPendiente
	argv, err := fleet.ArgvComoTexto(c.Argv)
	if err != nil {
		return fleet.Comando{}, err
	}
	// EL ORIGEN SE NORMALIZA AL ESCRIBIR, no al leer: un valor raro que entre desde un llamador
	// nuevo se guarda como desconocido en vez de crear una categoría que ninguna superficie sabe
	// dibujar. Lo desconocido ya tiene significado; lo inventado, no.
	c.Origen = fleet.OrigenValido(c.Origen)
	// ── LA CLASIFICACIÓN SE EXIGE ACÁ, QUE ES LA ÚNICA PUERTA ───────────────────────────────
	//
	// Mismo motivo que el techo de la cola de más arriba: exec, pantalla, shell y las políticas
	// entran todos por esta función. Una etiqueta que se comprueba en la tool es una etiqueta que
	// el quinto camino no va a llevar, y el modo de falla no es «falta un dato»: es que la fila
	// cae a `HechoSinClasificar` y desaparece de la cronología sin que nadie se entere —o, peor,
	// que alguien la clasifique por defecto y vuelva la fuga que la migración 46 vino a cerrar.
	//
	// Falla RUIDOSA y no silenciosa: rechazar el encolado se ve en el acto y se arregla en el
	// llamador. Guardarla sin etiqueta se descubre meses después, mirando una cronología que
	// calla.
	if fleet.OpsClasificadasPorFila[primerArgv(c.Argv)] && fleet.TipoDeComando(c) == fleet.HechoSinClasificar {
		return fleet.Comando{}, fmt.Errorf(
			"la operación interna %q no puede encolarse sin declarar su plano: el mismo argv lo emiten pantalla, shell y exec, "+
				"y sin `Clasificacion` la cronología no puede saber a quién mostrársela", primerArgv(c.Argv))
	}
	_, err = e.db.Exec(
		`INSERT INTO device_commands (id, device_id, project_id, principal, argv, timeout_seg, estado, creado, origen, plano)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.DeviceID, c.ProjectID, c.Principal, argv,
		int(c.Timeout.Seconds()), string(c.Estado), c.Creado.UTC().Format(time.RFC3339), string(c.Origen),
		string(c.Clasificacion),
	)
	if err != nil {
		return fleet.Comando{}, fmt.Errorf("error al encolar el comando para %q: %w", c.DeviceID, err)
	}
	return c, nil
}

// TomarComandos entrega a una máquina los comandos que le tocan y los marca `entregado`.
//
// Es una operación de UN SOLO PASO por diseño: leer y marcar en la misma transacción. Si fueran
// dos, dos latidos concurrentes de la misma máquina —que pasan, un agente reiniciado deja al
// anterior terminando— se llevarían el mismo comando y se ejecutaría dos veces. Un `systemctl
// restart` duplicado es molesto; un script de migración duplicado, no.
//
// Los VENCIDOS se marcan `expirado` y NO se entregan (F10): si el agente estuvo caído una semana,
// no despierta y corre lo que se pidió el lunes.
func (e *DbEngine) TomarComandos(deviceID string, ahora time.Time, tope int) ([]fleet.Comando, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || tope <= 0 {
		return nil, nil
	}
	tx, err := e.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("error al abrir la transacción de la cola: %w", err)
	}
	defer tx.Rollback()

	out, err := tomarComandosEnTx(tx, deviceID, ahora, tope)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("error al confirmar la entrega de comandos: %w", err)
	}
	return out, nil
}

// tomarComandosEnTx es el CUERPO de la entrega, sin abrir ni cerrar la transacción.
//
// Se partió así para que el LATIDO pueda meter el vencimiento y la entrega en la misma
// transacción en la que estampa la señal de vida (ver latido.go): a 2000 máquinas cada 30 s, dos
// transacciones separadas por latido son dos fsync donde alcanzaba uno. El paso de la
// transacción por parámetro es lo que hace que la operación siga siendo indivisible desde las
// DOS puertas — dos latidos concurrentes de la misma máquina no pueden llevarse el mismo comando,
// que es la razón por la que esto era una transacción desde el principio.
//
// Asume `deviceID` ya recortado y `tope > 0`: las guardas son del llamador.
func tomarComandosEnTx(tx *sql.Tx, deviceID string, ahora time.Time, tope int) ([]fleet.Comando, error) {
	// Primero vencer lo viejo. Se hace acá y no en un barrido de fondo porque el momento en que
	// importa es JUSTO antes de entregar: es la única ventana donde un comando podría colarse.
	limite := ahora.Add(-fleet.ComandoVidaMax).UTC().Format(time.RFC3339)
	// Lo que vence sin entregarse también lleva su secreto encima, y ya no va a servirle a nadie:
	// se tapa ANTES de marcarlo, en la misma transacción, por la misma razón que abajo.
	if err := taparPantallasPendientesVencidas(tx, deviceID, limite); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(
		`UPDATE device_commands SET estado = ?, terminado = ?, error = ?
		  WHERE device_id = ? AND estado = ? AND creado < ?`,
		string(fleet.EstadoExpirado), ahora.UTC().Format(time.RFC3339),
		"venció antes de que el agente lo levantara", deviceID, string(fleet.EstadoPendiente), limite,
	); err != nil {
		return nil, fmt.Errorf("error al vencer comandos viejos: %w", err)
	}

	rows, err := tx.Query(
		`SELECT `+columnasComando+` FROM device_commands
		  WHERE device_id = ? AND estado = ? ORDER BY creado LIMIT ?`,
		deviceID, string(fleet.EstadoPendiente), tope)
	if err != nil {
		return nil, fmt.Errorf("error al leer la cola de %q: %w", deviceID, err)
	}
	var out []fleet.Comando
	for rows.Next() {
		c, err := escanearComando(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("error al recorrer la cola de %q: %w", deviceID, err)
	}
	rows.Close()

	for i := range out {
		if _, err := tx.Exec(
			`UPDATE device_commands SET estado = ?, entregado = ? WHERE id = ?`,
			string(fleet.EstadoEntregado), ahora.UTC().Format(time.RFC3339), out[i].ID,
		); err != nil {
			return nil, fmt.Errorf("error al marcar entregado %q: %w", out[i].ID, err)
		}
		// EL SECRETO SE TAPA EN LA MISMA TRANSACCIÓN QUE ENTREGA (A74). La contraseña de una
		// sesión de pantalla viaja en el argv y el agente la acaba de recibir en `out`: desde este
		// instante la fila no la necesita para nada —quien cierra la sesión lee sólo argv[0] y
		// argv[1], que se conservan— y dejarla cruda en la tabla era una copia en claro que ninguna
		// superficie de lectura mostraba pero cualquier `sqlite3` sí. Va acá y no en una poda
		// diferida porque la sesión dura horas y una poda a los treinta días llega tarde por
		// definición.
		if err := taparArgvConSecreto(tx, out[i].ID, out[i].Argv); err != nil {
			return nil, err
		}
		out[i].Estado = fleet.EstadoEntregado
		out[i].Entregado = ahora
	}
	return out, nil
}

// taparArgvConSecreto reescribe el argv GUARDADO de un comando que lleva secreto a la forma de
// fleet.ArgvDeBitacora, que conserva la operación y el id de sesión y tapa el resto.
//
// Se acota por argv[0] EXACTO y no por prefijo `musubi:`: de las operaciones internas sólo la de
// pantalla lleva una contraseña, y tapar las demás (avisar, preguntar) borraría el texto que se
// le mostró al usuario — que es justo lo que la cronología necesita para explicar qué pasó. El
// argv en memoria NO se toca: el llamador ya lo tiene y se lo debe al agente tal cual.
func taparArgvConSecreto(tx *sql.Tx, id string, argv []string) error {
	if len(argv) == 0 || argv[0] != fleet.OpPantalla {
		return nil
	}
	texto, err := fleet.ArgvComoTexto(fleet.ArgvDeBitacora(argv))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE device_commands SET argv = ? WHERE id = ?`, texto, id); err != nil {
		return fmt.Errorf("error al tapar la contraseña de pantalla del comando %q: %w", id, err)
	}
	return nil
}

// taparPantallasPendientesVencidas tapa el secreto de las operaciones de pantalla que están por
// vencer sin haberse entregado. El prefijo del LIKE es sólo un filtro barato para no traer la
// cola entera de una máquina que estuvo caída; la decisión la toma argv[0], exacto, en Go.
func taparPantallasPendientesVencidas(tx *sql.Tx, deviceID, limite string) error {
	prefijo, err := fleet.ArgvComoTexto([]string{fleet.OpPantalla})
	if err != nil {
		return err
	}
	// `["musubi:pantalla"]` → `["musubi:pantalla",%`: el cierre se cambia por la coma que sigue
	// al primer elemento cuando hay más de uno.
	prefijo = strings.TrimSuffix(prefijo, "]") + ",%"
	rows, err := tx.Query(
		`SELECT id, argv FROM device_commands
		  WHERE device_id = ? AND estado = ? AND creado < ? AND argv LIKE ?`,
		deviceID, string(fleet.EstadoPendiente), limite, prefijo)
	if err != nil {
		return fmt.Errorf("error al buscar pantallas vencidas de %q: %w", deviceID, err)
	}
	type fila struct{ id, argv string }
	var vencidas []fila
	for rows.Next() {
		var f fila
		if err := rows.Scan(&f.id, &f.argv); err != nil {
			rows.Close()
			return fmt.Errorf("error al escanear una pantalla vencida: %w", err)
		}
		vencidas = append(vencidas, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("error al recorrer las pantallas vencidas de %q: %w", deviceID, err)
	}
	rows.Close()
	for _, f := range vencidas {
		if err := taparArgvConSecreto(tx, f.id, fleet.ArgvDesdeTexto(f.argv)); err != nil {
			return err
		}
	}
	return nil
}

// GuardarResultado registra cómo salió un comando.
//
// `deviceID` NO es informativo: es la GUARDA (F3). Se exige que el comando pertenezca a esa
// máquina, y el llamador derivó ese id del TOKEN. Sin esto, cualquier device de la flota podría
// escribir el resultado de un comando ajeno y envenenar la bitácora de otro.
//
// La salida se trunca acá también, no sólo en el agente: el agente es un cliente y sus límites
// son una cortesía, no una garantía.
func (e *DbEngine) GuardarResultado(deviceID, comandoID string, exit *int, stdout, stderr, errCanal string, ahora time.Time) error {
	deviceID, comandoID = strings.TrimSpace(deviceID), strings.TrimSpace(comandoID)
	if deviceID == "" || comandoID == "" {
		return ErrComandoAjeno
	}
	var dueno, estado string
	err := e.db.QueryRow(`SELECT device_id, estado FROM device_commands WHERE id = ?`, comandoID).Scan(&dueno, &estado)
	if err == sql.ErrNoRows {
		return ErrComandoAjeno
	}
	if err != nil {
		return fmt.Errorf("error al buscar el comando %q: %w", comandoID, err)
	}
	if dueno != deviceID {
		return ErrComandoAjeno
	}
	// Un comando ya terminado no se re-escribe: la bitácora es append-once por fila. Un agente
	// que reintenta el reporte no puede cambiar un resultado que ya se leyó.
	if estado == string(fleet.EstadoTerminado) {
		return nil
	}

	so, _ := fleet.TruncarSalida(stdout)
	se, _ := fleet.TruncarSalida(stderr)
	_, err = e.db.Exec(
		`UPDATE device_commands SET estado = ?, terminado = ?, exit_code = ?, stdout = ?, stderr = ?, error = ?
		  WHERE id = ?`,
		string(fleet.EstadoTerminado), ahora.UTC().Format(time.RFC3339), exit, so, se, errCanal, comandoID,
	)
	if err != nil {
		return fmt.Errorf("error al guardar el resultado de %q: %w", comandoID, err)
	}
	return nil
}

// ComandoPorID devuelve un comando. Lo usa la espera acotada de musubi_fleet_exec.
func (e *DbEngine) ComandoPorID(id string) (fleet.Comando, bool, error) {
	row := e.db.QueryRow(`SELECT `+columnasComando+` FROM device_commands WHERE id = ?`, strings.TrimSpace(id))
	c, err := escanearComando(row)
	if errors.Is(err, sql.ErrNoRows) {
		return fleet.Comando{}, false, nil
	}
	if err != nil {
		return fleet.Comando{}, false, err
	}
	return c, true, nil
}

// BitacoraDeComandos devuelve la historia de un proyecto, lo más reciente primero.
func (e *DbEngine) BitacoraDeComandos(projectID, deviceID string, tope int) ([]fleet.Comando, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" || tope <= 0 {
		return nil, nil
	}
	q := `SELECT ` + columnasComando + ` FROM device_commands WHERE project_id = ?`
	args := []any{projectID}
	if d := strings.TrimSpace(deviceID); d != "" {
		q += ` AND device_id = ?`
		args = append(args, d)
	}
	q += ` ORDER BY creado DESC LIMIT ?`
	args = append(args, tope)

	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al leer la bitácora de %q: %w", projectID, err)
	}
	defer rows.Close()
	var out []fleet.Comando
	for rows.Next() {
		c, err := escanearComando(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al recorrer la bitácora de %q: %w", projectID, err)
	}
	return out, nil
}

// PodarSalidasDeComandos vacía stdout/stderr de los comandos más viejos que `dias`, SIN borrar la
// fila (F2).
//
// Es la separación que hace usable la bitácora: quién corrió qué y cómo salió se conserva —es lo
// que se mira después de un incidente—, pero la SALIDA puede traer una clave en un log o datos de
// un cliente, y no hay razón para guardarla para siempre. Devuelve cuántas filas podó.
func (e *DbEngine) PodarSalidasDeComandos(dias int, ahora time.Time) (int64, error) {
	if dias <= 0 {
		return 0, nil
	}
	limite := ahora.AddDate(0, 0, -dias).UTC().Format(time.RFC3339)
	res, err := e.db.Exec(
		`UPDATE device_commands SET stdout = '', stderr = ''
		  WHERE creado < ? AND (stdout <> '' OR stderr <> '')`, limite)
	if err != nil {
		return 0, fmt.Errorf("error al podar las salidas de comandos: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer el resultado de la poda: %w", err)
	}
	return n, nil
}

func escanearComando(row escaneable) (fleet.Comando, error) {
	var (
		c                    fleet.Comando
		argv, estado, creado string
		origen, plano        string
		entregado, terminado sql.NullString
		exit                 sql.NullInt64
		timeoutSeg           int
	)
	if err := row.Scan(
		&c.ID, &c.DeviceID, &c.ProjectID, &c.Principal, &argv, &timeoutSeg, &estado,
		&creado, &entregado, &terminado, &exit, &c.Stdout, &c.Stderr, &c.Error, &origen, &plano,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fleet.Comando{}, err
		}
		return fleet.Comando{}, fmt.Errorf("error al escanear un comando: %w", err)
	}
	c.Argv = fleet.ArgvDesdeTexto(argv)
	c.Timeout = time.Duration(timeoutSeg) * time.Second
	c.Estado = fleet.EstadoComando(estado)
	// Se normaliza también al LEER: una fila escrita a mano o por una versión futura no puede
	// meter una categoría que las superficies no saben dibujar. Lo que no reconozco es
	// desconocido, que ya tiene significado propio.
	c.Origen = fleet.OrigenValido(fleet.OrigenComando(origen))
	// La clasificación NO se normaliza acá: TipoDeComando ya trata lo que no reconoce como «no
	// declarado» y cae al argv. Normalizarla al leer la borraría, y entonces una fila escrita por
	// una versión futura perdería el dato en vez de conservarlo para cuando esta versión lo
	// entienda.
	c.Clasificacion = fleet.TipoDeHecho(plano)
	if t, ok := parseObsTime(creado); ok {
		c.Creado = t
	}
	if entregado.Valid {
		if t, ok := parseObsTime(entregado.String); ok {
			c.Entregado = t
		}
	}
	if terminado.Valid {
		if t, ok := parseObsTime(terminado.String); ok {
			c.Terminado = t
		}
	}
	if exit.Valid {
		v := int(exit.Int64)
		c.ExitCode = &v
	}
	return c, nil
}

// ── Estado de las políticas de flota (S10b · A24) ───────────────────────────────────────────
//
// El cooldown de una política vivía sólo en memoria, así que un reinicio del cerebro lo rearmaba
// entero. Estas tres funciones lo hacen durar. Nada más: el RESULTADO de la acción sigue estando
// en la bitácora, que es la misma para lo automático y lo manual.

// CooldownsDePoliticas devuelve el último disparo de cada (política, máquina, alcance).
//
// LA CLAVE DEL MAPA INTERIOR ES `device_id\x00alcance`, y no el device solo. El alcance vacío es
// una política de host; con contenido, lo que toca adentro de la máquina (hoy, un servicio). Se
// devuelve compuesta y no en tres niveles para que el llamador arme la misma clave que usa en
// memoria con una sola función del dominio — dos formas de componer la misma clave es cómo se
// desincronizan.
//
// Se lee TODO de una vez, al arrancar, y no fila por fila en cada evaluación: con 40 máquinas y 5
// políticas serían 200 consultas por tick para un dato que sólo cambia cuando algo dispara. El
// servidor lo siembra en su mapa en memoria y desde ahí escribe hacia los dos lados.
func (e *DbEngine) CooldownsDePoliticas() (map[string]map[string]time.Time, error) {
	rows, err := e.db.Query(`SELECT policy, device_id, alcance, last_fired FROM fleet_policy_state`)
	if err != nil {
		return nil, fmt.Errorf("error al leer el estado de las políticas: %w", err)
	}
	defer rows.Close()
	out := make(map[string]map[string]time.Time)
	for rows.Next() {
		var politica, device, alcance, cuando string
		if err := rows.Scan(&politica, &device, &alcance, &cuando); err != nil {
			return nil, fmt.Errorf("error al escanear el estado de una política: %w", err)
		}
		t, err := time.Parse(time.RFC3339, cuando)
		if err != nil {
			// Una fila ilegible se SALTEA, no aborta la carga. El costo de saltearla es un
			// cooldown de menos (una acción de más, auditada); el de abortar sería arrancar sin
			// NINGÚN cooldown, que es el fallo que esta tabla vino a evitar.
			continue
		}
		if out[politica] == nil {
			out[politica] = make(map[string]time.Time)
		}
		out[politica][device+"\x00"+alcance] = t.UTC()
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al recorrer el estado de las políticas: %w", err)
	}
	return out, nil
}

// MarcarDisparoDePolitica estampa que esta política actuó sobre esta máquina.
//
// UPSERT sobre la clave compuesta: el par no puede tener dos filas, así que no hay que decidir
// cuál gana.
func (e *DbEngine) MarcarDisparoDePolitica(politica, deviceID, alcance string, cuando time.Time) error {
	politica, deviceID = strings.TrimSpace(politica), strings.TrimSpace(deviceID)
	// `alcance` SÍ puede venir vacío: es lo que corresponde a una política de host. No se valida
	// contra vacío porque el vacío es un valor legítimo, no una omisión.
	alcance = strings.TrimSpace(alcance)
	if politica == "" || deviceID == "" {
		return fmt.Errorf("marcar el disparo de una política exige política y dispositivo")
	}
	_, err := e.db.Exec(
		`INSERT INTO fleet_policy_state (policy, device_id, alcance, last_fired) VALUES (?, ?, ?, ?)
		 ON CONFLICT(policy, device_id, alcance) DO UPDATE SET last_fired = excluded.last_fired`,
		politica, deviceID, alcance, cuando.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("error al marcar el disparo de la política %q: %w", politica, err)
	}
	return nil
}

// PodarEstadoDePoliticas borra las filas de políticas que ya no están configuradas.
//
// Sin esto, cada política que alguien renombra o saca deja su fila para siempre: una tabla que
// sólo crece, alimentada por un archivo de configuración que se edita a mano. Es chica, pero es
// exactamente la clase de basura que después nadie se anima a limpiar porque no sabe si importa.
//
// UNA LISTA VACÍA NO BORRA NADA, y es deliberado: «no hay políticas configuradas» es también lo
// que se ve cuando alguien está editando el YAML y lo dejó a medias, o cuando el cerebro arrancó
// sin su sección. Borrar todo el historial de cooldowns por eso sería irreversible; conservarlo
// cuesta unas filas. Para limpiar de verdad hay que tener al menos una política viva.
func (e *DbEngine) PodarEstadoDePoliticas(vivas []string) (int64, error) {
	if len(vivas) == 0 {
		return 0, nil
	}
	marcas := strings.TrimSuffix(strings.Repeat("?,", len(vivas)), ",")
	args := make([]interface{}, 0, len(vivas))
	for _, v := range vivas {
		args = append(args, v)
	}
	res, err := e.db.Exec(`DELETE FROM fleet_policy_state WHERE policy NOT IN (`+marcas+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("error al podar el estado de las políticas: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer el resultado de la poda de políticas: %w", err)
	}
	return n, nil
}

// primerArgv devuelve argv[0] recortado, o "" si no hay. Existe para que la guarda de encolado no
// tenga que repetir el chequeo de largo, que es justo donde se cuela un panic.
func primerArgv(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	return strings.TrimSpace(argv[0])
}
