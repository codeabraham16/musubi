package memory

import "testing"

// El guard de cambio de modelo de embedding: registra el modelo activo, es idempotente
// con el mismo modelo, actualiza el registro ante un switch, y es no-op con modelID vacío.
func TestWarnOnEmbedModelSwitch(t *testing.T) {
	e := newTestEngine(t)

	// Primer arranque: sin previo, registra el modelo sin romper.
	e.WarnOnEmbedModelSwitch("static:a")
	if v, ok, _ := e.GetMeta(MetaEmbedModel); !ok || v != "static:a" {
		t.Fatalf("esperaba registrar static:a, obtuve %q ok=%v", v, ok)
	}

	// Mismo modelo: sin cambios.
	e.WarnOnEmbedModelSwitch("static:a")
	if v, _, _ := e.GetMeta(MetaEmbedModel); v != "static:a" {
		t.Fatalf("no debía cambiar, obtuve %q", v)
	}

	// Switch a otro modelo: actualiza el registro (el warning es side-effect de log).
	e.WarnOnEmbedModelSwitch("ollama")
	if v, _, _ := e.GetMeta(MetaEmbedModel); v != "ollama" {
		t.Fatalf("esperaba actualizar a ollama, obtuve %q", v)
	}

	// modelID vacío (sin embedder): no-op, no toca el registro.
	e.WarnOnEmbedModelSwitch("")
	if v, _, _ := e.GetMeta(MetaEmbedModel); v != "ollama" {
		t.Fatalf("vacío debía ser no-op, obtuve %q", v)
	}
}

// Con vectores ya almacenados, un switch atraviesa el branch de conteo sin romper.
func TestWarnOnEmbedModelSwitchWithVectors(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("o1", "topic", "hola mundo", []float32{1, 2, 3}); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}
	e.WarnOnEmbedModelSwitch("m1")
	e.WarnOnEmbedModelSwitch("m2") // prev=m1 y hay vectores → branch de warning
	if v, _, _ := e.GetMeta(MetaEmbedModel); v != "m2" {
		t.Fatalf("esperaba m2 tras el switch con vectores, obtuve %q", v)
	}
}
