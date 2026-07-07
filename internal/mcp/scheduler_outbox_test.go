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

// Escenario: tope de reintentos transitorios → dead al llegar a max_attempts.
func TestDrainMaxAttemptsGoesDead(t *testing.T) {
	// MaxAttempts=2, backoff 1s: falla, reintenta (attempts→1), falla de nuevo (attemptsSoFar=2≥2) → dead.
	s, stub, closeFn := wireSync(t, config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, MaxAttempts: 2, BackoffBaseSeconds: 1, BackoffMaxSeconds: 5})
	defer closeFn()
	stub.mu.Lock()
	stub.status = http.StatusInternalServerError
	stub.mu.Unlock()

	saveShared(t, s, "obs1")

	// 1er drain: transitorio con margen → pending (attempts=1).
	s.drainOutboxOnce(context.Background())
	if p, _, dead := statsOf(t, s); p != 1 || dead != 0 {
		t.Fatalf("tras 1er fallo esperaba pending=1 dead=0, obtuve pending=%d dead=%d", p, dead)
	}

	// 2do drain (tras el backoff): alcanza el tope → dead.
	time.Sleep(1200 * time.Millisecond)
	s.drainOutboxOnce(context.Background())
	if p, _, dead := statsOf(t, s); p != 0 || dead != 1 {
		t.Errorf("al tope de reintentos esperaba pending=0 dead=1, obtuve pending=%d dead=%d", p, dead)
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
