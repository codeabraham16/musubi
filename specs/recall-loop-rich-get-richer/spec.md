# Spec — recall-loop-rich-get-richer

Vocabulario RFC 2119. Alcance: `scoreCandidates` en `internal/memory/recall.go`.

## El invariante

- **R0 — Ninguna señal del RANKING puede ser un acumulador que el propio ranker incrementa sin fuga.** Toda señal endógena (escrita por el ranker) DEBE **decaer** si deja de usarse, de modo que el lazo sea **auto-limitado** y no un runaway. Las señales **exógenas** (que el ranker no puede modificar) NO tienen esta restricción.

## N4.a — La recencia mide NOVEDAD, no "cuándo te lo mostré"

- **R1** — El término RRF de **recencia** DEBE derivar de **`created_at`** (cuándo se escribió la memoria), NO de `last_accessed`.
  - *Razón:* `last_accessed` lo escribe el ranker. Hoy, una memoria vieja mostrada hace 5 minutos supera en recencia a una escrita ayer. La novedad genuina pierde contra la circularidad.
- **R2** — Recuperar una observación NO DEBE, por sí solo, mejorar su rango de **recencia** en el recall siguiente.

## N4.b — La frecuencia mide TASA de uso, no total acumulado

- **R3** — El término RRF de **frecuencia** DEBE derivar de una **tasa** (`access_count / (edad_en_días + 1)`), no del `access_count` crudo.
- **R4** — La tasa DEBE **decaer con el tiempo si la observación deja de usarse**: a igual `access_count`, la más **vieja** DEBE tener menor tasa. Es lo que convierte el acumulador en un integrador **con fuga** (R0).
- **R5** — El denominador DEBE estar **suavizado** (`edad + 1`) para que una observación recién creada (edad ≈ 0) no produzca una tasa que explote.
- **R6** — Una transformación **monótona** del contador (p. ej. `log`) NO alcanza: `freqRank` es un **rango**, y `rank(log(x)) == rank(x)`. La tasa DEBE cambiar el **orden**, no sólo la magnitud.

## Lo que NO cambia

- **R7** — `bumpAccess` NO DEBE cambiar: sigue escribiendo `last_accessed` y `access_count`. Son datos válidos.
- **R8** — `decay.go` (olvido / refuerzo de Ebbinghaus) NO DEBE cambiar. Ahí el uso del acceso es **legítimo** (lo que usás no se olvida) y **no es circular**: el olvido no elige qué mostrar.
- **R9** — Los demás términos RRF (léxico, vector, grafo, co-ocurrencia, importancia) DEBEN quedar bit-idénticos.

**Escenario N4.a** — *Given* una observación **vieja** (creada hace 200 días) que acaba de ser recuperada (`last_accessed` = ahora) y otra **nueva** (creada ayer, nunca accedida), *When* se scorean, *Then* la **nueva** DEBE tener mejor rango de **recencia**. (Hoy gana la vieja: es el bug.)

**Escenario N4.b (fuga)** — *Given* dos observaciones con el **mismo** `access_count`, una creada hace 200 días y otra hace 2, *When* se scorean, *Then* la **joven** DEBE tener mejor rango de **frecuencia** (su tasa de uso es mayor).

**Escenario N4.c (no explota)** — *Given* una observación con edad ≈ 0 y 1 acceso, *When* se computa su tasa, *Then* el valor es finito y razonable (no divide por cero).

**Escenario N4.d (el lazo se corta)** — *Given* una observación recuperada N veces, *When* pasa el tiempo **sin** volver a usarse, *Then* su ventaja de frecuencia **se erosiona** (a diferencia de hoy, donde el `access_count` acumulado no baja **nunca**).

## No-objetivos (verificables)

- NO se elimina `bumpAccess` ni las columnas `last_accessed` / `access_count`.
- NO se toca la retención ni el olvido.
