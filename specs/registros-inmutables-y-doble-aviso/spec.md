# Spec — registros-inmutables-y-doble-aviso

Vocabulario RFC 2119. Alcance: `internal/memory/conflicts.go`, `internal/memory/band.go`.

## F1 — La banda es el COMPLEMENTO de la cola

- **R1** — La banda NO DEBE mostrar ningún par que la cola vaya a proponer como relación. La condición DEBE ser la **misma** que decide la entrada a la cola (`relevantPair`), no una **proxy** sobre una sola de sus señales.
- **R2** — La banda sigue exigiendo coseno dentro de `[BandFloor, CosineFloor)` (R1 **agrega** una exclusión, no reemplaza el rango).

**Por qué importa:** a la cola se entra por **dos puertas** (léxico **O** coseno). Filtrar por el **coseno solo** deja pasar los que entraron por la **léxica** — y ése es exactamente el doble aviso que se observó en producción.

## F2 — Un registro histórico nunca es DESTINO de otra clase

- **R3** — Un `git-commit` y un contrato `sdd/*` son **registros históricos**: lo que **pasó** y lo que se **acordó**. NO DEBEN ser el **target** de una relación cuyo **source** sea de **otra clase**.
- **R4** — La regla DEBE ser **ASIMÉTRICA**: bloquea `X → registro` (con X de otra clase), pero NO bloquea `registro → X`.
- **R5** — La regla NO DEBE aplicarse entre artefactos de la **misma clase**: `commit → commit` y `sdd → sdd` (de cambios **distintos**) siguen detectándose.
- **R6** — Una memoria común (ni commit ni SDD) SÍ PUEDE ser target de cualquiera: no es un registro histórico, puede envejecer.

## Lo que NO cambia

- **R7** — Piso de coseno, AND-gate, gate de novedad, `band_floor` y las guardas de #203: **intactos**.
- **R8** — Ninguna guarda oculta memoria. **Evitan CREAR** una relación; no archivan ni marcan `superseded`.

## Escenarios

**S.a (sin doble aviso)** — *Given* un par con **léxico alto** (entra a la cola) y **coseno 0.82** (cae en la banda), *When* se guarda, *Then* aparece **sólo** como relación `pending`, y **NO** además como vecino de banda.

**S.b (la banda sigue viendo lo suyo)** — *Given* un par con **léxico bajo** (NO entra a la cola) y **coseno 0.82**, *Then* **SÍ** aparece como vecino de banda. (F1 no puede vaciar la banda de lo que era su razón de ser.)

**S.c (nota → commit: bloqueado)** — *Given* un `git-commit` guardado, *When* se guarda una **nota** muy parecida, *Then* NO se propone relación: ninguna nota puede reemplazar a lo que pasó.

**S.d (nota → SDD: bloqueado)** — Ídem con un contrato `sdd/<cambio>/<fase>` como target.

**S.e (commit → nota: SE CONSERVA)** — *Given* una **nota** que dice *«usamos X»*, *When* se guarda un `git-commit` *«feat: migrar de X a Y»* parecido, *Then* la relación **SÍ** se propone: el commit es **evidencia** de que la nota envejeció. **Éste es el test que prueba que la regla es asimétrica y no un martillo.**

**S.f (commit → commit: SE CONSERVA)** — Dos commits parecidos ⇒ la relación **sí** se propone (misma clase).

**S.g (SDD → SDD de cambios distintos: SE CONSERVA)** — `sdd/a/design` vs `sdd/b/design` ⇒ la relación **sí** se propone (misma clase).

**S.h (nada se oculta)** — En cualquiera de los anteriores, **ninguna** observación queda `superseded` ni `archived`.

## No-objetivos (verificables)

- NO se reduce la detección entre artefactos de la **misma** clase (S.f, S.g).
- NO se pierde la dirección **útil** de la relación con registros históricos (S.e).
- NO se vacía la banda de su razón de ser (S.b).
