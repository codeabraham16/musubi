package mcp

import (
	"context"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// Tests del AUTO-DRAIN del acervo (pilar Musubi Renaissance, el "molino continuo"): RunDistillScheduler +
// distillBatchOnce. El scheduler es no-op sin motor de cognición; con motor, destila una tanda por tick.

// TestDistillSchedulerNoOpSinCognicion: sin motor (NoopProvider por default), RunDistillScheduler retorna
// de inmediato — así es seguro lanzarlo desde cualquier entrypoint (un daemon local sin cognición no hace
// nada). Es el mismo contrato que RunOutboxScheduler sin syncClient.
func TestDistillSchedulerNoOpSinCognicion(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	done := make(chan struct{})
	go func() { s.RunDistillScheduler(context.Background(), time.Millisecond, 3); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunDistillScheduler sin motor de cognición debía retornar de inmediato (no-op)")
	}
}

// TestDistillBatchOnceDestila: con motor, un tick del auto-drain destila el backlog pendiente (escribe la
// tarjeta y marca el blob), sin principal y sin tomar el candado del despacho de entrada.
func TestDistillBatchOnceDestila(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `[{"slug":"jerarquia","content":"una sola cosa manda por pantalla"}]`}
	seedBlob(t, s, "b1", "ingested/article/aaa", "un artículo de diseño")

	s.distillBatchOnce(context.Background(), 5)

	if _, found, _ := s.engine.LatestObservationByTopicInProject("design-corpus/jerarquia", designCorpusScope); !found {
		t.Error("el auto-drain no escribió la tarjeta destilada")
	}
	if n, _ := s.engine.CountObservationsMissingRelation(designCorpusScope, distillRawPrefix, distillMarker); n != 0 {
		t.Errorf("tras el auto-drain no debía quedar backlog; quedan %d", n)
	}
}
