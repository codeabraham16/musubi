package main

import (
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

func TestHasStaticTable(t *testing.T) {
	dir := t.TempDir()
	if hasStaticTable(dir) {
		t.Error("un dir vacío no tiene tabla")
	}
	os.WriteFile(filepath.Join(dir, "model.safetensors"), []byte("x"), 0o644)
	if hasStaticTable(dir) {
		t.Error("falta tokenizer.json: no debería contar como tabla")
	}
	os.WriteFile(filepath.Join(dir, "tokenizer.json"), []byte("y"), 0o644)
	if !hasStaticTable(dir) {
		t.Error("con ambos archivos debería contar como tabla")
	}
}

// Sin tabla y sin provider explícito ⇒ recall léxico (NoopProvider), no falla.
func TestResolveEmbedderNoTable(t *testing.T) {
	cfg := config.Config{Embedding: config.EmbeddingConfig{Provider: "none"}}
	p := resolveEmbedder(cfg, t.TempDir())
	if embedding.Enabled(p) {
		t.Error("sin tabla debería quedar en léxico (NoopProvider)")
	}
}

// Auto-detección + DEGRADACIÓN ELEGANTE: hay archivos de tabla en la ubicación estándar
// pero son corruptos ⇒ resolveEmbedder NO aborta, cae a léxico.
func TestResolveEmbedderGracefulDegradation(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Archivos presentes (dispara la auto-detección) pero basura (falla la carga).
	os.WriteFile(filepath.Join(dir, "model.safetensors"), []byte("basura"), 0o644)
	os.WriteFile(filepath.Join(dir, "tokenizer.json"), []byte("basura"), 0o644)

	cfg := config.Config{Embedding: config.EmbeddingConfig{Provider: "none"}}
	p := resolveEmbedder(cfg, root) // no debe entrar en pánico ni abortar
	if embedding.Enabled(p) {
		t.Error("una tabla corrupta debería degradar a léxico, no quedar 'enabled'")
	}
}

// Un provider explícito inválido también degrada a léxico en vez de abortar.
func TestResolveEmbedderExplicitBadDegrades(t *testing.T) {
	cfg := config.Config{Embedding: config.EmbeddingConfig{Provider: "static", StaticPath: ""}}
	p := resolveEmbedder(cfg, t.TempDir())
	if embedding.Enabled(p) {
		t.Error("static con path vacío debería degradar a léxico")
	}
}
