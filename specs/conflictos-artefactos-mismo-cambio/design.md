# Design — conflictos-artefactos-mismo-cambio

## D1 — Una sola guarda, en el tope del loop de candidatas

```go
for _, c := range cands {
    if complementaryPair(src, c) {
        continue          // <-- acá
    }
    lex := Similarity(src.content, c.content)
    ...
}
```

**Rationale.** `DetectRelations` tiene **un único** loop donde nacen **todas** las relaciones. Poner la guarda ahí la vuelve **imposible de saltear**: no hay un segundo camino por el que una relación pueda colarse. Es la misma jugada que M4 (#195), que hizo `markSuperseded` inalcanzable *por construcción* en vez de confiar en que alguien se acuerde.

Va **antes** de `Similarity` y del coseno, así que además es **gratis**: se ahorra el scoring de los pares que igual iba a descartar.

**Descartado — filtrar en `relevantPair`.** Es donde uno lo pondría por reflejo, pero `relevantPair` responde *"¿este par tiene señal suficiente?"*. La guarda responde otra pregunta: *"¿este par es siquiera comparable?"*. Mezclarlas haría que un cambio de umbral pudiera resucitar el ruido. Son ortogonales y quedan separadas.

**Descartado — filtrar en `conflictCandidates` (el pool).** El pool se arma con FTS + vecinos vectoriales; filtrar ahí obligaría a repetir la lógica en dos ramas (léxica y vectorial). Un solo punto, después de la unión.

## D2 — La guarda es puramente estructural: `topic_key`, nunca contenido

```go
// sddChange devuelve el <cambio> de un topic_key con forma sdd/<cambio>/<fase>.
// "" si no matchea (degradación segura: el par cae al camino normal).
func sddChange(topicKey string) string {
    rest, ok := strings.CutPrefix(topicKey, "sdd/")
    if !ok {
        return ""
    }
    change, _, ok := strings.Cut(rest, "/")   // exige la barra de la <fase>
    if !ok || change == "" {
        return ""
    }
    return change
}

func isSDD(topicKey string) bool  { return sddChange(topicKey) != "" }
func isCommit(topicKey string) bool { return topicKey == commitTopicKey }

// complementaryPair: dos artefactos que NO son redundantes entre sí por su ESTRUCTURA,
// independientemente de cuánto se parezcan. No es una heurística: es un hecho.
func complementaryPair(a, b obsRow) bool {
    // G1 — hermanos del mismo cambio SDD: proposal, spec, design... se complementan,
    // no se duplican. Ninguno se puede borrar.
    if ch := sddChange(a.topicKey); ch != "" && ch == sddChange(b.topicKey) {
        return true
    }
    // G2 — el EVENTO vs el CONTRATO: un commit no puede reemplazar a un spec, ni al revés.
    if isCommit(a.topicKey) && isSDD(b.topicKey) {
        return true
    }
    if isSDD(a.topicKey) && isCommit(b.topicKey) {
        return true
    }
    return false
}
```

**Rationale.** La decisión no mira el contenido **a propósito**. El parecido entre un `spec` y un `design` del mismo cambio es **alto y correcto** — mirar el texto sólo puede confundir. Lo que decide es de **qué son artefactos**, y eso vive en el `topic_key`, que `obsRow` ya carga. Cero costo, cero esquema.

`sddChange` exige la **barra de la fase** (`sdd/<cambio>/<fase>`): sin ella devuelve `""`. Así un `topic_key` raro como `sdd/loquesea` no activa la guarda — cae al camino normal, que es el comportamiento de hoy (R3).

**Descartado — usar el tipo de memoria (`mem_type`: semantic/episodic/procedural).** Es tentador ("un commit es episódico, un spec es semántico"), pero es **demasiado grueso**: metería en la misma bolsa pares que sí queremos comparar (dos notas semánticas *sí* pueden ser duplicados). El `topic_key` dice exactamente lo que necesito y nada más.

## D3 — La guarda es simétrica

G2 se chequea en **ambos órdenes** (`a→b` y `b→a`). No es un detalle cosmético: `DetectRelations` corre con `src` = la observación **recién guardada**, así que el orden depende de **quién llegó último**. Si guardo el commit después del SDD, `src` es el commit; si corro el SDD después, `src` es el contrato. Una guarda asimétrica taparía **la mitad** de los casos, de forma no determinista según el orden de guardado — el peor tipo de bug.

## D4 — `commitTopicKey` pasa a ser una constante compartida

Hoy el string `"git-commit"` vive en `cmd/musubi/capture.go`. La guarda lo necesita en `internal/memory`. Se declara **una** constante en `internal/memory` y la captura la usa desde ahí.

**Rationale.** Dos literales `"git-commit"` en paquetes distintos es una bomba de tiempo: el día que uno cambie, la guarda deja de aplicar **en silencio** (falso negativo, sin error, sin test rojo — el peor modo de fallo). Una constante hace que ese desacople sea imposible.

## Lo que este diseño NO hace

- No toca umbrales, ni el AND-gate, ni el gate de novedad.
- No oculta, archiva ni marca `superseded` nada. **Una guarda evita CREAR una relación.** Es un `continue`, no un `DELETE`.
- No filtra pares entre memorias **comparables** (dos notas, dos commits, un commit y una nota): ésos siguen exactamente el camino de hoy.
