# Proposal — recall-importance-rrf

## Intención

Corregir el defecto **Q3** de la fusión del recall (auditoría del techo semántico): en `scoreCandidates` (`internal/memory/recall.go:297`) el score final es `rrf * importance`, un multiplicador **sin techo**. Con `importance` hasta 10, una memoria apenas relevante puede barrer matches semánticamente mucho mejores: un `importance:10` con la peor posición de todos los pools gana igual. Es un multiplicador que **anula** la relevancia en vez de **desempatarla**.

Fix 100% model-free: **plegar `importance` como un término RRF propio** (uno más, junto a recencia/frecuencia/léxico/vector/grafo/co-ocurrencia), acotado a la misma escala `1/(rrfK+rank)`. Así la importancia pasa de *override* a *nudge/desempate*: cuando la relevancia es comparable, la importancia decide; cuando un candidato es claramente más relevante, gana por relevancia.

## Por qué ahora

Es el siguiente quick-win de la Fase 1 del roadmap Semantic Hardening (después de Q1+Q2, PR #190). La auditoría lo marcó explícito: *"`score=rrf*importance` sin techo → importance:10 barre matches. Plegar importance como término RRF propio."* Cierra el último modo en que una sola señal domina la fusión, alineando la importancia con la filosofía de RRF (ninguna señal aplasta a las otras).

## Alcance

- **Solo `scoreCandidates`**: reemplazar `score = rrf * imp` por un término RRF adicional `1/(rrfK+impRank)` sumado al resto, con `score = rrf`.
- **Ranking de importancia TIE-AWARE (denso)**: candidatos con igual importancia comparten rango. Es el punto fino: con `rankBy` posicional, el caso común (todos en `importance` default = 1.0) generaría rangos distintos arbitrarios → reordenamiento espurio. Con rango denso, todos empatan en rango 0 → término constante → **orden relativo intacto** cuando no hay importancias elevadas.
- Tests: rango denso directo; importancia como desempate (se preserva); importancia que **ya no** anula una relevancia claramente superior (el fix); caso uniforme sin reordenamiento.
- Sin cambio de firma de `scoreCandidates` (la importancia se lee de `candidate.importance`, como recencia/frecuencia): blast radius mínimo.

## Fuera de alcance (explícito)

- Peso configurable/tuneable de la importancia (M7) — acá entra como término RRF plano, sin peso; la ponderación fina es un cambio aparte.
- Calibración empírica de cuánto debe pesar la importancia — se elige la equivalencia RRF estándar (un término más).
- Cualquier cambio a cómo se ASIGNA o persiste `importance` (captura, promote): esto solo toca cómo se **usa** en el ranking.
- Los otros términos RRF (recencia/léxico/vector/grafo/co-ocurrencia) quedan idénticos.

## Estrategia de rollback

- Cambio acotado a una función pura y determinista (`scoreCandidates`) + un helper nuevo (`importanceRank`). Revertir el PR restaura `rrf * imp` por completo.
- Sin config nueva, sin migración de esquema, sin cambio de datos. `score` sigue siendo solo una clave de orden (los consumidores rankean, no asertan magnitud absoluta).

## Riesgos

- Cambia el ranking del recall cuando hay importancias no uniformes → un test que codificaba el comportamiento viejo (`TestScoreCandidatesFusion`: "importance:10 debe ganar pese a la peor posición") **debe reescribirse** para asertar el comportamiento nuevo y correcto. Es el fix, no una regresión.
- `recalleval` asierta propiedades (monotonía, MRR>0, métricas ∈ [0,1]), no valores exactos → no hay golden que regenerar; el término es aditivo y acotado, las propiedades se mantienen.
- Riesgo residual: la importancia queda como desempate "débil" (spread ~1/60 entre rangos). Es intencional y es exactamente el objetivo de Q3 (bounded fusion); la ponderación reforzada, si se mide necesaria, es M7.
