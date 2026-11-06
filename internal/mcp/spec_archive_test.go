package mcp

// Tests de projectSDDProcessDelta: la capa de proyección (spec-graph F4, H2) que adapta el delta spec
// de PROCESO del SDD a forma canónica ANTES de specgraph.MergeSpecInto. Fijan la corrección de los dos
// defectos que encontró la review adversarial del archive:
//   - H1: las claves de proceso (artifact/schema_version/change) NO deben contaminar la canónica.
//   - H7: el status de proceso del delta (draft) NO debe pisar/degradar el status de la canónica.
// Cada test sabotea EXACTAMENTE el invariante que declara (anotación `// arnes:`), model-free y puro.

import (
	"strings"
	"testing"

	"musubi/internal/specgraph"
)

// procesoDeltaF4 es el frontmatter de PROCESO tal cual lo emite el flujo SDD (la forma real que vio la
// review en specs/spec-graph-f4-federacion/spec.md): artifact/schema_version/change/status, SIN id/type.
const procesoDeltaF4 = "---\n" +
	"artifact: spec\n" +
	"schema_version: \"1.0\"\n" +
	"change: spec-graph-f4-federacion\n" +
	"status: draft\n" +
	"---\n\n" +
	"# Especificación — federación opt-in de specs\n\n" +
	"## Requisitos\n\n" +
	"- R1. El vocabulario DEBE reconocer `shared`.\n"

// TestProyeccionPromocionPodaProcesoEInyectaIdentidad — PROMOCIÓN (H1): proyectar un delta de proceso y
// promoverlo (canónica inexistente) produce una canónica VÁLIDA (id/type/status inyectados) y SIN una
// sola clave de proceso. Es el caso primario del archive hoy: no existe ninguna SPEC.md todavía.
//
// Sabotaje que lo pone en ROJO: neutralizar la poda de claves de proceso (el loop que las borra
// pasa a un no-op). Entonces artifact/schema_version/change sobreviven y contaminan la canónica (H1).
// arnes: archivo="internal/mcp/spec_archive.go"
// arnes: de="	for _, k := range sddProcessKeys {\n		delete(clean, k)\n	}"
// arnes: a="	_ = sddProcessKeys"
func TestProyeccionPromocionPodaProcesoEInyectaIdentidad(t *testing.T) {
	proyectado, err := projectSDDProcessDelta([]byte(procesoDeltaF4), false, "spec-graph", "module-design", "active")
	if err != nil {
		t.Fatalf("proyectar para promoción: %v", err)
	}
	merged, conflicts, err := specgraph.MergeSpecInto(nil, proyectado)
	if err != nil {
		t.Fatalf("promover: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("promoción: no debía haber conflictos, hay %d", len(conflicts))
	}
	fm := specgraph.ParseFile(string(merged)).Frontmatter
	// Identidad y status inyectados: la canónica nace válida (R13).
	if v, _ := specgraph.Scalar(fm, specgraph.FieldID); v != "spec-graph" {
		t.Errorf("promoción: id=%q, esperaba la identidad inyectada del módulo", v)
	}
	if v, _ := specgraph.Scalar(fm, specgraph.FieldType); v != "module-design" {
		t.Errorf("promoción: type=%q, esperaba el type inyectado del módulo", v)
	}
	if v, _ := specgraph.Scalar(fm, specgraph.FieldStatus); v != "active" {
		t.Errorf("promoción: status=%q, esperaba el status de promoción explícito", v)
	}
	// H1: NINGUNA clave de proceso sobrevive a la proyección.
	for _, k := range []string{sddFieldArtifact, sddFieldSchemaVersion, sddFieldChange} {
		if _, hay := fm[k]; hay {
			t.Errorf("promoción (H1): la clave de proceso %q contaminó la canónica", k)
		}
	}
	// El cuerpo del delta se conserva entero.
	if !strings.Contains(string(merged), "## Requisitos") {
		t.Errorf("promoción: el cuerpo del delta debía conservarse; salida:\n%s", merged)
	}
}

// TestProyeccionMergeStatusCanonicoAutoritativo — MERGE (H7): fusionar un delta de proceso status:draft
// en una canónica status:active NO degrada el status. La canónica es autoritativa sobre su ciclo de vida.
//
// Sabotaje que lo pone en ROJO: quitar el delete del status del delta. El status:draft del proceso
// viaja al merge y degrada el status:active de la canónica (H7).
// arnes: archivo="internal/mcp/spec_archive.go"
// arnes: de="	delete(clean, specgraph.FieldStatus)"
// arnes: a=""
func TestProyeccionMergeStatusCanonicoAutoritativo(t *testing.T) {
	canon := []byte("---\nid: spec-graph\ntype: module-design\nstatus: active\ntitle: Spec-Graph\n---\n# Spec-Graph\n\n## Panorama\n\ntexto base\n")
	proyectado, err := projectSDDProcessDelta([]byte(procesoDeltaF4), true, "", "", "")
	if err != nil {
		t.Fatalf("proyectar para merge: %v", err)
	}
	merged, _, err := specgraph.MergeSpecInto(canon, proyectado)
	if err != nil {
		t.Fatalf("fusionar: %v", err)
	}
	fm := specgraph.ParseFile(string(merged)).Frontmatter
	if v, _ := specgraph.Scalar(fm, specgraph.FieldStatus); v != "active" {
		t.Errorf("merge (H7): status=%q, el delta draft NO debía degradar la canónica active", v)
	}
}

// TestProyeccionMergeNoContaminaConProceso — MERGE (H1): fusionar el delta de proceso tampoco mete las
// claves de proceso en la canónica existente, y conserva su cuerpo mientras agrega el del delta.
//
// Sabotaje que lo pone en ROJO: neutralizar la poda de claves de proceso (el loop que las borra
// pasa a un no-op). En el merge, artifact/schema_version/change contaminan la canónica existente (H1).
// arnes: archivo="internal/mcp/spec_archive.go"
// arnes: de="	for _, k := range sddProcessKeys {\n		delete(clean, k)\n	}"
// arnes: a="	_ = sddProcessKeys"
func TestProyeccionMergeNoContaminaConProceso(t *testing.T) {
	canon := []byte("---\nid: spec-graph\ntype: module-design\nstatus: active\ntitle: Spec-Graph\n---\n# Spec-Graph\n\n## Panorama\n\ntexto base\n")
	proyectado, err := projectSDDProcessDelta([]byte(procesoDeltaF4), true, "", "", "")
	if err != nil {
		t.Fatalf("proyectar para merge: %v", err)
	}
	merged, _, err := specgraph.MergeSpecInto(canon, proyectado)
	if err != nil {
		t.Fatalf("fusionar: %v", err)
	}
	fm := specgraph.ParseFile(string(merged)).Frontmatter
	for _, k := range []string{sddFieldArtifact, sddFieldSchemaVersion, sddFieldChange} {
		if _, hay := fm[k]; hay {
			t.Errorf("merge (H1): la clave de proceso %q contaminó la canónica", k)
		}
	}
	if !strings.Contains(string(merged), "texto base") {
		t.Errorf("merge: la prosa de la canónica debía conservarse")
	}
	if !strings.Contains(string(merged), "## Requisitos") {
		t.Errorf("merge: la sección nueva del delta debía agregarse")
	}
}

// TestProyeccionPromocionExigeIdentidad — R13: promover (canónica inexistente) un delta de proceso SIN
// pasar id+type del módulo es error, no una canónica inválida silenciosa.
//
// Sabotaje que lo pone en ROJO: quitar la guarda que exige id+type del módulo en promoción. Promover
// sin identidad NO falla y la canónica nace inválida en silencio (R13).
// arnes: archivo="internal/mcp/spec_archive.go"
// arnes: de="	if moduleID == \"\" || moduleType == \"\" {\n		return nil, fmt.Errorf(\"promover el delta a una SPEC.md canónica nueva exige id y type del módulo (R13): el spec.md de proceso del SDD no los trae\")\n	}"
// arnes: a=""
func TestProyeccionPromocionExigeIdentidad(t *testing.T) {
	if _, err := projectSDDProcessDelta([]byte(procesoDeltaF4), false, "", "module-design", "active"); err == nil {
		t.Error("promoción sin id del módulo debía fallar (R13)")
	}
	if _, err := projectSDDProcessDelta([]byte(procesoDeltaF4), false, "spec-graph", "", "active"); err == nil {
		t.Error("promoción sin type del módulo debía fallar (R13)")
	}
}

// TestProyeccionPromocionStatusValido — R13: promover con un status fuera del vocabulario de ciclo de
// vida es error; una canónica no nace con un status basura.
//
// Sabotaje que lo pone en ROJO: quitar la guarda que valida el status contra el vocabulario. Promover
// con un status basura NO falla (R13).
// arnes: archivo="internal/mcp/spec_archive.go"
// arnes: de="	if promoteStatus != \"\" && !slices.Contains(specgraph.SpecStatuses, promoteStatus) {\n		return nil, fmt.Errorf(\"status de promoción %q fuera del vocabulario de ciclo de vida %v (R13)\", promoteStatus, specgraph.SpecStatuses)\n	}"
// arnes: a=""
func TestProyeccionPromocionStatusValido(t *testing.T) {
	if _, err := projectSDDProcessDelta([]byte(procesoDeltaF4), false, "spec-graph", "module-design", "banana"); err == nil {
		t.Error("promoción con status fuera del vocabulario debía fallar (R13)")
	}
	// Un status válido del vocabulario sí pasa (control: el guard no rechaza de más).
	if _, err := projectSDDProcessDelta([]byte(procesoDeltaF4), false, "spec-graph", "module-design", "draft"); err != nil {
		t.Errorf("promoción con status válido no debía fallar: %v", err)
	}
}
