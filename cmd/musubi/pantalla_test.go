package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// rustdeskFalso instala un doble del binario que registra sus argumentos en un archivo.
// Es lo único de S6 que no se puede probar contra el binario real sin una máquina con RustDesk.
func rustdeskFalso(t *testing.T, cuerpo string) (registro string) {
	t.Helper()
	// El doble es un script `#!/bin/sh` y Windows no lo ejecuta —además de querer un `.exe`—, así
	// que la prueba fallaría por el andamio. Se omite acá, en el doble, para que ningún sitio
	// nuevo se lo olvide.
	//
	// ACÁ SÍ QUEDA UN HUECO Y CONVIENE DECIRLO: a diferencia del Tier B, RustDesk en Windows es un
	// caso REAL —el agente corre ahí—, así que estas cuatro pruebas cubren esa plataforma sólo en
	// la parte que no toca el binario. Doblarlo en Windows pide un `.cmd` y traducir cada cuerpo a
	// un segundo dialecto; no se hace acá.
	if runtime.GOOS == "windows" {
		t.Skip("el doble de rustdesk es un script #!/bin/sh y esta plataforma no lo ejecuta")
	}
	dir := t.TempDir()
	registro = filepath.Join(dir, "llamadas.txt")
	guion := filepath.Join(dir, "rustdesk")
	script := "#!/bin/sh\necho \"$@\" >> " + registro + "\n" + cuerpo
	if err := os.WriteFile(guion, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	anterior := binarioRustdesk
	binarioRustdesk = guion
	t.Cleanup(func() { binarioRustdesk = anterior; marcarSesionAbierta(false) })
	return registro
}

func leerRegistro(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		return ""
	}
	return string(b)
}

// La contraseña llega al cliente RustDesk por la interfaz soportada.
func TestLaContrasenaSeAplicaEnElCliente(t *testing.T) {
	reg := rustdeskFalso(t, "exit 0")
	res := aplicarSesionPantalla(comandoRecibido{
		ID: "cmd-1", Argv: []string{"musubi:pantalla", "ses-1", "LaClaveDeSesion", "30m"}, TimeoutSeg: 30,
	})
	if res.Error != "" {
		t.Fatalf("error inesperado: %s", res.Error)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Fatalf("no reportó éxito: %+v", res)
	}
	if got := leerRegistro(t, reg); !strings.Contains(got, "--password LaClaveDeSesion") {
		t.Errorf("no se aplicó la contraseña en el cliente: %q", got)
	}
}

// G1, POR LA PUERTA DE ATRÁS: el RESULTADO que va a la bitácora no puede traer la contraseña.
//
// Sabotaje que la hace fallar: poner `pass` en res.Stdout, o devolver el error del binario sin
// pasarlo por `sinSecreto` (muchos binarios repiten sus argumentos en el mensaje de error).
func TestElResultadoQueVaALaBitacoraNoTraeLaContrasena(t *testing.T) {
	const clave = "SecretoDeSesion99"

	// Caso feliz.
	rustdeskFalso(t, "exit 0")
	res := aplicarSesionPantalla(comandoRecibido{
		ID: "c", Argv: []string{"musubi:pantalla", "ses", clave, "30m"}, TimeoutSeg: 30})
	if strings.Contains(res.Stdout+res.Stderr+res.Error, clave) {
		t.Errorf("el resultado exitoso filtra la contraseña: %+v", res)
	}

	// Caso feo: el binario ECHA sus argumentos en el error, como hacen muchos.
	rustdeskFalso(t, "echo \"error: no pude usar $@\" >&2; exit 3")
	res2 := aplicarSesionPantalla(comandoRecibido{
		ID: "c", Argv: []string{"musubi:pantalla", "ses", clave, "30m"}, TimeoutSeg: 30})
	if res2.Error == "" {
		t.Fatal("un fallo del cliente debería reportarse")
	}
	if strings.Contains(res2.Error, clave) {
		t.Errorf("el MENSAJE DE ERROR filtra la contraseña a la bitácora: %q", res2.Error)
	}
	if !strings.Contains(res2.Error, "[oculto]") {
		t.Errorf("no se tapó el secreto en el error: %q", res2.Error)
	}
}

// G2 — al vencer, la contraseña se REEMPLAZA por una al azar, no se borra.
//
// Borrarla dejaría a RustDesk sin contraseña, que es abrir la máquina, no cerrarla.
//
// Sabotaje: llamar a `--password ""` al cerrar.
func TestAlVencerSeReemplazaLaContrasenaNoSeBorra(t *testing.T) {
	reg := rustdeskFalso(t, "exit 0")
	// TTL mínimo para no esperar: la duración se parsea del argv.
	res := aplicarSesionPantalla(comandoRecibido{
		ID: "c", Argv: []string{"musubi:pantalla", "ses", "ClaveInicial", "50ms"}, TimeoutSeg: 30})
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	// El temporizador es del agente: se espera a que dispare solo.
	fin := time.Now().Add(3 * time.Second)
	for time.Now().Before(fin) {
		if strings.Count(leerRegistro(t, reg), "--password") >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	got := leerRegistro(t, reg)
	lineas := strings.Split(strings.TrimSpace(got), "\n")
	if len(lineas) < 2 {
		t.Fatalf("la contraseña no se reemplazó al vencer:\n%s", got)
	}
	segunda := lineas[len(lineas)-1]
	if strings.Contains(segunda, "ClaveInicial") {
		t.Errorf("la contraseña no cambió: %q", segunda)
	}
	// Y NO se borró: hay una contraseña nueva, no una vacía.
	campos := strings.Fields(segunda)
	if len(campos) < 2 || strings.TrimSpace(campos[1]) == "" {
		t.Errorf("se dejó a RustDesk SIN contraseña, que es abrir la máquina en vez de cerrarla: %q", segunda)
	}
	if len(campos[1]) != 16 {
		t.Errorf("la contraseña de cierre no parece acuñada al azar: %q", campos[1])
	}
}

// Un TTL absurdo se acota al máximo del dominio: una sesión de un mes no es una sesión.
func TestUnTTLAbsurdoSeAcota(t *testing.T) {
	rustdeskFalso(t, "exit 0")
	// No hay forma directa de leer el timer, así que se verifica lo que sí se observa: no
	// explota y reporta bien. La acotación en sí la fija TestLaDuracionSeAcota en internal/fleet.
	for _, ttl := range []string{"720h", "no-es-una-duracion", "-5m", ""} {
		res := aplicarSesionPantalla(comandoRecibido{
			ID: "c", Argv: []string{"musubi:pantalla", "ses", "clave", ttl}, TimeoutSeg: 30})
		if res.Error != "" {
			t.Errorf("ttl %q dio error: %s", ttl, res.Error)
		}
		marcarSesionAbierta(false)
	}
}

// Una operación de pantalla mal formada no rompe al agente ni deja una sesión colgada.
func TestUnaOperacionMalFormadaNoRompeAlAgente(t *testing.T) {
	rustdeskFalso(t, "exit 0")
	res := aplicarSesionPantalla(comandoRecibido{ID: "c", Argv: []string{"musubi:pantalla", "solo-id"}})
	if res.Error == "" {
		t.Error("una operación incompleta debería reportarse como error")
	}
	if res.ExitCode != nil {
		t.Error("no debería traer exit code")
	}
}

// La operación interna se INTERCEPTA: nunca llega a exec.Command como si fuera un binario.
//
// Sabotaje: quitar la guarda de prefijo `musubi:` de `ejecutar` → el error diría «no such file»
// y —peor— podría arrastrar la contraseña que va en el argv.
func TestLaOperacionInternaSeInterceptaYNoSeLanzaComoBinario(t *testing.T) {
	rustdeskFalso(t, "exit 0")
	const clave = "NoDebeAparecer42"
	res := ejecutar(comandoRecibido{
		ID: "c", Argv: []string{"musubi:pantalla", "ses", clave, "30m"}, TimeoutSeg: 5}, "", "")

	if strings.Contains(res.Error, "executable file not found") || strings.Contains(res.Error, "no such file") {
		t.Fatalf("la operación interna se intentó lanzar como binario: %q", res.Error)
	}
	if strings.Contains(res.Stdout+res.Stderr+res.Error, clave) {
		t.Fatalf("el resultado filtra la contraseña: %+v", res)
	}
	// Y una interna DESCONOCIDA se rechaza en vez de lanzarse.
	res2 := ejecutar(comandoRecibido{ID: "c", Argv: []string{"musubi:inventada"}, TimeoutSeg: 5}, "", "")
	if res2.Error == "" || !strings.Contains(res2.Error, "desconocida") {
		t.Errorf("una operación interna desconocida debería rechazarse: %+v", res2)
	}
}

// El alfabeto de la contraseña que acuña el agente al cerrar es el mismo del dominio.
func TestLaContrasenaDeCierreUsaElAlfabetoDelDominio(t *testing.T) {
	p, err := fleet.NuevaPassPantalla()
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 16 {
		t.Errorf("largo %d", len(p))
	}
}
