package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// TestRunScheduledMaintenanceThrottle verifica que el ciclo de fondo respeta el throttle
// (T5.1): corre la primera vez (sin marca previa) y se saltea dentro del intervalo.
func TestRunScheduledMaintenanceThrottle(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{}) // AutoIntervalHours=24 (default)

	ran, _, err := s.RunScheduledMaintenance()
	if err != nil {
		t.Fatalf("corrida #1 error: %v", err)
	}
	if !ran {
		t.Fatal("la primera corrida (sin marca previa) debió correr")
	}

	ran2, _, err := s.RunScheduledMaintenance()
	if err != nil {
		t.Fatalf("corrida #2 error: %v", err)
	}
	if ran2 {
		t.Error("la segunda corrida inmediata no debió correr (throttle de 24h)")
	}
}

// TestAutoMaintainAfterSaves verifica el trigger por volumen (T5.3): al cruzar
// AutoAfterSaves se dispara un mantenimiento async; por debajo del umbral no.
func TestAutoMaintainAfterSaves(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.maintenance.AutoAfterSaves = 3
	s.maintenance.AutoIntervalHours = 0 // sin throttle: el trigger corre el ciclo

	save := func(n int) {
		argBytes, _ := json.Marshal(map[string]interface{}{
			"topic_key": "t", "content": fmt.Sprintf("contenido distinto %d para no deduplicar", n),
		})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_save_observation", Arguments: argBytes})
		if _, e := s.handleToolsCall(context.Background(), params); e != nil {
			t.Fatalf("save %d error: %+v", n, e)
		}
	}
	maintRan := func() bool {
		v, _, _ := s.engine.GetMeta("last_maintenance")
		return v != ""
	}

	// Bajo el umbral (2 de 3): el contador sube pero NO se resetea. Se chequea el INVARIANTE
	// DETERMINISTA (saveCount), no un side-effect sensible al timing: maybeTriggerMaintenance hace
	// saveCount.Store(0) SINCRÓNICAMENTE sólo al cruzar el umbral, así que count==2 prueba que el
	// trigger no corrió (antes esto se verificaba con un sleep+maintRan, que flakeaba bajo carga).
	save(1)
	save(2)
	if got := s.saveCount.Load(); got != 2 {
		t.Fatalf("con 2 saves bajo el umbral, saveCount=%d (esperaba 2: sin reset ⇒ sin trigger)", got)
	}
	if maintRan() {
		t.Fatal("con 2 saves (umbral 3) no debía haberse disparado mantenimiento")
	}

	// Cruza el umbral: el trigger resetea el contador (sincrónico, prueba determinista de que
	// disparó) y corre el ciclo en goroutine (async).
	save(3)
	if got := s.saveCount.Load(); got != 0 {
		t.Fatalf("al cruzar el umbral, saveCount debe resetear a 0 (prueba determinista del trigger), got %d", got)
	}
	// El ciclo corre async: esperar (acotado) a que marque last_maintenance.
	ran := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if maintRan() {
			ran = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ran {
		t.Error("3 saves con auto_after_saves=3 debió disparar el mantenimiento")
	}
}

// TestMaintenanceSchedulerRunsAndStops verifica que el scheduler de fondo (T5.2) corre el
// ciclo por ticker y se detiene al cancelar el ctx, conviviendo con dispatch concurrente
// (lecturas y escrituras) — la serialización por dispatchMu la valida -race en CI.
func TestMaintenanceSchedulerRunsAndStops(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.maintenance.AutoIntervalHours = 0 // 0 ⇒ siempre due: el ticker corre en cada tick.

	dispatch := func(name string, args map[string]interface{}) {
		argBytes, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: name, Arguments: argBytes})
		_, _ = s.handleToolsCall(context.Background(), params)
	}

	ctx, cancel := context.WithCancel(context.Background())
	schedDone := make(chan struct{})
	go func() {
		defer close(schedDone)
		s.RunMaintenanceScheduler(ctx, 2*time.Millisecond)
	}()

	// Dispatch concurrente mientras el scheduler corre: mezcla lecturas (RLock) y
	// escrituras (Lock) contra el Lock exclusivo del mantenimiento.
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				dispatch("musubi_search_keyword", map[string]interface{}{"query": "x"})
			} else {
				dispatch("musubi_save_observation", map[string]interface{}{"topic_key": "t", "content": fmt.Sprintf("observación número %d", n)})
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond) // dejar correr varios ticks

	cancel()
	select {
	case <-schedDone:
	case <-time.After(2 * time.Second):
		t.Fatal("el scheduler no paró tras cancelar el ctx")
	}

	if v, _, _ := s.engine.GetMeta("last_maintenance"); v == "" {
		t.Error("el scheduler debió correr al menos un ciclo (last_maintenance quedó vacío)")
	}
}
