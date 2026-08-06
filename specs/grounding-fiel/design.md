# Diseño — Grounding fiel para `musubi_ask`

## Dónde está el defecto

`internal/mcp/methods_cognition.go`, el armado del prompt:

```go
fmt.Fprintf(&b, "[%s] (%s%s)\n%s\n\n", it.ID, it.TopicKey, age, it.Gist)
```

`it.Gist`. Dos cosas de una: el contenido va truncado, y `it.Provenance` —que el recall ya calcula y
ya devuelve al caller— no se usa.

## La hidratación: reusar, pero no la función que ya existe

`GetObservationsBudgetCtx` (`internal/memory/expand.go`) ya hace exactamente el empaquetado que hace
falta: toma ids en orden, mete contenidos completos hasta que el siguiente no entra, garantiza el
primero truncándolo si hace falta, y acota por proyecto con `projectScopeFrom`.

**Pero no se puede llamar tal cual**, y este es el hallazgo que dobló el diseño: en su línea 100
llama a `bumpAccess`. Y `Recall` **ya llamó a `bumpAccess`** sobre esos mismos ids en su línea 297.
Llamarla desde `ask` contaría el acceso **dos veces** por pregunta.

No es cosmético. `recall.go:362` documenta la invariante N4 —«EL RANKER NO SE ALIMENTA DE SU PROPIA
SALIDA»— y el ranking usa frecuencia de acceso. Doblar el conteo sobre justo las memorias que más se
consultan es meterle al ranker una realimentación positiva silenciosa.

**Solución**: partir la función en dos.

```
hydrateByIDs(ctx, ids, budget)            ← el empaquetado puro, sin efectos
├── GetObservationsBudgetCtx(...)         ← = hydrateByIDs + bumpAccess   (memory_expand, sin cambios)
└── HydrateForGroundingCtx(...)           ← = hydrateByIDs                (ask: el acceso ya lo contó Recall)
```

El comportamiento de `memory_expand` queda bit-idéntico: sigue llamando a la misma función pública,
que sigue contando el acceso. Lo que cambia es que ahora existe una puerta que no lo cuenta, con el
nombre diciendo para qué es.

## El presupuesto

El `token_budget` de la tool cuenta **gists**, que pesan ~20 tokens. Contenidos completos pesan varias
veces más, así que hidratar todo lo que el recall selecciona no entra en ningún prompt.

Regla, una sola:

```
presupuesto de hidratación = min(token_budget_efectivo × 2, 16000)
```

Los que entran van con contenido completo; **los que no, se quedan en gist** — no desaparecen. Por eso
G2 se cumple: la lista de memorias es la misma, cambia cuánto se manda de cada una.

El factor 2 es deliberadamente conservador: da profundidad al tope del ranking y conserva la
amplitud abajo. Y escala con lo que pidió el caller, así que un `token_budget` chico sigue dando un
prompt chico — sin una perilla nueva que calibrar.

## Las dos cosas que se pierden si uno hidrata a lo bruto

Acá está el trabajo real del cambio, y ninguna de las dos se ve leyendo el prompt.

**1. La advertencia de ranciedad.** `markStaleOrigins` (`origins.go:264`) hace
`Items[i].Gist = staleWarning(s) + Items[i].Gist`. Vive **pegada al gist**, no en un campo aparte que
el prompt lea. Cambiar `it.Gist` por el contenido crudo la borra sin que nada se ponga rojo: el
prompt se ve perfecto y el modelo deja de saber que la nota puede estar vencida.

Se repone anteponiéndola al contenido hidratado. Eso obliga a exportar `staleWarning` →
`StaleWarning`, porque el prompt se arma en el paquete `mcp` y la advertencia se calcula en `memory`.
La alternativa —reconstruir el texto en `mcp`— sería dos formatos de advertencia divergiendo.

**2. El sello de procedencia.** No se pierde: nunca estuvo. `RecallItem.Provenance` ya viene poblado
y ya filtrado por `stampProvenance` (vacío cuando es `human`), así que alcanza con sumarlo a la
cabecera de cada bloque:

```
[<id>] (<topic> · <fecha> · <procedencia>)
<advertencia de rancio><contenido completo, o gist si no entró>
```

Con `human` la cabecera queda igual que hoy, así que el caso normal no cambia de forma.

## Por qué la cuarentena no se filtra (G5)

`HydrateForGroundingCtx` lee **por id**, sin el predicado canónico de visibilidad — igual que
`memory_expand`, y por la misma razón declarada en Q0b: exige conocer el id de antemano.

Acá los ids salen de `res.Items`, o sea del `Recall`, que **sí** aplica
`archived = 0 AND superseded_by IS NULL AND quarantined = 0`. La muralla se sostiene por
**procedencia de los ids**, no por filtrado en la hidratación.

Eso es una garantía más frágil que un `WHERE`, y por eso lleva test propio: si alguien mañana
alimenta la hidratación desde otra lista, nada más en el repo se pone rojo.

## Degradación (G6)

Si la hidratación devuelve error, se logea y se sigue con los gists. `ask` ya trata así al embedder
—si no puede embeber la pregunta, sigue con léxico—; la profundidad merece el mismo trato que la
semántica: es una mejora del resultado, no un requisito para responder.

## Riesgos, dichos de frente

- **Cruza más texto al motor externo.** Antes gists truncados, ahora contenido completo. El portero
  de privacidad lo cubre y quedó verificado en el cable, pero la superficie es objetivamente mayor y
  el que apague el portero (`gateway.mode: off`) se expone a más de lo que se exponía antes de este
  cambio.
- **Prompts más caros.** Más tokens por pregunta ⇒ más costo y más latencia. El tope de 16000 y el
  factor 2 lo acotan, pero es un costo real a cambio de una respuesta mejor fundamentada.
- **El tope es un número elegido, no medido.** 16000 sale de lo que entra cómodo en un contexto
  moderno, no de una medición de calidad de respuesta. Cuando haya un motor real enchufado va a
  poder calibrarse con datos; hasta entonces es un default defendible, no óptimo.
