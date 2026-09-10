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
	"strconv"
	"strings"
	"testing"
)

var (
	// EL ENCABEZADO PUEDE VENIR NUMERADO, Y ÉSE ERA LA MITAD DE UN AGUJERO.
	//
	// Hasta el 2026-09-10 esto era `^#+\s*lo que queda fuera`, que rechaza
	// `## 2 · Lo que queda fuera (y va a `ABIERTO.md`)` — la forma que usan los `spec.md` del
	// track. La otra mitad estaba en el glob de abajo, que miraba sólo `tasks.md`. **Las dos
	// secciones afectadas fallaban por los DOS motivos a la vez**, así que arreglar uno solo no
	// habría cambiado nada y la auditoría anterior las dio por barridas: es la forma exacta de
	// «una guarda presente en N-1 de N caminos», con los dos agujeros tapándose entre sí.
	encabezadoFuera = regexp.MustCompile(`(?i)^#+\s*(?:\d+\s*[·.)\-]\s*)?lo que queda fuera`)
	// Un ítem de primer nivel: `- **algo**`. Las continuaciones van indentadas y no cuentan.
	itemDeCabo = regexp.MustCompile(`^- \*\*`)
	// Las marcas que dan por CUBIERTO un cabo. `S7c` y `S5b` incluidos: el sufijo es parte del
	// nombre del slice, no ruido.
	tieneCasa = regexp.MustCompile(`HECH|DESCARTAD|\*\*A\d+|\*\*B\d+|\*\*S\d+[a-z]?\*\*|por diseño|Cero dependencias|despliegue`)
)

func TestNingunCaboDeFlotaSeQuedaSinRegistro(t *testing.T) {
	// TODO EL TRACK, NO SÓLO `tasks.md`. Los ocho cabos que se destaparon el 2026-09-10 vivían en
	// dos `spec.md`, y un cabo declarado fuera de alcance vale lo mismo esté en el archivo que
	// esté: el que lo lee no sabe cuál de los dos abrió.
	specs, err := filepath.Glob(filepath.Join("..", "..", "specs", "flota-*", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Si el glob deja de encontrar los specs (se movieron, se renombraron), la prueba pasaría
	// vacía y en verde — el modo de fallo más peligroso que puede tener un barrido.
	if len(specs) < 35 {
		t.Fatalf("sólo se encontraron %d archivos de spec de flota; el barrido no está mirando donde cree", len(specs))
	}

	huerfanos, secciones, items := 0, 0, 0
	for _, ruta := range specs {
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		dentro := false
		for n, linea := range strings.Split(string(crudo), "\n") {
			if strings.HasPrefix(linea, "#") {
				dentro = encabezadoFuera.MatchString(linea)
				if dentro {
					secciones++
				}
				continue
			}
			if !dentro || !itemDeCabo.MatchString(linea) {
				continue
			}
			items++
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

	// CERO TIENE QUE SIGNIFICAR «MIRÉ Y ESTÁ LIMPIO», NUNCA «NO PUDE MIRAR».
	//
	// Sin esto, aflojar `encabezadoFuera` hasta que no matchee nada —o renombrar las secciones—
	// deja la prueba en verde habiendo barrido CERO ítems, que es indistinguible de «no hay cabos
	// sueltos». Es el defecto que acaba de costar ocho cabos invisibles, así que la cuenta de lo
	// que el barrido ENCONTRÓ es parte de la guarda y no una estadística.
	if secciones < 12 || items < 40 {
		t.Fatalf("el barrido reconoció %d sección(es) «Lo que queda fuera» y %d ítem(s) en %d archivos, y son al menos 12 y 40: "+
			"cambió la forma de los specs y esta guarda dejó de mirar — un cero acá NO es «no hay cabos sueltos»",
			secciones, items, len(specs))
	}
	if huerfanos == 0 {
		t.Logf("%d archivos de spec de flota barridos, %d secciones «Lo que queda fuera», %d ítems, cero cabos sin registro",
			len(specs), secciones, items)
	}
}

// El registro tiene que seguir EXISTIENDO y conservar sus dos tablas. Si alguien lo borra o lo
// vacía, la prueba de arriba seguiría en verde (los specs no cambiaron) mientras el archivo al
// que mandan sus mensajes deja de decir nada.
//
// ESTA PRUEBA ERAN CUATRO `strings.Contains` Y NO SERVÍA PARA LO QUE DICE SU NOMBRE.
//
// Dos de los cuatro fragmentos eran `"| A"` y `"| B"`, buscados en el archivo ENTERO. Con eso:
//
//   - la tabla 1 reducida a UNA SOLA FILA la dejaba en verde — que es justo el «alguien vació la
//     tabla» que el comentario dice cubrir;
//   - y ni siquiera hacía falta una fila: `"| A"` matchea en cualquier renglón de prosa de la
//     sección 3 que lleve una barra y una A, y este archivo tiene decenas de bloques con tablas
//     y con código adentro. O sea que el vacío total de las dos tablas también podía pasar.
//
// «El registro sigue en pie» no es «el archivo menciona una barra»: es que sus tablas tengan
// ENCABEZADO, filas con número, y una celda de estado que diga de quién es cada cabo. Eso es lo
// que se afirma acá, y por eso la guarda parsea en vez de buscar texto.
//
// Sabotaje que la hace fallar: borrar las filas de cualquiera de las dos tablas, o dejar vacía la
// celda de estado de una fila de la tabla 1.
func TestElRegistroDeAbiertosSigueEnPie(t *testing.T) {
	texto := registroDeAbiertos(t)

	// Las cuatro secciones, EN ORDEN. El orden importa porque los dos parseos de abajo cortan el
	// archivo por estos límites: si se cruzan, se estaría leyendo una tabla como si fuera la otra.
	secciones := []string{
		"## 1 · Con slice asignado",
		"## 2 · Decisiones de NO hacer",
		"## 3 · Cerrado en este track",
		"## Cómo se usa este archivo",
	}
	corte := make([]int, len(secciones))
	previo := -1
	for i, s := range secciones {
		corte[i] = strings.Index(texto, s)
		if corte[i] < 0 {
			t.Fatalf("ABIERTO.md perdió la sección %q: el registro dejó de tener la forma que sus propias reglas describen", s)
		}
		if corte[i] <= previo {
			t.Fatalf("la sección %q de ABIERTO.md no está después de %q: las secciones se reordenaron y este barrido leería una tabla por otra", s, secciones[i-1])
		}
		previo = corte[i]
	}

	// Las dos tablas vivas, con lo que cada una promete.
	for _, tabla := range []struct {
		nombre     string
		cuerpo     string
		encabezado string
		prefijo    string
		piso       int
		celdas     int
		porque     string
	}{
		{
			nombre: "1 · Con slice asignado", cuerpo: texto[corte[0]:corte[1]],
			encabezado: "| # | Qué falta | Por qué no está | Slice |",
			prefijo:    "A", piso: 20, celdas: 4,
			porque: "no queda ni un cabo con dueño: o se terminó todo, o alguien vació la tabla",
		},
		{
			nombre: "2 · Decisiones de NO hacer", cuerpo: texto[corte[1]:corte[2]],
			encabezado: "| # | Qué | Por qué no |",
			prefijo:    "B", piso: 15, celdas: 3,
			porque: "no queda ni una decisión de no-hacer: sospechoso",
		},
	} {
		if !strings.Contains(tabla.cuerpo, tabla.encabezado) {
			t.Errorf("la tabla «%s» de ABIERTO.md perdió su encabezado %q.\n  Sin encabezado no hay columnas, y las guardas que cuentan celdas dejan de tener contra qué contar.",
				tabla.nombre, tabla.encabezado)
		}
		fila := regexp.MustCompile(`(?m)^\| (` + tabla.prefijo + `\d+) \|`)
		filas := fila.FindAllStringSubmatch(tabla.cuerpo, -1)
		if len(filas) < tabla.piso {
			t.Errorf("la tabla «%s» de ABIERTO.md tiene %d fila(s) y el piso es %d — %s.\n  Un registro con la tabla vaciada se lee igual que uno donde no queda nada abierto, y son cosas opuestas.",
				tabla.nombre, len(filas), tabla.piso, tabla.porque)
		}
		for _, l := range strings.Split(tabla.cuerpo, "\n") {
			if !fila.MatchString(l) {
				continue
			}
			c := celdasDeFila(l)
			if len(c) != tabla.celdas {
				continue // lo cuenta TestTodaFilaDeAbiertoTieneLasCeldasDeSuEncabezado, con su mensaje
			}
			for j, celda := range c {
				if strings.TrimSpace(celda) == "" {
					t.Errorf("la fila %s de la tabla «%s» tiene la celda %d VACÍA.\n  «Nada queda abierto sin dueño» es la primera línea de este archivo: una celda vacía es un cabo sin dueño con cara de fila completa.",
						strings.TrimSpace(c[0]), tabla.nombre, j+1)
				}
			}
		}
	}

	// Las reglas son la parte del archivo que las guardas citan en sus mensajes. Si desaparecen,
	// cada «anotalo en ABIERTO.md» manda a un lugar que ya no explica cómo se anota.
	reglas := texto[corte[3]:]
	for i := 1; i <= 6; i++ {
		if !strings.Contains(reglas, "\n"+strconv.Itoa(i)+". ") {
			t.Errorf("«Cómo se usa este archivo» perdió la regla %d.\n  Las guardas del track mandan a leerla; una regla que no está se lee como una regla que no existe.", i)
		}
	}
}

// celdasDeFila parte una fila de tabla en sus celdas REALES: las barras escapadas (`\|`) son texto
// —GFM las respeta adentro de un code span— y no separan nada. Es la misma cuenta que hace
// TestTodaFilaDeAbiertoTieneLasCeldasDeSuEncabezado, y por el mismo falso positivo medido: la fila
// de A98 lleva `Get-Process musubi \| Select-Object` en su celda.
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

// ────────────────────────────────────────────────────────────────────────────────────────────
// LA REGLA 1 NO TENÍA GUARDA, Y ERA LA MÁS ROTA DE LAS SEIS
//
// «Al cerrar un slice, BORRAR su línea de la tabla 1» se cumplió a mano hasta que dejó de
// cumplirse. Medido el 2026-09-10: **21 de las 48 filas** de la tabla 1 declaraban en su propia
// celda de estado que el cabo ya estaba cerrado. La tabla que contesta «¿qué falta?» contestaba
// con un 44 % de ruido, y quien contara filas para dimensionar lo que queda contaba casi el doble.
//
// Y no era sólo ruido: **A99 y A102 decían «✔ cerrado; falta el redespliegue»**, que es una
// contradicción en cuatro palabras. Un barrido que filtre las filas SIN «cerrado» no las ve, y uno
// que filtre las que LO tienen las borra siendo cabos vivos. La palabra tapaba las dos lecturas.
//
// POR QUÉ NO ES UN grep DE «cerrado» SOBRE LA CELDA
//
// Porque ese grep tiene un falso positivo REAL y puesto: la celda de estado de **A31** arrastra
// párrafos de medición, y adentro dice «`VerificarFirma` **falla cerrado** y lo dice con todas las
// letras» — el modo de fallo de una verificación de firma, no el estado de un cabo. Una guarda que
// acusa a una fila correcta se termina apagando, y este repo ya midió siete guardas satisfechas
// por un comentario, un mensaje de error o un prefijo: preguntar por la palabra donde no decide
// nada es el defecto, no el descuido.
//
// Así que la guarda mira la CABEZA DEL VEREDICTO de la columna que decide (la última de la tabla 1,
// «Slice»): su primera oración, que es lo que el lector toma como estado de la fila. Ahí «cerrado»
// sí es un estado. Los dos controles están clavados: los veredictos reales tienen que dispararla, y
// la celda de A31 no.
//
// Sabotaje que la hace fallar: ponerle `| — (cerrado) |` a cualquier fila de la tabla 1.
// ────────────────────────────────────────────────────────────────────────────────────────────

var (
	// El vocabulario con el que ESTE archivo declara resuelta una fila. No se inventó ninguno: los
	// tres salen de las 22 celdas de estado que estaban puestas el 2026-09-10 («— (cerrado)»,
	// «✔ cerrado el 2026-09-05», «(decisión tomada: cerrado)», «(nada pendiente; queda como
	// registro)»). Agregar uno es una decisión visible acá, no un patrón más ancho.
	veredictoDeCierre = regexp.MustCompile(`(?i)\bcerrad[oa]s?\b|\bnada pendiente\b|\bqueda como registro\b`)
	enfasisMarkdown   = regexp.MustCompile("[*~`✔]+")
	filaDeTabla1      = regexp.MustCompile(`^\| (A\d+) \|`)
)

// cabezaDelVeredicto devuelve la primera oración de la celda de estado, sin el énfasis de Markdown.
// Es lo que un lector toma como VEREDICTO de la fila; lo que venga detrás es la medición que la
// sostiene, y ahí las palabras no deciden.
func cabezaDelVeredicto(celda string) string {
	limpia := strings.TrimSpace(enfasisMarkdown.ReplaceAllString(celda, ""))
	if i := strings.Index(limpia, ". "); i >= 0 {
		return limpia[:i]
	}
	return strings.TrimSuffix(limpia, ".")
}

func TestNingunaFilaDeLaTabla1SeDeclaraCerrada(t *testing.T) {
	texto := registroDeAbiertos(t)

	// CONTROL POSITIVO: los veredictos que REALMENTE estuvieron puestos tienen que dispararla.
	// Sin esto, aflojar `veredictoDeCierre` o `cabezaDelVeredicto` deja la prueba en verde para
	// siempre sin mirar nada, que es la forma en que estas guardas se apagan solas.
	for _, real := range []string{
		"— (cerrado)",
		"✔ cerrado el 2026-09-05, repo y máquina",
		"**gio** ✔ cerrado; falta el redespliegue",
		"**gio** (decisión tomada: cerrado y verificado en vivo)",
		"**gio** (nada pendiente; queda como registro)",
	} {
		if !veredictoDeCierre.MatchString(cabezaDelVeredicto(real)) {
			t.Fatalf("el reconocedor ya no ve como cierre %q, que es una celda que ESTUVO puesta en la tabla 1.\n  Se aflojó, y con eso esta prueba dejó de poder fallar.", real)
		}
	}
	// CONTROL NEGATIVO — EL FALSO POSITIVO CONOCIDO, CLAVADO CON EL TEXTO REAL DE A31.
	// «falla cerrado» es cómo se comporta `VerificarFirma`, no el estado del cabo. Si alguien
	// reescribe esta guarda como un grep de la celda entera, ESTE control es el que lo frena.
	a31 := "~~**acción del operador** (cuesta plata y trámite)~~ **YA NO CUESTA PLATA — ver abajo.** " +
		"**Medido de nuevo el 2026-09-01**: `VerificarFirma` **falla cerrado** y lo dice con todas las letras."
	if veredictoDeCierre.MatchString(cabezaDelVeredicto(a31)) {
		t.Fatal("la guarda da por CERRADA la celda de A31, que dice «`VerificarFirma` falla cerrado».\n  Está preguntando por la palabra y no por el veredicto: es el falso positivo que esta prueba existe para no tener.")
	}

	desde := inicioTabla1.FindStringIndex(texto)
	hasta := regexp.MustCompile(`(?m)^## 2 ·`).FindStringIndex(texto)
	if desde == nil || hasta == nil || hasta[0] <= desde[0] {
		t.Fatalf("no se encontraron las secciones «## 1 ·» y «## 2 ·» de ABIERTO.md; el barrido no está mirando donde cree")
	}

	revisadas := 0
	for _, l := range strings.Split(texto[desde[0]:hasta[0]], "\n") {
		m := filaDeTabla1.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		c := celdasDeFila(l)
		if len(c) < 2 {
			continue
		}
		revisadas++
		cabeza := cabezaDelVeredicto(c[len(c)-1])
		if veredictoDeCierre.MatchString(cabeza) {
			t.Errorf("la fila **%s** de la tabla 1 declara en su columna de estado que el cabo está resuelto:\n    %s\n"+
				"  La regla 1 de este archivo dice que al cerrar un slice se BORRA su línea de la tabla 1 y su texto baja a la sección 3.\n"+
				"  Una fila cerrada adentro de la tabla que contesta «¿qué falta?» es una respuesta falsa, y encima rompe el conteo: el 2026-09-10 eran 21 de 48.\n"+
				"  Si el cabo NO está cerrado del todo, el estado tiene que decir qué falta (así se arreglaron A99 y A102, que decían «cerrado; falta el redespliegue»).",
				m[1], cabeza)
		}
	}
	// CERO FILAS REVISADAS NO ES «TODO LIMPIO»: es que el parseo dejó de encontrar la tabla.
	if revisadas < 20 {
		t.Fatalf("sólo se revisaron %d fila(s) de la tabla 1 y el piso es 20: cambió el formato del archivo y esta guarda dejó de mirar", revisadas)
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
	revisadas := map[int]bool{}
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

	// CONTROL DE COBERTURA: TODA FILA CON IDENTIFICADOR TIENE QUE HABER SIDO REVISADA.
	//
	// «3 tablas y 30 filas» NO ALCANZABA, y se midió el 2026-09-05: la tabla de la sección 1 estaba
	// PARTIDA en la fila de A92 —le faltaba la barra de cierre y su celda de dueño, y detrás venía un
	// blockquote de corrección metido adentro de la tabla— así que el recorrido la daba por terminada
	// ahí y las DIECISÉIS filas siguientes (A93 … A111) no se revisaban. Con 4 tablas y 47 filas
	// contadas, el control seguía en verde: contaba contra un número fijo que ya estaba superado.
	//
	// Y NO ERA SÓLO LA PRUEBA. En Markdown un blanco TERMINA la tabla, así que esas dieciséis filas
	// se dibujaban como texto suelto con barras verticales — la mitad reciente del registro dejó de
	// leerse como registro, que es exactamente el daño que B20 ya había medido con una celda.
	//
	// Este control cuenta lo revisado contra lo que el archivo TIENE, que es la única cuenta que no
	// envejece: cada fila `| A<n> |` o `| B<n> |` tiene que estar entre las visitadas, y si falta, se
	// la NOMBRA — que es lo que convierte «algo no se revisó» en «buscá la fila anterior a A93».
	//
	// Sabotaje que la hace fallar: quitarle la barra de cierre a cualquier fila que no sea la última
	// de su tabla, o meter una línea en blanco entre dos filas.
	idDeFila := regexp.MustCompile(`^\| ([AB]\d+) \|`)
	var sinRevisar []string
	primera := 0
	for i, l := range lineas {
		if m := idDeFila.FindStringSubmatch(l); m != nil && !revisadas[i] {
			if primera == 0 {
				primera = i + 1
			}
			sinRevisar = append(sinRevisar, m[1])
		}
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
