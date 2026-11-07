---
artifact: proposal
schema_version: "1.0"
change: r3-senales-al-recall
status: draft
---

# Propuesta — Despertar el corte R3: señales declaradas al recall

## Intención

El gate conservador del decididor de skills (R3, `hayMatchFuerte`) tiene su brazo de **match
fuerte DORMIDO en producción**: `toolRecall` invoca `exigirSkillSiCorresponde(…, reflejas=nil, …)`
a propósito, porque `musubi_recall` sólo recibe TEXTO LIBRE y el alcance casa por IGUALDAD contra un
tag, no por substring de la query. Con `reflejas=nil`, `hayMatchFuerte` es siempre `false` y el corte
nunca ahorra una llamada al motor. El VERIFY del decididor lo confirmó de primera mano: el test G3
custodia bien la UNIDAD pero guarda un branch que producción nunca ejecuta — es el "sitio equivocado"
que la revisión adversarial caza. Este cambio abre el CANAL para que ese corte sea alcanzable: el
llamador puede DECLARAR su situación (archivos tocados, fase, tarea) y, cuando un trigger ya pinó una
skill con evidencia, el motor NO se gasta.

## Alcance

- **Incluye:**
  - Tres parámetros OPCIONALES en el schema de `musubi_recall`: `modified_files` (lista),
    `phase`, `task` — calcados del `skills.ResolveRequest` ya existente.
  - El cableado en `toolRecall`: cuando el llamador declara algo y el decididor está habilitado,
    resolver el reflejo model-free sobre el arsenal ya cargado (`NewResolverDeCatalogo(arsenal).
    ResolveConDetalle(req)`) y pasarlo a `exigirSkillSiCorresponde` en vez de `nil`.
  - Tests que prueben: (a) con señales que pinan fuerte ⇒ 0 llamadas al motor (corte vivo en el
    camino de producción); (b) sin señales ⇒ idéntico a hoy (`reflejas=nil`); (c) señales que NO
    pinan (glob que no casa) ⇒ el motor igual corre.
  - Regen del golden de `tools/list` y actualización de la prosa del nodo tipado `skills-decididor`.

- **No incluye:**
  - INFERIR las señales (derivarlas de `git status`, de la fase del SDD, de archivos recién
    editados). Eso es la Fork B (ver Enfoque): acopla I/O al camino caliente del recall y rompe
    "declarados, no inferidos". Queda documentada como alternativa, NO se implementa.
  - Tocar `ElegirSkill`, el prompt, el A/B, la caché o el freno. El cambio es puramente del BORDE
    (qué entra al gate), no del juicio.
  - Cambiar el default del dial ni el comportamiento del daemon local (sigue `NoopProvider`, no-op).

## Enfoque

**Fork A — señales explícitas (ELEGIDA).** El llamador declara `modified_files`/`phase`/`task` como
params opcionales. Determinista, model-free, opt-in. Espeja exactamente la filosofía del
`ResolveRequest` existente ("Phase y Task son DECLARADOS por el llamador, no inferidos") y la del
`ResolveSkills` ("primero existe el canal, después se aprieta"). Sin señales ⇒ `nil` ⇒ el camino de
texto libre de hoy, byte a byte. Adopción incremental: el corte se enciende recién cuando un llamador
manda señales, y nadie se rompe por no hacerlo.

**Fork B — auto-derivar (DESCARTADA, documentada).** El daemon deriva las señales de `git status` /
estado del SDD. Más magia, más acople, no determinista, y mete I/O (git) en el camino caliente del
recall. Se deja anotada en el design como el camino que NO se toma y por qué.

## Impacto

- Áreas/archivos afectados: `internal/mcp/methods.go` (args + cableado en `toolRecall`),
  `internal/mcp/registry.go` (schema), `internal/mcp/testdata/toolslist*.golden.json` (regen),
  tests nuevos en `internal/mcp`, y la prosa de `specs/skills-decididor/spec.md`.
- Compatibilidad: **aditivo y retrocompatible**. Los tres params son opcionales; un llamador que no
  los manda obtiene exactamente el comportamiento actual. `musubi_recall` sigue read-only
  (`roLeeConEscrituraIncidental`): resolver el reflejo lee el catálogo en memoria, no escribe nada
  nuevo. Sin migración de base.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| El corte saltea el motor cuando SÍ habría aportado | Por diseño sólo saltea con `hayMatchFuerte` (glob que casó archivo real o alcance declarado): el motor existe para cazar un trigger demasiado angosto; si un trigger YA disparó, el motor no suma. Documentado en el gate. |
| Doble lectura de disco (arsenal + resolver) | Resolver sobre el arsenal YA cargado en memoria (`NewResolverDeCatalogo`), no re-leer `.musubi/skills`. Una sola lectura, bajo el mismo RLock. |
| El test nuevo vuelve a custodiar un branch muerto | El test (a) siembra señales reales por el camino de `toolRecall` (no inyecta `reflejas` a mano) y asierta 0 llamadas: si el cableado se desconecta, cae. Sabotaje declarado: volver `reflejas=nil` en el call-site ⇒ (a) en rojo. |
| Romper el golden de forma invisible | Regen con `-update` y revisar el diff del golden: deben aparecer SÓLO los tres params nuevos bajo `musubi_recall`. |

## Criterio de éxito

- `musubi_recall` acepta `modified_files`/`phase`/`task`; sin ellos, comportamiento idéntico a hoy
  (test de regresión en verde).
- Con señales que pinan fuerte y el decididor habilitado, el motor NO se llama (test en verde, y su
  sabotaje —`reflejas=nil`— lo pone en rojo).
- `go vet ./... && go build ./... && go test -race -count=1 ./internal/...` en verde.
- Golden de `tools/list` regenerado; `musubi_spec_validate` sin colgantes tras actualizar la prosa.
