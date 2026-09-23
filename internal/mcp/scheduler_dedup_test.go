package mcp

import (
	"context"
	"math"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestSharpenBatchOnceRespetaGate: el afilado de fondo es OPT-IN (maintenance.AutoSharpenPairs). Con el
// gate en 0 (default) es no-op aunque haya gemelas y el juez diga MERGE; encendido, afila una tanda.
func TestSharpenBatchOnceRespetaGate(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.cognition = &fakeCognition{answer: `{"verdict":"MERGE"}`}
	seedCard(t, s, "c1", "contraste-a", "contraste mínimo 4.5:1", []float32{1, 0, 0, 0})
	seedCard(t, s, "c2", "contraste-b", "el texto necesita 4.5:1", []float32{0.98, 0.2, 0, 0})

	admin := &Principal{Name: "root", Role: RoleAdmin}

	// Gate apagado (default 0): no-op. El par sigue candidato.
	s.maintenance.AutoSharpenPairs = 0
	s.sharpenBatchOnce(context.Background())
	rep, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep.Scanned != 1 {
		t.Fatalf("con el gate apagado no debía afilarse nada; scanned=%d", rep.Scanned)
	}

	// Gate encendido: afila y la gemela queda archivada.
	s.maintenance.AutoSharpenPairs = 5
	s.sharpenBatchOnce(context.Background())
	rep2, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.9, "dry_run": true})
	if rep2.Scanned != 0 {
		t.Errorf("con el gate encendido la gemela debía fusionarse; scanned=%d", rep2.Scanned)
	}
}

// TestElAfiladoDeFondoAlcanzaLaFranjaDondeViveUnaGemelaMedida — el piso POR DEFAULT, que es el que corre.
//
// EL EJE QUE NADIE VARIABA. Todas las pruebas del afilador pasan `floor: 0.9` a mano, así que el piso
// por default —el único que usa el afilado de fondo, y el que en el cerebro corre solo con
// `auto_sharpen_pairs: 2`— no lo ejercitaba nadie. Podía valer 0.5 o 0.99 y la suite seguía verde.
//
// Y VALÍA LO QUE NO DEBÍA. Hasta el 2026-09-23 era 0.84, contra un comentario que dice que las gemelas
// viven desde 0.82. Medido ese día con la función real sobre el acervo del cerebro: 33 pares en la
// franja 0.82-0.84 que no llegaban nunca al juez, y justo arriba de la raya el juez fusionaba 6 de 8.
//
// EL COSENO DE ESTA PRUEBA ES UN HECHO, NO UN VALOR CÓMODO: 0.833 es lo que midió el par
// `contraste-color-texto` | `contraste-4-5-a-1`, que por sus tópicos es la misma gemela que el
// paquete usa de ejemplo. Por eso se clava y no se deriva del piso: derivarlo («el piso más un
// poquito») dejaría a la prueba midiéndose a sí misma, y cualquier piso la pasaría.
//
// LA OTRA DIRECCIÓN TAMBIÉN SE PRUEBA: bajar el piso es un cambio LEGÍTIMO (manda más pares al juez,
// que es quien pone la precisión) y esta guarda no tiene que castigarlo. Si lo castigara, estaría
// clavando un número en vez de cuidar la franja.
//
// Sabotaje que la hace fallar: devolverle al piso el 0.84 que tenía.
// arnes: archivo="internal/mcp/methods_dedup.go"
// arnes: de="\tdedupDefaultFloor = 0.82"
// arnes: a="\tdedupDefaultFloor = 0.84"
// arnes: arreglo_de="\tdedupDefaultFloor = 0.82"
// arnes: arreglo_a="\tdedupDefaultFloor = 0.80"
func TestElAfiladoDeFondoAlcanzaLaFranjaDondeViveUnaGemelaMedida(t *testing.T) {
	const cosenoMedido = 0.833

	a := []float32{1, 0, 0, 0}
	b := []float32{cosenoMedido, float32(math.Sqrt(1 - cosenoMedido*cosenoMedido)), 0, 0}

	// EL CONTROL DE QUE EL FIXTURE DICE LO QUE ESTA PRUEBA CREE. Los vecinos de este archivo usan
	// pares a 0.98, muy arriba de cualquier piso; si este vector saliera parecido, la prueba pasaría
	// sin haber tocado la franja que vino a cuidar.
	c, err := memory.CosineSimilarity(a, b)
	if err != nil || math.Abs(float64(c)-cosenoMedido) > 1e-3 {
		t.Fatalf("el fixture no tiene el coseno medido: dio %v (err %v), se esperaba %.3f", c, err, cosenoMedido)
	}

	s := newTestServer(t, embedding.NoopProvider{})
	fake := &fakeCognition{answer: `{"verdict":"MERGE"}`}
	s.cognition = fake
	seedCard(t, s, "c1", "contraste-color-texto", "el texto necesita contraste suficiente contra el fondo", a)
	seedCard(t, s, "c2", "contraste-4-5-a-1", "contraste mínimo 4.5:1 para texto normal", b)

	// Por el camino del scheduler y no por la herramienta: es el que en el cerebro corre solo, y el
	// único que no deja pasar un piso explícito.
	s.maintenance.AutoSharpenPairs = 5
	s.sharpenBatchOnce(context.Background())

	if !fake.called {
		t.Fatalf("el afilado de fondo no le consultó al juez un par a coseno %.3f: el piso por default (%v) "+
			"corta la franja donde el acervo del cerebro tiene gemelas medidas, y el afilado queda ocioso "+
			"con trabajo pendiente", cosenoMedido, dedupDefaultFloor)
	}
	admin := &Principal{Name: "root", Role: RoleAdmin}
	rep, _ := callSharpen(t, s, admin, map[string]any{"floor": 0.5, "dry_run": true})
	if rep.Scanned != 0 {
		t.Errorf("el juez dijo MERGE y la gemela sigue candidata (scanned=%d): se consultó pero no se fusionó", rep.Scanned)
	}
}
