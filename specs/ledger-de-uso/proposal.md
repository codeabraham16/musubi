# Propuesta — Ledger de uso (F0 del track «Potencia medida»)

## El problema, en una frase

Musubi no puede responder **cuáles de sus 50 herramientas se usan**, así que toda decisión sobre
dónde invertir potencia se toma por opinión.

## Cómo apareció

Auditando qué capacidad estaba construida y apagada. La respuesta a la mitad de la pregunta fue
fácil —la config es generosa y casi todo nace encendido— pero la otra mitad resultó **imposible de
responder con los instrumentos actuales**:

| Instrumento | Por qué no sirve |
|---|---|
| Histograma por-tool en `observability.go` | Vive en memoria (`atomic.Int64`). **Se resetea en cada reinicio.** |
| `GET /metrics` | Pide bearer, y **el modo daemon stdio ni siquiera levanta HTTP** — o sea, el 99 % del uso no tiene dónde exponerlo. |
| Tabla `telemetry_logs` | El nombre engaña: guarda errores de compilación de `musubi_log_error`. |

La ironía vale decirla: **un producto de memoria que no recuerda qué hace.**

## Por qué esto va primero y no el juez del recall

Porque sin medidor no hay forma de saber si el juez mejoró algo. El plan del track lo pone como F0
justamente para no repetir el patrón que la auditoría contra FounderOS dejó medido: Musubi
**construye capacidad más rápido de lo que la consume** — 1 256 líneas de motor DAG con cero
workflows, `Required` declarado y no aplicado, 145 campos de config sin validar. Nada de eso se ve
porque nada lo contradice con datos.

El ledger es la primera vez que el proyecto se aplica a sí mismo la vara que ya le exige a todo lo
demás: umbrales calibrados sobre 77 028 pares, `recall-gate` en CI.

## Qué NO es

- **No es tracing ni APM.** No hay spans, ni contexto distribuido, ni dependencias nuevas.
- **No registra argumentos ni contenido.** Sólo el nombre de la tool y sus métricas. Un ledger que
  guardara los argumentos sería la peor fuga posible en un sistema cuyo trabajo es acumular
  conocimiento sensible.
- **No reemplaza a `/metrics`.** Los contadores en memoria siguen sirviendo para scrapeo en vivo;
  esto agrega la historia que sobrevive al reinicio.
- **No cambia el comportamiento de ninguna tool.** Es observación pura.

## La idea

Un punto de estrangulamiento, una tabla, y una lectura.

```
handleToolsCall  ──▶  buffer en memoria  ──▶  flush periódico  ──▶  tool_invocations
   (el ÚNICO camino          (append bajo         (goroutine,           (persistente,
    de toda tool)             mutex, µs)           sin dispatchMu)       con retención)
```

El registro va en `methods.go:handleToolsCall`, que es por donde pasa **toda** llamada — stdio y
HTTP, éxitos, errores, y los rechazos por rol y por cuota. Eso hace la cobertura **estructural**: no
depende de que quien escriba la próxima tool se acuerde de instrumentarla. Es la misma decisión que
en F1 del pilar de cognición hizo imposible construir un motor sin portero.

## Costo y reversibilidad

Una migración aditiva, un buffer con su goroutine de flush, y una tool de lectura. Sin dependencias
nuevas, sin red, sin cambios en el camino de datos de ninguna tool. Si el ledger se apaga o falla,
todo sigue funcionando exactamente igual — es la invariante L2.
