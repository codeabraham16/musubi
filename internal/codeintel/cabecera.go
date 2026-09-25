package codeintel

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// cabecera.go saca, sin modelo, un RESUMEN de un archivo Go a partir de su comentario de cabecera:
// el que escribió el desarrollador para decir qué es el archivo. Es la única fuente de «resumen»
// que se puede regenerar sola, y la tienen 289 de los 362 .go de producción de Musubi (medido el
// 2026-09-24). El índice incremental del grafo la usa para mantener al día los gists de
// code_memory sin depender de que un agente se acuerde de llamar a musubi_save_code — medido: 6
// veces en 244.480 invocaciones.

// VersionResumenDeCabecera identifica la REGLA con la que se sacan los resúmenes automáticos. Si
// cambia la regla (qué comentario cuenta, el tope, qué archivos reciben resumen), hay que subirla:
// el índice del grafo la compara contra el sello guardado en la base y, si difiere, barre el árbol
// entero UNA vez. Sin eso, un archivo que no cambió conservaría para siempre el texto de la regla
// vieja, porque la huella del contenido sigue al archivo y no a quien lo resume — el mismo modo de
// falla que GraphDeriverVersion vino a cerrar para el grafo.
const VersionResumenDeCabecera = "1-cabecera-sin-tests"

// topeResumenDeCabecera acota el resumen en runas: es un titular, no el comentario entero.
const topeResumenDeCabecera = 300

// ResumenDeCabecera devuelve el primer párrafo del comentario de cabecera de un archivo Go, en una
// sola línea y con tope, o "" si el archivo no recibe resumen automático.
//
// QUÉ COMENTARIO CUENTA, en orden: el doc del paquete (el pegado al `package`), y si no hay, el
// primer grupo de comentarios LIBRE entre la cláusula `package` (o los imports) y la primera
// declaración. «Libre» quiere decir que no es el doc de ninguna declaración: el comentario pegado
// a la primera función es el doc de ESA función, no del archivo, y tomarlo haría pasar el porqué
// de una función por el resumen del archivo entero. Es la forma que usa buena parte del repo
// (p. ej. cmd/musubi/capture.go): el comentario del archivo va después de los imports.
//
// QUÉ SE DESCARTA: las directivas (`//go:build`, `// +build`, `//nolint`), el aviso de código
// generado y los comentarios de adentro del bloque de imports.
//
// QUÉ NO RECIBE RESUMEN: lo que no es Go, y los `_test.go`. Los tests son ~280 archivos con
// cabecera en Musubi; darles gist a todos inundaría code_memory —y el push al central— con
// resúmenes de pruebas que nadie consulta antes de leer. Es una decisión, y por eso forma parte
// de VersionResumenDeCabecera: revertirla re-siembra.
func ResumenDeCabecera(path, content string) string {
	base := filepath.Base(path)
	if strings.ToLower(filepath.Ext(base)) != ".go" || strings.HasSuffix(base, "_test.go") {
		return ""
	}
	return CabeceraDe(path, content)
}

// CabeceraDe es la extracción de ResumenDeCabecera SIN la política de qué archivos reciben gist
// automático: devuelve la cabecera de cualquier .go, tests incluidos. La usa la vista de un gist de
// agente, que muestra al lado la cabecera al día del archivo — ahí no hay nada que inundar.
func CabeceraDe(path, content string) string {
	if strings.ToLower(filepath.Ext(path)) != ".go" {
		return ""
	}
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, "", content, parser.ParseComments|parser.SkipObjectResolution)
	if f == nil || f.Name == nil {
		return ""
	}
	if f.Doc != nil {
		if r := primerParrafo(f.Doc.Text()); r != "" {
			return r
		}
	}
	return primerParrafo(comentarioLibreDeCabecera(f))
}

// comentarioLibreDeCabecera busca el primer grupo de comentarios que no es de ninguna declaración
// y que está entre el `package` y la primera declaración que no es un import. "" si no hay.
func comentarioLibreDeCabecera(f *ast.File) string {
	deDeclaracion := map[*ast.CommentGroup]bool{}
	marcar := func(cgs ...*ast.CommentGroup) {
		for _, cg := range cgs {
			if cg != nil {
				deDeclaracion[cg] = true
			}
		}
	}
	limite := token.NoPos // dónde empieza la primera declaración que no es import
	type rango struct{ desde, hasta token.Pos }
	var imports []rango
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			marcar(d.Doc)
		case *ast.GenDecl:
			marcar(d.Doc)
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ImportSpec:
					marcar(s.Doc, s.Comment)
				case *ast.ValueSpec:
					marcar(s.Doc, s.Comment)
				case *ast.TypeSpec:
					marcar(s.Doc, s.Comment)
				}
			}
			if d.Tok == token.IMPORT {
				imports = append(imports, rango{d.Pos(), d.End()})
				continue
			}
		}
		if limite == token.NoPos {
			limite = decl.Pos()
		}
	}
	for _, cg := range f.Comments {
		if cg.Pos() < f.Name.End() {
			continue // antes del `package`: licencias y directivas de build, no el porqué del archivo
		}
		if limite != token.NoPos && cg.End() > limite {
			break
		}
		if deDeclaracion[cg] {
			continue
		}
		adentro := false
		for _, r := range imports {
			if cg.Pos() > r.desde && cg.End() < r.hasta {
				adentro = true
				break
			}
		}
		if adentro {
			continue
		}
		if txt := cg.Text(); primerParrafo(txt) != "" {
			return txt
		}
	}
	return ""
}

// primerParrafo toma el primer párrafo con texto de un comentario ya limpio (CommentGroup.Text),
// saltea las directivas que Text() no filtra y el aviso de código generado, lo junta en una línea
// y lo acota a topeResumenDeCabecera runas.
func primerParrafo(txt string) string {
	for _, parrafo := range strings.Split(txt, "\n\n") {
		var lineas []string
		for _, l := range strings.Split(parrafo, "\n") {
			l = strings.TrimSpace(l)
			if l == "" || esDirectivaDeComentario(l) {
				continue
			}
			lineas = append(lineas, l)
		}
		if len(lineas) == 0 {
			continue
		}
		r := strings.Join(strings.Fields(strings.Join(lineas, " ")), " ")
		if strings.HasPrefix(r, "Code generated") {
			continue
		}
		if utf8.RuneCountInString(r) > topeResumenDeCabecera {
			r = string([]rune(r)[:topeResumenDeCabecera-1]) + "…"
		}
		return r
	}
	return ""
}

// esDirectivaDeComentario reconoce las directivas que CommentGroup.Text() deja pasar porque llevan
// un espacio después de las barras (`// +build`) o no llevan dos puntos (`//nolint`).
func esDirectivaDeComentario(l string) bool {
	for _, p := range []string{"+build", "go:build", "nolint", "go:generate"} {
		if strings.HasPrefix(l, p) {
			return true
		}
	}
	return false
}
