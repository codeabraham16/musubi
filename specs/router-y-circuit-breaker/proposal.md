# Propuesta — Router de cognición + circuit breaker (F2)

## Por qué ahora

F1 y F1.5 pusieron porteros en las dos fronteras por las que el texto sale de Musubi. Pero hoy el
único motor real conectado es el endpoint Max en loopback, que es **privado**: la frontera existe y
casi nadie la cruza.

F2 es la fase que hace que la cruce de verdad — conectar la flota gratis (Groq, NVIDIA, Cerebras) —
y a la vez la que decide **qué puede ir a dónde**. Sin router, usar la flota gratis sería exponer la
memoria entera a servicios que **entrenan con lo que reciben**.

## La idea que hace que esto sea corto

El roadmap pide una regla dura: *lo marcado como secreto JAMÁS va al tier gratis; en duda, al Max o a
model-free*. La forma obvia de implementarla sería que el router inspeccione el texto y decida.

**No hace falta.** F1 ya dejó la primitiva, y encaja mejor de lo que parece:

- un motor de tier `free` se configura con `gateway.mode: refuse`;
- si el texto lleva un secreto, ese motor devuelve `ErrSecretsBlocked` **sin haber mandado nada**;
- el router trata ese error **no como una falla del motor, sino como una señal de ruteo**: escala al
  siguiente tier.

Así la regla dura no es un `if` que alguien pueda olvidar de escribir: es una consecuencia de que el
portero del motor barato esté en `refuse`. El router no necesita saber qué es un secreto — sólo
necesita saber qué hacer cuando un motor dice "esto no lo mando".

## Qué se propone

1. **Flota ordenada** en vez de un motor único: una lista de motores con `tier` (`free` | `private`).
   El default de `tier` es **`free`** (asumir no confiable es la dirección segura), y el default del
   portero de un motor `free` es **`refuse`**.
2. **Router** que es él mismo un `Provider`: prueba en orden y devuelve la primera respuesta. Todo lo
   que está río arriba no se entera de que hay más de un motor.
3. **Circuit breaker por motor**: N fallas seguidas lo abren; queda fuera de la rotación por un
   cooldown; después entra una sola prueba (half-open). Un motor caído no se reintenta en cada
   llamada.
4. **Agotamiento explícito**: si todos los motores están abiertos o fallan, se devuelve un error que
   el caller ya sabe manejar degradando a model-free. Nunca se rompe.

## Qué NO se propone

- **Presupuesto diario por proveedor.** Necesita estado durable entre reinicios; va con la telemetría
  (F5), no acá.
- **Caché semántico.** Es F3.
- **Elegir tier por dificultad de la consulta** (el "dial de potencia"). Es F5. Acá el orden lo fija
  el usuario en la config; lo único automático es *saltear lo que está roto o lo que se niega*.
- **Cambiar el contrato de `Provider`.** El router es un `Provider` más: nada río arriba se toca.
