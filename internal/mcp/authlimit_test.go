package mcp

import (
	"testing"
	"time"
)

func TestAuthLimiterLockout(t *testing.T) {
	base := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	l := newAuthLimiter(3, time.Minute)
	const ip = "100.79.0.9"

	// Antes de fallar: no bloqueada.
	if l.locked(ip, base) {
		t.Fatal("una IP sin fallos no debería estar bloqueada")
	}
	// 2 fallos: aún no bloquea (umbral 3).
	l.fail(ip, base)
	l.fail(ip, base)
	if l.locked(ip, base) {
		t.Fatal("2 fallos < 3: no debería bloquear todavía")
	}
	// 3er fallo: bloquea.
	l.fail(ip, base)
	if !l.locked(ip, base) {
		t.Fatal("al 3er fallo debería bloquear")
	}
	// Sigue bloqueada dentro de la ventana.
	if !l.locked(ip, base.Add(30*time.Second)) {
		t.Fatal("debería seguir bloqueada dentro de la ventana")
	}
	// Expira pasada la ventana.
	if l.locked(ip, base.Add(2*time.Minute)) {
		t.Fatal("pasada la ventana ya no debería estar bloqueada")
	}

	// Otra IP no se ve afectada.
	other := "100.79.0.10"
	l.fail(other, base)
	l.fail(other, base)
	if l.locked(other, base) {
		t.Fatal("una IP distinta no debería heredar el lockout")
	}
	// reset limpia el contador: tras 2 fallos + reset, hacen falta 3 nuevos.
	l.reset(other)
	l.fail(other, base)
	l.fail(other, base)
	if l.locked(other, base) {
		t.Fatal("tras reset, 2 fallos no deberían bloquear")
	}

	// nil-safety: un limiter nil o una ip vacía nunca bloquean ni panican.
	var nilLim *authLimiter
	if nilLim.locked("x", base) {
		t.Fatal("limiter nil no debería bloquear")
	}
	nilLim.fail("x", base)
	nilLim.reset("x")
}
