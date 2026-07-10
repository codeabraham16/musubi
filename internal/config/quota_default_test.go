package config

import "testing"

// TestEffectiveQuotaPerMinute valida la semántica de la cuota-ON-default de Track 18: 0 ⇒ default
// (protección encendida), negativo ⇒ sin límite (opt-out explícito), positivo ⇒ ese valor.
func TestEffectiveQuotaPerMinute(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, defaultServiceQuotaPerMinute}, // omitido ⇒ default ON
		{-1, 0},                           // opt-out explícito ⇒ sin límite
		{300, 300},                        // valor explícito ⇒ respetado
	}
	for _, c := range cases {
		if got := (ServiceConfig{QuotaPerMinute: c.in}).EffectiveQuotaPerMinute(); got != c.want {
			t.Errorf("EffectiveQuotaPerMinute(%d)=%d, esperaba %d", c.in, got, c.want)
		}
	}
}
