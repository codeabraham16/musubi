package main

import "testing"

// TestMenuAction verifica el parseo de la elección del menú interactivo:
// L->local, G->global, cualquier otra cosa (incluyendo vacío)->quit, case-insensitive.
func TestMenuAction(t *testing.T) {
	cases := map[string]string{
		"L":     "local",
		"l":     "local",
		" l ":   "local",
		"local": "local",
		"LOCAL": "local",
		"G":     "global",
		"g":     "global",
		"global": "global",
		"Q":     "quit",
		"":      "quit",
		"x":     "quit",
		"otra":  "quit",
	}
	for in, want := range cases {
		if got := menuAction(in); got != want {
			t.Errorf("menuAction(%q) = %q, quiero %q", in, got, want)
		}
	}
}
