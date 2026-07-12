# Spec — banda-ciega-vecinos-al-guardar

Vocabulario RFC 2119. Alcance: `internal/memory/conflicts.go`, `internal/mcp/methods.go`, `internal/config`.

## El invariante

- **R0 — MOSTRAR NO ES ENCOLAR.** Los vecinos de la banda ciega DEBEN mostrarse al agente y **NO DEBEN persistirse**: cero filas nuevas en `observation_relations`, cero `pending`, cero `superseded`. Es **texto informativo**, no un compromiso.
  - Corolario: si se apaga la feature, **no queda rastro** en la base. No hay nada que migrar ni revertir.

## La banda

- **R1** — Un vecino DEBE entrar en la banda si su coseno cae en `[BandFloor, CosineFloor)`: por encima del piso de la banda, y **por debajo** del piso del dedup (si lo alcanzara, ya sería una relación `pending` y se mostraría por el camino de siempre).
- **R2** — `BandFloor` DEBE ser configurable. `BandFloor <= 0` o `BandFloor >= CosineFloor` **apaga** la feature (rollback sin código).
- **R3** — Sin coseno disponible (embedder apagado, o sin vector de la procedencia actual) NO DEBE mostrarse ningún vecino: la banda es una noción **puramente vectorial**.
- **R4** — Los vecinos DEBEN mostrarse **ordenados por coseno descendente** y recortados a un **techo duro** (`maxBandNeighbors`). Si hay más, el mensaje DEBE **decir cuántos quedaron afuera** — un recorte silencioso miente sobre la cobertura.

## Quién los ve

- **R5** — Sólo el camino **explícito del agente** (`musubi_save_observation`, `musubi_save_fact`) muestra vecinos. Los caminos **automáticos** (captura de commits, error→fix — los que corren con `DetectOnly`) NO DEBEN mostrarlos: no hay nadie en el loop a quien mostrarle, y el texto se perdería.
- **R6** — Las **guardas estructurales** de #203 (`complementaryPair`) DEBEN aplicarse también a los vecinos: no tiene sentido mostrarle al agente el `design` del mismo cambio SDD que acaba de guardar.

## Lo que NO cambia

- **R7** — El piso de coseno (`CosineFloor`), el AND-gate, el gate de novedad y el umbral léxico NO se tocan. La cola de `pending` se comporta **exactamente igual que hoy**.
- **R8** — NO se intenta **decidir** si hay contradicción. Se muestra el par y se pregunta; **el veredicto es del agente** (evaluación de predicados ⇒ techo semántico).

## Escenarios

**S.a (el vecino de la banda se muestra)** — *Given* una observación guardada, *When* se guarda otra con coseno **0.82** contra ella (banda: floor 0.80, dedup 0.85), *Then* la respuesta del `save` **menciona** a la primera, y **NO** se crea ninguna fila en `observation_relations`.

**S.b (por encima del piso del dedup NO se duplica el aviso)** — *Given* un par con coseno **0.92**, *Then* se crea la relación `pending` de siempre y ese vecino **NO** aparece además como vecino de banda (no se avisa dos veces por lo mismo).

**S.c (por debajo de la banda, silencio)** — *Given* un par con coseno **0.60**, *Then* no se muestra vecino alguno.

**S.d (sin coseno, silencio)** — *Given* el embedder apagado, *Then* el `save` responde exactamente como hoy: sin vecinos.

**S.e (apagado por config)** — *Given* `BandFloor = 0`, *Then* no se muestra ningún vecino (rollback sin tocar código).

**S.f (el camino automático no muestra)** — *Given* `DetectOnly` (captura de commits), *Then* NO se calculan ni muestran vecinos de banda.

**S.g (el techo es honesto)** — *Given* 7 vecinos en la banda y un techo de 3, *Then* se muestran los 3 de **mayor** coseno y el mensaje **dice que hay 4 más**.

**S.h (las guardas de #203 valen acá también)** — *Given* que se guarda `sdd/x/design` y existe `sdd/x/spec` con coseno en la banda, *Then* NO se muestra como vecino.

## No-objetivos (verificables)

- NO se persiste **ninguna** relación por la banda (R0/S.a).
- NO se modifica el comportamiento de la cola `pending` (R7/S.b).
- NO se detecta toda contradicción: **una con coseno < BandFloor sigue invisible**, y eso es un límite **conocido y declarado**, no un descuido.
