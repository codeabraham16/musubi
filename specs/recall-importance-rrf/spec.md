# Spec — recall-importance-rrf

Vocabulario RFC 2119. Alcance: `scoreCandidates` + un helper `importanceRank` en `internal/memory/recall.go`. 100% model-free, determinista.

## Q3 — Importancia como término RRF

- **R1** — `scoreCandidates` DEBE sumar la importancia como un término RRF más (`1/(rrfK+impRank)`), NO como multiplicador del `rrf` acumulado. El `score` final DEBE ser la suma de términos RRF (sin factor multiplicativo por importancia).
- **R2** — El rango de importancia DEBE ser **tie-aware (denso)**: candidatos con igual importancia efectiva comparten el mismo rango; el rango incrementa en 1 solo al bajar de valor de importancia. El orden es por importancia efectiva **descendente** (mayor importancia ⇒ mejor rango = 0).
- **R3** — La importancia efectiva DEBE normalizar los valores `<= 0` a `1.0` (mismo default histórico), de modo que una importancia ausente/no seteada empate con la default.
- **R4** — Cuando TODOS los candidatos tienen la misma importancia efectiva, el término de importancia DEBE ser una constante idéntica para todos ⇒ NO DEBE alterar el orden relativo determinado por los demás términos RRF.
- **R5** — Entre dos candidatos con rango idéntico en todos los demás pools (recencia, frecuencia, léxico, vector, grafo, co-ocurrencia), el de **mayor** importancia efectiva DEBE rankear primero (la importancia preserva su rol de desempate).
- **R6** — Un candidato con la importancia máxima pero un rango claramente **peor** en los pools de relevancia NO DEBE, solo por importancia, superar a un candidato con rango claramente mejor en esos pools (fin del override; la relevancia domina, la importancia desempata).
- **R7** — La firma de `scoreCandidates` NO DEBE cambiar: la importancia se lee de `candidate.importance` (como recencia/frecuencia). El resto de los términos RRF DEBEN quedar bit-idénticos.

**Escenario Q3.a (rango denso)** — *Given* importancias `[10, 5, 1, 1]`, *When* se computa `importanceRank`, *Then* los rangos son `[0, 1, 2, 2]` (los dos de importancia 1 comparten rango 2).

**Escenario Q3.b (desempate preservado, R5)** — *Given* dos candidatos con idéntico rango en todos los pools y importancias 5 vs 1, *When* se scorea, *Then* el de importancia 5 rankea primero.

**Escenario Q3.c (no-override, R6)** — *Given* candidato `A` con el MEJOR rango léxico/vectorial e importancia 1, y candidato `C` con el PEOR rango en todos los pools e importancia 10, *When* se scorea, *Then* `A` rankea antes que `C`.

**Escenario Q3.d (uniforme sin reordenamiento, R4)** — *Given* varios candidatos todos con importancia 1.0 y rangos distintos en los pools, *When* se scorea, *Then* el orden es el mismo que produciría la fusión de esos pools sin ninguna influencia de importancia (el término de importancia es constante).

## Rangos densos — requisito emergente (descubierto en implement)

La implementación reveló que R5 y R6 son **irreconciliables** mientras los rangos rompan empates POSICIONALMENTE (`rankBy` asigna 0,1,2… aun a valores iguales; `lexRank`/`coocRank` asignaban la posición del resultado FTS, por rowid). Con ese ruido, dos observaciones de relevancia IDÉNTICA (mismo contenido) quedan "a un rango de distancia" — **indistinguible de una brecha de relevancia genuina de un rango**. Entonces cualquier peso de importancia que gane el empate de R5 también overridea la brecha chica de R6, y viceversa. No hay constante mágica que separe los dos casos: hay que eliminar el ruido en el origen.

- **R8** — Todos los rangos de pool DEBEN ser **densos**: candidatos con el mismo valor de señal comparten rango; el rango sólo incrementa al pasar a un valor estrictamente peor.
  - Recencia y frecuencia: vía `denseRankBy` (empates de `effectiveRecency` / `accessCount`).
  - Léxico (`lexRank`) y co-ocurrencia (`coocRank`): densos por **score bm25** de FTS (empate de score ⇒ mismo rango), no por posición/rowid.
- **R9** — Con R8, una relevancia idéntica produce un rango idéntico ⇒ el empate es REAL ⇒ la importancia lo desempata (R5); una brecha de relevancia de ≥1 rango es una señal genuina ⇒ la relevancia manda (R6). Los dos casos quedan separables.

**Escenario Q3.e** — *Given* dos observaciones con contenido idéntico (mismo score bm25) e importancias 1 y 5, *When* se hace `Recall`, *Then* comparten `lexRank` (empate real) y la de importancia 5 rankea primero.

## No-objetivos (verificables)

- NO hay peso/coeficiente configurable para la importancia (entra como término RRF plano) — eso es M7.
- NO se toca la asignación ni la persistencia de `importance` (captura/promote intactos).
- NINGÚN cambio auto-suprime, auto-archiva ni cambia qué se persiste: Q3 solo reordena el ranking del recall.
