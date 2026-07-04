package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"unicode"
)

// digest.go contiene utilidades MODEL-FREE (deterministas, sin LLM) para la
// memoria eficiente de Musubi: estimación de tokens, gist extractivo y hash de
// contenido para deduplicar. Son la base del recall por presupuesto.

// Divisores chars/token por tipo de contenido (ASCII) por defecto. Cuanto más
// denso en símbolos, menor el divisor (más tokens por carácter). Conservadores.
const (
	defaultDivProse = 4.0 // prosa natural ~4 chars/token
	defaultDivCode  = 3.4 // código fuente ~3.4 (símbolos, identificadores cortados)
	defaultDivJSON  = 2.6 // JSON ~2.6 (comillas, llaves, dos puntos, comas)
)

// divNonASCII es el divisor de los caracteres no-ASCII no-CJK (acentos, símbolos,
// emoji): el tokenizer real suele partirlos en MÁS tokens que el ASCII común, así que
// se cuentan más densos (~0.5 tok/char) para NO sub-estimar prosa acentuada (español).
// Fijo, no calibrable: corrige un sesgo estructural (por-carácter), no un promedio.
const divNonASCII = 2.0

// Divisores activos. Son los defaults salvo que una calibración opt-in
// (musubi calibrate --apply, vía count_tokens) los ajuste y persista. Se cargan
// desde la DB al abrir el motor; nunca cambian en el camino del server.
var (
	divProse = defaultDivProse
	divCode  = defaultDivCode
	divJSON  = defaultDivJSON
)

// ConfigureDivisors ajusta los divisores activos (ignora valores <= 0).
func ConfigureDivisors(prose, code, jsn float64) {
	if prose > 0 {
		divProse = prose
	}
	if code > 0 {
		divCode = code
	}
	if jsn > 0 {
		divJSON = jsn
	}
}

// ResetDivisors restaura los divisores por defecto.
func ResetDivisors() {
	divProse, divCode, divJSON = defaultDivProse, defaultDivCode, defaultDivJSON
}

// CurrentDivisors devuelve los divisores activos (prose, code, json).
func CurrentDivisors() (float64, float64, float64) {
	return divProse, divCode, divJSON
}

// defaultGistMaxTokens es el tope de tokens de un gist cuando no se configura otro
// (usado por el backfill y como valor por defecto del recall).
const defaultGistMaxTokens = 24

// tokenEstimatorVersion identifica la versión del estimador de tokens. Al
// cambiar (nuevos divisores/heurística), la migración recomputa la columna
// `tokens` de las filas existentes para que el presupuesto siga siendo coherente.
const tokenEstimatorVersion = "v3-seg-nonascii"

// metaTokenEstimatorVersion es la clave de meta donde se persiste la versión con
// la que se computó por última vez la columna `tokens`.
const metaTokenEstimatorVersion = "token_estimator_version"

// contentKind clasifica el contenido para elegir el divisor de tokens adecuado.
type contentKind int

const (
	kindProse contentKind = iota
	kindCode
	kindJSON
)

// classifyContent infiere el tipo de contenido con features baratas y
// deterministas (sin LLM): JSON si abre con {/[ y tiene estructura de objeto;
// código si la densidad de símbolos es alta o hay fences; si no, prosa.
func classifyContent(s string) contentKind {
	t := strings.TrimSpace(s)
	if t == "" {
		return kindProse
	}
	if looksLikeJSON(t) {
		return kindJSON
	}
	if strings.Contains(t, "```") {
		return kindCode
	}
	// Densidad de símbolos típicos de código sobre el total de runas.
	const codeSymbols = "{}()[];=<>/\\|&*+_$#@"
	var sym, total int
	for _, r := range t {
		total++
		if strings.ContainsRune(codeSymbols, r) {
			sym++
		}
	}
	if total > 0 && float64(sym)/float64(total) >= 0.06 {
		return kindCode
	}
	return kindProse
}

// EstimateTokens estima de forma determinista cuántos tokens ocupa un texto. Es una
// aproximación model-free sesgada a no subcontar: JSON estructural se estima como blob;
// el markdown con fences de código se estima POR SEGMENTO (prosa vs código) para que un
// solo bloque no clasifique todo como código y sobre-estime; el resto va por densidad.
func EstimateTokens(s string) int {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0
	}
	if looksLikeJSON(t) {
		return estimateTokensFor(s, kindJSON)
	}
	if strings.Contains(t, "```") {
		return estimateSegmented(s)
	}
	return estimateTokensFor(s, classifyContent(s))
}

// looksLikeJSON detecta un objeto/array JSON por su encabezado: barato y determinista.
func looksLikeJSON(t string) bool {
	if t == "" {
		return false
	}
	c := t[0]
	return (c == '{' || c == '[') && strings.ContainsRune(t, '"') &&
		(strings.ContainsRune(t, ':') || strings.ContainsRune(t, ','))
}

// estimateSegmented estima un contenido markdown separando los bloques de código (entre
// fences ```) de la prosa y sumando cada parte con su divisor. Corrige el sesgo por el
// que un único fence hacía estimar TODO el blob como código (~17% de sobre-estimación
// que recortaba el recall). Determinista, sin LLM.
func estimateSegmented(s string) int {
	var prose, code strings.Builder
	inFence := false
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence // el marcador del fence cuenta como código
			code.WriteString(line)
			code.WriteByte('\n')
			continue
		}
		if inFence {
			code.WriteString(line)
			code.WriteByte('\n')
		} else {
			prose.WriteString(line)
			prose.WriteByte('\n')
		}
	}
	return estimateTokensFor(prose.String(), kindProse) + estimateTokensFor(code.String(), kindCode)
}

// estimateTokensFor estima tokens para un tipo de contenido dado. Los CJK pesan ~1 token
// cada uno; los no-ASCII no-CJK (acentos/emoji) se cuentan más densos (divNonASCII) para
// no sub-estimar prosa acentuada; el ASCII usa el divisor calibrado del tipo.
func estimateTokensFor(s string, kind contentKind) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	ascii, nonascii, cjk := countChars(s)
	div := divisorFor(kind)
	tokens := float64(ascii)/div + float64(nonascii)/divNonASCII + float64(cjk)
	return int(math.Ceil(tokens))
}

// countChars separa las runas en ASCII, no-ASCII no-CJK (acentos/símbolos/emoji) y CJK,
// cada grupo con su propio peso de tokens.
func countChars(s string) (ascii, nonascii, cjk int) {
	for _, r := range s {
		switch {
		case isCJK(r):
			cjk++
		case r < 128:
			ascii++
		default:
			nonascii++
		}
	}
	return ascii, nonascii, cjk
}

// divisorFor devuelve el divisor activo del tipo de contenido.
func divisorFor(kind contentKind) float64 {
	switch kind {
	case kindCode:
		return divCode
	case kindJSON:
		return divJSON
	default:
		return divProse
	}
}

// isCJK indica si la runa es un ideograma/silabario CJK (cada uno ~1 token).
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// ContentHash devuelve el sha256 (hex) del contenido normalizado por espacios,
// para detectar duplicados exactos al guardar.
func ContentHash(content string) string {
	norm := strings.Join(strings.Fields(content), " ")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// markdownLead son los caracteres de marcado que se recortan al inicio de un
// contenido para producir un gist limpio (encabezados, listas, citas, fences).
const markdownLead = "#>-*`~ \t\r\n"

// Gist devuelve un titular extractivo del contenido, acotado a maxTokens. Toma la
// primera oración; si excede el presupuesto, la trunca a maxTokens respetando
// límites de palabra y runa, y agrega una elipsis. Es determinista y sin LLM.
func Gist(content string, maxTokens int) string {
	if maxTokens <= 0 {
		maxTokens = 24
	}

	norm := strings.Join(strings.Fields(strings.TrimLeft(content, markdownLead)), " ")
	if norm == "" {
		return ""
	}

	lead := firstSentence(norm)
	if EstimateTokens(lead) <= maxTokens {
		return lead
	}
	return truncateToTokens(lead, maxTokens)
}

// firstSentence devuelve el texto hasta el primer terminador de oración (. ! ?)
// seguido de espacio o fin de cadena. Si no hay terminador, devuelve todo norm.
func firstSentence(norm string) string {
	for i := 0; i < len(norm); i++ {
		c := norm[i]
		if c == '.' || c == '!' || c == '?' {
			// Boundary real: fin de cadena o espacio a continuación
			// (evita cortar en "v1.0").
			if i+1 >= len(norm) || norm[i+1] == ' ' {
				return norm[:i+1]
			}
		}
	}
	return norm
}

// truncateToTokens recorta s para que EstimateTokens(resultado) <= maxTokens, en
// límite de palabra y de runa, dejando lugar para la elipsis. Usa el mismo
// estimador (por tipo) que el presupuesto, achicando hasta entrar; determinista.
func truncateToTokens(s string, maxTokens int) string {
	if maxTokens <= 0 {
		return "…"
	}
	if EstimateTokens(s) <= maxTokens {
		return s
	}
	runes := []rune(s)
	// Cota superior de runas: el divisor más grande (prosa) maximiza chars/token.
	cut := int(float64(maxTokens) * divProse)
	if cut >= len(runes) {
		cut = len(runes) - 1
	}
	for cut > 0 {
		trunc := string(runes[:cut])
		if idx := strings.LastIndex(trunc, " "); idx > 0 {
			trunc = trunc[:idx]
		}
		cand := strings.TrimRight(trunc, " ") + "…"
		if EstimateTokens(cand) <= maxTokens {
			return cand
		}
		cut = cut * 3 / 4 // achicar y reintentar
	}
	return "…"
}
