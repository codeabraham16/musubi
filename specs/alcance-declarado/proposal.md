# Propuesta — El porqué del `*` se vuelve alcance

Track: **Forja global**, F1. Sale directo de `specs/forja-global/investigacion.md`.

## El hallazgo que ordena esto

La investigación empezó creyendo que faltaba vocabulario para expresar alcance. Medir mostró dos
cosas, y la segunda corrige a la primera:

1. **La pregunta exige archivos.** `musubi_resolve_skills` **rechaza** una lista vacía, así que una
   skill que no habla de archivos sólo puede declarar `triggers: ['*']`.
2. **★ Pero el «cuándo» ya está escrito.** Las 6 skills con `*` lo dicen en `always_because`, el
   campo que `trigger-honesto` creó para justificar el comodín ante un humano:

| skill | lo que ya dice | eje |
|---|---|---|
| `plan-ahead` | «planificar es una **FASE** del trabajo» | fase |
| `sdd-flow` | «gobierna el **flujo completo de un cambio**» | fase |
| `adversarial-review` | «se activa **al cerrar un cambio de riesgo**; el disparador es el **momento**» | fase |
| `orchestrate-multiagent` | «se activa por la **FORMA de la tarea** (grande y paralelizable)» | tarea |
| `audit-structure-flow` | «el disparador es el **pedido de auditoría**» | tarea |
| `project-profile` | «describe el **proyecto entero**; no hay archivo al que atarlo» | ninguno |

**Seis autores, por separado, escribieron qué eje necesitaban, y caen en dos.** No hay taxonomía que
inventar: hay una que leer. Este spec no diseña un vocabulario — lo **transcribe** desde el único
corpus que existe, y por eso empieza chico y con evidencia detrás de cada valor.

## Lo que se construye

Un campo `applies_to` con vocabulario cerrado, que el resolvedor **sí** mira, y una pregunta que
acepta que el llamador **declare qué está haciendo** en vez de exigirle un archivo.

## La restricción que manda: sin inferencia

El README declara el núcleo **model-free y determinista**; la cognición es el tercer pilar y es
opt-in. Un resolvedor que le pida a un modelo que juzgue qué skill aplica está descartado **por
principio, no por costo**.

La salida no es resignarse: **el modelo no necesita juzgar, necesita declarar**. Un agente que dice
«estoy planificando» convierte la fase en un campo más para matchear con igualdad de strings. El
agente ya sabe en qué fase está; hoy no tiene dónde decirlo. Cero llamadas al modelo.

## Aditivo, y esto es deliberado

`applies_to` **agrega** una forma de matchear; no quita ninguna. Las 5 skills que lo reciben
conservan su `triggers: ['*']`.

La tentación es migrarlas y sacarles el comodín —«ahora sí están bien declaradas»—. Sería un error
en este slice: si ningún llamador declara todavía su fase, esas skills dejarían de activarse y el
arreglo se vería como una regresión. Primero existe el canal, después se aprieta.

## Lo que NO se construye

- **Progressive disclosure.** Es el problema 3 de la investigación y tiene su propio spec. Ojo con
  una interacción ya detectada: el nivel 1 (`name`+`description`) **no** contiene el «cuándo» de
  estas 6 skills, que vive en `always_because`.
- **Medir si una skill sirvió.** Problema 4, sin resolver.
- **Un eje para el juicio** («aplica cuando el código huele a X»). Eso necesita un modelo; si algún
  día se quiere, será opt-in como la cognición.
- **No se toca `project-profile`.** Es genuinamente de proyecto entero y ningún valor del
  vocabulario le queda: forzarlo sería inventar el eje que este spec dice no inventar.
