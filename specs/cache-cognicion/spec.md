# Spec — Caché de cognición (F3)

Contrato observable. Cada invariante está numerado y tiene una prueba que **sabe fallar**: si se
rompe la propiedad, el test se pone rojo. Un invariante sin test que lo pueda tumbar es decoración.

---

## Alcance

Toda llamada a `cognition.Provider.Ask`, es decir los mismos dos seams que cubre el portero de F1:
`musubi_ask` y el juez de pertinencia del recall.

El caché se instala **por fuera** del portero (`caller → caché → portero → motor`). El porqué —y el
diseño más elegante que se descartó— está en `proposal.md`.

Fuera de alcance: los embeddings. Un embedder devuelve un vector determinista para un texto dado;
cachearlo es otro problema, con otra clave y otra política de vencimiento.

---

## Invariantes

### K0 — Un hit devuelve lo que habría devuelto el motor *(el invariante fundamental)*

Para todo par `(system, user)`, si el caché responde, responde **exactamente** lo que el motor
devolvió para **ese mismo par**. Nunca la respuesta de otro prompt.

> Es la razón de existir de todo esto. Si sólo se pudiera verificar uno, es este. Un caché que
> devuelve la respuesta de otra pregunta no es un caché lento: es memoria corrupta.

### K1 — Un miss llama al motor, siempre

Sin entrada válida, la llamada pasa al motor tal cual. El caché no inventa, no aproxima y no
devuelve vacío para ahorrarse la llamada.

### K2 — Los errores NO se cachean

Si el motor devuelve error —timeout, rate-limit, circuito abierto, `ErrSecretsBlocked`— eso **no**
entra al caché.

> Cachear un fallo transitorio lo vuelve permanente: un rate-limit de 30 segundos quedaría servido
> durante todo el TTL. El caché guarda respuestas, no resultados.

### K3 — Bit-identidad con el caché apagado

Con `cognition.cache.enabled: false` (o el pilar apagado), el decorador **no se instala** y el
comportamiento es idéntico al de antes de esta fase. No se paga ni un ciclo.

### K4 — Cota dura de memoria

El caché **nunca crece sin techo**. Al llegar a `max_entries` desaloja la entrada usada hace más
tiempo (LRU), **una**, no todas.

> El `rerankCache` que esto reemplaza se vaciaba entero al llenarse: tiraba 511 entradas buenas para
> hacer lugar a una. Desalojar de a una es la diferencia entre un caché y un balde con agujero.

### K5 — El caché es model-free

El matcheo es **exacto** y no depende de ningún modelo, ni de un embedder, ni de la red.

**Esta fase NO entrega matcheo semántico, y hay que decirlo en vez de dejar el nombre sugiriéndolo.**
El roadmap la llamaba "caché semántico"; lo que se construyó es un caché exacto. Las razones:

1. La similitud necesita un **embedder**, que vive en `internal/embedding` y no en este paquete.
   Cablearlo acá acopla los dos pilares por una optimización.
2. Embeber cada prompt para buscar un vecino **cuesta una llamada de red** por consulta. Contra un
   embedder remoto eso se come buena parte de lo que el caché venía a ahorrar; sólo cierra con un
   embedder local.
3. Un hit por similitud devuelve la respuesta de **otra pregunta**, parecida pero distinta. Eso es
   una tensión directa con K0, y merece su propia fase con su propio umbral medido, no venir de
   arrastre en ésta.

El seam natural para agregarlo después es una función de similitud inyectable en `newCached`. No se
construye ahora: un seam sin implementación es código muerto que además compromete un diseño.

### K6 — Una entrada vencida no se sirve

Pasado `ttl`, la entrada se trata como inexistente. El reloj es **inyectable**, para que el test del
vencimiento no dependa de dormir.

### K7 — Seguro bajo concurrencia

El daemon MCP atiende en paralelo. Dos llamadas simultáneas con la misma clave no corrompen el
caché ni el LRU.

El test de concurrencia comprueba lo que se puede comprobar sin el detector: que la cota se respete
bajo presión y que hits+misses cuadren con las llamadas hechas. **La ausencia de carreras la
verifica la CI con `-race`**, no esta máquina: `-race` exige cgo y acá no hay compilador de C, así
que correrlo local no es una opción. Decirlo importa — un invariante que se declara verificado
donde no se verificó es peor que uno sin test.

### K8 — La clave distingue system de user

`Ask(a, b)` y `Ask(b, a)` son llamadas distintas y no pueden colisionar. Tampoco puede colisionar
un par cuya concatenación coincida: `("ab","c")` ≠ `("a","bc")`.

> Es el bug clásico de las claves compuestas por concatenación, y es una fuga de K0.

---

## Configuración

```yaml
cognition:
  cache:
    enabled: true       # default true cuando hay motor real
    max_entries: 512    # cota dura; LRU
    ttl_seconds: 3600   # 0 = sin vencimiento
```

Un `max_entries` <= 0 con el caché encendido es **error de config**, no un default silencioso: un
caché sin cota es una fuga de memoria con nombre amable.

---

## Criterios de aceptación

1. Los 9 invariantes con test propio, y cada test verificado **fallando** al sabotear la
   implementación.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde, más
   `go test -race` sobre el paquete.
3. Cero cambios de comportamiento con el caché apagado (K3 cubierto por test).
4. Test adversarial: claves que se parecen (K8), entrada vencida justo en el borde, desalojo bajo
   presión, error del motor seguido de éxito para el mismo prompt, y llamadas concurrentes.
