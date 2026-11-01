package specgraph

// Pruebas de graph.go: de las entradas detectadas sale un grafo cuya identidad
// de nodo es el `id` (no la ruta), donde el PRIMER archivo gana ante ids
// duplicados, y con adyacencia hacia adelante e inversa por cada clase de enlace.

import (
	"reflect"
	"testing"
)

// nodo arma una entrada con id+type y cualquier campo extra (parent, depends-on…).
// Los campos lista DEBEN ser []string: List() sólo reconoce []string y string, así
// que un []any quedaría invisible para el grafo.
func nodo(path, id, typ string, extra Frontmatter) SpecFileEntry {
	fm := Frontmatter{FieldID: id, FieldType: typ}
	for k, v := range extra {
		fm[k] = v
	}
	return SpecFileEntry{Path: path, Frontmatter: fm}
}

// TestBuildGraphSkipsNonSpecs fija que una entrada sin id o sin type NO entra al
// grafo: es la frontera entre una spec y el diario SDD (R1), llevada al build.
//
// Sabotaje que la hace fallar: que el salteo pida AMBOS ausentes (AND) en vez de
// cualquiera (OR), y entonces una entrada con id pero sin type se cuela como nodo.
// arnes: archivo="internal/specgraph/graph.go"
// arnes: de="if !okID || !okType {"
// arnes: a="if !okID && !okType {"
func TestBuildGraphSkipsNonSpecs(t *testing.T) {
	entries := []SpecFileEntry{
		nodo("ok/SPEC.md", "real", "task-spec", nil),
		{Path: "solo-id/SPEC.md", Frontmatter: Frontmatter{FieldID: "x"}},
		{Path: "solo-type/SPEC.md", Frontmatter: Frontmatter{FieldType: "task-spec"}},
		{Path: "vacio/SPEC.md", Frontmatter: Frontmatter{}},
		{Path: "nil/SPEC.md", Frontmatter: nil},
	}
	g := BuildGraph(entries)
	if len(g.Nodes) != 1 || g.Nodes["real"] == nil {
		t.Fatalf("sólo la entrada con id+type debía ser nodo; NodeOrder=%v", g.NodeOrder)
	}
	if !reflect.DeepEqual(g.NodeOrder, []string{"real"}) {
		t.Errorf("NodeOrder = %v; quería [real]", g.NodeOrder)
	}
}

// TestBuildGraphFirstFileWinsOnDuplicateId fija la regla de desempate: ante dos
// archivos con el mismo id, el PRIMERO es el nodo canónico, pero AMBAS rutas
// quedan registradas en DuplicateIds para que validate las denuncie.
func TestBuildGraphFirstFileWinsOnDuplicateId(t *testing.T) {
	entries := []SpecFileEntry{
		nodo("primero/SPEC.md", "dup", "task-spec", Frontmatter{FieldTitle: "El primero"}),
		nodo("segundo/SPEC.md", "dup", "module-design", Frontmatter{FieldTitle: "El segundo"}),
	}
	g := BuildGraph(entries)
	if g.Nodes["dup"].Path != "primero/SPEC.md" || g.Nodes["dup"].Title != "El primero" {
		t.Errorf("el primer archivo debía ganar; dio Path=%q Title=%q", g.Nodes["dup"].Path, g.Nodes["dup"].Title)
	}
	if !reflect.DeepEqual(g.DuplicateIds["dup"], []string{"primero/SPEC.md", "segundo/SPEC.md"}) {
		t.Errorf("DuplicateIds[dup] = %v; quería ambas rutas en orden", g.DuplicateIds["dup"])
	}
}

// TestBuildGraphAdjacency fija que parent y los campos de lista generan aristas,
// con adyacencia hacia adelante (from→to) e inversa (to→from), y que NodeOrder
// respeta el orden de primer avistaje.
func TestBuildGraphAdjacency(t *testing.T) {
	entries := []SpecFileEntry{
		nodo("goal/SPEC.md", "goal", "goal-and-requirements", nil),
		nodo("mod/SPEC.md", "mod", "module-design", Frontmatter{FieldParent: "goal"}),
		nodo("task/SPEC.md", "task", "task-spec", Frontmatter{
			FieldParent:    "mod",
			FieldDependsOn: []string{"goal"},
		}),
	}
	g := BuildGraph(entries)

	if !reflect.DeepEqual(g.NodeOrder, []string{"goal", "mod", "task"}) {
		t.Errorf("NodeOrder = %v; quería [goal mod task]", g.NodeOrder)
	}
	// parent: mod→goal, task→mod. forward[parent][mod] = [goal]; reverse[parent][goal] = [mod].
	if got := g.Forward[FieldParent]["mod"]; !reflect.DeepEqual(got, []string{"goal"}) {
		t.Errorf("forward parent de mod = %v; quería [goal]", got)
	}
	if got := g.Reverse[FieldParent]["goal"]; !reflect.DeepEqual(got, []string{"mod"}) {
		t.Errorf("reverse parent de goal = %v; quería [mod]", got)
	}
	// depends-on: task→goal.
	if got := g.Forward[FieldDependsOn]["task"]; !reflect.DeepEqual(got, []string{"goal"}) {
		t.Errorf("forward depends-on de task = %v; quería [goal]", got)
	}
	if got := g.Reverse[FieldDependsOn]["goal"]; !reflect.DeepEqual(got, []string{"task"}) {
		t.Errorf("reverse depends-on de goal = %v; quería [task]", got)
	}
	// 3 aristas en total (2 parent + 1 depends-on).
	if len(g.Edges) != 3 {
		t.Errorf("Edges = %d; quería 3 (2 parent + 1 depends-on): %+v", len(g.Edges), g.Edges)
	}
}
