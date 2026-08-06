# Spec — Ledger de uso

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**: si se rompe la propiedad,
el test se pone rojo. Un invariante sin test que lo pueda tumbar es decoración.

---

## Alcance

| Superficie | Estado |
|---|---|
| `tools/call` (stdio y HTTP) | **en alcance** — toda invocación |
| Rechazos por rol y por cuota | **en alcance** — son el dato más interesante, no un caso borde |
| `initialize`, `tools/list` y demás métodos JSON-RPC | fuera: no son trabajo, son protocolo |
| Los contadores en memoria de `/metrics` | fuera: se quedan como están, sirven para scrapeo en vivo |
| Tokens y costo de las tools que llaman al LLM | fuera de F0 — se suma cuando haya de dónde leerlos |

---

## Invariantes

### L0 — Ninguna llamada escapa al ledger *(el invariante fundamental)*

Toda invocación de `tools/call` queda registrada: las que devuelven bien, las que devuelven error, y
las **rechazadas por rol o por cuota** antes de llegar al handler.

> La garantía es **estructural, no de disciplina**: el registro vive en el único punto por el que
> pasan todas. La única forma de escribir una tool que no quede registrada sería no pasar por
> `handleToolsCall`, que es una desviación visible en el diff.

### L1 — El ledger nunca guarda argumentos ni contenido

Se registran el nombre de la tool, la duración, el resultado, el proyecto y quién llamó. **Nunca**
el `arguments` crudo, ni el texto de una observación, ni una consulta de recall.

> Es lo que hace al ledger seguro de leer, de exportar y de mirar desde el cuerpo. Un registro de
> invocaciones con los argumentos adentro sería una segunda copia de toda la memoria sensible del
> sistema, sin ninguna de sus murallas — y `save_observation` recibe exactamente el contenido que el
> portero de privacidad existe para proteger.

### L2 — Registrar no puede hacer fallar una llamada

Si el ledger no puede escribir —disco lleno, base bloqueada, lo que sea— la tool responde igual y el
resultado es idéntico. Best-effort total, con el fallo en el log y no en la respuesta.

### L3 — Funciona en modo stdio

El daemon por stdin/stdout —que es como se usa el 99 % del tiempo— registra igual que el modo HTTP.
Es exactamente el hueco que deja `/metrics`, y el motivo de existir de esta fase.

### L4 — Sobrevive al reinicio

Los datos están en la base, no en memoria. Reiniciar el daemon no pierde la historia.

> Esta es la diferencia entera contra lo que ya existía. Sin esto no hay fase.

### L5 — El camino caliente no espera al disco

El handler sólo hace un append en memoria bajo mutex. La escritura a la base ocurre después, en una
goroutine aparte, **sin tomar `dispatchMu`** — tomarlo re-entraría el lock que el handler todavía
tiene y sería un deadlock, la misma trampa que `maybeTriggerMaintenance` documenta.

### L6 — El crecimiento está acotado

El ledger tiene retención y purga. Una tabla de telemetría que crece sin techo termina siendo el
problema que vino a diagnosticar, y encima en un sistema que se autodiagnostica con `doctor`.

### L7 — Lo que se pierde al morir, se dice

Un flush pendiente se pierde si el proceso muere de golpe. Es una decisión aceptada —telemetría, no
libro mayor— y por eso el ledger **no promete completitud absoluta**: promete no perder nada por
diseño y perder como mucho una ventana de flush por accidente.

---

## Configuración

Un solo bloque, y nace **encendido**:

```yaml
usage_ledger:
  enabled: true          # observación pura, sin efecto sobre ninguna tool
  flush_interval_seconds: 10
  retention_days: 90
```

Nace encendido porque no tiene contraindicación —no cambia comportamiento, no manda nada afuera, no
guarda contenido— y porque un medidor disponible-para-apagar termina apagado, que es literalmente el
problema que esta fase viene a arreglar.

---

## Criterios de aceptación

1. Los 8 invariantes con test propio, y cada test verificado **fallando** al sabotear la
   implementación.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde.
3. Una base vieja migra sin perder filas.
4. Test adversarial: la base cae en medio de un flush, un rechazo por cuota, una tool que entra en
   pánico, y el ledger apagado por config.
5. **La métrica de cierre del track**: tras uso real, poder listar las tools ordenadas por
   invocaciones y por latencia p95 — y que el motor DAG quede con veredicto por dato.
