package embedding

// static.go implementa un Provider MODEL-FREE AT INFERENCE: genera embeddings con
// una tabla estática token→vector (formato model2vec/POTION) + mean-pooling, SIN
// correr ninguna red neuronal en runtime y SIN cgo. La tabla se destiló offline de
// un sentence-transformer (por eso NO es "model-free absoluto", sí "model-free at
// inference": no hay forward pass ni runtime de modelo dentro del server, misma
// categoría que servir vectores GloVe). El tokenizer es un WordPiece BERT propio
// (bit-exacto vs HuggingFace), cuya única dep es golang.org/x/text para el NFD del
// strip-accents. La tabla NO se versiona en el repo: el usuario apunta static_path a
// un directorio con model.safetensors + tokenizer.json (bring-your-own-table).

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// tokenizer convierte texto en ids de token (índices en la tabla). Lo implementan tanto
// el WordPiece BERT (tablas inglesas) como el Unigram/SentencePiece (multilingües); el
// StaticProvider despacha al que corresponda según el tokenizer.json.
type tokenizer interface {
	EncodeIDs(text string) []int
}

// StaticProvider genera embeddings por lookup + mean-pool sobre una tabla estática.
type StaticProvider struct {
	table   [][]float32
	dim     int
	tok     tokenizer
	modelID string // identidad de la tabla, para la provenance del vector (S1)
}

// NewStaticProvider carga la tabla (dir/model.safetensors) y el tokenizer
// (dir/tokenizer.json). modelID identifica la tabla para la regla de homogeneidad.
func NewStaticProvider(dir string) (*StaticProvider, error) {
	if dir == "" {
		return nil, fmt.Errorf("static_path vacío: apuntá embedding.static_path a un directorio con model.safetensors + tokenizer.json")
	}
	table, dim, err := loadStaticTable(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		return nil, fmt.Errorf("tabla estática: %w", err)
	}
	tok, err := loadTokenizer(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		return nil, fmt.Errorf("tokenizer: %w", err)
	}
	return &StaticProvider{
		table:   table,
		dim:     dim,
		tok:     tok,
		modelID: "static:" + filepath.Base(filepath.Clean(dir)),
	}, nil
}

func (p *StaticProvider) Name() string    { return p.modelID }
func (p *StaticProvider) Dimensions() int { return p.dim }

// Embed = tokenizar (WordPiece, sin special tokens) → lookup por token → mean →
// L2-normalize. Reproduce bit-exacto model2vec/POTION (config normalize=true; pca/zipf
// vienen horneados en la tabla al destilar, así que en inferencia son no-op).
func (p *StaticProvider) Embed(_ context.Context, text string) ([]float32, error) {
	ids := p.tok.EncodeIDs(text)
	acc := make([]float64, p.dim)
	n := 0
	for _, id := range ids {
		if id < 0 || id >= len(p.table) {
			continue
		}
		row := p.table[id]
		for j := 0; j < p.dim; j++ {
			acc[j] += float64(row[j])
		}
		n++
	}
	out := make([]float32, p.dim)
	if n == 0 {
		return out, nil // texto sin tokens conocidos: vector cero (coseno 0, no rompe)
	}
	inv := 1.0 / float64(n)
	var norm2 float64
	for j := 0; j < p.dim; j++ {
		acc[j] *= inv
		norm2 += acc[j] * acc[j]
	}
	l2 := math.Sqrt(norm2)
	if l2 > 0 {
		for j := 0; j < p.dim; j++ {
			out[j] = float32(acc[j] / l2)
		}
	}
	return out, nil
}

// --- safetensors loader (tensor "embeddings" F32 [vocab,dim]) ---

type stTensor struct {
	Dtype       string  `json:"dtype"`
	Shape       []int   `json:"shape"`
	DataOffsets []int64 `json:"data_offsets"`
}

// loadStaticTable lee el tensor "embeddings" de un safetensors: 8 bytes LE con la
// longitud del header JSON, el header, y el blob contiguo de f32 little-endian.
func loadStaticTable(path string) ([][]float32, int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(raw) < 8 {
		return nil, 0, fmt.Errorf("safetensors demasiado corto")
	}
	hlen := binary.LittleEndian.Uint64(raw[:8])
	hdrEnd := 8 + int(hlen)
	if hlen == 0 || hdrEnd > len(raw) {
		return nil, 0, fmt.Errorf("header safetensors inválido")
	}
	var hdr map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:hdrEnd], &hdr); err != nil {
		return nil, 0, fmt.Errorf("header JSON: %w", err)
	}
	rawTensor, ok := hdr["embeddings"]
	if !ok {
		return nil, 0, fmt.Errorf("safetensors sin tensor \"embeddings\"")
	}
	var ti stTensor
	if err := json.Unmarshal(rawTensor, &ti); err != nil {
		return nil, 0, fmt.Errorf("tensor embeddings: %w", err)
	}
	if ti.Dtype != "F32" || len(ti.Shape) != 2 {
		return nil, 0, fmt.Errorf("esperaba embeddings F32 2D, obtuve %s %v", ti.Dtype, ti.Shape)
	}
	vocab, dim := ti.Shape[0], ti.Shape[1]
	if len(ti.DataOffsets) != 2 {
		return nil, 0, fmt.Errorf("data_offsets inválidos")
	}
	start, end := hdrEnd+int(ti.DataOffsets[0]), hdrEnd+int(ti.DataOffsets[1])
	if start < 0 || end > len(raw) || end-start != vocab*dim*4 {
		return nil, 0, fmt.Errorf("blob de embeddings inconsistente")
	}
	blob := raw[start:end]
	table := make([][]float32, vocab)
	off := 0
	for i := 0; i < vocab; i++ {
		row := make([]float32, dim)
		for j := 0; j < dim; j++ {
			row[j] = math.Float32frombits(binary.LittleEndian.Uint32(blob[off : off+4]))
			off += 4
		}
		table[i] = row
	}
	return table, dim, nil
}

// --- WordPiece BERT hand-rolled (bit-exacto vs HuggingFace tokenizers) ---

type wordPiece struct {
	vocab                                         map[string]int
	unk                                           string
	prefix                                        string
	maxChars                                      int
	lowercase, stripAccents, cleanText, handleCJK bool
}

// loadTokenizer lee tokenizer.json y construye el tokenizer según model.type: WordPiece
// (tablas BERT, p. ej. las inglesas de POTION) o Unigram (SentencePiece, multilingües).
func loadTokenizer(path string) (tokenizer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var head struct {
		Normalizer   json.RawMessage `json:"normalizer"`
		PreTokenizer tkMetaspace     `json:"pre_tokenizer"`
		Model        struct {
			Type string `json:"type"`
		} `json:"model"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, fmt.Errorf("tokenizer.json: %w", err)
	}
	switch head.Model.Type {
	case "WordPiece":
		return newWordPiece(b)
	case "Unigram":
		var doc struct {
			Model tkUnigramModel `json:"model"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("tokenizer.json (unigram): %w", err)
		}
		return newUnigram(head.Normalizer, head.PreTokenizer, doc.Model)
	case "":
		return nil, fmt.Errorf("tokenizer.json sin model.type")
	default:
		return nil, fmt.Errorf("tokenizer model.type %q no soportado (usá WordPiece o Unigram)", head.Model.Type)
	}
}

// tkMetaspace y tkUnigramModel son vistas parciales del tokenizer.json para la rama Unigram.
type tkMetaspace struct {
	Replacement   string `json:"replacement"`
	PrependScheme string `json:"prepend_scheme"`
}

type tkUnigramModel struct {
	UnkID int                 `json:"unk_id"`
	Vocab [][]json.RawMessage `json:"vocab"`
}

// newWordPiece construye el tokenizer WordPiece BERT desde el contenido de tokenizer.json.
func newWordPiece(b []byte) (*wordPiece, error) {
	var t struct {
		Normalizer struct {
			CleanText          bool  `json:"clean_text"`
			HandleChineseChars bool  `json:"handle_chinese_chars"`
			StripAccents       *bool `json:"strip_accents"`
			Lowercase          bool  `json:"lowercase"`
		} `json:"normalizer"`
		Model struct {
			Vocab                   map[string]int `json:"vocab"`
			UnkToken                string         `json:"unk_token"`
			ContinuingSubwordPrefix string         `json:"continuing_subword_prefix"`
			MaxInputCharsPerWord    int            `json:"max_input_chars_per_word"`
		} `json:"model"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	if len(t.Model.Vocab) == 0 {
		return nil, fmt.Errorf("tokenizer.json sin vocabulario WordPiece")
	}
	strip := t.Normalizer.Lowercase // strip_accents=null sigue a lowercase
	if t.Normalizer.StripAccents != nil {
		strip = *t.Normalizer.StripAccents
	}
	unk := t.Model.UnkToken
	if unk == "" {
		unk = "[UNK]"
	}
	maxc := t.Model.MaxInputCharsPerWord
	if maxc == 0 {
		maxc = 100
	}
	return &wordPiece{
		vocab: t.Model.Vocab, unk: unk, prefix: t.Model.ContinuingSubwordPrefix,
		maxChars: maxc, lowercase: t.Normalizer.Lowercase, stripAccents: strip,
		cleanText: t.Normalizer.CleanText, handleCJK: t.Normalizer.HandleChineseChars,
	}, nil
}

func wpIsWhitespace(r rune) bool {
	if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

func wpIsControl(r rune) bool {
	if r == '\t' || r == '\n' || r == '\r' {
		return false
	}
	return unicode.In(r, unicode.Cc, unicode.Cf, unicode.Co, unicode.Cs)
}

func wpIsChineseChar(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) || (r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) || (r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) || (r >= 0x2F800 && r <= 0x2FA1F)
}

func wpIsPunctuation(r rune) bool {
	if (r >= 33 && r <= 47) || (r >= 58 && r <= 64) || (r >= 91 && r <= 96) || (r >= 123 && r <= 126) {
		return true
	}
	return unicode.In(r, unicode.P)
}

func wpStripAccents(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (w *wordPiece) normalize(text string) string {
	s := text
	if w.cleanText {
		var b strings.Builder
		for _, r := range s {
			if r == 0 || r == 0xFFFD || wpIsControl(r) {
				continue
			}
			if wpIsWhitespace(r) {
				b.WriteRune(' ')
				continue
			}
			b.WriteRune(r)
		}
		s = b.String()
	}
	if w.handleCJK {
		var b strings.Builder
		for _, r := range s {
			if wpIsChineseChar(r) {
				b.WriteRune(' ')
				b.WriteRune(r)
				b.WriteRune(' ')
			} else {
				b.WriteRune(r)
			}
		}
		s = b.String()
	}
	if w.stripAccents {
		s = wpStripAccents(s)
	}
	if w.lowercase {
		s = strings.ToLower(s)
	}
	return s
}

func (w *wordPiece) preTokenize(s string) []string {
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case wpIsWhitespace(r):
			flush()
		case wpIsPunctuation(r):
			flush()
			words = append(words, string(r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return words
}

func (w *wordPiece) tokenizeWord(word string) []string {
	runes := []rune(word)
	if len(runes) > w.maxChars {
		return []string{w.unk}
	}
	var out []string
	start := 0
	for start < len(runes) {
		end := len(runes)
		cur := ""
		found := false
		for start < end {
			sub := string(runes[start:end])
			if start > 0 {
				sub = w.prefix + sub
			}
			if _, ok := w.vocab[sub]; ok {
				cur = sub
				found = true
				break
			}
			end--
		}
		if !found {
			return []string{w.unk}
		}
		out = append(out, cur)
		start = end
	}
	return out
}

// EncodeIDs reproduce tokenizer.encode(text, add_special_tokens=false).ids
func (w *wordPiece) EncodeIDs(text string) []int {
	words := w.preTokenize(w.normalize(text))
	var ids []int
	for _, word := range words {
		for _, tok := range w.tokenizeWord(word) {
			if id, ok := w.vocab[tok]; ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
