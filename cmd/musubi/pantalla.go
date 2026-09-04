package main

// pantalla.go es la mitad del plano visual que corre EN LA MÁQUINA (S6).
//
// Musubi no transporta video: el que lo hace es RustDesk, y de eso no se escribe una línea acá.
// Lo único que hace este archivo es aplicar la contraseña de sesión que acuñó el cerebro, y
// —esto es lo importante— PROGRAMAR SU PROPIO VENCIMIENTO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL VENCIMIENTO VIVE ACÁ Y NO EN EL CEREBRO
//
// La alternativa obvia es que el cerebro encole un segundo comando cuando la sesión vence. Es
// peor, y por una razón concreta: si el cerebro se cae, si la red se corta, si alguien apaga
// Musubi — la contraseña queda puesta para siempre. Una caducidad que depende de que otra máquina
// siga viva no es una caducidad, es una promesa.
//
// Con el temporizador local, lo peor que puede pasar es que el AGENTE muera; y si el agente
// muere, se lleva el temporizador y la contraseña queda... puesta. Por eso el reemplazo también
// se intenta al arrancar (ver `cerrarSesionesColgadas`): un agente que vuelve no hereda una
// sesión abierta de su encarnación anterior.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"musubi/internal/fleet"
)

// sesionAbierta recuerda si hay una contraseña puesta y QUÉ HABÍA ANTES, para poder devolverlo
// al vencer.
//
// Sigue siendo estado de proceso —el que sobrevive a la muerte del agente es la marca en disco de
// pantalla_respaldo.go—, pero ya no es un booleano suelto: cerrar bien exige saber qué se pisó, y
// un `hay` sin el `previas` al lado es justo la información que faltaba para no destruir la
// contraseña del dueño de la máquina.
var sesionAbierta struct {
	sync.Mutex
	hay     bool
	previas []passPrevia
}

// aplicarSesionPantalla ejecuta la operación interna `musubi:pantalla <sesion> <pass> <ttl>`.
//
// Devuelve un resultado con la MISMA forma que un comando normal, para que el agente lo reporte
// por el camino que ya existe. Lo que NUNCA hace es incluir la contraseña en el resultado: la
// bitácora no puede aprenderla por la puerta de atrás.
func aplicarSesionPantalla(c comandoRecibido) resultadoDeComando {
	res := resultadoDeComando{ComandoID: c.ID}
	if len(c.Argv) < 4 {
		res.Error = "operación de pantalla mal formada"
		return res
	}
	sesion, pass, ttlTexto := c.Argv[1], c.Argv[2], c.Argv[3]
	ttl, err := time.ParseDuration(ttlTexto)
	if err != nil || ttl <= 0 || ttl > fleet.SesionDuracionMax {
		ttl = fleet.SesionDuracionDefault
	}

	// LA FOTO DE ANTES SE SACA ANTES DE TOCAR NADA. RustDesk tiene una sola ranura de contraseña
	// permanente y la sesión la va a pisar; lo único que hace reversible ese pisón es haber
	// copiado el valor viejo un instante antes. Ver pantalla_respaldo.go.
	antes := fotoDeLasConfigs()

	if err := ponerPassRustdesk(pass); err != nil {
		// El error NO puede llevar la contraseña: va derecho a la bitácora.
		res.Error = "no se pudo aplicar la contraseña de pantalla: " + sinSecreto(err.Error(), pass)
		return res
	}

	// QUÉ SE PISÓ, MEDIDO Y NO SUPUESTO: cuál de los RustDesk.toml candidatos cambió es el único
	// dato que dice a dónde hay que devolver el valor viejo, y depende de con qué cuenta corre el
	// agente. Se compara la foto contra el estado de ahora.
	previas := loQueCambio(antes)
	marcarSesionAbiertaCon(previas)
	if err := guardarRespaldo(respaldoPantalla{
		Sesion: sesion, Vence: time.Now().Add(ttl), Previas: previas,
	}); err != nil {
		// NO se aborta la sesión por esto. El temporizador de este proceso ya tiene las previas en
		// memoria, así que el cierre normal va a restituir igual; lo que se pierde es la red para
		// el caso en que el agente muera antes de vencer. Se dice, porque es la diferencia entre
		// «se restituye siempre» y «se restituye salvo que además se caiga».
		fmt.Fprintf(os.Stderr, "%s la sesión de pantalla no dejó marca en disco: si el agente muere antes de vencer, la contraseña anterior no vuelve sola (%v)\n", cYellow("!"), err)
	}

	// El temporizador propio. Reemplaza la contraseña por una al azar QUE NADIE CONOCE — no la
	// borra: dejar a RustDesk sin contraseña sería abrir la máquina, no cerrarla.
	time.AfterFunc(ttl, func() { cerrarSesionPantalla("venció la ventana") })

	cero := 0
	res.ExitCode = &cero
	res.Stdout = fmt.Sprintf("sesión %s aplicada, vence en %s", sesion, ttl)
	return res
}

// ponerPassRustdesk aplica una contraseña permanente en el cliente RustDesk local.
//
// `rustdesk --password <x>` es la interfaz soportada del cliente. Es la ÚNICA línea de todo el
// track que depende de un binario externo, y por eso está aislada acá: todo lo demás del slice
// —la acuñación, el vencimiento, la bitácora, la compuerta— se prueba sin RustDesk instalado.
func ponerPassRustdesk(pass string) error {
	bin, err := rutaRustdesk()
	if err != nil {
		// Este error se ve en la bitácora del comando y en la respuesta de musubi_fleet_screen.
		// Que diga DÓNDE se buscó es la diferencia entre «arreglalo en un minuto» y «probá cosas».
		return err
	}
	cmd := exec.Command(bin, "--password", pass)
	salida, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(salida)))
	}
	return nil
}

// cerrarSesionPantalla cierra la puerta DEVOLVIENDO lo que había, y sólo scramblea cuando no hay
// nada que devolver.
//
// EL ORDEN DE PREFERENCIA ES TODO EL ARREGLO. Antes esta función tenía un solo camino —poner una
// contraseña al azar que nadie conoce— y ese camino, aplicado sobre la ranura PERMANENTE de
// RustDesk, no cerraba una sesión: destruía la contraseña del dueño de la máquina. Ahora:
//
//  1. Si se guardó lo que había, se restituye TAL CUAL en el archivo del que se sacó. La sesión
//     queda cerrada igual —la contraseña de sesión deja de estar puesta— y el dueño recupera la
//     suya sin haberla tenido que decir nunca.
//  2. Si NO había contraseña previa en ningún lado, recién ahí se scramblea. Dejar a RustDesk sin
//     contraseña sería abrir la máquina, no cerrarla, así que el valor al azar sigue siendo la
//     respuesta correcta para ese caso —y sólo para ese—.
func cerrarSesionPantalla(motivo string) {
	sesionAbierta.Lock()
	hay := sesionAbierta.hay
	previas := sesionAbierta.previas
	sesionAbierta.Unlock()

	// LA MARCA EN DISCO ES LA QUE SOBREVIVE A LA MUERTE DEL AGENTE. Si este proceso no sabe de
	// ninguna sesión, todavía puede haberla dejado abierta su encarnación anterior, y ahí está
	// escrito qué restituir. Sin esto, el cierre de arranque no tenía cómo saber nada y por eso
	// se lo llamaba a ciegas —que es exactamente lo que rompía la contraseña en cada reinicio—.
	if !hay {
		if r, ok := leerRespaldo(); ok {
			hay, previas = true, r.Previas
		}
	}
	if !hay {
		return
	}

	restituidos := 0
	for _, p := range previas {
		if !p.Habia {
			continue
		}
		if err := escribirCampoPassword(p.Ruta, p.Linea); err != nil {
			fmt.Fprintf(os.Stderr, "%s no se pudo devolver la contraseña anterior de RustDesk: %v\n", cYellow("!"), err)
			continue
		}
		restituidos++
	}

	if restituidos == 0 {
		// No había nada que devolver —o no se pudo—, así que la puerta se cierra con un valor que
		// nadie conoce. Es el comportamiento de siempre, ahora acotado a su caso.
		nueva, err := fleet.NuevaPassPantalla()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s no se pudo cerrar la sesión de pantalla: %v\n", cYellow("!"), err)
			return
		}
		if err := ponerPassRustdesk(nueva); err != nil {
			fmt.Fprintf(os.Stderr, "%s no se pudo cerrar la sesión de pantalla: %v\n", cYellow("!"), err)
			return
		}
	}

	marcarSesionAbierta(false)
	borrarRespaldo()
	if restituidos > 0 {
		fmt.Printf("%s sesión de pantalla cerrada (%s) · se devolvió la contraseña anterior\n", cDim("■"), motivo)
		return
	}
	// EL CASO QUE QUEDA TAMBIÉN SE DICE, y no es un detalle: acá la máquina se queda con una
	// contraseña que NADIE conoce. Es el comportamiento correcto —dejarla vacía abriría la
	// máquina— pero si se anuncia igual que el caso en que sí se devolvió, desde la silla del
	// dueño sigue siendo «se me cambió sola», nada más que en menos ocasiones. Un fantasma más
	// chico sigue siendo un fantasma, y ésta es exactamente la mitad que el arreglo de arriba no
	// puede cubrir: cuando no había nada que devolver, no hay nada que devolver.
	fmt.Printf("%s sesión de pantalla cerrada (%s)\n", cDim("■"), motivo)
	fmt.Printf("%s no había una contraseña anterior que devolver, así que RustDesk queda con una "+
		"al azar que nadie conoce. Si querés una tuya, ponela en RustDesk → Configuración → "+
		"Seguridad.\n", cYellow("!"))
}

// cerrarSesionColgadaDeArranque es el cierre que corre al levantar el agente, y es una función
// PROPIA en vez de una llamada suelta por una razón que ya se pagó una vez.
//
// El arranque no puede decidir que «hay» una sesión: no sabe. Lo único que lo sabe es la marca en
// disco que dejó la encarnación anterior. Antes, agent.go forzaba `marcarSesionAbierta(true)`
// para que el cierre corriera igual, y como cerrar significaba scramblear la contraseña
// PERMANENTE de RustDesk, cada reinicio del agente destruía la del dueño de la máquina.
//
// Con la condición acá adentro, volver a meter ese defecto exige romper esta función a propósito
// —y la prueba que la cubre—, en vez de alcanzar con agregar una línea en el arranque. Es también
// la función que el comentario de cabecera prometía desde el principio con el nombre
// `cerrarSesionesColgadas` y que nunca había existido.
func cerrarSesionColgadaDeArranque() {
	if _, hay := leerRespaldo(); !hay {
		return
	}
	cerrarSesionPantalla("arranque del agente")
}

func marcarSesionAbierta(v bool) {
	sesionAbierta.Lock()
	sesionAbierta.hay = v
	if !v {
		sesionAbierta.previas = nil
	}
	sesionAbierta.Unlock()
}

// marcarSesionAbiertaCon abre la sesión recordando qué hay que devolver al cerrarla.
func marcarSesionAbiertaCon(previas []passPrevia) {
	sesionAbierta.Lock()
	sesionAbierta.hay = true
	sesionAbierta.previas = previas
	sesionAbierta.Unlock()
}

// sinSecreto tapa la contraseña si el mensaje de error la trajo de vuelta. Un binario externo
// puede echar sus argumentos en el mensaje, y ese mensaje va derecho a la bitácora.
func sinSecreto(msg, pass string) string {
	if pass == "" {
		return msg
	}
	return strings.ReplaceAll(msg, pass, "[oculto]")
}

// idRustdeskLocal lee el identificador público del cliente. Vacío si RustDesk no está instalado
// o no responde.
//
// Devolver "" SIGUE SIENDO correcto —el latido no puede fallar porque falte una pieza opcional—
// pero ya no es mudo. Antes, una máquina con RustDesk instalado y una sin él producían el mismo
// silencio, y eso ocultó durante todo un track que el binario simplemente no se encontraba
// (Windows no lo pone en el PATH). El aviso sale UNA vez por motivo: el latido corre cada pocos
// segundos y un aviso por latido es ruido, que es otra forma de silencio.
func idRustdeskLocal() string {
	bin, err := rutaRustdesk()
	if err != nil {
		avisarUnaVez("rustdesk-ausente", "el plano de pantalla no está disponible: %v", err)
		return ""
	}
	cmd := exec.Command(bin, "--get-id")
	salida, err := cmd.Output()
	if err != nil {
		// Acá SÍ hay algo roto: el binario está y no contesta. Un permiso, una instalación a
		// medias, un cliente que no arrancó todavía.
		avisarUnaVez("rustdesk-mudo", "RustDesk está en %s pero no devuelve su id: %v", bin, err)
		return ""
	}
	id := strings.TrimSpace(string(salida))
	if len(id) > 32 {
		return ""
	}
	return id
}

// avisarUnaVez imprime un aviso a stderr la PRIMERA vez que se ve cada motivo.
//
// El agente late cada pocos segundos; un aviso por latido llena el journal y deja de leerse, que
// es exactamente el mismo resultado que no avisar. Una vez por motivo y por vida del proceso: si
// el problema se arregla y vuelve, el reinicio del agente lo vuelve a decir.
var avisosDados sync.Map

func avisarUnaVez(motivo, formato string, args ...any) {
	if _, ya := avisosDados.LoadOrStore(motivo, true); ya {
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", cYellow("!"), fmt.Sprintf(formato, args...))
}

// avisarCada es avisarUnaVez para los problemas que DURAN.
//
// «Una vez por vida del proceso» es la política correcta para un hecho que no cambia —no hay
// RustDesk instalado, el inventario se recortó— y es la política equivocada para una falla que
// puede estar pasando ahora: el agente arranca, lo dice, y tres días después ese renglón se fue
// con la rotación del journal. Quien mira el log de hoy ve una máquina en silencio y concluye que
// está sana.
//
// Se separó de avisarUnaVez en vez de agregarle un parámetro porque son dos decisiones distintas
// y conviene que se lean distinto en el punto de uso: una dice «esto es así», la otra «esto sigue
// roto».
var avisosConReloj sync.Map

func avisarCada(motivo string, cada time.Duration, formato string, args ...any) {
	ahora := time.Now()
	if previo, hab := avisosConReloj.Load(motivo); hab {
		if ahora.Sub(previo.(time.Time)) < cada {
			return
		}
	}
	avisosConReloj.Store(motivo, ahora)
	fmt.Fprintf(os.Stderr, "%s %s\n", cYellow("!"), fmt.Sprintf(formato, args...))
}
