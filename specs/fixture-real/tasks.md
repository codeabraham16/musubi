# Tareas — El banco mide a escala real

Estado: **completo**. Build, `go vet ./...`, `gofmt`, `golangci-lint` y la suite entera en verde.
7 invariantes (K1–K7).

## Lo que se midió antes de proponer

- [x] **T0 — ★ El repo es PÚBLICO.** `gh repo view` ⇒ `"visibility": "PUBLIC"`. Eso corrige lo que yo
      había dicho («el fixture tiene que salir de memoria real»): volcar memoria real a `testdata/`
      la publicaría en GitHub, con IPs del tailnet y decisiones internas adentro. La convención
      correcta ya estaba tomada — los docs del `golden.json` son sintéticos.

- [x] **T0b — La memoria real, medida.** 1.210 observaciones vivas. **834 topics distintos**, pero
      sólo **27 con ≥3 observaciones** — y uno de esos es `git-commit` con **247**, que es un cajón
      de sastre y no un tema. Quedan ~26 topics usables, de 3 a 8 notas cada uno.

- [x] **T0c — El fixture dorado es demasiado chico para decidir.** 26 docs, 12 queries, 1–2
      relevantes. A esa escala una query que se mueve vale ~8 puntos de MRR: cualquier diferencia
      entre dos jueces queda enterrada en el ruido.

## Lo construido

- [x] **T1 — `FixtureDesdeDB`**, en sólo lectura (`mode=ro`) y sin escribir nada al disco.

- [x] **T2 — Etiquetas por `topic_key`**, con su limitación documentada en el código y repetida en el
      log de cada corrida.

- [x] **T3 — Los filtros**: mínimo 3 por topic, máximo 50, y exclusión por prefijo
      (`git-commit`, `sdd/`, `project/profile`). Lo excluido sigue en el corpus como distractor.

- [x] **T4 — La medición**, guardada por `MUSUBI_FIXTURE_DB` con `t.Skip` — el mismo patrón que ya
      usa `TestSemanticVsOllamaReal`. Es una medición, no un gate.

- [x] **T5 — 7 invariantes** (K1–K7), todos contra una base temporal de contenido conocido: no
      dependen de que exista memoria real.

## El número, que es el punto

Medido en vivo sobre la memoria de este proyecto:

```
fixture real: 1210 docs · 26 consultas · 4.1 relevantes por consulta

config                     MRR  R@1    nDCG@1   R@5    nDCG@5   R@10   nDCG@10
lexical                  0.532  0.105   0.423  0.353   0.382  0.456   0.417
```

| | dorado | real |
|---|---|---|
| docs | 26 | **1.210** (46×) |
| consultas | 12 | **26** |
| relevantes/consulta | 1–2 | **4,1** |
| MRR model-free | 0.663 | **0.532** |

Más grande **y más difícil**. Con R@10 en 0.456 hay margen real donde un juez puede aportar — que es
justamente lo que hacía falta: un fixture donde el model-free ya acierta todo no puede medir a un
juez, porque no le deja nada que arreglar.

## Lo que enseñó

**La pregunta correcta no era «cómo hago el fixture más grande» sino «de dónde salen las etiquetas».**
Un fixture grande con etiquetas derivadas del propio ranking es peor que uno chico bien etiquetado:
mide si el ranker coincide consigo mismo y devuelve un número alto que no significa nada. El
`topic_key` sirve porque lo escribió una persona describiendo de qué habla la nota, mucho antes y sin
saber cómo se iba a recuperar.

**Y verificar dónde se publica va antes de decidir qué se publica.** Si no hubiera corrido
`gh repo view`, habría commiteado la memoria interna del usuario a un repo público — y el error no
se habría notado en ningún test.

## Fuera de alcance

- ~~**No se corre contra el motor real.**~~ **CERRADO** por `specs/juez-real/` (2026-08-08): este
  fixture, generado de la memoria del cerebro central (1.216 docs / 36 consultas), es el que sostuvo
  la medición. El delta con juez dio **nDCG@1 +0.472**.
- **No hay etiquetado a mano.** Sería el patrón oro y es tiempo del dueño; el formato lo admite.
- **El fixture dorado no se tocó.**
