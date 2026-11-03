---
artifact: proposal
schema_version: "1.0"
change: la-sombra-declara-su-cobertura
status: draft
---

# Propuesta — la tabla de la sombra declara su cobertura

## Intención

`musubi conflicts shadow` imprime una tasa de acuerdo por veredicto y **no dice sobre cuántos pares
la calculó**. Corrido el 2026-11-02 contra la base viva de este repo, el comando imprime: de
**1.138** relaciones, **901** son re-detectables (las dos puntas visibles) y de ésas sólo **4** son
medibles, porque **897 tienen firma ajena**. Otras **237** no entran al cómputo porque una punta está
archivada, superseded o en cuarentena — y **2** de ésas son auto-resoluciones del detector que ya
surtieron efecto: su propio `supersedes` dejó invisible al target, así que el par **se autoexcluye**
en el acto.

Las cifras son una **foto** y se mueven rápido, porque **arbitrar un conflicto las mueve** y trabajar
en esto arbitra conflictos. La serie medida del día: 1.131 / 901 / **6**, después 1.138 / 901 / **4**,
y al cerrar el expediente 1.140 / 903 / **0**. Lo llevó a cero este mismo trabajo, arbitrando
relaciones por otro motivo y sin tocar ninguno de los 4 medibles. O sea que el techo es
**auto-consumible** —cualquier arbitraje drena un `pending`, y un `heuristic` aplicado se autoexcluye—
y su estado estacionario es CERO.

Por eso el criterio de éxito de abajo pide que el **desglose esté impreso**, no que coincida con un
número de este documento. ⚠️ Y **no** pide que cierren las dos identidades: la verify midió que son
tautologías —cierran igual con un 56 % de error en el titular—, así que son necesarias y no
suficientes. Ver el criterio 1.

O sea que la tabla que alguien lee para decidir si mueve el umbral de 0,70 reporta sobre una porción
mínima del corpus —hoy una **fracción del 1 %**—, y nada en la salida lo dice. «supersedes: tasa 0,95» se lee igual si salió de 20 pares
o de 20 sobre 1.100.

Y el dato del recorte **no es recuperable después**: `ajenos` y `tirados` son campos del
`shadowWorker` en memoria y salen únicamente en la línea de log del apagado
(`internal/mcp/shadow.go:227`). Cuando el proceso muere, el denominador desaparece. No es un
problema de presentación —«el dato está en el log y no en la tabla»—: es que el dato **deja de
existir**.

## Alcance

- **Incluye:**
  - Una cobertura **derivada por SQL** del estado actual del corpus: pares con relación,
    re-detectables, medibles, recortados por firma ajena, y no re-detectables por punta invisible.
  - `musubi conflicts shadow` la declara **antes** de la tabla; `--json` la trae como objeto con
    tags explícitos.
  - Que el reporte informe aunque `shadow_verdicts` esté vacía, que es el caso de hoy: **0 medidas**
    en el libro aunque el techo no sea cero.
  - Un custodio de `SaveShadowVerdict` en `internal/memory`, que hoy no tiene **ninguno** de su
    propio paquete aunque ahí se computa `agree`, la columna de la que sale la tasa.

- **No incluye:**
  - La **pantalla model-free** en `encolarSombra` (el par procedencia-señal imposible). El dueño ya
    resolvió la escalada del panel eligiendo la opción (B), explícitamente sin ella.
  - El caso **S6** (el entrante sin firma).
  - El custodio del contrato de `UpsertObsRelation` (un par `pending` **firmado** pierde la guarda).
    Medido hoy: **latente**, nadie produce ese estado. Va a su propio PR.
  - Persistir `medidos`/`fallidos`/`tirados`/`ajenos` del worker. La propuesta es justamente **no**
    persistir contadores.
  - Tocar la guarda `setConservandoVeredicto`, el esquema, o el camino de guardado.

## Enfoque

**Derivar, no contar.** El recorte no es una propiedad de una corrida del worker: es una propiedad
del **corpus**. Contarlo en memoria lo vuelve efímero y dependiente de qué proceso lo vio; derivarlo
por SQL en el momento de reportar lo vuelve durable, siempre al día, sin migración de esquema y sin
una sola escritura en el camino caliente del guardado.

Es la misma política que el repo ya aplica en otro lado: preferir agregación SQL o aritmética
determinista antes que estado nuevo.

## Impacto

- Áreas/archivos afectados: `internal/memory/shadow.go` (la consulta), `cmd/musubi/conflicts.go`
  (la salida), más sus tests. `internal/mcp/shadow.go` sólo en prosa, para que el aviso del rastro
  deje de prometer lo que ya no hace falta mirar en el journal.
- Compatibilidad: **no rompe nada**. Sin esquema nuevo, sin migración, sin cambio de firma de nada
  exportado. La tabla existente se imprime igual; gana una cabecera arriba. El `--json` gana una
  clave; los consumidores que ignoran claves desconocidas no se enteran.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| El criterio de «medible» vive en Go (`encolarSombra`) y lo necesito en SQL: dos copias divergen en silencio | Armar el fragmento SQL desde las **mismas constantes** (`RelStatusPending`, `RelResolvedByHeuristic`), como ya hace `setConservandoVeredicto`, y pinearlo con una directiva `// arnes: de=…` para que el pin sea de conducta y no de texto |
| El criterio de «re-detectable» podría copiarse a mano y salir mal | Reusar `visibleObsPredicateDe(alias)`, que existe para esto y que `obsrank.go:62-63` ya usa para las dos puntas de una arista. El comentario de `scope.go:51` documenta que el único sitio que lo escribió a mano **salió mal** |
| La cifra puede leerse como «la sombra está rota» cuando el recorte es correcto | La cabecera nombra el **motivo** de cada recorte, no sólo el número: un veredicto ajeno no se mide a propósito |
| Una consulta de cobertura sobre un corpus grande podría pesar | Es una agregación por `COUNT` sobre `observation_relations` con dos JOIN por id (clave primaria), y corre **una vez por invocación de un comando de consola**, no en el camino de guardado |

## Criterio de éxito

1. `musubi conflicts shadow` contra la base viva imprime la cobertura **con su desglose**: el total
   del libro, los re-detectables, y los baldes del recorte cada uno con su número y su motivo. El
   criterio es que el desglose esté impreso, **no** que coincida con una cifra de este documento
   —las cifras son una foto del corpus y se movieron dos veces el mismo día en que se escribió
   esto—. ⚠️ **Y NO alcanza con que cierren las dos identidades**: el panel midió que cierran
   igual con un error del 56 % en el titular, porque son tautologías (ver el plan de pruebas del
   design). Que cierren es **necesario y no suficiente**; lo que hay que verificar aparte es que
   `Redetectables` cuente RELACIONES y no fuentes —`COUNT(1) == COUNT(DISTINCT r.id)`— y que
   `PuntaNoVisible` no salga negativo.
2. Cada requisito de la spec tiene un custodio que se pone **rojo** bajo sabotaje del invariante que
   declara, con el sabotaje verificado por `diff` antes de creerle al resultado.
3. `go build` + `go vet` + `go test -count=1` en verde con `GOWORK=off` y `-tags treesitter`.
