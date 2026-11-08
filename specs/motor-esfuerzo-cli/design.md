# Diseño — `motor`: `--effort` desde el nivel del cliente

## La decisión central: Esfuerzo es un campo APARTE de Thinking, no una variante del mismo

El mismo dato del cliente —`options.reasoning`— alimenta DOS traducciones, una por motor, y por eso
hay dos campos en `Pedido` en vez de uno:

- **`Thinking`** es un BUDGET en tokens, y sólo lo entiende el **ClienteAPI** (la Messages API, que
  toma `thinking.budget_tokens`). Lo resuelve `presupuestoDeThinking`.
- **`Esfuerzo`** es un NIVEL cualitativo, y sólo lo entiende el **Lanzador** (`claude -p --effort`,
  que NO toma un budget por bandera — toma `low…max`). Lo resuelve `esfuerzoDesdeReasoning`.

Son dos traducciones del mismo origen para dos destinos distintos. Unificarlas en un solo campo
obligaría a un motor a deshacer lo que el otro necesita.

## Por qué NO comparten normalización — la asimetría del clamp

Esta es la trampa, y está documentada en el comentario de `Pedido.Esfuerzo` (`motor.go:134-137`):

```
presupuestoDeThinking:  xhigh → high,  max → high     (CLAMPA)
esfuerzoDesdeReasoning:  xhigh → xhigh, max → max      (PASA DERECHO)
```

El clamp de `presupuestoDeThinking` **no es una decisión de este repo**: lo hace pi (`clampReasoning`,
`simple-options.ts:78`) porque su tabla de presupuestos tiene **cuatro** niveles y los dos de arriba
no tienen uno propio. Sin el clamp, un cliente que pide `max` caería en el cero del struct y se
quedaría SIN thinking justo cuando pidió el máximo.

`--effort`, en cambio, **acepta `xhigh` y `max` nativos** (medido 2026-11-07, `claude --help`). Si
`esfuerzoDesdeReasoning` copiara el clamp de thinking, le recortaría al usuario los dos niveles más
altos del Lanzador — el motor que efectivamente corre en producción. Por eso pasa derecho. **La
asimetría es el diseño**, no un descuido: dos destinos con vocabularios distintos exigen dos
normalizaciones distintas, aunque el origen sea el mismo.

## Las reglas de `esfuerzoDesdeReasoning`

```go
func esfuerzoDesdeReasoning(reasoning string) string {
    nivel := strings.TrimSpace(strings.ToLower(reasoning))
    switch nivel {
    case "low", "medium", "high", "xhigh", "max":
        return nivel          // passthrough: los cinco que --effort acepta
    case "minimal":
        return "low"          // pi lo tiene; --effort no. Al piso del CLI.
    default:
        return ""             // sin bandera: caída segura
    }
}
```

El `default → ""` es deliberado y es lo contrario de "ignorar el valor raro": un `--effort` con un
valor inventado **no lo ignora el CLI, lo rechaza y mata el turno**. Devolver `""` hace que el
Lanzador NO emita la bandera (ver la guarda abajo), y el CLI usa su propio default. Un nivel que esta
versión no conoce —porque el cliente se adelantó, o escribió cualquier cosa— degrada a "sin modular"
en vez de a "turno muerto".

El `TrimSpace`+`ToLower` es por robustez de borde: el nivel puede venir de un yaml escrito a mano o de
una versión distinta del cliente.

## La guarda en el Lanzador

```go
if p.Esfuerzo != "" {
    args = append(args, "--effort", p.Esfuerzo)
}
```

`""` = ausencia de bandera, NO bandera con valor vacío. La distinción es de correctitud: `--effort ""`
le llega al CLI como un nivel vacío que rechaza. La guarda y el `default → ""` de arriba son las dos
mitades de la misma garantía — una produce el centinela, la otra lo honra.

Va **después de `--model`** en el argv, sin dependencia de orden real (el CLI parsea por nombre), por
legibilidad: el nivel se lee pegado al modelo que modula.

## Dónde queda cada pieza

| Pieza | Archivo | Rol |
|---|---|---|
| `Esfuerzo string` | `motor.go` (`Pedido`) | el dato, con el comentario que explica la asimetría |
| `esfuerzoDesdeReasoning` | `entrada.go` | normaliza el nivel del cliente al de `--effort` |
| cableado en `PedidoDesdeProxy` | `entrada.go` | puebla `Esfuerzo` desde `options.reasoning` |
| `--effort` en `argumentosCLI` | `lanzador.go` | emite la bandera (sólo el Lanzador) |

## Lo que viaja al cuerpo (repo thinkrail, cambio separado)

Para que el selector exista en la UI, el provider `pi-musubi` declara `reasoning: true` en sus seis
modelos. Eso le dice a pi que el modelo razona, lo que (a) muestra el selector de esfuerzo y (b) hace
que pi mande `options.reasoning` en el pedido. Sin ese flip, el motor honraría un nivel que el cliente
nunca llegaría a mandar. **Orden de entrega:** primero el binario `musubi` con `--effort`, después el
rebuild del cuerpo con `reasoning: true` — al revés, el selector sería un botón que manda un dato que
el motor todavía tira.
