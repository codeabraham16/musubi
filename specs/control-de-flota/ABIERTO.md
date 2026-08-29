# Control de flota — registro de lo ABIERTO

> **Nada queda abierto sin dueño.** Cada línea de acá tiene un slice asignado o una razón
> declarada de por qué NO se va a hacer. Si algo se cierra, se borra de esta tabla; si aparece
> algo nuevo, se anota acá el mismo día.
>
> Última revisión: **2026-08-28** (tras S6c, S7b, las dos auditorías, el DESPLIEGUE REAL en `musubi-server`,
> el **despliegue del relay de pantalla + el arreglo de la ruta de RustDesk**, y la **integración de
> U1 (procesos y memoria libre), S11 (empuje OTLP) y S12 (servicios)** — de donde salen A46 y A47,
> que ningún slice podía ver solo).
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
| A31 | **El binario de Windows no está firmado** | NordVPN —y cualquier EDR con filtrado por reputación— bloquea la salida de ejecutables sin firma, y el síntoma (`WSAEACCES`) no menciona ni firma ni antivirus: `curl.exe` da HTTP 200 y el binario da `WSAEACCES` al mismo host y puerto. Se sortea con una excepción de *split tunneling* **por ruta**, que se rompe si el binario se mueve. **PRUEBA LIMPIA (2026-08-27)**: el MISMO binario, en la MISMA máquina, dio `WSAEACCES` como `musubi-nuevo.exe` y latió sin problema medio minuto después como `musubi.exe` — la excepción de NordVPN es por RUTA, y eso quedó medido en vez de deducido. **ALCANCE MEDIDO: hoy afecta a UNA máquina** (`kernelos-pc`, la única con NordVPN, confirmado por gio 2026-08-27); en el resto el agente conecta sin nada. Por eso NO es urgente y NO justifica todavía el costo de un certificado. **Se revisa si aparece una segunda máquina con filtrado por proceso, o si se despliega fuera de la red propia** — ahí el certificado deja de ser un lujo. | **acción del operador** (cuesta plata y trámite) |
| A32 | **Las alertas no se ven en el CRM** | `public.alertas` se llena y la vista `alertas_activas` existe, pero **ninguna página las muestra**: hoy sólo se ven por Telegram o por SQL. Falta una vista en `crm-musubi.git` (Next.js, ya usa `@supabase/ssr`). **Gio decidió posponerlo el 2026-08-27**: las alertas llegan al teléfono y quedan escritas, que es lo que importa; lo visual espera a que se ordene el monitoreo (A33). | **sin asignar** (decisión de gio) |
| A33 | **CINCO cosas de monitoreo mirando lo mismo, no tres** | Al ir a mirar de verdad (2026-08-27) aparecieron dos más de las que decía esta línea: además de **OpenObserve** (`:5080`, que alimenta la sección Monitoreo del CRM), **Uptime Kuma** y **Prometheus + Alertmanager** (Telegram + CRM), hay un **`alerter.py` por cron cada 5 min** que consulta OpenObserve y avisa por **WhatsApp** (puente Baileys), con su propia máquina de estado, sus propios umbrales (disco 85 %, RAM 95 %) y su propia meta-vigilancia de colectores; y **UptimeRobot**, que el propio `alerter.py` nombra como quien cubre la muerte total del server. Volumen medido del canal de WhatsApp: **200 avisos en 30 días** (28 jul → 27 ago), o sea 6-7 por día — molesto, no una manguera. **La duplicación no es hipotética**: `alerter.py` alerta por disco ≥85 % y `DiscoPorLlenarse` también; hoy miran objetivos distintos, pero es el mismo trabajo hecho dos veces. Prometheus entró porque las reglas de Musubi están escritas para él — pero **debí preguntar antes de montarlo**. **Se revisa antes de agregar cualquier vista nueva de monitoreo** (bloquea A32 y A36). | **acción del operador** (es una decisión de producto, no técnica) |
| A14 | **Grabación de sesión de pantalla** | Decisión legal antes que técnica; no se hace sin que alguien la tome. | sin asignar |
| A18 | **Pantalla en Android** (scrcpy sobre ADB) | La matriz de S1 concede `screen` a Tier C, pero el motor es otro distinto del de RustDesk. **Su sombra ya está tapada (S6c)**: pedir la pantalla de un Tier C se NIEGA y la capacidad inerte se ve en el inventario y en el panel. Falta el motor. | **S8b** |
| A20 | **iOS: medir o controlar** | Requiere un MDM con perfil de supervisión — un producto entero. Musubi lo tiene en el inventario y lo dice. **Puede que nunca se haga, y está bien.** | sin asignar |
| A17 | **SNMP / MQTT / Redfish** | Los tres piden una librería (dependencia nueva) o un protocolo binario a mano. SSH cubre routers, NAS, Raspberry Pis y servers sin agente — la mayoría de lo que hay. | **S7c** |
| A26 | **`musubi shell` no funciona desde Windows** | El modo crudo se pide con `stty`, que no existe en la consola de Windows (ahí es `SetConsoleMode`). Desde Linux o macOS sí, contra cualquier Tier B. | **S5d** |
| A27 | **La ventana no se redimensiona (SIGWINCH)** | No es «no se hizo»: el transporte elegido no lo permite. En Tier B el pty lo posee el `sshd` remoto y en Tier A lo posee `script`, así que **no tenemos su descriptor maestro** y no hay a quién mandarle un `TIOCSWINSZ`. El tamaño se fija al abrir. **Medido contra un `sshd` real (S7b)**: el `ioctl` del pty remoto da `0 0`, pero `tput` devuelve 24/80 y `top` dibuja — el fallback por `LINES`/`COLUMNS` alcanza para lo que se usa. Si el redimensionado importa de verdad, obliga a escribir el pty a mano —ioctls por OS y por arquitectura— y entonces se paga entero. | **S5d** |
| A35 | **El relay propio está desplegado y VACÍO** | `hbbs`/`hbbr` corren en `musubi-server` atados al tailnet, con su clave generada y los cuatro puertos contestando — y **ningún cliente se registra contra él**: los dos Windows siguen apuntando al servidor PÚBLICO de RustDesk. Cambiar la configuración del cliente por el canal de comandos **cortaría la sesión de RustDesk que gio está usando en ese momento**, así que no se hace de prestado. Ojo con lo que esto NO significa: el plano de pantalla de Musubi **funciona igual** contra el servidor público —la compuerta, la contraseña acuñada, el vencimiento y la bitácora son de Musubi, no del relay—; lo que falta es dejar de depender de infraestructura ajena para el video. | **acción del operador** (②) |
| A36 | **Al relay no lo vigila nadie** | Si `hbbs` muere, la primera noticia es que alguien no puede abrir una pantalla. No hay regla de alerta ni target de scrape: el relay no expone métricas de Prometheus, así que haría falta un `blackbox_exporter` (una pieza más) o una regla sobre el latido del contenedor. **Sale del mismo nudo que A33**: montar más vigilancia antes de decidir cuál de los tres stacks es el de verdad es agrandar el problema. | **sin asignar** (después de A33) |
| A37 | **La identidad del relay sólo está a salvo de la mitad de las cosas** | `~/musubi-rustdesk/data/id_ed25519` es la identidad del relay: si se pierde, el relay vuelve con OTRA clave y **todos los clientes de la flota dejan de conectar hasta que alguien los reconfigure uno por uno**. **Media parte resuelta (2026-08-27)**: `preparar.sh` deja una copia en `.musubi/backups/rustdesk-relay/` con permiso 0600 —más cerrado que el 0644 del original— y un `LEEME.txt` con el procedimiento de restauración. Eso cubre que el volumen se borre, un `preparar.sh` mal corrido, o que el contenedor se lleve el archivo. **Lo que sigue abierto es lo otro**: la copia vive en el MISMO disco, y el backup del cerebro **sigue siendo local-only** por decisión de gio del 2026-08-27. Contra perder el host no protege nada, y eso está dicho en la salida del script y custodiado por una prueba — un respaldo que no aclara contra qué NO protege es peor que ninguno, porque alguien deja de buscar el de verdad. | **acción del operador** (③ · `BACKUP_REMOTE`) |
| A38 | **La columna de procesos no está en el panel** | U1 absorbió `num_procesos` y `mem_libre` en la muestra: los dos llegan a `musubi_fleet_metrics` y a Prometheus (`musubi_fleet_device_processes`, `musubi_fleet_device_memory_free_bytes`), y **el panel no los dibuja**. `cmd/musubi/assets/flota.html` no cambió, a propósito: la vista es un slice aparte y meterla en el de la medición mezcla dos diffs que se revisan distinto. Ojo con el dato: la columna tiene que distinguir «0 procesos» de «no medido» —macOS y los agentes viejos mandan 0— o repite en la pantalla el cero mentiroso que el resto del track evita. | **sin asignar** (slice de panel) |
| A39 | **`num_cpu` se publica crudo y un 0 se lee «esta máquina no tiene CPUs»** | `methods_fleet.go` copia `m.NumCPU` tal cual a la respuesta de la tool. Es exactamente la clase de cero mentiroso que U1 arregló para `num_procesos` con `enteroONull`, y el helper ya está escrito al lado. **No se hizo en U1 a propósito**: cambiarlo toca el camino de una métrica que ya se está mirando en producción, y merece su propio diff y su propia prueba en vez de viajar de polizón. Barato de hacer, barato de olvidar. | **sin asignar** (un diff de tres líneas) |
| A41 | **El empuje no tiene backoff con memoria entre ticks** | Un destino caído se reintenta cada 30 s para siempre, con el mismo intervalo. No hay outbox ni espaciado creciente: el reintento ES el próximo tick. Está acotado a propósito —el aviso de un fallo permanente sale UNA vez y `musubi_push_failures_total` cuenta el resto—, así que el costo real de un destino muerto es un POST fallido cada 30 s contra loopback. **Se revisa si el destino alguna vez deja de ser loopback**: contra un collector remoto, reintentar sin espaciar es exactamente cómo se martilla a alguien que ya está caído. | **sin asignar** (después de que el push tenga un destino remoto) |
| A44 | **No hay política de flota sobre la salud de un servicio** | Las políticas de S10 pueden reiniciar algo cuando el disco se llena, y **no** cuando un servicio se cae. La razón es de esquema, no de diseño: `fleet_policy_state` tiene clave `(policy, device_id)` y una política sobre un SERVICIO no entra en esa clave sin migrar la tabla — dos servicios de la misma máquina compartirían cooldown, que es peor que no tener la política. Queda anotado ahora, que es barato, en vez de cerrarse la puerta. | **sin asignar** (después de A42 y A43) |
| A45 | **`go test -race ./...` no termina en 30 minutos** | No es un deadlock ni una carrera: **la corrida completa reporta CERO `DATA RACE`**. Es que `modernc.org/sqlite` —el SQLite en Go puro que evita cgo— corre ~10× más lento bajo el detector, y **cada prueba abre una base nueva que aplica las 37 migraciones**. Medido: una sola base cuesta **13,5 s** bajo `-race`; con cientos de pruebas, `internal/mcp` e `internal/memory` se comen el `-timeout 30m` parseando SQL (`runnable` dentro de `applyMigrations`, no bloqueado). **Empeora sola con cada migración nueva, y este trabajo agregó dos**: medido contra un worktree limpio de HEAD, 12,7 s → 13,5 s, un **6 %**. Casi todo el costo es anterior. La CI usa `-race` y nadie lo miraba porque `go test ./...` sin él pasa entero. Lo que se necesita es que las pruebas **compartan una base migrada** (una plantilla que se copia) en vez de migrar de cero cada una. | **sin asignar** |
| A49 | **Nada verifica lo que sale por el cable del empuje** | `receptorDePrueba` no mira el método, ni un header, ni el cuerpo; `ultimoCuerpo` es **código muerto** — está definida y no la llama ninguna prueba. El verificador dejó tres sabotajes en verde con la suite entera: si alguien toca `enviar` y borra el `Content-Type`, Prometheus contesta 400 a cada POST y nada se pone rojo. **Media parte tapada (2026-08-28)**: la prueba nueva contra un Prometheus de verdad (`fleet_otlp_real_test.go`) sí ejercita el cable completo — pero es **opt-in**, no corre en CI, así que la suite de todos los días sigue sin mirar el sobre. Falta una prueba de unidad sobre método, headers y `Content-Type`. | **sin asignar** |
| A50 | **Revocar la concesión `metrics` en caliente deja el empuje mudo** | El arranque exige que el principal del empuje tenga `metrics`, pero `principals.yaml` **se recarga cada 10 s**: si alguien le saca la concesión después, `empujarUnaVez` deja de mandar puntos y **no avisa ni cuenta un fallo** — `pedidos` se congela y `puntos` queda en 0. Medido por el verificador con tres ticks. El empuje muere en silencio con la configuración perfecta, que es el modo de fallo que este track persigue. | **sin asignar** |
| A51 | **`incluir_revocados: true` promete algo que no puede dar** | El esquema de `musubi_fleet_services` dice «Incluir los servicios dados de baja **y los de máquinas revocadas**». La segunda mitad es falsa: los servicios de una máquina revocada no salen nunca, porque el listado los filtra por `PuedeSobreDevice` sobre un device que ya no está en el barrido. Las filas existen en la base —la migración 36 y `RevocarServiciosDeDevice` las conservan a propósito para la auditoría— y no hay forma de verlas. **O se arregla el comportamiento, o se arregla la promesa**; hoy la tool miente en su propia descripción. | **sin asignar** |
| A52 | **La fila del sondeo de Tier B no tiene ninguna prueba** | El verificador borró la línea `fila["mem_libre"] = m.MemLibre` y cambió `enteroONull(m.NumProcesos)` por el entero crudo, y `go test ./internal/mcp -run 'Sond\|Barrido\|TierB'` quedó en **ok**. Un operador que sondea un Tier B ve la respuesta sin `mem_libre` y con `num_procesos: 0` —el cero que significa «no sé», que es justo el bug que el campo nuevo vino a evitar— sin que nada se ponga rojo. | **sin asignar** |
| A57 | **El agente no sabe pedir consentimiento, así que `pide` bloquea en vez de preguntar** | El eje de consentimiento está en el dominio, en el esquema (v38) y aplicado en el camino de pantalla. Lo que falta es la mitad del AGENTE: dibujar un diálogo en la máquina destino, esperar la respuesta, y reportar `puede_preguntar` en el latido. Mientras no exista, `puede_preguntar` es 0 para toda la flota y un `pide` se endurece a `prohibido` — que es honesto y es el comportamiento correcto, pero significa que **el grado más útil del eje todavía no se puede usar**. También falta la entrega del `avisa`: hoy se abre la sesión y queda un WARN en el log del cerebro diciendo que el aviso no se pudo entregar. Es visible, no silencioso, y no alcanza. | **sin asignar** (fase 1 de la maqueta) |
| A56 | **El principal del panel no tiene concesión de flota, así que ve el inventario y no el estado** | `panel-central` está en `principals.yaml` con `read: all` y SIN sección `fleet:`. Las capacidades de flota no se derivan del rol a propósito (si no, un token de lectura sería una puerta trasera al eje de capacidades entero), así que el panel lista las máquinas y recibe `sin_permiso: 3` en métricas y `sin_permiso: 54` en servicios. **No es un bug: es la compuerta funcionando y nadie se la concedió.** Se cierra agregándole `fleet: {metrics: ["*"]}` — y NADA más: ni `exec`, ni `screen`, ni `shell`. Un panel mira; una credencial que vive en la configuración de otro servicio es exactamente la que no querés que pueda ejecutar nada. | **acción del operador** (escribir en `principals.yaml` está fuera de lo que puede hacer el asistente) |
| A54 | **El blindaje del agente no se revisa cuando el agente gana una capacidad** | `musubi-agente.service` se blindó para «lee /proc y habla con loopback» (`ProtectHome=read-only`, `ProtectSystem=strict`). A42 le dio un trabajo nuevo —enumerar contenedores— y el blindaje lo prohibía. El síntoma no fue «permiso denegado» en ningún lado: fue `podman ps` saliendo con código 1, y **hasta este track, en silencio**. Resuelto para contenedores con un drop-in acotado; lo que queda abierto es que **no hay nada que ate una capacidad del agente a las rutas que necesita**. La próxima —leer un log, tocar un socket, escribir un cache— va a repetir la forma exacta: la función no anda, la unidad no dice por qué, y nadie sospecha del archivo que está bien. Lo que se necesita es que el agente **declare** lo que va a tocar y que el despliegue lo verifique, en vez de descubrirlo en producción. | **sin asignar** |
| A53 | **`TestPushDelPorteDeProduccionCruzaEntero` falla bajo `-race`, solo y sin carga** | Federa 14.000 nodos (5,2 MB crudos) con un plazo de 60 s para cliente y servidor. Bajo el detector de carreras, comprimir y serializar eso tarda **más de 90 s**, y el test muere con `context deadline exceeded` — **no es una carrera**: la corrida no reporta un solo `DATA RACE`. **No lo puso este trabajo, y se midió en vez de suponerse**: en un worktree limpio de HEAD falla igual, 93,04 s y el mismo mensaje. Distinto de A45 (que es la suite entera comiéndose el timeout por acumulación): éste falla **aislado**. El arreglo es del test, no del código: o su plazo se escala cuando corre instrumentado, o el grafo de prueba baja de tamaño sin dejar de cruzar `maxRequestBody`. | **sin asignar** |
| A22 | **El otro lado del dead-man's switch — y no es sólo el de Musubi** | `MusubiSiempreViva` late hacia un receptor `watchdog`, pero hace falta un servicio EXTERNO que espere ese ping y grite si falta (Healthchecks.io, Dead Man's Snitch, o un cron en otra máquina). **No era media alarma: era NINGUNA**, y el motivo anotado acá tapaba algo más grande (ver A29). Es el eslabón 4 de 4. **Y hay un segundo latido igual de desarmado en la misma máquina** (2026-08-27): `monitoring/infra/alerter.py` tiene escrito su propio dead-man —late a `HEARTBEAT_URL` en cada corrida, y sólo si el puente de WhatsApp responde, que es un diseño mejor que el nuestro— pero **`HEARTBEAT_URL` está vacía en `meta.env`**. Dos sistemas independientes con el mismo hueco y por el mismo motivo: el código está, el endpoint externo no. **Una sola cuenta de watchdog resuelve los dos**, con un check por cada uno — nunca compartiendo el mismo, porque entonces un latido tapa la muerte del otro. | **acción del operador**, después de A29 (`deploy/docker/README.md`) |

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
