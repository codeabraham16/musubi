package skills

import (
	"testing"
)

// TestMatchGlob verifica la función exportada MatchGlob con casos tabla:
// match por basename, match por ruta completa, sin match, y patrón inválido.
func TestMatchGlob(t *testing.T) {
	casos := []struct {
		nombre   string
		glob     string
		file     string
		esperado bool
	}{
		{
			nombre:   "match por basename *.go",
			glob:     "*.go",
			file:     "internal/memory/database.go",
			esperado: true,
		},
		{
			nombre:   "match por ruta completa cmd/*",
			glob:     "cmd/*",
			file:     "cmd/main.go",
			esperado: true,
		},
		{
			nombre:   "sin match - extensión diferente",
			glob:     "*.go",
			file:     "README.md",
			esperado: false,
		},
		{
			nombre:   "patrón inválido no hace panic - devuelve false",
			glob:     "[invalido",
			file:     "main.go",
			esperado: false,
		},
		{
			nombre:   "match exacto por basename",
			glob:     "Makefile",
			file:     "sub/Makefile",
			esperado: true,
		},
		{
			nombre:   "match por basename dir/*.ts ignorado (distinto dir)",
			glob:     "src/*.ts",
			file:     "other/file.ts",
			esperado: false,
		},
		{
			nombre:   "match por ruta completa dir/*.ts",
			glob:     "src/*.ts",
			file:     "src/index.ts",
			esperado: true,
		},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			resultado := MatchGlob(tc.glob, tc.file)
			if resultado != tc.esperado {
				t.Errorf("MatchGlob(%q, %q) = %v, esperaba %v", tc.glob, tc.file, resultado, tc.esperado)
			}
		})
	}
}
