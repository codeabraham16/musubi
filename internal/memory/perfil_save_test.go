package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// perfil_save_test.go NO es un test de invariantes: es un instrumento de medición que se corre a
// mano contra una COPIA de una base real, para partir el costo de un guardado en sus etapas.
//
// Nace de una medición del ledger (2026-08-11): `musubi_save_observation` promedia 4.642 ms en el
// cerebro central y 1.822 ms en el repo local. El central tiene embedder de red (bge-m3 por ollama,
// 2.786 ms medidos para un texto de 3.500 caracteres) y el local usa el estático en proceso — así
// que ~1,8 s NO son el embedder y aparecen en las dos máquinas. Esto busca esos 1,8 s.
//
// Se saltea si no está la copia. Correr con:
//
//	MUSUBI_PERF_DB=<carpeta que contiene .musubi/> go test ./internal/memory -run TestPerfilDeGuardado -v
func TestPerfilDeGuardado(t *testing.T) {
	raiz := os.Getenv("MUSUBI_PERF_DB")
	if raiz == "" {
		t.Skip("sin MUSUBI_PERF_DB: este perfil corre a mano contra una copia de una base real")
	}

	e, err := NewDbEngine(raiz)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	var obs, emb int
	_ = e.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&obs)
	_ = e.db.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&emb)
	t.Logf("corpus: %d observaciones · %d embeddings", obs, emb)

	// El costo del FTS depende de cuántos términos DISTINTOS tenga el texto, así que un lorem
	// repetido mentiría: se arma con vocabulario variado, como una nota real.
	texto := func(chars int) string {
		var sb strings.Builder
		for i := 0; sb.Len() < chars; i++ {
			fmt.Fprintf(&sb, "decision%d sobre el modulo%d del cerebro con su motivo y su medicion. ", i, i%37)
		}
		return sb.String()[:chars]
	}

	// ── LA CURVA ──────────────────────────────────────────────────────────────────────────
	// Lo que importa no es el número de un caso, es CÓMO ESCALA con el largo de la nota: si
	// crece más que lineal, el problema no es "está lento" sino "se rompe con las notas largas",
	// que son justo las que valen la pena guardar.
	t.Logf("")
	t.Logf("COSTO DEL POOL LEXICO SEGUN EL LARGO DE LA NOTA")
	for _, n := range []int{500, 1500, 3500, 8000, 18000} {
		fila := obsRow{id: "x", content: texto(n)}
		inicio := time.Now()
		cands, err := e.lexicalConflictCandidates(fila, 50)
		if err != nil {
			t.Fatalf("pool lexico (%d ch): %v", n, err)
		}
		ms := float64(time.Since(inicio).Microseconds()) / 1000
		t.Logf("  %6d caracteres -> %8.1f ms   (%d candidatas · %.2f ms por cada 100 ch)",
			n, ms, len(cands), ms/float64(n)*100)
	}
	t.Logf("")

	contenido := texto(3500)
	t.Logf("contenido de prueba: %d caracteres", len(contenido))

	medir := func(nombre string, f func() error) time.Duration {
		t.Helper()
		inicio := time.Now()
		if err := f(); err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		d := time.Since(inicio)
		t.Logf("  %-34s %8.1f ms", nombre, float64(d.Microseconds())/1000)
		return d
	}

	ctx := context.Background()

	// 1) La escritura sola, SIN vector y SIN detección. Es el piso de lo que cuesta persistir.
	id := fmt.Sprintf("perf-%d", time.Now().UnixNano())
	escritura := medir("escritura (sin vector, sin deteccion)", func() error {
		return e.SaveObservation(id, "perf/medicion", contenido, nil)
	})

	// 2) La detección de conflictos sobre esa misma fila, que es lo que corre DESPUÉS de guardar
	//    en cada save y cuyo resultado sólo sirve para contarlo en la respuesta.
	var rels []ObsRelation
	deteccion := medir("DetectRelations (post-guardado)", func() error {
		var err error
		rels, err = e.DetectRelations(id, ConflictOptions{})
		return err
	})
	t.Logf("  (la deteccion devolvio %d relacion(es))", len(rels))

	// 3) Las dos mitades de la detección, por separado.
	src, ok, err := e.loadObsRow(id)
	if err != nil || !ok {
		t.Fatalf("loadObsRow: %v (ok=%v)", err, ok)
	}
	lexico := medir("  |- pool lexico (FTS del contenido)", func() error {
		_, err := e.lexicalConflictCandidates(src, 50)
		return err
	})
	var vec []float32
	semantico := medir("  |- pool semantico (busqueda vectorial)", func() error {
		vec, _ = e.observationVector(id)
		if vec == nil {
			return nil
		}
		_, err := e.SearchObservations(ctx, vec, 50)
		return err
	})
	if vec == nil {
		t.Logf("  (sin vector para esta fila: el pool semantico no corrio)")
	}

	total := escritura + deteccion
	t.Logf("")
	t.Logf("REPARTO — total %0.1f ms", float64(total.Microseconds())/1000)
	t.Logf("  escritura   %5.1f %%", 100*float64(escritura)/float64(total))
	t.Logf("  deteccion   %5.1f %%   (lexico %.0f ms · semantico %.0f ms)",
		100*float64(deteccion)/float64(total),
		float64(lexico.Microseconds())/1000, float64(semantico.Microseconds())/1000)
}
