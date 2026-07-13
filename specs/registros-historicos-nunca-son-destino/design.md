# Design — registros-historicos-nunca-son-destino

## D1 — La función colapsa a la regla

```go
func complementaryPair(_, b obsRow) bool {
    return historicalRecord(b.topicKey)
}
```

Y `sameKind` desaparece (queda sin usos).

**Rationale — por qué esto NO es un atajo.** G1, G2 y G3 se descubrieron por separado, en tres PRs, cada una a partir de un ruido distinto que apareció dogfooding. Al quitar la excepción `!sameKind`, las dos primeras quedan **subsumidas por construcción**:

| guarda | target | ¿histórico? |
|---|---|---|
| G1 — hermanos del mismo cambio SDD | `sdd/<cambio>/<fase>` | **sí** |
| G2 — el evento vs el contrato | commit o `sdd/…` | **sí** |
| G3 — registro histórico como destino | commit o `sdd/…` | **sí** (por definición) |

No es que se borren tres reglas: es que **eran la misma regla vista desde tres ángulos**, y recién ahora se la ve entera. El código pasa a decir lo que el principio dice.

**El riesgo de colapsar, y su red.** Si mañana alguien angosta `historicalRecord`, G1 y G2 desaparecerían **en silencio**. Por eso los tests que las pinean **no se tocan**: siguen verdes sin una línea de cambio, y son a la vez (a) la prueba de que el colapso preserva el comportamiento y (b) la red que lo mantiene preservado.

## D5 — Los tests que pinean la excepción, y `DetectOnly`

Cuatro tests exigen **lo contrario** de lo que este PR establece (que commit↔commit y sdd↔sdd sí se juzguen). Se reescriben. Lo que protegían de verdad —que la guarda no se vuelva un **martillo**— queda cubierto por los dos tests con **destino = nota**, que siguen verdes sin tocarse.

**`DetectOnly` NO queda muerto, pero sus tests apuntaban a un camino ya bloqueado.** M4 nació para los commits: en la captura automática todos caen en el balde `git-commit`, y sin `DetectOnly` dos mensajes parecidos se auto-ocultaban. Con la guarda estructural **entre dos commits ya no nace ninguna relación**, así que el auto-supersede es inalcanzable *antes* de que `DetectOnly` opine. Los commits quedan protegidos **dos veces**, y la garantía fuerte pasa a ser la estructural.

Pero `DetectOnly` sigue siendo **load-bearing** en el otro lugar donde se usa: la telemetría guarda los error→fix en el balde `error-fix`, que **no** es un registro histórico. Ahí el auto-supersede sigue siendo posible y `DetectOnly` es **lo único** que impide que un arreglo nuevo tape a uno viejo por parecerse.

⇒ Los tests de M4 se re-apuntan a `error-fix`. **Un test que sólo cubre un camino ya bloqueado río arriba no prueba nada** — habría quedado verde para siempre sin custodiar nada.

Además, el gemelo (`...WouldAutoSupersede`) tenía un `t.Skip` para el caso en que los umbrales dejaran de auto-superseder: **un skip que pasa en verde es un test que dejó de demostrar su premisa en silencio.** Pasa a `t.Fatal`.

**Descartado — dejar las tres guardas escritas "por claridad".** Tres `if` de los cuales dos **no pueden ejecutarse nunca** no son claridad: son código muerto que miente sobre por qué funciona. El lector concluiría que hacen falta.

## D2 — La asimetría es la única sutileza, y se conserva sola

Se mira **sólo `b`**. El parámetro `a` pasa a ser `_`, y eso es **la firma diciendo la verdad**: el source es irrelevante.

El caso que NO se puede romper es `commit → nota`: un commit `feat: migrar de X a Y` **sí** vuelve obsoleta la nota `usamos X`. Ahí el target es la **nota**, que no es histórica ⇒ la relación nace. Sigue funcionando **sin una línea de código extra**, y S.e lo pinea.

## D3 — Nada retroactivo

Las 169 relaciones existentes **no se tocan**. La guarda actúa cuando **nace** una relación; las que ya nacieron, ya nacieron — y varias tienen veredicto humano. Reescribir el pasado para que coincida con la regla nueva sería, justamente, **tachar el libro mayor** — el pecado que este PR viene a impedir.

## D4 — La medición

Un programa contra la memoria real que corre las guardas de HOY y las de la PROPUESTA sobre las 169 relaciones y compara. El resultado que valida la propuesta:

```
bloqueadas por las guardas de HOY: 102
bloqueadas por la PROPUESTA:       136
  => encoladas de más HOY:          34  (20% de la cola)
veredictos SUSTANTIVOS que romperíamos: 0
```

**Si ese último número no fuera 0, la propuesta estaría mal y se caería.** No es una formalidad: es la condición de aceptación.

## Lo que NO se toca

- Scoring, umbrales, auto-resolve, banda ciega, MMR.
- Las relaciones existentes.
- La capacidad de un registro histórico de ser **source**.
