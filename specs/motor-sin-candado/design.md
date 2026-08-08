# Diseño — El motor no traba la casa

Seis decisiones, cada una con su alternativa descartada.

---

## D1 · La clase de candado es un campo nuevo cuyo cero es lo de hoy

`toolEntry` gana un campo, y el cero de Go **es** el comportamiento actual:

```go
type lockClass int

const (
	// lockFromReadOnly (el cero) es lo histórico: el dispatcher deriva el candado de readOnly.
	// Es el cero A PROPÓSITO — las 53 tools que no hablan con la red no se tocan.
	lockFromReadOnly lockClass = iota
	// lockSelf: el dispatcher NO toma nada; el handler acota su propia sección crítica.
	// Reservado para handlers con I/O externa (motor LLM, embedder).
	lockSelf
)
```

**Por qué el cero y no una enumeración explícita.** Una clase obligatoria en las 55 registraciones
serían 55 oportunidades de mover un candado sin querer, para arreglar dos tools. El cero que
reproduce el estado previo hace que el diff sea legible: **lo que cambia es exactamente lo que se
declara**.

## D2 · El dispatcher consulta un mapa precomputado, igual que hoy

`s.toolLock map[string]lockClass`, poblado en `newServer` en el mismo bucle que `toolReadOnly`. Sólo
se guardan las entradas con clase distinta de cero: un miss devuelve el cero, que es el default
correcto.

**Alternativa descartada:** recorrer el registry por llamada. `toolReadOnly` ya existe justo para no
hacer eso; agregar un recorrido lineal en el camino caliente para ahorrar un mapa sería ir para atrás.

## D3 · `readOnly` conserva el nombre y pasa a significar sólo autorización

No se renombra a `readerOK` ni nada parecido. Renombrar toca 22 registraciones, `canCall` y sus
tests, y el riesgo de que una quede afuera es real. Lo que sí cambia es su **comentario**, que hoy
afirma «el dispatch las corre bajo RLock» — a partir de acá eso es el *default*, no la regla.

## D4 · El helper acota la sección crítica y suelta con `defer`

```go
// withReadLock corre fn bajo el RLock del despacho. Para handlers en clase lockSelf: acota el
// candado al acceso a la base y deja la I/O externa AFUERA. El defer es el invariante — si fn
// entra en pánico, el candado igual se suelta y el servidor sigue atendiendo (G6).
func (s *McpServer) withReadLock(fn func()) {
	s.dispatchMu.RLock()
	defer s.dispatchMu.RUnlock()
	fn()
}
```

Firma `func()` y no `func() error`: los handlers necesitan devolver varios valores, y una clausura
que asigna a variables del scope de afuera es más simple que inventar un tipo de retorno.

**Por qué `defer` y no `RUnlock()` al final del cuerpo.** Es la diferencia entre un cuello de botella
y un deadlock. El cuello se destraba solo en 120 s; un `RLock` filtrado por un pánico no se destraba
nunca. Cuesta cero y elimina el peor final del refactor.

## D5 · Recall y Ask quedan cortados en tres tramos

```
                    embeber la consulta   ── SIN candado ── red, 30 s
    withReadLock {  leer la base       }  ── RLock       ── sin I/O externa
                    llamar al motor       ── SIN candado ── red, 120 s
```

- **`toolRecall`**: embeber → `withReadLock{ engine.Recall }` → `rerankIfEnabled`.
- **`toolAsk`**: embeber → `withReadLock{ engine.Recall + hydrateGrounding }` → `cognition.Ask`.

En `ask`, la hidratación del grounding también toca la base, así que entra en la misma sección
crítica en vez de abrir una segunda.

## Alternativas descartadas

### A · Marcar `musubi_recall` como `readOnly` y listo

Es el atajo obvio —un carácter— y es tentador porque el bump es SQL atómico. Se descarta por **dos**
razones, y la segunda es la que lo mata:

1. `readOnly` también gobierna autorización, así que abriría la tool a los principals `reader`.
   Arreglar performance abriendo un permiso no es un arreglo.
2. **No alcanzaría igual.** El `RWMutex` de Go da preferencia al escritor: un `RLock` sostenido 120 s
   bloquea a todo escritor que llegue, y un escritor en espera bloquea a los lectores siguientes. Un
   candado compartido tampoco puede cruzar una llamada de red — sólo cambia a quién hace esperar.

### B · Sacar el rerank de `toolRecall` y correrlo en el dispatcher tras soltar

Metería una dependencia de cognición en el despacho genérico y obligaría al dispatcher a conocer el
tipo concreto del resultado del recall. Capa equivocada: el dispatcher despacha, no sabe de juicio
de pertinencia.

### C · Refactorizar los 55 handlers para que cada uno tome su candado

Es el diseño correcto a largo plazo. También son 55 oportunidades de romper algo para arreglar 2, en
un servidor que ya está en producción. La clase de candado deja la puerta abierta a hacerlo después,
tool por tool, con la red de pruebas puesta.

## Rollback

Quitar `lock: lockSelf` de las dos entradas devuelve el comportamiento exacto de hoy. Sin migración
de datos, sin cambio de esquema — y `toolslist.golden.json` debe quedar byte-idéntico, que es
justamente el detector de que el campo no se filtró a la interfaz pública.
