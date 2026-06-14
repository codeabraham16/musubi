// Package detector inspecciona la raíz de un proyecto y detecta los ecosistemas
// presentes (Go, Node.js, Python, Rust, etc.) usando solo la biblioteca estándar.
// Nunca es fatal: los errores de parseo de un manifest se degradan a un StackResult
// mínimo y se loguean con logx.Warn. Proyectos políglotas producen múltiples resultados.
package detector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/logx"
)

// StackResult describe un ecosistema detectado en la raíz del proyecto.
type StackResult struct {
	// Ecosystem identifica el lenguaje o plataforma: Go, Node.js, Rust, Python, etc.
	Ecosystem string `json:"ecosystem"`
	// Frameworks lista los frameworks detectados (puede ser nil/vacío si no se identificaron).
	Frameworks []string `json:"frameworks"`
	// ManifestPath es la ruta relativa al manifest que activó la detección.
	ManifestPath string `json:"manifest_path"`
	// ModuleName es el nombre del módulo o paquete si aplica (go.mod module, package.json name).
	ModuleName string `json:"module_name"`
}

// detector define la estrategia de detección para un ecosistema.
type detectorDef struct {
	ecosystem string
	// manifests es la lista de nombres de archivo a verificar (en orden).
	manifests []string
	// parse extrae frameworks y nombre de módulo del contenido del manifest.
	// Nunca debe hacer panic. Devuelve frameworks (puede ser nil) y moduleName.
	parse func(data []byte) (frameworks []string, moduleName string)
}

// packageJSON es la estructura mínima para parsear package.json de Node.js.
type packageJSON struct {
	Name            string            `json:"name"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// composerJSON es la estructura mínima para parsear composer.json de PHP.
type composerJSON struct {
	Require map[string]string `json:"require"`
}

// detectores es el registro de detectores por ecosistema.
// El orden determina la prioridad en caso de ambigüedad (aunque todos se evalúan).
var detectores = []detectorDef{
	{
		ecosystem: "Go",
		manifests: []string{"go.mod"},
		parse:     parsearGoMod,
	},
	{
		ecosystem: "Node.js",
		manifests: []string{"package.json"},
		parse:     parsearPackageJSON,
	},
	{
		ecosystem: "Rust",
		manifests: []string{"Cargo.toml"},
		parse:     parsearCargoToml,
	},
	{
		ecosystem: "Python",
		manifests: []string{"pyproject.toml", "setup.py", "requirements.txt"},
		parse:     parsearPython,
	},
	{
		ecosystem: "Java",
		manifests: []string{"pom.xml", "build.gradle", "build.gradle.kts"},
		parse:     parsearJava,
	},
	{
		ecosystem: "Ruby",
		manifests: []string{"Gemfile"},
		parse:     parsearRuby,
	},
	{
		ecosystem: "PHP",
		manifests: []string{"composer.json"},
		parse:     parsearComposerJSON,
	},
	{
		ecosystem: ".NET",
		// Los proyectos .NET usan un glob *.csproj/*.fsproj; se manejan por separado.
		manifests: nil,
		parse:     parsearDotNet,
	},
	{
		ecosystem: "Docker",
		manifests: []string{"Dockerfile"},
		parse:     parsearDocker,
	},
	{
		ecosystem: "Dart",
		manifests: []string{"pubspec.yaml"},
		parse:     parsearPresenciaSimple,
	},
	{
		ecosystem: "Elixir",
		manifests: []string{"mix.exs"},
		parse:     parsearPresenciaSimple,
	},
	{
		ecosystem: "C/C++",
		manifests: []string{"CMakeLists.txt"},
		parse:     parsearPresenciaSimple,
	},
}

// maxDepth es la profundidad máxima (relativa a root) que el walk recursivo
// explora al buscar manifests en un monorepo. root está a profundidad 0.
const maxDepth = 4

// dirsExcluidos son los directorios que el walk recursivo nunca desciende.
// Mantenerlos fuera del recorrido evita explosión de costo (node_modules, etc.)
// y resultados ruidosos (dependencias vendoreadas, artefactos de build).
var dirsExcluidos = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"dist":         true,
	"build":        true,
	"target":       true,
	"__pycache__":  true,
	".musubi":      true,
}

// DetectStack inspecciona root (y sus subdirectorios, hasta maxDepth) y devuelve
// los ecosistemas detectados. Es RECURSIVO: en un monorepo sin manifest en la
// raíz pero con manifests en subdirectorios (ej. admin/package.json,
// backend/go.mod) produce un StackResult por cada directorio con manifest, con
// ManifestPath RELATIVO a root. Un paquete único en la raíz conserva el
// comportamiento original (ManifestPath == "go.mod", etc.).
// NUNCA es fatal: errores de parseo de un manifest se degradan a un StackResult
// mínimo (solo ecosystem+manifest) y se loguean con logx.Warn.
// error != nil solo para fallos sistémicos (root inaccesible).
// Proyectos políglotas producen múltiples StackResult.
// Directorio vacío o sin manifests reconocidos devuelve []StackResult{} (no nil).
func DetectStack(root string) ([]StackResult, error) {
	// Verificar que el directorio raíz sea accesible.
	if _, err := os.ReadDir(root); err != nil {
		return nil, err
	}

	var resultados []StackResult

	// Recorrer root y subdirectorios (hasta maxDepth), detectando en cada uno.
	caminarDirectorios(root, func(dir string, depth int) {
		resultados = append(resultados, detectarEnDirectorio(root, dir)...)
	})

	// Garantizar slice no-nil para proyectos sin manifests conocidos.
	if resultados == nil {
		return []StackResult{}, nil
	}
	return resultados, nil
}

// caminarDirectorios recorre dir y sus subdirectorios hasta maxDepth (relativo a
// dir, que está a profundidad 0), invocando fn por cada directorio visitado
// (incluido dir mismo). Omite los directorios de dirsExcluidos sin descender en
// ellos. El skip-list + maxDepth mantienen el recorrido barato.
func caminarDirectorios(dir string, fn func(dir string, depth int)) {
	var recorrer func(actual string, depth int)
	recorrer = func(actual string, depth int) {
		fn(actual, depth)
		if depth >= maxDepth {
			return
		}
		entradas, err := os.ReadDir(actual)
		if err != nil {
			// Directorio ilegible: degradar silenciosamente, no es fatal.
			return
		}
		for _, e := range entradas {
			if !e.IsDir() {
				continue
			}
			nombre := e.Name()
			if dirsExcluidos[nombre] {
				continue
			}
			recorrer(filepath.Join(actual, nombre), depth+1)
		}
	}
	recorrer(dir, 0)
}

// detectarEnDirectorio aplica todos los detectores a un único directorio dir y
// devuelve los StackResult encontrados. El ManifestPath se expresa relativo a
// root (ej. "admin/package.json"); para dir == root queda solo el nombre del
// manifest (ej. "go.mod"), preservando el comportamiento original.
func detectarEnDirectorio(root, dir string) []StackResult {
	var resultados []StackResult

	for _, det := range detectores {
		// .NET es un caso especial: usa glob para encontrar *.csproj/*.fsproj.
		if det.ecosystem == ".NET" {
			if r, ok := detectarDotNet(dir); ok {
				r.ManifestPath = rutaRelativaManifest(root, dir, r.ManifestPath)
				resultados = append(resultados, r)
			}
			continue
		}

		// Para el resto de ecosistemas, probar cada manifest en orden.
		for _, manifest := range det.manifests {
			candidato := filepath.Join(dir, manifest)
			info, err := os.Stat(candidato)
			if err != nil {
				// Manifest no existe: probar el siguiente.
				continue
			}

			rutaRel := rutaRelativaManifest(root, dir, manifest)

			if info.IsDir() {
				// El manifest es un directorio (no un archivo): degradar sin error.
				logx.Warn("manifest es un directorio, se omite el parseo",
					"ecosistema", det.ecosystem, "manifest", rutaRel)
				// Agregar resultado mínimo para indicar presencia detectada.
				resultados = append(resultados, StackResult{
					Ecosystem:    det.ecosystem,
					Frameworks:   nil,
					ManifestPath: rutaRel,
					ModuleName:   "",
				})
				break
			}

			data, err := os.ReadFile(candidato)
			if err != nil {
				logx.Warn("no se pudo leer el manifest, se usa resultado mínimo",
					"ecosistema", det.ecosystem, "manifest", rutaRel, "error", err)
				resultados = append(resultados, StackResult{
					Ecosystem:    det.ecosystem,
					Frameworks:   nil,
					ManifestPath: rutaRel,
					ModuleName:   "",
				})
				break
			}

			frameworks, moduleName := det.parse(data)
			resultados = append(resultados, StackResult{
				Ecosystem:    det.ecosystem,
				Frameworks:   frameworks,
				ManifestPath: rutaRel,
				ModuleName:   moduleName,
			})
			break // Solo se usa el primer manifest encontrado por ecosistema.
		}
	}

	return resultados
}

// rutaRelativaManifest construye el ManifestPath relativo a root para un manifest
// ubicado en dir. Si dir == root, devuelve solo el nombre del manifest
// (comportamiento original); de lo contrario, prefija la ruta relativa del
// subdirectorio (ej. "admin/package.json").
func rutaRelativaManifest(root, dir, manifest string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == "." {
		return manifest
	}
	// Normalizar a "/" para consistencia entre SO (Windows usa "\").
	return filepath.ToSlash(filepath.Join(rel, manifest))
}

// detectarDotNet busca archivos *.csproj o *.fsproj en el root.
// Devuelve el primer resultado encontrado y true si se detectó al menos uno.
func detectarDotNet(root string) (StackResult, bool) {
	for _, ext := range []string{"*.csproj", "*.fsproj"} {
		matches, err := filepath.Glob(filepath.Join(root, ext))
		if err != nil || len(matches) == 0 {
			continue
		}
		// Usar el primero encontrado.
		manifest := filepath.Base(matches[0])
		data, err := os.ReadFile(matches[0])
		if err != nil {
			logx.Warn("no se pudo leer el archivo .csproj/.fsproj",
				"archivo", matches[0], "error", err)
			return StackResult{
				Ecosystem:    ".NET",
				ManifestPath: manifest,
			}, true
		}
		var frameworks []string
		// Extraer TargetFramework si está presente.
		if tf := extraerEtiquetaXML(string(data), "TargetFramework"); tf != "" {
			frameworks = []string{tf}
		}
		return StackResult{
			Ecosystem:    ".NET",
			Frameworks:   frameworks,
			ManifestPath: manifest,
		}, true
	}
	return StackResult{}, false
}

// extraerEtiquetaXML extrae el contenido de una etiqueta XML simple (sin atributos anidados).
func extraerEtiquetaXML(contenido, etiqueta string) string {
	apertura := "<" + etiqueta + ">"
	cierre := "</" + etiqueta + ">"
	inicio := strings.Index(contenido, apertura)
	if inicio < 0 {
		return ""
	}
	inicio += len(apertura)
	fin := strings.Index(contenido[inicio:], cierre)
	if fin < 0 {
		return ""
	}
	return strings.TrimSpace(contenido[inicio : inicio+fin])
}

// parsearGoMod extrae el nombre de módulo y frameworks de un go.mod.
func parsearGoMod(data []byte) (frameworks []string, moduleName string) {
	// Mapeo de dependencias a nombres de framework.
	frameworkMap := map[string]string{
		"gin-gonic/gin":   "gin",
		"labstack/echo":   "echo",
		"gofiber/fiber":   "fiber",
		"gorilla/mux":     "mux",
		"go-chi/chi":      "chi",
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		linea := strings.TrimSpace(scanner.Text())
		// Extraer nombre de módulo de la primera línea "module X".
		if strings.HasPrefix(linea, "module ") {
			moduleName = strings.TrimSpace(strings.TrimPrefix(linea, "module "))
			continue
		}
		// Detectar frameworks en las líneas require.
		for dep, framework := range frameworkMap {
			if strings.Contains(linea, dep) {
				frameworks = appendIfMissing(frameworks, framework)
			}
		}
	}
	return frameworks, moduleName
}

// parsearPackageJSON extrae nombre y frameworks de un package.json de Node.js.
func parsearPackageJSON(data []byte) (frameworks []string, moduleName string) {
	// Mapeo de paquetes a nombres de framework.
	frameworkMap := map[string]string{
		"react":          "react",
		"next":           "Next.js",
		"vue":            "vue",
		"@angular/core":  "angular",
		"express":        "express",
		"@nestjs/core":   "nest",
		"svelte":         "svelte",
		"vite":           "vite",
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		logx.Warn("package.json malformado, se usa resultado mínimo", "error", err)
		return nil, ""
	}

	moduleName = pkg.Name

	// Verificar en dependencies y devDependencies.
	for dep, framework := range frameworkMap {
		if _, ok := pkg.Dependencies[dep]; ok {
			frameworks = appendIfMissing(frameworks, framework)
		}
		if _, ok := pkg.DevDependencies[dep]; ok {
			frameworks = appendIfMissing(frameworks, framework)
		}
	}
	return frameworks, moduleName
}

// parsearCargoToml extrae nombre de crate y frameworks de un Cargo.toml.
func parsearCargoToml(data []byte) (frameworks []string, moduleName string) {
	frameworkMap := map[string]string{
		"actix-web": "actix",
		"tokio":     "tokio",
		"axum":      "axum",
		"rocket":    "rocket",
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	enPackage := false
	enDependencies := false

	for scanner.Scan() {
		linea := strings.TrimSpace(scanner.Text())

		// Detectar secciones.
		if linea == "[package]" {
			enPackage = true
			enDependencies = false
			continue
		}
		if linea == "[dependencies]" || strings.HasPrefix(linea, "[dependencies.") {
			enPackage = false
			enDependencies = true
			continue
		}
		if strings.HasPrefix(linea, "[") {
			enPackage = false
			// Continuar si es otra sección de dependencias (ej. [dev-dependencies]).
			if !strings.HasPrefix(linea, "[dependencies") {
				enDependencies = false
			}
			continue
		}

		// Extraer nombre del paquete en sección [package].
		if enPackage && strings.HasPrefix(linea, "name") {
			partes := strings.SplitN(linea, "=", 2)
			if len(partes) == 2 {
				moduleName = strings.Trim(strings.TrimSpace(partes[1]), `"`)
			}
		}

		// Detectar frameworks en dependencias (también fuera de sección explícita,
		// usando Contains en toda la línea como heurística robusta).
		for dep, framework := range frameworkMap {
			if strings.Contains(linea, dep) {
				frameworks = appendIfMissing(frameworks, framework)
			}
		}
		_ = enDependencies // usado como contexto de sección
	}
	return frameworks, moduleName
}

// parsearPython extrae frameworks de archivos de Python (requirements.txt, pyproject.toml, setup.py).
func parsearPython(data []byte) (frameworks []string, moduleName string) {
	frameworkMap := map[string]string{
		"django":  "django",
		"fastapi": "fastapi",
		"flask":   "flask",
		"pytest":  "pytest",
	}

	contenido := strings.ToLower(string(data))
	for dep, framework := range frameworkMap {
		if strings.Contains(contenido, dep) {
			frameworks = appendIfMissing(frameworks, framework)
		}
	}
	return frameworks, ""
}

// parsearJava extrae frameworks de pom.xml o build.gradle.
func parsearJava(data []byte) (frameworks []string, moduleName string) {
	contenido := string(data)
	if strings.Contains(contenido, "spring-boot") || strings.Contains(contenido, "springframework") {
		frameworks = appendIfMissing(frameworks, "spring-boot")
	}
	return frameworks, ""
}

// parsearRuby extrae frameworks de un Gemfile.
func parsearRuby(data []byte) (frameworks []string, moduleName string) {
	contenido := string(data)
	if strings.Contains(contenido, "rails") {
		frameworks = appendIfMissing(frameworks, "rails")
	}
	return frameworks, ""
}

// parsearComposerJSON extrae frameworks de composer.json de PHP.
func parsearComposerJSON(data []byte) (frameworks []string, moduleName string) {
	var comp composerJSON
	if err := json.Unmarshal(data, &comp); err != nil {
		logx.Warn("composer.json malformado, se usa resultado mínimo", "error", err)
		return nil, ""
	}

	for dep := range comp.Require {
		if strings.Contains(dep, "laravel") {
			frameworks = appendIfMissing(frameworks, "laravel")
		}
		if strings.Contains(dep, "symfony") {
			frameworks = appendIfMissing(frameworks, "symfony")
		}
	}
	return frameworks, ""
}

// parsearDocker es una función de parse trivial para Dockerfile (solo presencia).
func parsearDocker(_ []byte) (frameworks []string, moduleName string) {
	return nil, ""
}

// parsearPresenciaSimple es un parse que solo indica presencia sin extraer frameworks.
func parsearPresenciaSimple(_ []byte) (frameworks []string, moduleName string) {
	return nil, ""
}

// parsearDotNet no se usa en el flujo genérico (se maneja con glob), pero se declara
// para satisfacer la interfaz del registro.
func parsearDotNet(_ []byte) (frameworks []string, moduleName string) {
	return nil, ""
}

// appendIfMissing agrega un elemento a un slice solo si no está ya presente.
func appendIfMissing(slice []string, elem string) []string {
	for _, e := range slice {
		if e == elem {
			return slice
		}
	}
	return append(slice, elem)
}
