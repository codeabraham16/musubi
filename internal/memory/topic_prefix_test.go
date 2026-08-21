package memory

import "testing"

// TestObservationsByTopicPrefixInProject valida la query del MÉTODO VIVO (Musubi Renaissance · CAPA 2):
// trae las observaciones de un tenant por prefijo de topic, ordenadas por importancia, aislando el prefijo
// y el proyecto.
func TestObservationsByTopicPrefixInProject(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	// Dos tarjetas de método (distinta importancia) + una de otro prefijo + una de otro tenant.
	if err := e.SaveObservationTypedFrom(proj, "seed", "m1", "design-method/jerarquia", "una cosa manda", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTypedFrom(proj, "seed", "m2", "design-method/un-cta", "un solo CTA por pantalla", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTypedFrom(proj, "seed", "c1", "design-corpus/tabla", "patrón de tabla", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTypedFrom("otro-tenant", "seed", "x1", "design-method/ajeno", "método de otro proyecto", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	got, err := e.ObservationsByTopicPrefixInProject(proj, "design-method/", 10)
	if err != nil {
		t.Fatalf("ObservationsByTopicPrefixInProject: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("esperaba 2 tarjetas de método (no la de otro prefijo ni la de otro tenant); obtuve %d: %+v", len(got), got)
	}
	// Orden por importancia DESC: m2 (1.0) antes que m1 (0.5).
	if got[0].ID != "m2" || got[1].ID != "m1" {
		t.Errorf("esperaba orden por importancia [m2, m1]; obtuve [%s, %s]", got[0].ID, got[1].ID)
	}
	// El límite acota.
	if lim, _ := e.ObservationsByTopicPrefixInProject(proj, "design-method/", 1); len(lim) != 1 || lim[0].ID != "m2" {
		t.Errorf("con limit 1 esperaba solo la más importante (m2); obtuve %+v", lim)
	}
	// Prefijo sin match ⇒ vacío (no error).
	if none, err := e.ObservationsByTopicPrefixInProject(proj, "design-nada/", 10); err != nil || len(none) != 0 {
		t.Errorf("prefijo sin match debe dar vacío sin error; got=%d err=%v", len(none), err)
	}
}
