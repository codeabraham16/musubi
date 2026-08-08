# Spec — El motor no traba la casa

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## H1 · Los dos ejes se separan

El descriptor de tool gana una **clase de candado**, independiente de `readOnly`:

| Clase | Qué hace el dispatcher | Quién la usa |
|---|---|---|
| `lockDefault` (cero) | lo de hoy: `RLock` si `readOnly`, `Lock` si no | las 53 tools restantes, **sin editar ninguna** |
| `lockSelf` | **no toma nada**; el handler declara su propia sección crítica | `musubi_recall`, `musubi_ask` |

El cero de Go es el comportamiento actual **a propósito**: una clase que hubiera que declarar en las
55 tools sería 55 oportunidades de cambiar algo sin querer, para arreglar dos.

`readOnly` conserva su nombre y pasa a gobernar **sólo autorización** (`Principal.canCall`). Sigue
siendo el default de la clase de candado, pero ahora es un default **que se puede pisar** — que es
justamente lo que hoy no se puede.

### G1 — La clase por default reproduce exactamente la de hoy

Tabla sobre las 55 tools del registry: toda tool que no declara clase corre bajo el mismo candado
que antes del cambio (`readOnly ⇒ compartido`, si no ⇒ `exclusivo`). Sin esta prueba, el refactor
puede mover el candado de una tool cualquiera y nadie se entera hasta que se pierde una escritura.

### G2 — Las escrituras siguen serializadas

Dos `musubi_save_observation` concurrentes no se pisan. Es el invariante que el lock exclusivo
existe para sostener, y hay que probar que sobrevive al cambio — no alcanza con no tocarlo.

---

## H2 · Ningún candado del dispatcher cruza una llamada al motor

`musubi_recall` y `musubi_ask` pasan a `lockSelf` y quedan en tres tramos:

```
embeber la consulta      → SIN candado   (red, timeout 30 s)
leer la base             → RLock         (acotado, sin I/O externa)
llamar al motor          → SIN candado   (red, timeout 120 s)
```

El tramo del medio es lo único que necesita candado, y el bump de accesos que lo justificaba es una
sentencia SQL atómica (`access_count = access_count + 1`), no un read-modify-write de Go.

### G3 — Un `musubi_ask` lento no bloquea a otra tool

Con un motor falso que se cuelga hasta que la prueba lo suelta: un `musubi_ask` en vuelo y un
`musubi_save_observation` en paralelo ⇒ **el save termina sin esperar al ask**. Es el invariante
central del spec. Medido hoy: ese ask tardó 25 s y bloqueó todo.

### G4 — Un `musubi_recall` con el juez encendido tampoco bloquea

Mismo montaje, con `read_time_rerank: true`. Es una prueba **aparte** de G3 y no un duplicado: el
juez entra por otro camino (`rerankIfEnabled` adentro de `toolRecall`) y se puede arreglar uno y
dejar el otro roto.

### G5 — El embedder tampoco corre bajo candado

Con un embedder falso que se cuelga, otra tool sigue respondiendo. Está en el mismo camino y con el
mismo defecto; arreglar sólo el LLM dejaría un segundo cuello con 30 s de techo.

### G6 — El candado se suelta aunque el handler entre en pánico

Un handler `lockSelf` que revienta adentro de su sección crítica no deja el candado tomado: la
llamada siguiente responde. Sin esto, el refactor cambia un cuello de botella por un deadlock, que
es peor — el cuello se destraba solo, el deadlock no.

---

## H3 · La autorización no se mueve

Este spec cambia **concurrencia**, no permisos. Ni un principal gana ni pierde acceso a nada.

### G7 — Un `reader` sigue sin poder llamar al motor

`musubi_ask` y `musubi_recall` siguen negados a un principal `reader`, exactamente como hoy. Es el
control que impide que el arreglo de performance se convierta en una apertura de acceso: marcar
`recall` como `readOnly` para destrabar el candado habría hecho justo eso.

### G8 — El mapa de autorización completo queda igual

Tabla sobre las 55 tools: el conjunto llamable por un `reader` es **idéntico** antes y después (las
22 marcadas `readOnly`, ni una más ni una menos).

### G9 — La autorización NO se deriva de la clase de candado

Una tool en `lockSelf` no queda por eso llamable por un `reader`. Los dos ejes son independientes y
hay que probarlo, porque el atajo obvio al implementar es volver a atarlos.

---

## H4 · El recall no pierde nada por ganar concurrencia

### G10 — Recalls concurrentes suman TODOS los accesos

N recalls en paralelo sobre la misma observación dejan `access_count` en N. El UPDATE es atómico en
SQL; esta prueba lo fija como contrato en vez de dejarlo como suposición del que leyó la query.

### G11 — Con el juez apagado, `musubi_recall` es bit-idéntico

Sin `read_time_rerank`, el orden devuelto es el mismo de antes y **no se toca el motor** (contador
de llamadas del provider falso en 0). Es la configuración del central hoy: si este spec cambiara
algo ahí, lo cambiaría en producción sin que nadie lo haya pedido.

---

## Alcance declarado

- **Quién puede usar el motor no cambia.** Hoy alcanza con ser `writer` — 6 de los 8 principals del
  central lo son, gio incluido. Es un agujero real y es el paso 4 de F1, no éste.
- **No hay presupuesto ni medición de gasto.** `CognitionStats` cuenta caché, portero y escalaciones
  del router; tokens y costo no existen. Paso 3 de F1.
- **El juez sigue apagado en el central.** Este spec lo vuelve *encendible* sin romper nada. Si se
  enciende o no lo decide el banco de F2.
- **La contención que queda.** Con el motor afuera del candado, el cuello que sobra es el de las
  escrituras (saves en 4,9 s de media, p95 10,2 s, todas bajo lock exclusivo). No se toca acá: es un
  problema de la base, no del motor, y mezclarlos haría un spec que no se puede verificar.
