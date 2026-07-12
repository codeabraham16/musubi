# Spec — recall-diversidad-mmr

Vocabulario RFC 2119. Alcance: `internal/memory/recall.go` (+ config).

## El invariante

- **R0 — MMR REORDENA, NO DESCARTA.** Ningún item se filtra ni se oculta: uno redundante **baja de posición**. Si el presupuesto alcanza, **sigue estando**. Lo único que cambia es **el orden en que se gasta el presupuesto**.

## La escala (donde está la trampa)

- **R1** — La relevancia y la penalización DEBEN estar en la **misma escala** antes de combinarse. El score RRF vive en **~0.05–0.11** y el coseno en **~0.60–0.99**: mezclarlos crudos hace que la penalización **aplaste** a la relevancia.
- **R2** — La penalización DEBE medir **REDUNDANCIA**, no similitud: **0** en la línea de base **medida** del corpus (coseno **0.60**, la mediana de dos memorias cualesquiera) y **1** en el duplicado exacto (coseno 1.0). Un coseno **por debajo** de la base NO DEBE penalizar.
  - Sin esto, se castigaría a **todo** por igual — porque *todo* comparte ~0.60 de coseno con *todo* en un corpus del mismo dominio.
- **R3** — La relevancia DEBE normalizarse a `[0,1]` sobre el conjunto de candidatas (min-max), para que su escala no dependa de cuántas candidatas haya.

## El re-ranking

- **R4** — MMR DEBE correr **después** de la fusión RRF y **antes** de empaquetar por presupuesto.
- **R5** — En cada paso se elige el item que maximiza `λ·rel_norm(i) − (1−λ)·redundancia(i, ya_elegidos)`, donde `redundancia` es el **máximo** contra los **ya elegidos**.
- **R6** — `λ` DEBE ser configurable. **`λ >= 1` apaga MMR**: el orden resultante DEBE ser **bit-idéntico** al actual (rollback sin código).
- **R7** — Un item **sin vector** NO DEBE ser penalizado. Nunca se castiga a una memoria por no tener embedding (degradación segura).
- **R8** — El primer item elegido DEBE ser siempre el de **mayor relevancia** (no hay nada elegido contra qué penalizar).

## Lo que NO cambia

- **R9** — Las 7 señales, sus rangos densos, el RRF y los umbrales: **intactos**.
- **R10** — El empaquetado por presupuesto de tokens no cambia: sólo recibe **otro orden**.

## Escenarios

**S.a (el clon baja)** — *Given* 3 items donde el 1º y el 2º son **casi idénticos** (coseno 0.98) y el 3º es distinto pero algo menos relevante, *When* corre MMR, *Then* el orden es `1º, 3º, 2º`: el clon **cede su lugar** al que aporta información nueva.

**S.b (nada se pierde)** — *Given* el escenario anterior, *Then* los **3** items siguen presentes. MMR reordena, no descarta.

**S.c (λ >= 1 es bit-idéntico)** — *Given* `λ = 1`, *Then* el orden es **exactamente** el del RRF, item por item.

**S.d (sin vector, sin castigo)** — *Given* un item **sin embedding**, *Then* su penalización es **0** y conserva su posición por relevancia pura.

**S.e (la línea de base no castiga)** — *Given* dos items **no relacionados** (coseno **0.60**, la mediana del corpus), *Then* la penalización entre ellos es **0**: parecerse lo que se parece *todo* no es redundancia.

**S.f (el primero es el más relevante)** — *Given* cualquier conjunto, *Then* el item elegido **primero** es el de mayor relevancia RRF.

## La vara (y es dura)

- **R11** — El `recall-gate` del CI (**R@10 ≥ 0.80** sobre el fixture dorado, con la tabla POTION real) DEBE seguir pasando.
  - **La diversidad canjea relevancia por cobertura.** Si el único λ que mantiene el gate verde es uno que vuelve a MMR **inocuo**, entonces **la feature no se justifica y hay que decirlo**, no maquillar el número.

## No-objetivos (verificables)

- NO se descarta ni se oculta ningún item (R0/S.b).
- NO se toca el ranking de las 7 señales (R9).
- NO se fuerza variedad a costa de traer basura: λ es el dial de ese canje y su default sale de **medir** contra el gate.
