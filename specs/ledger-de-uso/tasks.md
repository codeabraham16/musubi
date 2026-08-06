# Tareas — Ledger de uso (F0 · track «Potencia medida»)

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y las 17 suites del repo en verde.

- [x] **T1 — Esquema.** Migración **v23** más la tabla en la baseline de `database.go`.
      Hay que tocar **los dos lados**: la baseline arma la base nueva y la migración la ya
      existente. Acá el patrón es seguro porque `CREATE TABLE IF NOT EXISTS` **sí** es idempotente
      —a diferencia de `ALTER TABLE ADD COLUMN`, que fue lo que rompió toda base nueva cuando la
      v22 repitió la DDL a mano.

      La mitad del diseño es **lo que la tabla NO tiene**: ni argumentos, ni resultado, ni mensaje
      de error. La fuga es imposible porque no hay dónde escribirla.

- [x] **T2 — El punto de estrangulamiento.** El registro va en `handleToolsCall`, con **tres**
      enganches y no uno: el retorno normal, el rechazo por rol y el rechazo por cuota. Más un
      `defer` para el handler que entra en pánico — sin él, la invocación que MÁS interesa (la que
      rompió algo) sería la única sin registrar, porque el `recover` está más arriba y esta función
      nunca vuelve por su `return`.

- [x] **T3 — Buffer + flush.** `append` bajo mutex en el camino caliente; una goroutine drena por
      lote. **No toma `dispatchMu`**: el handler todavía lo tiene y re-entrarlo es deadlock, la
      misma trampa que documenta `maybeTriggerMaintenance`.

- [x] **T4 — Motor** (`internal/memory/toolledger.go`): insert por lote en una transacción,
      `ToolUsage` con p95 sobre datos crudos, purga, y el formateador —que vive en `memory` y no en
      el handler porque el cuerpo va a querer el mismo formato en F5.

- [x] **T5 — `musubi_tool_usage`** (50 → 51 tools), con sus dos consumidores declarados antes de
      escribirla, que es lo que la regla del track exige.

- [x] **T6 — Purga colgada del mantenimiento** que ya existe, en vez de un timer nuevo.

- [x] **T7 — Config** `usage_ledger`, que **nace encendido**. `Enabled` es `*bool` para distinguir
      "no lo escribieron" de "lo apagaron a propósito": con un bool pelado, omitir el bloque
      apagaría el medidor, que es el default equivocado.

- [x] **T8 — Una FUGA DE AISLAMIENTO que cazó el propio repo.** El barrido `TestEveryReadOnlyTool
      Classified` obligó a clasificar la tool nueva, y al hacerlo quedó claro que la tabla tiene
      `project_id` y la consulta **no lo filtraba**: en el cerebro central, un miembro habría visto
      qué herramientas usa OTRO equipo y con qué frecuencia — patrón de trabajo ajeno, que es
      información de negocio. Se arregló con `scopedCtx` + `scopeClause`, y se sumó el caso al
      barrido con su marcador (`VICTIMTOOL`), reproduciendo la fuga antes de taparla.

      La lección: el guard de completitud del Track 19 existe justamente porque scopear
      tool-por-tool siempre deja una hermana federada. Funcionó.

- [x] **T9 — Conteos y goldens.** 50 → 51 en `server_test.go`, `http_test.go`,
      `dispatch_concurrent_test.go`, el golden regenerado con `-update`, y el README (que tiene su
      propio test de coherencia). Más la guarda de esquema en `outbox_test.go`, 22 → 23.

- [x] **T10 — Sabotaje: 9 mutaciones, cada una puso en rojo el test de su invariante.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | No registrar el rechazo por rol | L0 | rojo |
      | No registrar el rechazo por cuota | L0 | rojo |
      | Registrar los errores como `ok` | L0 | rojo |
      | Meter los argumentos crudos en el ledger | L1 | rojo |
      | Que un fallo del ledger propague | L2 | rojo |
      | Escribir sincrónico en el camino caliente | L5 | rojo |
      | Sacar el techo del buffer | L6 | rojo |
      | Que la purga ignore la retención | L6 | rojo |
      | `outcome` acepta texto libre | taxonomía | rojo |

## Dos veces que el test estuvo mal, no el código

Las dos son formas de **pasar en verde sin probar nada**, y las dos aparecieron sólo por sabotear.

1. **L0 no cubría los rechazos.** El invariante dice «incluidos los rechazados» y el test sólo
   ejercitaba éxito y error, así que borrar el registro del rechazo por rol pasaba en verde. Se
   agregó un test con un `reader` que intenta escribir y un `writer` que agota su cuota.

2. **La purga pasaba por TIMING, no por lógica.** La fila «reciente» se insertaba con
   `RecordToolInvocations`, que la sella con `CURRENT_TIMESTAMP` — granularidad de **segundo**. Al
   purgar dentro del mismo segundo, `created_at < datetime('now')` daba falso y la fila sobrevivía
   por casualidad: el sabotaje que pone la retención en cero pasaba en verde. Se arregló insertando
   las dos filas con fecha explícita, para que la frontera sea inequívoca.

## Fuera de alcance, dicho de frente

- **Tokens y costo por invocación.** El handler no los devuelve hoy, así que el ledger no puede
  registrarlos. Es lo primero que conviene sumar cuando haya de dónde leerlos: sin eso, «cuánto me
  cuesta cada tool» sigue sin respuesta.
- **Un flush pendiente se pierde si el proceso muere de golpe** (L7). Aceptado: es telemetría, no
  el libro mayor.
- **Los 10 segundos de flush y los 90 días de retención son números elegidos, no medidos.** Se
  podrán calibrar cuando el propio ledger diga cuánto escribe por día.
- **El ledger no se mide a sí mismo.** El costo del `append` es real aunque sea de microsegundos;
  no se promete cero.
