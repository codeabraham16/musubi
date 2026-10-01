# Spec — El juez ve ids cortos

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**, y el sabotaje que la
pone en rojo está declarado junto a la prueba (directivas `// arnes:`).

Los invariantes van con **I** (de «ids») y no con K: en `internal/cognition` la K ya es la del
caché (`cache_test.go`, K0–K8), y un «K4» en un comentario de `rerank.go` mandaba a dos lugares.

---

## H1 · El juez nunca ve un id real

### I1 — El prompt rotula `id-1..id-N`, en el orden de los candidatos

`PromptJuez(query, cands)` DEBE poner a cada candidato en su línea como `[id-<i+1>] <gist>`, en el
mismo orden en que llegan. Ningún id real DEBE aparecer en el `system` ni en el `user`.

- **Given** candidatos con ids reales (UUIDs)
- **When** se arma el prompt
- **Then** el `user` tiene `[id-1] …`, `[id-2] …`, … y ningún UUID.

## H2 · La respuesta se traduce de vuelta, sin inventar nada

### I2 — La traducción es exacta

Si el motor contesta `["id-3","id-1","id-2"]` sobre tres candidatos, `Rerank` DEBE devolver los
ids REALES del tercero, el primero y el segundo, en ese orden.

Lo mismo DEBE valer con el top-K de producción (12 candidatos), donde los rótulos llegan a dos
dígitos: el prompt termina en `[id-10]`, `[id-11]`, `[id-12]`, y `["id-12","id-10","id-1","id-11"]`
se traduce a los candidatos 12, 10, 1 y 11.

### I3 — Un id corto desconocido o repetido se ignora

Un rótulo fuera de rango (`id-9` con 3 candidatos), uno que no es rótulo (`x`) o uno repetido
(`id-2` dos veces) NO DEBEN aparecer en el orden devuelto. Lo que el juez omitió DEBE quedar al final,
en su orden model-free: eso lo hace `ReordenarIDs`, cuyo contrato (J7) no cambia.

- **Given** tres candidatos a, b, c
- **When** el motor contesta `["id-9","id-2","x","id-2","id-1"]`
- **Then** `Rerank` devuelve [b, a] y `ReordenarIDs([a,b,c], …)` da [b, a, c].

### I4 — Un array sin ningún rótulo conocido es un error, como J8

Si ninguno de los elementos es un rótulo de esta llamada, `Rerank` DEBE devolver error y no un orden
vacío. Incluye los dos casos que un modelo o un doble de prueba producirían sin mala intención:
contestar con los **ids reales** (el protocolo viejo) y **copiar el ejemplo** del system prompt
(`["id-a","id-b","id-c"]`). Si no dieran error, el llamador reordenaría contra nada y el fallo se
vería como «el juez no cambió nada». También es error un array que sólo trae otra grafía del rótulo
(`ID-1`, `id1`, `id-01`, `[id-3]`, un número pelado) o el rótulo de la primera versión (`c1..cN`):
D5.

## H3 · Hacia afuera no cambia nada

### I5 — `Rerank` y `ReordenarIDs` conservan firma y contrato

`Rerank` devuelve ids reales, en el orden del juez. J1 (el prompt de producción es el del paquete,
byte a byte), J2, J3, J4–J6 (banco) y J7–J9 DEBEN seguir verdes. Sólo se migran los dobles de
prueba que contestaban con ids reales.

### I6 — Las tolerancias del parseo se conservan

La prosa alrededor del array, un objeto que lo envuelve y un bloque ```` ```json ```` DEBEN seguir
parseando, ahora con rótulos.

---

## Fuera de alcance

- **El system prompt no cambia.** Su ejemplo no coincide con ningún rótulo, a propósito: I4.
- **La normalización de rótulos** (mayúsculas, espacios, corchetes pegados) se decide con la
  evidencia del modelo local (`medicion.md`), no por adelantado. Aceptar formas de más es aceptar
  basura sin error.
- **Encender el juez**, **medir el motor de producción** y **ampliar el fixture**: ver la propuesta.
