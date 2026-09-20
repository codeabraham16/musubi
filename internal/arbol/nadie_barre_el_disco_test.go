package arbol_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/arbol"
)

// TestNingunaPruebaAveriguaQueHayEnElRepoCaminandoElDisco — la guarda que hace que este cabo
// CONVERJA en vez de cerrarse doce veces.
//
// EL CABO ERA A128, y lo que lo define no es cuántos sitios había sino que cerrarlos uno por uno
// no termina nunca: el que escriba el `filepath.WalkDir` número trece no va a saber que existía
// una regla. Por eso acá no hay una lista de sitios arreglados —eso envejece— sino UNA pregunta
// sobre todo el árbol: ninguna prueba averigua qué hay en el repo caminando el disco.
//
// LO QUE SÍ SE PUEDE CAMINAR ES LO QUE LA PRUEBA CONSTRUYÓ. Un `t.TempDir()` que la propia prueba
// llenó es la pregunta correcta para un `WalkDir`: ahí el disco ES la fuente, no hay índice de git
// que consultar, y recorrerlo es la única forma de ver qué quedó. La regla distingue esos dos
// mundos y no prohíbe una función.
//
// NO PUEDE QUEDARSE MUDA, y eso importa más que de costumbre acá: una guarda que barre el árbol y
// no encuentra nada se ve idéntica a un árbol sano. Si el parseo deja de reconocer llamadas —o si
// desaparece el único caso legítimo que queda—, falla en vez de callarse.
//
// Sabotaje que la hace fallar: apuntar a la raíz del repo el único recorrido legítimo que queda.
// Sale roja por el control de «no quedó ningún legítimo» y no por la acusación, porque ese
// recorrido es el mismo que sostiene el control — conviene saberlo antes de leer el rojo.
//
// LA OTRA DIRECCIÓN, la que de verdad importa, se verificó a mano el 2026-09-20 y tiene una trampa
// que vale más que el sabotaje: se agregó un `_test.go` nuevo con un `filepath.WalkDir(raiz, …)` y
// la guarda NO LO VIO, porque el archivo estaba SIN TRACKEAR y este barrido deriva de `git
// ls-files`. Con `git add` sale roja nombrando archivo, línea y función. Es la propia medicina de
// este cabo aplicada a su guarda: para el repo, un archivo sin `git add` no existe.
// arnes: archivo="internal/mcp/arsenal_arranque_test.go"
// arnes: de="filepath.WalkDir(filepath.Dir(root), func"
// arnes: a="filepath.WalkDir(filepath.Join(\"..\", \"..\"), func"
func TestNingunaPruebaAveriguaQueHayEnElRepoCaminandoElDisco(t *testing.T) {
	raiz := filepath.Join("..", "..")
	pruebas, err := arbol.ConSufijo(raiz, "_test.go")
	if err != nil {
		t.Fatalf("no pude enumerar las pruebas del repo: %v", err)
	}

	type sitio struct {
		ruta  string
		linea int
		raizE string
		fn    string
	}
	var culpables, legitimos []sitio

	for _, rel := range pruebas {
		// El propio archivo de esta guarda nombra `filepath.WalkDir` en su prosa; lo que se mira
		// es el AST, así que un comentario no puede acusarla ni salvarla.
		crudo, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		fset := token.NewFileSet()
		arch, err := parser.ParseFile(fset, rel, crudo, 0)
		if err != nil {
			continue // no compila o tiene un build tag raro: no es lo que esta guarda mide
		}
		for _, decl := range arch.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				llamada, ok := n.(*ast.CallExpr)
				if !ok || len(llamada.Args) == 0 {
					return true
				}
				sel, ok := llamada.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				paq, ok := sel.X.(*ast.Ident)
				if !ok || paq.Name != "filepath" || (sel.Sel.Name != "Walk" && sel.Sel.Name != "WalkDir") {
					return true
				}
				s := sitio{
					ruta:  rel,
					linea: fset.Position(llamada.Pos()).Line,
					raizE: textoDe(crudo, fset, llamada.Args[0]),
					fn:    fn.Name.Name,
				}
				if apuntaALaRaizDelRepo(crudo, fset, fn, s.raizE) {
					culpables = append(culpables, s)
				} else {
					legitimos = append(legitimos, s)
				}
				return true
			})
		}
	}

	if len(culpables)+len(legitimos) == 0 {
		t.Fatal("no se reconoció NI UNA llamada a `filepath.Walk`/`WalkDir` en todo el repo.\n" +
			"  Eso no es «nadie camina el disco»: es que este barrido dejó de reconocerlas, y una\n" +
			"  guarda que no encuentra nada se ve igual que un árbol sano. Revisá el parseo.")
	}
	if len(legitimos) == 0 {
		t.Fatal("no quedó NI UN recorrido legítimo (sobre un árbol que la propia prueba construyó) en todo el repo.\n" +
			"  Esta guarda distingue dos mundos, y si el mundo permitido desaparece deja de poder\n" +
			"  demostrar que sabe distinguirlos: sería verde por vacío. Si de verdad ya no hay\n" +
			"  ninguno, esta guarda necesita otro control, no un número más bajo.")
	}
	t.Logf("%d recorrido(s) sobre un árbol propio, %d que trepan al repo", len(legitimos), len(culpables))

	for _, c := range culpables {
		t.Errorf("%s:%d (%s) averigua qué hay en el repo CAMINANDO EL DISCO: `filepath.Walk…(%s)`.\n"+
			"  El disco de cada checkout es distinto: un `.go` sin trackear pone esta guarda roja SÓLO\n"+
			"  en local, y un archivo que quedó en un worktree viejo la puede poner VERDE sobre algo\n"+
			"  que el repo no contiene. Lo que viaja al clone lo define git, no el directorio.\n"+
			"  Arreglo: `arbol.Archivos(raiz)` o `arbol.ConSufijo(raiz, \".go\")`, que derivan de\n"+
			"  `git ls-files` y fallan si no midieron nada.\n"+
			"  Si de verdad querés recorrer un árbol que ESTA prueba construyó, que su raíz no trepe\n"+
			"  con `..`: ahí el disco sí es la fuente y esta guarda no se mete.",
			c.ruta, c.linea, c.fn, c.raizE)
	}
}

// textoDe devuelve el código fuente exacto de una expresión.
func textoDe(crudo []byte, fset *token.FileSet, e ast.Expr) string {
	ini, fin := fset.Position(e.Pos()).Offset, fset.Position(e.End()).Offset
	if ini < 0 || fin > len(crudo) || ini >= fin {
		return "?"
	}
	return string(crudo[ini:fin])
}

// apuntaALaRaizDelRepo dice si la raíz de un recorrido sube hasta el árbol del repo.
//
// SE NOMBRA EL DEFECTO Y NO SU COMPLEMENTO, y eso se aprendió acá mismo: la primera versión pedía
// que la raíz saliera de un `t.TempDir()` y acusó a un caso sano —el arsenal recorre el árbol que
// le devuelve su propio ayudante, que ES un TempDir pero no lo dice con esa palabra—. Preguntar
// «¿es de los permitidos?» obliga a enumerar TODAS las formas de construir un árbol propio, que no
// converge; preguntar «¿sube al repo?» tiene una sola forma y es la que se quiere prohibir.
//
// La marca es `..`: un recorrido que trepa por encima del paquete está apuntando al árbol, y ésa
// es exactamente la superficie cuyo contenido lo decide git y no el directorio.
//
// SE SIGUE LA CADENA ENTERA Y NO UN SALTO, y esto también se aprendió midiendo: la versión de un
// salto dejaba pasar dos sitios reales. En uno la raíz viaja `d ← dirs ← Glob(Join(raiz…)) ← raiz`,
// y en el otro `raiz ← raices ← []string{Join("..", …)}`. Resolver sólo la asignación del sitio de
// la llamada es enumerar profundidades, que es la misma trampa un piso más abajo. Acá se marca
// todo identificador de la función cuya definición mencione `..`, y después se propaga a cualquiera
// que mencione a un marcado, hasta que deje de cambiar.
func apuntaALaRaizDelRepo(crudo []byte, fset *token.FileSet, fn *ast.FuncDecl, expr string) bool {
	if strings.Contains(expr, "..") {
		return true
	}
	// Cada identificador de la función, con los textos de los que sale.
	origen := map[string][]string{}
	anotar := func(nombre string, e ast.Expr) {
		if nombre == "" || nombre == "_" || e == nil {
			return
		}
		origen[nombre] = append(origen[nombre], textoDe(crudo, fset, e))
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch a := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range a.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(a.Rhs) {
					anotar(id.Name, a.Rhs[i])
				} else if ok && len(a.Rhs) == 1 {
					// `a, b := f()` — los dos salen de la misma llamada.
					anotar(id.Name, a.Rhs[0])
				}
			}
		case *ast.RangeStmt:
			if id, ok := a.Value.(*ast.Ident); ok {
				anotar(id.Name, a.X)
			}
			if id, ok := a.Key.(*ast.Ident); ok {
				anotar(id.Name, a.X)
			}
		case *ast.ValueSpec:
			for i, nm := range a.Names {
				if i < len(a.Values) {
					anotar(nm.Name, a.Values[i])
				}
			}
		}
		return true
	})

	// Punto fijo: arranca en los que mencionan `..` y se propaga a quien los mencione.
	sube := map[string]bool{}
	for nombre, defs := range origen {
		for _, d := range defs {
			if strings.Contains(d, "..") {
				sube[nombre] = true
			}
		}
	}
	for cambio := true; cambio; {
		cambio = false
		for nombre, defs := range origen {
			if sube[nombre] {
				continue
			}
			for _, d := range defs {
				if !soloConstruyeRutas(d) {
					continue
				}
				for _, tok := range identificadores(d) {
					if sube[tok] {
						sube[nombre] = true
						cambio = true
					}
				}
			}
		}
	}
	for _, tok := range identificadores(expr) {
		if sube[tok] {
			return true
		}
	}
	return false
}

// soloConstruyeRutas dice si un fragmento arma una ruta a partir de sus partes, o si en el medio
// hay una llamada que puede devolver cualquier cosa.
//
// ES LO QUE IMPIDE QUE LA PROPAGACIÓN SE COMA EL ÁRBOL ENTERO, y hubo que medirlo: sin esto, un
// `s, root := servidorConCentral(t, central)` marcaba a `root` como ruta del repo porque alguno de
// sus argumentos lo era, y la guarda terminó acusando al único caso sano que queda. Que una función
// RECIBA algo del repo no vuelve del repo a lo que DEVUELVE.
//
// Se dejan pasar sólo las llamadas de `filepath`, que es lo que arma rutas. Un `append`, un `Glob`
// propio o cualquier ayudante corta la cadena: ahí la raíz ya no se puede seguir leyendo, y esta
// guarda prefiere no acusar a acusar de más.
func soloConstruyeRutas(d string) bool {
	for i := 0; i < len(d); i++ {
		if d[i] != '(' {
			continue
		}
		j := i
		for j > 0 && (d[j-1] == '_' || d[j-1] == '.' ||
			(d[j-1] >= 'a' && d[j-1] <= 'z') || (d[j-1] >= 'A' && d[j-1] <= 'Z') ||
			(d[j-1] >= '0' && d[j-1] <= '9')) {
			j--
		}
		if llamada := d[j:i]; llamada != "" && !strings.HasPrefix(llamada, "filepath.") {
			return false
		}
	}
	return true
}

// identificadores parte un fragmento de código en las palabras que podrían ser variables.
func identificadores(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
}
