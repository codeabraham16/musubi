# Changelog

Todos los cambios notables de Musubi se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [Unreleased]

### Added
- **El impulso entra a la escena principal: una llamada real recorre el árbol de quien la hizo.**
  Hasta ahora los troncos estaban puestos pero quietos; el pulso sólo existía en la lente aparte.
  - **La regla que manda: un pulso = un evento real.** `impulsos.mjs` no tiene bucle, temporizador
    ni nada que fabrique luz — `nacer()` es la única puerta y la llama el riel. Verificado **en
    pantalla, no en el buffer**: con la animación pausada, un cuadro sin evento contra el anterior
    da **0 píxeles de diferencia** en el lienzo, y una ráfaga de gio da 215.994.
  - **La atribución se ve**: gio y `davantis-mando-admin` encienden centros separados por **217 px**.
    Un principal desconocido (`pepe-inventado`), uno sin credencial y uno con dueño declarado pero
    sin terminal propia (`b1-adjudicador`) encienden **exactamente cero**: tener dueño decide el
    racimo, no prende una neurona. Ninguno se reparte a dedo.
  - **Las dos capas se separan por brillo**: el trabajo real llega **4,8×** más fuerte que el
    sondeo sobre la misma neurona. En la ventana medida el 98,1 % del tráfico es sondeo, así que
    sin esa separación el 1,9 % que importa quedaría enterrado.
  - Costo medido bajo ráfaga sostenida: **0,50 ms de JS por cuadro** — 3 % del presupuesto de un
    frame a 60 fps, y sólo mientras hay pulsos vivos. El resto del tiempo no se escribe nada.
  - 12 invariantes en `node --test`, **cada uno verificado fallando bajo un sabotaje dirigido**.
    Uno salió VACUO en el primer intento: el vencimiento del pulso estaba defendido en dos lugares
    y el segundo tapaba al sabotaje, así que ahora vive en uno solo.

### Fixed
- **Un evento sin neurona encendía el tronco más grande.** `Number(null)` es `0`, no `NaN`, así
  que coercionar el índice mandaba todo principal sin declarar al tronco 0 — y los racimos van
  ordenados por volumen, o sea que el panel le atribuía la llamada a la terminal que más escribe.
  Lo agarró su propio test antes de llegar a la pantalla.
- **El ámbar de una falla era invisible.** Dos defectos encadenados, los dos medidos en pantalla:
  el aviso tomaba el **máximo** de las fallas y lo dividía por la **suma** de la carga, así que
  ocho fallas apiladas daban 1/8 de aviso; y el ámbar del HUD (`#f5c451`) sobre un medio aditivo
  es casi blanco. La relación azul/rojo del frente no se movía ni un punto entre una ráfaga que
  salía bien y una que fallaba entera: **0,498 contra 0,497**. Ahora el aviso se suma como la carga
  y el frente usa el **mismo tono (41°) a saturación plena**: **0,487 contra 0,089**, un factor 5,5.
  Arreglar sólo el primero llevaba a 0,445 — real, medible y todavía invisible con las dos capturas
  puestas al lado, que es por qué la verificación era mirar y no sólo medir.
- **Las neuronas con dendritas viven DENTRO de la escena principal.** Cada terminal es un tronco
  con su árbol, plantado en el racimo de su persona, y las memorias de esa persona lo orbitan.
  - **12.010 segmentos en UNA draw call**, con el adelgazamiento resuelto en el vertex shader
    (`position.xz *= mix(1.0, aTaper, altura)`). Lo que hace que una rama se lea como dendrita y
    no como un palito es que la punta sea más fina que la base; en canvas 2D eso era una propiedad
    del trazo, acá es geometría — y emitir un cono por segmento serían 12.010 geometrías.
  - Costo medido: **59,9 FPS · p50 16,6 ms** con los árboles puestos, contra 60,3 sin ellos.
  - `dendritas.mjs` es geometría pura y corre en `node --test`: 7 invariantes, cada uno verificado
    fallando bajo un sabotaje. Custodian lo que se vería como un cambio estético y por eso nadie
    miraría — que el árbol sea el MISMO en cada recarga (PRNG semillado, no `Math.random`), que
    adelgace, que el conteo tenga techo, y que la distancia que usa el impulso se mida **a lo
    largo de la rama** y no en línea recta.
  - **El libro mayor y lo sin atribuir NO llevan tronco.** Nadie los firma, así que no hay neurona
    que dibujar; fabricarles una sería inventar un autor. Se ven como lo que son: nubes de puntos.
  - Y un bug que no daba excepción: `disposeMeshes()` limpiaba el bosque y `rebuildMeshes()`
    empieza llamándolo, así que la geometría llegaba VACÍA al constructor de la malla.

### Changed
- **La escena principal agrupa por QUIÉN, no por dominio.** El sujeto del panel cambia: cada
  memoria sigue siendo un punto, pero el racimo y el color pasan a ser la persona que la escribió.
  - **Los racimos ahora existen de verdad.** `layout.mjs` decía «SIN atractores de dominio»: el
    dominio nunca fue una posición, sólo un color. Con 90 dominios eso se veía como parches; con 4
    racimos de 500-1.000 nodos la repulsión gana y queda una esfera pareja (medido: a los 50 s de
    asentado, cero separación). Ahora el centrado tira hacia el ancla del racimo y cada uno tiene
    su propio volumen. Un nodo sin ancla usa el camino de antes, así que la lente código no cambia.
  - **Los dos números que lo gobiernan salen de un banco, no de probar a ojo.** «Holgura» =
    separación entre centros / suma de radios; por debajo de 1 los racimos se pisan. Con el tirón
    solo daba **0,38**, y alejando las anclas hasta el borde llegaba a 0,70 — nunca a 1, porque la
    repulsión está calibrada para llenar el elipsoide. Con volumen propio: **1,26**.
  - **Tres clases, no una.** Sólo 802 de 2.217 memorias (36,2 %) traen `author`. Agrupar por
    persona a secas dejaría el 62 % en una mancha gris. Pero 1.027 de esas huérfanas son
    `git-commit` y `sdd/` — los dos géneros que **escribe el propio motor**, y que Musubi ya llama
    LIBRO MAYOR en `internal/config/config.go:355`. Van a su racimo, declarado, y no compiten entre
    personas. Lo que de verdad no se pudo atribuir (390) también se declara.
  - El género gana sobre el autor: un `git-commit` es el registro del repo aunque la fila diga
    quién lo firmó. Si mandara el autor, un backfill mudaría el racimo entero de golpe.

### Fixed
- **El lag de la lente de personas, medido y con causa.** 11.012 `stroke()` por cuadro —155.068
  llamadas de dibujo por segundo, todas en CPU— daban **13,8 FPS** (p50 69,4 ms). La misma escena
  en WebGL, en la misma GPU, da **60,3 FPS** (p50 16,6 ms). Al mover el agrupamiento por persona a
  la escena WebGL, hereda esos 60.
- **Un contador que mandaba a una acción imposible.** «N eventos sin neurona · falta declarar su
  dueño» contaba también los del spool de esta máquina, que llegan sin credencial: ahí no hay dueño
  que declarar. Medido en 40 s: los 8 que reportaba eran los 8 locales, y los principales sin
  neurona eran cero. Ahora son dos contadores con dos frases.

### Added
- **El censo de actores: quién LLAMA al cerebro, no sólo quién escribe.** El ledger guarda
  `principal` en cada invocación desde el primer día, pero lo único que se podía preguntar era
  «qué tools se usan». Los bots y los servicios —`crm-cabina`, `b1-adjudicador`, `davantis-crm`—
  no aparecían en ninguna vista: no escriben memoria, sólo llaman. Ahora tienen neurona propia.
  - `memory.ActorUsage` agrupa `tool_invocations` por principal, parte las llamadas en sondeo y
    trabajo, y cuenta cuántas **tools distintas** tocó cada uno — que es lo que separa a un poller
    de un agente cuando los dos tienen el mismo total.
  - `GET /api/actores` en el cerebro central, con **la misma auth y la misma tenancy que
    `/api/stream`**: el censo dice quién trabaja, cuándo y a qué ritmo, o sea el patrón de trabajo
    de un equipo. Un principal acotado ve lo suyo.
  - El panel lo **proxea** en `/api/actores` con un cache de 60 s, para que el bearer no baje al
    navegador. **El estado viaja siempre**: `apagado`, `viejo` (un central sin el endpoint, que
    responde 404), `caido` o `sin_permiso` se dibujan distinto de «no hay actores». Un 404 leído
    como lista vacía pintaría un sistema desierto sobre un cerebro trabajando.
  - **Sólo sirve contra el central, y está dicho donde hace falta.** Medido sobre la base local:
    230.682 invocaciones y las 230.682 con `principal` vacío — en stdio no hay credencial que
    atribuir. Por eso no hay camino local: devolvería la lista vacía siempre.
  - En el dibujo, un actor es un **anillo** `◯` y una terminal un **disco** `◉`. El actor no lleva
    árbol dendrítico —una dendrita acá representa memoria escrita y un actor no escribe— sino una
    **corona de radios rectos, uno por cada tool distinta que llama**. Y el anillo va punteado
    cuando la atribución sale de la convención del nombre en vez de una declaración.
  - Un principal declarado **no nace como nodo aparte**: su volumen se le suma a su terminal, que
    es la misma identidad con dos naturalezas. Los que no tienen dueño declarado van a un racimo
    **`(servicios)`** que se ordena SIEMPRE último —no es una persona— y el panel dice cuántos son
    en vez de repartirlos a dedo.
  - **Los dueños de los servicios están DECLARADOS, con su cita.** `crm-cabina` y
    `b1-adjudicador` no eran huérfanos: estaban documentados en la memoria y yo había leído el
    `project_id` vacío de la cabina como falta de información cuando era una decisión de diseño
    («el project_id existe para ATRIBUIR lo que un principal ESCRIBE; una cabina no escribe»).
    La tabla `DUEÑOS` es aparte de `ACTORES` a propósito: una dice de quién ES una credencial,
    la otra dice qué terminal ES. Tener dueño decide el racimo, no enciende una neurona.
  - Lo declarado se dibuja entero y lo inferido del nombre, **punteado**. Hoy quedan tres
    credenciales punteadas (`davantis-musubi-design`, `-lienzo-corpus-reader`,
    `-renaissance-seed`): nadie escribió de quién son, sólo lo sugiere el prefijo.
  - 15 invariantes nuevos entre Go y `node --test`, cada uno verificado fallando bajo un sabotaje
    que ataca lo que ese test declara. Uno de ellos encontró una fuga real: el proxy reenviaba el
    cuerpo de error del central al navegador, y con él el bearer.

- **El panel tiene una tercera lente: `personas`.** Memoria y código dibujan QUÉ sabe el cerebro;
  ésta dibuja **quién lo escribe**. Cada terminal es una neurona con dendritas en 3D, los despachos
  entre ellas son axones dirigidos, y cada persona es un racimo. Se llega con el botón de lente o
  directo por URL con `?lens=personas`, que además le sirve al CRM para enlazar una vista concreta.
  - **La persona sale de `author`**, no de una lista escrita a mano, y las credenciales del mismo
    humano colapsan en una sola: `davantis`, `davantis-admin`, `davantis-mando-admin` y
    `davantis-altura` son una persona, no cuatro.
  - **La terminal sale del texto**, porque no existe como campo — se firma a mano en el encabezado
    del gist. `personas.mjs` es un parser y está tratado como tal: 11 invariantes en `node --test`,
    cada uno verificado fallando bajo un sabotaje que ataca lo que ese test declara.
  - **Firmar no es mencionar, y confundirlos daba respuestas falsas.** La persona de una terminal
    sale de quién **firma** como ella (el gist EMPIEZA con el rol), no de quién la nombra: a
    `ALTURA` la menciona más gio que Gabriel, así que por menciones el racimo se la llevaba gio.
    Con la regla de firma el dato se lee solo: `ALTURA` tiene 80 menciones y **1** firma — es un
    dominio, no una terminal— y `GIO` tiene 91 menciones y **0** firmas, porque es una persona.
  - El render es canvas 2D y no three.js aunque three ya esté en el bundle: lo que hace que una
    rama parezca dendrita es el trazo que adelgaza hacia la punta, y eso en 2D es una propiedad
    del stroke y en 3D es geometría por segmento.
  - **El HUD acompaña a la lente.** La tarjeta de dominios pasa a ser la de **personas** —cada una
    con el mismo color con que se dibujó su racimo—, los KPI cambian de sujeto (terminales,
    despachos, personas) y la guía explica lo que esta lente muestra. Se declaran además los dos
    números que faltaban para no leer una muestra como si fuera un total: cuántos **pares** se
    escriben, y cuántas notas **sin autor** quedaron fuera del reparto.
  - El encuadre se **mide del DOM**: la escena se centra en el rectángulo que el HUD no tapa, y se
    calcula sobre los nodos y sus etiquetas, dejando que las dendritas se salgan del cuadro — que
    es lo que hace que el dibujo se lea frondoso y no tímido.
  - **Se mueve, y cada movimiento dice algo.** Por cada axón viajan luces: una **luz = un despacho**,
    y cuántas viajan a la vez sale de `veces`, o sea de cuántas veces esas dos terminales se
    escribieron. Cada neurona **late** según su **calor** —cuánto se recupera lo que escribió— y la
    que nadie consulta **se queda quieta**, que también es información. El giro lento se **detiene**
    en cuanto hay hover, zoom o desplazamiento: ahí ya estás mirando algo.
    - Se descartó animar por **recencia**: medido sobre el cerebro local, las once terminales
      tienen su nota más nueva a menos de medio día, así que ese canal las pinta a todas igual.
      El calor sí tiene rango real (0 en `REFUTADOR`, 435 en `AUDITOR`).
  - **Zoom hasta 40× para entrar en las neuronas chicas**, que era el punto: `SALA DE MANDO` tiene
    10 notas y a escala 1 es un punto de 3 px. La rueda acerca **hacia el puntero** (si no, el zoom
    tira siempre al centro y las del borde son inalcanzables), `shift+arrastrá` desplaza y el
    **doble click entra** en una neurona con una transición suave; en el vacío, vuelve a la vista
    completa. Lo que hace que eso no cueste un frame es el **LOD por nivel de rama** —cada
    duplicación del zoom habilita un nivel más, y los niveles finos ni se recorren de lejos— más el
    **descarte por pantalla** por neurona antes de proyectar su copa.
  - **El hover explica la terminal**: de quién es, cuántas notas la nombran, cuántas la firman, su
    calor, y a quién le escribe y de quién recibe. Reusa el mismo `#tip` que la lente de memoria.
- **El impulso eléctrico de la lente `personas` sale de invocaciones REALES, y de nada más.** Cada
  llamada a una tool que llega por el riel en vivo enciende **un** frente que recorre las dendritas
  de la neurona que la disparó, desde el soma hacia las puntas.
  - **Un pulso = un evento.** Se eliminó el bucle de luces que recorría los axones sin que hubiera
    pasado nada: eran despachos reales, pero el *momento* en que viajaban era inventado. Ahora si
    el cerebro está quieto, el dibujo está quieto — y eso también es información.
  - La neurona sale del `principal` del evento por una **tabla declarada**, no inferida.
    `personaDe()` colapsa en el primer guion y aplicada a los tokens inventaría personas:
    `b1-adjudicador` daría «b1» y `crm-cabina` daría «crm», que son **servicios, no personas**.
    Los que no están declarados **no pulsan**, y el panel dice cuántos eventos quedaron sin neurona.
  - `kind` (lo clasifica el servidor) separa las dos capas: el **sondeo** es un frente tenue y el
    **trabajo real** uno saturado. Medido sobre 7 días: 225.967 invocaciones, **98,2 % sondeo**.
    Un `kind` desconocido cae del lado del trabajo, nunca del sondeo: esconder ahí algo que no
    sabemos qué es sería perder cognición en el ruido.
  - `outcome` pinta el ámbar de «falló» —el mismo que ya usa el HUD para aviso— y `ms` da el
    grosor, en escala logarítmica porque el rango medido va de 0,15 ms a 60.041 ms.
  - El frente se dibuja **aditivo** (`globalCompositeOperation = 'lighter'`) con halo y núcleo:
    donde dos ramas encendidas se cruzan el brillo se suma. Medido, es la diferencia entre un
    impulso que mueve el brillo del cuadro 0,3 % (invisible) y uno que lo mueve 1,67 %.
  - El **backlog no pulsa**: al conectar llegan de golpe los eventos ya ocurridos (230 en la
    corrida medida) y dispararlos sería mostrar como presente algo pasado.

### Fixed
- **El panel se ahogaba solo y quedaba en blanco.** `/api/pulse` corre el diagnóstico completo del
  cerebro en cada llamada; sobre la memoria local (54 MB de base con 56 MB de WAL) eso **mide
  45–51 s**. Con el sondeo cada 5 s se lanzaba un pedido nuevo antes de que volviera el anterior:
  a los 30 s había seis en vuelo —el tope de conexiones por origen de un navegador— y desde ahí
  **todo `fetch` quedaba encolado para siempre**. El síntoma no era lentitud: era el panel entero
  en «—» sin un error en consola, porque las promesas no se resolvían ni fallaban.
  - Ahora el sondeo **no se apila** (guardia de pedido en vuelo) y el **grafo se pide en paralelo
    al pulso**, que no dependen entre sí. Medido: el dibujo aparece a los **5,5 s** en vez de
    nunca; los contadores llenan cuando vuelve el pulso.
  - Queda pendiente lo de fondo: correr `Diagnose()` entero en cada pulso de 5 s es caro y no hace
    falta a esa frecuencia.
- **La lente de personas dibujaba ENCIMA de la de memoria.** `#brain` tenía `display:block` por
  selector de ID, que le gana a la regla `[hidden]{display:none}` del navegador, así que ocultar
  el canvas desde JS **no ocultaba nada**: las 2171 neuronas con bloom seguían pintadas debajo y
  la pantalla se volvía una mancha blanca. Sólo se reproduce entrando por la lente de memoria y
  cambiando después — entrando por `?lens=personas` la esfera nunca llega a dibujarse.
- **`npm test` del panel corría UN archivo, no los tests.** El script decía
  `node --test src/layout.test.mjs`, así que cualquier test nuevo quedaba fuera de CI sin que nada
  se pusiera rojo. Ahora es `src/*.test.mjs`.

- **El grafo del cerebro ahora dice QUIÉN escribió cada nota.** `musubi_brain_graph` devolvía
  `id · topic · domain · mem_type · importance · heat · age_days · recency_days · gist` y nada más:
  el **autor no viajaba**. La columna existe en `observations` desde la migración v16 y
  `musubi_recall` ya la devolvía en cada item, así que el dato estaba ahí y el grafo era la única
  superficie que lo dejaba afuera.
  - Efecto práctico: **agrupar el grafo por persona era imposible sin leer el texto de las notas**.
    Para dibujar quién le escribe a quién había que sacar la identidad del encabezado del gist con
    una expresión regular — arqueología de texto sobre un campo que ya existía.
  - Arregla de paso una limitación anotada: `musubi export` tampoco traía el autor, porque
    serializa el mismo `BrainNeuron`. Ahora sí, sin tocar `export`.
  - El campo va `omitempty`: las observaciones anteriores a v16 tienen autor vacío, y **«sin
    atribución» no es lo mismo que «autor = cadena vacía»**. Un consumidor que reciba el campo
    ausente sabe que esa nota no tiene autoría, en vez de creer que alguien firmó en blanco.
  - Es aditivo: ningún consumidor existente se rompe, y el `total_neurons`/`truncated` no cambian.

- **El riel en vivo ya no muestra sólo el central: ahora también se ve el trabajo de esta máquina.**
  El feed vive dentro del `McpServer` y el único que lo exponía era `ListenAndServeHTTP` — o sea
  `musubi serve`. Un daemon stdio, que es lo que usa cada sesión de un agente contra la memoria
  local, publicaba sus eventos a un feed que **nadie escuchaba**: emisor construido, receptor
  apagado. Trabajabas todo el día contra el cerebro local y el panel no mostraba nada tuyo.
  - **Un archivo por proceso, no uno compartido**, y lo decidió una medición: hay **7 daemons stdio
    vivos a la vez** en esta máquina. Siete escritores sobre un mismo archivo es contención y
    —peor— líneas entrelazadas, que se leen como un evento corrupto y se descartan sin que nadie se
    entere. Cada daemon escribe `.musubi/live/<pid>.jsonl`, lo acota solo y lo borra al salir.
  - **Se poda lo que dejan los que mueren de golpe**, con dos condiciones a la vez: el proceso
    muerto **y** el archivo quieto un rato. Sólo la primera no alcanza porque los PID de Windows se
    reciclan, y borrar el archivo de un daemon vivo lo deja invisible. Sin poda, cada muerto dejaría
    restos que el panel relee para siempre — que es exactamente la forma del bug de los `bridge
    -watch` huérfanos que medimos el mismo día.
  - **La procedencia viaja en el evento** (`origen`) y el panel la muestra. Un riel que mezcla lo de
    esta máquina con lo de toda la empresa sin decir cuál es cuál afirma algo falso. De paso se
    cerró una pérdida silenciosa: la clave de deduplicación era `seq|at`, y los dos orígenes numeran
    su `seq` por separado, así que una colisión descartaba un evento sin decir nada.
  - **El riel existe aunque no haya central.** Antes, sin URL o sin token no había riel y una ruta
    aparte servía la explicación; ahora lo local hay que verlo igual, así que el motivo viaja por el
    mismo canal. Lo que no cambió: sin central **no se reintenta contra nada**, y el panel sigue
    recibiendo una frase accionable en vez de un 404.
  - Verificado punta a punta contra un daemon real, no sólo por unidad: `musubi_doctor` apareció en
    el riel con su duración medida (2.054 ms) y `origen: local`. Y el ciclo de vida del archivo se
    observó segundo a segundo: aparece, recibe su evento, y **desaparece cuando el daemon sale**.
  - Trece invariantes en `specs/riel-local/`, cada uno verificado fallando bajo un sabotaje que
    ataca lo que declara. **Uno tiene un hueco, y está escrito en el propio test**: el candado del
    escritor no lo cubre ninguna prueba local — saboteado, pasó 20 de 20— y lo custodia la CI con
    `-race`.

## [0.106.0] - 2026-08-22

### Added
- **El panel entró al CI: hasta ahora ningún job tocaba node.** El frontend del dashboard viajaba
  sin red de contención, y el fallo que eso deja pasar es de los caros porque es silencioso: se
  edita un `.mjs`, se olvida `npm run build`, y el binario embebe el bundle viejo. Compila,
  arranca, y muestra otra cosa. El job `panel` reconstruye y exige que no cambie **ni un byte**.
  - **La física del layout se separó a `src/layout.mjs`** para poder probarla. `dashboard.mjs`
    arranca WebGL, cuelga listeners de `document` y abre un `EventSource` en su nivel superior:
    importarlo fuera de un navegador es imposible, así que la única forma de probar el asentado era
    recortar funciones del fuente con un script — frágil e imposible de correr en CI.
  - **El refactor es idéntico bit a bit**, verificado contra la física commiteada en tres escalas
    (600/3.678/8.362 nodos, incluida la que va por el camino exacto O(n²)) y en **dos geometrías**:
    esférica y elipsoidal. La elipsoide no es decorativa — con `rx=ry=rz` un intercambio de radios
    al extraer `clampBrain` sería invisible. **Peor desvío 0,00e+0** en los seis casos.
  - **Cinco invariantes en `src/layout.test.mjs`**, cada uno verificado fallando bajo un sabotaje
    que ataca lo que ese invariante declara: que rebanar la iteración no cambia el resultado, que
    con presupuesto 0 igual se avanza (si el chequeo de tiempo fuera antes del trozo, el asentado
    no terminaría nunca — es un cuelgue, no una lentitud), que `settleStart` reinicia la pasada en
    curso (`bhGrow` reasigna los arrays del árbol), que el trabajo es proporcional al cambio, y que
    `settleTick` no miente sobre haber terminado.
  - Las versiones de `package.json` pasan a **exactas**: `three` se empaqueta DENTRO del bundle y
    `esbuild` decide cómo se minifica, así que un `^` haría fallar la comparación de bytes al
    publicarse cualquier parche aguas arriba, sin que nadie hubiera tocado el panel. Y `esbuild`
    quedó en **0.28.1**, que es la versión que realmente construye el bundle commiteado: el
    manifiesto declaraba `^0.24.2`, que nunca lo había construido.

### Fixed
- **El panel dejaba de responder mientras acomodaba el grafo, y una sola memoria nueva lo
  reacomodaba entero.** Dos causas distintas, las dos medidas. La primera es un defecto de la
  corrección anterior: repartir el asentado en iteraciones no repartía nada, porque **una sola
  iteración cuesta 26,4 ms** sobre los 3.678 nodos de memoria y **73,0 ms** sobre los 8.362 de
  código, contra un presupuesto de 6 ms por frame. El grano era 12× el tramo, así que el
  `do{}while` de `settleTick` corría igual la iteración entera. La segunda es que `changed` daba
  true con **un** nodo nuevo, y eso disparaba `iterSettle(n)` completo: 55 iteraciones sobre 3.678
  nodos = **1,45 s de CPU cada vez que se guardaba una observación** — con los hooks de captura
  escribiendo seguido, el panel trabándose cada pocos segundos y reacomodando de lugar todo lo que
  estabas mirando.
  - **La iteración ahora se corta por dentro**, en trozos de 256 nodos (`settlePasada`). Dónde
    cortar salió de medir, no de suponer: con 8.362 nodos la iteración cuesta 68,5 ms **con**
    18.073 aristas y 68,4 ms **con cero** — las aristas no cuestan nada, todo el costo es la
    repulsión por nodo. Se rebana por nodo y nada más; resortes e integración quedan enteros.
  - **Y sale idéntico bit a bit.** La repulsión de cada nodo lee un árbol Barnes-Hut ya armado y su
    propia posición, que no cambia hasta la integración del final, y escribe sólo su velocidad; por
    eso el orden no importa. Verificado contra la iteración entera con presupuesto **cero** —el
    corte más agresivo posible, hasta 33 pedazos por iteración— en tres escalas, incluida la que
    usa el camino exacto O(n²) que no se rebana: **600/600, 3.678/3.678 y 8.362/8.362 posiciones
    idénticas, peor desvío 0,00e+0**. Distribución con el presupuesto real de 6 ms: p50 6,8 ms,
    p95 8,5 ms, máximo 14,0 ms, **0 frames por encima de 16,6 ms**.
  - **Las iteraciones ahora son proporcionales al cambio** (`iterParaCambio`). Un grafo ya asentado
    que recibe unas pocas memorias nuevas no necesita re-asentarse: el resto está en equilibrio y
    el damping lo deja donde está. Pasa de 55 iteraciones a `4 + nuevas×2`.
- **La lente código se reconstruía entera en cada pulso, desde datos idénticos.** Se baja una sola
  vez y no cambia entre pollings, pero `renderLens` rearmaba sus 8.362 nodos, 17.661 aristas, listas
  de adyacencia y dos Map grandes cada 5 segundos: **28,8 ms** medidos —casi dos frames— más la
  basura de ~26.000 objetos para el GC. Ese era el tirón periódico. Ahora se saltea cuando el objeto
  del grafo es el mismo. **Sólo se saltea código, y la asimetría no es un detalle**: `fetchGraph`
  asigna un objeto nuevo, así que para código identidad distinta significa exactamente «hay grafo
  nuevo»; `aplicaDeltas` en cambio **muta memoria en el lugar**, y comparar por identidad allí
  saltearía una reconstrucción necesaria y apagaría el latido, que es la razón de ser del panel.

- **Un test del relay del riel en vivo esperaba algo que no probaba nada, y falló en CI.**
  `TestRelayMandaElTokenPorHeaderYNoPorURL` llamaba a `leerFrames(…, 2, …)` con el comentario
  «fuerza a que el relay ya haya conectado» — y no fuerza nada: `suscribir()` **sintetiza** el
  `enlace` y el `backlog` en el momento, sin haber hablado con nadie. Medido: un relay al que
  nunca se le arrancó `run()` devuelve igual esos 2 frames **en 0 s**. El test leía entonces un
  `Authorization` vacío y lo reportaba como «el relay mandó mal el token», que es justo lo
  contrario de lo que pasaba. Venía pasando de suerte, porque la goroutine del relay solía ganarle
  al viaje HTTP; bajo `-race` con la máquina cargada, perdió.
  Ahora el cerebro falso avisa por un canal cuando el pedido llegó de verdad, y el test espera
  **eso**. De paso se cierra una carrera de datos real: lo que veía el handler se escribía en la
  goroutine del servidor y se leía en la del test sin candado — `-race` no la había marcado porque
  las tripas de `net/http` crean un orden incidental, que no es una garantía.
  Reproducido y verificado: con el relay conectando 300 ms tarde, el patrón viejo falla **5 de 5**
  con el mismo mensaje que CI y el nuevo pasa **5 de 5**. Y el test sigue atrapando lo que
  custodia, comprobado con dos sabotajes por separado: sacarle el header lo hace fallar, y
  **dejarle el header pero mandar además el token en la query también** — que es la mitad del
  invariante que el primer sabotaje no tocaba.

## [0.105.0] - 2026-08-22

### Fixed
- **El grafo dejó de rehacer, cada frame, trabajo que no cambió.** Al sacarle el tope, la lente
  código del cerebro central pasó a 8.193 nodos y 17.661 aristas — y el bucle de animación
  recomponía TODAS las matrices de instancia en cada frame: ~26.000 composiciones y **2 MB de
  buffers subidos a la GPU**, cuando el presupuesto entero de un frame a 60 fps son 16,6 ms.
  Medido con las clases reales de three.js a esa escala: **43,9 ms de JS por frame**, antes de
  dibujar nada, del bloom y del SMAA. La lente memoria (3.678/3.420) costaba 11,8 ms y por eso
  se sentía bien; la de código no.
  Tres cambios, cada uno medido antes de escribirlo:
  - **Nodos**: la 3×3 (rotación × escala) es CONSTANTE por nodo —`RAD` sólo se escribe en
    `rebuildMeshes` y `n.r` no se reasigna nunca—, así que ya queda sembrada ahí y por frame sólo
    se escriben los 3 floats de la traslación, directo en el buffer. 6,20 ms → 1,57 ms.
  - **Aristas**: base ortonormal escrita a mano en lugar de `setFromUnitVectors` + `compose`. El
    cilindro es simétrico radialmente, así que cualquier par de ejes perpendiculares al eje A→B
    sirve y el quaternion sobra. Era el **62% del costo del frame**: 10,33 ms → 3,04 ms.
  - **Colores**: se escriben sólo mientras hay actividad, y una última vez al apagarse. En la
    lente código no hay actividad NUNCA (es reposo puro), así que esos ~25.000 writes por frame
    eran íntegramente basura.
  Y el bucle entero se saltea cuando nada se movió: con la animación en pausa, sin arrastre y con
  el residuo ya decaído, las posiciones son idénticas a las del frame anterior. Rotar el grafo
  pausado pasa a costar **cero**.
  Resultado medido, en proceso aislado y tres corridas por escenario: **3,5× en la lente código
  del central** (43,9 ms → 12,7 ms). Los absolutos varían bastante entre corridas por GC y JIT; lo
  estable es el ratio, y en todas el "antes" se pasa del presupuesto y el "después" entra.
  La base ortonormal se verificó contra el método viejo en **200.004 casos** —aleatorios más los
  borde: exactamente vertical, casi vertical, invertido y de largo cero—: eje coincidente a 1,2e-11,
  ortogonalidad a 2,8e-16 y **cero determinantes negativos**. Ese último control atajó un bug real:
  la primera versión tenía el producto cruz al revés (`q = d × p` en vez de `p × d`), la terna salía
  zurda, el winding se invertía y el back-face culling se habría comido las aristas.

- **Cambiar a la lente código congelaba el panel tres segundos, y volvía a congelarlo en cada
  toggle.** Eran dos causas distintas apiladas.
  La primera: el asentado del layout (`settle`) era un `for` sincrónico. Medido con las funciones
  reales extraídas del fuente, sobre los grafos de verdad: memoria local (2.097/2.000) **654 ms**,
  código local (6.113/14.235) **3.037 ms**, código del central (8.362/18.073) **3.952 ms**. Durante
  ese rato `requestAnimationFrame` no corre, así que no es que baje el framerate: **no hay frames**.
  La segunda: `POS` —el mapa que recuerda dónde quedó cada nodo para no re-asentar— era **uno solo,
  compartido por las dos lentes**. Al pasar a código, las claves guardadas eran las de memoria, todos
  los nodos salían `_new`, y el layout se recalculaba entero. Cada ida y vuelta costaba el
  congelamiento completo, para siempre. `prevIds` tenía el mismo defecto: salía de `NEURONS`, que en
  ese momento todavía tiene el grafo de la OTRA lente.
  - `POS` pasa a ser `{memory, code}` y `prevIds` sale de las claves de la lente que se está
    construyendo. Con esto el segundo toggle ya no asienta nada.
  - `settle` se parte en `settleStart` / `settleStep` / `settleTick(ms)`: el mismo cómputo total,
    pero repartido en tramos de 6 ms por frame. El grafo se ve organizarse en vez de congelarse.
  - Bandera `ASENTADO` por lente. Sin ella, cambiar de lente **a mitad** del asentado sembraba `POS`
    con posiciones a medio ordenar, `changed` daba `false` de ahí en más y el grafo quedaba
    desordenado permanentemente. Es un agujero que abrió el propio arreglo.
  - Lo dibujado **persigue** a la física en vez de saltar con ella (`refrescarBase(k)`, k=0,16).
    Hace falta porque el layout es violento de verdad al arrancar: nodos al azar en todo el volumen
    y un clamp que permite 40 unidades por iteración sobre un radio de 118 — un tercio del cerebro
    de un salto. Antes eso pasaba entero adentro del congelamiento y no se veía. La física no
    cambia: sigue corriendo exacta sobre `n.x/y/z`; lo único amortiguado es dónde se pinta.
  Verificado contra la versión anterior con el mismo estado inicial y las mismas iteraciones:
  **2.097/2.097 y 6.113/6.113 posiciones idénticas, peor desvío 0,00e+0**. El refactor no movió la
  física ni un bit.

### Added
- **RIEL EN VIVO: el panel muestra lo que el cerebro hace, mientras lo hace.** Cada invocación de
  tool sale por `GET /api/stream` (SSE) en el instante en que termina, con su hora real, su
  resultado, su latencia y —cuando hay token— de quién fue.
  **No sale de la base, y eso es lo que lo hace posible.** El ledger de uso ya guarda todas las
  invocaciones, así que lo obvio era que el panel leyera `tool_invocations` con un cursor. Medido,
  no puede ser en vivo por tres razones estructurales: el buffer baja a disco cada 10 s por
  diseño; `created_at` se estampa en el `INSERT`, o sea que es la hora del *flush* y no la de la
  llamada (en la base local hay hasta **23 invocaciones compartiendo timestamp**); y esa columna
  tiene resolución de un segundo, así que dos llamadas separadas por 40 ms son simultáneas para
  ella. Publicar en proceso resuelve las tres: en la prueba punta a punta los eventos salieron a
  `.818 · .840 · .861 · .880 · .900 · .918` — 20 ms de separación, cada uno con su hora.
  El ledger sigue siendo la **historia** (sobrevive al reinicio, se consulta con SQL); esto es el
  **presente**.
  - **El sondeo se separa del trabajo, porque el sondeo es casi todo.** Medido sobre 24 h: en la
    base local, 97.815 de 97.889 invocaciones (**99,92%**) fueron tres tools de sondeo, y el
    trabajo real fueron 4 `save_observation` y 1 `recall`; en el cerebro central, 13.919 de 18.363
    fueron `musubi_sync_pull` sola. Un riel crudo es una pared de ruido donde lo que importa pasa
    sin que nadie lo vea. Cada evento viene clasificado `trabajo`/`sondeo`, el panel muestra el
    sondeo como un pulso agregado (`sondeo · N/min`) en vez de filas, y `?kind=trabajo` lo saca
    directamente del cable. **La lista es de sondeo y el default es trabajo**, no al revés: así una
    tool nueva nace visible en vez de invisible.
  - **Aislamiento por proyecto, no sólo autenticación.** El evento lleva `principal` y `project`, así
    que la regla de `/metrics` («¿es un principal válido?») no alcanzaba: un miembro acotado a lo
    suyo vería en tiempo real a qué hora trabaja otro equipo, con qué herramientas y a qué ritmo. El
    filtro va adentro del feed, no en el handler, para que no dependa de que cada endpoint futuro se
    acuerde de aplicarlo.
  - **Nunca frena el camino caliente.** `publish` corre en la salida de toda tool: los envíos son no
    bloqueantes y a la pestaña que no lee se le descartan eventos. Lo descartado **se cuenta y se
    avisa** (`perdidos`) — un feed que pierde en silencio le hace creer al que mira que vio todo.
  - **Ni argumentos, ni resultados, ni mensajes de error.** Mismo invariante L1 del ledger, y acá
    pesa más: un feed en vivo es la superficie más fácil de dejar abierta sin querer. El struct no
    tiene dónde ponerlo, y hay un test por reflexión que rompe el día que alguien le agregue un
    campo que sí podría.
- **El panel local se enlaza al cerebro central por un relay** (`musubi dashboard --central <url>`,
  o `$MUSUBI_CENTRAL_URL` + `$MUSUBI_TOKEN`). El navegador nunca ve el token: el relay lo guarda,
  abre **una** conexión al cerebro y la reparte entre las pestañas. Sin eso habría que mandarle el
  bearer al navegador —donde queda en el historial si va por query string, y sirve para llamar a
  todo el cerebro, no sólo al feed— y además abrirle CORS al cerebro. El riel dice siempre en qué
  estado está el enlace (`conectado` / `caído` / `apagado`, con el motivo): con ~23 eventos de
  trabajo por hora, «hace veinte minutos que no pasa nada» es un estado **normal** y se ve idéntico
  a un enlace cortado.
  El panel apunta al central y no a la base local a propósito: la base local no tiene qué mostrar
  —1 recall en 24 horas— ni puede decir de quién fue nada, porque `principal` está vacío en sus
  130.471 filas.
- **Medidor de frame opt-in con `?stats=1`.** Reporta fps, ms de frame y el corte entre *mis bucles*
  (JS optimizable) y *render+bloom* (GPU), más draw calls, triángulos y dpr. Existe porque «se siente
  pesado» no es una medición, y porque `renderer.info.reset()` ya se llamaba todos los frames sin que
  nadie leyera `renderer.info`: la mitad del plumbing estaba puesta. Cuesta cero cuando no está el
  flag.

## [0.104.0] - 2026-08-21

### Changed
- **El grafo del dashboard ya no tiene tope: crece con la memoria.** Estaba capado a 300
  neuronas, y el motivo real no era el render sino el TRANSPORTE — el front pedía
  `/api/snapshot` entero cada 5 segundos. Medido: 481 KB con el tope puesto y **2,3 MB sin él**,
  que por un túnel SSH son 31 s y 117 s por pedido. A los 5 segundos ya había seis pedidos
  apilados y ninguno terminaba. El síntoma que se veía era un grafo **apagado**, no lento: la
  actividad sólo se enciende diffeando un snapshot nuevo contra el anterior, así que sin datos
  frescos todo decae a color de reposo en menos de un segundo. Subir el número sin tocar esto lo
  habría empeorado 4×.
  Ahora son dos caminos. `/api/graph?lens=memory|code` trae el grafo **entero**, con ETag: se baja
  una vez y un segundo pedido con `If-None-Match` devuelve 304 y 0 bytes. `/api/pulse?since=` es
  el sondeo: **18 KB contra 481 KB**, con los contadores reales, los dominios sin sus hojas
  (2.809 en el cerebro central, 278 KB que el front nunca leyó) y los DELTAS de actividad.
  Dos decisiones sostienen que esto escale, y cada una tiene su test: **la huella del grafo no
  incluye calor ni recencia** —si las incluyera, cada recall dispararía una re-bajada del grafo
  entero—, y **las memorias nuevas viajan completas en el pulso**, así que guardar algo se
  incorpora sin re-bajar nada. El grafo sólo se vuelve a pedir cuando el conteo del server no
  coincide con el del cliente, o sea cuando algo DESAPARECIÓ, que un delta no puede reconstruir.
  `memory.NoLimit` existe porque `limit <= 0` caía al default: no había forma de expresar «todo»,
  el tope no se podía sacar ni queriendo. El cero sigue cayendo al default de 300 para las tools
  MCP, que son otra superficie: un agente no quiere 3.667 nodos en su contexto por accidente.
  `/api/snapshot` queda intacto — lo leen `musubi-body` y el CRM desde afuera.
- **Las aristas pasaron de una malla cada una a una sola instanciada, y el layout usa Barnes-Hut.**
  Eran los dos techos del render. Cada sinapsis era un `Mesh` con su propio `ShaderMaterial`: a
  486 se notaba poco, pero el grafo completo del central tiene 3.411 y eso son 3.411 draw calls.
  Ahora es **una** `InstancedMesh` y lo que era un uniform por material es un atributo por
  instancia; sólo `uTime` queda compartido.
  El `settle()` era O(n²): a 3.667 neuronas son 6,7 M de pares **por iteración**, ~600 M en total,
  que congela la pestaña. Una grilla espacial no servía —el corte de repulsión es `rx*0.85`, o sea
  el 85% del radio del cerebro, así que casi todos los pares caen dentro y las 27 celdas vecinas
  son el volumen entero—, y por eso va un octree con criterio de apertura. Medido contra el bucle
  exacto **del mismo código**: 1,0% de error de fuerza en una iteración con theta=0,7, y 2,0 s
  para 3.667 neuronas × 90 iteraciones. El bucle exacto se conserva para grafos de menos de 700
  nodos, así que los proyectos chicos mantienen su layout bit a bit.
  ⚠️ Nota de método: la divergencia de posiciones tras 60 iteraciones es del 100% del radio, y
  **eso no es un error** — un sistema repulsivo es caótico y amplifica cualquier diferencia; los
  dos llegan a equilibrios distintos e igualmente válidos. El exacto no es «la respuesta
  correcta», es *un* equilibrio. Medir a 60 iteraciones y concluir «bug» fue el primer diagnóstico
  y era falso.

## [0.103.0] - 2026-08-21

### Changed
- **El re-embedding del histórico va en lotes de 16, y rinde 1,37× — no 4,58× como estaba anotado.**
  `/api/embed` de Ollama ya aceptaba un array en `input` y ya devolvía `embeddings` como array;
  mandar de a uno pagaba una ida y vuelta HTTP y un arranque de inferencia **por texto**.
  **Medido contra el embebedor real** (bge-m3 en el server, textos de ~1.100 caracteres):

  | lote | ms/texto | acel. |
  |---:|---:|---:|
  | 1 | 917,5 | 1,00× |
  | 4 | 758,9 | 1,21× |
  | 8 | 686,5 | 1,34× |
  | **16** | **670,1** | **1,37×** |
  | 32 | 669,9 | 1,37× |
  | 64 | 654,7 | 1,40× |

  ⚠️ **La medición corrige la creencia que motivó el cambio.** El tiempo *total* crece casi lineal
  con el tamaño del lote (0,92 s → 41,90 s de 1 a 64): el modelo en CPU **no paraleliza el
  cómputo**, así que lo único que el lote ahorra es el overhead por pedido. Es una mejora real y
  modesta. 16 y no 64 porque ahí la curva ya está plana y el resto del tamaño sólo agrega riesgo:
  un fallo a mitad tira el lote entero y el pedido HTTP crece con textos que pueden ser dossiers.
  **La guarda que hace esto seguro, y por qué se verifica DOS veces:** los vectores se aparean con
  las observaciones **por índice**, así que un lote que devuelve una cantidad distinta a la pedida
  no produce ningún error visible — corre los vectores una posición y le escribe a cada observación
  el embedding de **otra**. La memoria queda semánticamente barajada y nada lo explica. Se
  comprueba en `embedding.EmbedBatch` (todos los proveedores) y otra vez en `EmbedBackfill`, porque
  a `memory` no le llega un Provider sino una **función opaca**: confiar ahí en la garantía de un
  paquete que no se ve es cómo un invariante se vuelve folklore.
  `Embed` de Ollama ahora delega en el lote de uno, para que el truncado, el manejo de status y el
  parseo no puedan divergir entre el caso simple y el lote. Y el portero de privacidad **reenvía**
  el lote: si no lo hiciera, `EmbedBatch` caería al bucle y la mejora no ocurriría **en silencio** —
  hay un test cuyo único trabajo es atajar esa falla.

### Added
- **Una observación ya se puede anclar a un método con la misma clave que da el grafo.** El grafo
  de código identifica un método como `Tipo.Metodo`, la doc de `origin_paths` recomienda «preferí
  el símbolo»… y el ancla **rechazaba esa clave**: sólo comía el nombre pelado. Dos subsistemas que
  nunca acordaron qué *es* un símbolo, y el resultado práctico era que la recomendación no se podía
  cumplir para métodos — justo la superficie pública.
  Y el nombre pelado tenía un segundo problema, más silencioso: **funde homónimos**. Un ancla a
  `Close` cubre *todos* los `Close` del archivo, así que la marca de rancio salta por cambios en
  código que la nota no describe — la falla que `origins.go` advierte en su propio comentario, un
  nivel más abajo. Medido en este repo: 8 archivos con métodos homónimos, 18 métodos, 7 fuera de
  tests.
  Ahora `codeintel.Symbol` lleva el receptor en un campo aparte (`Recv`) y el ancla acepta **las
  dos formas**: `Tipo.Metodo` resuelve exacto, y el nombre pelado sigue funcionando **igual que
  antes**. Eso último no es comodidad: las anclas ya guardadas están escritas en la forma pelada, y
  angostar el matcheo las habría mandado a `missing` de un día para otro — una marca de rancio
  masiva y falsa, el ruido exacto que el mecanismo existe para no producir.
  `musubi_detect_changes` **reporta** la forma calificada (es la identidad, y la clave que se copia
  para anclar) pero **busca** con la pelada (son términos de FTS, donde el punto sólo parte el
  token). Dos trabajos distintos con la misma lista era parte del enredo.
- **`conflicts.ledger_prefixes`: el despliegue puede declarar qué géneros de nota son LIBRO MAYOR.**
  El motor ya trataba así a los commits y a los contratos SDD —se leen y se citan, pero nadie puede
  pedir un veredicto que los reemplace— porque los escribe él. Cada equipo, en cambio, inventa
  géneros propios que tampoco se pueden tachar (correspondencia entre agentes, actas, bitácoras), y
  ésos son convención del despliegue, no del producto: hardcodearlos metería la costumbre de un
  usuario adentro del motor de todos.
  **Medido en el cerebro central el 2026-08-17:** 465 relaciones pendientes, el 83% apretadas en la
  franja 0,30–0,35, pegada al piso del detector. No eran contradicciones: eran 27 notas
  `terminales/` —despachos entre agentes— pareándose **entre sí por la plantilla que comparten**
  (cabeceras, emoji, nombres de destinatario). 27×26/2 = 351, el grueso de la cola. Dos cartas a
  destinatarios distintos no pueden contradecirse, así que esos pares nunca iban a producir un
  veredicto — y una cola que no se puede drenar deja de leerse entera, incluida la contradicción
  real que aparezca mañana.
  La guarda se aplica **sólo en `complementaryPair`, y deliberadamente NO en `dominiosAjenos`**,
  aunque `historicalRecord` esté en las dos: un commit es evidencia sobre el mundo y por eso puede
  envejecer una nota de cualquier tema, mientras que un género declarado por configuración sólo dice
  «esto no se tacha». Eximirlo del guardia de dominios agregaría pares cruzados, o sea lo contrario
  de lo que se buscaba. Hay un test que sella esa diferencia.
  Es **asimétrica**, igual que la regla que extiende: un despacho SÍ puede envejecer una nota (la
  carta trae mediciones); lo que no se puede es tacharlo a él. Y nunca oculta memoria — el caller
  hace `continue`, así que el peor caso de un prefijo mal declarado es una relación de menos en la
  cola, jamás una observación de menos en el recall. Vacío por defecto: una instalación que no lo
  declara se comporta exactamente como antes.
  El **doctor poda con la misma regla**, y eso no es un extra: la guarda impide que nazcan
  relaciones nuevas, pero las que ya existen sólo desaparecen si el `stale_conflicts` las reconoce.
  Sin eso, declarar un prefijo dejaría la canilla cerrada y el balde lleno — y `doctor.go` ya
  declaraba podar con «la misma función que la detección, no una aproximación que pueda divergir de
  la guarda». Hay un test que impide que esa frase se vuelva mentira.

- **El grafo ya ve las llamadas a métodos de otro paquete, y con eso `musubi_impact` deja de
  subestimar el blast radius.** Hasta ahora `variable.Metodo()` no generaba arista si el tipo venía
  de otro paquete: el extractor descartaba el call-site porque el receptor no era ni el receptor del
  método ni un alias de import. Efecto medido: `DbEngine.AutoEmbedBackfill` decía `callers: []`
  cuando lo llama `cmd/musubi/main.go#func:autoBackfill`. Como los métodos exportados SON la
  superficie pública, la herramienta que existe para decir «qué se rompe si toco esto» contestaba de
  menos, en silencio y justo donde más importa.
  El comentario original decía que resolverlo «exigiría inferencia de tipos», y para el caso general
  es cierto. Pero hay un subconjunto donde el tipo está **escrito**: parámetros, resultados con
  nombre y `var x pkg.T`. Eso se lee del AST sin inferir nada, y es lo que ahora se resuelve. Un `:=`
  sigue sin resolverse a propósito.
  Se descartó la alternativa barata —indexar los métodos por su nombre pelado y resolver cuando hay
  uno solo— porque **inventa aristas**: un `client.Do()` sobre un tipo de terceros se resolvería
  hacia un `Do` propio que casualmente fuera único, y una arista inventada manda a revisar código
  que no participa. Hay un test que defiende esa política.
  Medido sobre el repo: **+25 aristas cross-paquete (8 % más que las 315 que había)**, todas en la
  dirección que estaba ciega —del binario hacia adentro—. `buildExportSnapshot` solo gana 8.
  El camino incremental también las ve: la consulta por directorio ahora trae `func` y `method`, sin
  lo cual habría resuelto menos que el índice completo y la diferencia habría aparecido sola.

### Fixed
- **El dashboard mostraba el largo de un array recortado como si fuera el total, y lo hacía en
  cuatro lugares.** El patrón correcto estaba escrito UNA vez —las neuronas dicen `300/3660`— y al
  copiarlo a mano a los otros recortes se copió sólo la mitad: se recortaba y no se contaba. Medido
  contra el cerebro central: **486 sinapsis dibujadas sobre 3620 reales** y **1149 aristas de código
  sobre 17661**, las dos impresas peladas al lado de un contador que sí declaraba su truncado — y esa
  vecindad le enseña al ojo que donde no hay barra el número es el total.
  El daño peor no era visual sino por MCP: `musubi_brain_graph` con `limit` chico devuelve
  `"synapses": []` con `"truncated": true`, y un agente lee de ahí que **la memoria no tiene ninguna
  relación**. La misma base devolvía 0, 486 o 3620 sinapsis según un tope de render que quien llama
  ni siquiera puso.
  `BrainGraph` gana `total_synapses` + `synapses_truncated`; `CodeGraphViz` gana `total_edges`,
  `edges_truncated` y `total_modules`. **El denominador NO es `COUNT(*)` sobre la tabla**: ese número
  incluye relaciones que tocan observaciones archivadas, superadas o en cuarentena —que no se
  dibujarían ni sin tope—, así que publicarlo cambiaría una mentira por otra; y en el central sería
  peor, porque `brainSynapses` lee sin `scopeClause` y un conteo crudo expondría la cardinalidad de
  otros tenants. El gemelo honesto es «relaciones con los DOS extremos visibles», que ya estaba en
  memoria antes del cap y se tiraba. Las aristas de código se cuentan **deduplicadas por
  (kind, from, to)**, el mismo criterio con que se filtra lo dibujado, porque en una lectura federada
  la misma arista lógica llega una vez por cada `project_id`.
  **Lo que impide que vuelva** es estructural, no un campo más: los totales entran a los DTOs por
  constructores POSICIONALES (un literal con campos nombrados deja el olvidado en cero y en silencio
  —así nació esto—), el recorte pasa por un helper único que devuelve el total quiera o no el
  llamador, y un test por reflexión exige que **toda** colección con tag `json:"x"` tenga `total_x` y
  su bandera. Los seis tests nuevos se verificaron FALLANDO bajo sabotaje dirigido al invariante que
  cada uno declara; el único test de sinapsis que existía corría con `limit=100` sobre 3 neuronas, o
  sea que la rama del recorte no se ejecutaba nunca y nada podía ponerse rojo.
- **El snapshot afirmaba dos poblaciones distintas para la misma memoria.** `insights.observations.active`
  se derivaba restando sólo `archived`, mientras siete líneas más abajo `utilization` usaba el
  predicado canónico: dos «activas» en el mismo JSON, 64 de diferencia en el central. Y `TopicTree`
  filtraba con un `archived = 0` propio —exactamente lo que `braingraph.go` documenta en un
  comentario que NO hay que hacer—, así que el árbol de dominios sumaba una población y el grafo
  dibujaba otra. Se agrega `observations.visible` (recuperables) en vez de cambiarle el valor a
  `active`, que alimenta `readiness` y consumidores externos; `TopicTree` y `TopicDomainCounts` pasan
  al predicado canónico y `graph.total_observations` publica el visible.
  **Cambia un número publicado**: `graph.total_observations` baja al universo recuperable (en el
  central, de 3724 a 3660). Es un cambio hacia la verdad y lo ven `musubi-body` y el CRM.
  Ahora las cinco cifras coinciden y hay un test end-to-end que lo exige:
  `utilization.active` == `observations.visible` == `brain.total_neurons` == `graph.total_observations`
  == suma de `domains[].count`. Verificado contra la base real: 2075 las cinco.
- **El contador de dominios y su leyenda salían de la muestra, con el dato bueno a mano.** Decía
  «46 dominios» donde había 90, y los conteos de la leyenda eran cuántos de los 300 nodos dibujados
  caían en cada dominio (sumaban 200, no el acervo). `graph.domains` ya trae la agregación SQL sobre
  toda la memoria, pero estaba detrás de un `||` que nunca se alcanzaba porque el valor de la muestra
  siempre es truthy. El ranking además venía sesgado por saliencia: un dominio grande y frío no
  aparecía. En la lente código el KPI «Nodos» mostraba los 400 dibujados y **se contradecía con el
  contador de arriba**, que en la misma pantalla decía `400/8193`.
- **`musubi_tokens` no repetía su propio contrato.** El `status: over` permanente con `session_id`
  vacío es correcto y está documentado en `ledger.go` —en un servidor always-on el ledger no rota y
  el total es un acumulado de por vida—, pero la descripción de la tool hablaba de «sesión» y de
  «presupuesto» sin decirlo, así que invitaba a leer una alarma donde no la hay. Sólo cambia el texto:
  el ledger no se toca.
- **El fondo de la cola de conflictos era inalcanzable, y no por culpa de quien la drena.**
  `musubi_conflicts` tenía dos órdenes —`recent` y `confidence`— y **los dos son estables**: pedir
  siempre «las primeras 30» devuelve siempre las mismas 30. Un consumidor con tope nunca llega al
  fondo, por construcción. Medido en el central el 2026-08-14: **405 pendientes**, la más vieja del
  **2026-07-29**, y un adjudicador corriendo por timer que jamás la había tocado. La conclusión
  anterior —«hay que arreglar el prompt del adjudicador»— era falsa: el adjudicador no tenía con qué
  pedirlas.

  Dos cambios, y hacen falta los dos:
  - **`order: "oldest"`** drena en FIFO, la pendiente más vieja primero.
  - **La respuesta avisa.** Cuando la lista se trunca en un orden estable, ahora vienen
    `oldest_pending_at` (cuán vieja es la más vieja que el filtro alcanza) y un `tail_hint` que
    nombra la salida. Un orden nuevo que el consumidor no sabe que existe no arregla nada: el que
    tiene que cambiar de conducta es el agente que lee la respuesta, y por eso el aviso va en el
    resultado y no en un log. Con `order: "oldest"` el aviso NO aparece — la más vieja ya está en la
    página y repetirlo sería ruido.

  El aviso respeta **los mismos filtros** que la lista. Calcularlo sobre la tabla entera le mostraría
  a quien filtra por `min_lex` la fecha de una relación que su filtro excluye, y lo mandaría a buscar
  una cola que para él no existe.

  Verificado por sabotaje: dejar `oldest` como alias silencioso del default —el modo de falla más
  probable de este cambio, porque la tool aceptaría el parámetro sin quejarse— pone en rojo los dos
  tests de orden.
- **La federación del grafo de código estaba muerta hacía semanas, y el sistema decía que estaba
  bien.** `musubi_codegraph_index` manda el grafo entero al central en UN solo POST; el central topea
  el body en 4 MiB. Medido el 2026-08-14 contra este repo: **4.958.147 bytes** (5.194 nodos, 11.225
  aristas, 113 gists) contra un tope de 4.194.304. El central respondía `-32700 "error leyendo el
  body"` y ahí moría.

  Lo grave no fue el tope sino **cómo se veía desde afuera**. El push es best-effort a propósito —un
  fallo de red no debe romper el index local—, así que `codegraph_index` seguía devolviendo verde con
  un `federated:false` discreto al costado, y el error de verdad se iba al stderr del daemon, que en
  un MCP no lo mira nadie. El central quedó congelado en 3.502 nodos mientras el local iba por 6.450.
  Nada estaba roto *visiblemente*: simplemente el cerebro compartido dejó de aprender del código.

  El push ahora **viaja comprimido**: los mismos 4.958.147 bytes salen en 363.640 (13,6:1 — es JSON
  con las mismas claves repetidas miles de veces, el caso ideal para gzip). Con eso el grafo entra
  con más de un orden de magnitud de margen.

  Tres decisiones que conviene no deshacer sin leer el porqué:
  - **El tope del cable NO se tocó.** Sigue en 4 MiB. Aceptar gzip agrega un SEGUNDO tope, aguas
    abajo, sobre el body ya descomprimido (64 MiB) — porque descomprimir sin límite es una bomba:
    4 MiB de gzip pueden expandirse a gigabytes y voltear un central always-on. Que ese número pueda
    ser tan alto se apoya en un detalle del handler: **la autenticación corre antes de leer el body**,
    así que un cuerpo comprimido sólo lo manda un principal ya autenticado.
  - **Se comprime sólo por encima de 1 MiB, no siempre.** Un central viejo no entiende
    `Content-Encoding: gzip`; comprimir de entrada convertiría los pushes chicos —que hoy sí
    funcionan— en errores de parseo. Efecto lateral a respetar: **el central se actualiza ANTES que
    los clientes que lo empujan.**
  - **Los errores del body ahora se distinguen.** Pasarse de tamaño, gzip corrupto y lectura cortada
    colapsaban los tres en un mismo `"error leyendo el body"`, y desde el cliente eso era
    indistinguible de un bug de serialización. Es la razón por la que la falla tardó tanto en
    encontrarse. El de tamaño además nombra el tope y sugiere gzip.

  Si algún día el grafo no entra **ni comprimido**, el arreglo ya no es este: hay que trocear el
  push. El cliente lo dice con los dos números en el error, para que el próximo no tenga que
  reproducirlo para enterarse.

### Changed
- **Tres tools que nadie invocó en 90 días salen del catálogo, y dos ganan su disparador escrito.**
  Salió de cruzar los DOS ledgers —central y local, ventana de 90 días— contra `tools/list`: 13 de
  las 53 expuestas estaban en cero absoluto. El catálogo se cobra por sesión y por servidor, así que
  cada una se le cobra a todos los repos en cada arranque, la llamen o no.

  **El criterio NO fue el conteo, y eso es lo que salvó el cambio.** «Cero invocaciones» no alcanza:
  `musubi_token_revoke` está en cero y eso es una *buena* noticia — nunca hubo que revocar un token.
  El criterio fue buscarle el CONSUMIDOR a cada una. Y aparecieron cinco que **musubi-body llama en
  código vivo**: `token_revoke` (panel de identidades), `author_skill`, `search_skills` y
  `log_skill_decision` (la Forja) y `code_graph_viz` (la vista del Grafo). Dormirlas habría roto tres
  pantallas del cuerpo en silencio. Su cero no mide falta de cableado: mide que nadie las usó nunca.

  Se durmieron sólo **tres**, cada una porque tiene un duplicado que SÍ corre solo:
  `musubi_resolve_skills` (el harness ya lee `.claude/skills/*/SKILL.md`, que `musubi setup` escribe
  junto al `.musubi/skills/*.yaml`), `musubi_detect_stack` (el hook de `SessionStart` ya corre
  `musubi detect --hook-mode` antes del primer turno) y `musubi_discover_skills` (ya era opt-in por
  `sourcing.marketplace_enabled`, y la solapa Comunidad del cuerpo usa `search_skills`).

  Dormir no es retirar: siguen implementadas, testeadas y **despachables si alguien las nombra**;
  sólo pierden el lugar en `tools/list`, y `MUSUBI_TOOLS_ALL=1` las devuelve sin recompilar.

  Y dos que se quedan estrenan **disparador nombrado en su propia descripción**, que es lo único que
  el agente lee: `musubi_maintain` («después de una carga masiva, sin esperar al ciclo de 24 h») y
  `musubi_detect_changes` («antes de cerrar un cambio, para saber qué verificar y qué decisión quedó
  obsoleta»). Una tool sin disparador escrito no se invoca sola por buena que sea.

  Catálogo: 53 → 50 tools. El test de dormancia exige declarar cada una con su motivo medido, y el
  sabotaje lo confirma: despertar una sin sacarla de la lista pone rojas la prueba y el golden.

### Fixed
- **La frescura de un gist ya cruza de máquina: «no se sabe» dejó de reportarse como «rancio».**
  Encontrado estrenando `musubi_recall_code` contra el cerebro central, no en review. Tras federar
  los gists (#301/#302) la tool empezó a devolver contenido de verdad —el gist de un archivo de
  altura, con sus símbolos y sus líneas, desde otra máquina—, pero `fresh` venía **siempre** false.

  No era que el gist estuviera rancio: la frescura se derivaba del disco **del servidor que
  contesta**, y el central no tiene el repo de otro proyecto. Contra el daemon local eso es
  correcto (el que pregunta y el que responde miran el mismo árbol); contra el central es una
  pregunta que el que responde no puede contestar. Control positivo: el mismo mecanismo devuelve
  `fresh=true` cuando el archivo sí está a la vista.

  El costo era exactamente el propósito de la tool. Su contrato dice «false ⇒ conviene re-leerlo»,
  así que un agente que le hiciera caso re-leía **siempre** el archivo cuando el gist venía del
  central — y ahorrar esa re-lectura es lo único para lo que existe la memoria de código. El gist
  federado quedaba informativo para un humano y sin valor para un agente obediente.

  Ahora la respuesta trae `freshness` con **tres** estados, en vez de aplastar dos hechos distintos
  en un booleano: `fresh` (el archivo no cambió), `stale` (cambió, re-leelo) y `unknown` (nadie
  pudo mirarlo, o el gist se guardó sin huella). Afirmar «rancio» sobre un archivo que no se puede
  ver es inventar un hecho que nadie midió. Además `musubi_recall_code` acepta un `fingerprint`
  opcional: un llamador programático que ya tiene el contenido en la mano manda su propia huella y
  **le gana al disco del servidor**, porque es el único que miró el archivo de verdad — con eso el
  central puede afirmar frescura sin ver el repo. El booleano `fresh` conserva su semántica exacta
  (true sólo con identidad verificada), así que ningún cliente existente cambia de comportamiento.

  Era un defecto **latente**: `musubi_recall_code` estaba entre las tools con cero invocaciones del
  ledger, y salió a la luz justamente al encenderla.

- **Al federar los gists, gana el del proyecto y no el sin atribuir.** Encontrado verificando el
  deploy de la federación (#301), no en review: el cliente reportó `gists=25` y en el central
  aterrizaron **23**. La diferencia no era una pérdida — la tabla admite dos gists del mismo
  archivo, porque la PK es `(path, project_id)`, y en altura-erp conviven los anteriores a la
  atribución multi-tenant (`project_id=''`, de junio) con los que se volvieron a gistear después
  con el suyo (de julio). Dos paths estaban duplicados, y al federarlos colapsaban en uno.

  El problema era **cuál** de los dos ganaba: se mandaban los dos y se quedaba el último que
  insertara el receptor, con el orden entre filas de igual path sin definir. En la corrida real
  ganó el correcto, pero por casualidad — bastaba un `VACUUM` o un plan de consulta distinto para
  federar el gist rancio de junio. El sabotaje lo confirma: sin la regla explícita gana el viejo.

  Ahora `AllCodeMemoryCtx` devuelve **un solo gist por path**, prefiriendo el del proyecto sobre el
  sin atribuir y, a igualdad, el más recientemente tocado — la misma regla de desempate que
  `GetCodeMemoryCtx` ya usaba para las lecturas de a uno. Un empate que se resuelve solo hoy es un
  bug que aparece mañana.

### Removed
- **`docs/propuesta-tenancy-gio.md` sale del repo: describía accesos y trabajo ajeno en un repo
  PÚBLICO.** No tenía secretos ni tokens, pero publicaba el mapa de capacidades del cerebro central
  —que un principal nombrado tiene `read: all` y `write: any`—, el reparto de los topics de esa
  persona con sus conteos, y la ruta del código que decide la atribución. Es el inventario de
  accesos de un sistema privado, en un repositorio que cualquiera clona.

  **El análisis no se perdió: se movió entero al cerebro** (`seguridad/tenancy-del-principal-gio`),
  que es privado y además es donde esa clase de conocimiento pertenece. Antes de borrar el archivo
  se verificó que el hallazgo NO estuviera ya en la memoria — no estaba, sólo vivía acá, así que
  borrar primero habría perdido el contenido y no sólo el archivo.

### Fixed
- **El outbox del cerebro central deja de resembrarse solo, y con eso `sync_status` vuelve a decir
  la verdad.** Estaba anotado como una alarma inherente del nodo terminal — "encola y nunca drena,
  no te preocupes". Es cierto en el efecto y falso en la causa. El journal del arranque del
  2026-08-12 lo muestra entero: `purgadas 1401 fila(s) 'shared' pendientes huérfanas`, y **39
  segundos después estaban las 1.409 de vuelta**, con `attempts=0` y sin error. No eran reintentos:
  eran filas nuevas.

  Eran dos piezas peleándose. `reconcileOutboxOnStartup` purga bien, pero `BackfillOutbox` corre en
  **cada apertura de la base** —no sólo al arrancar el servicio— y no miraba la config; en el server
  hay un timer que abre la base cada 30 minutos. La purga vivía segundos.

  Debajo había una discrepancia más vieja: la purga preguntaba por el DESTINO
  (`!Enabled || CentralURL == ""`) y el gate del encolado preguntaba por la INTENCIÓN (`Enabled`).
  Un nodo con `enabled: true` y sin `central_url` caía justo en la grieta. Ahora las dos pasan por
  `config.SyncConfig.HasDestination()`, y `NewDbEngine` lo deriva de la config en vez de esperar a
  que el entrypoint lo fije a mano — que es lo que dejaba afuera a los comandos CLI (`capture`,
  `ingest`, `turn`), los únicos que seguían encolando una fila muerta por captura.

  No se pierde intención durable: cuando un nodo GANA un destino, la próxima apertura siembra todo
  lo acumulado, que es exactamente para lo que existe `BackfillOutbox`. Hay un test que lo prueba, y
  los cuatro tests nuevos se verificaron rompiendo el arreglo de cuatro maneras distintas — incluida
  la guarda pasada de rosca, que sí sería pérdida de datos. Un workspace sin `config.yaml` conserva
  el default histórico de encolar.

- **Los gists de código ahora federan: el central tenía la estructura y ninguno de los titulares.**
  Medido en el cerebro central el 2026-08-12: **4.862 nodos y 10.100 aristas** federados contra
  **cero** filas en `code_memory`. `musubi_codegraph_push` llevaba nodos y aristas desde Track 20 ·
  F6 y nunca llevó los gists, así que `musubi_recall_code` contra el cerebro compartido no tenía
  nada que devolver — y ésa es justamente la única vía al gist en un proyecto que no tiene hooks.

  El campo `gists` se lee como **puntero**, y no es un detalle: en un protocolo de reemplazo,
  "no mandé gists" y "no tengo gists" no significan lo mismo. Omitir la clave deja intacto lo
  guardado (así un cliente viejo empujando el mismo proyecto desde otra máquina no le borra los
  gists a uno nuevo); mandarla vacía sí reemplaza. Del lado del emisor, si falla la lectura local
  se aborta el push entero en vez de mandar una lista vacía, que el central leería como un borrado.

  Aislamiento por tenant igual que el grafo: el `DELETE` del reemplazo está scopeado por
  `project_id` y la atribución sale del principal, no del payload. Los cuatro tests se verificaron
  rompiendo el arreglo de cuatro maneras, incluidas las dos que serían pérdida de datos.

### Added
- **El grafo de código ahora habla antes de que escribas, que es cuando importa.** `musubi_impact`
  contesta "¿qué se rompe si cambio esto?" desde Track 20 y **nunca la contestó**: cero invocaciones
  en los 400 días del ledger del cerebro central, con **3.771 símbolos indexados** en este repo,
  1.135 en altura-erp y 988 en musubi-body. No era que faltara la herramienta. El único empujón
  vivía al final del mensaje de LECTURA —"profundizá con musubi_impact"— o sea en el turno
  equivocado: para cuando el agente decide cambiar una firma, ese texto quedó veinte mensajes atrás.

  Ahora el hook `PreToolUse` cuelga de dos matchers. Al leer sigue haciendo lo de siempre; al
  **editar** inyecta el RADIO DE IMPACTO — qué símbolos del archivo tienen callers, cuántos de ellos
  son de producción, y cuántos arrastra el cierre transitivo:

  ```
  [Musubi — radio de impacto] Vas a editar «internal/memory/recall.go».
  - buildFTSQueryRanked ← 5 directo(s), 3 fuera de tests · 5 en total: …
  - scanCandidates      ← 3 directo(s), 3 fuera de tests · 3 en total: …
  (+15 símbolo(s) más con callers)
  ```

  **Rankea por callers de PRODUCCIÓN, no por callers a secas**, y esa distinción cambió el
  resultado: medido en este repo, ordenar por el total ponía arriba `scoreCandidates` (9 callers, 8
  de ellos `Test*`) y enterraba lo realmente usado. Un test que se rompe lo canta el compilador; un
  caller de producción que se rompe, no.

  Un archivo indexado **sin** callers no produce silencio sino una línea que lo dice: "no arrastra a
  nadie conocido" es justo lo que uno quiere saber antes de tocarlo, y confundirlo con "no sé" sería
  perder la distinción que el resto de esta memoria se toma el trabajo de mantener.

  Medido sobre un archivo real de 38 símbolos: **908 caracteres (~252 tokens) y ~280 ms**, contra
  los 1.745 de la superficie de lectura. Va **encendido por defecto** —a diferencia de la de
  lectura, que es opt-in— porque es cuatro veces más chica, dispara mucho menos seguido, y es inerte
  sin grafo. Dejarla detrás de un flag la habría condenado a lo mismo que condenó a `musubi_impact`:
  existir apagada.

### Fixed
- **`MergeClaudeSettings` deduplicaba por comando y no por (matcher, comando)**, así que era
  imposible atar el mismo binario a dos matchers del mismo evento: el segundo registro se
  descartaba en silencio como "idéntico". Salió a la luz al colgar `precheck --hook-mode` de
  `PreToolUse` para lectura y para edición. La idempotencia y el reemplazo-por-ruta-vieja siguen
  valiendo, ahora **dentro de cada matcher**.

### Changed
- **Seis tools que nadie llamó nunca dejan de ocupar el catálogo, sin dejar de existir.** El
  catálogo MCP se paga entero en cada arranque de cada repo: medido en el registro real, `tools/list`
  pesa **56.655 caracteres (~15.700 tokens)** y no hay forma de que un agente lo lea en partes.
  Contra el ledger del cerebro central —400 días, 129.913 invocaciones— **26 de las 59 tools tienen
  cero llamadas**, y seis de ellas no las tienen porque falte oportunidad sino porque no tienen
  quién las llame: `musubi_save_fact` (el grafo de hechos se llena por `musubi_propose_facts`),
  `musubi_log_error` y `musubi_resolve_telemetry` (la tabla `telemetry_logs` tiene **1 fila** en el
  repo más usado), `musubi_debate` (sus tres tablas en **cero** en los 9 repos), `musubi_promote`
  (con `team_mode:true` —el default de los repos activos— `save_observation` ya escribe `shared` de
  entrada: 1.238 de 1.371 acá, 230 de 230 en altura-erp) y `musubi_workflow`, que además es **la
  tool más cara del catálogo**: 3.582 caracteres, el 6,3 % del total.

  **Dormir no es retirar, y la diferencia es el punto.** Una tool dormida sigue implementada,
  testeada y **despachable por `tools/call`**: lo único que pierde es el lugar en `tools/list`.
  Retirarla habría borrado andamiaje que funciona —el motor de debate y el de DAG están enteros, lo
  que les falta es que alguien los estrene—, y la decisión es reversible por tool con un booleano.
  `MUSUBI_TOOLS_ALL=1` devuelve el catálogo completo sin recompilar.

  Medido: **56.103 → 47.998 caracteres, −8.105 (−14,4 %)** por sesión y por repo.

  Ojo con el ledger antes de sacar conclusiones de un cero: sólo registra el camino **MCP**, y el
  ledger *local* recién existe desde el 2026-08-06. Dos de los ceros de esta auditoría eran
  artefactos de esa ventana (`musubi_codegraph_index` y `musubi_codegraph_push` corrieron entre el
  07-25 y el 07-29), y uno era un falso positivo al revés: `workflow_runs` tiene 68 filas, pero son
  todas `sdd-*` y las crea `musubi_sdd`, no `musubi_workflow`.

  De paso muere una clase de drift: tres tests que no hablan del catálogo tenían el número `59`
  hardcodeado y se rompían al dormir una sola tool. Ahora lo derivan del registro.

### Fixed
- **`confidence` significaba dos cosas distintas según la fila, y desde afuera no se podía notar.**
  En una relación PENDIENTE era `max(léxico, coseno)`; en una auto-resuelta, el léxico solo. Un mismo
  0,86 podía ser «comparten muchos trigramas» o «el coseno entre dos documentos cualesquiera» — y la
  línea de base del coseno documento-contra-documento, medida en este repo, da p50 0,60 y llega a
  **0,884 para pares SIN ninguna relación**. O sea que la mitad alta de la escala es ruido con forma
  de señal.

  Costó de la peor manera: el 2026-08-11 se triaron los conflictos del cerebro central por
  «confianza ≥ 0,85» creyendo que eso ordenaba por gravedad, cuando ordenaba por parecido.

  Las dos señales ahora se guardan **por separado** (`lex` y `cosine`, migración v27) y viajan en la
  respuesta de `musubi_conflicts`. `confidence` **no se toca**: cambiarle el significado rompería a
  cualquiera que ya filtre por ella. Y se suma `min_lex`, que filtra por el solape léxico solo — el
  filtro que sirve para triar y que no existía.

  **`nil` no es `0`, y ésa es la parte del diseño que hay que sostener.** Las columnas son nullable y
  los campos son punteros: una relación anterior a la v27 no tiene el desglose y no se puede
  reconstruir sin volver a scorear los pares. Un `0` sería una mentira —un coseno de 0 quiere decir
  «ortogonales», que es información— así que las viejas quedan ausentes, que quiere decir «no se
  sabe». Por lo mismo `min_lex` **descarta** las filas sin desglose: no se puede afirmar que superan
  un umbral que nadie midió.

### Added
- **`musubi_conflicts` acepta filtros: `count_only`, `limit`, `min_confidence` y `order`.** No
  aceptaba **ninguno**: devolvía la cola entera, siempre. Medido en el cerebro central el
  2026-08-11: 358 relaciones pendientes, **77 KB por respuesta**, y el consumidor principal —el
  panel del cuerpo, que ya está desplegado— la pedía completa **cada 4 segundos** para leer un solo
  entero. Son ~69 MB por hora de app abierta, por instancia, para mostrar un número. Con
  `count_only=true` el mismo panel pide lo que necesita y nada más.

  Y el otro costo, menos visible: sin `limit` ni orden **la cola no se podía triar**. Eso explica
  cómo terminó barrida a plantilla — 166 relaciones resueltas en un minuto, 140 de ellas `related`
  con siete razones distintas. Una cola que no se puede recorrer de a poco se despacha entera o no
  se despacha.

  Dos decisiones que parecen detalles y no lo son: **`count` es siempre el TOTAL** que matchea los
  filtros aunque `limit` recorte la lista —si el conteo se truncara, un panel que pagina mostraría
  el tamaño de la página para siempre, y el número seguiría siendo plausible—, y el recorte **se
  anuncia** con `truncated:true`, porque un tope silencioso se lee como «no hay más».

  Sin argumentos la tool se comporta EXACTAMENTE como antes: los clientes desplegados no cambian.

  La descripción advierte de algo que la tool no puede arreglar: en una relación pendiente,
  `confidence` es `max(léxico, coseno)`, y el coseno entre documentos SIN relación ya llega a 0,88
  medido en este repo. Un `min_confidence` alto no trae «las contradicciones más seguras» sino las
  más **parecidas**. Sirve para acotar el payload, no como ranking de gravedad — decirlo en el
  schema evita que el próximo lo use de triage como se usó hasta hoy.

### Fixed
- **Una sola observación imposible dejó de bloquear el re-embedding de todas las demás.**
  Medido en el cerebro central: una observación de 11.700 caracteres que ese Ollama rechaza con
  `400 the input length exceeds the context length` —y que `truncate` **no** salva: falla igual
  sola, con `truncate=true`, con `false` y sin el campo— mantuvo al backfill parado **tres días**
  con 33 observaciones pendientes. El backfill abortaba en la primera que fallara, y como la
  corrida es *resumible* volvía a empezar **por esa misma**: reintentaba, chocaba, y las otras 32
  no se embebían nunca. **«Resumible» no alcanza cuando el primer ítem siempre falla.**
  Ahora un lote que cae entero se reintenta **texto por texto**, así el rechazo cuesta sólo su
  propio lugar. La rechazada se cuenta aparte (`failed`, distinto de `skipped`), se loguea **con su
  id y su tamaño** —que es el dato con el que se arregla— y **no se persiste nada suyo**, así que
  sigue pendiente para el próximo intento o para otro embebedor.
  ⚠️ **Y la regla de corte, que se MIDE en vez de deducirse.** Que falle todo el lote admite dos
  lecturas opuestas —«esos textos son imposibles» y «el embebedor está caído»— y confundirlas
  cuesta caro en las dos direcciones: hacia el verde, una corrida que termina bien con 33 fallidas
  y 0 embebidas se lee como éxito y no la mira nadie; hacia el rojo, el **estado estacionario**
  —una sola observación imposible, sola en la cola— gritaría «está caído, corré el backfill a
  mano» en cada arranque del daemon, para siempre, con un diagnóstico falso y una instrucción que
  no arregla nada. Deducirlo del progreso de la corrida **no alcanza**: con un lote de una sola
  observación no hay evidencia con qué. Así que se le **pregunta** al embebedor, con un texto
  trivial que cualquiera sano acepta: si contesta está vivo y la culpa es de los textos; si tampoco
  puede con eso, se aborta. La sonda cuesta un pedido, y sólo cuando ya falló todo.
  De regalo arregla el mismo bloqueo por el lado del portero de privacidad: en modo `refuse` un
  solo texto con secreto tumbaba el lote entero.
  `musubi embed backfill` informa las rechazadas y **sale con código 2**: quedó memoria afuera del
  recall semántico, y un script encadenado tiene que poder verlo.

### Fixed
- **Los textos largos dejan de embeberse a medias, en silencio.** El embebedor no tiene un umbral
  de largo: tiene una **banda**, y los dos lados son malos. Medido contra el ollama del cerebro
  central (bge-m3, 2026-08-18):

  | tramo | tokens | qué pasa |
  |---|---|---|
  | ≤ ~8.970 caracteres | ≤ ~2.048 | entra bien |
  | ~8.971 – 18.151 | ~2.048 – `num_ctx` | **HTTP 400 «input length exceeds the context length»** |
  | ≥ ~18.152 | ≥ `num_ctx` | entra, **truncado en silencio** |

  O sea que **entradas más grandes funcionan y una más chica falla** — el síntoma que delata que
  leer el error literalmente («es muy largo») lleva a la conclusión equivocada. Y lo de arriba es
  peor que lo de abajo: abajo se pierde la observación con un error visible; arriba se guarda un
  vector calculado sobre el primer pedazo y se lo presenta como si representara el documento
  entero. **Probado, no supuesto:** dos textos que diferían en 140.000 caracteres devolvieron el
  vector **idéntico** (coseno `1,000000`). En el cerebro central eran **40 observaciones**; la más
  larga mide 79.029 caracteres y se había embebido ~11% de ella.

  ⚠️ **Y subir `num_ctx` no arregla nada: lo empeora.** Al pasar de 4096 a 8192 el borde de abajo
  **no se movió** (8.970 → 8.921 caracteres), porque es una constante del embebedor y no una
  fracción del contexto. Lo único que se corrió fue el borde de arriba, o sea que subir el contexto
  **ensancha la banda muerta**. Era la opción que parecía obvia y la medición la descartó.

  El arreglo va del lado de Musubi: el texto se **parte y se promedian los vectores de las partes**
  (media de vectores normalizados), así el final del documento deja de ser invisible para el
  recall. **Model-free y sin tokenizador:** Musubi no puede contar tokens, así que no cablea una
  constante que estaría mal para el próximo modelo o para un idioma más denso — el trozo que el
  embebedor rechaza se parte al medio y se reintenta, hasta un piso. El límite se descubre
  midiendo, como el resto del sistema.

  Dos cosas que el troceo **no** cambia, y hay un test para cada una: el camino de los textos que
  ya entraban es **bit-idéntico** (si no, todos los vectores guardados quedarían incomparables con
  los nuevos, sin un solo error), y el lote se conserva mientras ningún texto necesite troceo, que
  es donde está la velocidad medida.

  **El troceador va DEBAJO del portero de privacidad**, no encima: el texto se tapa entero y recién
  después se parte. Al revés, un secreto que caiga sobre el corte queda partido en dos mitades que
  ninguna regla reconoce y sale **sin tapar** — verificado invirtiendo las capas, que hace viajar
  `AKIA1234` crudo hacia el proveedor.

### Added
- **`musubi embed backfill --all`** re-embebe TODAS las observaciones activas, no sólo las de otra
  procedencia. Existe porque «mismo `model_id`» no siempre significa «mismo vector»: cuando cambia
  **cómo** se embebe y no **con qué**, la procedencia no lo detecta y esas filas no las lista nadie
  — quedan mal para siempre. Es el caso del troceo de arriba: las 40 observaciones largas no están
  rancias por procedencia, pero su vector representa el 11% de su contenido. Es caro (re-embebe la
  base entera), y por eso es explícito y no automático.


### Fixed
- **El lote de embeddings se acotaba por cantidad y el costo depende del tamaño.** `embed backfill
  --all` en el cerebro central llenó el log de `context deadline exceeded`: lotes de 16 textos que
  no llegaban a tiempo. Medido contra el embebedor real (bge-m3, 2026-08-18), el costo de un pedido
  es **lineal en caracteres**, ~0,5–0,8 ms cada uno:

  | pedido | caracteres | tarda | vs. los 30 s fijos |
  |---|---:|---:|---|
  | 1 texto × 1.000 | 1.000 | 0,8 s | ok |
  | 16 × 1.000 | 16.000 | 8,1 s | ok |
  | 16 × 3.000 | 48.000 | 27,8 s | **al filo** |
  | 16 × 6.000 | 96.000 | 65,5 s | **se pasa** |

  O sea que los 30 s fijos eran, en realidad, un tope de ~50.000 caracteres que nadie había
  escrito. **Un tope por cantidad no puede acotar un costo que depende del tamaño**, y el resultado
  fue que el lote quedó *desactivado de hecho* justo para los textos grandes —cada uno caía al
  reintento uno por uno— y con él la mejora de 1,37× que se había medido. No se perdió ninguna
  observación, porque esa red ya estaba puesta; se perdió la velocidad, en silencio.

  Dos cambios, uno por punta. **El lote ahora se corta por texto acumulado** (40.000 caracteres,
  ~23 s medidos) en vez de por cantidad, y las tandas **conservan el orden**, que es lo único que
  el caller tiene para aparear cada vector con su observación. Y **el plazo se calcula con lo que
  se pide** —base más una parte por carácter, con ~3× de margen sobre lo medido y un techo para que
  un embebedor colgado se note— en vez de ser el mismo para un renglón que para un dossier. Nunca
  queda más estricto que los 30 s anteriores: un pedido que antes andaba no puede empezar a fallar
  por este cambio.

  ⚠️ Los tests se anclan a los **segundos medidos**, no a la constante elegida. Es lo que ataja el
  error de unidades: con µs en vez de ms por carácter el plazo de 96.000 caracteres crecería 0,2 s
  en vez de 192 s, quedaría igual que el fijo de antes y el arreglo sería puro comentario.

### Changed
- **Se corrige una creencia documentada sobre `truncate` de Ollama.** El comentario decía que
  pedirlo hacía el texto largo «robusto y model-free», porque Ollama recortaría al contexto exacto
  del modelo. Es falso, y estuvo escrito meses: hay una franja de largos que devuelve 400 igual —con
  `true`, con `false`, sin el campo y con `num_ctx` explícito— y por encima de ella el recorte
  ocurre **en silencio**. Quien protege de verdad es el troceo.


## [0.102.1] - 2026-08-11

### Fixed
- **El puntaje de coherencia comparaba dos escalas distintas y se ganaba solo con historia.** Medía
  «pendientes AHORA contra resueltas DE TODA LA VIDA». El primer resultado real del cerebro central
  lo dejó a la vista: **372 pendientes contra 780 resueltas** daba verde, pero las 780 son de años y
  las 372 son de hoy. Un cerebro que arbitró mucho hace un año y hace seis meses no arbitra nada
  seguía dando verde hasta que la cola pasara las 780 — o sea, hasta mucho después de que hiciera
  falta un indicador para notarlo.

  Ahora las dos preguntas se responden con datos de la MISMA ventana: *¿alguien arbitra todavía?*
  (resoluciones dentro de la ventana > 0) y *¿la cola crece o se achica?* (resoluciones ≥ detecciones
  nuevas, ambas en la ventana). El backlog acumulado sigue en la evidencia, pero como número que el
  lector interpreta y ya no como vara que el tiempo supera solo. El corolario deseado: un backlog
  heredado grande **no** castiga a quien se puso a limpiarlo, porque lo que se mide es el flujo.
- **El chequeo de mantenimiento prometía curación y medía un latido.** La marca `last_maintenance` la
  refresca el scheduler del propio binario, no una persona: en una instalación viva siempre está
  fresca. Se detectó mirando el primer dato real — la marca estaba a un minuto de la consulta porque
  el servicio acababa de reiniciarse. El chequeo era correcto; el rótulo, no. Ahora se llama «el
  scheduler de mantenimiento está vivo» y su `why` aclara que verde ahí significa «el scheduler
  corre», no «alguien curó la memoria». Medir bien y rotular mal es la forma más silenciosa de que un
  indicador mienta.
- **El fallo intermitente de `TestG4RecallConJuezLentoNoBloqueaOtraTool` tenía causa, no azar.** Se
  reprodujo y se midió: **4 fallos en 400 corridas** (~1 %), y **0 en 400** después del arreglo.

  Los tests de «el motor no traba la casa» siembran observaciones casi idénticas —difieren en un
  carácter— bajo el mismo `topic_key`, que es justo lo que el detector de conflictos llama
  casi-duplicado. El auto-supersede dispara cuando la segunda es *estrictamente* más nueva, y esa
  comparación se hace sobre `created_at`, que es el `CURRENT_TIMESTAMP` de SQLite: **resolución de un
  segundo**. Si las tres escrituras caen en el mismo segundo no pasa nada; si el segundo tickea entre
  la segunda y la tercera, la tercera oculta a las otras dos, el recall devuelve **un** item, el juez
  no se activa —necesita ≥2— y la tool termina sin llamar al motor. Ése era el síntoma exacto. La
  detección de conflictos queda apagada en esos tests, que no la prueban: la misma decisión, por el
  mismo motivo, que ya había tomado `TestMaintainTool`.

  **El arreglo no se deja librado a que el próximo se acuerde.** `sembrar()` ahora RECHAZA que lo
  llamen con la detección encendida, nombrando el problema y la línea que lo resuelve. Hace falta
  porque olvidarse es lo que pasa: la primera versión de este arreglo tocó un solo constructor de
  servidor y dejó el mismo bug vivo en `motor_con_freno_test.go`, que arma el suyo por otro lado. Con
  la guarda, olvidarse produce un rojo **determinista** que dice qué hacer, en lugar de un
  intermitente del 1 % que dice cualquier cosa.
- **La precondición de esos tests verificaba lo que no importaba.** `exigeSembradas` contaba FILAS
  con `CountObservations`, y el auto-supersede **no borra la fila**: le prende una marca que la
  esconde del recall. La precondición daba verde con 3 filas mientras el recall veía 1, y el test se
  caía dos pasos después con un mensaje que mandaba a revisar el candado — medía bien, pero otra
  cosa. Ahora verifica los items que el recall DEVUELVE, y al fallar imprime las dos cifras.

  Se pagó sola en la primera corrida: fue esta precondición nueva la que, en el CI de este mismo PR,
  encontró que el arreglo estaba incompleto — `TestM2RecallSinPresupuestoDegrada` cayó diciendo
  «3 filas, 1 item visible, algo las OCULTÓ». Un diagnóstico completo en una corrida, en vez de un
  flake más para la pila.

## [0.102.0] - 2026-08-11

### Changed
- **La revisión adversarial ahora exige la evidencia ANTES del debate.** `adversarial-review`
  abría el debate de una: N escépticos con lentes distintos deliberando sobre un cambio que nadie
  había ejecutado. Eso tiene nombre —*teatro de verificación*— y su forma de fallar es aprobar y
  que el CI falle después. Ahora los dos primeros pasos son reunir la evidencia (correr build,
  vet/lint y tests **uno mismo**, sin creerle al implementador; revisar el alcance; buscar tests
  deshabilitados o aserciones comentadas) y **sabotear cada invariante nuevo**: si romper a
  propósito el código que la prueba dice cubrir no la pone en rojo, esa prueba no medía nada.

  El sabotaje viene con las dos trampas que ya nos costaron tiempo: la mutación **vacua** —no
  altera lo que la prueba observa, así que el verde es correcto y no informa— y el **sitio
  equivocado** —la prueba mide bien, pero otra cosa—. Se suma la postura por defecto del
  verificador: **RECHAZAR**; el escéptico que no pudo verificar su lente vota `no_real`, porque
  aprobar de más deja un cambio malo en `main` y rechazar de más cuesta una vuelta.

  El invariante que lo fija no comprueba que la skill *mencione* correr tests —eso lo cumpliría un
  párrafo al final que nadie lee en orden— sino que la evidencia y el sabotaje **aparezcan antes**
  de abrir el debate, porque un agente ejecuta la lista en el orden en que está escrita.
- **La escalada de una unidad de trabajo cuenta qué pasó, en vez de repetir un string fijo.** Cuando
  una unidad agotaba sus reintentos, el dead-letter escribía siempre lo mismo: «lease agotado:
  superó el máximo de reintentos». Con ese mensaje, dos situaciones OPUESTAS eran indistinguibles —
  cinco agentes distintos muriendo en tiempos dispares (la infraestructura está inestable) y el
  mismo agente muriendo cinco veces a los ~30 s (la unidad tiene un cuelgue reproducible). La
  segunda es un bug esperando a que alguien lo mire; la primera es reintentar y seguir.

  Ahora cada reclamo deja una línea —`agente<TAB>instante`— y la escalada arma el diagnóstico:
  cuántos reclamos hubo, quiénes la tomaron, cuánto la retuvo cada uno, y **el veredicto sobre el
  patrón** dicho explícito en vez de dejárselo deducir al lector. Sin historia (base recién
  migrada) cae al mensaje de siempre: perder el detalle es aceptable, inventarlo no.

  La historia se anota en el MISMO `UPDATE` atómico del claim, con una concatenación. Un
  leer-modificar-escribir habría perdido, entre dos reclamos concurrentes, justo el registro que
  explica la carrera. Y el nombre del agente se sanea antes de escribirse: es texto de afuera y acá
  hace de separador, así que un agente llamado `a<TAB>b` podría inyectar entradas falsas.

  La columna **no guarda el motivo de la falla**, a propósito y por la misma razón que la v23: un
  motivo es texto libre que sale del trabajo mismo, y esto se lee en un mensaje de escalada que
  termina en un reporte — sería una vía para que contenido sensible salga sin pasar por el portero
  de privacidad. Con agente y marca de tiempo alcanza para distinguir azar de patrón.

### Added
- **Cada unidad de trabajo declara cuánta autonomía tiene** (`musubi_work`). Musubi sabía decir
  QUIÉN opera —el rol del token: `reader`/`writer`/`admin`— y eso es una propiedad de la
  credencial. Le faltaba la otra mitad: *cuánta autonomía tiene esta tarea*. Un mismo agente, con
  el mismo token, recibe «andá y mirá, no toques nada» en una unidad y «arreglalo solo» en la
  siguiente; si el único lugar donde vive esa diferencia es la cabeza del que la dio, el día que el
  agente hace de más nadie puede señalar la regla que rompió, porque no había regla.

  Ahora el que postea la unidad fija su techo: **L1** sólo reporta, **L2** arregla pero el cierre
  necesita la firma de otro (maker/checker), **L3** cierra solo. Del otro lado, el que cierra
  declara qué hizo: `effect=report` (no cambié nada) o `effect=apply` (cambié algo). Omitirlo vale
  como `apply` —fail-closed: quien no declara es indistinguible de quien tocó— y `autonomy`
  ausente vale como `L3`, que es exactamente lo que la pizarra hacía hasta hoy: ningún batch en
  vuelo cambia de comportamiento.

  La firma de L2 (`action=approve`, con un `reviewer` que **no** puede ser el dueño) queda atada al
  `fencing_token` del intento que se revisó. Ése es el detalle que hace que valga: sin él, una
  unidad firmada cuyo dueño se cuelga y es retomada por otro agente arrastraría la firma vieja, y
  el trabajo NUEVO —que nadie miró— cerraría amparado por la revisión del viejo. Cerrar como
  `failed` nunca se frena, en ningún nivel: una L1 que no pudiera ni declararse fallida quedaría
  colgada hasta vencer el lease y se escalaría sola como un agente desaparecido.

  Lo que esto **no** es: una jaula contra un agente que miente. Quien tocó el disco y declara
  `report` cierra igual, como hoy puede escribir cualquier cosa en `result`; contener eso pide un
  sandbox, no una columna. Lo que sí hace es que el encargo deje de ser tácito y que la regla la
  aplique el motor —dentro del mismo `UPDATE` atómico del cierre— en vez de la memoria del humano.
- **`musubi_readiness`: qué tan lista está la instalación, medido por lo que HIZO.** Las
  evaluaciones de madurez de la industria son cuestionarios: alguien marca casilleros sobre su
  propio equipo y sale un nivel. El problema no es que mientan — es que miden la INTENCIÓN. El
  equipo que responde «sí, revisamos todo lo que produce el agente» y el que efectivamente lo
  revisa sacan el mismo puntaje, porque el cuestionario nunca miró lo que pasó. Musubi tiene lo que
  un cuestionario no tiene: un ledger de invocaciones, una cola de contradicciones, un grafo de
  código y el estado de su propia memoria. Todo eso es comportamiento registrado.

  Cinco dimensiones, todas sobre datos que el cerebro ya guardaba: si lo **invocan** de verdad, si
  esas invocaciones **salen bien** (errores y rechazos por separado — no son lo mismo), si hay
  **memoria viva** y se la mantiene, si las **contradicciones** detectadas se arbitran o sólo se
  acumulan, y si el **código está indexado**.

  **La regla que le da los dientes: una señal NO OBSERVADA puntúa CERO.** No «N/A», no se saltea,
  no se promedia entre las demás. Si el grafo nunca se indexó, esa dimensión vale 0 y el global
  baja. Sin esa regla el puntaje premia no medir, que es exactamente cómo un indicador se convierte
  en una medalla: quien apaga la instrumentación sube de nivel. Por eso una instalación recién
  parida —sana, migrada, todo verde— puntúa **0**: es el caso que ningún chequeo de integridad sabe
  ver, porque no hay nada roto y el valor entregado igual es nulo.

  Y la segunda regla, que lo hace auditable: **el puntaje es opinable, la evidencia no.** Cada
  dimensión devuelve los números crudos con los que se calculó y, cuando puntúa cero, dice por qué.
  Los umbrales (5 % de fallas, tres superficies distintas, siete días de frescura) son convenciones
  discutibles, y están juntas y comentadas justamente para que se discutan; «hubo 250 invocaciones
  y 50 fallaron» no es una convención.

  Read-only y acotada al proyecto de la credencial: en el cerebro central el puntaje de un proyecto
  no se calcula con el uso, los conflictos ni el grafo de los demás.
- **Catálogo de modos de falla** (`docs/failure-modes.md`) con severidad S1/S2/S3. Seis entradas,
  todas con un caso medido y su fecha: medir bien en el sitio equivocado, capacidad desplegada que
  nadie puede invocar, la ventana rodante que miente, el sabotaje vacuo, el estado rancio en la
  memoria, y la capacidad cara que no se anuncia con su precio. La regla de admisión es que una
  falla entra cuando **se observó**, no cuando se imagina.
- **El juez de pertinencia se pide por consulta, no por servidor.** El 2026-08-10 se midió el juez
  contra la base que corre en producción —recall híbrido sobre 1.303 documentos de memoria real— y
  el resultado trajo dos números que hay que leer juntos: **+114 % en nDCG@1** (pone lo correcto
  primero) y **~8,5 s por consulta**. También mostró lo que el juez NO hace: `R@10` se mueve 5,6 %,
  porque es un reordenador y no puede encontrar lo que el recall no trajo.

  Con esos dos números, el dial global quedaba con la forma equivocada: encenderlo mete 8,5 s en el
  camino caliente de todo recall —incluido el sondeo de un cliente— y apagarlo se lo niega a quien
  está esperando una respuesta y esos segundos los pagaría con gusto. Ahora `musubi_recall` acepta
  `rerank` como tri-estado: **ausente** ⇒ decide la config, igual que siempre; **true** ⇒ esta
  consulta lo compra aunque el servidor esté en `balanced`; **false** ⇒ esta consulta lo rechaza
  aunque el servidor esté en `turbo`. Ese último no es simetría decorativa: es lo que deja a un
  sondeo correr barato sin bajarle el dial a todos los demás.

  Un `rerank:true` compra el intento, no un privilegio: pasa por el mismo freno de gasto, el mismo
  caché y la misma degradación best-effort. Y el parámetro **se anuncia con su precio** en
  `tools/list` — un opt-in caro sin costo escrito termina dentro de un bucle.

### Fixed
- **El timeout de los tests dejó de ser un default heredado.** `go test` sin `-timeout` corta a los
  **10 minutos por paquete**, y `internal/memory` venía consumiendo 6 min 13 s de esos 10 con
  `-race`. El CI empezó a fallar en builds sanos: dos corridas murieron a los 11 m 31 s y 11 m 18 s
  —duraciones casi idénticas, que es la firma de un tope y no de un test intermitente— y la primera
  lectura fue «es un flake». No lo era. Ahora los dos jobs declaran `-timeout 20m`.

  El segundo arreglo salió de que el primero estaba incompleto: sólo se había tocado el job `test`,
  y el que más lo necesitaba era `test-cross` en Windows, que sin `-race` igual tarda 12 min. Un
  arreglo a medias en un gate de CI es peor que ninguno: convence de que el problema ya se resolvió.

## [0.101.0] - 2026-08-11

### Added
- **Ya se puede medir el arsenal, y sin mentir sobre lo que se mide.** Hasta acá nadie podía decir
  qué skill vale la pena: `skill_decisions` guarda «acepté o rechacé **instalarla**», que es otra
  pregunta, y el ledger de uso no guarda argumentos —a propósito, es una garantía de privacidad— así
  que ni siquiera indirectamente se sabía qué skill se activó. `musubi_skill_usage` cuenta ahora,
  por skill: cuántas veces matcheó, **por qué evidencia** (alcance declarado, glob real o comodín),
  cuántas veces viajó su cuerpo y cuántas se lo pidieron por nombre.

  **El instrumento lo creó el cambio de niveles, sin proponérselo**: mientras cada resolución
  entregaba todos los cuerpos no había ninguna decisión que observar. Ahora el llamador ve el nivel 1
  y elige si el cuerpo vale sus tokens — y eso es lo más cerca de «sirvió» a lo que se llega sin un
  modelo. **Se guarda con el nombre de lo que es, un pedido, y no con el de lo que se le parece**: no
  hay campo `utilidad`, ni puntaje, ni ranking, y la salida dice explícitamente que si una skill
  sirvió no se puede medir sin juicio.

  De ahí salen tres lecturas accionables: **muerta** (nunca matcheó), **candidata a retiro** (matcheó
  N veces y nadie abrió su cuerpo) y **candidata a alcance declarado** (matcheó siempre por comodín y
  sin embargo le piden el cuerpo — o sea, aplica de verdad y no tiene cómo decir cuándo). Marca
  patrones; **no retira ni apaga nada**.

  Son contadores y no un log de eventos, así que la tabla queda acotada al tamaño del arsenal y no
  necesita purga. Se escriben con el buffer del ledger que ya existía: el conteo en el camino caliente
  es un append en memoria, nunca disco con el lock de dispatch tomado, y un fallo al persistir jamás
  puede hacer fallar una herramienta.
- **El arsenal se escribe en el formato que el agente lee de verdad.** Las skills existían,
  estaban validadas y federadas — y nada las usaba, porque vivían en un formato que el agente no
  mira. Ahora se exportan como `SKILL.md` con su «cuándo» adelante: las once del equipo se anuncian
  con **587 tokens** en vez de los 3.720 que costaría mandarlas enteras.
- **`musubi_doctor` acepta `deep`.** `deep:false` es un pulso de salud barato: saltea
  `db_integrity`, `fts_consistency` y `stale_gists`, las tres pasadas caras (~675 ms). Ausente o
  `true` sigue siendo el diagnóstico completo, así que ningún cliente viejo pierde cobertura. Nació
  de una medición incómoda: el sondeo de un cliente corría el diagnóstico pesado **cada 4 segundos**.

### Changed
- **`musubi_resolve_skills` deja de mandar el arsenal entero en cada respuesta.** Devolvía los
  `Skill` completos, `rules` incluido, sin niveles: como 6 de las 11 skills declaran
  `triggers: ['*']` y matchean cualquier archivo, sus cuerpos viajaban siempre, fueran o no
  relevantes. Ahora **el cuerpo viaja con la evidencia**: se lleva su `rules` la skill que matcheó
  por un glob real o por el alcance que el llamador declaró; la que entró sólo por su comodín llega
  con nombre, descripción y —esto es lo que la hace utilizable— la cláusula `cuando`, que para esas
  seis no vive en la `description` sino en su `always_because`.

  Medido contra el arsenal real de este repo: tocar un `.go` baja de **3.207 a 1.774 tokens (−45 %)**,
  y tocar un archivo que no matchea ningún glob, de **1.750 a 317 (−82 %)**. Ninguna skill
  desaparece de la lista —lo que se omite es el cuerpo, y se declara con `body_omitted`— y el nivel 2
  ya existía: `musubi_list_skills` con `query:"<nombre>"` devuelve el cuerpo completo. Hay un
  parámetro `detail` con `full` (todo, el comportamiento anterior) y `summary` (nada); un valor con
  typo es error y no cae al default.

  De paso se arregla un contrato que mentía: la tool serializaba `skills.Skill`, que sólo tiene tags
  YAML, así que emitía las claves con los nombres de campo de Go (`"Name"`, `"AppliesTo"`) y filtraba
  `managed_checksum` y `generated_at`. Nadie lo había notado porque la tool medía **cero llamadas en
  30 días** en el ledger local y en el central.
- **`deep` ahora se anuncia en `tools/list`.** Se implementó, se mergeó y se desplegó… y el sondeo
  siguió costando lo mismo, porque el parámetro no figuraba en el `inputSchema`: ningún cliente MCP
  podía descubrirlo. Una capacidad desplegada que nadie puede invocar es una capacidad que no
  existe. La prueba que lo fija es de *catálogo*, no de comportamiento — con el sabotaje, la de
  comportamiento sigue en verde, porque el handler acepta el flag igual.
- Dependencias: `modernc.org/sqlite` 1.55.0 → 1.56.0 y `github.com/odvcencio/gotreesitter`
  0.47.0 → 0.48.1.

### Fixed
- **El juez se medía contra una base que no corre en producción.** `TestMedicionJuezReal` comparaba
  `lexical` contra `lexical+juez` sin embedder, pero el cerebro central hace búsqueda **híbrida**
  desde el 2026-07-28. Así, el delta le acreditaba al juez todo lo que ya aportaba el vector: la
  aritmética era correcta, el número salía grande, y respondía una pregunta que nadie había hecho.
  Es el modo de falla más caro de una medición — medir bien en el sitio equivocado y salir verde
  igual. Ahora los dos brazos son `hybrid` y `hybrid+juez`, idénticos salvo el juez, y el test se
  **saltea con motivo** si falta `MUSUBI_OLLAMA_URL` en vez de degradar a léxico en silencio.
  El invariante quedó fijado en `TestElJuezSeMideSobreLaBaseDeProduccion`, que corre en CI sin red
  ni cuota: el defecto no rompía nada, y por eso hacía falta un test que lo mirara a propósito.
- **Un central caído tumbaba la sesión entera.** Cuando el cerebro central estaba inalcanzable, el
  arranque se quedaba colgado hasta el límite de 60 s y se llevaba puesta la sesión completa — el
  modo de falla exactamente al revés del que corresponde, porque el central es un complemento y la
  memoria local basta para trabajar. Ahora falla rápido y la sesión sigue: **63 s → 15 s**.
- **Ocultar una nota dejó de ser silencioso.** Un veredicto `supersedes` esconde la observación
  target del recall, pero nada avisaba de las OTRAS relaciones que la apuntaban: si alguien había
  construido algo encima, quedaba citando lo que ya no se ve. Salió de un caso real, y lo levantó
  quien lo provocó, no el sistema. Ahora el veredicto cuenta cuántas referencias quedan huérfanas y
  con qué veredicto. Avisa, no bloquea: el que resuelve sigue siendo quien decide.

## [0.100.0] - 2026-08-08

### Added
- **Las skills dejan de morir en la máquina donde se escribieron.** Hasta acá una skill guardada
  desde una terminal se quedaba ahí para siempre: el sync movía memoria y grafo de código, pero no
  skills, y no existía ninguna herramienta para instalar en un proyecto una skill del cerebro
  central. Se veía en el dato — el arsenal del central tenía **una** skill mientras una sola PC
  tenía once que nunca subieron. Ahora hay un arsenal compartido de verdad:

  - `musubi_promote_skill` sube una skill local al cerebro central. **Explícita a propósito**: nada
    sube solo, porque hay skills que son locales por naturaleza y otras que dispararían en todos los
    proyectos de todos. La curaduría es del dueño; la herramienta sólo la hace fácil.
  - `musubi_list_skills` gana `source`: `local` (el default de siempre, intacto), `central` para ver
    el arsenal —con cada entrada marcada `installed: true|false`— y `all` para las locales más lo
    que falta. Sin esto, instalar exigía saber el nombre exacto de memoria.
  - `musubi_install_skill` la baja y la escribe en el proyecto, marcada con su procedencia para
    poder responder «¿esto lo escribí yo o lo adopté?» sin adivinar.
  - `musubi provision --skills` deja el arsenal instalado al unir un proyecto nuevo. Sin el flag no
    se hace ninguna llamada al central: unir una máquina no puede depender de que el arsenal esté
    sano.

  Nada se pisa sin pedirlo, y lo que baja del central pasa por la misma puerta de escritura que
  `musubi_save_skill`, con su gate de calidad y su guarda de path traversal: el contenido del
  arsenal es dato remoto, y tratarlo como confiable «porque es nuestro» es como se cuela un escape
  de directorio.
- **Ledger de uso: Musubi ya puede responder qué herramientas se usan de verdad.** Hasta acá no
  podía, y eso hacía que toda decisión sobre dónde invertir esfuerzo se tomara por opinión: los
  contadores por-tool vivían en memoria y se reseteaban en cada reinicio, `/metrics` pide bearer, y
  el modo daemon —que es el 99 % del uso— ni siquiera levanta HTTP donde exponerlos. Ahora cada
  invocación queda en la base con su latencia y su resultado, y `musubi_tool_usage` la devuelve
  agrupada por herramienta con media y p95. La cobertura es **estructural**: el registro vive en el
  único punto por el que pasan todas las llamadas, así que incluye los errores, los rechazos por rol
  y por cuota, y hasta el handler que entra en pánico.

  **El ledger nunca guarda argumentos ni contenido** — el esquema no tiene dónde ponerlos, así que
  la fuga es imposible y no depende de que nadie se olvide. Y no puede hacer fallar una llamada:
  si no puede escribir, la herramienta responde igual.

### Changed
- **La cola de conflictos deja de llenarse de pares que no tienen nada que ver.** Medido sobre la
  memoria real: de 494 relaciones sólo 45 exigían una decisión — **9,8 %** — y 31 de las 36
  pendientes cruzaban temas completamente distintos (una auditoría de un servidor contra otra de un
  videojuego). La causa es que el detector dispara por parecido de **forma**: dos auditorías se
  parecen muchísimo entre sí aunque no hablen de lo mismo. Ahora dos observaciones de dominios
  distintos no proponen relación, salvo que una sea un registro histórico —un commit o un contrato
  SDD— que sí puede volver obsoleta una nota de cualquier tema.

  **Cero señal perdida**: las 45 relaciones que sí exigían decisión se conservan enteras, y la cola
  de pendientes baja de 36 a 6. La guarda nunca oculta memoria: evita *crear* una relación, jamás
  archiva ni marca nada como reemplazado.
- **`musubi_ask` fundamenta sobre el contenido completo, no sobre gists truncados.** El prompt de
  grounding mandaba al motor el gist de cada memoria —cortado a mitad de frase— así que el modelo
  sintetizaba sobre resúmenes mutilados y la calidad de la respuesta quedaba limitada por el
  truncado, no por el modelo. Ahora se hidrata el contenido íntegro de los mejores candidatos
  dentro de un presupuesto derivado del `token_budget` que ya acepta la tool (sin perilla nueva); los
  que no entran **siguen en el prompt con su gist**, así que cambia la profundidad y nunca la
  selección. Apareció auditando el cable con un motor falso en loopback, no leyendo el código.
- **El sello de procedencia ahora viaja en el prompt del motor.** Era un agujero de Q3 en el camino
  de `ask`: el recall marcaba la procedencia para el caller pero la cabecera que veía el motor sólo
  llevaba id, topic y fecha, así que una inferencia de un LLM ya corroborada le llegaba al
  sintetizador indistinguible de una nota verificada a mano. Las memorias `human` siguen sin marca:
  si todas la llevaran, el sello sería ruido de fondo.
- **La hidratación por id se partió en dos puertas.** `musubi_memory_expand` sigue contabilizando el
  acceso, igual que siempre; el grounding de `ask` usa una puerta que **no** lo cuenta, porque
  `Recall` ya lo contó sobre esos mismos ids y fundamentar una pregunta es un uso, no dos. Sin esto
  cada `ask` habría inflado `access_count` al doble justo sobre las memorias más consultadas, y el
  ranker habría empezado a alimentarse de su propia salida.

  Ojo con el efecto de borde: esto **agranda lo que cruza al motor externo** (antes gists
  truncados, ahora contenido completo). Lo cubre el portero de privacidad, que nace encendido y
  quedó verificado en el cable; quien lo apague a mano se expone a más que antes de este cambio.

## [0.99.0] - 2026-08-05

### Added
- **Dial de potencia y telemetría de la cognición.** Decidir "cuánto LLM quiero" obligaba a
  entender y coordinar cuatro perillas sueltas; ahora hay una: `cognition.effort` con `eco`,
  `balanced` y `turbo`. No es maquinaria nueva — es un **preset sobre las perillas que ya existen**,
  y `turbo` no inventa potencia: prende el juez del recall y le sube el top-K. **Lo escrito a mano
  siempre le gana al preset**, así que agregar `effort` a una config existente no le cambia el
  comportamiento en silencio; sin `effort` declarado no pasa absolutamente nada. Un valor mal
  escrito rompe el arranque en vez de caer a un default.

  Y lo segundo, que es lo que faltaba de verdad: **`musubi_cognition_stats`**, para responder con
  datos si el caché rinde, si el portero de privacidad actúa seguido o nunca, y qué motor de la
  flota se está cayendo. Tres fases habían dejado esas preguntas anotadas sin instrumento. Devuelve
  hits/misses y tasa de acierto del caché, llamadas y bloqueos del portero con los **tipos** de
  secreto tapados (nunca los valores), y escaladas del router con los circuitos abiertos. Es
  read-only: leer no resetea nada.

- **Caché de respuestas del motor de cognición.** Una pregunta idéntica ya no se paga dos veces:
  cuesta latencia, cuota y —en la flota gratis— rate-limit compartido. Es un caché **exacto**, con
  cota dura y desalojo LRU **de a una** (el `rerankCache` que reemplaza se vaciaba entero al
  llenarse: tiraba 511 entradas buenas para hacer lugar a una) y vencimiento opcional por TTL. Se
  configura con `cognition.cache` y nace encendido cuando hay motor real; apagarlo devuelve el
  comportamiento anterior byte a byte.

  **No es semántico y el nombre no lo sugiere**: no busca preguntas "parecidas". Un hit por
  similitud devolvería la respuesta de *otra* pregunta, lo que está en tensión directa con la
  garantía central del caché, así que queda como fase propia con su umbral medido.

  Va **por fuera** del portero de privacidad. La opción de ponerlo adentro —que habría evitado
  guardar secretos y subido el hit rate— se descartó al verificarla: el contador de marcadores del
  portero reintenta ante colisiones del texto crudo, así que dos sesiones con el mismo prompt tapado
  pueden numerar distinto y la respuesta cacheada se rehidrataría con el secreto equivocado. El
  costo de la decisión, dicho de frente: el caché guarda prompts y respuestas crudos en memoria
  (nunca en disco).
- **Cuarentena de escritura y procedencia en el libro mayor.** `musubi_ask` sintetiza texto con un
  LLM, y hasta ahora nada impedía tomar esa respuesta y guardarla con `musubi_save_observation`:
  quedaba en la memoria **indistinguible de una nota verificada a mano**. El grafo de hechos ya
  tenía esta guarda desde el pilar Cognición; el libro mayor no. Ahora cada observación lleva un
  sello de **procedencia** (`human`, `deterministic` o `llm:<modelo>`) y una **confianza**, y hay
  una puerta nueva —`musubi_propose_observation`— por la que entra todo lo que generó un modelo:
  queda **en cuarentena**, o sea que no aparece en ningún recall, no se puede promover a `shared` y
  no viaja al cerebro central. Sale sólo con `musubi_corroborate`, que **conserva el sello**:
  corroborar la hace visible, no la convierte en una nota humana. El sello no se puede falsificar
  porque la tool de cuarentena **no expone** parámetro de procedencia — es *por dónde entraste*, no
  lo que dijiste ser, la misma decisión estructural que hizo imposible construir un motor sin
  portero. Ojo con no confundirlo con `author`, que es *qué credencial* escribió: un agente-LLM y
  una persona usan la misma. El recall marca sólo lo que **no** es `human`, para que la marca
  signifique algo. Sin llamar a la tool nueva no hay una sola fila en cuarentena y el comportamiento
  es idéntico al anterior. Ver `specs/cuarentena-escritura-procedencia/`.

- **Portero de privacidad entre la memoria y cualquier LLM externo.** Hasta ahora, encender el pilar
  de Cognición mandaba el texto de la memoria **tal cual** al motor: un token `msb_`, una clave de
  API o una contraseña dentro de un connection string cruzaban la red sin más. Ahora todo motor real
  nace envuelto en un portero que **tapa los secretos antes de salir y los repone en la respuesta**,
  así el modelo razona sobre `[[MSB:ai-provider-key:1]]` y nunca ve el valor. La detección no se
  duplicó: la decide `internal/redact`, que ya estaba auditado, y el paquete nuevo
  (`internal/privacy`) sólo agrega lo que le faltaba para este uso — poder deshacerse. El envoltorio
  se aplica dentro de `cognition.NewProvider`, el único constructor del pilar, así que **no hay forma
  de esquivarlo**, tampoco desde código futuro. Configurable con `cognition.gateway.mode`: `scrub`
  (default, tapa y repone), `refuse` (si hay un secreto no manda nada — para motores en los que no se
  confía, como los tiers gratis que entrenan con lo que reciben) y `off` (hay que escribirlo, y avisa
  por log). Un modo mal escrito **apaga el pilar entero** en vez de caer en silencio a "sin
  protección": falla cerrado, porque sin motor no hay frontera que cruzar. El camino model-free queda
  bit-idéntico: sin motor configurado no se envuelve nada. Ver `specs/gateway-privacidad-cognicion/`.
- **Flota de motores de cognición con router y circuit breaker.** En vez de un motor único se puede
  declarar una lista ordenada con `cognition.fleet`: el router prueba de arriba a abajo, saltea los
  que el breaker tiene abiertos y devuelve la primera respuesta. Un motor que falla N veces seguidas
  queda fuera por un cooldown y después recibe **una sola** llamada de prueba; si toda la flota está
  caída, la cognición falla explícito y Musubi sigue model-free.

  Lo que hace usable la flota gratis es cómo se resuelve la regla dura *«un secreto no va a un
  servicio que entrena con lo que recibe»*: un motor `tier: free` nace con su portero en `refuse`, y
  el router trata esa negativa **no como una falla sino como una señal de ruteo** — escala al
  siguiente tier. El router no sabe qué es un secreto; sólo sabe qué hacer cuando un motor dice «esto
  no lo mando». Si no hay ningún motor `private`, el texto no se manda a ninguno. `tier` default es
  `free`: asumir "no confiable" es la dirección segura, y confiar se declara. Sin `fleet`, el
  comportamiento es bit-idéntico al del motor único. Ver `specs/router-y-circuit-breaker/`.
- **El portero también cubre los embeddings**, que eran la segunda superficie por la que el texto se
  iba a un servicio externo. Al ir a cerrar ese hallazgo resultó **más grande de lo anotado**: los
  cuatro puntos donde `redact` ya se aplicaba están condicionados a `scope == shared` o a
  `forceRedact` (el flag del cerebro central), así que en un workspace normal el contenido salía
  crudo hacia el embedder — y **las consultas no estaban protegidas en ninguna configuración**. Ahora
  todo embedder que hable por un socket (`ollama`, `openai`) nace envuelto, con `embedding.gateway.mode`
  y los mismos tres modos. `none` y `static` no se tocan: no mandan texto a ningún lado. Acá no hay
  nada que rehidratar —un embedder devuelve un vector—, así que alcanza con la redacción de una vía,
  que además es determinista: el mismo texto da el mismo vector. Y como el portero vive en el
  constructor, lo que se indexa y lo que se consulta pasan por la misma transformación, así que tapar
  no degrada la búsqueda. Ver `specs/gateway-privacidad-embeddings/`.
- **`musubi doctor` ahora diagnostica los dos porteros** (checks `cognition_gateway` y
  `embedding_gateway`). Una guarda de
  privacidad apagada sólo se veía en el log de arranque de un daemon — es decir, no se veía. El check
  informa el motor y el modo efectivo, y se pone **en rojo** si el portero está en `off`, con la
  línea de config que hay que quitar para volver al default protegido. Con el pilar apagado avisa en
  amarillo si el modo está mal escrito, para que el error aparezca ahora y no el día que se encienda.
- **Prueba estructural en el gate de verificación.** El gate NO congelaba nada: un step en
  `verifying` podía re-completarse, pisar su resultado y hacer que el veredicto en curso aterrizara
  sobre bytes distintos de los revisados — quedaba `done` sin que nadie verificara lo que finalmente
  valía. Ahora el candidato queda **congelado** (re-completarlo falla) y su identidad de contenido
  viaja en el journal; `action=verify` acepta `target_digest` y rechaza, sin tocar el estado, un
  veredicto que apunta a otro candidato. Además un step puede declarar `verify_target` (globs de
  archivos, soporta `**`): con eso el candidato deja de ser lo que el agente **dice** y pasa a ser lo
  que el proyecto **es** — Musubi deriva la identidad leyendo el disco al congelar y la **re-deriva**
  al emitir el veredicto, así que si los archivos cambiaron el `pass` no se aplica y el gate se
  reabre. Esa comprobación no pasa por el agente, que por lo tanto no puede afirmarla ni negarla. La
  deriva toma el camino de `fail` (con una reflexión que la explica) en vez de devolver error, para
  que siempre haya avance dentro del presupuesto de intentos. Sin migración: el digest se deriva del
  journal, como las reflexiones.
- **Observaciones atadas al estado que las originó.** `musubi_save_observation` acepta
  `origin_paths`: de qué habla la observación, como `ruta/archivo.go` o —mejor—
  `ruta/archivo.go#NombreDelSimbolo`. Se guarda el fingerprint del contenido y el recall lo
  **re-deriva del disco**; si cambió, la observación vuelve **marcada** como posiblemente rancia,
  nombrando qué se movió (y distinguiendo "cambió" de "ya no existe"). **Preferí el símbolo**: un
  archivo grande cambia todo el tiempo por motivos ajenos a la nota y la marca se vuelve ruido,
  mientras que el símbolo cambia cuando cambia lo que la nota describe y aguanta que le desplacen
  las líneas. Símbolos en Go, TS/JS y Python vía `codeintel`, extraídos del contenido **actual** y
  no de los rangos del grafo persistido (que valen para el snapshot en que se indexó, justo lo
  equivocado cuando el archivo cambia). Cierra el hueco que el detector de conflictos no puede ver: como
  compara observaciones ENTRE SÍ, nunca detecta una nota por lo demás válida con una línea vencida
  adentro ("PENDIENTE: X" cuando X ya se hizo) — para eso hay que comparar la nota contra el mundo,
  no contra otras notas. Es el hermano derivable de `created_at`: la edad es un proxy de si una
  memoria sigue valiendo, el fingerprint es evidencia. **Marca, nunca oculta**: que un archivo
  cambie no prueba que la nota sea falsa, así que la observación se sirve igual y en la misma
  posición del ranking (hay un test que lo fija). Opt-in puro: sin `origin_paths` todo se comporta
  exactamente como antes. Tabla satélite nueva (`observation_origins`) con FK `ON DELETE CASCADE`
  — la única del esquema que referencia `observations`, así que las anclas no necesitan limpieza
  manual al borrar. Las anclas **no viajan** al cerebro central: un fingerprint sólo tiene sentido
  contra el checkout de la máquina que lo calculó. Tope de 10 rutas por observación y anclar a una
  ruta inexistente es error (nacería marcada). `musubi doctor` gana el check `orphan_origins`
  (auto-curable). SDD completo en `specs/observaciones-atadas-a-su-origen/`.
- **Grafo de código POLYGLOT en el binario distribuido (Track 20 · F4 completo).** El indexador
  ahora deriva grafo de código de **TypeScript/TSX/JavaScript/JSX y Python**, no sólo Go. El pase
  tree-sitter (`gotreesitter`, runtime 100% Go, sin CGo) ya existía detrás del build tag `treesitter`
  pero (a) el binario del release NO lo linkeaba y (b) el walker/refresh del indexador filtraban a
  `.go`, así que el pase polyglot nunca recibía archivos: un repo JS/TS mostraba grafo de código
  vacío. Se cablea `codeintel.IndexableForGraph` (siempre `.go`; TS/JS/Py sólo con el tag) en
  `walkSourceTree`/`refreshCodeGraphForPackage`, se saltan salidas generadas (`dist`/`coverage`), y
  el release y un job de CI pasan a compilar con `-tags treesitter` + los `grammar_subset_*`
  (~8 MB de gramáticas embebidas). Medido sobre Altura real: de 0 a ~1.3k nodos / ~1.9k aristas.
- **Grafo por MCP en una llamada: `musubi_code_graph_viz` + `musubi_brain_graph`.**
  Dos tools read-only que devuelven el grafo COMPLETO renderizable (nodos +
  aristas / neuronas + sinapsis, top-N) en una sola llamada — antes sólo salían
  por `musubi export` o el dashboard HTTP. Habilitan que un cliente MCP (p. ej.
  el *cuerpo* de Musubi) dibuje el grafo del cerebro por el mismo canal, local o
  central. `musubi_code_graph_viz` ya venía scopeado por tenant; para
  `musubi_brain_graph` se agregó `DbEngine.BrainGraphCtx` (filtra observations
  por el proyecto de la credencial), así el central no filtra memoria de otros
  tenants (cubierto por el barrido de aislamiento).
- **Cognición · F3.5c — juez de pertinencia read-time (opt-in).** Nueva opción
  `cognition.read_time_rerank` (bool, default `false`) que, tras el ranking model-free del recall,
  hace que el motor LLM re-ordene los primeros `read_time_rerank_top_k` candidatos por relevancia a
  la consulta. Es el seam de mayor riesgo (latencia + rate-limit), por eso nace APAGADO (recall
  bit-idéntico y 100% model-free) y es selectivo, cacheado por `(consulta+ids)` y **best-effort**:
  ante cualquier fallo/timeout/parseo malo se mantiene el orden model-free. Sólo re-ordena, no descarta.
- **Cognición · F3.5b — `musubi_ask` (cognición a-demanda).** Nueva herramienta MCP que responde una
  pregunta en lenguaje natural SINTETIZANDO la memoria relevante (RAG) y citando los ids que la
  respaldan, vía un motor LLM opcional. Es de sólo lectura y OPT-IN: sin `cognition.provider`
  configurado devuelve un error explícito y Musubi sigue model-free (binario bit-idéntico). Se suma un
  motor real `cognition.provider: openai-compat` (alias `litellm`) que habla con cualquier endpoint de
  chat OpenAI-compatible (ej. el proxy que respalda una suscripción por el Agent SDK); la master key se
  lee de la env var nombrada en `cognition.auth_token_env`, nunca del yaml. La interfaz `cognition.Provider`
  gana `Ask()` de forma aditiva. Herramientas MCP: 43 → 44.

### Fixed

- **El grafo neuronal del dashboard mostraba memoria que el recall ya descartaba.** `brainGraphAt`
  filtraba con un `archived = 0` propio en vez del predicado canónico de visibilidad, así que
  dibujaba como neuronas las observaciones **reemplazadas** (`superseded_by`) — notas que ya fueron
  superadas por otra y que ningún recall devuelve. Ahora usa el mismo predicado que todo el resto.
  Apareció tirando del hilo de la cuarentena: sin este cambio, una observación sin corroborar se
  habría dibujado en el dashboard.

### Changed

- `cognition.read_time_rerank` pasó de `bool` a puntero en el YAML. Para el usuario no cambia nada
  (`true`/`false`/ausente se siguen escribiendo igual), pero internamente permite distinguir "no lo
  escribieron" de "lo apagaron", que es lo que hace que el dial no pise una decisión explícita.

## [0.98.2] - 2026-07-28

### Fixed

- **Embedder Ollama: truncación del input al contexto del modelo.** `OllamaProvider` pasó del
  deprecado `/api/embeddings` a `POST /api/embed` con `truncate: true`: ante un texto más largo que el
  contexto del modelo (memorias/dossiers vs. el contexto de bge-m3), Ollama lo **recorta** en vez de
  devolver `500 "input length exceeds the context length"` —que abortaba el `musubi embed backfill`—.
  Robusto y model-free: Ollama trunca al límite exacto del modelo, sin que el server tenga que adivinar
  un tope de caracteres/tokens. Incluye la herramienta de medición A/B `TestSemanticVsOllamaReal`
  (medido: bge-m3 híbrido R@10=0.917 vs POTION 0.833 sobre el fixture dorado). Habilita usar un embedder
  local fuerte (bge-m3) como señal semántica del cerebro.

## [0.98.1] - 2026-07-28

**Endurecimiento post-auditoría integral de v0.98.0** (8 agentes, 8 lentes: arquitectura, recall,
grafo/bi-temporal, seguridad, orquestación, datos/SQLite, cognición/code-graph, tests/CI). 13
correcciones con test, cada una con su *porqué* verificado, respetando lo que estaba así por diseño.
El núcleo model-free y todas las rutas OFF-by-default siguen **bit-idénticos**. Decisiones de
arquitectura tomadas en el camino: se **preservan** SQLite single-writer + federación, la cognición
*caller-borrowed*, y el juicio abstracto delegado al caller (no es deuda).

### Fixed

- **Aislamiento por tenant en la consolidación.** `Consolidate` ya no fusiona observaciones de
  proyectos distintos aunque el texto empate por trigramas (clave de dedup + guard por `project_id`).
- **`sync_seq` atómico en la ingesta inbound.** El bump de `sync_seq` se pliega DENTRO del UPSERT (una
  sola sentencia), cerrando la ventana donde un update entrante podía quedar con seq inconsistente.
- **Cuarentena de cognición por allowlist.** El filtro de hechos autoritativos pasó de denylist
  (`NOT LIKE 'llm-extract:%'`) a allowlist (`source = 'agent'`): cualquier procedencia futura
  no-`agent` queda excluida del read autoritativo por default.
- **Determinismo de `ResolveEntityName`.** Ante empate de similitud resuelve por orden lexicográfico
  estable (`ORDER BY name`), no por el orden físico volátil tras un VACUUM.
- **Degradación elegante del recall.** Una señal OPCIONAL que falla (pool vectorial / co-ocurrencia /
  centralidad de grafo) ya no tumba el recall léxico: se loguea, degrada y sigue.
- **Migración v14 anti-brick.** Barre relaciones huérfanas ANTES del rebuild bajo FK, evitando que una
  relación legacy huérfana impida abrir la base.
- **Atomicidad paso↔artefacto en el flujo SDD.** `musubi_sdd` complete persiste el artefacto ANTES de
  cerrar la fase: un fallo del artefacto deja la fase reintentable en vez de romper en silencio el
  contrato "fase done ⟹ artefacto existe" del que dependen las fases siguientes.
- **Guard de polaridad en la consolidación.** `Consolidate` ya no fusiona una afirmación con su
  negación ("usa X" vs "no usa X", que empatan en trigramas 0.89 > 0.85): un candidato con distinta
  cuenta de negadores explícitos no es duplicado. Model-free y acotado a negación explícita (medido
  antes de arreglar); antónimos/negación implícita siguen delegados al caller.
- **Schemas de array válidos.** `musubi_propose_facts` (`facts`) y `musubi_work` (`units`) declaran
  `items`, cumpliendo JSON Schema.
- **Blindaje del RMW de orquestación.** El complete del DAG/SDD es un read-modify-write cuya seguridad
  depende del Lock exclusivo del dispatch (single-writer); se documenta el invariante y se agrega
  `musubi_sdd` al guard de clasificación read/write. Un despliegue multi-proceso requeriría
  CAS-by-version (atado a la posture de concurrencia; no se implementa especulativamente).
- **README y guardas de test.** El README refleja las **43** tools reales (con guard de conteo contra
  el registro). Dos flakes eliminados de raíz asertando invariantes DETERMINISTAS en vez de
  side-effects sensibles al timing: `TestAutoMaintainAfterSaves` (invariante `saveCount` + espera de
  la TERMINACIÓN de la goroutine async, que en Windows rompía el cleanup del `TempDir`) y
  `TestModelFreeBaselineNoRegression` (seeding con `created_at` CONSTANTE vía la primitiva
  `DbEngine.SetObservationCreatedAt` ⇒ el recall model-free es una función pura del fixture; nueva
  guarda `TestModelFreeBaselineDeterministic`; la baseline commiteada no cambia, 0.66 ya era el valor
  determinista).

## [0.98.0] - 2026-07-28

**Pilar Cognición — el 3er pilar de Musubi (junto a Memoria y Orquestación).** Añade cognición
respetando el contrato que preserva la identidad model-free: *el LLM PROPONE, nunca escribe directo
al libro mayor durable*, y el core sigue sin llamar a ningún LLM (la cognición se *presta del
caller*, como `musubi_judge`/`debate`). Cinco fases (F0–F4), todas aditivas y **OFF por default**:
un Musubi sin `cognition.*` configurado es **bit-idéntico** al anterior. Loop completo
`propose → resolve → [cuarentena] → review → corroborate → sweep`. Ver `sdd/cognicion-f*` en la
memoria de Musubi.

### Added

- **Pilar Cognición · F0 (instrumentación).** Cimientos del 3er pilar de Musubi (junto a Memoria y
  Orquestación), de riesgo cero y OPT-IN. El contrato que lo hace compatible con la identidad
  model-free: *el LLM PROPONE, nunca escribe directo al libro mayor*. Tres piezas aditivas:
  - **Procedencia de aristas** — columna `source` en `relations` (migración v20,
    `NOT NULL DEFAULT 'agent'`) + `SaveFactFromSourced(...)`, para auditar, excluir y revertir hechos
    según su origen (`agent` | `llm-extract:<model_id>` | `heuristic`). La procedencia se fija al CREAR
    la arista (la precedencia al re-afirmar se define en F1: corroboración agent-wins).
  - **Baseline de no-regresión** del recall model-free en `internal/recalleval`
    (`testdata/baseline_modelfree.json` + test que falla si MRR/Recall@k/nDCG@k caen; regenerable con
    `MUSUBI_UPDATE_BASELINE=1`).
  - **Andamiaje `internal/cognition`** — interfaz `Provider` + `NoopProvider` + factory +
    `CognitionConfig` + `mcp.WithCognition`, APAGADO por default: sin `cognition.provider` en la config,
    Musubi es bit-idéntico a un cerebro model-free. F0 no realiza ninguna llamada real a un LLM.
- **Pilar Cognición · F1 (cuarentena y corroboración).** Vuelve *enforceable* el invariante del pilar
  en el grafo de hechos, model-free y sin conectar ningún LLM:
  - **Cuarentena en read-time** — el read autoritativo (`RecallFacts`/`FactPath`, vía el único choke
    point `liveFactFilter`) EXCLUYE por default las aristas propuestas por un LLM
    (`source LIKE 'llm-extract:%'`); quedan invisibles hasta ser corroboradas.
  - **Corroboración por precedencia** — al re-afirmar un triplete, la procedencia se promueve hacia lo
    autoritativo (agent-wins): un `agent` corrobora una propuesta LLM; una re-propuesta LLM nunca
    degrada un hecho de agente.
  - **Propose-only** — la invalidación por cardinalidad sólo la dispara un save `agent`: una propuesta
    LLM no puede tachar lo autoritativo. Bit-idéntico para los datos actuales.
- **Pilar Cognición · F2 (propuestas caller-borrowed + revisión).** El pilar ya PRODUCE cognición, por
  la vía más segura — *caller-borrowed*: sin LLM en el server, sin red, sin superficie de ToS:
  - **`musubi_propose_facts`** — el agente-LLM aporta tripletas que extrajo; entran al grafo en
    cuarentena con procedencia `llm-extract:<model>` (no autoritativas, invisibles a `recall_facts` por
    default, no invalidan nada). Mismo patrón que `musubi_judge`/`debate`: Musubi delega la cognición
    al caller y nunca llama a un LLM.
  - **`recall_facts include_proposed`** — flag para revisar las propuestas antes de corroborarlas.
  - Loop completo: proponer → revisar → corroborar con `musubi_save_fact` (F1 las promueve a
    autoritativas). Aditivo: `recall_facts` sin el flag es bit-idéntico.
- **Pilar Cognición · F3 (guardas de calidad sobre las propuestas).** Dos guardas deterministas y
  model-free que moderan lo que un motor LLM propone, ambas OFF por default (⇒ bit-idéntico):
  - **Enum de predicados** (`cognition.allowed_predicates`) — `musubi_propose_facts` rechaza el
    lote entero si alguna tripleta usa un predicado fuera del vocabulario controlado
    (case-insensitive), para que el LLM no fragmente la ontología en sinónimos. No afecta a
    `musubi_save_fact` autoritativo; vocabulario vacío ⇒ allow-all.
  - **Barrido de propuestas rancias** (`cognition.proposal_ttl_hours`) — el mantenimiento
    (`musubi_maintain` + auto-mantenimiento) invalida las propuestas en cuarentena no corroboradas
    más viejas que el TTL, para que la cuarentena no crezca sin fin. Nunca toca lo autoritativo
    (`source=agent`) ni lo ya invalidado; TTL 0 ⇒ no-op. Reporta el conteo en `proposals_swept`.
- **Pilar Cognición · F4 (resolución de entidades para propuestas).** Tercera guarda de calidad,
  también determinista y model-free: `musubi_propose_facts` canonicaliza el `subject`/`object` de
  una tripleta a una entidad EXISTENTE suficientemente parecida (similitud de Jaccard sobre
  trigramas ≥ `cognition.entity_resolution_threshold`, el mismo criterio que la consolidación),
  para no fragmentar el grafo con variantes de la misma entidad (`potions`→`potion`, typos). Se
  elige trigramas sobre coseno a propósito: sin dependencia de embeddings, determinista, fiel a la
  identidad model-free. Sólo aplica a propuestas — `musubi_save_fact` autoritativo no se toca — y
  el umbral 0 (default) la desactiva ⇒ bit-idéntico. La respuesta informa cuántos nombres canonicalizó.

## [0.97.0] - 2026-07-26

Endurecimiento post-auditoría exhaustiva (v0.96.0): 15 hallazgos corregidos con TDD +
1 decisión de diseño documentada. Ver `audit/2026-07-26-*` en la memoria de Musubi.

### Security

- **Aislamiento por tenant en la detección de conflictos del ingest.** El pool de candidatas de
  `DetectRelations`/`BandNeighbors` no estaba acotado por proyecto: en el central multi-tenant la
  respuesta de `save_observation` filtraba ids+gists de otros tenants y un auto-supersede podía OCULTAR
  memoria ajena. Ahora una guarda de tenant en el único loop donde nacen las relaciones lo impide.
- **Inyección de argumentos de git en `detect_changes`.** El `ref` del cliente se pasaba crudo a
  `git diff`; un `ref="--output=/x"` escribía/truncaba archivos. Se rechaza `-` inicial + `--end-of-options`.
- **Gate de admin en operaciones destructivas globales.** `maintain` y `doctor(repair)` operan sobre
  toda la base sin scope; ahora exigen un principal admin (el diagnóstico de `doctor` sigue abierto).
- **`/metrics` exige autenticación cuando hay registry** (antes caía abierto en el setup multi-tenant
  sin token legacy).
- **`ingest_url` redacta antes de embeber** (secretos ya no derivaban el vector al-reposo).
- **`promote`/`judge` acotados por proyecto** (`*Ctx`): un principal no muta memoria/relaciones de otro
  tenant conociendo su id.
- **Redacción de secretos en hex puro** (el catch-all de entropía no cubría hex; se excluyen las
  longitudes de hash de git). **Path traversal** en la carga de workflows por nombre. La guarda
  cross-tenant de `save_observation` ya no falla-abierta al errar su lectura.

### Fixed

- **Recall: señales de edad y frecuencia revividas.** `ageDays` parseaba `created_at` solo con RFC3339
  pero SQLite lo guarda con formato de espacio ⇒ edad 0 siempre, castigo por edad muerto y `accessRate`
  degenerado al contador crudo (rich-get-richer). Ahora parsea ambos formatos.
- **`provision --brain` habilita el sync en proyectos ya inicializados.** Detectaba "ya configurado" por
  un match textual `^sync:` que el config por defecto siempre emite (deshabilitado); ahora consulta el
  estado real y reemplaza el bloque en su lugar.
- **Ingest de artículos con tope de tamaño** (`io.LimitReader`, anti-OOM/DoS en el central always-on).
- **Grafo de código federado ya no se reporta stale** en el central compartido (sus archivos no están
  en disco; no se juzga frescura por fingerprint ahí).
- **Sync entrante: no se salta filas tras un fallo de ingest** (el cursor avanza solo hasta la última OK
  contigua). **Outbox: fencing por estado** — una marca rezagada no resucita una fila terminal.
- **Robustez:** el cargador de tokenizer estático (`spm`) degrada en vez de paniquear ante un
  `tokenizer.json` malformado.

### Added

- **`sync_seq`: la memoria shared editada se re-propaga al equipo.** Migración **v19** (columna monótona
  `sync_seq`, backfill = rowid); el pull entrante pagina por ella en vez de por `rowid`, así una EDICIÓN
  de una obs shared ya sincronizada vuelve a bajarse (antes el mirror quedaba stale) y el cursor es
  estable ante VACUUM.
- Los comandos `ingest` y `catalog harvest` ahora aparecen en la ayuda (`musubi help`).

## [0.96.0] - 2026-07-25

### Added

- **Federación del grafo de código al cerebro central (Track 20 · F6).** El grafo de código de cada
  proyecto ahora viaja al central: tras `musubi_codegraph_index`, el daemon local empuja su grafo
  entero (nodos + aristas) por una tool nueva `musubi_codegraph_push`, y el central lo **REEMPLAZA**
  scopeado por el `project_id` del **PRINCIPAL** del token — aislamiento por tenant: un `write=own`
  no puede plantar su grafo en otro proyecto (el `project_id` del payload se ignora, misma guarda
  `writeOriginFor` que `save_observation`). **Best-effort**: gateado por sync + team mode; un fallo
  del push **jamás** rompe el index (el grafo local ya quedó bien) y re-indexar re-empuja
  (consistencia eventual). Cierra la **topología federada** que el Track declaró desde F1 — falta
  sólo la vista en la cabina CRM (F7, repo aparte). Push por-request full-replace (batching diferido).
  Model-free.

## [0.95.0] - 2026-07-25

### Added

- **Grafo de código derivado del AST de Go (Track 20 · F1).** `internal/codeintel` ahora emite un
  GRAFO —nodos (archivo/símbolo/paquete) y aristas tipadas `IMPORTS`/`CONTAINS`/`CALLS`— derivado
  del AST, **model-free y Go puro**. `IMPORTS` y `CONTAINS` son exactos (confianza 1.0); `CALLS`
  resuelve llamadas **intra-paquete** (match único, confianza 1.0) y **difiere** las cross-paquete
  precisas (la dependencia ya vive en `IMPORTS`). Id de nodo estable (`path#kind:name`, con receiver
  para métodos) — una función y un método homónimos nunca colisionan.
  - **Persistencia federable con invalidación por fingerprint.** Migración **v18** (`code_graph_nodes`
    / `code_graph_edges`), scopeada por `project_id` (mismo patrón de tenancy que `code_memory`). El
    grafo NACE derivado y se persiste para poder **federarse** (el cerebro central no tiene el fuente)
    y servir consultas baratas; cada fila lleva el `src_fingerprint` del archivo del que se derivó, así
    una desincronía se reporta **STALE** en vez de mentir. La arista es propiedad de su `src_path`: el
    refresco borra por archivo y reinserta (nunca deja aristas stale). Aditiva: no toca ninguna tabla.
  - **Poblado como efecto de `save_code`.** Guardar el gist de un `.go` deriva y persiste el grafo de
    su paquete (best-effort, no falla el guardado). En F1 **no hay tool pública ni hook que responda
    consultas** — eso es F2. Las aristas son **sólo derivadas**, nunca provistas por el agente.
- **Consultar el grafo de código sin leer archivos (Track 20 · F2-A).** Cuatro tools MCP nuevas,
  model-free: `musubi_codegraph_index` (indexa el repo entero: recorre los paquetes Go y persiste su
  grafo), y tres de consulta read-only y scopeadas — `musubi_code_graph` (callees/callers/imports de
  un símbolo, o los símbolos de un archivo), `musubi_impact` (cierre transitivo de callers: "qué se
  rompe si cambio X") y `musubi_map` (panorama: conteos, god-nodes por grado, entry points). Cada
  respuesta **anota `stale`** comparando el fingerprint guardado con el actual del archivo (cierra el
  gap de staleness de F1). El hook `PreToolUse` que inyecta el subgrafo antes de leer queda para F2-B
  (con opt-in de config).
- **El puente código↔memoria (Track 20 · F3).** Nueva tool `musubi_code_context` (read-only): dado un
  símbolo devuelve su **estructura** (nodo + callees/callers) **y su porqué** — las decisiones/gotchas
  de la memoria que lo mencionan (`explained_by`, topic_keys). Es el análogo de `musubi_entity_context`
  para el código. El weld se **deriva al consultar** (FTS por nombre/archivo, scopeada por proyecto),
  **no** como arista escrita a mano — respeta el invariante de F1 ("aristas sólo derivadas") y no se
  pudre. Es el diferencial del Track: *qué es esto, a qué llama, quién lo llama, y por qué es así*.
- **El hook que responde antes de leer (Track 20 · F2-B).** El hook `PreToolUse` de `musubi precheck`
  gana una 3ª superficie: cuando vas a `Read` un archivo Go **indexado**, inyecta su **estructura**
  (imports + funciones/métodos con a quién llaman y quién los llama) para navegarlo **sin leerlo** —
  la palanca de tokens de Graphify, model-free. **Opt-in**: apagado por default; se enciende con la
  variable de entorno `MUSUBI_CODEGRAPH_HOOK=1` (así no cambia la experiencia actual hasta que lo
  quieras). Aun encendido es inerte hasta correr `musubi_codegraph_index`. Se contabiliza en el
  ledger de tokens (`precheck_codegraph`).
- **Aristas para TypeScript/JavaScript/Python (Track 20 · F4).** El grafo de código ahora puede
  derivar símbolos + imports + CALLS intra-archivo para `.ts/.tsx/.js/.jsx/.py`, usando
  **`gotreesitter`** (un runtime tree-sitter **100% Go, sin CGo**). Es **opt-in a nivel de build**:
  el binario por default de Musubi queda **idéntico y lean** (no linkea tree-sitter; esos lenguajes
  siguen solo-símbolos), y las aristas se activan compilando con
  `-tags 'treesitter grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_javascript grammar_subset_python'`
  (los `grammar_subset_*` acotan las gramáticas embebidas a las que usamos: pocos MB, no las ~206).
  Mismo modelo que Go (F1): CALLS **intra-archivo** con match único; cross-archivo diferido.
  Nota de mantenimiento: `gotreesitter` es una dependencia **tag-gated**; no la borres con
  `go mod tidy` sin el tag `treesitter`.
- **Índice incremental + poda de fantasmas (Track 20 · F5).** `musubi_codegraph_index` gana
  `mode:"incremental"`: reconcilia el grafo con el working tree **reusando el `src_fingerprint`
  ya guardado por archivo** (el propio grafo es el estado anterior — sin git ni cursor de commit).
  Sólo re-deriva los paquetes con archivos **modificados/nuevos**, **poda** los nodos/aristas de
  archivos **borrados/renombrados** (scopeado por `project_id`) y salta lo sin cambios (0 paquetes
  si nada cambió). `mode:"full"` sigue siendo el default. Devuelve además `pruned` y `skipped`.
  - **Fix de correctitud (staleness de archivos borrados).** Un nodo cuyo archivo ya no existía se
    reportaba `stale:false` (se veía **fresco**), así que `impact`/`code_graph` podían apuntar a
    código eliminado como si estuviera vigente. Ahora un archivo ausente/ilegible cuenta como
    **stale**.
  - **Visibilidad en `musubi_map`.** El panorama reporta cuántos archivos están `stale` (cambiaron
    desde el índice) o `ghosts` (borrados) — si son >0, conviene re-indexar.
- **Lente "código" en el dashboard-cerebro (Track 20 · bonus visual).** El dashboard WebGL gana un
  toggle **memoria ↔ código**: la misma escena three.js ahora también dibuja el GRAFO DE CÓDIGO —
  **color por módulo** (paquete), **tamaño por centralidad** y **aristas tipadas** (`llama` / `importa`
  / `contiene`). Al pasar el mouse sobre un símbolo, revela **qué memorias lo explican**
  (`EXPLICADO_POR`, weld F3 derivado por FTS on-demand). Backend: nuevo `CodeGraphViz(ctx, limit)`
  (análogo a `BrainGraph`, pero **scopeado por proyecto** — no cruza tenants) servido en el mismo
  `/api/snapshot` (campo `code`), y `/api/explained` para el hover. La centralidad **pondera los
  `IMPORTS` a la baja** para que los god-nodes propios del proyecto destaquen sobre los paquetes de
  la stdlib. Model-free, read-only, loopback. El motor de render no cambió: la lente rellena los
  mismos campos que el grafo de memoria.

## [0.94.0] - 2026-07-17

> **El cerebro solo muestra actividad real.** Al refrescar o entrar/salir del dashboard ya no se
> encienden neuronas "de bienvenida": la primera carga arranca en reposo y solo pulsa lo que
> cambia de verdad entre polls.

### Fixed

- **El brain-dashboard ya no fabrica actividad al cargar.** `firstLoad` encendía la neurona más
  reciente y propagaba un glow (`thinking=0.6`) a las vecinas — un pulso de bienvenida que aparecía
  en cada refresh. Ahora la primera carga es **reposo puro**.
- **Se eliminan los falsos "escribir"/"relacionar" por churn del top-300.** El dashboard muestra 300
  de N neuronas por saliencia; una que cruzaba el borde entre polls se marcaba como memoria/relación
  nueva sin serlo. Ahora `escribir` requiere que la memoria sea genuinamente joven (`age_days<0.02`)
  y `relacionar` que ambos extremos de la sinapsis ya estuvieran visibles.

## [0.93.0] - 2026-07-17

> **El cerebro se ve como un cerebro.** El brain-dashboard pasa de Canvas 2D a WebGL (three.js):
> nodos facetados con luz de borde, sinapsis con un pulso de luz continuo, bloom cinematográfico y
> movimiento libre — con la actividad en vivo apegada a los datos reales del snapshot, sin inventar.

### Changed

- **El brain-dashboard se reescribe de Canvas 2D a WebGL puro (three.js), embebido en el binario.**
  Los nodos son icosaedros facetados con rim-light fresnel (`InstancedMesh`); las sinapsis son tubos
  con un shader de **pulso continuo** (banda de luz viajera) en lugar de partículas discretas.
  Post-proceso UnrealBloom + SMAA + MSAA, `TrackballControls` para mover/zoom libre, y arrastrar una
  neurona con vuelta por resorte (los vecinos conectados la siguen). Layout force-directed esférico:
  las conectadas quedan más juntas y el resto más separado.
  - **La actividad EN VIVO está apegada a datos reales** del snapshot (diff entre polls de 5 s):
    memoria nueva → *escribir* (verde), heat/recencia → *recordar* (cian), sinapsis nueva →
    *relacionar* (ámbar), reposo azul tenue. Nada fabricado.
  - **Empaquetado sin runtime de build.** El bundle (`dashboard.bundle.js`) se **commitea** y lo
    consume `go:embed`; compilar Go **no** necesita node. El toolchain (esbuild + three) queda en
    `package.json`/`.gitignore` y solo se corre al tocar el frontend. `dashboard.go` sirve el bundle
    en `/dashboard.bundle.js` same-origin sobre loopback (sin CDN, offline).

## [0.92.0] - 2026-07-15

> **El índice no necesita una segunda copia del texto.** La FTS guardaba su propio duplicado del
> contenido; ahora lo LEE de la tabla base. Menos disco, misma búsqueda — con un cuidado: el índice
> pasa a depender del rowid, y el rowid lo puede mover un VACUUM.

### Changed

- **La búsqueda de texto (FTS) pasa a EXTERNAL-CONTENT (Track 16 F3).** `observations_fts` ya no
  guarda su propia copia del contenido: lo referencia desde `observations` por rowid
  (`content='observations'`). Elimina la duplicación del texto en disco (el contenido pesaba dos
  veces). Migración **v17**, idempotente (una base fresca ya nace external-content; una vieja se
  convierte y se re-puebla con `'rebuild'`).
  - **El pivote de diseño — VACUUM.** `observations` no tiene `INTEGER PRIMARY KEY`, así que su
    rowid lo **renumera un VACUUM**, y la FTS external-content indexa por rowid. Sin remediarlo, cada
    VACUUM dejaría la búsqueda devolviendo basura **en silencio**. `Compact` ahora **reconstruye la
    FTS después de vacuumear** (único sitio que vacuumea la base viva; el backup DR usa `VACUUM INTO`
    a un archivo aparte, que no toca los rowids del origen).
  - **Detección más fina.** El `integrity-check` del doctor pasa a la forma `rank=1`, que valida no
    sólo el b-tree interno sino que los tokens **coincidan con el contenido** — atrapa el desync por
    rowid que el check básico no ve. El repair usa el comando `'rebuild'` (relee de la tabla base).
  - Triggers external-content (el `'delete'` toma los valores viejos de `old.*`) y queries que joinean
    por `rowid`. Cubierto por tests adversariales: sobrevida a VACUUM, update/delete re-indexan, y la
    conversión desde la FTS regular.

> **Crecer para siempre no es un plan.** El olvido archiva lo que cae bajo un umbral de
> saliencia, pero un tenant de alto ingest cuyas memorias nunca bajan del umbral crece sin
> techo. La retención por tiempo (purga por edad) tampoco lo acota si el ingest supera a la
> purga. Faltaba el bound que SIEMPRE aplica: una cuota.

### Added

- **Cuota de crecimiento por tenant (Track 16 F3).** Un techo configurable de observaciones
  **activas por `project_id`** (`maintenance.max_active_per_project`): cuando un proyecto lo
  supera, el mantenimiento archiva sus memorias **más frías** (menor saliencia, la misma
  fórmula del olvido) hasta volver bajo el techo. Es lo que acota de verdad el crecimiento del
  cerebro central 24/7, donde ni el olvido por umbral ni la purga por edad lo garantizan.
  - **Por tenant y no global:** en el central multi-tenant, una cuota global dejaría que un
    proyecto ruidoso desalojara la memoria de otro. Cada `project_id` se acota por separado.
  - **Evicción = archivar (reversible),** no borrar: la purga por edad hace el borrado duro
    después, con su período de gracia. La cuota nunca pierde memoria de forma irreversible.
  - **Protecciones:** respeta la importancia deliberada (cuenta para el techo pero no se
    evicta) y el período de gracia; y **nunca evicciona memoria sin sincronizar** (fila de
    outbox no `sent`) — archivarla podría dejarla varada sin llegar al central.
  - Streaming con un heap acotado a lo que sobra del techo: memoria O(excedente), no O(activas)
    — no re-materializa el corpus. Off por default y no se enciende en un upgrade silencioso
    (mismo cuidado que la purga); `musubi init` lo escribe visible y editable.

### Security

- **SAST en CI: gosec (Track 16 F4).** Un gate de análisis estático de seguridad que complementa
  a `govulncheck`: éste atrapa dependencias con CVE conocido; gosec atrapa **patrones inseguros en
  nuestro propio código** — SQL interpolado, crypto débil, TLS sin verificar, credenciales
  hardcodeadas. Hoy el codebase da **cero hallazgos reales**; el gate lockea ese cero y atrapa la
  regresión futura.
  - Ruleset **curado** (severity≥medium, confidence=high) que excluye las clases de FP sistemático
    o comportamiento **de diseño** en una CLI/herramienta de provisioning (lectura de archivos que
    el operador nombra, ejecución de `git`/`tailscale`, `IN()` con placeholders `?`, permisos
    deliberados en artefactos compartibles). Cada exclusión está justificada en el workflow.
  - El único hit del ruleset curado (`VACUUM INTO`, que no admite parámetros enlazados y usa un
    destino que construimos nosotros) queda documentado con un `#nosec G201` en el código.

- **Redacción de secretos a paridad de gitleaks (Track 16 F4).** El redactor model-free (la guarda
  que tapa credenciales antes de que la captura automática las mande a la memoria COMPARTIDA) suma
  las reglas de forma de más valor que le faltaban frente a gitleaks — priorizando las relevantes al
  propio proyecto:
  - **Claves de proveedores de IA** (`sk-ant-` Anthropic, `sk-proj-`/`sk-` OpenAI) — las usa el
    propio Musubi; una filtrada en la memoria de equipo sería grave. El separador `-` las distingue
    de las de Stripe (`sk_live_`).
  - **Token de bot de Telegram** (`\d{8,10}:…`) — lo usa el gateway de chat.
  - GitHub PAT fino (`github_pat_`, que la regla `gh[opsur]_` no cubría), GitLab (`glpat-`), Slack
    (`xox…` + webhooks), SendGrid, Twilio, npm.
  - **Contraseñas en connection strings** (`scheme://user:PASS@host`): las passwords humanas son de
    BAJA entropía, así que el catch-all no las veía, pero un `postgres://u:p@host` filtrado es una
    fuga real. Se redacta sólo la contraseña.
  - El catch-all de entropía sigue cubriendo los formatos desconocidos; esto agrega CERTEZA sobre
    los prefijos distintivos (que además pueden ser cortos o de baja entropía). 11 casos de test.

### Changed

- **Benchmarks a escala (n=100k) en CI (Track 16 F3).** El `bench-guard` de cada push valida el
  escalado de memoria a 1k/10k; faltaba confirmar la asíntota a escala real — justo donde la
  auditoría marcó los riesgos (Consolidate materializando el corpus, IVF ">10k jamás
  benchmarkeado"). `BenchmarkMaintain` ahora también corre a 100k bajo `MUSUBI_BENCH_SCALE`, y un
  workflow **`bench-scale`** (semanal + a demanda, no en cada push por el costo de sembrar 100k
  filas) vigila que la búsqueda vectorial siga sublineal y el mantenimiento sub-cuadrático a 100k.
  Es un canario de escala, no un gate de PR.

## [0.91.0] - 2026-07-15

### Added

- **`musubi cerebro` — el canal de la sala de mando.** Un servidor MCP por **stdio** que no tiene
  memoria propia: **reenvía** cada llamada al cerebro central por HTTP, poniendo la credencial él
  mismo. Es lo que convierte a Musubi en sala de mando *en la práctica*: desde su repo se consulta
  la memoria de **todos** los proyectos, sin replicarla.
  - **Por qué no un `"type": "http"` en el `.mcp.json`:** el cliente MCP-sobre-HTTP de Claude Code
    hoy **no envía los `headers`** que declarás
    ([#48514](https://github.com/anthropics/claude-code/issues/48514)) — la credencial nunca llega — y
    además intenta OAuth **por descubrimiento** en vez de por un 401
    ([#46879](https://github.com/anthropics/claude-code/issues/46879)), terminando en un
    `SDK auth failed` que no dice nada. Acá el header **lo pone Musubi**: no hay nada que el cliente
    pueda omitir. Y stdio no tiene OAuth ni sesión: es un pipe.
  - **Ver todo ≠ replicar todo.** El canal **consulta** el cerebro en vivo; no baja la memoria de los
    demás proyectos a la base local. Si lo hiciera, el recall del repo competiría para siempre con
    ruido de producción ajena. Dos planos: el daemon local (acotado, rápido, offline) y este canal
    (federado, en vivo).
  - `MUSUBI_CENTRAL_URL` + `MANDO_MUSUBI_TOKEN` (o `--url` / `--token-env`). Fail-closed: sin token no
    arranca, en vez de encadenar 401 silenciosos.

### Fixed

- **Una línea de stdin ilegible ya no desaparece en silencio.** El canal distinguía mal *"no parsea"*
  de *"es una notificación"* (que, por diseño, no lleva respuesta): una línea corrupta se **tragaba**
  y el cliente esperaba **para siempre** una respuesta que nunca iba a llegar. Ahora un JSON ilegible
  se contesta con un parse error (`-32700`).
  - Lo destapó un **BOM UTF-8**: cualquier productor que escriba UTF-8 "con firma" (PowerShell, por
    caso) antepone `\xef\xbb\xbf` al stream, y esa marca **invisible** rompía la **primera** línea —
    que es justo el `initialize`. El síntoma era desconcertante: el canal contestaba `tools/list` pero
    no el handshake. El BOM ahora se tolera; el bug de fondo (tragarse lo ilegible) era el grave.

> **Ver todo y poder tocar todo son dos cosas distintas.** El rol las tenía colapsadas en un solo
> enum, y por eso el cerebro central no sabía expresar ni una sala de mando ni una cabina.

### Added

- **Alcance y autoridad son ejes independientes.** Un principal ahora declara **qué VE**
  (`read: own|all`) y **qué ESCRIBE** (`write: none|own|any`) por separado. El `role` sigue
  funcionando como atajo — `reader`/`writer`/`admin` significan exactamente lo mismo que antes — pero
  ya no es la única forma de hablar. Esto habilita las dos identidades que el enum **no sabía decir**:
  - **Sala de mando** (`read: all` + `write: own`) — el repo de **Musubi**: ve los 3 proyectos para
    diagnosticarlos, pero su escritura **se clava en su propio tenant**, aunque declare otro. Antes
    había que darle `admin`, que además lo dejaba escribir dentro de la memoria de producción ajena.
  - **Cabina** (`read: all` + `write: none`) — el **CRM** y el **gateway**: ven todo, no mutan nada.
    Antes no existía el término medio: `reader` sólo veía su tenant y `admin` escribía en todos.
  - `musubi token new --read all --write own`; `musubi token list` ahora muestra **VE** y **ESCRIBE**
    (las capacidades efectivas), porque una cabina y un reader normal comparten rol y no se
    distinguían.

### Security

- **Una escritura sin proyecto ya no cae "sin atribuir".** Una fila con `project_id` vacío es
  visible desde **TODOS los tenants** (el filtro de recall la deja pasar). Un `admin` que guardaba
  sin declarar proyecto la producía **en silencio** — medido en el cerebro real: **2 filas de test
  contaminando los 3 proyectos**. Ahora se rechaza (`-32001`): quien escribe con `write: any` debe
  **declarar** el proyecto, y quien tiene `write: own` lo toma de su credencial.
- La guarda fail-closed del registro pasó a expresarse sobre los **ejes** y no sobre el rol: quien
  **escribe lo suyo** debe **tener** lo suyo, y quien **lee lo suyo** también. Sin `project_id`, el
  primero escribiría sin atribuir y el segundo vería todos los proyectos.
- **La trampa del cero:** el valor cero de un string es `""`, así que un `Principal` construido a
  mano tendría capacidades vacías y caería en un comportamiento accidental (un `reader` podría
  **mutar**; un `admin` dejaría de ser federado). Las capacidades **caen al rol** cuando no están
  declaradas, y hay un test que lo fija. Tres tests existentes lo destaparon antes del merge.

> **Ante la duda, no se tira la memoria.** Reintentar de más es barato y acotado; perder una
> observación es irreversible. La clasificación de fallos del sync tenía esa asimetría al revés.

### Fixed

- **El sync ya no manda memoria a dead-letter por un fallo TRANSITORIO del central.** La
  clasificación de errores JSON-RPC era una **lista negra de uno**: *todo* permanente salvo la cuota
  (`-32002`, carveada a mano en Track 19). Así, un **`-32603` del central —típicamente un
  `SQLITE_BUSY` por contención—** mandaba la observación a **dead-letter sin reintentar una sola
  vez**: memoria perdida en silencio, con el `sync_status` en verde. Y salta justo en el **sync
  inicial grande de una máquina nueva**, que es cuando más contención hay y cuando menos perdonable
  es perder memoria.
  - Ahora la lista es de **PERMANENTES** (`-32700`, `-32600`, `-32601`, `-32602`, `-32001`): los
    errores donde el central **rechazó** el pedido y reenviarlo idéntico no cambia nada. Un fallo
    **interno** suyo, o un código que no conocemos, nace **transitorio** — el outbox reintenta con
    backoff y corta solo al llegar a `max_attempts`.
  - Arregla la **forma**, no un caso más: la cuota se había carveado caso por caso; cualquier código
    nuevo del central ya nace del lado seguro.
  - El mismo bug estaba en el camino del **pull**: un fallo interno del central cortaba la bajada
    entera y la máquina se quedaba sin memoria.
  - Lo dead-letereado se recupera con `musubi_sync_requeue` — no hace falta reconstruir nada.

- **El cerebro central dejó de encolar lo que nunca iba a enviar.** El central es un nodo
  **terminal**: sirve memoria, pero no tiene upstream a dónde empujarla. Aun así encolaba en su
  outbox **cada observación que ingería**, y esas filas quedaban `pending` **para siempre** (el drain
  ni arranca sin sync configurado). No era un loop —nunca enviaba nada— pero acumulaba una fila
  muerta por observación: **571 en el cerebro real**. Peor que el peso muerto: hacía que
  `sync_status` contra el cerebro reportara *"571 pendientes de envío, 0 enviadas"*, una **señal de
  salud que miente** — ya mandó a investigar un problema inexistente dos veces. Ahora un nodo que
  sirve **sin sync saliente** no encola. Un cliente encola como siempre; un central encadenado a
  otro central (con sync configurado) también.

> **Aislar la atribución no es aislar la escritura.** Track 17 cerró la *falsificación* (un writer no
> puede declarar que su memoria es de otro proyecto). Faltaba lo simétrico: que tampoco pueda
> **corromper** la memoria de otro proyecto que ya existe.

### Security

- **Un writer del proyecto A ya no puede pisarle el contenido a una observación del proyecto B.** El
  UPSERT por id **no pisa `project_id`** (correcto: un re-save no debe reasignar la atribución) — pero
  tampoco había ninguna guarda que impidiera el UPSERT en sí. Resultado: conociendo un id ajeno, un
  writer acotado escribía dentro del tenant de otro, y la fila quedaba **atribuida a su dueño con
  contenido ajeno**. Y los ids ajenos **se filtran**: cualquier cliente que alguna vez sincronizó con
  la credencial equivocada se los bajó. Ahora la escritura cross-tenant se rechaza (`ErrCrossTenant`,
  `-32001` en MCP). El caller sin tenant (admin/federado/stdio local) conserva el acceso pleno.
- **El dedup por `content_hash` ya no cruza tenants.** `FindByContentHash` no filtraba por proyecto:
  un writer cuyo contenido coincidía con el de OTRO proyecto recibía **el id ajeno** con
  `deduped=true` y **su observación no se guardaba** — pérdida silenciosa de memoria. Ahora el dedup
  se acota al tenant que escribe (las filas legacy sin atribuir siguen siendo candidatas, para no
  romper el dedup de lo anterior a Track 16).

### Fixed

- **En team mode, los commits capturados ya viajan al cerebro.** La captura guardaba con
  `ScopeLocal` **hardcodeado**: corre en el CLI, que no pasa por el `defaultScope()` del servidor MCP,
  así que `team_mode` ni se miraba. Resultado: **lo único que Musubi captura SOLO era justo lo único
  que nunca cruzaba de máquina.** Medido en la memoria real de este repo: la PC tenía **481**
  observaciones locales y la laptop **70** — unos 400 commits capturados de un lado eran invisibles
  del otro. La memoria *deliberada* era de equipo; la *automática*, de máquina. Al revés del contrato
  del flag, que dice *«la captura de este proyecto es CENTRAL por naturaleza»*.
  - El comentario que lo justificaba (*«nunca shared: C3 no debe filtrar un secreto de un diff»*)
    quedó **obsoleto**: la redacción corre hoy en el **borde a `shared` dentro de `saveObservation`**,
    por cualquier ruta, no sólo vía `promote`. Y la captura guarda subject + body + nombres de
    archivo, **no el diff**.
  - Sin riesgo de duplicados: el id del commit es **determinístico desde su contenido**, así que si
    dos máquinas capturan el mismo commit el central lo **upsertea en la misma fila**.
  - Un proyecto personal (sin `team_mode`) sigue capturando `local`: nada cambia.

- **Una fila que cayó en el tenant equivocado ya no es una trampa silenciosa.** Como el UPSERT
  preserva `project_id`, reenviarla con el token CORRECTO la actualizaba **dentro del tenant ajeno**,
  sin reasignarla y sin avisar. Encontrado en producción: una observación quedó en el tenant de otro
  proyecto por un token mal configurado, y el intento de repararla desde el cliente sólo la reescribió
  en el lugar equivocado. Ahora falla ruidosamente y le dice al caller que use un id nuevo: reasignar
  el tenant de una fila existente sólo puede hacerlo un admin en el central.

## [0.90.0] - 2026-07-13

> **El libro mayor no se tacha.** Un commit es lo que PASÓ; un contrato SDD es lo que se ACORDÓ.
> Ninguno se puede des-hacer — así que ninguna relación puede nacer apuntándolos. Sólo las
> **creencias** (las notas) se reemplazan.

### Fixed

- **Un registro histórico nunca es DESTINO de una relación.** La guarda G3 tenía una excepción —
  *«…salvo que ambos sean de la misma clase»*— que dejaba pasar **commit vs commit** y **contrato vs
  contrato**. Medido sobre las **169 relaciones** de una memoria real: esos pares eran el **20% de la
  cola** y produjeron **CERO veredictos sustantivos**. Los 8 `supersedes` que existen son **todos
  `nota → nota`**. La práctica ya respetaba la regla; el código recién ahora la escribe.
  - La excepción se justificó con *«dos commits pueden ser el mismo commit»*. **Falso**: 16 pares
    commit↔commit, cero duplicados. Los commits son únicos — tienen SHA. Y `supersedes` **oculta** el
    destino: que un commit oculte a otro es **borrar historia**.

### Changed

- **Las tres guardas eran UNA.** G1 (hermanos SDD), G2 (el evento vs el contrato) y G3 se
  descubrieron por separado, en tres PRs, cada una a partir de un ruido distinto. Al quitar la
  excepción, las dos primeras quedan **subsumidas**: sus destinos son históricos por definición. La
  función colapsa a un predicado. **Sus tests siguen verdes sin una línea de cambio** — son a la vez
  la prueba del colapso y la red que impide que se pierdan en silencio.
- **La asimetría se conserva** (y es lo que impide que la regla sea un martillo): se mira **sólo el
  destino**. Un commit `feat: migrar de X a Y` **sí** vuelve obsoleta la nota `usamos X` — es
  evidencia de que la nota envejeció.
- Los tests de `DetectOnly` (M4) se re-apuntan del balde `git-commit` al balde `error-fix`. Para los
  commits la guarda estructural ahora **subsume** a `DetectOnly` (la relación ni siquiera nace), pero
  el flag **sigue siendo load-bearing** en la telemetría, que no es un registro histórico. **Un test
  que cubre un camino ya bloqueado río arriba queda verde para siempre sin custodiar nada.**

## [0.89.0] - 2026-07-12

> **El gist vuelve a servir para lo que existe: decidir.** Un cuarto de ellos no te dejaba decidir
> nada — y la causa era una línea del extractor, no la forma de escribir las memorias.

### Fixed
- **El 24% de los gists no te dejaban decidir nada.** Medido en la memoria real: **110 de 461**
  gists usaban menos de 15 tokens de un techo de **24**, y lo que decían era esto:

  ```
  "SDD tasks — brain-dashboard BACKEND."
  "SDD verify — debate-topology VERDE."
  ```

  **El gist existe para UNA cosa: que el agente decida si vale la pena EXPANDIR la memoria.** Es la
  pieza central del recall por presupuesto. **Uno que no deja decidir es peor que inútil: cuesta
  tokens y te obliga a expandir igual — o sea, a pagar dos veces por lo que debía anticipar.**

  **La causa era una línea:** `Gist()` tomaba la **primera oración y se detenía**. Si esa oración
  eran 8 tokens, **abandonaba 16** sin intentar decir nada más. No era un problema de cómo se
  redactan los contratos SDD: era **del extractor**.

  Ahora el gist **llena su techo** (que no cambia — lo que cambia es que **se usa**), y el `doctor`
  gana una reparación **`stale_gists`** para recalcular los que quedaron viejos. El gist es
  **derivado** de `content`: regenerarlo es **idempotente** y no puede perder nada.
  > **La regla que sonaba prolija resultó ser la peor, y sólo medirlo lo mostró.** El diseño original
  > decía *«nunca truncar una oración a la mitad — un gist cortado tampoco deja decidir»*. Suena
  > bien. Pero con esa regla **sólo mejoraban 39 de 461**, y **no** los que motivaron el cambio: en
  > los peores casos la segunda oración es **larga** y no entra, así que quedaban mudos igual.
  > Truncando la última para llenar el techo: **181 mejoran**.

  **El canje, con el número y no con una intuición:** los gists mudos caen de **24% a 3%**, al costo
  de **~5 items menos** por consulta (de ~39 a ~34 en un presupuesto de 700 tokens). Menos memorias,
  pero **cada una decidible**.

### Added
- **`musubi doctor` detecta y repara los gists que desaprovechan su presupuesto** (`stale_gists`).
  La reparación es **explícita** (`--apply`), nunca un efecto colateral silencioso del arranque:
  reescribir cientos de gists sin que nadie lo pida sería un cambio invisible en la superficie que
  el agente lee.

## [0.88.0] - 2026-07-12

> **El recall deja de repetirse.** Sabía rankear cada memoria por separado; ahora también cuida que
> el **conjunto** que te entrega no sea lo mismo dicho siete veces.

### Added
- **El recall ya no gasta el presupuesto contando lo mismo siete veces (MMR / diversidad).** El
  ranker fusiona **siete señales** y hace bien su trabajo… pero **ninguna mira lo que YA se eligió**.
  Optimiza **relevancia por item**; nadie optimizaba **la utilidad del conjunto** — y el presupuesto
  de tokens es **del conjunto**.

  Medido en la memoria real: una consulta traía **las siete fases SDD** de un cambio, **las siete**
  de otro y 5 de un tercero. Varias sin aportar nada — el gist de `tasks` es literalmente
  *«17 tareas.»*. Y la nota del **principio destilado**, el item más útil, quedaba **6ª, por debajo
  de 5 contratos del mismo cambio**.

  Ahora una candidata que **repite** lo que ya se eligió **baja de posición**. Configurable con
  `memory.mmr_lambda` (default **0.75**); en **1** se apaga y el orden es **bit-idéntico** al de
  antes.
  > **La penalización mide REDUNDANCIA, no similitud** — y esa distinción es todo. El coseno entre
  > dos memorias **cualesquiera** del corpus es **0.60** (medido): parecerse *eso* no es redundancia,
  > es **estar escritas en el mismo idioma**. Penalizar sobre coseno crudo castigaría a **todo** por
  > igual. La escala va de **0 en esa línea de base** a **1 en el duplicado exacto**.
  >
  > **MMR reordena, NO descarta.** Un item redundante **baja**; si el presupuesto alcanza, **sigue
  > estando**.

  **Honestidad sobre la magnitud:** en el λ seguro (0.75) la redundancia baja **~16%** — es una
  mejora **moderada**, no dramática. El `recall-gate` (R@10) queda **intacto en 0.833** con cualquier
  λ… pero **eso sólo prueba que no daña**: el fixture dorado son documentos **distintos**, sin
  redundancia que penalizar, así que **no puede medir el beneficio**. Ése se midió aparte, sobre la
  memoria real. Por debajo de **λ = 0.72** la diversidad empieza a **promover items sin relación con
  la consulta** — ahí está el límite, y por eso el default no baja de 0.75.

## [0.87.1] - 2026-07-12

> **La v0.87.0 duró un `save`.** El primer uso real de la banda ciega encontró dos defectos en ella
> — y ninguno era un umbral mal puesto: los dos eran **decir una cosa y escribir otra**.

### Fixed
- **Dos defectos que encontró el PRIMER uso real de la banda ciega (v0.87.0).** Un solo
  `musubi_save_observation` — una nota destilando el aprendizaje de la sesión — generó **8
  pendientes**, y una de ellas salió **además** en la banda.

  **El doble aviso.** El diseño decía *«si el par ya es `pending`, no avisar dos veces»*, pero la
  condición escrita fue `coseno >= piso` — y eso es una **proxy equivocada**: a la cola se entra por
  **dos puertas** (léxico **o** coseno). Un par que entró por la **léxica**, con coseno **0.849**
  (justo por debajo del piso), caía igual en la banda. Ahora la banda pregunta con **la misma
  función** que decide la cola: **es su complemento**, no un rango de coseno. Llamarla en vez de
  copiarla es lo que evita que vuelvan a divergir.

  **El veredicto imposible.** Las 8 pendientes eran la nota contra **los artefactos del trabajo que
  la nota resumía** (contratos SDD y commits). Y el único veredicto disponible habría sido *«esta
  nota reemplaza al commit»* o *«…al spec»* — **que no significa nada**: un commit es lo que
  **pasó**; un contrato SDD es lo que se **acordó**. **No se pueden des-hacer.** Pedir un juicio que
  ya está decidido de antemano es, por definición, ruido.
  > **La regla, y su asimetría — que es lo que la vuelve una regla y no un martillo.** Un registro
  > histórico nunca puede ser el **destino** de una relación propuesta por algo de otra clase. Pero
  > **al revés sí importa**: un commit *«feat: migrar de X a Y»* **sí** puede volver obsoleta una
  > nota que decía *«usamos X»* — el commit es **evidencia** de que la nota envejeció. Ese caso se
  > conserva, igual que `commit ↔ commit` y `SDD ↔ SDD` de cambios distintos.

## [0.87.0] - 2026-07-12

> **La memoria deja de ser sólo un archivo y empieza a discutirte.** Hasta acá Musubi sabía detectar
> lo que se **repetía**; ahora también avisa cuando algo puede estar **contradiciendo** lo que ya
> sabía — que es el error que de verdad duele, porque te deja creyendo algo falso.

### Added
- **Musubi ahora te avisa cuando lo que guardás puede CONTRADECIR algo que ya sabía.** Salió de un
  falso negativo **real**: una memoria decía *«NordVPN y Tailscale no pueden coexistir»* y la
  solución posterior lo **dio vuelta** — y Musubi **nunca relacionó las dos**.

  **Por qué se le escapaba, y por qué no bastaba con bajar el umbral.** El piso de coseno del dedup
  (0.85) está calibrado sobre **duplicados** — los casi-idénticos dan ~0.99. Pero **una contradicción
  no es un duplicado**: decir *lo contrario* usa **otras palabras**, así que vive estructuralmente
  **más abajo** en la escala. El detector está afinado para encontrar **redundancia**, y la
  contradicción es su opuesto. **Un solo umbral no puede hacer los dos trabajos.**

  Medido sobre las 436 observaciones reales (94.830 pares): el par que se contradice da coseno
  **0.806** (piso 0.85 ✗) y similitud léxica **0.213** (piso 0.30 ✗) — pasó por debajo de **las dos
  puertas**. Y sin embargo ese 0.806 es **más similar que el 99% de todos los pares**: no era una
  señal débil perdida en el ruido, era de las más fuertes que había.

  Bajar el piso a 0.80 lo habría atrapado… y **triplicado la cola** (medido: ×2.9), o sea ~3
  veredictos extra **por cada memoria nueva**.

  Ahora existe una **banda ciega** propia — `[band_floor, cosine_floor)` — y sus vecinos **se te
  muestran al guardar**, con la pregunta explícita de si algo quedó superado.
  > **MOSTRAR NO ES ENCOLAR — la distinción que resuelve el trade-off.** La falla real no fue que el
  > detector no **decidiera**: fue que **nunca le mostró el par al agente**. Encolar una relación
  > cuesta caro (exige un veredicto y **vive** en la cola); mostrarle los vecinos al que ya está ahí,
  > con el contexto fresco, cuesta **~cero**. Por eso la banda **no persiste nada**: es un aviso, no
  > un compromiso.
  >
  > Y el código que la implementa es **de sólo lectura** — no conoce `UpsertObsRelation`, así que
  > **no puede** crear una relación aunque quisiera. El invariante no depende de que nadie se
  > olvide: es **imposible** llegar ahí.

  Configurable con `conflicts.band_floor` (default **0.80**, medido). En **0** se apaga y el `save`
  responde exactamente como antes. **Límite declarado:** una contradicción con coseno por debajo del
  piso **sigue invisible**, y decidir *si* dos memorias se contradicen sigue siendo del agente —
  evaluar el predicado («¿esto niega aquello?») es el techo semántico de los embeddings estáticos.

## [0.86.4] - 2026-07-12

> **Otro bug que encontró el uso, no el diseño** — y esta vez la feature se quejó de sí misma: los
> contratos SDD de este mismo fix generaron, al guardarse, exactamente el ruido que el fix elimina.

### Fixed
- **La cola de conflictos ya no se llena de ruido que Musubi se fabrica sola.** Medido en la memoria
  real: **14 de 23** relaciones pendientes eran **artefactos del MISMO cambio relacionándose entre
  sí**. El flujo SDD guarda **7 contratos por cambio** (proposal → spec → design → …) y los siete
  describen *el mismo cambio*, así que por construcción se parecen. El detector los veía parecidos y
  pedía un veredicto por cada par. El commit de ese mismo cambio también se parecía a sus propios
  contratos (coseno hasta **0.93** contra su `proposal`).

  Pero un `proposal` y un `design` **no son duplicados: son complementarios**. Ninguno se puede
  borrar sin perder el rastro del razonamiento. Pedir un juicio ahí es pedir que se decida algo que
  no tiene decisión.

  Ahora dos guardas **estructurales** (deciden por el `topic_key`, sin mirar el contenido) evitan
  **crear** esas relaciones: las fases del mismo cambio SDD entre sí, y un `git-commit` contra un
  contrato SDD — el **evento** vs. el **acuerdo**, donde ninguno puede reemplazar al otro. La
  detección entre memorias **comparables** (dos notas, dos commits, un commit y una nota) no se toca.
  > **El daño real no era el ruido: era la erosión.** Una cola llena de falsos positivos **deja de
  > leerse**, y el día que aparezca la contradicción **real** se pierde entre las demás. El dedup
  > semántico vale lo que valga la **credibilidad** de su cola.
  >
  > **Y ninguna guarda oculta memoria.** Es un `continue`, no un `DELETE`: evita *crear* una
  > relación. El peor caso de un falso negativo es una relación **de menos en la cola** — jamás una
  > observación de menos en el recall.

## [0.86.3] - 2026-07-12

> **Un bug que encontró el uso, no el diseño.** Salió al estrenar el dedup semántico de v0.86.0
> contra la memoria real: marcó relaciones contra **dos observaciones del mismo commit**.

### Fixed
- **La captura ya no guarda dos veces el mismo commit cuando mergeás con squash.** Encontrado en la
  memoria real, no en teoría: `musubi capture` guarda el commit de la **rama**, y después el
  **squash-merge** crea en `main` un commit **nuevo** con el **mismo mensaje** más el sufijo `(#123)`
  (y GitHub reescribe el trailer `Co-Authored-By` → `Co-authored-by`). La captura lo veía como nuevo
  y lo **guardaba otra vez**. El dedup por **hash exacto** no lo agarraba: el texto cambió apenas.
  Y es redundante **por construcción** — tras un squash, el commit de la rama **ya no existe** en la
  historia de `main`; el canónico es el del merge.

  Ahora el id de una observación de commit se deriva **determinísticamente** de una **clave
  normalizada** (sin el sufijo `(#NNN)` del subject, insensible a mayúsculas). El gemelo del squash
  cae en el **mismo id** ⇒ **actualiza** la observación existente con el contenido canónico en vez de
  crear un duplicado. **Nada se oculta ni se descarta: se actualiza.** La clave incluye el cuerpo y
  la **lista de archivos**, así que dos commits genuinamente distintos con el mismo título no
  colisionan.
  > **Por qué acá SÍ se resuelve solo, si el track entero insiste en no auto-suprimir.** Un duplicado
  > **semántico** (otras palabras, mismo significado) es una **interpretación** y por eso requiere
  > juicio ⇒ va a `pending` (dedup semántico + gate de novedad). Un gemelo de **squash** es un hecho
  > **estructural**: el mismo commit, mismo cuerpo, mismos archivos, reformulado mecánicamente por
  > GitHub. Es tan seguro como el dedup por hash exacto — y no cuesta un veredicto en cada PR.

## [0.86.2] - 2026-07-12

> **Cierra el track «Semantic Hardening».** Con esto, el camino de reparación de la memoria ya no
> depende de poder leer lo que está roto.

### Fixed
- **El `doctor` ya puede reparar el índice FTS cuando está corrupto — antes fallaba justo ahí (Fase 0
  / P0, track Semantic Hardening).** Lo vivimos en vivo: con la memoria corrupta, `musubi doctor`
  decía `db_integrity: corruption ... observations_fts (repairable: false)` **y al mismo tiempo**
  `fts_consistency: índice FTS sincronizado ✓ ok`. **El check que VEÍA el problema no lo podía
  arreglar, y el que lo PODÍA arreglar no lo veía.** Tres fallas que se componían en cadena:
  - **La detección era ciega.** `fts_consistency` (el único con reparación y el único en el
    auto-heal) detectaba comparando `COUNT(*)` de las dos tablas. **Un índice internamente corrupto
    puede tener el conteo PERFECTO** ⇒ reportaba `ok`. Ahora corre además el comando **nativo
    `integrity-check` de FTS5**, que valida la estructura interna del índice.
  - **La reconstrucción recorría lo corrupto.** Hacía `DELETE FROM observations_fts`, que **recorre
    el b-tree** ⇒ tocaba las páginas corruptas ⇒ **fallaba justo en el caso que debía curar**. Ahora
    usa **`DROP TABLE` + recrear + re-poblar**: `DROP` libera las páginas **sin leer el contenido**.
  - **El backup previo también.** El auto-heal respalda antes de reparar con `VACUUM INTO`, que **lee
    toda la base** ⇒ fallaba ⇒ **abortaba antes de reparar nada**. Ahora, si `VACUUM INTO` falla, cae
    a una **copia cruda de bytes** (`.db` + `.wal` + `.shm`), que **no parsea páginas** y por lo tanto
    sobrevive a una base corrupta. Se logea explícitamente como **backup de rescate** (puede quedar
    inconsistente si hay escrituras concurrentes): es un backup peor, y aun así infinitamente mejor
    que **ninguno**. El camino feliz no cambia — `VACUUM INTO` se sigue intentando primero.
  > El principio: **nada del camino de reparación puede depender de LEER lo que está roto.** Suena
  > obvio, y sin embargo las tres etapas (detectar → respaldar → reconstruir) lo violaban.

## [0.86.1] - 2026-07-12

### Fixed
- **El ranker del recall dejó de alimentarse de su propia salida (N4, track Semantic Hardening).**
  Cada recall llama a `bumpAccess`, que sobre lo que **acaba de devolver** escribe `last_accessed` y
  `access_count + 1`. Y esas **mismas dos columnas** alimentaban dos términos del score RRF
  (recencia y frecuencia). Lazo cerrado con realimentación positiva: **lo que el ranker mostraba se
  volvía más mostrable** ⇒ se volvía a mostrar ⇒ subía más. La memoria nueva o poco usada no podía
  entrar. Medido sobre la base real (409 observaciones): el **10% más accedido concentraba el 62% de
  todos los accesos**, el **69% nunca se accedió**, y el **31%** ya no rankeaba por su fecha de
  creación.
  - **La recencia ahora mide NOVEDAD** (`created_at`), no *"cuándo te lo mostré"* (`last_accessed`).
    Antes, una memoria de hace 6 meses que el ranker mostró hace 5 minutos le ganaba en "recencia" a
    una escrita ayer.
  - **La frecuencia ahora es una TASA de uso** (accesos ÷ días de vida), no el total acumulado. Para
    seguir arriba hay que ser útil **últimamente**, no haberlo sido **alguna vez**: la ventaja **se
    erosiona** si deja de usarse. El acumulador desbocado pasa a ser un integrador **con fuga**.
  > El criterio que ordena el fix: señales **exógenas** (el ranker **no** las puede cambiar:
  > `created_at`, el texto, el vector) vs **endógenas** (las escribe el ranker: `last_accessed`,
  > `access_count`). Rankear con una señal endógena **sin fuga** es circular por definición.
  >
  > Ojo con el arreglo "obvio": amortiguar la magnitud (p. ej. `log(access_count)`) **no habría hecho
  > nada** — el término es un **rango**, y toda transformación monótona conserva el orden
  > (`rank(log(x)) == rank(x)`). Hay que cambiar el **orden**, y para eso el tiempo tiene que entrar
  > en la cuenta.

  **El olvido NO cambia.** `decay.go` también usa el acceso, y ahí es **legítimo** (refuerzo de
  Ebbinghaus: lo que usás no se olvida) y **no es circular** — el olvido no elige qué mostrar. Dos
  usos del mismo dato: uno correcto, otro circular. Sólo se tocó el **ranking**.

## [0.86.0] - 2026-07-12

> Cierra el track **«Semantic Hardening»**: la última fuente de memoria que no tenía ningún control
> —la que Musubi captura **sola**— ahora también pasa por el dedup.

### Added
- **La memoria que Musubi captura SOLA ahora también pasa por la detección de duplicados (M4, track
  Semantic Hardening).** `DetectRelations` se llamaba **únicamente** desde `musubi_save_observation`
  (lo que el agente guarda **explícito**). Los **dos** caminos de captura **automática** —los commits
  (C3) y el error→fix (C4)— la salteaban por completo: su único dedup era el **hash exacto** del
  contenido, así que **cualquier otra redacción se guardaba como memoria nueva e independiente, sin
  marca ni relación**. Es la fuente de **mayor volumen** de memoria y era la de **menos** control.
  Ahora un commit (o un arreglo) que duplica algo ya guardado queda **marcado** `pending` para que lo
  juzgue el agente.
  > **En el camino automático la detección NUNCA auto-oculta ni descarta nada** (`DetectOnly`). El
  > auto-supersede se dispara con *mismo `topic_key` + léxico alto + más reciente*, y en la captura
  > **todos** los commits comparten `topic_key = "git-commit"` — que ahí es un **balde**, no un tema.
  > Sin esta guarda, dos commits de mensaje parecido (*"fix: typo en el README"* / *"fix: typo en el
  > README del core"*) **se auto-ocultarían entre sí**: pérdida de memoria automática y silenciosa,
  > justo donde no hay ningún agente mirando. Hay un test que **demuestra** ese peligro (sin la
  > guarda, el commit viejo queda `superseded`). Tampoco hay auto-NOOP: el duplicado **se guarda
  > igual** y sólo queda marcado — descartarlo en silencio sería perder memoria.

  Costo medido: **~6 ms** por commit capturado sobre 401 observaciones (la captura ya paga ~1.2 s
  cargando la tabla, y sólo corre cuando hay commits nuevos). `conflicts.enabled: false` lo apaga.

## [0.85.0] - 2026-07-12

> **Track «Semantic Hardening».** Cuatro slices que atacan el *techo semántico* de la memoria
> model-free. Salieron de una investigación (96 agentes) + una auditoría con verificación adversarial
> (13 agentes), y cada uno arregla un **bug medido**, no una intuición. Hilo conductor: la semántica
> **amplía y rutea**, pero **nunca decide sola** qué memoria se oculta.
>
> **Migración: ninguna acción requerida.** Tus vectores se re-generan solos en el primer arranque.

### Added
- **Dedup SEMÁNTICO: el duplicado dicho con otras palabras ya no es invisible (M1/Q4 + M2, track
  Semantic Hardening).** La detección de relaciones era **100% léxica**: el pool de candidatas salía
  sólo de FTS y el veredicto sólo del Jaccard de trigramas. Una observación que **repite algo ya
  guardado pero con otras palabras** nunca entraba al pool ⇒ **nunca se detectaba**. No es que se
  juzgara mal: era **invisible**. Ahora el pool suma un **pool vectorial** (vecinos por coseno) y el
  veredicto usa **las dos señales**, léxica y semántica.
  > **El coseno NUNCA auto-oculta memoria.** Los embeddings estáticos no evalúan predicados: miden
  > *de qué* se habla, no *qué* se afirma — *"usamos X"* y *"ya NO usamos X"* tienen coseno **alto**.
  > Por eso auto-resolver exige **las dos** señales altas (**AND-gate**): el coseno sólo **corrobora**,
  > nunca decide solo. Como el auto-resolve conserva la condición léxica de siempre y le **suma** una,
  > las auto-supresiones son por construcción un **subconjunto** de las de antes: **agregar semántica
  > no puede hacer desaparecer memoria**. El coseno sólo puede volver **visible** (como `pending`, para
  > que lo juzgue el agente) un duplicado que hoy se ignora, o **degradar** a `pending` una
  > auto-resolución que no corrobora. Hay un property test sobre 10.201 combinaciones que lo verifica.

  Umbrales nuevos (`conflicts.cosine_floor` = 0.85, `conflicts.cosine_auto_threshold` = 0.90),
  **calibrados midiendo 77.028 pares reales**, no estimados: dos observaciones **no relacionadas** ya
  dan ~**0.60** de coseno (texto del mismo dominio) y el ruido llega a **0.884**; los casi-duplicados
  reales están en ~**0.99**. ⚠️ Esta escala **no** es la de `memory.vector_floor` (0.30): allá se compara
  *query* vs documento, acá documento vs **documento**. `cosine_floor: 0` vuelve al dedup léxico
  histórico. Sin embedder, el comportamiento es **idéntico** al de siempre.

### Fixed
- **Embeddings — el `model_id` ahora identifica el CONTENIDO de la tabla, no el nombre de su carpeta
  (N1, track Semantic Hardening).** El `StaticProvider` armaba su identidad como
  `"static:" + basename(dir)`: re-destilar la tabla **in-place** (mismo directorio, vectores
  distintos) **no cambiaba el `model_id`**, así que los vectores viejos seguían pareciendo
  compatibles y la búsqueda los comparaba por coseno contra los de la tabla nueva ⇒ **ranking
  corrupto en silencio**, sin error ni aviso. Ahora el id es `static:<nombre>@<checksum>`, con un
  checksum del contenido de `model.safetensors` **y** de `tokenizer.json` (los dos cambian los
  vectores). Una tabla distinta es una identidad distinta, y el contrato de procedencia (F2.2)
  excluye solo a los vectores viejos. Es la **precondición** de cualquier función que confíe en el
  coseno (p. ej. el dedup semántico).
- **Embeddings — re-embedding automático al cambiar de modelo (M3).** El server **avisaba** de que
  había memoria sin vector del modelo actual, pero no lo **remediaba**: el recall semántico quedaba
  apagado hasta que alguien corriera `musubi embed backfill` **a mano**. Ahora el arranque detecta el
  hueco y lo cierra solo, **en background** (no bloquea el arranque: un daemon bajo systemd tiene
  timeout, y re-embeber una base grande tardaría minutos). Logea inicio y fin, así que la degradación
  temporal del recall durante la ventana es **visible**, no silenciosa. Sin hueco, es un no-op.
  > **Migración (one-time, automática):** al actualizar, el `model_id` de tu tabla cambia (ahora
  > lleva checksum) ⇒ tus vectores existentes quedan **excluidos** —invisibles, **no corruptos**— y
  > el re-embedding automático los regenera en el primer arranque. No hay que hacer nada.

- **Recall — la importancia deja de aplastar la relevancia (Q3, track Semantic Hardening).** El score
  era `rrf * importance`, un **multiplicador sin techo**: con `importance:10`, una memoria apenas
  relevante **barría** matches mucho mejores (la importancia *anulaba* la relevancia en vez de
  desempatarla). Ahora la importancia entra como **un término RRF más** (`1/(rrfK+rango)`), a la misma
  escala acotada que recencia/frecuencia/léxico/vector/grafo/co-ocurrencia: **desempata** cuando la
  relevancia es comparable, pero ya **no puede overridear** una relevancia claramente superior.
- **Recall — rangos DENSOS en todos los pools (Q3).** Los rangos rompían empates **posicionalmente**:
  `rankBy` daba 0,1,2… aun a valores iguales, y `lexRank`/`coocRank` usaban la posición del resultado
  FTS (**por rowid**). Así, dos observaciones de relevancia **idéntica** quedaban "a un rango de
  distancia" — indistinguible de una brecha real — lo que hacía imposible que la importancia
  desempatara sin, a la vez, overridear brechas genuinas. Ahora los empates **comparten rango**:
  recencia/frecuencia/importancia vía rango denso, y léxico/co-ocurrencia densos por **score bm25**
  (`ftsSearch` ahora expone el score). Elimina orden arbitrario por rowid y hace la fusión RRF
  determinista ante empates.

- **Recall híbrido — piso de coseno en el pool vectorial (Q1, track Semantic Hardening).** El pool
  vectorial del recall **descartaba la similitud coseno** e inyectaba hasta 50 vecinos con **peso RRF
  pleno sin umbral** (un coseno 0.42 pesaba igual que 0.95), metiendo ruido de baja señal en el
  ranking. Ahora se aplica un **piso** configurable (`memory.vector_floor`, default `0.30`): los
  vecinos por debajo se descartan **antes** de entrar al ranking. `vector_floor: 0` restaura el
  comportamiento histórico (sin piso). Solo afecta el recall híbrido (con vector de query); el recall
  léxico queda idéntico.
- **Recall — degradación elegante ante FTS corrupto (Q2, track Semantic Hardening).** Un error de
  **corrupción del índice FTS** tumbaba TODO el recall, aunque hubiera un pool vectorial semántico
  servible. Ahora, ante corrupción (SQLITE_CORRUPT / FTS malformado), el recall **logea y degrada** a
  pool no-léxico (el vectorial y/o el fallback llenan) en vez de abortar; cualquier **otro** error se
  sigue propagando (la degradación se acota a la clase corrupción, para no enmascarar fallos reales).

## [0.84.0] - 2026-07-11

### Added
- **Sync entrante — scheduler cliente · LOOP CERRADO (C5.3b-2 — track captura-automática de equipo).**
  Cierra el loop de memoria de equipo **end-to-end**: `SyncClient.Pull` (POST `musubi_sync_pull` al
  central) + `RunInboundScheduler`/`drainInboundOnce` que baja páginas de la memoria `shared` del
  proyecto, las **ingiere localmente** (anti-loop, sin re-encolar) y avanza un **cursor persistente**
  (`sync:inbound_cursor`). Se arranca en el daemon cuando hay sync configurado **y** `team_mode`.
  Ahora: **capturás en una máquina → fluye al central (C5.2) → baja a las otras (C5.3) → el recall
  local lo surfacea**, offline y sin red en el hot path (pull, no recall federado en vivo → preserva
  local-first).
- **Sync entrante — primitivos (C5.3a — track captura-automática de equipo).** Base del *pull* que
  hará que un proyecto de equipo VEA la memoria del central en cada máquina **preservando
  local-first** (el recall sigue local/offline; un scheduler bajará la memoria `shared` del central a
  la DB local en vez de consultar por red en el hot path). Este slice entrega los dos primitivos del
  engine: **`ListSharedForPull`** (el central lista la memoria `shared` del proyecto de la credencial,
  paginada por cursor `rowid`, aislada por T17-19) e **`IngestShared`** (el cliente persiste una obs
  bajada **SIN encolarla en el outbox** — la garantía **anti-loop**: lo bajado del central no se
  re-sube). El **tool MCP `musubi_sync_pull`** (central, read-only, scopeado por credencial) ya expone
  ese pull; el scheduler entrante + el cursor persistente (client side) son el slice siguiente
  (C5.3b-2).
- **Team-mode: captura auto-central por proyecto (C5.2 — track captura-automática de equipo).** Un
  proyecto con `memory.team_mode: true` hace que una observación capturada **SIN scope explícito** se
  persista como **`shared`** (fluye al cerebro central vía el outbox, con redacción de secretos en el
  borde) en vez de `local`. Es la pieza que hace que la memoria de un proyecto de equipo se comparta
  **sola, sin pedirlo**. Aplica a la captura proactiva del agente (C1) y a error→fix (C4); un scope
  explícito (`local`/`shared`) se respeta como escape hatch. Default **off** ⇒ comportamiento
  histórico (captura local). La captura de commits (C3) queda local por ahora (mayor riesgo de
  secretos en diffs; slice aparte).
- **Atribución por persona en la memoria (C5.1 — track captura-automática de equipo).** Las
  observaciones ganan un campo `author` **derivado de la credencial** (`principal.Name`) y
  **sellado server-side** —el cliente no puede falsificarlo, el central lo re-deriva de su propia
  credencial de sync e ignora el payload—, para que la memoria compartida de un equipo registre
  QUIÉN aportó cada cosa. Migración aditiva **v16** (`ADD COLUMN author`, sin rebuild);
  backward-compat: la captura local/legacy/stdio queda con `author` vacío (comportamiento bit-a-bit
  al previo). Es el cimiento del cerebro de equipo; el **recall ya expone el `author`** de cada
  memoria en su resultado (`json:"author,omitempty"`). El filtrado por autor y el team-mode
  auto-shared llegan en slices siguientes (C5.2–C5.4).
- **Deploy turnkey de Prometheus para el cerebro (`deploy/prometheus/`).** `install-musubi-prometheus.sh`
  (systemd nativo, idempotente, verifica el sha256 del release oficial) levanta un Prometheus que scrapea
  `127.0.0.1:7717/metrics` con el bearer por `credentials_file` (el token no toca la config) y carga las 7
  reglas de `musubi-alerts.yml`, **validadas con `promtool` antes de arrancar**. Cierra el hueco de
  operabilidad de la auditoría: `/metrics` exponía contadores ricos pero nada disparaba sobre ellos.

## [0.83.1] - 2026-07-10

**Track 19 — sellar la clase de tenancy (parche quirúrgico).** La auditoría de re-medición post-Track 18
(veredicto **4.2/5**) encontró **por tercera vez** la misma clase de fuga de lectura cross-tenant en una
superficie no enumerada, más una regresión de durabilidad que introdujo la cuota-ON de v0.83.0. Este
parche cierra ambas y —clave— sella la clase **por contrato** para que no reincida.

### Security
- **`resolve_skills` / `search_skills` aislados por proyecto (T19.1).** `resolve_skills` corría `noCtx` y
  devolvía la telemetría *relevante* (`GetUnresolvedTelemetryLogsForFiles`) SIN scope: un writer del
  proyecto B recibía `file_path`+`error_message`+`suggested_patch` de otros tenants por colisión de
  basename. `search_skills` leía `skill_decisions` federado (behavior-bleed de `rejected` ajenos). Ambos
  pasan a ctx-aware (`GetUnresolvedTelemetryLogsForFilesCtx`, `GetSkillDecisionsCtx`). **Sellado por
  contrato:** `TestReadSurfaceClassIsolation` barre 8 superficies de lectura con datos cross-tenant y
  falla si el marcador del otro tenant aparece; `TestEveryReadOnlyToolClassified` exige que toda tool
  `readOnly` nueva esté clasificada (cubierta por el barrido, o declarada sin lectura scopeada) — así una
  hermana federada no puede colarse.

### Fixed
- **El drain del outbox ya no dead-letterea memoria `shared` cuando el central rate-limita (T19.2).**
  Regresión introducida por la cuota-ON-default de v0.83.0: `classifyResponse` clasificaba **cualquier**
  error JSON-RPC como permanente, así que un `codeQuotaExceeded` (-32002) del central mandaba la
  observación a dead-letter (pérdida recuperable solo con `sync_requeue` manual). Una cuota es un límite
  **temporal**: ahora se trata como transitorio (reintento con backoff). Guard: `TestSyncClientQuotaIsTransient`.

## [0.83.0] - 2026-07-10

**Track 18 — tenancy hardening ("cerrar la clase").** La auditoría de re-medición post-Track 17
(veredicto **4.0/5**, +0.5 sobre 3.5) verificó que Track 17 cerró de verdad los HIGH nombrados,
pero la caza adversarial destapó la **misma clase** de fuga (superficie de lectura sin scope ·
ingest sin redactar · default fail-open) en superficies que el primer informe **no enumeró**. Este
release cierra esos 3 HIGH residuales y una segunda ola de endurecimiento de operabilidad.

### Security
- **Aislamiento de `detect_changes` por proyecto (T18.1).** La 10ª superficie de lectura (readOnly,
  alcanzable por un reader) cruzaba el diff local con la memoria compartida usando el ctx **crudo**:
  `relatedMemory`→`SearchObservationsFTS` leía observaciones federadas y `gistStale`→`GetCodeMemory`
  (variante federada; tras la migración v13 varias filas comparten `path`) comparaba contra el gist
  de **otro** proyecto ⇒ fuga de metadata + staleness falso. Ahora deriva el scope de la credencial
  (`scopedCtx`) y usa `GetCodeMemoryCtx`. Guard: `TestDetectChangesEnforcesProjectScope`.
- **Aislamiento + redacción del subsistema de telemetría/decisiones (T18.2, migración v15).** El
  subsistema escapaba **dos** garantías a la vez: `telemetry_logs`/`skill_decisions` no tenían
  `project_id` (⇒ `resolve_telemetry` leía/resolvía el log crudo de cualquier proyecto; los hotspots
  y decisiones de `insights` sumaban entre tenants), y `log_error`/`resolve_telemetry` escribían
  **crudo** al pozo compartido. La migración v15 agrega `project_id` a ambas tablas (ADD COLUMN, sin
  rebuild); los saves atribuyen por credencial, las lecturas se acotan (`ResolveTelemetryLogAndGetCtx`,
  `GetSkillDecisionsCtx`, `insights` scopeado) y el ingest se redacta antes del embedding. Guards:
  `TestMigrationV15AddsProjectIdPreservingData`, `TestTelemetryAndDecisionsEnforceProjectScope`,
  `TestLogErrorRedactsAndAttributes`.

### Changed
- **Tenancy fail-closed: `reader`/`writer` exigen `project_id` (T18.3).** Un principal reader/writer
  con `project_id` vacío resolvía a scope vacío ⇒ recall **federado** + escritura sin atribuir, y el
  `token new` default (rol writer, proyecto vacío) lo producía en silencio. Ahora `AddPrincipal` y
  `loadPrincipals` lo **rechazan** (solo `admin` puede ser federado, por diseño).
- **Cuota de uso ON por default (T18.5).** `service.quota_per_minute == 0` ahora resuelve a un default
  generoso (600/min por principal, vía `EffectiveQuotaPerMinute`); **negativo** ⇒ sin límite (opt-out
  explícito); `>0` ⇒ ese valor. Protege al central por default sin lastimar el uso normal.
- **`StrictTenancy` + WARNING de arranque en bind remoto (T18.5).** `service.strict_tenancy` (default
  false) hace que un bind no-loopback **exija** un registro de principals real (rechaza el modo
  "legacy admin-federado" = un único bearer con acceso total). Apagado, un WARNING de arranque siempre
  lo hace visible. Además: **unicidad de nombres** de principals al cargar (el nombre es la clave de la
  cuota). Guards: `TestEffectiveQuotaPerMinute`, `TestIsRemoteLegacyTenancy`,
  `TestLoadPrincipalsRejectsDuplicateNames`.

### Added
- **Revocación en caliente del registro de principals (T18.4).** Antes `loadPrincipals` corría una
  sola vez al arranque, así que revocar/dar de alta a un miembro no surtía efecto hasta reiniciar (una
  revocación diferida es un agujero). Ahora un `reloadableRegistry` con `atomic.Pointer` + un goroutine
  que vigila el mtime del archivo (mtime-poll, 0-deps) recarga en caliente; una recarga fallida
  **conserva** el snapshot vigente (fail-safe: un typo no deja al equipo afuera). Guards:
  `TestReloadableRegistryHotRevoke`, `TestReloadableRegistryKeepsSnapshotOnBadReload`.
- **Alertas Prometheus + runbook + gauge de staleness del backup (T18.7).** `/metrics` exponía
  contadores ricos pero nada disparaba sobre ellos (operabilidad reactiva) y un evento de DR quedaba
  no-paginable. Nuevo gauge `musubi_backup_offhost_age_seconds` (-1 si nunca/no configurado);
  `deploy/musubi-alerts.yml` con reglas para los eventos de mayor consecuencia (down, backup stale,
  outbox dead, índice sin entrenar, rechazos de cuota/authz, tasa de error); `deploy/RUNBOOK.md` con
  qué hacer ante cada una. Guard: `TestOperationalStatsBackupAge`.

### Fixed
- **`doctor` detecta el backup off-host que NUNCA funcionó (T18.6).** `musubi doctor` daba VERDE
  cuando el backup off-host nunca tuvo éxito (la marca `.last_offhost` solo se escribe tras un envío
  OK, así que su ausencia era indistinguible de una instancia local). Ahora `deploy/musubi-backup.sh`
  escribe `.last_offhost_error` en cada fallo (y la borra al éxito), y `checkOffhostBackup` avisa si
  hay error sin éxito previo (o más nuevo que el último éxito). Guard: `TestCheckOffhostBackupErrorMarker`.

**Esquema en v15** (`telemetry_logs.project_id` + `skill_decisions.project_id`; la guarda
`ErrSchemaTooNew` protege binarios viejos de la flota). Verde: build + `go test ./...` + lint + CI
cross-platform + recall-gate.

## [0.82.0] - 2026-07-10

### Added
- **Operabilidad 24/7: métricas por-tool + contadores de rechazo + COUNT cacheado en `/metrics` (Track 17, T17.5).**
  Cierra los huecos de observabilidad que marcó la auditoría de cierre. **(1) Métricas por-tool:** el histograma de
  latencia era sólo agregado (no se veía QUÉ tool se llama más, cuál falla o cuál es la más lenta). Ahora, además del
  agregado, se emiten `musubi_tool_invocations_total{tool,result}` y `musubi_tool_latency_seconds_{sum,count}{tool}`
  (avg = sum/count), orden alfabético para un scrape determinista. **(2) Rechazos visibles:** las tools/call negadas
  por **rol** (authz) o **cuota** eran invisibles en `/metrics` (la request HTTP contaba como ok), ocultando abusos o
  clientes mal configurados; ahora `musubi_tool_rejections_total{reason="authz|quota"}` los cuenta. **(3) COUNT
  cacheado + con timeout:** los gauges de dominio re-ejecutaban los `COUNT` O(n) sobre `observations` en **cada**
  scrape; ahora se cachean con un TTL corto (15s) y los `COUNT` corren con un deadline (5s) para que una base lenta no
  cuelgue el scrape (best-effort: si vence, se omiten los gauges ese ciclo). Guards: `TestServerMetricsToolHistogram`
  (por-tool + rechazos), `TestDomainGaugeCacheTTL`.
- **`musubi embed backfill`: re-embeber el histórico (Track 17, T17.3).** Al encender la memoria semántica sobre una
  base con observaciones previas —o al cambiar de embedder— esas observaciones quedaban SIN vector de la procedencia
  actual y eran **invisibles** para el recall semántico para siempre; `WarnOnEmbedModelSwitch` avisaba del hueco pero
  no ofrecía remedio. El nuevo subcomando `EmbedBackfill` recorre las observaciones activas sin vector del modelo
  actual (sin fila en `embeddings` o con `model_id` distinto), las re-embebe con el embedder resuelto (mismo que
  serve/daemon), reconstruye el índice IVF una vez y actualiza la marca de modelo. Es **idempotente y resumible**
  (una fila ya re-embebida no se vuelve a listar). Sin semántica encendida ⇒ mensaje claro y salida. Guards:
  `TestEmbedBackfillReembedsHistory`, `TestEmbedBackfillSkipsEmptyVectors`.
- **Gate de calidad R@10 del recall semántico en CI (Track 17, T17.3c).** El harness `recalleval` medía léxico vs
  semántico con la tabla POTION real pero `TestSemanticVsLexicalReal` **sólo logueaba** el reporte (y se salteaba en
  CI): la calidad del recall no era un contrato defendido, sólo una medición de una vez. Ahora el test **asserta** un
  piso: híbrido **R@10 ≥ 0.80** (medido 0.833; léxico 0.750) y híbrido ≥ léxico (el win semántico debe ser aditivo).
  Nuevo job `recall-gate` en CI que **cachea** la tabla (~488MB, SHA-256 pinneado; sólo se baja en cache miss) y corre
  la evaluación con `MUSUBI_POTION_DIR`. Atrapa una regresión real (bug en el tokenizer Unigram, en el ranking híbrido
  o en la tabla) que degrade el recall — con el mismo molde de ratchet que el piso de cobertura y el `bench-guard`.

### Fixed
- **Procedencia de vector real por-modelo: `ollama`/`openai` ya no mezclan modelos en silencio (Track 17, T17.3).**
  El `model_id` que estampa la procedencia del vector salía de `Provider.Name()`, que para `ollama`/`openai` devolvía
  la **constante** `"ollama"`/`"openai"` — así, dos modelos distintos de **igual dimensión** bajo el mismo provider
  (p.ej. `nomic-embed-text` vs `mxbai-embed-large` a 768) compartían `model_id` y se **mezclaban** en la búsqueda por
  coseno, corrompiendo el recall en silencio (la única guarda previa, por dimensión, no los distinguía). Ahora
  `Name()` incluye el modelo (`"ollama:<model>"` / `"openai:<model>"`), de modo que la regla de homogeneidad los
  separa. `static` ya era correcto (incluía la tabla). *Nota:* tras actualizar, los vectores `ollama`/`openai` viejos
  quedan con la procedencia antigua y salen del recall hasta correr `musubi embed backfill` (arriba).

### Changed
- **DR off-host segura por default + dead-man's-switch + test de restore en CI (Track 17, T17.4).** Cierra el
  hallazgo **CRÍTICO** de la auditoría (perder el disco del cerebro central = perder toda la memoria compartida),
  que seguía abierto porque el backup off-host era un **no-op silencioso**. Tres cambios: **(1) fallo-cerrado** —
  `deploy/musubi-backup.sh` con `BACKUP_REMOTE` vacío ahora **falla** (exit≠0 ⇒ la unidad systemd queda `failed` y
  se ve en `systemctl status`) en vez de reportar "éxito" dejando el snapshot solo-local; el modo local-only se
  acepta **explícitamente** con `BACKUP_ALLOW_LOCAL_ONLY=1`. **(2) dead-man's-switch** — tras cada envío off-host
  exitoso el script deja una marca `.last_offhost`; un nuevo check de `musubi doctor` (`offhost_backup`) **avisa**
  (warning, no error; no afecta `readyz`) si esa marca envejece > 48h (el timer dejó de shipear). Marca ausente ⇒
  ok (no genera falsos positivos en máquinas de desarrollo sin timer). **(3) test de restore en CI** —
  `TestBackupToProducesRestorableSnapshot` toma un snapshot (`VACUUM INTO`), lo **restaura** como base nueva y
  verifica `integrity_check` + esquema + datos de las 3 familias (observación/hecho/código): "tenemos backups"
  pasa de afirmación no verificada a camino ejercitado en cada corrida. Verificado end-to-end con binario real
  (fallo-cerrado / escape hatch / envío + marca). *Nota de despliegue:* el servidor con `BACKUP_REMOTE` vacío
  empezará a fallar el timer hasta configurar un destino remoto o setear `BACKUP_ALLOW_LOCAL_ONLY=1`.

## [0.81.0] - 2026-07-10

### Fixed
- **Invalidación por cardinalidad cross-tenant del grafo de hechos — corrección de correctitud (Track 17, migración
  v14).** Con `UNIQUE(from_id, predicate, to_id)`, la invalidación por cardinalidad de un predicado **funcional**
  (single-valued: `works_at`, `estado_actual`…) cruzaba proyectos: en un cerebro central compartido, guardar
  `(Ana, works_at, Acme)` desde el proyecto A **cerraba la ventana** de `(Ana, works_at, Globex)` viva en el
  proyecto B (un tenant mutaba silenciosamente la verdad de otro). La migración v14 reconstruye `relations` con
  `UNIQUE(from_id, predicate, to_id, project_id)` (`project_id NOT NULL DEFAULT ''`, filas legacy → `''`), y la
  invalidación se acota **estrictamente** al proyecto de origen. Además el mismo triple ya puede coexistir entre
  proyectos (antes colisionaba en el `ON CONFLICT`).
- **Colisión cross-tenant de la memoria de código (`code_memory`) — corrección de correctitud (Track 17, migración
  v13).** `code_memory` tenía `PRIMARY KEY(path)`, así que en un cerebro central compartido dos proyectos con el
  mismo `path` (p.ej. `internal/auth.go`) **colisionaban** en el `ON CONFLICT(path)` y se **pisaban el gist** entre
  sí. La migración v13 reconstruye la tabla con `UNIQUE(path, project_id)` (`project_id NOT NULL DEFAULT ''`, filas
  legacy → `''`), de modo que cada proyecto tiene su propia entrada por archivo.

### Security
- **Aislamiento (parcial) de `musubi_insights` por proyecto (Track 17, T17.1c).** `insights` reportaba los counts de
  observations (`total`/`active`/`archived`) de **todos** los proyectos, filtrando el **volumen** de la memoria ajena.
  Ahora `InsightsCtx` acota esos counts al proyecto de la **credencial** (mismo `scopeClause`); `admin`/stdio ⇒
  federado. Es un aislamiento **parcial deliberado**: los hotspots de errores (`telemetry_logs`) y las decisiones de
  skills (`skill_decisions`) siguen federados porque sus tablas **no** tienen `project_id` (scopearlas requeriría otra
  migración; diferido, bajo riesgo). Con esto **todas las superficies de lectura respaldadas por `observations`/
  `relations`/`code_memory` quedan aisladas** — cierra el HIGH de cross-project bleed de la auditoría de cierre. Guard:
  `TestInsightsCtxScopesObservationCounts`.
- **Aislamiento del grafo de hechos (`recall_facts` / `entity_context` / `fact_path`) por proyecto (Track 17,
  T17.1b-4, migración v14).** La última superficie de lectura sin aislar: el recorrido del grafo devolvía hechos de
  **todos** los proyectos. Ahora `SaveFactFrom` atribuye la arista al proyecto de la **credencial** y un helper único
  (`liveFactFilter`) **pliega el scope de proyecto dentro del filtro bi-temporal** que comparten las tres superficies
  de traversal —BFS (`expandFrontier`), recall asociativo (PageRank) y camino más corto (`pathNeighbors`)—, de modo
  que las tres quedan scopeadas por un solo punto de cambio. `entity_context` acota además la parte de **prosa**
  (`observationGistsCtx`). Las **entidades** siguen siendo globales (se comparten los nodos; sólo las aristas se
  atribuyen). `recall_facts`/`entity_context` pasaron a ctx-aware y `save_fact` deriva el origen de la credencial;
  `admin`/stdio ⇒ federado. Guards: `TestFactsReadNoBleed`, `TestFactsCardinalityPerProject`,
  `TestFactPathProjectScope`, `TestFactsPageRankProjectScope`, `TestEntityContextProjectScope`,
  `TestMigrationV14RebuildsRelationsPreservingData`.
- **Aislamiento de `musubi_recall_code` por proyecto (Track 17, T17.1b-3).** Sobre la migración v13 (arriba):
  `SaveCodeMemoryFrom` atribuye el gist al proyecto de la **credencial** (no a un espacio global) y
  `GetCodeMemoryCtx` acota la lectura al proyecto del principal, prefiriendo su propia fila sobre la sin atribuir.
  `musubi_save_code`/`musubi_recall_code` pasaron a ctx-aware. `admin`/stdio ⇒ federado. Guard:
  `TestCodeMemoryProjectIsolationAndNoCollision`.
- **Aislamiento de `musubi_conflicts` por proyecto (Track 17, T17.1b-2).** Extiende el aislamiento multi-tenant a
  la superficie de conflictos de memoria: antes `musubi_conflicts` devolvía las relaciones pendientes de TODOS los
  proyectos. Ahora `PendingObsRelationsCtx` hace `JOIN` a `observations` por el `source_id` y filtra por el
  `project_id` **derivado de la credencial** (mismo `scopeClause` que las demás superficies); `admin`/stdio ⇒
  federado. `musubi_conflicts` pasó a ctx-aware. Sin migración (aprovecha el `project_id` que ya vive en
  `observations`). Guard: `TestConflictsEnforcePrincipalScope`.
- **Redacción de TODO ingest al central: `save_fact` y `save_code` ya no escriben secretos crudos (Track 17, T17.2).**
  La auditoría de cierre encontró que la redacción forzada server-side (`forceRedact`) cubría **solo**
  `save_observation` — `save_fact` (subject/predicate/object) y `save_code` (gist/symbols) escribían contenido
  **crudo** al pozo compartido, recuperable por `recall_facts`/`recall_code`, mientras el `Threat_Model` lo declaraba
  falsamente como "redacta TODO ingest". Ahora un helper único (`redactIfForced`) pasa **las tres** tools por la
  redacción cuando el bind es no-loopback (el central). Además: en `save_observation` el contenido se redacta
  **ANTES** de computar el embedding (el vector at-rest ya no se deriva del secreto crudo) y el `topic_key` también
  se cubre. El `Threat_Model.md` se corrigió para reflejar el alcance real **y** advertir que la redacción es
  **best-effort heurística** (reduce, no garantiza; un secreto corto o de baja entropía puede escapar), no una
  garantía dura. Guard: `TestForceRedactCoversAllIngest`. En loopback local el contenido queda crudo (el dev lo necesita).
- **Atribución de escritura por credencial: se cierra el write-poisoning cross-tenant (Track 17, T17.1b-1).**
  Complementa T17.1a (aislamiento de LECTURA) con su contracara de ESCRITURA: `musubi_save_observation` confiaba en
  el `project_id` que declaraba el cliente, así que un `writer`/`reader` acotado a un proyecto podía atribuir una
  observación a OTRO proyecto (o dejarla sin atribuir, visible para todos), evadiendo el aislamiento recién
  cerrado. Ahora el origen se **deriva de la credencial** (`principalFrom(ctx)`): un principal no-admin siempre
  escribe atribuido a SU proyecto; se ignora el `project_id` de los args. El origen explícito se respeta solo para
  **admin/legacy** (ingest del central, para quien se diseñó la variante `*From`). `musubi_save_observation` pasó a
  ctx-aware (`countingSaveCtx`). Guard: `TestWriteAttributionFromPrincipal`.
- **Aislamiento multi-tenant: se cierra la fuga de CONTENIDO cross-project (Track 17, T17.1a).** La auditoría de
  cierre encontró que el scope por-credencial estaba cableado en UNA sola superficie de lectura (`musubi_recall`):
  las demás consultaban la memoria SIN filtro de proyecto, así que un principal acotado a un proyecto leía el
  contenido crudo de TODOS. Esta unidad cierra las 3 superficies que devuelven contenido completo —
  `musubi_search_keyword`, `musubi_search_semantic` y `musubi_memory_expand` (la fuga más grave: hidrataba por id
  arbitrario). Diseño de mínima superficie: un `ProjectScope` que viaja por el **contexto** (`WithProjectScope`/
  `projectScopeFrom`) y un helper SQL `scopeClause` centralizado (mismo criterio que `filterCandidatesByProject`
  del recall: el proyecto pedido + las filas sin atribuir); las funciones de lectura del engine lo aplican sin
  cambiar la firma de `StorageBackend` ni sus ~30 callers. El MCP deriva el scope de la credencial (`recallScopeFor`)
  y lo inyecta (`scopedCtx`); `musubi_memory_expand` pasó a ctx-aware. Ausencia de scope (stdio local / admin /
  legacy) ⇒ federado, comportamiento histórico bit-a-bit. Guards de no-bleed: `TestReadIsolationByProjectScope`
  (motor, las 3 funciones) + `TestReadSurfacesEnforcePrincipalScope` (e2e MCP). **Pendiente en T17.1b:** las
  superficies de metadata/grafo (`recall_facts`, `entity_context`, `recall_code`, `insights`, `conflicts`) y la
  atribución de ESCRITURA por credencial (poisoning).

### Added
- **README en inglés + cross-link ES↔EN (adopción por terceros, Track 16 / Producible F4).** Cierra la Fase 4.
  Toda la documentación estaba solo en español, así que un adoptante anglófono no tenía onboarding. Nuevo
  `README.en.md` — espejo fiel del README (instalación, inicio rápido, cómo funciona, capacidades, herramientas
  MCP, configuración, referencia de CLI, búsqueda semántica, desarrollo, roadmap; diagramas Mermaid con labels
  traducidos y anchors del TOC en inglés). Ambos READMEs llevan un selector de idioma cruzado en el encabezado.
- **CI cross-platform: validación en Windows y macOS (adopción por terceros, Track 16 / Producible F4).** Hasta
  ahora todos los jobs de CI corrían solo en `ubuntu-latest`; los binarios se cross-compilan para 6 targets pero
  nunca se *testeaban* fuera de Linux. Nuevo job `test-cross` con `strategy.matrix: [windows-latest, macos-latest]`
  que corre `go vet` + `go build` + `go test ./...` en cada uno. El job `test` de ubuntu sigue siendo el canónico
  (race + piso de cobertura + govulncheck). Sin `-race` en la matriz a propósito: evita depender de cgo/gcc en
  Windows (el driver SQLite es `modernc` puro Go, así que build y test no necesitan un compilador C).

### Changed
- **`musubi provision` ahora EXIGE `--brain` (adopción por terceros, Track 16 / Producible F4).** Antes `--brain`
  defaulteaba a `100.79.126.62:7717` — la IP del tailnet del AUTOR: un tercero que corría `musubi provision` sin
  flags terminaba sondeando/cableando contra la máquina del autor. Se eliminó ese default personal (constante
  `provision.DefaultBrain`) y ahora `provision` falla con un mensaje claro si falta `--brain`, apuntando a `musubi
  setup` para quien solo quiere setear el proyecto localmente sin un cerebro central. Mismo criterio en los scripts
  de deploy: `deploy/connect-brain-linux.sh` (`BRAIN_IP` requerido vía `${BRAIN_IP:?…}`) y
  `deploy/connect-brain-windows.ps1` (`-BrainIp` requerido con check explícito). Ningún archivo versionado apunta ya
  a infra del autor. (El `repoOwner`/catálogos siguen en `codeabraham16/musubi` — ese ES el repo público real.)

## [0.80.0] - 2026-07-09

### Added
- **`/metrics` accionable: latencia de tools + gauges de dominio (Track 16 / Producible F3.1).** Antes `/metrics`
  solo exponía 4 contadores de requests HTTP por resultado — un operador 24/7 no veía nada del dominio. Ahora,
  manteniendo cero dependencias (renderer Prometheus hecho a mano), agrega: (a) **histograma de latencia**
  `musubi_tool_duration_seconds` (buckets + `_sum` + `_count`, lock-free) y contador `musubi_tool_calls_total`
  {ok,error} por cada `tools/call`, instrumentado en el choke point `handleToolsCall` (cubre stdio y HTTP); (b)
  **gauges de dominio** pulled at scrape vía un accesor nuevo `DbEngine.OperationalStats()`: `musubi_observations`,
  `musubi_embeddings_active`, `musubi_vector_index_size`, `musubi_vector_index_trained`, `musubi_sync_outbox`
  {pending,sent,dead} y `musubi_sync_outbox_oldest_pending_age_seconds` (atraso del sync). Los gauges se exponen
  vía una interfaz opcional (`opStatsProvider`) type-asserted al render, así los backends de test que no la
  implementan no rompen el scrape. Las métricas viven en un `serverMetrics` compartido en el `McpServer`.
- **Benchmark de búsqueda vectorial a escala + guard de sublinealidad del IVF (Track 16 / Producible F3.3).** El
  único benchmark vectorial topaba en n=10 000 (justo el umbral donde el IVF se activa), así que el régimen donde
  el índice debe ganarle al full-scan quedaba sin medir ni proteger en CI. `BenchmarkSearchVector` ahora fuerza el
  entrenamiento síncrono del IVF (mide la ruta indexada de forma determinista, no el full-scan transitorio) y suma
  un caso de escala **n=100 000 opt-in** (env `MUSUBI_BENCH_SCALE`, porque sembrar 100k tarda minutos). Nuevo
  **bench-guard en CI** que corre `BenchmarkSearchVector` a n=1k y n=10k y verifica que la memoria por búsqueda
  crezca SUB-LINEALmente (`B/op(10k)/B/op(1k)` ≈ 3.7x medido, ~√10; umbral 6): una regresión que rompa el IVF y
  caiga a full-scan lo llevaría a ~lineal (~10x). Se mide `B/op` (determinista) y no wall-time, igual que el guard
  de `BenchmarkMaintain`.
- **Cuota de uso por-principal (Track 16 / Producible F3.2).** Cierra la Fase 3. Hasta ahora, una vez autenticado,
  un principal podía hacer llamadas ilimitadas: el único rate-limit era el lockout de auth por-IP (anti fuerza
  bruta del bearer). Nuevo `quotaLimiter` (ventana deslizante en memoria, model-free, espeja `authLimiter`) que
  limita las `tools/call` **por identidad de principal** por minuto, enforced en el choke point `handleToolsCall`
  (tras autorizar por rol, antes de tomar el lock — no serializa los rechazos). Superar la cuota devuelve el nuevo
  código `codeQuotaExceeded` (-32002; la credencial es válida, solo excedió el uso). Configurable con
  `service.quota_per_minute` (0 = sin límite, default). Solo aplica cuando hay principal (serve con registro); en
  stdio local (agente confiable, sin principal) no hay cuota. Distintos principals tienen cuotas independientes.

## [0.79.1] - 2026-07-09

### Fixed
- **`musubi embed pull` ahora cae a IPv4 cuando el IPv6 no tiene ruta (Track 16 / Producible, pulido de Fase 4).**
  En máquinas con IPv6 *configurado pero sin ruta real* (VPN que tuneliza sólo IPv4, red que anuncia IPv6 sin
  salida), la descarga de la tabla fallaba con `dial tcp [2600:…]:443: connect: network is unreachable` porque el
  cliente HTTP por default de Go no reintentaba por IPv4. Ahora el downloader usa un cliente que, ante un error de
  *red/host inalcanzable* (`ENETUNREACH`/`EHOSTUNREACH`), **reintenta forzando `tcp4`** — sin romper las redes
  IPv6-only (que aciertan en el primer intento) ni cambiar el camino feliz. Se detectó dogfooteando el despliegue
  de la Fase 2 en una laptop Linux con IPv6 roto.
- **El mensaje de éxito de `musubi embed pull` ya no manda a editar `config.yaml` de gusto.** Desde 16.2f la
  memoria semántica es *auto-ON* (`resolveEmbedder` detecta la tabla en la ubicación estándar y la enciende al
  reiniciar), pero el mensaje seguía diciendo "para activar, poné `provider: static`…" — heredado y engañoso.
  Ahora, si la tabla quedó en la ruta estándar del modelo default, informa que **se auto-detecta al reiniciar el
  daemon** (sin tocar config); sólo si quedó fuera (por `--out` o un modelo no-default) muestra las líneas de
  `config.yaml` a declarar.

### Security
- **Toolchain de Go a `1.26.5` en CI/release por `GO-2026-5856`** — leak de privacidad en *Encrypted Client Hello*
  de `crypto/tls`, presente en go1.26.4 y corregido en go1.26.5. El pin flotante `1.26.x` se había quedado en
  1.26.4 (retraso del manifest de `setup-go`), así que `govulncheck` empezó a marcar la stdlib; se fija **exacto a
  `1.26.5`** en los tres jobs de `ci.yml` y en `release.yml` para que los binarios publicados se compilen con la
  stdlib parcheada.

## [0.79.0] - 2026-07-08

### Added
- **Captura automática (C3/C4) con embeddings — cierra la Fase 2 (Track 16 / Producible 16.2e).** Las memorias
  auto-capturadas se guardaban con vector `nil`, así que quedaban FUERA del recall semántico (sólo participaban
  las guardadas por herramienta). Ahora, cuando la semántica está encendida, **C3** (commits nuevos, hook `Stop`
  vía `musubi capture`) y **C4** (par error→fix al resolver telemetría) generan su embedding: `runCapture`
  resuelve el embedder con la MISMA auto-detección + degradación elegante que `serve`/`daemon` (`resolveEmbedder`)
  y estampa la MISMA procedencia (`SetVectorModelID`, F2.2) para que los vectores sean homogéneos; C4 usa un
  helper best-effort en el MCP server. Best-effort en ambos: un fallo de embedding devuelve `nil` (ese ítem queda
  léxico) sin romper el turno ni el resolve. Con esto, TODA la memoria —capturada o guardada explícitamente—
  participa del recall semántico. Golden intacto.
- **Memoria semántica ON por default con auto-detección + degradación elegante (Track 16 / Producible 16.2f).**
  Cierra la Fase 2: la semántica se enciende sola cuando se puede y NUNCA rompe el arranque. El entrypoint
  (`serve`/`daemon`) ahora resuelve el embedder con `resolveEmbedder`: si no hay provider explícito (`none`/vacío)
  y existe una tabla en la ubicación estándar (`<workspace>/.musubi/embeddings/potion-multilingual-128M`, la que
  baja `musubi embed pull`), enciende `static` automáticamente; si no hay tabla —o si cargarla falla— cae a
  **recall léxico** en vez de abortar (antes un error de embeddings hacía `os.Exit`). **Medición del gate** (con
  la tabla real de POTION multilingüe, sobre el fixture dorado): la semántica es un **win aditivo** — `R@10`
  0.75→**0.83** (recupera ~1/3 de los relevantes del hueco de vocabulario) **sin regresión** en `R@1`/`R@5`/`MRR`.
  Test de medición repetible (`recalleval`, gated por `MUSUBI_POTION_DIR`). También: fix del flag `--out` de
  `embed pull` (el modelo posicional se extrae antes de parsear, así `embed pull <modelo> --out X` funciona) y
  `.musubi/embeddings/` va al `.gitignore` (tablas de cientos de MB, puro dato). Golden intacto.
- **`musubi embed pull` — descarga turnkey de la tabla de embeddings + carga plana (Track 16 / Producible 16.2d).**
  Hace la memoria semántica *lista para encender* sin pasos manuales. Nuevo comando **`musubi embed pull
  [modelo] [--out DIR] [--mirror URL]`** que baja una tabla estática (por default `potion-multilingual-128M`,
  ES+EN) con **checksum SHA-256 pinneado**, de forma **atómica** (baja a `.part`, verifica tamaño + hash, y sólo
  entonces renombra) e **idempotente** (si ya está con el checksum correcto, no re-descarga). La tabla es PURO
  DATO: se baja una vez en el setup y en runtime no corre ninguna red ni modelo (model-free at inference). El
  flag `--mirror` permite re-hostearla en infra propia (Forgejo/servidor del tailnet) manteniendo el checksum
  pinneado, así un mirror comprometido no puede colar otra tabla. Registro `embedding.KnownModels` con URLs y
  hashes verificados contra el oid LFS de la fuente. Además, `StaticProvider` ahora carga la tabla **PLANA** (un
  solo `[]float32` de vocab×dim en vez de ~500K slices): para la multilingüe (500K×256 ≈ 488 MB) evita cientos de
  miles de headers de slice y mejora la localidad de caché. Golden intacto.
- **Tokenizer Unigram/SentencePiece en Go puro — habilita tablas MULTILINGÜES (Track 16 / Producible 16.2c).**
  El `StaticProvider` sólo sabía tokenizar WordPiece BERT (tablas inglesas). Las tablas multilingües de
  model2vec/POTION (ES+EN reales, p. ej. `potion-multilingual-128M`) usan **Unigram/SentencePiece** —otro
  formato de `tokenizer.json`— así que no cargaban. Este PR agrega un tokenizer Unigram **bit-exacto vs
  HuggingFace, en Go puro y sin cgo**, reproduciendo todo el pipeline: normalizer con `precompiled_charsmap`
  (trie DARTS de SentencePiece) + reglas `Replace` + `Strip`, pre-tokenizer `Metaspace` (▁), y segmentación
  `Unigram` por Viterbi sobre ~500K piezas con log-probs. La única sutileza vs HF (recomposición de secuencias
  descompuestas por grapheme) se resuelve con `NFC` antes del charsmap, que da idéntico resultado para toda
  entrada realista. `static.go` se refactorizó a una interfaz `tokenizer` con dispatch por `model.type`
  (WordPiece | Unigram); el WordPiece existente no cambia de comportamiento. **Validado bit-exacto** contra el
  tokenizer real de POTION multilingüe (test gated por `MUSUBI_SPM_TESTDATA`; referencia `text→ids` en testdata)
  y con unit tests sintéticos del Viterbi/normalizer. Precede a 16.2d (traer la tabla). Golden intacto.
- **Contrato de vector + procedencia — regla de homogeneidad (Track 16 / Producible 16.2b).** El núcleo de
  ROBUSTEZ de la memoria semántica, hecho ANTES de encenderla (S1 de Track 15). Hasta ahora un vector no
  registraba QUÉ modelo lo produjo: al cambiar de embedder, los vectores viejos (otra procedencia) se comparaban
  por coseno con los nuevos y **corrompían el recall EN SILENCIO** cuando compartían dimensión (misma dim, otro
  espacio semántico ⇒ similitudes basura coladas al top). La única guarda previa era por dimensión (el
  dim-guard), que no cubre el borde same-dim; sólo había un *warning* (`WarnOnEmbedModelSwitch`) que recomendaba
  limpiar a mano. Ahora: migración v12 añade `embeddings.model_id`; cada engine estampa la **procedencia** de su
  embedder (`SetVectorModelID`, cableado en `serve`/`daemon` con `provider.Name()`) en todo vector que escribe; y
  la búsqueda exacta (full-scan y por-celda IVF) aplica la **regla de homogeneidad**: sólo compara vectores de la
  MISMA procedencia que el de consulta. Los de otro modelo quedan **excluidos automáticamente** (no se mezclan ni
  corrompen el ranking) — el warning pasa a ser informativo (re-embeber para recuperarlos). Aditiva y
  backward-compat: `''` = procedencia desconocida (legacy y engines sin embedder nombrado) sólo compara contra
  `''`, así que el comportamiento histórico —y todos los tests/bench sin `SetVectorModelID`— no cambian. Golden
  intacto.
- **Harness de calidad de recall (Track 16 / Producible 16.2a).** Primer paso de la Fase 2: una forma
  REPETIBLE y determinista de MEDIR qué tan bueno es el recall, para poder probar con números —no con fe— que
  encender la señal semántica mejora sobre el baseline léxico ANTES de cambiar el default (el audit fue tajante:
  *harness primero*). Nuevo paquete `internal/recalleval`, 100% model-free y sin red: métricas estándar de IR
  (`recall@k`, `MRR`, `nDCG@k`) como aritmética pura + un runner que siembra un motor de memoria temporal con un
  **fixture dorado versionado** (`testdata/golden.json`: 26 docs de memoria de dev ES/EN + 12 queries
  etiquetadas) y evalúa una o más configuraciones de recall sobre el mismo corpus. El fixture incluye a propósito
  queries de **hueco de vocabulario/traducción** (bug↔error, deploy↔despliegue, olvido↔decay) donde el léxico
  debería fallar y la semántica ganar. Baseline medido: **R@10 léxico = 0.75** (el léxico no encuentra el 25% de
  los relevantes ni en el top-10 → margen que la tabla POTION debe cerrar en 16.2c). El camino híbrido (con
  vector) queda ejercitado end-to-end con un embedder sintético para que la integración de la tabla real no
  descubra bugs tarde. Golden de MCP intacto.

## [0.78.0] - 2026-07-08

### Added
- **Hardening del borde del central — lockout + threat model + ACLs (Track 16 / Producible 16.1e).** Cierra la
  Fase 1. (1) **Lockout anti fuerza-bruta**: tras 5 fallos de auth desde una IP, el central la bloquea 60s
  (`authLimiter`, en memoria, model-free) — antes el adivinado online del bearer era ilimitado para cualquier
  peer del tailnet. (2) **Threat model documentado** (`docs/Threat_Model.md`): borde de confianza, activos,
  amenazas→mitigaciones y riesgos residuales — fija qué cubre WireGuard y qué no. (3) **Guía de ACLs de
  Tailscale**: la policy default es allow-all, así que se documenta cómo restringir el puerto del brain a
  dispositivos autorizados (defensa en profundidad, no confiar solo en el rango CGNAT). Cierra los hallazgos
  *low* de superficie HTTP, threat model y least-privilege de red (`audit/2026-07-08`). Golden intacto.
- **Redacción forzada server-side en el central (Track 16 / Producible 16.1d).** La redacción de secretos se
  disparaba por el VALOR del scope declarado por el cliente (`scope==shared`), así que un cliente podía escribir
  un secreto **crudo** en el cerebro compartido mandando `scope=local`. Ahora el central **redacta SIEMPRE**,
  independiente del scope declarado: un bind **no-loopback** (infra compartida) enciende `forceRedact`
  **fail-closed** (no se puede desactivar), y un bind loopback puede optar por `service.force_redact`. Con
  `forceRedact`, todo ingest se trata como `shared` ⇒ la redacción de C2 corre siempre. Cierra el hueco de
  ingest crudo del hallazgo de seguridad (`audit/2026-07-08`). Backward-compatible (stdio local y loopback sin
  el flag: sin cambios); golden intacto.
- **Enforcement del aislamiento por credencial (Track 16 / Producible 16.1c-3).** El cable que cierra la Fase 1:
  el scope del recall se **deriva del principal autenticado** (su `project_id` sale de la credencial, no lo
  auto-declara el cliente). `toolRecall` ahora acota el recall al proyecto del principal — un `reader`/`writer`
  con `project_id` **solo recupera memoria de su proyecto** (más la sin atribuir), mientras un `admin` ve
  **federado** (todos). Sin principal (stdio local) o sin `project_id` ⇒ sin scope (federado, histórico). Con
  esto el aislamiento de 16.1b se **activa automáticamente** por credencial: se cierra el hallazgo **high** de
  cross-project bleed. Lógica pura en `recallScopeFor(principal)`; enforcement e2e verificado (writer ve solo lo
  suyo, admin ve todo). Backward-compatible; golden intacto.
- **CLI `musubi token` — gestión del registro de principals (Track 16 / Producible 16.1c-2).** Hace usable la
  identidad por-principal sin computar hashes a mano: **`musubi token new --name X --project Y --role writer`**
  genera un token opaco (256 bits, prefijo `msb_`), guarda su **SHA-256** en `.musubi/principals.yaml` (nunca el
  token crudo) y lo imprime **una sola vez** para entregárselo al miembro; **`list`** muestra nombre/rol/proyecto
  (jamás el hash); **`revoke --name X`** da de baja. Rechaza nombres duplicados y roles inválidos; crea el
  archivo (600) si falta. El token generado **autentica de una** contra el registro (round-trip verificado).
  Runbook actualizado en `docs/Server_Brain_Onboarding.md`. Golden intacto.
- **Identidad por-principal — registro de tokens + autorización por rol (Track 16 / Producible 16.1c-1).**
  Cierra el core del hallazgo **high** _"un único bearer sin identidad/rotación/revocación/authz"_. El central
  puede cargar un **registro de principals** (`.musubi/principals.yaml` o `service.principals_file`) que mapea
  el **SHA-256** de cada token a `{name, project_id, role}` — credenciales **por-miembro revocables** (borrás la
  línea) en vez de un token compartido. El archivo guarda el **hash**, nunca el token crudo (un leak no da
  credenciales usables). En modo `serve`, cada request se autentica contra el registro y el principal viaja en el
  contexto; el dispatch aplica **authz por rol**: `reader` solo tools de lectura, `writer` lee+escribe, `admin`
  todo (deniega con `codeUnauthorized`). **Backward-compatible**: sin archivo de registro sigue el modo de un
  único bearer, y el `MUSUBI_TOKEN` legacy sigue válido como `admin`; el daemon stdio local no tiene principal
  (confianza local, acceso pleno). Runbook de alta/revocación en `docs/Server_Brain_Onboarding.md`. Golden
  intacto. (El CLI `musubi token new|revoke|list` y el enforcement `project_id`→recall llegan en 16.1c-2/16.1c-3.)
- **Aislamiento por proyecto en el recall + federación opt-in (Track 16 / Producible 16.1b).** Segundo paso de
  la Fase 1: el recall puede acotarse a un proyecto. `RecallOptions` suma `ProjectScope` y `Federate` — con
  scope y sin federate, el recall **descarta los candidatos de otros proyectos** (conserva el proyecto pedido y
  las filas sin atribuir); `Federate` los vuelve a ver todos (el opt-in del modelo "aislado + federación opt-in"
  elegido por el usuario). Implementado como **choke point único**: todos los pools (léxico, vectorial,
  co-ocurrencia) confluyen en `cands`, así que se filtra una sola vez —limpio y sin reescribir 11 queries—
  llevando el `project_id` del candidato en la fila. **Backward-compatible**: `ProjectScope` vacío ⇒
  comportamiento histórico (federado) bit-a-bit; el enforcement por defecto lo cableará la identidad (16.1c).
  Avanza el hallazgo **high** de cross-project bleed (`audit/2026-07-08`). Golden intacto.
- **Atribución multi-tenant — el central preserva el `project_id` de origen (Track 16 / Producible 16.1a).**
  Primer paso de la Fase 1 (cerebro multi-tenant). Antes, al ingerir una observación sincronizada, el central
  estampaba **su propio** `project_id` y descartaba el del proyecto de origen (`saveObservation` usaba siempre
  `e.projectID`, y `toolSaveObservation` ni leía el campo) — sin atribución no hay sobre qué aislar. Ahora el
  handler lee `project_id` del payload y lo **preserva**: nuevas variantes `SaveObservationTypedFrom` /
  `SaveObservationDedupedTypedFrom` estampan el proyecto de ORIGEN (`""` ⇒ el `project_id` del engine, así el
  guardado local no cambia). El sync client ya enviaba el `project_id`; ahora el central lo respeta. Cimiento del
  aislamiento por proyecto (16.1b). Backward-compatible: sin cambios en el recall todavía; golden intacto.

- **DR del cerebro central — backup consistente + off-host + runbook de restore (Track 16 / Producible 16.0b).**
  El nodo central es el único punto donde converge la memoria compartida de todos los proyectos; perder su
  `memory.db` sin backup off-host era irreversible. Ahora: (1) el backup usa **`VACUUM INTO`** en vez de copiar
  el archivo con `io.Copy` tras un `wal_checkpoint` — snapshot *transaccionalmente consistente* en un paso, sin
  lockear el daemon ni arriesgar un estado a medias por escrituras concurrentes; (2) nuevo comando **`musubi
  backup [--out <dir>]`** (puro-Go, no requiere `sqlite3` en el host) que imprime la ruta del snapshot; (3)
  `deploy/musubi-backup.sh` + un **timer systemd diario** (instalado por `install-musubi-brain.sh`) que shipa el
  snapshot **off-host** (`rsync`/`rclone`/`cp`) con retención; (4) **runbook de restore probado** en
  `docs/Server_Brain_Onboarding.md`. Cierra el hallazgo **crítico** «el central no tiene DR» de `audit/2026-07-08`.
- **Fuente única de versión + release verificable (Track 16 / Producible 16.0a).** La versión vivía en dos
  lugares que derivaron: el tag de git (vía `-ldflags -X main.version`) y `cmd/musubi/versioninfo.json` (el
  recurso de Windows), que quedó congelado en `0.57.0.0` con el proyecto en `0.78` porque el paso manual de
  regenerarlo se saltó ~20 releases. Ahora hay un archivo **`VERSION`** como fuente ÚNICA: un test
  (`TestVersioninfoMatchesVERSION`) falla si `versioninfo.json` diverge de `VERSION`, y `release.yml` **aborta
  el release** si el tag no coincide con `VERSION` y **regenera el `.syso`** desde `versioninfo.json` con
  `goversioninfo` pineado (`@v1.4.0`) — el `.exe` de Windows ya no puede reportar una versión vieja. Cierra el
  hallazgo *high* «release no cortada / versión con dos fuentes de verdad divergentes» de la auditoría
  (`audit/2026-07-08`).
- **Guarda de compatibilidad de esquema hacia adelante (Track 16 / Producible 16.0c).** Un binario viejo que
  abría una base migrada por uno más nuevo antes corría un no-op silencioso y operaba a ciegas sobre columnas/
  tablas que no conocía — riesgo de corrupción lógica en una flota mixta (laptop/PC/central con binarios de
  distinta versión). Ahora `applyMigrations` **falla-cerrado**: si el `user_version` de la base supera la última
  migración que este binario conoce, se niega a abrir con el error centinela **`ErrSchemaTooNew`** (sin degradar
  ni avanzar la versión), en vez de continuar. Cierra el hallazgo *medium* «sin guarda de compatibilidad de
  esquema en runtime» de la auditoría de producibilidad (`audit/2026-07-08`). Aditivo, golden intacto.
- **Captura automática C4 — capturar el par error→fix al resolver telemetría.** El par error→fix es *la
  memoria de código más valiosa*, y Musubi ya lo tenía en la tabla de telemetría (`musubi_log_error` guarda
  el error + el parche propuesto) pero moría ahí. Ahora, cuando se llama **`musubi_resolve_telemetry`** (el fix
  se confirmó), se **captura el par como memoria local** — `"Error en <file>: <mensaje> → Arreglado con:
  <parche>"` (`procedural`, deduplicada) — recuperable por recall. Model-free, best-effort (un fallo de la
  captura no rompe el resolve), y solo captura si hay un parche registrado (anti-ruido). Queda **local** (al
  compartir por `promote`, la redacción de C2 lo limpia). **Cierra el track de captura automática (C1 proactiva
  + C2 redacción + C3 commits + C4 error→fix).** Aditivo: sin tools nuevas, golden intacto.
- **PC auto-configurable P2 — `musubi provision` deja el proyecto seteado.** P1 conectaba la máquina al
  cerebro; ahora `provision` también **deja el proyecto 100% seteado como Musubi** — workspace `.musubi/`,
  skills cognitivas, templates SDD y los **4 hooks** (SessionStart con el priming de captura proactiva **C1**,
  UserPromptSubmit, PreToolUse, y **Stop** con la captura de commits **C3**) — reusando los helpers de `setup`.
  Consecuencia: una máquina recién provista tiene **la captura automática y la memoria de código funcionando
  de fábrica**, no solo la conexión al cerebro. Best-effort (un fallo del setup local no revierte la conexión
  ya lograda), idempotente, y `--dry-run` no muta. Aditivo: `setup` sin cambios, golden intacto.
- **Captura automática C3 — captura de commits (red de seguridad determinista).** Un hook **`Stop`**
  (`musubi capture --hook-mode`) que, al cerrar cada turno, captura los **commits nuevos** del repo como
  memoria **local**, sin depender del agente ni de un LLM — el mensaje de commit **es el "por qué"** destilado
  por el humano. Model-free: lee `git log` incremental desde el último HEAD capturado (guardado en meta,
  global al repo; la primera vez solo el HEAD, para no ingerir toda la historia), **clasifica por keyword**
  (fix/bug/security → alto; feat/refactor/perf → medio; y **omite** merge/wip/cortos y chore/docs/style/test/
  build/ci), y guarda subject + body + archivos tocados, **deduplicado**. **No-op silencioso** si no es un
  repo git, no hay commits nuevos, o todos son triviales. La captura es **local** (nunca comparte: un secreto
  de un diff no cruza; compartir pasa por `promote`, que C2 redacta). `setup` registra el hook `Stop`
  (idempotente). Cierra el track de captura automática (C1 proactiva + C2 redacción + C3 commits). Aditivo:
  sin tools nuevas, golden intacto.
- **Captura automática C2 — redacción de secretos en el borde a `shared` (más seguro que el SOTA).** Como la
  captura es **shared-by-default**, un secreto que el agente capture no debe terminar en el cerebro que ve
  todo el equipo. Nuevo paquete `internal/redact` (model-free, **sin dependencias nuevas**): `Redact(text)`
  combina **reglas por forma** (AWS/GitHub/Stripe/Google/JWT/PEM/bearer/`KEY=valor`, RE2) con un **catch-all
  de entropía de Shannon** para formatos desconocidos, respetando una allowlist de placeholders (y **sin
  tocar git SHAs**). La guarda se aplica **en el borde donde una observación se vuelve `shared`**
  (`saveObservation` con scope shared y `PromoteObservation`): el contenido se limpia ANTES de persistir, y
  como el outbox reconstruye el payload desde la fila, **nada sin redactar cruza al central por ninguna ruta**.
  La memoria **`local` queda intacta** (los secretos pueden vivir en tu propia máquina; se limpian solo al
  compartir). Ningún competidor (Mem0/Letta/Zep/Copilot) documenta redacción. Aditivo: sin deps, sin tools
  nuevas, golden intacto.
- **Captura automática C1 — captura proactiva (el cerebro aprende mientras trabajás).** Musubi ya
  RECUPERA memoria solo; ahora también **empuja a capturarla sola**. El hook SessionStart inyecta un
  bloque conciso (`startup_capture`) que instruye al agente a **guardar por su cuenta, sin que se lo
  pidan**, los aprendizajes durables — **decisiones** (el porqué), **gotchas**, **estado del trabajo**
  y **hechos de código** — con las tools correctas y con criterio de salencia (solo lo reusable/no-obvio,
  nada de trivialidades); además **desambigua "shared"** = memoria compartida del cerebro, NO un tag ni
  commit de git. El recordatorio por turno pasa a ser **prescriptivo** (nombra qué capturar, no solo el
  conteo). El bloque **respeta el hook silencioso**: viaja solo cuando el arranque ya tiene algo que
  decir. La extracción la hace el agente (que es el LLM), no Musubi — costo LLM cero, coherente con el
  diseño model-free. Es la Fase 1 del track de captura automática; la captura es **local** (compartir al
  cerebro llega en una fase posterior, detrás de la redacción de secretos). Aditivo: sin tools nuevas, golden intacto.
- **PC auto-configurable P1 — `musubi provision` (unir una máquina al cerebro).** Nuevo subcomando que
  lleva un equipo a estar **unido al cerebro central** en un comando, idempotente y cross-platform. El
  corazón es un **preflight de red VPN-agnóstico**: sonda dos caminos (un destino público de control por IP
  literal —sin DNS— y el cerebro en el tailnet) y clasifica el entorno en `Clean` / `SplitExcluded`
  (el runtime va directo y solo ve la malla) / `Tunneled` (el runtime está atrapado en el túnel y no ve la
  malla) / `Isolated`, con **guía accionable en prosa sin nombrar ningún producto de VPN**. Si el cerebro no
  es alcanzable, **frena el self-check y explica el paso faltante** en vez de fallar en silencio. Luego
  asegura Tailscale, aplica la **apertura del tailnet** (reglas de firewall `TS-Allow-Tailnet-In/Out` en
  Windows / allowlist de subred en Linux, idempotentes; si falta admin, instruye sin abortar), **cablea el
  `.mcp.json`** con las entradas `musubi` (local) y `musubi-cerebro` (remota, bearer por `${MUSUBI_TOKEN}` —
  el secreto nunca toca el archivo) preservando lo existente, y hace el **self-check reach + auth** contra el
  cerebro. También deja el bloque **`sync:`** en el `.musubi/config.yaml` (idempotente, preservando la config
  previa) para que el daemon LOCAL **suba solo la memoria `shared`** al cerebro (outbox de F2) — con
  `allow_insecure_token: true` porque el central es `http://` sobre el tailnet (WireGuard ya cifra); sin este
  paso el `.mcp.json` conectaba pero el auto-sync quedaba apagado. `--dry-run` diagnostica y muestra el plan
  sin mutar. Porta a Go la lógica probada en `deploy/connect-brain-*`. Aditivo: no agrega tools MCP (el golden no cambia).

### Changed
- **Cerebro híbrido — sync más robusto (offline-first de verdad).** Se corrigió una grieta de F2 que
  destapó una prueba real: un fallo **transitorio** del sync (cerebro central caído, VPN reconectando) que
  acumulaba `max_attempts` terminaba en **dead-letter permanente**, perdiendo memoria `shared` que sólo
  estaba temporalmente sin poder entregarse. Ahora un fallo transitorio (red/timeout/5xx/429) **nunca muere**:
  reintenta indefinidamente con backoff exponencial acotado; **sólo** un fallo permanente (4xx/params/auth)
  va a dead-letter. Además, dos tools nuevos le dan **ojos y una red de seguridad** al sync: **`musubi_sync_status`**
  (read-only) reporta cuántas observaciones están pendientes/enviadas/en dead-letter, la antigüedad de la más
  vieja pendiente y el último error; **`musubi_sync_requeue`** devuelve las que quedaron en dead-letter a la
  cola de envío (útil tras un corte). Aditivo y backward-compatible; con `sync.enabled=false` nada cambia.

### Added
- **Cerebro híbrido F2 — outbox durable + cliente de sync saliente (offline-first).** El conocimiento
  marcado `shared` (F1) ahora **viaja al cerebro central** por su cuenta. Cuando una observación se promueve
  o se guarda como `shared`, se encola una fila en una **tabla `outbox` durable** (migración v11, aditiva)
  **dentro de la misma transacción** que cambia el scope (*transactional outbox*: o quedan ambos o ninguno).
  Un **scheduler de drain** —arrancado en `daemon` y en `serve`, que **no toma el lock de dispatch**—
  reclama lotes con un `UPDATE … RETURNING` atómico (lease sobre `next_attempt_at`, con auto-recuperación
  de reclamos colgados) y los empuja al `musubi serve` central vía JSON-RPC `tools/call` →
  `musubi_save_observation` remoto, con el `id` de la observación como clave: la re-entrega es un no-op
  gracias al UPSERT `ON CONFLICT(id)` del receptor (**at-least-once con efecto exactly-once**). Es
  **offline-first**: si el central está caído la fila queda `pending` con *backoff* exponencial (jitter,
  tope) y drena sola al recuperarse; los errores permanentes (4xx) o el tope de reintentos van a
  *dead-letter* (`status='dead'`). Un **backfill** idempotente al abrir la DB siembra el outbox con las
  `shared` que ya existían de F1. El re-sync ante cambio de contenido se detecta por `content_hash`. Config
  nueva bajo `sync:` (`enabled` —**off por default**—, `central_url`, `auth_token_env` —el token **nunca**
  en el YAML, siempre por env var—, `drain_interval_seconds`, `batch_size`, `max_attempts`,
  `backoff_base/max_seconds`, `lease_seconds`, `allow_insecure_token`). Cero dependencias nuevas; el set de
  tools MCP no cambia; con `sync.enabled=false` el comportamiento es idéntico al de antes. Es la Fase 2 del
  track de 5 (F3 central multi-proyecto, F4 federated recall, F5 hardening).
- **Cerebro híbrido F1 — modelo de `scope` (local/shared) + `project_id` en la memoria.** Fundación del
  cerebro central compartido: cada observación lleva ahora un `scope` (`local`, default = comportamiento
  histórico; o `shared`, candidata a sincronizarse con el cerebro central en fases siguientes) y un
  `project_id` que la ata a su proyecto (migración v10, aditiva y backward-compatible). `musubi_save_observation`
  acepta un parámetro opcional `scope` (validado); un tool nuevo **`musubi_promote`** eleva una observación
  local a `shared` (idempotente). Internamente se **centralizó el predicado de visibilidad**
  (`archived = 0 AND superseded_by IS NULL`) en una única constante (`visibleObsPredicate`), refactorizando
  las queries de lectura sin cambiar el SQL — el *seam* para el filtrado por scope que viene. Todo aditivo:
  las bases y observaciones previas se comportan idéntico (0 regresiones). Es la Fase 1 de un track de 5
  (F2 sync offline-first, F3 central multi-proyecto, F4 federated recall, F5 hardening).
- **Dashboard-cerebro (`musubi dashboard`): la memoria como grafo neuronal 3D en vivo.** Nuevo backend
  `internal/memory/braingraph.go` que expone las observaciones activas como **neuronas** y las
  `observation_relations` como **sinapsis** (`DbEngine.BrainGraph`), read-only y model-free —saliencia
  `importance*exp(-age/30)+ln(1+heat)` computada en Go, cap top-N, sin aristas colgantes—. `musubi export`
  suma el campo `brain` al snapshot y `musubi dashboard` lo renderiza en un canvas: cerebro 3D con
  **spreading-activation real** (solo dispara con actividad entre polls), HUD glass (salud/tokens/
  orquestación/dominios/actividad) y polling de `/api/snapshot`. El volumen **se expande simétricamente con
  la población** (radio ∝ N^⅓, encuadre estable) y el render se **autorregula por FPS** (LOD por
  prominencia, bloom sin `ctx.filter`, gobernador de calidad) para sostener miles de neuronas. Loopback-only,
  0 tokens, proceso aparte.
- **Scripts de despliegue del cerebro central en `deploy/`** (`install-musubi-brain.sh` +
  `connect-brain-linux.sh` / `connect-brain-windows.ps1`): montan Musubi como daemon MCP sobre HTTP
  (`musubi serve`) en un servidor Linux y conectan cada dispositivo cliente, en **un comando por
  máquina**. El de servidor es idempotente —binario+checksum, `restorecon` de SELinux, workspace,
  bloque `service:`, token que **no se regenera** al re-correr, unit systemd, `tailscale0` en la zona
  `trusted` del firewall, y verificación de `/readyz`+`tools/list`—. Los de cliente hacen el onboarding
  (Tailscale, allowlist de NordVPN, entrada remota `musubi-cerebro` en el `.mcp.json` con el token por
  referencia `${MUSUBI_TOKEN}`, y verificación con auth). Codifican el runbook de
  `docs/Server_Brain_Onboarding.md`.

### Changed
- **`backupDB()` migrado a `VACUUM INTO`**: el backup del auto-heal del `doctor` ahora es un snapshot
  consistente y compactado en vez de una copia cruda del archivo.

## [0.77.0] - 2026-07-04

Auditoría del sistema de tokens, Frente #3 (d) — **el recordatorio de captura cuenta las tres superficies**.
Cierra el Frente #3 y la auditoría. Correctness del loop dirigido, model-free, sin migración.

### Fixed
- **El recordatorio de captura ya no da falsos positivos con `save_fact`/`save_code`**: `buildCaptureReminder`
  usaba `CountObservations` como señal de "se guardó algo" entre turnos, así que persistir un **hecho**
  (`musubi_save_fact`) o un **gist de código** (`musubi_save_code`) no reiniciaba el contador y el nudge saltaba
  igual —aun cuando el propio texto sugería `musubi_save_fact`—. Ahora la señal deriva de un nuevo
  `CountSavedItems()` que suma las tres superficies (`observations` + `relations` + `code_memory`) en una sola
  query; es un total monótono ante cualquier save nuevo. La lógica de umbral/turnos/session-scoping no cambia.

## [0.76.0] - 2026-07-04

Auditoría del sistema de tokens, Frente #3 (c) — **delta del run en `musubi_workflow`**. Las acciones
incrementales dejan de re-serializar la definición inmutable del workflow en cada respuesta. Model-free, sin
cambios de esquema ni de estado persistido.

### Changed
- **Las respuestas incrementales de `musubi_workflow` omiten `definition`**: cada acción (`complete`,
  `provide`, `verify`, `rollback`, `abort`, `compensated`) devolvía el `run` COMPLETO, incluido el DAG entero
  (`definition`: todos los steps con títulos y directivas `verify`/`await`/`compensate`) — que **no cambia tras
  `start`**. En un run de varios pasos era el mayor bloque repetido del payload. Ahora esas acciones devuelven
  una vista `run` sin `definition` (conserva `run_id`/`workflow_id`/`status`/`step_status`/`step_results`/
  `step_iters`); el snapshot completo —con `definition`— sigue disponible en `start`, `status` y `resume` (los
  puntos donde el caller no tiene estado previo). Solo cambia la SERIALIZACIÓN de la respuesta: el estado en
  SQLite y la capa de memoria quedan intactos.

## [0.75.0] - 2026-07-04

Auditoría del sistema de tokens, Frente #3 (b) — **búsqueda gist-first**. `musubi_search_semantic` y
`musubi_search_keyword` dejan de serializar la `Observation` completa por hit (el mayor costo de tokens
model-facing recurrente que quedaba) y devuelven titulares acotados por presupuesto. Model-free, sin migración.

### Changed
- **`musubi_search_semantic` / `musubi_search_keyword` son gist-first**: antes ambas devolvían el objeto
  `Observation` COMPLETO (contenido full × N hits) en cada llamada. Ahora devuelven por hit
  `{id, topic_key, gist, similarity?, full_tokens}` —el titular extractivo en lugar del contenido— con el
  payload total acotado por un presupuesto de tokens (`searchGistBudget`, top-1 garantizado). El contenido
  completo se hidrata bajo demanda por `id` con `musubi_recall`/`musubi_memory_expand`. `similarity` solo
  aparece en la búsqueda semántica; `full_tokens` informa el costo de hidratar. Sin nuevos parámetros de
  schema (el `limit` existente sigue acotando la cantidad). Modelado en la capa MCP: las queries de memoria
  y el esquema quedan intactos.

## [0.74.0] - 2026-07-04

Auditoría del sistema de tokens, 3ª tanda — dos de los tres frentes que quedaban: **relevancia del recall por turno**
y **adelgazar el schema de tools** (costo fijo por turno). Ambos model-free y sin perder eficacia.

### Fixed
- **El recall por turno filtra stopwords** (relevancia): la superficie MÁS caliente (recall en cada
  UserPromptSubmit) corría un MATCH de FTS **crudo** —`el`/`de`/`la`/`the`/`of` incluidos— que diluía el OR y dejaba
  que la recencia volcara el orden, colando memorias tangenciales-pero-recientes. Ahora usa un nuevo flag
  `RankedFTS` que descarta stopwords (es/en) y tokens de 1 runa antes de armar la query (con fallback seguro si todo
  era ruido). **Opt-in**: el recall del tool `musubi_recall` queda bit-a-bit igual; solo cambia el recall por turno.

### Changed
- **Descripciones de tools más densas** (−~625 tok/turno de costo FIJO): las 5 mega-descripciones
  (`musubi_workflow`, `musubi_work`, `musubi_debate`, `musubi_sdd`, `musubi_author_skill`) embebían el protocolo
  completo paso-a-paso, pagado en el schema cada turno. Se recortó el racional y la verbosidad redundante
  **preservando cada action y feature con su trigger→action→params** (la respuesta de la tool guía las features
  avanzadas cuando aplican). El schema de las 31 tools bajó de ~30.1k a ~27.6k chars. Sigue en 31 tools.

### Notes
- Frente que queda de la auditoría (#3): cachear `gist_tokens` (necesita migración), `search_semantic`/`keyword`
  gist-first con budget, delta en las respuestas de `musubi_workflow`, y `capture_reminder` contando todas las
  superficies de guardado. Documentado en `audit/2026-07-04-token-system`.

## [0.73.0] - 2026-07-04

Auditoría del sistema de tokens, 2ª tanda — **precisión del estimador** (los hallazgos #8/#9). Ambos son puro win,
model-free y 100% bajo control del server: mejoran la exactitud de la estimación de tokens SIN sacrificar recall. El
estimador versionado recomputa la columna `tokens` de todas las filas al abrir el motor (aplica al reiniciar).

### Fixed
- **Estimación por-segmento del markdown** (#8): antes, un solo fence ` ``` ` en una observación clasificaba **todo**
  el blob como código (`/3.4`), sobre-estimando ~12–17% y haciendo que el recall empaquetara **menos memoria de la
  que cabía**. Ahora `EstimateTokens` separa los bloques de código (entre fences) de la prosa y estima cada parte con
  su divisor. Recupera budget de recall real. JSON estructural se sigue estimando como blob completo.
- **Peso de caracteres no-ASCII** (#9): los acentos/emoji se contaban por runa y se dividían por el divisor de prosa
  (`/4`), **sub-estimando** la prosa acentuada — dirección insegura para un presupuesto, y todo el corpus es en
  español. Ahora los no-ASCII no-CJK se cuentan más densos (`divNonASCII=2.0`, ~0.5 tok/char), restaurando el sesgo
  conservador. La calibración opt-in descuenta esta contribución fija al ajustar los divisores por tipo.

### Notes
- El estimador pasa a `v3-seg-nonascii`: al reiniciar, recomputa `tokens`/`gist` de todas las filas una vez
  (idempotente). Pendientes mayores de la auditoría aún abiertos: adelgazar el schema de tools (~7.500 tok/turno,
  con el asterisco del prompt-caching client-side) y el floor de relevancia del recall por turno. Sigue en 31 tools.

## [0.72.0] - 2026-07-04

Auditoría del sistema de ahorro de tokens (4 agentes anclados en código + verificación adversarial) → **bundle de
quick-wins**: menos tokens sin sacrificar una gota de recall. El veredicto fue "sano ~8.5/10; el desperdicio está
concentrado, no es arquitectónico". Este release ataca 5 de los hallazgos de mayor ROI y riesgo casi nulo.

### Changed
- **Respuestas JSON de las tools compactas** (`jsonResult`: `MarshalIndent`→`Marshal`): la indentación era ~**28%**
  de whitespace puro en cada payload estructurado (recall, tokens, workflow, search, doctor…) que el cliente MCP
  parsea igual. −28% en toda respuesta JSON de tool.
- **`content_hash` fuera del payload model-facing** (`RecallItem`, `json:"-"`): eran 64 hex (~25 tokens) por item de
  maquinaria server-side (la inyección diferencial la consume in-process en Go) que viajaban al modelo sin que los
  use. Se conserva como campo Go; deja de serializarse.

### Fixed
- **`turn_batch` sin delta guard**: era el único bloque por turno que se re-inyectaba **cada turno** mientras había
  un batch activo (~53 tok/turno). Ahora usa el mismo `turnSurfaceChanged` que los otros bloques: solo emite cuando
  el progreso del batch cambió.
- **El recall por turno ignoraba los toggles semánticos**: la superficie MÁS caliente (recall en cada
  UserPromptSubmit) corría léxico puro, sin Stemming/Cooccurrence/GraphCentrality —los puentes model-free que la tool
  `musubi_recall` sí usa (Tracks 14/B4)—. Ahora se propagan desde `memory.*`: **mismos tokens, más relevancia**.
- **Metaclaves de captura no session-scoped**: `loop_obs_seen`/`loop_turns_since_save` sangraban entre sesiones (una
  sesión nueva heredaba el contador de la anterior y podía disparar el nudge de captura sin actividad propia). Ahora
  llevan el `session_id` como sufijo, igual que el estado delta.

### Notes
- Diferido de este bundle (necesita señal nueva en el recall + más superficie de test): floor de relevancia (no
  inyectar recencia disfrazada en prompts genéricos). Documentado en `audit/2026-07-04-token-system`. Pendientes
  mayores de la auditoría: adelgazar el schema de tools (~7.500 tok fijos/turno) y precisión del estimador
  (segmentación de markdown, peso no-ASCII). Sigue en 31 tools.

## [0.71.0] - 2026-07-04

Track 15, S1 (cierre) — **guard de cambio de modelo de embedding**. Con la Capa 2 (StaticProvider) ya es fácil
alternar tablas de embedding; si dos modelos comparten dimensión, sus vectores no son comparables por coseno pero el
`dim-guard` existente no los distingue (mezcla silenciosa que degrada el recall). Este release cierra ese borde con
la opción proporcionada: **visibilidad**, no maquinaria pesada.

### Added
- **Aviso de cambio de modelo de embedding** — al arrancar, si el modelo activo (`Provider.Name()`) cambió respecto
  del último run **y hay vectores previos de otro modelo**, se logea un warning claro (con conteo y acción sugerida:
  limpiar/re-embeber si el cambio fue same-dimension). Registra el modelo activo en `meta` para el próximo arranque.
  **Sin migración, sin cambiar el recall, no-op sin embedder.** Cubre el borde same-dim que el `dim-guard`
  (CosineSimilarity falla si dim≠, IVF descarta la dimensión minoritaria) no alcanza. Descartada la provenance
  per-row completa (columna `model_id` + filtro) por sobre-ingeniería para una realidad de un embedder por proceso.
  Cierra el backlog de Track 15 (S3 multilingüe = elección de asset sin código; Capa 1 y TLogic diferidos por
  decisión de ROI). Sigue en 31 tools.

## [0.70.0] - 2026-07-04

Track 15, Capa 2 — **semántica model-free _at inference_**. La auditoría dejó como frontera de fondo que Musubi, por
ser model-free, no "entiende": su recall combina señales léxicas/estructurales pero no capta sinonimia real
(`deploy`≈`despliegue`) salvo que un embedder externo esté configurado. Este release da esa capacidad **sin runtime
de modelo y sin cgo**: un provider que genera embeddings con una **tabla estática** token→vector (formato
model2vec/POTION) + mean-pooling — cero forward pass de red neuronal.

### Added
- **`StaticProvider` (embedding.provider=`static`)** — embeddings por lookup en una tabla estática destilada
  (model2vec/POTION) + mean-pool + L2-normalize, con un **WordPiece BERT propio bit-exacto** (BertNormalizer con
  strip-accents por NFD, greedy longest-match, `[UNK]`). Cae directo en el pipeline ya existente (tabla `embeddings`
  + índice IVF + coseno + fusión RRF) — **cero cambios en memory/mcp**. La tabla la aporta el usuario en
  `embedding.static_path` (bring-your-own-table: `model.safetensors` + `tokenizer.json`); **off por defecto**
  (`NoopProvider`), feature 100% aditiva. Bit-exactitud validada contra model2vec (12 strings EN/ES/acentos/
  puntuación, cosine 1.000000). Claim honesto: **"model-free _at inference_"** — la tabla se destiló offline de un
  sentence-transformer (misma categoría que servir vectores GloVe), **no** "model-free absoluto". Única dep nueva:
  `golang.org/x/text` (NFD del strip-accents). Sigue en 31 tools.

### Notes
- Diferido con criterio: provenance/homogeneidad de vector por `model_id` (el dim-guard existente ya cubre el switch
  de modelos de distinta dimensión), default multilingüe (`potion-multilingual-128M`), y bundling/auto-download del
  asset (hoy bring-your-own-path).

## [0.69.0] - 2026-07-04

Track 14, #2 — **2ª ola de semántica model-free**: stemming query-time por prefijo. Ataca el miss de recall más
común (morfológico): sin esto, buscar "deploy" no encontraba una memoria que dice "deploys" o "deployment", porque
el FTS matchea tokens exactos.

### Added
- **Stemming por prefijo en el recall** (sin dependencia, sin re-indexar): con el flag on, cada término de la query
  se reduce a una raíz con un stemmer **liviano y conservador** (recorta un sufijo de flexión ES+EN dejando raíz
  ≥4 runas; términos <5 quedan intactos) y se matchea por **prefijo FTS** (`"deploy"*`), atrapando las variantes de
  sufijo (`deploy`/`deploys`/`deployment`, `casa`/`casas`). Fiel a la identidad: **cero dependencia nueva** (se
  descartó Snowball para no romper la disciplina de 3 deps), **sin re-indexado ni migración**, model-free y
  determinista. Config `memory.recall_stemming` (default ON; `false` desactiva); off por zero-value preserva el
  match exacto histórico bit-a-bit. Honesto: cubre variantes de **sufijo**, no cambios de raíz (`despliegue`↔
  `desplegar` — eso requeriría un stemmer completo). Segunda ola de #2 tras la co-ocurrencia/PRF. Sigue en 31 tools.

## [0.68.0] - 2026-07-04

Track 14 (post-auditoría v0.65.0), #2 — **primer slice de semántica model-free** en el recall. La auditoría marcó
como gap estratégico que, sin embedder externo, la única señal de contenido era léxica (FTS token-exact): "deploy"
no encontraba una memoria que dice "despliegue". Este release agrega un **puente de vocabulario derivado del
corpus**, sin LLM ni modelo, manteniendo el determinismo.

### Added
- **Recall por co-ocurrencia / pseudo-relevance feedback (PRF)** — 6ª señal RRF opcional: tras el recall léxico,
  toma los top resultados (pseudo-relevantes), cosecha los términos que **co-ocurren** con la query en ellos
  (aparecen en ≥2 de esos docs, excluyendo la query y stopwords) y corre un 2º FTS con esos términos para **traer
  observaciones con vocabulario distinto** que la query original no encontró (el puente `deploy`↔`despliegue`). La
  "semántica" se **deriva del corpus** — no se importa de un modelo: pura tokenización + conteo + FTS, determinista.
  Realización **index-free** de la co-ocurrencia (sin índice global persistido, sin tabla, sin migración). Config
  `memory.recall_cooccurrence` (default ON; se desactiva con `false`); off por zero-value preserva el recall
  histórico bit-a-bit. Honesto: el valor es corpus-dependiente (con memoria escasa degrada a no-op). Primer paso de
  #2; quedan olas futuras (stemming EN+ES, LSA/SVD, índice global de co-ocurrencia). Sigue en 31 tools.

## [0.67.0] - 2026-07-04

Track 14 (post-auditoría v0.65.0), ola de endurecimiento — A2 (limpieza de código muerto, #4) + A3 (blindaje de
tests, #5).

### Added
- **Fuzzing sobre los parsers model-free** (primeros fuzz tests del repo, cerrando el hueco "cero fuzzing" de la
  auditoría): `FuzzSimilarity` (Jaccard de trigramas — invariantes [0,1] + simetría + no-NaN), `FuzzEvalCondition`
  (parser de expresiones `when`/`repeat_while` — determinismo + no-panic), `FuzzBuildFTSQuery` (constructores de
  query FTS — tolerancia a puntuación/unicode/bytes nulos). ~50–100k ejecuciones por fuzzer sin panics.
- **Test de concurrencia REAL del claim de la pizarra** (`TestClaimWorkUnitConcurrentNoDoubleClaim`): N agentes en
  goroutines compiten por M unidades; verifica que ninguna se reclama dos veces y que se reclaman exactamente las M
  (antes la "atomicidad" sólo se probaba en secuencial). Se apoya en el `UPDATE...RETURNING` bajo el write-lock de
  SQLite (`busy_timeout`); CI lo corre con `-race`.

### Removed
- **Cruft genuino eliminado**: `writeMCPConfig` (envoltorio duplicado de `writeMCPConfigAt`, sólo lo usaba su
  test) e `internal/codeintel/imports.go` completo (`ExtractImports` y helpers, usado únicamente por su propio
  test, sin ningún feature que lo consumiera). Al auditar el "código muerto" que marcó la auditoría se distinguió
  cruft de **andamiaje intencional**: se PRESERVARON `bootstrap.RemoteEntry`/`MergeRemoteMCPServer` (groundwork
  documentado del home-server: apuntar clientes al `musubi serve` central sobre la VPN) y `FakeRunner` (falso de
  git usado por los tests; `deadcode` lo marca sólo porque analiza desde `main`, no desde los tests). Borrar
  groundwork por "reducir superficie" habría destruido trabajo planeado; se removió sólo lo genuinamente muerto.

## [0.66.0] - 2026-07-04

Track 14 (post-auditoría v0.65.0), A1 — **modelo de fallo del motor de workflows**. La auditoría profunda encontró
un bug funcional latente: `RunAborted` estaba declarado pero nunca se usaba, y un step `failed` dejaba el run en
`running` para siempre (run zombie) con sus dependientes bloqueados, sin forma de abortarlo. Este release cierra ese
hueco: el estado del run ahora se **deriva** correctamente de los estados de sus steps, y hay un abort explícito.

### Fixed
- **Un run wedgeado por un step fallido ya no queda zombie**: si un step queda `failed` y bloquea todo progreso
  posible, el run transiciona a un estado terminal `failed` (con evento `run_failed` en el journal), en vez de
  quedar `running` indefinidamente. La transición es **derivada y model-free** (`computeRunStatus`): mientras haya
  progreso posible —una rama independiente en curso, un gate humano/verify sin resolver, un step con `when` que
  podría saltarse— el run **no** se marca failed (sin falsos-fallo). El happy-path (`run_done`) queda idéntico.

### Added
- **`musubi_workflow action=abort`** (run_id, razón opcional en `result`): aborta explícitamente un run atascado o
  no deseado → estado terminal `aborted` (evento `run_aborted`), y deja de despachar steps. Idempotente; falla si el
  run ya concluyó con éxito (`done`/`compensated`). Un run `failed`/`aborted` **todavía se puede compensar** con
  `rollback` (saga LIFO de los steps completados). Un run terminal (done/failed/aborted/compensated) no despacha más
  steps. Sin migración (los estados nuevos fluyen por la columna `status` existente). Sigue en 31 tools.

## [0.65.0] - 2026-07-04

`musubi setup` ahora **refresca las skills cognitivas manejadas** cuando el binario las actualiza, **sin pisar las
que el usuario editó**. Antes, `writeCognitiveSkills` saltaba cualquier archivo existente, así que un update de una
skill (p. ej. `adversarial-review` → `musubi_debate`) nunca llegaba a los repos ya inicializados — había que copiar
el `.yaml` a mano a cada repo. Ahora cada skill lleva su propia prueba de integridad y la propagación es un
`musubi setup`.

### Changed
- **Refresh de skills manejadas por checksum**: cada skill cognitiva que escribe Musubi lleva un `managed_checksum`
  (sha256 de su contenido canónico, CRLF-agnóstico). En el próximo `setup`, Musubi lo usa para decidir de forma
  determinista: si el archivo sigue **exactamente** como Musubi lo escribió (checksum coincide) → lo **refresca** a
  la versión nueva; si el usuario lo **editó** (checksum no coincide, o el archivo no parsea) → lo **preserva**. Un
  archivo legacy idéntico a la versión actual se **adopta** (gana el checksum, sin cambiar contenido). **Regla de
  oro (safety): ante la mínima duda, preservar** — Musubi nunca pisa trabajo del usuario. Idempotente: un `setup`
  sin cambios no reescribe ni reporta nada. `setup` informa qué skills actualizó. Campo `managed_checksum` opcional
  (omitempty), no afecta el loader ni el gate de calidad; solo aplica a las skills cognitivas (no a las escritas a
  mano ni a las de auto-discovery). Cierra el hueco de propagación que obligaba a copiar skills a mano a los repos.

## [0.64.1] - 2026-07-04

Cierra el lazo de v0.64.0: la skill cognitiva **`adversarial-review`** ahora USA el subsistema `musubi_debate` en
vez de describir el patrón como prosa sobre la pizarra. Así el determinismo y la persistencia que agregó el debate
se aprovechan de verdad en la revisión adversarial (y en la fase verify del flujo SDD).

### Changed
- **`adversarial-review` cableada a `musubi_debate`**: la revisión adversarial pasa de coordinar escépticos por la
  pizarra (`musubi_work`) con conteo de mayoría "a mano" a orquestar un **debate estructurado**: `open` (rounds=2,
  quorum=mayoría) → cada escéptico (un LENTE: correctitud/seguridad/repro/contrato) postea su refutación con `post`
  → `advance` habilita una 2ª ronda de **crítica cruzada** (cada uno ve y rebate las refutaciones ajenas) → `vote`
  (real|no_real) → `tally` da el **veredicto por mayoría DETERMINISTA y persistido**. no_consensus (empate/sin
  quórum) ⇒ se defiere el juicio a `musubi_judge`. El veredicto y las posturas quedan reproducibles. Solo cambia la
  plantilla de la skill (model-free); ninguna tool nueva.

## [0.64.0] - 2026-07-04

Debate multi-agente (**Society of Minds**) como subsistema ejecutable y determinista, model-free — reabriendo C3,
que en Track 13 se había descartado como subsistema. Un audit del código (con evidencia file:line) confirmó que el
andamiaje del debate se compone solo PARCIALMENTE de las primitivas existentes: la skill `adversarial-review` ya lo
simula como PROSA para el LLM, pero faltan tres mecanismos estructurales para tenerlo como topología ejecutable
(fan-out/rondas parametrizados, agregador N-ario, unidad multi-postura). Este release provee los dos que son
model-free —posturas atribuidas por ronda (crítica cruzada persistida) y tally determinista— y deja el juicio
semántico donde corresponde: en el LLM. **Primer incremento del catálogo desde hace varias olas: 30 → 31 tools**
(un subsistema genuinamente nuevo justifica su tool propia, como `musubi_work` y `musubi_workflow`). Migración v9.

### Added
- **`musubi_debate` — debate multi-agente model-free** (acciones `open` / `post` / `advance` / `vote` / `tally` /
  `status`): Musubi NO razona — estructura las rondas, PERSISTE las posturas atribuidas por agente y ronda (crítica
  cruzada reproducible) y CUENTA los votos; los sub-agentes (LLM) producen las posturas, las críticas y los votos.
  Ciclo: `open` (topic, rounds, quorum opcional) → N sub-agentes postean con `post` → `advance` cierra la ronda y
  devuelve las posturas previas como material de crítica para la siguiente → `vote` → `tally`. El **tally es 100%
  determinista**: gana el `choice` con el máximo ESTRICTO de votos que alcance el quórum → el debate se cierra con
  ese ganador; empate, bajo quórum o sin votos ⇒ `no_consensus` (sigue abierto: se puede `advance`+re-votar, o
  deferir el juicio a `musubi_judge`). El juicio SEMÁNTICO (elegir/sintetizar) se queda en el LLM. Migración v9
  (`debates`, `debate_postures` con `UNIQUE(debate_id,round,agent)`, `debate_votes` con `UNIQUE(debate_id,agent)`,
  `ON DELETE CASCADE`). Subsistema aislado y aditivo: no toca recall/work/workflow. Multi-Agent Debate / Society of
  Minds. **El catálogo pasa de 30 a 31 tools** (incremento deliberado).

## [0.63.0] - 2026-07-03

Track 13 — B4 (memoria más inteligente, cierre). **Centralidad de grafo como 5ª señal RRF del recall**, la última
pieza de la receta HippoRAG que faltaba, dogfoodeada por el flujo SDD completo con verificación adversarial;
model-free / Go-puro / aditiva. Hallazgo de scoping: la fusión RRF del recall **ya era híbrida** (keyword FTS +
recencia + frecuencia + semántica vectorial coseno, T5.7 R2) — "B4 = FTS + semántico vía RRF" ya estaba entregado.
Lo único que faltaba de HippoRAG era la señal de **centralidad de grafo**, que hoy solo corría sobre el grafo de
**hechos** (`recall_facts`), no sobre observaciones. Catálogo en 30 tools; sin migraciones (todo derivado al vuelo).

### Added
- **Centralidad de grafo en el recall de observaciones** (5ª señal RRF, config `memory.recall_graph_centrality`,
  **default ON**): una observación que es **hub** de un cluster relacionado (muchas `related`/`supersedes`/
  `conflicts_with` en `observation_relations`) sube en el ranking aunque el FTS/vector no la priorizara
  (*spreading activation*, estilo HippoRAG). Se computa por **Personalized PageRank** sobre el grafo de relaciones
  vivo (ambas puntas no archivadas ni superseded, no dirigido), sembrado uniformemente en el pool de candidatos ya
  recuperado y **rerank-only** (no incorpora candidatos nuevos, a diferencia del pool vectorial). **DERIVE-not-store**:
  se deriva al vuelo, sin tabla de scores. Reutiliza el kernel de power-iteration de PageRank (extraído a
  `pprPowerIteration`, compartido con `recall_facts`; equivalencia one-hot verificada). El `zero-value` de código
  preserva el comportamiento histórico **bit-a-bit** (equivalencia probada); se activa por config (double-default,
  patrón de `decay_reinforcement_k`). Se desactiva con `recall_graph_centrality: false`.

## [0.62.0] - 2026-07-03

Track 13 — Ola C (orquestación avanzada). **Contract-Net bidding** sobre la pizarra multi-agente, model-free
y aditivo, dogfoodeado por el flujo SDD completo con verificación adversarial. C1 (pipelines declarativos PDL/SAMMO)
resultó **ya cubierto** — los workflows de Musubi ya son datos declarativos (defs YAML en `.musubi/workflows/`,
DAG, condicionales, loops, expresiones). C3 (debate topologies) se **descartó como subsistema**: el patrón se
compone con las primitivas existentes (verify-gate + Reflexion, pizarra multi-agente, `musubi_judge`) sin agregar
framework. Catálogo en 30 tools; una migración aditiva (v8).

### Added
- **Contract-Net bidding en la pizarra multi-agente** (`musubi_work` acciones `bid` / `award` / `bids`): cuando
  los sub-agentes difieren en aptitud, en vez de asignar por *claim* de orden de llegada (first-come), la unidad
  se **anuncia** y los agentes **ofertan** (`bid`, un score donde **mayor = mejor** aptitud/confianza, que produce
  el propio agente — model-free); el orquestador revisa con `bids` y **adjudica** con `award` a la mejor oferta.
  La adjudicación **reusa la maquinaria de lease/TTL/fencing** existente: la unidad queda `claimed` por el ganador
  y sigue el flujo `heartbeat`/`complete` normal. Determinista (desempate por antigüedad y agente), atómico
  (`UPDATE ... RETURNING` guardado por `status='open'` — un doble `award` es no-op). Coexiste con el claim
  first-come (el orquestador elige el protocolo por unidad). Migración v8 (`work_bids`, con `ON DELETE CASCADE`
  al limpiar el batch). Contract-Net (Smith, 1980).

## [0.61.0] - 2026-07-03

Track 13 — Ola B (memoria más inteligente). Cuatro features sobre el pilar de memoria, cada una dogfoodeada por
el flujo SDD completo con verificación adversarial, todas **model-free / Go-puro / aditivas**: recall asociativo
por **Personalized PageRank**, **tipo de memoria** (mem_type) con olvido diferenciado, **refuerzo Ebbinghaus** del
olvido (heat) y **consultas de camino** en el grafo. El catálogo sigue en 30 tools. Una sola migración aditiva
(v7, `mem_type`); todo lo demás se deriva al vuelo. B4 (RRF hybrid) queda para una ola futura por riesgo.

### Added
- **Recall asociativo por Personalized PageRank** (`musubi_recall_facts rank=pagerank`): el BFS de vecindad, al
  cortar por `max_facts`, dejaba los hechos en orden de inserción (arbitrario) y perdía los relevantes a 2+ saltos.
  El nuevo modo corre **PPR** personalizado a la entidad semilla sobre el grafo de hechos y devuelve primero los
  más relevantes por cercanía asociativa multi-hop (score de un hecho = suma del PageRank de sus extremos). Power
  iteration pura (damping 0.85, hasta 200 iteraciones, corte por tolerancia L1), grafo no dirigido, masa
  conservada (nodos colgantes reinyectan al restart). Compone con lo bi-temporal: `rank=pagerank` + `as_of` da
  **PageRank point-in-time**. Default (`rank=''`/`bfs`) intacto (equivalencia byte-idéntica). **Sin migración**
  (se deriva de `relations`). HippoRAG.
- **Tipo de memoria (`mem_type`) con olvido diferenciado** (`musubi_save_observation mem_type=...`): cada
  observación puede declararse `semantic` (conocimiento estable), `episodic` (eventos puntuales) o `procedural`
  (cómo hacer algo) — enum model-free que aporta el agente. El tipo **modula el olvido**: episódico se enfría antes
  (peso de saliencia 0.6), semántico neutro (1.0), procedural más durable (1.5); sin tipo = 1.0 (idéntico a antes).
  Un guardado sin tipo **preserva** la clasificación existente (solo un tipo no vacío la cambia). Migración v7
  aditiva (`ADD COLUMN mem_type`). LangMem.
- **Refuerzo Ebbinghaus del olvido (heat)**: la vida media de la recencia deja de ser fija — cada acceso (repaso)
  la **alarga**, así las memorias frecuentemente accedidas ("calientes") resisten el archivado (spacing effect):
  `vida_media_efectiva = vida_media · (1 + K · ln(1+accesos))`. `K` es `maintenance.decay_reinforcement_k`
  (default 0.5, activo en el daemon; `K=0` reproduce exactamente el olvido previo). Clamp defensivo: nunca acelera
  el olvido. **Sin migración** (usa `access_count`). MemoryOS.
- **Consultas de camino en el grafo** (`musubi_recall_facts to=<entidad>`): responde "¿cómo se conecta X con Y?"
  devolviendo el **camino más corto** (cadena de hechos, en orden) entre dos entidades. BFS no dirigido con
  reconstrucción por predecesores; acotado por `max_hops`; compone con lo bi-temporal (`as_of` → camino
  point-in-time). **Sin migración** (se deriva de `relations`).

## [0.60.0] - 2026-07-03

Track 13 — Ola A (cosechar el run journal). Frutos de observabilidad y robustez sobre el journal de v0.59.0.
Cuatro features, cada una dogfoodeada por el flujo SDD completo y **sin migración de esquema** (todo se apoya en
el journal `run_events` de v0.59.0): **export OpenTelemetry**, **saga (compensación LIFO)**, **HITL
(interrupt/resume durable)** y **gate de verificación + Reflexion**. `musubi_workflow` pasó de 8 a 13 acciones;
el catálogo sigue en 30 tools; todo aditivo y model-free.

### Added
- **Gate de verificación duro + Reflexion en workflows** (`musubi_workflow action=verify`): cierra el
  *verification-generation gap* (generar es fácil, verificar es el cuello de botella). Un step puede declarar
  `verify` (la directiva de qué chequear); al completarlo con `done` **no** queda hecho: entra en `verifying`
  (no terminal, bloquea a sus dependientes) hasta que un veredicto lo resuelva. `action=verify` (run_id, step,
  verdict `pass|fail`, reflexión en `result`): **pass** → `done` (uniforme: journalea `step_completed`);
  **fail** → registra la **reflexión** y, si queda presupuesto de intentos, **reabre** el step para un reintento
  informado (**Reflexion**); al agotarse (`max_iterations`, default 3), el step queda `failed` (el gate no se
  satisface). Las reflexiones acumuladas se devuelven para informar el reintento y quedan en el journal. Nuevo
  estado (`verifying`) y eventos (`step_verifying`, `step_reflection`). **Sin migración**. Model-free: Musubi
  impone la estructura del gate y registra; el veredicto lo produce el agente, idealmente con una lente
  adversarial (la skill `adversarial-review` lo fomenta) — adversarial > auto-chequeo.
- **HITL: interrupt/resume durable en workflows** (`musubi_workflow action=provide`): un step puede declarar
  `await` (un prompt), volviéndolo un **gate humano**. Al quedar listo, el run se **pausa** en él
  (`waiting_input`) en vez de ofrecerlo para ejecutar, bloquea a sus dependientes, y las respuestas lo surface en
  `waiting` con su prompt. Se reanuda con `action=provide` (run_id, step, input, status): `done` = aprobado (el
  `input` queda como resultado, los dependientes se destraban), `failed` = rechazado (siguen bloqueados). La
  espera es **durable** por construcción (estado + journal en SQLite): se puede proveer la decisión **en otra
  sesión** y el run continúa exactamente donde estaba (patrón interrupt/resume de LangGraph). Un gate con `when`
  falso se salta en vez de pausar. Nuevo estado de step (`waiting_input`) y evento de journal (`step_waiting`).
  **Sin migración**. Model-free: Musubi expone QUÉ espera y su prompt; el aviso al humano es del integrador.
- **Saga: compensación LIFO en workflows** (`musubi_workflow action=rollback` / `compensated`): el motor sabía
  avanzar un DAG pero no **deshacer**. Ahora un step puede declarar `compensate` (la directiva de cómo revertirlo);
  `action=rollback` inicia la **saga** y devuelve el plan de compensación en orden **LIFO** (inverso al de
  completado) de los steps completados con compensación; el agente ejecuta cada *undo* y reporta con
  `action=compensated` (run_id, step), que devuelve el plan restante; al vaciarse, el run queda `compensated`. El
  plan se **deriva del run journal** (re-entrante e idempotente: compensar dos veces un step es no-op; re-`rollback`
  recomputa lo que falta). Model-free: Musubi coordina QUÉ y EN QUÉ ORDEN; el agente ejecuta el undo real.
  Nuevos estados de run (`compensating`, `compensated`) y eventos de journal (`run_rollback`, `step_compensated`,
  `run_compensated`). **Sin migración** (el campo viaja en la definición ya persistida). El disparo es explícito
  (un step `failed` no fuerza rollback; la política es del agente).
- **Export OpenTelemetry del run journal** (`musubi_workflow action=otel`): exporta un run de workflow como una
  **traza OTLP/JSON** estándar (el run es un *trace*, cada step un *span*), lista para ingerir en cualquier
  collector (Jaeger, Grafana Tempo, etc.). La traza se **deriva** del journal en el momento del export (principio
  "derivar, no guardar-y-desfasar" — sin tabla de spans, sin migración, sin drift). IDs OTel **deterministas**
  (trace_id 16 bytes de `run_id`, span_id 8 bytes de `run_id`+`step_id`, por SHA-256 truncado): re-exportar da la
  misma traza. Status por step (`failed`→ERROR, `done`→OK, `skipped` marcado), atributos (`musubi.seq`,
  `event_type`, `result`, `workflow_id`), `service.name=musubi`. Model-free, Go puro, **sin el SDK de OTel** (el
  OTLP/JSON se emite a mano). Musubi sólo devuelve el JSON; el transporte al collector es del consumidor
  (local-first). Alinea con la dirección del servidor casero (Musubi como cerebro + orquestador observable).

## [0.59.0] - 2026-07-03

Track 13 — endurecimiento de los dos pilares (memoria + orquestación) con ingeniería SOTA, toda model-free.
Tres cambios, cada uno dogfoodeado por el flujo SDD completo: un **bugfix de liveness** en la pizarra (lease/TTL),
la **invalidación bi-temporal** del grafo de hechos (memoria que ya no envejece mal), y el **run journal
append-only** con idempotencia (cimiento de replay/observabilidad). Esquema evolucionado a la versión v6. El
catálogo sigue en 30 tools; todo aditivo y retrocompatible.

### Fixed
- **Bug de liveness en la pizarra multi-agente (`musubi_work`)**: una unidad que un sub-agente reclamaba y luego
  abandonaba (crash, timeout, sesión cerrada) quedaba en `claimed` **para siempre** — ningún otro agente podía
  retomarla y el batch nunca cerraba. Ahora cada claim toma un **lease con vencimiento (TTL)**: si el dueño no lo
  renueva, la unidad se recicla automáticamente en el próximo `claim` (reclamo *lazy*, sin proceso de fondo).

### Added
- **Run journal append-only + idempotencia por step** (Track 13, orquestación): el motor de workflows
  (`musubi_workflow`) sólo guardaba un **snapshot mutable**, sin idempotencia (un `complete` repetido
  sobrescribía en silencio) ni historia (no se podía auditar/exportar/replay). Ahora cada transición del run
  (arranque, step completado/saltado/reabierto, run cerrado) se registra en un **journal append-only**
  (`run_events`), escrito en la **misma transacción** que actualiza el snapshot — event-sourcing con read-model
  materializado, así journal y estado corriente nunca divergen. `complete` acepta una **`idempotency_key`**
  opcional: reintentar con la misma clave es un **no-op seguro** (no re-aplica ni duplica). Nueva acción
  `journal` (run_id) que devuelve la traza de eventos del run (`WorkflowJournal`). Es el cimiento estructural de
  replay/HITL/saga/observabilidad (OTel), que quedan habilitados para cambios futuros. Migración de esquema
  **v6** (tabla `run_events` con `UNIQUE(run_id, seq)` y `UNIQUE(run_id, idempotency_key)`), aditiva: el
  snapshot y su API siguen intactos.
- **Invalidación bi-temporal del grafo de hechos** (Track 13, memoria): hasta ahora `musubi_save_fact` sólo
  **acumulaba** tripletas y nunca retiraba ninguna, así que `(Ana, trabaja_en, Acme)` y `(Ana, trabaja_en,
  Globex)` convivían como si ambas fueran verdad. Ahora el grafo es **bi-temporal** (patrón Zep/Graphiti,
  model-free): para un predicado **funcional** (*single-valued*: `trabaja_en`, `estado_actual`, `vive_en`…,
  declarados en `graph.single_valued_predicates`), guardar un objeto nuevo **invalida** automáticamente el
  anterior por **cardinalidad** — sin LLM, sin entender el texto. El hecho viejo no se borra: se le cierra la
  ventana de validez (`valid_from`/`valid_to`, `invalidated_at`, `superseded_by`), de modo que la historia queda
  auditable. `musubi_recall_facts` devuelve por defecto sólo la **verdad actual** y acepta un parámetro **`as_of`**
  para consulta *point-in-time* ("qué era verdad en tal momento"). `musubi_save_fact` acepta un `valid_from`
  opcional y **revive** un hecho invalidado si se re-afirma. Migración de esquema **v5** (4 columnas aditivas +
  índice + backfill `valid_from = created_at`), retrocompatible. Los predicados *many-valued* (no declarados) no
  invalidan nada.
- **Lease/TTL + heartbeat + fencing token en `musubi_work`** (Track 13, orquestación): patrón *visibility timeout*
  (SQS) / lease (Chubby) sobre la pizarra, 100% model-free. Nuevo `action=heartbeat` para renovar el lease
  mientras el sub-agente trabaja; el `claim` devuelve un **fencing token** monótono que `heartbeat`/`complete`
  validan para bloquear al "worker zombie" (un agente expropiado que revive con un token viejo afecta 0 filas),
  incluso cuando dos agentes comparten el mismo id. Dead-letter automático (`failed`) tras `max_attempts` reclamos,
  para no reciclar indefinidamente una unidad que siempre falla. TTL y máximo de reintentos configurables
  (`multiagent.lease_ttl_seconds` = 300, `multiagent.max_attempts` = 5). Migración de esquema **v4** (columnas
  aditivas `owner_id`/`lease_expires_at`/`heartbeat_at`/`attempts`/`fencing_token` + índice), retrocompatible.
  Semántica *at-least-once* → el trabajo delegado debe ser idempotente.

## [0.58.0] - 2026-07-03

Release de dos hitos: **el pilar de orquestación/SDD elevado a co-igual de la memoria** (Track 12) y la
**inteligencia de cambios de código** (`musubi_detect_changes`). El catálogo de tools pasó de 27 a 30.

### Added
- **`musubi_detect_changes` — inteligencia de cambios de código (model-free, Go puro)**: nueva tool que corre
  `git diff` y, para cada archivo tocado, RE-DERIVA sus símbolos del contenido **actual** (`go/ast` para `.go`;
  escáner liviano para `.ts/.tsx/.js/.jsx/.py`) en vez de confiar en datos guardados — así el diff y los
  símbolos viven siempre en el mismo sistema de coordenadas y nunca se desalinean. Reporta, por archivo: el
  tipo de cambio, los símbolos afectados por los hunks, si su gist de memoria de código quedó *stale*
  (fingerprint) y qué observaciones/decisiones lo referencian. Es de solo-lectura y se engancha en la fase
  `verify` del flujo SDD para acotar qué verificar y qué decisión quedó potencialmente obsoleta. Nuevo paquete
  `internal/codeintel` (extractor de símbolos/imports + parser de diff unified), sin dependencias con cgo.
- **`musubi_save_code` deriva símbolos automáticamente**: cuando no se pasa `symbols`, se extraen del contenido
  actual del archivo (anclados al mismo fingerprint), evitando el string manual que se desincronizaba. Si el
  llamador pasa `symbols` explícito, se respeta (compat hacia atrás).
- **Flujo SDD guiado — `musubi_sdd`** (Track 12 O1): genera por vos el workflow canónico de un cambio
  (`proposal→spec→design→tasks→implement→verify→archive`) sobre el motor DAG, sin escribir YAML, y guía fase
  por fase; al cerrar cada fase persiste su contrato de resultado en memoria (`sdd/<change>/<phase>`) para que
  las siguientes lo recuperen por referencia barata en vez de releer archivos. Resumible entre sesiones.
- **Estimador de ahorro por delegación — `musubi_work action=savings`** (Track 12 O2): estimación model-free
  de los tokens ahorrados al delegar en la pizarra vs. hacerlo inline (aislamiento de contexto), con
  parámetros configurables.
- **Sistema avanzado de creación de skills** (Track 12): validador de calidad model-free
  (`internal/skills/quality.go`) que puntúa una skill contra las best-practices de Agent Skills (description
  como disparador en 3ª persona ≤1024 chars, name sin reservadas, triggers acotados, rules con ejemplo) y
  bloquea el guardado si tiene errores; nueva tool **`musubi_author_skill`** (reporte scoreado sin guardar, o
  guardado tras pasar el gate; reporta el tier de confiabilidad de la fuente).
- **Skills cognitivas embebidas**: `sdd-flow`, `adversarial-review` y `designing-web-ui` (WCAG AA + escala de
  espaciado 4/8px), incluidas en el bundle de `musubi setup`.
- **Cerebro remoto self-hosted** (Track 12 S): soporte para apuntar el MCP a una instancia central de Musubi
  vía entrada remota con token por variable de entorno; incluye runbook de onboarding.

### Changed
- **Dashboard de la memoria**: nuevo pilar de orquestación (runs/batches) en el snapshot y la vista (Track 12
  O4), y barrido completo a un sistema de espaciado 4/8px + escala tipográfica (skill `designing-web-ui`).

## [0.57.0] - 2026-06-23

### Added
- **Auditoría UX del dashboard contra el skill `ui-ux-pro-max`** (Track 11): se aplicó el *pre-delivery
  checklist* del skill (reglas de accesibilidad, timing de animación y contraste). El dashboard ya cumplía la
  mayoría (focos visibles, teclado en el grafo, *skeleton*, cifras tabulares, formato locale, sin emojis como
  iconos); esta release cierra los gaps detectados.

### Changed
- **Movimiento reducido**: la barra de carga deja de animarse bajo `prefers-reduced-motion: reduce` y se
  acortan todas las transiciones — el movimiento es 100% opcional. El *placeholder* de carga pasa de un
  *shimmer* de texto (que con `color:transparent` podía dejar los números de los KPIs invisibles en algunos
  *frames*) a un simple atenuado por opacidad: la barra superior indeterminada es la única señal de carga y
  nunca oculta contenido.
- **Chip de filtro accesible**: el chip «dominio ✕» del panel de memorias pasa a ser un control de verdad
  (`role="button"`, `tabindex`, `aria-label`) y se puede limpiar el filtro con `Enter`/`Espacio` (antes era
  solo *click*).
- **Timing de micro-interacción**: el *count-up* de KPIs y gauge baja de 620 ms a **400 ms** (regla del skill:
  micro-interacciones ≤ 400 ms).
- **Reveal escalonado**: los nodos del grafo aparecen con *stagger* de 35 ms por nodo (más natural; bajo
  movimiento reducido aparecen al instante).
- **Contraste AA**: el color de texto secundario `--dim` sube a ~4.6:1 sobre el fondo (antes ~4.2:1) para
  cumplir el mínimo 4.5:1 de texto normal.

## [0.56.0] - 2026-06-23

### Added
- **Pulido visual + UX del dashboard** (Track 11): el dashboard local sube de nivel manteniendo la estructura,
  los datos en vivo y el coste **0 tokens**:
  - **Sistema visual refinado**: tokens de contraste/espaciado/radio/elevación, fondo con aura sutil de la
    marca, cabeceras de sección con barra de acento y KPIs con franja superior de color por métrica.
  - **Micro-interacciones**: los números de los KPIs y el gauge hacen *count-up* animado (easeOutCubic), el
    gauge tiñe su halo según el estado del presupuesto, y los nodos del grafo aparecen con un *pop* suave.
  - **Estados**: barra de carga indeterminada + *skeleton* shimmer mientras llega el primer snapshot (sin
    parpadeo brusco), estados vacíos más claros y *hover* de las tarjetas de memoria.
  - **Accesibilidad**: navegación por teclado del grafo (`Tab` + `Enter`/`Espacio`), `aria-label` y anillos de
    foco en los nodos, mejor contraste de texto y todo el movimiento bajo `prefers-reduced-motion`.
- **Path del proyecto en la cabecera**: el snapshot trae un campo `project` (nombre de la carpeta raíz) y el
  dashboard lo muestra («proyecto X»), para no confundir de qué workspace son los datos.

### Changed
- El grafo solo se re-dibuja cuando cambian los datos o el estado (expandido/filtro) — antes se re-renderizaba
  en cada *poll* de 4 s, re-animando los nodos y perdiendo el *hover*. Ahora una firma de render lo evita.

## [0.55.0] - 2026-06-23

### Added
- **Grafo de conocimiento interactivo** (Track 11): el mapa pasa de una «estrella» plana a un grafo de
  **dos niveles, vivo y explorable**:
  - **Drill-down**: cada dominio se abre en sus **sub-temas reales** (`roadmap` → `track-8`, `track-7`…);
    arranca con el más activo ya expandido. Clic en un dominio lo abre **y filtra** las memorias de abajo.
  - **Brillo por recencia**: los temas con actividad reciente brillan; los viejos se apagan.
  - **Hover** → tooltip con conteo, «última actividad hace X» y un ejemplo de memoria.
  - **Aristas curvas con peso** (grosor ∝ nº de memorias, opacidad ∝ recencia) + leyenda.
- **`DbEngine.TopicTree()`** (`internal/memory/topics.go`): arma el árbol dominio → temas de las
  observaciones activas, con conteo y última actividad por nodo (`DomainNode`/`TopicLeaf`). El snapshot de
  `export` ahora expone ese árbol en `graph.domains` (antes solo `{domain, count}`).

### Changed
- `graph.domains` del snapshot ahora es el árbol enriquecido (cada dominio trae `last_activity` y `topics`).
- Las memorias recientes del snapshot suben de 12 a 20 (mejor cobertura del filtro por dominio).

## [0.54.0] - 2026-06-23

### Added
- **Dashboard legible** (Track 11): el dashboard deja de ser solo métricas técnicas y suma contenido que un
  humano puede leer para familiarizarse con Musubi:
  - **«Lo que Musubi recuerda»**: las memorias reales del proyecto en lenguaje claro (tema + resumen + hace
    cuánto), no solo conteos.
  - **«Actividad reciente»**: una línea de tiempo cronológica de lo último que se guardó (la memoria
    «creciendo» mientras trabajás).
  - **Explicaciones**: cada sección técnica con una línea que la traduce a lenguaje claro + tooltips en los
    KPIs.
- **`DbEngine.RecentObservations(limit)`** (`internal/memory/operations.go`): devuelve las últimas
  observaciones NO archivadas en forma legible (`ObsCard`: tema, gist, fecha, importancia); cae al recorte
  del contenido si falta el gist. El snapshot de `export` ahora incluye el campo `recent`.

### Notes
- Frontend en `cmd/musubi/assets/dashboard.html` (data-driven). Tests: `TestRecentObservations` y la
  verificación de `recent` en `TestBuildExportSnapshot`.

## [0.53.0] - 2026-06-23

### Added
- **`musubi dashboard`** (UI local en vivo): nuevo subcomando que sirve una **interfaz web de solo lectura**
  de la memoria —salud, gobernador de tokens (gauge + barras por superficie + umbrales watch/over), checks y
  un **mapa de conocimiento** radial por dominio—. El HTML va **embebido en el binario** (`//go:embed`) y se
  actualiza solo (polling a `/api/snapshot`, que reusa el snapshot de `export`).
  - **Opt-in y cero tokens**: corre como proceso aparte, no se engancha a ningún hook ni inyecta nada al
    contexto del agente. Los datos van de SQLite al navegador, sin LLM en el medio.
  - **Solo loopback** (`127.0.0.1` por defecto, puerto `7777`): por diseño es de uso local; rechaza bind a
    interfaces públicas. Flags: `--addr <host:port>`, `--no-open` (no abrir el navegador).

### Notes
- `dashboard.go` (`runDashboard`, `dashboardHandler`, `isLoopbackAddr`, `openBrowser`) + asset embebido en
  `cmd/musubi/assets/dashboard.html` (data-driven: renderiza desde el JSON y hace polling). Tests:
  `TestDashboardSnapshotEndpoint`, `TestDashboardIndexServesHTML`, `TestIsLoopbackAddr`.

## [0.52.0] - 2026-06-23

### Added
- **`musubi export`** (observabilidad): nuevo subcomando CLI que vuelca un **snapshot JSON** del estado de
  la memoria —salud (`doctor`), insights, ledger de tokens (`tokens`) y un **mapa de conocimiento** por
  dominio de topic— en stdout o a un archivo (`--out <ruta>`). Read-only, model-free, una sola pasada.
  Es la fuente de datos estable para dashboards y observabilidad externa: reúne las mismas vistas que las
  tools MCP en un único documento con forma fija que consumen las UIs.
- **`DbEngine.TopicDomainCounts()`** (`internal/memory/topics.go`): agrega las observaciones activas por el
  **dominio** del topic (prefijo antes del primer `/`; `roadmap/track-7` → `roadmap`), ordenado por cantidad.
  Alimenta el mapa de conocimiento sin LLM (agregación SQL determinista).

### Notes
- `buildExportSnapshot` (`cmd/musubi/export.go`) compone el documento reusando `Diagnose`/`Insights`/
  `LedgerStatus().Budget`/`TopicDomainCounts`; sin duplicar lógica. Tests: `TestBuildExportSnapshot`,
  `TestTopicDomainCounts`.

## [0.51.0] - 2026-06-22

### Added
- **Brevedad del gobernador** (Track 9 / T9.5): nueva superficie por turno `turn_brevity` que inyecta una
  directiva para que el agente responda **conciso**, recortando los tokens de **SALIDA** (las respuestas
  del modelo). Cierra el arco del gobernador de tokens: medir (T9.1) → avisar (T9.3) → **reducir la salida**.
  Hasta ahora todas las superficies solo acotaban la **ENTRADA** (el contexto inyectado); esta toca el otro
  lado del presupuesto. Inspirada en la skill de comunidad `caveman`, pero nativa y atada al gobernador.
- **Config `memory.brevity_mode`** (opt-in, default `off`):
  - `off` — no inyecta nada (sin cambios de comportamiento).
  - `lite` / `full` / `ultra` — fija el nivel de concisión; se inyecta **una vez por sesión** (la directiva
    persiste en contexto, no se repite turno a turno).
  - `auto` — solo dispara cuando el gasto de la sesión cruza `session_token_budget` (mismo umbral que la
    alerta proactiva), de modo que **bajo presupuesto su costo es cero**. Requiere `session_token_budget > 0`.
  - Un valor inválido degrada a `off`: un typo nunca enciende la directiva. Toda directiva **preserva exacto**
    el código, comandos, rutas, nombres de API, versiones y flags.

### Notes
- `buildBrevityNudge`/`brevityDirective` en `turn.go`; throttle por `session_id`+modo (`loop_brevity_injected`).
  La superficie se contabiliza en el ledger holístico como `turn_brevity`. Tests: `TestTurnBrevityManual…`,
  `TestTurnBrevityAuto…`, `TestTurnBrevityOffSilent`, `TestBrevityDirectiveLevelsDiffer`, `TestLoadBrevityMode…`.

## [0.50.0] - 2026-06-22

### Added
- **Pulido de la instalación y el `usage`** (Track 10 / T10.2): tres mejoras de UX del CLI surgidas de la
  auditoría de primera experiencia:
  - **Guardia anti "trampa del doble clic"**: si en el menú interactivo se elige instalar **local** en una
    carpeta que NO parece un proyecto (sin `go.mod`/`package.json`/`.git`/…, típico de hacer doble clic
    sobre el `.exe` en Descargas), Musubi avisa y pide confirmación explícita, sugiriendo la opción Global.
    En un proyecto real procede sin molestar.
  - **Aviso de fragilidad del modo local**: tras `setup` sin instalación global, si el `.mcp.json` queda
    referenciando el binario por ruta absoluta (sin `MUSUBI_BIN` ni `musubi` en el PATH), avisa que mover
    o borrar el binario rompe la carga, con un tip hacia el modo Global (ruta estable).
  - **`usage` agrupado y alineado**: el muro de texto pasa a secciones (Instalación, Servidor MCP,
    Memoria, Catálogo, Binario, Hooks) con columnas alineadas y headers en color.

### Notes
- Helpers `looksLikeProject` (heurística por manifiestos/`.git`), `isYes` (confirmación s/si/y/yes) y
  `confirmLocalDir`. El padding del `usage` se aplica ANTES de colorear, así el alineado no se descuadra
  con o sin ANSI. Tests: `TestLooksLikeProject`, `TestIsYes`.

## [0.49.0] - 2026-06-22

### Added
- **Consola de Windows en UTF-8 + color en el CLI** (Track 10 / T10.1, experiencia de instalación): al
  arrancar, Musubi inicializa la consola de Windows (`SetConsoleOutputCP(CP_UTF8)` + habilita
  `ENABLE_VIRTUAL_TERMINAL_PROCESSING`) — 100% Go vía syscall a kernel32, sin CGo. **Arregla el mojibake
  del primer contacto**: en un cmd.exe fresco (codepage OEM 850/437) los `✓` y acentos que emite `setup`
  salían como basura (`✓`→`Ô£ô`, `Reabrí`→`ReabrÝ`). Ahora renderizan bien y se desbloquea el color ANSI.
  El menú de instalación por doble clic y la salida de `setup` ahora usan color (verde `✓`, headers en
  cyan, énfasis en negrita).

### Notes
- El color es **seguro por defecto**: solo se emite cuando stdout es una TERMINAL real, el VT está
  habilitado y `NO_COLOR` no está seteada. En los hooks, el daemon y los pipes/redirecciones (donde
  stdout es el canal JSON-RPC o una captura) la salida queda **en texto plano** — verificado que
  `setup` piped y `detect --hook-mode` no emiten ANSI y el JSON de hook sigue limpio. Archivos:
  `console_windows.go` / `console_other.go` (build-tagged) y `style.go` (helper de estilo memoizado por TTY).

## [0.48.0] - 2026-06-22

### Changed
- **Superficies por turno delta-aware: fase y conflictos solo se reinyectan al cambiar** (Track 9 / T9.4):
  el recordatorio de fase del pipeline (`turn_phase`) y el aviso de conflictos (`turn_conflicts`) se
  inyectaban **enteros cada turno**. Una simulación de sesión realista contra el ledger holístico
  (`footprint_test.go`) mostró que `turn_phase` era el costo que **más escala**: ~58 tok/turno **sin
  delta** → en una sesión de 40 turnos ≈ **2.300 tokens** repitiendo la misma línea, más que cualquier
  costo de arranque (que es one-time). Ahora ambos siguen el mismo principio que `turn_recall`: se
  inyectan completos **solo cuando cambian** (la fase al avanzar de fase/tarea; los conflictos al
  cambiar la cantidad) y callan mientras tanto (el agente ya los tiene en contexto). Medido: `turn_phase`
  232→58 (primera sesión) y 224→56 (establecida) sobre 4 turnos; el ahorro crece con la longitud de la sesión.

### Notes
- Helper `turnSurfaceChanged` (delta por superficie, con el `session_id` como prefijo para reiniciar al
  cambiar de sesión, igual que el delta del recall). Estado en meta `loop_phase_injected` /
  `loop_conflicts_injected`. Nuevo `footprint_test.go`: simula una primera sesión (proyecto nuevo: dispara
  cognitivo + generación de skills) y una establecida (perfilada) y reporta el footprint por superficie —
  auditoría reproducible que fundamentó esta decisión sobre datos, no intuición.

## [0.47.0] - 2026-06-22

### Added
- **Alerta proactiva del gobernador por turno** (Track 9 / T9.3): cuando el gasto acumulado de la sesión
  cruza el presupuesto blando (`memory.session_token_budget`), el hook por turno (UserPromptSubmit) inyecta
  **una** línea avisando —**una sola vez por sesión** (throttle por `session_id`), para no convertir el
  aviso en ruido—. Cierra el lazo del gobernador: T9.2 lo mostraba **si el agente consultaba**
  `musubi_tokens`; ahora el aviso es **proactivo**, con el desglose a un comando de distancia. Sigue siendo
  **blando** (no recorta nada) y model-free. Con `session_token_budget: 0` queda desactivado.

### Notes
- El aviso vive en `buildBudgetAlert` (lee el ledger ANTES de contabilizar el turno, así que puede atrasarse
  un turno respecto del cruce exacto: oportuno sin ser molesto) y se contabiliza como la superficie
  `budget_alert` del propio ledger. Throttle vía meta `loop_budget_alerted`. `turnOutput` recibe el
  presupuesto desde `cfg.Memory.SessionTokenBudget`.

## [0.46.0] - 2026-06-22

### Added
- **Gobernador de sesión: presupuesto blando de tokens + reporte** (Track 9 / T9.2): nueva opción
  `memory.session_token_budget` (default **8000**, `0` = sin techo) y `musubi_tokens` ahora devuelve el
  reporte del **gobernador**: total, presupuesto, **restante**, **% usado**, **estado** (`ok` <75% ·
  `watch` ≥75% · `over` ≥100%) y el **desglose por superficie ordenado por gasto** (cada una con su % del
  total). Sobre el ledger holístico de T9.1, esto convierte los números crudos en una señal accionable:
  de un vistazo se ve cuánto contexto consume Musubi y **qué superficie** lo domina. Es **blando**: no
  recorta nada (eso arriesgaría eficiencia); solo mide y reporta para que el gasto sea visible y acotable.

### Notes
- El estado/umbrales viven en `TokenLedger.Budget(budget)` (model-free, determinista, testeable). El
  presupuesto es del bloque `memory`; un `session_token_budget: 0` EXPLÍCITO se respeta (opt-out) y no se
  pisa con el default. La alerta PROACTIVA por turno (avisar al cruzar el techo sin que el agente consulte)
  queda para T9.3. Golden de `tools/list` regenerado por el cambio de descripción de `musubi_tokens`.

## [0.45.0] - 2026-06-22

### Changed
- **Ledger holístico de tokens: medir TODAS las superficies, no solo el recall** (Track 9 / T9.1): el
  ledger de tokens (`musubi_tokens`) ahora contabiliza **cada** superficie que inyecta contexto, no
  solo el priming y el recall por turno. Antes quedaban **invisibles** —y por lo tanto sin medir ni
  optimizar— el bloque cognitivo de arranque, las instrucciones de generación de skills, la salud, la
  fase del pipeline, el batch multi-agente, los conflictos, el recordatorio de captura y las dos
  superficies del PreToolUse (memoria de código y errores conocidos). El proyecto creció en superficies
  de contexto pero el ledger seguía mirando solo una: "no podés optimizar lo que no medís". Es el
  cimiento de la evolución del sistema de tokens (medir antes de optimizar, misma disciplina que Track 7).

### Notes
- La contabilidad se centraliza en el punto de **ensamblado** de cada hook (`assembleAccounted`), que
  estima el texto FINAL de cada bloque —header, ids y formato incluidos, que es la huella real que entra
  al contexto— en vez de que cada builder contabilice por su cuenta (la mayoría no lo hacía). Sigue siendo
  model-free y determinista (`EstimateTokens`). Nuevas superficies en el ledger: `startup_health`,
  `startup_cognitive`, `startup_skillgen`, `turn_phase`, `turn_batch`, `turn_conflicts`,
  `capture_reminder`, `precheck_code`, `precheck_telemetry` (se suman a `startup_priming`, `turn_recall`,
  `hydration`, `code_recall`). `startup_priming`/`turn_recall` pasan a medirse sobre el bloque final
  (antes solo el contenido de los gists, sub-reportando el header).

## [0.44.0] - 2026-06-22

### Changed
- **Mejor ranking del catálogo cosechado: tope de skills por repo** (Track 8 / T8.5): el cosechador
  (`musubi catalog harvest`) ahora acota cuántas skills aporta un mismo repo de GitHub (flag
  `--max-per-repo`, default **3**). Las estrellas que reporta el marketplace son del **repo**, no de
  la skill, así que un monorepo enorme y muy estrellado (ej. `openclaw/openclaw` con 379k) inundaba el
  top con skills mediocres y tapaba otras más enfocadas. Con el cap se conservan las N mejores de cada
  repo, dejando lugar a más variedad y relevancia. `--max-per-repo 0` desactiva el tope.

### Notes
- `HarvestMarketplace` aplica el cap sobre la lista ya ordenada por estrellas (se queda con las N de
  mayor ranking por repo). `repoKey` extrae `owner/repo` de la URL de GitHub. Tests: cap por repo,
  modo sin tope, y extracción de `repoKey`.

## [0.43.1] - 2026-06-22

### Fixed
- **`updatedAt` del marketplace tolera número o string** (Track 8): el endpoint de skillsmp
  devuelve `updatedAt` a veces como string (`"1781667763"`) y a veces como número JSON
  (`1781667763`). El struct lo esperaba string, así que una sola entrada con formato numérico
  hacía fallar el decode de **toda la respuesta de esa seed** → en la cosecha real se perdían
  seeds enteras (Go y Node.js, las más importantes). Ahora un tipo tolerante (`flexString`)
  normaliza ambos a string. Detectado al generar el catálogo inicial de producción.
- **El Action de cosecha baja el binario del release en vez de `go install`** (`deploy/musubi-skills/`):
  el `go.mod` declara el módulo como `musubi` (no la URL de GitHub), así que `go install
  github.com/codeabraham16/musubi/cmd/musubi@latest` falla ("module declares its path as: musubi").
  El workflow ahora descarga `musubi-linux-amd64` del último release con `gh release download`.
  Detectado al correr el Action central por primera vez.

## [0.43.0] - 2026-06-22

### Added
- **`musubi_discover_skills` lee un catálogo estático por default** (Track 8 / T8.4, cierra el ciclo):
  el descubrimiento ya **no pega a la API del marketplace en vivo** salvo como fallback. Sirve desde un
  catálogo **curado y estático** (`marketplace-index.json` publicado por el cosechador central),
  cacheado con TTL → **cero rate limit para el usuario** (el límite de 50/día deja de aplicar). Si el
  catálogo no está configurado o no está disponible, cae con gracia a la API en vivo (transición sin
  fricción mientras el archivo aún no existe). La respuesta incluye `"source": "catalog" | "live"`.
- Config `sourcing.marketplace_catalog_url` (default: el `marketplace-index.json` en el repo
  `musubi-skills`). `skillsource.FetchMarketplaceCatalog` (lee el catálogo estático) y
  `skillsource.FilterMarketplaceSkills` (filtra local por query: algún término en nombre/desc/id,
  preservando el orden por estrellas).
- **Workflow del cosechador central** en `deploy/musubi-skills/` (`harvest.yml` + `README.md`): un
  GitHub Action listo para copiar al repo `musubi-skills` que corre `musubi catalog harvest`
  semanalmente (con `SKILLSMP_API_KEY` como secret) y publica el catálogo. Es lo que hace que un solo
  cosechador toque la API y todos los usuarios lean el archivo estático.

### Notes
- Con esto el plan de "las 3 palancas" queda cerrado: API key (T8.1) + caché (T8.2) son el pipeline de
  ingesta que alimenta el catálogo cosechado (T8.3) que se sirve estático (T8.4). El modo live persiste
  como fallback y para `marketplace_catalog_url` vacío.
- Tests: `discover_skills` desde catálogo estático (no toca la API live) y fallback a live cuando el
  catálogo falla; `FetchMarketplaceCatalog` (parseo + error no-fatal) y `FilterMarketplaceSkills`.

## [0.42.0] - 2026-06-22

### Added
- **Cosechador del marketplace** (Track 8 / T8.3, Palanca 3): nuevo subcomando
  **`musubi catalog harvest`** que arma un **catálogo estático** de Agent Skills del marketplace,
  curado por *seeds* (stacks/keywords) y estrellas. La idea del trayecto: en vez de que cada usuario
  pegue a la API en vivo (y choque con el rate limit de 50/día anónimo), un cosechador central corre
  de vez en cuando y publica este JSON; el descubrimiento lo leerá de un archivo (cero rate limit,
  llega en T8.4). No se mirrorea el 1.7M: se cura un subconjunto por relevancia y popularidad.
  Flags: `--seeds a,b,c` (default: Go, Python, Node.js, Rust, …), `--top N` por seed, `--min-stars N`,
  `--out ruta`, `--api-key-env NOMBRE` (default `SKILLSMP_API_KEY`; vacío ⇒ tier anónimo), `--url`.
- **`skillsource.HarvestMarketplace`**: núcleo cosechable y testeable — recibe un `fetch` inyectable
  (sin acoplar a la red), consulta cada seed, **deduplica por id** (gana la de más estrellas), filtra
  por `min-stars` y ordena por estrellas desc (desempate estable por id). Best-effort: una seed que
  falla se omite con warn y la cosecha sigue. `MarketplaceCatalog` es el formato de salida
  (`version`, `generated`, `seeds`, `skills`); el timestamp lo setea el CLI (núcleo determinista).

### Notes
- El cosechador usa **solo metadatos de skillsmp** en esta etapa (id/name/description/githubUrl/stars);
  la validación/enriquecimiento contra GitHub como fuente de verdad queda para un PR siguiente. El
  `discover_skills` sigue en vivo por ahora; T8.4 lo conmuta a leer el catálogo estático por default.
- Un ejemplo del formato vive en `internal/skillsource/testdata/marketplace-index.example.json`
  (validado por test). Escritura **atómica** (temp + rename) reusando el patrón de `catalog merge`.

## [0.41.0] - 2026-06-22

### Added
- **Caché de sourcing con TTL** (Track 8 / T8.2): las respuestas de red del sourcing de skills
  —catálogo curado (`musubi_search_skills`) y marketplace (`musubi_discover_skills`)— se cachean en
  memoria con TTL = `sourcing.cache_seconds` (default 3600s). Una query repetida sale del caché en vez
  de pegar de nuevo a la red: como la query de descubrimiento sin argumentos se deriva del stack y es
  **estable**, esto convierte N llamadas en 1 fetch + (N-1) hits locales, **preservando el rate limit**
  del marketplace (el tier anónimo es de 50/día). Es además la base de ingesta del futuro cosechador
  del catálogo (un harvest re-consulta lo mismo entre corridas; el caché le ahorra presupuesto de API).
  Solo se cachean fetches exitosos (un error transitorio reintenta). `cache_seconds: 0` lo desactiva.

### Notes
- El caché (`sourcingCache`) es seguro para concurrencia: las tools de sourcing son read-only y se
  despachan en paralelo bajo RLock, así que el caché se protege con su propio mutex (limpieza perezosa
  de entradas vencidas). Tests: hit/miss, expiración, modo inerte, y que dos `discover_skills` con la
  misma query pegan al marketplace una sola vez.

## [0.40.0] - 2026-06-22

### Added
- **`musubi_discover_skills`** (Track 8 / T8.1, tool nº27): descubre **Agent Skills** (formato
  SKILL.md) de la comunidad en un marketplace externo (por defecto skillsmp.com, ~1.7M skills
  indexadas de GitHub público), **filtradas por el stack del proyecto**. El marketplace tiene escala
  pero no conoce tu proyecto; Musubi aporta la pieza que falta: si no pasás `query`, la deriva del
  stack detectado (ecosistemas + frameworks). Es un canal **separado** del catálogo curado
  (`musubi_search_skills`) y deliberadamente **solo de descubrimiento**: devuelve metadatos + el
  `githubUrl` de cada skill para que el usuario los **revise e instale por su cuenta**. Musubi nunca
  baja, ejecuta ni instala el SKILL.md (contenido no confiable de GitHub arbitrario; el propio
  marketplace avisa "revisá el código antes de instalar"). Read-only.
- **`skillsource.FetchMarketplaceSkills`**: cliente del endpoint de búsqueda del marketplace
  (`GET /api/v1/skills/search`), con el mismo patrón que `FetchCatalog` (timeout por contexto,
  backstop anti-DoS de tamaño, degradación graciosa). Acota `limit` a [1,100], ordena por estrellas
  y, si hay API key, la envía como `Authorization: Bearer` (sube el rate limit; sin key usa el tier
  anónimo). Omite entradas sin `id` o sin `githubUrl`.
- Config: `sourcing.marketplace_enabled` (bool, **default false: opt-in**), `sourcing.marketplace_url`
  (default `https://skillsmp.com`) y `sourcing.marketplace_api_key_env` (NOMBRE de la env var con la
  API key; el secreto no se guarda en el yaml, mismo criterio que `embedding.api_key_env`).

### Notes
- **Por qué opt-in y solo descubrimiento**: indexar 1.7M SKILL.md de GitHub arbitrario es contenido
  no confiable. Mantenerlo apagado por defecto y limitar a *recomendar + enlazar* (nunca instalar)
  preserva las invariantes de Musubi: local-first (degradación graciosa, red opcional), model-free y
  el modelo de confianza "revisá antes de instalar". No se mergea al gate de aplicabilidad (Hermes):
  el marketplace no expone triggers/capabilities, así que se trata como canal aparte.
- Tests: parseo/mapeo del adapter, armado del request (path, query escapada, `limit` acotado,
  `Authorization` con/sin key), degradación (HTTP≠200, JSON inválido, `success=false`); a nivel tool:
  deshabilitado→guía, query derivada del stack, query explícita con prioridad, marketplace caído→texto.

## [0.39.0] - 2026-06-22

### Changed
- **Mantenimiento ~9× más rápido y 18× menos memoria a escala** (Track 7 / T7.1): un harness de
  benchmarks de escala (`internal/memory/bench_test.go`) reveló que `Maintain` escalaba de forma
  cuadrática (10k observaciones: **37.5s y 3.27 GB**), y el profiler ubicó el cuello real en
  `Consolidate`: el conteo de solapamiento de trigramas reconstruía un `map[int]int` por cada
  observación (el 56% del tiempo se iba en `mapassign`). Como los índices de canónicos son densos, se
  reemplazó ese mapa por un **slice reutilizado** (`overlap []int` + lista de tocados para resetear en
  O(tocados)). Resultado, **a igualdad de resultado** (mismos tests): Maintain 10k baja a **3.97s y
  181 MB** (9.4× / 18×). La super-linealidad asintótica residual (las postings de trigramas crecen con
  n) queda para T7.2 como problema de *set-similarity-join*, con sus propios tests de equivalencia.

### Added
- **`(*ivfIndex).RemoveBatch(ids)`**: saca un lote de observaciones del índice vectorial bajo un único
  `Lock`, agrupando por celda y filtrando cada celda tocada una sola vez (O(celdas tocadas) en vez de
  O(borrados × celda) del loop de `Remove`). Idempotente con ids ausentes o repetidos; deja el índice
  en el mismo estado que llamar `Remove` uno por uno (test de equivalencia). La consolidación, el decay
  y la purga del mantenimiento lo usan en lugar del loop, para no re-tomar el lock por cada id cuando
  hay embeddings. La correctitud del recall ya la garantiza el re-filtro SQL del engine.
- **Job de CI `bench-guard`**: corre `BenchmarkMaintain` a 1k y 10k y falla si la **memoria asignada**
  escala de forma cuadrática (ratio B/op(10k)/B/op(1k) > 20). Se mide memoria y no tiempo a propósito:
  es determinista y estable en runners compartidos. Atrapa una regresión al patrón O(n²) sin falsos
  positivos por ruido de scheduler.

### Notes
- `bench_test.go` usa datasets sintéticos deterministas (seed fija), sin red ni embeddings reales, solo
  stdlib: mide cómo escala el motor (save, recall léxico/híbrido, FTS, vector, Maintain, prime) sin deps
  nuevas. Es la base de medición de Track 7.

## [0.38.0] - 2026-06-20

### Changed
- **`.mcp.json` y hooks portables** (sobreviven a formateos, cambios de usuario y clones en otra
  máquina): `musubi setup` ya no hardcodea la ruta absoluta del binario ni del proyecto para Claude
  Code. El `command` del server se escribe como `${MUSUBI_BIN:-<ruta>}` (resoluble por la env var
  `MUSUBI_BIN`, con la ruta actual como fallback) y se **omite** `MUSUBI_HOME`: el daemon toma la raíz
  del proyecto de `CLAUDE_PROJECT_DIR`, que Claude Code inyecta automáticamente en el entorno del
  server. Los hooks invocan `musubi` por PATH cuando está instalado global. Resultado: el `.mcp.json`
  se vuelve commiteable y no se rompe al reinstalar o mover el proyecto. Cursor y otros agentes que no
  expanden `${VAR}` mantienen rutas absolutas (`AgentTarget.PortableConfig`).
- El instalador **global** (doble-clic, `install.ps1`, `install.sh`) ahora exporta `MUSUBI_BIN` con la
  ruta del binario instalado, además del PATH: al reinstalar tras un formateo, **todos** los proyectos
  con `.mcp.json` portable vuelven a resolver el binario sin tocar ninguno.

### Added
- `workspaceDir` resuelve la raíz con la cadena `MUSUBI_HOME → CLAUDE_PROJECT_DIR → cwd`.
- `AgentTarget.PortableConfig` distingue agentes que soportan config portable (Claude Code) de los que
  no (Cursor).

### Notes
- Tests: `.mcp.json` portable vs absoluto; `workspaceDir` con `CLAUDE_PROJECT_DIR` y su prioridad.

## [0.37.0] - 2026-06-19

### Added
- **`musubi_insights`** (Track 6 / T6.4, cierra Track 6): tool read-only que resume de un vistazo lo
  que Musubi aprendió del proyecto — tamaño de la memoria (observaciones totales / activas /
  archivadas), **hotspots** de archivos con más errores no resueltos, decisiones de skills
  (aceptadas / rechazadas por su decisión más reciente, last-write-wins), último mantenimiento y
  **salud** del ciclo. Es la cara "dashboard" de la observabilidad activa: todo agregación
  SQL/aritmética determinista, sin LLM.
- `(*DbEngine).Insights` + `InsightsReport` (en la interfaz `Insighter` de `StorageBackend`). La tool
  cuenta como tool nº26, clasificada **read-only** (corre concurrente bajo RLock).

### Notes
- Tests: `TestInsights` (observaciones activas/archivadas, errores+hotspots, decisiones last-wins);
  guard de clasificación read-only y golden de `tools/list` actualizados.

## [0.36.0] - 2026-06-19

### Added
- **Surfacing proactivo de errores conocidos** (Track 6 / T6.3): el hook `precheck` (PreToolUse Read)
  ahora, ANTES de que el agente lea un archivo, también surfacea los **errores no resueltos** que
  Musubi tiene registrados de ESE archivo (telemetría), con su `id` y el fix sugerido. "Este archivo
  ya te dio este error, este fue el fix" — sin que el agente lo pida. Se combina con el aviso de
  memoria de código existente; acotado a los 3 errores más recientes para no inundar el contexto.
  - Reusa `GetUnresolvedTelemetryLogsForFiles` (T6.2). El hook sigue siendo best-effort y model-free.

### Changed
- `precheckOutput` se refactorizó en `codeMemoryMessage` + `telemetryMessage` (combina ambas
  superficies); el interfaz `codeStore` del hook ahora también lee telemetría por archivo.

### Notes
- Test: `TestPrecheckSurfacesKnownErrors` (surfacea error + id + fix sugerido).

## [0.35.0] - 2026-06-19

### Changed
- **Telemetría relevante en `musubi_resolve_skills`** (Track 6 / T6.2): en vez de devolver TODA la
  telemetría no resuelta, ahora devuelve solo los errores de los **archivos que el agente está
  tocando** (`modified_files`), matcheando por ruta completa o por nombre base (tolera prefijos y
  separadores `\`/`/` distintos). El error que viste antes en *este* archivo se surfacea; el ruido del
  resto no.

### Added
- `GetUnresolvedTelemetryLogsForFiles(files)` en el motor (+ interfaz `TelemetryStore`): lookup de
  errores no resueltos por archivo, reusable por el hook proactivo (T6.3).
- `TestGetUnresolvedTelemetryLogsForFiles`: match por ruta/basename, exclusión de resueltos, vacío.

## [0.34.0] - 2026-06-19

### Changed
- **`musubi_search_skills` aprende de las decisiones** (Track 6 / T6.1, abre la observabilidad
  activa): el listado de candidatos ahora **excluye las skills que el usuario ya rechazó**
  (`musubi_log_skill_decision` con `decision: rejected`). Cierra el lazo de aprendizaje pasivo: hasta
  ahora `skill_decisions` se escribía pero nadie la consumía, así que una skill rechazada se
  re-proponía en cada sesión.
  - **Last-write-wins**: una skill rechazada y luego aceptada vuelve a proponerse. Matchea por `id`
    (slug), la misma clave que `log_skill_decision`. Best-effort: si la lectura de decisiones falla,
    el listado se devuelve sin filtrar (nunca rompe la búsqueda).

### Added
- `TestExcludeRejectedSkills` (+ caso sin decisiones): valida la exclusión y el last-write-wins.

## [0.33.0] - 2026-06-19

### Added
- **Persistencia del índice IVF (arranque caliente)** (Track 5 / T5.8, cierra Track 5): el índice
  vectorial se serializa a un snapshot binario `<db>.vindex` (magic + dim + centroides, `encoding/binary`
  stdlib) tras cada rebuild. Al arrancar, si el snapshot es válido se **restauran los centroides y se
  reasignan los vectores activos saltando k-means** (el costo caro), en vez de re-entrenar desde cero.
  - El `.vindex` es un **caché derivado y reconstruible**: ante cualquier problema (ausente, corrupto,
    o incompatible) se cae al rebuild normal — nunca panic ni bloqueo de arranque, nunca compromete
    correctness (el engine re-filtra y re-rankea exacto).
  - **Endurecido por revisión adversarial** (16 agentes, 0 críticos/altos): escritura **atómica**
    (tmp + `os.Rename`, sin `.vindex` truncado ante crash); **guard de `k`** que descarta el snapshot
    si la cantidad de centroides diverge >2× de la natural para el `n` actual (dataset que cambió de
    tamaño entre sesiones → evita degradar el recall con `NProbe` fijo); validación de dim (drift de
    modelo) y de cotas (archivo corrupto no dispara asignaciones gigantes).

### Notes
- Tests: `TestVectorIndexWarmStart` (warm-start == rebuild), `TestVectorIndexWarmStartRejectsStaleK`,
  `TestVectorIndexWarmStartDimMismatch`, `TestIndexSnapshotRoundTrip`, `TestReadIndexSnapshotRejectsCorrupt`.
- Limitación conocida documentada: el snapshot no detecta un cambio de modelo de embeddings de la
  misma dimensión (se refresca en el próximo rebuild; agregar un fingerprint cruzaría la capa
  "model-free" del motor). `scoreCandidates`/`targetCentroidCount` ahora compartidos para no divergir.

## [0.32.0] - 2026-06-19

### Added
- **Recall híbrido** (Track 5 / T5.7 R2, la pieza de mayor impacto de la ola): cuando hay un proveedor
  de embeddings, `musubi_recall` suma un **pool de candidatos por similitud vectorial** (coseno) al
  pool léxico (FTS), **unidos por id** (union, no intersección), y agrega una **4ta señal RRF** por
  rango vectorial. Así una consulta como "fixed N+1 query" puede recuperar "database performance
  regression" aunque no compartan palabras. La query se embebe en la capa MCP (best-effort: si el
  embedder falla, el recall sigue 100% léxico).
- `augmentWithVectorPool` + `candidatesByIDs` en el motor; `RecallOptions.QueryVector`.

### Changed
- `scoreCandidates` suma el término vectorial detrás de `vecRank` (mismo patrón que `lexRank`).
  **Sin proveedor de embeddings (`NoopProvider`) el comportamiento es idéntico al histórico** —
  `QueryVector` vacío ⇒ `vecRank` nil ⇒ recall 100% léxico.

### Notes
- Tests: `TestRecallHybridUnionViaVector` (el pool vectorial trae una obs sin match léxico),
  `TestScoreCandidatesVectorSignal`. Cierra T5.7 (el slice de mayor impacto y riesgo de Track 5).

## [0.31.0] - 2026-06-19

### Changed
- **Recall multi-pool** (Track 5 / T5.7 R1, prepara el recall híbrido): `recallCandidates` devuelve
  ahora el ranking keyword (`lexRank`, id→posición) por separado, y `scoreCandidates` toma mapas de
  rank por pool en vez de derivar el rango keyword del orden del slice. Un candidato ausente de un
  pool simplemente no suma ese término RRF. Esto deja listo unir la señal vectorial (R2) sin
  ambigüedad de rangos.
  - **Bit-idéntico al histórico** con `NoopProvider` (solo el pool léxico): toda la batería de tests
    de recall existente pasa sin cambios de comportamiento. `lexRank` nil (fallback por recencia)
    omite el término keyword igual que antes.

### Added
- `TestScoreCandidatesLexRankEquivalence`: garantiza que `lexRank` por orden de slice == el viejo
  `keywordMeaningful=true`, y que nil / id ausente omite el término keyword.

## [0.30.0] - 2026-06-19

### Changed
- **FTS ponderado por IDF-aproximado** (Track 5 / T5.6, abre la ola de recall): nueva
  `buildFTSQueryRanked` que descarta el ruido que diluye el `OR` del `MATCH` — stopwords (lista
  determinista es/en) y tokens de una sola runa (p. ej. la `N` y el `1` de `N+1`) — pero **preserva
  entidades cortas** significativas (`Go`, `DB`, `API`). Si la consulta es toda ruido, cae a
  `buildFTSQuery` para no perder recall. Proxy de IDF determinista, sin LLM.
  - Adoptada en `conflictCandidates` (detección de conflictos) y `EntityContext` (grafo): menos
    ramas `OR`, candidatos más limpios. El path de `musubi_recall` se mantiene en `buildFTSQuery`
    hasta el recall híbrido (T5.7), para no calibrar el RRF sobre un pool que aún cambia.

### Added
- `TestBuildFTSQueryRanked`: descarta stopwords y tokens de 1 runa, preserva `Go`/`DB`/`API`,
  fallback no vacío ante consulta toda de ruido.

## [0.29.0] - 2026-06-19

### Changed
- **Olvido reversible** (Track 5 / T5.5, cierra la ola de autonomía): la consolidación de
  casi-duplicados ahora **archiva** el duplicado (soft-delete: `archived=1` + `archived_at` +
  `superseded_by` al canónico) en vez de **borrarlo físicamente**. Queda oculto del recall pero
  **recuperable**; el borrado definitivo lo hace `PurgeArchived` tras el período de gracia de
  retención (que limpia relaciones y embeddings). Así una fusión por falso positivo de trigramas no
  pierde datos.
- **Decay paginado**: el olvido escanea por **keyset paginado** (`id > lastID`) en vez de cargar todo
  el set activo en RAM, acotando la memoria en bases grandes. La saliencia se sigue computando en Go
  con la **misma fórmula** (no se movió a SQL): el conjunto archivado es **idéntico** al histórico,
  sin riesgo de regresión por diferencias de float/timestamps.

### Added
- **Protección por importancia en el decay**: `maintenance.decay_protect_importance` (float, default 0
  = off). Las observaciones con `importance >=` a ese valor (conocimiento deliberado: decisiones,
  arquitectura) **no se auto-archivan** por más viejas/frías que estén. Nota: Musubi no tiene columna
  `type`; la protección usa `importance`, la señal de "conocimiento deliberado" del esquema real.
- Tests: `TestDecayPaginationEquivalence` (paginado == una-pasada, garantía de no-regresión),
  `TestDecayProtectsHighImportance`, `TestConsolidateSoftDeletesDuplicate`.

## [0.28.0] - 2026-06-19

### Added
- **Auto-curación en el ciclo de mantenimiento** (Track 5 / T5.4): el scheduler de fondo ahora también
  se auto-cura. Tras cada mantenimiento corre `AutoHeal`: diagnostica y **repara automáticamente solo
  los checks de bajo riesgo** (`fts_consistency`, `missing_digests`, `orphan_relations`) en modo apply
  (con backup previo). `db_integrity` y `schema_migrations` quedan **fuera a propósito**: se reportan,
  no se auto-aplican.
- **Salud surfaceada en el arranque**: `AutoHeal` persiste el último `DiagnoseReport` (post-repair) en
  meta (`last_health`); el hook `SessionStart` lo lee (lectura barata, no re-diagnostica) e inyecta una
  advertencia con los problemas **no auto-reparables** si la base no está sana. Si está sana, silencioso.
- `(*DbEngine).AutoHeal` (+ en la interfaz `Doctor`), `buildHealthContext` en el hook de arranque.
- Tests: `TestAutoHealRepairsLowRisk`, `TestHealthContextSurfacesIssues`.

## [0.27.0] - 2026-06-19

### Added
- **Trigger de mantenimiento por volumen de saves** (Track 5 / T5.3): además del ticker temporal de
  T5.2, el daemon dispara ahora un mantenimiento tras `maintenance.auto_after_saves` saves
  (observaciones / hechos / código), para que una sesión intensa no espere al próximo tick. Es
  **opt-in**: `0` = desactivado (default).
  - El disparo es **async** (goroutine): el handler de save ya sostiene el write-lock de `dispatchMu`,
    así que correr el ciclo inline lo re-entraría (deadlock); la goroutine toma el lock al liberarse.
    Respeta el throttle (`MaintenanceDue`) y mantiene **un solo ciclo en vuelo** (`atomic.Bool` CAS);
    el contador es un `atomic.Int64` que se resetea al disparar.
  - Nuevo campo de config `maintenance.auto_after_saves` (int, default 0).
- `TestAutoMaintainAfterSaves`: verifica que cruzar el umbral dispara el mantenimiento y que por
  debajo no.

## [0.26.0] - 2026-06-19

### Added
- **Scheduler de auto-mantenimiento de fondo** (Track 5 / T5.2, corazón de la ola de autonomía): el
  daemon corre ahora el ciclo de mantenimiento (consolidar + olvidar + purgar + compactar) de forma
  recurrente vía un `time.Ticker`, no solo una vez al arrancar. Un daemon long-running se mantiene
  solo, sin necesidad de reinicio.
  - La corrida de arranque pasó a una goroutine best-effort: un `VACUUM` grande ya **no bloquea** el
    primer pedido del daemon.
  - El ticker y la corrida de arranque se **serializan contra el dispatch de tools** tomando el
    write-lock del server (`dispatchMu`, de T4.5) y respetan el throttle de T5.1 (`MaintenanceDue`).
    El ciclo se detiene limpio en el shutdown (cancelación de contexto por señal o EOF de stdin).
  - Métodos nuevos del server: `RunScheduledMaintenance` (una corrida throttled, bajo lock) y
    `RunMaintenanceScheduler` (loop por ticker hasta cancelar el contexto).
- `TestMaintenanceSchedulerRunsAndStops` (corre bajo `-race` en CI: ticker + dispatch concurrente de
  lecturas y escrituras contra el lock exclusivo del mantenimiento) y
  `TestRunScheduledMaintenanceThrottle`.

## [0.25.0] - 2026-06-19

### Changed
- **Throttle + `force` en `musubi_maintain`** (Track 5 / T5.1, abre la ola de autonomía del daemon):
  la tool consulta ahora el throttle del auto-mantenimiento (`MaintenanceDue`) antes de correr. Si el
  último mantenimiento fue hace menos del intervalo configurado (`maintenance.auto_interval_hours`),
  devuelve un no-op informativo (`{skipped: true, reason, last_maintenance}`) en vez de re-disparar
  consolidación + VACUUM. Pasá `force: true` para ignorar el throttle (mantenimiento on-demand
  explícito). Tras correr, marca `last_maintenance`.
  - Protege contra que un agente dispare el ciclo en loop, y establece el contrato `force` que
    reusará el scheduler de fondo (T5.2). `auto_interval_hours: 0` ⇒ sin throttle (siempre corre).
- `musubi_doctor` expone ahora `last_maintenance` para visibilidad del estado del ciclo, sin cambiar
  el contrato `DiagnoseReport` (el campo se suma; los existentes se preservan).

### Added
- `TestMaintainThrottleAndForce` y `TestDoctorExposesLastMaintenance`: guardas del throttle, del
  override por `force` y de la visibilidad de `last_maintenance`.

## [0.24.0] - 2026-06-19

### Changed
- **Concurrencia de lectura en el transporte HTTP** (Track 4 / T4.5): el dispatch ahora usa un
  `sync.RWMutex` y clasifica cada tool por si muta estado. Las **7 tools de solo-lectura**
  (`search_semantic`, `search_keyword`, `recall_facts`, `entity_context`, `conflicts`,
  `detect_stack`, `search_skills`) corren **concurrentes entre sí** (RLock); las que mutan toman el
  lock exclusivo (serializadas, sin lost-updates de read-modify-write). Se removió la serialización
  global del handler HTTP: peticiones de lectura concurrentes ya no se encolan detrás de una sola.
  - La clasificación es **fail-safe**: una tool es de-escritura por defecto; solo se marca
    `readOnly` tras verificar que no escribe DB, ni índice, ni ledger, ni hace `bumpAccess`. (Por eso
    `recall`/`memory_expand`/`recall_code` quedan como escritura: bumpean acceso o registran tokens.)
  - El modo stdio (un goroutine) no cambia: el RWMutex queda siempre libre, costo nulo.

### Added
- `TestToolReadOnlyClassification`: congela el conjunto exacto de tools de solo-lectura y es un guard
  de regresión contra marcar como `readOnly` una tool que muta (bug RMW que `-race` no detecta).
  `TestConcurrentReadDispatch`: dispara tools de lectura en paralelo (corre bajo `-race` en CI).

## [0.23.0] - 2026-06-19

### Added
- **Modo servicio: observabilidad** (Track 4 / T4.4, **cierra el track de modo servicio**). Endpoints
  operativos en el transporte HTTP, todo stdlib (+ el `uuid` ya presente), cero dependencias nuevas:
  - **`GET /healthz`** — liveness (200 si el proceso responde). Sin auth.
  - **`GET /readyz`** — readiness: sondea el motor con una lectura barata (`GetMeta`); 200 si responde,
    503 si no, para que un orquestador no rutee tráfico hasta que la DB esté lista. Sin auth.
  - **`GET /metrics`** — contadores en formato texto Prometheus (`musubi_http_requests_total` por
    resultado: ok / client_error / unauthorized / server_error). Detrás de auth si hay token (datos
    operativos); abierto en loopback sin token.
  - **Correlation IDs**: cada request al MCP recibe un `X-Request-Id` (el entrante si viene, o uno
    nuevo) que se devuelve en la respuesta, para trazar peticiones extremo a extremo.

## [0.22.0] - 2026-06-19

### Added
- **Modo servicio: autenticación, bind remoto y TLS** (Track 4 / T4.3). Habilita exponer el
  servidor MCP más allá de loopback, de forma segura:
  - **Bearer token** (`service.auth_token_env`): nombra una variable de entorno con el token (nunca
    en el YAML, patrón de `embedding.api_key_env`). Si hay token, todo request exige
    `Authorization: Bearer <token>`, comparado en **tiempo constante** (`crypto/subtle`).
  - **Gating de bind**: un `service.addr` **no-loopback exige token** — `musubi serve` se niega a
    arrancar si no lo hay. El bind loopback puede seguir sin auth (default de desarrollo) con la
    defensa anti DNS-rebinding (Host + Origin) ya existente.
  - **TLS opcional** (`service.tls_cert_file` + `service.tls_key_file`): si ambos están, sirve HTTPS.
    Un bind remoto sin TLS **avisa** que el token viaja en texto plano (no bloquea: un proxy que
    termina TLS es válido).
  - La defensa anti DNS-rebinding (Host loopback + Origin local) aplica solo en modo loopback; en
    remoto el token es el gate (los checks de Host romperían clientes legítimos).
- Tests: auth requerido/aceptado/rechazado, `resolveServiceAuth` (matriz loopback × token), y
  `validBearer` (prefijo/trim/constant-time). Cero dependencias nuevas (`crypto/subtle`, stdlib).

### Security
- Endurecimientos fail-closed (de una revisión de seguridad adversarial de la superficie remota):
  - `auth_token_env` nombrada pero con la env var vacía/ausente ahora es **error de arranque** (antes
    deshabilitaba la auth en silencio, contra la intención del operador).
  - Config TLS medio-seteada (solo `tls_cert_file` o solo `tls_key_file`) es **error** (antes
    degradaba a HTTP en texto plano en silencio).
  - Bind remoto con token pero **sin TLS** ahora **falla** salvo `service.allow_insecure_token: true`
    explícito (para deploys con un proxy que termina TLS). Antes solo avisaba.
  - Piso de TLS pineado explícitamente a 1.2 (`tls.Config{MinVersion}`).

## [0.21.0] - 2026-06-19

### Added
- **Modo servicio: transporte HTTP** (Track 4 / T4.2). Nuevo subcomando `musubi serve` que expone
  el servidor MCP sobre HTTP (`POST /mcp`, JSON-RPC 2.0) además del stdio por defecto. Mismo dispatch,
  mismas tools, misma config del motor — corre sobre el seam `Dispatch` de v0.20.0.
  - **Opt-in y seguro por defecto**: bloque de config `service:` con `enabled: false` por defecto; un
    workspace existente sin ese bloque no abre ningún puerto. `musubi serve` se niega a arrancar sin
    `service.enabled: true` (o un `--addr host:port` / `--enable` explícito).
  - **Solo loopback en este release**: bind a `127.0.0.1:7717` por defecto; un `addr` no-loopback es
    error de arranque (la autenticación y el bind remoto llegan en el próximo slice). Defensa de
    superficie: validación de `Host` loopback + rechazo de `Origin` cross-site (mitiga DNS-rebinding),
    techo de body (4 MiB), y timeouts de lectura/escritura/idle contra slow-loris.
  - **Concurrencia serializada**: las peticiones HTTP se serializan sobre un mutex (línea base segura,
    sin riesgo de read-modify-write en el motor). La concurrencia real es un slice posterior, tras la
    auditoría RMW; el seam `Dispatch` ya la deja lista.
  - `GET /mcp` (upgrade SSE) reservado (405): Musubi no emite mensajes server-initiated todavía.
  - **Cero dependencias nuevas**: todo `net/http` + stdlib.
- Tests del transporte HTTP (`http_test.go`): tools/list, initialize, tool-call, notificación→202,
  errores parse/method, `GET`→405, rechazo cross-origin, rechazo de bind no-loopback, y la tabla de
  `isLoopbackHost`.

## [0.20.0] - 2026-06-19

### Changed
- **Seam de dispatch** (Track 4 / T4.1, **abre el track de modo servicio**): se extrajo
  `(*McpServer).Dispatch(ctx, req) (JsonRpcResponse, bool)` del viejo `handleRequest`. Ahora el
  dispatch **devuelve** la respuesta en vez de escribirla a un campo compartido `s.out`; cada
  transporte serializa su propia escritura (`writeResponse(out, resp)`). Esto **elimina el único
  hazard de memoria** del servidor (la mutación de `s.out` + `send`) y deja a `Dispatch` seguro para
  llamarse concurrentemente — el prerequisito para los transportes de red de Track 4 (HTTP en v0.21.0).
  - El modo stdio (`musubi daemon`) queda **idéntico en comportamiento**: un goroutine, secuencial,
    60s por request, shutdown graceful. Solo cambió la plomería interna.
  - `Dispatch` lee únicamente estado fijado en `NewMcpServer` (registro de tools, motor, embedder,
    config) y no muta nada compartido; los handlers no escriben campos del servidor.

### Added
- Test de concurrencia `TestDispatchConcurrentSafe`: 64 goroutines disparan lecturas y escrituras
  en paralelo contra un servidor + motor compartidos (saves que ejercitan el `Add` al índice IVF y
  el rebuild en background, búsquedas que toman el RLock, `tools/list`). Corre bajo `-race` en CI
  como red de seguridad permanente de la concurrencia.

## [0.19.0] - 2026-06-19

### Added
- **Interfaz `StorageBackend`** (Track 3 / T3.2): el contrato completo que un backend de memoria
  debe cumplir para servir a la app (servidor MCP + CLI). `*memory.DbEngine` (SQLite local-first,
  puro Go, model-free) es la implementación de referencia; un backend alternativo —p.ej. el modo
  servicio de Track 4— implementa la misma interfaz **sin que los consumidores cambien**. Es el seam
  de extensibilidad de Track 3.
  - Compuesta de interfaces de rol chicas (idioma Go: "interfaces chicas, compuestas") —
    `ObservationStore`, `GraphStore`, `RelationStore`, `WorkStore`, `WorkflowStore`, `LedgerStore`,
    `MetaStore`, `PhaseStore`, `Maintainer`, `Doctor`, `Calibrator`, etc. — para que cada consumidor
    dependa solo del subconjunto que usa.
  - `internal/mcp` ahora depende de `memory.StorageBackend`, no de `*memory.DbEngine` concreto.
    Esto **desacopla el layer MCP del motor** y habilita tests de handlers en aislamiento con un
    backend falso (ver `TestStorageBackendSeam_ConflictsViaFake`).
  - Aserción en tiempo de compilación `var _ StorageBackend = (*DbEngine)(nil)`: agregar un método al
    contrato que el motor no implemente —o cambiar una firma— rompe la compilación de inmediato.

### Fixed
- El test golden de `tools/list` ahora normaliza el fin de línea (CRLF→LF) antes de comparar: era
  frágil en working trees de Windows con `git autocrlf` (el repo guarda LF pero el checkout deja CRLF).
  CI (Linux) no se veía afectado; el fix lo hace robusto en cualquier entorno.

## [0.18.0] - 2026-06-19

### Added
- **Registro de tools map-based** (Track 3 / T3.1, **abre el track de velocidad y extensibilidad**).
  Agregar una herramienta MCP exigía mantener sincronizados TRES lugares (el schema en `tools/list`,
  un `case` en el switch de `tools/call`, y un conteo manual en los tests). Ahora cada tool es una
  sola `toolEntry` (`internal/mcp/registry.go`) que liga su schema con su handler; `tools/list` itera
  el registro en orden y `tools/call` resuelve por mapa en O(1). **Agregar una tool = una entrada**.
  Las firmas que no usan el `context` del request se adaptan con `noCtx` sin tocar el cuerpo del handler.
- Test **golden** del catálogo (`TestToolsListGolden` + `testdata/toolslist.golden.json`): congela la
  salida JSON exacta de `tools/list` (nombres, descripciones, schemas y orden) — el refactor quedó
  probado byte-idéntico. Test de **consistencia estructural** (`TestRegistryConsistency`): garantiza que
  la lista de schemas y el mapa de dispatch sean siempre el mismo conjunto (sin tools sin handler ni
  handlers huérfanos).
- **CI endurecido**: `golangci-lint` (gate con `.golangci.yml`: linters estándar + preset de
  manejo de errores idiomático), **piso de cobertura** (CI falla si baja de 70%), `govulncheck`
  (escaneo de vulnerabilidades) y **Dependabot** (módulos Go + GitHub Actions). Antes el CI solo
  corría `vet`/`build`/`test -race`.

### Changed
- El dispatch de `tools/call` pasó de un `switch` de 25 ramas a una búsqueda por mapa
  (`s.toolIndex[name]`); la lista de `tools/list` pasó de un slice hand-mantenido a la iteración del
  registro. Comportamiento idéntico (verificado con el golden + verificación adversarial del binding
  nombre→handler contra el baseline).

### Fixed
- Limpieza de lint: eliminado el `const charsPerToken` muerto; mensajes de error de Ollama en
  minúscula (ST1005); comentarios de paquete en `memory`, `skills`, `mcp` y el comando `musubi`.

## [0.17.0] - 2026-06-19

### Added
- **Retención y compactación de memoria** (Track 1 / T1.3, **cierra el track de cimientos de datos**).
  Acota el crecimiento perpetuo de la base y reclama espacio, manteniéndose local-first y model-free:
  - **Purga dura** (`PurgeArchived`): borra DEFINITIVAMENTE las observaciones archivadas cuyo
    `archived_at` supera la ventana de retención (`maintenance.purge_archived_after_days`, default 90),
    en una transacción que limpia embeddings (FK CASCADE), relaciones semánticas y punteros
    `superseded_by`. El olvido (decay) solo marcaba `archived` sin borrar nunca; esto las elimina.
  - **Compactación física** (`Compact`): `wal_checkpoint(TRUNCATE)` + `PRAGMA optimize` siempre, y
    `VACUUM` tras una purga que borró filas (`maintenance.vacuum`, default true).
  - **`engine.Maintain`** centraliza el ciclo (consolidar → olvidar → purgar → compactar); lo comparten
    el subcomando `maintain`, el auto-mantenimiento del daemon y la tool MCP `musubi_maintain`.
  - Columna `archived_at` (migración v3): la ventana de retención cuenta **desde el archivado**
    (período de gracia), no desde el último acceso.
  - Índice `idx_obs_archived` (migración v2) — primera migración post-baseline, sobre el framework de v0.15.0.

### Changed
- **Consolidación O(n²) → ~O(n)**: índice invertido de trigramas + bucket de igualdad exacta, en vez de
  comparar cada observación contra todos los canónicos. Resultado idéntico al algoritmo previo (verificado
  con un test diferencial); escala a bases grandes.
- Tuning explícito del pool de conexiones SQLite (`SetMaxOpenConns`/`Idle`/`ConnMaxIdleTime`).
- Hidratación de observaciones (`expand.go`) ahora respeta el `context` del caller (variantes `…Ctx`),
  en vez de un `context.Background()` interno que ignoraba el deadline.

### Fixed
- La purga (hard-delete irreversible) **ya no se habilita por un upgrade silencioso**: un config sin bloque
  `maintenance` queda con la purga desactivada; solo se activa con el campo explícito.
- `Decay` trocea su `UPDATE … IN (…)` (antes podía superar el tope de parámetros y abortar el ciclo de
  mantenimiento en bases grandes).
- Al consolidar una observación que era fuente de un `supersede`, los punteros `superseded_by` se
  re-apuntan al canónico (la observación ocultada sigue oculta, no reaparece en el recall).

## [0.16.0] - 2026-06-19

### Added
- **Índice vectorial IVF para búsqueda semántica a escala** (Track 1 / T1.2). Reemplaza el
  full-scan O(n) de la búsqueda semántica (que cargaba y deserializaba **todos** los embeddings
  por query y se degradaba a ~10k observaciones) por un índice invertido por centroides k-means,
  **model-free y en Go puro** (sin dependencias nuevas, sin CGo). Diseño elegido por un panel
  multi-agente (IVF sobre HNSW/SQ8) y validado con verificación adversarial:
  - **No retiene vectores en RAM**: solo centroides + la membresía de cada celda (ids). Footprint
    residente ~10-90 MB incluso a 1M de observaciones; los vectores se cargan de SQLite **solo**
    para las celdas sondeadas.
  - **Exacto por debajo del umbral**: con menos de `exact_threshold` embeddings (o índice sin
    entrenar, o dimensión incompatible) la búsqueda es el full-scan exacto de siempre. Por encima,
    el IVF solo **acota** candidatos y el ranking final sigue siendo coseno **exacto**, re-filtrado
    `archived=0 AND superseded_by IS NULL` contra SQLite: el índice nunca compromete la correctitud
    (a lo sumo, el recall entre rebuilds). Test de regresión exige **recall@10 ≥ 0.92**.
  - k-means++ (sembrado D²) + reseed de centroides muertos; manejo de drift de dimensión
    (entrena con la dim mayoritaria); updates incrementales (`Add`/`Remove`) y re-entrenamiento
    throttled en segundo plano.
  - Bloque de config `vector_index` (`enabled`, `exact_threshold`, `nprobe`, `rebuild_*`, `kmeans_*`).

### Changed
- `internal/memory`: `SearchObservations` ahora despacha entre el camino IVF y el full-scan exacto
  (conservado intacto como `searchExactFullScan`). `saveObservation` mantiene el índice al día
  post-commit; `Decay` y la marca de superseded lo sincronizan.
- Lifecycle del `DbEngine`: `Close()` espera a las tareas de índice en segundo plano antes de
  cerrar la base (evita use-after-close del `*sql.DB`).

## [0.15.0] - 2026-06-19

### Added
- **Esquema versionado con migraciones** (`PRAGMA user_version`): runner que aplica las
  migraciones pendientes, **cada una en su propia transacción** (DDL + bump de versión atómicos;
  si una falla, rollback y la versión no avanza). La migración `baseline` encapsula el esquema
  histórico completo + las columnas de eficiencia de memoria; es idempotente sobre bases
  preexistentes (una base v0.14 solo avanza su `user_version` sin reescribir nada). Track 1 (T1.1)
  del rumbo de escalabilidad perpetua: habilita cambios de esquema NO aditivos (renames, tipos,
  tablas nuevas con backfill) de forma ordenada y resumible, que antes no tenían camino de upgrade.

### Changed
- `internal/memory/database.go`: el esquema (`initSchema`/`migrateObservations`) se refactorizó
  sobre una interfaz `execQuerier` (satisfecha por `*sql.DB` y `*sql.Tx`) para que la migración
  baseline corra dentro de una transacción. Los métodos previos se conservan como wrappers (sin
  cambio de comportamiento para el auto-repair del doctor ni los tests). Los backfills que dependen
  de la versión del estimador de tokens siguen como pasos idempotentes post-migración.

## [0.14.0] - 2026-06-18

### Added
- Soporte multi-agente en `musubi setup`: `--agent <claude|cursor>` registra el servidor MCP
  en la config del agente (`.mcp.json` para Claude, `.cursor/mcp.json` para Cursor). Abstracción
  `AgentTarget` + detección de agentes presentes en el proyecto. Los hooks siguen siendo
  específicos de Claude Code. Track B del roadmap.

## [0.13.0] - 2026-06-18

### Added
- **Motor de orquestación DAG (model-free)** — tool `musubi_workflow` (`start`/`next`/`complete`/`status`/`resume`).
  Musubi define el grafo (`.musubi/workflows/<id>.yaml`), persiste el estado del run en SQLite
  (tabla `workflow_runs`, **resumible entre sesiones**) y devuelve los steps listos; el agente
  ejecuta. Un step queda listo cuando todas sus `needs` están `done` o `skipped`. Tracks A1+A2.
- Control de flujo en workflows: un step puede llevar `when` (expresión model-free, ej.
  `step.build.result contains ok`); si es falsa el step se salta (`skipped`), expresando
  gate/if_then/switch sin tipos de step separados. Evaluador de expresiones seguro (sin eval).
- `musubi_workflow action=resume` para retomar un run en otra sesión (estado + steps listos).
- Loops en workflows: un step con `repeat_while` (+ `max_iterations`, cota anti-infinito) se
  re-ejecuta mientras la condición sea verdadera. Tracks A3.
- `musubi_workflow action=validate` (valida una definición sin correrla) y `action=list`
  (lista los runs con su progreso). Con esto Track A (motor DAG) queda completo.
- Templates de artefactos SDD (`proposal`/`spec`/`design`/`tasks`) versionados: `musubi setup`
  los deja en `.musubi/templates/sdd/`. Scaffold con `schema_version`, idempotente.
- `docs/Roadmap_spec-kit_adoption.md`: plan de orquestación DAG, multi-agente y templates SDD
  (inspirado en spec-kit, adaptado a local-first/model-free).

## [0.12.0] - 2026-06-18

### Added
- Skill cognitiva `audit-structure-flow` en el bundle de arranque: cada `musubi setup`
  la escribe en `.musubi/skills/`. Audita estructura y flujo del codebase (organización,
  acoplamiento, capas, ciclos, código muerto, propagación de context/errores) con
  hallazgos priorizados. También publicada en el catálogo de skills (#47, #48).
- VERSIONINFO del `.exe` reproducible: `cmd/musubi/versioninfo.json` + `go:generate`
  como única fuente de verdad (antes se editaban los `.syso` a mano) (#43).
- README con banner SVG animado y diagramas Mermaid (arquitectura, auto-descubrimiento,
  loop por turno) (#45).

### Changed
- Higiene de estructura (sin cambio de comportamiento): eliminado el paquete huérfano
  `internal/telemetry`; `methods.go` partido (1386→1073) extrayendo el catálogo de tools;
  `main.go` partido (601→207) a `setup.go` e `install.go` (#46).
- Más cobertura de tests en `cmd/musubi` (helpers de setup, calibrate, doctor, catalog) (#44).

## [0.11.0] - 2026-06-18

### Added
- Proveedor de embeddings `openai`: usa la API de OpenAI o cualquier servidor
  compatible con su esquema (LM Studio, vLLM, LocalAI…). La API key se lee de una
  env var (`api_key_env`, default `OPENAI_API_KEY`) y nunca se guarda en el yaml.
- `LICENSE` (MIT), este `CHANGELOG.md` y `CONTRIBUTING.md`.
- Plantillas de issue/PR en `.github/` y badges de CI, release y licencia en el README.

### Changed
- Hardening de robustez: propagación de `context.Context` con timeouts en la capa
  de memoria/embeddings, chequeo de `rows.Err()`, graceful shutdown del daemon
  (SIGINT/SIGTERM), recuperación de panics en los handlers JSON-RPC y validación
  del campo `jsonrpc`.
- Cobertura de tests: `internal/mcp` a 75.8% y `cmd/musubi` a 45.6%.

### Fixed
- `extract_deps`: parseo correcto de dependencias tipo `pydantic[extras]>=2.0`.

## [0.10.0] - 2026-06-16

### Added
- Memoria de código automática: hook `PreToolUse(Read)` que muestra el gist de un
  archivo antes de leerlo (#40).
- Gists de archivos con frescura por fingerprint, model-free (#39).

## [0.9.1] - 2026-06-16

### Changed
- Fin de la doble inyección priming↔turno: el priming siembra el delta (#38).
- Documentado el sistema de eficiencia de tokens; `calibrate` es opcional y gratis.

### Added
- Test de auditoría del footprint de tokens de Musubi (#37).

## [0.9.0] - 2026-06-16

### Added
- Calibración opt-in del estimador de tokens contra `count_tokens`, con
  contabilidad del priming (#36).

## [0.8.0] - 2026-06-16

### Added
- Núcleo de eficiencia de tokens: estimador calibrado + ledger + inyección delta,
  todo model-free (#35).

## [0.7.3] - 2026-06-16

### Fixed
- Resueltos los hallazgos BAJO de la auditoría completa (#34).

## [0.7.2] - 2026-06-16

### Fixed
- Hardening: arreglados los 9 hallazgos ALTO/MEDIO de la auditoría multi-agente (#33).

## [0.7.1] - 2026-06-16

### Changed
- Hardening de la capa de orquestación (auditoría multi-agente) (#31).

## [0.7.0] - 2026-06-16

### Added
- Multi-agente: pizarra compartida (`musubi_work`) para orquestar sub-agentes,
  model-free (#30).

## [0.6.0] - 2026-06-16

### Added
- Loop dirigido + pipeline por fases (`musubi_phase`) para orquestación model-free (#29).

## [0.5.0] - 2026-06-16

### Added
- Resolución de conflictos semánticos entre observaciones, model-free (#28).
- `musubi doctor` con auto-repair (y backup).

## [0.4.0] - 2026-06-15

### Changed
- Mejoras internas y bump de VERSIONINFO del `.exe` (#27).

## [0.3.1] - 2026-06-15

### Fixed
- VERSIONINFO del `.exe` actualizada (#25).

## [0.3.0] - 2026-06-15

### Added
- Auto-update del binario: comando `musubi update` + aviso de versión nueva al
  arrancar el daemon (#24).

## [0.2.4] - 2026-06-14

### Added
- Doble clic en `Musubi.exe` muestra el menú de instalación (local/global) (#18).

## [0.2.3] - 2026-06-14

### Fixed
- Reducción de falsos positivos de antivirus: VERSIONINFO en el `.exe` +
  checksums SHA-256 en las releases (#17).

## [0.2.2] - 2026-06-14

### Changed
- La release publica el binario de Windows como `Musubi.exe` (#16).

## [0.2.1] - 2026-06-14

### Added
- Icono embebido en el `.exe` de Windows (#15).

## [0.2.0] - 2026-06-14

### Added
- Instalador con elección de alcance: local al repo o global en la PC (#13).

## [0.1.0] - 2026-06-13

### Added
- Distribución inicial: instaladores de una línea, workflow de release y setup
  por doble clic.
- Servidor MCP en Go con memoria persistente local-first sobre SQLite (FTS5 +
  búsqueda semántica opcional vía Ollama), resolución dinámica de skills y
  telemetría de errores.

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.106.0...HEAD
[0.106.0]: https://github.com/codeabraham16/musubi/compare/v0.105.0...v0.106.0
[0.105.0]: https://github.com/codeabraham16/musubi/compare/v0.104.0...v0.105.0
[0.104.0]: https://github.com/codeabraham16/musubi/compare/v0.103.0...v0.104.0
[0.103.0]: https://github.com/codeabraham16/musubi/compare/v0.102.1...v0.103.0
[0.102.1]: https://github.com/codeabraham16/musubi/compare/v0.102.0...v0.102.1
[0.102.0]: https://github.com/codeabraham16/musubi/compare/v0.101.0...v0.102.0
[0.101.0]: https://github.com/codeabraham16/musubi/compare/v0.100.0...v0.101.0
[0.100.0]: https://github.com/codeabraham16/musubi/compare/v0.99.0...v0.100.0
[0.99.0]: https://github.com/codeabraham16/musubi/compare/v0.98.2...v0.99.0
[0.98.2]: https://github.com/codeabraham16/musubi/compare/v0.98.1...v0.98.2
[0.98.1]: https://github.com/codeabraham16/musubi/compare/v0.98.0...v0.98.1
[0.98.0]: https://github.com/codeabraham16/musubi/compare/v0.97.0...v0.98.0
[0.97.0]: https://github.com/codeabraham16/musubi/compare/v0.96.0...v0.97.0
[0.96.0]: https://github.com/codeabraham16/musubi/compare/v0.95.0...v0.96.0
[0.95.0]: https://github.com/codeabraham16/musubi/compare/v0.94.0...v0.95.0
[0.94.0]: https://github.com/codeabraham16/musubi/compare/v0.93.0...v0.94.0
[0.93.0]: https://github.com/codeabraham16/musubi/compare/v0.92.0...v0.93.0
[0.92.0]: https://github.com/codeabraham16/musubi/compare/v0.91.0...v0.92.0
[0.91.0]: https://github.com/codeabraham16/musubi/compare/v0.90.0...v0.91.0
[0.78.0]: https://github.com/codeabraham16/musubi/compare/v0.77.0...v0.78.0
[0.44.0]: https://github.com/codeabraham16/musubi/compare/v0.43.1...v0.44.0
[0.43.1]: https://github.com/codeabraham16/musubi/compare/v0.43.0...v0.43.1
[0.43.0]: https://github.com/codeabraham16/musubi/compare/v0.42.0...v0.43.0
[0.42.0]: https://github.com/codeabraham16/musubi/compare/v0.41.0...v0.42.0
[0.41.0]: https://github.com/codeabraham16/musubi/compare/v0.40.0...v0.41.0
[0.40.0]: https://github.com/codeabraham16/musubi/compare/v0.39.0...v0.40.0
[0.39.0]: https://github.com/codeabraham16/musubi/compare/v0.38.0...v0.39.0
[0.17.0]: https://github.com/codeabraham16/musubi/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/codeabraham16/musubi/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/codeabraham16/musubi/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/codeabraham16/musubi/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/codeabraham16/musubi/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/codeabraham16/musubi/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/codeabraham16/musubi/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/codeabraham16/musubi/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/codeabraham16/musubi/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/codeabraham16/musubi/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/codeabraham16/musubi/compare/v0.7.3...v0.8.0
[0.7.3]: https://github.com/codeabraham16/musubi/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/codeabraham16/musubi/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/codeabraham16/musubi/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/codeabraham16/musubi/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/codeabraham16/musubi/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/codeabraham16/musubi/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/codeabraham16/musubi/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/codeabraham16/musubi/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/codeabraham16/musubi/compare/v0.2.4...v0.3.0
[0.2.4]: https://github.com/codeabraham16/musubi/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/codeabraham16/musubi/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/codeabraham16/musubi/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/codeabraham16/musubi/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/codeabraham16/musubi/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/codeabraham16/musubi/releases/tag/v0.1.0
