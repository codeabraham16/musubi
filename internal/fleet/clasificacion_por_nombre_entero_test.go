package fleet

// clasificacion_por_nombre_entero_test.go es la guarda ESTRUCTURAL del eje del nombre: la gemela de
// TestUnNombreParecidoAUnaOperacionConocidaEsDesconocido, que prueba nombres.
//
// Una prueba de nombres cierra lo que enumera, y el eje del nombre no se termina: la revisión de T7
// dejó en verde la lista de ocho formas con `OpPantalla + "."`, el alfabeto ASCII que la reemplazó
// caza cualquier separador de UN carácter, y un separador de dos (`OpPantalla + "::"`) vuelve a
// pasar. Agrandar el alfabeto a pares sólo mueve el borde. Lo que converge es preguntar por la FORMA
// DE NACER de la clasificación —cómo está escrito el código que decide—, y eso es lo que hace este
// archivo, sobre el AST del paquete resuelto con go/types: cada nombre se sigue por el OBJETO al que
// se refiere, no por cómo se llama.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// importadorVacio contesta cada import con un paquete VACÍO y completo.
//
// Esta guarda necesita saber A QUÉ se refiere cada nombre —el local que recibió la cabeza, la
// constante de una operación, la función ejecutableDe—, y eso es resolución de nombres del paquete,
// no tipos de otros. Cargar los imports de verdad es compilar medio árbol en cada corrida; con
// paquetes vacíos go/types se queja de `strings.HasPrefix` —«no existe»— y sigue, y lo propio queda
// resuelto igual. `strings.HasPrefix` se reconoce por la ruta del paquete importado, que sí se
// resuelve.
type importadorVacio struct{}

// Import implementa types.Importer.
func (importadorVacio) Import(ruta string) (*types.Package, error) {
	p := types.NewPackage(ruta, path.Base(ruta))
	p.MarkComplete()
	return p, nil
}

// paqueteTipado es el paquete de producción parseado y pasado por go/types, con el padre de cada
// nodo: lo que hace falta para seguir un valor por los locales que lo reciben y preguntar dónde
// termina.
type paqueteTipado struct {
	fset   *token.FileSet
	files  []*ast.File
	info   *types.Info
	escala *types.Scope
	padres map[ast.Node]ast.Node
}

// tiparElPaquete parsea los archivos de producción del directorio —los que compila `go build`, de
// todas las plataformas— y los resuelve con go/types.
//
// LOS ERRORES DE TIPOS SE TRAGAN A PROPÓSITO: con importadorVacio hay decenas (`strings.HasPrefix`
// «no existe», los colector_* de cada plataforma se redeclaran entre sí), y ninguno impide resolver
// un nombre del paquete o un local. Lo que sí se exige es que lo que la guarda lee exista: eso lo
// controlan sus pisos.
func tiparElPaquete(t *testing.T) *paqueteTipado {
	t.Helper()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	p := &paqueteTipado{
		fset: token.NewFileSet(),
		info: &types.Info{
			Defs:  map[*ast.Ident]types.Object{},
			Uses:  map[*ast.Ident]types.Object{},
			Types: map[ast.Expr]types.TypeAndValue{},
		},
		padres: map[ast.Node]ast.Node{},
	}
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		// SIN comentarios: lo que se lee es el código, y un comentario no decide nada.
		f, err := parser.ParseFile(p.fset, n, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		p.files = append(p.files, f)
	}
	conf := types.Config{Importer: importadorVacio{}, Error: func(error) {}}
	pkg, _ := conf.Check("musubi/internal/fleet", p.fset, p.files, p.info)
	if pkg == nil {
		t.Fatal("go/types no devolvió el paquete: esta guarda no puede resolver ningún nombre")
	}
	p.escala = pkg.Scope()
	for _, f := range p.files {
		var pila []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				pila = pila[:len(pila)-1]
				return true
			}
			if len(pila) > 0 {
				p.padres[n] = pila[len(pila)-1]
			}
			pila = append(pila, n)
			return true
		})
	}
	return p
}

// faltaDeForma es un lugar del código donde el nombre se decide de una forma que no es la igualdad
// entera.
type faltaDeForma struct {
	pos token.Pos
	msg string
}

// lectorDelNombre lee cómo el paquete decide sobre el nombre de una operación interna.
//
// LA CABEZA es el argv[0] que el agente despacha: el valor de ejecutableDe(…) en todo el paquete, y
// el de LimpiarArgv(…)[0] en ejecutableDe —que la define— y en toda función que ya decide sobre
// operaciones internas (la que nombra una operación o el prefijo, o llama a ejecutableDe,
// EsOperacionInterna o TipoDeArgv). Fuera de ésas, LimpiarArgv(…)[0] es otra decisión: la allowlist
// (PermiteArgv, I10) compara la cabeza ENTERA contra lo que escribió el operador, y no es de esta
// guarda.
type lectorDelNombre struct {
	*paqueteTipado
	ejecutableDe, limpiarArgv, prefijo types.Object
	declEjecutableDe                   *ast.FuncDecl
	declEsOperacionInterna             *ast.FuncDecl
	ops                                map[types.Object]string // constante de una operación → su nombre
	decide                             map[*ast.FuncDecl]bool  // las funciones que deciden sobre operaciones internas
	cabezas                            map[types.Object]bool   // los locales que reciben la cabeza
	limpios                            map[types.Object]bool   // los locales que reciben un LimpiarArgv(…)
	faltas                             []faltaDeForma

	// Lo que se midió, para los pisos: una guarda que no encuentra nada contesta «no hay reglas
	// raras» igual que una que leyó todo.
	llamadasAEjecutableDe, usosDeLaCabeza, nombresComparados int
}

// leerElNombre arma el lector: resuelve las piezas, marca quién decide y sigue la cabeza por los
// locales.
func leerElNombre(t *testing.T) *lectorDelNombre {
	t.Helper()
	l := &lectorDelNombre{
		paqueteTipado: tiparElPaquete(t),
		ops:           map[types.Object]string{},
		decide:        map[*ast.FuncDecl]bool{},
		cabezas:       map[types.Object]bool{},
		limpios:       map[types.Object]bool{},
	}
	delPaquete := func(nombre string) types.Object {
		t.Helper()
		o := l.escala.Lookup(nombre)
		if o == nil {
			t.Fatalf("el paquete no declara %s: se mudó o se renombró, y esta guarda no está mirando nada", nombre)
		}
		return o
	}
	l.ejecutableDe = delPaquete("ejecutableDe")
	l.limpiarArgv = delPaquete("LimpiarArgv")
	l.prefijo = delPaquete("PrefijoOperacionInterna")
	deciden := map[types.Object]bool{
		l.ejecutableDe: true, delPaquete("EsOperacionInterna"): true, delPaquete("TipoDeArgv"): true, l.prefijo: true,
	}
	for valor, nombre := range opsInternasDeclaradas(t) {
		c, ok := delPaquete(nombre).(*types.Const)
		if !ok {
			t.Fatalf("%s (%q) no es una constante para go/types: el barrido del fuente y la resolución no "+
				"dicen lo mismo", nombre, valor)
		}
		l.ops[c] = nombre
		deciden[c] = true
	}
	if len(l.ops) == 0 {
		t.Fatal("no se resolvió ninguna operación declarada: las comparaciones de abajo no reconocerían ninguna")
	}
	l.declEjecutableDe = l.unaFuncion(t, "ejecutableDe")
	l.declEsOperacionInterna = l.unaFuncion(t, "EsOperacionInterna")
	for _, f := range l.files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			l.decide[fd] = fd == l.declEjecutableDe
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && deciden[l.info.Uses[id]] {
					l.decide[fd] = true
				}
				return !l.decide[fd]
			})
		}
	}
	l.seguirLosLocales()
	return l
}

// unaFuncion devuelve la única función sin receptor con ese nombre.
func (l *lectorDelNombre) unaFuncion(t *testing.T, nombre string) *ast.FuncDecl {
	t.Helper()
	var out []*ast.FuncDecl
	for _, f := range l.files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == nombre {
				out = append(out, fd)
			}
		}
	}
	if len(out) != 1 {
		t.Fatalf("el paquete declara %d funciones %s y esta guarda lee exactamente una: o se mudó, o el "+
			"barrido se rompió, y en los dos casos no está mirando nada", len(out), nombre)
	}
	return out[0]
}

// seguirLosLocales marca, hasta que no cambie nada, cada local que recibe la cabeza o un
// LimpiarArgv(…). Es por OBJETO: `op := ejecutableDe(argv)` y `cabeza := ejecutableDe(argv)` son la
// misma forma, y un local que la recibe en cualquier punto de la función queda marcado para toda
// ella.
func (l *lectorDelNombre) seguirLosLocales() {
	marcar := func(lhs, rhs ast.Expr) bool {
		o := l.local(lhs)
		if o == nil {
			return false
		}
		cambio := false
		if l.esCabeza(rhs) && !l.cabezas[o] {
			l.cabezas[o], cambio = true, true
		}
		if l.esLimpio(rhs) && !l.limpios[o] {
			l.limpios[o], cambio = true, true
		}
		return cambio
	}
	for cambio := true; cambio; {
		cambio = false
		for _, f := range l.files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					if (x.Tok == token.DEFINE || x.Tok == token.ASSIGN) && len(x.Lhs) == len(x.Rhs) {
						for i := range x.Lhs {
							cambio = marcar(x.Lhs[i], x.Rhs[i]) || cambio
						}
					}
				case *ast.ValueSpec:
					if len(x.Names) == len(x.Values) {
						for i := range x.Names {
							cambio = marcar(x.Names[i], x.Values[i]) || cambio
						}
					}
				}
				return true
			})
		}
	}
}

// local devuelve el objeto si `e` nombra una variable local de una función, y nil si no.
func (l *lectorDelNombre) local(e ast.Expr) types.Object {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name == "_" {
		return nil
	}
	o := l.info.Defs[id]
	if o == nil {
		o = l.info.Uses[id]
	}
	if v, ok := o.(*types.Var); !ok || v.IsField() || v.Parent() == l.escala {
		return nil
	}
	return o
}

// llamaA dice si `e` es una llamada a la función `fn` del paquete.
func (l *lectorDelNombre) llamaA(e ast.Expr, fn types.Object) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	return ok && l.info.Uses[id] == fn
}

// esCabeza dice si `e` es la cabeza: ejecutableDe(…), LimpiarArgv(…)[0] en una función que decide, o
// un local que la recibió.
func (l *lectorDelNombre) esCabeza(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.CallExpr:
		return l.llamaA(x, l.ejecutableDe)
	case *ast.IndexExpr:
		cero, ok := ast.Unparen(x.Index).(*ast.BasicLit)
		fd := l.funcDe(x)
		return ok && cero.Kind == token.INT && cero.Value == "0" && fd != nil && l.decide[fd] && l.esLimpio(x.X)
	case *ast.Ident:
		return l.cabezas[l.info.Uses[x]]
	}
	return false
}

// esLimpio dice si `e` es un LimpiarArgv(…) o un local que lo recibió.
func (l *lectorDelNombre) esLimpio(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.CallExpr:
		return l.llamaA(x, l.limpiarArgv)
	case *ast.Ident:
		return l.limpios[l.info.Uses[x]]
	}
	return false
}

// esOp dice si `e` es, entera, una constante de operación declarada.
func (l *lectorDelNombre) esOp(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, es := l.ops[l.info.Uses[id]]
	return es
}

// esElPrefijo dice si `e` es, entera, PrefijoOperacionInterna.
func (l *lectorDelNombre) esElPrefijo(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && l.info.Uses[id] == l.prefijo
}

// nombraOperacion dice si `e` nombra una operación interna: una Op*, el prefijo o un literal que
// empieza con el prefijo, solos o sumados a otro texto. `OpPantalla + "::"` NOMBRA una operación, y
// ése es el punto: es otro nombre armado con uno decidido.
func (l *lectorDelNombre) nombraOperacion(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return l.esOp(x) || l.esElPrefijo(x)
	case *ast.BasicLit:
		v, err := strconv.Unquote(x.Value)
		return x.Kind == token.STRING && err == nil && strings.HasPrefix(v, PrefijoOperacionInterna)
	case *ast.BinaryExpr:
		return x.Op == token.ADD && (l.nombraOperacion(x.X) || l.nombraOperacion(x.Y))
	}
	return false
}

// paqueteYFuncion devuelve la ruta del paquete importado y el nombre de la función de una llamada
// `paquete.Funcion(…)`; vacíos si la llamada no es de ésas.
func (l *lectorDelNombre) paqueteYFuncion(c *ast.CallExpr) (string, string) {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", ""
	}
	pn, ok := l.info.Uses[id].(*types.PkgName)
	if !ok {
		return "", ""
	}
	return pn.Imported().Path(), sel.Sel.Name
}

// esLaPruebaDelPrefijo dice si `e` es `strings.HasPrefix(<la cabeza>, PrefijoOperacionInterna)`.
func (l *lectorDelNombre) esLaPruebaDelPrefijo(e ast.Expr) bool {
	c, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 2 {
		return false
	}
	ruta, fn := l.paqueteYFuncion(c)
	return ruta == "strings" && fn == "HasPrefix" && l.esCabeza(c.Args[0]) && l.esElPrefijo(c.Args[1])
}

// funcDe devuelve la función que contiene al nodo, o nil si está en el nivel del paquete.
func (l *lectorDelNombre) funcDe(n ast.Node) *ast.FuncDecl {
	for m := n; m != nil; m = l.padres[m] {
		if fd, ok := m.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

// subir devuelve `e` con los paréntesis que lo envuelven, y lo que hay afuera de ellos.
func (l *lectorDelNombre) subir(e ast.Expr) (ast.Expr, ast.Node) {
	h, p := e, l.padres[e]
	for {
		par, ok := p.(*ast.ParenExpr)
		if !ok {
			return h, p
		}
		h, p = par, l.padres[par]
	}
}

// corto imprime un nodo en una línea, con gofmt y cortado: lo que se muestra en el mensaje.
func (l *lectorDelNombre) corto(n ast.Node) string {
	var b bytes.Buffer
	if err := format.Node(&b, l.fset, n); err != nil {
		return fmt.Sprintf("%T", n)
	}
	s := []rune(strings.Join(strings.Fields(b.String()), " "))
	if len(s) > 160 {
		return string(s[:160]) + "…"
	}
	return string(s)
}

// faltar anota una forma que no es la igualdad entera: dónde (`pos`), en qué función (la que
// contiene a `en`) y qué.
func (l *lectorDelNombre) faltar(pos token.Pos, en ast.Node, formato string, args ...any) {
	donde := "el nivel del paquete"
	if fd := l.funcDe(en); fd != nil {
		donde = fd.Name.Name
	}
	l.faltas = append(l.faltas, faltaDeForma{pos: pos, msg: "en " + donde + ", " + fmt.Sprintf(formato, args...)})
}

// juzgarLaCabeza recorre cada lectura de la cabeza y pregunta en qué termina.
func (l *lectorDelNombre) juzgarLaCabeza() {
	for _, f := range l.files {
		ast.Inspect(f, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			switch e.(type) {
			case *ast.CallExpr, *ast.IndexExpr, *ast.Ident:
			default:
				return true
			}
			if !l.esCabeza(e) {
				return true
			}
			if l.llamaA(e, l.ejecutableDe) {
				l.llamadasAEjecutableDe++
			}
			l.juzgarUso(e)
			return true
		})
	}
}

// juzgarUso es EL INVARIANTE: la cabeza sólo termina en una comparación del nombre entero contra una
// operación declarada, en la prueba del prefijo de EsOperacionInterna, o en el return de ejecutableDe.
func (l *lectorDelNombre) juzgarUso(e ast.Expr) {
	h, p := l.subir(e)
	fd := l.funcDe(h)
	bien := func() { l.usosDeLaCabeza++ }
	mal := func(que string, en ast.Node) {
		l.faltar(h.Pos(), h, "la cabeza `%s` %s: %s", types.ExprString(h), que, l.corto(en))
	}
	switch x := p.(type) {
	case *ast.AssignStmt:
		if slices.Contains(x.Lhs, h) {
			return // se escribe, no se lee
		}
		if i := slices.Index(x.Rhs, h); i >= 0 && (x.Tok == token.DEFINE || x.Tok == token.ASSIGN) &&
			len(x.Lhs) == len(x.Rhs) && l.local(x.Lhs[i]) != nil {
			bien() // pasa a un local, que se sigue
			return
		}
		mal("se guarda en algo que no es un local de la función", x)
	case *ast.ValueSpec:
		if i := slices.Index(x.Values, h); i >= 0 && len(x.Names) == len(x.Values) && l.local(x.Names[i]) != nil {
			bien()
			return
		}
		mal("se guarda en algo que no es un local de la función", x)
	case *ast.SwitchStmt:
		ok := true
		for _, st := range x.Body.List {
			for _, v := range st.(*ast.CaseClause).List {
				if !l.esOp(v) {
					ok = false
					mal("es la etiqueta de un switch con un caso que no es una operación declarada", v)
				}
			}
		}
		if ok {
			bien()
		}
	case *ast.IndexExpr:
		if x.Index != h {
			mal("se lee por pedazos", x)
			return
		}
		if porque := l.mapaDeOperaciones(x.X); porque != "" {
			mal("es la clave de un mapa que no es un literal de operaciones quieto ("+porque+")", x)
			return
		}
		if l.seEscribe(x) {
			mal("es la clave con que se ESCRIBE un mapa", x)
			return
		}
		bien()
	case *ast.BinaryExpr:
		otro := x.X
		if otro == h {
			otro = x.Y
		}
		if (x.Op == token.EQL || x.Op == token.NEQ) && l.esOp(otro) {
			bien()
			return
		}
		mal("se compara con algo que no es una operación declarada, entera", x)
	case *ast.CallExpr:
		if fd == l.declEsOperacionInterna && len(x.Args) == 2 && x.Args[0] == h && l.esLaPruebaDelPrefijo(x) {
			bien()
			return
		}
		mal("se le pasa a una llamada", x)
	case *ast.ReturnStmt:
		if fd == l.declEjecutableDe {
			bien()
			return
		}
		mal("se devuelve, y fuera de ejecutableDe eso es otra cabeza que nadie sigue", x)
	case *ast.SliceExpr:
		mal("se corta", x)
	default:
		mal("termina en un lugar que esta guarda no conoce", p)
	}
}

// mapaDeOperaciones dice por qué `e` NO es un mapa literal de operaciones que nadie modifica ("" si
// lo es): un literal con claves que son constantes de operación, escrito ahí o en la declaración de
// la variable, y leído sólo por índice.
func (l *lectorDelNombre) mapaDeOperaciones(e ast.Expr) string {
	switch x := ast.Unparen(e).(type) {
	case *ast.CompositeLit:
		return l.clavesDeOperaciones(x)
	case *ast.Ident:
		v, ok := l.info.Uses[x].(*types.Var)
		if !ok {
			return x.Name + " no es una variable"
		}
		lit, ok := ast.Unparen(l.valorDeclarado(v)).(*ast.CompositeLit)
		if !ok {
			return x.Name + " no se declara con un literal"
		}
		if porque := l.clavesDeOperaciones(lit); porque != "" {
			return porque
		}
		var otros []token.Pos
		for id, o := range l.info.Uses {
			if o != v {
				continue
			}
			h, p := l.subir(id)
			if ix, ok := p.(*ast.IndexExpr); !ok || ix.X != h || l.seEscribe(ix) {
				otros = append(otros, id.Pos())
			}
		}
		if len(otros) > 0 {
			slices.Sort(otros)
			return fmt.Sprintf("%s se usa para algo más que leerlo por índice en %s", x.Name, l.fset.Position(otros[0]))
		}
		return ""
	}
	return "no es un mapa"
}

// clavesDeOperaciones dice por qué el literal no es un mapa con claves de operación ("" si lo es).
func (l *lectorDelNombre) clavesDeOperaciones(cl *ast.CompositeLit) string {
	tv, ok := l.info.Types[cl]
	if !ok || tv.Type == nil {
		return "no sé de qué tipo es el literal"
	}
	if _, esMapa := tv.Type.Underlying().(*types.Map); !esMapa {
		return "el literal no es un mapa"
	}
	for _, el := range cl.Elts {
		if kv, ok := el.(*ast.KeyValueExpr); !ok || !l.esOp(kv.Key) {
			return "la clave " + types.ExprString(el) + " no es una operación declarada"
		}
	}
	return ""
}

// valorDeclarado devuelve el valor con que se declara la variable, o nil.
func (l *lectorDelNombre) valorDeclarado(v types.Object) ast.Expr {
	for id, o := range l.info.Defs {
		if o != v {
			continue
		}
		switch d := l.padres[id].(type) {
		case *ast.ValueSpec:
			if i := slices.Index(d.Names, id); i >= 0 && len(d.Values) == len(d.Names) {
				return d.Values[i]
			}
		case *ast.AssignStmt:
			if i := slices.Index(d.Lhs, ast.Expr(id)); i >= 0 && d.Tok == token.DEFINE && len(d.Lhs) == len(d.Rhs) {
				return d.Rhs[i]
			}
		}
	}
	return nil
}

// seEscribe dice si el índice está del lado que se escribe.
func (l *lectorDelNombre) seEscribe(ix *ast.IndexExpr) bool {
	h, p := l.subir(ix)
	switch x := p.(type) {
	case *ast.AssignStmt:
		return slices.Contains(x.Lhs, h)
	case *ast.IncDecStmt:
		return x.X == h
	case *ast.UnaryExpr:
		return x.Op == token.AND
	}
	return false
}

// retornosDe llama a `f` con cada return de la función (no los de una función literal adentro).
func retornosDe(fd *ast.FuncDecl, f func(*ast.ReturnStmt)) {
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			f(x)
		}
		return true
	})
}

// juzgarLasPrimitivas pregunta si ejecutableDe y EsOperacionInterna devuelven SÓLO lo que son.
func (l *lectorDelNombre) juzgarLasPrimitivas() {
	devuelveLaCabeza := false
	retornosDe(l.declEjecutableDe, func(r *ast.ReturnStmt) {
		if len(r.Results) == 1 {
			if l.esCabeza(r.Results[0]) {
				devuelveLaCabeza = true
				return
			}
			if lit, ok := ast.Unparen(r.Results[0]).(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, err := strconv.Unquote(lit.Value); err == nil && v == "" {
					return
				}
			}
		}
		l.faltar(r.Pos(), r, "se devuelve algo que no es la cabeza de LimpiarArgv ni el vacío: %s", l.corto(r))
	})
	if !devuelveLaCabeza {
		l.faltar(l.declEjecutableDe.Body.Rbrace, l.declEjecutableDe, "ningún return devuelve la cabeza de "+
			"LimpiarArgv (su [0], por el local que sea): ejecutableDe cambió de forma, o esta guarda dejó de "+
			"reconocerla")
	}
	esLaPrueba := false
	retornosDe(l.declEsOperacionInterna, func(r *ast.ReturnStmt) {
		if len(r.Results) == 1 && l.esLaPruebaDelPrefijo(r.Results[0]) {
			esLaPrueba = true
			return
		}
		l.faltar(r.Pos(), r, "se devuelve algo que no es strings.HasPrefix(<la cabeza>, PrefijoOperacionInterna): %s",
			l.corto(r))
	})
	if !esLaPrueba {
		l.faltar(l.declEsOperacionInterna.Body.Rbrace, l.declEsOperacionInterna,
			"ningún return devuelve la prueba del prefijo sobre la cabeza")
	}
}

// paquetesQueComparan son los de la biblioteca estándar por donde Go compara o busca un texto en
// otro. Un nombre de operación pasado a uno de éstos es una comparación, aunque no tenga `==`.
var paquetesQueComparan = map[string]bool{"strings": true, "bytes": true, "regexp": true, "path": true, "path/filepath": true}

// juzgarLosNombres es LA OTRA MITAD: una regla que lea el argv[0] CRUDO no pasa por la cabeza, pero
// para comparar tiene que nombrar lo que busca. Todo nombre de operación que aparece en una
// comparación tiene que ser una operación entera frente a la cabeza, o el prefijo en la prueba de
// EsOperacionInterna.
func (l *lectorDelNombre) juzgarLosNombres() {
	for _, f := range l.files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				switch x.Op {
				case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
				default:
					return true
				}
				for _, par := range [][2]ast.Expr{{x.X, x.Y}, {x.Y, x.X}} {
					nombre, otro := par[0], par[1]
					if !l.nombraOperacion(nombre) {
						continue
					}
					if (x.Op == token.EQL || x.Op == token.NEQ) && l.esOp(nombre) && l.esCabeza(otro) {
						l.nombresComparados++
						continue
					}
					l.faltar(nombre.Pos(), nombre, "el nombre de una operación (%s) se compara con algo que no "+
						"es la cabeza entera: %s", types.ExprString(nombre), l.corto(x))
				}
			case *ast.SwitchStmt:
				if x.Tag == nil {
					return true
				}
				for _, st := range x.Body.List {
					for _, v := range st.(*ast.CaseClause).List {
						if !l.nombraOperacion(v) {
							continue
						}
						if l.esOp(v) && l.esCabeza(x.Tag) {
							l.nombresComparados++
							continue
						}
						l.faltar(v.Pos(), v, "el caso %s de un switch sobre %s compara el nombre de una operación "+
							"con algo que no es la cabeza entera", types.ExprString(v), types.ExprString(x.Tag))
					}
				}
			case *ast.CallExpr:
				ruta, fn := l.paqueteYFuncion(x)
				if !paquetesQueComparan[ruta] {
					return true
				}
				for i, arg := range x.Args {
					if !l.nombraOperacion(arg) {
						continue
					}
					if i == 1 && l.funcDe(x) == l.declEsOperacionInterna && l.esElPrefijo(arg) && l.esLaPruebaDelPrefijo(x) {
						l.nombresComparados++
						continue
					}
					l.faltar(arg.Pos(), arg, "el nombre de una operación (%s) se le pasa a %s.%s: %s",
						types.ExprString(arg), path.Base(ruta), fn, l.corto(x))
				}
			}
			return true
		})
	}
}

// LA CLASIFICACIÓN POR NOMBRE SÓLO COMPARA EL NOMBRE ENTERO, Y ESO SE MIRA EN EL CÓDIGO QUE DECIDE.
//
// Del argv de una fila, la cronología saca el tipo por la CABEZA: ejecutableDe, el argv[0] que
// LimpiarArgv deja y que el agente despacha con igualdad exacta (cmd/musubi/ejecutor.go). Una regla
// que no compare esa cabeza ENTERA contra una operación decidida —un prefijo con un separador, una
// normalización, una excepción— le presta la clasificación de una operación a un nombre que nadie
// decidió, y la cronología muestra como pantalla algo que la máquina rechaza como desconocido
// (C1-m4 en la auditoría A131).
//
// ESTA GUARDA NO ENUMERA NOMBRES NI COPIA CÓDIGO: pregunta por el INVARIANTE, sobre el AST de los
// archivos de producción del paquete, y cada valor se sigue por el objeto y no por el nombre del
// local. Tres preguntas:
//
//   - LA CABEZA (ver lectorDelNombre) sólo puede terminar en: la etiqueta de un switch —con o sin
//     inicialización— cuyos casos son operaciones declaradas; la clave con que se LEE un mapa
//     literal cuyas claves son operaciones declaradas y que nadie modifica; un `==`/`!=` contra una
//     operación declarada; en EsOperacionInterna, `strings.HasPrefix(…, PrefijoOperacionInterna)`;
//     en ejecutableDe, su return; o un local, que se sigue. Cualquier otra cosa es ROJO: otra
//     función de strings, un corte, un índice, pasarla a otra llamada, compararla contra algo que no
//     es una operación entera.
//   - LAS DOS PRIMITIVAS devuelven sólo lo que son: ejecutableDe, la cabeza de LimpiarArgv o el
//     vacío; EsOperacionInterna, la prueba del prefijo sobre la cabeza. Una excepción que mire el
//     argv por otro lado —sin la cabeza y sin nombrar ninguna operación— cae acá.
//   - LOS NOMBRES: el nombre de una operación (una Op*, el prefijo o un literal `"musubi:…"`, solo o
//     sumado a otro texto) que aparece en una COMPARACIÓN —un operador de comparación, el caso de un
//     switch, una función de strings, bytes, regexp o path— sólo puede ser una operación entera
//     frente a la cabeza, o el prefijo en la prueba de EsOperacionInterna. Es la otra mitad: una
//     regla sobre el argv[0] CRUDO no pasa por la cabeza, pero tiene que nombrar lo que busca.
//
// Una operación nueva entra sin tocar esta prueba: es un caso más del switch, o una clave más del
// mapa. Una forma nueva de decidir es ROJO, y el mensaje dice dónde y qué.
//
// LA PRIMERA VERSIÓN PREGUNTABA OTRA COSA Y CASTIGABA EL ARREGLO (falla 7). Comparaba el TEXTO de
// ejecutableDe, EsOperacionInterna, TipoDeArgv y TipoDeComando contra una copia escrita a mano, y la
// segunda revisión de T7 le pasó cuatro cambios correctos y equivalentes: el switch con
// inicialización (`switch op := ejecutableDe(argv); op {`), el mapa de búsqueda exacta que había
// propuesto la auditoría, renombrar el local de ejecutableDe y un `default:` dentro del switch. Los
// cuatro la ponían ROJA, con mensajes que los acusaban de «comparar algo que no es el nombre
// entero». Un golden del texto no distingue una forma nueva de una regla nueva. Las cuatro primeras
// directivas de abajo declaran esos cuatro cambios como su `arreglo`, y el arnés mide las dos
// direcciones.
//
// MEDIDO sobre la rama rebasada en 70811c1, con `go test ./internal/fleet` ENTERO y cada directiva
// puesta: las cinco reglas `::` de abajo —en TipoDeArgv, ejecutableDe, EsOperacionInterna,
// TipoDeComando y HechoDeComando—, la del argv[0] crudo y la excepción sin nombres de
// EsOperacionInterna caen SÓLO por esta prueba: ningún nombre que recorren las de comportamiento
// tiene dos dos-puntos seguidos. La lectura cruda en ejecutableDe la ven además
// TestLaCabezaDelArgvSeLeeComoLaDespachaElAgente y
// TestUnaPoliticaNoEncolaUnaOperacionInternaDisfrazada. Con cada uno de los cuatro arreglos el
// paquete entero queda en verde.
//
// EL CONTROL, porque una guarda estructural que no lee nada contesta «no hay reglas raras» igual que
// una que leyó todo: ejecutableDe y EsOperacionInterna se encuentran exactamente una vez, cada
// operación declarada se resuelve a su constante, y tiene que haber al menos una llamada a
// ejecutableDe, un uso de la cabeza aceptado y un nombre comparado contra la cabeza. Si la
// resolución de nombres se rompiera, la cabeza que devuelve ejecutableDe dejaría de reconocerse, y
// eso ya es ROJO.
//
// LO QUE NO MIRA, dicho para que nadie le crea de más: una regla fuera de las dos primitivas que no
// nombre ninguna operación y no lea la cabeza —o la lea en una función auxiliar que no nombra
// ninguna operación ni llama a las primitivas— (en TipoDeArgv, `strings.Contains(strings.Join(argv,
// " "), "::")`: no compara nombres, compara formas); un nombre armado con pedazos que no empiezan
// con el prefijo, o con fmt; la clasificación que vive fuera de este paquete (la superficie de mcp,
// y el despacho del agente en cmd/musubi, que compara con igualdad exacta contra estas constantes);
// y el eje de los blancos y el del largo, que miden TestLaCabezaDelArgvSeLeeComoLaDespachaElAgente y
// TestUnaOperacionInternaSeClasificaIgualConCualquierLargo.
//
// Sabotaje: una regla por prefijo con separador de dos caracteres en TipoDeArgv → `musubi:pantalla::x`
// se muestra como pantalla y ninguna prueba de nombres lo ve.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tswitch ejecutableDe(argv) {"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(argv), OpPantalla+\"::\") {\n\t\treturn HechoCanalPantalla\n\t}\n\tswitch ejecutableDe(argv) {"
// arnes: arreglo_de="\tswitch ejecutableDe(argv) {"
// arnes: arreglo_a="\tswitch op := ejecutableDe(argv); op {"
// Sabotaje: que ejecutableDe normalice la cabeza cortándola en `::` → `musubi:pantalla::x` se lee
// como `musubi:pantalla`, que el agente no despacha así.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn limpio[0]"
// arnes: a="\treturn strings.SplitN(limpio[0], \"::\", 2)[0]"
// arnes: arreglo_de="\tlimpio := LimpiarArgv(argv)\n\tif len(limpio) == 0 {\n\t\treturn \"\"\n\t}\n\treturn limpio[0]"
// arnes: arreglo_a="\tcabeza := LimpiarArgv(argv)\n\tif len(cabeza) == 0 {\n\t\treturn \"\"\n\t}\n\treturn cabeza[0]"
// Sabotaje: que EsOperacionInterna exceptúe los nombres con `::` → caen en `comando`, que ve
// cualquiera con `exec`.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="PrefijoOperacionInterna)\n}"
// arnes: a="PrefijoOperacionInterna) && !strings.Contains(ejecutableDe(argv), \"::\")\n}"
// arnes: arreglo_de="\tswitch ejecutableDe(argv) {\n\tcase OpPantalla:\n\t\treturn HechoCanalPantalla\n\tcase OpShell:\n\t\treturn HechoCanalShell\n\t}\n\treturn HechoSinClasificar\n}\n"
// arnes: arreglo_a="\tif t, ok := clasificacionPorArgv[ejecutableDe(argv)]; ok {\n\t\treturn t\n\t}\n\treturn HechoSinClasificar\n}\n\nvar clasificacionPorArgv = map[string]TipoDeHecho{OpPantalla: HechoCanalPantalla, OpShell: HechoCanalShell}\n"
// Sabotaje: una regla por prefijo con `::` en TipoDeComando, antes de caer a TipoDeArgv → la
// clasificación de la fila lee el nombre por otro lado.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn TipoDeArgv(c.Argv)\n}"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(c.Argv), OpPantalla+\"::\") {\n\t\treturn HechoCanalPantalla\n\t}\n\treturn TipoDeArgv(c.Argv)\n}"
// arnes: arreglo_de="\tcase OpShell:\n\t\treturn HechoCanalShell\n\t}\n\treturn HechoSinClasificar\n}"
// arnes: arreglo_a="\tcase OpShell:\n\t\treturn HechoCanalShell\n\tdefault:\n\t\treturn HechoSinClasificar\n\t}\n}"
// Sabotaje: la regla `::` en HechoDeComando, la puerta por la que nace el hecho de la cronología (la
// de la segunda revisión de T7) → `musubi:pantalla::x` nace como pantalla aunque TipoDeArgv lo
// esconda.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\treturn Hecho{\n\t\tCuando:     c.Creado,"
// arnes: a="\tif strings.HasPrefix(ejecutableDe(c.Argv), OpPantalla+\"::\") {\n\t\ttipo = HechoCanalPantalla\n\t}\n\treturn Hecho{\n\t\tCuando:     c.Creado,"
// Sabotaje: la regla `::` de TipoDeArgv sobre el argv[0] CRUDO, sin pasar por la cabeza → nada de lo
// que sigue a ejecutableDe la ve; la ve el nombre de la operación comparado contra otra cosa.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tswitch ejecutableDe(argv) {"
// arnes: a="\tif strings.HasPrefix(strings.TrimSpace(argv[0]), OpPantalla+\"::\") {\n\t\treturn HechoCanalPantalla\n\t}\n\tswitch ejecutableDe(argv) {"
// Sabotaje: que EsOperacionInterna exceptúe lo que tenga `::` mirando el argv crudo, sin la cabeza y
// sin nombrar ninguna operación → caen en `comando`.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="func EsOperacionInterna(argv []string) bool {\n"
// arnes: a="func EsOperacionInterna(argv []string) bool {\n\tif strings.Contains(strings.Join(argv, \" \"), \"::\") {\n\t\treturn false\n\t}\n"
// Sabotaje: que ejecutableDe devuelva la primera parte CRUDA recortada en vez de la de LimpiarArgv
// (la forma del vivo) → la decisión se toma sobre un argv que no es el que corre.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif len(limpio) == 0 {\n\t\treturn \"\"\n\t}\n"
// arnes: a="\tif len(limpio) == 0 {\n\t\treturn \"\"\n\t}\n\tif len(argv) > 0 {\n\t\treturn strings.TrimSpace(argv[0])\n\t}\n"
func TestLaClasificacionPorNombreSoloComparaElNombreEntero(t *testing.T) {
	l := leerElNombre(t)
	l.juzgarLaCabeza()
	l.juzgarLasPrimitivas()
	l.juzgarLosNombres()

	const porque = "La clasificación por nombre sólo puede comparar el nombre ENTERO —la cabeza que el agente " +
		"despacha— contra una operación declarada. Si el cambio es legítimo, enseñale la forma a esta " +
		"guarda, y medí antes que siga comparando el nombre entero."
	sort.SliceStable(l.faltas, func(i, j int) bool { return l.faltas[i].pos < l.faltas[j].pos })
	for _, f := range l.faltas {
		t.Errorf("%s: %s\n%s", l.fset.Position(f.pos), f.msg, porque)
	}

	// LOS PISOS: lo que la guarda tiene que haber encontrado para que su verde diga algo.
	if l.llamadasAEjecutableDe == 0 {
		t.Fatal("no encontré ni una llamada a ejecutableDe en el paquete: o se mudó, o la resolución de nombres " +
			"se rompió, y esta guarda no está mirando nada")
	}
	if l.usosDeLaCabeza == 0 {
		t.Fatal("ningún uso de la cabeza quedó aceptado: el reconocedor no reconoce ni las formas buenas")
	}
	if l.nombresComparados == 0 {
		t.Fatal("ningún nombre de operación se comparó contra la cabeza: o la clasificación dejó de comparar " +
			"nombres, o esta guarda dejó de verlos")
	}
}
