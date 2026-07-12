# Proposal — doctor-fts-repair-hardening

## Intención

Cerrar la **Fase 0 (P0)** del track: **el `doctor` no puede reparar justo cuando más se lo necesita.**

Lo vivimos en vivo. Con la memoria de este repo corrupta, `musubi doctor` dijo:

```
db_integrity: corruption ... observations_fts   (repairable: false)
fts_consistency: índice FTS sincronizado        ✓ ok
```

**El check que VE el problema no lo puede arreglar, y el que lo PUEDE arreglar no lo ve.** La base se salvó sola (un checkpoint del WAL reescribió las páginas durante un backfill) — pura suerte.

## Tres fallas que se COMPONEN

| | Qué pasa |
|---|---|
| **P0.3 — La detección es ciega** | `fts_consistency` (el único con `apply`, y el único en el auto-heal) detecta con `countFTSDrift`, que sólo compara `COUNT(*)` de `observations` vs `observations_fts`. **Un índice FTS internamente corrupto puede tener el conteo PERFECTO** ⇒ drift = 0 ⇒ reporta **"ok"**. Mientras tanto `db_integrity` sí ve la corrupción… pero **no tiene `apply`** (está fuera del auto-heal a propósito). |
| **P0.2 — El repair recorre lo corrupto** | `applyRebuildFTS` hace `DELETE FROM observations_fts`, que **recorre el b-tree** del índice ⇒ toca las páginas corruptas ⇒ **falla**. Justo la operación que tenía que curarlo. |
| **P0.1 — El backup previo también** | El auto-heal respalda antes de reparar: `backupDB` → `BackupTo` → **`VACUUM INTO`**, que **lee toda la base** ⇒ falla sobre las páginas corruptas ⇒ **aborta antes de reparar nada**. |

Se componen en cadena: **la detección es ciega; aunque viera, el backup fallaría; y aunque el backup anduviera, el repair fallaría.** Tres puertas cerradas en fila.

## Alcance

- **P0.3 — Detección real.** `fts_consistency` suma el comando nativo de FTS5 `integrity-check` al `COUNT` drift. Detecta corrupción **interna**, no sólo desincronización de conteo.
- **P0.2 — Reconstrucción que no recorre.** `applyRebuildFTS` pasa de `DELETE FROM` a **`DROP TABLE` + recrear + re-poblar** desde `observations`. `DROP` libera páginas **sin recorrer** el contenido; `DELETE` lo recorre.
  - Ojo: `observations_fts` es una tabla FTS5 **regular** (sin `content=`), o sea guarda su propia copia sincronizada por triggers. Eso **descarta** el comando `'rebuild'` de FTS5, que sólo aplica a tablas *contentless* o *external-content*. Por eso DROP+recreate y no `'rebuild'`.
- **P0.1 — Backup page-agnóstico.** Si `VACUUM INTO` falla (justamente porque la base está corrupta), caer a una **copia cruda de bytes** (`io.Copy` del `.db` + `.wal` + `.shm`). No parsea páginas ⇒ funciona sobre una base corrupta. Es un backup **de rescate**: peor que el consistente, infinitamente mejor que ninguno.

## Fuera de alcance (explícito)

- NO se mete `db_integrity` en el auto-heal: reparar una corrupción de la base (no del índice) sin supervisión sigue siendo demasiado riesgoso. Lo que cambia es que **el check que sí repara ahora VE** el problema.
- No se toca el esquema, ni los triggers, ni el resto de los checks.

## Estrategia de rollback

Revertir el PR restaura el comportamiento exacto. Sin migración de esquema. El backup crudo es un **fallback**: el camino feliz (`VACUUM INTO`) queda intacto y sigue siendo el primero.

## Riesgos

- **DROP+recreate sobre un FTS corrupto podría fallar igual** si la corrupción está en la estructura misma del b-tree de las shadow tables. Es estrictamente **mejor** que `DELETE` (que garantizado recorre el contenido), pero no es una bala de plata. Hay que decirlo y no vender más de lo que es.
- **El backup crudo puede capturar un estado a medias** si hay escrituras concurrentes (por eso `VACUUM INTO` existe). Se usa **sólo como fallback** cuando el consistente ya falló, y se **logea claramente** que es un backup de rescate.
- Reproducir corrupción real de FTS5 en un test es difícil. Hay que ser honesto sobre qué se puede testear de verdad (el camino de DROP+recreate, el fallback del backup, la detección con `integrity-check`) y qué no (corromper páginas a mano de forma portable).
