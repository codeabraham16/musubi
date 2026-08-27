# Control de flota — registro de lo ABIERTO

> **Nada queda abierto sin dueño.** Cada línea de acá tiene un slice asignado o una razón
> declarada de por qué NO se va a hacer. Si algo se cierra, se borra de esta tabla; si aparece
> algo nuevo, se anota acá el mismo día.
>
> Última revisión: **2026-08-27** (tras S6c, S7b, las dos auditorías, y el DESPLIEGUE REAL en `musubi-server`).
>
> **La regla 2 de abajo ya no depende de que alguien se acuerde**: la verifica
> `TestNingunCaboDeFlotaSeQuedaSinRegistro`. Un cabo nuevo sin número de registro rompe la suite.

---

## 1 · Con slice asignado (se va a hacer)

| # | Qué falta | Por qué no está | Slice |
|---|---|---|---|
| A1 | **CPU y memoria en macOS** | Viven detrás de **mach** (`host_processor_info`, `host_statistics64`), no de sysctl. Sin cgo hay que armar el mensaje IPC a mano: mucho código delicado, y una superficie fea en el proceso que corre en todas las máquinas. Hoy macOS mide disco, carga, uptime y CPUs. | **S4c** |
| A2 | **Temperatura en Windows** | Se saca por WMI (`MSAcpi_ThermalZoneTemperature`). WMI desde Go sin dependencias es COM crudo, y muchos equipos no exponen el sensor igual. | **S4c** |
| A3 | **Verificación en hardware real de ~~Windows~~/macOS** | Los colectores **cross-compilan y su aritmética está probada** (`cpudelta_test.go`, `sysctlparse_test.go`), pero **nadie los corrió en un Mac ni en un Windows de verdad**. La capa de syscalls está sin ejercitar. **WINDOWS: VERIFICADO (2026-08-27)** — el agente corre en `kernelos-pc` (Windows 11, 8 núcleos, 34 GB) y el colector mide CPU 12,8 %, memoria 46,7 %, disco 91,4 % y swap 28,8 % correctamente. Los dos `None` que devuelve son los huecos ya declarados y no fallas: `load1` **no existe** en Windows, y `temp_c` es **A2**. **macOS sigue bloqueado**: gio no tiene Mac por ahora. | **S4c** |
| A30 | **No hay Tier B para Windows** | El camino sin agente —que el cerebro sondee por SSH— lee `/proc`, que Windows no tiene. Así que **la única forma de medir un Windows es instalarle el agente**, con todo lo que eso arrastra: un binario sin firmar que los VPN con filtrado por proceso bloquean (medido: NordVPN devuelve `WSAEACCES` mientras `curl.exe` al mismo host y puerto da HTTP 200). Con Tier B para Windows nada de eso haría falta. Requiere un colector que hable WMI o PowerShell remoto sobre SSH. | **S7c** |
| A31 | **El binario de Windows no está firmado** | NordVPN —y cualquier EDR con filtrado por reputación— bloquea la salida de ejecutables sin firma, y el síntoma (`WSAEACCES`) no menciona ni firma ni antivirus: `curl.exe` da HTTP 200 y el binario da `WSAEACCES` al mismo host y puerto. Se sortea con una excepción de *split tunneling* **por ruta**, que se rompe si el binario se mueve. **ALCANCE MEDIDO: hoy afecta a UNA máquina** (`kernelos-pc`, la única con NordVPN, confirmado por gio 2026-08-27); en el resto el agente conecta sin nada. Por eso NO es urgente y NO justifica todavía el costo de un certificado. **Se revisa si aparece una segunda máquina con filtrado por proceso, o si se despliega fuera de la red propia** — ahí el certificado deja de ser un lujo. | **acción del operador** (cuesta plata y trámite) |
| A32 | **Las alertas no se ven en el CRM** | `public.alertas` se llena y la vista `alertas_activas` existe, pero **ninguna página las muestra**: hoy sólo se ven por Telegram o por SQL. Falta una vista en `crm-musubi.git` (Next.js, ya usa `@supabase/ssr`). **Gio decidió posponerlo el 2026-08-27**: las alertas llegan al teléfono y quedan escritas, que es lo que importa; lo visual espera a que se ordene el monitoreo (A33). | **sin asignar** (decisión de gio) |
| A33 | **Tres stacks de monitoreo mirando lo mismo** | `musubi-server` corre **OpenObserve** (`:5080`, que alimenta la sección Monitoreo del CRM), **Uptime Kuma**, y ahora **Prometheus + Alertmanager**. Los tres se agregaron por caminos distintos y nadie decidió cuál es el de verdad. Prometheus entró porque las reglas de Musubi están escritas para él y porque el dead-man's switch no existía en ninguno de los otros — pero **debí preguntar antes de montarlo**, y la duplicación es real. **Se revisa antes de agregar cualquier vista nueva de monitoreo** (bloquea A32). | **acción del operador** (es una decisión de producto, no técnica) |
| A14 | **Grabación de sesión de pantalla** | Decisión legal antes que técnica; no se hace sin que alguien la tome. | sin asignar |
| A18 | **Pantalla en Android** (scrcpy sobre ADB) | La matriz de S1 concede `screen` a Tier C, pero el motor es otro distinto del de RustDesk. **Su sombra ya está tapada (S6c)**: pedir la pantalla de un Tier C se NIEGA y la capacidad inerte se ve en el inventario y en el panel. Falta el motor. | **S8b** |
| A20 | **iOS: medir o controlar** | Requiere un MDM con perfil de supervisión — un producto entero. Musubi lo tiene en el inventario y lo dice. **Puede que nunca se haga, y está bien.** | sin asignar |
| A17 | **SNMP / MQTT / Redfish** | Los tres piden una librería (dependencia nueva) o un protocolo binario a mano. SSH cubre routers, NAS, Raspberry Pis y servers sin agente — la mayoría de lo que hay. | **S7c** |
| A26 | **`musubi shell` no funciona desde Windows** | El modo crudo se pide con `stty`, que no existe en la consola de Windows (ahí es `SetConsoleMode`). Desde Linux o macOS sí, contra cualquier Tier B. | **S5d** |
| A27 | **La ventana no se redimensiona (SIGWINCH)** | No es «no se hizo»: el transporte elegido no lo permite. En Tier B el pty lo posee el `sshd` remoto y en Tier A lo posee `script`, así que **no tenemos su descriptor maestro** y no hay a quién mandarle un `TIOCSWINSZ`. El tamaño se fija al abrir. **Medido contra un `sshd` real (S7b)**: el `ioctl` del pty remoto da `0 0`, pero `tput` devuelve 24/80 y `top` dibuja — el fallback por `LINES`/`COLUMNS` alcanza para lo que se usa. Si el redimensionado importa de verdad, obliga a escribir el pty a mano —ioctls por OS y por arquitectura— y entonces se paga entero. | **S5d** |
| A29 | **La cadena de alertas nunca se desplegó** | Se descubrió mirando `musubi-server` (100.79.126.62): el cerebro está vivo y sirviendo `/metrics`, y **nadie lo scrapea**. No hay Prometheus (el 9090 es **Cockpit**) ni Alertmanager. O sea que las 9 reglas de S4b, las de políticas de S10 y el dead-man's switch estaban escritos, probados e **INERTES**. El registro no lo vio porque cubre CÓDIGO y esto es DESPLIEGUE. Ya existe lo que faltaba: `deploy/docker/` (compose + `preparar.sh` + receta verificable). | **acción del operador** (`deploy/docker/README.md` ①) |
| A22 | **El otro lado del dead-man's switch** | `MusubiSiempreViva` late hacia un receptor `watchdog`, pero hace falta un servicio EXTERNO que espere ese ping y grite si falta (Healthchecks.io, Dead Man's Snitch, o un cron en otra máquina). **No era media alarma: era NINGUNA**, y el motivo anotado acá tapaba algo más grande (ver A29). Es el eslabón 4 de 4. | **acción del operador**, después de A29 (`deploy/docker/README.md`) |

## 2 · Decisiones de NO hacer (revisables, no pendientes)

| # | Qué | Por qué no |
|---|---|---|
| B1 | **`gopsutil`** | Daría los tres OS de una y sería la **7ª dependencia directa** de un repo que tiene 6 y un `observability.go` escrito a propósito con «cero dependencias nuevas». Se prefirió el seam + un colector honesto por OS. **Se revisa si aparece un cuarto OS.** |
| B2 | **Selectores por tag en los grants** | Sólo `["*"]` o nombres. Agrupar por tag es tentador y **no hay todavía un caso real** que lo pida. Entra cuando lo haya, con su prueba. |
| B3 | **Tools para administrar los grants por red** | Las concesiones se editan en `principals.yaml`, que ya recarga en caliente (≤10 s). Una tool para **otorgarse capacidades a uno mismo** por la red merece más cuidado que un slice de fundación. |
| B4 | **Métricas por proceso y por interfaz de red** | El agregado del host primero. El detalle, cuando haya una pregunta que lo pida. |
| B5 | **Tabla de series temporales en SQLite** | Musubi guarda el **presente**; la historia la guarda Prometheus. 40 máquinas cada 30 s son 115.000 filas diarias que nadie consulta salvo para graficar. |
| B6 | **Relay público por default** | Sólo por device marcado (acceso híbrido). |
| B7 | **Condiciones de política como EXPRESIÓN** | El `when:` es un enum acotado sobre los campos de la muestra, no un mini-lenguaje. Un evaluador de expresiones que decide qué comando correr en una máquina ajena es una superficie que no se justifica todavía. **Se revisa cuando haga falta una condición compuesta real** (p. ej. «disco bajo Y uptime alto»), no antes. |
| B8 | **Acciones de política que no sean un comando** | Nada de webhooks ni de «apagar la máquina» como primitiva: todo lo que hace una política es *un exec que ya podrías haber hecho a mano*, con la autoridad de alguien y en la misma bitácora. Cada acción nueva es un camino de autoridad nuevo que habría que compuertar por separado. **Se revisa si aparece un caso que el exec no cubra.** |
| B12 | **Verificar el `rustdesk_id` contra el relay** | Era el plan de A13 y **no es viable ni serviría**: hbbs (el relay OSS) no expone API para eso —habría que hablarle su protobuf, o sea reimplementar medio cliente— y aunque la expusiera sólo diría qué CONEXIÓN reclama ese id ahora, no cuál de nuestras máquinas es. Se cerró A13 por el otro lado: detectando la COLISIÓN, que es la firma del ataque y además el caso benigno frecuente (imágenes clonadas). **Queda sin cubrir** una máquina que declare un id que no colisiona con ninguna de las nuestras; de ésa se ve que el id CAMBIÓ. **Se revisa si RustDesk Pro o un hbbs con API entran en el despliegue.** |
| B10 | **Grabación del contenido de una sesión de shell** | Misma decisión legal que A14 (grabación de pantalla) y mismo dueño: nadie. `SesionShell` no tiene dónde guardarlo, y hay una prueba de FORMA que custodia esa ausencia — la única manera de proteger algo que no existe. **Se revisa si alguien toma la decisión, no antes.** |
| B11 | **Reconectar a una sesión de shell viva** | Si el relay se corta, la sesión MUERE: no queda un proceso huérfano esperando que alguien vuelva. Reconectar sale caro (hay que retener la salida de un cliente ausente, con todo lo que eso implica para la contrapresión) y su beneficio es comodidad. **Se revisa si las desconexiones resultan frecuentes en uso real.** |
| B17 | **«Flota» significa dos cosas distintas en el mismo servidor** | La sección **Flota** del CRM inventaría *bots, puentes y servicios*, publicada a mano con `flota publicar` y leída de un archivo. La **flota** de Musubi son *máquinas midiéndose solas*. Comparten nombre y no comparten nada más, así que en algún momento alguien va a mirar una creyendo que es la otra. **No se renombra todavía** porque tocar la barra lateral del CRM es decisión de gio y el costo de la confusión hoy es bajo (un solo usuario, que sabe la diferencia). **Se revisa el día que alguien más use el CRM.** |
| B13 | **Probar contra un `sshd` con PAM, contraseñas o `ForceCommand`** | El `sshd` de S7b corre **sin root y sin PAM a propósito** —es lo que permite levantarlo sin instalar nada— así que esa rama queda sin ejercitar. Musubi nunca manda contraseñas (`BatchMode=yes`), y `ForceCommand` rompería cualquier RMM por igual. **Se revisa si aparece un host que las exija.** |
| B14 | **Probar contra un host REMOTO de verdad** | S7b fue loopback: sin latencia, sin MTU, sin cortes de red a mitad de una shell. Lo que sí cubre —`-tt`, host key, puerto, pty, cierre— no depende de la distancia. **Se revisa cuando haya un Tier B real enrolado.** |
| B15 | **Otras implementaciones de servidor SSH** | dropbear, el `sshd` de un router, el OpenSSH de Windows. Todas hablan el mismo protocolo pero difieren en `AcceptEnv` y en la shell de login, que es justo donde apareció el `--` de más. **Se revisa cuando entre el primer aparato que no sea OpenSSH de Linux.** |
| B16 | **`exec` en Tier C** | La matriz de S1 no se lo concede y sigue sin concedérselo: en Android depende de que ADB esté habilitado, y prometerlo al enrolar sería mentir. Un móvil da métricas y (con S8b) pantalla; una shell no. **Se revisa si aparece una flota de Androids con ADB garantizado por MDM.** |
| B9 | **Alertas por-tenant** | Las reglas de flota se evalúan sobre las series que la credencial del scrape puede ver, así que un despliegue con varios tenants necesitaría un Prometheus (o un principal) por tenant. Hoy hay uno. **Se revisa el día que dos tenants compartan cerebro y no quieran compartir alertas.** |

## 3 · Cerrado en este track (para no volver a abrirlo por olvido)

S1 registro · S2 agente + las dos puertas · S3 la compuerta de tres lados · S4 telemetría Linux ·
S4b export a Prometheus + autorreporte + `README.en.md` + 9 reglas de alerta ·
S4c **colectores de Windows y macOS** (compilan en 6 combinaciones OS/arch) ·
**S5 ejecución remota auditada** (one-shot, con bitácora permanente y salida que caduca) ·
**S7 Tier B por SSH** (sin guardar credenciales: se invoca al ssh del sistema) ·
**S8 Tier C y la sonda remota** (parseo de /proc compartido por las tres fuentes; el techo de iOS declarado) ·
**S9 panel de flota** (página aparte del bundle WebGL; el estado viaja siempre; `—` nunca es `0 %`) ·
**S6 pantalla sobre RustDesk self-hosted** (contraseña por sesión que Musubi NUNCA guarda, vencimiento
aplicado por el agente, relay como systemd atado al tailnet). **FASE 1 COMPLETA.**

**S10 alertas y políticas** — cierra CINCO cabos de una vez, porque los cinco eran la misma
pregunta: qué hace el sistema cuando nadie está mirando.
· **A11** la poda cuelga del latido propio de la flota, no del mantenimiento de la memoria
· **A19** sondeo automático — y con él el **umbral de «en línea» POR TIER**, sin el cual el sondeo
  no arreglaba nada: un Tier B sondeado cada 5 min seguía figurando caído el 97 % del tiempo
· **A12** allowlist de comandos, en la CREDENCIAL y no en el aparato, exhaustiva una vez presente
· **A10** políticas de auto-heal que **no tienen autoridad propia**: actúan con la de un principal,
  por la misma compuerta y a la misma bitácora, y se apagan solas si se lo revoca
· **A4** Alertmanager con dead-man's switch — el silencio deja de ser indistinguible de la sordera.
**FASE 2 COMPLETA.**

**S9b + S10b — la deuda de S10, atendida antes de abrir nada nuevo.**
· **A24** el cooldown de las políticas sobrevive un reinicio (migración 33). Vivía sólo en
  memoria, y reiniciar es lo primero que alguien hace justo cuando las políticas están disparando
· **A23** el auto-heal se VE: `politicas_activas` para todos, el detalle con `exec`, y sobre todo
  **`puede_actuar`** — una política inerte se veía idéntica a una que funciona
· **A21** se llega a `/flota` desde el panel del cerebro y se vuelve. **Sin tocar el bundle**: el
  motivo anotado acá («habría que tocar el bundle WebGL») era incorrecto — la CI compara los
  bytes de `dashboard.bundle.js`, no los de la cáscara `dashboard.html`
· **de yapa**, un hueco silencioso que destapó el e2e: un `principals_file` RELATIVO se resolvía
  contra el CWD del proceso, y un archivo ausente NO falla —cae a modo legacy—, así que un
  servicio con otro `WorkingDirectory` degradaba toda la identidad por-miembro a un solo bearer
  admin-federado sin decir nada. Ahora cuelga del workspace.

**S6b la procedencia del `rustdesk_id`** — cierra A13, y **por un camino distinto al anotado**:
verificar contra el relay no era viable ni habría servido. Lo que sí ataca el caso real es la
COLISIÓN —dos máquinas diciendo ser la misma pantalla—, que es a la vez la firma del ataque y el
caso benigno más frecuente (imágenes clonadas: RustDesk deriva su id de la máquina). La pantalla
**se niega** en vez de avisar, la colisión se mira globalmente **sin nombrar lo ajeno**, y el
cambio de id queda escrito con su valor anterior.

**S5c shell en Tier A + el puerto de un Tier B** — cierra A25. A un Tier A **no le entra nadie**
(NAT, sin puertos abiertos), así que el canal es un ENCUENTRO: el cerebro deja la sesión esperando
y avisa por la cola de comandos; el agente se conecta desde su lado y abre el pty con `script`.
La guarda que importa es **«¿esta sesión es de ESTA máquina?»**: por ese canal viajan las teclas,
contraseñas de sudo incluidas. Y de paso, un hueco que apareció al empezar: **`gio@nas:2222` era
inalcanzable** — ssh no entiende esa forma y el error mandaba a depurar el DNS de un host sano.

**S5b shell interactiva (Tier B)** — cierra A5. La decisión que sostiene el slice: **`shell` es
una CUARTA capacidad y no se implica nunca de `exec`**. S10 había partido `exec` en «poder
ejecutar» y «poder ejecutar cualquier cosa»; una shell interactiva es el tercer permiso y se lleva
puestos a los otros dos, así que gatearla con `exec` habría vuelto decoración la allowlist por
comando. Además: el **id de sesión NO es una credencial** (cada request re-autoriza entero, así
que revocar corta el prompt abierto), dos techos que aplica el cerebro (vida e inactividad),
**contrapresión** en el buffer —la única salida que ni tumba el cerebro ni garglea la terminal—,
y la bitácora registra QUE hubo acceso sin guardar QUÉ se tecleó. Tier B por `ssh -tt`, que pone
el pty del lado remoto y evita todo syscall de pty. Modo crudo con el `stty` del sistema.

**S6c la pantalla que no tenía motor** — tapa la sombra de A18, que era un hueco **ACTIVO** y no
una tarea futura. La matriz le concede `screen` a Tier C y hace bien —un móvil TIENE framebuffer—
pero `methods_pantalla` sólo habla RustDesk, así que un Android pasaba la autorización, pasaba «en
línea», **acuñaba la contraseña, la mostraba la única vez que se muestra**, y encolaba el comando
en una cola que en Tier C no drena nadie. Faltaba una segunda pregunta: «¿este tier sabe honrar
`screen`?» la contesta la matriz; «¿y Musubi tiene con qué?» no la contestaba nadie. Ahora sí
(`fleet.MotorDePantalla`), **se niega antes de acuñar**, y la capacidad inerte se ve en el
inventario y en el panel en vez de dibujarse igual que una viva.

**S7b la shell y el exec contra un `sshd` DE VERDAD** — cierra A28, y la razón anotada para no
hacerlo era **medio incorrecta**: es cierto que no hay `openssh-server` y que instalarlo es del
operador, pero se puede bajar el `.deb` sin `sudo`, extraerlo y correr un `sshd` **sin privilegios**
en loopback, que sólo acepta al usuario que lo corre. Y ahí estaba esperando el bug: un `--` de más
después del host llegaba a la shell remota como parte del comando (`bash: --: invalid option`), así
que **TODOS los exec de Tier B fallaban** — S7 nunca funcionó contra un servidor real. Ninguna
prueba lo vio porque todas usan un `ssh` de mentira **que nunca corre una shell**. La prueba nueva
corta los argumentos en el destino, los junta con espacios y los corre por una shell real: es lo
que hace el sshd. Verificado además `-tt`, el pty remoto, la host key estricta, el `-p` del puerto,
`SetEnv=LINES/COLUMNS` y el cierre sin huérfanas. Es la **tercera** razón anotada que envejeció mal
(van A21, A13 y A28): los motivos también caducan, y este archivo hay que releerlo, no sólo apendarlo.

**Auditoría del propio registro** — se aplicó al archivo la regla que el archivo manda aplicar a
los specs, y **no se cumplía**. Nueve ítems declarados «fuera de alcance» ya se habían hecho en
slices posteriores: el enlace del panel, la poda de salidas, el export a Prometheus, los colectores
de Windows y macOS, las métricas de Tier B, el autorreporte de versión y dirección, los build tags,
las tools MCP y la CLI. Un spec que dice «esto no está» sobre algo que SÍ está es peor que un
pendiente: quien lo lee decide con información falsa. Los nueve quedaron tachados con el slice que
los cerró; dos que nunca tuvieron número lo tienen (**B16** `exec` en Tier C); y cada cabo vivo
**nombra su número de registro**, que es lo que vuelve el barrido auto-verificable.

Y la regla pasó a ser una prueba, con tres sabotajes: un cabo nuevo sin registro la rompe, vaciar
las tablas de ABIERTO.md la rompe, y —el modo de fallo peligroso— que el barrido deje de encontrar
los specs y pase vacío EN VERDE también la rompe.

**Auditoría del DESPLIEGUE** — la del registro miró los specs; ésta miró la máquina, y encontró el
hueco más grande del track: **la cadena de alertas no existía**. El cerebro exponía `/metrics` y
nadie lo scrapeaba, así que todo lo construido en S4b y S10 para vigilar estaba inerte. Dos
lecciones quedan: que `deploy/` tenía la CONFIGURACIÓN de las alertas y ningún camino de despliegue
para ellas (el cerebro sí tenía instalador; sus alertas no), y que **un registro de cabos que sólo
cubre código deja pasar la mitad del sistema**. Se agregó `deploy/docker/` y dos guardas que
custodian lo que se rompe solo: que los TRES archivos que fijan el puerto de Prometheus coincidan
—si divergen, `up{job="prometheus"}` queda DOWN y falla el instrumento que mide el fallo— y que no
vuelvan al **9090, que es de Cockpit**: ahí el riesgo no es que no arranque, es que alguien vea una
UI y crea que Prometheus anda.

**Despliegue real (2026-08-27)** — el track dejó de ser código y pasó a correr. Cerebro
`0.108.0-flota` en `musubi-server` (Rocky 10), migración 28→35 ensayada antes contra una copia de
la base de producción, dos máquinas enroladas —una Linux, una Windows— y las 19 alertas evaluando
contra Telegram. **Cierra la mitad Windows de A3** y abre **A30** y **A31**, que sólo se ven
cuando el agente tiene que instalarse en una máquina ajena de verdad.

---

## Cómo se usa este archivo

1. Al cerrar un slice, **borrar** su línea de la tabla 1.
2. Al declarar algo fuera de alcance, **agregarlo** a la tabla 1 (con slice) o a la 2 (con razón).
   Un `## Lo que queda fuera` en un spec que no aparezca acá es un cabo suelto de verdad.
3. La tabla 2 no es un cementerio: cada línea dice **bajo qué condición se revisa**.
4. **Este archivo cubre el track «Control de flota», no el repo entero.** Otros tracks tienen sus
   propios `## Lo que queda fuera` —`specs/riel-local/` es el que hoy tiene ítems vivos sin
   registro propio— y NO están acá. Leer este archivo como «todo lo abierto de Musubi» sería el
   mismo error que dio origen a la tabla: creer cubierto lo que nadie miró.
5. **Los MOTIVOS también caducan.** Tres de los anotados acá resultaron falsos al ir a mirar
   (A21 «habría que tocar el bundle», A13 «verificar contra el relay», A28 «no se puede sin
   instalar un servidor»). Antes de dar por bueno un «no se hizo porque X», verificá X.
