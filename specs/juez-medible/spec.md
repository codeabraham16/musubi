# Spec — El juez se puede medir

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## H1 · Hay UN juez, no dos

El juez sale a `internal/cognition` como unidad reusable:

```go
// Candidato es lo mínimo que el juez necesita ver de una memoria.
type Candidato struct{ ID, Gist string }

// Rerank pide al motor que ordene los candidatos por pertinencia y devuelve los ids en el orden
// que dictó. NO cachea, NO fija timeout y NO decide el top-K: todo eso es del llamador.
func Rerank(ctx context.Context, p Provider, query string, cands []Candidato) ([]string, error)
```

`rerankIfEnabled` queda como adaptador: arma los candidatos, consulta su caché, llama a `Rerank`,
reordena.

### J1 — El prompt es el mismo, byte a byte

Lo que el motor recibe desde `rerankIfEnabled` es **idéntico** a lo que recibe desde
`cognition.Rerank` con los mismos candidatos. Es LA prueba del spec: sin ella, el banco puede medir
una imitación y devolver un número con aspecto de autoridad sobre algo que no corre en producción.

### J2 — El juez no cachea

Dos llamadas a `cognition.Rerank` con la misma entrada golpean el motor **dos veces**. El caché es
una decisión del llamador; adentro del juez falsearía toda medición.

### J3 — Y sin embargo el caché de producción sigue vivo

Dos recalls idénticos con el juez encendido llaman al motor **una** vez. Control de J2: mover el
caché afuera no puede costar la protección del rate-limit que el caché vino a dar.

---

## H2 · El banco mide el juez de verdad

`Config` gana el brazo de rerank y `rankedIDs` lo aplica sobre el head con el mismo top-K de
producción.

### J4 — Con el brazo, el orden evaluado cambia

Un juez falso que invierte el head empeora MRR y nDCG de forma medible. Si las métricas no se
mueven, el brazo está desconectado y el banco mentiría diciendo «el juez no aporta».

### J5 — Sin el brazo, nada se mueve

Una `Config` sin juez da exactamente los mismos números que hoy: el baseline model-free commiteado
en `testdata/baseline_modelfree.json` no cambia. Es el control de J4.

### J6 — El banco no memoiza entre queries

N queries del fixture ⇒ N llamadas al motor. Un banco que reusa respuestas mide el caché, no el juez.

---

## H3 · El juez nunca pierde una memoria

### J7 — Reordena, no descarta

Ids que el juez inventó se ignoran; memorias que no mencionó quedan **al final, en su orden
original**. Un juez que puede hacer desaparecer una memoria del recall es peor que no tener juez.

### J8 — Una respuesta imparseable es un error, no un orden vacío

Si el motor contesta prosa sin array, `Rerank` devuelve error. Devolver `nil` sin error haría que el
llamador reordenara contra una lista vacía y el fallo se vería como «el juez no cambió nada».

### J9 — Si el motor falla, el orden model-free sobrevive

`rerankIfEnabled` ante error del motor devuelve el recall intacto. Es la degradación que ya existía
y que la extracción no puede romper.

---

## Alcance declarado

- **El juez no se enciende.** Este spec construye el instrumento; encenderlo es decisión del dueño,
  con el número en la mano.
- **El fixture no se amplía.** 26 docs / 12 queries alcanza para verificar que el brazo funciona,
  **no** para que una diferencia de MRR signifique algo. Ampliarlo con memoria real es otro trabajo.
- **No se corre contra el motor real.** La config local no tiene bloque `cognition` (verificado), así
  que la corrida real necesita el central, o el túnel con `LITELLM_MASTER_KEY` — un secreto del dueño.
- **Presupuesto y autorización del motor** siguen siendo los pasos 3 y 4 de F1.
