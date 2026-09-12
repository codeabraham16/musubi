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
const anclasEnProsaAlDia = 712

// holguraDelTecho es cuánto se deja bajar antes de exigir que el techo se ajuste.
//
// LAS DOS DIRECCIONES SON NECESARIAS Y LA SEGUNDA NO ES OBVIA. Un techo que sólo prohíbe subir se
// PODRE: se mecanizan 300 anclas, el techo sigue en 774, y la guarda deja de medir sin ponerse
// roja ni una vez. Con la holgura, un lote grande obliga a bajar el número — que es el único
// registro de que la deuda bajó.
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
