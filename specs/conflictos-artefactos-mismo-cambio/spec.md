# Spec — conflictos-artefactos-mismo-cambio

Vocabulario RFC 2119. Alcance: `internal/memory/conflicts.go`.

## El invariante

- **R0 — Una guarda NUNCA oculta memoria.** Evita **crear** una relación; no archiva, no marca `superseded`, no borra. El peor caso de un falso negativo es **una relación de menos en la cola**, jamás una observación de menos en el recall.

## G1 — Hermanos del mismo cambio SDD

- **R1** — Si el `topic_key` de **ambas** observaciones tiene la forma `sdd/<cambio>/<fase>` y el `<cambio>` es el **mismo**, NO se DEBE proponer relación alguna (ni `pending` ni automática).
- **R2** — La comparación del `<cambio>` DEBE ser exacta: dos cambios **distintos** (`sdd/foo/design` vs `sdd/bar/design`) NO caen en la guarda y siguen el camino normal.
- **R3** — Un `topic_key` que NO matchea la forma `sdd/<cambio>/<fase>` NO activa la guarda (degradación segura: se comporta como hoy).

## G2 — Clases de artefacto distintas

- **R4** — Un `git-commit` y un contrato `sdd/*` NUNCA DEBEN producir una relación entre sí, **cualquiera sea el cambio**. Son artefactos de naturaleza distinta: el **evento** vs. el **contrato**. Ninguno puede reemplazar al otro.
- **R5** — G2 NO DEBE afectar la detección entre dos `git-commit` (ahí el parecido **sí** puede ser redundancia ⇒ sigue yendo a `pending`).
- **R6** — G2 NO DEBE afectar la detección entre un `git-commit` (o un `sdd/*`) y una memoria **común** (una nota, un roadmap): ese par sigue el camino normal.

## Lo que NO cambia

- **R7** — El umbral de coseno, el AND-gate (#193) y el gate de novedad (#195) NO se tocan. Todo lo que **sí** requiere juicio sigue yendo a `pending`.
- **R8** — La detección entre memorias de la **misma** clase (dos notas, dos commits) NO se toca.
- **R9** — NO se hace limpieza retroactiva en el código.

## Escenarios

**S.a (hermanos SDD: sin relación)** — *Given* `sdd/mi-cambio/spec` ya guardado, *When* se guarda `sdd/mi-cambio/design` con contenido muy parecido, *Then* NO se crea **ninguna** relación entre ambos.

**S.b (cambios distintos: sí se detecta)** — *Given* `sdd/cambio-a/design`, *When* se guarda `sdd/cambio-b/design` muy parecido, *Then* la relación **sí** se propone (`pending`): son cambios distintos y el parecido **puede** ser significativo.

**S.c (commit vs su SDD: sin relación)** — *Given* `sdd/mi-cambio/proposal`, *When* se guarda un `git-commit` cuyo mensaje describe ese mismo cambio (coseno alto), *Then* NO se crea relación.

**S.d (commit vs commit: sigue detectando)** — *Given* un `git-commit`, *When* se guarda otro `git-commit` muy parecido, *Then* la relación **sí** se propone (`pending`). G2 no lo tapa.

**S.e (commit vs nota: sigue detectando)** — *Given* una memoria común muy parecida, *When* se guarda un `git-commit`, *Then* la relación **sí** se propone.

**S.f (ninguna guarda oculta nada)** — *Given* cualquiera de los escenarios anteriores, *Then* **ninguna** observación queda `archived` ni `superseded`, y las dos siguen apareciendo en el recall.

## No-objetivos (verificables)

- NO se oculta ni archiva ninguna observación (R0/S.f).
- NO se reduce la detección entre memorias comparables (S.b, S.d, S.e).
