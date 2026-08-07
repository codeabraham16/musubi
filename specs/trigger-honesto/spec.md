# Spec — El trigger honesto

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## H1 · El campo `always_because`

`skills.Skill` gana un campo opcional:

| Campo | YAML | JSON | Significado |
|---|---|---|---|
| `AlwaysBecause` | `always_because` | `always_because` | por qué esta skill se activa **siempre** |

Es `omitempty` en las dos serializaciones: una skill con triggers acotados no lo necesita y el
archivo queda limpio.

**Piso de contenido: 20 caracteres** (`MinAlwaysBecauseChars`). No es un capricho de longitud — es
lo mínimo para que la frase diga algo. `"porque sí"` no es una declaración de alcance.

### G1 — El campo sobrevive al ida y vuelta del YAML

Escribir una skill con `always_because` y volver a leerla del disco devuelve el mismo texto. Si el
campo no se persiste, todo lo demás es teatro.

### G2 — El campo cruza la red entera

`promote` → central → `list` → `install` conserva la justificación **literal**. Son cuatro sobres
distintos y cada uno la puede tirar en silencio:

| Sobre | Struct | Si le falta el campo |
|---|---|---|
| subida | `skillPayload` | el central guarda la skill sin razón |
| recepción | `argsGuardarSkill` | idem, y **del lado del central**, que corre otro binario |
| catálogo | `skillListada` | se ve el arsenal pero no por qué algo está siempre encendido |
| vuelta | `ListArsenal` → `skills.Skill` | se instala sin la razón que justificó subirla |

Es exactamente la trampa que ya documenta `skillPayload`: serializar con los tags equivocados no
falla, **guarda vacío**.

---

## H2 · El detector deja de ser ingenuo

`allWildcard` (todos los triggers son `*`) se reemplaza por `anyWildcard` (**alguno** lo es).

> Un `*` entre los triggers hace que el resto no signifique nada: la skill ya se activa en todo.

Dos hallazgos distintos, con códigos estables:

| Código | Cuándo | Severidad |
|---|---|---|
| `triggers_over_broad` | hay `*` y **no** hay `always_because` válido | warning |
| `triggers_wildcard_masks_specific` | hay `*` **mezclado** con triggers específicos | warning |

### G3 — Declarar el motivo apaga la advertencia

Una skill con `["*"]` y un `always_because` de ≥20 caracteres **no** emite
`triggers_over_broad`. Ése es el punto entero: el campo hace trabajo real, no es burocracia.

### G4 — Una justificación de relleno no alcanza

`always_because: "porque sí"` (9 caracteres) deja el warning encendido.

### G5 — El caso enmascarado se nombra aparte, y el motivo no lo excusa

`["*", "*.go"]` emite `triggers_wildcard_masks_specific` **tenga o no** `always_because`: la razón
justifica el `*`, no vuelve honestos a los triggers decorativos que lo acompañan.

### G6 — Un predicado, dos consumidores

`WildcardUnjustified(Skill)` es la **única** definición de «`*` sin declarar». La usan el score de
calidad y la puerta de promoción. Dos copias se desincronizarían y dirían cosas distintas de la
misma skill.

---

## H3 · La puerta de promoción cobra

`musubi_promote_skill` gana dos chequeos que **hoy no tiene**:

1. **El gate de calidad completo** (`ValidateSkillQuality`). Los errores bloquean, como en las otras
   tres puertas. Hoy la única puerta sin gate es justo la que publica para todos.
2. **El `*` injustificado bloquea** — sólo acá. Es un `warning` en el score y un `error` en esta
   puerta, a propósito: en tu proyecto el alcance es tu problema; en el arsenal es de todos.

### G7 — Lo que no pasa el gate no sube

Promover una skill con errores de calidad devuelve error y **no llama al central**. Se verifica
contando llamadas: un rechazo que igual hace el POST no es un rechazo.

### G8 — El `*` sin declarar no sube, y el mensaje dice cómo arreglarlo

El error nombra el campo `always_because` y el piso de caracteres. Un rechazo que no dice qué hacer
convierte la herramienta en adivinanza — que es el mismo defecto que la Fase B arregló en
`install_skill`.

### G9 — El `*` declarado sube sin fricción

Con `always_because` válido, promover funciona exactamente como antes. Control del G8: si la única
prueba fuera el rechazo, romper la tool entera pasaría en verde.

### G10 — Lo acotado nunca se toca

Una skill con `["*.go"]` no ve ninguno de los dos chequeos nuevos. Es la mayoría del arsenal y no
debe pagar por un problema que no tiene.

---

## Alcance declarado

- **Local no cambia.** `musubi_save_skill` sigue aceptando `*` sin justificar; sale el warning y
  listo. Endurecerlo ahí rompería 7 skills vivas de este repo por un problema que sólo existe
  cuando el conocimiento se comparte.
- **Lo ya promovido no se toca.** Este gate corre en promociones nuevas. El arsenal tiene 2 skills
  y las dos están acotadas (`go-hygiene`: `*.go`), así que no hay nada que migrar — pero si lo
  hubiera, tampoco se migraría solo.
- **Un central viejo tira el campo en silencio.** Si el otro lado no conoce `always_because`, la
  skill llega sin razón y nadie se entera. No se agrega negociación de versión: se documenta, y se
  verifica contra el central real antes de dar el track por cerrado.
