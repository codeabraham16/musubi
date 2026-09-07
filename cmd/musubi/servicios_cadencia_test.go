package main

import (
	"errors"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// LA PREGUNTA AL SISTEMA OPERATIVO TIENE SU PROPIO FRENO, distinto del que frena el envío.
//
// El agente late cada 30 s y antes enumeraba en CADA latido. En Windows eso lanza PowerShell y
// pide Win32_Service por WMI: ~3 s por vuelta (3,13 s y 2,84 s, cronometradas). El inventario en
// cambio se manda como mucho cada 5 min, así que casi todas esas preguntas terminaban en «no
// cambió nada, no mando nada» — tres segundos de PowerShell cada medio minuto, en cada máquina de
// la flota, todo el día.

// espiaEnumeracion cuenta cuántas veces se le preguntó de verdad al sistema.
func espiaEnumeracion(t *testing.T, lista []fleet.ReporteServicio, err error) *int {
	t.Helper()
	veces := 0
	anterior := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) {
		veces++
		return lista, err
	}
	reiniciarFrenos()
	t.Cleanup(func() {
		enumerarServicios = anterior
		reiniciarFrenos()
	})
	return &veces
}

func unServicio(estado fleet.EstadoServicio) []fleet.ReporteServicio {
	return []fleet.ReporteServicio{{
		Nombre: "postgres", Clase: "systemd",
		Salud: fleet.SaludServicio{Tomada: time.Now(), Estado: estado},
	}}
}

func TestNoSeLePreguntaAlSistemaEnCadaLatido(t *testing.T) {
	veces := espiaEnumeracion(t, unServicio(fleet.EstadoCorriendo), nil)

	// Diez latidos seguidos, que es lo que pasa en cinco minutos de agente.
	for i := 0; i < 10; i++ {
		serviciosDelLatido()
	}
	if *veces != 1 {
		t.Errorf("se le preguntó al sistema %d veces en 10 latidos, esperaba 1: cada pregunta cuesta ~3 s en Windows", *veces)
	}
}

// LA OTRA MITAD, y sin ella «no preguntar NUNCA» pasaría el test de arriba: pasado el intervalo,
// hay que volver a preguntar. Un inventario congelado es peor que uno que cuesta.
func TestPasadoElIntervaloSeVuelveAPreguntar(t *testing.T) {
	veces := espiaEnumeracion(t, unServicio(fleet.EstadoCorriendo), nil)

	serviciosDelLatido()
	if *veces != 1 {
		t.Fatalf("la primera vuelta tiene que preguntar, preguntó %d veces", *veces)
	}

	// Se envejece la respuesta guardada en vez de dormir un minuto: la prueba mide la REGLA, no
	// el reloj de pared. Dormir la haría lenta y flaky, que es de dónde venimos.
	ultimaEnumeracion.Lock()
	ultimaEnumeracion.cuando = time.Now().Add(-intervaloEnumeracion - time.Second)
	ultimaEnumeracion.Unlock()

	serviciosDelLatido()
	if *veces != 2 {
		t.Errorf("pasado el intervalo hay que volver a preguntar; se preguntó %d veces en total", *veces)
	}
}

// El error se guarda igual que la lista. Un sistema que no contesta tampoco tiene que ser
// interrogado cada 30 s: sin esto, la máquina donde la enumeración falla es justo la que más
// PowerShell arranca.
func TestUnSistemaQueNoContestaTampocoSeInterrogaEnCadaLatido(t *testing.T) {
	veces := espiaEnumeracion(t, nil, errors.New("systemd no contesta"))

	for i := 0; i < 5; i++ {
		lista, mandar, _ := serviciosDelLatido()
		if mandar || lista != nil {
			t.Fatalf("con la enumeración rota no hay nada que mandar, y devolvió mandar=%v lista=%v", mandar, lista)
		}
	}
	if *veces != 1 {
		t.Errorf("se reintentó %d veces en 5 latidos: un sistema que falla no se interroga más seguido que uno sano", *veces)
	}
}

// EL FRENO DE LA PREGUNTA NO PUEDE COMERSE EL DEL ENVÍO. Son dos cosas distintas y el riesgo al
// juntarlas es que un cambio real quede escondido detrás de la caché por más de lo que dura.
func TestUnCambioViajaApenasSeVuelveAPreguntar(t *testing.T) {
	anterior := enumerarServicios
	enumerarServicios = func() ([]fleet.ReporteServicio, error) { return unServicio(fleet.EstadoCorriendo), nil }
	reiniciarFrenos()
	t.Cleanup(func() {
		enumerarServicios = anterior
		reiniciarFrenos()
	})

	_, mandar, confirmar := serviciosDelLatido()
	if !mandar {
		t.Fatal("el primer inventario tiene que viajar")
	}
	confirmar()

	// El servicio se cae, pero todavía no venció la caché: el agente no puede saberlo.
	enumerarServicios = func() ([]fleet.ReporteServicio, error) { return unServicio(fleet.EstadoFallado), nil }
	if _, mandar, _ := serviciosDelLatido(); mandar {
		t.Error("mandó un cambio que todavía no podía haber visto: la caché no se está usando")
	}

	// Vence la caché y el cambio viaja en la primera vuelta posterior, sin esperar los 5 minutos
	// del envío periódico.
	ultimaEnumeracion.Lock()
	ultimaEnumeracion.cuando = time.Now().Add(-intervaloEnumeracion - time.Second)
	ultimaEnumeracion.Unlock()

	lista, mandar, _ := serviciosDelLatido()
	if !mandar {
		t.Fatal("el servicio se cayó y el inventario no viajó: quedaría hasta 5 minutos sin verse")
	}
	if len(lista) != 1 || lista[0].Salud.Estado != fleet.EstadoFallado {
		t.Errorf("viajó el inventario VIEJO: %+v", lista)
	}
}

// El freno de la pregunta es más rápido que el del envío, y ese orden importa: al revés, la caché
// escondería cambios durante más tiempo del que el envío periódico tolera, y un servicio caído
// podría tardar más de lo que promete intervaloInventarioCompleto.
func TestSePreguntaMasSeguidoDeLoQueSeManda(t *testing.T) {
	if intervaloEnumeracion >= intervaloInventarioCompleto {
		t.Errorf("intervaloEnumeracion (%s) tiene que ser MENOR que intervaloInventarioCompleto (%s): "+
			"si no, el envío periódico llegaría con datos más viejos que su propia promesa",
			intervaloEnumeracion, intervaloInventarioCompleto)
	}
}
