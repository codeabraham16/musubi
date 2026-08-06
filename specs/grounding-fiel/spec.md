# Spec — Grounding fiel para `musubi_ask`

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**: si se rompe la propiedad,
el test se pone rojo. Un invariante sin test que lo pueda tumbar es decoración.

---

## Alcance

| Superficie | Estado |
|---|---|
| El prompt de grounding de `musubi_ask` | **en alcance** |
| `musubi_recall` (model-free) | fuera: sigue devolviendo gists, sin un byte de cambio |
| El juez read-time (`read_time_rerank`) | fuera: reordena candidatos, no arma el prompt |
| La selección de qué memorias fundamentan | fuera: la decide el recall, y no se toca |

---

## Invariantes

### G1 — El grounding manda el CONTENIDO COMPLETO *(el invariante fundamental)*

Para toda memoria que entre en el presupuesto de hidratación, el prompt lleva su contenido íntegro,
no el gist. Una observación cuyo contenido excede el largo del gist llega entera.

> Es la razón de existir del cambio. Si sólo se pudiera verificar uno, es este.

### G2 — La hidratación cambia la PROFUNDIDAD, nunca la SELECCIÓN

El conjunto de memorias del grounding —cuáles y en qué orden— es exactamente el que devolvió el
recall. Hidratar no agrega ni saca ni reordena. `grounded_on` sigue contando lo mismo.

> Separar las dos decisiones es lo que hace revisable el cambio: si mañana una respuesta sale mal,
> se sabe si falló *qué se eligió* (recall, model-free, auditable) o *cuánto se mandó* (esto).

### G3 — La advertencia de ranciedad sobrevive al cambio

Una observación con anclas que ya no coinciden con el disco llega al motor **con su advertencia**.

Hoy la advertencia viaja **pegada al gist** (`markStaleOrigins` la antepone). Cambiar el gist por el
contenido crudo la borraría **en silencio**: el prompt seguiría viéndose bien y el modelo dejaría de
enterarse de que la nota puede estar vencida. Es la regresión más fácil de no notar de todo el
cambio, y por eso es invariante y no comentario.

### G4 — El sello de procedencia viaja en el prompt

Una memoria con procedencia `llm:*` o `deterministic` llega al motor **marcada**. Una `human` no
lleva marca (si todas la llevaran, el sello sería ruido y dejaría de leerse).

Esto **no existe hoy**: es un agujero de Q3 en el camino de `ask`, no una consecuencia de hidratar.
Se cierra acá porque es el mismo prompt.

### G5 — La cuarentena no entra por la puerta de la hidratación

Una observación en cuarentena **nunca** aparece en el prompt de grounding.

La hidratación por id **no aplica** el predicado canónico de visibilidad — a propósito (Q0b). La
muralla se sostiene porque los ids salen **exclusivamente** del recall, que sí lo aplica. Es una
garantía por procedencia de los ids, no por filtrado, y por eso necesita test propio: si alguien
mañana alimenta la hidratación desde otra fuente, esto se cae sin que nada más se ponga rojo.

### G6 — Un fallo hidratando no tumba la pregunta

Si la hidratación falla, `ask` responde igual, fundamentado en los gists. La profundidad es una
mejora; su ausencia degrada la calidad, no la disponibilidad.

### G7 — El prompt tiene techo

La hidratación respeta un presupuesto de tokens derivado del que pidió el caller. Lo que no entra se
queda en gist. El prompt no puede crecer sin límite por tener mucha memoria relevante.

### G8 — Hidratar no cuenta como un segundo acceso

Fundamentar una pregunta es **un** uso de la memoria, no dos. `Recall` ya contabiliza el acceso de lo
que devuelve; hidratar esos mismos ids no puede volver a incrementarlo.

> Sin esto, `ask` inflaría `access_count` al doble de lo real sobre justo las memorias que más se
> consultan, y el ranking del recall empezaría a alimentarse de su propia salida — que es
> exactamente lo que la invariante N4 del ranker prohíbe.

---

## Configuración

Sin config nueva. El presupuesto de hidratación se **deriva** del `token_budget` que ya acepta la
tool, por un factor fijo y con tope. Una perilla más sería una perilla que nadie calibra: el caller
ya expresa cuánta memoria quiere con el parámetro que existe.

---

## Criterios de aceptación

1. Los 8 invariantes con test propio, y cada test verificado **fallando** al sabotear la
   implementación.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde.
3. El camino model-free (`musubi_recall`) verificado bit-idéntico.
4. Test adversarial: hidratación que falla, observación en cuarentena entre las candidatas,
   observación rancia, y una memoria cuyo contenido excede el presupuesto entero.
