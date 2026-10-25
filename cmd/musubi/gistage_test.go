package main

import (
	"testing"
	"time"
)

// TestGistAgeLeeLosLayoutsDeLaMemoria fija el CONTRATO del parser de edad: las tres formas en que
// una created_at llega desde la memoria tienen que dar una edad, y cualquier otra cosa degrada a ""
// (nunca una edad inventada). El layout «2006-01-02» es, además, EXACTAMENTE lo que empaqueta
// soloFecha (internal/memory) en RecallItem.CreatedAt; TestLaNotaBajadaConservaSuEdad cierra el
// lazo productor→parser por el camino real.
func TestGistAgeLeeLosLayoutsDeLaMemoria(t *testing.T) {
	const dia = 24 * time.Hour
	hace := func(d time.Duration, layout string) string {
		return time.Now().UTC().Add(-d).Format(layout)
	}
	casos := []struct {
		nombre  string
		entrada string
		quiero  string
	}{
		{"dia a secas (soloFecha) hoy", hace(0, "2006-01-02"), " · hoy"},
		{"dia a secas (soloFecha) 5 dias", hace(5*dia, "2006-01-02"), " · hace 5d"},
		{"layout de la base 90 dias", hace(90*dia, "2006-01-02 15:04:05"), " · hace 3m"},
		{"RFC3339 2 anios", hace(800*dia, time.RFC3339), " · hace 2a"},
		{"basura degrada a vacio", "no-es-una-fecha", ""},
		{"vacio degrada a vacio", "", ""},
		{"fecha futura degrada a vacio", hace(-10*dia, "2006-01-02 15:04:05"), ""},
	}
	for _, c := range casos {
		if got := gistAge(c.entrada); got != c.quiero {
			t.Errorf("%s: gistAge(%q) = %q; quiero %q", c.nombre, c.entrada, got, c.quiero)
		}
	}
}
