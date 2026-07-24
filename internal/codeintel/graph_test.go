package codeintel

import "testing"

const modPath = "example.com/mod"

// Fixture: un paquete Go de dos archivos que ejercita todo lo derivable en F1.
//   - func Alpha llama a beta (same-package), a fmt.Println y a util.Help (cross-package).
//   - method (*Server).Alpha es homónima de la func Alpha ⇒ prueba la desambiguación por Key.
var fixtureFiles = map[string]string{
	"pkg/a.go": `package pkg

import (
	"fmt"
	"example.com/mod/internal/util"
)

func Alpha() {
	beta()
	fmt.Println("hi")
	util.Help()
}

func beta() {}
`,
	"pkg/b.go": `package pkg

type Server struct{}

func (s *Server) Alpha() {}

const Version = "1.0"

var Count int
`,
}

func findNode(g PackageGraph, key string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Key == key {
			return &g.Nodes[i]
		}
	}
	return nil
}

func hasEdge(g PackageGraph, from, to, kind string) bool {
	for _, e := range g.Edges {
		if e.FromKey == from && e.ToKey == to && e.Kind == kind {
			return true
		}
	}
	return false
}

func TestDerivePackage_Contains(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)

	for _, key := range []string{
		"pkg/a.go#func:Alpha",
		"pkg/a.go#func:beta",
		"pkg/b.go#type:Server",
		"pkg/b.go#const:Version",
		"pkg/b.go#var:Count",
	} {
		if findNode(g, key) == nil {
			t.Errorf("falta el nodo %q", key)
		}
	}

	// Cada símbolo lo CONTIENE su archivo.
	if !hasEdge(g, "pkg/a.go", "pkg/a.go#func:Alpha", EdgeContains) {
		t.Error("falta CONTAINS a.go → func:Alpha")
	}
	if !hasEdge(g, "pkg/b.go", "pkg/b.go#type:Server", EdgeContains) {
		t.Error("falta CONTAINS b.go → type:Server")
	}
}

func TestDerivePackage_MethodVsFuncDisambiguation(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)

	fn := findNode(g, "pkg/a.go#func:Alpha")
	me := findNode(g, "pkg/b.go#method:Server.Alpha")
	if fn == nil || me == nil {
		t.Fatalf("la func y el método homónimos deben ser nodos distintos: func=%v method=%v", fn, me)
	}
	if fn.Kind != KindFunc {
		t.Errorf("Alpha (func) tiene kind %q, esperaba %q", fn.Kind, KindFunc)
	}
	if me.Kind != KindMethod || me.Name != "Server.Alpha" {
		t.Errorf("Server.Alpha (method) mal derivado: kind=%q name=%q", me.Kind, me.Name)
	}
}

func TestDerivePackage_Imports(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)

	fmtNode := findNode(g, PackageKey("fmt"))
	if fmtNode == nil || !fmtNode.External {
		t.Errorf("fmt debe ser un nodo package externo: %v", fmtNode)
	}
	utilNode := findNode(g, PackageKey("example.com/mod/internal/util"))
	if utilNode == nil || utilNode.External {
		t.Errorf("el paquete in-module NO debe marcarse external: %v", utilNode)
	}

	if !hasEdge(g, "pkg/a.go", PackageKey("fmt"), EdgeImports) {
		t.Error("falta IMPORTS a.go → fmt")
	}
	if !hasEdge(g, "pkg/a.go", PackageKey("example.com/mod/internal/util"), EdgeImports) {
		t.Error("falta IMPORTS a.go → util")
	}
}

func TestDerivePackage_CallsIntraPackageOnly(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)

	// Alpha → beta es intra-paquete: DEBE existir con confianza 1.0.
	if !hasEdge(g, "pkg/a.go#func:Alpha", "pkg/a.go#func:beta", EdgeCalls) {
		t.Error("falta CALLS Alpha → beta")
	}
	for _, e := range g.Edges {
		if e.Kind == EdgeCalls && e.Confidence != 1.0 {
			t.Errorf("CALLS %s→%s con confianza %v, esperaba 1.0", e.FromKey, e.ToKey, e.Confidence)
		}
	}

	// Las llamadas cross-paquete (fmt.Println, util.Help) se DIFIEREN: no debe haber ninguna
	// arista CALLS hacia un nodo package.
	for _, e := range g.Edges {
		if e.Kind == EdgeCalls && (e.ToKey == PackageKey("fmt") || e.ToKey == PackageKey("example.com/mod/internal/util")) {
			t.Errorf("no debería haber CALLS cross-paquete: %s → %s", e.FromKey, e.ToKey)
		}
	}
}

func TestDerivePackage_Provenance(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)
	for _, e := range g.Edges {
		if e.Provenance != ProvExtracted {
			t.Errorf("arista %s con proveniencia %q, esperaba EXTRACTED", e.Kind, e.Provenance)
		}
	}
}

func TestDerivePackage_BrokenFileNoPanic(t *testing.T) {
	files := map[string]string{
		"pkg/wip.go": "package pkg\n\nfunc Broken( {\n", // sintaxis inválida (edición en curso)
		"pkg/ok.go":  "package pkg\n\nfunc Ok() {}\n",
	}
	// No debe entrar en pánico; el archivo sano sí debe aportar su símbolo.
	g := DerivePackage("pkg", files, modPath)
	if findNode(g, "pkg/ok.go#func:Ok") == nil {
		t.Error("el archivo sano debe derivarse aunque un hermano no compile")
	}
}

func TestDerivePackage_NonGoEmpty(t *testing.T) {
	files := map[string]string{
		"pkg/styles.css": "body { color: red; }",
		"pkg/app.ts":     "export function f() {}",
	}
	g := DerivePackage("pkg", files, modPath)
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Errorf("los archivos no-Go no deben emitir nodos/aristas en F1: %d nodos, %d aristas", len(g.Nodes), len(g.Edges))
	}
}

func TestExtractImports(t *testing.T) {
	content := `package pkg

import (
	"fmt"
	f "errors"
)
`
	imps := ExtractImports("x.go", content)
	if len(imps) != 2 {
		t.Fatalf("esperaba 2 imports, obtuve %d: %v", len(imps), imps)
	}
	if imps[0].Path != "fmt" || imps[0].Alias != "" {
		t.Errorf("import 0 mal: %+v", imps[0])
	}
	if imps[1].Path != "errors" || imps[1].Alias != "f" {
		t.Errorf("import 1 (con alias) mal: %+v", imps[1])
	}
	if ExtractImports("x.css", "body{}") != nil {
		t.Error("un archivo no-Go no debe devolver imports")
	}
}
