package memory

import "testing"

// M3: AutoEmbedBackfill cierra SOLO el hueco de procedencia, en background y sin intervención
// manual. Sin él, cambiar de modelo (p. ej. por el checksum de N1) APAGA el recall semántico hasta
// que alguien corra `musubi embed backfill` a mano.

// M3.a — hay observaciones sin vector del modelo actual ⇒ las re-embebe (en background).
func TestAutoEmbedBackfillClosesProvenanceGap(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"h1", "h2", "h3"} {
		if err := e.SaveObservation(id, "t/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	e.SetVectorModelID("static:tabla@aaaa")

	if n, err := e.countStaleEmbeddings(); err != nil || n != 3 {
		t.Fatalf("esperaba 3 pendientes, obtuve n=%d err=%v", n, err)
	}

	e.AutoEmbedBackfill(fixedEmbed)
	e.bgWG.Wait() // el backfill corre en background: esperarlo como haría Close()

	if n := countEmbeddingsWithModel(t, e, "static:tabla@aaaa"); n != 3 {
		t.Errorf("esperaba 3 vectores con la procedencia actual tras el auto-backfill, obtuve %d", n)
	}
	if n, err := e.countStaleEmbeddings(); err != nil || n != 0 {
		t.Errorf("tras el auto-backfill no debería quedar nada pendiente, obtuve n=%d err=%v", n, err)
	}
}

// N1+M3 juntos: un cambio de CHECKSUM (re-destilación in-place) deja los vectores viejos con otra
// procedencia; el auto-backfill los re-embebe con la nueva. Es el escenario real del upgrade.
func TestAutoEmbedBackfillReembedsAfterChecksumChange(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b"} {
		if err := e.SaveObservation(id, "t/x", "contenido "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Modelo viejo: tabla re-destilada ⇒ mismo basename, checksum distinto.
	e.SetVectorModelID("static:tabla@viejo")
	e.AutoEmbedBackfill(fixedEmbed)
	e.bgWG.Wait()
	if n := countEmbeddingsWithModel(t, e, "static:tabla@viejo"); n != 2 {
		t.Fatalf("precondición: esperaba 2 vectores con el modelo viejo, obtuve %d", n)
	}

	// Tras re-destilar: el checksum cambia ⇒ los vectores viejos quedan excluidos por procedencia.
	e.SetVectorModelID("static:tabla@nuevo")
	if n, err := e.countStaleEmbeddings(); err != nil || n != 2 {
		t.Fatalf("con el checksum nuevo las 2 obs deben quedar pendientes, obtuve n=%d err=%v", n, err)
	}

	e.AutoEmbedBackfill(fixedEmbed)
	e.bgWG.Wait()

	if n := countEmbeddingsWithModel(t, e, "static:tabla@nuevo"); n != 2 {
		t.Errorf("esperaba 2 vectores re-embebidos con el checksum nuevo, obtuve %d", n)
	}
	if n := countEmbeddingsWithModel(t, e, "static:tabla@viejo"); n != 0 {
		t.Errorf("los vectores viejos debían quedar sobrescritos (INSERT OR REPLACE), quedan %d", n)
	}
}

// M3.b — sin hueco, es un no-op (el caso común en cada arranque: no debe re-embeber ni hacer ruido).
func TestAutoEmbedBackfillNoopWhenNoGap(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t/x", "contenido", nil); err != nil {
		t.Fatal(err)
	}
	e.SetVectorModelID("static:tabla@aaaa")
	e.AutoEmbedBackfill(fixedEmbed)
	e.bgWG.Wait()

	// Segunda corrida: ya no hay pendientes ⇒ no debe re-embeber (el embedder ni se llama).
	calls := 0
	counting := func(s string) ([]float32, error) { calls++; return fixedEmbed(s) }
	e.AutoEmbedBackfill(counting)
	e.bgWG.Wait()

	if calls != 0 {
		t.Errorf("sin hueco de procedencia el auto-backfill no debe embeber nada, hubo %d llamadas", calls)
	}
}

// M3 (R11) — sin embedder nombrado no hay semántica que backfillear: no-op, sin error.
func TestAutoEmbedBackfillNoopWithoutModelID(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t/x", "contenido", nil); err != nil {
		t.Fatal(err)
	}
	calls := 0
	e.AutoEmbedBackfill(func(s string) ([]float32, error) { calls++; return fixedEmbed(s) })
	e.bgWG.Wait()
	if calls != 0 {
		t.Errorf("sin vectorModelID no debe embeber nada, hubo %d llamadas", calls)
	}
}

// M3.c — engine ya cerrado ⇒ spawnBackground no lanza trabajo (no hay use-after-close del *sql.DB).
func TestAutoEmbedBackfillDoesNotSpawnWhenClosed(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("x", "t/x", "contenido", nil); err != nil {
		t.Fatal(err)
	}
	e.SetVectorModelID("static:tabla@aaaa")

	// Simular el estado cerrado sin cerrar la base (el test aún debe poder consultar).
	e.lifecycleMu.Lock()
	e.closed = true
	e.lifecycleMu.Unlock()

	calls := 0
	e.AutoEmbedBackfill(func(s string) ([]float32, error) { calls++; return fixedEmbed(s) })
	e.bgWG.Wait()

	if calls != 0 {
		t.Errorf("con el engine cerrado no debe lanzarse el backfill, hubo %d llamadas", calls)
	}

	e.lifecycleMu.Lock()
	e.closed = false // restaurar para que el cleanup del test cierre limpio
	e.lifecycleMu.Unlock()
}
