# Design — registros-inmutables-y-doble-aviso

## D1 — La banda usa `relevantPair`: la MISMA función que decide la cola

```go
// band.go, dentro del loop:
lex := Similarity(src.content, c.content)
if relevantPair(lex, &cos, opts) {
    continue          // ya lo va a ver por la cola. No se avisa dos veces.
}
if cos < opts.BandFloor || cos >= opts.CosineFloor {
    continue
}
```

**Rationale.** El bug no fue un umbral mal puesto: fue **describir una intención con una proxy**. La intención era *«no muestres lo que la cola ya muestra»*; lo escrito fue *«no muestres lo que supera el piso de coseno»*. Son distintas **porque a la cola se entra por dos puertas** — y el par que rompió esto entró por la **léxica**.

La forma de que no vuelva a divergir es que la banda **no reimplemente el criterio: que llame al mismo**. Si mañana `relevantPair` gana una tercera puerta, la banda la hereda **gratis**. Una condición duplicada es una condición que **va a divergir**.

**Descartado — consultar `observation_relations` para ver si el par ya existe.** Funciona, pero mete una consulta a la base para responder algo que es **puramente una decisión de criterio**, y ata la banda al **estado persistido** (¿y si la relación se resolvió y ya no está `pending`?). El criterio es la fuente de verdad, no la tabla.

## D2 — `historicalRecord`: la clase de artefacto, no el contenido

```go
// Un REGISTRO HISTÓRICO es lo que PASÓ (un commit) o lo que se ACORDÓ (un contrato SDD).
// No se puede des-hacer: nada de otra clase lo puede reemplazar.
func historicalRecord(topicKey string) bool {
    return isCommit(topicKey) || isSDD(topicKey)
}

// sameKind: dos artefactos de la MISMA naturaleza. Entre ellos el parecido SÍ puede
// significar redundancia (el gemelo del squash), así que se siguen comparando.
func sameKind(a, b string) bool {
    return (isCommit(a) && isCommit(b)) || (isSDD(a) && isSDD(b))
}
```

**Rationale.** Igual que en #203, la decisión se toma con el `topic_key` y **sin mirar el contenido**. La pregunta no es *«¿estos textos se parecen?»* (se parecen, y eso es correcto), sino *«¿el veredicto que esta relación habilita significa algo?»*.

## D3 — La guarda es ASIMÉTRICA, y va en el MISMO choke point

```go
func complementaryPair(a, b obsRow) bool {   // a = source (la recién guardada), b = target
    ...G1 y G2 de #203...

    // G3 — un REGISTRO HISTÓRICO no puede ser el DESTINO de otra clase. El único veredicto
    // que esta relación habilitaría es "esto reemplaza al commit / al spec", y eso NO SIGNIFICA
    // NADA: no se puede des-hacer lo que pasó. Pedir un juicio ya decidido de antemano es ruido.
    //
    // ASIMÉTRICA A PROPÓSITO: al revés SÍ importa. Un commit "feat: migrar de X a Y" SÍ puede
    // volver obsoleta una nota que decía "usamos X" — el commit es EVIDENCIA de que envejeció.
    // Por eso se mira sólo el TARGET (b), y se exceptúa la misma clase.
    if historicalRecord(b.topicKey) && !sameKind(a.topicKey, b.topicKey) {
        return true
    }
    return false
}
```

**Rationale del choke point.** `complementaryPair` ya es el **único** lugar donde se decide si un par es siquiera **comparable**, y lo llaman **los dos** caminos (la cola en `DetectRelations` y la banda en `BandNeighbors`). Meter G3 ahí la hace valer en **ambos** sin cablearla dos veces — y sin poder olvidarse de uno.

**Rationale de la asimetría.** Es lo que separa una **regla** de un **martillo**. Un martillo diría *«commit y nota no se comparan»* y perdería el caso valioso: **el commit como evidencia de que una nota quedó vieja**. La dirección **importa**, y el código lo dice explícitamente mirando `b` (el target) y no `a`.

**Nota sobre `complementaryPair` y su nombre.** Ya no es sólo *«complementarios»*: ahora también cubre *«el veredicto no significaría nada»*. El nombre queda porque ambas responden **la misma pregunta** — *«¿este par es siquiera comparable?»* —, que es la pregunta que la función existe para contestar.

## D4 — Lo que NO se hace

- **No se toca `relevantPair`, ni los umbrales, ni el AND-gate, ni `band_floor`.** El bug no estaba en los números.
- **No se oculta nada.** Las tres guardas hacen `continue`: **evitan crear** una relación. El peor caso sigue siendo *una relación de menos en la cola*, jamás *una observación de menos en el recall*.
- **No se limpia retroactivamente.** Las 8 pendientes ya creadas se resuelven a mano como `related` — que es, literalmente, el único veredicto que podían tener.
