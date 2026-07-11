# Diseño — C5.1 Atribución (`author`)

> SDD · fase **design** · change `captura-automatica-c5-equipo` · slice **C5.1**
> Rol: Diseñador — la opción más simple que cumple la spec; cada decisión con su rationale.

## D1 — Regla única: `author = principalFrom(ctx).Name` en el write path

**Decisión.** El autor se deriva **siempre** de la credencial en el handler de escritura MCP,
nunca de un parámetro del cliente. Unifica R3.2 (derivar de credencial) y R3.3 (sellar en el
central): es la *misma* regla aplicada donde está la credencial autoritativa.

- Cliente local (stdio / sin auth): no hay principal ⇒ `author = ""` (backward-compat, R3.6).
- Central (sync con el token de la persona): principal = "davantis" ⇒ `author = "davantis"`.
- Token legacy admin: `Name = "legacy"` ⇒ se mapea a `author = ""` (no es una persona; honra
  R3.6 "federado sin atribución"). Un admin **nombrado** sí conserva su nombre.

**Rationale.** El sellado a prueba de spoofing sale **por construcción**: el central re-deriva de
su propia credencial y no mira el payload. No hace falta que el payload de sync cargue `author`.

**Alternativa descartada.** Cargar `author` en el payload del outbox + validar en el central.
Descartada: más superficie y el central igual debe re-derivar para no confiar en el cliente →
el `author` del payload sería redundante. **El sync client NO se toca.**

## D2 — Schema: migración v16 aditiva

`ALTER TABLE observations ADD COLUMN author TEXT NOT NULL DEFAULT ''`. Sin rebuild (no cambia
PK/UNIQUE, igual que v15). Propia transacción, detrás de la guarda `user_version`.
`latestSchemaVersion()` 15 → 16. Filas viejas ⇒ `author = ''` (R3.6). **Solo observations en
C5.1**; facts/code/telemetry quedan para un follow-up (mantener el slice atómico).

## D3 — Threading del origen

Se extiende el par de métodos `*From` de observaciones para llevar `author` junto al
`originProjectID` (mismo estilo string que ya usa el project_id):

- `SaveObservationTypedFrom(originProjectID, author, id, topicKey, content, importance, memType, scope, embedding)`
- `SaveObservationDedupedTypedFrom(originProjectID, author, topicKey, content, importance, memType, scope, embedding)`
- `saveObservation(..., originProjectID, author, embedding)` — persiste `author` en el INSERT;
  en UPDATE **no** pisa el author original (espeja cómo project_id se preserva en re-saves).

Los wrappers legacy (`SaveObservation`, `SaveObservationTyped`) pasan `author=""`. La interfaz
`StorageBackend` (backend.go) suma el parámetro en esos dos métodos.

**Rationale.** Un parámetro string paralelo al project_id es la mínima perturbación consistente
con el patrón vigente; evita inventar un struct `WriteOrigin` que tocaría todos los callers.

## D4 — Derivación en el handler MCP

En `toolSaveObservation` (methods.go): `author := authorFrom(principalFrom(ctx))` — helper que
devuelve `p.Name` salvo para nil / legacy-admin (⇒ ""). Se pasa a
`SaveObservationDedupedTypedFrom(origin, author, …)`. El `origin` (project_id) ya se deriva ahí.

## D5 — Surfacing en el recall (R3.5, SHOULD)

`Observation` y `SearchResult`/gist result ganan campo `Author`. El recall/`memory_expand`
incluyen `author` cuando no está vacío (aditivo, no rompe el golden si el campo se omite en
vacío). **Filtro `author=`** en recall: **diferido** (no en C5.1) — R3.5 es SHOULD, el valor
inmediato es que el dato exista y se sincronice sellado.

## D6 — Backward-compat y golden

- Sin principal ⇒ `author=""`, scope sin cambios ⇒ comportamiento bit-a-bit actual (AC3).
- El catálogo de tools NO cambia (no se agrega parámetro de cliente; author no es input) ⇒
  **golden test intacto** (AC2). `author` es puramente derivado + salida.

## Riesgos de diseño

- **UPDATE preservando author:** un re-save de una obs ajena no debe reasignarle el autor →
  el UPDATE excluye `author` (solo INSERT lo setea), igual que project_id.
- **Nombre del principal como dato:** `Name` ya se valida al crear el principal; no hay
  inyección. Se persiste tal cual.
