package embedding

// spm.go implementa un tokenizer SentencePiece/Unigram (el que usan los modelos
// MULTILINGÜES de model2vec/POTION, p. ej. potion-multilingual-128M) en Go puro, para
// que el StaticProvider soporte tablas multilingües además del WordPiece BERT (inglés).
// Reproduce BIT-EXACTO el pipeline de HuggingFace tokenizers para este tipo:
//
//	1. Normalizer (Sequence): Precompiled charsmap (NFKC de SentencePiece, trie DARTS) +
//	   reglas Replace (colapso de espacios, espaciado de puntuación) + Strip.
//	2. Pre-tokenizer: Metaspace (antepone ▁ y reemplaza espacios por ▁).
//	3. Model: Unigram — segmentación por Viterbi que maximiza la suma de log-probs de las
//	   piezas del vocab, con fallback a unk para runas sin cobertura.
//
// El único punto donde nos apartamos de una re-implementación literal de HF es el
// tratamiento de secuencias DESCOMPUESTAS (base + combinante): HF procesa por grapheme y
// recompone las <6 bytes; acá aplicamos NFC ANTES del charsmap, lo que da EL MISMO
// resultado para toda entrada realista (validado bit-exacto contra el tokenizer de HF en
// casos ES/EN + NFKC duros). golang.org/x/text/unicode/norm ya es dependencia (WordPiece).

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// unkPenalty es la penalización de SentencePiece para runas sin pieza en el vocab
// (kUnkPenalty = 10.0): el score de unk = min(score de las piezas) - 10, de modo que el
// camino de Viterbi sólo emite unk cuando ninguna pieza cubre esa runa.
const unkPenalty = 10.0

// unigram es un tokenizer SentencePiece/Unigram cargado desde un tokenizer.json de HF.
type unigram struct {
	steps    []normStep     // normalizer (en orden)
	vocab    map[string]int // pieza -> id (índice en el vocab)
	scores   []float64      // id -> log-prob
	maxRunes int            // longitud (en runas) de la pieza más larga (cota de Viterbi)
	unkScore float64        // score de una runa sin cobertura
	unkID    int            // id del token unk (unk_id del modelo)
	repl     string         // símbolo de Metaspace (▁)
}

// EncodeIDs reproduce tokenizer.encode(text, add_special_tokens=false).ids para Unigram.
func (u *unigram) EncodeIDs(text string) []int {
	s := text
	for _, st := range u.steps {
		s = st.apply(s)
	}
	if s == "" {
		return nil
	}
	// Metaspace (prepend_scheme "always", split=false): un solo pre-token con ▁ al frente
	// y cada espacio convertido en ▁.
	ms := u.repl + strings.ReplaceAll(s, " ", u.repl)
	return u.viterbi([]rune(ms))
}

// viterbi encuentra la segmentación de máxima verosimilitud (suma de log-probs) sobre las
// runas dadas, con fallback a unk por runa. Determinista, model-free.
func (u *unigram) viterbi(runes []rune) []int {
	n := len(runes)
	const ninf = -1e18
	best := make([]float64, n+1)
	bpID := make([]int, n+1)
	bpPrev := make([]int, n+1)
	for i := 1; i <= n; i++ {
		best[i] = ninf
	}
	for i := 0; i < n; i++ {
		if best[i] == ninf {
			continue
		}
		lim := u.maxRunes
		if lim > n-i {
			lim = n - i
		}
		for l := 1; l <= lim; l++ {
			if id, ok := u.vocab[string(runes[i:i+l])]; ok {
				if sc := best[i] + u.scores[id]; sc > best[i+l] {
					best[i+l] = sc
					bpID[i+l] = id
					bpPrev[i+l] = i
				}
			}
		}
		// Fallback unk (id 1): una runa sin pieza. unkScore es muy negativo, así que sólo
		// gana cuando no hay ninguna pieza que cubra esta posición.
		if sc := best[i] + u.unkScore; sc > best[i+1] {
			best[i+1] = sc
			bpID[i+1] = u.unkID
			bpPrev[i+1] = i
		}
	}
	var ids []int
	for pos := n; pos > 0; pos = bpPrev[pos] {
		ids = append([]int{bpID[pos]}, ids...)
	}
	return ids
}

// --- normalizer: pasos en orden (Precompiled / Replace / Strip) ---

type normStep interface{ apply(string) string }

// precompiledStep aplica el charsmap de SentencePiece: NFC (para componer secuencias
// descompuestas como hace HF por grapheme) y luego, runa por runa, la forma normalizada
// del trie (identidad si no hay entrada). NO usa longest-match global: HF procesa por
// carácter, así que una entrada multi-char del trie no debe "tragarse" runas contiguas.
type precompiledStep struct{ d *darts }

func (p precompiledStep) apply(s string) string {
	b := []byte(norm.NFC.String(s))
	var out strings.Builder
	out.Grow(len(b))
	i := 0
	for i < len(b) {
		_, sz := utf8.DecodeRune(b[i:])
		chunk := b[i : i+sz]
		if mlen, mval, ok := p.d.match(chunk); ok && mlen == sz {
			out.WriteString(p.d.normString(mval))
		} else {
			out.Write(chunk)
		}
		i += sz
	}
	return out.String()
}

// replaceStep es una regla Replace del normalizer: literal (pattern.String) o por regex
// (pattern.Regex). re == nil ⇒ reemplazo literal.
type replaceStep struct {
	re      *regexp.Regexp
	literal string
	content string
}

func (r replaceStep) apply(s string) string {
	if r.re != nil {
		return r.re.ReplaceAllString(s, r.content)
	}
	return strings.ReplaceAll(s, r.literal, r.content)
}

// stripStep recorta espacios en blanco de los extremos (Strip de HF).
type stripStep struct{}

func (stripStep) apply(s string) string { return strings.TrimSpace(s) }

// --- darts-clone double-array trie (formato precompiled_charsmap de SentencePiece) ---

// darts decodifica el precompiled_charsmap: 4 bytes uint32 LE con el tamaño del trie, el
// blob del trie (array de uint32) y el blob `norm` de formas normalizadas (strings
// terminados en NUL; el valor de una hoja es el offset en este blob).
type darts struct {
	units []uint32
	norm  []byte
}

func newDarts(raw []byte) (*darts, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("charsmap demasiado corto")
	}
	trieSize := binary.LittleEndian.Uint32(raw[:4])
	if 4+int(trieSize) > len(raw) || trieSize%4 != 0 {
		return nil, fmt.Errorf("charsmap con tamaño de trie inválido")
	}
	trieBlob := raw[4 : 4+trieSize]
	units := make([]uint32, trieSize/4)
	for i := range units {
		units[i] = binary.LittleEndian.Uint32(trieBlob[i*4:])
	}
	return &darts{units: units, norm: raw[4+trieSize:]}, nil
}

func dartsHasLeaf(u uint32) bool { return (u>>8)&1 == 1 }
func dartsValue(u uint32) uint32 { return u & 0x7fffffff }
func dartsLabel(u uint32) uint32 { return u & 0x800000ff }
func dartsOffset(u uint32) uint32 {
	return (u >> 10) << ((u & 0x200) >> 6)
}

// match hace un commonPrefixSearch darts-clone y devuelve la longitud (en bytes) y el
// valor (offset en `norm`) del prefijo MÁS LARGO de key presente en el trie.
func (d *darts) match(key []byte) (int, uint32, bool) {
	pos := uint32(0)
	unit := d.units[pos]
	pos ^= dartsOffset(unit)
	bestLen, bestVal := 0, uint32(0)
	found := false
	for i := 0; i < len(key); i++ {
		pos ^= uint32(key[i])
		if int(pos) >= len(d.units) {
			break
		}
		unit = d.units[pos]
		if dartsLabel(unit) != uint32(key[i]) {
			break
		}
		pos ^= dartsOffset(unit)
		if dartsHasLeaf(unit) {
			if int(pos) >= len(d.units) {
				break
			}
			bestLen = i + 1
			bestVal = dartsValue(d.units[pos])
			found = true
		}
	}
	return bestLen, bestVal, found
}

// normString lee la forma normalizada (string terminado en NUL) que empieza en el offset.
func (d *darts) normString(off uint32) string {
	end := off
	for end < uint32(len(d.norm)) && d.norm[end] != 0 {
		end++
	}
	return string(d.norm[off:end])
}

// --- parsing de tokenizer.json (rama Unigram) ---

// newUnigram construye el tokenizer Unigram desde el tokenizer.json ya deserializado
// parcialmente (normalizer + pre_tokenizer + model). Devuelve error si algún componente
// no es el esperado (Metaspace, vocab no vacío, etc.).
func newUnigram(normalizer json.RawMessage, metaspace tkMetaspace, model tkUnigramModel) (*unigram, error) {
	if len(model.Vocab) == 0 {
		return nil, fmt.Errorf("tokenizer Unigram sin vocabulario")
	}
	steps, err := parseNormalizer(normalizer)
	if err != nil {
		return nil, err
	}
	repl := metaspace.Replacement
	if repl == "" {
		repl = "▁" // ▁ por defecto
	}
	unkID := model.UnkID
	if unkID < 0 || unkID >= len(model.Vocab) {
		unkID = 1 // [UNK] convencional
	}
	u := &unigram{
		steps:    steps,
		vocab:    make(map[string]int, len(model.Vocab)),
		scores:   make([]float64, len(model.Vocab)),
		maxRunes: 1,
		unkID:    unkID,
		repl:     repl,
	}
	minScore := 0.0
	for id, entry := range model.Vocab {
		tok, sc, perr := parseVocabEntry(entry)
		if perr != nil {
			return nil, fmt.Errorf("vocab[%d]: %w", id, perr)
		}
		u.vocab[tok] = id
		u.scores[id] = sc
		if l := len([]rune(tok)); l > u.maxRunes {
			u.maxRunes = l
		}
		if sc < minScore {
			minScore = sc
		}
	}
	u.unkScore = minScore - unkPenalty
	return u, nil
}

// parseNormalizer aplana un normalizer (posiblemente un Sequence anidado) a una lista
// ordenada de pasos. Soporta Sequence, Precompiled, Replace y Strip; cualquier otro tipo
// es un error (mejor fallar que divergir en silencio del tokenizer de referencia).
func parseNormalizer(raw json.RawMessage) ([]normStep, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("normalizer: %w", err)
	}
	switch head.Type {
	case "Sequence":
		var seq struct {
			Normalizers []json.RawMessage `json:"normalizers"`
		}
		if err := json.Unmarshal(raw, &seq); err != nil {
			return nil, err
		}
		var out []normStep
		for _, n := range seq.Normalizers {
			sub, err := parseNormalizer(n)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
		return out, nil
	case "Precompiled":
		var p struct {
			Charsmap string `json:"precompiled_charsmap"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		cm, err := base64.StdEncoding.DecodeString(p.Charsmap)
		if err != nil {
			return nil, fmt.Errorf("precompiled_charsmap base64: %w", err)
		}
		d, err := newDarts(cm)
		if err != nil {
			return nil, err
		}
		return []normStep{precompiledStep{d: d}}, nil
	case "Replace":
		var r struct {
			Pattern struct {
				String string `json:"String"`
				Regex  string `json:"Regex"`
			} `json:"pattern"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		if r.Pattern.Regex != "" {
			re, err := regexp.Compile(r.Pattern.Regex)
			if err != nil {
				return nil, fmt.Errorf("normalizer Replace regex %q: %w", r.Pattern.Regex, err)
			}
			return []normStep{replaceStep{re: re, content: r.Content}}, nil
		}
		return []normStep{replaceStep{literal: r.Pattern.String, content: r.Content}}, nil
	case "Strip":
		return []normStep{stripStep{}}, nil
	case "":
		return nil, fmt.Errorf("normalizer sin campo type")
	default:
		return nil, fmt.Errorf("normalizer tipo %q no soportado", head.Type)
	}
}

// parseVocabEntry lee una entrada del vocab Unigram: ["pieza", log_prob].
func parseVocabEntry(entry []json.RawMessage) (string, float64, error) {
	if len(entry) != 2 {
		return "", 0, fmt.Errorf("esperaba [pieza, score], obtuve %d elementos", len(entry))
	}
	var tok string
	if err := json.Unmarshal(entry[0], &tok); err != nil {
		return "", 0, fmt.Errorf("pieza: %w", err)
	}
	var sc float64
	if err := json.Unmarshal(entry[1], &sc); err != nil {
		return "", 0, fmt.Errorf("score: %w", err)
	}
	return tok, sc, nil
}
