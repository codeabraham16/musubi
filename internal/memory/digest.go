package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// digest.go contiene utilidades MODEL-FREE (deterministas, sin LLM) para la
// memoria eficiente de Musubi: estimación de tokens, gist extractivo y hash de
// contenido para deduplicar. Son la base del recall por presupuesto.

// charsPerToken es la heurística de conversión caracteres->tokens (~4 chars/token).
const charsPerToken = 4

// defaultGistMaxTokens es el tope de tokens de un gist cuando no se configura otro
// (usado por el backfill y como valor por defecto del recall).
const defaultGistMaxTokens = 24

// EstimateTokens estima de forma determinista cuántos tokens ocupa un texto
// (~1 token cada 4 runas, redondeo hacia arriba). Es una aproximación suficiente
// para presupuestar recalls sin depender de un tokenizador real.
func EstimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := utf8.RuneCountInString(s)
	return (n + charsPerToken - 1) / charsPerToken // ceil(n/4)
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

// truncateToTokens recorta s a ~maxTokens, en límite de palabra y de runa,
// dejando lugar para la elipsis final.
func truncateToTokens(s string, maxTokens int) string {
	limit := maxTokens * charsPerToken
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}

	cut := limit - 1 // reservar una runa para la elipsis
	if cut < 1 {
		cut = 1
	}
	trunc := string(runes[:cut])
	if idx := strings.LastIndex(trunc, " "); idx > 0 {
		trunc = trunc[:idx]
	}
	return strings.TrimRight(trunc, " ") + "…"
}
