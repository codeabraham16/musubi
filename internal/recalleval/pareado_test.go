package recalleval

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func scoresDe(nombre string, rr map[string]float64) Scores {
	pc := make(map[string]MetricasConsulta, len(rr))
	for id, v := range rr {
		pc[id] = MetricasConsulta{RR: v, RecallAtK: map[int]float64{10: v}, NDCGAtK: map[int]float64{10: v}}
	}
	return Scores{Config: nombre, PorConsulta: pc}
}

// TestCompararPareadoDistingueLoQueElPromedioEsconde es el caso que justifica todo el archivo: dos
// brazos con el MISMO promedio y comportamientos opuestos.
func TestCompararPareadoDistingueLoQueElPromedioEsconde(t *testing.T) {
	// A: todas las consultas en 0.5.
	a := scoresDe("base", map[string]float64{"q1": 0.5, "q2": 0.5, "q3": 0.5, "q4": 0.5})
	// B: mismo promedio (0.5), pero una consulta se fue al cielo y tres al piso.
	b := scoresDe("parejo", map[string]float64{"q1": 0.5, "q2": 0.5, "q3": 0.5, "q4": 0.5})
	c := scoresDe("desparejo", map[string]float64{"q1": 1.4, "q2": 0.2, "q3": 0.2, "q4": 0.2})

	cmpParejo, err := CompararPareado(a, b, "rr", 0)
	if err != nil {
		t.Fatal(err)
	}
	if cmpParejo.Gana != 0 || cmpParejo.Pierde != 0 || cmpParejo.Empata != 4 {
		t.Errorf("brazos idénticos: esperaba 0/0/4, obtuve %d/%d/%d", cmpParejo.Gana, cmpParejo.Pierde, cmpParejo.Empata)
	}

	cmpDesparejo, err := CompararPareado(a, c, "rr", 0)
	if err != nil {
		t.Fatal(err)
	}
	// El promedio de los dos es idéntico (0.5), y sin embargo uno movió TODO.
	if cmpDesparejo.Gana != 1 || cmpDesparejo.Pierde != 3 {
		t.Errorf("esperaba gana=1 pierde=3, obtuve gana=%d pierde=%d", cmpDesparejo.Gana, cmpDesparejo.Pierde)
	}
	if d := cmpDesparejo.DeltaMedio; d < -1e-9 || d > 1e-9 {
		t.Errorf("el delta medio tiene que ser ~0 (por eso el promedio engaña): obtuve %+.6f", d)
	}
	// Y tiene que decir DÓNDE: la peor caída primero.
	if len(cmpDesparejo.PeoresCaidas) == 0 || cmpDesparejo.PeoresCaidas[0].Delta >= 0 {
		t.Error("no reportó las consultas que empeoraron, que es lo único accionable")
	}
}

// TestCompararPareadoAvisaCuandoLaMuestraEsRuido fija la corrección del juez: con pocos pares
// discordantes el conteo NO es un veredicto, y la salida tiene que decirlo.
func TestCompararPareadoAvisaCuandoLaMuestraEsRuido(t *testing.T) {
	a := scoresDe("base", map[string]float64{"q1": 0.5, "q2": 0.5, "q3": 0.5})
	b := scoresDe("otro", map[string]float64{"q1": 0.9, "q2": 0.5, "q3": 0.5})
	cmp, err := CompararPareado(a, b, "rr", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmp.String(), "ruido, no señal") {
		t.Errorf("con 1 par discordante la salida tiene que advertir que el conteo es ruido; salida:\n%s", cmp.String())
	}
}

// TestCompararPareadoExigeElDetalle: sin PorConsulta no se puede comparar de a pares, y hay que
// decirlo en vez de devolver ceros con cara de resultado.
func TestCompararPareadoExigeElDetalle(t *testing.T) {
	if _, err := CompararPareado(Scores{Config: "a"}, Scores{Config: "b"}, "rr", 0); err == nil {
		t.Error("sin PorConsulta tiene que fallar, no devolver un cero que parece medición")
	}
}

// scoresRR arma Scores con sólo RR por consulta, para las pruebas de significancia.
func scoresRR(nombre string, rr map[string]float64) Scores {
	pc := make(map[string]MetricasConsulta, len(rr))
	for id, v := range rr {
		pc[id] = MetricasConsulta{RR: v}
	}
	return Scores{Config: nombre, PorConsulta: pc}
}

// TestPSignoDaLaBinomialExacta fija los valores contra la cuenta a mano, incluido el «~10-2 para
// p<0,05» que el encabezado de pareado.go afirma: 10-2 tiene que pasar ese umbral y 9-2 no.
//
// Sabotaje: el test de signo a UNA cola en vez de a dos (el p sale la mitad de chico).
// arnes: archivo="internal/recalleval/pareado.go"
// arnes: de="return math.Min(1, 2*cola)"
// arnes: a="return math.Min(1, cola)"
func TestPSignoDaLaBinomialExacta(t *testing.T) {
	casos := []struct {
		gana, pierde, empata int
		quiero               float64
	}{
		{10, 0, 0, 2.0 / 1024},      // 2·C(10,0)/2^10
		{8, 2, 5, 2.0 * 56 / 1024},  // 2·(1+10+45)/2^10 — los empates no cuentan
		{10, 2, 0, 2.0 * 79 / 4096}, // 2·(1+12+66)/2^12 = 0,0386
		{9, 2, 0, 2.0 * 67 / 2048},  // 2·(1+11+55)/2^11 = 0,0654
		{3, 3, 0, 1},                // simétrico: la cola doble pasa de 1 y se recorta
		{0, 0, 12, 1},               // sin discordantes no hay evidencia
		{0, 7, 1, 2.0 / 128},        // el sentido no importa: es a dos colas
	}
	for _, c := range casos {
		cmp := ComparacionPareada{Gana: c.gana, Pierde: c.pierde, Empata: c.empata}
		if got := cmp.PSigno(); math.Abs(got-c.quiero) > 1e-12 {
			t.Errorf("PSigno(gana %d, pierde %d, empata %d) = %.6f, quiero %.6f", c.gana, c.pierde, c.empata, got, c.quiero)
		}
	}
	if p := (ComparacionPareada{Gana: 10, Pierde: 2}).PSigno(); p >= 0.05 {
		t.Errorf("10-2 tiene que dar p<0,05 (lo afirma el encabezado); dio %.4f", p)
	}
	if p := (ComparacionPareada{Gana: 9, Pierde: 2}).PSigno(); p < 0.05 {
		t.Errorf("9-2 NO tiene que dar p<0,05; dio %.4f", p)
	}
	// Con muchas consultas no desborda (C(1000, 500) no entra en un float64).
	if p := (ComparacionPareada{Gana: 500, Pierde: 500}).PSigno(); p != 1 {
		t.Errorf("500-500 tiene que dar 1; dio %v", p)
	}
	if p := (ComparacionPareada{Gana: 600, Pierde: 400}).PSigno(); !(p > 0 && p < 1e-8) {
		t.Errorf("600-400 tiene que dar un p minúsculo y positivo; dio %v", p)
	}
}

// TestDeltasSalenOrdenadosPorConsulta: el bootstrap remuestrea por posición, así que el orden de
// Deltas tiene que ser el del id y no el del mapa.
//
// Sabotaje: recorrer las consultas en el orden del mapa.
// arnes: archivo="internal/recalleval/pareado.go"
// arnes: de="\tsort.Strings(ids) // ver Deltas: el orden del mapa cambia entre corridas\n"
// arnes: a=""
func TestDeltasSalenOrdenadosPorConsulta(t *testing.T) {
	a := scoresRR("a", map[string]float64{"q3": 0, "q1": 0, "q2": 0, "q4": 0})
	b := scoresRR("b", map[string]float64{"q3": 0.3, "q1": 0.1, "q2": 0.2})
	for i := 0; i < 20; i++ { // el orden del mapa cambia entre recorridos: se prueba varias veces
		cmp, err := CompararPareado(a, b, "rr", 0)
		if err != nil {
			t.Fatal(err)
		}
		quiero := []float64{0.1, 0.2, 0.3} // q1, q2, q3; q4 no está en B y no se compara
		if len(cmp.Deltas) != len(quiero) {
			t.Fatalf("Deltas = %v, quiero %v", cmp.Deltas, quiero)
		}
		for j := range quiero {
			if math.Abs(cmp.Deltas[j]-quiero[j]) > 1e-12 {
				t.Fatalf("Deltas = %v, quiero %v (ordenados por id de consulta)", cmp.Deltas, quiero)
			}
		}
	}
}

// TestIntervaloBootstrapEsDeterministaYRodeaAlDelta: misma semilla ⇒ mismo intervalo, y el
// intervalo contiene al delta medio observado.
//
// Sabotaje: el borde superior del intervalo es el mismo percentil que el inferior.
// arnes: archivo="internal/recalleval/pareado.go"
// arnes: de="iHi := int(math.Ceil((1-alfa)*float64(reps))) - 1"
// arnes: a="iHi := iLo"
func TestIntervaloBootstrapEsDeterministaYRodeaAlDelta(t *testing.T) {
	rr := map[string]float64{}
	base := map[string]float64{}
	for i := 0; i < 26; i++ {
		id := fmt.Sprintf("q%02d", i)
		base[id] = 0.5
		rr[id] = 0.5 + float64(i%5-1)*0.1 // mezcla de subas, bajas y empates
	}
	cmp, err := CompararPareado(scoresRR("a", base), scoresRR("b", rr), "rr", 0)
	if err != nil {
		t.Fatal(err)
	}
	lo1, hi1 := cmp.IntervaloBootstrap(5000, 42, 0.95)
	for i := 0; i < 5; i++ {
		cmp2, _ := CompararPareado(scoresRR("a", base), scoresRR("b", rr), "rr", 0)
		lo2, hi2 := cmp2.IntervaloBootstrap(5000, 42, 0.95)
		if lo1 != lo2 || hi1 != hi2 {
			t.Fatalf("misma comparación y misma semilla dieron intervalos distintos: [%v,%v] vs [%v,%v]", lo1, hi1, lo2, hi2)
		}
	}
	if !(lo1 <= cmp.DeltaMedio && cmp.DeltaMedio <= hi1) || !(lo1 < hi1) {
		t.Errorf("el IC95 [%v, %v] tiene que rodear al delta medio %v y tener ancho", lo1, hi1, cmp.DeltaMedio)
	}
	// Deltas todos iguales: el intervalo colapsa en ese valor.
	todos := map[string]float64{"x": 0.7, "y": 0.7, "z": 0.7}
	cero := map[string]float64{"x": 0.5, "y": 0.5, "z": 0.5}
	c2, _ := CompararPareado(scoresRR("a", cero), scoresRR("b", todos), "rr", 0)
	if lo, hi := c2.IntervaloBootstrap(1000, 1, 0.95); math.Abs(lo-0.2) > 1e-12 || math.Abs(hi-0.2) > 1e-12 {
		t.Errorf("con todos los deltas en 0,2 el intervalo tiene que ser [0,2, 0,2]; dio [%v, %v]", lo, hi)
	}
	if lo, hi := (ComparacionPareada{}).IntervaloBootstrap(1000, 1, 0.95); lo != 0 || hi != 0 {
		t.Errorf("sin deltas tiene que dar (0, 0); dio (%v, %v)", lo, hi)
	}
}
