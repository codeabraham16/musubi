package memory

import "testing"

func TestConfigureDivisorsAffectsEstimate(t *testing.T) {
	defer ResetDivisors()
	base := EstimateTokens("abcdefgh") // prosa, 8 chars / 4 = 2
	if base != 2 {
		t.Fatalf("baseline esperado 2, obtuve %d", base)
	}
	ConfigureDivisors(8, 0, 0) // prosa a 8 chars/token -> 1
	if got := EstimateTokens("abcdefgh"); got != 1 {
		t.Errorf("tras calibrar prosa a 8, esperaba 1 token, obtuve %d", got)
	}
	ResetDivisors()
	if got := EstimateTokens("abcdefgh"); got != 2 {
		t.Errorf("tras reset esperaba 2, obtuve %d", got)
	}
}

func TestFitDivisorForSamples(t *testing.T) {
	// 8 chars no-CJK, 2 tokens reales -> divisor 4.0
	samples := []TokenSample{
		{Text: "abcdefgh", Kind: kindProse, Actual: 2},
		{Text: "abcdefgh", Kind: kindProse, Actual: 2},
	}
	if d := fitDivisorForSamples(samples); d < 3.9 || d > 4.1 {
		t.Errorf("esperaba divisor ~4.0, obtuve %v", d)
	}
}

func TestBuildCalibrationReport(t *testing.T) {
	counts := []TextCount{
		{Text: "una oración de prosa normal y corriente", Actual: 9},
		{Text: `{"a":1,"b":2}`, Actual: 9},
	}
	rep := BuildCalibrationReport(counts)
	if len(rep.PerKind) != 2 {
		t.Fatalf("esperaba 2 tipos en el reporte, obtuve %d", len(rep.PerKind))
	}
	for _, k := range rep.PerKind {
		if k.Samples == 0 || k.ActualTokens == 0 || k.SuggestedDivisor <= 0 {
			t.Errorf("entrada incompleta del reporte: %+v", k)
		}
	}
}

func TestSaveLoadDivisorsRoundtrip(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveDivisors(3.0, 2.0, 1.5); err != nil {
		t.Fatalf("SaveDivisors error: %v", err)
	}
	p, c, j, ok, err := e.LoadDivisors()
	if err != nil || !ok {
		t.Fatalf("LoadDivisors ok=%v err=%v", ok, err)
	}
	if p != 3.0 || c != 2.0 || j != 1.5 {
		t.Errorf("divisores mal persistidos: %v %v %v", p, c, j)
	}
}

func TestRecomputeTokensAppliesNewDivisors(t *testing.T) {
	defer ResetDivisors()
	e := newTestEngine(t)
	content := "abcdefghabcdefgh" // 16 chars prosa -> /4 = 4 tokens
	if err := e.SaveObservation("c1", "t", content, nil); err != nil {
		t.Fatal(err)
	}
	var before int
	e.db.QueryRow(`SELECT tokens FROM observations WHERE id=?`, "c1").Scan(&before)
	if before != 4 {
		t.Fatalf("tokens inicial esperado 4, obtuve %d", before)
	}

	// Calibrar prosa a 8 chars/token y recomputar -> 16/8 = 2.
	if err := e.SaveDivisors(8, 0, 0); err != nil {
		t.Fatal(err)
	}
	ConfigureDivisors(8, 0, 0)
	if err := e.RecomputeTokens(); err != nil {
		t.Fatalf("RecomputeTokens error: %v", err)
	}
	var after int
	e.db.QueryRow(`SELECT tokens FROM observations WHERE id=?`, "c1").Scan(&after)
	if after != 2 {
		t.Errorf("tras recomputar con divisor 8 esperaba 2 tokens, obtuve %d", after)
	}
}

func TestEngineOpenAppliesCalibratedDivisors(t *testing.T) {
	defer ResetDivisors()
	root := t.TempDir()
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SaveDivisors(5.0, 3.0, 2.0); err != nil {
		t.Fatal(err)
	}
	e.Close()

	ResetDivisors() // simular proceso nuevo con divisores por defecto
	e2, err := NewDbEngine(root)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()
	p, c, j := CurrentDivisors()
	if p != 5.0 || c != 3.0 || j != 2.0 {
		t.Errorf("al abrir, los divisores calibrados deben aplicarse: %v %v %v", p, c, j)
	}
}
