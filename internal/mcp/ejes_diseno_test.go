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
	"fmt"
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
	if _, ok := s.ejeDeConsulta(t.Context(), []float32{1, 0, 0}); ok {
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

// I-EJE6 · EL SLUG DECLARA EL EJE, Y EL VOCABULARIO NO PUEDE SER LA ÚNICA VÍA.
//
// Al sembrar las primeras 28 tarjetas de `presencia` y `terminacion`, sólo 6 y 5 de 14 caían por
// vocabulario — porque el vocabulario lo había inventado yo en vez de sacarlo del material. Y
// derivarlo del material tampoco servía: salían palabras genéricas de diseño («patrón», «elemento»,
// «tamaño») que habrían etiquetado medio acervo. Quien escribe una tarjeta necesita una manera
// EXACTA de decir a qué eje pertenece.
//
// SABOTAJE: sacar el camino del slug ⇒ una tarjeta que nombra su eje en el topic y no repite su
// vocabulario en el cuerpo queda sin etiquetar, o sea inalcanzable por ruteo.
func TestEjesElSlugDeclaraElEje(t *testing.T) {
	// EL FIXTURE NO PUEDE ETIQUETARSE POR VOCABULARIO, o el test pasa por el otro camino y no prueba
	// nada. La primera versión usaba el topic real `presencia-un-solo-protagonista`, que trae DOS
	// palabras del vocabulario en el propio slug —«presencia» y «protagonista»— así que el sabotaje
	// quedaba verde. Acá el resto del slug y el cuerpo evitan el vocabulario a propósito.
	const cuerpo = "Antes de componer, decidí cuál es el único bloque que justifica abrir esto."
	const slug = "presencia-el-bloque-que-manda"

	// Guarda de la premisa: SIN el prefijo del eje, esto NO se etiqueta.
	if control := ejesDeTarjeta(designCorpusPrefix+"otro-el-bloque-que-manda", cuerpo); control["presencia"] {
		t.Fatal("el fixture etiqueta por vocabulario: el test pasaría sin el camino del slug")
	}

	ejes := ejesDeTarjeta(designCorpusPrefix+slug, cuerpo)
	if !ejes["presencia"] {
		t.Errorf("el slug declaraba 'presencia' y quedó sin etiquetar: %v", clavesDe(ejes))
	}
	if ejes["terminacion"] {
		t.Errorf("el slug de presencia también reclamó terminacion: %v", clavesDe(ejes))
	}
}

// C-EJE-R1 · EL RUTEO DECLARA CONTRA QUÉ SE DECIDIÓ.
//
// Nació de un caso medido en vivo (2026-08-31): «un panel de tickets» ruteaba a `login` —y el brief
// salía con exigencias de pantalla de acceso y la prohibición del mensaje «usuario o contraseña
// incorrectos» para un panel de soporte—, mientras que «un panel de incidencias», «un panel de
// reclamos de soporte», «un panel» a secas y hasta «un panel de tickets DE SOPORTE» ruteaban todos a
// `dashboard`. Una decisión que se da vuelta con dos palabras estaba contra un borde, y el brief no
// tenía cómo decirlo porque el segundo candidato se descartaba.
func TestRuteoDeclaraElSegundoCandidato(t *testing.T) {
	// TRES NIVELES DE SIMILITUD, Y EL ALTO VA DESPUÉS DEL MEDIO. La primera versión usaba el
	// embebedor de eje fijo, donde todo lo que no es el elegido puntúa 0: ahí el «segundo» es
	// cualquier otro eje con 0, y sale igual con el código sano que con el saboteado. El sabotaje que
	// borra el ascenso del ganador anterior a segundo puesto pasó VERDE.
	//
	// Para que el caso distinga hace falta un segundo IDENTIFICABLE, y que el ganador aparezca DESPUÉS
	// en el recorrido — si apareciera antes, el segundo se llenaría por la otra rama y el sabotaje
	// seguiría escondido.
	// Y HAY QUE PROBAR LOS DOS ÓRDENES. El segundo puesto se llena por DOS ramas distintas —el ganador
	// anterior que baja, y un candidato nuevo que no llega a primero— y con un solo orden cada rama
	// tapa a la otra: saboteando cualquiera de las dos, el test pasaba VERDE por culpa de la que
	// quedaba viva. Con el segundo ANTES del ganador se ejerce el ascenso; con el segundo DESPUÉS se
	// ejerce la otra.
	primero, ultimo := ejesDeDiseno[0].Nombre, ejesDeDiseno[len(ejesDeDiseno)-1].Nombre
	if primero == ultimo {
		t.Fatal("la premisa no se cumple: hacen falta al menos dos ejes")
	}
	for _, caso := range []struct{ nombre, alto, medio string }{
		{"el segundo aparece ANTES que el ganador", ultimo, primero},
		{"el segundo aparece DESPUÉS que el ganador", primero, ultimo},
	} {
		t.Run(caso.nombre, func(t *testing.T) { probarRuteo(t, caso.alto, caso.medio) })
	}
}

func probarRuteo(t *testing.T, alto, medio string) {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), &embebedorGraduado{alto: alto, medio: medio})

	ruta, ok := s.ejeDeConsulta(t.Context(), []float32{1, 0, 0})
	if !ok {
		t.Fatal("el fixture no ruteó: la premisa del caso no se cumple")
	}
	if ruta.Eje != alto {
		t.Fatalf("ruteó a %q y el fixture pone arriba a %q", ruta.Eje, alto)
	}
	if ruta.Segundo != medio {
		t.Errorf("el segundo es %q (%.3f) y el fixture pone segundo a %q: el ganador anterior no se está ascendiendo",
			ruta.Segundo, ruta.SimSeg, medio)
	}
	if ruta.SimSeg > ruta.Sim {
		t.Errorf("el segundo (%.3f) le gana al primero (%.3f): el orden está al revés", ruta.SimSeg, ruta.Sim)
	}

	// Y la nota tiene que llevar LOS DOS NOMBRES Y LOS DOS NÚMEROS. Un dato, no un adjetivo: poner
	// un umbral de «ajustado» sería fijar a ojo dónde empieza, sin haberlo medido.
	nota := notaDeRuteo(ruta)
	for _, quiero := range []string{ruta.Eje, ruta.Segundo,
		fmt.Sprintf("%.2f", ruta.Sim), fmt.Sprintf("%.2f", ruta.SimSeg)} {
		if !strings.Contains(nota, quiero) {
			t.Errorf("la nota de ruteo no dice %q: %s", quiero, nota)
		}
	}
	// Sin ruteo no se estampa una nota: un aviso sobre algo que no pasó es ruido.
	if n := notaDeRuteo(rutaDeEje{}); n != "" {
		t.Errorf("sin eje ruteado se emitió una nota igual: %q", n)
	}
}

// embebedorGraduado da TRES niveles de similitud contra la consulta {1,0,0}: 1,0 para `alto`, 0,8
// para `medio` y 0 para el resto. Con eso el segundo puesto tiene nombre propio y se puede afirmar
// cuál debería ser.
type embebedorGraduado struct{ alto, medio string }

func (e *embebedorGraduado) Enabled() bool   { return true }
func (e *embebedorGraduado) Dimensions() int { return 3 }
func (e *embebedorGraduado) ModelID() string { return "graduado" }
func (e *embebedorGraduado) Name() string    { return "graduado" }
func (e *embebedorGraduado) Embed(_ context.Context, txt string) ([]float32, error) {
	for _, x := range ejesDeDiseno {
		if x.Desc != txt {
			continue
		}
		switch x.Nombre {
		case e.alto:
			return []float32{1, 0, 0}, nil
		case e.medio:
			return []float32{0.8, 0.6, 0}, nil
		}
		return []float32{0, 0, 1}, nil // ortogonal
	}
	return []float32{1, 0, 0}, nil // la consulta
}
