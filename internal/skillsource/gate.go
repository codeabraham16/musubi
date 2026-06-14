package skillsource

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"musubi/internal/detector"
	"musubi/internal/logx"
	"musubi/internal/skills"
)

// depthCap es la profundidad máxima de directorios visitados por el bounded walk
// (relativo a root). Archivos a profundidad > depthCap se omiten.
const depthCap = 4

// countCap es la cantidad máxima de archivos visitados en el bounded walk.
const countCap = 10000

// directoriosOmitidos es el conjunto de nombres de directorio que el bounded walk
// siempre omite para evitar ruido (deps vendoreadas, build output, VCS, etc.).
var directoriosOmitidos = map[string]bool{
	".git":          true,
	"node_modules":  true,
	"vendor":        true,
	"dist":          true,
	"build":         true,
	"target":        true,
	"__pycache__":   true,
	".musubi":       true,
}

// IsApplicable evalúa si una CatalogEntry aplica al proyecto dado por (root, deps, stacks).
// Implementa un gate de 4 condiciones en orden fail-fast:
//  1. Ecosistema: entry.Stacks ∩ detected ecosystems ≠ ∅
//  2. Deps: si entry.Deps no vacío, ≥1 dep debe estar en deps[ecosistema]
//  3. Archivos: ≥1 trigger glob coincide con un archivo real (bounded walk)
//  4. Capabilities: todas en PATH
//
// Devuelve (true, evidence) si todas pasan; (false, evidence parcial) si alguna falla.
func IsApplicable(entry CatalogEntry, root string, deps map[string][]string, stacks []detector.StackResult) (bool, ApplicabilityEvidence) {
	var ev ApplicabilityEvidence

	// --- Paso 1: ecosistema ---
	ecosistema, ok := matchEcosistema(entry.Stacks, stacks)
	if !ok {
		return false, ev
	}
	ev.MatchedStack = ecosistema

	// --- Paso 2: dependencias ---
	if len(entry.Deps) > 0 {
		matched := matchDeps(entry.Deps, deps, ecosistema)
		if len(matched) == 0 {
			return false, ev
		}
		ev.MatchedDeps = matched
	}

	// --- Paso 3: archivos (bounded walk) ---
	if len(entry.Triggers) > 0 {
		count := matchArchivos(entry.Triggers, root)
		if count == 0 {
			return false, ev
		}
		ev.MatchedFileCount = count
	}

	// --- Paso 4: capabilities ---
	if len(entry.Capabilities) > 0 {
		missing := checkCapabilities(entry.Capabilities)
		if len(missing) > 0 {
			ev.MissingCaps = missing
			return false, ev
		}
	}

	return true, ev
}

// matchEcosistema devuelve el primer ecosistema de entry.Stacks que coincide
// exactamente con algún StackResult.Ecosystem. Devuelve ("", false) si no hay match.
func matchEcosistema(entryStacks []string, stacks []detector.StackResult) (string, bool) {
	for _, s := range stacks {
		for _, es := range entryStacks {
			if es == s.Ecosystem {
				return s.Ecosystem, true
			}
		}
	}
	return "", false
}

// matchDeps busca al menos una dep de entryDeps en las deps del proyecto.
// Busca en deps[ecosistema] primero; si no hay entrada para ese ecosistema,
// busca en todos los ecosistemas. La comparación es case-insensitive por substring.
// Devuelve las deps del proyecto que coincidieron.
func matchDeps(entryDeps []string, deps map[string][]string, ecosistema string) []string {
	// Intentar primero con el ecosistema coincidente
	proyectoDeps, ok := deps[ecosistema]
	if !ok || len(proyectoDeps) == 0 {
		// Fallback: buscar en todos los ecosistemas
		var todas []string
		for _, dd := range deps {
			todas = append(todas, dd...)
		}
		proyectoDeps = todas
	}

	var matched []string
	for _, entryDep := range entryDeps {
		entryDepLower := strings.ToLower(entryDep)
		for _, pd := range proyectoDeps {
			if strings.Contains(strings.ToLower(pd), entryDepLower) {
				matched = appendIfMissingStr(matched, pd)
				break
			}
		}
	}
	return matched
}

// matchArchivos hace un bounded walk desde root buscando archivos que coincidan
// con algún patrón glob de triggers. Usa skills.MatchGlob. Omite directorios en
// directoriosOmitidos y respeta depthCap y countCap. Devuelve la cantidad de
// archivos coincidentes (0 si ninguno).
func matchArchivos(triggers []string, root string) int {
	count := 0
	visitados := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Error de acceso: ignorar en best-effort
			return nil
		}

		if d.IsDir() {
			nombre := d.Name()
			// Omitir directorios ruidosos
			if directoriosOmitidos[nombre] {
				return filepath.SkipDir
			}
			// Verificar profundidad relativa al root
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && rel != "." {
				profundidad := len(strings.Split(rel, string(filepath.Separator)))
				if profundidad > depthCap {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Límite de archivos visitados
		visitados++
		if visitados > countCap {
			return filepath.SkipAll
		}

		// Verificar si el archivo coincide con algún trigger
		for _, trigger := range triggers {
			if skills.MatchGlob(trigger, path) || skills.MatchGlob(trigger, d.Name()) {
				count++
				// Un archivo coincidente es suficiente para pasar el gate
				// Continuar para contar todos los coincidentes (MatchedFileCount)
				break
			}
		}
		return nil
	})

	if err != nil {
		logx.Warn("skillsource: error en bounded walk", "root", root, "error", err)
	}

	return count
}

// checkCapabilities verifica que todas las capabilities estén disponibles en PATH.
// Devuelve las que no se encontraron.
func checkCapabilities(caps []string) []string {
	var missing []string
	for _, cap := range caps {
		if _, err := exec.LookPath(cap); err != nil {
			missing = append(missing, cap)
		}
	}
	return missing
}

// appendIfMissingStr agrega elem a slice solo si no está ya presente.
func appendIfMissingStr(slice []string, elem string) []string {
	for _, s := range slice {
		if s == elem {
			return slice
		}
	}
	return append(slice, elem)
}
