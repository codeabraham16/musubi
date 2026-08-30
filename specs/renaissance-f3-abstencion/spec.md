# SDD spec — renaissance-f3-abstencion

## Contrato

`designBrief` gana dos campos:

- `retrieval`: `"semantico"` | `"fts"` — con qué se buscó. **Siempre presente.**
- `degraded_reason`: `""` | `"sin_material"` | `"bajo_umbral"` | `"sin_recuperador"` — por qué no hay
  material. Presente sólo cuando `degraded` es true.

Constantes nuevas: `designSimilitudMinima` (el piso) y `designEmbedTimeout` (5 s).

## Invariantes

### I-ABS1 · lo que no llega al piso no se sirve, y la abstención se declara
Por el camino semántico, un hit con similitud por debajo del piso se descarta. Si no queda ninguno,
el brief sale con `corpus` VACÍO, `degraded: true` y `degraded_reason: "bajo_umbral"`. Nunca con
relleno.

**Sabotaje:** un pedido cuyos candidatos están todos apenas por debajo del piso ⇒ tiene que abstener.
Y subir el piso por encima de todos los candidatos de un pedido legítimo ⇒ también abstiene. El test
mueve el piso en las dos direcciones para que no pase por casualidad.

### I-ABS2 · el modo de recuperación siempre se declara
`retrieval` nunca queda vacío, ni cuando abstiene, ni cuando degrada, ni cuando no hay embebedor.

**Sabotaje:** correr sin embebedor ⇒ `retrieval` tiene que decir `"fts"`, no quedar vacío. La caída
a léxico era exactamente el silencio que se está cerrando.

### I-ABS3 · abstenerse no rompe el brief
Aunque el corpus quede vacío, el brief conserva el núcleo estático, la precedencia, la marca y el
método. Abstenerse es decir «no tengo material específico para esto», no devolver nada.

**Sabotaje:** consulta fuera de dominio ⇒ `corpus` vacío pero `principles`, `brand` y `precedence`
presentes.

### I-ABS4 · el piso no se aplica donde no hay puntaje
Por FTS no hay similitud que comparar, así que el piso no corre y `degraded_reason` nunca puede ser
`"bajo_umbral"` en ese modo. Declararlo ahí sería inventar una medición.

**Sabotaje:** forzar el camino FTS con candidatos ⇒ `degraded_reason` vacío y `retrieval: "fts"`.

## Métricas

| Métrica | Antes | Después (esperado) |
|---|---|---|
| M2 abstención (sonda, embebedor real) | 0,00 | **1,00** |
| M2 abstención (banco, FTS) | 0,25 | sin cambio — el piso no aplica ahí |
| Falsa abstención en pedidos legítimos (sonda) | — | **0** (métrica nueva; si sube, el piso baja) |

## Riesgo principal, con su medición

**El piso deja afuera pedidos buenos.** Se calibra contra el set dorado: la sonda cuenta cuántos
pedidos LEGÍTIMOS terminan abstenidos. Si ese número no es cero, el piso está mal y baja. Por eso la
sonda mide las dos caras y no sólo la que conviene.

## Fuera de alcance

Elegir el método según el pedido (F4) y normalizar la consulta larga (F5). En particular: un prompt
gigante va a seguir degradando el recall — lo que cambia acá es que **falla en 5 s y lo dice**, en vez
de tardar 30 s y mentir.
