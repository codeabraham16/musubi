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

// TestEmbedderCaroDeConstruirProtegeElHookPorTurno fija la guarda que impide que el hook por turno
// vuelva a cargar la tabla estática en cada prompt.
//
// LA REGRESIÓN QUE FIJA, con sus números: al cablear el embebedor en el hook, `musubi turn
// --hook-mode` pasó de 0,29 s a 11-23 s con picos de 1,0-1,3 GB de RSS, porque NewStaticProvider
// hace os.ReadFile de 512 MB en CADA construcción y el hook es un proceso efímero. Con el
// `"timeout": 10` de .claude/settings.json eso no es lentitud: es el hook MATADO y el turno sin
// memoria — peor que el léxico que tenía antes.
//
// La guarda de 2 s que vive adentro de buildTurnRecall NO cubría esto: el costo está en la
// CONSTRUCCIÓN, antes de que exista un ctx al que ponerle timeout.
func TestEmbedderCaroDeConstruirProtegeElHookPorTurno(t *testing.T) {
	root := t.TempDir()

	t.Run("static declarado es caro", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Embedding.Provider = "static"
		if !embedderCaroDeConstruir(cfg, root) {
			t.Error("el provider static carga la tabla entera: tiene que declararse caro")
		}
	})

	t.Run("sin tabla bajada NO es caro", func(t *testing.T) {
		cfg := config.Config{} // provider vacío, sin tabla en disco
		if embedderCaroDeConstruir(cfg, root) {
			t.Error("sin tabla no hay nada que cargar: no puede declararse caro")
		}
	})

	t.Run("auto-deteccion de la tabla es cara", func(t *testing.T) {
		// El caso REAL de esta máquina: provider vacío pero la tabla bajada, así que
		// resolveEmbedder la auto-detecta y construye el static.
		dir := filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range []string{"model.safetensors", "tokenizer.json"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if !hasStaticTable(dir) {
			t.Skip("hasStaticTable no reconoce esta tabla de juguete; el caso se cubre en el de arriba")
		}
		cfg := config.Config{} // vacío: dispara la auto-detección
		if !embedderCaroDeConstruir(cfg, root) {
			t.Error("con la tabla bajada la auto-detección construye el static: tiene que declararse caro")
		}
	})

	t.Run("un provider por red NO es caro de construir", func(t *testing.T) {
		cfg := config.Config{}
		cfg.Embedding.Provider = "ollama"
		if embedderCaroDeConstruir(cfg, root) {
			t.Error("construir un cliente HTTP no lee 512 MB: no puede declararse caro")
		}
	})
}
