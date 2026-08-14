package codeintel

import "testing"

// Tests de las llamadas CROSS-PAQUETE (Track 20 · F8-A).
//
// Existen por una medición, no por una idea: `musubi_impact` sobre
// `internal/codeintel/graph.go#func:DerivePackage` devolvía 9 callers y los 9 eran tests de su
// propio paquete. El único caller de producción —`internal/mcp/methods_codegraph.go`— no aparecía.
// La herramienta que contesta "¿qué se rompe si cambio esto?" estaba ciega justo en el salto que
// importa, y no fallaba: contestaba con cara de respuesta correcta.
//
// El fixture de graph_test.go ya ejercita los dos lados que hacen falta y por eso se reusa:
// `util.Help()` es cross-paquete IN-MODULE, y `fmt.Println` es la llamada externa que sirve de
// CONTROL NEGATIVO — sin ella, un test que "ve" el cross-paquete no distinguiría entre resolver
// imports del módulo y resolver cualquier selector.

// paqueteUtil es el paquete DESTINO del fixture. Se deriva de verdad en vez de fabricar nodos a
// mano: así el índice se arma con las mismas node_keys que produciría el indexador real, y el test
// no puede pasar por coincidir con una key inventada.
var paqueteUtil = map[string]string{
	"internal/util/util.go": `package util

func Help() {}

func noExportada() {}
`,
}

func pendienteHacia(g PackageGraph, name string) *PendingCall {
	for i := range g.PendingCalls {
		if g.PendingCalls[i].Name == name {
			return &g.PendingCalls[i]
		}
	}
	return nil
}

func indiceDelFixture(t *testing.T) *ModuleIndex {
	t.Helper()
	idx := NewModuleIndex(modPath)
	idx.Add(DerivePackage("internal/util", paqueteUtil, modPath).Nodes)
	return idx
}

// R1 — DerivePackage recolecta el call-site calificado in-module y lo emite como PENDIENTE, no como
// arista. Las dos mitades importan: si emitiera la arista estaría inventando una key que no puede
// conocer, y si no emitiera nada no habría qué resolver después.
func TestDerivePackage_EmitePendienteCrossPaquete(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)

	pc := pendienteHacia(g, "Help")
	if pc == nil {
		t.Fatalf("no se emitió el pendiente hacia util.Help; pendientes = %+v", g.PendingCalls)
	}
	if pc.FromKey != "pkg/a.go#func:Alpha" {
		t.Errorf("FromKey = %q, esperaba el símbolo que hace la llamada", pc.FromKey)
	}
	if pc.ImportPath != modPath+"/internal/util" {
		t.Errorf("ImportPath = %q, esperaba el import ya resuelto desde el alias", pc.ImportPath)
	}
	if pc.SrcPath != "pkg/a.go" {
		t.Errorf("SrcPath = %q, esperaba el archivo del CALLER: es quien tiene que poseer la arista", pc.SrcPath)
	}

	// Y todavía NO hay arista: DerivePackage sola no puede resolverla.
	for _, e := range g.Edges {
		if e.Kind == EdgeCalls && e.ToKey != "pkg/a.go#func:beta" {
			t.Errorf("DerivePackage emitió una arista CALLS que no debería poder resolver: %s → %s", e.FromKey, e.ToKey)
		}
	}
}

// R3 — CONTROL NEGATIVO, y es el test que le da valor a todos los demás. Una llamada a stdlib NO
// genera pendiente. Sin esto, "el cross-paquete funciona" no distinguiría entre resolver imports del
// módulo y tragarse cualquier selector.
func TestDerivePackage_LoExternoNoGeneraPendiente(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)

	if pc := pendienteHacia(g, "Println"); pc != nil {
		t.Errorf("fmt.Println generó un pendiente (%+v): el filtro in-module no discrimina", *pc)
	}
	// Control POSITIVO del control: el fixture sí tiene la llamada externa, así que la ausencia de
	// pendiente significa "filtrado" y no "no había nada que filtrar".
	if !hasEdge(g, "pkg/a.go", PackageKey("fmt"), EdgeImports) {
		t.Fatal("el fixture perdió el import de fmt: sin él este test pasa por el motivo equivocado")
	}
	// Y la dependencia a nivel paquete se sigue representando como siempre.
	if n := findNode(g, PackageKey("fmt")); n == nil || !n.External {
		t.Error("el nodo de paquete externo fmt tiene que seguir existiendo y marcado External")
	}
}

// R2 — el alias se resuelve por los imports del PROPIO archivo. Dos archivos del mismo paquete
// pueden aliasear distinto el mismo import; si la tabla fuera del paquete, el segundo pisaría al
// primero y uno de los dos resolvería mal (o dejaría de resolver).
func TestDerivePackage_AliasPorArchivo(t *testing.T) {
	files := map[string]string{
		"pkg/con_alias.go": `package pkg

import u "example.com/mod/internal/util"

func ConAlias() { u.Help() }
`,
		"pkg/sin_alias.go": `package pkg

import "example.com/mod/internal/util"

func SinAlias() { util.Help() }
`,
	}
	g := DerivePackage("pkg", files, modPath)

	if len(g.PendingCalls) != 2 {
		t.Fatalf("esperaba 2 pendientes (uno por archivo), obtuve %d: %+v", len(g.PendingCalls), g.PendingCalls)
	}
	for _, pc := range g.PendingCalls {
		if pc.ImportPath != modPath+"/internal/util" {
			t.Errorf("%s resolvió a %q; los dos alias apuntan al mismo import path", pc.FromKey, pc.ImportPath)
		}
	}
}

// R4 + R6 — el cierre del círculo: tras resolver contra el índice, la arista SÍ existe y apunta a la
// node_key real del otro paquete. Es lo que `musubi_impact` necesita para dejar de mentir.
func TestResolveCrossPackageCalls_CreaLaArista(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)
	edges := ResolveCrossPackageCalls(g.PendingCalls, indiceDelFixture(t))

	if len(edges) != 1 {
		t.Fatalf("esperaba 1 arista cross-paquete, obtuve %d: %+v", len(edges), edges)
	}
	e := edges[0]
	if e.FromKey != "pkg/a.go#func:Alpha" || e.ToKey != "internal/util/util.go#func:Help" {
		t.Errorf("arista = %s → %s, esperaba Alpha → Help", e.FromKey, e.ToKey)
	}
	if e.Kind != EdgeCalls || e.Confidence != 1.0 || e.Provenance != ProvExtracted {
		t.Errorf("arista mal tipada: kind=%s conf=%v prov=%s", e.Kind, e.Confidence, e.Provenance)
	}
	// SrcPath = archivo del CALLER. No es cosmético: el refresco borra por SrcPath, así que es lo
	// que hace que re-indexar el paquete que cambió recalcule su propia arista cross.
	if e.SrcPath != "pkg/a.go" {
		t.Errorf("SrcPath = %q; tiene que ser el archivo del caller o el refresco incremental no la posee", e.SrcPath)
	}
}

// R5 — AMBIGÜEDAD: dos funcs homónimas en el mismo paquete (el caso real son build tags) ⇒ NO se
// emite arista. Es el defecto más fácil de introducir: un map[nombre]key se sobrescribe en silencio
// y devuelve el último visto, o sea INVENTA una resolución con cara de correcta.
func TestResolveCrossPackageCalls_AmbiguoSeOmite(t *testing.T) {
	ambiguo := map[string]string{
		"internal/util/a_linux.go":   "package util\n\nfunc Help() {}\n",
		"internal/util/a_windows.go": "package util\n\nfunc Help() {}\n",
	}
	idx := NewModuleIndex(modPath)
	idx.Add(DerivePackage("internal/util", ambiguo, modPath).Nodes)

	g := DerivePackage("pkg", fixtureFiles, modPath)
	if edges := ResolveCrossPackageCalls(g.PendingCalls, idx); len(edges) != 0 {
		t.Errorf("con dos candidatos homónimos se eligió uno en vez de omitir: %+v", edges)
	}

	// Control: con UN solo candidato el mismo código sí resuelve. Sin esto, el test pasaría igual
	// si la resolución estuviera rota del todo.
	if edges := ResolveCrossPackageCalls(g.PendingCalls, indiceDelFixture(t)); len(edges) != 1 {
		t.Errorf("con un candidato único esperaba 1 arista, obtuve %d", len(edges))
	}
}

// R5 (mitad ausente) — un símbolo que no está en el índice se omite sin ruido ni pánico. Pasa de
// verdad: un paquete todavía no indexado, o una func que se borró.
func TestResolveCrossPackageCalls_AusenteSeOmite(t *testing.T) {
	g := DerivePackage("pkg", fixtureFiles, modPath)
	vacio := NewModuleIndex(modPath)
	if edges := ResolveCrossPackageCalls(g.PendingCalls, vacio); len(edges) != 0 {
		t.Errorf("con el índice vacío no debería salir ninguna arista, salieron %+v", edges)
	}
	if edges := ResolveCrossPackageCalls(nil, indiceDelFixture(t)); len(edges) != 0 {
		t.Errorf("sin pendientes no hay aristas, salieron %+v", edges)
	}
	if edges := ResolveCrossPackageCalls(g.PendingCalls, nil); len(edges) != 0 {
		t.Errorf("sin índice no hay aristas, salieron %+v", edges)
	}
}

// El índice sólo indexa funcs TOP-LEVEL, y sólo del módulo. Un import externo no tiene directorio
// que mirar, y por eso no puede resolverse aunque su nombre coincida.
func TestModuleIndex_AlcanceYExternos(t *testing.T) {
	idx := indiceDelFixture(t)

	if _, ok := idx.LookupFunc(modPath+"/internal/util", "Help"); !ok {
		t.Error("Help es func top-level exportada del módulo: tiene que resolver")
	}
	if _, ok := idx.LookupFunc(modPath+"/internal/util", "noExportada"); !ok {
		t.Error("el índice no filtra por exportación: una llamada intra-módulo a una minúscula es válida si existe")
	}
	if _, ok := idx.LookupFunc("strings", "HasPrefix"); ok {
		t.Error("un import externo no debe resolver nunca: su grafo no es nuestro")
	}
	if _, ok := idx.LookupFunc(modPath+"/internal/util", "NoExiste"); ok {
		t.Error("un nombre inexistente no puede resolver")
	}
}

// DirForImportPath es la traducción import-path → directorio, y tiene un caso que se olvida: la
// RAÍZ del módulo, que es "." y no cadena vacía (así la nombra el resto del indexador).
func TestDirForImportPath(t *testing.T) {
	casos := []struct {
		importPath string
		wantDir    string
		wantOK     bool
	}{
		{modPath, ".", true},
		{modPath + "/internal/util", "internal/util", true},
		{modPath + "/cmd", "cmd", true},
		{"strings", "", false},
		{"github.com/otro/mod/pkg", "", false},
		// Prefijo que NO es frontera de segmento: `example.com/modificado` empieza con `example.com/mod`
		// pero es OTRO módulo. Si la comprobación fuera un HasPrefix pelado, entraría.
		{modPath + "ificado/pkg", "", false},
	}
	for _, c := range casos {
		dir, ok := DirForImportPath(c.importPath, modPath)
		if ok != c.wantOK || dir != c.wantDir {
			t.Errorf("DirForImportPath(%q) = (%q, %v), esperaba (%q, %v)", c.importPath, dir, ok, c.wantDir, c.wantOK)
		}
	}
}

// ImportPathsOf le dice al camino incremental qué paquetes ir a buscar a la base: tiene que
// deduplicar, o la consulta pediría el mismo directorio una vez por call-site.
func TestImportPathsOf(t *testing.T) {
	pend := []PendingCall{
		{ImportPath: "b/dos"}, {ImportPath: "a/uno"}, {ImportPath: "b/dos"}, {ImportPath: "a/uno"},
	}
	got := ImportPathsOf(pend)
	if len(got) != 2 || got[0] != "a/uno" || got[1] != "b/dos" {
		t.Errorf("ImportPathsOf = %v, esperaba [a/uno b/dos] deduplicado y ordenado", got)
	}
	if ImportPathsOf(nil) != nil {
		t.Error("sin pendientes no hay paths que consultar")
	}
}
