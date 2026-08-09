# Spec — El motor tiene freno propio

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// config
func (c CognitionConfig) EffectiveMotorQuotaPerHour() int

// mcp
func WithMotorQuota(perHour int) Option

// memory
const OutcomeDeniedMotor = "denied_motor"
```

---

## H1 · El presupuesto se cobra donde se gasta, y sólo ahí

### M1 — `musubi_ask` sin presupuesto se RECHAZA

Error explícito con el código de cuota. Quien llamó pidió una respuesta razonada del motor:
devolverle un texto armado de otra forma, o una respuesta vacía con cara de éxito, sería mentirle.

### M2 — `musubi_recall` sin presupuesto DEGRADA, no falla

Devuelve el orden model-free y `error == nil`. Quien llamó pidió memoria, no un juez. Es la misma
regla que ya rige cuando el motor se cae o el juez devuelve basura — un límite de gasto es otra
forma de que el juez no esté disponible.

### M3 — Con el juez apagado, el recall no consume presupuesto

Es el modo por default. Un recall model-free no llama al motor, así que no puede gastar: si
consumiera cuota, el freno estaría estrangulando una tool gratis.

### M4 — Un acierto de caché no consume presupuesto

El caché de rerank existe para no llamar al motor. Cobrarle a una respuesta memoizada contaría gasto
que no ocurrió, y agotaría el presupuesto justamente en el caso en que el sistema se está portando
bien.

### M5 — Rechazar no gasta

El presupuesto se consulta **antes** de la llamada al motor. Una llamada rechazada no puede haber
costado nada.

---

## H2 · El freno es por principal y no cambia las reglas existentes

### M6 — El presupuesto es POR PRINCIPAL

Agotar el de uno no toca al otro. Si fuera global, un cliente desbocado dejaría sin motor a todos —
que es el fallo que este spec viene a evitar, no a mover de lugar.

### M7 — Sin principal no hay freno

En stdio local no hay identidad contra la cual contar, exactamente como la cuota general. Un freno
que se aplicara a una clave vacía sería un límite compartido por todos los usos anónimos.

### M8 — Cero ⇒ default; negativo ⇒ sin límite

La MISMA semántica que `quota_per_minute`, y por eso mismo se prueba: dos números de configuración
que se parecen y significan cosas distintas es la forma más barata de que alguien se apague el freno
creyendo que lo apretaba.

---

## H3 · Nada de esto pasa en silencio

### M9 — El rechazo de `musubi_ask` queda en el ledger con outcome propio

`denied_motor`, distinguible de `denied_quota`. Si compartieran outcome, sería imposible responder
«¿me frenó el límite general o el del motor?» — que es la única pregunta que se hace cuando algo
deja de andar.

### M10 — La degradación del recall queda contada

El recall devuelve `ok` y el usuario no ve nada raro: si además no quedara registro, el sistema
estaría dejando de usar el juez sin que nadie pueda enterarse. Un contador en `/metrics` y una línea
de log.

---

## Alcance declarado

- **Se cuentan LLAMADAS, no tokens ni dinero.** Es lo que el sistema puede saber por sí mismo, sin
  depender de que un proveedor reporte bien ni de una tabla de precios que envejece.
- **En memoria, como la cuota general.** Se pierde al reiniciar. Un freno que sobreviva a los
  reinicios necesita persistencia y es otro spec; éste acota el daño de un bucle, que es el riesgo
  real.
- **No hay capability nueva.** Ver la propuesta.
- **El juez sigue apagado.** Esto es la precondición para encenderlo, no el encendido.
