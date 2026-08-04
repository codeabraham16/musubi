# Spec — Portero de privacidad para los embeddings (F1.5)

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## Alcance

Toda salida de texto hacia un proveedor de embeddings que hable por un socket, es decir **toda
llamada a `embedding.Provider.Embed`** cuando el provider es `ollama` u `openai`/`openai-compatible`.

Seams reales hoy (6, todos verificados en el código):

| Seam | Dónde | Qué texto |
|---|---|---|
| `musubi_save_observation` | `internal/mcp/methods.go:238` | contenido de la observación |
| `musubi_search_semantic` | `internal/mcp/methods.go:1185` | **query del usuario** |
| `musubi_recall` | `internal/mcp/methods.go:1726` | **query del usuario** |
| `musubi_ask` | `internal/mcp/methods_cognition.go:79` | **pregunta del usuario** |
| captura automática (C4) | `internal/mcp/methods.go:1853` (`embedIfEnabled`) | texto capturado |
| documentos SDD | `internal/mcp/methods_sdd.go:126` | contenido del documento |

---

## Invariantes

### E0 — Ningún secreto detectado cruza hacia el embedder *(el fundamental)*

Para todo texto `T`, si `redact.Redact(T)` reporta un hallazgo en `[s,e)`, entonces el texto que
recibe el `Provider` interno **no contiene** la subcadena `T[s:e)`.

Aplica **por igual a queries y a contenido**: las queries son la ruta que hoy no tiene ninguna
protección, en ninguna configuración.

### E1 — Determinismo

Para todo texto `T`, dos llamadas a `Embed(T)` producen el **mismo** texto tapado, y por lo tanto el
mismo vector.

> Sin esto el índice vectorial se vuelve inconsistente consigo mismo: el mismo contenido guardado dos
> veces daría vectores distintos, y la deduplicación por similitud dejaría de funcionar.

### E2 — Coherencia entre índice y consulta

El tapado se aplica en el **mismo lugar** para el texto que se indexa y para el texto que se
consulta. No existe una ruta que embeba crudo del lado del índice y tapado del lado de la query, ni
al revés.

> Es lo que garantiza que tapar no degrade la recuperación: si ambos lados ven la misma
> transformación, la distancia entre ellos se preserva.

### E3 — Falla cerrado

Si el portero no puede garantizar E0, **no se manda nada** al proveedor y el caller recibe un error.
Nunca se degrada a "mando el texto crudo por las dudas". El pánico se atrapa con `recover`: esto
corre dentro del daemon MCP.

| Situación | Error |
|---|---|
| Modo `refuse` y había secretos | `ErrSecretsBlocked` |
| El portero falló o entró en pánico | `ErrGatewayFailed` |

### E4 — Bit-identidad de los proveedores sin red

Con `provider` vacío, `none` o `static`, el comportamiento es **idéntico** al de antes de este
cambio: no se envuelve nada y no se paga ni un ciclo.

`static` es una tabla en proceso (model2vec/POTION): no abre un socket, así que no hay frontera. Es
el mismo razonamiento que exime al `NoopProvider` en F1, y por eso el invariante es estructural y no
una promesa.

### E5 — El modo `off` es explícito y ruidoso

El portero está **encendido por defecto** cuando hay un embedder con red. Apagarlo exige escribirlo
en la config, deja aviso en el log **y un check rojo en `musubi doctor`**.

Un `mode` desconocido **no** cae a un default silencioso: apaga la semántica entera (falla-cerrado —
sin embedder no hay frontera que cruzar).

### E6 — Una sola fuente de verdad sobre los modos

`scrub`/`refuse`/`off` significan lo mismo para la cognición y para los embeddings, y se validan con
el **mismo** código (`config.NormalizeGatewayMode`). Agregar un modo no puede dejar a uno de los dos
pilares desactualizado.

---

## Configuración

```yaml
embedding:
  provider: openai
  gateway:
    mode: scrub    # scrub (default) | refuse | off
```

| Modo | Comportamiento |
|---|---|
| `scrub` | **default.** Reemplaza cada secreto por `[REDACTED:<tipo>]` y embebe el texto tapado |
| `refuse` | Si detecta un secreto, **no embebe**: devuelve error |
| `off` | Sin portero. Requiere escribirlo y avisa por log y por doctor |

**Sobre `refuse`, dicho de frente:** en el seam de `save_observation` un error de embedding **aborta
el guardado entero**. Con `refuse`, guardar una observación que contiene un secreto falla. Es
fail-closed y deliberado, pero es un cambio de comportamiento — por eso `refuse` no es el default.
En los demás seams el error degrada con gracia (la captura automática devuelve `nil`, y el recall
cae a léxico).

---

## Criterios de aceptación

1. Los 7 invariantes con test propio, cada uno verificado **fallando** al sabotear la implementación.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run ./...` en verde.
3. Cero cambios de comportamiento con `none`/`static` (E4 cubierto por test).
4. Test adversarial: query con secreto (la ruta que hoy no tiene nada), texto vacío, texto sin
   secretos (no debe alterarse ni un byte), y el mismo texto dos veces (determinismo).
