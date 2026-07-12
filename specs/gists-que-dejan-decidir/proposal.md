# Proposal — gists-que-dejan-decidir

## Intención

**El 24% de los gists no te dejan decidir nada.** Medido sobre la memoria real: **110 de 461** gists usan menos de 15 tokens de un techo de **24**, y lo que dicen es esto:

```
 8 tok | sdd/brain-dashboard/spec    | "SDD spec — brain-dashboard BACKEND."
 8 tok | sdd/debate-topology/verify  | "SDD verify — debate-topology VERDE."
 9 tok | sdd/brain-dashboard/tasks   | "SDD tasks — brain-dashboard BACKEND."
11 tok | sdd/recall-diversidad-mmr/tasks | "SDD tasks — recall-diversidad-mmr 17 tareas."
```

**El gist existe para UNA cosa: que el agente decida si vale la pena expandir la memoria.** Es la pieza central del recall por presupuesto — traés gists baratos, y sólo hidratás lo que importa.

> **Un gist que no te deja decidir es PEOR que inútil: cuesta tokens y no da nada.** Te obliga a expandir para saber qué dice — es decir, a pagar **dos veces** por la información que el gist debía anticipar.

## La causa (una línea)

`Gist()` toma **la primera oración y se detiene**:

```go
lead := firstSentence(norm)
if EstimateTokens(lead) <= maxTokens {
    return lead          // <-- y si la primera oración son 8 tokens, ahí queda
}
```

Si esa oración es corta, el gist **abandona 16 de sus 24 tokens** sin siquiera intentar decir algo más. No es un problema de los contratos SDD ni de cómo se redactan: **es del extractor.**

## Alcance

- **F1** — `Gist()` sigue **sumando oraciones** mientras entren en `maxTokens`, en vez de cortar en la primera. El techo **no cambia**: lo que cambia es que se **usa**.
- **F2** — Reparación en el `doctor` para **regenerar** los gists existentes con el extractor nuevo. Sin esto, el fix sólo aplicaría a memoria futura y los 110 gists mudos quedarían mudos para siempre.

## El canje, dicho de frente

Los items afectados pasan de **~11 a ~24 tokens** ⇒ en el **mismo presupuesto**, entran **menos memorias por consulta**.

**Es un canje deliberado y es el correcto:** una memoria de la que **no podés decidir nada** no está realmente "en el resultado" — es un puntero mudo que igual vas a tener que expandir. Preferimos **menos items, cada uno decidible**, antes que más items que obligan a una segunda vuelta.

Hay que **medirlo**, no asumirlo: cuántos items menos entran en un recall típico.

## Fuera de alcance (explícito)

- NO se toca el techo del gist (`gist_max_tokens`), ni el presupuesto del recall, ni el ranking, ni MMR.
- NO se cambia el contenido de ninguna observación. El gist es **derivado**: regenerarlo es idempotente y no pierde información.
- NO se cambia cómo se redactan los contratos SDD (eso sería tratar el síntoma en vez de la causa).

## Estrategia de rollback

El gist es **derivado del contenido**: revertir el PR y regenerar devuelve los gists de antes, bit a bit. Sin migración, sin pérdida.

## Riesgos

- **Entran menos items por consulta.** Es el canje aceptado. Hay que **medir** cuántos y decirlo.
- **Una segunda oración puede ser peor que nada** (ruido, boilerplate). Mitigación: se corta por **tokens**, no por cantidad de oraciones; y si la primera oración ya llena el techo, el comportamiento es **idéntico al de hoy**.
- **La regeneración toca 461 gists.** Es idempotente y derivada, pero conviene que sea una **reparación explícita del doctor**, no un efecto colateral silencioso de arrancar el binario.
