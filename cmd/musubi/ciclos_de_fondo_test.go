package main

import (
	"context"
	"io"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// ciclosDeFondo_test.go custodia el CABLEADO de los ciclos de fondo del daemon: que el grafo espere
// al mantenimiento de arranque y que nada de lo que se lanza bloquee a runDaemon.

// cicloFalso retiene el mantenimiento de arranque hasta que se cierre `soltar`, y los schedulers
// hasta que se cancele el contexto, como los de verdad.
type cicloFalso struct {
	soltar       chan struct{}
	grafoLanzado chan (<-chan struct{})
}

func nuevoCicloFalso() *cicloFalso {
	return &cicloFalso{soltar: make(chan struct{}), grafoLanzado: make(chan (<-chan struct{}), 1)}
}

func (f *cicloFalso) RunScheduledMaintenance() (bool, memory.MaintenanceReport, error) {
	<-f.soltar
	return false, memory.MaintenanceReport{}, nil
}

func (f *cicloFalso) RunMaintenanceScheduler(ctx context.Context, _ time.Duration) { <-ctx.Done() }

func (f *cicloFalso) RunCodeGraphScheduler(ctx context.Context, _ time.Duration, despuesDe <-chan struct{}) {
	f.grafoLanzado <- despuesDe
	<-ctx.Done()
}

var ambosCiclos = config.MaintenanceConfig{AutoIntervalHours: 6, GraphIndexHours: 1}

// lanzarConPlazo llama a lanzarMantenimientoYGrafo desde OTRA goroutine y le da 5 s para volver.
//
// Toda prueba de este archivo la llama por acá y no directo, por lo que pasa con S5: un scheduler
// sin `go` no vuelve nunca, y si la llamada está en la goroutine de la prueba el t.Fatal no llega
// jamás. La prueba se queda esperando un ctx que sólo cancela su propio Cleanup, y el que falla es
// el PAQUETE entero, por el -timeout de go test. Medido con UNA sola prueba llamando directo: la
// otra falló a los 5 s y el paquete murió igual con «panic: test timed out after 5m0s», 300 s.
// Con el plazo la prueba falla, su Cleanup cancela el ctx y el scheduler falso vuelve solo.
func lanzarConPlazo(t *testing.T, ctx context.Context, srv ciclosDeFondo, m config.MaintenanceConfig) {
	t.Helper()
	volvio := make(chan struct{})
	go func() {
		defer close(volvio)
		lanzarMantenimientoYGrafo(ctx, srv, m, io.Discard)
	}()
	select {
	case <-volvio:
	case <-time.After(5 * time.Second):
		t.Fatal("lanzarMantenimientoYGrafo no volvió en 5 s: algún ciclo corre en la goroutine de runDaemon y el loop stdio no arranca")
	}
}

// S5 — LANZAR LOS CICLOS NO BLOQUEA AL DAEMON.
//
// runDaemon llama esto y DESPUÉS atiende el loop stdio. Un scheduler lanzado sin `go` no vuelve
// nunca, y el daemon arranca mudo: el cliente MCP espera un initialize que nadie contesta.
//
// Sabotaje que la pone roja: lanzar RunCodeGraphScheduler sin `go` (S5). Cada prueba de este archivo
// que la llama falla a los 5 s, y el PAQUETE no cuelga: ninguna la llama sin plazo (ver lanzarConPlazo).
// arnes: archivo="cmd/musubi/ciclos_de_fondo.go"
// arnes: de="go srv.RunCodeGraphScheduler("
// arnes: a="srv.RunCodeGraphScheduler("
func TestLanzarLosCiclosDeFondoNoBloqueaAlDaemon(t *testing.T) {
	f := nuevoCicloFalso()
	close(f.soltar)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // desbloquea al que se haya quedado esperando, así la prueba no deja nada colgado

	lanzarConPlazo(t, ctx, f, ambosCiclos)
	select {
	case <-f.grafoLanzado:
	case <-time.After(5 * time.Second):
		t.Fatal("con graph_index_hours > 0 el scheduler del grafo no se lanzó")
	}
}

// S2 — EL GRAFO ESPERA A QUE TERMINE EL MANTENIMIENTO DE ARRANQUE.
//
// Los dos escriben, y el mantenimiento puede traer un VACUUM. El canal que recibe el scheduler del
// grafo tiene que seguir abierto mientras el mantenimiento corre, y cerrarse cuando termina.
//
// Sabotaje que la pone roja: cerrar el canal al ENTRAR a la goroutine del mantenimiento y no al salir (S2).
// arnes: archivo="cmd/musubi/ciclos_de_fondo.go"
// arnes: de="defer close(mantenimientoDeArranque)"
// arnes: a="close(mantenimientoDeArranque)"
func TestElGrafoEsperaAlMantenimientoDeArranque(t *testing.T) {
	f := nuevoCicloFalso()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() {
		select {
		case <-f.soltar:
		default:
			close(f.soltar)
		}
	})

	lanzarConPlazo(t, ctx, f, ambosCiclos)
	var despuesDe <-chan struct{}
	select {
	case despuesDe = <-f.grafoLanzado:
	case <-time.After(5 * time.Second):
		t.Fatal("el scheduler del grafo no se lanzó")
	}
	select {
	case <-despuesDe:
		t.Fatal("el canal del grafo se cerró con el mantenimiento de arranque todavía corriendo: los dos compiten por la base")
	case <-time.After(300 * time.Millisecond):
	}
	close(f.soltar)
	select {
	case <-despuesDe:
	case <-time.After(5 * time.Second):
		t.Fatal("el mantenimiento de arranque terminó y el canal del grafo no se cerró en 5 s: el grafo no arranca nunca")
	}

	// Con el mantenimiento apagado no hay nada que esperar.
	g := nuevoCicloFalso()
	lanzarConPlazo(t, ctx, g, config.MaintenanceConfig{GraphIndexHours: 1})
	select {
	case d := <-g.grafoLanzado:
		select {
		case <-d:
		case <-time.After(5 * time.Second):
			t.Fatal("con el mantenimiento apagado el canal del grafo tenía que llegar cerrado")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("con el mantenimiento apagado el scheduler del grafo no se lanzó")
	}
}
