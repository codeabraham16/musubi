// Command preparar-release hace las ediciones MECÁNICAS de un release y deja el árbol listo para
// el viaje hacia afuera —pero NO lo ejecuta—. Automatiza los tres toques que a mano se saltean o
// se hacen mal (CONTRIBUTING §«Publicar un release»):
//
//  1. VERSION ← X.Y.Z (la fuente ÚNICA de verdad).
//  2. cmd/musubi/versioninfo.json ← X.Y.Z en sus cuatro campos (el recurso del .exe de Windows).
//  3. CHANGELOG.md: `## [Unreleased]` pasa a `## [X.Y.Z] - fecha`, se abre un `## [Unreleased]`
//     vacío arriba, y los enlaces de comparación del pie se corren una versión.
//
// Después corre `notas-release -readme` (regenera el bloque «Novedades» de los dos README desde el
// CHANGELOG ya editado) y las tres guardas que el CI exige verdes, con la salida real. Lo editorial
// —el `### Destacado` y el `### Highlights` que el README publica— lo escribe una persona a mano en
// el `## [Unreleased]` ANTES de cortar; esta herramienta no lo inventa, sólo avisa si falta.
//
// Lo que NO hace, a propósito (regla de oro: toda acción hacia afuera se aprueba en el momento):
// commit, tag, push, firma, subida del manifest ni deploy. Eso lo imprime como la lista de
// comandos a correr, para que la persona los lance cuando quiera.
//
// Uso:
//
//	go run ./deploy/cmd/preparar-release -version 0.145.0
//	go run ./deploy/cmd/preparar-release -version 0.145.0 -n            # dry-run: no escribe nada
//	go run ./deploy/cmd/preparar-release -version 0.145.0 -solo-editar  # edita, no corre regen ni guardas
//	go run ./deploy/cmd/preparar-release -version 0.145.0 -estricto     # falta Destacado/Highlights => error
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func main() { os.Exit(ejecutar(os.Args[1:], os.Stdout, os.Stderr)) }

var (
	// Acepta X.Y.Z pelado: sin «v» delante y sin sufijo. VERSION tiene exactamente este formato
	// (dos puntos), y la guarda TestVersioninfoMatchesVERSION lo exige.
	reVersionEstricta = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	// El enlace de «sin publicar» del CHANGELOG, de donde salen la base y la URL de comparación.
	reUnreleasedLink = regexp.MustCompile(`^\[Unreleased\]: (https?://\S+/compare/)v(\d+\.\d+\.\d+)\.\.\.HEAD$`)
	// Marcador OPCIONAL del bloque «sin publicar» de un README (igual que notas-release).
	reBaseSinPublicarREADME = regexp.MustCompile(`^<!-- sin-publicar:inicio base=(\S*) -->$`)
	reModuloMusubi          = regexp.MustCompile(`(?m)^module musubi$`)
)

// ejecutar es main sin el os.Exit, para poder probarlo. Códigos: 0 bien, 1 si el trabajo falló
// (algo ilegible, nada que publicar, una guarda roja), 2 si el uso es incorrecto.
func ejecutar(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("preparar-release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	version := fs.String("version", "", "versión X.Y.Z a cortar (sin «v» delante)")
	fecha := fs.String("fecha", time.Now().Format("2006-01-02"), "fecha del release, AAAA-MM-DD")
	root := fs.String("root", "", "raíz del repo (por defecto se busca el go.mod subiendo desde el directorio actual)")
	n := fs.Bool("n", false, "dry-run: muestra qué cambiaría y no escribe nada")
	soloEditar := fs.Bool("solo-editar", false, "escribe las ediciones pero no corre notas-release ni las guardas")
	estricto := fs.Bool("estricto", false, "si la versión nueva no trae `### Destacado` y `### Highlights`, falla en vez de avisar")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "uso: go run ./deploy/cmd/preparar-release -version X.Y.Z [opciones]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	uso := func(formato string, a ...any) int {
		fmt.Fprintf(stderr, "preparar-release: "+formato+"\n", a...)
		fs.Usage()
		return 2
	}
	if fs.NArg() > 0 {
		return uso("argumentos de más: %q", fs.Args())
	}
	nueva, err := parsearSemver(*version)
	if err != nil {
		return uso("-version %q: %v", *version, err)
	}
	if _, err := time.Parse("2006-01-02", *fecha); err != nil {
		return uso("-fecha %q no tiene la forma AAAA-MM-DD", *fecha)
	}

	raiz := *root
	if raiz == "" {
		raiz, err = raizDelRepo()
		if err != nil {
			fmt.Fprintf(stderr, "preparar-release: %v\n", err)
			return 1
		}
	}

	rutaVERSION := filepath.Join(raiz, "VERSION")
	rutaVI := filepath.Join(raiz, "cmd", "musubi", "versioninfo.json")
	rutaCL := filepath.Join(raiz, "CHANGELOG.md")

	crudoVERSION, err := os.ReadFile(rutaVERSION)
	if err != nil {
		fmt.Fprintf(stderr, "preparar-release: no se pudo leer VERSION: %v\n", err)
		return 1
	}
	actual, err := parsearSemver(strings.TrimSpace(string(crudoVERSION)))
	if err != nil {
		fmt.Fprintf(stderr, "preparar-release: VERSION tiene %q, que no es X.Y.Z: %v\n", strings.TrimSpace(string(crudoVERSION)), err)
		return 1
	}
	if !actual.menorQue(nueva) {
		fmt.Fprintf(stderr, "preparar-release: la versión nueva (%s) tiene que ser MAYOR que la actual (%s)\n", nueva, actual)
		return 1
	}

	crudoVI, err := os.ReadFile(rutaVI)
	if err != nil {
		fmt.Fprintf(stderr, "preparar-release: no se pudo leer versioninfo.json: %v\n", err)
		return 1
	}
	nuevoVI, err := bumpVersioninfo(crudoVI, nueva)
	if err != nil {
		fmt.Fprintf(stderr, "preparar-release: versioninfo.json: %v\n", err)
		return 1
	}

	crudoCL, err := os.ReadFile(rutaCL)
	if err != nil {
		fmt.Fprintf(stderr, "preparar-release: no se pudo leer CHANGELOG.md: %v\n", err)
		return 1
	}
	nuevoCL, prev, avisosCurada, err := cortarChangelog(string(crudoCL), nueva, *fecha)
	if err != nil {
		fmt.Fprintf(stderr, "preparar-release: CHANGELOG.md: %v\n", err)
		return 1
	}
	if *estricto && len(avisosCurada) > 0 {
		for _, a := range avisosCurada {
			fmt.Fprintf(stderr, "preparar-release: %s\n", a)
		}
		return 1
	}

	nuevoVERSION := nueva.String() + "\n"

	if *n {
		fmt.Fprintf(stdout, "dry-run: NO se escribió nada. Cortaría %s (sobre %s) con fecha %s.\n\n", nueva, prev, *fecha)
		fmt.Fprintf(stdout, "VERSION: %s -> %s\n", actual, nueva)
		fmt.Fprintf(stdout, "versioninfo.json: %s.0 en sus cuatro campos\n", nueva)
		fmt.Fprintf(stdout, "CHANGELOG.md: `## [Unreleased]` -> `## [%s] - %s`, nuevo `## [Unreleased]` vacío, y enlaces del pie corridos\n", nueva, *fecha)
		imprimirAvisos(stdout, avisosCurada)
		return 0
	}

	if err := os.WriteFile(rutaVERSION, []byte(nuevoVERSION), 0o644); err != nil {
		fmt.Fprintf(stderr, "preparar-release: no se pudo escribir VERSION: %v\n", err)
		return 1
	}
	if err := os.WriteFile(rutaVI, nuevoVI, 0o644); err != nil {
		fmt.Fprintf(stderr, "preparar-release: no se pudo escribir versioninfo.json: %v\n", err)
		return 1
	}
	if err := os.WriteFile(rutaCL, []byte(nuevoCL), 0o644); err != nil {
		fmt.Fprintf(stderr, "preparar-release: no se pudo escribir CHANGELOG.md: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "✓ VERSION, versioninfo.json y CHANGELOG.md editados para %s (sobre %s).\n", nueva, prev)
	imprimirAvisos(stdout, avisosCurada)

	if *soloEditar {
		fmt.Fprintln(stdout, "\n-solo-editar: no se corrió notas-release ni las guardas. Corré a mano:")
		fmt.Fprintln(stdout, "  go run ./deploy/cmd/notas-release -readme")
		imprimirGuardas(stdout)
		imprimirViaje(stdout, nueva)
		return 0
	}

	// Regenerar el bloque «Novedades» de los dos README desde el CHANGELOG ya editado.
	fmt.Fprintln(stdout, "\n→ notas-release -readme")
	if err := correrEnRaiz(raiz, stdout, stderr, "go", "run", "./deploy/cmd/notas-release", "-readme"); err != nil {
		fmt.Fprintf(stderr, "preparar-release: notas-release -readme falló: %v\n", err)
		fmt.Fprintln(stderr, "Las ediciones de VERSION/versioninfo/CHANGELOG YA están en disco; revisá y corregí antes de seguir.")
		return 1
	}

	// Avisar si algún README tiene un bloque «sin publicar» cuya base quedó vieja con el corte.
	for _, nombre := range []string{"README.md", "README.en.md"} {
		if aviso := revisarSinPublicar(raiz, nombre, nueva); aviso != "" {
			fmt.Fprintf(stdout, "⚠ %s\n", aviso)
		}
	}

	// Las tres guardas que el CI exige verdes. Son la autoridad final: si el corte dejó una
	// sección sin titulares o algo quedó desalineado, acá se ve en rojo con la salida real.
	fmt.Fprintln(stdout, "\n→ guardas")
	rojo := false
	if err := correrEnRaiz(raiz, stdout, stderr, "go", "test", "./cmd/musubi/", "-run", "^TestVersioninfoMatchesVERSION$"); err != nil {
		rojo = true
	}
	if err := correrEnRaiz(raiz, stdout, stderr, "go", "test", "./deploy/cmd/notas-release/", "-run",
		"^(TestChangelogRealTieneLaEstructuraEsperada|TestReadmeNovedadesAlDia|TestSinPublicarDeLosReadmeReales)$"); err != nil {
		rojo = true
	}
	if rojo {
		fmt.Fprintln(stderr, "\npreparar-release: una guarda quedó en ROJO. Las ediciones están en disco; corregí lo que marca arriba.")
		return 1
	}

	fmt.Fprintln(stdout, "\n✓ Guardas verdes. El árbol está listo para el release.")
	imprimirViaje(stdout, nueva)
	return 0
}

func imprimirAvisos(w io.Writer, avisos []string) {
	for _, a := range avisos {
		fmt.Fprintf(w, "⚠ %s\n", a)
	}
}

func imprimirGuardas(w io.Writer) {
	fmt.Fprintln(w, "  go test ./cmd/musubi/ -run '^TestVersioninfoMatchesVERSION$'")
	fmt.Fprintln(w, "  go test ./deploy/cmd/notas-release/ -run '^(TestChangelogRealTieneLaEstructuraEsperada|TestReadmeNovedadesAlDia|TestSinPublicarDeLosReadmeReales)$'")
}

// imprimirViaje lista —sin ejecutarlo— el viaje de comandos hacia afuera. Cada paso privilegiado
// o hacia afuera se aprueba en el momento (regla de oro); por eso esto se imprime y no se corre.
func imprimirViaje(w io.Writer, v semver) {
	fmt.Fprintln(w, "\n── Viaje hacia afuera (revisá y lanzá a mano; NADA de esto se corrió) ──")
	fmt.Fprintln(w, "1. Revisá el diff y commiteá:")
	fmt.Fprintf(w, "     git add VERSION cmd/musubi/versioninfo.json CHANGELOG.md README.md README.en.md\n")
	fmt.Fprintf(w, "     git commit -m \"chore(release): cortar %s\"\n", v)
	fmt.Fprintln(w, "2. Mergeá a main (PR o fast-forward) y traé main local al día.")
	fmt.Fprintf(w, "3. Etiquetá y empujá el tag (release.yml compila 6 plataformas y aborta si el tag != VERSION):\n")
	fmt.Fprintf(w, "     git tag v%s && git push origin v%s\n", v, v)
	fmt.Fprintln(w, "4. Firmá A MANO (las claves privadas NO están en el CI; el orden es irreversible):")
	fmt.Fprintln(w, "     ./deploy/firmar-windows.sh    # PRIMERO")
	fmt.Fprintln(w, "     ./deploy/firmar-release.sh    # DESPUÉS")
	fmt.Fprintln(w, "5. Subí manifest.json + .sig al release.")
	fmt.Fprintln(w, "6. Deploy (con el visto bueno del dueño, en el momento):")
	fmt.Fprintln(w, "     sudo ./deploy/redesplegar-cerebro.sh <binario-nuevo> <sha256>")
	fmt.Fprintln(w, "     # actualizar las máquinas de la malla, y verificar con `musubi cerebro --alcance`")
}

// ---- semver (X.Y.Z pelado) --------------------------------------------------------------------

type semver struct{ a, b, c int }

func parsearSemver(s string) (semver, error) {
	m := reVersionEstricta.FindStringSubmatch(s)
	if m == nil {
		return semver{}, fmt.Errorf("no tiene la forma X.Y.Z")
	}
	// Los \d+ ya garantizan que Atoi no falla.
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	c, _ := strconv.Atoi(m[3])
	return semver{a, b, c}, nil
}

func (v semver) menorQue(o semver) bool {
	if v.a != o.a {
		return v.a < o.a
	}
	if v.b != o.b {
		return v.b < o.b
	}
	return v.c < o.c
}

func (v semver) String() string { return fmt.Sprintf("%d.%d.%d", v.a, v.b, v.c) }

// ---- versioninfo.json -------------------------------------------------------------------------

// versioninfo refleja cmd/musubi/versioninfo.json campo por campo y EN ORDEN, para que el
// round-trip no reordene ni pierda nada: el encoder emite los campos en orden de declaración.
// La guarda TestFielAlArchivoReal garantiza que el reflejo sea byte-exacto.
type versioninfo struct {
	FixedFileInfo struct {
		FileVersion    cuaterna `json:"FileVersion"`
		ProductVersion cuaterna `json:"ProductVersion"`
		FileFlagsMask  string   `json:"FileFlagsMask"`
		FileFlags      string   `json:"FileFlags"`
		FileOS         string   `json:"FileOS"`
		FileType       string   `json:"FileType"`
		FileSubType    string   `json:"FileSubType"`
	} `json:"FixedFileInfo"`
	StringFileInfo struct {
		CompanyName      string `json:"CompanyName"`
		FileDescription  string `json:"FileDescription"`
		FileVersion      string `json:"FileVersion"`
		InternalName     string `json:"InternalName"`
		LegalCopyright   string `json:"LegalCopyright"`
		OriginalFilename string `json:"OriginalFilename"`
		ProductName      string `json:"ProductName"`
		ProductVersion   string `json:"ProductVersion"`
		Comments         string `json:"Comments"`
	} `json:"StringFileInfo"`
	VarFileInfo struct {
		Translation struct {
			LangID    string `json:"LangID"`
			CharsetID string `json:"CharsetID"`
		} `json:"Translation"`
	} `json:"VarFileInfo"`
	IconPath     string `json:"IconPath"`
	ManifestPath string `json:"ManifestPath"`
}

type cuaterna struct {
	Major int `json:"Major"`
	Minor int `json:"Minor"`
	Patch int `json:"Patch"`
	Build int `json:"Build"`
}

// bumpVersioninfo pone X.Y.Z en los cuatro campos de versión (FixedFileInfo numérico y
// StringFileInfo como "X.Y.Z.0") y re-serializa con el mismo formato que el archivo: indent de
// dos espacios, sin escapar HTML, y con el salto de línea final que agrega el encoder.
func bumpVersioninfo(crudo []byte, v semver) ([]byte, error) {
	var vi versioninfo
	dec := json.NewDecoder(bytes.NewReader(crudo))
	dec.DisallowUnknownFields() // si el archivo gana un campo que no reflejamos, fallar en vez de perderlo
	if err := dec.Decode(&vi); err != nil {
		return nil, fmt.Errorf("no se pudo parsear (¿campo nuevo sin reflejar en la struct?): %w", err)
	}
	cuat := cuaterna{Major: v.a, Minor: v.b, Patch: v.c, Build: 0}
	vi.FixedFileInfo.FileVersion = cuat
	vi.FixedFileInfo.ProductVersion = cuat
	vi.StringFileInfo.FileVersion = v.String() + ".0"
	vi.StringFileInfo.ProductVersion = v.String() + ".0"

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&vi); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---- CHANGELOG --------------------------------------------------------------------------------

// cortarChangelog convierte el `## [Unreleased]` en `## [X.Y.Z] - fecha`, abre un `## [Unreleased]`
// vacío arriba, y corre los enlaces de comparación del pie una versión. Devuelve el CHANGELOG
// nuevo, la versión previa (la base del corte) y los avisos editoriales (falta Destacado/Highlights).
//
// Falla si no hay nada que publicar bajo `## [Unreleased]`: cortar ahí dejaría una versión sin
// titulares y la guarda TestChangelogRealTieneLaEstructuraEsperada se pondría roja.
func cortarChangelog(cl string, v semver, fecha string) (nuevo, prev string, avisos []string, err error) {
	// El repo es LF (.gitattributes eol=lf); trabajamos en LF y preservamos el final del archivo.
	if strings.Contains(cl, "\r\n") {
		return "", "", nil, fmt.Errorf("tiene CRLF; el repo es LF (.gitattributes eol=lf). Normalizalo antes")
	}
	lineas := strings.Split(cl, "\n")

	idxUnreleased := -1
	for i, l := range lineas {
		if strings.TrimSpace(l) == "## [Unreleased]" {
			idxUnreleased = i
			break
		}
	}
	if idxUnreleased == -1 {
		return "", "", nil, fmt.Errorf("no encontré el encabezado `## [Unreleased]`")
	}

	// El cuerpo de Unreleased va hasta el próximo `## ` (que cierra la sección). Ese cuerpo es lo
	// que pasa a ser la versión nueva.
	idxFinCuerpo := len(lineas)
	for i := idxUnreleased + 1; i < len(lineas); i++ {
		if strings.HasPrefix(strings.TrimSpace(lineas[i]), "## ") {
			idxFinCuerpo = i
			break
		}
	}
	cuerpo := lineas[idxUnreleased+1 : idxFinCuerpo]
	hayTitular := false
	tieneDestacado, tieneHighlights := false, false
	for _, l := range cuerpo {
		if strings.HasPrefix(l, "- ") {
			hayTitular = true
		}
		switch strings.TrimSpace(l) {
		case "### Destacado":
			tieneDestacado = true
		case "### Highlights":
			tieneHighlights = true
		}
	}
	if !hayTitular {
		return "", "", nil, fmt.Errorf("`## [Unreleased]` no tiene nada que publicar (ni una viñeta `- `): escribí las entradas antes de cortar")
	}
	if !tieneDestacado {
		avisos = append(avisos, "el `## [Unreleased]` no trae `### Destacado` (el resumen en español que el README publica): escribilo a mano antes de cortar, o el README caerá al fallback")
	}
	if !tieneHighlights {
		avisos = append(avisos, "el `## [Unreleased]` no trae `### Highlights` (el resumen en inglés del README): escribilo a mano antes de cortar, o el README caerá al fallback")
	}

	// Enlaces del pie: correr `[Unreleased]` a la versión nueva e insertar el enlace de la versión.
	idxLink := -1
	var urlBase string
	for i, l := range lineas {
		if m := reUnreleasedLink.FindStringSubmatch(l); m != nil {
			idxLink = i
			urlBase = m[1]
			prev = m[2]
			break
		}
	}
	if idxLink == -1 {
		return "", "", nil, fmt.Errorf("no encontré el enlace `[Unreleased]: .../compare/vX.Y.Z...HEAD` en el pie")
	}
	anterior, errPrev := parsearSemver(prev)
	if errPrev != nil {
		return "", "", nil, fmt.Errorf("la base del enlace [Unreleased] (%q) no es X.Y.Z: %v", prev, errPrev)
	}
	if !anterior.menorQue(v) {
		return "", "", nil, fmt.Errorf("la versión nueva (%s) no es mayor que la del enlace [Unreleased] (%s)", v, anterior)
	}

	// Construir el archivo nuevo. Dos splices sobre el slice de líneas: primero el pie (índice más
	// alto) para no correr el índice del encabezado.
	nuevoLinkUnreleased := fmt.Sprintf("[Unreleased]: %sv%s...HEAD", urlBase, v)
	nuevoLinkVersion := fmt.Sprintf("[%s]: %sv%s...v%s", v, urlBase, prev, v)
	out := append([]string(nil), lineas...)
	out = append(out[:idxLink], append([]string{nuevoLinkUnreleased, nuevoLinkVersion}, out[idxLink+1:]...)...)

	// El encabezado: después de `## [Unreleased]` insertar una línea en blanco y `## [X.Y.Z] - fecha`.
	// La línea en blanco que ya seguía a Unreleased queda como separador bajo la versión nueva.
	encabezadoVersion := fmt.Sprintf("## [%s] - %s", v, fecha)
	out = append(out[:idxUnreleased+1], append([]string{"", encabezadoVersion}, out[idxUnreleased+1:]...)...)

	return strings.Join(out, "\n"), prev, avisos, nil
}

// revisarSinPublicar devuelve un aviso si el README tiene el bloque OPCIONAL «sin publicar» y su
// `base=` no es la versión recién cortada (la foto de `main` quedó vieja). Vacío si no hay bloque
// o si está al día. La guarda TestSinPublicarDeLosReadmeReales es la autoridad; esto sólo avisa.
func revisarSinPublicar(raiz, nombre string, v semver) string {
	crudo, err := os.ReadFile(filepath.Join(raiz, nombre))
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(strings.ReplaceAll(string(crudo), "\r\n", "\n"), "\n") {
		if m := reBaseSinPublicarREADME.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			if m[1] != v.String() {
				return fmt.Sprintf("%s: el bloque «sin publicar» dice base=%s y la versión es %s; actualizalo a mano o borralo (lo canta TestSinPublicarDeLosReadmeReales)", nombre, m[1], v)
			}
			return ""
		}
	}
	return ""
}

// ---- utilidades -------------------------------------------------------------------------------

// raizDelRepo sube desde el directorio actual hasta el go.mod con `module musubi`.
func raizDelRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && reModuloMusubi.Match(b) {
			return dir, nil
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return "", fmt.Errorf("no encontré la raíz del repo (un go.mod con `module musubi`) subiendo desde el directorio actual")
		}
		dir = padre
	}
}

// correrEnRaiz corre un comando con la raíz del repo como directorio y vuelca su salida.
func correrEnRaiz(raiz string, stdout, stderr io.Writer, nombre string, args ...string) error {
	cmd := exec.Command(nombre, args...)
	cmd.Dir = raiz
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
