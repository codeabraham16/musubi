# Spec — Gateway de privacidad para la cognición (F1)

Contrato observable. Cada invariante está numerado y tiene una prueba que **sabe fallar**: si se
rompe la propiedad, el test se pone rojo. Un invariante sin test que lo pueda tumbar es decoración.

---

## Alcance

Toda salida de texto desde Musubi hacia un motor de cognición externo, es decir **toda llamada a
`cognition.Provider.Ask`**. Hoy eso son dos seams reales:

| Seam | Dónde |
|---|---|
| `musubi_ask` (cognición a-demanda) | `internal/mcp/methods_cognition.go:117` |
| Juez de pertinencia en el recall | `internal/mcp/methods_cognition.go:208` |

Fuera de alcance en F1 (queda anotado, no silenciado): los proveedores de **embeddings**
(`internal/embedding`) también pueden mandar texto afuera. Se trata en su propia fase para no
mezclar dos superficies con contratos distintos.

---

## Invariantes

### R0 — Ningún secreto detectado cruza la frontera *(el invariante fundamental)*

Para todo texto `T`, si `redact.Redact(T)` reporta un hallazgo en `[s,e)`, entonces el texto que
recibe el `Provider` interno **no contiene** la subcadena `T[s:e)`.

> Es la razón de existir de todo esto. Si sólo se pudiera verificar uno, es este.

### R1 — Reversibilidad exacta

Para todo texto `T`: `Restore(Scrub(T)) == T`.

Sin pérdida, sin normalizar espacios, sin reordenar. Byte a byte.

### R2 — Restore no fabrica

`Restore` sustituye **únicamente** marcadores que esa misma sesión emitió. Un marcador inventado por
el modelo —o traído en el texto de entrada— se devuelve **intacto**: no se sustituye por otro
secreto, no rompe, no entra en pánico.

> Sin esto, un modelo malicioso podría escribir `[[MSB:token:1]]` en su respuesta para hacer aparecer
> un secreto que nunca vio. Con R2 sólo puede recuperar lo que ya estaba en *su propio* prompt.

### R3 — Estabilidad e inyectividad del mapeo

Dentro de una sesión:
- el **mismo** secreto recibe **siempre** el mismo marcador (el modelo puede razonar sobre "el mismo
  valor" apareciendo dos veces);
- secretos **distintos** reciben marcadores **distintos**.

### R4 — Falla cerrado

Si el gateway no puede garantizar R0 por cualquier motivo, **no se envía nada** al motor y el caller
recibe un error explícito. Nunca se degrada a "mando el texto crudo por las dudas".

Dos caminos, dos errores distinguibles:

| Situación | Error | Por qué separado |
|---|---|---|
| Modo `refuse` y había secretos | `ErrSecretsBlocked` | Es una decisión de política: reintentar contra el mismo motor no sirve |
| El portero falló o entró en pánico | `ErrGatewayFailed` | Es una falla técnica, y el caller puede degradar a model-free |

El pánico se atrapa con `recover`: este código corre dentro del daemon MCP, y dejarlo propagar
tumbaría el proceso entero. Atrapado, el peor caso es un error que el caller sabe manejar.

### R5 — El marcador no colisiona

Si el texto de entrada ya contiene algo con la forma de un marcador, el gateway **no lo pisa ni se
confunde**: los marcadores que emite se eligen de modo que no aparezcan en la entrada.

> Es el borde que separa un round-trip que anda "casi siempre" de uno que anda siempre.

### R6 — Bit-identidad del camino model-free

Con `cognition.provider` vacío o `none`, el comportamiento es **idéntico** al de antes de este
cambio: `NoopProvider` no se envuelve y no se paga ni un ciclo.

### R7 — El modo `off` es explícito y ruidoso

El gateway está **encendido por defecto** cuando hay un motor real. Apagarlo exige escribirlo en la
config, y deja un aviso en el log **y un check rojo en `musubi doctor`**. No hay forma de quedarse
sin portero por accidente ni por omisión.

Un aviso que sólo vive en el log de arranque de un daemon no lo lee nadie: por eso el estado del
portero es parte del diagnóstico (`musubi doctor --check cognition_gateway`), que es donde alguien
sí mira.

---

## Configuración

```yaml
cognition:
  provider: openai-compat
  gateway:
    mode: scrub    # scrub (default) | refuse | off
```

| Modo | Comportamiento |
|---|---|
| `scrub` | **default.** Tapa los secretos con marcadores reversibles, envía, rehidrata la respuesta |
| `refuse` | Si detecta un secreto, **no envía**: devuelve error. Para proveedores en los que no se confía nada |
| `off` | Sin portero. Requiere escribirlo explícitamente y avisa por log |

Un `mode` desconocido **no** cae a un default silencioso: `NewProvider` devuelve error y el pilar
entero queda apagado (model-free). Es falla-cerrado —sin motor no hay frontera que cruzar— y se ve
tanto en el log de arranque como en el doctor. Una config mal escrita no puede terminar en "sin
protección".

---

## Criterios de aceptación

1. Los 7 invariantes con test propio, y cada test verificado **fallando** al sabotear la
   implementación (un test que nunca se vio en rojo no prueba nada).
2. `go build ./...`, `go vet ./...` y `go test ./...` en verde.
3. Cero cambios de comportamiento con el pilar apagado (R6 cubierto por test).
4. Test adversarial: secretos pegados, superpuestos, repetidos, en los bordes del texto, texto vacío,
   texto que ya contiene marcadores, y respuesta del modelo con marcadores inventados.
