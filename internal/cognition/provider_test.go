package cognition

import (
	"testing"

	"musubi/internal/config"
)

// TestNoopIsDefaultAndDisabled: el pilar nace apagado. NoopProvider no está "enabled" y su Name
// es "none" (ninguna arista se atribuye a un LLM mientras el pilar esté apagado).
func TestNoopIsDefaultAndDisabled(t *testing.T) {
	if Enabled(NoopProvider{}) {
		t.Error("NoopProvider no debería contar como cognición habilitada")
	}
	if got := (NoopProvider{}).Name(); got != "none" {
		t.Errorf("NoopProvider.Name()=%q, esperaba \"none\"", got)
	}
}

// TestFactoryDefaultsToNoop: config vacía o "none" ⇒ Noop; un provider desconocido ⇒ error
// explícito (fail-closed, no Noop silencioso).
func TestFactoryDefaultsToNoop(t *testing.T) {
	for _, prov := range []string{"", "none"} {
		p, err := NewProvider(config.CognitionConfig{Provider: prov})
		if err != nil {
			t.Fatalf("NewProvider(%q): error inesperado %v", prov, err)
		}
		if Enabled(p) {
			t.Errorf("NewProvider(%q) debería estar deshabilitado", prov)
		}
	}

	if _, err := NewProvider(config.CognitionConfig{Provider: "claude-fantasia"}); err == nil {
		t.Error("un motor desconocido debería devolver error (fail-closed), no Noop")
	}
}
