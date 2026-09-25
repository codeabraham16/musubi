# La sesión con hogar

**Estado: plan, sin construir. Escrito el 2026-09-24.**

Qué es esto: el plan para que una sesión de trabajo del cuerpo deje de vivir en una sola PC. Nace de
un pedido explícito del usuario y de dos incidentes concretos, no de una idea de arquitectura.

---

## 1. Lo que se pide, y por qué

Dicho por el usuario, y conviene conservar sus dos ejemplos porque son el criterio de éxito:

> «Esta PC, que es mi principal, se apaga, y no puedo confiar en que siempre estará activa. Ayer
> estaba trabajando en un PDF importante y quedó en esta PC, se me apagó, y murió el PDF aquí
> encerrado.»

> «Si yo mando un comando desde aquí y se me apaga en medio de la respuesta, poder ver eso desde la
> otra PC, porque aquí es como un remoto.»

Y la forma completa de la idea, que es más grande que el síntoma:

> «Vamos a pasar de trabajar en estas terminales locales a máquinas en el cerebro trabajando
> siempre. Cada persona tiene su propio cuerpo en el cerebro y cada uno puede ejecutar estilo una PC
> remota en el cerebro.»

**El problema real no es pérdida de datos: es ENCIERRO.** El PDF no se corrompió — existía, pero
estaba del otro lado de una máquina apagada. La propiedad a perseguir no es «que no se pierda», es
**que nada viva únicamente en una máquina, en ningún momento, ni siquiera mientras se está
haciendo**. Un sync cada cinco minutos no habría salvado ese PDF: se apagó antes de los cinco
minutos.

De ahí sale la única regla de diseño que gobierna todo lo demás: **nada espera a «terminar» para
viajar.**

⚠ Y una distinción que hay que tener a la vista desde el principio, porque cuestan órdenes de
magnitud distintos:

| | qué hace falta |
|---|---|
| **Ver desde otra PC lo que pasó hasta el corte** | espejar la salida mientras se produce |
| **Que el comando SIGA corriendo tras el corte** | que el proceso no viva en la PC que se apaga |

La primera es cimiento de la segunda. La segunda es el salto que el usuario describe.

---

## 2. Lo que YA está decidido, y no se vuelve a discutir

Esto no es un lienzo en blanco. Hay una decisión de arquitectura tomada el 2026-09-07 con 15
agentes y escépticos, observación `6312db2d-422d-4d99-9786-def4fa39f8cc`
(`arquitectura/un-enlace-no-dos-relojes`), y **el pedido del usuario es exactamente su fase 5**.

**Las nueve fases, en orden de dependencia:** F0 identidad de build · F1 los procesos dejan de colgar
de una ventana · F2 un solo camino de despliegue · F3 el sobre y el cursor · F4 lápidas +
anti-entropía · **F5 el empuje** · F6 outbox tipado · F7 el pizarrón con hogar · F8 instrumentos. En
serie, la estimación de esa decisión es de **14 a 21 meses**.

Tres cosas de ahí que este plan hereda y no discute:

1. **F5 va tarde a propósito.** Textual: «un empuje sobre un pull que no sabe reanudar por época ni
   reparar por anti-entropía convierte cada corte en pérdida silenciosa. **MEDIO EMPUJE ES PEOR QUE
   NINGUNO**». O sea: hacer «tiempo real» primero no acelera el plan, lo vuelve peligroso —
   silencioso y confiable en apariencia.
2. **El estado mutable y exclusivo NO se replica: TIENE HOGAR.** Textual: «replicar work_units por
   el outbox sería un DEFECTO, no un arreglo — el outbox es last-writer-wins y un claim es exclusión
   mutua; dos réplicas dan dos dueños convencidos de serlo». **Una sesión abierta en dos PCs es
   exactamente eso.** Por lo tanto la sesión no se replica: vive en el central y se *reclama*.
   Esto no contradice al usuario — **lo formaliza**: «el original es el del cerebro» se construye
   como *hogar*, no como réplica.
3. **La regla de reparto (dirección del usuario, 2026-08-06):** «si el cuerpo puede hacer algo que la
   terminal no puede, ese algo está en el lugar equivocado». **Esta capacidad se construye en el
   CEREBRO**, y el cuerpo la consume. Construirla dentro de `musubi-body` sería el error más caro del
   plan.

### Lo que está descartado a propósito

Es lo primero que se re-propone cuando alguien lee «réplica en vivo», así que va escrito:
**nada de Raft, NATS, Postgres, Litestream, cr-sqlite, rqlite, Temporal ni DBOS.** El argumento está
medido sobre *esta* malla: «somos 5 máquinas donde una laptop se apaga y hay una Raspberry de
fichaje; un quórum de 3 deja al nodo aislado sin poder comprometer NI UNA escritura, y eso viola
local-first más fuerte que un servicio externo».

**Un original remoto con réplica local es, literalmente, un sistema de consenso.** Por eso la frase
«el original es el del cerebro y la copia es el local», tomada al pie de la letra, choca con una
decisión ya tomada con evidencia. Lo que sobrevive de la intención —y es lo que el usuario quiere de
verdad— es el **hogar**: el central tiene la última palabra sobre quién es el dueño de una sesión, y
cada máquina trabaja local y sin red cuando hace falta.

---

## 3. Lo que se midió el 2026-09-24

Cuatro relevamientos en paralelo, cada hallazgo verificado por un segundo agente que intentó
tumbarlo. Lo que importa para este plan:

### El cuerpo no tiene qué sincronizar todavía

- **Los ocho paquetes `*state` del cuerpo no persisten NADA.** Una sola operación de archivo en los
  ocho, y es una lectura (`internal/chatstate/format.go:37`). La sesión vive en RAM y muere al
  cerrar. **No hay una «copia local» que replicar: hay que crear el primer original.**
- **La persistencia multi-terminal YA FUE CONSTRUIDA — en la interfaz vieja.** `body-rs/src/chatlog.rs`
  guarda por proyecto+sesión: transcript (tope 500 mensajes), índice de terminales, geometría, y
  `SessionMeta { id, title, claude_id, detached }` — con escritura atómica temp+rename. En disco hoy:
  11 archivos, **288 KB**. La interfaz nueva no lo tiene: `grep -rn "chatlog" gpui-spike/src/` → 0.
  Es el patrón de siempre en este proyecto: se construyó, se migró la UI, y quedó apagado.
- **El transcript de verdad no es del cuerpo, es del CLI `claude`:** `~/.claude/projects` pesa
  **4,5 GB**, y el `.jsonl` de una sola sesión pesa **857 MB** y crece ~9 KB/min. El cuerpo sólo
  guarda un puntero (`SessionID`, que alimenta `--resume`). **Replicar «todo» en vivo, si «todo»
  incluye el transcript crudo, no es replicación: es una mudanza.** Lo que sí viaja es el *frame*
  que el puente ya emite —lo que la pantalla pinta—, ~140 KB por terminal. Cuatro órdenes de
  magnitud menos.

### Qué puede viajar y qué no

| pieza | viaja | por qué |
|---|---|---|
| el frame del Hilo (mensajes, estado, `session_id`) | **sí** | es el trabajo |
| `prefs.json` (gate, modo, modelo, esfuerzo) | **sí** | retomar con otro modelo cambia la sesión |
| `workspace.json` (proyecto abierto) | **traducido** | hoy guarda rutas absolutas |
| `winmain.json`, `g-*.json` (geometría) | **no** | píxeles físicos: `[-8,-8,1600,860]` |
| `secrets/central_token.bin` | **no puede** | cifrado con DPAPI, ámbito del usuario |

**La regla que sale de ahí, y que conviene repetir: lo que describe el TRABAJO viaja; lo que describe
el VIDRIO se queda.**

⚠ `workspace.json` guarda la ruta absoluta tal cual, y ya se rompe dentro de una sola máquina:
conviven `C:\Proyectos\Musubi`, `D:\Proyectos\Musubi` y `d:\Proyectos\Musubi` como tres proyectos
distintos, siendo el mismo junction. **Una sesión tiene que anclarse a un identificador de proyecto
—el `project_id` que el central ya maneja—, no a una ruta.** Es el primer contrato a definir, antes
de escribir una línea de sync.

### El riel que ya existe

- **El outbox es durable y está encendido**: transaccional, con lease, backoff e idempotencia, y
  reintenta para siempre. La mitad saliente del plan no hay que inventarla.
- **Latencia medida: ~25 s de subida y 0-13 s de bajada.** El techo lo pone el *ticker*, no la red:
  bajar a segundos es cambiar el disparo, no rediseñar.
- ✅ **El eco está cerrado en el código** (2026-09-24, `fix/el-eco-del-sync`). Falta encenderlo:
  instalar el binario y reiniciar el daemon. Ver «El eco, medido y cerrado» más abajo.
- 🔴 **El riel de bajada tiene un agujero irreversible, y NO es el que decía esta línea.** Lo que había
  escrito acá («el central es un nodo terminal, no sincroniza hacia afuera, 1.674 observaciones que
  nadie puede bajar») era falso en la causa. El defecto real es que **el filtro de proyecto salta
  filas y el cursor no retrocede**: 64 filas que el central sirve hoy son inalcanzables para siempre en
  esta PC, 980 en la laptop, 2.558 en la base del CRM. Ver «El riel de bajada, medido de punta a
  punta» más abajo.

### El eco, medido y cerrado

La nota vieja decía «el 30 %», y era una estimación de otra pasada. Medido de nuevo el 2026-09-24
sobre `.musubi/memory.db` de esta PC, con lo que se puede **probar**:

| | |
|---|---|
| filas del outbox | 3.307 |
| de autor que esta máquina **no pudo** estampar | **943 (28,5 %)** — `gio` 617, `davantis` 260, `davantis-2` 66 |
| de autor vacío | 1.902 — **no se puede atribuir**, ni a eco ni a envío real |

O sea: el 28,5 % es piso probado, no el total. Los 1.902 de autor vacío quedan sin decidir a
propósito; inventarles un origen sería peor que no saberlo.

**La causa.** `IngestShared` no encola lo que baja — eso es cierto y tiene test. Lo que nadie
preguntó es qué pasa en la **apertura siguiente de la base**: ahí corre `BackfillOutbox`, que siembra
una fila `pending` por cada `shared` **sin** fila de outbox, y lo bajado del central es exactamente
eso. La red de seguridad del backfill deshacía el anti-loop del ingest, una apertura después. El test
del anti-loop exigía *cero filas de outbox*, que es justo la condición que el backfill interpreta
como «shared que falta encolar»: medía un proxy, no la propiedad.

**El daño no era sólo tráfico.** `IngestShared` redacta el contenido **otra vez** y recalcula gist,
hash y tokens. Si esa copia difiere de la del central, el eco la sube y **pisa el original**; otro
nodo baja lo pisado, lo redacta de nuevo, y la degradación avanza sola.

**El arreglo.** Un estado terminal nuevo, `espejo`: el sello de que la fila entró por el sync
entrante, escrito en la misma transacción que el UPSERT. Con el sello, el `NOT EXISTS` del backfill
no la ve y el claim no la reclama. No es una mordaza — una edición local posterior cambia el
`content_hash` y la devuelve a `pending` como a cualquier otra—, no pisa una `pending` local que
todavía no salió, y `musubi_sync_status` **reporta el número**, porque un arreglo que sólo deja de
hacer algo es invisible.

**Lo que NO se tocó, y es decisión:** el UPSERT del ingest sigue pisando el contenido local con el
del central (último que escribe gana) — ése es el diseño declarado del enlace. Y las 943 filas ya
ecoadas quedan en `sent`: es terminal, no vuelven a subir.

### El riel de bajada, medido de punta a punta

Este plan apoya los escalones 2 y 3 en una suposición que nunca se había verificado: que si la sesión
vive en el cerebro, otra máquina la puede traer. Medido el 2026-09-24 sobre la base **real** del
central (`/home/musubi/musubi-brain/.musubi/memory.db`, 238 MB — ⚠️ hay una **base señuelo** de 466 KB
congelada en `/home/musubi/.musubi/`, medirla es concluir cualquier cosa):

| | |
|---|---|
| observaciones en el central | 5.068 |
| `scope = 'shared'` | 3.392 |
| `scope = 'local'` | 1.676 |
| shared que un pull **sí** entrega (cursor 0) | **3.069** |
| shared que ningún pull entrega jamás | 323 — y las 323 son `archived` o `superseded`, o sea **correctamente** excluidas |
| shared con `sync_seq <= 0` | **0** |
| cursor de bajada de esta PC | **10.969**, contra un `max(sync_seq)` de 10.967 en el central |

La hipótesis que valía la pena descartar era el `sync_seq = 0` — el propio código advierte que una fila
así «nunca es > cursor ⇒ INVISIBLE para siempre» —: **cero filas**. Ésa está cerrada.

🔴 **Pero el riel NO está sano, y la primera versión de esta sección decía que sí. La prueba era
circular.** Comparar el cursor contra `max(sync_seq)` del central y verlo al día no demuestra que bajó
lo que había en el camino: demuestra que el cursor llegó arriba, que es exactamente el síntoma.

**El filtro de proyecto no oculta filas: las SALTA, y el salto es irreversible.**

El acotamiento por tenant entra al mismo `WHERE` que `sync_seq > ?` y el `LIMIT` se aplica después, así
que el filtro no acorta la página: la corre hacia arriba por encima de las filas ajenas. El
`next_cursor` se calcula con el máximo de las filas **que se devolvieron** —ya filtradas—, así que lo
descartado queda debajo del cursor sin haberse entregado. El cliente lo adopta, y `AvanzarCursorBajada`
no retrocede nunca (`WHERE ... < ...`). Nada lo rebobina: `musubi_sync_requeue` toca el dead-letter del
*outbox*, no la bajada. La única salida es editar `meta` a mano.

**El daño se cobra cuando la credencial se ensancha.** Con un token `read:own` la fila ajena no
corresponde y no falta. Al pasar a `read:all` el filtro desaparece, pero el cursor ya está arriba: esa
historia no vuelve nunca.

Medido cruzando los ids del universo pullable del central contra la base de esta PC:

| | |
|---|---|
| pullable en el central | 3.073 |
| presentes acá | 3.009 |
| **ausentes** | **64**, todas con `sync_seq` ≤ cursor (10.990); **0** por encima |
| `altura` bajo seq 855 | **61 de 61 ausentes** — y **0 de 643** por encima |
| `last-chaos` | 1 de 1 ausente |

La prueba de que es el cursor y no un borrado es el **entrelazado**: seq 709 presente, 711–717
ausentes, **718 presente**, 719 y 721 ausentes, 722 presente. `sync_seq` se asigna `MAX+1` al insertar,
así que ese orden es el de llegada: cuando el pull entregó la 718, las 711–717 ya existían y no
vinieron. (Las 2 de `musubi` se cuentan aparte: pueden ser un borrado en duro local.)

**Y esto pega justo en el escalón 4.** «Una máquina por persona» significa que cada uno se enrola con
su token acotado. Bajo este defecto, el espejo de cada máquina nueva queda permanentemente incompleto
para todo lo que ya existía cuando su token era angosto. Medido en otras bases por la misma pasada: la
laptop no puede bajar **980 de 3.071** (32 %), y la base del CRM tiene **2.558** fuera de alcance.

⚠️ **De paso, el cabezal de `internal/memory/inboundsync.go` estaba viejo**: documentaba la limitación
del `rowid`, ya cerrada. Se reescribió con esta limitación, que es la que está abierta.

**Qué eran entonces las «1.674».** Son las 1.676 filas con `scope = 'local'`, un tercio de la memoria
del central. No es que el central no sincronice: es que `local` **por diseño no se espeja nunca**. Se
reparten en dos grupos muy distintos:

- **1.648 humanas, `quarantined = 0`** — se **pueden leer** consultando el central (verificado: una
  `design-corpus/...` volvió en una búsqueda contra el central), pero no bajan a ninguna base local.
  El grueso es el corpus del motor de diseño (1.371 de autor `destilador`, proyecto `musubi-design`,
  de agosto) y 275 `ingested/*`. Contraprueba desde la otra punta: en la base de esta PC hay **0**
  filas de autor `destilador` y **0** `ingested/*`. Y ninguna de las 1.676 tiene gemela compartida por
  `content_hash`: es memoria **única**, no copias.
- **28 propuestas de LLM en cuarentena** (`provenance = llm:*`, `quarantined = 1`) — y esto **no es un
  defecto, es la regla funcionando**: `musubi_propose_observation` las deja invisibles al recall hasta
  que alguien las corrobore, y a propósito no viajan. Dos de ellas son de hoy y son notas de este
  mismo trabajo.

**La decisión que esto destapa, y es del usuario, no del código:** que las 1.648 sean online-only es
consistente con `local`, pero significa que *no existen* sin el central y nunca entran al recall
rápido de una máquina. Promoverlas a `shared` las haría bajar a **todas** las máquinas — disco y
tráfico en cada una. Para las sesiones del plan no cambia nada (van a nacer `shared`, porque el
central tiene `team_mode: true`); para el corpus de diseño y los documentos ingeridos es una
elección de arquitectura que conviene tomar a propósito y no por omisión.

### Los canales en vivo que existen

Nueve canales, ningún websocket. Todo el tráfico normal es la máquina preguntando. Lo que sí existe:

- **SSE `/api/stream` en el central** — empuje de verdad, ya construido y probado… pero termina en el
  panel web, y **el cuerpo no lo consume**.
- **Long-poll de shell, 25 s** — atraviesa NAT sin abrir puertos en la máquina. Es el molde para
  cualquier canal nuevo.
- **El latido de flota, 30 s** — el central *ya puede* empujar y ejecutar argv en esta PC, hoy. Sirve
  para mandar «abrí esta sesión» sin escribir transporte nuevo; no sirve para llamarlo tiempo real.
- **`--resume <session_id>` ya funciona** y se usa en cada cambio de modelo. La mitad cara de
  «retomar el hilo» está construida: lo que falta es transporte de estado, no lógica.

### El techo del server

12 hilos (Xeon E-2136), **15 GB de RAM con ~6 disponibles**, carga 2,2 de 12. `/` al 76 % (18 GB
libres) y `/home` con 92 GB. Traducción para «una máquina por persona»: **CPU sobra, la RAM es el
cuello, y los targets de compilación no entran en `/`**. Alcanza para máquinas de *trabajo* —editar,
correr comandos, git, el cerebro, builds livianos—, no para que dos personas compilen `gpui` a la
vez (eso solo pide varios GB y 18 minutos). Decisión derivada: lo que quema CPU sigue yendo al
runner, no a la máquina de cada persona.

---

## 4. La escalera

Cada escalón sirve solo. El orden no es de dificultad: es de dependencia, y el primero es el que
resuelve el dolor que originó el pedido.

### Paso 0 — Que la sesión exista ✅ *hecho, en `main` del cuerpo*

La interfaz nueva recupera lo que la vieja ya tenía: el Hilo se persiste, con su índice de terminales
y su `claude_id`, anclado a un **identificador de proyecto** en vez de a una ruta absoluta.

Sin esto no hay nada que sincronizar. Es portar `chatlog.rs` a `gpui-spike` con el contrato corregido.

**Entregado el 2026-09-24** (PR #8 del cuerpo, `main` verde en la corrida 87): `sesion.rs` persiste
mensajes, el `session_id` y las preferencias por proyecto; `bridge chat -resume` —que existía del lado
Go y esta interfaz nunca pasaba— ya se usa, así que un arranque continúa el hilo en vez de abrir uno
nuevo; el guardado va con freno de 1,5 s y el pendiente se **arrastra** en vez de descartarse, porque
el último frame de un turno es justo el que se pierde cuando la máquina se apaga. La geometría **no**
se persiste: son píxeles físicos y en otra pantalla quedan fuera de cuadro.

Y el vacío **avisa** cuando el identificador sale del nombre de la carpeta, porque entonces la sesión
puede no encontrarse desde otra máquina.

### Paso 1 — Que no viva sólo acá ← *el que salva el PDF*

Lo persistido sale hacia el central **al ocurrir**, por el riel que ya existe y es durable. No es
empuje, no es tiempo real: es escritura pasante. Desde otra máquina se puede **leer** en qué quedó
una sesión, incluso si la PC de origen está apagada.

Prerequisito duro: **cerrar el eco** antes de subir volumen. ✅ Cerrado en el código el 2026-09-24
(`fix/el-eco-del-sync`); queda **encenderlo** — instalar el binario y reiniciar el daemon, que es
decisión del usuario porque reinicia el cerebro de esta máquina.

Cómo se comprueba que quedó encendido, y no sólo construido: `musubi_sync_status` tiene que empezar a
reportar un número de **espejo** mayor a cero después del primer pull. Si sigue en cero con el binario
nuevo corriendo, el sello no se está poniendo.

### Paso 2 — Retomar en otra PC

Con el frame y el `session_id` en el central, abrir el cuerpo en la laptop y seguir. `--resume` ya
existe; lo que se agrega es que el id y el frame se puedan traer.

🔴 **Y este escalón SÍ tiene que arreglar transporte, al revés de lo que decía la primera versión de
esta línea.** El pull entrega 3.069 de 3.392 shared y no hay ninguna fila con `sync_seq <= 0` —eso está
bien—, pero el cursor de bajada **salta** lo que el filtro de proyecto descarta y no retrocede nunca
(ver «El riel de bajada, medido de punta a punta»). Para este plan la consecuencia es directa: una
sesión que nazca mientras el token de la otra máquina esté acotado **no se va a poder traer después**.

✅ **CONSTRUIDO el 2026-09-25, por el camino 1.** De las tres opciones que estaban escritas acá se
eligió **el cursor por alcance**, y las otras dos quedan descartadas con su motivo:

1. ✅ **Cursor por alcance, no por base.** El central **declara** en cada pull el recorte con el que
   sirvió (campo `alcance`), el cliente lo guarda en `sync:inbound_alcance`, y cuando cambia reinicia
   el cursor a cero en la misma transacción. Se eligió porque es el único que **repara lo ya roto**: una
   base con un cursor viejo y sin la clave registrada cuenta como «alcance desconocido» y rebobina una
   vez, así que las 64 filas de esta PC y las 980 de la laptop vuelven solas al actualizar el binario.
2. ⛔ **`next_cursor` desde lo ESCANEADO en vez de lo devuelto** — descartado: arregla el paginado pero
   **no** el ensanchamiento, que es el caso que duele, y no repara nada de lo ya perdido.
3. ⛔ **Un rebobinado explícito** (`musubi_sync_rewind`) — descartado: deja la reparación a que alguien
   se acuerde de correrla, que es precisamente cómo se llegó hasta acá.

Tres decisiones del diseño que conviene no revisitar sin leer el porqué:

- **El alcance lo declara el CENTRAL, no lo deduce el cliente.** El recorte lo decide la credencial del
  lado del central; una llamada aparte (`whoami`) podría contestar por otra credencial. Viaja en la
  misma respuesta.
- **Un central viejo no manda el campo, y el vacío se lee como «no sé», no como «cambió».** Tratarlo al
  revés reiniciaría el corpus entero en cada tick contra ese central.
- **Federado y proyecto vacío son la MISMA huella**, porque `scopeClause` no filtra en ninguno de los
  dos casos. La huella tiene que cambiar cuando cambia el **filtro**, no cuando cambia la etiqueta de
  la credencial.

**El costo, que se asume:** el primer tick tras actualizar re-baja el corpus una vez. La ingesta es
idempotente y con el sello `espejo` no rebota, pero sí bumpea el `sync_seq` local de cada fila — en un
nodo que a su vez sirve pulls, sus clientes las van a ver como recién editadas.

### Paso 3 — El hogar y el empuje

La sesión tiene dueño: el central arbitra quién la tiene abierta, y avisa por empuje en vez de que
las máquinas pregunten. **Acá es donde este plan se encuentra con F5 y F7 de «Un enlace, no dos
relojes», y hereda su condición: no antes del sobre, el cursor y la anti-entropía.**

### Paso 4 — La máquina por persona

El salto que describe el usuario. Requiere, además de todo lo anterior, la conversación de capacidad
del punto 3 y la separación por identidad.

---

## 5. Lo que necesita una decisión del usuario

1. **Por dónde arrancar.** Los pasos 0 y 1 son semanas y ya resuelven el PDF encerrado. El paso 4 es
   el salto completo y vive detrás de una arquitectura estimada en 14-21 meses en serie. No son
   excluyentes; son dos horizontes distintos y conviene decir cuál se financia primero.
2. **Qué se lleva una sesión.** La propuesta es la regla del punto 3 —el trabajo viaja, el vidrio se
   queda— pero la lista concreta (¿el scroll?, ¿qué terminal estaba enfocada?) es una decisión de
   producto, no técnica.
3. **Qué corre en el cerebro.** Con 6 GB de RAM disponibles, «una PC remota por persona» y «compilar
   ahí» no entran juntas. Hay que elegir qué tipo de trabajo vive allá.

---

## 6. Lo que no hay que volver a proponer

- El stack de consenso distribuido (punto 2). Está descartado con un argumento medido sobre esta
  malla.
- «Tiempo real primero». Medio empuje es peor que ninguno: un empuje sin reanudación por época ni
  reparación convierte cada corte en pérdida silenciosa, que es *exactamente* el modo de falla que
  este plan viene a cerrar.
- Construir la sesión compartida dentro de `musubi-body`. Va en el cerebro, por la regla de reparto.
