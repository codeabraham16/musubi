# Spec — doctor-fts-repair-hardening

Vocabulario RFC 2119. Alcance: `internal/memory/doctor.go`.

## El invariante

- **R0 — El camino de reparación NO DEBE depender de leer lo que está roto.** Ni la detección, ni el backup previo, ni la reconstrucción pueden requerir recorrer las páginas corruptas: son exactamente las que no se pueden leer.

## P0.3 — La detección VE la corrupción

- **R1** — `fts_consistency` DEBE correr el comando nativo de FTS5 **`integrity-check`** además del drift de `COUNT(*)`.
- **R2** — Si el `integrity-check` falla, el check DEBE reportar `warning`/`error` con `Repairable: true` (o sea: **entra al auto-heal**). Hoy un índice corrupto con el conteo correcto reporta **`ok`**.
- **R3** — El drift de `COUNT(*)` DEBE seguir detectándose (es un modo de falla distinto: desincronización sin corrupción).

## P0.2 — La reconstrucción NO recorre el índice corrupto

- **R4** — `applyRebuildFTS` DEBE reconstruir con **`DROP TABLE` + recrear + re-poblar** desde `observations`, NO con `DELETE FROM observations_fts`.
  - *Razón:* `DELETE` recorre el b-tree del índice ⇒ toca las páginas corruptas ⇒ falla. `DROP` libera páginas **sin recorrer** el contenido.
  - *No sirve* el comando `'rebuild'` de FTS5: `observations_fts` es una tabla **regular** (sin `content=`), y `'rebuild'` sólo aplica a *contentless* / *external-content*.
- **R5** — Tras reconstruir, el índice DEBE quedar **funcional y completo**: una búsqueda FTS posterior DEBE encontrar las observaciones existentes.
- **R6** — Los **triggers** de sincronización DEBEN seguir funcionando tras el DROP+recreate (están sobre `observations`, no sobre la tabla FTS, así que sobreviven — pero hay que **verificarlo**).

## P0.1 — El backup funciona sobre una base corrupta

- **R7** — Si `VACUUM INTO` falla, `BackupTo` DEBE caer a una **copia cruda de bytes** del archivo (`.db` y, si existen, `.wal` y `.shm`). Una copia de bytes **no parsea páginas** ⇒ funciona sobre una base corrupta.
- **R8** — El fallback DEBE **logearse claramente** como un backup **de rescate** (posiblemente inconsistente si hubo escrituras concurrentes), para no dar una falsa sensación de seguridad.
- **R9** — El camino feliz NO DEBE cambiar: `VACUUM INTO` se sigue intentando **primero**, y si funciona, el resultado es idéntico al de hoy.
- **R10** — Si **ambos** caminos fallan, DEBE devolverse un error (no un backup silenciosamente ausente).

**Escenario F.a (la detección ve)** — *Given* un `integrity-check` de FTS que falla, *When* corre `doctor`, *Then* `fts_consistency` reporta el problema como **reparable** (hoy: `ok`).

**Escenario F.b (la reconstrucción funciona)** — *Given* observaciones guardadas, *When* corre `applyRebuildFTS`, *Then* la tabla FTS queda recreada y una búsqueda posterior encuentra las observaciones.

**Escenario F.c (los triggers sobreviven)** — *Given* un rebuild ya hecho, *When* se guarda una observación NUEVA, *Then* el FTS la indexa (el trigger sigue vivo) y la búsqueda la encuentra.

**Escenario F.d (backup de rescate)** — *Given* que `VACUUM INTO` falla, *When* se pide un backup, *Then* se produce igualmente un archivo por copia cruda, y se avisa que es de rescate.

**Escenario F.e (el camino feliz no cambia)** — *Given* una base sana, *When* se pide un backup, *Then* se usa `VACUUM INTO` (no el fallback) y el resultado es una base válida.

## No-objetivos (verificables)

- NO se mete `db_integrity` en el auto-heal (reparar la base, no el índice, sin supervisión sigue siendo demasiado riesgoso).
- NO se tocan el esquema, los triggers ni los demás checks.
