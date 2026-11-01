package mcp

// Tests de BORDE de las 8 tools del grafo de specs (spec-graph F1 + recall F3-A):
// se despachan por handleToolsCall —el MISMO camino que un cliente MCP, con la
// clasificación read/write y el candado del dispatch— y se afirma sobre la salida
// SERIALIZADA. El core (GrepSpecs, ResolveSpecPath, BuildGraph, Recall) ya tiene
// su batería en internal/specgraph; lo que se fija ACÁ son los invariantes que la
// spec declara sobre la TOOL y cuyo sitio de declaración es este handler, no el core:
//   - R12: musubi_spec_grep devuelve la lista aunque esté vacía como `[]`, no `null`.
//   - R15: musubi_spec_create FALLA si el id ya está en uso.
//   - R14/#6: musubi_spec_graph serializa el DTO con claves en minúscula, no los
//     nombres de campo Go en PascalCase (un cliente que parsea en minúscula leería
//     un objeto vacío).
//   - R15(b): musubi_spec_create rechaza un id que YAML no vuelve a leer igual
//     (p. ej. "1.0" → float 1): la identidad en disco dejaría de ser la pedida.
//   - musubi_spec_recall: query obligatorio (borde), hits vacíos como `[]` no `null`,
//     DTO en minúscula, y —el invariante central model-free (R5)— rankea SIN embebedor
//     (NoopProvider) en vez de rechazar como search_semantic. El sitio de la
//     degradación es este handler (embed=nil cuando !embedding.Enabled); el core
//     cubre el resto en recall_test.go.
// Cada ancla «Sabotaje» lleva su directiva `// arnes:` mecanizada (archivo/de/a)
// colgada debajo, así el censo del arnés (internal/mcp/arnes_censo_test.go) la
// cuenta como mecanizada y no como deuda en prosa. Los sabotajes son una
// sustitución de texto que compila: R12 cambia el make por un slice nil, R15
// desarma la compuerta del id duplicado, R14/#6 devuelve el struct crudo en vez
// del DTO, R15(b) anula la comparación del round-trip, y los tres de recall
// (query, lista, DTO) espejan a los de grep/graph. storewindows_test.go no
// entra en el censo porque su prosa no arranca con «Sabotaje» (no es un ancla).

import (
	"encoding/json"
	"os"
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

// escribirCodigo deja un archivo de CÓDIGO en la raíz del proyecto (al que anclar). No es una spec:
// el índice scanea SPEC.md, así que un .go conviven sin afectarlo.
func escribirCodigo(t *testing.T, dir, rel, contenido string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSpecValidateReportaRancedumbreSinVoltearValid fija R8/D8 en su sitio de declaración: validate
// REPORTA las anclas rancias de todo el grafo en su propia sección, pero eso NO vuelve inválida una
// spec que es estructuralmente sana. `valid` es integridad del grafo de specs (dangling/duplicados/
// ciclos/tipos/rechazadas); la deriva de CÓDIGO es otra cosa, y un validate gateado en CI no debe
// llorar porque alguien editó una función.
//
// Sabotaje que lo pone en ROJO: acoplar `valid` a que no haya anclas rancias (&& len(StaleAnchors)==0);
// entonces esta spec sana con un ancla cambiada reportaría valid=false y el assert de abajo falla.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="out.Valid = specgraph.IsValid(report) && len(unknown) == 0 && len(rejected) == 0"
// arnes: a="out.Valid = specgraph.IsValid(report) && len(unknown) == 0 && len(rejected) == 0 && len(out.StaleAnchors) == 0"
func TestSpecValidateReportaRancedumbreSinVoltearValid(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	escribirCodigo(t, dir, "code.go", "package p\n\nfunc Foo() int { return 1 }\n")
	// Una spec estructuralmente VÁLIDA, anclada al símbolo Foo.
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/a/SPEC.md", "id": "s-a", "type": "task-spec", "title": "A",
		"anchors": []string{"code.go#Foo"},
	}); e != nil {
		t.Fatalf("create con ancla no debía fallar: %+v", e)
	}
	// Se cambia el cuerpo de Foo: el ancla queda "changed".
	escribirCodigo(t, dir, "code.go", "package p\n\nfunc Foo() int { return 999 }\n")

	res, e := call(t, s, "musubi_spec_validate", map[string]interface{}{})
	if e != nil {
		t.Fatalf("validate falló: %+v", e)
	}
	var out struct {
		Valid        bool `json:"valid"`
		StaleAnchors []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			Reason string `json:"reason"`
		} `json:"staleAnchors"`
	}
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("no se pudo parsear validate: %v", err)
	}
	// La rancedumbre se REPORTA, con el id de la spec dueña…
	if len(out.StaleAnchors) != 1 || out.StaleAnchors[0].Reason != "changed" ||
		out.StaleAnchors[0].ID != "s-a" || out.StaleAnchors[0].Path != "code.go#Foo" {
		t.Errorf("validate debía reportar 1 ancla 'changed' en s-a/code.go#Foo, dio %+v", out.StaleAnchors)
	}
	// …pero NO voltea `valid`: la integridad estructural del grafo sigue intacta (D8).
	if !out.Valid {
		t.Error("una spec estructuralmente válida con un ancla rancia DEBE seguir siendo valid=true (D8)")
	}
}

// TestSpecGetSurfaceaRancedumbre fija R4/R5 en el borde: get re-deriva las anclas de la spec y
// adjunta la marca como CAMPO EXTRA (stale + staleWarning), sin tocar la identidad del nodo. El
// "surface-only" es estructural: el handler arma `out` desde node.* y sólo AGREGA stale/staleWarning,
// así que la identidad no puede cambiar por diseño (no hay línea que la mute); lo mecanizable —y lo
// que acá se fija— es que la marca efectivamente APAREZCA en la salida de get.
//
// Sabotaje que lo pone en ROJO: anular la condición que adjunta la marca (len(stale) > 0 → < 0), con
// lo que get nunca surfacea la rancedumbre y los asserts de `stale`/`staleWarning` fallan.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="if stale := specgraph.StalenessOf(s.projectPath, node.Frontmatter); len(stale) > 0 {"
// arnes: a="if stale := specgraph.StalenessOf(s.projectPath, node.Frontmatter); len(stale) < 0 {"
func TestSpecGetSurfaceaRancedumbre(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	escribirCodigo(t, dir, "code.go", "package p\n\nfunc Foo() int { return 1 }\n")
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/a/SPEC.md", "id": "s-a", "type": "task-spec", "title": "A",
		"anchors": []string{"code.go#Foo"},
	}); e != nil {
		t.Fatalf("create con ancla no debía fallar: %+v", e)
	}
	escribirCodigo(t, dir, "code.go", "package p\n\nfunc Foo() int { return 42 }\n")

	res, e := call(t, s, "musubi_spec_get", map[string]interface{}{"id": "s-a"})
	if e != nil {
		t.Fatalf("get falló: %+v", e)
	}
	var out specNodeOut
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("no se pudo parsear get: %v", err)
	}
	// La identidad del nodo queda intacta (surface-only).
	if out.ID != "s-a" || out.Type != "task-spec" || out.Title != "A" {
		t.Errorf("la marca no debe tocar la identidad del nodo: id=%q type=%q title=%q", out.ID, out.Type, out.Title)
	}
	// Y la marca aparece como campo extra, con su warning en prosa.
	if len(out.Stale) != 1 || out.Stale[0].Reason != "changed" || out.Stale[0].Path != "code.go#Foo" {
		t.Errorf("get debía surfacear el ancla cambiada, dio %+v", out.Stale)
	}
	if !strings.HasPrefix(out.StaleWarning, "⚠ posiblemente rancia") {
		t.Errorf("el staleWarning en prosa debía estar presente, dio %q", out.StaleWarning)
	}
}

// TestSpecCreateConAnchorRechazaSimboloRoto es cobertura del wiring de create (R2 en el borde): un
// ancla a un símbolo inexistente hace FALLAR el create —no nace una spec con un ancla rota—, y una
// ancla válida crea bien con el digest ya escrito en el frontmatter. El sabotaje mecanizado de R2
// vive en internal/specgraph/anchors_test.go (BuildAnchorEntries); acá sólo se fija que el handler
// propague el error en vez de tragárselo.
func TestSpecCreateConAnchorRechazaSimboloRoto(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	escribirCodigo(t, dir, "code.go", "package p\n\nfunc Foo() int { return 1 }\n")

	_, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/a/SPEC.md", "id": "s-a", "type": "task-spec", "title": "A",
		"anchors": []string{"code.go#NoExiste"},
	})
	if e == nil {
		t.Fatal("un create con un ancla a un símbolo inexistente debía fallar")
	}
	if !strings.Contains(e.Message, "al archivo entero") {
		t.Errorf("el error debía ofrecer la salida 'al archivo entero', dio: %s", e.Message)
	}

	// Un ancla BUENA crea bien, y el digest queda escrito en el frontmatter en disco.
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/b/SPEC.md", "id": "s-b", "type": "task-spec", "title": "B",
		"anchors": []string{"code.go#Foo"},
	}); e != nil {
		t.Fatalf("create con un ancla válida no debía fallar: %+v", e)
	}
	res, e := call(t, s, "musubi_spec_get", map[string]interface{}{"id": "s-b"})
	if e != nil {
		t.Fatalf("get falló: %+v", e)
	}
	if !strings.Contains(specCallText(t, res), "code.go#Foo=") {
		t.Errorf("el frontmatter debía llevar el ancla con su digest (code.go#Foo=…); salida: %s", specCallText(t, res))
	}
}

// TestSpecRecallHandlerRechazaQueryVacio fija el BORDE de recall: query es obligatorio. Un query en
// blanco no es "traeme todo" —para eso están grep sin patrón o validate—; es un llamado mal formado,
// y el handler lo corta con codeInvalidParams ANTES de rankear (nada que embeber, nada que puntuar).
// Un query de sólo espacios también cuenta como vacío (strings.TrimSpace), por eso el caso es "   ".
//
// Sabotaje que lo pone en ROJO: anular la compuerta del query vacío, con lo que un query en blanco
// entra a Recall (keywordScore con qset vacío puntúa 0 para todo) y devuelve hits SIN error, así que
// el "debía fallar" no se cumple.
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="if strings.TrimSpace(args.Query) == \"\" {"
// arnes: a="if strings.TrimSpace(args.Query) == \"\" && false {"
func TestSpecRecallHandlerRechazaQueryVacio(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	_, e := call(t, s, "musubi_spec_recall", map[string]interface{}{"query": "   "})
	if e == nil {
		t.Fatal("un recall con query en blanco debía fallar, pero pasó")
	}
	if e.Code != codeInvalidParams {
		t.Errorf("esperaba codeInvalidParams por query vacío, obtuve code=%d msg=%q", e.Code, e.Message)
	}
	if !strings.Contains(e.Message, "obligatorio") {
		t.Errorf("el error debía señalar que el query es obligatorio; obtuve: %s", e.Message)
	}
}

// TestSpecRecallHandlerSinHitsSerializaListaNoNull fija go-hygiene #6 en el borde de recall: un repo
// sin specs devuelve "hits":[], no "hits":null. El core ya devuelve make([]SpecHit,0) en el corpus
// vacío (recall.go), pero el DTO la RE-ARMA en toSpecRecallOut, así que el contrato de lista-no-null
// tiene un segundo sitio de declaración ACÁ: un cliente MCP que itera la lista no debe tragarse null.
//
// Sabotaje que lo pone en ROJO: cambiar en toSpecRecallOut el
// `hits := make([]specHitOut, 0, len(result.Hits))` por `var hits []specHitOut` (nil ⇒ "hits":null).
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="hits := make([]specHitOut, 0, len(result.Hits))"
// arnes: a="var hits []specHitOut"
func TestSpecRecallHandlerSinHitsSerializaListaNoNull(t *testing.T) {
	// Repo sin specs: recall devuelve cero hits. El query es obligatorio pero su valor da igual, no
	// hay corpus que rankear.
	s := newTestServerWithPath(t, t.TempDir())
	res, e := call(t, s, "musubi_spec_recall", map[string]interface{}{"query": "lo que sea"})
	if e != nil {
		t.Fatalf("recall sin specs NO debía dar error (model-free, R5): %+v", e)
	}
	text := specCallText(t, res)
	if !strings.Contains(text, `"hits":[]`) {
		t.Errorf("recall vacío debe serializar \"hits\":[] (no null); salida: %s", text)
	}
	if strings.Contains(text, `"hits":null`) {
		t.Errorf("recall serializó la lista vacía como null (go-hygiene #6 roto); salida: %s", text)
	}
}

// TestSpecRecallHandlerSerializaClavesMinuscula fija R14/#6 en el borde de recall: la salida cruza la
// red con claves en minúscula (DTO con tags json), no con los nombres de campo Go de
// specgraph.RecallResult/SpecHit, que NO tienen tags —serializarlos directo emitiría "Hits"/"ID"/
// "Score", y un cliente MCP que parsea en minúscula leería un objeto vacío (musubi-rules #6)—. El
// handler convierte con toSpecRecallOut. Se crea UNA spec para que haya al menos un hit y el assert
// mire también las claves de la spec (id/type/path/score), no sólo las del contenedor.
//
// Sabotaje que lo pone en ROJO: devolver el resultado crudo en vez del DTO, con lo que la salida
// vuelve a PascalCase y el assert de minúscula falla (espeja el sabotaje de graph).
// arnes: archivo="internal/mcp/methods_spec.go"
// arnes: de="return jsonResult(toSpecRecallOut(result))"
// arnes: a="return jsonResult(result)"
func TestSpecRecallHandlerSerializaClavesMinuscula(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/a/SPEC.md", "id": "s-a", "type": "task-spec", "title": "Alfa",
	}); e != nil {
		t.Fatalf("create falló: %+v", e)
	}
	res, e := call(t, s, "musubi_spec_recall", map[string]interface{}{"query": "alfa"})
	if e != nil {
		t.Fatalf("recall falló: %+v", e)
	}
	text := specCallText(t, res)
	// Las claves del DTO (contenedor + hit) en minúscula. truncated/id/type/path/score no tienen
	// omitempty, así que aparecen siempre aunque el hit tenga score 0.
	for _, k := range []string{`"hits":`, `"truncated":`, `"id":`, `"type":`, `"path":`, `"score":`} {
		if !strings.Contains(text, k) {
			t.Errorf("la salida de recall debía traer la clave %s en minúscula; salida: %s", k, text)
		}
	}
	// Y NINGUNA en PascalCase: eso sería el struct crudo del core cruzando el borde.
	for _, k := range []string{`"Hits":`, `"Truncated":`, `"ID":`, `"Type":`, `"Path":`, `"Score":`} {
		if strings.Contains(text, k) {
			t.Errorf("la salida de recall filtró la clave %s en PascalCase (DTO sin tags, R14/#6 roto); salida: %s", k, text)
		}
	}
}

// TestSpecRecallHandlerRankeaModelFree es el invariante CENTRAL en el borde (R5): SIN embebedor (el
// server de test usa embedding.NoopProvider ⇒ Enabled==false ⇒ embed=nil), recall NO rechaza como
// search_semantic: degrada a keyword y RANKEA. Dos specs, una con el término del query en el título y
// otra sin él; la que matchea sale primera, con score > 0 y sin error. Es cobertura de comportamiento
// (como el round-trip), por el camino real del dispatch; no lleva ancla mecanizada porque el sabotaje
// de R5 vive en el core (recall_test.go) y acá sólo se fija que el borde no lo tape exigiendo embebedor.
func TestSpecRecallHandlerRankeaModelFree(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	// La que matchea el query ("reintentos" cae en el título ⇒ en el Content crudo que puntúa keyword).
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/retry/SPEC.md", "id": "spec-retry", "type": "task-spec", "title": "politica de reintentos",
	}); e != nil {
		t.Fatalf("create retry falló: %+v", e)
	}
	// La que NO matchea.
	if _, e := call(t, s, "musubi_spec_create", map[string]interface{}{
		"path": "specs/auth/SPEC.md", "id": "spec-auth", "type": "task-spec", "title": "autenticacion de usuarios",
	}); e != nil {
		t.Fatalf("create auth falló: %+v", e)
	}
	res, e := call(t, s, "musubi_spec_recall", map[string]interface{}{"query": "reintentos"})
	if e != nil {
		t.Fatalf("recall sin embebedor NO debía dar error (R5, model-free): %+v", e)
	}
	var out specRecallOut
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("no se pudo parsear recall: %v", err)
	}
	if len(out.Hits) == 0 {
		t.Fatal("recall model-free debía devolver hits, devolvió cero")
	}
	// La spec que matchea el query por keyword sale PRIMERA, con score positivo…
	if out.Hits[0].ID != "spec-retry" {
		t.Errorf("la spec que matchea el query debía rankear primera; hits[0].ID=%q", out.Hits[0].ID)
	}
	if out.Hits[0].Score <= 0 {
		t.Errorf("el hit que matchea debía tener score > 0 (keyword), dio %v", out.Hits[0].Score)
	}
	// …ESTRICTAMENTE por encima de la que no matchea (score 0), probando que rankeó y no sólo listó.
	if len(out.Hits) > 1 && out.Hits[0].Score <= out.Hits[1].Score {
		t.Errorf("la spec que matchea (score %v) debía rankear estrictamente sobre la otra (score %v)",
			out.Hits[0].Score, out.Hits[1].Score)
	}
}
