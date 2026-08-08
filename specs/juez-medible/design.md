# Diseño — El juez se puede medir

Cinco decisiones, cada una con su alternativa descartada.

---

## D1 · El juez vive en `internal/cognition`, no en un paquete nuevo

Ya es el paquete del pilar: define `Provider`, el null-object y el portero. El juez es un consumidor
del motor, no una capa aparte.

**Alternativa descartada:** un `internal/rerank`. Un paquete de un archivo que importa `cognition`
para usar su única interfaz no separa nada — sólo agrega un salto más para leer el mismo código.

## D2 · `Candidato` es un tipo propio y no `memory.RecallItem`

```go
type Candidato struct{ ID, Gist string }
```

El juez no necesita saber de importancia, decaimiento ni procedencia. Atarlo al tipo del libro mayor
lo devolvería a depender de una capa que no le corresponde, y el banco tendría que fabricar
`RecallItem` completos para llamarlo.

## D3 · El juez NO cachea, NO pone timeout y NO recorta el top-K

Las tres son políticas del **llamador**, y las tres tienen dueños distintos:

| política | producción | banco |
|---|---|---|
| caché | sí — estira la suscripción y protege el rate-limit | **no**, falsearía la medición |
| timeout | `askTimeout` como backstop | el del contexto de la corrida |
| top-K | de la config (`read_time_rerank_top_k`) | de la `Config` del banco |

Meter cualquiera de las tres adentro del juez obligaría al banco a medir con las reglas de
producción — que es justamente lo que impide medir.

**El caché es el caso filoso**, porque es tentador: está pegado al juez en el código de producción y
parece parte de él. No lo es. Es lo que hace que dos recalls iguales cuesten una sola llamada, y en
un banco eso convierte «cuánto mejora el juez» en «cuánto repite el fixture».

## D4 · `DefaultTopK` es una constante compartida, no un 12 en cada lado

`internal/mcp.defaultRerankTopK` pasa a ser un alias de `cognition.DefaultTopK`.

**Alternativa descartada:** dejar el 12 en los dos lados y agregar una prueba que compare los
valores. No se puede: `defaultRerankTopK` es un símbolo no exportado y el banco no lo ve. Una
constante compartida no necesita prueba que la vigile — no hay dos cosas que puedan divergir.

## D5 · El banco NO se traga el error del juez; producción sí

Es la única asimetría deliberada del diseño:

- **Producción** degrada en silencio al orden model-free. El usuario pidió memoria, no un juez; que
  el LLM esté caído no puede romperle el recall.
- **El banco** aborta la corrida con error. Un juez roto que devuelve el orden model-free daría
  «el juez no aporta nada» — una conclusión **falsa con cara de medición**, que es peor que un test
  en rojo.

## Rollback

`Config.Juez` nil ⇒ el brazo no existe y el banco es bit-idéntico (J5 lo fija). En producción,
`rerankIfEnabled` conserva su firma y su comportamiento; la extracción es interna. No hay cambio de
esquema: `toolslist.golden.json` queda intacto.
