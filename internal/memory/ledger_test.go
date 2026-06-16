package memory

import "testing"

func TestLedgerAddAndStatus(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.LedgerAdd("s1", "turn_recall", 10); err != nil {
		t.Fatalf("LedgerAdd error: %v", err)
	}
	if _, err := e.LedgerAdd("s1", "hydration", 5); err != nil {
		t.Fatalf("LedgerAdd error: %v", err)
	}
	l, err := e.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus error: %v", err)
	}
	if l.SessionID != "s1" || l.Total != 15 {
		t.Errorf("esperaba session s1 total 15, obtuve %+v", l)
	}
	if l.Surfaces["turn_recall"] != 10 || l.Surfaces["hydration"] != 5 {
		t.Errorf("conteos por superficie incorrectos: %+v", l.Surfaces)
	}
}

func TestLedgerResetsOnNewSession(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	l, err := e.LedgerAdd("s2", "turn_recall", 7)
	if err != nil {
		t.Fatalf("LedgerAdd error: %v", err)
	}
	if l.SessionID != "s2" || l.Total != 7 {
		t.Errorf("una sesión nueva debe reiniciar el ledger; obtuve %+v", l)
	}
}

func TestLedgerEmptySessionKeepsCurrent(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	// sessionID vacío (sin id de hook): acumula bajo la sesión activa, no reinicia.
	l, _ := e.LedgerAdd("", "hydration", 5)
	if l.SessionID != "s1" || l.Total != 15 {
		t.Errorf("sessionID vacío debe acumular en la sesión activa; obtuve %+v", l)
	}
}

func TestLedgerReset(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	if err := e.LedgerReset(); err != nil {
		t.Fatalf("LedgerReset error: %v", err)
	}
	l, _ := e.LedgerStatus()
	if l.Total != 0 || len(l.Surfaces) != 0 {
		t.Errorf("tras reset el ledger debe quedar vacío; obtuve %+v", l)
	}
}
