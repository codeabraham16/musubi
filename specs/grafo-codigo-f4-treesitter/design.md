---
artifact: design
schema_version: "1.0"
change: grafo-codigo-f4-treesitter
status: draft
---

# Diseño técnico — Aristas TS/JS/Python vía tree-sitter (Track 20 · F4)

## Decisiones
| # | Decisión | Alternativas | Por qué |
|---|----------|--------------|---------|
| D1 | **`gotreesitter`** (runtime tree-sitter 100% Go) | sidecar node/rust; wazero+WASM (malivvan) | Único que respeta sin-cgo + un-binario SIN proceso externo ni WASM bridge. Maduro (v0.47.0, MIT), trae extractores language-neutral. malivvan es pre-release; el sidecar rompe el binario único y choca con el VPN del usuario (node excluido). |
| D2 | **Opt-in por build tag `treesitter`** (2 archivos: on/off) | Siempre-on | El default no linkea tree-sitter → binario intacto/lean; TS/Py-edges es un build opt-in (como F2-B). |
| D3 | **`grammar_subset_*`** para acotar gramáticas | Embeber las ~206 (24MB) | Baja el binario a pocos MB embebiendo solo TS/TSX/JS/Python. |
| D4 | **CALLS intra-archivo**: enclosingDef(byte-range) = caller; callee = def homónimo único | go/types-equivalente cross-archivo | Barato y exacto intra-archivo; cross-archivo diferido (honesto, como Go en F1). |
| D5 | Reusar extractores `ExtractDefinitionSpans`/`ExtractCalls`/`ExtractImportsFromSource` | Escribir queries tree-sitter a mano | La lib ya los da language-neutral; menos código, más robusto. |

## Implementación
- `treesit_on.go` (`//go:build treesitter`): `languageFor` (ext→gramática), `derivePolyglotFile`
  (parse → defs+calls+imports → Node/Edge), `enclosingDef` (byte-range), `lineAtByte`, `tsKind`.
- `treesit_off.go` (`//go:build !treesitter`): `polyglotSupported`/`derivePolyglotFile` no-op.
- `graph.go` `DerivePackage`: pase polyglot al final, mergea por los mismos `addNode`/`addEdge` (dedup).
- Nodo destino de IMPORTS: `PackageKey(path)`, `External = Relative==0` (bare = externo).

## Trade-offs
- Gana: TS/Py con aristas reales, sin cgo, sin sidecar, binario por default intacto.
- Cede: build opt-in (dos configuraciones); imports TS parciales; cross-archivo diferido; `gotreesitter`
  como dep tag-gated (cuidado con `go mod tidy`).

## Plan de pruebas
- `treesit_test.go` (`//go:build treesitter`): TS+Py → símbolos+CONTAINS+CALLS+IMPORTS.
- `treesit_off_test.go` (`//go:build !treesitter`): TS/Py → 0 nodos/aristas (default).
- `graph_test.go`: extensiones no soportadas (.css/.md) → vacío en ambos builds.
