# Diseño — Ledger de uso

## El punto de estrangulamiento

`internal/mcp/methods.go`, dentro de `handleToolsCall`. Por ahí pasa **toda** llamada a tool, de los
dos transportes, y ya hay instrumentación en memoria:

```go
start := time.Now()
result, rpcErr := handler(ctx, callReq.Arguments)
if s.metrics != nil {
    s.metrics.recordTool(callReq.Name, time.Since(start), rpcErr == nil)
}
```

El ledger se engancha al lado de esa línea. Y —esto es lo que L0 exige y lo que la instrumentación
actual **no** hace— también en los dos `return` tempranos de más arriba: el rechazo por rol
(`p.canCall`) y el rechazo por cuota (`s.quota.allow`). Hoy esos dos incrementan un contador
agregado y se pierde de qué tool se trataba.

## Por qué el buffer, y por qué la goroutine no toma el lock

El handler corre **con `dispatchMu` tomado** (write-lock para las tools que mutan, read-lock para
las de lectura). Dos consecuencias que fijan el diseño:

1. **Escribir a la base ahí adentro alargaría el lock** en el camino caliente de cada tool.
2. **La goroutine de flush no puede tomar `dispatchMu`.** Es la misma trampa que
   `maybeTriggerMaintenance` documenta: el handler todavía lo tiene, y re-entrarlo es deadlock.

Por eso: el handler hace un `append` a un slice bajo un mutex propio (microsegundos, sin disco), y
una goroutine aparte drena ese slice y hace un INSERT por lote **directo contra la base**, sin pasar
por `dispatchMu`.

Eso es seguro porque la base se abre con `busy_timeout(5000)` y `journal_mode(WAL)` en el DSN
(`internal/memory/database.go:107`), con un pool de 8 conexiones: SQLite serializa los escritores y
espera hasta 5 s antes de rendirse. Y si se rinde, L2 dice que no pasa nada.

## Esquema

Migración **v23**, aditiva, tabla nueva:

```sql
CREATE TABLE IF NOT EXISTS tool_invocations (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  tool        TEXT    NOT NULL,
  outcome     TEXT    NOT NULL,   -- ok | error | denied_role | denied_quota | panic
  duration_us INTEGER NOT NULL,
  project_id  TEXT    NOT NULL DEFAULT '',
  principal   TEXT    NOT NULL DEFAULT '',
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_tool_invocations_tool ON tool_invocations(tool);
CREATE INDEX IF NOT EXISTS idx_tool_invocations_ts   ON tool_invocations(created_at);
```

**Nótese lo que NO está**: no hay columna de argumentos, ni de resultado, ni de mensaje de error.
Eso es L1, y es una decisión de esquema y no de código — la fuga es imposible porque no hay dónde
escribirla. `outcome` es una taxonomía cerrada de cinco valores, no texto libre, por la misma razón
que la procedencia de F4: un mensaje de error puede llevar adentro el contenido que lo causó.

**Los dos índices son a propósito.** Las dos únicas consultas del ledger son «agrupá por tool» y
«dame la ventana reciente / purgá lo viejo». Sin ellos, la purga hace scan completo sobre la tabla
que más crece.

## Retención

La purga cuelga del mantenimiento que ya existe (`scheduler.go`), no de un timer nuevo: borra lo más
viejo que `retention_days`. Un solo `DELETE ... WHERE created_at <` con el índice de fecha.

## La lectura

Una tool nueva, `musubi_tool_usage`, que devuelve por tool: invocaciones, tasa de error, latencia
media y p95, y última vez que se usó. Con ventana configurable en días.

**Sus dos consumidores están declarados** —la regla del track exige eso antes de escribir nada—: el
agente, que puede preguntarse qué está usando de verdad, y el cuerpo, que en F5 va a tener el panel.
La p95 se calcula en SQL sobre `duration_us`; no hace falta histograma en la base porque los datos
crudos están.

## Qué NO se toca

Los contadores en memoria de `observability.go` se quedan exactamente como están. Sirven para
scrapeo en vivo por Prometheus y son baratos; el ledger agrega la historia, no la reemplaza. Duplicar
la información en dos lugares es aceptable acá porque responden preguntas distintas: uno «qué está
pasando ahora», el otro «qué pasó estas semanas».

## Riesgos, dichos de frente

- **El ledger mide todo menos a sí mismo.** El costo del `append` es real aunque sea de
  microsegundos. Se acota midiéndolo en el test de L5 y no prometiendo cero.
- **Un flush pendiente se pierde si el proceso muere de golpe** (L7). Aceptado: es telemetría, no
  el libro mayor. Bajar la ventana de flush reduce la pérdida y sube el costo de escritura; 10 s es
  un default elegido, no medido.
- **`principal` es un nombre de persona o máquina.** En el central eso responde «quién usa qué», que
  es justamente lo que se quiere; pero es un dato de atribución y hay que saber que está ahí antes
  de exportar el ledger a ningún lado.
- **Una tool nueva sube el catálogo de 50 a 51.** Hay cuatro lugares que fijan ese número
  (`server_test.go`, `http_test.go`, `dispatch_concurrent_test.go` y el golden) y el README, que ya
  arrastra un error propio: titula «50 herramientas» y enumera 46.
