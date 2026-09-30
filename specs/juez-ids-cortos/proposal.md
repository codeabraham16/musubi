# Propuesta — El juez ve ids cortos

Track: **Potencia medida**. Sigue a `specs/juez-medible/`, que dejó UNA sola definición del juez en
`internal/cognition/rerank.go` (prompt, llamada y parseo), usada igual por producción y por el banco.

## La pregunta

> ¿Cuánto del costo del juez se va en copiar ids, y cuánto se recupera sin perder exactitud?

Hoy el juez recibe cada candidato como `[<uuid de 36 caracteres>] gist` y tiene que devolver el
array con TODOS esos UUIDs, en orden. Un UUID son ~23 tokens (25 con las comillas y la coma), y los
hex alternados tokenizan a 1,6 caracteres por token (el texto común anda por 4). Con el top-K por
defecto (12), la salida sola son ~280 tokens de copiar hex, y la salida es la parte cara: se genera
token a token.

## Qué se construye

1. **`PromptJuez` rotula a los candidatos `id-1..id-N`** en vez de ponerles el id real. El juez
   nunca ve un id real. (La primera versión rotulaba `c1..cN`; la pasada 1 de la medición mostró
   por qué no alcanzaba: ver D1 y `medicion.md`.)
2. **El parseo traduce de vuelta**: `ParsearOrdenDeIDs` recibe los candidatos y devuelve ids REALES.
   Un id corto desconocido o repetido se ignora. Lo que el juez omite sigue quedando al final en su
   orden model-free, porque eso ya lo hace `ReordenarIDs` y no se toca.
3. **El contrato hacia afuera no cambia**: `Rerank` sigue devolviendo ids reales y `ReordenarIDs`
   sigue igual. Producción (`internal/mcp`) y el banco (`internal/recalleval`) no se enteran.
4. **Un instrumento para medirlo**: `internal/recalleval/juez_ids_medicion_test.go`, detrás de env
   vars y fuera de CI. Vuelca los prompts del banco (sin modelo) y corre el banco con un juez de
   Ollama LOCAL.

## Lo que NO se construye

- **No se enciende el juez en el central** ni se cambia su config. Es decisión del dueño.
- **No se mide contra el motor de producción** (sonnet vía LiteLLM/claude-agent-sdk). Está
  prohibido sacar texto de la máquina, así que la latencia de producción no se mide acá. Y esa
  latencia tiene un piso de ~4,3 s de spawn del harness (memoria c8ee49e2) que los ids no tocan.
- **No se amplía el fixture**: 12 consultas alcanzan para verificar, no para que una diferencia
  de MRR sea significativa (lo dice `specs/juez-medible`).
- **No se cambia el system prompt.** Su ejemplo (`["id-a","id-b","id-c"]`) tiene la forma de los
  rótulos pero con LETRAS, así que no nombra a ningún candidato. Si un modelo lo copia, la
  respuesta da error y se ve. Con un ejemplo de rótulos válidos, esa falla pasaría callada.

## Rollback

Un solo commit: `rerank.go`, sus pruebas, los dobles migrados de `internal/mcp`, el instrumento
de medición y estos specs. Revertirlo vuelve a los ids reales. No hay estado persistido: el caché
de producción guarda ids REALES (la clave es consulta+ids reales), así que un rollback no deja
entradas envenenadas.
