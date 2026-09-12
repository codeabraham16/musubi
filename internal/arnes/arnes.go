// Package arnes lee los sabotajes que el árbol declara en sus comentarios y los vuelve
// EJECUTABLES.
//
// ═════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EXISTE: EL ÁRBOL PROMETE 774 SABOTAJES Y NADIE PODÍA CORRERLOS
//
// Este repo pide que toda guarda venga con un sabotaje que la ponga en rojo, y la disciplina
// rinde: `deploy/pruebas/sabotaje.sh` —que corre UNO— encontró seis guardas huecas el día que se
// escribió. Pero el sabotaje se declara EN PROSA, y la prosa no se puede correr.
//
// LOS NÚMEROS, TODOS MEDIDOS EL 2026-09-12 SOBRE `git ls-files` EN LA BASE 00728f1, y cada uno con
// su definición pegada, porque son respuestas a preguntas DISTINTAS y mezclarlas es cómo se llegó
// al problema que este paquete viene a arreglar:
//
//	«¿cuántas promesas de sabotaje hay declaradas en un comentario?»
//	    774 anclas en 163 archivos de prueba  ← lo que cuenta este lector, con go/ast, 30 formas
//
//	«¿cuántas usan la frase canónica "Sabotaje que la hace fallar"?»
//	    396 anclas en  93 archivos            ← lo que cuenta el cabo A123, y cuenta BIEN
//
//	«¿cuántas quedan afuera de esa frase?»
//	    378 anclas, y 70 archivos de prueba en los que NINGUNA ancla la usa
//
//	«¿cuántas funciones Test hay en total?»
//	    3.213                                 ← el denominador: las 774 son el 24 %
//	    (`grep "^func Test"` dice 3.219: cuenta SEIS `func TestMain` que viven adentro de
//	     literales crudos en internal/testbudget/paquetes_test.go, o sea fixtures de una guarda.
//	     El AST no los ve. Ese 3.219 estuvo escrito acá y era el número del grep — lo levantó
//	     otra sesión midiéndolo con su propio go/ast. Es el mismo defecto que el de más abajo,
//	     esta vez en el DENOMINADOR: un fixture contaminando el censo.)
//
//	«¿cuántas tuvieron veredicto alguna vez?»
//	    38, a mano, en las auditorías A107 y A120 — el 4,9 % de 774
//	     6 de esas 38 NO rompían lo que decían romper — el 15,8 % de lo auditado MIENTE
//
// EL 774 NO ES UN HECHO DEL MUNDO: ES LA SALIDA DE UN PREDICADO, Y EL PREDICADO VA ESCRITO
//
// Otra sesión replicó este censo con su propio programa de go/ast sobre 00728f1 pelado, sin una
// línea compartida con éste. Reprodujo 396/93 AL DÍGITO, y el total NO:
//
//	569 / 160   su predicado: «sabotaje» + una palabra de consecuencia a ≤80 chars
//	774 / 163   este predicado (abajo)
//	955 / 207   la cota superior: cualquier comentario que mencione la palabra
//
// O sea que el ESCALAR es hipersensible a la definición y el 774 sólo significa algo con su
// predicado al lado. Publicarlo pelado sería A123 otra vez, un nivel más arriba. Lo que SÍ
// converge es el conteo de ARCHIVOS —160 contra 163, y 65 contra 70 sin ninguna canónica—, así
// que la forma del hallazgo se sostiene aunque el total dependa de quién pregunta.
//
// EL PREDICADO DE ESTE LECTOR, exacto (es `esAncla`, y esto es su prosa):
//
//	una línea de comentario que, después de sacarle viñetas y espacios,
//	  · empieza con «sabotaje» o «sabotajes» (sin importar la caja), Y
//	  · tiene dos puntos en esa misma línea, Y
//	  · o arranca con «Sabotaje» en mayúscula inicial, o su línea anterior cerró oración.
//
// Y UNA CONSECUENCIA PRÁCTICA PARA QUIEN TOQUE `esAncla`: el techo de la guarda del censo
// (`anclasEnProsaAlDia`) está medido CON ESTE PREDICADO. Si lo cambiás, el número deja de valer y
// hay que volver a medirlo EN EL MISMO COMMIT, diciéndolo. Subir el techo porque el predicado se
// ensanchó y no decirlo convierte la guarda en la defensora del defecto que vigila.
//
// A123 NO CONTÓ MAL: contó INCOMPLETO, y la diferencia importa. Cuenta exactamente la frase que
// enumera, y su 396/93 se reprodujo al dígito. Lo que pasa es que el árbol escribe la misma
// promesa de 30 formas —«Sabotaje:», «Sabotaje que la pone roja:», «Sabotaje visto rojo:»,
// «Sabotajes MEDIDOS el 2026-09-05:»…— y 39 anclas llevan el encabezado EN MAYÚSCULAS, de las
// cuales sólo 3 usan la frase canónica: la forma dominante en mayúsculas es `SABOTAJE:`, con 30.
// (Ese detalle está escrito así porque la primera versión de este comentario decía «42 en
// mayúsculas» sin decir de qué conjunto, y quien fuera a reconstruirlo iba a grepear la canónica
// en mayúsculas, encontrar 3, y concluir que este censo está inflado. Lo levantó otra sesión.)
//
// O sea que la contabilidad de la deuda tiene el defecto dominante del repo —una guarda que
// enumera formas no converge— aplicado a la cuenta de su propia deuda. Por eso este lector NO
// enumera frases: pregunta por el TOKEN `Sabotaje` al empezar una línea de comentario, con dos
// puntos en la misma línea. Una forma nueva entra sola.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTE CENSO NO VE, Y SE LO ENCONTRÓ OTRA SESIÓN
//
// Este lector cuenta anclas DECLARADAS EN EL FUENTE. Hay guardas que fueron saboteadas de verdad
// —control en verde, sabotaje aplicado, la línea del rojo leída— y dejaron la evidencia en el
// MENSAJE DEL COMMIT y en el cuerpo del PR, no en el código. Medidas: `normalizacion_fijada_test.go`
// y `carga_streaming_test.go` tienen CERO menciones de la palabra, y están verificadas.
//
// Eso no es la forma 31 de escribir la frase: es otro archivo, y ensanchar el patrón no lo alcanza
// nunca. Y tiene una consecuencia que hay que decir en voz alta: un sabotaje declarado en un
// mensaje de commit NO SE VUELVE A CORRER. La disciplina se cumplió una vez y después no es
// auditable — que es la diferencia entre una prueba y una anécdota. La salida no es ampliar el
// censo al historial: es que la declaración viva donde el arnés la pueda ejecutar.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LA DIRECTIVA VA EN EL COMENTARIO, PEGADA A LA PROSA, Y LA PARSEA EL SCANNER DE GO
//
// Un manifiesto aparte —un YAML, una tabla en otro archivo— sería un DERIVADO ESCRITO A MANO de
// la prosa, y se despega: el código se mueve, el manifiesto queda. Por eso la directiva vive en el
// mismo comentario que la explica, y es la única fuente.
//
// El problema de meter literales de código adentro de un `//` es el escapado: el texto a sabotear
// trae comillas, backticks, barras y llaves. NO se inventa un esquema de escapado —cada esquema es
// una superficie de bugs nueva—: la directiva se escribe con literales de string DE GO y la lee
// `go/scanner`, o sea el mismo lexer que compila el archivo. Las comillas dobles, las crudas con
// backtick y todos los escapes salen gratis y correctos, y `strconv.Quote` al escribir es la
// inversa exacta de `strconv.Unquote` al leer.
//
// CADA LÍNEA LLEVA SU PROPIO `arnes:`, y eso no es verbosidad: es no pelear con gofmt. La primera
// versión usaba una línea y continuaciones indentadas debajo; gofmt las trata como bloque de
// código, les mete una línea `//` en blanco antes, el parser cortaba ahí, y las ocho directivas
// recién escritas quedaron con `de=""`.
//
//	// Sabotaje que la hace fallar: cambiar fleet.CapShell por fleet.CapExec en toolFleetShell.
//	// arnes: archivo="internal/mcp/methods_shell.go"
//	// arnes: de="!existe || !PuedeSobreDevice(p, d, fleet.CapShell)"
//	// arnes: a="!existe || !PuedeSobreDevice(p, d, fleet.CapExec)"
//
// UNA SOLA OPERACIÓN, Y ES A PROPÓSITO: reemplazar un literal ÚNICO por otro. Las tres formas en
// que el árbol describe sus sabotajes —«cambiar X por Y», «sacar el if …», «agregar Z»— son todas
// esa misma operación (borrar es a="", agregar es de="<ancla>" a="<ancla>\n<nuevo>"). Enumerar
// verbos sería volver a enumerar formas.
//
// LA UNICIDAD ES LA GUARDA: si `de` aparece dos veces, no se sabe cuál se tocó, y si aparece cero
// veces el sabotaje NO SE APLICÓ y el verde que venga después no dice nada. Las dos se denuncian.
// Es la falla que ya nos costó una corrida: el arnés que no aplica el sabotaje reporta lo mismo
// que la guarda que no cubre.
//
// LO QUE ESTE PAQUETE NO HACE: no corre nada. Lee y valida. El que corre es
// `deploy/pruebas/sabotaje.sh`, que ya sabe distinguir las OCHO formas de falso verde (control sin
// sabotaje, build roto, cero pruebas ejecutadas, guarda invertida…). Reescribirlo acá sería
// fabricar la función que ya existe.
package arnes

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// PrefijoDirectiva es lo que marca una línea de comentario como directiva legible por máquina.
// ASCII a propósito: la lee un grep, un sed y un humano apurado, y un carácter no-ASCII pegado a
// una variable ya mató un guion en este repo de una forma invisible.
const PrefijoDirectiva = "arnes:"

// Ancla es una promesa de sabotaje encontrada en un comentario.
type Ancla struct {
	Archivo string // ruta relativa a la raíz del repo, con `/`
	Linea   int    // línea de la PRIMERA línea del ancla
	Prosa   string // el texto tal cual, para que un humano lo reconozca
	Prueba  string // la función Test a la que el comentario está pegado; "" si flota

	// LineaFin es la última línea de PROSA de este ancla: el punto donde una directiva `arnes:`
	// tiene que insertarse para quedar pegada a su propia explicación y no a la del ancla vecina.
	LineaFin int

	// Directiva, si la hay. Nil = ancla en prosa y nada más: la deuda.
	Directiva *Directiva

	// NoMecanizable, si está declarado, es el motivo por el que este sabotaje NO se puede
	// escribir como un reemplazo de texto. Hay una clase ESTRUCTURAL medida el 2026-09-05 y
	// explica los 12 que habían quedado sin veredicto: cuando la guarda es el único lector de
	// una variable o el único uso de un import, borrarla deja `declared and not used` y el
	// sabotaje literal NO PUEDE COMPILAR NUNCA. Un rojo por build roto se lee igual que un rojo
	// por guarda que funciona, así que ese sabotaje no prueba nada y hay que decirlo, no correrlo.
	NoMecanizable string

	// Quejas son los problemas de ESTA directiva. Un ancla con directiva Y quejas NO cuenta como
	// mecanizada: contarla infla la cobertura con sabotajes que no se pueden correr, que es la
	// misma mentira que este arnés viene a cazar, una vuelta más adentro.
	Quejas []string
}

// Directiva es un sabotaje ejecutable: qué archivo, qué texto por qué texto.
type Directiva struct {
	Archivo string // el archivo de PRODUCCIÓN a sabotear (no el de prueba)
	De      string // literal a reemplazar; tiene que aparecer EXACTAMENTE UNA VEZ
	A       string // con qué se reemplaza; "" borra

	// Paquete y Prueba se DERIVAN si no se declaran: el paquete es el directorio del archivo de
	// prueba donde vive el ancla, y la prueba es la función a la que el comentario está pegado.
	// Se declaran sólo cuando el ancla flota o cuando la guarda vive en otro paquete.
	Paquete string
	Prueba  string

	// ArregloDe/ArregloA son la OTRA DIRECCIÓN, la falla 7 de sabotaje.sh: un cambio LEGÍTIMO que
	// la guarda tiene que seguir aceptando. Sin esto no se sabe si la guarda castiga el arreglo
	// —premia el defecto y manda a sacar la línea buena—, que es peor que no mirar.
	ArregloDe string
	ArregloA  string
}

// Censo es lo que el árbol declara, contado.
type Censo struct {
	Raiz     string
	Anclas   []Ancla
	Archivos int // archivos de prueba mirados
	Quejas   []string

	// FuncionesTest y PruebasConAncla son EL DENOMINADOR HONESTO, y están acá porque otra sesión
	// le encontró el agujero a la primera versión de este censo.
	//
	// Este lector cuenta ANCLAS DECLARADAS EN EL FUENTE. No cuenta guardas que fueron saboteadas
	// de verdad y dejaron la evidencia en OTRO LADO: `musubi-62` midió cuatro guardas suyas
	// —normalizacion_fijada, carga_streaming, red_del_parser, procedencia_del_vector— con control
	// en verde, sabotaje aplicado y la línea del rojo leída, y la evidencia vive en el mensaje del
	// commit y en el cuerpo del PR. En el fuente no hay ni una mención.
	//
	// Eso NO es la forma 31 de escribir la frase: es otro archivo, y ensanchar el patrón no lo
	// alcanza nunca. Y tiene una consecuencia que hay que decir: un sabotaje declarado en un
	// mensaje de commit NO SE VUELVE A CORRER. La disciplina se cumplió una vez y después no es
	// auditable — que es exactamente la diferencia entre una prueba y una anécdota.
	//
	// Por eso el censo publica las dos cosas: cuántas anclas hay, y sobre cuántas funciones Test.
	// Decir «774 promesas sin correr» sin el denominador se lee como si 774 fuera el universo, y
	// es el 24 %.
	//
	// Y SE CUENTA CON EL AST Y NO CON UN grep, que no es un detalle de estilo: `grep "^func Test"`
	// da 3.219 y el AST 3.213, y las seis de diferencia son `func TestMain` que viven adentro de
	// literales crudos en `internal/testbudget/paquetes_test.go` — fixtures de otra guarda. Un
	// denominador inflado hace que la cobertura se vea peor de lo que es, que es el error simétrico
	// del que este arnés persigue. Lo levantó otra sesión comparando su AST contra mi grep.
	//
	// OJO CON LEER LA RESTA COMO DEUDA PURA: una prueba de tabla con quince subtests lleva UN
	// ancla, un ancla puede cubrir tres pruebas hermanas, y un helper con nombre `Test…` no
	// defiende ningún invariante. La resta es el techo de lo que falta, no su medida.
	FuncionesTest   int
	PruebasConAncla int

	// SinUbicar son anclas que el lector vio en el texto crudo y NO pudo colocar en ningún
	// comentario del AST. Tiene que ser cero: si no lo es, el lector tiene un agujero y hay que
	// decirlo, no restarlo del total en silencio.
	SinUbicar []string

	// SinTrackear son `_test.go` que existen en el árbol y NO están en el índice de git.
	//
	// ESTO ME PASÓ ESCRIBIENDO ESTE PAQUETE Y POR ESO ESTÁ ACÁ. El censo deriva de `git ls-files`
	// —que es lo correcto: barrer el disco mete adentro `.claude/` con sus ~64k `.go` ajenos— pero
	// el índice no conoce un archivo nuevo. Escribí ocho directivas en un `_test.go` recién creado
	// y el censo contestó «0 mecanizadas»: la MISMA forma que «restaurar un archivo nuevo con
	// `git checkout` deja el sabotaje puesto», que este repo ya pagó.
	//
	// En CI esto es siempre vacío. En la máquina de quien escribe es justo lo que está escribiendo,
	// así que se AVISA y no se falla: un cero que significa «todavía no lo agregaste» no puede
	// salir por la misma puerta que un cero que significa «no hay nada».
	SinTrackear []string
}

// Mecanizadas es la primera de las CUATRO categorías en que se parte el censo —con Rotas, Exentas
// y Pendientes— y son las anclas cuya directiva se pudo leer: las únicas que este arnés corre.
//
// La cuarta —Rotas— existe porque la primera versión no la tenía y contaba como mecanizada una
// directiva ilegible: la cobertura subía con sabotajes que nadie podía correr.
func (c Censo) Mecanizadas() []Ancla {
	return c.filtrar(func(a Ancla) bool { return a.Directiva != nil && len(a.Quejas) == 0 })
}

// Rotas son las anclas que declararon una directiva y la directiva no se pudo leer.
func (c Censo) Rotas() []Ancla {
	return c.filtrar(func(a Ancla) bool { return a.Directiva != nil && len(a.Quejas) > 0 })
}

// Exentas son las anclas que declararon POR QUÉ su sabotaje no se puede mecanizar.
func (c Censo) Exentas() []Ancla {
	return c.filtrar(func(a Ancla) bool { return a.Directiva == nil && a.NoMecanizable != "" })
}

// Pendientes son las anclas que siguen sólo en prosa: la deuda que este arnés mide.
func (c Censo) Pendientes() []Ancla {
	return c.filtrar(func(a Ancla) bool {
		return a.Directiva == nil && a.NoMecanizable == "" && len(a.Quejas) == 0
	})
}

func (c Censo) filtrar(ok func(Ancla) bool) []Ancla {
	var out []Ancla
	for _, a := range c.Anclas {
		if ok(a) {
			out = append(out, a)
		}
	}
	return out
}

// ArchivosDePrueba pregunta a git qué `_test.go` trackea el repo.
//
// `git ls-files` Y NO UN BARRIDO DEL DISCO, y no es una preferencia de estilo: `.claude/` tiene
// ~64.149 `.go` de otros proyectos y un respaldo sin trackear en `.musubi/` ya hizo que una guarda
// acusara a tres lectores que no existían. Un barrido falla SÓLO EN LOCAL y pasa SIEMPRE en CI,
// que es el peor de los dos mundos.
func ArchivosDePrueba(raiz string) ([]string, error) {
	salida, err := exec.Command("git", "-C", raiz, "ls-files", "-z", "*_test.go").Output()
	if err != nil {
		return nil, fmt.Errorf("no pude preguntarle a git qué trackea desde %s: %w — no medí nada", raiz, err)
	}
	var out []string
	for _, rel := range strings.Split(string(salida), "\x00") {
		if rel == "" {
			continue
		}
		// `ls-files` lee el ÍNDICE: un archivo borrado del árbol y todavía sin `git add` sigue
		// listado, y abrirlo fallaría.
		if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash(rel))); err != nil {
			continue
		}
		out = append(out, rel)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("git no listó NI UN `_test.go` trackeado desde %s: eso no es «no hay pruebas», "+
			"es que este enumerador no miró nada", raiz)
	}
	sort.Strings(out)
	return out, nil
}

// pruebasSinTrackear pregunta por los `_test.go` que están en el árbol y no en el índice.
//
// `--exclude-standard` respeta `.gitignore`, así que lo ignorado a propósito no aparece. Un fallo
// de git acá no es fatal —esto es un aviso, no una medición— pero tampoco se inventa un vacío:
// devolver nil cuando no se pudo preguntar es lo correcto sólo porque el llamador ya no depende de
// esto para decidir nada.
func pruebasSinTrackear(raiz string) []string {
	salida, err := exec.Command("git", "-C", raiz, "ls-files", "-z", "-o", "--exclude-standard", "*_test.go").Output()
	if err != nil {
		return nil
	}
	var out []string
	for _, rel := range strings.Split(string(salida), "\x00") {
		if rel != "" {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out
}

// Censar lee todo el árbol y devuelve lo que declara.
func Censar(raiz string) (Censo, error) {
	archivos, err := ArchivosDePrueba(raiz)
	if err != nil {
		return Censo{}, err
	}
	c := Censo{Raiz: raiz, Archivos: len(archivos), SinTrackear: pruebasSinTrackear(raiz)}
	for _, rel := range archivos {
		r, err := censarArchivo(raiz, rel)
		if err != nil {
			// UN ARCHIVO QUE NO SE PUEDE LEER ES UNA QUEJA, NO UN CERO. Saltearlo en silencio es
			// exactamente «un cero que significa no sé».
			c.Quejas = append(c.Quejas, fmt.Sprintf("%s: no pude parsearlo: %v", rel, err))
			continue
		}
		c.Anclas = append(c.Anclas, r.anclas...)
		c.Quejas = append(c.Quejas, r.quejas...)
		c.SinUbicar = append(c.SinUbicar, r.sinUbicar...)
		c.FuncionesTest += r.funcionesTest
		c.PruebasConAncla += r.pruebasConAncla
	}
	return c, nil
}

// censarArchivo parsea UN archivo de prueba.
//
// `parser.ParseComments` Y NO EL MODO 0, y acá eso es todo el punto: con el modo 0 el parser
// DESCARTA los comentarios, así que este lector encontraría cero anclas y diría que el árbol no
// promete nada. Es la trampa simétrica de la que ya pagamos en la guarda del candado, donde el
// modo 0 hacía que ningún comentario pudiera satisfacer la guarda: ahí el bug era ver comentarios,
// acá es no verlos.
// loDeUnArchivo es lo que sale de censarArchivo. Es un struct y no seis valores de retorno
// porque seis valores posicionales es cómo se termina pasando `quejas` donde iba `sinUbicar`.
type loDeUnArchivo struct {
	anclas          []Ancla
	quejas          []string
	sinUbicar       []string
	funcionesTest   int
	pruebasConAncla int
}

func censarArchivo(raiz, rel string) (loDeUnArchivo, error) {
	ruta := filepath.Join(raiz, filepath.FromSlash(rel))
	src, err := os.ReadFile(ruta)
	if err != nil {
		return loDeUnArchivo{}, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, ruta, src, parser.ParseComments)
	if err != nil {
		return loDeUnArchivo{}, err
	}

	// A qué prueba está pegado cada grupo de comentarios. El `Doc` de un FuncDecl es el grupo
	// inmediatamente anterior, que es exactamente dónde el árbol escribe sus anclas.
	dePrueba := map[*ast.CommentGroup]string{}
	funcionesTest, conAncla := 0, 0
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name == nil {
			continue
		}
		// LA FUNCIÓN TIENE QUE SER UNA PRUEBA DE VERDAD: nombre `Test…` y receptor nil. Contar
		// helpers con nombre `Test…` infla el denominador, y un denominador inflado hace que la
		// cobertura se vea peor de lo que es — el error simétrico del que este arnés persigue.
		esPrueba := fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test")
		if esPrueba {
			funcionesTest++
		}
		if fn.Doc == nil {
			continue
		}
		dePrueba[fn.Doc] = fn.Name.Name
		if esPrueba && grupoTieneAncla(fset, fn.Doc) {
			conAncla++
		}
	}

	var anclas []Ancla
	var quejas, sinUbicar []string
	vistas := map[int]bool{} // líneas ya colocadas, para cruzar contra el texto crudo

	for _, g := range f.Comments {
		lineas := lineasDe(fset, g)
		for i, ln := range lineas {
			if !esAncla(ln.texto, anteriorDe(lineas, i)) {
				continue
			}
			vistas[ln.linea] = true
			// EL ALCANCE DE UN ANCLA TERMINA DONDE EMPIEZA LA SIGUIENTE, y no al final del grupo.
			// La primera versión buscaba las directivas en TODO el grupo: dos anclas en un mismo
			// bloque de comentarios —que el árbol tiene, p. ej. despliegue_poda_test.go— se
			// quedaban las dos con las MISMAS directivas, o sea que una de las dos declaraba un
			// sabotaje que no era el suyo. Es la forma de siempre: una respuesta plausible a otra
			// pregunta.
			fin := len(lineas)
			for j := i + 1; j < len(lineas); j++ {
				if esAncla(lineas[j].texto, anteriorDe(lineas, j)) {
					fin = j
					break
				}
			}
			mias := lineas[i:fin]
			a := Ancla{
				Archivo:  rel,
				Linea:    ln.linea,
				LineaFin: finDeLaProsa(mias),
				Prosa:    prosaDesde(lineas, i),
				Prueba:   dePrueba[g],
			}
			d, motivo, qs := directivaDe(mias, rel, a.Prueba)
			a.Directiva, a.NoMecanizable, a.Quejas = d, motivo, qs
			for _, q := range qs {
				quejas = append(quejas, fmt.Sprintf("%s:%d: %s", rel, ln.linea, q))
			}
			anclas = append(anclas, a)
		}
	}

	// EL CONTROL DE «LOS VI A TODOS»: el mismo criterio, aplicado al texto crudo. Si una línea
	// parece un ancla y el AST no la colocó, este lector tiene un agujero — y un agujero en el
	// enumerador se ve idéntico a un árbol sin deuda.
	anterior := ""
	for n, linea := range strings.Split(string(src), "\n") {
		t := strings.TrimSpace(linea)
		if !strings.HasPrefix(t, "//") {
			anterior = "" // se cortó el bloque de comentarios: lo que siga arranca oración
			continue
		}
		cuerpo := strings.TrimSpace(strings.TrimPrefix(t, "//"))
		if esAncla(cuerpo, anterior) && !vistas[n+1] {
			sinUbicar = append(sinUbicar, fmt.Sprintf("%s:%d: %s", rel, n+1, t))
		}
		anterior = cuerpo
	}
	return loDeUnArchivo{
		anclas:          anclas,
		quejas:          quejas,
		sinUbicar:       sinUbicar,
		funcionesTest:   funcionesTest,
		pruebasConAncla: conAncla,
	}, nil
}

// grupoTieneAncla dice si un comentario de doc promete al menos un sabotaje.
func grupoTieneAncla(fset *token.FileSet, g *ast.CommentGroup) bool {
	lineas := lineasDe(fset, g)
	for i, l := range lineas {
		if esAncla(l.texto, anteriorDe(lineas, i)) {
			return true
		}
	}
	return false
}

type lineaCom struct {
	linea int
	texto string // sin el `//` y sin espacios al borde
	cruda string // sin el `//`, con la indentación interna intacta
}

// lineasDe aplana un grupo de comentarios a líneas con su número REAL en el archivo.
//
// El número sale del FileSet y no de contar líneas a mano: un `/* */` de varias líneas y un
// archivo con `\r\n` desalinean cualquier conteo propio, y un número de línea equivocado en el
// mensaje de una guarda manda a mirar el lugar que no es.
func lineasDe(fset *token.FileSet, g *ast.CommentGroup) []lineaCom {
	var out []lineaCom
	for _, c := range g.List {
		t := c.Text
		base := fset.Position(c.Slash).Line
		switch {
		case strings.HasPrefix(t, "//"):
			cuerpo := strings.TrimPrefix(t, "//")
			out = append(out, lineaCom{linea: base, texto: strings.TrimSpace(cuerpo), cruda: cuerpo})
		case strings.HasPrefix(t, "/*"):
			// Un bloque `/* */` cuenta como varias líneas. El árbol no los usa para anclas, pero
			// no se descartan en silencio.
			cuerpo := strings.TrimSuffix(strings.TrimPrefix(t, "/*"), "*/")
			for i, l := range strings.Split(cuerpo, "\n") {
				out = append(out, lineaCom{linea: base + i, texto: strings.TrimSpace(l), cruda: l})
			}
		}
	}
	return out
}

// esAncla decide si una línea de comentario ABRE una promesa de sabotaje.
//
// NO ENUMERA FRASES. Pregunta por el token `Sabotaje`/`Sabotajes` al EMPEZAR la línea, con dos
// puntos en la misma línea. Con eso entran las 29 formas que el árbol ya escribió —«Sabotaje que
// la hace fallar:», «Sabotaje:», «Sabotaje visto rojo:», «Sabotajes MEDIDOS el 2026-09-05:»…— y
// las que alguien escriba mañana.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL PEDAZO QUE NO ES OBVIO: LA ORACIÓN QUE SE ENVUELVE
//
// La primera versión de este lector contó 780 anclas donde el grep estricto veía 732, y las 48 de
// más NO eran anclas que el grep subcontaba: eran menciones a mitad de oración que cayeron al
// EMPEZAR el renglón porque el comentario se envolvió ahí.
//
//	// La matriz mide que se PREGUNTE. Que la respuesta llegue es otra cosa, y lo descubrió un
//	// sabotaje: cambiar `ShellSinPermiso` por `ShellAbriendo` … dejaba la matriz en VERDE.
//
// Esa segunda línea arranca con «sabotaje» y tiene dos puntos, y no promete nada: el ancla de
// verdad está cuatro líneas más abajo. Un lector que las cuente infla la deuda con ruido, y una
// deuda inflada es tan inútil como una subcontada — la diferencia se la come el que decide.
//
// EL DISCRIMINADOR NO ES LA MAYÚSCULA, ES SI LA LÍNEA ANTERIOR CERRÓ ORACIÓN. Pedir mayúscula
// alcanzaría hoy (las 29 formas la tienen) pero dejaría muda a un ancla escrita en minúscula, que
// es justo la que nadie revisaría. Con la regla posicional entran las dos y el ruido queda afuera.
func esAncla(texto, anterior string) bool {
	t := strings.TrimLeft(texto, "·-*# \t")
	bajo := strings.ToLower(t)
	if !strings.HasPrefix(bajo, "sabotaje") {
		return false
	}
	// Los dos puntos tienen que venir DESPUÉS de la palabra y en esta misma línea: es lo que
	// separa un encabezado («Sabotaje que la hace fallar: …») de una oración que arranca con la
	// palabra y sigue de largo.
	if !strings.Contains(t, ":") {
		return false
	}
	if strings.HasPrefix(t, "Sabotaje") {
		return true
	}
	return cierraOracion(anterior)
}

// cierraOracion dice si la línea de comentario anterior TERMINÓ una oración, y por lo tanto la que
// sigue empieza una nueva. Una línea vacía, una regla de separación y el arranque de un grupo
// cuentan como cierre.
func cierraOracion(anterior string) bool {
	t := strings.TrimRight(strings.TrimSpace(anterior), " \t")
	if t == "" {
		return true
	}
	// Las reglas con las que el árbol separa secciones (═, ─, ━) no son texto: lo que venga
	// después arranca de cero.
	if strings.Trim(t, "═─━=-_") == "" {
		return true
	}
	r := []rune(t)
	return strings.ContainsRune(".:;!?»)", r[len(r)-1])
}

// anteriorDe devuelve la línea de comentario previa dentro del mismo grupo, o "" si es la primera.
func anteriorDe(lineas []lineaCom, i int) string {
	if i <= 0 {
		return ""
	}
	return lineas[i-1].texto
}

func prosaDesde(lineas []lineaCom, i int) string {
	var sb strings.Builder
	for j := i; j < len(lineas); j++ {
		t := lineas[j].texto
		if j > i && (t == "" || esAncla(t, anteriorDe(lineas, j)) || esDirectiva(t)) {
			break
		}
		if esDirectiva(t) {
			break
		}
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(t)
	}
	return strings.TrimSpace(sb.String())
}

// finDeLaProsa devuelve la última línea de comentario de este ancla que todavía es PROSA: se corta
// en la primera línea vacía, en la primera directiva, o al final del alcance del ancla.
//
// Es el punto de inserción. Meter la directiva al final del GRUPO la dejaría, cuando hay dos anclas
// en el mismo bloque, pegada a la explicación de la otra.
func finDeLaProsa(mias []lineaCom) int {
	fin := mias[0].linea
	for i, l := range mias {
		if i > 0 && (l.texto == "" || esDirectiva(l.texto)) {
			break
		}
		fin = l.linea
	}
	return fin
}

func esDirectiva(texto string) bool {
	return strings.HasPrefix(strings.TrimSpace(texto), PrefijoDirectiva)
}

// directivaDe junta la directiva de un grupo de comentarios.
//
// CADA LÍNEA SE DECLARA SOLA, CON SU PROPIO `arnes:`, Y ESO NO ES VERBOSIDAD: ES NO PELEAR CON
// gofmt.
//
// La primera versión usaba una línea `arnes:` y continuaciones indentadas debajo. gofmt las
// reformatea —en un comentario de doc, una línea indentada es un bloque de código, y le inserta
// una línea `//` en blanco antes— así que el parser cortaba en la línea vacía y las ocho
// directivas que acababa de escribir quedaron con `de=""`. El censo las contó como mecanizadas y
// `Validar` informó «el `de` aparece 26808 veces», que es `strings.Count(s, "")` contestando lo
// que se le preguntó.
//
// Dos lecciones, las dos de la casa: una herramienta que pelea con el formateador pierde en
// silencio y en el momento menos oportuno; y `de=""` no significa «borrar todo», significa «no lo
// pude leer» — otro cero que quiere decir «no sé».
func directivaDe(lineas []lineaCom, rel, pruebaDerivada string) (*Directiva, string, []string) {
	var payload strings.Builder
	hay := false
	for _, l := range lineas {
		if !esDirectiva(l.texto) {
			continue
		}
		hay = true
		payload.WriteString(strings.TrimPrefix(strings.TrimSpace(l.texto), PrefijoDirectiva))
		// El salto mantiene separados los pares de líneas distintas: el scanner de Go inserta un
		// punto y coma ahí y `camposDe` lo ignora a propósito.
		payload.WriteString("\n")
	}
	if !hay {
		return nil, "", nil
	}

	campos, quejas := camposDe(payload.String())
	if motivo, ok := campos["no_mecanizable"]; ok {
		// UNA EXENCIÓN SIN MOTIVO NO ES UNA EXENCIÓN, es un `skip`. El motivo es lo único que
		// impide que esta salida se use para vaciar el censo.
		if len(strings.TrimSpace(motivo)) < 20 {
			quejas = append(quejas, "`no_mecanizable` necesita un motivo de verdad (≥20 caracteres): "+
				"la clase estructural medida es «borrar la guarda deja el import huérfano y no compila», "+
				"y sin el motivo escrito esta salida se vuelve un `skip`")
		}
		return nil, motivo, quejas
	}

	d := &Directiva{
		Archivo:   campos["archivo"],
		De:        campos["de"],
		A:         campos["a"],
		Paquete:   campos["paquete"],
		Prueba:    campos["prueba"],
		ArregloDe: campos["arreglo_de"],
		ArregloA:  campos["arreglo_a"],
	}
	if d.Prueba == "" {
		d.Prueba = pruebaDerivada
	}
	if d.Paquete == "" {
		// El paquete se DERIVA del archivo de prueba donde vive el ancla, que es donde vive la
		// guarda. Escribirlo a mano sería una copia de algo que el árbol ya dice.
		d.Paquete = "./" + filepath.ToSlash(filepath.Dir(rel))
	}
	// Los campos obligatorios se EXIGEN acá y no al correr: una directiva incompleta tiene que
	// ser un rojo del censo —barato, en CI, en milisegundos— y no una sorpresa noventa segundos
	// después de arrancar el corredor.
	if d.Archivo == "" {
		quejas = append(quejas, "falta `archivo=\"…\"`: qué archivo de producción se sabotea")
	}
	if d.De == "" {
		quejas = append(quejas, "falta `de=\"…\"`: el literal a reemplazar")
	}
	if d.Prueba == "" {
		quejas = append(quejas, "no pude derivar la prueba (el ancla no está pegada a un `func Test…`): "+
			"declarala con `prueba=\"TestLoQueSea\"`")
	}
	if d.De != "" && d.De == d.A {
		quejas = append(quejas, "`de` y `a` son iguales: el sabotaje no cambiaría nada y su verde diría "+
			"«no lo apliqué», no «la guarda no cubre»")
	}
	if len(quejas) > 0 {
		return d, "", quejas
	}
	return d, "", nil
}

// camposDe parte `clave="valor"` usando el SCANNER DE GO.
//
// No hay un parser de comillas escrito a mano acá a propósito: el texto a sabotear trae comillas,
// backticks y barras, y cada esquema de escapado casero es una superficie de bugs nueva. `go/scanner`
// es el mismo lexer que compila el archivo, así que los literales crudos con backtick, los escapes
// `\n` y las comillas anidadas salen bien por construcción.
func camposDe(payload string) (map[string]string, []string) {
	var quejas []string
	fset := token.NewFileSet()
	file := fset.AddFile("directiva", -1, len(payload))
	var s scanner.Scanner
	s.Init(file, []byte(payload), func(_ token.Position, msg string) {
		quejas = append(quejas, "la directiva no es legible: "+msg)
	}, 0)

	campos := map[string]string{}
	var clave string
	esperaIgual, esperaValor := false, false
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		switch {
		case tok == token.SEMICOLON:
			// El scanner inserta punto y coma en los saltos de línea. Una directiva multilínea es
			// normal, así que no significa nada acá.
			continue
		case esperaValor:
			if tok != token.STRING {
				quejas = append(quejas, fmt.Sprintf("`%s` tiene que traer un literal de string de Go (comillas dobles o backticks), no %q", clave, lit))
				esperaValor, clave = false, ""
				continue
			}
			v, err := strconv.Unquote(lit)
			if err != nil {
				quejas = append(quejas, fmt.Sprintf("`%s`: no pude leer el literal %s: %v", clave, lit, err))
			} else if _, repe := campos[clave]; repe {
				quejas = append(quejas, fmt.Sprintf("`%s` está dos veces: no se sabe cuál vale", clave))
			} else {
				campos[clave] = v
			}
			esperaValor, clave = false, ""
		case esperaIgual:
			if tok != token.ASSIGN {
				quejas = append(quejas, fmt.Sprintf("después de `%s` esperaba `=` y vino %q", clave, lit))
				esperaIgual, clave = false, ""
				continue
			}
			esperaIgual, esperaValor = false, true
		case tok == token.IDENT:
			clave, esperaIgual = lit, true
			if !claveConocida(clave) {
				quejas = append(quejas, fmt.Sprintf("clave desconocida `%s`: las que hay son %s", clave, strings.Join(clavesValidas, ", ")))
			}
		default:
			quejas = append(quejas, fmt.Sprintf("no esperaba %q en la directiva", lit))
		}
	}
	if esperaIgual || esperaValor {
		quejas = append(quejas, fmt.Sprintf("la directiva se corta después de `%s`", clave))
	}
	return campos, quejas
}

var clavesValidas = []string{"archivo", "de", "a", "paquete", "prueba", "arreglo_de", "arreglo_a", "no_mecanizable"}

func claveConocida(k string) bool {
	for _, v := range clavesValidas {
		if v == k {
			return true
		}
	}
	return false
}

// Validar comprueba, SIN CORRER NADA, que cada directiva siga apuntando a donde dice.
//
// Esto es lo que impide que el corpus se podra. Correr 739 sabotajes cuesta horas y no entra en
// CI; comprobar que sus anclas todavía existen cuesta milisegundos y entra. Una directiva cuyo
// `de` dejó de estar —porque el código se movió— es un sabotaje que NO SE APLICA, y su verde se
// lee igual que «la guarda cubre». Acá se vuelve rojo.
func Validar(c Censo) []string {
	var males []string
	for _, a := range c.Mecanizadas() {
		d := a.Directiva
		ruta := filepath.Join(c.Raiz, filepath.FromSlash(d.Archivo))
		b, err := os.ReadFile(ruta)
		if err != nil {
			males = append(males, fmt.Sprintf("%s:%d: `archivo=%q` no se puede leer: %v",
				a.Archivo, a.Linea, d.Archivo, err))
			continue
		}
		switch n := strings.Count(string(b), d.De); {
		case n == 0:
			males = append(males, fmt.Sprintf("%s:%d: el `de` de este sabotaje YA NO ESTÁ en %s. "+
				"El código se movió y el sabotaje no se aplicaría: su verde diría «no lo apliqué», "+
				"no «la guarda cubre». Buscá el texto nuevo y actualizá la directiva.",
				a.Archivo, a.Linea, d.Archivo))
		case n > 1:
			males = append(males, fmt.Sprintf("%s:%d: el `de` aparece %d veces en %s: no se sabría cuál "+
				"se tocó. Alargá el literal hasta que sea único.",
				a.Archivo, a.Linea, n, d.Archivo))
		}
		if d.ArregloDe != "" {
			if n := strings.Count(string(b), d.ArregloDe); n != 1 {
				males = append(males, fmt.Sprintf("%s:%d: el `arreglo_de` aparece %d veces en %s (tiene que ser 1)",
					a.Archivo, a.Linea, n, d.Archivo))
			}
		}
	}
	return males
}

// Aplicar hace el reemplazo en el lugar, exigiendo unicidad.
//
// Es la mitad que el corredor le pasa a `sabotaje.sh` como «comando que sabotea». El archivo se
// reescribe CONSERVANDO EL MODO: `cp` sobre un archivo que existe no lo toca, así que un
// reemplazo que crea el archivo de cero le deja el modo del umask — pasó, y el guion que quedó sin
// bit de ejecución murió con `Permission denied` en la corrida siguiente, que manda a mirar
// cualquier cosa menos acá.
func Aplicar(ruta, de, a string) error {
	info, err := os.Stat(ruta)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(ruta)
	if err != nil {
		return err
	}
	if n := strings.Count(string(b), de); n != 1 {
		return fmt.Errorf("el texto a reemplazar aparece %d veces en %s y tiene que aparecer exactamente 1: "+
			"con 0 el sabotaje NO SE APLICA (y su verde no dice nada), con más de 1 no se sabe cuál se tocó",
			n, ruta)
	}
	return os.WriteFile(ruta, []byte(strings.Replace(string(b), de, a, 1)), info.Mode().Perm())
}
