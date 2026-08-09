# Spec — El canal arranca, o falla rápido

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
const defaultDialTimeoutSeg = 5
func clienteCerebro(timeoutSeg, dialSeg int) *http.Client
// flag nuevo: --dial-timeout
```

Las pruebas usan **198.51.100.1** (TEST-NET-2, RFC 5737): existe para documentación y nadie la
rutea, así que un intento de conexión se queda esperando — el escenario exacto del central apagado.
No se usa un puerto cerrado de localhost porque ahí el SO contesta `RST` al instante y la prueba
mediría otra cosa.

---

## H1 · El tiempo de un intento fallido lo elegimos nosotros

### C1 — El dial tiene su propio timeout, y manda

Con request en 60 s y dial en 1 s, un intento contra una IP que no rutea tiene que rendirse en
**segundos, no en los ~21 s del sistema operativo**. Es el corazón del arreglo: hasta ahora ese
número no lo había pactado nadie.

### C4 — Un dial ≤ 0 cae al default, no a «sin límite»

`net.Dialer{Timeout: 0}` significa **sin timeout**: exactamente el comportamiento que causó el
incidente. Un cero tiene que ser el default seguro, no la puerta de atrás que lo reintroduce.

Se verifica el **efecto** (cuánto tarda en rendirse), no el campo: un campo se puede leer bien y
estar cableado a otro dialer.

---

## H2 · El arranque entra en el presupuesto del host

### C2 — La secuencia completa, medida

No alcanza con que UNA request sea rápida: el incidente lo produjo la **suma** de las tres del
arranque. La prueba mide `initialize` + `notifications/initialized` + `tools/list` contra el número
real que dio el host —**60 s**— y exige entrar en la mitad.

El margen es deliberado: holgado para un runner lento, e imposible de pasar con los ~63 s del
comportamiento viejo.

---

## H3 · Separar los dos timeouts no rompe lo que ya andaba

### C3 — Un central vivo pero lento no se corta por el dial

Una vez establecida la conexión, el que manda es el timeout de **request**. Confundirlos rompería a
un central sano que tarda en devolver un `tools/list` grande — que es justo lo que el timeout de 60 s
existe para permitir.

### C5 — El timeout de request sigue vivo

Separar los dos no puede haber borrado el que ya estaba: un central que **acepta la conexión y
después se queda mudo** tiene que cortarse igual.

---

## Alcance declarado

- **Sin reintentos ni circuit breaker.** Con el central caído, cada llamada de la sesión sigue
  costando sus 5 s. Lo que este spec compra es que la sesión **arranque**.
- **El default de request no se toca** (60 s): cubre a un central vivo pero lento.
- **Los 5 s salen de la medición**, no del gusto: 3 forwards × 5 = 15 s, cómodo aun si el host
  apurara su presupuesto.
