# Handoff al cuerpo — el panel pide 77 KB cada 4 s para leer un entero

**Para:** la terminal de `musubi-body`.
**De:** la sala de mando (repo Musubi). Acá se documenta; el cambio se aplica allá.
**Fecha:** 2026-08-11. **Cerebro central:** `v0.102.1`.

## El hallazgo, medido

En el cerebro central hay **358 relaciones de conflicto pendientes**. `musubi_conflicts` devuelve la
lista completa: **77 KB por respuesta**.

El cuerpo la pide **cada 4 segundos** en modo watch, y usa exactamente un campo:

```go
// cmd/musubi-body/bridge.go:523
if cf, err := c.Conflicts(); err == nil {
    m.Conflicts = cf.Count      // ← el único campo que se usa
}
```

- `-interval` default: **4 segundos** (`bridge.go:117`).
- `internal/brain/memoria.go:151` llama a la tool con `map[string]any{}` — sin argumentos.
- `memoriaThrottled` **sí** throttlea la salud (90 s) y la integridad profunda (10 min), pero
  `Conflicts()` queda afuera del throttle, en la misma función. El comentario del código lo dice
  literal: *«conflicts sigue en vivo aparte»* (`bridge.go:390-398`).

**Costo: ~69 MB por hora de app abierta, por instancia**, para pintar un número. Escala lineal con
cada cuerpo desplegado, y el payload crece con la cola.

Del lado del ledger del central: `musubi_conflicts` se llamó **7.674 veces en 30 días** por
`davantis-admin` y **933** por `davantis-mando`.

## Lo que ya está del lado del cerebro

PR **#294** le agrega parámetros a `musubi_conflicts` (antes no aceptaba **ninguno**):

| Parámetro | Tipo | Para qué |
|---|---|---|
| `count_only` | bool | devuelve sólo el conteo, sin las relaciones |
| `limit` | number | tope de la lista. **No** afecta a `count` |
| `min_confidence` | number | descarta las de confianza menor |
| `order` | string | `recent` (default) \| `confidence` |

Sin argumentos se comporta igual que siempre, así que el cuerpo **no se rompe** si no cambia nada.
Pero tampoco ahorra un byte hasta que mande `count_only`.

Respuesta con los filtros:

```json
{ "count": 358, "relations": [], "count_only": true }
{ "count": 358, "relations": [ ... 10 ... ], "truncated": true }
```

`count` es **siempre el total** que matchea los filtros, aunque `limit` recorte la lista. Si algún
día el panel pagina, el badge tiene que seguir leyendo `count` y no `len(relations)`.

## Los dos cambios propuestos

**1 · El snapshot pide sólo el conteo.** Es el que paga solo.

```go
// internal/brain/memoria.go — Conflicts() para el badge del snapshot
c.callTool("musubi_conflicts", map[string]any{"count_only": true})
```

De 77 KB a unos pocos bytes, sin perder nada: el snapshot ya sólo usa `cf.Count`.

**2 · Throttlear `Conflicts()` como ya se throttlea la salud.** La cola no cambia entre dos
parpadeos; refrescarla cada 4 s no aporta frescura útil. Un TTL en la línea de `healthRefreshTTL`
(90 s) alcanza. Con el punto 1 ya casi no cuesta, pero sigue siendo una llamada RPC al central cada
4 segundos por instancia.

**Ojo:** la vista Memoria (`cmd/musubi-body-ui/memoria.go:147` y `cmd/musubi-body/govern.go:220`) sí
necesita las relaciones. Esas llamadas van **sin** `count_only` — el cambio es sólo para el badge del
snapshot.

## Una oportunidad, si se toca esa vista

Con `limit` y `order=confidence` la vista Memoria puede paginar en vez de traer las 358 de una. Hoy
muestra ~10-15 filas sin scroll, así que está bajando 77 KB para pintar quince.

**Pero no ordenes por confianza creyendo que es gravedad.** En una relación pendiente `confidence` es
`max(léxico, coseno)`, y el coseno entre documentos **sin relación** ya llega a 0,884 medido en el
repo del cerebro. Ordena por *parecido*, no por *contradicción*. Sirve para acotar el payload; no
para decidir qué mirar primero.

## Lo que NO hay que cambiar

- **El copy de la vista.** «Memorias sin resolver» / «Resolver conflictos» está bien. Un reporte
  intermedio afirmaba que la card decía «la memoria empieza a fallar» — se verificó y **es falso**.
- **El badge en sí.** Mostrar el número está bien; lo que estaba mal era cómo se conseguía.

## Contexto que ayuda a decidir

`pending` significa **sin juzgar**, y no oculta nada: las dos memorias siguen visibles en el recall.
Sólo `supersedes` oculta, y hubo 50 en toda la historia del cerebro. De 787 relaciones resueltas,
**621 `related` + 65 `not_conflict` = 87 % dijo «acá no había conflicto»**, contra 15
`conflicts_with` (1,9 %).

O sea: el detector mide **parecido**, no contradicción. Una cola grande de pendientes no es una
memoria enferma — es un detector con poca precisión y una cola que nadie puede triar. Eso importa
para el peso visual que se le dé al indicador en el panel.
