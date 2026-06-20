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
