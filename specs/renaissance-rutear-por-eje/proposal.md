# Rutear por eje — la taxonomía del acervo de diseño

Fase 2 del [plan de cierre](../renaissance-el-presupuesto-protege-el-material/proposal.md) de
[renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md).

## El problema, medido

Dos maneras de pedir lo mismo devolvían material distinto: **M1 = 0,10** sobre los 16 pedidos del
set dorado, con tres pedidos en 0,00. Y no lo arreglaba nada de lo que probamos antes — ni servir el
corpus completo (M1 pasó de 0,09 a 0,10), ni cambiar qué parte de un artículo se sirve.

La causa, medida contra el acervo real: **dos tarjetas de diseño al azar se parecen tanto como una
consulta a su mejor resultado.** Coseno p99 entre pares al azar **0,668**; una consulta real con su
mejor match, **0,533–0,643**. A esa granularidad el embebedor no separa nada, y por eso ningún cambio
de ranking movía la aguja.

## El hallazgo que abre la puerta

**El mismo embebedor SÍ discrimina entre 19 ejes bien separados.**

| | estabilidad entre las 3 paráfrasis |
|---|---|
| eje top-1 | **0,73** |
| tarjetas (el motor hoy) | 0,10 |

El paso consulta→eje es **7× más estable** que consulta→tarjeta. No era un embebedor malo: era la
granularidad equivocada.

## M1 simulada de punta a punta, antes de escribir código

Sobre el acervo real de 1.736 entradas y los 16 pedidos dorados, sin tocar el motor:

| método | M1 |
|---|---|
| motor hoy — similitud sobre 1.438 tarjetas | 0,10 |
| ruteo léxico por eje | 0,23 |
| eje por embebedor **top-2** → tarjetas | 0,30 |
| eje por embebedor **top-1** → tarjetas | **0,50** |

**Top-1 y no top-2**, que es contraintuitivo: sumar el segundo eje EMPEORA. El segundo eje es
bastante menos estable y lo único que aporta es varianza.

El criterio de muerte de esta fase era «si rutear no supera a rankear, se descarta». Lo supera 5×.

## El diseño

1. **19 ejes con su descripción y su vocabulario** (`ejes_diseno.go`). Se embebe la **descripción**,
   no el nombre: el acervo casi nunca dice «a11y», dice «contraste» y «lector de pantalla».
2. Los 19 vectores se calculan **una vez por proceso** y se cachean. Son constantes del binario.
3. Con el **mismo vector** de la consulta que ya se calculaba —así que no cuesta una llamada más— se
   elige el eje top-1 y se sirven sus tarjetas **ordenadas por importancia**.
4. Por debajo del piso de similitud NO se rutea: se cae al camino de siempre. Un pedido que no se
   parece a ningún eje es justo donde forzar una taxonomía inventaría una respuesta.
5. El brief declara `retrieval: "eje"` y `axis: "<nombre>"`.

**Importancia y no similitud** para ordenar dentro del eje: el orden por similitud es exactamente el
que no es reproducible entre paráfrasis. Una vez que el eje acotó el tema, el desempate tiene que ser
una propiedad de la TARJETA y no de cómo se escribió el pedido.

## Dos cosas que apareció construyendo

**El ruteo se saltaba la diversificación de F4** y el top-6 volvía a poder ser seis paráfrasis de la
misma idea. Lo agarró `TestDesignElTopKNoColapsaEnLoMismo`. Se arregla trayendo `designHolguraEje`
candidatos por lugar, para que `elegirCorpus` tenga de dónde elegir.

**Y un bug latente que ya estaba:** sin similitud, la relevancia para el MMR era `1 - i/len(src)`, o
sea que **el valor de la sexta tarjeta cambiaba según cuántos candidatos se hubieran traído**. Ahora
es rango recíproco `1/(1+i)`, que es libre de escala. Con el ruteo el defecto se volvió visible
porque todos los candidatos llegan sin similitud.

## Lo que esta fase NO hace

- **No mide M3 por eje**, y es deliberado: si se etiqueta por eje y después se pregunta «¿la tarjeta
  servida toca un eje?», da 1,00 **por construcción**. M1 sí es honesta — que tres paráfrasis caigan
  en el mismo eje no lo garantiza el etiquetado.
- **No llega a M1 = 0,80.** El techo con esta taxonomía es 0,73 (la estabilidad del eje), y la
  simulación da 0,50 porque sólo el **41 %** del acervo recibe etiqueta con el vocabulario léxico.
  Los dos levers quedan medidos y abiertos.
- **No mejora las descripciones de eje.** `login` resultó un **eje imán**: aparece de top-2 en
  pedidos que no tienen nada que ver. Va al plan siguiente.
