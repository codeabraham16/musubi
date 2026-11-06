package memory

import (
	"testing"
	"time"
)

// Invariantes del brazo A/B (spec skills-decididor, R9/E8).

func TestBrazoABEsDeterminista(t *testing.T) {
	f := time.Date(2026, 11, 6, 15, 30, 0, 0, time.UTC)
	for _, cohorte := range []string{"", "davantis-1", "musubi-design", "proj-x"} {
		a := BrazoAB(cohorte, f)
		b := BrazoAB(cohorte, f)
		if a != b {
			t.Fatalf("cohorte %q: no determinista, %q != %q", cohorte, a, b)
		}
		if a != BrazoOn && a != BrazoOff {
			t.Fatalf("cohorte %q: brazo inválido %q", cohorte, a)
		}
	}
}

// El crossover es por DÍA, no por instante: dos consultas de la misma cohorte el mismo día UTC
// caen en el mismo brazo aunque cambie la hora. Si no, el tratamiento y el control se mezclarían
// dentro de una sesión.
func TestBrazoABEstableDentroDelDia(t *testing.T) {
	cohorte := "davantis-1"
	manana := time.Date(2026, 11, 6, 1, 0, 0, 0, time.UTC)
	noche := time.Date(2026, 11, 6, 23, 59, 0, 0, time.UTC)
	if BrazoAB(cohorte, manana) != BrazoAB(cohorte, noche) {
		t.Fatalf("el brazo cambió dentro del mismo día UTC")
	}
}

// R9/E8 — CROSSOVER: existe un día en que la misma cohorte cae en el brazo opuesto. Sin esto no hay
// crossover y el análisis pareado no se sostiene. Sabotaje: hashear sólo la cohorte (sin la fecha)
// ⇒ nunca cruza ⇒ rojo.
func TestBrazoABCruzaEntreDias(t *testing.T) {
	cohorte := "davantis-1"
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	primero := BrazoAB(cohorte, base)
	cruzo := false
	for d := 1; d < 60; d++ {
		if BrazoAB(cohorte, base.AddDate(0, 0, d)) != primero {
			cruzo = true
			break
		}
	}
	if !cruzo {
		t.Fatalf("en 60 días la cohorte %q nunca cambió de brazo: no hay crossover", cohorte)
	}
}

// Dos cohortes distintas no caen todas en el mismo brazo el mismo día: el hash reparte. (Control de
// que no degeneró a una constante.)
func TestBrazoABReparteEntreCohortes(t *testing.T) {
	f := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	vistos := map[string]bool{}
	for i := 0; i < 50; i++ {
		vistos[BrazoAB(string(rune('a'+i%26))+time.Duration(i).String(), f)] = true
	}
	if len(vistos) < 2 {
		t.Fatalf("50 cohortes cayeron todas en el mismo brazo: el hash no reparte")
	}
}

// R9 — la clave usa UTC, no la hora local. El MISMO instante expresado en dos husos debe dar el
// mismo brazo. Sabotaje: quitar .UTC() en BrazoAB ⇒ el instante visto en EST formatea el día
// anterior ⇒ brazos distintos ⇒ rojo.
func TestBrazoABUsaUTC(t *testing.T) {
	// En UTC es el día 7; en una zona -05 el mismo instante es todavía el día 6.
	instante := time.Date(2026, 11, 7, 2, 0, 0, 0, time.UTC)
	este := time.FixedZone("EST", -5*3600)
	mismoInstanteOtraZona := instante.In(este) // mismo punto en el tiempo, otra representación
	if BrazoAB("c", instante) != BrazoAB("c", mismoInstanteOtraZona) {
		t.Fatalf("BrazoAB debe normalizar a UTC: el mismo instante dio brazos distintos según la zona")
	}
}
