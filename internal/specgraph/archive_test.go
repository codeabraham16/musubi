package specgraph

// Destino del `archive` del SDD (spec-graph F4): la función pura MergeSpecInto. Fija R13/R14/R15/E8/E9:
// crea la canónica desde el delta si no existe (promoción), fusiona secciones aditivas, y SURFACEA sin
// pisar la prosa cuando dos secciones con el mismo heading divergen. Es model-free y sin I/O.

import (
	"bytes"
	"strings"
	"testing"
)

// TestMergeSpecIntoCreaFusionaYSurfacea cubre las tres bocas de MergeSpecInto en un solo test (es una
// función pura, así que los casos no comparten estado): CREAR (E8, delta verbatim), FUSIÓN ADITIVA
// (una sección nueva entra sin conflicto) y CONFLICTO (E9/R15: mismo heading con cuerpo divergente ⇒
// `conflicts` no vacío y la prosa de la canónica NO se pisa).
//
// Sabotaje que lo pone en ROJO: invertir la comparación de cuerpos (`==` → `!=`) en mergeBodySections.
// Con `!=`, una sección divergente se trata como «idéntica» (continue) y NO se surfacea: `conflicts`
// queda vacío y el caso de conflicto se pone rojo. Ataca exactamente el invariante surface-not-auto.
// arnes: archivo="internal/specgraph/archive.go"
// arnes: de="if strings.TrimSpace(c.content) == strings.TrimSpace(d.content) {"
// arnes: a="if strings.TrimSpace(c.content) != strings.TrimSpace(d.content) {"
func TestMergeSpecIntoCreaFusionaYSurfacea(t *testing.T) {
	// CREAR (E8): sin canónica, el delta ES la canónica y se devuelve VERBATIM (lossless por identidad).
	delta := []byte("---\nid: m1\ntype: module-design\ntitle: M1\n---\n# M1\n\ncuerpo base\n")
	merged, conflicts, err := MergeSpecInto(nil, delta)
	if err != nil {
		t.Fatalf("crear: %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("crear: no debía haber conflictos, hay %d", len(conflicts))
	}
	if !bytes.Equal(merged, delta) {
		t.Errorf("crear: la canónica debía ser el delta verbatim; obtuve:\n%s", merged)
	}

	// Un delta sin id/type no se puede promover (R13).
	if _, _, err := MergeSpecInto(nil, []byte("# sin frontmatter\n")); err == nil {
		t.Error("crear: un delta sin id/type no debía poder promoverse a canónica (R13)")
	}

	canon := []byte("---\nid: m1\ntype: module-design\ntitle: M1\n---\n# M1\n\n## A\n\ncuerpo A\n")

	// FUSIÓN ADITIVA: una sección cuyo heading no existe en la canónica entra, sin conflicto.
	deltaAdd := []byte("---\nid: m1\ntype: module-design\ntitle: M1\n---\n# M1\n\n## B\n\ncuerpo B\n")
	mergedAdd, conflictsAdd, err := MergeSpecInto(canon, deltaAdd)
	if err != nil {
		t.Fatalf("fusión aditiva: %v", err)
	}
	if len(conflictsAdd) != 0 {
		t.Errorf("aditiva: no debía haber conflicto, hay %d", len(conflictsAdd))
	}
	if !strings.Contains(string(mergedAdd), "cuerpo A") || !strings.Contains(string(mergedAdd), "cuerpo B") {
		t.Errorf("aditiva: debía conservar A y agregar B; salida:\n%s", mergedAdd)
	}

	// CONFLICTO (E9/R15): mismo heading, cuerpo divergente ⇒ se surfacea en `conflicts` y la prosa de
	// la canónica queda intacta; la del delta NO se escribe (la reconcilia una persona).
	deltaConf := []byte("---\nid: m1\ntype: module-design\ntitle: M1\n---\n# M1\n\n## A\n\ncuerpo A DISTINTO del delta\n")
	mergedConf, conflictsConf, err := MergeSpecInto(canon, deltaConf)
	if err != nil {
		t.Fatalf("conflicto: %v", err)
	}
	if len(conflictsConf) == 0 {
		t.Error("conflicto: un heading con cuerpo divergente debía surfacearse, no fusionarse solo (R15)")
	}
	if !strings.Contains(string(mergedConf), "cuerpo A") {
		t.Error("conflicto: la prosa de la canónica NO debía pisarse")
	}
	if strings.Contains(string(mergedConf), "cuerpo A DISTINTO del delta") {
		t.Error("conflicto: la prosa del delta se escribió en la canónica; debía sólo surfacearse")
	}
}
