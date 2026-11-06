package mcp

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// T12 — R1 (surfacing): detectSpecContradictions, disparado por una proyección de spec recién escrita,
// surfacea la creencia contradictoria Y enuncia LOS DOS desenlaces. Ocultar una creencia CORRECTA sería
// el error, así que el texto debe nombrar tanto "la spec gana → musubi_judge supersedes" como "la
// creencia tiene razón → actualizá el SPEC.md, NO la ocultes".
//
// Sabotaje que lo pone en rojo: borrar la mitad "la creencia tiene razón" (el fragmento con
// `musubi_spec_update`) del texto de surfacing → la aserción del segundo desenlace falla.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de=" si la CREENCIA tiene razón (la spec quedó atrás), NO la ocultes: actualizá el SPEC.md con `musubi_spec_update`."
// arnes: a=""
func TestF5SurfacingEnunciaLosDosDesenlaces(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	// Proyección de una spec `shared` (como F4a) y una creencia humana contradictoria con alto solape
	// léxico bajo el mismo topic, para que la detección la encuentre por el piso léxico (model-free).
	if err := s.engine.ProjectSpecShared("s1", "", "Usamos MySQL como base de datos principal del sistema.", nil); err != nil {
		t.Fatalf("ProjectSpecShared: %v", err)
	}
	if err := s.engine.SaveObservation("cre", "spec/s1", "Usamos PostgreSQL como base de datos principal del sistema.", nil); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}

	surf := s.detectSpecContradictions("s1")
	if surf == "" {
		t.Fatal("R1: el surfacing no debe ser vacío cuando hay una creencia contradictoria")
	}
	// Primer desenlace: la spec gana → arbitrar con musubi_judge.
	if !strings.Contains(surf, "musubi_judge") {
		t.Errorf("R1: el surfacing debe nombrar el desenlace 'la spec gana' (musubi_judge), obtuve:\n%s", surf)
	}
	// Segundo desenlace (la mitad que el sabotaje borra): la creencia tiene razón → actualizar el SPEC.md.
	if !strings.Contains(surf, "musubi_spec_update") {
		t.Errorf("R1: el surfacing debe nombrar el desenlace 'la creencia tiene razón' (musubi_spec_update), obtuve:\n%s", surf)
	}
	// Y debe identificar la relación a arbitrar, con la creencia como target.
	if !strings.Contains(surf, "relation_id=") || !strings.Contains(surf, "creencia=cre") {
		t.Errorf("R1: el surfacing debe exponer el relation_id y la creencia target, obtuve:\n%s", surf)
	}
}
