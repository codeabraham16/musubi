# Spec — Niveles en `musubi_resolve_skills`

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// skills
type ComoMatcheo string
const (
    PorAlcance ComoMatcheo = "alcance" // applies_to contra la fase/tarea DECLARADA
    PorGlob    ComoMatcheo = "glob"    // un trigger de archivo real
    PorComodin ComoMatcheo = "comodin" // sólo el '*': ninguna evidencia
)
type SkillResuelta struct { Skill; Matcheo ComoMatcheo; ConCuerpo bool }

func (r *Resolver) ResolveConDetalle(req ResolveRequest) ([]SkillResuelta, error)
func SeleccionarCuerpos(res []SkillResuelta, presupuesto int) []SkillResuelta
const PresupuestoDeCuerpos = 8192
```

Y en la tool, un parámetro nuevo: `detail` ∈ `auto` (default) · `full` · `summary`.

---

## H1 · El cuerpo viaja con la evidencia

### A1 — Match por glob real ⇒ cuerpo incluido

Tocar `main.go` trae `go-hygiene` **con `rules`**. Un glob que nombra la extensión es evidencia de
que la skill aplica: recortarle el cuerpo sería ahorrar donde no hay duda.

### A2 — Match por alcance declarado ⇒ cuerpo incluido

Declarar `phase:planning` trae `plan-ahead` **con `rules`**. Es la evidencia más fuerte de las tres:
el llamador dijo con todas las letras qué está haciendo.

### A3 — Match sólo por `*` ⇒ nivel 1, sin cuerpo

Tocar `main.go` trae `plan-ahead` **sin `rules`** —matcheó porque su trigger es `*`, no porque el
archivo tenga algo que ver— y **con su `cuando`**, para que el llamador pueda pedir el cuerpo si
corresponde. Es el corazón del spec: son ~1.750 tokens por resolución.

### A4 — La evidencia gana al comodín

Una skill con `triggers: ['*', '*.go']` que matchea `main.go` cuenta como `glob`, no como `comodin`.
La precedencia es `alcance` > `glob` > `comodin`: se clasifica por la mejor razón que la dejó entrar,
no por la primera que se evalúa.

---

## H2 · El techo corta aunque haya evidencia

### A5 — Pasado el presupuesto, los cuerpos que no entran se omiten

Con evidencia de sobra —tres `.go` en un arsenal grande— los cuerpos paran en
`PresupuestoDeCuerpos`. Sin techo, la regla de evidencia sola no acota nada.

### A6 — La selección es determinista

La misma entrada produce exactamente la misma selección, siempre. Orden: primero `alcance`, después
`glob`, y dentro de cada grupo por nombre ascendente. Una skill que no entra se saltea y se sigue —
así una chica detrás de una grande no queda castigada por el orden.

---

## H3 · Nada se pierde en silencio

### A7 — Toda skill matcheada aparece; lo que se omite es el cuerpo, y se declara

El largo de `active_skills` **no cambia** entre `auto` y `full`. Lo que cambia es que algunas entradas
traen `body_omitted: true`. Una skill que desaparece de la lista es indistinguible de una que no
matcheó: el peor modo de falla posible, el mismo que el vocabulario cerrado evita en `applies_to`.

### A8 — El nivel 1 no queda mudo

Toda entrada sin cuerpo trae `cuando` no vacío. Es el corolario del §4 de la investigación: para las
6 skills con `*`, el «cuándo» no vive en la `description` sino en `always_because`. Un nivel 1 que no
lo incluya las deja mudas justo en la capa donde se decide si se cargan — y entonces el ahorro se
paga en calidad, que es exactamente lo que no se quiere.

### A9 — La respuesta dice cuántos cuerpos faltan y cómo pedirlos

Cuando se omitió al menos uno, la respuesta trae la cuenta y nombra a `musubi_list_skills` como la
forma de traerlo. El nivel 2 ya existe; lo que falta es que se sepa.

---

## H4 · El llamador manda

### A10 — `detail: "full"` devuelve todos los cuerpos

Es lo de hoy, verbatim, y es la red: cualquiera que dependa del comportamiento viejo tiene una línea
para recuperarlo.

### A11 — `detail: "summary"` no devuelve ninguno

Ni siquiera los que tienen evidencia. Es la sonda más barata posible: «¿qué aplica acá?».

### A12 — Un `detail` inválido es ERROR

No cae al default. Precedente G6 de `musubi_list_skills`: degradar un typo en silencio produce una
respuesta que se lee como un dato y no como una falla.

---

## H5 · El contrato JSON deja de mentir

### A13 — Las claves son snake_case, no los nombres de campo de Go

`skills.Skill` tiene **sólo tags YAML**. Serializarla directo —lo que la tool hace hoy— emite `"Name"`,
`"Description"`, `"AppliesTo"`, y filtra `ManagedChecksum` y `GeneratedAt`, que son metadatos de cómo
Musubi gestiona el archivo en disco. Es exactamente la falla que `skillListada` documenta para
`musubi_list_skills`; acá nunca se arregló porque nadie llamaba a la tool. Se arregla ahora, con el
mismo remedio: un DTO con tags JSON.

---

## Alcance declarado

- **Cero inferencia.** Clasificar el matcheo es mirar qué rama del `OR` dejó entrar a la skill.
- **El `*` no se aprieta.** Las 6 wildcard siguen matcheando cualquier archivo. Lo único que cambia
  es que su cuerpo no viaja gratis.
- **El presupuesto es una constante, no config.** `8192` B ≈ 2.048 tokens: hoy un cambio típico en
  `.go` matchea con evidencia `analyze-project` + `deduce-conventions` + `go-hygiene` +
  `musubi-rules` = 5.063 B, y entra entero. El techo empieza a morder alrededor de 1,6× eso. Un knob
  sin un arsenal grande es una decisión que nadie puede tomar con datos.
- **No se toca el export a SKILL.md.** Ahí los niveles ya son del consumidor, y medido cuestan 587
  tokens fijos para las 11 contra 3.720 si se cargara todo.
