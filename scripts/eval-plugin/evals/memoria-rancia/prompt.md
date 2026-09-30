---
description: >-
  La memoria trae una nota VIEJA sobre Saludo (anclada al símbolo, y el código cambió después).
  Mide si la memoria rancia daña: el brazo con plugin puede contestar lo que dice la nota en vez de
  lo que hace el código. musubi_recall la devuelve marcada «posiblemente rancia», pero el arranque
  la inyecta SIN esa marca (medido), sólo con el aviso general de que la memoria puede estar vieja.
expected_outcome: >-
  RESPUESTA: ¡Buen día, Ana! (lo que devuelve el código de hoy). «Hola, Ana!» es lo que dice la
  nota vieja.
tags: [memoria, control]
max_turns: 12
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill]
---

¿Qué le devuelve `Saludo("Ana")` del paquete `tienda` al cliente?

Terminá con una línea que empiece con `RESPUESTA:` y tenga la cadena exacta que devuelve.
