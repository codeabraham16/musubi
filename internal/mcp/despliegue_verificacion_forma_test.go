package mcp

// despliegue_verificacion_forma_test.go — RONDA 2 del cabo del checksum.
//
// LA RONDA 1 CERRÓ EL FAIL-OPEN. ESTE ARCHIVO CIERRA LOS AGUJEROS DE LA GUARDA QUE LO CERRÓ.
//
// Las pruebas de `despliegue_checksum_test.go` corren el bloque que decide y miran si el archivo
// apareció en el destino. Eso es lo correcto — pero mide UNA corrida, con UN entorno: el de la
// prueba. Un saboteador midió que se podían reabrir los fail-open sin poner nada en rojo:
//
//  1. envolver TODA la verificación del paso 1 en `if [ -z "${MUSUBI_SIN_VERIFICAR:-}" ]` con un
//     `else` que sólo loguea. La prueba no exporta esa variable, así que corre por la rama que
//     verifica y da verde. Lo mismo en el relay de RustDesk, donde hbbs/hbbr quedan como unidades
//     systemd. Y los tres lugares donde el guion AFIRMA por escrito que «NO existe una variable
//     para SALTEAR la verificación» —la cabecera, el comentario del paso 1 y el commit— seguían
//     diciéndolo con CERO código detrás. Un doc que miente es peor que uno que falta: el que lo
//     lee deja de buscar.
//  2. borrar la comparación y el `die` de los pasos 5b y 5c, dejando el `sha256sum` y el pin: el
//     guion de backup —que un timer corre como el usuario del cerebro, con el token cargado— y el
//     de redespliegue —que reemplaza el binario del cerebro como root— se instalan con lo que
//     venga. Sus «custodios» comparan que el pin coincida con el archivo: ven cambios de BYTES, no
//     que la comprobación siga existiendo.
//  3. borrar los dos `die` del instalador de Prometheus dejando el `curl`, el `awk` y el
//     `ok "Checksum verificado"`: imprime que verificó sin verificar nada.
//  4. hacer que `redesplegar-cerebro.sh` ADOPTE como sha esperado el del binario que le pasan
//     cuando no se lo dan: la comparación queda vacuamente cierta, que es A111 escrito otra vez.
//
// QUÉ MIRA ESTE ARCHIVO Y POR QUÉ NO ES UN GREP. Ninguna guarda de acá pregunta por un texto: ni
// por `MUSUBI_SIN_VERIFICAR` (el próximo se llama distinto), ni por la palabra `die`, ni por el
// mensaje del `ok`. Se derivan del guion tres cosas mecánicas:
//
//   - QUÉ SE MIDE: las variables que el bloque asigna desde un `sha256sum`. Ese es el número real.
//   - QUIÉN LO COMPARA: las líneas de test (`[`, `[[`, `test`) que nombran esa variable.
//   - QUIÉN LO CONSUME: las líneas que corren `install`.
//
// y se exige que la comparación DOMINE al consumo: que esté antes y bajo el mismo conjunto de
// condiciones —o menos—, de modo que no exista camino que llegue al `install` sin haber pasado por
// ella. Envolver la verificación en un `if` nuevo rompe la dominancia aunque el `if` venga apagado
// por default; borrar la comparación deja el consumo sin dominador; y comparar dos variables que
// salen las DOS del `sha256sum` del mismo archivo es la comparación vacua del punto 4.
//
// Y ADEMÁS SE EJECUTA (despliegue_verificacion_corrida_test.go): cada bloque corre de verdad con
// `curl` de mentira, y una barrida prueba una por una TODAS las variables de entorno que el propio
// bloque lee y que nadie le asigna —la lista sale del guion, no de acá— para que ninguna pueda
// convertir un camino que frena en uno que instala.

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL LECTOR DE FORMA DE BASH.
//
// No es un parser completo: reconoce las palabras reservadas que abren y cierran bloques, y lo
// hace sobre TOKENS —no sobre el texto—, así que un `if` adentro de un mensaje entrecomillado, de
// un comentario o de un heredoc no cuenta. Eso es justamente lo que hace que estas guardas no las
// satisfaga un texto que no decide, que es el defecto que este repo repite.
//
// Y cuando NO entiende lo que lee —comillas sin cerrar, bloques sin balancear— devuelve error y la
// prueba muere. «No pude medir» y «medí y está bien» no pueden ser el mismo verde.
// ─────────────────────────────────────────────────────────────────────────────────────────────

// tokenDeGuion es un token con la línea (1-based, relativa al bloque) en la que empieza.
type tokenDeGuion struct {
	linea int
	tok   string
}

// reHeredoc casa `<<EOF`, `<<-EOF`, `<<'PY'` y `<<"UNIT"`. RE2 no tiene retro-referencias, así
// que las tres formas van como alternativas y se toma el grupo que haya casado.
var reHeredoc = regexp.MustCompile(`^<<-?(?:'([A-Za-z_][A-Za-z0-9_]*)'|"([A-Za-z_][A-Za-z0-9_]*)"|([A-Za-z_][A-Za-z0-9_]*))$`)

func delimitadorDeHeredoc(tok string) (string, bool) {
	m := reHeredoc.FindStringSubmatch(tok)
	if m == nil {
		return "", false
	}
	for _, g := range m[1:] {
		if g != "" {
			return g, true
		}
	}
	return "", false
}

// tokenizarBloque parte un tramo de bash en tokens. Trata como UN token cada palabra
// entrecomillada (aunque abarque varias líneas), cada `$(...)`, cada `${...}` y cada “ `...` “;
// descarta comentarios y cuerpos de heredoc; y emite los operadores de control (`;` `;;` `&&`
// `||` `|` `&`) por separado, que son los que abren "posición de comando".
func tokenizarBloque(bloque string) ([]tokenDeGuion, error) {
	var salida []tokenDeGuion
	var actual strings.Builder
	linea, lineaTok := 1, 1
	r := []rune(bloque)
	var heredocs []string // delimitadores pendientes para el próximo salto de línea

	cerrar := func() {
		if actual.Len() > 0 {
			t := actual.String()
			salida = append(salida, tokenDeGuion{lineaTok, t})
			actual.Reset()
			if d, ok := delimitadorDeHeredoc(t); ok {
				heredocs = append(heredocs, d)
			}
		}
	}
	abrir := func() {
		if actual.Len() == 0 {
			lineaTok = linea
		}
	}
	// saltarHeredocs consume el cuerpo de los heredocs pendientes. Devuelve el índice después del
	// último delimitador.
	saltarHeredocs := func(i int) (int, error) {
		for len(heredocs) > 0 {
			delim := heredocs[0]
			heredocs = heredocs[1:]
			for {
				j := i
				for j < len(r) && r[j] != '\n' {
					j++
				}
				fila := strings.TrimSpace(string(r[i:j]))
				i = j
				if i < len(r) {
					i++
					linea++
				}
				if fila == delim {
					break
				}
				if j >= len(r) {
					return i, errorForma{"un heredoc <<" + delim + " no se cierra dentro del bloque"}
				}
			}
		}
		return i, nil
	}

	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\n':
			cerrar()
			linea++
			if len(heredocs) > 0 {
				j, err := saltarHeredocs(i + 1)
				if err != nil {
					return nil, err
				}
				i = j - 1
			}
		case c == '#' && actual.Len() == 0:
			for i < len(r) && r[i] != '\n' {
				i++
			}
			i-- // que el '\n' lo procese la vuelta siguiente
		case c == ' ' || c == '\t' || c == '\r':
			cerrar()
		case c == '\\' && i+1 < len(r):
			abrir()
			actual.WriteRune(c)
			i++
			if r[i] == '\n' {
				linea++ // continuación de línea: no corta el comando
			}
			actual.WriteRune(r[i])
		case c == '\'' || c == '"' || c == '`':
			abrir()
			comilla := c
			actual.WriteRune(c)
			cerrada := false
			for i++; i < len(r); i++ {
				if r[i] == '\n' {
					linea++
				}
				if comilla != '\'' && r[i] == '\\' && i+1 < len(r) {
					actual.WriteRune(r[i])
					i++
					if r[i] == '\n' {
						linea++
					}
					actual.WriteRune(r[i])
					continue
				}
				actual.WriteRune(r[i])
				if r[i] == comilla {
					cerrada = true
					break
				}
			}
			if !cerrada {
				return nil, errorForma{"una comilla " + string(comilla) + " abierta en la línea " +
					strconv.Itoa(lineaTok) + " no se cierra dentro del bloque"}
			}
		case c == '$' && i+1 < len(r) && (r[i+1] == '(' || r[i+1] == '{'):
			abrir()
			abre, cierra := r[i+1], '}'
			if abre == '(' {
				cierra = ')'
			}
			nivel, cerrada := 0, false
			for ; i < len(r); i++ {
				if r[i] == '\n' {
					linea++
				}
				actual.WriteRune(r[i])
				if r[i] == '\'' || r[i] == '"' { // los paréntesis de adentro de una cadena no cuentan
					q := r[i]
					for i++; i < len(r); i++ {
						if r[i] == '\n' {
							linea++
						}
						actual.WriteRune(r[i])
						if r[i] == q {
							break
						}
					}
					continue
				}
				if r[i] == abre {
					nivel++
				} else if r[i] == cierra {
					nivel--
					if nivel == 0 {
						cerrada = true
						break
					}
				}
			}
			if !cerrada {
				return nil, errorForma{"una sustitución abierta en la línea " + strconv.Itoa(lineaTok) +
					" no se cierra dentro del bloque"}
			}
		case c == ';':
			cerrar()
			lineaTok = linea
			if i+1 < len(r) && r[i+1] == ';' {
				salida = append(salida, tokenDeGuion{linea, ";;"})
				i++
			} else {
				salida = append(salida, tokenDeGuion{linea, ";"})
			}
		case c == '&' || c == '|':
			// `&>` y `>&` son redirecciones, no operadores de control; `&&`/`||`/`|`/`&` sí.
			if c == '&' && i+1 < len(r) && r[i+1] == '>' {
				abrir()
				actual.WriteRune(c)
				continue
			}
			cerrar()
			lineaTok = linea
			if i+1 < len(r) && r[i+1] == c {
				salida = append(salida, tokenDeGuion{linea, string([]rune{c, c})})
				i++
			} else {
				salida = append(salida, tokenDeGuion{linea, string(c)})
			}
		default:
			abrir()
			actual.WriteRune(c)
		}
	}
	cerrar()
	return salida, nil
}

// abrePosicionDeComando dice si, después de este token, el siguiente arranca un comando nuevo.
func abrePosicionDeComando(tok string) bool {
	switch tok {
	case ";", ";;", "&&", "||", "|", "&", "then", "else", "do", "{", "(", "!", "if", "elif", "while", "until":
		return true
	}
	return false
}

// reDefinicionDeFuncion casa `nombre(){`, `nombre()` y `nombre() {` — la forma en que estos
// guiones definen log/ok/die/aviso/volver_atras.
var reDefinicionDeFuncion = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*\(\)\{?$`)

// marco es un bloque abierto. `apertura` guarda con qué palabra se abrió, para que un `}` no
// cierre un `if` ni un `fi` cierre un `case`.
type marcoBash struct {
	apertura string
	id       string
	linea    int
}

// lineaDeGuion es una línea del bloque con la PILA DE RAMAS que la encierra. La pila identifica
// cada rama por separado: el `then` de un `if` y su `else` son dos marcos distintos, así que una
// comparación que vive en uno NO domina a un `install` que vive en el otro.
type lineaDeGuion struct {
	n      int // 1-based dentro del bloque
	texto  string
	tokens []string
	pila   []string
}

// leerFormaDelBloque calcula la pila de ramas de cada línea del bloque.
func leerFormaDelBloque(bloque string) ([]lineaDeGuion, error) {
	toks, err := tokenizarBloque(bloque)
	if err != nil {
		return nil, err
	}
	filas := strings.Split(bloque, "\n")
	lineas := make([]lineaDeGuion, len(filas))
	for i, f := range filas {
		lineas[i] = lineaDeGuion{n: i + 1, texto: f}
	}

	var pila []marcoBash
	enPila := func() []string {
		out := make([]string, len(pila))
		for i, m := range pila {
			out[i] = m.id
		}
		return out
	}
	// A cada línea se le atribuye la pila con la que EMPIEZA. Las líneas sin tokens —comentarios,
	// vacías, cuerpos de heredoc— se llevan la pila vigente en ese punto, que es la única lectura
	// que no miente cuando después hay que preguntar «¿bajo qué condiciones vive esta línea?».
	hastaLinea := 0
	anotarHasta := func(n int) {
		p := enPila()
		for i := hastaLinea; i < n && i < len(lineas); i++ {
			lineas[i].pila = p
		}
		if n > hastaLinea {
			hastaLinea = n
		}
	}
	// fijar reescribe la pila de una línea que empieza CERRANDO un bloque: el `fi` vive afuera del
	// `if` que cierra.
	fijar := func(n int) {
		if n >= 1 && n <= len(lineas) && len(lineas[n-1].tokens) <= 1 {
			lineas[n-1].pila = enPila()
		}
	}

	comando := true
	lineaPrevia := 0
	for _, t := range toks {
		if t.linea != lineaPrevia {
			comando = true // un salto de línea termina el comando anterior
			lineaPrevia = t.linea
		}
		anotarHasta(t.linea)
		if t.linea >= 1 && t.linea <= len(lineas) {
			lineas[t.linea-1].tokens = append(lineas[t.linea-1].tokens, t.tok)
		}
		cierra := func(esperado ...string) error {
			if len(pila) == 0 {
				return errorForma{"línea " + strconv.Itoa(t.linea) + ": `" + t.tok +
					"` cierra un bloque que nadie abrió"}
			}
			top := pila[len(pila)-1].apertura
			ok := false
			for _, e := range esperado {
				if top == e {
					ok = true
				}
			}
			if !ok {
				return errorForma{"línea " + strconv.Itoa(t.linea) + ": `" + t.tok + "` cierra un `" +
					top + "` abierto en la línea " + strconv.Itoa(pila[len(pila)-1].linea)}
			}
			pila = pila[:len(pila)-1]
			fijar(t.linea)
			return nil
		}
		switch {
		case t.tok == ")" || t.tok == "}":
			// Se cierran sin importar la posición: `( cd x && y )` termina con el `)` pegado a un
			// argumento, y el cuerpo de una función con un `}` solo.
			esperada := "("
			if t.tok == "}" {
				esperada = "{"
			}
			if len(pila) > 0 && pila[len(pila)-1].apertura == esperada {
				if err := cierra(esperada); err != nil {
					return nil, err
				}
			}
		case !comando:
			// nada: sólo las palabras en posición de comando son reservadas
		case t.tok == "if" || t.tok == "case" || t.tok == "for" || t.tok == "while" ||
			t.tok == "until" || t.tok == "select" || t.tok == "{" || t.tok == "(":
			fijar(t.linea)
			pila = append(pila, marcoBash{apertura: t.tok, linea: t.linea,
				id: "L" + strconv.Itoa(t.linea) + ":" + t.tok})
		case reDefinicionDeFuncion.MatchString(t.tok):
			fijar(t.linea)
			if strings.HasSuffix(t.tok, "{") {
				pila = append(pila, marcoBash{apertura: "{", linea: t.linea,
					id: "L" + strconv.Itoa(t.linea) + ":func"})
			}
		case t.tok == "fi":
			if err := cierra("if"); err != nil {
				return nil, err
			}
		case t.tok == "esac":
			if err := cierra("case"); err != nil {
				return nil, err
			}
		case t.tok == "done":
			if err := cierra("for", "while", "until", "select"); err != nil {
				return nil, err
			}
		case t.tok == "else" || t.tok == "elif":
			if len(pila) == 0 || pila[len(pila)-1].apertura != "if" {
				return nil, errorForma{"línea " + strconv.Itoa(t.linea) + ": un `" + t.tok + "` sin `if`"}
			}
			fijar(t.linea)
			top := &pila[len(pila)-1]
			top.id = "L" + strconv.Itoa(top.linea) + ":" + t.tok + "@" + strconv.Itoa(t.linea)
		}
		comando = abrePosicionDeComando(t.tok)
	}
	if len(pila) != 0 {
		var abiertos []string
		for _, m := range pila {
			abiertos = append(abiertos, m.apertura+" de la línea "+strconv.Itoa(m.linea))
		}
		return nil, errorForma{"el bloque termina sin cerrar: " + strings.Join(abiertos, ", ")}
	}
	anotarHasta(len(lineas) + 1) // las líneas del final que no tenían ningún token
	return lineas, nil
}

type errorForma struct{ msg string }

func (e errorForma) Error() string { return e.msg }

// dominaA dice si la pila `a` es un prefijo de `b`: o sea, si todo camino que llega a `b` pasó
// antes por `a`. Es la forma exacta de «el install es inalcanzable sin haber comparado».
func dominaA(a, b []string) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var (
	reAsignaDeSha   = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=.*\bsha256sum\b`)
	reReferenciaVar = regexp.MustCompile(`\$\{?#?([A-Za-z_][A-Za-z0-9_]*)`)
	reAsignacionVar = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)(=|\+=|\[)`)
)

func esComandoDeTest(tok string) bool { return tok == "[" || tok == "[[" || tok == "test" }

func esSalidaQueFrena(tok string) bool {
	return tok == "die" || tok == "exit" || tok == "volver_atras"
}

// nombraLaVariable dice si el token nombra a esa variable ($v, ${v}, "${v:-x}"...). Se compara el
// NOMBRE extraído, no una subcadena: `$got` no se satisface con `$gotcha`.
func nombraLaVariable(tok, v string) bool {
	for _, m := range reReferenciaVar.FindAllStringSubmatch(tok, -1) {
		if m[1] == v {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────────────────────

// bloqueVerificado describe un tramo de guion que trae un artefacto de afuera y lo deja instalado.
type bloqueVerificado struct {
	rel    string // ruta relativa a deploy/
	nombre string
	desde  string
	hasta  string
	// consumo: los comandos cuya ejecución significa «el artefacto quedó puesto». Se buscan como
	// TOKEN, no como subcadena: la palabra `install` adentro de un comentario o de un mensaje no
	// cuenta, que es exactamente el error que este repo repite.
	consumo []string
}

// losBloquesQueBajanEInstalan — la lista de los HERMANOS, en un solo lugar.
//
// Todo lo que este repo trae de afuera y deja puesto en un servidor está acá. Si aparece un
// sexto, agregarlo es una línea; no agregarlo deja el mismo agujero abierto en el guion nuevo,
// que es la forma en que este repo pierde: la lección aprendida de un lado y no del hermano.
func losBloquesQueBajanEInstalan() []bloqueVerificado {
	return []bloqueVerificado{
		{
			rel: "install-musubi-brain.sh", nombre: "paso 1 — el binario del cerebro",
			desde: "# ── 1. Binario", hasta: "# ── 2. Workspace",
			consumo: []string{"install"},
		},
		{
			rel: "install-musubi-brain.sh", nombre: "paso 5b — musubi-backup.sh",
			desde: "# ── 5b. Backup programado", hasta: "# ── 5c. El guion de REDESPLIEGUE",
			consumo: []string{"install"},
		},
		{
			rel: "install-musubi-brain.sh", nombre: "paso 5c — redesplegar-cerebro.sh",
			desde: "# ── 5c. El guion de REDESPLIEGUE", hasta: "# ── 6. Firewall",
			consumo: []string{"install"},
		},
		{
			rel: "prometheus/install-musubi-prometheus.sh", nombre: "paso 2 — los binarios de Prometheus",
			desde: "# ── 2. Binarios de Prometheus", hasta: "# ── 3. Directorios",
			consumo: []string{"install"},
		},
		{
			rel: "rustdesk/install-rustdesk-relay.sh", nombre: "hbbs/hbbr del relay de RustDesk",
			desde: "# ── Binarios", hasta: "# ── systemd",
			consumo: []string{"install"},
		},
		{
			// El redespliegue no baja nada: recibe el binario y el sha por argumento. Pero el
			// `install` que reemplaza /usr/local/bin/musubi tiene que estar dominado igual, y su
			// comparación es la que el sabotaje de la ronda 2 dejó vacuamente cierta.
			rel: "redesplegar-cerebro.sh", nombre: "el reemplazo del binario del cerebro",
			desde: `NUEVO="${1:-}"`, hasta: "# ── VERIFICAR",
			consumo: []string{"install"},
		},
	}
}

// TestNingunInstallEsAlcanzableSinHaberComparadoElSha — la guarda estructural.
//
// EL AGUJERO QUE CIERRA: una vía de escape agregada a mano (una variable de entorno nueva, una
// bandera, un archivo centinela) que envuelva la verificación en un `if` con `else`. La prueba que
// EJECUTA el bloque corre por la rama de siempre y no la ve. Acá no hace falta que la vía se
// active: alcanza con que exista, porque parte el camino en dos y sólo uno compara.
//
// POR QUÉ NO ES UN GREP DE `MUSUBI_SIN_VERIFICAR`: el próximo se va a llamar distinto. Una lista
// de nombres malos siempre le falta el que viene. Lo que se mira es la FORMA —quién compara el
// número medido y bajo qué condiciones llega el `install`— y esa no depende del nombre.
func TestNingunInstallEsAlcanzableSinHaberComparadoElSha(t *testing.T) {
	comprobados := 0
	for _, b := range losBloquesQueBajanEInstalan() {
		t.Run(b.rel+" — "+b.nombre, func(t *testing.T) {
			guion := leerGuionDeDespliegue(t, b.rel)
			bloque := bloqueEntreMarcas(t, guion, b.rel, b.desde, b.hasta)
			lineas, err := leerFormaDelBloque(bloque)
			if err != nil {
				t.Fatalf("no pude leer la forma del bloque %q de deploy/%s: %v.\n"+
					"Un análisis que no entendió la forma NO puede decir que está bien: esto es "+
					"«no pude medir», y tiene que verse distinto de «medí y está bien»", b.nombre, b.rel, err)
			}

			// 1. QUÉ SE MIDE: las variables que salen de un `sha256sum`. Ese es el número real del
			//    artefacto; todo lo demás es lo que alguien DICE que tendría que ser.
			medido := map[string]int{} // nombre → línea donde se asigna
			for _, l := range lineas {
				for _, tok := range l.tokens {
					if m := reAsignaDeSha.FindStringSubmatch(tok); m != nil {
						medido[m[1]] = l.n
					}
				}
			}
			if len(medido) == 0 {
				t.Fatalf("el bloque %q de deploy/%s no calcula el sha256 de nada (no hay ninguna "+
					"asignación desde `sha256sum`).\nO se quitó la medición —y entonces se instala lo "+
					"que venga— o el bloque cambió de forma y esta guarda dejó de mirar donde se decide.",
					b.nombre, b.rel)
			}

			// 2. QUIÉN LO COMPARA: líneas de test que nombran una variable medida.
			var comparaciones []lineaDeGuion
			for _, l := range lineas {
				hayTest := false
				for _, tok := range l.tokens {
					if esComandoDeTest(tok) {
						hayTest = true
					}
				}
				if !hayTest {
					continue
				}
				for v := range medido {
					if lineaNombra(l, v) {
						comparaciones = append(comparaciones, l)
						break
					}
				}
			}
			if len(comparaciones) == 0 {
				t.Fatalf("el bloque %q de deploy/%s CALCULA el sha256 y no lo compara con nada.\n"+
					"Es el sabotaje medido: se borran la comparación y el `die` y quedan el `curl`, el "+
					"`sha256sum` y el `ok \"checksum verificado\"` — el guion IMPRIME que verificó sin "+
					"haber verificado nada.\nVariables medidas y nunca comparadas: %v",
					b.nombre, b.rel, clavesMedidas(medido))
			}

			// 3. LA COMPARACIÓN NO PUEDE SER VACUA. Si los dos lados salen del `sha256sum` del mismo
			//    archivo, comparar es cierto siempre. Es A111 exacto: `[[ "$ESQUEMA" -ge 37 ]]` con la
			//    base en 46 pasó seis redespliegues sin comprobar nada, y `SHA_ESPERADO` adoptando el
			//    sha del binario que le pasan es la misma figura sobre el checksum.
			for _, c := range comparaciones {
				cuantosMedidos := 0
				for v := range medido {
					if lineaNombra(c, v) {
						cuantosMedidos++
					}
				}
				if cuantosMedidos > 1 {
					t.Errorf("en deploy/%s (%s), la línea %d compara DOS valores que salen los dos de "+
						"`sha256sum` del mismo artefacto:\n  %s\n"+
						"Una comparación así es vacuamente cierta: no puede ponerse roja, y se ve "+
						"idéntica a una que funciona. El valor ESPERADO tiene que venir de afuera "+
						"—un pin, un argumento, un .sha256 descargado—, nunca del archivo que se está "+
						"verificando.", b.rel, b.nombre, c.n, strings.TrimSpace(c.texto))
				}
			}

			// 4. LA COMPARACIÓN TIENE QUE FRENAR. Un `if` que compara y sigue de largo no es una
			//    compuerta. Se acepta el `|| die` en la misma línea o un `die`/`exit` adentro de la
			//    rama que abre.
			frenan := 0
			for _, c := range comparaciones {
				if comparacionFrena(c, lineas) {
					frenan++
				}
			}
			if frenan == 0 {
				t.Errorf("en deploy/%s (%s) ninguna de las %d comparaciones del sha termina en un "+
					"`die`/`exit`: se compara y se sigue igual, que es lo mismo que no comparar.",
					b.rel, b.nombre, len(comparaciones))
			}

			// 5. Y TIENE QUE DOMINAR AL CONSUMO: no puede existir un camino al `install` que no haya
			//    pasado por ella.
			consumos := 0
			for _, l := range lineas {
				if !lineaCorre(l, b.consumo) {
					continue
				}
				consumos++
				dominada := false
				for _, c := range comparaciones {
					if c.n < l.n && dominaA(c.pila, l.pila) && comparacionFrena(c, lineas) {
						dominada = true
						break
					}
				}
				if !dominada {
					t.Errorf("deploy/%s (%s): el `install` de la línea %d NO está dominado por ninguna "+
						"comparación del sha256.\n  install: %s\n  ramas que lo encierran: %v\n"+
						"  comparaciones y sus ramas: %s\n"+
						"O sea: existe un camino que llega a instalar sin haber comparado nada. Es el "+
						"sabotaje medido de la ronda 2 —envolver la verificación en un `if` nuevo con "+
						"su `else`—, y no importa que la vía de escape venga apagada por default: "+
						"termina copiada en un runbook y de ahí en todas las máquinas.\n"+
						"El arreglo NO es agregarle el nombre de la variable a una lista: es que la "+
						"comparación vuelva a estar en el camino de todos.",
						b.rel, b.nombre, l.n, strings.TrimSpace(l.texto), l.pila, ramasDe(comparaciones))
				}
			}
			if consumos == 0 {
				t.Fatalf("en el bloque %q de deploy/%s no encontré ninguna línea que corra %v: o el "+
					"bloque dejó de instalar por ahí y esta guarda mira donde ya no se decide, o "+
					"cambió de forma. En los dos casos estaría en verde sin haber mirado nada.",
					b.nombre, b.rel, b.consumo)
			}
			comprobados++
			t.Logf("%d install dominados por %d comparación(es) del sha medido %v",
				consumos, len(comparaciones), clavesMedidas(medido))
		})
	}
	if comprobados == 0 {
		t.Fatal("no quedó ningún bloque comprobado: esta prueba estaría en verde sin haber mirado nada")
	}
}

// lineaNombra dice si algún token de la línea nombra a la variable v.
func lineaNombra(l lineaDeGuion, v string) bool {
	for _, tok := range l.tokens {
		if nombraLaVariable(tok, v) {
			return true
		}
	}
	return false
}

// lineaCorre dice si la línea EJECUTA alguno de esos comandos. Se compara token contra token, así
// que ni un comentario ni un mensaje entrecomillado que nombre `install` la satisfacen.
func lineaCorre(l lineaDeGuion, comandos []string) bool {
	for _, tok := range l.tokens {
		for _, c := range comandos {
			if tok == c {
				return true
			}
		}
	}
	return false
}

// comparacionFrena dice si el camino de falla de esa comparación termina en `die`/`exit`. Dos
// formas: `[ ... ] || die ...` en la misma línea, o un `if` cuya rama contiene un `die`/`exit`.
func comparacionFrena(c lineaDeGuion, lineas []lineaDeGuion) bool {
	for _, tok := range c.tokens {
		if esSalidaQueFrena(tok) {
			return true
		}
	}
	abre := false
	for _, tok := range c.tokens {
		if tok == "if" {
			abre = true
		}
	}
	if !abre {
		return false
	}
	for _, l := range lineas {
		if l.n <= c.n {
			continue
		}
		if len(l.pila) <= len(c.pila) || !dominaA(c.pila, l.pila) {
			break // salió de la rama sin encontrar el die
		}
		for _, tok := range l.tokens {
			if esSalidaQueFrena(tok) {
				return true
			}
		}
	}
	return false
}

func ramasDe(cs []lineaDeGuion) string {
	var b []string
	for _, c := range cs {
		b = append(b, "L"+strconv.Itoa(c.n)+"["+strings.Join(c.pila, "/")+"]")
	}
	return strings.Join(b, "  ")
}

func clavesMedidas(m map[string]int) []string {
	var k []string
	for v := range m {
		k = append(k, v)
	}
	return k
}
