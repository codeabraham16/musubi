package specgraph

// Pruebas de store.go: la resolución endurecida de rutas (ResolveSpecPath y sus
// helpers puros) y el índice en memoria (descubrimiento, cache, memoización del
// grafo, directorios ignorados y SPEC.md rechazados). Lo específico de Windows
// vive en store_windows_test.go; la carrera del scan se martilla en
// TestIndexConcurrentAccessNoSeRompe (el candado lo verifica -race en CI).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func escribirArchivo(t *testing.T, root, rel, content string) string {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", abs, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
	return abs
}

// specDoc arma el contenido de una SPEC.md con id+type y líneas de frontmatter
// extra (p.ej. "parent: goal").
func specDoc(id, typ string, extra ...string) string {
	lines := []string{fence, "id: " + id, "type: " + typ}
	lines = append(lines, extra...)
	lines = append(lines, fence, "cuerpo de "+id)
	return strings.Join(lines, "\n")
}

// TestHasWindowsNamespaceSyntax fija la detección de sintaxis de ruta de Windows:
// cualquier `:` (drive o stream) o dos separadores iniciales (UNC/device). Es
// pura, así que corre en todos los SO.
//
// Sabotaje que la hace fallar: que deje de mirar el `:`, y entonces una ruta con
// drive (C:) o un stream alterno (archivo:flujo) pasan la compuerta.
// arnes: archivo="internal/specgraph/store.go"
// arnes: de="if strings.Contains(path, \":\") {"
// arnes: a="if strings.Contains(path, \"\\x00\") {"
func TestHasWindowsNamespaceSyntax(t *testing.T) {
	casos := []struct {
		path string
		want bool
	}{
		{`C:\x.md`, true},
		{"a:flujo.md", true},
		{`\\server\share\x.md`, true},
		{"//server/share/x.md", true},
		{"sub/normal.md", false},
		{"también-con-acento/x.md", false},
	}
	for _, c := range casos {
		if got := HasWindowsNamespaceSyntax(c.path); got != c.want {
			t.Errorf("HasWindowsNamespaceSyntax(%q) = %v; quería %v", c.path, got, c.want)
		}
	}
}

// TestIsPathInsideRoot fija la compuerta de contención: dentro es dentro, y un
// target que escapa por `..` cae afuera.
func TestIsPathInsideRoot(t *testing.T) {
	root := t.TempDir()
	if !IsPathInsideRoot(root, filepath.Join(root, "a", "b.md")) {
		t.Error("un hijo de root debía estar adentro")
	}
	if IsPathInsideRoot(root, filepath.Join(root, "..", "fuera.md")) {
		t.Error("un target que escapa por .. NO debía estar adentro")
	}
}

// TestCompareWalkEntries fija el orden estable del walk: primero por clave NFC,
// y el nombre crudo sólo desempata. Es lo que hace que un id duplicado resuelva
// siempre al mismo archivo, sin depender del orden del filesystem.
func TestCompareWalkEntries(t *testing.T) {
	if CompareWalkEntries(ToWalkEntry("a", false), ToWalkEntry("b", false)) != -1 {
		t.Error("a debía ordenar antes que b por clave")
	}
	if CompareWalkEntries(ToWalkEntry("b", false), ToWalkEntry("a", false)) != 1 {
		t.Error("b debía ordenar después de a")
	}
	if CompareWalkEntries(ToWalkEntry("x", false), ToWalkEntry("x", false)) != 0 {
		t.Error("dos entradas idénticas debían empatar en 0")
	}
}

// TestResolveSpecPath fija el camino feliz y que cada vector peligroso falla
// CERRADO (R11). Los vectores específicos de Windows (drive/stream) están en
// store_windows_test.go; acá van los que valen en todos los SO.
func TestResolveSpecPath(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, root, "sub/SPEC.md", specDoc("x", "task-spec"))

	t.Run("camino feliz", func(t *testing.T) {
		rel, abs, err := ResolveSpecPath(root, "sub/SPEC.md")
		if err != nil {
			t.Fatalf("una ruta válida no debía fallar: %v", err)
		}
		if rel != "sub/SPEC.md" {
			t.Errorf("rel = %q; quería sub/SPEC.md (con «/»)", rel)
		}
		if abs != filepath.Join(root, "sub", "SPEC.md") {
			t.Errorf("abs = %q; quería %q", abs, filepath.Join(root, "sub", "SPEC.md"))
		}
	})

	vectores := []struct {
		nombre string
		root   string
		path   string
	}{
		{"vacía", root, ""},
		{"absoluta POSIX", root, "/etc/x.md"},
		{"no es .md", root, "sub/notas.txt"},
		{"escapa con ..", root, "../escape.md"},
		{"raíz ausente", filepath.Join(root, "no-existe"), "sub/SPEC.md"},
		{"dir ignorado", root, ".git/SPEC.md"},
	}
	for _, v := range vectores {
		t.Run(v.nombre, func(t *testing.T) {
			if _, _, err := ResolveSpecPath(v.root, v.path); err == nil {
				t.Errorf("el vector %q debía fallar cerrado y no falló", v.nombre)
			}
		})
	}
}

// TestIndexScanAndGraph fija el descubrimiento: el índice encuentra las specs
// bajo la raíz, arma su grafo y resuelve id→ruta; un .md que no es spec (un
// README) no entra como nodo.
func TestIndexScanAndGraph(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, root, "goal/SPEC.md", specDoc("goal", "goal-and-requirements"))
	escribirArchivo(t, root, "mod/SPEC.md", specDoc("mod", "module-design", "parent: goal"))
	escribirArchivo(t, root, "README.md", "# no soy una spec")

	idx := NewIndex(root)
	g := idx.Graph()
	if len(g.Nodes) != 2 || g.Nodes["goal"] == nil || g.Nodes["mod"] == nil {
		t.Fatalf("quería 2 nodos (goal, mod); NodeOrder=%v", g.NodeOrder)
	}
	if got, ok := idx.PathForID("mod"); !ok || got != "mod/SPEC.md" {
		t.Errorf("PathForID(mod) = %q,%v; quería mod/SPEC.md,true", got, ok)
	}
	if len(idx.ContentEntries()) != 2 {
		t.Errorf("ContentEntries debía traer las 2 specs, no el README")
	}
}

// TestIndexMemoizacion fija D-memo: Graph() devuelve el MISMO grafo mientras el
// conjunto de archivos no cambia, y uno NUEVO cuando aparece un archivo.
func TestIndexMemoizacion(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, root, "a/SPEC.md", specDoc("a", "task-spec"))
	idx := NewIndex(root)

	g1 := idx.Graph()
	if idx.Graph() != g1 {
		t.Error("sin cambios, Graph() debía devolver el mismo puntero memoizado")
	}
	escribirArchivo(t, root, "b/SPEC.md", specDoc("b", "task-spec"))
	g2 := idx.Graph()
	if g2 == g1 {
		t.Error("al aparecer un archivo, el grafo debía reconstruirse (puntero nuevo)")
	}
	if len(g2.Nodes) != 2 {
		t.Errorf("el grafo reconstruido debía tener 2 nodos, tiene %d", len(g2.Nodes))
	}
}

// TestWalkIgnoresDirs fija D6: una SPEC.md dentro de un directorio ignorado
// (.git, node_modules, testdata) no se indexa; una visible sí.
//
// Sabotaje que la hace fallar: que la regla del punto exija DOS puntos, y
// entonces .git deja de estar ignorado y su SPEC.md se cuela al grafo.
// arnes: archivo="internal/specgraph/store.go"
// arnes: de="strings.HasPrefix(name, \".\") ||"
// arnes: a="strings.HasPrefix(name, \"..\") ||"
func TestWalkIgnoresDirs(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, root, "real/SPEC.md", specDoc("real", "task-spec"))
	escribirArchivo(t, root, ".git/SPEC.md", specDoc("gitspec", "task-spec"))
	escribirArchivo(t, root, "node_modules/paq/SPEC.md", specDoc("nm", "task-spec"))
	escribirArchivo(t, root, "testdata/SPEC.md", specDoc("td", "task-spec"))

	g := NewIndex(root).Graph()
	if len(g.Nodes) != 1 || g.Nodes["real"] == nil {
		t.Fatalf("sólo la spec visible debía indexarse; NodeOrder=%v", g.NodeOrder)
	}
}

// TestRejectedSpecs fija D9: un archivo llamado SPEC.md que no parsea como spec
// se denuncia con su motivo, mientras que un `spec.md` en MINÚSCULA (diario SDD)
// NO se denuncia — el nombre se compara case-sensitive.
//
// Sabotaje que la hace fallar: que el filtro del nombre se invierta y rechace
// todo MENOS las SPEC.md, dejando pasar en silencio una SPEC.md rota.
// arnes: archivo="internal/specgraph/store.go"
// arnes: de="if filepath.Base(abs) != specFileName {"
// arnes: a="if filepath.Base(abs) == specFileName {"
func TestRejectedSpecs(t *testing.T) {
	root := t.TempDir()
	escribirArchivo(t, root, "bien/SPEC.md", specDoc("ok", "task-spec"))
	escribirArchivo(t, root, "mal/SPEC.md", fence+"\ntitle: sin identidad\n"+fence+"\ncuerpo")
	escribirArchivo(t, root, "diario/spec.md", fence+"\ntitle: minúscula\n"+fence+"\ncuerpo")

	rej := NewIndex(root).RejectedSpecs()
	if len(rej) != 1 {
		t.Fatalf("quería 1 rechazo (mal/SPEC.md), dio %+v", rej)
	}
	if rej[0].Rel != "mal/SPEC.md" || rej[0].Reason == "" {
		t.Errorf("rechazo mal reportado: %+v", rej[0])
	}
}

// TestIndexConcurrentAccessNoSeRompe martilla los métodos públicos del índice
// desde muchas goroutines. scan() es un read-modify-write sobre el cache y el
// graphCache, protegido por el Mutex EXCLUSIVO del Index (D3); sin ese candado,
// dos goroutines escribirían el map del cache a la vez. El runtime de Go puede
// abortar con «concurrent map writes» acá mismo, y -race lo acusa en CI: por eso
// el candado es exclusivo y no de lectura (go-hygiene #4). Este test comprueba
// que con el candado no hay deadlock ni panic y el resultado sigue siendo sano.
func TestIndexConcurrentAccessNoSeRompe(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		escribirArchivo(t, root, fmt.Sprintf("s%d/SPEC.md", i), specDoc(fmt.Sprintf("id%d", i), "task-spec"))
	}
	idx := NewIndex(root)

	var wg sync.WaitGroup
	for w := 0; w < 48; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			switch w % 4 {
			case 0:
				idx.Graph()
			case 1:
				idx.ContentEntries()
			case 2:
				idx.RejectedSpecs()
			case 3:
				idx.PathForID("id1")
			}
		}(w)
	}
	wg.Wait()

	if g := idx.Graph(); len(g.Nodes) != 5 {
		t.Errorf("tras el martilleo el grafo debía tener 5 nodos, tiene %d", len(g.Nodes))
	}
}

// TestIndexReparseaArchivoModificado fija R10: una spec que cambió en disco DEBE
// reparsearse en el próximo scan; el índice no puede servir contenido rancio.
// scan() revalida por (modTime,size), así que un cuerpo de OTRO largo se detecta
// sí o sí, sin depender de la resolución de mtime del filesystem.
//
// Sabotaje que la hace fallar: que scan() relea SÓLO cuando el archivo es nuevo en
// el cache (!ok) e ignore el cambio de modTime/size, y entonces sirve para siempre
// el contenido viejo que cacheó la primera vez.
// arnes: archivo="internal/specgraph/store.go"
// arnes: de="if !ok || !entry.modTime.Equal(fi.ModTime()) || entry.size != fi.Size() {"
// arnes: a="if !ok {"
func TestIndexReparseaArchivoModificado(t *testing.T) {
	root := t.TempDir()
	abs := escribirArchivo(t, root, "a/SPEC.md", specDoc("a", "task-spec"))
	idx := NewIndex(root)

	ce := idx.ContentEntries()
	if len(ce) != 1 || strings.Contains(ce[0].Content, "reescrito") {
		t.Fatalf("el primer scan debía cachear sólo el cuerpo original; dio %+v", ce)
	}

	// Reescribir con un cuerpo de OTRO largo (el size cambia seguro) y empujar el
	// modTime, para no depender de la resolución temporal del filesystem.
	nuevo := specDoc("a", "task-spec") + "\ncuerpo reescrito, bastante más largo que el de antes"
	if err := os.WriteFile(abs, []byte(nuevo), 0o644); err != nil {
		t.Fatalf("reescribir: %v", err)
	}
	futuro := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(abs, futuro, futuro); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	ce = idx.ContentEntries()
	if len(ce) != 1 {
		t.Fatalf("seguía habiendo 1 spec; dio %d", len(ce))
	}
	if !strings.Contains(ce[0].Content, "reescrito") {
		t.Errorf("el índice quedó RANCIO: el contenido debía reparsearse al nuevo cuerpo, dio %q", ce[0].Content)
	}
}

// TestResolveSpecPathRechazaSymlink fija R11(b): una ruta que CRUZA un symlink se
// rechaza; el índice nunca sigue un enlace. Un symlink-a-directorio sería la vía
// para leer specs de fuera de la raíz saltándose la contención, así que la
// compuerta falla CERRADO. En Windows crear un symlink pide privilegio: si
// os.Symlink no puede, el test se saltea —no es un fallo del invariante— y el CI
// de Linux lo corre de verdad.
//
// Sabotaje que la hace fallar: que la compuerta de reparse points quede muerta, y
// entonces una ruta que pasa por el enlace resuelve en vez de rechazarse.
// arnes: archivo="internal/specgraph/store.go"
// arnes: de="if isReparsePoint(walked) {"
// arnes: a="if false && isReparsePoint(walked) {"
func TestResolveSpecPathRechazaSymlink(t *testing.T) {
	root := t.TempDir()
	fuera := t.TempDir()
	escribirArchivo(t, fuera, "SPEC.md", specDoc("secreta", "task-spec"))

	enlace := filepath.Join(root, "link")
	if err := os.Symlink(fuera, enlace); err != nil {
		t.Skipf("no se pudieron crear symlinks acá (%v): falta privilegio/Developer Mode", err)
	}

	if _, _, err := ResolveSpecPath(root, "link/SPEC.md"); err == nil {
		t.Error("una ruta que pasa por un symlink debía rechazarse (R11b) y resolvió")
	}
}

// TestResolveSpecPathRechazaNombresNoPortables fija R16/D6: un nombre de segmento
// legal en el filesystem de HOY pero que rompería al clonar en otro (reservado de
// Windows, carácter prohibido, punto o espacio final) se rechaza ANTES de tocar el
// disco. No es un problema del host actual: es que el próximo que haga `git clone`
// en Windows no podría materializar el archivo. La compuerta corre en TODOS los SO
// a propósito —un repo se comparte entre filesystems—, así que este test no se
// saltea en Linux. Ninguno de estos segmentos existe en disco: ResolveSpecPath
// tolera el final inexistente (para que create resuelva una spec nueva), así que
// SIN la compuerta estas rutas resolverían — que es lo que hace medible el sabotaje.
//
// Sabotaje que la hace fallar: anular la compuerta, y entonces un nombre no
// portable resuelve en vez de rechazarse.
// arnes: archivo="internal/specgraph/store.go"
// arnes: de="if HasPortabilityHostileName(segment) {"
// arnes: a="if false && HasPortabilityHostileName(segment) {"
func TestResolveSpecPathRechazaNombresNoPortables(t *testing.T) {
	vectores := []string{
		"foo/aux.md",   // nombre reservado de Windows (AUX)
		"foo/con.md",   // reservado (CON)
		"foo/CON.md",   // reservado, case-insensitive
		"foo/a?b.md",   // carácter prohibido en Windows
		"foo/a|b.md",   // carácter prohibido
		"bad./SPEC.md", // componente que termina en punto
	}
	for _, p := range vectores {
		t.Run(p, func(t *testing.T) {
			if _, _, err := ResolveSpecPath(t.TempDir(), p); err == nil {
				t.Errorf("el nombre no portable %q debía rechazarse y resolvió", p)
			}
		})
	}
}
