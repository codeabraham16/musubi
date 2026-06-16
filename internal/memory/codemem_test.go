package memory

import "testing"

func TestSaveAndGetCodeMemory(t *testing.T) {
	e := newTestEngine(t)
	cm := CodeMemory{
		Path:        "internal/foo/bar.go",
		Gist:        "Paquete bar: parsea config y expone Load().",
		Symbols:     "Load() L10; parse() L42",
		Fingerprint: "abc123",
		Tokens:      9,
	}
	if err := e.SaveCodeMemory(cm); err != nil {
		t.Fatalf("SaveCodeMemory error: %v", err)
	}
	got, ok, err := e.GetCodeMemory("internal/foo/bar.go")
	if err != nil || !ok {
		t.Fatalf("GetCodeMemory ok=%v err=%v", ok, err)
	}
	if got.Gist != cm.Gist || got.Symbols != cm.Symbols || got.Fingerprint != cm.Fingerprint || got.Tokens != 9 {
		t.Errorf("recuperado distinto de lo guardado: %+v", got)
	}
}

func TestSaveCodeMemoryUpserts(t *testing.T) {
	e := newTestEngine(t)
	_ = e.SaveCodeMemory(CodeMemory{Path: "a.go", Gist: "viejo", Fingerprint: "f1", Tokens: 1})
	if err := e.SaveCodeMemory(CodeMemory{Path: "a.go", Gist: "nuevo", Fingerprint: "f2", Tokens: 2}); err != nil {
		t.Fatalf("upsert error: %v", err)
	}
	got, _, _ := e.GetCodeMemory("a.go")
	if got.Gist != "nuevo" || got.Fingerprint != "f2" {
		t.Errorf("el upsert debe reemplazar el registro previo, obtuve %+v", got)
	}
}

func TestGetCodeMemoryMissing(t *testing.T) {
	e := newTestEngine(t)
	if _, ok, err := e.GetCodeMemory("no/existe.go"); err != nil || ok {
		t.Errorf("un path sin memoria debe dar ok=false, ok=%v err=%v", ok, err)
	}
}
