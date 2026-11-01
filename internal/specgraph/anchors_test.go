package specgraph

// Pruebas del anclaje y la ranciedad de specs (F2). Cubren las primitivas puras de anchors.go:
// construir/mergear entradas `ref=digest` (R2/D7), detectar deriva (R4/R7), ignorar el ruido de E/S
// ajena (R6), el round-trip lossless con digest (R11) y que specgraph NO dependa de la memoria (R9).
//
// specgraph es FS-nativo: estos tests arman un repo de verdad en un tempdir y miden contra el disco,
// igual que el motor real. El digest se cruza contra codeintel.SymbolFingerprint —la MISMA primitiva
// que usa una observación— para fijar D3: un ancla de spec y una de observación coinciden byte a byte.

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/codeintel"
)

// repoConArchivos arma un tempdir con los archivos dados (ruta relativa con '/') y devuelve la raíz.
func repoConArchivos(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, contenido := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const srcDos = `package p

func Uno() {}

func Dos() int {
	return 2
}
`

// TestBuildAnchorEntriesCaminoFeliz fija D3: la entrada que se guarda es `ref=digest`, y ese digest
// es EXACTAMENTE el que produce codeintel.SymbolFingerprint —la primitiva compartida con la memoria—.
// Un ancla de archivo entero usa FileFingerprint.
func TestBuildAnchorEntriesCaminoFeliz(t *testing.T) {
	root := repoConArchivos(t, map[string]string{"p.go": srcDos, "notas.txt": "hola"})

	entradas, err := BuildAnchorEntries(root, []string{"p.go#Dos", "notas.txt"})
	if err != nil {
		t.Fatalf("BuildAnchorEntries: %v", err)
	}
	if len(entradas) != 2 {
		t.Fatalf("esperaba 2 entradas, dio %d: %v", len(entradas), entradas)
	}
	digSimbolo, err := codeintel.SymbolFingerprint(root, "p.go", "Dos")
	if err != nil {
		t.Fatal(err)
	}
	if entradas[0] != "p.go#Dos="+digSimbolo {
		t.Errorf("el ancla de símbolo debe llevar el digest de SymbolFingerprint; dio %q", entradas[0])
	}
	digArchivo, err := codeintel.FileFingerprint(root, "notas.txt")
	if err != nil {
		t.Fatal(err)
	}
	if entradas[1] != "notas.txt="+digArchivo {
		t.Errorf("el ancla de archivo entero debe llevar el digest de FileFingerprint; dio %q", entradas[1])
	}
}

// TestBuildAnchorEntriesSimboloInexistente fija R2/D4: anclar a un símbolo que no existe (o a un
// archivo de un lenguaje sin extractor) es un ERROR al guardar, no un ancla que nacería rancia. Y el
// error OFRECE la salida —anclar al archivo entero—, que es lo accionable.
//
// Sabotaje que lo pone en ROJO: que BuildAnchorEntries no distinga ErrSymbolNotFound y lo trate como
// un fallo de E/S genérico; el mensaje pierde la guía "al archivo entero" y el assert de abajo falla.
// arnes: archivo="internal/specgraph/anchors.go"
// arnes: de="if errors.Is(derr, codeintel.ErrSymbolNotFound) {"
// arnes: a="if errors.Is(derr, codeintel.ErrSymbolNotFound) && false {"
func TestBuildAnchorEntriesSimboloInexistente(t *testing.T) {
	root := repoConArchivos(t, map[string]string{"p.go": srcDos, "datos.txt": "x"})

	// Símbolo que no está en el archivo.
	if _, err := BuildAnchorEntries(root, []string{"p.go#NoExiste"}); err == nil ||
		!strings.Contains(err.Error(), "al archivo entero") {
		t.Errorf("un símbolo inexistente debe dar error con la salida 'al archivo entero', dio: %v", err)
	}
	// Lenguaje sin extractor: un símbolo en un .txt tampoco resuelve (D4).
	if _, err := BuildAnchorEntries(root, []string{"datos.txt#Algo"}); err == nil ||
		!strings.Contains(err.Error(), "al archivo entero") {
		t.Errorf("un símbolo en un lenguaje sin extractor debe dar el mismo error, dio: %v", err)
	}
}

// TestBuildAnchorEntriesRechazos agrupa las validaciones de entrada que son todas ERROR, no truncado:
// ruta inexistente, directorio, digest pre-suministrado y ruta fuera del repo.
func TestBuildAnchorEntriesRechazos(t *testing.T) {
	root := repoConArchivos(t, map[string]string{"p.go": srcDos})
	if err := os.MkdirAll(filepath.Join(root, "undir"), 0o755); err != nil {
		t.Fatal(err)
	}
	casos := []struct {
		nombre, ref, substr string
	}{
		{"inexistente", "no/existe.go", "no existe"},
		{"directorio", "undir", "directorio"},
		{"digest pre-suministrado", "p.go=abc123", "no debe incluir un digest"},
		{"fuera del repo", "../afuera.go", "fuera del repo"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := BuildAnchorEntries(root, []string{c.ref})
			if err == nil || !strings.Contains(err.Error(), c.substr) {
				t.Errorf("%s: esperaba error con %q, dio: %v", c.nombre, c.substr, err)
			}
		})
	}
}

// TestBuildAnchorEntriesDedupYTope: misma ref dos veces colapsa a UNA entrada (dedup por ref, D7); y
// pasar más de maxAnchors refs distintas es un error explícito, no un truncado silencioso.
func TestBuildAnchorEntriesDedupYTope(t *testing.T) {
	root := repoConArchivos(t, map[string]string{"p.go": srcDos})
	entradas, err := BuildAnchorEntries(root, []string{"p.go#Dos", "p.go#Dos"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 1 {
		t.Errorf("la misma ref repetida debe colapsar a 1 entrada, dio %d: %v", len(entradas), entradas)
	}

	// Exactamente maxAnchors archivos distintos: OK.
	files := map[string]string{}
	refs := make([]string, 0, maxAnchors+1)
	for i := 0; i <= maxAnchors; i++ {
		nombre := "f" + strconv.Itoa(i) + ".txt"
		files[nombre] = "x"
		refs = append(refs, nombre)
	}
	root2 := repoConArchivos(t, files)
	if _, err := BuildAnchorEntries(root2, refs[:maxAnchors]); err != nil {
		t.Errorf("exactamente maxAnchors anclas debe ser válido, dio: %v", err)
	}
	// Una más: error (no truncado).
	if _, err := BuildAnchorEntries(root2, refs); err == nil || !strings.Contains(err.Error(), "demasiadas anclas") {
		t.Errorf("exceder el tope debe dar error 'demasiadas anclas', dio: %v", err)
	}
}

// TestMergeAnchorEntries fija D7: el conjunto final de un update. Una ref que ya estaba conserva su
// POSICIÓN y toma el digest nuevo (re-vouch); una ref nueva va al final; una ref que no vino en fresh
// se conserva tal cual (un update que toca una ref no borra el changed legítimo de otra).
func TestMergeAnchorEntries(t *testing.T) {
	existing := []string{"a.go=h1", "b.go=h2"}
	fresh := []string{"a.go=h1b", "c.go=h3"}
	out, err := MergeAnchorEntries(existing, fresh)
	if err != nil {
		t.Fatal(err)
	}
	quiero := []string{"a.go=h1b", "b.go=h2", "c.go=h3"}
	if strings.Join(out, "|") != strings.Join(quiero, "|") {
		t.Errorf("merge: quiero %v, dio %v", quiero, out)
	}

	// La unión no puede exceder el tope.
	diez := make([]string, 0, maxAnchors)
	for i := 0; i < maxAnchors; i++ {
		diez = append(diez, "f"+string(rune('0'+i))+".go=h")
	}
	if _, err := MergeAnchorEntries(diez, []string{"nuevo.go=h"}); err == nil ||
		!strings.Contains(err.Error(), "demasiadas anclas") {
		t.Errorf("la unión por encima del tope debe dar error, dio: %v", err)
	}
}

// TestStalenessChanged fija R4: un ancla cuyo contenido difiere del digest de base se marca "changed",
// con la ref como Path. Es la mitad "cambió" de la ranciedad.
//
// Sabotaje que lo pone en ROJO: anular la comparación contra el baseline (&& false), con lo que un
// contenido cambiado nunca se marca y el assert de abajo falla.
// arnes: archivo="internal/specgraph/anchors.go"
// arnes: de="if a.Baseline != \"\" && vigente != a.Baseline {"
// arnes: a="if a.Baseline != \"\" && vigente != a.Baseline && false {"
func TestStalenessChanged(t *testing.T) {
	root := repoConArchivos(t, map[string]string{"p.go": srcDos})
	entradas, err := BuildAnchorEntries(root, []string{"p.go#Dos"})
	if err != nil {
		t.Fatal(err)
	}
	fm := Frontmatter{FieldAnchors: entradas}

	// Sin tocar nada: no hay nada rancio.
	if stale := StalenessOf(root, fm); len(stale) != 0 {
		t.Fatalf("recién anclado no debe haber nada rancio, dio %v", stale)
	}
	// Se cambia el CUERPO de Dos.
	if err := os.WriteFile(filepath.Join(root, "p.go"),
		[]byte("package p\n\nfunc Uno() {}\n\nfunc Dos() int {\n\treturn 99\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := StalenessOf(root, fm)
	if len(stale) != 1 || stale[0].Reason != StaleChanged || stale[0].Path != "p.go#Dos" {
		t.Errorf("tras cambiar el cuerpo esperaba 1 'changed' en p.go#Dos, dio %v", stale)
	}
}

// TestStalenessMissing fija la otra mitad de R4: si el archivo o el símbolo anclado DESAPARECE, se
// marca "missing" —la señal de deriva más fuerte—, y funciona aun sin baseline.
func TestStalenessMissing(t *testing.T) {
	// Símbolo borrado del archivo.
	root := repoConArchivos(t, map[string]string{"p.go": srcDos})
	entradas, err := BuildAnchorEntries(root, []string{"p.go#Dos"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "p.go"), []byte("package p\n\nfunc Uno() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := StalenessOf(root, Frontmatter{FieldAnchors: entradas})
	if len(stale) != 1 || stale[0].Reason != StaleMissing {
		t.Errorf("borrar el símbolo debe dar 1 'missing', dio %v", stale)
	}

	// Archivo entero borrado.
	root2 := repoConArchivos(t, map[string]string{"p.go": srcDos})
	entradas2, err := BuildAnchorEntries(root2, []string{"p.go"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root2, "p.go")); err != nil {
		t.Fatal(err)
	}
	stale2 := StalenessOf(root2, Frontmatter{FieldAnchors: entradas2})
	if len(stale2) != 1 || stale2[0].Reason != StaleMissing {
		t.Errorf("borrar el archivo debe dar 1 'missing', dio %v", stale2)
	}
}

const srcABClose = `package p

type A struct{}

func (a A) Close() error { return nil }

type B struct{}

func (b B) Close() error { return nil }
`

// TestStalenessPrecisaAlSimbolo fija R7, el corazón del porqué se ancla al símbolo y no al archivo:
// un ancla a A.Close NO salta cuando cambia B.Close en el mismo archivo, ni cuando a A.Close sólo se
// le desplazan las líneas. Esa precisión es lo que hace la marca algo a lo que prestarle atención.
//
// Sabotaje que lo pone en ROJO: que anchorFingerprint hashee el archivo entero aun cuando hay símbolo
// (SymbolFingerprint → FileFingerprint); entonces cambiar B.Close marca el ancla a A.Close y falla.
// arnes: archivo="internal/specgraph/anchors.go"
// arnes: de="return codeintel.SymbolFingerprint(root, path, symbol)"
// arnes: a="return codeintel.FileFingerprint(root, path)"
func TestStalenessPrecisaAlSimbolo(t *testing.T) {
	// Cambio en OTRO símbolo del mismo archivo: no marca.
	root := repoConArchivos(t, map[string]string{"p.go": srcABClose})
	entradas, err := BuildAnchorEntries(root, []string{"p.go#A.Close"})
	if err != nil {
		t.Fatalf("anclar a A.Close: %v", err)
	}
	cambioEnB := strings.Replace(srcABClose, "func (b B) Close() error { return nil }",
		"func (b B) Close() error { return errors.New(\"x\") }", 1)
	if err := os.WriteFile(filepath.Join(root, "p.go"), []byte(cambioEnB), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale := StalenessOf(root, Frontmatter{FieldAnchors: entradas}); len(stale) != 0 {
		t.Errorf("cambiar B.Close NO debe marcar el ancla a A.Close, dio %v", stale)
	}

	// Desplazamiento de líneas de A.Close (cuerpo idéntico): no marca.
	root2 := repoConArchivos(t, map[string]string{"p.go": srcABClose})
	entradas2, err := BuildAnchorEntries(root2, []string{"p.go#A.Close"})
	if err != nil {
		t.Fatal(err)
	}
	conLineasArriba := "package p\n\n// comentario nuevo\n// otra línea\n" + strings.TrimPrefix(srcABClose, "package p\n")
	if err := os.WriteFile(filepath.Join(root2, "p.go"), []byte(conLineasArriba), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale := StalenessOf(root2, Frontmatter{FieldAnchors: entradas2}); len(stale) != 0 {
		t.Errorf("desplazar las líneas de A.Close NO debe marcarlo, dio %v", stale)
	}
}

// TestStalenessIgnoraErrorAjeno fija R6: un error de E/S que NO es evidencia de deriva (acá: el ancla
// apunta a un directorio, os.ReadFile falla con EISDIR, que no es NotExist ni ErrSymbolNotFound) NO
// marca. Marcar por ruido de E/S entrenaría a ignorar la marca.
//
// Sabotaje que lo pone en ROJO: colapsar el switch a `case err != nil`, con lo que CUALQUIER error
// —incluido el de leer un directorio— marca missing, y el assert de abajo (espera 0) falla.
// arnes: archivo="internal/specgraph/anchors.go"
// arnes: de="case os.IsNotExist(err), errors.Is(err, codeintel.ErrSymbolNotFound):"
// arnes: a="case err != nil:"
func TestStalenessIgnoraErrorAjeno(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "undir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Ancla a mano a un directorio, con un baseline cualquiera: leerlo falla con un error que NO es
	// NotExist. R6 dice no marcar.
	fm := Frontmatter{FieldAnchors: []string{"undir=deadbeef"}}
	if stale := StalenessOf(root, fm); len(stale) != 0 {
		t.Errorf("un error de E/S ajeno (leer un directorio) NO debe marcar, dio %v", stale)
	}
}

// TestStalenessSinBaselineNoMarcaChanged fija D2: un ancla escrita a mano sin "=digest" (baseline
// vacío) NO puede marcar "changed" —no hay contra qué comparar— pero SÍ marca "missing" si lo
// anclado desaparece. Comparar contra "" marcaría todo como cambiado.
func TestStalenessSinBaselineNoMarcaChanged(t *testing.T) {
	root := repoConArchivos(t, map[string]string{"p.go": srcDos})
	fm := Frontmatter{FieldAnchors: []string{"p.go#Dos"}} // a mano, sin digest

	// Aunque el contenido "difiera" de un baseline inexistente, no hay changed.
	if err := os.WriteFile(filepath.Join(root, "p.go"),
		[]byte("package p\n\nfunc Uno() {}\n\nfunc Dos() int {\n\treturn 7\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if stale := StalenessOf(root, fm); len(stale) != 0 {
		t.Errorf("sin baseline no debe marcar changed, dio %v", stale)
	}
	// Pero si el símbolo desaparece, SÍ marca missing (funciona sin baseline).
	if err := os.WriteFile(filepath.Join(root, "p.go"), []byte("package p\n\nfunc Uno() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale := StalenessOf(root, fm)
	if len(stale) != 1 || stale[0].Reason != StaleMissing {
		t.Errorf("sin baseline, un símbolo borrado SÍ marca missing, dio %v", stale)
	}
}

// TestStaleWarning fija el formato del canal en prosa, espejo exacto de memory.StaleWarning.
func TestStaleWarning(t *testing.T) {
	if w := StaleWarning(nil); w != "" {
		t.Errorf("sin anclas rancias el warning es vacío, dio %q", w)
	}
	w := StaleWarning([]StaleAnchor{
		{Path: "a.go#X", Reason: StaleChanged},
		{Path: "b.go", Reason: StaleMissing},
	})
	quiero := "⚠ posiblemente rancia (cambió: a.go#X; ya no existe: b.go) — "
	if w != quiero {
		t.Errorf("warning:\n quiero %q\n dio    %q", quiero, w)
	}
}

// TestAnchorsRoundTripLossless fija R11: una spec con anclas `ref=digest` sobrevive una edición del
// frontmatter con la PROSA byte a byte intacta, y el digest (que contiene '=' y '#') sobrevive el
// round-trip YAML sin corromperse. Es la vía SetList que usa el update anchor-aware (D7).
func TestAnchorsRoundTripLossless(t *testing.T) {
	cuerpo := "# La spec\n\nTexto en prosa con `backticks` y un #hash suelto.\n\n- punto uno\n- punto dos\n"
	original := "---\n" +
		"id: s-1\n" +
		"type: feature\n" +
		"# este comentario del frontmatter se conserva\n" +
		"title: Una spec\n" +
		"---\n" + cuerpo

	nuevas := []string{"internal/x.go#Tipo.Metodo=abc123def", "otro.go=99aa"}
	out, err := UpdateFrontmatterText(original, FrontmatterEdit{
		SetList: map[string][]string{FieldAnchors: nuevas},
	})
	if err != nil {
		t.Fatalf("UpdateFrontmatterText: %v", err)
	}

	// La prosa queda intacta, byte a byte.
	parsed := ParseFile(out)
	if parsed.Body != cuerpo {
		t.Errorf("la prosa debe quedar byte a byte intacta;\n quiero %q\n dio    %q", cuerpo, parsed.Body)
	}
	// Las anclas, con su digest (y su '#' y su '='), sobreviven el round-trip.
	got := List(parsed.Frontmatter, FieldAnchors)
	if strings.Join(got, "|") != strings.Join(nuevas, "|") {
		t.Errorf("las anclas deben sobrevivir con digest intacto;\n quiero %v\n dio    %v", nuevas, got)
	}
	// El comentario del frontmatter también se conserva (lossless).
	if !strings.Contains(out, "# este comentario del frontmatter se conserva") {
		t.Error("la edición debe conservar los comentarios del frontmatter")
	}
}

// TestSpecgraphNoDependeDeMemoria fija R9/D9 de forma estructural: specgraph es FS-nativo y depende
// SÓLO de codeintel (+ yaml para parsear). Si algún día alguien lo hiciera depender del motor de
// memoria, del outbox o del sync central, el fingerprint "viajaría" y este test —que inspecciona los
// imports reales, no los comentarios— lo atrapa antes del merge.
func TestSpecgraphNoDependeDeMemoria(t *testing.T) {
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	prohibidos := []string{"internal/memory", "outbox", "centralsync", "inboundsync"}
	fset := token.NewFileSet()
	vioCodeintel := false
	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, nombre, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", nombre, err)
		}
		for _, imp := range f.Imports {
			ruta := strings.Trim(imp.Path.Value, `"`)
			if ruta == "musubi/internal/codeintel" {
				vioCodeintel = true
			}
			for _, mal := range prohibidos {
				if strings.Contains(ruta, mal) {
					t.Errorf("%s importa %q: specgraph no puede depender de la memoria/outbox/sync (R9/D9)", nombre, ruta)
				}
			}
		}
	}
	// Guarda del propio test: si no vio el import a codeintel, no estaba escaneando el paquete real
	// (y un verde sería vacuo).
	if !vioCodeintel {
		t.Error("el test no encontró el import a codeintel: ¿escaneó el paquete correcto?")
	}
}
