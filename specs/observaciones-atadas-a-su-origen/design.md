---
artifact: design
schema_version: "1.0"
change: observaciones-atadas-a-su-origen
status: draft
---

# Diseño — Observaciones atadas al estado que las originó

## Decisión 1 — Tabla satélite, no columnas en `observations`

```sql
CREATE TABLE IF NOT EXISTS observation_origins (
    observation_id TEXT NOT NULL REFERENCES observations(id) ON DELETE CASCADE,
    path           TEXT NOT NULL,   -- relativa a la raíz, normalizada con '/'
    fingerprint    TEXT NOT NULL,   -- sha256 hex del contenido AL GUARDAR
    captured_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (observation_id, path)
);
CREATE INDEX IF NOT EXISTS idx_obs_origins_path ON observation_origins(path);
```

**Rationale:** la relación es 1:N (una observación puede hablar de varios archivos), así que en
columnas no entra sin desnormalizar. Además cumple R5 de la forma más fuerte posible: las 968
observaciones existentes no se tocan ni se migran, simplemente no tienen filas acá.

**Alternativa descartada:** reusar `observation_relations`. Es observación↔observación y su
semántica es la de veredictos entre notas (`pending/scoped/supersedes`). Meter un archivo ahí
ensuciaría el detector de conflictos y la cola de `musubi_conflicts`.

**FK con `ON DELETE CASCADE`, contra la costumbre de la base.** Hoy ninguna tabla referencia
`observations` y por eso borrar en duro exige limpiar satélites a mano — algo que ya nos mordió.
El DSN abre con `_pragma=foreign_keys(1)`, así que la FK es efectiva y R14 sale gratis. Hay que
actualizar la receta de borrado en duro, que hoy documenta la limpieza manual.

## Decisión 2 — Reusar `FileFingerprint`, no inventar hash

`internal/memory/codepath.go:31` ya expone exactamente lo que pide R4:

```go
func FileFingerprint(root, path string) (string, error)  // sha256 hex del contenido
```

Es la misma función que alimenta `code_memory.fingerprint`, así que un ancla y el índice de
código de la misma ruta son comparables byte a byte. Las rutas se normalizan con
`NormalizeCodePath` (mismo archivo), que ya resuelve absolutas contra `root` y pasa a `/`.

**Nada nuevo que testear a nivel hash**: se hereda el comportamiento ya probado.

## Decisión 3 — La marca es campo estructurado Y texto en el gist

```go
// StaleOrigin describe un ancla que ya no coincide con el disco.
type StaleOrigin struct {
    Path   string `json:"path"`
    Reason string `json:"reason"` // "changed" | "missing"
}

type RecallItem struct {
    // ...
    Stale []StaleOrigin `json:"stale,omitempty"`
}
```

y el gist se prefija con `⚠ posiblemente rancia (cambió: internal/memory/workflow.go) — `.

**Rationale:** hay precedente literal en el mismo struct. `CreatedAt` se agregó con este
comentario: *"sin ella el agente no distingue una nota de ayer de una de 8 meses y trata la vieja
como verdad ACTUAL — la raíz de la divagación por memoria caduca"*. Esta feature es la versión
**derivable** de esa misma pelea: la edad es un proxy, el fingerprint es evidencia. El texto en el
gist es lo que el agente lee (R10); el campo estructurado es para el dashboard y para tests que no
dependan del wording. `omitempty` garantiza salida byte-idéntica sin anclas (R5).

## Decisión 4 — Verificar sólo lo ya seleccionado, con memo por llamada

El enganche va en `Recall` (`internal/memory/recall.go:144`), **después** de rankear y recortar por
presupuesto, **antes** de devolver. Nunca antes: si influyera en la selección violaría R12 y R11.

```go
// markStaleOrigins anota los items cuyo ancla ya no coincide con el disco.
// Memo por llamada: varias observaciones suelen anclar al mismo archivo.
func (e *DbEngine) markStaleOrigins(items []RecallItem) []RecallItem
```

Una sola consulta `WHERE observation_id IN (...)` para todos los items, y un `map[string]string`
de ruta→fingerprint calculado como mucho una vez por ruta. Con un recall típico de ~15 items eso
son a lo sumo un puñado de lecturas de archivo.

**Alternativa descartada:** reusar `code_memory.fingerprint` como valor "actual" en vez de leer el
disco. Es más barato pero *ese* fingerprint también puede estar viejo — compararíamos una foto
contra otra foto y la respuesta no sería derivada del mundo, que es todo el punto.

## Decisión 5 — Sólo anclas declaradas en esta iteración

`origin_paths` explícito en `musubi_save_observation`. **No** se infiere de los archivos tocados en
el turno, aunque el hook de codegraph tenga la señal.

**Rationale:** un ancla mal inferida es peor que ninguna. Ata la nota a un archivo del que no habla
y genera una marca que nunca se apaga — ruido permanente, y el ruido permanente es exactamente
cómo se erosiona la confianza en una señal. Declarar es barato; inferir mal es caro y silencioso.
Queda como refinamiento una vez que haya uso real que muestre qué se infiere bien.

## Decisión 6 — Tope de 10 rutas por observación

Excederlo es error al guardar, no truncado silencioso.

**Rationale:** acota el costo de R12 y, sobre todo, es una señal de diseño: si una observación
necesita anclarse a más de diez archivos, no habla de un estado concreto y la marca no le va a
servir a nadie. El truncado silencioso daría una observación anclada a medias, que es la clase de
media-verdad que este cambio vino a eliminar.

## Decisión 7 — Las anclas no viajan (R6)

El payload del outbox se arma con los campos de `observations`; al vivir en otra tabla, las anclas
quedan fuera **sin escribir código**. Se agrega un test que lo fija, para que un refactor futuro
del sync no las arrastre por accidente: un fingerprint calculado en kernelos-pc no significa nada
contra el checkout del server.

## Cambios de superficie

- `internal/memory/database.go` — DDL de `observation_origins` en el bootstrap del esquema.
- `internal/memory/recall.go` — `StaleOrigin`, campo `Stale` en `RecallItem`, `markStaleOrigins`.
- `internal/memory/observations.go` (o donde viva el save) — persistir anclas en la misma tx que la
  observación; validar existencia (R3) y tope (D6).
- `internal/memory/doctor.go` — chequeo `orphan_origins` (R15).
- `internal/mcp/methods.go` + `registry.go` — arg `origin_paths`; golden.
- Sin cambios en workflow, cognición ni codegraph.

## Riesgo principal y su contención

El fracaso más probable no es técnico: es que la marca genere ruido y se aprenda a ignorarla. Lo
contienen D5 (sólo declarado), D6 (tope bajo) y R11 (nunca oculta). Si aun así aparece ruido, la
salida NO es empezar a ocultar: es afinar qué se ancla.
