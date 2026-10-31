package motor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// DE DÓNDE SALEN LOS BYTES DEL TOKEN SE CUSTODIA LEYENDO EL CÓDIGO, PORQUE NINGUNA PRUEBA DE
// COMPORTAMIENTO PUEDE VERLO.
//
// `espejo_test.go` ya mide dos de los tres ejes del token, y los dos con sabotaje en rojo
// verificado: el ANCHO (`len(tok) != 32`, :351) y que no sea FIJO (sabotaje M6 contra
// `TestDosEspejosNoCompartenPuertoNiToken`, :368). El tercero —LA FUENTE— no tenía nada, y el
// hueco se midió el 2026-10-31: cambiando el import de `crypto/rand` a `math/rand`, `go vet`
// pasa —misma firma `Read([]byte) (int, error)`— y el paquete entero da `ok` con CERO fallas.
//
// NO ES QUE A ESAS DOS PRUEBAS LES FALTE UNA ASERCIÓN: es que no hay ninguna que se les pueda
// agregar. Dos tokens de `math/rand` siguen siendo DISTINTOS entre sí —la secuencia avanza— y
// siguen midiendo 32 caracteres hex, así que las dos guardas que existen son correctas y pasan
// con razón. Lo que cambia no es observable desde el valor: el estado de `math/rand` es
// reconstruible, así que cualquier proceso local puede enumerar los tokens que este proceso va a
// emitir. Y la ruta ES la credencial del espejo.
//
// SE LEE EL AST Y NO EL TEXTO por lo que ya se midió en `cmd/musubi/motor_cableado_test.go`: un
// sabotaje anclado en un literal con el vecino al lado deja de aplicarse cuando el vecino cambia,
// y su verde pasa entonces a significar «no lo apliqué», que es exactamente el verde que esta
// clase de guarda existe para no producir.
//
// ⚠️ Y EL PRECIO ES EL MISMO QUE PAGAN ESAS CINCO: `arnes -overlay` NO LES PUEDE DAR VEREDICTO.
// Ese modo reemplaza archivos para el BUILD de Go, y estas dos guardas no EJECUTAN `espejo.go`:
// lo PARSEAN del disco, así que ven el archivo sano y pasan con el sabotaje puesto. Su veredicto
// se saca aplicando al disco, con una prueba TESTIGO del mismo paquete verde — que es lo que
// separa un rojo de la guarda de un rojo del paquete.
// ─────────────────────────────────────────────────────────────────────────────────────────────

// espejoParseado devuelve el AST de `espejo.go`. La ruta es relativa porque `go test` corre con
// el directorio del paquete como cwd, que es el mismo donde vive el archivo.
func espejoParseado(t *testing.T) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "espejo.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("no se pudo parsear internal/motor/espejo.go: %v", err)
	}
	return f
}

// funcionDelEspejo busca una función de nivel superior por nombre y exige que tenga cuerpo: una
// declaración sin cuerpo no se puede inspeccionar, y devolverla haría que las guardas de abajo
// pasaran por no encontrar nada que revisar.
func funcionDelEspejo(t *testing.T, f *ast.File, nombre string) *ast.FuncDecl {
	t.Helper()
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if ok && fd.Name != nil && fd.Name.Name == nombre && fd.Body != nil && fd.Recv == nil {
			return fd
		}
	}
	t.Fatalf("`espejo.go` ya no declara la función %s con cuerpo", nombre)
	return nil
}

// rutaImportada decodifica el literal del import con `strconv.Unquote`, que es la inversa exacta
// de cómo el parser lo guardó.
func rutaImportada(t *testing.T, imp *ast.ImportSpec) string {
	t.Helper()
	ruta, err := strconv.Unquote(imp.Path.Value)
	if err != nil {
		t.Fatalf("import con un literal ilegible (%s): %v", imp.Path.Value, err)
	}
	return ruta
}

// nombreLocal devuelve con qué identificador se nombra un import en este archivo: el alias si lo
// tiene, y el último segmento de la ruta si no. Hace falta porque la guarda de la llamada busca
// `<local>.Read`, y un `crand "crypto/rand"` seguiría siendo correcto.
func nombreLocal(ruta string, imp *ast.ImportSpec) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	seg := strings.Split(ruta, "/")
	return seg[len(seg)-1]
}

// TestElTokenDelEspejoSaleDeUnaFuenteCriptografica custodia el único eje del token que no se ve
// desde afuera: de dónde salen los 16 bytes.
//
// LA REGLA ES POR RUTA Y NO POR IDENTIFICADOR, y eso es a propósito. Mirar que se llame
// `rand.Read` no distingue nada —`math/rand` también se llama `rand` y también tiene `Read`—, así
// que lo que se revisa es la RUTA de todo import que termine aportando un paquete llamado `rand`.
// Así quedan cazados los tres disfraces del mismo cambio: `"math/rand"`, `"math/rand/v2"` (cuyo
// último segmento es `v2`, por eso se miran TODOS los segmentos y no sólo el último) y un alias
// `rand "math/rand"` que dejaría el resto del archivo idéntico.
//
// Sabotaje que la hace fallar: el import pasa a `math/rand`, que compila igual y da un token predecible.
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\"crypto/rand\""
// arnes: a="\t\"math/rand\""
func TestElTokenDelEspejoSaleDeUnaFuenteCriptografica(t *testing.T) {
	f := espejoParseado(t)

	local := ""
	for _, imp := range f.Imports {
		ruta := rutaImportada(t, imp)
		if ruta == "crypto/rand" {
			local = nombreLocal(ruta, imp)
			continue
		}
		for _, seg := range strings.Split(ruta, "/") {
			if seg == "rand" {
				t.Errorf("`espejo.go` importa %q: el token del espejo tiene que salir de crypto/rand "+
					"y de nada más, porque la ruta ES la credencial", ruta)
			}
		}
	}
	if local == "" {
		t.Fatalf("`espejo.go` ya no importa crypto/rand")
	}

	// Y que la función lo USE: un import presente que nadie llama no genera un byte. Se cuenta
	// la llamada en vez de sólo buscarla, para que el día que aparezca una segunda fuente de
	// bytes en la misma función haya que volver a leer esto en vez de heredar el verde.
	fd := funcionDelEspejo(t, f, "tokenDelEspejo")
	n := 0
	ast.Inspect(fd.Body, func(nodo ast.Node) bool {
		llamada, ok := nodo.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := llamada.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil || sel.Sel.Name != "Read" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == local {
			n++
		}
		return true
	})
	if n != 1 {
		t.Errorf("tokenDelEspejo llama %d veces a %s.Read, se esperaba exactamente 1", n, local)
	}
}

// TestElTokenDelEspejoNoSeDegradaAUnValorFijo custodia la rama del error, y es el ÚNICO camino
// para hacerlo: esa rama es inalcanzable con el Go de este repo —`crypto/rand.Read` ya no
// devuelve error, como explica el comentario de `tokenDelEspejo`— así que ninguna prueba de
// comportamiento puede ejecutarla ni, por lo tanto, verla cambiar.
//
// Eso convierte a cualquier sabotaje contra ella en un mutante VACUO para la suite: se midió el
// 2026-10-31 y reemplazar el `return` por `return "0000000000000000", nil` deja el paquete entero
// en verde, lo cual es la respuesta CORRECTA de unas pruebas que sólo observan comportamiento. No
// es un hueco que se tape con una aserción más; es una decisión de diseño que sólo se puede
// custodiar estructuralmente. Y la decisión importa: si el contrato de `crypto/rand` se afloja o
// este código se compila con un Go anterior, un fallback con `nil` emitiría ese token fijo como
// si fuera bueno, que es el modo de falla que el comentario de la función nombra.
//
// Sabotaje que la hace fallar: la rama del error devuelve un token fijo con nil en vez de fallar.
// arnes: archivo="internal/motor/espejo.go"
// arnes: de="\t\treturn \"\", fmt.Errorf(\"generar el token del espejo: %w\", err)"
// arnes: a="\t\treturn \"0000000000000000\", nil"
func TestElTokenDelEspejoNoSeDegradaAUnValorFijo(t *testing.T) {
	fd := funcionDelEspejo(t, espejoParseado(t), "tokenDelEspejo")

	visto := false
	ast.Inspect(fd.Body, func(nodo ast.Node) bool {
		si, ok := nodo.(*ast.IfStmt)
		if !ok || !esErrNoNil(si.Cond) {
			return true
		}
		visto = true
		for _, st := range si.Body.List {
			ret, ok := st.(*ast.ReturnStmt)
			if !ok {
				continue
			}
			if len(ret.Results) != 2 {
				t.Errorf("la rama del error devuelve %d valores, se esperaban 2", len(ret.Results))
				continue
			}
			// El token tiene que ser la cadena VACÍA y no un valor cualquiera: lo que no se
			// puede emitir acá es algo que el caller pueda confundir con un token bueno.
			lit, ok := ret.Results[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || lit.Value != `""` {
				t.Errorf("la rama del error no devuelve la cadena vacía como token")
			}
			if id, ok := ret.Results[1].(*ast.Ident); ok && id.Name == "nil" {
				t.Errorf("la rama del error devuelve nil: un token sin error es un token que el " +
					"caller va a usar")
			}
		}
		return true
	})
	if !visto {
		t.Errorf("tokenDelEspejo ya no revisa el error de Read: sin ese `if`, un día en que la " +
			"fuente falle el token sale igual")
	}
}

// esErrNoNil reconoce la condición `err != nil`.
func esErrNoNil(e ast.Expr) bool {
	bin, ok := e.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	x, ok := bin.X.(*ast.Ident)
	if !ok || x.Name != "err" {
		return false
	}
	y, ok := bin.Y.(*ast.Ident)
	return ok && y.Name == "nil"
}
