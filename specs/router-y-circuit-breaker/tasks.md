# Tareas — Router de cognición + circuit breaker (F2)

Estado: **completo**. Build, vet, lint y las 17 suites en verde.

- [x] **T1 — Config**: `FleetEngineConfig`, `BreakerConfig`, `TierFree`/`TierPrivate`,
      `NormalizeTier`, `DefaultGatewayModeForTier`, y los defaults efectivos del breaker.

- [x] **T2 — `internal/cognition/breaker.go`**: breaker por motor con reloj inyectado. Tres estados
      implícitos (cerrado / abierto / half-open) y `probing` para el "exactamente una prueba".

- [x] **T3 — `internal/cognition/router.go`**: `router` como `Provider`, `ErrAllEnginesDown`,
      composición de la flota con la misma fábrica del motor único, e `inspectFleet` para el doctor.

- [x] **T4 — Cableado en `NewProvider`**: con flota manda el router; sin flota no se instancia nada
      nuevo y el camino de F1 queda bit-idéntico.

- [x] **T5 — Tests**: C0–C7 más los bordes (motor inválido, contexto cancelado, éxito que resetea el
      conteo, negativa durante la prueba half-open).

- [x] **T6 — Sabotaje**: 5 mutaciones. Cuatro en rojo al primer intento; una **no**, y ver abajo.

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | El tier gratis tapa y manda (no rechaza) | C0 | rojo (3 tests) |
      | Negarse cuenta como falla | C2 | rojo (2 tests) |
      | El breaker nunca abre | C3 | rojo (4 tests) |
      | El router se instancia siempre | C6 | rojo (4 tests, **incluidos los R6 de F1**) |
      | Sin reserva de prueba half-open | C4 | **verde** al primer intento → ver abajo |

- [x] **T7 — Docs**: `.musubi/config.example.yaml`, README, CHANGELOG.

## Dos correcciones que salieron de verificar

### El test de C7 afirmaba algo que el diseño nunca prometió

La primera versión exigía `intentos <= failures` bajo 50 goroutines, y se puso en rojo con 4 intentos.
**La que estaba mal era la aserción.** En estado cerrado el breaker no reserva turno —serializar las
llamadas mataría el throughput—, así que muchas goroutines entran antes de que alguna cruce el
umbral. Un circuit breaker acota los intentos *nuevos*, no los que ya están en vuelo.

Reescrito para afirmar lo que sí vale: que el circuito **abra**, que deje de admitir intentos, y que
el motor sano absorba el resto. La aclaración quedó escrita en la spec para que nadie la "arregle"
de nuevo.

### El test de C4 no sabía fallar

Sacar la reserva half-open **no puso nada en rojo**. El test lanzaba 10 goroutines al vencer el
cooldown y esperaba una sola prueba — pero la primera que falla vuelve a abrir el circuito antes de
que las otras lleguen a `allow()`, así que la ventana de carrera es diminuta y el test pasaba por
suerte de timing.

Se agregó un test determinista contra el breaker directamente: `allow()` dos veces seguidas sin
reportar resultado en el medio. Recién ahí el sabotaje se puso rojo. El test concurrente se conserva
como humo, pero **el que guarda el invariante es el determinista**.

Es el tercer test flojo que encuentra el sabotaje en este track. El patrón se repite: **un test
concurrente que "pasa" no prueba exclusión mutua** — hay que probarla secuencialmente.

## Verificado por el CI, no por mí

`-race` **no corre en esta máquina** (requiere cgo y no hay gcc). El job canónico del CI corre
`go test -race ./...` en ubuntu, así que C7 lo verifica el CI. Se anota para no dar por hecho algo
que no se ejecutó localmente.

## Fuera de alcance, dicho de frente

- **Presupuesto diario por proveedor.** Necesita estado durable entre reinicios; en memoria mentiría
  al reiniciar. Va con la telemetría (F5).
- **Caché semántico**: F3.
- **Elegir tier por dificultad de la consulta** (el "dial de potencia"): F5. Acá el orden lo fija el
  usuario; lo único automático es saltear lo roto y lo que se niega.
- **Un motor lento no es un motor caído.** El breaker cuenta errores, no latencia; a la lentitud la
  acota el `request_timeout_seconds` que ya existía.
