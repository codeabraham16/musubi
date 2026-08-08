# Propuesta — El banco mide a escala real

Track: **Potencia medida**, F2.3. Sigue a `specs/juez-medible/`, que enchufó el juez al banco pero
dejó anotado el hueco:

> *«El fixture no se amplía. 26 docs / 12 queries alcanza para verificar que el brazo funciona, no
> para que una diferencia de MRR signifique algo.»*

## Lo que yo dije, y por qué estaba mal

Escribí que el fixture *«tiene que salir de memoria real, no inventada»*. **El repo de Musubi es
público** (verificado con `gh repo view`: `"visibility": "PUBLIC"`).

Volcar memoria real a `testdata/` la publicaría en GitHub — y esa memoria tiene IPs del tailnet,
nombres de servicios y decisiones internas. Mirando el `golden.json` que ya existía, la convención
correcta ya estaba tomada: sus docs son **sintéticos**, escritos a mano en el dominio de Musubi.

## El problema real

El fixture dorado tiene **26 docs y 12 queries**, con 1 o 2 relevantes cada una. A esa escala, que
una query cambie de lugar mueve el MRR ~8 puntos: cualquier diferencia entre dos jueces queda
enterrada en el ruido. Sirve para verificar que el cableado funciona; no para decidir nada.

## Qué se construye

Un **generador** que arma el fixture desde una base de memoria real, en sólo lectura, y **sin
escribir nada al repo**: el fixture vive en memoria mientras dura la corrida.

Los dos fixtures tienen trabajos distintos y ninguno reemplaza al otro:

| | dorado (sintético, versionado) | real (generado, efímero) |
|---|---|---|
| para qué | red de regresión en CI | la medición |
| tamaño | 26 docs / 12 queries | **1.210 docs / 26 queries** |
| corre en CI | sí | no (se saltea sin `MUSUBI_FIXTURE_DB`) |

## De dónde salen las etiquetas de relevancia

Es la única pregunta que importa, porque un fixture automático con etiquetas malas es peor que uno
chico con etiquetas buenas.

**Del `topic_key`.** Las observaciones que comparten topic son las relevantes para una consulta sobre
ese topic. Lo que lo hace defendible: **el topic lo asignó el autor al escribir la nota**, con total
independencia de cómo se recupera. Derivarlas del propio ranking —o de un LLM— haría un banco
circular, que es la trampa en la que ya caí dos veces esta misma jornada.

## Lo que este etiquetado NO puede hacer, dicho ahora

Asume dos cosas falsas: que todo lo del topic es igual de relevante, y que **nada fuera del topic lo
es**. Una nota en `roadmap/track-potencia-medida` y otra en `cognicion/donde-esta-encendido-el-motor`
pueden ser las dos relevantes para «el motor de cognición», y acá la segunda cuenta como fallo.

Consecuencia: **los absolutos salen subestimados** y no hay que leerlos como «el recall es malo». Lo
válido es el **delta entre dos arms** sobre el mismo fixture, porque el sesgo es idéntico para los
dos — y el delta es exactamente lo que F2 vino a medir.

## Lo que NO se construye

- **El fixture dorado no se amplía ni se reemplaza.** Sigue siendo la red de regresión de CI.
- **No se corre contra el motor real.** Sigue necesitando el central o el túnel con
  `LITELLM_MASTER_KEY`. Este spec deja el banco listo a escala; encender el motor es el paso final.
- **No se etiqueta a mano.** Sería el patrón oro y es tiempo del dueño; si algún día hace falta, el
  formato de `Fixture` ya lo admite.
