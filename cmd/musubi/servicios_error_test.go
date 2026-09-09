package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// «NO PUDE ENUMERAR» TIENE QUE VIAJAR, Y JUSTO CUANDO NO VIAJA EL INVENTARIO.
//
// EL DEFECTO, MEDIDO EL 2026-09-09 EN `davantis-1`.
//
// Cuando `enumerarServicios` falla, `serviciosDelLatido` devuelve `mandar=false` y el inventario NO
// se manda. Eso está bien y es deliberado: media lista haría que el cerebro pode lo que no vino.
// Pero del lado del cerebro ese silencio era IDÉNTICO al de un inventario que no cambió, así que
// una máquina con el enumerador roto se veía exactamente igual que una sana y estable.
//
// Lo medido: 64 alertas `ServicioSinNoticias`, todas de la misma máquina, 61 horas sin reportar un
// solo servicio — con el agente VIVO (último latido hacía 4,8 s) y mandando CPU 49,7 % y uptime sin
// problema. Sesenta y cuatro alertas para UNA causa, y ninguna de las 64 la nombra. Eso no es una
// alarma: es ruido que enseña a ignorar el canal, la misma lección que dejaron los trece
// `MaquinaCaida` de A79.
//
// EL ERROR FÁCIL DE COMETER ACÁ ES PONER LA ASIGNACIÓN ADENTRO DEL `if mandarInventario`, donde
// vive su vecina `ServiciosOmitidos`. Leído por encima parece el lugar natural —los dos hablan del
// inventario— y sería exactamente al revés: el único caso en que hay algo que decir es cuando NO se
// manda nada. Adentro del `if`, el campo no se emitiría NUNCA en el caso que importa, y la prueba
// de que el defecto es fácil es que su vecina correcta está tres líneas más arriba.
//
// Esta guarda ejercita el comportamiento —hace fallar la enumeración de verdad y mira qué queda en
// el sobre— en vez de leer el archivo buscando un texto. Un `strings.Contains` sobre agent.go no
// puede distinguir «está afuera del if» de «está adentro».
func TestElFalloDeEnumeracionViajaAunqueElInventarioNoViaje(t *testing.T) {
	anterior := enumerarServicios
	t.Cleanup(func() {
		enumerarServicios = anterior
		olvidarEnumeracion()
	})

	// 1 · CON LA ENUMERACIÓN ROTA: no hay inventario que mandar, y SÍ hay motivo.
	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		return nil, errors.New("no se pudo hablar con el SCM: acceso denegado")
	}
	olvidarEnumeracion()

	lista, _, mandar, _ := serviciosDelLatido()
	if mandar || lista != nil {
		t.Fatalf("con la enumeración rota no hay nada que mandar; devolvió mandar=%v lista=%v", mandar, lista)
	}
	motivo := motivoDeEnumeracionFallida()
	if motivo == "" {
		t.Fatal("la enumeración falló y el motivo NO viaja: desde el cerebro esta máquina se ve " +
			"idéntica a una cuyo inventario no cambió, que es el defecto que costó 64 alertas sin causa")
	}
	if !strings.Contains(motivo, "acceso denegado") {
		t.Errorf("el motivo no dice qué pasó: %q. Se guarda el texto y no un booleano justamente "+
			"porque las causas se arreglan distinto", motivo)
	}

	// 2 · CUANDO SE ARREGLA, EL MOTIVO SE VACÍA. Si no, la alerta no se apaga nunca y la máquina
	// queda marcada como rota para siempre — la misma trampa que documenta `FijarServiciosOmitidos`.
	// `unServicio` es el helper que ya usan las pruebas de cadencia de este paquete: se reusa en vez
	// de armar el struct a mano, que es como se cuelan pruebas que compilan y no representan nada.
	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		return unServicio(fleet.EstadoCorriendo), nil
	}
	olvidarEnumeracion()

	if _, _, mandar, _ := serviciosDelLatido(); !mandar {
		t.Fatal("con la enumeración sana el inventario tiene que viajar")
	}
	if m := motivoDeEnumeracionFallida(); m != "" {
		t.Errorf("la enumeración se arregló y el motivo quedó puesto (%q): la alerta no se apagaría "+
			"nunca y la máquina quedaría marcada como rota para siempre", m)
	}
}

// Y EL SOBRE LO LLEVA AFUERA DEL `if`, QUE ES LA MITAD QUE LA PRUEBA DE ARRIBA NO PUEDE VER.
//
// `motivoDeEnumeracionFallida()` puede funcionar perfecto y el campo no viajar igual, si la
// asignación quedó adentro del `if mandarInventario`. Eso no se puede ejercitar sin levantar un
// agente entero contra un cerebro de prueba, así que acá se comprueba sobre el CÓDIGO — pero
// mirando la ESTRUCTURA (en qué bloque cae la línea) y no un texto suelto, que es la diferencia
// entre esta guarda y las siete que A107 encontró satisfechas por un comentario.
func TestLaAsignacionDelFalloNoQuedaAdentroDelIfDelInventario(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("agent.go"))
	if err != nil {
		t.Fatalf("no se pudo leer agent.go: %v", err)
	}
	lineas := strings.Split(string(b), "\n")

	iAsigna, iIf := -1, -1
	for i, l := range lineas {
		s := strings.TrimSpace(l)
		if strings.HasPrefix(s, "//") {
			continue // los comentarios no asignan nada
		}
		if iAsigna < 0 && strings.Contains(s, "carga.ServiciosError") && strings.Contains(s, "=") {
			iAsigna = i
		}
		if iIf < 0 && strings.HasPrefix(s, "if mandarInventario") {
			iIf = i
		}
	}
	if iAsigna < 0 {
		t.Fatal("no se encontró la asignación de `carga.ServiciosError` en agent.go: el fallo del " +
			"enumerador no viaja en el latido, así que el cerebro no puede decir por qué una máquina " +
			"está callada")
	}
	if iIf < 0 {
		t.Fatal("no se encontró el `if mandarInventario` en agent.go: esta guarda quedó vieja y no " +
			"está midiendo nada")
	}

	// El bloque del `if` va desde su llave hasta que la profundidad vuelve a cero.
	prof := 0
	fin := -1
	for i := iIf; i < len(lineas); i++ {
		prof += strings.Count(lineas[i], "{") - strings.Count(lineas[i], "}")
		if i > iIf && prof <= 0 {
			fin = i
			break
		}
	}
	if fin < 0 {
		t.Fatal("no se pudo delimitar el bloque de `if mandarInventario`: sin eso un verde acá no " +
			"significaría nada")
	}
	if iAsigna > iIf && iAsigna <= fin {
		t.Errorf("`carga.ServiciosError` (línea %d) está ADENTRO del `if mandarInventario` "+
			"(líneas %d-%d).\n\n"+
			"Ahí no se emite nunca en el único caso que importa: cuando la enumeración falla NO hay "+
			"inventario que mandar, y ése es justamente el momento en que el cerebro necesita saber "+
			"por qué. Va AFUERA del `if`.\n"+
			"Su vecina `carga.ServiciosOmitidos` sí va adentro, y no es una inconsistencia: aquélla "+
			"sólo tiene sentido cuando hay lista, ésta sólo cuando no la hay.",
			iAsigna+1, iIf+1, fin+1)
	}
}
