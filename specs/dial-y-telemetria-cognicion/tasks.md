# Tareas — Dial de potencia y telemetría (F5)

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y las 17 suites en verde.

- [x] **T1 — Dial** (`internal/config/effort.go`): `NormalizeEffort`, `ApplyEffort` y
      `ReadTimeRerankOn`. Tres niveles sobre tres perillas reales.

- [x] **T2 — `ReadTimeRerank` pasó de `bool` a `*bool`.** Es el cambio que hace posible D0: con un
      bool pelado, «no lo escribieron» y «lo escribieron en false» son el mismo cero de Go y el
      preset pisaría lo explícito. Misma razón por la que `cache.enabled` ya era puntero.

- [x] **T3 — Resolución en `config.Load`**, y no en `cognition.NewProvider`: el servidor MCP guarda
      su propia copia de `CognitionConfig` para decidir el juez del recall, así que resolverlo en la
      fábrica dejaría esa copia sin dial. Un dial que rige en la mitad de los consumidores es peor
      que no tener dial.

- [x] **T4 — Contadores** (`internal/cognition/telemetry.go`): `gatewayStats` y `routerStats`
      detrás de PUNTEROS —`guarded` es un tipo por valor y con campos cada copia contaría por su
      cuenta—, más la interfaz `statsReporter` que arma la foto recorriendo la cadena.

- [x] **T5 — Instrumentación** en las tres capas: portero (llamadas, tapadas, bloqueadas, pánicos y
      TIPOS), router (escaladas, agotamientos, circuitos abiertos) y caché (hits, misses, tamaño).

- [x] **T6 — `musubi_cognition_stats`**, read-only. Tool MCP y no `musubi doctor` porque los
      contadores viven en memoria del proceso que atiende y el CLI es OTRO PROCESO: el doctor
      reportaría ceros para siempre, que es peor que no tener número.

- [x] **T7 — 17 tests de invariantes**: 9 del dial (`internal/config/effort_test.go`) y 8 de la
      telemetría (`internal/cognition/telemetry_test.go`), con prefijo `D*` para no colisionar con
      los `C*` del router ni los `K*` del caché en el mismo paquete.

- [x] **T8 — Sabotaje: 5 mutaciones, cada una en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | El preset pisa lo explícito | D0 | rojo en los dos sentidos (turbo pisa false, eco pisa true) |
      | `""` cae a `balanced` (default implícito) | D1 | rojo — 3 aserciones |
      | Un `effort` desconocido cae a `balanced` | D2 | rojo — los 5 valores malos |
      | Meter el prompt en los tipos del portero | D5 | rojo (`FUGA D5`) |
      | Resetear los contadores al leerlos | D8 | rojo (`FUGA D8`) |

- [x] **T9 — Dos tests-guarda satisfechos declarando, no silenciando.**
      `TestToolReadOnlyClassification` y `TestEveryReadOnlyToolClassified` obligan a clasificar cada
      tool read-only. `musubi_cognition_stats` se declaró en `noScopedRead`: no lee ninguna tabla,
      así que no hay nada que acotar por proyecto.

- [x] **T10 — Golden y conteos**: 47 → 48. Regenerado con `-update`, más los cuatro tests y el
      README que fijan el número.

## Lo que la medición ya deja ver

Esta fase **no** confirma ni desmiente el "+20 pts" del roadmap: entrega el instrumento, no la
medición. Lo honesto es decirlo — un instrumento recién puesto no tiene serie histórica, y la única
forma de tener el número es dejarlo correr.

## Fuera de alcance

- **Exportar la telemetría.** Sin red, sin archivo, sin Prometheus. Persistirla trae preguntas
  propias: ¿por proyecto?, ¿cuánta retención?, ¿qué pasa con los tipos de secreto en disco?
- **Medir CALIDAD.** Se cuentan llamadas, hits y escaladas. Si el juez del recall *acierta* es otra
  medición, con corpus dorado, y ya tiene su gate (`recall-gate`).
- **Un cuarto nivel del dial.** Hay tres perillas reales; un dial que promete cinco ejes y mueve
  tres miente sobre lo que controla.
