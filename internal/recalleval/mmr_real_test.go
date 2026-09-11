package recalleval

import (
	"context"
	"os"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestBarridoMMREnLosDosEjes mide la diversificación en el eje que optimiza (redundancia) Y en el
// que paga (relevancia), que es la única forma de juzgarla.
//
// SEMÁNTICA DE LAMBDA, porque el resultado no se lee sin esto: diversify() retorna sin tocar nada
// con `lambda >= 1 || lambda <= 0`. O sea que 0 es MMR APAGADO —la línea base— y los valores
// intermedios diversifican más cuanto MÁS BAJOS son.
//
// MEDIDO sobre el corpus real (1953 docs, 85 consultas, POTION multilingüe, docs desde content):
//
//	lambda   redundancia@10   R@10     nDCG@10
//	0.00     0.7453           0.4063   0.3510   <- MMR apagado: la base
//	0.90     0.7202  (-3%)    0.3727   0.3253
//	0.75     0.6313 (-15%)    0.2679   0.2544   <- el default de producción
//	0.50     0.4806 (-36%)    0.1308   0.1613
//
// MMR HACE SU TRABAJO: la redundancia baja de forma monótona. Lo que el número dice es el PRECIO:
// al default compra 15% menos repetición a cambio de 34% de R@10.
//
// ⚠️ Y ESTE FIXTURE NO PUEDE CERRAR LA DISCUSIÓN, por una razón estructural que hay que tener
// delante antes de tocar el default: las etiquetas de relevancia salen del `topic_key`, así que
// los documentos relevantes de una consulta SON los que se parecen entre sí. MMR los separa por
// definición, y el costo de relevancia que se mide acá es una COTA SUPERIOR, inflada por
// construcción. Lo mismo pasa del otro lado: parte de la "redundancia" que MMR saca es
// material relevante.
//
// LO QUE DESBLOQUEA LA DECISIÓN es una etiqueta de relevancia que NO derive de la similitud, y ya
// existe sin usarse: cada `musubi_memory_expand` es el agente diciendo «de todos los gists que me
// diste, ESTE lo quiero entero» — relevancia elegida, no derivada. Hay 53 en la bitácora y
// `tool_invocations` no guarda el observation_id. Ése es el item que convierte esta medición en un
// veredicto.
func TestBarridoMMREnLosDosEjes(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirP := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirP == "" {
		t.Skip("faltan env")
	}
	prov, err := embedding.NewStaticProvider(dirP)
	if err != nil {
		t.Fatal(err)
	}
	embed := func(s string) ([]float32, error) { return prov.Embed(context.Background(), s) }
	fx, err := FixtureDesdeDB(ruta, OpcionesFixtureReal{})
	if err != nil {
		t.Fatal(err)
	}
	eng, err := SeedEngine(t.TempDir(), fx, embed)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	// Los vectores se leen DE LA BASE SEMBRADA, no se recalculan: recalcularlos duplicaba el
	// trabajo de embeber 1953 contenidos y hacía que el test muriera por OOM (la máquina tiene
	// 7 GB y la tabla POTION son 512 MB). Además es lo correcto: la similitud tiene que salir de
	// los mismos vectores que el ranker compara.
	vecs, err := eng.VectoresDePruebas()
	if err != nil {
		t.Fatal(err)
	}
	sim := func(a, b string) float64 {
		va, vb := vecs[a], vecs[b]
		if va == nil || vb == nil {
			return 0
		}
		c, err := memory.CosineSimilarity(va, vb)
		if err != nil {
			return 0
		}
		return float64(c)
	}

	t.Logf("%-10s %-14s %-10s %-10s", "MMRLambda", "redundancia@10", "R@10", "nDCG@10")
	for _, lambda := range []float64{0, 0.5, 0.75, 0.9} {
		o := OptsDeProduccion()
		o.MMRLambda = lambda
		cfg := Config{Name: "mmr", Opts: o, UseVector: true}
		var red, rec, ndcg float64
		var n int
		for _, q := range fx.Queries {
			if len(q.Relevant) == 0 {
				continue
			}
			rel := map[string]bool{}
			for _, id := range q.Relevant {
				rel[id] = true
			}
			ranked, err := rankedIDs(context.Background(), eng, q.Text, cfg, embed, len(fx.Docs))
			if err != nil {
				t.Fatal(err)
			}
			red += RedundanciaAtK(ranked, 10, sim)
			rec += RecallAtK(ranked, rel, 10)
			ndcg += NDCGAtK(ranked, rel, 10)
			n++
		}
		t.Logf("%-10.2f %-14.4f %-10.4f %-10.4f", lambda, red/float64(n), rec/float64(n), ndcg/float64(n))
	}
}
