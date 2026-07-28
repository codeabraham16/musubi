package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"musubi/internal/embedding"
)

// failingEmbedder cuenta como Enabled (no es NoopProvider) pero su Embed siempre falla:
// fuerza el fallo de persistSDDArtifact para probar el ordenamiento crash-safe del complete.
type failingEmbedder struct{}

func (failingEmbedder) Embed(context.Context, string) ([]float32, error) {
	return nil, errors.New("embed caído (test)")
}
func (failingEmbedder) Dimensions() int { return 8 }
func (failingEmbedder) Name() string    { return "failing-test" }

// sddOut es la forma de la respuesta de musubi_sdd que asertan los tests.
type sddOut struct {
	Change    string   `json:"change"`
	RunID     string   `json:"run_id"`
	Status    string   `json:"status"`
	Active    string   `json:"active"`
	Ready     []string `json:"ready"`
	Directive string   `json:"directive"`
	Role      string   `json:"role"`
	Template  string   `json:"template"`
	Note      string   `json:"note"`
	Done      bool     `json:"done"`
	Phases    []struct {
		Phase  string `json:"phase"`
		Status string `json:"status"`
		Result string `json:"result"`
	} `json:"phases"`
}

func parseSDD(t *testing.T, res interface{}) sddOut {
	t.Helper()
	var out sddOut
	if err := json.Unmarshal([]byte(textOf(t, res)), &out); err != nil {
		t.Fatalf("no se pudo parsear la respuesta SDD: %v", err)
	}
	return out
}

func TestSDDFullFlowAndHandoff(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	const change = "Add Auth"

	// start → la primera fase activa es proposal, con su plantilla.
	res, e := call(t, s, "musubi_sdd", map[string]interface{}{"action": "start", "change": change})
	if e != nil {
		t.Fatalf("start: %+v", e)
	}
	out := parseSDD(t, res)
	if out.RunID != "sdd-add-auth" || out.Status != "running" {
		t.Fatalf("start inesperado: run_id=%q status=%q", out.RunID, out.Status)
	}
	if out.Active != "proposal" || out.Template != ".musubi/templates/sdd/proposal.md" {
		t.Fatalf("fase activa/plantilla inesperada: active=%q template=%q", out.Active, out.Template)
	}
	if out.Role == "" {
		t.Fatalf("la fase activa debería traer su rol especializado")
	}

	// Cerrar proposal con un contrato → el handoff persiste sdd/add-auth/proposal.
	res, e = call(t, s, "musubi_sdd", map[string]interface{}{
		"action": "complete", "change": change, "phase": "proposal",
		"summary":   "propuesta lista: agregar auth con JWT",
		"artifacts": []string{"specs/auth-proposal.md"},
		"risks":     []string{"rotación de claves"},
	})
	if e != nil {
		t.Fatalf("complete proposal: %+v", e)
	}
	out = parseSDD(t, res)
	if out.Active != "spec" {
		t.Fatalf("tras proposal la fase activa debería ser spec, es %q", out.Active)
	}
	if out.Note == "" {
		t.Fatalf("complete debería avisar que guardó el artefacto en memoria")
	}

	// El artefacto quedó en memoria bajo el id determinista y se puede hidratar.
	res, e = call(t, s, "musubi_memory_expand", map[string]interface{}{
		"ids": []string{"sdd/add-auth/proposal"},
	})
	if e != nil {
		t.Fatalf("memory_expand: %+v", e)
	}
	if body := textOf(t, res); !strings.Contains(body, "propuesta lista") || !strings.Contains(body, "rotación de claves") {
		t.Fatalf("el artefacto SDD no se persistió con su contenido; obtuve:\n%s", body)
	}

	// Recorrer el resto de las fases hasta cerrar el flujo.
	for _, phase := range []string{"spec", "design", "tasks", "implement", "verify", "archive"} {
		res, e = call(t, s, "musubi_sdd", map[string]interface{}{
			"action": "complete", "change": change, "phase": phase,
			"summary": "fase " + phase + " hecha",
		})
		if e != nil {
			t.Fatalf("complete %s: %+v", phase, e)
		}
	}
	out = parseSDD(t, res)
	if !out.Done || out.Status != "done" {
		t.Fatalf("tras archive el flujo debería estar done; status=%q done=%v", out.Status, out.Done)
	}

	// status reconstruye el estado completo: todas las fases done.
	res, _ = call(t, s, "musubi_sdd", map[string]interface{}{"action": "status", "change": change})
	out = parseSDD(t, res)
	for _, p := range out.Phases {
		if p.Status != "done" {
			t.Errorf("fase %q debería estar done, está %q", p.Phase, p.Status)
		}
	}
}

func TestSDDImplementDirectiveReferencesHandoff(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	const change = "Add Auth"
	call(t, s, "musubi_sdd", map[string]interface{}{"action": "start", "change": change})
	for _, phase := range []string{"proposal", "spec", "design", "tasks"} {
		call(t, s, "musubi_sdd", map[string]interface{}{
			"action": "complete", "change": change, "phase": phase, "summary": phase + " ok",
		})
	}
	// Con tasks cerrada, implement queda activa y su directiva referencia el recall.
	res, _ := call(t, s, "musubi_sdd", map[string]interface{}{"action": "next", "change": change})
	out := parseSDD(t, res)
	if out.Active != "implement" {
		t.Fatalf("fase activa debería ser implement, es %q", out.Active)
	}
	if !strings.Contains(out.Directive, "sdd/add-auth") {
		t.Fatalf("la directiva de implement debería referenciar sdd/add-auth: %q", out.Directive)
	}
}

// TestSDDCompleteAtomicityArtifactFirst prueba el ordenamiento crash-safe (auditoría #23): si el
// guardado del artefacto falla, la fase NO queda 'done' — nunca hay una fase cerrada sin su artefacto
// (el contrato "fase done ⟹ artefacto existe" del que dependen las fases siguientes). El complete
// persiste el artefacto ANTES de marcar el step, así que un fallo del artefacto deja la fase reintenta-
// ble en vez de romper el contrato en silencio.
func TestSDDCompleteAtomicityArtifactFirst(t *testing.T) {
	s := newTestServer(t, failingEmbedder{})
	const change = "Add Auth"
	call(t, s, "musubi_sdd", map[string]interface{}{"action": "start", "change": change})

	// complete proposal: persistSDDArtifact falla (embed caído) ANTES de cerrar el step.
	if _, e := call(t, s, "musubi_sdd", map[string]interface{}{
		"action": "complete", "change": change, "phase": "proposal", "summary": "propuesta lista",
	}); e == nil {
		t.Fatal("esperaba error: el guardado del artefacto debía fallar")
	}

	// INVARIANTE: proposal NO quedó 'done' (habría sido una fase cerrada sin artefacto).
	res, _ := call(t, s, "musubi_sdd", map[string]interface{}{"action": "status", "change": change})
	out := parseSDD(t, res)
	for _, p := range out.Phases {
		if p.Phase == "proposal" && p.Status == "done" {
			t.Fatalf("proposal quedó 'done' pese a que el artefacto falló: contrato step↔artefacto roto")
		}
	}
	if out.Active != "proposal" {
		t.Errorf("proposal debería seguir activa (reintentable) tras el fallo, active=%q", out.Active)
	}
}

func TestSDDErrorPaths(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	cases := []map[string]interface{}{
		{"action": "boom", "change": "c"},                        // action inválida
		{"action": "start"},                                      // falta change
		{"action": "next", "change": "noexiste"},                 // flujo inexistente
		{"action": "complete", "change": "c", "phase": "spec"},   // falta summary (y no arrancó)
		{"action": "complete", "change": "c", "summary": "x"},    // falta phase
	}
	for i, args := range cases {
		if _, e := call(t, s, "musubi_sdd", args); e == nil {
			t.Errorf("caso %d (%v): esperaba error", i, args["action"])
		}
	}
}
