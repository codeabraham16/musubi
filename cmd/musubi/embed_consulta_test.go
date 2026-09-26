package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

// escribirTablaUnigramDeJuguete deja en dir una tabla estática Unigram cargable: tokenizer.json con
// una pieza por runa imprimible ASCII (más algunas largas) y un model.safetensors con valores que
// dependen del id. Alcanza para que un texto tapado por el portero o partido en trozos dé OTRO
// vector, que es lo que esta prueba necesita ver.
func escribirTablaUnigramDeJuguete(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	vocab := []any{[]any{"[PAD]", 0.0}, []any{"[UNK]", 0.0}, []any{"▁", -2.0}}
	for r := rune(33); r < 127; r++ {
		vocab = append(vocab, []any{string(r), -4.0 - float64(r%7)/10})
	}
	for _, p := range []string{"▁el", "▁turno", "▁hook", "▁clave", "▁tabla", "▁vector", "REDACTED", "▁AKIA", "▁de", "▁la"} {
		vocab = append(vocab, []any{p, -1.5})
	}
	doc := map[string]any{
		"normalizer":    map[string]any{"type": "Sequence", "normalizers": []any{map[string]any{"type": "Strip"}}},
		"pre_tokenizer": map[string]any{"type": "Metaspace", "replacement": "▁", "prepend_scheme": "always", "split": false},
		"model":         map[string]any{"type": "Unigram", "unk_id": 1, "vocab": vocab},
	}
	b, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(dir, "tokenizer.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	const dim = 8
	filas := len(vocab)
	blob := make([]byte, filas*dim*4)
	for i := 0; i < filas*dim; i++ {
		v := float32(math.Sin(float64(i)*1.7 + 0.3))
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(v))
	}
	hdr, _ := json.Marshal(map[string]any{
		"embeddings": map[string]any{"dtype": "F32", "shape": []int{filas, dim}, "data_offsets": []int{0, len(blob)}},
	})
	out := make([]byte, 8, 8+len(hdr)+len(blob))
	binary.LittleEndian.PutUint64(out, uint64(len(hdr)))
	out = append(append(out, hdr...), blob...)
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// textosDeConsultaDelTurno son los textos en los que el embebedor de consulta y el del daemon se
// separarían si el de consulta pasara por un envoltorio que el del daemon no tiene: uno de más de
// 6.000 bytes (el troceador lo partiría y promediaría) y uno con forma de secreto (el portero lo
// taparía). El secreto se arma en ejecución para que el escáner de secretos no lo vea en el fuente.
func textosDeConsultaDelTurno() []string {
	secreto := "AKIA" + "1234567890ABCDEF"
	largo := strings.Repeat("el turno del hook trae el vector de la tabla ", 160) // ~7.000 bytes
	return []string{largo, "la clave " + secreto + " quedó en el log", "el hook del turno", ""}
}

// compararEmbebedores falla si el de consulta no da, bit a bit, el vector del daemon.
func compararEmbebedores(t *testing.T, daemon, consulta embedding.Provider) {
	t.Helper()
	if !embedding.Enabled(daemon) {
		t.Fatalf("control: el embebedor del daemon salió apagado (%T), así que no hay contra qué comparar", daemon)
	}
	if daemon.Name() != consulta.Name() {
		t.Fatalf("procedencia distinta: daemon %q, consulta %q", daemon.Name(), consulta.Name())
	}
	for _, tx := range textosDeConsultaDelTurno() {
		a, errA := daemon.Embed(context.Background(), tx)
		b, errB := consulta.Embed(context.Background(), tx)
		if errA != nil || errB != nil {
			t.Fatalf("errores: daemon=%v consulta=%v", errA, errB)
		}
		if len(a) != len(b) {
			t.Fatalf("largo %d contra %d para un texto de %d bytes", len(a), len(b), len(tx))
		}
		for j := range a {
			if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
				t.Fatalf("texto de %d bytes (%q…): el vector de consulta difiere del del daemon en la componente %d (%v contra %v). "+
					"El de consulta pasó por un envoltorio (troceo o portero) que el del daemon no tiene",
					len(tx), tx[:min(len(tx), 30)], j, b[j], a[j])
			}
		}
	}
}

// TestEmbebedorDeConsultaIgualAlDelDaemon fija que el embebedor de CONSULTA —el que va a usar el
// hook por turno— da el MISMO vector que el del daemon, que es el que llenó el índice. Los dos se
// arman como en producción: el del daemon con resolveEmbedder, y el de consulta con
// NewProviderDeConsulta sobre la misma config (configDelEmbebedor), así que pasan por el MISMO
// constructor único y por el mismo envoltorio.
//
// Comparar los dos proveedores DESNUDOS no alcanza, y ése era el defecto del plan original: hoy dan
// lo mismo, pero si el de consulta dejara de estar exento del troceo o del portero, un prompt de
// más de 6.000 bytes saldría promediado por trozos y una consulta con forma de secreto saldría
// tapada, mientras el índice tiene el vector del texto entero y sin tapar.
//
// Sabotaje: sacar la consulta liviana de las exenciones del troceo.
// arnes: archivo="internal/embedding/trozos.go"
// arnes: de="case NoopProvider, *StaticProvider, *ConsultaLiviana:"
// arnes: a="case NoopProvider, *StaticProvider:"
//
// Sabotaje: sacar la consulta liviana de las exenciones del portero.
// arnes: archivo="internal/embedding/gateway.go"
// arnes: de="case NoopProvider, *StaticProvider, *ConsultaLiviana:"
// arnes: a="case NoopProvider, *StaticProvider:"
// arnes: colision_ok="TestEmbebedorDeConsultaIgualAlDelDaemonConLaTablaReal"
func TestEmbebedorDeConsultaIgualAlDelDaemon(t *testing.T) {
	root := t.TempDir()
	escribirTablaUnigramDeJuguete(t, filepath.Join(root, ".musubi", "embeddings", defaultEmbedModel))
	cfg := config.Config{} // provider vacío: la auto-detección de esta máquina

	daemon := resolveEmbedder(cfg, root) // escribe el índice del tokenizer, como el daemon real
	ec, auto := configDelEmbebedor(cfg, root)
	if !auto {
		t.Fatal("control: la tabla de juguete no se auto-detectó")
	}
	consulta, err := embedding.NewProviderDeConsulta(ec)
	if err != nil {
		t.Fatalf("NewProviderDeConsulta después de que el daemon escribió los sidecars: %v", err)
	}
	if _, ok := consulta.(*embedding.ConsultaLiviana); !ok {
		t.Errorf("el embebedor de consulta tenía que ser la consulta liviana desnuda, y es %T", consulta)
	}
	compararEmbebedores(t, daemon, consulta)
}

// TestEmbebedorDeConsultaIgualAlDelDaemonConLaTablaReal es la misma comparación sobre POTION, en el
// job recall-gate. Escribe los sidecars al lado de la tabla, como lo haría el daemon.
//
// Sabotaje: sacar la consulta liviana de las exenciones del portero.
// arnes: archivo="internal/embedding/gateway.go"
// arnes: env="MUSUBI_POTION_DIR"
// arnes: de="case NoopProvider, *StaticProvider, *ConsultaLiviana:"
// arnes: a="case NoopProvider, *StaticProvider:"
// arnes: colision_ok="TestEmbebedorDeConsultaIgualAlDelDaemon"
func TestEmbebedorDeConsultaIgualAlDelDaemonConLaTablaReal(t *testing.T) {
	dir := os.Getenv("MUSUBI_POTION_DIR")
	if dir == "" {
		t.Skip("sin MUSUBI_POTION_DIR: esta comparación necesita la tabla POTION real")
	}
	cfg := config.Config{Embedding: config.EmbeddingConfig{Provider: "static", StaticPath: dir}}
	daemon := resolveEmbedder(cfg, t.TempDir())
	ec, _ := configDelEmbebedor(cfg, t.TempDir())
	consulta, err := embedding.NewProviderDeConsulta(ec)
	if err != nil {
		t.Fatalf("NewProviderDeConsulta(%s): %v", dir, err)
	}
	compararEmbebedores(t, daemon, consulta)
}
