package fleet

// cronologia.go es la LÍNEA DE TIEMPO de UNA máquina: qué le hizo Musubi, cuándo, por qué plano y
// con la autoridad de quién. Es la fundación de la fase 5 (lo cognitivo), y no se puede saltear:
// correlacionar «desde el martes anda lenta» con algo exige, primero, poder listar qué pasó el
// martes — y hasta hoy eso vive partido en tres bitácoras que nadie cruza.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SÓLO SE CONSTRUYE SOBRE TABLAS APPEND-ONLY, Y ESA ES LA DECISIÓN QUE MÁS DUELE
//
// La tentación es armar la cronología con todo lo que tenga fecha: `devices.last_seen`,
// `services.last_report`, `fleet_policy_state.last_fired`. No se hace, y el motivo es que esas
// columnas guardan el ÚLTIMO valor, no la historia. Una política que disparó cuarenta veces el
// martes aparecería como UNA línea, y la línea diría la hora de la última — o sea que la
// cronología mostraría, con toda confianza, un martes tranquilo.
//
// Un renglón que resume cuarenta es peor que un renglón ausente: el ausente se nota.
//
// Así que las fuentes son las tres tablas que sí registran cada ocurrencia: `device_commands`,
// `screen_sessions` y `shell_sessions`. Lo que queda afuera se DECLARA en la respuesta (ver
// HuecosDeLaCronologia) en vez de disimularse — una línea de tiempo que no dice qué no vio es
// exactamente cómo se llega a concluir «no pasó nada» cuando lo que pasó no estaba en la fuente.
//
// LAS POLÍTICAS SÍ ESTÁN, y no por excepción: su acción se encola con EncolarComando igual que la
// de una persona (I16), así que cada disparo real es una fila de `device_commands`. Lo que NO se
// ve es el disparo que no llegó a encolar nada; ése vive sólo en `last_fired`.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA COMPUERTA VIAJA CON EL HECHO, NO CON LA CONSULTA
//
// Las tres bitácoras de origen tienen tres compuertas distintas: `exec` para los comandos,
// `screen:view` para las pantallas, `shell` para las shells. Una tool que las une y compuerta con
// UNA capacidad —la que sea— filtra mal en una de las dos direcciones: con la más laxa le muestra
// a alguien con `exec` quién tuvo un prompt; con la más estricta le esconde sus propios comandos
// a quien puede correrlos.
//
// Por eso CapDeHecho es una función TOTAL sobre el tipo de hecho, y su default es no mostrar. El
// tipo de hecho se decide por lo que el hecho REVELA, no por la tabla de la que salió: una fila
// de `device_commands` cuyo argv es `musubi:pantalla` revela que alguien miró una pantalla, así
// que pide `screen:view` aunque viva en la tabla de comandos.

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ── Las operaciones internas del canal ──────────────────────────────────────────────────────
//
// El canal de comandos lo comparten el exec, la pantalla y la shell; el agente distingue por el
// primer argumento. Estos nombres son PROTOCOLO —los escribe el cerebro y los lee el agente—, y
// viven acá porque la clasificación de la cronología los necesita y porque tenerlos repartidos en
// literales por tres paquetes es cómo uno se queda viejo sin que nadie se entere.
const (
	// PrefijoOperacionInterna marca lo que NO es un comando del host. `musubi_fleet_exec` lo
	// rechaza a propósito: sin esa guarda, alguien con `exec` encolaría a mano una operación de
	// pantalla y se saltearía la compuerta de `screen`.
	PrefijoOperacionInterna = "musubi:"

	OpPantalla  = "musubi:pantalla"  // aplicar la contraseña de una sesión de pantalla
	OpAvisar    = "musubi:avisar"    // avisarle al usuario de la máquina (eje `avisa`)
	OpPreguntar = "musubi:preguntar" // pedirle permiso al usuario de la máquina (eje `pide`)
	OpShell     = "musubi:shell"     // abrir una shell interactiva en Tier A
)

// EsOperacionInterna dice si este argv es un mensaje del canal y no un comando del host.
func EsOperacionInterna(argv []string) bool {
	return len(argv) > 0 && strings.HasPrefix(strings.TrimSpace(argv[0]), PrefijoOperacionInterna)
}

// ArgvDeBitacora devuelve el argv SIN secreto, listo para mostrarse en cualquier superficie.
//
// ES OBLIGATORIO EN TODA SUPERFICIE QUE MUESTRE UN ARGV, y por eso vive en el dominio y no en la
// tool: la contraseña de una sesión de pantalla viaja en el argv —tiene que llegar a la máquina
// de alguna forma— y `device_commands` guarda el argv tal cual. Cada superficie nueva que se
// olvide de llamar a esto entrega contraseñas de sesión a quien pueda leer la bitácora, y la
// garantía G1 («Musubi nunca guarda la contraseña») se cae por la puerta de al lado: no la
// guardaría, la mostraría.
//
// Se escribió DOS VECES en este repo antes de vivir acá —una en la bitácora de exec, otra al
// armar la cronología— y ésa es exactamente la duplicación que envejece mal: la copia que se
// queda vieja es siempre la del camino que se usa menos.
func ArgvDeBitacora(argv []string) []string {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) != OpPantalla {
		return argv
	}
	// Se conserva el id de sesión (sirve para cruzar con la bitácora de pantalla) y se tapa el
	// resto. El largo del resultado no depende del secreto.
	//
	// El marcador va SIN ángulos: `encoding/json` escapa `<` y `>` por default, así que un
	// `<oculto>` sale como `\u003coculto\u003e` y una bitácora leída en crudo se vuelve ilegible
	// justo en la línea que más se mira.
	id := ""
	if len(argv) > 1 {
		id = argv[1]
	}
	return []string{OpPantalla, id, "[oculto]"}
}

// ── La ventana ──────────────────────────────────────────────────────────────────────────────

// Ventana es el intervalo que se mira. Es OBLIGATORIA y no tiene un modo «todo»: una cronología
// sin ventana es una bitácora más, y el tope la cortaría por lo más reciente sin decirlo — que es
// justo la forma en que una lista corta se lee como «no pasó nada».
type Ventana struct {
	Desde time.Time
	Hasta time.Time
}

const (
	// VentanaDefault es lo que se mira si nadie declara nada: un día. Alcanza para «¿qué pasó
	// anoche?», que es la pregunta que se hace apurado.
	VentanaDefault = 24 * time.Hour
	// VentanaMax acota el barrido. No es una regla de negocio: es que más allá de esto la
	// consulta deja de ser barata y la respuesta deja de ser legible.
	VentanaMax = 30 * 24 * time.Hour
)

// VentanaHasta arma la ventana que termina en `hasta` y dura `d`. Centraliza los dos defaults
// para que ninguna superficie invente los suyos.
func VentanaHasta(hasta time.Time, d time.Duration) Ventana {
	if d <= 0 {
		d = VentanaDefault
	}
	if d > VentanaMax {
		d = VentanaMax
	}
	return Ventana{Desde: hasta.Add(-d), Hasta: hasta}
}

// Valida rechaza lo que no se puede consultar. Fail-closed: una ventana mal armada NO se
// convierte en «traeme todo».
func (v Ventana) Valida() error {
	if v.Desde.IsZero() || v.Hasta.IsZero() {
		return fmt.Errorf("la ventana necesita las dos puntas: `desde` y `hasta`")
	}
	if !v.Desde.Before(v.Hasta) {
		return fmt.Errorf("la ventana va al revés: `desde` (%s) no es anterior a `hasta` (%s)",
			v.Desde.UTC().Format(time.RFC3339), v.Hasta.UTC().Format(time.RFC3339))
	}
	if v.Duracion() > VentanaMax {
		return fmt.Errorf("la ventana pedida es de %s y el máximo es %s", v.Duracion(), VentanaMax)
	}
	return nil
}

func (v Ventana) Duracion() time.Duration { return v.Hasta.Sub(v.Desde) }

// Normalizada lleva las dos puntas a la granularidad que el ALMACENAMIENTO tiene: el segundo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SIN ESTO, «LAS ÚLTIMAS 24 HORAS» NO INCLUYE LO QUE PASÓ RECIÉN — Y ÉSE ES EL PEOR CASO
//
// Las fechas se guardan en RFC3339 SIN fracción de segundo, así que un comando encolado a las
// 22:29:58.7 queda escrito como `22:29:58`. Una ventana que termina «ahora» termina en
// 22:29:58.7, que al formatearse para la consulta se convierte en `22:29:58` — y con el borde
// superior ABIERTO, ese comando queda afuera.
//
// El síntoma es el que más engaña de todos: alguien reinicia un servicio, entra a mirar la
// cronología y la ve vacía. Lo encontró el barrido de aislamiento, que exige que el admin
// federado SÍ vea el dato — el control positivo, no la aserción de fuga.
//
// LA PUNTA DE ARRIBA SE REDONDEA HACIA ARRIBA y la de abajo hacia abajo: las dos hacia AFUERA. En
// una ventana con bordes sub-segundo eso la agranda hasta un segundo por punta, que es el error
// mínimo posible cuando el dato guardado no tiene más resolución. Redondear hacia adentro sería
// perder hechos, y perderlos justo en el borde que alguien eligió mirar.
//
// UNA PUNTA SIN FRACCIÓN NO SE MUEVE, y eso conserva el mosaico: dos ventanas consecutivas
// declaradas en segundos enteros —«00 a 12» y «12 a 24»— siguen sin solaparse ni contar dos veces
// el hecho de las 12. A resolución sub-segundo el mosaico ya no es posible, y disimularlo sería
// prometer una precisión que la tabla no tiene.
func (v Ventana) Normalizada() Ventana {
	desde := v.Desde.Truncate(time.Second)
	hasta := v.Hasta.Truncate(time.Second)
	if hasta.Before(v.Hasta) {
		hasta = hasta.Add(time.Second)
	}
	return Ventana{Desde: desde, Hasta: hasta}
}

// Contiene es SEMIABIERTA [Desde, Hasta). Con las dos puntas cerradas, dos ventanas consecutivas
// —«las 00 a las 12» y «las 12 a las 24»— contarían dos veces el hecho de las 12 en punto, y
// sumar los dos tramos daría un total que no existe.
func (v Ventana) Contiene(t time.Time) bool {
	return !t.Before(v.Desde) && t.Before(v.Hasta)
}

// ── Los hechos ──────────────────────────────────────────────────────────────────────────────

// PlanoDeFlota es a QUÉ plano de capacidad pertenece un hecho. Se muestra porque cambia cómo se
// lee todo lo demás: que alguien haya ENTRADO a una máquina y que algo se haya EJECUTADO en ella
// son dos riesgos distintos y dos conversaciones distintas.
type PlanoDeFlota string

const (
	PlanoActuar PlanoDeFlota = "actuar" // se ejecutó algo en la máquina
	PlanoEntrar PlanoDeFlota = "entrar" // alguien estuvo adentro (pantalla o shell)
)

// TipoDeHecho es QUÉ pasó. Es un enum cerrado a propósito: cada tipo tiene una capacidad asociada
// y agregar uno sin asociarle ninguna lo deja invisible, no público.
type TipoDeHecho string

const (
	HechoComando  TipoDeHecho = "comando" // un exec: de una persona o de una política
	HechoPantalla TipoDeHecho = "sesion_pantalla"
	HechoShell    TipoDeHecho = "sesion_shell"
	// HechoCanalPantalla y HechoCanalShell son las filas de `device_commands` que en realidad son
	// plomería del plano de entrar. Se muestran como lo que revelan —que hubo una sesión— y no
	// como «alguien corrió un comando», que es lo que dice la tabla de la que salen.
	HechoCanalPantalla TipoDeHecho = "canal_pantalla"
	HechoCanalShell    TipoDeHecho = "canal_shell"
	// HechoSinClasificar es una operación interna que esta versión no conoce. NO SE MUESTRA A
	// NADIE, y se cuenta aparte: es el default fail-closed de CapDeHecho hecho visible.
	HechoSinClasificar TipoDeHecho = "sin_clasificar"
)

// TiposDeHecho es el enum entero. Existe para que una prueba pueda recorrerlo y exigir que cada
// tipo tenga capacidad y plano — sin eso, un tipo nuevo se agrega y nadie se entera de que quedó
// sin compuerta hasta que alguien lo ve en una respuesta.
var TiposDeHecho = []TipoDeHecho{
	HechoComando, HechoPantalla, HechoShell,
	HechoCanalPantalla, HechoCanalShell, HechoSinClasificar,
}

// CapDeHecho dice QUÉ CAPACIDAD hace falta para ver este hecho. El segundo valor en false
// significa «no se muestra», y es el default de todo lo que no está en la lista.
//
// EL DEFAULT ES NO MOSTRAR, y no es prudencia decorativa: el día que se agregue una operación
// interna nueva —un `musubi:algo` que todavía no existe—, la cronología la va a ocultar en vez de
// clasificarla como comando común. Si el default fuera `exec`, esa operación nueva se le
// mostraría a todo el que pueda ejecutar, revelando el plano al que pertenece antes de que nadie
// haya decidido quién puede verla.
func CapDeHecho(t TipoDeHecho) (Cap, bool) {
	switch t {
	case HechoComando:
		return CapExec, true
	case HechoPantalla, HechoCanalPantalla:
		// Alcanza con poder MIRAR: quien tiene `screen:view` ya puede ver esa pantalla, así que
		// negarle saber quién más la vio no protege nada. Mismo criterio que musubi_fleet_sessions.
		return CapScreenView, true
	case HechoShell, HechoCanalShell:
		return CapShell, true
	}
	return "", false
}

// PlanoDeHecho es la otra mitad de la clasificación. Se separa de CapDeHecho porque son dos
// preguntas distintas —quién puede verlo, y cómo se lee— y colapsarlas obligaría a que cada plano
// tuviera exactamente una capacidad, que hoy no es cierto.
func PlanoDeHecho(t TipoDeHecho) (PlanoDeFlota, bool) {
	switch t {
	case HechoComando:
		return PlanoActuar, true
	case HechoPantalla, HechoShell, HechoCanalPantalla, HechoCanalShell:
		return PlanoEntrar, true
	}
	return "", false
}

// TipoDeArgv clasifica una fila de `device_commands` por lo que REVELA.
func TipoDeArgv(argv []string) TipoDeHecho {
	if !EsOperacionInterna(argv) {
		return HechoComando
	}
	switch strings.TrimSpace(argv[0]) {
	case OpPantalla, OpAvisar, OpPreguntar:
		return HechoCanalPantalla
	case OpShell:
		return HechoCanalShell
	}
	return HechoSinClasificar
}

// Hecho es una línea de la cronología. Es una VISTA, igual que SesionViva: no hay tabla de
// hechos, y no la va a haber mientras las fuentes sigan siendo append-only por su cuenta.
//
// NO TIENE LOS CAMPOS PROPIOS DE CADA FUENTE —ni exit_code, ni stdout, ni último tráfico—, y eso
// es deliberado: la mitad de las filas los tendría en cero y ese cero se leería como un dato. Lo
// que hay acá es lo que las tres fuentes comparten de verdad: cuándo, qué, quién, cómo terminó.
// Para el detalle está la bitácora del plano, y `Referencia` es con qué buscarlo ahí.
type Hecho struct {
	// Cuando es el COMIENZO del hecho, siempre: cuándo se encoló el comando, cuándo se abrió la
	// sesión. La ventana filtra por esto y no por el final — un comando que arrancó adentro de la
	// ventana pertenece a esa ventana aunque termine después, que es como se lee una línea de
	// tiempo.
	Cuando time.Time
	Tipo   TipoDeHecho
	Plano  PlanoDeFlota

	DeviceID string
	Device   string

	// Principal es CON LA AUTORIDAD DE QUIÉN. En un comando disparado por una política es el
	// principal de la política, en la misma columna y con el mismo peso que una persona (I16).
	// No hay columna que diga «esto fue automático»: la distinción se lee del nombre, por
	// convención, y eso está registrado como A59.
	Principal string

	// Referencia es el id en la bitácora de su plano. Es lo que permite pasar de la cronología al
	// detalle sin adivinar.
	Referencia string

	// Estado viaja como texto porque cada fuente tiene su propio enum y no son el mismo conjunto.
	// Traducirlos a uno común inventaría estados que ninguna de las tres tiene.
	Estado string

	// Argv sólo lo llevan los hechos que salen de `device_commands`, YA PASADO por
	// ArgvDeBitacora. Nil en los demás: una sesión no tiene argv, y un slice vacío se leería como
	// «corrió algo sin argumentos».
	Argv []string

	// Termino es cuándo dejó de estar en curso. Cero = no terminó, o no se sabe. No se rellena
	// con `Cuando` para que no parezca instantáneo lo que duró dos horas.
	Termino time.Time
}

// Duracion devuelve cuánto duró, y si eso se sabe. El booleano es el punto: un `0` devuelto a
// secas se dibuja como «duró nada» y lo que pasa es que todavía está corriendo.
func (h Hecho) Duracion() (time.Duration, bool) {
	if h.Termino.IsZero() || h.Termino.Before(h.Cuando) {
		return 0, false
	}
	return h.Termino.Sub(h.Cuando), true
}

// ── Las puertas desde cada fuente ────────────────────────────────────────────────────────────
//
// Son las ÚNICAS tres formas de fabricar un Hecho, por el mismo motivo que DesdeSesionPantalla y
// DesdeSesionShell son las únicas puertas a SesionViva: dos consumidores que traducen por su
// cuenta terminan discrepando en qué es «cuándo» — y acá eso movería hechos de ventana.

func HechoDeComando(c Comando, device string) Hecho {
	tipo := TipoDeArgv(c.Argv)
	plano, _ := PlanoDeHecho(tipo)
	return Hecho{
		Cuando:     c.Creado,
		Tipo:       tipo,
		Plano:      plano,
		DeviceID:   c.DeviceID,
		Device:     device,
		Principal:  c.Principal,
		Referencia: c.ID,
		Estado:     string(c.Estado),
		Argv:       ArgvDeBitacora(c.Argv),
		Termino:    c.Terminado,
	}
}

func HechoDeSesionPantalla(s SesionPantalla, device string) Hecho {
	return Hecho{
		Cuando:     s.Creada,
		Tipo:       HechoPantalla,
		Plano:      PlanoEntrar,
		DeviceID:   s.DeviceID,
		Device:     device,
		Principal:  s.Principal,
		Referencia: s.ID,
		Estado:     string(s.Estado),
		Termino:    s.Cerrada,
	}
}

func HechoDeSesionShell(s SesionShell, device string) Hecho {
	return Hecho{
		Cuando:     s.Creada,
		Tipo:       HechoShell,
		Plano:      PlanoEntrar,
		DeviceID:   s.DeviceID,
		Device:     device,
		Principal:  s.Principal,
		Referencia: s.ID,
		Estado:     string(s.Estado),
		Termino:    s.Cerrada,
	}
}

// OrdenarHechos deja la cronología del más nuevo al más viejo, con desempate estable.
//
// EL DESEMPATE NO ES ADORNO: dos hechos del mismo instante —pasa cuando se abre una sesión de
// pantalla, porque la sesión y su comando de canal se escriben juntos— saldrían en orden distinto
// en cada llamada, y una lista que se reordena sola mientras se mira es una lista en la que nadie
// confía. La referencia desempata porque es lo único único que hay.
func OrdenarHechos(hs []Hecho) {
	sort.SliceStable(hs, func(i, j int) bool {
		if !hs[i].Cuando.Equal(hs[j].Cuando) {
			return hs[i].Cuando.After(hs[j].Cuando)
		}
		return hs[i].Referencia < hs[j].Referencia
	})
}

// ── Lo que la cronología NO vio ─────────────────────────────────────────────────────────────

// HuecosDeLaCronologia enumera qué NO contiene esta línea de tiempo, y viaja EN LA RESPUESTA.
//
// Es el mismo criterio que el respaldo del relay: un registro que no aclara contra qué NO protege
// es peor que ninguno, porque alguien deja de buscar el de verdad. Acá el riesgo es exacto — una
// cronología vacía se lee como «no pasó nada en esa máquina», cuando lo que quiere decir es «no
// pasó nada DE LO QUE YO MIRO».
//
// Es una lista fija y no un cálculo: son límites de diseño, no del dato. Cambian cuando cambia el
// diseño, y entonces se edita acá — en un solo lugar, que es lo que evita que una superficie diga
// una cosa y otra diga otra.
func HuecosDeLaCronologia() []string {
	return []string{
		"No hay serie temporal: Musubi guarda el PRESENTE de cada máquina y la historia la guarda Prometheus (B5). Esta cronología dice qué se HIZO, no cómo estuvo.",
		"No hay logs del host: nada de journalctl ni del visor de eventos. Lo que se ve es lo que pasó por Musubi.",
		"No hay historial de salud de servicios: `services` guarda el último reporte, no sus transiciones. Un servicio que se cayó y volvió tres veces no deja tres marcas.",
		"No hay historial de disparos de política: `fleet_policy_state` guarda el ÚLTIMO. Los disparos SÍ aparecen —cada uno encola un comando (I16)—, pero un disparo que no llegó a encolar nada no deja rastro acá.",
		"No hay contenido: ni lo tecleado en una shell, ni lo visto en una pantalla, ni la salida de un comando. Eso es grabación, y es una decisión que nadie tomó (A14, B10).",
	}
}
