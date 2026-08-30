# F3 · Saber cuándo no se sabe

Fase 3 de [renaissance-rey-del-diseno](../renaissance-rey-del-diseno/proposal.md).

## El problema, medido

El 2026-08-29 le pedí al motor **una receta de empanadas** y devolvió seis patrones de diseño con
`degraded` apagado — la misma cara de confianza que ante un pedido legítimo. Lo mismo con
«xqzvbnm qwrtyp», con `!!!!!!!!` y con un solo carácter. Siete de siete.

La separación EXISTE y no se usa:

| | similitud del mejor resultado |
|---|---|
| pedidos legítimos | 0,533 – 0,558 |
| basura y temas ajenos | 0,362 – 0,442 |

`degraded` sólo se enciende si la búsqueda devuelve **cero filas**, y por el camino semántico eso no
pasa nunca: siempre hay algo con similitud mayor que cero. Es el antipatrón de la casa — **el valor
de fallo es idéntico al valor tranquilizador**.

Y hay un segundo silencio: con un prompt de 25 KB o más, el embebedor agota su timeout de **30 s** y
el motor cae a búsqueda léxica **sin decirlo** (desaparece el campo `similarity`, `degraded` sigue
apagado). Treinta segundos con una persona esperando no es una espera, es un fallo — y de paso es un
vector de saturación barato contra un embebedor que se comparte con recall y save.

## Qué entrega

1. **Piso de similitud.** Un patrón por debajo del piso no se sirve. Si ninguno lo pasa, el motor lo
   dice en vez de rellenar.
2. **`degraded` con causa**: `sin_material` · `bajo_umbral` · `sin_recuperador`.
3. **El modo de recuperación se declara** siempre: `retrieval: "semantico" | "fts"`. Que el caller
   sepa con qué lo buscaron.
4. **Timeout del embebedor 30 s → 5 s.** Falla rápido, cae a léxico, y lo declara.

## Honestidad sobre el alcance

El piso es una comparación de similitud, así que **sólo se puede aplicar por el camino semántico**.
Por el camino léxico (FTS) no hay puntaje que comparar: ahí la abstención sigue siendo la que el
propio FTS produce cuando no matchea nada. Se declara el modo para que la diferencia sea visible en
vez de invisible.

Por eso M2 se mide de verdad en la **sonda** (embebedor real), no en el banco estructural, que corre
sobre FTS. En el banco se prueba la ARITMÉTICA del piso con un embebedor determinista de prueba —
que es testear la lógica, no la calidad del recuperador.
