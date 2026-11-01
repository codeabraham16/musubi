package codeintel

// Pruebas directas de las primitivas de huella (T1/T7 de spec-graph F2). Viven acá, en su nuevo
// hogar, porque son el núcleo que COMPARTEN un ancla de observación (internal/memory) y un ancla de
// spec (internal/specgraph): la regla de matcheo calificado (#326) y el formato de byte del hash
// tienen que existir en UN solo lugar, o los dos subsistemas derivan y dejan de ser comparables.
//
// La suite #326 de internal/memory (TestFingerprintDeSimboloIgnoraDesplazamientoDeLineas,
// TestAnclaASimboloInexistenteSeRechaza y las de símbolos declarados) queda verde SIN tocar vía los
// alias finos de origins.go: eso es la guarda del move T1. Acá se fija lo mismo desde el nuevo hogar.

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ponerArchivo escribe un archivo en un tempdir y devuelve la raíz. Nombre propio para no chocar con
// los helpers de los otros _test.go del paquete.
func ponerArchivo(t *testing.T, rel, contenido string) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestSymbolMatchesCalificadoVsPelado fija R3/#326: la forma calificada `Tipo.Metodo` exige receptor
// Y nombre —así NO funde homónimos—, y la forma pelada `Nombre` matchea el comportamiento histórico
// (función, tipo o método de cualquier receptor). Es el invariante que hace que un ancla precisa a
// `A.Close` no salte por un cambio en `B.Close`.
//
// Sabotaje que lo pone en ROJO: que la forma calificada deje de exigir el receptor (&& → ||), con lo
// que `B.Close` pasaría a matchear un ancla a `A.Close` y el assert de abajo fallaría. No se puede
// saboteárselo borrando el término `s.Recv == tipo` a secas: deja `tipo` sin usar y no compila; el ||
// lo mantiene usado y rompe sólo la semántica.
// arnes: archivo="internal/codeintel/fingerprint.go"
// arnes: de="return s.Recv == tipo && s.Name == metodo"
// arnes: a="return s.Recv == tipo || s.Name == metodo"
func TestSymbolMatchesCalificadoVsPelado(t *testing.T) {
	aClose := Symbol{Name: "Close", Kind: "method", Recv: "A"}
	bClose := Symbol{Name: "Close", Kind: "method", Recv: "B"}
	libre := Symbol{Name: "Close", Kind: "func"}

	// Calificada: matchea sólo su receptor.
	if !SymbolMatches(aClose, "A.Close") {
		t.Error("A.Close debe matchear el método Close de A")
	}
	if SymbolMatches(bClose, "A.Close") {
		t.Error("A.Close NO debe matchear el Close de OTRO receptor (B): la forma calificada no funde homónimos")
	}
	// Pelada: matchea cualquier receptor y también una función libre.
	if !SymbolMatches(aClose, "Close") || !SymbolMatches(bClose, "Close") || !SymbolMatches(libre, "Close") {
		t.Error("la forma pelada `Close` debe matchear el comportamiento histórico (cualquier receptor y función libre)")
	}
}

// TestSymbolFingerprintEsElNucleoCompartido fija D3: SymbolFingerprint NO tiene su propio formato de
// hash, delega en FingerprintSymbols —el único lugar donde vive el formato de byte—. Por eso un ancla
// de spec y una de observación sobre el mismo símbolo salen byte a byte iguales. Se comprueba contra
// el cálculo manual por el mismo camino.
func TestSymbolFingerprintEsElNucleoCompartido(t *testing.T) {
	const src = "package p\n\nfunc Uno() {}\n\nfunc Dos() int {\n\treturn 2\n}\n"
	root := ponerArchivo(t, "p.go", src)

	porArchivo, err := SymbolFingerprint(root, "p.go", "Dos")
	if err != nil {
		t.Fatalf("SymbolFingerprint: %v", err)
	}
	// El MISMO cálculo, armado a mano desde el contenido y el extractor: debe coincidir al byte.
	porNucleo, err := FingerprintSymbols(src, ExtractSymbols("p.go", src), "Dos")
	if err != nil {
		t.Fatalf("FingerprintSymbols: %v", err)
	}
	if porArchivo != porNucleo {
		t.Errorf("SymbolFingerprint debe delegar en FingerprintSymbols (mismo digest): %q vs %q", porArchivo, porNucleo)
	}
}

// TestSymbolFingerprintIgnoraDesplazamientoDeLineas fija el invariante estilo S5/#252 desde el nuevo
// hogar: el digest de un símbolo no cambia porque se le muevan las líneas, sólo porque cambie su
// contenido. Es lo que hace la marca ÚTIL (anclar al símbolo y no al archivo entero).
func TestSymbolFingerprintIgnoraDesplazamientoDeLineas(t *testing.T) {
	root := ponerArchivo(t, "p.go", "package p\n\nfunc Dos() int {\n\treturn 2\n}\n")
	antes, err := SymbolFingerprint(root, "p.go", "Dos")
	if err != nil {
		t.Fatal(err)
	}
	// Se agrega código ARRIBA de Dos: baja de línea, su cuerpo es idéntico.
	if err := os.WriteFile(filepath.Join(root, "p.go"),
		[]byte("package p\n\n// una\n// dos\nfunc Uno() {}\n\nfunc Dos() int {\n\treturn 2\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	despues, err := SymbolFingerprint(root, "p.go", "Dos")
	if err != nil {
		t.Fatal(err)
	}
	if antes != despues {
		t.Error("el fingerprint del símbolo no debe cambiar por un desplazamiento de líneas")
	}
	// Pero SÍ cambia si cambia el cuerpo.
	if err := os.WriteFile(filepath.Join(root, "p.go"),
		[]byte("package p\n\nfunc Dos() int {\n\treturn 3\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cambiado, err := SymbolFingerprint(root, "p.go", "Dos")
	if err != nil {
		t.Fatal(err)
	}
	if cambiado == antes {
		t.Error("el fingerprint del símbolo DEBE cambiar cuando cambia su cuerpo")
	}
}

// TestSymbolFingerprintSimboloInexistenteEsErrSymbolNotFound: pedir un símbolo que no está devuelve
// el centinela, no un hash cualquiera. Es lo que deja al llamador distinguir "ya no existe" (deriva
// legítima) de un fallo de E/S.
func TestSymbolFingerprintSimboloInexistenteEsErrSymbolNotFound(t *testing.T) {
	root := ponerArchivo(t, "p.go", "package p\n\nfunc Uno() {}\n")
	if _, err := SymbolFingerprint(root, "p.go", "NoExiste"); !errors.Is(err, ErrSymbolNotFound) {
		t.Errorf("un símbolo inexistente debe dar ErrSymbolNotFound, dio: %v", err)
	}
}

// TestFileFingerprintCambiaConContenido: la huella del archivo entero cambia con el contenido y da
// error si el archivo no existe.
func TestFileFingerprintCambiaConContenido(t *testing.T) {
	root := ponerArchivo(t, "a.txt", "hola")
	h1, err := FileFingerprint(root, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("chau"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, err := FileFingerprint(root, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Error("la huella del archivo debe cambiar con el contenido")
	}
	if _, err := FileFingerprint(root, "no-existe.txt"); err == nil {
		t.Error("un archivo inexistente debe dar error")
	}
}

// TestSplitOriginRefYNormalize: las dos primitivas de ruta. SplitOriginRef parte por el ÚLTIMO '#'
// (un símbolo sin '#' es ancla de archivo entero); NormalizeCodePath vuelve la ruta relativa a root,
// limpia y con '/'.
func TestSplitOriginRefYNormalize(t *testing.T) {
	if p, s := SplitOriginRef("internal/foo/bar.go#Tipo.Metodo"); p != "internal/foo/bar.go" || s != "Tipo.Metodo" {
		t.Errorf("SplitOriginRef dio (%q, %q)", p, s)
	}
	if p, s := SplitOriginRef("internal/foo/bar.go"); p != "internal/foo/bar.go" || s != "" {
		t.Errorf("sin '#' el símbolo debe quedar vacío, dio (%q, %q)", p, s)
	}
	// Raíz real del SO (absoluta con letra de disco en Windows): una ruta absoluta DENTRO de root
	// debe volver relativa y con '/'. Usar una "/tmp/..." literal no sirve acá: en Windows no es
	// absoluta (le falta el disco) y la rama de filepath.IsAbs no se ejercería.
	root := t.TempDir()
	if got := NormalizeCodePath(root, filepath.Join(root, "sub", "x.go")); got != "sub/x.go" {
		t.Errorf("NormalizeCodePath de una absoluta dentro de root debe quedar relativa con '/', dio %q", got)
	}
}
