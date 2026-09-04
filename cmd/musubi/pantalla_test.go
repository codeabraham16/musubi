package main

import (
	"errors"
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

// UN ARRANQUE SIN SESIÓN PREVIA NO TOCA LA CONTRASEÑA DEL DUEÑO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO QUE ESTA PRUEBA CIERRA, Y POR QUÉ NO SE VEÍA
//
// El arranque forzaba `sesionAbierta.hay = true` «por las dudas» y cerraba. Como `sesionAbierta`
// es estado de PROCESO, un agente que arranca no puede saber si su encarnación anterior dejó algo
// puesto — y suponer que sí significa que TODO arranque acuñaba una contraseña al azar y se la
// ponía a RustDesk, hubiera habido sesión o no.
//
// RustDesk tiene UNA sola ranura de contraseña permanente, así que eso DESTRUÍA la que el dueño
// de la máquina había elegido, en cada reinicio del agente. En `davantis-1`, que lleva quince
// cortes de energía en diez días, una vez por corte. Desde la silla de esa persona es
// indistinguible de «RustDesk me cambia la contraseña solo» — que es exactamente como se reportó.
//
// LO QUE SE MIDE ES SI SE LLAMÓ AL BINARIO, no un booleano interno: el daño es que `rustdesk
// --password` corra, y cualquier prueba que mire otra cosa se puede satisfacer sin cubrirlo.
//
// Sabotaje: volver a llamar marcarSesionAbierta(true) incondicionalmente antes de cerrar, o que
// hayMarcaDeSesion devuelva true siempre.
func TestSinMarcaEnDiscoElArranqueNoTocaLaContrasena(t *testing.T) {
	reg := rustdeskFalso(t, "exit 0")
	ruta, err := rutaMarcaDeSesion()
	if err != nil {
		t.Skipf("no se pudo resolver la ruta de la marca: %v", err)
	}
	os.Remove(ruta)
	t.Cleanup(func() { os.Remove(ruta) })

	// Esto es lo que hace el arranque del agente (agent.go), copiado acá porque `runAgent` late
	// contra la red y no se puede llamar en una prueba.
	if hayMarcaDeSesion() {
		marcarSesionAbierta(true)
		cerrarSesionPantalla("arranque del agente")
	}

	if llamadas := leerRegistro(t, reg); llamadas != "" {
		t.Fatalf("un arranque SIN sesión previa llamó a rustdesk: %q\nEso reemplaza la contraseña permanente que eligió el dueño de la máquina, en cada reinicio del agente.", llamadas)
	}
}

// Y CON LA MARCA PUESTA, SÍ CIERRA. Es el control positivo: sin él, un `hayMarcaDeSesion` que
// devolviera siempre false pasaría la prueba de arriba y dejaría la sesión abierta para siempre,
// que es el defecto opuesto y peor.
//
// Sabotaje: que hayMarcaDeSesion devuelva siempre false.
func TestConLaMarcaPuestaElArranqueSiCierraLaSesion(t *testing.T) {
	reg := rustdeskFalso(t, "exit 0")
	ruta, err := rutaMarcaDeSesion()
	if err != nil {
		t.Skipf("no se pudo resolver la ruta de la marca: %v", err)
	}
	if err := os.WriteFile(ruta, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(ruta) })

	if hayMarcaDeSesion() {
		marcarSesionAbierta(true)
		cerrarSesionPantalla("arranque del agente")
	}

	llamadas := leerRegistro(t, reg)
	if !strings.Contains(llamadas, "--password") {
		t.Fatalf("había una sesión de la encarnación anterior y el arranque no la cerró: %q", llamadas)
	}
	// Y la marca se fue: si quedara, el arranque siguiente cerraría una sesión que ya no existe
	// y volvería a pisar la contraseña del dueño.
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("la marca sobrevivió al cierre: %v", err)
	}
}

// LA MARCA DE DISCO Y LA DE PROCESO SE MUEVEN JUNTAS. Separarlas deja que una diga que hay sesión
// y la otra que no, y la que manda al arrancar es la que nadie mira mientras el proceso vive.
//
// Sabotaje: que marcarSesionAbierta deje de escribir o de borrar el archivo.
func TestLaMarcaDeDiscoSigueALaDeProceso(t *testing.T) {
	ruta, err := rutaMarcaDeSesion()
	if err != nil {
		t.Skipf("no se pudo resolver la ruta de la marca: %v", err)
	}
	t.Cleanup(func() { os.Remove(ruta); marcarSesionAbierta(false) })

	marcarSesionAbierta(true)
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("se abrió una sesión y la marca de disco no quedó: %v — un agente que reinicie no va a saber que hay algo que cerrar", err)
	}
	if !hayMarcaDeSesion() {
		t.Fatal("hayMarcaDeSesion no ve la marca que acaba de escribirse")
	}

	marcarSesionAbierta(false)
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("se cerró la sesión y la marca sigue en disco: %v — el próximo arranque va a pisar la contraseña del dueño sin motivo", err)
	}
	if hayMarcaDeSesion() {
		t.Fatal("hayMarcaDeSesion sigue diciendo que hay sesión después de cerrarla")
	}
}

// UN ERROR AL MIRAR LA MARCA SE CONTESTA «SÍ HABÍA». El sesgo del error tiene que costar una
// contraseña de sesión, no una máquina abierta.
//
// SE PRUEBA SOBRE marcaSegunStat Y NO SOBRE hayMarcaDeSesion, y eso lo enseñó el sabotaje: la
// ruta sale de os.Executable(), que no se puede mover en una prueba, así que el caso «Stat falló
// por algo que no es "no existe"» era INALCANZABLE. La primera versión de esta prueba ponía un
// directorio en la ruta —sobre un directorio Stat contesta sin error— y por eso pasaba en verde
// con la rama saboteada.
//
// Sabotaje: devolver false en el caso por defecto de marcaSegunStat.
func TestAnteLaDudaLaMarcaSeContestaQueSi(t *testing.T) {
	casos := []struct {
		nombre   string
		err      error
		esperado bool
	}{
		{"el archivo está", nil, true},
		{"no está: es una respuesta, no una duda", os.ErrNotExist, false},
		{"permiso denegado: no sé, así que sí", os.ErrPermission, true},
		{"cualquier otra cosa: no sé, así que sí", errors.New("E/S"), true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := marcaSegunStat(c.err); got != c.esperado {
				t.Fatalf("marcaSegunStat(%v) = %v, esperaba %v", c.err, got, c.esperado)
			}
		})
	}
}
