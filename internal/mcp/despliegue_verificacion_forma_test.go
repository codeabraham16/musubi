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
//
// ─── RONDA 3: LAS FUGAS FINAS, Y UN FALSO POSITIVO ───────────────────────────────────────────
//
// Lo de arriba quedó en pie. Lo que esta ronda arregla es la guarda misma, en cuatro puntos, y
// todos son la misma enfermedad: preguntar por LA FORMA de algo en vez de derivarlo.
//
//  A. UN FALSO POSITIVO, que es más urgente que cualquier fuga. `install -d "$(dirname "$BIN")"`
//     —crear el directorio destino, que este repo hace en ocho lugares— ponía esto rojo acusando
//     al autor de abrir un fail-open. El campo `consumo` era el token `install` a secas. Ahora se
//     PARSEA el argv con las reglas de `install(1)`. Una guarda que se pone roja sobre código sano
//     la apaga el próximo que la cruza, y a partir de ahí no protege de nada.
//
//  B. EL LECTOR NO ENTENDÍA `nombre() {` CON ESPACIO, y lo peor es que no lo decía: el cuerpo se
//     leía como nivel superior, así que mover TODA la verificación a una función y llamarla sólo
//     si no existe un centinela dejaba las once guardas en verde e instalaba un binario adulterado
//     con RC=0. La cabecera de acá prometía que un bloque que no se entiende MATA la prueba, y
//     estaba mis-parseando en silencio. Y al revés: `nombre(){` SÍ empujaba marco, así que el mismo
//     refactor escrito sin el espacio —sano— daba rojo. La guarda premiaba la forma insegura y
//     castigaba la segura según dónde cayera un espacio.
//
//  C. LAS FUNCIONES SE INLINEAN. Que el cuerpo tenga su marco no alcanza: una función se ejecuta
//     donde LA LLAMAN. Ahora cada llamada se reemplaza por el cuerpo con la pila del punto de
//     llamada adelante, así que la llamada incondicional queda verde y la condicionada, roja.
//
//  D. EL `die` TENÍA QUE EXISTIR, NO SER INCONDICIONAL. `if [ "$want" != "$got" ]; then if [ ! -f
//     /etc/musubi/instalar-sin-verificar ]; then die; fi; fi` quedaba verde, y también
//     `[ a = b ] || [ -f centinela ] || die`. Se exige que el camino de falla llegue al `die` sin
//     ninguna condición nueva en el medio. Es la misma función para los seis bloques, así que el
//     relay de RustDesk —donde el sabotaje se midió— queda cerrado por el mismo arreglo.

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
//
// LAS DOS FORMAS TIENEN QUE VALER LO MISMO, Y NO VALÍAN. El lector sólo empujaba marco cuando el
// token terminaba en `{`; con el espacio de por medio (`nombre() {`) el `{` quedaba fuera de
// posición de comando, no se empujaba nada, el cuerpo se leía como nivel superior y el `}` de
// cierre no pop-eaba ni daba error. Resultado medido: mover TODA la verificación del paso 1 a
// `verificar_el_binario() {` y llamarla sólo si no existe un archivo centinela dejaba las once
// guardas en verde, y corrido de verdad instalaba un binario adulterado con RC=0 después de
// imprimir «Checksum del binario verificado».
//
// Y el castigo era al revés del riesgo: `nombre(){` SÍ empujaba marco, así que el MISMO refactor
// escrito sin el espacio —que es sano cuando la llamada es incondicional— daba ROJO. La guarda
// premiaba la forma insegura y castigaba la segura según dónde cayera un espacio. El repo usa las
// dos (24 con espacio, 51 sin).
var reDefinicionDeFuncion = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\(\)\{?$`)

// nombreDeFuncionDefinida devuelve el nombre si el token es una definición de función.
func nombreDeFuncionDefinida(tok string) (string, bool) {
	m := reDefinicionDeFuncion.FindStringSubmatch(tok)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// marcaDeCuerpo es el id que lleva el marco del cuerpo de una función. El nombre viaja adentro
// para poder inlinear las llamadas después.
func marcaDeCuerpo(linea int, nombre string) string {
	return "L" + strconv.Itoa(linea) + ":func:" + nombre
}

// nombreDelMarcoDeFuncion devuelve el nombre de la función si ese marco es un cuerpo de función.
func nombreDelMarcoDeFuncion(id string) (string, bool) {
	if i := strings.Index(id, ":func:"); i >= 0 {
		return id[i+len(":func:"):], true
	}
	return "", false
}

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
	n      int // 1-based dentro del bloque, para los mensajes
	orden  int // posición en la secuencia YA inlineada, para preguntar «¿quién viene antes?»
	texto  string
	tokens []string
	pila   []string
	// defDeFuncion trae el nombre cuando esta línea DEFINE una función. Una definición no se
	// ejecuta donde está escrita: se ejecuta en cada llamada, y por eso se saca de la secuencia.
	defDeFuncion string
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
	// esperandoCuerpo trae el nombre de la función cuya `{` todavía no llegó. Es lo que hace que
	// `nombre() {` y `nombre(){` empujen el MISMO marco.
	esperandoCuerpo := ""
	esperandoNombre := false // después de la palabra reservada `function`
	for _, t := range toks {
		if t.linea != lineaPrevia {
			comando = true // un salto de línea termina el comando anterior
			lineaPrevia = t.linea
		}
		if esperandoNombre {
			nombre := t.tok
			if n, ok := nombreDeFuncionDefinida(t.tok); ok {
				nombre = n
			}
			if !reNombreDeFuncion.MatchString(nombre) {
				return nil, errorForma{"línea " + strconv.Itoa(t.linea) + ": después de `function` " +
					"esperaba un nombre y vino `" + t.tok + "`"}
			}
			esperandoNombre = false
			esperandoCuerpo = nombre
			if t.linea >= 1 && t.linea <= len(lineas) {
				lineas[t.linea-1].defDeFuncion = nombre
			}
			if !strings.HasSuffix(t.tok, "{") {
				anotarHasta(t.linea)
				if t.linea >= 1 && t.linea <= len(lineas) {
					lineas[t.linea-1].tokens = append(lineas[t.linea-1].tokens, t.tok)
				}
				comando = true
				continue
			}
		}
		if esperandoCuerpo != "" && t.tok != "{" && t.tok != "(" {
			// «No pude medir» no puede ser el mismo verde que «medí y está bien»: si no entiendo
			// con qué se abre el cuerpo de la función, no puedo decir dónde vive lo que hay adentro.
			return nil, errorForma{"línea " + strconv.Itoa(t.linea) + ": la definición de `" +
				esperandoCuerpo + "()` no abre su cuerpo con `{` ni con `(` sino con `" + t.tok +
				"`, y no sé bajo qué condiciones vive lo que hay adentro"}
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
			id := "L" + strconv.Itoa(t.linea) + ":" + t.tok
			if esperandoCuerpo != "" {
				id = marcaDeCuerpo(t.linea, esperandoCuerpo)
				esperandoCuerpo = ""
			}
			pila = append(pila, marcoBash{apertura: t.tok, linea: t.linea, id: id})
		case t.tok == "function":
			esperandoNombre = true
		case esNombreDeFuncionDefinida(t.tok):
			nombre, _ := nombreDeFuncionDefinida(t.tok)
			fijar(t.linea)
			if t.linea >= 1 && t.linea <= len(lineas) {
				lineas[t.linea-1].defDeFuncion = nombre
			}
			if strings.HasSuffix(t.tok, "{") {
				pila = append(pila, marcoBash{apertura: "{", linea: t.linea,
					id: marcaDeCuerpo(t.linea, nombre)})
			} else {
				esperandoCuerpo = nombre // el `{` viene después: `nombre() {`
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
		comando = abrePosicionDeComando(t.tok) || esperandoCuerpo != "" || esperandoNombre
	}
	if esperandoCuerpo != "" || esperandoNombre {
		return nil, errorForma{"el bloque termina en una definición de función sin cuerpo"}
	}
	if len(pila) != 0 {
		var abiertos []string
		for _, m := range pila {
			abiertos = append(abiertos, m.apertura+" de la línea "+strconv.Itoa(m.linea))
		}
		return nil, errorForma{"el bloque termina sin cerrar: " + strings.Join(abiertos, ", ")}
	}
	anotarHasta(len(lineas) + 1) // las líneas del final que no tenían ningún token
	return inlinearLlamadas(lineas)
}

// -----------------------------------------------------------------------------------------------
// LAS FUNCIONES SE EJECUTAN DONDE LAS LLAMAN.
//
// Arreglar el lector para que `nombre() {` empuje marco cierra el fail-open —el cuerpo deja de
// leerse como nivel superior— pero abre un FALSO POSITIVO que sería peor: mover la verificación a
// una función y llamarla SIN condición es un refactor sano, y con el cuerpo adentro de su propio
// marco la comparación dejaría de dominar al `install` de afuera. Rojo sobre código bueno.
//
// Por eso no se decide sobre el texto tal como está escrito: se INLINEA. Cada llamada a una
// función definida en el bloque se reemplaza por el cuerpo de esa función, con la pila del punto
// de llamada adelante. Entonces:
//
//   - `verificar_el_binario` llamada al ras del bloque   → la comparación queda al ras: VERDE.
//   - llamada bajo `if [ ! -f /etc/musubi/instalar-sin-verificar ]` → la comparación hereda ese
//     `if` y ya no domina al `install`: ROJO, que es la fuga medida.
//   - función definida y nunca llamada → su cuerpo desaparece de la secuencia, y con él la
//     comparación: ROJO por «calcula el sha y no lo compara con nada».
//
// Y da igual cómo se escriba la definición: `nombre(){`, `nombre() {` y `function nombre {` pasan
// por el mismo camino.
// -----------------------------------------------------------------------------------------------

// profundidadMaximaDeLlamadas corta la expansión. Que se acabe no es «está bien»: es no pude medir.
const profundidadMaximaDeLlamadas = 8

// cuerpoDeFuncion son las líneas de una función, con la pila YA relativa a su propio cuerpo.
type cuerpoDeFuncion struct {
	nombre string
	lineas []lineaDeGuion
}

// enPosicionDeComando dice si el token i de esa línea arranca un comando (y entonces un nombre de
// función ahí es una LLAMADA, y no el argumento de otra cosa ni un texto suelto).
func enPosicionDeComando(tokens []string, i int) bool {
	if i == 0 {
		return true
	}
	return abrePosicionDeComando(tokens[i-1])
}

// inlinearLlamadas saca de la secuencia los cuerpos de las funciones y los pega en cada llamada.
func inlinearLlamadas(lineas []lineaDeGuion) ([]lineaDeGuion, error) {
	// 1. QUÉ FUNCIONES HAY. El marco del cuerpo lleva el nombre adentro.
	cuerpos := map[string]*cuerpoDeFuncion{}
	marcos := map[string]string{} // id del marco → nombre
	for _, l := range lineas {
		for _, id := range l.pila {
			if n, ok := nombreDelMarcoDeFuncion(id); ok {
				marcos[id] = n
			}
		}
	}
	esMarcoDeFuncion := func(pila []string) (string, int, bool) {
		for i, id := range pila {
			if n, ok := marcos[id]; ok {
				return n, i, true
			}
		}
		return "", 0, false
	}
	for _, l := range lineas {
		if l.defDeFuncion != "" {
			if cuerpos[l.defDeFuncion] == nil {
				cuerpos[l.defDeFuncion] = &cuerpoDeFuncion{nombre: l.defDeFuncion}
			}
			// La línea de la definición entra al cuerpo: en `f() { cmp; die; }` todo vive ahí.
			rel := l
			rel.pila = nil
			rel.defDeFuncion = ""
			cuerpos[l.defDeFuncion].lineas = append(cuerpos[l.defDeFuncion].lineas, rel)
			continue
		}
		if n, i, ok := esMarcoDeFuncion(l.pila); ok {
			rel := l
			rel.pila = append([]string{}, l.pila[i+1:]...)
			if cuerpos[n] == nil {
				cuerpos[n] = &cuerpoDeFuncion{nombre: n}
			}
			cuerpos[n].lineas = append(cuerpos[n].lineas, rel)
		}
	}

	// 2. LA SECUENCIA DE AFUERA: todo lo que no es cuerpo ni definición.
	var afuera []lineaDeGuion
	for _, l := range lineas {
		if l.defDeFuncion != "" {
			continue
		}
		if _, _, ok := esMarcoDeFuncion(l.pila); ok {
			continue
		}
		afuera = append(afuera, l)
	}

	expandidas, err := expandirSecuencia(afuera, cuerpos, 0)
	if err != nil {
		return nil, err
	}
	for i := range expandidas {
		expandidas[i].orden = i + 1
	}
	return expandidas, nil
}

// expandirSecuencia pega, después de cada llamada, el cuerpo de la función llamada.
func expandirSecuencia(lineas []lineaDeGuion, cuerpos map[string]*cuerpoDeFuncion, prof int) ([]lineaDeGuion, error) {
	if prof > profundidadMaximaDeLlamadas {
		return nil, errorForma{"las llamadas a funciones se anidan más de " +
			strconv.Itoa(profundidadMaximaDeLlamadas) + " niveles (¿recursión?): no puedo decir bajo " +
			"qué condiciones vive cada comparación, y «no pude medir» no es «medí y está bien»"}
	}
	var salida []lineaDeGuion
	for _, l := range lineas {
		salida = append(salida, l)
		for i, tok := range l.tokens {
			f := cuerpos[tok]
			if f == nil || !enPosicionDeComando(l.tokens, i) {
				continue
			}
			cuerpo, err := expandirSecuencia(f.lineas, cuerpos, prof+1)
			if err != nil {
				return nil, err
			}
			for _, bl := range cuerpo {
				bl.pila = append(append([]string{}, l.pila...), bl.pila...)
				salida = append(salida, bl)
			}
		}
	}
	return salida, nil
}

var reNombreDeFuncion = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func esNombreDeFuncionDefinida(tok string) bool {
	_, ok := nombreDeFuncionDefinida(tok)
	return ok
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
				usos, dudosos := invocacionesDeConsumo(l, b.consumo)
				for _, d := range dudosos {
					t.Errorf("deploy/%s (%s), línea %d: no pude clasificar esta invocación de `%s`:\n  %s\n"+
						"No sé si copia el artefacto o sólo crea un directorio, así que NO puedo decir que "+
						"está dominada por la comparación del sha. «No pude medir» tiene que verse "+
						"distinto de «medí y está bien»: por eso esto es rojo y no un verde silencioso.\n"+
						"Si la invocación es legítima, enseñale la opción a `clasificarInstall`.",
						b.rel, b.nombre, l.n, d.comando, strings.TrimSpace(l.texto))
				}
				if usos == 0 {
					continue
				}
				consumos += usos
				dominada := false
				for _, c := range comparaciones {
					if c.orden < l.orden && dominaA(c.pila, l.pila) && comparacionFrena(c, lineas) {
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
				t.Fatalf("en el bloque %q de deploy/%s no encontré ninguna invocación de %v que ponga un "+
					"artefacto en su destino (las que sólo crean directorios —`install -d`— no cuentan): "+
					"o el bloque dejó de instalar por ahí y esta guarda mira donde ya no se decide, o "+
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

// -----------------------------------------------------------------------------------------------
// ¿ESTE `install` PONE EL ARTEFACTO, O SÓLO CREA UN DIRECTORIO?
//
// EL FALSO POSITIVO QUE CIERRA, MEDIDO: agregar al paso 1 la línea
//
//	install -d -m 0755 "$(dirname "$BIN")"
//
// —crear el directorio destino, que no instala ningún artefacto y que este repo YA hace en ocho
// lugares (install-musubi-brain.sh:170 y :222, prometheus/install-musubi-prometheus.sh:91 y :92,
// docker/preparar.sh:60 y :61, y dos veces en el RUNBOOK)— ponía roja a
// TestNingunInstallEsAlcanzableSinHaberComparadoElSha con un mensaje que acusaba al autor de haber
// abierto una vía de escape fail-open. El campo `consumo` era el token `install` a secas, y no
// distingue instalar un ARTEFACTO de hacer un `mkdir -p`.
//
// UN FALSO POSITIVO ES PEOR QUE UNA FUGA: el próximo que agregue un directorio se come un rojo
// incomprensible sobre código sano, y lo que se apaga después es la guarda entera — y entonces ya
// no protege de nada.
//
// Y NO SE ARREGLA MIRANDO LA FORMA («¿el texto trae un -d?»), que le erraría a `-dm0755`, a
// `--directory` y a la próxima manera de escribirlo. Se PARSEA el argv de la invocación con las
// reglas de opciones de `install(1)` de coreutils —qué letras cortas llevan argumento y cuáles no,
// qué opciones largas lo llevan— y se pregunta si quedó activado el modo directorio.
//
// LO QUE EL PARSER NO ENTIENDE NO SE DECLARA INOCENTE. Devuelve «no sé», y quien pregunta lo
// reporta como ROJO explícito. Un verde silencioso sobre algo que no se entendió es exactamente la
// figura que este archivo persigue en los guiones.
// -----------------------------------------------------------------------------------------------

// claseDeUso es el veredicto sobre UNA invocación.
type claseDeUso int

const (
	usoConsume     claseDeUso = iota // copia un artefacto a su destino
	usoNoConsume                     // no pone ningún artefacto (p. ej. `install -d`)
	usoNoEntendido                   // no pude clasificarla: rojo, nunca verde
)

// Las opciones cortas de `install(1)`, separadas por lo único que hace falta para recorrer un argv
// sin confundir el argumento de una opción con un operando: si llevan argumento o no.
const (
	instalarCortasConArgumento = "mogtSZ"     // -m 0755, -o root, -g root, -t DIR, -S SUF, -Z CTX
	instalarCortasSinArgumento = "bcCDdpsTvP" // banderas
)

// instalarLargasConArgumento son las opciones largas que EXIGEN argumento. `--backup` y `--context`
// lo llevan opcional, y GNU sólo acepta el opcional pegado con `=`, así que no se comen el token
// siguiente.
var instalarLargasConArgumento = map[string]bool{
	"mode": true, "owner": true, "group": true, "target-directory": true,
	"suffix": true, "strip-program": true,
}

var instalarLargasSinArgumento = map[string]bool{
	"directory": true, "backup": true, "context": true, "compare": true, "debug": true,
	"preserve-timestamps": true, "strip": true, "no-target-directory": true, "verbose": true,
	"preserve-context": true, "help": true, "version": true,
}

// desentrecomillar saca las comillas de un token entero (`"-d"` -> `-d`). No interpreta nada más:
// un token con expansiones adentro vuelve tal cual y termina tratado como operando.
func desentrecomillar(tok string) string {
	if len(tok) >= 2 && (tok[0] == '\'' || tok[0] == '"') && tok[len(tok)-1] == tok[0] {
		return tok[1 : len(tok)-1]
	}
	return tok
}

// clasificarInstall recorre el argv de un `install` y dice si pone un artefacto o sólo crea
// directorios.
func clasificarInstall(argv []string) claseDeUso {
	directorio, soloOperandos := false, false
	for i := 0; i < len(argv); i++ {
		a := desentrecomillar(argv[i])
		switch {
		case soloOperandos || a == "-" || !strings.HasPrefix(a, "-"):
			// operando: el origen o el destino
		case a == "--":
			soloOperandos = true
		case strings.HasPrefix(a, "--"):
			nombre, pegado := strings.TrimPrefix(a, "--"), false
			if j := strings.Index(nombre, "="); j >= 0 {
				nombre, pegado = nombre[:j], true
			}
			switch {
			case instalarLargasConArgumento[nombre]:
				if !pegado {
					i++ // el argumento es el token siguiente
				}
			case instalarLargasSinArgumento[nombre]:
				if nombre == "directory" {
					directorio = true
				}
			default:
				return usoNoEntendido
			}
		default: // racimo de opciones cortas: -d, -dm0755, -m 0755 ...
			for k := 1; k < len(a); k++ {
				c := a[k]
				if strings.IndexByte(instalarCortasConArgumento, c) >= 0 {
					if k == len(a)-1 {
						i++ // el argumento viene en el token siguiente
					}
					break // el resto del racimo ES el argumento
				}
				if c == 'd' {
					directorio = true
					continue
				}
				if strings.IndexByte(instalarCortasSinArgumento, c) >= 0 {
					continue
				}
				return usoNoEntendido
			}
		}
	}
	if directorio {
		return usoNoConsume
	}
	return usoConsume
}

// usoDudoso es una invocación que no se pudo clasificar.
type usoDudoso struct{ comando string }

// esOperadorDeControl corta el argv: lo que viene después ya es otro comando.
func esOperadorDeControl(tok string) bool {
	switch tok {
	case ";", ";;", "&&", "||", "|", "&", "then", "else", "do", "fi", "done", "{", "}", "(", ")":
		return true
	}
	return false
}

// invocacionesDeConsumo cuenta, en una línea, las invocaciones de esos comandos que EFECTIVAMENTE
// dejan el artefacto puesto, y devuelve aparte las que no se pudieron clasificar.
//
// Se compara token contra token, así que ni un comentario ni un mensaje entrecomillado que nombre
// `install` cuentan — que es el error que este repo repite. Y a cada invocación se le arma su argv
// (hasta el próximo operador de control) para poder PREGUNTARLE qué hace, en vez de suponerlo por
// el nombre del comando.
func invocacionesDeConsumo(l lineaDeGuion, comandos []string) (int, []usoDudoso) {
	n := 0
	var dudosos []usoDudoso
	for i, tok := range l.tokens {
		esCandidato := false
		for _, c := range comandos {
			if tok == c {
				esCandidato = true
			}
		}
		if !esCandidato {
			continue
		}
		var argv []string
		for j := i + 1; j < len(l.tokens); j++ {
			if esOperadorDeControl(l.tokens[j]) {
				break
			}
			argv = append(argv, l.tokens[j])
		}
		clase := usoConsume
		if tok == "install" {
			clase = clasificarInstall(argv)
		}
		switch clase {
		case usoConsume:
			n++
		case usoNoEntendido:
			dudosos = append(dudosos, usoDudoso{comando: tok})
		}
	}
	return n, dudosos
}

// comparacionFrena dice si el camino de falla de esa comparación termina SIEMPRE en un
// `die`/`exit`.
//
// LA FUGA QUE CIERRA, MEDIDA: se preguntaba si EXISTÍA un `die` en la rama, no si era
// INCONDICIONAL. Entonces
//
//	if [ "$want" != "$got" ]; then
//	  if [ ! -f /etc/musubi/instalar-sin-verificar ]; then die ...; fi
//	fi
//
// dejaba las once guardas en verde: hay un `die` adentro, sólo que con una vía de escape delante.
// La misma figura con una variable de entorno, igual. Y en la forma de lista,
// `[ "$a" = "$b" ] || [ -f centinela ] || die` también, porque el token `die` estaba en la línea.
//
// Ahora se exige que el `die` esté en el camino de falla SIN NINGUNA CONDICIÓN NUEVA en el medio:
//   - en la misma línea, el operador que sigue al test tiene que llevar directo a un `die`/`exit`
//     (se acepta un `{ ...; die ...; }` cuyos comandos van separados por `;`);
//   - en la forma de bloque, el `die` tiene que vivir en la rama misma, a UN nivel de la
//     comparación — no anidado más adentro, que es donde vive la vía de escape.
//
// Esto vale para los seis bloques de losBloquesQueBajanEInstalan(): es la misma función para todos,
// así que el relay de RustDesk —donde el sabotaje se midió como
// `[[ "$TENGO_SHA" == "$QUIERO_SHA" ]] || [[ -f /etc/rustdesk/sin-verificar ]] || die`— queda
// cerrado por el mismo arreglo. La lección aprendida de un lado y no del hermano es cómo pierde
// este repo.
func comparacionFrena(c lineaDeGuion, lineas []lineaDeGuion) bool {
	if frenaEnLaMismaLinea(c.tokens) {
		return true
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
		if l.orden <= c.orden {
			continue
		}
		if len(l.pila) <= len(c.pila) || !dominaA(c.pila, l.pila) {
			break // salió de la rama sin encontrar el die
		}
		if len(l.pila) != len(c.pila)+1 {
			continue // anidado más adentro: hay otra condición en el medio, no frena siempre
		}
		for _, tok := range l.tokens {
			if esSalidaQueFrena(tok) {
				return true
			}
		}
	}
	return false
}

// frenaEnLaMismaLinea mira una comparación escrita en una sola línea y dice si su camino de falla
// llega a un `die`/`exit` sin pasar por ninguna condición más.
func frenaEnLaMismaLinea(toks []string) bool {
	if len(toks) > 0 && toks[0] == "if" {
		for i, t := range toks {
			if t == "then" {
				return frenaIncondicional(toks[i+1:])
			}
		}
		return false
	}
	i := 0
	for i < len(toks) && !esComandoDeTest(toks[i]) {
		i++
	}
	if i >= len(toks) {
		return false
	}
	for i < len(toks) && toks[i] != "&&" && toks[i] != "||" && toks[i] != ";" && toks[i] != "|" {
		i++
	}
	if i >= len(toks) || (toks[i] != "&&" && toks[i] != "||") {
		return false
	}
	return frenaIncondicional(toks[i+1:])
}

// frenaIncondicional recorre una lista de comandos y dice si se llega a un `die`/`exit` sin que en
// el medio aparezca una condición nueva: ni otro test, ni un `if`, ni un `&&`/`||`.
func frenaIncondicional(toks []string) bool {
	for i := 0; i < len(toks); {
		switch {
		case toks[i] == "{" || toks[i] == "(" || toks[i] == ";" || toks[i] == "}" || toks[i] == ")":
			i++
		case esSalidaQueFrena(toks[i]):
			return true
		case esComandoDeTest(toks[i]) || toks[i] == "if" || toks[i] == "elif" || toks[i] == "!":
			return false // una condición más en el camino: ya no frena siempre
		default:
			for i < len(toks) && toks[i] != ";" && toks[i] != "&&" && toks[i] != "||" &&
				toks[i] != "|" && toks[i] != "}" && toks[i] != ")" {
				i++
			}
			if i < len(toks) && (toks[i] == "&&" || toks[i] == "||" || toks[i] == "|") {
				return false // el die de más adelante quedaría condicionado a este comando
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
