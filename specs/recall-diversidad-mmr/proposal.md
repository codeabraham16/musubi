# Proposal — recall-diversidad-mmr

## Intención

**El recall no está mal rankeado: se está REPITIENDO.** Medido sobre la memoria real, con la consulta *«banda ciega y detección de contradicciones en la memoria»* (presupuesto 1200 tokens, 60 items):

| Cambio | Fases suyas que aparecen en el resultado |
|---|---|
| `banda-ciega-vecinos-al-guardar` | **7** — *las siete* (proposal, spec, design, tasks, implement, verify, archive) |
| `registros-inmutables-y-doble-aviso` | **7** |
| `conflictos-artefactos-mismo-cambio` | **5** |

**19 de 60 items — casi un tercio del presupuesto — son 3 cambios contados siete veces cada uno.**

Y varios **no aportan nada**: el gist de `tasks` es literalmente *«17 tareas.»*; el de `verify`, *«VERIFICACIÓN ADVERSARIAL.»*.

Lo más grave: la nota del **principio destilado** (`principios/deteccion-de-memoria`) — posiblemente el item **más útil** de todos — quedó **6ª, por debajo de 5 contratos SDD del mismo cambio**.

## El diagnóstico

El ranker fusiona **siete señales** (léxico, vector, recencia, frecuencia, centralidad, co-ocurrencia, importancia) y hace bien su trabajo: los siete contratos de un cambio **son** relevantes para una consulta sobre ese cambio. **Pero ninguna señal mira lo que YA se eligió.**

> El ranking optimiza **relevancia por item**. Nadie optimiza **la utilidad del CONJUNTO.** Y el presupuesto de tokens es del **conjunto**.

Es un fallo de **redundancia**, no de relevancia. El flujo SDD (7 contratos por cambio) lo amplifica: es exactamente la misma estructura que ya nos mordió en la cola de conflictos (#203), ahora del lado del **recall**.

## Alcance

**MMR (Maximal Marginal Relevance)** como paso de **re-ranking** después de la fusión RRF y **antes** de empaquetar por presupuesto:

```
elegir iterativamente el item que maximiza:
    λ · relevancia(i)  −  (1−λ) · max_{j ya elegido} similitud(i, j)
```

- La **similitud** entre items es el **coseno de sus vectores** (los mismos que ya usa el recall).
- **λ configurable.** `λ = 1` ⇒ MMR **apagado**, comportamiento actual **bit-idéntico** (rollback sin código).
- Sin vector ⇒ **sin penalización** (degradación segura: nunca se castiga a una memoria por no tener embedding).

## Fuera de alcance (explícito)

- NO se tocan las 7 señales, ni sus pesos, ni el RRF, ni los umbrales.
- NO se filtra ni se oculta nada: MMR **reordena**, no descarta. Un item redundante **baja**, no desaparece — si el presupuesto alcanza, sigue estando.
- NO se toca el gate de conflictos ni la banda.

## El instrumento que hace esto honesto

El CI ya tiene un **`recall-gate`**: R@10 ≥ 0.80 sobre un fixture dorado, con la tabla POTION real. **La diversidad canjea relevancia por cobertura**, así que el gate es exactamente lo que va a decir si me pasé de λ. **Si R@10 cae por debajo del piso, la propuesta está mal calibrada — y me entero antes de mergear, no después.**

## Estrategia de rollback

`λ = 1` en config ⇒ MMR apagado, ranking **bit-idéntico** al actual. Sin migración, sin esquema, sin datos.

## Riesgos

- **MMR puede BAJAR el R@10.** Es el riesgo central y por eso el gate es la vara. Si para no romperlo hay que poner λ tan alto que MMR no haga nada, **la feature no se justifica** y hay que decirlo.
- **La penalización usa coseno, y el coseno de documentos del mismo dominio arranca ALTO (~0.60 medido).** Si se penaliza sobre esa base, se castigaría a *todo*. La penalización tiene que morder **sólo** donde hay redundancia real.
- **Diversidad ≠ utilidad.** Forzar variedad puede meter items peores sólo por ser distintos. λ es el dial de ese canje, y su default debe salir de **medir**, no de estimar.
