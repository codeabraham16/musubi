package embedding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeUnigramTokenizer escribe un tokenizer.json de juguete Unigram (SIN Precompiled, para
// no depender de un charsmap real): así se testea el Viterbi + Metaspace + parsing del vocab
// de forma determinista. El charsmap real se valida en TestUnigramRealBitExact (gated).
func writeUnigramTokenizer(t *testing.T, dir string) {
	t.Helper()
	doc := map[string]any{
		"normalizer": map[string]any{
			"type": "Sequence",
			"normalizers": []any{
				map[string]any{"type": "Strip", "strip_left": true, "strip_right": true},
				map[string]any{"type": "Replace", "pattern": map[string]any{"Regex": " {2,}"}, "content": " "},
			},
		},
		"pre_tokenizer": map[string]any{"type": "Metaspace", "replacement": "▁", "prepend_scheme": "always", "split": false},
		"model": map[string]any{
			"type":    "Unigram",
			"unk_id":  1,
			"vocab":   []any{[]any{"[PAD]", 0.0}, []any{"[UNK]", -20.0}, []any{"▁ab", -1.0}, []any{"▁a", -2.0}, []any{"b", -3.0}, []any{"▁", -5.0}},
		},
	}
	b, _ := json.Marshal(doc)
	if err := os.WriteFile(filepath.Join(dir, "tokenizer.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUnigramEncode(t *testing.T) {
	dir := t.TempDir()
	writeUnigramTokenizer(t, dir)
	tok, err := loadTokenizer(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatalf("loadTokenizer: %v", err)
	}
	if _, ok := tok.(*unigram); !ok {
		t.Fatalf("esperaba *unigram, obtuve %T", tok)
	}
	cases := map[string][]int{
		"ab":  {2},       // "▁ab" (score -1.0) gana sobre "▁a"+"b" (-5.0)
		"a":   {3},       // "▁a"
		"z":   {5, 1},    // "▁" + z(unk): z no está en el vocab
		" ab": {2},       // Strip recorta el espacio de borde ⇒ igual que "ab"
		"":    nil,       // vacío ⇒ sin tokens
	}
	for in, want := range cases {
		got := tok.EncodeIDs(in)
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

func TestLoadTokenizerRejectsUnknownModel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tokenizer.json"), []byte(`{"model":{"type":"BPE"}}`), 0o644)
	if _, err := loadTokenizer(filepath.Join(dir, "tokenizer.json")); err == nil {
		t.Error("un model.type desconocido debería fallar")
	}
}

func TestNewDartsRejectsBadCharsmap(t *testing.T) {
	if _, err := newDarts([]byte{1, 2}); err == nil {
		t.Error("charsmap < 4 bytes debería fallar")
	}
	// trie_size que excede el buffer.
	if _, err := newDarts([]byte{0xff, 0xff, 0xff, 0xff}); err == nil {
		t.Error("trie_size fuera de rango debería fallar")
	}
	// trie_size == 0: units vacío ⇒ darts.match paniqueaba en d.units[0] (auditoría #16).
	if _, err := newDarts([]byte{0, 0, 0, 0}); err == nil {
		t.Error("trie_size == 0 (trie vacío) debería fallar en vez de producir un darts que paniquea")
	}
}

// REGRESIÓN (auditoría 2026-07-26, #16): un tokenizer.json malformado (charsmap corrupto) podía
// producir un darts con units vacío u offsets fuera de rango, y match/normString paniqueaban con
// index-out-of-range en vez de degradar. Todo el resto del cargador devuelve error; estos también.
func TestDartsMatchAndNormStringDoNotPanic(t *testing.T) {
	empty := &darts{} // units vacío
	if _, _, ok := empty.match([]byte("hola")); ok {
		t.Fatal("un trie vacío no debe encontrar prefijo")
	}
	d := &darts{norm: []byte("abc\x00")}
	if s := d.normString(999); s != "" {
		t.Fatalf("un offset fuera de rango debe dar \"\", obtuve %q", s)
	}
}

func TestParseNormalizerUnsupported(t *testing.T) {
	if _, err := parseNormalizer(json.RawMessage(`{"type":"Lowercase"}`)); err == nil {
		t.Error("un normalizer no soportado debería fallar")
	}
}

// TestUnigramRealBitExact valida BIT-EXACTO contra el tokenizer real de POTION multilingüe.
// Requiere el asset local (tokenizer.json), apuntado por MUSUBI_SPM_TESTDATA (un directorio
// con el tokenizer.json de potion-multilingual-128M). Sin la env var, se saltea (CI no lo
// corre; no se commitea el asset de 18MB). La referencia text→ids está en testdata/.
func TestUnigramRealBitExact(t *testing.T) {
	dir := os.Getenv("MUSUBI_SPM_TESTDATA")
	if dir == "" {
		t.Skip("MUSUBI_SPM_TESTDATA no seteado: se saltea la validación contra el tokenizer real de POTION")
	}
	tok, err := loadTokenizer(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatalf("loadTokenizer(real): %v", err)
	}
	ref, err := os.ReadFile("testdata/spm_potion_ids.json")
	if err != nil {
		t.Fatalf("referencia: %v", err)
	}
	var cases []struct {
		Text string `json:"text"`
		IDs  []int  `json:"ids"`
	}
	if err := json.Unmarshal(ref, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		got := tok.EncodeIDs(c.Text)
		if len(got) != len(c.IDs) {
			t.Errorf("EncodeIDs(%q) = %v, quería %v", c.Text, got, c.IDs)
			continue
		}
		for i := range got {
			if got[i] != c.IDs[i] {
				t.Errorf("EncodeIDs(%q) = %v, quería %v", c.Text, got, c.IDs)
				break
			}
		}
	}
}
