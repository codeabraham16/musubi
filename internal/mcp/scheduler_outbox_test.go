package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// wireSync arma el server con un SyncClient apuntando al stub y una SyncConfig de test. Devuelve
// el server para drenar y el stub para controlar la respuesta del "central".
func wireSync(t *testing.T, cfg config.SyncConfig) (*McpServer, *centralStub, func()) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	stub := newCentralStub()
	ts := httptest.NewServer(stub)
	t.Setenv("MUSUBI_TEST_TOKEN", "tok")
	cfg.CentralURL = ts.URL
	cfg.AuthTokenEnv = "MUSUBI_TEST_TOKEN"
	cfg.AllowInsecureToken = true
	if cfg.RequestTimeoutSeconds == 0 {
		cfg.RequestTimeoutSeconds = 2
	}
	client, err := NewSyncClient(cfg)
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	s.SetSyncClient(client, cfg)
	return s, stub, ts.Close
}

// saveShared guarda una observación 'shared' (que encola en el outbox por el enqueue transaccional).
func saveShared(t *testing.T, s *McpServer, id string) {
	t.Helper()
	if err := s.engine.SaveObservationTyped(id, "topic", "contenido de "+id, 1.0, "semantic", memory.ScopeShared, nil); err != nil {
		t.Fatalf("SaveObservationTyped: %v", err)
	}
}

func statsOf(t *testing.T, s *McpServer) (pending, sent, dead int) {
	t.Helper()
	p, se, d, err := s.engine.OutboxStats()
	if err != nil {
		t.Fatalf("OutboxStats: %v", err)
	}
	return p, se, d
}

// Escenario: drain exitoso marca sent.
func TestDrainSuccessMarksSent(t *testing.T) {
	s, _, closeFn := wireSync(t, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, MaxAttempts: 5, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	defer closeFn()

	saveShared(t, s, "obs1")
	if p, _, _ := statsOf(t, s); p != 1 {
		t.Fatalf("precondición: esperaba 1 pending, hay %d", p)
	}
	s.drainOutboxOnce(context.Background())
	p, sent, dead := statsOf(t, s)
	if p != 0 || sent != 1 || dead != 0 {
		t.Errorf("tras drain exitoso esperaba pending=0 sent=1 dead=0, obtuve %d/%d/%d", p, sent, dead)
	}
}

// Escenario: offline-first — central caído deja pending con attempts↑; al volver, sent.
func TestDrainOfflineFirstRecovery(t *testing.T) {
	// BackoffBaseSeconds=1 para que la recuperación sea rápida (next_attempt_at = now+1s).
	s, stub, closeFn := wireSync(t, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, MaxAttempts: 5, BackoffBaseSeconds: 1, BackoffMaxSeconds: 5})
	defer closeFn()

	stub.mu.Lock()
	stub.status = http.StatusInternalServerError // central "caído"
	stub.mu.Unlock()

	saveShared(t, s, "obs1")
	s.drainOutboxOnce(context.Background())
	if p, sent, dead := statsOf(t, s); p != 1 || sent != 0 || dead != 0 {
		t.Fatalf("con central caído esperaba pending=1 sent=0 dead=0, obtuve %d/%d/%d", p, sent, dead)
	}

	// El central vuelve; esperar a que venza el backoff y drenar de nuevo.
	stub.mu.Lock()
	stub.status = http.StatusOK
	stub.mu.Unlock()
	time.Sleep(1200 * time.Millisecond)
	s.drainOutboxOnce(context.Background())
	if p, sent, dead := statsOf(t, s); p != 0 || sent != 1 || dead != 0 {
		t.Errorf("tras recuperación esperaba pending=0 sent=1 dead=0, obtuve %d/%d/%d", p, sent, dead)
	}
}

// Escenario: fallo permanente (400) va directo a dead-letter.
func TestDrainPermanentGoesDead(t *testing.T) {
	s, stub, closeFn := wireSync(t, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, MaxAttempts: 5, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	defer closeFn()
	stub.mu.Lock()
	stub.status = http.StatusBadRequest
	stub.mu.Unlock()

	saveShared(t, s, "obs1")
	s.drainOutboxOnce(context.Background())
	if p, sent, dead := statsOf(t, s); p != 0 || sent != 0 || dead != 1 {
		t.Errorf("un 400 debía ir a dead, esperaba 0/0/1, obtuve %d/%d/%d", p, sent, dead)
	}
	// No se reintenta en drains posteriores (sigue dead, no vuelve a pending).
	s.drainOutboxOnce(context.Background())
	if _, _, dead := statsOf(t, s); dead != 1 {
		t.Errorf("una fila dead no debía reintentarse, dead=%d", dead)
	}
}

// Escenario: OFFLINE-FIRST (sync-hardening) — un fallo TRANSITORIO NUNCA va a dead, aunque
// falle muchas más veces que max_attempts. Reintenta indefinidamente con backoff capado, y al
// recuperarse el central, se entrega. (Antes, con la política por-conteo, moría a las 2.)
func TestDrainTransientNeverDies(t *testing.T) {
	// MaxAttempts=2 a propósito: bajo la política vieja, al 2do fallo iba a dead. Ahora NO.
	s, stub, closeFn := wireSync(t, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, MaxAttempts: 2, BackoffBaseSeconds: 1, BackoffMaxSeconds: 2})
	defer closeFn()
	stub.mu.Lock()
	stub.status = http.StatusInternalServerError // central "caído" (transitorio)
	stub.mu.Unlock()

	saveShared(t, s, "obs1")

	// Drenar varias veces superando MaxAttempts: la fila DEBE seguir pending, nunca dead.
	for i := 0; i < 4; i++ {
		s.drainOutboxOnce(context.Background())
		if _, _, dead := statsOf(t, s); dead != 0 {
			t.Fatalf("un fallo transitorio nunca debe ir a dead (iter %d): dead=%d", i, dead)
		}
		time.Sleep(1100 * time.Millisecond) // dejar vencer el backoff para re-reclamar
	}
	if p, sent, dead := statsOf(t, s); p != 1 || sent != 0 || dead != 0 {
		t.Fatalf("tras muchos fallos transitorios esperaba pending=1 sent=0 dead=0, obtuve %d/%d/%d", p, sent, dead)
	}

	// El central vuelve: la fila (que nunca murió) se entrega.
	stub.mu.Lock()
	stub.status = http.StatusOK
	stub.mu.Unlock()
	time.Sleep(2200 * time.Millisecond) // backoff capado a 2s
	s.drainOutboxOnce(context.Background())
	if p, sent, dead := statsOf(t, s); p != 0 || sent != 1 || dead != 0 {
		t.Errorf("tras recuperación esperaba pending=0 sent=1 dead=0, obtuve %d/%d/%d", p, sent, dead)
	}
}

// Escenario: requeue de dead-letter → un dead vuelve a la cola y se entrega al recuperarse.
func TestDrainRequeueRevivesDead(t *testing.T) {
	s, stub, closeFn := wireSync(t, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, MaxAttempts: 5, BackoffBaseSeconds: 1, BackoffMaxSeconds: 2})
	defer closeFn()
	stub.mu.Lock()
	stub.status = http.StatusBadRequest // permanente → dead
	stub.mu.Unlock()

	saveShared(t, s, "obs1")
	s.drainOutboxOnce(context.Background())
	if _, _, dead := statsOf(t, s); dead != 1 {
		t.Fatalf("un 400 debía ir a dead, dead=%d", dead)
	}

	// Requeue: dead → pending. Y con el central sano, se entrega.
	n, err := s.engine.RequeueDeadOutbox()
	if err != nil || n != 1 {
		t.Fatalf("RequeueDeadOutbox esperaba (1,nil), obtuve (%d,%v)", n, err)
	}
	stub.mu.Lock()
	stub.status = http.StatusOK
	stub.mu.Unlock()
	s.drainOutboxOnce(context.Background())
	if p, sent, dead := statsOf(t, s); p != 0 || sent != 1 || dead != 0 {
		t.Errorf("tras requeue+recuperación esperaba 0/1/0, obtuve %d/%d/%d", p, sent, dead)
	}
}

// TestOutboxSchedulerDisabledNoop: sin syncClient, RunOutboxScheduler es un no-op inmediato.
func TestOutboxSchedulerDisabledNoop(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.RunOutboxScheduler(ctx, 10*time.Millisecond) // syncClient nil ⇒ retorna ya
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunOutboxScheduler sin cliente debía retornar de inmediato")
	}
}
