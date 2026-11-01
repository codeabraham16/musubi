---
artifact: design
schema_version: "1.0"
change: readiness-ve-la-frescura-del-grafo
status: draft
---

# Diseño técnico — la frescura del grafo entra a `musubi_readiness` por la capa MCP

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| **D1** | **Un solo check compuesto** (`el índice está al día`), no uno por eje | Tres checks: `sin archivos stale`, `sin fantasmas`, `sin archivos sin indexar` | **Los tres ejes se curan con EL MISMO ACTO.** `aplicarIncremental` ([methods_codegraph.go:771](../../internal/mcp/methods_codegraph.go)) re-hashea los stale, indexa los que faltaban y **poda los fantasmas** con `PruneGraphFilesFrom` (línea 777). Tres checks sugerirían tres remedios donde hay uno: `musubi_codegraph_index`. Secundario: diluye menos los dos checks que ya existen (3 en vez de 5). No se pierde diagnóstico — el `Why` nombra los tres números |
| **D2** | **Un umbral de 5 %, el mismo para los tres ejes** | Un umbral por eje; umbrales binarios (cero tolerancia) | **Derivado, no elegido a ojo.** Sobre los últimos 300 commits de este repo: mediana 4 archivos tocados (0,29 % de los 1.395 del grafo), p90 15 (1,08 %), p95 22 (1,58 %), **máximo 65 (4,66 %)**. El 5 % cubre el commit más grande medido sin dar falso positivo, y es el mismo tope que `readiness` ya usa para errores y rechazos, así que el archivo queda consistente consigo mismo. Un umbral por eje sería más fino en teoría, pero no tengo dato para calibrar tres números distintos, y tres convenciones son tres cosas que discutir en vez de una |
| **D3** | El enriquecido vive en **`toolReadiness`**, no en `dimensionGrafo` | Pasarle al engine un callback o una interfaz de frescura | `dimensionGrafo` es método de `*DbEngine` en `internal/memory`; `graphFreshness` es método de `*McpServer` porque necesita disco, `projectPath` y scope. **El engine no puede llamarla: la desconexión es una frontera de capas, no un olvido.** Inyectar una interfaz haría que `memory` dependa de un concepto de `mcp`. `toolReadiness` ya es un paso-a-través y está del lado correcto |
| **D4** | Cuando la frescura **no aplica**, no se agrega el check y se declara en la evidencia. **`Observed` no se toca** | Marcar la dimensión `Observed: false`; agregar el check igual (pasaría gratis) | R4. La dimensión **sí** fue observada —hay nodos y aristas, y eso se midió—; lo no medido es un sub-aspecto. Poner `Observed: false` la mandaría a `Unobserved` y le pondría score 0 por regla, castigando a un central sano. Y agregar el check igual es el cero ambiguo: subiría el puntaje **por no haber mirado** |
| **D5** | **`graphFreshness` devuelve también sus dos denominadores.** Esto AJUSTA el R7 de la spec, que decía no tocarla | Recalcular los denominadores en `toolReadiness`; un helper nuevo al lado | Ver abajo, «La tensión con R7» |
| **D6** | **El recálculo del puntaje entra por un método nuevo, `(*ReadinessReport).Recalcular()` en `internal/memory`** · *decidido al implementar: el diseño daba por hecho que se podía recalcular desde `mcp`* | Copiar la fórmula en `toolReadiness`; exportar `puntaje` y `redondear` | `puntaje` y `redondear` son **minúsculas**, así que no son alcanzables desde `internal/mcp`. Copiar la fórmula es el peor de los tres: la dimensión enriquecida se calcularía con una versión que puede quedar vieja mientras las otras cuatro usan la nueva, **dentro del mismo promedio**, y nada lo marcaría. Exportar las dos primitivas expone aritmética que no significa nada sola. Un método sobre un tipo que ya es público deja la regla donde se define, y es idempotente sobre un reporte intacto —verificado POR LECTURA e, en `verify`, por MEDICIÓN con el engine real (x1 == x2 == antes): las cinco dimensiones cumplen `Score == puntaje(Checks)`, y las que retornan temprano lo hacen sin checks con `Score` en cero-value, que coincide con `puntaje(nil) == 0`. ⚠️ **No había un test que lo ejerciera**: el panel lo marcó y la vuelta 2 lo agrega, porque «verificado» leyendo no es lo mismo que custodiado |
| **D7** | **Una dimensión con `Observed: false` NO puede ganar el check: guarda propia y salida temprana** · *defecto encontrado al implementar, el diseño no lo vio* | Dejar que el check se agregue y confiar en que los contadores en cero lo pongan en rojo | Si el grafo nunca se indexó, la dimensión vale 0 por la regla de oro de `readiness` («una señal no observada puntúa cero»). Pero en un proyecto **sin código indexable** los tres contadores dan cero, el check **PASA**, y una dimensión sin dato terminaría puntuando **1.00** — exactamente el premio por no medir que ese archivo existe para impedir. Es distinto de D4: allá la dimensión SI fue observada y falta un sub-aspecto; acá no hay dato de ninguna clase. Se ahorra de paso el costo de hashear el árbol para un veredicto ya cerrado |

| **D8** | **Con los dos denominadores en cero no se agrega el check: guarda propia** · *defecto encontrado en `verify`, revisando el diff — ni la spec ni el design lo vieron* | Confiar en que los contadores en cero lo pongan en rojo (es justo lo que NO pasa); mirar sólo `enGrafo` | R8. Es la **segunda puerta** al premio de D7, y la guarda de D7 no la tapa: allá la dimensión **no** está observada; acá **sí** lo está —`GraphStatsCtx` cuenta todos los nodos, pero `GraphFileFingerprintsCtx` filtra `WHERE path != ''`— y lo que falta son los archivos. Con `enGrafo == 0 && enDisco == 0` las tres proporciones valen 0 **por construcción** (`proporcion` lo hace a propósito, para no publicar un NaN) y el check pasaba en verde sin leer un archivo. La condición es el 0/0 **exacto**, y en `verify` se midieron los cuatro: (>0,>0) mide los tres ejes; (0,>0) cae en rojo solo; (0,0) es este; y **(>0,0) NO cae en rojo solo —puede dar VERDE—**, porque `ghosts` lee del disco directo mientras `enDisco` sale de `walkSourceTree`, que saltea `testdata`, `vendor`, `node_modules`, `dist`, `coverage` y todo directorio con punto: un archivo del grafo ahí existe, se lee y coincide, y el chequeo pasa con score 0.67 y `archivos_en_disco: 0`. **La frase «los otros tres caen en rojo solos» era falsa** (ver el detalle en R8). No mueve la guarda: en ese cuadrante los dos ejes con denominador real se midieron y dieron 0, así que el verde no es el premio por no mirar |

## La tensión con R7, y por qué se resuelve ampliando la firma

R7 dice que `graphFreshness` no debe modificarse. Pero R3 pide checks **proporcionales**, y una
proporción necesita denominador — y los dos denominadores son **distintos**: `stale` y `ghosts`
salen de recorrer `stored` (los archivos que están en el grafo), mientras que `missing` sale de
recorrer `enDisco` (los archivos indexables del árbol). No es un detalle: usar `nodos` como
denominador daría porcentajes absurdos, porque son 16.787 símbolos contra 1.395 archivos.

Las tres salidas:

1. **Recalcular en `toolReadiness`** — llamar otra vez a `GraphFileFingerprintsCtx` y a
   `walkSourceTree()`. Duplica el costo medido (~0,28 s → ~0,56 s) para obtener dos números que la
   función acaba de calcular y tirar.
2. **Un helper nuevo al lado** — y entonces hay dos formas de preguntar lo mismo en el mismo
   archivo. El comentario de `graphFreshness` advierte exactamente contra eso: *«Dos formas de
   preguntar lo mismo en el mismo archivo es como una de las dos se queda vieja»*.
3. **Ampliar la firma** para devolver `len(stored)` y `len(enDisco)`, que ya se calculan adentro.

Se elige la 3. **R7 existía para proteger la SEMÁNTICA de `graphFreshness` —su fail-safe, su
política de no inventar podredumbre—, no su firma.** Ampliarla es aditivo: el cuerpo no cambia, el
fail-safe sigue devolviendo ceros donde corresponde, y `toolMap` se adapta ignorando los dos
valores nuevos. La spec se corrige en consecuencia; el requisito que importa —no tocar el
comportamiento— se cumple entero.

## Enfoque de implementación

```
toolReadiness(ctx)
  ├─ rep := engine.Readiness(scopedCtx)        ← sin cambios (volumen, desde la DB)
  ├─ enriquecerGrafo(&rep, scoped)             ← NUEVO, del lado MCP
  │    ├─ si arbolFueraDeAlcance():
  │    │     evidence["frescura_no_medida"] = "<razón>"   → y NO agrega check
  │    └─ si no:
  │          stale, ghosts, missing, archivosEnGrafo, archivosEnDisco := graphFreshness(scoped)
  │          evidence["stale"|"ghosts"|"missing"] = los tres crudos
  │          evidence["archivos_en_grafo"|"archivos_en_disco"] = los denominadores
  │          Checks = append(Checks, ReadinessCheck{...})
  │          Score  = puntaje(Checks)           ← se RECALCULA, o el check nuevo no pesa
  └─ jsonResult(rep)
```

⚠️ **El paso que es fácil olvidar y deja el check de adorno:** `Score` ya viene calculado por el
engine sobre los dos checks viejos. Si se agrega un check y no se recalcula el puntaje de la
dimensión **ni el `Score` global del reporte** (que es el promedio de las dimensiones), el check
nuevo aparece en rojo y el número sigue en 1.0. Es el patrón `guarda-definida-y-desconectada` otra
vez, a un nivel más adentro — y tiene su propio invariante en el plan de pruebas.

## Contratos / interfaces

```go
// ANTES
func (s *McpServer) graphFreshness(scoped context.Context) (stale, ghosts, missing int)

// DESPUÉS — aditivo: mismos tres primeros valores, misma semántica, mismo fail-safe
func (s *McpServer) graphFreshness(scoped context.Context) (stale, ghosts, missing, archivosEnGrafo, archivosEnDisco int)
```

⚠️ **Ajuste de nombres respecto de lo que este diseño escribió primero** (`enGrafo, enDisco`):
`enDisco` **colisiona con una variable local** del cuerpo de `graphFreshness` —la que recibe el
resultado de `walkSourceTree()`— y nombrar así el retorno **no compila**. Quedaron
`archivosEnGrafo` / `archivosEnDisco`, que de paso coinciden con las claves de evidencia
(`archivos_en_grafo`, `archivos_en_disco`). La semántica no cambia: sigue siendo aditivo.

Forma del check (el `Why` carga el diagnóstico completo, que es lo que D1 compra):

```
{ name: "el índice está al día",
  pass: false,
  why:  "87 de 1395 archivos desactualizados (6.2 %, tope 5 %) · 0 fantasmas · 12 sin indexar
         de 1407 en disco · se cura con musubi_codegraph_index" }
```

Claves nuevas en `evidence`, todas crudas (R2): `stale`, `ghosts`, `missing`,
`archivos_en_grafo`, `archivos_en_disco`. Donde no aplica, en su lugar: `frescura_no_medida`.

## Trade-offs

- **Se gana:** la dimensión deja de puntuar 1.0 con un índice inservible, y el argumento del
  fail-safe de `methods_codegraph.go:371` pasa a apoyarse en una alarma que alguien escucha.
- **Se cede costo:** `musubi_readiness` pasa de ser puro SQL agregado a leer y hashear 17,9 MB.
  Medido: **~0,28 s en Python, que es el techo** (Go es más rápido en lectura y en sha256). Para
  una tool de diagnóstico fuera del camino caliente, es aceptable; el número queda escrito para
  poder discutirlo si algún día duele.
- **Se cede peso de los checks viejos:** la dimensión pasa de 2 a 3 checks, así que cada uno vale
  1/3 en vez de 1/2. Efecto secundario honesto: un proyecto con nodos pero **sin aristas** y con el
  índice fresco pasa de 0.5 a 0.67. Es un caso degenerado y raro, su check sigue en rojo a la
  vista, y es el precio de D1 — con tres checks la distorsión sería peor (0.5 → 0.8).

## Plan de pruebas

Regla 8 del repo: cada invariante con su sabotaje verificado en ROJO, y el sabotaje tiene que
atacar **el invariante que el test declara**.

| Invariante | Test | Sabotaje que lo tiene que poner en rojo |
|---|---|---|
| Un grafo podrido no puntúa 1.0 | grafo sembrado con fingerprints que no coinciden | hacer que el check pase siempre (`Pass: true`) |
| **El puntaje REFLEJA el check nuevo** | el score de la dimensión baja, y el global con él | agregar el check **sin** recalcular `Score` — el bug más probable de esta implementación |
| La evidencia trae los tres crudos | las cinco claves presentes y con los valores esperados | devolver el porcentaje en vez del crudo |
| Donde no aplica, no se agrega el check | árbol fuera de alcance | agregar el check igual (pasaría gratis y **subiría** el puntaje) |
| Los dos checks de volumen quedan intactos | sus `Name` y su semántica | renombrar uno |
| El umbral es proporcional, no binario | 1 archivo stale de 1.395 (0,07 %) **pasa** | volverlo binario (`stale == 0`) |

⚠️ **Trampa de la mutación vacua, ya medida en este repo:** sembrar un fingerprint distinto no sirve
si el test no comprueba primero que el archivo **entró** al cómputo. Un `stored` vacío daría cero
stale y el test pasaría sin medir nada. Cada test comprueba el denominador antes de comprobar el
numerador — la misma estructura de dos mitades que usa `TestJournalNoSecretLeak`.
