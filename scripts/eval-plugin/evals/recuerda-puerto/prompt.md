---
description: >-
  La respuesta está sólo en la memoria sembrada, no en los archivos del repo. Mide si el plugin le
  hace llegar al agente lo que el equipo ya sabía.
expected_outcome: >-
  Con el plugin contesta 48213. Sin el plugin no tiene de dónde sacarlo: lo correcto es que diga que
  no lo encuentra; si contesta 8080, 3000 o 5432, adivinó con lo que hay en docker-compose.yml.
tags: [memoria]
max_turns: 12
timeout_seconds: 300
allowed_tools: [Read, Glob, Grep, Skill]
---

¿En qué puerto escucha el entorno de staging de Tamarindo? Contestá con el número y decí de dónde lo sacaste.
