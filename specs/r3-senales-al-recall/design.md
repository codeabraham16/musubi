---
artifact: design
schema_version: "1.0"
change: r3-senales-al-recall
status: draft
---

# Diseño técnico — Despertar el corte R3: señales declaradas al recall

## Decisiones de arquitectura
| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Señales EXPLÍCITAS como params opcionales de `musubi_recall` (Fork A) | Fork B: auto-derivar de `git status` / fase del SDD | Determinista, model-free, sin I/O en el camino caliente; espeja `ResolveRequest` ("declarados, no inferidos") y "primero el canal, después se aprieta". Fork B acopla git al recall y vuelve el corte no reproducible. |
| D2 | Resolver el reflejo sobre el arsenal YA cargado con `NewResolverDeCatalogo(arsenal).ResolveConDetalle(req)` | Llamar `s.resolver.ResolveConDetalle(req)` (re-lee disco) | El arsenal ya se cargó con `LoadSkills` en el mismo bloque; re-leer `.musubi/skills` sería una segunda lectura de disco por recall. El resolver de catálogo usa el MISMO matcher probado. |
| D3 | Resolver SÓLO cuando `req.DeclaraAlgo()` y `len(arsenal)>0` | Resolver siempre y dejar que `hayMatchFuerte(nil-equivalente)` corte | Sin señales no hay nada que resolver; evitar el `verifyCapabilities`/matcheo inútil mantiene el caso texto-libre (el más común) sin trabajo extra, y `reflejas` queda nil, idéntico a hoy. |
| D4 | El guard `decididorHabilitado()` sigue envolviendo TODO (disco + resolución) | Resolver el reflejo fuera del guard | S7: el daemon local no debe tocar el disco de skills. Mantener disco+resolución dentro del `if s.decididorHabilitado() && s.resolver != nil` preserva local-first. |
| D5 | `reflejas` se resuelve en `toolRecall` (el borde), no dentro de `exigirSkillSiCorresponde` | Mover la resolución al seam de cognición | El seam ya recibe `reflejas` como parámetro (su contrato no cambia); la fuente de la señal es del BORDE, que es quien parsea los args del tool. Un solo call-site cambia. |

## Enfoque de implementación

Un único call-site cambia: `internal/mcp/methods.go`, bloque TRAMO 3b de `toolRecall`.

**1) Args del tool** — agregar tres campos al struct anónimo de `toolRecall`:

```go
var args struct {
    Query       string `json:"query"`
    TokenBudget int    `json:"token_budget"`
    Rerank      *bool  `json:"rerank"`
    // R3 (decididor) — señales OPCIONALES de la situación del llamador. Pueblan el reflejo
    // model-free que APAGA el motor cuando un trigger ya pinó una skill con evidencia (glob que
    // casó un archivo, o alcance declarado). Ausentes ⇒ nil ⇒ texto libre como siempre. Espejan
    // skills.ResolveRequest; el daemon local nunca llega a usarlas (guard decididorHabilitado).
    ModifiedFiles []string `json:"modified_files"`
    Phase         string   `json:"phase"`
    Task          string   `json:"task"`
}
```

**2) Cableado** — reemplazar el `reflejas=nil` del call-site. El bloque queda:

```go
if s.decididorHabilitado() && s.resolver != nil {
    var arsenal []skills.Skill
    s.withReadLock(func() {
        if cat, lerr := s.resolver.LoadSkills(); lerr == nil {
            arsenal = cat
        } else {
            logx.Error("decididor de skills: no pude leer el arsenal, recall sin exigencia", "error", lerr)
        }
    })
    // R3 DESPIERTO: si el llamador DECLARÓ señales (archivos/fase/tarea), resolvemos el reflejo
    // model-free sobre el arsenal ya cargado —sin re-leer disco— y se lo pasamos al gate. Sin
    // señales declaradas, reflejas = nil y el camino es idéntico al de texto libre. El alcance casa
    // por igualdad de tag y el glob contra el nombre del archivo, así que ahora SÍ hay con qué pinar.
    var reflejas []skills.SkillResuelta
    if req := (skills.ResolveRequest{ModifiedFiles: args.ModifiedFiles, Phase: args.Phase, Task: args.Task}); req.DeclaraAlgo() && len(arsenal) > 0 {
        if rs, rerr := skills.NewResolverDeCatalogo(arsenal).ResolveConDetalle(req); rerr == nil {
            reflejas = rs
        } else {
            logx.Error("decididor de skills: no pude resolver el reflejo, gate sin match-fuerte", "error", rerr)
        }
    }
    res = s.exigirSkillSiCorresponde(ctx, args.Query, res, reflejas, arsenal)
}
```

**3) Schema** — en `registry.go`, bajo las Properties de `musubi_recall`:

```go
"modified_files": {Type: "array", Description: "OPCIONAL. Archivos que la tarea está tocando. Si uno casa el glob de una skill, el decididor NO gasta el motor: el trigger ya eligió. Vacío ⇒ sólo el texto libre decide.", Items: &Property{Type: "string", Description: "ruta o nombre de archivo"}},
"phase":          {Type: "string", Description: "OPCIONAL. La fase declarada (vocabulario de applies_to, ej. 'implement'). Si casa el alcance de una skill, el decididor no gasta el motor."},
"task":           {Type: "string", Description: "OPCIONAL. La tarea declarada (vocabulario de applies_to). Misma lógica que phase."},
```

**4) Golden** — regenerar `toolslist.golden.json` y `toolslist-completo.golden.json` con
`go test ./internal/mcp -run TestToolsListGolden -update`.

**5) Prosa del nodo tipado** — en `specs/skills-decididor/spec.md` (Interface), anotar que el input del
gate `hayMatchFuerte` llega de las señales declaradas en `musubi_recall` (R3 alcanzable), sin tocar el
frontmatter. Se edita a mano (prosa), NO con las tools de spec.

## Contratos / interfaces

- `exigirSkillSiCorresponde(ctx, query, res, reflejas []skills.SkillResuelta, arsenal []skills.Skill)`
  — **sin cambios de firma**: ya recibe `reflejas`. Sólo cambia quién lo llena.
- `skills.ResolveRequest{ModifiedFiles, Phase, Task}` y `ResolveConDetalle` — **sin cambios**: se
  reutilizan tal cual.
- `musubi_recall` input schema — **aditivo**: +3 propiedades opcionales, `Required` sigue `["query"]`.
- Clase read-only de `musubi_recall` — **sin cambios**: `roLeeConEscrituraIncidental`.

## Trade-offs

- **Se gana:** el corte R3 pasa de dead-code-en-producción a alcanzable; un ahorro real de llamadas
  al motor cuando un trigger ya acertó; el test G3 deja de custodiar un branch que prod nunca corre.
- **Se cede:** el ahorro sólo se materializa si el llamador manda señales — adopción incremental, no
  automática. Es deliberado (D1): el canal existe, apretar la adopción es un paso aparte.
- **Costo nulo en el caso común:** sin señales, cero trabajo extra (D3); en el daemon local, el guard
  corta antes de tocar disco (D4).

## Plan de pruebas

Tests nuevos en `internal/mcp` (junto a `decididor_skills_test.go`), todos por el camino REAL de
`toolRecall` (no inyectando `reflejas` a mano — eso es lo que convirtió a G3 en "sitio equivocado"):

1. **Match fuerte por glob apaga el motor** — decididor habilitado con un motor-espía que cuenta
   llamadas; arsenal con skill `*.go`; `musubi_recall` con `modified_files:["x.go"]` ⇒ 0 llamadas al
   espía. **Sabotaje declarado:** volver `reflejas=nil` en el call-site ⇒ el espía cuenta 1 ⇒ rojo.
2. **Match por alcance apaga el motor** — skill `applies_to:[implement]`; recall con
   `phase:"implement"` ⇒ 0 llamadas.
3. **Sin señales, el motor corre** — recall sin señales ⇒ el espía cuenta 1 (regresión del camino de
   hoy). **Sabotaje:** si el cableado pusiera algo en `reflejas` sin señales ⇒ rojo.
4. **Señal que no casa, el motor corre** — skill `*.py`, recall con `modified_files:["x.go"]` ⇒ 1
   llamada (el glob no casa, no hay match fuerte).
5. **Daemon local ignora señales** — `NoopProvider`; recall con señales ⇒ sin exigencia, y (si es
   observable) sin lectura de disco de skills.
6. **Golden** — `TestToolsListGolden` en verde tras `-update`; revisar a ojo que el diff del golden
   sean SÓLO los 3 params bajo `musubi_recall`.

Gate del repo antes de cerrar: `GOWORK=off go vet ./... && go build ./... && go test -race -count=1
./internal/mcp/... ./internal/skills/...`. Sabotaje verificado a mano en los tests 1 y 3.
