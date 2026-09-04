package main

// avisador.go es lo que el agente usa para hablarle a la PERSONA que está en la máquina (A57).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// SE MIDE, NO SE SUPONE, Y SE MIDEN LAS DOS MITADES
//
// «Saber avisar» es que haya DÓNDE dibujar y CON QUÉ. Una sola no alcanza: un Linux con `DISPLAY`
// puesto y sin `notify-send` ni `zenity` no puede avisarle a nadie, y un servidor con `zenity`
// instalado y sin sesión gráfica tampoco. Afirmar la capacidad con media comprobación haría que
// un `pide` prometa un permiso que nunca se va a pedir — la promesa vacía que este eje evita.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EL MOTIVO VIAJA AUNQUE SEA UN `false`
//
// El desenlace natural de este eje sin agente es que `pide` se endurezca a `prohibido` en toda la
// flota. Si eso pasa y lo único que hay es un cero, nadie sabe si es porque no hay escritorio,
// porque falta un paquete, o porque el agente corre como servicio de sistema — y los tres se
// arreglan distinto. El motivo es lo que convierte «no puedo» en algo accionable.

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// capacidadDeAvisar se resuelve UNA VEZ por proceso y se cachea.
//
// No cambia mientras el agente vive: si alguien instala zenity o inicia sesión gráfica, el
// reinicio del agente lo recoge. Medirlo en cada latido serían dos `LookPath` cada 30 segundos
// para un valor que no se mueve, y —peor— haría que la capacidad reportada parpadee.
var capacidadCacheada *fleet.CapacidadDeAvisar

// medirCapacidadDeAvisar la calcula (o devuelve la cacheada).
func medirCapacidadDeAvisar() fleet.CapacidadDeAvisar {
	if capacidadCacheada == nil {
		c := detectarAvisador()
		capacidadCacheada = &c
	}
	return *capacidadCacheada
}

// hayEscritorio dice si este proceso tiene una sesión gráfica a la que dibujarle.
//
// LAS VARIABLES SON EL ÚNICO INDICIO HONESTO en Linux. Un agente que corre como servicio de
// sistema NO las tiene —systemd no exporta DISPLAY ni WAYLAND_DISPLAY, del mismo modo que no
// exporta XDG_RUNTIME_DIR— y eso es exactamente lo que hay que detectar: ese agente no puede
// dibujar nada aunque la máquina tenga escritorio, porque no está en la sesión de nadie.
func hayEscritorio() (bool, string) {
	if d := strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")); d != "" {
		return true, "wayland"
	}
	if d := strings.TrimSpace(os.Getenv("DISPLAY")); d != "" {
		return true, "x11"
	}
	return false, ""
}

// avisar muestra un mensaje SIN esperar respuesta. Es la entrega de `avisa`.
//
// NO BLOQUEA MÁS DE UNOS SEGUNDOS: el agente atiende los comandos EN SERIE, así que un avisador
// colgado deja a esa máquina sin atender nada más — ni exec, ni pantalla, ni shell. El cerebro la
// vería latiendo y muda, que es el peor de los estados porque parece sano.
func avisar(texto string) error {
	c := medirCapacidadDeAvisar()
	if !c.Puede {
		return errNoSePuedeAvisar{motivo: c.Motivo}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return correrAvisador(ctx, c, recortarAviso(texto))
}

// preguntar muestra una pregunta y espera la respuesta hasta AvisoTimeout. Es la entrega de `pide`.
//
// EL VENCIMIENTO NIEGA, y no concede. Decidido por gio: quien escribió `pide` pidió que nadie
// entre sin permiso, y el silencio no es permiso. Pero se devuelve `sin_respuesta` y no `negada`,
// porque el comportamiento es el mismo y el diagnóstico no: si el motivo es siempre «nadie
// contestó», esa máquina no debería estar en `pide`.
func preguntar(texto string) fleet.RespuestaAviso {
	c := medirCapacidadDeAvisar()
	if !c.Puede {
		return fleet.RespuestaNoSePudo
	}
	ctx, cancel := context.WithTimeout(context.Background(), fleet.AvisoTimeout)
	defer cancel()
	r := correrPregunta(ctx, c, recortarAviso(texto))
	// El ctx vencido gana sobre lo que haya devuelto la herramienta: un zenity matado por el
	// timeout sale con código distinto de cero, que sin esto se leería como «el usuario dijo que
	// no». Son dos cosas distintas y la bitácora tiene que poder separarlas.
	if ctx.Err() != nil {
		return fleet.RespuestaSinRespuesta
	}
	return r
}

// recortarAviso acota lo que se dibuja. El texto lo arma el cerebro y termina en la pantalla de
// otra persona: diez mil caracteres no son un aviso, son una ventana que alguien cierra sin leer.
func recortarAviso(s string) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	if len(rs) <= fleet.AvisoTextoMax {
		return s
	}
	return string(rs[:fleet.AvisoTextoMax-1]) + "…"
}

// errNoSePuedeAvisar lleva el motivo, para que el resultado que vuelve al cerebro diga qué
// arreglar en vez de sólo que falló.
type errNoSePuedeAvisar struct{ motivo string }

func (e errNoSePuedeAvisar) Error() string {
	if e.motivo == "" {
		return "esta máquina no puede avisarle a nadie"
	}
	return "esta máquina no puede avisarle a nadie: " + e.motivo
}

// primeroDisponible devuelve el primer ejecutable de la lista que esté en el PATH.
func primeroDisponible(candidatos ...string) string {
	for _, c := range candidatos {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

// ── La operación interna del canal ───────────────────────────────────────────────────────────

// comandoAvisarAgente es la operación que el cerebro encola para hablarle al usuario de una
// máquina. Como `musubi:pantalla`, NO es un ejecutable del host y nunca debe llegar a
// exec.Command: si llegara, el error diría «no such file» y el mensaje arrastraría el texto.
// SALE DEL DOMINIO, no de un literal repetido acá. El cerebro deriva el nombre de fleet.OpAvisar
// (methods_shell.go ya lo hace con OpShell) y el agente lo tenía escrito a mano: si el dominio lo
// renombrara, el agente no seguiría y la operación interna caería en el camino del exec —
// exactamente lo que el comentario de ejecutor.go prohíbe, porque no es un ejecutable del host.
const comandoAvisarAgente = fleet.OpAvisar

// atenderAviso ejecuta `musubi:avisar <texto>`.
//
// ES FIRE-AND-FORGET, y por eso es la entrega de `avisa` y no de `pide`: no espera respuesta y
// no puede bloquear. `pide` necesita el camino asincrónico —el cerebro pregunta, el agente
// contesta en un latido posterior— y es otro slice; mezclarlos acá haría que el agente se quede
// sesenta segundos sin atender NADA MÁS, porque los comandos se atienden en serie.
//
// UN AVISO QUE NO SE PUDO DAR SE REPORTA COMO FALLA, no como éxito silencioso. El cerebro ya
// tiene el estado «se abrió la pantalla y el aviso no se pudo entregar»; lo que no puede es
// creer que se entregó cuando no.
func atenderAviso(comandoID string, argv []string) resultadoDeComando {
	res := resultadoDeComando{ComandoID: comandoID}
	if len(argv) < 2 {
		res.Error = "musubi:avisar sin texto"
		return res
	}
	texto := strings.TrimSpace(strings.Join(argv[1:], " "))
	if texto == "" {
		res.Error = "musubi:avisar con texto vacío"
		return res
	}
	if err := avisar(texto); err != nil {
		res.Error = err.Error()
		return res
	}
	cero := 0
	res.ExitCode = &cero
	// LA SALIDA NO REPITE EL TEXTO DEL AVISO. Ese texto lo armó el cerebro y ya lo tiene; volver
	// a mandarlo lo deja duplicado en la bitácora de comandos, que es de lectura más amplia que
	// la de sesiones de pantalla.
	res.Stdout = "aviso entregado"
	return res
}

// comandoPreguntarAgente es la operación que PIDE PERMISO. A diferencia de avisar, espera.
const comandoPreguntarAgente = fleet.OpPreguntar // del dominio, ver comandoAvisarAgente

// atenderPregunta ejecuta `musubi:preguntar <sesionID> <texto>` y contesta.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// BLOQUEA HASTA UN MINUTO, Y ESO TIENE UN COSTO QUE HAY QUE DECIR
//
// El agente atiende los comandos EN SERIE, así que mientras esta ventana está abierta esa máquina
// no atiende nada más: ni exec, ni otra pantalla, ni shell. Es aceptable porque un `pide` es un
// evento raro y humano —alguien está por entrar a tu escritorio— y porque el plazo está acotado
// por el dominio. Lo que NO sería aceptable es que no tuviera plazo: sin él, una ventana que nadie
// cierra deja la máquina muda para siempre y el cerebro la ve latiendo, que parece sano.
//
// LA RESPUESTA VIAJA EN stdout Y NO EN EL CÓDIGO DE SALIDA. Un exit distinto de cero se lee como
// «el comando falló», y «el usuario dijo que no» NO es una falla: es el sistema haciendo lo que
// se le pidió. Mezclarlos haría que la bitácora registre un error donde hubo una decisión.
func atenderPregunta(comandoID string, argv []string) resultadoDeComando {
	res := resultadoDeComando{ComandoID: comandoID}
	if len(argv) < 3 {
		res.Error = "musubi:preguntar necesita <sesion> <texto>"
		return res
	}
	texto := strings.TrimSpace(strings.Join(argv[2:], " "))
	if texto == "" {
		res.Error = "musubi:preguntar con texto vacío"
		return res
	}
	r := preguntar(texto)
	cero := 0
	res.ExitCode = &cero
	// El formato lo lee el cerebro: prefijo fijo más la respuesta. Se prefiere un prefijo a un
	// stdout pelado para que una salida inesperada —un binario que imprime algo raro— no se
	// interprete como una respuesta.
	res.Stdout = prefijoRespuestaPermiso + string(r)
	return res
}

// prefijoRespuestaPermiso marca la salida como una respuesta de permiso y no como texto suelto.
// Del CONTRATO: estaba declarado también en el cerebro, con el mismo valor y sin nada que los
// atara. Ver internal/fleet/protocolo.go para el modo de falla, que es silencioso y asimétrico.
const prefijoRespuestaPermiso = fleet.PrefijoRespuestaPermiso
