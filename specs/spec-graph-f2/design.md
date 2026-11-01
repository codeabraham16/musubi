---
artifact: design
schema_version: "1.0"
change: spec-graph-f2
status: draft
---

# Diseño técnico — spec-graph · F2 (anclaje + rancedumbre)

> Resuelve las 5 preguntas abiertas del `spec.md`. La viga —dónde vive la huella de base— la decidió el
> dueño: **en el propio `SPEC.md`, co-viajando por git** (consulta del 2026-11-01). Las otras 4 se deciden
> acá con su rationale. El principio rector es el de la propuesta: **reusar, no reinventar** — el ancla de
> un spec y el ancla de una observación sobre la misma ruta deben ser **comparables**, así que comparten
> la misma huella, no dos mecanismos paralelos.

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| **D1** | **La huella de base vive en el frontmatter del `SPEC.md`** (co-viaja por git). | (b) tabla `spec_anchors` en `memory.db`; (c) deducirla de git (comparar contra el último commit que tocó el spec). | El spec vive en git: la rancedumbre pasa a ser **verdad del artefacto**, visible en cualquier checkout/máquina. Sin DB ni migración (R10). Preserva la identidad de spec-graph: índice derivado del FS, sin estado paralelo (F1/R10). (b) deja la base **ciega en un checkout nuevo** y hereda el peor filo del gotcha de cambio de rama [daa576e9]. (c) no ensucia el frontmatter pero ata todo a git y tiene bordes raros (spec sin commitear, ramas divergentes). **Elección del dueño.** |
| **D2** | La huella se guarda **inline en cada entrada de `anchors`**, forma `ruta#símbolo=<digest>` (se parte por el **último `=`**). Tool-managed. Una entrada **sin `=`** (p. ej. escrita a mano) es válida: da `missing` ya, y `changed` se activa al primer guardado por tool. | Campo paralelo `anchor-baseline` alineado por índice (frágil: se desalinea al reordenar). Mapa anidado `{ref: digest}` (el parser de frontmatter **descarta** los objetos anidados — `parse.go`). | Los valores del frontmatter son `string \| []string`, no hay mapas. `=` **no aparece** en rutas ni en símbolos Go/TS/Py (sí `/ . - _ #`), así que parte sin ambigüedad y `SplitOriginRef` reusa la mitad izquierda tal cual. La entrada es un string inerte: round-trip lossless como cualquier otra (R11). |
| **D3** | **Factorizar las primitivas puras a `internal/codeintel`**: `SymbolFingerprint`, `FileFingerprint`, `SymbolMatches`, `SplitOriginRef`, `NormalizeCodePath`. `internal/memory` y `internal/specgraph` las comparten; `memory` conserva **alias finos** (`symbolFingerprint = codeintel.SymbolFingerprint`, …) para no tocar su suite #326. | Duplicar el fingerprint + match en `specgraph`. Usar `codeintel.ExtractSymbols` directo y reimplementar el match. | Las dos alternativas **bifurcan la regla calificada de #326** (`Tipo.Metodo` no funde homónimos) — justo lo que go-hygiene y musubi-rules prohíben. Una regla, un lugar. Hace **comparable** el ancla de spec con la de observación (son el mismo byte-hash sobre las mismas líneas), que es el enfoque declarado de la propuesta. `codeintel` ya es el dueño de `Symbol`/`ExtractSymbols` y ambos paquetes ya lo importan: el hogar natural. |
| **D4** | **Derivado-only**: spec-graph usa sólo el extractor (`fuenteDerivada`), **sin** el fallback declarado ni el `prefijoDeclarado` de observaciones. | Portar también el fallback declarado (symbols de `code_memory`). | spec-graph no tiene `code_memory` ni DB. Anclar a un símbolo en un lenguaje **sin extractor** debe ser **ERROR al guardar** (R2), que es exactamente lo que da el camino derivado (`errSymbolNotFound`). Menos superficie; sin el riesgo de rancio-masivo-por-cambio-de-build que el prefijo mitiga (ese riesgo nace **del fallback**, que acá no existe). El prefijo queda como vía futura si F-posterior quisiera declarado. |
| **D5** | La marca de rancedumbre **la cargan `get` y `validate`**. `grep` y `graph` **no**. | Que las 4 read-only la carguen. | Re-derivar lee archivos **de código** del disco (≤10 por spec). El costo se justifica en un `get` de un solo nodo y en `validate` (cuyo trabajo *es* reportar problemas), no en `grep` (paginado, output de snippets de línea sin slot por-nodo, ya acotado por bytes) ni en `graph` (recorte masivo: ≤10 lecturas × N nodos sería un pico). F3 puede sumar overlay de cobertura cuando lo pida. |
| **D6** | **Cómputo fresco por llamada**, acotado a ≤10 anclas. **NO** se cachea en el `(mtime,size)` del spec. | Memoizar la huella vigente por `(anchored-path, mtime, size)`. | El archivo **anclado** es otro archivo que el `SPEC.md`; el cache del índice (F1/R10) keya los bytes del spec, no los del código. Fresco es ≤10 lecturas chicas + extract: barato y **siempre correcto** (sin bug de invalidación). La memoización es optimización futura si un repo con miles de specs la pidiera. |
| **D7** | La huella se **(re)captura sólo al escribir `anchors`**: en `create` con `anchors`, y en `update` que **toca** `anchors`. Un `update` ajeno (p. ej. `set status`) **no** re-vouchea. Dedup **por ref** (no por string); el digest recién calculado gana. | Recapturar en cada `update`. | Un `update status` que recalculara digests **borraría en silencio** un `changed` legítimo — el autor estaría re-dando-fe del código sin querer, y la señal moriría. Dedup por string rompería porque `ruta#S` (a mano) y `ruta#S=hash` (tool) se verían distintos. El editor lossless gana una vía **anchor-aware** (ver Contratos). |
| **D8** | `valid` sigue siendo **integridad estructural** (dangling / duplicate / cycles / unknown-type / rejected). La rancedumbre se reporta en una **sección nueva** del output de `validate` **sin voltear `valid`**. | Que un ancla `missing` voltee `valid=false` (como un enlace colgante). | R5 es **surface-only**: la marca no cambia estado. Conflatear «grafo roto» con «prosa vencida» haría que un `validate` gateado en CI llorara por deriva de código. `missing` de un **ancla** es deriva de código, no un defecto del grafo de specs. F3+ puede dejar que un proyecto opte por tratar `missing` como fallo. |
| **D9** | **R9 se sostiene gratis en F2**: `specgraph` no escribe ningún payload de sync/outbox. git **no es** «el central». F4 **hereda** la obligación de excluir el digest del gist de spec federado. | — | «El central» es el cerebro de memoria por outbox (cross-proyecto), no el repo git (que es justo donde **queremos** que la base co-viaje entre máquinas del mismo proyecto — el punto de D1). F2 no federa specs, así que ningún digest sale por sync. El test de R9 es negativo: el subsistema no tiene escritor de outbox. |

## Enfoque de implementación

**Paquetes tocados.** Tres, en capas, sin romper la frontera D1 de F1 (core fiel · política en tools):

1. **`internal/codeintel`** (nuevo archivo `fingerprint.go`): recibe por **move + export** las primitivas puras
   que hoy viven en `internal/memory/origins.go`. Es un refactor mecánico; `go build ./...` y la suite #326
   de `memory` (que se queda donde está, llamando a los alias finos) lo guardan.
2. **`internal/specgraph`** (nuevo archivo `anchors.go`): el vocabulario (`FieldAnchors="anchors"` sumado al
   const block, a `ListFields` y a `FieldOrder` de `parse.go`), el parseo de una entrada de ancla a
   `(ref, baseline)`, y el cómputo de rancedumbre `StalenessOf(root, fm)` que espeja `markStaleOrigins`:
   re-deriva, compara, devuelve `[]StaleAnchor` **sin reordenar ni descartar**, tragándose los errores de
   E/S ajenos al contenido (R6).
3. **`internal/mcp/methods_spec.go`**: `create`/`update` validan y fingerprint-ean `anchors` (vía
   `specgraph.BuildAnchorEntries`); `get`/`validate` adjuntan la marca a sus DTOs. El golden de `tools/list`
   se regenera (R12) porque `create`/`update` ganan el parámetro `anchors`; la **clasificación no cambia**
   (`create`/`update` siguen write; `get`/`validate` siguen read-only).

**Flujo de escritura (`create`/`update`).** El handler toma los `anchors` crudos de los args →
`specgraph.BuildAnchorEntries(root, refs)` normaliza (reusa `NormalizeCodePath`), valida existencia de la
ruta y resolubilidad del símbolo, y **calcula el digest** de cada uno; si algo no resuelve, devuelve el
error de R2 (nombra el símbolo ausente y ofrece anclar al archivo entero). Las entradas enriquecidas
`ruta#símbolo=digest` entran al frontmatter: en `create`, como `fm[FieldAnchors]`; en `update`, el handler
resuelve el **conjunto final** (existentes ∪ nuevas, keyed por ref, digest nuevo gana) y lo escribe
preservando lossless todo lo demás. `anchors` **no** usa el `setListField`/`AddList` genérico: tiene su
propia vía anchor-aware (D7).

**Flujo de lectura (`get`/`validate`).** Tras construir el nodo/grafo (como F1), se llama
`specgraph.StalenessOf(root, fm)` por spec con anclas. El resultado se mapea a un DTO tagueado y se adjunta;
en `get` además se antepone al campo que el agente lee una advertencia legible tipo `StaleWarning`
(estructura para el dashboard/tests, prosa para el modelo — el mismo doble canal de origins.go).

**Degradación con gracia** (go-hygiene #3): sin anclas, todo es idéntico a F1. Un ancla sin digest da
`missing`/nada pero nunca rompe `get`. Un error de E/S transitorio **no** marca (R6). `StalenessOf` nunca
entra al camino de escritura: leer jamás muta el `SPEC.md`.

## Contratos / interfaces

**Primitivas factorizadas (nuevas exportadas en `internal/codeintel`):**
```go
func SymbolFingerprint(root, path, symbol string) (string, error) // derivada; errSymbolNotFound si no está
func FileFingerprint(root, path string) (string, error)           // sha256 del archivo entero
func SymbolMatches(s Symbol, pedido string) bool                  // regla #326: `Tipo.Metodo` exacta; `Nombre` pelada
func SplitOriginRef(ref string) (path, symbol string)             // parte por el último '#'
func NormalizeCodePath(root, path string) string
var ErrSymbolNotFound error
// internal/memory conserva: symbolFingerprint = codeintel.SymbolFingerprint (alias fino, suite #326 intacta)
```

**Gramática de una entrada de `anchors`** (lo que vive en el frontmatter):
```
entrada    := ref [ "=" digest ]
ref        := ruta [ "#" símbolo ]        # idéntica a un origin_path de observación (comparabilidad, D3)
símbolo    := Nombre | Tipo "." Metodo    # calificado = no funde homónimos (#326)
digest     := hex(sha256)                 # derivado-only en F2; se parte por el ÚLTIMO '='
# a mano, sin "=digest": válido → missing funciona, changed pende hasta el 1er guardado por tool (D2)
```

**Tipos y funciones nuevas en `internal/specgraph` (`anchors.go`):**
```go
type StaleAnchor struct {
    Path   string `json:"path"`   // el ref: ruta o ruta#símbolo
    Reason string `json:"reason"` // "changed" | "missing" (mismos valores que memory.StaleOrigin)
}
func StalenessOf(root string, fm Frontmatter) []StaleAnchor          // surface-only; no reordena; traga E/S ajena (R6)
func BuildAnchorEntries(root string, refs []string) ([]string, error) // valida+fingerprint; error R2; tope maxAnchors=10
```

**Adiciones a los DTOs de MCP (`methods_spec.go`):**
```go
// get: specNodeOut gana
Stale        []specStaleAnchorOut `json:"stale,omitempty"`        // vacío ⇒ ausente (surface-only)
StaleWarning string               `json:"staleWarning,omitempty"` // prosa antepuesta que el agente lee
// validate: specValidateOut gana (NO voltea `valid`, D8)
StaleAnchors []specStaleAnchorReport `json:"staleAnchors"`        // {id, path, reason}; [] nunca null (go-hygiene #6)
```

## Trade-offs

- **Se gana:** la rancedumbre **co-viaja** (verdad del artefacto en todo checkout); cero DB, cero migración;
  la identidad FS-derived de spec-graph intacta; una **sola** regla #326 compartida; el ancla de spec y la de
  observación son byte-comparables. El cambio de rama produce a lo sumo un `changed` defendible (el código de
  la otra rama *es* distinto del que el spec vouchó), no el `missing` falso del gotcha [daa576e9].
- **Se cede:** un **digest visible** en el frontmatter (ruido de máquina en un archivo humano) — el precio
  honesto del co-travel. Re-vouchear un ancla a propósito es explícito (quitar y re-agregar; no hay
  `--rebaseline` en F2). `grep`/`graph` no muestran rancedumbre (hay que pasar por `get`/`validate`).
- **Deuda consciente:** derivado-only deja afuera los lenguajes sin extractor para anclas de símbolo (dan
  error al guardar; la salida es anclar al archivo entero). F4 deberá excluir el digest del gist federado (D9).

## Plan de pruebas

Cada invariante con su test **saboteable** (musubi-rules #8: el sabotaje ataca el invariante que el test
declara). Todo con `GOWORK=off`; `-race` en CI (Linux); los brazos tras `runtime.GOOS` **deben** correr en la
matriz cross-platform (lección de F1: verde local en Windows ≠ verde de CI).

- **codeintel (factoring):** la suite #326 existente (`TestS1…`, line-shift, `TestEnGoElExtractorLeGana…`)
  queda **verde sin cambios** llamando a los alias — prueba que el move no bifurcó la regla. Sabotaje: romper
  `SymbolMatches` pone esos tests en rojo desde su nuevo hogar.
- **anclar y validar al guardar (R2):** `create`/`update` con `anchors` a símbolo existente crean; a símbolo
  inexistente **fallan al guardar** con el mensaje que ofrece el archivo entero. Tope `maxAnchors=10` → error,
  no truncado. Sabotaje: aflojar la validación a warning deja pasar el ancla rota.
- **changed / missing (R4):** editar el cuerpo del símbolo anclado → `changed`; borrar/renombrar → `missing`.
  Sabotaje: anclar el fingerprint a los rangos del grafo persistido (en vez de re-derivar del disco) hace que
  `changed` no dispare.
- **precisión al símbolo (R7, estilo S5/#326):** con `A.Close` y `B.Close` en el mismo archivo, tocar
  `B.Close` **no** marca una spec anclada a `A.Close`; desplazarle líneas a `A.Close` sin cambiar su contenido
  **no** marca. Sabotaje: hashear el archivo entero en vez del rango del símbolo → el test se pone rojo.
- **surface-only (R5):** una spec `changed` vuelve en la misma posición, mismo `status`, sólo con el campo
  extra. Sabotaje: hacer que la marca reordene o filtre → rojo.
- **E/S ajena no marca (R6):** un error que no es «no existe» deja la spec sin marca.
- **round-trip lossless con `anchors` (R11):** parse→serialize→parse de un `SPEC.md` con `anchors` con digest
  conserva prosa byte-a-byte, comentarios, orden, y el digest sobrevive (guarda estilo el id-roundtrip de F1).
- **validate reporta y no voltea `valid` (R8/D8):** un repo con un ancla `changed` y otra `missing` las
  reporta; `valid` sigue reflejando sólo la integridad estructural.
- **la huella no viaja (R9):** test negativo — `specgraph` no expone/llama ningún escritor de outbox/sync.
- **golden + clasificación (R12):** `TestToolsListGolden` verde tras regenerar; `create`/`update` siguen
  write, `get`/`validate` read-only (una tool read-only que mutara sería un lost-update que `-race` no ve).
