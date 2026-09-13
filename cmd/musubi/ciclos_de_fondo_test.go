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

// S5 — LANZAR LOS CICLOS NO BLOQUEA AL DAEMON.
//
// runDaemon llama esto y DESPUÉS atiende el loop stdio. Un scheduler lanzado sin `go` no vuelve
// nunca, y el daemon arranca mudo: el cliente MCP espera un initialize que nadie contesta.
func TestLanzarLosCiclosDeFondoNoBloqueaAlDaemon(t *testing.T) {
	f := nuevoCicloFalso()
	close(f.soltar)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // desbloquea al que se haya quedado esperando, así la prueba no deja nada colgado

	volvio := make(chan struct{})
	go func() {
		defer close(volvio)
		lanzarMantenimientoYGrafo(ctx, f, ambosCiclos, io.Discard)
	}()
	select {
	case <-volvio:
	case <-time.After(5 * time.Second):
		t.Fatal("lanzarMantenimientoYGrafo no volvió en 5 s: algún ciclo corre en la goroutine de runDaemon y el loop stdio no arranca")
	}
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

	lanzarMantenimientoYGrafo(ctx, f, ambosCiclos, io.Discard)
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
	lanzarMantenimientoYGrafo(ctx, g, config.MaintenanceConfig{GraphIndexHours: 1}, io.Discard)
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
