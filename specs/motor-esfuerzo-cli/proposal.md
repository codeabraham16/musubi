# Propuesta — `motor`: propagar el nivel de esfuerzo a `claude -p`

## El problema

El selector de esfuerzo (reasoning) del cliente —pi, y por lo tanto el cuerpo— es un **botón
muerto** cuando el turno lo sirve el Lanzador. pi manda `options.reasoning` en cada pedido
(`low`/`medium`/`high`/`xhigh`/`max`, más `minimal`), y el motor lo recibía y lo **tiraba**: sólo lo
leía `presupuestoDeThinking`, que resuelve un budget en tokens para la Messages API. El Lanzador
—`claude -p`, el motor de SUSCRIPCIÓN, que es el que corre en producción porque la API está vedada
por el guardarraíl de los 400— no miraba ese dato por ningún lado. El usuario movía el selector y el
turno salía idéntico.

`claude -p` sí sabe modular el razonamiento: acepta `--effort` (medido el 2026-11-07 contra este
`claude.exe`, `claude --help`), con cinco niveles **cualitativos** — `low`, `medium`, `high`,
`xhigh`, `max`. No toma un budget en tokens por bandera; toma el nivel. O sea que el dato que el
cliente ya mandaba tenía una puerta de entrada al Lanzador, sin usar.

## El objetivo

Que el Lanzador **honre** el nivel de razonamiento del cliente: tomar `options.reasoning`,
normalizarlo al vocabulario de `--effort`, y emitirlo en el argv de `claude -p`. Con eso el selector
del cuerpo pasa a modular de verdad el turno.

## Alcance

- Un campo nuevo `Esfuerzo` en `Pedido` (`motor.go`), poblado en `PedidoDesdeProxy` (`entrada.go`).
- Una función `esfuerzoDesdeReasoning` (`entrada.go`) que normaliza el nivel del cliente al de
  `--effort`.
- La emisión de `--effort <nivel>` en `argumentosCLI` (`lanzador.go`), después de `--model`.
- Del lado del cuerpo (repo thinkrail, fuera de este repo): `reasoning: true` en los seis modelos del
  provider `pi-musubi`, para que la UI de pi muestre el selector y mande `options.reasoning`.

## Fuera de alcance

- **El ClienteAPI no cambia.** Su normalización (`presupuestoDeThinking`) resuelve un budget en
  tokens y ya existía; Esfuerzo es su contracara para el Lanzador, no la reemplaza.
- No se toca el protocolo con pi: `options.reasoning` ya viajaba.
- No se agrega configuración: el nivel sale del pedido, no de un yaml.

## Por qué importa el detalle de la normalización

`--effort` **rechaza** un valor que no conoce y **mata el turno** (el CLI no lo ignora). Eso vuelve
la normalización una cuestión de correctitud, no de prolijidad: un `--effort ""` o un `--effort
minimal` crudo no son no-ops, son turnos muertos. De ahí las tres reglas de `esfuerzoDesdeReasoning`
—passthrough de los cinco válidos, `minimal→low`, desconocido→`""` (sin bandera)— y la asimetría con
`presupuestoDeThinking`, que sí clampa `xhigh/max→high`. El detalle vive en `design.md`.
