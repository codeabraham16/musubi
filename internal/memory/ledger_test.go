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

func TestBudgetReport(t *testing.T) {
	l := TokenLedger{
		SessionID: "s1",
		Total:     8000,
		Surfaces:  map[string]int{"startup_cognitive": 5000, "turn_recall": 2000, "startup_priming": 1000},
	}

	// Con presupuesto excedido: estado "over", restante negativo, % usado 100.
	b := l.Budget(8000)
	if b.Status != "over" {
		t.Errorf("8000/8000 debe ser 'over', obtuve %q", b.Status)
	}
	if b.Budget != 8000 || b.Remaining != 0 || b.PctUsed != 100 {
		t.Errorf("budget/remaining/pct incorrectos: %+v", b)
	}
	// Desglose ordenado por gasto desc, con % del total.
	if len(b.Surfaces) != 3 || b.Surfaces[0].Surface != "startup_cognitive" {
		t.Fatalf("esperaba 3 superficies con la mayor primero, obtuve %+v", b.Surfaces)
	}
	if b.Surfaces[0].Pct != 63 { // 5000/8000 = 62.5 -> 63 (redondeo)
		t.Errorf("pct de la superficie top: esperaba 63, obtuve %d", b.Surfaces[0].Pct)
	}

	// Estado "watch" a partir del 75%.
	if got := l.Budget(10000).Status; got != "watch" { // 8000/10000 = 80%
		t.Errorf("80%% debe ser 'watch', obtuve %q", got)
	}
	// Estado "ok" por debajo del 75%.
	if got := l.Budget(16000).Status; got != "ok" { // 8000/16000 = 50%
		t.Errorf("50%% debe ser 'ok', obtuve %q", got)
	}
	// Sin presupuesto (0): "unbudgeted", sin techo ni % pero con desglose.
	un := l.Budget(0)
	if un.Status != "unbudgeted" || un.Budget != 0 || len(un.Surfaces) != 3 {
		t.Errorf("budget 0 debe dar 'unbudgeted' con desglose, obtuve %+v", un)
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
