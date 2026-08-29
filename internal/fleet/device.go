package fleet

// fleet es el DOMINIO de la flota: qué es un dispositivo controlado, de qué tier es, qué se le
// puede pedir y cuándo se lo considera en línea. Es dominio PURO —sin base, sin red, sin OS—,
// igual que internal/skills: la persistencia vive en internal/memory y el transporte en
// internal/mcp. La dirección de dependencias es memory → fleet, nunca al revés.
//
// POR QUÉ UN PAQUETE NUEVO Y NO UNA ETIQUETA SOBRE LO QUE YA HAY. Musubi ya sabe de PROYECTOS y
// de PRINCIPALS (personas con token). Una máquina no es ninguna de las dos: sobrevive a la sesión,
// no tiene rol de memoria, y lo que se le puede pedir depende de su hardware, no de su permiso.
// Meterla como un principal más haría que administrar la memoria del equipo otorgue, de rebote,
// control sobre las máquinas — el puente de privilegio que este track evita a propósito (ver A5).

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Tier es cuánto se puede hacer sobre un dispositivo, y lo decide CÓMO se llega a él, no una
// preferencia. Está en el modelo porque "controlar todo dispositivo" no es homogéneo: prometer lo
// mismo para un portátil con agente y para un router por SNMP es prometer lo que no se puede dar.
type Tier string

const (
	// TierAgente: corre el binario `musubi agent` en el host (Linux, Windows, macOS).
	// Control máximo: telemetría, ejecución y pantalla.
	TierAgente Tier = "A"
	// TierProtocolo: sin binario en el device. Se lo maneja por su protocolo nativo
	// (SSH, SNMP, MQTT, Redfish/IPMI): routers, NAS, IoT, servers ajenos.
	TierProtocolo Tier = "B"
	// TierMovil: Android/iOS. Techo real y declarado — un móvil no entrega una shell
	// como un servidor, y iOS apenas deja mirar.
	TierMovil Tier = "C"
)

// Cap es una capacidad concedida sobre un dispositivo. Son los tres planos del track, y se
// conceden POR SEPARADO: mirar las métricas de una máquina y poder ejecutar en ella son permisos
// de peso muy distinto, y colapsarlos en uno solo es cómo un panel de monitoreo se convierte sin
// querer en una consola remota.
type Cap string

const (
	CapMetrics Cap = "metrics" // leer telemetría del host
	CapExec    Cap = "exec"    // ejecutar comandos / abrir terminal
	CapScreen  Cap = "screen"  // sesión de pantalla CON control
	// CapScreenView es MIRAR la pantalla sin poder tocarla, y es una capacidad aparte porque
	// mirar y controlar son actos distintos con consecuencias distintas.
	//
	// Hasta acá `screen` era un solo bit, así que dárselo a alguien para que DIAGNOSTICARA —ver
	// qué pasa en la pantalla de una máquina— le daba también el teclado y el mouse. La
	// alternativa era no dárselo, y entonces no podía ayudar. MeshCentral separa exactamente esto
	// (`MESHRIGHT_REMOTECONTROL` contra `MESHRIGHT_REMOTEVIEWONLY`) y tiene además un tercer
	// grado (`DESKLIMITEDINPUT`) que acá no hace falta todavía.
	//
	// COMPATIBILIDAD HACIA ATRÁS, que no es un detalle: `screen` sigue significando EXACTAMENTE
	// lo que significaba —control—, así que ninguna concesión existente cambia de sentido al
	// desplegar esto. Lo nuevo es una capacidad MÁS ACOTADA, no una redefinición de la vieja.
	// Redefinir `screen` como «sólo mirar» habría sacado silenciosamente el control a todos los
	// que hoy lo tienen, y nadie se habría enterado hasta necesitarlo.
	CapScreenView Cap = "screen:view" // sesión de pantalla, SÓLO mirar
	// CapShell es la SHELL INTERACTIVA, y es una capacidad aparte de CapExec a propósito (S5b · T1).
	//
	// S10 partió `exec` en dos permisos: poder ejecutar (la concesión) y poder ejecutar CUALQUIER
	// COSA (la allowlist por comando). Una shell interactiva es el tercero, y es el que se lleva
	// puestos a los otros dos: quien obtiene un prompt corre lo que quiera, las veces que quiera,
	// sin que nadie vuelva a mirar un argv.
	//
	// Gatearla con `exec` habría convertido la allowlist en decoración — y peor, en decoración en
	// la que alguien confía: un principal acotado a `journalctl` obtendría, tecleando otra cosa,
	// exactamente lo que la allowlist le negaba.
	CapShell Cap = "shell" // shell interactiva (pty)
)

// capsPorTier es la MATRIZ del invariante A4: qué sabe honrar cada tier.
//
// Las ausencias son las que importan y ninguna es arbitraria:
//   - B no tiene `screen`: un switch administrado por SNMP no tiene framebuffer que capturar.
//     No es una restricción de política, es que no existe la cosa.
//   - C no tiene `exec`: iOS no da shell, y en Android depende de que ADB esté habilitado —
//     o sea que no se puede prometer al dar de alta. Cuando S8 modele el companion de Android,
//     será una capacidad declarada aparte, con su propia prueba.
//   - C tampoco tiene `shell` (S5b), por exactamente el mismo motivo que no tiene `exec`: si no
//     se puede prometer ejecutar un comando, menos se puede prometer un prompt.
//
// Conceder una capacidad fuera de esta matriz FALLA EN EL ALTA. La alternativa —aceptarla y
// fallar recién cuando alguien la use— cambia un error de configuración visible por un bug
// intermitente en producción.
var capsPorTier = map[Tier][]Cap{
	TierAgente:    {CapMetrics, CapExec, CapScreen, CapScreenView, CapShell},
	TierProtocolo: {CapMetrics, CapExec, CapShell},
	TierMovil:     {CapMetrics, CapScreen, CapScreenView},
}

// Errores del dominio. Se exportan para que la capa de transporte los traduzca a códigos JSON-RPC
// sin comparar cadenas.
var (
	ErrTierDesconocido = errors.New("tier desconocido")
	ErrCapDesconocida  = errors.New("capacidad desconocida")
	ErrCapFueraDeTier  = errors.New("el tier no sabe honrar esa capacidad")
	ErrSinProyecto     = errors.New("un dispositivo sin project_id sería visible desde todos los proyectos")
	ErrSinNombre       = errors.New("el dispositivo necesita un nombre")
)

// Device es un dispositivo controlado, tal como lo conoce el registro.
//
// Lo que NO tiene, y es deliberado:
//   - NO tiene campo `online`. Se DERIVA de LastSeen (ver EnLinea y el invariante A8): un
//     booleano guardado se queda en `true` para siempre cuando la máquina muere de golpe, que
//     es precisamente cuando querés saber que se cayó.
//   - NO tiene el token. El registro guarda su SHA-256 y nada más (A2).
type Device struct {
	ID        string // lo asigna el CEREBRO al dar de alta; el cliente nunca lo elige (A1)
	Name      string // legible, único dentro del proyecto
	ProjectID string // tenancy: el mismo eje que aísla la memoria (A6)
	Tier      Tier
	Caps      []Cap    // capacidades CONCEDIDAS; el cero no permite nada (A5)
	OS        string   // linux | windows | darwin | android | ios | otro
	Arch      string   // amd64 | arm64 | ...
	Address   string   // dirección por la que se lo alcanza (normalmente el tailnet)
	AgentVer  string   // versión del agente, vacío en Tier B
	Tags      []string // etiquetas libres para agrupar (sala, cliente, criticidad)

	// Consentimiento es la POLÍTICA: qué se le debe a quien está usando esta máquina cuando
	// alguien pide entrar. Vacío = no declarado, y lo resuelve el default del dominio — no se
	// guarda un grado concreto para que cambiar el default no deje las filas viejas atrás.
	Consentimiento Consentimiento
	// PuedePreguntar es la CAPACIDAD MEDIDA: si en esta máquina hay dónde dibujar un diálogo y
	// quién lo conteste. La reporta el agente. Falso mientras nadie la haya medido, que es lo
	// honesto: afirmar que se puede preguntar sin haberlo comprobado haría que `pide` se comporte
	// como si hubiera alguien del otro lado.
	PuedePreguntar bool

	EnrolledAt time.Time
	LastSeen   time.Time // cero = nunca latió
	Revoked    bool

	// RustdeskID es el identificador PÚBLICO del cliente RustDesk de esta máquina (S6). Lo
	// reporta el agente. No es un secreto: sin la contraseña de sesión no sirve para entrar.
	RustdeskID string
	// RustdeskIDPrevio y RustdeskIDCambiado guardan que ese id SE MOVIÓ (S6b · A13). Lo reporta
	// la propia máquina, así que es entrada no confiable: un id que cambia solo tiene dos
	// explicaciones —se reinstaló, o alguien miente— y las dos ameritan quedar escritas.
	RustdeskIDPrevio   string
	RustdeskIDCambiado time.Time

	// UltimaMuestra es la telemetría más reciente que reportó la máquina (S4), o nil si nunca
	// reportó. Es el PRESENTE, no una serie: la historia, si hace falta, la guarda Prometheus.
	UltimaMuestra *Muestra
}

// NormalizarTier acepta la letra o el nombre largo, en cualquier caja. Es tolerante en la ENTRADA
// (un humano escribe "agente" o "a") y estricto en el MODELO: hacia adentro sólo circulan A, B, C.
func NormalizarTier(s string) (Tier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "a", "agente", "agent", "nativo":
		return TierAgente, nil
	case "b", "protocolo", "protocol", "sin-agente", "agentless":
		return TierProtocolo, nil
	case "c", "movil", "móvil", "mobile":
		return TierMovil, nil
	default:
		return "", fmt.Errorf("%w: %q (esperaba A=agente nativo, B=por protocolo, C=móvil)", ErrTierDesconocido, s)
	}
}

// NormalizarCaps valida y deduplica una lista de capacidades, devolviéndola en orden estable.
// El orden estable importa para que la fila guardada no cambie según cómo la escribió el cliente.
func NormalizarCaps(in []string) ([]Cap, error) {
	vistas := make(map[Cap]bool, len(in))
	for _, s := range in {
		c := Cap(strings.ToLower(strings.TrimSpace(s)))
		if c == "" {
			continue
		}
		switch c {
		case CapMetrics, CapExec, CapScreen, CapScreenView, CapShell:
			vistas[c] = true
		default:
			return nil, fmt.Errorf("%w: %q (esperaba metrics, exec o screen)", ErrCapDesconocida, s)
		}
	}
	return ordenar(vistas), nil
}

// ordenar devuelve las capacidades en el orden canónico (el de la matriz), no alfabético:
// metrics < exec < screen es también el orden de poder, y así se lee la fila.
func ordenar(set map[Cap]bool) []Cap {
	orden := map[Cap]int{CapMetrics: 0, CapExec: 1, CapScreenView: 2, CapScreen: 3, CapShell: 4}
	out := make([]Cap, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return orden[out[i]] < orden[out[j]] })
	return out
}

// CapsDelTier devuelve las capacidades que ese tier sabe honrar. Devuelve una copia: el llamador
// no puede mutar la matriz.
func CapsDelTier(t Tier) []Cap {
	src := capsPorTier[t]
	out := make([]Cap, len(src))
	copy(out, src)
	return out
}

// TierAdmite dice si el tier puede honrar la capacidad.
func TierAdmite(t Tier, c Cap) bool {
	for _, ok := range capsPorTier[t] {
		if ok == c {
			return true
		}
	}
	return false
}

// Permite es la pregunta que hace el camino caliente: ¿este dispositivo tiene concedida esta
// capacidad, ahora mismo?
//
// Fail-closed por construcción (A5): un Device cero —el que sale de un scan fallido, de un test
// mal armado o de un JSON incompleto— no permite NADA. Y un dispositivo revocado tampoco, aunque
// la fila conserve sus capacidades: revocar tiene que cortar sin depender de que alguien además
// vacíe la lista.
// ConsentimientoEfectivo cruza la POLÍTICA de esta máquina con lo que la máquina PUEDE hacer.
//
// Es el único lugar donde se juntan las dos, y por eso está acá y no en el llamador: si cada
// camino que abre una sesión resolviera por su cuenta, el de pantalla y el de shell terminarían
// discrepando el día que uno de los dos se toque. Las `extra` son las otras fuentes —el proyecto,
// un grupo— y se acumulan con la regla de siempre: gana la más restrictiva.
func (d Device) ConsentimientoEfectivo(extra ...Consentimiento) Consentimiento {
	fuentes := append([]Consentimiento{d.Consentimiento}, extra...)
	return ResolverConsentimiento(fuentes...).AplicarACapacidadDePreguntar(d.PuedePreguntar)
}

// Implica dice si tener `otorgada` alcanza para lo que pide `pedida`.
//
// HAY UNA SOLA IMPLICACIÓN Y NO ES SIMÉTRICA: `screen` (controlar) alcanza para `screen:view`
// (mirar), porque quien mueve el mouse ya está viendo la pantalla — negarle mirar sería un
// absurdo. Al revés NO, y ésa es toda la razón de haber partido la capacidad: si `screen:view`
// alcanzara para controlar, la capacidad nueva no acotaría nada y sería decoración.
//
// Se escribe como una función y no como un mapa de «caps equivalentes» a propósito. Un mapa
// invita a agregar pares simétricos sin pensar, y la asimetría es justamente el punto.
func Implica(otorgada, pedida Cap) bool {
	if otorgada == pedida {
		return true
	}
	return otorgada == CapScreen && pedida == CapScreenView
}

func (d Device) Permite(c Cap) bool {
	if d.Revoked {
		return false
	}
	for _, tiene := range d.Caps {
		if tiene == c {
			// Cinturón y tirantes: aunque la fila diga `screen`, un Tier B no la puede honrar.
			// Sin esto, una fila escrita por una versión anterior (o a mano) elude A4.
			return TierAdmite(d.Tier, c)
		}
	}
	return false
}

// EnLinea deriva el estado de conexión de la última señal de vida. NO hay campo guardado, y esa
// ausencia es el invariante A8.
//
// El umbral lo elige QUIEN PREGUNTA, y por eso no hay un default acá: un panel que refresca cada
// 5 s y una alerta que despierta a alguien de madrugada no pueden compartir la definición de
// "caído".
//
// SOBRE LA GUARDA `LastSeen.IsZero()`, porque es fácil malinterpretar para qué está. NO es la que
// hace que un dispositivo recién dado de alta figure caído: `time.Duration` satura en ~292 años
// (max int64 de nanosegundos), así que `ahora.Sub(cero)` da 2562047h y supera CUALQUIER umbral
// realista. Ese caso ya sale fail-closed solo, por aritmética.
//
// La guarda existe para el otro caso, que sí es alcanzable: un llamador que pasa un `ahora` CERO
// —un reloj inyectado sin inicializar en un test, una estructura de config a medio llenar—. Ahí
// `cero.Sub(cero)` es 0, entra dentro de cualquier umbral, y sin la guarda un dispositivo que
// NUNCA latió se reportaría EN LÍNEA. Está fijada por TestEnLineaConRelojCeroNoInventaVida.
func (d Device) EnLinea(ahora time.Time, umbral time.Duration) bool {
	if d.Revoked || d.LastSeen.IsZero() || umbral <= 0 {
		return false
	}
	return ahora.Sub(d.LastSeen) <= umbral
}

// ValidarAlta chequea lo que tiene que ser cierto ANTES de que el dispositivo exista en el
// registro. Todo lo que valida es fail-closed: ante la duda, no se da de alta.
func ValidarAlta(d Device) error {
	if strings.TrimSpace(d.Name) == "" {
		return ErrSinNombre
	}
	// A6 — sin proyecto, la fila se ve desde todos los tenants. Ya pasó con las observaciones
	// (2 filas de test contaminando 3 proyectos, medido en el cerebro real); no se repite.
	if strings.TrimSpace(d.ProjectID) == "" {
		return fmt.Errorf("%w (dispositivo %q)", ErrSinProyecto, d.Name)
	}
	if _, ok := capsPorTier[d.Tier]; !ok {
		return fmt.Errorf("%w: %q", ErrTierDesconocido, d.Tier)
	}
	// A4 — no se conceden promesas que el tier no puede cumplir.
	for _, c := range d.Caps {
		if !TierAdmite(d.Tier, c) {
			return fmt.Errorf("%w: tier %s no admite %q (admite: %s)", ErrCapFueraDeTier, d.Tier, c, capsComoTexto(CapsDelTier(d.Tier)))
		}
	}
	return nil
}

// capsComoTexto serializa capacidades a CSV canónico. Lo usan los mensajes de error y la capa de
// persistencia — una sola forma de escribirlas, para que la fila y el error digan lo mismo.
func capsComoTexto(cs []Cap) string {
	if len(cs) == 0 {
		return ""
	}
	partes := make([]string, len(cs))
	for i, c := range cs {
		partes[i] = string(c)
	}
	return strings.Join(partes, ",")
}

// CapsComoTexto es capsComoTexto para los otros paquetes (persistencia y transporte).
func CapsComoTexto(cs []Cap) string { return capsComoTexto(cs) }

// CapsDesdeTexto revierte CapsComoTexto. Tolera basura histórica en la columna DESCARTÁNDOLA en
// vez de fallar: una capacidad que este binario no conoce no se puede honrar, así que tratarla
// como ausente es la lectura fail-closed. Que una fila ilegible impidiera LISTAR la flota sería
// peor que ignorar un campo.
func CapsDesdeTexto(s string) []Cap {
	set := make(map[Cap]bool)
	for _, p := range strings.Split(s, ",") {
		switch c := Cap(strings.ToLower(strings.TrimSpace(p))); c {
		case CapMetrics, CapExec, CapScreen, CapScreenView, CapShell:
			set[c] = true
		}
	}
	return ordenar(set)
}

// ── Credencial del dispositivo ──────────────────────────────────────────────────────────────

// NuevoToken genera la credencial de un dispositivo: 32 bytes de crypto/rand en hex.
//
// Vive en el DOMINIO y no en la capa de persistencia a propósito: así el único lugar del código
// que puede fabricar una credencial es también el único que define cómo se guarda (HashToken), y
// no hay forma de que un camino nuevo invente su propio esquema más débil.
func NuevoToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("no se pudo generar la credencial del dispositivo: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken devuelve el SHA-256 hex de una credencial. El registro guarda ESTO, nunca el token
// crudo (invariante A2), igual que principals.yaml hace con las personas.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// MotorDePantalla dice QUÉ motor sirve la pantalla de un tier, y si Musubi tiene alguno.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SON DOS PREGUNTAS DISTINTAS Y CONFUNDIRLAS DEJABA ABIERTA UNA MENTIRA
//
// capsPorTier responde «¿este tier SABE honrar `screen`?». Para un Android la respuesta es sí:
// tiene framebuffer, y por eso la matriz se lo concede. Esta función responde la otra mitad, que
// hasta hoy no preguntaba nadie: «¿y Musubi tiene con qué abrirla?».
//
// Mientras nadie preguntara la segunda, `musubi_fleet_screen` sobre un Tier C pasaba la
// autorización, pasaba «en línea» (la sonda le escribe last_seen), ACUÑABA la contraseña, la
// mostraba una única vez, y encolaba `musubi:pantalla` en una cola que en Tier C NO DRENA NADIE
// —el agente es de Tier A—. El comando vencía a los 15 minutos y la bitácora quedaba diciendo
// que se abrió una sesión de pantalla. Una promesa que el código hacía y no cumplía.
//
// El motor de Android es scrcpy sobre ADB, que es OTRO distinto del de RustDesk (A18 → S8b).
// Hasta que exista, la respuesta honesta es negarse.
func MotorDePantalla(t Tier) (string, bool) {
	switch t {
	case TierAgente:
		return "rustdesk", true
	case TierMovil:
		// scrcpy sobre ADB — todavía no está. Ver specs/control-de-flota/ABIERTO.md (A18).
		return "", false
	default:
		// Tier B ni siquiera llega acá: no tiene `screen` en la matriz, porque no tiene
		// framebuffer. Si aparece un tier nuevo, cae acá y se niega hasta que alguien decida.
		return "", false
	}
}
