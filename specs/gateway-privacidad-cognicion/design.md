# Diseño — Gateway de privacidad para la cognición (F1)

## Dónde se enchufa, y por qué ahí

`cognition.NewProvider` es el **único** constructor del pilar (`cmd/musubi/embed.go:201` es su único
caller, y de ahí sale al server MCP por `WithCognition`). Envolver ahí significa que **todo** motor
real nace ya envuelto — el de hoy y el que se agregue mañana. Envolver en el caller, en cambio,
dejaría la puerta abierta a que un segundo caller la esquive sin querer.

```
cmd/musubi/embed.go  resolveCognition()
        │
        ▼
cognition.NewProvider(cfg)
        │   ┌──────────────────────────────┐
        └──►│ guarded{ inner: OpenAICompat }│──► HTTP ──► LLM
            └──────────────────────────────┘
                   scrub ▲        ▼ restore
```

`NoopProvider` **no se envuelve**: sin motor no hay frontera que cuidar, y así R6 (bit-identidad) es
cierto por construcción, no por cuidado.

## Reusar la detección, agregar sólo la reversibilidad

`internal/redact.Redact(text) (string, []Finding)` ya resuelve el problema difícil —qué es un
secreto— y está auditado. Devuelve los hallazgos **ya ordenados, deduplicados y sin solapamientos**,
con offsets sobre el texto original.

`internal/privacy` **no re-implementa detección**. Llama a `redact.Redact`, **descarta** el texto
redactado (irreversible, no sirve acá) y se queda con los `[]Finding` para hacer su propia
sustitución reversible. Una sola fuente de verdad sobre qué es un secreto: si mañana `redact` aprende
un patrón nuevo, el gateway lo hereda gratis.

## El mapeo reversible

```go
type Session struct {
    byValue map[string]string  // secreto → marcador   (R3: estabilidad)
    byToken map[string]string  // marcador → secreto   (R2: sólo lo emitido)
    seen    []string           // textos ya scrubbeados (R5: evitar colisión)
    n       int
    finds   []redact.Finding
}
```

- `Scrub(text)` recorre los hallazgos de atrás para adelante (así los offsets no se corren) y
  reemplaza cada uno por su marcador.
- `Restore(text)` reemplaza **sólo** las claves de `byToken`. Lo que no esté en ese mapa se devuelve
  tal cual → **R2**.

### Forma del marcador

```
[[MSB:<tipo>:<n>]]        p.ej.  [[MSB:ai-provider-key:1]]
```

ASCII puro y sin caracteres que un modelo tienda a reformatear. Lleva el **tipo** adentro a
propósito: le da al modelo el contexto que necesita para razonar ("acá había una clave de API") sin
darle el valor.

### Cómo se garantiza R5 sin escapes

Al acuñar un marcador se verifica que **no aparezca ya** en ninguno de los textos vistos; si
apareciera, se incrementa el contador y se prueba de nuevo. Es determinista, termina siempre (el
espacio de índices es infinito y el texto finito), y evita toda la fragilidad de escapar y
desescapar.

Como consecuencia, un `[[MSB:token:5]]` que venga en la entrada **jamás** estará en `byToken`, así
que `Restore` no lo toca. R5 y R2 se sostienen mutuamente.

### Por qué de atrás para adelante

Sustituir de izquierda a derecha invalida los offsets de los hallazgos siguientes en cuanto el
marcador tiene distinto largo que el secreto. Recorrer en reversa mantiene válidos todos los offsets
que faltan procesar. Es la diferencia entre un round-trip exacto y uno que se desalinea con dos
secretos en la misma línea.

## El decorador

```go
func (g guarded) Ask(ctx context.Context, system, user string) (string, error) {
    sess := privacy.NewSession()
    s := sess.Scrub(system)
    u := sess.Scrub(user)

    if g.mode == ModeRefuse && sess.Count() > 0 {
        return "", ErrSecretsBlocked   // R4: no se envía nada
    }
    ans, err := g.inner.Ask(ctx, s, u)
    if err != nil {
        return "", err
    }
    return sess.Restore(ans), nil
}
```

**Una sesión por llamada.** `system` y `user` comparten el mapeo (el mismo secreto en los dos recibe
el mismo marcador → R3), y el mapeo muere con la llamada: no hay estado global ni fuga entre
llamadas concurrentes. Por eso `Session` **no necesita mutex**: no se comparte.

## Decisiones y sus alternativas descartadas

| Decisión | Por qué | Qué se descartó |
|---|---|---|
| Gateway **encendido por defecto** | Es una guarda de seguridad; el default seguro es protegido. No rompe bit-identidad porque sólo aplica cuando la cognición ya está encendida (que es opt-in) | Default `off` "para no cambiar comportamiento": dejaría fugas por omisión |
| Reusar `redact` | Una sola fuente de verdad; hereda auditorías futuras | Un detector propio: duplicaría reglas y divergiría |
| Marcador con el tipo adentro | El modelo razona mejor sabiendo *qué* se tapó | Marcador opaco `[[MSB:1]]`: pierde contexto útil sin ganar seguridad |
| Colisión por reintento | Determinista y simple | Escapar/desescapar: más código y más formas de romperse |
| `mode` desconocido = error | Una config mal escrita no puede terminar sin protección | Caer a un default: falla abierta silenciosa |

## Modo de falla, dicho de frente

**Los embeddings son una segunda salida, y F1 no la cubre.** Verificado en el código:
`internal/embedding/factory.go` ofrece `ollama` y `openai`/`openai-compatible`, que mandan el texto
a un endpoint. Si ese endpoint es remoto, el texto sale sin pasar por este portero. Queda para su
propia fase: el contrato de un embedder es distinto (devuelve un vector, no texto, así que **no hay
nada que rehidratar** — ahí la política correcta es `refuse` o un embedder local, no `scrub`).

Mientras tanto hay una segunda capa que sí cubre el destino final: `redact.Redact` ya se aplica en
el camino de guardado y de sync (`internal/mcp/methods.go:1705`, `internal/memory/inboundsync.go:94`,
`internal/memory/operations.go:316`, `internal/memory/scope.go:124`). Si una respuesta rehidratada
terminara guardándose, el secreto se tapa antes de llegar a la memoria compartida.

El gateway protege contra **fuga de credenciales por forma**. No protege contra:

- **Datos sensibles sin forma de secreto** (el nombre de un cliente, una decisión de negocio). Eso es
  juicio semántico y no es model-free; queda para la política de router (F2), que decide *a qué tier*
  puede ir cada cosa.
- **Un secreto que `redact` no conoce.** El catch-all de entropía cubre lo aleatorio y el de hex lo
  hexadecimal, pero una contraseña corta y de baja entropía fuera de un connection string no se
  detecta. Es una limitación heredada y consciente del detector.

Se documenta acá para que nadie lea "gateway de privacidad" y asuma más de lo que hace.
