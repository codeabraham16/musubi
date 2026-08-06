# Tareas — Grounding fiel para `musubi_ask`

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y las 17 suites del repo en verde.

- [x] **T1 — Partir la hidratación en dos puertas** (`internal/memory/expand.go`).
      `hydrateByIDs` es el empaquetado puro; `GetObservationsBudgetCtx` = eso + `bumpAccess` (sin
      cambios para `memory_expand`); `HydrateForGroundingCtx` = eso solo.

      No es un refactor cosmético: `Recall` ya llama a `bumpAccess` sobre los mismos ids
      (`recall.go:297`), así que reusar la función existente contaría **dos accesos por pregunta**.
      El ranking usa frecuencia ⇒ sería el ranker realimentándose con su propia salida, lo que
      `recall.go:362` prohíbe explícitamente (invariante N4).

- [x] **T2 — `HydrateForGroundingCtx` en la interfaz `StorageBackend`** (`backend.go`), con el
      porqué del "no cuenta el acceso" escrito ahí, que es donde lo va a leer el que implemente otro
      backend.

- [x] **T3 — `staleWarning` → `StaleWarning`** (`origins.go`). El prompt se arma en el paquete
      `mcp` y la advertencia se calcula en `memory`. La alternativa —reescribir el texto allá—
      serían dos formatos de advertencia divergiendo.

- [x] **T4 — Hidratación en `toolAsk`** con presupuesto derivado: `min(token_budget × 2, 16000)`.
      Sin perilla nueva: el caller ya expresa cuánta memoria quiere con el parámetro que existe.

- [x] **T5 — El prompt.** Contenido completo cuando entró, gist cuando no; advertencia de rancio
      repuesta; sello de procedencia en la cabecera.

      El sello **no era una consecuencia** de hidratar: era un agujero de Q3 que estaba desde antes.
      El recall marcaba la procedencia para el caller humano y el prompt del motor no la llevaba, o
      sea que una inferencia corroborada le llegaba al sintetizador con la misma cara que una nota
      verificada. Se cierra acá porque es el mismo `Fprintf`.

- [x] **T6 — 11 tests** (8 invariantes + 3 de la partición de la hidratación).

- [x] **T7 — Sabotaje: 8 mutaciones, cada una puso en rojo el test de su invariante.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | El prompt vuelve a usar `it.Gist` | G1 | rojo |
      | El prompt sólo itera lo hidratado | G2 | rojo |
      | No reponer la advertencia de rancio | G3 | rojo |
      | Cabecera sin sello de procedencia | G4 | rojo |
      | Sacar `quarantined = 0` del predicado canónico | G5 | rojo |
      | Un fallo hidratando devuelve error | G6 | rojo |
      | Presupuesto de hidratación ilimitado | G7 | rojo |
      | La puerta de grounding vuelve a bumpear | G8 | rojo |

## Dos veces que el test estuvo mal, no el código

Vale escribirlas porque las dos son formas de **pasar en verde sin probar nada**, que es el modo de
falla que este proyecto ya se comió en F4 y otra vez esta semana auditando el portero.

1. **G4 se autoenvenenó con su propio dato.** La aserción "una memoria humana no lleva sello" se
   escribió como `!Contains(prompt, "human")`, y el topic de la memoria de prueba era `t/humana` —
   que contiene `human`. El test fallaba por su nombre de topic. Se arregló buscando la forma real
   del sello (`· human`) y renombrando el topic.

2. **G2 no detectaba su propio sabotaje.** La primera versión usaba tres memorias chiquitas que
   entraban **todas** en la hidratación, así que la rama "esta no entró" nunca se ejecutaba: el
   sabotaje que borra del prompt justo a las no hidratadas pasaba en **verde**. Reescrito para forzar
   hidratación PARCIAL y verificarlo como control antes de afirmar nada.

   La segunda versión tampoco servía por otro motivo: corría un `Recall` aparte para comparar el
   orden, y ese `Recall` **bumpea los accesos**, cambiándole el ranking al `Recall` que hace `ask`.
   El test fallaba por su propia sonda. La versión final saca todo de un único prompt.

## Fuera de alcance, dicho de frente

- **Cruza más texto al motor externo.** Antes gists truncados, ahora contenido completo. Lo cubre el
  portero de F1 —verificado en el cable el 2026-08-05— pero la superficie es objetivamente mayor, y
  quien ponga `gateway.mode: off` se expone a más que antes de este cambio.
- **El tope de 16000 es un número elegido, no medido.** Sale de lo que entra cómodo en un contexto
  moderno, no de una medición de calidad de respuesta. Se podrá calibrar recién cuando haya un motor
  real enchufado.
- **`musubi_recall` sigue devolviendo gists.** El camino model-free no se toca: quien quiera el
  contenido completo sigue usando `musubi_memory_expand`.
- **El juez read-time sigue viendo gists.** Reordena candidatos, no sintetiza; darle contenido
  completo multiplicaría el costo del camino caliente para decidir un orden. Es otra decisión.
