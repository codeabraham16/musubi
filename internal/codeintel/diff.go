package codeintel

import (
	"regexp"
	"strconv"
	"strings"
)

// Tipos de cambio de un archivo en un diff.
const (
	ChangeAdded    = "added"
	ChangeModified = "modified"
	ChangeDeleted  = "deleted"
	ChangeRenamed  = "renamed"
)

// LineRange es un rango de líneas 1-based inclusivo.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// FileDiff resume el cambio de un archivo: su path (destino), el path viejo si hubo
// rename, el tipo de cambio y los rangos de línea del LADO NUEVO afectados por los hunks.
type FileDiff struct {
	Path       string      `json:"path"`
	OldPath    string      `json:"old_path,omitempty"`
	ChangeType string      `json:"change_type"`
	NewRanges  []LineRange `json:"new_ranges"`
	Binary     bool        `json:"binary,omitempty"`
}

var reHunk = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// ParseUnifiedDiff parsea la salida de `git diff` (formato unified, sin color) y devuelve
// un FileDiff por archivo con los rangos del lado nuevo. Salta archivos binarios (sin
// hunks). Los rangos corresponden al estado NUEVO, que es contra el que se re-derivan los
// símbolos: por eso nunca se desalinean.
func ParseUnifiedDiff(gitOut string) []FileDiff {
	var files []FileDiff
	var cur *FileDiff
	flush := func() {
		if cur != nil {
			files = append(files, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(gitOut, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			cur = &FileDiff{ChangeType: ChangeModified}
			if a, b, ok := parseDiffGitPaths(line); ok {
				cur.OldPath, cur.Path = a, b
			}
		case cur == nil:
			// preámbulo antes del primer archivo; ignorar.
			continue
		case strings.HasPrefix(line, "new file mode"):
			cur.ChangeType = ChangeAdded
		case strings.HasPrefix(line, "deleted file mode"):
			cur.ChangeType = ChangeDeleted
		case strings.HasPrefix(line, "rename from "):
			cur.OldPath = strings.TrimPrefix(line, "rename from ")
			cur.ChangeType = ChangeRenamed
		case strings.HasPrefix(line, "rename to "):
			cur.Path = strings.TrimPrefix(line, "rename to ")
			cur.ChangeType = ChangeRenamed
		case strings.HasPrefix(line, "Binary files "):
			cur.Binary = true
		case strings.HasPrefix(line, "+++ b/"):
			cur.Path = strings.TrimPrefix(line, "+++ b/")
		case strings.HasPrefix(line, "--- a/"):
			if cur.OldPath == "" {
				cur.OldPath = strings.TrimPrefix(line, "--- a/")
			}
		case strings.HasPrefix(line, "@@"):
			if r, ok := parseHunkNewRange(line); ok {
				cur.NewRanges = append(cur.NewRanges, r)
			}
		}
	}
	flush()
	return files
}

// parseHunkNewRange extrae el rango del lado nuevo de un encabezado de hunk. Un conteo de
// 0 líneas (borrado puro) no aporta rango nuevo y se descarta.
func parseHunkNewRange(header string) (LineRange, bool) {
	m := reHunk.FindStringSubmatch(header)
	if m == nil {
		return LineRange{}, false
	}
	start, _ := strconv.Atoi(m[1])
	count := 1
	if m[2] != "" {
		count, _ = strconv.Atoi(m[2])
	}
	if count <= 0 {
		return LineRange{}, false
	}
	return LineRange{Start: start, End: start + count - 1}, true
}

// parseDiffGitPaths extrae "a/x" y "b/y" de una línea `diff --git a/x b/y`.
func parseDiffGitPaths(line string) (old, neu string, ok bool) {
	rest := strings.TrimPrefix(line, "diff --git ")
	// Camino común sin espacios en los paths: "a/x b/y".
	if i := strings.Index(rest, " b/"); i > 0 && strings.HasPrefix(rest, "a/") {
		return rest[2:i], rest[i+3:], true
	}
	return "", "", false
}

// SymbolsInRanges devuelve los símbolos cuyo rango [StartLine, EndLine] solapa alguno de
// los rangos dados (solape inclusivo). Es el cruce hunk↔símbolo del detector.
func SymbolsInRanges(syms []Symbol, ranges []LineRange) []Symbol {
	var out []Symbol
	for _, s := range syms {
		for _, r := range ranges {
			if s.StartLine <= r.End && s.EndLine >= r.Start {
				out = append(out, s)
				break
			}
		}
	}
	return out
}
