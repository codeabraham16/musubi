package memory

// verifytarget.go implementa la PRUEBA ESTRUCTURAL del gate de verificación.
//
// El digest de `result` congela lo que el agente DIJO. Esto congela lo que el
// proyecto ES: un step puede declarar `verify_target` (globs de archivos) y Musubi
// calcula la identidad leyendo el disco. Al emitir el veredicto vuelve a derivarla:
// si los archivos cambiaron desde que se congeló el candidato, el veredicto no
// vale, porque revisó otra cosa. El agente no participa de esa comprobación y por
// lo tanto no puede afirmarla ni negarla — es lo único que el sistema deriva solo.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// skipDirs son carpetas que nunca forman parte de un candidato: ruido, artefactos
// de build o la propia memoria de Musubi (que cambia al journalear y haría que el
// digest nunca coincidiera consigo mismo).
var skipDirs = map[string]bool{
	".git": true, ".musubi": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".next": true, "__pycache__": true,
}

// globMatch matchea una ruta relativa (con '/') contra un patrón estilo glob con
// soporte de `**` (cero o más segmentos). Los segmentos sueltos usan path.Match,
// así que `*`, `?` y `[...]` funcionan como es habitual dentro de un segmento.
func globMatch(pattern, name string) bool {
	return segMatch(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func segMatch(pat, seg []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			// `**` final: matchea todo lo que quede.
			if len(pat) == 1 {
				return true
			}
			// Probar consumiendo 0..n segmentos.
			for i := 0; i <= len(seg); i++ {
				if segMatch(pat[1:], seg[i:]) {
					return true
				}
			}
			return false
		}
		if len(seg) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], seg[0]); err != nil || !ok {
			return false
		}
		pat, seg = pat[1:], seg[1:]
	}
	return len(seg) == 0
}

// resolveVerifyTarget devuelve las rutas relativas (con '/') que matchean los
// patrones, ordenadas y sin repetir. Recorre el árbol UNA vez.
func resolveVerifyTarget(root string, patterns []string) ([]string, error) {
	clean := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
		if p != "" {
			clean = append(clean, strings.TrimPrefix(p, "./"))
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("verify_target vacío")
	}

	seen := map[string]bool{}
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // un directorio ilegible no invalida el resto
		}
		if d.IsDir() {
			if p != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		for _, pat := range clean {
			if globMatch(pat, rel) {
				seen[rel] = true
				break
			}
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("no se pudo recorrer %s: %w", root, walkErr)
	}

	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out, nil
}

// verifyTargetDigest calcula la identidad de contenido del candidato: sha256 sobre
// (ruta + sha256 del contenido) de cada archivo, en orden. Determinista entre
// corridas y entre máquinas (rutas normalizadas a '/'). Devuelve además los
// archivos que entraron, para poder decir QUÉ se congeló.
//
// Que no matchee ningún archivo es un ERROR, no un digest vacío: congelar un
// candidato inexistente daría un gate que siempre pasa, que es peor que no tenerlo.
func verifyTargetDigest(root string, patterns []string) (string, []string, error) {
	files, err := resolveVerifyTarget(root, patterns)
	if err != nil {
		return "", nil, err
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf(
			"verify_target no matcheó ningún archivo (patrones: %s): revisá las rutas, son relativas a la raíz del proyecto",
			strings.Join(patterns, ", "))
	}

	h := sha256.New()
	for _, rel := range files {
		data, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if rerr != nil {
			return "", nil, fmt.Errorf("no se pudo leer %s: %w", rel, rerr)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\x00%s\x00", rel, hex.EncodeToString(sum[:]))
	}
	return hex.EncodeToString(h.Sum(nil)), files, nil
}
