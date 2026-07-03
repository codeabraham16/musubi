---
artifact: design
schema_version: "1.0"
change: deteccion-de-cambios-de-codigo
status: draft
---

# Diseño técnico — Detección de cambios de código

## Decisiones de arquitectura
| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Paquete nuevo `internal/codeintel`, aislado, sin dependencia del motor DB | Meterlo en `internal/memory` | El extractor es lógica pura de código; separarlo mantiene `memory` enfocado en persistencia y permite testearlo con corpus sin DB |
| D2 | `.go` vía `go/ast` + `go/parser` en modo tolerante (`parser.SkipObjectResolution`, ignora errores parciales) | tree-sitter, regex para Go | `go/ast` es stdlib (cero deps, Go puro), exacto en rangos, y ya está en el toolchain; regex para Go sería peor y tree-sitter rompe la restricción |
| D3 | `.ts/.tsx/.js/.jsx/.py` vía **set de regex ancladas a línea**, no tokenizer | Mini-tokenizer por lenguaje | Menos código, determinista, y la spec ya manda omitir ante ambigüedad; un tokenizer es sobre-ingeniería para el objetivo (símbolos top-level inequívocos) |
| D4 | El extractor calcula `EndLine` por **profundidad de llaves/indentación** hasta el cierre del bloque | Solo `StartLine` | El solape hunk↔símbolo (R11) necesita rango, no punto; para Go lo da el AST, para el resto se estima por bloque |
| D5 | `detect_changes` lee el **contenido actual** del working tree para extraer símbolos, y usa el **lado nuevo** del hunk | Usar símbolos guardados en `code_memory` | Núcleo de la robustez: ambos lados (diff y símbolos) salen del MISMO estado nuevo → nunca se desalinean (mata modes 1 y 2 del crítico técnico) |
| D6 | `related_memory` se resuelve por **búsqueda keyword** del path y de cada símbolo cambiado sobre observaciones/facts | Recall semántico (embeddings) | Keyword por path exacto + nombre de símbolo es barato, determinista y preciso; el semántico traería ruido y depende de proveedor de embeddings |
| D7 | Handler en archivo nuevo `internal/mcp/methods_detect.go`; git vía `exec` envuelto en un helper testeable | Handler dentro de `methods.go` | `methods.go` ya es grande; aislar facilita el test del parser de diff con fixtures |
| D8 | `save_code` auto-deriva `symbols` con `codeintel` cuando el llamador no lo pasa; si lo pasa, respeta el suyo | Siempre auto, o nunca | Compat hacia atrás (R16) y permite override manual, pero por default arregla el "nadie gistea símbolos frescos" |
| D9 | `detect_changes` es read-only (`readOnly:true`); NO persiste el resultado de staleness (solo lo reporta) | Marcar el gist stale en DB al vuelo | Read-only es más simple, seguro y coherente con el resto de tools de consulta; la frescura ya se computa por fingerprint en el momento |

## Enfoque de implementación

### Componente 1 — `internal/codeintel/`
- `symbols.go`: dispatcher por extensión + implementaciones.
  - `extractGo(content) []Symbol` — `go/parser.ParseFile` tolerante; recorre `ast.FuncDecl`,
    `ast.GenDecl` (type/var/const top-level); `StartLine/EndLine` desde `fset.Position`.
  - `extractBrace(content, lang) []Symbol` — para JS/TS: regex ancladas
    (`^\s*(export\s+)?(async\s+)?function\s+(\w+)`, `^\s*(export\s+)?class\s+(\w+)`,
    `^\s*(export\s+)?const\s+(\w+)\s*=\s*(async\s*)?\(` ) + cierre por conteo de llaves.
  - `extractPy(content) []Symbol` — regex (`^(\s*)def\s+(\w+)`, `^(\s*)class\s+(\w+)`) +
    cierre por des-indentación.
- `imports.go`: `ExtractImports(path, content) []Import` — `go/ast` para Go; regex
  `import ... from` / `require(` / `^import ` para el resto.
- `diff.go`: `ParseUnifiedDiff(out string) []FileDiff` — parsea `diff --git`, `+++/---`,
  y hunks `@@ -a,b +c,d @@` → rangos del lado nuevo; clasifica change_type; salta binarios.
- `git.go`: `Runner` interface (`Diff(ref, staged) (string, error)`) con impl real vía
  `os/exec` (`git --no-pager diff --no-color [--staged] [ref]`) e impl fake para tests.

### Componente 2 — `internal/mcp/methods_detect.go`
`toolDetectChanges(raw)`:
1. Parsear args `{ref?, staged?}`.
2. `runner.Diff(...)` → `ParseUnifiedDiff` → `[]FileDiff`.
3. Por archivo modificado/agregado: leer contenido actual → `codeintel.ExtractSymbols` →
   filtrar los que solapan hunks nuevos → `changed_symbols`.
4. `GetCodeMemory(key)` + `FileFingerprint` → `gist_stale`.
5. Keyword-search de `path` + símbolos → `related_memory` (ids/topic_keys).
6. Ensamblar `DetectReport` compacto (R14) y devolver `jsonResult`.

### Componente 3 — wiring
- `registry.go`: alta de `musubi_detect_changes` (`readOnly:true`); conteo 29→30; golden.
- `methods.go` `toolSaveCode`: si `args.Symbols==""`, derivar vía `codeintel` desde el
  contenido actual antes de armar `CodeMemory` (D8).
- `sdd.go` `SDDPhaseDirective("verify", …)`: agregar "Corré musubi_detect_changes para
  acotar qué símbolos tocaste y qué gists/decisiones quedaron stale antes de verificar."

## Contratos / interfaces
```go
// internal/codeintel
type Symbol struct { Name, Kind string; StartLine, EndLine int } // Kind: func|method|class|type|const|var|def
type Import struct { Path string; Line int }
type FileDiff struct { Path, OldPath, ChangeType string; NewRanges []LineRange } // ChangeType: added|modified|deleted|renamed
type LineRange struct { Start, End int }

func ExtractSymbols(path, content string) []Symbol   // dispatch por extensión; nunca panic; [] si no soportado
func ExtractImports(path, content string) []Import
func ParseUnifiedDiff(gitOut string) []FileDiff
func SymbolsInRanges(syms []Symbol, ranges []LineRange) []Symbol // solape inclusivo

type Runner interface { Diff(ref string, staged bool) (string, error) }

// reporte de la tool
type DetectReport struct {
  Files []FileChange `json:"files"`
  Summary string     `json:"summary"`
}
type FileChange struct {
  Path string            `json:"path"`
  ChangeType string      `json:"change_type"`
  ChangedSymbols []string `json:"changed_symbols"`
  GistStale bool         `json:"gist_stale"`
  RelatedMemory []string `json:"related_memory"`
}
```

## Trade-offs
- **Se gana:** correctitud por diseño (derivar del estado actual), model-free, Go puro, cero
  deps nuevas, integración directa con memoria/SDD (el diferenciador). Testeable con fixtures.
- **Se cede:** el escáner no-Go es aproximado (símbolos top-level, no anidados profundos);
  `EndLine` por llaves/indentación puede errar en código muy irregular → se acota a top-level
  y se degrada, nunca se inventa. No hay impacto transitivo (callers) — es otro track.

## Plan de pruebas
- `codeintel/symbols_test.go`: corpus por lenguaje (Go real con `go/ast`, TS con arrow/class,
  Py con def/class anidada), incluyendo el **caso de corrimiento de líneas** (símbolo movido)
  y el de **archivo que no compila** (degradación sin panic). Table-driven.
- `codeintel/diff_test.go`: fixtures de `git diff` (added/modified/deleted/renamed, binario,
  multi-hunk) → rangos nuevos correctos.
- `codeintel/overlap_test.go`: `SymbolsInRanges` con solapes borde (símbolo justo en el límite).
- `mcp/methods_detect_test.go`: `Runner` fake con diff fijo + archivos temporales →
  report esperado (símbolos, gist_stale, related_memory). Verifica read-only.
- `mcp/methods_test.go`: caso `save_code` sin symbols → se auto-derivan; con symbols → respeta.
- Golden `tools/list` (server_test/http_test/dispatch_concurrent_test): 29→30.
