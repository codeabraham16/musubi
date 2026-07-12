# Spec — captura-commit-squash-twin

Vocabulario RFC 2119. Alcance: `cmd/musubi/capture.go`.

## El invariante

- **R0 — Ninguna memoria se pierde ni se oculta.** El gemelo del squash NO se descarta ni se marca `superseded`: **actualiza** (UPSERT) la observación existente a su forma **canónica**. El resultado es **una** observación, con el contenido del merge.
  - Es un NOOP **seguro** por la misma razón que el dedup por hash exacto: no es una interpretación, es un **hecho estructural** (el mismo commit, reformulado mecánicamente).

## La clave de dedup

- **R1** — El id de una observación de commit DEBE derivarse **determinísticamente** de una **clave normalizada** de su contenido.
- **R2** — La normalización DEBE quitar el sufijo `(#NNN)` que el squash-merge le agrega al **subject** (sólo del subject, no del cuerpo).
- **R3** — La normalización DEBE ser **insensible a mayúsculas**, para absorber el reescrito del trailer (`Co-Authored-By` → `Co-authored-by`).
- **R4** — La clave DEBE incluir el **cuerpo** y la **lista de archivos**, no sólo el subject: es lo que evita que dos commits genuinamente distintos con el mismo título colisionen.

## El comportamiento

- **R5** — Si el id ya existe, el commit DEBE **actualizar** esa observación con el contenido nuevo (el canónico, con `(#NNN)`), y NO DEBE crear una observación nueva.
- **R6** — Un UPSERT DEBE preservar `created_at` y las estadísticas de acceso de la observación original (el gemelo del merge **no** debe resetear su antigüedad).
- **R7** — El **gate de novedad (M4)** NO DEBE correr sobre un UPSERT: no hay memoria nueva que relacionar. (Igual que hoy no corre sobre un dedup por hash exacto.)
- **R8** — El contador de commits capturados NO DEBE contar el UPSERT como un guardado nuevo.

## Lo que NO cambia

- **R9** — El dedup **semántico** (#193) y el gate de novedad (#195) NO se tocan: siguen siendo la red de seguridad de todo lo que **sí** requiere juicio.
- **R10** — Un PR con **varios** commits produce un mensaje de squash **distinto** (título del PR) ⇒ **no** es un gemelo ⇒ se guarda como observación aparte. Correcto y esperado.
- **R11** — Las observaciones de commit ya guardadas (con id UUID) NO se tocan: la captura sólo procesa commits **nuevos**.

**Escenario S.a (el gemelo NO duplica)** — *Given* un commit ya capturado desde la rama, *When* llega su gemelo del squash-merge (mismo mensaje + `(#123)`), *Then* NO se crea una observación nueva: se **actualiza** la existente, y el total de observaciones de commit **no sube**.

**Escenario S.b (el contenido queda canónico)** — *Given* el escenario anterior, *When* se lee la observación, *Then* su contenido es el del **merge** (el que trae `(#123)`).

**Escenario S.c (no se resetea la antigüedad)** — *Given* el escenario anterior, *Then* el `created_at` de la observación sigue siendo el del commit **original**.

**Escenario S.d (commits distintos NO colisionan)** — *Given* dos commits con el **mismo subject** pero **archivos distintos**, *When* se capturan, *Then* producen **dos** observaciones (la lista de archivos entra en la clave).

**Escenario S.e (el trailer reescrito no rompe el match)** — *Given* un commit con `Co-Authored-By:` y su gemelo con `Co-authored-by:`, *Then* caen en la **misma** clave.

## No-objetivos (verificables)

- NO se oculta (`superseded`) ni se descarta ninguna observación.
- NO se hace limpieza retroactiva de los duplicados ya existentes.
