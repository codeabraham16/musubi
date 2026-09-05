package mcp

// Guarda del track «Control de flota»: NINGÚN cabo suelto sin registro.
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
// Así que la regla deja de depender de que alguien se acuerde. Cada ítem de un `## Lo que queda
// fuera` tiene que declarar UNA de estas cosas:
//
//   - su número de registro (**A17**, **B4**) — está anotado y tiene dueño;
//   - en qué slice se HIZO o se DESCARTÓ — ya no falta;
//   - «por diseño» / «despliegue» — nunca fue código de este track;
//   - «Cero dependencias nuevas» — es la coletilla de disciplina, no un pendiente.
//
// Si agregás un cabo nuevo sin ninguna de esas marcas, esta prueba falla y te manda a ABIERTO.md.
// Ese es exactamente el punto.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	encabezadoFuera = regexp.MustCompile(`(?i)^#+\s*lo que queda fuera`)
	// Un ítem de primer nivel: `- **algo**`. Las continuaciones van indentadas y no cuentan.
	itemDeCabo = regexp.MustCompile(`^- \*\*`)
	// Las marcas que dan por CUBIERTO un cabo. `S7c` y `S5b` incluidos: el sufijo es parte del
	// nombre del slice, no ruido.
	tieneCasa = regexp.MustCompile(`HECH|DESCARTAD|\*\*A\d+|\*\*B\d+|\*\*S\d+[a-z]?\*\*|por diseño|Cero dependencias|despliegue`)
)

func TestNingunCaboDeFlotaSeQuedaSinRegistro(t *testing.T) {
	specs, err := filepath.Glob(filepath.Join("..", "..", "specs", "flota-*", "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Si el glob deja de encontrar los specs (se movieron, se renombraron), la prueba pasaría
	// vacía y en verde — el modo de fallo más peligroso que puede tener un barrido.
	if len(specs) < 10 {
		t.Fatalf("sólo se encontraron %d specs de flota; el barrido no está mirando donde cree", len(specs))
	}

	huerfanos := 0
	for _, ruta := range specs {
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		dentro := false
		for n, linea := range strings.Split(string(crudo), "\n") {
			if strings.HasPrefix(linea, "#") {
				dentro = encabezadoFuera.MatchString(linea)
				continue
			}
			if !dentro || !itemDeCabo.MatchString(linea) {
				continue
			}
			if tieneCasa.MatchString(linea) {
				continue
			}
			huerfanos++
			corto := linea
			if len(corto) > 90 {
				corto = corto[:90] + "…"
			}
			t.Errorf("%s:%d — cabo sin registro:\n    %s\n  Anotalo en specs/control-de-flota/ABIERTO.md (tabla 1 con slice, o tabla 2 con la condición bajo la que se revisa) y nombrá acá su número.",
				ruta, n+1, corto)
		}
	}
	if huerfanos == 0 {
		t.Logf("%d specs de flota barridos, cero cabos sin registro", len(specs))
	}
}

// El registro tiene que seguir EXISTIENDO y conservar sus dos tablas. Si alguien lo borra o lo
// vacía, la prueba de arriba seguiría en verde (los specs no cambiaron) mientras el archivo al
// que mandan sus mensajes deja de decir nada.
func TestElRegistroDeAbiertosSigueEnPie(t *testing.T) {
	crudo, err := os.ReadFile(filepath.Join("..", "..", "specs", "control-de-flota", "ABIERTO.md"))
	if err != nil {
		t.Fatalf("no se pudo leer el registro de abiertos: %v", err)
	}
	texto := string(crudo)
	for _, quiero := range []struct{ frag, porque string }{
		{"## 1 · Con slice asignado", "la tabla de lo que SÍ se va a hacer"},
		{"## 2 · Decisiones de NO hacer", "la tabla de lo declarado fuera, con su condición de revisión"},
		{"| A", "no queda ni un cabo con dueño: o se terminó todo, o alguien vació la tabla"},
		{"| B", "no queda ni una decisión de no-hacer: sospechoso"},
	} {
		if !strings.Contains(texto, quiero.frag) {
			t.Errorf("ABIERTO.md perdió %q — %s", quiero.frag, quiero.porque)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// LAS DOS GUARDAS QUE SIGUEN CUSTODIAN EL REGISTRO POR DENTRO, NO A LOS SPECS
//
// La de arriba pregunta «¿este cabo está anotado?». Una auditoría del 2026-09-02 encontró que la
// pregunta de al lado —«¿el número al que apunta significa algo?»— tampoco se cumplía, y por dos
// caminos distintos que a mano no se ven porque las filas están lejos una de otra:
//
//   - **A33** se citaba DOS VECES como decisión pendiente y no tenía fila en ninguna tabla. Se lo
//     había convertido en `B20` cuatro días antes y nadie escribió la conversión: quien buscaba
//     A33 no encontraba nada, que es peor que no encontrar el cabo — se lee como «esto ya no existe».
//   - `B13` nombraba TRES decisiones distintas y `B14` dos. «Se revisa en B13» dejaba de
//     identificar cuál, así que la condición de revisión —lo único que la tabla 2 promete— quedaba
//     sin dueño.
//
// POR QUÉ SÓLO SE MIRA LA PARTE VIVA (las tablas 1 y 2, no la sección 3)
//
// La regla 1 del propio archivo manda BORRAR la fila cuando un cabo se cierra, así que la sección
// «Cerrado en este track» nombra decenas de números que —correctamente— ya no tienen fila. Exigir
// una fila para cada número citado ahí sería exigir que el registro se contradiga a sí mismo, y la
// prueba mandaría a resucitar cabos cerrados. Lo que sí tiene que valer es más fino: un cabo VIVO
// —una fila de las tablas, o la prosa que cuelga de ella— no puede apuntar a un número que el
// archivo no DEFINE en ningún lado. Definir es una de estas cinco: tener fila propia, tener
// entrada de cierre, ser lo que cerró un slice («cierra A13»), haberse convertido en otro número
// («era A61», «A22 → B13»), o estar registrado en el cuerpo de una entrada.
// ────────────────────────────────────────────────────────────────────────────────────────────

var (
	inicioTabla1   = regexp.MustCompile(`(?m)^## 1 ·`)
	inicioCerrado  = regexp.MustCompile(`(?m)^## 3 ·`)
	filaDeRegistro = regexp.MustCompile(`^\| ([AB]\d+) `)
	numeroA        = regexp.MustCompile(`\bA\d+\b`)
)

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

// parteViva devuelve las líneas de las tablas 1 y 2 —lo que sigue abierto—, sin la sección de
// cerrados. Incluye la prosa que cuelga de una fila: en este archivo una fila puede seguir en
// párrafos sueltos debajo, y esos párrafos son parte del cabo.
func parteViva(t *testing.T, texto string) []string {
	t.Helper()
	desde := inicioTabla1.FindStringIndex(texto)
	hasta := inicioCerrado.FindStringIndex(texto)
	if desde == nil || hasta == nil || hasta[0] <= desde[0] {
		t.Fatalf("no se encontraron las secciones «## 1 ·» y «## 3 ·» de ABIERTO.md; el barrido no está mirando donde cree")
	}
	return strings.Split(texto[desde[0]:hasta[0]], "\n")
}

// TestNingunNumeroDeRegistroSeUsaDosVeces: un número repetido es peor que uno faltante, porque no
// se nota. Nadie lee las dos tablas de corrido buscando choques, y las filas que chocan suelen
// haberse escrito con semanas de diferencia.
//
// Sabotaje que la hace fallar: duplicar cualquier fila de la tabla 2 con el número de otra.
func TestNingunNumeroDeRegistroSeUsaDosVeces(t *testing.T) {
	vivo := parteViva(t, registroDeAbiertos(t))

	donde := map[string][]int{}
	orden := []string{}
	for n, linea := range vivo {
		m := filaDeRegistro.FindStringSubmatch(linea)
		if m == nil {
			continue
		}
		if _, visto := donde[m[1]]; !visto {
			orden = append(orden, m[1])
		}
		donde[m[1]] = append(donde[m[1]], n+1)
	}
	// El modo de fallo peligroso: que el parseo deje de encontrar filas y la prueba pase vacía y
	// en verde. Con 15 cabos y 20 decisiones al escribirse esto, 20 es un piso holgado.
	if len(donde) < 20 {
		t.Fatalf("sólo se reconocieron %d filas en las tablas de ABIERTO.md; cambió el formato y esta guarda dejó de mirar", len(donde))
	}
	for _, num := range orden {
		if len(donde[num]) > 1 {
			t.Errorf("**%s** nombra %d filas distintas de ABIERTO.md (líneas %v de la parte viva).\n  Un número repetido no identifica nada: «se revisa en %s» deja de decir cuál.\n  Dale un número LIBRE a las repetidas —por encima del máximo en uso— y anotá en la fila por qué cambió.",
				num, len(donde[num]), donde[num], num)
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
	vivo := parteViva(t, texto)

	conFila := map[string]bool{}
	for _, linea := range vivo {
		if m := filaDeRegistro.FindStringSubmatch(linea); m != nil {
			conFila[m[1]] = true
		}
	}
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
			if conFila[num] || visto[num] || estaDefinido(texto, num) {
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
// para que agregar una sexta sea una decisión visible, con su comentario, en vez de un patrón más
// ancho que apaga la prueba de a poco.
func estaDefinido(texto, num string) bool {
	q := regexp.QuoteMeta(num)
	for _, patron := range []string{
		`(?m)^\| ` + q + ` `,                                  // su propia fila en la tabla 1 o 2
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

// TODA FILA DE UNA TABLA TIENE LAS CELDAS QUE SU ENCABEZADO DECLARA.
//
// Lo encontró una fila real, no una hipótesis: `B20` tenía CUATRO celdas —terminaba en
// `| **decidido** |`— en una tabla cuyo encabezado declara TRES (`| # | Qué | Por qué no |`).
// Era una fila con forma de tabla 1 metida en la tabla 2, y llevaba así desde que se renumeró.
//
// NO ES COSMÉTICO, Y ES POR ESO QUE HAY GUARDA. Markdown no falla con una celda de más: la
// DESCARTA. Así que lo que se caía al renderizar era exactamente la palabra que esa fila existe
// para decir —`decidido`—, y donde la gente LEE el registro esa fila se veía igual que una sin
// resolver. Un formato que se traga el dato más importante de una fila sin avisar es la misma
// familia que persigue todo este archivo: no falla, miente en silencio.
//
// La guarda es general y no nombra a B20: cuenta las celdas del encabezado de CADA tabla y exige
// que sus filas coincidan. Una tabla nueva queda cubierta sin tocar esta prueba.
//
// Sabotaje que la hace fallar: devolverle a B20 su ` | **decidido** |`, o quitarle una celda a
// cualquier fila de cualquier tabla.
func TestTodaFilaDeAbiertoTieneLasCeldasDeSuEncabezado(t *testing.T) {
	crudo, err := os.ReadFile(filepath.Join("..", "..", "specs", "control-de-flota", "ABIERTO.md"))
	if err != nil {
		t.Fatalf("no se pudo leer ABIERTO.md: %v", err)
	}

	// `| a | b |` son 2 celdas: se cuentan los separadores internos, que es lo que usa el
	// renderizador para decidir dónde corta.
	//
	// UN `\|` NO ES UN SEPARADOR, y contarlo como tal fue un falso POSITIVO medido el 2026-09-05:
	// la fila de A98 llevaba `Get-Process musubi \| Select-Object` y esta cuenta la daba por rota
	// cuando ya estaba arreglada. GFM respeta la barra invertida DENTRO de un code span, que es
	// justamente donde hace falta — los backticks NO protegen el pipe, y ahí estaba el defecto
	// original.
	celdas := func(l string) int {
		l = strings.TrimSpace(l)
		n := 0
		for i := 0; i < len(l); i++ {
			if l[i] == '|' && (i == 0 || l[i-1] != '\\') {
				n++
			}
		}
		return n - 1
	}
	// El separador de un encabezado es la línea de guiones: `|---|---|`.
	esSeparador := func(l string) bool {
		l = strings.TrimSpace(l)
		return strings.HasPrefix(l, "|") && strings.Trim(l, "|-: \t") == ""
	}
	esFila := func(l string) bool {
		l = strings.TrimSpace(l)
		return strings.HasPrefix(l, "|") && strings.HasSuffix(l, "|")
	}

	lineas := strings.Split(string(crudo), "\n")
	tablas, filasVistas := 0, 0
	esperadas, encabezadoEn := 0, 0
	for i, l := range lineas {
		switch {
		case esSeparador(l):
			// El encabezado es la línea de ARRIBA. Si no era una fila, esto no es una tabla.
			if i > 0 && esFila(lineas[i-1]) {
				esperadas, encabezadoEn = celdas(lineas[i-1]), i
				tablas++
			} else {
				esperadas = 0
			}
		case esperadas > 0 && esFila(l):
			filasVistas++
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
			// el blanco daba la tabla por terminada y dejaba de revisar, o sea que se ponía EN VERDE
			// sobre todo lo que venía después.
			//
			// Medido el 2026-09-05: había NUEVE líneas en blanco adentro de la tabla de la sección 1,
			// y la primera estaba a catorce filas del encabezado — así que A77, A79, A88, A89, A96,
			// A90..A95, A98 y A99 no se revisaban Y no renderizaban como tabla. Entre ellas, la de
			// A98, con una celda de más que el renderizador se come. Es el mismo defecto que B20 ya
			// registró, y esta prueba existía para atraparlo.
			//
			// Un blanco ANTES de un encabezado o del final de la sección sí es legítimo: la tabla se
			// terminó de verdad. La diferencia es si DESPUÉS del blanco siguen viniendo filas.
			siguen := false
			for j := i + 1; j < len(lineas); j++ {
				if strings.TrimSpace(lineas[j]) == "" {
					continue
				}
				siguen = esFila(lineas[j]) && !esSeparador(lineas[j])
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

	// CONTROL DE QUE MIRÓ ALGO: si cambiara el formato del archivo —tablas sin línea de guiones,
	// otro estilo de barras— los reconocedores dejarían de matchear y esta prueba pasaría en verde
	// sin haber contado una sola celda. Hoy hay 3 tablas y más de 30 filas.
	if tablas < 3 || filasVistas < 30 {
		t.Fatalf("se reconocieron %d tabla(s) y %d fila(s) en ABIERTO.md, y son al menos 3 y 30: "+
			"cambió el formato del archivo y esta guarda dejó de mirar", tablas, filasVistas)
	}
}
