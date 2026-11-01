package mcp

// Tests de BORDE de las 7 tools del grafo de specs (spec-graph F1): se despachan
// por handleToolsCall —el MISMO camino que un cliente MCP, con la clasificación
// read/write y el candado del dispatch— y se afirma sobre la salida SERIALIZADA.
// El core (GrepSpecs, ResolveSpecPath, BuildGraph) ya tiene su batería en
// internal/specgraph; lo que se fija ACÁ son los invariantes que la spec declara
// sobre la TOOL y cuyo sitio de declaración es este handler, no el core:
//   - R12: musubi_spec_grep devuelve la lista aunque esté vacía como `[]`, no `null`.
//   - R15: musubi_spec_create FALLA si el id ya está en uso.
//   - R14/#6: musubi_spec_graph serializa el DTO con claves en minúscula, no los
//     nombres de campo Go en PascalCase (un cliente que parsea en minúscula leería
//     un objeto vacío).
//   - R15(b): musubi_spec_create rechaza un id que YAML no vuelve a leer igual
//     (p. ej. "1.0" → float 1): la identidad en disco dejaría de ser la pedida.
// Cada ancla «Sabotaje» lleva su directiva `// arnes:` mecanizada (archivo/de/a)
// colgada debajo, así el censo del arnés (internal/mcp/arnes_censo_test.go) la
// cuenta como mecanizada y no como deuda en prosa. Los cuatro sabotajes son una
// sustitución de texto que compila: R12 cambia el make por un slice nil, R15
// desarma la compuerta del id duplicado, R14/#6 devuelve el struct crudo en vez
// del DTO, y R15(b) anula la comparación del round-trip. storewindows_test.go no
// entra en el censo porque su prosa no arranca con «Sabotaje» (no es un ancla).

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// specCallText extrae el texto de una respuesta de tool (CallToolResponse). Falla
// el test si la forma no es la esperada: una tool de spec que no devuelve texto es
// un bug de contrato, no un caso a tolerar.
func specCallText(t *testing.T, res interface{}) string {
	t.Helper()
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("respuesta inesperada de una tool de spec: %+v", res)
	}
	return resp.Content[0].Text
}

// TestSpecGrepHandlerSinMatchesSerializaListaNoNull fija R12 en su sitio de
// declaración: el handler arma la lista con make([]string,0) (methods_spec.go), así
// que un grep sin resultados serializa `"matches":[]`. El contrato de algo que
// lista es devolver una lista aunque esté vacía (go-hygiene #6): un slice nil
// serializa a `null`, y para el cliente MCP `[]` y `null` no son lo mismo.
//
// Sabotaje que lo pone en ROJO: cambiar en toolSpecGrep el
// `matches := make([]string, 0, len(result.Matches))` por `var matches []string`
// (nil ⇒ `"matches":null`). El core no lo cubre: GrepSpecs devuelve su propio
// slice; esta lista de strings formateados es del handler.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="matches := make([]string, 0, len(result.Matches))"
// arnes: a="var matches []string"
func TestSpecGrepHandlerSinMatchesSerializaListaNoNull(t *testing.T) {
	// Repo sin specs: el caso más limpio de "lista vacía". El patrón es irrelevante
	// (no hay nada que recorrer), pero es obligatorio, así que va uno cualquiera.
	s := newTestServerWithPath(t, t.TempDir())
	res, e := call(t, s, "musubi_spec_grep", map[string]interface{}{
		"pattern": "zzz-no-aparece-en-ninguna-spec-zzz",
	})
	if e != nil {
		t.Fatalf("grep sin matches no debía dar error: %+v", e)
	}
	text := specCallText(t, res)
	if !strings.Contains(text, `"matches":[]`) {
		t.Errorf("grep vacío debe serializar \"matches\":[] (no null); salida: %s", text)
	}
	if strings.Contains(text, `"matches":null`) {
		t.Errorf("grep serializó la lista vacía como null (contrato R12 roto): %s", text)
	}
}

// TestSpecCreateHandlerRechazaIdDuplicado fija R15: create es EXCLUSIVO, falla si
// el id ya está en uso. Dos specs con el mismo id colisionan en el grafo (gana la
// primera del walk) y rompen la navegación, así que la compuerta tiene que frenar
// el segundo create ANTES de escribir un archivo.
//
// Sabotaje que lo pone en ROJO: desarmar en toolSpecCreate la compuerta
// `if existing, ok := s.specIndex.PathForID(args.ID); ok {` (p. ej. `ok && false`):
// el segundo create escribiría specs/b/SPEC.md con el id ya en uso y devolvería
// éxito, con lo que este test fallaría en el "debía fallar". El re-scan del Index
// en PathForID es lo que hace que el segundo create VEA al primero sin reindexar.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="if existing, ok := s.specIndex.PathForID(args.ID); ok {"
// arnes: a="if existing, ok := s.specIndex.PathForID(args.ID); ok && false {"
func TestSpecCreateHandlerRechazaIdDuplicado(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/a/SPEC.md", "id": "dup-id", "type": "goal-and-requirements", "title": "A",
	}); e != nil {
		t.Fatalf("el primer create no debía fallar: %+v", e)
	}
	// Mismo id, OTRA ruta (así el que frena es la compuerta de id, no la de archivo existente).
	_, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/b/SPEC.md", "id": "dup-id", "type": "goal-and-requirements", "title": "B",
	})
	if e == nil {
		t.Fatal("un create con un id ya en uso debía fallar, pero pasó")
	}
	if !strings.Contains(e.Message, "en uso") {
		t.Errorf("el error debía señalar el id en uso; obtuve: %s", e.Message)
	}
}

// TestSpecCreateGetHandlerRoundTrip es cobertura de comportamiento del borde: una
// spec creada por la tool se recupera por get con su identidad intacta. Cierra el
// hueco de que la capa de parseo de argumentos + los DTOs de salida (create→disk,
// get→json) podrían regresar sin que el core se entere. Ejercita de paso el create
// ANIDADO (specs/rt/ no existe hasta el MkdirAll del handler) y el re-scan del
// Index que hace que get vea el archivo recién escrito.
func TestSpecCreateGetHandlerRoundTrip(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path":  "specs/rt/SPEC.md",
		"id":    "rt-1",
		"type":  "module-design",
		"title": "Round Trip",
	}); e != nil {
		t.Fatalf("create falló: %+v", e)
	}
	res, e := call(t, s, "musubi_spec_get", map[string]interface{}{"id": "rt-1"})
	if e != nil {
		t.Fatalf("get de la spec recién creada falló: %+v", e)
	}
	var out specNodeOut
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("no se pudo parsear la salida de get: %v", err)
	}
	if out.ID != "rt-1" || out.Type != "module-design" || out.Title != "Round Trip" {
		t.Errorf("get no devolvió la identidad creada: id=%q type=%q title=%q", out.ID, out.Type, out.Title)
	}
	if filepath.ToSlash(out.Path) != "specs/rt/SPEC.md" {
		t.Errorf("path esperado specs/rt/SPEC.md, obtuve %q", out.Path)
	}
}

// TestSpecGraphHandlerSerializaClavesMinuscula fija R14/#6 en el borde MCP: el
// recorte del grafo cruza la red con claves en minúscula (DTO con tags json), no
// con los nombres de campo Go en PascalCase. GraphSliceResult y sus SpecNode/
// SpecEdge NO tienen tags, así que serializarlos directo emitiría "Root"/"Nodes"/
// "From"; un cliente MCP que parsea en minúscula leería un objeto con todos los
// campos vacíos —la trampa real de skills.Skill (go-hygiene #6 / musubi-rules #6),
// una falla que se lee como un dato—. El handler convierte con toSpecGraphSliceOut,
// que SÍ lleva tags. Se arma padre←hijo por parent para que el recorte subtree
// emita al menos UNA arista y el assert mire también las claves from/to/kind.
//
// Sabotaje que lo pone en ROJO: devolver el resultado crudo en vez del DTO, con lo
// que la salida vuelve a PascalCase y el assert de minúscula falla.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="return jsonResult(toSpecGraphSliceOut(result))"
// arnes: a="return jsonResult(result)"
func TestSpecGraphHandlerSerializaClavesMinuscula(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/padre/SPEC.md", "id": "padre", "type": "goal-and-requirements", "title": "Padre",
	}); e != nil {
		t.Fatalf("create padre falló: %+v", e)
	}
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/hijo/SPEC.md", "id": "hijo", "type": "task-spec", "title": "Hijo", "parent": "padre",
	}); e != nil {
		t.Fatalf("create hijo falló: %+v", e)
	}
	res, e := call(t, s, "musubi_spec_graph", map[string]interface{}{
		"root": "padre", "direction": "subtree",
	})
	if e != nil {
		t.Fatalf("graph falló: %+v", e)
	}
	text := specCallText(t, res)
	// Las claves del DTO (contenedor + arista) tienen que estar en minúscula.
	for _, k := range []string{`"root":`, `"direction":`, `"nodes":`, `"edges":`, `"from":`, `"to":`, `"kind":`} {
		if !strings.Contains(text, k) {
			t.Errorf("la salida del grafo debía traer la clave %s en minúscula; salida: %s", k, text)
		}
	}
	// Y NINGUNA en PascalCase: eso sería el struct crudo cruzando el borde.
	for _, k := range []string{`"Root":`, `"Nodes":`, `"Edges":`, `"From":`, `"Kind":`} {
		if strings.Contains(text, k) {
			t.Errorf("la salida del grafo filtró la clave %s en PascalCase (DTO sin tags, R14/#6 roto); salida: %s", k, text)
		}
	}
}

// TestSpecCreateHandlerRechazaIdQueNoRoundTrippea fija R15(b): un id que YAML no
// vuelve a leer como el mismo texto se rechaza. SerializeFrontmatter emite el
// escalar SIN comillas, así que el id "1.0" se re-parsea como el float 1 (gist
// "1") y la identidad en disco deja de ser la que se pidió —toda arista que
// apunte a "1.0" quedaría colgada—. La guarda re-parsea lo serializado y rechaza
// el create si el id no sobrevive el round-trip. Un id "1" SÍ sobrevive (gist
// "1" == "1"), así que la guarda no sobre-rechaza.
//
// Sabotaje que lo pone en ROJO: anular la comparación del round-trip, con lo que
// "1.0" se crea igual y el "debía fallar" no se cumple.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="if gotID, ok := specgraph.Scalar(reparsed.Frontmatter, specgraph.FieldID); !ok || gotID != args.ID {"
// arnes: a="if gotID, ok := specgraph.Scalar(reparsed.Frontmatter, specgraph.FieldID); (!ok || gotID != args.ID) && false {"
func TestSpecCreateHandlerRechazaIdQueNoRoundTrippea(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	// "1" sobrevive el round-trip (YAML lo lee como el número 1, gist "1" == "1").
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/ok/SPEC.md", "id": "1", "type": "task-spec", "title": "Uno",
	}); e != nil {
		t.Fatalf("un id que SÍ round-trippea (\"1\") no debía fallar: %+v", e)
	}
	// "1.0" NO sobrevive: YAML lo lee como el float 1 (gist "1" != "1.0").
	_, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/bomba/SPEC.md", "id": "1.0", "type": "task-spec", "title": "Uno coma cero",
	})
	if e == nil {
		t.Fatal("un id que no sobrevive el round-trip YAML (\"1.0\") debía fallar, pero pasó")
	}
	if !strings.Contains(e.Message, "round-trip") {
		t.Errorf("el error debía señalar el round-trip; obtuve: %s", e.Message)
	}
}
