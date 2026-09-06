# Spec — Profundidad derivada del diff

## La tabla de escalones

Seis señales aportan 0-2 puntos; la séptima, 0-1. Total **0..13**.

| señal | 0 puntos | 1 punto | 2 puntos |
|---|---|---|---|
| archivos | ≤ 2 | 3-9 | ≥ 10 |
| líneas (agregadas + borradas) | ≤ 30 | 31-200 | ≥ 201 |
| símbolos tocados | ≤ 2 | 3-9 | ≥ 10 |
| paquetes | 1 | 2-3 | ≥ 4 |
| callers en el radio | 0 | 1-9 | ≥ 10 |
| paquetes en el radio | ≤ 1 | 2-3 | ≥ 4 |
| hay borrados | no | sí | — |

## Los tres niveles

| puntos | nivel | jueces | rondas | quórum |
|---|---|---|---|---|
| 0-2 | `minima` | 1 | 1 | 1 |
| 3-6 | `estandar` | 3 | 2 | 2 |
| ≥ 7 | `profunda` | 5 | 2 | 3 |

**El piso** no es un gusto: nunca menos de un juez y una ronda. Lo que baja con un cambio chico es
el costo, no la existencia de la revisión.

**El techo tampoco:** nunca más de **dos rondas**, porque dos es lo que el motor de debate soporta.
Con más, `AdvanceDebate` se vuelve un no-op mudo y el panel *cree* que debatió cuando no debatió —
peor que no debatir, porque queda registrado como que sí.

## 🔴 El piso de honestidad: el cero del radio es ambiguo

El radio de impacto es estructuralmente un **piso, nunca un techo**: el grafo omite a propósito las
llamadas que no resuelve, y sólo indexa `.go` salvo que el binario traiga los seis tags de
treesitter.

Entonces **«radio 0» significa dos cosas incompatibles** — «no arrastra a nadie» y «no puedo
saberlo» — y las dos puntúan igual: cero. Ese es el valor de fallo disfrazado de valor
tranquilizador: si se le cree, un cambio en código sin indexar puntúa como el más inocuo posible.

Por eso:

1. Antes de creerle a un cero se comprueba que el **nodo exista** (`GetGraphNodeCtx`).
2. Si algún archivo tocado no es indexable, no tiene nodos, o el grafo no contesta, se marca
   `radio_ciego` y el nivel **no puede bajar a `minima`**.
3. **El motivo va escrito** en el veredicto. Subir el nivel en silencio dejaría al lector sin poder
   distinguir «el cambio es grande» de «el índice está flojo», que son cosas muy distintas.
4. El piso **sólo sube**: un cambio ya profundo con el radio ciego sigue profundo.

Efecto práctico que conviene aceptar de entrada: en este repo hay `.go` sin un solo nodo en el
grafo, así que `minima` va a ser **raro** hasta que alguien indexe. Es correcto y es honesto, pero
va a parecer un bug si nadie lo escribe antes. Queda escrito.

## El borrado, que era el punto ciego

`parseHunkNewRange` **descarta** los hunks de borrado puro, y hace bien: los rangos son coordenadas
del estado nuevo y un borrado no tiene estado nuevo. Correcto para cruzar hunks con símbolos, y
engañoso para medir el **tamaño** del cambio: sin contadores propios, borrar doscientas líneas mide
exactamente igual que no tocar nada.

Por eso `FileDiff` gana `Agregadas` y `Borradas`, contadas del cuerpo del diff, y `líneas` es la
suma de las dos. La séptima señal marca además que hubo borrado, sea un archivo entero o un hunk.

## Invariantes (cada uno con su test y su sabotaje verificado en rojo)

| # | invariante | por qué importa |
|---|---|---|
| P1 | ninguna señal está cableada de adorno: cada una, sola, sube el puntaje | una señal que nunca mueve el resultado es indistinguible de una rota, hasta que alguien confía en ella |
| P2 | los escalones caen exactamente donde dice la tabla | un borde corrido cambia el panel de cambios enteros |
| P3 | el total vive en 0..13 | el techo define los cortes |
| P4 | los cortes de nivel son 2 / 6 | los casos se arman con señales reales, no con un total inventado |
| P5 | un radio ciego no puede dar `minima`, y el motivo viaja | ver arriba |
| P5b | el piso sólo sube, nunca baja | un piso que baja es un techo disfrazado |
| P6 | ningún panel sale del piso ni del techo | dos rondas es lo que el motor soporta |
| P7 | un hunk que sólo borra cuenta | era el punto ciego |
| P8 | la derivación cuenta archivos, paquetes y líneas como dice | binarios afuera |
| P9 | el desglose suma el total y trae las siete | un desglose que no cuadra invita a verificar y devuelve una verificación falsa |

## Contrato de salida

`musubi_detect_changes` gana el campo `revision` con: `nivel`, `puntos`, `panel`
(`jueces`/`rondas`/`quorum`), `senales` (las siete, con su valor y su punto) y `motivos`.

El tope de sondeo (25 símbolos) **no se calla**: si recorta, lo dice en `motivos`. Un recorte
silencioso se lee como «se miró todo» justo cuando es lo contrario.
