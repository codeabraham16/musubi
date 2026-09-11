package memory

import (
	"math"
	"testing"
)

// LOS PESOS POR SEÑAL EXISTEN PARA PODER MEDIR, NO PARA CAMBIAR NADA TODAVÍA.
//
// Hasta acá las siete señales del RRF valían 1.0, y eso NO era una decisión medida: es el default
// del algoritmo, que existe justamente para no tener que elegir pesos. Poder ponderarlas es el
// requisito para averiguar si alguna merece pesar distinto — y hasta que esa medición exista, el
// comportamiento tiene que quedar EXACTAMENTE como estaba.
//
// Por eso la guarda central de este archivo no es sobre los pesos: es sobre la IDENTIDAD.

// scoreDe devuelve el score de un id dentro de un resultado.
func scoreDe(scored []scoredCandidate, id string) float64 {
	for _, s := range scored {
		if s.id == id {
			return s.score
		}
	}
	return math.NaN()
}

// A — CON PESOS AUSENTES, LA FÓRMULA ES LA DE ANTES, AL BIT.
//
// La aserción compara contra el valor calculado A MANO con la fórmula histórica, y no contra otra
// llamada a la misma función: comparar la función consigo misma no distingue «no cambió» de
// «cambiaron las dos igual». Si alguien toca un coeficiente, acá se ve.
//
// El candidato es UNO SOLO y sin fecha a propósito: con un solo candidato todos los rangos densos
// valen 0, así que el valor esperado no depende del orden ni de la edad y la aserción queda exacta
// en vez de aproximada.
func TestConPesosAusentesLaFormulaEsLaHistorica(t *testing.T) {
	cands := []candidate{{id: "a", importance: 1}}
	lex := map[string]int{"a": 0}
	vec := map[string]int{"a": 0}
	graf := map[string]int{"a": 0}
	cooc := map[string]int{"a": 0}

	got := scoreDe(scoreCandidates(cands, lex, vec, graf, cooc, nil, testNow()), "a")

	// Las SIETE señales con rango 0 y peso 1: recencia, frecuencia, léxico, vector, grafo,
	// co-ocurrencia e importancia. El factor de edad absoluta es 1.0 porque createdAt está vacío
	// (edad desconocida ⇒ sin castigo, la degradación segura de ageDays).
	quiero := 7 * (1.0 / float64(rrfK))
	if math.Abs(got-quiero) > 1e-12 {
		t.Errorf("el score con pesos ausentes cambió respecto de la fórmula histórica.\n  obtuve:  %.15f\n  esperaba: %.15f (7 señales × 1/(rrfK=%d))", got, quiero, rrfK)
	}
}

// B — `nil` Y `PesosUniformes()` SON LA MISMA COSA.
//
// Es la otra mitad de la identidad: sin esto, «nil = uniformes» sería una afirmación de comentario.
// Y es lo que permite que el banco pase pesos explícitos sin que la línea base se le mueva.
func TestPesosNilYUniformesSonIndistinguibles(t *testing.T) {
	cands := []candidate{
		{id: "a", importance: 1, accessCount: 3, createdAt: "2026-01-01 00:00:00"},
		{id: "b", importance: 2, accessCount: 0, createdAt: "2026-06-01 00:00:00"},
		{id: "c", importance: 1, accessCount: 9, createdAt: "2025-01-01 00:00:00"},
	}
	lex := map[string]int{"a": 0, "b": 1, "c": 2}
	vec := map[string]int{"b": 0}

	u := PesosUniformes()
	conNil := scoreCandidates(cands, lex, vec, nil, nil, nil, testNow())
	conUni := scoreCandidates(cands, lex, vec, nil, nil, &u, testNow())

	if len(conNil) != len(conUni) {
		t.Fatalf("distinta cantidad de resultados: %d vs %d", len(conNil), len(conUni))
	}
	for i := range conNil {
		if conNil[i].id != conUni[i].id || conNil[i].score != conUni[i].score {
			t.Errorf("posición %d difiere: nil=(%s, %.15f) uniformes=(%s, %.15f)",
				i, conNil[i].id, conNil[i].score, conUni[i].id, conUni[i].score)
		}
	}
}

// C — UN PESO EN CERO APAGA SU SEÑAL, Y ESO ES DELIBERADO.
//
// Es la semántica que hace posible la medición: «¿cuánto aporta el vector?» se responde poniéndolo
// en 0 y mirando el delta. La alternativa —tratar el cero como 1.0 para que nadie se lastime— haría
// que la señal NO SE PUEDA APAGAR, y entonces el barrido mediría siempre lo mismo.
//
// EL FILO ESTÁ EN EL PUNTERO, no en el cero: `nil` es «no opinó» y da uniformes; un struct con
// ceros es «lo dije». Sin el puntero, el valor cero de cualquier caller que no conozca el campo
// apagaría las siete señales y el ranker devolvería el pool en el orden en que llegó — sin fallar,
// contestando cualquier cosa.
func TestUnPesoEnCeroApagaSuSenal(t *testing.T) {
	cands := []candidate{{id: "a", importance: 1}, {id: "b", importance: 1}}
	// Sólo 'a' tiene rango vectorial: con peso 1 le da ventaja, con peso 0 no le da nada.
	vec := map[string]int{"a": 0}

	u := PesosUniformes()
	conVector := scoreDe(scoreCandidates(cands, nil, vec, nil, nil, &u, testNow()), "a")

	sinVector := u
	sinVector.Vector = 0
	apagado := scoreDe(scoreCandidates(cands, nil, vec, nil, nil, &sinVector, testNow()), "a")

	if !(conVector > apagado) {
		t.Errorf("con Vector=0 el score de 'a' tendría que BAJAR (pierde el aporte de esa señal): con=%.15f apagado=%.15f", conVector, apagado)
	}
	// Y el delta tiene que ser EXACTAMENTE el término que se sacó, no «algo menos»: si fuera otro
	// número, el peso estaría entrando por un camino que no es el suyo.
	if d := conVector - apagado; math.Abs(d-1.0/float64(rrfK)) > 1e-12 {
		t.Errorf("el delta de apagar Vector fue %.15f, esperaba exactamente 1/(rrfK=%d)=%.15f", d, rrfK, 1.0/float64(rrfK))
	}
}

// D — CADA PESO GOBIERNA SU PROPIA SEÑAL, Y NINGUNA OTRA.
//
// Siete términos y un solo copy-paste mal hecho alcanza para que dos señales compartan peso — y el
// síntoma sería un barrido que atribuye a una lo que produjo la otra, o sea una conclusión numérica
// perfectamente calculada sobre la pregunta equivocada. La tabla recorre las siete.
func TestCadaPesoGobiernaSuPropiaSenal(t *testing.T) {
	casos := []struct {
		nombre               string
		aplica               func(*PesosRRF)
		lex, vec, graf, cooc map[string]int
	}{
		{"Lexico", func(p *PesosRRF) { p.Lexico = 0 }, map[string]int{"a": 0}, nil, nil, nil},
		{"Vector", func(p *PesosRRF) { p.Vector = 0 }, nil, map[string]int{"a": 0}, nil, nil},
		{"Grafo", func(p *PesosRRF) { p.Grafo = 0 }, nil, nil, map[string]int{"a": 0}, nil},
		{"Coocurrencia", func(p *PesosRRF) { p.Coocurrencia = 0 }, nil, nil, nil, map[string]int{"a": 0}},
		{"Recencia", func(p *PesosRRF) { p.Recencia = 0 }, nil, nil, nil, nil},
		{"Frecuencia", func(p *PesosRRF) { p.Frecuencia = 0 }, nil, nil, nil, nil},
		{"Importancia", func(p *PesosRRF) { p.Importancia = 0 }, nil, nil, nil, nil},
	}
	cands := []candidate{{id: "a", importance: 1}}
	esperado := 1.0 / float64(rrfK)

	for _, c := range casos {
		u := PesosUniformes()
		base := scoreDe(scoreCandidates(cands, c.lex, c.vec, c.graf, c.cooc, &u, testNow()), "a")
		p := PesosUniformes()
		c.aplica(&p)
		sin := scoreDe(scoreCandidates(cands, c.lex, c.vec, c.graf, c.cooc, &p, testNow()), "a")
		if d := base - sin; math.Abs(d-esperado) > 1e-12 {
			t.Errorf("%s: apagarlo movió el score en %.15f, esperaba exactamente %.15f. "+
				"Si es 0, ese peso no gobierna nada; si es el doble, gobierna también otra señal.",
				c.nombre, d, esperado)
		}
	}
}
