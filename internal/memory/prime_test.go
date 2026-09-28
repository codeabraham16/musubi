package memory

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"musubi/internal/config"
)

// TestPrimeContextRespetaBudget: diez notas que juntas pasan largo el presupuesto; lo que entra no
// lo pasa, y algo entra.
//
// CADA NOTA CON SU PROPIO TEMA, A PROPÓSITO. Estaban las diez en "topic/x", y desde que el priming
// deja una nota por tema entraba UNA sola: el presupuesto de 40 ya no cortaba nada, y la prueba
// seguía verde con un empaquetar que no mirara el presupuesto (el sabotaje de abajo, medido).
//
// Sabotaje: empaquetar no corta en el presupuesto.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\t} else if result.UsedTokens+cost > budget {\n"
// arnes: a="\t\t} else if false && result.UsedTokens+cost > budget {\n"
func TestPrimeContextRespetaBudget(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 10; i++ {
		id := "p" + string(rune('a'+i))
		if err := e.SaveObservation(id, "topic/x-"+id, "Observación de prueba número con bastante contenido para gastar varios tokens "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	res, err := e.PrimeContext(40)
	if err != nil {
		t.Fatalf("PrimeContext error: %v", err)
	}
	if res.UsedTokens > 40 {
		t.Errorf("used_tokens=%d excede el budget 40", res.UsedTokens)
	}
	if res.Count == 0 {
		t.Error("se esperaba al menos un item dentro del budget")
	}
	for _, it := range res.Items {
		if it.Gist == "" {
			t.Error("cada item de priming debe traer un gist")
		}
	}
}

func TestPrimeContextVacio(t *testing.T) {
	e := newTestEngine(t)
	res, err := e.PrimeContext(100)
	if err != nil {
		t.Fatalf("PrimeContext error en DB vacía: %v", err)
	}
	if res.Count != 0 || len(res.Items) != 0 {
		t.Errorf("DB vacía debe dar 0 items, obtuve %d", res.Count)
	}
}

func TestPrimeContextPriorizaSalience(t *testing.T) {
	e := newTestEngine(t)
	// Observación poco importante.
	if err := e.SaveObservationWithImportance("low", "topic/low", "dato menor irrelevante", 0.5, nil); err != nil {
		t.Fatal(err)
	}
	// Observación muy importante: debe aparecer primero / entrar con budget chico.
	if err := e.SaveObservationWithImportance("high", "topic/high", "decisión de arquitectura crítica del proyecto", 5.0, nil); err != nil {
		t.Fatal(err)
	}
	res, err := e.PrimeContext(200)
	if err != nil {
		t.Fatalf("PrimeContext error: %v", err)
	}
	if res.Count == 0 {
		t.Fatal("se esperaban items")
	}
	if res.Items[0].ID != "high" {
		t.Errorf("la observación más importante debe rankear primero, obtuve %q", res.Items[0].ID)
	}
}

func TestPrimeContextExcluyeArchivadas(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("vis", "topic/vis", "observación visible", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("arc", "topic/arc", "observación archivada", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = 'arc'`); err != nil {
		t.Fatal(err)
	}
	res, err := e.PrimeContext(500)
	if err != nil {
		t.Fatalf("PrimeContext error: %v", err)
	}
	for _, it := range res.Items {
		if it.ID == "arc" {
			t.Error("PrimeContext no debe incluir observaciones archivadas")
		}
	}
}

// temaDelDashboard es el tema que se comía el bloque de arranque en la medición del 2026-09-26.
const temaDelDashboard = "project/brain-dashboard-webgl"

// corpusDeUnTema son cinco notas del MISMO tema, con la saliencia más alta del acervo y en orden
// (d1 es la más saliente), y tres de otros temas que pesan menos. Los textos miden parecido: así,
// con lugar para cuatro, el tema entero cabría en el presupuesto si nadie lo cortara.
func corpusDeUnTema() []notaDeReparto {
	return []notaDeReparto{
		{"musubi", "d1", temaDelDashboard, "El dashboard WebGL del cerebro dibuja las neuronas con instancias.", 5},
		{"musubi", "d2", temaDelDashboard, "El dashboard WebGL del cerebro colorea las aristas por proyecto.", 4.9},
		{"musubi", "d3", temaDelDashboard, "El dashboard WebGL del cerebro elige el nodo con picking por GPU.", 4.8},
		{"musubi", "d4", temaDelDashboard, "El dashboard WebGL del cerebro agrupa los hilos de cada rama.", 4.7},
		{"musubi", "d5", temaDelDashboard, "El dashboard WebGL del cerebro pausa el render cuando no se ve.", 4.6},
		{"musubi", "o1", "gotchas/nordvpn", "NordVPN excluye por ruta exacta: el curl de MinGW no ve la malla.", 1},
		{"musubi", "o2", "hooks/turno", "El hook del turno trae lo propio primero y lo ajeno con tope.", 1},
		{"musubi", "o3", "sync/outbox", "El outbox del sync sube cada nota nueva al cerebro central.", 1},
	}
}

// TestPrimingUnaNotaPorTema: con lugar para cuatro notas, entra UNA del tema que domina la
// saliencia —la más saliente— y las tres de otros temas ocupan el lugar que liberó. Sin cortar, el
// tema se comía el bloque: entraban cuatro de sus cinco notas y ninguna de las otras.
//
// Corre por los dos caminos que llegan a PrimeContextCtx: todo el acervo (el modo «mezclado» y el
// respaldo del arranque) y el acotado a lo propio, que es el que usa el hook en «aparte».
//
// Sabotaje: el priming no saltea el tema repetido.
// arnes: archivo="internal/memory/prime.go"
// arnes: de="\t\t\tif visto[tema] {\n"
// arnes: a="\t\t\tif false && visto[tema] {\n"
func TestPrimingUnaNotaPorTema(t *testing.T) {
	e := sembrarReparto(t, corpusDeUnTema())
	alcance := AlcanceDelTurnoSegun(config.OtrosProyectosAparte, 2, "musubi")

	for _, camino := range []struct {
		nombre string
		primar func(budget int) (RecallResult, error)
	}{
		{"todo el acervo", e.PrimeContext},
		{"acotado a lo propio", func(budget int) (RecallResult, error) {
			return e.PrimeContextCtx(WithProjectScope(context.Background(), alcance.ScopeDelPriming()), budget)
		}},
	} {
		t.Run(camino.nombre, func(t *testing.T) {
			// El presupuesto es lo que cuestan d1 y las tres de otros temas, medido con los mismos
			// gists que empaqueta el priming: no depende de cómo estime tokens el estimador de hoy.
			holgado, err := camino.primar(5000)
			if err != nil {
				t.Fatal(err)
			}
			costo := map[string]int{}
			for _, it := range holgado.Items {
				costo[it.ID] = EstimateTokens(it.Gist)
			}
			budget := 0
			for _, id := range []string{"d1", "o1", "o2", "o3"} {
				if costo[id] == 0 {
					t.Fatalf("con presupuesto de sobra tenía que entrar %s, y entró %v: sin esa nota no se mide nada", id, idsDelReparto(holgado))
				}
				budget += costo[id]
			}

			res, err := camino.primar(budget)
			if err != nil {
				t.Fatal(err)
			}
			var delTema []string
			for _, it := range res.Items {
				if it.TopicKey == temaDelDashboard {
					delTema = append(delTema, it.ID)
				}
			}
			if len(delTema) != 1 || delTema[0] != "d1" {
				t.Errorf("de %s tenía que entrar sólo d1, la más saliente, y entraron %v (bloque: %v)", temaDelDashboard, delTema, idsDelReparto(res))
			}
			for _, id := range []string{"o1", "o2", "o3"} {
				if !trae(res, id) {
					t.Errorf("con el tema repetido afuera, %s tenía que entrar en el lugar que quedó (bloque: %v, presupuesto %d)", id, idsDelReparto(res), budget)
				}
			}
		})
	}
}

// TestPrimingElTemaQueNoCabeQuedaAfuera: el borde del presupuesto. La deduplicación va ANTES de
// empaquetar, así que si la nota más saliente de un tema no cabe en lo que queda, el tema queda
// afuera y el lugar lo toma la próxima de OTRO tema que quepa. La segunda del mismo tema no entra
// aunque sea más corta: sería una versión menos saliente del tema, elegida sólo por el largo.
//
// Sabotaje: el priming anota el tema pero no lo recuerda.
// arnes: archivo="internal/memory/prime.go"
// arnes: de="\t\t\tvisto[tema] = true\n"
// arnes: a="\t\t\tvisto[tema] = false\n"
func TestPrimingElTemaQueNoCabeQuedaAfuera(t *testing.T) {
	e := sembrarReparto(t, []notaDeReparto{
		{"musubi", "x1", "arranque/primera", "La primera del ranking entra siempre.", 5},
		{"musubi", "a1", "tema/a", "La nota más saliente del tema a es la más larga del corpus: habla del hook de arranque, del presupuesto de tokens y de por qué el bloque de memoria se llenaba con un solo tema.", 4},
		{"musubi", "a2", "tema/a", "El tema a, corto.", 3},
		{"musubi", "b1", "tema/b", "El tema b, en pocas palabras.", 2},
	})
	// Lo que cuesta cada nota en empaquetar: su gist guardado, o el que sale del contenido si no
	// tiene uno. Medido acá y no supuesto, para no depender de cómo estime el estimador de hoy.
	costo := func(id string) int {
		var gist, contenido string
		if err := e.db.QueryRow(`SELECT COALESCE(gist,''), content FROM observations WHERE id = ?`, id).Scan(&gist, &contenido); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(gist) == "" {
			gist = Gist(contenido, defaultGistMaxTokens)
		}
		return EstimateTokens(gist)
	}
	if costo("a1") <= costo("b1") || costo("a2") > costo("b1") {
		t.Fatalf("el corpus no mide el borde: a1 (%d) tiene que costar más que b1 (%d), y a2 (%d) no más", costo("a1"), costo("b1"), costo("a2"))
	}
	// Después de x1 queda lugar justo para b1: a1 no cabe, y a2 cabría.
	budget := costo("x1") + costo("b1")

	res, err := e.PrimeContext(budget)
	if err != nil {
		t.Fatal(err)
	}
	if got := idsDelReparto(res); !reflect.DeepEqual(got, []string{"x1", "b1"}) {
		t.Errorf("con a1 sin lugar, el tema a tenía que quedar afuera y entrar b1: entró %v (presupuesto %d)", got, budget)
	}
}

// TestPrimingElTemaVacioNoSeDeduplica: un topic vacío no agrupa nada —no dice de qué habla la
// nota—, así que dos notas sin tema entran las dos. Y «vacío» incluye el topic en blanco, que el MCP
// rechaza igual que al vacío: dos notas con "  " tampoco son un mismo tema.
//
// Sabotaje: el tema vacío también se deduplica.
// arnes: archivo="internal/memory/prime.go"
// arnes: de="\t\tif tema != \"\" {\n"
// arnes: a="\t\tif tema != \"\" || tema == \"\" {\n"
//
// Sabotaje: un topic en blanco deja de contar como vacío.
// arnes: archivo="internal/memory/prime.go"
// arnes: de="\t\ttema := strings.TrimSpace(c.topicKey)\n"
// arnes: a="\t\ttema := strings.Clone(c.topicKey)\n"
func TestPrimingElTemaVacioNoSeDeduplica(t *testing.T) {
	e := sembrarReparto(t, []notaDeReparto{
		{"musubi", "v1", "", "Una nota sin tema sobre el hook de arranque.", 0},
		{"musubi", "v2", "", "Otra nota sin tema, sobre el outbox del sync.", 0},
		{"musubi", "b1", "  ", "Una nota con el tema en blanco, sobre NordVPN.", 0},
		{"musubi", "b2", "  ", "Otra nota con el tema en blanco, sobre el kiosko.", 0},
	})
	res, err := e.PrimeContext(5000)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"v1", "v2", "b1", "b2"} {
		if !trae(res, id) {
			t.Errorf("una nota sin tema no se deduplica, y %s no entró: %v", id, idsDelReparto(res))
		}
	}
}
