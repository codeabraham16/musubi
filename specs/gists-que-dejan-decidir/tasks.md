# Tasks — gists-que-dejan-decidir

## Implementación

- [x] **T1** — `Gist()` acumula oraciones hasta llenar `maxTokens`; la rama que ya lo llenaba queda **intacta**.
- [x] **T2** — Si la última oración **no entra entera**, se **trunca para llenar el techo** (coherente con lo que `Gist()` ya hacía con la primera).
- [x] **T3** — `doctor`: check `stale_gists` + reparación `applyRegenGists` (**explícita**, `--fix`).
- [x] **T4** — `staleGist(gist, content)` = *«el guardado difiere del que produce el extractor ACTUAL»* — la única definición de «viejo» que no depende de recordar qué versión lo escribió.

## Tests

- [x] **T5** — S.a: con techo de sobra, el gist **suma oraciones**.
- [x] **T6** — S.b: **nunca** excede el techo (barrido de techos).
- [x] **T7** — S.e: una sola oración ⇒ no se inventa nada.
- [x] **T8** — S.f/S.g: la reparación es **idempotente** y **no toca** `content`.
- [x] **T9** — Los dos tests que **pineaban el bug** (`TestGistFirstSentence`, `TestRecallReturnsGistsNotFullContent`) se reescriben para pinear el **contrato nuevo**, sin aflojar lo que sí protegían (que el gist **no** sea el contenido completo y **no** exceda su techo).

## Medición (obligatoria)

- [x] **T10** — Medido sobre la memoria real: **gists mudos 110 (24%) → 13 (3%)**; **items por recall de 700 tokens: ~39 → ~34 (−13%)**.
- [x] **T11** — La primera regla («no truncar la 2ª oración») **fue refutada por la medición**: sólo arreglaba 39 de 461 y **dejaba mudos justo a los que motivaron el cambio**.

## Cierre

- [x] **T12** — `go test ./...` verde + `golangci-lint` limpio.
- [ ] **T13** — CHANGELOG + PR.

## Orden

T1→T4 (motor), T5→T9 (tests), T10-T11 (la medición, que **corrigió el diseño**), T12→T13.

**T11 es la tarea que más enseñó:** la regla que *sonaba* prolija era **peor** que la que *sonaba* sucia. Sólo medir lo mostró.
