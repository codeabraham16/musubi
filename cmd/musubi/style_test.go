package main

import (
	"strings"
	"sync"
	"testing"
)

// TestApplyColor verifica el envoltorio ANSI: sin habilitar, texto plano; habilitado,
// envuelve con el código SGR y el reset. Es la pieza testeable sin depender del TTY.
func TestApplyColor(t *testing.T) {
	if got := applyColor(false, "32", "hola"); got != "hola" {
		t.Errorf("deshabilitado debe devolver texto plano, obtuve %q", got)
	}
	got := applyColor(true, "32", "hola")
	if !strings.HasPrefix(got, "\x1b[32m") || !strings.HasSuffix(got, "\x1b[0m") || !strings.Contains(got, "hola") {
		t.Errorf("habilitado debe envolver con SGR y reset, obtuve %q", got)
	}
}

// TestColorHelpersPlainWhenDisabled fuerza color off (NO_COLOR) y verifica que los
// helpers de alto nivel no inyectan secuencias de escape — clave para que la salida
// de hooks/pipes quede limpia.
func TestColorHelpersPlainWhenDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// Resetear el memoizado de useColor para que tome el entorno de este test.
	colorOnce = sync.Once{}
	colorOn = false
	for _, fn := range []func(string) string{cGreen, cCyan, cYellow, cBold, cDim} {
		if got := fn("x"); strings.Contains(got, "\x1b[") {
			t.Errorf("con NO_COLOR el helper no debe emitir ANSI, obtuve %q", got)
		}
	}
}
