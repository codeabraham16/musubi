# Spec — Dominios ajenos no se juzgan

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## Definición

El **dominio** de una observación es el primer segmento de su `topic_key`, hasta la primera barra:

| topic_key | dominio |
|---|---|
| `gio/auditoria-terminales-2026-08-05` | `gio` |
| `roadmap/track-potencia-medida` | `roadmap` |
| `git-commit` | `git-commit` |
| `bugs` | `bugs` |

Sin barra, el `topic_key` entero es el dominio.

---

## Invariantes

### D1 — Dos observaciones de dominios distintos no proponen relación *(el invariante fundamental)*

Si el dominio de A ≠ el dominio de B, el detector **no crea** ninguna relación entre ellas, sin
importar cuánto se parezcan léxica o vectorialmente.

### D2 — `git-commit` es la excepción, y es la única

Un par donde alguno de los dos lados tiene dominio `git-commit` **sí** propone relación aunque los
dominios difieran.

> No es un parche para salvar un caso: es la única señal cross-dominio que existe en toda la
> historia medida (`git-commit` × `bugs`). Un commit no es un dominio temático, es el registro de lo
> que pasó, y por eso puede volver obsoleta una nota de cualquier tema. La asimetría ya está
> reconocida en el código: `complementaryPair` documenta que un commit «feat: migrar de X a Y» SÍ
> puede envejecer una nota que decía «usamos X».

### D3 — Mismo dominio: nada cambia

Dos observaciones del mismo dominio se evalúan exactamente como antes. La guarda no toca el
scoring, ni los umbrales, ni el AND-gate, ni el gate de novedad.

### D4 — La guarda no oculta memoria

Es un `continue`: evita **crear** una relación. Nunca archiva, nunca marca `superseded_by`, nunca
saca nada del recall. El peor caso de un falso negativo es una relación de menos en la cola.

> Es la misma decisión que las guardas del PR #203, y la razón por la que una guarda de precisión
> es segura: el costo de equivocarse es asimétrico y cae del lado inofensivo.

### D5 — La guarda es simétrica

El orden de los argumentos no cambia el resultado. A diferencia de `complementaryPair` —que mira
sólo el destino a propósito— acá la pregunta «¿son del mismo tema?» no tiene lado, y qué
observación se guardó última es un accidente.

### D6 — Convive con las guardas que ya existen

`complementaryPair` (el par histórico) y el aislamiento por tenant siguen aplicándose. La nueva se
suma; no reemplaza ninguna, y ninguna la vuelve inalcanzable.

---

## Configuración

Ninguna. Una perilla acá sería una perilla que nadie calibra, y el default correcto está medido:
cero señal perdida sobre 494 relaciones reales.

---

## Criterios de aceptación

1. Los 6 invariantes con test propio, cada uno verificado **fallando** al sabotear.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde.
3. Los tests de las guardas del #203 siguen verdes **sin tocarse** — son la red que impide perder
   esas reglas en silencio.
4. Test adversarial: `topic_key` sin barra, con barra inicial, vacío, y un par `git-commit` ×
   `git-commit`.
