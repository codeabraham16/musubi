package cognition

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"musubi/internal/config"
)

// Invariantes de la TELEMETRÍA (F5). Ver specs/dial-y-telemetria-cognicion/spec.md.
// Cada test se verificó FALLANDO al sabotear la implementación (ver tasks.md).

// --- D4: los contadores cuentan lo que pasó ----------------------------------------------------

func TestD4ElPorteroCuentaLlamadasYTapadas(t *testing.T) {
	sp := &spy{answer: "ok"}
	g, err := newGuarded(sp, GatewayModeScrub)
	if err != nil {
		t.Fatalf("newGuarded: %v", err)
	}
	ctx := context.Background()

	// Dos con secreto, una limpia.
	if _, err := g.Ask(ctx, "", "la clave es "+tSecretoAWS); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if _, err := g.Ask(ctx, "", "el token es "+tSecretoGH); err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if _, err := g.Ask(ctx, "", "una pregunta sin nada adentro"); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	st := Stats(g)
	if st.GatewayCalls != 3 {
		t.Errorf("D4: llamadas = %d, esperaba 3", st.GatewayCalls)
	}
	if st.GatewayScrubbed != 2 {
		t.Errorf("D4: con tapado = %d, esperaba 2 (la tercera venía limpia)", st.GatewayScrubbed)
	}
	if st.GatewayBlocked != 0 {
		t.Errorf("D4: en modo scrub no se bloquea nada, contó %d", st.GatewayBlocked)
	}
}

func TestD4CuentaLosBloqueosPorPolitica(t *testing.T) {
	sp := &spy{answer: "no deberías ver esto"}
	g, _ := newGuarded(sp, GatewayModeRefuse)

	if _, err := g.Ask(context.Background(), "", "clave "+tSecretoAWS); !errors.Is(err, ErrSecretsBlocked) {
		t.Fatalf("esperaba ErrSecretsBlocked, obtuve %v", err)
	}
	st := Stats(g)
	if st.GatewayBlocked != 1 {
		t.Errorf("D4: bloqueos = %d, esperaba 1", st.GatewayBlocked)
	}
	if st.GatewayCalls != 1 {
		t.Errorf("D4: una llamada bloqueada SIGUE siendo una llamada; contó %d", st.GatewayCalls)
	}
}

// --- D5: la telemetría nunca contiene un secreto ----------------------------------------------

func TestD5LosContadoresNoGuardanNingunSecreto(t *testing.T) {
	sp := &spy{answer: "ok"}
	g, _ := newGuarded(sp, GatewayModeScrub)
	if _, err := g.Ask(context.Background(), "sistema con "+tSecretoAWS, "y "+tSecretoGH); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	st := Stats(g)
	if len(st.GatewayTypes) == 0 {
		t.Fatalf("el test no está probando nada: no se registró ningún tipo")
	}
	// Se serializa TODA la foto y se busca el secreto adentro. Es la única forma honesta de
	// verificar esto: mirar campo por campo deja pasar el que se agregue mañana.
	blob := fmt.Sprintf("%#v", st)
	for _, sec := range []string{tSecretoAWS, tSecretoGH} {
		if strings.Contains(blob, sec) {
			t.Errorf("FUGA D5: el secreto %q apareció en la telemetría:\n%s", sec, blob)
		}
	}
	// Y sin embargo el TIPO sí tiene que estar: si no, no informa nada.
	var vioTipo bool
	for tipo := range st.GatewayTypes {
		if strings.TrimSpace(tipo) != "" {
			vioTipo = true
		}
	}
	if !vioTipo {
		t.Errorf("D5: se esperaban tipos de secreto clasificados, no una lista vacía")
	}
}

// --- D6: contar no cambia el comportamiento ----------------------------------------------------

func TestD6ContarNoCambiaLaRespuesta(t *testing.T) {
	// Con telemetría (el camino normal, newGuarded le pone stats).
	sp1 := &spy{answer: "respuesta"}
	conStats, _ := newGuarded(sp1, GatewayModeScrub)
	got1, err1 := conStats.Ask(context.Background(), "s", "clave "+tSecretoAWS)

	// Sin telemetría: guarded a mano con stats nil (el nil-guard de gatewayStats).
	sp2 := &spy{answer: "respuesta"}
	sinStats := guarded{inner: sp2, mode: GatewayModeScrub}
	got2, err2 := sinStats.Ask(context.Background(), "s", "clave "+tSecretoAWS)

	if got1 != got2 || (err1 == nil) != (err2 == nil) {
		t.Errorf("D6: contar cambió el resultado: %q/%v vs %q/%v", got1, err1, got2, err2)
	}
	if sp1.gotUser != sp2.gotUser {
		t.Errorf("D6: contar cambió lo que le llega al motor:\n con=%q\n sin=%q", sp1.gotUser, sp2.gotUser)
	}
}

// --- D7: seguro bajo concurrencia (las carreras las verifica la CI con -race) -----------------

func TestD7LosContadoresNoPierdenIncrementos(t *testing.T) {
	// Usa `espia` y NO `spy`: el spy de F1 escribe sus campos sin lock porque nació para tests
	// secuenciales. Llamarlo desde 8 goroutines es una carrera EN EL TEST, no en el portero — la
	// encontró la CI con -race, que es exactamente para lo que la spec dice que sirve.
	e := &espia{}
	g, _ := newGuarded(e, GatewayModeScrub)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				if _, err := g.Ask(ctx, "", "clave "+tSecretoAWS); err != nil {
					t.Errorf("Ask concurrente: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if st := Stats(g); st.GatewayCalls != 200 {
		t.Errorf("D7: se perdieron incrementos: llamadas = %d, esperaba 200", st.GatewayCalls)
	}
}

// --- D8: leer no muta --------------------------------------------------------------------------

func TestD8LeerNoResetaLosContadores(t *testing.T) {
	sp := &spy{answer: "ok"}
	g, _ := newGuarded(sp, GatewayModeScrub)
	if _, err := g.Ask(context.Background(), "", "clave "+tSecretoAWS); err != nil {
		t.Fatalf("Ask: %v", err)
	}

	a := Stats(g)
	b := Stats(g)
	if a.GatewayCalls != 1 || b.GatewayCalls != 1 {
		t.Errorf("FUGA D8: leer reseteó los contadores: primera=%d segunda=%d", a.GatewayCalls, b.GatewayCalls)
	}
}

// --- La foto atraviesa toda la cadena de decoradores -------------------------------------------

func TestLaFotoAtraviesaCacheYPortero(t *testing.T) {
	// Cadena real de fábrica: cached → guarded → motor.
	cfg := config.CognitionConfig{
		Provider: "openai-compat",
		Endpoint: "http://127.0.0.1:9/v1",
	}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, esCache := p.(*cached); !esCache {
		t.Fatalf("esperaba la cadena con caché, obtuve %T", p)
	}
	// El motor real no va a contestar (puerto 9), pero eso no impide que el portero cuente: lo
	// que se mide es el trabajo del portero, no el éxito del motor.
	_, _ = p.Ask(context.Background(), "", "clave "+tSecretoAWS)

	st := Stats(p)
	if st.GatewayCalls != 1 {
		t.Errorf("la foto no llegó hasta el portero a través del caché: llamadas=%d", st.GatewayCalls)
	}
	if st.CacheMisses != 1 {
		t.Errorf("el caché tenía que contar 1 miss, contó %d", st.CacheMisses)
	}
	if st.CacheHits != 0 {
		t.Errorf("no hubo hits posibles, contó %d", st.CacheHits)
	}
}

// Sin cognición no hay nada que medir, y pedirlo no rompe.
func TestStatsConElPilarApagadoDevuelveCeros(t *testing.T) {
	p, err := NewProvider(config.CognitionConfig{})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	st := Stats(p)
	if st.GatewayCalls != 0 || st.CacheHits != 0 || st.RouterEscalations != 0 {
		t.Errorf("con el pilar apagado todo debe dar cero, obtuve %+v", st)
	}
}
