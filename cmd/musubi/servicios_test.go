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
		if got := estadoDeWindows(s); got == fleet.EstadoCorriendo {
			t.Errorf("Windows %s se reporta como corriendo", s)
		}
	}
	// Y la contraparte, para que el arreglo no sea «nada corre nunca».
	if estadoDeSystemd("active", "running") != fleet.EstadoCorriendo {
		t.Error("un servicio activo y corriendo dejó de reportarse como corriendo")
	}
	if estadoDeWindows("Running") != fleet.EstadoCorriendo {
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
	csv := "\"Name\",\"Status\",\"StartType\"\n" +
		"\"SQL Server (MSSQLSERVER), instancia\",\"Running\",\"Automatic\"\n" +
		"\"Spooler\",\"Stopped\",\"Automatic\"\n" +
		"\"Manual y sano\",\"Stopped\",\"Manual\"\n" +
		"\"Manual y roto\",\"Cualquiera\",\"Manual\"\n"
	rs := parsearGetService(csv, time.Now())
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
	if nombres["Spooler"] != fleet.EstadoDetenido {
		t.Error("un servicio Automatic y detenido no se reportó: es la fila que uno quiere ver")
	}
	if _, hay := nombres["Manual y sano"]; hay {
		t.Error("se reportó un servicio Manual y detenido: Windows tiene cientos")
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
