package mcp

import (
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// TestAnchorCoverageNoFiltraCandidatasEntreTenants sella B1 de review/doctor-rancidez-silenciosa.
//
// musubi_doctor SIN repair está abierto a cualquier LECTOR (toolDoctor) y los checks del registry son
// GLOBALES: func(*DbEngine) CheckResult, sin ctx ni scope de tenant. anchor_coverage es el PRIMER check
// que nombra FILAS concretas (topic_key + token de notas) y no sólo agregados —conteos, fechas, rutas de
// sistema—, así que su detalle cruzaría la valla de tenant en el central. El borde MCP borra el Detail
// para principals NO-admin; el agregado (Message, el ratio) sí viaja. El CLI y el stdio local (principal
// nil → isAdmin) lo muestran entero.
func TestAnchorCoverageNoFiltraCandidatasEntreTenants(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	bob := &Principal{Name: "bob", Role: RoleWriter, ProjectID: "tenantB"}
	alice := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "tenantA"}
	admin := &Principal{Name: "root", Role: RoleAdmin, ProjectID: "tenantA"}

	// bob (tenantB) guarda una nota que cita código y queda SIN ancla → candidata de anchor_coverage.
	if _, e := callAsPrincipal(t, s, bob, "musubi_save_observation", map[string]any{
		"topic_key": "secreto-b/roadmap-privado",
		"content":   "el plan privado de B vive en cuenta_privada_b.go",
	}); e != nil {
		t.Fatalf("bob no pudo guardar: %+v", e)
	}

	const topic, token = "roadmap-privado", "cuenta_privada_b.go"

	// CONTROL POSITIVO: un admin (como el stdio local) SÍ ve la candidata en el Detail. Prueba que el
	// check la PRODUCE —sin la redacción, fugaría— y que lo que la oculta a alice es la redacción, no la
	// ausencia de dato.
	resAdmin, e := callAsPrincipal(t, s, admin, "musubi_doctor", map[string]any{"check": "anchor_coverage"})
	if e != nil {
		t.Fatalf("admin doctor: %+v", e)
	}
	if out := textOf(t, resAdmin); !strings.Contains(out, token) {
		t.Fatalf("CONTROL+: el admin debe ver la candidata (%s) en el Detail, no la vio:\n%s", token, out)
	}

	// alice (writer de OTRO tenant) no debe ver ni el topic_key ni el token de la nota de bob, ni por un
	// check puntual (RunCheck) ni por el diagnóstico completo (Diagnose).
	for _, args := range []map[string]any{{"check": "anchor_coverage"}, {}} {
		res, e := callAsPrincipal(t, s, alice, "musubi_doctor", args)
		if e != nil {
			t.Fatalf("alice doctor %v: %+v", args, e)
		}
		out := textOf(t, res)
		if strings.Contains(out, topic) || strings.Contains(out, token) {
			t.Errorf("FUGA: alice (tenantA) ve datos de una nota de tenantB vía musubi_doctor %v:\n%s", args, out)
		}
		// Pero el check NO desaparece para alice: sigue viendo el AGREGADO, sólo sin los nombres de filas.
		if !strings.Contains(out, "anchor_coverage") {
			t.Errorf("alice debe seguir viendo el check anchor_coverage (el agregado) con args=%v:\n%s", args, out)
		}
	}
}
