# Diseño — Dial de potencia y telemetría (F5)

## El dial se resuelve en `config.Load`, no en la fábrica

Parecía natural resolver el preset dentro de `cognition.NewProvider`. **No alcanza**: el servidor
MCP guarda su propia copia de `CognitionConfig` para decidir si corre el juez del recall
(`s.cognitionCfg.ReadTimeRerankOn()`). Resolverlo en la fábrica dejaría esa copia con el dial sin
aplicar, y `turbo` prendería el juez… en ningún lado.

Va en `config.Load`, que es el único punto por el que pasa toda config cargada. **Un dial que rige
en la mitad de los consumidores es peor que no tener dial.**

## `ReadTimeRerank` pasó de `bool` a `*bool`

Es el cambio de tipo que hace posible D0. Con un `bool` pelado, «no lo escribieron» y «lo
escribieron en `false`» son el mismo cero de Go, así que el preset no puede distinguirlos y
terminaría pisando lo explícito.

Es exactamente la misma razón por la que `cache.enabled` ya era `*bool`. La resolución se
centraliza en `ReadTimeRerankOn()` para que ningún consumidor tenga que acordarse de desreferenciar.

## Los contadores viven detrás de punteros

`guarded` es un tipo **por valor** (`func (g guarded) Ask`). Si los contadores fueran campos, cada
copia de la struct contaría por su cuenta y los números no sumarían nada. Por eso `stats
*gatewayStats`, y por eso `gatewayStats.record` tolera un receptor nil: un `guarded` construido a
mano en un test no necesita telemetría, y no debería explotar por no tenerla.

## La foto se arma recorriendo la cadena

```go
type statsReporter interface{ reportStats(*CognitionStats) }
```

Cada capa suma lo suyo y delega hacia adentro. Así `Stats(p)` funciona sin que nadie tenga que
saber en qué orden quedaron apilados los decoradores — que además cambia según haya flota o motor
único:

```
motor único:  cached → guarded → OpenAICompat
con flota:    cached → router  → [guarded → motor] × N
```

El router además recorre sus motores para juntar el portero de cada uno y anotar qué circuitos están
abiertos **ahora**.

Detalle de locking: `cached.reportStats` **suelta su mutex antes** de bajar a la capa siguiente. Si
lo mantuviera, tendría dos locks tomados a la vez y abriría la puerta a un orden de adquisición
inconsistente el día que alguien agregue una capa que llame hacia arriba.

## Por qué una tool MCP y no `musubi doctor`

Los contadores viven **en memoria del proceso que atiende**, y el CLI es **otro proceso**. `musubi
doctor` construiría su propio provider con el caché vacío y reportaría **ceros para siempre** — un
número convincente y falso, que es peor que no tener número.

`musubi_cognition_stats` queda clasificada como `readOnly` y en `noScopedRead`: no lee ninguna tabla,
así que no hay nada que acotar por proyecto. Los dos tests-guarda que obligan a clasificarla se
satisficieron declarándola, no silenciándolos.

## Detalles que parecen menores y no lo son

- **`cache_hit_rate` es `null` sin llamadas, no `0`.** Cero por ciento y «no hay datos» son cosas
  distintas, y confundirlas hace tomar decisiones sobre humo.
- **Los tipos de secreto se ordenan** antes de imprimirse: sin eso, dos lecturas seguidas del mismo
  estado parecen distintas (el orden de un mapa en Go es aleatorio).
- **El portero cuenta ANTES de llamar al motor.** Lo que se mide es el trabajo del portero, no el
  éxito del motor: una llamada que el motor rechaza igual pasó por la redacción.
- **`GatewayFailed` debería ser 0 siempre.** Cuenta los pánicos que atajó el `recover`. Si no es
  cero hay un bug en la redacción, y por eso el resumen lo grita en vez de listarlo al pasar.

## Lo que NO hace

- **No exporta nada.** Sin red, sin archivo, sin Prometheus. Contadores en memoria que se leen a
  pedido y mueren con el proceso. Persistirlos es otra fase con otras preguntas (¿por proyecto?,
  ¿cuánta retención?, ¿qué pasa con los tipos de secreto en disco?).
- **No mide calidad.** Cuenta llamadas, hits y escaladas. Si el juez del recall *acierta* es otra
  medición, con corpus dorado, y ya tiene su propio gate (`recall-gate`).
- **No hay un cuarto nivel.** Tres perillas reales ⇒ tres niveles. Un dial que promete cinco ejes y
  mueve tres miente sobre lo que controla.
