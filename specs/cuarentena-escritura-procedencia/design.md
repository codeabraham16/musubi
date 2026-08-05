# Diseño — Cuarentena de escritura y procedencia (F4)

## La decisión que sostiene todo lo demás

`internal/memory/scope.go:33` define el predicado canónico de "observación visible":

```go
const visibleObsPredicate = "archived = 0 AND superseded_by IS NULL"
```

Su propio comentario dice que se centralizó **para dejar el seam** de futuros filtros. Diez
consultas repartidas en nueve archivos lo concatenan en vez de repetir el literal:

| Archivo | Camino |
|---|---|
| `operations.go:459`, `operations.go:517` | recall **vectorial** (dos variantes) |
| `operations.go:586` | recall **léxico** (FTS5) |
| `prime.go:28` | priming de arranque |
| `context.go:82` | contexto de entidad |
| `conflicts.go:519` | vecinos para detección de conflictos |
| `consolidate.go:68` | consolidación |
| `opstats.go:49` | estadísticas |
| `embed_backfill.go:23` | backfill de embeddings |
| `inboundsync.go:62` | **cola del sync saliente hacia el central** |
| `braingraph.go:86` | **grafo neuronal del dashboard** — *no lo usaba; se sumó en esta fase* |

Entonces Q0 se cumple **agregando tres palabras en un solo lugar**:

```go
const visibleObsPredicate = "archived = 0 AND superseded_by IS NULL AND quarantined = 0"
```

Esto importa más de lo que parece. La alternativa —poner el filtro en cada consulta— deja la muralla
a merced de que el próximo que escriba una query se acuerde. Acá **la única forma de escribir una
consulta de recall que se saltee la cuarentena es no usar el predicado canónico**, que es una
desviación visible en el diff.

Y hay un regalo: `inboundsync.go:62` es la cola del sync saliente. El mismo cambio da **Q6 gratis**:
una observación en cuarentena no puede viajar al cerebro central.

### El que NO estaba en la lista, y casi se escapa

`braingraph.go` filtraba con un `archived = 0` **propio** en vez del predicado canónico. Como el
grafo neuronal alimenta el dashboard —la cara visual del cerebro—, una observación en cuarentena se
habría dibujado ahí como neurona. La primera versión de este diseño afirmaba cobertura total sin
haberlo verificado; apareció al revisar.

Al pasarlo al predicado canónico se corrige además una inconsistencia **anterior a esta fase**: el
grafo también dibujaba las observaciones REEMPLAZADAS (`superseded_by`). Eso **cambia lo que muestra
el dashboard** y está decidido a conciencia — una nota superada no es memoria viva — pero es un
cambio visible que no se esconde en el diff de otra cosa.

> La lección para el próximo: que exista un predicado canónico no garantiza que todas las consultas
> lo usen. Hay que ir a buscar las que no.

### Los caminos que quedan afuera a propósito

`GetObservationsBudgetCtx` (`musubi_memory_expand`), `band.go:154` (gist para la banda) y
`conflicts.go:372` (detalle para el detector) leen **por id** sin el predicado. Se deja así: exigen
conocer el id de antemano y ningún camino de listado lo entrega. **Q0 impide descubrir, no impide
leer lo que ya sabés que existe.** Está escrito acá y en la spec (Q0b) para que nadie lo lea como
cobertura total.

Y `promote` (`local → shared`) tampoco pasa por el predicado: es un UPDATE por id. Se bloquea
explícitamente en `PromoteObservationCtx`, con test propio.

## Esquema

Migración **v22** (la última es la 21), tres columnas con default, sin reescribir la tabla:

```sql
ALTER TABLE observations ADD COLUMN provenance  TEXT    NOT NULL DEFAULT 'human';
ALTER TABLE observations ADD COLUMN confidence  REAL    NOT NULL DEFAULT 1.0;
ALTER TABLE observations ADD COLUMN quarantined INTEGER NOT NULL DEFAULT 0;
```

Los defaults hacen el backfill solos: `ADD COLUMN ... NOT NULL DEFAULT` en SQLite rellena las filas
existentes con el default. Q1, Q5 y Q7 quedan satisfechos para lo viejo sin una sola pasada de
escritura.

**Sin índice sobre `quarantined`, a propósito.** Casi todas las filas valen 0, así que un índice
sobre una columna de baja cardinalidad no lo usaría el planificador y sólo costaría escrituras. El
predicado ya viaja pegado a `archived = 0`, que tiene el mismo perfil y tampoco lo tiene.

## Las dos puertas

```
                        provenance        confidence   quarantined
musubi_save_observation   'human'            1.0            0        ← el camino de siempre
   (o 'deterministic' si el caller lo declara)
musubi_propose_observation 'llm:<modelo>'  del caller       1         ← puerta nueva, forzada
musubi_corroborate         (se conserva)   (se conserva)    0         ← salida explícita
```

`musubi_propose_observation` está modelada sobre `toolProposeFacts`, que ya resuelve este mismo
problema para el grafo: mismo patrón de `model` → `caller` por default, misma redacción forzada al
central, misma atribución por credencial (`writeOriginFor`).

**Q2 es estructural, no declarativo.** La tool no expone parámetro de procedencia ni de cuarentena:
los escribe ella. Un modelo no puede declararse `human` porque la puerta por la que escribe no tiene
esa perilla. Es la misma decisión que en F1 hizo imposible construir un motor sin portero — el
sello es *por dónde entraste*, no *qué dijiste que sos*.

`musubi_corroborate` **conserva el sello de procedencia**. Corroborar no convierte una inferencia en
una nota humana; sólo la hace visible. Un `llm:groq/llama-3.3` corroborado sigue diciendo que salió
de un modelo, y el recall lo sigue marcando (Q3).

## Por qué no se llama `promote`

`musubi_promote` ya significa `local → shared` en la memoria híbrida. Son dos ejes distintos:

| Eje | Valores | Pregunta que responde |
|---|---|---|
| `scope` | `local` / `shared` | ¿viaja al cerebro central? |
| `quarantined` | `1` / `0` | ¿tiene autoridad para aparecer en el recall? |

Reusar el nombre ahorraría una tool y costaría que alguien, dentro de seis meses, promueva a
`shared` creyendo que está corroborando. La colisión de nombres es una trampa, no una economía.

## Procedencia en el recall (Q3)

`RecallItem` gana `Provenance string`. El formateador lo muestra **sólo cuando no es `human`**: si
todo dijera `[human]` el sello se volvería ruido y dejaría de leerse, que es el mismo mecanismo por
el que una alarma siempre encendida enseña a ignorar el rojo.

## Validación en la frontera (Q7)

`confidence` fuera de `[0,1]` se **rechaza**, no se recorta. Recortar en silencio convierte el error
de quien llama en un dato plausible y equivocado guardado para siempre. Y una procedencia fuera de
la taxonomía es error, no default silencioso — misma regla que el `mode` desconocido del gateway de
F1, que apaga el pilar entero en vez de adivinar.

## Riesgos, dichos de frente

- **Una base vieja migrada queda con todo en `human`.** No sabemos qué generó cada fila. Está
  argumentado en la spec, pero es una afirmación que el esquema no puede respaldar: es la mejor
  descripción disponible, no una verdad verificada.
- **La cuarentena no llega retroactivamente.** Si alguien ya guardó una respuesta de `musubi_ask`
  como observación normal, esta fase no la encuentra ni la marca. Cierra la puerta de acá en
  adelante; no audita el pasado.
- **Un caller puede seguir usando `save_observation` para contenido de LLM.** Q2 protege la puerta
  de cuarentena, no obliga a usarla. La honestidad del sello depende de que el agente use la tool
  correcta — igual que hoy depende de que use `propose_facts` y no `save_fact`. Cerrar eso del todo
  exigiría detectar texto generado por IA, que es justo lo que la propuesta dice que esto no es.
