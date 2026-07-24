---
artifact: proposal
schema_version: "1.0"
change: grafo-codigo-f2b-hook
status: draft
---

# Propuesta — El hook que responde antes de leer (Track 20 · F2-B)

## Intención
La mayor palanca de tokens del Track (patrón Graphify, ~71×): que ANTES de `Read` de un archivo
Go, Musubi inyecte su **estructura** (imports + símbolos con callers/callees) para que el agente
lo navegue **sin leerlo entero**. F2-A dejó el grafo consultable por tools; F2-B lo hace automático
en el hook `PreToolUse` que ya existe (`musubi precheck`, el que hoy inyecta gists/telemetría).

## Alcance
- **Incluye:** una 3ª superficie en `precheck` — `codeGraphMessage` — que arma el contexto de
  estructura de un archivo indexado. **OPT-IN** por env var `MUSUBI_CODEGRAPH_HOOK` (default OFF).
  Contabilizado en el ledger (`precheck_codegraph`).
- **No incluye:** cambiar la config/hooks del usuario (el hook ya está instalado; esto es solo el
  comando Go). Aristas TS/Py (F4). Cualquier cambio al grafo (solo lo consulta).

## Enfoque
Reusa el grafo persistido (F1) vía las lecturas scopeadas de F2-A (`ListGraphNodesForFileCtx`,
`GraphOutEdgesCtx`, `GraphInEdgesCtx`). Model-free (solo recorre). **Doble seguro de no-sorpresa:**
apagado por default (env var), y aun encendido, inerte hasta que exista grafo (`musubi_codegraph_index`).
Salida compacta y acotada (≤10 símbolos, ≤5 refs c/u): tiene que costar MUCHO menos que leer.

## Impacto
- `cmd/musubi/precheck.go`: interfaz `codeStore` +3 métodos de grafo; `codeGraphMessage` + helpers;
  gate por env var. `precheck_test.go`: fake +grafo, 2 tests.
- Compatibilidad: aditivo; sin env var, comportamiento **bit-a-bit** al actual.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Ruido/tokens en cada Read | Opt-in default OFF + inerte sin índice + salida acotada + ledger para medir |
| Muchas queries por archivo grande | Cap de símbolos (10) y refs (5) |

## Criterio de éxito
1. Con `MUSUBI_CODEGRAPH_HOOK=1` y archivo indexado, el hook inyecta estructura (imports + callers/callees).
2. Sin el env var, NO se inyecta nada nuevo (comportamiento actual intacto). Cubierto por test.
3. Model-free, sin cgo, suite verde.

## Rollback
Aditivo. Quitar el bloque gateado / la env var revierte sin tocar datos.
