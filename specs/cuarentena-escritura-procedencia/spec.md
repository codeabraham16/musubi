# Spec — Cuarentena de escritura y procedencia (F4)

Contrato observable. Cada invariante está numerado y tiene una prueba que **sabe fallar**: si se
rompe la propiedad, el test se pone rojo. Un invariante sin test que lo pueda tumbar es decoración.

---

## Alcance

El **libro mayor de observaciones**. El grafo de hechos queda fuera a propósito: ya tiene sus dos
murallas (`musubi_propose_facts` con `source='llm-extract:<model>'`, no autoritativo, fuera de
`recall_facts` por default y sin poder invalidar nada). Duplicar eso sería inventar en vez de cablear.

| Superficie | Estado |
|---|---|
| `observations` (libro mayor) | **en alcance** — hoy no tiene ningún sello de procedencia |
| `facts` (grafo) | fuera: ya resuelto en el 3er pilar |
| Embeddings | fuera: no producen contenido, producen vectores |

---

## Taxonomía de procedencia

Tres valores, cerrados. Un valor fuera del conjunto es un error, no un default silencioso.

| Sello | Qué significa | Cómo se obtiene |
|---|---|---|
| `human` | Lo decidió una persona (posiblemente tipeado por un agente en su nombre) | `musubi_save_observation` — el camino de siempre |
| `deterministic` | Lo derivó código sin modelo (hooks de captura, `detect_changes`, indexadores) | `musubi_save_observation` declarándolo |
| `llm:<modelo>` | Lo generó un modelo de lenguaje | **sólo** `musubi_propose_observation` |

`human` es el default de las filas que ya existen. Es una decisión consciente y hay que decirla de
frente: **no sabemos** qué generó cada fila vieja, pero todas entraron por un camino donde una
persona o su agente eligió el contenido, y no había motor de cognición escribiendo. Marcarlas
`human` describe lo que efectivamente son; marcarlas `unknown` agregaría ruido a cada recall para
no informar nada.

---

## Invariantes

### Q0 — Una observación en cuarentena no aparece en ningún camino de LISTADO *(el invariante fundamental)*

Para toda observación con `quarantined = 1`, **ninguna operación que enumere o descubra
observaciones la incluye**: recall léxico, recall vectorial, priming, contexto de entidad, vecinos
para conflictos, consolidación, estadísticas, backfill de embeddings, cola del sync saliente **y el
grafo neuronal que alimenta el dashboard**.

> Es la razón de existir de la muralla. Si sólo se pudiera verificar uno, es este. Y hay que
> probarlo **por cada camino**, no sólo por el léxico: una muralla con una puerta lateral no es
> una muralla.

**Por qué dice «listado» y no «recall».** La primera redacción de este invariante decía "ninguna
llamada a `Recall` la devuelve". Al verificarla contra el código apareció que `brainGraphAt`
—el grafo del dashboard— no es `Recall` y no usaba el predicado canónico: la spec se cumplía al
pie de la letra mientras el dashboard dibujaba memoria en cuarentena. Un invariante que se cumple
mientras el daño ocurre está mal escrito, no bien cumplido.

### Q0b — La hidratación por id explícito queda FUERA, y se dice

`GetObservationsBudgetCtx` (`musubi_memory_expand`), la lectura de gist de la banda y la lectura de
detalle del detector de conflictos leen **por id** sin el predicado de visibilidad. Eso se mantiene
así a propósito: exigen conocer el id de antemano, y ningún camino de listado va a entregarlo.

No es un descuido, es la frontera del invariante: **Q0 impide descubrir, no impide leer lo que ya
sabés que existe.** Dejarlo implícito sería peor que la excepción — el próximo que lea el diseño
asumiría cobertura total.

### Q1 — Toda observación tiene sello

No existe fila con `provenance` NULL o vacío. Ni las nuevas, ni las migradas, ni las que entran por
sync desde el central.

### Q2 — El sello no se puede falsificar desde el camino de cuarentena

`musubi_propose_observation` **fuerza** `provenance = 'llm:<modelo>'` y `quarantined = 1`. No expone
ningún parámetro para pedir otra cosa. Un modelo no puede declararse `human` porque la puerta por la
que escribe no tiene esa perilla.

> El sello es *por dónde entraste*, no *qué dijiste que sos*. Eso lo hace estructural en vez de
> declarativo, que es la misma decisión que en F1 hizo imposible construir un motor sin portero.

### Q3 — El recall muestra el sello de lo que no es `human`

Un ítem con procedencia `llm:*` o `deterministic` llega al caller **marcado**. Una inferencia de un
modelo nunca se presenta con la misma cara que una nota verificada.

### Q4 — Salir de cuarentena es explícito y deja rastro

Ninguna observación sale de cuarentena sola: ni por antigüedad, ni por accesos, ni por decaimiento,
ni porque el recall la haya rozado. Sólo por una acción explícita, que **conserva el sello de
procedencia** — corroborar no convierte una inferencia en un hecho humano, sólo la hace visible.

### Q5 — Bit-identidad del camino existente

Sin llamar a la tool nueva, el comportamiento es idéntico al de antes: toda fila queda
`provenance='human'`, `confidence=1.0`, `quarantined=0`, y el recall devuelve exactamente lo mismo.

### Q6 — La cuarentena no se filtra por sync ni por federación

Una observación en cuarentena **no se promueve a `shared`** y **no entra al outbox**. Sin esto, el
texto de un LLM sin corroborar viajaría al cerebro central y aparecería como memoria de equipo en
las máquinas de otras personas — que es exactamente el daño que la muralla existe para evitar,
multiplicado.

### Q7 — La confianza es un número acotado y honesto

`confidence ∈ [0,1]`. Un valor fuera de rango se rechaza en la frontera, no se recorta en silencio.
Las filas existentes valen `1.0`: entraron por un camino donde alguien decidió el contenido.

---

## Configuración

Sin config nueva. La muralla no tiene interruptor a propósito: un `quarantine.enabled: false` sería
exactamente el agujero que esto viene a tapar, y el proyecto ya aprendió en F1 que un apagado
disponible termina apagado.

Lo que sí es opt-in es *usar* la puerta: sin llamar a `musubi_propose_observation` no hay ninguna
fila en cuarentena y nada cambia.

---

## Criterios de aceptación

1. Los 9 invariantes con test propio, y cada test verificado **fallando** al sabotear la
   implementación (un test que nunca se vio en rojo no prueba nada).
2. Q0 probado **por cada camino de listado** por separado, no sólo por el léxico. Incluye el grafo
   neuronal, que es el que se había escapado en la primera lectura.
3. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde.
4. Una base creada con el esquema viejo migra sin perder filas y con todas las columnas pobladas.
5. Test adversarial: `confidence` fuera de rango, procedencia inventada, `propose` con el id de una
   observación existente, corroborar algo que no está en cuarentena, y una observación en cuarentena
   intentando llegar al central por las dos vías (`promote` y outbox).
