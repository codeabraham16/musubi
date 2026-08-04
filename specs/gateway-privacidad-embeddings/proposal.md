# Propuesta — Portero de privacidad para los embeddings (F1.5)

## De dónde sale

F1 puso un portero entre la memoria y el motor de cognición, y dejó anotado un hallazgo abierto: los
proveedores de **embeddings** son una segunda superficie de salida que esa fase no cubría.

Al ir a cerrarlo, el hueco resultó **más grande de lo que la propia F1 decía**.

## Lo que se creía y lo que es

F1 afirmó que `redact.Redact` mitigaba parcialmente esta superficie, porque se aplica en cuatro
puntos del camino de guardado y sync. Verificado en el código, **los cuatro están condicionados**:

| Punto | Condición real |
|---|---|
| `internal/mcp/methods.go:1701` (`redactIfForced`) | sólo si `forceRedact` — que es el flag del **cerebro central**, no de un workspace normal |
| `internal/memory/operations.go:315` | sólo si `scope == shared` |
| `internal/memory/scope.go:124` | sólo al **promover** una observación a `shared` |
| `internal/memory/inboundsync.go:94` | sólo en el ingest del central |

O sea: protegen **lo que se guarda como compartido y lo que viaja al central**. No protegen nada de
lo que sale hacia el embedder. En un workspace local con un embedder remoto, **todo sale crudo**:

- `save_observation` → `s.embedder.Embed(embCtx, content)` con `content` sin redactar (`redactIfForced`
  es no-op sin `forceRedact`).
- `search_semantic`, `recall` y `musubi_ask` → embeben la **query del usuario tal cual**. Ninguna
  ruta redacta queries, en ninguna configuración. Una pregunta como *"¿qué era el token msb_…?"*
  sale entera.
- La captura automática (C4) y el guardado de documentos SDD → texto crudo.

La afirmación de F1 era cómoda y falsa. Queda corregida en su propia spec.

## Qué se propone

El mismo principio que en F1, adaptado a un contrato distinto: **envolver el Provider dentro del
único constructor** (`embedding.NewProvider`), de modo que todo embedder que hable por un socket
nazca protegido.

La diferencia importante con la cognición: un embedder devuelve un **vector**, no texto. No hay
respuesta que rehidratar. Eso no lo vuelve imposible de proteger — lo vuelve **más simple**: alcanza
con la redacción de una sola vía que `internal/redact` ya hace, sin necesidad del mapeo reversible de
`internal/privacy`.

(F1 dijo que acá "la política correcta es `refuse` o embedder local, no `scrub`". También estaba mal:
`scrub` es exactamente lo que corresponde, y es el default.)

## Qué NO se propone

- **Tocar `static` ni `none`.** No mandan texto a ningún lado: no hay frontera que cuidar, y el
  camino model-free queda bit-idéntico.
- **Rehidratar nada.** No aplica.
- **Re-embeber lo ya guardado.** Los vectores viejos se calcularon sobre texto crudo; sólo difieren
  para textos que contienen secretos. Se documenta y queda para `musubi embed backfill`.
