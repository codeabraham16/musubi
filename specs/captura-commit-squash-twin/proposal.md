# Proposal — captura-commit-squash-twin

## Intención

**Cada PR mergeado con squash deja DOS memorias del mismo commit.** Encontrado en la memoria real del dogfood, no en teoría:

| | |
|---|---|
| `13:30:08` | `feat(dedup): dedup semantico -- el duplicado ...` ← commit en la **rama** |
| `13:39:58` | `feat(dedup): dedup semantico -- el duplicado ... **(#193)**` ← el **mismo**, tras el squash-merge |

`musubi capture` corre sobre la rama y guarda el commit. Después el squash-merge crea en `main` un commit **nuevo** con el **mismo mensaje** más el sufijo `(#NNN)` (y GitHub reescribe el trailer `Co-Authored-By` a `Co-authored-by`). La captura lo ve como **nuevo** y lo guarda otra vez.

**El dedup por hash exacto no lo agarra**, porque el texto cambió apenas. Y es **redundante por construcción**: tras un squash, el commit de la rama **ya no existe** en la historia de `main`. El canónico es el del merge.

Medido en la memoria real: **3 pares** (6 observaciones, 3 redundantes) sobre 58 commits. Ya se limpiaron a mano; esto **cierra la fuente**.

## Por qué no alcanza con M4

Con M4 (#195), estos gemelos ahora quedan marcados como **`pending`** para que los juzgue el agente. Eso está bien como red de seguridad, **pero es ruido recurrente**: te pediría un veredicto en **cada PR**, para un caso que es **deducible sin juicio**. Model-free puede resolver esto solo, sin molestar a nadie.

## La clave: no es un "duplicado semántico", es el MISMO commit reformulado

Esta distinción decide el diseño:

- Un **duplicado semántico** (otras palabras, mismo significado) **requiere juicio** ⇒ `pending`, lo decide el agente. Eso es #193/#195.
- Un **gemelo de squash** es el **mismo commit**, con el mismo cuerpo y **los mismos archivos**, reformulado mecánicamente por GitHub. Es un hecho **estructural**, no una interpretación.

Por eso acá **sí** se puede resolver automáticamente — igual que el dedup por hash exacto, que también es un NOOP seguro ("contenido idéntico **es** lo mismo").

## Alcance

- **Id determinístico** para las observaciones de commit, derivado de una **clave normalizada** del contenido (sufijo `(#NNN)` del subject removido; clave insensible a mayúsculas para absorber el reescrito del trailer).
- Como el id **es** la clave de dedup, el gemelo del squash cae en el **mismo id** ⇒ **UPSERT** de la observación existente con el contenido **canónico** (el del merge, que trae el `(#NNN)`).
- **Resultado: UNA observación, con el contenido canónico.** No se oculta nada, no se descarta nada, no se pierde memoria — **se actualiza a su forma canónica.**
- Sin cambio de esquema. Las observaciones de commit ya guardadas (con UUID) **no se tocan**: la captura sólo procesa commits **nuevos**.

## Fuera de alcance (explícito)

- NO se toca el dedup **semántico** (#193) ni el gate de novedad (#195): siguen siendo la red de seguridad para todo lo que **sí** requiere juicio.
- NO se toca `classifyCommit`, ni qué commits se capturan, ni las otras superficies de memoria.
- NO se hace limpieza retroactiva en el código (los 3 pares existentes ya se limpiaron a mano).

## Estrategia de rollback

Revertir el PR restaura el id UUID + dedup por hash exacto. Sin migración de esquema, sin cambio de datos. Las observaciones con id determinístico siguen siendo observaciones normales.

## Riesgos

- **Colisión de clave entre commits genuinamente distintos.** Dos commits con el **mismo subject, mismo body y mismos archivos** caerían en el mismo id ⇒ el segundo **sobrescribiría** al primero. Mitigación fuerte: el contenido capturado **incluye la lista de archivos**, así que dos commits distintos de verdad casi siempre difieren. Y si subject + body + archivos coinciden **exactamente**, es discutible que sean memorias distintas. Aun así hay que **decirlo**, no esconderlo.
- El UPSERT preserva `created_at` y las stats de acceso (es el comportamiento de `SaveObservationTyped`), así que el gemelo del merge **no resetea** la antigüedad del original. Hay que verificarlo con un test.
- Si un PR tiene **varios** commits, el squash genera un mensaje **distinto** (título del PR + lista) ⇒ **no** es un gemelo y se guarda aparte, que es lo correcto. El fix aplica al caso de **un commit por PR**.
