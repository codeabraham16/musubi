# Spec — Router de cognición + circuit breaker (F2)

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## Invariantes

### C0 — Un texto con secretos nunca llega a un motor de tier `free` *(el fundamental)*

Aunque el motor `free` esté **primero** en la flota, un prompt que contiene un secreto detectado no
llega a él: su portero lo corta antes de la red, y el router escala.

> Es la regla dura del roadmap, y es la razón de existir de la fase. Si sólo se pudiera verificar
> uno, es este.

### C1 — Orden y primera respuesta

El router prueba los motores **en el orden de la config** y devuelve la respuesta del primero que
conteste bien. No consulta a los siguientes.

### C2 — `ErrSecretsBlocked` no es una falla del motor

Un motor que se niega por política **no suma al contador del breaker**. Negarse es su trabajo, no un
síntoma de que esté roto: si contara, tres prompts con secretos seguidos apagarían un motor sano.

Se distingue de `ErrGatewayFailed` (falla técnica del portero), que **sí** cuenta.

### C3 — El breaker abre y saltea

Tras `failures` fallas **consecutivas**, el motor queda abierto y el router **no lo intenta** durante
`cooldown`. Una respuesta exitosa **resetea** el contador.

### C4 — Half-open: una sola prueba

Vencido el cooldown, el motor recibe **exactamente una** llamada de prueba. Si anda, el circuito se
cierra y vuelve a la rotación normal. Si falla, se vuelve a abrir por otro cooldown completo.

> Sin el "exactamente una", diez llamadas concurrentes al vencer el cooldown se van todas contra un
> motor que probablemente sigue caído.

### C5 — Agotamiento explícito, nunca crash

Si todos los motores están abiertos, fallan o se niegan, el router devuelve un error explícito
(`ErrAllEnginesDown`) y el caller degrada a model-free. **Nunca** entra en pánico ni cuelga.

Si el agotamiento fue por negativa de política y no por fallas, el error lo dice: `ErrSecretsBlocked`
envuelto, para que el caller distinga "no hay motor" de "no te lo mando".

### C6 — Bit-identidad sin flota

Sin `cognition.fleet` configurada, el comportamiento es **idéntico** al de F1: se construye el motor
único de siempre y no se instancia ningún router.

### C7 — Seguro entre goroutines

El estado del breaker se lee y escribe desde llamadas concurrentes (el server MCP despacha en
paralelo). Bajo `-race`, N goroutines contra el router no producen carreras, el circuito **abre** y
deja de admitir intentos nuevos, y el conteo no se pierde.

> **Lo que este invariante NO dice.** En estado cerrado el breaker no reserva turno —serializar las
> llamadas mataría el throughput—, así que bajo concurrencia los intentos contra un motor roto
> **pueden superar `failures`** antes de que alguna goroutine alcance a cruzar el umbral. Eso no es
> un bug: un circuit breaker acota los intentos *nuevos*, no los que ya están en vuelo.
>
> Vale anotarlo porque la primera versión de este test afirmaba `intentos <= failures` y se puso en
> rojo — la que estaba mal era la aserción, no el código.

---

## Configuración

```yaml
cognition:
  fleet:
    - name: groq
      provider: openai-compat
      endpoint: https://api.groq.com/openai/v1
      model: llama-3.3-70b-versatile
      auth_token_env: GROQ_API_KEY
      tier: free          # free (default) | private
    - name: max
      provider: openai-compat
      endpoint: http://127.0.0.1:4000/v1
      model: claude-sonnet-4-5
      auth_token_env: MUSUBI_COGNITION_TOKEN
      tier: private
  breaker:
    failures: 3           # default 3
    cooldown_seconds: 60  # default 60
```

**Defaults que protegen:**

| Cosa | Default | Por qué |
|---|---|---|
| `tier` de una entrada | `free` | Asumir "no confiable" es la dirección segura. Confiar se declara. |
| `gateway.mode` de un motor `free` | `refuse` | La regla dura del roadmap, hecha estructura. |
| `gateway.mode` de un motor `private` | `scrub` | Igual que en F1. |

Un `tier` desconocido es error de config. `fleet` vacía equivale a no tener flota (C6).

Declarar `gateway.mode: scrub` en un motor `free` **está permitido** —el texto sale tapado, así que
no hay fuga de credenciales— pero `musubi doctor` lo marca en amarillo: el resto de la memoria sigue
yendo a un servicio que entrena con lo que recibe, y eso es una decisión que conviene ver.

---

## Criterios de aceptación

1. Los 8 invariantes con test propio, cada uno verificado **fallando** al sabotear.
2. `go build`, `go vet`, `golangci-lint` y `go test ./...` en verde, más `go test -race` en el
   paquete del router (C7).
3. Cero cambios de comportamiento sin `fleet` (C6 cubierto por test).
4. El test de C4 no puede depender del reloj real: el tiempo se inyecta.
