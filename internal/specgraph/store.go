// store.go porta core/store.ts: resolución endurecida de rutas (ResolveSpecPath)
// y el índice en memoria del filesystem (Index) que descubre specs, cachea su
// contenido y memoiza el grafo. Dos deltas de Musubi sobre thinkrail:
//
//   - D6: los directorios ignorados ESPEJAN walkSourceTree (el mismo árbol que
//     camina el grafo de código), no el set fijo de thinkrail. Es un predicado,
//     no un set, por la regla dot-prefix (cubre .git/.claude/.musubi/… sin
//     enumerarlos).
//   - D9: RejectedSpecs() reporta los archivos llamados SPEC.md que NO parsean
//     como spec. Es política Musubi; el core (validate.go) queda fiel.
//
// D3: el Index lleva un sync.Mutex EXCLUSIVO (no RWMutex). Cada método público
// llama scan(), que es un read-modify-write sobre el cache y el graphCache: bajo
// lock de lectura sería un lost-update que -race NO detecta (carrera de lógica,
// no de memoria). Ver go-hygiene #4.

package specgraph

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/norm"
)

// SpecFileExtension es la extensión que DEBE tener todo archivo de spec.
const SpecFileExtension = ".md"

// specFileName es el nombre que marca la intención "esto quería ser una spec de
// módulo". Se compara CASE-SENSITIVE a propósito (R1/R12, decisión #1 del dueño):
// el diario SDD usa `spec.md`/`proposal.md` en MINÚSCULA, que no son specs y NO
// deben reportarse. Foldear el nombre haría gritar por cada spec.md del diario.
const specFileName = "SPEC.md"

// isIgnoredDirName espeja la lógica de walkSourceTree (D6): dot-dirs por prefijo
// + un puñado de carpetas de build/deps por nombre. Es la clave de la
// consistencia con el grafo de código (D2): las dos vistas caminan el mismo árbol.
func isIgnoredDirName(name string) bool {
	return strings.HasPrefix(name, ".") ||
		name == "vendor" || name == "testdata" ||
		name == "node_modules" || name == "dist" || name == "coverage"
}

// fold normaliza un nombre a NFC y lo pasa a minúscula, igual que el
// `name.normalize("NFC").toLowerCase()` de thinkrail. Se usa SÓLO en la
// resolución de rutas (sobre-rechazo por case-fold), nunca en el walk.
func fold(name string) string {
	return strings.ToLower(norm.NFC.String(name))
}

func isSep(b byte) bool { return b == '\\' || b == '/' }

// HasWindowsNamespaceSyntax detecta drive/stream/UNC/namespace de Windows. Porta
// `win32.parse(path).root !== "" || path.includes(":")`: cualquier `:` (drive
// `C:`, ADS `archivo:flujo`) o dos separadores iniciales (UNC `\\srv\share`,
// device `\\?\`, `\\.\`). Se evalúa sólo en Windows (en POSIX un `:` en un
// nombre es legal), pero es pura y testeable en cualquier SO.
func HasWindowsNamespaceSyntax(path string) bool {
	if strings.Contains(path, ":") {
		return true
	}
	if len(path) >= 2 && isSep(path[0]) && isSep(path[1]) {
		return true
	}
	return false
}

// HasPortabilityHostileName detecta un nombre de segmento que es legal en el
// filesystem donde se CREA pero rompe al clonar el repo en otro (R16). Un repo
// creado en Linux puede llamar a una spec `aux.md` o `a?b.md`; al clonarlo en
// Windows el checkout falla o renombra en silencio y la spec queda inalcanzable.
// Se rechaza en la creación —el único momento en que podemos evitarlo— sin exigir
// que el host sea Windows: la regla es la UNIÓN de las restricciones de todos los
// filesystems, no las del SO actual. Cubre control chars, los reservados de
// Windows <>:"|?*, punto o espacio final (Windows los come al crear) y los device
// names CON/PRN/AUX/NUL/COMn/LPTn (reservados aunque lleven extensión).
func HasPortabilityHostileName(segment string) bool {
	if segment == "" {
		return false
	}
	for _, r := range segment {
		if r < 0x20 {
			return true
		}
		switch r {
		case '<', '>', ':', '"', '|', '?', '*':
			return true
		}
	}
	if last := segment[len(segment)-1]; last == '.' || last == ' ' {
		return true
	}
	// Los device names se comparan sobre el STEM (antes del primer punto): en
	// Windows `AUX.md` está tan reservado como `AUX`, igual que `NUL.txt`.
	stem := segment
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	lower := strings.ToLower(stem)
	switch lower {
	case "con", "prn", "aux", "nul":
		return true
	}
	if len(lower) == 4 && (strings.HasPrefix(lower, "com") || strings.HasPrefix(lower, "lpt")) && lower[3] >= '1' && lower[3] <= '9' {
		return true
	}
	return false
}

// IsPathInsideRoot es la compuerta de contención final: target cae dentro de root
// sólo si el rel no es "..", no empieza con "../" y no es absoluto. Usa
// filepath nativo, así que en Windows aplica semántica win32 (volumen-aware) y
// filepath.Rel devuelve error entre volúmenes distintos ⇒ fuera (falla cerrado).
func IsPathInsideRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// isReparsePoint detecta un symlink O un junction (reparse point de Windows).
// os.Lstat marca un symlink con ModeSymlink, pero un junction creado con
// `mklink /J` NO lo lleva: Go lo reporta como ModeIrregular (medido). Si la guarda
// sólo mirara ModeSymlink, un create a través de un junction ESCAPARÍA de la raíz
// —el mismo escape que la guarda de symlink frena—, porque IsPathInsideRoot
// compara la ruta LÉXICA sin resolver el enlace. Por eso se miran las dos modes.
func isReparsePoint(target string) bool {
	fi, err := os.Lstat(target)
	return err == nil && fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0
}

// resolves es true si el target existe (incluido un symlink colgante): Lstat no
// sigue el enlace, igual que el lstatSync de thinkrail.
func resolves(target string) bool {
	_, err := os.Lstat(target)
	return err == nil
}

// directoryEntries lista los nombres de un directorio. ok=false ⇔ el `null` de
// thinkrail (no se pudo listar): el caller lo trata como error, distinto de un
// directorio vacío (ok=true, lista vacía).
func directoryEntries(dir string) ([]string, bool) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}
	names := make([]string, 0, len(des))
	for _, de := range des {
		names = append(names, de.Name())
	}
	return names, true
}

// resolvePathSegment canonicaliza UN segmento contra las entradas reales del
// directorio padre. Si el segmento no existe literal pero el padre existe, busca
// por case-fold: 0 coincidencias ⇒ error, >1 ⇒ error (NO adivina). Rechaza un
// directorio ignorado. El chequeo de ignorado foldea (sobre-rechazo), al revés
// que el walk que compara el nombre crudo — la asimetría deliberada de thinkrail.
func resolvePathSegment(entries []string, segment string, exists bool) (string, error) {
	name := segment
	if exists && !contains(entries, segment) {
		folded := fold(segment)
		matches := make([]string, 0)
		for _, e := range entries {
			if fold(e) == folded {
				matches = append(matches, e)
			}
		}
		if len(matches) == 0 {
			return "", fmt.Errorf("path component %q resolves to no entry its parent directory lists", segment)
		}
		if len(matches) > 1 {
			return "", fmt.Errorf("path component %q matches more than one entry on this filesystem (%s)", segment, quoteJoin(matches))
		}
		name = matches[0]
	}
	if isIgnoredDirName(fold(name)) {
		return "", fmt.Errorf("path is inside an ignored directory (%q) and would not be indexed", name)
	}
	return name, nil
}

func quoteJoin(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, it := range items {
		quoted = append(quoted, fmt.Sprintf("%q", it))
	}
	return strings.Join(quoted, ", ")
}

// ResolveSpecPath valida y canonicaliza una ruta root-relative que da el agente,
// devolviendo (rel con "/", abs nativo). Falla CERRADO en cada vector (R11): vacío,
// absoluta, sintaxis Windows, no-.md, `..`, raíz ausente, directorio no-listable,
// segmento que no resuelve o es ambiguo, symlink en el camino, o escape final de
// la raíz. El `.md` se chequea dos veces a propósito: el segundo chequeo, sobre el
// rel canónico, atrapa un archivo real con extensión en otra caja (`b.MD`).
func ResolveSpecPath(root, path string) (rel string, abs string, err error) {
	if strings.TrimSpace(path) == "" {
		return "", "", errors.New("path must not be empty")
	}
	// Una barra inicial "/" es raíz en POSIX y en win32 (node coincide en ambos);
	// una "\" inicial es raíz sólo en win32. filepath.IsAbs en Windows NO ve "/foo"
	// como absoluta, así que se chequea aparte para no dejar pasar un rooted-path.
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return "", "", fmt.Errorf("path must be root-relative, not absolute: %s", path)
	}
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(path, `\`) {
			return "", "", fmt.Errorf("path must be root-relative, not absolute: %s", path)
		}
		if HasWindowsNamespaceSyntax(path) {
			return "", "", fmt.Errorf("path must not use Windows drive or stream syntax: %s", path)
		}
	}
	if !strings.HasSuffix(path, SpecFileExtension) {
		return "", "", fmt.Errorf("spec files must end in %s: %s", SpecFileExtension, path)
	}

	segments := strings.Split(filepath.Clean(path), string(filepath.Separator))
	if contains(segments, "..") {
		return "", "", fmt.Errorf("path must stay inside the project root: %s", path)
	}
	if _, serr := os.Stat(root); serr != nil {
		return "", "", fmt.Errorf("project root does not exist: %s", root)
	}

	walked := root
	walkedExists := true
	canonical := make([]string, 0, len(segments))
	for _, segment := range segments {
		// R16/D6: un nombre no portable se rechaza ANTES de resolverlo contra el
		// disco; no es un problema del host actual sino del próximo que clone.
		if HasPortabilityHostileName(segment) {
			return "", "", fmt.Errorf("path component %q is not portable across filesystems: %s", segment, path)
		}
		var entries []string
		if walkedExists {
			listed, ok := directoryEntries(walked)
			if !ok {
				return "", "", fmt.Errorf("path passes through a directory the index cannot list: %s", path)
			}
			entries = listed
		}
		exists := walkedExists && resolves(filepath.Join(walked, segment))
		name, rerr := resolvePathSegment(entries, segment, exists)
		if rerr != nil {
			return "", "", fmt.Errorf("%w: %s", rerr, path)
		}
		walked = filepath.Join(walked, name)
		if isReparsePoint(walked) {
			return "", "", fmt.Errorf("path passes through a symlink, which the index never follows: %s", path)
		}
		canonical = append(canonical, name)
		walkedExists = exists
	}

	rel = strings.Join(canonical, "/")
	if !strings.HasSuffix(rel, SpecFileExtension) {
		return "", "", fmt.Errorf("spec files must end in %s: %s", SpecFileExtension, path)
	}
	if !IsPathInsideRoot(root, walked) {
		return "", "", fmt.Errorf("path must stay inside the project root: %s", path)
	}
	return rel, walked, nil
}

// --- Walk ordenado (determinista, independiente del orden del readdir del SO) ---

// WalkEntry es un candidato del walk con su clave de orden NFC. Key desempata
// primero (orden estable entre sistemas); Name sólo rompe empates de Key.
type WalkEntry struct {
	Name string
	Key  string
	Dir  bool
}

func ToWalkEntry(name string, dir bool) WalkEntry {
	return WalkEntry{Name: name, Key: norm.NFC.String(name), Dir: dir}
}

// CompareWalkEntries ordena por Key (NFC) y luego por Name crudo. Este orden es
// lo que hace que un id duplicado resuelva SIEMPRE al mismo archivo (BuildGraph:
// gana el primero), sin depender del orden que devuelva el filesystem.
func CompareWalkEntries(a, b WalkEntry) int {
	if a.Key != b.Key {
		if a.Key < b.Key {
			return -1
		}
		return 1
	}
	if a.Name != b.Name {
		if a.Name < b.Name {
			return -1
		}
		return 1
	}
	return 0
}

// --- Índice en memoria ---

// SpecFileRecord es una spec detectada: ruta absoluta, rel con "/", contenido
// crudo (sólo de specs reales; "" para no-specs) y su frontmatter.
type SpecFileRecord struct {
	Abs         string
	Rel         string
	Content     string
	Frontmatter Frontmatter
}

// SpecReject es un archivo llamado SPEC.md que NO parseó como spec (D9).
type SpecReject struct {
	Rel    string
	Reason string
}

// cacheEntry memoiza un archivo por (modTime,size). frontmatter puede ser nil
// (el `null` de thinkrail). content está vacío para no-specs: no gastamos
// memoria cacheando el cuerpo de un README; sólo el de las specs reales.
type cacheEntry struct {
	rel         string
	modTime     time.Time
	size        int64
	content     string
	frontmatter Frontmatter
}

// Index descubre y cachea las specs bajo una raíz. Memoiza el grafo: sólo lo
// reconstruye cuando el CONJUNTO de archivos cambió (alta/baja/modificación),
// no en cada consulta.
type Index struct {
	root       string
	mu         sync.Mutex
	cache      map[string]*cacheEntry
	graphCache *SpecGraph
}

func NewIndex(root string) *Index {
	return &Index{root: root, cache: make(map[string]*cacheEntry)}
}

// AbsPath resuelve un rel contra la raíz. No toma el lock: root es inmutable.
func (idx *Index) AbsPath(rel string) string {
	return filepath.Join(idx.root, rel)
}

// toRel convierte una ruta absoluta a rel con separadores "/".
func toRel(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs) // no debería pasar; fail-safe
	}
	return filepath.ToSlash(rel)
}

// walk devuelve las rutas absolutas de los `.md` bajo dir, en orden estable
// (CompareWalkEntries), recorriendo subdirectorios no-ignorados. NO sigue
// symlinks: ni un symlink-a-archivo (no es regular) ni uno-a-directorio (IsDir
// es falso para el dirent del symlink) entran. best-effort: un dir ilegible se
// saltea sin abortar, igual que thinkrail.
func (idx *Index) walk(dir string) []string {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	candidates := make([]WalkEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if de.IsDir() {
			if !isIgnoredDirName(name) { // nombre CRUDO (asimetría con resolve)
				candidates = append(candidates, ToWalkEntry(name, true))
			}
		} else if de.Type().IsRegular() && strings.HasSuffix(name, SpecFileExtension) {
			candidates = append(candidates, ToWalkEntry(name, false))
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return CompareWalkEntries(candidates[i], candidates[j]) < 0
	})
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		abs := filepath.Join(dir, c.Name)
		if c.Dir {
			out = append(out, idx.walk(abs)...)
		} else {
			out = append(out, abs)
		}
	}
	return out
}

// scan refresca el cache contra el disco y devuelve las specs actuales. Es un
// read-modify-write: por eso TODO caller público toma idx.mu EXCLUSIVO antes de
// llamarlo (D3). Revalida por (modTime,size); un archivo sin cambios no se
// relee ni se reparsea. Invalida graphCache sólo si el conjunto cambió.
func (idx *Index) scan() []SpecFileRecord {
	seen := make(map[string]bool)
	specs := make([]SpecFileRecord, 0)
	changed := false

	for _, abs := range idx.walk(idx.root) {
		seen[abs] = true
		fi, serr := os.Stat(abs)
		if serr != nil {
			continue
		}
		entry, ok := idx.cache[abs]
		if !ok || !entry.modTime.Equal(fi.ModTime()) || entry.size != fi.Size() {
			content, rerr := os.ReadFile(abs)
			if rerr != nil {
				if _, existed := idx.cache[abs]; existed {
					delete(idx.cache, abs)
					changed = true
				}
				continue
			}
			parsed := ParseFile(string(content))
			cached := ""
			if IsSpec(parsed.Frontmatter) {
				cached = string(content)
			}
			entry = &cacheEntry{
				rel:         toRel(idx.root, abs),
				modTime:     fi.ModTime(),
				size:        fi.Size(),
				content:     cached,
				frontmatter: parsed.Frontmatter,
			}
			idx.cache[abs] = entry
			changed = true
		}
		if IsSpec(entry.frontmatter) {
			specs = append(specs, SpecFileRecord{
				Abs: abs, Rel: entry.rel, Content: entry.content, Frontmatter: entry.frontmatter,
			})
		}
	}

	for abs := range idx.cache {
		if !seen[abs] {
			delete(idx.cache, abs)
			changed = true
		}
	}

	if changed {
		idx.graphCache = nil
	}
	return specs
}

// Graph devuelve el grafo derivado, reconstruyéndolo sólo si el conjunto cambió.
func (idx *Index) Graph() *SpecGraph {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	specs := idx.scan()
	if idx.graphCache == nil {
		entries := make([]SpecFileEntry, 0, len(specs))
		for _, r := range specs {
			entries = append(entries, SpecFileEntry{Path: r.Rel, Frontmatter: r.Frontmatter})
		}
		idx.graphCache = BuildGraph(entries)
	}
	return idx.graphCache
}

// ContentEntries devuelve las specs con su contenido crudo (para grep).
func (idx *Index) ContentEntries() []SpecContentEntry {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	specs := idx.scan()
	out := make([]SpecContentEntry, 0, len(specs))
	for _, r := range specs {
		out = append(out, SpecContentEntry{Path: r.Rel, Content: r.Content, Frontmatter: r.Frontmatter})
	}
	return out
}

// recordForID busca la spec con ese id. Asume el lock tomado: existe para que
// RecordForID y PathForID no se auto-bloqueen (sync.Mutex no es reentrante).
// Devuelve la PRIMERA en orden de walk, consistente con "gana el primero" del grafo.
func (idx *Index) recordForID(id string) (SpecFileRecord, bool) {
	for _, r := range idx.scan() {
		if v, ok := Scalar(r.Frontmatter, FieldID); ok && v == id {
			return r, true
		}
	}
	return SpecFileRecord{}, false
}

// RecordForID devuelve la spec con ese id, o ok=false si no existe.
func (idx *Index) RecordForID(id string) (SpecFileRecord, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.recordForID(id)
}

// PathForID devuelve el rel de la spec con ese id, o ok=false si no existe.
func (idx *Index) PathForID(id string) (string, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	r, ok := idx.recordForID(id)
	if !ok {
		return "", false
	}
	return r.Rel, true
}

// RejectedSpecs devuelve los archivos llamados SPEC.md que NO parsearon como
// spec (D9). Itera el cache (que tiene TODOS los .md, specs o no) tras un scan,
// y ordena por rel para una salida determinista (el map de Go no tiene orden).
func (idx *Index) RejectedSpecs() []SpecReject {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.scan()
	out := make([]SpecReject, 0)
	for abs, entry := range idx.cache {
		if filepath.Base(abs) != specFileName {
			continue
		}
		if IsSpec(entry.frontmatter) {
			continue
		}
		out = append(out, SpecReject{Rel: entry.rel, Reason: rejectReason(entry.frontmatter)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out
}

// rejectReason explica por qué un SPEC.md no es spec: frontmatter ausente/roto,
// o qué campo de identidad falta.
func rejectReason(fm Frontmatter) string {
	if fm == nil {
		return "frontmatter ausente o YAML inválido"
	}
	_, hasID := Scalar(fm, FieldID)
	_, hasType := Scalar(fm, FieldType)
	switch {
	case !hasID && !hasType:
		return "sin id ni type"
	case !hasID:
		return "sin id"
	case !hasType:
		return "sin type"
	default:
		return "no es una spec válida"
	}
}
