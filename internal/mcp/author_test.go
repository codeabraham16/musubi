package mcp

import "testing"

// TestAuthorFrom valida la derivación de la atribución por PERSONA (C5.1 / R3.2, R3.6): author es
// el nombre del principal, salvo nil (stdio/local sin auth) o el admin legacy (token único sin
// identidad de persona) ⇒ "". Un admin NOMBRADO conserva su nombre. Nunca del cliente.
func TestAuthorFrom(t *testing.T) {
	cases := []struct {
		desc string
		p    *Principal
		want string
	}{
		{"nil (stdio local)", nil, ""},
		{"legacy admin (sin identidad)", &Principal{Name: "legacy", Role: RoleAdmin}, ""},
		{"writer nombrado", &Principal{Name: "ana", ProjectID: "acme", Role: RoleWriter}, "ana"},
		{"admin nombrado (es persona)", &Principal{Name: "davantis", Role: RoleAdmin}, "davantis"},
	}
	for _, tc := range cases {
		if got := authorFrom(tc.p); got != tc.want {
			t.Errorf("%s: authorFrom = %q, esperaba %q", tc.desc, got, tc.want)
		}
	}
}
