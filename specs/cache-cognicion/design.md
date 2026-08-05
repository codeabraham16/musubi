# Diseño — Caché de cognición (F3)

## El orden de las capas es la decisión principal

```
caller → cached → guarded (portero) → router/motor → LLM
```

El caché queda **por fuera** del portero. Las tres alternativas y por qué se eligió ésta:

| Dónde | Qué se cachea | Por qué no |
|---|---|---|
| Adentro del portero | prompt ya tapado | **Puede reponer el secreto equivocado** — ver abajo |
| Adentro con clave extendida | tapado + secuencia de marcadores | Correcto, pero obliga al portero a exponerle su mapeo al caché |
| **Afuera** ✅ | prompt y respuesta crudos | Guarda secretos en RAM. Es el costo que se acepta |

### El bug que descartó la opción elegante

`privacy.Session.mint` acuña marcadores con un contador que **reintenta ante colisión**, y la
colisión se chequea contra `s.seen`, que guarda el texto **crudo**:

```
Sesión A:  secretos (a, b)                    → marcadores 1, 2
Sesión B:  MISMO texto tapado, pero su texto
           crudo contiene "[[MSB:key:1]]"
           adentro del valor de a             → saltea el 1 → marcadores 2, 3

Respuesta cacheada de A: "... [[MSB:key:2]] ..."   (quiere decir b)
B la rehidrata con su mapeo: 2 → a                 (pone a)
```

Los dos secretos son de B, así que no es una fuga entre sesiones. Es una **respuesta incorrecta y
silenciosa**, que es peor de encontrar.

> La lección general: un caché que colapsa dos entradas distintas en una clave hereda TODAS las
> suposiciones de la función que las normaliza. Acá la normalización era el portero, y su
> determinismo tenía una excepción.

### El costo de estar afuera, sin maquillar

El caché guarda prompts y respuestas **crudos, en memoria**. Es contenido que el proceso ya tiene en
RAM porque salió de la base local, así que no agrega una clase de exposición nueva — agrega **tiempo
de retención**. Nada toca disco. Lo que sí se pierde es hit rate: dos prompts que difieren sólo en
el valor de un secreto ya no colapsan.

## Estructura

`map[string]*list.Element` + `container/list` = LRU clásico. El mapa da O(1) de búsqueda; la lista
da orden de uso y desalojo O(1) por la cola.

La clave **prefija cada parte con su largo** antes de hashear. Sin eso, `("ab","c")` y `("a","bc")`
colisionan — el bug clásico de las claves compuestas, y acá una violación directa de K0.

El reloj es un campo `now func() time.Time`, nil ⇒ `time.Now`. Un test de TTL que duerme de verdad
es lento y, peor, intermitente.

## Qué reemplaza

`internal/mcp/methods_cognition.go` tenía un `rerankCache` global: `map[string][]string` con mutex y
**evicción por vaciado total** al llegar a 512. Este caché lo supera en tres cosas: cubre todas las
llamadas al motor y no sólo el juez, desaloja de a una, y vence.

> Nota de alcance: esta fase **no borra** el `rerankCache`. Cachea un objeto distinto —el orden de
> ids que dictó el juez, no la respuesta cruda— y sacarlo obliga a re-parsear la respuesta en cada
> hit. Es una limpieza aparte; dejar los dos conviviendo sin decirlo sería deuda escondida.

## `Enabled` es `*bool`, no `bool`

Para distinguir "no lo escribieron" de "lo apagaron". Con un `bool` pelado, el cero de Go haría que
**omitir el bloque apagara el caché** — lo contrario del default buscado.

## Lo que NO tiene, y por qué

- **Matcheo semántico.** Ver K5 en la spec: no está construido, y el nombre de la carpeta se cambió
  de `cache-semantico-cognicion` a `cache-cognicion` para que no lo sugiera.
- **Persistencia.** El caché muere con el proceso. Un caché en disco necesita invalidación,
  versionado del formato y una decisión sobre guardar secretos en disco — que es un no.
- **Métrica expuesta.** `Stats()` cuenta hits y misses, pero nada los publica todavía: eso es F5,
  que es la fase de telemetría. Se cuentan desde ahora para que F5 no tenga que tocar este archivo.
