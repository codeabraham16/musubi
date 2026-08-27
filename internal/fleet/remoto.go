package fleet

// remoto.go ejecuta comandos en dispositivos de TIER B — los que no pueden correr un agente:
// routers, NAS, servers ajenos, Raspberry Pis. Track «Control de flota», S7.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE INVOCA AL `ssh` DEL SISTEMA, Y NO ES PEREZA
//
// La alternativa era `golang.org/x/crypto/ssh` y una tabla de llaves en la base. Sería la 7ª
// dependencia directa del repo, y —más grave— pondría a Musubi a guardar credenciales de acceso
// con permiso de escritura a toda la flota: exactamente el llavero que S6 se negó a construir
// para las contraseñas de pantalla.
//
// Invocando al `ssh` del sistema, LA CREDENCIAL NUNCA ENTRA A MUSUBI. Ni el secreto ni una
// referencia a él. Las llaves las administra quien opera, con ssh-agent y ~/.ssh/config, como ya
// lo hace — y se hereda gratis todo lo que ya tiene configurado: jump hosts, certificados,
// claves por host, known_hosts.
//
// El costo se declara: el cerebro necesita `ssh` instalado y hay un fork+exec por comando. Para
// decenas de máquinas no es nada. Si algún día son miles, se revisa.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// binarioSSH es el cliente. Es `var` para poder apuntarlo a un doble en las pruebas.
var binarioSSH = "ssh"

// ResultadoRemoto es cómo salió un comando en un Tier B. Misma forma y misma distinción que en
// Tier A: ExitCode es el resultado del comando; Error es un fallo del CANAL.
type ResultadoRemoto struct {
	ExitCode *int
	Stdout   string
	Stderr   string
	Error    string
}

// EjecutarPorSSH corre `argv` en `destino` (host o user@host).
//
// NUNCA devuelve error al llamador: un comando que falla es un RESULTADO. Misma regla que el
// ejecutor del agente, y por la misma razón — un `grep` que no encuentra nada devuelve 1 y no es
// una máquina rota.
func EjecutarPorSSH(destino string, argv []string, timeout time.Duration) ResultadoRemoto {
	var res ResultadoRemoto
	destino = strings.TrimSpace(destino)
	if destino == "" {
		res.Error = "el dispositivo no tiene dirección: enrolalo con `address` (host o user@host)"
		return res
	}
	argv = LimpiarArgv(argv)
	if len(argv) == 0 {
		res.Error = "comando vacío"
		return res
	}
	if timeout <= 0 || timeout > ComandoTimeoutMax {
		timeout = ComandoTimeoutDefault
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binarioSSH, argumentosSSH(destino, argv, timeout)...)
	cmd.Stdin = nil // igual que en Tier A: nada que leer, así que el remoto ve EOF y no cuelga
	// WaitDelay ES LO QUE HACE QUE EL TIMEOUT SIRVA DE VERDAD, y su ausencia era un bug.
	//
	// CommandContext mata al proceso al vencer el contexto, pero `Run` no vuelve ahí: vuelve
	// cuando se cierran las TUBERÍAS de stdout/stderr. Si el comando dejó un hijo en background,
	// ese hijo heredó las tuberías y las mantiene abiertas — así que `Run` sigue esperando
	// aunque el proceso original ya esté muerto. Medido: un comando de 1 s de timeout tardaba 30.
	//
	// Con WaitDelay, pasado ese margen se cierran las tuberías a la fuerza y el llamador se
	// libera. El margen existe para no perder la salida de un comando que estaba terminando bien.
	cmd.WaitDelay = 2 * time.Second

	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()

	res.Stdout, _ = TruncarSalida(so.String())
	res.Stderr, _ = TruncarSalida(se.String())

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.Error = fmt.Sprintf("el comando excedió su timeout de %s y se cortó la conexión", timeout)
	case err == nil:
		cero := 0
		res.ExitCode = &cero
	default:
		var salida *exec.ExitError
		if errors.As(err, &salida) {
			code := salida.ExitCode()
			// 255 es el código con el que `ssh` reporta SUS PROPIOS fallos (no pudo conectar,
			// host key desconocida, autenticación rechazada). Es ambiguo —un comando remoto
			// también puede salir 255— pero tratarlo como fallo de CANAL es lo correcto en la
			// abrumadora mayoría de los casos, y la alternativa es peor: un «exit 255» a secas
			// manda a alguien a depurar el comando cuando el problema es la conexión.
			if code == 255 {
				res.Error = explicarFalloSSH(destino, se.String())
			} else {
				res.ExitCode = &code
			}
		} else {
			res.Error = "no se pudo invocar ssh: " + err.Error()
		}
	}
	return res
}

// argumentosSSH arma la línea de comandos.
func argumentosSSH(destino string, argv []string, timeout time.Duration) []string {
	segundos := int(timeout.Seconds())
	if segundos < 1 {
		segundos = 1
	}
	args := []string{
		// BatchMode: NUNCA preguntar nada por consola. Sin esto, un ssh que quiere una
		// passphrase o una confirmación espera una entrada que en un servidor no existe, y el
		// comando cuelga hasta el timeout.
		"-o", "BatchMode=yes",
		// El timeout de CONEXIÓN, aparte del timeout del comando: un host apagado tiene que
		// fallar rápido en vez de consumir toda la ventana.
		"-o", fmt.Sprintf("ConnectTimeout=%d", minInt(segundos, 15)),
		// StrictHostKeyChecking NO SE DESACTIVA. Ver explicarFalloSSH: un RMM con
		// `StrictHostKeyChecking=no` es MITM-able por diseño, y es la línea que más tienta
		// aflojar «para que ande».
		"-o", "StrictHostKeyChecking=yes",
	}
	host, puerto := destinoYPuertoSSH(destino)
	if puerto != "" {
		args = append(args, "-p", puerto)
	}
	// UN SOLO `--`, Y VA ANTES DEL HOST. El segundo —que estuvo acá hasta que A28 lo destapó—
	// rompía TODOS los exec de Tier B: ssh no interpreta lo que va después del destino, lo JUNTA
	// con espacios y se lo entrega a la shell de login remota, así que ese `--` llegaba del otro
	// lado como parte del comando y bash contestaba `--: invalid option`. Ninguna prueba lo vio
	// porque todas usan un ssh de mentira que nunca corre una shell.
	//
	// Y no hacía falta: el segundo `--` estaba para proteger un argv que empiece con guion, pero
	// eso YA lo da el citado de abajo. Un `'-sr'` entre comillas simples llega a la shell remota
	// como el NOMBRE de un comando (`-sr: command not found`), nunca como una opción.
	args = append(args, "--", host)
	// Cada argumento se CITA para la shell remota. Del otro lado de ssh SIEMPRE hay una shell:
	// sin citar, `echo $HOME` se expande y el mismo comando hace cosas distintas según el tier.
	for _, a := range argv {
		args = append(args, citarParaShell(a))
	}
	return args
}

// citarParaShell envuelve un argumento en comillas simples POSIX.
//
// Dentro de comillas simples NADA se interpreta: ni variables, ni globs, ni sustitución de
// comandos. El único carácter especial es la comilla simple misma, que se cierra, se escapa y se
// reabre — el idioma `'\”`.
func citarParaShell(a string) string {
	return "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
}

// explicarFalloSSH convierte el ruido de ssh en algo accionable.
//
// Importa más de lo que parece: el fallo más común al enrolar un Tier B es la host key
// desconocida, y el mensaje crudo de ssh manda a la gente a buscar `StrictHostKeyChecking=no` en
// internet — que es la peor solución posible. Se dice cuál es la buena.
func explicarFalloSSH(destino, stderr string) string {
	s := strings.ToLower(stderr)
	switch {
	case strings.Contains(s, "host key verification failed"), strings.Contains(s, "no matching host key"):
		return fmt.Sprintf("la clave de host de %q no está verificada. NO desactives StrictHostKeyChecking: "+
			"agregá el host una vez con `ssh-keyscan -H %s >> ~/.ssh/known_hosts` DESPUÉS de verificar la huella por un canal confiable.",
			destino, hostDe(destino))
	case strings.Contains(s, "remote host identification has changed"):
		return fmt.Sprintf("LA CLAVE DE HOST DE %q CAMBIÓ. Puede ser una reinstalación... o alguien en el medio. "+
			"No lo ignores: verificá la huella nueva por un canal confiable antes de tocar known_hosts.", destino)
	case strings.Contains(s, "permission denied"):
		return fmt.Sprintf("%q rechazó la autenticación. Musubi NO guarda credenciales: usa las de tu ssh-agent y "+
			"~/.ssh/config. Probá `ssh %s` a mano desde este mismo host.", destino, destino)
	case strings.Contains(s, "connection timed out"), strings.Contains(s, "connection refused"),
		strings.Contains(s, "could not resolve"), strings.Contains(s, "no route to host"):
		return fmt.Sprintf("no se pudo alcanzar %q: %s", destino, primeraLinea(stderr))
	default:
		if l := primeraLinea(stderr); l != "" {
			return "ssh falló: " + l
		}
		return "ssh falló sin decir por qué"
	}
}

func hostDe(destino string) string {
	if i := strings.LastIndex(destino, "@"); i >= 0 {
		return destino[i+1:]
	}
	return destino
}

func primeraLinea(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SSHFalsoParaTest instala un doble del cliente ssh y devuelve la función que lo restaura.
//
// Vive en el código de producción y no en un _test.go porque lo usan las pruebas de OTRO paquete
// (internal/mcp, que ejercita el ruteo por tier). `binarioSSH` es privado a propósito —nadie
// fuera de acá debería poder apuntar el cliente a otro lado— y ésta es la única puerta, explícita
// y nombrada, para abrirlo en una prueba.
func SSHFalsoParaTest(t interface{ Fatal(...any) }, cuerpo string) (restaurar func()) {
	dir, err := os.MkdirTemp("", "sshfalso")
	if err != nil {
		t.Fatal(err)
	}
	guion := filepath.Join(dir, "ssh")
	if err := os.WriteFile(guion, []byte("#!/bin/sh\n"+cuerpo+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	anterior := binarioSSH
	binarioSSH = guion
	return func() { binarioSSH = anterior; os.RemoveAll(dir) }
}

// ── Telemetría remota (S7b / S8) ────────────────────────────────────────────────────────────

// binarioADB es el cliente de Android Debug Bridge. `var` para poder doblarlo en las pruebas.
var binarioADB = "adb"

// Transporte es CÓMO se llega a un dispositivo que no corre el agente.
type Transporte string

const (
	TransporteSSH Transporte = "ssh" // Tier B: routers, NAS, servers sin agente
	TransporteADB Transporte = "adb" // Tier C: Android — que ES Linux, con el mismo /proc
)

// separadorProc parte las secciones de la lectura remota. Se eligió una cadena que no puede
// aparecer en /proc ni en la salida de df.
const separadorProc = "@@musubi@@"

// guionLecturaProc es UNA sola invocación que trae todo.
//
// Una por archivo serían seis viajes de red y seis fork+exec por dispositivo y por sondeo. Con
// una flota de decenas de máquinas eso es la diferencia entre un sondeo que termina y uno que no.
//
// `2>/dev/null` en la temperatura y en df: en un router o en un Android esos caminos pueden no
// existir, y un stderr ruidoso ensuciaría la sección siguiente. Lo que falta queda vacío, que es
// exactamente lo que el parseo interpreta como «no medido».
const guionLecturaProc = `cat /proc/stat 2>/dev/null; echo '` + separadorProc + `'; ` +
	`cat /proc/meminfo 2>/dev/null; echo '` + separadorProc + `'; ` +
	`cat /proc/loadavg 2>/dev/null; echo '` + separadorProc + `'; ` +
	`cat /proc/uptime 2>/dev/null; echo '` + separadorProc + `'; ` +
	`df -B1 / 2>/dev/null; echo '` + separadorProc + `'; ` +
	`cat /sys/class/thermal/thermal_zone0/temp 2>/dev/null; echo '` + separadorProc + `'; ` +
	`grep -c ^processor /proc/cpuinfo 2>/dev/null`

// EsIOS reconoce un dispositivo Apple móvil por el `os` que se declaró al enrolarlo.
//
// Existe para poder decir la verdad temprano. iOS NO expone /proc, no permite ejecutar código
// ajeno y no da acceso a métricas del sistema sin un MDM con perfil de supervisión — que es un
// producto entero, con su ceremonia de inscripción y su certificado.
//
// Musubi puede tener un iPhone en el INVENTARIO (existe, es de alguien, está en un proyecto) y no
// puede medirlo ni controlarlo. Intentarlo y devolver un error de adb sería peor: mandaría a
// alguien a depurar el cable cuando el problema es que la cosa no se puede.
func EsIOS(sistemaOperativo string) bool {
	s := strings.ToLower(strings.TrimSpace(sistemaOperativo))
	return s == "ios" || s == "ipados" || strings.HasPrefix(s, "iphone") || strings.HasPrefix(s, "ipad")
}

// ErrIOSNoSeMide es el techo declarado de iOS.
var ErrIOSNoSeMide = errors.New("iOS no expone /proc ni permite ejecutar nada sin un MDM con perfil de supervisión: " +
	"este dispositivo puede estar en el inventario, pero no se puede medir ni controlar desde Musubi. No es una limitación de Musubi, es de la plataforma")

// TomarMuestraRemota lee el estado de un dispositivo por SSH o por ADB.
//
// `cpu` lleva el estado entre sondeos para poder derivar el porcentaje; el llamador guarda uno
// POR DISPOSITIVO. Con nil, la muestra sale sin CPU — que es lo honesto en el primer sondeo.
func TomarMuestraRemota(destino string, tr Transporte, cpu *ContadorCPUExportado, timeout time.Duration) (Muestra, error) {
	var res ResultadoRemoto
	switch tr {
	case TransporteADB:
		res = ejecutarPorADB(destino, []string{"sh", "-c", guionLecturaProc}, timeout)
	default:
		res = EjecutarPorSSH(destino, []string{"sh", "-c", guionLecturaProc}, timeout)
	}
	if res.Error != "" {
		return Muestra{}, errors.New(res.Error)
	}
	if res.ExitCode != nil && *res.ExitCode != 0 && strings.TrimSpace(res.Stdout) == "" {
		return Muestra{}, fmt.Errorf("el dispositivo no devolvió nada (exit %d): %s", *res.ExitCode, primeraLinea(res.Stderr))
	}
	l, ok := ParsearLecturaRemota(res.Stdout)
	if !ok {
		// Sin /proc no hay nada que medir. Pasa con un router de firmware propietario: responde
		// al ssh y no tiene /proc. Decirlo es mejor que devolver una muestra de ceros.
		return Muestra{}, errors.New("el dispositivo respondió pero no expone /proc: no se puede medir por este camino")
	}
	var interno *contadorCPU
	if cpu != nil {
		interno = &cpu.c
	}
	return MuestraDesde(l, interno), nil
}

// ParsearLecturaRemota parte la salida del guion en sus secciones.
//
// Devuelve ok=false si del otro lado NO HAY UN LINUX, y la validación es SEMÁNTICA, no de
// presencia: la primera versión sólo miraba que las secciones no estuvieran vacías, y un router
// de firmware propietario que contesta `Welcome to RouterOS` con exit 0 pasaba el filtro. Lo
// agarró la prueba que usa esa respuesta exacta.
//
// El resultado de aceptarla habría sido una Muestra de ceros guardada como si fuera una
// medición: el panel mostrando 0 % de CPU, 0 de RAM y 0 de disco en un router que anda
// perfectamente. Ese cero se cree, y es exactamente contra lo que existe todo el diseño.
func ParsearLecturaRemota(salida string) (LecturasProc, bool) {
	partes := strings.Split(salida, separadorProc)
	tomar := func(i int) string {
		if i < len(partes) {
			return strings.TrimSpace(partes[i])
		}
		return ""
	}
	l := LecturasProc{
		Stat:    tomar(0),
		Meminfo: tomar(1),
		Loadavg: tomar(2),
		Uptime:  tomar(3),
		Df:      tomar(4),
		TempMil: tomar(5),
	}
	if n, err := strconv.Atoi(strings.TrimSpace(tomar(6))); err == nil && n > 0 {
		l.NumCPU = n
	}
	// SEMÁNTICA: que /proc/stat parsee como jiffies, o que meminfo traiga un MemTotal. Texto
	// suelto no alcanza.
	_, _, statOK := ParsearJiffies(l.Stat)
	var mem Muestra
	ParsearMeminfo(l.Meminfo, &mem)
	if !statOK && mem.MemTotal == 0 {
		return LecturasProc{}, false
	}
	return l, true
}

// ejecutarPorADB corre un comando en un Android.
//
// ADB no tiene el problema del citado que sí tiene ssh: `adb shell` con argumentos separados los
// pasa... a una shell igual. Así que se cita con el MISMO idioma, por la misma razón — que un
// argv signifique lo mismo en todos los tiers.
func ejecutarPorADB(serie string, argv []string, timeout time.Duration) ResultadoRemoto {
	var res ResultadoRemoto
	serie = strings.TrimSpace(serie)
	if serie == "" {
		res.Error = "el dispositivo no tiene dirección: enrolalo con `address` (el serial o host:puerto de adb)"
		return res
	}
	argv = LimpiarArgv(argv)
	if len(argv) == 0 {
		res.Error = "comando vacío"
		return res
	}
	if timeout <= 0 || timeout > ComandoTimeoutMax {
		timeout = ComandoTimeoutDefault
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"-s", serie, "shell"}
	for _, a := range argv {
		args = append(args, citarParaShell(a))
	}
	cmd := exec.CommandContext(ctx, binarioADB, args...)
	cmd.Stdin = nil
	cmd.WaitDelay = 2 * time.Second

	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()

	res.Stdout, _ = TruncarSalida(so.String())
	res.Stderr, _ = TruncarSalida(se.String())

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		res.Error = fmt.Sprintf("el comando excedió su timeout de %s y se cortó", timeout)
	case err == nil:
		cero := 0
		res.ExitCode = &cero
	default:
		var salida *exec.ExitError
		if errors.As(err, &salida) {
			code := salida.ExitCode()
			res.ExitCode = &code
		} else {
			res.Error = explicarFalloADB(serie, err)
		}
	}
	// adb reporta SUS propios fallos por stdout/stderr con exit 0 — «device not found», «device
	// unauthorized»— así que hay que mirar el texto. Es feo y es como es.
	if res.Error == "" {
		if fallo := detectarFalloADB(serie, res.Stdout+res.Stderr); fallo != "" {
			res.Error = fallo
			res.ExitCode = nil
		}
	}
	return res
}

func explicarFalloADB(serie string, err error) string {
	if strings.Contains(err.Error(), "executable file not found") {
		return "no está instalado `adb` en el cerebro: hace falta para llegar a un dispositivo Android (paquete android-tools-adb o platform-tools)"
	}
	return fmt.Sprintf("no se pudo invocar adb para %q: %v", serie, err)
}

// detectarFalloADB traduce los fallos que adb reporta como texto en vez de como exit code.
//
// El de `unauthorized` es EL importante: significa que el diálogo «¿Permitir depuración USB?» no
// se aceptó en la pantalla del teléfono. Sin esa traducción, quien opera ve una salida vacía y no
// tiene forma de saber que el problema está en la mano de alguien, no en la red.
func detectarFalloADB(serie, texto string) string {
	t := strings.ToLower(texto)
	switch {
	case strings.Contains(t, "device unauthorized"), strings.Contains(t, "device still authorizing"):
		return fmt.Sprintf("el dispositivo %q no autorizó la depuración: hay que aceptar el diálogo «¿Permitir depuración USB?» EN LA PANTALLA del teléfono. Musubi no puede aceptarlo por vos, y eso es a propósito.", serie)
	// OJO con el matcheo: adb dice `error: device '<serial>' not found`, CON el serial en el
	// medio. Buscar la cadena "device not found" no matchea nunca — lo agarró la prueba, que usa
	// el mensaje real de adb y no una versión idealizada.
	case strings.Contains(t, "not found"), strings.Contains(t, "device offline"):
		return fmt.Sprintf("adb no encuentra el dispositivo %q. Si es por red, hace falta un `adb connect %s` previo; si es por USB, que esté enchufado y con depuración activada.", serie, serie)
	case strings.Contains(t, "more than one device"):
		return "hay más de un dispositivo conectado y el serial no alcanzó para elegir: enrolá el dispositivo con su serial exacto (`adb devices`)"
	}
	return ""
}

// destinoYPuertoSSH separa el puerto de la dirección de un Tier B.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL HUECO QUE CIERRA, Y POR QUÉ ERA PEOR DE LO QUE PARECE
//
// `gio@nas:2222` es la forma que cualquiera escribe —es la que usan scp, rsync, git y media
// internet— y ssh NO la entiende: para él eso es un hostname entero. Antes de esta función, dar
// de alta un NAS o un router en un puerto no estándar (que es lo normal en cuanto alguien mueve
// el 22) fallaba con:
//
//	ssh: Could not resolve hostname 127.0.0.1:2222: Name or service not known
//
// O sea que el error mandaba a depurar el DNS de un host que estaba perfecto. Es exactamente la
// clase de mensaje engañoso contra la que existe explicarFalloSSH, y esta vez lo producíamos
// nosotros al armar mal la invocación.
//
// SÓLO VALE PARA SSH. En ADB, `host:puerto` ES el serial del dispositivo y hay que pasarlo
// entero: partirlo ahí rompería todos los Android por red. Por eso esta función se llama desde
// los dos armadores de argumentos de ssh y de ningún otro lado.
// ────────────────────────────────────────────────────────────────────────────────────────────
func destinoYPuertoSSH(destino string) (host, puerto string) {
	destino = strings.TrimSpace(destino)
	usuario := ""
	if i := strings.LastIndex(destino, "@"); i >= 0 {
		usuario, destino = destino[:i+1], destino[i+1:]
	}
	// IPv6 entre corchetes: [::1]:2222 (y también [::1] a secas).
	if strings.HasPrefix(destino, "[") {
		if cierra := strings.Index(destino, "]"); cierra > 0 {
			dir := destino[:cierra+1]
			resto := destino[cierra+1:]
			if strings.HasPrefix(resto, ":") && esPuerto(resto[1:]) {
				return usuario + dir, resto[1:]
			}
			return usuario + dir, ""
		}
		return usuario + destino, ""
	}
	// UN solo `:` con dígitos detrás es un puerto. VARIOS `:` sin corchetes es una IPv6 pelada
	// (`::1`, `fe80::1`), donde el último grupo también son «dígitos» — por eso se cuenta, y no
	// alcanza con mirar lo que hay después del último `:`.
	if strings.Count(destino, ":") == 1 {
		if i := strings.Index(destino, ":"); esPuerto(destino[i+1:]) {
			return usuario + destino[:i], destino[i+1:]
		}
	}
	return usuario + destino, ""
}

// esPuerto dice si eso es un número de puerto plausible. No alcanza con «son dígitos»: un
// hostname como `maquina:0` o `host:99999` no describe ningún puerto real, y tratarlo como tal
// produciría un error de ssh todavía más raro que el original.
func esPuerto(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	return n > 0 && n <= 65535
}
