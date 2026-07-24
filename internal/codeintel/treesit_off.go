//go:build !treesitter

package codeintel

// treesit_off.go es la implementación POR DEFAULT (sin el build tag `treesitter`): los lenguajes
// no-Go quedan solo-símbolos (comportamiento histórico), y el binario NO linkea gotreesitter — se
// mantiene lean y model-free/Go-puro. Compilar con `-tags treesitter` (+ los grammar_subset_*)
// reemplaza esto por la derivación real vía tree-sitter (ver treesit_on.go).

func polyglotSupported(string) bool { return false }

func derivePolyglotFile(string, string) ([]Node, []Edge) { return nil, nil }
