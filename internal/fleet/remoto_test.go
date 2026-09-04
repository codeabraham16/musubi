package fleet

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sshFalso instala un doble del cliente que registra sus argumentos.
//
// TIENE UN TECHO, y conviene tenerlo escrito acá arriba: este doble nunca corre la shell del otro
// lado, así que verifica QUÉ se le pasa a ssh y no QUÉ TERMINA EJECUTÁNDOSE. Esa mitad ciega
// escondió durante todo el track un `--` de más que rompía cada exec de Tier B. La cubre
// TestLoQueLlegaALaShellRemotaEsEjecutable, que corre por una shell real lo que ssh entrega.
func sshFalso(t *testing.T, cuerpo string) (registro string) {
	t.Helper()
	dir := t.TempDir()
	registro = filepath.Join(dir, "args.txt")
	guion := filepath.Join(dir, "ssh")
	// "$@" con comillas preserva los argumentos tal como llegaron, uno por línea.
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> " + registro + "; done\n" + cuerpo
	if err := os.WriteFile(guion, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	anterior := binarioSSH
	binarioSSH = guion
	t.Cleanup(func() { binarioSSH = anterior })
	return registro
}

func leerArgs(t *testing.T, ruta string) []string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		return nil
	}
	var out []string
	out = append(out, strings.Split(strings.TrimRight(string(b), "\n"), "\n")...)
	return out
}

// H2c — LA VERIFICACIÓN DE HOST KEY NO SE DESACTIVA.
//
// Un RMM con `StrictHostKeyChecking=no` es MITM-able por diseño: quien se meta en el medio de la
// red recibe los comandos y devuelve lo que quiera. Es la línea que más tienta aflojar «para que
// ande», y por eso tiene su propia prueba.
//
// Sabotaje que la hace fallar: cambiarlo a `no` o `accept-new`.
func TestNuncaSeDesactivaLaVerificacionDeHostKey(t *testing.T) {
	reg := sshFalso(t, "exit 0")
	EjecutarPorSSH("router.local", []string{"uptime"}, 10*time.Second)
	args := strings.Join(leerArgs(t, reg), " ")

	if !strings.Contains(args, "StrictHostKeyChecking=yes") {
		t.Errorf("no se exige la verificación de host key: %q", args)
	}
	for _, flojo := range []string{"StrictHostKeyChecking=no", "StrictHostKeyChecking=accept-new", "UserKnownHostsFile=/dev/null"} {
		if strings.Contains(args, flojo) {
			t.Errorf("se afloja la verificación con %q: el canal queda MITM-able", flojo)
		}
	}
	// H2b — BatchMode: nunca preguntar nada por consola.
	if !strings.Contains(args, "BatchMode=yes") {
		t.Errorf("sin BatchMode, un ssh que pide passphrase cuelga hasta el timeout: %q", args)
	}
}

// H2a — UN ARGV SIGNIFICA LO MISMO EN LOS DOS TIERS.
//
// Del otro lado de ssh SIEMPRE hay una shell. Sin citar, `echo $HOME` se expande y el mismo
// comando hace cosas distintas según el tier — la clase de sorpresa que convierte un comando de
// mantenimiento en un incidente.
//
// Sabotaje que la hace fallar: pasar el argv sin citarParaShell.
func TestElArgvSeCitaParaLaShellRemota(t *testing.T) {
	reg := sshFalso(t, "exit 0")
	EjecutarPorSSH("host", []string{"echo", "$HOME y *", "un'apostrofe"}, 10*time.Second)
	args := leerArgs(t, reg)

	// Los últimos tres son el comando, cada uno citado entero.
	n := len(args)
	if n < 3 {
		t.Fatalf("faltan argumentos: %v", args)
	}
	quiero := []string{`'echo'`, `'$HOME y *'`, `'un'\''apostrofe'`}
	got := args[n-3:]
	for i := range quiero {
		if got[i] != quiero[i] {
			t.Errorf("argumento %d = %q, esperaba %q — sin citar, la shell remota lo interpreta", i, got[i], quiero[i])
		}
	}
}

// El citado resiste todo lo que una shell interpreta.
func TestElCitadoNeutralizaLaShell(t *testing.T) {
	casos := []struct{ entrada, quiero string }{
		{"simple", `'simple'`},
		{"$HOME", `'$HOME'`},
		{"a b", `'a b'`},
		{"`whoami`", "'`whoami`'"},
		{"$(rm -rf /)", `'$(rm -rf /)'`},
		{"a;b|c&d", `'a;b|c&d'`},
		{"comilla'sola", `'comilla'\''sola'`},
		{"'", `''\'''`},
		{"", `''`},
	}
	for _, c := range casos {
		if got := citarParaShell(c.entrada); got != c.quiero {
			t.Errorf("citarParaShell(%q) = %q, esperaba %q", c.entrada, got, c.quiero)
		}
	}
}

// El 255 de ssh es un fallo de CANAL, no un exit code del comando: un «exit 255» a secas manda a
// alguien a depurar el comando cuando el problema es la conexión.
//
// Sabotaje: tratar el 255 como exit code normal.
func TestEl255DeSSHEsFalloDeCanalNoResultado(t *testing.T) {
	sshFalso(t, "echo 'ssh: connect to host router.local port 22: Connection refused' >&2; exit 255")
	res := EjecutarPorSSH("router.local", []string{"uptime"}, 5*time.Second)
	if res.ExitCode != nil {
		t.Errorf("un fallo de ssh se reportó como exit code %d del comando", *res.ExitCode)
	}
	if res.Error == "" || !strings.Contains(res.Error, "alcanzar") {
		t.Errorf("el error no explica el fallo de conexión: %q", res.Error)
	}

	// Pero un exit code REAL del comando remoto sí pasa como resultado.
	sshFalso(t, "exit 47")
	res2 := EjecutarPorSSH("host", []string{"false"}, 5*time.Second)
	if res2.Error != "" {
		t.Errorf("un exit code del comando se reportó como fallo de canal: %q", res2.Error)
	}
	if res2.ExitCode == nil || *res2.ExitCode != 47 {
		t.Errorf("exit code perdido: %+v", res2)
	}
}

// EL MENSAJE MÁS IMPORTANTE DEL SLICE: ante una host key desconocida, el error tiene que
// mandar a la solución BUENA.
//
// El fallo más común al enrolar un Tier B es exactamente éste, y el mensaje crudo de ssh manda a
// la gente a buscar `StrictHostKeyChecking=no` en internet — que es la peor solución posible.
//
// Sabotaje: devolver el stderr crudo de ssh sin traducirlo.
func TestElErrorDeHostKeyMandaALaSolucionBuenaYNoALaMala(t *testing.T) {
	sshFalso(t, "echo 'Host key verification failed.' >&2; exit 255")
	res := EjecutarPorSSH("gio@router.local", []string{"uptime"}, 5*time.Second)

	if !strings.Contains(res.Error, "ssh-keyscan") {
		t.Errorf("el error no dice CÓMO arreglarlo: %q", res.Error)
	}
	if !strings.Contains(res.Error, "NO desactives") {
		t.Errorf("el error no advierte contra la solución mala: %q", res.Error)
	}
	// Y el host se extrae bien de user@host, para que el comando sugerido sirva copiado tal cual.
	if !strings.Contains(res.Error, "router.local >>") {
		t.Errorf("el comando sugerido no usa el host limpio: %q", res.Error)
	}

	// Una clave que CAMBIÓ es más grave y se dice distinto: puede ser alguien en el medio.
	sshFalso(t, "echo 'WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!' >&2; exit 255")
	res2 := EjecutarPorSSH("router.local", []string{"uptime"}, 5*time.Second)
	if !strings.Contains(res2.Error, "alguien en el medio") {
		t.Errorf("una clave cambiada no se distingue de una desconocida: %q", res2.Error)
	}
}

// Un dispositivo sin dirección se rechaza con algo accionable, no con un ssh que falla raro.
func TestUnTierBSinDireccionSeRechazaConAlgoUtil(t *testing.T) {
	res := EjecutarPorSSH("   ", []string{"uptime"}, 5*time.Second)
	if res.Error == "" || !strings.Contains(res.Error, "address") {
		t.Errorf("no se explica que falta la dirección: %q", res.Error)
	}
	if res.ExitCode != nil {
		t.Error("no debería traer exit code")
	}
}

// El timeout corta la conexión y lo dice.
func TestElTimeoutCortaLaConexionRemota(t *testing.T) {
	sshFalso(t, "sleep 30")
	arranque := time.Now()
	res := EjecutarPorSSH("host", []string{"algo"}, time.Second)
	if tardo := time.Since(arranque); tardo > 5*time.Second {
		t.Errorf("tardó %s: el timeout no cortó", tardo)
	}
	if res.Error == "" || !strings.Contains(res.Error, "timeout") {
		t.Errorf("no se explica el timeout: %+v", res)
	}
	if res.ExitCode != nil {
		t.Error("un comando cortado por timeout no debería traer exit code")
	}
}

// La salida se acota también acá: un `cat` sobre un log de 4 GB en un router no puede volcar
// 4 GB en el cerebro.
func TestLaSalidaRemotaSeAcota(t *testing.T) {
	sshFalso(t, "yes AAAAAAAA | head -c 300000; exit 0")
	res := EjecutarPorSSH("host", []string{"cat", "/var/log/enorme"}, 20*time.Second)
	if len(res.Stdout) > SalidaMaxBytes+len(AvisoTruncado) {
		t.Errorf("la salida no se acotó: %d bytes", len(res.Stdout))
	}
	if !strings.Contains(res.Stdout, "truncada") {
		t.Error("se truncó sin dejar la marca")
	}
}

// UN TIER B EN UN PUERTO NO ESTÁNDAR TIENE QUE SER ALCANZABLE.
//
// `gio@nas:2222` es la forma que cualquiera escribe —la que usan scp, rsync, git y media
// internet— y ssh NO la entiende: para él eso es un hostname entero. El error resultante era
// «Could not resolve hostname 127.0.0.1:2222», que manda a depurar el DNS de un host que está
// perfecto. Y mover el 22 es lo primero que hace cualquiera con un NAS expuesto.
//
// Sabotaje que la hace fallar: devolver (destino, "") siempre.
func TestUnTierBEnUnPuertoNoEstandarSeAlcanza(t *testing.T) {
	casos := []struct {
		address, host, puerto string
		porque                string
	}{
		{"gio@nas.local:2222", "gio@nas.local", "2222", "la forma que cualquiera escribe"},
		{"nas.local:2222", "nas.local", "2222", "sin usuario"},
		{"gio@nas.local", "gio@nas.local", "", "sin puerto: nada que separar"},
		{"192.168.1.5:22022", "192.168.1.5", "22022", "IP con puerto"},
		// IPv6: los dos puntos son parte de la dirección, no un puerto. Partir acá rompería
		// cualquier máquina alcanzada por IPv6 — que en un tailnet son todas.
		{"gio@fe80::1", "gio@fe80::1", "", "IPv6 pelada: NO tiene puerto"},
		{"::1", "::1", "", "IPv6 pelada corta"},
		{"gio@[fe80::1]:2222", "gio@[fe80::1]", "2222", "IPv6 entre corchetes CON puerto"},
		{"[::1]", "[::1]", "", "IPv6 entre corchetes sin puerto"},
		// Lo que parece un puerto y no lo es tiene que quedarse pegado: inventar un `-p 0`
		// produce un error de ssh más raro que el original.
		{"maquina:0", "maquina:0", "", "el 0 no es un puerto"},
		{"maquina:99999", "maquina:99999", "", "fuera de rango"},
		{"maquina:web", "maquina:web", "", "no es numérico"},
	}
	for _, c := range casos {
		host, puerto := destinoYPuertoSSH(c.address)
		if host != c.host || puerto != c.puerto {
			t.Errorf("%s (%s): destinoYPuertoSSH(%q) = (%q, %q); esperaba (%q, %q)",
				c.address, c.porque, c.address, host, puerto, c.host, c.puerto)
		}
	}
}

// Y el puerto llega a la línea de comando de ssh, en las DOS invocaciones: el one-shot y la
// interactiva. Una sola de las dos arreglada sería peor que ninguna — la máquina andaría para
// ejecutar comandos y no para abrir una terminal, sin ninguna pista de por qué.
//
// Sabotaje que la hace fallar: arreglar sólo argumentosSSH y dejar argumentosShellSSH.
func TestElPuertoLlegaALaLineaDeComandoEnLosDosCaminos(t *testing.T) {
	tiene := func(args []string, quiero ...string) bool {
		for i := 0; i+len(quiero) <= len(args); i++ {
			ok := true
			for j, q := range quiero {
				if args[i+j] != q {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return false
	}

	unaVez := argumentosSSH("gio@nas:2222", []string{"uptime"}, 30*time.Second)
	if !tiene(unaVez, "-p", "2222") {
		t.Errorf("el one-shot no pasa el puerto: %v", unaVez)
	}
	if tiene(unaVez, "gio@nas:2222") {
		t.Errorf("el one-shot manda el puerto pegado al host: %v", unaVez)
	}

	interactiva := argumentosShellSSH("gio@nas:2222", 24, 80)
	if !tiene(interactiva, "-p", "2222") {
		t.Errorf("la shell interactiva no pasa el puerto: %v", interactiva)
	}
	if tiene(interactiva, "gio@nas:2222") {
		t.Errorf("la shell interactiva manda el puerto pegado al host: %v", interactiva)
	}
	// Y sin puerto no se inventa ningún -p.
	if tiene(argumentosSSH("gio@nas", []string{"uptime"}, 30*time.Second), "-p") {
		t.Error("se agregó un -p a una dirección sin puerto")
	}
}

// A28 — LO QUE SSH LE ENTREGA A LA SHELL REMOTA TIENE QUE SER EJECUTABLE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTA PRUEBA TIENE ESTA FORMA RARA, Y NO PODÍA TENER OTRA
//
// Todas las pruebas de este paquete usan un `ssh` de mentira que registra sus argumentos. Con
// eso alcanza para verificar QUÉ se le pasa a ssh, y por eso durante todo el track pareció que
// el Tier B andaba. Pero ssh NO INTERPRETA lo que va después del destino: lo junta con espacios
// y se lo entrega a la shell de LOGIN del otro lado. Un ssh de mentira nunca corre esa shell,
// así que hay una clase entera de errores que no puede ver — y había uno: un segundo `--`
// después del host llegaba como parte del comando y bash contestaba `--: invalid option`.
// TODOS los exec de Tier B fallaban, y ninguna prueba se enteró.
//
// Así que esta prueba hace lo que hace el sshd: corta los argumentos en el destino, junta el
// resto con espacios y lo corre por una shell DE VERDAD. No necesita un servidor; necesita
// dejar de simular la mitad que importa.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLoQueLlegaALaShellRemotaEsEjecutable(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("sin bash no se puede reproducir lo que hace el sshd")
	}
	const destino = "gio@nas"
	casos := []struct {
		nombre, espera string
		argv           []string
	}{
		{"comando simple", "hola\n", []string{"echo", "hola"}},
		// El citado tiene que sobrevivir el viaje: sin él, `$HOME` se expandiría del lado remoto
		// y el mismo comando haría cosas distintas según la máquina.
		{"no expande variables", "$HOME\n", []string{"echo", "$HOME"}},
		{"un argumento con espacios sigue siendo UNO", "hola mundo\n", []string{"echo", "hola mundo"}},
		{"una comilla simple no rompe la línea", "no's\n", []string{"echo", "no's"}},
		// La razón declarada del segundo `--` era proteger un argv que empieza con guion. El
		// citado ya lo hace: llega como NOMBRE de comando, no como opción de la shell.
		{"un argumento con guion no se lee como opción", "-sr\n", []string{"echo", "-sr"}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			args := argumentosSSH(destino, c.argv, 30*time.Second)
			i := -1
			for k, a := range args {
				if a == destino {
					i = k
					break
				}
			}
			if i < 0 {
				t.Fatalf("el destino %q no aparece en los argumentos: %v", destino, args)
			}
			// Exactamente lo que hace ssh con lo que sigue al destino.
			remoto := strings.Join(args[i+1:], " ")
			out, err := exec.Command("bash", "-c", remoto).CombinedOutput()
			if err != nil {
				t.Fatalf("la shell remota NO pudo ejecutar %q: %v\nsalida: %s", remoto, err, out)
			}
			if string(out) != c.espera {
				t.Errorf("la shell remota devolvió %q, esperaba %q (línea: %q)", out, c.espera, remoto)
			}
		})
	}
}
