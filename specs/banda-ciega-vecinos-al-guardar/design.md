# Design — banda-ciega-vecinos-al-guardar

## D1 — `BandNeighbors` es un método SEPARADO y de SOLO LECTURA

```go
type BandNeighbor struct {
    ID       string
    TopicKey string
    Gist     string
    Cosine   float64
}

// No escribe NADA. No conoce UpsertObsRelation.
func (e *DbEngine) BandNeighbors(obsID string, opts ConflictOptions) ([]BandNeighbor, int, error)
//                                                    devuelve: los del techo, cuántos quedaron afuera
```

**Rationale — el invariante se vuelve estructural, no disciplinario.** R0 dice *«mostrar no es encolar»*. La forma **barata** de cumplirlo sería agregar un `if` dentro de `DetectRelations` y acordarse de no persistir. La forma **robusta** es que el código de la banda **no tenga camino de escritura**: es una función de lectura que **jamás llama a `UpsertObsRelation`**. No puede crear una relación **aunque quisiera**.

Es la misma jugada de M4 (que volvió `markSuperseded` inalcanzable) y de #203 (la guarda en el único choke point). **Un invariante que depende de que alguien se acuerde, tarde o temprano se rompe.**

Beneficio secundario: `DetectRelations` — el camino que **sí** toca la cola — **no se modifica ni una línea**. Riesgo cero sobre lo que ya funciona.

**Descartado — devolver los vecinos desde `DetectRelations`** (cambiando su firma o su tipo de retorno). Ahorraría una consulta, pero mete el código de la banda **adentro** del único lugar que escribe relaciones, que es exactamente donde no lo quiero. El costo que evito no lo vale: un `save` explícito del agente es una operación **rara** (no es un hot path) contra un SQLite **local**.

## D2 — Mismo pool, mismos cosenos que el detector

`BandNeighbors` reusa `conflictCandidates(src, srcVec, pool)` + `candidateCosines(srcVec, ids)`.

**Rationale.** Lo que se le muestra al agente tiene que ser **lo mismo que vio el detector**, medido igual. Si la banda armara su propio pool con otro criterio, un vecino podría aparecer/desaparecer por razones que nadie puede explicar — y la confianza en el aviso se cae.

## D3 — Las guardas de #203 valen acá también

`complementaryPair(src, c)` se aplica **igual** (R6). Es la **misma función**, no una copia.

**Rationale.** Mostrarle al agente el `spec` del mismo cambio SDD que acaba de guardar es exactamente el ruido que #203 sacó de la cola. Sería absurdo sacarlo por una puerta y meterlo por la otra.

## D4 — La banda es semiabierta: `[BandFloor, CosineFloor)`

```go
if cos < opts.BandFloor || cos >= opts.CosineFloor { continue }
```

**Rationale.** El extremo superior es **exclusivo** a propósito: si el coseno alcanza `CosineFloor`, el par **ya es una relación `pending`** y el agente ya lo ve por el camino de siempre (R1/S.b). Avisar dos veces por lo mismo entrena al agente a ignorar el aviso — la erosión otra vez, pero autoinfligida.

## D5 — Sólo el camino explícito, y se nota en la ESTRUCTURA

`detectAndSurface(id)` (el camino del agente) llama a `BandNeighbors`. `detectOnly(id)` (captura de commits, error→fix) **no lo llama**.

**Rationale.** No hace falta un flag ni un `if opts.DetectOnly` adentro: los caminos automáticos simplemente **no invocan** la función. Un aviso que nadie lee no es un aviso — es basura en un log.

## D6 — El mensaje es corto, ordenado y HONESTO sobre lo que recorta

```
⚠️ 2 memoria(s) hablan de esto mismo sin ser duplicados. ¿Alguna queda SUPERADA por lo que acabás
de guardar? (si sí: musubi_judge con relation=supersedes)
  · [server/nordvpn-tailscale-wfp-bloqueo] (0.81) INVESTIGACIÓN PROFUNDA (deep-research, 99 agentes)…
  · [server/nordvpn-tailscale-SOLUCION] (0.80) ✅ RESUELTO — NordVPN + Tailscale SÍ COEXISTEN…
  (hay 4 más por debajo del techo)
```

**Rationale.** Se muestra el **gist**, no el contenido: es lo que el recall ya usa para no quemar tokens. Se ordena por coseno **descendente** (lo más parecido primero, que es lo más probable que importe). Y el `(hay N más)` **no es cosmético**: un recorte **silencioso** le dice al agente *«esto es todo»* cuando no lo es — y eso es peor que no avisar, porque genera **falsa cobertura**.

La pregunta es **explícita y accionable** (nombra la herramienta y el veredicto). Un aviso que no dice qué hacer con él se ignora.

## D7 — `BandFloor` en config, default 0.80 (MEDIDO, no estimado)

`Conflicts.BandFloor = 0.80`. Sale de la medición sobre las 436 observaciones reales: el par contradictorio da **0.806**, y el p99 de todos los pares es **0.803**.

**Rationale.** El default deja entrar, aproximadamente, **el 1% más similar** de los pares — que es la definición operativa de *«esto habla de lo mismo»*. `BandFloor <= 0` o `>= CosineFloor` **apaga** la feature (R2): rollback sin tocar código ni datos.

**Honestidad sobre el número:** 0.80 sale de **UNA** medición sobre **UNA** memoria. No es una constante de la naturaleza. Por eso es **configurable** y por eso el default se documenta como lo que es: **una heurística calibrada, no una verdad**.

## Lo que este diseño NO hace

- **No decide** si hay contradicción. Muestra el par y pregunta. Evaluar el predicado (*«¿esto niega aquello?»*) es el **techo semántico** — requiere LLM, y el LLM es el agente.
- **No detecta toda contradicción.** Una con coseno **< 0.80** sigue invisible. Es un límite **declarado**, no un descuido.
- No toca la cola, ni el piso del dedup, ni el AND-gate, ni el gate de novedad.
