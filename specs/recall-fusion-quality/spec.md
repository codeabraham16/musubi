# Spec — recall-fusion-quality

Vocabulario RFC 2119. Alcance: `internal/memory/recall.go` (+ wiring de config en la capa MCP).

## Q1 — Piso de coseno en el pool vectorial

- **R1** — `augmentWithVectorPool` DEBE descartar los `SearchResult` cuya `Similarity` sea **< piso** ANTES de construir `vecRank` y de reunir los ids `missing`. Un candidato descartado NO DEBE recibir término RRF vectorial ni ser traído al pool.
- **R2** — El piso DEBE ser configurable por el caller (campo en `RecallOptions`, cableado desde config en la capa MCP).
- **R3** — Un piso `<= 0` DEBE reproducir el comportamiento histórico **bit-a-bit** (sin filtrado): es el interruptor de rollback.
- **R4** — El recall 100% léxico (sin `QueryVector`) NO DEBE cambiar en absoluto (no pasa por `augmentWithVectorPool`).
- **R5** — El filtrado NO DEBE alterar el orden relativo de los resultados que sobreviven (mismo `vecRank` que tendrían entre sí).

**Escenario Q1.a** — *Given* `SearchObservations` devuelve sims `[0.90, 0.52, 0.28, 0.11]` y piso `0.40`, *When* se augmenta el pool, *Then* solo los de `0.90` y `0.52` entran a `vecRank` (posiciones 0 y 1) y a `missing`; los de `0.28`/`0.11` se descartan.

**Escenario Q1.b** — *Given* piso `0` (o negativo), *When* se augmenta, *Then* el resultado es idéntico al histórico (los 4 entran).

## Q2 — Degradación elegante ante FTS corrupto

- **R6** — Si `recallCandidates` falla con un error de **corrupción del FTS** (SQLITE_CORRUPT / "malformed" / "database disk image"), `Recall` DEBE registrar el evento (log/telemetría) y **continuar** con pool léxico vacío (`cands` sin candidatos léxicos, `lexRank == nil`), permitiendo que el pool vectorial y/o el fallback por recencia llenen el resultado.
- **R7** — Cualquier OTRO error de `recallCandidates` DEBE seguir propagándose (`return err`): la degradación se acota a la clase de corrupción.
- **R8** — Tras degradar, si hay `QueryVector`, el recall DEBE devolver los candidatos del pool vectorial (resultado NO vacío).
- **R9** — Tras degradar, si NO hay `QueryVector` ni candidatos, el resultado DEBE ser vacío y SIN error (no un fallo duro).

**Escenario Q2.a** — *Given* el FTS está corrupto y hay un `QueryVector` servible, *When* se llama `Recall`, *Then* devuelve items del pool vectorial, sin error.

**Escenario Q2.b** — *Given* `recallCandidates` falla por un error NO-corrupción (p. ej. contexto cancelado), *When* se llama `Recall`, *Then* propaga el error.

## No-objetivos (verificables)

- El scoring de `scoreCandidates` (fórmula RRF × importance) NO se toca (eso es Q3, fuera de alcance).
- Ninguna ruta auto-suprime, auto-archiva ni auto-supersede memoria: Q1 solo filtra la ENTRADA al pool de ranking.
