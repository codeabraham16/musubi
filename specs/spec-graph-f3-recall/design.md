---
artifact: design
schema_version: "1.0"
change: spec-graph-f3-recall
status: draft
---

# Diseño técnico — spec-graph · F3-A (recall semántico sobre specs)

> Resuelve las 5 preguntas abiertas del `spec.md` (Q1–Q5) más las decisiones de arquitectura que
> se derivan de ellas. El principio rector es el de la propuesta y el de todo el subsistema:
> **FS-nativo (fresco por construcción) y model-free (fallback sin embebedor)**. La viga —dónde
> viven los vectores— cae sola apenas se mira el código: **spec-graph no toca la DB en ningún
> lado** (las anclas de F2 viven en el frontmatter, NO hay tabla `spec_anchors`; verificado con
> grep sobre `*.go`), así que meter una tabla sería el PRIMER acoplamiento a `memory.db` del
> subsistema, y reintroduciría justo el problema de sync/ranciedad que spec-graph existe para evitar.

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| **D1 (Q1)** | **Los vectores viven en un cache en memoria por proceso**, con su propio `sync.Mutex`, keyed por `path` y validado por un hash del `Content`. **NO hay tabla DB, NO hay migración.** | (b) tabla `spec_vectors` en `memory.db` keyed por `(path,mtime,size,embedder_id)`; (c) reusar la tabla `embeddings` de observaciones. | spec-graph tiene **cero** acoplamiento a DB hoy — verificado: `spec_anchors` **no existe como tabla**, las anclas de F2 son frontmatter re-derivado del disco (`StalenessOf`). Una tabla sería el primer estado paralelo del subsistema y heredaría el problema de invalidación que el `(mtime,size)` del índice (F1) resuelve gratis. El corpus de specs de un repo es chico (decenas, no los miles de observaciones), así que el recómputo en frío es barato y perezoso. R13 se resuelve en **sin migración**. El cache es RMW ⇒ mutex propio (patrón D3 del `Index`), no el candado del despacho. |
| **D2 (Q3)** | **El fallback keyword es una señal PROPIA sobre el corpus FS**: cobertura de términos del query contra el body de cada spec. **NO** reusa `musubi_search_keyword`. | Reusar el FTS5 de observaciones (`musubi_search_keyword`). | El FTS5 indexa la tabla `observations` de `memory.db`, **no** los bodies de specs. Reusarlo exigiría espejar las specs en `memory.db` — exactamente lo que la propuesta descarta (acoplaría spec-graph al motor de memoria y reintroduciría sync/ranciedad). La señal propia se computa sobre los `SpecContentEntry` que `ContentEntries()` ya reúne del FS: determinista, model-free, sin DB. |
| **D3 (Q2)** | **Un vector por spec** (el `Content` entero, el mismo texto que grepea `GrepSpecs`). Sin chunking. | Chunking del body con merge de scores por chunk. | Un `SPEC.md` es corto; el chunking agrega la complejidad de partir y recombinar scores sin beneficio medible en este corpus. El riesgo de **truncado silencioso** del embebedor (gotcha ollama banda-muerta) queda acotado porque los specs son cortos; si una F futura trajera specs largos, el chunking es la vía — se difiere, no se descarta. Se embebe el `Content` crudo (incluye el frontmatter) por simplicidad y para calzar con el corpus de grep; stripear el frontmatter antes de embeber es refinamiento futuro. |
| **D4 (Q4)** | **Snippet determinista por solapamiento de términos, independiente del modo de ranking**: la línea del body con más términos distintos del query (desempate: menor nº de línea). Si **ninguna** línea contiene un término literal (match puro-semántico, el caso estrella), cae a la **primera línea de prosa no vacía** del body. | (a) primera línea no vacía siempre; (b) fragmento de mayor relevancia semántica. | (a) es pobre cuando el match está en el medio del body. (b) exige un vector por línea (coste y complejidad de D3 que se rechazó). El solapamiento de términos da un preview útil en **los dos** modos (semántico y keyword), es 100% determinista y no cuesta un embed extra. Se recorta a ~200 chars contra el presupuesto de respuesta. |
| **D5 (Q5)** | **La tool se llama `musubi_spec_recall`.** | `musubi_spec_search`. | Espeja el par de la memoria: `musubi_recall` (semántico, con fallback) ↔ `musubi_search_keyword` (léxico), y el par del propio subsistema `musubi_spec_grep` (literal) ↔ `musubi_spec_recall` (semántico). "recall" ya connota "traé lo relevante por significado" en el catálogo; "search" es el léxico. |
| **D6** | **Clasificación: `readOnly: true` + `lock: lockSelf`** (los dos ejes, separados). Regenerar los **3 goldens** de `tools/list` y el conteo del README. | `readOnly: true` a secas (candado derivado = RLock sostenido toda la llamada). | Espejo **exacto** de `musubi_search_semantic`. `readOnly` gobierna la AUTORIZACIÓN (un principal reader puede llamarla) y el escalón de sólo-lectura (sirve sin escritura — de hecho sin DB). `lockSelf` gobierna la CONCURRENCIA: el embed es I/O de red (techo 30 s); sostener el candado del despacho —aun el COMPARTIDO— durante la red serializa el server (bloquea a todo escritor, y la captura de memoria escribe). El RMW del cache lo protege el mutex del `Recaller`, no el candado. No hace falta tramo `withReadLock`: no toca estado mutable de `s` (todo detrás del mutex del `Index` o del cache). |
| **D7** | **El seam del embebedor es una interfaz mínima `Embedder` definida EN `specgraph`**, satisfecha estructuralmente por el `Provider` que ya tiene el server. El coseno se **redefine local** (aritmética pura). | Importar `internal/embedding` y/o `internal/memory` desde `specgraph`. | spec-graph es FS-nativo y **no puede depender del motor** (misma regla por la que `anchors.go` redefine `StaleChanged`/`StaleMissing` en vez de importar `memory`). `Embedder interface { Embed(ctx, text) ([]float32, error) }` la cumple el `guarded` Provider del server **tal cual**, así que el **portero de privacidad (R7) se preserva sin que `specgraph` lo conozca**: el portero está en el constructor del Provider (invariante E2), de modo que quien embebe ve al `guarded`, no al backend. El coseno es 15 líneas de aritmética pura: redefinirlo evita arrastrar `internal/memory` entero. |
| **D8** | **Degradación en dos niveles, con scores en `[0,1]` coherentes.** Sin embebedor (`Enabled=false`) → **todo** el corpus por keyword. Con embebedor pero el embed del **query** falla/timeout → degrada **toda** la llamada a keyword (R5). El embed de **un** spec falla (`ErrSecretsBlocked` u otro) → **ese** spec cae a keyword (R7). `cosine` se clampa a `[0,1]`; `keywordScore = distinctMatched/distinctQueryTerms ∈ [0,1]`. | Mezclar coseno crudo (`[-1,1]`) con conteos de keyword sin escala común (ranking incoherente); o errorear cuando una spec no se puede embeber. | R5 y R7 exigen que la tool **nunca** falle por falta/bloqueo del embebedor. Mantener los dos modos en `[0,1]` los hace interleave sin un factor arbitrario: una spec secret-blocked que contiene todos los términos del query rankea como un match semántico decente, que es lo correcto. El orden es SIEMPRE `(score desc, id asc, path asc)` ⇒ R6; la densidad de ocurrencias se deja **fuera** del score para no introducir un desempate que contradiga R6. |
| **D9** | **Frescura (R9) en dos capas, gratis.** Capa FS: `ContentEntries()` re-escanea y revalida `(mtime,size)` en cada llamada (F1). Capa cache: la entrada se keya por `path` y se invalida comparando un hash del `Content`; body cambiado ⇒ `Content` nuevo ⇒ hash nuevo ⇒ miss ⇒ recómputo. | Keyar el cache por `(mtime,size)` plumbeado desde el `Index`. | `ContentEntries()` **no** expone `(mtime,size)` (devuelve `Path/Content/Frontmatter`), pero el `Content` ya refleja el disco revalidado, así que un hash del `Content` da la misma garantía sin plumbing nuevo. El embebedor es fijo por proceso (se fija en `NewMcpServer`), así que no puede servirse un vector de "otro embebedor" dentro de un proceso; igual se guarda el nombre del embebedor en el `Recaller` como defensa. |

## Enfoque de implementación

**Paquetes tocados.** Dos capas, respetando la frontera D1 de F1 (core fiel en `specgraph` · política/wiring en `mcp`):

1. **`internal/specgraph`** (nuevo archivo `recall.go`): el recall semántico contenido, sin tocar la DB
   ni importar el motor. Expone:

   ```go
   // Embedder es el seam MÍNIMO que specgraph necesita del motor de embeddings, definido acá para no
   // depender de internal/embedding (FS-nativo). Lo satisface el Provider YA BLINDADO del server, así
   // que el portero de privacidad (R7) viaja gratis: quien embebe ve al guarded, no al backend.
   type Embedder interface {
       Embed(ctx context.Context, text string) ([]float32, error)
   }

   type SpecHit struct {
       ID, Type, Title, Path string
       Score                 float64
       Snippet               string
   }

   // RecallResult espeja la forma de GrepResult (paginado estable).
   type RecallResult struct {
       Hits       []SpecHit
       Truncated  bool
       NextOffset int
   }

   type RecallOptions struct {
       Query  string
       Type   string // "" = sin filtro
       Limit  int    // <=0 → DefaultRecallLimit
       Offset int
   }

   // Recaller guarda el cache de vectores por proceso (su propio mutex, D3). Vive en el server,
   // construido una vez; el corpus llega FRESCO en cada Recall (D9 capa FS).
   type Recaller struct {
       mu      sync.Mutex
       cache   map[string]cachedVec // path → {hash, vec}
       embName string               // defensa R9: embebedor fijo por proceso
   }
   func NewRecaller(embName string) *Recaller

   // Recall rankea el corpus por relevancia al query. embEnabled viene de embedding.Enabled(provider):
   //   false            → modo keyword para todo el corpus (R5).
   //   true, query falla → degrada toda la llamada a keyword (R5).
   //   true, spec falla  → esa spec cae a keyword (R7); el resto por coseno.
   func (r *Recaller) Recall(ctx context.Context, entries []SpecContentEntry, opts RecallOptions,
       emb Embedder, embEnabled bool) (RecallResult, error)
   ```

   Helpers puros y testeables por separado: `cosine(a, b []float32) float64` (clamp `[0,1]`),
   `keywordScore(qterms map[string]struct{}, content string) float64` (`distinctMatched/len(qterms)`),
   `bestSnippet(qterms, content) string` (D4), `tokenize(s) []string` (lower + split por no-alfanumérico),
   `bodyLines(content) []string` (saltea el bloque de frontmatter `---…---`). `DefaultRecallLimit = 20`.

   - **is-a-spec (R8) gratis**: `ContentEntries()` ya devuelve SÓLO specs reales (`scan()` filtra por
     `IsSpec`), así que el diario SDD (`artifact`+`schema_version`, sin `id`+`type`) nunca entra al corpus.
   - **filtro `type` (R10)**: se filtra el corpus por `Scalar(fm, FieldType)` antes de rankear.
   - **orden determinista (R6)**: `slices.SortFunc` por `(score desc, id asc, path asc)`.
   - **paginado (R10)**: tras ordenar, `offset`/`limit`; `Truncated` y `NextOffset` como `GrepResult`.
   - **concurrencia**: el embed corre FUERA del mutex del cache (lock → mirar cache → unlock → embed →
     lock → guardar → unlock); dos llamadas concurrentes pueden re-embeber el mismo spec (desperdicio,
     nunca incorrecto: mismo vector). No se sostiene el mutex durante la red, igual que no se sostiene
     el candado del despacho.

2. **`internal/mcp`**: el wiring y la política.
   - `server.go`: campo `specRecaller *specgraph.Recaller`, construido en `NewMcpServer` junto a
     `specIndex` (`specgraph.NewRecaller(embedder.Name())`). Persiste el cache entre llamadas.
   - `methods_spec.go`: handler `toolSpecRecall` + DTOs tagueados (`specRecallOut`, `specHitOut`, claves
     minúscula, `make` para slices no-nil). Arma el corpus con `s.specIndex.ContentEntries()`, consulta
     `embedding.Enabled(s.embedder)`, y llama `s.specRecaller.Recall(ctx, entries, opts, s.embedder, enabled)`
     con un `context.WithTimeout(ctx, 30*time.Second)`. `query` vacío ⇒ `codeInvalidParams` (R1). `limit`
     se recorta por arriba (tope duro, como `specGrepLimitMax`).
   - `registry.go`: nueva `toolEntry` para `musubi_spec_recall` con `readOnly: true` **y** `lock: lockSelf`
     (D6), en el bloque de las spec tools. Regenerar los **3 goldens** (`go test ./internal/mcp -run
     TestToolsListGolden -update`) y el conteo del README (`readme_toolcount_test.go`).

**Sin migración** (D1): no se toca `internal/memory/migrations.go`. R13 se cumple porque no hay tabla.

## Contratos / invariantes y su test (R12)

Cada invariante lleva UN test que se pone en **rojo bajo sabotaje del invariante que declara** (no una
validación cualquiera; la defensa en profundidad no debe taparlo):

| Invariante | Test (rojo bajo sabotaje de…) |
|---|---|
| R4 semántico: el significado gana al texto (caso estrella "espera exponencial" / query "backoff") | con un `Embedder` de prueba determinista que mapea sinónimos cerca, la spec sin la palabra literal sale al top. Sabotaje: devolver coseno 0 ⇒ cae del top. |
| R5 fallback sin error (invariante central) | `embEnabled=false` ⇒ devuelve hits por keyword, `err==nil`. Sabotaje: hacer que `Recall` retorne error sin embebedor ⇒ test rojo. |
| R5 degradación cuando el embed del query falla | `Embedder` que retorna error en el query ⇒ cae a keyword sin error. Sabotaje: no atrapar el error del embed ⇒ rojo. |
| R6 orden determinista | dos specs con el mismo score ⇒ salen en orden `(id, path)` estable entre corridas. Sabotaje: ordenar sólo por score (sort inestable) ⇒ orden no reproducible. |
| R7 portero / spec secret-blocked cae a keyword | `Embedder` que retorna `ErrSecretsBlocked`-like para un spec ⇒ ese spec aparece (score keyword), sin error. Sabotaje: propagar el error del spec ⇒ rojo. |
| R8 el diario SDD no es spec | un `proposal.md` (`artifact`+`schema_version`) en el corpus del test ⇒ NO aparece. Sabotaje: no filtrar `IsSpec` ⇒ aparece ⇒ rojo. |
| R9 frescura | embeber una vez; cambiar el `Content`; segunda llamada rankea el body nuevo. Sabotaje: keyar el cache sólo por `path` (sin hash) ⇒ sirve el vector viejo ⇒ rojo. |
| R10 filtro `type` + paginado | `type="module-design"` ⇒ sólo ese tipo; `limit/offset` ⇒ `Truncated`/`NextOffset` correctos. Sabotaje: ignorar `type` ⇒ rojo. |
| R3/D6 read-only + lockSelf | `read_surface_class_test`/golden: la tool es `readOnly` y `lockSelf`; ninguna llamada muta un `SPEC.md`. Sabotaje: marcarla write ⇒ el test de superficie de lectura cambia. |

## Fuera de alcance (recordatorio del `spec.md`)

Cross-link con `musubi_code_graph`/`musubi_impact` y overlay en `musubi_map` (F3-B); fundir en
`musubi_recall`; federación (F4); arbitraje (F5). El coseno y el keyword de acá **no** se comparten con
el motor de memoria: son copias locales por la regla FS-nativa (D7).
