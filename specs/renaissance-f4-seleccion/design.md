# SDD design — renaissance-f4-seleccion

## Una búsqueda, dos salidas

El pool ya traía las tarjetas de método —se agranda a propósito para que no compitan con los
patrones— y hasta ahora se descartaban con `excludeTopicPrefix`. Ahora `particionarPorPrefijo` las
separa: van a `method[]` ordenadas por relevancia al pedido, y el resto al corpus. **El vector de la
consulta ya estaba calculado**, así que elegir el método no cuesta una llamada más al embebedor.

## Tres decisiones que conviene mirar

**1 · La relevancia sólo manda donde hay puntaje.** Por FTS lo que llega es un match léxico, no una
medida de relevancia. Usarlo para ELEGIR el método hacía desaparecer del brief una tarjeta buena sólo
porque no compartía palabras con el pedido — y sin que nadie se enterara. Mismo criterio que el piso
de F3: donde no hay puntaje, no se toman decisiones de ranking. La diferencia se declara en
`method_source`: `relevancia` | `importancia` | `static`.

**2 · En modo semántico NO hay fallback, y es a propósito.** La primera versión caía al orden por
importancia cuando el piso dejaba el método vacío — y eso volvía a meter exactamente las tarjetas que
el piso acababa de descartar, deshaciendo en silencio la decisión recién tomada. Lo agarró
`TestDesignElNucleoNoDependeDelAcervo`. Si el acervo no tiene método para este pedido, el criterio
igual viaja: el núcleo universal vive en `principles` y es del código (F1+F2).

**3 · La reserva de artículos no da vuelta la prioridad, sólo abre lugar.** `preferCuratedSources`
ponía TODAS las tarjetas destiladas antes que todos los artículos: ese orden resolvió el problema de
2026-08-20 (las tarjetas cortas perdían en similitud contra blobs de miles de tokens) y con 1.438
tarjetas pasó a causar el opuesto — los artículos dejaron de entrar. Ahora lo curado se queda con la
mayoría de los lugares y los artículos con `designReservaCrudos`.

## El pool

`designPoolMax` = 300, deliberadamente **mayor que `maxLimit`**: `maxLimit` acota lo que un caller
puede PEDIR, esto acota lo que el motor MIRA antes de elegir. Son cosas distintas y confundirlas es lo
que dejaba los artículos fuera de alcance.

## La diversidad

`diversificar` es MMR (Carbonell y Goldstein 1998) con solape léxico como medida de parecido:
`relevancia − λ·max(parecido con lo ya elegido)`, λ = 0,45. Model-free y determinista. Sin similitud
(camino léxico) usa la posición invertida como relevancia, para que el criterio siga teniendo con qué
comparar.

El sabotaje de I-SEL3 no describe el comportamiento anterior: lo **implementa** (`porSimilitudPura`) y
compara contra él, para que el test no dependa de que alguien recuerde tocar una constante.

## Un test que venía pasando por coincidencia

`TestDesignMethodExcluidoDelCorpus` afirmaba que la tarjeta de método aparecía en `Principles`. Desde
F1+F2 eso es falso —`Principles` es el núcleo estático— pero seguía en verde porque el núcleo dice,
palabra por palabra, «una sola cosa manda por pantalla»: la misma frase que la tarjeta sembrada. Un
test que pasa por el texto de otra cosa no defiende nada. Ahora afirma el contrato real.
