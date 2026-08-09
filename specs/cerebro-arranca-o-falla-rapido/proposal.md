# Propuesta — El canal arranca, o falla rápido

Nace de un **incidente real**, no de una idea: con el cerebro central caído por un corte de luz, a
otra máquina del equipo **no le arrancaba Claude Code entero**.

## El síntoma, tal como se vio

```
Error: Subprocess initialization did not complete within 60000ms
       — check authentication and network connectivity
[info] OAuth tokens found in secure storage
```

El mensaje culpa a la autenticación y a la red. **Las dos estaban bien.** Medido en esa máquina:
`api.anthropic.com:443` alcanzable, tokens presentes, 19,76 GB de RAM libres y cero procesos
colgados. El diagnóstico que sugiere el error habría mandado a revisar credenciales durante horas.

## La causa, cronometrada

`musubi cerebro` es un MCP server por stdio que reenvía cada llamada al central por HTTP. Con el
central apagado, **cada request paga el timeout de conexión del sistema operativo** (~21 s en
Windows), y el arranque de MCP hace tres:

```
initialize                 21 s
notifications/initialized  21 s   ← nadie espera su respuesta, y cuesta igual
tools/list                 21 s
                         ------
                           63 s   contra los 60 s que el host da para inicializar
```

**Ninguna request sola llegaba al timeout de 60 s configurado**: el que las sumaba era el arranque.
Margen de fallo: **3 segundos**.

## Por qué importa más de lo que parece

El precio no es perder el canal federado —eso sería aceptable y esperable con el central caído—
sino que **la sesión entera no levanta**. Un servidor MCP inalcanzable tiene que degradar a «ese
servidor no está», nunca tumbar al agente.

Y la falla es **remota y silenciosa**: apagar el central deja sin herramienta a cualquier máquina
del equipo que tenga el canal configurado, con un mensaje que apunta al lado equivocado.

## Qué se construye

Un **timeout de dial propio**, separado del de request. Son dos preguntas distintas y hoy las
gobernaba una sola: *«¿cuánto espero a que el otro extremo conteste?»* (request, 60 s) y *«¿cuánto
espero para saber si siquiera está?»* (dial, hasta ahora el del SO).

Con 5 s de dial, el mismo arranque cuesta **~15 s** — medido, no estimado.

## Lo que NO se construye

- **No hay reintentos ni circuit breaker.** Con el central caído cada llamada de la sesión seguirá
  costando sus 5 s. Es aceptable y es otro spec: lo que este arreglo compra es que la sesión
  **arranque**.
- **No se cambia el default de request (60 s).** Cubre a un central vivo pero lento, que es un caso
  legítimo y distinto.
- **No se toca nada en la máquina afectada.** El arreglo es del binario; ahí no había nada roto.
