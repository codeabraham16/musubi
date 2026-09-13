package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// ciclosDeFondo es lo que lanzarMantenimientoYGrafo usa del servidor MCP. Existe para que el
// cableado se pruebe con un servidor de mentira: los dos errores que importan acá —el grafo que no
// espera al mantenimiento y el scheduler que bloquea el loop stdio— no los ve ninguna prueba del
// paquete mcp, porque viven en CÓMO se lanzan las goroutines y no en lo que hacen.
type ciclosDeFondo interface {
	RunScheduledMaintenance() (ran bool, rep memory.MaintenanceReport, err error)
	RunMaintenanceScheduler(ctx context.Context, interval time.Duration)
	RunCodeGraphScheduler(ctx context.Context, interval time.Duration, despuesDe <-chan struct{})
}

// lanzarMantenimientoYGrafo arranca el auto-mantenimiento de la memoria y el reindexado del grafo, y
// VUELVE ENSEGUIDA: todo lo que lanza corre en goroutines, porque quien lo llama es runDaemon y
// después tiene que atender el loop stdio. Lo que se lanza se para cancelando ctx.
//
// Auto-mantenimiento (Track 5 / T5.2): el daemon es long-running; sin esto el ciclo cognitivo
// (consolidar/olvidar/purgar) solo correría una vez al arrancar. Dos goroutines best-effort que
// serializan contra el dispatch vía el write-lock del server: (1) una corrida de arranque NO
// bloqueante (un VACUUM grande no demora el primer pedido); (2) un ticker periódico.
//
// `mantenimientoDeArranque` se cierra cuando esa corrida de arranque TERMINA (o enseguida, si el
// mantenimiento está apagado): la corrida de arranque del grafo lo espera, porque las dos escriben y
// no hay motivo para que compitan por la base en el primer minuto del daemon.
//
// Las dos cosas las custodian las pruebas de ciclos_de_fondo_test.go, con sus sabotajes mecanizados:
// el canal que se cierra ANTES del mantenimiento y el scheduler lanzado sin `go`.
func lanzarMantenimientoYGrafo(ctx context.Context, srv ciclosDeFondo, m config.MaintenanceConfig, avisos io.Writer) {
	mantenimientoDeArranque := make(chan struct{})
	if m.AutoIntervalHours > 0 {
		go func() {
			defer close(mantenimientoDeArranque)
			if ran, rep, mErr := srv.RunScheduledMaintenance(); mErr != nil {
				fmt.Fprintf(avisos, "musubi: auto-mantenimiento de arranque falló: %v\n", mErr)
			} else if ran {
				fmt.Fprintf(avisos, "musubi: auto-mantenimiento: %d fusionadas, %d archivadas, %d evictadas, %d purgadas\n", rep.Consolidate.Merged, rep.Decay.Archived, rep.Evicted, rep.Purged)
			}
		}()
		go srv.RunMaintenanceScheduler(ctx, time.Duration(m.AutoIntervalHours*float64(time.Hour)))
	} else {
		close(mantenimientoDeArranque)
	}
	// El grafo de código se mantiene solo (P3). Va en su PROPIO gate y no colgado del de
	// mantenimiento: son dos ciclos con costos y riesgos distintos, y quien apague el mantenimiento
	// de la memoria no está pidiendo que además se le quede rancio el grafo. Corre una vez al
	// arrancar (después del mantenimiento de arranque) y después por intervalo.
	if m.GraphIndexHours > 0 {
		go srv.RunCodeGraphScheduler(ctx, time.Duration(m.GraphIndexHours*float64(time.Hour)), mantenimientoDeArranque)
	}
}
