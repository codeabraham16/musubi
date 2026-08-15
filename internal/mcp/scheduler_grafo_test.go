package mcp

import (
	"context"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// Un intervalo <= 0 tiene que APAGAR el ciclo, no correrlo una vez ni colgarse. Es el dial de
// apagado, y si no vuelve el arranque del daemon se queda esperando para siempre.
func TestSchedulerDelGrafoApagadoVuelveEnseguida(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	for _, intervalo := range []time.Duration{0, -time.Hour} {
		listo := make(chan struct{})
		go func() {
			s.RunCodeGraphScheduler(context.Background(), intervalo)
			close(listo)
		}()
		select {
		case <-listo:
		case <-time.After(2 * time.Second):
			t.Fatalf("con interval=%v el scheduler no volvió: el daemon quedaría colgado", intervalo)
		}
	}
}

// Cancelar el contexto tiene que cortar el ciclo. Sin esto, el apagado del daemon dejaría una
// goroutine tomando el candado del despacho después de que todo lo demás se fue.
func TestSchedulerDelGrafoMuereConSuContexto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() {
		s.RunCodeGraphScheduler(ctx, time.Hour)
		close(listo)
	}()
	cancel()
	select {
	case <-listo:
	case <-time.After(2 * time.Second):
		t.Fatal("el scheduler ignoró la cancelación del contexto")
	}
}

// EL INVARIANTE QUE MÁS IMPORTA: una corrida es BEST-EFFORT. Sobre un workspace sin código que
// indexar no puede entrar en pánico ni dejarse tomado el candado — si lo dejara, la PRÓXIMA tool
// que despache se colgaría para siempre, y el síntoma aparecería lejos de la causa.
func TestUnaCorridaDelGrafoNoSeQuedaConElCandado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	s.reindexCodeGraphOnce(context.Background())

	liberado := make(chan struct{})
	go func() {
		s.dispatchMu.Lock()
		s.dispatchMu.Unlock()
		close(liberado)
	}()
	select {
	case <-liberado:
	case <-time.After(2 * time.Second):
		t.Fatal("la corrida se quedó con dispatchMu: la próxima tool que despache se cuelga")
	}
}
