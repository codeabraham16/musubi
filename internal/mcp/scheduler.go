package mcp

// scheduler.go implementa el auto-mantenimiento de fondo del daemon (Track 5 / T5.2):
// un ciclo recurrente que mantiene la memoria filosa SIN requerir reinicio. Antes el
// mantenimiento corría una sola vez, síncrono, en el arranque — un daemon long-running
// nunca volvía a mantenerse. Todo se serializa contra el dispatch de tools vía el
// write-lock de dispatchMu (el mismo punto de serialización que usa el transporte HTTP),
// y respeta el throttle de T5.1 (MaintenanceDue).

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"musubi/internal/logx"
	"musubi/internal/memory"
)

// countingSave envuelve un handler de save para contar las corridas exitosas y, al cruzar
// el umbral maintenance.AutoAfterSaves, disparar un mantenimiento (T5.3). El conteo va por
// el wrapper para no instrumentar cada return de éxito de los handlers.
func (s *McpServer) countingSave(h func(json.RawMessage) (interface{}, *RpcError)) func(json.RawMessage) (interface{}, *RpcError) {
	return func(raw json.RawMessage) (interface{}, *RpcError) {
		res, rpcErr := h(raw)
		if rpcErr == nil {
			s.maybeTriggerMaintenance()
		}
		return res, rpcErr
	}
}

// maybeTriggerMaintenance incrementa el contador de saves y, si cruza el umbral, dispara
// un mantenimiento en goroutine (async) — NO inline: el handler de save ya tiene el
// write-lock de dispatchMu, así que correr el ciclo acá re-entraría el lock (deadlock). La
// goroutine lo toma cuando el handler lo libera. maintBusy mantiene un solo ciclo en vuelo.
func (s *McpServer) maybeTriggerMaintenance() {
	threshold := s.maintenance.AutoAfterSaves
	if threshold <= 0 {
		return // desactivado (opt-in)
	}
	if s.saveCount.Add(1) < int64(threshold) {
		return
	}
	s.saveCount.Store(0)
	if !s.maintBusy.CompareAndSwap(false, true) {
		return // ya hay un mantenimiento en vuelo
	}
	go func() {
		defer s.maintBusy.Store(false)
		if _, _, err := s.RunScheduledMaintenance(); err != nil {
			logx.Error("auto-mantenimiento por volumen de saves falló", "error", err)
		}
	}()
}

// maintenanceOptions arma las opciones del ciclo desde la config del server. La comparten
// la tool musubi_maintain y el scheduler de fondo, para no duplicar el mapeo.
func (s *McpServer) maintenanceOptions() memory.MaintenanceOptions {
	return memory.MaintenanceOptions{
		DedupThreshold:         s.maintenance.DedupThreshold,
		DecayHalfLifeDays:      s.maintenance.DecayHalfLifeDays,
		DecayMinSalience:       s.maintenance.DecayMinSalience,
		DecayMinAgeDays:        s.maintenance.DecayMinAgeDays,
		DecayProtectImportance: s.maintenance.DecayProtectImportance,
		DecayReinforcementK:    s.maintenance.DecayReinforcementK,
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
	// Auto-curación (T5.4): el ciclo automático también se auto-cura. Repara solo los
	// checks de bajo riesgo (apply con backup) y persiste el reporte para el hook de
	// arranque. Best-effort: un fallo acá no invalida el mantenimiento ya hecho.
	if health, hErr := s.engine.AutoHeal(); hErr != nil {
		logx.Error("scheduler: auto-curación falló", "error", hErr)
	} else if health.Status != "ok" {
		logx.Info("scheduler: auto-curación dejó problemas no auto-reparables", "status", health.Status)
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

// RunOutboxScheduler drena el OUTBOX del cerebro híbrido (F2) en un ticker periódico hasta
// que ctx se cancela. Es el GEMELO de RunMaintenanceScheduler pero NO toma dispatchMu: el
// drain hace I/O de red (segundos por fila) y tomar el lock global congelaría todas las tools
// (D8/R6). El claim y los marks son transacciones cortas del engine (thread-safe por sí solas);
// el POST ocurre entre medio, fuera de todo lock. interval<=0 o syncClient nil desactivan el
// drain. Pensado para correr en su propia goroutine; bloquea hasta la cancelación.
func (s *McpServer) RunOutboxScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 || s.syncClient == nil {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.drainOutboxOnce(ctx)
		}
	}
}

// drainOutboxOnce reclama un batch del outbox y empuja cada fila al central, aplicando el
// resultado (sent / retry con backoff / dead). Best-effort: un fallo de una fila no aborta el
// batch. Cada item trae Attempts (intentos ya fallidos): un fallo transitorio va a dead cuando
// se alcanzó max_attempts, si no se reprograma con backoff exponencial+jitter; un fallo
// permanente va directo a dead (R11-R13). El ctx corta el barrido a mitad si hay shutdown.
func (s *McpServer) drainOutboxOnce(ctx context.Context) {
	items, err := s.engine.ClaimOutboxBatch(s.syncCfg.BatchSize, s.syncCfg.LeaseSeconds)
	if err != nil {
		logx.Error("drain: no se pudo reclamar el batch del outbox", "error", err)
		return
	}
	for _, item := range items {
		select {
		case <-ctx.Done():
			return
		default:
		}
		perr := s.syncClient.Push(item)
		if perr == nil {
			if merr := s.engine.MarkOutboxSent(item.ObsID); merr != nil {
				logx.Error("drain: no se pudo marcar como enviado", "obs_id", item.ObsID, "error", merr)
			}
			continue
		}
		// Fallo permanente (params/auth) → dead-letter sin reintentar. También si el
		// intento recién fallado alcanzó el tope de reintentos configurado.
		attemptsSoFar := item.Attempts + 1
		if errors.Is(perr, errPermanent) || attemptsSoFar >= s.syncCfg.MaxAttempts {
			if merr := s.engine.MarkOutboxDead(item.ObsID, perr.Error()); merr != nil {
				logx.Error("drain: no se pudo marcar como dead", "obs_id", item.ObsID, "error", merr)
			}
			continue
		}
		// Fallo transitorio con margen: reprogramar con backoff exponencial + jitter.
		backoff := backoffSeconds(attemptsSoFar, s.syncCfg.BackoffBaseSeconds, s.syncCfg.BackoffMaxSeconds)
		if merr := s.engine.MarkOutboxRetry(item.ObsID, backoff, perr.Error()); merr != nil {
			logx.Error("drain: no se pudo reprogramar el reintento", "obs_id", item.ObsID, "error", merr)
		}
	}
}

// backoffSeconds calcula el backoff del n-ésimo intento (n>=1): exponencial base*2^(n-1),
// acotado por max, más un jitter de hasta +20% (D9). El jitter evita el thundering herd
// cuando muchas filas vencen juntas al recuperarse la red. El resultado queda garantizado en
// [base*2^(n-1), base*2^(n-1)*1.2], siempre acotado por max (rango verificable en tests).
func backoffSeconds(attempts, base, max int) int {
	if base <= 0 {
		base = 5
	}
	if max <= 0 {
		max = 300
	}
	n := attempts
	if n < 1 {
		n = 1
	}
	// exp = base * 2^(n-1) con saturación temprana a max para no desbordar en caídas largas.
	exp := base
	for i := 1; i < n; i++ {
		exp *= 2
		if exp >= max {
			exp = max
			break
		}
	}
	if exp > max {
		exp = max
	}
	// Jitter en [0, 20%] del valor base, sin superar max (mantiene el resultado acotado).
	jitterCap := exp / 5
	jitter := 0
	if jitterCap > 0 {
		jitter = rand.Intn(jitterCap + 1)
	}
	v := exp + jitter
	if v > max {
		v = max
	}
	return v
}
