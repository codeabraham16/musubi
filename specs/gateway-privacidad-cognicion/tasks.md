# Tareas — Gateway de privacidad para la cognición (F1)

Estado: **completo**. Build, vet y las 17 suites del repo en verde.

- [x] **T1** — `internal/privacy/session.go`: `Session`, `NewSession`, `Scrub`, `Restore`,
      `Count`, `Findings`, `Types`. Detección delegada a `redact.Redact`. Sustitución en reversa.
      Acuñación de marcador con reintento anti-colisión (R5).

- [x] **T2** — `internal/privacy/session_test.go`: 18 casos de round-trip (R1), marcador inventado
      (R2), estabilidad e inyectividad (R3), colisión simple y múltiple (R5), y adversariales:
      vacío, pegados, repetidos, bordes, multilínea, acentos y emoji, prosa larga.

- [x] **T3** — `internal/config`: `GatewayConfig` dentro de `CognitionConfig`.

- [x] **T4** — `internal/cognition/gateway.go`: decorador `guarded`, `ErrSecretsBlocked`,
      `ErrGatewayFailed`, `scrubPrompt` con `recover`, y cableado en `NewProvider` vía
      `newBaseProvider` (imposible construir un motor real sin envolver).

- [x] **T5** — `internal/cognition/gateway_test.go`: R0 con motor espía, R4 (`refuse` no invoca),
      R6 (Noop intacto), R7 (`off` explícito / modo desconocido rompe), más delegación de `Name`,
      propagación de errores y no-mezcla de sesiones entre llamadas.

- [x] **T6** — `go build ./...`, `go vet ./...`, `go test ./...` en verde (17 paquetes).

- [x] **T7** — **Sabotaje**: 10 mutaciones aplicadas, cada una puso en rojo el test de su invariante.

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Sustituir de izquierda a derecha | R1 | rojo |
      | `Scrub` no tapa nada | R0 | rojo (`FUGA R0`) |
      | `Restore` resuelve cualquier marcador | R2 | rojo |
      | Sin chequeo de colisión | R5 | rojo |
      | `refuse` no corta | R4 | rojo |
      | Envolver también al Noop | R6 | rojo |
      | Sacar el `recover` de `scrubPrompt` | R4 (técnico) | rojo — el pánico tumba el proceso |
      | `off` se reporta como `ok` | R7 / doctor | rojo (3 tests, incluido el de coherencia) |
      | Modo inválido con el pilar apagado pasa en silencio | doctor | rojo |
      | El seam de test se come el camino de producción | R0 | rojo (`FUGA R0`) |

- [x] **T8** — Auditoría adversarial. Tres mejoras aplicadas a partir de ella: `recover` para que un
      pánico no tumbe el daemon MCP, camino rápido en la detección de colisión, y aviso explícito de
      que los offsets de `Findings()` son por-texto. Un hallazgo abierto documentado: los embeddings
      son una segunda salida que esta fase no cubre.

- [x] **T9** — CHANGELOG actualizado.

## Cierre de la fase (huecos detectados al auditar la propia F1)

- [x] **T10 — La feature era invisible.** `cognition.gateway.mode` no aparecía en ninguna doc: sólo
      en el código y en esta spec. Documentado en `.musubi/config.example.yaml` (bloque `cognition:`
      completo, con el porqué de cada modo) y en el README (sección «El portero de privacidad», con
      el ejemplo del round-trip y la tabla de modos). El README además listaba `sync` como bloque
      inexistente: corregido de paso.

- [x] **T11 — El portero no se veía en ningún diagnóstico.** `musubi doctor` no mencionaba la
      cognición, así que apagar la guarda sólo dejaba rastro en el log de arranque del daemon. Nuevo
      check `cognition_gateway`: `cognition.InspectGateway(cfg)` decide el estado y
      `cmd/musubi/doctor.go` lo traduce a `memory.CheckResult`.

      Vive en el CLI y no en el registry de `internal/memory` a propósito: esos checks diagnostican
      la BASE DE DATOS, y el portero es una propiedad de la CONFIG — meterlo ahí obligaría a que la
      memoria conozca la cognición.

      `InspectGateway` reusa `newBaseProvider` y `normalizeGatewayMode` (extraída para ser la única
      fuente de verdad sobre qué modos existen), así el diagnóstico no puede divergir de lo que el
      constructor realmente hace. Hay un test que verifica exactamente esa coherencia.

      Estados: pilar apagado ⇒ `ok`; apagado con modo mal escrito ⇒ `warning` (el error aparece hoy y
      no el día que lo enciendan); `scrub`/`refuse` ⇒ `ok`; `off` ⇒ **`error`**; modo o provider
      desconocido ⇒ `error`.

      Decisión consciente: `off` es rojo **permanente** mientras esté puesto. El proyecto evita las
      alarmas siempre-encendidas porque enseñan a ignorar el rojo — pero ésas son alarmas *falsas*.
      Ésta es verdadera y el riesgo es continuo, así que corresponde que moleste; se apaga quitando
      una línea de config.

- [x] **T12 — El `recover` no tenía test.** Era el único invariante verificado sólo por inspección.
      `guarded` ahora lleva un campo `newSession func() scrubSession` (nil ⇒ `privacy.NewSession`,
      el único camino en producción) para que el test pueda inyectar una sesión que entra en pánico.

      Es un campo y no una variable de paquete para no introducir estado global mutable entre tests.
      El seam trae su propio riesgo —que se coma el camino real— y por eso tiene test propio; al
      sabotearlo, el que se puso en rojo fue el invariante fundamental R0.

## Fuera de alcance, dicho de frente

- **Embeddings** (`internal/embedding`, providers `ollama` y `openai-compatible`): segunda superficie
  de salida. Necesita su propia fase porque el contrato es distinto — un embedder devuelve un vector,
  no texto, así que no hay nada que rehidratar y la política correcta es `refuse` o embedder local.
- **Datos sensibles sin forma de secreto** (nombres de clientes, decisiones de negocio): es juicio
  semántico, no model-free. Corresponde a la política de router de F2.
- **Telemetría del portero** (cuántos secretos se taparon, de qué tipo, con qué frecuencia): sin
  medición no se sabe si el portero actúa seguido o nunca. Corresponde a F5, que es la fase de dial
  + telemetría.
