# Design — recall-importance-rrf

## Cambio central en `scoreCandidates`

Antes (`recall.go:293-297`):
```go
imp := c.importance
if imp <= 0 { imp = 1.0 }
out[i] = scoredCandidate{candidate: c, score: rrf * imp}
```

Después: la importancia es un término RRF más. Se precalcula un `impRank` (denso) igual que `recencyRank`/`freqRank`, y en el loop se suma `1/(rrfK+impRank[c.id])`; el `score` es el `rrf` acumulado, sin multiplicador.

```go
impRank := importanceRank(cands)   // junto a recencyRank, freqRank
...
if r, ok := coocRank[c.id]; ok { rrf += 1.0 / float64(rrfK+r) }
rrf += 1.0 / float64(rrfK+impRank[c.id])   // Q3: importancia como término RRF acotado
out[i] = scoredCandidate{candidate: c, score: rrf}
```

`impRank` está definido para TODO candidato (no es un pool opcional como lex/vec), así que se indexa directo sin el `if _, ok`.

## Helper `importanceRank` (dense rank) — R2

`rankBy` NO sirve: rompe empates posicionalmente (stable sort ⇒ rangos 0,1,2… distintos aun con importancias iguales). Con importancia uniforme (caso común) eso inyectaría reordenamiento espurio. Se necesita rango **denso**: mismo valor ⇒ mismo rango.

```go
// importanceRank rankea por importancia efectiva DESC con empates DENSOS: candidatos con igual
// importancia comparten rango. A diferencia de rankBy (posicional), los empates no crean orden
// espurio — con importancia uniforme (el caso común) todos caen en rango 0, el término RRF de
// importancia es constante y el orden relativo lo deciden los demás pools (R4). Convierte la
// importancia de multiplicador-override a desempate acotado (Q3).
func importanceRank(cands []candidate) map[string]int {
    ordered := make([]candidate, len(cands))
    copy(ordered, cands)
    sort.SliceStable(ordered, func(i, j int) bool {
        return effectiveImportance(ordered[i]) > effectiveImportance(ordered[j])
    })
    ranks := make(map[string]int, len(ordered))
    rank := 0
    for i, c := range ordered {
        if i > 0 && effectiveImportance(c) < effectiveImportance(ordered[i-1]) {
            rank++
        }
        ranks[c.id] = rank
    }
    return ranks
}

// effectiveImportance normaliza la importancia no seteada (<=0) a 1.0, el default histórico,
// para que empate con la default en el ranking (R3).
func effectiveImportance(c candidate) float64 {
    if c.importance <= 0 { return 1.0 }
    return c.importance
}
```

- **R4 (uniforme):** todos igual ⇒ `rank` nunca incrementa ⇒ todos rango 0 ⇒ término `1/(rrfK+0)` idéntico ⇒ constante aditiva ⇒ orden intacto.
- **R2/R3:** orden por importancia efectiva desc; `<=0`→1.0.
- `sort.SliceStable` mantiene determinismo (empatados quedan en orden de entrada, pero como comparten rango da igual para el score).

## Impacto en tests

- **`TestScoreCandidatesFusion` (recall_test.go:229) — REESCRIBIR.** Hoy asierta "c (importance:10, peor lexRank) debe ganar" = el bug. Se reescribe como `TestScoreCandidatesImportanceNoOverride` (R6/Q3.c): el candidato con mejor relevancia gana pese a menor importancia.
- **`TestRecallImportanceBoost` (recall_test.go:162) — SE MANTIENE.** Igual contenido/relevancia, importancia 1 vs 5 ⇒ el de 5 gana como desempate (R5). Sigue pasando.
- **Nuevos tests unitarios:**
  - `TestImportanceRankDense` (Q3.a): `importanceRank` sobre importancias `[10,5,1,1]` ⇒ rangos `[0,1,2,2]`; incluye un `importance:0` que debe normalizar a 1.0 y empatar.
  - `TestScoreCandidatesImportanceTiebreak` (R5/Q3.b): dos candidatos idénticos en todo salvo importancia ⇒ mayor importancia primero.
  - `TestScoreCandidatesImportanceUniform` (R4/Q3.d): con importancia uniforme, el orden lo fija el pool léxico (candidato con mejor lexRank primero), sin que la importancia lo altere.
- **`recalleval`:** asierta propiedades (monotonía, MRR>0, [0,1]), no valores exactos ⇒ sin cambios; el término es aditivo y acotado.
- **`toolslist.golden.json`:** independiente del scoring ⇒ sin cambios.

## Rangos densos (R8/R9) — lo que la implementación obligó a agregar

El término RRF de importancia por sí solo **no alcanzaba**: los tests fallaron y el diagnóstico fue estructural, no un bug de tipeo.

- `rankBy` rompe empates **posicionalmente** (stable sort ⇒ 0,1,2… aun con valores iguales). Con recencia/frecuencia empatadas, el primero del slice se llevaba un rango 0 inmerecido en VARIOS términos, y ese ruido (varios × `1/(rrfK+i)`) ahogaba al único término de importancia.
- `lexRank`/`coocRank` se construían como `rank[c.id] = i` sobre el orden del resultado FTS ⇒ dos observaciones de **contenido idéntico** (mismo bm25) recibían rangos 0 y 1 por **rowid**. Esa diferencia espuria de un rango tiene exactamente la MISMA magnitud que una brecha de relevancia real de un rango.

Consecuencia: **R5 (importancia desempata) y R6 (importancia no overridea) se vuelven indistinguibles.** Se verificó numéricamente que un peso de importancia que gana el empate de R5 también overridea la brecha chica de R6 (probado con w=2: 'c' con importance:10 volvía a barrer a 'a'). No es cuestión de calibrar: es ruido en el origen.

**Fix:** rangos densos en todos los pools.
- `denseRankBy(cands, less)` — generaliza `rankBy`; tras el sort, `less(prev, cur)` es true sólo si `prev` es estrictamente mejor (en un empate ambos `less` dan false) ⇒ el rango no incrementa ⇒ comparten rango. Se usa para recencia, frecuencia e importancia.
- `ftsSearch` ahora devuelve también el **score bm25** (`SELECT rank`, slice paralelo). `recallCandidates` y `augmentWithCooccurrencePool` arman `lexRank`/`coocRank` densos por score (`scores[i] != scores[i-1]` ⇒ `rank++`).

Con esto: relevancia idéntica ⇒ **mismo rango** ⇒ empate REAL ⇒ la importancia desempata limpio. Brecha de ≥1 rango ⇒ señal genuina ⇒ la relevancia manda. Sin constantes mágicas ni pesos.

**Impacto en producción:** bajo. bm25 sólo empata exacto en contenido (casi) idéntico; recencia rara vez empata (timestamps). Donde antes había orden arbitrario por rowid/slice, ahora hay un empate honesto resuelto por las otras señales. Es una mejora de determinismo, no un cambio de ranking para candidatos de relevancia distinta.

## Alternativas descartadas

- **Multiplicador acotado** (`1 + w·log(importance)`): reintroduce un peso arbitrario `w` a calibrar; la auditoría pidió término RRF, que es escala-consistente con los otros pools por construcción.
- **`rankBy` posicional para importancia:** viola R4 (reordenamiento espurio con importancia uniforme). Por eso el helper denso dedicado.
- **Ponderar el término de importancia** (p. ej. `2/(rrfK+rank)`): PROBADO Y DESCARTADO con números. Con w=2 la importancia gana el empate de R5 pero vuelve a overridear la brecha chica de R6 ('c' importance:10 barría a 'a' de nuevo). Cualquier w que satisfaga uno viola el otro **mientras exista el ruido posicional**; una vez densificados los rangos, no hace falta ningún peso. (Los pesos tuneables siguen siendo M7, fuera de alcance.)
- **Multiplicador comprimido/saturante** (`1 + k·log(imp)`, factor acotado a ~1.1): misma tensión — el ratio del empate (~1.000x) y el de la brecha chica (~1.011x) quedan tan cerca que sólo una constante frágil los separa. Descartado por no ser defendible.
- **Importancia como clave de orden secundaria** (desempate puro tras `score`): elegante, pero tampoco funciona con ruido posicional — el ruido rompe el empate ANTES de que la clave secundaria se consulte. Y una vez densificados los rangos, el término RRF ya da la semántica buscada (además de permitir que la importancia influya en cuasi-empates, no sólo en empates exactos).
