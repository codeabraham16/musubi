package embedding

import (
	"context"
	"strings"
	"testing"

	"musubi/internal/config"
)

// lote_test.go cubre EmbedBatch: la garantía de que un lote devuelve EXACTAMENTE tantos vectores
// como textos se pidieron, y que el portero de privacidad no anula el lote sin que nadie se entere.

// espiaLote implementa BatchProvider y devuelve la cantidad que se le indique, para poder simular
// un proveedor que no respeta el contrato.
type espiaLote struct {
	spy
	devolver int  // cuántos vectores devolver (-1 = tantos como textos)
	usoLote  bool // si se lo llamó por el camino de lote
	lotes    [][]string
}

func (e *espiaLote) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	e.usoLote = true
	e.lotes = append(e.lotes, texts)
	n := e.devolver
	if n < 0 {
		n = len(texts)
	}
	out := make([][]float32, n)
	for i := range out {
		out[i] = []float32{0.1, 0.2}
	}
	return out, e.err
}

// ⚠️ L1 — LA GARANTÍA. Un proveedor que devuelve de menos se corta acá, con un error que nombra al
// proveedor. Sin esto, el caller aparea por índice y le escribe a cada observación el vector de
// otra: memoria barajada, sin un solo error en el camino.
func TestEmbedBatchCortaSiElProveedorDevuelveDeMenos(t *testing.T) {
	p := &espiaLote{devolver: 2}
	_, err := EmbedBatch(context.Background(), p, []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("un proveedor que devuelve 2 vectores para 3 textos DEBE cortar el lote")
	}
	if !strings.Contains(err.Error(), "spy") {
		t.Errorf("el error debería nombrar al proveedor para que se sepa a quién reclamarle: %v", err)
	}
}

// L2 — También corta si devuelve de MÁS: la garantía es sobre la cuenta exacta, no sobre «faltan».
func TestEmbedBatchCortaSiDevuelveDeMas(t *testing.T) {
	p := &espiaLote{devolver: 4}
	if _, err := EmbedBatch(context.Background(), p, []string{"a", "b", "c"}); err == nil {
		t.Fatal("devolver de más también desalinea el apareo por índice")
	}
}

// L3 — Un proveedor SIN lote nativo no falla: cae al bucle y mantiene la misma garantía de cuenta.
// Es lo que hace que Noop, Static y cualquier implementador viejo sigan andando sin tocarlos.
func TestEmbedBatchCaeAlBucleSinLoteNativo(t *testing.T) {
	p := &spy{}
	out, err := EmbedBatch(context.Background(), p, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("el bucle devolvió %d vectores para 3 textos", len(out))
	}
	if len(p.got) != 3 {
		t.Errorf("el bucle debería llamar al proveedor una vez por texto, llamó %d", len(p.got))
	}
}

// ⚠️ L4 — EL TEST QUE ATAJA LA FALLA MÁS CARA, porque es la que NO se nota. El portero envuelve al
// proveedor en el constructor, así que quien pide un lote ve al `guarded`, no al Ollama de adentro.
// Si `guarded` dejara de implementar BatchProvider, EmbedBatch caería al bucle: nada falla, nada
// se loguea, y la mejora simplemente no ocurre. Un test verde sobre una mejora que no pasa.
func TestElPorteroNoAnulaElLote(t *testing.T) {
	interno := &espiaLote{devolver: -1}
	g, err := newGuarded(interno, config.GatewayModeScrub)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := g.(BatchProvider); !ok {
		t.Fatal("el portero dejó de implementar BatchProvider: el lote se pierde en silencio")
	}

	out, err2 := EmbedBatch(context.Background(), g, []string{"uno", "dos", "tres"})
	err = err2
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("esperaba 3 vectores, obtuve %d", len(out))
	}
	if !interno.usoLote {
		t.Error("el portero no reenvió por el camino de LOTE: el margen medido se pierde")
	}
	if len(interno.lotes) != 1 || len(interno.lotes[0]) != 3 {
		t.Errorf("el lote llegó partido al proveedor: %v", interno.lotes)
	}
}

// L5 — Y el portero sigue TAPANDO en el lote: la privacidad no se relaja por embeber de a varios.
func TestElPorteroTapaCadaTextoDelLote(t *testing.T) {
	interno := &espiaLote{devolver: -1}
	g, err := newGuarded(interno, config.GatewayModeScrub)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := EmbedBatch(context.Background(), g, []string{"hola", tSecretoAWS, "chau"}); err != nil {
		t.Fatal(err)
	}
	if len(interno.lotes) != 1 {
		t.Fatalf("esperaba un lote, hubo %d", len(interno.lotes))
	}
	for _, txt := range interno.lotes[0] {
		if strings.Contains(txt, tSecretoAWS) {
			t.Error("el secreto llegó CRUDO al proveedor por el camino del lote")
		}
	}
}
