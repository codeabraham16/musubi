package mcp

// ejes_diseno_test.go — invariantes del RUTEO POR EJE (plan de cierre, fase 2, 2026-08-30).
//
// El defecto que esta fase ataca, medido contra el central: dos maneras de pedir lo mismo devolvían
// material distinto (M1 = 0,10 sobre 16 pedidos reales). La causa medida es que dos tarjetas de
// diseño al azar se parecen tanto como una consulta a su mejor resultado, así que la similitud no
// separa nada a esa granularidad. El mismo embebedor SÍ separa 19 ejes: el eje top-1 coincide entre
// paráfrasis el 73 % de las veces contra el 10 % de las tarjetas.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// I-EJE1 · LA TAXONOMÍA ETIQUETA POR VOCABULARIO, NO POR EL NOMBRE DEL EJE.
//
// El acervo casi nunca dice «a11y»: dice «contraste», «lector de pantalla», «foco visible». Un
// etiquetado que buscara el nombre del eje dejaría casi todo sin etiquetar.
//
// SABOTAJE: etiquetar buscando el nombre del eje en vez de su vocabulario ⇒ una tarjeta que habla
// de accesibilidad sin nombrarla queda sin eje y este test se pone rojo.
func TestEjesEtiquetanPorVocabularioNoPorNombre(t *testing.T) {
	casos := []struct {
		topic, texto, espera string
	}{
		{"design-corpus/x", "el contraste del texto tiene que pasar AA y el foco visible al navegar con teclado", "a11y"},
		{"design-corpus/y", "las columnas numéricas van alineadas a la derecha y el encabezado queda fijo al desplazar las filas", "tabla"},
		{"design-corpus/z", "cada campo valida al salir y el error se muestra junto al input, no en un resumen", "formulario"},
	}
	for _, c := range casos {
		ejes := ejesDeTarjeta(c.topic, c.texto)
		if !ejes[c.espera] {
			t.Errorf("«%s…» tenía que caer en %q; cayó en %v", c.texto[:40], c.espera, clavesDe(ejes))
		}
		// Y ninguna de las tres NOMBRA su eje: si lo nombraran, el test pasaría por la razón
		// equivocada y no probaría nada sobre el vocabulario.
		if strings.Contains(strings.ToLower(c.texto), c.espera) {
			t.Errorf("el fixture nombra su propio eje (%q): el test pasaría sin vocabulario", c.espera)
		}
	}
}

// I-EJE2 · EL ACENTO NO ROMPE LA ETIQUETA.
//
// Sin normalizar, «validacion» del vocabulario no matchea «validación» del acervo — y el etiquetado
// se pierde justo las tarjetas escritas con más cuidado.
//
// SABOTAJE: sacar sinAcento de palabrasNormalizadas ⇒ la versión acentuada deja de etiquetar.
func TestEjesElAcentoNoRompeLaEtiqueta(t *testing.T) {
	// TODAS las palabras que etiquetan llevan acento, y eso es el punto. La primera versión usaba
	// «la validación del campo del formulario»: sin normalizar, «campo» y «formulario» alcanzaban
	// solos para etiquetar y el sabotaje pasaba VERDE. Un fixture donde el invariante tiene un
	// camino alternativo no prueba el invariante.
	const conAcento = "la navegación por menú y pestaña"
	const sinAcento = "la navegacion por menu y pestana"

	sin := ejesDeTarjeta("design-corpus/a", sinAcento)
	con := ejesDeTarjeta("design-corpus/a", conAcento)
	if !sin["navegacion"] {
		t.Fatalf("el fixture sin acentos ni siquiera etiqueta: %v", clavesDe(sin))
	}
	if !con["navegacion"] {
		t.Errorf("la versión acentuada no etiquetó: %v — el acervo escrito con cuidado queda mudo", clavesDe(con))
	}
	if len(sin) != len(con) {
		t.Errorf("el acento cambió el etiquetado: sin=%v con=%v", clavesDe(sin), clavesDe(con))
	}
}

// I-EJE3 · UNA MENCIÓN SUELTA NO ETIQUETA.
//
// Con umbral 1, cualquier tarjeta que diga «contraste» una vez cae a la vez en color, jerarquia y
// a11y, y la etiqueta deja de significar algo.
//
// SABOTAJE: bajar designEjeMinHits a 1 ⇒ la tarjeta de una sola mención se lleva varios ejes.
func TestEjesUnaMencionSueltaNoEtiqueta(t *testing.T) {
	if designEjeMinHits < 2 {
		t.Fatalf("designEjeMinHits=%d: con 1, una mención suelta etiqueta y el eje no significa nada", designEjeMinHits)
	}
	ejes := ejesDeTarjeta("design-corpus/suelta", "una nota sobre el contraste, y nada más")
	if ejes["a11y"] && ejes["color"] && ejes["jerarquia"] {
		t.Errorf("una sola mención se llevó tres ejes: %v", clavesDe(ejes))
	}
}

// I-EJE4 · SIN LA TABLA COMPLETA NO SE RUTEA.
//
// Si un eje no se pudo embeber, nunca gana, y el ruteo manda en silencio los pedidos de ese tema a
// otro lado. Una falla que se lee como una decisión es peor que no rutear.
//
// SABOTAJE: devolver la tabla a medias en vez de nil ante un error del embebedor ⇒ el motor rutea
// con una taxonomía incompleta y no hay nada que lo declare.
func TestEjesSinTablaCompletaNoSeRutea(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	// Un embebedor que falla en una de las descripciones: alcanza para invalidar la tabla entera.
	s := NewMcpServer(engine, t.TempDir(), &embebedorQueFallaEn{falla: ejesDeDiseno[3].Desc})
	if v := s.vectoresDeEje(t.Context()); v != nil {
		t.Errorf("con un eje sin embeber la tabla tiene que ser nil; salió con %d entradas", len(v))
	}
	if _, _, ok := s.ejeDeConsulta(t.Context(), []float32{1, 0, 0}); ok {
		t.Error("sin tabla no se puede rutear, y dijo que sí")
	}
}

// I-EJE5 · EL EJE SE DECLARA EN EL BRIEF.
//
// Un brief que llegó por taxonomía y no lo dice obliga a adivinar si el material salió del tema o
// del azar del ranking. `retrieval` y `axis` lo cuentan.
//
// SABOTAJE: no poblar Axis ⇒ el brief rutea pero no lo declara y este test se pone rojo.
func TestEjesElRuteoSeDeclaraEnElBrief(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")
	for i := 0; i < 8; i++ {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "t"+string(rune('a'+i)),
			designCorpusPrefix+"tabla-"+string(rune('a'+i)),
			"las columnas de la tabla se ordenan y el encabezado queda fijo al desplazar las filas",
			1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	// Embebedor que pone la consulta EXACTAMENTE sobre la descripción del eje «tabla» y lejos del
	// resto: fija el eje elegido sin depender de un modelo real.
	s := NewMcpServer(engine, t.TempDir(), &embebedorDeEje{eje: "tabla"})

	b := callDesign(t, s, nil, "una tabla densa", "web")
	if b.Retrieval != recuperacionPorEje {
		t.Errorf("retrieval quedó en %q y tenía que declarar el ruteo (%q)", b.Retrieval, recuperacionPorEje)
	}
	if b.Axis != "tabla" {
		t.Errorf("axis quedó en %q y tenía que decir por qué eje se ruteó", b.Axis)
	}
	if len(b.Corpus) == 0 {
		t.Error("se ruteó y no llegó material")
	}
}

func clavesDe(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ─── embebedores de prueba ────────────────────────────────────────────────────────────────────
// Fijan el resultado del ruteo sin depender de un modelo real. NO miden calidad de recuperación:
// existen para ejercitar la aritmética del ruteo, que es lo que estos invariantes declaran.

// embebedorQueFallaEn devuelve error para UN texto y anda para el resto.
type embebedorQueFallaEn struct{ falla string }

func (e *embebedorQueFallaEn) Enabled() bool   { return true }
func (e *embebedorQueFallaEn) Dimensions() int { return 3 }
func (e *embebedorQueFallaEn) ModelID() string { return "falla-en-uno" }
func (e *embebedorQueFallaEn) Name() string    { return "falla-en-uno" }
func (e *embebedorQueFallaEn) Embed(_ context.Context, txt string) ([]float32, error) {
	if txt == e.falla {
		return nil, errors.New("este texto no")
	}
	return []float32{1, 0, 0}, nil
}
func (e *embebedorQueFallaEn) EmbedBatch(ctx context.Context, txts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(txts))
	for _, t := range txts {
		v, err := e.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// embebedorDeEje pone todo sobre un eje y el resto en otra dirección: el eje ganador es conocido.
type embebedorDeEje struct{ eje string }

func (e *embebedorDeEje) Enabled() bool   { return true }
func (e *embebedorDeEje) Dimensions() int { return 2 }
func (e *embebedorDeEje) ModelID() string { return "eje-fijo" }
func (e *embebedorDeEje) Name() string    { return "eje-fijo" }
func (e *embebedorDeEje) Embed(_ context.Context, txt string) ([]float32, error) {
	for _, x := range ejesDeDiseno {
		if x.Desc == txt {
			if x.Nombre == e.eje {
				return []float32{1, 0}, nil
			}
			return []float32{0, 1}, nil // ortogonal: coseno 0, bajo el piso
		}
	}
	return []float32{1, 0}, nil // la consulta cae sobre el eje elegido
}
func (e *embebedorDeEje) EmbedBatch(ctx context.Context, txts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(txts))
	for _, t := range txts {
		v, _ := e.Embed(ctx, t)
		out = append(out, v)
	}
	return out, nil
}
