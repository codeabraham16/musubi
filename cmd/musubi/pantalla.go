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
// se intenta al arrancar — pero SÓLO SI HUBO UNA SESIÓN, y eso hay que saberlo de un arranque al
// siguiente. Ver `hayMarcaDeSesion`.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL ARRANQUE PISABA LA CONTRASEÑA DEL DUEÑO EN CADA REINICIO
//
// La versión anterior forzaba `sesionAbierta.hay = true` antes de cerrar, «por las dudas». Como
// `sesionAbierta` es estado de PROCESO, un agente que arranca no puede saber si su encarnación
// anterior dejó algo puesto — y suponer que sí significa que TODO arranque acuñaba una contraseña
// al azar y se la ponía a RustDesk, hubiera habido sesión o no.
//
// RustDesk tiene UNA sola ranura de contraseña permanente, así que eso destruía la contraseña que
// el dueño de la máquina había elegido. En cada reinicio del agente. Desde la silla de esa
// persona es indistinguible de «RustDesk me cambia la contraseña solo», y en `davantis-1` —que
// lleva quince cortes de energía en diez días— pasó una vez por corte.
//
// El comentario que había acá mandaba a ver `cerrarSesionesColgadas`, una función que NO EXISTE:
// era la única aparición de ese nombre en el repo. Un doc que nombra código inexistente hace que
// nadie vaya a mirar lo que sí hay.
//
// Ahora la marca vive EN DISCO, al lado del binario: si está, hubo sesión y se cierra; si no
// está, no se toca nada.
//
// LO QUE ESTO **NO** ARREGLA, y queda declarado: cuando SÍ hubo sesión, al cerrarla la contraseña
// del dueño tampoco se restituye —se reemplaza por una al azar—. El arreglo de verdad es guardar
// el campo `password` del RustDesk.toml antes de aplicar y devolverlo al vencer, y eso exige medir
// qué guarda ese archivo (en las versiones nuevas el valor va ofuscado, así que `--password`, que
// toma texto plano, no alcanzaría para devolverlo). Ver A84.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"musubi/internal/fleet"
)

// sesionAbierta recuerda si hay una contraseña puesta, para poder cerrarla al vencer. Es estado
// de PROCESO; lo que sobrevive a un reinicio es la marca en disco de acá abajo.
var sesionAbierta struct {
	sync.Mutex
	hay bool
}

// nombreMarcaDeSesion es el archivo que dice «esta máquina tiene una contraseña de sesión puesta».
//
// VA AL LADO DEL BINARIO Y NO EN %LOCALAPPDATA%, por la misma razón que `cambiar-agente.cmd`
// (A71): con `-AlArranque` el agente corre como SYSTEM, y ahí `%LOCALAPPDATA%` es el perfil del
// sistema y no la instalación. Un agente que escribe la marca como usuario y la busca como SYSTEM
// no la encuentra, y no encontrarla significa «no había sesión» — falla en silencio y hacia el
// lado que deja la contraseña puesta.
//
// NO GUARDA NINGÚN SECRETO: sólo dice que hay algo que cerrar. La contraseña de sesión no se
// escribe en ningún lado, ni acá ni en la bitácora.
const nombreMarcaDeSesion = ".musubi-pantalla-abierta"

func rutaMarcaDeSesion() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), nombreMarcaDeSesion), nil
}

// hayMarcaDeSesion dice si la encarnación anterior dejó una sesión puesta.
//
// UN ERROR AL MIRAR NO ES «NO HABÍA». Si no se puede resolver la ruta o leer el directorio, se
// contesta que SÍ: el sesgo del error tiene que ser cerrar de más —cuesta una contraseña de
// sesión— y no de menos, que cuesta dejar abierta una máquina.
func hayMarcaDeSesion() bool {
	ruta, err := rutaMarcaDeSesion()
	if err != nil {
		return true
	}
	_, err = os.Stat(ruta)
	return marcaSegunStat(err)
}

// marcaSegunStat traduce lo que contestó `os.Stat` a la única pregunta que importa.
//
// ESTÁ SEPARADA PARA QUE SE PUEDA PROBAR. La ruta de la marca sale de `os.Executable()`, que en
// una prueba apunta al binario de pruebas y no se puede mover, así que el caso «Stat falló por
// algo que NO es "no existe"» —permiso denegado, un padre que no es directorio— no se puede
// fabricar desde afuera. Con la decisión adentro de hayMarcaDeSesion, esa rama era INALCANZABLE
// para cualquier prueba: se podía cambiar por `return false` y todo seguía en verde.
//
// Lo descubrió el sabotaje, no la revisión: la primera versión de la prueba ponía un DIRECTORIO
// en la ruta, y sobre un directorio `Stat` contesta sin error — así que ejercía la rama de
// arriba y creía estar cubriendo ésta.
func marcaSegunStat(err error) bool {
	if err == nil {
		return true // el archivo está: hubo sesión
	}
	if os.IsNotExist(err) {
		return false // no está, y eso es una respuesta: no hubo sesión
	}
	// CUALQUIER OTRA COSA ES «NO SÉ», Y «NO SÉ» SE RESUELVE CERRANDO. El sesgo del error tiene
	// que costar una contraseña de sesión, no una máquina abierta.
	return true
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

	if err := ponerPassRustdesk(pass); err != nil {
		// El error NO puede llevar la contraseña: va derecho a la bitácora.
		res.Error = "no se pudo aplicar la contraseña de pantalla: " + sinSecreto(err.Error(), pass)
		return res
	}
	marcarSesionAbierta(true)

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

// cerrarSesionPantalla reemplaza la contraseña por una al azar que nadie conoce.
func cerrarSesionPantalla(motivo string) {
	sesionAbierta.Lock()
	hay := sesionAbierta.hay
	sesionAbierta.Unlock()
	if !hay {
		return
	}
	nueva, err := fleet.NuevaPassPantalla()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s no se pudo cerrar la sesión de pantalla: %v\n", cYellow("!"), err)
		return
	}
	if err := ponerPassRustdesk(nueva); err != nil {
		fmt.Fprintf(os.Stderr, "%s no se pudo cerrar la sesión de pantalla: %v\n", cYellow("!"), err)
		return
	}
	marcarSesionAbierta(false)
	// SE DICE QUE LA CONTRASEÑA PERMANENTE QUEDÓ REEMPLAZADA, porque si no, esto es un fantasma.
	// RustDesk tiene una sola ranura y Musubi la usó; al cerrar se pone una al azar que nadie
	// conoce —dejarla vacía abriría la máquina—. Desde la silla del dueño, sin este aviso, es
	// indistinguible de «RustDesk me cambia la contraseña solo», y así se reportó dos veces.
	fmt.Printf("%s sesión de pantalla cerrada (%s)\n", cDim("■"), motivo)
	fmt.Printf("%s la contraseña permanente de RustDesk quedó reemplazada por una al azar. Si vos "+
		"habías puesto una, volvé a ponerla en RustDesk → Configuración → Seguridad. Musubi no la "+
		"guarda ni la puede restituir (ver A84).\n", cYellow("!"))
}

// marcarSesionAbierta mueve las DOS marcas —la de proceso y la de disco— en el mismo lugar.
//
// Están juntas a propósito: separarlas deja que una diga que hay sesión y la otra que no, y la
// que manda al arrancar es justamente la que nadie mira mientras el proceso vive.
func marcarSesionAbierta(v bool) {
	sesionAbierta.Lock()
	sesionAbierta.hay = v
	sesionAbierta.Unlock()

	ruta, err := rutaMarcaDeSesion()
	if err != nil {
		return
	}
	if v {
		// El contenido es para una persona que encuentre el archivo; el código sólo mira si está.
		_ = os.WriteFile(ruta, []byte("hay una contraseña de sesión de pantalla puesta en RustDesk\n"), 0o600)
		return
	}
	_ = os.Remove(ruta)
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
