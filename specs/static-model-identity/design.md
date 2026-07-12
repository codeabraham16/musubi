# Design — static-model-identity

## N1 — Checksum de contenido en el `model_id`

**Hash elegido: CRC32-C (Castagnoli, acelerado por hardware) + los TAMAÑOS, compactado a 12 hex.**

**Primero elegí SHA-256 y la medición lo refutó.** Sobre la tabla real (488MB): SHA-256 = **1.16s**, CRC32-C = **39ms** (30x). La estimación de "~0.2-0.5s con SHA-NI" era optimista y, sobre todo, **el costo caía en el lugar equivocado**: `musubi capture` (la captura automática) resuelve el embedder y carga la tabla **en cada corrida**, así que un hash lento le **duplicaría el arranque** (1.18s → 2.23s). No es una micro-optimización.

Esto **no es un uso criptográfico**: hay que detectar que la tabla cambió, no resistir a un adversario. Para compensar los 32 bits del CRC se mezclan además los **tamaños** de ambos archivos: una colisión tendría que coincidir en CRC **y** en longitud exacta, **en los dos archivos a la vez**. El `sha256` final corre sobre 24 bytes (gratis) y sólo sirve para compactar todo en un id corto y legible.

```go
var castagnoli = crc32.MakeTable(crc32.Castagnoli)

func staticTableChecksum(tableRaw, tokRaw []byte) string {
    var seed [24]byte
    binary.BigEndian.PutUint32(seed[0:4], crc32.Checksum(tableRaw, castagnoli))
    binary.BigEndian.PutUint64(seed[4:12], uint64(len(tableRaw)))
    binary.BigEndian.PutUint32(seed[12:16], crc32.Checksum(tokRaw, castagnoli))
    binary.BigEndian.PutUint64(seed[16:24], uint64(len(tokRaw)))
    sum := sha256.Sum256(seed[:])
    return hex.EncodeToString(sum[:])[:12]
}
```

**Costo final medido (tabla real, 488MB):** checksum **43ms**, carga completa 1.18s ⇒ el checksum es el **~3.7%** de la carga (con SHA-256 era el ~52%).

- **R3 (estabilidad):** el digest depende SÓLO de los bytes ⇒ mtime/ruta/orden no influyen.
- **R5 (sin I/O extra):** `loadStaticTable` ya hace `os.ReadFile(model.safetensors)`; se le hace devolver también ese `raw` (o el digest). El `tokenizer.json` también se lee ya en `loadTokenizer`; se lee su `raw` una vez y se reusa.
- **R6:** el `model_id` sigue llevando el basename delante ⇒ dos dirs distintos con igual contenido siguen difiriendo.

**Refactor mínimo:** `loadStaticTable` pasa a devolver el digest del `raw` que ya leyó (no el `raw` entero, para no retener 488MB de más). `loadTokenizer` idem, o se lee el `tokenizer.json` una vez en `NewStaticProvider` y se pasa a ambos.

## M3 — `AutoEmbedBackfill` en background

```go
// AutoEmbedBackfill cierra SOLO el hueco de procedencia: si hay observaciones activas sin vector
// del model_id actual (memoria previa a encender la semántica, o de otro modelo tras un cambio de
// tabla/checksum), lanza EmbedBackfill EN BACKGROUND. Sin esto, cambiar de modelo apaga la
// semántica hasta un `musubi embed backfill` manual. No bloquea el arranque (R9).
func (e *DbEngine) AutoEmbedBackfill(embed func(string) ([]float32, error)) {
    if e.vectorModelID == "" || embed == nil { return }   // R11
    n, err := e.countStaleEmbeddings()
    if err != nil { logx.Warn(...); return }
    if n == 0 { return }                                   // R7: no-op, el caso común
    logx.Info("re-embebiendo memoria histórica en background", "pendientes", n, "modelo", e.vectorModelID)
    e.spawnBackground(func() {                             // R8: bgWG, no arranca si closed, Close espera
        res, err := e.EmbedBackfill(embed)
        if err != nil { logx.Warn("auto-backfill falló", "error", err); return }
        logx.Info("auto-backfill completo", "embebidas", res.Embedded, "omitidas", res.Skipped)  // R10
    })
}
```

- `countStaleEmbeddings()` reusa exactamente el predicado del `SELECT` de `EmbedBackfill` (`em.observation_id IS NULL OR em.model_id != ?`) en forma de `COUNT(*)` ⇒ una sola fuente de verdad para "qué está pendiente".
- **Por qué contar antes de spawnear:** el caso común (cada arranque, sin cambios) es 0 pendientes; contar es una query barata y evita lanzar una goroutine y logear en cada boot (R7).
- **Por qué background y no síncrono:** un daemon bajo systemd tiene timeout de arranque (~90s por default); una base grande tardaría minutos y **fallaría el arranque de la unit**. `spawnBackground` ya resuelve el cierre limpio (no hay use-after-close).
- **Ventana de degradación:** mientras corre, las obs aún no re-embebidas siguen excluidas del recall semántico. Es lo mismo que pasa HOY de forma permanente hasta el backfill manual; acá dura lo que dura el backfill y se logea (R10).

**Wiring:** en los 2 call-sites de `cmd/musubi/main.go` que ya hacen `SetVectorModelID` + `WarnOnEmbedModelSwitch`, se agrega `engine.AutoEmbedBackfill(func(s string) ([]float32, error) { return embedder.Embed(ctx, s) })`. El engine sigue **model-free**: recibe el callback, no embebe.

## Alternativas descartadas

- **Backfill síncrono al arrancar:** simple, sin concurrencia, pero **rompe el arranque bajo systemd** en bases grandes (timeout). Descartado por R9.
- **Checksum sólo del `model.safetensors`:** deja pasar un cambio de tokenizer, que **sí** cambia los vectores. Viola R2.
- **Checksum en `meta` en vez de en el `model_id`:** detectaría el cambio, pero los vectores viejos **conservarían el mismo `model_id`** ⇒ el filtro de procedencia los seguiría dejando entrar a la búsqueda ⇒ el ranking seguiría corrupto. El valor de meterlo en el `model_id` es justamente que la maquinaria de procedencia que YA existe (F2.2) hace el trabajo sola.
- **Borrar los vectores viejos al detectar el cambio:** destructivo e innecesario. El contrato de procedencia ya los vuelve invisibles, y el backfill los sobrescribe (`INSERT OR REPLACE`). Nada que borrar.
- **SHA-256 sobre el contenido completo:** era la elección inicial; **la medición la descartó** (1.16s sobre 488MB, duplicando el arranque de `musubi capture`). Ver arriba.
- **Hash rápido (FNV-1a):** el `hash/fnv` de stdlib procesa byte-a-byte; sobre 488MB no gana nada. Sin ventaja.
- **`hash/maphash`:** está *randomizado por proceso* ⇒ el mismo contenido daría distinto checksum en cada arranque. **Viola R3** (determinismo) de forma catastrófica: invalidaría todos los vectores en cada boot.
- **Cachear el checksum en un sidecar** (keyed por size+mtime): evitaría rehashear, pero agrega un archivo en el directorio del usuario (que puede ser read-only) y **debilita la garantía** (un re-destill que preserve size y mtime pasaría inadvertido). Con CRC32-C a 43ms, no hace falta.
