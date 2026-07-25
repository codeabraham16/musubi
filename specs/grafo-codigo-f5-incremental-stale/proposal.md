---
artifact: proposal
schema_version: "1.0"
change: grafo-codigo-f5-incremental-stale
status: archived
---

# Propuesta — F5: mantener el grafo de código honesto y barato (incremental + poda de fantasmas)

## Intención
El grafo de código (Track 20) responde impacto/callers/panorama sin leer archivos, pero
hoy sólo se refresca con un índice **FULL** (WalkDir de todo el repo + re-derivar cada
paquete). Eso tiene dos consecuencias: (1) re-indexar es caro, así que se hace poco y el
grafo envejece; (2) un archivo **borrado o renombrado deja nodos/aristas fantasma que ni
siquiera se marcan stale** — `cgStale` devuelve `false` cuando el archivo no existe, así que
código eliminado se ve *fresco* y `impact`/`callers` pueden apuntar a símbolos que ya no
existen. F5 cierra el track haciendo barato mantener el grafo alineado con el working tree.

## Alcance
- **Incluye:**
  - **F5.1 · Índice incremental.** Una variante de `codegraph_index` que reconcilia el grafo
    con el working tree comparando el `src_fingerprint` ya guardado por archivo contra el
    actual: refresca sólo los paquetes con archivos **modificados** (fp distinto) o **nuevos**
    (path no presente en el grafo), y salta los idénticos. El índice FULL sigue existiendo como
    default/fallback.
  - **F5.2 · Poda de fantasmas.** Borrar nodos/aristas de archivos que ya **no existen** en
    disco (borrados/renombrados), scopeado por proyecto. Incluye corregir el bug de `cgStale`
    para que un archivo faltante cuente como stale, no como fresco.
  - **F5.3 · Visibilidad de staleness.** `map` y el resumen del índice reportan cuántos nodos
    **stale** y **fantasma** hay, para que el agente sepa cuándo re-indexar (hoy el stale sólo
    asoma por-nodo si justo consultás ese símbolo).
- **No incluye:**
  - Marcador de consistencia a nivel commit-SHA / versión global del grafo: los fingerprints
    por-archivo ya cubren el caso práctico; se difiere (deuda documentada).
  - Refresco automático dentro del hook precheck / en cada lectura (metería latencia sorpresa);
    F5 es tool-driven. Un auto-refresh opcional queda como posible bonus posterior.
  - Aristas nuevas, CALLS cross-paquete o multi-lenguaje (fueron F1/F4).
  - Detección de renames a nivel de nodo: un rename se trata como borrar el viejo + agregar el
    nuevo (la reconciliación por fingerprint ya lo cubre).

## Enfoque
Reusar lo que F1 ya dejó montado en vez de agregar maquinaria:
- El motor ya persiste `src_fingerprint` por nodo y arista, y `UpsertPackageGraphFrom` ya hace
  **delete-by-source + reinsert** por archivo. La reconciliación no necesita un ref de git ni un
  cursor de "último SHA indexado": el **propio grafo guardado es el estado anterior**. Se listan
  los paths del grafo, se comparan fingerprints con el disco (mismo `FileFingerprint` que usa
  `cgStale`), y se clasifican en sin-cambio / modificado / fantasma; los paquetes con `.go`
  nuevos se descubren con un WalkDir liviano. Sólo los paquetes tocados se re-derivan.
- La poda es una primitiva nueva y chica en `internal/memory/codegraph.go` (borrar filas por
  `project_id`+`path` que ya no están en disco), simétrica al delete-by-source existente.
- `GitRunner.Diff` (ya existe) queda como alternativa evaluada pero **no elegida**: el diff por
  fingerprint es más puro (no depende de un ref base ni de estar en un repo git) y aprovecha
  datos que ya tenemos.

## Impacto
- Áreas/archivos afectados:
  - `internal/mcp/methods_codegraph.go` — modo incremental en el índice, cómputo de la poda,
    contadores stale/fantasma; fix de `cgStale` para archivo faltante.
  - `internal/memory/codegraph.go` — primitivas: listar paths distintos del grafo (scopeado),
    borrar filas por path (poda), contar nodos stale/huérfanos.
  - Tests: `internal/mcp/methods_codegraph_test.go`, `internal/memory/codegraph_test.go`.
- Compatibilidad: **aditivo**. El índice FULL sigue siendo el comportamiento por defecto; el
  incremental es opt-in por argumento. Sin migración de esquema (se computa sobre columnas
  existentes). El shape de las respuestas gana campos, no rompe los actuales.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Reconciliación O(archivos del grafo) en llamadas `stat` | Es mucho más barato que re-derivar/parsear; sólo se parsea lo cambiado. Best-effort, sin abortar por un archivo ilegible. |
| Poda mal scopeada borra filas de otro tenant | Reusar el borrado scopeado por `project_id` ya existente; nunca borrar sin atribución de credencial. |
| Falso "nuevo/fantasma" por normalización de paths inconsistente | Reusar exactamente `NormalizeCodePath`/`FileFingerprint` que usa el resto de codegraph. |
| Incremental deja el grafo desalineado si algo se escapa | El índice FULL sigue disponible como reconciliación total garantizada; el incremental nunca es la única vía. |

## Criterio de éxito
Tras modificar, borrar y agregar archivos `.go` y correr el índice **incremental**: el grafo
refleja el working tree — sin nodos fantasma, sin stale silencioso — tocando **sólo** los
paquetes afectados. Un archivo borrado deja de aparecer en `impact`/`code_graph`. En un repo sin
cambios, el incremental re-deriva **0 paquetes**. `map` reporta el conteo stale/fantasma.
`go build ./... && go test ./...` en verde.
