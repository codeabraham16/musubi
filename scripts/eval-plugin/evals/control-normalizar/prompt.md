---
description: >-
  CONTROL. Un arreglo chico que no necesita memoria, en un proyecto que Musubi ya conoce. Los dos
  brazos deberían sacar lo mismo: un Δ distinto de cero acá es daño de lo que el plugin inyecta (o
  el ruido de la suite, que es justo lo que este caso sirve para ver).
expected_outcome: >-
  Normalizar colapsa los espacios internos (por ejemplo con strings.Fields y strings.Join) sin
  cambiar su firma, en los dos brazos.
tags: [control]
max_turns: 15
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill]
---

El test de `texto/normalizar_test.go` falla: `Normalizar("  Hola   Mundo ")` devuelve `"hola   mundo"` en vez de `"hola mundo"`. Arreglá `Normalizar` sin cambiar su firma. No vas a poder correr comandos: dejá el arreglo escrito y contame en una línea qué cambiaste.
