//go:build !treesitter

package codeintel

// treesit_off.go es la implementación POR DEFAULT (sin el build tag `treesitter`): los lenguajes
// no-Go quedan solo-símbolos (comportamiento histórico), y el binario NO linkea gotreesitter — se
// mantiene lean y model-free/Go-puro. Compilar con `-tags treesitter` (+ los grammar_subset_*)
// reemplaza esto por la derivación real vía tree-sitter (ver treesit_on.go).

func polyglotSupported(string) bool { return false }

func derivePolyglotFile(string, string) ([]Node, []Edge) { return nil, nil }

// PolyglotHabilitado dice si ESTE binario linkeó tree-sitter.
//
// Existe porque el dato no se puede deducir de afuera y su ausencia se ve igual que un repo sin
// código: dos binarios idénticos a la vista indexan cantidades muy distintas, y `--version`
// devolvía lo mismo en los dos. Medido: el binario del árbol no tenía el tag, y `construir.sh`
// tampoco lo pone — sólo lo pone el workflow de release. Que el nivel VIAJE (a version, al doctor
// y al hint del grafo) es lo que convierte «el grafo salió vacío» en una respuesta.
func PolyglotHabilitado() bool { return false }
