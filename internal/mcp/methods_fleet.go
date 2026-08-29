package mcp

// methods_fleet.go es la ADMINISTRACIÓN de la flota por parte de las PERSONAS: dar de alta una
// máquina, ver el inventario y cortarle el acceso. Es el track «Control de flota», slice S2.
//
// Las tres tools se autentican como cualquier otra: contra el registro de principals. La
// credencial que ENTREGAN, en cambio, es de otra especie —vive en la tabla `devices`— y sólo
// sirve para la puerta del dispositivo (fleet_http.go). Esa separación es el diseño del slice y
// está explicada en fleet_http.go, donde se sostiene.
//
// Mintear la credencial de una máquina es ADMIN por la misma razón que mintear la de una persona
// (musubi_token_new): es la joya de la corona del plano de control.

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// umbralEnLineaDefault es cuánto silencio tolera el listado antes de dar una máquina por caída.
//
// 90 s no es un número redondo elegido de memoria: es 3 × el intervalo de latido por defecto del
// agente (30 s, ver cmd/musubi/agent.go). Con un solo intervalo de margen, cualquier hipo de red
// pinta la flota de rojo; con tres, hace falta perder tres latidos seguidos. Si alguna vez cambia
// el intervalo del agente, este número lo sigue — están atados por el factor, no por el valor.
const umbralEnLineaDefault = 90 * time.Second

// sondaIntervaloDefault es cada cuánto el cerebro sale a buscar a los dispositivos SIN agente
// (S10 · A19). 5 min es el compromiso: cada sondeo es una conexión SSH/ADB y un fork+exec por
// máquina, así que 30 s convertiría al cerebro en la carga principal de un router doméstico.
const sondaIntervaloDefault = 5 * time.Minute

// umbralEnLineaTope acota el umbral derivado. Sin tope, configurar un sondeo cada 6 h haría que
// «en línea» significara «respondió alguna vez hoy» — un semáforo que nunca se pone en rojo no
// informa nada, y es peor que no tenerlo porque parece que sí.
const umbralEnLineaTope = time.Hour

// umbralEnLineaPara es cuánto silencio tolera CADA TIER antes de darse por caído (S10 · I2).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ NO PODÍA SEGUIR SIENDO UN SOLO NÚMERO
//
// 90 s es 3 × el latido del agente, y para Tier A es exactamente el número correcto. Para un
// Tier B es un sinsentido: esa máquina NO LATE. No tiene agente, no sabe que Musubi existe; la
// única señal de vida que puede haber es que el cerebro haya ido a buscarla y la haya encontrado.
//
// O sea que su frescura máxima posible es el INTERVALO DE SONDEO. Con sondeo cada 5 min y umbral
// de 90 s, un router perfectamente sano figura caído el 97 % del tiempo — y `MaquinaCaida`
// dispara para siempre, que es la forma más eficiente de conseguir que alguien silencie la
// alerta y con ella todas las demás.
//
// El factor 3 se conserva por el mismo motivo de siempre: hacen falta tres fallos seguidos, no
// un hipo. Lo que cambia es CONTRA QUÉ se multiplica.
// ────────────────────────────────────────────────────────────────────────────────────────────
func umbralEnLineaPara(d fleet.Device, intervaloSonda time.Duration) time.Duration {
	if d.Tier == fleet.TierAgente {
		return umbralEnLineaDefault // late solo: 3 × 30 s
	}
	if intervaloSonda <= 0 {
		intervaloSonda = sondaIntervaloDefault
	}
	u := 3 * intervaloSonda
	if u < umbralEnLineaDefault {
		// Un sondeo más rápido que el latido no vuelve el umbral más exigente que el de Tier A:
		// no hay razón para ser más duro con la máquina que menos control tenemos sobre ella.
		return umbralEnLineaDefault
	}
	if u > umbralEnLineaTope {
		return umbralEnLineaTope
	}
	return u
}

// umbralEnLinea es umbralEnLineaPara con el intervalo que este cerebro tiene configurado.
func (s *McpServer) umbralEnLinea(d fleet.Device) time.Duration {
	return umbralEnLineaPara(d, s.sondaIntervalo)
}

// toolFleetEnroll da de alta un dispositivo y devuelve su token CRUDO una sola vez.
//
// El token se muestra una vez y no hay forma de recuperarlo: el registro sólo guarda su SHA-256.
// Es deliberado y es el mismo trato que reciben las personas.
func (s *McpServer) toolFleetEnroll(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_fleet_enroll da de alta una máquina en el plano de control: requiere un principal admin")
	}
	var args struct {
		Name    string   `json:"name"`
		Tier    string   `json:"tier"`
		Caps    []string `json:"caps"`
		OS      string   `json:"os"`
		Arch    string   `json:"arch"`
		Address string   `json:"address"`
		Tags    []string `json:"tags"`
		Project string   `json:"project"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	// B9 — el proyecto sale de la CREDENCIAL. writeOriginFor es la misma guarda que sella las
	// escrituras de memoria: un principal acotado (write=own) enrola en SU tenant aunque declare
	// otro. Enrolar una máquina en el proyecto ajeno sería plantar un agente en la flota de otro.
	proyecto, ok := writeOriginFor(p, args.Project)
	if !ok || strings.TrimSpace(proyecto) == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto del dispositivo: declaralo en `project` (un dispositivo sin proyecto sería visible desde todos los tenants)")
	}

	tier, err := fleet.NormalizarTier(args.Tier)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	caps, err := fleet.NormalizarCaps(args.Caps)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	// C7 — NO SE PUEDE OTORGAR LO QUE NO SE TIENE (S3). Sin esto hay un escalamiento corto y
	// real: alguien con `exec` sobre dos máquinas nombradas da de alta una tercera con `exec` y
	// se amplía el alcance solo. Que el gate de admin ya haya pasado no alcanza — ser admin de
	// la MEMORIA no otorga capacidades sobre las MÁQUINAS, que es la valla del track entero.
	for _, c := range caps {
		if !puedeOtorgar(p, c) {
			return nil, rpcErrorf(codeUnauthorized,
				"no podés conceder %q: tu credencial no tiene esa capacidad de flota con el comodín [\"*\"] en principals.yaml (otorgar en una máquina nueva es ampliarte el alcance)", c)
		}
	}

	token, err := fleet.NuevoToken()
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	d, err := s.engine.AltaDevice(fleet.Device{
		Name:      args.Name,
		ProjectID: proyecto,
		Tier:      tier,
		Caps:      caps,
		OS:        strings.TrimSpace(args.OS),
		Arch:      strings.TrimSpace(args.Arch),
		Address:   strings.TrimSpace(args.Address),
		Tags:      limpiarTags(args.Tags),
	}, token)
	if err != nil {
		// Toda la validación (nombre, proyecto, tier, matriz de capacidades, duplicado) vive en
		// el dominio y en la base: acá se propaga como argumento inválido, no como error interno.
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}

	return jsonResult(map[string]interface{}{
		"token":      token, // se muestra UNA vez; entregáselo a la máquina por un canal seguro
		"device_id":  d.ID,
		"name":       d.Name,
		"project_id": d.ProjectID,
		"tier":       string(d.Tier),
		"caps":       capsComoLista(d.Caps),
		"aviso":      "el token no se puede recuperar: el registro sólo guarda su SHA-256. Sirve ÚNICAMENTE para POST /fleet/heartbeat, no para /mcp.",
	})
}

// toolFleetList devuelve el inventario del proyecto, con `online` DERIVADO en el momento de
// servir (B11): sigue sin haber columna, el campo se calcula en cada respuesta.
func (s *McpServer) toolFleetList(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project        string `json:"project"`
		IncluirBajas   bool   `json:"include_revoked"`
		UmbralSegundos int    `json:"umbral_segundos"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}

	proyectos, truncado := s.proyectosParaLeer(p, args.Project)
	if len(proyectos) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar de qué proyecto listar la flota: declaralo en `project`")
	}

	// UmbralSegundos, si viene, PISA el umbral por tier: es la perilla de quien está depurando y
	// quiere ver la flota con su propio criterio. Sin él, cada máquina usa el suyo (I2).
	umbralExplicito := time.Duration(args.UmbralSegundos) * time.Second

	ahora := time.Now()
	filas := make([]map[string]interface{}, 0)
	enLinea := 0
	for _, proyecto := range proyectos {
		devices, err := s.engine.ListarDevices(proyecto, args.IncluirBajas)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		for _, d := range devices {
			umbral := umbralExplicito
			if umbral <= 0 {
				umbral = s.umbralEnLinea(d)
			}
			vivo := d.EnLinea(ahora, umbral)
			if vivo {
				enLinea++
			}
			fila := map[string]interface{}{
				"name":      d.Name,
				"device_id": d.ID,
				// EL PROYECTO VA EN LA FILA, no sólo en el encabezado. Con `read: all` la respuesta
				// mezcla tenants, y una tabla que no dice de quién es cada máquina es peor que una
				// que no las muestra: invita a actuar sobre la de otro cliente.
				"project": d.ProjectID,
				"tier":    string(d.Tier),
				"caps":    capsComoLista(d.Caps),
				"os":      d.OS,
				"arch":    d.Arch,
				"address": d.Address,
				"tags":    d.Tags,
				"online":  vivo, // DERIVADO, no guardado
				// C8 — la INTERSECCIÓN real entre lo que la máquina admite y lo que ESTA credencial
				// tiene concedido. `caps` dice qué se le puede pedir a la máquina; `puedo` dice qué
				// se le puede pedir DESDE ACÁ. Un inventario que muestra sólo lo primero enseña a
				// ignorar el campo: la mitad de las veces no se corresponde con lo que pasa al pedirlo.
				"puedo":       capsComoLista(capsQuePuede(p, d)),
				"enrolled_at": d.EnrolledAt.UTC().Format(time.RFC3339),
				"revoked":     d.Revoked,
				// El umbral viaja POR MÁQUINA porque desde S10 ya no es uno solo: un Tier A se da por
				// caído a los 90 s y un Tier B al triple de su intervalo de sondeo (I2). Sin este
				// campo, dos filas con el mismo `silencio_segundos` y distinto `online` parecen un bug.
				"umbral_segundos": int(umbral.Seconds()),
			}
			// QUÉ BINARIO CORRE ESTA MÁQUINA. El dato se guardaba desde el principio —el agente lo
			// manda en cada latido y `LatirDevice` lo escribe en `agent_version`— y NO SE MOSTRABA EN
			// NINGÚN LADO. Una columna llena que nadie puede leer.
			//
			// El costo apareció auditando: `kernelos-pc` figuraba en línea, latiendo, y con CERO
			// servicios. Eso tiene dos causas posibles y opuestas —corre un binario anterior a la
			// enumeración, o corre el nuevo y su enumerador falla— y no había forma de distinguirlas.
			// Es el modo de fallo de todo este track: dos causas distintas con el mismo síntoma.
			//
			// Vacío significa «esta máquina nunca latió con una versión», que es cierto para un Tier B
			// (no corre nuestro binario) y sospechoso para un Tier A. Se omite el campo en vez de
			// mandar "" para que la ausencia se vea como ausencia.
			//
			// NO VA COMO ETIQUETA DE PROMETHEUS, y la decisión es deliberada: la versión cambia en
			// cada despliegue, así que como label crearía una serie nueva por máquina y por versión —
			// exactamente el «ninguna etiqueta puede tomar un valor que rota» que este track sostiene.
			// Acá el lugar correcto es el inventario, que es donde alguien pregunta qué corre dónde.
			if d.AgentVer != "" {
				fila["agent_version"] = d.AgentVer
			}
			// QUÉ SE LE DEBE A QUIEN USA ESTA MÁQUINA. Una política de acceso que se puede
			// escribir y no se puede leer no se puede auditar — y es el mismo hueco exacto que
			// tenía `agent_version`: el dato guardado, y ninguna tool que lo muestre. Se encontró
			// verificando el despliegue, no escribiendo el código.
			//
			// VIAJAN LOS DOS VALORES Y NO UNO. `consentimiento_efectivo` es lo que RIGE, y es lo
			// que hay que mirar para saber qué va a pasar. `consentimiento` sólo aparece si
			// alguien lo DECLARÓ, y su ausencia dice algo distinto de su presencia: «nadie lo
			// decidió, rige el default» no es lo mismo que «alguien puso avisa». Colapsarlos
			// escondería la pregunta que un auditor hace primero — ¿esto lo decidió alguien?
			//
			// Cuando los dos difieren, la máquina no puede preguntarle a nadie y un `pide` se
			// endureció a `prohibido`. Verlo acá evita descubrirlo el día que una sesión no abre.
			fila["consentimiento_efectivo"] = string(d.ConsentimientoEfectivo())
			if d.Consentimiento != "" {
				fila["consentimiento"] = string(d.Consentimiento)
			}
			fila["puede_preguntar"] = d.PuedePreguntar
			// La allowlist EFECTIVA, cuando la hay. Ausente ⇒ sin restricción; presente y vacía ⇒
			// ningún comando. Las dos cosas se dibujan distinto a propósito: si «puede todo» y «no
			// puede nada» compartieran celda, el campo no serviría para decidir nada.
			if permitidos, hay := comandosPermitidos(p, d); hay {
				fila["comandos_permitidos"] = permitidos
			}
			// A23 — QUÉ ACTÚA SOLO SOBRE ESTA MÁQUINA. S10 dejó al cerebro ejecutando comandos sin
			// una persona detrás y eso no se veía en ningún lado: una máquina con auto-heal encima era
			// indistinguible de una sin él. El detalle exige `exec` (misma regla que la bitácora); el
			// CONTEO lo ve cualquiera que vea la máquina, porque «acá pasa algo automático» no es un
			// secreto y ocultarlo dejaría a alguien viendo cambiar una máquina sin ninguna pista.
			// A13 — el id de pantalla, y si es AMBIGUO. Se dice en el inventario y no sólo al fallar
			// una apertura: alguien que revisa la flota tiene que poder ver el problema antes de
			// necesitar la pantalla, no en el momento en que la necesita.
			if d.RustdeskID != "" && PuedeSobreDevice(p, d, fleet.CapScreen) {
				fila["rustdesk_id"] = d.RustdeskID
				if otras, fuera, err := s.engine.QuienMasDiceSer(d.ID, d.RustdeskID, proyecto); err == nil && (len(otras) > 0 || fuera > 0) {
					fila["rustdesk_id_ambiguo"] = true
					fila["rustdesk_id_tambien_en"] = otras
					if fuera > 0 {
						fila["rustdesk_id_fuera_de_alcance"] = fuera
					}
				}
				// Un id que CAMBIÓ no bloquea nada —reinstalar una máquina es normal— pero se ve:
				// la otra explicación posible es que alguien esté mintiendo.
				if !d.RustdeskIDCambiado.IsZero() {
					fila["rustdesk_id_cambio"] = d.RustdeskIDCambiado.UTC().Format(time.RFC3339)
					fila["rustdesk_id_previo"] = d.RustdeskIDPrevio
				}
			}
			// A18 — UNA CAPACIDAD INERTE NO PUEDE DIBUJARSE IGUAL QUE UNA VIVA. Es la misma lección
			// que `puede_actuar` en las políticas (A23): `screen` concedido sobre un tier sin motor
			// se veía idéntico a `screen` sobre un Tier A, y `puedo` lo listaba como ejercible. Se
			// dice acá y no sólo al fallar la apertura, por el mismo motivo que el id ambiguo: quien
			// revisa la flota tiene que poder verlo ANTES de necesitar la pantalla.
			if PuedeSobreDevice(p, d, fleet.CapScreen) {
				if _, hayMotor := fleet.MotorDePantalla(d.Tier); !hayMotor {
					fila["pantalla_sin_motor"] = true
				}
			}
			if detalle, total := s.politicasSobre(p, d); total > 0 {
				fila["politicas_activas"] = total
				if detalle != nil {
					fila["politicas"] = detalle
				}
			}
			if d.LastSeen.IsZero() {
				// Nunca latió. Decirlo explícito evita que un `last_seen` vacío se lea como un
				// error de serialización.
				fila["last_seen"] = nil
				fila["nunca_latio"] = true
			} else {
				fila["last_seen"] = d.LastSeen.UTC().Format(time.RFC3339)
				fila["silencio_segundos"] = int(ahora.Sub(d.LastSeen).Seconds())
			}
			filas = append(filas, fila)
		}
	}

	res := map[string]interface{}{
		"projects": proyectos,
		"total":    len(filas),
		"online":   enLinea,
		"devices":  filas,
	}
	// `project_id` SE MANTIENE cuando hay uno solo. Es el 99 % de los llamadores y sacarlo sería
	// romperlos por una generalización que no les toca.
	if len(proyectos) == 1 {
		res["project_id"] = proyectos[0]
	}
	// Un barrido recortado se DICE. Un panel que muestra 64 proyectos de 80 sin avisar enseña a
	// creer que ésos son todos.
	if truncado {
		res["proyectos_truncados"] = true
	}
	// El umbral GLOBAL sólo existe si alguien lo impuso. Publicar uno cuando cada tier usa el
	// suyo sería peor que no publicar nada: el número saldría en la respuesta y contradiría, en
	// silencio, el `online` de la mitad de las filas.
	if umbralExplicito > 0 {
		res["umbral_segundos"] = int(umbralExplicito.Seconds())
	}
	return jsonResult(res)
}

// toolFleetRevoke es el KILL-SWITCH: la máquina deja de autenticar en el acto y su fila queda
// para la auditoría. Admin, como el alta.
func (s *McpServer) toolFleetRevoke(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_fleet_revoke corta el acceso de una máquina: requiere un principal admin")
	}
	var args struct {
		Name    string `json:"name"`
		Project string `json:"project"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	if strings.TrimSpace(args.Name) == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `name`: el nombre del dispositivo a dar de baja")
	}
	proyecto, ok := writeOriginFor(p, args.Project)
	if !ok || strings.TrimSpace(proyecto) == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto del dispositivo: declaralo en `project`")
	}

	revocado, err := s.engine.RevocarDevice(proyecto, args.Name)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	if !revocado {
		return nil, rpcErrorf(codeInvalidParams, "no hay un dispositivo activo llamado %q en el proyecto %q", strings.TrimSpace(args.Name), proyecto)
	}
	return jsonResult(map[string]interface{}{
		"revoked":    true,
		"name":       strings.TrimSpace(args.Name),
		"project_id": proyecto,
		"efecto":     "deja de autenticar en el acto; la fila queda para la auditoría",
	})
}

// fleetReadScopeFor decide DE QUÉ proyecto se lista la flota. Misma disciplina que brandScopeFor:
// el argumento `project` sólo lo respeta un principal read=all (sala de mando / cabina), que ve
// todos los tenants; uno acotado lista el suyo, declare lo que declare.
//
// Devuelve "" si no hay proyecto resoluble, y el llamador convierte eso en un ERROR en vez de
// listar. Es a propósito: una vista federada de TODA la flota tiene que ser una decisión visible
// con su propio método, no el efecto colateral de un parámetro sin llenar. Es la misma lección
// que ya dejó ListarDevices en S1.
func fleetReadScopeFor(p *Principal, declarado string) string {
	declarado = strings.TrimSpace(declarado)
	if p == nil {
		return declarado // stdio local: confianza local, el llamador manda
	}
	if declarado != "" {
		if read, _ := p.caps(); read == ReadAll {
			return declarado
		}
	}
	return p.ProjectID
}

// proyectosParaLeer resuelve QUÉ proyectos responde una tool de LECTURA de flota.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL PANEL NO PODÍA VER NADA, Y EL SÍNTOMA CULPABA AL CEREBRO
//
// `fleetReadScopeFor` cae al `ProjectID` del principal cuando no se declara un proyecto. Para el
// principal del panel —`read: all`, `project_id: ""` a propósito, porque no es de un tenant— eso
// daba vacío, y las tres tools contestaban «no se pudo determinar de qué proyecto». El panel lo
// dibujaba como `estado: caido`, que se lee como «el cerebro se cayó» estando el cerebro
// perfecto. Medido: `musubi_fleet_list`, `_metrics` y `_services` fallaban las tres igual.
//
// La salida NO es aflojar el WHERE por proyecto: eso sería un «listar todo» y se llevaría puesto
// el aislamiento por tenant. Es enumerar los proyectos y consultar CADA UNO POR SEPARADO, con su
// WHERE intacto — que es exactamente lo que el export federado a Prometheus ya hacía desde S11.
// Quien tiene `read: all` recibe todos porque eso es lo que su credencial concede; quien está
// acotado sigue viendo el suyo y nada más.
func (s *McpServer) proyectosParaLeer(p *Principal, declarado string) (proyectos []string, truncado bool) {
	if pr := fleetReadScopeFor(p, declarado); pr != "" {
		return []string{pr}, false
	}
	return proyectosVisibles(s.engine, p)
}

// limpiarTags saca vacíos y espacios. Las tags son texto libre del administrador: no se validan
// contra un enum (agrupar por «sala», «cliente-x» o «crítico» es cosa de quien opera), pero sí se
// normalizan para que la columna CSV no acumule basura.
func limpiarTags(in []string) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		if t = strings.TrimSpace(t); t != "" && !strings.Contains(t, ",") {
			out = append(out, t)
		}
	}
	return out
}

// capsComoLista serializa capacidades para el JSON de salida. Lista y no CSV: el consumidor es un
// panel o un agente, y una lista no lo obliga a partir strings.
func capsComoLista(cs []fleet.Cap) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = string(c)
	}
	return out
}

// toolFleetMetrics devuelve la telemetría de la flota. Es el PRIMER CONSUMIDOR REAL de la
// compuerta de S3: sólo aparecen las máquinas donde esta credencial tiene concedida `metrics`.
//
// Que la lista salga VACÍA para alguien sin concesiones no es un bug ni una cortesía: es la
// compuerta funcionando. `musubi_fleet_list` deja ver el INVENTARIO (qué máquinas hay) y esta
// tool deja ver el ESTADO (cómo están) — son dos permisos distintos porque son dos cosas
// distintas, y el uso de CPU de un servidor dice bastante más sobre un negocio que su nombre.
func (s *McpServer) toolFleetMetrics(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project string `json:"project"`
		Device  string `json:"device"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	proyectos, truncado := s.proyectosParaLeer(p, args.Project)
	if len(proyectos) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar de qué proyecto leer las métricas: declaralo en `project`")
	}

	filtro := strings.TrimSpace(args.Device)
	ahora := time.Now()
	filas := make([]map[string]interface{}, 0)
	sinPermiso, sinMuestra := 0, 0

	for _, proyecto := range proyectos {
		devices, err := s.engine.ListarDevices(proyecto, false)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		for _, d := range devices {
			if filtro != "" && d.Name != filtro {
				continue
			}
			if !PuedeSobreDevice(p, d, fleet.CapMetrics) {
				sinPermiso++
				continue
			}
			if d.UltimaMuestra == nil {
				sinMuestra++
				continue
			}
			fila := filaDeMetricas(d, *d.UltimaMuestra, ahora, s.umbralEnLinea(d))
			fila["project"] = d.ProjectID
			filas = append(filas, fila)
		}
	}

	res := map[string]interface{}{
		"projects": proyectos,
		"devices":  filas,
	}
	if len(proyectos) == 1 {
		res["project_id"] = proyectos[0]
	}
	if truncado {
		res["proyectos_truncados"] = true
	}
	// Se DICE cuántas quedaron fuera y por qué. Una lista corta sin explicación se lee como
	// «no hay más máquinas», que es distinto de «no podés verlas» y de «todavía no reportaron».
	if sinPermiso > 0 {
		res["sin_permiso"] = sinPermiso
	}
	if sinMuestra > 0 {
		res["aun_sin_reportar"] = sinMuestra
	}
	return jsonResult(res)
}

// filaDeMetricas arma la fila de una máquina. Los porcentajes se derivan acá y no en el agente:
// el agente manda lo que MIDIÓ (bytes, jiffies) y el cerebro deriva lo que se MUESTRA. Así un
// agente viejo no queda mostrando porcentajes calculados con una fórmula que ya cambió.
func filaDeMetricas(d fleet.Device, m fleet.Muestra, ahora time.Time, umbral time.Duration) map[string]interface{} {
	fila := map[string]interface{}{
		"name":         d.Name,
		"tier":         string(d.Tier),
		"os":           d.OS,
		"online":       d.EnLinea(ahora, umbral),
		"tomada":       m.Tomada.UTC().Format(time.RFC3339),
		"antiguedad_s": int(ahora.Sub(m.Tomada).Seconds()),
		"num_cpu":      m.NumCPU,
		"uptime_seg":   m.UptimeSeg,
		"mem_total":    m.MemTotal,
		"mem_usada":    m.MemUsada,
		"disco_total":  m.DiscoTotal,
		"disco_usado":  m.DiscoUsado,
		"disco_libre":  m.DiscoDisponible,
	}
	// Los campos que pueden ser DESCONOCIDOS viajan como null, nunca como 0 (D1/D3). Un 0
	// inventado es indistinguible de un 0 medido, y la diferencia importa justo cuando alguien
	// está mirando el panel para entender una caída.
	fila["cpu_pct"] = m.CPUPct
	fila["temp_c"] = m.TempC
	// La carga no existe en Windows: nil, no 0.
	fila["load1"] = m.Load1
	fila["load5"] = m.Load5
	fila["load15"] = m.Load15
	// mem_libre es *uint64: nil ⇒ null. Windows y macOS no la exponen, y decir 0 sería decir «no
	// le queda nada de RAM libre», que es lo contrario de «no lo sé».
	fila["mem_libre"] = m.MemLibre
	// num_procesos es un entero, así que el cero se traduce ACÁ. Copiarlo crudo pintaría «esta
	// máquina no tiene procesos» en todo macOS y en todo agente viejo — y en Prometheus, una
	// caída a cero en cada gráfico. (`num_cpu` arriba sí se copia crudo y tiene el mismo
	// problema: es el mismo arreglo pero otro diff, anotado como cabo.)
	fila["num_procesos"] = enteroONull(m.NumProcesos)
	fila["mem_pct"] = fleet.PctUsado(m.MemUsada, m.MemTotal)
	fila["disco_pct"] = fleet.PctUsado(m.DiscoUsado, m.DiscoTotal)
	fila["swap_pct"] = fleet.PctUsado(m.SwapUsada, m.SwapTotal)
	return fila
}

// enteroONull traduce el «0 = no medido» de un contador entero al null del JSON.
//
// Es el gemelo de lo que los punteros hacen solos para los float: un contador que no se pudo
// medir tiene que llegar a la pantalla como un hueco, no como un cero que se cree.
func enteroONull(n int) interface{} {
	if n <= 0 {
		return nil
	}
	return n
}
