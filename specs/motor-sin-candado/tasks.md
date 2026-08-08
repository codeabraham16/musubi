# Tareas — El motor no traba la casa

Estado: **completo**. Build, `go vet ./...`, `gofmt`, `golangci-lint` (0 issues) y la suite entera
(17/17 paquetes) en verde.

## Lo que se midió antes de proponer

- [x] **T0 — El defecto, con números.** `handleToolsCall` sostiene el candado con `defer` alrededor
      del handler entero. `musubi_recall` no es `readOnly` ⇒ candado **exclusivo**. El juez
      (`rerankIfEnabled`) y `musubi_ask` llaman al LLM adentro. Techo real: 120 s (cliente HTTP del
      provider) bajo un backstop de 150 s (`askTimeout`). Del ledger del central, 30 días: el único
      `musubi_ask` real tardó **25.128 ms**, y `save_observation` promedia 4.856 ms / p95 10.194 ms
      bajo el mismo candado.

- [x] **T0b — Por qué el arreglo es barato.** El bump que justifica el candado exclusivo es
      `UPDATE observations SET access_count = access_count + 1 WHERE id IN (...)`: **una sentencia
      atómica**, no el read-modify-write que el comentario del candado dice estar protegiendo.

- [x] **T0c — El defecto de fondo.** `readOnly` gobierna concurrencia Y autorización en un solo
      booleano. Delator: el comentario de `toolAsk` declara *«es de sólo lectura»* y la tool no está
      marcada — el código ya sabe que los ejes no coinciden y no tiene cómo decirlo.

- [x] **T0d — Lo que el análisis descartó.** Cambiar el bind de litellm **no hace falta**: el central
      ya sirve `musubi_ask` en la tailnet con la cognición encendida. El motor ya es alcanzable; lo
      que falta es control. Eso saca del plan el paso que yo había puesto como central.

## Lo construido

- [x] **T1 — H1: la clase de candado.** `lockClass` en `toolEntry`, con `lockFromReadOnly` = cero de
      Go = comportamiento histórico. Ninguna de las otras 53 registraciones se editó.

- [x] **T2 — H1: `readOnly` queda como eje de autorización.** Mismo nombre, mismo efecto en
      `canCall`, mismos 22 marcados. Deja de ser la última palabra sobre el candado.

- [x] **T3 — H2: `toolRecall` en tres tramos.** Embeber (sin candado) → `withReadLock{ Recall }` →
      juez (sin candado).

- [x] **T4 — H2: `toolAsk` en tres tramos.** Embeber → `withReadLock{ Recall + hidratación }` → LLM.
      Los dos accesos a la base entran en la MISMA sección crítica: son consecutivos y el segundo
      depende del primero, así que partirlos sólo agregaría un ciclo de toma-y-suelta.

- [x] **T5 — 11 invariantes** (G1–G9 y G11 en `internal/mcp`, G10 en `internal/memory`, que es donde
      vive `bumpAccess`). Cada uno con su control: G2 prueba que las escrituras SIGUEN serializadas,
      G8 que el mapa de autorización no se movió, y G11 que con el juez apagado —la config del
      central hoy— no cambia nada.

- [x] **T6 — Sabotaje: 10 mutaciones, las 10 en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | El default de la clase se invierte | G1 | rojo |
      | `save_observation` pasa a compartido | G2 | rojo |
      | `musubi_ask` vuelve a la clase por default | G3 | rojo |
      | El juez vuelve a correr adentro de la sección crítica | G4 | rojo |
      | El embeber vuelve a quedar adentro del candado | G5 | rojo |
      | Se saca el `defer` del helper | G6 | rojo |
      | La autorización se deriva de la clase de candado | G7·G9 | rojo |
      | **Se marca `musubi_recall` como `readOnly`** | G8 | rojo *(ver abajo: primero salió verde)* |
      | El bump pasa a read-modify-write en Go | G10 | rojo |
      | El juez se llama con la flag apagada | G11 | rojo |

- [x] **T7 — El esquema NO cambió.** `toolslist.golden.json` quedó byte-idéntico (`git diff` vacío) y
      `TestToolsListGolden` pasa sin regenerar. La clase de candado no se filtró a la interfaz.

- [x] **T8 — Medido, y el número queda en el test.** Con el motor colgado, la tool concurrente
      responde en **2 ms** (G3), **3 ms** (G4) y **2 ms** (G5). Antes del arreglo esperaba a que el
      motor contestara: en los sabotajes la sonda se come el timeout de la prueba, y en producción
      el techo es de 120 s. El `t.Logf` quedó permanente para que el número se vea con `go test -v`
      en vez de vivir en un mensaje de commit.

## Tres cosas que el trabajo enseñó

**★ El sabotaje encontró un defecto en MI PROPIA PRUEBA.** G8 comparaba
`canCall(toolReadOnly[n])` contra `e.readOnly` — y eso es una tautología, porque `canCall` devuelve
exactamente `readOnly` para un reader. Marcar `musubi_recall` como `readOnly` dejaba la prueba en
**verde**, que es justo el cambio que G8 existe para atajar. Se reescribió contra una **lista fija
de 22 nombres** escrita a mano: derivar el esperado del código que estás probando no prueba nada.

**La sonda de concurrencia tiene que ser ESCRITORA, no lectora.** La primera versión de G5 usaba
`musubi_conflicts` (sólo lectura). Si el embed volviera a quedar adentro de la sección crítica —que
toma `RLock`— dos lectores conviven y la prueba pasaría igual. Sólo un escritor se bloquea con
**cualquier** candado tomado, compartido o exclusivo. Se cambió a `musubi_save_fact`, que además no
toca el embedder.

**★ Se puso rojo en CI, y mi primera explicación fue la equivocada.** `windows-latest` falló G4 con
*«el juez nunca llamó al motor»*. El mismo job pasó en el otro run del mismo commit — la firma
clásica de un flaky — pero **no era el flaky conocido del scheduler, era mi test**, y eso sólo se
supo leyendo el log en vez de asumirlo por la firma.

Diagnostiqué **lentitud**: el paquete tardó 318 s en ese runner y la espera era de 5 s. Subí el
presupuesto a 60 s… y **volvió a fallar, quemando los 60 s completos**. O sea que la hipótesis era
falsa: el juez genuinamente no se llamaba, y ningún presupuesto lo iba a arreglar.

El defecto de verdad estaba **en el diseño de la prueba**, y era doble:

1. **La goroutine descartaba el error de la tool.** Si el recall fallaba, esa información —la única
   que explicaba el fallo— se tiraba a la basura.
2. **El `select` sólo tenía dos salidas** (`entró` o timeout). Cuando la tool terminaba *sin* tocar
   el falso, la prueba no tenía forma de notarlo: esperaba el presupuesto entero y después acusaba
   al reloj. **El mensaje de error era mentira**, y encima cara: 60 s para decir algo falso.

El arreglo tiene tres partes: `esperarQueElFalsoReciba` agrega `terminó` al `select` —así ese caso
se detecta al instante y se nombra como precondición rota, no como bloqueo—; el error de la tool se
guarda y se imprime; y `exigeSembradas` verifica que las 3 observaciones sobrevivieron, porque el
juez necesita ≥2 items y algo que las colapsara daría exactamente este síntoma.

Las dos esperas también se separaron, que sigue siendo correcto aunque no fuera la causa:

| | qué es | presupuesto |
|---|---|---|
| `esperaArranque` | **viveza**: que el falso reciba la llamada. Es el preámbulo, no la aserción | 60 s |
| `esperaMax` | **la aserción**: que la sonda concurrente no quede bloqueada | 30 s |

Ser generoso no debilita nada: bajo el defecto la sonda queda bloqueada hasta que la prueba suelte
el motor —indefinidamente—, así que el timeout cambia cuánto tarda en detectarse el fallo, no si se
detecta. Verificado: los 10 sabotajes siguen en rojo con los presupuestos nuevos.

**Queda dicho de frente: la causa raíz del rojo en CI todavía no está identificada.** Localmente,
30 corridas seguidas dieron 3 observaciones, 3 items y el motor llamado — no se reprodujo (y sin
`gcc` en esta PC tampoco se puede correr `-race`, que es como corre ese job). Lo que cambió es que
la prueba ahora **dice la verdad cuando falla**: la próxima vez va a nombrar cuál precondición se
rompió en vez de culpar al candado.

**El falso bloqueante necesitó un interruptor.** Guardar una observación también embebe, así que un
embedder que se cuelga siempre frenaba la propia siembra y la prueba moría antes de medir nada. El
campo `activo` se prende recién después de sembrar. Es el tipo de detalle que, sin resolver, empuja
a escribir la prueba con `sleep` — y ahí deja de detectar el defecto.

## Fuera de alcance, dicho de frente

- **Quién puede usar el motor no cambió.** Hoy alcanza con ser `writer` — 6 de los 8 principals del
  central lo son, gio incluido. Este spec lo dejó **igual de abierto**. Es el paso 4 de F1.
- **No hay presupuesto ni medición de gasto.** `CognitionStats` cuenta caché, portero y escalaciones
  del router; tokens y costo no existen (`budget|quota|spend` no aparece en `internal/cognition/`).
  Paso 3 de F1.
- **El juez sigue apagado en el central.** Este spec lo vuelve *encendible* sin romper nada; si se
  enciende lo decide el banco de F2.
- **La contención que queda es de las escrituras.** Con el motor afuera del candado, el cuello que
  sobra son los saves (4,9 s de media, p95 10,2 s, todos bajo lock exclusivo). Es un problema de la
  base, no del motor.
