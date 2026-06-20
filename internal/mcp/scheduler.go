package mcp

// scheduler.go implementa el auto-mantenimiento de fondo del daemon (Track 5 / T5.2):
// un ciclo recurrente que mantiene la memoria filosa SIN requerir reinicio. Antes el
// mantenimiento corría una sola vez, síncrono, en el arranque — un daemon long-running
// nunca volvía a mantenerse. Todo se serializa contra el dispatch de tools vía el
// write-lock de dispatchMu (el mismo punto de serialización que usa el transporte HTTP),
// y respeta el throttle de T5.1 (MaintenanceDue).

import (
	"context"
	"time"

	"musubi/internal/logx"
	"musubi/internal/memory"
)

// maintenanceOptions arma las opciones del ciclo desde la config del server. La comparten
// la tool musubi_maintain y el scheduler de fondo, para no duplicar el mapeo.
func (s *McpServer) maintenanceOptions() memory.MaintenanceOptions {
	return memory.MaintenanceOptions{
		DedupThreshold:         s.maintenance.DedupThreshold,
		DecayHalfLifeDays:      s.maintenance.DecayHalfLifeDays,
		DecayMinSalience:       s.maintenance.DecayMinSalience,
		DecayMinAgeDays:        s.maintenance.DecayMinAgeDays,
		PurgeArchivedAfterDays: s.maintenance.PurgeArchivedAfterDays,
		Vacuum:                 s.maintenance.Vacuum,
	}
}

// RunScheduledMaintenance corre el ciclo de mantenimiento UNA vez si el throttle lo
// permite, serializado contra el dispatch de tools (toma el write-lock exclusivo). Es
// best-effort: devuelve si corrió, el resumen y el error. La verificación del throttle va
// DENTRO del lock para no solapar dos ciclos (arranque + primer tick, o dos ticks).
func (s *McpServer) RunScheduledMaintenance() (ran bool, rep memory.MaintenanceReport, err error) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()

	due, derr := s.engine.MaintenanceDue(s.maintenance.AutoIntervalHours)
	if derr != nil {
		return false, rep, derr
	}
	if !due {
		return false, rep, nil
	}
	start := time.Now()
	rep, err = s.engine.Maintain(s.maintenanceOptions())
	if err != nil {
		return false, rep, err
	}
	if mErr := s.engine.MarkMaintenanceNow(); mErr != nil {
		logx.Error("scheduler: no se pudo marcar last_maintenance", "error", mErr)
	}
	logx.Info("scheduler: mantenimiento",
		"merged", rep.Consolidate.Merged, "archived", rep.Decay.Archived,
		"purged", rep.Purged, "dur", time.Since(start).String())
	return true, rep, nil
}

// RunMaintenanceScheduler corre RunScheduledMaintenance en un ticker periódico hasta que
// ctx se cancela (shutdown del daemon). interval<=0 desactiva el scheduler. Pensado para
// correr en su propia goroutine; bloquea hasta la cancelación.
func (s *McpServer) RunMaintenanceScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, _, err := s.RunScheduledMaintenance(); err != nil {
				logx.Error("scheduler: mantenimiento falló", "error", err)
			}
		}
	}
}
