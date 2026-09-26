package mcp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A131 · T3 (revisión 2) — LA ALLOWLIST DE COMANDOS Y EL COMODÍN SE LEEN SÓLO CON LA GRAMÁTICA:
// NADIE LOS LEE POR SU CUENTA, NI EN internal/mcp NI EN internal/fleet.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ UNA GUARDA SOBRE EL CÓDIGO, Y NO SÓLO SOBRE LO QUE HACE
//
// La revisión 2 de T3 encontró escrito, en el doc de fleet.EsComodin, «ES LA ÚNICA LECTURA DEL
// COMODÍN», y en el de fleet.SelectorAlcanza, «la leen todos los que la necesitan». Las dos frases
// eran falsas: argvPermitido y comandosPermitidos leían la clave de `fleet_exec_allow` con
// `p.ExecAllow[d.Name]` y el comodín con `p.ExecAllow[comodinFlota]`, por su cuenta y sin recortar,
// mientras el informe del rename leía la MISMA clave con SelectorNombra. Y ninguna prueba de
// comportamiento lo veía, por construcción: sobre una clave que llega recortada, la lectura propia
// contesta igual que la gramática, y parsearExecAllow las recorta todas. Una copia de una comparación
// que hoy da lo mismo es justo la que se separa sin avisar —el día que cambia la otra, o el día que un
// segundo camino arma allowlists—, y una tabla de comportamiento sólo la ve cuando ya se separó.
//
// Así que esto mira lo que las tablas no pueden: QUIÉN LEE. Enumera LECTORES y no formas de comparar
// —una lista de formas no converge; la de lectores sí, porque cada uno nuevo la pone roja—:
//
//   - LA ALLOWLIST. Cada lectura del campo `ExecAllow` (o de una variable local que lo copia) en el
//     código de producción de internal/mcp tiene que ser una de éstas: compararlo con nil, medirlo
//     con len, pasárselo a fleet.EntradaDeAllowlist —la gramática— o a parsearExecAllow —el parser,
//     que recorta las claves—, copiarlo a una variable local o al mismo campo de otro principal, o
//     recorrerlo en avisosDeInterpretes, que lee las claves para NOMBRARLAS en un aviso de arranque y
//     no decide nada sobre una máquina. Cualquier otra —un índice, un recorrido en otro lado,
//     pasarlo a otra función— es una lectura propia.
//   - EL COMODÍN. En internal/mcp no aparece: ni fleet.ComodinMaquinas ni un `"*"` escrito. En
//     internal/fleet, ComodinMaquinas sólo se lee adentro de EsComodin, y el único `"*"` es su valor.
//
// PISO: la gramática existe y es la que se dice —EntradaDeAllowlist lee la clave con SelectorNombra y
// el comodín con EsComodin, y no busca en su mapa por clave; EsComodin lee ComodinMaquinas, que vale
// `"*"`—, y los tres lectores que la revisión nombró (argvPermitido, comandosPermitidos e
// impactoDeNombre) le pasan su `ExecAllow`. Si no encuentra la gramática, o a alguno de los tres,
// falla: una guarda que no encuentra lo que custodia mide cero, y cero se lee como «está bien». Los
// archivos salen de `git ls-files` (archivosGo): lo que no está en el repo no es código de producción.
//
// Si algún día un `"*"` de internal/mcp no es el comodín de las máquinas —el trigger de una skill,
// por ejemplo—, se exceptúa acá con su porqué; no se le cambia la forma para que la guarda no lo vea.
//
// Sabotaje: argvPermitido vuelve a leer la allowlist por clave, sin la gramática (el código de antes
// de la revisión 2). El arreglo copia `p.ExecAllow` a una variable local antes de pasársela a la
// gramática: es la misma lectura, y la guarda mide quién lee, no cómo se escribe.
// arnes: archivo="internal/mcp/fleet_authz.go"
// arnes: de="\tif comandos, _, hay := fleet.EntradaDeAllowlist(p.ExecAllow, d.Name); hay {\n\t\treturn fleet.PermiteArgv(comandos, argv) // 2 y 3\n\t}\n"
// arnes: a="\tif lista, hay := p.ExecAllow[d.Name]; hay {\n\t\treturn fleet.PermiteArgv(lista, argv) // 2\n\t}\n\tif lista, hay := p.ExecAllow[fleet.ComodinMaquinas]; hay {\n\t\treturn fleet.PermiteArgv(lista, argv) // 3\n\t}\n"
// arnes: arreglo_de="\tif comandos, _, hay := fleet.EntradaDeAllowlist(p.ExecAllow, d.Name); hay {\n\t\treturn fleet.PermiteArgv(comandos, argv) // 2 y 3\n"
// arnes: arreglo_a="\tporMaquina := p.ExecAllow\n\tif comandos, _, hay := fleet.EntradaDeAllowlist(porMaquina, d.Name); hay {\n\t\treturn fleet.PermiteArgv(comandos, argv) // 2 y 3\n"
//
// Sabotaje: comandosPermitidos vuelve a leer la allowlist por clave, sin la gramática. El arreglo,
// otra vez, es la misma lectura a través de una variable local.
// arnes: archivo="internal/mcp/fleet_authz.go"
// arnes: de="\tif comandos, _, hay := fleet.EntradaDeAllowlist(p.ExecAllow, d.Name); hay {\n\t\treturn comandos, true\n\t}\n"
// arnes: a="\tif lista, hay := p.ExecAllow[d.Name]; hay {\n\t\treturn lista, true\n\t}\n\tif lista, hay := p.ExecAllow[fleet.ComodinMaquinas]; hay {\n\t\treturn lista, true\n\t}\n"
// arnes: arreglo_de="\tif comandos, _, hay := fleet.EntradaDeAllowlist(p.ExecAllow, d.Name); hay {\n\t\treturn comandos, true\n"
// arnes: arreglo_a="\tporMaquina := p.ExecAllow\n\tif comandos, _, hay := fleet.EntradaDeAllowlist(porMaquina, d.Name); hay {\n\t\treturn comandos, true\n"
//
// Sabotaje: la gramática misma copia la lectura del comodín en vez de preguntarle a EsComodin. Da lo
// mismo sobre toda clave —recorta y compara—, así que ninguna prueba de comportamiento lo ve: es la
// segunda lectura que se separa el día que cambie la primera. El arreglo guarda la respuesta de
// EsComodin en una variable: sigue siendo la misma lectura.
// arnes: archivo="internal/fleet/politica.go"
// arnes: de="\t\tif EsComodin(clave) {\n"
// arnes: a="\t\tif strings.TrimSpace(clave) == ComodinMaquinas {\n"
// arnes: arreglo_de="\t\tif EsComodin(clave) {\n"
// arnes: arreglo_a="\t\tif esComodin := EsComodin(clave); esComodin {\n"
func TestLaAllowlistYElComodinSeLeenSoloConLaGramatica(t *testing.T) {
	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()
	var deMcp, deFleet []*ast.File
	for _, rel := range archivosGo(t, raiz) {
		dir := path.Dir(rel)
		if strings.HasSuffix(rel, "_test.go") || (dir != "internal/mcp" && dir != "internal/fleet") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("no pude leer %s: %v — no medí nada", rel, err)
		}
		f, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v — no medí nada", rel, err)
		}
		if dir == "internal/mcp" {
			deMcp = append(deMcp, f)
		} else {
			deFleet = append(deFleet, f)
		}
	}
	if len(deMcp) == 0 || len(deFleet) == 0 {
		t.Fatalf("PISO: encontré %d archivo(s) de producción de internal/mcp y %d de internal/fleet: sin los dos, "+
			"esta guarda no mira a nadie", len(deMcp), len(deFleet))
	}

	propias, porGramatica := lectoresDeLaAllowlist(fset, deMcp)
	for _, v := range propias {
		t.Errorf("%s: %s lee `ExecAllow` por su cuenta (%s). La clave de `fleet_exec_allow` es un selector de "+
			"máquina: la lee fleet.EntradaDeAllowlist, con SelectorNombra y EsComodin y los bordes recortados, la MISMA "+
			"búsqueda para la compuerta, el inventario y el informe del rename. Una lectura propia contesta igual "+
			"mientras las claves lleguen recortadas, y se separa sin avisar el día que no", v.donde, v.funcion, v.como)
	}
	for _, v := range lecturasDelComodin(fset, deMcp, deFleet) {
		t.Errorf("%s: %s lee el comodín por su cuenta (%s). El comodín se lee con fleet.EsComodin —y por ella "+
			"SelectorAlcanza, SelectorNombra y EntradaDeAllowlist—, que recorta los bordes; una segunda lectura es "+
			"la que hizo que puedeOtorgar y la allowlist discreparan con la gramática sobre ` * `", v.donde, v.funcion, v.como)
	}

	// PISO: la gramática existe y es la que se dice.
	for _, falta := range gramaticaQueFalta(fset, deFleet) {
		t.Errorf("PISO: %s. Sin eso, «pasa por la gramática» no significa nada: la búsqueda de la allowlist sería "+
			"otra copia de la comparación, que es lo que esta guarda existe para no dejar escribir", falta)
	}
	// PISO: los tres lectores que la revisión 2 nombró le pasan su allowlist a la gramática.
	for _, nombre := range []string{"argvPermitido", "comandosPermitidos", "impactoDeNombre"} {
		if !porGramatica[nombre] {
			t.Errorf("PISO: %s no le pasa `ExecAllow` a fleet.EntradaDeAllowlist (o no existe). Es uno de los tres "+
				"lectores de la clave de `fleet_exec_allow` —la compuerta, el inventario y el informe del rename—, y "+
				"los tres tienen que contestar con la misma búsqueda para no discrepar sobre qué entrada manda", nombre)
		}
	}
}

// lecturaPropia es un lugar del código que lee algo que sólo debería leer la gramática.
type lecturaPropia struct {
	pos           token.Position
	donde         string
	funcion, como string
}

func nuevaLecturaPropia(fset *token.FileSet, n ast.Node, funcion, como string) lecturaPropia {
	p := fset.Position(n.Pos())
	return lecturaPropia{pos: p, donde: p.String(), funcion: funcion, como: como}
}

// ordenarLecturas las deja en el orden del código, archivo por archivo y línea por línea.
func ordenarLecturas(ls []lecturaPropia) {
	sort.Slice(ls, func(i, j int) bool {
		if ls[i].pos.Filename != ls[j].pos.Filename {
			return ls[i].pos.Filename < ls[j].pos.Filename
		}
		if ls[i].pos.Line != ls[j].pos.Line {
			return ls[i].pos.Line < ls[j].pos.Line
		}
		return ls[i].pos.Column < ls[j].pos.Column
	})
}

// lectoresDeLaAllowlist clasifica cada lectura de `ExecAllow` del código de producción de internal/mcp.
// Devuelve las lecturas propias y qué funciones le pasan su `ExecAllow` a fleet.EntradaDeAllowlist.
func lectoresDeLaAllowlist(fset *token.FileSet, archivos []*ast.File) ([]lecturaPropia, map[string]bool) {
	var propias []lecturaPropia
	porGramatica := map[string]bool{}
	for _, f := range archivos {
		fleetLocal := nombreLocalDeFleet(f)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			copias := copiasDeLaAllowlist(fn.Body)
			var pila []ast.Node
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if n == nil {
					pila = pila[:len(pila)-1]
					return true
				}
				padre := ast.Node(fn.Body)
				if len(pila) > 0 {
					padre = pila[len(pila)-1]
				}
				pila = append(pila, n)
				e, ok := n.(ast.Expr)
				if !ok || !esLaAllowlist(e, copias) || esUnNombreDeCampo(padre, e) {
					return true
				}
				como := ""
				switch p := padre.(type) {
				case *ast.BinaryExpr:
					otro := p.Y
					if otro == e {
						otro = p.X
					}
					if id, esId := otro.(*ast.Ident); !esId || id.Name != "nil" || (p.Op != token.EQL && p.Op != token.NEQ) {
						como = "la compara con algo que no es nil"
					}
				case *ast.CallExpr:
					switch fun := p.Fun.(type) {
					case *ast.SelectorExpr:
						if x, esId := fun.X.(*ast.Ident); esId && x.Name == fleetLocal && fun.Sel.Name == "EntradaDeAllowlist" {
							if len(p.Args) > 0 && p.Args[0] == e {
								porGramatica[fn.Name.Name] = true
							}
						} else {
							como = "se la pasa a " + nombreDeLaLlamada(p)
						}
					case *ast.Ident:
						if fun.Name != "len" && fun.Name != "parsearExecAllow" {
							como = "se la pasa a " + fun.Name
						}
					default:
						como = "se la pasa a " + nombreDeLaLlamada(p)
					}
				case *ast.AssignStmt:
					// Del lado izquierdo es escribirla; del derecho, copiarla a una variable local, que desde ahí
					// se mira con estas mismas reglas (copiasDeLaAllowlist).
					if !estaEnLaLista(p.Lhs, e) && !copiaAUnaVariable(p, e) {
						como = "la asigna a algo que no es una variable local"
					}
				case *ast.ValueSpec:
					// `var x = p.ExecAllow`: una copia, que se sigue igual que la de arriba.
				case *ast.KeyValueExpr:
					if k, esId := p.Key.(*ast.Ident); !esId || k.Name != "ExecAllow" || p.Value != e {
						como = "la copia a otro campo"
					}
				case *ast.RangeStmt:
					if p.X == e && fn.Name.Name != "avisosDeInterpretes" {
						como = "recorre sus claves"
					}
				case *ast.IndexExpr:
					como = "la indexa por clave"
				default:
					como = fmt.Sprintf("la usa donde ninguna regla lo prevé: %T", padre)
				}
				if como != "" {
					propias = append(propias, nuevaLecturaPropia(fset, e, fn.Name.Name, como))
				}
				return true
			})
		}
	}
	ordenarLecturas(propias)
	return propias, porGramatica
}

// esLaAllowlist dice si la expresión es el campo `ExecAllow` o una variable local que lo copia.
func esLaAllowlist(e ast.Expr, copias map[string]bool) bool {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name == "ExecAllow"
	case *ast.Ident:
		return copias[x.Name]
	}
	return false
}

// esUnNombreDeCampo dice si el identificador es el NOMBRE de un campo —`x.nombre`, o la clave de un
// literal compuesto— y no una variable: una copia que se llame igual que un campo no es ese campo.
func esUnNombreDeCampo(padre ast.Node, e ast.Expr) bool {
	switch p := padre.(type) {
	case *ast.SelectorExpr:
		return p.Sel == e
	case *ast.KeyValueExpr:
		_, esId := e.(*ast.Ident)
		return esId && p.Key == e
	}
	return false
}

// copiasDeLaAllowlist devuelve las variables locales que reciben un `ExecAllow` —o una copia de una
// copia—, para que una variable intermedia no esconda un índice ni castigue una lectura correcta.
func copiasDeLaAllowlist(cuerpo *ast.BlockStmt) map[string]bool {
	copias := map[string]bool{}
	for cambio := true; cambio; {
		cambio = false
		ast.Inspect(cuerpo, func(n ast.Node) bool {
			var izq, der []ast.Expr
			switch s := n.(type) {
			case *ast.AssignStmt:
				izq, der = s.Lhs, s.Rhs
			case *ast.ValueSpec:
				for _, nombre := range s.Names {
					izq = append(izq, nombre)
				}
				der = s.Values
			default:
				return true
			}
			if len(izq) != len(der) {
				return true
			}
			for i := range der {
				if id, esId := izq[i].(*ast.Ident); esId && id.Name != "_" && esLaAllowlist(der[i], copias) && !copias[id.Name] {
					copias[id.Name] = true
					cambio = true
				}
			}
			return true
		})
	}
	return copias
}

// copiaAUnaVariable dice si, en esta asignación, `e` está del lado derecho y va a parar a un
// identificador (una copia que copiasDeLaAllowlist sigue).
func copiaAUnaVariable(s *ast.AssignStmt, e ast.Expr) bool {
	if len(s.Lhs) != len(s.Rhs) {
		return false
	}
	for i := range s.Rhs {
		if s.Rhs[i] == e {
			_, esId := s.Lhs[i].(*ast.Ident)
			return esId
		}
	}
	return false
}

// lecturasDelComodin devuelve cada lectura del comodín que no es la de la gramática: en internal/mcp,
// cualquier fleet.ComodinMaquinas o `"*"`; en internal/fleet, ComodinMaquinas fuera de EsComodin y
// un `"*"` que no sea su valor.
func lecturasDelComodin(fset *token.FileSet, deMcp, deFleet []*ast.File) []lecturaPropia {
	var out []lecturaPropia
	revisar := func(f *ast.File, enFleet bool) {
		fleetLocal := nombreLocalDeFleet(f)
		for _, decl := range f.Decls {
			funcion := "(nivel de paquete)"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				funcion = fn.Name.Name
			}
			var valorDelComodin ast.Node // el `"*"` que declara ComodinMaquinas: el único permitido
			ast.Inspect(decl, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ValueSpec:
					for i, nombre := range x.Names {
						if enFleet && nombre.Name == "ComodinMaquinas" && i < len(x.Values) {
							valorDelComodin = x.Values[i]
						}
					}
				case *ast.BasicLit:
					if v, err := strconv.Unquote(x.Value); x.Kind == token.STRING && err == nil && v == "*" && n != valorDelComodin {
						out = append(out, nuevaLecturaPropia(fset, x, funcion, "escribe un `\"*\"`"))
					}
				case *ast.SelectorExpr:
					if id, esId := x.X.(*ast.Ident); !enFleet && esId && id.Name == fleetLocal && x.Sel.Name == "ComodinMaquinas" {
						out = append(out, nuevaLecturaPropia(fset, x, funcion, "lee fleet.ComodinMaquinas"))
					}
				case *ast.Ident:
					if enFleet && x.Name == "ComodinMaquinas" && funcion != "EsComodin" && !esLaDeclaracionDelComodin(decl, x) {
						out = append(out, nuevaLecturaPropia(fset, x, funcion, "lee ComodinMaquinas"))
					}
				}
				return true
			})
		}
	}
	for _, f := range deMcp {
		revisar(f, false)
	}
	for _, f := range deFleet {
		revisar(f, true)
	}
	ordenarLecturas(out)
	return out
}

// esLaDeclaracionDelComodin dice si este identificador es el nombre en `const ComodinMaquinas = "*"`.
func esLaDeclaracionDelComodin(decl ast.Decl, id *ast.Ident) bool {
	g, ok := decl.(*ast.GenDecl)
	if !ok || g.Tok != token.CONST {
		return false
	}
	for _, sp := range g.Specs {
		if vs, ok := sp.(*ast.ValueSpec); ok {
			for _, nombre := range vs.Names {
				if nombre == id {
					return true
				}
			}
		}
	}
	return false
}

// gramaticaQueFalta dice qué le falta a la gramática de internal/fleet para ser la que los docs dicen.
func gramaticaQueFalta(fset *token.FileSet, deFleet []*ast.File) []string {
	var comodinVale string
	var esComodin, entrada *ast.FuncDecl
	for _, f := range deFleet {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, nombre := range vs.Names {
						if nombre.Name != "ComodinMaquinas" || i >= len(vs.Values) {
							continue
						}
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok {
							comodinVale, _ = strconv.Unquote(lit.Value)
						}
					}
				}
			case *ast.FuncDecl:
				switch {
				case x.Recv != nil:
				case x.Name.Name == "EsComodin":
					esComodin = x
				case x.Name.Name == "EntradaDeAllowlist":
					entrada = x
				}
			}
		}
	}
	var faltas []string
	if comodinVale != "*" {
		faltas = append(faltas, "internal/fleet no declara `const ComodinMaquinas = \"*\"`")
	}
	if esComodin == nil || !nombraA(esComodin.Body, "ComodinMaquinas") {
		faltas = append(faltas, "fleet.EsComodin no existe, o no lee ComodinMaquinas")
	}
	if entrada == nil {
		return append(faltas, "fleet.EntradaDeAllowlist no existe")
	}
	for _, llamada := range []string{"SelectorNombra", "EsComodin"} {
		if !llamaA(entrada.Body, llamada) {
			faltas = append(faltas, fmt.Sprintf("fleet.EntradaDeAllowlist (%s) no llama a %s", fset.Position(entrada.Pos()), llamada))
		}
	}
	if params := entrada.Type.Params.List; len(params) > 0 && len(params[0].Names) > 0 {
		mapa := params[0].Names[0].Name
		ast.Inspect(entrada.Body, func(n ast.Node) bool {
			if ix, ok := n.(*ast.IndexExpr); ok {
				if id, esId := ix.X.(*ast.Ident); esId && id.Name == mapa {
					faltas = append(faltas, fmt.Sprintf("%s: fleet.EntradaDeAllowlist busca en su mapa por clave, sin la gramática",
						fset.Position(ix.Pos())))
				}
			}
			return true
		})
	}
	return faltas
}

// nombraA dice si el cuerpo menciona este identificador.
func nombraA(cuerpo *ast.BlockStmt, nombre string) bool {
	hay := false
	if cuerpo != nil {
		ast.Inspect(cuerpo, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == nombre {
				hay = true
			}
			return !hay
		})
	}
	return hay
}

// llamaA dice si el cuerpo llama a esta función del mismo paquete.
func llamaA(cuerpo *ast.BlockStmt, nombre string) bool {
	hay := false
	if cuerpo != nil {
		ast.Inspect(cuerpo, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if id, esId := c.Fun.(*ast.Ident); esId && id.Name == nombre {
					hay = true
				}
			}
			return !hay
		})
	}
	return hay
}

// nombreLocalDeFleet dice con qué nombre importa este archivo a musubi/internal/fleet ("" si no lo
// importa). Con un alias, la guarda lo sigue.
func nombreLocalDeFleet(f *ast.File) string {
	for _, imp := range f.Imports {
		if ruta, err := strconv.Unquote(imp.Path.Value); err == nil && ruta == "musubi/internal/fleet" {
			if imp.Name != nil {
				return imp.Name.Name
			}
			return "fleet"
		}
	}
	return ""
}

// nombreDeLaLlamada da un nombre legible para lo que se llama.
func nombreDeLaLlamada(c *ast.CallExpr) string {
	switch fun := c.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if x, ok := fun.X.(*ast.Ident); ok {
			return x.Name + "." + fun.Sel.Name
		}
		return fun.Sel.Name
	}
	return fmt.Sprintf("una llamada (%T)", c.Fun)
}

// estaEnLaLista dice si la expresión está, por identidad, en la lista.
func estaEnLaLista(lista []ast.Expr, e ast.Expr) bool {
	for _, x := range lista {
		if x == e {
			return true
		}
	}
	return false
}
