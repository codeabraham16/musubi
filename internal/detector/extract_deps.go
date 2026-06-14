package detector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"musubi/internal/logx"
)

// depsCacheEntry almacena el resultado de un parse de manifest junto a su mtime
// para invalidar el caché cuando el archivo cambia.
type depsCacheEntry struct {
	mtime time.Time
	deps  map[string][]string
}

// depsCache es un caché a nivel de paquete para ExtractDeps, indexado por ruta
// de manifest principal. Evita re-parsear manifests sin cambios en sucesivas llamadas.
var depsCache sync.Map

// ExtractDeps inspecciona root (y sus subdirectorios, hasta maxDepth) y devuelve
// un mapa de ecosystem→[]rawDepKey. Es RECURSIVO/AGREGADO: en un monorepo parsea
// cada manifest encontrado y hace la UNIÓN deduplicada de las deps por ecosistema.
// Un paquete único en la raíz conserva el comportamiento original.
// Es best-effort y nunca fatal: manifests ausentes o malformados generan logx.Warn
// y producen un slice vacío para ese ecosistema.
// El único error posible es cuando root es inaccesible.
// Omite los directorios de dirsExcluidos (node_modules, vendor, etc.) sin descender,
// lo que mantiene el recorrido barato.
func ExtractDeps(root string) (map[string][]string, error) {
	// Verificar que el directorio raíz sea accesible
	if _, err := os.ReadDir(root); err != nil {
		return nil, err
	}

	resultado := make(map[string][]string)

	// agregar hace la unión deduplicada de deps en el ecosistema dado.
	agregar := func(ecosistema string, deps []string) {
		if deps == nil {
			return
		}
		actual := resultado[ecosistema]
		for _, d := range deps {
			actual = appendIfMissing(actual, d)
		}
		resultado[ecosistema] = actual
	}

	// Recorrer root y subdirectorios (hasta maxDepth), agregando por directorio.
	// Cada extractor por ecosistema conserva su caché por archivo (mtime).
	caminarDirectorios(root, func(dir string, depth int) {
		agregar("Go", extraerDepsGo(dir))
		agregar("Node.js", extraerDepsNode(dir))
		agregar("Python", extraerDepsPython(dir))
		agregar("Rust", extraerDepsRust(dir))
	})

	return resultado, nil
}

// leerConCache lee el contenido de un archivo con invalidación por mtime.
// Si el archivo no existe, devuelve nil, nil (no error).
// Si falla la lectura, loguea y devuelve nil, nil.
func leerConCache(ruta string) ([]byte, bool) {
	info, err := os.Stat(ruta)
	if err != nil {
		// Archivo ausente: no es error
		return nil, false
	}

	// Comprobar si el caché sigue vigente
	if entrada, ok := depsCache.Load(ruta); ok {
		cached := entrada.(depsCacheEntry)
		if cached.mtime.Equal(info.ModTime()) {
			return nil, true // cache hit; el llamador usa cached.deps
		}
	}

	data, err := os.ReadFile(ruta)
	if err != nil {
		logx.Warn("no se pudo leer manifest para ExtractDeps", "path", ruta, "error", err)
		return nil, false
	}
	return data, false
}

// cargarDesdeCache devuelve las deps cacheadas para un manifest dado, si existen y siguen vigentes.
func cargarDesdeCache(ruta string) (map[string][]string, bool) {
	info, err := os.Stat(ruta)
	if err != nil {
		return nil, false
	}
	if entrada, ok := depsCache.Load(ruta); ok {
		cached := entrada.(depsCacheEntry)
		if cached.mtime.Equal(info.ModTime()) {
			return cached.deps, true
		}
	}
	return nil, false
}

// guardarEnCache almacena las deps de un manifest en el caché con su mtime actual.
func guardarEnCache(ruta string, deps map[string][]string) {
	info, err := os.Stat(ruta)
	if err != nil {
		return
	}
	depsCache.Store(ruta, depsCacheEntry{
		mtime: info.ModTime(),
		deps:  deps,
	})
}

// extraerDepsGo parsea go.mod y extrae las rutas de módulo del bloque require.
func extraerDepsGo(root string) []string {
	ruta := filepath.Join(root, "go.mod")

	// Intentar desde caché
	if cached, hit := cargarDesdeCache(ruta); hit {
		return cached["Go"]
	}

	data, err := os.ReadFile(ruta)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Warn("no se pudo leer go.mod para ExtractDeps", "error", err)
		}
		return nil
	}

	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	enRequire := false

	for scanner.Scan() {
		linea := strings.TrimSpace(scanner.Text())

		// Require multilínea: require (...)
		if linea == "require (" {
			enRequire = true
			continue
		}
		if enRequire && linea == ")" {
			enRequire = false
			continue
		}

		// Require de una sola línea: require module/path v1.0.0
		if strings.HasPrefix(linea, "require ") && !strings.HasSuffix(linea, "(") {
			partes := strings.Fields(strings.TrimPrefix(linea, "require "))
			if len(partes) >= 1 {
				modulo := partes[0]
				if !strings.HasPrefix(modulo, "//") {
					deps = appendIfMissing(deps, modulo)
				}
			}
			continue
		}

		// Dentro del bloque require multilínea
		if enRequire && linea != "" && !strings.HasPrefix(linea, "//") {
			partes := strings.Fields(linea)
			if len(partes) >= 1 {
				modulo := partes[0]
				deps = appendIfMissing(deps, modulo)
			}
		}
	}

	result := map[string][]string{"Go": deps}
	guardarEnCache(ruta, result)
	return deps
}

// extraerDepsNode parsea package.json y extrae claves de dependencies y devDependencies.
func extraerDepsNode(root string) []string {
	ruta := filepath.Join(root, "package.json")

	// Intentar desde caché
	if cached, hit := cargarDesdeCache(ruta); hit {
		return cached["Node.js"]
	}

	data, err := os.ReadFile(ruta)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Warn("no se pudo leer package.json para ExtractDeps", "error", err)
		}
		return nil
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		logx.Warn("package.json malformado en ExtractDeps", "error", err)
		result := map[string][]string{"Node.js": {}}
		guardarEnCache(ruta, result)
		return []string{}
	}

	var deps []string
	for nombre := range pkg.Dependencies {
		deps = appendIfMissing(deps, nombre)
	}
	for nombre := range pkg.DevDependencies {
		deps = appendIfMissing(deps, nombre)
	}

	result := map[string][]string{"Node.js": deps}
	guardarEnCache(ruta, result)
	return deps
}

// extraerDepsPython extrae dependencias de requirements.txt y/o pyproject.toml.
// Elimina especificadores de versión (>=, ==, ~=, etc.) y extras ([extras]).
func extraerDepsPython(root string) []string {
	// Intentar requirements.txt primero
	rutaReqs := filepath.Join(root, "requirements.txt")
	rutaPyproject := filepath.Join(root, "pyproject.toml")

	// Verificar si hay caché válido para requirements.txt
	if cached, hit := cargarDesdeCache(rutaReqs); hit {
		return cached["Python"]
	}

	var deps []string
	var usarReqs bool

	dataReqs, errReqs := os.ReadFile(rutaReqs)
	if errReqs == nil {
		usarReqs = true
		scanner := bufio.NewScanner(bytes.NewReader(dataReqs))
		for scanner.Scan() {
			linea := strings.TrimSpace(scanner.Text())
			if linea == "" || strings.HasPrefix(linea, "#") {
				continue
			}
			// Eliminar especificador de versión y extras
			nombre := normalizarDepPython(linea)
			if nombre != "" {
				deps = appendIfMissing(deps, nombre)
			}
		}
	}

	// Si no hay requirements.txt, intentar pyproject.toml
	if !usarReqs {
		if cached, hit := cargarDesdeCache(rutaPyproject); hit {
			return cached["Python"]
		}

		dataPyproject, err := os.ReadFile(rutaPyproject)
		if err == nil {
			deps = extraerDepsPyproject(dataPyproject)
		}
	}

	rutaPrincipal := rutaReqs
	if !usarReqs {
		rutaPrincipal = rutaPyproject
	}

	if len(deps) > 0 || usarReqs {
		result := map[string][]string{"Python": deps}
		guardarEnCache(rutaPrincipal, result)
	}

	return deps
}

// normalizarDepPython elimina especificadores de versión y extras de un nombre de dep Python.
// Ej: "fastapi>=0.110.0" → "fastapi", "pydantic[extras]>=2.0" → "pydantic"
func normalizarDepPython(linea string) string {
	// Separadores de versión en requirements.txt
	for _, sep := range []string{">=", "==", "~=", "!=", "<=", ">", "<", "["} {
		if idx := strings.Index(linea, sep); idx >= 0 {
			linea = linea[:idx]
		}
	}
	return strings.TrimSpace(linea)
}

// extraerDepsPyproject hace un escaneo línea a línea de pyproject.toml
// buscando secciones [project.dependencies] o [tool.poetry.dependencies].
func extraerDepsPyproject(data []byte) []string {
	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	enDeps := false

	for scanner.Scan() {
		linea := strings.TrimSpace(scanner.Text())

		// Detectar sección de dependencias
		if linea == "[project.dependencies]" || linea == "[tool.poetry.dependencies]" {
			enDeps = true
			continue
		}
		// Nueva sección termina la anterior
		if strings.HasPrefix(linea, "[") {
			enDeps = false
			continue
		}

		if enDeps && linea != "" && !strings.HasPrefix(linea, "#") {
			// Formato: nombre = ">=1.0" o nombre = {version = "..."}
			// También puede ser una lista en formato TOML inline
			partes := strings.SplitN(linea, "=", 2)
			nombre := strings.Trim(strings.TrimSpace(partes[0]), `"'`)
			if nombre != "" && !strings.HasPrefix(nombre, "#") {
				deps = appendIfMissing(deps, nombre)
			}
		}
	}
	return deps
}

// extraerDepsRust parsea Cargo.toml y extrae claves de [dependencies] y [dev-dependencies].
func extraerDepsRust(root string) []string {
	ruta := filepath.Join(root, "Cargo.toml")

	// Intentar desde caché
	if cached, hit := cargarDesdeCache(ruta); hit {
		return cached["Rust"]
	}

	data, err := os.ReadFile(ruta)
	if err != nil {
		if !os.IsNotExist(err) {
			logx.Warn("no se pudo leer Cargo.toml para ExtractDeps", "error", err)
		}
		return nil
	}

	var deps []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	enDeps := false

	for scanner.Scan() {
		linea := strings.TrimSpace(scanner.Text())

		// Detectar sección [dependencies] o [dev-dependencies]
		if linea == "[dependencies]" || linea == "[dev-dependencies]" {
			enDeps = true
			continue
		}
		// Subsección como [dependencies.nombre] — seguir en modo deps
		if strings.HasPrefix(linea, "[dependencies.") || strings.HasPrefix(linea, "[dev-dependencies.") {
			enDeps = true
			continue
		}
		// Nueva sección distinta termina el modo deps
		if strings.HasPrefix(linea, "[") {
			enDeps = false
			continue
		}

		if enDeps && linea != "" && !strings.HasPrefix(linea, "#") {
			// Formato: nombre = "versión" o nombre = { version = "...", ... }
			partes := strings.SplitN(linea, "=", 2)
			if len(partes) >= 1 {
				nombre := strings.TrimSpace(partes[0])
				if nombre != "" {
					deps = appendIfMissing(deps, nombre)
				}
			}
		}
	}

	result := map[string][]string{"Rust": deps}
	guardarEnCache(ruta, result)
	return deps
}
