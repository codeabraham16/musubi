---
artifact: proposal
schema_version: "1.0"
change: deteccion-de-cambios-de-codigo
status: draft
---

# Propuesta — Detección de cambios de código (inteligencia de impacto model-free)

## Intención
Hoy las fases SDD `implement`/`verify` no saben QUÉ cambió: se apoyan en recall por
clave y en gists escritos a mano que se desactualizan en silencio. Queremos que Musubi
responda, de forma confiable y model-free, "este diff tocó estos símbolos, dejó estos
gists stale y afecta estas decisiones/SDD" — convirtiendo la memoria de decisiones en
algo **reactivo al código**, sin volverse un indexador de code-graph inferior.

## Alcance
- **Incluye:**
  - Un extractor de estructura en **Go puro** (`internal/codeintel`): símbolos e imports
    por archivo, derivados del **estado actual** del archivo (nunca de datos guardados).
    `.go` vía `go/ast` (stdlib, exacto). `.ts/.tsx/.js/.jsx/.py` vía escáner liviano
    determinista. Otras extensiones → granularidad de archivo.
  - Una tool `musubi_detect_changes`: corre `git diff`, parsea los hunks del lado nuevo,
    re-deriva símbolos del archivo actual, y reporta símbolos cambiados **exactos** +
    cruce con `code_memory.fingerprint` (gists stale) + cruce con memoria
    (observaciones/decisiones/SDD que referencian esos archivos/símbolos).
  - Mejora de `musubi_save_code`: auto-derivar el campo `symbols` con el extractor en
    vez del string manual, anclado al mismo fingerprint (símbolos y hash del mismo snapshot).
  - Enganche en la directiva de la fase SDD `verify`.
- **No incluye (explícito):**
  - **Aristas de código emitidas por el AGENTE** (`save_fact` a mano). Rechazado 4/4 por
    los críticos: se pudren sin invalidación. Si se hacen aristas, se **derivan**, no se emiten.
  - **Mapeo símbolo por línea guardada a mano** (el que se desincroniza). Se re-deriva.
  - **tree-sitter / cgo / paridad de 158 lenguajes**. Rompe Go-puro y es el terreno de
    DeusData; no se compite ahí.
  - Grafo de llamadas cross-lenguaje completo. Las aristas derivadas (IMPORTS, y CALLS
    solo donde `go/ast` lo da gratis) quedan para una fase 2 acotada y honesta, fuera de
    este cambio salvo que el design lo habilite.

## Enfoque
Principio rector: **derivar, no guardar-y-desfasar**. Todo dato estructural se re-deriva
del archivo actual en el momento de la consulta. Así el `git diff` (coordenadas del estado
nuevo) y los símbolos (extraídos de ese mismo estado nuevo) siempre alinean — se elimina
por diseño la contradicción de coordenadas viejas vs. nuevas que hundía la versión ingenua.
El extractor es una capa nueva y aislada; `detect_changes` la compone con la infra que ya
existe (fingerprint, recall, git). Model-free de punta a punta: git y el parser son
deterministas; ningún LLM en el camino.

## Impacto
- Áreas/archivos afectados:
  - Nuevo paquete `internal/codeintel/` (extractor + parser de diff).
  - `internal/mcp/methods.go` / nuevo `methods_detect.go` (handler de la tool).
  - `internal/mcp/registry.go` (registrar `musubi_detect_changes`; +1 tool → 30).
  - `internal/memory/codemem.go` (save_code auto-deriva symbols).
  - `internal/memory/sdd.go` (directiva de `verify` referencia detect_changes).
  - Tests nuevos por componente + golden de tools/list.
- Compatibilidad: aditivo. `save_code` sigue aceptando symbols manual si se pasa (fallback);
  la auto-derivación es el default. Sube el conteo de tools (actualizar snapshots/tests).

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Los escáneres regex de JS/TS/Py dan falsos símbolos | Acotar a construcciones inequívocas (func/class/def/export const arrow); test de corpus; degradar a nivel-archivo ante duda, nunca inventar |
| `go/ast` falla en archivos que no compilan (mitad de edición) | Parseo tolerante (parser.ParseFile con modo recovery); si falla, degradar a nivel-archivo |
| Parsear `git diff` a mano es frágil | Usar formato unified estable + `--no-color`; parser cubierto con tests de hunks reales |
| Scope creep hacia code-graph (lo que criticaron los agentes) | Línea roja dura: aristas SOLO derivadas y fuera de este cambio; foco en "qué decisión queda obsoleta", no en "qué llama a qué" |
| Costo de tokens del reporte | Reporte compacto y estructurado; símbolos por nombre, no cuerpos |

## Criterio de éxito
1. `musubi_detect_changes` sobre un diff real reporta los símbolos cambiados **correctos**
   incluso tras corrimiento de líneas (donde la versión ingenua fallaba) — cubierto por test.
2. Marca correctamente qué gists quedaron stale por fingerprint.
3. Cruza con memoria y lista las decisiones/SDD afectadas.
4. `save_code` guarda símbolos derivados del estado actual, no del string manual.
5. Todo model-free, Go puro (sin nuevas deps con cgo), build + tests verdes.
6. La directiva de `verify` usa la tool.
