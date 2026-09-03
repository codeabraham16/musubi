package mcp

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// codegraph_miss_explicado_test.go custodia que un MISS del grafo de código diga POR QUÉ.
//
// EL DEFECTO QUE ORIGINA ESTO, medido el 2026-09-03: `musubi_code_graph` devolvía
// `{"found": false}` y nada más. Pero el grafo indexa el árbol CHECKOUTEADO, así que un símbolo de
// otra rama no está —y el grafo está sano—. En este repo eran 95 commits y 334 archivos de
// `feat/control-de-flota` invisibles desde `main`: preguntar por `SSHFalsoParaTest` daba
// `found:false` con el grafo perfecto. Un miss mudo se lee como «la herramienta no sabe», empuja de
// vuelta a `grep`, y así es como una herramienta correcta se gana la desconfianza.
//
// Los cuatro casos que siguen son las cuatro causas REALES de un miss, y cada uno pide su pista.

// proyectoIndexado arma un repo mínimo con un paquete indexado y devuelve el server.
func proyectoIndexado(t *testing.T) (*McpServer, string) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	s := newTestServerWithPath(t, dir)
	mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{})
	return s, dir
}

// G1 — el archivo está indexado pero el símbolo no existe: la pista es el nombre, y se listan los
// símbolos que el archivo SÍ tiene para que el siguiente intento no sea otra adivinanza.
func TestMissEnArchivoIndexadoListaLosSimbolosQueSiEstan(t *testing.T) {
	s, _ := proyectoIndexado(t)

	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
		map[string]interface{}{"symbol": "pkg/a.go#func:NoExiste"}))

	if cg["found"] != false {
		t.Fatalf("NoExiste no debería encontrarse: %v", cg)
	}
	hint, _ := cg["hint"].(string)
	if hint == "" {
		t.Fatal("un miss sin hint es el defecto que este test custodia")
	}
	// La pista tiene que mandar al NOMBRE, no a re-indexar ni a otra rama: el archivo está.
	if !containsInAny(cg["symbols_in_file"], "Alpha") || !containsInAny(cg["symbols_in_file"], "beta") {
		t.Errorf("debería listar los símbolos del archivo (Alpha, beta), obtuve %v", cg["symbols_in_file"])
	}
}

// G2 — el archivo existe en disco pero NO está en el grafo: la pista es indexar. Se distingue de G3
// porque el remedio es distinto, y una pista que no distingue manda a buscar donde no está.
func TestMissDeArchivoSinIndexarMandaAIndexar(t *testing.T) {
	s, dir := proyectoIndexado(t)
	// Un archivo nuevo, en disco, todavía fuera del índice.
	writeFile(t, filepath.Join(dir, "pkg", "nuevo.go"), "package pkg\n\nfunc Gamma() {}\n")

	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
		map[string]interface{}{"symbol": "pkg/nuevo.go#func:Gamma"}))

	hint, _ := cg["hint"].(string)
	if hint == "" {
		t.Fatal("un miss sin hint es el defecto que este test custodia")
	}
	if !contieneAlguna(hint, "codegraph_index", "indexar") {
		t.Errorf("un archivo en disco fuera del grafo debe mandar a INDEXAR, obtuve %q", hint)
	}
	// Y NO debe hablar de otra rama: el archivo está acá.
	if contieneAlguna(hint, "otra rama", "CHECKOUTEADA") {
		t.Errorf("el archivo existe en disco: la pista no debe culpar a otra rama, obtuve %q", hint)
	}
}

// G3 — el archivo no existe en el árbol: la pista nombra la causa real, que es la rama. Es el caso
// que originó todo esto y el único donde el miss es CORRECTO y hay que decirlo.
func TestMissDeOtraRamaLoDiceEnVezDeCallar(t *testing.T) {
	s, _ := proyectoIndexado(t)

	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
		map[string]interface{}{"symbol": "internal/fleet/remoto.go#func:SSHFalsoParaTest"}))

	hint, _ := cg["hint"].(string)
	if hint == "" {
		t.Fatal("un miss sin hint es el defecto que este test custodia")
	}
	if !contieneAlguna(hint, "rama", "CHECKOUTEADA") {
		t.Errorf("un archivo ausente del árbol debe explicar que el grafo indexa la rama checkouteada, obtuve %q", hint)
	}
}

// G4 — el node_key mal formado no es «no existe»: es un error de sintaxis del que pregunta, y
// mandarlo a indexar o a otra rama lo haría perder el tiempo en el lugar equivocado.
func TestMissConNodeKeySinFormatoEnsenaElFormato(t *testing.T) {
	s, _ := proyectoIndexado(t)

	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
		map[string]interface{}{"symbol": "Alpha"}))

	hint, _ := cg["hint"].(string)
	if !contieneAlguna(hint, "path#kind:name") {
		t.Errorf("un node_key sin '#' debe enseñar el formato, obtuve %q", hint)
	}
}

// G5 — DE QUÉ ÁRBOL ES EL ÍNDICE. Sin esto la pista de G3 es una hipótesis sin evidencia: el que
// pregunta no puede comparar contra su propio HEAD. Se registra al INDEXAR (no al consultar) para
// que describa el árbol del que salió el grafo y no el de hoy.
func TestElIndiceRegistraDeQueCommitEs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() {}\n")
	s := newTestServerWithPath(t, dir)

	// Este repo temporal NO es un repo git: el registro es best-effort y no debe romper el índice.
	idx := decodeCG(t, mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{}))
	if n, _ := idx["nodes"].(float64); n <= 0 {
		t.Fatalf("sin git el índice tiene que funcionar igual, got %v", idx)
	}
	if _, hay := idx["indexed_head"]; hay {
		t.Errorf("fuera de un repo git no hay commit que declarar, obtuve %v", idx["indexed_head"])
	}

	// Y dentro de un repo git sí viaja, y el miss lo repite para que se pueda comparar.
	if !hayGit() {
		t.Skip("sin git en el PATH no se puede verificar la otra mitad")
	}
	gitInit(t, dir)
	idx2 := decodeCG(t, mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{}))
	head, _ := idx2["indexed_head"].(string)
	if head == "" {
		t.Fatalf("dentro de un repo git el índice debe declarar su commit, obtuve %v", idx2)
	}
	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
		map[string]interface{}{"symbol": "otro/x.go#func:Nada"}))
	if cg["indexed_head"] != head {
		t.Errorf("el miss debe repetir el commit del índice (%q) para poder compararlo, obtuve %v", head, cg["indexed_head"])
	}
}

// G6 — el modo ARCHIVO también explica. Devolvía dos listas vacías, que se lee igual que «este
// archivo no tiene símbolos» cuando la verdad puede ser que no esté indexado.
func TestArchivoSinSimbolosTambienExplica(t *testing.T) {
	s, _ := proyectoIndexado(t)

	cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
		map[string]interface{}{"path": "pkg/fantasma.go"}))

	if hint, _ := cg["hint"].(string); hint == "" {
		t.Errorf("un archivo sin símbolos debe decir por qué, obtuve %v", cg)
	}
	if cg["path"] == nil {
		t.Errorf("la respuesta del modo archivo debe conservar 'path', obtuve %v", cg)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────────────────────────

func contieneAlguna(s string, quiere ...string) bool {
	for _, q := range quiere {
		if q != "" && strings.Contains(s, q) {
			return true
		}
	}
	return false
}

func hayGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// gitInit convierte el dir temporal en un repo con UN commit, que es lo mínimo para que
// `git rev-parse HEAD` devuelva algo.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	pasos := [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"-c", "commit.gpgsign=false", "commit", "-qm", "inicial"},
	}
	for _, p := range pasos {
		cmd := exec.Command("git", append([]string{"-C", dir}, p...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("no se pudo preparar el repo git de prueba (%v): %s", err, out)
		}
	}
}
