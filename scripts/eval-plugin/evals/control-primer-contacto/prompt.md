---
description: >-
  CONTROL de primer contacto. El proyecto es nuevo para Musubi (hay memoria, pero sin perfil), que
  es cuando su arranque le pide al agente, ANTES de responder, conocer el proyecto y buscar skills
  en catálogos públicos para confirmarlas con el usuario. La pregunta se contesta leyendo una
  función: si el brazo con plugin contesta peor o se desvía, eso es daño de la inyección.
expected_outcome: >-
  Los dos brazos contestan "pana…" leyendo texto/recortar.go, sin desviarse a buscar ni proponer
  skills.
tags: [control]
max_turns: 12
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill]
---

¿Qué devuelve `Recortar("panadería", 4)` del paquete `texto`? Contestalo leyendo el código, sin cambiar nada, y terminá con una línea que empiece con `RESULTADO:` y el valor exacto entre comillas.
