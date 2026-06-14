package skillsource

import (
	"path/filepath"
	"testing"

	"musubi/internal/detector"
)

// TestIsApplicableTodasLasCondicionesOK verifica que una entrada que cumple los 4
// pasos del gate es marcada como aplicable, con evidencia completa.
func TestIsApplicableTodasLasCondicionesOK(t *testing.T) {
	root := t.TempDir()
	// Crear archivo .go para el trigger
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	entrada := CatalogEntry{
		ID:       "go-gin",
		Stacks:   []string{"Go"},
		Deps:     []string{"github.com/gin-gonic/gin"},
		Triggers: []string{"*.go"},
		// "go" debería estar en PATH en cualquier entorno de desarrollo Go
		Capabilities: []string{"go"},
		RulesURL:     "https://example.com/go-gin.md",
	}

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {"github.com/gin-gonic/gin"}}

	ok, ev := IsApplicable(entrada, root, deps, stacks)
	if !ok {
		t.Fatalf("esperaba IsApplicable=true, obtuve false; evidencia: %+v", ev)
	}
	if ev.MatchedStack != "Go" {
		t.Errorf("MatchedStack: esperaba 'Go', obtuve %q", ev.MatchedStack)
	}
	if len(ev.MatchedDeps) == 0 {
		t.Error("esperaba MatchedDeps no vacío")
	}
	if ev.MatchedFileCount < 1 {
		t.Errorf("MatchedFileCount: esperaba ≥1, obtuve %d", ev.MatchedFileCount)
	}
}

// TestIsApplicableStackMismatch verifica que un ecosistema diferente al detectado
// hace fallar el gate en el paso 1.
func TestIsApplicableStackMismatch(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "main.rs"), "fn main() {}")

	entrada := CatalogEntry{
		ID:       "rust-axum",
		Stacks:   []string{"Rust"},
		Triggers: []string{"*.rs"},
		RulesURL: "https://example.com/rust-axum.md",
	}

	// Proyecto solo con Go stack
	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}}

	ok, _ := IsApplicable(entrada, root, deps, stacks)
	if ok {
		t.Error("esperaba IsApplicable=false por stack mismatch, obtuve true")
	}
}

// TestIsApplicableDepNoPresente verifica que una dep requerida ausente hace fallar el paso 2.
func TestIsApplicableDepNoPresente(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	entrada := CatalogEntry{
		ID:       "go-gin",
		Stacks:   []string{"Go"},
		Deps:     []string{"github.com/gin-gonic/gin"},
		Triggers: []string{"*.go"},
		RulesURL: "https://example.com/go-gin.md",
	}

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	// Deps de Go no incluyen gin
	deps := map[string][]string{"Go": {"github.com/some-other/dep"}}

	ok, _ := IsApplicable(entrada, root, deps, stacks)
	if ok {
		t.Error("esperaba IsApplicable=false por dep ausente, obtuve true")
	}
}

// TestIsApplicableDepsVaciasOmiteCheckDep verifica que una entrada con Deps vacíos
// no requiere deps del proyecto (skill a nivel de ecosistema).
func TestIsApplicableDepsVaciasOmiteCheckDep(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	entrada := CatalogEntry{
		ID:       "go-ecosystem",
		Stacks:   []string{"Go"},
		Deps:     []string{}, // vacío: ecosistema-nivel, no requiere dep específica
		Triggers: []string{"*.go"},
		RulesURL: "https://example.com/go-eco.md",
	}

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}} // sin deps en el proyecto

	ok, ev := IsApplicable(entrada, root, deps, stacks)
	if !ok {
		t.Errorf("esperaba IsApplicable=true (deps vacías = ecosistema-nivel), evidencia: %+v", ev)
	}
}

// TestIsApplicableNoFileMatch verifica que cuando ningún trigger glob encuentra
// archivos en el proyecto, el gate falla en el paso 3.
func TestIsApplicableNoFileMatch(t *testing.T) {
	root := t.TempDir()
	// Solo hay archivos .go, no .rs
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	entrada := CatalogEntry{
		ID:       "rust-axum",
		Stacks:   []string{"Go"}, // Stack coincide
		Deps:     []string{},     // sin dep requerida
		Triggers: []string{"*.rs"},
		RulesURL: "https://example.com/rust-axum.md",
	}

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}}

	ok, _ := IsApplicable(entrada, root, deps, stacks)
	if ok {
		t.Error("esperaba IsApplicable=false por ausencia de archivos trigger, obtuve true")
	}
}

// TestIsApplicableCapabilityAusente verifica que una capability no instalada
// hace fallar el paso 4 y aparece en MissingCaps.
func TestIsApplicableCapabilityAusente(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	entrada := CatalogEntry{
		ID:           "go-with-nonexistent-tool",
		Stacks:       []string{"Go"},
		Deps:         []string{},
		Triggers:     []string{"*.go"},
		Capabilities: []string{"herramienta-que-no-existe-en-path-12345"},
		RulesURL:     "https://example.com/x.md",
	}

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}}

	ok, ev := IsApplicable(entrada, root, deps, stacks)
	if ok {
		t.Error("esperaba IsApplicable=false por capability ausente, obtuve true")
	}
	if len(ev.MissingCaps) == 0 {
		t.Error("esperaba MissingCaps no vacío cuando la capability no existe")
	}
}

// TestIsApplicableWalkOmiteDirectoriosRuidosos verifica que el bounded walk
// no visita node_modules, .git, vendor ni archivos a profundidad > depthCap.
func TestIsApplicableWalkOmiteDirectoriosRuidosos(t *testing.T) {
	root := t.TempDir()

	// Archivo válido en raíz
	escribirArchivo(t, filepath.Join(root, "main.go"), "package main")

	// Archivos en directorios que deben omitirse
	escribirArchivo(t, filepath.Join(root, "node_modules", "lib", "index.go"), "package lib")
	escribirArchivo(t, filepath.Join(root, ".git", "hooks", "pre-commit.go"), "package hooks")
	escribirArchivo(t, filepath.Join(root, "vendor", "pkg", "pkg.go"), "package pkg")

	// Archivo a profundidad 5 (> depthCap=4): debe omitirse
	escribirArchivo(t, filepath.Join(root, "a", "b", "c", "d", "e", "deep.go"), "package deep")

	entrada := CatalogEntry{
		ID:       "go-test",
		Stacks:   []string{"Go"},
		Deps:     []string{},
		Triggers: []string{"*.go"},
		RulesURL: "https://example.com/x.md",
	}

	stacks := []detector.StackResult{{Ecosystem: "Go"}}
	deps := map[string][]string{"Go": {}}

	// El gate debe ser true (main.go en raíz coincide) aunque haya archivos en
	// directorios ruidosos — esos directorios se omiten, no causan fallo.
	ok, ev := IsApplicable(entrada, root, deps, stacks)
	if !ok {
		t.Errorf("esperaba IsApplicable=true (main.go en raíz coincide), evidencia: %+v", ev)
	}
}
