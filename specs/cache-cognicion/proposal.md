# Propuesta — Caché de cognición (F3)

## El problema, en una frase

Cada llamada al motor cuesta **latencia, cuota y —en la flota gratis— rate-limit compartido**, y una
parte grande de esas llamadas pregunta lo mismo que ya se preguntó hace un minuto.

## Qué ya existe, y por qué no alcanza

`internal/mcp/methods_cognition.go` tiene un `rerankCache`: un `map[string][]string` global con
mutex y **evicción tonta** (se vacía entero al llegar a 512). Sirve, y demuestra que hasta el
matcheo exacto pega — pero tiene tres límites que no se arreglan agrandándolo:

1. **Sólo cubre el juez del recall.** `musubi_ask`, que es la llamada cara, no cachea nada.
2. **Vaciarse entero** tira 512 entradas buenas para hacer lugar a una.
3. **No vence nunca.** Una respuesta de hace seis horas se sirve igual.

## Dónde va, y por qué NO donde parecía mejor

La idea elegante era ponerlo **adentro** del portero de privacidad, cacheando el prompt ya tapado:
el caché no guardaría un secreto jamás, y dos prompts que sólo difieren en el valor del secreto
colapsarían al mismo texto, subiendo el hit rate.

**Se descartó porque el supuesto no se sostiene.** En `privacy.Session.mint`, el marcador se acuña
con un contador que **reintenta ante colisión**, y la colisión se chequea contra el texto **crudo**:

- Sesión A tapa dos secretos → marcadores `1` y `2`.
- Sesión B produce el **mismo texto tapado**, pero su texto crudo contiene algo con forma de
  marcador dentro del propio secreto → saltea el índice 1 y acuña `2` y `3`.
- La respuesta cacheada de A dice `2` queriendo decir *el segundo secreto*. B la rehidrata con su
  mapeo y pone **el primero**.

No es una fuga entre sesiones —los dos secretos son de B— pero es una respuesta incorrecta y
silenciosa. Es un caso rebuscado; también es exactamente la clase de borde que hace que un caché
"ande casi siempre".

Se podría cerrar metiendo la secuencia de marcadores en la clave, pero eso obliga al portero a
exponerle su mapeo al caché y acopla dos piezas que hoy son independientes y auditables por
separado. **Va afuera**: `caller → caché → portero → motor`.

El costo de esa decisión, dicho de frente: el caché guarda prompts y respuestas **crudos, en
memoria**. Es contenido que el proceso ya tiene en RAM porque salió de la base local, así que no
agrega una clase de exposición nueva — agrega tiempo de retención, y nada toca disco. Lo que sí
cuesta es hit rate: dos prompts que difieren sólo en el valor de un secreto ya no colapsan.

## Qué NO es

- **No es un caché de respuestas "parecidas" por defecto.** Arranca **exacto y model-free**. La
  similitud es opt-in y necesita un embedder; sin él, el caché sigue funcionando.
- **No decide si una respuesta sigue siendo cierta.** Vence por tiempo y por cambio de prompt, no
  por juicio semántico.
- **No cachea errores.** Un fallo transitorio cacheado se vuelve permanente.

## Costo y reversibilidad

Un decorador más en `NewProvider`, un LRU acotado y dos campos de config. Sin dependencias nuevas,
sin red, sin estado en disco. Con `cognition.cache.enabled: false` el decorador no se instala y el
comportamiento es el de antes, byte a byte.
