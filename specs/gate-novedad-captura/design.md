# Design — gate-novedad-captura

## `DetectOnly`: interceptar el VEREDICTO, no el pool

`DetectOnly` se aplica **después** de `decideRelation` y **antes** de persistir. Así el pool y el cómputo de señales (léxico + coseno, con el AND-gate de #193) quedan **idénticos** — sólo cambia qué se hace con el veredicto (R5).

En `DetectRelations`, tras decidir:

```go
rel := decideRelation(src, c, lex, cos, opts)
if opts.DetectOnly {
    // Camino AUTOMÁTICO (captura): detectar y MARCAR, nunca decidir. Ver R0.
    rel = pendingRel(src, c, lex, cos)
}
```

Y el `markSuperseded` queda inalcanzable por construcción: sólo corre si `rel.Relation == RelSupersedes && rel.Status == RelStatusResolved`, y `pendingRel` nunca produce eso. **No hay que "acordarse" de no ocultar memoria: es imposible llegar ahí.** (Mismo principio que el AND-gate de #193: el invariante vive en la estructura, no en una promesa.)

Reusar `pendingRel` (el helper de #193) da R4 gratis: la `Confidence` ya es la señal más fuerte de las dos.

## Por qué `DetectOnly` y no "bajar los umbrales"

La tentación es subir `auto_resolve_threshold` a 1.1 en la config de captura para que nunca auto-resuelva. **No sirve:** es frágil (un cambio de config lo rompe), es implícito (nadie entiende por qué hay un umbral imposible) y **no protege de verdad** — el invariante quedaría dependiendo de un número. Un flag explícito dice **qué** garantiza y lo hace **verificable con un test**.

## El peligro concreto que esto evita

`decideRelation` auto-supersede cuando: `lex >= auto` **Y** `cos >= cosAuto` **Y** `src.topicKey == cand.topicKey` **Y** `src.createdAt > cand.createdAt`.

En la captura de commits **todos** llevan `topic_key = "git-commit"`. Ese campo, que en el camino del agente identifica un **tema** (y por eso el auto-supersede tiene sentido), acá es un **balde**. Dos commits parecidos —`fix: typo en el README` / `fix: typo en el CHANGELOG`— tienen léxico alto, mismo "topic" y el nuevo es más reciente ⇒ **auto-supersede** ⇒ el commit viejo **desaparece del recall**. Automático, silencioso, y sin ningún agente mirando.

Es un caso donde una heurística **correcta en un contexto** (topic elegido por el agente) es **destructiva en otro** (topic como balde). El flag hace explícita esa diferencia de contexto.

## Cableado

**C3 (`capture.go`)** — `captureCommits` recibe un `detect func(obsID)` (inyectado, igual que `embed`), así el core sigue testeable sin engine real:

```go
id, deduped, err := store.SaveObservationDedupedTyped(...)
if err != nil { return saved, err }
if !deduped && detect != nil {
    detect(id)   // sólo sobre lo que REALMENTE se guardó (R6): un dedup por hash no crea observación nueva
}
saved++
```

`runCapture` arma el `detect` desde `cfg.Conflicts` con `DetectOnly: true`, y **traga los errores** (R8: la captura es una red de seguridad, no puede romper el turno).

**C4 (`methods.go:1588`)** — tras el `SaveObservationDedupedTypedFrom` del error→fix, correr `DetectRelations` con `DetectOnly: true`, best-effort (log + seguir). Es fire-and-forget: el valor del `musubi_log_error` no puede depender de que la detección funcione.

**El camino explícito no se toca:** `detectAndSurface` sigue con `DetectOnly` en `false` (cero) ⇒ auto-resolve como siempre. Ahí el `topic_key` lo eligió el agente y el AND-gate de #193 ya lo protege.

## Costo — a MEDIR, no a asumir

`DetectRelations` por commit = 1 query FTS + 1 búsqueda vectorial + los cosenos del pool (≤ `candidate_pool` = 10). La captura sólo corre cuando hay commits **nuevos**, y el binario ya paga ~1.2s cargando la tabla. El delta debería ser marginal — **pero se mide** antes de afirmarlo (la lección de #192: la estimación del hash estaba 30x errada).

## Alternativas descartadas

- **Saltear el guardado del duplicado (auto-NOOP):** **prohibido por R1.** Con cosenos que en dominio homogéneo arrancan en 0.60, un NOOP automático descartaría memoria legítima **en silencio**. Un `pending` de más cuesta atención; un guardado perdido cuesta **memoria**. El error se comete siempre para el lado seguro.
- **Fusionar (merge) el duplicado con el existente:** requiere entender **qué** afirma cada uno (¿complementa? ¿contradice? ¿corrige?) — un predicado. Los estáticos no lo evalúan. Es cancha del agente.
- **Umbral imposible en vez de flag:** ver arriba.
- **Correr la detección también sobre los dedup-por-hash:** no hay observación nueva que relacionar; `FindByContentHash` ya devolvió la existente.
