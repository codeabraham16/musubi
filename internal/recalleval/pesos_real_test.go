package recalleval

import (
	"context"
	"os"
	"sort"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// ¿ALGUNA SEÑAL DEL RRF MERECE PESAR DISTINTO DE 1?
//
// Las siete valían 1.0, y eso NO era una decisión medida: es el default de Reciprocal Rank Fusion,
// que existe justamente para no tener que elegir pesos. Este barrido existe para averiguar si
// alguna merece otro, y está escrito para poder contestar QUE NO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA PARTICIÓN TRAIN/TEST NO ES CEREMONIA: ES LA ÚNICA DEFENSA CONTRA ESTE FIXTURE
//
// Las etiquetas salen del `topic_key`, así que «relevante» y «léxicamente parecido» son casi la
// misma cosa. Un barrido sobre TODAS las consultas encontraría el peso que mejor explota ese
// sesgo —subir el léxico— y el número saldría lindo. Ese es el mismo mecanismo que infla el costo
// de MMR en el barrido hermano, y ahí ya nos costó una conclusión.
//
// Con la partición, el peso se ELIGE mirando la mitad TRAIN y se REPORTA sobre la mitad TEST, que
// no participó de la elección. Si la mejora no sobrevive el cruce, era del fixture y no del ranker.
//
// ★ Y AUN ASÍ NO ALCANZA PARA MOVER PRODUCCIÓN, porque las dos mitades comparten el sesgo del
// etiquetado: sobrevivir el cruce descarta el sobreajuste, no el sesgo. La medición que sí lo
// descartaría necesita la etiqueta por EXPANSIÓN (EtiquetadoPorExpansion), que empezó a
// registrarse en la v57 y arranca vacía. Por eso este archivo mide y reporta, y NO cambia ningún
// default: hacerlo sería mover el ranker con el número que este fixture sabe dar.

// pesosSweep son las variantes que se prueban de a una, dejando el resto en 1.0.
//
// SE BARRE DE A UNA SEÑAL y no en grilla completa, y no es por costo: una grilla de 7 dimensiones
// devuelve un vector de pesos que nadie puede explicar, y sobre un fixture sesgado eso es
// exactamente cómo se sobreajusta sin darse cuenta. De a una, el resultado contesta una pregunta
// que se puede defender: «¿cuánto aporta ESTA señal?».
var pesosSweep = []struct {
	senal string
	set   func(*memory.PesosRRF, float64)
}{
	{"lexico", func(p *memory.PesosRRF, v float64) { p.Lexico = v }},
	{"vector", func(p *memory.PesosRRF, v float64) { p.Vector = v }},
	{"grafo", func(p *memory.PesosRRF, v float64) { p.Grafo = v }},
	{"coocurrencia", func(p *memory.PesosRRF, v float64) { p.Coocurrencia = v }},
	{"recencia", func(p *memory.PesosRRF, v float64) { p.Recencia = v }},
	{"frecuencia", func(p *memory.PesosRRF, v float64) { p.Frecuencia = v }},
	{"importancia", func(p *memory.PesosRRF, v float64) { p.Importancia = v }},
}

// pesosValores: apagar, mitad, doble. El 1.0 es la línea base y no se repite.
var pesosValores = []float64{0, 0.5, 2}

// particion reparte las consultas en dos mitades ESTABLES. El corte es por índice sobre la lista
// ya ordenada del fixture (que a su vez es determinista): sin un criterio estable, dos corridas
// elegirían el peso mirando conjuntos distintos y el cruce no significaría nada.
func particion(qs []Query) (train, test []Query) {
	ordenadas := append([]Query{}, qs...)
	sort.Slice(ordenadas, func(i, j int) bool { return ordenadas[i].ID < ordenadas[j].ID })
	for i, q := range ordenadas {
		if len(q.Relevant) == 0 {
			continue
		}
		if i%2 == 0 {
			train = append(train, q)
		} else {
			test = append(test, q)
		}
	}
	return train, test
}

// mideSobre corre una config sobre un conjunto de consultas y devuelve (R@10, nDCG@10).
func mideSobre(t *testing.T, eng *memory.DbEngine, embed func(string) ([]float32, error),
	qs []Query, cfg Config, tope int) (float64, float64) {
	t.Helper()
	var rec, ndcg float64
	for _, q := range qs {
		rel := map[string]bool{}
		for _, id := range q.Relevant {
			rel[id] = true
		}
		ranked, err := rankedIDs(context.Background(), eng, q.Text, cfg, embed, tope)
		if err != nil {
			t.Fatal(err)
		}
		rec += RecallAtK(ranked, rel, 10)
		ndcg += NDCGAtK(ranked, rel, 10)
	}
	n := float64(len(qs))
	return rec / n, ndcg / n
}

func TestBarridoDePesosPorSenal(t *testing.T) {
	ruta := os.Getenv("MUSUBI_FIXTURE_DB")
	dirP := os.Getenv("MUSUBI_POTION_DIR")
	if ruta == "" || dirP == "" {
		t.Skip("faltan MUSUBI_FIXTURE_DB / MUSUBI_POTION_DIR")
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

	train, test := particion(fx.Queries)
	if len(train) < 5 || len(test) < 5 {
		t.Fatalf("partición degenerada (train=%d test=%d): con tan pocas consultas el cruce no dice nada", len(train), len(test))
	}
	t.Logf("corpus %d docs · consultas: %d train / %d test · MMR APAGADO (λ=0)", len(fx.Docs), len(train), len(test))

	// ════════════════════════════════════════════════════════════════════════════════════════
	// EL BARRIDO CORRE CON MMR APAGADO, Y NO ES UN DETALLE: SIN ESO MIDE OTRA COSA
	//
	// Primera corrida, con el λ de producción (0.75): apagar `recencia`, `frecuencia` o
	// `importancia` daba las TRES el mismo +0.0200 de nDCG, hasta el cuarto decimal. Tres señales
	// distintas no coinciden así por casualidad.
	//
	// La causa son dos hechos que por separado son correctos:
	//
	//  1. LAS TRES ESTÁN PLANAS EN EL FIXTURE. `SeedEngine` fija `created_at` a una constante (para
	//     que la evaluación no flakee), guarda sin importancia (todas 1.0) y el harness no bumpea,
	//     así que `access_count` es 0 para todas. Con los valores empatados, `denseRankBy` les da a
	//     TODAS el rango 0: el término es una CONSTANTE idéntica para cada candidato, o sea cero
	//     información.
	//  2. `normalizeScores` DIVIDE POR EL MÁXIMO, no min-max. Eso NO es invariante a restar una
	//     constante: sacársela a todos ENSANCHA la relevancia normalizada, y con la relevancia más
	//     separada MMR diversifica menos.
	//
	// Juntos: apagar una señal plana no mejora el ranking — APAGA UN POCO DE MMR. Y MMR en este
	// fixture cuesta relevancia (medido en mmr_real_test.go). Los tres «+0.0200» eran el costo de
	// MMR entrando por otra puerta, bien calculado y contestando otra pregunta.
	//
	// Con λ=0 el barrido mide la FUSIÓN y nada más. Y queda un control gratis que prueba todo lo
	// anterior: las tres señales planas tienen que dar Δ EXACTAMENTE 0, porque restarle una
	// constante a todos preserva el orden. Si no dan 0, esta explicación es falsa.
	conPesos := func(p memory.PesosRRF) Config {
		o := OptsDeProduccion()
		o.Pesos = &p
		o.MMRLambda = 0 // MMR apagado: ver arriba
		return Config{Name: "pesos", Opts: o, UseVector: true}
	}

	baseTrainR, baseTrainN := mideSobre(t, eng, embed, train, conPesos(memory.PesosUniformes()), len(fx.Docs))
	t.Logf("%-14s %-6s %-10s %-10s %s", "señal", "peso", "R@10", "nDCG@10", "Δ nDCG vs uniforme")
	t.Logf("%-14s %-6s %-10.4f %-10.4f %s", "(uniforme)", "1.0", baseTrainR, baseTrainN, "—  ← la línea base")

	// planas son las señales que este fixture NO PUEDE medir, y decirlo es parte del resultado:
	// `SeedEngine` les da el mismo valor a todos los documentos. Un barrido que las reporte como si
	// las hubiera medido informa «medí siete» habiendo medido cuatro.
	planas := map[string]bool{"recencia": true, "frecuencia": true, "importancia": true}

	type mejor struct {
		senal string
		peso  float64
		ndcg  float64
	}
	top := mejor{senal: "(uniforme)", peso: 1, ndcg: baseTrainN}

	for _, s := range pesosSweep {
		for _, v := range pesosValores {
			p := memory.PesosUniformes()
			s.set(&p, v)
			r, n := mideSobre(t, eng, embed, train, conPesos(p), len(fx.Docs))
			t.Logf("%-14s %-6.1f %-10.4f %-10.4f %+.4f", s.senal, v, r, n, n-baseTrainN)
			if n > top.ndcg {
				top = mejor{senal: s.senal, peso: v, ndcg: n}
			}
			// EL CONTROL QUE PRUEBA EL DIAGNÓSTICO. Las tres señales planas no pueden mover NADA
			// con MMR apagado: su término es constante y restar una constante preserva el orden.
			// Un delta distinto de cero acá significa que la explicación de arriba está mal, y
			// entonces todo lo que sigue se lee distinto.
			if planas[s.senal] && n != baseTrainN {
				t.Errorf("%s=%.1f movió el nDCG a %.4f (base %.4f) con MMR apagado. "+
					"Esa señal está PLANA en el fixture (todos los candidatos empatan), así que su "+
					"término es constante y no puede cambiar el orden. Si se movió, o no está plana "+
					"o el peso entra por un camino que no es el suyo.", s.senal, v, n, baseTrainN)
			}
		}
	}

	// EL CRUCE. El mejor de TRAIN se mide sobre TEST, que no participó de la elección. Es lo único
	// que separa «encontré un peso mejor» de «le encontré la vuelta a estas consultas».
	baseTestR, baseTestN := mideSobre(t, eng, embed, test, conPesos(memory.PesosUniformes()), len(fx.Docs))
	t.Logf("")
	t.Logf("mejor de TRAIN: %s=%.1f (nDCG %.4f vs %.4f uniforme)", top.senal, top.peso, top.ndcg, baseTrainN)

	if top.senal == "(uniforme)" {
		t.Logf("VEREDICTO: ninguna variante le ganó a los pesos uniformes ni siquiera en TRAIN, "+
			"que es la mitad donde se eligió. No hay nada que cruzar. Uniforme se queda. "+
			"(TEST uniforme: R@10 %.4f · nDCG@10 %.4f)", baseTestR, baseTestN)
		return
	}

	p := memory.PesosUniformes()
	for _, s := range pesosSweep {
		if s.senal == top.senal {
			s.set(&p, top.peso)
		}
	}
	testR, testN := mideSobre(t, eng, embed, test, conPesos(p), len(fx.Docs))
	t.Logf("%-24s %-10s %-10s", "en TEST", "R@10", "nDCG@10")
	t.Logf("%-24s %-10.4f %-10.4f", "uniforme", baseTestR, baseTestN)
	t.Logf("%-24s %-10.4f %-10.4f  (Δ %+.4f)", top.senal, testR, testN, testN-baseTestN)

	if testN <= baseTestN {
		t.Logf("VEREDICTO: la mejora NO SOBREVIVE el cruce. Era del fixture, no del ranker. Uniforme se queda.")
		return
	}
	t.Logf("VEREDICTO: la mejora sobrevive el cruce (%+.4f nDCG en TEST). NO alcanza para cambiar el "+
		"default por sí sola: las dos mitades comparten el sesgo del etiquetado por topic_key, y este "+
		"barrido no lo puede descartar. Hay que repetirlo con EtiquetadoPorExpansion cuando haya datos.", testN-baseTestN)
}
