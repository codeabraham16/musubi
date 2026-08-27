package mcp

// politicas.go es AUTO-HEAL: ejecución remota sin una persona detrás. Track «Control de flota», S10.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ES LO MÁS PELIGROSO DEL TRACK ENTERO, Y LO QUE LO HACE DEFENDIBLE CABE EN UNA FRASE:
//
//	UNA POLÍTICA NO TIENE AUTORIDAD PROPIA.
//
// Nombra un principal de principals.yaml y actúa con la suya: la misma compuerta de tres lados de
// S3, la misma allowlist de S10, la misma bitácora que las personas. No hay un segundo camino a
// la ejecución remota — hay el mismo camino, recorrido por un temporizador en vez de por alguien.
//
// La alternativa (un daemon que ejecuta «porque es el daemon») habría sido más corta de escribir
// y sería exactamente el puente de privilegio que el track viene esquivando desde el proposal:
// bastaría con poder editar el archivo de configuración del cerebro para tener root en 40
// máquinas, sin figurar en ninguna concesión y sin dejar un nombre en la auditoría.
//
// Y una consecuencia que conviene ver de frente: si alguien REVOCA al principal de una política,
// la política se apaga sola en el próximo tick. No hay que acordarse de apagarla en dos lugares.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// aplicarPoliticas evalúa todas las políticas contra todas las máquinas de un proyecto y devuelve
// cuántas acciones se dispararon.
func (s *McpServer) aplicarPoliticas(proyecto string, ahora time.Time) int {
	if len(s.politicas) == 0 {
		return 0 // I15: sin sección, no existe
	}
	// Se RE-LEE la lista después del sondeo: las muestras que acaban de entrar son justamente las
	// que hay que juzgar. Evaluar sobre la lista previa al barrido significaría reaccionar siempre
	// con un tick de atraso, y con un tick de atraso el cooldown y la condición se desincronizan.
	devices, err := s.engine.ListarDevices(proyecto, false)
	if err != nil {
		logx.Error("políticas: no se pudieron listar los dispositivos", "proyecto", proyecto, "error", err)
		return 0
	}
	acciones := 0
	for _, pol := range s.politicas {
		for _, d := range devices {
			if s.evaluarPolitica(pol, d, ahora) {
				acciones++
			}
		}
	}
	return acciones
}

// evaluarPolitica decide y, si corresponde, actúa sobre UNA máquina. Devuelve si actuó.
//
// El orden de las guardas es de más barato a más caro, pero sobre todo es de más específico a más
// general: primero lo que descarta la mayoría sin tocar nada.
func (s *McpServer) evaluarPolitica(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {
	if !pol.Alcanza(d.Name) {
		return false
	}
	if d.Revoked {
		return false
	}
	// I13 — NO SE ACTÚA SOBRE UNA MUESTRA RANCIA.
	//
	// Es la guarda que más veces va a evitar un desastre y la más fácil de olvidar. Si la máquina
	// dejó de reportar, el último dato que tenemos puede ser de hace horas: el disco pudo haberse
	// vaciado hace veinte minutos, o el proceso que consumía la RAM pudo haber muerto. Actuar con
	// eso no es reaccionar tarde — es reaccionar a algo que ya no está pasando.
	//
	// El umbral es el MISMO que decide «en línea», y por tier (I2): si la máquina figura caída, su
	// muestra es rancia por definición.
	umbral := s.umbralEnLinea(d)
	if d.UltimaMuestra == nil || !d.EnLinea(ahora, umbral) {
		return false
	}
	if ahora.Sub(d.UltimaMuestra.Tomada) > umbral {
		// Late pero dejó de MEDIR: el agente vive y el colector murió. Es el fallo silencioso que
		// `up` no detecta, y una política que no lo mirara actuaría eternamente sobre la última
		// muestra buena — que, siendo la última buena, siempre cruza el umbral.
		return false
	}
	valor, dispara := pol.Dispara(d.UltimaMuestra)
	if !dispara {
		return false
	}
	// I14 — cooldown por (política × máquina), contado desde el DISPARO y no desde el resultado:
	// lo que hay que espaciar es la decisión de actuar, y el comando puede tardar.
	clave := pol.Nombre + "\x00" + d.ID
	if previo, hay := s.ultimoDisparo.Load(clave); hay {
		if t, ok := previo.(time.Time); ok && ahora.Sub(t) < pol.CooldownEfectivo() {
			return false
		}
	}

	// El principal se resuelve AHORA, en cada evaluación, contra el snapshot vigente del registro
	// (que se recarga en caliente cada 10 s). Resolverlo una vez al arranque habría dejado a las
	// políticas actuando en nombre de credenciales ya revocadas.
	if s.buscarPrincipal == nil {
		return false
	}
	pr, existe := s.buscarPrincipal.porNombre(pol.Principal)
	if !existe {
		// Se revocó o se le cambió el nombre entre el arranque y ahora. Se dice fuerte: una
		// política que dejó de poder actuar es una alarma apagada, y las alarmas apagadas en
		// silencio son la razón por la que existe la mitad de este slice.
		//
		// PERO SE DICE UNA SOLA VEZ. Esto no es un estado transitorio: dura hasta que alguien
		// edite principals.yaml. Un WARN por tick son 288 líneas idénticas por día, que es
		// exactamente cómo se entierra la línea que sí importa — el mismo criterio con el que el
		// resto del scheduler sólo se anuncia cuando hubo trabajo. Lo destapó el e2e: 17 avisos
		// idénticos en un minuto. La MÉTRICA sí se incrementa siempre, porque de ella vive la
		// alerta PoliticaSinPermiso: lo que se acota es el ruido, no la señal.
		s.avisarUnaVez("sin_principal:"+pol.Nombre, func() {
			logx.Warn("política sin principal: no actúa (no se repite este aviso hasta que se resuelva)",
				"politica", pol.Nombre, "principal", pol.Principal, "device", d.Name,
				"nota", "el principal ya no está en principals.yaml; la política quedó inerte")
		})
		s.metrics.contarPolitica(pol.Nombre, "sin_principal")
		return false
	}
	// Volvió a resolver: se rearma el aviso, para que una segunda revocación se vuelva a avisar.
	s.avisosDados.Delete("sin_principal:" + pol.Nombre)

	// LAS DOS COMPUERTAS, LAS MISMAS QUE PARA UNA PERSONA. No hay atajo por ser automático.
	if !PuedeSobreDevice(pr, d, fleet.CapExec) {
		// Por máquina, y una vez: un rechazo de compuerta también es un estado que dura hasta que
		// alguien edite el registro, no un evento.
		s.avisarUnaVez("compuerta:"+pol.Nombre+"\x00"+d.ID, func() {
			logx.Warn("política rechazada por la compuerta: el principal no tiene `exec` sobre esa máquina",
				"politica", pol.Nombre, "principal", pol.Principal, "device", d.Name)
		})
		s.metrics.contarPolitica(pol.Nombre, "rechazada")
		return false
	}
	s.avisosDados.Delete("compuerta:" + pol.Nombre + "\x00" + d.ID)
	if !argvPermitido(pr, d, pol.Hacer) {
		permitidos, _ := comandosPermitidos(pr, d)
		s.avisarUnaVez("allowlist:"+pol.Nombre+"\x00"+d.ID, func() {
			logx.Warn("política rechazada por la allowlist del principal",
				"politica", pol.Nombre, "principal", pol.Principal, "device", d.Name,
				"pidio", pol.Hacer[0], "permitidos", permitidos)
		})
		s.metrics.contarPolitica(pol.Nombre, "rechazada")
		return false
	}
	s.avisosDados.Delete("allowlist:" + pol.Nombre + "\x00" + d.ID)

	// El cooldown se marca ANTES de ejecutar. Si se marcara después, un comando lento (o un
	// cerebro que se cae a mitad) dejaría la puerta abierta para que el próximo tick dispare otra
	// vez: la tormenta que el cooldown viene a evitar empieza justo cuando algo va mal.
	s.ultimoDisparo.Store(clave, ahora)
	// ...y HACIA EL DISCO, para que sobreviva un reinicio (A24). El mapa en memoria sigue siendo
	// el camino caliente —una lectura por par y por tick contra la base serían 200 consultas para
	// un dato que sólo cambia cuando algo dispara—, y la tabla es su respaldo durable.
	//
	// Un fallo al persistir NO cancela la acción: el comando ya está decidido y auditado, y el
	// costo del fallo es un cooldown que no sobrevive al próximo reinicio. Cancelar la acción por
	// no poder anotar el cooldown sería dejar el problema sin atender para proteger la anotación.
	if err := s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, ahora); err != nil {
		logx.Error("política: no se pudo persistir el cooldown (sobrevive en memoria, no a un reinicio)",
			"politica", pol.Nombre, "device", d.Name, "error", err)
	}

	logx.Info("política dispara",
		"politica", pol.Nombre, "device", d.Name, "principal", pol.Principal,
		"condicion", pol.Umbral(), "medido", valor, "hacer", pol.Hacer)

	if err := s.correrAccionDePolitica(pol, pr, d, ahora); err != nil {
		logx.Error("política: la acción falló", "politica", pol.Nombre, "device", d.Name, "error", err)
		s.metrics.contarPolitica(pol.Nombre, "error")
		return false
	}
	s.metrics.contarPolitica(pol.Nombre, "ok")
	return true
}

// correrAccionDePolitica encola (Tier A) o ejecuta (Tier B) la acción.
//
// I16 — VA A LA MISMA BITÁCORA QUE LAS PERSONAS, con el nombre del principal de la política. Un
// operador ve `auto-heal` en la misma tabla y con las mismas columnas que ve `gio`. Un segundo
// registro de auditoría «para lo automático» es cómo se llega a auditar sólo la mitad de lo que
// pasa — y la mitad automática es justo la que nadie miró ejecutarse.
func (s *McpServer) correrAccionDePolitica(pol fleet.Politica, pr *Principal, d fleet.Device, ahora time.Time) error {
	cmd, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: d.ProjectID, Principal: pr.Name,
		Argv: pol.Hacer, Timeout: fleet.ComandoTimeoutDefault,
	})
	if err != nil {
		return err
	}
	// Tier B no tiene agente que levante la cola: el cerebro sale a ejecutar, igual que en la
	// tool. Se hace SÍNCRONO dentro del barrido —el timeout del comando lo acota— porque el
	// paralelismo del barrido ya está acotado y esto no es un camino de request.
	if d.Tier == fleet.TierProtocolo {
		return s.correrPorSSH(d, cmd, fleet.ComandoTimeoutDefault, ahora)
	}
	// Tier A: queda encolado y el agente lo levanta en su próximo latido. NO se espera el
	// resultado: el barrido no es una request y nadie está del otro lado esperando una respuesta.
	// El resultado aparece en la bitácora cuando llegue.
	return nil
}

// avisarUnaVez ejecuta `emitir` sólo la primera vez que se ve esa clave.
//
// Existe porque los fallos de configuración de una política NO son eventos: son ESTADOS que duran
// hasta que alguien edita un archivo. Repetirlos en cada tick los convierte en el ruido que
// entierra a la línea que sí importa — el mismo criterio con el que el resto del scheduler sólo
// se anuncia cuando hubo trabajo. La clave se borra cuando la condición se resuelve, así que una
// recaída vuelve a avisar.
func (s *McpServer) avisarUnaVez(clave string, emitir func()) {
	if _, yaEstaba := s.avisosDados.LoadOrStore(clave, true); yaEstaba {
		return
	}
	emitir()
}

// cargarCooldowns siembra el mapa en memoria con lo que haya en la base (A24).
//
// SIN ESTO, EL COOLDOWN ES UNA GARANTÍA QUE DURA LO QUE DURE EL PROCESO. Y el reinicio no es un
// evento raro que ocurra en momentos tranquilos: es lo primero que alguien hace cuando algo va
// mal, que es exactamente cuando las políticas están disparando. El caso concreto: la política
// vacía un journal, el operador reinicia el cerebro treinta segundos después para tocar otra
// cosa, y la política vuelve a vaciarlo porque la muestra vieja todavía cruza el umbral.
//
// Best-effort: si la base no contesta se arranca con los cooldowns vacíos y se dice. Negarse a
// arrancar por esto sería peor — el cerebro entero abajo para proteger un espaciado.
func (s *McpServer) cargarCooldowns() {
	if len(s.politicas) == 0 {
		return
	}
	porPolitica, err := s.engine.CooldownsDePoliticas()
	if err != nil {
		logx.Warn("políticas: no se pudo cargar el cooldown persistido; se arranca sin él (una política podría actuar antes de tiempo, una vez)", "error", err)
		return
	}
	n := 0
	for _, pol := range s.politicas {
		for deviceID, cuando := range porPolitica[pol.Nombre] {
			s.ultimoDisparo.Store(pol.Nombre+"\x00"+deviceID, cuando)
			n++
		}
	}
	if n > 0 {
		logx.Info("políticas: cooldowns recuperados del reinicio", "pares", n)
	}
}

// ── Lo que el inventario tiene que poder mostrar (S9b · A23) ────────────────────────────────

// politicasSobre describe las políticas que aplican a una máquina, para el inventario y el panel.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO ES PARTE DE LA SEGURIDAD Y NO DE LA COSMÉTICA
//
// S10 dejó al cerebro ejecutando comandos en máquinas ajenas sin una persona detrás, y eso NO SE
// VEÍA EN NINGÚN LADO salvo hurgando la bitácora después del hecho. Una máquina con auto-heal
// encima era indistinguible de una sin él. Quien mira el inventario tiene que poder saber que ahí
// hay algo que actúa solo, qué haría, y con la autoridad de quién.
//
// EL CAMPO QUE MÁS IMPORTA ES `puede_actuar`. Una política mal configurada —su principal perdió la
// concesión, o el comando se cayó de la allowlist— se ve EXACTAMENTE IGUAL que una que funciona:
// las dos figuran en la lista y ninguna hace nada visible hasta que la condición se cumple. Es
// una alarma apagada, y la única forma de que alguien lo note antes del incidente es decirlo acá.
//
// QUIÉN VE QUÉ. El detalle exige `exec` sobre esa máquina, la misma regla que la bitácora: saber
// qué comando corre en un servidor es casi tan revelador como poder correrlo. Pero el CONTEO se
// muestra a cualquiera que vea la máquina — que exista algo automático encima no es un secreto, y
// ocultarlo del todo dejaría a quien sólo tiene `metrics` viendo cambiar una máquina sin ninguna
// pista de por qué.
// ────────────────────────────────────────────────────────────────────────────────────────────
func (s *McpServer) politicasSobre(p *Principal, d fleet.Device) (detalle []map[string]interface{}, total int) {
	if len(s.politicas) == 0 {
		return nil, 0
	}
	verDetalle := PuedeSobreDevice(p, d, fleet.CapExec)
	for _, pol := range s.politicas {
		if !pol.Alcanza(d.Name) {
			continue
		}
		total++
		if !verDetalle {
			continue
		}
		fila := map[string]interface{}{
			"nombre":       pol.Nombre,
			"principal":    pol.Principal,
			"condicion":    pol.Umbral(),
			"hacer":        pol.Hacer,
			"cooldown_min": int(pol.CooldownEfectivo().Minutes()),
			// puede_actuar es la INTERSECCIÓN real, evaluada ahora contra el registro vigente:
			// las mismas dos compuertas que la política va a atravesar cuando le toque.
			"puede_actuar": s.politicaPuedeActuar(pol, d),
		}
		// El último disparo viaja como null cuando nunca actuó: «todavía no» y «actuó hace mucho»
		// son cosas distintas, y una fecha inventada las confundiría — el mismo criterio que
		// gobierna exit_code y los porcentajes en todo el track.
		fila["ultimo_disparo"] = nil
		if v, hay := s.ultimoDisparo.Load(pol.Nombre + "\x00" + d.ID); hay {
			if t, ok := v.(time.Time); ok {
				fila["ultimo_disparo"] = t.UTC().Format(time.RFC3339)
			}
		}
		detalle = append(detalle, fila)
	}
	return detalle, total
}

// politicaPuedeActuar responde «si la condición se cumpliera ahora mismo, ¿pasaría algo?».
//
// Es deliberadamente la MISMA cadena de guardas que evaluarPolitica, y no una reimplementación
// aproximada: un indicador que dijera «sí» donde la política dice «no» sería peor que no tenerlo,
// porque enseñaría a confiar en él.
func (s *McpServer) politicaPuedeActuar(pol fleet.Politica, d fleet.Device) bool {
	if d.Revoked || s.buscarPrincipal == nil {
		return false
	}
	pr, existe := s.buscarPrincipal.porNombre(pol.Principal)
	if !existe {
		return false
	}
	return PuedeSobreDevice(pr, d, fleet.CapExec) && argvPermitido(pr, d, pol.Hacer)
}
