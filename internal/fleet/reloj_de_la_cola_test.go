package fleet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/arbol"
)

// EL RELOJ DE LA COLA SE CUENTA EN UN SOLO LUGAR, Y LOS TRES QUE LO USAN LO LEEN DE AHÍ.
//
// El doc de LimiteDeVida dice que es la única cuenta del reloj de la cola. Cuando se escribió (A131,
// tema T9) no lo era: el techo de EncolarComando conservaba su copia —`c.Creado.Add(-fleet.
// ComandoVidaMax)`— y lo encontró la revisión, no una prueba. Ninguna podía: la copia daba el MISMO
// texto que LimiteDeVida formateado (el RFC3339 corta la fracción igual que el Truncate), así que las
// tres guardas del techo (cola_test.go) y la de la vista contra la toma quedaban verdes con ella y sin
// ella. Y «hoy da lo mismo» es justo cómo la vista y la toma habían terminado discrepando en el
// segundo del borde: dos cuentas equivalentes, hasta que una cambia sola.
//
// POR ESO ESTO PREGUNTA POR EL FUENTE Y NO POR EL RESULTADO, sobre el repo entero (git ls-files, no
// el disco): ComandoVidaMax —la constante que el reloj resta— se lee SÓLO adentro de LimiteDeVida, y
// los tres lectores que su doc nombra la llaman. Se enumeran los consumidores de la constante y no
// las formas de la cuenta: una copia nueva, se escriba como se escriba, tiene que nombrar la
// constante o dejar de llamar a la función.
//
// EXPOSICIÓN: cero diferencia observable. La copia y LimiteDeVida daban el mismo límite en todo
// instante; lo que esto cuida es el próximo cambio de reloj, que se haría en un lado solo.
//
// LO QUE NO VE, dicho: una cuenta que no nombre la constante —un `15 * time.Minute` escrito a mano en
// una función que no sea ninguno de los tres lectores—. Eso ya no es una copia del reloj sino un reloj
// nuevo; si lo escribe la vista o la toma, lo caza por comportamiento
// TestLaVistaYLaTomaVencenUnPendienteEnElMismoSegundo (internal/memory).
//
// Sabotaje: devolverle al techo de la cola su copia del límite, que es lo que encontró la revisión →
// la cuenta vuelve a estar en dos lugares y ninguna prueba de comportamiento lo nota.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tvivos := fleet.LimiteDeVida(c.Creado).UTC().Format(time.RFC3339)\n"
// arnes: a="\tvivos := c.Creado.Add(-fleet.ComandoVidaMax).UTC().Format(time.RFC3339)\n"
// arnes: colision_ok="TestLoVencidoNoOcupaLugarEnLaCola"
func TestElRelojDeLaColaSeCuentaEnUnSoloLugar(t *testing.T) {
	const (
		constante = "ComandoVidaMax"
		unica     = "LimiteDeVida"
		suArchivo = "internal/fleet/comando.go"
	)
	// Los lectores que nombra el doc de LimiteDeVida, con el archivo donde viven.
	lectores := map[string]string{
		"Vencido":           "internal/fleet/comando.go",
		"tomarComandosEnTx": "internal/memory/comandos.go",
		"EncolarComando":    "internal/memory/comandos.go",
	}
	// Las lecturas de la constante que NO cuentan el reloj —un texto que sólo MUESTRA la vida
	// máxima, por ejemplo— van acá, por «archivo función», con su porqué. Hoy no hay ninguna: la
	// excepción se escribe a la vista, no se gana callando la guarda.
	leenSinContar := map[string]string{}

	raiz := filepath.Join("..", "..")
	gos, err := arbol.ConSufijo(raiz, ".go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var ajenas []string // lecturas de la constante fuera de la cuenta única
	enLaUnica := 0
	hallados, llaman := map[string]bool{}, map[string]bool{}
	for _, rel := range gos {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(raiz, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v", rel, err)
		}
		// Los nombres que DECLARAN (el `ComandoVidaMax = …` del bloque const) no son lecturas.
		declaran := map[*ast.Ident]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if vs, ok := n.(*ast.ValueSpec); ok {
				for _, id := range vs.Names {
					declaran[id] = true
				}
			}
			return true
		})
		for _, d := range f.Decls {
			donde := "a nivel de paquete"
			fn, esFunc := d.(*ast.FuncDecl)
			if esFunc {
				donde = fn.Name.Name
				if lectores[fn.Name.Name] == rel {
					hallados[fn.Name.Name] = true
					llaman[fn.Name.Name] = llaman[fn.Name.Name] || llamaA(fn, unica)
				}
			}
			ast.Inspect(d, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id.Name != constante || declaran[id] {
					return true
				}
				if donde == unica && rel == suArchivo {
					enLaUnica++
					return true
				}
				if _, muestra := leenSinContar[rel+" "+donde]; muestra {
					return true
				}
				ajenas = append(ajenas, rel+":"+strconv.Itoa(fset.Position(id.Pos()).Line)+" ("+donde+")")
				return true
			})
		}
	}

	// EL CONTROL POSITIVO: la lectura que tiene que estar. Sin ella, el barrido no está mirando el
	// fuente, y «ninguna copia» se leería igual que un repo sano.
	if enLaUnica == 0 {
		t.Fatalf("el barrido no encontró la lectura de %s adentro de %s (%s): no está mirando el fuente, "+
			"o la cuenta única se movió y esta prueba mide algo que ya no existe", constante, unica, suArchivo)
	}
	for _, sitio := range ajenas {
		t.Errorf("%s se lee en %s, fuera de %s: el reloj de la cola vuelve a contarse en dos lugares, y el "+
			"día que uno cambie la vista, la toma y el techo van a decir cosas distintas del mismo comando — "+
			"que llame a %s (o, si esa lectura sólo muestra la vida máxima y no cuenta nada, que figure en "+
			"leenSinContar con su porqué)", constante, sitio, unica, unica)
	}
	nombres := make([]string, 0, len(lectores))
	for n := range lectores {
		nombres = append(nombres, n)
	}
	sort.Strings(nombres)
	for _, n := range nombres {
		switch {
		case !hallados[n]:
			t.Errorf("el doc de %s nombra a %s (%s) como lector y no está ahí: o se movió, o el doc cuenta "+
				"lectores que ya no existen", unica, n, lectores[n])
		case !llaman[n]:
			t.Errorf("%s (%s) no llama a %s: o hace su propia cuenta del reloj de la cola, o dejó de usarlo y "+
				"el doc lo sigue nombrando", n, lectores[n], unica)
		}
	}
}

// llamaA dice si el cuerpo de fn llama a una función con ese nombre, a secas (`LimiteDeVida(…)`) o
// calificada (`fleet.LimiteDeVida(…)`).
func llamaA(fn *ast.FuncDecl, nombre string) bool {
	if fn.Body == nil {
		return false
	}
	hay := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := c.Fun.(type) {
		case *ast.Ident:
			hay = hay || f.Name == nombre
		case *ast.SelectorExpr:
			hay = hay || f.Sel.Name == nombre
		}
		return true
	})
	return hay
}
