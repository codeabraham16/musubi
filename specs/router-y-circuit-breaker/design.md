# Diseño — Router de cognición + circuit breaker (F2)

## Forma general

El router **es un `Provider`**. Eso mantiene intacto todo lo de río arriba: `NewProvider` sigue
devolviendo un `Provider`, el server sigue recibiendo uno, y las tools no se enteran de que abajo
hay una flota.

```
cognition.NewProvider(cfg)
        │
        ├── sin fleet ──► guarded{ motor único }          (F1, sin cambios)
        │
        └── con fleet ──► router{ [ engine₀, engine₁, … ] }
                                    │        │
                        guarded{groq}  guarded{max}
                          mode=refuse    mode=scrub
```

Cada entrada de la flota se construye con **la misma fábrica de siempre** (`newBaseProvider` +
`newGuarded`). El router no fabrica motores: los compone. Así todo lo que F1 garantiza sobre un
motor —que nace envuelto, que el modo se valida, que el Noop no se toca— vale igual acá, gratis.

## La composición que resuelve C0

Un motor `free` se configura con `gateway.mode: refuse`. Entonces:

```
Ask("… la clave es AKIA… …")
  ├─ engine₀ (groq, free, refuse)  ──► ErrSecretsBlocked   ← NO salió a la red
  │                                     el router NO cuenta esto como falla
  └─ engine₁ (max, private, scrub) ──► sale tapado, responde
```

El router **no sabe qué es un secreto**. Sólo sabe que si un motor dice "esto no lo mando", hay que
probar el siguiente. La regla dura del roadmap deja de ser un `if` que alguien pueda olvidar y pasa a
ser una propiedad de cómo está armada la flota.

Y el corolario que importa: si **no hay** motor privado, el prompt con secretos no se manda a
ninguno. Se devuelve el agotamiento y el caller degrada a model-free. Es exactamente el "en duda,
fallback local" del roadmap, sin código extra.

## Por qué `ErrSecretsBlocked` no cuenta para el breaker (C2)

Un breaker mide **salud**. Un motor que se niega por política está sano: hizo justo lo que se le
pidió. Si contara como falla, tres prompts con secretos seguidos apagarían un motor perfectamente
bueno, y el cuarto prompt —limpio— se iría al tier caro sin motivo.

`ErrGatewayFailed` sí cuenta: eso es el portero roto, y un portero roto es un motor inusable.

## El breaker

```go
type breaker struct {
    mu       sync.Mutex
    fails    int
    openUntil time.Time
    probing  bool      // hay una prueba half-open en vuelo
}
```

Tres estados implícitos, sin enum:

| Estado | Cómo se reconoce | Qué hace el router |
|---|---|---|
| cerrado | `openUntil` en el pasado | lo intenta |
| abierto | `now < openUntil` | lo saltea |
| half-open | venció `openUntil` y `!probing` | lo intenta **una vez** (`probing = true`) |

`probing` es lo que hace cierto el "exactamente una" de C4: sin él, diez goroutines que llegan justo
al vencer el cooldown se van todas contra un motor que probablemente sigue caído. La que entra pone
la bandera; las demás lo saltean como si siguiera abierto.

**El reloj se inyecta** (`now func() time.Time`). Un test de cooldown que dependa del reloj real es
un test lento y flaky; con el reloj inyectado, C3 y C4 se prueban en microsegundos y sin `sleep`.

## Decisiones y alternativas descartadas

| Decisión | Por qué | Qué se descartó |
|---|---|---|
| El router es un `Provider` | Nada río arriba se toca; se compone con lo de F1 | Un tipo nuevo con su propia interfaz: obligaría a tocar el server y las tools |
| `tier` default `free` | Asumir no confiable es la dirección segura; confiar se **declara** | Default `private`: un motor mal configurado quedaría tratado como de confianza |
| El motor `free` se niega, el router escala | La regla dura queda estructural, no imperativa | Que el router inspeccione el texto: duplicaría la detección y se podría olvidar |
| Reloj inyectado | Tests deterministas y rápidos | `time.Sleep` en los tests: lentos, flaky, y no prueban el borde exacto |
| `probing` en vez de contador de half-open | Simple y suficiente para "exactamente una" | Un semáforo con N pruebas: complejidad sin beneficio acá |
| Sin presupuesto diario | Requiere estado durable entre reinicios | Meterlo en memoria: mentiría al reiniciar |

## Modo de falla, dicho de frente

- **El orden lo fija el usuario, no la dificultad de la consulta.** Acá no hay "dial de potencia":
  el router saltea lo roto y lo que se niega, nada más. Elegir tier por dificultad es F5.
- **Sin presupuesto diario**, una flota mal configurada puede quemar cuota de un tier gratis rápido.
  El breaker cubre el caso "el proveedor devuelve error" (incluido un 429), pero no "gasté demasiado
  y todavía responde".
- **Un motor lento no es un motor caído.** El breaker cuenta errores, no latencia. Un motor que
  responde en 30s no se abre; lo acota el timeout por llamada, que ya existe
  (`request_timeout_seconds`).
- **`scrub` en un tier gratis está permitido.** No hay fuga de credenciales, pero el resto del texto
  sí llega a un servicio que entrena con lo que recibe. Se marca en amarillo en el doctor en vez de
  prohibirlo: es una decisión legítima del dueño de los datos, pero tiene que verse.
