# SDD spec — renaissance-el-presupuesto-protege-el-material

## Contrato

### El corpus del brief
`designBrief.Corpus` pasa de `[]searchHit` (id + gist de ~90 chars + costo de hidratar) a
`[]patronItem`:

```go
type patronItem struct {
	ID, Topic, Fuente, Texto string
	Similarity               float32 // sólo por el camino semántico
	Recortado                bool    // el texto no vino entero
	FullTokens               int     // lo que mide entero, si se recortó
}
```

Misma forma que `metodoItem`: los dos bloques de material se leen igual y llevan su procedencia.

### El presupuesto
`designBriefBudget` 2.600 → 6.000 tokens. Sigue siendo **tope duro**.
`designPatronItemMax` = 1.800 chars por tarjeta de corpus (nuevo).
`designPisoCorpus` = 5, `designPisoBloque` = 3 (método).

### El orden en que se cede
1. método por encima de su piso
2. corpus por encima de su piso
3. **la marca** (recorte con aviso ruidoso)
4. último recurso: los pisos se rompen, primero método y después corpus

### Los umbrales
`m4_tokens_p50_max` 2.500 → 3.400 · `m4_tokens_max` 2.600 → 6.000 (= `designBriefBudget`) ·
`m5_fraccion_variable_min` 0,13 → **0,40**. Aflojar un umbral es un cambio deliberado y va con su
justificación adentro del archivo, que es donde se lee en el diff.

---

## Invariantes

Cada uno con el sabotaje que tiene que verlo fallar, y el sabotaje ataca **el invariante declarado**.

### I-MAT2 · el corpus viaja con su texto completo, no con un titular
El patrón servido contiene el cuerpo de la tarjeta, no su cabeza. `corpus_note` no manda a expandir
lo que ya está servido.

**Sabotaje:** volver a cortar el texto a 90 chars en `comoPatronItem` ⇒ el cierre del patrón deja de
estar en el brief. *Verificado en rojo:* `el cuerpo del patrón no llegó al brief; se sirvieron 117
chars de 505`.

### I-MAT3 · una tarjeta gorda no se lleva el brief puesto
Un artículo `ingested/*` entra recortado a `designPatronItemMax`, lo declara en `recortado`, y dice
cuánto mide entero en `full_tokens`.

**Sabotaje:** subir `designPatronItemMax` a 999999 ⇒ *verificado en rojo*, y el modo de falla resultó
peor de lo previsto: el corpus sale **vacío**. Sin tope por tarjeta el artículo no desborda el brief
—la escalera lo impide— sino que se lleva puesto al material entero para hacerse lugar.

### I-MAT4 · el recorte no parte un carácter en dos
El texto servido es UTF-8 válido y sin caracteres de reemplazo.

**Sabotaje:** volver a `txt[:max]` ⇒ *verificado en rojo*: `el recorte dejó un carácter de reemplazo`.

⚠️ **El fixture verifica su propia premisa.** Las dos primeras versiones elegían el largo del relleno
a mano para que el corte cayera adentro de un carácter de dos bytes, y las dos veces cayó en el borde:
**el sabotaje pasaba verde y el test no probaba nada**. Ahora el relleno se ajusta hasta que el byte
del corte no sea principio de carácter, y si no se logra el test falla en vez de pasar.

### I-PRE5 · el material tiene piso duro: la marca cede antes de romperlo
Con una marca que sola no entra en el presupuesto, el brief conserva `designPisoCorpus` patrones y
`designPisoBloque` tarjetas de método; la marca sale recortada, con su aviso, y declarada en
`truncated`.

**Sabotaje:** reponer la escalera vieja (`len(b.Method) > 0` / `len(b.Corpus) > 0` antes de tocar la
marca) ⇒ *verificado en rojo*: `el corpus cayó a 0, bajo su piso duro de 5 — la marca se comió el
material`.

### I-PRE6 · el piso del corpus es más alto que el del método
Cuando falta lugar sobrevive lo específico, no lo universal.

**Sabotaje:** igualar los dos pisos ⇒ *verificado en rojo*.

### I-BANCO7 · la compuerta es la especificidad, no el tamaño
Reponer prosa constante grande pone el banco en rojo por M5 aunque el brief entre en el tope.

**Sabotaje:** inyectar 120 líneas de criterio universal en `designPrinciples` ⇒ *verificado en rojo*:
M5 **0,45 → 0,243** contra un umbral de 0,40, con `m4_tokens_max` en **5.946, bajo el techo de
6.000**. El tamaño solo no lo habría atrapado.

### I-BANCO8 · M3 no puede subir por servir más texto
`TocaLosEjes` mira siempre los primeros `cabezaM3` (90) bytes de cada patrón, que es lo que el motor
servía como gist hasta este cambio.

**Sabotaje:** buscar el eje en el texto completo ⇒ M3 sube sin que el material haya mejorado, y el
antes/después del cambio deja de ser comparable. Es la trampa del proxy: la métrica pasaría a medir
el tamaño del bloque disfrazado de precisión.

### I-PRE2b · el test del presupuesto no puede quedar vacuo
Cada caso del test de presupuesto declara de antemano si espera recorte.

**Sabotaje:** un test que sólo verifique «si hubo recorte, se declaró» pasa verde el día que el motor
deje de recortar nunca **y** el día que deje de servir nada, porque la premisa se evapora y la
aserción no se ejecuta. Es lo que pasó al subir el tope: dos de tres casos dejaron de recortar y la
mitad del test se apagó en silencio.

---

## Fuera de alcance

- La recuperación: mismo pool, mismo piso de similitud, misma selección.
- El checklist anti-genérico. Es conocimiento por escribir, y va sobre este presupuesto.
- Mover M3/M1. Se miden con la sonda después de desplegar; que **no** se muevan es un resultado
  informativo, no un fracaso de esta fase.
