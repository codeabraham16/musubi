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
	"strconv"
	"strings"
	"time"

	"musubi/internal/cognition"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// countingSaveCtx envuelve un handler de save CTX-AWARE para contar las corridas exitosas y, al cruzar
// el umbral maintenance.AutoAfterSaves, disparar un mantenimiento (T5.3). El conteo va por el wrapper
// para no instrumentar cada return de éxito de los handlers. (Antes había una variante no-ctx
// `countingSave`; quedó sin uso al pasar toolPromote a ctx-aware por el aislamiento #11.)
//
// countingSaveCtx es para handlers CTX-AWARE (Track 17): toolSaveObservation
// necesita el principal del contexto para derivar la atribución de escritura por credencial.
func (s *McpServer) countingSaveCtx(h func(context.Context, json.RawMessage) (interface{}, *RpcError)) func(context.Context, json.RawMessage) (interface{}, *RpcError) {
	return func(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
		res, rpcErr := h(ctx, raw)
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
		MaxActivePerProject:    s.maintenance.MaxActivePerProject,
		Vacuum:                 s.maintenance.Vacuum,
		ProposalTTLHours:       s.cognitionCfg.ProposalTTLHours,
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
	// Retención del LEDGER DE USO (F0 · invariante L6). Cuelga del mantenimiento que ya existe
	// en vez de un timer propio: una tabla de telemetría sin techo termina siendo el problema
	// que vino a diagnosticar, y encima en un sistema que se autodiagnostica con `doctor`.
	// Best-effort como todo lo del ledger: si falla, se logea y el mantenimiento sigue.
	purgadasLedger := s.purgarLedger()

	logx.Info("scheduler: mantenimiento",
		"merged", rep.Consolidate.Merged, "archived", rep.Decay.Archived,
		"evicted", rep.Evicted, "purged", rep.Purged,
		"ledger_purgado", purgadasLedger, "dur", time.Since(start).String())
	return true, rep, nil
}

// purgarLedger borra las invocaciones más viejas que la retención configurada. Devuelve cuántas
// para el log. Silencioso y sin efecto si el ledger está apagado o el backend no lo soporta.
func (s *McpServer) purgarLedger() int64 {
	if s.ledger == nil || s.ledgerRetentionDays <= 0 {
		return 0
	}
	purgador, ok := s.engine.(interface {
		PurgeToolInvocations(ctx context.Context, retentionDays int) (int64, error)
	})
	if !ok {
		return 0
	}
	n, err := purgador.PurgeToolInvocations(context.Background(), s.ledgerRetentionDays)
	if err != nil {
		logx.Warn("ledger de uso: no se pudo purgar", "error", err)
		return 0
	}
	return n
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

// metaInboundCursor guarda el rowid del central hasta el que ya bajamos memoria shared (C5.3b).
const metaInboundCursor = "sync:inbound_cursor"

// RunInboundScheduler baja periódicamente la memoria 'shared' del proyecto DESDE el central (sync
// ENTRANTE, C5.3b): el espejo de RunOutboxScheduler en sentido de bajada. Solo corre si hay sync
// configurado (syncClient) Y el proyecto está en team mode (memory.team_mode) — un proyecto local no
// baja nada. Preserva local-first: baja a la DB local, el recall sigue offline y rápido.
func (s *McpServer) RunInboundScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 || s.syncClient == nil || !s.memory.TeamMode {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.drainInboundOnce(ctx)
		}
	}
}

// drainInboundOnce baja páginas de memoria shared del central desde el cursor guardado y las ingiere
// localmente (IngestShared, anti-loop: NO re-encola en el outbox). Avanza el cursor con el mayor
// rowid del lote. Best-effort: un fallo de red reintenta en el próximo tick; un fallo de ingest de
// una fila se logea y no aborta el batch. Tope de páginas por tick para no monopolizar.
func (s *McpServer) drainInboundOnce(ctx context.Context) {
	var cur int64
	if raw, ok, _ := s.engine.GetMeta(metaInboundCursor); ok {
		cur, _ = strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	}
	limit := s.syncCfg.BatchSize
	if limit <= 0 {
		limit = 200
	}
	for page := 0; page < 20; page++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		items, next, err := s.syncClient.Pull(cur, limit)
		if err != nil {
			logx.Error("inbound: no se pudo bajar del central (reintenta en el próximo tick)", "error", err)
			return
		}
		if len(items) == 0 {
			return
		}
		// El cursor avanza SÓLO hasta la última fila ingerida OK de forma CONTIGUA. Antes se avanzaba a
		// `next` aunque un IngestShared fallara ⇒ esa fila quedaba saltada PARA SIEMPRE (hueco permanente
		// en el mirror local; auditoría 2026-07-26 #5). Ahora un fallo (p. ej. SQLITE_BUSY transitorio)
		// corta el avance: la fila y las siguientes se re-bajan en el próximo tick. Como un batch SIN
		// fallos sí avanza, no hay livelock (una fila "veneno" permanente frena, pero se ve en los logs).
		lastOK := cur
		failed := false
		for _, o := range items {
			if _, ierr := s.engine.IngestShared(o); ierr != nil {
				logx.Error("inbound: no se pudo ingerir obs shared; NO se avanza el cursor más allá (se reintenta en el próximo tick)", "id", o.ID, "rowid", o.RowID, "error", ierr)
				failed = true
				break
			}
			if o.RowID > lastOK {
				lastOK = o.RowID
			}
		}
		advanceTo := lastOK
		if !failed && next > advanceTo {
			advanceTo = next // batch completo OK: honrar el cursor que devolvió el central
		}
		if advanceTo > cur {
			cur = advanceTo
			if merr := s.engine.SetMeta(metaInboundCursor, strconv.FormatInt(cur, 10)); merr != nil {
				logx.Error("inbound: no se pudo guardar el cursor entrante", "error", merr)
			}
		}
		if failed {
			return // no seguir paginando tras un fallo: se retoma desde acá en el próximo tick
		}
		if len(items) < limit {
			return
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
		// Política OFFLINE-FIRST (sync-hardening): un fallo PERMANENTE (4xx/params/auth, o
		// error JSON-RPC del receptor) es irrecuperable reintentando lo mismo → dead-letter.
		// Un fallo TRANSITORIO (red/timeout/5xx/429) NUNCA muere: reintenta indefinidamente
		// con backoff exponencial acotado. El tope por CONTEO se eliminó a propósito —
		// confundía "central temporalmente inalcanzable" con "irrecuperable" y hacía perder
		// memoria shared en un corte de minutos. Lo genuinamente atascado se ve por
		// musubi_sync_status y se rescata con musubi_sync_requeue.
		if errors.Is(perr, errPermanent) {
			if merr := s.engine.MarkOutboxDead(item.ObsID, perr.Error()); merr != nil {
				logx.Error("drain: no se pudo marcar como dead", "obs_id", item.ObsID, "error", merr)
			}
			continue
		}
		// Transitorio: reprogramar con backoff exponencial + jitter (attempts sólo alimenta
		// el backoff y la observabilidad; ya no dispara dead).
		backoff := backoffSeconds(item.Attempts+1, s.syncCfg.BackoffBaseSeconds, s.syncCfg.BackoffMaxSeconds)
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

// RunCodeGraphScheduler re-indexa el grafo de código de forma INCREMENTAL cada `interval`, hasta
// que ctx se cancela. interval<=0 lo desactiva.
//
// EL AGUJERO QUE TAPA. Hasta acá el grafo sólo se indexaba si un AGENTE llamaba
// musubi_codegraph_index: no hay subcomando CLI, así que ni un hook de git ni un timer podían
// hacerlo. Dependía de que alguien se acordara. Medido el 2026-08-15 en el cerebro central: el
// grafo estaba fechado el día anterior y no contenía los cuatro PRs de esa jornada.
//
// Y un grafo rancio NO falla ruidosamente. Contesta — con la forma que el código tenía antes. Lo
// consumen musubi_impact y el precheck, que avisan del radio de impacto ANTES de escribir, así que
// el precio de la ranciedad se paga en una decisión de código, no en una consulta curiosa.
//
// POR QUÉ INCREMENTAL Y NO COMPLETO: el incremental compara el fingerprint de cada archivo contra
// el guardado y sólo re-deriva los paquetes sucios. Con el árbol quieto —el caso de casi todos los
// ticks— la corrida es leer fingerprints y salir. Un índice completo cada 6 h sería quemar CPU para
// llegar al mismo grafo.
//
// BEST-EFFORT, como el resto del scheduler: un fallo se loguea y el ciclo sigue. El grafo es una
// AYUDA para el recall y el impacto; que un tick falle no puede tumbar el daemon ni bloquear una
// herramienta.
func (s *McpServer) RunCodeGraphScheduler(ctx context.Context, interval time.Duration) {
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
			s.reindexCodeGraphOnce(ctx)
		}
	}
}

// reindexCodeGraphOnce corre UN índice incremental bajo el candado del despacho.
//
// Toma dispatchMu por el mismo motivo que RunScheduledMaintenance: escribe nodos y aristas, y sin
// el candado se cruzaría con una tool en vuelo. Va en su propio método —y no inline en el select—
// para que el `defer Unlock` cierre en cada vuelta en vez de acumularse hasta que muera el ticker.
func (s *McpServer) reindexCodeGraphOnce(ctx context.Context) {
	s.dispatchMu.Lock()
	defer s.dispatchMu.Unlock()

	res, err := s.indexIncremental(ctx)
	if err != nil {
		logx.Error("scheduler: el índice incremental del grafo falló", "error", err)
		return
	}
	// Sólo se anuncia cuando HUBO trabajo. Un log por tick con "0 paquetes" en un daemon que vive
	// días es ruido que entierra la línea que sí importa.
	if refreshed, _ := res["packages"].(int); refreshed > 0 {
		logx.Info("scheduler: grafo de código re-indexado", "paquetes", refreshed, "podados", res["pruned"])
	}
	// Federación best-effort, igual que en la tool: si el gate está apagado no hace nada, y un
	// fallo del push jamás invalida el índice local, que ya quedó bien.
	s.pushCodeGraphToCentral(ctx)
}

// RunDistillScheduler es el AUTO-DRAIN del acervo de diseño (pilar Musubi Renaissance, el "molino
// continuo"): cada `interval` destila una tanda de `batch` blobs `ingested/*` en tarjetas `design-corpus/*`,
// sin intervención, hasta que ctx se cancela. Es el GEMELO de RunOutboxScheduler y por el mismo motivo NO
// toma dispatchMu: la destilación hace I/O de red (motor LLM + embedder), y el núcleo (runDistillBatch)
// acota su propia sección crítica (read bajo RLock, write bajo Lock, LLM afuera). interval<=0 o SIN motor
// de cognición lo desactivan (no-op inmediato), así que es seguro lanzarlo desde cualquier entrypoint;
// sólo el central con motor lo enciende de verdad. Pensado para correr en su propia goroutine.
func (s *McpServer) RunDistillScheduler(ctx context.Context, interval time.Duration, batch int) {
	if interval <= 0 || !cognition.Enabled(s.cognition) {
		return
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.distillBatchOnce(ctx, batch)
		}
	}
}

// distillBatchOnce destila UNA tanda del acervo y logea el resultado. Best-effort: un fallo del backlog o
// de un blob se logea y el ciclo sigue en el próximo tick. Corre SIN principal (ctx de fondo) ⇒ no hay
// cuota de motor por-principal; el gasto lo dosifican el tamaño de la tanda y el intervalo del scheduler.
func (s *McpServer) distillBatchOnce(ctx context.Context, batch int) {
	rep, err := s.runDistillBatch(ctx, batch, false)
	if err != nil {
		logx.Error("scheduler: auto-drain del acervo falló", "error", err)
		return
	}
	// Sólo se anuncia cuando HUBO trabajo (como el grafo): un log por tick con 0 en un daemon que vive
	// días es ruido que entierra la línea que importa.
	if rep.Distilled > 0 || rep.Cards > 0 {
		logx.Info("scheduler: acervo destilado", "blobs", rep.Distilled, "tarjetas", rep.Cards, "quedan", rep.Remaining)
	}
}
