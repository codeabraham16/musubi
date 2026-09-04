package main

// pantalla_respaldo.go existe por un defecto que se vio en producción antes que en el código: a
// una máquina de la flota «se le cambiaba sola» la contraseña de RustDesk —dos veces— y quien la
// usaba no tenía forma de enterarse de por qué.
//
// LA CAUSA. RustDesk tiene UNA SOLA ranura de contraseña permanente, y la sesión de pantalla la
// usaba de borrador: `musubi_fleet_screen` corre `rustdesk --password <la de la sesión>` —que es
// la PERMANENTE, no una temporal— y al vencer la reemplazaba por una al azar QUE NADIE CONOCE.
// O sea que la contraseña que eligió el dueño de la máquina vivía hasta la próxima sesión de
// pantalla, y después no la sabía nadie: ni él ni Musubi. Desde su silla eso es indistinguible
// de «RustDesk me cambia la contraseña solo», que fue exactamente el reporte.
//
// LO QUE HACE ESTE ARCHIVO: guardar lo que había ANTES y devolverlo al cerrar. No la contraseña
// —Musubi no la sabe, no la puede saber y no la quiere saber— sino el BLOB CIFRADO tal cual está
// en el archivo de RustDesk. Entra opaco y sale opaco, y esa es justo la propiedad que lo hace
// seguro de manejar: restituir no exige entender.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL ARCHIVO SE DETECTA EN VEZ DE SABERSE
//
// Windows tiene DOS RustDesk.toml —el del servicio, bajo el perfil de LocalService, y el del
// usuario— y el que atiende las conexiones entrantes es el del SERVICIO. Cuál de los dos escribe
// `--password` depende de con qué cuenta corre el agente, y eso no se puede afirmar desde acá:
// en la máquina donde se encontró el defecto corre como SYSTEM y escribe el del servicio, y en
// otra instalación puede no ser así. Medido en esa máquina, además, el usuario cambió su
// contraseña por la ventana y quedó SÓLO en la config del usuario, mientras el servicio seguía
// con otra: los dos archivos existen y llevan valores distintos.
//
// Así que acá no se adivina. Se toma una FOTO del campo `password` de todos los candidatos, se
// aplica la contraseña de sesión, y se mira CUÁL CAMBIÓ. Ese —o esos, si cambió más de uno— es
// el archivo a restituir, y su valor viejo es lo que hay que devolverle. Es la misma lección que
// ya dejó escrita rustdesk_ruta.go: la lista de lugares se escribe, y lo que no se sabe se mide.
//
// LO QUE ESTO NO GARANTIZA, Y HAY QUE DECIRLO: devolver el blob al archivo no obliga al SERVICIO
// que ya está corriendo a releerlo. El procedimiento manual con el que se investigó el defecto
// reiniciaba el servicio después de `--password` «para que la tome», así que es probable que el
// valor viejo recién vuelva a estar vivo en el próximo arranque de RustDesk. Aun así la
// diferencia es la que importa: antes la contraseña del dueño se PERDÍA para siempre, y ahora
// vuelve. Reiniciar el servicio de RustDesk por cuenta propia sería una decisión más invasiva
// —corta cualquier sesión en curso y rota la contraseña temporal— y no se toma acá.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// envConfigRustdesk fuerza el archivo de configuración de RustDesk, igual que MUSUBI_RUSTDESK_BIN
// fuerza el binario. Es la salida de emergencia para una instalación que no está en la lista.
const envConfigRustdesk = "MUSUBI_RUSTDESK_CONFIG"

// envRespaldoPantalla fuerza dónde se deja la marca de sesión abierta. Sólo las pruebas y una
// instalación rara lo necesitan.
const envRespaldoPantalla = "MUSUBI_PANTALLA_RESPALDO"

// campoPassword captura la línea del campo `password` TAL CUAL está escrita, con sus comillas y
// su espaciado.
//
// SE TRABAJA CON LA LÍNEA ENTERA Y NO CON EL VALOR PARSEADO a propósito: lo que hay que devolver
// es exactamente lo que había, y un round-trip por un parser de TOML es una oportunidad de
// reescribir el archivo de otro programa de una forma que ese programa no espera. Copiar la
// línea y volver a ponerla no reformatea nada.
//
// Se toma la PRIMERA coincidencia: TOML no admite la misma clave dos veces en el mismo nivel, y
// en el RustDesk.toml real `password` aparece arriba de todo, antes de cualquier sección.
var campoPassword = regexp.MustCompile(`(?m)^[ \t]*password[ \t]*=.*$`)

// passPrevia es lo que había en UN archivo antes de que la sesión lo pisara.
type passPrevia struct {
	Ruta string `json:"ruta"`
	// Linea es la línea `password = '...'` completa y sin interpretar. Va vacía cuando el
	// archivo no tenía contraseña puesta, y ese caso se distingue con Habia: «no había nada»
	// y «había esto» se cierran distinto —uno se puede scramblear tranquilo, el otro no—.
	Linea string `json:"linea,omitempty"`
	Habia bool   `json:"habia"`
}

// respaldoPantalla es la marca de que hay una sesión abierta y qué hay que devolver al cerrarla.
//
// VIVE EN DISCO Y NO SÓLO EN MEMORIA, y ese es el segundo defecto que este archivo cierra. El
// comentario de cabecera de pantalla.go promete que un agente que vuelve «no hereda una sesión
// abierta de su encarnación anterior» y manda a ver `cerrarSesionesColgadas`, una función que NO
// EXISTE: la única aparición de ese nombre en todo el repo es esa promesa. Lo que hay es
// cerrarSesionPantalla("arranque del agente"), que arranca preguntando por sesionAbierta.hay
// —estado de PROCESO, que al bootear vale false— y sale sin hacer nada. O sea que la red de
// seguridad era código muerto: un agente que se reiniciaba con una sesión puesta dejaba la
// contraseña de sesión viva para siempre.
type respaldoPantalla struct {
	Sesion  string       `json:"sesion"`
	Vence   time.Time    `json:"vence"`
	Previas []passPrevia `json:"previas"`
}

// candidatosConfigRustdesk lista los RustDesk.toml que `--password` puede llegar a escribir.
//
// La lista está ESCRITA y no descubierta por el sistema por la misma razón que en
// rustdesk_ruta.go: cuando algo no aparece, el error tiene que poder decir dónde se miró.
func candidatosConfigRustdesk() []string {
	if forzado := strings.TrimSpace(os.Getenv(envConfigRustdesk)); forzado != "" {
		return []string{forzado}
	}
	var rs []string
	switch runtime.GOOS {
	case "windows":
		// El del SERVICIO primero: es el que atiende las conexiones entrantes cuando RustDesk
		// está instalado como servicio, que es la instalación normal en Windows.
		rs = append(rs, filepath.Join(os.Getenv("SystemRoot"), "ServiceProfiles", "LocalService",
			"AppData", "Roaming", "RustDesk", "config", "RustDesk.toml"))
		rs = append(rs, filepath.Join(os.Getenv("SystemRoot"), "System32", "config", "systemprofile",
			"AppData", "Roaming", "RustDesk", "config", "RustDesk.toml"))
		if ad := os.Getenv("APPDATA"); ad != "" {
			rs = append(rs, filepath.Join(ad, "RustDesk", "config", "RustDesk.toml"))
		}
	case "darwin":
		if h, err := os.UserHomeDir(); err == nil {
			rs = append(rs, filepath.Join(h, "Library", "Preferences", "com.carriez.RustDesk", "RustDesk.toml"))
		}
		rs = append(rs, "/var/root/Library/Preferences/com.carriez.RustDesk/RustDesk.toml")
	default:
		if h, err := os.UserHomeDir(); err == nil {
			rs = append(rs, filepath.Join(h, ".config", "rustdesk", "RustDesk.toml"))
		}
		rs = append(rs, "/root/.config/rustdesk/RustDesk.toml")
	}
	return rs
}

// leerCampoPassword devuelve la línea del campo `password` tal cual, y si estaba.
//
// Un archivo que no existe o que no se puede leer NO es un error acá: es «no había nada que
// respaldar en este candidato», que es información válida y frecuente —de tres candidatos, lo
// normal es que uno o dos no existan—.
func leerCampoPassword(ruta string) (string, bool) {
	b, err := os.ReadFile(ruta) // #nosec G304 -- ruta de la lista de candidatos, no entrada remota
	if err != nil {
		return "", false
	}
	l := campoPassword.Find(b)
	if l == nil {
		return "", false
	}
	return string(l), true
}

// escribirCampoPassword devuelve una línea `password = ...` a su archivo, reemplazando la que
// haya. Reescribe de forma atómica y preserva el modo, igual que escribirTokens.
func escribirCampoPassword(ruta, linea string) error {
	b, err := os.ReadFile(ruta) // #nosec G304 -- ruta de la lista de candidatos, no entrada remota
	if err != nil {
		return fmt.Errorf("no se pudo leer %q para restituir la contraseña: %w", ruta, err)
	}
	if !campoPassword.Match(b) {
		// No se INVENTA la línea: si el campo desapareció, el archivo no es el que creíamos y
		// escribirle a ciegas es peor que no tocarlo. Se dice y se sale.
		return fmt.Errorf("%q ya no tiene un campo `password` donde restituir", ruta)
	}
	// La función de reemplazo devuelve la línea guardada LITERAL: con ReplaceAll y un string de
	// reemplazo, un `$1` adentro del blob cifrado se expandiría como referencia de grupo y
	// corrompería la contraseña. Es el modo de falla que sólo aparece con ciertos valores.
	nuevo := campoPassword.ReplaceAllFunc(b, func([]byte) []byte { return []byte(linea) })

	modo := os.FileMode(0o600)
	if fi, err := os.Stat(ruta); err == nil {
		modo = fi.Mode().Perm()
	}
	dir := filepath.Dir(ruta)
	tmp, err := os.CreateTemp(dir, ".rustdesk-*")
	if err != nil {
		return fmt.Errorf("no se pudo crear el temporal en %q: %w", dir, err)
	}
	nombre := tmp.Name()
	defer func() { _ = os.Remove(nombre) }() // no-op si el rename ya lo movió
	if _, err := tmp.Write(nuevo); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("no se pudo escribir la config de RustDesk: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("no se pudo sincronizar la config de RustDesk: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("no se pudo cerrar el temporal: %w", err)
	}
	if err := os.Chmod(nombre, modo); err != nil {
		return fmt.Errorf("no se pudo fijar el modo: %w", err)
	}
	if err := os.Rename(nombre, ruta); err != nil {
		return fmt.Errorf("no se pudo reemplazar %q: %w", ruta, err)
	}
	return nil
}

// fotoDeLasConfigs lee el campo `password` de todos los candidatos. Es la foto de ANTES.
func fotoDeLasConfigs() []passPrevia {
	cs := candidatosConfigRustdesk()
	out := make([]passPrevia, 0, len(cs))
	for _, ruta := range cs {
		linea, habia := leerCampoPassword(ruta)
		out = append(out, passPrevia{Ruta: ruta, Linea: linea, Habia: habia})
	}
	return out
}

// loQueCambio compara la foto de antes contra el estado de ahora y devuelve, de los archivos que
// CAMBIARON, lo que tenían antes. Eso —y sólo eso— es lo que hay que restituir al cerrar.
//
// Un archivo que no cambió no se toca ni se recuerda: restituirle un valor idéntico sería
// reescribir sin motivo la configuración de otro programa.
func loQueCambio(antes []passPrevia) []passPrevia {
	var out []passPrevia
	for _, p := range antes {
		ahora, hay := leerCampoPassword(p.Ruta)
		if hay == p.Habia && ahora == p.Linea {
			continue
		}
		out = append(out, p)
	}
	return out
}

// rutaRespaldoPantalla dice dónde vive la marca de sesión abierta.
//
// Al lado del token del dispositivo cuando lo hay: es el directorio de estado que el instalador
// ya creó con los permisos correctos, y meter la marca en otro lado obligaría a resolver ese
// problema una segunda vez.
//
// ELIGE EL PRIMER CANDIDATO DONDE DE VERDAD SE PUEDA ESCRIBIR, y para saberlo crea el directorio.
// Un getter con efecto es feo y se hace igual: si devolviera una ruta sin poder escribirla, el
// agente quedaría sin red para el caso en que muera con una sesión puesta —justo el caso para el
// que existe la marca— y se enteraría recién al fallar. Probar acá también garantiza que leer y
// escribir coincidan, que es lo que se rompe si cada lado elige su propio fallback.
func rutaRespaldoPantalla() string {
	if r := strings.TrimSpace(os.Getenv(envRespaldoPantalla)); r != "" {
		return r
	}
	var candidatos []string
	if tok := strings.TrimSpace(os.Getenv(envTokenFile)); tok != "" {
		candidatos = append(candidatos, filepath.Dir(tok))
	}
	if runtime.GOOS == "windows" {
		if pd := os.Getenv("ProgramData"); pd != "" {
			candidatos = append(candidatos, filepath.Join(pd, "musubi"))
		}
	} else {
		candidatos = append(candidatos, "/var/lib/musubi")
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		candidatos = append(candidatos, filepath.Join(cfg, "musubi"))
	}
	for _, dir := range candidatos {
		if err := os.MkdirAll(dir, 0o700); err == nil {
			return filepath.Join(dir, "pantalla-previa.json")
		}
	}
	// Ningún candidato sirvió. Se devuelve el primero igual para que el error diga una ruta real
	// en vez de una vacía: enterarse de dónde NO se pudo escribir es la mitad del arreglo.
	if len(candidatos) > 0 {
		return filepath.Join(candidatos[0], "pantalla-previa.json")
	}
	return "pantalla-previa.json"
}

// guardarRespaldo deja la marca en disco, 0600.
//
// EL ARCHIVO LLEVA EL BLOB CIFRADO de la contraseña anterior, así que nace y vive 0600. No es una
// clase de exposición nueva —el mismo blob ya está en el RustDesk.toml de esta misma máquina— y
// se borra en cuanto la sesión se cierra, pero mientras existe merece el mismo cuidado que el
// token del dispositivo.
func guardarRespaldo(r respaldoPantalla) error {
	ruta := rutaRespaldoPantalla()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return fmt.Errorf("no se pudo crear el directorio de %q: %w", ruta, err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("no se pudo serializar el respaldo de pantalla: %w", err)
	}
	return os.WriteFile(ruta, b, 0o600)
}

// leerRespaldo trae la marca si la hay. Sin marca no es error: es una máquina sin sesión abierta,
// que es el caso normal en cada arranque.
//
// AUSENTE E ILEGIBLE NO SON LO MISMO, Y ÉSA ES TODA LA DECISIÓN DE ESTA FUNCIÓN.
//
// Que el archivo NO EXISTA es información positiva y confiable: no quedó ninguna sesión abierta,
// y el arranque no tiene que tocar nada. Es el caso de todos los arranques normales.
//
// Que el archivo EXISTA y no se pueda usar —sin permiso, truncado, con el JSON roto— es lo
// contrario: alguien lo escribió, así que lo más probable es que sí haya quedado una sesión
// puesta. Contestar «no había» ahí dejaría la contraseña de sesión viva PARA SIEMPRE en esa
// máquina, y el error que la causó no vuelve a aparecer nunca porque nadie lo mira.
//
// Así que el sesgo va para el lado de cerrar. Cuesta una contraseña —no se sabe qué restituir, y
// el cierre termina scrambleando, que es el caso que el llamador anuncia en voz alta— y no una
// máquina abierta. La dirección del sesgo la trajo la otra sesión que persiguió este mismo bug;
// lo que se agrega acá es no aplicárselo al archivo ausente, que es el 99 % de los arranques y
// donde ese mismo sesgo destruiría la contraseña del dueño en cada reinicio: exactamente el
// defecto del que salió todo esto.
func leerRespaldo() (respaldoPantalla, bool) {
	b, err := os.ReadFile(rutaRespaldoPantalla()) // #nosec G304 -- ruta propia, no entrada remota
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return respaldoPantalla{}, false
	default:
		return respaldoPantalla{}, true
	}
	var r respaldoPantalla
	if err := json.Unmarshal(b, &r); err != nil {
		return respaldoPantalla{}, true
	}
	return r, true
}

// borrarRespaldo saca la marca. Que no esté no es error.
func borrarRespaldo() {
	if err := os.Remove(rutaRespaldoPantalla()); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%s no se pudo borrar la marca de sesión de pantalla: %v\n", cYellow("!"), err)
	}
}
