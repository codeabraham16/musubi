# Tareas — El motor tiene freno propio

Estado: **completo**. Build, `go vet ./...`, la suite entera y `golangci-lint` (0 issues) en verde.
10 invariantes (M1–M10), 12 sabotajes, los 12 en rojo.

## Lo que se midió antes de proponer, y cómo corrigió el plan

- [x] **T0 — ★ La urgencia que yo había declarado no existe.** Dije que esto era lo más urgente
      porque «cualquier writer puede gastar la suscripción». El ledger del central: **3 llamadas a
      `musubi_ask` en 30 días, las tres de `davantis-admin`**. `gio` tiene 49 llamadas en 9 tools y
      **ninguna** al motor. Nadie fuera del dueño lo usó jamás.

- [x] **T0b — La medición ya existía.** El ledger (`tool_invocations`) guarda tool, resultado,
      duración, principal y proyecto, y sobrevive a los reinicios. «Quién gastó» se contestó con una
      consulta, no con código nuevo. Lo que faltaba no era medir: era **frenar**.

- [x] **T0c — El hueco exacto, con su número.** La cuota existente cuenta todas las tools por igual
      y su default es **600/min** — calibrado para tools gratis. Aplicado al motor son 864.000
      llamadas al modelo por día antes de que algo diga que no.

- [x] **T0d — Sólo dos caminos llegan al motor**, y salió de leer el código: `musubi_ask`
      (`s.cognition.Ask`) y `musubi_recall` vía `rerankIfEnabled`. Con dos detalles que cambiaron el
      diseño: el costo del recall es **condicional** (sólo con `read_time_rerank` encendido) y **un
      acierto del caché de rerank no gasta**.

## Lo construido

- [x] **T1 — `motorQuota`**, un `quotaLimiter` aparte con ventana de una **hora**. Se reusó el tipo
      que ya existía en vez de escribir un contador nuevo.

- [x] **T2 — El cobro va donde se gasta**, no en el despacho: `hayPresupuestoDeMotor` se llama justo
      antes de cada llamada al modelo. Cobrar en el despacho habría estrangulado los dos casos
      gratis de T0d.

- [x] **T3 — La asimetría**: `musubi_ask` se **rechaza** (`codeMotorQuota`), `musubi_recall`
      **degrada** al orden model-free. No es una regla nueva — es la que ya rige cuando el motor se
      cae.

- [x] **T4 — `OutcomeDeniedMotor`**, aparte de `denied_quota`. Y **entra solo al reporte que ya
      existe**: `ToolUsage` cuenta `outcome LIKE 'denied_%'`, así que el rechazo aparece en
      `musubi_tool_usage` sin tocar la consulta. Verificado en M9, no supuesto.

- [x] **T5 — `motor_quota_per_hour`** con la MISMA semántica que `quota_per_minute` (0 ⇒ default,
      negativo ⇒ sin límite). Default **60/hora**, que sale de medir: ~150× la tasa real, holgado
      para una tarde intensa y suficiente para que un bucle queme 60 y no 36.000.

- [x] **T6 — La degradación se cuenta**: `musubi_tool_rejections_total{reason="motor_quota"}`. El
      recall devuelve ok y el usuario no ve nada; sin el contador, el sistema podría dejar de usar
      el juez sin que nadie se enterara.

- [x] **T7 — 10 invariantes** (M1–M10).

- [x] **T8 — Sabotaje: 12 mutaciones, las 12 en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | `musubi_ask` no consulta el presupuesto | M1 | rojo |
      | El rechazo pierde su código propio | M1 | rojo |
      | El recall no consulta y llama igual | M2 | rojo |
      | Cobra aunque el juez esté apagado | M3 | rojo |
      | Cobra antes de mirar el caché | M4 | rojo |
      | Cobra DESPUÉS de llamar al motor | M5 | rojo |
      | El presupuesto se vuelve global | M6 | rojo |
      | Sin principal también se frena | M7 | rojo |
      | El cero deja de dar el default | M8 | rojo |
      | El rechazo se cuenta como error genérico | M9 | rojo |
      | `denied_motor` colisiona con `denied_quota` | M9 | rojo |
      | La degradación no se cuenta | M10 | rojo |

      Se mutó por **número de línea** y no por patrón: `if !s.hayPresupuestoDeMotor(ctx) {` aparece
      dos veces —una por cada camino al motor— y un `sed` por patrón habría tocado las dos, mutando
      dos invariantes a la vez y arruinando el experimento.

## Lo que el trabajo enseñó

**Cinco pruebas mías pasaban sin ejercitar nada, y el motor espía lo delató.** M1, M5, M6, M7 y M9
fallaron con «el motor recibió 0 llamadas». La causa: `musubi_ask` **corta antes de llamar al motor
si el recall del grounding vuelve vacío**, y la base de prueba estaba sin sembrar. Sin el contador
del motor espía, la versión rota habría sido una prueba que verifica que un freno frena algo que
nunca arrancó. El arreglo fue sembrar en el helper y hacer que la pregunta **matchee** lo sembrado —
una pregunta que no recupera nada deja el mismo agujero.

**El caché fue lo que obligó a mover el cobro de lugar.** Lo natural era cobrar en el despacho, junto
a la cuota general. Leer `rerankIfEnabled` mostró dos casos que no gastan un centavo —el juez
apagado y el acierto de caché— y que el cobro en el despacho habría penalizado. El presupuesto se
consulta donde ocurre la llamada, y por eso M3 y M4 existen.

## Fuera de alcance, dicho de frente

- **No hay capability nueva.** Sería autorización para usuarios que no existen (T0). El freno queda,
  y la capability se agrega encima el día que alguien más que el dueño toque el motor.
- **El freno vive en memoria** y se pierde al reiniciar, como la cuota general. Uno persistente es
  otro spec; éste acota el daño de un bucle, que es el riesgo real.
- **El juez sigue apagado.** Esto es la precondición para encenderlo, no el encendido.
- **Se cuentan llamadas, no tokens ni dinero.** Es lo que el sistema puede saber por sí mismo.
