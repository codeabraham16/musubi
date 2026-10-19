package main

import (
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
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

// TestElHookPorTurnoUsaLaConsultaLiviana fija el ENCENDIDO del embebedor en el hook por turno
// (decisión #2): con la tabla estática presente y per_turn_recall en true, embebedorDelHook arma el
// embebedor por el camino LIVIANO (ConsultaLiviana) y NO por NewStaticProvider.
//
// Antes acá había TestEmbedderCaroDeConstruirProtegeElHookPorTurno, que fijaba la guarda inversa: el
// hook devolvía nil con la tabla presente, para no pagar os.ReadFile de 512 MB por prompt (0,29 s →
// 11-23 s / 1,3 GB, matado por el timeout de 10 s). Esa guarda se retiró junto con
// embedderCaroDeConstruir: el camino liviano da el MISMO vector sin cargar la tabla (~17 ms / ~24 MB
// medidos), así que la premisa «static es caro» dejó de ser cierta para el hook.
//
// La equivalencia liviano==daemon la fijan TestEmbebedorDeConsultaIgualAlDelDaemon[ConLaTablaReal].
// Esto fija lo otro: que el HOOK efectivamente rutea por ahí.
func TestElHookPorTurnoUsaLaConsultaLiviana(t *testing.T) {
	t.Run("recall por turno apagado ⇒ nil aunque esté la tabla", func(t *testing.T) {
		root := t.TempDir()
		escribirTablaUnigramDeJuguete(t, filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel))
		_ = resolveEmbedder(config.Config{}, root) // escribe los sidecars, como el daemon
		engine, err := memory.NewDbEngine(root)
		if err != nil {
			t.Fatal(err)
		}
		defer engine.Close()
		cfg := config.Config{} // Loop.PerTurnRecall queda en false
		if emb := embebedorDelHook(cfg, root, engine); emb != nil {
			t.Errorf("con per_turn_recall apagado el hook no construye nada, y devolvió %T", emb)
		}
	})

	t.Run("tabla presente + encendido ⇒ consulta liviana, no la tabla entera", func(t *testing.T) {
		root := t.TempDir()
		escribirTablaUnigramDeJuguete(t, filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel))
		// El daemon corre una vez y deja los sidecars (tokenizer.idx + identidad.json) que el camino
		// liviano necesita; sin ellos NewProviderDeConsulta falla y el hook cae a léxico.
		_ = resolveEmbedder(config.Config{}, root)
		engine, err := memory.NewDbEngine(root)
		if err != nil {
			t.Fatal(err)
		}
		defer engine.Close()
		cfg := config.Config{}
		cfg.Loop.PerTurnRecall = true
		emb := embebedorDelHook(cfg, root, engine)
		if !embedding.Enabled(emb) {
			t.Fatal("con la tabla y el recall por turno encendido el hook tiene que traer un embebedor vivo (antes devolvía nil acá)")
		}
		if _, ok := emb.(*embedding.ConsultaLiviana); !ok {
			t.Errorf("el hook tiene que armar la consulta liviana; es %T (¿volvió a resolveEmbedder / NewStaticProvider?)", emb)
		}
	})

	t.Run("tabla sin sidecars ⇒ léxico, no pánico", func(t *testing.T) {
		root := t.TempDir()
		// Tabla presente (dispara la auto-detección) pero SIN que el daemon haya escrito los
		// sidecars: el camino liviano no tiene el índice del tokenizer, así que degrada a léxico.
		escribirTablaUnigramDeJuguete(t, filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel))
		engine, err := memory.NewDbEngine(root)
		if err != nil {
			t.Fatal(err)
		}
		defer engine.Close()
		cfg := config.Config{}
		cfg.Loop.PerTurnRecall = true
		if emb := embebedorDelHook(cfg, root, engine); embedding.Enabled(emb) {
			t.Errorf("sin los sidecars del tokenizer el hook tiene que caer a léxico, y trajo %T vivo", emb)
		}
	})
}
