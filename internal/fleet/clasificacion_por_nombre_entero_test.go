package fleet

// clasificacion_por_nombre_entero_test.go es la guarda ESTRUCTURAL del eje del nombre: la gemela de
// TestUnNombreParecidoAUnaOperacionConocidaEsDesconocido, que prueba nombres.
//
// Una prueba de nombres cierra lo que enumera, y el eje del nombre no se termina: la revisión de T7
// dejó en verde la lista de ocho formas con `OpPantalla + "."`, el alfabeto ASCII que la reemplazó
// caza cualquier separador de UN carácter, y un separador de dos (`OpPantalla + "::"`) vuelve a
// pasar. Agrandar el alfabeto a pares sólo mueve el borde. Lo que converge es preguntar por la FORMA
// DE NACER de la clasificación —cómo está escrito el código que decide—, y eso es lo que hace este
// archivo.

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

// funcionesDelPaquete devuelve, por nombre, las funciones sin receptor que declaran los archivos
// de producción del paquete, con el FileSet que hace falta para imprimirlas.
func funcionesDelPaquete(t *testing.T) (map[string][]*ast.FuncDecl, *token.FileSet) {
	t.Helper()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	out := map[string][]*ast.FuncDecl{}
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		// SIN ParseComments: lo que se compara es el código, y un comentario nuevo adentro de una
		// función no es un cambio de forma.
		f, err := parser.ParseFile(fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
				out[fn.Name.Name] = append(out[fn.Name.Name], fn)
			}
		}
	}
	return out, fset
}

// enUnaLinea imprime un nodo con gofmt y los blancos colapsados: la forma que se compara, para que
// un salto de línea o una sangría no cuenten como un cambio de forma.
func enUnaLinea(t *testing.T, fset *token.FileSet, n ast.Node) string {
	t.Helper()
	var b bytes.Buffer
	if err := format.Node(&b, fset, n); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// LA CLASIFICACIÓN POR NOMBRE SÓLO COMPARA EL NOMBRE ENTERO, Y ESO SE MIRA EN LA FORMA DEL CÓDIGO.
//
// Del argv de una fila, la cronología saca el tipo por un solo camino: TipoDeComando (si la fila no
// trae su clasificación) → TipoDeArgv → EsOperacionInterna y ejecutableDe, la cabeza que LimpiarArgv
// deja y que el agente despacha con igualdad exacta (cmd/musubi/ejecutor.go). Una regla en
// cualquiera de esos cuatro que no sea la igualdad entera —un prefijo con un separador, una
// normalización, una excepción— le presta la clasificación de una operación decidida a un nombre
// que nadie decidió, y la cronología muestra como pantalla algo que la máquina rechaza como
// desconocido (C1-m4 en la auditoría A131).
//
// ESTA GUARDA NO ENUMERA NOMBRES: pregunta UNA cosa, si esos cuatro tienen la forma que sólo puede
// comparar el nombre entero. ejecutableDe y EsOperacionInterna, exactas; TipoDeArgv, tres
// sentencias —el `if` del prefijo, un switch sobre ejecutableDe(argv) cuyos casos son constantes
// declaradas del bloque de operaciones y cuyos cuerpos son `return <un TipoDeHecho declarado>`, y
// el `return HechoSinClasificar`—; TipoDeComando, que del argv lea sólo `TipoDeArgv(c.Argv)`. Una
// operación nueva entra sin tocar esta prueba: es un caso más del switch. Una forma nueva cualquiera
// es ROJO, y el mensaje pide enseñársela; lo que no hace es callarla.
//
// MEDIDO: las cuatro directivas de abajo agregan una regla con el separador `::` —una en cada
// pieza—, y con cada una `go test ./internal/fleet` cae SÓLO por ésta: las pruebas de nombres
// quedan en verde, porque ningún nombre que recorren tiene dos dos-puntos seguidos.
//
// EL CONTROL, porque una guarda estructural que no lee nada contesta «no hay reglas raras» igual
// que una que leyó todo: cada función se encuentra exactamente una vez, y el switch leído tiene que
// decir lo mismo que clasificacionPorArgvDecidida —si el reconocedor se rompiera, leería un switch
// vacío y eso ya no coincide—.
//
// LO QUE NO MIRA, dicho para que nadie le crea de más: quién CONSUME el tipo (HechoDeComando, la
// superficie de mcp), que no clasifica sino que recibe el tipo ya decidido; el despacho del agente,
// que vive en cmd/musubi y ya compara con igualdad exacta contra las constantes de este paquete; y
// LimpiarArgv, que la usan el cerebro y el agente, así que un cambio ahí lo hacen los dos lados
// igual y lo guardado sigue siendo lo despachado (eso lo cuida
// TestLaCabezaDelArgvSeLeeComoLaDespachaElAgente).
//
// Sabotaje: una regla por prefijo con separador de dos caracteres en TipoDeArgv → `musubi:pantalla::x`
// se muestra como pantalla y ninguna prueba de nombres lo ve.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tswitch ejecutableDe(argv) {"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(argv), OpPantalla+\"::\") {\n\t\treturn HechoCanalPantalla\n\t}\n\tswitch ejecutableDe(argv) {"
// Sabotaje: que ejecutableDe normalice la cabeza cortándola en `::` → `musubi:pantalla::x` se lee
// como `musubi:pantalla`, que el agente no despacha así.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn limpio[0]"
// arnes: a="\treturn strings.SplitN(limpio[0], \"::\", 2)[0]"
// Sabotaje: que EsOperacionInterna exceptúe los nombres con `::` → caen en `comando`, que ve
// cualquiera con `exec`.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="PrefijoOperacionInterna)\n}"
// arnes: a="PrefijoOperacionInterna) && !strings.Contains(ejecutableDe(argv), \"::\")\n}"
// Sabotaje: una regla por prefijo con `::` en TipoDeComando, antes de caer a TipoDeArgv → la
// clasificación de la fila lee el nombre por otro lado.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn TipoDeArgv(c.Argv)\n}"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(c.Argv), OpPantalla+\"::\") {\n\t\treturn HechoCanalPantalla\n\t}\n\treturn TipoDeArgv(c.Argv)\n}"
func TestLaClasificacionPorNombreSoloComparaElNombreEntero(t *testing.T) {
	fns, fset := funcionesDelPaquete(t)
	una := func(nombre string) *ast.FuncDecl {
		t.Helper()
		if len(fns[nombre]) != 1 {
			t.Fatalf("el paquete declara %d funciones %s y esta guarda lee exactamente una: o se mudó, o "+
				"el barrido se rompió, y en los dos casos no está mirando nada", len(fns[nombre]), nombre)
		}
		return fns[nombre][0]
	}
	sentencias := func(fn *ast.FuncDecl) []string {
		var out []string
		for _, s := range fn.Body.List {
			out = append(out, enUnaLinea(t, fset, s))
		}
		return out
	}
	const ensenale = "Si el cambio es legítimo, enseñale la forma nueva a esta prueba, y medí antes que " +
		"siga comparando el nombre entero."

	// 1 · LAS DOS PIEZAS QUE LEEN LA CABEZA, EXACTAS. Son dos líneas cada una y no tienen por qué
	// crecer: cualquier cosa que se les agregue es una regla sobre el nombre.
	for _, e := range []struct {
		fn, porque string
		quiero     []string
	}{
		{"ejecutableDe", "es la cabeza que el agente despacha —la de LimpiarArgv— y nada más; una " +
			"normalización acá (cortar en un separador, pasar a minúsculas) le presta a un nombre " +
			"ajeno la clasificación de uno decidido, y el agente no normaliza igual",
			[]string{"limpio := LimpiarArgv(argv)", `if len(limpio) == 0 { return "" }`, "return limpio[0]"}},
		{"EsOperacionInterna", "es el prefijo sobre esa cabeza y nada más; una condición de más saca " +
			"nombres del canal y los manda a `comando`, que ve cualquiera con `exec`",
			[]string{"return strings.HasPrefix(ejecutableDe(argv), PrefijoOperacionInterna)"}},
	} {
		if got := sentencias(una(e.fn)); !slices.Equal(got, e.quiero) {
			t.Errorf("%s cambió de forma: %s.\n  es:       %q\n  esperaba: %q\n%s", e.fn, e.porque, got, e.quiero, ensenale)
		}
	}

	// 2 · TipoDeArgv: TRES SENTENCIAS, Y LA DEL MEDIO ES UN SWITCH DE IGUALDAD ENTERA.
	tda := una("TipoDeArgv")
	cuerpo := tda.Body.List
	if len(cuerpo) != 3 {
		t.Fatalf("TipoDeArgv tiene %d sentencias y la forma que esta guarda sabe leer tiene 3 —el `if` del "+
			"prefijo, el switch de igualdad y `return HechoSinClasificar`—:\n  %s\nUna sentencia de más es "+
			"una regla más, y una regla que no es la igualdad entera clasifica nombres que nadie decidió. %s",
			len(cuerpo), strings.Join(sentencias(tda), "\n  "), ensenale)
	}
	if got, quiero := enUnaLinea(t, fset, cuerpo[0]), "if !EsOperacionInterna(argv) { return HechoComando }"; got != quiero {
		t.Errorf("TipoDeArgv abre con %q y no con %q: lo que no es interno tiene que ser `comando` y nada "+
			"más. %s", got, quiero, ensenale)
	}
	if got, quiero := enUnaLinea(t, fset, cuerpo[2]), "return HechoSinClasificar"; got != quiero {
		t.Errorf("TipoDeArgv cierra con %q y no con %q: lo interno que nadie decidió tiene que esconderse. %s",
			got, quiero, ensenale)
	}
	sw, ok := cuerpo[1].(*ast.SwitchStmt)
	if !ok || sw.Init != nil || sw.Tag == nil || enUnaLinea(t, fset, sw.Tag) != "ejecutableDe(argv)" {
		t.Fatalf("la sentencia del medio de TipoDeArgv no es `switch ejecutableDe(argv) {…}` sin "+
			"inicialización: %s\nUn switch sobre otra cosa —la cabeza cortada, en minúsculas, o sin "+
			"etiqueta con condiciones— compara algo que no es el nombre entero. %s", enUnaLinea(t, fset, cuerpo[1]), ensenale)
	}
	ops := map[string]string{} // nombre de la constante → la operación
	for valor, nombre := range opsInternasDeclaradas(t) {
		ops[nombre] = valor
	}
	tipos := map[string]TipoDeHecho{} // nombre de la constante → el tipo
	for valor, nombre := range tiposDeHechoDeclarados(t) {
		tipos[nombre] = TipoDeHecho(valor)
	}
	leida := map[string]TipoDeHecho{} // lo que el switch decide: operación → tipo
	for _, st := range sw.Body.List {
		cc := st.(*ast.CaseClause)
		if len(cc.List) == 0 {
			t.Errorf("el switch de TipoDeArgv tiene un `default:` (%s): es una regla para todo nombre que "+
				"nadie decidió, y eso tiene que caer al `return HechoSinClasificar` de abajo", enUnaLinea(t, fset, cc))
			continue
		}
		var tipo TipoDeHecho
		esRetorno := false
		if len(cc.Body) == 1 {
			if r, ok := cc.Body[0].(*ast.ReturnStmt); ok && len(r.Results) == 1 {
				if id, ok := r.Results[0].(*ast.Ident); ok {
					tipo, esRetorno = tipos[id.Name]
				}
			}
		}
		if !esRetorno {
			t.Errorf("un caso de TipoDeArgv no es `return <un TipoDeHecho declarado>`: %s. Un cuerpo con "+
				"lógica es una regla adentro del caso. %s", enUnaLinea(t, fset, cc), ensenale)
			continue
		}
		for _, x := range cc.List {
			id, esNombre := x.(*ast.Ident)
			op, declarada := "", false
			if esNombre {
				op, declarada = ops[id.Name]
			}
			if !declarada {
				t.Errorf("TipoDeArgv compara la cabeza contra %s, que no es una operación del bloque const: "+
					"la igualdad entera sólo protege si es contra un nombre que alguien decidió (una "+
					"expresión como `OpPantalla + \"-x\"` es otro nombre)", enUnaLinea(t, fset, x))
				continue
			}
			leida[op] = tipo
		}
	}
	// EL CONTROL: el switch leído dice lo decidido. Si el reconocedor no leyera nada, `leida`
	// quedaría vacía y esto ya no coincide.
	quiero := map[string]TipoDeHecho{}
	for op, tipo := range clasificacionPorArgvDecidida() {
		if tipo != HechoSinClasificar {
			quiero[op] = tipo
		}
	}
	if len(quiero) == 0 {
		t.Fatal("clasificacionPorArgvDecidida no clasifica ninguna operación por argv: el control no controla nada")
	}
	if !maps.Equal(leida, quiero) {
		t.Errorf("el switch de TipoDeArgv decide %v y la decisión escrita (clasificacionPorArgvDecidida) es %v", leida, quiero)
	}

	// 3 · TipoDeComando LEE EL ARGV SÓLO A TRAVÉS DE TipoDeArgv. Es la puerta que usa la cronología,
	// y una lectura del nombre acá no pasaría por nada de lo de arriba.
	tdc := una("TipoDeComando")
	if len(tdc.Type.Params.List) != 1 || len(tdc.Type.Params.List[0].Names) != 1 {
		t.Fatalf("TipoDeComando ya no recibe un solo parámetro con nombre: esta guarda no sabe dónde mirar. %s", ensenale)
	}
	param := tdc.Type.Params.List[0].Names[0].Name
	usos, porSelector, argvs, alTipo := 0, 0, 0, 0
	ast.Inspect(tdc.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if x.Name == param {
				usos++
			}
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == param {
				porSelector++
				if x.Sel.Name == "Argv" {
					argvs++
				}
			}
		case *ast.CallExpr:
			if enUnaLinea(t, fset, x) == "TipoDeArgv("+param+".Argv)" {
				alTipo++
			}
		}
		return true
	})
	if argvs != 1 || alTipo != 1 || usos != porSelector {
		t.Errorf("TipoDeComando lee el argv %d vez/veces, %d por `TipoDeArgv(%s.Argv)`, y pasa `%s` entero "+
			"%d vez/veces: la clasificación de una fila mira el nombre por un camino que no es TipoDeArgv. "+
			"El cuerpo es:\n  %s\n%s", argvs, alTipo, param, param, usos-porSelector,
			strings.Join(sentencias(tdc), "\n  "), ensenale)
	}
}
