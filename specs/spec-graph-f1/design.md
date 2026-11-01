---
artifact: design
schema_version: "1.0"
change: spec-graph-f1
status: draft
---

# Diseño técnico — spec-graph · F1

> Fundado en el core REAL de thinkrail (`c:\Proyectos\thinkrail\packages\spec-graph`, Apache-2.0),
> leído archivo por archivo. El `core/` es pi-free (sólo `yaml` + `node:fs/path`); la capa `tools/`
> está acoplada a `@earendil-works/pi-coding-agent`. Esa frontera es la que guía el port.

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Portar `core/` fiel a un paquete Go NUEVO `internal/specgraph` (pi-free), y **re-escribir** las 7 tools nativas en `internal/mcp` (no portar la capa `tools/` de pi). | Transpilar/envolver el TS; meter las tools dentro de `specgraph`. | El `core` ya es portable (arista unidireccional host→paquete); la capa `tools/` depende de `ExtensionAPI`/`typebox`/`AgentToolResult`, y Musubi tiene SU convención de tools (`registry.go`, `RpcError`, candados, 3 goldens). La capa de tools debe ser de Musubi. |
| D2 | Raíz del índice = `s.projectPath` (la MISMA que caminan `walkSourceTree`/codegraph). | Raíz configurable; `cwd` del proceso. | El `SPEC.md` es fractal (al lado de cada módulo) por todo el repo; una sola raíz, consistente con el grafo de código; sin config nueva. |
| D3 | `specgraph.Index` es **internamente sincronizado** (su propio `sync.Mutex` sobre `scan`/`graph`/`contentEntries`/`recordForID`), porque `scan()` es un **read-modify-write** del cache. Las 7 tools se clasifican `readOnly` por su EFECTO sobre el repo (grep/get/graph/validate = read-only; create/update/delete = write), **no** por si tocan el cache del índice. | Que las tools de lectura tomen el Lock EXCLUSIVO del dispatch. | Marcar read-only las 4 de lectura es honesto (no mutan repo/DB) y las deja llamables por un principal `reader`; pero dos de ellas corriendo bajo RLock compartido llamarían `scan()` en paralelo y competirían por el `map` del cache. El índice guarda su cache efímero con su PROPIO lock; `-race` lo prueba. (go-hygiene #4: el read-modify-write va bajo lock exclusivo.) |
| D4 | Edición lossless con `yaml.v3` **`yaml.Node`** (parse→mutar→encode), replicando `updateFrontmatterText`: edición estructurada `set`/`remove`/`addList`/`removeList` + sus 4 guardas + preservación de **BOM** y **CRLF** + listas en estilo flow. | Re-serializar el frontmatter entero desde el mapa parseado. | Los specs son `.md` git-trackeados; una reescritura destructiva ensucia el diff y borra comentarios humanos. El Node de yaml.v3 preserva comentarios y orden de claves. |
| D5 | Modelo de frontmatter para el LADO LECTURA (grafo/query) = `map[string]any` (string \| []string) con accessors `Scalar`/`List`. El LADO ESCRITURA nunca pasa por el mapa: opera sobre el TEXTO del archivo vía `yaml.Node`. Vocabulario fiel: `Fields`, `SpecTypes`(5), `SpecStatuses`(5: draft/active/stale/done/deprecated), `LinkKinds`(parent, depends-on, references, implements). `RequiredFields`=[id,type,title] (para `create`); is-a-spec=[id,type]. | Un struct tipado por campo. | Fiel a thinkrail; is-a-spec sigue siendo id+type, así el diario SDD (`artifact`/`schema_version`/…) NO se captura como spec. `status:stale` existe en el vocabulario (lo pone una persona), pero ninguna tool lo auto-escribe (decisión 3 del dueño). |
| D6 | `ResolveSpecPath(root, path)` porta el endurecimiento ENTERO: vacío / absoluto / sintaxis Windows (drive o `:`) / debe terminar en `.md` / sin `..` / raíz existe / canonicalización por-segmento con case-fold (NFC+lower) que NO adivina (0 match→error, >1 match→error) / rechazo de dir ignorado / rechazo de symlink (hoja o directorio, apunte donde apunte) / contención final dentro de la raíz. NFC con `x/text/unicode/norm`. Dirs ignorados espejan `walkSourceTree` (dot-dirs, vendor, testdata, node_modules, dist, build, coverage). | Un `filepath.Clean` + prefijo de raíz. | Estas tools escriben archivos desde una ruta que da el agente; la confusión de rutas = escritura fuera del repo. El algoritmo de thinkrail ya falla cerrado en cada vector. |
| D7 | Regla always-on de 4 líneas → al texto base `instruccionesAgente` (agente.go), surfaced por `handleInitialize` en `instructions` + `_meta["anthropic/alwaysLoad"]`; DEBE entrar en el tope del cliente (`largoParaElCliente`, `TestElMapaNoPasaElTope`). SKILL en `.claude/skills/spec-graph/SKILL.md`, instalada por `musubi setup`; el mapa de skills la lista sola una vez presente. | Inyectarla por un hook (turn/precheck). | El bloque `instructions` del MCP es el canal always-on cache-friendly que YA existe (es la sección "## musubi"); un solo lugar, con presupuesto de largo medido. |
| D8 | Actualizar los **3 goldens**: `testdata/toolslist.golden.json` (+ `toolslist-completo.golden.json`) y `testdata/contrato-tools.golden.json` ({readOnly, actuaAfuera}). Las 7 con `actuaAfuera=false` (sólo archivos locales). En `registry.go`: 4 read-only, 3 write; `lock` default desde readOnly (sin `lockSelf`: no hay red/LLM); `ro` default desde readOnly (las de lectura no tocan la DB). | Tocar sólo el toolslist. | El catálogo es la única fuente de verdad Y el tripwire de seguridad por-tool; `contrato_tool_test.go` se cae si una tool nueva no entra al golden o cambia de clase. |
| D9 | `musubi_spec_validate` reporta los **3 chequeos de thinkrail** (dangling, duplicate, parent-cycles) MÁS **2 deltas de Musubi** que la spec exige (R12): (a) `type` fuera del vocabulario de 5; (b) un archivo llamado **`SPEC.md`** que NO es spec (`id`/`type` ausente o YAML malformado). El delta (b) se keyea por el **nombre `SPEC.md`** (señal de intención), NO por "cualquier `.md` sin id+type". El core `ValidateGraph` queda FIEL (3 chequeos); los deltas viven en el handler MCP, que compone `ValidateGraph` + scan de `type` sobre los nodos + `index.RejectedSpecs()`. | Portar validate tal cual (incumple R12). Marcar CUALQUIER `.md` sin id+type (inútil: gritaría por cada README y cada `proposal.md` del diario SDD). | La spec MANDA esos reportes (R12); el nombre `SPEC.md` (decisión #1 del dueño) es la única señal honesta de "esto quería ser una spec". Poner el delta en la capa de tools respeta la frontera D1 (core fiel, política Musubi en las tools). |

## Enfoque de implementación

**Paquete `internal/specgraph` (pi-free, sin cgo).** Port 1:1 de `core/`:
- `parse.go` ← `parse.ts`: `ParseFile`, `IsSpec`, `Scalar`/`List`, `SerializeFrontmatter`, `UpdateFrontmatterText` (+ `FrontmatterEdit`), constantes de vocabulario. Split del frontmatter por el fence `---`, con BOM-strip y CR-strip por línea; YAML malformado ⇒ `frontmatter=nil` (no panic).
- `graph.go` ← `graph.ts`: `BuildGraph` → `SpecGraph{nodes, edges, forward, reverse, duplicateIds}`. Identidad = `id`; el PRIMER archivo gana el nodo; `covers`/`tags` NO son aristas.
- `validate.go` ← `validate.ts`: `ValidateGraph` → dangling links + duplicate ids + **parent cycles** (recorrido de la cadena `parent`).
- `query.go` ← `query.ts`: `GrepSpecs` (substring/regex, `ignoreCase` default true, filtros type/tag/parent/depends-on, paginado limit/offset/maxBytes) y `GraphSlice` (subtree/ancestors/neighbors, depth, cada arista una vez).
- `store.go` ← `store.ts`: `ResolveSpecPath` (D6) y `Index` (D2/D3). `Index` camina el árbol en orden estable (NFC key, luego raw), stat por archivo, reparsea sólo si `(mtimeMs,size)` cambió, cachea el contenido SÓLO de specs, invalida `graphCache` ante cualquier cambio del conjunto.

**Tools nativas en `internal/mcp` (`methods_spec.go` + registro).** Cada una: lee params (schema JSON como el resto de `registry.go`), toma `getIndex(s.projectPath)`, llama al core, formatea un resumen de texto + payload estructurado con el helper de salida/truncado que ya usan las tools de lectura. `musubi_spec_grep` reusa ese truncado (puede ser grande). El `Index` se memoiza en el `McpServer` (un `map[root]*specgraph.Index` guardado, o uno solo ya que la raíz es fija).

**Puntos de contacto (aditivo):** `internal/mcp/registry.go` (7 `toolEntry` + clasificación), `internal/mcp/agente.go` (`instruccionesAgente`: +4 líneas), `.claude/skills/spec-graph/SKILL.md` (nueva), los 3 goldens. CHANGELOG `[Unreleased]` + bump semántico.

## Contratos / interfaces

**Core (firmas Go):**
```
func ParseFile(content string) (fm Frontmatter, body string)
func IsSpec(fm Frontmatter) bool
func SerializeFrontmatter(fm Frontmatter) string
func UpdateFrontmatterText(text string, edit FrontmatterEdit) (string, error)
func BuildGraph(entries []SpecFileEntry) *SpecGraph
func ValidateGraph(g *SpecGraph) ValidationReport
func GrepSpecs(entries []SpecContentEntry, opts GrepOptions) GrepResult
func GraphSlice(g *SpecGraph, opts SliceOptions) GraphSliceResult
func ResolveSpecPath(root, path string) (rel, abs string, err error)
func NewIndex(root string) *Index   // Graph/ContentEntries/RecordForID/PathForID/AbsPath/RejectedSpecs, mutex interno
type FrontmatterEdit struct { Set map[string]string; Remove []string; AddList, RemoveList map[string][]string }
type SpecReject struct { Rel, Reason string }   // un SPEC.md que falló isSpec (id/type ausente o YAML malformado)
```

> **Composición de `musubi_spec_validate` (D9):** el core `ValidateGraph` queda FIEL a thinkrail (dangling + duplicate + parent-cycles). El handler MCP COMPONE ese reporte + un scan de `type` fuera de `SpecTypes` sobre los nodos + `index.RejectedSpecs()` (los archivos llamados `SPEC.md` que no parsearon como spec). La política Musubi vive en la capa de tools; el core no se contamina (frontera D1).

**Las 7 tools MCP** (todas `actuaAfuera=false`):

| Tool | readOnly | Params | Devuelve |
|------|----------|--------|----------|
| `musubi_spec_grep` | ✅ | `pattern` (req), `regex?`, `ignoreCase?`(true), `type?`, `tag?`, `parent?`, `dependsOn?`, `limit?`(1..200), `offset?`(≥0) | páginas `path:line: snippet`, `truncated`, `nextOffset` |
| `musubi_spec_get` | ✅ | `id` (req) | nodo (id/type/title/path/frontmatter) + links hacia adelante e inversos (kind→target, path\|missing). Sin cuerpo. |
| `musubi_spec_graph` | ✅ | `root`(id, req), `direction`(subtree\|ancestors\|neighbors, req), `depth?`(1), `edge?`(para neighbors, default depends-on) | slice: nodes + edges + missing |
| `musubi_spec_validate` | ✅ | — | dangling links + duplicate ids + parent cycles + `type` fuera de vocabulario + archivos `SPEC.md` que NO son spec (`id`/`type` ausente o YAML malformado), o "válido". **F1: no reporta ranciedad.** |
| `musubi_spec_create` | ❌ | `path`(req, root-rel `.md`), `id`(req), `type`(req enum), `title`(req), `status?`, `parent?`, `dependsOn?[]`, `references?[]`, `implements?[]`, `covers?[]`, `tags?[]` | `Created <rel> (id)`. Falla si el archivo existe, el id ya está en uso, o la ruta no resuelve. Escritura exclusiva; cuerpo = headings por type. |
| `musubi_spec_update` | ❌ | `id`(req), `set?`(scalars), `remove?[]`, `addList?`, `removeList?` | `Updated frontmatter of <path>`. Lossless; prosa intacta. Guardas: no `set` id, no `set` de list-field, no `remove` id/type, debe seguir siendo spec. |
| `musubi_spec_delete` | ❌ | `id`(req) | `Deleted <path>`. Avisa de correr `validate` por links que quedan colgantes. |

## Trade-offs
- **`yaml.v3` no expone `lineWidth:0`** (thinkrail lo usa para no envolver). Un `title` largo podría envolverse a ~80 col y romper el round-trip byte-exacto. Mitigación: listas en flow (cortas, OK) + golden de round-trip; si un escalar largo envuelve, fijar su `Style` para evitar folding. **Riesgo abierto, se verifica en implement.**
- **Dos implementaciones** (TS upstream + Go) a mantener en sync si thinkrail evoluciona. Se acepta: el core es chico y estable; el comentario de cada archivo Go apunta al `.ts` de origen.
- **Índice en memoria** por raíz, sostenido en el server: cachea el contenido sólo de specs reales (no-specs guardan `""`). Acotado por la cantidad de specs del repo.
- **Asimetría deliberada walk vs resolve** (thinkrail): el `walk` ignora por nombre EXACTO (`NODE_MODULES` en mayúsculas SÍ se indexa) mientras `resolveSpecPath` sobre-rechaza por case-fold. Se porta tal cual para no divergir; documentado con su test.

## Plan de pruebas
Se porta `core.test.ts` como tests de tabla en `internal/specgraph` — ES el set de invariantes, cada uno saboteable:
- **parse/serialize:** scalars+inline arrays; block arrays→lista; YAML malformado→nil (no panic); sin fence→nil; CR-strip (última línea CRLF; flow list CRLF); serialize↔parse round-trip (+coma en item); orden de claves; status en su lugar; vacíos descartados.
- **updateFrontmatterText (lossless):** preserva comentarios + campos no-dialecto; reaplica CRLF; cuerpo byte-idéntico (endings mezclados); line-ending leído del frontmatter, NO del cuerpo; conserva BOM; rechaza `set` de list-field; NUNCA des-espec-ea (blank/rename/remove id/type).
- **graph:** forward+reverse; duplicate id (primer archivo gana). **slice:** subtree/ancestors/neighbors; cada arista una vez por profundidad.
- **grep:** filtros de metadata; `truncated` sólo si hay match más allá del limit; CR-strip para patrones anclados; limit inválido→default.
- **validate:** dangling + duplicate + parent cycles (core); + delta Musubi: `type` desconocido flaggeado; `SPEC.md` sin `type` flaggeado; `SPEC.md` con YAML malformado flaggeado; un `README.md`/`proposal.md` SIN `id`+`type` **NO** flaggeado (el keyeo por nombre `SPEC.md` evita el grito masivo).
- **Index:** ignora no-specs/node_modules/dot-dirs; re-globea agregados; reparsea modificados; borra eliminados; memoiza el grafo (rebuild sólo si cambia el conjunto); `recordForID` cachea la lectura; archivo que entra/sale de spec-hood; **orden de walk estable** (un id duplicado resuelve igual siempre); directorio en su lugar entre archivos hermanos.
- **resolveSpecPath:** rel canónico; rechaza afuera/`.txt`/ignorado; sintaxis Windows drive/stream (+ compuerta de contención final con semántica win32, test `//go:build windows`); symlink dir (aun apuntando adentro); symlink hoja (colgante o no); alias por case; falla-cerrado raíz-ausente/dir-no-listable; orden de walk NFC.
- **Concurrencia (D3):** test con `-race` que martilla `Index.Graph()`/`ContentEntries()` desde N goroutines. **Sabotaje:** quitar el lock interno ⇒ `-race` acusa escritura concurrente del map.
- **Capa MCP:** los 3 goldens en verde tras `-update`; el tripwire `contrato_tool_test.go` fija {readOnly, actuaAfuera} de las 7; el `read_concurrency_test` incluye las 4 de lectura; un test de que `instrucciones` sigue entrando en el tope del cliente con las 4 líneas nuevas.

Cada invariante se somete al sabotaje en la fase `verify` (adversarial-review): romper el código que la prueba dice cubrir y exigir ROJO, con el sabotaje atacando el invariante declarado, no una validación cualquiera.
