# Spec — static-model-identity

Vocabulario RFC 2119. Alcance: `internal/embedding/static.go` (N1), `internal/memory/embed_backfill.go` (M3), wiring en `cmd/musubi/main.go`.

## N1 — El `model_id` identifica el CONTENIDO de la tabla

- **R1** — El `model_id` del `StaticProvider` DEBE tener la forma `static:<basename>@<checksum>`, donde `<checksum>` deriva del **contenido** de la tabla.
- **R2** — El checksum DEBE cubrir **tanto** los bytes de `model.safetensors` **como** los de `tokenizer.json`: un cambio en cualquiera de los dos cambia los vectores producidos, y por lo tanto DEBE cambiar la identidad.
- **R3** — El checksum DEBE ser **determinista y estable**: cargar dos veces la misma tabla (mismo contenido) DEBE dar el mismo `model_id`, sin importar mtime, ruta absoluta ni orden de lectura.
- **R4** — Re-destilar la tabla **in-place** (mismo directorio y nombre, contenido distinto) DEBE producir un `model_id` **distinto**. Éste es el bug de raíz: hoy produce el mismo.
- **R5** — El checksum NO DEBE requerir I/O adicional: se calcula sobre los bytes que `loadStaticTable` ya lee.
- **R6** — Dos directorios con **nombre distinto** pero contenido **idéntico** DEBEN seguir teniendo `model_id` distinto (el basename sigue siendo parte de la identidad; el checksum sólo la refina).

**Escenario N1.a** — *Given* una tabla en `dir/`, *When* se carga dos veces, *Then* el `model_id` es idéntico las dos veces.

**Escenario N1.b** — *Given* una tabla ya cargada, *When* se modifica el contenido de `model.safetensors` in-place y se recarga, *Then* el `model_id` cambia.

**Escenario N1.c** — *Given* una tabla ya cargada, *When* se modifica sólo `tokenizer.json` in-place y se recarga, *Then* el `model_id` **también** cambia (R2).

## M3 — Auto-backfill al detectar el hueco de procedencia

- **R7** — `AutoEmbedBackfill` DEBE contar primero las observaciones **activas** sin vector de la procedencia **actual**. Si el conteo es **0**, NO DEBE lanzar ninguna goroutine ni hacer trabajo (no-op silencioso: es el caso común en cada arranque).
- **R8** — Si el conteo es > 0, DEBE lanzar `EmbedBackfill` **en background** usando `spawnBackground`, de modo que: no se lance si el engine ya está cerrado, quede rastreado por `bgWG`, y `Close()` lo espere antes de cerrar la base (sin use-after-close del `*sql.DB`).
- **R9** — El arranque del server NO DEBE bloquearse esperando el backfill (un daemon bajo systemd tiene timeout de arranque; una tabla grande tardaría minutos).
- **R10** — DEBE logear el **inicio** (cuántas observaciones va a re-embeber) y el **fin** (resultado), para que la degradación temporal del recall semántico durante la ventana sea **visible**, no silenciosa.
- **R11** — Si no hay embedder nombrado (`vectorModelID == ""`), DEBE ser no-op (no hay semántica que backfillear).
- **R12** — DEBE preservarse la idempotencia: tras un backfill completo, un arranque posterior encuentra 0 pendientes (R7 ⇒ no-op).

**Escenario M3.a** — *Given* observaciones guardadas con un `model_id` viejo y un embedder nuevo activo, *When* arranca el engine y se llama `AutoEmbedBackfill`, *Then* las observaciones quedan re-embebidas con la procedencia nueva.

**Escenario M3.b** — *Given* todas las observaciones ya tienen vector de la procedencia actual, *When* se llama `AutoEmbedBackfill`, *Then* es un no-op (no re-embebe nada).

**Escenario M3.c** — *Given* un engine ya cerrado, *When* se llama `AutoEmbedBackfill`, *Then* no lanza trabajo (no hay use-after-close).

## No-objetivos (verificables)

- NO se toca el algoritmo de embedding, el tokenizer ni el formato de la tabla: los **vectores** que produce una tabla dada son bit-idénticos a los de antes; sólo cambia **cómo se la nombra**.
- NO se implementa dedup semántico (M1+Q4): este cambio es su precondición.
- NINGÚN vector se borra: los de procedencia vieja quedan **excluidos** por el contrato F2.2 (invisibles) y luego **sobrescritos** por el backfill (`INSERT OR REPLACE`).
