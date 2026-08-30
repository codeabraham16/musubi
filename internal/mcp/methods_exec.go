package mcp

// methods_exec.go es el PLANO DE TERMINAL visto desde las personas: pedir que una máquina corra
// algo, y leer la bitácora de lo que se corrió. Track «Control de flota», S5.
//
// Es la primera cosa del track que CAMBIA EL ESTADO de una máquina ajena, así que las guardas no
// son ceremonia: la compuerta de S3 (tenencia ∧ concesión ∧ aparato) decide, y todo queda escrito
// antes de que nada se ejecute.

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// esperaPasoExec es cada cuánto se relee el comando mientras se espera su resultado.
//
// 250 ms es un compromiso medido contra lo que hay del otro lado: el agente entrega el resultado
// en cuanto termina, así que la latencia real la domina el comando, no este sondeo. Bajarlo a 50
// ms multiplicaría por cinco las consultas para ganar milisegundos que nadie percibe.
const esperaPasoExec = 250 * time.Millisecond

// esperaMaxExec es lo MÁS que musubi_fleet_exec bloquea antes de devolver «todavía sin
// resultado».
//
// 45 s está atado al deadline del transporte, que es de 60 s por request (config
// service.request_timeout_seconds). Si la espera lo superara, el caller no recibiría la nota
// honesta «sigue corriendo, buscalo en la bitácora»: recibiría un timeout de HTTP, que se lee
// como «el cerebro no anda» cuando en realidad todo funciona.
//
// Lo destapó un sabotaje: sin la compuerta, un exec sobre una máquina inexistente esperaba 90 s
// —timeout del comando (30) más dos márgenes (60)— o sea 30 s MÁS de lo que el transporte
// aguanta. Un comando con timeout largo sale por el camino de `no_wait` y se consulta después.
const esperaMaxExec = 45 * time.Second

// bitacoraTopeDefault y bitacoraTopeMax acotan cuánto devuelve la bitácora.
const (
	bitacoraTopeDefault = 20
	bitacoraTopeMax     = 200
)

// toolFleetExec encola un comando y espera su resultado hasta el timeout.
func (s *McpServer) toolFleetExec(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Device  string   `json:"device"`
		Argv    []string `json:"argv"`
		Timeout int      `json:"timeout_seg"`
		Project string   `json:"project"`
		NoWait  bool     `json:"no_wait"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`: en qué máquina ejecutar")
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}

	d, existe, err := s.engine.DevicePorNombre(proyecto, nombre)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	// Un device inexistente y uno que no podés tocar dan LA MISMA respuesta. Distinguirlos
	// convertiría la tool en un oráculo de qué máquinas existen en un proyecto que no ves.
	if !existe || !PuedeSobreDevice(p, d, fleet.CapExec) {
		return nil, rpcErrorf(codeUnauthorized,
			"no podés ejecutar en %q: o no existe en el proyecto %q, o tu credencial no tiene la capacidad `exec` sobre esa máquina (ver la sección `fleet:` de principals.yaml)", nombre, proyecto)
	}

	timeout := fleet.ComandoTimeoutDefault
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
	}
	if err := fleet.ValidarComando(args.Argv, timeout); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	// ESCALAMIENTO CERRADO: las operaciones INTERNAS (`musubi:*`) no se pueden encolar por acá.
	//
	// El canal de comandos lo comparten el exec y la pantalla, y el agente distingue por el
	// primer argumento. Sin esta guarda, alguien con `exec` podría encolar a mano un
	// `musubi:pantalla` y acuñarse una sesión de pantalla SIN tener `screen` — o sea, saltarse la
	// compuerta usando la otra mitad de ella. Que `exec` y `screen` sean permisos distintos deja
	// de ser cierto si uno puede fabricar los mensajes del otro.
	if len(args.Argv) > 0 && strings.HasPrefix(strings.TrimSpace(args.Argv[0]), "musubi:") {
		return nil, rpcErrorf(codeUnauthorized,
			"`musubi:*` son operaciones internas del canal, no comandos del host: no se pueden encolar con exec. Para una sesión de pantalla usá musubi_fleet_screen, que exige la capacidad `screen`.")
	}
	// EL CUARTO LADO (S10): poder ejecutar y poder ejecutar CUALQUIER COSA son dos permisos.
	//
	// Se aplica acá —después de la compuerta, antes de encolar— y vale para los DOS transportes:
	// la cola del agente (Tier A) y el SSH del cerebro (Tier B) salen los dos de esta función, así
	// que la allowlist no tiene una puerta de atrás por el lado del protocolo.
	//
	// El rechazo se CUENTA, y con razón propia: un token bien configurado que choca contra su
	// propia allowlist no es lo mismo que un intento de tocar una máquina ajena.
	if !argvPermitido(p, d, args.Argv) {
		s.metrics.execAllowDenied.Add(1)
		permitidos, _ := comandosPermitidos(p, d)
		return nil, rpcErrorf(codeUnauthorized,
			"tu credencial puede ejecutar en %q, pero no ese comando: la allowlist (`fleet_exec_allow` en principals.yaml) permite %v sobre esa máquina", d.Name, permitidos)
	}

	// F1 — LA BITÁCORA SE ESCRIBE ANTES DE EJECUTAR. Desde acá, el pedido está registrado pase
	// lo que pase: se caiga el cerebro, muera el agente, se apague la máquina.
	cmd, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: proyecto, Principal: nombrePrincipal(p),
		Origen: fleet.OrigenPersona,
		Argv:   args.Argv, Timeout: timeout,
	})
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}

	base := map[string]interface{}{
		"command_id": cmd.ID,
		"device":     d.Name,
		"project_id": proyecto,
		"argv":       cmd.Argv,
	}
	encolarYVolver := func(nota string) (interface{}, *RpcError) {
		base["estado"] = string(fleet.EstadoPendiente)
		base["nota"] = nota
		return jsonResult(base)
	}

	// TIER B: no hay agente que levante la cola, así que el CEREBRO sale a buscar la máquina
	// (S7). El comando ya quedó auditado arriba —F1 vale igual— y acá se ejecuta de una.
	//
	// No se chequea `EnLinea`: un Tier B no late, así que «en línea» sólo puede significar «la
	// última vez que pudimos llegar». El INTENTO es la prueba de vida.
	if d.Tier == fleet.TierProtocolo {
		return s.ejecutarEnTierB(d, cmd, timeout, base)
	}

	if args.NoWait {
		return encolarYVolver("encolado; consultá el resultado con musubi_fleet_log")
	}
	// LA MÁQUINA ESTÁ CAÍDA: no tiene sentido bloquear al llamador hasta el tope esperando a
	// alguien que no está latiendo. Se dice, y el comando queda encolado —vence a los 15 min si
	// nadie lo levanta (F10)—. Es información útil, no un fallo.
	if !d.EnLinea(time.Now(), s.umbralEnLinea(d)) {
		return encolarYVolver("la máquina no está latiendo, así que nadie va a levantar el comando ahora mismo. Queda encolado y vence en 15 min si el agente no vuelve.")
	}
	// Un comando cuyo timeout no entra en lo que el transporte aguanta se encola y se consulta
	// después: esperarlo daría un timeout de HTTP, que se lee como «el cerebro no anda».
	if timeout >= esperaMaxExec {
		return encolarYVolver("el timeout del comando supera lo que el transporte aguanta esperando; quedó encolado y corriendo. Buscá el resultado con musubi_fleet_log.")
	}

	// La espera es ACOTADA, y el techo lo pone el transporte (ver esperaMaxExec). Si vence, el
	// comando NO se cancela —puede estar corriendo— y se dice explícitamente dónde buscar el
	// resultado: dejar creer que no se ejecutó sería peor que esperar.
	paciencia := timeout + fleet.ComandoTimeoutDefault
	if paciencia > esperaMaxExec {
		paciencia = esperaMaxExec
	}
	final, err := s.esperarComando(ctx, cmd.ID, paciencia)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if final.Estado != fleet.EstadoTerminado {
		base["estado"] = string(final.Estado)
		base["nota"] = "todavía sin resultado: la máquina puede estar caída, o el comando sigue corriendo. NO se canceló; buscá el resultado con musubi_fleet_log."
		return jsonResult(base)
	}
	return jsonResult(conResultado(base, final, time.Now()))
}

// esperarComando relee el comando hasta que termine o se agote la paciencia.
//
// Respeta el ctx del request: si el cliente se va, se deja de consultar. Sin eso, un cliente que
// corta la conexión dejaría al servidor sondeando la base durante minutos por nadie.
func (s *McpServer) esperarComando(ctx context.Context, id string, paciencia time.Duration) (fleet.Comando, error) {
	limite := time.Now().Add(paciencia)
	for {
		c, existe, err := s.engine.ComandoPorID(id)
		if err != nil {
			return fleet.Comando{}, err
		}
		if !existe {
			return fleet.Comando{}, nil
		}
		if c.Estado == fleet.EstadoTerminado || c.Estado == fleet.EstadoExpirado || time.Now().After(limite) {
			return c, nil
		}
		select {
		case <-ctx.Done():
			return c, nil
		case <-time.After(esperaPasoExec):
		}
	}
}

// toolFleetLog devuelve la bitácora: quién ejecutó qué, dónde y cómo salió.
//
// Es readOnly y por-proyecto, pero NO muestra los comandos de máquinas sobre las que no tenés
// `exec`: saber qué se corrió en un servidor es casi tan revelador como poder correrlo.
func (s *McpServer) toolFleetLog(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project string `json:"project"`
		Device  string `json:"device"`
		Limite  int    `json:"limite"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	tope := bitacoraTopeDefault
	if args.Limite > 0 {
		tope = args.Limite
	}
	if tope > bitacoraTopeMax {
		tope = bitacoraTopeMax
	}

	// Qué máquinas puede ver: se resuelve UNA vez y se usa para filtrar, en vez de consultar la
	// compuerta por cada fila de la bitácora.
	devices, err := s.engine.ListarDevices(proyecto, true)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	nombrePorID := make(map[string]string, len(devices))
	for _, d := range devices {
		if PuedeSobreDevice(p, d, fleet.CapExec) {
			nombrePorID[d.ID] = d.Name
		}
	}

	// Se piden más filas de las que se devuelven porque el filtro por permiso corta después: sin
	// el margen, una bitácora dominada por una máquina que no ves devolvería una lista vacía y
	// parecería que no pasó nada.
	crudos, err := s.engine.BitacoraDeComandos(proyecto, strings.TrimSpace(args.Device), tope*4)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	filas := make([]map[string]interface{}, 0, tope)
	ocultos := 0
	// UNA sola lectura del reloj para toda la bitácora: con `time.Now()` por fila, dos comandos
	// del mismo instante podrían caer a distinto lado del vencimiento dentro de la MISMA
	// respuesta, y una lista que se contradice a sí misma no la explica nadie.
	ahora := time.Now()
	for _, c := range crudos {
		nombre, puede := nombrePorID[c.DeviceID]
		if !puede {
			ocultos++
			continue
		}
		if len(filas) >= tope {
			continue
		}
		fila := map[string]interface{}{
			"command_id": c.ID,
			"device":     nombre,
			"principal":  c.Principal,
			// LA CONTRASEÑA DE PANTALLA VIAJA EN EL ARGV —tiene que llegar a la máquina de
			// alguna forma— y esta tabla guarda el argv tal cual. Sin este ocultamiento, la
			// bitácora entregaría contraseñas de sesión a cualquiera que pueda leerla, y la
			// garantía G1 («Musubi nunca guarda la contraseña») se caería por la puerta de al
			// lado: no la guardaría, pero la mostraría.
			"argv":   ocultarArgvDePantalla(c.Argv),
			"creado": c.Creado.UTC().Format(time.RFC3339),
		}
		filas = append(filas, conResultado(fila, c, ahora))
	}
	res := map[string]interface{}{"project_id": proyecto, "total": len(filas), "comandos": filas}
	if ocultos > 0 {
		res["sin_permiso"] = ocultos
	}
	return jsonResult(res)
}

// conResultado agrega los campos del resultado a una fila. Vive acá porque exec y log tienen que
// mostrar EXACTAMENTE lo mismo: dos formateos distintos del mismo dato es cómo un panel y una
// consola terminan discrepando sobre si un comando salió bien.
func conResultado(fila map[string]interface{}, c fleet.Comando, ahora time.Time) map[string]interface{} {
	// DERIVADO, no el estado guardado — y va acá porque ÉSTE es el único lugar que escribe la
	// clave: la fila de la bitácora ya lo derivaba un poco más arriba y esta línea lo pisaba con
	// el crudo, así que el arreglo era código muerto. Lo cazó la prueba que compara las dos
	// superficies, no la lectura del diff.
	//
	// `expirado` sólo se ESTAMPA cuando el agente viene a pedir su cola. Una máquina cuyo agente
	// no vuelve deja sus comandos en `pendiente` para siempre — medido en producción: cincuenta
	// comandos de diez horas con una vida máxima de quince minutos, dibujados como pendientes.
	fila["estado"] = string(c.EstadoActual(ahora))
	// El ORIGEN, con la misma regla que la cronología: null cuando no se sabe, jamás "persona".
	// Las dos superficies leen la misma tabla y no pueden contar dos historias — la lección de
	// A39, otra vez.
	if c.Origen == fleet.OrigenDesconocido {
		fila["origen"] = nil
		fila["automatico"] = nil
	} else {
		fila["origen"] = string(c.Origen)
		fila["automatico"] = c.Origen.EsAutomatico()
	}
	// exit_code viaja como null mientras no haya terminado: «todavía no» y «terminó con 0» son
	// cosas distintas, y un 0 por default las confundiría.
	fila["exit_code"] = c.ExitCode
	if c.Stdout != "" {
		fila["stdout"] = c.Stdout
	}
	if c.Stderr != "" {
		fila["stderr"] = c.Stderr
	}
	if c.Error != "" {
		fila["error"] = c.Error
	}
	if !c.Terminado.IsZero() {
		fila["terminado"] = c.Terminado.UTC().Format(time.RFC3339)
		if !c.Entregado.IsZero() {
			fila["duracion_ms"] = c.Terminado.Sub(c.Entregado).Milliseconds()
		}
	}
	return fila
}

// nombrePrincipal es QUIÉN queda en la bitácora. Sale de la credencial, nunca del cliente: es la
// columna de la que depende toda la auditoría. Sin principal (stdio local) se dice así, en vez de
// dejarla vacía y que parezca un dato perdido.
func nombrePrincipal(p *Principal) string {
	if p == nil {
		return "local"
	}
	if n := strings.TrimSpace(p.Name); n != "" {
		return n
	}
	return "desconocido"
}

// ejecutarEnTierB corre el comando por SSH y guarda el resultado en la MISMA bitácora.
//
// Que el transporte sea SSH en vez de la cola es un detalle del transporte: la tool, la
// compuerta y la bitácora son las mismas, y quien opera no debería tener que saber por dónde
// viajó el comando.
func (s *McpServer) ejecutarEnTierB(d fleet.Device, cmd fleet.Comando, timeout time.Duration, base map[string]interface{}) (interface{}, *RpcError) {
	if err := s.correrPorSSH(d, cmd, timeout, time.Now()); err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	final, _, err := s.engine.ComandoPorID(cmd.ID)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	base["transporte"] = "ssh"
	return jsonResult(conResultado(base, final, time.Now()))
}

// correrPorSSH ejecuta un comando ya encolado en un Tier B y guarda su resultado.
//
// Vive aparte de ejecutarEnTierB porque tiene DOS llamadores: la tool (una persona pidiéndolo) y
// las políticas de S10 (un temporizador). Que compartan esta función no es ahorro de líneas: es
// la garantía de que lo automático y lo manual dejan EXACTAMENTE el mismo rastro. Dos copias de
// esto se habrían separado a la primera corrección, y la que se quedaría vieja sería la que nadie
// mira ejecutarse.
func (s *McpServer) correrPorSSH(d fleet.Device, cmd fleet.Comando, timeout time.Duration, ahora time.Time) error {
	res := fleet.EjecutarPorSSH(d.Address, cmd.Argv, timeout)

	// Se guarda con el DeviceID como dueño, igual que si lo hubiera reportado un agente: la
	// bitácora no distingue el transporte, y no debería.
	if err := s.engine.GuardarResultado(d.ID, cmd.ID, res.ExitCode, res.Stdout, res.Stderr, res.Error, ahora); err != nil {
		return err
	}

	// H3a — «en línea» de un Tier B es «la última vez que pudimos llegar». Se estampa sólo si
	// SE LLEGÓ: un fallo de canal (host caído, credencial rechazada) no es una prueba de vida, y
	// estamparlo igual haría que una máquina inalcanzable figure viva para siempre.
	if res.Error == "" {
		if _, err := s.engine.LatirDevice(d.ID, ahora, ""); err != nil {
			// No es fatal: el comando corrió y su resultado ya está guardado.
			_ = err
		}
	}
	return nil
}
