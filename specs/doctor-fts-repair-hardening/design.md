# Design — doctor-fts-repair-hardening

## El principio

**Nada del camino de reparación puede depender de LEER lo que está roto.** Suena obvio dicho así, y sin embargo las tres etapas (detectar → respaldar → reconstruir) lo violaban.

## P0.3 — Detección: `integrity-check` nativo de FTS5

```go
// ftsIntegrityErr corre el comando NATIVO de FTS5 'integrity-check', que valida la estructura
// INTERNA del índice. Es lo que countFTSDrift no puede ver: un índice corrupto puede tener el
// COUNT(*) perfecto (las filas están; lo que está roto es el b-tree del índice invertido).
func ftsIntegrityErr(e *DbEngine) error {
    _, err := e.db.Exec(`INSERT INTO observations_fts(observations_fts) VALUES('integrity-check')`)
    return err
}
```

`checkFTS` corre **las dos** comprobaciones (son **modos de falla distintos**):
- **`integrity-check` falla** ⇒ corrupción **interna** ⇒ `error`, `Repairable: true` ⇒ **entra al auto-heal**.
- **drift de `COUNT(*)`** ⇒ desincronización (filas de más/de menos) **sin** corrupción ⇒ `warning`, reparable (como hoy).

El orden importa: primero el `integrity-check` (el más grave), porque si el índice está corrupto el `COUNT` puede ser una lectura sin sentido.

## P0.2 — Reconstrucción: `DROP` en vez de `DELETE`

```sql
DROP TABLE IF EXISTS observations_fts;
CREATE VIRTUAL TABLE observations_fts USING fts5(id UNINDEXED, topic_key UNINDEXED, content);
INSERT INTO observations_fts(id, topic_key, content) SELECT id, topic_key, content FROM observations;
```

- **`DELETE FROM` recorre el b-tree** del índice para borrar fila por fila ⇒ toca las páginas corruptas ⇒ **falla justo en el caso que tenía que curar**.
- **`DROP TABLE` libera las páginas sin leer el contenido** ⇒ sobrevive a la corrupción del contenido.
- **`'rebuild'` de FTS5 NO sirve acá.** Es el comando que uno buscaría primero, pero sólo aplica a tablas *contentless* o *external-content* (`content=...`). `observations_fts` es una FTS5 **regular**: guarda su propia copia, sincronizada por triggers. Sobre ella, `'rebuild'` da error. Por eso DROP+recreate.
- **Los triggers sobreviven**: están definidos sobre `observations` (`AFTER INSERT/UPDATE/DELETE`), no sobre la tabla FTS. Dropear la tabla FTS no los borra, y siguen apuntando a `observations_fts` **por nombre** ⇒ al recrearla, vuelven a funcionar. **Esto se verifica con un test** (F.c), no se asume.
- La definición de la tabla se toma de **una sola fuente** (una constante compartida con el esquema) para que no puedan divergir.

## P0.1 — Backup: fallback page-agnóstico

```
VACUUM INTO  →  ¿falló?  →  copia CRUDA de bytes (.db + .wal + .shm)
```

- `VACUUM INTO` **lee y reescribe todas las páginas** ⇒ es exactamente lo que **no** se puede hacer sobre una base corrupta. Pero es el mejor backup cuando la base está **sana** (transaccionalmente consistente + compacta) ⇒ **se sigue intentando primero** (R9).
- El fallback hace `io.Copy` de los **bytes** ⇒ no parsea nada ⇒ **funciona sobre una base corrupta**.
- Se copian también `.wal` y `.shm` **si existen**: sin el WAL, la copia del `.db` puede quedar sin los commits recientes.
- Se **logea como backup DE RESCATE** (R8): puede ser inconsistente si hubo escrituras concurrentes. Hay que decirlo — un backup que parece bueno y no lo es, es peor que uno que se sabe imperfecto.

**Es un backup peor, y aun así infinitamente mejor que ninguno.** Hoy, cuando la base está corrupta, el auto-heal **aborta sin respaldar ni reparar**.

## Qué se puede testear de verdad (honestidad sobre la cobertura)

Corromper páginas de SQLite a mano, de forma **portable** y **determinista**, es frágil (depende del formato de página, del tamaño, del layout). No lo voy a fingir. Lo que **sí** se testea:

- **F.b/F.c** — que `DROP`+recreate deja el índice **funcional** y que **los triggers siguen vivos** (una observación nueva se indexa y se encuentra). Esto es lo que más fácilmente se rompe en este cambio, y es 100% testeable.
- **F.a** — que un `integrity-check` OK reporta `ok`, y que el drift de `COUNT` se sigue detectando.
- **F.d/F.e** — el fallback del backup: forzando el fallo de `VACUUM INTO` (destino inválido / base cerrada) se verifica que **igual produce un archivo**, y que con una base sana usa el camino feliz.

Lo que **no** se testea: corrupción real de páginas. Se ataca por **construcción** (no leer lo roto), no por un test que simule un daño específico.

## Alternativas descartadas

- **Meter `db_integrity` en el auto-heal:** reparar la **base** (no el índice) sin supervisión es demasiado riesgoso. La solución correcta no es darle poder al que ve, sino **darle vista al que ya tiene el poder** (`fts_consistency`).
- **`'rebuild'` de FTS5:** no aplica a tablas regulares. Ver arriba.
- **Backup crudo SIEMPRE:** perdería la consistencia transaccional que `VACUUM INTO` da gratis en el caso sano (que es el 99.9%).
