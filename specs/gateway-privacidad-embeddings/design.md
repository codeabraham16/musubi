# Diseño — Portero de privacidad para los embeddings (F1.5)

## Dónde se enchufa

Igual que en F1, y por la misma razón: `embedding.NewProvider` es el **único** constructor del
embedder, así que envolver ahí hace que todo proveedor con red nazca protegido — el de hoy y el que
se agregue mañana.

```
resolveEmbedder()
        │
        ▼
embedding.NewProvider(cfg)
        │   ┌────────────────────────────┐
        └──►│ guarded{ inner: OpenAI }   │──► HTTP ──► API
            └────────────────────────────┘
                    redact ▲   (no hay vuelta: devuelve un vector)
```

`newBaseProvider` queda separado y privado, igual que en cognición: no existe una versión sin portero
que un caller futuro pueda tomar por error.

## Por qué acá alcanza con `redact` y no hace falta `privacy`

`internal/privacy` existe porque la cognición necesita **deshacer** el tapado en la respuesta. Un
embedder devuelve un `[]float32`: no hay respuesta de texto, no hay nada que rehidratar.

Entonces alcanza con la redacción de una sola vía que `internal/redact` ya hace, que además:

- **es determinista** (E1): el mismo texto da el mismo `[REDACTED:<tipo>]`, y por lo tanto el mismo
  vector;
- **colapsa secretos del mismo tipo** en el mismo token. Para un vector eso es una virtud, no una
  pérdida: nadie quiere recuperar semánticamente *por el valor* de una clave.

F1 había anotado que acá "la política correcta es `refuse` o embedder local, no `scrub`". Es al
revés: la ausencia de vuelta hace el problema **más simple**, no imposible. `scrub` es el default.

## Qué se envuelve y qué no

| Provider | ¿Manda texto por un socket? | ¿Se envuelve? |
|---|---|---|
| `""` / `none` | no (falla explícito) | no |
| `static` | no — tabla model2vec en proceso | **no** |
| `ollama` | sí (HTTP) | sí |
| `openai` / `openai-compatible` | sí (HTTPS) | sí |

**La regla es "¿abre un socket?", no "¿el host es remoto?".** Un `ollama` en `localhost` también se
envuelve, a propósito:

- `base_url` es config, y la config se cambia. Una regla que depende de parsear una URL para decidir
  si hay riesgo **va a estar mal el día que alguien mueva el endpoint** — y va a estar mal en
  silencio, que es la peor forma.
- El costo de envolver de más es casi nulo acá: sólo cambia el vector de textos que **contienen un
  secreto detectado**, que es justamente el texto que uno no quiere poder buscar por su valor.

Es la misma decisión estructural que exime al `NoopProvider` en F1: se decide por lo que el tipo
*hace*, no por lo que su configuración *dice*.

## Determinismo y coherencia índice↔consulta

E1 y E2 son los invariantes propios de esta fase (no existían en F1, donde cada llamada era
independiente). Se sostienen por construcción:

- **E1** porque `redact.Redact` es una función pura y determinista;
- **E2** porque el portero está en el **constructor**, así que las seis rutas —las tres que indexan y
  las tres que consultan— usan literalmente el mismo objeto. No hay forma de que una tape y otra no.

Es la ventaja concreta de envolver en la fábrica en vez de parchear cada call site: E2 no es una
regla que haya que recordar, es una consecuencia.

## Una sola fuente de verdad sobre los modos (E6)

F1 dejó `normalizeGatewayMode` dentro de `internal/cognition`. Al necesitar los mismos tres modos
acá, copiarla sería garantizar la divergencia. Se **mueve a `internal/config`**
(`config.NormalizeGatewayMode`), que es el paquete que ya define `GatewayConfig` y del que los dos
pilares dependen. `internal/cognition` pasa a delegar.

`EmbeddingConfig` reusa el **mismo tipo** `GatewayConfig`: mismos modos, misma semántica, misma
validación.

## Modo de falla, dicho de frente

- **Vectores mixtos.** Lo ya indexado se calculó sobre texto crudo; lo nuevo, sobre texto tapado.
  Sólo difieren para textos que contienen un secreto detectado. Se corrige con
  `musubi embed backfill`. No se hace automático: recalcular todo el índice es una decisión del
  usuario, no un efecto colateral de actualizar.
- **`refuse` puede romper el guardado.** En `save_observation` un error de embedding aborta el save.
  Es fail-closed y deliberado; por eso `refuse` no es el default y está documentado en la spec.
- **Falsos positivos del detector.** El catch-all de entropía puede marcar como secreto una cadena
  legítima de alta entropía (un hash, un id largo). Tapada, su vector cambia y su recuperación
  semántica se degrada. Es una limitación **heredada** del detector, idéntica a la que ya afecta al
  camino de guardado compartido — no una nueva.
- **Lo que sigue sin cubrirse:** datos sensibles sin forma de secreto (el nombre de un cliente). Es
  juicio semántico, no model-free.
