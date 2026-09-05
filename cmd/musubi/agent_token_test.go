package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// credDePrueba es un llavero de un solo token sin archivo detrás: lo que necesita cualquier prueba
// que sólo quiera latir. Sin archivo, Sumar falla a propósito —eso es lo que se afirma abajo.
func credDePrueba(tok string) *credencial {
	return &credencial{tokens: []string{tok}, probado: make([]bool, 1)}
}

// credConArchivo escribe un llavero en disco y lo carga por la variable de entorno.
func credConArchivo(t *testing.T, tokens ...string) (*credencial, string) {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(ruta, []byte(strings.Join(tokens, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envTokenFile, ruta)
	t.Setenv(envToken, "")
	c, err := cargarCredencial()
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("no cargó ninguna credencial")
	}
	return c, ruta
}

// EL ARCO COMPLETO DE UNA ROTACIÓN, DEL LADO DEL AGENTE.
//
// Es la prueba que faltaba y por eso el arco no funcionaba: el cerebro mandaba `token_nuevo` en la
// respuesta del latido y el struct del agente no tenía el campo, así que encoding/json lo tiraba
// en silencio. La rotación vencía siempre, y ABIERTO.md la describía como si funcionara.
//
// Sabotaje que la hace fallar: borrar el campo TokenNuevo del struct de la respuesta en latir().
func TestElAgenteGuardaElTokenNuevoYLoEstrena(t *testing.T) {
	c, ruta := credConArchivo(t, "viejo")

	if err := c.Sumar("nuevo"); err != nil {
		t.Fatalf("no se pudo sumar el token de la rotación: %v", err)
	}
	// PERSISTIDO ANTES DE ESTRENARSE: el archivo tiene los dos, y el que se va a usar es el nuevo.
	if got := tokensDeArchivo(leer(t, ruta)); len(got) != 2 || got[0] != "viejo" || got[1] != "nuevo" {
		t.Fatalf("el archivo quedó %v; se esperaba [viejo nuevo]", got)
	}
	if c.Actual() != "nuevo" {
		t.Errorf("el agente sigue presentando %q en vez del nuevo", c.Actual())
	}

	// Y cuando el nuevo funciona, el llavero se colapsa: el viejo ya no se presenta nunca más.
	if err := c.Funciono(); err != nil {
		t.Fatalf("no se pudo colapsar: %v", err)
	}
	if got := tokensDeArchivo(leer(t, ruta)); len(got) != 1 || got[0] != "nuevo" {
		t.Errorf("después de funcionar el archivo quedó %v; se esperaba sólo [nuevo]", got)
	}
}

// EL CORTE DESPUÉS DE QUE EL CEREBRO PROMOVIÓ ES EL ÚNICO QUE PODÍA DEJAR LA MÁQUINA AFUERA.
//
// El cerebro recibió el latido con el nuevo, promovió y MATÓ el viejo. La máquina se cortó antes de
// colapsar el archivo, así que vuelve con los dos y presentando el viejo, que ya no vale. Sin el
// fallback, eso es 401 eterno y una visita a la máquina — justo lo que la rotación existe para
// evitar. Con el llavero, se prueba el otro y se recupera sola.
//
// Sabotaje que la hace fallar: quitar el `if cred.Rechazado()` del case res.revocado del bucle, o
// hacer que Rechazado devuelva siempre false.
func TestTrasUnCorteElAgenteSeRecuperaConElOtroTokenDelLlavero(t *testing.T) {
	c, _ := credConArchivo(t, "viejo", "nuevo")

	if got := c.Usar(); got != "viejo" {
		t.Fatalf("arrancó con %q; el orden del archivo manda", got)
	}
	// 401 con el viejo: hay otro sin probar.
	if !c.Rechazado() {
		t.Fatal("con dos tokens en el archivo, un 401 no puede darse por baja del dispositivo")
	}
	if got := c.Usar(); got != "nuevo" {
		t.Fatalf("el fallback eligió %q en vez del nuevo", got)
	}
	// Y si el segundo también da 401, ESO sí es una baja: el kill-switch no se afloja.
	if c.Rechazado() {
		t.Error("con todos los tokens probados y rechazados, el agente tiene que darse de baja")
	}
}

// UN ARCHIVO DE UNA SOLA LÍNEA —lo que hay hoy en todas las máquinas— ES UN LLAVERO VÁLIDO.
//
// Sabotaje que la hace fallar: exigir un formato nuevo (JSON, dos líneas obligatorias) en
// tokensDeArchivo.
func TestElArchivoDeUnaLineaSigueSiendoValidoYSinSaltoFinalTambien(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "token")
	// SIN salto final, que es lo que deja un `printf '%s'` del instalador.
	if err := os.WriteFile(ruta, []byte("msb_solo_uno"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envTokenFile, ruta)
	c, err := cargarCredencial()
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || len(c.tokens) != 1 || c.tokens[0] != "msb_solo_uno" {
		t.Fatalf("no se leyó el token de una línea: %+v", c)
	}

	// Y AGREGAR NO PUEDE PEGAR LOS DOS EN UNA LÍNEA. Sin el salto que apendarToken mete, el
	// archivo quedaría con `msb_solo_unonuevo`: un token inválido Y sin el viejo, o sea la máquina
	// afuera con el archivo entero perdido.
	if err := c.Sumar("msb_el_nuevo"); err != nil {
		t.Fatal(err)
	}
	got := tokensDeArchivo(leer(t, ruta))
	if len(got) != 2 || got[0] != "msb_solo_uno" || got[1] != "msb_el_nuevo" {
		t.Errorf("el append pegó las líneas: %v", got)
	}
}

// SIN ARCHIVO NO SE ADOPTA NADA, Y SE DICE.
//
// Un agente con la credencial en una variable de entorno no puede reescribirla desde adentro del
// proceso. Adoptar el token nuevo sólo en memoria sería lo peor de los dos mundos: el cerebro
// promueve al recibirlo, mata el viejo, y el próximo arranque lee de la env un token que ya no
// vale. Fallar y decirlo deja la rotación sin completar, que es un no-evento.
//
// Sabotaje que la hace fallar: que Sumar devuelva nil cuando ruta == "".
func TestSinArchivoLaRotacionNoSeAdoptaEnSilencio(t *testing.T) {
	c := credDePrueba("el-de-la-env")
	err := c.Sumar("el-nuevo")
	if err == nil {
		t.Fatal("adoptó una rotación sin tener dónde persistirla: el próximo arranque quedaría afuera")
	}
	if !strings.Contains(err.Error(), envTokenFile) {
		t.Errorf("el error no dice qué falta (%s): %v", envTokenFile, err)
	}
	if c.Actual() != "el-de-la-env" {
		t.Errorf("cambió el token en uso a %q pese a no poder guardarlo", c.Actual())
	}
}

// COLAPSAR EL LLAVERO NO AFLOJA EL MODO DEL ARCHIVO.
//
// El archivo es una credencial en reposo: una instalación que lo dejó 0400 no puede terminar 0600
// después de la primera rotación. Y el colapso es justo el camino donde puede pasar sin que nadie
// lo note, porque reemplaza el archivo por un temporal —que nace 0600— en vez de escribir el que
// estaba. Se afirma sólo donde los bits existen; en Windows el permiso lo da la ACL del directorio
// y es del instalador, no de esto.
//
// Sabotaje que la hace fallar: quitar el os.Chmod de escribirTokens.
func TestColapsarElLlaveroNoAflojaElModoDelArchivo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NTFS no mapea los bits de modo POSIX; acá el permiso lo da la ACL del directorio")
	}
	c, ruta := credConArchivo(t, "viejo", "nuevo")
	if err := os.Chmod(ruta, 0o400); err != nil {
		t.Fatal(err)
	}
	// Se está usando el segundo y funcionó: el archivo queda con ése solo. Colapsar reemplaza por
	// rename, que necesita escribir el DIRECTORIO y no el archivo — así que un 0400 no lo frena.
	c.i = 1
	if err := c.Funciono(); err != nil {
		t.Fatalf("no se pudo colapsar un archivo 0400: %v", err)
	}
	if got := tokensDeArchivo(leer(t, ruta)); len(got) != 1 || got[0] != "nuevo" {
		t.Fatalf("el colapso dejó %v; se esperaba sólo [nuevo]", got)
	}
	fi, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o400 {
		t.Errorf("la credencial quedó %v; se esperaba 0400 — rotar no puede aflojar un permiso", got)
	}
}

// UN ARCHIVO QUE NO SE PUEDE ESCRIBIR LO DICE, EN VEZ DE PERDER LA ROTACIÓN EN SILENCIO.
//
// Apendear sí necesita escribir el archivo, así que un 0400 lo frena de verdad. Lo importante es
// que el error nombre la ruta: sin eso, quien opere ve una rotación que nunca se completa y no
// tiene por dónde empezar. Adoptar el token sólo en memoria sería peor —el cerebro promueve al
// recibirlo, mata el viejo, y el próximo arranque lee del disco uno que ya no vale.
//
// Sabotaje que la hace fallar: que Sumar se trague el error de apendarToken y devuelva nil.
func TestUnaCredencialNoEscribibleNoSeTragaLaRotacion(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("hace falta que los bits del inodo frenen la escritura: ni root ni una plataforma sin modo POSIX")
	}
	c, ruta := credConArchivo(t, "viejo")
	if err := os.Chmod(ruta, 0o400); err != nil {
		t.Fatal(err)
	}
	err := c.Sumar("nuevo")
	if err == nil {
		t.Fatal("dio por guardada una rotación en un archivo que no se puede escribir")
	}
	if !strings.Contains(err.Error(), ruta) {
		t.Errorf("el error no nombra la ruta y no hay por dónde empezar: %v", err)
	}
	if c.Actual() != "viejo" {
		t.Errorf("cambió el token en uso a %q pese a no haber podido guardarlo", c.Actual())
	}
}

// EL AGENTE LEE `token_nuevo` DE LA RESPUESTA DEL LATIDO, QUE ES EL DEFECTO QUE HABÍA.
//
// Ésta es LA prueba del arco, y su ausencia es la razón por la que la rotación no funcionaba: el
// cerebro mandaba el campo desde el primer día (fleet_http.go) y el struct con el que el agente
// decodifica la respuesta tenía sólo `muestra` y `comandos`. encoding/json descarta lo desconocido
// EN SILENCIO, así que no había error, ni log, ni nada: la rotación vencía siempre y ABIERTO.md la
// describía como si funcionara punta a punta.
//
// Las otras pruebas de este archivo ejercitan el llavero en aislamiento y ninguna habría atrapado
// eso — se verificó saboteándolo: quitar el campo del struct las deja TODAS en verde.
//
// Sabotaje que la hace fallar: borrar el campo TokenNuevo del struct de la respuesta en latir(), o
// no propagarlo a resultadoLatido.
func TestElLatidoTraeElTokenDeUnaRotacionEnCurso(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// La forma exacta que arma el cerebro en fleet_http.go.
		_, _ = w.Write([]byte(`{"muestra":"guardada","comandos":[],"token_nuevo":"msb_el_rotado"}`))
	}))
	defer ts.Close()

	// El inventario se anula por el mismo motivo que en las otras pruebas del latido: dejarlo
	// mirando lo que corra en el host haría el resultado dependiente de la máquina.
	anterior := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) { return nil, nil }
	t.Cleanup(func() { enumerarServicios = anterior })

	res := latir(ts.URL+"/fleet/heartbeat", "tok-viejo", nil)
	if !res.ok {
		t.Fatalf("el latido falló: %+v", res)
	}
	if res.tokenNuevo != "msb_el_rotado" {
		t.Fatalf("el agente no leyó el token de la rotación: %q — el cerebro lo manda y "+
			"encoding/json lo descarta en silencio si el campo no está en el struct", res.tokenNuevo)
	}
}

func leer(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
