package main

// Pruebas de `musubi shell` (S5b).

import (
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// SI LA TERMINAL NO SE PUEDE PREPARAR, NO SE ABRE NINGUNA SESIÓN.
//
// Lo destapó el e2e corriendo la CLI sin tty: abría la sesión, después fallaba en el modo crudo, y
// dejaba una sesión huérfana en el cerebro. Como sólo se permite una viva por persona y máquina
// (T7), el próximo intento durante los 15 minutos siguientes recibía esa sesión muerta en vez de
// una nueva — o sea que un error de entorno inutilizaba la máquina por un cuarto de hora.
//
// Sabotaje que la hace fallar: volver a abrir la sesión antes de preparar la terminal.
func TestSinTerminalDeVerdadNoSeAbreNingunaSesion(t *testing.T) {
	// Un cerebro que registra si alguien le pidió abrir algo. Si la CLI lo llama, falló el orden.
	llamado := false
	srv := servidorQueRegistra(&llamado)
	defer srv.Close()

	// El proceso de test no tiene /dev/tty, que es exactamente el caso que se está fijando.
	if _, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		t.Skip("este proceso SÍ tiene terminal de control; la prueba necesita un entorno sin tty")
	}

	err := sesionInteractiva(srv.URL, "un-token", "nas", "casa")
	if err == nil {
		t.Fatal("sin terminal de verdad tendría que fallar")
	}
	if !strings.Contains(err.Error(), "crudo") && !strings.Contains(err.Error(), "terminal") {
		t.Errorf("el error no explica el problema real: %v", err)
	}
	if llamado {
		t.Error("se abrió una sesión en el cerebro antes de comprobar que la terminal servía: queda huérfana y bloquea la máquina 15 minutos")
	}
}

// servidorQueRegistra es un cerebro de mentira que anota si alguien le pidió abrir una sesión.
func servidorQueRegistra(llamado *bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*llamado = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"session_id\":\"x\",\"vence\":\"nunca\"}"}]}}`))
	}))
}

// TODO RESULTADO LLEVA EL ID DEL COMANDO, POR CUALQUIER CAMINO.
//
// Olvidarlo no falla ruidosamente: el cerebro responde 403 («ese comando no es de esta máquina»),
// el agente logea y sigue, y el comando queda `entregado` PARA SIEMPRE — la bitácora nunca
// registra que terminó. Pasó de verdad con `musubi:shell` y sólo se vio en un e2e; leyendo el
// código no se nota, porque la función que devuelve el resultado está en otro archivo que el que
// lo inicializa.
//
// Sabotaje que la hace fallar: devolver un resultadoDeComando sin ComandoID en cualquiera de las
// ramas de `ejecutar`.
func TestTodoResultadoLlevaElIdDelComandoPorCualquierCamino(t *testing.T) {
	// Una shell que termina sola al instante: no queremos un proceso interactivo en una prueba.
	t.Setenv("SHELL", "/bin/true")
	// Un cerebro que rechaza todo, para que el bombeo vuelva enseguida.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "cerrada", http.StatusGone)
	}))
	defer srv.Close()

	casos := []struct {
		nombre string
		argv   []string
	}{
		{"comando común", []string{"echo", "hola"}},
		{"operación interna desconocida", []string{"musubi:inventada"}},
		{"shell mal formada", []string{"musubi:shell"}},
		{"shell que el cerebro rechaza", []string{"musubi:shell", "una-sesion", "24", "80"}},
	}
	for _, c := range casos {
		res := ejecutar(comandoRecibido{ID: "cmd-" + c.nombre, Argv: c.argv, TimeoutSeg: 5}, srv.URL, "tok")
		if res.ComandoID != "cmd-"+c.nombre {
			t.Errorf("%s: el resultado volvió con ComandoID=%q; sin él el cerebro responde 403 y el comando queda «entregado» para siempre",
				c.nombre, res.ComandoID)
		}
	}
}

// UNA SESIÓN CERRADA NO DEJA UN ZOMBIE.
//
// Matar el pty sin cosecharlo deja una entrada de proceso sin nadie que la recoja, por cada
// sesión. No consume CPU ni memoria y no se nota en semanas — hasta que la máquina se queda sin
// PIDs. Lo destapó un e2e: `pgrep script` devolvía `[script] <defunct>` después de cerrar, y a ojo
// parecía un pty que no había muerto.
//
// Sabotaje que la hace fallar: quitar el p.cmd.Wait() de ptyLocal.cerrar.
func TestCerrarElPtyNoDejaUnZombie(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el pty local sólo existe en unix")
	}
	t.Setenv("SHELL", "/bin/cat") // vive hasta que se le cierre la entrada
	pty, err := abrirPtyLocal(24, 80)
	if err != nil {
		t.Skipf("no se pudo abrir un pty (¿falta `script`?): %v", err)
	}
	pid := pty.cmd.Process.Pid
	pty.cerrar()

	// Tras cosechar, el estado del proceso está disponible y no queda entrada en la tabla.
	if pty.cmd.ProcessState == nil {
		t.Fatal("el proceso del pty no se cosechó: queda un zombie por cada sesión de shell")
	}
	// Y comprobado desde afuera, para no confiar sólo en la estructura de Go.
	if b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat"); err == nil {
		if strings.Contains(string(b), " Z ") {
			t.Errorf("el pid %d quedó en estado Z (zombie)", pid)
		}
	}
}
