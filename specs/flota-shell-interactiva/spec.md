# S5b — Shell interactiva

> Cierra **A5** de `ABIERTO.md`. S5 dejó el one-shot: mandás un argv, esperás, leés la salida.
> Cubre la mayoría de los casos y no cubre ninguno de los que importan cuando algo está roto —
> `top`, `journalctl -f`, un `apt` que pregunta, un editor, un shell para mirar alrededor.

---

## 0 · La decisión que manda sobre todo el resto

**UNA SHELL INTERACTIVA NO ES `exec`. ES UNA CAPACIDAD APARTE.**

S10 partió `exec` en dos permisos: **poder ejecutar** (la concesión) y **poder ejecutar cualquier
cosa** (la allowlist). Una shell interactiva es el tercero, y es el que se lleva puestos a los
otros dos: quien obtiene un prompt ejecuta lo que quiera, todas las veces que quiera, sin que
nadie vuelva a mirar un argv.

Si `musubi shell` se gateara con `exec`, entonces un principal con

```yaml
fleet:
  exec: ["nas-casa"]
fleet_exec_allow:
  nas-casa: ["journalctl"]
```

tendría, escribiendo un comando distinto, exactamente lo que la allowlist le estaba negando. La
allowlist pasaría a ser decoración — y peor: decoración en la que alguien confía.

Así que **`shell` es la cuarta capacidad**, con las mismas reglas que las otras tres: se declara,
la ausencia no significa nada, y no se deriva de ningún rol ni de ninguna otra capacidad.

---

## 1 · Invariantes

### La capacidad

**T1 — `shell` nunca se implica.** Ni de `exec`, ni de `admin`, ni de tener `exec: ["*"]`. Un
principal con acceso total de ejecución y sin `shell` declarado no abre una sesión. Es la misma
asimetría del track: administrar no otorga flota, y ejecutar no otorga prompt.

**T2 — Está en la matriz del tier, y Tier C queda afuera.** A y B la admiten; C no, por el mismo
motivo por el que no admite `exec`: en iOS no existe y en Android depende de que ADB esté
habilitado, o sea que no se puede prometer al dar de alta.

**T3 — Un device ya dado de alta NO gana `shell` por esta versión.** La capacidad vive en la fila
del dispositivo; los existentes no la tienen y hay que concedérsela explícitamente. Nada de
migraciones que otorguen permisos.

### La sesión

**T4 — Toda sesión queda en la bitácora: quién, a qué máquina, cuándo empezó y cuándo terminó.**
Es lo más sensible del track entero. Lo que NO se guarda es el CONTENIDO —ni lo tecleado ni lo
impreso—: eso es grabación, y la grabación es una decisión legal antes que técnica (ver A14).

**T5 — Ninguna sesión es eterna.** Dos techos, y son distintos:
- **Vida máxima** — una sesión olvidada abierta es una puerta trasera con nombre de nadie.
- **Inactividad** — una terminal abierta en una pestaña que nadie mira es exactamente eso.

Los aplica **el cerebro**, que es quien tiene el relay: si dependieran de la máquina remota, una
máquina comprometida se los saltearía.

**T6 — EL ID DE SESIÓN NO ES UNA CREDENCIAL.** Cada request del stream lleva el bearer de la
persona y se re-autoriza: se comprueba que la sesión existe, que es SUYA, que no venció y que la
concesión **sigue** vigente. Revocar a alguien en `principals.yaml` tiene que cortarle la sesión
en curso, no sólo impedirle abrir la próxima.

Sin esto, el id sería un token portador con permisos de shell y sin vencimiento propio — el error
clásico de las APIs de sesión.

**T7 — Una sesión por vez y por (persona, máquina).** No es una limitación técnica: dos prompts
simultáneos de la misma persona en la misma máquina es casi siempre una sesión olvidada más una
nueva, y la olvidada es la peligrosa.

### El transporte

**T8 — Sin dependencias nuevas.** Nada de WebSocket: dos streams HTTP half-duplex (uno que baja
la salida, otro que sube lo tecleado) hacen el trabajo con la biblioteca estándar, atraviesan
cualquier proxy y usan el mismo bearer que el resto.

**T9 — El cerebro no interpreta el flujo.** Relay de bytes. No parsea, no filtra, no reescribe:
lo que se teclea llega tal cual y lo que sale se muestra tal cual. Un relay que intenta ser listo
rompe `vim`, rompe el color y rompe el redibujado.

**T10 — Si el relay se corta, la sesión MUERE.** No queda un proceso huérfano del otro lado
esperando que alguien vuelva. La reconexión es una comodidad; un shell sin dueño es un problema.

---

## 2 · Alcance de esta entrega

**Tier B (SSH), de punta a punta.** Es donde vive la mayoría de lo que hay —routers, NAS,
Raspberry Pis, servidores sin agente— y donde el trabajo pesado ya está hecho: `ssh -tt` asigna
el pty del lado remoto, así que **el cerebro no necesita un solo syscall de pty**.

Lo que Musubi agrega sobre un `ssh` a mano, y es todo lo que este slice vale:
la compuerta por persona y por máquina, la bitácora, el alcance desde donde sea a través del
cerebro, los techos de vida e inactividad — y que **no se guarda ninguna credencial**.

## 3 · Lo que queda fuera (va a `ABIERTO.md`)

- **Tier A (agente con pty propio)** — necesita abrir `/dev/ptmx` y forkear un shell en la máquina
  remota. Sin cgo son ioctls a mano, y en Windows es ConPTY, que es otro mundo. **→ S5c.**
- **Cliente Windows en modo crudo** — la CLI necesita poner la terminal en raw. En unix son dos
  ioctls de termios; en la consola de Windows es `SetConsoleMode`. **→ S5c.**
- **Grabación del contenido de la sesión** — misma decisión legal que A14, mismo dueño.
- **Reconexión a una sesión viva** — ver T10: sale caro y su beneficio es comodidad.
- **Redimensionado de la ventana (SIGWINCH)** — el tamaño se fija al abrir. Sin esto `top` se
  dibuja con el ancho inicial si alguien agranda la ventana a mitad de sesión. **→ S5c.**
