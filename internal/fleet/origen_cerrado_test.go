package fleet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// origenesDecididos es la DECISIÓN sobre cada origen del enum: si es automático. Escrita a mano a
// propósito, y cerrada contra el fuente: TestElOrigenEsUnaListaBlancaCerradaContraSuEnum exige que
// tenga cada constante OrigenComando que el paquete declara, y ninguna más.
func origenesDecididos() map[OrigenComando]bool {
	return map[OrigenComando]bool{
		OrigenPersona:     false,
		OrigenPolitica:    true,
		OrigenDesconocido: false,
	}
}

// origenesFueraDelEnum deriva del enum los valores que se le PARECEN sin serlo —mayúsculas, un
// blanco de más, una letra de más— y suma dos que no se parecen a nada. Son los que un llamador
// nuevo, una fila escrita a mano o una versión futura podrían traer.
func origenesFueraDelEnum(t *testing.T, enum map[OrigenComando]bool) []OrigenComando {
	t.Helper()
	vistos := map[OrigenComando]bool{}
	var out []OrigenComando
	sumar := func(o OrigenComando) {
		if enum[o] || vistos[o] {
			return
		}
		vistos[o] = true
		out = append(out, o)
	}
	for o := range enum {
		s := string(o)
		sumar(OrigenComando(strings.ToUpper(s)))
		sumar(OrigenComando(s + " "))
		sumar(OrigenComando(" " + s))
		sumar(OrigenComando(s + "x"))
	}
	sumar("cron")
	sumar("robot")
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// EL ORIGEN ES UNA LISTA BLANCA, Y LA LISTA ESTÁ CERRADA CONTRA EL ENUM QUE LA DEFINE.
//
// La guarda vieja (TestUnOrigenDesconocidoNoEsPersonaNiAutomatico) recorría SÓLO los tres valores
// del enum, y sobre un dominio de tres una lista negra de dos es extensionalmente igual a la blanca.
// La auditoría A131 (C3-m7) cambió EsAutomatico por `o != OrigenPersona && o != OrigenDesconocido`:
// verde en fleet, memory y mcp, y `"cron"` pasaba a ser automático. Acá entran también los valores
// de AFUERA del enum —derivados de él—, y el enum mismo sale del bloque const: un origen nuevo
// declarado sin decidir si es automático pone esto en rojo antes de llegar a una superficie.
//
// EXPOSICIÓN medida por la auditoría: cero. De las 12.821 filas de device_commands, ninguna tiene un
// origen fuera del enum, y las dos puertas del store normalizan con OrigenValido antes de que algo
// llegue a EsAutomatico. El hueco era de la guarda; lo que cuida esto es el próximo llamador.
//
// Y EL ENUM QUE RECORREN LOS OTROS PAQUETES TAMBIÉN SE CIERRA ACÁ. Las pruebas de memory (las puertas
// de escritura y de lectura) y de mcp (las dos superficies) tenían cada una su lista escrita a mano;
// la revisión de T9 lo señaló: un cuarto origen se decidía acá y ninguna de las dos lo recorría.
// Ahora recorren fleet.OrigenesDeComando, y esta prueba exige que sea el bloque const entero.
//
// Sabotaje: EsAutomatico como lista NEGRA de dos (C3-m7) → un origen raro se cuenta como automático.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="func (o OrigenComando) EsAutomatico() bool { return o == OrigenPolitica }"
// arnes: a="func (o OrigenComando) EsAutomatico() bool { return o != OrigenPersona && o != OrigenDesconocido }"
// arnes: colision_ok="TestUnOrigenDesconocidoNoEsPersonaNiAutomatico"
// Sabotaje: declarar un origen nuevo sin decidirlo → OrigenValido lo guardaría como desconocido y
// nadie decidió si es automático.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\tOrigenDesconocido OrigenComando = \"\"\n)"
// arnes: a="\tOrigenDesconocido OrigenComando = \"\"\n\tOrigenSistema OrigenComando = \"sistema\"\n)"
// Sabotaje: que la lista exportada olvide un origen → memory y mcp dejan de recorrerlo sin enterarse.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="var OrigenesDeComando = []OrigenComando{OrigenPersona, OrigenPolitica, OrigenDesconocido}"
// arnes: a="var OrigenesDeComando = []OrigenComando{OrigenPersona, OrigenDesconocido}"
func TestElOrigenEsUnaListaBlancaCerradaContraSuEnum(t *testing.T) {
	decididos := origenesDecididos()
	declarados := constantesDeclaradas(t, "OrigenComando")
	// EL PISO: si el barrido no encontrara nada, la comparación de abajo no mediría nada.
	if len(declarados) < 3 {
		t.Fatalf("el barrido encontró %d constantes OrigenComando (%v); el enum tenía 3 cuando se "+
			"escribió esta prueba: el barrido está roto", len(declarados), declarados)
	}
	for valor, nombre := range declarados {
		if _, ok := decididos[OrigenComando(valor)]; !ok {
			t.Errorf("%s (%q) está declarado y no tiene decisión: nadie dijo si OrigenValido lo conserva "+
				"ni si es automático, y la cronología lo dibujaría según lo que salga por defecto", nombre, valor)
		}
	}
	for o := range decididos {
		if _, ok := declarados[string(o)]; !ok {
			t.Errorf("la decisión tiene %q y ninguna constante del paquete lo declara", o)
		}
	}
	// La lista exportada, la que recorren memory y mcp, contra el mismo bloque const.
	enLista := map[string]bool{}
	for _, o := range OrigenesDeComando {
		if enLista[string(o)] {
			t.Errorf("OrigenesDeComando repite %q", o)
		}
		enLista[string(o)] = true
	}
	for valor, nombre := range declarados {
		if !enLista[valor] {
			t.Errorf("%s (%q) está declarado y falta en OrigenesDeComando: las pruebas de memory (cómo se "+
				"guarda y cómo se lee) y de mcp (cómo se dibuja) no lo recorren", nombre, valor)
		}
	}
	for valor := range enLista {
		if _, ok := declarados[valor]; !ok {
			t.Errorf("OrigenesDeComando tiene %q y ninguna constante del paquete lo declara", valor)
		}
	}

	for o, automatico := range decididos {
		if got := OrigenValido(o); got != o {
			t.Errorf("OrigenValido(%q) = %q: un origen del enum se guardaría como otro", o, got)
		}
		if got := o.EsAutomatico(); got != automatico {
			t.Errorf("%q.EsAutomatico() = %v y la decisión es %v", o, got, automatico)
		}
	}

	raros := origenesFueraDelEnum(t, decididos)
	// EL PISO de los de afuera: cuatro variantes por cada valor del enum, más dos sin parecido.
	if len(raros) < 10 {
		t.Fatalf("sólo %d orígenes fuera del enum (%q): la derivación se rompió", len(raros), raros)
	}
	for _, r := range raros {
		if got := OrigenValido(r); got != OrigenDesconocido {
			t.Errorf("OrigenValido(%q) = %q: un origen que no está en el enum se guardaría como una "+
				"categoría que ninguna superficie sabe dibujar", r, got)
		}
		if r.EsAutomatico() {
			t.Errorf("%q se cuenta como automático y no está en el enum: una lista blanca no le da el bit "+
				"a lo que no conoce, y la cronología le atribuiría a una regla lo que nadie sabe quién hizo", r)
		}
	}
}

// EL ORIGEN DEL HECHO LO DECIDE LA PUERTA POR LA QUE ENTRA, Y SÓLO ESO.
//
// Un hecho que sale de `device_commands` arrastra el origen de su fila, sea del tipo que sea; uno
// que sale de una sesión no lleva ninguno. La guarda vieja (TestElHechoArrastraElOrigenDelComando)
// clavaba dos ejes, y la auditoría A131 encontró los dos:
//
//   - la CLASE DE FILA: sólo un exec común. Con HechoDeComando descartando el origen de toda fila
//     que no fuera `comando` (C4-m3), los avisos y las operaciones de canal salían `origen: null`, y
//     fleet, memory y mcp quedaban verdes. Exposición: 26 filas de canal con origen `persona` en los
//     últimos 30 días (22 `musubi:avisar`, la última del 2026-09-21; 4 `musubi:pantalla`).
//   - la CLASE DE SESIÓN: sólo la shell. Con `Origen: OrigenPersona` en HechoDeSesionPantalla (C4-m2)
//     la pantalla salía `persona` y la shell null: dos sesiones contando historias distintas. El
//     comentario de la guarda vieja hasta lo pedía. Exposición: las 4 sesiones de pantalla del
//     cerebro, del 2026-09-02 al 05.
//
// Acá se recorre la puerta de los comandos con cada origen del enum, cada operación declarada más un
// comando del host y una desconocida, y cada clasificación que una fila puede traer; y las dos
// puertas de sesión. Las puertas mismas salen del fuente: toda función del paquete que devuelve un
// Hecho, y todo literal `Hecho{…}` de producción, tiene que estar en una de las tres decididas. Una
// cuarta puerta pide su decisión antes de fabricar un solo hecho.
//
// Sabotaje: que la sesión de pantalla lleve origen `persona` (C4-m2) → las dos sesiones cuentan
// historias distintas.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\t\tTipo:       HechoPantalla,\n"
// arnes: a="\t\tTipo:       HechoPantalla,\n\t\tOrigen:     OrigenPersona,\n"
// Sabotaje: que HechoDeComando descarte el origen de toda fila que no sea `comando` (C4-m3) → los
// avisos y las operaciones de canal pierden quién los originó.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="func HechoDeComando(c Comando, device string) Hecho {\n\ttipo := TipoDeComando(c)\n"
// arnes: a="func HechoDeComando(c Comando, device string) Hecho {\n\ttipo := TipoDeComando(c)\n\tif tipo != HechoComando {\n\t\tc.Origen = OrigenDesconocido\n\t}\n"
func TestElOrigenDelHechoLoDecideSuPuerta(t *testing.T) {
	decididas := map[string]bool{"HechoDeComando": true, "HechoDeSesionPantalla": true, "HechoDeSesionShell": true}
	halladas := puertasDeHecho(t)
	// EL PISO: el barrido tiene que encontrar al menos las tres que existen.
	if len(halladas) < len(decididas) {
		t.Fatalf("el barrido encontró %d puertas a Hecho (%v) y hay %d: está roto", len(halladas), halladas, len(decididas))
	}
	for nombre, por := range halladas {
		if !decididas[nombre] {
			t.Errorf("%s fabrica un Hecho (%s) y no es una de las tres puertas decididas: nadie dijo qué "+
				"origen lleva lo que sale de ahí", nombre, por)
		}
	}
	for nombre := range decididas {
		if _, ok := halladas[nombre]; !ok {
			t.Errorf("%s dejó de ser una puerta a Hecho: la decisión de abajo mide algo que ya no existe", nombre)
		}
	}

	producidos := map[TipoDeHecho]int{}

	// La puerta de los comandos: el origen de la fila viaja SIEMPRE, sea el tipo que sea.
	var ops []string
	for op := range opsInternasDeclaradas(t) {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	argvs := [][]string{{"systemctl", "restart", "nginx"}, {"musubi:todavia-no-existe", "x"}}
	for _, op := range ops {
		argvs = append(argvs, []string{op, "Musubi: fulano está haciendo algo acá."})
	}
	clasificaciones := append([]TipoDeHecho{"", "plano_del_futuro"}, TiposDeHecho...)
	for o := range origenesDecididos() {
		for _, argv := range argvs {
			for _, cl := range clasificaciones {
				h := HechoDeComando(Comando{Argv: argv, Clasificacion: cl, Origen: o}, "pc")
				if h.Origen != o {
					t.Errorf("HechoDeComando(%s, clasificación %q) salió de tipo %q con origen %q y la fila dice %q: "+
						"la cronología diría que no se sabe quién originó algo que la bitácora sí dice",
						argv[0], cl, h.Tipo, h.Origen, o)
				}
				producidos[h.Tipo]++
			}
		}
	}

	// Las puertas de las sesiones: no llevan origen. La columna A59 es de device_commands.
	sesiones := []Hecho{
		HechoDeSesionPantalla(SesionPantalla{ID: "s", Principal: "gio", Estado: SesionActiva}, "pc"),
		HechoDeSesionShell(SesionShell{ID: "s", Principal: "gio", Estado: ShellActiva}, "pc"),
	}
	for _, h := range sesiones {
		if h.Origen != OrigenDesconocido {
			t.Errorf("un hecho %q salió con origen %q: una sesión no es una fila de device_commands, y que una "+
				"sola de las dos puertas lo llene hace que dos sesiones cuenten historias distintas", h.Tipo, h.Origen)
		}
		producidos[h.Tipo]++
	}

	// EL PISO: cada tipo del enum salió de alguna puerta.
	for _, tipo := range TiposDeHecho {
		if producidos[tipo] == 0 {
			t.Errorf("ninguna puerta produjo un hecho %q: la prueba no midió su origen", tipo)
		}
	}
}

// constantesDeclaradas devuelve valor → nombre de cada constante del tipo `tipo` que declaran los
// archivos de producción del paquete: con el tipo escrito (`X Tipo = "x"`) o convertido
// (`X = Tipo("x")`). Es el barrido que ata una lista escrita a mano al bloque const que la define.
func constantesDeclaradas(t *testing.T, tipo string) map[string]string {
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
				tipado := esIdent(vs.Type, tipo)
				for i, nombre := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					v := vs.Values[i]
					if c, ok := v.(*ast.CallExpr); ok && esIdent(c.Fun, tipo) && len(c.Args) == 1 {
						v = c.Args[0]
					} else if !tipado {
						continue
					}
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("%s: %s es un %s con un valor que no es un literal; ampliá el barrido",
							fset.Position(vs.Pos()), nombre.Name, tipo)
					}
					valor, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					out[valor] = nombre.Name
				}
			}
		}
	}
	return out
}

// puertasDeHecho devuelve, por nombre de función, las que fabrican un Hecho en los archivos de
// producción del paquete —las que lo devuelven, las que escriben un literal `Hecho{…}` y las que
// escriben uno con el tipo elidido adentro de `[]Hecho{{…}}`— y el motivo por el que cuentan.
//
// LO QUE NO VE, dicho: un Hecho armado desde su valor cero (`var h Hecho` y asignaciones) en una
// función que no lo devuelve, y cualquier literal fuera de este paquete. Hoy no hay ninguno: los
// únicos `Hecho{` de producción del repo son los de las tres puertas.
func puertasDeHecho(t *testing.T) map[string]string {
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
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Type.Results != nil {
				for _, r := range fn.Type.Results.List {
					if esIdent(r.Type, "Hecho") {
						out[fn.Name.Name] = "devuelve un Hecho"
					}
				}
			}
			if fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(nd ast.Node) bool {
				cl, ok := nd.(*ast.CompositeLit)
				if !ok {
					return true
				}
				if esIdent(cl.Type, "Hecho") {
					out[fn.Name.Name] = "escribe un literal Hecho{…} en " + fset.Position(cl.Pos()).String()
				}
				if arr, ok := cl.Type.(*ast.ArrayType); ok && esIdent(arr.Elt, "Hecho") {
					for _, el := range cl.Elts {
						if interior, ok := el.(*ast.CompositeLit); ok && interior.Type == nil {
							out[fn.Name.Name] = "escribe un literal []Hecho{{…}} en " + fset.Position(cl.Pos()).String()
						}
					}
				}
				return true
			})
		}
	}
	return out
}
