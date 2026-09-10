package mcp

// Guarda del track «Control de flota»: NINGÚN cabo suelto sin registro, y el registro no miente
// sobre sí mismo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO ES UNA PRUEBA Y NO UNA COSTUMBRE
//
// `specs/control-de-flota/ABIERTO.md` dice, en su propia sección «Cómo se usa este archivo»:
// «Un `## Lo que queda fuera` en un spec que no aparezca acá es un cabo suelto de verdad».
//
// Esa regla se cumplió a mano durante todo el track, y a mano se rompió: un barrido encontró
// NUEVE ítems declarados fuera de alcance que ya se habían hecho en slices posteriores —specs
// afirmando que algo «no está» cuando estaba— y dos que nunca tuvieron número de registro.
// Un spec que miente sobre lo que falta es peor que un pendiente: quien lo lee aprende algo falso
// y decide con eso.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SEGUNDA RONDA (2026-09-10): ONCE SABOTAJES QUEDARON EN VERDE, Y CASI TODOS ERAN TRIVIALES
//
// La ronda anterior arregló el DEFECTO. Un barrido adversario sobre las GUARDAS encontró que
// estaban ancladas en la FORMA LITERAL del bug viejo, y las evadió sin esfuerzo:
//
//  1. El `ABIERTO.md` entero de `f9be15a` —las mismas 22 filas cerradas adentro de la tabla 1,
//     byte por byte— con UN espacio de padding de más en la columna del número pasaba en verde:
//     los dos regex de fila exigían exactamente un espacio detrás de la barra. Una tabla de
//     Markdown se PARSEA (partir por la barra, limpiar la celda, trabajar con celdas); adivinar
//     su forma con un regex es lo que hacía que un cambio cosmético apagara la guarda entera.
//  2. El veredicto en la SEGUNDA oración escapaba: la guarda cortaba en el primer punto, y 18 de
//     22 filas se salvaban escribiendo «falta el redespliegue. Por lo demás quedó cerrado».
//  3. El vocabulario era una lista de tres palabras sacadas de las celdas que YA estaban:
//     «hecho», «resuelto», «completado», «listo» y «ya salió» pasaban en verde. **Una lista de
//     palabras malas siempre le falta la próxima**, así que la guarda dejó de preguntar por la
//     palabra mala: la columna que decide tiene que EMPEZAR con una de las cuatro formas de
//     asignación que este registro usa (un slice, `sin asignar`, `gio`, `acción del operador`).
//     Es una lista de las formas BUENAS, y por eso una palabra de cierre nueva —cualquiera—
//     falla en vez de pasar.
//  4. El piso del barrido de specs era GLOBAL: con 14 secciones y 45 ítems (de 15 y 47) la
//     prueba pasaba habiendo perdido una sección entera por un renombre. El piso ahora es POR
//     ARCHIVO, y además una sección renombrada se caza por lo que dice su título.
//  5. El glob `specs/flota-*/*.md` no recursaba: un `spec.md` en un subdirectorio del track era
//     invisible. Ahora se camina el árbol.
//  6. `tieneCasa` daba por registrado un ítem por las palabras «despliegue», «por diseño»,
//     «Cero dependencias» y `HECH` **como substring**, que son prosa corriente en estos specs.
//     Ahora cada marca está anclada en la forma que DECIDE, no en la palabra suelta.
//  7. Sólo se reconocía la viñeta `- **`: el asterisco, el ítem sin negrita y el indentado no
//     sólo escapaban a la revisión, ni siquiera contaban para el piso. Y la marca de un ítem
//     puede venir en su segunda línea, así que ahora se revisa el ÍTEM entero y no la línea.
//  8. La tabla 2 no tenía ninguna guarda: ni contra una fila que se declara cerrada (la hermana
//     de la regla 1) ni contra una fila sin condición de revisión (la regla 3).
//  9. Los números de esquema y los nombres de prueba citados en el registro no se cruzaban con
//     nada. Así se pudrieron A99, A102 y A116: apuntaban a una migración que ya no era.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// TERCERA RONDA (2026-09-10): LO QUE QUEDÓ DECLARADO Y NO CERRADO, Y UN HUECO DEL MAPA
//
// La segunda ronda dejó tres cosas dichas en un comentario y no arregladas. Las tres se midieron
// otra vez, en verde, y se cierran acá:
//
//  10. LA TABLA 2 SEGUÍA CON UNA LISTA DE PALABRAS. `abreConCierre` era «cerrado, hecho, resuelto,
//      completado, listo, terminado…»: nueve palabras sacadas de las celdas que ya estaban, que es
//      el defecto que la tabla 1 arregló en la ronda 2 y que acá quedó puesto. «Finiquitado en
//      S12.» y «Ya se implementó y quedó andando.» pasaban en verde. Ahora se pregunta por la
//      MORFOLOGÍA —participio perfectivo o pretérito perfecto simple—, con la familia de la
//      deliberación («decidido», «descartado», «se midió») excusada porque en ESA tabla es el
//      contenido correcto. Ver el bloque de `hechoConsumadoAlAbrir`.
//  11. `tieneCasa` DABA POR REGISTRADO UN ÍTEM POR MENCIONAR UN NÚMERO. La marca principal era
//      «un tramo en negrita que contenga algo con forma de número de registro», así que
//      `**La API expone A1 y B2 como campos JSON.**` tenía casa sin estar anotado en ningún lado.
//      Ahora la cita tiene que ser el TOKEN que el track usa para citar el registro —el número
//      ocupando todo el tramo en negrita, `(**A42**)`, `(**A17 → S7c**)`, `**A1/A2/A3**`—, no un
//      número suelto adentro de una oración. Medido sobre los 66 ítems reales: ninguno cambia.
//  12. UNA SECCIÓN ESCRITA COMO PÁRRAFO NO LA CONTABA NADIE. `flota-pantalla-sin-motor/tasks.md`
//      escribe su «Lo que queda fuera» sin viñetas, así que el barrido decía «0 ítems, cero cabos
//      sin registro» sobre un texto que no había mirado, y el piso por archivo tampoco lo cubría
//      porque el archivo no estaba en la tabla. Ahora, cuando una sección no tiene NI UNA viñeta,
//      su párrafo vale como un cabo y se le pide registro igual; y el archivo tiene su piso.
//
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// UNA TABLA DE MARKDOWN SE PARSEA, NO SE ADIVINA
//
// Todo lo que sigue lee ABIERTO.md a través de estas funciones. Ninguna guarda vuelve a preguntar
// «¿la línea empieza con `| A12 |`?», porque ésa era la pregunta que un espacio de alineación
// contestaba que no — y con ella se apagaban las dos guardas de la tabla 1 a la vez.
// ════════════════════════════════════════════════════════════════════════════════════════════

// celdasDeFila parte una fila de tabla en sus celdas REALES: las barras escapadas (`\|`) son texto
// —GFM las respeta adentro de un code span— y no separan nada. Es por un falso positivo medido: la
// fila de A98 lleva `Get-Process musubi \| Select-Object` en su celda.
//
// Devuelve las celdas SIN recortar: quien las use decide si el padding importa.
func celdasDeFila(l string) []string {
	l = strings.TrimSpace(l)
	var celdas []string
	var actual strings.Builder
	for i := 0; i < len(l); i++ {
		if l[i] == '|' && (i == 0 || l[i-1] != '\\') {
			celdas = append(celdas, actual.String())
			actual.Reset()
			continue
		}
		actual.WriteByte(l[i])
	}
	celdas = append(celdas, actual.String())
	// La primera y la última son lo de afuera de las barras de los extremos.
	if len(celdas) < 3 {
		return nil
	}
	return celdas[1 : len(celdas)-1]
}

var enfasisMarkdown = regexp.MustCompile("[*~`✔]+")

// tramoDeCodigo es un code span de Markdown con su contenido.
var tramoDeCodigo = regexp.MustCompile("`[^`]*`")

// sinCodigo reemplaza los code spans por un espacio. Ahí adentro viven nombres de función y rutas
// —`TestAlgoDesplegado`, `rustdesk-relay-instalado.sh`—, que terminan como un participio sin ser
// uno. Quien pregunte por la FORMA de una palabra tiene que sacarlos antes; quien pregunte por un
// número de esquema o un nombre de prueba, no.
func sinCodigo(s string) string {
	return tramoDeCodigo.ReplaceAllString(s, " ")
}

// sinEnfasis normaliza una celda para poder LEERLA: le saca el énfasis de Markdown y el padding.
// Es lo que hace que `| **A99** |`, `|  A99  |` y `| A99 |` sean el mismo dato.
func sinEnfasis(celda string) string {
	return strings.TrimSpace(enfasisMarkdown.ReplaceAllString(celda, ""))
}

type filaMD struct {
	celdas []string // normalizadas: sin énfasis y sin padding
	crudas []string // tal cual están en el archivo
	linea  int      // 1-based
}

func (f filaMD) id() string {
	if len(f.celdas) == 0 {
		return ""
	}
	return f.celdas[0]
}

type tablaMD struct {
	columnas     []string // encabezado normalizado a minúsculas
	encabezadoEn int      // 1-based
	filas        []filaMD
}

// columna devuelve el índice de una columna POR SU NOMBRE. Las guardas piden «Slice» o «Por qué
// no», nunca «la última»: si alguien agrega una columna, la guarda tiene que seguir mirando la que
// decide o decir que no la encuentra — jamás mirar en silencio la de al lado.
func (t tablaMD) columna(nombre string) int {
	for i, c := range t.columnas {
		if c == strings.ToLower(nombre) {
			return i
		}
	}
	return -1
}

var idDeRegistro = regexp.MustCompile(`^[AB]\d+$`)

func esSeparadorMD(l string) bool {
	l = strings.TrimSpace(l)
	return strings.HasPrefix(l, "|") && strings.Trim(l, "|-: \t") == ""
}

func esFilaMD(l string) bool {
	l = strings.TrimSpace(l)
	return strings.HasPrefix(l, "|") && strings.HasSuffix(l, "|")
}

// parseTablasMD encuentra TODAS las tablas del documento: un encabezado, su línea de guiones, y
// las filas que vengan detrás hasta que dejen de venir.
func parseTablasMD(texto string) []tablaMD {
	lineas := strings.Split(texto, "\n")
	var out []tablaMD
	for i := 1; i < len(lineas); i++ {
		if !esSeparadorMD(lineas[i]) || !esFilaMD(lineas[i-1]) || esSeparadorMD(lineas[i-1]) {
			continue
		}
		enc := celdasDeFila(lineas[i-1])
		if enc == nil {
			continue
		}
		t := tablaMD{encabezadoEn: i}
		for _, c := range enc {
			t.columnas = append(t.columnas, strings.ToLower(sinEnfasis(c)))
		}
		for j := i + 1; j < len(lineas); j++ {
			if !esFilaMD(lineas[j]) || esSeparadorMD(lineas[j]) {
				break
			}
			crudas := celdasDeFila(lineas[j])
			if crudas == nil {
				break
			}
			f := filaMD{crudas: crudas, linea: j + 1}
			for _, c := range crudas {
				f.celdas = append(f.celdas, sinEnfasis(c))
			}
			t.filas = append(t.filas, f)
		}
		out = append(out, t)
	}
	return out
}

// tablaConColumnas devuelve la tabla cuyo encabezado es EXACTAMENTE ése. Si no está, la guarda que
// la pidió falla diciendo que no encontró dónde mirar: es la diferencia entre «miré y está bien» y
// «no pude mirar», que es el modo de fallo que este archivo persigue.
func tablaConColumnas(tablas []tablaMD, nombres ...string) (tablaMD, bool) {
	for _, t := range tablas {
		if len(t.columnas) != len(nombres) {
			continue
		}
		ok := true
		for i, n := range nombres {
			if t.columnas[i] != strings.ToLower(n) {
				ok = false
				break
			}
		}
		if ok {
			return t, true
		}
	}
	return tablaMD{}, false
}

// registroDeAbiertos devuelve el texto del registro. Falla —no saltea— si no está: una guarda que
// se apaga sola cuando el archivo se mueve es la que deja pasar el problema que busca.
func registroDeAbiertos(t *testing.T) string {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join("..", "..", "specs", "control-de-flota", "ABIERTO.md"))
	if err != nil {
		t.Fatalf("no se pudo leer el registro de abiertos: %v", err)
	}
	return string(crudo)
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// EL BARRIDO DE LOS SPECS DEL TRACK
// ════════════════════════════════════════════════════════════════════════════════════════════

var (
	// EL ENCABEZADO PUEDE VENIR NUMERADO, Y ÉSE ERA LA MITAD DE UN AGUJERO.
	//
	// Hasta el 2026-09-10 esto era `^#+\s*lo que queda fuera`, que rechaza
	// `## 2 · Lo que queda fuera (y va a `ABIERTO.md`)` — la forma que usan los `spec.md` del
	// track. La otra mitad estaba en el glob, que miraba sólo `tasks.md`. **Las dos secciones
	// afectadas fallaban por los DOS motivos a la vez**, así que arreglar uno solo no habría
	// cambiado nada: es la forma exacta de «una guarda presente en N-1 de N caminos».
	encabezadoFuera = regexp.MustCompile(`(?i)^#+\s*(?:\d+\s*[·.)\-]\s*)?lo que queda fuera`)

	// UN TÍTULO QUE HABLA DE LO QUE QUEDA AFUERA Y QUE EL RECONOCEDOR NO ACEPTA ES UN RENOMBRE,
	// NO UNA SECCIÓN QUE NO EXISTE. Renombrar `## Lo que queda fuera` a `## Fuera de alcance`
	// dejaba la sección entera invisible sin que nada dijera nada — el piso global sobrevivía
	// porque le sobraban dos secciones y siete ítems. Esta red ancha convierte ese renombre en un
	// mensaje que dice qué archivo y qué línea.
	encabezadoSospechoso = regexp.MustCompile(`(?i)^#+.*(?:queda fuera|fuera de alcance|fuera del alcance|excluid)`)

	// UNA VIÑETA ES UNA VIÑETA — Y LA LISTA ORDENADA TAMBIÉN ES UNA LISTA.
	//
	// Antes sólo contaba `- **`, así que el asterisco, el ítem sin negrita y el indentado no sólo
	// escapaban a la revisión: **ni siquiera sumaban al contador**, con lo cual el piso tampoco
	// los veía. Se arregló para `-`, `*` y `+` … y quedó afuera la OTRA mitad de la gramática de
	// listas de Markdown: la ordenada. Medido el 2026-09-10 — dos cabos escritos como
	// `1. **…**` / `2. **…**` sin número de registro pasaban en VERDE, el conteo seguía diciendo
	// 66 ítems, y encima el segundo se pegaba como línea de continuación del ítem anterior y
	// heredaba SU registro.
	//
	// Por eso esto ya no es una colección de marcadores que alguien fue agregando de a uno: es la
	// gramática de viñetas de CommonMark completa —bullet (`-` `*` `+`) y ordenada (`N.` `N)`)—,
	// que es la lista de formas que el formato define. No hay una séptima.
	vinetaDeCabo = regexp.MustCompile(`^\s*(?:[-*+]|\d{1,9}[.)])\s+\S`)
)

// marcaDeRegistro es UNA de las cosas que un ítem declarado fuera de alcance puede decir para
// tener casa. Cada una está anclada en la FORMA QUE DECIDE y no en la palabra:
//
//   - `HECH` era un substring, así que `HECHOS de alerta` daba por hecho un ítem. Ahora HECHO y
//     DESCARTADO tienen que decir EN QUÉ SLICE, que es lo que la regla pide.
//   - `por diseño` matcheaba dentro de «por diseño responsivo en pantallas chicas». Ahora tiene
//     que estar donde una razón se cierra: delante de un `:`, un `.`, un `)` o el fin del ítem.
//   - `Cero dependencias` es una COLETILLA fija, no un prefijo: `**Cero dependencias nuevas para
//     el enrutador de severidad.**` no es la coletilla, es un cabo con la coletilla adentro.
//   - `despliegue` es prosa corrientísima en estos specs. La marca es la CATEGORÍA
//     (`es **despliegue**`, `material de despliegue`), no la palabra.
//
// Todas son formas BUENAS: si alguien inventa una forma nueva de decir «esto tiene casa», la
// prueba se pone roja y la decisión de aceptarla se toma acá, a la vista.
var marcasDeRegistro = []struct {
	nombre string
	re     *regexp.Regexp
}{
	{"HECHO/HECHA EN un slice",
		regexp.MustCompile(`\b(?:HECH[OA]S?|HIZO)\b\*{0,2}\s+en\s+\*{0,2}[SU]\d+`)},
	{"DESCARTADO EN un slice",
		regexp.MustCompile(`\bDESCARTAD[OA]S?\b\*{0,2}\s+en\s+\*{0,2}[SU]\d+`)},
	{"«por diseño» cerrando la razón",
		regexp.MustCompile(`(?i)\bpor diseño\*{0,2}\s*(?:[:.,;)\]»]|$)`)},
	{"la coletilla exacta «Cero dependencias nuevas»",
		regexp.MustCompile(`\*\*\s*Cero dependencias nuevas\.?\s*\*\*`)},
	{"«despliegue» como CATEGORÍA, no como palabra",
		regexp.MustCompile(`\*\*despliegue\*\*|\bmaterial de despliegue\b`)},
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LA CITA DE REGISTRO ES UN TOKEN, NO UN NÚMERO SUELTO EN LA PROSA
//
// La marca más usada era `\*\*[^*]*\b[ABS]\d+[a-z]?\b[^*]*\*\*`: «cualquier tramo en negrita que
// mencione algo con forma de número de registro». Eso no pregunta por la forma que ASIGNA el cabo,
// pregunta por un texto que aparece en la prosa. Medido el 2026-09-10 con seis variaciones, y
// cinco escapaban; la más limpia:
//
//	- **La API expone A1 y B2 como campos JSON.**
//
// Ese ítem no está registrado en ningún lado: nombra dos identificadores del producto que resultan
// tener forma de número de registro, y con eso se daba por anotado. Igual pasaba con
// `**el umbral de B5 minutos**` o `**Se documenta en A4 de otro track**`.
//
// La forma que DECIDE es la que el track usa para citar el registro y que se lee de un vistazo: el
// número —solo, o encadenado con otros— OCUPANDO TODO el tramo en negrita, como en `(**A42**)`,
// `(**A17 → S7c**)`, `**A1/A2/A3**` o `cierra **A25**`. Un tramo en negrita que además tiene prosa
// adentro no es una cita: es una oración que menciona un número.
//
// Se midió sobre los 66 ítems reales del track antes de ponerla: ninguno cambia de veredicto.
// ════════════════════════════════════════════════════════════════════════════════════════════

var (
	tramoEnNegrita = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	// El tramo entero, sin prosa: uno o más números encadenados con `→`, `/`, `,`, `;`, `+` o «y».
	soloCitaDeRegistro = regexp.MustCompile(`^[ABS]\d+[a-z]?(?:\s*(?:[/,;+]|y|→|->|➜)\s*[ABS]\d+[a-z]?)*$`)
)

// citaElRegistro dice si el ítem lleva un número de registro CITADO —el tramo en negrita es el
// número y nada más—, y no sólo un número mencionado adentro de una oración.
func citaElRegistro(item string) bool {
	for _, m := range tramoEnNegrita.FindAllStringSubmatch(item, -1) {
		if soloCitaDeRegistro.MatchString(strings.TrimSpace(m[1])) {
			return true
		}
	}
	return false
}

func tieneCasa(item string) bool {
	if citaElRegistro(item) {
		return true
	}
	for _, m := range marcasDeRegistro {
		if m.re.MatchString(item) {
			return true
		}
	}
	return false
}

// PISO POR ARCHIVO, NO GLOBAL.
//
// El piso global («al menos 12 secciones y 40 ítems») sólo cazaba el renombre MASIVO: con 14
// secciones y 45 ítems —de 15 y 47— la prueba pasaba habiendo perdido `## 2 · Lo que queda fuera`
// entera en `specs/flota-alertas-y-politicas/spec.md`. Medido, y en verde.
//
// Un piso por archivo no tiene ese margen: la sección que se pierde se pierde EN SU ARCHIVO, y el
// mensaje dice cuál. Los números son los que hay hoy; bajarlos es una decisión visible acá.
var pisoPorArchivo = map[string]struct{ secciones, items int }{
	"specs/flota-agente-y-latido/tasks.md":    {1, 3},
	"specs/flota-alertas-y-politicas/spec.md": {1, 3},
	"specs/flota-empuje-otlp/tasks.md":        {1, 8},
	"specs/flota-exec-auditado/tasks.md":      {1, 4},
	"specs/flota-movil-y-sonda/tasks.md":      {1, 4},
	"specs/flota-panel/tasks.md":              {1, 4},
	"specs/flota-pantalla-rustdesk/tasks.md":  {1, 4},
	// EL ÚNICO ARCHIVO DEL TRACK CON SECCIÓN Y SIN PISO — el hueco lo encontró el verificador el
	// 2026-09-10 y no era de nadie. Su «Lo que queda fuera» no tiene viñetas: es un párrafo suelto,
	// y un párrafo suelto no lo contaba nadie. Ahora vale como UN cabo (ver `cerrarSeccion`), así
	// que su piso es 1 y renombrar o vaciar la sección se pone rojo como en los otros catorce.
	"specs/flota-pantalla-sin-motor/tasks.md":     {1, 1},
	"specs/flota-registro-dispositivos/tasks.md":  {1, 4},
	"specs/flota-scopes-de-token/tasks.md":        {1, 3},
	"specs/flota-servicios/tasks.md":              {1, 9},
	"specs/flota-shell-contra-sshd-real/tasks.md": {1, 3},
	"specs/flota-shell-interactiva/spec.md":       {1, 5},
	"specs/flota-telemetria-del-host/tasks.md":    {1, 6},
	"specs/flota-tier-b-protocolo/tasks.md":       {1, 4},
}

// cabo es un ítem COMPLETO: la viñeta y sus líneas de continuación. La marca que lo registra puede
// venir en la segunda línea —`~~**Cooldown persistente.**~~` la lleva ahí— y una guarda que mira
// línea por línea la pierde.
type cabo struct {
	ruta  string
	linea int
	texto string
}

// specsDelTrack camina el árbol. **El glob `specs/flota-*/*.md` no recursaba**, así que un
// `spec.md` metido en un subdirectorio del track era invisible: la guarda decía «41 archivos
// barridos» y el cabo estaba en el 42.
func specsDelTrack(t *testing.T) []string {
	t.Helper()
	raiz := filepath.Join("..", "..", "specs")
	dirs, err := filepath.Glob(filepath.Join(raiz, "flota-*"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range dirs {
		err := filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(out)
	return out
}

// clave normaliza una ruta del walk a la forma con la que la nombra `pisoPorArchivo`.
func clave(ruta string) string {
	return filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(ruta), "../../"))
}

func TestNingunCaboDeFlotaSeQuedaSinRegistro(t *testing.T) {
	specs := specsDelTrack(t)
	// Si el barrido deja de encontrar los specs (se movieron, se renombraron), la prueba pasaría
	// vacía y en verde — el modo de fallo más peligroso que puede tener un barrido.
	if len(specs) < 35 {
		t.Fatalf("sólo se encontraron %d archivos de spec de flota; el barrido no está mirando donde cree", len(specs))
	}

	type cuenta struct{ secciones, items int }
	visto := map[string]cuenta{}
	huerfanos := 0
	totalSecciones, totalItems := 0, 0

	for _, ruta := range specs {
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		lineas := strings.Split(string(crudo), "\n")
		c := cuenta{}
		dentro := false
		var actual *cabo
		// El párrafo que abre la sección, antes de la primera viñeta. Ver `cerrarSeccion`.
		var preambulo *cabo
		vinetasEnLaSeccion := 0

		revisar := func(item *cabo) {
			c.items++
			if !tieneCasa(item.texto) {
				huerfanos++
				t.Errorf("%s:%d — cabo sin registro:\n    %s\n  Anotalo en specs/control-de-flota/ABIERTO.md (tabla 1 con slice, o tabla 2 con la condición bajo la que se revisa) y nombrá acá su número, o decí en qué slice se HIZO o se DESCARTÓ.",
					item.ruta, item.linea, recorte(strings.TrimSpace(strings.ReplaceAll(item.texto, "\n", " ")), 110))
			}
		}

		cerrar := func() {
			if actual == nil {
				return
			}
			revisar(actual)
			actual = nil
		}

		// UNA SECCIÓN SIN NI UNA VIÑETA NO ES UNA SECCIÓN LIMPIA: ES UNA QUE NO SE ENTENDIÓ.
		//
		// `specs/flota-pantalla-sin-motor/tasks.md` escribe su «Lo que queda fuera» como un párrafo
		// suelto, sin viñetas. El barrido contaba 0 ítems, 0 huérfanos y decía «cero cabos sin
		// registro» — verde silencioso sobre un texto que no había mirado. Y el piso por archivo
		// tampoco lo cubría, porque el archivo no estaba en la tabla.
		//
		// El párrafo de apertura de una sección que SÍ tiene viñetas es otra cosa —en
		// `flota-shell-interactiva/spec.md` es una nota de método, no un cabo—, así que sólo se
		// revisa cuando la sección entera no tuvo ni una viñeta: ahí el párrafo ES el contenido.
		cerrarSeccion := func() {
			cerrar()
			if preambulo != nil && vinetasEnLaSeccion == 0 {
				revisar(preambulo)
			}
			preambulo = nil
			vinetasEnLaSeccion = 0
		}

		for n, linea := range lineas {
			if strings.HasPrefix(linea, "#") {
				cerrarSeccion()
				dentro = encabezadoFuera.MatchString(linea)
				if dentro {
					c.secciones++
				} else if encabezadoSospechoso.MatchString(linea) {
					t.Errorf("%s:%d — este encabezado habla de lo que queda AFUERA y el barrido no lo reconoce:\n    %s\n"+
						"  Renombrar la sección la vuelve invisible entera, y el piso global no lo notaba: con 14 secciones y 45 ítems (de 15 y 47) la prueba pasaba igual.\n"+
						"  Devolvele el título `## Lo que queda fuera` (podés numerarlo: `## 2 · Lo que queda fuera`).",
						ruta, n+1, strings.TrimSpace(linea))
				}
				continue
			}
			if !dentro {
				continue
			}
			if strings.TrimSpace(linea) == "" {
				cerrar()
				continue
			}
			if vinetaDeCabo.MatchString(linea) {
				cerrar()
				vinetasEnLaSeccion++
				actual = &cabo{ruta: ruta, linea: n + 1, texto: linea}
				continue
			}
			if actual != nil {
				actual.texto += "\n" + linea
				continue
			}
			// Prosa suelta: cuelga del preámbulo de la sección.
			if preambulo == nil {
				preambulo = &cabo{ruta: ruta, linea: n + 1, texto: linea}
			} else {
				preambulo.texto += "\n" + linea
			}
		}
		cerrarSeccion()
		visto[clave(ruta)] = c
		totalSecciones += c.secciones
		totalItems += c.items
	}

	// CERO TIENE QUE SIGNIFICAR «MIRÉ Y ESTÁ LIMPIO», NUNCA «NO PUDE MIRAR» — Y AHORA POR ARCHIVO.
	for ruta, piso := range pisoPorArchivo {
		v, ok := visto[ruta]
		if !ok {
			t.Errorf("%s ya no lo barre esta guarda: o se movió, o se renombró, o el recorrido dejó de llegar.\n"+
				"  Mientras no esté, sus cabos son invisibles y la prueba se pondría en verde sin haberlos mirado.", ruta)
			continue
		}
		if v.secciones < piso.secciones || v.items < piso.items {
			t.Errorf("%s: el barrido reconoció %d sección(es) «Lo que queda fuera» y %d ítem(s), y el piso de ESTE archivo es %d y %d.\n"+
				"  Un piso global no ve esto: con 14 secciones y 45 ítems sobre 15 y 47, una sección entera renombrada pasaba en verde. Medido.\n"+
				"  Si sacaste un ítem a propósito, bajá el piso de este archivo en `pisoPorArchivo` —a la vista— en el mismo commit.",
				ruta, v.secciones, v.items, piso.secciones, piso.items)
		}
	}

	if huerfanos == 0 {
		t.Logf("%d archivos de spec de flota barridos, %d secciones «Lo que queda fuera», %d ítems, cero cabos sin registro",
			len(specs), totalSecciones, totalItems)
	}
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// EL REGISTRO SIGUE EN PIE
//
// Si alguien lo borra o lo vacía, el barrido de arriba seguiría en verde (los specs no cambiaron)
// mientras el archivo al que mandan sus mensajes deja de decir nada.
//
// ESTA PRUEBA ERAN CUATRO `strings.Contains` Y NO SERVÍA PARA LO QUE DICE SU NOMBRE: dos de los
// fragmentos eran `"| A"` y `"| B"` buscados en el archivo entero, así que la tabla 1 reducida a
// UNA fila —o a ninguna— pasaba en verde. Ahora parsea.
//
// Sabotaje que la hace fallar: borrar las filas de cualquiera de las dos tablas, o dejar vacía la
// celda de estado de una fila.
// ════════════════════════════════════════════════════════════════════════════════════════════

func TestElRegistroDeAbiertosSigueEnPie(t *testing.T) {
	texto := registroDeAbiertos(t)

	// Las cuatro secciones, EN ORDEN. El orden importa porque los cortes de abajo parten el
	// archivo por estos límites: si se cruzan, se estaría leyendo una parte por otra.
	secciones := []string{
		"## 1 · Con slice asignado",
		"## 2 · Decisiones de NO hacer",
		"## 3 · Cerrado en este track",
		"## Cómo se usa este archivo",
	}
	previo := -1
	for i, s := range secciones {
		en := strings.Index(texto, s)
		if en < 0 {
			t.Fatalf("ABIERTO.md perdió la sección %q: el registro dejó de tener la forma que sus propias reglas describen", s)
		}
		if en <= previo {
			t.Fatalf("la sección %q de ABIERTO.md no está después de %q: las secciones se reordenaron y este barrido leería una tabla por otra", s, secciones[i-1])
		}
		previo = en
	}

	tablas := parseTablasMD(texto)
	for _, caso := range []struct {
		nombre   string
		columnas []string
		prefijo  string
		piso     int
		porque   string
	}{
		{"1 · Con slice asignado", []string{"#", "qué falta", "por qué no está", "slice"}, "A", 20,
			"no queda ni un cabo con dueño: o se terminó todo, o alguien vació la tabla"},
		{"2 · Decisiones de NO hacer", []string{"#", "qué", "por qué no"}, "B", 15,
			"no queda ni una decisión de no-hacer: sospechoso"},
	} {
		tab, ok := tablaConColumnas(tablas, caso.columnas...)
		if !ok {
			t.Errorf("la tabla «%s» de ABIERTO.md ya no tiene el encabezado %v.\n  Sin encabezado no hay columnas, y las guardas que miran LA COLUMNA QUE DECIDE dejan de saber cuál es.",
				caso.nombre, caso.columnas)
			continue
		}
		n := 0
		for _, f := range tab.filas {
			if !strings.HasPrefix(f.id(), caso.prefijo) || !idDeRegistro.MatchString(f.id()) {
				continue
			}
			n++
			for j, celda := range f.celdas {
				if celda == "" {
					t.Errorf("la fila %s de la tabla «%s» (línea %d) tiene la celda %d VACÍA.\n  «Nada queda abierto sin dueño» es la primera línea de este archivo: una celda vacía es un cabo sin dueño con cara de fila completa.",
						f.id(), caso.nombre, f.linea, j+1)
				}
			}
		}
		if n < caso.piso {
			t.Errorf("la tabla «%s» de ABIERTO.md tiene %d fila(s) y el piso es %d — %s.\n  Un registro con la tabla vaciada se lee igual que uno donde no queda nada abierto, y son cosas opuestas.",
				caso.nombre, n, caso.piso, caso.porque)
		}
	}

	// Las reglas son la parte del archivo que las guardas citan en sus mensajes. Si desaparecen,
	// cada «anotalo en ABIERTO.md» manda a un lugar que ya no explica cómo se anota.
	reglas := texto[strings.Index(texto, "## Cómo se usa este archivo"):]
	for i := 1; i <= 6; i++ {
		if !strings.Contains(reglas, "\n"+strconv.Itoa(i)+". ") {
			t.Errorf("«Cómo se usa este archivo» perdió la regla %d.\n  Las guardas del track mandan a leerla; una regla que no está se lee como una regla que no existe.", i)
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LA REGLA 1: UNA FILA DE LA TABLA 1 NO PUEDE DECLARARSE CERRADA
//
// «Al cerrar un slice, BORRAR su línea de la tabla 1» se cumplió a mano hasta que dejó de
// cumplirse. Medido el 2026-09-10: **21 de las 48 filas** declaraban en su celda de estado que el
// cabo ya estaba cerrado, y dos de ellas —A99 y A102— estaban ABIERTAS escondidas detrás de la
// palabra («cerrado; falta el redespliegue»).
//
// POR QUÉ NO ES UN grep DE PALABRAS SOBRE LA CELDA, Y POR QUÉ TAMPOCO ES UNA LISTA MÁS LARGA
//
// La primera versión miraba la CABEZA del veredicto y buscaba tres palabras («cerrado», «nada
// pendiente», «queda como registro») sacadas de las 22 celdas que ya estaban. Un saboteador la
// evadió once veces en una tarde: con «hecho», con «resuelto», con «completado», con «listo», con
// «ya salió», y —lo peor— dejando el cierre para la SEGUNDA oración, que es exactamente la forma
// de A99/A102 con las cláusulas dadas vuelta.
//
// **Una lista de palabras malas siempre le falta la próxima.** Así que la guarda dio vuelta la
// pregunta: la columna que decide («Slice») tiene que EMPEZAR con una de las cuatro formas con las
// que este registro asigna un cabo VIVO —un slice, `sin asignar`, `gio`, `acción del operador`—.
// Es una lista de formas BUENAS: «hecho», «resuelto» y las que vengan no están en ella y por eso
// FALLAN, sin que nadie tenga que anticiparlas.
//
// Y EL FALSO POSITIVO DE A31 SE ARREGLÓ MIRANDO DONDE DECIDE, NO PONIENDO UNA EXCEPCIÓN
//
// A31 decía «`VerificarFirma` **falla cerrado**» —el modo de fallo de una verificación de firma, no
// el estado de un cabo— y estaba en la celda de Slice porque esa celda arrastraba **4878
// caracteres** de medición: cincuenta veces la más larga de la tabla. La prosa se plegó a la
// columna «Por qué no está», que es donde va, y la columna que decide quedó corta y sin ambigüedad.
// Por eso ahora se puede revisar la celda ENTERA —todas sus oraciones— sin una sola excepción, y el
// tope de largo de abajo es lo que mantiene esa propiedad.
//
// Sabotaje que la hace fallar: ponerle `| — (cerrado) |`, `| hecho |` o
// `| **gio** — falta el redespliegue. Por lo demás quedó cerrado. |` a cualquier fila de la tabla 1.
// ════════════════════════════════════════════════════════════════════════════════════════════

var (
	// LAS CUATRO FORMAS DE ASIGNAR UN CABO VIVO. Son las que el archivo usa; agregar una quinta es
	// una decisión visible acá, y NO agregarla hace que la guarda falle en vez de pasar.
	asignacionPendiente = regexp.MustCompile(`(?i)^(?:S\d+[a-z]?|sin asignar|gio|acción del operador|accion del operador)\b`)

	// EL CIERRE ESCONDIDO EN EL CALIFICADOR, Y POR QUÉ ESTO DEJÓ DE SER UNA LISTA DE PALABRAS.
	//
	// `asignacionPendiente` cierra el caso en que la palabra de cierre REEMPLAZA al dueño. No
	// cierra el que de verdad pasó: A99, A102 y A116 tenían dueño —`**gio** — …`— y el cierre
	// venía DETRÁS. Ahí el único que decide es este reconocedor, y mientras fue una lista de
	// diez palabras se lo evadía con la once: se midió el 2026-09-10 que 20 de 20 redacciones
	// («finiquitado», «clausurado», «zanjado», «liquidado», «archivado», «cumplido»,
	// «implementado y verificado», …) pasaban en VERDE detrás de un `**gio** —` legítimo.
	//
	// El arreglo no es una lista más larga: es dejar de preguntar por la palabra y preguntar por
	// la FORMA GRAMATICAL con la que el castellano dice «esto ya se hizo».
	//
	//   (a) `participioPerfectivo` — la clase PRODUCTIVA: todo participio regular en -ado/-ido
	//       (con sus femeninos y plurales). Es de donde salen las palabras nuevas, así que una
	//       que nadie escribió todavía cae acá sin que haya que anticiparla. Se piden 3+ letras
	//       de raíz para no comerse `nada`, `cada` ni `vida`.
	//   (b) los IRREGULARES, que por definición no son productivos y sí son una lista cerrada:
	//       hecho, resuelto, puesto, visto, muerto, dicho, escrito, abierto, cubierto, roto, dado.
	//   (c) las locuciones que no son participio ninguno («nada pendiente», «ya salió»).
	//
	// LO QUE ESTO TODAVÍA NO AGARRA, dicho acá y no escondido: una perífrasis sin participio
	// («ya está», «sin novedad», «se hizo», «quedó atrás», «funcionando en las 4 máquinas»).
	// La columna que decide es prosa libre y la prosa libre no se clasifica entera; lo que la
	// acota es el tope de 220 caracteres y `asignacionPendiente`.
	// OJO CON `\b` Y LAS VOCALES ACENTUADAS: el `\w` de RE2 es ASCII, así que en «salió» la `ó` NO
	// es carácter de palabra y `\b` detrás de ella no casa al final del texto. Por eso las
	// locuciones se arman SIN el `\b` de cierre y los participios CON él.
	participioPerfectivo = `\b(?:\w{3,}(?:ad|id)[oa]s?|hech[oa]s?|resuelt[oa]s?|puest[oa]s?|vist[oa]s?|muert[oa]s?|dich[oa]s?|escrit[oa]s?|abiert[oa]s?|cubiert[oa]s?|rot[oa]s?|dad[oa]s?|list[oa]s?)\b`
	locucionesDeCierre   = `\b(?:nada pendiente|queda como registro|ya sali[oó]|ya est[áa]|sin novedad|se hizo|qued[óo] atr[áa]s|no queda nada)`

	veredictoDeCierre = regexp.MustCompile(`(?i)` + participioPerfectivo + `|(?i)` + locucionesDeCierre)

	// LA APERTURA DE LA CELDA: LOS AUXILIARES QUE VIENEN ANTES DEL VERBO QUE DECIDE.
	//
	// «Ya se implementó», «Ya está hecho», «Quedó resuelto», «Fue descartado»: la palabra que dice
	// si el ítem existe viene DETRÁS de una cadena de auxiliares. Se comen acá para poder mirar el
	// verbo. `no` NO está en la lista a propósito: «No se implementó porque…» es una razón, no un
	// cierre, y meterlo lo daría vuelta.
	auxiliaresAlAbrir = regexp.MustCompile(`(?i)^(?:(?:ya|se|est[áa]|est[áa]n|qued[óo]|quedaron|fue|fueron|son|ha|han)\s+)*`)

	// La primera palabra de lo que queda, sin la puntuación pegada.
	palabraSuelta = regexp.MustCompile(`^[\p{L}\p{M}]+`)

	// Las locuciones de cierre que NO son un verbo, ancladas al arranque. `ya est[áa]` pide
	// puntuación o fin de texto detrás porque suelto se come «Ya está en el backlog», que no
	// cierra nada.
	locucionAlAbrir = regexp.MustCompile(`(?i)^(?:nada pendiente|queda como registro|ya sali[oó]|sin novedad|se hizo|qued[óo] atr[áa]s|no queda nada|decisi[óo]n tomada|ya est[áa](?:[.,;:!]|$))`)

	// EL OBJETIVO DE UN DESPLIEGUE CITADO EN LA CELDA QUE DECIDE.
	//
	// Decía sólo `esquema`, y el registro llama a la misma cosa de las dos maneras —la fila de
	// A99 dice «esquema **53**; la columna es de la migración 50»—. Medido el 2026-09-10:
	// `(migración **49**)` en la columna que decide pasaba en VERDE, que es el defecto de A116
	// exacto escrito con la palabra hermana. Ahora usa la MISMA alternancia que `citaDeEsquema`,
	// que ya la tenía: las dos preguntas al repo se hacen con el mismo vocabulario, y no hay una
	// que conozca un sinónimo que a la otra le falta.
	esquemaObjetivo = regexp.MustCompile(`(?i)\b(?:esquema|migraci[oó]n)e?s?\s+\*{0,2}(\d+)`)
)

// LA COLUMNA QUE DECIDE NO ES UN LUGAR PARA PROSA.
//
// 4878 caracteres tenía la celda de Slice de A31, contra 101 de la siguiente más larga. Ese solo
// hecho creaba el falso positivo que obligó a mirar sólo la primera oración, y mirar sólo la
// primera oración es lo que dejó pasar 18 de 22 sabotajes. El tope no es cosmético: es lo que
// permite que la guarda revise la celda entera.
const topeDeCeldaQueDecide = 220

func tabla1(t *testing.T, texto string) tablaMD {
	t.Helper()
	tab, ok := tablaConColumnas(parseTablasMD(texto), "#", "qué falta", "por qué no está", "slice")
	if !ok {
		t.Fatalf("no se encontró la tabla 1 de ABIERTO.md por su encabezado `| # | Qué falta | Por qué no está | Slice |`.\n  La guarda pide la columna que decide POR SU NOMBRE: si el encabezado cambió, prefiere fallar antes que mirar en silencio la columna de al lado.")
	}
	return tab
}

func TestNingunaFilaDeLaTabla1SeDeclaraCerrada(t *testing.T) {
	texto := registroDeAbiertos(t)

	// CONTROL POSITIVO — LAS CELDAS QUE ESTUVIERON PUESTAS DE VERDAD, MÁS LAS ONCE FORMAS CON LAS
	// QUE UN SABOTEADOR EVADIÓ LA VERSIÓN ANTERIOR. Sin esto, aflojar cualquiera de las dos mitades
	// deja la prueba en verde para siempre sin mirar nada.
	for _, real := range []string{
		"— (cerrado)",
		"✔ cerrado el 2026-09-05, repo y máquina",
		"**gio** ✔ cerrado; falta el redespliegue",
		"**gio** (decisión tomada: cerrado y verificado en vivo)",
		"**gio** (nada pendiente; queda como registro)",
		// Las evasiones medidas el 2026-09-10, todas en verde contra la versión anterior:
		"**gio** — falta el redespliegue del cerebro. Por lo demás quedó cerrado el 2026-09-05.",
		"**gio** — hecho",
		"**gio** — resuelto",
		"**gio** — completado",
		"**gio** — listo",
		"**gio** — ya salió",
		"hecho",
		"resuelto en S12",
	} {
		if motivo := porQueNoEsUnaAsignacionViva(real); motivo == "" {
			t.Fatalf("el reconocedor ya no ve como CIERRE la celda %q, que es una que estuvo puesta o que evadió a la versión anterior de esta guarda.\n  Se aflojó, y con eso esta prueba dejó de poder fallar.", real)
		}
	}
	// CONTROL NEGATIVO — LAS CELDAS REALES DE LA TABLA 1, INCLUIDA LA DE A31 YA PLEGADA.
	for _, real := range []string{
		"**S4c**",
		"sin asignar",
		"**sin asignar** (decisión de gio)",
		"(sin asignar — (b) es la que hace visible a todas las demás)",
		"**gio** — falta el redespliegue del cerebro (esquema **53**; la columna es de la migración 50)",
		"**acción del operador** (③ · `BACKUP_REMOTE`)",
	} {
		if motivo := porQueNoEsUnaAsignacionViva(real); motivo != "" {
			t.Fatalf("la guarda acusa a la celda %q, que es una asignación viva y correcta: %s\n  Una guarda que acusa filas correctas se termina apagando.", real, motivo)
		}
	}
	// CONTROL NEGATIVO 2 — «falla cerrado» ES UN MODO DE FALLO, Y VIVE EN LA COLUMNA DE AL LADO.
	// El texto real de A31. La guarda no lo mira porque pide la columna «Slice» POR SU NOMBRE: la
	// corrección no fue una excepción, fue dejar de leer donde no se decide.
	a31 := "**Medido de nuevo el 2026-09-01**: `VerificarFirma` **falla cerrado** y lo dice con todas las letras."
	tab := tabla1(t, texto)
	iSlice, iPorQue := tab.columna("slice"), tab.columna("por qué no está")
	if iSlice < 0 || iPorQue < 0 {
		t.Fatal("la tabla 1 perdió la columna «Slice» o «Por qué no está»")
	}
	if iSlice == iPorQue {
		t.Fatal("«Slice» y «Por qué no está» resolvieron a la misma columna")
	}
	_ = a31 // se afirma abajo, sobre la fila real

	esquemaMax := esquemaMaximoDelRepo(t)
	revisadas := 0
	for _, f := range tab.filas {
		if !idDeRegistro.MatchString(f.id()) {
			continue
		}
		if len(f.celdas) <= iSlice {
			continue // lo cuenta TestTodaFilaDeAbiertoTieneLasCeldasDeSuEncabezado, con su mensaje
		}
		revisadas++
		celda := f.celdas[iSlice]

		if len([]rune(celda)) > topeDeCeldaQueDecide {
			t.Errorf("la fila **%s** (línea %d) tiene %d caracteres en la columna «Slice», y el tope es %d.\n"+
				"  Esa columna dice DE QUIÉN es el cabo y qué falta; la medición va en «Por qué no está».\n"+
				"  No es cosmético: la celda de A31 llegó a 4878 caracteres y ahí adentro decía «`VerificarFirma` falla cerrado», el falso positivo que obligó a esta guarda a mirar sólo la primera oración — y mirar sólo la primera oración es lo que dejó pasar 18 de 22 sabotajes.",
				f.id(), f.linea, len([]rune(celda)), topeDeCeldaQueDecide)
			continue
		}

		// Se le pasa la celda CRUDA: `porQueNoEsUnaAsignacionViva` necesita ver los backticks para
		// poder sacar los tramos de código antes de preguntar por la forma de las palabras.
		if motivo := porQueNoEsUnaAsignacionViva(f.crudas[iSlice]); motivo != "" {
			t.Errorf("la fila **%s** (línea %d) no declara un dueño VIVO en su columna «Slice»: %s\n    %s\n"+
				"  La regla 1 dice que al cerrar un slice se BORRA su línea de la tabla 1 y su texto baja a la sección 3.\n"+
				"  Una fila cerrada adentro de la tabla que contesta «¿qué falta?» es una respuesta falsa, y encima rompe el conteo: el 2026-09-10 eran 21 de 48.\n"+
				"  Si el cabo NO está cerrado del todo, el estado tiene que decir QUÉ FALTA detrás del dueño (así se arreglaron A99 y A102, que decían «cerrado; falta el redespliegue»).",
				f.id(), f.linea, motivo, recorte(celda, 160))
		}

		// EL OBJETIVO DE UN DESPLIEGUE ENVEJECE, Y ENVEJECE HACIA EL LADO TRANQUILIZADOR.
		// A116 mandaba desplegar «el cerebro a esquema 48» con el repo en 53, y la 48 de hoy es
		// otra migración. Un objetivo viejo que además señala otra cosa no falla: tranquiliza.
		//
		// SE MIRA EL NÚMERO MÁS ALTO DE LA CELDA, NO CADA NÚMERO. La celda que decide nombra el
		// destino y, a veces, de dónde viene: A99 dice «esquema **53**; la columna es de la
		// migración 50». Exigirle a CADA número que sea el máximo acusa esa fila, que es correcta
		// —medido: el 2026-09-10 se rompió así y hubo que revertirlo—. El destino es el más nuevo
		// que la celda nombra; los otros son de dónde se viene. Y mirar sólo la palabra «esquema»
		// tampoco servía: `(migración **49**)` es el mismo defecto con la palabra hermana, y
		// pasaba en VERDE.
		objetivo, hayObjetivo := 0, false
		for _, m := range esquemaObjetivo.FindAllStringSubmatch(celda, -1) {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			hayObjetivo = true
			if n > objetivo {
				objetivo = n
			}
		}
		if hayObjetivo && objetivo != esquemaMax {
			t.Errorf("la fila **%s** (línea %d) manda desplegar a «esquema/migración %d» y el repo está en **%d**.\n"+
				"  Es el defecto de A116 exacto: un objetivo obsoleto en la columna que decide, y encima la migración %d de hoy es otra cosa.\n"+
				"  El número sale de `internal/memory/migrations.go`, no de un informe.",
				f.id(), f.linea, objetivo, esquemaMax, objetivo)
		}
	}
	// CERO FILAS REVISADAS NO ES «TODO LIMPIO»: es que el parseo dejó de encontrar la tabla.
	if revisadas < 20 {
		t.Fatalf("sólo se revisaron %d fila(s) de la tabla 1 y el piso es 20: cambió el formato del archivo y esta guarda dejó de mirar", revisadas)
	}
}

// porQueNoEsUnaAsignacionViva devuelve "" si la celda declara un dueño vivo, y si no, POR QUÉ no.
//
// Dos preguntas, en este orden:
//
//  1. ¿Arranca con una de las cuatro formas de asignación? Es una lista de formas BUENAS: cualquier
//     palabra de estado —incluidas las que nadie escribió todavía— falla acá.
//  2. ¿El calificador que sigue al dueño declara un cierre? Es donde escapaban 18 de 22 filas,
//     escribiendo el veredicto en la segunda oración.
func porQueNoEsUnaAsignacionViva(celda string) string {
	// UN VEREDICTO ES PROSA, NO UN IDENTIFICADOR. La morfología de participio mira el final de
	// las palabras, y un `TestLoQueSeaDesplegado` o un `deploy/rustdesk-relay-instalado.sh`
	// terminan igual que el participio que buscamos sin decir NADA del estado del cabo. Medido:
	// `TestUnaPruebaQueNoExisteEnNingunLado` hacía saltar la guarda por «…nLado». Los tramos de
	// código se sacan antes de preguntar, que es donde el registro pone nombres y rutas.
	limpia := sinEnfasis(sinCodigo(celda))
	desnuda := strings.TrimLeft(limpia, "(—–- ")
	m := asignacionPendiente.FindString(desnuda)
	if m == "" {
		// `false`: en la tabla 1 «decidido»/«descartado» TAMPOCO son un dueño. La excepción de la
		// familia de la deliberación existe para la tabla 2, donde esas palabras son el contenido
		// correcto de la celda. Acá sólo se elige el texto del mensaje.
		if hechoConsumadoAlAbrir(desnuda, false) != "" {
			return "arranca declarando un ESTADO y no un dueño"
		}
		return "no arranca con un slice, `sin asignar`, `gio` ni `acción del operador`"
	}
	calificador := desnuda[len(m):]
	if hit := veredictoDeCierre.FindString(calificador); hit != "" {
		return fmt.Sprintf("detrás del dueño declara el cabo resuelto (%q)", hit)
	}
	return ""
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// «ESTO YA PASÓ», POR MORFOLOGÍA Y NO POR VOCABULARIO
//
// `abreConCierre` era una lista de nueve palabras —cerrado, hecho, resuelto, completado, listo,
// terminado…— sacadas de las celdas que ya estaban. Es el mismo defecto que la tabla 1 pagó y
// arregló: **una lista de palabras malas siempre le falta la próxima**. Medido el 2026-09-10 sobre
// la tabla 2, que era lo único que todavía dependía de ella: «Finiquitado en S12.» y «Ya se
// implementó y quedó andando.» al arranque de la celda pasaban en VERDE.
//
// La pregunta ya no es qué palabra es, sino con qué FORMA el castellano declara un hecho
// consumado. Son dos, y las dos son productivas —de ahí salen las palabras que nadie escribió
// todavía—:
//
//   - el participio perfectivo regular (-ado/-ido, con femeninos y plurales), que es de donde
//     vienen «finiquitado», «zanjado», «liquidado», «archivado», «implementado»;
//   - el pretérito perfecto simple de 3ª persona (-ó, -aron, -ieron): «se implementó», «salió».
//
// LOS IRREGULARES QUE NO ENTRAN, Y POR QUÉ — ES DONDE ESTABAN LOS FALSOS POSITIVOS.
// `participioPerfectivo` (tabla 1) lista once irregulares. Acá se miran sólo `hecho` y `resuelto`.
// Los otros nueve abren oraciones perfectamente sanas en una celda que explica una razón:
// «**Dado** que el relay se corta…», «**Puesto** que hbbs no expone API…», «**Visto** el costo…»,
// «**Dicho** esto…», «**Vista** del CRM…», «**Lista** blanca de comandos…». Son conjunciones,
// marcadores de discurso y sustantivos, no veredictos. Fuera de la cabeza de la celda no molestan
// —por eso la tabla 1 sí los usa—, pero en la cabeza acusarían a filas correctas.
//
// LA FAMILIA DE LA DELIBERACIÓN, QUE ES CONTENIDO CORRECTO Y NO UN CIERRE.
// «**Decidido** por gio el 2026-08-29: Prometheus + Alertmanager es el autoritativo» es la cabeza
// REAL de B20, y es exactamente lo que la tabla 2 existe para decir. «Se midió y son 67 usos», «Se
// probó con dropbear», «Descartado por costo» son lo mismo: hablan de lo que se hizo CON LA
// DECISIÓN, no de que el ítem exista. Por eso hay una lista —de formas BUENAS, como la de la tabla
// 1— con los lemas de esa familia. Una que falte se pone ROJA en vez de pasar, y agregarla es una
// decisión visible acá; el mensaje de la guarda lo dice con todas las letras.
//
// LO QUE ESTO NO AGARRA, DICHO ACÁ: sólo se mira la CABEZA de la celda (hasta el primer punto). El
// veredicto escondido en la tercera oración de una celda de 4000 caracteres de prosa se escapa. En
// la tabla 1 eso se cerró con el tope de 220 caracteres sobre la columna que decide; la columna
// «Por qué no» de la tabla 2 es prosa larga por diseño y no admite ese tope.
// ════════════════════════════════════════════════════════════════════════════════════════════

// irregularesQueSiCierran: los dos participios irregulares que en la CABEZA de una celda no
// significan otra cosa. Ver arriba por qué los otros nueve quedaron afuera.
var irregularesQueSiCierran = map[string]bool{
	"hecho": true, "hecha": true, "hechos": true, "hechas": true,
	"resuelto": true, "resuelta": true, "resueltos": true, "resueltas": true,
	"listo": true, "listos": true, // «lista»/«listas» no: es un sustantivo
}

// lemasDeDeliberación: lo que se hace CON una decisión. No dicen que el ítem exista.
var lemasDeDeliberacion = []string{
	"decid", "decisi", "descart", "rechaz", "desestim", "posterg", "declin",
	"prefer", "prefir", "elegi", "eligi", "optad", "optó", "optaron", "acord",
	"evalu", "consider", "medi", "midi", "midió", "prob", "intent", "discut",
	"analiz", "revis",
}

// esHechoConsumado dice si UNA palabra declara un hecho ya pasado, por su forma.
func esHechoConsumado(p string) bool {
	if irregularesQueSiCierran[p] {
		return true
	}
	r := []rune(p)
	for _, suf := range []string{"ados", "adas", "idos", "idas", "ado", "ada", "ido", "ida"} {
		// 3+ letras de raíz para no comerse «nada», «cada», «vida», «duda».
		if strings.HasSuffix(p, suf) && len(r)-len([]rune(suf)) >= 3 {
			return true
		}
	}
	if strings.HasSuffix(p, "ó") && len(r) >= 3 {
		return true
	}
	for _, suf := range []string{"aron", "ieron"} {
		if strings.HasSuffix(p, suf) && len(r)-len([]rune(suf)) >= 2 {
			return true
		}
	}
	return false
}

// esDeliberacion dice si la palabra viene de la familia «lo que hicimos con la decisión».
func esDeliberacion(p string) bool {
	for _, lema := range lemasDeDeliberacion {
		if strings.HasPrefix(p, lema) {
			return true
		}
	}
	return false
}

// hechoConsumadoAlAbrir devuelve la palabra (o locución) con la que la celda ARRANCA declarando un
// hecho consumado, y "" si abre con otra cosa. `exceptuarDeliberacion` excusa a la familia
// «decidido / descartado / se midió …», que es el contenido correcto de la tabla 2.
func hechoConsumadoAlAbrir(cabeza string, exceptuarDeliberacion bool) string {
	s := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(cabeza), " \t—–-(«\"'*~✔•"))
	bajo := strings.ToLower(s)
	resto := auxiliaresAlAbrir.ReplaceAllString(bajo, "")
	if p := palabraSuelta.FindString(resto); p != "" && esHechoConsumado(p) {
		if exceptuarDeliberacion && esDeliberacion(p) {
			return ""
		}
		return p
	}
	// Las locuciones se miran DESPUÉS del verbo: así «Ya está hecho» resuelve por «hecho» y «Ya
	// está decidido» se excusa por «decidido», en vez de quedar los dos atrapados por «ya está».
	if loc := locucionAlAbrir.FindString(bajo); loc != "" {
		if exceptuarDeliberacion && esDeliberacion(loc) {
			return ""
		}
		return loc
	}
	return ""
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LA TABLA 2 TAMPOCO ES UN CEMENTERIO — Y NO TENÍA NINGUNA GUARDA
//
// Se habían declarado deliberadas dos ausencias. Se re-decidieron el 2026-09-10, después de medir
// lo barato que era evadir la guarda de la tabla 1:
//
//   - la hermana de la regla 1: una DECISIÓN que se declara cerrada en la tabla de lo que sigue
//     decidido. Es el mismo daño y en el mismo archivo;
//   - la regla 3 del propio archivo: «la tabla 2 no es un cementerio: cada línea dice bajo qué
//     condición se revisa». Seis de veintiuna filas no lo decían.
//
// La condición de revisión se pide por su forma canónica —una oración que empieza con «Se
// revisa»— y no adivinando si una frase cualquiera suena condicional. Es una lista de UNA forma
// buena: una paráfrasis nueva no la evade, falla.
// ════════════════════════════════════════════════════════════════════════════════════════════

var condicionDeRevision = regexp.MustCompile(`(?i)\bse revisa\b`)

func TestNingunaFilaDeLaTabla2EsUnCementerio(t *testing.T) {
	texto := registroDeAbiertos(t)
	tab, ok := tablaConColumnas(parseTablasMD(texto), "#", "qué", "por qué no")
	if !ok {
		t.Fatal("no se encontró la tabla 2 de ABIERTO.md por su encabezado `| # | Qué | Por qué no |`")
	}
	i := tab.columna("por qué no")
	if i < 0 {
		t.Fatal("la tabla 2 perdió su columna «Por qué no»")
	}

	// CONTROL POSITIVO — las formas que estuvieron puestas Y las que evadieron a la lista de nueve
	// palabras. Las dos últimas se midieron en verde el 2026-09-10 contra la versión anterior.
	for _, cierre := range []string{
		"— (cerrado)",
		"Ya está hecho en S12.",
		"Finiquitado en S12.",
		"Ya se implementó y quedó andando.",
		"Zanjado con el despliegue del 2026-09-08.",
		"Quedó liquidado por S12b.",
		"Fue archivado: ya está en producción.",
		"Nada pendiente.",
	} {
		if hechoConsumadoAlAbrir(cierre, true) == "" {
			t.Fatalf("el reconocedor ya no ve como CIERRE la cabeza %q, que es una que estuvo puesta o que evadió a la lista de nueve palabras.\n  Se aflojó, y con eso esta prueba dejó de poder fallar.", cierre)
		}
	}
	// CONTROL NEGATIVO — LAS CABEZAS REALES DE ESTA TABLA, Y LA FAMILIA DE LA DELIBERACIÓN.
	// «Decidido por gio…» es la cabeza REAL de B20: una guarda que la acusa se termina apagando.
	for _, sana := range []string{
		"Decidido por gio el 2026-08-29: Prometheus + Alertmanager es el autoritativo para ALERTAR.",
		"Descartado por costo: serían las primeras foreign keys del repo.",
		"Se midió el 2026-09-01 y son 67 usos en 17 archivos.",
		"Se probó contra dropbear y no cambia nada.",
		"No se implementó porque el caso real todavía no apareció.",
		"Daría los tres OS de una y sería la 7ª dependencia directa.",
		"El agregado del host primero.",
		"Nada de webhooks ni de «apagar la máquina» como primitiva.",
		"Dado que el relay se corta, la sesión MUERE.",
		"Puesto que hbbs no expone API, no es viable.",
		"Lista blanca de comandos: va con S10.",
		"Ya está en el backlog de otro track.",
	} {
		if v := hechoConsumadoAlAbrir(sana, true); v != "" {
			t.Fatalf("la guarda acusa de cementerio a la cabeza %q por %q, y es una razón correcta —o la familia de la deliberación, que es lo que esta tabla existe para decir—.\n  Una guarda que acusa filas correctas se termina apagando.", sana, v)
		}
	}
	if condicionDeRevision.MatchString("no hay condición: esto no se toca más") {
		t.Fatal("`condicionDeRevision` da por buena una celda sin condición: se aflojó")
	}

	revisadas := 0
	for _, f := range tab.filas {
		if !idDeRegistro.MatchString(f.id()) || len(f.celdas) <= i {
			continue
		}
		revisadas++
		celda := f.celdas[i]
		cabeza := celda
		if p := strings.Index(cabeza, ". "); p >= 0 {
			cabeza = cabeza[:p]
		}
		if v := hechoConsumadoAlAbrir(cabeza, true); v != "" {
			t.Errorf("la fila **%s** de la tabla 2 (línea %d) ARRANCA declarando un hecho consumado (%q):\n    %s\n"+
				"  La tabla 2 dice qué se decidió NO hacer, no qué se hizo. Si se hizo, la fila se borra y su texto baja a la sección 3, igual que en la tabla 1 (regla 1).\n"+
				"  Esto ya NO es una lista de palabras: se pregunta por la FORMA —participio perfectivo o pretérito— así que una redacción nueva («finiquitado», «zanjado») falla en vez de pasar.\n"+
				"  Si el verbo dice lo que se hizo CON LA DECISIÓN y no que el ítem exista (medir, probar, descartar, decidir…), su lema va en `lemasDeDeliberacion`, acá, a la vista.",
				f.id(), f.linea, v, recorte(cabeza, 140))
		}
		if !condicionDeRevision.MatchString(celda) {
			t.Errorf("la fila **%s** de la tabla 2 (línea %d) no dice bajo qué condición se revisa.\n"+
				"  Es la regla 3 de este archivo: «la tabla 2 no es un cementerio: cada línea dice bajo qué condición se revisa».\n"+
				"  Escribila con esa forma —**Se revisa si …** / **Se revisa cuando …** / **Se revisa el día que …**— que es la que esta guarda reconoce, a propósito: una paráfrasis nueva falla en vez de pasar.\n"+
				"  Si de verdad no hay condición, la decisión no es revisable y no va en esta tabla.",
				f.id(), f.linea)
		}
	}
	if revisadas < 15 {
		t.Fatalf("sólo se revisaron %d fila(s) de la tabla 2 y el piso es 15: cambió el formato del archivo y esta guarda dejó de mirar", revisadas)
	}
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// UN CABO VIVO NO PUEDE TENER TAMBIÉN SU ENTRADA DE CIERRE
//
// La forma estructural de la regla 1, la que no depende de ninguna palabra en la celda: si el
// número tiene fila en la tabla 1 o 2 **y además** la sección 3 dice que está cerrado, una de las
// dos cosas es mentira. Ésta es la que agarra el caso real —se cerró el cabo, se escribió su
// entrada, y nadie borró la fila— sin preguntarle nada al texto de la fila.
// ════════════════════════════════════════════════════════════════════════════════════════════

func TestNingunCaboVivoTieneEntradaDeCierre(t *testing.T) {
	texto := registroDeAbiertos(t)
	vivo, cerrado := partesDelRegistro(t, texto)

	ids := map[string]int{}
	for _, tab := range parseTablasMD(vivo) {
		for _, f := range tab.filas {
			if idDeRegistro.MatchString(f.id()) {
				ids[f.id()] = f.linea
			}
		}
	}
	if len(ids) < 20 {
		t.Fatalf("sólo se reconocieron %d filas vivas en ABIERTO.md; cambió el formato y esta guarda dejó de mirar", len(ids))
	}
	// CONTROL POSITIVO: la forma de cierre que se busca tiene que existir en la sección 3.
	if !regexp.MustCompile(`(?i)\bA\d+\*{0,2}[ ,;:]+cerrad[oa]s?\b`).MatchString(cerrado) {
		t.Fatal("la sección 3 de ABIERTO.md no contiene ni una entrada con la forma «A70 CERRADO»: cambió cómo se cierra un cabo y esta guarda dejó de poder fallar")
	}

	orden := make([]string, 0, len(ids))
	for k := range ids {
		orden = append(orden, k)
	}
	sort.Strings(orden)
	for _, num := range orden {
		q := regexp.QuoteMeta(num)
		for _, patron := range []string{
			`(?i)\b` + q + `\*{0,2}[ ,;:]+cerrad[oa]s?\b`, // «A70 CERRADO»
			`(?i)\bcierran? +\*{0,2}` + q + `\b`,          // «S6b … cierra A13»
		} {
			if m := regexp.MustCompile(patron).FindStringIndex(cerrado); m != nil {
				t.Errorf("**%s** tiene fila VIVA (línea %d de ABIERTO.md) y a la vez su entrada de cierre en la sección 3:\n    …%s…\n"+
					"  Una de las dos cosas es mentira. La regla 1 dice que al cerrar se BORRA la fila; si sigue abierto, la entrada de cierre no debería decir que se cerró.",
					num, ids[num], recorte(strings.TrimSpace(cerrado[max(0, m[0]-90):min(len(cerrado), m[1]+50)]), 170))
				break
			}
		}
	}
}

// partesDelRegistro corta el archivo en «lo vivo» (tablas 1 y 2 con su prosa) y «lo cerrado»
// (sección 3 hasta las reglas).
func partesDelRegistro(t *testing.T, texto string) (vivo, cerrado string) {
	t.Helper()
	i1 := regexp.MustCompile(`(?m)^## 1 ·`).FindStringIndex(texto)
	i3 := regexp.MustCompile(`(?m)^## 3 ·`).FindStringIndex(texto)
	i4 := regexp.MustCompile(`(?m)^## Cómo se usa`).FindStringIndex(texto)
	if i1 == nil || i3 == nil || i4 == nil || !(i1[0] < i3[0] && i3[0] < i4[0]) {
		t.Fatalf("no se encontraron las secciones «## 1 ·», «## 3 ·» y «## Cómo se usa» en orden; el barrido no está mirando donde cree")
	}
	return texto[i1[0]:i3[0]], texto[i3[0]:i4[0]]
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// EL REGISTRO NO CITA CÓDIGO QUE NO EXISTE
//
// Así se pudrieron A99, A102 y A116: los tres mandaban desplegar «migración 47» / «esquema 48» y
// las columnas que necesitaban vivían en la 50 y la 51. Un número de esquema viejo **no falla:
// tranquiliza**, porque una base parada en 47 tiene ese número y no tiene la columna.
//
// Y la tercera de la serie de `cerrarSesionesColgadas`: A109 nombraba
// `TestElTokenNuevoSePersisteAntesDeEstrenarse`, una función que no existe en el repo. Un doc que
// nombra código inexistente hace que nadie vaya a mirar lo que sí hay.
//
// Las dos son guardas baratas que evitan que el registro vuelva a mentir solo.
// ════════════════════════════════════════════════════════════════════════════════════════════

var (
	versionDeMigracion = regexp.MustCompile(`(?m)^\s*version:\s*(\d+),`)
	citaDeEsquema      = regexp.MustCompile(`(?i)\b(?:esquema|migraci[oó]n)e?s?\s+\*{0,2}(\d+)`)
	citaDePrueba       = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]{3,}`)
	definicionDePrueba = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\s*\(`)
)

// esquemaMaximoDelRepo lee las versiones de `internal/memory/migrations.go`. Lee el CAMPO que las
// declara, no un comentario ni una constante que alguien puede olvidar de subir.
func esquemaMaximoDelRepo(t *testing.T) int {
	t.Helper()
	crudo, err := os.ReadFile(filepath.Join("..", "..", "internal", "memory", "migrations.go"))
	if err != nil {
		t.Fatalf("no se pudo leer internal/memory/migrations.go: %v", err)
	}
	max := 0
	n := 0
	for _, m := range versionDeMigracion.FindAllStringSubmatch(string(crudo), -1) {
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		n++
		if v > max {
			max = v
		}
	}
	// UN CERO ACÁ SERÍA «NO PUDE MEDIR» CON CARA DE «MEDÍ Y ESTÁ BIEN». Si el struct cambia de
	// forma y el parseo no encuentra nada, esta guarda tiene que caerse, no dar por buena
	// cualquier cita de esquema del registro.
	if n < 40 {
		t.Fatalf("sólo se reconocieron %d migraciones en internal/memory/migrations.go y hay más de 40: cambió la forma del struct y este cruce dejó de medir", n)
	}
	return max
}

func TestElRegistroNoCitaEsquemasNiPruebasQueNoExisten(t *testing.T) {
	texto := registroDeAbiertos(t)
	max := esquemaMaximoDelRepo(t)

	// (1) NINGÚN NÚMERO DE ESQUEMA CITADO PUEDE SER MAYOR QUE EL QUE EXISTE.
	// El objetivo obsoleto lo caza `TestNingunaFilaDeLaTabla1SeDeclaraCerrada` sobre la columna
	// que decide; acá se caza el otro lado: un número inventado o adelantado.
	citados := 0
	for n, linea := range strings.Split(texto, "\n") {
		for _, m := range citaDeEsquema.FindAllStringSubmatch(linea, -1) {
			v, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			citados++
			if v > max {
				t.Errorf("ABIERTO.md:%d cita «%s» y la migración más alta del repo es **%d**.\n"+
					"  El número sale de `internal/memory/migrations.go`. Un esquema que el repo no tiene manda a verificar contra algo que no existe.",
					n+1, strings.TrimSpace(m[0]), max)
			}
		}
	}
	if citados < 10 {
		t.Fatalf("sólo se reconocieron %d citas de esquema/migración en ABIERTO.md y hay más de 10: cambió cómo el registro las escribe y este cruce dejó de medir", citados)
	}

	// (2) TODO NOMBRE DE PRUEBA CITADO EXISTE EN EL ÁRBOL.
	//
	// La excepción no es una lista de nombres: es que el párrafo DIGA que no existe. La única cita
	// fantasma que este archivo tiene es justamente la historia del error, y su párrafo dice «esa
	// función NO existe en el repo». Quien quiera citar un fantasma tiene que decir que lo es.
	// LA EXCEPCIÓN TIENE QUE ESTAR DONDE DECIDE, NO EN EL VECINO.
	//
	// Esto miraba «¿el PÁRRAFO dice "no existe"?», y una tabla de Markdown es UN párrafo: sus 31
	// filas no tienen renglón en blanco entre sí. La tabla 1 dice «no existe» seis veces —`load1`
	// **no existe** en Windows, `stty` que no existe en la consola, una unidad que en esta PC no
	// existe…—, todas hablando de otra cosa. Resultado medido el 2026-09-10: el cruce estaba
	// MUERTO para las 31 filas de la tabla 1, y citar un `TestQueNoExisteEnNingunLado` en la
	// columna que decide pasaba en VERDE. 28 de 75 citas del archivo quedaban exentas así.
	//
	// Es el defecto que este repo ya pagó dos veces: la guarda pregunta por un texto que no
	// decide nada, satisfecho por un comentario, un mensaje o el vecino. Ahora la exención se
	// pide en la UNIDAD donde vive la cita: si la línea es una fila de tabla, la celda; si no, la
	// oración. Un «no existe» a seis celdas de distancia ya no exime a nadie.
	reales := pruebasDelArbol(t)
	citadas := 0
	{
		for _, unidad := range unidadesDeCita(texto) {
			declaraAusencia := strings.Contains(strings.ToLower(unidad), "no existe")
			for _, nombre := range citaDePrueba.FindAllString(unidad, -1) {
				citadas++
				if reales[nombre] || declaraAusencia {
					continue
				}
				t.Errorf("ABIERTO.md cita `%s`, que no existe en el árbol.\n"+
					"  Es la tercera de la serie de `cerrarSesionesColgadas`: un doc que nombra código inexistente hace que nadie vaya a mirar lo que sí hay.\n"+
					"  O corregís el nombre, o —si estás contando la historia de un nombre que se fue— decilo EN LA MISMA CELDA (o la misma oración): «no existe». Un «no existe» en la fila de al lado ya no alcanza.", nombre)
			}
		}
	}
	if citadas < 30 {
		t.Fatalf("sólo se reconocieron %d citas de pruebas en ABIERTO.md y hay más de 30: cambió cómo el registro las escribe y este cruce dejó de medir", citadas)
	}
}

// unidadesDeCita parte el registro en las unidades dentro de las cuales una excepción vale.
//
// EL DEFECTO QUE ARREGLA, EXACTO: la exención se pedía «en el mismo párrafo», con los párrafos
// cortados por `\n\n`. **Una tabla de Markdown no tiene renglones en blanco**, así que las 31
// filas de la tabla 1 eran UN párrafo, y ese párrafo dice «no existe» seis veces hablando de otras
// cosas (`load1` no existe en Windows, `stty` no existe en la consola, una unidad que en esta PC
// no existe). Con eso el cruce de nombres de prueba estaba MUERTO para toda la tabla: 28 de las 75
// citas del archivo quedaban exentas por el vecino. Medido el 2026-09-10.
//
// La regla es entonces: **la prosa se agrupa por párrafo, la tabla por celda**. El párrafo sigue
// siendo la unidad de la prosa porque ahí es donde el registro escribe de verdad su excepción —la
// cita va en una oración y el «no existe» en la siguiente, y las dos citas legítimas de
// `TestElTokenNuevoSePersisteAntesDeEstrenarse` tienen esa forma—. Lo que deja de valer es que una
// celda exima a otra.
func unidadesDeCita(texto string) []string {
	var out []string
	var prosa []string
	cerrarProsa := func() {
		if len(prosa) > 0 {
			out = append(out, strings.Join(prosa, "\n"))
			prosa = nil
		}
	}
	for _, parrafo := range strings.Split(texto, "\n\n") {
		for _, linea := range strings.Split(parrafo, "\n") {
			if esFilaMD(linea) && !esSeparadorMD(linea) {
				if celdas := celdasDeFila(linea); celdas != nil {
					cerrarProsa()
					out = append(out, celdas...)
					continue
				}
			}
			prosa = append(prosa, linea)
		}
		cerrarProsa()
	}
	return out
}

// pruebasDelArbol junta los nombres de TODA función `TestXxx` del repo.
func pruebasDelArbol(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	raiz := filepath.Join("..", "..")
	err := filepath.WalkDir(raiz, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil // un directorio ilegible no puede dar por buena una cita
		}
		if e.IsDir() {
			switch e.Name() {
			case ".git", "node_modules", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(e.Name(), "_test.go") {
			return nil
		}
		crudo, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, m := range definicionDePrueba.FindAllStringSubmatch(string(crudo), -1) {
			out[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no se pudo caminar el repo buscando pruebas: %v", err)
	}
	// CERO PRUEBAS ENCONTRADAS SERÍA «NO PUDE MIRAR» CON CARA DE «TODAS LAS CITAS SON FALSAS».
	if len(out) < 500 {
		t.Fatalf("sólo se encontraron %d funciones Test en el árbol; el recorrido no está mirando donde cree", len(out))
	}
	return out
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LAS GUARDAS QUE CUSTODIAN LOS NÚMEROS
//
// Una auditoría del 2026-09-02 encontró que «¿el número al que apunta significa algo?» tampoco se
// cumplía, y por dos caminos que a mano no se ven porque las filas están lejos una de otra:
//
//   - **A33** se citaba DOS VECES como decisión pendiente y no tenía fila en ninguna tabla. Se lo
//     había convertido en `B20` cuatro días antes y nadie escribió la conversión.
//   - `B13` nombraba TRES decisiones distintas y `B14` dos. «Se revisa en B13» dejaba de
//     identificar cuál.
//
// POR QUÉ SÓLO SE MIRA LA PARTE VIVA: la regla 1 manda BORRAR la fila cuando un cabo se cierra, así
// que la sección 3 nombra decenas de números que —correctamente— ya no tienen fila.
// ════════════════════════════════════════════════════════════════════════════════════════════

var numeroA = regexp.MustCompile(`\bA\d+\b`)

// filasVivas devuelve las filas de las tablas 1 y 2, y las líneas de la parte viva para poder
// citar prosa que cuelga de una fila.
func filasVivas(t *testing.T, texto string) (map[string]int, []string) {
	t.Helper()
	vivo, _ := partesDelRegistro(t, texto)
	ids := map[string]int{}
	for _, tab := range parseTablasMD(vivo) {
		for _, f := range tab.filas {
			if idDeRegistro.MatchString(f.id()) {
				if _, ya := ids[f.id()]; !ya {
					ids[f.id()] = 0
				}
				ids[f.id()]++
			}
		}
	}
	return ids, strings.Split(vivo, "\n")
}

// TestNingunNumeroDeRegistroSeUsaDosVeces: un número repetido es peor que uno faltante, porque no
// se nota. Nadie lee las dos tablas de corrido buscando choques.
//
// Sabotaje que la hace fallar: duplicar cualquier fila de la tabla 2 con el número de otra.
func TestNingunNumeroDeRegistroSeUsaDosVeces(t *testing.T) {
	ids, _ := filasVivas(t, registroDeAbiertos(t))
	if len(ids) < 20 {
		t.Fatalf("sólo se reconocieron %d filas en las tablas de ABIERTO.md; cambió el formato y esta guarda dejó de mirar", len(ids))
	}
	orden := make([]string, 0, len(ids))
	for k := range ids {
		orden = append(orden, k)
	}
	sort.Strings(orden)
	for _, num := range orden {
		if ids[num] > 1 {
			t.Errorf("**%s** nombra %d filas distintas de ABIERTO.md.\n  Un número repetido no identifica nada: «se revisa en %s» deja de decir cuál.\n  Dale un número LIBRE a las repetidas —por encima del máximo en uso— y anotá en la fila por qué cambió.",
				num, ids[num], num)
		}
	}
}

// TestUnCaboVivoNoApuntaAUnNumeroQueElRegistroNoDefine: si una fila viva dice «esto lo decide A33»
// y A33 no está definido en ningún lado del archivo, el lector se queda sin el hilo justo donde el
// registro prometía tenerlo.
//
// Sabotaje que la hace fallar: citar **A999** en cualquier fila de las tablas.
func TestUnCaboVivoNoApuntaAUnNumeroQueElRegistroNoDefine(t *testing.T) {
	texto := registroDeAbiertos(t)
	conFila, vivo := filasVivas(t, texto)
	if len(conFila) < 20 {
		t.Fatalf("sólo se reconocieron %d filas en las tablas de ABIERTO.md; cambió el formato y esta guarda dejó de mirar", len(conFila))
	}

	// CONTROL POSITIVO. Sin esto, aflojar un patrón de `estaDefinido` —o escribirlo tan ancho que
	// matchee cualquier cosa— dejaría la prueba en verde para siempre sin mirar nada.
	if estaDefinido(texto, "A9999") {
		t.Fatal("`estaDefinido` da por definido un número que no existe: los patrones se aflojaron y esta prueba ya no puede fallar")
	}

	visto := map[string]bool{}
	for n, linea := range vivo {
		for _, num := range numeroA.FindAllString(linea, -1) {
			if _, tiene := conFila[num]; tiene || visto[num] || estaDefinido(texto, num) {
				visto[num] = true
				continue
			}
			visto[num] = true
			t.Errorf("línea %d de la parte viva de ABIERTO.md cita **%s**, que el registro no define en ningún lado.\n    %s\n  O le das su fila en la tabla 1 o 2, o —si se cerró o se convirtió en otro número— decilo donde se cerró: «%s CERRADO», «cierra %s», «(era %s)».",
				n+1, num, recortar(linea, 120), num, num, num)
		}
	}
}

// estaDefinido dice si el archivo DEFINE ese número en algún lado, y no sólo lo menciona.
//
// Las ocho formas son las que el registro ya usa; no se inventó ninguna. Están acá y no inline
// para que agregar una novena sea una decisión visible, con su comentario, en vez de un patrón más
// ancho que apaga la prueba de a poco.
func estaDefinido(texto, num string) bool {
	q := regexp.QuoteMeta(num)
	for _, patron := range []string{
		`(?m)^\|\s*\*{0,2}` + q + `\*{0,2}\s*\|`,              // su propia fila en la tabla 1 o 2
		`(?i)\b` + q + `\*{0,2} +cerrad[oa]s?\b`,              // «A70 CERRADO», «A44 cerrado»
		`(?i)\bcierran? +\*{0,2}` + q + `\b`,                  // «S6b … cierra A13»
		`(?i)\bera +\*{0,2}` + q + `\b`,                       // «(era A61)» — lo absorbió otro número
		q + `\*{0,2} *→`,                                      // «A22 → B13»
		`(?i)\bregistrado como +\*{0,2}` + q + `\b`,           // «Registrado como **A56**»
		`(?m)^\*\*20\d\d-\d\d-\d\d[^*\n]{0,60}· +` + q + `\b`, // encabezado de entrada, que acá siempre abre con la fecha
		`(?m)^[-·*] +\*\*` + q + ` *—`,                        // viñeta que DEFINE: el número y enseguida la raya
	} {
		if regexp.MustCompile(patron).MatchString(texto) {
			return true
		}
	}
	return false
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// TODA FILA DE UNA TABLA TIENE LAS CELDAS QUE SU ENCABEZADO DECLARA.
//
// Lo encontró una fila real, no una hipótesis: `B20` tenía CUATRO celdas —terminaba en
// `| **decidido** |`— en una tabla cuyo encabezado declara TRES. Markdown no falla con una celda
// de más: la DESCARTA. Así que lo que se caía al renderizar era exactamente la palabra que esa
// fila existe para decir, y donde la gente LEE el registro esa fila se veía igual que una sin
// resolver.
//
// Sabotaje que la hace fallar: devolverle a B20 su ` | **decidido** |`, quitarle una celda a
// cualquier fila, quitarle la barra de cierre a una fila que no sea la última de su tabla, o meter
// una línea en blanco entre dos filas.
// ════════════════════════════════════════════════════════════════════════════════════════════

func TestTodaFilaDeAbiertoTieneLasCeldasDeSuEncabezado(t *testing.T) {
	crudo := registroDeAbiertos(t)
	lineas := strings.Split(crudo, "\n")

	// `| a | b |` son 2 celdas: se cuentan los separadores internos, que es lo que usa el
	// renderizador para decidir dónde corta.
	//
	// UN `\|` NO ES UN SEPARADOR, y contarlo como tal fue un falso POSITIVO medido el 2026-09-05:
	// la fila de A98 llevaba `Get-Process musubi \| Select-Object`.
	celdas := func(l string) int { return len(celdasDeFila(l)) }

	tablas, filasVistas := 0, 0
	revisadas := map[int]bool{}
	esperadas, encabezadoEn := 0, 0
	for i, l := range lineas {
		switch {
		case esSeparadorMD(l):
			// El encabezado es la línea de ARRIBA. Si no era una fila, esto no es una tabla.
			if i > 0 && esFilaMD(lineas[i-1]) {
				esperadas, encabezadoEn = celdas(lineas[i-1]), i
				tablas++
			} else {
				esperadas = 0
			}
		case esperadas > 0 && esFilaMD(l):
			filasVistas++
			revisadas[i] = true
			if n := celdas(l); n != esperadas {
				t.Errorf("ABIERTO.md línea %d: la fila tiene %d celdas y su encabezado (línea %d) "+
					"declara %d.\n    %s\n  Markdown DESCARTA la celda de más sin avisar, así que el "+
					"dato que sobra no se ve en ningún lado: no es un detalle de formato, es una "+
					"celda que existe en el archivo y no existe para quien lo lee. Pliegala en la "+
					"columna que corresponda, o dale a la tabla la columna que le falta.",
					i+1, n, encabezadoEn, esperadas, recorte(strings.TrimSpace(l), 110))
			}
		case esperadas > 0 && strings.TrimSpace(l) == "":
			// UNA LÍNEA EN BLANCO ADENTRO DE UNA TABLA LA PARTE EN DOS, Y ESO ES PEOR QUE UNA CELDA
			// PERDIDA: en Markdown la tabla TERMINA ahí, así que las filas que siguen se renderizan
			// como texto suelto con barras verticales. Y para esta prueba era peor todavía: al ver
			// el blanco daba la tabla por terminada y dejaba de revisar.
			//
			// Medido el 2026-09-05: había NUEVE líneas en blanco adentro de la tabla de la sección 1.
			//
			// Un blanco ANTES de un encabezado o del final de la sección sí es legítimo. La
			// diferencia es si DESPUÉS del blanco siguen viniendo filas.
			siguen := false
			for j := i + 1; j < len(lineas); j++ {
				if strings.TrimSpace(lineas[j]) == "" {
					continue
				}
				siguen = esFilaMD(lineas[j]) && !esSeparadorMD(lineas[j])
				break
			}
			if siguen {
				t.Errorf("ABIERTO.md línea %d: hay una línea EN BLANCO adentro de la tabla que empieza "+
					"en la línea %d, y después del blanco siguen viniendo filas.\n"+
					"  Markdown TERMINA la tabla en el blanco, así que todo lo que sigue se dibuja como "+
					"texto suelto con barras verticales en vez de como filas — el registro entero deja de "+
					"leerse. Sacá la línea en blanco.", i+1, encabezadoEn)
			}
			esperadas = 0
		case esperadas > 0:
			esperadas = 0 // se terminó la tabla
		}
	}

	// CONTROL DE QUE MIRÓ ALGO: si cambiara el formato del archivo, los reconocedores dejarían de
	// matchear y esta prueba pasaría en verde sin haber contado una sola celda.
	if tablas < 3 || filasVistas < 30 {
		t.Fatalf("se reconocieron %d tabla(s) y %d fila(s) en ABIERTO.md, y son al menos 3 y 30: "+
			"cambió el formato del archivo y esta guarda dejó de mirar", tablas, filasVistas)
	}

	// CONTROL DE COBERTURA: TODA FILA CON IDENTIFICADOR TIENE QUE HABER SIDO REVISADA.
	//
	// «3 tablas y 30 filas» NO ALCANZABA, y se midió el 2026-09-05: la tabla de la sección 1 estaba
	// PARTIDA en la fila de A92, así que las DIECISÉIS filas siguientes no se revisaban y el control
	// seguía en verde porque contaba contra un número fijo ya superado.
	//
	// Este control cuenta lo revisado contra lo que el archivo TIENE. Y la identificación de una
	// fila ya NO es un regex de forma: un espacio de padding de más apagaba la cuenta entera.
	var sinRevisar []string
	primera := 0
	for i, l := range lineas {
		c := celdasDeFila(l)
		if len(c) == 0 || !idDeRegistro.MatchString(sinEnfasis(c[0])) || revisadas[i] {
			continue
		}
		if primera == 0 {
			primera = i + 1
		}
		sinRevisar = append(sinRevisar, sinEnfasis(c[0]))
	}
	if len(sinRevisar) > 0 {
		t.Errorf("ABIERTO.md: %d fila(s) con número de registro NO se revisaron: %v.\n"+
			"  La primera está en la línea %d. El recorrido dio la tabla por terminada antes de "+
			"llegar, y en Markdown esas filas TAMPOCO se dibujan como tabla: se ven como texto suelto "+
			"con barras verticales, así que el registro deja de leerse justo en lo más reciente.\n"+
			"  Mirá la fila ANTERIOR a la primera que falta: o le falta la barra de cierre, o hay un "+
			"blanco, un blockquote o prosa metidos adentro de la tabla. Una corrección va PLEGADA "+
			"adentro de la celda de su fila, nunca como bloque suelto entre filas.",
			len(sinRevisar), sinRevisar, primera)
	}
}
