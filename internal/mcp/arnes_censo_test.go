package mcp

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/arnes"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// LA DEUDA DE SABOTAJES SE MIDE EN CI, Y NO PUEDE CRECER EN SILENCIO
//
// El árbol promete un sabotaje por guarda y lo escribe en prosa. Medido el 2026-09-12 sobre
// `git ls-files` en la base 00728f1, y cada número con su definición al lado porque son preguntas
// distintas:
//
//	774 anclas en 163 archivos  ← con el predicado de `arnes.esAncla`, que está escrito allá
//	396 anclas en  93 archivos  ← las que usan la frase canónica: lo que cuenta el cabo A123
//	3.213 funciones Test        ← el denominador, contado con el AST: las 774 son el 24 %
//	 38 con veredicto alguna vez (A107 y A120, a mano) — el 4,9 % de 774
//	  6 de esas 38 NO rompían lo que decían romper — el 15,8 % de lo auditado MIENTE
//
// A123 NO CUENTA MAL, CUENTA INCOMPLETO, y la distinción no es cortesía: su 396/93 se reprodujo al
// dígito. Lo que pasa es que enumera UNA frase y el árbol escribe la promesa de 30 formas, así que
// quedan 378 anclas afuera y 70 archivos de prueba en los que NINGUNA ancla usa la canónica. De
// esas formas, 39 anclas llevan el encabezado en MAYÚSCULAS y sólo 3 son la frase canónica: la
// dominante en mayúsculas es `SABOTAJE:`, con 30.
//
// Es el defecto dominante del repo —una guarda que enumera formas no converge— aplicado a la
// contabilidad de su propia deuda.
//
// PERO EL 774 ES LA SALIDA DE UN PREDICADO Y NO UN HECHO DEL MUNDO. Otra sesión lo replicó con su
// propio go/ast: reprodujo 396/93 al dígito, y para el total su predicado da 569 y la cota superior
// —cualquier mención— da 955. El 774 cae adentro de ese bracket, o sea que es defendible y a la vez
// hipersensible a la definición. Lo que converge es el conteo de ARCHIVOS (160 contra 163). El
// predicado exacto está en `arnes.esAncla`, y EL TECHO DE ABAJO ESTÁ MEDIDO CON ÉL: si alguien
// cambia el predicado, este número deja de valer y hay que re-medirlo en el mismo commit.
//
// Y el 3.213 se cuenta con el AST: `grep "^func Test"` da 3.219 porque cuenta seis `func TestMain`
// que viven adentro de literales crudos en `internal/testbudget/paquetes_test.go`. Acá estuvo
// escrito el 3.219 y era el número del grep.
//
// Y LO QUE ESTE CENSO NO VE, que lo levantó otra sesión: hay guardas saboteadas de verdad cuya
// evidencia vive en el MENSAJE DEL COMMIT y no en el fuente (`normalizacion_fijada_test.go` y
// `carga_streaming_test.go`: cero menciones de la palabra, las dos verificadas). Un sabotaje
// declarado en un commit no se vuelve a correr: se cumplió una vez y después no es auditable.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTA GUARDA EXIGE, Y POR QUÉ CADA COSA
//
// Correr los 774 sabotajes cuesta horas —cada uno son cuatro invocaciones de `go test`— y no entra
// en CI. Eso es `go run ./deploy/cmd/arnes -correr`, a mano y por lotes. Lo que SÍ entra en CI, en
// milisegundos, es comprobar que el corpus no se podrió:
//
//	· CERO DIRECTIVAS ROTAS. Una directiva que no se puede leer se contaría como «mecanizada» y
//	  nadie la correría: la cobertura subiría con sabotajes que no existen.
//	· CERO DIRECTIVAS QUE DEJARON DE APUNTAR. Si el `de` ya no está en el archivo —porque el código
//	  se movió— el sabotaje NO SE APLICA, y su verde se lee igual que «la guarda cubre». Es la
//	  tercera cara de «el `-run` del sabotaje escrito a mano».
//	· CERO ANCLAS SIN UBICAR. Un agujero en el enumerador se ve idéntico a un árbol sin deuda.
//	· LA DEUDA NO CRECE. Un ancla nueva sin directiva es una promesa nueva que nadie puede correr.
//
// Sabotaje que la hace fallar: agregar un `// Sabotaje: algo` a cualquier `_test.go` sin su
// `arnes:` (la deuda sube y pasa el techo), o cambiarle el `de` a cualquier directiva por un texto
// que no exista.
// `prueba=` VA DECLARADO PORQUE ESTA ANCLA FLOTA: vive en la cabecera del archivo y no pegada al
// `func Test…`, así que el lector no la puede derivar del AST. Y no la saltea en silencio — la
// denunció en su primera corrida, que es cómo apareció esta línea.
//
// LA PRIMERA DIRECTIVA DE ESTA ANCLA ERA UN ROJO FALSO, y sobrevivió siete corridas porque daba
// ROJO. Saboteaba la unicidad de `arnes.Aplicar`, y ESTA GUARDA NO LLAMA A `Aplicar`: sólo llama a
// `Censar` y a `Validar`. Se ponía roja igual porque el sabotaje cambiaba una línea que OTRAS DOS
// directivas usan como su `de`, las dos dejaban de apuntar, y `Validar` denunciaba eso. O sea que
// la prueba caía por el daño al corpus, no por el defecto declarado — un rojo real por el motivo
// equivocado, con el archivo, la prueba y la línea del fallo todos correctos.
//
// Un verde inesperado hace preguntar; un rojo esperado no. Por eso la encontró un refutador ajeno
// que estaba midiendo otra cosa, y no yo. La comprobación que ahora la habría cazado al escribirla
// es `arnes.Colisiones`.
//
// Y LA SEGUNDA DIRECTIVA TAMBIÉN ERA UN ROJO FALSO, por la misma razón estructural y por eso vale
// escribirla: esta guarda llama a `Validar`, y `Validar` LEE `internal/arnes/arnes.go` DEL DISCO en
// tiempo de ejecución. O sea que CUALQUIER sabotaje a ese archivo que caiga sobre el `de` de otra
// directiva la hace autodetectarse, y `arnes_test.go` cubre ese archivo tan densamente que casi
// toda línea interesante es el ancla de alguien. No es que elegí mal dos veces: este archivo no se
// puede sabotear honestamente para ESTA guarda.
//
// La salida es la que la prosa de arriba ya decía y yo no había leído bien: el sabotaje que ejercita
// a esta guarda NO es tocar el lector, es AGREGARLE DEUDA AL CORPUS. Una línea `// Sabotaje:` nueva
// en un `_test.go` sin su `arnes:` sube la deuda de 712 a 713 y el techo se pone rojo — que es
// exactamente la primera de las dos formas que el párrafo de arriba nombra, textual.
//
// Saborear un `_test.go` es la excepción justificada a «no saboteés un archivo de prueba»: esa regla
// existe para que una guarda no se mida contra sí misma, y acá el SUJETO de la guarda es el corpus
// de archivos de prueba. El archivo elegido no es el suyo ni el del lector, y como `a` contiene a
// `de` entero, no le rompe el ancla a nadie: `Colisiones` da 0.
//
// ESTA GUARDA ES LA ÚNICA DEL ÁRBOL QUE SE VUELVE NO-MEDIBLE POR SU PROPIO SUJETO, y conviene
// saberlo antes de ir a buscar el defecto. Mide la deuda del árbol; si la deuda está pasada, su
// CONTROL arranca en rojo —la prueba ya falla sin el sabotaje— y `sabotaje.sh` se abstiene con
// «CONTROL EN ROJO: su rojo no prueba nada». Abstenerse es lo correcto: un rojo que ya estaba no
// dice nada sobre el sabotaje, y acreditárselo sería contar como cobertura un rojo ajeno.
//
// Pero la consecuencia es una recursión: MIENTRAS EL TECHO ESTÉ PASADO, LA DIRECTIVA QUE CUSTODIA
// EL TECHO NO SE PUEDE VERIFICAR. Se destraba sola en cuanto alguien escribe la directiva que
// faltaba. Lo midió otra sesión corriendo este arnés contra un árbol con la deuda en 714, y lo
// escribo acá para que el próximo que vea «sin veredicto» acá no busque el defecto en la directiva.
// arnes: prueba="TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre"
// arnes: archivo="internal/mcp/sonda_permiso_test.go"
// arnes: de="package mcp"
// arnes: a="package mcp\n\n// Sabotaje: un ancla nueva SIN su directiva, puesta a propósito para que suba la deuda."

// anclasEnProsaAlDia es LA DEUDA MEDIDA, con su fecha. Es un hecho del mundo —cuántas promesas sin
// ejecutar tenía el árbol ese día— así que va clavado: derivarlo del árbol dejaría a esta guarda
// midiéndose contra sí misma y aceptando cualquier número.
//
// SÓLO BAJA. Si tu cambio la sube, la salida NO es subir esta constante: es escribir la directiva
// `arnes:` de tu ancla nueva, o declararla `no_mecanizable="<motivo>"` si el sabotaje literal no
// puede compilar. Subir el techo es exactamente cómo una guarda se convierte en la defensora del
// defecto que vigila — y este repo ya lo pagó con `TestG1`, que enumeraba cuatro herramientas y
// PROHIBÍA arreglar las otras seis.
//
// Y SI EL TECHO SE PASA, MIRÁ LAS DOS PRIMERAS CONDICIONES DEL PREDICADO, NO LA TERCERA. Otra
// sesión reimplementó `esAncla` leyendo SÓLO su descripción en prosa —sin ver el código— y obtuvo
// 774/163 al dígito, así que el número es reproducible desde el texto. Pero además midió la
// sensibilidad de cada cláusula, y el reparto es desparejo:
//
//	el predicado completo ........................ 774
//	sin los «:» como cierre de oración ........... 774
//	sin que la línea vacía cuente como cierre .... 774
//	sin la tercera condición ENTERA .............. 780
//
// O sea que «empieza con sabotaje» + «tiene dos puntos» hacen el 99,2 % del trabajo y la cláusula
// posicional decide SEIS anclas. Es la que impide contar una oración que se envolvió, así que no
// sobra —esos seis son ruido puro— pero la prosa la hace sonar más pesada de lo que es. Quien
// venga a ajustar el censo tocándola no va a mover el número.
// BAJÓ DE 774 A 712 EN EL MISMO DÍA QUE SE MIDIÓ, y el número nuevo es el registro de eso: se
// mecanizaron 62 anclas de una muestra SISTEMÁTICA (una de cada 13, para que el veredicto no
// saliera medido sobre las que yo hubiera elegido). La holgura hizo exactamente lo que tenía que
// hacer: con el techo en 774 esta guarda se puso roja pidiendo que se bajara.
// BAJÓ DE 642 A 623 EL 2026-09-16, Y LO ENCONTRÓ EL ARNÉS: ESTA GUARDA ESTABA EN VERDE SOBRE SU
// PROPIO SABOTAJE. `arnes -correr -paquete ./internal/mcp` dio 117 corridas, 116 en ROJO y UNA en
// VERDE. Dos sesiones llegaron al mismo 623 el mismo día, por caminos distintos (ver #544); el
// número coincide porque es la deuda medida, no una elección.
//
// LA PARTE QUE NO ES OBVIA, Y QUE CONVIENE LEER ANTES DE TOCAR `holguraDelTecho`: el sabotaje
// declarado arriba agrega EXACTAMENTE UN ancla. O sea que esta guarda sólo puede ponerse roja si
// `anclasEnProsaAlDia <= pendientes`, y por lo tanto QUEDA HUECA CON CUALQUIER HOLGURA >= 1.
//
// Las dos condiciones del predicado parecen cubrirse entre sí y no lo hacen: la primera —«la deuda
// subió»— es la que el sabotaje ejercita, y la segunda —«bajá el techo»— recién se despierta
// pasadas las 30. Entre una y otra hay una ventana de 30 anclas en la que el ratchet no mide nada
// y TAMPOCO SE PONE ROJA PARA DECIRLO. Así fue como se llegó acá: un lote mecanizó 22 anclas, la
// deuda cayó a 623, el techo se quedó en 642, y la diferencia de 19 pasó por debajo del umbral.
//
// El precio de ese hueco es que la única forma de que esta guarda mida es dejar el techo PEGADO a
// la deuda. Por eso se baja a 623 y no a 630: cualquier número mayor la devuelve al estado en que
// se la encontró. Y la regla general, que vale más que este número: cuando el sabotaje mueve el
// indicador en UNA unidad, cualquier tolerancia > 0 en el umbral vuelve la guarda hueca.
// BAJÓ DE 623 A 619 EL 2026-09-16, por la misma razón que la vez anterior y el mismo día: #549
// mecanizó 5 anclas y no ajustó este número, así que la diferencia volvió a ser > 0 y con eso la
// guarda vuelve a ser hueca (ver el bloque de arriba: su sabotaje suma UNA sola ancla).
//
// Y DE 619 A LO QUE MIDE HOY, resolviendo un conflicto entre dos ramas que bajaron el mismo
// número a la vez: 619 y 617 eran los dos correctos EN SU PROPIO ÁRBOL y ninguno lo es en el
// integrado. Un número derivado no se elige entre los dos lados de un conflicto: se vuelve a
// contar con la herramienta, sobre el árbol ya mergeado.
//
// Y DE 613 A 612 EL 2026-09-19. Es el caso que A123 tiene escrito como advertencia, visto desde
// adentro: #553 mecanizó cuatro anclas y bajó el techo a 613 sobre SU árbol, correcto ahí; este
// commit mecaniza una más y, ya rebasado sobre el main con #553 adentro, el censo da 612. Las dos
// ramas pasan verdes por separado y la segunda en mergear es la que tiene que volver a contar —no
// porque la primera se haya equivocado, sino porque el techo es un derivado del ÁRBOL ENTERO y
// ninguna de las dos pudo medir el árbol que resulta de las dos. Se vuelve a correr `-validar`
// sobre el árbol rebasado y se escribe lo que dice: 612.
//
// Y DE 612 A 610 EL 2026-09-19, y esta vez NO por mecanizar: dos anclas pasaron a
// `no_mecanizable`. Son las dos guardas del bit de ejecución, cuyo sabotaje es `git update-index
// --chmod=-x` o borrar un archivo — ninguna de las dos cosas es una sustitución de texto, que es
// lo único que este arnés sabe aplicar. La deuda baja igual, porque `anclasEnProsaAlDia` cuenta lo
// que NADIE puede correr, y una exención declarada con su motivo ya no es una promesa sin dueño.
// Cuidado con la comodidad que esto abre: declarar `no_mecanizable` baja el número igual que
// mecanizar, y es mucho más barato. El motivo se escribe para que se pueda discutir.
//
// Y DE 610 A 609 EL 2026-09-19: se mecanizó el sabotaje de la viñeta del mensaje de alerta, que
// SÍ es una sustitución de texto en `deploy/prometheus/alertmanager.yml`.
//
// Y DE 609 A LO QUE MIDE HOY, mismo commit que mecaniza: las ocho anclas del eje de
// consentimiento —`internal/fleet/consentimiento_test.go`—, que es lo que se le debe a la persona
// sentada frente a la máquina cuando alguien abre una sesión.
//
// ESTA RAMA TUVO QUE RECONTAR TRES VECES, y vale más que el número: escribió 605 sobre su propio
// árbol, 602 después de integrar el main que había bajado a 610, y esto después de integrar el que
// bajó a 609. No es que las mediciones anteriores estuvieran mal — cada una era correcta sobre el
// árbol que pudo ver. Es que el techo es un derivado del ÁRBOL ENTERO y nadie puede medir un árbol
// que todavía no existe. Mientras varias ramas mecanicen a la vez, la última en mergear recuenta,
// y eso no es un costo evitable: es lo que significa que el número sea derivado.
//
// Y DE 601 A LO QUE MIDE HOY, el mismo día y también en el commit que mecaniza: las ocho del eje de
// AUTORIZACIÓN de flota —`internal/mcp/fleet_authz_test.go`, el slice S3—, que es quién puede
// pedirle qué a qué máquina. Esta rama había escrito 597 sobre su propio árbol y después 594 sobre
// el de la rama de arriba: le pasó lo mismo que a ella, un nivel más abajo. Se recuenta sobre el
// integrado y se escribe eso.
//
// Y DE 593 A 581 EL 2026-09-19 — y esta vez el arnés lo cazó EN VIVO, que es la mejor evidencia
// que hay de que el párrafo de arriba no es teórico. Se mecanizaron las doce anclas del eje de
// CUATRO OJOS (`internal/mcp/fleet_cuatro_ojos_test.go`) y se corrieron los 67 sabotajes del
// paquete ANTES de tocar este número: 66 en ROJO y UNO en VERDE, y el verde era ESTA guarda.
// Con la deuda en 581 y el techo todavía en 593, su sabotaje —que suma UNA sola ancla— llegaba a
// 582 y no cruzaba nada. O sea que bajar el número no es contabilidad: es lo único que le devuelve
// el filo, y el instrumento lo demuestra sin que haya que creerle a nadie.
//
// Y DE 581 A 570 EL 2026-09-19: las once anclas del eje de EJECUCIÓN REMOTA
// (`internal/mcp/fleet_exec_test.go`), que es la superficie más cara del sistema — correr un
// comando en la máquina de otro. Bajado en el MISMO commit, por lo que dice el párrafo de arriba:
// la vez anterior el arnés cazó a esta guarda en verde por no hacerlo, y no hace falta repetirlo.
//
// Y DE 570 A 552 EL 2026-09-20: las dieciocho anclas del DOMINIO de flota —`device_test.go` y
// `servicio_test.go` de `internal/fleet`—, que es la matriz de tiers, la respuesta de `Permite`,
// la derivación de «en línea» y la forma de un reporte de servicio. Se eligió este paquete y no el
// archivo más gordo de `internal/mcp` por una razón de instrumento: acá el arnés corre en segundos
// y allá en minutos, así que las dieciocho se pudieron MEDIR una por una antes de escribirlas.
//
// Y DE 552 A LO QUE MIDE HOY, el mismo día: las catorce anclas del INVENTARIO DE SERVICIOS
// (`internal/mcp/servicios_test.go`) — qué corre en cada máquina, que es de donde salen las
// alertas. Dos de las catorce flotaban adentro del cuerpo de su prueba y llevan `prueba=`.
// Esta rama había escrito 556 sobre su propio árbol, medido antes de que el lote de dominio de
// flota bajara el techo a 552. Se recuenta sobre el integrado, como siempre.
//
// Y DE 538 A 519 EL 2026-09-20: las diecinueve anclas de `cmd/musubi` —el PANEL de flota y el
// inventario que el agente mide en la máquina—. Cinco de las diecinueve tenían la prosa MAL, el
// número más alto de la tanda, y una de ellas destapó un defecto en la propia guarda: le faltaba
// `olvidarEnumeracion()`, y sin eso la caché que le deja caliente la prueba de al lado se come su
// stub. Aislada daba ROJO y junto a su hermana VERDE — o sea que el arnés, que juzga con `-run`,
// la certificaba sana mientras en el CI no cubría nada.
//
// Y DE 519 A 495 EL 2026-09-20: las veinticuatro anclas de la PERSISTENCIA de flota
// (`internal/memory`: cola, comandos, devices, latido y servicios) — qué se guarda y qué se puede
// leer de vuelta. Las veinticuatro se midieron aisladas Y con el paquete entero, que es la
// disciplina que estrenó el lote anterior: una guarda puede dar ROJO con `-run` y VERDE en el CI.
//
// Y DE 495 A 470 EL 2026-09-20: las veinticinco anclas de TELEMETRÍA —`fleet_otlp_test.go` y
// `fleet_metrics_test.go`—, o sea qué mide cada máquina, a quién se le atribuye y cómo sale
// exportado. SEIS de las veinticinco tenían la prosa mal, el número más alto de la tanda, y una
// estaba RANCIA: nombraba un `Delete` que ya no existe en el árbol porque el rearme se mudó
// adentro de otra función. Una promesa en prosa no se entera cuando el código se mueve.
//
// Y DE 470 A 448 EL 2026-09-20: las veintidós anclas de POLÍTICAS Y BARRIDO —`fleet_politicas_test.go`
// y `fleet_s10_test.go`—, o sea lo que el sistema hace SOLO sobre una máquina: reiniciar un
// servicio, podar, sondear al que no tiene agente. Si una de ésas está hueca, el sistema actúa por
// su cuenta y nada lo atrapa.
//
// Y DE 448 A 423 EL 2026-09-20, el commit que cruza la MITAD del corpus: las veinticinco anclas de
// SHELL, SONDA y CONTEXTO —las dos superficies que quedan con acceso directo a una máquina, más lo
// que el cerebro le cuenta a quien pregunta por una—. Cuatro de las veinticinco tenían la prosa mal,
// y una de ellas de una forma nueva: DOS anclas del mismo bloque declaraban el MISMO corte, y sólo
// enciende en una de las dos, porque el fixture de la otra mide un valor que la guarda deja pasar.
//
// Y DE 423 A 395 EL 2026-09-20: las veintiocho anclas de RENDIMIENTO y CRONOLOGÍA —qué HIZO un
// servicio (no en qué estado está) y el relato de qué pasó en una máquina y cuándo—. Tres de las
// veintiocho tenían la prosa mal, y las tres por la misma causa de fondo: nombraban un corte que
// NO es el que decide.
//
// UNA: «llamar a ReportarServicios (el del latido) desde el handler» no borra nada. La poda es
// OTRA llamada, PodarServiciosAusentes, que el handler del latido hace aparte y después. «Usar el
// camino del latido» son dos operaciones y sólo la segunda es la que hace daño.
//
// DOS: «sacar Ventana.Normalizada del camino» es un no-op. Hay DOS normalizaciones en el camino y
// la que queda vuelve a redondear; el corte que decide está adentro de la función, no en ninguno
// de sus llamadores.
//
// TRES: «no escribir el origen en politicas.go» deja la prueba VERDE, porque siembra el comando A
// MANO con el origen ya puesto: prueba que el campo viaja, no que alguien lo setea. Ese cableado
// lo custodia un hermano, en fleet_politicas_test.go, y allá sí está mecanizado.
//
// La familia es una sola y vale escribirla: CONTÁ LAS COMPUERTAS ANTES DE NOMBRAR EL CORTE. Un
// camino con dos guardas en serie, o con la escritura y la lectura separadas, no se corta por
// cualquiera de sus mitades — y la prosa, que se escribe mirando el código una vez, elige casi
// siempre la mitad que se ve primero.
//
// Y DE 395 A 379 EL 2026-09-20: las dieciséis anclas del AGENTE —`cmd/musubi/agent_test.go` y
// `agent_token_test.go`—, o sea la mitad que corre EN la máquina: cómo late, cómo aguanta un
// cerebro caído y cómo sostiene su credencial mientras rota. Una estaba hueca y dos eran una
// guarda contada dos veces.
//
// LA HUECA ERA RANCIA ADEMÁS, y es la primera de la campaña que estaba DENUNCIADA en su propio
// archivo sin que nadie corrigiera el ancla. `TestElAgenteGuardaElTokenNuevoYLoEstrena` prometía
// «borrar el campo TokenNuevo del struct de la respuesta en latir()»; se aplicó el corte y quedó
// VERDE, porque esa prueba no toca latir() en ningún momento. Ciento setenta líneas más abajo, el
// bloque de `TestElLatidoTraeElTokenDeUnaRotacionEnCurso` ya lo decía con todas las letras —«se
// verificó saboteándolo: quitar el campo del struct las deja TODAS en verde»— y el ancla de arriba
// siguió prometiendo lo contrario. Y hoy ni siquiera hay struct local en latir(): la respuesta se
// decodifica con `fleet.RespuestaLatido`, el tipo compartido.
//
// LO QUE ESO ENSEÑA: una medición escrita en el archivo NO corrige el ancla que la contradice. Las
// dos conviven, y la que se lee primero es la de arriba. Mecanizar es lo que las obliga a
// encontrarse.
//
// LAS DOS CONTADAS DOS VECES las destapó el censo solo, como COLISIÓN de directivas: el techo del
// backoff ya lo custodiaba `TestElBackoffTieneTecho` con exactamente el mismo corte, y el sello del
// inventario tenía dos hermanas sobre las mismas líneas de `servicios.go`. En los dos casos la
// respuesta NO fue declarar `colision_ok` sino elegir el corte que esta prueba y sólo ésta mira:
// el jitter en un caso, el punto del sello en el otro. `colision_ok` es para cuando las dos
// guardas cubren la línea de verdad; acá una de las dos estaba prometiendo el trabajo de la otra.
//
// Y DE 379 A 364 EL 2026-09-21: las quince anclas del DOMINIO de la cronología y de la EXPOSICIÓN
// —`internal/fleet/cronologia_test.go` y `exposicion_test.go`—: qué se puede ver de la línea de
// tiempo y qué se puede creer de un endpoint `/metrics` ajeno. Ninguna resultó hueca, y el barrido
// se corrió sobre el PAQUETE ENTERO: 83 de 83 en rojo.
//
// LO QUE DEJÓ ESTE LOTE NO ES UN HUECO SINO UNA REGLA SOBRE `colision_ok`. El censo denunció tres
// colisiones y sólo UNA merecía respuesta:
//
// SÍ: el redondeo de `Ventana.Normalizada` lo custodian DOS pruebas de verdad, una por paquete
// —`TestLaVentanaSeNormalizaHaciaAfuera` mira la aritmética y `TestLoQueAcabaDePasarEntraEnLaVentana`
// mira la consulta contra la base—. Las dos cubren la línea y sus motivos son distintos, así que
// se declaró `colision_ok` de los DOS lados: la respuesta de una sola deja la otra sin contestar.
//
// NO, LAS OTRAS DOS: el default de `TipoDeArgv` y la compuerta del parser ya tenían su guarda, y
// lo que estaba por poner era el mismo corte dos veces. Ahí la respuesta no es declarar la
// colisión sino MOVER EL CORTE al punto que esta prueba —y sólo ésta— mira: un `case` explícito en
// vez del default, y el `return` final en vez de la condición de la compuerta.
//
// El criterio, que vale para todo lo que viene: `colision_ok` se declara cuando las dos guardas
// cubren la línea de verdad. Si una de las dos está prometiendo el trabajo de la otra, declararla
// sería apagar el aviso en vez de contestarlo.
//
// Y DE 364 A 340 EL 2026-09-21: las veinticuatro anclas de LA CAPA DE MEDICIÓN —cómo se alcanza una
// máquina ajena por SSH (`remoto_test.go`), qué se absorbe de su colector (`colector_externo_test.go`)
// y cómo se mide una máquina de verdad (`procparse_test.go`, `colector_linux_test.go`,
// `colector_test.go`)—. El barrido corrió sobre el paquete entero: 104 de 104 en rojo.
//
// LA ÚLTIMA PROSA RANCIA DEL CORPUS, y se buscó a propósito. `colector_linux_test.go` prometía
// «usar MemFree en leerMemoria» y `leerMemoria` YA NO EXISTE: el parseo se disolvió adentro de
// ParsearMeminfo. Se barrieron las 364 anclas en prosa buscando símbolos nombrados ahí y ausentes
// del árbol, y ésta era la única.
//
// EL BARRIDO SE ENCONTRABA A SÍ MISMO, y esa es la lección del instrumento. La primera corrida dio
// CERO rancias: buscaba cada símbolo con `git grep -- '*.go'`, que incluye el archivo donde vive la
// prosa, así que todo símbolo se hallaba a sí mismo en su propia promesa. Hay que excluir el
// archivo del ancla Y descartar los hits que caen en comentarios. Es la familia «la guarda se
// detecta a sí misma», esta vez del lado de la herramienta que audita.
//
// TRES PROMESAS NO SE MECANIZARON PORQUE EL CORTE YA TENÍA DUEÑO. La asignación de MemFree en
// ParsearMeminfo ya la custodian DOS guardas con su colisión contestada —una borra la asignación,
// la otra inventa un 0— y encima había otras tres anclas prometiendo lo mismo desde otros archivos.
// Se fusionaron con una nota que dice dónde vive la red. Una tercera promesa sobre la misma línea
// no agrega red: agrega la ilusión de que hay más.
//
// Y UN `colision_ok` QUE SOBRABA: dos cortes míos declaraban pisarse y el censo contestó que NO se
// pisan, porque el `a` de los dos CONSERVA su propio `de` (insertan un return y renombran la
// función vieja). Un `a` que preserva el `de` no rompe a nadie. La herramienta caza la respuesta
// rancia igual que caza la promesa rancia.
//
// Y DE 340 A 326 EL 2026-09-21: las catorce anclas del DESPLIEGUE —el relay de pantalla
// (`despliegue_relay_test.go`) y el actualizador del agente de Windows
// (`despliegue_actualizador_agente_test.go`)—. Es el primer lote donde casi nada de lo saboteado es
// Go: se cortan `compose.yml`, `preparar.sh`, `lib-agente-windows.sh`, `cambiar-agente.cmd`,
// `matar-zombis-agente.sh` y un colector en Python. Una resultó hueca.
//
// LA HUECA ES DE UNA FAMILIA QUE YA ESTÁ ESCRITA Y VOLVIÓ CON OTRA CARA: una guarda de TEXTO cubre
// una FORMA, no el defecto. `TestElActualizadorNoEligeLaInstalacionPorElPrimerProcesoQueAparezca`
// exige las DOS cadenas —`Get-Process musubi` y `Select-Object -First 1`— en la MISMA línea. El
// primer corte que se probó fue `$conToken = @($cands | Select-Object -First 1)`: elige la
// instalación por el primer proceso que aparezca, que es EXACTAMENTE el cabo A98, y la prueba quedó
// VERDE porque la grafía no coincide.
//
// NO SE ENSANCHÓ LA CONDICIÓN, Y ESO TAMBIÉN ES LA LECCIÓN. `Select-Object -First 1` a secas es
// legítimo en cualquier otra tubería del guion, y una guarda que lo prohíba entero se vuelve un
// estorbo que alguien termina apagando. Lo que corresponde es que quede ESCRITO qué custodia: la
// tubería histórica, no la idea. El discriminante de verdad —elegir por `device.token`— lo custodia
// la aserción de al lado, y la mitad «ante la duda, parar» la custodia la prueba siguiente.
//
// Y UNA COLISIÓN LEGÍTIMA MÁS, la tercera de la campaña: la regla del desglose
// (`total > Atendidas` en rendimiento.go) la custodian el DOMINIO —TestElDesgloseSumaMenosOIgualPeroNuncaMas—
// y el CONTRATO DEL RELAY, que manda un reporte donde el desglose IGUALA a las sondas. Las dos
// cubren la línea de verdad y sus motivos difieren, así que `colision_ok` de los dos lados.
//
// Y DE 326 A 308 EL 2026-09-21: las dieciocho anclas de LA PUERTA DE LA FLOTA y la PANTALLA
// —`fleet_test.go` y `fleet_pantalla_test.go`—: quién entra por dónde, de qué tenant se lee, qué
// pesa más que una capacidad concedida, y el plano de pantalla entero. Dieciséis con directiva, DOS
// declaradas `no_mecanizable`, ninguna hueca: 23 de 23 en rojo.
//
// LAS DOS EXENTAS LO SON POR RAZONES ESTRUCTURALES, Y LAS DOS SON MEJORES QUE LA GUARDA.
//
// La del LOCKOUT ya traía su razonamiento escrito con mediciones desde antes —el bloque del limiter
// está repetido byte por byte en las tres puertas, así que ningún corte cerca es único— y lo único
// que le faltaba era la CLAVE. Estuvo contada como deuda sin serlo.
//
// La de la CONTRASEÑA DE PANTALLA se midió acá: la sesión se PERSISTE antes de que la contraseña
// exista. `AbrirSesionPantalla` la escribe en el llamador y `entregarPantalla` —la única función que
// acuña la clave— RECIBE la fila ya creada, así que no hay un solo punto del programa donde la
// contraseña y la fila coexistan. Guardarla exigiría cuatro sitios: migración, struct, INSERT y
// lectura. Eso no es una guarda que falta: es un invariante que el diseño hace irrepresentable.
//
// DOS PROSAS APUNTABAN AL SITIO EQUIVOCADO, y las dos por la misma razón: nombraban el bloque que
// se ve primero. «Sacar el `switch consent` de toolFleetScreen» corta el switch que elige cómo
// AVISAR de una sesión que YA se va a abrir; el que RECHAZA es un `if consent.Bloquea()` treinta
// líneas más arriba. Y las de /mcp y del latido prometían «caer a DevicePorToken» y «resolver contra
// opt.registry»: ninguna de las dos puertas tiene el otro resolutor a mano —`autenticarPersona`
// recibe `httpOptions`, que sólo trae el registro de personas— así que el corte literal exigiría
// cambiar una firma. Se mecanizó la propiedad que la prueba afirma de verdad: que una credencial
// que el resolutor PROPIO no reconoce no entra igual.
//
// Y LA COLISIÓN DIRECCIONAL, por tercera vez en la campaña: se declararon dos `colision_ok` que el
// censo devolvió por RANCIOS. Un `a` que CONSERVA su propio `de` no rompe a nadie, así que la
// colisión existe en un solo sentido y contestarla de los dos sobra. Vale como regla: antes de
// declarar la pareja, mirar si el `a` preserva el literal.
//
// Y DE 308 A 294 EL 2026-09-21: las catorce anclas de AVISOS y MANTENIMIENTO —lo que el sistema le
// dice a quien usa la máquina antes de entrarle, y lo que decide NO hacer mientras está en ventana—.
// Ninguna hueca al cierre: 13 de 13 en rojo. Pero el camino hasta ahí dejó cuatro cosas escritas.
//
// UNA DIRECTIVA EN EL ANCLA EQUIVOCADA, Y ESCONDIDA DETRÁS DE UNA EXENCIÓN. En aviso_test.go el
// bloque de dos sabotajes tenía la segunda línea declarada `no_mecanizable` —con un motivo largo y
// medido— y el `archivo`/`de`/`a` del PRIMERO caído DEBAJO, o sea dentro del alcance de la exenta.
// El alcance de un ancla termina donde empieza la siguiente: la primera figuraba como deuda sin
// serlo y la segunda cargaba una directiva que no le tocaba. Y al repararlo apareció una COLISIÓN
// real que la exención venía tapando — el censo no mira las directivas de un ancla exenta.
//
// UN SABOTAJE QUE NO COMPILA NO PRUEBA NADA, y el arnés lo dice aparte de los verdes: «sin
// veredicto». La prosa pedía cambiar `PuedePreguntar` de `*bool` a `bool`; el árbol entero lo trata
// como puntero y el paquete no arma. Se corta la CONSECUENCIA —que un campo ausente se vuelva un
// `false` explícito— que es exactamente lo que el puntero existe para impedir.
//
// Y DOS PARES DE «MOTIVOS REPETIDOS», los dos legítimos y los dos anotados en vez de forzados. Una
// prueba que observa UNA cosa no puede separar dos caminos que llegan a esa cosa: «acuñar la clave
// antes de preguntar» y «seguir el camino normal en un `pide`» caen en la misma línea con el mismo
// texto, y lo mismo pasa con «guardar la respuesta en la columna equivocada» y «devolver el mismo
// mensaje para los tres». En los dos casos queda mecanizado el corte más estructural y ESCRITA la
// medición que dice por qué el otro no agrega red.
//
// Y DE 294 A 277 EL 2026-09-21: las diecisiete anclas del EXPORTADOR DE PROMETHEUS, repartidas en
// cinco archivos —el scrape, los servicios, la versión del agente, el inventario y el truncado—.
// Es lo que leen el panel y TODAS las alertas, así que una guarda hueca acá no se ve: se ve como
// una alerta que nunca dispara. 18 de 18 en rojo al cierre.
//
// SEIS COLISIONES EN UN SOLO LOTE, Y ESO ES UNA PROPIEDAD DEL SITIO. El scrape y el empuje OTLP son
// DOS superficies sobre UNA tabla de series, así que sus guardas se pisan por construcción: es la
// lección A39 del propio repo, vista desde el arnés. Cinco eran parejas legítimas y se declararon;
// una era mía, por un `de` ambiguo, y se arregló alargando el literal. El detalle que vale: cuando
// el censo dice «el `de` aparece 2 veces», la ambigüedad también CONFUNDE al detector de
// colisiones, que denuncia pares que no existen. Se arregla la unicidad primero y después se mira
// qué colisiones quedan de verdad.
//
// LA COLISIÓN ES DIRECCIONAL — cuarta vez, y ya no hace falta volver a medirlo: un `a` que CONSERVA
// su `de` no pisa a nadie. En este lote dos declaraciones volvieron por rancias justo por eso.
//
// Y OTRO SABOTAJE QUE NO COMPILA, con una cara nueva: no por tipos, sino por CÓDIGO INALCANZABLE.
// Anteponer un `return` al cuerpo de un `case` deja muerto lo que sigue y `go vet` lo rechaza, así
// que el corte tiene que REEMPLAZAR el `return` final del caso en vez de adelantarse a él.
//
// Y DE 277 A 264 EL 2026-09-21: las trece anclas del MOTOR DE DISEÑO —`ejes_diseno_test.go` y
// `formas_diseno_test.go`—: cómo se etiqueta una tarjeta por su VOCABULARIO y no por su nombre, y
// cómo el motor ACOTA las formas plausibles sin elegir por vos. 15 de 15 en rojo (las trece más dos
// de control). Dos intentos míos fallaron antes, y los dos enseñan algo.
//
// EL EJE QUE LA PRUEBA MIRA NO ES EL QUE UNO SUPONE. `TestFormasUnaPropiedadNoTieneForma` recorre
// una lista tipeada —color, a11y, tipografia, terminacion, estado-vacio— y el primer corte le dio
// candidatas a «paleta», que no está en esa lista: VERDE. Cuando el sabotaje es AGREGAR algo a una
// tabla, hay que agregarlo exactamente donde la prueba va a mirar, y eso se lee de la prueba, no
// del dominio.
//
// Y LA TERCERA CARA DE «EL SABOTAJE NO COMPILA»: vaciar un campo de un struct puede dejar SIN USAR
// la variable que lo llenaba. `Shape: forma` → `Shape: ""` deja a `forma` declarada y no usada, y
// Go no compila. `Shape: forma[:0]` da lo mismo —cadena vacía— y conserva el uso.
//
// LAS CINCO ANCLAS DE `specs_sin_cabos_test.go` QUEDAN PARA UN LOTE PROPIO: sabotean TABLAS DE
// MARKDOWN del registro de cabos, con filas de miles de caracteres y `|`, `**` y backticks adentro.
// Es un objetivo nuevo para el arnés y merece su propio cuidado con los escapes, no el final de una
// tanda larga.
//
// Y DE 264 A 249 EL 2026-09-21: las quince anclas de las POLÍTICAS y los AVISOS del dominio
// —`internal/fleet/politica_test.go`, `politica_servicio_test.go` y `aviso_test.go`—: las reglas
// que el sistema aplica SOLO sobre una máquina, y lo que le dice a quien la está usando. Catorce
// con directiva, UNA exenta, 16 de 16 en rojo al cierre.
//
// UNA GUARDA HUECA QUE NO ERA UNA PROMESA FALSA SINO UN FIXTURE FLOJO, y es una cara nueva.
// `TestDosServiciosDeLaMismaMaquinaNoCompartenEnfriamiento` compara las claves de cooldown de dos
// políticas sobre `nginx` y sobre `postgres` — pero les da NOMBRES DISTINTOS, y el nombre ya entra
// en la clave. Sacarle el servicio a `ClaveDeCooldown` la dejaba VERDE: lo que separaba las claves
// era el nombre, no el servicio. Se agregó la mitad que faltaba —la MISMA política sobre dos
// servicios— que es exactamente el caso que el comentario de `ClaveDeCooldown` describe. Acá no
// alcanzaba con anotar el hallazgo: la guarda no cubría su invariante y ahora sí.
//
// LA CUARTA CARA DE «EL SABOTAJE NO COMPILA»: `duplicate case`. Hacer que dos constantes de un enum
// sean iguales rompe el `switch` que las lista JUNTAS en un mismo `case`. El corte que sí compila
// convierte una de las dos en VARIABLE — un `case` no constante no dispara la comprobación de
// duplicados— y de paso el sabotaje sigue diciendo lo que prometía.
//
// Y LA EXENTA ES LA MISMA FAMILIA QUE LAS DOS ANTERIORES: «sacarle el campo Motivo a
// CapacidadDeAvisar» lo usa LA PROPIA PRUEBA, que construye el struct con ese campo y después lo
// lee. Quitarlo no es una sustitución sino una refactorización, y no hay otro corte que la ponga en
// rojo porque el valor se fija en el literal del test. Lo que custodia es la FORMA del tipo, y esa
// clase de invariante la sostiene el compilador.
//
// Y DE 249 A 235 EL 2026-09-21: las catorce anclas de la SEGURIDAD DEL BINARIO Y DEL CANAL —el
// blindaje de la unidad de systemd (`cmd/musubi/blindaje_test.go`), la firma de las actualizaciones
// (`internal/selfupdate/firma_test.go`) y la rotación de credenciales (`internal/mcp/rotacion_test.go`)—.
// Doce con directiva, DOS exentas, 18 de 18 en rojo al cierre.
//
// UNA GUARDA QUE MIRA LA SUPERFICIE DE LECTURA Y NO LA BASE. `TestElTokenDeLaRotacionNoQuedaEnClaroEnLaBase`
// promete que en reposo hay hashes y no credenciales. Se guardó el token EN CLARO en la columna y la
// prueba quedó VERDE: lo único que inspecciona es el JSON de `ListarDevices`, y `fleet.Device` no
// tiene ningún campo que lleve ese token. Lo que custodia de verdad es que la REPRESENTACIÓN no lo
// exponga —cierto y valioso— y no el invariante que su nombre anuncia. Cubrirlo exigiría leer la
// columna directamente, y este paquete no tiene por dónde. Queda dicho en vez de fingido.
//
// UNA EXENCIÓN QUE SU PROPIA PROSA YA ANUNCIABA. El ancla de la re-serialización del manifiesto dice
// con todas las letras «TestUnManifiestoConFirmaAjenaNoVerifica no lo caza»; se mecanizó igual para
// comprobarlo y el barrido devolvió VERDE, como estaba escrito. Los fixtures firman y verifican el
// MISMO arreglo de bytes, así que reordenar claves no les cambia nada; lo que rompe es un release
// REAL. La red contra eso es el aviso en el doc de VerificarFirma, no una guarda.
//
// Y UN SABOTAJE QUE PANIQUEABA EN VEZ DE FALLAR. Neutralizar la guarda de largo de clave dejaba que
// `ed25519.Verify` entrara con una clave corta y PANIQUEARA; el arnés lo marcó «rojo sospechoso»
// porque no podía aislar un motivo propio. El corte correcto es el que la prosa pedía literalmente
// —devolver nil— y da un rojo limpio. Un pánico no es una guarda contestando: es el proceso
// muriéndose antes de que la guarda hable.
//
// Y DE 235 A 230 EL 2026-09-21: las cinco anclas del REGISTRO DE CABOS
// (`internal/mcp/specs_sin_cabos_test.go`), y con ellas el arnés estrena un objetivo nuevo: sabotea
// TABLAS DE MARKDOWN de `specs/control-de-flota/ABIERTO.md`. Ya cortaba Go, fixtures JSON, compose,
// shell, PowerShell, `.cmd` y Python; ahora también documentación. 7 de 7 en rojo al cierre.
//
// LAS TRES QUE FALLARON PRIMERO ERAN TODAS EL MISMO ERROR MÍO, Y ES UNO QUE VALE ANOTAR: APUNTARLE
// A LA PRUEBA EQUIVOCADA. Estas anclas FLOTAN —viven en bloques de doc entre funciones, no pegadas
// a un `func Test…`— así que el lector devuelve la prueba vacía y hay que declarar `prueba=` a mano.
// Declararla mirando el bloque de arriba NO alcanza: el bloque explica un invariante y la prueba que
// lo custodia puede estar cien líneas más abajo y llamarse distinto. Una quedó apuntando a un nombre
// que ni siquiera existe, y ahí el arnés no dice VERDE sino algo mejor: «EL PATRÓN NO SELECCIONA
// NINGUNA PRUEBA… un verde de cero pruebas es indistinguible de un verde de mil».
//
// LA RECETA, para las que queden: buscar el `func Test…` cuyo CUERPO hace la aserción que el
// sabotaje rompería, no el que está más cerca del comentario.
//
// Y DE 230 A 218 EL 2026-09-21: las doce anclas del CANAL DE SHELL DEL AGENTE y de las ALERTAS DEL
// DESPLIEGUE. 13 de 13 en rojo al cierre, pero tres fallaron primero y la tercera destapó un tope
// del arnés que no estaba escrito.
//
// UN `de` MULTILÍNEA NO MATCHEA EN UN ARCHIVO CON CRLF. `deploy/agente-windows.ps1` tiene
// terminadores CRLF —es un guion de Windows— y el `\n` de una directiva produce LF solo, así que un
// literal de dos líneas NO SE ENCUENTRA NUNCA. El censo lo denuncia con «el `de` de este sabotaje YA
// NO ESTÁ», que es cierto y manda a buscar el texto nuevo cuando el texto está intacto y lo que
// falla es el salto de línea. LA SALIDA: en un archivo CRLF, el `de` va en UNA SOLA LÍNEA. Si hace
// falta abrir un bloque, se abre en la misma línea (`if ($true) { $x = …`).
//
// LAS OTRAS DOS FUERON MÍAS Y SON LA MISMA FAMILIA QUE YA ESTÁ ESCRITA: apuntar al archivo o al
// texto equivocado. Una cortaba `prometheus.yml` cuando la prueba lee el INSTALADOR y el COMPOSE;
// la otra renombraba el job a `alertmanager-apagado`, que SIGUE CONTENIENDO «alertmanager» — y la
// prueba busca por subcadena. Un renombre no es un borrado si el nombre viejo sobrevive adentro del
// nuevo.
//
// Y DE 218 A 200 EL 2026-09-21: las dieciocho anclas del LATIDO Y EL INVENTARIO DE LA FLOTA
// —`latido_una_tx_test.go`, `flota_test.go`, `fleet_vidared_test.go` y `fleet_renombrar_test.go`—.
// 18 de 18 en rojo, y el lote entró SIN una sola hueca: es el primero en el que el diseño acertó de
// entrada, y no por suerte.
//
// LO QUE CAMBIÓ ES EL MÉTODO: VERIFICAR LOS LITERALES ANTES DE APLICAR NADA. El guion de aplicación
// ahora desescapa cada `de` —DOS veces, una por su propia fuente y otra por el formato de la
// directiva— y cuenta sus apariciones en el archivo destino; si alguno no es único, aborta sin
// tocar el árbol. Los lotes anteriores gastaban tres o cuatro vueltas de aplicar → validar →
// corregir en unicidad; éste, ninguna.
//
// TRES PAREJAS DE `colision_ok` EN UN SOLO LOTE, y las tres son la misma forma: una LÍNEA que dos
// pruebas miran desde ángulos opuestos. El autorreporte del latido lo cortan el techo de
// transacciones y el inventario; `proyectosParaLeer` lo cortan «no pude leer la lista» y «la lista
// vino truncada»; y el autorreporte además choca con la guarda de que sólo toca la fila del token.
// Cuando una línea concentra varios invariantes, las colisiones no son un problema del arnés: son
// el mapa de cuántas cosas dependen de esa línea.
//
// Y UN SABOTAJE QUE NO COMPILÓ, quinta cara: `if false {` sobre una condición deja SIN USAR las
// variables que la condición leía. El corte que compila las conserva —`if !hay && v == X && false`—
// y dice lo mismo.
const anclasEnProsaAlDia = 200

// holguraDelTecho es cuánto se deja bajar antes de exigir que el techo se ajuste.
//
// LAS DOS DIRECCIONES SON NECESARIAS Y LA SEGUNDA NO ES OBVIA. Un techo que sólo prohíbe subir se
// PODRE: se mecanizan 300 anclas, el techo sigue en 774, y la guarda deja de medir sin ponerse
// roja ni una vez. Con la holgura, un lote grande obliga a bajar el número — que es el único
// registro de que la deuda bajó.
// Y HAY UN MATIZ MEDIDO EL 2026-09-16, que el párrafo de arriba no cubría: la holgura tolera
// hasta 30, pero el SABOTAJE de esta guarda suma UNA sola línea de deuda. O sea que apenas alguien
// mecaniza un ancla, ese +1 ya no cruza el techo y esta guarda queda HUECA — verde sobre su propio
// sabotaje— sin que el CI proteste, porque la holgura todavía está dentro de lo tolerado. Se
// descubrió corriendo el arnés después de mecanizar 19 anclas en una tanda: techo 642, deuda 623.
//
// Por eso bajar el número en el MISMO commit que mecaniza no es cosmética contable: es lo único
// que le devuelve el filo al sabotaje. La holgura sigue en 30 a propósito —bajarla a 0 haría que
// dos lotes en paralelo se pisen el número— y el precio de esa elección es este matiz, que ahora
// está escrito.
const holguraDelTecho = 30

func TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre(t *testing.T) {
	raiz := filepath.Join("..", "..")
	c, err := arnes.Censar(raiz)
	if err != nil {
		t.Fatalf("no pude censar el árbol: %v — no medí nada", err)
	}

	// EL CONTROL VA PRIMERO: un cero acá no es «el árbol no promete sabotajes», es «este censo no
	// miró nada», y todo lo que sigue daría verde.
	if len(c.Anclas) == 0 || c.Archivos < 400 {
		t.Fatalf("el censo miró %d archivos y encontró %d anclas: eso no es un árbol sin deuda, es "+
			"un enumerador que no está mirando el repo", c.Archivos, len(c.Anclas))
	}

	if len(c.SinUbicar) > 0 {
		t.Errorf("el lector vio %d ancla/s en el texto crudo y no pudo colocarlas en el AST. Un agujero "+
			"en el enumerador se ve idéntico a un árbol sano:\n  %s",
			len(c.SinUbicar), strings.Join(c.SinUbicar, "\n  "))
	}

	if rotas := c.Rotas(); len(rotas) > 0 {
		var lineas []string
		for _, a := range rotas {
			lineas = append(lineas, a.Archivo+":"+strconv.Itoa(a.Linea)+": "+strings.Join(a.Quejas, "; "))
		}
		t.Errorf("%d directiva/s `arnes:` no se pueden leer. NO cuentan como mecanizadas a propósito: "+
			"una directiva ilegible que se cuenta como cubierta hace subir la cobertura con sabotajes "+
			"que nadie puede correr.\n  %s", len(rotas), strings.Join(lineas, "\n  "))
	}

	// LOS SABOTAJES QUE SE PISAN SE INFORMAN Y NO FALLAN, y la distinción la enseñó la medición.
	//
	// Dos directivas sobre la misma línea de producción pueden ser dos guardas cubriéndola desde
	// ángulos distintos: el par de `internal/fleet` cae con dos motivos DISTINTOS, así que las dos
	// miden y sería un error prohibirlo. Pero también pueden ser un rojo falso, como lo fue la
	// directiva de esta misma ancla durante siete corridas. Lo único cierto en los dos casos es que
	// a lo sumo UNA de las dos es sobre el COMPORTAMIENTO de esa línea; la otra, si cae, cae por el
	// daño al corpus. Una guarda que grita en el caso legítimo enseña a ignorarla, así que esto
	// queda como aviso con nombre y línea, para ir a mirar.
	if choques := arnes.Colisiones(c); len(choques) > 0 {
		t.Logf("%d sabotaje/s se pisan entre sí (a lo sumo uno de cada par mide comportamiento):\n  %s",
			len(choques), strings.Join(choques, "\n  "))
	}

	// ESTA ES LA QUE IMPIDE QUE EL CORPUS SE PODRA, Y ES LA RAZÓN DE QUE ESTA GUARDA VIVA EN CI.
	// Los 774 sabotajes no se pueden correr acá; que sus anclas sigan existiendo, sí.
	if males := arnes.Validar(c); len(males) > 0 {
		t.Errorf("%d directiva/s dejaron de apuntar a donde dicen. Un `de` que ya no está significa que "+
			"el sabotaje NO SE APLICA, y su verde se lee igual que «la guarda cubre»:\n  %s",
			len(males), strings.Join(males, "\n  "))
	}

	pendientes := len(c.Pendientes())
	switch {
	case pendientes > anclasEnProsaAlDia:
		t.Errorf("LA DEUDA SUBIÓ: %d anclas en prosa contra un techo de %d.\n"+
			"Agregaste %d promesa/s de sabotaje que nadie puede correr. La salida NO es subir "+
			"`anclasEnProsaAlDia`: es escribirle la directiva `arnes:` a tu ancla nueva\n"+
			"    // arnes: archivo=\"ruta/al/archivo.go\"\n"+
			"    // arnes: de=\"el literal exacto a reemplazar\"\n"+
			"    // arnes: a=\"con qué se reemplaza (vacío borra)\"\n"+
			"o declararla `no_mecanizable=\"<motivo>\"` si el sabotaje literal no puede compilar "+
			"(el caso típico: la guarda es el único lector de un import o de una variable, así que "+
			"borrarla deja `imported and not used` y el rojo sería por build roto).\n"+
			"\nY SI ESTÁS EN CI Y NO AGREGASTE NINGUNA PROMESA, ESTO NO ES TUYO. Este techo es un "+
			"derivado del ÁRBOL ENTERO clavado en un archivo, así que dos ramas que agregan una "+
			"promesa cada una pasan verdes por separado y la SEGUNDA en mergear rompe sin haber "+
			"cambiado nada. Pasó la primera vez que esta guarda entró: dos PRs en 9/9 con 48 "+
			"segundos entre un merge y el otro. Bajá el log entero, mirá QUÉ archivo trae las "+
			"anclas nuevas —`go run ./deploy/cmd/arnes -detalle`— y si no es tuyo, avisale a quien "+
			"lo trajo en vez de tocar la constante.\n"+
			"Correlo con: go run ./deploy/cmd/arnes -detalle",
			pendientes, anclasEnProsaAlDia, pendientes-anclasEnProsaAlDia)
	case anclasEnProsaAlDia-pendientes > holguraDelTecho:
		t.Errorf("EL TECHO SE AFLOJÓ: hay %d anclas en prosa y el techo dice %d (%d de más).\n"+
			"Bajá `anclasEnProsaAlDia` a %d. Un techo que sólo prohíbe subir se podre: deja de medir "+
			"sin ponerse rojo ni una vez, y el número que baja es el único registro de que la deuda bajó.",
			pendientes, anclasEnProsaAlDia, anclasEnProsaAlDia-pendientes, pendientes)
	}

	// EL LOG PUBLICA LAS DOS FRACCIONES, Y NO UNA. «Cobertura 1,4 %» sobre las anclas dice cuánto
	// de lo DECLARADO es ejecutable; «774 de 3.219» dice cuánto del árbol declara algo. Publicar
	// sólo la primera hace que 774 se lea como el universo, y es el 24 %.
	t.Logf("%d archivos · %d anclas en %d pruebas de %d (%.0f %% del árbol declara) · %d mecanizadas · %d exentas · %d en prosa (techo %d) · ejecutable %.1f %% de lo declarado",
		c.Archivos, len(c.Anclas), c.PruebasConAncla, c.FuncionesTest,
		100*float64(c.PruebasConAncla)/float64(c.FuncionesTest),
		len(c.Mecanizadas()), len(c.Exentas()), pendientes, anclasEnProsaAlDia,
		100*float64(len(c.Mecanizadas()))/float64(len(c.Anclas)))
}
