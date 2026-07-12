# Spec — dedup-semantico-coseno

Vocabulario RFC 2119. Alcance: `internal/memory/conflicts.go` + config.

## Invariante de seguridad (la que manda sobre todo lo demás)

- **R0 — El coseno NO DEBE poder auto-suprimir memoria.** Para todo par (src, cand), el veredicto con coseno NO DEBE ser **más permisivo** que el veredicto léxico-puro de hoy. Formalmente: el conjunto de relaciones `supersedes`+`resolved` que produce el gate nuevo DEBE ser un **subconjunto** del que produce el gate viejo, para el mismo pool. El coseno sólo puede: (a) **agregar** relaciones `pending`, o (b) **degradar** una auto-resolución a `pending`. Nunca (c) crear una auto-supresión nueva.
  - *Razón:* los embeddings estáticos no evalúan predicados. *"Usamos X"* y *"ya NO usamos X"* tienen coseno alto. Un OR (o el coseno decidiendo solo) **ocultaría memoria contradictoria en silencio**.

## M2 — El pool ve por semántica

- **R1** — `conflictCandidates` DEBE unir al pool léxico (FTS) un **pool vectorial**: los vecinos por coseno del vector de la observación fuente. La unión (no la intersección) es lo que hace visible al duplicado escrito con **otras palabras**.
- **R2** — Si no hay vector de la fuente (embedder apagado, o sin vector de la procedencia actual), el pool DEBE ser **exactamente** el léxico de hoy (sin error).
- **R3** — El pool NO DEBE incluir la propia observación, ni archivadas, ni superseded (mismas exclusiones que hoy).

## M1/Q4 — El veredicto usa las dos señales (AND-gate)

Para cada candidata se computan **`lex`** (Jaccard de trigramas, como hoy) y **`cos`** (coseno; **ausente** si falta algún vector).

- **R4** — **Auto-resolver** (`supersedes` o `related` con `status=resolved`) DEBE exigir `lex >= auto_resolve_threshold` **Y** `cos >= cosine_auto_threshold`. Las dos. (Con `cos` ausente, ver R7.)
- **R5** — Si `cos >= cosine_floor` pero `lex < similarity_floor` (**duplicado semántico**: mismo significado, otras palabras) ⇒ DEBE emitirse `pending`. Hoy este caso se **ignora**: es el falso negativo que el cambio cierra.
- **R6** — Si `lex >= auto_resolve_threshold` pero `cos < cosine_auto_threshold` (léxicamente casi idénticas pero el coseno no corrobora) ⇒ DEBE **degradarse a `pending`**, NO auto-resolverse. Es el AND-gate protegiendo R0.
- **R7 (equivalencia)** — Si `cos` está **ausente** (sin embedder, o sin vector de alguna de las dos), el veredicto DEBE ser **bit-idéntico al histórico** (léxico puro). Es el camino de rollback y el que corren los usuarios sin semántica.
- **R8** — Un par por debajo de **ambos** pisos (`lex < similarity_floor` y `cos < cosine_floor`) DEBE ignorarse (sin relación).
- **R9** — `cosine_floor <= 0` DEBE desactivar la participación del coseno ⇒ comportamiento histórico (interruptor de rollback por config).

**Escenario D.a (duplicado semántico — el falso negativo que se cierra)** — *Given* dos observaciones que dicen lo mismo con **otras palabras** (`lex` bajo, `cos` alto), *When* se guarda la segunda, *Then* se emite una relación **`pending`** (hoy: **nada**).

**Escenario D.b (AND-gate degrada)** — *Given* `lex = 0.9` (casi idénticas) pero `cos = 0.5` (el coseno no corrobora), *When* se detecta, *Then* el veredicto es **`pending`**, NO `supersedes/resolved`.

**Escenario D.c (auto-resolve sobrevive)** — *Given* un casi-duplicado real (`lex = 0.9`, `cos = 0.99`), mismo `topic_key`, la nueva más reciente, *When* se detecta, *Then* auto-resuelve `supersedes` (igual que hoy).

**Escenario D.d (equivalencia sin vectores)** — *Given* un engine **sin embedder**, *When* se detectan relaciones, *Then* el resultado es **idéntico** al del comportamiento histórico.

**Escenario D.e (R0, el invariante)** — *Given* cualquier par, *When* se compara el gate nuevo con el viejo, *Then* el gate nuevo **nunca** produce un `supersedes/resolved` que el viejo no produjera.

## No-objetivos (verificables)

- NINGÚN auto-merge ni auto-borrado: el dedup semántico **sólo rutea a `pending`**. Fusionar/suprimir lo decide el agente (`musubi_judge`).
- NO se toca `Similarity` (Jaccard), `consolidate.go` ni la retención.
