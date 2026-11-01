package specgraph

// Pruebas de query.go: grepSpecs (búsqueda model-free paginada con filtros y
// presupuesto de bytes) y graphSlice (recorte del grafo en tres direcciones).

import (
	"strings"
	"testing"
)

func sliceIDs(nodes []*SpecNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

// TestGrepBasico fija la forma de un match: ruta, línea 1-indexada y snippet
// recortado.
func TestGrepBasico(t *testing.T) {
	entries := []SpecContentEntry{
		{Path: "a.md", Content: "primera\n  objetivo acá  \ntercera"},
	}
	r, err := GrepSpecs(entries, GrepOptions{Pattern: "objetivo"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(r.Matches) != 1 {
		t.Fatalf("quería 1 match, dio %d", len(r.Matches))
	}
	m := r.Matches[0]
	if m.Path != "a.md" || m.Line != 2 || m.Snippet != "objetivo acá" {
		t.Errorf("match = %+v; quería {a.md 2 \"objetivo acá\"}", m)
	}
}

// TestGrepCaseCRyBOM fija tres normalizaciones del snippet: ignore-case por
// defecto, el \r final de CRLF recortado, y el BOM descartado en la línea 0.
func TestGrepCaseCRyBOM(t *testing.T) {
	t.Run("ignore-case por defecto", func(t *testing.T) {
		r, _ := GrepSpecs([]SpecContentEntry{{Path: "a.md", Content: "FOO mayúscula"}}, GrepOptions{Pattern: "foo"})
		if len(r.Matches) != 1 || r.Matches[0].Snippet != "FOO mayúscula" {
			t.Errorf("ignore-case falló: %+v", r.Matches)
		}
	})
	t.Run("recorta el \\r de CRLF", func(t *testing.T) {
		r, _ := GrepSpecs([]SpecContentEntry{{Path: "b.md", Content: "con cr\r"}}, GrepOptions{Pattern: "con cr"})
		if len(r.Matches) != 1 || strings.Contains(r.Matches[0].Snippet, "\r") {
			t.Errorf("no recortó el \\r: %+v", r.Matches)
		}
	})
	t.Run("descarta el BOM en la línea 0", func(t *testing.T) {
		r, _ := GrepSpecs([]SpecContentEntry{{Path: "c.md", Content: bom + "arranca con bom"}}, GrepOptions{Pattern: "arranca"})
		if len(r.Matches) != 1 || r.Matches[0].Snippet != "arranca con bom" {
			t.Errorf("no descartó el BOM: %+v", r.Matches)
		}
	})
}

// TestGrepFiltros fija que cada filtro de frontmatter acota el conjunto: una
// entrada que no matchea el filtro ni se escanea.
func TestGrepFiltros(t *testing.T) {
	entries := []SpecContentEntry{
		{Path: "task.md", Content: "común", Frontmatter: Frontmatter{FieldType: "task-spec", FieldParent: "mod", FieldTags: []string{"urgente"}, FieldDependsOn: []string{"goal"}}},
		{Path: "mod.md", Content: "común", Frontmatter: Frontmatter{FieldType: "module-design"}},
	}
	casos := []struct {
		nombre string
		f      SpecFilters
		want   string // ruta esperada única, o "" si ninguna
	}{
		{"type", SpecFilters{Type: "task-spec"}, "task.md"},
		{"parent", SpecFilters{Parent: "mod"}, "task.md"},
		{"tag", SpecFilters{Tag: "urgente"}, "task.md"},
		{"depends-on", SpecFilters{DependsOn: "goal"}, "task.md"},
		{"type sin match", SpecFilters{Type: "submodule-design"}, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			r, err := GrepSpecs(entries, GrepOptions{SpecFilters: c.f, Pattern: "común"})
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if c.want == "" {
				if len(r.Matches) != 0 {
					t.Errorf("quería 0 matches, dio %+v", r.Matches)
				}
				return
			}
			if len(r.Matches) != 1 || r.Matches[0].Path != c.want {
				t.Errorf("quería 1 match en %s, dio %+v", c.want, r.Matches)
			}
		})
	}
}

// TestGrepOffsetGuard fija que un offset negativo es un error, no un paginado
// silenciosamente roto.
//
// Sabotaje que la hace fallar: que la guarda corra el umbral y deje pasar el -1.
// arnes: archivo="internal/specgraph/query.go"
// arnes: de="if offset < 0 {"
// arnes: a="if offset < -1 {"
func TestGrepOffsetGuard(t *testing.T) {
	_, err := GrepSpecs([]SpecContentEntry{{Path: "a.md", Content: "x"}}, GrepOptions{Pattern: "x", Offset: -1})
	if err == nil {
		t.Error("un offset negativo debía dar error")
	}
}

// TestGrepLimitYMaxBytes fija el paginado: Limit<=0 cae al default (no a "cero
// matches"), y MaxBytes trunca marcando Truncated + NextOffset, garantizando al
// menos un match aunque ese solo match exceda el presupuesto (si no, el paginado
// no avanzaría nunca).
func TestGrepLimitYMaxBytes(t *testing.T) {
	entries := []SpecContentEntry{{Path: "a.md", Content: "foo\nfoo\nfoo"}}

	t.Run("Limit<=0 trae todo, no nada", func(t *testing.T) {
		r, _ := GrepSpecs(entries, GrepOptions{Pattern: "foo", Limit: 0})
		if len(r.Matches) != 3 || r.Truncated {
			t.Errorf("Limit=0 debía traer los 3 matches sin truncar, dio %d trunc=%v", len(r.Matches), r.Truncated)
		}
	})

	t.Run("MaxBytes garantiza >=1 match y pagina", func(t *testing.T) {
		r, _ := GrepSpecs(entries, GrepOptions{Pattern: "foo", MaxBytes: 1})
		if len(r.Matches) != 1 || !r.Truncated || r.NextOffset != 1 {
			t.Errorf("con MaxBytes=1 quería 1 match, Truncated, NextOffset=1; dio %d trunc=%v next=%d", len(r.Matches), r.Truncated, r.NextOffset)
		}
	})
}

// TestGraphSliceSubtree fija que subtree desciende por los hijos (aristas parent
// INVERSAS): desde el root se alcanza a su hijo.
//
// Sabotaje que la hace fallar: que subtree camine las aristas parent HACIA
// ADELANTE (padres) en vez de inversas, y entonces desde un root sin padre no
// baja a ningún hijo.
// arnes: archivo="internal/specgraph/query.go"
// arnes: de="for _, child := range graph.Reverse[FieldParent][id] {"
// arnes: a="for _, child := range graph.Forward[FieldParent][id] {"
func TestGraphSliceSubtree(t *testing.T) {
	g := BuildGraph([]SpecFileEntry{
		nodo("root/SPEC.md", "root", "goal-and-requirements", nil),
		nodo("child/SPEC.md", "child", "task-spec", Frontmatter{FieldParent: "root"}),
	})
	res := GraphSlice(g, SliceOptions{Root: "root", Direction: "subtree"})
	if !contains(sliceIDs(res.Nodes), "child") {
		t.Errorf("subtree del root debía incluir a child; dio %v", sliceIDs(res.Nodes))
	}
}

// TestGraphSliceAncestorsYNeighbors fija las otras dos direcciones: ancestors
// sube por los padres; neighbors cruza una clase de arista (depends-on por
// defecto) en ambos sentidos. Depth<=0 se trata como 1.
func TestGraphSliceAncestorsYNeighbors(t *testing.T) {
	arbol := BuildGraph([]SpecFileEntry{
		nodo("root/SPEC.md", "root", "goal-and-requirements", nil),
		nodo("child/SPEC.md", "child", "task-spec", Frontmatter{FieldParent: "root"}),
	})
	t.Run("ancestors sube al padre", func(t *testing.T) {
		res := GraphSlice(arbol, SliceOptions{Root: "child", Direction: "ancestors", Depth: 0})
		if !contains(sliceIDs(res.Nodes), "root") {
			t.Errorf("ancestors de child debía incluir a root; dio %v", sliceIDs(res.Nodes))
		}
	})

	dag := BuildGraph([]SpecFileEntry{
		nodo("a.md", "a", "task-spec", Frontmatter{FieldDependsOn: []string{"b"}}),
		nodo("b.md", "b", "task-spec", Frontmatter{FieldDependsOn: []string{"c"}}),
		nodo("c.md", "c", "task-spec", nil),
	})
	t.Run("neighbors cruza depends-on en ambos sentidos", func(t *testing.T) {
		res := GraphSlice(dag, SliceOptions{Root: "b", Direction: "neighbors"})
		ids := sliceIDs(res.Nodes)
		if !contains(ids, "a") || !contains(ids, "c") {
			t.Errorf("neighbors de b debía incluir a (inverso) y c (adelante); dio %v", ids)
		}
	})
}

// TestGraphSliceMissing fija que un target referenciado que no existe como nodo
// cae en Missing, no en Nodes.
func TestGraphSliceMissing(t *testing.T) {
	g := BuildGraph([]SpecFileEntry{
		nodo("a.md", "a", "task-spec", Frontmatter{FieldDependsOn: []string{"fantasma"}}),
	})
	res := GraphSlice(g, SliceOptions{Root: "a", Direction: "neighbors"})
	if !contains(res.Missing, "fantasma") {
		t.Errorf("fantasma debía estar en Missing; dio %v", res.Missing)
	}
	if contains(sliceIDs(res.Nodes), "fantasma") {
		t.Errorf("fantasma no debía estar en Nodes: %v", sliceIDs(res.Nodes))
	}
}
