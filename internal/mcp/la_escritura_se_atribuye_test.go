package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTodaEscrituraConOrigenExplicitoLoResuelveOLoClava exige que el origen de una escritura no
// pueda venir de cualquier lado.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ CUSTODIA
//
// El backend marca con el sufijo `From` los métodos que reciben el tenant EXPLÍCITO: la fila se
// atribuye a lo que le pasen. Si quien lo pasa no lo derivó de la credencial, una fila puede
// quedar atribuida a otro tenant o sin atribuir — y una fila sin atribuir la ven TODOS. Eso es el
// write-poisoning cross-tenant que Track 17 dice cerrar, y lo cierra en 10 de 11 sitios con
// `writeOriginFor(principalFrom(ctx), …)`.
//
// EL SITIO 11 NO ES UN HUECO Y POR ESO LA GUARDA TIENE DOS RAMAS. `runDistillBatch` escribe con
// `distillScope`, una CONSTANTE del paquete —«es el tenant del acervo; se destila SÓLO ahí»— y su
// herramienta está cerrada con `isAdmin()`. Escribir a un tenant fijo a propósito es legítimo.
//
// LO QUE NO PUEDE PASAR es que «escribo a un tenant fijo» y «me olvidé de resolver» se vean igual,
// que es exactamente lo que pasaba antes de esta guarda: los dos son «no llamó a writeOriginFor».
// Por eso la segunda rama exige una CONSTANTE DEL PAQUETE y no cualquier variable: una constante
// no la puede mover el llamador. Aceptar «cualquier identificador» volvería la guarda un no-op —
// todos los sitios pasan un identificador.
//
// NO SE EXIGE LA PUERTA DE ADMIN EN LA MISMA FUNCIÓN, y vale decir por qué: `runDistillBatch` tiene
// DOS llamadores —la tool (con admin) y el scheduler (sin credencial)— así que exigirla ahí la
// pondría roja sobre un diseño correcto. Lo que hace segura a esa rama no es la puerta: es que el
// origen no dependa de quien llame.
//
// POR QUÉ SE DERIVA DEL AST Y NO SE ENUMERAN LOS SITIOS: enumerarlos no converge, y once que hoy
// están bien no dicen nada sobre el doce. Lo que se fija es la propiedad.
//
// Sabotaje que la hace fallar: en `runDistillBatch`, pasar `s.defaultScope()` en vez de la
// constante `distillScope` → el origen pasa a depender del estado del servidor y la guarda lo
// nombra.
// arnes: archivo="internal/mcp/methods_distill.go"
// arnes: de="s.engine.SaveObservationTypedFrom(distillScope, distillAuthor"
// arnes: a="s.engine.SaveObservationTypedFrom(s.defaultScope(), distillAuthor"
// arnes: prueba="TestTodaEscrituraConOrigenExplicitoLoResuelveOLoClava"
//
// Y LA OTRA DIRECCIÓN PRUEBA LO QUE HACE QUE LA SEGUNDA RAMA NO SEA UNA LISTA DISFRAZADA: escribir
// a OTRA constante del paquete tiene que seguir en verde. `distillScope = designCorpusScope`, así
// que pasar la de la derecha es el mismo tenant escrito de otra forma — legítimo. Si la guarda se
// pusiera roja acá estaría aceptando un NOMBRE en vez de la propiedad «el llamador no lo puede
// mover», que es enumerar con otro disfraz.
// arnes: arreglo_de="s.engine.SaveObservationTypedFrom(distillScope, distillAuthor"
// arnes: arreglo_a="s.engine.SaveObservationTypedFrom(designCorpusScope, distillAuthor"
func TestTodaEscrituraConOrigenExplicitoLoResuelveOLoClava(t *testing.T) {
	const resuelve = "writeOriginFor"

	fset := token.NewFileSet()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var archivos []*ast.File
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", n), nil, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v — no medí nada", n, err)
		}
		archivos = append(archivos, f)
	}

	// LAS CONSTANTES DEL PAQUETE, derivadas y no listadas: lo que las hace seguras es que el
	// llamador no las pueda mover, y eso lo decide el `const`, no su nombre.
	constantes := map[string]bool{}
	for _, f := range archivos {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, nombre := range vs.Names {
						constantes[nombre.Name] = true
					}
				}
			}
		}
	}
	if len(constantes) == 0 {
		t.Fatal("no encontré NI UNA constante de paquete: el barrido está roto y la segunda rama " +
			"de esta guarda aceptaría cualquier cosa")
	}

	sitios := 0
	for _, f := range archivos {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			derivaDeLaCredencial := false
			var escrituras []*ast.CallExpr
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == resuelve {
					derivaDeLaCredencial = true
					return true
				}
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if sel.Sel.Name == resuelve {
					derivaDeLaCredencial = true
					return true
				}
				// `s.engine.XFrom(...)`: el sufijo es la convención del backend para «recibe el
				// tenant explícito».
				if !strings.HasSuffix(sel.Sel.Name, "From") {
					return true
				}
				if x, ok := sel.X.(*ast.SelectorExpr); !ok || x.Sel.Name != "engine" {
					return true
				}
				escrituras = append(escrituras, c)
				return true
			})

			for _, c := range escrituras {
				sitios++
				if derivaDeLaCredencial {
					continue
				}
				metodo := c.Fun.(*ast.SelectorExpr).Sel.Name
				// SEGUNDA RAMA: un origen que el llamador no puede mover.
				if len(c.Args) > 0 {
					if id, ok := c.Args[0].(*ast.Ident); ok && constantes[id.Name] {
						continue
					}
				}
				t.Errorf("%s · %s llama a engine.%s() y el origen NO sale de %s() ni es una "+
					"constante del paquete.\n"+
					"  `*From` recibe el tenant EXPLÍCITO: la fila se atribuye a lo que le pases. Si\n"+
					"  eso no se deriva de la credencial, la escritura puede caer en otro tenant — o\n"+
					"  sin atribuir, y una fila sin atribuir la ven TODOS.\n"+
					"  Dos salidas, y las dos tienen que ser explícitas: resolvé el origen con\n"+
					"  `writeOriginFor(principalFrom(ctx), …)` y negate si devuelve !ok, O escribí a un\n"+
					"  tenant fijo pasando una CONSTANTE del paquete, como hace `runDistillBatch` con\n"+
					"  `distillScope`. Una variable no sirve para lo segundo: el llamador la mueve.",
					fset.Position(c.Pos()), fn.Name.Name, metodo, resuelve)
			}
		}
	}

	// EL CONTROL DE QUE SE MIRÓ ALGO: un verde de cero sitios se lee igual que un verde de todos.
	if sitios == 0 {
		t.Fatal("no encontré NI UNA escritura `engine.*From(` en el paquete: o el barrido está " +
			"roto o la convención del backend cambió de nombre. Esta guarda no estaría midiendo nada.")
	}
	t.Logf("%d escritura/s con origen explícito revisadas", sitios)
}
