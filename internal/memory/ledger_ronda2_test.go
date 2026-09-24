package memory

import (
	"fmt"
	"testing"
	"time"
)

// HALLAZGO (revisión ronda 2, lente producción): las sesiones no vencen nunca —sólo salen por
// desalojo— y el desalojo saca la de MENOR total. Con el uso, las 64 casillas se llenan con las
// sesiones más gordas de la historia (terminales de días anteriores, ya cerradas), que así se
// vuelven inmortales, y toda sesión VIVA que todavía no las alcanza es la candidata a desalojo en
// cuanto escribe cualquier otra. Dos terminales abiertas a la vez se borran la cuenta mutuamente en
// cada escritura: el defecto que el formato por sesión venía a cerrar, de vuelta por el desalojo.
func TestLedgerSaturadoDosTerminalesVivasNoSeBorranEntreSi(t *testing.T) {
	e := newTestEngine(t)
	// 64 sesiones de días anteriores, cerradas, cada una con un día de trabajo encima.
	for i := 0; i < maxSesionesEnLedger; i++ {
		if _, err := e.LedgerAdd(fmt.Sprintf("vieja-%02d", i), "startup_priming", 20000); err != nil {
			t.Fatal(err)
		}
	}
	// Hoy: dos terminales abiertas, A y B, que alternan turnos de 500 tokens.
	for turno := 0; turno < 5; turno++ {
		if _, err := e.LedgerAdd("A", "turn_recall", 500); err != nil {
			t.Fatal(err)
		}
		if _, err := e.LedgerAdd("B", "turn_recall", 500); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := e.LedgerStatusDe("A")
	b, _ := e.LedgerStatusDe("B")
	if a.Total != 2500 {
		t.Errorf("A gastó 2500 en 5 turnos y el ledger dice %d: B la desalojó en cada escritura", a.Total)
	}
	if b.Total != 2500 {
		t.Errorf("B gastó 2500 en 5 turnos y el ledger dice %d", b.Total)
	}
}

// Misma raíz, el caso que la ronda 1 dio por cerrado: la terminal principal lanza un sub-agente con id
// propio y espera. Con el ledger lleno de sesiones históricas más gordas que ella, la PRIMERA
// escritura del sub-agente desaloja a la principal, porque es la única candidata chica.
func TestLedgerSaturadoElSubAgenteNoDesalojaALaPrincipal(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < maxSesionesEnLedger; i++ {
		_, _ = e.LedgerAdd(fmt.Sprintf("vieja-%02d", i), "startup_priming", 20000)
	}
	_, _ = e.LedgerAdd("principal", "startup_priming", 6000)
	_, _ = e.LedgerAdd("sub-1", "precheck_code", 50)
	if p, _ := e.LedgerStatusDe("principal"); p.Total != 6000 {
		t.Errorf("la principal llevaba 6000 y quedó en %d tras la primera escritura del sub-agente", p.Total)
	}
}

// Las sesiones que llevan HORAS sin escribir salen antes que una viva, aunque la viva sea más chica
// y ya no esté entre las últimas que escribieron: sin esto, las más gordas de la historia se vuelven
// inmortales y la viva es la candidata cada vez que el tope se pasa.
func TestLedgerLasSesionesDeAyerSalenAntesQueUnaViva(t *testing.T) {
	e := newTestEngine(t)
	ahora := time.Now()
	relojLedger = func() time.Time { return ahora.Add(-26 * time.Hour) }
	t.Cleanup(func() { relojLedger = time.Now })
	for i := 0; i < maxSesionesEnLedger; i++ {
		if _, err := e.LedgerAdd(fmt.Sprintf("ayer-%02d", i), "startup_priming", 20000); err != nil {
			t.Fatal(err)
		}
	}
	relojLedger = func() time.Time { return ahora }
	if _, err := e.LedgerAdd("viva", "turn_recall", 500); err != nil {
		t.Fatal(err)
	}
	// Más escrituras que las protegidas por recientes: «viva» deja de estar entre las últimas.
	for i := 0; i < sesionesRecientesProtegidas+2; i++ {
		if _, err := e.LedgerAdd(fmt.Sprintf("sub-%02d", i), "precheck_code", 50); err != nil {
			t.Fatal(err)
		}
	}
	if l, _ := e.LedgerStatusDe("viva"); l.Total != 500 {
		t.Fatalf("una sesión viva fue desalojada mientras había sesiones de ayer: total=%d", l.Total)
	}
	sesiones, _ := e.LedgerSesiones()
	if len(sesiones) != maxSesionesEnLedger {
		t.Fatalf("el tope no se respetó: %d sesiones", len(sesiones))
	}
	for _, s := range sesiones {
		if s.UltimaEscritura == "" {
			t.Errorf("la sesión %s no dice cuándo escribió por última vez", s.SessionID)
		}
	}
}
