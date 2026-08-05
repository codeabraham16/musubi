# Propuesta — Cuarentena de escritura y procedencia (F4)

## El problema, en una frase

`musubi_ask` sintetiza texto con un LLM. Nada impide tomar esa respuesta y guardarla con
`musubi_save_observation`, donde **queda en el libro mayor indistinguible de una nota verificada a
mano**. La memoria es el producto; una inferencia disfrazada de hecho la corrompe en silencio.

## Por qué esto va ahora y no después del caché

F2 encendió el router: la potencia ya está prendida. La muralla que impide que el LLM escriba
verdad, no. F3 (caché) hace que el motor se llame *menos*; no cierra ningún agujero. Entre subir la
potencia y cerrar la jaula, primero se cierra la jaula.

## Qué ya existe y NO se reconstruye

El plan [[roadmap/motor-cognicion-y-blindaje-cerebro]] dice que F4 es **cablear, no inventar**. Es
literal — la mitad ya está construida:

| Muralla | Camino de **hechos** (grafo) | Camino de **observaciones** (libro mayor) |
|---|---|---|
| 2 — cuarentena | **YA ESTÁ**: `musubi_propose_facts` entra en cuarentena, no es autoritativo, no aparece en `recall_facts` por default y no invalida nada | **NO EXISTE** |
| 3 — procedencia | **YA ESTÁ**: `source = 'llm-extract:<model>'` | **NO EXISTE** |

Las columnas de `observations` son `id, topic_key, content, created_at, archived_at, mem_type,
scope, project_id, author, sync_seq, gist, content_hash, tokens, last_accessed, access_count,
importance, archived, superseded_by`. **Ninguna dice de dónde salió el contenido.**

Ojo con `author`: existe, pero es la atribución por credencial del Track C5 —*qué persona o máquina*
escribió— no *qué clase de proceso generó el contenido*. Un agente-LLM y una persona escriben con la
misma credencial. No sirve para esto.

## Qué NO es

- **No es un detector de texto generado por IA.** No adivina. La procedencia se **declara en el
  camino de escritura**: entrar por la puerta de cuarentena *es* el sello.
- **No es un juez de veracidad.** No decide si algo es cierto. Decide con cuánta autoridad entra.
- **No toca `promote`.** `musubi_promote` ya significa `local → shared` (memoria híbrida). Es otro
  eje y no se le cambia el significado; mezclarlos sería una trampa para el que venga después.
- **No cambia nada del camino de hoy.** Sin usar las tools nuevas, el comportamiento es bit-idéntico.

## La idea

Dos columnas y una puerta.

```
musubi_save_observation      → provenance='human'      confidence=1.0  quarantined=0  → recall normal
musubi_propose_observation   → provenance='llm:<model>' confidence=c   quarantined=1  → INVISIBLE al recall
                                    │
                                    └── corroborar explícitamente ──→ entra al recall, con el sello puesto
```

La cuarentena no es un flag que el caller pide amablemente: **es la puerta por la que entró**.
`musubi_propose_observation` fuerza el sello y no acepta que le pidan otro. Un LLM no puede
declararse humano porque la tool por la que escribe no tiene ese parámetro.

## Costo y reversibilidad

Una migración con dos columnas y un flag, una tool MCP nueva modelada sobre `propose_facts`, y un
`WHERE quarantined = 0` en el recall. Sin dependencias nuevas, sin red. Las columnas tienen default,
así que una base vieja sigue funcionando sin tocar nada, y sin llamar a la tool nueva no hay una sola
fila en cuarentena.
