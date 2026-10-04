// Command notas-release arma las notas de cada release de Musubi a partir del CHANGELOG.md y
// mantiene al día el bloque «Novedades» de los dos README.
//
// POR QUÉ EXISTE. Las releases de GitHub salían con el cuerpo vacío —el job `create-release` de
// release.yml sólo le pasaba `tag_name` a la acción— y el README no decía una palabra de lo que
// cambió en cada versión. Esa información ya estaba escrita, en el CHANGELOG.md. Este comando la
// baja a dos lugares sin que nadie la copie a mano:
//
//	notas-release -version 0.141.0            cuerpo de la release, a stdout, en español
//	notas-release -version 0.141.0 -lang en   lo mismo, con los rótulos en inglés
//	notas-release -readme                     reescribe lo que hay entre los marcadores
//	                                          `<!-- novedades:inicio -->` y `<!-- novedades:fin -->`
//	                                          de README.md (es) y README.en.md (en)
//
// Sólo stdlib, y sin red: lee el CHANGELOG del disco. Si algo falla, falla diciendo qué (exit 1);
// los errores de uso salen con exit 2. El workflow de release lo corre con `continue-on-error`, así
// que un fallo acá nunca impide publicar la release: sale sin texto, como salía antes.
//
// CÓMO SE ELIGE EL TITULAR de cada viñeta de primer nivel (`- ` en la columna 0). Es una función
// determinista del texto y no pasa por ningún modelo:
//
//  1. Se toma el PÁRRAFO DE CABEZA de la viñeta: su primera línea y las de continuación, unidas con
//     un espacio, hasta la primera línea en blanco, sub-lista o cerca de código.
//  2. Se descartan los emoji y símbolos del principio (todo lo que precede a la primera letra,
//     número, `*` o backtick; los signos que ABREN un par —paréntesis, comillas, `¿`— se respetan
//     para no dejar la mitad del par).
//  3. Si empieza con `**`, el titular es el texto hasta el `**` que cierra, aunque esté en otra
//     línea. Si ese texto tiene menos de 15 runas o termina en `:`, es una etiqueta y no una
//     frase: se le suma lo que falta de la primera oración. Si no empieza con `**`, el titular es
//     la primera oración (hasta el primer `. ` fuera de un code span, sin cortar en `p. ej.`).
//  4. Se limpia: sin `**`, `[texto](url)` queda en `texto`, sin `. : , ;` al final, espacios
//     colapsados y todo `<` fuera de un code span pasa a `&lt;` para que no abra una etiqueta.
//  5. Tope de 160 runas: se corta en el límite de palabra anterior y se agrega `…`. Si el corte
//     deja un backtick abierto, se corta ANTES de ese backtick: un titular nunca sale con el
//     código a medias.
//
// El orden de los grupos es Added, Changed, Fixed, Security, Removed. `Notes` y cualquier grupo
// que no esté en esa lista no aportan titulares ni cuentan en el «y N más».
//
// LOS GRUPOS CURADOS. Los titulares se derivan del texto y a veces salen con la jerga de quien
// escribió la viñeta. Por eso una versión puede traer, escritos a mano por quien publica,
// `### Destacado` (viñetas en español) y `### Highlights` (las mismas ideas en inglés): 3 o 4
// oraciones llanas, y son LO PRIMERO que se muestra.
//
//   - Cada viñeta curada sale con su TEXTO COMPLETO: no pasa por la regla del titular (ni se queda
//     con la primera oración ni se corta a 160 runas) y conserva su punto final. Sí pasa por la
//     misma limpieza de Markdown (sin `**`, sin enlaces, `<` escapado, backticks balanceados) y por
//     la misma neutralización de «N herramientas» que un titular.
//   - Cuerpo de la release: el bloque curado va primero, con hasta 6 viñetas, y después los grupos
//     de siempre. Con `-lang en` es Highlights, o Destacado si no hay Highlights.
//   - README en español: si hay Destacado, hasta 4 viñetas curadas y el cierre «Y todo lo demás, en
//     el CHANGELOG» en lugar de «… y N más». README en inglés: Highlights; si no hay, Destacado (en
//     español); si no hay ninguno, los titulares de siempre. El español nunca muestra Highlights.
//   - No son un grupo más: no se listan con los demás ni cuentan en el «y N más». Un grupo curado
//     sin viñetas es como si no existiera, y el encabezado no distingue mayúsculas.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	repoPorDefecto = "codeabraham16/musubi"

	// Marcadores del README. Los escribe el generador y los respeta tal cual: moverlos o
	// renombrarlos desconecta la guarda de `go test`.
	marcaInicio = "<!-- novedades:inicio -->"
	marcaFin    = "<!-- novedades:fin -->"

	// El bloque «sin publicar» es opcional y se escribe a mano: el generador no lo toca, sólo
	// se guarda que no quede desfasado respecto de la última versión del CHANGELOG.
	prefijoSinPublicar = "<!-- sin-publicar:inicio base="
	marcaSinPublicar   = "<!-- sin-publicar:fin -->"

	topeTitular  = 160 // runas, sin contar el `…` del corte
	topeRelease  = 6   // titulares por grupo en el cuerpo de la release
	topeReadme   = 4   // titulares por versión en el README
	negritaCorta = 15  // por debajo de esto una negrita es una etiqueta, no una frase

	versionesPorDefecto = 3 // versiones que muestra el README si no se pide otra cosa (-n)
)

// gruposEnOrden es el orden en que se muestran los grupos, y también el conjunto que cuenta:
// cualquier otro grupo del CHANGELOG (`Notes`, por ejemplo) se lee pero no aporta titulares.
var gruposEnOrden = []string{"Added", "Changed", "Fixed", "Security", "Removed"}

// Los dos grupos CURADOS. Quedan FUERA de gruposEnOrden a propósito: ese es el conjunto de los que
// se listan y se cuentan, y los curados no se listan como un grupo más ni suman al «y N más».
const (
	grupoDestacado  = "Destacado"
	grupoHighlights = "Highlights"
)

var (
	// `## [0.141.0] - 2026-09-14`. La fecha es opcional para que un encabezado sin ella no
	// haga desaparecer la versión; el enlace `(url)` tras los corchetes también se tolera.
	reEncabezadoVersion = regexp.MustCompile(`^\[(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\](?:\([^)]*\))?(?:\s+-\s+(\d{4}-\d{2}-\d{2}))?`)
	reVersionArg        = regexp.MustCompile(`^v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)$`)
	reRepo              = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	reNumero            = regexp.MustCompile(`^\d+$`)
	reEnlaceEnLinea     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

	// Una línea que arranca una lista (sangrada o no) termina el párrafo de cabeza.
	reListaAnidada = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])(?:\s|$)`)

	// La guarda internal/mcp/readme_toolcount_test.go busca «<N> herramientas» / «<N> tools» en
	// los README y los compara con el catálogo vivo. Un titular histórico («de 43 a 58
	// herramientas») no es una afirmación sobre el catálogo de hoy, pero la regexp de esa guarda
	// no lo sabe. Se le quita la coincidencia con un espacio de no separación, que se ve igual
	// y que `\s` de RE2 no reconoce.
	reConteoHerramientas = regexp.MustCompile(`(?i)(\d)\s+(herramientas|tools)`)

	reBaseSinPublicar = regexp.MustCompile(`^<!-- sin-publicar:inicio base=(\S*) -->$`)
)

// textos reúne todo lo que cambia con el idioma: rótulos, plantillas y formato de fecha.
type textos struct {
	grupo      map[string]string // nombre del grupo en el CHANGELOG -> rótulo
	masRelease string            // %d = titulares que no entraron en el tope
	masReadme  string            // %d = cuántos más; %s = enlace a la sección del CHANGELOG
	cierre     string            // %s = enlace a la sección
	sinSeccion string            // %s = enlace al CHANGELOG, sin ancla
	notaFinal  string            // línea que cierra el bloque del README, o vacío
	fecha      func(t time.Time) string

	// Lo del bloque curado. `curadas` dice QUÉ viñetas muestra este idioma (puede ser ninguna).
	destacado   string // rótulo del bloque curado en el cuerpo de la release
	todoLoDemas string // %s = enlace a la sección; cierra el bloque curado del README
	curadas     func(s seccion) []string
}

var textosEs = textos{
	grupo: map[string]string{
		"Added": "Añadido", "Changed": "Cambiado", "Fixed": "Corregido",
		"Security": "Seguridad", "Removed": "Eliminado",
	},
	masRelease: "… y %d más",
	masReadme:  "… y %d más en el [CHANGELOG](%s)",
	cierre:     "Detalle completo de esta versión en el [CHANGELOG](%s).",
	sinSeccion: "Esta versión no tiene sección propia en el [CHANGELOG](%s).",
	fecha: func(t time.Time) string {
		meses := [12]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
		return fmt.Sprintf("%d %s %d", t.Day(), meses[t.Month()-1], t.Year())
	},
	destacado:   "Destacado",
	todoLoDemas: "Y todo lo demás, en el [CHANGELOG](%s)",
	// El español nunca muestra Highlights: con un Destacado vacío sale el fallback, que está en
	// español, y no las viñetas en inglés.
	curadas: func(s seccion) []string { return s.Destacado },
}

var textosEn = textos{
	grupo: map[string]string{
		"Added": "Added", "Changed": "Changed", "Fixed": "Fixed",
		"Security": "Security", "Removed": "Removed",
	},
	masRelease: "… and %d more",
	masReadme:  "… and %d more in the [CHANGELOG](%s)",
	cierre:     "Full details for this release are in the [CHANGELOG](%s) (written in Spanish).",
	sinSeccion: "This release has no section of its own in the [CHANGELOG](%s).",
	notaFinal:  "_The changelog is written in Spanish._",
	fecha: func(t time.Time) string {
		meses := [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
		return fmt.Sprintf("%s %d, %d", meses[t.Month()-1], t.Day(), t.Year())
	},
	destacado:   "Highlights",
	todoLoDemas: "And everything else, in the [CHANGELOG](%s)",
	// Highlights si la versión los trae; si no, el Destacado en español —que es la única
	// curación que hay— antes que los titulares derivados, que son justo lo que se quiso evitar.
	curadas: func(s seccion) []string {
		if len(s.Highlights) > 0 {
			return s.Highlights
		}
		return s.Destacado
	},
}

func main() {
	os.Exit(ejecutar(os.Args[1:], os.Stdout, os.Stderr))
}

// ejecutar es main sin el os.Exit, para poder probarlo. Devuelve el código de salida: 0 bien,
// 1 si el trabajo falló (archivo ilegible, marcadores rotos, versión ausente con -estricto) y
// 2 si el uso es incorrecto.
func ejecutar(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("notas-release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	version := fs.String("version", "", "versión X.Y.Z de la que se arman las notas (acepta una «v» delante)")
	lang := fs.String("lang", "es", "idioma de los rótulos: es o en (con -readme se ignora: se escriben los dos)")
	estricto := fs.Bool("estricto", false, "con -version: si el CHANGELOG no tiene sección de esa versión, falla (exit 1) en vez de dar un texto mínimo")
	readme := fs.Bool("readme", false, "reescribe el bloque «Novedades» de README.md y README.en.md")
	n := fs.Int("n", versionesPorDefecto, "con -readme: cuántas versiones se muestran, las más nuevas primero")
	changelog := fs.String("changelog", "CHANGELOG.md", "ruta del CHANGELOG")
	repo := fs.String("repo", repoPorDefecto, "dueño/nombre del repositorio, para los enlaces")
	root := fs.String("root", ".", "con -readme: carpeta donde viven README.md y README.en.md")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "uso: go run ./deploy/cmd/notas-release (-version X.Y.Z | -readme) [opciones]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	uso := func(formato string, a ...any) int {
		fmt.Fprintf(stderr, "notas-release: "+formato+"\n", a...)
		fs.Usage()
		return 2
	}
	var numeroVersion string
	switch {
	case fs.NArg() > 0:
		return uso("argumentos de más: %q", fs.Args())
	case (*version == "") == !*readme:
		return uso("hay que pedir exactamente una cosa: -version o -readme")
	case *lang != "es" && *lang != "en":
		return uso("-lang %q no existe (es o en)", *lang)
	case !reRepo.MatchString(*repo):
		return uso("-repo %q no tiene la forma dueño/nombre", *repo)
	case *n < 1:
		return uso("-n %d: tiene que ser 1 o más", *n)
	}
	if *version != "" {
		m := reVersionArg.FindStringSubmatch(*version)
		if m == nil {
			return uso("-version %q no tiene la forma X.Y.Z", *version)
		}
		numeroVersion = m[1]
	}

	texto, err := os.ReadFile(*changelog)
	if err != nil {
		fmt.Fprintf(stderr, "notas-release: no se pudo leer el CHANGELOG: %v\n", err)
		return 1
	}
	secs, err := parsearChangelog(string(texto))
	if err != nil {
		fmt.Fprintf(stderr, "notas-release: %s: %v\n", *changelog, err)
		return 1
	}
	secs = ordenadas(secs)

	if *readme {
		if err := actualizarReadmes(*root, secs, *n, *repo, stdout); err != nil {
			fmt.Fprintf(stderr, "notas-release: %v\n", err)
			return 1
		}
		return 0
	}

	t := textosEs
	if *lang == "en" {
		t = textosEn
	}
	s := buscar(secs, numeroVersion)
	if s == nil {
		if *estricto {
			fmt.Fprintf(stderr, "notas-release: el CHANGELOG no tiene una sección para la versión %s\n", numeroVersion)
			return 1
		}
		fmt.Fprintf(stdout, t.sinSeccion+"\n", urlChangelog(*repo, numeroVersion, ""))
		return 0
	}
	fmt.Fprint(stdout, cuerpoRelease(*s, t, *repo))
	return 0
}

// seccion es una versión del CHANGELOG ya reducida a lo que se usa: sus titulares por grupo y,
// aparte, las viñetas de los dos grupos curados.
type seccion struct {
	Version   string
	Fecha     string // "2026-09-14", o vacío si el encabezado no la trae
	Ancla     string // ancla de GitHub del encabezado
	Linea     int    // línea (desde 1) del encabezado, para los mensajes
	Titulares map[string][]string

	// Viñetas de `### Destacado` y de `### Highlights`, ya limpias y con su texto COMPLETO; nil si
	// la versión no trae el grupo o lo trae sin viñetas. Van fuera de Titulares a propósito: así
	// no hay forma de que se listen o se cuenten como un grupo más.
	Destacado  []string
	Highlights []string
}

// cerca es una cerca de código abierta: se cierra con la misma marca y al menos el mismo largo.
type cerca struct {
	car   byte // '`' o '~'; 0 si no hay cerca abierta
	largo int
	linea int
}

// parsearChangelog lee el CHANGELOG y devuelve sus versiones EN EL ORDEN DEL ARCHIVO.
//
// Lo que sabe del formato (keep-a-changelog, sin adornos propios):
//   - `## [X.Y.Z] - fecha` abre una versión. Cualquier otro `## ` —`[Unreleased]`, por ejemplo—
//     cierra la anterior y NO abre nada: lo que cuelga de ahí no es de ninguna versión.
//   - `### Grupo` abre un grupo; sólo cuentan los de gruposEnOrden y los dos curados (`Destacado`
//     y `Highlights`), cuyas viñetas se guardan COMPLETAS en vez de reducirse a un titular.
//   - `- ` en la columna 0 es una viñeta de primer nivel. Las sub-viñetas y los párrafos que
//     siguen a una línea en blanco son detalle, no titular (ni parte de la viñeta curada).
//   - Lo que está dentro de una cerca de código es literal: ahí puede haber un `## [9.9.9]`,
//     un `### Added` o un `- ` en la columna 0 de mentira, y no se leen como estructura.
//
// Falla en lugar de adivinar cuando el archivo no se puede leer con certeza: una cerca que no
// cierra se tragaría en silencio todas las versiones que siguen, y una versión repetida
// duplicaría su entrada en el README.
func parsearChangelog(texto string) ([]seccion, error) {
	texto = strings.ReplaceAll(texto, "\r\n", "\n")
	var (
		secs   []seccion
		actual = -1 // índice en secs de la versión abierta; -1 fuera de cualquier versión
		grupo  string
		cabeza []string // párrafo de cabeza de la viñeta abierta; nil si no hay una abierta
		cerr   cerca
	)
	registrar := func() {
		if cabeza != nil && actual >= 0 && grupo != "" {
			s := &secs[actual]
			texto := strings.Join(cabeza, " ")
			switch grupo {
			case grupoDestacado:
				if c := completa(texto); c != "" {
					s.Destacado = append(s.Destacado, c)
				}
			case grupoHighlights:
				if c := completa(texto); c != "" {
					s.Highlights = append(s.Highlights, c)
				}
			default:
				if t := titular(texto); t != "" {
					s.Titulares[grupo] = append(s.Titulares[grupo], t)
				}
			}
		}
		cabeza = nil
	}

	for i, linea := range strings.Split(texto, "\n") {
		num := i + 1
		if cerr.car != 0 {
			if cierraCerca(linea, cerr) {
				cerr = cerca{}
			}
			continue
		}
		if c, ok := abreCerca(linea, num); ok {
			registrar() // una cerca interrumpe el párrafo de cabeza
			cerr = c
			continue
		}
		if nivel, titulo, ok := encabezadoATX(linea); ok {
			registrar()
			switch nivel {
			case 1:
				actual, grupo = -1, ""
			case 2:
				actual, grupo = -1, ""
				if m := reEncabezadoVersion.FindStringSubmatch(titulo); m != nil {
					for _, previa := range secs {
						if previa.Version == m[1] {
							return nil, fmt.Errorf("la versión %s aparece dos veces (líneas %d y %d)", m[1], previa.Linea, num)
						}
					}
					secs = append(secs, seccion{
						Version: m[1], Fecha: m[2], Ancla: anclaGitHub(titulo), Linea: num,
						Titulares: map[string][]string{},
					})
					actual = len(secs) - 1
				}
			case 3:
				grupo = grupoCanonico(titulo)
			}
			continue
		}
		switch {
		case strings.HasPrefix(linea, "- "):
			registrar()
			cabeza = []string{strings.TrimSpace(linea[2:])}
		case strings.TrimSpace(linea) == "":
			registrar()
		case cabeza != nil && reListaAnidada.MatchString(linea):
			registrar()
		case cabeza != nil:
			cabeza = append(cabeza, strings.TrimSpace(linea))
		}
	}
	registrar()
	if cerr.car != 0 {
		return nil, fmt.Errorf("la cerca de código abierta en la línea %d no se cierra: se tragaría todo lo que sigue", cerr.linea)
	}
	return secs, nil
}

// abreCerca reconoce el principio de una cerca de código: tres o más backticks o tildes, con la
// sangría que sea. Una línea con backticks que vuelven a aparecer después (```x```) es código en
// línea, no una cerca.
func abreCerca(linea string, num int) (cerca, bool) {
	t := strings.TrimLeft(linea, " \t")
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return cerca{}, false
	}
	largo := 0
	for largo < len(t) && t[largo] == t[0] {
		largo++
	}
	if largo < 3 || (t[0] == '`' && strings.Contains(t[largo:], "`")) {
		return cerca{}, false
	}
	return cerca{car: t[0], largo: largo, linea: num}, true
}

func cierraCerca(linea string, c cerca) bool {
	t := strings.TrimLeft(linea, " \t")
	largo := 0
	for largo < len(t) && t[largo] == c.car {
		largo++
	}
	return largo >= c.largo && strings.TrimSpace(t[largo:]) == ""
}

// encabezadoATX reconoce `# `..`###### ` en la columna 0 y devuelve el nivel y el texto.
func encabezadoATX(linea string) (nivel int, titulo string, ok bool) {
	for nivel < len(linea) && linea[nivel] == '#' {
		nivel++
	}
	if nivel == 0 || nivel > 6 {
		return 0, "", false
	}
	resto := linea[nivel:]
	if resto != "" && resto[0] != ' ' && resto[0] != '\t' {
		return 0, "", false
	}
	return nivel, strings.TrimSpace(resto), true
}

// grupoCanonico devuelve el nombre del grupo si cuenta (sin distinguir mayúsculas) y "" si no:
// los cinco de gruposEnOrden, que aportan titulares, y los dos curados, Destacado y Highlights,
// que aportan viñetas completas. `### destacado` vale, y en cualquier posición de la versión.
func grupoCanonico(titulo string) string {
	for _, g := range gruposEnOrden {
		if strings.EqualFold(titulo, g) {
			return g
		}
	}
	for _, g := range []string{grupoDestacado, grupoHighlights} {
		if strings.EqualFold(titulo, g) {
			return g
		}
	}
	return ""
}

// anclaGitHub calcula el ancla que GitHub le da a un encabezado: se parte del TEXTO que se ve
// (`[t](url)` es `t`), en minúsculas, se descarta todo lo que no sea letra, marca, número, guion
// o conector (el `_`) y los espacios pasan a guiones. Por eso `## [0.141.0] - 2026-09-14` es
// `#01410---2026-09-14`: los puntos y los corchetes desaparecen y los tres espacios de « - »
// quedan como tres guiones.
func anclaGitHub(titulo string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(reEnlaceEnLinea.ReplaceAllString(titulo, "$1")) {
		switch {
		case unicode.IsLetter(c), unicode.IsMark(c), unicode.IsNumber(c), unicode.Is(unicode.Pc, c), c == '-':
			b.WriteRune(c)
		case c == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

// ordenadas devuelve las versiones de la más nueva a la más vieja según semver, sin tocar la
// entrada. El orden del archivo no se da por bueno: una versión pegada fuera de lugar no puede
// aparecer en el README como si fuera la última.
func ordenadas(secs []seccion) []seccion {
	out := append([]seccion(nil), secs...)
	sort.SliceStable(out, func(i, j int) bool {
		return compararVersiones(out[i].Version, out[j].Version) > 0
	})
	return out
}

func buscar(secs []seccion, version string) *seccion {
	for i := range secs {
		if secs[i].Version == version {
			return &secs[i]
		}
	}
	return nil
}

// compararVersiones sigue semver: los tres números se comparan como números (0.10.0 > 0.9.0, que
// como texto sería al revés) y una versión con pre-release va por debajo de la misma sin él.
func compararVersiones(a, b string) int {
	na, prea := partirVersion(a)
	nb, preb := partirVersion(b)
	for i := range na {
		if c := compararNumeros(na[i], nb[i]); c != 0 {
			return c
		}
	}
	switch {
	case prea == preb:
		return 0
	case prea == "":
		return 1
	case preb == "":
		return -1
	}
	ia, ib := strings.Split(prea, "."), strings.Split(preb, ".")
	for i := 0; i < len(ia) && i < len(ib); i++ {
		x, y := ia[i], ib[i]
		numX, numY := reNumero.MatchString(x), reNumero.MatchString(y)
		var c int
		switch {
		case numX && numY:
			c = compararNumeros(x, y)
		case numX:
			c = -1 // un identificador numérico va antes que uno alfanumérico
		case numY:
			c = 1
		default:
			c = strings.Compare(x, y)
		}
		if c != 0 {
			return c
		}
	}
	return len(ia) - len(ib)
}

func partirVersion(v string) (nums [3]string, pre string) {
	if i := strings.IndexByte(v, '-'); i >= 0 {
		v, pre = v[:i], v[i+1:]
	}
	for i, p := range strings.SplitN(v, ".", 3) {
		nums[i] = p
	}
	return nums, pre
}

// compararNumeros compara dos números decimales en texto sin pasarlos a int: no hay tope de
// tamaño y los ceros a la izquierda no cuentan.
func compararNumeros(a, b string) int {
	a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

// ---- titulares ----------------------------------------------------------------------------

// titular reduce el párrafo de cabeza de una viñeta a una línea. Las reglas están en la
// documentación del paquete; devuelve "" si no queda nada que mostrar.
func titular(cabeza string) string {
	s := sinPrefijo(cabeza)
	if s == "" {
		return ""
	}
	r := []rune(s)
	cod, _ := escanearCodigo(r)

	var texto []rune
	if strings.HasPrefix(s, "**") {
		if j := cierreNegrita(r, cod); j >= 0 {
			negrita := strings.TrimSpace(string(r[2:j]))
			if utf8.RuneCountInString(negrita) < negritaCorta || strings.HasSuffix(negrita, ":") {
				texto = append(append(texto, r[2:j]...), r[j+2:finDeOracion(r, cod, j+2)]...)
			} else {
				texto = r[2:j]
			}
		} else {
			texto = r[2:finDeOracion(r, cod, 2)]
		}
	} else {
		texto = r[:finDeOracion(r, cod, 0)]
	}
	return limpiar(texto)
}

// sinPrefijo descarta lo que precede a la primera runa con la que puede empezar un titular: una
// letra, un número, `*` o un backtick —el emoji de una viñeta («🔴 **...**») es lo típico—, y
// también los signos que abren un par, para no dejar sólo el cierre en el titular.
func sinPrefijo(s string) string {
	for i, c := range s {
		if unicode.IsLetter(c) || unicode.IsNumber(c) || strings.ContainsRune("*`([{\"'«“‘¿¡", c) {
			return strings.TrimSpace(s[i:])
		}
	}
	return ""
}

// escanearCodigo marca qué runas están dentro de un code span (con sus delimitadores) y cuáles
// son backticks sueltos, leyendo los backticks como CommonMark: una tira de N backticks abre un
// code span que cierra la siguiente tira de EXACTAMENTE N. Una tira sin pareja no es código: es
// texto con backticks, y queda marcada en `suelta`.
func escanearCodigo(r []rune) (enCodigo, suelta []bool) {
	n := len(r)
	enCodigo, suelta = make([]bool, n), make([]bool, n)
	for i := 0; i < n; {
		if r[i] != '`' {
			i++
			continue
		}
		j := i
		for j < n && r[j] == '`' {
			j++
		}
		cierre := -1
		for k := j; k < n; {
			if r[k] != '`' {
				k++
				continue
			}
			m := k
			for m < n && r[m] == '`' {
				m++
			}
			if m-k == j-i {
				cierre = m
				break
			}
			k = m
		}
		if cierre < 0 {
			for x := i; x < j; x++ {
				suelta[x] = true
			}
			i = j
			continue
		}
		for x := i; x < cierre; x++ {
			enCodigo[x] = true
		}
		i = cierre
	}
	return enCodigo, suelta
}

// cierreNegrita devuelve el índice del `**` que cierra la negrita con la que empieza r, ignorando
// los que caen dentro de un code span; -1 si no cierra.
func cierreNegrita(r []rune, cod []bool) int {
	for i := 2; i+1 < len(r); i++ {
		if r[i] == '*' && r[i+1] == '*' && !cod[i] {
			return i
		}
	}
	return -1
}

// abreviaturas que terminan en punto y NO terminan la oración: «p. ej.», «vs.», «i.e.».
var abreviaturas = map[string]bool{"ej": true, "vs": true, "aprox": true, "i.e": true, "e.g": true}

// finDeOracion devuelve el índice del punto que cierra la primera oración de r a partir de
// `desde` —un `. ` fuera de un code span que no sea el de una abreviatura—, o len(r) si no hay.
func finDeOracion(r []rune, cod []bool, desde int) int {
	for i := desde; i+1 < len(r); i++ {
		if r[i] != '.' || r[i+1] != ' ' || cod[i] || esAbreviatura(r, i) {
			continue
		}
		return i
	}
	return len(r)
}

func esAbreviatura(r []rune, punto int) bool {
	ini := punto
	for ini > 0 && (unicode.IsLetter(r[ini-1]) || r[ini-1] == '.') {
		ini--
	}
	palabra := strings.ToLower(string(r[ini:punto]))
	if abreviaturas[palabra] {
		return true
	}
	// «p. ej.»: el punto de la «p» también es de abreviatura, pero «p» sola no lo es.
	return palabra == "p" && strings.HasPrefix(string(r[punto+1:]), " ej.")
}

// nbsp es el espacio de no separación. Se ve igual que un espacio, pero `\s` de RE2 no lo reconoce.
const nbsp = string(rune(0xA0))

// limpiar deja el texto del titular listo para el README o la release: sin marcas de énfasis ni
// enlaces, con los `<` escapados, sin signos sobrando al final, dentro del tope y con los
// backticks balanceados. Si no puede cumplir todo eso devuelve "" antes que un titular que rompa
// alguna de esas promesas.
func limpiar(texto []rune) string {
	s := normalizar(texto, false)
	// Lo normal es que se corte una sola vez y que normalizar no tenga nada más que hacer: el
	// corte ya cae antes de cualquier backtick abierto. El caso que da otra vuelta es un titular
	// que EMPIEZA con un code span más largo que el tope: no hay nada anterior que conservar, el
	// corte lo deja con un backtick sin pareja y normalizar se lo quita, y al quitárselo lo que
	// estaba protegido pasa a ser prosa y puede crecer (un `<` pasa a `&lt;`).
	for i := 0; i < 8; i++ {
		c := normalizar([]rune(recortar(s)), false)
		if utf8.RuneCountInString(c) <= topeTitular+1 {
			return neutralizarConteos(c)
		}
		s = c
	}
	return ""
}

// neutralizarConteos cambia el espacio de «N herramientas» / «N tools» por uno de no separación,
// para que la guarda de los README no lea un número histórico como una afirmación sobre el
// catálogo de hoy. Va SIEMPRE al final de la limpieza: una vez sacados los `**` y los enlaces, un
// «**80** tools» ya es un «80 tools».
func neutralizarConteos(s string) string {
	return reConteoHerramientas.ReplaceAllString(s, "$1"+nbsp+"$2")
}

// completa deja el texto de UNA viñeta curada (`### Destacado` / `### Highlights`) listo para el
// README o la release. No es un titular: no elige una oración, no corta a 160 runas y conserva el
// signo final —es una oración completa que escribió quien publica—. Pasa por el mismo núcleo de
// limpieza que el titular (sin `**` ni enlaces, `<` escapado, backticks balanceados, espacios
// compactados) y por la misma neutralización de conteos. Devuelve "" si no queda nada que mostrar.
//
// No usa `limpiar` a propósito: `limpiar` es la limpieza MÁS el tope del titular, y recortar una
// viñeta curada la dejaría sin el texto completo que se pidió.
func completa(cabeza string) string {
	return neutralizarConteos(normalizar([]rune(cabeza), true))
}

// normalizar aplica a un punto fijo todo lo que no depende del tope: quitar `**` y enlaces,
// quitar los backticks sin pareja, escapar el `<` y compactar. Va a un punto fijo porque cada paso
// puede dejar servido el siguiente: al sacar el `**` de «`a`**`b`» dos backticks quedan pegados y
// forman una tira nueva, y al sacar un backtick suelto de «*`*» queda un `**`. Cada vuelta que
// cambia algo acorta el texto (o cambia un `<` por un `&lt;`, que ya no vuelve a cambiar), así que
// converge; el tope de vueltas sólo está para que un error acá no cuelgue el workflow de release.
// `conservarFinal` deja intactos los signos del final: el titular los saca, la viñeta curada no.
func normalizar(r []rune, conservarFinal bool) string {
	for i := 0; i < 32; i++ {
		cod, _ := escanearCodigo(r)
		sig := sinSueltas(sinMarcas(r, cod))
		cod, _ = escanearCodigo(sig)
		sig = []rune(compactar(string(escaparMenor(sig, cod)), conservarFinal))
		if string(sig) == string(r) {
			break
		}
		r = sig
	}
	return string(r)
}

// compactar colapsa los espacios y, salvo que se pida conservarlos, saca los signos que sobran al
// final (`. : , ;`).
func compactar(s string, conservarFinal bool) string {
	s = strings.Join(strings.Fields(s), " ")
	if conservarFinal {
		return s
	}
	return sinSignosFinales(s)
}

// sinSignosFinales saca del final de s los espacios y los signos `. : , ;`, salvo el `;` que
// cierra un `&lt;`: ese punto y coma es parte del `<` escapado y no un signo que sobra. Sin esta
// excepción un titular que termina en `<` quedaba con «&lt» a secas.
func sinSignosFinales(s string) string {
	for s != "" && strings.IndexByte(" .:,;", s[len(s)-1]) >= 0 && !strings.HasSuffix(s, "&lt;") {
		s = s[:len(s)-1]
	}
	return s
}

// sinMarcas quita el `**` y reemplaza `[texto](url)` por `texto`. Nada de eso se aplica dentro de
// un code span: ahí el texto es literal y debe llegar igual.
func sinMarcas(r []rune, cod []bool) []rune {
	out := make([]rune, 0, len(r))
	for i := 0; i < len(r); {
		switch {
		case cod[i]:
			out = append(out, r[i])
			i++
		case r[i] == '*' && i+1 < len(r) && r[i+1] == '*' && !cod[i+1]:
			i += 2
		case r[i] == '[':
			if cierra, despues := enlaceEn(r, cod, i); cierra > 0 {
				out = append(out, sinMarcas(r[i+1:cierra], cod[i+1:cierra])...)
				i = despues
				continue
			}
			out = append(out, r[i])
			i++
		default:
			out = append(out, r[i])
			i++
		}
	}
	return out
}

// escaparMenor pasa a `&lt;` todo `<` que no esté en un code span: en el README o en la release
// un `<tag>` suelto abriría una etiqueta HTML y se tragaría el resto de la línea.
func escaparMenor(r []rune, cod []bool) []rune {
	out := make([]rune, 0, len(r))
	for i, c := range r {
		if c == '<' && !cod[i] {
			out = append(out, '&', 'l', 't', ';')
			continue
		}
		out = append(out, c)
	}
	return out
}

// enlaceEn reconoce un enlace `[texto](url)` que abre en r[abre] y devuelve el índice del `]` y
// el siguiente al `)`; (-1, -1) si lo que sigue no es un enlace. El texto puede llevar code spans
// con corchetes adentro.
func enlaceEn(r []rune, cod []bool, abre int) (cierra, despues int) {
	prof := 0
	for j := abre; j < len(r); j++ {
		if cod[j] {
			continue
		}
		switch r[j] {
		case '[':
			prof++
		case ']':
			prof--
			if prof > 0 {
				continue
			}
			if j+1 >= len(r) || r[j+1] != '(' {
				return -1, -1
			}
			for k := j + 2; k < len(r); k++ {
				if r[k] == ')' {
					return j, k + 1
				}
				if unicode.IsSpace(r[k]) {
					return -1, -1
				}
			}
			return -1, -1
		}
	}
	return -1, -1
}

// recortar aplica el tope: corta en el límite de palabra anterior y agrega `…`. Si el corte
// deja un backtick abierto, corta antes de él, para no dejar un fragmento de código a medias.
func recortar(s string) string {
	r := []rune(s)
	if len(r) <= topeTitular {
		return s
	}
	corte := topeTitular
	if r[topeTitular] != ' ' {
		for k := topeTitular - 1; k > 0; k-- {
			if r[k] == ' ' {
				corte = k
				break
			}
		}
	}
	cand := []rune(sinSignosFinales(string(r[:corte])))
	if _, suelta := escanearCodigo(cand); primero(suelta) >= 0 {
		// Si el titular arranca con un code span más largo que el tope no queda nada antes del
		// backtick: ahí se prefiere el texto sin los backticks sueltos a un «…» solo.
		if previo := sinSignosFinales(string(cand[:primero(suelta)])); previo != "" {
			cand = []rune(previo)
		}
	}
	return string(cand) + "…"
}

// sinSueltas quita los backticks que no tienen pareja. Un titular llega acá ya balanceado casi
// siempre; el caso que queda es un backtick suelto en la prosa, que en el README abriría un code
// span fantasma en la línea.
func sinSueltas(r []rune) []rune {
	_, suelta := escanearCodigo(r)
	if primero(suelta) < 0 {
		return r
	}
	out := make([]rune, 0, len(r))
	for i, c := range r {
		if !suelta[i] {
			out = append(out, c)
		}
	}
	return out
}

func primero(marcas []bool) int {
	for i, m := range marcas {
		if m {
			return i
		}
	}
	return -1
}

// ---- salida ---------------------------------------------------------------------------------

func urlChangelog(repo, version, ancla string) string {
	u := "https://github.com/" + repo + "/blob/v" + version + "/CHANGELOG.md"
	if ancla != "" {
		u += "#" + ancla
	}
	return u
}

func urlRelease(repo, version string) string {
	return "https://github.com/" + repo + "/releases/tag/v" + version
}

// enOrden junta los titulares de una versión recorriendo los grupos en el orden de gruposEnOrden.
func enOrden(s seccion) []string {
	var todos []string
	for _, g := range gruposEnOrden {
		todos = append(todos, s.Titulares[g]...)
	}
	return todos
}

// cuerpoRelease arma el texto de la release: primero, si la versión lo trae, el bloque curado con
// hasta topeRelease viñetas completas; después, por cada grupo con titulares, hasta topeRelease, y
// al final el enlace a la sección completa del CHANGELOG EN EL TAG de la versión (el ancla sólo
// existe ahí si el archivo era ese). Lo curado no se repite como grupo ni suma a ningún «y N más».
func cuerpoRelease(s seccion, t textos, repo string) string {
	var b strings.Builder
	if curadas := t.curadas(s); len(curadas) > 0 {
		fmt.Fprintf(&b, "### %s\n\n", t.destacado)
		for i, x := range curadas {
			if i == topeRelease {
				break
			}
			fmt.Fprintf(&b, "- %s\n", x)
		}
		b.WriteString("\n")
	}
	for _, g := range gruposEnOrden {
		h := s.Titulares[g]
		if len(h) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", t.grupo[g])
		for i, x := range h {
			if i == topeRelease {
				fmt.Fprintf(&b, "- "+t.masRelease+"\n", len(h)-topeRelease)
				break
			}
			fmt.Fprintf(&b, "- %s\n", x)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, t.cierre+"\n", urlChangelog(repo, s.Version, s.Ancla))
	return b.String()
}

// bloqueReadme arma las líneas de «Novedades»: las `n` versiones más nuevas con hasta topeReadme
// viñetas cada una. Si la versión trae un bloque curado en el idioma de `t`, son sus viñetas
// completas y el cierre es «todo lo demás, en el CHANGELOG»; si no, son los titulares derivados,
// recorriendo los grupos en orden, con su «y N más».
func bloqueReadme(secs []seccion, n int, t textos, repo string) []string {
	if n > len(secs) {
		n = len(secs)
	}
	var out []string
	for i, s := range secs[:n] {
		if i > 0 {
			out = append(out, "")
		}
		cabecera := fmt.Sprintf("**[v%s](%s)**", s.Version, urlRelease(repo, s.Version))
		if f := fechaCorta(s.Fecha, t); f != "" {
			cabecera += " · " + f
		}
		out = append(out, cabecera)
		if curadas := t.curadas(s); len(curadas) > 0 {
			out = append(out, "")
			for j, x := range curadas {
				if j == topeReadme {
					break
				}
				out = append(out, "- "+x)
			}
			out = append(out, "", fmt.Sprintf(t.todoLoDemas, urlChangelog(repo, s.Version, s.Ancla)))
			continue
		}
		todos := enOrden(s)
		if len(todos) == 0 {
			continue
		}
		out = append(out, "")
		for j, x := range todos {
			if j == topeReadme {
				break
			}
			out = append(out, "- "+x)
		}
		if len(todos) > topeReadme {
			out = append(out, "", fmt.Sprintf(t.masReadme, len(todos)-topeReadme, urlChangelog(repo, s.Version, s.Ancla)))
		}
	}
	if t.notaFinal != "" {
		out = append(out, "", t.notaFinal)
	}
	return out
}

// fechaCorta formatea la fecha del encabezado; si no se puede leer devuelve el texto tal cual
// antes que perderla.
func fechaCorta(fecha string, t textos) string {
	if fecha == "" {
		return ""
	}
	d, err := time.Parse("2006-01-02", fecha)
	if err != nil {
		return fecha
	}
	return t.fecha(d)
}

// ---- README ---------------------------------------------------------------------------------

// ubicarMarcadores busca los marcadores como LÍNEAS ENTERAS y devuelve dónde empieza lo que hay
// entre ellos: `desde` es el byte siguiente al salto de línea del marcador de inicio y `hasta` el
// primero de la línea del marcador de fin. También devuelve el salto de línea que usa el archivo.
// Falla si falta alguno, si se repite o si el fin viene antes que el inicio: cualquiera de esos
// casos haría que el generador escribiera en el lugar equivocado.
func ubicarMarcadores(readme string) (desde, hasta int, eol string, err error) {
	type marca struct{ linea, inicio, fin int }
	var inicios, fines []marca
	eol = "\n"
	for off, num := 0, 1; off < len(readme); num++ {
		fin := len(readme)
		if k := strings.IndexByte(readme[off:], '\n'); k >= 0 {
			fin = off + k + 1
		}
		switch strings.TrimSpace(readme[off:fin]) {
		case marcaInicio:
			inicios = append(inicios, marca{num, off, fin})
		case marcaFin:
			fines = append(fines, marca{num, off, fin})
		}
		off = fin
	}
	for _, m := range []struct {
		nombre string
		marcas []marca
	}{{marcaInicio, inicios}, {marcaFin, fines}} {
		switch len(m.marcas) {
		case 0:
			return 0, 0, "", fmt.Errorf("falta el marcador %s", m.nombre)
		case 1:
		default:
			lineas := make([]string, len(m.marcas))
			for i, x := range m.marcas {
				lineas[i] = fmt.Sprint(x.linea)
			}
			return 0, 0, "", fmt.Errorf("el marcador %s aparece %d veces (líneas %s)", m.nombre, len(m.marcas), strings.Join(lineas, ", "))
		}
	}
	if fines[0].inicio < inicios[0].fin {
		return 0, 0, "", fmt.Errorf("el marcador %s (línea %d) viene antes que %s (línea %d)", marcaFin, fines[0].linea, marcaInicio, inicios[0].linea)
	}
	if strings.HasSuffix(readme[inicios[0].inicio:inicios[0].fin], "\r\n") {
		eol = "\r\n"
	}
	return inicios[0].fin, fines[0].inicio, eol, nil
}

// reemplazarBloque devuelve el README con lo que hay entre los marcadores sustituido por el
// bloque —una línea en blanco, el bloque, una línea en blanco—. Todo lo demás queda byte por
// byte igual, y los saltos de línea nuevos usan los del archivo (LF o CRLF).
func reemplazarBloque(readme string, bloque []string) (string, error) {
	desde, hasta, eol, err := ubicarMarcadores(readme)
	if err != nil {
		return "", err
	}
	// El bloque «sin publicar» se escribe a mano y el generador no lo toca. Si alguien lo pone
	// ADENTRO de los marcadores, reescribir lo borraría sin avisar: se prefiere frenar.
	if strings.Contains(readme[desde:hasta], "<!-- sin-publicar:") {
		return "", errors.New("el bloque «sin publicar» está ENTRE los marcadores de novedades y el generador lo borraría: " +
			"ponelo afuera, antes del marcador de inicio o después del de fin")
	}
	var b strings.Builder
	b.WriteString(readme[:desde])
	b.WriteString(eol)
	for _, l := range bloque {
		b.WriteString(l)
		b.WriteString(eol)
	}
	b.WriteString(eol)
	b.WriteString(readme[hasta:])
	return b.String(), nil
}

// actualizarReadmes reescribe README.md (es) y README.en.md (en) bajo `root`. Calcula los dos
// resultados ANTES de escribir ninguno: si uno de los dos tiene los marcadores rotos no se toca
// ninguno, y el que ya estaba al día no se reescribe.
func actualizarReadmes(root string, secs []seccion, n int, repo string, stdout io.Writer) error {
	if len(secs) == 0 {
		return errors.New("el CHANGELOG no tiene ninguna versión con la forma `## [X.Y.Z]`")
	}
	type destino struct {
		ruta, actual, nuevo string
		perm                os.FileMode
	}
	var destinos []destino
	for _, d := range []struct {
		nombre string
		t      textos
	}{{"README.md", textosEs}, {"README.en.md", textosEn}} {
		ruta := filepath.Join(root, d.nombre)
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return fmt.Errorf("no se pudo leer %s: %w", ruta, err)
		}
		info, err := os.Stat(ruta)
		if err != nil {
			return fmt.Errorf("no se pudo leer %s: %w", ruta, err)
		}
		nuevo, err := reemplazarBloque(string(datos), bloqueReadme(secs, n, d.t, repo))
		if err != nil {
			return fmt.Errorf("%s: %w", ruta, err)
		}
		destinos = append(destinos, destino{ruta, string(datos), nuevo, info.Mode().Perm()})
	}
	for _, d := range destinos {
		if d.nuevo == d.actual {
			fmt.Fprintf(stdout, "%s: sin cambios\n", d.ruta)
			continue
		}
		if err := os.WriteFile(d.ruta, []byte(d.nuevo), d.perm); err != nil {
			return fmt.Errorf("no se pudo escribir %s: %w", d.ruta, err)
		}
		fmt.Fprintf(stdout, "%s: actualizado\n", d.ruta)
	}
	return nil
}

// comprobarSinPublicar guarda el bloque OPCIONAL «sin publicar» de un README. Ese bloque lo
// escribe una persona: es una foto de lo que hay en `main` más allá de la última versión, y la
// foto dice sobre qué versión se sacó con `base=X.Y.Z`. Cuando el CHANGELOG suma una versión
// nueva la foto queda vieja, y esta función lo canta. Sin bloque no hay nada que guardar.
func comprobarSinPublicar(readme, nombre, ultima string) error {
	var inicios, fines []int
	var base string
	for i, linea := range strings.Split(strings.ReplaceAll(readme, "\r\n", "\n"), "\n") {
		linea = strings.TrimSpace(linea)
		if m := reBaseSinPublicar.FindStringSubmatch(linea); m != nil {
			inicios = append(inicios, i+1)
			base = m[1]
		} else if strings.HasPrefix(linea, prefijoSinPublicar) {
			return fmt.Errorf("%s:%d: el marcador de inicio de «sin publicar» tiene que ser exactamente `%sX.Y.Z -->`", nombre, i+1, prefijoSinPublicar)
		}
		if linea == marcaSinPublicar {
			fines = append(fines, i+1)
		}
	}
	if len(inicios) == 0 {
		return nil
	}
	if len(inicios) > 1 || len(fines) != 1 || fines[0] < inicios[0] {
		return fmt.Errorf("%s: el bloque «sin publicar» tiene que abrir una vez con `%sX.Y.Z -->` y cerrar una vez con `%s`, en ese orden", nombre, prefijoSinPublicar, marcaSinPublicar)
	}
	if !reVersionArg.MatchString(base) || strings.HasPrefix(base, "v") {
		return fmt.Errorf("%s:%d: base=%q no tiene la forma X.Y.Z", nombre, inicios[0], base)
	}
	if base != ultima {
		return fmt.Errorf("%s:%d: el bloque «sin publicar» es una foto de `main` que quedó vieja: dice base=%s y la última versión del CHANGELOG es %s. "+
			"Actualizalo a mano con lo que quedó fuera de la versión nueva y cambiá `base=` a %s, o borrá el bloque entero",
			nombre, inicios[0], base, ultima, ultima)
	}
	return nil
}
