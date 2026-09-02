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

	if svs := serviciosDelLatido(); svs != nil {
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
	if lista := serviciosDelLatido(); lista != nil {
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
