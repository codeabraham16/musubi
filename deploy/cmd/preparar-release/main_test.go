package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- semver -----------------------------------------------------------------------------------

func TestParsearSemver(t *testing.T) {
	ok := []struct {
		in   string
		want semver
	}{
		{"0.144.0", semver{0, 144, 0}},
		{"1.2.3", semver{1, 2, 3}},
		{"10.20.30", semver{10, 20, 30}},
	}
	for _, c := range ok {
		got, err := parsearSemver(c.in)
		if err != nil || got != c.want {
			t.Errorf("parsearSemver(%q) = %v, %v; quería %v, nil", c.in, got, err, c.want)
		}
	}
	malas := []string{"", "v0.1.0", "0.1", "0.1.0.0", "0.1.0-rc1", "a.b.c", " 0.1.0"}
	for _, m := range malas {
		if _, err := parsearSemver(m); err == nil {
			t.Errorf("parsearSemver(%q) no dio error y debía", m)
		}
	}
}

func TestMenorQue(t *testing.T) {
	casos := []struct {
		a, b semver
		want bool
	}{
		{semver{0, 144, 0}, semver{0, 145, 0}, true},
		{semver{0, 144, 0}, semver{0, 144, 1}, true},
		{semver{0, 144, 0}, semver{1, 0, 0}, true},
		{semver{0, 145, 0}, semver{0, 144, 0}, false},
		{semver{0, 144, 0}, semver{0, 144, 0}, false}, // igual no es menor
		{semver{1, 0, 0}, semver{0, 999, 999}, false},
	}
	for _, c := range casos {
		if got := c.a.menorQue(c.b); got != c.want {
			t.Errorf("%v.menorQue(%v) = %v; quería %v", c.a, c.b, got, c.want)
		}
	}
}

// ---- versioninfo.json -------------------------------------------------------------------------

// TestBumpVersioninfoFielAlArchivoReal es la red de seguridad del reflejo: round-trip del
// versioninfo.json REAL a su MISMA versión tiene que dar bytes idénticos. Si la struct se olvida un
// campo (DisallowUnknownFields lo caza al leer) o el formato difiere en un byte, este test lo canta.
func TestBumpVersioninfoFielAlArchivoReal(t *testing.T) {
	raiz, err := raizDelRepo()
	if err != nil {
		t.Fatalf("raízDelRepo: %v", err)
	}
	ruta := filepath.Join(raiz, "cmd", "musubi", "versioninfo.json")
	original, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	// La versión que hoy tiene VERSION, para bumpear a la MISMA y comparar byte a byte.
	crudoV, err := os.ReadFile(filepath.Join(raiz, "VERSION"))
	if err != nil {
		t.Fatalf("no se pudo leer VERSION: %v", err)
	}
	v, err := parsearSemver(strings.TrimSpace(string(crudoV)))
	if err != nil {
		t.Fatalf("VERSION no es X.Y.Z: %v", err)
	}
	got, err := bumpVersioninfo(original, v)
	if err != nil {
		t.Fatalf("bumpVersioninfo: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("el round-trip a la misma versión (%s) no es byte-idéntico.\n--- original ---\n%s\n--- generado ---\n%s", v, original, got)
	}
}

const versioninfoDePrueba = `{
  "FixedFileInfo": {
    "FileVersion": {
      "Major": 0,
      "Minor": 144,
      "Patch": 0,
      "Build": 0
    },
    "ProductVersion": {
      "Major": 0,
      "Minor": 144,
      "Patch": 0,
      "Build": 0
    },
    "FileFlagsMask": "3f",
    "FileFlags": "00",
    "FileOS": "040004",
    "FileType": "01",
    "FileSubType": "00"
  },
  "StringFileInfo": {
    "CompanyName": "codeabraham16",
    "FileDescription": "Musubi",
    "FileVersion": "0.144.0.0",
    "InternalName": "musubi",
    "LegalCopyright": "MIT",
    "OriginalFilename": "Musubi.exe",
    "ProductName": "Musubi",
    "ProductVersion": "0.144.0.0",
    "Comments": "x"
  },
  "VarFileInfo": {
    "Translation": {
      "LangID": "0409",
      "CharsetID": "04B0"
    }
  },
  "IconPath": "icon.ico",
  "ManifestPath": ""
}
`

func TestBumpVersioninfoCambiaLosCuatroCampos(t *testing.T) {
	got, err := bumpVersioninfo([]byte(versioninfoDePrueba), semver{0, 145, 2})
	if err != nil {
		t.Fatalf("bumpVersioninfo: %v", err)
	}
	var vi versioninfo
	if err := json.Unmarshal(got, &vi); err != nil {
		t.Fatalf("salida ilegible: %v", err)
	}
	if vi.FixedFileInfo.FileVersion != (cuaterna{0, 145, 2, 0}) {
		t.Errorf("FixedFileInfo.FileVersion = %+v; quería {0 145 2 0}", vi.FixedFileInfo.FileVersion)
	}
	if vi.FixedFileInfo.ProductVersion != (cuaterna{0, 145, 2, 0}) {
		t.Errorf("FixedFileInfo.ProductVersion = %+v; quería {0 145 2 0}", vi.FixedFileInfo.ProductVersion)
	}
	if vi.StringFileInfo.FileVersion != "0.145.2.0" {
		t.Errorf("StringFileInfo.FileVersion = %q; quería \"0.145.2.0\"", vi.StringFileInfo.FileVersion)
	}
	if vi.StringFileInfo.ProductVersion != "0.145.2.0" {
		t.Errorf("StringFileInfo.ProductVersion = %q; quería \"0.145.2.0\"", vi.StringFileInfo.ProductVersion)
	}
	// Un campo que no es de versión no se toca.
	if vi.StringFileInfo.CompanyName != "codeabraham16" || vi.IconPath != "icon.ico" {
		t.Errorf("se tocó un campo que no era de versión: %q / %q", vi.StringFileInfo.CompanyName, vi.IconPath)
	}
}

func TestBumpVersioninfoRechazaCampoDesconocido(t *testing.T) {
	// Si el archivo gana un campo que la struct no refleja, hay que fallar en vez de perderlo.
	conExtra := strings.Replace(versioninfoDePrueba, `"IconPath": "icon.ico",`, `"IconPath": "icon.ico",`+"\n  \"CampoNuevo\": 1,", 1)
	if _, err := bumpVersioninfo([]byte(conExtra), semver{0, 145, 0}); err == nil {
		t.Error("bumpVersioninfo no falló ante un campo desconocido y debía")
	}
}

// ---- CHANGELOG --------------------------------------------------------------------------------

const changelogConContenido = `# Changelog

Preámbulo.

## [Unreleased]

### Destacado
- Algo en español.

### Highlights
- Something in English.

### Added
- Una cosa nueva.

## [0.144.0] - 2026-10-25

### Added
- Lo viejo.

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.144.0...HEAD
[0.144.0]: https://github.com/codeabraham16/musubi/compare/v0.143.0...v0.144.0
`

func TestCortarChangelogFeliz(t *testing.T) {
	nuevo, prev, avisos, err := cortarChangelog(changelogConContenido, semver{0, 145, 0}, "2026-10-26")
	if err != nil {
		t.Fatalf("cortarChangelog: %v", err)
	}
	if prev != "0.144.0" {
		t.Errorf("prev = %q; quería 0.144.0", prev)
	}
	if len(avisos) != 0 {
		t.Errorf("avisos = %v; quería ninguno (trae Destacado y Highlights)", avisos)
	}
	want := `# Changelog

Preámbulo.

## [Unreleased]

## [0.145.0] - 2026-10-26

### Destacado
- Algo en español.

### Highlights
- Something in English.

### Added
- Una cosa nueva.

## [0.144.0] - 2026-10-25

### Added
- Lo viejo.

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.145.0...HEAD
[0.145.0]: https://github.com/codeabraham16/musubi/compare/v0.144.0...v0.145.0
[0.144.0]: https://github.com/codeabraham16/musubi/compare/v0.143.0...v0.144.0
`
	if nuevo != want {
		t.Errorf("CHANGELOG cortado no coincide.\n--- quería ---\n%s\n--- obtuvo ---\n%s", want, nuevo)
	}
}

func TestCortarChangelogRechazaUnreleasedVacio(t *testing.T) {
	// Unreleased sin ninguna viñeta: cortar dejaría una versión sin titulares.
	cl := "# Changelog\n\n## [Unreleased]\n\n## [0.144.0] - 2026-10-25\n\n### Added\n- Viejo.\n\n[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.144.0...HEAD\n[0.144.0]: https://github.com/codeabraham16/musubi/compare/v0.143.0...v0.144.0\n"
	if _, _, _, err := cortarChangelog(cl, semver{0, 145, 0}, "2026-10-26"); err == nil {
		t.Error("cortarChangelog no falló con un Unreleased vacío y debía")
	} else if !strings.Contains(err.Error(), "nada que publicar") {
		t.Errorf("el error no explica por qué: %v", err)
	}
}

func TestCortarChangelogAvisaSinCurada(t *testing.T) {
	cl := "# Changelog\n\n## [Unreleased]\n\n### Added\n- Algo.\n\n## [0.144.0] - 2026-10-25\n\n### Added\n- Viejo.\n\n[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.144.0...HEAD\n[0.144.0]: https://github.com/codeabraham16/musubi/compare/v0.143.0...v0.144.0\n"
	_, _, avisos, err := cortarChangelog(cl, semver{0, 145, 0}, "2026-10-26")
	if err != nil {
		t.Fatalf("cortarChangelog: %v", err)
	}
	if len(avisos) != 2 {
		t.Errorf("avisos = %v; quería 2 (falta Destacado y Highlights)", avisos)
	}
}

func TestCortarChangelogRechazaCRLF(t *testing.T) {
	cl := strings.ReplaceAll(changelogConContenido, "\n", "\r\n")
	if _, _, _, err := cortarChangelog(cl, semver{0, 145, 0}, "2026-10-26"); err == nil {
		t.Error("cortarChangelog no rechazó CRLF y debía")
	}
}

func TestCortarChangelogRechazaSinLinkUnreleased(t *testing.T) {
	cl := "# Changelog\n\n## [Unreleased]\n\n### Added\n- Algo.\n\n## [0.144.0] - 2026-10-25\n\n### Added\n- Viejo.\n"
	if _, _, _, err := cortarChangelog(cl, semver{0, 145, 0}, "2026-10-26"); err == nil {
		t.Error("cortarChangelog no falló sin el enlace [Unreleased] y debía")
	}
}

func TestCortarChangelogRechazaVersionNoMayor(t *testing.T) {
	// 0.144.0 es la base del enlace; cortar 0.144.0 o menos no es mayor.
	for _, v := range []semver{{0, 144, 0}, {0, 143, 9}} {
		if _, _, _, err := cortarChangelog(changelogConContenido, v, "2026-10-26"); err == nil {
			t.Errorf("cortarChangelog(%v) no falló y la versión no es mayor que 0.144.0", v)
		}
	}
}

// ---- revisarSinPublicar -----------------------------------------------------------------------

func TestRevisarSinPublicar(t *testing.T) {
	dir := t.TempDir()
	escribir := func(nombre, cont string) {
		if err := os.WriteFile(filepath.Join(dir, nombre), []byte(cont), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Sin bloque: sin aviso.
	escribir("README.md", "# Proyecto\n\nTexto.\n")
	if a := revisarSinPublicar(dir, "README.md", semver{0, 145, 0}); a != "" {
		t.Errorf("sin bloque dio aviso: %q", a)
	}
	// Base al día: sin aviso.
	escribir("README.md", "# P\n\n<!-- sin-publicar:inicio base=0.145.0 -->\n### x\n<!-- sin-publicar:fin -->\n")
	if a := revisarSinPublicar(dir, "README.md", semver{0, 145, 0}); a != "" {
		t.Errorf("base al día dio aviso: %q", a)
	}
	// Base vieja: avisa.
	escribir("README.md", "# P\n\n<!-- sin-publicar:inicio base=0.144.0 -->\n### x\n<!-- sin-publicar:fin -->\n")
	if a := revisarSinPublicar(dir, "README.md", semver{0, 145, 0}); a == "" {
		t.Error("base vieja no avisó y debía")
	}
	// README inexistente: sin aviso (no revienta).
	if a := revisarSinPublicar(dir, "NO_EXISTE.md", semver{0, 145, 0}); a != "" {
		t.Errorf("README inexistente dio aviso: %q", a)
	}
}

// ---- ejecutar (integración en repo temporal) --------------------------------------------------

// repoDePrueba arma un repo mínimo con go.mod, VERSION, versioninfo.json y CHANGELOG.
func repoDePrueba(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	escribir := func(ruta, cont string) {
		p := filepath.Join(dir, ruta)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(cont), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escribir("go.mod", "module musubi\n\ngo 1.23\n")
	escribir("VERSION", "0.144.0\n")
	escribir("cmd/musubi/versioninfo.json", versioninfoDePrueba)
	escribir("CHANGELOG.md", changelogConContenido)
	return dir
}

func TestEjecutarSoloEditarEscribe(t *testing.T) {
	dir := repoDePrueba(t)
	var out, errb bytes.Buffer
	cod := ejecutar([]string{"-version", "0.145.0", "-root", dir, "-fecha", "2026-10-26", "-solo-editar"}, &out, &errb)
	if cod != 0 {
		t.Fatalf("código %d; stderr: %s", cod, errb.String())
	}
	leer := func(ruta string) string {
		b, err := os.ReadFile(filepath.Join(dir, ruta))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if v := leer("VERSION"); v != "0.145.0\n" {
		t.Errorf("VERSION = %q; quería \"0.145.0\\n\"", v)
	}
	if vi := leer("cmd/musubi/versioninfo.json"); !strings.Contains(vi, `"FileVersion": "0.145.0.0"`) || strings.Contains(vi, "144") {
		t.Errorf("versioninfo.json no quedó en 0.145.0:\n%s", vi)
	}
	cl := leer("CHANGELOG.md")
	if !strings.Contains(cl, "## [0.145.0] - 2026-10-26") {
		t.Errorf("CHANGELOG sin el encabezado nuevo:\n%s", cl)
	}
	if !strings.Contains(cl, "[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.145.0...HEAD") {
		t.Error("CHANGELOG no corrió el enlace [Unreleased]")
	}
	if !strings.Contains(cl, "[0.145.0]: https://github.com/codeabraham16/musubi/compare/v0.144.0...v0.145.0") {
		t.Error("CHANGELOG no insertó el enlace de la versión nueva")
	}
}

func TestEjecutarDryRunNoEscribe(t *testing.T) {
	dir := repoDePrueba(t)
	antesV, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	antesCL, _ := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	antesVI, _ := os.ReadFile(filepath.Join(dir, "cmd/musubi/versioninfo.json"))
	var out, errb bytes.Buffer
	cod := ejecutar([]string{"-version", "0.145.0", "-root", dir, "-n"}, &out, &errb)
	if cod != 0 {
		t.Fatalf("código %d; stderr: %s", cod, errb.String())
	}
	despuesV, _ := os.ReadFile(filepath.Join(dir, "VERSION"))
	despuesCL, _ := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	despuesVI, _ := os.ReadFile(filepath.Join(dir, "cmd/musubi/versioninfo.json"))
	if !bytes.Equal(antesV, despuesV) || !bytes.Equal(antesCL, despuesCL) || !bytes.Equal(antesVI, despuesVI) {
		t.Error("el dry-run escribió en disco y no debía")
	}
	if !strings.Contains(out.String(), "dry-run") {
		t.Errorf("la salida no dice dry-run:\n%s", out.String())
	}
}

func TestEjecutarRechazaVersionVieja(t *testing.T) {
	dir := repoDePrueba(t)
	for _, v := range []string{"0.144.0", "0.143.0"} {
		var out, errb bytes.Buffer
		if cod := ejecutar([]string{"-version", v, "-root", dir, "-solo-editar"}, &out, &errb); cod == 0 {
			t.Errorf("ejecutar(-version %s) devolvió 0 y la versión no es mayor que 0.144.0", v)
		}
	}
}

func TestEjecutarUsoIncorrecto(t *testing.T) {
	dir := repoDePrueba(t)
	casos := [][]string{
		{"-root", dir},                                          // sin -version
		{"-version", "v0.145.0", "-root", dir},                  // con v
		{"-version", "0.145.0", "-fecha", "ayer", "-root", dir}, // fecha mala
		{"-version", "0.145.0", "-root", dir, "sobra"},          // argumento de más
	}
	for _, args := range casos {
		var out, errb bytes.Buffer
		if cod := ejecutar(args, &out, &errb); cod != 2 {
			t.Errorf("ejecutar(%q) = %d; quería 2 (uso incorrecto). stderr: %s", args, cod, errb.String())
		}
	}
}
