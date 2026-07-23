package ingest

import (
	"regexp"
	"strings"
)

// subtitles.go convierte los subtítulos crudos (VTT/SRT) que baja yt-dlp en texto plano legible.
// Es la lógica validada a mano el 2026-07-23 (prototipo Python) portada a Go: los auto-captions de
// YouTube traen timestamps inline, tags de posición y la línea RODANTE repetida en cada cue, así que
// hay que limpiarlos y deduplicar. Model-free y determinista.

var (
	subTag     = regexp.MustCompile(`<[^>]+>`)    // <c>, </c>, <00:00:01.000> (timestamps inline)
	subBracket = regexp.MustCompile(`\[[^\]]*\]`) // [Música], [Aplausos]
	subCueNum  = regexp.MustCompile(`^\d+$`)      // número de cue de un SRT
	subWS      = regexp.MustCompile(`\s+`)
)

// CleanSubtitles normaliza un VTT/SRT a texto plano: descarta cabeceras/timestamps/números de cue,
// quita tags <...> y anotaciones [..], y deduplica las líneas consecutivas repetidas (la rodante de
// los auto-captions). Devuelve las frases unidas por espacio.
func CleanSubtitles(raw string) string {
	var lines []string
	for _, ln := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(strings.TrimRight(ln, "\r"))
		if t == "" ||
			t == "WEBVTT" ||
			strings.Contains(t, "-->") ||
			subCueNum.MatchString(t) ||
			strings.HasPrefix(t, "Kind:") ||
			strings.HasPrefix(t, "Language:") ||
			strings.HasPrefix(t, "NOTE") {
			continue
		}
		t = subTag.ReplaceAllString(t, "")
		t = subBracket.ReplaceAllString(t, "")
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		// Dedupe consecutivo: los auto-captions repiten la línea anterior en el cue siguiente.
		if len(lines) > 0 && lines[len(lines)-1] == t {
			continue
		}
		lines = append(lines, t)
	}
	return strings.TrimSpace(subWS.ReplaceAllString(strings.Join(lines, " "), " "))
}
