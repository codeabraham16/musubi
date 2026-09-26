package fleettest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"musubi/internal/arbol"
)

// Grafo es el grafo de llamadas de UN paquete, leído del fuente de producción (sin `_test.go`) y sin
// tipos: quién nombra a quién.
//
// EXISTE PARA QUE UNA GUARDA PREGUNTE POR EL INVARIANTE Y NO POR LA FORMA DE LA LLAMADA. La primera
// versión de TestElRelojDeLaColaSeCuentaEnUnSoloLugar exigía que cada lector llamara DIRECTAMENTE a
// fleet.LimiteDeVida, y la segunda revisión de A131 (tema T9) la midió castigando un refactor
// correcto: extraer `textoDelLimiteDeVida` en memory y llamarlo desde los dos lectores. Lo que la
// guarda quería saber era si el lector LLEGA a la cuenta única, no si la escribe en su propio cuerpo.
// Con el grafo, «llega» se sigue por los helpers del paquete, que es lo que un refactor mueve.
//
// Las claves son `Nombre` para las funciones y `Tipo.Nombre` para los métodos (el tipo sin `*` ni
// parámetros de tipo). Una referencia a otro paquete se guarda como `ruta/del/import.Nombre`.
//
// CUENTA LO QUE SE NOMBRA, NO SÓLO LO QUE SE LLAMA: pasar `fleet.LimiteDeVida` como valor también es
// llegar a ella. Y resuelve sin tipos, así que APROXIMA POR ARRIBA en un caso, dicho: `x.M()` sobre un
// valor que no es ni un paquete importado ni el receptor del método se enlaza con TODO método `M` del
// paquete. Eso puede sumar un camino que no existe; nunca quita uno que sí. Quien lo use para exigir
// que algo NO llegue tiene que saberlo.
type Grafo struct {
	// Dir es el directorio del paquete, relativo a la raíz del repo (`internal/memory`).
	Dir     string
	nodos   map[string]*nodo
	metodos map[string][]string // nombre del método → sus claves `Tipo.Nombre`
}

type nodo struct {
	archivo  string
	linea    int
	locales  map[string]bool // claves de funciones y métodos del mismo paquete
	externas map[string]bool // `ruta.Nombre` de otros paquetes
	// porNombre son los `x.M` que no se pudieron atar a un receptor ni a un import: se resuelven
	// contra todos los métodos `M` del paquete cuando el grafo termina de leerse.
	porNombre map[string]bool
}

// LeerGrafo arma el grafo del paquete que vive en `dir` (relativo a `raiz`), con los `.go` que git
// trackea ahí —no los del disco: una guarda que barre el disco mide otra cosa en cada checkout—, sin
// los `_test.go` y sin bajar a subdirectorios, que son otros paquetes.
//
// FALLA SI NO LEE NADA. Un grafo vacío contestaría «no llega» para todo, y eso se leería como un
// hallazgo cuando es que no se miró.
func LeerGrafo(raiz, dir string) (*Grafo, error) {
	gos, err := arbol.ConSufijo(raiz, ".go")
	if err != nil {
		return nil, err
	}
	g := &Grafo{Dir: dir, nodos: map[string]*nodo{}, metodos: map[string][]string{}}
	fset := token.NewFileSet()
	var archivos []*ast.File
	var rutas []string
	for _, rel := range gos {
		if path.Dir(rel) != dir || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(raiz, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("no pude parsear %s: %w", rel, err)
		}
		archivos = append(archivos, f)
		rutas = append(rutas, rel)
	}
	if len(archivos) == 0 {
		return nil, fmt.Errorf("git no trackea NI UN .go de producción en %s: el grafo quedaría vacío y todo «no llega»", dir)
	}

	// Primera pasada: qué funciones y métodos declara el paquete. Hace falta ANTES de leer los
	// cuerpos, porque un cuerpo puede nombrar una función declarada en un archivo posterior.
	funciones := map[string]bool{}
	for _, f := range archivos {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			clave := claveDe(fn)
			if fn.Recv == nil {
				funciones[fn.Name.Name] = true
			} else if !contiene(g.metodos[fn.Name.Name], clave) {
				g.metodos[fn.Name.Name] = append(g.metodos[fn.Name.Name], clave)
			}
		}
	}

	for i, f := range archivos {
		imports := importsDe(f)
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			clave := claveDe(fn)
			// Un mismo nombre puede declararse en dos archivos con build tags distintos
			// (`_linux.go`, `_windows.go`): se juntan, que es lo que la unión de plataformas es.
			n := g.nodos[clave]
			if n == nil {
				n = &nodo{archivo: rutas[i], linea: fset.Position(fn.Pos()).Line,
					locales: map[string]bool{}, externas: map[string]bool{}, porNombre: map[string]bool{}}
				g.nodos[clave] = n
			}
			if fn.Body == nil {
				continue
			}
			receptor, tipo := "", ""
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				tipo = tipoBase(fn.Recv.List[0].Type)
				if len(fn.Recv.List[0].Names) > 0 {
					receptor = fn.Recv.List[0].Names[0].Name
				}
			}
			var mirar func(ast.Node) bool
			mirar = func(nd ast.Node) bool {
				switch x := nd.(type) {
				case *ast.SelectorExpr:
					if id, ok := x.X.(*ast.Ident); ok {
						switch {
						case imports[id.Name] != "":
							n.externas[imports[id.Name]+"."+x.Sel.Name] = true
						case id.Name == receptor && tipo != "":
							if contiene(g.metodos[x.Sel.Name], tipo+"."+x.Sel.Name) {
								n.locales[tipo+"."+x.Sel.Name] = true
							}
						default:
							n.porNombre[x.Sel.Name] = true
						}
					} else {
						n.porNombre[x.Sel.Name] = true
						ast.Inspect(x.X, mirar)
					}
					// El `Sel` no es un nombre suelto: no se baja a él.
					return false
				case *ast.Ident:
					if funciones[x.Name] {
						n.locales[x.Name] = true
					}
				}
				return true
			}
			ast.Inspect(fn.Body, mirar)
		}
	}
	for _, n := range g.nodos {
		for nombre := range n.porNombre {
			for _, m := range g.metodos[nombre] {
				n.locales[m] = true
			}
		}
	}
	return g, nil
}

// Existe dice si el paquete declara esa función o ese método (`Nombre` o `Tipo.Nombre`).
func (g *Grafo) Existe(clave string) bool {
	_, ok := g.nodos[clave]
	return ok
}

// Donde devuelve `archivo:línea` de la declaración, o "" si no existe.
func (g *Grafo) Donde(clave string) string {
	n := g.nodos[clave]
	if n == nil {
		return ""
	}
	return n.archivo + ":" + strconv.Itoa(n.linea)
}

// Claves devuelve, ordenadas, todas las funciones y métodos que el paquete declara.
func (g *Grafo) Claves() []string {
	out := make([]string, 0, len(g.nodos))
	for k := range g.nodos {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Camino devuelve la cadena de llamadas por la que `desde` llega a `objetivo`, o nil si no llega.
//
// `objetivo` es una clave local (`escanearComando`, `LimiteDeVida`) o una referencia a otro paquete
// (`musubi/internal/fleet.LimiteDeVida`). El camino empieza en `desde` y termina en `objetivo`; si
// `desde` es el objetivo mismo, es de largo uno. Recorre a lo ancho, así que el camino es el más corto.
func (g *Grafo) Camino(desde, objetivo string) []string {
	if _, ok := g.nodos[desde]; !ok {
		return nil
	}
	previo := map[string]string{desde: ""}
	cola := []string{desde}
	armar := func(fin string) []string {
		var c []string
		for k := fin; k != ""; k = previo[k] {
			c = append([]string{k}, c...)
		}
		return c
	}
	for len(cola) > 0 {
		k := cola[0]
		cola = cola[1:]
		if k == objetivo {
			return armar(k)
		}
		n := g.nodos[k]
		if n == nil {
			continue
		}
		if n.externas[objetivo] {
			return append(armar(k), objetivo)
		}
		vecinos := make([]string, 0, len(n.locales))
		for v := range n.locales {
			vecinos = append(vecinos, v)
		}
		sort.Strings(vecinos)
		for _, v := range vecinos {
			if _, visto := previo[v]; !visto {
				previo[v] = k
				cola = append(cola, v)
			}
		}
	}
	return nil
}

// Exportada dice si una clave es alcanzable desde otro paquete: el nombre exportado y, si es un
// método, el tipo también.
func Exportada(clave string) bool {
	tipo, nombre, esMetodo := strings.Cut(clave, ".")
	if !esMetodo {
		return ast.IsExported(tipo)
	}
	return ast.IsExported(tipo) && ast.IsExported(nombre)
}

func claveDe(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return tipoBase(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

// tipoBase saca el `*` y los parámetros de tipo de un receptor: `*Caja[T]` → `Caja`.
func tipoBase(e ast.Expr) string {
	for {
		switch x := e.(type) {
		case *ast.StarExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.Ident:
			return x.Name
		default:
			return ""
		}
	}
}

// importsDe devuelve nombre local → ruta de cada import del archivo. Sin alias, el nombre es el
// último tramo de la ruta, que es lo que el repo usa siempre; un import con `_` o `.` no nombra nada.
func importsDe(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, im := range f.Imports {
		ruta, err := strconv.Unquote(im.Path.Value)
		if err != nil {
			continue
		}
		nombre := path.Base(ruta)
		if im.Name != nil {
			nombre = im.Name.Name
		}
		if nombre == "_" || nombre == "." {
			continue
		}
		out[nombre] = ruta
	}
	return out
}

func contiene(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
