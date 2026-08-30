# flota-en-vivo — design

## Piezas (todas sobre plomería existente)

| pieza | archivo | qué reusa |
|---|---|---|
| `SyncConfig.FlotaVivo *bool` + `FlotaVivoActivo()` | internal/config/config.go | el bloque `sync:` y su gate `HasDestination()` |
| `SyncClient.PushFlota(ctx, []LiveEvent)` | internal/mcp/syncclient.go | URL base + token ya resueltos, http.Client con timeout |
| `RunFlotaVivo(ctx)` — el remitente | internal/mcp/flota.go | `liveFeed.subscribe` (drop-contado incluido) |
| `handlerFlota(opt)` — el receptor | internal/mcp/flota.go + ruta en http.go | auth de `/api/stream` (registry.resolve / token plano), `clasificarTool`, `liveFeed.publish` |
| arranque | cmd/musubi/main.go | el mismo bloque que enciende los schedulers de sync |

## Decisiones

1. **El opt-in es la frontera del sync.** `FlotaVivo` es `*bool`: `nil` = activo cuando
   `HasDestination()` (la máquina ya cruza esa frontera con la memoria entera; los nombres de
   tools son estrictamente menos sensibles), `false` = apagado explícito. gio queda afuera solo.
2. **Batch por tamaño o por reloj**: flush a los 32 eventos o 2 s, lo que llegue primero. A un
   evento de trabajo cada varios segundos, esto es un POST cada 2 s como MUCHO, y casi siempre
   ninguno.
3. **Descartar, no encolar.** Si el POST falla, el lote se tira (log con rate-limit). El buffer
   del suscriptor (`liveSubBuf`) ya acota memoria y cuenta pérdidas. La durabilidad es del sync
   de memoria, no de la telemetría.
4. **No viaja el backlog del ring** al suscribirse: un restart del daemon no re-manda historia.
5. **El receptor re-sella todo lo interpretable**: principal/project del token, kind por
   `clasificarTool`, outcome normalizado a {ok, error}, tool contra `^musubi_[a-z0-9_]{1,64}$`
   (lo que no matchea se descarta esa fila), `at` se acepta si parsea RFC3339 y está a ±5 min
   del reloj del server — si no, se estampa ahora. Seq lo asigna el feed del central.
6. **`origen: "flota"`** en el evento publicado: el panel distingue «el central hizo» de «una
   terminal de la flota hizo» sin tocar el struct (el campo ya existía con "local"/"central").
7. **No-loop estructural**: el remitente sólo manda eventos `origen == "local"` (los suyos). Un
   evento `"flota"` publicado en el central jamás se re-reenvía aunque alguien encendiera el
   remitente ahí.
8. **Nada persiste en el central**: `handlerFlota` publica al feed y responde 202 con el conteo
   aceptado/descartado. El spool del central no se toca.

## Volumen

Medido 2026-08-26: 97.815/97.889 invocaciones locales en 24 h eran sondeo. Lo que viaja es la
diferencia: decenas de eventos por día por máquina, ~200 bytes cada uno.
