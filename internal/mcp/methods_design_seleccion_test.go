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
	var topics []string
	for _, m := range b.Method {
		topics = append(topics, m.Topic)
	}
	if len(topics) != 1 || topics[0] != "design-method/pega" {
		t.Errorf("esperaba sólo la tarjeta relevante; sirvió %v", topics)
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
	for _, m := range b.Method {
		if m.Topic == "design-method/lejana" {
			t.Error("una tarjeta bajo el piso no debería servirse como método")
		}
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
