package memory

import "testing"

func TestSimilarityIdentical(t *testing.T) {
	if s := Similarity("hola mundo", "hola mundo"); s != 1.0 {
		t.Errorf("idénticos deberían dar 1.0, obtuve %v", s)
	}
	if s := Similarity("", ""); s != 1.0 {
		t.Errorf("ambos vacíos deberían dar 1.0, obtuve %v", s)
	}
}

func TestSimilarityNormalizes(t *testing.T) {
	if s := Similarity("hola mundo", "  HOLA   mundo "); s != 1.0 {
		t.Errorf("debería normalizar mayúsculas/espacios a 1.0, obtuve %v", s)
	}
}

func TestSimilarityNearDuplicateHigh(t *testing.T) {
	a := "el patron singleton garantiza una sola instancia en go"
	b := "el patron singleton garantiza una sola instancia en go!"
	if s := Similarity(a, b); s < 0.7 {
		t.Errorf("casi-duplicados deberían dar alta similitud (>0.7), obtuve %v", s)
	}
}

func TestSimilarityDifferentLow(t *testing.T) {
	a := "autenticacion con jwt y refresh tokens"
	b := "optimizacion de indices en la base de datos"
	if s := Similarity(a, b); s > 0.3 {
		t.Errorf("textos distintos deberían dar baja similitud (<0.3), obtuve %v", s)
	}
}
