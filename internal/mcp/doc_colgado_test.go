package mcp

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

// TestNingunComentarioDeDocQuedaColgadoDeUnaDeclaracionAjena — la forma de nacer del comentario robado.
//
// EL DEFECTO. Se inserta una declaración X entre el comentario de doc de Y y la propia Y. Go pega
// el comentario a lo que tenga inmediatamente debajo, así que el doc de Y pasa a documentar a X, y
// Y queda sin nada. Compila, `go vet` calla y el lint también: ST1020 sólo mira funciones
// EXPORTADAS, y casi todo este árbol es no exportado.
//
// MEDIDO EL 2026-09-24: había TREINTA Y CUATRO. Entre ellos `toolEntry` —la unidad central del
// registro de tools— con su doc colgado de `roClass`; `ParsearTempMiligrados`, exportada, con el
// suyo colgado de dos constantes; y `startOutboxDrain`, cuyo párrafo había quedado repartido en
// TRES lugares por dos inserciones sucesivas. Dos de los robados eran docs de pruebas con anclas
// del arnés, y alguien ya se había topado con eso: les agregó `prueba="…"` a mano, que arregla el
// síntoma (el arnés sabe de qué prueba es el sabotaje) y deja viva la causa. Y la memoria del
// proyecto lo tenía anotado CINCO veces como lección, lo que dice lo que vale una lección que no
// tiene guarda.
//
// LA PREGUNTA ES SOBRE LA FORMA DE NACER, NO SOBRE LOS CASOS. Enumerar los 34 tapaba esos 34; esta
// guarda pregunta por la firma que deja CUALQUIER inserción de ese tipo: el comentario de X
// arranca con el nombre de otra declaración Y del paquete, e Y no tiene comentario propio.
//
// POR QUÉ LA SEGUNDA CONDICIÓN, Y NO SÓLO LA PRIMERA. Sin ella acusaba 57, y 23 eran inocentes:
// docs de pruebas que arrancan nombrando lo que prueban («`Validar` rechaza…» sobre
// `TestUnaPoliticaMalEscrita…`). Ésos no roban nada, porque `Validar` tiene su propio doc. El
// robo deja a Y sin comentario; la mención no. A una regla que se aprieta se la mide por a quién
// acusa, y ésta, con las dos condiciones, acusó a 34 y los 34 eran de verdad.
//
// LO QUE ESTA GUARDA NO VE, dicho para que nadie le crea de más: una COLA robada. Si la inserción
// cae a mitad de un párrafo, el pedazo de abajo queda pegado a la declaración siguiente y NO
// arranca con ningún nombre —así se encontró el resto de `startOutboxDrain`, a la cabeza del doc
// de `reconcileOutboxOnStartup`—. Detectarlo pediría adivinar dónde termina una idea, y eso no
// converge.
//
// Sabotaje que la hace fallar: reponer el robo que la motivó —meter una declaración entre el
// comentario de `sharpenToolEntry` y la función—.
// arnes: archivo="internal/mcp/methods_dedup.go"
// arnes: de="func (s *McpServer) sharpenToolEntry() toolEntry {"
// arnes: a="var sabotajeRobaElComentario = 0\n\nfunc (s *McpServer) sharpenToolEntry() toolEntry {"
func TestNingunComentarioDeDocQuedaColgadoDeUnaDeclaracionAjena(t *testing.T) {
	raiz := filepath.Join("..", "..")
	gos, err := arbol.ConSufijo(raiz, ".go")
	if err != nil {
		t.Fatal(err)
	}

	type clave struct{ dir, paquete string }
	type archivo struct {
		rel string
		f   *ast.File
	}
	fset := token.NewFileSet()
	porPaquete := map[clave][]archivo{}
	for _, rel := range gos {
		f, err := parser.ParseFile(fset, filepath.Join(raiz, rel), nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v", rel, err)
		}
		k := clave{filepath.Dir(rel), f.Name.Name}
		porPaquete[k] = append(porPaquete[k], archivo{rel, f})
	}
	// EL CONTROL DE QUE MIRÓ ALGO. Una guarda que no parseó nada contesta «ningún robo» igual que
	// una que miró el árbol entero.
	if len(porPaquete) < 20 {
		t.Fatalf("sólo se armaron %d paquetes: el enumerador no está mirando el árbol", len(porPaquete))
	}

	var colgados []string
	for _, archivos := range porPaquete {
		nombres := map[string]bool{}
		conDocPropio := map[string]bool{}
		marcar := func(n string, docs ...*ast.CommentGroup) {
			nombres[n] = true
			for _, d := range docs {
				if d != nil && primeraPalabraDeDoc(d) == n {
					conDocPropio[n] = true
				}
			}
		}
		for _, a := range archivos {
			for _, d := range a.f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					marcar(d.Name.Name, d.Doc)
				case *ast.GenDecl:
					for _, sp := range d.Specs {
						switch sp := sp.(type) {
						case *ast.TypeSpec:
							marcar(sp.Name.Name, sp.Doc, d.Doc)
						case *ast.ValueSpec:
							for _, n := range sp.Names {
								marcar(n.Name, sp.Doc, d.Doc)
							}
						}
					}
				}
			}
		}
		revisar := func(rel string, doc *ast.CommentGroup, propios []string) {
			if doc == nil {
				return
			}
			y := primeraPalabraDeDoc(doc)
			if y == "" || !nombres[y] || conDocPropio[y] {
				return
			}
			for _, n := range propios {
				if n == y {
					return
				}
			}
			colgados = append(colgados, rel+":"+strconv.Itoa(fset.Position(doc.Pos()).Line)+
				"  el comentario de "+strings.Join(propios, ", ")+" arranca con «"+y+"», y "+y+" quedó sin comentario")
		}
		for _, a := range archivos {
			for _, d := range a.f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					revisar(a.rel, d.Doc, []string{d.Name.Name})
				case *ast.GenDecl:
					var todos []string
					for _, sp := range d.Specs {
						switch sp := sp.(type) {
						case *ast.TypeSpec:
							todos = append(todos, sp.Name.Name)
							revisar(a.rel, sp.Doc, []string{sp.Name.Name})
						case *ast.ValueSpec:
							var ns []string
							for _, n := range sp.Names {
								ns = append(ns, n.Name)
							}
							todos = append(todos, ns...)
							revisar(a.rel, sp.Doc, ns)
						}
					}
					revisar(a.rel, d.Doc, todos)
				}
			}
		}
	}
	if len(colgados) > 0 {
		sort.Strings(colgados)
		t.Fatalf("%d comentario/s de doc quedaron colgados de una declaración ajena. Casi siempre es que "+
			"alguien insertó algo ENTRE el comentario de una declaración y la declaración:\n  %s\n"+
			"Arreglo: subí la declaración insertada por encima del comentario, que vuelva a quedar pegado "+
			"a su dueño. Si el nombre del comentario es viejo (un renombre), cambiale la primera palabra.",
			len(colgados), strings.Join(colgados, "\n  "))
	}
}

// primeraPalabraDeDoc es la primera palabra de un comentario de doc, sin la puntuación que la
// sigue: el nombre que el comentario dice documentar.
func primeraPalabraDeDoc(doc *ast.CommentGroup) string {
	campos := strings.Fields(doc.Text())
	if len(campos) == 0 {
		return ""
	}
	return strings.TrimRight(campos[0], ".,:;")
}
