package skillsource

import (
	"reflect"
	"testing"
)

// entradaMerge construye una CatalogEntry mínima para pruebas de merge.
// No incluye Excerpt para simplificar — merge no valida, solo fusiona.
func entradaMerge(id, name string) CatalogEntry {
	return CatalogEntry{
		ID:       id,
		Name:     name,
		Stacks:   []string{"Go"},
		Triggers: []string{"*.go"},
		RulesURL: "https://example.com/" + id + ".md",
		Excerpt:  "Excerpt de " + id,
		Source:   "musubi-catalog-v1",
	}
}

// TestMergeNoCollisions verifica que base e incoming con IDs disjuntos
// producen un merged con todas las entradas, sin colisiones, ordenadas por ID.
func TestMergeNoCollisions(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries:        []CatalogEntry{entradaMerge("b-skill", "Skill B")},
	}
	incoming := Catalog{
		CatalogVersion: 1,
		Entries:        []CatalogEntry{entradaMerge("a-skill", "Skill A")},
	}

	merged, collisions := MergeCatalogs(base, incoming)

	if len(collisions) != 0 {
		t.Errorf("esperaba 0 colisiones, obtuve %v", collisions)
	}
	if len(merged.Entries) != 2 {
		t.Errorf("esperaba 2 entradas, obtuve %d", len(merged.Entries))
	}
	// Deben estar ordenadas por ID ascendente
	if merged.Entries[0].ID != "a-skill" || merged.Entries[1].ID != "b-skill" {
		t.Errorf("entradas fuera de orden: %v", []string{merged.Entries[0].ID, merged.Entries[1].ID})
	}
	if merged.CatalogVersion != 1 {
		t.Errorf("CatalogVersion: esperaba 1, obtuve %d", merged.CatalogVersion)
	}
}

// TestMergeOneCollision verifica que un ID compartido produce un overwrite
// (incoming gana) y una entrada en collisions.
func TestMergeOneCollision(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries:        []CatalogEntry{entradaMerge("go-gin", "Gin Base")},
	}
	incoming := Catalog{
		CatalogVersion: 1,
		Entries:        []CatalogEntry{entradaMerge("go-gin", "Gin Incoming")},
	}

	merged, collisions := MergeCatalogs(base, incoming)

	if len(collisions) != 1 || collisions[0] != "go-gin" {
		t.Errorf("esperaba collisions=[go-gin], obtuve %v", collisions)
	}
	if len(merged.Entries) != 1 {
		t.Errorf("esperaba 1 entrada, obtuve %d", len(merged.Entries))
	}
	if merged.Entries[0].Name != "Gin Incoming" {
		t.Errorf("incoming debía ganar, obtuve name %q", merged.Entries[0].Name)
	}
}

// TestMergeMultipleCollisions verifica N colisiones → N líneas en collisions.
func TestMergeMultipleCollisions(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries: []CatalogEntry{
			entradaMerge("a", "A Base"),
			entradaMerge("b", "B Base"),
			entradaMerge("c", "C Base"),
		},
	}
	incoming := Catalog{
		CatalogVersion: 1,
		Entries: []CatalogEntry{
			entradaMerge("b", "B Incoming"),
			entradaMerge("c", "C Incoming"),
		},
	}

	merged, collisions := MergeCatalogs(base, incoming)

	if len(collisions) != 2 {
		t.Errorf("esperaba 2 colisiones, obtuve %d: %v", len(collisions), collisions)
	}
	// collisions deben estar ordenadas
	if collisions[0] != "b" || collisions[1] != "c" {
		t.Errorf("collisions fuera de orden: %v", collisions)
	}
	// Versiones incoming ganan
	for _, e := range merged.Entries {
		switch e.ID {
		case "b":
			if e.Name != "B Incoming" {
				t.Errorf("b: incoming debía ganar, obtuve %q", e.Name)
			}
		case "c":
			if e.Name != "C Incoming" {
				t.Errorf("c: incoming debía ganar, obtuve %q", e.Name)
			}
		}
	}
}

// TestMergeIntraIncomingDuplicate verifica que duplicados dentro de incoming
// producen last-wins (última entrada gana) y NO generan entrada en collisions.
func TestMergeIntraIncomingDuplicate(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries:        []CatalogEntry{},
	}
	incoming := Catalog{
		CatalogVersion: 1,
		Entries: []CatalogEntry{
			entradaMerge("dup", "Primera"),
			entradaMerge("dup", "Segunda"),
		},
	}

	merged, collisions := MergeCatalogs(base, incoming)

	// No debe haber colisión base/incoming porque base está vacía de ese ID
	if len(collisions) != 0 {
		t.Errorf("duplicado intra-incoming no debe producir colisiones base, obtuve %v", collisions)
	}
	if len(merged.Entries) != 1 {
		t.Errorf("esperaba 1 entrada (last-wins), obtuve %d", len(merged.Entries))
	}
	if merged.Entries[0].Name != "Segunda" {
		t.Errorf("last-wins: esperaba 'Segunda', obtuve %q", merged.Entries[0].Name)
	}
}

// TestMergeEmptyIncoming verifica que incoming vacío produce merged == copia de base.
func TestMergeEmptyIncoming(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries: []CatalogEntry{
			entradaMerge("x", "X"),
			entradaMerge("y", "Y"),
		},
	}
	incoming := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{}}

	merged, collisions := MergeCatalogs(base, incoming)

	if len(collisions) != 0 {
		t.Errorf("esperaba 0 colisiones, obtuve %v", collisions)
	}
	if len(merged.Entries) != 2 {
		t.Errorf("esperaba 2 entradas (copia de base), obtuve %d", len(merged.Entries))
	}
	if merged.CatalogVersion != 1 {
		t.Errorf("CatalogVersion: esperaba 1, obtuve %d", merged.CatalogVersion)
	}
}

// TestMergeVersionReconcileEmptyBase verifica que si base está vacía (sin entradas,
// versión 0), merged toma la versión de incoming.
func TestMergeVersionReconcileEmptyBase(t *testing.T) {
	base := Catalog{CatalogVersion: 0, Entries: []CatalogEntry{}}
	incoming := Catalog{CatalogVersion: 2, Entries: []CatalogEntry{entradaMerge("x", "X")}}

	merged, _ := MergeCatalogs(base, incoming)

	if merged.CatalogVersion != 2 {
		t.Errorf("base vacía: esperaba CatalogVersion=2 (de incoming), obtuve %d", merged.CatalogVersion)
	}
}

// TestMergeVersionReconcileBaseWins verifica que si base tiene entradas,
// merged toma la versión de base (aunque incoming difiera).
func TestMergeVersionReconcileBaseWins(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries:        []CatalogEntry{entradaMerge("a", "A")},
	}
	incoming := Catalog{
		CatalogVersion: 2,
		Entries:        []CatalogEntry{entradaMerge("b", "B")},
	}

	merged, _ := MergeCatalogs(base, incoming)

	if merged.CatalogVersion != 1 {
		t.Errorf("base no vacía: esperaba CatalogVersion=1 (de base), obtuve %d", merged.CatalogVersion)
	}
}

// TestMergeVersionsMatch verifica que versiones iguales en base e incoming
// producen esa misma versión en merged.
func TestMergeVersionsMatch(t *testing.T) {
	base := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{entradaMerge("a", "A")}}
	incoming := Catalog{CatalogVersion: 1, Entries: []CatalogEntry{entradaMerge("b", "B")}}

	merged, _ := MergeCatalogs(base, incoming)

	if merged.CatalogVersion != 1 {
		t.Errorf("esperaba CatalogVersion=1, obtuve %d", merged.CatalogVersion)
	}
}

// TestMergeInputImmutability verifica que base.Entries e incoming.Entries
// no son modificados por MergeCatalogs.
func TestMergeInputImmutability(t *testing.T) {
	baseEntries := []CatalogEntry{entradaMerge("a", "A"), entradaMerge("b", "B")}
	incomingEntries := []CatalogEntry{entradaMerge("b", "B Nuevo"), entradaMerge("c", "C")}

	base := Catalog{CatalogVersion: 1, Entries: baseEntries}
	incoming := Catalog{CatalogVersion: 1, Entries: incomingEntries}

	// Copias antes de la llamada
	baseAntes := make([]CatalogEntry, len(base.Entries))
	copy(baseAntes, base.Entries)
	incomingAntes := make([]CatalogEntry, len(incoming.Entries))
	copy(incomingAntes, incoming.Entries)

	MergeCatalogs(base, incoming)

	// Verificar que los slices originales no cambiaron
	if !reflect.DeepEqual(base.Entries, baseAntes) {
		t.Error("base.Entries fue modificado por MergeCatalogs")
	}
	if !reflect.DeepEqual(incoming.Entries, incomingAntes) {
		t.Error("incoming.Entries fue modificado por MergeCatalogs")
	}
	// Verificar que el slice header no fue reutilizado (len intacto)
	if len(base.Entries) != 2 {
		t.Errorf("len(base.Entries) cambió: esperaba 2, obtuve %d", len(base.Entries))
	}
	if len(incoming.Entries) != 2 {
		t.Errorf("len(incoming.Entries) cambió: esperaba 2, obtuve %d", len(incoming.Entries))
	}
}

// TestMergeDeterministicOrder verifica que entradas en orden mixto producen
// output ordenado ascendente por ID.
func TestMergeDeterministicOrder(t *testing.T) {
	base := Catalog{
		CatalogVersion: 1,
		Entries: []CatalogEntry{
			entradaMerge("z-skill", "Z"),
			entradaMerge("m-skill", "M"),
		},
	}
	incoming := Catalog{
		CatalogVersion: 1,
		Entries: []CatalogEntry{
			entradaMerge("a-skill", "A"),
			entradaMerge("f-skill", "F"),
		},
	}

	merged, _ := MergeCatalogs(base, incoming)

	expected := []string{"a-skill", "f-skill", "m-skill", "z-skill"}
	got := make([]string, len(merged.Entries))
	for i, e := range merged.Entries {
		got[i] = e.ID
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("orden incorrecto: esperaba %v, obtuve %v", expected, got)
	}
}
