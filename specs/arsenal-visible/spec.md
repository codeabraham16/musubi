# Spec — El arsenal se ve

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## La tool

`musubi_list_skills` — lista las skills guardadas en el arsenal (`.musubi/skills/*.yaml`).

| Argumento | Tipo | Obligatorio | Significado |
|---|---|---|---|
| `query` | string | no | filtra por subcadena en nombre o descripción, sin distinguir mayúsculas |
| `limit` | number | no | corta la cantidad de resultados; ≤ 0 significa sin tope |

Devuelve un **array JSON** de objetos.

---

## Invariantes

### A1 — Las claves del JSON son las que el cuerpo parsea *(el invariante fundamental)*

La respuesta usa exactamente `name`, `description`, `triggers`, `capabilities`, `source`,
`source_url`, `rules`, en minúscula.

> Es el invariante fundamental porque su violación **no da error**: el cuerpo desserializaría un
> array de objetos con todos los campos vacíos y el panel mostraría filas en blanco. Un bug que
> parece funcionar es peor que el que ya teníamos.

### A2 — Sin skills, devuelve `[]` y no `null`

Directorio ausente o vacío ⇒ array vacío. Nunca `null`, nunca un error.

> `LoadSkills` devuelve un slice nil cuando no acumula nada, y `json.Marshal(nil slice)` produce
> `null`. El cliente del cuerpo trata `"null"` como «sin skills», así que hoy no rompería — pero
> el contrato de una tool que lista es devolver una lista, y depender de esa cortesía del cliente
> es dejar un filo para el próximo consumidor.

### A3 — `query` filtra por nombre y por descripción, sin distinguir mayúsculas

Una skill entra si el texto aparece en cualquiera de los dos campos.

### A4 — `limit` recorta; ausente o ≤ 0 no recorta

### A5 — La tool es read-only y no lee ninguna tabla con `project_id`

Lee el filesystem, igual que `musubi_detect_stack`. Va en `noScopedRead` del guard de completitud
del Track 19, y ese guard debe seguir en verde **sin tocarse**.

> El arsenal del central es **deliberadamente compartido** — es el arsenal de empresa. No es dato
> de un tenant, así que no hay nada que scopear. Lo que sí es por-proyecto son las *decisiones*
> sobre skills (`skill_decisions`), y esta tool no las mira: por eso no filtra lo rechazado (ver
> la propuesta).

### A6 — Una skill con YAML roto no tumba la lista

Archivo ilegible, YAML inválido o `name` vacío ⇒ se saltea y el resto se devuelve.

> Ya es el comportamiento de `LoadSkills`; el invariante lo sella para que exponerlo por MCP no lo
> cambie. Un arsenal que se cae entero por un archivo mal escrito no sirve.

### A7 — El cuerpo reporta la falla en vez de mostrar una lista vacía

Si el canal falla, el snapshot lleva el error en `snap.Errors`; el panel no puede quedar en blanco
en silencio.

---

## Criterios de aceptación

1. Los 7 invariantes con test propio, cada uno verificado **fallando** al sabotear.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde, en los dos
   repos (cerebro y cuerpo).
3. `TestEveryReadOnlyToolClassified` sigue verde **sin relajarse**: la tool nueva se clasifica, no
   se exime.
4. Prueba de extremo a extremo contra el binario: el cuerpo lista el arsenal real del central.
