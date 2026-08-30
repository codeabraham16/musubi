package mcp

// methods_design_seleccion_test.go — invariantes de Musubi Renaissance F4.
//
// Los tres defectos que esta fase cierra, medidos el 2026-08-29 contra el central:
//   · el bloque de método tenía el MISMO hash para un ERP de escritorio, un juego móvil, una landing
//     y un gráfico de series — no respondía nada sobre el pedido;
//   · en un pool de 58 candidatos salieron 58 micro-tarjetas y 0 artículos completos, así que toda la
//     profundidad del acervo era inalcanzable;
//   · el top-6 de «tabla densa con filtros» eran cuatro maneras de decir la misma idea.

import (
	"math"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// acervoDirigido siembra observaciones a similitudes CONOCIDAS de la consulta "CONSULTA": cada
// entrada declara su topic, su texto y a qué similitud queda. Igual que en el banco de F3, esto no
// simula calidad de recuperación — fija los números para poder ejercitar la lógica de selección.
type entradaDirigida struct {
	topic, texto string
	sim          float64
}

func acervoDirigido(t *testing.T, entradas []entradaDirigida) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	engine.SetProjectID("")

	angulos := map[string]float64{"CONSULTA": 0}
	for i, e := range entradas {
		marca := "ANCLA" + string(rune('a'+i))
		sim := e.sim
		if sim > 1 {
			sim = 1
		}
		angulos[marca] = math.Acos(sim)
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", "id"+string(rune('a'+i)),
			e.topic, marca+" "+e.texto, 1.0-float64(i)*0.01, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	emb := &embebedorPorAngulo{angulos: angulos}
	s := NewMcpServer(engine, t.TempDir(), emb)
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

// I-SEL1 · el método depende del pedido. Antes se servían siempre las mismas tarjetas, ordenadas por
// importancia: verificado contra el central, el hash del bloque de método era idéntico para cuatro
// pedidos radicalmente distintos. Un bloque que no cambia con el pedido no transporta información
// sobre el pedido — ocupa canal y desplaza a lo que sí responde.
func TestDesignElMetodoSigueAlPedido(t *testing.T) {
	// Dos tarjetas de método: una pega con la consulta, la otra no. Y un patrón para que el corpus no
	// quede vacío (si no, el brief degrada por bajo umbral antes de llegar al método).
	s := acervoDirigido(t, []entradaDirigida{
		{"design-method/pega", "criterio que aplica a este pedido", 0.95},
		{"design-method/no-pega", "criterio de otra superficie", 0.10},
		{"design-corpus/patron", "un patrón cualquiera", 0.90},
	})

	b := callDesign(t, s, nil, "CONSULTA", "web")
	if b.MethodSource != "relevancia" {
		t.Fatalf("con embebedor el método debe salir por relevancia; fue %q", b.MethodSource)
	}
	// Lo que F4 garantiza es el ORDEN, no la exclusión: la tarjeta que habla del pedido va PRIMERO.
	// Excluir por poco parecido fue un error que costó una regresión en producción — ver
	// TestDesignElMetodoLlegaAUnPedidoDeDominio.
	if len(b.Method) == 0 {
		t.Fatal("el método del acervo tiene que servirse")
	}
	if b.Method[0].Topic != "design-method/pega" {
		var topics []string
		for _, m := range b.Method {
			topics = append(topics, m.Topic)
		}
		t.Errorf("la tarjeta relevante al pedido tiene que ir primero; salió %v", topics)
	}
}

// REGRESIÓN DE PRODUCCIÓN (2026-08-30). Con el piso aplicado también al método, un pedido de dominio
// concreto —«tabla densa de inventario de Altura con lotes»— recibía el bloque de método VACÍO:
// ninguna de las 30 tarjetas arbitradas llegaba a 0,48, porque un principio UNIVERSAL es por
// construcción menos parecido a un pedido concreto que un patrón que habla justo de eso.
//
// Medido en el central: con «el color se gana: un acento dominante» servía 3 tarjetas; con el pedido
// de Altura, cero. La capa 2 quedaba muda justo donde alguien está diseñando.
func TestDesignElMetodoLlegaAUnPedidoDeDominio(t *testing.T) {
	// El método queda LEJOS del pedido (0,30, bajo el piso del corpus) y el patrón cerca.
	s := acervoDirigido(t, []entradaDirigida{
		{"design-method/universal", "criterio que aplica a toda pantalla", 0.30},
		{"design-corpus/patron", "un patrón que habla del pedido", 0.90},
	})
	b := callDesign(t, s, nil, "CONSULTA", "web")

	if len(b.Method) == 0 {
		t.Error("REGRESIÓN: el método universal no llegó a un pedido de dominio concreto")
	}
	if b.MethodSource != "relevancia" {
		t.Errorf("esperaba method_source 'relevancia'; fue %q", b.MethodSource)
	}
	// Y el piso SIGUE mandando sobre el corpus: eso no se tocó.
	for _, h := range b.Corpus {
		if h.Similarity < designSimilitudMinima {
			t.Errorf("el piso del corpus se aflojó: %s con %.3f", h.TopicKey, h.Similarity)
		}
	}
}

// I-SEL2 · el núcleo está siempre, gane o no por relevancia. F1+F2 lo dejó en `principles`, que es
// código; por eso el método del acervo se puede elegir 100 % por relevancia sin riesgo de quedarse sin
// criterio. Este test lo defiende: aunque NINGUNA tarjeta del acervo sea relevante, el criterio queda.
func TestDesignElNucleoNoDependeDelAcervo(t *testing.T) {
	s := acervoDirigido(t, []entradaDirigida{
		{"design-method/lejana", "criterio de otro planeta", 0.05},
		{"design-corpus/patron", "un patrón cualquiera", 0.90},
	})
	b := callDesign(t, s, nil, "CONSULTA", "web")

	if !strings.Contains(b.Principles, "JERARQU") || !strings.Contains(b.Principles, "UN CTA") {
		t.Errorf("el núcleo estático tiene que estar completo; got=%.140q", b.Principles)
	}
	// Y el método lejano SÍ se sirve: es universal, y retenerlo por poco parecido era la regresión.
	// Lo que lo acota es la cantidad (designMetodoRelevante), no un umbral de similitud.
	if len(b.Method) == 0 {
		t.Error("el método universal tiene que llegar aunque no se parezca al pedido")
	}
	if len(b.Method) > designMetodoRelevante {
		t.Errorf("el método se acota por CANTIDAD: %d supera el tope %d", len(b.Method), designMetodoRelevante)
	}
}

// I-SEL3 · el top-k no colapsa en variaciones de lo mismo. El caso real: para «tabla densa de lotes
// con filtros y alertas» el motor servía colapsar filas, filtros post-búsqueda, filtros drill-down y
// cortina de dos niveles — cuatro veces la misma idea ocupando cuatro de los seis lugares.
func TestDesignElTopKNoColapsaEnLoMismo(t *testing.T) {
	// Ocho patrones casi idénticos, con similitud ALTA, y uno distinto con similitud MENOR. Sin
	// diversidad los ocho clones se llevan todos los lugares y el distinto nunca entra.
	entradas := []entradaDirigida{}
	for i := 0; i < 8; i++ {
		entradas = append(entradas, entradaDirigida{
			topic: "design-corpus/clon-" + string(rune('a'+i)),
			texto: "filtros filtrado filtrar tabla tablas filas columnas densidad densas",
			sim:   0.95,
		})
	}
	entradas = append(entradas, entradaDirigida{
		topic: "design-corpus/distinto",
		texto: "contraste tipografico escala vertical ritmo respiracion margenes",
		sim:   0.72,
	})
	s := acervoDirigido(t, entradas)

	b := callDesign(t, s, nil, "CONSULTA", "web")
	var hallado bool
	for _, h := range b.Corpus {
		if h.TopicKey == "design-corpus/distinto" {
			hallado = true
		}
	}
	if !hallado {
		var ids []string
		for _, h := range b.Corpus {
			ids = append(ids, h.TopicKey)
		}
		t.Errorf("el candidato DISTINTO no entró al top-k; salieron %v", ids)
	}

	// SABOTAJE: con la diversidad apagada (λ=0) los clones ganan por similitud pura y el distinto
	// queda afuera. Se comprueba sobre la función directamente para que el invariante no dependa de
	// que alguien recuerde tocar la constante.
	fuentes := make([]searchSource, 0, len(entradas))
	for i, e := range entradas {
		fuentes = append(fuentes, searchSource{
			id: "id" + string(rune('a'+i)), topicKey: e.topic, content: e.texto, sim: float32(e.sim),
		})
	}
	conDiversidad := diversificar(fuentes, 6)
	if !contieneTopic(conDiversidad, "design-corpus/distinto") {
		t.Error("diversificar debería dejar entrar al distinto")
	}
	sinDiversidad := porSimilitudPura(fuentes, 6)
	if contieneTopic(sinDiversidad, "design-corpus/distinto") {
		t.Error("el sabotaje no sabotea: por similitud pura el distinto NO debería entrar")
	}
}

func contieneTopic(src []searchSource, topic string) bool {
	for _, s := range src {
		if s.topicKey == topic {
			return true
		}
	}
	return false
}

// porSimilitudPura es el comportamiento ANTERIOR: top-n por similitud, sin penalizar la redundancia.
// Vive en el test para que el sabotaje de I-SEL3 compare contra algo real y no contra una descripción.
func porSimilitudPura(src []searchSource, n int) []searchSource {
	if len(src) < n {
		n = len(src)
	}
	return src[:n]
}

// I-SEL4 · los artículos completos tienen lugar reservado. Sin reserva, 1.438 micro-tarjetas los
// desplazan SIEMPRE por pura aritmética — medido: en un pool de 58 salieron 58 tarjetas y 0 artículos,
// y ahí está toda la profundidad del acervo (~3.057 tokens por artículo contra ~61 por tarjeta).
func TestDesignLosArticulosCompletosTienenLugar(t *testing.T) {
	entradas := []entradaDirigida{}
	for i := 0; i < 10; i++ { // muchas tarjetas cortas, todas mejor rankeadas
		entradas = append(entradas, entradaDirigida{
			topic: "design-corpus/tarjeta-" + string(rune('a'+i)),
			texto: "tarjeta corta " + string(rune('a'+i)),
			sim:   0.95,
		})
	}
	entradas = append(entradas,
		entradaDirigida{topic: "ingested/articulo-uno", texto: "artículo largo con desarrollo real", sim: 0.70},
		entradaDirigida{topic: "ingested/articulo-dos", texto: "otro artículo con profundidad", sim: 0.68})
	s := acervoDirigido(t, entradas)

	b := callDesign(t, s, nil, "CONSULTA", "web")
	crudos := 0
	for _, h := range b.Corpus {
		if strings.HasPrefix(h.TopicKey, prefijoCrudo) {
			crudos++
		}
	}
	if crudos == 0 {
		var ids []string
		for _, h := range b.Corpus {
			ids = append(ids, h.TopicKey)
		}
		t.Errorf("ningún artículo completo entró al corpus; salieron %v", ids)
	}
	// Y lo curado sigue teniendo la mayoría: la reserva da lugar, no da vuelta la prioridad.
	if crudos > len(b.Corpus)/2 {
		t.Errorf("la reserva se llevó %d de %d lugares; debería ser minoría", crudos, len(b.Corpus))
	}
}

// REGRESIÓN DE PRODUCCIÓN, SEGUNDA VUELTA (2026-08-30). La primera corrección (#367) le sacó el piso
// de similitud al método y NO alcanzó: medido contra el central con ese fix ya desplegado, «tabla
// densa de inventario de Altura con lotes» seguía trayendo CERO tarjetas.
//
// La causa estaba una etapa antes: el método salía del POOL, que es un top-N por similitud sobre el
// tenant entero —1.438 tarjetas de corpus y 268 artículos contra 30 de método— así que en un pedido
// de dominio las de método ni siquiera entraban. No es que se filtraran: nunca llegaban. Un criterio
// UNIVERSAL no le puede ganar en similitud a un patrón que habla justo del pedido.
func TestDesignElMetodoNoCompitePorElPool(t *testing.T) {
	s, e := bancoDesign(t)
	sembrarAtaque(t, e, designCorpusScope, "m1", "design-method/jerarquia",
		"JERARQUIA: una sola cosa manda por pantalla.", 1.0)
	sembrarAtaque(t, e, designCorpusScope, "m2", "design-method/el-color-se-gana",
		"EL COLOR SE GANA: un acento dominante, el resto neutro.", 0.9)

	// El caso exacto: el pool NO trajo una sola tarjeta de método. El set base tiene que salir igual.
	cards, source := s.designMethodCards(nil, recuperacionSemantica)
	if len(cards) == 0 {
		t.Error("REGRESIÓN: sin método en el pool, el bloque quedó vacío")
	}
	if source != "importancia" {
		t.Errorf("sin señal de relevancia el orden es por importancia; fue %q", source)
	}

	// Y cuando el pool SÍ trae señal, la usa para reordenar sin descartar nada.
	conSenal := []searchSource{{id: "x", topicKey: "design-method/el-color-se-gana", content: "…", sim: 0.9}}
	cards2, source2 := s.designMethodCards(conSenal, recuperacionSemantica)
	if source2 != "relevancia" {
		t.Errorf("con señal del pool el orden es por relevancia; fue %q", source2)
	}
	if len(cards2) != len(cards) {
		t.Errorf("reordenar no puede DESCARTAR: %d con señal vs %d sin señal", len(cards2), len(cards))
	}
	if cards2[0].Topic != "design-method/el-color-se-gana" {
		t.Errorf("lo que el pool trajo va primero; fue %q", cards2[0].Topic)
	}
}
