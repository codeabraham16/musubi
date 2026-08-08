# Propuesta — El juez se puede medir

Track: **Potencia medida**, F2. Depende de F1 (`specs/motor-sin-candado/`), que ya sacó la llamada al
motor de abajo del candado del despacho — sin eso, encender el juez serializaba el cerebro.

## La pregunta que F2 tiene que contestar

> ¿El juez de pertinencia del recall **mejora el orden lo suficiente** como para pagar su latencia y
> su cuota?

Hoy esa pregunta se contesta con opinión. El juez existe (`read_time_rerank`), nace apagado, y está
apagado en el central. Encenderlo o no es una decisión que **nadie midió**.

## Lo que ya existe, medido

`internal/recalleval` tiene lo caro hecho:

| pieza | qué hace |
|---|---|
| `Evaluate` | corre las queries del fixture y agrega MRR, Recall@k y nDCG@k |
| `Run` | siembra un engine y compara varias configs **sobre el mismo corpus** |
| `FormatReport` | la tabla comparativa |
| `testdata/baseline_modelfree.json` | el baseline model-free, commiteado |
| barridos | semántico, MMR y ollama, ya escritos |

No hay que construir un banco. Hay que **enchufarle el juez**.

## El problema que hay que resolver primero

El juez de verdad vive en `internal/mcp`:

```go
func (s *McpServer) rerankIfEnabled(ctx, query string, res memory.RecallResult) memory.RecallResult
```

Está atado a cuatro cosas del servidor: `s.cognition`, `s.cognitionCfg`, el tipo
`memory.RecallResult` y un **caché global de paquete** (`rerankCache`).

Y el banco no lo puede llamar. Su unidad de configuración es:

```go
type Config struct {
	Name      string
	Opts      memory.RecallOptions
	UseVector bool
}
```

Sin brazo de rerank.

**La salida fácil es la trampa:** agregarle al banco su propia versión del prompt y del parseo. Ahí
el banco mide una **imitación** del juez, y un banco que mide una imitación es *peor* que no tener
banco — devuelve un número con aspecto de autoridad sobre algo que no es lo que corre en producción.
El día que el prompt de producción cambie, el número seguiría igual de convincente y ya sería falso.

## Qué se construye

1. **El juez sale a una unidad reusable**: prompt, llamada al motor y parseo del orden, sin saber de
   `McpServer` ni de `RecallResult`. `rerankIfEnabled` queda como adaptador delgado.
2. **El caché NO se va con él.** En producción estira la suscripción y protege el rate-limit
   compartido; en el banco falsearía la medición devolviendo respuestas viejas. Es una decisión del
   *llamador*, no del juez.
3. **`Config` gana el brazo de rerank** y `rankedIDs` lo aplica sobre el head, con el mismo top-K
   que usa producción.
4. **Un motor falso determinista** para que el banco tenga pruebas sin gastar cuota ni depender de
   la red.

## Lo que NO se construye, dicho ahora

- **No se enciende el juez en el central.** Este spec produce el instrumento; la decisión de
  encenderlo es del dueño y con el número en la mano.
- **No se amplía el fixture acá.** Hoy son 26 docs / 12 queries, que alcanza para verificar que el
  brazo funciona pero **no** para que una diferencia de MRR signifique algo. Ampliarlo con memoria
  real es trabajo aparte, y mezclarlo haría que este spec no se pueda verificar solo.
- **No se corre el banco contra el motor real.** Esta PC no tiene bloque `cognition` en su config
  (verificado), así que la corrida real necesita el central o el túnel con `LITELLM_MASTER_KEY` —
  un secreto que pone el dueño. Queda como paso siguiente, explícito.
- **No se toca el presupuesto ni la autorización del motor.** Siguen siendo los pasos 3 y 4 de F1.
