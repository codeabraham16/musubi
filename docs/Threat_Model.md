# Modelo de amenazas — cerebro central de Musubi

Alcance: el nodo central (`musubi serve`) que agrega la memoria **compartida** de varios
proyectos sobre una malla privada (Tailscale/WireGuard). El daemon local (`musubi daemon`,
stdio) queda fuera: corre en la máquina del dev, sin superficie de red, bajo confianza local.
El **plano de flota** —agente, exec, shell y pantalla— tiene su propio borde de confianza y va
en su propia sección, al final de este documento.

## Borde de confianza

- **Confiable:** el transporte de la malla (WireGuard cifra e autentica en tránsito entre
  peers) y el disco del servidor (acceso root/OS del host).
- **NO confiable:** cualquier cliente que presente un token, incluso dentro de la malla; un
  dispositivo de la malla comprometido; un token reusado fuera de la malla.

WireGuard da **confidencialidad e integridad en tránsito dentro de la malla**. NO da
autorización (quién puede hacer qué), ni aislamiento entre proyectos, ni protección si un
token se filtra o un peer se compromete. Esas garantías las provee Musubi por encima.

## Activos

1. La `memory.db` central (memoria compartida de todos los proyectos) — confidencialidad e
   integridad.
2. Los tokens de los principals (credenciales de acceso).
3. La disponibilidad del servicio (es el punto de convergencia de la memoria del equipo).

## Amenazas y mitigaciones

| Amenaza | Mitigación (dónde) |
|---|---|
| Fuga de un token compartido = acceso total | **Identidad por-principal** (16.1c): un token por miembro, revocable individualmente; el archivo guarda el SHA-256, no el token |
| Un miembro lee la memoria de otro proyecto | **Aislamiento por proyecto** derivado de la credencial: `recall` (16.1c-3) y las lecturas de **contenido** —`search_keyword`, `search_semantic`, `memory_expand`— se acotan al `project_id` del principal (T17.1a); la **escritura** también se atribuye por credencial, no por lo que declare el cliente (T17.1b-1). Solo `admin` ve federado. *Pendiente (T17.1b-2): las superficies de metadata/grafo (`recall_facts`, `entity_context`, `recall_code`, `insights`, `conflicts`) todavía consultan federado.* |
| Escalamiento: un `reader` muta memoria | **Authz por rol** (16.1c): `reader` solo tools de lectura; deniega con `codeUnauthorized` |
| Un secreto crudo entra al pozo compartido | **Redacción forzada server-side** en TODO ingest al central —`save_observation` (content + topic_key, **antes** del embedding), `save_fact` y `save_code`— cuando el bind es no-loopback (T17.2), fail-closed, sin importar el `scope` declarado. **Es BEST-EFFORT heurístico** (formas conocidas de secreto + entropía): **reduce, no garantiza** la fuga —un secreto corto o de baja entropía puede escapar—; no confiar en la redacción como única barrera. |
| Fuerza bruta del bearer | **Lockout** (16.1e): N fallos por IP ⇒ bloqueo temporal; comparación en tiempo constante (no filtra por timing) |
| Token en texto plano en tránsito | Fail-closed: bind no-loopback **exige** token; sin TLS, hay que optar explícitamente por `allow_insecure_token` (válido solo si WireGuard/un proxy cubren el cifrado) |
| DNS-rebinding (modo loopback) | Chequeo de Host loopback + Origin local |
| DoS por body gigante / slow-loris | `MaxBytesReader` 4 MiB en el cable + timeouts de lectura/escritura |
| Bomba de descompresión (`Content-Encoding: gzip`) | Segundo tope de 64 MiB sobre el body **ya descomprimido**; el de 4 MiB sigue rigiendo en el cable. Se apoya en que la **auth corre ANTES de leer el body**, así que sólo un principal autenticado puede mandar comprimido: si alguna vez se mueve ese orden, hay que revisar el número |
| Movimiento lateral desde cualquier peer de la malla | **ACLs de Tailscale** (ver abajo): restringir el puerto del brain a principals concretos, no confiar solo en el rango CGNAT |
| Pérdida del disco central | **DR** (16.0b): backup consistente off-host + restore probado |

## ACLs de Tailscale (defensa en profundidad)

La regla de firewall del host abre el puerto a todo el rango de la malla (`100.64.0.0/10`).
Por defecto la **policy de Tailscale es allow-all**, así que cualquier dispositivo del tailnet
alcanza el brain. Restringilo en la [policy del tailnet](https://tailscale.com/kb/1018/acls),
por ejemplo permitiendo el puerto solo desde un tag de dispositivos autorizados:

```jsonc
{
  "tagOwners": { "tag:musubi-client": ["autogroup:admin"] },
  "acls": [
    // Solo los dispositivos etiquetados como musubi-client llegan al brain:7717.
    { "action": "accept", "src": ["tag:musubi-client"], "dst": ["tag:musubi-brain:7717"] }
    // (sin una regla que lo permita, el resto del tailnet NO alcanza el puerto)
  ]
}
```

Estrechar solo la regla de firewall del host no basta: sin ACLs, la policy default de
Tailscale ya deja pasar a todos.

## Riesgos residuales (conocidos)

- **Host comprometido:** root en el servidor lee la `memory.db` y el registro de hashes (no
  los tokens crudos, pero sí puede reemplazarlos). Fuera de alcance de la app.
- **Reuso de token fuera de la malla:** si un token se usa por fuera de WireGuard sin TLS,
  viaja en claro. Mitigar con TLS o no exponer el servicio fuera del tailnet.
- **Confianza en el `project_id` declarado por el sync client:** el ingest preserva el
  `project_id` de origen que envía el cliente (16.1a). Dentro de la malla con tokens
  por-principal es aceptable; un endurecimiento futuro es derivar también el `project_id` de
  ESCRITURA de la credencial, no solo el de lectura.

---

# Modelo de amenazas — el plano de flota

Alcance: el track «Control de flota» — el registro de dispositivos, el agente que late, y los tres
planos que TOCAN una máquina ajena: **telemetría**, **exec/shell** y **pantalla**. Va aparte del
cerebro de memoria porque el activo es de otra clase: la memoria compartida es un dato que se LEE;
una máquina de la flota es un host donde se EJECUTA.

Y conviene decirlo de frente: un plano de exec + shell + pantalla sobre red **tiene la forma de un
RAT**, y lo único que lo separa de uno es el modelo de autorización. Por eso esto no es un anexo —
es la parte del diseño que hay que poder mostrar antes que el código
(`specs/control-de-flota/proposal.md`, sección «Seguridad como primera clase»).

## Borde de confianza (flota)

- **Confiable:** el transporte de la malla y el disco del cerebro, con los mismos supuestos de
  arriba.
- **NO confiable: el AGENTE.** Corre en CADA máquina de la flota —la de un cliente, un portátil
  que viaja, un Windows con un antivirus ajeno—, así que es la superficie que más probablemente se
  compromete. Todo lo que reporta (la muestra, el inventario de servicios, la versión, el
  `rustdesk_id`) es **entrada de un cliente**, aunque su credencial sea válida.
- **NO confiable: el operador**, más allá de lo que su credencial concede. Ser admin de la memoria
  NO otorga nada sobre la flota: la ausencia de concesión significa ninguna máquina, nunca todas
  (`internal/mcp/fleet_authz.go:21`, `internal/mcp/principals.go:64`).

Las credenciales viven en **DOS ALMACENES DISTINTOS, y la separación es estructural** y no una
promesa: las personas en `principals.yaml` autentican en `/mcp`; los dispositivos en la tabla
`devices` autentican en `/fleet/…` y **sólo ahí** (`internal/mcp/fleet_http.go:18`). Si el token de
un agente abriera `/mcp`, robar UNA máquina cualquiera entregaría `musubi_recall` sobre la memoria
de toda la empresa: el plano de monitoreo se volvería el plano de exfiltración.

## Activos (flota)

1. **El token de dispositivo** — la credencial del agente: autentica el latido, la entrega de la
   cola y el reporte de resultados.
2. **La cola `device_commands`** — lo que una máquina VA A EJECUTAR. Escribir ahí es ejecución
   remota diferida, con o sin alguien mirando.
3. **La contraseña de una sesión de pantalla** — el permiso más invasivo del track.
4. **El relay RustDesk** (`hbbs`/`hbbr`) y su identidad `data/id_ed25519` — el camino por donde
   viaja el video, FUERA del proceso de Musubi.
5. **`principals.yaml`** — quién puede qué sobre qué máquina. Es el único lugar donde se otorga
   capacidad de flota, y a propósito no hay ninguna tool que la conceda.
6. **La bitácora** — `device_commands`, `screen_sessions`, `shell_sessions` y el usage ledger:
   quién tocó qué máquina, cuándo y con la autoridad de quién. Es lo que se mira después de un
   incidente, que es justo cuando importa que exista.

## Amenazas y mitigaciones (flota)

| Amenaza | Mitigación (dónde) |
|---|---|
| **Agente comprometido** afirma ser otra máquina, o ensucia el panel de todas | **La identidad sale del token y de ningún otro lado**: el cuerpo del latido no tiene dónde poner un `device_id`, un `name` ni un `project` (`internal/mcp/fleet_http.go:53`), y el id se resuelve buscando el hash de la credencial (`internal/memory/devices.go:102`). El id tampoco lo elige quien se enrola: lo genera el cerebro (`internal/memory/devices.go:32`). Un resultado se acepta sólo para un comando de la PROPIA máquina, y una fila terminada no se re-escribe (`internal/memory/comandos.go:253`). Y el techo de lo que puede correr vive en la CREDENCIAL de la persona, no en el dispositivo: una máquina comprometida no puede auto-otorgarse nada (`internal/mcp/principals.go:76`) |
| **Cerebro comprometido** | **Lo que NO tiene, que es toda la mitigación posible acá**: del token de dispositivo guarda el SHA-256 y nunca el crudo (`internal/memory/devices.go:39`, `internal/fleet/device.go:406`); la contraseña de pantalla **no tiene columna** —la estructura de la sesión no la puede guardar— (`internal/fleet/pantalla.go:6`); y de una shell no se guarda el contenido, ni lo tecleado ni lo impreso (`internal/fleet/shell.go:9`). Root en el cerebro NO recupera credenciales de la flota. Lo que sí puede —encolar comandos— es riesgo residual y está declarado abajo |
| **Operador malicioso** (credencial legítima, intención ajena) | **La compuerta es una conjunción de tres lados y ninguno suple a otro**: tenencia del proyecto ∧ concesión sobre ESA máquina ∧ que el aparato la admita (`internal/mcp/fleet_authz.go:35`). `shell` es una capacidad APARTE y no se deriva de `exec`, porque quien obtiene un prompt se saltea cualquier allowlist (`internal/mcp/methods_shell.go:54`, `internal/fleet/device.go:68`); la allowlist por comando recorta `exec` y su default de «no lo pensé» es «no puede» (`internal/mcp/fleet_authz.go:172`); dar de alta y revocar son admin (`internal/mcp/methods_fleet.go:89`, `internal/mcp/methods_fleet.go:378`) y conceder una capacidad al enrolar exige tenerla con comodín (`internal/mcp/fleet_authz.go:139`). Las sesiones tienen techo duro y de inactividad (`internal/fleet/pantalla.go:25`, `internal/fleet/shell.go:37`, `internal/fleet/shell.go:43`), la concesión se **re-evalúa a mitad de sesión** —revocar corta el prompt abierto— (`internal/mcp/shell_relay.go:137`), y **todo queda escrito, también los rechazos** (`internal/mcp/methods.go:140`). La bitácora se muestra por capacidad y no por tabla: una fila de comandos cuyo argv es `musubi:pantalla` pide `screen:view` (`internal/fleet/cronologia.go:235`) |
| **Token de dispositivo filtrado** | **Kill-switch inmediato**: `musubi_fleet_revoke` marca la fila y **borra el hash** en la misma transacción, arrastrando sus servicios (`internal/memory/devices.go:264`, `internal/memory/devices.go:270`); un device revocado no admite nada de nadie, y ése es el primer lado de la compuerta (`internal/mcp/fleet_authz.go:39`, `internal/fleet/device.go:257`). El token **no abre `/mcp`** (`internal/mcp/fleet_http.go:621`) y no puede nombrar el comando de otra máquina (`internal/mcp/fleet_http.go:445`). La fila QUEDA: la auditoría de después del incidente necesita saber de quién era la telemetría |
| **Renombrar como cambio de autorización (A64)** | Tres cosas indexan por NOMBRE de máquina y ninguna por id —las concesiones, la allowlist de comandos y el alcance de las políticas—, así que un rename puede SACAR `exec`, DARLO, o meter una máquina adentro de una política que la va a tocar sola (`internal/mcp/principals_reload.go:22`). Por eso **la tool no renombra en el primer llamado**: informa el impacto de los DOS nombres —el que se pierde y el que se HEREDA— y se planta (`internal/mcp/methods_renombrar.go:3`, `internal/mcp/methods_renombrar.go:70`); el recorrido del registro es `impactoDeNombre` (`internal/mcp/principals.go:373`), es admin (`internal/mcp/methods_renombrar.go:41`), y el nombre nuevo se valida antes (`internal/fleet/device.go:320`). **No arregla `principals.yaml` solo**, a propósito: sería el cerebro editando la credencial de una persona (`internal/mcp/methods_renombrar.go:23`) |
| **La muestra y el inventario son entrada no confiable** | Una muestra increíble se **RECHAZA ENTERA** y el latido sigue valiendo —estar viva y saber medirse son cosas distintas—, y no se «corrige» el valor, porque corregir esconde el problema (`internal/fleet/muestra.go:141`). La telemetría tiene su propio techo, más chico que el del transporte, y el del cuerpo entero no lo afloja (`internal/fleet/muestra.go:122`, `internal/mcp/fleet_http.go:304`, `internal/mcp/fleet_http.go:338`); el inventario tiene tope por servicio y por latido (`internal/fleet/servicio.go:66`) y el nombre de una unit se valida (`internal/fleet/servicio.go:264`). Lo que la máquina dice de sí misma sale a la exposición de Prometheus, así que el valor de cada label se ESCAPA: un device llamado `a"b` partiría el scrape entero, no sólo su serie (`internal/mcp/fleet_prometheus.go:326`). Y el texto de un `node_exporter` ajeno se recorre a mano, sin confiar en su forma (`internal/fleet/exposicion.go:127`). **Un dato que no se pudo medir se OMITE, no se inventa un cero** (`internal/fleet/muestra.go:7`, `internal/fleet/alcance.go:20`) |
| **La contraseña de pantalla queda para siempre en el argv de la bitácora (A74)** | Tiene que llegar a la máquina de alguna forma, y el camino es el argv de `musubi:pantalla`. **Toda superficie que muestre un argv la tapa**, y la función vive en el dominio justamente para que ninguna superficie nueva se la olvide (`internal/fleet/cronologia.go:72`). Y ya no queda en la tabla: **se tapa en la MISMA transacción que la entrega** —desde ese instante la fila no la necesita— y también al vencer sin haberse entregado (`internal/memory/comandos.go:151`, `internal/memory/comandos.go:191`). Las salidas de los comandos se podan por retención, sin borrar la fila (`internal/memory/comandos.go:322`). La ventana que queda está declarada abajo |
| **Relay RustDesk abierto** (Musubi no transporta video) | Musubi decide QUIÉN mira QUÉ pantalla y lo audita; el video va directo entre los dos clientes (`internal/mcp/methods_pantalla.go:5`). El relay **exige la clave**: sin `-k`, cualquier cliente que apunte a esa dirección se registra y el relay propio queda tan abierto como el público, con el agravante de que uno cree que no lo está (`deploy/rustdesk/compose.yml:46`). El `rustdesk_id` es público —sin la contraseña de sesión no abre nada— y aun así se muestra sólo a quien tiene `screen` (`internal/mcp/fleet_http.go:68`, `internal/mcp/methods_fleet.go:285`); dos máquinas que dicen ser el mismo id **cierran la pantalla** en vez de conectarse a la moneda al aire (`internal/mcp/methods_pantalla.go:213`), y que ese id se MOVIÓ queda registrado (`internal/fleet/device.go:147`). La contraseña se acuña con `crypto/rand`, dura 30 minutos por default y 4 horas como techo (`internal/fleet/pantalla.go:112`, `internal/fleet/pantalla.go:25`) |
| **Una política de auto-heal como puerta lateral** | Una política **no tiene autoridad propia**: toda su capacidad es la de un principal declarado, y pasa por las MISMAS dos compuertas que una persona (`internal/fleet/politica.go:114`, `internal/mcp/politicas.go:201`, `internal/mcp/politicas.go:212`). Si ese principal desaparece de `principals.yaml`, la política queda inerte y lo avisa. El sondeo de fondo corre sin principal pero **sólo escribe**: todo camino de LECTURA sigue pasando por la compuerta con la credencial de quien pregunta (`internal/mcp/scheduler_flota.go:291`) |
| **`/metrics` como fuga de la telemetría de la flota** | Detrás de auth SIEMPRE que la auth esté activa —antes gateaba sólo por el token legacy, así que en el setup multi-tenant caía ABIERTO— y el principal se CAPTURA, no se descarta: qué máquinas se ven depende de quién scrapea (`internal/mcp/http.go:264`). El filtro es la misma `PuedeSobreDevice` de las tools (`internal/mcp/fleet_prometheus.go:103`) |

## Riesgos residuales del plano de flota (lo que NO está mitigado)

Sin adorno, porque un modelo de amenazas que sólo enumera lo resuelto no sirve para decidir nada.
Ninguna de éstas está cerrada hoy —algunas están a medias y se dice cuál mitad— y el plan de
escala las toma en olas posteriores. Hasta entonces son riesgo ASUMIDO, que es distinto de
riesgo ausente.

- **La identidad de las personas no vence y no tiene segundo factor.** `principals.yaml` guarda el
  SHA-256 de un token y nada más: no hay fecha de expiración, ni rotación forzada, ni MFA
  (`internal/mcp/principals.go:58`). Un token filtrado vale hasta que alguien lo note y edite el
  archivo — la recarga en caliente hace que ESE corte sea inmediato
  (`internal/mcp/principals_reload.go:12`), que es distinto de que el token caduque solo.
- **El agente no está firmado, y nadie verifica qué binario corre.** No hay firma de código ni
  verificación de integridad del agente; el inventario muestra la versión que el agente DECLARA,
  que es exactamente lo que un agente comprometido controla. Y el auto-update del cuerpo por la
  malla, cuando se habilita, sirve manifest y binarios **sin auth**, con el tailnet como única
  frontera (`internal/mcp/http.go:253`).
- **El token de dispositivo no rota.** Se acuña una vez al enrolar y vive lo que viva la máquina;
  la única herramienta es revocar y volver a enrolar, que es un viaje a la máquina
  (`internal/memory/devices.go:39`, `internal/memory/devices.go:264`). En una flota de dos mil
  equipos eso no escala.
- **No hay postura del endpoint.** Nada mide si la máquina tiene el disco cifrado, antivirus al
  día, parches, o si quien la usa es administrador local. La flota sabe cuánta CPU usa una máquina
  y no sabe si es confiable: hoy la respuesta a «¿la dejo entrar?» es «tiene token».
- **Cerebro comprometido = ejecución en toda la flota.** No recupera credenciales (ver la tabla),
  pero puede encolar comandos en cada máquina y los agentes los van a levantar y correr: el token
  del dispositivo autentica al AGENTE contra el cerebro, no al cerebro contra el agente. No hay
  firma de comandos ni aprobación fuera de banda. Root en el cerebro es root en la flota.
- **El gate por device del acceso híbrido no existe todavía.** El proposal declara relay público
  «SÓLO para devices marcados, con su propio gate» (`specs/control-de-flota/proposal.md:75`); en
  código no hay ninguna marca por máquina — al relay lo alcanza cualquiera que tenga su clave.
- **El consentimiento gobierna los CUATRO caminos, y `pide` ya se honra en los cuatro.** *(Este
  punto estuvo vencido TRES veces: en tres afirmaciones hasta el 2026-09-04; otra vez el
  2026-09-05 cuando A85 y A86 cambiaron el comportamiento que describía; y una tercera el mismo día
  cuando se descubrió que decía «tres caminos» y había un cuarto que no consultaba el eje. Lo que
  decía antes está resumido al final, porque su forma importa más que su contenido.)*

  El eje —`libre` < `avisa` < `pide` < `prohibido`— **lo consultan los cuatro caminos**:
  `musubi_fleet_exec` (`internal/mcp/methods_exec.go:60`), `musubi_fleet_shell`
  (`internal/mcp/methods_shell.go:75` y `:116`), `musubi_fleet_screen`
  (`internal/mcp/methods_pantalla.go:92`, `:162` y `:178`) y **el auto-heal**
  (`internal/mcp/politicas.go`, tercera compuerta de `actuarSiCorresponde`, desde **A91**).
  `prohibido` cierra los cuatro y `avisa` notifica en los cuatro.

  **EL CUARTO FALTABA, Y ES EL QUE NADIE MIRA EJECUTARSE.** Medido el 2026-09-05 corriendo el
  barrido real: `libre`, `avisa`, `pide` y `prohibido` daban los cuatro «1 comando encolado», y bajo
  `avisa` no se encolaba ningún aviso. El comentario de `actuarSiCorresponde` decía «LAS DOS
  COMPUERTAS, LAS MISMAS QUE PARA UNA PERSONA» mientras una persona ya pasaba tres. Alcanzaba con
  una política con `devices: ["*"]` para que el cerebro ejecutara en una máquina marcada
  `prohibido`. Es la misma forma que A83 —la shell como tercer camino sin el aviso— y las dos veces
  el camino nuevo llegó en un ARCHIVO nuevo: por eso la guarda de hoy no cuenta caminos sino que
  exige que todo archivo que llame a `EncolarComando` llame también a `ConsentimientoEfectivo`
  (`TestTodoArchivoQueLeHaceHacerAlgoAUnaMaquinaConoceElEjeDeConsentimiento`, por AST y no por
  texto: la primera versión miraba el texto y un comentario la dejaba en verde).

  **`pide` ERA la excepción, y fallaba de la peor manera: parecía honrado y no lo estaba.**
  `AvisaAlUsuario()` devuelve true también para `pide` —su propio doc lo dice, «preguntar es avisar
  y algo más»— así que los caminos que sólo tenían ramas de `avisa` le mandaban a la persona una
  NOTIFICACIÓN que no podía contestar mientras el operador ya estaba adentro. El grado promete
  «tiene que aceptar; sin respuesta, no hay sesión», y eso no pasaba.

  **CERRADO EN LOS TRES CAMINOS, y cada uno como corresponde a su forma:**

  - `musubi_fleet_screen` siempre lo implementó (`methods_pantalla.go:178`).
  - `musubi_fleet_shell` lo implementa desde **A85** (2026-09-04): flujo de dos llamadas, la sesión
    queda en `esperando_permiso` SIN tocar SSH y el prompt se entrega recién si dijeron que sí.
  - `musubi_fleet_exec` **se endurece a `prohibido`** desde **A86** (2026-09-05, decisión de gio).
    No pregunta, y ésa es la diferencia que importa: una shell es una SESIÓN y tiene dónde esperar
    la respuesta; un exec es una orden suelta y no. Preguntar por comando metería un diálogo de
    hasta minuto y medio en cada orden de una ráfaga. Endurecer no inventa comportamiento nuevo —es
    la misma regla que el dominio ya aplica cuando no hay a quién preguntarle— y sesga el error
    hacia el lado que SE NOTA: bloquear de más rompe el auto-heal y alguien lo ve; ejecutar sin
    preguntar no se nota nunca. **Lo que se paga está dicho**: una máquina en `pide` no recibe
    auto-heal, y la salida es de su dueño —bajarla a `avisa`—, no del código.
  - **el auto-heal** también, desde **A91** (2026-09-05, decisión de gio), con el mismo criterio y
    por la misma razón elevada: un `exec` a mano al menos tiene una persona del otro lado que puede
    reintentar; un barrido corre solo, así que actuar bajo `pide` sería romper la promesa sin nadie
    que lo note.

  **Y LA PREMISA DE ESA DECISIÓN NO ERA CIERTA CUANDO SE TOMÓ, lo que vale más que el arreglo.** El
  argumento fue «bloquear de más SE NOTA: el auto-heal deja de actuar y alguien lo ve». Medido el
  2026-09-05: no existía ninguna métrica ni alerta de un rechazo por consentimiento, así que lo
  único que avisaba era el texto de un rechazo RPC pedido a mano — y el auto-heal, además, NO dejaba
  de actuar. La decisión pudo ser la correcta; el mecanismo que la justificaba no existía. Ahora sí:
  `musubi_fleet_policy_actions_total{result=~"consentimiento_.*"}` cuenta `prohibido` y `pide`
  SEPARADOS —el segundo mide cuánto se ganaría implementando la pregunta por política— y la alerta
  `PoliticaFrenadaPorConsentimiento` los saca a la superficie con `for: 6h`, porque una política
  frenada es un ESTADO y no un evento.

  La matriz **caminos × grados** (`internal/mcp/consentimiento_matriz_test.go`) ejerce las doce
  celdas y las deja escritas; la asimetría exec/shell vive ahí y un cambio de comportamiento se ve
  en esa tabla antes que en producción.

  **LO QUE CONVIENE NO OLVIDAR DE CÓMO ESTUVO ROTO**, porque la forma se repite y el contenido ya no
  aplica: (1) el texto original remitía a A75 como «decisión abierta, no bug», y A75 se había
  CERRADO entero — un agujero descrito correctamente, etiquetado como decidido y apuntando a un cabo
  cerrado es la forma más eficiente de que nadie vaya a mirarlo, porque quien lo lee concluye que
  alguien ya lo pensó. (2) La guarda que existía recorría los TRES CAMINOS y daba tranquilidad por
  haber generalizado, pero fijaba `avisa` en las tres filas y nunca probaba `pide`: generalizaba
  sobre una dimensión de dos, y una tabla que cubre la mitad de una matriz se siente igual de
  completa que una que la cubre entera.

  **Y ADEMÁS LAS DOS REFERENCIAS DE LÍNEA HABÍAN VENCIDO** —`:81` apunta hoy a un comentario y
  `:148` a la mitad de una frase—, que es el modo de falla propio de citar líneas en prosa: no rompe
  nada, no lo avisa nadie, y manda a leer otra cosa con cara de precisión.
- **No se graba nada de una sesión.** Ni pantalla ni shell: queda que HUBO acceso —quién, a qué
  máquina, cuándo y por cuánto—, no qué se hizo adentro (`internal/fleet/shell.go:9`). Es una
  decisión legal antes que técnica y todavía no tiene dueño (A14 y B10 en `ABIERTO.md`); mientras
  no la tenga, la auditoría de un exec es exacta y la de una shell es un rectángulo de tiempo.
- **La contraseña de pantalla sigue teniendo una ventana en claro.** Se tapa al entregarse o al
  vencer, y las dos cosas pasan cuando la máquina PIDE su cola: una máquina que deja de latir se
  queda con la fila cruda hasta que vuelva (`internal/memory/comandos.go:107`). Para una máquina
  viva la ventana es de minutos; para una apagada, indefinida. Quien pueda leer el `.db` —o un
  backup tomado en esa ventana— la lee sin pasar por ninguna compuerta.
- **El respaldo de la identidad del relay es local-only.** `data/id_ed25519` tiene copia en el
  MISMO disco (`deploy/rustdesk/preparar.sh:126`): contra perder el host no protege nada, y
  perderla obliga a reconfigurar a mano todos los clientes de la flota (A37 en `ABIERTO.md`).
