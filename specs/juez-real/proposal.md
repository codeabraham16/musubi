# Propuesta — El juez, contra el motor real

Track: **Potencia medida**, cierre de F2. Es el último eslabón de tres specs que sólo juntos
responden una pregunta:

| spec | qué dejó |
|---|---|
| `motor-sin-candado` | el motor puede tardar sin trabar la casa |
| `juez-medible` | el banco puede llamar al juez **de producción**, no a una copia |
| `fixture-real` | el banco mide sobre 1.216 docs, no sobre 26 |
| **éste** | **el número** |

Los tres anteriores construyeron el instrumento. Ninguno contestó la pregunta que abrió el track:
**¿el juez de pertinencia aporta, y cuánto?**

## Por qué no hay `spec.md`

Los tres specs anteriores tienen invariantes con pruebas que saben fallar, porque agregan
**comportamiento**. Éste no agrega comportamiento: agrega una **medición**, y una medición no se
verifica con un test, se verifica corriéndola y mirando si el número es coherente.

Escribir un `spec.md` acá sería ceremonia. Lo que sí hace falta —y va en `tasks.md`— es decir qué
prueba el número y, sobre todo, **qué no prueba**.

## Lo que se construye

Un solo test, `TestMedicionJuezReal`, guardado por env vars y por lo tanto invisible para CI: corre
los dos brazos —model-free y model-free+juez— sobre el **mismo corpus, en la misma corrida**, y
imprime el delta aparte de la tabla.

El delta va aparte a propósito: restar dos filas a ojo es exactamente donde alguien se equivoca de
signo y saca la conclusión al revés.

## Lo que NO se construye

- **El juez sigue apagado en producción.** Este spec entrega el número; encender
  `read_time_rerank` es decisión del dueño, y ahora tiene con qué decidir.
- **No corre en CI, nunca.** Gastar cuota de una suscripción en cada push sería una forma cara de no
  aprender nada.
- **No hay presupuesto ni autorización por principal.** Sigue siendo el paso que falta, y este número
  lo vuelve **más** urgente: hace atractivo encender algo que hoy cualquier `writer` puede gastar.
