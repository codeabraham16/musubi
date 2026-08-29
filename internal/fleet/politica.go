package fleet

// politica.go es el dominio puro de S10: qué comandos deja pasar una allowlist, y cuándo una
// política decide actuar. Sin base de datos, sin red y sin reloj propio — todo entra por
// parámetro, que es lo que hace que estas dos decisiones se puedan probar de verdad.
//
// Las dos cosas viven en el mismo archivo porque se necesitan mutuamente: una política es
// ejecución remota SIN una persona detrás, y lo único que la hace defendible es que su alcance se
// pueda acotar a un puñado de comandos (spec I11 + I8).

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ── La allowlist ────────────────────────────────────────────────────────────────────────────

// PermiteArgv dice si una lista de comandos permitidos deja pasar este argv.
//
// EL MATCH ES SOBRE argv[0] Y EXACTO (I10). Sin basename y sin globs, y la razón no es purismo:
// comparar por basename dejaría pasar `/tmp/evil/systemctl` contra una entrada `systemctl` — o
// sea, el bypass exacto que la allowlist viene a cerrar. Quien quiera permitir la ruta absoluta,
// que la escriba; es una decisión, no un descuido.
//
// Una lista VACÍA no permite nada. Es el bug clásico de las allowlists (el `len == 0 ⇒ pasa
// todo` que parece defensivo y es lo contrario), y acá es un caso de prueba con nombre propio.
// «Sin restricción» se expresa NO teniendo lista, que es una situación distinta y la resuelve
// quien llama, no esta función.
func PermiteArgv(permitidos []string, argv []string) bool {
	limpio := LimpiarArgv(argv)
	if len(limpio) == 0 {
		return false
	}
	for _, p := range permitidos {
		if strings.TrimSpace(p) == limpio[0] {
			return true
		}
	}
	return false
}

// interpretes son los programas que, al permitirlos, permiten TODO lo que puedan lanzar.
//
// No se bloquean: `bash` en una allowlist puede ser exactamente lo que alguien quiere, y decidir
// por esa persona sería incorrecto. Pero callarlo sería peor — la allowlist se escribe una vez y
// se lee dentro de dos años—, así que el arranque lo dice con nombre y máquina (I10b).
var interpretes = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "ksh": true, "dash": true, "fish": true,
	"cmd": true, "cmd.exe": true, "powershell": true, "powershell.exe": true, "pwsh": true,
	"python": true, "python3": true, "perl": true, "ruby": true, "node": true, "php": true,
	"awk": true, "gawk": true, "env": true, "xargs": true, "find": true, "ssh": true,
	"sudo": true, "doas": true, "su": true, "nohup": true, "setsid": true, "timeout": true,
}

// EsInterprete dice si permitir este comando equivale a permitir cualquier otro. Compara por
// basename A PROPÓSITO, al revés que PermiteArgv: acá no se está autorizando nada, se está
// buscando a quién avisarle, y `/usr/bin/bash` merece el mismo aviso que `bash`.
func EsInterprete(comando string) bool {
	c := strings.TrimSpace(comando)
	if c == "" {
		return false
	}
	return interpretes[strings.ToLower(filepath.Base(c))]
}

// ── Las políticas ───────────────────────────────────────────────────────────────────────────

// Condicion es QUÉ mira una política. Es un enum acotado sobre los campos de la muestra y no un
// mini-lenguaje de expresiones: un evaluador de expresiones que decide qué comando correr en una
// máquina ajena es una superficie que todavía no se justifica (ver «Lo que queda fuera»).
type Condicion string

const (
	CondDiscoPct      Condicion = "disco_pct"       // % del disco raíz OCUPADO
	CondDiscoLibrePct Condicion = "disco_libre_pct" // % ESCRIBIBLE que queda (dispara al BAJAR)
	CondMemPct        Condicion = "mem_pct"
	CondCPUPct        Condicion = "cpu_pct"
	CondCargaPorCore  Condicion = "carga_por_core"
	CondTempC         Condicion = "temp_c"

	// ── CONDICIONES SOBRE UN SERVICIO (A44) ────────────────────────────────────────────────
	//
	// Las de arriba miran una MÉTRICA DEL HOST y se cumplen con un número. Éstas miran el
	// estado de UN servicio concreto adentro de una máquina, y por eso exigen el campo
	// `Servicio`: sin él, «reiniciá el que se cayó» obligaría a meter el nombre del servicio
	// dentro del comando en tiempo de ejecución, y ahí está el problema — ver el comentario de
	// `Servicio`.
	//
	// CondServicioCaido se cumple cuando el servicio está `fallado` o `detenido`. NO se cumple
	// con `desconocido`: una máquina que no pudo enumerar sus servicios no está diciendo que el
	// postgres esté caído, y reiniciar algo por no haber podido mirarlo es exactamente el
	// automatismo que nadie quiere. Es la misma asimetría que sostiene toda la tool de servicios.
	CondServicioCaido Condicion = "servicio_caido"
	// CondServicioReinicios se cumple cuando el contador de reinicios supera el umbral. Sirve
	// para lo que `servicio_caido` no ve: un servicio que ANDA porque su supervisor lo levanta
	// cada treinta segundos. Está corriendo en cada mirada y no está sano.
	CondServicioReinicios Condicion = "servicio_reinicios"
)

// CooldownMin es el piso del cooldown. Una política sin cooldown dispararía en CADA tick hasta
// que la métrica baje —y la métrica no baja hasta que el comando termine—, así que el caso sin
// cooldown no es «más reactivo»: es una tormenta de comandos idénticos (I14).
const CooldownMin = time.Minute

// CooldownDefault es lo que se usa cuando la política no lo declara.
const CooldownDefault = 30 * time.Minute

// Politica es «si esta máquina cruza este número, corré esto — con la autoridad de esta persona».
type Politica struct {
	Nombre string

	// Principal es de QUIÉN es la autoridad con la que actúa. Obligatorio, y es el invariante
	// central del slice (I11): una política no tiene poder propio. Toda su capacidad es la de
	// esta credencial, evaluada por la MISMA compuerta que evalúa a una persona.
	Principal string

	Cuando Condicion
	Supera float64

	// Sobre son selectores de máquina, igual que en las concesiones: "*" o nombres exactos.
	Sobre []string

	// Servicio es QUÉ servicio mira esta política. Obligatorio para las condiciones de servicio
	// y prohibido para las de host.
	//
	// ────────────────────────────────────────────────────────────────────────────────────────
	// SE NOMBRA UNO SOLO, Y NO SE PERMITE «EL QUE SE HAYA CAÍDO»
	//
	// La forma cómoda sería una política sin `servicio` que actúe sobre cualquiera que falle,
	// con el nombre sustituido dentro del comando (`systemctl restart {{servicio}}`). Se
	// descarta, y el motivo no es purismo:
	//
	// El nombre del servicio lo REPORTA LA MÁQUINA. Es entrada no confiable, del mismo modo que
	// su telemetría. Sustituirlo dentro de un argv significa que un dato de la máquina termina
	// siendo un ARGUMENTO de un comando que el cerebro ejecuta con la autoridad de un principal
	// — y la allowlist por comando, que es lo que acota `exec`, se validaría ANTES de saber qué
	// va a ejecutarse de verdad. La allowlist pasaría a ser decoración exactamente en el camino
	// donde no hay una persona mirando.
	//
	// Con un servicio nombrado, el argv es fijo y conocido al validar. Diez servicios que
	// vigilar son diez políticas: más verboso, y cada una dice en su nombre qué hace. El día que
	// haga falta lo genérico, el trabajo no es la sustitución — es que la allowlist se evalúe
	// después de ella, y eso es un slice propio.
	Servicio string

	Hacer    []string
	Cooldown time.Duration
}

// EstadoCuentaComoCaido decide si el estado de un servicio cumple `servicio_caido`.
//
// `desconocido` NO CUENTA, y ésta es la regla que sostiene toda la tool de servicios llegando al
// plano de ACTUAR — que es donde de verdad cuesta romperla.
//
// Una máquina que no pudo ENUMERAR sus servicios no está diciendo que el postgres esté caído:
// está diciendo que no sabe. Reiniciar algo por no haber podido mirarlo es exactamente el
// automatismo que nadie quiere, y el caso no es raro — un `systemctl` que falla por permisos, un
// agente recién arrancado, una fuente que abortó el inventario a propósito.
//
// El vacío tampoco cuenta, por lo mismo: es el cero de Go, no una medición.
func EstadoCuentaComoCaido(e EstadoServicio) bool {
	return e == EstadoFallado || e == EstadoDetenido
}

// ClaveDeCooldown es la unidad sobre la que se cuenta el enfriamiento.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LLEVA EL SERVICIO, Y SIN ESO A44 ERA PEOR QUE NO TENER LA POLÍTICA
//
// El cooldown se llevaba por (política × máquina). Para una política de host alcanza: hay un
// solo disco por máquina. Para una de servicio NO: dos políticas distintas sobre `nginx` y sobre
// `postgres` de la misma máquina compartirían enfriamiento, así que reiniciar uno DEJARÍA MUDO
// al otro durante todo el cooldown — y el segundo servicio se quedaría caído sin que nada actúe,
// justo por haber actuado sobre el primero.
//
// El nombre de la política ya distingue las dos políticas entre sí; lo que faltaba era el
// alcance DENTRO de la máquina. Para las de host queda vacío, así que las claves existentes no
// cambian de forma y el estado guardado sigue valiendo.
func (p Politica) ClaveDeCooldown(deviceID string) string {
	return p.Nombre + "\x00" + deviceID + "\x00" + strings.TrimSpace(p.Servicio)
}

// EsDeServicio dice si esta política mira un servicio en vez de una métrica del host.
func (p Politica) EsDeServicio() bool {
	return p.Cuando == CondServicioCaido || p.Cuando == CondServicioReinicios
}

// Validar revisa lo que se puede saber SIN la base ni el registro de principales. Lo que depende
// de ellos (que el principal exista, que tenga alguna concesión de exec) lo valida el servidor al
// arrancar — pero también al arrancar, nunca en caliente (I12).
func (p Politica) Validar() error {
	if strings.TrimSpace(p.Nombre) == "" {
		return fmt.Errorf("una política sin `nombre` no se puede nombrar en un log ni en una métrica")
	}
	if strings.TrimSpace(p.Principal) == "" {
		return fmt.Errorf("política %q: falta `principal`. Una política NO tiene autoridad propia: actúa con la de alguien declarado en principals.yaml", p.Nombre)
	}
	servicio := strings.TrimSpace(p.Servicio)
	switch p.Cuando {
	case CondDiscoPct, CondDiscoLibrePct, CondMemPct, CondCPUPct, CondCargaPorCore, CondTempC:
		// LAS DE HOST PROHÍBEN `servicio`, y no es tiquismiquis: una política de disco con un
		// servicio declarado se lee como «vigilá el disco de ese servicio», que no es lo que hace
		// y no existe. Aceptarlo en silencio dejaría a alguien creyendo que vigila algo distinto
		// de lo que vigila.
		if servicio != "" {
			return fmt.Errorf("política %q: la condición %q mira una métrica de la MÁQUINA, así que `servicio` sobra. "+
				"Si lo que querés es vigilar un servicio, usá servicio_caido o servicio_reinicios", p.Nombre, p.Cuando)
		}
	case CondServicioCaido, CondServicioReinicios:
		if servicio == "" {
			return fmt.Errorf("política %q: la condición %q necesita `servicio`. No se admite «el que se haya caído»: "+
				"el nombre lo reporta la máquina, y sustituirlo dentro del comando haría que un dato no confiable "+
				"termine siendo un argumento — con la allowlist validada antes de saber qué se va a ejecutar", p.Nombre, p.Cuando)
		}
		if !NombreDeServicioValido(servicio) {
			return fmt.Errorf("política %q: `servicio` no es un nombre válido", p.Nombre)
		}
	default:
		return fmt.Errorf("política %q: condición desconocida %q (usá disco_pct, disco_libre_pct, mem_pct, cpu_pct, carga_por_core, temp_c, servicio_caido o servicio_reinicios)", p.Nombre, p.Cuando)
	}
	if len(LimpiarSelectores(p.Sobre)) == 0 {
		return fmt.Errorf("política %q: `sobre` no nombra ninguna máquina (usá [\"*\"] para todas)", p.Nombre)
	}
	if err := ValidarComando(p.Hacer, ComandoTimeoutDefault); err != nil {
		return fmt.Errorf("política %q: `hacer` no es un comando válido: %w", p.Nombre, err)
	}
	// El escalamiento que S6 cerró para exec vale igual acá: el canal de comandos lo comparten la
	// ejecución y la pantalla, y `musubi:*` son mensajes internos del canal, no comandos del host.
	if strings.HasPrefix(strings.TrimSpace(p.Hacer[0]), "musubi:") {
		return fmt.Errorf("política %q: `musubi:*` son operaciones internas del canal, no comandos del host", p.Nombre)
	}
	if p.Cooldown != 0 && p.Cooldown < CooldownMin {
		return fmt.Errorf("política %q: cooldown de %s es demasiado corto (mínimo %s): dispararía en cada tick hasta que la métrica baje, y la métrica no baja hasta que el comando termine", p.Nombre, p.Cooldown, CooldownMin)
	}
	return nil
}

// CooldownEfectivo aplica el default sin mutar la política.
func (p Politica) CooldownEfectivo() time.Duration {
	if p.Cooldown <= 0 {
		return CooldownDefault
	}
	return p.Cooldown
}

// Alcanza dice si esta política aplica a esta máquina. Mismo selector que las concesiones, para
// que no haya dos gramáticas de «qué máquinas» que se puedan desincronizar.
func (p Politica) Alcanza(nombreDevice string) bool {
	for _, s := range LimpiarSelectores(p.Sobre) {
		if s == "*" || s == nombreDevice {
			return true
		}
	}
	return false
}

// LimpiarSelectores normaliza una lista de selectores de máquina.
func LimpiarSelectores(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Dispara evalúa la condición contra una muestra. Devuelve el valor medido y si hay que actuar.
//
// UN DATO AUSENTE NO DISPARA, Y NO ES LO MISMO QUE UN CERO. Es el invariante que gobierna el
// track entero desde S4, y acá es más caro que en un panel: una máquina Windows no tiene load
// average —no es que valga 0.00, es que no existe—, así que una política sobre `carga_por_core`
// tiene que IGNORARLA, no leerle un 0 y decidir que está sana. Y al revés: un `temp_c` ausente en
// una máquina sin sensor no puede leerse como 0 °C, que sería «fresquísima».
//
// Nunca dispara una condición que no se pudo medir; el que no puede medir, no opina.
// DisparaSobreServicio evalúa una condición de servicio. Devuelve el valor medido y si dispara.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA MUESTRA RANCIA TIENE SU GEMELO ACÁ, Y ES LA GUARDA QUE MÁS IMPORTA
//
// `Dispara` no actúa sobre una muestra vieja (I13): si la máquina dejó de reportar, el disco pudo
// haberse vaciado hace veinte minutos y actuar sería reaccionar a algo que ya no pasa.
//
// Lo mismo, exacto, para un servicio: un inventario viejo puede decir `fallado` sobre algo que
// alguien ya levantó a mano. Peor todavía — el inventario se DEJA DE MANDAR cuando una fuente
// falla, a propósito, así que «sin noticias» es un estado que este mismo sistema produce. Actuar
// sobre eso sería reiniciar servicios porque el agente no pudo enumerarlos.
//
// Por eso pide `fresco` explícito y no lo deriva de una fecha acá: la frescura la decide el
// umbral de la máquina, que ya se calcula en un solo lugar.
// EL VALOR VUELVE COMO PUNTERO, y no es un detalle de estilo: nil significa NO MEDIDO.
//
// Con un float64 a secas, «esta plataforma no sabe cuántas veces se reinició» y «se reinició cero
// veces» devuelven lo mismo, y la diferencia deja de existir para todo el que lea el resultado —
// el panel, la métrica, el log del disparo. Es el mismo idioma del «no sé» que usa toda la
// telemetría, aplicado al plano de actuar.
//
// (La primera versión devolvía float64 y una prueba lo dejó pasar: con cualquier umbral válido,
// 0 no dispara, así que tratar el nil como cero era BEHAVIORALMENTE IDÉNTICO. La prueba afirmaba
// guardar una distinción que el código no hacía. Se hizo real en vez de borrar la afirmación.)
func (p Politica) DisparaSobreServicio(sv Servicio, fresco bool) (*float64, bool) {
	if !p.EsDeServicio() {
		return nil, false
	}
	// Un servicio revocado no dispara nada: está dado de baja, aunque su última salud diga que
	// se cayó. Actuar sobre lo que alguien sacó del inventario es actuar sobre lo que decidió
	// que no le importe.
	if sv.Revocado || !fresco {
		return nil, false
	}
	switch p.Cuando {
	case CondServicioCaido:
		// Acá el valor SÍ se midió: sabemos el estado del servicio. 1 = caído, 0 = no.
		v := 0.0
		if EstadoCuentaComoCaido(sv.EstadoActual()) {
			v = 1
		}
		return &v, v == 1
	case CondServicioReinicios:
		// AUSENTE NO ES CERO, tampoco acá. Una plataforma que no sabe decir cuántas veces se
		// reinició algo —el SCM de Windows no lo da— no está diciendo que se reinició cero veces.
		// Tratarlo como 0 haría que la política NUNCA dispare en esas máquinas, en silencio.
		if sv.Salud == nil || sv.Salud.Reinicios == nil {
			return nil, false
		}
		n := float64(*sv.Salud.Reinicios)
		return &n, n > p.Supera
	}
	return nil, false
}

func (p Politica) Dispara(m *Muestra) (float64, bool) {
	if m == nil {
		return 0, false
	}
	var v *float64
	switch p.Cuando {
	case CondDiscoPct:
		v = PctUsado(m.DiscoUsado, m.DiscoTotal)
	case CondDiscoLibrePct:
		// La ÚNICA condición que dispara al BAJAR, y la que un operador quiere para el disco: se
		// mira lo ESCRIBIBLE, no lo «no usado». La reserva de root (~5 %) no es ninguna de las dos
		// cosas, así que un disco «al 92 % usado» puede tener 0 bytes para una aplicación.
		if libre := PctUsado(m.DiscoDisponible, m.DiscoTotal); libre != nil {
			return *libre, *libre < p.Supera
		}
		return 0, false
	case CondMemPct:
		v = PctUsado(m.MemUsada, m.MemTotal)
	case CondCPUPct:
		v = m.CPUPct
	case CondCargaPorCore:
		if m.Load5 != nil && m.NumCPU > 0 {
			c := *m.Load5 / float64(m.NumCPU)
			v = &c
		}
	case CondTempC:
		v = m.TempC
	}
	if v == nil {
		return 0, false
	}
	return *v, *v > p.Supera
}

// Umbral describe el sentido de la comparación, para que un log o una métrica no mientan sobre
// qué se comparó contra qué.
func (p Politica) Umbral() string {
	if p.Cuando == CondDiscoLibrePct {
		return fmt.Sprintf("%s < %.1f", p.Cuando, p.Supera)
	}
	return fmt.Sprintf("%s > %.1f", p.Cuando, p.Supera)
}
