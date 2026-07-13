# Spec — registros-historicos-nunca-son-destino

Vocabulario RFC 2119. Alcance: `internal/memory/conflicts.go`.

## El invariante

- **R0 — Un REGISTRO HISTÓRICO nunca es el DESTINO de una relación.** Un `git-commit` (lo que pasó) y un `sdd/<cambio>/<fase>` (lo que se acordó) NO se pueden des-hacer, así que ninguna relación DEBE nacer apuntando a ellos — de ninguna clase, incluida la propia.

## F1 — La guarda

- **R1** — `complementaryPair(a, b)` DEBE devolver `true` siempre que `b.topicKey` sea un registro histórico (`git-commit` o `sdd/<cambio>/<fase>`).
- **R2** — La guarda DEBE mirar **sólo el target** (`b`). El `source` (`a`) NO DEBE influir. Esta asimetría es la que permite que un commit vuelva obsoleta una nota.
- **R3** — La guarda DEBE aplicarse en el **único punto** donde nacen las relaciones (el loop de candidatas de `DetectRelations`) y en la banda ciega (`BandNeighbors`), **antes** de cualquier scoring. NO DEBE ser posible saltearla.
- **R4** — El comportamiento de G1 (hermanos SDD) y G2 (evento vs contrato) DEBE preservarse **bit a bit**. Quedan subsumidos, no eliminados.

## Lo que NO cambia

- **R5** — El scoring léxico/coseno, los umbrales, el auto-resolve, la banda ciega y MMR: **intactos**.
- **R6** — Las relaciones **ya existentes** NO se tocan: ni se borran, ni se re-juzgan, ni se ocultan. El cambio rige para las relaciones **futuras**.
- **R7** — Un registro histórico SÍ PUEDE seguir siendo **source**: un commit puede volver obsoleta una nota.

## Escenarios

**S.a (commit no reemplaza a commit)** — *Given* dos observaciones `git-commit` con textos casi idénticos, *When* se guarda la segunda, *Then* NO nace ninguna relación entre ellas.

**S.b (contrato no reemplaza a contrato de otro cambio)** — *Given* `sdd/cambio-a/spec` y `sdd/cambio-b/spec` con textos muy parecidos, *Then* NO nace ninguna relación.

**S.c (G1 sigue valiendo)** — *Given* `sdd/x/spec` y `sdd/x/design` del MISMO cambio, *Then* NO nace ninguna relación (hermanos, se complementan).

**S.d (G2 sigue valiendo)** — *Given* un `git-commit` y un `sdd/x/spec`, en CUALQUIER orden de guardado, *Then* NO nace ninguna relación.

**S.e (la asimetría se conserva — el caso que NO se debe romper)** — *Given* una nota `usamos X` ya guardada y un commit `feat: migrar de X a Y` que se guarda después, *When* el par supera los umbrales, *Then* SÍ nace la relación (`source` = commit, `target` = nota). **El commit es evidencia de que la nota envejeció.**

**S.f (nota vs nota intacta)** — *Given* dos notas contradictorias, *Then* la relación nace como siempre. Es el único par donde `supersedes` significa algo.

**S.g (la banda ciega respeta la misma guarda)** — *Given* un vecino en la banda cuyo `topic_key` es histórico, *Then* NO se muestra.

## La medición (obligatoria)

- **R8** — DEBE reportarse, sobre las **169 relaciones reales**, cuántas bloquea la guarda nueva y **cuántos veredictos sustantivos se romperían**. El número esperado de veredictos rotos es **0**; si no lo fuera, la propuesta está mal y se cae.

## No-objetivos (verificables)

- NO se rompe ningún veredicto sustantivo pasado (R8).
- NO se pierde la asimetría (S.e).
- NO cambia el comportamiento de G1 ni G2 (R4, S.c, S.d).
