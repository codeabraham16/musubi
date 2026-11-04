---
artifact: design
schema_version: "1.0"
change: spec-graph-f4-archive
status: draft
---

# Diseño técnico — destino del `archive` del SDD (spec-graph · F4b)

> El principio rector es la frontera de F1: **el core fiel, la política en las tools**. El merge de specs
> (`MergeSpecInto`) es **puro y agnóstico del SDD**; el conocimiento de que el `spec.md` del SDD es un
> **diario de proceso** y no una spec canónica es **política**, y vive en la capa de tools como una
> proyección que transforma el delta ANTES del merge. Esta es la decisión que el dueño delegó
> (*«haz lo tu te dy perismo y sigue»*) entre **H1** (meter el conocimiento del SDD dentro del core) y
> **H2** (proyectar en la capa de tools): se eligió **H2** (D8). Fija los dos hallazgos que la review
> adversarial de F4 (`review/spec-graph-f4-federacion`, debate `dfa7f90d`) marcó como **bloqueante 2**:
> el delta de proceso contaminaba la canónica (**H1-contaminación**) y degradaba su `status` (**H7**).

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| **D8** | **La proyección del delta de proceso vive en la capa de tools (`internal/mcp/spec_archive.go`), no en el core (`internal/specgraph`).** `MergeSpecInto` queda puro y agnóstico del SDD; `projectSDDProcessDelta` es la política que adapta el `spec.md` de proceso a forma canónica ANTES de llamarlo. | **(H1)** meter el conocimiento del SDD —podar `artifact`/`schema_version`/`change`, status canónico-autoritativo— **DENTRO** de `MergeSpecInto`. | **La decisión delegada.** H1 acoplaría el core a la plantilla genérica del SDD: `MergeSpecInto` dejaría de ser un merge canónica↔canónica reusable y tendría ramas de un dialecto de proceso. H2 respeta la frontera de F1 (verificado en la review: el fix convergente de ambos verificadores fue *«proyectar ANTES del merge puro, sin acoplar la plantilla SDD al esquema de spec»*). El core sigue sirviendo para cualquier merge de specs. |
| **D9** | **Estrategia de resolución del merge en `MergeSpecInto`:** frontmatter escalar → **gana el delta** que lo declara (salvo `id`, excluido); listas (`dependsOn`/`references`/`implements`/`covers`/`tags`) → **unión dedup**; `anchors` → **unión POR REF** con el digest del delta ganando (`MergeAnchorEntries`, heredado de F2/D7), no por string entero; cuerpo → **por secciones de heading**, aditivas al final, las que chocan **surfaceadas**. | Auto-fusión de prosa (merge de 3 vías); unión cruda de `anchors` por string. | Hereda de F2 **«marca, nunca auto-escribe»**: auto-fusionar prosa produciría texto que **nadie escribió**, y la reconciliación semántica es de una persona. La unión cruda de `anchors` dejaría `ruta#S=viejo` y `ruta#S=nuevo` como **dos** entradas; por-ref resuelve el conjunto final (el digest nuevo gana), como ya hace `update`. El escalar gana-el-delta es la convención de edición; `id` se excluye porque es la **identidad** del módulo. |
| **D10** | **La poda de claves de proceso es una DENYLIST** (`var sddProcessKeys = {artifact, schema_version, change}`), y `status` se trata **aparte** (se quita siempre; en promoción se repone con el del módulo). | **Allowlist**: conservar sólo el vocabulario de spec conocido (`ListFields`/`FieldOrder`) y tirar todo lo demás. | Una **allowlist tiraría el frontmatter canónico CUSTOM del autor** (un módulo que agrega `owner:`, `since:`, lo que sea fuera del vocabulario base) — justo lo que una canónica debe poder llevar. La **denylist preserva** eso y sólo quita lo que el SDD inyecta. **Residual honesto:** una clave de proceso **nueva** del SDD no se podaría. Riesgo **bajo**: el vocabulario de proceso (`artifact`/`schema_version`/`change`) es chico, cerrado y **estable** (lo fija la plantilla `.musubi/templates/sdd/*.md`, igual en las 4 fases documentales). Por eso es una `var` nombrada —un **único sitio**— que crece con el proceso si éste crece. |
| **D11** | **La proyección reserializa con `specgraph.SerializeFrontmatter`, NO con `UpdateFrontmatterText`.** | Construir el delta proyectado editando con `UpdateFrontmatterText` (la vía lossless de F2). | Las **guardas** de `UpdateFrontmatterText` (`frontmatter_edit.go`) lo **prohíben**: `Set` con `key==FieldID` ⇒ `errCannotRename`; `Remove` de los IdentityFields ⇒ error; y un guard final **exige** `id`+`type` o `errLeavesInvalid`. El delta proyectado en modo **MERGE** NO tiene `id`/`type` (los pone la canónica) ⇒ esas guardas lo rechazarían. `SerializeFrontmatter` es el emisor del **create-path**: reordena por `FieldOrder`, tolera un mapa sin `id`/`type`, y emite forma canónica limpia. No viola R16 (lo lossless es la edición de la **canónica**, no la del delta). |
| **D12** | **La tool ESCRIBE la canónica cuando el merge NO tiene conflictos; con conflictos no escribe y surfacea.** El DTO lo refleja (`written`), y el payload **omite** `merged` cuando escribió (ya está en disco) e **incluye** `merged` cuando no (para inspección). | Devolver siempre sólo `{merged, conflicts}` y dejar la escritura al caller/persona. | **Archivar es *persistir* la fusión.** Una tool que sólo devuelve texto deja la fase `archive` **sin destino real**: el `merged` se perdería salvo que el agente lo reescriba a mano —justo el trabajo que la tool viene a evitar—. Con conflictos **no** auto-escribe (surface-not-auto, hereda de F2/D9): escribir prosa que nadie reconcilió es exactamente lo que «marca, nunca auto-escribe» prohíbe. Por esto se clasifica **`write`** (R21): es un read-modify-write sobre la canónica, y marcarla read-only sería un lost-update que `-race` no ve. |

## Enfoque de implementación

**Dos capas, respetando la frontera de F1 (core fiel · política en tools):**

1. **`internal/specgraph/archive.go` — el core puro.** `MergeSpecInto(canonical, delta []byte) (merged
   []byte, conflicts []SectionConflict, err error)`:
   - `delta` vacío ⇒ **error** (nada que fusionar).
   - `canonical` vacío ⇒ **PROMOCIÓN**: se exige que el delta sea spec válida (id+type, R13) y se devuelve
     **verbatim** (E8).
   - si no ⇒ **MERGE**: guarda de identidad (delta id ≠ canónica id ⇒ error, R14); frontmatter por
     `frontmatterMergeEdit` + `UpdateFrontmatterText` (escalar gana-el-delta salvo `id`, listas unión,
     `anchors` por `MergeAnchorEntries`, D9); cuerpo por `mergeBodySections` + `appendSections` (aditivo,
     conflictos surfaceados, R15). El cuerpo de la canónica queda byte a byte (R16).

2. **`internal/mcp/spec_archive.go` — la política SDD.** `projectSDDProcessDelta(delta []byte,
   hasCanonical bool, moduleID, moduleType, promoteStatus string) ([]byte, error)`: parsea el delta, copia
   el frontmatter a un mapa fresco, **poda** `sddProcessKeys`, **quita** `status`; en **MERGE** reserializa
   sin inyectar nada; en **PROMOCIÓN** exige id+type (R13), valida `promoteStatus` contra
   `specgraph.SpecStatuses`, inyecta id/type/status y reserializa (D11). Puro, model-free, sin I/O.

**Flujo de la fase `archive`:** `toolSpecArchive` (handler en `methods_spec_archive.go`) lee el delta
`specs/<change>/spec.md` y la `SPEC.md` canónica del módulo del FS (vía `os.OpenRoot` + `ResolveSpecPath`)
→ llama `projectSDDProcessDelta(delta, hasCanonical, id, type, status)` → `MergeSpecInto(canonical,
proyectado)` → aplica la **política de escritura (D12): SIN conflictos escribe la canónica** (`written:true`;
archivar es *persistir* la fusión) y omite `merged`; **CON conflictos NO escribe** y surfacea
`conflicts`+`merged` para que una persona reconcilie esa prosa y vuelva a archivar (surface-not-auto, R15).
La directiva de la fase `archive` (`sdd.go:~106`) pasa de sólo-aconsejar «guardá el aprendizaje» a
**invocar** la tool. `toolSDD` **no se toca** (D8).

**Degradación con gracia** (go-hygiene #3): la tool es la única puerta; sin invocarla el SDD es idéntico a
hoy. Errores de E/S ajenos al contenido fallan **diciendo por qué** (no un merge a medias silencioso).

## Contratos / interfaces

**Core (`internal/specgraph/archive.go`):**
```go
// SectionConflict: una sección de prosa que el merge NO pudo resolver sola (mismo heading, cuerpo distinto).
type SectionConflict struct {
    Heading string `json:"heading"` // la línea de heading que diverge, tal cual en la canónica
    Canon   string `json:"canon"`   // la prosa de la canónica (la que se conserva)
    Delta   string `json:"delta"`   // la prosa del delta (la que NO se escribió, para que el caller decida)
}
// MergeSpecInto fusiona delta en canonical. canonical vacío ⇒ promueve (E8); si no, fusiona (E9).
// Error sólo ante entradas que hacen el merge imposible (delta vacío, delta sin id/type al crear,
// id del delta ≠ id de la canónica, tope de anclas excedido). PURA, surface-not-auto, lossless.
func MergeSpecInto(canonical, delta []byte) (merged []byte, conflicts []SectionConflict, err error)
```

**Política (`internal/mcp/spec_archive.go`):**
```go
// Claves del encabezado de PROCESO del SDD (vocabulario de la plantilla, no de spec-graph). DENYLIST (D10).
var sddProcessKeys = []string{sddFieldArtifact, sddFieldSchemaVersion, sddFieldChange}
// Proyecta el delta de proceso a forma canónica ANTES del merge. hasCanonical=false ⇒ promoción (exige
// e inyecta moduleID/moduleType/promoteStatus, R13); true ⇒ merge (no inyecta; la canónica es autoritativa).
func projectSDDProcessDelta(delta []byte, hasCanonical bool, moduleID, moduleType, promoteStatus string) ([]byte, error)
```

**Adición de MCP:** tool `musubi_spec_archive` — handler en `methods_spec_archive.go` y la 10ª `toolEntry`
en `methods_spec.go` — que lee delta+canónica del FS, proyecta→fusiona, **escribe la canónica si no hay
conflictos** y devuelve `{mode, change, path, written, conflicts, merged?, message}`; clasificada `write`
(sin `readOnly` en la `toolEntry`, D12/R21). Agregar una tool viva arrastra tres consecuencias mecánicas
que el build/CI vigila: el golden de `tools/list` + el contrato (regenerados), el **conteo** del README, y
el **censo** (`internal/censo`) — `musubi_spec_archive` entra como `DebeInvocarse`, igual que sus nueve
hermanas, porque la invoca la fase `archive` del SDD y un cero persistente sería el «se construye y no se
enciende» que A3 surfacea (R21).

## Trade-offs
- **Se gana:** el core (`MergeSpecInto`) queda **puro y reusable** (cualquier merge de specs, no sólo SDD);
  H1/H7 se fijan **sin una sola rama de SDD** en `internal/specgraph`; el merge es **lossless** y
  **surface-not-auto** (no inventa prosa, no pierde bytes de la canónica); la cobertura saboteable tapa el
  hueco de «la única tool de spec sin tests».
- **Se cede:** la poda es **denylist** (D10) — una clave de proceso nueva del SDD no se podaría hasta
  agregarla a `sddProcessKeys`. Declarado, de riesgo bajo, un único sitio.
- **Cubierto a nivel tool (sin deuda pendiente de este PR):** lo que el **bloqueante 2** marcó a ese nivel
  —H1 en el borde, R15/R16 (no contaminar, surface-not-auto) y que *«`archive` era la única tool de spec
  sin tests»*— queda con los **3 tests saboteables a nivel tool** (T7, R20), más la política de escritura
  (D12) y H7 (el status no degrada). Los `model-free #2/#3` que la review adversarial de F4 también citó son
  de la **federación** (#789) —viven en código que esta mitad no trae—, así que **no entran acá**.

## Plan de pruebas

Cada invariante con su test **saboteable** (musubi-rules #8: el sabotaje ataca el invariante que el test
declara). Todo con `GOWORK=off -tags treesitter -count=1`; **`-race` en CI (Linux)**, no en davantis-1
(no-cgo por diseño); `gofmt -l` **limpio** antes de reportar.

- **promoción limpia (R13/R19/E8 — el camino por defecto del SDD, test primero):** proyectar+promover un
  delta de proceso ⇒ canónica con id/type/status inyectados y **sin** clave de proceso. _Sabotaje:_
  neutralizar la poda (`for … delete` → `_ = sddProcessKeys`) ⇒ los tests H1 ven las claves y se ponen
  rojos. ✅ _hecho (`spec_archive_test.go`): sabotaje confirmado no-vacuo — SÓLO los 2 tests H1 rojos._
- **merge no degrada status (R19/E9 — H7):** fusionar un delta `draft` en una canónica `active` ⇒ status
  queda `active`. _Sabotaje:_ quitar `delete(clean, FieldStatus)` ⇒ el test H7 se pone rojo. ✅ _hecho:
  sabotaje confirmado no-vacuo — SÓLO el test H7 rojo (promoción lo repone aparte)._
- **merge no contamina + aditivo (R15/R19/E9 — H1):** el frontmatter fusionado no trae claves de proceso;
  la prosa de la canónica se conserva y la sección nueva se agrega. _Sabotaje:_ idem poda. ✅ _hecho._
- **promoción exige identidad y status válido (R13/R19/E15):** promover sin id/type ⇒ error; con status
  fuera de `SpecStatuses` ⇒ error. _Sabotaje:_ borrar cada guard ⇒ el test deja de ver el error. ✅ _hecho._
- **core: crea/fusiona/surfacea + identidad + lossless (R13/R14/R15/R16/E8/E9/E12/E13/E14):** promoción
  verbatim; merge aditivo; conflicto surfaceado sin pisar; id ≠ ⇒ error; frontmatter/cuerpo canónico
  intactos. _Sabotaje:_ p.ej. hacer que el conflicto se auto-escriba ⇒ el test de surface se pone rojo.
  ✅ _core cubierto (`archive_test.go`); los 3 tests a nivel tool se suman en T7 (`methods_spec_archive_test.go`)._
- **borde: write-when-clean / surface-on-conflict (D12/R21 — T7):** por `handleToolsCall`, merge limpio ⇒
  `written:true` y la canónica **cambia en disco**; conflicto ⇒ `written:false`, `conflicts`+`merged` en la
  salida y el FS **intacto** (ni la prosa en conflicto ni la sección nueva se escriben). _Sabotaje:_ aflojar
  la compuerta `len(conflicts)==0` → `>=0` ⇒ escribe con conflicto y el test ve la sección nueva en disco.
  ✅ _hecho (`methods_spec_archive_test.go`)._
- **golden + clasificación + censo (R21):** `TestToolsListGolden` y el **contrato** verdes tras regenerar
  (tool nueva + clase `write`); conteo de tools del README (89→90 visible, 94→95 registradas);
  `TestCadaToolVivaEstaCensada` verde con `musubi_spec_archive` censada `DebeInvocarse`. _Sabotaje:_ marcar
  la tool read-only ⇒ el dispatch toma `RLock` sobre una op que muta (lost-update que `-race` no ve); y
  quitarla del censo ⇒ `TestCadaToolVivaEstaCensada` rojo (tool viva sin clase de invocación). ✅
  _hecho (T8); el contrato registra `musubi_spec_archive` `read_only:false` y el censo la clasifica._
