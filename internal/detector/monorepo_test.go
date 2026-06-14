package detector

import (
	"path/filepath"
	"testing"
)

// manifestPathPresente verifica que algún StackResult tenga el ManifestPath dado.
// La comparación normaliza separadores a "/" para ser robusta entre SO.
func manifestPathPresente(resultados []StackResult, ruta string) bool {
	objetivo := filepath.ToSlash(ruta)
	for _, r := range resultados {
		if filepath.ToSlash(r.ManifestPath) == objetivo {
			return true
		}
	}
	return false
}

// resultadoPorManifestPath devuelve el StackResult con el ManifestPath dado.
func resultadoPorManifestPath(resultados []StackResult, ruta string) (StackResult, bool) {
	objetivo := filepath.ToSlash(ruta)
	for _, r := range resultados {
		if filepath.ToSlash(r.ManifestPath) == objetivo {
			return r, true
		}
	}
	return StackResult{}, false
}

// TestDetectStackMonorepo verifica que un monorepo sin manifest en la raíz pero
// con manifests en subdirectorios produzca un StackResult por subdirectorio,
// con el ManifestPath relativo a la raíz.
func TestDetectStackMonorepo(t *testing.T) {
	root := t.TempDir()
	// Raíz SIN manifest. Subdirectorios con manifests.
	escribirArchivo(t, filepath.Join(root, "admin", "package.json"),
		`{"name":"admin","dependencies":{"react":"18.0.0"}}`)
	escribirArchivo(t, filepath.Join(root, "backend", "go.mod"),
		"module github.com/ejemplo/backend\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.0\n)\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if len(resultados) != 2 {
		t.Fatalf("esperaba 2 resultados (Node.js admin + Go backend), obtuve %d: %+v", len(resultados), resultados)
	}

	if !ecosistemaPresente(resultados, "Node.js") {
		t.Errorf("esperaba ecosistema 'Node.js' en monorepo, resultados: %+v", resultados)
	}
	if !ecosistemaPresente(resultados, "Go") {
		t.Errorf("esperaba ecosistema 'Go' en monorepo, resultados: %+v", resultados)
	}

	// Los ManifestPath deben ser relativos a la raíz.
	if !manifestPathPresente(resultados, "admin/package.json") {
		t.Errorf("esperaba ManifestPath 'admin/package.json', resultados: %+v", resultados)
	}
	if !manifestPathPresente(resultados, "backend/go.mod") {
		t.Errorf("esperaba ManifestPath 'backend/go.mod', resultados: %+v", resultados)
	}

	// ModuleName del backend Go debe parsearse correctamente.
	goRes, ok := resultadoPorManifestPath(resultados, "backend/go.mod")
	if !ok {
		t.Fatalf("no se encontró el resultado de backend/go.mod")
	}
	if goRes.ModuleName != "github.com/ejemplo/backend" {
		t.Errorf("ModuleName backend: esperaba %q, obtuve %q", "github.com/ejemplo/backend", goRes.ModuleName)
	}
}

// TestDetectStackMonorepoTresPaquetes verifica un monorepo estilo casino con
// admin/package.json, backend/go.mod y mobile/package.json (tres paquetes).
func TestDetectStackMonorepoTresPaquetes(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "admin", "package.json"),
		`{"name":"admin","dependencies":{"next":"13.0.0"}}`)
	escribirArchivo(t, filepath.Join(root, "backend", "go.mod"),
		"module github.com/casino/backend\n\ngo 1.21\n")
	escribirArchivo(t, filepath.Join(root, "mobile", "package.json"),
		`{"name":"mobile","dependencies":{"react":"18.0.0"}}`)

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if len(resultados) != 3 {
		t.Fatalf("esperaba 3 resultados, obtuve %d: %+v", len(resultados), resultados)
	}
	for _, ruta := range []string{"admin/package.json", "backend/go.mod", "mobile/package.json"} {
		if !manifestPathPresente(resultados, ruta) {
			t.Errorf("esperaba ManifestPath %q, resultados: %+v", ruta, resultados)
		}
	}
}

// TestDetectStackRootManifestSiguePresente verifica que un proyecto de un solo
// paquete en la raíz conserve el comportamiento original: ManifestPath == "go.mod".
func TestDetectStackRootManifestSiguePresente(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "go.mod"),
		"module github.com/ejemplo/app\n\ngo 1.21\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if len(resultados) != 1 {
		t.Fatalf("esperaba 1 resultado para paquete único en raíz, obtuve %d: %+v", len(resultados), resultados)
	}
	goRes, _ := resultadoPorEcosistema(resultados, "Go")
	if goRes.ManifestPath != "go.mod" {
		t.Errorf("ManifestPath raíz: esperaba %q (sin prefijo de subdir), obtuve %q", "go.mod", goRes.ManifestPath)
	}
}

// TestDetectStackIgnoraNodeModules verifica que un package.json dentro de
// node_modules/ sea ignorado por el walk recursivo.
func TestDetectStackIgnoraNodeModules(t *testing.T) {
	root := t.TempDir()
	// Un paquete real en app/, y un package.json "ruidoso" dentro de node_modules.
	escribirArchivo(t, filepath.Join(root, "app", "package.json"),
		`{"name":"app","dependencies":{"react":"18.0.0"}}`)
	escribirArchivo(t, filepath.Join(root, "node_modules", "left-pad", "package.json"),
		`{"name":"left-pad","dependencies":{"vue":"3.0.0"}}`)
	escribirArchivo(t, filepath.Join(root, "app", "node_modules", "react", "package.json"),
		`{"name":"react"}`)

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	// Solo debe detectarse app/package.json — los de node_modules se ignoran.
	if len(resultados) != 1 {
		t.Fatalf("esperaba 1 resultado (app/package.json), obtuve %d: %+v", len(resultados), resultados)
	}
	if !manifestPathPresente(resultados, "app/package.json") {
		t.Errorf("esperaba ManifestPath 'app/package.json', resultados: %+v", resultados)
	}
	if manifestPathPresente(resultados, "node_modules/left-pad/package.json") {
		t.Errorf("no se esperaba detectar manifests dentro de node_modules, resultados: %+v", resultados)
	}
}

// TestDetectStackIgnoraDirectoriosExcluidos verifica que vendor, dist, build,
// target, .git, __pycache__ y .musubi sean omitidos.
func TestDetectStackIgnoraDirectoriosExcluidos(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "src", "go.mod"),
		"module github.com/ejemplo/src\n\ngo 1.21\n")
	for _, excl := range []string{"vendor", "dist", "build", "target", ".git", "__pycache__", ".musubi"} {
		escribirArchivo(t, filepath.Join(root, excl, "package.json"),
			`{"name":"ruido","dependencies":{"react":"18.0.0"}}`)
	}

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if len(resultados) != 1 {
		t.Fatalf("esperaba 1 resultado (src/go.mod), obtuve %d: %+v", len(resultados), resultados)
	}
	if !manifestPathPresente(resultados, "src/go.mod") {
		t.Errorf("esperaba ManifestPath 'src/go.mod', resultados: %+v", resultados)
	}
}

// TestDetectStackProfundidadAcotada verifica que un manifest más allá de maxDepth
// (4 niveles relativos a la raíz) NO se detecte.
func TestDetectStackProfundidadAcotada(t *testing.T) {
	root := t.TempDir()
	// Profundidad 5 (a/b/c/d/e/go.mod) — más allá de maxDepth=4, debe ignorarse.
	muyProfundo := filepath.Join(root, "a", "b", "c", "d", "e", "go.mod")
	escribirArchivo(t, muyProfundo, "module github.com/ejemplo/profundo\n\ngo 1.21\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if len(resultados) != 0 {
		t.Fatalf("esperaba 0 resultados (manifest fuera de maxDepth), obtuve %d: %+v", len(resultados), resultados)
	}
}

// TestDetectStackProfundidadDentroDelLimite verifica que un manifest dentro de
// maxDepth sí se detecte.
func TestDetectStackProfundidadDentroDelLimite(t *testing.T) {
	root := t.TempDir()
	// Profundidad 3 (a/b/c/go.mod) — dentro de maxDepth=4.
	dentro := filepath.Join(root, "a", "b", "c", "go.mod")
	escribirArchivo(t, dentro, "module github.com/ejemplo/dentro\n\ngo 1.21\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !manifestPathPresente(resultados, "a/b/c/go.mod") {
		t.Errorf("esperaba ManifestPath 'a/b/c/go.mod' dentro del límite, resultados: %+v", resultados)
	}
}

// --- ExtractDeps recursivo ---

// TestExtractDepsMonorepo verifica que ExtractDeps agregue (union) las deps de
// todos los manifests en subdirectorios, por ecosistema.
func TestExtractDepsMonorepo(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "admin", "package.json"),
		`{"name":"admin","dependencies":{"react":"18.0.0","express":"4.18.0"}}`)
	escribirArchivo(t, filepath.Join(root, "mobile", "package.json"),
		`{"name":"mobile","dependencies":{"react-native":"0.72.0"}}`)
	escribirArchivo(t, filepath.Join(root, "backend", "go.mod"),
		"module github.com/ejemplo/backend\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.0\n)\n")

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	nodeDeps, ok := deps["Node.js"]
	if !ok {
		t.Fatalf("esperaba clave 'Node.js' en monorepo, deps: %v", deps)
	}
	for _, esperada := range []string{"react", "express", "react-native"} {
		if !contiene(nodeDeps, esperada) {
			t.Errorf("esperaba %q en union de deps Node.js, obtuve: %v", esperada, nodeDeps)
		}
	}

	goDeps, ok := deps["Go"]
	if !ok {
		t.Fatalf("esperaba clave 'Go' en monorepo, deps: %v", deps)
	}
	if !contiene(goDeps, "github.com/gin-gonic/gin") {
		t.Errorf("esperaba 'github.com/gin-gonic/gin' en deps Go, obtuve: %v", goDeps)
	}
}

// TestExtractDepsMonorepoDedupe verifica que una dep presente en dos manifests
// del mismo ecosistema aparezca una sola vez (union deduplicada).
func TestExtractDepsMonorepoDedupe(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "admin", "package.json"),
		`{"name":"admin","dependencies":{"react":"18.0.0"}}`)
	escribirArchivo(t, filepath.Join(root, "mobile", "package.json"),
		`{"name":"mobile","dependencies":{"react":"18.2.0"}}`)

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	nodeDeps := deps["Node.js"]
	count := 0
	for _, d := range nodeDeps {
		if d == "react" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("esperaba 'react' exactamente una vez (dedupe), apareció %d veces: %v", count, nodeDeps)
	}
}

// TestExtractDepsIgnoraNodeModules verifica que ExtractDeps no agregue deps de
// manifests dentro de node_modules.
func TestExtractDepsIgnoraNodeModules(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "app", "package.json"),
		`{"name":"app","dependencies":{"react":"18.0.0"}}`)
	escribirArchivo(t, filepath.Join(root, "node_modules", "left-pad", "package.json"),
		`{"name":"left-pad","dependencies":{"vue":"3.0.0"}}`)

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	nodeDeps := deps["Node.js"]
	if !contiene(nodeDeps, "react") {
		t.Errorf("esperaba 'react' (app), obtuve: %v", nodeDeps)
	}
	if contiene(nodeDeps, "vue") {
		t.Errorf("no se esperaba 'vue' (dentro de node_modules), obtuve: %v", nodeDeps)
	}
}

// TestExtractDepsRootSiguePresente verifica que el comportamiento de un solo
// paquete en la raíz se conserve idéntico.
func TestExtractDepsRootSiguePresente(t *testing.T) {
	root := t.TempDir()
	escribirGoMod(t, root, []string{"github.com/gin-gonic/gin", "github.com/stretchr/testify"})

	deps, err := ExtractDeps(root)
	if err != nil {
		t.Fatalf("ExtractDeps error inesperado: %v", err)
	}

	goDeps := deps["Go"]
	for _, esperada := range []string{"github.com/gin-gonic/gin", "github.com/stretchr/testify"} {
		if !contiene(goDeps, esperada) {
			t.Errorf("esperaba %q en deps Go raíz, obtuve: %v", esperada, goDeps)
		}
	}
}
