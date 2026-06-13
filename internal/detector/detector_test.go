package detector

import (
	"os"
	"path/filepath"
	"testing"
)

// escribirArchivo es un helper que escribe un archivo en la ruta dada, creando
// los directorios intermedios si es necesario.
func escribirArchivo(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(ruta), err)
	}
	if err := os.WriteFile(ruta, []byte(contenido), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", ruta, err)
	}
}

// ecosistemaPresente verifica que la lista de resultados contenga al menos un
// resultado con el ecosistema dado.
func ecosistemaPresente(resultados []StackResult, ecosistema string) bool {
	for _, r := range resultados {
		if r.Ecosystem == ecosistema {
			return true
		}
	}
	return false
}

// frameworkPresente verifica que algún resultado tenga el framework dado.
func frameworkPresente(resultados []StackResult, framework string) bool {
	for _, r := range resultados {
		for _, f := range r.Frameworks {
			if f == framework {
				return true
			}
		}
	}
	return false
}

// resultadoPorEcosistema devuelve el primer StackResult con el ecosistema dado.
func resultadoPorEcosistema(resultados []StackResult, ecosistema string) (StackResult, bool) {
	for _, r := range resultados {
		if r.Ecosystem == ecosistema {
			return r, true
		}
	}
	return StackResult{}, false
}

// --- Tests principales ---

// TestDetectarProyectoGo verifica la detección de un proyecto Go con framework gin.
func TestDetectarProyectoGo(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "go.mod"),
		"module github.com/ejemplo/app\n\ngo 1.21\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.0\n)\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Go") {
		t.Fatalf("esperaba ecosistema 'Go', resultados: %+v", resultados)
	}

	goResult, _ := resultadoPorEcosistema(resultados, "Go")
	if goResult.ManifestPath != "go.mod" {
		t.Errorf("ManifestPath: esperaba %q, obtuve %q", "go.mod", goResult.ManifestPath)
	}
	if goResult.ModuleName != "github.com/ejemplo/app" {
		t.Errorf("ModuleName: esperaba %q, obtuve %q", "github.com/ejemplo/app", goResult.ModuleName)
	}
	if !frameworkPresente(resultados, "gin") {
		t.Errorf("esperaba framework 'gin', frameworks en Go: %v", goResult.Frameworks)
	}
}

// TestDetectarProyectoNode verifica la detección de Node.js con Next.js.
func TestDetectarProyectoNode(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "package.json"),
		`{"name":"mi-app","dependencies":{"next":"13.0.0","react":"18.0.0"}}`)

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Node.js") {
		t.Fatalf("esperaba ecosistema 'Node.js', resultados: %+v", resultados)
	}

	nodeResult, _ := resultadoPorEcosistema(resultados, "Node.js")
	if nodeResult.ModuleName != "mi-app" {
		t.Errorf("ModuleName: esperaba %q, obtuve %q", "mi-app", nodeResult.ModuleName)
	}
	if !frameworkPresente(resultados, "Next.js") {
		t.Errorf("esperaba framework 'Next.js', frameworks: %v", nodeResult.Frameworks)
	}
}

// TestDetectarProyectoRust verifica la detección de Rust con actix-web.
func TestDetectarProyectoRust(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "Cargo.toml"),
		"[package]\nname = \"mi-crate\"\nversion = \"0.1.0\"\n\n[dependencies]\nactix-web = \"4\"\ntokio = { version = \"1\", features = [\"full\"] }\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Rust") {
		t.Fatalf("esperaba ecosistema 'Rust', resultados: %+v", resultados)
	}

	rustResult, _ := resultadoPorEcosistema(resultados, "Rust")
	if rustResult.ModuleName != "mi-crate" {
		t.Errorf("ModuleName: esperaba %q, obtuve %q", "mi-crate", rustResult.ModuleName)
	}
	if !frameworkPresente(resultados, "actix") {
		t.Errorf("esperaba framework 'actix', frameworks: %v", rustResult.Frameworks)
	}
}

// TestDetectarProyectoPython verifica la detección de Python con django.
func TestDetectarProyectoPython(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "requirements.txt"),
		"django>=4.0\nrequests==2.28.0\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Python") {
		t.Fatalf("esperaba ecosistema 'Python', resultados: %+v", resultados)
	}
	if !frameworkPresente(resultados, "django") {
		t.Errorf("esperaba framework 'django', resultados: %+v", resultados)
	}
}

// TestDetectarProyectoJava verifica la detección de Java/Kotlin con pom.xml.
func TestDetectarProyectoJava(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "pom.xml"),
		"<project><dependencies><dependency><groupId>org.springframework.boot</groupId></dependency></dependencies></project>")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Java") {
		t.Fatalf("esperaba ecosistema 'Java', resultados: %+v", resultados)
	}
}

// TestDetectarProyectoRuby verifica la detección de Ruby con rails.
func TestDetectarProyectoRuby(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "Gemfile"),
		"source 'https://rubygems.org'\ngem 'rails', '~> 7.0'\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Ruby") {
		t.Fatalf("esperaba ecosistema 'Ruby', resultados: %+v", resultados)
	}
	if !frameworkPresente(resultados, "rails") {
		t.Errorf("esperaba framework 'rails', resultados: %+v", resultados)
	}
}

// TestDetectarProyectoPHP verifica la detección de PHP con laravel.
func TestDetectarProyectoPHP(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "composer.json"),
		`{"require":{"laravel/framework":"^10.0","php":"^8.1"}}`)

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "PHP") {
		t.Fatalf("esperaba ecosistema 'PHP', resultados: %+v", resultados)
	}
	if !frameworkPresente(resultados, "laravel") {
		t.Errorf("esperaba framework 'laravel', resultados: %+v", resultados)
	}
}

// TestDetectarProyectoDotNet verifica la detección de .NET con un archivo .csproj.
func TestDetectarProyectoDotNet(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "MiApp.csproj"),
		"<Project Sdk=\"Microsoft.NET.Sdk\"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, ".NET") {
		t.Fatalf("esperaba ecosistema '.NET', resultados: %+v", resultados)
	}
}

// TestDetectarDocker verifica que Dockerfile genere un resultado Docker.
func TestDetectarDocker(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "Dockerfile"),
		"FROM golang:1.21\nWORKDIR /app\nCOPY . .\nRUN go build -o main .\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if !ecosistemaPresente(resultados, "Docker") {
		t.Fatalf("esperaba ecosistema 'Docker', resultados: %+v", resultados)
	}
}

// TestDetectarProyectoPoliglota verifica que un proyecto con go.mod + Dockerfile
// produzca al menos dos resultados (Go y Docker).
func TestDetectarProyectoPoliglota(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "go.mod"),
		"module github.com/ejemplo/poliglota\n\ngo 1.21\n")
	escribirArchivo(t, filepath.Join(root, "Dockerfile"),
		"FROM golang:1.21\nWORKDIR /app\n")

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("DetectStack error inesperado: %v", err)
	}

	if len(resultados) < 2 {
		t.Fatalf("esperaba al menos 2 resultados (Go + Docker), obtuve %d: %+v", len(resultados), resultados)
	}
	if !ecosistemaPresente(resultados, "Go") {
		t.Error("esperaba ecosistema 'Go' en proyecto poliglota")
	}
	if !ecosistemaPresente(resultados, "Docker") {
		t.Error("esperaba ecosistema 'Docker' en proyecto poliglota")
	}
}

// TestDirectorioVacioDevuelveSliceVacio verifica que un directorio sin manifests
// devuelva un slice vacío (no nil) y sin error.
func TestDirectorioVacioDevuelveSliceVacio(t *testing.T) {
	root := t.TempDir()

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("error inesperado en directorio vacío: %v", err)
	}
	if resultados == nil {
		t.Fatal("esperaba slice vacío no-nil, obtuve nil")
	}
	if len(resultados) != 0 {
		t.Fatalf("esperaba 0 resultados, obtuve %d: %+v", len(resultados), resultados)
	}
}

// TestPackageJsonMalformado verifica que un package.json inválido no produzca
// pánico ni error fatal — se degrada a un resultado mínimo sin frameworks.
func TestPackageJsonMalformado(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, filepath.Join(root, "package.json"),
		"{esto no es json valido {{{{")

	// No debe hacer panic ni retornar error
	var resultados []StackResult
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic inesperado con package.json malformado: %v", r)
			}
		}()
		resultados, err = DetectStack(root)
	}()

	if err != nil {
		t.Fatalf("error inesperado con manifest malformado: %v", err)
	}
	// Con JSON malformado, se puede degradar: el ecosistema Node.js puede estar presente
	// (con frameworks vacíos) o ausente. Lo importante es que no haya pánico ni error.
	_ = resultados
}

// TestManifestInaccesible verifica que un go.mod que es un directorio (no legible)
// no produzca error — se degrada graciosamente.
func TestManifestInaccesible(t *testing.T) {
	root := t.TempDir()
	// Crear un directorio llamado "go.mod" (inaccesible como archivo)
	dirGoMod := filepath.Join(root, "go.mod")
	if err := os.MkdirAll(dirGoMod, 0755); err != nil {
		t.Fatalf("MkdirAll go.mod-dir: %v", err)
	}

	resultados, err := DetectStack(root)
	if err != nil {
		t.Fatalf("error inesperado con manifest inaccesible: %v", err)
	}
	// go.mod existe (como directorio), por lo que puede o no detectar Go degradado.
	// Lo importante: sin pánico y sin error.
	_ = resultados
}
