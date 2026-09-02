# Control de flota — registro de lo ABIERTO

> **Nada queda abierto sin dueño.** Cada línea de acá tiene un slice asignado o una razón
> declarada de por qué NO se va a hacer. Si algo se cierra, se borra de esta tabla; si aparece
> algo nuevo, se anota acá el mismo día.
>
> **Última revisión: 2026-09-02 (auditoría del propio registro, la segunda).** Se le aplicó a este
> archivo la regla que el archivo le exige a los specs, y **no se cumplía en cuatro lugares**.
>
> **(1) A33 se citaba dos veces como decisión pendiente y no tenía fila en ninguna tabla** — justo
> lo que la regla 2 prohíbe. No hubo que adivinar dónde fue a parar: el mismo commit (`defb268`,
> 2026-08-29) borró la fila de A33 y agregó en su lugar la de «cuál de los cinco sistemas de
> monitoreo manda», que hoy es **B20** y lleva su «(era A33)». Las dos citas apuntan ahí.
>
> **(2) Tres números B estaban repetidos**: `B13` nombraba tres decisiones distintas y `B14` dos.
> Un número repetido es peor que uno faltante —«se revisa en B13» dejaba de identificar nada— y
> además es invisible al leer, porque las filas están lejos una de otra. Quedaron **B18**, **B19**
> y **B20**, y cada una dice por qué le tocó ceder el número.
>
> **(3) A70 no tenía entrada de cierre propia.** Su trabajo estaba contado (2026-09-02 bis) pero
> sin el número, y el número vive en **seis archivos de código**: quien seguía la pista desde el
> código no llegaba a ninguna parte. Ahora la entrada lo nombra.
>
> **(4) Las fases del `proposal.md` y las de este registro no coinciden**, y **«fase 3» no existe
> en ningún archivo del repo**. Queda declarado en **B21**, con el mapa entre las dos numeraciones.
>
> Y se anotaron **dos cabos que la auditoría encontró sin registrar**: **A74** (la contraseña de una
> sesión de pantalla queda EN CLARO en `device_commands` — tapada en toda superficie de lectura,
> pero no en la tabla) y **A75** (el eje de consentimiento sólo lo consulta el camino de pantalla:
> `exec` y `shell` no lo miran). Con eso son **17 cabos abiertos**, todos con dueño o razón.
>
> **Y esa misma tarde se anotaron dos más, que salieron de construir el mapa de cobertura**:
> **A76** (los contenedores de una máquina Windows son invisibles para la flota — 11 corriendo en
> `davantis-1` y ninguno reportado, con dos rotos hacía días) y **A77** (dos máquinas del tailnet
> no están enroladas, y una de ellas corría un binario veintitrés versiones atrás sin que nada lo
> dijera). Con eso son **19 cabos abiertos**, todos con dueño o razón declarada.
>
> **Y esa misma tarde, uno más: A78**, encontrado leyendo el código mientras se perseguía por qué
> `davantis-1` llevaba 129 minutos sin reportar servicios. Un inventario VACÍO deja a la máquina
> muda para siempre mientras el agente cree que reportó — las dos mitades de la condición se
> contradicen en silencio. **No quedó probado que fuera la causa de ese caso** (la máquina se cayó
> entera antes de poder medirlo), y se anota como lo que es: un modo de falla del código. Con eso
> son **20 cabos**.
>
> **Y a las 11:48 se cerró A69 por decisión de gio**: su PC volvió al servidor público de
> RustDesk, que era la opción (c) de las tres anotadas. Quedan **19**.
>
> **Y la regla 2 sumó dos guardas**, porque estas tres cosas se rompieron a mano y a mano se iban a
> volver a romper: en `internal/mcp/specs_sin_cabos_test.go`, un número repetido en las tablas rompe
> la suite, y un cabo VIVO que apunte a un número que el registro no define en ningún lado, también.
>
> Revisión anterior del mismo día: se cerraron **A68** (un agente atrasado ya avisa) y **A73**
> (el repo y producción se comparan, a pedido y desatendido). Con eso **no queda ningún cabo sin
> dueño ni sin una razón declarada**: los 15 que quedaban abiertos tenían slice, operador, o una
> decisión de gio anotada. **A72 se cerró el mismo día**: el disco de `davantis-1`
> pasó de 6,8 % a 20,1 % libre.
>
> Revisión anterior: **2026-08-29**, tras una tanda larga: **A44** (políticas sobre la salud de un
> servicio), los **seis cabos de la misma familia** —A38, A39, A49, A50, A51, A52: código cuyo modo
> de falla no se veía desde ninguna prueba—, **A54** (el agente declara lo que va a tocar y el
> despliegue lo verifica) y **A45 + A53** (`go test -race ./...` vuelve a terminar: 8m12s contra
> «nunca en 30 minutos»). Se cerraron además **A56** (verificado en producción) y **A22 → B13**
> (gio despriorizó el watchdog externo, y una decisión tomada no puede seguir figurando como
> pendiente).
>
> **2026-08-29 (cierre del día)**: cerrados además **A56**, **A57**, **A58** y la **fase 4** entera, y abierta
> la **fase 5** con sus dos primeros slices (**S13 · la cronología** y **S14 · el cruce con la memoria**), que dejó **A59**, **A60** y **A61** anotados el mismo día.
> Quedan **15 cabos**, y **ninguno sin dueño o sin una razón declarada de por qué no se hace**.
>
> **A59 se abrió y se cerró el mismo día**: la columna `origen` (migración 41) hace que la cronología
> pueda decir qué disparó una regla y qué pidió una persona.
>
> **La regla 2 de abajo ya no depende de que alguien se acuerde**: la verifica
> `TestNingunCaboDeFlotaSeQuedaSinRegistro`. Un cabo nuevo sin número de registro rompe la suite.

---

## 1 · Con slice asignado (se va a hacer)

| # | Qué falta | Por qué no está | Slice |
|---|---|---|---|
| A1 | **CPU y memoria en macOS** | Viven detrás de **mach** (`host_processor_info`, `host_statistics64`), no de sysctl. Sin cgo hay que armar el mensaje IPC a mano: mucho código delicado, y una superficie fea en el proceso que corre en todas las máquinas. Hoy macOS mide disco, carga, uptime y CPUs. | **S4c** |
| A2 | **Temperatura en Windows** | Se saca por WMI (`MSAcpi_ThermalZoneTemperature`). WMI desde Go sin dependencias es COM crudo, y muchos equipos no exponen el sensor igual. **LA PREMISA CAMBIÓ el 2026-09-02 y conviene anotarlo antes de que se olvide**: desde **A42** el agente de Windows YA lanza PowerShell en cada latido —y desde **A70** lo que pide en esa invocación es `Get-CimInstance Win32_Service`, para los servicios—, así que «COM crudo» dejó de ser el bloqueo. El costo real hoy es otro y es menor: pedir la zona térmica en la MISMA invocación (acoplando dos colectores) o pagar un segundo `powershell` cada 30 s en una máquina ajena. Lo que no cambió es la otra mitad de la nota: muchos equipos no exponen el sensor, así que la serie va a estar AUSENTE en buena parte de la flota — y eso ya está bien manejado (`temp_c` es un puntero). | **S4c** |
| A3 | **Verificación en hardware real de ~~Windows~~/macOS** | Los colectores **cross-compilan y su aritmética está probada** (`cpudelta_test.go`, `sysctlparse_test.go`), pero **nadie los corrió en un Mac ni en un Windows de verdad**. La capa de syscalls está sin ejercitar. **WINDOWS: VERIFICADO (2026-08-27)** — el agente corre en `kernelos-pc` (Windows 11, 8 núcleos, 34 GB) y el colector mide CPU 12,8 %, memoria 46,7 %, disco 91,4 % y swap 28,8 % correctamente. Los dos `None` que devuelve son los huecos ya declarados y no fallas: `load1` **no existe** en Windows, y `temp_c` es **A2**. **macOS sigue bloqueado**: gio no tiene Mac por ahora. | **S4c** |
| A30 | **No hay Tier B para Windows** | El camino sin agente —que el cerebro sondee por SSH— lee `/proc`, que Windows no tiene. Así que **la única forma de medir un Windows es instalarle el agente**, con todo lo que eso arrastra: un binario sin firmar que los VPN con filtrado por proceso bloquean (medido: NordVPN devuelve `WSAEACCES` mientras `curl.exe` al mismo host y puerto da HTTP 200). Con Tier B para Windows nada de eso haría falta. Requiere un colector que hable WMI o PowerShell remoto sobre SSH. | **S7c** |
| A31 | **El binario de Windows no está firmado** | NordVPN —y cualquier EDR con filtrado por reputación— bloquea la salida de ejecutables sin firma, y el síntoma (`WSAEACCES`) no menciona ni firma ni antivirus: `curl.exe` da HTTP 200 y el binario da `WSAEACCES` al mismo host y puerto. Se sortea con una excepción de *split tunneling* **por ruta**, que se rompe si el binario se mueve. **PRUEBA LIMPIA (2026-08-27)**: el MISMO binario, en la MISMA máquina, dio `WSAEACCES` como `musubi-nuevo.exe` y latió sin problema medio minuto después como `musubi.exe` — la excepción de NordVPN es por RUTA, y eso quedó medido en vez de deducido. **ALCANCE MEDIDO: hoy afecta a UNA máquina** (`kernelos-pc`, la única con NordVPN, confirmado por gio 2026-08-27); en el resto el agente conecta sin nada. Por eso NO es urgente y NO justifica todavía el costo de un certificado. **Se revisa si aparece una segunda máquina con filtrado por proceso, o si se despliega fuera de la red propia** — ahí el certificado deja de ser un lujo. | **acción del operador** (cuesta plata y trámite) **Medido de nuevo el 2026-09-01, y el alcance es mayor de lo que decía esta nota**: desde `davantis-1`, PowerShell **no alcanza ni el puerto del cerebro** (`7717 = False`) mientras el agente le late cada 30 s. O sea que la autorización es **por ARCHIVO**, no por aplicación ni por carpeta: en esa máquina lo único que habla con el tailnet es `musubi.exe` en su ruta exacta, más RustDesk desde que se excluyó. La consecuencia operativa es concreta: **toda administración remota que no sea el propio agente está cortada ahí** —no se puede descargar un archivo, ni sondear un puerto, ni probar conectividad— y cualquier medición hecha con PowerShell sobre esa máquina mide otra cosa. Costó dos diagnósticos equivocados el mismo día. `gio`, sin VPN, se actualizó entera por el canal de comandos en 40 segundos. |
| A32 | **Las alertas no se ven en el CRM** | `public.alertas` se llena y la vista `alertas_activas` existe, pero **ninguna página las muestra**: hoy sólo se ven por Telegram o por SQL. Falta una vista en `crm-musubi.git` (Next.js, ya usa `@supabase/ssr`). **Gio decidió posponerlo el 2026-08-27**: las alertas llegan al teléfono y quedan escritas, que es lo que importa; lo visual espera a que se ordene el monitoreo (era **A33**, hoy **B20**). **Ojo con esa espera**: la decisión se tomó el 2026-08-29 y su consecuencia (d) dice que **desbloquea A32**, así que lo que sigue pendiente acá es la PÁGINA, no la decisión. | **sin asignar** (decisión de gio) |
| A14 | **Grabación de sesión de pantalla** | Decisión legal antes que técnica; no se hace sin que alguien la tome. | sin asignar |
| A18 | **Pantalla en Android** (scrcpy sobre ADB) | La matriz de S1 concede `screen` a Tier C, pero el motor es otro distinto del de RustDesk. **Su sombra ya está tapada (S6c)**: pedir la pantalla de un Tier C se NIEGA y la capacidad inerte se ve en el inventario y en el panel. Falta el motor. | **S8b** |
| A20 | **iOS: medir o controlar** | Requiere un MDM con perfil de supervisión — un producto entero. Musubi lo tiene en el inventario y lo dice. **Puede que nunca se haga, y está bien.** | sin asignar |
| A17 | **SNMP / MQTT / Redfish** | Los tres piden una librería (dependencia nueva) o un protocolo binario a mano. SSH cubre routers, NAS, Raspberry Pis y servers sin agente — la mayoría de lo que hay. | **S7c** |
| A26 | **`musubi shell` no funciona desde Windows** | El modo crudo se pide con `stty`, que no existe en la consola de Windows (ahí es `SetConsoleMode`). Desde Linux o macOS sí, contra cualquier Tier B. | **S5d** |
| A27 | **La ventana no se redimensiona (SIGWINCH)** | No es «no se hizo»: el transporte elegido no lo permite. En Tier B el pty lo posee el `sshd` remoto y en Tier A lo posee `script`, así que **no tenemos su descriptor maestro** y no hay a quién mandarle un `TIOCSWINSZ`. El tamaño se fija al abrir. **Medido contra un `sshd` real (S7b)**: el `ioctl` del pty remoto da `0 0`, pero `tput` devuelve 24/80 y `top` dibuja — el fallback por `LINES`/`COLUMNS` alcanza para lo que se usa. Si el redimensionado importa de verdad, obliga a escribir el pty a mano —ioctls por OS y por arquitectura— y entonces se paga entero. | **S5d** |
| A37 | **La identidad del relay sólo está a salvo de la mitad de las cosas** | `~/musubi-rustdesk/data/id_ed25519` es la identidad del relay: si se pierde, el relay vuelve con OTRA clave y **todos los clientes de la flota dejan de conectar hasta que alguien los reconfigure uno por uno**. **Media parte resuelta (2026-08-27)**: `preparar.sh` deja una copia en `.musubi/backups/rustdesk-relay/` con permiso 0600 —más cerrado que el 0644 del original— y un `LEEME.txt` con el procedimiento de restauración. Eso cubre que el volumen se borre, un `preparar.sh` mal corrido, o que el contenedor se lleve el archivo. **Lo que sigue abierto es lo otro**: la copia vive en el MISMO disco, y el backup del cerebro **sigue siendo local-only** por decisión de gio del 2026-08-27. Contra perder el host no protege nada, y eso está dicho en la salida del script y custodiado por una prueba — un respaldo que no aclara contra qué NO protege es peor que ninguno, porque alguien deja de buscar el de verdad. | **acción del operador** (③ · `BACKUP_REMOTE`) |
| A41 | **El empuje no tiene backoff con memoria entre ticks** | Un destino caído se reintenta cada 30 s para siempre, con el mismo intervalo. No hay outbox ni espaciado creciente: el reintento ES el próximo tick. Está acotado a propósito —el aviso de un fallo permanente sale UNA vez y `musubi_push_failures_total` cuenta el resto—, así que el costo real de un destino muerto es un POST fallido cada 30 s contra loopback. **Se revisa si el destino alguna vez deja de ser loopback**: contra un collector remoto, reintentar sin espaciar es exactamente cómo se martilla a alguien que ya está caído. | **sin asignar** (después de que el push tenga un destino remoto) |



| A74 | **La contraseña de una sesión de pantalla queda EN CLARO en la base** | Tiene que llegar a la máquina de alguna forma, y el camino es el `argv` de la operación interna `musubi:pantalla` (`internal/mcp/methods_pantalla.go`), que `device_commands` guarda **tal cual** (`internal/memory/comandos.go`). **Toda superficie de LECTURA la tapa** —bitácora, cronología, inventario— con `fleet.ArgvDeBitacora`, que vive en el dominio justamente para que ninguna superficie nueva se la olvide, y hay pruebas que lo custodian: quien mira Musubi por sus tools ve `['musubi:pantalla','<id>','[oculto]']` y nunca el secreto. **Lo que NO está declarado en ningún spec es el riesgo residual**, y es doble: quien pueda leer el archivo `.db` —o un backup, que es local-only por A37— la lee sin pasar por ninguna compuerta; y **no caduca**, porque `PodarSalidasDeComandos` vacía `stdout`/`stderr` y **no toca `argv`**, así que cada contraseña de sesión queda en la tabla para siempre. Su propio comentario dice que la salida «puede traer una clave en un log» — la de pantalla está en la columna de al lado y no se poda. Encima `specs/flota-pantalla-rustdesk/tasks.md` afirma en su verificación end-to-end «EL ARCHIVO .db ENTERO ✓ no está», y para la fila del comando eso hoy no es cierto. **El daño está acotado y conviene decirlo**: el agente vence la contraseña por su cuenta (G2), así que una vieja no abre ninguna sesión; lo que queda es un registro histórico de secretos que G1 promete no tener. Las dos salidas son de gio: **podar el `argv` de pantalla** cuando el comando termina (la poda ya existe, le falta la columna), o **declarar el riesgo residual** en el spec y dejarlo como está. | **sin asignar** (decisión de gio) |

| A75 | **El consentimiento no cubre `exec` ni `shell`** | El eje —cuatro grados ordenados, `libre` < `avisa` < `pide` < `prohibido`, default `avisa`— se resuelve en `Device.ConsentimientoEfectivo` y **lo consulta un solo camino: el de pantalla** (`internal/mcp/methods_pantalla.go`, dos veces: antes de acuñar la sesión y para preguntar). `musubi_fleet_exec` y `musubi_fleet_shell` **no lo invocan**, así que en una máquina declarada `pide`, mirar la pantalla pregunta y **abrir una shell no**. La asimetría no está escrita en ningún lado, y hay dos señales de que no fue deliberada: el comentario de `ConsentimientoEfectivo` justifica vivir en el dominio para que «el de pantalla y el de shell» no discrepen —como si los dos lo usaran—, y la nota que abrió el eje lo modela sobre MeshCentral, que sí tiene los gemelos para terminal. **No se cablea de una, y por eso es decisión y no bug**: prender el eje sobre `exec` cambia el comportamiento de máquinas ajenas —una política de auto-heal que hoy corre sola pasaría a esperar a una persona que quizá no está enfrente—, y `avisa` en cada exec convierte el aviso en ruido, que es exactamente cómo se enseña a poner `libre` en todo y a apagar el eje entero. Lo que hay que decidir: **qué le debe Musubi a quien está sentado en la máquina cuando lo que se abre es una shell y no una pantalla**, y si la respuesta cambia según el `origen` del comando (persona o política), que la migración 41 ya distingue. | **sin asignar** (decisión de gio) |

| A76 | **Los contenedores de una máquina Windows son invisibles para la flota** | El agente enumera contenedores desde A42 y los reporta como servicios — pero **sólo en Linux**, con `podman ps`. La asimetría se midió el 2026-09-02: `musubi-server` reporta 57 servicios de los cuales **14 son contenedores**; `davantis-1` reporta 64 y **ninguno**, con **11 contenedores de Docker Desktop corriendo** (el Supabase local de `altura-erp`). **El costo ya se pagó ese mismo día**: `supabase_vector_altura-erp` llevaba días en bucle de reinicio y `edge-runtime` había muerto hacía tres con código 255. Se encontraron **con la mano**, buscando espacio en disco — ninguna alerta existía, porque la serie no existe. Es el mismo colector con otra fuente (`docker ps` en vez de `podman ps`), y en cuanto los reporte, `ServicioCaido` y `ServicioReiniciandose` empiezan a cubrirlos sin escribir una regla nueva. **Ojo con el segundo**: hoy está declarado `ausente_en: os=windows` porque el SCM no expone reinicios, y un contenedor SÍ los expone — esa declaración va a necesitar afinarse a «servicios del SCM» cuando esto entre. | **sin asignar** |
| A77 | **Dos máquinas del tailnet no están en la flota, y una ya costó** | El tailnet tiene cinco nodos y la flota cuatro devices: **`davantis` (esta máquina, la de desarrollo) y `raspberrypi` no están enroladas**. No es una omisión inocua: el 2026-09-02 se descubrió que el `musubi` local de `davantis` era **`0.107.0` del 27 de agosto —veintitrés versiones atrás—** y que su daemon MCP no arrancaba (`el esquema de la base es más nuevo que este binario`). El fail-closed de las migraciones hizo su trabajo y dijo qué hacer, pero **nadie lo dijo antes**: es A68 otra vez, y `musubi_fleet_device_agent_stale` **no lo cubre** justamente porque esa máquina no está en la flota. La decisión no es obvia y por eso queda como cabo y no como tarea: enrolar la máquina de desarrollo la mete en el mismo tablero que la producción, con su ruido; no enrolarla deja fuera del radar al equipo desde el que se opera todo. La `raspberrypi` es la pregunta más simple —¿sostiene algo?— y hoy nadie la sabe. | **sin asignar** |
| A78 | **Un inventario de servicios VACÍO deja a la máquina muda para siempre, y el agente cree que reportó** | `serviciosDelLatido()` sella `ultimoInventario.enviado = time.Now()` **antes** de que el llamador decida si manda algo, y el llamador es `if svs := serviciosDelLatido(); len(svs) > 0`. Con una lista vacía las dos mitades se contradicen en silencio: el agente marca «enviado», el latido no lleva nada, y a los 5 minutos se repite igual — **para siempre, sin un solo error en ningún lado**. El cerebro no borra nada (la poda sólo corre cuando llega una lista), así que su inventario queda congelado y sólo ENVEJECE. La única señal es `musubi_fleet_service_last_report_seconds` creciendo, y lo único que la mira es `ServicioSinNoticias` a los 30 minutos. **La rama de ERROR no tiene este problema** y conviene tenerlas separadas: ahí se hace `return nil` sin tocar `enviado`, así que reintenta en cada latido — pero `avisarUnaVez` la anuncia **una sola vez por vida del proceso**, o sea que un agente que lleva días sin poder enumerar lo dijo una vez y nadie lo vio. **Encontrado el 2026-09-02 leyendo el código**, mientras se perseguía por qué `davantis-1` llevaba 129 minutos sin reportar servicios; **no quedó probado que fuera la causa de ESE caso** —la máquina se cayó entera antes de poder medirlo— y por eso se anota como lo que es: un modo de falla que existe en el código, no un diagnóstico. Se cierra sellando `enviado` sólo cuando el latido de verdad se lleva la lista, y haciendo que una lista vacía sea un hecho reportable en vez de un silencio. | **sin asignar** |
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
| B19 | **Unificar los dos formatos de fecha de la base** (era A61) | La nota de A61 decía «nueve consultas de recall». **Se midió el 2026-09-01 y son 67 usos de `CURRENT_TIMESTAMP` en 17 archivos** contra 51 de `time.RFC3339`: no es un slice, es tocar la capa de memoria entera —el núcleo del producto, no de la flota—. Y del otro lado, el riesgo REAL es una consulta que CRUCE las dos familias de tabla, y hoy hay **exactamente una**: `contexto.go`, ya contenida (constante con nombre, parseo dual, prueba con la hora fijada). Cambiar 67 lugares del núcleo para cerrar un riesgo con un solo punto de contacto ya custodiado es un mal negocio. **Lo que sí se hizo**: clavar las DOS suposiciones sobre el driver de las que depende la contención —que `CURRENT_TIMESTAMP` escribe `YYYY-MM-DD HH:MM:SS` (preguntado con un GLOB DENTRO de SQLite, para que el driver no se entrometa) y que al LEER devuelve RFC3339—, porque si el driver cambia el síntoma no es un error sino una ventana vacía, y un vacío se lee como «no había nada escrito ese día». **Se revisa si aparece una SEGUNDA consulta que cruce las dos familias**: ahí deja de ser un caso aislado. **Renumerado el 2026-09-02**: entró como `B13` el 2026-09-01 con el número ya ocupado dos veces, y fue el último de los tres en llegar. |
| B12 | **Verificar el `rustdesk_id` contra el relay** | Era el plan de A13 y **no es viable ni serviría**: hbbs (el relay OSS) no expone API para eso —habría que hablarle su protobuf, o sea reimplementar medio cliente— y aunque la expusiera sólo diría qué CONEXIÓN reclama ese id ahora, no cuál de nuestras máquinas es. Se cerró A13 por el otro lado: detectando la COLISIÓN, que es la firma del ataque y además el caso benigno frecuente (imágenes clonadas). **Queda sin cubrir** una máquina que declare un id que no colisiona con ninguna de las nuestras; de ésa se ve que el id CAMBIÓ. **Se revisa si RustDesk Pro o un hbbs con API entran en el despliegue.** |
| B10 | **Grabación del contenido de una sesión de shell** | Misma decisión legal que A14 (grabación de pantalla) y mismo dueño: nadie. `SesionShell` no tiene dónde guardarlo, y hay una prueba de FORMA que custodia esa ausencia — la única manera de proteger algo que no existe. **Se revisa si alguien toma la decisión, no antes.** |
| B13 | **El watchdog externo del dead-man's switch** | `MusubiSiempreViva` late hacia un receptor `watchdog` y hace falta un servicio EXTERNO que espere ese ping y grite si falta (Healthchecks.io, Dead Man's Snitch, un cron en otra máquina). **Gio lo despriorizó el 2026-08-29**, con sus palabras: «ya no tenemos algo así ahorita, no es muy importante hacer eso externo por si acaso». Era A22 y estaba anotado como pendiente del operador; una decisión tomada que sigue figurando como pendiente ensucia la lista y hace que se deje de leer. **Lo que esto deja descubierto, dicho sin adorno**: si el cerebro entero muere, nadie afuera se entera — el latido que avisaría de su muerte sale del propio cerebro. Hoy eso lo cubre, de hecho y no por diseño, que gio mire el panel. **Y hay un segundo latido igual de desarmado en la misma máquina**: `monitoring/infra/alerter.py` tiene su propio dead-man pero `HEARTBEAT_URL` está vacía en `meta.env`. Una sola cuenta de watchdog resolvería los dos, con un check por cada uno —nunca compartiendo el mismo, porque entonces un latido tapa la muerte del otro—. **Se revisa si el cerebro pasa a sostener algo que gio no mira todos los días.** **CONSERVA el B13 aunque no fue el primero en llevarlo** (lo tomó el 2026-08-29, y la fila de `sshd` con PAM lo tenía desde el 2026-08-27): `deploy/docker/README.md` §② ya cita **B13** para este watchdog, y una referencia desde otro archivo no se arregla renumerando acá. |
| B20 | **Cuál de los cinco sistemas de monitoreo manda** (era A33) | **Decidido por gio el 2026-08-29: `Prometheus + Alertmanager` es el autoritativo para ALERTAR.** Es donde ya viven todas las reglas de Musubi —host, servicios, políticas, empuje y las tres nuevas de rendimiento—, cada una con su sección de runbook. **Consecuencias declaradas, para que no haya que volver a deducirlas:** (a) **OpenObserve queda sólo como almacén de la vista del CRM**, no como origen de alertas — así que `collect-infra.sh` sigue corriendo hasta que esa vista lea de Prometheus, aunque su DATO sea redundante campo por campo con el del agente; (b) **`alerter.py` + WhatsApp se apagan o se reducen a lo que Prometheus no cubra** — hoy duplica la alerta de disco ≥85 %, que es el mismo trabajo hecho dos veces; (c) **Uptime Kuma y UptimeRobot quedan como respaldo declarado** para la muerte total del host, que es lo único que Prometheus no puede avisar de sí mismo; (d) **se desbloquean A32 y A36**, que esperaban esta decisión para no agrandar el problema. **Esta fila ERA A33** —el mismo commit (`defb268`) borró aquella y creó ésta el 2026-08-29—, y hasta el 2026-09-02 esa conversión no estaba escrita en ningún lado: dos citas de A33 quedaron apuntando a un número sin fila, que es exactamente lo que la regla 2 prohíbe. **Renumerada de `B14` a `B20`** el 2026-09-02, porque `B14` ya era de la fila de abajo desde el 2026-08-27. | **decidido** |
| B11 | **Reconectar a una sesión de shell viva** | Si el relay se corta, la sesión MUERE: no queda un proceso huérfano esperando que alguien vuelva. Reconectar sale caro (hay que retener la salida de un cliente ausente, con todo lo que eso implica para la contrapresión) y su beneficio es comodidad. **Se revisa si las desconexiones resultan frecuentes en uso real.** |
| B17 | **«Flota» significa dos cosas distintas en el mismo servidor** | La sección **Flota** del CRM inventaría *bots, puentes y servicios*, publicada a mano con `flota publicar` y leída de un archivo. La **flota** de Musubi son *máquinas midiéndose solas*. Comparten nombre y no comparten nada más, así que en algún momento alguien va a mirar una creyendo que es la otra. **No se renombra todavía** porque tocar la barra lateral del CRM es decisión de gio y el costo de la confusión hoy es bajo (un solo usuario, que sabe la diferencia). **Se revisa el día que alguien más use el CRM.** |
| B18 | **Probar contra un `sshd` con PAM, contraseñas o `ForceCommand`** | El `sshd` de S7b corre **sin root y sin PAM a propósito** —es lo que permite levantarlo sin instalar nada— así que esa rama queda sin ejercitar. Musubi nunca manda contraseñas (`BatchMode=yes`), y `ForceCommand` rompería cualquier RMM por igual. **Se revisa si aparece un host que las exija.** **Renumerada el 2026-09-02, y cediendo el número siendo la primera que lo tuvo** (2026-08-27): el `B13` se lo queda el watchdog externo porque `deploy/docker/README.md` ya lo cita así, y romper una cita desde otro archivo para respetar el orden de llegada sería cambiar un problema chico por uno peor. |
| B14 | **Probar contra un host REMOTO de verdad** | S7b fue loopback: sin latencia, sin MTU, sin cortes de red a mitad de una shell. Lo que sí cubre —`-tt`, host key, puerto, pty, cierre— no depende de la distancia. **Se revisa cuando haya un Tier B real enrolado.** **CONSERVA el B14** por orden de llegada (2026-08-27); la fila de monitoreo, que lo duplicaba desde el 2026-08-29, pasó a B20. |
| B15 | **Otras implementaciones de servidor SSH** | dropbear, el `sshd` de un router, el OpenSSH de Windows. Todas hablan el mismo protocolo pero difieren en `AcceptEnv` y en la shell de login, que es justo donde apareció el `--` de más. **Se revisa cuando entre el primer aparato que no sea OpenSSH de Linux.** |
| B16 | **`exec` en Tier C** | La matriz de S1 no se lo concede y sigue sin concedérselo: en Android depende de que ADB esté habilitado, y prometerlo al enrolar sería mentir. Un móvil da métricas y (con S8b) pantalla; una shell no. **Se revisa si aparece una flota de Androids con ADB garantizado por MDM.** |
| B21 | **Las fases del `proposal.md` y las de este registro no se renumeran: se declara el mapa** | `specs/control-de-flota/proposal.md` numera **Fase 0** (S1-S3), **Fase 1** (S4-S6), **Fase 2** (S7-S9) y **Fase 3** (S10 · alertas y políticas). Este registro, en cambio, dio por «**FASE 1 COMPLETA**» recién con S1…S9 y por «**FASE 2 COMPLETA**» con S10, y de ahí siguió con **fase 4** (servicios) y **fase 5** (lo cognitivo). O sea que las dos numeraciones **dicen cosas distintas con las mismas palabras**, y **«fase 3» no aparece en ningún archivo del repo**: no es que falte, es que nunca se usó. Los specs de cada slice siguen la del proposal (`flota-registro-dispositivos` = Fase 0, `flota-telemetria-del-host` = Fase 1, `flota-tier-b-protocolo` = Fase 2), así que renumerar cualquiera de los dos lados **desactualiza archivos que hoy son correctos por dentro** y que se citan entre sí. El mapa, que es lo único que faltaba: **proposal 0+1+2 → «fase 1» del registro · proposal 3 (S10) → «fase 2» del registro · fase 4 y fase 5 son posteriores al proposal y no tienen equivalente ahí**. **Se revisa si alguien que no escribió el track tiene que leer el proposal como plan vigente, o si se abre una fase nueva**: ahí conviene un solo esquema y el costo de renumerar ya está pago. |
| B9 | **Alertas por-tenant** | Las reglas de flota se evalúan sobre las series que la credencial del scrape puede ver, así que un despliegue con varios tenants necesitaría un Prometheus (o un principal) por tenant. Hoy hay uno. **Se revisa el día que dos tenants compartan cerebro y no quieran compartir alertas.** |

## 3 · Cerrado en este track (para no volver a abrirlo por olvido)

**2026-09-02 (sáb) · UNA MÁQUINA CAÍDA DEJA DE PRODUCIR TRES ALERTAS.**

Prometheus no borra la serie de una máquina muerta: **la CONGELA**. Conserva el último valor hasta
que caduca, así que una comparación sobre ese valor se sostiene indefinidamente — y el `for:`, que
existe para filtrar picos, **la confirma en vez de descartarla**.

Medido con `davantis-1` caída: el tablero mostraba `MaquinaCaida` (verdadera) **más `CPUSostenida` y
`MaquinaLateSinMedir`** disparando sobre la última muestra que llegó antes de morir. Dos de las tres
describían una máquina que no estaba, y `MaquinaLateSinMedir` afirmaba textualmente «late pero dejó
de medir» de algo que no late.

**El patrón que lo arregla YA EXISTÍA** —`ServicioCaido` usa `unless on(device) (musubi_fleet_device_up
== 0)` desde S12— y estaba aplicado en **3 de 15** reglas. Ahora está en las 12 que leen una serie
que se congela. `unless` y no `and up == 1` por una razón concreta: conserva las etiquetas del lado
izquierdo, y con `and` el mensaje perdería su `{{ $labels.service }}`.

**EL CASO INCÓMODO, aceptado a sabiendas**: si una máquina se cae PORQUE se le llenó el disco,
`DiscoPorLlenarse` deja de sonar y sólo queda `MaquinaCaida`. Sobre una máquina que no está no se
puede accionar nada, el dato sigue en Prometheus, y un aviso que no se puede atender sólo entrena a
ignorar el canal.

**LA PRUEBA FALLA CERRADA**, que es lo que la hace durar: la lista es de EXCEPCIONES, no de reglas a
vigilar. Una métrica de flota nueva entra vigilada por default y hay que declararla si no
corresponde; al revés, nacería sin guarda y en silencio.

**Y UN SABOTAJE ENCONTRÓ UN AGUJERO EN MI PROPIA PRUEBA.** La comprobación de las excepciones estaba
DESPUÉS del filtro «¿lee una serie que se congela?», y `MaquinaCaida` lee `musubi_fleet_device_up`,
que está en la lista de las que no se congelan — así que esa rama era **inalcanzable justo para las
dos que más importan**. Se le podía poner la guarda a `MaquinaCaida`, dejándola imposible de
disparar —la flota entera sin aviso de máquina caída— y el test pasaba en verde. Tres sabotajes: dos
fallaron como debían y el tercero encontró esto.

Resultado en producción, con la máquina todavía caída: **de tres alertas por un evento a una.**


**2026-09-02 (vie) · A69 CERRADO — gio vuelve al servidor público, y la flota queda partida en dos.**

El cabo decía que migrar al relay propio dejó afuera a todo cliente que no esté en la malla: una PC
de logística sin tailnet dejó de ver a `gio`. Se anotaron tres salidas y **gio eligió la (c)**:
devolver esa máquina al servidor público.

**Hecho y verificado contra el servidor, no contra el archivo**, que es la distinción que este track
paga cada vez que la saltea:

	rendezvous_server = 'rs-ny.rustdesk.com:21116'   ← lo escribió RustDesk al registrarse
	--get-id: 132570932                              ← el MISMO id de antes
	rs-ny.rustdesk.com:21116 -> True

**EL ID NO CAMBIÓ, Y ESO NO FUE SUERTE.** El `RustDesk.toml` ya traía `rs-ny = true` en
`keys_confirmed`: la máquina había estado en el público antes de la migración y su registro seguía
vivo ahí. Por eso la vuelta fue limpia y **no hubo que tocar `rustdesk_id` en el cerebro** — con un
id nuevo habría que haberlo actualizado a mano, y la detección de colisiones (A13) habría visto un
cambio que no era un ataque.

**LA LECCIÓN DE A35 SE APLICÓ, Y ESTA VEZ EN EL ORDEN CORRECTO.** Editar el `.toml` de una app viva
no sirve: el proceso que sobrevive lo reescribe desde memoria. El script **aborta y vuelve a
levantar RustDesk** si al momento de escribir queda un proceso; midió `procesos vivos: 0` antes de
tocar nada. Y son **DOS configuraciones**, la de `meirn` y la del servicio en
`C:\Windows\ServiceProfiles\LocalService`: la del servicio es la que manda, y es la que la
interfaz nunca hubiera arreglado sola. Las dos quedaron sin rastro del relay propio, con las cuatro
copias de respaldo al lado.

**Se sacaron cuatro líneas y ninguna más**: `rendezvous_server`, `custom-rendezvous-server`,
`relay-server` y `key`. El `RustDesk.toml` —id cifrado, contraseña, par de claves— **no se tocó**.

**LA CONSECUENCIA, QUE NO ES UN CABO PERO HAY QUE DECIRLA.** La flota queda con las dos Windows en
servidores de ID DISTINTOS: `gio` en el público y `davantis-1` en el relay propio. **No se ven entre
ellas** — es exactamente la situación que en A35 dejó a `davantis-1` sin poder llegar a `gio` para
arreglarla. Hoy no molesta porque `davantis-1` está caída, pero va a molestar cuando vuelva.

Y queda una pregunta abierta para el operador, que no se decide desde acá: **el relay propio ahora
sirve a UNA sola máquina**. Si `davantis-1` también volviera al público, el relay no tendría a
nadie; si se queda, hay que sostener un relay para un solo cliente.


**2026-09-02 (qui) · EL MAPA DE COBERTURA — «¿esa regla vigila a ESTA máquina?»**

No cierra un cabo numerado: cierra una PREGUNTA que no se podía contestar desde ningún lado, y que
resultó ser la de un nivel más adentro que la de A73.

`verificar-despliegue.sh` contesta «¿está la regla cargada?». Con las 35 reglas desplegadas y
**todas sus métricas presentes** —verificado, no supuesto—, se midió esto:

	TemperaturaAlta        1 serie de 4 máquinas
	CargaPorCoreAlta       2 de 4
	ServicioReiniciandose  54 series, TODAS del servidor  → 0 de las 2 Windows
	ServicioLento          1 serie                        → 1 servicio de 184

Cada hueco tenía una razón buena —Windows no tiene load average, el SCM no expone reinicios, A2
sigue abierto— y **ninguna se podía leer**. La regla cargada, su métrica presente, y esa dimensión
de esa máquina a ciegas. Verde por el motivo equivocado, sin que hubiera un bug en ningún lado.

`deploy/verificar-cobertura.sh` cruza, por máquina y por regla, si existen las series que esa
expresión necesita. Hoy: `musubi-server` 18/19, las dos Windows 13/16, `altura-db` 9/10.

**LA RAZÓN LA ESCRIBE LA REGLA**, en su anotación `ausente_en:`, no el verificador. Misma forma que
el `# despliegue:` de A73 y por lo mismo: un catálogo de excepciones que vive en el verificador se
desincroniza de las reglas y termina perdonando huecos que ya no corresponden.

**TRES CASILLEROS, y ésa es la decisión de diseño que hace útil el informe.** Un límite estructural
(`os=windows`: no hay load average que activar, no lo va a haber nunca) NO es lo mismo que una
cobertura opt-in sin activar (`sin-declarar`: la serie aparece en cuanto alguien la configura).
Meter la segunda en la primera habría borrado del informe que `ServicioLento` cubre 1 servicio de
184 — que no es una falla, pero tampoco es cobertura.

**DOS BUGS DE MODELADO PROPIOS, y salieron de leer las expresiones en vez de contar métricas.**
`A unless B` exigía `B` como cobertura, lo que invierte el sentido de `MaquinaSinInventario` —que
existe justamente para avisar que esa serie FALTA—. Y una regla que ya se auto-limita con
`{tier="A"}` no necesita que nadie le declare el hueco: pedir la declaración sería escribir dos
veces la misma decisión, y dos copias se contradicen el día que alguien cambia una.

**Y UN FALSO VERDE PROPIO, que es lo que más vale de la entrada.** Al juntar las 25 consultas en una
sola conexión (100 s → 2,7 s) las que llevaban espacios se partieron en varios argumentos. No volvió
una sola serie, y el informe dijo `0/0 · toda ausencia de cobertura está declarada` — **la frase más
tranquilizadora posible sobre nada**. Es exactamente la enfermedad que este verificador vino a cazar,
cometida por el verificador. Ahora corta con error sobre el conjunto vacío y una prueba lo custodia.

Cinco sabotajes ejecutados, uno end-to-end contra producción. **Y quedó anotado lo que NO mira**: el
contenido de cada regla (un umbral que cambió tiene el mismo nombre), y las máquinas que no están
enroladas — que es justamente **A77**.


**2026-09-02 (qui) · A72 CERRADO — 31 GB, y el hallazgo es que casi nada era basura.**

`davantis-1` estaba al **6,8 %** de disco (15,8 GB de 232,1) con `DiscoPorLlenarse` disparando. Hoy
está al **20,1 %** (46,8 GB). Pero lo que enseñó el cabo no es el número: es lo que costó llegar a
él.

**LA HIPÓTESIS INICIAL ERA QUE SOBRABA BASURA, Y ERA FALSA.** Se pidió «limpiá todo lo que no se
use», y al verificarlo **casi todo estaba instalado y en uso**:

  · `Riot Games`, 42,25 GB —el ítem más grande del disco— tenía escritura de ayer. No era el
    anti-cheat tocando archivos al arrancar: el `LeagueClient.log` mostraba una sesión de las 17:03
    a las 23:49. Se jugó. Descartado como candidato **por evidencia, no por prudencia**.
  · `Docker`, 30,37 GB, tenía once contenedores arriba hacía 21 horas: el Supabase local de
    `altura-erp`. Infraestructura viva, no un resto olvidado.

Lo que de verdad no se usaba sumaba **1,89 GB** (`hermes`, 56 días sin tocarse). Nada más. El resto
del espacio salió de **cachés que se regeneran** —`Temp` 9,02, `go-build` 7,16, modcache de Go 4,39,
`.cargo/registry` 1,29, `npm-cache` 1,19, shaders de AMD 0,99— y de **datos que gio decidió soltar**
(`curseforge` 3,44 y `.lunarclient` 1,31, sin uso hacía un mes).

**EL MÉTODO FUE EL DATO, NO LA HEURÍSTICA.** Cada carpeta se juzgó con tres hechos: si el programa
figura instalado, si su proceso corre ahora, y cuál es el archivo más nuevo que tiene adentro. Sin
eso, `Riot Games` era el candidato obvio por tamaño — y borrarlo habría sido borrar algo que se usa
todos los días.

**LO QUE SE DECIDIÓ NO HACER, Y POR QUÉ.** Quedan 7,99 GB reclamados **adentro** del disco virtual de
Docker (38 imágenes → 28, cero huérfanas) que NO aparecen en `C:`: un `.vhdx` no se encoge solo.
Devolverlos exige apagar Docker y correr `diskpart` sobre el archivo donde vive la base del Supabase.
Cuando se planteó, el umbral ya se había cruzado sin eso — **la razón para aceptar el riesgo se había
evaporado mientras se trabajaba**, y se volvió a preguntar en vez de ejecutar la respuesta vieja.
Gio eligió no compactar. El purgado igual sirve: el disco virtual deja de crecer.

**Y UN GOTCHA DEL CANAL DE EXEC, que ya está en la memoria.** La primera tanda intentó borrar seis
cachés (310.000 archivos) en un solo comando, se pasó del `timeout_seg: 540` y **el agente la mató a
mitad**. La salida sólo llega cuando el comando TERMINA, así que un corte deja sin saber qué se
borró. Con caché da igual —npm, Go y Cargo se re-descargan— pero el mismo patrón sobre datos deja una
instalación a medias en silencio. Desde entonces: **un comando por objetivo**. Medido de paso:
`Remove-Item -Recurse` hace unos 300 archivos por segundo; `go clean -modcache` liberó 4,39 GB en
298 s haciendo el trabajo por fuera del pipeline de PowerShell.

**Sin efecto colateral**: los once contenedores siguieron arriba durante todo el purgado, y
`DiscoPorLlenarse` se apagó sola — no se tocó su umbral.


**2026-09-02 (qua) · A68 Y A73 CERRADOS — y los dos son la misma enfermedad vista de dos alturas.**

**A68 · un agente que se quedó atrás era invisible.** `agent_version` se guardaba y
`musubi_fleet_list` la mostraba, pero nada AVISABA: un agente veinticuatro versiones atrás se veía
idéntico a uno al día. Se descubrió de casualidad el 2026-09-01, mirando otra cosa, y el costo fue
concreto — A67 se había desplegado el día anterior y **no podía correr en las dos máquinas para las
que se escribió**, porque su binario no tenía la capacidad.

Ahora lo dice `musubi_fleet_device_agent_stale`, y **la decisión que hace útil a la métrica es qué
compara**. El cerebro corre `0.130.0-flota.38a0a9f` y los Windows `0.130.0-flota.e140e0c`: mismo
release, commits distintos, que es lo normal —el binario de Windows se cruza a mano y el del
cerebro se redespliega varias veces por día—. Comparando la cadena entera, **la flota entera queda
marcada después de cada despliegue**, y se queda así hasta que alguien cruce el binario a cada
máquina. Una alarma encendida siempre es una alarma apagada. Por eso compara el NÚCLEO semver.

**Y eso deja un hueco que está declarado, no descubierto después**: una capacidad que entra al
cerebro sin tocar el archivo `VERSION` no se ve. La comparación mide lo que VERSION declara, y
VERSION lo bumpea una persona.

**Los tres «no sé» son distintos y se separan a propósito.** Un Tier B no corre nuestro binario:
ausente. El CEREBRO sin su versión (un build sin ldflags): ausente también, porque marcar a la
flota entera sería culparla de un problema nuestro y mandar a alguien a revisar diez agentes sanos.
Una versión ilegible: ésa SÍ se responde, con 1 — no sabemos cuánto se atrasó, sabemos que no es la
nuestra.

La versión **no viaja como etiqueta**, y eso ya estaba escrito en `labelsDeFlota` desde antes: una
etiqueta con la versión deja la serie re-etiquetándose sola en cada actualización, con las viejas
huérfanas. Ahora además hay una prueba que lo custodia.

**A73 · las guardas validaban el repo y producción divergía.** `TestLaCadenaDeAlertasSeVigilaASiMisma`
pasaba en verde mientras el job `alertmanager` no estaba desplegado: sin ese scrape,
`CadenaDeAlertasFallando` —la alerta que vigila que las alertas se entreguen— no podía dispararse
nunca. La guarda leía `deploy/prometheus/prometheus.yml`; el servidor corría otro archivo.

Se cierra por **dos caminos, porque las dos mitades del problema son distintas**.

**A pedido**: `deploy/verificar-despliegue.sh` le pregunta a las APIs —no a los archivos—, que es la
única respuesta que importa: un archivo correcto que Prometheus no releyó se ve igual que uno bueno.
Es el error que costó una hora el 2026-08-31, cuando un `sed -i` sobre un bind-mount cambió el inodo
y la recarga contestó 200 sobre el archivo equivocado. Distingue tres cosas que se ven parecidas y
se arreglan distinto —sin desplegar, desplegado a medias, y reglas huérfanas de un despliegue
anterior— y **no denuncia los archivos parkeados a propósito**, que lo declaran en su propia línea
`# despliegue:`. Un informe que denuncia lo que está bien deja de leerse a las dos semanas.

**Desatendido**: cada archivo de reglas vigila el conteo del OTRO. **Cruzado, y ése es el punto**: un
archivo que declara su propio conteo se despliega junto con el conteo, las dos mitades se mueven a la
vez y la comprobación no falla nunca. Cruzándolos, un despliegue a medias rompe la simetría en la
dirección que sea y alguno de los dos grita. Se probó en producción de la única forma que vale: las
dos alertas estuvieron disparando mientras el repo tenía 4 reglas que el servidor no, y se apagaron
solas al desplegarlas.

Y `ScrapeQueElRepoDeclaraYNoExiste` cubre el caso exacto que abrió el cabo, con la distinción que
importa: `up == 0` es «no contesta»; acá el job **ni existe**, así que ninguna regla que use sus
métricas puede dispararse. Una prueba compara esa lista contra los `job_name` del repo, en las dos
direcciones — un scrape nuevo sin vigilar rompe la suite, y vigilar uno que no existe también.

**EL NÚMERO DE LA CUSTODIA NO PUEDE PUDRIRSE**, que era el riesgo obvio de esta forma. Lo calcula la
suite desde el YAML: agregar una alerta sin actualizar el conteo del archivo que la custodia rompe
las pruebas **en el repo**, antes de que la alarma quede encendida en producción para siempre.

**LO QUE NINGUNO DE LOS DOS MIRA, dicho acá y en el encabezado del script.** El contenido de cada
regla: un umbral que cambió en el repo y no en producción tiene el mismo nombre y no se ve. Los
scrapes de sitio, que son por sitio a propósito. Y las máquinas de la flota, que las cubre A68.

**Ocho sabotajes ejecutados, y el primero enseñó lo de siempre**: no compilaba (`declared and not
used`), así que la prueba ni corrió y el silencio se veía igual que una guarda que funciona. Se
reescribió para que compilara, y ahí sí falló — con el síntoma exacto que se buscaba: la máquina al
día marcada como atrasada.

**Y EN EL CAMINO CAYÓ EL MISMO BICHO EN LA MÁQUINA QUE NO ESTÁ EN LA FLOTA.** El `musubi` local de
`Davantis` era `0.107.0` del 27 de agosto —veintitrés versiones atrás— y su daemon MCP no arrancaba:
`el esquema de la base es más nuevo que este binario`. El fail-closed de las migraciones hizo
exactamente su trabajo y dijo qué hacer. Es A68 otra vez, y **`agent_stale` no lo habría visto**:
esa máquina no está enrolada. Queda dicho porque el patrón es el mismo y el remedio no alcanza.


**2026-09-02 (ter) · A71 CERRADO — el script que actualiza el agente vivía sólo en las máquinas.**

`cambiar-agente.cmd` reemplaza el binario del agente, con prueba de latido y vuelta atrás. Y hasta
hoy **no estaba en el repo**: existía en las dos Windows, escrito a mano, sin versionar y sin una
sola prueba. Eso es lo que explica que tuviera dos fallas y que ninguna se hubiera visto — no eran
sutiles, no había dónde mirarlas.

**Una · tomaba su carpeta de `%LOCALAPPDATA%`.** Con el agente corriendo como SYSTEM eso apunta a
`C:\WINDOWS\system32\config\systemprofile\AppData\Local`, no a la instalación. El script no
encontraba el binario nuevo, escribía «NO HAY BINARIO NUEVO» y salía sin tocar nada: **fallaba en
silencio, que es justo lo que ese script existe para no hacer.** Ahora usa `%~dp0`, lo único que no
depende de quién lo ejecuta.

**Dos · no mataba al proceso viejo.** `schtasks /end` termina la tarea, no al hijo que el envoltorio
oculto dejó corriendo. Y como el paso [2] RENOMBRA el binario en uso —el truco que hace posible
todo esto— ese proceso sigue vivo desde `musubi.exe.viejo`. En `davantis-1` un agente **v0.106.0
zombi llevaba horas latiendo** después de una actualización que el log daba por exitosa, y por eso
la máquina figuraba en la versión vieja: el zombi ganaba la carrera del latido. Además tomaba el
archivo, así que el `del` daba «Access is denied» y disparaba el rollback.

**La matanza va por RUTA EXACTA, no por nombre de imagen**, y eso no es prolijidad: en estas
máquinas también corre la app de escritorio en `AppData\Local\Programs\musubi\musubi.exe`. Un
`taskkill /IM musubi.exe` la cerraría de un saque. Se descubrió mirando la lista de procesos antes
de matar — un patrón ancho (`*\Musubi\*`) que se había escrito para el arreglo la habría barrido.

El instalador ahora lo copia de su lado, así que una máquina nueva nace pudiendo actualizarse. Si
falta, lo dice en amarillo y sigue: no poder auto-actualizarse no es motivo para no instalar.

Tres sabotajes ejecutados. Y la prueba **falló primero contra el script correcto**: su aserción de
«no matar por nombre de imagen» matcheaba el COMENTARIO que explica por qué no se hace. Una guarda
que no distingue el código de su explicación prohíbe documentar la decisión.

**2026-09-02 (bis) · A70 CERRADO — `Automatic` no significa lo mismo en Windows que `enabled` en systemd.**

Actualizar los dos agentes Windows encendió **dieciséis `ServicioCaido`** de golpe. Ninguno era
real: `sppsvc`, `MapsBroker`, `edgeupdate`, los updaters de Google y Edge, `asus`. Todos
**automáticos y apagados por diseño** — `sppsvc` corre cuando Windows valida la licencia y se
apaga, `MapsBroker` sólo si alguien abre Mapas, los updaters cuando toca actualizar.

**La primera explicación fue equivocada y conviene decirlo**: se leyó como «servicios manuales que
no deberían reportarse», y el filtro que los descarta ya existía y estaba bien. Al medirlo, los
cinco daban `StartMode=Auto`. El problema no era el filtro: era su premisa.

`enabled` en systemd significa «arranca al boot y tiene que estar corriendo». `Automatic` en
Windows **no significa eso**: un servicio puede declararse automático y ser *delayed* o
*trigger-start*, y quedarse apagado hasta que algo lo despierte. El código estaba bien escrito
para el sistema equivocado.

**Lo que lo separa es un dato, no una heurística.** Medido en `gio`: los 8 automáticos detenidos
tenían `ExitCode=0` y `ServiceSpecificExitCode=0`. Cero es «terminó bien»; 1067 es «se murió»;
1077 es «nunca arrancó desde el boot». `Get-Service` no expone ese campo — `Win32_Service` sí, y
en la misma llamada— así que se cambió de fuente.

**Estado nuevo: `ocioso`**, que es «detenido POR DISEÑO». No emite `musubi_fleet_service_up` ni en
0 ni en 1, que es la misma regla que gobierna el resto del exportador: un valor que no aplica no
se exporta como cero. La serie ausente deja a `ServicioCaido` sin nada que matchear. Cuando ese
mismo servicio se muera de verdad, el agente lo reporta `fallado`, la serie aparece en 0, y la
alerta dispara — lo que desaparece es el ruido, no la señal. Y `ocioso` no cuenta como caído para
las políticas, así que ninguna acción automática se dispara contra un servicio que está bien.

**Tres sabotajes ejecutados, y el segundo enseñó algo.** El primer intento de sabotear la regla
del ExitCode **no aplicó el cambio** —el `python` no matcheó— y no imprimió nada; el segundo
intento **no compiló** (`declared and not used`), así que la prueba tampoco corrió. Recién el
tercero, escrito para compilar, la hizo fallar. *Un sabotaje que no llega a ejecutarse se ve
igual que una guarda que funciona*, y las dos veces lo que se vio fue silencio.

**El número vivía en el código y no acá, y eso se arregla el 2026-09-02.** `A70` se cita en
`internal/fleet/servicio.go`, `internal/mcp/fleet_prometheus_servicios.go`,
`cmd/musubi/servicios_windows.go`, `cmd/musubi/servicios_parsers.go` y dos pruebas —seis archivos—,
y esta entrada contaba el trabajo **sin nombrarlo**: quien seguía la pista desde un comentario del
código llegaba a un registro donde `A70` sólo aparecía de refilón, dentro de la nota de A2. Un
número que el código usa y el registro no define es la misma enfermedad que A33, vista del otro
lado.

**2026-09-02 · A35 CERRADO — las dos máquinas en el relay propio, y el instalador tenía un bicho.**

El relay llevaba desde el 27 de agosto corriendo, sano por las tres vías que existían, y con la
tabla `peer` **en cero**. Hoy tiene dos:

    1740888405  ·  davantis-1  ·  2026-09-01 17:52
     132570932  ·  gio         ·  2026-09-02 02:53

**Cada máquina falló por un motivo distinto y ninguno era el relay.**

`davantis-1` tenía la config bien y no llegaba: el NordVPN le cortaba la ruta al tailnet. Se
resolvió con split tunneling. Y ahí se midió lo que quedó anotado en A31: **la autorización es por
ARCHIVO** — desde esa máquina PowerShell no alcanza ni el puerto del cerebro mientras el agente le
late cada 30 s. Toda administración remota que no sea el propio agente está cortada ahí, y
cualquier medición hecha con otra herramienta mide otra cosa. Costó dos diagnósticos equivocados.

`gio` tenía la red bien y la config se le revertía. La causa: **editar el `.toml` de una app que
está corriendo no sirve** — el proceso que sobrevive lo reescribe con lo que tiene en memoria. La
primera vez se vio «Access is denied» en dos procesos del `taskkill` y se siguió igual; por eso se
perdió. La segunda vez, con el agente ya como SYSTEM, `procesos vivos: 0` antes de escribir, y las
DOS configs —la del usuario y la del SERVICIO— quedaron. La del servicio es la que manda y es la
que la interfaz nunca hubiera arreglado sin el candado.

**EL ORDEN IMPORTABA Y SE VIOLÓ.** Con una migrada y la otra no, las dos quedan en servidores de ID
distintos y **no se ven**, así que desde `davantis-1` ya no se llegaba a `gio` para arreglarla. La
regla «la máquina desde la que operás, al final» estaba escrita desde el principio; se rompió
porque el camino automatizado funcionó justo en la que no correspondía y nadie re-evaluó el orden
cuando eso pasó.

**Y EN EL CAMINO CAYÓ UN BICHO DEL INSTALADOR, que es el hallazgo más limpio de la sesión.**
`-AlArranque` y el endurecimiento del `device.token` eran **mutuamente incompatibles**: el token
queda con una sola regla ACL —el usuario que instaló— y la tarea corre como SYSTEM, que entonces no
puede leer su propio token. La tarea decía «Attempted to run», volvía a `Ready` al instante, y la
máquina quedaba fuera de la flota sin un solo mensaje de error.

Lo que hace que valga la pena contarlo: **el instalador tiene un paso que existe justamente para no
fallar en silencio** —prueba un latido real antes de crear la tarea— y ese paso pasa el token por
VARIABLE DE ENTORNO, corriendo como el usuario. Nunca abre el archivo. Probaba un camino distinto
del que la tarea iba a usar, y daba verde sobre una instalación muerta.

Arreglado, con guarda y dos sabotajes ejecutados. Y de paso quedó dicho que negarle el token a
SYSTEM no era una defensa: SYSTEM puede tomar posesión de cualquier archivo de la máquina.

**Lo que `gio` ganó de yapa:** corre al arranque como SYSTEM, así que ya no depende de que alguien
inicie sesión —el modo de fallo que la dejó tres días fuera de la flota— y `exec` tiene permisos de
administrador. El precio, tomado a sabiendas: sesión 0, `puede_preguntar` en false, y un `pide` que
se endurece a `prohibido` en esa máquina.

**2026-09-01 · A67 CERRADO — la alcanzabilidad se le pregunta al CLIENTE, no al servidor.**

El colector del relay sondeaba sus tres puertos desde el propio `musubi-server`. Eso verifica que
el servicio contesta ahí, que es lo único que ese punto de vista puede ver — y no es la pregunta.
La pregunta de un relay es si un CLIENTE lo alcanza.

Vino diciendo `OK` desde que se instaló mientras la tabla `peer` estaba en cero y ningún cliente
había llegado nunca. Medido el 2026-09-01, con el relay «sano» por las tres vías que existían —dos
contenedores arriba y los tres puertos contestando—: `davantis-1` daba `tcp21116=False` y `gio`
daba `True` contra el MISMO puerto. **La salud era distinta según quién preguntara, y el chequeo
sólo conocía la respuesta del que nunca falla.** La causa concreta fue el VPN de una de las dos
(A31), pero el agujero es independiente de la causa: un punto de vista único no puede medir
alcanzabilidad.

**Cómo se cerró.** El agente sondea, en cada latido, los destinos que le declararon en
`MUSUBI_ALCANCE` (`host:puerto`, hasta cuatro, en paralelo para que cuatro timeouts no se sumen
adentro de un latido de 30 s). El resultado viaja **dentro de la muestra**: es una medición que la
máquina toma de su entorno, con el mismo dueño y la misma frecuencia que la CPU. Y como
`last_sample` es una columna de TEXTO con ese JSON, **no costó migración** — una muestra vieja sin
el campo se lee igual, con la lista vacía.

**AUSENTE NO ES FALSO, y acá decide si la alerta sirve o es ruido.** Una máquina sin destinos no
emite la serie. Si emitiera `0`, toda la flota sin configurar dispararía desde el día uno — que es
exactamente cómo se le enseña a alguien a ignorar el canal entero.

**La métrica dice «llego a TODO» y no lleva etiqueta `destino`.** Sus valores los elige quien
configura cada máquina, así que como etiqueta serían cardinalidad sin techo por flota. Cuál falla
se responde en `musubi_fleet_list` (`no_alcanza`, y sólo los caídos), que es la misma decisión que
este track ya había tomado con el desglose de servicios. Se llama `musubi_fleet_device_reach_up`
—familia `device_`— así que cae bajo el `drop` del scrape que se angostó ayer y no hizo falta
tocar la config de Prometheus.

**Y EN EL CAMINO CAYÓ UNA GUARDA QUE NO GUARDABA.** La alerta nueva citaba
`deploy/RUNBOOK.md#maquinaquenoalcanzasudestino`, una sección que todavía no existía, y
`TestCadaRunbookDeUnaAlertaApuntaAUnaSeccionQueExiste` **pasó en verde**: su patrón exigía que la
anotación EMPEZARA con `deploy/RUNBOOK.md#`, y la forma «...instrucción. Ver deploy/RUNBOOK.md#x»
—que ya usaba otra alerta— nunca se verificaba. Se ensanchó el patrón y ahora falla por los dos
lados: ancla inventada, y sección borrada del runbook. Los dos sabotajes, ejecutados.

Es el mismo hallazgo que el resto de estos días: una comprobación que contesta con seguridad sobre
algo que no mira.

**2026-08-31 (bis) · A60 CERRADO — y la regla que proponía esta misma nota era incorrecta.**

Un comando `entregado` que nunca reporta se quedaba así **para siempre**: `terminado` lo estampa el
reporte del agente, y si el agente se murió a mitad ese reporte no llega nunca. La fila quedaba
indistinguible de un comando que está corriendo ahora mismo.

La nota de A60 proponía dos caminos: un estado nuevo, o «una regla sobre `entregado + timeout +
margen`». **Se hizo el primero y el segundo resultó estar mal**, y descubrir por qué fue el trabajo
real del cabo.

El agente ejecuta la tanda **en orden y de a uno**, reportando cada resultado antes de pasar al
siguiente, y una tanda trae hasta diez comandos de hasta diez minutos cada uno. Así que el último
de una tanda espera a los nueve de adelante **antes de que su propio timeout empiece siquiera a
correr**: puede estar legítimamente `entregado` casi cien minutos sin que nada esté mal. Con la
regla propuesta, ese comando se dibujaría muerto mientras corre.

**Y ése es el error caro de los dos.** Un comando vivo marcado perdido manda a alguien a
relanzarlo —dos veces el mismo `systemctl`, dos veces el mismo borrado—. Uno perdido marcado tarde
sólo se ve tarde. Así que la cota es conservadora por construcción: `ComandosPorEntregaMax ×
ComandoTimeoutMax + MargenDeReporte`. Nunca puede matar a uno vivo.

**Lo que NO se hizo, y está anotado en el código, no olvidado.** Como el agente reporta de a uno,
los comandos que siguen `entregado` en una máquina son exactamente los que faltan, y los creados
antes que uno son los que tiene por delante. Esa cota decaería sola y sería mucho más fina. Pide
contexto de la máquina entera, y `EstadoActual` es un método de UN comando con tres llamadores,
dos de los cuales no tienen la lista. Se prefirió la cota gruesa y correcta antes que el refactor.

**Se derivó, no se guardó**, igual que `Vencido` y que el «en línea» de un dispositivo: una columna
de estado que hay que ir a actualizar miente en cuanto nadie la actualiza. Y `entregado` sin fecha
de entrega —las filas anteriores a que el campo existiera— **no** se dibuja perdido: un dato
ausente no es un cero, que es la regla que gobierna el track entero desde S4.

De paso se unificó `maxComandosPorLatido`, que estaba definido en el transporte y ahora sale del
dominio: la cota depende de él, así que dos definiciones se habrían desincronizado y vuelto la
derivación incorrecta EN SILENCIO.

Tres guardas, con sus tres sabotajes ejecutados —sacar la rama del embudo, poner la regla obvia, y
sacar la guarda del agujero de datos— y el archivo restaurado idéntico al respaldo. La prueba de la
tanda larga existe exactamente por el error que casi se comete.

**2026-08-31 · EL MOTOR DE POLÍTICAS ESTABA VACÍO, Y ESO TAPABA DOS BICHOS.**

Se cerró **A64 en producción** —`kernelos-pc` pasó a `davantis-1`, y los tres nombres (Musubi,
tailscale, quien la usa) dicen por fin lo mismo— y se sacó **`davantis-admin`**, que era una
credencial de admin VIVA cuyo valor en claro no controlaba nadie. En su lugar quedó
`davantis-consola`, con el token en un archivo modo 600 y `MUSUBI_TOKEN_FILE` en el perfil.

Pero lo que importa del día es otra cosa. **El motor de políticas llevaba desde S10 desplegado,
validado, con cooldown persistido y métrica propia — y sin UNA SOLA REGLA configurada.** El
`acciones=0` de cada barrido lo venía diciendo hace meses. Al poner la primera política real y
hacerla disparar, aparecieron dos fallas que ninguna de las dos mitades habría mostrado sola.

**Uno · el scrape descartaba la única métrica que el empuje no lleva.** El job `musubi` tenía
`metric_relabel_configs` sobre `musubi_fleet_.*`, con buen motivo: la telemetría de flota llega
además por OTLP y con las dos series cada alerta salía duplicada (medido: 5 alertas, 10 avisos).
Pero `musubi_fleet_policy_actions_total` es la ÚNICA de esa familia con un solo productor —sale
del scrape y el empuje no la lleva— y la regla ancha la barría. `PoliticaQueNoCura` y
`PoliticaSinPermiso` **no podían dispararse nunca**, ni con políticas configuradas y actuando.

En este mismo archivo eso figuraba explicado: «no tienen series porque no hay ninguna política
configurada. Verificado, no supuesto». **Era cierto y tapaba lo de abajo.** Dos causas apiladas, y
la de arriba alcanzaba para cerrar la pregunta. Sólo se vio haciendo las dos mitades: configurar
la política Y verla disparar. Con una sola, la conclusión hubiera sido «ahora sí anda» o «sigue
sin haber políticas».

**Dos · la serie nacía en la primera acción y moría en cada reinicio.** `politicaStats` declara
desde S10 que se emite aunque valga cero, «porque el silencio y el cero no son lo mismo». El
código nunca lo hizo: el mapa nacía vacío y `renderPoliticas` cortaba. Las dos alertas son
`increase(...)`, y un `increase()` sobre una serie ausente no devuelve nada — o sea que no podían
distinguir «no actuó» de «el cerebro dejó de exportar», que es la distinción entera por la que ese
comentario existe. Se vio al reiniciar después de configurar la primera política: la serie que
acababa de aparecer desapareció, sin log, sin nada.

**EL PATRÓN DEL DÍA, Y ES INCÓMODO: las tres cosas estaban DOCUMENTADAS BIEN y el código hacía
otra cosa.** El comentario de `prometheus.yml` decía `musubi_fleet_device_.*` mientras la regla
decía `musubi_fleet_.*`. El de `politicaStats` prometía emitir en cero mientras el render cortaba.
Y este archivo explicaba la falta de series con una razón verdadera que tapaba la real. Nadie
mintió; nadie comparó. **Cuando un comentario y su código discrepan, el comentario suele ser el
que tenía razón** — es lo que alguien pensó despacio; el código es lo que salió apurado.

**A65 CERRADO — `musubi-tool.sh` ya dice con qué credencial habló.** `MUSUBI_TOKEN` le gana a
`MUSUBI_TOKEN_FILE` (correcto: una variable es una decisión más reciente) pero lo hacía en
silencio, así que una variable a medio setear de hace media hora le ganaba al archivo recién
creado y el archivo no se abría ni una vez. Costó cuatro intentos con el YAML, el hash, la ruta,
el proceso y la recarga todos verificados correctos: la causa estaba en el shell, el único lugar
donde nadie miró porque nada apuntaba ahí. Ahora avisa cuando las dos fuentes están puestas, y un
401 dice qué credencial usó, cuántos caracteres tenía, y nombra la ventana de recarga de 10 s
—que era la causa más probable y no figuraba en el mensaje viejo—. Probado contra los cuatro
caminos, incluido el feliz.

**A66 CERRADO — el informe de un rename declara lo que NO puede ver.** Enumeraba `principals.yaml`
y `config.yaml`, y con eso se leía como exhaustivo. No lo era: `device` es también una etiqueta de
Prometheus, donde el nombre viejo no se migra ni se borra —sus series quedan huérfanas, las nuevas
arrancan sin historia— y una alerta que filtre por el viejo no falla: deja de disparar. El cerebro
no puede comprobarlo (le EMPUJA métricas, no le consulta), así que **declara el hecho en vez de
inventarse una verificación que no puede hacer**. Un informe que calla lo que no mira se lee como
si lo hubiera mirado.

**LOS ERRORES PROPIOS, que fueron tres.** Se dio un `sed -i` sobre un archivo bind-montado en
podman: `sed -i` no edita, REEMPLAZA —cambia el inodo— y el contenedor quedó mirando el archivo
anterior, que ya no tenía nombre. Peor: la recarga contestó `200`, honesta y perfectamente inútil,
porque Prometheus releyó con éxito el archivo equivocado. (`install`, que usa `preparar.sh`, SÍ
conserva el inodo: se midió antes de acusarlo.) Se le echó la culpa a una carrera con la recarga
de 10 s cuando la recarga había funcionado bien desde el principio. Y se puso un bloque de
RESULTADOS dentro de un recuadro de código, que se ejecutó como si fueran comandos.

**2026-08-30 (7) · LA VERSIÓN SE DERIVA, NO SE TIPEA — y el que numeraba mal era yo.**

El redespliegue imprimió `de: 0.115.0-reparto.cc2ae9c` → `a: 0.133.0-rename.7219b3e`, que se lee
como un salto de dieciocho versiones o como un rollback, según de qué lado se mire. No era ninguna
de las dos: **era el mismo tronco**. La otra sesión leía el archivo `VERSION` del repo (0.115.0) y
le ponía el sufijo de su track; yo venía inventando números a mano —0.127, 0.128, 0.130, 0.133—
durante seis despliegues seguidos.

**El que estaba mal era yo**, y lo confirmé mirando el binario respaldado antes de acusar a nadie:
el que decía `0.115.0` tenía adentro la cronología, el contexto, la búsqueda por frase y la
migración 41. El código era actual; el número, mío.

**Y ESO NO ES COSMÉTICO.** El `de:`/`a:` del script de redespliegue es el ÚNICO registro de qué
estuvo corriendo —no hay tabla que lo guarde— y hoy fue exactamente lo que permitió descubrir que
dos sesiones se estaban pisando los binarios. Con dos esquemas de numeración conviviendo, ese
aviso pasa a mentir: un rollback real se ve igual que un choque de convenciones.

`deploy/construir.sh` deriva la versión de dos cosas que no se pueden discutir: el archivo
`VERSION` y el commit. Y agrega **`-sucio`** cuando el árbol tiene cambios sin commitear — no lo
prohíbe (a veces hace falta probar algo rápido) pero lo DECLARA, porque un binario construido de
un árbol sucio no se puede reconstruir después: el commit que anuncia no es el código que corre.
Hoy la etiqueta ajena se pudo auditar justamente porque su commit existía.

**No se redesplegó para corregir la etiqueta**: el código que corre es el mismo commit, y reiniciar
el cerebro por un número sería pagar una indisponibilidad por cosmética. Se corrige solo en el
próximo.

**2026-08-30 (6) · A64 CERRADO — renombrar una máquina existe, y avisa lo que va a cambiar.**

Se abrió hoy porque `kernelos-pc` (Musubi), `davantis-1` (tailscale) y «Davantis» (quien la usa)
son el mismo equipo, y esa divergencia hizo que un diagnóstico se corriera en la máquina
equivocada. Hasta ahora renombrar no existía: había que dar de baja y volver a enrolar, lo que da
un id NUEVO — la máquina aparece vacía y su bitácora, sus sesiones y sus servicios quedan colgando
de un device revocado con otro nombre. **Cambiar un nombre costaba el historial entero.**

**Y AL IR A DISEÑARLO APARECIÓ LO QUE IMPORTABA DE VERDAD: renombrar NO es cosmético, es un cambio
de autorización disfrazado.** Tres cosas de este repo indexan por NOMBRE y ninguna por id:

	tieneGrant        → las concesiones de capacidad de `principals.yaml`
	argvPermitido     → la allowlist de comandos por máquina (`exec_allow`)
	Politica.Alcanza  → a qué máquinas alcanza una política (`config.yaml`)

Un rename puede sacarle `exec` a alguien, dárselo, o meter una máquina adentro del alcance de una
política que la va a tocar sola. Todo en silencio, con el síntoma apareciendo días después como
«esto dejó de andar».

- **NO RENOMBRA EN EL PRIMER LLAMADO.** Informa y se planta. Es la misma forma que tiene todo lo
  caro de este track: el default de «no lo pensé» es que no pase nada.
- **INFORMA LAS DOS DIRECCIONES.** Lo que va a dejar de alcanzarla (quien nombra el nombre viejo)
  y —el sentido que a nadie se le ocurre mirar— **lo que la máquina va a HEREDAR**: si algo ya
  nombraba el nombre nuevo, la renombrada se queda con esa autorización sin que nadie se la haya
  dado.
- **EL COMODÍN NO SE LISTA.** Una concesión sobre todas las máquinas sobrevive a cualquier rename;
  listarla sería ruido que tapa lo que sí se rompe.
- **LA ALLOWLIST VA APARTE DE LAS CONCESIONES** porque se pierden distinto: quedarse sin concesión
  niega el acceso, que es ruidoso y se nota; quedarse sin entrada de `exec_allow` con la SECCIÓN
  presente deniega TODO comando por el paso 4 de `argvPermitido` — igual de silencioso y mucho más
  confuso.
- **NO EDITA `principals.yaml`.** Eso sería el cerebro cambiando la credencial de una persona, y
  las concesiones se escriben a mano a propósito (B3). Lo que sí hace es decir exactamente qué
  editar y dónde.
- **QUEDA EN EL LOG DEL CEREBRO** con las dos puntas: un rename cambia una columna y no deja
  rastro en ninguna tabla, así que sin esa línea la pregunta «¿por qué esta máquina se llama
  distinto?» no tiene respuesta en ningún lado.

De paso entró `fleet.NombreDeDeviceValido`: el nombre viaja a etiquetas de Prometheus, a los
selectores de `principals.yaml` y a cada línea de log, así que no puede llevar control, comas ni
comillas. **NO se aplica en el alta todavía** —`ValidarAlta` sólo exige que no esté vacío— y eso
está dicho en el propio comentario: apretarlo ahí exige mirar qué hay enrolado antes de empezar a
rechazar, y no se hace de rebote.

**Siete sabotajes.** Y una lección del MÉTODO, repetida: la tanda se pasó del tiempo y dejó uno
aplicado en el árbol. Esta vez lo verifiqué a propósito antes de seguir —`diff` contra los
respaldos— en vez de que lo cazara el test siguiente por un motivo que no tenía nada que ver.
La segunda tanda corrió en segundo plano y terminó con esa verificación adentro del script.

**2026-08-30 (5) · A63 CERRADO — la causa no era la red: era una ventana que estorbaba.**

`gio` llevaba TRES DÍAS fuera de la flota. La máquina estaba viva —ping en 2,9 ms, tailscale la
veía, puerto 22 abierto—, así que lo muerto era el agente. Y el motivo estaba en una sola línea:

    LastTaskResult 3221225786  =  0xC000013A  =  STATUS_CONTROL_C_EXIT

**Alguien cerró la consola negra porque molestaba.** No se apagó la máquina, no falló la red, y no
fue NordVPN —que sólo afecta a la OTRA Windows, algo que yo había supuesto mal y corregí al ir a
mirar—.

**UNA PIEZA DE INFRAESTRUCTURA QUE VIVE DENTRO DE UNA VENTANA MOLESTA SE APAGA SOLA**, tarde o
temprano. Eso no es comodidad: es un modo de falla, y llevaba tres días activo sin que nadie
supiera por qué. El instalador ahora trae `-Oculto` y las dos Windows ya corren así.

**LA FORMA OBVIA DE OCULTARLA ERA LA EQUIVOCADA.** `-AlArranque` también esconde la ventana, y de
paso sobrevive al cierre de sesión — pero corre como SYSTEM, o sea en la SESIÓN 0, y ahí no hay a
quién dibujarle: `puede_preguntar` pasa a false y todo el eje de consentimiento de A57 se apaga en
esa máquina. Esconder una ventana no puede costar la capacidad de preguntarle a la persona. Con
`-Oculto` el agente sigue siendo el usuario, en su sesión.

**Y EL DETALLE QUE HABRÍA CONVERTIDO EL ARREGLO EN UNA REGRESIÓN SILENCIOSA**: el tercer argumento
del `Run` del lanzador. Con `False`, el `.vbs` arranca el agente y sale, la tarea se da por
CUMPLIDA en el acto, y el `-RestartCount 999` deja de protegerla — si el agente muere, nadie lo
levanta. La ventana desaparece (lo que se ve) y la red de contención también (lo que no). Va con
`True`.

**LO QUE QUEDA Y SE ACEPTA, dicho para no volver a discutirlo:** los 11.007 comandos vencidos de
`gio` siguen en su bitácora. Podarlos es borrar auditoría, y el daño real está acotado —la
cronología es por VENTANA, así que sólo los ve quien mire el 28 y el 29 de agosto—. Se quedan.

Y el agente sigue muriendo al cerrar sesión: eso es lo que `-AlArranque` arregla, con su costo, y
es una decisión que hoy no hace falta tomar.

**2026-08-30 (4) · A36 CERRADO — y en el camino apareció una alerta que NO PODÍA DISPARARSE.**

A36 decía «no hay regla de alerta ni target de scrape» y hacía falta un `blackbox_exporter`. Eso
quedó viejo: el inventario de servicios (S12) llegó después, el agente enumera los dos
contenedores del relay solos, y `ServicioCaido` ya avisa si el proceso muere. **Media pieza estaba
puesta y nadie lo había mirado.**

Lo que faltaba era la otra mitad, la que rompe de verdad: **un contenedor levantado que no acepta
conexiones**. Para `musubi_fleet_service_up` eso se ve idéntico a sano —es exactamente la
distinción que motivó la fase 4— hasta que alguien no puede abrir una pantalla, que era el síntoma
anotado.

- **`reportar-relay.py`**, con el molde de `reportar-bot.py`: abre los tres puertos TCP del relay
  y reporta por `/fleet/service-health`. Sin pieza nueva: reusa el camino que la fase 4 ya dejó
  desplegado.
- **SERVICIO PROPIO Y NO EL DEL CONTENEDOR.** Reportar sobre `musubi-rustdesk-hbbs` lo pondría a
  pelear con el AGENTE por la misma fila, y la salud la decidiría una carrera entre dos
  reportantes. `relay-rustdesk` significa otra cosa —«responde en sus puertos»— y son dos
  preguntas distintas.
- **EL UDP NO SE PRUEBA.** Sin handshake no hay forma de distinguir «llegó y nadie contestó» de
  «se perdió el paquete», y una medición que no distingue esas dos cosas es ruido con forma de dato.

**Y ACÁ APARECIÓ LO GRUESO.** Al verificar la cadena completa, el dato estaba guardado
(`latencia_p95_ms: 1`) y la serie NO existía en Prometheus. La causa: **el receptor OTLP le agrega
la unidad canónica al nombre** cuando el nombre no termina en ella, así que
`musubi_fleet_service_latency_p95_ms` con `Unit: "ms"` entra como
`musubi_fleet_service_latency_p95_ms_milliseconds` — y `ServicioLento`, que consulta el nombre
declarado, **no podía dispararse nunca**.

Estuvo escondido porque DOS omisiones se tapaban entre sí: las reglas de la fase 4 no estaban
desplegadas *y* ningún servicio reportaba latencia todavía. Con cualquiera de las dos presente el
síntoma seguía siendo silencio, que se ve igual que «todo bien». Y este camino **no tiene otra
red**: el scrape de `/metrics` descarta `musubi_fleet_.*` a propósito —para que cada dato tenga un
solo productor—, así que una serie mal nombrada en el empuje no existe en ningún lado.

**Y NO ERA LA PRIMERA VEZ.** En el TSDB quedó `musubi_fleet_device_cpus_bytes`, un fantasma sin
muestras: un binario anterior declaraba la CANTIDAD DE NÚCLEOS con unidad `By`. Se arregló en
algún momento sin que nadie escribiera por qué — el mecanismo se descubrió, se corrigió y se
olvidó. Por eso ahora es una prueba y no un comentario:
`TestNingunaSerieCambiaDeNombreAlEntrarPorOTLP` recorre las 26 series y exige que la unidad esté
vacía o que el nombre ya termine en su forma canónica.

**La prueba se negó a suponer**: al escribirla marcó dos unidades que yo no había mapeado (`%` y
`Cel`) en vez de dejarlas pasar. Las dos se verificaron CONTRA EL PROMETHEUS DE PRODUCCIÓN antes de
darlas por buenas.

**2026-08-30 (3) · A62 — la cola de un dispositivo ya no crece sin techo. Lo pidió una medición.**

Al ir a mirar por qué el agente de `gio` no levantaba su cola, el número no era el que yo había
dicho: **11.007 comandos pendientes**, no cien. Encolados entre el 28 a las 08:33 y el 29 a las
19:06 —treinta y cuatro horas a cinco por minuto— contra una máquina que llevaba dos días sin
latir. El lazo paró solo; no había nada que apagar.

**NO EXPLOTÓ NADA, Y ESO ERA SUERTE DE DISEÑO.** F10 vence todo lo que pasa de quince minutos
antes de entregarlo, así que ni uno se habría ejecutado cuando la máquina volviera. Pero eso es
una consecuencia, no un límite: `EncolarComando` aceptaba lo que le dieran, la tabla crecía sin
tope y la bitácora de esa máquina quedaba ilegible para siempre. **El día que el lazo corra contra
una máquina VIVA, lo que se acumula sí se ejecuta.**

- **EL TECHO VA EN `EncolarComando` Y NO EN LA TOOL**, porque ésa es la única puerta de escritura:
  la comparten el exec, la pantalla, la shell y el motor de políticas. Un techo en una sola de las
  cuatro es un techo que se esquiva por las otras tres.
- **SE CUENTA SÓLO LO QUE TODAVÍA PODRÍA EJECUTARSE**, y ésa es la parte que hay que pensar. Si
  contara todo lo pendiente, una máquina que estuvo caída un día quedaría bloqueada PARA SIEMPRE
  —sus miles de filas muertas ocupando el cupo— y destrabarla exigiría borrar bitácora, que es
  justo lo que este repo no hace. Lo vencido no es presión de cola: es historia.
- **DE PASO ES UN FRENO DE RITMO**: cincuenta dentro de cualquier ventana de quince minutos. El
  lazo de `gio` iba a setenta y cinco por ventana, así que habría rebotado a los cincuenta incluso
  contra una máquina viva.
- **«COLA LLENA» NO ES UN ARGUMENTO INVÁLIDO.** Devolverlo como tal manda a la persona a corregir
  su comando cuando el problema es la MÁQUINA: el mismo argv sobre otro host entra sin chistar. El
  mensaje dice qué mirar y con qué tool.

**Tres sabotajes.** Uno no compiló la primera vez —variable sin usar— y no probaba nada hasta
rehacerlo. Y una prueba mía mentía sobre lo que simulaba: encolaba el «pasado» de una máquina
caída con todas las marcas de tiempo IGUALES, y así cada comando ve a los otros como frescos y el
techo salta con razón. Una acumulación real viene repartida en el tiempo, y así quedó.

**2026-08-30 (2) · A59 CERRADO — la cronología ya puede decir qué lo hizo una regla.**

Se abrió esta mañana, escribiendo S13: una política y una persona escriben en `device_commands`
con la MISMA forma, y la diferencia se leía del nombre del principal. En una línea de tiempo eso
importa: «auto-heal reinició nginx cuarenta veces» y «alguien llamado auto-heal lo reinició
cuarenta veces» son dos relatos distintos, y sólo uno es cierto.

- **SE GUARDA, NO SE DERIVA** (migración 41, `device_commands.origen`). Derivarlo del nombre del
  principal contra `config.yaml` sería lo barato y sería falso: las políticas se agregan y se
  sacan, así que un comando de hace tres meses disparado por un principal que hoy ya no es una
  política se etiquetaría como manual. El resto del dominio deriva lo que sigue siendo cierto
  ahora —que una sesión venció, que un comando expiró— y guarda lo que OCURRIÓ. Ésta es de las
  segundas.
- **EL DEFAULT `''` SIGNIFICA «NO SE SABE» Y NO «PERSONA».** Es el cero mentiroso de A39 llevado al
  origen: rellenar las filas viejas con `persona` haría que cada disparo automático anterior a la
  migración figure como una acción humana. En la cronología de una máquina eso es atribuirle a
  alguien algo que no hizo. Un backfill por nombre de principal tampoco sirve: reproduciría la
  convención que la columna viene a reemplazar.
- **`automatico` ES LISTA BLANCA**: lo desconocido no es automático **y tampoco es manual**. Las dos
  superficies lo emiten en null, que es la única forma de que un panel pueda dibujar la diferencia.
- **LAS DOS SUPERFICIES, otra vez** (A39): la bitácora y la cronología leen la misma tabla y no
  pueden contar dos historias.

**Ocho sabotajes, y tres enseñaron lo mismo que ya había enseñado el día**: probar el CABLEADO y no
el campo. Sembrar el comando a mano con `Origen: politica` puesto verifica que el campo viaja, no
que alguien lo setea donde tiene que setearlo — así que el sabotaje de quitarle el origen al motor
de políticas quedaba verde. La verificación se movió al test que ya recorría el camino entero
(`TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas`), y el control de `persona` pasó a
entrar por `musubi_fleet_exec` en vez de por el motor.

**Y un tropiezo del método, no del código**: la tanda de sabotajes se pasó del tiempo ANTES de
restaurar el último, así que quedó aplicado en el árbol. Lo cazó el test siguiente, que falló por
un motivo que no tenía nada que ver con lo que estaba probando. Un sabotaje que no se revierte es
un bug introducido a mano.

**2026-08-30 · FASE 5 · S14 — EL CRUCE: la flota le pregunta a la memoria.**

S13 contesta qué HIZO Musubi en una máquina. Esto contesta qué SABÍA — y cruzarlas es lo único de
todo el track que ningún panel del mercado puede dar, porque ninguno tiene al lado la memoria del
equipo y su código.

- **CORRELACIÓN, NO CAUSA, y no es una advertencia legal: es el diseño.** Lo tentador es que
  conteste «anda lenta PORQUE el martes se desplegó X». Lo haría ADIVINANDO, y una causa adivinada
  con aire de certeza es peor que no decir nada: manda a alguien a arreglar lo que no está roto y,
  la segunda vez que acierta, se le empieza a creer. Junta los hechos y declara cómo los enlazó.
- **LOS DOS ENLACES NO SE MEZCLAN, y el fuerte gana.** `termino` es que el texto NOMBRA la máquina
  o uno de sus servicios; `ventana` es que sólo coincide en el tiempo. Presentados iguales,
  cualquier coincidencia temporal se lee como una pista. Cuando una nota entra por las dos vías se
  conserva el fuerte: al revés perdería justo el peso que la hace útil.
- **LOS TÉRMINOS SALEN DEL INVENTARIO, no del texto de los comandos.** Sacar palabras del argv
  parece obvio y sobre datos reales produce basura —`cmd`, `type`, `/c`, una ruta de Windows—.
  Salen el nombre de la máquina y sus servicios: pocos, exactos, explicables. Un término que no
  sirve pasa a ser un problema de inventario, que sí se arregla.
- **LOS TÉRMINOS SON INFORMACIÓN Y SE COMPUERTAN.** Decir «busqué postgres en esta máquina» es
  decir que ahí corre un postgres, que es lo que `metrics` protege. Sin esa capacidad no se arman,
  y la respuesta marca `servicios_ocultos` en vez de devolver una lista corta que se leería como
  «esta máquina no corre nada».
- **LA MEMORIA SE LEE EN EL PROYECTO DE LA MÁQUINA**, no en el alcance de quien pregunta. Un
  `read: all` que usara SU alcance traería notas de otro tenant: no sería una fuga —puede verlas
  por otra puerta— pero sí una RESPUESTA FALSA, con el sello de una herramienta que dice haber
  correlacionado.
- **LA MURALLA 2 VALE TAMBIÉN POR ESTA PUERTA.** El predicado canónico lo cumplen nueve consultas
  de recall; ésta es la décima y la PRIMERA QUE LLEGA DESDE FUERA DEL RECALL, que es por donde una
  muralla se rodea sin querer.
- **LA COMPUERTA Y LA RESOLUCIÓN DE MÁQUINA SE EXTRAJERON de S13**, no se copiaron. Una compuerta
  duplicada es la peor duplicación posible acá: la copia que se queda vieja es la del camino que se
  usa menos, y «quedarse vieja» significa mostrarle a alguien un plano que no le corresponde.

**LAS FECHAS, QUE ES DONDE ESTO SE ROMPE EN SILENCIO (A61).** En la misma base conviven dos
formatos: la flota escribe RFC3339 desde Go y la memoria deja que SQLite ponga `CURRENT_TIMESTAMP`.
Comparar con el equivocado NO da error: da vacío, y ese vacío se lee como «no había nada escrito
ese día». Y hay una trampa arriba de la trampa: **el driver convierte al LEER y no al COMPARAR**
—`modernc.org/sqlite` devuelve RFC3339 sobre una columna `DATETIME` aunque los bytes no lo estén—,
así que mirar lo que vuelve en Go lleva a la conclusión equivocada sobre cómo comparar.

**18 SABOTAJES, Y LOS TRES QUE NO ROMPIERON FUERON PRUEBAS MÍAS DEMASIADO FLOJAS.** No código
débil: verificación débil, que es peor porque no se nota.

- Comparar la ventana en RFC3339 quedaba verde con una ventana de 24 h, porque a esa escala
  **manda la fecha y no la hora**. La guarda se rehízo con la hora FIJADA y una ventana del mismo
  día.
- Sacar el predicado de visibilidad quedaba verde porque **ninguna prueba sembraba una observación
  tapada**.
- Contar el mínimo de un término en bytes quedaba verde porque elegí «año»: 4 bytes y 3 runas,
  que pasa contando de las dos formas. Con «ño» las dos cuentas se separan.

**2026-08-29 (8) · FASE 5 ARRANCA — S13, la cronología: qué le pasó a UNA máquina, en UNA ventana.**

Las tres bitácoras contestan «lo último que pasó en este plano». Ninguna contesta la pregunta que
alguien hace de verdad cuando algo anda mal, que es **«¿qué le pasó a esta máquina?»** — y hoy eso
se responde llamando a tres tools y ordenando a mano, o sea que no se responde. Al cruzarlas a mano
se pierde justo lo que importa: ver que la sesión de shell de las 14:02 y el comando de las 14:03
son la misma historia.

Es la FUNDACIÓN de la fase 5 y no su final. Correlacionar «desde el martes anda lenta» con algo
exige, primero, poder listar qué pasó el martes; lo que se cruza con la memoria y con el grafo de
código se apoya en esto.

- **SÓLO TABLAS APPEND-ONLY.** La tentación es armarla con todo lo que tenga fecha: `last_seen`,
  `last_report`, `last_fired`. Ésas guardan el ÚLTIMO valor, así que una política que disparó
  cuarenta veces el martes saldría como UNA línea con la hora de la última — y la cronología
  mostraría, con toda confianza, un martes tranquilo. **Un renglón que resume cuarenta es peor que
  un renglón ausente: el ausente se nota.** Las políticas SÍ están igual, porque su acción se
  encola en la misma bitácora que la de una persona (I16).
- **LA COMPUERTA VIAJA CON EL HECHO, no con la consulta.** Las tres fuentes tienen tres compuertas
  (`exec`, `screen:view`, `shell`). Compuertar la lista entera con UNA falla en las DOS
  direcciones: con la más laxa le muestra a alguien con `exec` quién tuvo un prompt; con la más
  estricta le esconde sus propios comandos a quien puede correrlos.
- **EL TIPO LO DECIDE LO QUE EL HECHO REVELA, no la tabla.** Una fila de `device_commands` cuyo
  argv es `musubi:pantalla` revela que alguien miró una pantalla: pide `screen:view`.
- **EL DEFAULT ES NO MOSTRAR.** Una operación interna que este cerebro no conoce no se le muestra a
  nadie, ni al que tenga todo, y se cuenta aparte. Si el default fuera `exec`, una operación nueva
  aparecería ante todo el que pueda ejecutar antes de que nadie decida quién puede verla. Ese
  fail-closed sería correcto Y SILENCIOSO, así que lo rompe una prueba que **lee el fuente**: toda
  `"musubi:*"` que el cerebro o el agente nombren tiene que estar clasificada. Se lee el fuente y
  no una lista declarada porque una lista declarada es justo lo que alguien se olvida de actualizar.
- **SIN NINGÚN PLANO VISIBLE SE EXPLICA, no se devuelve vacío**, y es lo contrario de lo que hacen
  las otras cuatro lecturas de flota. Una lista vacía en una LÍNEA DE TIEMPO se lee como una
  conclusión —«no pasó nada en esa máquina»—, no como una ausencia de datos.
- **`no_visto` VIAJA SIEMPRE**, también en una respuesta llena: no hay serie temporal (B5), ni logs
  del host, ni historial de salud de servicios, ni contenido de sesiones. Un registro que no aclara
  contra qué NO protege es peor que ninguno, porque alguien deja de buscar el de verdad.

**EL BUG QUE ENCONTRÓ EL CONTROL POSITIVO, Y NO LA ASERCIÓN DE FUGA.** El barrido de aislamiento
exige dos cosas: que el atacante NO vea el marcador ajeno y que el admin federado SÍ lo vea. Lo
primero pasó de una; lo segundo falló, y ahí estaba el bug: las fechas se guardan en RFC3339 **sin
fracción de segundo**, así que una ventana que termina «ahora» —`22:29:58.7`— se formatea como
`22:29:58`, y con el borde superior abierto **el comando encolado en ese mismo segundo queda
afuera**. El síntoma es el que más engaña de todos: alguien reinicia un servicio, entra a mirar y ve
la cronología vacía. Ahora las dos puntas se redondean hacia AFUERA, y una punta sin fracción no se
mueve — así el mosaico de ventanas consecutivas sigue sin contar dos veces el hecho del borde.

**Y una consecuencia del arreglo que valía por sí sola**: la comparación léxica de fechas era una
COSTUMBRE, no una regla. Dos consultas que ya existían dependían de que todo estuviera guardado en
UTC —el vencimiento de la cola y la poda de salidas— y funcionaban porque en producción nadie pasa
una fecha propia. Los cinco INSERT de flota ahora normalizan con `.UTC()`: la garantía pasó a ser
por construcción y no por suerte.

**Otras dos cosas que este slice ordenó de paso:** el saneo del argv de pantalla estaba escrito en
la tool de la bitácora y la cronología habría sido la segunda copia —y la copia que se queda vieja
es siempre la del camino que se usa menos—, así que bajó al dominio (`fleet.ArgvDeBitacora`) y se
aplica al CONSTRUIR el hecho, donde ninguna superficie futura puede olvidarse de llamarlo. Y los
nombres de las operaciones internas (`musubi:pantalla` y las otras tres) bajaron con él: estaban
como literales sueltos en tres paquetes.

**El barrido de aislamiento también mejoró**: `respText` abortaba el test ante un `RpcError`, lo que
dejaba fuera del barrido a toda tool que le NIEGUE el pedido al atacante en vez de devolverle una
lista vacía — o sea que la conducta mejor era la que no se podía verificar. Ahora el error también
se revisa contra el marcador, porque un mensaje que nombrara el dato ajeno filtraría igual que una
fila.

**22 sabotajes ejecutados.** Dos no rompieron a la primera: uno porque la normalización de la
ventana está en DOS capas y la prueba sólo cubría una (se agregó la que faltaba, en el motor), y
otros dos porque el sabotaje **no compilaba**, que no prueba nada — se rehicieron para que
compilaran y fuera la prueba la que fallara.

**Y UN HALLAZGO QUE NO SALIÓ DE NINGUNA PRUEBA, SINO DE USAR LA TOOL CONTRA LA FLOTA REAL.** La
primera cronología de una máquina Windows devolvió cincuenta comandos, todos en `pendiente`,
encolados **diez horas antes** — con una vida máxima de quince minutos. La causa: `expirado` se
estampa en UN solo lugar, adentro de `TomarComandos`, o sea **cuando el agente viene a pedir su
cola**. Si el agente no vuelve, nadie estampa nada.

Es la regla que este repo aplica en todos lados —«una columna de estado que hay que ir a actualizar
miente en cuanto nadie la actualiza»— y que las dos clases de sesión ya respetaban. Los comandos se
habían quedado afuera. Peor: **`Comando.Vencido` YA EXISTÍA, escrita y probada desde S5, y no la
llamaba nadie** — el mismo patrón de A58, capacidad construida e inalcanzable.

Se cableó por `EstadoActual` en **las dos** superficies, que es la lección de A39: una guarda sobre
una sola deja la otra mintiendo. Y ahí hubo un segundo error mío que atrapó justo esa prueba: puse
la derivación en la fila de la bitácora y `conResultado` la pisaba con el estado crudo dos líneas
después, así que el arreglo era **código muerto**. La prueba que compara las dos superficies lo
encontró; leer el diff, no.

**LO QUE ESTE SLICE DEJA ABIERTO, y hay que decirlo:** la bitácora no distingue el origen automático
del manual. Una política y una persona escriben con la misma forma y la diferencia se lee del nombre
del principal, por convención. Registrado como **A59**.

**2026-08-29 (7) · A57 CERRADO — `pide` ya puede preguntar, y el eje de consentimiento se usa entero.**

Faltaba el transporte, no la política: el latido va cada 30 s y el diálogo espera 60, así que una
respuesta tarda hasta minuto y medio. **Bloquear la llamada habría puesto un timeout de red en el
camino de una decisión humana**, donde el vencimiento significa otra cosa. Se partió en dos.

- **Un `pide` devuelve la espera y NO una contraseña.** El pedido crea la sesión en
  `esperando_permiso`, encola la pregunta y devuelve el id. **No se acuña nada todavía**: una
  credencial que existe se puede filtrar aunque nadie la use, y no se sabe si van a decir que sí.
  El operador vuelve y recibe la contraseña —o el motivo—.
- **Preguntar dos veces no está permitido.** Si ya hay una espera en curso se informa, no se
  repregunta: dos ventanas encima de la misma persona por el mismo pedido es cómo se le enseña a
  alguien a apretar «permitir» sin leer. Y una espera VENCIDA no bloquea las siguientes.
- **El permiso no es la credencial.** La vuelta pasa otra vez por toda la compuerta de
  capacidades: entre que se concedió y que se pide la contraseña pueden haber revocado la máquina,
  y el permiso del usuario no vale como autorización del sistema.
- **La respuesta viaja en stdout y no en el código de salida.** Un exit distinto de cero se lee
  como «el comando falló», y «el usuario dijo que no» NO es una falla: es el sistema haciendo lo
  que se le pidió. Con prefijo fijo, para que una salida inesperada no se interprete como respuesta.
- **Los TRES «no» se distinguen y cada uno dice qué hacer**: `negada` es una decisión que hay que
  respetar; `sin_respuesta` dice que esa máquina quizás no debería estar en `pide`; `no_se_pudo`
  que le falta software o le sobra aislamiento. Viven en **columna propia** (migración 40) y no en
  el texto de `error`, porque metidos en un texto libre la diferencia sobrevive exactamente hasta
  que alguien mejora la redacción del mensaje.
- **Una máquina no puede contestar por otra**, y la defensa es doble: la puerta de `/fleet/result`
  ya rechaza el comando ajeno, y `ResponderConsentimiento` lo rechaza otra vez con su propio
  `AND device_id = ?`. La segunda es la que sobrevive si alguien reordena el handler.
- **Sólo se contesta una vez**, y esa condición está en el WHERE: sin eso un agente podría mandar
  «negada» y después «concedida», y la bitácora registraría la última — que es la que un atacante
  elegiría.
- **`entregarPantalla` está EXTRAÍDA y no duplicada**, porque ahora tiene dos llamadores. Copiarla
  dejaría dos lugares donde acuñar credenciales y dos donde recordar que el argv no puede llegar a
  la bitácora; la copia que se queda vieja es siempre la del camino que se usa menos, que acá es
  justo el de mayor autoridad.

**DOS ERRORES MÍOS, LOS DOS ATRAPADOS POR PRUEBAS:**

**Inserté la migración 40 ANTES de la 39.** `latestSchemaVersion()` devuelve la versión del ÚLTIMO
elemento del slice, no la mayor, así que pasó a decir 39 con la 40 ya escrita. Lo atrapó de rebote
la guarda de la plantilla de A45 —escrita hoy para otra cosa—, y eso fue suerte: **no había nada
que exigiera el orden**. Ahora sí: `TestLasMigracionesEstanEnOrdenAscendenteYSinHuecos`.

**`AbrirSesionPantalla` pisaba el estado** con `solicitada` siempre. Estaba bien mientras ése era
el único comienzo posible; A57 agregó el otro. Se abrió la lista a los DOS estados iniciales
legítimos, cerrada a propósito: sin esa restricción un llamador podría abrir una sesión
directamente en `activa` y la bitácora registraría un acceso que nunca pasó por la compuerta.

**Y un sabotaje declarado que no rompía nada**: quitar el `cerrada = nil` al conceder. No afecta el
flujo — pero sí el PANEL, que mostraría como terminada una pantalla que alguien está usando. La
prueba no lo cubría; ahora sí, y entonces el sabotaje sí falla.

**Y una tercera mentira, encontrada mirando el panel y no el código.** Una sesión en
`esperando_permiso` tiene `cerrada` vacío y tres minutos de ventana por delante, así que pasaba
las dos condiciones de `SesionViva.Abierta` — el panel de sesiones vivas habría dicho **«alguien
está mirando esta pantalla» cuando todavía nadie dio permiso y no hay contraseña acuñada**. Es
peor que un falso negativo: manda a alguien a interrumpir una sesión que no existe, o le enseña
que la columna miente. `sin_permiso` no necesita excepción porque cae por `cerrada`; éste es el
único estado que sí, y por eso se nombra en vez de barrer con una lista.

**Nueve sabotajes ejecutados** en esta parte.


**2026-08-29 (6) · A57, la mitad del `avisa`: el agente aprende a hablarle a la persona.**

El eje de consentimiento estaba completo del lado del cerebro —la política se fija, se guarda, se
resuelve por la más restrictiva, se aplica— y **vacío del otro**: nadie en la máquina destino sabía
dibujar nada. Mientras tanto `puede_preguntar` era 0 para toda la flota, así que un `pide` se
endurecía a `prohibido`, y un `avisa` abría la sesión dejando un WARN que decía que el aviso no se
pudo entregar. Las dos cosas honestas, ninguna suficiente.

- **«Saber avisar» se MIDE, y se miden las DOS mitades**: que haya dónde dibujar Y con qué. Una
  sola no alcanza — un Linux con `DISPLAY` y sin `zenity` no puede avisarle a nadie, y un servidor
  con `zenity` y sin sesión gráfica tampoco.
- **El motivo viaja pegado al `false`.** Sin él, un `pide` endurecido a `prohibido` en toda la
  flota es un cero sin explicación, y las tres causas —no hay escritorio, falta un paquete, el
  agente corre como servicio— se arreglan distinto.
- **`puede_preguntar` es un PUNTERO en el cuerpo del latido**, y ésa es la decisión que evita
  romper una flota mezclada: un agente VIEJO no manda el campo, y con un bool pelado eso sería
  indistinguible de uno que midió y dijo que no. El nil se saltea; el `false` explícito sí escribe.
- **La trampa de Windows, atajada donde vive.** Desde Vista los servicios corren en la SESIÓN 0,
  aislada del escritorio: un `MessageBox` lanzado desde ahí **no falla y no lo ve nadie**, y el
  proceso espera un clic que nunca llega. Eso es un `pide` que parece funcionar, tarda 60 s y
  termina en «nadie contestó» cuando la verdad es que no había dónde preguntar. La detección mira
  la SESIÓN del proceso, no si el binario existe.
- **Y se resolvió con stdlib pura.** `golang.org/x/sys/windows` trae `ProcessIdToSessionId`
  envuelto — y usarla la promovería de indirecta a DIRECTA, o sea la **séptima** dependencia de un
  repo que tiene seis por decisión. Se usó `syscall.NewLazyDLL`, el mismo patrón que
  `colector_windows.go`. `go.mod` no se movió.
- **En zenity el exit 0 NO es «anduvo», es «dijo que sí»**, y en osascript el exit code no sirve
  para nada porque vale 0 para cualquier botón. Leer mal cualquiera de los dos concedería acceso
  cada vez que la ventana llega a abrirse; están escritos con su porqué al lado.
- **`notify-send` se declara como NO capaz.** Avisa pero no pregunta, y la capacidad que este eje
  mide es la de PREGUNTAR: media capacidad reportada como entera endurecería mal un `pide`.
- **El texto del aviso nombra a quien entra.** «Alguien está viendo tu pantalla» no le sirve a
  nadie: sin el nombre no hay a quién preguntarle, y el aviso se vuelve ruido que se cierra sin leer.
- **No se encola un aviso que la máquina no puede mostrar**: quedaría pendiente para siempre en su
  cola y ensuciaría la bitácora. Se deja la constancia en el log, como antes.

- **Un aviso que espera un clic no es un aviso, es una pregunta sin opciones.** `zenity --warning`
  BLOQUEA hasta que alguien aprieta OK: la primera versión dejaba al agente esperando los diez
  segundos del timeout en CADA aviso, y como atiende los comandos EN SERIE, esa máquina se quedaba
  sin atender nada más —ni exec, ni pantalla, ni shell—. El cerebro la vería latiendo y muda, que
  es el peor estado porque parece sano. Ahora el aviso prefiere `notify-send` (vuelve en el acto)
  y el zenity de respaldo lleva `--timeout`. Son dos preguntas distintas —«¿con qué aviso?» y
  «¿con qué pregunto?»— y la herramienta buena para una es mala para la otra.
- **Y la guarda del cuerpo del latido hizo su trabajo**: `TestElCuerpoNoLlevaIdentidadNunca` tiene
  lista BLANCA, así que las dos claves nuevas rompieron la suite hasta que se las declaró — con el
  examen que su propio mensaje exige. Ninguna dice QUIÉN ES la máquina: son lo que sabe de SÍ
  MISMA, como `version` y `direccion`, y la única fila que pueden tocar sigue siendo la del token.

**Nueve sabotajes ejecutados.** Uno de ellos no lo atrapa una prueba sino el COMPILADOR —hacer que
`sin_respuesta` sea un alias de `negada` produce dos `case` idénticos en un switch— y queda anotado
así en vez de atribuírselo a la suite.


**2026-08-29 (5) · A58 cerrado, y el bug que apareció al USARLO.**

La exposición Tier B estaba construida, probada y desplegada, y **no la usaba nadie**. Se cableó
contra la base de Altura en Supabase: `altura-db` enrolada como Tier B con `metrics` y nada más,
`.musubi/flota-exposicion.yaml` apuntando al endpoint, y la credencial en el entorno del cerebro
—sólo su NOMBRE viaja en el archivo—. El sondeo mide por exposición y devuelve CPU, memoria y disco.

**EL `montaje` NO ERA UNA SUTILEZA.** El endpoint expone dos sistemas de archivos:

    /       71,7 GiB   ← el sistema operativo del contenedor
    /data    7,8 GiB   ← el volumen de la base, que es el que se llena

Sin declarar `montaje: /data` se mira la raíz, y entonces **una base LLENA se ve como ~10 % de
disco usado**. No es una imprecisión: es una alarma que no suena nunca. Medido con el endpoint
real: `/` al 12,5 % contra `/data` al 5,0 %.

**Y AL MIRAR LA PRIMERA RESPUESTA APARECIÓ A39 OTRA VEZ, EN LA TERCERA SUPERFICIE.**

El sondeo devolvió `uptime_seg: 0`. El endpoint de Supabase **no expone `node_boot_time_seconds`**
—cero líneas, medido—, así que ese 0 significaba «no se midió» y se leía como «arrancó recién»:
plausible, falso, y manda a investigar un reinicio que no pasó. `num_cpu` igual.

A39 arregló `filaDeMetricas` y el exportador **y ató los dos con una prueba**. La fila del SONDEO
quedó afuera de esa guarda, y A52 —que sí probó esa fila— cubrió `mem_libre` y `num_procesos` pero
no estos dos. **Son TRES superficies, no dos**, y no lo encontró ninguna prueba: lo encontró usar
el sistema.

**Y la primera guarda que escribí para taparlo no servía.** REHACÍA el tramo final de `sondearUno`
en vez de llamarlo, así que verificaba su propia copia: los dos sabotajes declarados la dejaban en
VERDE, y el comentario que le puse afirmaba lo contrario. Se extrajo `completarFilaDeSondeo` para
que la prueba llame al MISMO código que corre en producción. Con la costura real, los cuatro
sabotajes disparan.

La segunda guarda ata la lista de campos: si alguien agrega uno a la fila del sondeo y no lo
clasifica, la suite se pone roja. Es lo que le faltó a A39 la primera vez.


**2026-08-29 (4) · FASE 4 — Musubi aprende a medir lo que un servicio HACE, no sólo si corre.**

`musubi_fleet_service_declare` existe —lo dice su propia descripción— para declarar «un Tier B que
no enumera solo, **un bot**, un puente». Y hasta hoy **un bot declarado se quedaba en
`desconocido` para siempre**: su salud «pasa a tener estado cuando la máquina lo reporte», y a un
bot que vive en una base gestionada en la nube no lo reporta ninguna máquina. La capacidad estaba
declarada y era inalcanzable — el mismo patrón que este track viene persiguiendo, esta vez en el
plano de control.

Y lo que un bot tiene para decir tampoco entraba en `SaludServicio`: no es «corriendo» ni
«fallado», es «atendí 47 consultas en el último minuto, 3 salieron mal, el p95 fue 820 ms».

- **`fleet.Rendimiento`** (`internal/fleet/rendimiento.go`): ventana, atendidas, fallidas,
  desglose por resultado y latencias. **Acá el CERO es un dato, y es el más importante** — al revés
  que en todo el resto del track. `atendidas: 0` significa «miré y no pasó nada», que es lo que
  distingue un bot callado de un colector muerto. La distinción vive un nivel más arriba:
  `Rendimiento == nil` es «no se midió». Por eso los conteos son `int` y las latencias punteros.
- **La ventana viaja con los números.** «47 atendidas» no significa nada sin saber en cuánto
  tiempo, y deducirla del intervalo del colector ata el gráfico a un número que vive en otra
  máquina — el mismo error que `scheduler_flota.go` documenta al negarse a colgar el empuje del
  intervalo de sondeo.
- **`POST /fleet/service-health`**, con el token del DISPOSITIVO. **No poda y no estampa señal de
  vida**, y las dos cosas son el motivo de que sea una puerta aparte: podar borraría los otros 53
  servicios de la máquina (un colector de un bot no está en condiciones de afirmar un inventario),
  y estampar vida haría que un host cuyo AGENTE murió pero cuyo colector vive figure sano — y el
  colector es lo que menos se cae, porque es un cron de un minuto.
- **No crea servicios, y no es prudencia sino un bug evitado.** El camino del latido crea con
  `declared = 0`, podable por ausencia; el siguiente latido del agente —que enumera systemd y
  contenedores— borraría el bot. El colector lo recrearía un minuto después y el servicio
  parpadearía en el panel para siempre. Un bot entra por `service_declare`, que lo hace inmune.
- **Un reporte imposible no pisa la última salud buena.** Acá NO vale la asimetría del latido
  —guardar el servicio con la salud vacía— porque el servicio ya existe: no hay nada que crear.
- **Las dos superficies coinciden**, con la misma clase de guarda que cerró A39 un nivel más
  arriba: la tool trae `rendimiento` y el exportador emite `_handled`, `_failed`,
  `_window_seconds` y `_latency_p95_ms`, y una prueba las ata. **El desglose NO se exporta**: sus
  claves las elige quien reporta, y una etiqueta cuyos valores decide un tercero es cardinalidad
  sin techo. Se mira en la tool y en el panel, donde una clave nueva cuesta una columna.
- **El panel lo dibuja**, y marca con ⚠ el servicio que corre y falla más de 1 de cada 5 — algo
  que `musubi_fleet_service_up` NO PUEDE VER: systemd ve el proceso vivo conteste bien o mal.
- **Tres reglas de alerta** con su runbook: `ServicioFallandoPorDentro`,
  `ColectorDeRendimientoMudo` (que mira la DESAPARICIÓN de la serie, no su cero) y `ServicioLento`.
  Las tres se apagan solas donde no aplican: un servicio de systemd no dispara ninguna nunca.
- **`deploy/colectores/reportar-bot.py`** reemplaza a `collect-bot.py`. Mismo SQL, mismo marcador,
  mismos números — si algo se rompe al migrar, que se rompa en el transporte y no en la medición.

**Veintiséis sabotajes ejecutados** (10 del dominio, 7 de la puerta, 5 del panel, 4 de la guarda
cruzada), incluido el que importa de verdad: quitarle el `AND device_id = ?` al UPDATE, que dejaría
a cualquier máquina de la flota reportando que el postgres de producción está caído.

**LO QUE FASE 4 NO CIERRA, Y HAY QUE DECIRLO:**

- **`collect-supabase.sh` NO es redundante todavía, al contrario de lo que decía el plan.** Al ir a
  mirar: `.musubi/flota-exposicion.yaml` **no existe** en producción y la base de Supabase **no
  está enrolada** — o sea que **el parseo de exposición Tier B está construido, probado, desplegado
  y sin usar**. Ese colector es hoy el ÚNICO que mira esa base, y apagarlo la dejaría ciega.
  Registrado como **A58**.
- **`collect-infra.sh` sí es redundante campo por campo** —mide exactamente lo que el agente de
  Musubi mide, y nada que Musubi no tenga— pero empuja a OpenObserve, que alimenta la sección
  Monitoreo del CRM. Apagarlo es una decisión de **A33** —que desde el 2026-08-29 es **B20**—, no una limpieza.


**2026-08-29 · A56 — el panel ya tiene su concesión, y la ficha se había quedado abierta.**
Gio la agregó el mismo día y el panel pasó a mostrar vitales y servicios; la fila quedó en la
tabla por olvido. **Verificado en producción el 2026-08-29**: `panel-central` tiene
`fleet: {metrics: ["*"]}` y NADA más — ni `exec`, ni `screen`, ni `shell` —, igual que
`prometheus`. Un panel mira; una credencial que vive en la configuración de otro servicio es
exactamente la que no querés que pueda ejecutar nada.

**2026-08-29 (3) · A45 y A53 — `go test -race ./...` termina, y la suite de todos los días corre
3× más rápido de paso.**

A45 no era un deadlock ni una carrera: la corrida instrumentada nunca reportó un solo `DATA RACE`.
Era que `modernc.org/sqlite` —el SQLite en Go puro que evita cgo— corre ~10× más lento bajo el
detector, y **cada prueba abría una base nueva y aplicaba las 39 migraciones de cero**. Son 280
pruebas en `internal/mcp` y 82 en `internal/memory`.

**Medido antes de tocar nada**, que es lo que decidió el arreglo:

| | sin `-race` | con `-race` |
|---|---|---|
| una prueba con base | 0,79 s | **7,8 s** |
| quince pruebas de `internal/mcp` | — | **69,8 s → 12,9 s** |

Y el resultado, medido igual:

| | antes | ahora |
|---|---|---|
| `internal/memory` sin `-race` | 135 s | **34,6 s** |
| `internal/mcp` sin `-race` | 170 s | **39 s** |
| **`-race ./...` completo** | **nunca terminó en 30 min** | **8 min 12 s, todo verde** |

- **El arreglo es una plantilla que se copia.** Las migraciones son deterministas: se pagan UNA
  VEZ por binario de prueba y después se copia el archivo. La prueba lo siembra ANTES de llamar a
  `NewDbEngine`, cuyo `runMigrations` lee `user_version` = la última y no hace nada.
- **No agrega una sola rama a `NewDbEngine`**, y ésa es la propiedad que importa: una optimización
  de pruebas que metiera una rama en producción compraría velocidad con el riesgo de que el camino
  que corre en la CI no sea el que corre en el servidor. Acá el código es el MISMO en los dos lados.
- **Vive en `internal/memory` y no en `memtest`** porque las pruebas de `internal/memory` son del
  paquete `memory` y no pueden importar `memtest` sin cerrar un ciclo. `memtest` es un envoltorio
  fino con `*testing.T` para los paquetes de afuera; la alternativa era duplicar treinta líneas en
  dos lados, que es exactamente cómo una de las dos copias se queda vieja.
- **`TestLaBaseSembradaEsIdenticaALaMigradaDeCero`** compara el DDL objeto por objeto contra una
  base migrada de cero. Es la garantía de que el arreglo no cambia lo que las pruebas prueban: una
  suite que corre rápido sobre un esquema distinto del de producción es peor que una lenta.

Y el desglose de la corrida verde bajo `-race`: `cmd/musubi` 136 s · `internal/mcp` 379 s ·
`internal/memory` 454 s. La CI usa `-timeout 20m` POR PAQUETE, así que el más lento queda con
2,6× de margen.

**A53 · el plazo, no la carrera.** `TestPushDelPorteDeProduccionCruzaEntero` federa 14.000 nodos
(5,2 MB crudos, a propósito por encima de `maxRequestBody`) con plazos de 60 s; bajo el detector
comprimir y serializar eso pasa de 90 s. Se escala el PLAZO a 300 s cuando corre instrumentado,
con build tags en archivos `_test.go` para que la etiqueta no toque el binario de producción.
Achicar el grafo no era opción: su razón de ser es superar el tope de 4 MiB, así que tiene piso.
**Y el diagnóstico viejo estaba incompleto**: la ficha decía «falla aislado», y aislado el paquete
entero pasa en 288 s — lo que lo tumba es la CONTENCIÓN de la corrida completa, con todos los
paquetes instrumentados peleando por CPU. Mismo causante, mismo arreglo, pero el motivo anotado
mandaba a buscar donde no era.

**Un error mío que casi entra, y su prueba.** En el pase manual de los últimos ocho sitios convertí
`servidorSobre`, que **reabre el mismo directorio a propósito** para simular un reinicio del
cerebro. Sembrar ahí pisa la base con la plantilla vacía. Lo revertí, y después reapliqué el error
para confirmar que el peligro era real y no teórico: `TestElCooldownSobreviveUnReinicioDelCerebro`
falla con «el cerebro reiniciado actuó 0 veces: esperaba 1». La sustitución MASIVA era segura por
construcción —cada `t.TempDir()` devuelve un directorio nuevo—; el riesgo estaba sólo en el pase
a mano.

**Y un sabotaje declarado que no existía.** Puse un `wal_checkpoint` explícito diciendo que era
imprescindible; al ejecutar su sabotaje la suite quedó VERDE. Medido en vez de supuesto:

    abierto  →  memory.db = 4.096 bytes,  memory.db-wal = 910.552 bytes
    cerrado  →  memory.db = 434.176 bytes, el -wal ya no existe

`Close()` hace checkpoint solo al cerrar la última conexión. Se sacó la llamada en vez de dejarla
con un comentario que decía lo contrario, y los números medidos ocupan el lugar de la afirmación
falsa. Lo load-bearing no era el checkpoint sino **el ORDEN**, y eso sí tiene sabotaje real:
registrar la plantilla sin cerrar da un archivo de 4 KB con `user_version` en 0, y la prueba lo
dice con esas palabras.


**2026-08-29 (2) · A54 — el agente declara lo que va a tocar, y el despliegue lo verifica.**

Lo que quedaba abierto no era el arreglo de contenedores —ése ya estaba— sino que **nada ataba
una capacidad del agente a las rutas que necesita**. La próxima iba a repetir la forma exacta: la
función no anda, la unidad no dice por qué, y nadie sospecha del archivo que está bien.

- **La declaración** (`cmd/musubi/blindaje.go`): cada necesidad lleva ruta, modo de acceso, si es
  opcional, la directiva exacta de systemd que la concede, y —el campo por el que existe— **el
  SÍNTOMA**. A54 costó dos días porque el fallo no dijo «permiso denegado» en ningún lado: fue un
  `podman ps` con código 1. La ruta se deduce de un strace; el síntoma es lo que alguien tiene
  delante *dos días antes* de pensar en correr un strace.
- **Se declara lo que ESTA máquina va a hacer.** Sin podman ni docker instalados, esas rutas no
  son una necesidad — y es la MISMA señal que decide si el trabajo corre (`hayEnPath`, el criterio
  de `enumerarFuente`) la que decide si su blindaje importa, no dos listas que se desincronicen.
  Una necesidad de más manda a alguien a abrir un permiso que nadie usa.
- **El verificador** (`musubi agent --revisar-blindaje`): prueba cada ruta **tocándola de verdad**.
  No alcanza con `access(W_OK)` ni con mirar permisos — el confinamiento de systemd es un MONTAJE
  de sólo lectura y los bits del inodo siguen diciendo que se puede. El único chequeo que ve un
  `ReadWritePaths` faltante es crear un archivo y borrarlo, que es lo que `podman ps` hace y lo
  que fallaba. No pide token: un diagnóstico que exige credencial es inútil justo cuando hace falta.
- **TRES desenlaces y no dos.** «Falta» y «no se puede» tienen arreglos distintos —un `mkdir`
  contra una línea de systemd— y la primera versión los mezclaba: **reproduje el bug de A54
  mientras lo arreglaba**, con el verificador echándole la culpa al blindaje por una ruta que
  simplemente no existía. Un verificador así manda a alguien a editar una unidad que está bien.
- **Verificado contra la máquina real.** Corrido bajo `ProtectHome=read-only` en `musubi-server`,
  las tres rutas dieron `read-only file system` y el informe nombró **las tres líneas exactas que
  el drop-in ya tiene** — derivadas de forma independiente. Fuera del confinamiento sale todo en
  verde, y eso está dicho en la ayuda: correrlo desde un shell normal no prueba nada.
- **Casi repito el bug adentro del arreglo.** La primera versión resolvía el directorio de
  runtime leyendo `XDG_RUNTIME_DIR`. Medido contra el agente real: el proceso tiene `HOME` y
  `USER` y **NO tiene `XDG_RUNTIME_DIR`** — systemd sólo la exporta en unidades de USUARIO, y el
  agente es una unidad de sistema con `User=`. Podman lo sabe y cae a `/run/user/<uid>`, que es
  por qué encuentra sus locks igual y por qué el drop-in tuvo que conceder esas rutas. Leyendo
  sólo la variable, el verificador declaraba **nada** para `/run` en la única máquina donde
  importa: salía en verde sin mirar las tres rutas que rompieron A42. Ahora busca donde busca
  podman, y tiene su prueba y su sabotaje.
- **La guarda estructural**: `TestElBlindajeDeLaUnidadConcedeLoQueElAgenteDECLARA` compara la
  declaración contra la unidad y sus drop-ins. **De acá en más, una capacidad nueva sin su ruta
  rompe la SUITE, no la producción.** Y la guarda corre en las dos direcciones: una ruta que la
  unidad concede y el agente no pide también falla, porque un permiso que sobrevive a la
  capacidad que lo justificaba es un agujero que nadie va a cerrar — nadie se acuerda de para qué
  estaba.
- **Y no da consejo donde no aplica.** Fuera de Linux el verificador se calla y sale con 0:
  `ReadWritePaths` es una directiva de systemd, y en un Windows —donde corren dos de las tres
  máquinas de esta flota— habría emitido `ReadWritePaths=C:\Users\gio\...`, un consejo con
  forma de respuesta que no aplica a nada. Dar una instrucción equivocada con confianza es el
  modo de falla que A54 documenta; hacerlo adentro del arreglo sería el colmo.
- **La unidad base entró al repo.** `deploy/systemd/musubi-agente.service` sólo existía en el
  servidor: el blindaje del que el agente depende no estaba versionado, así que la guarda no tenía
  contra qué comparar. Se trajo del servidor (esa dirección es la segura) tras revisar que no
  lleva un solo secreto — la unidad nombra la RUTA del archivo del token, nunca su valor.
- **La frase que era mentira, corregida donde vive.** La unidad decía «El agente sólo LEE /proc
  y habla con loopback. Nada de esto le hace falta.» Fue cierta hasta A42 y después no, sin que
  nadie volviera a leerla — y es parte de por qué la unidad nunca fue sospechosa: describía con
  seguridad un agente que ya no existía. La copia del repo ahora cuenta lo que pasó y remite al
  verificador. **La unidad instalada conserva el texto viejo**: el drop-in ya cubre lo funcional
  y editar una unidad viva por un comentario no vale el riesgo — queda como cambio cosmético para
  el próximo despliegue del agente.
- Y el error de enumeración ahora nombra al sospechoso: un `podman ps` que falla manda a
  `musubi agent --revisar-blindaje` en vez de dejar el mensaje que costó dos días.

**Doce sabotajes ejecutados.** Uno no rompió nada la primera vez y **no fue debilidad de la
prueba sino mía**: gofmt había alineado la clave del struct y mi reemplazo no coincidió, así que
el sabotaje nunca llegó a aplicarse. Aplicado bien, falla.


**2026-08-29 · A38, A39, A49, A50, A51 y A52 — seis cabos de la misma familia.**
Todos eran lo mismo: código en producción cuyo modo de falla NO se veía desde
ninguna prueba, y en dos de los tres casos tampoco desde el log.

- **A50 · el empuje se quedaba mudo de tres maneras y sólo decía una.** El arranque exige la
  concesión `metrics` y se niega a arrancar sin ella, pero `principals.yaml` se recarga en
  caliente cada 10 s: la concesión se puede ir DESPUÉS de un arranque válido. Ahora
  `empujarUnaVez` la vuelve a mirar en cada tick y avisa —una vez, rearmándose al recuperarse—
  distinguiendo los tres modos: el principal borrado, el principal sin sección `fleet:`, y la
  concesión que apunta a proyectos sin máquinas. **Ninguno cuenta un fallo, y eso es la decisión**:
  `musubi_push_failures_total` significa «no llegó a destino» y en dos de los tres ni se intentó;
  ensuciarlo rompería `MusubiPushOTLPNuncaLlego`, que separa «se cayó» de «nunca anduvo».
  De paso apareció algo peor y más viejo: **`musubi_push_datapoints` no podía valer 0 nunca**.
  Su propio HELP dice «un 0 sostenido = el empujador corre y no exporta nada», y todos los
  caminos de salida temprana se salteaban el `Store` — el gauge publicaba el último conteo bueno
  para siempre. Un tablero que lo mirara veía el empuje sano mientras estaba muerto.
  El runbook ahora tabula los tres modos con el `grep` que los distingue.
- **A38 · la columna de procesos, y la RAM libre, en el panel.** Los dos campos llegan a
  `musubi_fleet_metrics` y a Prometheus desde U1 y **el panel no los dibujaba** — el mismo patrón
  que `agent_version`: dato guardado que ninguna interfaz muestra. Entran al cajón de vitales.
  La RAM libre pasa por un `bytes()` nuevo: sin formatear se lee `1073741824` y nadie la mira. Y
  `bytes()` distingue las dos cosas que este track no deja confundir — **null se dibuja «sin
  dato»** (Windows y macOS no exponen MemFree, y «0 B libres» se lee como una máquina a punto de
  morir) **y el 0 MEDIDO se dibuja 0**, porque un disco lleno tiene cero disponibles de verdad y
  ése es justo el número por el que suena la alarma. Las dos mitades tienen su sabotaje.
- **A39 · el cero mentiroso era más grande que el cabo.** Estaba anotado como «un diff de tres
  líneas» sobre `num_cpu`. Al ir a hacerlo apareció que **`uptime_seg` tenía el mismo problema** y
  es peor: un `num_cpu: 0` se lee como imposible y alguien sospecha, pero un `uptime_seg: 0` se lee
  como «arrancó recién» —plausible— y manda a investigar un reinicio que no pasó. Y los pares de
  memoria y disco también salían crudos.
  **El exportador de Prometheus ya lo tenía bien** —las series se emiten con `> 0` o con
  `Total > 0` como condición— y la tool no. Dos superficies que leen la MISMA muestra y no
  coinciden en qué es «no medido» es peor que una sola equivocada: el gráfico muestra un hueco, la
  tabla muestra un 0, y no hay forma de saber cuál miente.
  **Los pares se compuertan por su TOTAL, no por su propio valor**, y ésa es la parte que
  `enteroONull` no puede hacer: `disco_libre: 0` en un disco LLENO es un cero MEDIDO y verdadero —
  el peor momento posible para convertirlo en «no sé», porque es exactamente el número por el que
  suena la alarma. Lo custodia un caso de prueba propio.
  Lo cierra `TestLaTablaYElExportadorCoincidenEnQueEsNoMedido`, que **ata las catorce series del
  exportador a sus catorce campos de la fila** y falla si una de las dos superficies cambia de
  opinión sin la otra. Una serie renombrada rompe la prueba en vez de dejar el par colgado.
  De paso, `enteroONull` pasó a ser genérica: los tres campos que la necesitan no comparten tipo
  (`int` y `uint64`), y dos funciones con el mismo cuerpo es exactamente cómo una de las dos se
  queda sin el arreglo la próxima vez.
- **A49 · nada miraba el sobre.** `receptorDePrueba` leía el cuerpo y contaba requests: ni el
  método, ni el path, ni un header. `ultimoCuerpo` estaba **definida y sin llamar**. Se le agregó
  la captura del sobre y dos pruebas: el POST de `application/json` al path configurado con el
  cuerpo OTLP bien formado, y el bearer viajando en el header y nunca en la URL.
- **A51 · `incluir_revocados` prometía en su propia descripción algo que no podía dar.** Los
  servicios de una máquina revocada no salían NUNCA: el kill-switch de la revocación tumbaba el
  device antes de mirar la concesión, y las filas —que la migración 36 y `RevocarServiciosDeDevice`
  conservan A PROPÓSITO para la auditoría— no tenían por dónde verse. **Se arregló el
  comportamiento, no la promesa**: una auditoría que nadie puede leer no es una auditoría, y
  «datos guardados que ninguna interfaz muestra» es el patrón que este track viene persiguiendo.
  El arreglo es `PuedeVerHistorialDeDevice`, que limpia el flag y **delega** en `PuedeSobreDevice`
  en vez de repetir la cadena: misma tenencia, misma concesión, mismo tier, y una regla nueva
  aplica sola en los dos lados. **No es «admin y listo»** — eso derivaría una capacidad de flota
  del rol, que es justo lo que C1 prohíbe. Y la revocación sigue siendo absoluta para todo lo que
  TOQUE la máquina: exec, pantalla y shell pasan por `PuedeSobreDevice` y ahí el kill-switch manda.
  La fila trae `device_revocado` aparte de `revocado`: son dos bajas distintas y confundirlas hace
  leer «esto se dejó de usar» donde lo que pasó es «esta máquina salió de la flota con todo adentro».
- **A52 · la fila del sondeo de Tier B.** El agujero no era una aserción que faltaba sino un
  **fixture que no llegaba a los campos**: `lecturaProcFalsa` tiene siete secciones de ocho y un
  meminfo sin `MemFree`, así que producía `MemLibre = nil` y `NumProcesos = 0` — borrar
  `fila["mem_libre"]` del código no cambiaba nada observable. Con `lecturaProcCompleta` los dos
  sabotajes del verificador disparan, más un tercero: cruzar `MemFree` con `MemAvailable`, que en
  una máquina real son 3,5 GB de diferencia y dos números igual de plausibles.

**Costura nueva en `logx`.** El aviso de A50 no cambia ningún contador a propósito, así que su
único efecto observable es la línea de log — y una prueba que mirara sólo la contabilidad interna
del «avisar una vez» quedaría en verde con la línea borrada. `logx.Capturar` existe para eso. El
logger pasó a vivir en un `atomic.Pointer`: el empuje y el barrido loguean desde sus propias
goroutines, y una variable pelada sería una carrera de manual justo en las pruebas que existen
para atrapar fallas silenciosas.

**Veintiún sabotajes ejecutados**, y dos de ellos **no rompieron nada la primera vez**. El de A50: la aserción buscaba
«concesión \`metrics\`», que también aparece en el mensaje del caso de al lado, así que la prueba
pasaba por el motivo equivocado. Se afiló a la frase que sólo dice ese caso. El de A51 —saltear
la tenencia al auditar— lo atrapaba la guarda del TIER y no la de tenencia: el caso de prueba
pasaba por la tool, donde el barrido por proyecto filtra al vecino ANTES de llegar a la compuerta.
Se agregó la aserción directa contra la función, con su control positivo al lado.


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

**2026-08-28 · el cerebro redesplegado y el empuje ANDANDO en producción:**

- **A43 — los servicios llegan a Prometheus, y hay quien avise cuando uno se cae.** Tres series
  por servicio, por los DOS caminos de salida (scrape y empuje), compartiendo la tabla y el juego
  de labels con las de máquina — dos copias discrepan el día que alguien agrega un campo, y eso
  se descubre semanas después cuando dos dashboards muestran cosas distintas. **La compuerta no
  se evalúa dos veces**: los servicios se buscan sólo para las máquinas que ya pasaron
  `PuedeSobreDevice`, porque un segundo recorrido es un segundo lugar donde olvidarla.
  Tres alertas nuevas, separadas por lo que hay que HACER y no por el síntoma: `ServicioCaido`
  —que se **inhibe** si la máquina entera está caída, o una caída se vuelve cincuenta mensajes—,
  `ServicioReiniciandose` (anda a los tumbos: `up` no lo puede mostrar porque en cada instante
  está arriba) y `ServicioSinNoticias` (no sabemos cómo está, y no saber no es estar bien).
  Verificado contra la base de producción: **144 series de servicio y 45 de máquina**, con las
  seis etiquetas estables y **sin el pid** — una etiqueta que rota deja una serie muerta por cada
  reinicio, que es la forma más común de matar un Prometheus y no da un solo error mientras pasa.

- **A42 — el agente enumera sus propios servicios.** systemd (una sola llamada a
  `systemctl show '*.service'`, no una por unit), contenedores de podman y docker, servicios de
  Windows por el SCM, y launchd en macOS. **Verificado en producción: 54 servicios, 36 units y
  18 contenedores, sin declarar uno solo a mano.** Se reporta lo que ALGUIEN DECIDIÓ que corra
  (habilitado) más lo que está roto: una unit deshabilitada e inactiva es ruido y hay cientos.
  El orden es determinista y prioriza lo fallado, porque el cerebro poda por ausencia y un
  recorte inestable daría de baja y de alta los mismos servicios cada latido.
  **Y el inventario NO viaja en cada latido**: viaja cuando cambió, más un piso de 5 minutos.
  Colgarlo de todos rompió una guarda que ya existía —«un latido sin muestra es de decenas de
  bytes»— mandando 7.180; con el intervalo más corto de la flota eso son 7 KB cada diez segundos
  por máquina. La guarda quedó MÁS estricta que antes: ahora también custodia que no se repita.
  **Un defecto que sólo se vio desplegando**: `podman` no estaba en el enum de clases, así que
  el cerebro les vaciaba la columna en silencio — 18 filas correctas indistinguibles de las que
  de verdad no saben quién las corre. Se agregó, con `launchd` y `kubernetes`, y una prueba que
  ata el enum a lo que los enumeradores emiten.

- **El redespliegue**, con el script versionado y su ensayo previo contra una copia de la base
  real: 35 → 37 en 3,6 s, las tres máquinas y sus `rustdesk_id` intactos, y las dos tools nuevas
  vivas (13 de flota en total).
- **El empuje OTLP corriendo**: `musubi_push_last_success_seconds 3`, `failures_total 0`,
  `datapoints 45`. Las series aterrizan con su etiqueta `project`.
- **Y el problema que encender el empuje CREÓ, medido y cerrado el mismo día.** El scrape y el
  empuje traían los dos la telemetría de flota con distinto `instance`: no se pisaban, así que
  no se veía como un error — se veía como que todo andaba. Pero **cada regla de flota matcheaba
  dos series y cada alerta salía duplicada**: 5 alertas se volvieron 10 avisos. Se resolvió con
  la tesis del módulo, no con un parche: el scrape descarta `musubi_fleet_*` y se queda con lo
  del cerebro (98 → 53 series), el empuje se queda con la flota (45). **Un solo productor por
  dato.** Y las dos mitades quedaron atadas por una prueba, porque el `drop` sólo es correcto
  mientras exista el empuje, y el empuje sólo es no-duplicación mientras exista el `drop`.

**2026-08-28 · lo que sólo se podía cerrar tocando producción:**

- **A40 — el empuje OTLP, probado contra un Prometheus de verdad.** Se encendió
  `--web.enable-otlp-receiver` en `musubi-server` (el POST pasó de 404 a 200, las 19 reglas
  siguieron cargadas) y se agregó `fleet_otlp_real_test.go`: una prueba **opt-in** que empuja un
  payload real y después **lo consulta de vuelta**. Aceptó 9 puntos y devolvió
  `musubi_fleet_device_up{device="prueba-real-…"} = 1` con su etiqueta `project`.
  **Y midió los dos sabotajes, que corrigieron lo que yo creía:** mandar `timeUnixNano` como
  número **no** rompe contra Prometheus (lo acepta; eso lo atajan las pruebas con receptor de
  mentira), y ponerle una `unit` a la serie **pasa las 24 pruebas de mentira en verde**,
  devuelve 200, y deja la serie renombrada e inencontrable. Ése es el único sabotaje que sólo
  ve un Prometheus real, y es el que justifica la prueba.
- **A46 — el empujador ya se vigila.** Tres reglas en `musubi-alerts.yml`, con su sección de
  runbook cada una, y **las tres se apagan solas cuando el empuje está apagado** — que es lo único
  que permite instalarlas SIEMPRE sin enseñarle a nadie a ignorar el canal: `failures_total` vale
  0 con el empuje off, y `last_success_seconds` **no existe** hasta el primer empuje aceptado.
  Se separan por ARREGLO, no por síntoma: `Fallando` (el destino rechaza), `Mudo` (anduvo y paró
  sin contar fallos — la firma de A50), y `NuncaLlegó` (hay fallos y ni un éxito: falta el flag o
  el path está mal). Las tres expresiones se validaron contra el Prometheus real, no sólo contra
  el parser.
- **A48 — `go test -race` dejó de estar rojo.** El contador de `lectorQueCuenta` lo escribía el
  `writeLoop` de net/http después de que `Do()` volvía. Pasó a `atomic.Int64` con un único
  camino de lectura. Verificado: 3 de 3 corridas con carrera antes, 3 de 3 limpias después.
  **No lo puso rojo el trabajo de unificación** — se reprodujo en un worktree limpio de HEAD,
  3 de 3. La CI venía roja y no se veía porque `go test ./...` sin `-race` pasa entero.

**2026-08-27 · el plano de pantalla, desplegado y arreglado:**

- **La ruta de RustDesk** (`cmd/musubi/rustdesk_ruta.go`). El agente buscaba `rustdesk` sólo en el
  PATH; Windows no lo pone ahí. Dos máquinas con RustDesk **instalado y corriendo** figuraban sin
  `rustdesk_id`, y eso se lee como «máquina sin pantalla configurada»: la ausencia de plano visual
  se veía **idéntica** con RustDesk presente o ausente. Ahora se busca donde lo deja cada
  instalador oficial, `MUSUBI_RUSTDESK_BIN` fuerza la ruta y **falla si apunta a nada**, y el
  agente avisa una vez por motivo distinguiendo «no está» de «está y no contesta». Verificado en
  producción: `gio` → `132570932`, `kernelos-pc` → `1740888405`.
- **El relay propio, corriendo** (`deploy/rustdesk/compose.yml` + `preparar.sh`). Podman rootless
  en `musubi-server`, atado al tailnet, los cuatro puertos verificados. Que los dos caminos de
  instalación no se vayan a la deriva lo custodia `despliegue_relay_test.go`.
  **Falta que los clientes lo usen: eso es A35, y es acción del operador a propósito.**


**2026-08-28 · el inventario era un trinquete: sólo sabía achicarse.**

Encontrado **verificando A43 en producción**, no por una prueba. Prometheus exportaba 36 series de
servicio de `musubi-server` y la máquina corre **54**: los 18 contenedores estaban en la base
**revocados y con la clase en blanco**, y el agente los venía reportando bien en cada latido desde
hacía horas. Dos defectos encadenados, ninguno con error en ningún lado:

- **El agente mandaba inventarios incompletos** (`cmd/musubi/servicios.go`,
  `servicios_linux.go`). Cualquier falla de una fuente era un `continue`, con este razonamiento
  escrito: «perder el inventario entero porque una fuente falló sería cambiar información parcial
  por ninguna». Es falso — **el cerebro poda por ausencia**, así que la lista no dice «encontré
  esto», dice «esto es lo que corre acá». Un `podman ps` que falla una vez no manda menos
  información: manda la afirmación de que esos 18 contenedores dejaron de existir. Ahora
  `enumerarFuente` separa los tres desenlaces —no está / está y falló / anduvo— y una fuente rota
  **aborta el inventario del latido**. No mandarlo no borra nada, y no es silencioso: los
  servicios se ponen `fresco: false` y salta `ServicioSinNoticias`.
- **El cerebro no deshacía la poda** (`internal/memory/servicios.go`). El UPDATE del reporte
  llevaba `AND revoked = 0` y el INSERT chocaba con el índice único y se descartaba: la fila
  revocada no volvía **nunca**, aunque la máquina la reportara para siempre. Podar por ausencia y
  no despodar por presencia es una asimetría, no una precaución. Ahora un reporte resucita lo que
  la poda se llevó (`declared = 0`) y **sólo eso**: lo que dio de alta una persona sigue volviendo
  por `fleet_service_declare`, que es alguien decidiéndolo. El comentario original —«que vuelva a
  aparecer tiene que ser una decisión»— era correcto; el error fue aplicárselo también a la mitad
  que nadie decidió.

Las cuatro pruebas nuevas tienen su sabotaje **ejecutado**, y el par del cerebro está escrito como
par a propósito: la forma más cómoda de hacer pasar la resurrección —sacar el WHERE y listo— la
caza la otra mitad.

**2026-08-28 (bis) · el blindaje del agente prohibía el trabajo que A42 le dio.**

La causa raíz de lo de arriba, encontrada porque el arreglo la dijo en voz alta al primer arranque:
`podman está instalado y no se pudo consultar: exit status 1`. El agente **nunca** pudo enumerar
contenedores en `musubi-server` — los 18 estaban en la base de una carga anterior, y el primer
inventario del agente los podó.

`musubi-agente.service` declara `ProtectHome=read-only` y `ProtectSystem=strict`, con este
comentario: «El agente sólo LEE /proc y habla con loopback. Nada de esto le hace falta». Era cierto
cuando se escribió. **A42 le dio un trabajo nuevo y nadie volvió a mirar el blindaje** — y
`podman ps` no es una lectura: medido con strace, abre `db.sql` y seis locks en modo escritura
bajo el home, más dos en `/run/user/1000`.

Se resuelve con un drop-in versionado (`deploy/systemd/musubi-agente-contenedores.conf`) que abre
**esas rutas y nada más**. Se evaluó el socket de la API de podman, que necesitaría una excepción
más chica en la unidad: se descartó porque **concede lo mismo o más** —crear un contenedor con un
bind-mount del host— así que cambiar de puerta compra código, no seguridad. Lo que la excepción
concede, dicho sin adorno: el agente puede **manejar** podman, no sólo listarlo. El techo sigue
siendo el usuario `musubi`, que ya era dueño del store; lo que se pierde es el confinamiento de
montaje que lo separaba de él.

**El cabo que queda vivo, y no es este drop-in:** cada capacidad nueva del agente puede chocar con
su propio blindaje, y el choque se ve como «la función no anda» y no como «la unidad la prohíbe».
Pasó una vez y va a volver a pasar. Registrado como **A54**.

**2026-08-28 (ter) · Tier B deja de querer decir «SSH»: el transporte de exposición.**

Primer trozo del paso 4 de la unificación. `Tier B` siempre dijo «sin binario en el device, por su
protocolo nativo», y hasta acá el único protocolo era SSH — lo que dejaba afuera una clase entera
de máquinas que Musubi tiene que poder mirar: **las que no dan shell y sí publican sus vitales**.
Una base gestionada en la nube es el caso exacto, y es la que `collect-supabase.sh` viene mirando
desde afuera del sistema.

- **El parseo** (`internal/fleet/exposicion.go`) lee el formato de exposición de Prometheus sin
  traer una dependencia: son ocho familias de métricas, y el parser oficial completo —con sus
  exemplars y sus histogramas nativos— sería pagar todo para usar una esquina. El fixture es un
  recorte **literal** del endpoint real, con la referencia del proyecto redactada. Eso importó: un
  fixture inventado habría traído `node_boot_time_seconds`, y su AUSENCIA es justamente el caso
  que más cuidado necesita.
- **El viaje** no sigue redirecciones (un 302 hacia otro host es un SSRF con nuestra credencial),
  acota el cuerpo con un byte de margen (leer justo el techo y parsear lo que entró daría una
  Muestra armada con texto truncado), y **nunca deja la credencial en un error** — el error de
  `net/http` lleva la URL entera adentro y esa URL puede traer un token.
- **La configuración** (`.musubi/flota-exposicion.yaml`) guarda el NOMBRE de la variable de
  entorno con la credencial, nunca la credencial. Y **rechaza** una URL con usuario y clave
  adentro: un secreto que ya entró a un archivo versionado no se puede des-filtrar.

**Dos hallazgos de ir a mirar, que ninguna prueba de escritorio da:**

1. **La compuerta del parser y `Muestra.Valida` se contradecían.** La compuerta pedía sólo
   `MemTotal`; la regla de los pares del dominio exige el total CON su usado. Un endpoint con el
   total y sin el disponible pasaba la compuerta y lo rechazaban después con «la muestra no es
   creíble» — cierto, y mandando a mirar el lugar equivocado. Dos guardas sobre lo mismo que no se
   enteran una de la otra terminan discutiendo en el mensaje de error.
2. **El endpoint real CACHEA su respuesta.** Medido: el contador de CPU no se mueve en 45 s. El
   porcentaje es una derivada, así que dos sondeos dentro de esa ventana no tienen contra qué
   restar y sale **null**. Correcto — y significa que el intervalo de sondeo tiene que superar el
   caché. El colector que esto reemplaza reportaba **0 %** en ese caso, o sea una base ociosa,
   dibujada con confianza.

Dieciséis sabotajes, todos ejecutados. Uno de ellos —el de la credencial en el error— **no falló
la primera vez** porque el parche no había matcheado; se rehízo y ahí sí cayó, filtrando el token
en el mensaje tal como se predecía.

**Lo que este trozo NO cierra, y por eso `collect-supabase.sh` sigue vivo:** el endpoint publica
además `pg_database_size_bytes` y las conexiones del pooler, que **no son vitales de host** y no
entran en `fleet.Muestra`. Una de las tres alertas que hoy existen mira las conexiones del pooler.
Registrado como **A55** — y cerrado el mismo día, más abajo, con una salida que no era ninguna de las dos que esta nota anticipaba.

**2026-08-28 (quater) · A55 cerrado, y NO como decía la nota: el plano de aplicación es de
Prometheus.**

A55 quedó anotado con dos salidas —extender el dominio, o modelar los números como salud de un
servicio— y al ir a hacerlo, las dos eran la equivocada. La tool de Musubi dice, y sostiene,
que **guarda el PRESENTE y que la serie temporal la guarda Prometheus**. Construir adentro de
Musubi un mecanismo para cargar gauges arbitrarias contradice ese límite escrito, y lo contradice
justo para reimplementar lo que Prometheus ya hace mejor.

Así que: **Musubi mide la máquina, Prometheus mide la aplicación**, sobre el mismo host y sin
pisarse. No es duplicación —son datos distintos— y la regla del track sigue en pie porque el
scrape nuevo TIRA todo `node_*`: de los vitales del host el productor es Musubi. Sin ese descarte
habría dos series de memoria para la misma máquina y las alertas saldrían dobles, exactamente
como pasó al encender el empuje OTLP.

- `deploy/prometheus/scrapes/altura-db.yml.ejemplo` — el scrape, cargado por un glob nuevo
  (`scrape_config_files`) con la misma lógica que las reglas: la referencia real del proyecto no
  viaja en el repo. Verificado contra promtool 3.1.0, incluido el detalle de que el archivo de
  sitio necesita la clave `scrape_configs:` adentro y una lista pelada se rechaza.
- `deploy/musubi-alerts-altura.yml` — cuatro reglas, validadas contra el Prometheus real.

**El hallazgo: una de las tres alertas de producción no podía sonar.** El alerter tenía
`("pooler_conns", 350, "Conexiones del pooler (de 400)")` sobre
`sum(pgbouncer_pools_server_used_connections)`. Dos números tipeados a mano y de cosas distintas:
el 400 es `pgbouncer_config_max_client_connections` —el límite del lado **cliente**— y lo que
sumaba son las conexiones del lado **servidor**, que es otro pool y mucho más chico (medido:
`free_servers` 50, `used_servers` 0, las tres sumas en 0). Vigilaba un número que tendría que
multiplicarse por siete para tocar su umbral. **Nunca sonó, y «nunca sonó» se lee igual que
«todo bien».** La regla nueva divide por la métrica que el propio pooler publica: si Supabase
cambia el plan, se ajusta sola.

Veintitrés sabotajes en total en este trozo y el anterior. **Dos de las pruebas de este archivo
estaban mal escritas y el sabotaje las dejó en verde:** una tenía un lazo que hacía `return` al
encontrar justo lo que decía prohibir, y la otra buscaba la métrica del denominador en el TEXTO
ENTERO — y la encontraba en el comentario que explica el error. Las dos se reescribieron para
mirar la regla y no el archivo, y ahí sí cayeron.

**2026-08-28 (quinquies) · la auditoría de las 25 reglas: dos ciegas más, y el dato que ya estaba.**

Después de encontrar que la alerta del pooler no podía sonar, se auditaron **las 25 reglas cargadas
en el Prometheus real**, una por una: se extrajeron los nombres de métrica de cada expresión, se
consultó cuántas series tiene cada uno, y para las de flota se evaluó el LADO IZQUIERDO del umbral
contra los valores de hoy. No es una revisión de escritorio: es contra los datos que hay.

Salieron limpias 22 de 25. Las tres que no:

- **`ServicioReiniciandose` estaba ciega para 18 de 54 servicios.** 54 series de
  `musubi_fleet_service_up` y sólo 36 de `musubi_fleet_service_restarts_total`; los 18 que
  faltaban eran los contenedores. Un contenedor con `restart: always` en bucle de caída es EL
  caso para el que existe esa alerta. `podman ps` sí sabe decirlo (`{{.Restarts}}`) y el agente
  no lo pedía. **Arreglado** — con degradación de formato, porque `docker ps` no conoce ese campo
  y con la regla nueva («una fuente que está y falla aborta el inventario») pedirlo a secas
  convertiría «este docker no entiende un campo» en «esta máquina no reporta nada».
- **`PoliticaQueNoCura` y `PoliticaSinPermiso` no tienen series**, porque no hay ninguna política
  configurada. Verificado, no supuesto. Es el caso «una regla cuya precondición no se cumple» que
  este archivo ya discute; son `increase(...) > N`, así que no disparan en falso. Se dejan.

  > **ESTA EXPLICACIÓN ERA CIERTA Y TAPABA LA REAL (2026-08-31).** Se configuró la primera
  > política de verdad, disparó —`acciones=1` en el barrido, con su línea «política dispara» en
  > el journal— y la serie **seguía sin existir**. El `metric_relabel_configs` del job `musubi`
  > descartaba `musubi_fleet_.*` entero, y `musubi_fleet_policy_actions_total` es la ÚNICA de esa
  > familia que el empuje OTLP **no** lleva: sale sólo del scrape. Así que las dos alertas no
  > podían dispararse **nunca**, ni con políticas configuradas y actuando.
  >
  > Dos causas apiladas, y la de arriba alcanzaba para cerrar la pregunta. Sólo se vio porque se
  > hicieron las DOS mitades —configurar una política *y* verla disparar—; con una sola, la
  > conclusión hubiera sido «ahora sí anda» o «sigue sin haber políticas».
  >
  > Y el dato que más incomoda: el comentario del propio `prometheus.yml` documentaba la regla
  > correcta (`musubi_fleet_device_.*`) mientras el código decía otra cosa. La documentación
  > tenía razón hacía meses y nadie las comparó. **Arreglado**: el drop nombra las dos familias
  > empujadas (`device_` y `service_`), y `TestElScrapeYElEmpujeNoTraenLoMismo` ahora falla
  > también si alguien lo ensancha de vuelta — sabotaje ejecutado, no supuesto.

**Y el hallazgo que explica todo lo demás: `agent_version` se guardaba y no se mostraba.**

`kernelos-pc` figuraba **en línea, latiendo cada 30 s, con CERO servicios**. Eso tiene dos causas
opuestas —binario anterior a la enumeración, o enumerador roto— y no había forma de distinguirlas.
El dato para hacerlo estaba en la base desde el principio: el agente manda su versión en cada
latido y `LatirDevice` la escribe. Ninguna tool la mostraba. Una columna llena que no se podía
leer.

Al sacarla a la luz, la respuesta fue inmediata:

    gio            v0.106.0-28-gdf2ec21-rustdesk
    kernelos-pc    v0.106.0-28-gdf2ec21-rustdesk
    musubi-server  0.111.1-trinquete.a3053e3

Las dos Windows corren un binario de **tres commits antes de A42**. No tienen enumeración de
servicios: no está rota, no existe. **El inventario de servicios cubre hoy una de tres máquinas**,
y eso no se veía en ningún lado.

Se agregó `MaquinaSinInventario`: un Tier A que late y no dice qué corre adentro. Cubre las tres
causas del mismo síntoma —binario viejo, enumerador roto, y la fuente que abortó el inventario a
propósito— porque las tres piden lo mismo: que alguien mire. Validada contra los datos reales:
dispara para `kernelos-pc`, no para `musubi-server` (que sí reporta) ni para `gio` (que está
caída, y ésa la cubre `MaquinaCaida`).

**Sobre `gio`, que llevaba dos días anotada como «apagada»: la causa es de diseño y estaba escrita.**

Responde al ping por el tailnet con 145 ms: la máquina está encendida y el agente no está
corriendo. El porqué estaba en el propio instalador: `agente-windows.ps1` registra la tarea con
**`-AtLogOn`**. El agente vive mientras haya alguien logueado, así que un equipo que se reinició
de madrugada y quedó en la pantalla de bloqueo **figura caído estando vivo**.

La elección estaba justificada en un comentario del script («un servicio de Windows exige
elevacion y un envoltorio») — lo que faltaba era **la consecuencia**, y sin ella el síntoma se
leyó como «la máquina está apagada» durante dos días.

Ahora el instalador tiene `-AlArranque`: registra al arranque y como SYSTEM, sin depender de que
nadie inicie sesión. Es **opt-in y no el default a propósito**, porque correr el agente como
SYSTEM cambia lo que la flota puede hacer en esa máquina — `musubi_fleet_exec` pasaría a
ejecutarse con privilegios de SYSTEM. Eso es una decisión de seguridad y la toma el operador, no
el despliegue. El camino por defecto ahora **avisa en pantalla** qué pasa si la máquina se
reinicia, y una prueba custodia las dos mitades: que el default no escale, y que su costo esté
dicho.

**2026-08-28 (sexies) · la limpieza de la tabla, y el vigilante que nadie vigilaba.**

Al preguntar «qué falta», lo primero fue verificar la tabla en vez de recitarla. Tres entradas
estaban vencidas y se borran, con lo que se midió:

- **A29** «la cadena de alertas nunca se desplegó» → Alertmanager 0.28.0, 23 h de uptime, dos
  receptores (`default`, `watchdog`), tres alertas en vuelo. Desplegada.
- **A34** «el servidor corre un binario 8 commits atrás» y **A47** «el redespliegue es una puerta
  de una sola dirección» → el cerebro corre `0.111.1`. Cruzada, y con vuelta atrás que también
  deshacía el esquema.

Y **A35 sigue viva y confirmada**: el relay arranca (`Start`, `relay-servers=…`) y su log no
registra un solo cliente. (El `Failed to store config: Bad configuration directory` que aparece
dos veces al arrancar **es ruido**: la clave `id_ed25519` está en el volumen desde el 27 y
sobrevivió al reinicio del 28. Se verificó antes de asustarse.)

**Lo que apareció mirando: el dead-man's switch no estaba «sin armar», estaba FALLANDO.**

`MusubiSiempreViva` sale cada 5 minutos hacia el receptor `watchdog`, y ese receptor apunta a
`url_file: /etc/musubi/watchdog_url` — un archivo que **no existe**. Medido:

    alertmanager_notifications_failed_total{integration="webhook"}  279
    alertmanager_notifications_total{integration="webhook"}         310
    alertmanager_notifications_total{integration="telegram"}         31   (0 fallos)

**387 errores desde el 2026-08-27 13:54**, o sea 32 horas, cada 5 minutos, mientras el MISMO
Alertmanager entregaba por Telegram sin un solo fallo. Un canal roto al lado de uno sano.

Y nada lo contaba: Prometheus scrapeaba **dos** targets —el cerebro y a sí mismo— y Alertmanager
no era ninguno. El error vivía en el log de un contenedor, que es donde las cosas van a no ser
vistas. Se agrega el job y la regla `CadenaDeAlertasFallando`, agrupada por `integration` y no por
`reason` para que un canal que rota entre motivos no dé varias alertas de lo mismo.

**Lo que esa regla NO puede hacer, dicho:** si la cadena entera muere, la alerta que avisa de eso
tampoco sale. Eso no lo arregla ningún scrape — lo arregla el dead-man's switch externo, que es
justamente lo que estaba roto. La regla cubre el caso real y frecuente: un receptor caído mientras
otro anda. Crear `watchdog_url` sigue siendo **acción del operador** (A22), y ahora el runbook
tiene los tres comandos exactos.

**2026-08-28 (septies) · el visualizador existía y estaba tapiado. El síntoma culpaba al cerebro.**

Gio dijo, con razón, que del módulo de monitoreo no había visto nada. Al ir a mirar por la puerta
por la que él mira —y no por la de atrás, que es la que yo venía usando— el panel de flota
contestaba esto:

    {"estado":"caido","detalle":"no se pudo determinar de qué proyecto listar la flota"}

**La página `/flota` estaba construida desde S9** —tabla de máquinas, sección de servicios,
botones de pantalla y exec— y su API no podía traer un solo dato.

La causa: el principal del panel es `panel-central`, `read: all` con `project_id: ""` **vacío a
propósito**, porque no pertenece a ningún tenant. Y `fleetReadScopeFor` cae al `ProjectID` del
principal cuando no se declara un proyecto. Vacío → error. **Las tres tools de lectura fallaban
igual**: `fleet_list`, `fleet_metrics` y `fleet_services`.

Y el síntoma MENTÍA, que es lo peor de todo: el panel dibujaba `estado: caido`, que se lee como
«el cerebro se murió», con el cerebro latiendo y exportando 233 series. Un panel que culpa al
backend por su propio problema de alcance manda a depurar el lugar equivocado.

**La salida no fue aflojar el WHERE por proyecto** —eso sería un «listar todo» y se llevaría
puesto el aislamiento entre tenants—: es enumerar los proyectos y consultar cada uno POR SEPARADO,
que es lo que el export federado a Prometheus ya hacía desde S11. **La maquinaria estaba
(`ProyectosConDevices`); las tools no la usaban.** Y cada fila ahora dice de qué proyecto es: con
`read: all` la tabla mezcla clientes, y una fila que no lo dice invita a actuar sobre la máquina
de otro.

Cuidado especial en `fleet_services`: filtrar por nombre de máquina no puede volverse un oráculo
de qué máquinas existen en el proyecto ajeno. Con el lazo, «no está en este proyecto» se saltea en
silencio y la forma de la respuesta no cambia.

**Verificado contra la base real**, con un cerebro y un panel temporales sobre una copia:

    estado: vivo
    gio            cpu=2.41%  disco=70.3%  servicios=None
    kernelos-pc    cpu=6.53%  disco=85.3%  servicios=None
    musubi-server  cpu=3.52%  disco=49.5%  servicios=54

**Queda una mitad que es del operador, y es config, no código:** `panel-central` no tiene sección
`fleet:` en `principals.yaml` —el principal `prometheus`, justo debajo, sí la tiene—, así que ve
QUÉ máquinas hay y no CÓMO están: `sin_permiso: 3` en métricas y `sin_permiso: 54` en servicios.
Es la compuerta funcionando como fue diseñada; nadie se la concedió. Alcanza con
`fleet: {metrics: ["*"]}` y nada más — un panel mira. Registrado como **A56**.

**La lección de método, que es la que importa:** durante días se reportó un sistema sano mirando
las tools, Prometheus y las series, y **nunca se abrió la pantalla por la que mira el operador**.
Todo lo que se verificó era cierto y ninguna de esas verificaciones tocaba el camino que él usa.

**2026-08-28 (octies) · fase 1 de la maqueta: el modelo de autoridad empieza por el dominio.**

Después de investigar seis proyectos de referencia (MeshCentral, Teleport, Guacamole, Fleet/osquery,
Netdata/Zabbix, Tactical RMM) y escribir la maqueta de tres planos, arranca la fase 1 por el
dominio, que es lo que no se puede agregar después.

**`screen` partido en dos.** Era un solo bit, así que dárselo a alguien para que DIAGNOSTICARA le
daba también el teclado y el mouse; la alternativa era no dárselo, y entonces no podía ayudar.
Ahora hay `screen:view` y `screen`, con una implicación **asimétrica**: quien controla puede mirar
—negárselo sería un absurdo—, quien mira no puede controlar —si pudiera, la capacidad nueva sería
decoración—. Copiado de MeshCentral (`MESHRIGHT_REMOTECONTROL` contra `MESHRIGHT_REMOTEVIEWONLY`).

**Compatibilidad hacia atrás, que no es un detalle:** `screen` sigue significando exactamente lo
que significaba. Redefinirlo como «sólo mirar» habría sacado el control a todos los que hoy lo
tienen, en silencio, hasta que alguien lo necesitara.

**El eje de consentimiento.** Es la ausencia más grave que tenía el modelo: una sesión de pantalla
se abría y la persona sentada enfrente no se enteraba. Cuatro grados ORDENADOS —`libre` < `avisa`
< `pide` < `prohibido`— y de ese orden sale la única regla que importa: **cuando dos fuentes
discrepan, gana la más restrictiva**. No es una cascada donde lo específico pisa a lo general: con
cascada, un `libre` en la fila de UN dispositivo anularía un `pide` puesto en el proyecto entero, y
el agujero se abriría por el lado que menos se audita.

El default es `avisa`, y las dos alternativas fallan por motivos opuestos, los dos escritos en el
código: `libre` deja cada máquina nueva sin protección sin que nadie lo haya decidido; `pide` traba
sesiones por algo que nadie configuró, y eso enseña a poner `libre` en todos lados para que deje de
molestar — un default demasiado estricto termina en menos seguridad.

**Una prueba encontró un bug propio antes de que existiera:** `ResolverConsentimiento` arrancaba el
acumulador en el default y tomaba el máximo, con lo cual `avisa` quedaba de PISO y **`libre` era
inalcanzable** aun declarándolo en todas las fuentes — contradiciendo el comentario que la propia
función tenía escrito arriba. Quedó con prueba propia porque la forma de romperlo (un acumulador
que arranca en el default) es demasiado natural para no volver.

Seis sabotajes en este trozo, todos ejecutados. **Lo que falta de la fase 1:** el eje existe y está
probado, y **todavía no está enchufado** — falta dónde se guarda por máquina y que el agente sepa
preguntar. Eso, más la sesión como objeto del dominio, es lo que sigue.

**2026-08-28 (nonies) · el consentimiento deja de ser un tipo suelto y se aplica.**

La migración **38** agrega DOS columnas a `devices` y no una, porque son hechos de dueños
distintos: `consentimiento` es una POLÍTICA que escribe quien administra y no cambia sola;
`puede_preguntar` es una CAPACIDAD MEDIDA que reporta el agente y cambia con el mundo. Juntarlas
obligaría a que la política mienta sobre el hardware, o a que un latido pise la política.

`consentimiento` arranca VACÍO y no en un grado: el default vive en el dominio, y tenerlo también
en el esquema dejaría las filas viejas atrás el día que cambie. `puede_preguntar` arranca en 0
para todos, y eso es correcto aunque incomode — ningún agente desplegado sabe preguntar todavía, y
arrancar en 1 sería afirmar una capacidad que nadie midió.

**`musubi_fleet_consent`**, la tool que faltaba: una columna que nadie puede escribir es
decoración. Es **admin y no `screen`** a propósito — si quien tiene acceso pudiera aflojar la
política, se estaría autorizando a sí mismo a no avisar y el eje sería adorno.

**El camino de pantalla lo consulta antes de acuñar nada**, en el mismo lugar donde se verifica el
motor y por el mismo motivo: el daño de mirarlo tarde no es fallar, es ENTREGAR una contraseña de
sesión —que se muestra una sola vez— para una sesión que no se tenía que abrir. Va después de la
compuerta de capacidad: quien no tiene `screen` no puede enterarse de la política de una máquina
que no debería saber que existe.

**Tres decisiones que quedan escritas:**

1. **`pide` sin interlocutor se endurece a `prohibido`, no se afloja a `libre`.** La salida cómoda
   convierte la configuración más estricta en la más permisiva justo en las máquinas donde nadie
   está mirando. El costo es real: `pide` en un servidor headless traba el acceso. Es un error de
   configuración VISIBLE, que es la clase buena.
2. **La degradación se dice al configurar**, no cuando alguien no puede entrar: la tool devuelve
   `guardado` y `efectivo` con la nota de por qué difieren.
3. **`avisa` no bloquea y deja constancia de que el aviso no se entregó.** Bloquear cerraría la
   flota por una capacidad que nadie desplegó; prometerlo en silencio sería justo lo que este eje
   viene a evitar.

Cuatro sabotajes más (48 en la tanda). **Lo que falta:** la mitad del agente —dibujar el diálogo y
reportar `puede_preguntar`—. Hasta entonces `pide` es honesto: bloquea, en vez de fingir.
Registrado como **A57**.

**2026-08-28 (decies) · la sesión única resultó ser una VISTA, no una tabla. La maqueta decía otra
cosa y estaba equivocada.**

La maqueta de tres planos proponía fusionar `screen_sessions` y `shell_sessions` en una sola tabla:
tienen casi las mismas columnas y eso invita. **Al ir a hacerlo, la encuesta del código dijo que
no.** Las TABLAS se parecen; los COMPORTAMIENTOS no:

    shell     UltimoTrafico (techo de inactividad) · una sola abierta por (principal × máquina)
              · un barrendero que cierra las vencidas
    pantalla  ninguna de las tres

Una tabla común tendría columnas que sólo aplican a la mitad de sus filas — el olor de esquema que
este repo evita en todos lados, y el mismo error que sería meter `UltimoTrafico` en la vista y
dejar que su cero se lea como «sin tráfico» en vez de «no aplica».

Lo que sí se comparte es la FORMA DE AUDITORÍA —quién, dónde, cuándo, cómo terminó— y eso es una
vista. La consola necesita listar; no necesita que sean la misma fila. Queda escrito en el código
por qué la maqueta decía otra cosa, y bajo qué condición valdría revisarlo: el día que aparezca una
tercera modalidad que se comporte como una de las dos.

- **`fleet.SesionViva`** — la forma común, con la modalidad viajando (una lista que junta pantallas
  y shells sin distinguirlas no sirve para decidir nada) y SIN los campos propios de cada una.
  `Abierta` se DERIVA: una columna de estado miente en cuanto nadie la actualiza, y acá mentiría
  diciendo que alguien sigue adentro de una máquina cuando ya salió. Las tres formas de estar
  cerrada están cubiertas, incluida la que se olvida — una sesión **sin vencimiento no es eterna,
  es una fila mal formada**.
- **`SesionesVivas`** — lee las dos tablas y las junta. El tope se aplica POR MODALIDAD y después
  al total: sin eso, un proyecto con miles de shells devolvería sus tres pantallas fuera del corte
  y se leería como «acá no hay sesiones de pantalla», que es distinto de «hay, y no entraron». Los
  nombres se piden CON las máquinas revocadas: una sesión sobre una máquina dada de baja sigue
  siendo un hecho de la auditoría, y perder su nombre justo ahí es perderlo cuando más se necesita.
  El orden desempata por id porque dos sesiones creadas en el mismo instante —abrir pantalla y
  shell juntas desde un panel— saldrían distinto en cada llamada.

Seis sabotajes más (**54 en la tanda**). Y una prueba volvió a corregir a su autor: fijaba los ids
a mano cuando los acuña el storage, así que fallaba por eso y no por lo que decía custodiar.

**Lo que falta:** exponer la vista en `musubi_fleet_sessions`, que hoy sólo lista pantallas.

**2026-08-29 · la vista llega a la tool, y partir una capacidad tenía una mitad olvidada.**

`musubi_fleet_sessions` deja de listar sólo pantallas y trae las dos modalidades. Con eso
aparecieron dos cosas, y las dos las encontraron pruebas.

**El bug: partir `screen` exigía tocar DOS ejes y sólo se tocó uno.** La implicación quedó puesta
en el de la credencial (`tieneGrant`) y NO en el del aparato (`Device.Permite`), que comparaba por
igualdad. Una máquina enrolada con `caps: ["screen"]` —o sea TODAS las existentes— no tiene
`screen:view` en su lista, así que figuraba incapaz de mirar la pantalla que ya deja controlar.
El efecto habría sido **silencioso y retroactivo**: la bitácora de sesiones vacía para toda la
flota, sin un solo error. Lo cazó una prueba que ya existía.

**La compuerta de la bitácora es POR MODALIDAD.** Antes la tool listaba sólo pantallas, así que
`screen` alcanzaba para todo lo que devolvía; ahora también trae shells, y usar la misma capacidad
para las dos dejaría ver **quién tuvo un prompt** a alguien que no puede abrir uno. Las pantallas
piden `screen:view` —quien ya puede mirar esa pantalla no gana nada con que le oculten quién más
la vio— y las shells piden `shell`.

**Y una invariante declarada sin prueba es decoración:** el sabotaje de generalizar esa compuerta
NO falló nada la primera vez. El cambio más sensible de la tool no tenía guarda. Se escribió, y
ahora la fuga que el comentario describía tiene quien la cace.

`abierta` viaja DERIVADO y no el estado guardado: una sesión puede figurar `activa` habiendo
vencido sin que nadie la marcara, y dibujar el estado crudo mostraría gente adentro de máquinas de
las que ya salió.

Y `puedo` ahora lista las capacidades IMPLICADAS, que es lo correcto: `puedo` no dice qué te
concedieron —eso es `caps`— dice qué podés EJERCER ahora. Un panel que decide qué botones habilitar
tiene dos, no uno. El orden va de menos a más poder.

**58 sabotajes en la tanda.** Con esto la fase 1 queda sustancialmente cerrada del lado del
cerebro; lo que falta es A57, la mitad del agente.

**2026-08-29 (bis) · verificado en producción, y otro dato guardado que nadie podía leer.**

El despliegue de la fase 1 entró y se verificó contra la flota real. **La implicación funciona
sobre datos existentes**: `gio` y `kernelos-pc` están enroladas sólo con `screen` —ninguna declara
`screen:view`— y su `puedo` ahora lista las dos. Ése era el bug retroactivo que habría vaciado la
bitácora de sesiones de toda la flota.

Y la compuerta admin del consentimiento se verificó del modo correcto: el token del asistente es
`writer`, así que la tool lo rechazó.

**El hueco que apareció al verificar:** `musubi_fleet_consent` escribía la política y NINGUNA tool
la mostraba. Una política de acceso que no se puede leer no se puede auditar — el mismo hueco
exacto que tenía `agent_version`, encontrado del mismo modo: usando la cosa, no escribiéndola.

Ahora el inventario trae **los dos valores y no uno**: `consentimiento_efectivo` (lo que RIGE) y
`consentimiento` sólo si alguien lo DECLARÓ. Su ausencia dice algo distinto de su presencia —
«nadie lo decidió, rige el default» no es «alguien puso avisa»— y ésa es la pregunta que un auditor
hace primero. Más `puede_preguntar`, que es lo que explica la diferencia cuando difieren.

**Y llegó al panel.** `/flota` gana dos columnas:

- **acceso**, con tres estados que se distinguen a propósito: el grado a secas, el grado marcado
  como heredado cuando nadie declaró nada, y el efectivo CON marca cuando se endureció. Sin ese
  tercero, un `pide` degradado se vería igual que un `prohibido` decidido y la degradación se
  descubriría el día que una sesión no abre.
- **agente**, que es lo que distingue «binario viejo» de «enumerador roto» — las dos causas
  opuestas del mismo síntoma que costaron dos días.

El pie de la página explica que `acceso` y `puedo` son ejes distintos: una columna que aparece sin
explicación se ignora.

**63 sabotajes en la tanda.**

**2026-08-29 (ter) · los tres pasos del operador, y tres errores propios en el camino.**

Los tres quedaron cerrados —el panel ve la flota completa, el watchdog quedó declarado como
ausente, y el binario corre—. Lo que vale la pena registrar son los errores del camino, porque los
tres son de clases distintas y los tres se cazaron **usando** el sistema.

**1 · Se arreglaron TRES de cuatro tools de lectura.** El arreglo del alcance del panel se aplicó a
`fleet_list`, `fleet_metrics` y `fleet_services`; `fleet_sessions` se escribió después y quedó
afuera. **El síntoma fue mudo**: la columna de sesiones del panel vacía, sin error a la vista,
porque el panel ignora a propósito los errores de esa llamada para no borrar la flota de la
pantalla. Una protección propia escondió un bug propio.

La guarda que faltaba no es una prueba por tool: es una sobre **la clase entera**, que enumera las
cuatro y exige que todas funcionen para un `read: all` sin proyecto. Una quinta que se agregue
mañana rompe ahí.

**2 · Y esa prueba, en su primera versión, no cazaba el sabotaje.** Verificaba que la llamada no
diera error. Pero el fallback no da error: para un principal con `ProjectID` vacío devuelve la
lista `[""]` —UN elemento, no cero— así que la guarda de `len == 0` no salta, la consulta corre
sobre un proyecto que no existe, y la tool **devuelve vacío y exitoso**. Se reescribió para exigir
que las cuatro digan haber barrido LOS DOS proyectos. *«No falló» no es «hizo lo que dice».*

**3 · Se copió el `alertmanager.yml` del REPO encima del del SERVIDOR, y se reinició.** El del repo
lleva `chat_id: 0` como marcador —lo reemplaza `preparar.sh` al instalar—, así que quedó una
configuración inválida y el Alertmanager arrancó roto. Lo salvó el respaldo hecho antes de tocar
nada; fueron ~2 minutos con la cadena degradada.

**El repo y el servidor NO son copias.** El repo tiene marcadores donde el servidor tiene secretos,
y esa asimetría está escrita en los comentarios del propio archivo. Regla que queda: en cualquier
archivo de despliegue con secretos, **se edita el del servidor; nunca se copia el del repo encima**.

**Y una regresión propia, cazada antes de que llegara al teléfono:** comentar la ruta de
`MusubiSiempreViva` no la desactiva — la manda al receptor por defecto. Como esa alerta está
SIEMPRE en firing a propósito, habría pasado a notificar por Telegram y al CRM cada 4 horas, para
siempre: la misma alarma eterna, mudada al canal que sí se lee. La salida correcta es un receptor
**`null`**: la alerta se sigue viendo en el Alertmanager y no le llega a nadie. Es el patrón de
kube-prometheus con su Watchdog, y reactivar el dead-man's switch es cambiar una palabra.

**67 sabotajes en la tanda.**

**2026-08-29 (quater) · A44 cerrado: el módulo deja de sólo mirar.**

El bloqueo anotado era el COOLDOWN, y era real. Se llevaba por (política × máquina): dos políticas
sobre `nginx` y `postgres` de la misma máquina caían en la misma fila, así que reiniciar uno
**dejaría muda** a la política del otro durante todo el enfriamiento — y el segundo se quedaría
caído sin que nada actúe, justo por haber actuado sobre el primero. Peor que no tener la política:
da la sensación de que algo vigila.

La **migración 39** recrea `fleet_policy_state` con la clave `(policy, device_id, alcance)`. Se
recrea porque SQLite no sabe cambiar una PRIMARY KEY. La columna se llama `alcance` y no
`servicio` a propósito: representa QUÉ toca la política adentro de la máquina, y lo próximo que se
vigile ahí —un contenedor, un montaje, una interfaz— va a querer el mismo espaciado sin migrar de
nuevo.

**La decisión de seguridad: no existe «reiniciá el que se cayó».** El nombre del servicio lo
REPORTA LA MÁQUINA; sustituirlo dentro de un argv haría que un dato no confiable termine siendo
argumento de algo que el cerebro ejecuta, con la allowlist validada ANTES de saber qué se va a
ejecutar. Sería decoración exactamente en el camino donde no hay una persona mirando. Con el
servicio nombrado, el argv es fijo al validar.

**La bifurcación es de la DECISIÓN, no de la ACCIÓN.** Las políticas de servicio no pasan por las
guardas de la muestra —son de la telemetría del host— y comparten todo lo demás: cooldown,
compuerta del principal, allowlist, bitácora. Dos caminos con reglas distintas para ejecutar es
cómo uno se queda sin una guarda el día que se toca el otro.

Y tres asimetrías que llegan por fin al plano de actuar: **`desconocido` no es `caído`** (no pudo
enumerar ≠ está caído), **un inventario viejo no dispara** (el agente deja de mandarlo a propósito
cuando una fuente falla, así que «sin noticias» es un estado que producimos nosotros), y **un
servicio ausente del inventario no es un servicio caído** (una política que reinicia lo que no
existe se lleva puesto el host donde alguien escribió mal el nombre).

**Una pieza faltaba y sólo apareció al montar el andamiaje:** `config.PolicyConfig` no tenía el
campo `service`. El dominio soportaba políticas de servicio y no había forma de declararlas —
habría quedado una función completa, probada e inalcanzable.

**Y cuatro sabotajes seguidos obligaron a rehacer una prueba, siempre por lo mismo:** el andamiaje
no ejercitaba lo que la prueba decía. Miraba el cooldown (que se escribe DESPUÉS de la compuerta),
después el aviso (que el servidor de prueba no puede emitir sin registro), después usó el principal
equivocado, y por último forzó una frescura que ya estaba. Ninguno era un bug del código: los
cuatro eran pruebas que pasaban por el motivo equivocado, y sólo el sabotaje lo revela.

**84 sabotajes en la tanda.**

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
6. **El número es la identidad: uno solo por cosa, y para siempre.** Un número nuevo va por encima
   del máximo en uso (hoy **A75** y **B21**) y NO se recicla uno libre: `A6`-`A9`, `A15` y `A16`
   nunca se usaron, y estrenarlos ahora haría que un lector con el archivo viejo en la cabeza lea
   otra cosa. Si un cabo se convierte en otro —de la tabla 1 a la 2, o al revés— la fila nueva dice
   **«(era A33)»** y la vieja se borra: sin esa marca, cada cita del número anterior apunta a la
   nada. Y si al cerrar algo tenés que citarlo, citalo donde se cierra («A70 CERRADO», «cierra
   A13»), que es lo que permite seguir la pista desde un comentario del código.
   Lo custodian `TestNingunNumeroDeRegistroSeUsaDosVeces` y
   `TestUnCaboVivoNoApuntaAUnNumeroQueElRegistroNoDefine`.
