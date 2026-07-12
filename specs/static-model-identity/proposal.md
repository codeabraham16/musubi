# Proposal — static-model-identity

## Intención

Corregir **N1** (auditoría del techo semántico, marcado *"fix de raíz"* y **precondición de cualquier re-destilación**) y su consecuencia obligada **M3**:

- **N1 — La identidad de la tabla estática es falsa.** `StaticProvider` construye su `model_id` como `"static:" + basename(dir)` (`internal/embedding/static.go:65`). Si re-destilás la tabla **in-place** (mismo directorio, vectores distintos), el `model_id` **NO cambia**. Los vectores viejos siguen pareciendo compatibles y el contrato de procedencia (F2.2) los deja entrar a la búsqueda, que los compara por coseno contra vectores de la tabla nueva. Resultado: **ranking corrupto en silencio**, sin ningún error ni aviso. El `model_id` debe identificar el **contenido** de la tabla, no el nombre de su carpeta.

- **M3 — Auto-backfill al cambiar de modelo.** N1 **solo** sería una regresión: al actualizar, el `model_id` nuevo (con checksum) invalida todos los vectores existentes → el contrato de procedencia los excluye → **la semántica se apaga** hasta que alguien corra `musubi embed backfill` a mano. Hoy el daemon sólo **avisa** del cambio (`WarnOnEmbedModelSwitch`) pero no lo **remedia**. Con M3, el arranque detecta el hueco y lo cierra solo.

Juntos: la identidad del modelo pasa a ser **real** y el cambio se **auto-sana**.

## Por qué ahora

Es la precondición del dedup semántico por coseno (M1+Q4), el próximo slice grande del track. Un dedup que compara vectores exige que la procedencia sea confiable: si dos vectores dicen ser del mismo modelo pero no lo son, el dedup borra o fusiona memoria por similitudes falsas. **N1 antes que cualquier cosa que confíe en el coseno.**

## Alcance

- **N1:** el `model_id` del `StaticProvider` pasa a `static:<basename>@<checksum>`, donde el checksum cubre el **contenido** de `model.safetensors` **y** de `tokenizer.json` (un cambio de tokenizer también cambia los vectores). `loadStaticTable` ya hace `os.ReadFile` de la tabla entera ⇒ el hash sale con **cero I/O extra**.
- **M3:** un `AutoEmbedBackfill` en el engine que, si hay observaciones activas sin vector de la procedencia actual, lanza `EmbedBackfill` **en background** vía el `spawnBackground` existente (rastreado por `bgWG`, no arranca si el engine está cerrado, `Close()` lo espera) y logea el resultado. Cableado en los 2 call-sites de arranque de `cmd/musubi/main.go`. Si no hay hueco, no lanza nada.
- Tests: el checksum cambia ante un cambio de tabla/tokenizer y es estable ante recargas; el auto-backfill cierra el hueco y es no-op cuando no hay nada que hacer.

## Fuera de alcance (explícito)

- El **dedup semántico** en sí (M1+Q4) — este cambio es su precondición, no el dedup.
- Cualquier cambio al algoritmo de embedding, al tokenizer o al formato de la tabla.
- Re-destilar o cambiar la tabla instalada (`potion-multilingual-128M` sigue igual).
- Retrieval-tuning (N2) y pesos tuneables (M7).

## Estrategia de rollback

- Revertir el PR restaura `model_id = "static:" + basename` y quita el auto-backfill. Sin migración de esquema, sin cambio de datos.
- **Migración hacia adelante (one-time, esperada y auto-sanada):** al actualizar, el `model_id` de la tabla instalada cambia ⇒ los vectores existentes quedan **excluidos** (invisibles, **no corruptos** — el contrato de procedencia ya los filtra) ⇒ el auto-backfill (M3) los re-embebe solo en el primer arranque. `EmbedBackfill` es idempotente y resumible: si el proceso muere a mitad, la corrida siguiente termina el resto.

## Riesgos

- **Costo de arranque:** hashear la tabla (~488MB en la multilingüe) suma ~0.2-0.5s. Se amortiza contra el `ReadFile` de esos mismos 488MB que ya se hacía; es la misma pasada de bytes, sin I/O extra.
- **El auto-backfill corre en background** mientras el server ya atiende: durante esa ventana el recall semántico devuelve menos (los vectores aún no re-embebidos están excluidos). Es degradación **temporal y honesta**, no corrupción — y es estrictamente mejor que el estado de hoy (apagado hasta intervención manual). Se logea inicio y fin.
- Un cambio de checksum **invalida los vectores de todos los repos con Musubi** (los 6) en su primer arranque tras el upgrade. Auto-sanado por M3, pero hay que propagarlo.
