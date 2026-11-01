package specgraph

// Pruebas de validate.go: los 3 chequeos portados de thinkrail (enlaces
// colgantes, ids duplicados, ciclos de parent) más el delta model-free de Musubi
// (UnknownTypes). El orden de salida es determinista porque recorre NodeOrder.

import (
	"reflect"
	"testing"
)

// TestValidateDanglingLinks fija que un enlace a un target que NO existe como
// nodo se denuncia, y que un enlace a un nodo existente NO se denuncia.
//
// Sabotaje que la hace fallar: que la condición se invierta y denuncie los
// enlaces que SÍ resuelven en vez de los colgantes.
// arnes: archivo="internal/specgraph/validate.go"
// arnes: de="if _, ok := graph.Nodes[target]; !ok {"
// arnes: a="if _, ok := graph.Nodes[target]; ok {"
func TestValidateDanglingLinks(t *testing.T) {
	g := BuildGraph([]SpecFileEntry{
		nodo("a/SPEC.md", "a", "task-spec", Frontmatter{FieldDependsOn: []string{"b", "fantasma"}}),
		nodo("b/SPEC.md", "b", "task-spec", nil),
	})
	rep := ValidateGraph(g)
	if len(rep.DanglingLinks) != 1 {
		t.Fatalf("quería 1 enlace colgante (a→fantasma), dio %d: %+v", len(rep.DanglingLinks), rep.DanglingLinks)
	}
	dl := rep.DanglingLinks[0]
	if dl.From != "a" || dl.Target != "fantasma" || dl.Kind != FieldDependsOn || dl.FromPath != "a/SPEC.md" {
		t.Errorf("enlace colgante mal reportado: %+v", dl)
	}
}

// TestValidateDuplicateIDs fija que dos archivos con el mismo id se denuncian con
// ambas rutas.
func TestValidateDuplicateIDs(t *testing.T) {
	g := BuildGraph([]SpecFileEntry{
		nodo("uno/SPEC.md", "dup", "task-spec", nil),
		nodo("dos/SPEC.md", "dup", "task-spec", nil),
	})
	rep := ValidateGraph(g)
	if len(rep.DuplicateIDs) != 1 || rep.DuplicateIDs[0].ID != "dup" {
		t.Fatalf("quería 1 id duplicado (dup), dio %+v", rep.DuplicateIDs)
	}
	if !reflect.DeepEqual(rep.DuplicateIDs[0].Paths, []string{"uno/SPEC.md", "dos/SPEC.md"}) {
		t.Errorf("rutas del duplicado = %v; quería ambas", rep.DuplicateIDs[0].Paths)
	}
}

// TestValidateParentCycles fija que un ciclo de parent (a→b→a) se detecta y se
// deduplica: el mismo anillo alcanzado desde distintos arranques se reporta una
// sola vez, por su conjunto de ids ordenado.
func TestValidateParentCycles(t *testing.T) {
	g := BuildGraph([]SpecFileEntry{
		nodo("a/SPEC.md", "a", "task-spec", Frontmatter{FieldParent: "b"}),
		nodo("b/SPEC.md", "b", "task-spec", Frontmatter{FieldParent: "a"}),
	})
	rep := ValidateGraph(g)
	if len(rep.ParentCycles) != 1 {
		t.Fatalf("quería 1 ciclo deduplicado, dio %d: %+v", len(rep.ParentCycles), rep.ParentCycles)
	}
	got := append([]string(nil), rep.ParentCycles[0].IDs...)
	sortStrings(got)
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("ids del ciclo (ordenados) = %v; quería [a b]", got)
	}
}

// TestIsValid fija que IsValid es true sólo con los 3 chequeos portados limpios,
// y que ignora los deltas de Musubi (un type desconocido no lo pone en false: eso
// lo decide el handler aparte).
func TestIsValid(t *testing.T) {
	limpio := BuildGraph([]SpecFileEntry{
		nodo("a/SPEC.md", "a", "task-spec", nil),
	})
	if !IsValid(ValidateGraph(limpio)) {
		t.Error("un grafo sin problemas estructurales debía ser válido")
	}

	colgante := BuildGraph([]SpecFileEntry{
		nodo("a/SPEC.md", "a", "task-spec", Frontmatter{FieldDependsOn: []string{"fantasma"}}),
	})
	if IsValid(ValidateGraph(colgante)) {
		t.Error("un grafo con un enlace colgante NO debía ser válido")
	}
}

// TestUnknownTypes fija el delta R3/R12: un nodo cuyo type no está en SpecTypes se
// denuncia, pero uno con un type conocido no.
func TestUnknownTypes(t *testing.T) {
	g := BuildGraph([]SpecFileEntry{
		nodo("ok/SPEC.md", "ok", "task-spec", nil),
		nodo("raro/SPEC.md", "raro", "inventado", nil),
	})
	ut := UnknownTypes(g)
	if len(ut) != 1 || ut[0].ID != "raro" || ut[0].Type != "inventado" {
		t.Errorf("quería 1 type desconocido (raro/inventado), dio %+v", ut)
	}
}
