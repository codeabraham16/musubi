# Tareas — Portero de privacidad para los embeddings (F1.5)

Estado: **completo**. Build, vet, lint y las 17 suites del repo en verde.

- [x] **T1 — Verificar el hueco antes de taparlo.** F1 afirmaba que `redact` mitigaba parcialmente
      esta superficie. Al leer el código, los cuatro puntos de redacción resultaron condicionados a
      `scope == shared` o a `forceRedact` (que es sólo el central). En un workspace local **todo sale
      crudo**, y las **queries no están protegidas en ninguna configuración**. La afirmación de F1
      queda corregida en su propia spec y en el PR.

- [x] **T2 — `config.NormalizeGatewayMode` + constantes de modo.** Movidas desde
      `internal/cognition` a `internal/config`, que es de quien dependen los dos pilares con portero.
      `internal/cognition` pasa a delegar y sus constantes son alias, no copias (E6).

- [x] **T3 — `Gateway GatewayConfig` en `EmbeddingConfig`.** El mismo tipo que usa la cognición:
      mismos modos, misma semántica, misma validación.

- [x] **T4 — `internal/embedding/gateway.go`.** Decorador `guarded`, `needsGateway`,
      `ErrSecretsBlocked`, `ErrGatewayFailed`, `scrubText` con `recover`, `InspectGateway`, y
      cableado en `NewProvider` vía `newBaseProvider` (imposible construir uno sin envolver).

      No usa `internal/privacy`: sin respuesta de texto no hay nada que rehidratar, así que alcanza
      con `redact.Redact`, que además es determinista (E1).

- [x] **T5 — `internal/embedding/gateway_test.go`.** E0 con embedder espía (contenido **y query**),
      E1 determinismo, E2 imposibilidad de obtener uno con red sin portero, E3 `refuse` + pánico,
      E4 `none`/`static` intactos, E5 `off` y modo desconocido, E6 modos compartidos, más texto
      limpio sin alterar y propagación de errores del backend.

- [x] **T6 — `TestNewProviderFactory` endurecido.** Pasaba igual tras el cambio porque sólo afirmaba
      sobre `Name()`, que el portero delega — o sea, ya no verificaba que abajo hubiera un motor
      real. Ahora afirma el envoltorio **y** el motor interno.

- [x] **T7 — `musubi doctor`.** Los dos porteros se diagnostican juntos (`configChecks`): nuevo check
      `embedding_gateway`, mismos estados y mismo criterio de rojo que `cognition_gateway`.

- [x] **T8 — Sabotaje**: 6 mutaciones, cada una en rojo.

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Detectar pero no tapar | E0 | rojo (3 tests, `FUGA E0`) |
      | Tapado no determinista | E1 | rojo |
      | `ollama` sale de la fábrica sin portero | E2 | rojo (2 tests, incluido el de fábrica) |
      | Sacar el `recover` | E3 | rojo — el pánico tumba el proceso |
      | Envolver también a `static` | E4 | rojo |
      | La cognición se guarda un modo propio | E6 | rojo **al segundo intento** (ver abajo) |

- [x] **T9 — Docs**: `.musubi/config.example.yaml`, README, CHANGELOG.

## El test que no sabía fallar

El primer test de E6 enumeraba nueve modos. Al sabotearlo —haciendo que la cognición aceptara un
modo propio, `"silencioso"`, que `config` no conoce— **el test pasó limpio**: la palabra inventada no
estaba en la lista.

Un test que enumera casos sólo prueba los casos que enumera. Se reescribió contra un corpus de
palabras que un dev podría elegir de verdad (`silencioso`, `quiet`, `disabled`, `auto`, `apagado`…)
más 400 cadenas pseudoaleatorias con semilla fija (reproducible en CI). Recién ahí el sabotaje se
puso en rojo.

Vale anotarlo porque es el segundo caso de la misma clase en este track: **la verificación se
verifica sola sólo si el sabotaje es de verdad**.

## Fuera de alcance, dicho de frente

- **Re-embeber lo ya indexado.** Los vectores viejos salieron de texto crudo; sólo difieren para
  textos con secretos. Se corrige con `musubi embed backfill`, y no se hace automático: recalcular
  el índice entero es una decisión del usuario, no un efecto colateral de actualizar.
- **Falsos positivos del detector de entropía.** Una cadena legítima de alta entropía tapada degrada
  su recuperación semántica. Limitación **heredada** del detector, idéntica a la que ya afecta al
  camino de guardado compartido.
- **Datos sensibles sin forma de secreto.** Juicio semántico, no model-free.
- **Telemetría de los dos porteros** (cuántos secretos se taparon y de qué tipo): F5.
