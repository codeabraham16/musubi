# Design — gists-que-dejan-decidir

## D1 — `Gist` acumula oraciones hasta llenar el techo

```go
func Gist(content string, maxTokens int) string {
    ...
    lead := firstSentence(norm)
    if EstimateTokens(lead) >= maxTokens {
        return truncateToTokens(lead, maxTokens)   // <-- comportamiento de HOY, intacto
    }
    // Antes se devolvía `lead` acá y se ABANDONABAN los tokens que sobraban.
    out, rest := lead, strings.TrimSpace(norm[len(lead):])
    for rest != "" {
        s := firstSentence(rest)
        if EstimateTokens(out+" "+s) > maxTokens {
            break                                   // no entra COMPLETA ⇒ no se agrega
        }
        out += " " + s
        rest = strings.TrimSpace(rest[len(s):])
    }
    return out
}
```

**Rationale.** El fix es **aditivo por construcción**: la rama que ya llenaba el techo (`>= maxTokens`) devuelve **exactamente** lo de hoy. Sólo se agrega texto **donde antes se tiraba presupuesto**. Eso hace que el riesgo de regresión sea cero para el 76% de las memorias que no estaban afectadas.

**El corte es por TOKENS y por oración COMPLETA.** No se agrega media frase: **un gist cortado al medio también es un gist que no deja decidir** — parece que informa y no informa. Preferimos quedarnos cortos antes que aparentar.

**Descartado — subir `gist_max_tokens`.** No arregla nada: el problema no es que el techo sea bajo, es que **no se usa**. Subirlo sólo agrandaría el presupuesto abandonado.

**Descartado — cambiar cómo se redactan los contratos SDD** (que no empiecen con "17 tareas."). Trataría el **síntoma**: cualquier memoria cuya primera oración sea corta sufre lo mismo. El bug es del **extractor**, y ahí se arregla.

## D2 — La reparación del doctor es EXPLÍCITA

Un check `stale_gists` con su `apply`, en el mismo registro que `missing_digests`.

**Rationale — por qué no regenerar en el arranque.** Sería más cómodo… y es exactamente lo que **no** hay que hacer: 461 gists reescritos **en silencio** al levantar el binario es un cambio invisible en la superficie que el agente lee. **Un cambio que nadie pidió y nadie ve es el que después nadie puede explicar.** El `doctor --fix` lo hace **visible, deliberado y auditable**.

**Es seguro por naturaleza**: el gist es **derivado** de `content`. Regenerarlo es **idempotente** y no puede perder información — el peor caso es que quede igual.

## D3 — La medición no es opcional

Los items afectados pasan de ~11 a ~24 tokens ⇒ en el mismo presupuesto **entran menos memorias**.

**El canje se acepta, pero se informa con el NÚMERO.** Se mide sobre la memoria real: cuántos items trae un recall típico antes y después. Decir *«creo que compensa»* no alcanza cuando se puede contar.

## Lo que NO se toca

- El techo (`gist_max_tokens`), el presupuesto del recall, el ranking, MMR, el empaquetado.
- `content`, `content_hash`, embeddings, relaciones. **El gist es lo único que se recalcula.**
