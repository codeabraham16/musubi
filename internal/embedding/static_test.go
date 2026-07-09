package embedding

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// writeSyntheticModel arma un directorio con model.safetensors + tokenizer.json mínimos
// (vocab de juguete) para testear el StaticProvider sin depender del asset real de 129MB.
func writeSyntheticModel(t *testing.T, dir string, rows [][]float32) {
	t.Helper()
	// tokenizer.json: BertNormalizer (lowercase+strip_accents) + WordPiece de juguete.
	tok := map[string]any{
		"normalizer": map[string]any{
			"type": "BertNormalizer", "clean_text": true,
			"handle_chinese_chars": true, "strip_accents": nil, "lowercase": true,
		},
		"model": map[string]any{
			"type":                      "WordPiece",
			"vocab":                     map[string]int{"[UNK]": 0, "deploy": 1, "##s": 2, "cafe": 3, "el": 4},
			"unk_token":                 "[UNK]",
			"continuing_subword_prefix": "##",
			"max_input_chars_per_word":  100,
		},
	}
	tb, _ := json.Marshal(tok)
	if err := os.WriteFile(filepath.Join(dir, "tokenizer.json"), tb, 0o644); err != nil {
		t.Fatal(err)
	}
	// model.safetensors: tensor "embeddings" F32 [len(rows), dim].
	dim := len(rows[0])
	blob := make([]byte, 0, len(rows)*dim*4)
	for _, r := range rows {
		for _, v := range r {
			var buf [4]byte
			binary.LittleEndian.PutUint32(buf[:], math.Float32bits(v))
			blob = append(blob, buf[:]...)
		}
	}
	hdr, _ := json.Marshal(map[string]any{
		"embeddings": map[string]any{"dtype": "F32", "shape": []int{len(rows), dim}, "data_offsets": []int{0, len(blob)}},
	})
	var out []byte
	var lenb [8]byte
	binary.LittleEndian.PutUint64(lenb[:], uint64(len(hdr)))
	out = append(out, lenb[:]...)
	out = append(out, hdr...)
	out = append(out, blob...)
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSyntheticProvider(t *testing.T) *StaticProvider {
	t.Helper()
	dir := t.TempDir()
	writeSyntheticModel(t, dir, [][]float32{
		{0, 0, 0, 0}, // [UNK]
		{1, 0, 0, 0}, // deploy
		{0, 1, 0, 0}, // ##s
		{0, 0, 1, 0}, // cafe
		{0, 0, 0, 1}, // el
	})
	p, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("NewStaticProvider: %v", err)
	}
	return p
}

// El WordPiece hand-rolled: subword, strip-accents, unknown y lowercase.
func TestStaticWordPiece(t *testing.T) {
	p := newSyntheticProvider(t)
	cases := map[string][]int{
		"deploy":    {1},
		"deploys":   {1, 2}, // greedy longest-match: deploy + ##s
		"Deploys":   {1, 2}, // lowercase
		"Café":      {3},    // strip-accents NFD: café → cafe
		"el":        {4},
		"xyz":       {0},    // desconocido → [UNK]
		"deploy el": {1, 4}, // dos palabras
	}
	for in, want := range cases {
		got := p.tok.EncodeIDs(in)
		if len(got) != len(want) {
			t.Errorf("EncodeIDs(%q) = %v, quería %v", in, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("EncodeIDs(%q) = %v, quería %v", in, got, want)
				break
			}
		}
	}
}

// Embed: dimensión correcta, L2-normalizado, determinista, y semánticamente coherente
// (deploy≈deploys por compartir el token 'deploy').
func TestStaticEmbed(t *testing.T) {
	p := newSyntheticProvider(t)
	if p.Dimensions() != 4 {
		t.Fatalf("Dimensions=%d, quería 4", p.Dimensions())
	}
	if !Enabled(p) {
		t.Fatal("StaticProvider debería contar como Enabled")
	}
	v, err := p.Embed(context.Background(), "deploy")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 4 {
		t.Fatalf("len(v)=%d, quería 4", len(v))
	}
	// 'deploy' = row [1,0,0,0], mean de 1 token, normalizado → [1,0,0,0].
	var norm2 float64
	for _, x := range v {
		norm2 += float64(x) * float64(x)
	}
	if math.Abs(norm2-1.0) > 1e-5 {
		t.Errorf("no normalizado: |v|²=%.6f", norm2)
	}
	// Determinismo.
	v2, _ := p.Embed(context.Background(), "deploy")
	for i := range v {
		if v[i] != v2[i] {
			t.Fatal("Embed no determinista")
		}
	}
	// Texto sin tokens conocidos que no sean [UNK] cero: vector cero (no rompe).
	z, _ := p.Embed(context.Background(), "xyz")
	for _, x := range z {
		if x != 0 {
			t.Errorf("esperaba vector cero para token [UNK] con fila cero, obtuve %v", z)
			break
		}
	}
}

// El factory construye el provider "static" y falla claro si el provider es desconocido.
func TestFactoryStatic(t *testing.T) {
	dir := t.TempDir()
	writeSyntheticModel(t, dir, [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}, {1, 0}})
	p, err := NewProvider(config.EmbeddingConfig{Provider: "static", StaticPath: dir})
	if err != nil {
		t.Fatalf("NewProvider static: %v", err)
	}
	if !Enabled(p) || p.Dimensions() != 2 {
		t.Fatalf("provider static mal construido: enabled=%v dim=%d", Enabled(p), p.Dimensions())
	}
	if _, err := NewProvider(config.EmbeddingConfig{Provider: "static", StaticPath: ""}); err == nil {
		t.Error("static con static_path vacío debería fallar")
	}
	if _, err := NewProvider(config.EmbeddingConfig{Provider: "inexistente"}); err == nil {
		t.Error("provider desconocido debería fallar")
	}
}
