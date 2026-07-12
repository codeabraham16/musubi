# Proposal — conflictos-artefactos-mismo-cambio

## Intención

**El 61% de la cola de pendientes es ruido fabricado, y crece con cada cambio.** Medido sobre la memoria real del dogfood: **14 de 23** relaciones `pending` son **artefactos del mismo cambio relacionándose entre sí**.

| Patrón | Relaciones |
|---|---|
| `sdd/<cambio>/archive` → `sdd/<MISMO cambio>/{design,spec,proposal,verify,implement}` | 10 |
| `git-commit` (PR #201) → `sdd/captura-commit-squash-twin/{proposal,spec,design,archive}` (coseno hasta **0.93**) | 4 |
| Relaciones **genuinas** (memorias distintas que hablan del mismo tema) | 9 |

El flujo SDD guarda **7 contratos por cambio** (proposal→…→archive). Los siete describen **el mismo cambio**: por construcción hablan de lo mismo. El detector los ve parecidos y pide un veredicto por cada par. El commit de ese mismo cambio también se parece a sus propios contratos.

**Pero un `proposal` y un `design` no son duplicados: son complementarios.** Ninguno se puede borrar. Pedir un juicio ahí es pedir que se decida algo que no tiene decisión.

## Por qué importa (el daño real no es el ruido, es la erosión)

Una cola llena de falsos positivos **deja de leerse**. Si cada PR mete ~10 pendientes falsas, el humano deja de mirar la cola — y el día que aparezca la **contradicción real**, se pierde entre las 23.

**El valor del dedup semántico (#193) depende de que la cola sea CREÍBLE.** Este ruido es, literalmente, una amenaza a la feature que lo genera.

## La distinción que habilita resolverlo solo

Es la misma que ganamos con el gemelo del squash (#201), un nivel más arriba:

> Que dos memorias del **mismo cambio** se parezcan es un **hecho estructural**, no una interpretación. El parecido entre el `spec` y el `design` de un cambio no significa redundancia — **significa que son del mismo cambio**. Es exactamente lo esperable.

Y cruzando clases de artefacto: un **commit** es el *evento*, un **spec** es el *contrato*. Uno nunca puede reemplazar al otro; borrar cualquiera **pierde información**. Que se parezcan es correcto, no sospechoso.

Por eso acá **no hace falta juicio**: se puede decidir con la **estructura** (el `topic_key`), sin mirar el significado.

## Alcance

Dos guardas **estructurales** en la detección de conflictos, antes de proponer cualquier relación:

- **G1 — Hermanos del mismo cambio SDD.** Si ambos `topic_key` son `sdd/<cambio>/<fase>` con el **mismo `<cambio>`** ⇒ **no se propone relación**. Son complementarios por construcción.
- **G2 — Clases de artefacto distintas.** Un `git-commit` y un contrato `sdd/*` **nunca** son redundantes entre sí ⇒ **no se propone relación**.

Ambas son decidibles con el `topic_key`, que `obsRow` **ya trae**. Sin cambio de esquema.

## Fuera de alcance (explícito)

- NO se toca el umbral de coseno, ni el AND-gate, ni el gate de novedad (#195). Todo lo que **sí** requiere juicio sigue yendo a `pending`.
- NO se ocultan ni archivan observaciones. Una guarda **evita crear** una relación; no borra memoria.
- NO se toca la detección entre memorias de la **misma** clase (dos notas, dos commits): ahí el parecido **sí** puede ser redundancia y se sigue delegando al agente.
- NO se hace limpieza retroactiva **en el código**. Las 14 pendientes ya existentes se resuelven a mano como `related` (veredicto honesto: son del mismo cambio, no se oculta nada).

## Estrategia de rollback

Revertir el PR restaura la detección actual. Sin migración, sin cambio de datos: las guardas sólo evitan **crear** relaciones nuevas. Las relaciones ya creadas siguen siendo relaciones normales.

## Riesgos

- **G2 podría tapar una contradicción real entre un commit y un spec.** Mitigación: no es un riesgo real — el veredicto que una relación habilita es `supersedes`/duplicado, y **un commit no puede reemplazar a un spec** (son artefactos de distinta naturaleza: el evento vs. el contrato). Si un commit contradice su spec, eso es un bug de código, no una contradicción de memoria — y no se arregla ocultando una de las dos.
- **G1 podría tapar una contradicción entre dos fases del mismo cambio** (ej. el `design` contradice al `spec`). Es un riesgo **real pero deliberado**: dentro de un cambio, la fase posterior **refina** a la anterior por diseño, y las 7 quedan como registro histórico del razonamiento. Ocultar una fase por parecerse a otra sería **perder el rastro**, que es justo lo que el flujo SDD existe para conservar.
- **Falsos negativos por un `topic_key` mal formado.** Si el prefijo no matchea `sdd/<cambio>/<fase>`, la guarda no aplica y el par cae en el camino normal (degradación segura: se comporta como hoy).
