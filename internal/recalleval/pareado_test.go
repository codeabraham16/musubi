package recalleval

import (
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
