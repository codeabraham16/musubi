# Tareas — El juez ve ids cortos

## Medición sin modelo (PASO 1)
- [x] T1 · `juez_ids_medicion_test.go`: `fixtureConIDsOpacos`, `juezGrabador` y `TestVolcarPromptsDelJuez`.
- [x] T2 · Volcar los prompts del binario BASE (slug/uuid × léxica/híbrida) y contar caracteres y
      tokens con tiktoken (o200k y cl100k), simulando los rótulos.

## Código (PASO 2)
- [x] T3 · `rerank.go`: `rotuloDelJuez`, `PromptJuez` con rótulos y `ParsearOrdenDeIDs(respuesta, cands)`
      que traduce y descarta desconocidos y repetidos. `Rerank` le pasa los candidatos.
- [x] T4 · `rerank_test.go`: migrar los dobles y agregar I1–I4, cada uno con sus directivas `// arnes:`.
- [x] T4b · I2 con 12 candidatos (el top-K de producción): rótulos de dos dígitos, con su sabotaje.
      El ancla es la firma de `rotuloDelJuez` y no `strconv.Itoa(i+1)`, que ya usa I2: `-validar`
      marcaba la colisión.
- [x] T5 · `internal/mcp`: migrar los dobles que contestaban con ids reales.
- [x] T6 · Correr `internal/cognition`, `internal/recalleval` y las pruebas del juez de `internal/mcp`.
- [x] T7 · Correr los sabotajes nuevos con el arnés y ver cada rojo por la razón correcta.
- [x] T8 · Volcar los prompts con el binario NUEVO: tienen que coincidir byte a byte con la simulación de T2.
- [x] T8b · Tras la pasada 1: el rótulo pasa de `c<i+1>` a `id-<i+1>` (D1), y se repiten T4–T8.

## Exactitud (PASO 3, sólo Ollama local)
- [x] T9 · llama3.2:3b, brazos INTERCALADOS por consulta (base y nuevo, alternando cuál va primero),
      con la memoria y el swap de cada llamada. Después `ollama stop`. Pasada 1: UUIDs contra `c…`
      (cortada tras 2 fallas en las 4 consultas que llamaron al juez); pasada 2: UUIDs contra `id-…`,
      en dos tandas: la guardia de swap cortó la primera con 8 de 12 consultas y la continuación
      corrió las 4 restantes.
- [x] T10 · Agregar MRR, nDCG, R@k, fallas, ids mal copiados, tokens y latencia por brazo → `medicion.md`.

## Cierre
- [ ] T11 · Revisión adversaria, commit local sin push y result de la unidad.
