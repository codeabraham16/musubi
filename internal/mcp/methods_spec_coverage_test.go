package mcp

// methods_spec_coverage_test.go fija, invariante por invariante, los contratos de musubi_spec_coverage
// (la 9ª spec-tool, el cross-link spec↔código de F3-B) EN SU SITIO DE DECLARACIÓN —el handler—, por el
// MISMO camino que un cliente MCP (handleToolsCall, clasificación read/write, candado del dispatch) y
// afirmando sobre la salida SERIALIZADA. El aislamiento cross-tenant NO va acá: vive en su test
// dedicado (D8), TestSpecCoverageCrossTenantIsolation (read_surface_class_test.go). Acá va la lógica de
// negocio con el scope FEDERADO de un call() sin principal (recallScopeFor(nil) ⇒ federado), que ve el
// grafo sembrado en el tenant por defecto con UpsertPackageGraph.
//
// Cada test lleva su directiva `// arnes:` mecanizada (archivo/de/a), para que el censo del arnés la
// cuente como mecanizada y no como deuda en prosa, y para que `go test -overlay` pueda aplicar el
// sabotaje sin tocar el árbol. El sabotaje ataca EL INVARIANTE QUE EL TEST DECLARA —no una validación
// cualquiera—, porque la defensa en profundidad tapa una mutación al costado y deja el test vacuo
// (adversarial-review / musubi-rules #8). Reusa escribirCodigo/call/specCallText de methods_spec_test.go.

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// coverageServer arma un server con projectPath propio y DEVUELVE TAMBIÉN el engine, para sembrar el
// grafo de código directo: UpsertPackageGraph escribe en el tenant por defecto, y el scope federado de
// un call() sin principal lo ve. (newTestServerWithPath no expone el engine, por eso este helper.)
func coverageServer(t *testing.T) (*McpServer, *memory.DbEngine, string) {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	dir := t.TempDir()
	return NewMcpServer(engine, dir, embedding.NoopProvider{}), engine, dir
}

// sembrarDiamante escribe graph/core.go con cuatro funcs y siembra el grafo Raiz→{Rama1,Rama2}→Hoja.
// Los callers transitivos de Hoja son {Rama1, Rama2, Raiz}: Raiz se alcanza por DOS caminos, así que el
// montaje prueba el dedup de la unión (R1); y los tres node_keys ordenan asc como [Raiz, Rama1, Rama2],
// así que prueba el orden determinista (R8/D10). El SrcFingerprint es irrelevante para resolver/medir
// (eso va por nombre y por aristas), así que va uno cualquiera.
func sembrarDiamante(t *testing.T, e *memory.DbEngine, dir string) {
	t.Helper()
	escribirCodigo(t, dir, "graph/core.go",
		"package core\n\nfunc Hoja() {}\nfunc Rama1() { Hoja() }\nfunc Rama2() { Hoja() }\nfunc Raiz() { Rama1(); Rama2() }\n")
	nodo := func(n string) memory.GraphNode {
		return memory.GraphNode{Key: "graph/core.go#func:" + n, Kind: "func", Name: n, Path: "graph/core.go", SrcFingerprint: "h"}
	}
	arista := func(from, to string) memory.GraphEdge {
		return memory.GraphEdge{
			FromKey: "graph/core.go#func:" + from, ToKey: "graph/core.go#func:" + to,
			Kind: "CALLS", Confidence: 1, Provenance: "test", SrcPath: "graph/core.go", SrcFingerprint: "h",
		}
	}
	if err := e.UpsertPackageGraph([]string{"graph/core.go"},
		[]memory.GraphNode{nodo("Hoja"), nodo("Rama1"), nodo("Rama2"), nodo("Raiz")},
		[]memory.GraphEdge{arista("Rama1", "Hoja"), arista("Rama2", "Hoja"), arista("Raiz", "Rama1"), arista("Raiz", "Rama2")},
	); err != nil {
		t.Fatalf("UpsertPackageGraph: %v", err)
	}
}

// crearSpec crea una spec por la tool (camino real, con su validación de anclas). anchors en forma
// cruda `ruta#símbolo` (sin digest): BuildAnchorEntries computa el baseline.
func crearSpec(t *testing.T, s *McpServer, id string, anchors []string) {
	t.Helper()
	args := map[string]interface{}{"path": "specs/" + id + "/SPEC.md", "id": id, "type": "task-spec", "title": id}
	if len(anchors) > 0 {
		args["anchors"] = anchors
	}
	if _, e := call(t, s, "musubi_spec_create", args); e != nil {
		t.Fatalf("create %q: %+v", id, e)
	}
}

// coverageForward llama a la dirección spec→código y parsea el DTO.
func coverageForward(t *testing.T, s *McpServer, specID string) specCoverageForwardOut {
	t.Helper()
	res, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{"spec_id": specID})
	if e != nil {
		t.Fatalf("coverage forward %q: %+v", specID, e)
	}
	var out specCoverageForwardOut
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("parse forward: %v", err)
	}
	return out
}

// TestSpecCoverageExigeExactamenteUnaDireccion fija R3/D4: exactamente UNA dirección por llamada. Cero
// deja la intención sin definir; dos la vuelve ambigua. El handler corta con codeInvalidParams ANTES de
// tocar el índice o el grafo.
//
// Sabotaje que lo pone en ROJO: aflojar la compuerta de "exactamente una" a "al menos una"
// (if dirs < 1) — la mutación VACUA que el panel encontró, que deja pasar el caso de DOS direcciones.
// Con la spec "x" creada, esa llamada ya no corta por R7 (spec inexistente) sino que resuelve un forward
// EXITOSO, así que el assert "dos direcciones debían dar codeInvalidParams" se rompe. Se mecaniza con
// < 1 y no con un apagado total a propósito: < 1 deja intacto el corte del caso cero, de modo que lo que
// ataca es la mitad "exactamente UNA" (rechazar dos), que es la que estaba sin custodiar.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="if dirs != 1 {"
// arnes: a="if dirs < 1 {"
func TestSpecCoverageExigeExactamenteUnaDireccion(t *testing.T) {
	s, _, dir := coverageServer(t)
	// La spec "x" DEBE existir para que el caso "dos direcciones" NO sea vacuo: sin ella, un spec_id
	// inexistente igual corta por R7 (:109 if !ok), así que apagar la compuerta XOR daría el MISMO
	// codeInvalidParams y el sabotaje quedaría verde. Con "x" creada, apagar la compuerta cae a un
	// forward EXITOSO ⇒ el assert "debía dar codeInvalidParams" se rompe (ataca R3, no R7).
	escribirCodigo(t, dir, "g/c.go", "package c\n\nfunc Y() {}\n")
	crearSpec(t, s, "x", []string{"g/c.go#Y"})
	// Cero direcciones: error de uso.
	if _, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("sin dirección debía dar codeInvalidParams, obtuve %+v", e)
	}
	// Dos direcciones (spec_id + symbol): ambiguo, error de uso.
	if _, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{"spec_id": "x", "symbol": "y"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("dos direcciones debían dar codeInvalidParams, obtuve %+v", e)
	}
	// Una sola dirección pasa la compuerta (symbol sin matches ⇒ respuesta vacía, no error).
	if _, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{"symbol": "nada.go#X"}); e != nil {
		t.Errorf("una sola dirección no debía dar error de compuerta, obtuve %+v", e)
	}
}

// TestSpecCoverageSinGrafoAvisaYNoMide fija R4/D3: sin grafo indexado el ancla se reporta igual pero con
// resolution="graph_not_indexed" y un warning, SIN blast —medir impacto sobre un grafo vacío mentiría
// "no rompe nada"—. graph_not_indexed es ortogonal a no_resuelto (R6) y a la ranciedad (R5).
//
// Sabotaje que lo pone en ROJO: volver graphIndexed siempre true (graphNodes >= 0, que como es un
// conteo nunca es negativo), con lo que sin grafo el ancla cae a la rama de resolución, da no_resuelto
// y NO pone warning.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="graphIndexed := graphNodes > 0"
// arnes: a="graphIndexed := graphNodes >= 0"
func TestSpecCoverageSinGrafoAvisaYNoMide(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "graph/core.go", "package core\n\nfunc Hoja() {}\n")
	crearSpec(t, s, "s-cob", []string{"graph/core.go#Hoja"})
	// Sin UpsertPackageGraph: el grafo está vacío.
	out := coverageForward(t, s, "s-cob")
	if len(out.Anchors) != 1 {
		t.Fatalf("esperaba 1 ancla, dio %d", len(out.Anchors))
	}
	if out.Anchors[0].Resolution != "graph_not_indexed" {
		t.Errorf("sin grafo la resolución debía ser graph_not_indexed, dio %q", out.Anchors[0].Resolution)
	}
	if out.Warning == "" {
		t.Error("sin grafo debía venir un warning")
	}
	if out.BlastCount != 0 || len(out.Anchors[0].BlastRadius) != 0 {
		t.Errorf("sin grafo no se mide impacto, dio blastCount=%d anchorBlast=%d", out.BlastCount, len(out.Anchors[0].BlastRadius))
	}
}

// TestSpecCoverageMarcaRanciedadSinTocarResolucion fija R5 y su ORTOGONALIDAD con la resolución: una
// ancla cuyo código cambió en disco se marca staleness="changed" PERO sigue resolviéndose contra el
// grafo (resolution="resolved"). La ranciedad es disco-vs-baseline (StalenessOf), independiente de si el
// símbolo está en el grafo: colapsarlas mentiría en los dos sentidos.
//
// Sabotaje que lo pone en ROJO: anular la asignación del staleness (isStale && false), con lo que un
// ancla rancia queda staleness="none" y el assert de "changed" falla.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="if r, isStale := staleByRef[a.Ref]; isStale {"
// arnes: a="if r, isStale := staleByRef[a.Ref]; isStale && false {"
func TestSpecCoverageMarcaRanciedadSinTocarResolucion(t *testing.T) {
	s, e, dir := coverageServer(t)
	sembrarDiamante(t, e, dir)
	crearSpec(t, s, "s-rancia", []string{"graph/core.go#Hoja"}) // baseline = Hoja actual
	// Se cambia el cuerpo de Hoja: el ancla queda "changed" en disco, pero el nodo sigue en el grafo.
	escribirCodigo(t, dir, "graph/core.go",
		"package core\n\nfunc Hoja() { _ = 1 }\nfunc Rama1() { Hoja() }\nfunc Rama2() { Hoja() }\nfunc Raiz() { Rama1(); Rama2() }\n")
	out := coverageForward(t, s, "s-rancia")
	if len(out.Anchors) != 1 {
		t.Fatalf("esperaba 1 ancla, dio %d", len(out.Anchors))
	}
	if out.Anchors[0].Staleness != "changed" {
		t.Errorf("el ancla con código cambiado debía marcar staleness=changed, dio %q", out.Anchors[0].Staleness)
	}
	// Ortogonalidad: cambió el disco, pero el símbolo sigue en el grafo ⇒ resuelve igual.
	if out.Anchors[0].Resolution != "resolved" {
		t.Errorf("la ranciedad NO debe impedir la resolución (ortogonales), dio %q", out.Anchors[0].Resolution)
	}
}

// TestSpecCoverageAnclaSinNodoEsNoResuelto fija R6: con grafo indexado pero un ancla cuyo símbolo no
// tiene nodo (archivo no indexado, lenguaje sin extractor, o grafo viejo) la resolución es "no_resuelto"
// —distinta de graph_not_indexed (acá SÍ hay grafo) y de missing (el símbolo SÍ está en disco)—.
//
// Sabotaje que lo pone en ROJO: volver inalcanzable la rama de len(matched)==0 (< 0), con lo que un
// ancla sin nodo cae a "resolved" con NodeKeys vacío y el assert de no_resuelto falla.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="if len(matched) == 0 {"
// arnes: a="if len(matched) < 0 {"
func TestSpecCoverageAnclaSinNodoEsNoResuelto(t *testing.T) {
	s, e, dir := coverageServer(t)
	sembrarDiamante(t, e, dir) // el grafo tiene nodos (graph/core.go) ⇒ graphIndexed=true
	// Un archivo con un símbolo que existe en DISCO (para que el create valide) pero NO en el grafo.
	escribirCodigo(t, dir, "otro/suelto.go", "package suelto\n\nfunc Huerfano() {}\n")
	crearSpec(t, s, "s-huerfano", []string{"otro/suelto.go#Huerfano"})
	out := coverageForward(t, s, "s-huerfano")
	if len(out.Anchors) != 1 {
		t.Fatalf("esperaba 1 ancla, dio %d", len(out.Anchors))
	}
	if out.Anchors[0].Resolution != "no_resuelto" {
		t.Errorf("un ancla con grafo indexado pero sin nodo debía ser no_resuelto, dio %q", out.Anchors[0].Resolution)
	}
	if len(out.Anchors[0].NodeKeys) != 0 {
		t.Errorf("un ancla no resuelta no debía traer node_keys, dio %v", out.Anchors[0].NodeKeys)
	}
}

// TestSpecCoverageSpecInexistenteEsError fija R7 (forward): un spec_id que no existe es un error de uso
// (codeInvalidParams), NUNCA un resultado vacío que se leería "sin cobertura" —esa ambigüedad enseña a
// no confiar en el error—.
//
// Sabotaje que lo pone en ROJO: anular la compuerta de existencia (if !ok && false), con lo que un
// spec_id inexistente usa el nodo cero (sin anclas) y devuelve un forward vacío sin error.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="if !ok {"
// arnes: a="if !ok && false {"
func TestSpecCoverageSpecInexistenteEsError(t *testing.T) {
	s, _, _ := coverageServer(t)
	if _, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{"spec_id": "no-existe"}); e == nil || e.Code != codeInvalidParams {
		t.Errorf("un spec_id inexistente debía dar codeInvalidParams, obtuve %+v", e)
	}
}

// TestSpecCoverageBlastRadiusDedupYOrdenado fija R1 (el blast es la UNIÓN DEDUP de los callers
// transitivos) y R8/D10 (ordenado por node_key, determinista —a diferencia de musubi_impact, que hereda
// el orden de descubrimiento del BFS de callersOfCtx, sin ORDER BY—). El diamante Raiz→{Rama1,Rama2}→Hoja
// alcanza Raiz por DOS caminos: aparece UNA vez (dedup), y los tres salen en orden asc.
//
// Sabotaje que lo pone en ROJO: invertir el orden en sortedStringSet (sort.Reverse), con lo que el blast
// sale descendente y el assert de orden asc falla. El dedup sigue bien (lo da el set), pero el
// determinismo del orden —que es la decisión D10— no.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="sort.Strings(out)"
// arnes: a="sort.Sort(sort.Reverse(sort.StringSlice(out)))"
func TestSpecCoverageBlastRadiusDedupYOrdenado(t *testing.T) {
	s, e, dir := coverageServer(t)
	sembrarDiamante(t, e, dir)
	crearSpec(t, s, "s-blast", []string{"graph/core.go#Hoja"})
	out := coverageForward(t, s, "s-blast")

	// El orden canónico asc, calculado por el test con SU PROPIA aritmética (no le pide el número a la
	// misma función que verifica).
	esperado := []string{"graph/core.go#func:Raiz", "graph/core.go#func:Rama1", "graph/core.go#func:Rama2"}
	sort.Strings(esperado)

	if out.BlastCount != 3 {
		t.Errorf("el blast dedup debía tener 3 callers (Raiz una sola vez), dio %d: %v", out.BlastCount, out.BlastRadius)
	}
	if !reflect.DeepEqual(out.BlastRadius, esperado) {
		t.Errorf("el blast debía venir ordenado asc por node_key\nesperado: %v\nobtuve:   %v", esperado, out.BlastRadius)
	}
	// Y el ancla resuelta refleja el mismo blast a su nivel.
	if len(out.Anchors) != 1 || out.Anchors[0].Resolution != "resolved" {
		t.Fatalf("esperaba 1 ancla resuelta, dio %+v", out.Anchors)
	}
	if !reflect.DeepEqual(out.Anchors[0].BlastRadius, esperado) {
		t.Errorf("el blast del ancla debía coincidir con la unión ordenada, obtuve %v", out.Anchors[0].BlastRadius)
	}
}

// TestSpecCoverageInversoMatcheaPorSimboloModuloYNodeKey fija la dirección código→spec: encuentra las
// specs cuyas anclas cubren un símbolo, aceptando TANTO la forma `ruta#símbolo` como el node_key
// `ruta#kind:name` (D5, vía symbolNameFromKey), y por módulo (underModule). Un símbolo sin cobertura
// devuelve specs=[] SIN error (R7 inverso: "sin cobertura" no es un error de uso).
//
// Sabotaje que lo pone en ROJO: romper la igualdad de símbolo (a.Symbol != wantSym), con lo que una
// consulta por el símbolo anclado deja de encontrar su spec y el assert de "la encuentra" falla.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="a.Symbol == wantSym"
// arnes: a="a.Symbol != wantSym"
func TestSpecCoverageInversoMatcheaPorSimboloModuloYNodeKey(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "graph/core.go", "package core\n\nfunc Hoja() {}\n")
	crearSpec(t, s, "s-inv", []string{"graph/core.go#Hoja"})

	inverso := func(args map[string]interface{}) specCoverageInverseOut {
		res, e := call(t, s, "musubi_spec_coverage", args)
		if e != nil {
			t.Fatalf("coverage inverso %v: %+v", args, e)
		}
		var out specCoverageInverseOut
		if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
			t.Fatalf("parse inverso: %v", err)
		}
		return out
	}
	cubre := func(out specCoverageInverseOut) bool {
		for _, sp := range out.Specs {
			if sp.ID == "s-inv" {
				return true
			}
		}
		return false
	}

	// Forma ancla `ruta#símbolo`.
	if !cubre(inverso(map[string]interface{}{"symbol": "graph/core.go#Hoja"})) {
		t.Error("la consulta por `ruta#símbolo` debía encontrar s-inv")
	}
	// Forma node_key `ruta#kind:name` (D5): symbolNameFromKey descarta el `func:`.
	if !cubre(inverso(map[string]interface{}{"symbol": "graph/core.go#func:Hoja"})) {
		t.Error("la consulta por node_key (func:Hoja) debía normalizar y encontrar s-inv (D5)")
	}
	// Por módulo.
	if !cubre(inverso(map[string]interface{}{"module": "graph"})) {
		t.Error("la consulta por módulo `graph` debía encontrar s-inv")
	}
	// Sin cobertura: specs vacío, SIN error (R7 inverso).
	sinCob := inverso(map[string]interface{}{"symbol": "graph/core.go#NoExiste"})
	if len(sinCob.Specs) != 0 {
		t.Errorf("un símbolo sin cobertura debía dar specs=[], dio %+v", sinCob.Specs)
	}
}

// TestSpecCoverageOverlayEsAnclaBased fija R13/D7: el overlay de cobertura por módulo es ANCLA-BASED —un
// módulo está cubierto si alguna ancla de alguna spec apunta a un archivo DENTRO de él—, NO "hay un
// SPEC.md en el dir". Musubi centraliza sus specs en specs/<change>/, así que el modelo co-locado de
// thinkrail daría casi todo descubierto y sería inútil.
//
// Sabotaje que lo pone en ROJO: marcar el dir de la PROPIA spec (node.Path) en vez del dir del ancla
// (a.Path), con lo que el módulo realmente anclado (internal/bar) deja de contarse como cubierto y
// aparece en without_spec.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="covered[path.Dir(a.Path)] = true"
// arnes: a="covered[path.Dir(node.Path)] = true; _ = a"
func TestSpecCoverageOverlayEsAnclaBased(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "internal/bar/y.go", "package bar\n\nfunc Bar() {}\n")
	escribirCodigo(t, dir, "internal/foo/x.go", "package foo\n\nfunc Foo() {}\n")
	// La spec vive en specs/<id>/ y ancla a internal/bar: bajo "SPEC.md-in-dir" NINGÚN módulo de código
	// quedaría cubierto; bajo ancla-based, internal/bar sí y internal/foo no.
	crearSpec(t, s, "s-overlay", []string{"internal/bar/y.go#Bar"})

	ov := s.coverageOverlay()
	sinSpec, _ := ov["without_spec"].([]string)
	contiene := func(xs []string, x string) bool {
		for _, v := range xs {
			if v == x {
				return true
			}
		}
		return false
	}
	// El módulo anclado NO está entre los descubiertos.
	if contiene(sinSpec, "internal/bar") {
		t.Errorf("internal/bar está anclado: no debía aparecer en without_spec %v", sinSpec)
	}
	// El módulo sin ancla SÍ está.
	if !contiene(sinSpec, "internal/foo") {
		t.Errorf("internal/foo no tiene ancla: debía aparecer en without_spec %v", sinSpec)
	}
	if ws, _ := ov["with_spec"].(int); ws < 1 {
		t.Errorf("con un módulo anclado with_spec debía ser >= 1, dio %v", ov["with_spec"])
	}
}

// TestMapCoverageFlagEsOptIn fija R11/D9: el overlay de cobertura en musubi_map es OPT-IN. Sin el flag la
// respuesta es byte-idéntica a la de siempre (el campo "coverage" NI APARECE); con coverage=true se
// agrega. El default byte-idéntico es lo que deja meter el cross-link sin tocar a ningún consumidor
// existente de map.
//
// Sabotaje que lo pone en ROJO: soltar el requisito del flag (quitar `&& args.Coverage`), con lo que una
// llamada SIN flag igual trae el overlay y el assert de "no aparece coverage" falla.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if json.Unmarshal(raw, &args) == nil && args.Coverage {"
// arnes: a="if json.Unmarshal(raw, &args) == nil {"
func TestMapCoverageFlagEsOptIn(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "internal/bar/y.go", "package bar\n\nfunc Bar() {}\n")
	crearSpec(t, s, "s-map", []string{"internal/bar/y.go#Bar"})

	// Sin flag: la respuesta NO menciona coverage.
	resSin, e := call(t, s, "musubi_map", map[string]interface{}{})
	if e != nil {
		t.Fatalf("map sin flag: %+v", e)
	}
	if strings.Contains(specCallText(t, resSin), "coverage") {
		t.Errorf("map sin flag no debía traer el overlay de coverage; salida: %s", specCallText(t, resSin))
	}
	// Con flag: aparece el overlay con sus campos.
	resCon, e := call(t, s, "musubi_map", map[string]interface{}{"coverage": true})
	if e != nil {
		t.Fatalf("map con flag: %+v", e)
	}
	text := specCallText(t, resCon)
	for _, k := range []string{`"coverage"`, `"modules_total"`, `"without_spec"`} {
		if !strings.Contains(text, k) {
			t.Errorf("map con coverage=true debía traer %s; salida: %s", k, text)
		}
	}
}

// TestSpecCoverageForwardOrdenaAnclasPorRef fija la cláusula "anchors por ref asc" de R8 en el forward.
// Estaba IMPLEMENTADA (:195) pero sin test no-vacuo: ningún fixture tenía ≥2 anclas, así que invertir ese
// sort no ponía rojo a nadie. Dos anclas declaradas al REVÉS deben salir asc por ref. El orden es
// independiente del grafo (no se siembra), así que el test aísla R8 de la resolución.
//
// Sabotaje que lo pone en ROJO: invertir la comparación del sort (< por >), con lo que las anclas salen
// desc y el assert de [core, more] falla.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="out.Anchors[i].Ref < out.Anchors[j].Ref"
// arnes: a="out.Anchors[i].Ref > out.Anchors[j].Ref"
func TestSpecCoverageForwardOrdenaAnclasPorRef(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "graph/core.go", "package core\n\nfunc Hoja() {}\n")
	escribirCodigo(t, dir, "graph/more.go", "package core\n\nfunc Otra() {}\n")
	// Declaradas al revés (more antes que core): BuildAnchorEntries conserva ese orden de aparición; el
	// forward DEBE reordenarlas asc por ref.
	crearSpec(t, s, "s-ord", []string{"graph/more.go#Otra", "graph/core.go#Hoja"})
	out := coverageForward(t, s, "s-ord")
	if len(out.Anchors) != 2 {
		t.Fatalf("esperaba 2 anclas, dio %d: %+v", len(out.Anchors), out.Anchors)
	}
	if out.Anchors[0].Ref != "graph/core.go#Hoja" || out.Anchors[1].Ref != "graph/more.go#Otra" {
		t.Errorf("las anclas debían salir asc por ref [core, more], dio [%s, %s]",
			out.Anchors[0].Ref, out.Anchors[1].Ref)
	}
}

// TestSpecCoverageInversoOrdenaSpecsPorId fija la cláusula "specs por id asc" de R8 en la inversa,
// también implementada (:250) pero sin test no-vacuo: ningún fixture tenía ≥2 specs. Como el índice de
// specs es un MAPA (orden de iteración aleatorio en Go), sin el sort la salida sería no-determinista; el
// sort la fija asc por id. Se crean b-antes-de-a para que el orden NO venga del de creación.
//
// Sabotaje que lo pone en ROJO: invertir la comparación del sort (< por >), con lo que las specs salen
// desc y el assert de [s-a, s-b] falla de forma determinista.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="out.Specs[i].ID < out.Specs[j].ID"
// arnes: a="out.Specs[i].ID > out.Specs[j].ID"
func TestSpecCoverageInversoOrdenaSpecsPorId(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "graph/core.go", "package core\n\nfunc Hoja() {}\n")
	crearSpec(t, s, "s-b", []string{"graph/core.go#Hoja"})
	crearSpec(t, s, "s-a", []string{"graph/core.go#Hoja"})
	res, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{"module": "graph"})
	if e != nil {
		t.Fatalf("coverage inverso por módulo: %+v", e)
	}
	var out specCoverageInverseOut
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("parse inverso: %v", err)
	}
	if len(out.Specs) != 2 {
		t.Fatalf("esperaba 2 specs bajo el módulo graph, dio %d: %+v", len(out.Specs), out.Specs)
	}
	if out.Specs[0].ID != "s-a" || out.Specs[1].ID != "s-b" {
		t.Errorf("las specs debían salir asc por id [s-a, s-b], dio [%s, %s]",
			out.Specs[0].ID, out.Specs[1].ID)
	}
}

// TestSpecCoverageModuloRespetaBordeDeSegmento fija el borde de segmento de underModule (D5): el módulo
// "graph" cubre "graph/core.go" pero NO "graphext/x.go" —compara contra "graph/", no contra el prefijo
// crudo "graph"—. Sin este test, borrar el "/" del prefijo pasaba verde (X3 del panel).
//
// Sabotaje que lo pone en ROJO: comparar contra el prefijo crudo (module sin el "/"), con lo que una
// consulta por "graph" matchea "graphext/x.go" y la spec de graphext aparece donde no debe.
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="strings.HasPrefix(filePath, module+\"/\")"
// arnes: a="strings.HasPrefix(filePath, module)"
func TestSpecCoverageModuloRespetaBordeDeSegmento(t *testing.T) {
	s, _, dir := coverageServer(t)
	escribirCodigo(t, dir, "graph/core.go", "package core\n\nfunc Hoja() {}\n")
	escribirCodigo(t, dir, "graphext/x.go", "package graphext\n\nfunc X() {}\n")
	crearSpec(t, s, "s-g", []string{"graph/core.go#Hoja"})
	crearSpec(t, s, "s-gx", []string{"graphext/x.go#X"})

	res, e := call(t, s, "musubi_spec_coverage", map[string]interface{}{"module": "graph"})
	if e != nil {
		t.Fatalf("coverage inverso por módulo: %+v", e)
	}
	var out specCoverageInverseOut
	if err := json.Unmarshal([]byte(specCallText(t, res)), &out); err != nil {
		t.Fatalf("parse inverso: %v", err)
	}
	tiene := func(id string) bool {
		for _, sp := range out.Specs {
			if sp.ID == id {
				return true
			}
		}
		return false
	}
	if !tiene("s-g") {
		t.Errorf("el módulo graph debía cubrir graph/core.go (s-g), specs=%+v", out.Specs)
	}
	if tiene("s-gx") {
		t.Errorf("el módulo graph NO debía cubrir graphext/x.go (borde de segmento), specs=%+v", out.Specs)
	}
}

// TestSpecCoverageAnclaMissingResuelveEnGrafoStale fija la ORTOGONALIDAD de R5 con la resolución (D3) en
// el régimen "grafo stale" que el escenario de la spec no observaba: un ancla cuyo símbolo se borró del
// DISCO sale staleness="missing" PERO, si el grafo no se reindexó y el nodo sigue ahí, resuelve igual y
// mide su blast. Las dos verdades (disco y grafo) se informan juntas; no se colapsan.
//
// Sabotaje que lo pone en ROJO: anular la asignación del staleness (isStale && false), con lo que el
// ancla missing queda staleness="none" y el assert de "missing" falla. (Comparte la mutación con
// TestSpecCoverageMarcaRanciedadSinTocarResolucion, que la ataca en el régimen "changed"; el arnés las
// cuenta por nombre de prueba.)
// arnes: archivo="internal/mcp/methods_spec_coverage.go"
// arnes: de="if r, isStale := staleByRef[a.Ref]; isStale {"
// arnes: a="if r, isStale := staleByRef[a.Ref]; isStale && false {"
func TestSpecCoverageAnclaMissingResuelveEnGrafoStale(t *testing.T) {
	s, e, dir := coverageServer(t)
	sembrarDiamante(t, e, dir) // el grafo tiene el nodo de Hoja y sus callers
	crearSpec(t, s, "s-miss", []string{"graph/core.go#Hoja"})
	// Se borra Hoja del DISCO (el archivo queda, sin el símbolo) y NO se reindexa el grafo: el nodo de
	// Hoja sigue en code_graph_nodes. Es el régimen stale que F3-B contempla (D3).
	escribirCodigo(t, dir, "graph/core.go",
		"package core\n\nfunc Rama1() {}\nfunc Rama2() {}\nfunc Raiz() { Rama1(); Rama2() }\n")
	out := coverageForward(t, s, "s-miss")
	if len(out.Anchors) != 1 {
		t.Fatalf("esperaba 1 ancla, dio %d", len(out.Anchors))
	}
	// Disco: el símbolo ya no está ⇒ missing.
	if out.Anchors[0].Staleness != "missing" {
		t.Errorf("símbolo borrado del disco debía marcar staleness=missing, dio %q", out.Anchors[0].Staleness)
	}
	// Grafo (no reindexado): el nodo sigue ⇒ resuelve y mide, ortogonal a la ranciedad.
	if out.Anchors[0].Resolution != "resolved" {
		t.Errorf("el grafo stale todavía resuelve: esperaba resolved, dio %q", out.Anchors[0].Resolution)
	}
	if out.Anchors[0].BlastCount != 3 {
		t.Errorf("Hoja sigue en el grafo con 3 callers: esperaba blast 3, dio %d", out.Anchors[0].BlastCount)
	}
}
