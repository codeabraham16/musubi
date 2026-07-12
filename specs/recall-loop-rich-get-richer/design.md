# Design — recall-loop-rich-get-richer

## El criterio que ordena todo: exógeno vs endógeno

- **Exógeno** — el ranker **no lo puede cambiar**: `created_at`, el texto, el vector. Prior legítimo.
- **Endógeno** — **lo escribe el propio ranker**: `last_accessed`, `access_count`. Rankear con esto es circular **por definición**.

No hay que prohibir el uso del acceso: hay que impedir que **se acumule para siempre**. Un lazo endógeno **sin fuga** es un acumulador desbocado; **con fuga**, es un integrador que se auto-limita.

## N4.a — Recencia = `created_at`

```go
recencyRank := denseRankBy(cands, func(a, b candidate) bool {
    return a.createdAt > b.createdAt   // antes: effectiveRecency (last_accessed ?? created_at)
})
```

`effectiveRecency` queda **sin uso en el ranking** y se elimina. Los campos `lastAccessed` / `access_count` **siguen** en `candidate` y en la base: los usa el olvido.

**Qué se pierde a propósito:** el prior de *"locality de sesión"* (lo que acabo de mirar sigue siendo relevante). Es intencional — **ese prior ES el lazo**. La parte legítima de esa intuición (*"esto se usa seguido últimamente"*) la conserva N4.b, pero acotada.

## N4.b — Frecuencia = TASA de uso, no total

```go
// accessRate: usos por día de vida. A igual cantidad de accesos, la observación MÁS VIEJA tiene
// menor tasa ⇒ la ventaja se ERO SIONA si deja de usarse. Convierte el acumulador desbocado
// (access_count sólo sube) en un integrador CON FUGA, que se auto-limita.
// El +1 del denominador suaviza: una observación recién creada (edad ~0) no explota.
func accessRate(c candidate, now time.Time) float64 {
    if c.accessCount <= 0 {
        return 0
    }
    return float64(c.accessCount) / (ageDays(c.createdAt, now) + 1)
}

freqRank := denseRankBy(cands, func(a, b candidate) bool {
    return accessRate(a, now) > accessRate(b, now)   // antes: a.accessCount > b.accessCount
})
```

**Por qué una TASA y no un `log`:** `freqRank` es un **rango**. Cualquier transformación **monótona** del contador da **exactamente el mismo rango** — `rank(log(x)) == rank(x)`. Amortiguar la **magnitud** no cambia **nada** en una fusión por rangos. Para romper el lock-in hay que cambiar el **orden**, y para eso el tiempo tiene que entrar en la cuenta. Es el error que uno cometería por instinto ("le pongo un log y listo") y no haría absolutamente nada.

**`now` inyectado**, no `time.Now()` adentro: `scoreCandidates` es una función pura y determinista, y los tests necesitan fijar el reloj. Se pasa desde `Recall`.

## Parseo de fechas

`created_at` es ISO8601 (`time.RFC3339`). `ageDays` parsea y devuelve días; ante un parseo fallido devuelve `0` (edad desconocida ⇒ tasa = `count/1` = el contador crudo, o sea el comportamiento previo para esa fila: **degradación segura**, no un error).

El orden **lexicográfico** de `created_at` sigue sirviendo para la recencia (ISO8601 ordena bien como string), así que N4.a no necesita parsear nada.

## Impacto

- **31%** de la base tiene `last_accessed` ⇒ su rango de recencia cambia. Es el objetivo.
- `recalleval` asierta **propiedades** (monotonía de recall@k, MRR>0, métricas ∈ [0,1]), no valores exactos ⇒ debería seguir verde. **Se verifica, no se asume.**
- Los tests que dependan de que un acceso mejore el rango DEBEN actualizarse: codifican el bug.

## Alternativas descartadas

- **`log(access_count)`:** no hace nada. `rank(log(x)) == rank(x)`. Ver arriba. (Sería el arreglo "obvio" y es puro placebo.)
- **Quitar la frecuencia del ranking:** tira una señal legítima (lo que se usa seguido **suele** ser relevante). Con la tasa se conserva **acotada**.
- **Quitar `bumpAccess`:** rompería el olvido (Ebbinghaus necesita el acceso para saber qué NO olvidar). El dato no es el problema; el problema es **usarlo crudo para rankear**.
- **Decaer el `access_count` en la base** (un job que lo baje con el tiempo): destruye información real (cuántas veces se usó de verdad) y además el olvido lo necesita. La tasa **calcula** la fuga en el momento del ranking, sin tocar el dato.
