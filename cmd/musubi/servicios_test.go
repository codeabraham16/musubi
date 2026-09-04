package main

// servicios_test.go custodia la enumeración de servicios del agente (A42).
//
// El invariante que manda sobre todos los demás es el ORDEN, y no es obvio por qué: el cerebro
// PODA POR AUSENCIA. Lo que esta máquina deja de reportar se da de baja. Si dos latidos seguidos
// eligen distintos 64 de los mismos 80 servicios, la diferencia se da de baja y se vuelve a dar
// de alta cada pocos segundos — el inventario latiría y el panel mostraría bajas que no pasaron.

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
	"os"

	"path/filepath"
)

func repDe(nombre string, estado fleet.EstadoServicio) fleet.ReporteServicio {
	return fleet.ReporteServicio{Nombre: nombre, Clase: "systemd",
		Salud: fleet.SaludServicio{Tomada: time.Now(), Estado: estado}}
}

// TestElRecorteEsEstableYDejaAfueraLoQueANDA.
//
// Dos exigencias en una, porque van juntas: el orden tiene que ser DETERMINISTA (o la poda del
// cerebro convierte el inventario en una luz de navidad) y tiene que priorizar lo roto (o el
// recorte deja afuera justo lo que hay que mirar).
//
// Sabotaje que la hace fallar: sacar el desempate por nombre de serviciosParaElLatido, o invertir
// prioridadDeReporte para que `corriendo` gane.
func TestElRecorteEsEstableYDejaAfueraLoQueANDA(t *testing.T) {
	var crudos []fleet.ReporteServicio
	for i := 0; i < fleet.ServiciosPorLatido+20; i++ {
		crudos = append(crudos, repDe(fmt.Sprintf("bueno-%03d", i), fleet.EstadoCorriendo))
	}
	crudos = append(crudos, repDe("zzz-roto", fleet.EstadoFallado), repDe("zzz-parado", fleet.EstadoDetenido))

	lista, afuera := serviciosParaElLatido(crudos)
	if len(lista) != fleet.ServiciosPorLatido {
		t.Fatalf("se reportan %d y el techo es %d", len(lista), fleet.ServiciosPorLatido)
	}
	if afuera != 22 {
		t.Errorf("quedaron afuera %d y se esperaban 22: un recorte que no se cuenta es un recorte que nadie arregla", afuera)
	}
	// Lo roto primero, aunque su nombre sea el último del alfabeto.
	if lista[0].Nombre != "zzz-roto" {
		t.Errorf("el primero es %q y debería ser el FALLADO: si no entra todo, el que sobra tiene que ser el que anda bien", lista[0].Nombre)
	}
	if lista[1].Nombre != "zzz-parado" {
		t.Errorf("el segundo es %q y debería ser el detenido", lista[1].Nombre)
	}

	// ESTABILIDAD: con la MISMA entrada en otro orden, la salida tiene que ser idéntica.
	revuelto := make([]fleet.ReporteServicio, len(crudos))
	for i := range crudos {
		revuelto[i] = crudos[len(crudos)-1-i]
	}
	otra, _ := serviciosParaElLatido(revuelto)
	for i := range lista {
		if lista[i].Nombre != otra[i].Nombre {
			t.Fatalf("el recorte NO es estable: en la posición %d salió %q y después %q. Con la poda por ausencia del cerebro, eso da de baja y de alta los mismos servicios cada latido",
				i, lista[i].Nombre, otra[i].Nombre)
		}
	}
}

// TestUnNombreRepetidoNoSeReportaDosVeces.
//
// Pasa de verdad: `docker.service` en systemd y un contenedor llamado `docker`. El cerebro
// guardaría uno y el otro quedaría como ausente en la misma pasada — se daría de baja solo.
//
// Sabotaje que la hace fallar: quitar el mapa `vistos`.
func TestUnNombreRepetidoNoSeReportaDosVeces(t *testing.T) {
	lista, _ := serviciosParaElLatido([]fleet.ReporteServicio{
		repDe("docker", fleet.EstadoCorriendo),
		{Nombre: "Docker", Clase: "podman", Salud: fleet.SaludServicio{Tomada: time.Now(), Estado: fleet.EstadoCorriendo}},
	})
	if len(lista) != 1 {
		t.Errorf("se reportaron %d filas para el mismo nombre: la poda del cerebro daría de baja a una de ellas en el mismo latido", len(lista))
	}
}

// TestUnEstadoDeTransicionNoEsCorriendo — ni en systemd ni en Windows.
//
// Un servicio en `activating` lleva minutos sin arrancar; uno en `StartPending` igual. Llamarlos
// «corriendo» los esconde justo cuando hay que mirarlos.
//
// Sabotaje que la hace fallar: mandar el `default` de estadoDeSystemd a EstadoCorriendo.
func TestUnEstadoDeTransicionNoEsCorriendo(t *testing.T) {
	for _, c := range []struct{ activo, sub string }{
		{"activating", "start-pre"}, {"deactivating", "stop"}, {"reloading", "reload"},
	} {
		if got := estadoDeSystemd(c.activo, c.sub); got == fleet.EstadoCorriendo {
			t.Errorf("systemd %s/%s se reporta como corriendo", c.activo, c.sub)
		}
	}
	for _, s := range []string{"StartPending", "StopPending", "ContinuePending"} {
		if got := estadoDeWindows(s, "0"); got == fleet.EstadoCorriendo {
			t.Errorf("Windows %s se reporta como corriendo", s)
		}
	}
	// Y la contraparte, para que el arreglo no sea «nada corre nunca».
	if estadoDeSystemd("active", "running") != fleet.EstadoCorriendo {
		t.Error("un servicio activo y corriendo dejó de reportarse como corriendo")
	}
	if estadoDeWindows("Running", "0") != fleet.EstadoCorriendo {
		t.Error("un servicio Running de Windows dejó de reportarse como corriendo")
	}
}

// TestSoloSeReportaLoQueALGUIENDeclaróQueCorra, más lo roto.
//
// Una máquina tiene cientos de units. Reportar «las primeras 64» no informa nada; reportar lo
// habilitado más lo fallado es exactamente la pregunta «¿está corriendo lo que tiene que correr?».
//
// Sabotaje que la hace fallar: sacar el `if !habilitada && !fallada { continue }`.
func TestSoloSeReportaLoQueAlguienDeclaroQueCorra(t *testing.T) {
	salida := strings.Join([]string{
		"Id=importante.service\nActiveState=active\nSubState=running\nUnitFileState=enabled\nMainPID=42\nNRestarts=0\nResult=success",
		"Id=ruido.service\nActiveState=inactive\nSubState=dead\nUnitFileState=disabled\nMainPID=0\nNRestarts=0\nResult=success",
		"Id=roto.service\nActiveState=failed\nSubState=failed\nUnitFileState=disabled\nMainPID=0\nNRestarts=7\nResult=exit-code",
	}, "\n\n")
	rs := parsearSystemctlShow(salida, time.Now())

	nombres := map[string]fleet.ReporteServicio{}
	for _, r := range rs {
		nombres[r.Nombre] = r
	}
	if _, hay := nombres["importante"]; !hay {
		t.Error("no se reporta una unit habilitada y corriendo")
	}
	if _, hay := nombres["roto"]; !hay {
		t.Error("no se reporta una unit FALLADA sólo porque está deshabilitada: es justo la que hay que ver")
	}
	if _, hay := nombres["ruido"]; hay {
		t.Error("se reporta una unit deshabilitada e inactiva: hay cientos, y llenarían el techo de 64 con ruido")
	}
	// El detalle del roto lleva el POR QUÉ, que es la mitad del diagnóstico.
	if !strings.Contains(nombres["roto"].Salud.Detalle, "exit-code") {
		t.Errorf("el detalle de la unit fallada no dice por qué murió: %q", nombres["roto"].Salud.Detalle)
	}
	if nombres["roto"].Salud.Reinicios == nil || *nombres["roto"].Salud.Reinicios != 7 {
		t.Error("se perdió NRestarts: es lo que distingue «anda» de «anda a los tumbos»")
	}
	// Y el PID de algo detenido NO puede ser 0: tiene que ser nil.
	if p := nombres["roto"].Salud.PID; p != nil {
		t.Errorf("una unit detenida reportó pid %d: un 0 se lee como «pid 0», que además existe", *p)
	}
}

// TestUnaFechaQueNoSeEntiendeQuedaEnNilYNoEnLaEpoca.
//
// systemd manda `ActiveEnterTimestamp=` vacío para lo que nunca arrancó. Parsearlo a la época
// Unix diría «corriendo desde 1970», que es una afirmación falsa y encima verosímil en una tabla.
//
// Sabotaje que la hace fallar: devolver &time.Time{} en vez de nil.
func TestUnaFechaQueNoSeEntiendeQuedaEnNilYNoEnLaEpoca(t *testing.T) {
	for _, s := range []string{"", "n/a", "cualquier cosa"} {
		if got := fechaDeSystemd(s); got != nil {
			t.Errorf("fechaDeSystemd(%q) devolvió %v en vez de nil", s, *got)
		}
	}
	if got := fechaDeSystemd("Wed 2026-08-26 22:07:16 -04"); got == nil {
		t.Error("no se parseó una fecha válida de systemd")
	}
}

// TestElParserDeWindowsSeLeeDesdeLinux — la razón por la que los parsers salieron de detrás del
// build tag. Una salida con comas y acentos rompe cualquier partido por espacios.
//
// Sabotaje que la hace fallar: partir por comas a mano en vez de usar encoding/csv.
func TestElParserDeWindowsSeLeeDesdeLinux(t *testing.T) {
	csv := "\"Name\",\"State\",\"StartMode\",\"ExitCode\"\n" +
		"\"SQL Server (MSSQLSERVER), instancia\",\"Running\",\"Auto\",\"0\"\n" +
		"\"Spooler\",\"Stopped\",\"Auto\",\"1067\"\n" +
		"\"MapsBroker\",\"Stopped\",\"Auto\",\"0\"\n" +
		"\"Manual y sano\",\"Stopped\",\"Manual\",\"0\"\n" +
		"\"Manual nunca arrancado\",\"Stopped\",\"Manual\",\"1077\"\n"
	rs := parsearServiciosWindows(csv, time.Now())
	nombres := map[string]fleet.EstadoServicio{}
	for _, r := range rs {
		nombres[r.Nombre] = r.Salud.Estado
		if r.Salud.PID != nil || r.Salud.Reinicios != nil {
			t.Errorf("%s: el SCM no da pid ni reinicios por esta vía y se inventaron", r.Nombre)
		}
	}
	if _, hay := nombres["SQL Server (MSSQLSERVER), instancia"]; !hay {
		t.Errorf("un nombre con coma y paréntesis se cortó: %v", nombres)
	}
	if nombres["Spooler"] != fleet.EstadoFallado {
		t.Errorf("un Automatic detenido con ExitCode 1067 no se reportó fallado: %v", nombres["Spooler"])
	}
	if _, hay := nombres["Manual y sano"]; hay {
		t.Error("se reportó un servicio Manual y detenido limpio: Windows tiene cientos")
	}
	// LA FILA QUE COSTÓ 75 ALARMAS FALSAS. Un servicio Manual apagado reporta ExitCode=1077
	// —«nunca se intentó arrancar desde el boot»— que en un Automatic es una falla y en un Manual
	// es lo NORMAL. Windows trae cientos. Si el código de salida se mira ANTES de filtrar por
	// tipo de arranque, todos entran como `fallado` y el canal se llena.
	//
	// Sabotaje: mover el filtro de `auto` para DESPUÉS de calcular el estado → falla acá.
	if _, hay := nombres["Manual nunca arrancado"]; hay {
		t.Errorf("se reportó un Manual con ExitCode 1077: eso es lo normal en un Manual, y son cientos: %v", nombres)
	}
}

// EN WINDOWS, `Automatic` NO SIGNIFICA «tiene que estar corriendo» (A70).
//
// Es la diferencia con systemd, donde `enabled` sí lo significa, y creer que eran lo mismo llenó
// el canal de dieciséis alarmas falsas: `sppsvc`, `MapsBroker`, `edgeupdate` y los updaters son
// automáticos y se apagan solos cuando terminan su trabajo. Medido en `gio` el 2026-09-02: 8 de
// 102 automáticos detenidos, los ocho con ExitCode 0.
//
// Lo que los separa es un DATO y no una heurística sobre el tipo de arranque: el código de
// salida. Cero es «terminó bien»; 1067 es «se murió»; 1077 es «nunca arrancó desde el boot».
//
// Sabotaje: devolver EstadoDetenido en vez de EstadoOcioso para "stopped" → falla acá, y el
// exportador vuelve a emitir up=0 para servicios que están bien.
func TestUnAutomaticoQueSeApagoLimpioEsOciosoYNoCaido(t *testing.T) {
	if got := estadoDeWindows("Stopped", "0"); got != fleet.EstadoOcioso {
		t.Errorf("un servicio apagado con salida limpia se reporta %q: eso dispara ServicioCaido sin que nada esté mal", got)
	}
	if got := estadoDeWindows("Stopped", "1067"); got != fleet.EstadoFallado {
		t.Errorf("un servicio que MURIÓ se reporta %q: es justo el que hay que ver", got)
	}
	if got := estadoDeWindows("Stopped", "1077"); got != fleet.EstadoFallado {
		t.Errorf("un servicio que nunca arrancó desde el boot se reporta %q: debía arrancar y no lo hizo", got)
	}
	// Y EL CÓDIGO NO MANDA SOBRE UNO QUE ESTÁ CORRIENDO: un servicio vivo puede arrastrar el
	// ExitCode de una caída anterior de la que ya se recuperó, y mirarlo ahí lo dibujaría
	// fallado mientras funciona.
	if got := estadoDeWindows("Running", "1067"); got != fleet.EstadoCorriendo {
		t.Errorf("un servicio corriendo con un ExitCode viejo se reporta %q", got)
	}
	// OCIOSO NO CUENTA COMO CAÍDO para las políticas: si contara, un `servicio_caido` sobre una
	// máquina Windows dispararía acciones automáticas contra servicios que están perfectos.
	if fleet.EstadoCuentaComoCaido(fleet.EstadoOcioso) {
		t.Error("`ocioso` cuenta como caído: una política actuaría sobre un servicio que está bien")
	}
}

// TestUnaFuenteRotaNoSeLlevaAlLatido.
//
// Si enumerar falla, el agente late IGUAL y sin inventario. Un agente que se calla porque no pudo
// listar sus units deja a la máquina figurando muerta por un motivo que no tiene nada que ver.
//
// Sabotaje que la hace fallar: devolver el error desde serviciosDelLatido en vez de nil.
func TestUnaFuenteRotaNoSeLlevaAlLatido(t *testing.T) {
	anterior := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		return nil, errors.New("systemd no contesta")
	}
	t.Cleanup(func() { enumerarServicios = anterior })

	if svs, mandar, _ := serviciosDelLatido(); mandar || svs != nil {
		t.Errorf("con el enumerador roto se reportaron %d servicios", len(svs))
	}
}

// TestUnNombreInvalidoNoViajaYNoTumbaAlResto.
//
// El nombre lo produce la máquina y va a una columna que después se dibuja. Uno inválido se
// descarta SOLO, sin llevarse el lote — perder el inventario entero por una unit rara sería
// cambiar información parcial por ninguna.
func TestUnNombreInvalidoNoViajaYNoTumbaAlResto(t *testing.T) {
	lista, _ := serviciosParaElLatido([]fleet.ReporteServicio{
		repDe("", fleet.EstadoCorriendo),
		repDe(strings.Repeat("x", fleet.NombreServicioMax+50), fleet.EstadoCorriendo),
		repDe("sano", fleet.EstadoCorriendo),
	})
	if len(lista) == 0 {
		t.Fatal("un nombre inválido se llevó puesto el lote entero")
	}
	for _, r := range lista {
		if !fleet.NombreDeServicioValido(r.Nombre) {
			t.Errorf("viajó un nombre inválido: %q", r.Nombre)
		}
	}
}

// UNA FUENTE QUE NO ESTÁ Y UNA FUENTE ROTA NO SON LO MISMO, Y LA DIFERENCIA ES DESTRUCTIVA.
//
// La versión anterior las mezclaba: cualquier falla de `podman ps` era un `continue`, con el
// razonamiento de que no tener docker instalado es lo normal. Pero el cerebro PODA POR AUSENCIA
// —la lista no dice «encontré esto», dice «esto es lo que corre acá»—, así que saltear una fuente
// rota no manda menos información: manda la afirmación de que sus servicios dejaron de existir.
// En el servidor real eso dio de baja 18 contenedores, y como no hay error en ningún lado, el
// síntoma fue que 18 filas desaparecieron y nadie supo por qué.
//
// Los tres desenlaces tienen que quedar separados:
//
//	no está          → se saltea, sin ruido, y el inventario sigue siendo completo
//	está y falló     → error, y el llamador NO manda inventario (no mandar no borra nada)
//	anduvo           → su salida
//
// Sabotaje que la hace fallar: devolver `("", false, nil)` en el caso `default` de enumerarFuente,
// que es exactamente el `continue` de antes.
func TestUnaFuenteRotaNoSeConfundeConUnaQueNoEstaInstalada(t *testing.T) {
	original := ejecutarParaEnumerar
	t.Cleanup(func() { ejecutarParaEnumerar = original })

	casos := []struct {
		nombre  string
		err     error
		hay     bool
		esperaE bool
	}{
		{"no está instalada", &exec.Error{Name: "docker", Err: exec.ErrNotFound}, false, false},
		{"está y falló", errors.New("permission denied"), true, true},
		{"está y salió con código", &exec.ExitError{}, true, true},
		{"anduvo", nil, true, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			ejecutarParaEnumerar = func(string, ...string) ([]byte, error) {
				return []byte("una-salida"), c.err
			}
			salida, hay, err := enumerarFuente("podman", "ps")
			if hay != c.hay {
				t.Errorf("hayFuente=%v, se esperaba %v", hay, c.hay)
			}
			if (err != nil) != c.esperaE {
				t.Errorf("err=%v, se esperaba error=%v", err, c.esperaE)
			}
			if c.esperaE && !strings.Contains(err.Error(), "podman") {
				t.Errorf("el error no nombra la fuente que falló: %v", err)
			}
			if !c.esperaE && !c.hay && salida != "" {
				t.Errorf("una fuente ausente devolvió salida: %q", salida)
			}
		})
	}
}

// Y LA CONSECUENCIA, QUE ES DISTINTA DE TestUnaFuenteRotaNoSeLlevaAlLatido.
//
// Aquélla mira el caso en que no quedó NADA (lista nil y error) y exige que el agente lata igual.
// Ésta mira el que de verdad hizo daño: quedó ALGO —systemd anduvo, podman no— y hay error. La
// tentación acá es aprovechar lo que se juntó, y es justo lo que no se puede: la poda del cerebro
// leería esa lista como «los contenedores ya no están». Nil no es una pérdida, es la única
// respuesta honesta, y no borra nada porque la poda sólo corre cuando llega una lista.
//
// Sabotaje que la hace fallar: en serviciosDelLatido, devolver `lista` en vez de `nil` cuando
// enumerarServicios da error.
func TestUnaListaParcialConErrorNoViajaAlCerebro(t *testing.T) {
	original := enumerarServicios
	t.Cleanup(func() { enumerarServicios = original })
	ultimoInventario.Lock()
	ultimoInventario.huella, ultimoInventario.enviado = "", time.Time{}
	ultimoInventario.Unlock()

	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		return []fleet.ReporteServicio{repDe("sshd", fleet.EstadoCorriendo)},
			fmt.Errorf("podman está instalado y no se pudo consultar: %w", errors.New("permission denied"))
	}
	if lista, mandar, _ := serviciosDelLatido(); mandar || lista != nil {
		t.Fatalf("con la enumeración rota el latido llevó %d servicios: esa lista da de baja lo que no trae: %+v", len(lista), lista)
	}
}

// LOS REINICIOS DE UN CONTENEDOR SE REPORTAN, Y SU AUSENCIA NO ES UN CERO.
//
// Encontrado auditando las 25 reglas de alerta contra el Prometheus real: 54 servicios con serie
// `up` y sólo 36 con serie de reinicios. Los 18 que faltaban eran los contenedores — o sea que
// `ServicioReiniciandose` estaba ciega justo para las cosas que se reinician solas. Un contenedor
// con `restart: always` en bucle de caída es EL caso para el que existe esa alerta.
//
// La otra mitad es la que se rompe callada: un runtime que no conoce `{{.Restarts}}` imprime
// `<no value>` o directamente no manda el campo. Parsear eso como 0 no deja un hueco, deja la
// afirmación «este contenedor no se reinició nunca» — que apaga la alerta con confianza.
//
// Sabotaje que la hace fallar: devolver `0, true` cuando el campo falta o no es un número.
func TestLosReiniciosDeUnContenedorViajanYSuAusenciaNoEsCero(t *testing.T) {
	casos := []struct {
		nombre    string
		linea     string
		reinicios *int
	}{
		{"podman con el campo", "vaultwarden\trunning\tUp 17 hours\t3", intPtr(3)},
		{"cero medido es cero", "supabase-db\trunning\tUp 2 weeks (healthy)\t0", intPtr(0)},
		{"docker sin el campo", "nginx\trunning\tUp 3 days", nil},
		{"el literal de un template que no entendió", "raro\trunning\tUp 1 hour\t<no value>", nil},
		{"basura en el campo", "raro2\trunning\tUp 1 hour\tmuchas", nil},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			rs := parsearContenedores(c.linea, "podman", time.Now())
			if len(rs) != 1 {
				t.Fatalf("se parsearon %d reportes", len(rs))
			}
			got := rs[0].Salud.Reinicios
			switch {
			case c.reinicios == nil && got != nil:
				t.Errorf("se inventó un contador de reinicios: %d. Un runtime que no lo sabe tiene "+
					"que dejar un hueco, no afirmar que no se reinició nunca", *got)
			case c.reinicios != nil && got == nil:
				t.Error("el contador de reinicios no viajó: la alerta de reinicios queda ciega")
			case c.reinicios != nil && *got != *c.reinicios:
				t.Errorf("reinicios = %d, se esperaba %d", *got, *c.reinicios)
			}
		})
	}
}

// EL FORMATO SE DEGRADA ANTES DE DARSE POR VENCIDO.
//
// `{{.Restarts}}` no lo entiende `docker ps` ni un podman viejo. Con la regla de que una fuente
// que ESTÁ y falla aborta el inventario entero, pedirlo a secas convertiría «este docker no
// conoce ese campo» en «esta máquina no reporta ningún servicio» — una regresión mucho peor que
// el hueco que se venía a tapar.
//
// Sabotaje que la hace fallar: quedarse con un solo formato, o devolver el error del primer
// intento en vez de seguir con el siguiente.
func TestSiElRuntimeNoEntiendeElFormatoRicoSeUsaElPobre(t *testing.T) {
	original := ejecutarParaEnumerar
	t.Cleanup(func() { ejecutarParaEnumerar = original })

	var pedidos []string
	ejecutarParaEnumerar = func(nombre string, args ...string) ([]byte, error) {
		formato := args[len(args)-1]
		pedidos = append(pedidos, formato)
		if strings.Contains(formato, "Restarts") {
			return nil, errors.New(`template: ps:1:13: can't evaluate field Restarts`)
		}
		return []byte("nginx\trunning\tUp 3 days\n"), nil
	}

	salida, hay, err := contenedoresDe("docker")
	if err != nil {
		t.Fatalf("un runtime que no conoce el campo tumbó la enumeración: %v", err)
	}
	if !hay {
		t.Fatal("el runtime está y se reportó como ausente")
	}
	if !strings.Contains(salida, "nginx") {
		t.Errorf("no se cayó al formato pobre: %q", salida)
	}
	if len(pedidos) != 2 {
		t.Fatalf("se intentaron %d formatos, se esperaban 2: %v", len(pedidos), pedidos)
	}
	if !strings.Contains(pedidos[0], "Restarts") || strings.Contains(pedidos[1], "Restarts") {
		t.Errorf("el orden de los formatos no es rico→pobre: %v", pedidos)
	}

	// Y si NINGUNO anda, sí es una falla: el runtime está y no se pudo consultar.
	ejecutarParaEnumerar = func(string, ...string) ([]byte, error) {
		return nil, errors.New("permission denied")
	}
	if _, hay, err := contenedoresDe("podman"); err == nil || !hay {
		t.Errorf("un runtime presente y roto no dio error: hay=%v err=%v", hay, err)
	}
}

func intPtr(n int) *int { return &n }

// TestElParserDeMacosSeLeeDesdeLinux es el hermano que faltaba del de Windows, y su ausencia se
// notó de la peor manera: `parsearLaunchctl` era LO ÚNICO del paquete que ningún archivo llamaba
// desde Linux, así que el linter lo denunció como código muerto la primera vez que corrió sobre
// esta rama. No estaba muerto —lo llama servicios_darwin.go— pero SÍ estaba sin probar, que es el
// hallazgo verdadero: A1/A3 dicen que macOS nunca corrió en hardware, y encima su parser era el
// único de los tres sin una prueba que lo ejerciera.
//
// Los parsers viven fuera de los build tags a propósito, justamente para poder hacer esto desde
// cualquier máquina. El de Windows ya lo aprovechaba; éste no.
func TestElParserDeMacosSeLeeDesdeLinux(t *testing.T) {
	// Formato real de `launchctl list`: PID, último código de salida, etiqueta. Un servicio que no
	// corre trae "-" en el PID, que NO es un número y por eso queda detenido.
	salida := "PID\tStatus\tLabel\n" +
		"431\t0\tcom.ejemplo.corriendo\n" +
		"-\t0\tcom.ejemplo.detenido\n" +
		"-\t78\tcom.ejemplo.fallado\n" +
		"912\t0\tcom.apple.mdworker\n" +
		"basura\n" +
		"\n"
	rs := parsearLaunchctl(salida, time.Now())

	porNombre := map[string]fleet.ReporteServicio{}
	for _, r := range rs {
		porNombre[r.Nombre] = r
		if r.Clase != "launchd" {
			t.Errorf("%s: clase %q, esperaba launchd", r.Nombre, r.Clase)
		}
	}

	if _, hay := porNombre["Label"]; hay {
		t.Error("el encabezado de launchctl entró como si fuera un servicio")
	}
	// Una línea de menos de tres campos no puede leerse: tomarla igual convertiría basura en un
	// servicio con nombre inventado.
	if len(rs) != 3 {
		t.Errorf("esperaba 3 servicios (encabezado, com.apple.* y la línea corta afuera), obtuve %d: %v", len(rs), porNombre)
	}

	// LO QUE MÁS IMPORTA DEL FILTRO: macOS trae CIENTOS de `com.apple.*`. Sin el descarte, el
	// inventario de una Mac sería casi entero ruido del sistema y taparía lo que alguien declaró.
	if _, hay := porNombre["com.apple.mdworker"]; hay {
		t.Error("se reportó un servicio com.apple.*: macOS trae cientos y ahogan el inventario")
	}

	if r := porNombre["com.ejemplo.corriendo"]; r.Salud.Estado != fleet.EstadoCorriendo {
		t.Errorf("un PID numérico > 0 no se leyó como corriendo: %v", r.Salud.Estado)
	} else if r.Salud.PID == nil || *r.Salud.PID != 431 {
		t.Errorf("el PID no se guardó: %v", r.Salud.PID)
	}

	// UN PID "-" NO ES UN CERO NI UN ERROR: es «no corre». Es el mismo invariante que gobierna el
	// track entero —lo que no se pudo medir no se inventa— aplicado al caso más común de launchctl.
	if r := porNombre["com.ejemplo.detenido"]; r.Salud.Estado != fleet.EstadoDetenido {
		t.Errorf("un PID «-» con salida 0 no se leyó como detenido: %v", r.Salud.Estado)
	} else if r.Salud.PID != nil {
		t.Errorf("se inventó un PID para un servicio que no corre: %v", *r.Salud.PID)
	}

	// Y el código de salida distinto de cero manda sobre el estado: no corre Y terminó mal.
	if r := porNombre["com.ejemplo.fallado"]; r.Salud.Estado != fleet.EstadoFallado {
		t.Errorf("una salida 78 no se leyó como fallado: %v", r.Salud.Estado)
	} else if r.Salud.Detalle == "" {
		t.Error("un servicio fallado no dijo con qué código: el operador queda sin el dato que decide qué mirar")
	}
}

// EL CÓDIGO DE SALIDA VIAJA CON EL SERVICIO FALLADO, y no sólo su veredicto.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ IMPORTA EL NÚMERO Y NO ALCANZA CON `fallado`
//
// Los dos códigos frecuentes significan cosas OPUESTAS para quien tiene que arreglarlo: 1067 es
// «el proceso terminó de forma inesperada» —arrancó y se murió— y 1077 es «no se intentó
// arrancarlo desde el último arranque», que en una máquina que viene de apagones sucios habla
// del arranque y no del servicio. Con `fallado` a secas hay que ir a la máquina para
// distinguirlas, y en `davantis-1` ir a la máquina es justamente lo caro.
//
// Windows era la ÚNICA de las cuatro plataformas que lo tiraba: systemd manda su `Result=`,
// launchctl su `salida=` y los contenedores su estado. Misma familia que A83 — tres caminos lo
// hacían y el cuarto no.
//
// Sabotaje: devolver "" siempre en detalleDeWindows, o publicar el código también cuando el
// servicio está corriendo (arrastra el de una caída anterior de la que ya se recuperó).
func TestElCodigoDeSalidaDeWindowsViajaConElServicioFallado(t *testing.T) {
	csv := "\"Name\",\"State\",\"StartMode\",\"ExitCode\"\n" +
		"\"murio\",\"Stopped\",\"Auto\",\"1067\"\n" +
		"\"nunca-arranco\",\"Stopped\",\"Auto\",\"1077\"\n" +
		"\"ocioso\",\"Stopped\",\"Auto\",\"0\"\n" +
		"\"sano-con-cicatriz\",\"Running\",\"Auto\",\"1067\"\n"
	por := map[string]fleet.ReporteServicio{}
	for _, r := range parsearServiciosWindows(csv, time.Now()) {
		por[r.Nombre] = r
	}
	if len(por) != 4 {
		t.Fatalf("se esperaban 4 servicios, hay %d", len(por))
	}

	// Los dos fallados llevan SU número, y son distinguibles entre sí.
	if d := por["murio"].Salud.Detalle; d != "salida=1067" {
		t.Errorf("un servicio que murió no dice por qué: detalle = %q", d)
	}
	if d := por["nunca-arranco"].Salud.Detalle; d != "salida=1077" {
		t.Errorf("un servicio que nunca arrancó no dice por qué: detalle = %q", d)
	}
	if por["murio"].Salud.Detalle == por["nunca-arranco"].Salud.Detalle {
		t.Error("dos fallas de causa opuesta llegan con el mismo detalle: el operador no las puede separar")
	}

	// Y NO SE PUBLICA DONDE NO APORTA. Un `0` no distingue nada, y en un servicio CORRIENDO el
	// código es la cicatriz de una caída anterior: mostrarlo pondría un número alarmante al lado
	// de algo sano.
	if d := por["ocioso"].Salud.Detalle; d != "" {
		t.Errorf("un servicio ocioso con salida limpia trae detalle: %q", d)
	}
	if d := por["sano-con-cicatriz"].Salud.Detalle; d != "" {
		t.Errorf("un servicio CORRIENDO publica el código de una caída vieja: %q — se lee como si estuviera roto ahora", d)
	}
	// Control: el veredicto sigue siendo el de antes. Sin esto, un cambio que rompiera los
	// estados pasaría mientras el detalle esté bien.
	if por["murio"].Salud.Estado != fleet.EstadoFallado || por["ocioso"].Salud.Estado != fleet.EstadoOcioso {
		t.Errorf("cambiaron los estados: murio=%v ocioso=%v", por["murio"].Salud.Estado, por["ocioso"].Salud.Estado)
	}
}

// LAS CUATRO PLATAFORMAS DICEN POR QUÉ FALLÓ UN SERVICIO, NO SÓLO QUE FALLÓ.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ESTA PRUEBA ES DE LA CLASE, Y LA DE ARRIBA ES DEL CASO
//
// El defecto que cerró TestElCodigoDeSalidaDeWindowsViajaConElServicioFallado tenía la forma de
// A83: TRES caminos hacían algo y el CUARTO no. Arreglar el cuarto no impide que aparezca un
// quinto sin `Detalle`, ni que uno de los otros tres lo pierda en un refactor — y ninguna de las
// dos cosas rompe nada visible, porque un `Detalle` vacío es exactamente lo que devuelve una
// plataforma que de verdad no sabe el motivo. Un hueco y una ausencia legítima se ven igual.
//
// Así que se recorren los cuatro con una muestra de FALLO real de cada formato, y cada uno tiene
// que traer su diagnóstico. Lo que dice cada uno es distinto y está bien que lo sea: systemd manda
// `Result=`, Windows el código de salida, launchctl el último exit, y un contenedor su `Status`
// legible. Lo que NO puede pasar es que alguno mande vacío.
//
// SE PUEDE HACER PORQUE LOS CUATRO PARSERS VIVEN FUERA DE LOS BUILD TAGS, que es todo el punto de
// ese reparto: dos de las cuatro plataformas no se pueden correr desde acá, así que una prueba de
// tabla como ésta es la única forma de cubrirlas juntas. Con los parsers detrás de su tag, esta
// guarda no se podría escribir.
//
// Sabotaje que la hace fallar: devolver "" en detalleDeSystemd, detalleDeWindows,
// detalleDeContenedor, o quitar el `salida=` de parsearLaunchctl. Cada uno rompe su propia fila.
func TestLasCuatroPlataformasDicenPorQueFalloUnServicio(t *testing.T) {
	ahora := time.Now()
	for _, c := range []struct {
		plataforma string
		servicio   string
		reportes   []fleet.ReporteServicio
		esperaEn   string // un fragmento que el detalle TIENE que contener
	}{
		{
			plataforma: "systemd",
			servicio:   "roto",
			reportes: parsearSystemctlShow(
				"Id=roto.service\nActiveState=failed\nSubState=failed\nUnitFileState=enabled\n"+
					"MainPID=0\nNRestarts=7\nResult=exit-code", ahora),
			esperaEn: "exit-code",
		},
		{
			plataforma: "windows",
			servicio:   "roto",
			reportes: parsearServiciosWindows("\"Name\",\"State\",\"StartMode\",\"ExitCode\"\n"+
				"\"roto\",\"Stopped\",\"Auto\",\"1067\"\n", ahora),
			esperaEn: "1067",
		},
		{
			plataforma: "launchd",
			servicio:   "com.ejemplo.roto",
			reportes:   parsearLaunchctl("PID\tStatus\tLabel\n-\t78\tcom.ejemplo.roto\n", ahora),
			esperaEn:   "78",
		},
		{
			plataforma: "contenedor",
			servicio:   "roto",
			reportes:   parsearContenedores("roto\tdead\tExited (137) 2 minutes ago\t3", "podman", ahora),
			esperaEn:   "137",
		},
	} {
		t.Run(c.plataforma, func(t *testing.T) {
			// CONTROL DE QUE LA MUESTRA LLEGÓ: sin esto, un parser que dejara de reconocer su
			// propio formato daría cero reportes y la prueba pasaría sin haber mirado un detalle.
			if len(c.reportes) != 1 {
				t.Fatalf("la muestra de %s produjo %d reportes y no 1: el parser no reconoció su "+
					"propio formato, así que esta fila no está midiendo nada", c.plataforma, len(c.reportes))
			}
			r := c.reportes[0]
			if r.Nombre != c.servicio {
				t.Fatalf("%s: el reporte es de %q y no de %q", c.plataforma, r.Nombre, c.servicio)
			}
			if r.Salud.Estado == fleet.EstadoCorriendo {
				t.Fatalf("%s: la muestra de FALLO se leyó como corriendo (%q): esta fila prueba el "+
					"detalle de un servicio fallado, no de uno sano", c.plataforma, r.Salud.Estado)
			}
			if strings.TrimSpace(r.Salud.Detalle) == "" {
				t.Errorf("%s: un servicio fallado no dice POR QUÉ. `fallado` a secas obliga a ir a "+
					"la máquina, y las otras tres plataformas sí lo dicen — un detalle vacío es "+
					"indistinguible de una plataforma que de verdad no sabe el motivo", c.plataforma)
			}
			if !strings.Contains(r.Salud.Detalle, c.esperaEn) {
				t.Errorf("%s: el detalle %q no trae el dato que lo hace accionable (%q)",
					c.plataforma, r.Salud.Detalle, c.esperaEn)
			}
		})
	}
}

// TODA PLATAFORMA QUE ENUMERA SERVICIOS ENUMERA TAMBIÉN SUS CONTENEDORES.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// A76, Y POR QUÉ ESTA GUARDA MIRA EL TEXTO
//
// El bloque que enumera contenedores vivía ADENTRO del enumerador de Linux, así que sumar una
// plataforma exigía acordarse de copiarlo — y nadie se acordó. Medido el 2026-09-02:
// `musubi-server` reportaba 57 servicios de los cuales 14 eran contenedores; `davantis-1`
// reportaba 64 y NINGUNO, con once de Docker Desktop corriendo. Dos estaban rotos
// —`supabase_vector` en bucle de reinicio hacía días, `edge-runtime` muerto hacía tres con código
// 255— y se encontraron A MANO, buscando espacio en disco. Ninguna alerta falló: la serie no
// existía, así que no había nada que pudiera ponerse rojo.
//
// LOS ENUMERADORES VIVEN DETRÁS DE BUILD TAGS y el compilador no puede ayudar acá: desde Linux
// sólo se compila `servicios_linux.go`, así que ni `go vet` ni una prueba de comportamiento ven a
// los otros dos. Leer el texto es el único amarre disponible, igual que con el instalador de
// Windows y el colector del relay. Es pobre y es el que hay.
//
// FALLA CERRADA ANTE UN ARCHIVO NUEVO: la lista no está escrita a mano, se descubre por glob. Una
// plataforma que aparezca mañana con su `enumerarServiciosDelSistema` entra vigilada por default,
// que es lo contrario de una lista de las que sí hay que revisar — ahí la quinta nacería sin
// guarda y en silencio.
//
// Sabotaje que la hace fallar: sacar la llamada a `enumerarContenedores` de cualquiera de los tres.
func TestTodaPlataformaQueEnumeraServiciosEnumeraSusContenedores(t *testing.T) {
	archivos, err := filepath.Glob("servicios_*.go")
	if err != nil {
		t.Fatal(err)
	}
	mirados := 0
	for _, f := range archivos {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		// Sólo los archivos que DEFINEN el enumerador: los de parseo y el de contenedores no lo
		// hacen, y exigirles la llamada sería ruido.
		if !strings.Contains(src, "func enumerarServiciosDelSistema(") {
			continue
		}
		mirados++
		// NO HAY EXCEPCIONES, y eso salió de escribir esta prueba. La primera versión eximía al
		// stub de «el resto de los sistemas» —parecía razonable: no tiene colector de servicios—
		// y la guarda lo marcó igual. Tenía razón: `docker` y `podman` no son de un sistema
		// operativo, y `contenedoresDe` ya trata «la herramienta no está» como `hay == false`. La
		// excepción habría dejado la misma trampa de A76 un escalón más abajo, y de paso obligaba
		// a reconocer el stub por su texto, que es un heurístico que envejece.
		if !strings.Contains(src, "enumerarContenedores(") {
			t.Errorf("%s define enumerarServiciosDelSistema y NO llama a enumerarContenedores.\n"+
				"  `docker` corre en las tres plataformas, así que sus contenedores son parte de "+
				"«qué corre acá». Sin esto la serie no existe para esa máquina, y una serie que no "+
				"existe no puede poner ninguna alerta en rojo — que es exactamente cómo dos "+
				"contenedores rotos sobrevivieron días en davantis-1.", f)
		}
	}
	// CONTROL DE QUE MIRÓ ALGO: si el glob o el nombre de la función cambiaran, el bucle no
	// iteraría y esta prueba pasaría en verde sin haber abierto un solo enumerador.
	if mirados < 4 {
		t.Fatalf("se encontraron %d enumeradores de plataforma y son al menos 4 (linux, windows, "+
			"darwin y el de «el resto»): cambió la forma del paquete y esta guarda dejó de mirar", mirados)
	}
}

// UN SERVICIO VIVO NO SE DIBUJA `fallado` POR UNA CAÍDA DE LA QUE YA SE RECUPERÓ.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA REGLA YA ESTABA ESCRITA, EN LA PLATAFORMA DE AL LADO
//
// `estadoDeWindows` la dice en su propio comentario: «el código de salida manda sobre el estado, y
// SÓLO cuando no está corriendo». launchd era la única de las cuatro plataformas que no la
// aplicaba: el `if` del código pisaba, sin condición, el `corriendo` que acababa de poner el PID.
//
// La segunda columna de `launchctl list` es el ÚLTIMO estado de salida, no el actual. launchd
// reinicia lo que tiene `KeepAlive`, así que un servicio que se cayó una vez, volvió solo y hoy
// tiene PID sigue mostrando ese código PARA SIEMPRE. Quedaba `fallado` mientras funcionaba, y de
// ahí salía un `musubi_fleet_service_up 0` sobre algo vivo: la falla más cara de un monitor, que
// es la que reporta una caída donde no la hubo.
//
// EL NÚMERO NO SE PIERDE: viaja en Detalle también cuando está vivo, porque «se recuperó de una
// salida 78» es justo lo que uno quiere ver al lado de un servicio que anda a los tumbos. Lo que
// cambia es el VEREDICTO, no el dato.
//
// Sabotaje: sacar el `if !vivo` de parsearLaunchctl → com.ejemplo.revivido vuelve a `fallado`.
func TestLaSalidaViejaNoMataUnServicioVivoEnMacos(t *testing.T) {
	// Formato real: PID, último código de salida, etiqueta. La fila del medio es el caso: tiene
	// PID (está corriendo AHORA) y arrastra el código de una caída anterior.
	salida := "PID\tStatus\tLabel\n" +
		"431\t0\tcom.ejemplo.sano\n" +
		"512\t78\tcom.ejemplo.revivido\n" +
		"-\t78\tcom.ejemplo.muerto\n"
	porNombre := map[string]fleet.ReporteServicio{}
	for _, r := range parsearLaunchctl(salida, time.Now()) {
		porNombre[r.Nombre] = r
	}

	revivido, hay := porNombre["com.ejemplo.revivido"]
	if !hay {
		t.Fatal("no se parseó com.ejemplo.revivido: la prueba no probaría nada")
	}
	if revivido.Salud.Estado != fleet.EstadoCorriendo {
		t.Errorf("un servicio CON PID quedó %q por un código de salida viejo: eso se exporta como service_up 0 y ServicioCaido anuncia una caída sobre algo que está corriendo",
			revivido.Salud.Estado)
	}
	if revivido.Salud.PID == nil || *revivido.Salud.PID != 512 {
		t.Errorf("el PID no sobrevivió: %v", revivido.Salud.PID)
	}
	// El número sigue estando; lo que cambió es el veredicto, no el dato.
	if revivido.Salud.Detalle != "salida=78" {
		t.Errorf("se perdió el detalle de la caída anterior: %q — «se recuperó de una salida 78» es lo que hace accionable a un servicio que anda a los tumbos", revivido.Salud.Detalle)
	}

	// Y LO QUE NO PUEDE CAMBIAR: apagado, el código SÍ manda. Sin este control, «no mires nunca
	// el código de salida» pasaría el test de arriba y perdería la única señal de que murió.
	muerto := porNombre["com.ejemplo.muerto"]
	if muerto.Salud.Estado != fleet.EstadoFallado {
		t.Errorf("un servicio SIN PID y con salida 78 quedó %q, esperaba fallado: apagado, el código es la pregunta que importa", muerto.Salud.Estado)
	}
	if sano := porNombre["com.ejemplo.sano"]; sano.Salud.Estado != fleet.EstadoCorriendo || sano.Salud.Detalle != "" {
		t.Errorf("el servicio sano quedó %q con detalle %q", sano.Salud.Estado, sano.Salud.Detalle)
	}
}
