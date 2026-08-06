# Propuesta — Dominios ajenos no se juzgan (F3 del track «Potencia medida»)

## El problema, medido

La cola de conflictos de la memoria real tiene **494 relaciones**. De ellas, **45 son señal**
—`supersedes` o `conflicts_with`, las únicas que exigen una decisión— y **413 son ruido**:
`related`, `compatible`, `not_conflict`, `scoped`, veredictos que significan «acá no había nada que
decidir».

**Proporción de señal: 9,8 %.**

Y la cola sigue creciendo: guardar una sola nota en una sesión de trabajo disparó ocho relaciones
pendientes de golpe, y la cola pasó de 20 a 36 en unas horas.

## Por qué esto importa más de lo que parece

El proyecto ya escribió la razón, en el PR #203 que atacó la primera clase de este mismo ruido:

> **El daño real no era el ruido, era la EROSIÓN**: una cola llena de falsos positivos DEJA DE
> LEERSE, y el día que aparezca la contradicción REAL se pierde entre las demás. El dedup semántico
> vale lo que valga la CREDIBILIDAD de su cola.

Con 9,8 % de señal, la cola ya está en ese territorio.

## Qué encontró la medición

De las 36 relaciones pendientes, **31 cruzan dominios de primer nivel distintos**: `gio` ×
`last-chaos-nostalgia`, `roadmap` × `cognicion`, `auditoria` × `minecraft`. Sólo 5 son del mismo
dominio.

Eso tiene una explicación simple: el detector dispara por **parecido de forma**. Dos auditorías
multi-agente se parecen muchísimo entre sí —misma estructura, mismo vocabulario de método, mismos
verbos— aunque una hable de las terminales de una persona y la otra del herramental de un
videojuego. El coseno las ve gemelas; el contenido no tiene nada que ver.

## La pregunta que decidió el diseño

Antes de proponer una guarda por dominio había que medir su **costo**: ¿alguna vez una relación
entre dominios distintos fue señal de verdad?

**De las 45 señales, 44 son del mismo dominio. Una sola cruza** — y es `git-commit` × `bugs`, que
tiene todo el sentido: un commit es un evento que puede volver obsoleta una nota de cualquier
dominio. `git-commit` no es un dominio temático, es el registro de lo que pasó.

La primera versión de la guarda eximía **sólo** a `git-commit`, y estaba mal: rompió un test que
ya existía (`TestSoloLasCreenciasSeReemplazan/contrato -> nota`), cuyo mensaje lo dice mejor que
cualquier comentario — *«la guarda se pasó de ancha, es un martillo»*. Un contrato SDD también es un
registro histórico, y por la misma razón puede volver obsoleta una nota de cualquier tema. La
excepción final se escribe con `historicalRecord`, no con `isCommit`.

Con esa excepción corregida, la simulación sobre el histórico completo da:

| | |
|---|---|
| Relaciones que la guarda evitaría crear | **128 de 494 (26 %)** |
| Señal perdida | **0** |
| Señal conservada | **45 de 45** |
| De las 36 pendientes de hoy | **30 evitadas, 6 quedan** |

Un cuarto menos de relaciones, cero señal perdida, y la cola de pendientes baja de 36 a 6.

## Qué NO es

- **No oculta memoria.** Es un `continue` en el loop de detección, no un `DELETE`. El peor caso de
  un falso negativo es *una relación de menos en la cola*, jamás una observación de menos en el
  recall. Misma decisión que las guardas del #203.
- **No toca umbrales, ni el AND-gate, ni el gate de novedad.** No mueve ninguna perilla de scoring.
- **No mira el contenido**, a propósito. El parecido entre dos auditorías de dominios distintos es
  alto y *correcto*; mirar el texto sólo confunde. La decisión se toma por el `topic_key`, igual que
  las guardas que ya existen.

## Costo y reversibilidad

Una función de comparación y un `continue`. Sin esquema, sin config, sin dependencias. Revertirla es
borrar tres líneas.
