# Tareas — El juez se puede medir

Estado: **completo**. Build, `go vet ./...`, `gofmt`, `golangci-lint` (0 issues) y la suite entera
(17/17 paquetes) en verde. 9 invariantes, 9 sabotajes, los 9 en rojo.

## Lo que se midió antes de proponer

- [x] **T0 — El banco ya existe y le falta una sola cosa.** `internal/recalleval` tiene `Evaluate`
      (MRR / Recall@k / nDCG@k), `Run` (compara configs sobre el mismo corpus), `FormatReport`, el
      baseline model-free commiteado y barridos semántico/MMR/ollama. No había que construir un banco.

- [x] **T0b — Y no podía llamar al juez.** `Config` era `{Name, Opts, UseVector}`: sin brazo. El juez
      vivía dentro de `rerankIfEnabled` en `internal/mcp`, atado a `s.cognition`, `s.cognitionCfg`,
      `memory.RecallResult` y un caché global de paquete.

- [x] **T0c — La salida fácil era la trampa.** Darle al banco su propia copia del prompt lo habría
      hecho medir una **imitación**: un número con aspecto de autoridad sobre algo que no corre en
      producción, que sigue igual de convincente el día que el prompt real cambie.

- [x] **T0d — Esta PC no puede correr el banco contra el motor real.** La config local no tiene
      bloque `cognition` (verificado: sólo `version`, `mode`, `skills_auto_resolve`, `project_id`,
      `memory`, `sync`). La corrida real necesita el central, o el túnel con `LITELLM_MASTER_KEY`.

## Lo construido

- [x] **T1 — El juez sale a `internal/cognition/rerank.go`**: `Candidato`, `PromptJuez`, `Rerank`,
      `ParsearOrdenDeIDs`, `ReordenarIDs` y `DefaultTopK`.

- [x] **T2 — `rerankIfEnabled` queda como adaptador**: arma los candidatos, consulta SU caché, llama
      a `cognition.Rerank` con SU timeout, reordena. El caché y el timeout no se fueron con el juez.

- [x] **T3 — Una sola regla de reordenamiento.** `reorderByIDs` ya no reimplementa «nunca se pierde
      una memoria»: delega en `cognition.ReordenarIDs` y sólo mapea ids de vuelta a items.

- [x] **T4 — Una sola constante de top-K.** `defaultRerankTopK` pasa a ser alias de
      `cognition.DefaultTopK`, y el banco usa la misma.

- [x] **T5 — `Config` gana `Juez` y `JuezTopK`**, y `rankedIDs` aplica el brazo sobre la cabeza del
      ranking igual que producción.

- [x] **T6 — 9 invariantes**, repartidos donde vive lo que prueban: J2/J7/J8 en `cognition`,
      J1/J3/J9 en `mcp`, J4/J5/J6 en `recalleval`.

- [x] **T7 — Sabotaje: 9 mutaciones, las 9 en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Producción arma el prompt sin los gists | J1 | rojo |
      | El juez cachea adentro | J2 | rojo |
      | Producción deja de cachear | J3 | rojo |
      | El brazo del banco se desconecta | J4 | rojo |
      | El brazo degrada el camino model-free | J5 | rojo |
      | El banco memoiza entre pasadas | J6 | rojo |
      | El juez descarta lo que no mencionó | J7 | rojo |
      | Imparseable devuelve orden vacío sin error | J8 | rojo |
      | El motor caído rompe el recall | J9 | rojo |

- [x] **T8 — El brazo se ve funcionando, con número.** Con un juez falso que INVIERTE la cabeza del
      ranking, sobre el fixture dorado: **MRR 0.6627 → 0.3877**. El brazo está enchufado de verdad;
      si estuviera desconectado, las métricas no se moverían.

## Dos cosas que el trabajo enseñó

**Dos pruebas mías estaban mal, y las dos lo dijeron los números.**

`{"orden": ["a","b"]}` lo puse como ejemplo de respuesta imparseable, y **sí parsea**: la tolerancia
toma del primer `[` al último `]`, así que rescata los ids correctos. El caso de prueba estaba mal,
no el código — y la tolerancia es deliberada, porque los modelos envuelven el array aunque se les
pida que no. Se convirtió en su propio invariante para que nadie la "arregle".

Y J6 esperaba «una llamada por query»: 12 queries, 10 llamadas. Tampoco era memoización — **dos
queries devuelven menos de 2 candidatos y ahí no hay nada que ordenar**, igual que en producción. Peor
todavía: contar llamadas dentro de UNA corrida no puede detectar un caché, porque cada query aparece
una sola vez. Se reformuló a correr la misma configuración **dos veces** y exigir que las llamadas se
dupliquen exactas (10 → 20).

**La asimetría de la degradación es el corazón del diseño.** Producción se traga el error del juez y
devuelve el orden model-free: el usuario pidió memoria, no un juez. El banco NO: aborta. Un juez roto
que devuelve el orden model-free daría «el juez no aporta nada» — una conclusión falsa con cara de
medición, que es peor que un test en rojo.

## Fuera de alcance, dicho de frente

- **El juez sigue apagado.** Este spec construye el instrumento; encenderlo es decisión del dueño,
  con el número en la mano.
- **El fixture no se amplió.** 26 docs / 12 queries alcanza para verificar que el brazo funciona,
  **no** para que una diferencia de MRR entre dos jueces reales signifique algo. Ampliarlo con
  memoria real es el trabajo que sigue, y es el que de verdad contesta la pregunta de F2.
- ~~**No se corrió contra el motor real.**~~ **CERRADO** por `specs/juez-real/` (2026-08-08): medido
  contra el cerebro central, **nDCG@1 0.333 → 0.806**. El instrumento ya dio su respuesta.
- **Presupuesto y autorización del motor** siguen siendo los pasos 3 y 4 de F1, intactos.
