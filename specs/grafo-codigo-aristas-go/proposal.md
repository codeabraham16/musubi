---
artifact: proposal
schema_version: "1.0"
change: grafo-codigo-aristas-go
status: draft
---

# Propuesta — Grafo de código: aristas derivadas en Go (Track 20 · F1)

## Intención
Es la **fase 2 acotada** que `deteccion-de-cambios-de-codigo` dejó explícitamente
diferida ("aristas derivadas (IMPORTS, y CALLS solo donde `go/ast` lo da gratis) quedan
para una fase 2 acotada y honesta"). Hoy `codeintel` extrae **símbolos por archivo pero
sin aristas**: el agente re-lee o Grepea para responder "quién llama a esto", "qué importa
a qué" y "qué se rompe si cambio X". Queremos un **grafo de código DERIVADO** (jamás escrito
a mano), model-free, per-proyecto y *federation-ready*, que sea la mitad estructural que a
Musubi le falta para cerrar el híbrido **AST ⊕ memoria** — y el cimiento de las consultas de
navegación/impacto (F2) y del enlace código↔memoria (F3).

## Alcance
- **Incluye:**
  - Extender `internal/codeintel` para **emitir aristas derivadas del AST de Go**:
    - `IMPORTS` (exacto, de los import specs) — confianza 1.0.
    - `CONTAINS` (paquete→archivo→símbolo, exacto) — confianza 1.0.
    - `CALLS` (derivadas; estrategia de resolución exacta vs. sintáctica = **decisión de
      design**, ver riesgos) — confianza por arista.
  - Un **modelo de grafo**: nodo = símbolo con **id estable** (`project_id` + `path#símbolo`);
    arista **tipada** con confianza y proveniencia (`EXTRACTED`=derivado del código).
  - **Persistencia scopeada por `project_id`** (tablas nuevas `code_graph_nodes` /
    `code_graph_edges`), con el **mismo patrón de tenancy que `code_memory`**, e
    **invalidación por fingerprint**: un nodo/arista se marca *stale* cuando el fingerprint
    del archivo origen ya no coincide. Nunca se hand-mantiene.
  - **Camino de poblado/refresco** que deriva del **estado ACTUAL** del archivo (mismo
    principio "derivar, no guardar-y-desfasar"), reusando `fingerprint` + `detect_changes`
    para refrescar solo lo que cambió.
- **No incluye (explícito):**
  - Tools públicas de consulta (`musubi_code_graph` / `impact` / `map`) y el hook
    `PreToolUse` que responde antes de Grep/Read → **F2**.
  - Arista `símbolo —[EXPLICADO_POR]→ decisión/gotcha` → **F3**.
  - Aristas para `.ts/.tsx/.js/.jsx/.py` (siguen **solo-símbolos**; sus aristas necesitan un
    parser real) → **F4** (tree-sitter, diferido).
  - El **sync real** al cerebro central y la **visualización en la cabina CRM** → fase
    posterior. F1 solo deja el storage *federation-ready* (`project_id` desde el día 1).
  - **Aristas EMITIDAS por el agente** (`save_fact` a mano): **NO, nunca**. Se derivan.
    (Rechazado 4/4 por los críticos en detección-de-cambios; mantenemos la línea roja.)
  - **tree-sitter / cgo / paridad de N lenguajes**: rompe Go-puro; no competimos ahí.

## Enfoque
Mismo principio rector que `codeintel`: **derivar, no guardar-y-desfasar**. Las aristas se
computan del AST del estado actual. Se **persisten** por dos razones concretas: (1) **federar**
— el server central no tiene el código fuente del proyecto, así que un grafo puramente
derivado-al-vuelo *no podría verse desde la cabina CRM*; (2) servir consultas baratas sin
re-parsear. Pero cada fila lleva el **fingerprint** de su archivo origen, de modo que una
desincronización se **detecta** (stale) en lugar de mentir. Reconciliación de las tres
fuerzas: **derivado** (no a mano) + **persistido** (federable) + **guardado por fingerprint**
(no se pudre). Model-free de punta a punta: `go/ast` (y posiblemente `go/types`, ambos stdlib),
git, sin LLM en el camino.

## Impacto
- Áreas/archivos afectados:
  - `internal/codeintel/`: nuevo emisor de aristas (p. ej. `edges.go`) + posible resolución
    de llamadas (`go/types`), reusando el `ExtractSymbols` existente.
  - `internal/memory/`: nuevo store `code_graph` (nodes/edges) scopeado por `project_id`,
    con DDL bootstrap (como `code_memory`).
  - `internal/mcp/`: en F1, solo un método **interno** de poblado/refresh; las tools
    públicas son F2.
  - Tests nuevos por componente + golden sobre el propio repo Go.
- Compatibilidad: **aditivo** (paquete + tablas nuevas). No toca `code_memory` ni
  `detect_changes`. Grafo vacío ⇒ todo sigue igual.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Resolución de `CALLS` imprecisa (métodos, interfaces, shadowing, cross-package) | Empezar por lo **exacto** (`IMPORTS`/`CONTAINS` + `CALLS` type-resueltos con `go/types` donde carga); marcar lo puramente sintáctico con confianza < 1.0; ante duda, omitir, nunca inventar |
| `go/types` necesita cargar el paquete (build) y puede fallar en repos a mitad de edición | Degradar a AST-sintáctico o a nivel-archivo; parseo tolerante; **nunca panic** |
| El grafo persistido se desincroniza del código (el pecado que criticaron los agentes) | **Invalidación por fingerprint obligatoria**: fila con fingerprint viejo se reporta *stale*, no como verdad; refresco incremental por `detect_changes` |
| Federación: el central no tiene el fuente | Por eso se persiste el grafo **derivado** (no se re-deriva en el central); `project_id` desde el día 1 para no migrar después |
| Scope creep hacia F2/F3 | Líneas rojas duras: F1 entrega **solo** el grafo derivado + storage; sin tools públicas, sin hook, sin weld a memoria |
| Tamaño del grafo en repos grandes | Aristas por **id de símbolo** (no cuerpos); índices por `project_id`+`path`; refresco incremental por fingerprint |

## Criterio de éxito
1. Sobre el **propio repo de Musubi** (Go), `codeintel` emite `IMPORTS` y `CONTAINS`
   **exactos** y `CALLS` derivados con confianza — cubierto por tests golden.
2. El store persiste nodos/aristas scopeados por `project_id` y los recupera; dos proyectos
   con el mismo `path` **no se pisan**.
3. Cambiar un archivo y refrescar deja *stale* (por fingerprint) **solo** lo afectado, y lo
   re-deriva correcto incluso tras **corrimiento de líneas**.
4. Un nodo/arista cuyo fingerprint no coincide con el archivo actual se reporta **STALE**
   (no como verdad) — test explícito.
5. Todo **model-free**, **Go puro** (sin cgo), `go build` + `go test` verdes.
6. El storage queda **federation-ready** (`project_id`) **sin activar aún** el sync (fase
   posterior).

## Estrategia de rollback
Aditivo por diseño (paquete + tablas nuevas, sin tools públicas). Rollback = no registrar el
poblado / *drop* de las tablas `code_graph_*`. `code_memory`, `detect_changes` y el resto del
motor quedan intactos.
