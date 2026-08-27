# S10 — Alertas y políticas

> Último slice del track «Control de flota». Cierra **cinco** cabos de `ABIERTO.md` de una vez
> —A4, A10, A11, A12, A19— porque los cinco son la misma pregunta vista desde ángulos distintos:
> **¿qué hace el sistema cuando nadie está mirando?**
>
> Hasta acá el track construyó un sistema que *responde bien cuando se le pregunta*. Todo lo que
> hace, lo hace porque alguien llamó una tool. Este slice le da el primer latido propio.

---

## 0 · Por qué estos cinco juntos, y no cinco slices

No es empaquetado por conveniencia. Los cinco se traban entre sí, y hacerlos por separado
obligaría a hacer dos veces la parte difícil:

- **A19 (sondear solo)** no se puede cerrar sin tocar el umbral de «en línea» — ver I2. Un Tier B
  sondeado cada 5 min sigue figurando caído con un umbral de 90 s, así que el sondeo automático
  *sin* el umbral por tier deja el síntoma exactamente igual y encima gasta red.
- **A10 (políticas)** es ejecución remota **sin una persona detrás**. Es lo más peligroso del
  track entero, y lo único que la hace defendible es que corra **con la autoridad de alguien**, no
  con la del daemon (I8). Ese «alguien» necesita poder estar acotado a un puñado de comandos ⇒
  necesita **A12**.
- **A12 (allowlist)** sin A10 es una perilla que casi nadie iba a girar. Con A10 es la diferencia
  entre `auto-heal` pudiendo vaciar un journal y `auto-heal` pudiendo hacer cualquier cosa.
- **A11 (poda)** y **A19** quieren el mismo latido de fondo. Dos tickers para el mismo subsistema
  es cómo se llega a que uno esté encendido y el otro no.
- **A4 (Alertmanager)** es el que convierte todo lo anterior en algo que se entera un humano. Y
  A10 **crea alertas nuevas que no existían** (I11): una política que actúa y no arregla es peor
  que no tener política, porque tapa el síntoma.

---

## 1 · Invariantes

### El latido de la flota

**I1 — La flota tiene su propio scheduler, y no cuelga del mantenimiento de la memoria.**
`RunFlotaScheduler` es un ticker aparte. La tentación era colgar la poda (A11) del
`RunScheduledMaintenance` que ya existe, y habría atado el vencimiento de las salidas de comandos
a `maintenance.auto_interval_hours` — un número que alguien puede poner en 0 por razones de
memoria, sin sospechar que con eso apaga la caducidad de datos de la flota.

**I2 — El umbral de «en línea» es POR TIER, porque las máquinas no dan señales de vida de la
misma manera.**
- **Tier A** late solo cada 30 s ⇒ umbral 90 s (3 latidos), como hasta ahora.
- **Tier B/C** no late: el cerebro lo va a buscar ⇒ el umbral se deriva del **intervalo de
  sondeo**, no de un latido que nunca existió.

Un solo número miente sobre uno de los dos. Con 90 s fijo y sondeo cada 5 min, un Tier B
perfectamente sano figura caído el **97 %** del tiempo — y `MaquinaCaida` dispara para siempre,
que es la manera más eficiente de que alguien silencie la alerta.

**I3 — El sondeo automático no mide lo que no le corresponde.** Saltea Tier A (tiene agente
propio, y no hay por dónde entrarle), saltea revocados, y exige `d.Permite(metrics)` — el eje del
**aparato**, que no depende de quién pregunte.

**I4 — El sondeo automático corre SIN principal, y eso no lo convierte en una puerta lateral.**
Escribe en la base; no le devuelve nada a nadie. Todo camino de LECTURA —tool, panel, `/metrics`—
sigue pasando por `PuedeSobreDevice` con la credencial de quien pregunta. Que el cerebro sepa algo
no es que vos puedas verlo.

**I5 — Un tick que sigue corriendo no arranca otro.** Con 40 máquinas por SSH, dos barridos
solapados son 80 conexiones. Bandera atómica, como `maintBusy`.

**I6 — La poda no borra la fila.** `PodarSalidasDeComandos` vacía `stdout`/`stderr` y deja el
registro: **qué se ejecutó, quién y cuándo es permanente; el contenido de la salida caduca.** Son
dos retenciones distintas a propósito.

### La allowlist

**I7 — La allowlist vive en la CREDENCIAL, nunca en la máquina.**
Fue la decisión de diseño del slice y el argumento es corto: un techo declarado por el aparato lo
declara *el aparato que se supone que está siendo acotado*. Una máquina comprometida se
auto-otorgaría `["*"]`. Como control de seguridad vale cero. La allowlist es del lado que la
máquina no puede tocar.

**I8 — La allowlist RESTRINGE, no otorga.** Se aplica *después* de la compuerta de tres lados de
S3, jamás en su lugar. Nadie gana acceso por aparecer en una allowlist.

**I9 — La sección entera es el opt-in, y una vez presente es EXHAUSTIVA.**
- Sin sección ⇒ sin restricción. Preserva exactamente lo que `exec` significa hoy; agregar la
  función no le rompe la configuración a nadie de un día para el otro.
- Con sección ⇒ una máquina sin entrada (y sin clave `"*"` de respaldo) **no permite nada**.
- Lista vacía (`nas: []`) ⇒ **cero comandos**, nunca todos. Es el bug clásico de las allowlists.

**I10 — El match es sobre `argv[0]` EXACTO.** Sin basename, sin globs. Comparar por basename
dejaría pasar `/tmp/evil/systemctl` contra una entrada `systemctl`, que es justo el bypass que la
allowlist venía a cerrar.

**I10b — Un intérprete en la allowlist ES la allowlist entera, y el arranque lo dice.**
`sh`, `bash`, `python`, `perl`, `ssh`, `env`, `xargs`… conceden todo lo que puedan lanzar.
Bloquearlos sería incorrecto (hay usos legítimos), callarlo sería peor: se **avisa por log al
arrancar**, con nombre y máquina.

### Las políticas

**I11 — Una política NO TIENE AUTORIDAD PROPIA.** Nombra un principal de `principals.yaml` y actúa
con la suya: misma compuerta, misma allowlist, misma bitácora. Si ese principal no tiene `exec`
sobre esa máquina, la política **no hace nada**.

Es el invariante que hace defendible el auto-heal entero. Sin él, «políticas» sería un segundo
camino a la ejecución remota que no pasa por el eje de capacidades — es decir, exactamente el
puente de privilegio que el track viene evitando desde el proposal.

**I12 — Se valida al ARRANCAR, no a las 3 de la mañana.** Principal inexistente ⇒ el servidor no
arranca. Principal sin ninguna concesión `exec` ⇒ tampoco: esa política está garantizadamente
muerta y descubrirlo durante un incidente es la peor hora posible.

**I13 — Una política no actúa sobre una muestra rancia.** Si el último dato es más viejo que el
umbral de «en línea» de esa máquina, no se evalúa. Actuar sobre datos viejos es actuar a ciegas:
el disco pudo haberse vaciado hace veinte minutos.

**I14 — Cooldown por (política × máquina).** Sin él una política dispara en cada tick hasta que la
métrica baje — y la métrica no baja hasta que el comando termine. Se cuenta desde el **disparo**,
no desde el resultado.

**I15 — Las políticas nacen APAGADAS.** Sin sección `fleet.politicas`, no existe.

**I16 — Cada acción queda en la MISMA bitácora que las de las personas**, con el nombre del
principal de la política. Un operador ve `auto-heal` al lado de `gio` en la misma tabla. Un
segundo registro de auditoría para lo automático es cómo se llega a auditar sólo la mitad.

### La notificación

**I17 — Una alerta que no llega a nadie no es una alerta.** Alertmanager pasa de comentario a
configuración desplegable, con receptor real.

**I18 — El silencio tiene que ser distinguible de la sordera.** Una regla `siempre firing`
(dead-man's switch) hacia un receptor watchdog: si *ese* deja de llegar, el problema es la cadena
de alertas, no la flota. Es la misma idea que `FlotaSinTelemetria`, un nivel más arriba.

**I19 — Una política que actúa y no cura, ALERTA.** Auto-heal que se dispara en loop no está
arreglando: está ocultando. Regla nueva: acciones repetidas sostenidas sobre la misma máquina.

---

## 2 · Lo que queda fuera (y va a `ABIERTO.md`)

- **Condiciones como expresión.** El `cuando:` es un enum acotado sobre los campos de la muestra,
  no un mini-lenguaje. Un evaluador de expresiones que decide qué comando correr como root es una
  superficie que no se justifica todavía.
- **Acciones que no sean un comando.** Nada de webhooks ni de «apagar la máquina» como primitiva:
  todo lo que hace una política es *un exec que ya podrías haber hecho a mano*.
- **Cooldown persistente.** Vive en memoria: un reinicio del cerebro rearma los cooldowns. Se
  anota; el caso malo (reinicio justo después de un disparo) es acotado y benigno.
