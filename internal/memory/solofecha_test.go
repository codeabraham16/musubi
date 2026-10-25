package memory

import "testing"

// TestSoloFecha fija el recorte de la marca de tiempo a su fecha: es lo que viaja en
// RecallItem.CreatedAt (specs/recall-envoltorio/). La EDAD de una memoria se lee en días; la hora es
// envoltorio que el agente re-lee en cada turno sin usarlo. Defensivo: lo que no tiene forma de fecha
// ISO se devuelve TAL CUAL, nunca se recorta a ciegas.
//
// Sabotaje: quitar la guarda y hacer siempre `return ts[:10]` ⇒ "" y "ayer" (len < 10) hacen panic.
// Las dos condiciones de la guarda son cada una load-bearing: aflojar a `len(ts)>=10` a secas recorta
// "1695000000123"; cambiar `&&` por `||` recorta "2026-06/29 10:11:12". Ambos casos lo cazan.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="len(ts) >= 10 && ts[4] == '-' && ts[7] == '-'"
// arnes: a="len(ts) >= 10"
func TestSoloFecha(t *testing.T) {
	casos := []struct{ in, want string }{
		{"2026-06-29 10:11:12", "2026-06-29"},          // layout de SQLite (espacio)
		{"2026-06-29T10:11:12Z", "2026-06-29"},         // RFC3339
		{"2026-06-29T07:11:12-03:00", "2026-06-29"},    // RFC3339 con zona
		{"2026-06-29", "2026-06-29"},                   // ya es fecha: idempotente
		{"", ""},                                       // sin fecha: se omite igual
		{"ayer", "ayer"},                               // no es fecha: no se toca
		{"1695000000", "1695000000"},                   // epoch (len 10 pero sin '-' en 4 y 7): no se toca
		{"2026/06/29", "2026/06/29"},                   // separador distinto: no se toca
		{"1695000000123", "1695000000123"},             // epoch ms (len>10, sin guiones): NO recortar — mata al mutante `len(ts)>=10` a secas
		{"2026-06/29 10:11:12", "2026-06/29 10:11:12"}, // un solo guión (pos 4, no 7): NO tocar — mata al mutante `&&`→`||`
	}
	for _, c := range casos {
		if got := soloFecha(c.in); got != c.want {
			t.Errorf("soloFecha(%q) = %q; esperaba %q", c.in, got, c.want)
		}
	}
}
