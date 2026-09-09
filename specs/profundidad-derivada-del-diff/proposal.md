# Propuesta — La profundidad de la revisión sale del cambio, no del criterio de nadie

Track: **Encender el arnés**, F2. Equivale a F6.2 del Track 21.

## El problema, medido

La skill `adversarial-review` sugiere hoy el mismo panel para todo: `rounds=2`, `quorum=2 de 3`. Un
typo en un comentario y una refactorización con cuarenta llamadores reciben el mismo tribunal.

Un criterio que no distingue no es un criterio: es un número fijo con aire de decisión. Y el costo
cae siempre del mismo lado — **revisar de más enseña a saltearse la revisión**, que es la forma en
que un arnés se apaga sin que nadie decida apagarlo.

## Lo que ya estaba, y por qué no alcanzaba

Musubi calculaba **todas** las señales necesarias y ninguna llegaba a juntarse:

| señal | quién ya la tenía |
|---|---|
| archivos, rangos, tipo de cambio | `codeintel.ParseUnifiedDiff` |
| símbolos tocados por los hunks | `codeintel.SymbolsInRanges` |
| radio de impacto (callers transitivos) | `DbEngine.GraphImpactCtx` |
| cobertura del índice | `DbEngine.GraphFileFingerprintsCtx` |
| qué es indexable | `codeintel.IndexableForGraph` |

**No había una sola ruta de código donde un `FileDiff` terminara en una llamada al grafo.** Esta
fase es esa ruta, más la aritmética que la convierte en una decisión.

## La forma de la solución

Siete señales enteras → puntos por **escalones** (0, 1 ó 2) → un total de 0 a 13 → tres niveles.

Escalones y no una curva continua a propósito: un número que no se puede recomputar de cabeza no se
discute, se obedece o se ignora. Con escalones, quien lee el veredicto puede rehacer la cuenta.

## Lo que esta fase NO hace

- **No juzga.** Dice cuánta revisión pedir, no si el cambio está bien. El juicio se delega (regla 3).
- **No bloquea.** Es una sugerencia dimensionada, no una barrera.
- **No adivina lo que no sabe.** Ver el piso de honestidad en `spec.md`: es la parte que más
  fácilmente se habría implementado mal, y la que más importa.

Relacionado: F1 (`cmd/musubi/reviewgate.go`) puede llamar a la función pura sin base ni MCP.
