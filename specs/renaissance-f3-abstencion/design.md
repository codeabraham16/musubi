# SDD design — renaissance-f3-abstencion

## Archivos

| Archivo | Qué cambia |
|---|---|
| `internal/mcp/methods_design.go` | `designSimilitudMinima`, `designEmbedTimeout`, las constantes de modo y causa, `resultadoRecall`, `sobreElPiso`; el brief gana `retrieval` y `degraded_reason` |
| `internal/mcp/methods_design_abstencion_test.go` | **nuevo** — I-ABS1..4 con un embebedor determinista |
| `internal/mcp/sonda_diseno_test.go` | mide también la FALSA abstención y las causas |

## `resultadoRecall`

`recallDesignCorpus` devolvía `(hits, degraded bool)`. Ese bool no distinguía **«no existe nada»** de
**«existe pero es malo»**, y no decía si la búsqueda había caído a léxico. Un motor que no puede
explicar su propio silencio obliga a quien lo llama a adivinar. Ahora devuelve una struct con el
material, el modo y la causa.

## El piso, y dónde NO se aplica

`sobreElPiso` descarta los candidatos bajo `designSimilitudMinima`. Corre **sólo por el camino
semántico**: por FTS no hay puntaje que comparar, y declarar `bajo_umbral` ahí sería inventar una
medición que no se hizo (I-ABS4). Por eso el modo se declara siempre — la diferencia entre los dos
caminos pasa a ser visible en vez de invisible.

El valor 0,48 sale de la separación medida contra el acervo real (basura 0,362–0,442; pedidos reales
0,533–0,558). Es una **calibración**, no una constante universal.

## Distinguir «no hay» de «no pude»

Si FTS devuelve error, la causa es `sin_recuperador` y no `sin_material`. Son dos problemas
distintos con dos arreglos distintos: el primero es un hueco del acervo, el segundo una falla del
motor. Confundirlos manda a arreglar lo que no está roto.

## El embebedor de prueba

`embebedorPorAngulo` mapea cada texto a un vector unitario en 2D según un ángulo elegido por su
contenido, así la similitud coseno entre dos textos es exactamente `cos(θ1−θ2)`. Sirve para pedir
«estos dos a 0,40» sin depender de ningún modelo.

**No simula calidad de recuperación** — ejercita la aritmética del piso, que es lógica pura. Medir
estabilidad de paráfrasis o precisión con un embebedor falso sería medir al embebedor falso, el modo
de falla que este repo ya documentó cuatro veces. Eso vive en la sonda, contra bge-m3 real.

## El riesgo se mide junto al beneficio

La sonda cuenta cuántos pedidos **legítimos** terminan abstenidos. Una sonda que sólo reportara la
cara que conviene sería un expediente, no un instrumento. Si ese número no es cero, el piso baja.

## Lo que esta fase NO arregla

Un prompt gigante va a seguir degradando el recall — eso es F5. Lo que cambia acá es que **falla en
5 s y lo dice**, en vez de tardar 30 s y mentir.
