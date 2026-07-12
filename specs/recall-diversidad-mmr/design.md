# Design — recall-diversidad-mmr

## D1 — MMR se inserta entre el scoring y el empaquetado

```go
scored := scoreCandidates(cands, lexRank, vecRank, graphRank, coocRank, time.Now().UTC())
scored = e.diversify(scored, opts.MMRLambda)   // <-- acá
result = packByBudget(scored, budget, gistMax)
```

**Rationale.** `scoreCandidates` responde *«¿qué tan relevante es cada item?»*. `packByBudget` responde *«¿cuántos entran?»*. **Falta la pregunta del medio: «¿qué tan útil es el CONJUNTO?»** — y ese es exactamente el hueco donde vive la redundancia. Poniendo MMR ahí, ni el scoring ni el empaquetado se enteran: reciben lo mismo que siempre, sólo que en **otro orden**.

`scoreCandidates` sigue siendo **pura y determinista** (no se toca). `diversify` es un método del engine sólo porque necesita **leer vectores**.

## D2 — La penalización mide REDUNDANCIA, no similitud

**Es la decisión central, y donde este track ya se tropezó una vez.**

```go
// redundancyBase: MEDIDO sobre 94.830 pares reales — dos memorias CUALESQUIERA del corpus tienen
// coseno mediano 0.60. Ése es el "piso del idioma", no redundancia: es lo que se parecen dos textos
// del mismo dominio por el solo hecho de estar en español y hablar de software.
const redundancyBase = 0.60

// redundancy mapea el coseno a [0,1]: 0 en la línea de base, 1 en el duplicado exacto.
func redundancy(cos float64) float64 {
    if cos <= redundancyBase {
        return 0
    }
    return (cos - redundancyBase) / (1 - redundancyBase)
}
```

**Rationale — por qué el coseno CRUDO no sirve.** Dos problemas, y los dos son fatales:

1. **Escalas incompatibles.** El score RRF vive en **~0.05–0.11**; el coseno, en **0.60–0.99**. En `λ·rel − (1−λ)·cos`, la penalización es **10× más grande** que la señal a la que se resta: con cualquier λ razonable, MMR dejaría de ser un ajuste y pasaría a **ser** el ranking.
2. **Todo se parece a todo.** Con base 0.60, *cada* par del corpus arrastra ~0.6 de penalización. Penalizar sobre eso es castigar a los items **por estar escritos en el mismo idioma**.

Reescalando, un coseno de **0.98** (dos fases del mismo cambio) da redundancia **0.95**, y uno de **0.62** (dos memorias ajenas) da **0.05**. **La penalización muerde donde hay redundancia real y calla donde no.**

Y la relevancia se normaliza **min-max** sobre las candidatas ⇒ ambas en `[0,1]`, comparables. Sin eso, λ no sería un dial: sería un número mágico distinto para cada consulta.

**Descartado — penalizar por `topic_key` (mismo cambio SDD).** Sería más barato y atraparía el caso que motivó todo esto. Pero es **demasiado específico**: sólo ve la redundancia que *ya* sabemos nombrar. Dos memorias redundantes con `topic_key` distinto —el caso general— seguirían pasando. El coseno **no necesita saber** que existe el flujo SDD.

## D3 — `λ >= 1` apaga MMR, y se nota en el CÓDIGO

```go
func (e *DbEngine) diversify(scored []scoredCandidate, lambda float64) []scoredCandidate {
    if lambda >= 1 || len(scored) < 2 {
        return scored          // bit-idéntico: ni se leen los vectores
    }
    ...
}
```

**Rationale.** El rollback no es *«poné λ alto y andá viendo»*: con `λ >= 1` la función **devuelve la entrada sin tocarla** y ni siquiera consulta la base. El comportamiento anterior es **inalcanzablemente idéntico**, no aproximadamente idéntico.

## D4 — Sin vector, sin castigo

Un item sin embedding recibe redundancia **0** y compite por **relevancia pura**.

**Rationale.** La alternativa —tratar «sin vector» como «redundante»— **enterraría** memoria por una razón que **no tiene nada que ver con su contenido** (que el backfill todavía no la alcanzó). Es la misma degradación segura que rige todo el track: **ante la duda, no castigues.**

## D5 — El costo: una consulta, no N²

Los vectores de las candidatas se leen **una vez** (`vectorsFor(ids)`, el mismo patrón de `candidateCosines`). El bucle MMR es `O(k·n)` sobre **cosenos en memoria** — sin base, sin red. Con `n` ≈ pocas decenas de candidatas, es ruido frente al FTS y al vector search que ya corrieron.

## D6 — λ NO se estima: se MIDE contra el gate

El CI ya tiene el **`recall-gate`**: R@10 ≥ 0.80 sobre el fixture dorado con la tabla POTION real.

**La diversidad canjea relevancia por cobertura, así que el gate es la vara.** El default de λ sale de **correrlo**, no de elegir un número lindo.

> **Y el compromiso incómodo, dicho de antemano:** si el único λ que mantiene el gate verde es uno que vuelve a MMR **inocuo**, entonces **la feature no se justifica** — y hay que decirlo y tirar el PR, no bajar la vara para que pase.

## Lo que este diseño NO hace

- **No descarta nada.** MMR **reordena**. Un item redundante baja; si el presupuesto alcanza, sigue estando.
- **No toca las 7 señales**, ni el RRF, ni los rangos densos, ni los umbrales, ni el empaquetado.
- **No sabe qué es un cambio SDD.** Ataca la **redundancia**, no un caso particular de ella.
