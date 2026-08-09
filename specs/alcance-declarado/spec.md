# Spec — El porqué del `*` se vuelve alcance

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// skills
type Skill struct { ...; AppliesTo []string `yaml:"applies_to,omitempty"` }
type ResolveRequest struct { ModifiedFiles []string; Phase string; Task string }
func (r *Resolver) ResolveSkills(req ResolveRequest) ([]Skill, error)
func VocabularioDeAlcance() []string
```

El vocabulario v1, **transcrito** de los `always_because` que ya existen:

| valor | de quién sale |
|---|---|
| `phase:planning` | `plan-ahead` — «planificar es una FASE del trabajo» |
| `phase:implementing` | `sdd-flow` — «el flujo completo de un cambio» |
| `phase:reviewing` | `adversarial-review` — «al cerrar un cambio de riesgo» · `sdd-flow` |
| `task:audit` | `audit-structure-flow` — «el pedido de auditoría» |
| `task:orchestration` | `orchestrate-multiagent` — «grande y paralelizable» |

---

## H1 · Se puede preguntar sin archivos

### A1 — Declarar una fase alcanza para resolver

`ResolveSkills({Phase: "planning"})`, sin un solo archivo, devuelve `plan-ahead`. Es el corazón del
spec: hoy esa llamada es un error de parámetros.

### A2 — Sin archivos y sin declaración sigue siendo un error

No se afloja la validación: se le agrega una segunda forma de satisfacerla. Una llamada que no dice
NADA no tiene respuesta útil, y devolver el arsenal entero sería peor que fallar.

---

## H2 · Es aditivo: nada de lo que andaba deja de andar

### A3 — Los globs siguen resolviendo igual

Con `ModifiedFiles` y sin declaración, el resultado es **idéntico** al de antes del cambio. Es la
red de regresión: 5 de las 11 skills viven de sus globs.

### A4 — El comodín sigue siendo comodín

Una skill con `triggers: ['*']` y `applies_to` **también** matchea por archivo. Si migrar a
`applies_to` le sacara el match por archivo, las 5 skills migradas dejarían de activarse mientras
ningún llamador declare su fase — un arreglo que se ve como regresión.

---

## H3 · El alcance declarado matchea con precisión

### A5 — Sólo matchea lo declarado

Declarar `phase:planning` NO activa la skill de `phase:reviewing`. Igualdad exacta de strings: sin
prefijos, sin heurística, sin distancia. Es lo que mantiene el matcher determinista y gratis.

### A6 — Un valor fuera del vocabulario es ERROR de validación

Vocabulario **cerrado**, como `validOutcome` del ledger o el enum de predicados de cognición. Un
`applies_to: [phase:planing]` con typo se vería igual que una skill que nunca aplica: silencioso, y
el peor modo de falla posible.

### A7 — Declarar no rompe a quien no declara

Una skill SIN `applies_to` nunca matchea por declaración, y una declaración no la excluye de
matchear por sus globs. Los dos caminos son independientes.

---

## H4 · El validador deja de mentir

### A8 — `desc_no_trigger` no aprueba por subcadena

`adversarial-review` **pasa hoy** el check y no debería: la lista busca `"al "` dentro del texto, y
«revisión advers**al** estilo…» lo contiene. Un token de 3 caracteres buscado adentro de palabras
aprueba cualquier cosa. El match tiene que respetar límites de palabra.

Medido: el validador reporta 6 de 11 en warning; las que de verdad no dicen cuándo son 8.

---

## Alcance declarado

- **El vocabulario nace con 5 valores** porque son los 5 que alguien escribió. Crecerá con evidencia,
  no con imaginación.
- **`project-profile` no se migra**: es de proyecto entero y ningún valor le queda. Forzarlo sería
  inventar el eje que la propuesta dice no inventar.
- **Cero inferencia.** Todo el matcheo es igualdad de strings y globs. El costo en tokens del
  resolvedor no cambia en esta fase — bajarlo es el spec de niveles.
- **No se aprieta nada todavía.** Las 5 migradas conservan su `*`.
