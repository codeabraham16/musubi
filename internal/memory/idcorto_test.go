package memory

import "testing"

// Sabotaje: si IDCorto deja de recortar (return id), el UUID viaja entero y este test cae; eso
// también lo cazaría el emisor, pero acá se fija la función pura.
// arnes: archivo="internal/memory/idcorto.go"
// arnes: de="return id[:8]"
// arnes: a="return id"
func TestIDCorto(t *testing.T) {
	casos := []struct{ in, want string }{
		// UUID canónico en minúscula: se reduce a los primeros 8 hex.
		{"2c580abd-1b3f-422c-8e5e-347eb568a584", "2c580abd"},
		{"d85d7fc3-d3fc-4ac8-8a92-52b92d45c0e6", "d85d7fc3"},
		// No-UUID: viaja verbatim.
		{"commit-dc5a4ed0467f603e", "commit-dc5a4ed0467f603e"},
		{"a1", "a1"},
		{"", ""},
		// Ya corto (8 hex, no es un UUID de 36): verbatim.
		{"2c580abd", "2c580abd"},
		// UUID en MAYÚSCULA: NO se acorta (lado seguro: el id entero resuelve por el camino exacto).
		{"2C580ABD-1B3F-422C-8E5E-347EB568A584", "2C580ABD-1B3F-422C-8E5E-347EB568A584"},
		// Casi-UUID con un no-hex en los primeros 8: verbatim.
		{"2c580abg-1b3f-422c-8e5e-347eb568a584", "2c580abg-1b3f-422c-8e5e-347eb568a584"},
		// Largo 36 pero con un guion fuera de lugar (posición 8 no es '-'): verbatim.
		{"2c580abd+1b3f-422c-8e5e-347eb568a584", "2c580abd+1b3f-422c-8e5e-347eb568a584"},
	}
	for _, c := range casos {
		if got := IDCorto(c.in); got != c.want {
			t.Errorf("IDCorto(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}
