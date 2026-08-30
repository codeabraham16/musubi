package mcp

// methods_design_abstencion_test.go — invariantes de Musubi Renaissance F3.
//
// Defienden que el motor sepa cuándo NO sabe. Medido el 2026-08-29: «receta de empanadas» devolvía
// seis patrones de diseño con `degraded` apagado, igual que un pedido legítimo — siete de siete
// consultas basura entraron con confianza total. La separación existía (basura 0,362–0,442; pedidos
// reales 0,533–0,558) y nadie trazaba la línea.
//
// El embebedor de prueba de abajo NO simula calidad de recuperación: produce similitudes CONTROLADAS
// para poder ejercitar la aritmética del piso. Medir estabilidad de paráfrasis o precisión con un
// embebedor falso sería medir al embebedor falso — eso vive en la sonda, contra bge-m3 real.

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// embebedorPorAngulo mapea cada texto a un vector unitario en 2D según un ángulo elegido por su
// contenido. Así la similitud coseno entre dos textos es exactamente cos(θ1−θ2) y se puede pedir
// "estos dos a 0,40" o "estos dos a 0,90" sin depender de ningún modelo.
type embebedorPorAngulo struct{ angulos map[string]float64 }

func (e *embebedorPorAngulo) Name() string    { return "fake-angulo" }
func (e *embebedorPorAngulo) Dimensions() int { return 2 }
func (e *embebedorPorAngulo) Embed(_ context.Context, text string) ([]float32, error) {
	ang := 0.0
	for marca, a := range e.angulos {
		if strings.Contains(text, marca) {
			ang = a
			break
		}
	}
	return []float32{float32(math.Cos(ang)), float32(math.Sin(ang))}, nil
}

// motorConSimilitudes arma un servidor donde cada observación queda a una similitud CONOCIDA de la
// consulta. `sims` es topic → similitud deseada contra el pedido "CONSULTA".
func motorConSimilitudes(t *testing.T, sims map[string]float64) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")

	angulos := map[string]float64{"CONSULTA": 0}
	i := 0
	for topic, sim := range sims {
		if sim > 1 {
			sim = 1
		}
		angulos["MARCA"+itoaSim(i)] = math.Acos(sim)
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "obs"+itoaSim(i), topic,
			"MARCA"+itoaSim(i)+" patrón de diseño sobre tablas y color.", 0.5, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
		i++
	}
	emb := &embebedorPorAngulo{angulos: angulos}
	s := NewMcpServer(engine, t.TempDir(), emb)
	// Los vectores se escriben al guardar sólo si el embebedor estaba puesto; acá se sembró antes, así
	// que se completan con el backfill — el mismo camino que usa el motor en producción.
	engine.SetVectorModelID("fake-angulo")
	if _, err := engine.EmbedBackfill(func(txts []string) ([][]float32, error) {
		out := make([][]float32, 0, len(txts))
		for _, txt := range txts {
			v, err := emb.Embed(t.Context(), txt)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}); err != nil {
		t.Fatal(err)
	}
	return s
}

func itoaSim(i int) string { return string(rune('a' + i)) }

// I-ABS1 · lo que no llega al piso no se sirve, y la abstención se declara con su causa.
func TestDesignAbstieneBajoElPiso(t *testing.T) {
	// Todos los candidatos por DEBAJO del piso (0,48): el motor tiene que abstenerse.
	bajo := motorConSimilitudes(t, map[string]float64{
		"design-corpus/uno": 0.40, "design-corpus/dos": 0.35, "design-corpus/tres": 0.20,
	})
	b := callDesign(t, bajo, nil, "CONSULTA", "web")
	if len(b.Corpus) != 0 {
		t.Errorf("todo estaba bajo el piso y sirvió %d patrones de relleno", len(b.Corpus))
	}
	if !b.Degraded || b.DegradedReason != bajoUmbral {
		t.Errorf("esperaba degraded con causa %q; got degraded=%v causa=%q", bajoUmbral, b.Degraded, b.DegradedReason)
	}

	// Con candidatos POR ENCIMA del piso, sirve normal y no degrada.
	alto := motorConSimilitudes(t, map[string]float64{
		"design-corpus/uno": 0.90, "design-corpus/dos": 0.85,
	})
	b2 := callDesign(t, alto, nil, "CONSULTA", "web")
	if len(b2.Corpus) == 0 {
		t.Error("con candidatos sobre el piso tendría que servir material")
	}
	if b2.Degraded || b2.DegradedReason != "" {
		t.Errorf("no debería degradar; degraded=%v causa=%q", b2.Degraded, b2.DegradedReason)
	}

	// Y el piso FILTRA de a uno: mezclando altos y bajos, sólo pasan los altos.
	mixto := motorConSimilitudes(t, map[string]float64{
		"design-corpus/bueno": 0.90, "design-corpus/malo": 0.30, "design-corpus/peor": 0.10,
	})
	b3 := callDesign(t, mixto, nil, "CONSULTA", "web")
	if len(b3.Corpus) != 1 {
		t.Errorf("esperaba que pasara sólo el candidato sobre el piso; pasaron %d", len(b3.Corpus))
	}
	for _, h := range b3.Corpus {
		if h.Similarity < designSimilitudMinima {
			t.Errorf("se sirvió un hit por debajo del piso: %s con %.3f", h.Topic, h.Similarity)
		}
	}
}

// SABOTAJE de I-ABS1 en la otra dirección: si el piso fuera inofensivo, subirlo por encima de TODOS
// los candidatos de un pedido bueno no cambiaría nada. Tiene que abstener igual — así el test no pasa
// por casualidad de los números elegidos.
func TestDesignElPisoMuerdeEnLasDosDirecciones(t *testing.T) {
	casos := []struct {
		nombre  string
		sim     float64
		abstien bool
	}{
		{"justo por debajo del piso", designSimilitudMinima - 0.02, true},
		{"justo por encima del piso", designSimilitudMinima + 0.02, false},
	}
	for _, c := range casos {
		s := motorConSimilitudes(t, map[string]float64{"design-corpus/x": c.sim})
		b := callDesign(t, s, nil, "CONSULTA", "web")
		abstuvo := b.Degraded && b.DegradedReason == bajoUmbral
		if abstuvo != c.abstien {
			t.Errorf("%s (sim≈%.2f): abstuvo=%v, esperaba %v", c.nombre, c.sim, abstuvo, c.abstien)
		}
	}
}

// I-ABS2 · el modo de recuperación siempre se declara. La caída silenciosa a búsqueda léxica —con el
// campo `similarity` desapareciendo sin explicación— era el otro silencio de esta capa.
func TestDesignSiempreDeclaraConQueBusco(t *testing.T) {
	conEmbebedor := motorConSimilitudes(t, map[string]float64{"design-corpus/x": 0.90})
	if b := callDesign(t, conEmbebedor, nil, "CONSULTA", "web"); b.Retrieval != recuperacionSemantica {
		t.Errorf("con embebedor esperaba %q; got %q", recuperacionSemantica, b.Retrieval)
	}

	sinEmbebedor, e := bancoDesign(t) // NoopProvider ⇒ camino léxico
	sembrarAtaque(t, e, designCorpusScope, "c1", "design-corpus/tablas",
		"En tablas densas, filas compactas y números tabulares.", 0.5)
	b := callDesign(t, sinEmbebedor, nil, "tablas densas", "web")
	if b.Retrieval != recuperacionLexica {
		t.Errorf("sin embebedor esperaba %q; got %q", recuperacionLexica, b.Retrieval)
	}
	// I-ABS4 · por FTS no hay puntaje que comparar, así que el piso no corre y "bajo_umbral" no se
	// puede declarar ahí: sería inventar una medición que no se hizo.
	if b.DegradedReason == bajoUmbral {
		t.Error("no se puede declarar bajo_umbral por el camino léxico: no hay similitud que comparar")
	}

	// Y también se declara cuando NO hay nada: abstener no puede dejar el campo vacío.
	vacio, _ := bancoDesign(t)
	if v := callDesign(t, vacio, nil, "cualquier cosa", "web"); v.Retrieval == "" {
		t.Error("el modo de recuperación quedó vacío al abstenerse")
	}
}

// I-ABS3 · abstenerse no rompe el brief. Decir «no tengo material específico para esto» no es lo
// mismo que no devolver nada: el núcleo, la precedencia y la marca siguen sirviendo.
func TestDesignAbstenerseNoRompeElBrief(t *testing.T) {
	s := motorConSimilitudes(t, map[string]float64{"design-corpus/x": 0.20})
	b := callDesign(t, s, nil, "CONSULTA", "web")

	if !b.Degraded {
		t.Fatal("esperaba abstención")
	}
	for nombre, bloque := range map[string]string{
		"principles": b.Principles, "precedence": b.Precedence,
		"material_note": b.MaterialNote, "brand": b.Brand, "emit": b.Emit, "instructions": b.Instructions,
	} {
		if strings.TrimSpace(bloque) == "" {
			t.Errorf("al abstenerse se perdió el bloque %q, que no depende del acervo", nombre)
		}
	}
	// Y el brief sigue siendo JSON válido y acotado.
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw)/4 > designBriefBudget {
		t.Errorf("un brief abstenido no debería exceder el presupuesto")
	}
}
