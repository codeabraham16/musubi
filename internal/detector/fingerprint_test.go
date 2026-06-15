package detector

import (
	"strings"
	"testing"
)

func TestStackFingerprintEstableYOrdenado(t *testing.T) {
	a := []StackResult{
		{Ecosystem: "Node.js", Frameworks: []string{"react"}},
		{Ecosystem: "Go"},
	}
	b := []StackResult{
		{Ecosystem: "Go"},
		{Ecosystem: "Node.js", Frameworks: []string{"react"}},
	}
	fa := StackFingerprint(a)
	fb := StackFingerprint(b)
	if fa == "" {
		t.Fatal("la huella no debería ser vacía para un stack con resultados")
	}
	if fa != fb {
		t.Errorf("la huella debe ser independiente del orden: %q != %q", fa, fb)
	}
	// La huella debe contener tanto el ecosistema como el framework calificado.
	if !strings.Contains(fa, "Go") || !strings.Contains(fa, "Node.js:react") {
		t.Errorf("la huella debe incluir ecosistema y framework calificado, obtuve %q", fa)
	}
}

func TestStackFingerprintVacio(t *testing.T) {
	if fp := StackFingerprint(nil); fp != "" {
		t.Errorf("stack vacío debe dar huella vacía, obtuve %q", fp)
	}
	if fp := StackFingerprint([]StackResult{}); fp != "" {
		t.Errorf("stack vacío debe dar huella vacía, obtuve %q", fp)
	}
}

func TestStackTokensDeduplica(t *testing.T) {
	// Monorepo con dos módulos Go: el token "Go" debe aparecer una sola vez.
	results := []StackResult{
		{Ecosystem: "Go", ManifestPath: "backend/go.mod"},
		{Ecosystem: "Go", ManifestPath: "tools/go.mod"},
	}
	tokens := StackTokens(results)
	count := 0
	for _, tk := range tokens {
		if tk == "Go" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("el token 'Go' debe estar deduplicado, apareció %d veces en %v", count, tokens)
	}
}

func TestStackDeltaDetectaLoNuevo(t *testing.T) {
	stored := StackFingerprint([]StackResult{{Ecosystem: "Go"}})
	current := []StackResult{
		{Ecosystem: "Go"},
		{Ecosystem: "Node.js", Frameworks: []string{"react"}},
	}
	delta := StackDelta(stored, current)
	if len(delta) == 0 {
		t.Fatal("se esperaba un delta no vacío al agregar Node.js")
	}
	hasNode := false
	for _, tk := range delta {
		if tk == "Node.js" {
			hasNode = true
		}
		if tk == "Go" {
			t.Errorf("Go ya estaba en la huella guardada, no debería estar en el delta: %v", delta)
		}
	}
	if !hasNode {
		t.Errorf("el delta debe incluir el ecosistema nuevo Node.js, obtuve %v", delta)
	}
}

func TestStackDeltaVacioSinCambios(t *testing.T) {
	results := []StackResult{
		{Ecosystem: "Go"},
		{Ecosystem: "Node.js", Frameworks: []string{"react"}},
	}
	stored := StackFingerprint(results)
	if delta := StackDelta(stored, results); len(delta) != 0 {
		t.Errorf("sin cambios de stack el delta debe ser vacío, obtuve %v", delta)
	}
}
