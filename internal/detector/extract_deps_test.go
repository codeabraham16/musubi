package detector

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Helpers de fixture ---

// escribirGoMod crea un go.mod mínimo con los requires dados.
func escribirGoMod(t *testing.T, root string, modulos []string) {
	t.Helper()
	contenido := "module github.com/ejemplo/app\n\ngo 1.21\n\nrequire (\n"
	for _, m := range modulos {
		contenido += "\t" + m + " v1.0.0\n"
	}
	contenido += ")\n"
	escribirArchivo(t, filepath.Join(root, "go.mod"), contenido)
}

// --- Tests de ExtractDeps por ecosistema ---

// TestExtractDepsGo verifica que go.mod se parsea y devuelve las dependencias de Go.
func TestExtractDepsGo(t *testing.T) {
	root := t.TempDir()
	escribirGoMod(t, root, []string{
		"github.com/gin-gonic/gin",
		"github.com/stretchr/testify",
	})

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	goDeps, ok := deps["Go"]
	if !ok {
		t.Fatalf("esperaba clave 'Go' en resultado, obtuve: %v", deps)
	}

	if !contiene(goDeps, "github.com/gin-gonic/gin") {
		t.Errorf("esperaba 'github.com/gin-gonic/gin' en deps Go, obtuve: %v", goDeps)
	}
	if !contiene(goDeps, "github.com/stretchr/testify") {
		t.Errorf("esperaba 'github.com/stretchr/testify' en deps Go, obtuve: %v", goDeps)
	}
}

// TestExtractDepsNode verifica que package.json devuelve deps y devDeps de Node.js.
func TestExtractDepsNode(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "package.json"),
		`{"name":"mi-app","dependencies":{"react":"18.0.0","express":"4.18.0"},"devDependencies":{"vite":"5.0.0","vitest":"1.0.0"}}`)

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	nodeDeps, ok := deps["Node.js"]
	if !ok {
		t.Fatalf("esperaba clave 'Node.js' en resultado, obtuve: %v", deps)
	}

	for _, esperada := range []string{"react", "express", "vite", "vitest"} {
		if !contiene(nodeDeps, esperada) {
			t.Errorf("esperaba %q en deps Node.js, obtuve: %v", esperada, nodeDeps)
		}
	}
}

// TestExtractDepsPythonRequirements verifica que requirements.txt se parsea
// y los especificadores de versión se eliminan correctamente.
func TestExtractDepsPythonRequirements(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "requirements.txt"),
		"fastapi>=0.110.0\ndjango==4.2.0\nrequests~=2.28\npydantic[extras]>=2.0\npytest\n")

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	pyDeps, ok := deps["Python"]
	if !ok {
		t.Fatalf("esperaba clave 'Python' en resultado, obtuve: %v", deps)
	}

	for _, esperada := range []string{"fastapi", "django", "requests", "pydantic", "pytest"} {
		if !contiene(pyDeps, esperada) {
			t.Errorf("esperaba %q en deps Python, obtuve: %v", esperada, pyDeps)
		}
	}
	// Verificar que no haya especificadores de versión
	for _, dep := range pyDeps {
		for _, spec := range []string{">=", "==", "~=", "[", ">", "<"} {
			if containsStr(dep, spec) {
				t.Errorf("dep Python %q no debería contener especificador de versión %q", dep, spec)
			}
		}
	}
}

// TestExtractDepsRust verifica que Cargo.toml devuelva las dependencias de Rust.
func TestExtractDepsRust(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "Cargo.toml"),
		"[package]\nname = \"mi-crate\"\nversion = \"0.1.0\"\n\n[dependencies]\naxum = \"0.7\"\ntokio = { version = \"1\", features = [\"full\"] }\nserde = \"1\"\n")

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	rustDeps, ok := deps["Rust"]
	if !ok {
		t.Fatalf("esperaba clave 'Rust' en resultado, obtuve: %v", deps)
	}

	for _, esperada := range []string{"axum", "tokio", "serde"} {
		if !contiene(rustDeps, esperada) {
			t.Errorf("esperaba %q en deps Rust, obtuve: %v", esperada, rustDeps)
		}
	}
}

// TestExtractDepsManifestAusenteNoEsFatal verifica que la ausencia de un manifest
// no produce error — ese ecosistema simplemente no aparece o está vacío.
func TestExtractDepsManifestAusenteNoEsFatal(t *testing.T) {
	root := t.TempDir()
	// Solo hay go.mod, no hay package.json
	escribirGoMod(t, root, []string{"github.com/gin-gonic/gin"})

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado con manifest ausente: %v", err)
	}

	// Go debe estar presente
	if _, ok := deps["Go"]; !ok {
		t.Error("esperaba clave 'Go' cuando go.mod existe")
	}
	// Node.js no debe estar presente o debe tener slice vacío (no error)
	if nodeDeps, ok := deps["Node.js"]; ok && len(nodeDeps) > 0 {
		t.Errorf("Node.js no debería tener deps cuando package.json está ausente, obtuve: %v", nodeDeps)
	}
}

// TestExtractDepsManifestMalformadoNoEsFatal verifica que un package.json inválido
// no produce error — se degrada con logx.Warn.
func TestExtractDepsManifestMalformadoNoEsFatal(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "package.json"), "{esto no es json valido {{{{")

	var deps map[string][]string
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic inesperado con package.json malformado: %v", r)
			}
		}()
		deps, err = ExtractDeps(root)
	}()

	if err != nil {
		t.Fatalf("ExtractDeps error inesperado con manifest malformado: %v", err)
	}
	// Node.js puede estar ausente o tener slice vacío — lo importante es no error/panic
	if nodeDeps, ok := deps["Node.js"]; ok {
		if len(nodeDeps) > 0 {
			t.Errorf("Node.js malformado debería tener deps vacío, obtuve: %v", nodeDeps)
		}
	}
}

// TestExtractDepsCacheHitEnLlamadasRepetidas verifica que una segunda llamada
// con manifests sin cambios devuelve el mismo resultado (cache hit).
func TestExtractDepsCacheHitEnLlamadasRepetidas(t *testing.T) {
	root := t.TempDir()
	escribirGoMod(t, root, []string{"github.com/gin-gonic/gin"})

	// Primera llamada
	deps1, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("primera llamada error: %v", err)
	}

	// Segunda llamada sin modificar archivos
	deps2, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("segunda llamada error: %v", err)
	}

	// Ambas deben contener las mismas deps
	goDeps1 := deps1["Go"]
	goDeps2 := deps2["Go"]

	if len(goDeps1) != len(goDeps2) {
		t.Errorf("cache hit: esperaba mismos resultados, primera=%v segunda=%v", goDeps1, goDeps2)
	}
}

// TestExtractDepsCacheMissConManifestModificado verifica que modificar go.mod
// invalida el caché y retorna las deps actualizadas.
func TestExtractDepsCacheMissConManifestModificado(t *testing.T) {
	root := t.TempDir()
	// Invalidar el caché global antes del test para evitar interferencia entre tests
	depsCache.Range(func(key, _ any) bool {
		depsCache.Delete(key)
		return true
	})

	goModPath := filepath.Join(root, "go.mod")
	escribirArchivo(t, goModPath, "module github.com/ejemplo/app\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.0.0\n)\n")

	// Primera llamada: cachea gin
	deps1, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("primera llamada error: %v", err)
	}
	if !contiene(deps1["Go"], "github.com/gin-gonic/gin") {
		t.Fatalf("esperaba gin en primera llamada, obtuve: %v", deps1["Go"])
	}

	// Esperar un momento para que mtime cambie
	time.Sleep(10 * time.Millisecond)

	// Modificar go.mod: reemplazar gin por fiber
	escribirArchivo(t, goModPath, "module github.com/ejemplo/app\n\ngo 1.21\n\nrequire (\n\tgithub.com/gofiber/fiber v2.0.0\n)\n")

	// Forzar cambio de mtime explícito en Windows (puede ser que la granularidad sea baja)
	ahora := time.Now().Add(time.Second)
	if err := os.Chtimes(goModPath, ahora, ahora); err != nil {
		t.Logf("advertencia: no se pudo cambiar mtime: %v", err)
	}

	// Segunda llamada: debería detectar el cambio de mtime y re-parsear
	deps2, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("segunda llamada error: %v", err)
	}

	goDeps2 := deps2["Go"]
	if contiene(goDeps2, "github.com/gin-gonic/gin") {
		t.Error("no esperaba gin después de modificar go.mod (debería ser cache miss)")
	}
	if !contiene(goDeps2, "github.com/gofiber/fiber") {
		t.Errorf("esperaba fiber en segunda llamada, obtuve: %v", goDeps2)
	}
}

// --- Helpers ---

// contiene verifica si un slice de strings contiene el elemento dado.
func contiene(slice []string, elem string) bool {
	for _, s := range slice {
		if s == elem {
			return true
		}
	}
	return false
}

// containsStr verifica si s contiene substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}
