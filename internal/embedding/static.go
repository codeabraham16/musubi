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
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
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

// StaticProvider genera embeddings por lookup + mean-pool sobre una tabla estática. La
// tabla se guarda PLANA (un solo []float32 de vocab*dim, la fila id ocupa
// [id*dim : id*dim+dim]) en vez de [][]float32: para tablas grandes (p. ej. la multilingüe
// de 500K×256 = ~488MB) evita ~500K headers de slice y mejora la localidad de caché.
type StaticProvider struct {
	table   []float32 // vocab*dim, row-major; la fila id es table[id*dim : (id+1)*dim]
	rows    int       // cantidad de filas (tamaño del vocab)
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
	table, rows, dim, crcTabla, nTabla, err := cargarTablaEnStreaming(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		return nil, fmt.Errorf("tabla estática: %w", err)
	}
	tokRaw, err := os.ReadFile(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		return nil, fmt.Errorf("tokenizer: %w", err)
	}
	tok, err := loadTokenizerBytes(tokRaw)
	if err != nil {
		return nil, fmt.Errorf("tokenizer: %w", err)
	}
	return &StaticProvider{
		table: table,
		rows:  rows,
		dim:   dim,
		tok:   tok,
		// N1: la identidad lleva el CONTENIDO, no sólo el nombre de la carpeta. Con sólo el
		// basename, re-destilar la tabla in-place NO cambiaba el model_id: los vectores viejos
		// seguían pareciendo compatibles y la búsqueda los comparaba por coseno contra los de la
		// tabla nueva ⇒ ranking corrupto EN SILENCIO. Con el checksum, una tabla distinta es una
		// identidad distinta y el contrato de procedencia (F2.2) excluye sola a los vectores viejos.
		modelID: "static:" + filepath.Base(filepath.Clean(dir)) + "@" +
			checksumDeCRC(crcTabla, nTabla, crc32.Checksum(tokRaw, castagnoli), int64(len(tokRaw))),
	}, nil
}

// trozoDeCarga es el buffer con el que se lee la tabla. Múltiplo de 4 —el tamaño de un float32—
// para que un trozo nunca parta un valor por la mitad.
const trozoDeCarga = 1 << 20

// topeDeHeader acota el JSON del safetensors antes de reservarle memoria: sin esto, un archivo
// corrupto que declare un header de 2^63 bytes hace que `make` pida esa cantidad. El header real
// de POTION son 80 bytes; 8 MB es holgado por varios órdenes.
const topeDeHeader = 8 << 20

// cargarTablaEnStreaming lee model.safetensors UNA sola vez, en trozos, y hace DOS cosas sobre cada
// trozo: acumula el CRC del archivo entero y convierte los bytes del blob a la tabla []float32 ya
// reservada. Devuelve la tabla, su forma, y el CRC y el tamaño que la identidad necesita.
//
// POR QUÉ NO `os.ReadFile` + `parseStaticTable`. Esa pareja tiene VIVOS AL MISMO TIEMPO los bytes
// crudos (488 MB en la tabla multilingüe) y su conversión a float32 (otros 488 MB). Medido sobre un
// `musubi daemon` real arrancando en este repo: pasa de 10 MB a **1353 MB de memoria anónima en
// menos de 8 segundos**. Leyendo en streaming el pico baja a la tabla sola.
//
// NO ES UNA MICRO-OPTIMIZACIÓN, Y EL NÚMERO ES EL ARGUMENTO. La máquina donde corre esto tiene
// 7,6 GB de RAM con el swap en uso y zram encima —o sea que lo swapeado NO liberó RAM, la
// comprimió—. Y hay un daemon por sesión de agente: tres vivos son ~2 GB de tabla duplicada. Con
// ese pico, `ENOMEM` en el ReadFile no es hipotético.
//
// LA IDENTIDAD SALE IDÉNTICA BIT A BIT, que es lo que vuelve seguro el cambio: CRC32 es
// incremental —`crc32.New(tab)` + `Write` por trozos da el MISMO uint32 que `crc32.Checksum` sobre
// el buffer entero— y el largo se cuenta sobre los bytes EFECTIVAMENTE leídos, no con `os.Stat`,
// así un archivo que crece mientras se lee no puede mentir sobre su tamaño. La guarda que lo fija
// es TestLaCargaEnStreamingDaLaMismaIdentidadYLosMismosValores.
func cargarTablaEnStreaming(ruta string) (tabla []float32, filas, dim int, crc uint32, leidos int64, err error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	defer func() { _ = f.Close() }()

	h := crc32.New(castagnoli)

	// (1) EL ENCABEZADO, que es chico: 8 bytes con el largo del JSON, y el JSON.
	var largo [8]byte
	if _, err := io.ReadFull(f, largo[:]); err != nil {
		return nil, 0, 0, 0, 0, fmt.Errorf("safetensors demasiado corto: %w", err)
	}
	_, _ = h.Write(largo[:])
	leidos = 8
	hlen := binary.LittleEndian.Uint64(largo[:])
	if hlen == 0 || hlen > topeDeHeader {
		return nil, 0, 0, 0, 0, fmt.Errorf("header safetensors inválido: declara %d bytes", hlen)
	}
	hdrRaw := make([]byte, hlen)
	if _, err := io.ReadFull(f, hdrRaw); err != nil {
		return nil, 0, 0, 0, 0, fmt.Errorf("header safetensors truncado: %w", err)
	}
	_, _ = h.Write(hdrRaw)
	leidos += int64(hlen)

	inicio, fin, filas, dim, err := ubicarEmbeddings(hdrRaw, leidos)
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}

	// (2) EL RESTO DEL ARCHIVO, en trozos. Lo que cae dentro del blob se convierte; lo que no,
	// se saltea. Todo se hashea, porque la identidad es del ARCHIVO y no sólo del tensor.
	tabla = make([]float32, filas*dim)
	buf := make([]byte, trozoDeCarga)
	var sobra [4]byte // bytes de un float32 partido entre dos trozos
	nSobra := 0
	escritos := 0
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
			trozoIni := leidos
			leidos += int64(n)
			// Recortar el trozo a la parte que cae dentro del blob.
			desde := max(trozoIni, inicio)
			hasta := min(leidos, fin)
			if desde < hasta {
				datos := buf[desde-trozoIni : hasta-trozoIni]
				escritos, nSobra = convertirTrozo(datos, tabla, escritos, sobra[:], nSobra)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, 0, 0, 0, 0, fmt.Errorf("leyendo la tabla: %w", rerr)
		}
	}
	if leidos < fin {
		return nil, 0, 0, 0, 0, fmt.Errorf("blob de embeddings truncado: el header declara que termina en %d y el archivo tiene %d bytes", fin, leidos)
	}
	if escritos != len(tabla) || nSobra != 0 {
		return nil, 0, 0, 0, 0, fmt.Errorf("blob de embeddings inconsistente: se convirtieron %d valores de %d (y sobraron %d bytes)", escritos, len(tabla), nSobra)
	}
	return tabla, filas, dim, h.Sum32(), leidos, nil
}

// convertirTrozo pasa `datos` a float32 little-endian dentro de `tabla` a partir de `escritos`,
// arrastrando el float32 que haya quedado partido entre este trozo y el anterior. Devuelve cuántos
// valores hay escritos en total y cuántos bytes quedaron a medias para el trozo siguiente.
func convertirTrozo(datos []byte, tabla []float32, escritos int, sobra []byte, nSobra int) (int, int) {
	// Primero, completar el valor que quedó partido.
	if nSobra > 0 {
		falta := 4 - nSobra
		if len(datos) < falta {
			nSobra += copy(sobra[nSobra:], datos)
			return escritos, nSobra
		}
		copy(sobra[nSobra:], datos[:falta])
		if escritos < len(tabla) {
			tabla[escritos] = math.Float32frombits(binary.LittleEndian.Uint32(sobra))
			escritos++
		}
		datos = datos[falta:]
		nSobra = 0
	}
	enteros := len(datos) / 4
	if enteros > len(tabla)-escritos {
		enteros = len(tabla) - escritos
	}
	for i := 0; i < enteros; i++ {
		tabla[escritos+i] = math.Float32frombits(binary.LittleEndian.Uint32(datos[i*4:]))
	}
	escritos += enteros
	if resto := len(datos) - enteros*4; resto > 0 && resto < 4 {
		nSobra = copy(sobra, datos[enteros*4:])
	}
	return escritos, nSobra
}

// ubicarEmbeddings lee del header del safetensors la forma del tensor "embeddings" y dónde
// empieza y termina su blob, en offsets ABSOLUTOS del archivo. `hdrEnd` es el byte donde termina
// el encabezado (8 + largo del JSON).
func ubicarEmbeddings(hdrRaw []byte, hdrEnd int64) (inicio, fin int64, filas, dim int, err error) {
	var hdr map[string]json.RawMessage
	if err := json.Unmarshal(hdrRaw, &hdr); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("header JSON: %w", err)
	}
	crudo, ok := hdr["embeddings"]
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("safetensors sin tensor \"embeddings\"")
	}
	var ti stTensor
	if err := json.Unmarshal(crudo, &ti); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("tensor embeddings: %w", err)
	}
	if ti.Dtype != "F32" || len(ti.Shape) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("esperaba embeddings F32 2D, obtuve %s %v", ti.Dtype, ti.Shape)
	}
	if len(ti.DataOffsets) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("data_offsets inválidos")
	}
	filas, dim = ti.Shape[0], ti.Shape[1]
	if filas <= 0 || dim <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("forma inválida: %dx%d", filas, dim)
	}
	inicio, fin = hdrEnd+ti.DataOffsets[0], hdrEnd+ti.DataOffsets[1]
	if ti.DataOffsets[0] < 0 || fin < inicio || fin-inicio != int64(filas)*int64(dim)*4 {
		return 0, 0, 0, 0, fmt.Errorf("blob de embeddings inconsistente: %d bytes para %dx%d", fin-inicio, filas, dim)
	}
	return inicio, fin, filas, dim, nil
}

// castagnoli es la tabla CRC32-C (polinomio Castagnoli), que Go acelera por HARDWARE (SSE4.2/ARM).
var castagnoli = crc32.MakeTable(crc32.Castagnoli)

// staticTableChecksum deriva la identidad de CONTENIDO de la tabla. Entran los DOS archivos porque
// los dos cambian los vectores que produce el provider: otra tabla ⇒ otros vectores; otro tokenizer
// ⇒ otra tokenización ⇒ otro mean-pool. Sólo depende de los bytes: mismo contenido ⇒ mismo checksum,
// sin importar mtime ni ruta.
//
// CRC32-C y no SHA-256: MEDIDO sobre la tabla real (488MB), SHA-256 tarda ~1.16s y CRC32-C ~39ms
// (30x). No es una micro-optimización: `musubi capture` (la captura automática) carga la tabla en
// cada corrida, así que un hash lento le DUPLICARÍA el arranque. Esto no es un uso criptográfico:
// sólo hay que detectar que la tabla cambió, no resistir un adversario. Para compensar los 32 bits
// del CRC se mezclan además los TAMAÑOS de ambos archivos: una colisión tendría que coincidir en CRC
// **y** en longitud exacta, en los dos archivos a la vez. El sha256 final es sobre 24 bytes (gratis)
// y sólo sirve para compactar todo eso en un id corto y legible.
func staticTableChecksum(tableRaw, tokRaw []byte) string {
	return checksumDeCRC(crc32.Checksum(tableRaw, castagnoli), int64(len(tableRaw)),
		crc32.Checksum(tokRaw, castagnoli), int64(len(tokRaw)))
}

// checksumDeCRC es LA derivación de la identidad, y la única. Toma los cuatro números que la
// definen en vez de los bytes, para que el camino en streaming (cargarTablaEnStreaming, que nunca
// tiene el archivo entero en memoria) y el camino sobre buffers (staticTableChecksum, que usan las
// pruebas) produzcan el MISMO id sin que haya dos implementaciones que se puedan desfasar.
//
// Es la diferencia entre derivar y copiar: escrita dos veces, la próxima vez que se toque el
// formato del seed una de las dos se queda atrás y los vectores viejos dejan de reconocerse —en
// silencio, que es como duele.
func checksumDeCRC(crcTabla uint32, nTabla int64, crcTok uint32, nTok int64) string {
	var seed [24]byte
	binary.BigEndian.PutUint32(seed[0:4], crcTabla)
	binary.BigEndian.PutUint64(seed[4:12], uint64(nTabla))
	binary.BigEndian.PutUint32(seed[12:16], crcTok)
	binary.BigEndian.PutUint64(seed[16:24], uint64(nTok))
	sum := sha256.Sum256(seed[:])
	return hex.EncodeToString(sum[:])[:12]
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
		if id < 0 || id >= p.rows {
			continue
		}
		base := id * p.dim
		for j := 0; j < p.dim; j++ {
			acc[j] += float64(p.table[base+j])
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

// parseStaticTable interpreta el tensor "embeddings" de un safetensors YA LEÍDO: 8 bytes LE con
// la longitud del header JSON, el header, y el blob contiguo de f32 little-endian. Devuelve la
// tabla PLANA (vocab*dim, row-major), el número de filas (vocab) y la dimensión. Toma los bytes
// (y no la ruta) para que el caller pueda hashearlos sin releer el archivo (N1).
func parseStaticTable(raw []byte) ([]float32, int, int, error) {
	if len(raw) < 8 {
		return nil, 0, 0, fmt.Errorf("safetensors demasiado corto")
	}
	hlen := binary.LittleEndian.Uint64(raw[:8])
	// LA COTA SE COMPARA EN uint64, ANTES DE CONVERTIR. Con `hdrEnd := 8 + int(hlen)` un `hlen` de
	// 2^63 o más se da VUELTA al convertir —`int(0xFFFFFFFFFFFFFFFF)` es -1—, `hdrEnd` queda
	// negativo o chico, la comparación contra `len(raw)` lo deja pasar, y el `raw[8:hdrEnd]` de
	// abajo larga `slice bounds out of range`. Medido el 2026-09-12 con tres valores.
	//
	// El largo declarado es un número que viene del ARCHIVO, o sea de afuera: un safetensors
	// corrupto o fabricado lo elige. Esto es la misma amenaza que `topeDeHeader` cubre en el
	// camino de streaming, y acá no estaba cubierta.
	if hlen == 0 || hlen > uint64(len(raw)-8) {
		return nil, 0, 0, fmt.Errorf("header safetensors inválido: declara %d bytes y el archivo tiene %d", hlen, len(raw))
	}
	hdrEnd := 8 + int(hlen)
	var hdr map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:hdrEnd], &hdr); err != nil {
		return nil, 0, 0, fmt.Errorf("header JSON: %w", err)
	}
	rawTensor, ok := hdr["embeddings"]
	if !ok {
		return nil, 0, 0, fmt.Errorf("safetensors sin tensor \"embeddings\"")
	}
	var ti stTensor
	if err := json.Unmarshal(rawTensor, &ti); err != nil {
		return nil, 0, 0, fmt.Errorf("tensor embeddings: %w", err)
	}
	if ti.Dtype != "F32" || len(ti.Shape) != 2 {
		return nil, 0, 0, fmt.Errorf("esperaba embeddings F32 2D, obtuve %s %v", ti.Dtype, ti.Shape)
	}
	vocab, dim := ti.Shape[0], ti.Shape[1]
	if len(ti.DataOffsets) != 2 {
		return nil, 0, 0, fmt.Errorf("data_offsets inválidos")
	}
	start, end := hdrEnd+int(ti.DataOffsets[0]), hdrEnd+int(ti.DataOffsets[1])
	if start < 0 || end > len(raw) || end-start != vocab*dim*4 {
		return nil, 0, 0, fmt.Errorf("blob de embeddings inconsistente")
	}
	blob := raw[start:end]
	table := make([]float32, vocab*dim) // plano: la fila id es table[id*dim : (id+1)*dim]
	for i := range table {
		table[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
	}
	return table, vocab, dim, nil
}

// --- WordPiece BERT hand-rolled (bit-exacto vs HuggingFace tokenizers) ---

type wordPiece struct {
	vocab                                         map[string]int
	unk                                           string
	prefix                                        string
	maxChars                                      int
	lowercase, stripAccents, cleanText, handleCJK bool
}

// loadTokenizer lee tokenizer.json y construye el tokenizer. Wrapper por RUTA sobre
// loadTokenizerBytes (que es el que usa NewStaticProvider, para hashear los bytes sin releerlos).
func loadTokenizer(path string) (tokenizer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadTokenizerBytes(b)
}

// loadTokenizerBytes construye el tokenizer desde un tokenizer.json YA LEÍDO, según model.type:
// WordPiece (tablas BERT, p. ej. las inglesas de POTION) o Unigram (SentencePiece, multilingües).
func loadTokenizerBytes(b []byte) (tokenizer, error) {
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
