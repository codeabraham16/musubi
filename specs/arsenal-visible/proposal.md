# Propuesta — El arsenal se ve (F5.1 del track «Potencia medida»)

## El bug, medido

El cuerpo de Musubi tiene un panel **Arsenal** que muestra las skills guardadas en el cerebro
central. **Siempre está vacío.** No por falta de skills: porque la pregunta nunca llega.

`internal/brain/skills.go` llama a la tool **`musubi_list_skills`**, que **no existe en el cerebro**
y nunca existió. El cliente se escribió contra una capacidad planeada y no construida.

Y el error se traga (`cmd/musubi-body/bridge.go:211`):

```go
if sk, err := cc.ListSkills("", 40); err == nil {   // err != nil ⇒ el bloque entero se saltea
    for _, s := range sk { snap.Arsenal = append(snap.Arsenal, ...) }
}
```

`snap.Arsenal` queda vacío y **nadie se entera**. El panel no dice «no pude preguntar»: dice, por
omisión, «no hay nada». Es la peor clase de falla — la que se lee como un dato.

## Por qué esta es la fase correcta para abrir F5

La regla del track es **«todo lo que se escribe, se usa»**, y nació de medir el fallo inverso de
Musubi: construye capacidad más rápido de lo que la consume (1.256 líneas de motor DAG con cero
workflows).

Esto es **el inverso exacto de ese antipatrón**: hay un consumidor ya escrito, con su DTO, su
parseo y su panel, esperando una capacidad que nunca se construyó. No hay que inventar el
consumidor ni justificarlo — está en el repo, compilando, fallando en silencio.

## Qué cuesta

Casi nada, y esa es parte del argumento. La capacidad **ya existe** del lado del cerebro:
`skills.Resolver.LoadSkills()` (`internal/skills/resolver.go:18`) lee y parsea
`.musubi/skills/*.yaml`, saltea los inválidos y devuelve slice vacío si no hay directorio. El
servidor MCP ya tiene un `Resolver` construido (`internal/mcp/server.go:254`).

Falta **exponerla**: una tool que la envuelva, filtre y recorte.

## La trampa que casi se cuela

`skills.Skill` tiene **sólo tags YAML**. Serializarla directo produce `{"Name":…,"Description":…}`
en mayúscula, y el cuerpo —que espera `{"name":…,"description":…}`— parsearía **sin error** un
array de objetos con todos los campos vacíos.

El panel pasaría de «vacío» a «N filas en blanco»: un bug peor, porque parece que funciona. Por eso
el diseño usa un DTO con tags JSON explícitos y hay un invariante que sella las claves.

## Alcance

1. **Cerebro**: la tool `musubi_list_skills`.
2. **Cuerpo**: que el error deje de tragarse, para que el próximo hueco se vea en vez de disfrazarse
   de dato vacío.

## Qué NO es

- **No toca cómo se guardan las skills.** `musubi_save_skill` y `musubi_author_skill` quedan igual.
- **No filtra por decisiones del usuario.** `musubi_search_skills` excluye lo rechazado porque
  puntúa un catálogo remoto para RECOMENDAR. Esta lista responde otra pregunta —«¿qué hay guardado
  en el arsenal?»— y ahí una skill rechazada para un proyecto sigue estando en el arsenal. Mezclar
  las dos preguntas es lo que haría a esta tool leer una tabla scopeada sin necesidad.
- **No instala ni ejecuta nada.** Devuelve lo que hay en disco.

## Consumidor y métrica (exigidos por la regla del track)

- **Consumidor:** el panel Arsenal del cuerpo, ya escrito, hoy roto.
- **Métrica de cierre:** el panel muestra las skills reales del central, y una falla del canal se
  reporta como falla en vez de como lista vacía.
