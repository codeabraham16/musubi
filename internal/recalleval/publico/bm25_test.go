package publico

import (
	"math"
	"reflect"
	"testing"
)

// La réplica de BM25Okapi contra un corpus hecho a mano, con la cuenta de rank_bm25 hecha a mano:
//
//	docs: d0="a b", d1="a c", d2="d"  → N=3, avgdl=5/3
//	idf(a) = ln(1,5) − ln(2,5) = −0,5108256 → NEGATIVO: está en 2 de 3 docs
//	idf(b) = idf(c) = idf(d) = ln(2,5) − ln(1,5) = 0,5108256
//	average_idf = (−0,5108256 + 3·0,5108256)/4 = 0,2554128 → piso = 0,25·0,2554128 = 0,0638532
//	con dl=2: tf·(k1+1)/(tf + k1·(1−b+b·dl/avgdl)) = 2,5/2,725 = 0,9174312
//	consulta "a b": d0 = (0,0638532 + 0,5108256)·0,9174312 = 0,5272285; d1 = 0,0638532·0,9174312
//	= 0,0585809; d2 = 0.
//
// El piso importa para el ORDEN, no sólo para el valor: sin él, «a» resta, d1 queda en −0,4686 y
// cae por debajo de d2, que no tiene ninguna palabra de la consulta.
//
// Sabotaje: el idf negativo queda negativo, sin el piso de rank_bm25.
// arnes: archivo="internal/recalleval/publico/bm25.go"
// arnes: de="\t\tm.idf[w] = piso\n"
// arnes: a="\t\tm.idf[w] += 0 * piso\n"
func TestBM25OkapiReplicaRankBM25(t *testing.T) {
	m := NuevoBM25Okapi([][]string{Tokens("a b"), Tokens("a c"), Tokens("d")})
	s := m.Puntajes(Tokens("a b"))
	quiero := []float64{0.5272285, 0.0585809, 0}
	for i := range quiero {
		if math.Abs(s[i]-quiero[i]) > 1e-6 {
			t.Fatalf("puntajes = %v, quiero %v (hechos a mano, ver el comentario)", s, quiero)
		}
	}
	if got := RankingBM25(s); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Fatalf("ranking = %v, quiero [0 1 2]", got)
	}
	// Una palabra que no está en el corpus suma 0 (`self.idf.get(q) or 0`), y una repetida en la
	// consulta suma dos veces.
	if s := m.Puntajes(Tokens("zzz")); s[0] != 0 || s[1] != 0 || s[2] != 0 {
		t.Fatalf("una palabra fuera del corpus sumó: %v", s)
	}
	uno, dos := m.Puntajes(Tokens("b")), m.Puntajes(Tokens("b b"))
	if math.Abs(dos[0]-2*uno[0]) > 1e-12 {
		t.Fatalf("la palabra repetida no suma dos veces: %v contra %v", dos[0], uno[0])
	}
}

// La tokenización del paper es `split(" ")`: sin minúsculas, sin puntuación, y con cadenas vacías
// entre espacios seguidos. Es el baseline que ellos midieron; «arreglarla» mediría otro.
func TestTokensEsElSplitDePython(t *testing.T) {
	if got := Tokens("What  degree?"); !reflect.DeepEqual(got, []string{"What", "", "degree?"}) {
		t.Fatalf("Tokens = %q", got)
	}
	m := NuevoBM25Okapi([][]string{Tokens("what x"), Tokens("y z")})
	if s := m.Puntajes(Tokens("What")); s[0] != 0 {
		t.Fatalf("«What» encontró a «what»: la réplica pasó a minúsculas y el paper no (%v)", s)
	}
}

// A igual puntaje, la posición MAYOR primero: el reverso de un orden ascendente estable. Es
// determinista, que es todo lo que se puede pedir sin portar el introsort de numpy; el medidor
// cuenta cuándo un empate cruza el corte.
func TestRankingBM25DesempataYAvisa(t *testing.T) {
	s := []float64{1, 0, 1, 0}
	r := RankingBM25(s)
	if !reflect.DeepEqual(r, []int{2, 0, 3, 1}) {
		t.Fatalf("ranking = %v, quiero [2 0 3 1]", r)
	}
	if !EmpateEnCorte(r, s, 1) {
		t.Error("@1 el primero y el segundo empatan en 1: el corte depende del desempate")
	}
	if EmpateEnCorte(r, s, 2) {
		t.Error("@2 el corte separa un 1 de un 0: no hay empate en el corte")
	}
	if EmpateEnCorte(r, s, 4) {
		t.Error("@k con k ≥ largo no hay nadie afuera: no puede haber empate en el corte")
	}
}
