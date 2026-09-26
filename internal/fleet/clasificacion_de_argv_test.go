package fleet

// clasificacion_de_argv_test.go cierra los huecos que la auditoría A131 (tema T7) encontró en las
// guardas de la CLASIFICACIÓN de la cronología —qué tipo, qué plano y qué argv visible salen de una
// fila de `device_commands`— y el defecto VIVO que apareció midiéndolos.
//
// Las cuatro guardas viejas clavaban un eje en un valor cómodo, la misma forma que T8 encontró en
// la ventana, y el defecto vivía en otro valor de ese eje:
//
//   - TestUnaOperacionInternaDesconocidaNoSeLeMuestraANadie probaba siempre con DOS partes. Con
//     `len(argv) > 1` en EsOperacionInterna, una operación nueva sin argumentos —un
//     `musubi:actualizar` de mañana— se clasificaba como comando del host y la veía cualquiera
//     con `exec` (C1-m3).
//   - Las operaciones de hoy se probaban con su nombre EXACTO. Un TipoDeArgv que comparara por
//     prefijo mandaba `musubi:pantalla-algo`, que nadie clasificó, al plano de pantalla (C1-m4).
//   - TestElPlanoDeUnAvisoLoDecideQuienLoEncolo miraba la CAPACIDAD del hecho y nunca su PLANO.
//     Con el plano calculado desde el argv, los avisos del exec salían con `plano: ""` (C1-m6).
//   - TestElArgvDeBitacoraNuncaLlevaLaContrasena usaba siempre `OpPantalla` sin nada alrededor.
//     Sin el recorte en ArgvDeBitacora, la contraseña pasaba con un espacio adelante (C1-m8).
//
// Acá se recorren los ejes enteros —el LARGO del argv, el NOMBRE de la operación, la FORMA de la
// cabeza y la PUERTA por la que nace el hecho— y el conjunto de operaciones sale del bloque const
// del paquete, no de una lista escrita a mano.
//
// EL VIVO salió del eje de la forma: la guarda de musubi_fleet_exec y la validación de las
// políticas decidían sobre el argv CRUDO, y el canal guarda y ejecuta el que deja LimpiarArgv. Ver
// ejecutableDe y TestUnaPoliticaNoEncolaUnaOperacionInternaDisfrazada.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// opsInternasDeclaradas devuelve valor → nombre de cada constante del paquete cuyo valor es una
// operación interna del canal: un literal que empieza con PrefijoOperacionInterna y nombra algo.
//
// Se lee el FUENTE y no una lista porque la lista es lo que se olvida: el día que alguien agregue
// `OpActualizar` al bloque, esta función lo trae sola y las pruebas de abajo lo recorren.
func opsInternasDeclaradas(t *testing.T) map[string]string {
	t.Helper()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				for i, nombre := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						// Una operación armada con una expresión (`PrefijoOperacionInterna + "x"`) no
						// la ve este barrido, y callarlo sería el verde por vacío de siempre.
						if mencionaElPrefijo(vs.Values[i]) {
							t.Fatalf("%s: %s arma una operación interna con una expresión; ampliá el barrido",
								fset.Position(vs.Pos()), nombre.Name)
						}
						continue
					}
					valor, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(valor, PrefijoOperacionInterna) && valor != PrefijoOperacionInterna {
						out[valor] = nombre.Name
					}
				}
			}
		}
	}
	return out
}

// mencionaElPrefijo dice si una expresión nombra el prefijo de las operaciones internas, por la
// constante o por su texto.
func mencionaElPrefijo(e ast.Expr) bool {
	hay := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			hay = hay || x.Name == "PrefijoOperacionInterna"
		case *ast.BasicLit:
			hay = hay || strings.Contains(x.Value, PrefijoOperacionInterna)
		}
		return !hay
	})
	return hay
}

// clasificacionPorArgvDecidida es QUÉ REVELA cada operación interna mirando sólo su argv. Va
// escrita a mano porque es una decisión y no un derivado; lo que se deriva es QUÉ operaciones
// existen, y TestCadaOperacionInternaDeclaradaTieneSuClasificacionDecidida exige que las dos listas
// sean la misma.
func clasificacionPorArgvDecidida() map[string]TipoDeHecho {
	return map[string]TipoDeHecho{
		OpPantalla: HechoCanalPantalla,
		OpShell:    HechoCanalShell,
		// Las dos que se clasifican por FILA: los tres caminos las encolan con el mismo argv, así
		// que el argv solo no sabe a quién mostrarlas (ver OpsClasificadasPorFila).
		OpAvisar:    HechoSinClasificar,
		OpPreguntar: HechoSinClasificar,
	}
}

// parecidosA deriva, de una operación conocida, nombres que se le PARECEN y que nadie clasificó.
//
// LA CONTINUACIÓN SALE DE UN ALFABETO CERRADO, no de una lista: el nombre seguido de CADA byte
// ASCII (0x00 a 0x7F), solo y con más texto detrás. Así, una regla que compare por prefijo con el
// separador que sea —`OpPantalla + "-"`, `+ "."`, `+ "/"`, `+ ":"`— encuentra acá el nombre que
// le presta la clasificación a un ajeno. La única exclusión es la que deshace LimpiarArgv, el
// recorte de strings.TrimSpace sobre el ejecutable: el nombre seguido de un blanco, solo, queda en
// el nombre exacto y ES la operación conocida (con texto detrás el blanco queda adentro y sí es
// otro nombre).
//
// Y tres que no continúan el nombre: con la última letra cortada (una regla que compare al revés,
// «el conocido empieza con esto»), en mayúsculas (una que no distinga) y con el nombre contenido
// en otro (una que busque contención o sufijo).
//
// LO QUE ESTE CONJUNTO NO VE, y hasta la ronda anterior el comentario decía que sí: un separador
// de DOS caracteres o más (`OpPantalla + "::"`: ningún nombre de acá tiene dos dos-puntos
// seguidos), o uno fuera de ASCII. La lista era de ocho formas escritas a mano y la revisión de T7
// la dejó en verde con `OpPantalla + "."`. Agrandar el alfabeto a pares o a runas no converge
// —cualquier largo tiene su largo más uno—, y por eso el resto del eje no lo cierra un nombre sino
// la FORMA del código que decide: ver TestLaClasificacionPorNombreSoloComparaElNombreEntero.
func parecidosA(op string) []string {
	nombre := strings.TrimPrefix(op, PrefijoOperacionInterna)
	out := []string{
		op[:len(op)-1],
		PrefijoOperacionInterna + strings.ToUpper(nombre),
		PrefijoOperacionInterna + "x" + nombre,
	}
	// Los imprimibles primero, para que el primer nombre que cae se lea sin escapes; después, los
	// de control y el espacio. Son los mismos 128 en cualquier orden.
	var alfabeto []byte
	for c := byte(0x21); c <= 0x7e; c++ {
		alfabeto = append(alfabeto, c)
	}
	for c := byte(0x00); c <= 0x20; c++ {
		alfabeto = append(alfabeto, c)
	}
	alfabeto = append(alfabeto, 0x7f)
	for _, c := range alfabeto {
		sigue := string(rune(c))
		if strings.TrimSpace(op+sigue) != op {
			out = append(out, op+sigue)
		}
		out = append(out, op+sigue+"x")
	}
	return out
}

// operacionesDePrueba son las declaradas, con la clasificación decidida, más las desconocidas
// derivadas de ellas (sus parecidos y una inventada), que tienen que dar HechoSinClasificar.
func operacionesDePrueba(t *testing.T) map[string]TipoDeHecho {
	t.Helper()
	declaradas := opsInternasDeclaradas(t)
	decidida := clasificacionPorArgvDecidida()
	// La inventada, y el prefijo pelado: el nombre vacío es el extremo del eje del nombre.
	out := map[string]TipoDeHecho{"musubi:todavia-no-existe": HechoSinClasificar, PrefijoOperacionInterna: HechoSinClasificar}
	for op := range declaradas {
		for _, p := range parecidosA(op) {
			if _, esConocida := declaradas[p]; !esConocida {
				out[p] = HechoSinClasificar
			}
		}
	}
	for op := range declaradas {
		out[op] = decidida[op]
	}
	return out
}

// TODA OPERACIÓN INTERNA DECLARADA TIENE SU CLASIFICACIÓN DECIDIDA, Y LA DECISIÓN NO INVENTA NINGUNA.
//
// Las pruebas de abajo recorren las operaciones que el bloque const declara, y comparan contra
// clasificacionPorArgvDecidida. Esta prueba ata las dos cosas: una operación nueva sin decisión es
// roja acá, en vez de colarse con la clasificación que le toque por accidente. Es la misma forma
// que TestTiposDeHechoEsElEnumEntero para los tipos.
//
// Y ata la tercera lista, OpsClasificadasPorFila: lo que el argv no sabe clasificar tiene que
// clasificarse por fila, o no lo ve nadie; y lo que se clasifica por fila no puede tener además una
// clasificación por argv, porque ésa es la que filtra un plano al vecino (migración 46).
//
// Sabotaje: declarar una operación nueva sin decidir qué revela → las pruebas de clasificación la
// recorrerían comparándola contra nada.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tOpShell     = \"musubi:shell\"     // abrir una shell interactiva en Tier A\n)"
// arnes: a="\tOpShell     = \"musubi:shell\"     // abrir una shell interactiva en Tier A\n\tOpActualizar = \"musubi:actualizar\"\n)"
func TestCadaOperacionInternaDeclaradaTieneSuClasificacionDecidida(t *testing.T) {
	declaradas := opsInternasDeclaradas(t)
	// EL PISO: si el barrido no encontrara nada, las comparaciones de abajo no medirían nada.
	if len(declaradas) < 4 {
		t.Fatalf("el barrido encontró %d operaciones internas (%v); había 4 cuando se escribió esta "+
			"prueba: el barrido está roto", len(declaradas), declaradas)
	}
	decidida := clasificacionPorArgvDecidida()
	for op, nombre := range declaradas {
		if _, ok := decidida[op]; !ok {
			t.Errorf("%s (%q) está declarada y nadie decidió qué revela su argv: sumala a "+
				"clasificacionPorArgvDecidida con el tipo que le corresponde", nombre, op)
		}
	}
	for op := range decidida {
		if _, ok := declaradas[op]; !ok {
			t.Errorf("clasificacionPorArgvDecidida decide sobre %q y ninguna constante del paquete lo declara", op)
		}
	}
	for op, tipo := range decidida {
		porFila := OpsClasificadasPorFila[op]
		if tipo == HechoSinClasificar && !porFila {
			t.Errorf("%q no se clasifica por argv ni por fila: la cronología la escondería de todos", op)
		}
		if tipo != HechoSinClasificar && porFila {
			t.Errorf("%q se clasifica por fila Y por argv (%q): el argv es el mismo en los tres planos y "+
				"clasificarlo por ahí le muestra la fila a quien no puede verla", op, tipo)
		}
	}
	for op := range OpsClasificadasPorFila {
		if _, ok := declaradas[op]; !ok {
			t.Errorf("OpsClasificadasPorFila nombra %q y ninguna constante del paquete lo declara", op)
		}
	}
}

// UNA OPERACIÓN INTERNA SE CLASIFICA IGUAL CON CUALQUIER LARGO DE ARGV, Y NUNCA COMO UN COMANDO.
//
// La guarda vieja (TestUnaOperacionInternaDesconocidaNoSeLeMuestraANadie) clavaba el largo en DOS,
// y TestTodaOperacionInternaDelCodigoEstaClasificada, que usa una sola parte, sólo pedía «algo
// distinto de sin_clasificar» — y `comando` lo es. Con `len(argv) > 1` en EsOperacionInterna, una
// operación sin argumentos caía como comando del host: visible para todo el que pueda ejecutar,
// revelando su plano antes de que nadie decidiera quién la ve. Fleet y mcp quedaban en verde.
//
// Acá se recorre el eje entero, de 1 a ArgvMaxPartes, que es todo lo que ValidarComando deja
// encolar, con cada operación declarada y cada desconocida derivada de ellas.
//
// EXPOSICIÓN medida por la auditoría: cero hoy. De 12.821 filas de `device_commands`, ninguna es
// una operación interna de una sola parte ni una `musubi:*` desconocida. Es latente: aparece el día
// que se agregue una operación sin argumentos, y ése es el día en que nada se ponía rojo.
//
// Sabotaje: exigir dos partes para reconocer una operación interna (C1-m3) → las de una sola parte
// se leen como comandos del host.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn strings.HasPrefix(ejecutableDe(argv), PrefijoOperacionInterna)"
// arnes: a="\treturn len(argv) > 1 && strings.HasPrefix(ejecutableDe(argv), PrefijoOperacionInterna)"
func TestUnaOperacionInternaSeClasificaIgualConCualquierLargo(t *testing.T) {
	casos := operacionesDePrueba(t)
	medidos := 0
	for op, quiero := range casos {
		// resultado → largos con los que salió, para decir el fallo en una línea y no en 64.
		malos := map[string][]int{}
		for largo := 1; largo <= ArgvMaxPartes; largo++ {
			argv := []string{op}
			for len(argv) < largo {
				argv = append(argv, "arg"+strconv.Itoa(len(argv)))
			}
			medidos++
			if !EsOperacionInterna(argv) {
				malos["no se reconoció como interna"] = append(malos["no se reconoció como interna"], largo)
				continue
			}
			if got := TipoDeArgv(argv); got != quiero {
				malos[string(got)] = append(malos[string(got)], largo)
			}
		}
		for resultado, largos := range malos {
			t.Errorf("%q tiene que clasificarse %q con cualquier largo, y con %d de %d largos (del %d al %d) dio "+
				"«%s» (`comando` = la ve cualquiera con `exec`)", op, quiero, len(largos), ArgvMaxPartes,
				largos[0], largos[len(largos)-1], resultado)
		}
	}
	// EL PISO: las 4 declaradas, sus parecidos y la inventada, cada una con los 64 largos.
	if minimo := 4 * ArgvMaxPartes; medidos < minimo {
		t.Fatalf("se midieron %d argv y el eje entero de 4 operaciones son %d: el recorrido no recorrió", medidos, minimo)
	}
}

// UN NOMBRE PARECIDO A UNA OPERACIÓN CONOCIDA ES DESCONOCIDO, Y NO SE LE MUESTRA A NADIE.
//
// Las guardas de las operaciones de hoy las probaban con su nombre EXACTO, así que no distinguían
// un TipoDeArgv que compara la igualdad de uno que compara el prefijo. Con prefijo,
// `musubi:pantalla-control` y `musubi:shell-root` —que nadie clasificó— caían en canal_pantalla y
// canal_shell y se mostraban; fleet quedaba verde y en mcp sólo caía el censo del arnés. El agente
// despacha por igualdad exacta, así que la cronología mostraría como pantalla algo que la máquina
// rechaza como «operación interna desconocida».
//
// Los parecidos se DERIVAN de cada operación declarada (parecidosA), así que una operación nueva
// trae los suyos.
//
// LO QUE CAZA ES UNA CLASE CERRADA, NO «CUALQUIER REGLA». La primera versión de esta prueba
// recorría ocho formas escritas a mano, y el comentario de parecidosA prometía que con ellas caía
// toda regla que no comparara la igualdad entera. La revisión de T7 lo midió falso: un
// `HasPrefix(…, OpPantalla+".")` antes del switch —prefijo con OTRO separador, la misma clase que
// C1-m4— dejaba fleet y mcp en verde, y `musubi:pantalla.algo` se habría mostrado como pantalla.
// Hoy la continuación sale del alfabeto ASCII entero (ver parecidosA), así que cae cualquier
// prefijo seguido de UN carácter ASCII, sea cual sea. Lo que no cae —un separador de dos
// caracteres, o uno fuera de ASCII— no lo puede cerrar ningún conjunto de nombres, y lo cierra la
// guarda de la forma: TestLaClasificacionPorNombreSoloComparaElNombreEntero.
//
// Y CADA PARECIDO PASA TAMBIÉN POR LA PUERTA DE LA CRONOLOGÍA. Hasta la segunda revisión de T7 se
// le preguntaban sólo a TipoDeArgv, y la cronología no arma el hecho con TipoDeArgv sino con
// HechoDeComando (vía TipoDeComando). La revisión puso una regla en esa puerta —`tipo =
// HechoCanalPantalla` para lo que empiece con `musubi:pantalla::`— y el paquete entero quedó en
// verde: ninguna guarda del nombre cruzaba la puerta. Ahora cada parecido nace también como hecho.
// La regla con `::` la caza la guarda de la forma (que mira el paquete entero); ésta caza la de un
// carácter, que es la directiva de abajo.
//
// EXPOSICIÓN medida por la auditoría: cero. Ninguna fila empieza con el nombre de otra operación
// sin serlo, y ninguna de las cuatro declaradas es prefijo de otra. Latente.
//
// Sabotaje: clasificar como pantalla todo lo que EMPIECE con `musubi:pantalla` (la forma de C1-m4)
// → los parecidos heredan el plano de pantalla.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tswitch ejecutableDe(argv) {"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(argv), OpPantalla) {\n\t\treturn HechoCanalPantalla\n\t}\n\tswitch ejecutableDe(argv) {"
// Sabotaje: clasificar como pantalla lo que empiece con `musubi:pantalla.` —el prefijo con otro
// separador que la lista de ocho formas dejaba en verde— → `musubi:pantalla.x` hereda el plano de
// pantalla.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tswitch ejecutableDe(argv) {"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(argv), OpPantalla+\".\") {\n\t\treturn HechoCanalPantalla\n\t}\n\tswitch ejecutableDe(argv) {"
// Sabotaje: la misma regla del `.`, pero en HechoDeComando —la puerta de la cronología— y no en
// TipoDeArgv → el hecho nace como pantalla aunque TipoDeArgv lo esconda.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn Hecho{\n\t\tCuando:     c.Creado,"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(c.Argv), OpPantalla+\".\") {\n\t\ttipo = HechoCanalPantalla\n\t}\n\treturn Hecho{\n\t\tCuando:     c.Creado,"
func TestUnNombreParecidoAUnaOperacionConocidaEsDesconocido(t *testing.T) {
	declaradas := opsInternasDeclaradas(t)
	medidos := 0
	for op := range declaradas {
		for _, parecido := range parecidosA(op) {
			if _, esConocida := declaradas[parecido]; esConocida {
				continue
			}
			medidos++
			tipo := TipoDeArgv([]string{parecido, "x"})
			if tipo != HechoSinClasificar {
				t.Errorf("%q se parece a %s y se clasificó %q: heredó la clasificación de un nombre que "+
					"nadie decidió, y el agente lo rechaza como desconocido", parecido, op, tipo)
			}
			if _, mostrable := CapDeHecho(tipo); mostrable {
				t.Errorf("%q, desconocido, resultó mostrable: el default tiene que ser NO mostrar", parecido)
			}
			// La puerta de la cronología: una fila sin clasificación propia, como las anteriores a la
			// migración 46, cae al argv por TipoDeComando.
			if h := HechoDeComando(Comando{Argv: []string{parecido, "x"}}, "pc"); h.Tipo != HechoSinClasificar {
				t.Errorf("%q se parece a %s y HechoDeComando —la puerta por la que nace el hecho de la "+
					"cronología— lo hizo nacer %q: la regla vive en la puerta y no en TipoDeArgv", parecido, op, h.Tipo)
			}
		}
	}
	if minimo := 4 * len(parecidosA(OpPantalla)); medidos < minimo {
		t.Fatalf("se midieron %d parecidos y con 4 operaciones son al menos %d: el recorrido no recorrió", medidos, minimo)
	}
	// EL PISO DEL ALFABETO. El de arriba se deriva de parecidosA, así que no vería una parecidosA
	// que perdiera la mitad de los bytes: exigiría menos y pasaría igual. Acá se pregunta por el
	// alfabeto mismo, que es un hecho: los 128 bytes ASCII, cada uno seguido de texto.
	enLaLista := map[string]bool{}
	for _, p := range parecidosA(OpPantalla) {
		enLaLista[p] = true
	}
	for c := 0; c < 0x80; c++ {
		if n := OpPantalla + string(rune(c)) + "x"; !enLaLista[n] {
			t.Errorf("parecidosA no continúa el nombre con el byte %#02x (%q): una regla por prefijo con "+
				"ese separador clasificaría nombres que nadie decidió y esta prueba no la vería", c, n)
		}
	}
}

// cabezasDe es `cabeza` tal cual y con cada blanco que LimpiarArgv le recorta al ejecutable
// adelante, detrás y a los dos lados: los de strings.TrimSpace, TODOS, que salen de
// blancosDeTrimSpace y no de una lista. Hasta la segunda revisión de T7 eran cinco de los 25.
func cabezasDe(cabeza string) []string {
	out := []string{cabeza}
	for _, b := range blancosDeTrimSpace() {
		out = append(out, b+cabeza, cabeza+b, b+cabeza+b)
	}
	return out
}

// formasDeLaCabeza arma un argv con cada una de cabezasDe(cabeza) adelante, y con partes antes: una
// vacía o hecha de un solo blanco —cada uno de blancosDeTrimSpace—, que LimpiarArgv saca, y dos
// vacías, que dejan el argv sin ejecutable. Son las transformaciones de LimpiarArgv y ninguna otra:
// lo que sale de acá es lo que un llamador puede mandar y el canal guardar distinto.
func formasDeLaCabeza(cabeza string, cola ...string) [][]string {
	adelante := [][]string{nil, {""}, {"", ""}}
	for _, b := range blancosDeTrimSpace() {
		adelante = append(adelante, []string{b})
	}
	var out [][]string
	for _, pre := range adelante {
		for _, c := range cabezasDe(cabeza) {
			argv := append(append(append([]string{}, pre...), c), cola...)
			out = append(out, argv)
		}
	}
	return out
}

// LA CABEZA DEL ARGV SE LEE COMO LA DESPACHA EL AGENTE, EN LAS TRES DECISIONES QUE SE TOMAN SOBRE ELLA.
//
// El canal limpia el argv de los dos lados —EncolarComando antes de guardar, el agente antes de
// despachar— y el agente despacha pantalla con `LimpiarArgv(argv)[0] == OpPantalla`
// (cmd/musubi/ejecutor.go). Todo lo que el cerebro decide sobre la cabeza tiene que decidirlo sobre
// ESA forma, o decide sobre un argv que nunca va a correr. Tres cosas, para cada forma:
//
//   - LimpiarArgv es IDEMPOTENTE: limpiar lo guardado da lo guardado. No lo era: con una parte
//     vacía adelante, el ejecutable salía sin recortar, se guardaba `" musubi:pantalla"` y el
//     agente ejecutaba `musubi:pantalla`.
//   - EsOperacionInterna y TipoDeArgv dan lo mismo sobre el argv crudo que sobre el limpio.
//   - ArgvDeBitacora tapa la contraseña de TODA forma que el agente despacharía como pantalla, y
//     conserva el id que el agente lee. La guarda vieja probaba sólo `OpPantalla` pelado, y sin el
//     recorte la contraseña pasaba con un espacio adelante (C1-m8).
//
// EXPOSICIÓN medida por la auditoría: cero filas con blancos alrededor de argv[0] entre 12.821; las
// cuatro `musubi:pantalla` guardadas son exactas y ya están tapadas. Pero el mundo que rompe la
// guarda sí existía en el código: una credencial con `exec` sin allowlist fabricaba esa forma por
// musubi_fleet_exec (ver TestConExecNoSeEncolaUnaOperacionInternaDisfrazada, en mcp).
//
// Sabotaje: que ArgvDeBitacora decida sobre argv[0] crudo (la forma de C1-m8) → la contraseña pasa
// con blancos alrededor o una parte vacía adelante.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif ejecutableDe(argv) != OpPantalla {"
// arnes: a="\tif len(argv) == 0 || argv[0] != OpPantalla {"
// Sabotaje: que LimpiarArgv deje de recortar el ejecutable que QUEDA primero → deja de ser
// idempotente y lo guardado no es lo que el agente ejecuta.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\tif len(out) > 0 {\n\t\tout[0] = strings.TrimSpace(out[0])\n\t}\n"
// arnes: a=""
// Sabotaje: una SEGUNDA normalización en ArgvDeBitacora, con un juego de blancos propio —el que
// midió la segunda revisión de T7: espacio, tab, CR, LF y el espacio duro, los cinco que esta prueba
// recorría— → con `\v`, `\f`, U+0085, U+2028… alrededor el agente despacha pantalla y la contraseña
// pasa. Con los cinco blancos de antes el paquete entero quedaba en verde.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="func ArgvDeBitacora(argv []string) []string {\n"
// arnes: a="func ArgvDeBitacora(argv []string) []string {\n\tfor _, a := range argv {\n\t\tif c := strings.Trim(a, \" \\t\\r\\n\\u00a0\"); c != \"\" {\n\t\t\tif c != OpPantalla {\n\t\t\t\treturn argv\n\t\t\t}\n\t\t\tbreak\n\t\t}\n\t}\n"
func TestLaCabezaDelArgvSeLeeComoLaDespachaElAgente(t *testing.T) {
	const secreto = "ContraseñaDeSesión123"
	// EL PISO DE LOS BLANCOS: los 25 de unicode.IsSpace, que es lo que recorta TrimSpace. Es un hecho
	// de Unicode y se clava: si blancosDeTrimSpace perdiera la mitad, las formas de abajo mirarían
	// menos y la cuenta de pantallas —que se deriva de ellas— bajaría con ellas sin decir nada.
	if n := len(blancosDeTrimSpace()); n < 25 {
		t.Fatalf("blancosDeTrimSpace encontró %d blancos y unicode.IsSpace tiene 25: las formas de la cabeza "+
			"no recorren todo lo que LimpiarArgv recorta", n)
	}
	cabezas := []string{"systemctl", "musubi:todavia-no-existe"}
	for op := range opsInternasDeclaradas(t) {
		cabezas = append(cabezas, op)
	}
	sort.Strings(cabezas)

	pantallas := 0
	for _, cabeza := range cabezas {
		for _, argv := range formasDeLaCabeza(cabeza, "ses-42", secreto, "30m0s") {
			limpio := LimpiarArgv(argv)
			if otra := LimpiarArgv(limpio); !slices.Equal(otra, limpio) {
				t.Errorf("LimpiarArgv no es idempotente con %q: el cerebro guarda %q y el agente, al "+
					"limpiar de su lado, ejecuta %q", argv, limpio, otra)
			}
			if a, b := EsOperacionInterna(argv), EsOperacionInterna(limpio); a != b {
				t.Errorf("EsOperacionInterna(%q)=%v y sobre lo que se guarda (%q) da %v: la guarda del exec "+
					"decidiría sobre un argv que no es el que corre", argv, a, limpio, b)
			}
			if a, b := TipoDeArgv(argv), TipoDeArgv(limpio); a != b {
				t.Errorf("TipoDeArgv(%q)=%q y sobre lo que se guarda (%q) da %q", argv, a, limpio, b)
			}
			if len(limpio) == 0 || limpio[0] != OpPantalla {
				continue
			}
			pantallas++
			visible := ArgvDeBitacora(argv)
			if strings.Contains(strings.Join(visible, " "), secreto) {
				t.Errorf("FUGA: con argv %q el agente abre una pantalla y ArgvDeBitacora dejó pasar la "+
					"contraseña: %q", argv, visible)
			}
			if len(visible) < 2 || visible[1] != limpio[1] {
				t.Errorf("con argv %q se perdió el id de sesión que el agente lee (%q): quedó %q", argv, limpio[1], visible)
			}
		}
	}
	// EL PISO: aunque sea sin partes adelante, cada cabeza de pantalla se despacha como pantalla.
	if minimo := len(cabezasDe(OpPantalla)); pantallas < minimo {
		t.Fatalf("sólo %d formas se despachan como pantalla y al menos %d tienen que hacerlo: la prueba no "+
			"miró la contraseña", pantallas, minimo)
	}

	// CONTROL: un comando del host no se toca, venga como venga. Sin esto, un ArgvDeBitacora que
	// tapara TODO pasaría la mitad de arriba.
	for _, argv := range formasDeLaCabeza("journalctl", "-u", "nginx") {
		if got := ArgvDeBitacora(argv); !slices.Equal(got, argv) {
			t.Errorf("ArgvDeBitacora tocó un comando del host: %q quedó %q", argv, got)
		}
	}
}

// clasificacionDeHecho es quién puede ver un hecho y cómo se lee.
type clasificacionDeHecho struct {
	cap       Cap
	mostrable bool
	plano     PlanoDeFlota
}

// clasificacionDeHechoDecidida es la clasificación de cada tipo de hecho. Es la DECISIÓN, escrita a
// mano; TestElHechoLlevaElPlanoYLaCapacidadDeSuTipoPorLasTresPuertas exige que cubra TiposDeHecho
// entero, y TestTiposDeHechoEsElEnumEntero ata esa lista al bloque const.
func clasificacionDeHechoDecidida() map[TipoDeHecho]clasificacionDeHecho {
	return map[TipoDeHecho]clasificacionDeHecho{
		HechoComando:       {CapExec, true, PlanoActuar},
		HechoCanalExec:     {CapExec, true, PlanoActuar},
		HechoPantalla:      {CapScreenView, true, PlanoEntrar},
		HechoCanalPantalla: {CapScreenView, true, PlanoEntrar},
		HechoShell:         {CapShell, true, PlanoEntrar},
		HechoCanalShell:    {CapShell, true, PlanoEntrar},
		HechoSinClasificar: {"", false, ""},
	}
}

// EL HECHO LLEVA EL PLANO Y LA CAPACIDAD DE SU TIPO, SALGA DE LA PUERTA QUE SALGA.
//
// La guarda vieja (TestElPlanoDeUnAvisoLoDecideQuienLoEncolo) miraba del Hecho sólo la CAPACIDAD.
// Con el plano de HechoDeComando calculado desde el argv —la puerta vieja, la que no distingue quién
// encoló un aviso— el tipo y la capacidad quedaban bien y el plano no: todo `musubi:avisar` y
// `musubi:preguntar` llegaba con `plano: ""` en vez del que declaró quien lo encoló, y fleet, memory
// y mcp quedaban verdes (C1-m6). Desde este tema el plano es un método derivado del tipo; esta
// prueba es la que dice que la derivación es la decidida, por las tres puertas y con cada
// clasificación que una fila puede traer.
//
// EXPOSICIÓN medida por la auditoría: el estado existía en producción. 22 `musubi:avisar`, todos del
// exec (`canal_exec`), el último del 2026-09-21; con la mutación, los 22 habrían llegado a quien
// tiene `exec` con `plano: ""` en vez de `actuar`.
//
// Sabotaje: que el plano de un hecho de comando salga del argv y no de su tipo (la forma de C1-m6)
// → los avisos pierden el plano que declaró quien los encoló.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tplano, _ := PlanoDeHecho(h.Tipo)\n\treturn plano\n}"
// arnes: a="\ttipo := h.Tipo\n\tif tipo != HechoPantalla && tipo != HechoShell {\n\t\ttipo = TipoDeArgv(h.Argv)\n\t}\n\tplano, _ := PlanoDeHecho(tipo)\n\treturn plano\n}"
func TestElHechoLlevaElPlanoYLaCapacidadDeSuTipoPorLasTresPuertas(t *testing.T) {
	decidida := clasificacionDeHechoDecidida()
	for _, tipo := range TiposDeHecho {
		if _, ok := decidida[tipo]; !ok {
			t.Fatalf("el tipo %q no tiene capacidad ni plano decididos en esta prueba: sumalo", tipo)
		}
	}
	if len(decidida) != len(TiposDeHecho) {
		t.Fatalf("la decisión cubre %d tipos y TiposDeHecho tiene %d: sobra alguno", len(decidida), len(TiposDeHecho))
	}

	revisados := map[TipoDeHecho]int{}
	revisar := func(puerta string, h Hecho, tipo TipoDeHecho) {
		t.Helper()
		if h.Tipo != tipo {
			t.Errorf("%s: el hecho salió de tipo %q y la clasificación de su fuente es %q", puerta, h.Tipo, tipo)
			return
		}
		quiero := decidida[h.Tipo]
		if got := h.Plano(); got != quiero.plano {
			t.Errorf("%s: un hecho %q llegó con plano %q, y es %q: el tipo decide quién lo ve y el plano "+
				"cómo se lee, y acá dicen dos cosas distintas", puerta, h.Tipo, got, quiero.plano)
		}
		if c, m := CapDeHecho(h.Tipo); c != quiero.cap || m != quiero.mostrable {
			t.Errorf("%s: un hecho %q pide %q (mostrable=%v), y es %q (mostrable=%v)", puerta, h.Tipo, c, m, quiero.cap, quiero.mostrable)
		}
		revisados[h.Tipo]++
	}

	// HechoDeComando: cada operación declarada, un comando del host y una desconocida, con cada
	// clasificación que una fila puede traer — todos los tipos, ninguna y una de una versión futura.
	var ops []string
	for op := range opsInternasDeclaradas(t) {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	var argvs [][]string
	for _, op := range ops {
		argvs = append(argvs, []string{op, "Musubi: fulano está haciendo algo acá."})
	}
	argvs = append(argvs, []string{"systemctl", "restart", "nginx"}, []string{"musubi:todavia-no-existe", "x"})
	clasificaciones := append([]TipoDeHecho{"", "plano_del_futuro"}, TiposDeHecho...)
	for _, argv := range argvs {
		for _, cl := range clasificaciones {
			c := Comando{Argv: argv, Clasificacion: cl, Creado: time.Now()}
			revisar("HechoDeComando("+argv[0]+", "+string(cl)+")", HechoDeComando(c, "pc"), TipoDeComando(c))
		}
	}
	revisar("HechoDeSesionPantalla", HechoDeSesionPantalla(SesionPantalla{ID: "s"}, "pc"), HechoPantalla)
	revisar("HechoDeSesionShell", HechoDeSesionShell(SesionShell{ID: "s"}, "pc"), HechoShell)

	// EL PISO: cada tipo del enum pasó por alguna puerta. Si una puerta dejara de producir uno, la
	// prueba estaría midiendo menos de lo que dice.
	for _, tipo := range TiposDeHecho {
		if revisados[tipo] == 0 {
			t.Errorf("ninguna puerta produjo un hecho %q: la prueba no midió su plano", tipo)
		}
	}
}

// UNA POLÍTICA NO ENCOLA UNA OPERACIÓN INTERNA DISFRAZADA.
//
// DEFECTO VIVO en el árbol sano (5017a45). Validar comparaba `Hacer[0]` crudo contra el prefijo, y
// ValidarComando y EncolarComando limpian el argv: `hacer: ["", "musubi:pantalla", …]` pasaba la
// validación —la cabeza cruda es vacía— y cada disparo encolaba un `musubi:pantalla` perfecto. Es
// la puerta lateral que TestUnaPoliticaNoPuedeFabricarMensajesInternosDelCanal cierra para la forma
// pelada: quien edite la config del cerebro se acuñaría sesiones de pantalla sin tener nunca
// `screen`. Esa prueba clavaba el eje de la forma en `["musubi:pantalla", …]` exacto.
//
// EXPOSICIÓN: la config de políticas del cerebro no se leyó (este tema no toca máquinas). Lo que
// sí midió la auditoría es la tabla: de sus 12.821 filas, las 7 de origen `politica` son comandos
// del host, así que ninguna política encoló nunca una operación interna por este camino.
//
// EL LARGO TAMBIÉN ES UN EJE, y la primera versión de esta prueba lo clavaba: todas sus formas
// llevaban cola —cuatro o cinco partes—, así que un `len(p.Hacer) > 1 &&` delante de la guarda la
// dejaba en verde (lo midió la revisión de T7) y un `hacer: ["musubi:avisar"]` pelado pasaba la
// validación. La guarda vieja, TestUnaPoliticaNoPuedeFabricarMensajesInternosDelCanal, usa dos
// partes: tampoco lo ve. Por eso cada forma va también SIN cola, que es el otro extremo del eje, y
// el piso exige que se hayan medido los dos.
//
// Sabotaje: que Validar vuelva a mirar sólo la primera parte cruda → pasa la forma con partes
// vacías adelante.
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\tif EsOperacionInterna(p.Hacer) {"
// arnes: a="\tif EsOperacionInterna(p.Hacer[:1]) {"
// arnes: colision_ok="TestUnaPoliticaNoEncolaUnaOperacionInternaDisfrazada"
// Sabotaje: que Validar exija dos partes para reconocer una operación interna (el eje del largo,
// la forma de C1-m3 en este consumidor) → pasa la operación interna sin argumentos.
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\tif EsOperacionInterna(p.Hacer) {"
// arnes: a="\tif len(p.Hacer) > 1 && EsOperacionInterna(p.Hacer) {"
// arnes: colision_ok="TestUnaPoliticaNoEncolaUnaOperacionInternaDisfrazada"
func TestUnaPoliticaNoEncolaUnaOperacionInternaDisfrazada(t *testing.T) {
	con := func(hacer []string) Politica {
		return Politica{Nombre: "n", Principal: "auto", Cuando: CondMemPct, Supera: 90,
			Sobre: []string{"*"}, Hacer: hacer, Cooldown: time.Hour}
	}
	// Cada forma de la cabeza con cola y sin cola: los dos extremos del largo que una política
	// puede traer.
	formas := func(cabeza string, cola ...string) [][]string {
		return append(formasDeLaCabeza(cabeza, cola...), formasDeLaCabeza(cabeza)...)
	}
	internas := []string{"musubi:todavia-no-existe"}
	for op := range opsInternasDeclaradas(t) {
		internas = append(internas, op)
	}
	sort.Strings(internas)
	porLargo := map[int]int{} // partes del argv que se guardaría → formas medidas
	for _, op := range internas {
		for _, hacer := range formas(op, "ses-7", "clave", "30m") {
			if len(LimpiarArgv(hacer)) == 0 {
				continue // sin ejecutable no es un comando: lo rechaza ValidarComando, no esta guarda
			}
			porLargo[len(LimpiarArgv(hacer))]++
			if err := con(hacer).Validar(); err == nil {
				t.Errorf("una política con `hacer: %q` pasó la validación y cada disparo encolaría %q, "+
					"una operación interna del canal: la config fabricando mensajes de la pantalla, la "+
					"shell o los avisos, que es la puerta lateral que S6 cerró", hacer, LimpiarArgv(hacer))
			}
		}
	}
	// EL PISO: los dos extremos del largo, para cada operación. Sin el de una parte, esta prueba
	// vuelve a ser la que dejaba pasar `len(p.Hacer) > 1`.
	if porLargo[1] < len(internas) || porLargo[4] < len(internas) {
		t.Fatalf("formas medidas por largo: %v, para %d operaciones: faltan las de una parte o las de "+
			"cuatro, y la prueba no recorrió el eje del largo", porLargo, len(internas))
	}
	// CONTROL: las mismas formas con un comando del host SÍ validan, con cola y sin ella. Sin esto,
	// un Validar que lo rechazara todo —o todo lo de una parte— pasaría la mitad de arriba.
	for _, hacer := range formas("systemctl", "restart", "nginx") {
		if len(LimpiarArgv(hacer)) == 0 {
			continue
		}
		if err := con(hacer).Validar(); err != nil {
			t.Errorf("una política con `hacer: %q`, un comando del host, no validó: %v", hacer, err)
		}
	}
}
