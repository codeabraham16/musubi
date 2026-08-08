# Propuesta — El motor no traba la casa

Track: **Potencia medida**, F1. Nace de una corrección explícita del usuario (2026-08-07):

> *«Yo quiero que ese motor sea alcanzable a todo aquel que tenga el poder de Musubi, no sólo en esta
> PC. Y quiero hacer que eso funcione a la perfección, no tenga fallas a la hora que se utilice y que
> no quede a medias.»*

## El planteo que tenía, y por qué estaba mal

Yo había planteado F1 como un problema de **alcance**: el motor vive en `127.0.0.1:4000` del server, hay
que decidir cómo lo alcanzan las otras máquinas (túnel SSH, exponer el bind, o API key). Medir el
código dice otra cosa.

**El motor ya es alcanzable por toda la malla.** El central tiene la cognición encendida:

```
cognition:
  provider: openai-compat
  endpoint: http://127.0.0.1:4000/v1
  model:    sonnet
  auth_token_env: LITELLM_MASTER_KEY
```

Y `musubi_ask` se sirve por el puerto 7717, que está en la tailnet con 8 principals (6 writers). El
bind a loopback protege a litellm del acceso *directo*, pero **el cerebro es la puerta de adelante y
ya está abierta**. Lo que falta no es alcance: es **control**.

Eso tumba el paso que yo había puesto como central — cambiar el bind de litellm **no hace falta**.

## Lo que sí es un defecto medido

`handleToolsCall` toma un lock de despacho **alrededor del handler entero**, con `defer Unlock()`:

```go
if readOnly { s.dispatchMu.RLock();  defer s.dispatchMu.RUnlock() }
else        { s.dispatchMu.Lock();   defer s.dispatchMu.Unlock()  }
```

Y `musubi_recall` **no** está marcada `readOnly` — porque bumpea contadores de acceso. Así que toma
el lock **exclusivo**. El juez del recall (`rerankIfEnabled`) corre adentro de ese lock, y llama al
LLM por red. Lo mismo `musubi_ask`.

Los timeouts: `askTimeout` 150 s de contexto, y el cliente HTTP del provider en 120 s. O sea que
**una sola llamada al motor puede tener el central entero tomado hasta dos minutos**.

No es teórico. Del ledger del central, 30 días:

| tool | llamadas | media | p95 |
|---|---|---|---|
| `musubi_ask` | 1 | **25.128 ms** | 25.128 ms |
| `musubi_recall` | 14 | 1.293 ms | 3.316 ms |
| `musubi_save_observation` | 131 | 4.856 ms | 10.194 ms |

El único `musubi_ask` real tardó **25 segundos con el lock exclusivo tomado**. Durante esos 25
segundos el cerebro no atendió a nadie más. Y ya hay contención sin juez: los saves promedian 4,9 s
bajo el mismo lock.

Encender el juez para N máquinas, con este cableado, convierte al cerebro en una fila de a uno. Eso
es exactamente *«que quede a medias»*, y pasa **antes** de cualquier discusión de credenciales.

## El defecto de fondo: un booleano decide dos cosas distintas

`readOnly` gobierna **dos ejes que no tienen por qué coincidir**:

| eje | qué decide | dónde |
|---|---|---|
| concurrencia | ¿corre bajo `RLock` o bajo `Lock`? | `handleToolsCall` |
| autorización | ¿un principal `reader` puede llamarla? | `Principal.canCall` |

Pegados en un booleano, fuerzan un canje falso: para que `recall` deje de serializar habría que
marcarla `readOnly`, y eso además **le abriría la tool a los readers**. Se arregla un problema
creando otro.

Y hay una contradicción viva que lo delata: el comentario de `toolAsk` dice *«NO escribe al libro
mayor: es de sólo lectura»* — pero la tool **no** está marcada `readOnly`. El código ya sabe que los
dos ejes no coinciden; sólo que no tiene cómo decirlo.

## El dato que hace el arreglo barato

El bump de accesos que justifica el lock exclusivo es esto:

```sql
UPDATE observations
SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1
WHERE id IN (...)
```

**Una sola sentencia atómica.** No es un read-modify-write en Go — que es literalmente lo que el
comentario del lock dice estar protegiendo (*«sin lost-updates de read-modify-write»*). SQLite ya lo
serializa. El lock exclusivo de Go no está comprando nada acá.

## Qué se construye

1. **Se separan los dos ejes.** `readOnly` queda como el eje de **autorización** (sin mover a nadie),
   y la concurrencia pasa a una clase declarada por tool cuyo default reproduce exactamente lo de hoy.
2. **Ningún lock del dispatcher cruza una llamada al motor.** `musubi_recall` y `musubi_ask` manejan
   su propia sección crítica: toman `RLock` sólo alrededor del acceso a la base, lo sueltan, y recién
   ahí hablan con el LLM.
3. **La llamada al embedder también sale de abajo del lock.** Está en el mismo camino, con 30 s de
   timeout, y es el mismo defecto: red bajo candado.

## Lo que NO se construye, dicho ahora

- **No se cambia quién puede usar el motor.** Ni un principal gana ni pierde acceso. Hoy alcanza con
  ser `writer`, y eso sigue igual — es un problema real, pero es **otro** spec (paso 4 de F1).
- **No se agrega presupuesto ni medición de gasto.** `CognitionStats` mide caché, portero y router;
  no mide tokens ni costo. Medido: `budget|quota|spend` no aparece en todo `internal/cognition/`.
  Es el paso 3 de F1.
- **No se enciende el juez en el central.** Sigue apagado, y encenderlo depende de que el banco de
  F2 muestre que vale la pena. Este spec hace que *se pueda* encender sin romper nada, no lo enciende.
- **No se toca el bind de litellm.** El análisis mostró que no hace falta.
