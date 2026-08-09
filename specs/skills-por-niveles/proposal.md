# Propuesta — El cuerpo de la skill viaja con la evidencia

Track **Forja global**, problema §3 de `specs/forja-global/investigacion.md`: *«sin niveles, "global" no
escala»*.

## ★ Lo primero: medir antes de construir corrigió el alcance

La investigación proyectó el muro contra `musubi_resolve_skills`: **~18.500 tokens por resolución con
100 skills**, porque la tool devuelve los `Skill` completos, `rules` incluido, y no hay niveles.

Ese cálculo sigue siendo correcto **para esa tool**. Lo que cambió desde entonces es dónde se
consumen las skills. #276 exportó el arsenal a `.claude/skills/<name>/SKILL.md`, y ese formato **ya
es progressive disclosure de fábrica**: el consumidor carga el frontmatter (`name` + `description`) y
lee el cuerpo recién cuando decide usar la skill.

Medido hoy contra las 11 instaladas:

| | bytes | ~tokens |
|---|---|---|
| nivel 1 de las 11 — `name` + la descripción con el «cuándo» | 2.349 | **587** |
| los 11 cuerpos | 12.532 | 3.133 |
| todo junto | 14.881 | 3.720 |

> El «cuándo» que #276 antepone cuesta **121 tokens** sobre los 466 que proyectó la investigación para
> un nivel 1 de `name`+`description` pelado. Es el precio de que las 6 skills con `*` no queden mudas
> justo en la capa donde se decide si se cargan — el corolario del §4.

**Conclusión incómoda y hay que decirla: la ganancia grande ya está.** En el camino que de verdad se
consume, un arsenal de 100 skills del perfil actual cuesta **~5.300 tokens fijos** y ningún cuerpo
hasta que haga falta. Eso es «global» funcionando, y no lo construye este spec: lo construyó #276.

## Entonces qué queda, y por qué igual vale hacerlo

Queda `musubi_resolve_skills`, que sigue devolviendo todo. Con honestidad sobre su tamaño:

- es el camino **programático** —el que usaría un hook, otra tool o el cuerpo—, no el del agente;
- **midió 0 llamadas en 30 días** en el ledger local y en el central. No hay un usuario sufriendo hoy;
- hoy inyecta **~1.750 tokens por resolución** (las 6 wildcard, que matchean cualquier archivo),
  relevantes o no.

Es una deuda chica y contenida. Se paga ahora porque es barata, porque deja los dos caminos con la
misma disciplina, y porque el día que algo lo llame no queremos descubrir el muro ahí.

## La regla, y por qué no es un número mágico

Podría ponerse un presupuesto de bytes y listo. Un umbral solo, sin criterio, es arbitrario: recorta
por tamaño, que no tiene nada que ver con relevancia.

La regla es otra: **el cuerpo viaja con la evidencia.**

| cómo matcheó la skill | qué se devuelve |
|---|---|
| por un glob **real** (`*.go`) | nivel 1 **+ cuerpo** |
| por **alcance declarado** (`applies_to` contra la fase/tarea que el llamador dijo) | nivel 1 **+ cuerpo** |
| **sólo** por el comodín `*` | nivel 1, con el «cuándo» |

Una skill que matcheó por `*` no tiene evidencia de ser relevante: su propio autor declaró en
`always_because` que no podía atarla a un archivo. Mandar su cuerpo en cada resolución es exactamente
el gasto que el §3 midió. Mandar su nivel 1 **con el «cuándo»** deja al llamador decidir con la misma
información con la que decide el agente en `.claude/skills/`.

Y encima, un **techo**: aunque todas hayan matcheado con evidencia, los cuerpos paran en un límite de
bytes. Sin él, tocar tres `.go` en un arsenal de 100 skills trae todos los cuerpos de `*.go`.

## El nivel 2 ya existe: no se agrega ninguna tool

`musubi_list_skills` con `query: "<nombre>"` ya devuelve la skill local **con `rules` completo**, y ya
está clasificada `readOnly`. El nivel 2 está construido. Agregar una tool nueva costaría contexto fijo
en cada sesión para duplicar algo que anda.

## Lo que esta propuesta NO hace

- **No mete inferencia.** Clasificar cómo matcheó una skill es mirar qué rama del `OR` la dejó entrar.
- **No aprieta el `*`.** Las 6 wildcard siguen matcheando cualquier archivo; lo único que cambia es
  que su cuerpo no viaja gratis. Apretar el trigger es otra decisión, y necesita que alguien declare
  fase primero.
- **No toca el export a SKILL.md.** Ahí los niveles ya son del consumidor.
- **No hace el techo configurable.** Sin un arsenal grande, un knob es una decisión que nadie puede
  tomar con datos. Es una constante con la aritmética escrita al lado.
