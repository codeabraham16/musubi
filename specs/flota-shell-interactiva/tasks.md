# S5b — tareas

Cierra **A5** de `ABIERTO.md`.

## La cuarta capacidad

- [x] `fleet.CapShell` en la matriz del tier: A y B sí, **C no** (mismo motivo que `exec`: en iOS
      no existe y en Android depende de que ADB esté habilitado, o sea que no se puede prometer al
      dar de alta).
- [x] **Nunca se implica.** Ni de `exec`, ni de `exec: ["*"]`, ni de `admin`. Un device ya dado de
      alta tampoco la gana: la capacidad vive en su fila y hay que concedérsela.
- [x] El mensaje de rechazo **nombra `shell` y explica por qué no alcanza `exec`**: el error más
      probable acá es que alguien tenga `exec` y crea que le basta, y decirlo mal lo manda a
      revisar la concesión equivocada.

**Por qué es una capacidad aparte** — es la decisión que sostiene el slice entero. S10 partió
`exec` en dos permisos: poder ejecutar (la concesión) y poder ejecutar *cualquier cosa* (la
allowlist por comando). Una shell interactiva es el tercero y se lleva puestos a los otros dos.
Gatearla con `exec` habría dejado a un principal acotado a `["journalctl"]` obteniendo, tecleando
otra cosa, exactamente lo que la allowlist le negaba — y la allowlist pasaría a ser decoración en
la que alguien confía.

## La sesión

- [x] Migración **34**: `shell_sessions`. **No hay columna para el contenido**, y hay una prueba
      de FORMA (por reflexión) que lo custodia — la única manera de proteger una ausencia.
- [x] Se registra **antes** de conectar: un intento fallido queda auditado igual.
- [x] **Dos techos, y son distintos** (los aplica el cerebro, no la máquina remota): vida máxima
      2 h e inactividad 15 min. Sólo con el de vida, una pestaña olvidada es un prompt vivo dos
      horas; sólo con el de inactividad, un `tail -f` la hace eterna.
- [x] Una sola sesión viva por (persona, máquina): se devuelve la que ya está, para que quien
      perdió su terminal pueda volver a ella y cerrarla.
- [x] El estado se DERIVA al leer, nunca se recalcula con un barrido.

## El relay

- [x] **El id de sesión NO es una credencial.** Cada request re-hace las cuatro preguntas: ¿existe?
      ¿es tuya? ¿sigue viva? ¿tu concesión SIGUE vigente? La cuarta es la que se olvida siempre, y
      es la que hace que revocar signifique algo — **corta el prompt abierto, no sólo el próximo**.
- [x] **Tres códigos distintos**, porque quien está del otro lado hace cosas distintas con cada
      uno: 401 (revisá tu credencial) · 403 (tu credencial está bien y ya no te alcanza) · 410 (la
      sesión terminó, abrí otra). El atajo de devolver 401 para todo manda a rotar tokens sanos.
      Lo que sí queda indistinguible, a propósito: «no existe» de «no es tuya».
- [x] Dos streams HTTP half-duplex, sin dependencias nuevas. El GET vuelve **apenas hay un byte**,
      no al vencer su espera: la latencia la pone la red, no el diseño.
- [x] **Contrapresión** en el buffer: con 256 KiB llenos, el escritor se frena. Es la única de las
      tres salidas posibles que no rompe nada — un buffer sin límite se come un `cat /dev/urandom`
      en la RAM del cerebro, y un ring buffer que descarta lo viejo *garglea la terminal*, que es
      peor que un error porque parece que funciona.

## Tier B, y por qué es el que se entrega

`ssh -tt` asigna el pty **del lado remoto**, así que el cerebro no necesita un solo syscall de pty.
Es exactamente por eso que Tier B entra en este slice y Tier A no. Las tres guardas del one-shot de
S5 se conservan: `BatchMode`, `ConnectTimeout` y **`StrictHostKeyChecking=yes`, que acá menos que
nunca se afloja** — por este canal viaja todo lo que la persona teclee, contraseñas de sudo
incluidas.

## La CLI

- [x] `musubi shell <maquina>`, con el modo crudo pedido a **`stty`** y no con ioctls a mano.
      Termios por syscall es una estructura distinta por OS y números de ioctl distintos por
      arquitectura; `stty` está en toda máquina unix y sabe de termios más que nosotros. Es el
      mismo criterio con el que el track invoca al `ssh` del sistema en vez de implementar SSH.
- [x] `stty -g` guarda el estado **entero**: si mañana se toca una bandera más, la restauración
      sigue siendo exacta sin cambiar nada.
- [x] La terminal se restaura por `defer` **y** por señal: si el proceso muere sin restaurar, queda
      inutilizable y hay que teclear `reset` a ciegas.
- [x] En la ayuda global (hay una prueba que lo exige desde que `ingest` vivió cableado e
      invisible).

## Pruebas

**22 nuevas.** **10 sabotajes verificados** uno por uno. Dos guardas preexistentes hicieron su
trabajo y se atendieron: el conteo de tools de los dos READMEs (71 → 73) y el golden de
`tools/list`.

Una prueba propia **no valía nada y se reescribió**: la de pérdida de bytes en el buffer leía en
un bucle apretado, así que el buffer casi nunca se llenaba y el camino del descarte —el único que
pierde bytes— no se ejecutaba. Pasaba con el sabotaje puesto. Con un **lector lento a propósito**
(que además es el caso real) el sabotaje pierde 780.208 de 1.048.576 bytes.

## E2E

Cerebro aislado (`MUSUBI_HOME` temporal, `127.0.0.1:7801`). Sin sshd local, el doble de `ssh` usa
`script` para asignar un **pty de verdad**: así se ejercita una sesión interactiva real sin montar
un servicio de red en la máquina de nadie.

| Qué | Resultado |
|---|---|
| Sesión interactiva | Prompt, eco del pty, `\r\n` de terminal, y `tty` → `/dev/pts/1` |
| T1: `exec` + allowlist pide una shell | **Rechazado**, nombrando `shell` |
| T6: el id no es credencial | sin bearer **401** · con bearer AJENO **401** · con el bueno **200** |
| Se le quita `shell:` a `op` en caliente | El **mismo id**, la **misma persona**: **403** en ≤10 s — *«la sesión se corta acá»* |
| T7 | El segundo `open` devolvió **el mismo `session_id`** |
| Contrapresión | **1,49 MB** por un buffer de 256 KiB a un lector lento: llegó la última línea, RSS del cerebro +4,8 MB |
| Bitácora | `nas · op · cerrada · 136s`, y las columnas son `id, device_id, project_id, principal, estado, creada, vence, ultimo_trafico, cerrada, error` — **ninguna guarda lo tecleado** |

**El e2e destapó un defecto real.** Corriendo la CLI sin tty, abría la sesión y *después* fallaba
en el modo crudo — dejando una sesión huérfana. Y como sólo se permite una viva por persona y
máquina, el próximo intento durante los 15 minutos siguientes recibía esa sesión muerta: un error
de entorno inutilizaba la máquina un cuarto de hora. Se invirtió el orden (preparar la terminal
cuesta lo mismo y falla sin haber tocado nada del otro lado), con su prueba y su sabotaje.

## Lo que NO se verificó, y hay que decirlo

**Nadie corrió esto contra un `sshd` real.** El e2e usa un pty real pero un `ssh` de mentira, así
que `ssh -tt` —el `-tt`, la negociación del pty remoto, `SetEnv=LINES/COLUMNS`, el manejo de la
host key— está sin ejercitar. Es la misma clase de hueco que A3 (los colectores de Windows y macOS
cross-compilan y nadie los corrió en hardware real). Queda anotado.
