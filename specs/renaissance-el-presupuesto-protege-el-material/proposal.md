# El presupuesto protege el material, no el sermón

Continúa [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md). No es una fase
nueva del plan de 9: corrige el **reparto** que dejaron F1 y F2.

## El problema, medido

Pedido del usuario tras seguir usando `musubi_design` en Altura: *«no hay problema si gasta un poco
más, lo importante es que haga un diseño bueno y no genérico»*.

Tenía razón, y el dato lo respalda: F1+F2 bajaron el brief de 5.850 a 2.400 tokens y **M3 (precisión)
se movió de 0,22 a 0,26; M1 (estabilidad) quedó clavada en 0,09**. El recorte compró abstención
(0 → 0,88) y cerró la inyección — o sea **seguridad, no calidad de diseño**.

Medido el 2026-08-30 contra el central, sobre un brief real de 2.533 tokens:

| bloque | ~tok | ¿varía con el pedido? |
|---|---|---|
| `method` | 439 | parcial |
| `brand` + `brand_tokens` | 684 | no |
| **`corpus`** | **251** | **sí** |
| `emit`+`precedence`+`principles`+`role`+`material_note`+`instructions` | 955 | no |

**El 66 % del brief era constante, y el material con el que se compone —el corpus— eran cuatro
titulares de 86 a 92 chars: el 10 %.** Las tarjetas completas miden 245 chars de mediana y no se
servían: se servía el gist y una nota que mandaba a `musubi_memory_expand`. El motor le entregaba al
agente **cuatro títulos y un sermón universal**.

Y una parte era autoinfligida: `precedence` + `material_note` + `role` + `instructions` +
`corpus_note` sumaban **2.475 chars (25 % del brief) explicando las reglas del brief**. Es la
gobernanza que agregaron F1 y F2 — el mismo defecto que esta casa le encontró a otros motores el
mismo día.

**El tope no estaba mal calibrado, estaba mal REPARTIDO.** Apretarlo más habría empeorado justo lo
que faltaba.

## Un segundo defecto, encontrado por el banco a mitad de camino

Con el presupuesto instrumentado apareció algo que ninguna métrica miraba: **con una marca gigante el
brief salía con `corpus: 0`, `method: 0` y `degraded` en FALSO**. La escalera de `cederUnItem` vaciaba
método y corpus hasta cero antes de tocar la marca. Un brief sin una sola pieza de conocimiento de
diseño, con cara de completo, entregado a alguien que pidió que le diseñen algo.

Que la marca gane por **precedencia** no es lo mismo que ganar por **espacio**: la precedencia decide
quién manda cuando dos partes se contradicen, no la autoriza a quedarse con todo el canal.

## Qué cambia

1. **El corpus viaja entero.** `patronItem` reemplaza a `searchHit` en el brief: mismo formato que
   `metodoItem` (topic + fuente + texto + procedencia), con tope por tarjeta (`designPatronItemMax`
   = 1.800) para que un artículo `ingested/*` de 12.000 chars no se lleve el brief puesto.
2. **El presupuesto sube de 2.600 a 6.000 tokens.**
3. **Los pisos del material son duros frente a la marca.** La marca cede —con su aviso ruidoso— antes
   de que el corpus baje de `designPisoCorpus` (5) o el método de `designPisoBloque` (3). El tope duro
   sigue siendo lo único innegociable: como último recurso los pisos se rompen, y queda declarado.
4. **El piso del corpus es más alto que el del método.** Cuando falta lugar sobrevive lo específico,
   no lo universal.
5. **La gobernanza se comprime**, sin perder un invariante: mismos textos, un tercio del canal.
6. **La compuerta del banco pasa de M4 a M5.**

## Por qué subir el techo no reabre el 2026-08-21

Es la pregunta que este cambio tiene que contestar, porque la regresión original **fue más tokens**:
30 tarjetas de método, 16.728 chars idénticos para cualquier pedido.

La compuerta ya no es el tamaño sino la **especificidad**. `m5_fraccion_variable_min` pasa de 0,13 a
0,40: para gastar más, el motor tiene que traer más material que cambie con el pedido.

Verificado con el sabotaje, no supuesto: reponiendo el sermón constante, **M5 se derrumba de 0,45 a
0,24 y el banco se pone rojo — mientras `m4_tokens_max` queda en 5.946, bajo el techo.** El tamaño
solo no lo habría atrapado; la fracción variable sí. Es exactamente la falla que M4 no supo ver a
tiempo en agosto.

## Qué NO hace este cambio

- No toca la recuperación: el mismo pool, el mismo piso de similitud, la misma selección. Sube lo que
  se sirve de cada patrón elegido, no cambia cuáles se eligen.
- No promete mover M3 ni M1. **Si con el corpus completo no se mueven, queda demostrado que el
  problema no es cuánto material sino cuál**, y eso empuja a la taxonomía (ver
  `renaissance/styler-ruteo-vs-similitud`). Ese es un resultado útil, no un fracaso.
- No agrega el checklist anti-genérico. Es conocimiento que hay que escribir con los tells medidos
  del motor real, y va sobre este presupuesto una vez que exista.
