package embedding

import (
	"context"
	"strings"
	"testing"
	"time"
)

// plazo_test.go defiende dos cosas que fallaron en producción el 2026-08-18, cuando
// `embed backfill --all` llenó el log de `context deadline exceeded`: que el plazo de un pedido
// mire cuánto texto lleva, y que el lote no arme pedidos más grandes de lo que el embebedor mastica
// en un tiempo razonable.

// medidas son los tiempos REALES del embebedor del cerebro central (bge-m3, 2026-08-18). Los tests
// se anclan a estos números y no a una constante elegida a gusto: si el plazo no le gana a lo
// medido, el pedido se corta solo.
var medidas = []struct {
	caracteres int
	tardo      time.Duration
}{
	{1000, 800 * time.Millisecond},
	{16000, 8100 * time.Millisecond},
	{48000, 27800 * time.Millisecond},
	{96000, 65500 * time.Millisecond},
}

// ⚠️ P1 — EL PLAZO LE TIENE QUE GANAR A LO MEDIDO, CON MARGEN. Es el test que ataja el error de
// unidades: con µs en vez de ms por carácter el plazo de 96.000 caracteres crecería 0,24 s en vez
// de 192 s, quedaría en los ~30 s de antes y el defecto seguiría vivo detrás de un comentario que
// dice lo contrario.
func TestP1ElPlazoLeGanaALoMedido(t *testing.T) {
	for _, m := range medidas {
		p := plazoPara([]string{strings.Repeat("x", m.caracteres)})
		if p <= m.tardo {
			t.Errorf("%d caracteres tardaron %v medidos y el plazo es %v: se corta solo",
				m.caracteres, m.tardo, p)
		}
		if p < 2*m.tardo {
			t.Errorf("%d caracteres: plazo %v contra %v medidos, menos de 2× de margen en un server compartido",
				m.caracteres, p, m.tardo)
		}
	}
}

// P2 — El plazo CRECE con el texto. Un plazo que no depende del tamaño es el defecto de origen:
// 30 s fijos alcanzaban para ~50.000 caracteres y nadie se enteraba hasta que un lote los pasaba.
func TestP2ElPlazoCreceConElTexto(t *testing.T) {
	chico := plazoPara([]string{strings.Repeat("x", 1000)})
	grande := plazoPara([]string{strings.Repeat("x", 96000)})
	if grande <= chico {
		t.Fatalf("el plazo no mira el tamaño: %v para 1.000 caracteres y %v para 96.000", chico, grande)
	}
	// Y no puede ser MÁS ESTRICTO que el fijo que había: un pedido que antes andaba no puede
	// empezar a fallar por este cambio.
	if chico < 30*time.Second {
		t.Errorf("el plazo de un pedido chico bajó de los 30 s que había: %v", chico)
	}
}

// P3 — Hay un techo. Sin él, el plazo crece con el texto y un embebedor colgado se esperaría casi
// para siempre, sin que nadie sospeche.
func TestP3ElPlazoTieneTecho(t *testing.T) {
	if p := plazoPara([]string{strings.Repeat("x", 100_000_000)}); p != plazoMaximo {
		t.Errorf("un pedido enorme tiene que toparse en %v, dio %v", plazoMaximo, p)
	}
}

// ⚠️ P4 — EL LOTE SE ACOTA POR TEXTO, NO POR CANTIDAD. Es la causa raíz de los timeouts vistos en
// producción: 16 era un tope de CANTIDAD y el costo del embebedor depende del TAMAÑO. Dieciséis
// textos de 6.000 caracteres son 96.000 en un solo pedido, y eso tardó 65,5 s medidos.
func TestP4ElLoteSeAcotaPorTextoNoPorCantidad(t *testing.T) {
	e := &espiaLote{devolver: -1}
	tr := newTroceado(e)

	// 16 textos que YA NO necesitan troceo individual (entran en trozoInicial) pero que juntos
	// pasan holgadamente el tope por pedido.
	textos := make([]string, 16)
	for i := range textos {
		textos[i] = strings.Repeat("x", 6000)
	}
	out, err := EmbedBatch(context.Background(), tr, textos)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 16 {
		t.Fatalf("volvieron %d vectores para 16 textos", len(out))
	}
	if len(e.lotes) < 2 {
		t.Fatalf("los 16 textos (96.000 caracteres) salieron en %d pedido(s): no se acotó por tamaño", len(e.lotes))
	}
	for i, lote := range e.lotes {
		n := 0
		for _, tx := range lote {
			n += len(tx)
		}
		if n > loteMaxChars {
			t.Errorf("el pedido %d llevó %d caracteres, más que el tope de %d", i, n, loteMaxChars)
		}
	}
}

// P5 — Y las tandas conservan el ORDEN. Es lo único que el caller tiene para aparear cada vector
// con su observación: barajarlo acá le escribiría a cada una el embedding de otra, sin un error.
func TestP5LasTandasConservanElOrden(t *testing.T) {
	e := &espiaOrden{}
	tr := newTroceado(e)

	textos := make([]string, 12)
	for i := range textos {
		// 5.000 caracteres: NO necesitan troceo individual (entran en trozoInicial), pero los 12
		// juntos son 60.000 y obligan a partir en tandas. Cada uno arranca con una letra distinta
		// para poder reconocerlo a la vuelta.
		textos[i] = strings.Repeat(string(rune('a'+i)), 5000)
	}
	out, err := EmbedBatch(context.Background(), tr, textos)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(textos) {
		t.Fatalf("volvieron %d vectores para %d textos", len(out), len(textos))
	}
	for i, tx := range textos {
		if out[i][0] != float32(tx[0]) {
			t.Fatalf("el vector %d no es el de su texto: esperaba la marca %v, vino %v",
				i, float32(tx[0]), out[i][0])
		}
	}
}

// espiaOrden devuelve un vector marcado con el PRIMER byte de cada texto, para poder comprobar el
// apareo posición por posición.
type espiaOrden struct{}

func (espiaOrden) Embed(_ context.Context, text string) ([]float32, error) {
	return []float32{float32(text[0])}, nil
}
func (espiaOrden) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = []float32{float32(t[0])}
	}
	return out, nil
}
func (espiaOrden) Dimensions() int { return 1 }
func (espiaOrden) Name() string    { return "orden" }
