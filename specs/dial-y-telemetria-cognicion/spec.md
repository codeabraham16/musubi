# Spec — Dial de potencia y telemetría (F5)

Contrato observable. Cada invariante está numerado y tiene una prueba que **sabe fallar**. Un
invariante sin test que lo pueda tumbar es decoración.

Prefijo **`D`** (dial) para no colisionar con los `C*` del router (F2) ni los `K*` del caché (F3),
que viven en el mismo paquete Go.

---

## El dial

`cognition.effort: eco | balanced | turbo`. Cada nivel es un preset sobre perillas que ya existen:

| Perilla | `eco` | `balanced` (default) | `turbo` |
|---|---|---|---|
| `read_time_rerank` | `false` | `false` | **`true`** |
| `read_time_rerank_top_k` | — | — | `12` |
| `cache.ttl_seconds` | `86400` (1 día) | `3600` (1 h) | `900` (15 min) |

Sólo hay tres filas porque **son las perillas que hay**. Un dial que promete cinco ejes y mueve tres
miente sobre lo que controla.

Las dos decisiones que no son obvias:

- **`balanced` deja el juez APAGADO.** Es el default y tiene que ser el que no sorprende: el juez
  del recall es el seam de mayor riesgo (latencia en el camino caliente y rate-limit), y F1 lo dejó
  apagado a propósito. Un default que enciende el camino caliente no es "balanceado", es turbo con
  otro nombre.
- **Más potencia ⇒ TTL más corto.** Parece al revés, pero no lo es: quien pide `turbo` quiere
  respuestas frescas y está dispuesto a pagarlas. Quien pide `eco` prefiere una respuesta de ayer
  antes que una llamada.

---

## Invariantes

### D0 — Lo explícito le gana al preset *(el invariante fundamental)*

Si alguien escribe una perilla a mano, el dial **no la pisa**. El preset sólo llena lo que quedó sin
declarar.

> Si el dial ganara, una config existente cambiaría de comportamiento en silencio al agregar
> `effort`. Un preset que pisa lo explícito no es un default: es una sorpresa.

Para poder distinguir «no lo escribieron» de «lo escribieron en false», las perillas booleanas que
el dial toca son `*bool`. Es la misma razón por la que `cache.enabled` ya lo era.

### D1 — Sin `effort` declarado, nada cambia

Config sin `effort` ⇒ comportamiento **bit-idéntico** al de antes de esta fase. El dial no tiene
default implícito que se aplique solo.

> `balanced` es el default *del dial*, no de Musubi. Son cosas distintas y confundirlas cambiaría el
> comportamiento de toda instalación existente.

### D2 — Un `effort` desconocido es error de arranque

`effort: turbol` **no** cae a `balanced` en silencio: falla explícito, igual que un `gateway.mode`
mal escrito apaga el pilar entero.

### D3 — El dial es puro

Resolver el preset no toca red, ni disco, ni reloj: misma config ⇒ misma config resuelta, siempre.

### D4 — Los contadores cuentan lo que pasó

Para cada superficie instrumentada, el contador refleja las operaciones **reales**:

| Contador | Qué cuenta |
|---|---|
| `cache.hits` / `cache.misses` | respuestas servidas del caché vs. delegadas al motor |
| `gateway.calls` / `gateway.scrubbed` / `gateway.blocked` | llamadas, cuántas taparon algo, cuántas se negaron |
| `gateway.types` | tipos de secreto tapados, **sin valores** |
| `router.escalations` / `router.exhausted` | veces que se pasó al siguiente motor / se acabó la flota |

### D5 — La telemetría NUNCA contiene un secreto

`gateway.types` lleva **tipos** (`aws-access-key`), jamás valores. Ningún contador guarda prompt ni
respuesta.

> Es una superficie de lectura nueva sobre el subsistema que maneja secretos. Que sólo cuente y
> clasifique no es un detalle de implementación: es el invariante que la hace segura de exponer.

### D6 — Contar no cambia el comportamiento

Con la telemetría activa, todo responde exactamente lo mismo que sin ella. Los contadores son
observadores, no participantes.

### D7 — Seguro bajo concurrencia

El daemon atiende en paralelo. Los contadores no se corrompen ni pierden incrementos.

> Verificado por la CI con `-race`: `-race` exige cgo y la máquina de desarrollo no tiene compilador
> de C. El test local comprueba que la suma cuadre con las llamadas hechas.

### D8 — Leer no muta

`musubi_cognition_stats` es read-only: no resetea contadores, no vacía el caché, no cierra circuitos.

> Un contador que se resetea al leerlo hace que dos lectores se roben los datos entre sí.

---

## Criterios de aceptación

1. Los 9 invariantes con test propio, cada uno verificado **fallando** al sabotear.
2. `go build`, `go vet`, `go test ./...` y `golangci-lint run` en verde.
3. D1 cubierto por test: sin `effort`, la config resuelta es idéntica a la de entrada.
4. Test adversarial: `effort` con mayúsculas y espacios, `effort` + perilla explícita en conflicto
   (los dos sentidos), y lectura de stats con el pilar apagado.
