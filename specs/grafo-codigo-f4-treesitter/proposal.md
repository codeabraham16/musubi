---
artifact: proposal
schema_version: "1.0"
change: grafo-codigo-f4-treesitter
status: draft
---

# Propuesta — Aristas TS/JS/Python vía tree-sitter (Track 20 · F4)

## Intención
Cerrar el multi-lenguaje del grafo: hoy solo Go tiene aristas (F1); TS/JS/Py quedan solo-símbolos
(regex, sin CALLS). F4 les da símbolos + imports + CALLS reales usando tree-sitter — **sin romper
el invariante sin-cgo / un-binario**.

## Alcance
- **Incluye:** derivación de `Node`/`Edge` para `.ts/.tsx/.js/.jsx/.py` con **`gotreesitter`**
  (runtime tree-sitter 100% Go, sin CGo), detrás del **build tag `treesitter`** (+ `grammar_subset_*`).
  Símbolos (CONTAINS), imports (IMPORTS), CALLS **intra-archivo**.
- **No incluye:** cambiar el binario por default (queda lean, sin tree-sitter linkeado); CALLS
  cross-archivo TS/Py (diferido, como el cross-paquete de Go en F1); resolución de módulos.

## Enfoque
Opt-in a nivel de build (mismo espíritu que F2-B): `treesit_off.go` (default, no-op, no linkea
gotreesitter → binario intacto) vs `treesit_on.go` (`//go:build treesitter`, deriva de verdad).
`DerivePackage` dispatcha por extensión. Los `grammar_subset_*` acotan las gramáticas embebidas
(pocos MB, no las ~206). Reusa los extractores language-neutral de gotreesitter
(`ExtractDefinitionSpans`/`ExtractCalls`/`ExtractImportsFromSource`). Model-free, sin LLM.

## Impacto
- `internal/codeintel/`: `treesit_on.go` (gated) + `treesit_off.go` + pase polyglot en `DerivePackage`.
- `go.mod`: +`gotreesitter` (tag-gated, `// indirect`; no `go mod tidy` sin el tag).
- Compatibilidad: **aditiva**; build por default bit-a-bit al actual (verificado: gotreesitter no linkeado).

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Bloat del binario (24MB, 206 gramáticas) | Build tag opt-in + `grammar_subset_*` (pocos MB); default no linkea nada |
| `go mod tidy` borra la dep tag-gated | Documentado; tidY con `-tags treesitter` |
| Imports TS no siempre extraen (ExtractImportsFromSource dio 0 en un caso) | Aceptable en F4; símbolos+calls (el núcleo) andan; imports Py sí andan |
| CALLS cross-archivo TS/Py | Diferido honesto (intra-archivo con match único), como Go en F1 |

## Criterio de éxito
1. Con `-tags treesitter`, un `.ts` y un `.py` producen símbolos + CONTAINS + CALLS intra-archivo.
2. **Sin** el tag, TS/Py no emiten grafo (default intacto) y gotreesitter **no** se linkea.
3. Model-free, sin cgo, ambos builds verdes.

## Rollback
Aditivo + gated. Quitar los archivos + la dep revierte; el default nunca cambió.
