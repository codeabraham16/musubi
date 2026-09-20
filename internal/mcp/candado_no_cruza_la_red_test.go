package mcp

import (
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/guiones"
)

// TODA TOOL QUE PUEDA LLEGAR A UNA LLAMADA DE RED TIENE QUE DECLARAR `lockSelf`, Y AL REVÉS.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA REGLA YA ESTABA ESCRITA. LA GUARDA DEFENDÍA SU VIOLACIÓN.
//
// `server.go` la dice sin ambigüedad: el handler que hace I/O externa (motor LLM, embedder) se
// declara `lockSelf` y acota su propia sección crítica. Sostener el candado del despacho durante
// una llamada de red serializa el servidor entero.
//
// La guarda que existía —`TestG1ClasePorDefaultEsLaDeHoy`— hacía lo contrario de custodiarla:
// ENUMERABA cuatro nombres y fallaba si CUALQUIER OTRA tool declaraba `lockSelf`.
//
//	declaradas := map[string]bool{"musubi_recall", "musubi_ask", "musubi_distill", "musubi_sharpen"}
//	...
//	if hayClase { t.Errorf("%s: no debería declarar clase de candado", e.Name) }
//
// O sea que arreglar el bug PONÍA LA SUITE EN ROJO. Una guarda que convierte el arreglo en una
// regresión aparente es peor que no tener guarda: le enseña al próximo a aflojarla. Y su hermana
// lo confesaba por escrito — la sonda de G5 fue elegida para NO pasar por el embebedor, «porque
// guardar una observación también pasa por el embedder y la sonda quedaría atrapada en el mismo
// cuelgue que se está midiendo». La prueba esquivaba a propósito el camino defectuoso.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ SE DERIVA DEL AST Y NO DE UNA LISTA
//
// Una lista de nombres es una copia de la regla, y por lo tanto la próxima en envejecer: es el
// defecto que esta guarda viene a cerrar, cometido de nuevo. Acá el conjunto se DEDUCE:
//
//  1. se parsea `internal/mcp` con modo 0 —sin comentarios, así ningún comentario puede satisfacer
//     ni disparar esta guarda—, con el conjunto de archivos que `go list` dice que compila;
//  2. se marcan las funciones que llaman al BORDE: los métodos de red de las dos interfaces propias
//     (`Embed`/`EmbedBatch` del embebedor, `Ask` del motor) y toda llamada que, RESUELTO SU TIPO,
//     sale del proceso — `(*net/http.Client).Do`, `os/exec.Command[Context]`, o una función de
//     `internal/fleet` que llegue a alguna de ésas (derivadas con el mismo análisis sobre fleet);
//  3. se propaga hacia atrás por el grafo de llamadas del paquete hasta los handlers;
//  4. se lee el registro —`Name:`, `handler:` y `lock:` del mismo literal— y se cruza.
//
// Agregar una tool nueva que embeba o que salga por HTTP o por SSH, o mover una llamada de red a
// una función que hoy no la tiene, pone esto rojo sin que nadie tenga que acordarse de nada.
//
// SE EXIGE EN LOS DOS SENTIDOS, y el segundo no es simetría decorativa:
//
//	llega al borde ⇒ declara lockSelf ......... el defecto: el candado cruza la red
//	declara lockSelf ⇒ llega al borde ......... el otro: o la declaración quedó muerta cuando se
//	                                            sacó la llamada, o el ANÁLISIS SE QUEDÓ CIEGO y
//	                                            está devolviendo un conjunto vacío
//
// Sin la segunda dirección, esta guarda podría pasar en verde con el grafo roto: un `Inspect` que
// no matchea nada da cero violaciones. El verde de un barrido vacío se ve idéntico al de un
// árbol sano, y eso ya nos costó una sesión entera.
//
// EL ANÁLISIS SOBRE-APROXIMA A PROPÓSITO. El grafo liga por nombre simple de función, así que
// `s.foo()` y `foo()` son el mismo nodo y dos funciones homónimas se colapsan. Eso produce falsos
// POSITIVOS, nunca falsos negativos — para una guarda es la dirección segura: acusa de más y
// alguien mira, en vez de callar de menos y nadie mira.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EL BORDE SE RECONOCE POR TIPO Y NO POR NOMBRE
//
// Hasta el 2026-09-13 el borde eran sólo el embebedor y el motor, y el comentario de acá mismo lo
// declaraba: «cualquier OTRA salida a la red queda fuera de su vista». Medido ese día, eran SIETE
// tools sosteniendo el candado del despacho mientras esperaban la red —por HTTP al central o por
// SSH a una máquina—, y con `fleet_exec` en Tier B el servidor podía quedar tomado hasta 10 min.
//
// El arreglo obvio era agregar `Do` a la lista, y se descartó MIDIÉNDOLO: en este paquete hay seis
// `.http.Do(` y un `l.stopOnce.Do(`, que es un `sync.Once`. Por nombre, apagar algo parecería salir
// a la red. Por tipo resuelto, las dos llamadas son `(*net/http.Client).Do` y `(*sync.Once).Do`, y
// no se confunden. Las primitivas son un HECHO DEL MUNDO —qué funciones de la stdlib salen del
// proceso— y por eso van clavadas; lo que se deriva es quién llega a ellas.
//
// EL CHEQUEO DE TIPOS ES INDULGENTE A PROPÓSITO. Sólo se cargan de verdad los paquetes de la stdlib
// que definen primitivas; cualquier otro entra vacío y sus errores se ignoran. Alcanza para
// resolver `c.http.Do` y para reconocer `fleet.X` por la ruta del paquete, y evita pedirle al CI
// los datos de exportación de 329 dependencias compiladas sin `-race`. El precio es que no se
// resuelven métodos sobre tipos de OTROS paquetes: eso queda escrito abajo.
//
// Y SALE DEL MISMO TOOLCHAIN QUE LA PRUEBA, que no es un detalle: el primer experimento de esto
// corrió `go list -export` con el toolchain del repo (1.26) y leyó los datos con el importador de
// otro (1.22), y el decodificador reventó con un pánico. Dentro de `go test`, los dos son el mismo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE ESTA GUARDA TODAVÍA NO VE, Y HAY QUE DECIRLO PARA QUE NADIE LE CONFÍE DE MÁS
//
//   - Una llamada de red por un MÉTODO de un tipo de otro paquete (`canal.LeerDeLaPersona()`): el
//     chequeo indulgente carga ese paquete vacío y el método no se resuelve. Las funciones de
//     `internal/fleet` sí se ven, porque se reconocen por la ruta del paquete y su nombre.
//   - Una salida a la red por una primitiva que no está en la lista (`net.Dial`, un cliente gRPC):
//     agregarla es agregar un hecho del mundo, no una forma, y se hace el día que aparezca.
//   - Un paquete que no sea `internal/mcp` ni `internal/fleet` y salga a la red por su cuenta.
//
// El alcance es `internal/mcp`, y eso SÍ está medido, no supuesto: `internal/memory` no llama al
// embebedor ni una vez, así que el vector siempre se calcula en este paquete y baja como dato.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_recall` (dirección 1), o
// sacársela a una tool y borrarle además la llamada al embebedor (dirección 2). Los dos corridos.
//
// EL `de` ARRANCA EN EL COMENTARIO Y NO EN EL `lock:`, porque `lock: lockSelf,` aparece cinco veces
// en el registro y el arnés exige que el literal sea ÚNICO —con dos ocurrencias no se sabría cuál
// se tocó, y con cero el sabotaje no se aplica y su verde no diría nada—. La línea de arriba es la
// que lo ancla a ESTA tool.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\t// sin atender a nadie hasta 120 s. El handler acota su propia sección crítica.\n\t\t\tlock: lockSelf,"
// arnes: a="\t\t\t// sin atender a nadie hasta 120 s. El handler acota su propia sección crítica."
// arnes: prueba="TestNingunCandadoDelDespachoCruzaUnaLlamadaDeRed"
func TestNingunCandadoDelDespachoCruzaUnaLlamadaDeRed(t *testing.T) {
	a := analizarCandado(t)
	trinquete := map[string]bool{}
	for _, n := range toolsQueTodaviaCruzanLaRed {
		trinquete[n] = true
	}

	var faltanLock, lockMuerto []string
	for nombre, e := range a.tools {
		var llega bool
		var porQuien string
		for _, h := range e.handlers {
			if a.alcanza[h] {
				llega, porQuien = true, h
				break
			}
		}
		switch {
		case llega && !e.lockSelf && !trinquete[nombre]:
			faltanLock = append(faltanLock, nombre+" (por "+porQuien+", "+e.pos+")")
		case !llega && e.lockSelf:
			lockMuerto = append(lockMuerto, nombre+" ("+strings.Join(e.handlers, "/")+", "+e.pos+")")
		}
	}
	sort.Strings(faltanLock)
	sort.Strings(lockMuerto)

	if len(faltanLock) > 0 {
		t.Errorf("EL CANDADO DEL DESPACHO CRUZA UNA LLAMADA DE RED en %d tools. Sus handlers pueden "+
			"llegar al embebedor, al motor, a HTTP o a un proceso externo, y el despachador les toma el "+
			"candado ANTES de entrar: mientras esa llamada tarda, el servidor entero no atiende a nadie.\n"+
			"La regla está escrita en server.go y estas tools no la cumplen:\n  %s\n"+
			"Arreglo: `lock: lockSelf` en su entrada del registro, y acotar la sección crítica con "+
			"withReadLock/withWriteLock adentro del handler. NO la agregues a toolsQueTodaviaCruzanLaRed: "+
			"esa lista sólo puede achicarse.",
			len(faltanLock), strings.Join(faltanLock, "\n  "))
	}
	if len(lockMuerto) > 0 {
		t.Errorf("%d tools declaran `lockSelf` y su handler NO llega a ninguna llamada de red:\n  %s\n"+
			"Una de dos, y hay que decidir cuál: o la llamada se sacó y la declaración quedó muerta "+
			"(entonces borrala, porque `lockSelf` deja al handler sin el candado que el despachador "+
			"le daba), o el ANÁLISIS DE ESTA GUARDA se quedó ciego y su verde de arriba no vale.",
			len(lockMuerto), strings.Join(lockMuerto, "\n  "))
	}

	t.Logf("%d archivos · %d tools legibles (%d con handler ilegible) · %d funciones tocan el borde, "+
		"%d lo alcanzan transitivamente · %d llamadas a primitivas resueltas · salidas de fleet: %s",
		a.archivos, len(a.tools), a.sinHandlerLegible, len(a.enElBorde), len(a.alcanza),
		a.primitivasVistas, strings.Join(a.salidasFleet, ", "))
}

// toolsQueTodaviaCruzanLaRed es el TRINQUETE: las tools que, medido el 2026-09-13, sostienen el
// candado del despacho mientras esperan una salida de red o de proceso.
//
// No es una lista de excepciones permitidas: es DEUDA declarada, y sólo puede achicarse. La guarda
// de arriba ya no las acusa una por una para que el cambio de borde pudiera entrar en verde; la de
// abajo exige que el conjunto medido sea EXACTAMENTE éste, así que:
//
//   - una tool nueva que cruce la red sin `lockSelf` no puede esconderse agregándose acá sin que
//     alguien lo vea en el diff, y mientras no se agregue, la guarda de arriba la acusa;
//   - una tool de la lista que se arregló (o que dejó de cruzar) pone la de abajo en ROJO hasta que
//     se la saca — una entrada rancia es una deuda que figura pagada sin que nadie la cobre.
//
// LAS SIETE ORIGINALES coincidían con la tabla armada leyendo el código a mano el mismo día; dos
// métodos independientes dieron el mismo conjunto. De ésas ya salieron SEIS —promote_skill,
// install_skill, list_skills, codegraph_index, fleet_probe y fleet_exec, cada una con su prueba de
// EFECTO—, así que el «siete» de arriba es la foto del 2026-09-13 y no el largo de esta lista. El
// número vivo lo cuenta la guarda de abajo contra el código, que es el único lugar donde no se
// puede quedar rancio.
var toolsQueTodaviaCruzanLaRed = []string{
	// ── VACÍA DESDE EL 2026-09-14 ────────────────────────────────────────────────────────────
	//
	// La última salida NO fue una conversión: fue una CORRECCIÓN DE ESTA GUARDA.
	//
	// `musubi_fleet_shell` figuraba acá por «Tier B: AbrirShellPorSSH», y esa entrada era falsa.
	// `AbrirShellPorSSH` hace `cmd.Start()` y vuelve; su `cmd.Wait()` corre en una goroutine. O sea
	// que abrir una shell NUNCA sostuvo el candado esperando a nadie — medido, 0,19 s con un `ssh`
	// falso que duerme 30 s. Estaba en la lista porque `primitivasDeSalida` preguntaba «¿construye un
	// proceso?» (`os/exec.Command`) en vez de «¿lo espera?» (`(*exec.Cmd).Run`), y porque el recorrido
	// le imputaba al llamador un `Wait()` que corre en otra goroutine.
	//
	// Las dos cosas se arreglaron y shell salió sola, sin tocarle una línea a la tool. Lo custodia
	// TestLaGuardaDistingueEsperarDeCruzar, porque un arreglo de la guarda que la guarda misma no
	// vigila se deshace en el primer refactor y nadie se entera.
	//
	// QUE ESTÉ VACÍA NO LA VUELVE INÚTIL: sigue siendo el único lugar donde una tool nueva que
	// espere con el candado tomado puede declararse, y la guarda de arriba la acusa mientras no lo
	// haga. Lo que ya no puede es esconder una entrada que no corresponde.
	//
	// musubi_fleet_exec SALIÓ el 2026-09-14, la sexta, y es la primera con DOS cuelgues de dos
	// techos distintos — y sólo uno de los dos era una llamada de red:
	//
	//	Tier B  `EjecutarPorSSH` hace `cmd.Run()`, que espera de verdad, hasta ComandoTimeoutMax:
	//	        DIEZ MINUTOS con el servidor entero serializado.
	//	Tier A  `esperarComando` relee la bitácora cada 250 ms hasta `esperaMaxExec` (45 s). No es
	//	        red: es un bucle contra la propia base, así que ESTA GUARDA NO PODÍA VERLO —busca
	//	        salidas del paquete fleet— y congelaba igual.
	//
	// El candado del resultado y del latido va dentro de `correrPorSSH` porque esa función la
	// comparten la tool y el barrido de políticas; puesto en el handler, el camino automático
	// escribiría sin candado y nadie lo notaría. Lo que NO se tocó, a propósito, es
	// `encolarAvisoDeAcceso`: shell y pantalla entran ahí con el exclusivo del despacho ya tomado,
	// así que meterle el candado adentro sería un deadlock en producción para esas dos.
	// Lo miden TestEjecutarEnUnTierBNoCongelaElServidor y TestEsperarElResultadoNoCongelaElServidor.
	// musubi_fleet_probe SALIÓ el 2026-09-14, la quinta, y es la primera cuyo corte NO se pudo copiar
	// del molde anterior: las cuatro previas hacían UN viaje y bastaba acotar un tramo, y ésta sale a
	// medir hasta 20 máquinas a 15 s cada una. El candado se toma y se suelta POR MÁQUINA —
	// `withReadLock` para listar, nada durante el sondeo, `withWriteLock` sólo alrededor del UPDATE
	// del latido—, así que entre una máquina y la siguiente el servidor respira. Partirlo es seguro y
	// está medido: el latido es una sentencia única sobre la fila de ESE dispositivo, y el contador
	// de CPU compartido entre sondeos ya tenía su propio mutex. Ver
	// TestSondearLaFlotaNoCongelaElServidor.
	// musubi_codegraph_index SALIÓ el 2026-09-13, la cuarta, y es la que cruzaba DOS fronteras y no
	// una: el POST al central y el `git rev-parse` de `commitDeHEAD`. El sello del head pasó de
	// `directo` a withWriteLock, porque `directo` significaba «ya tengo el candado del despacho» y
	// con `lockSelf` dejó de ser cierto. El tick de fondo que hacía lo mismo —y que esta guarda NO
	// ve, por no ser una tool del registro— ya lo había arreglado #505. Ver
	// TestIndexarElGrafoNoCongelaElServidor.
	// musubi_list_skills SALIÓ el 2026-09-13, la tercera, y es el caso DISTINTO de las siete: era la
	// única que declaraba `readOnly`, o sea la única que sostenía el candado COMPARTIDO y no el
	// exclusivo. No la salvaba: un escritor esperando bloquea también a los lectores nuevos. Su
	// sección crítica es de LECTURA (`withReadLock` sobre LoadSkills, que lee el disco) y se conserva
	// porque `writeSkillFile` escribe esos mismos .yaml bajo el exclusivo. Ver
	// TestListarElArsenalNoCongelaElServidor.
	// musubi_install_skill SALIÓ el 2026-09-13, la segunda. A diferencia de promote, ésta SÍ toca la
	// base —`writeSkillFile` estampa el fingerprint del stack con `SetMeta`— así que no alcanzaba con
	// declarar `lockSelf`: hubo que acotar la sección crítica. Ver
	// TestInstalarUnaSkillNoCongelaElServidor.
	// musubi_promote_skill SALIÓ DE ACÁ el 2026-09-13: declara `lockSelf` y suelta el candado
	// durante el POST al central. Fue la primera porque es la más simple de las siete —lee las
	// skills del DISCO y empuja por HTTP, así que no necesita ninguna sección crítica— y sirvió
	// para estrenar el molde de prueba que las otras seis van a copiar:
	// TestPromoverUnaSkillNoCongelaElServidor.
}

// TestLasToolsQueTodaviaCruzanLaRedSonLasDelTrinquete exige que la deuda declarada sea la medida.
//
// Sabotaje que la hace fallar: que `musubi_sync_pull` —hoy sana: sólo lee la base— le pida el
// arsenal al central por HTTP antes de listar. Cruza la red sin `lockSelf` y no está en la lista.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="\titems, err := s.engine.ListSharedForPull(s.scopedCtx(ctx), args.AfterRowID, args.Limit)\n"
// arnes: a="\t_, _ = s.syncClient.ListArsenal(\"\")\n\titems, err := s.engine.ListSharedForPull(s.scopedCtx(ctx), args.AfterRowID, args.Limit)\n"
// arnes: prueba="TestLasToolsQueTodaviaCruzanLaRedSonLasDelTrinquete"
func TestLasToolsQueTodaviaCruzanLaRedSonLasDelTrinquete(t *testing.T) {
	a := analizarCandado(t)
	medidas := map[string]bool{}
	for nombre, e := range a.tools {
		if e.lockSelf {
			continue
		}
		for _, h := range e.handlers {
			if a.alcanza[h] {
				medidas[nombre] = true
				break
			}
		}
	}
	declaradas := map[string]bool{}
	for _, n := range toolsQueTodaviaCruzanLaRed {
		declaradas[n] = true
	}

	var nuevas, rancias []string
	for n := range medidas {
		if !declaradas[n] {
			nuevas = append(nuevas, n)
		}
	}
	for n := range declaradas {
		if !medidas[n] {
			rancias = append(rancias, n)
		}
	}
	sort.Strings(nuevas)
	sort.Strings(rancias)

	if len(nuevas) > 0 {
		t.Errorf("%d tool/s cruzan la red sin `lockSelf` y NO están en toolsQueTodaviaCruzanLaRed: %s\n"+
			"No se agregan a la lista: la lista es deuda que sólo baja. Declaralas `lockSelf` y acotá "+
			"su sección crítica con withReadLock/withWriteLock.", len(nuevas), strings.Join(nuevas, ", "))
	}
	if len(rancias) > 0 {
		t.Errorf("%d entrada/s de toolsQueTodaviaCruzanLaRed YA NO cruzan la red sin `lockSelf`: %s\n"+
			"Si las arreglaste, sacalas de la lista — es exactamente lo que la lista existe para registrar. "+
			"Si no las tocaste, el ANÁLISIS se quedó ciego y hay que mirarlo antes de creerle a nada.",
			len(rancias), strings.Join(rancias, ", "))
	}
}

// TestLaGuardaDistingueEsperarDeCruzar custodia la corrección del 2026-09-14: que el borde se defina
// por la ESPERA y no por la construcción de un proceso.
//
// POR QUÉ HACE FALTA UNA GUARDA PARA LA GUARDA. El arreglo son dos piezas —la tabla de primitivas y
// el corte de `*ast.GoStmt`— y ninguna de las dos tiene nada que la sostenga: reponer
// `os/exec.Command` en la tabla, o borrar el corte, deja todo compilando y en verde, y la guarda
// vuelve a confundir `AbrirShellPorSSH` con `EjecutarPorSSH` sin que nadie se entere. Un arreglo que
// se deshace en silencio no es un arreglo.
//
// EL PAR ES LA ASERCIÓN, no cada mitad por separado. Exigir sólo que EjecutarPorSSH esté en el borde
// la dejaría verde con la tabla vieja (que también lo incluía, por el motivo equivocado); exigir
// sólo que AbrirShellPorSSH NO esté la dejaría verde si alguien vacía la tabla entera y el análisis
// queda ciego. Las dos juntas son lo que distingue «mide la espera» de «no mide nada».
//
// Los dos son HECHOS DEL CÓDIGO, verificables en internal/fleet:
//
//	EjecutarPorSSH    remoto.go: `err := cmd.Run()` — espera al proceso, hasta 10 min.
//	AbrirShellPorSSH  shell_ssh.go: `cmd.Start()`, y el `cmd.Wait()` adentro de un `go func()`.
//
// SON DOS PRUEBAS Y NO UNA, Y LA RAZÓN LA DIO EL CENSO. La primera versión era una sola con DOS
// «Sabotaje que la hace fallar:» y una sola directiva `arnes:`, así que el segundo quedaba de
// promesa en prosa — y `TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre` se puso roja diciendo
// exactamente eso: «agregaste 1 promesa de sabotaje que nadie puede correr». Una directiva lleva un
// `prueba=`, así que dos sabotajes mecanizados piden dos pruebas. Partirlas además mejora el
// diagnóstico: el rojo dice CUÁL de las dos piezas del arreglo se deshizo.
//
// Sabotaje que la hace fallar: borrar el corte de `*ast.GoStmt` en grafoDeLlamadas — el `Wait()` de
// la goroutine de limpieza vuelve a imputarse al llamador y AbrirShellPorSSH reaparece en el borde.
// arnes: archivo="internal/mcp/candado_no_cruza_la_red_test.go"
// arnes: de="\t\t\t\tif _, esGoroutine := n.(*ast.GoStmt); esGoroutine {\n\t\t\t\t\treturn false\n\t\t\t\t}\n"
// arnes: a=""
// arnes: prueba="TestLaGuardaDistingueEsperarDeCruzar"
func TestLaGuardaDistingueEsperarDeCruzar(t *testing.T) {
	enElBorde := salidasDeFleetAlDia(t)

	// LA QUE ESPERA TIENE QUE ESTAR. `EjecutarPorSSH` hace `cmd.Run()` y puede tardar
	// ComandoTimeoutMax (10 min) con el candado tomado: es el caso que esta guarda existe para ver.
	if !enElBorde["EjecutarPorSSH"] {
		t.Errorf("EjecutarPorSSH NO figura entre las salidas de fleet (%v).\n"+
			"  Hace `cmd.Run()`, que espera al proceso hasta 10 minutos. Si el análisis dejó de verlo,\n"+
			"  la guarda entera quedó ciega justo para el caso más caro que tiene que atrapar.",
			clavesOrdenadas(enElBorde))
	}

	// LA QUE NO ESPERA NO PUEDE ESTAR. `AbrirShellPorSSH` hace `cmd.Start()` y vuelve; su `Wait()`
	// corre en una goroutine y no bloquea a quien la llamó. Medido: 0,19 s con un ssh que duerme 30.
	if enElBorde["AbrirShellPorSSH"] {
		t.Errorf("AbrirShellPorSSH volvió a figurar entre las salidas de fleet (%v).\n"+
			"  No espera a nadie: `cmd.Start()` y vuelve, con el `cmd.Wait()` adentro de un `go func()`.\n"+
			"  Que reaparezca por ACÁ significa que se borró el corte de *ast.GoStmt en grafoDeLlamadas,\n"+
			"  y entonces una espera que corre en otra goroutine vuelve a imputársele al llamador.",
			clavesOrdenadas(enElBorde))
	}
}

// TestLaTablaDePrimitivasPreguntaPorLaEspera custodia la OTRA mitad del arreglo: que
// `primitivasDeSalida` liste lo que ESPERA y no lo que construye.
//
// Es la pregunta simétrica de la de arriba y hace falta por separado, porque las dos piezas se
// deshacen por caminos distintos: aquélla cae si alguien borra el corte de goroutines; ésta, si
// alguien repone `os/exec.Command` —que sólo arma un *exec.Cmd y no sale a ningún lado— entre las
// primitivas. Con cualquiera de las dos rota, `AbrirShellPorSSH` vuelve al borde y `fleet_shell`
// vuelve al trinquete por un cuelgue que no existe.
//
// LA ASERCIÓN ES SOBRE LA TABLA Y NO SOBRE EL RESULTADO, a propósito: preguntar otra vez por
// AbrirShellPorSSH sería repetir la prueba de arriba con otro nombre, y el día que las dos midan lo
// mismo una de las dos deja de cubrir su mitad sin que nadie lo note.
//
// Sabotaje que la hace fallar: reponer "os/exec.Command" en primitivasDeSalida.
// arnes: archivo="internal/mcp/candado_no_cruza_la_red_test.go"
// arnes: de="\t\"(*os/exec.Cmd).Run\":            \"un proceso externo (lo espera)\","
// arnes: a="\t\"os/exec.Command\": \"un proceso externo\","
// arnes: prueba="TestLaTablaDePrimitivasPreguntaPorLaEspera"
func TestLaTablaDePrimitivasPreguntaPorLaEspera(t *testing.T) {
	// Construir un comando NO es salir: `exec.Command` devuelve un *exec.Cmd y no habla con nadie.
	// Si vuelve a la tabla, todo el que arme un comando entra al borde aunque nunca lo espere.
	for _, construye := range []string{"os/exec.Command", "os/exec.CommandContext"} {
		if motivo, esta := primitivasDeSalida[construye]; esta {
			t.Errorf("%q volvió a primitivasDeSalida (como %q), y no hace esperar a nadie:\n"+
				"  sólo CONSTRUYE un *exec.Cmd. Con esto en la tabla, `AbrirShellPorSSH` —que hace\n"+
				"  `cmd.Start()` y vuelve— pesa lo mismo que `EjecutarPorSSH`, que espera hasta 10\n"+
				"  minutos con el candado tomado. Lo que hay que listar es la ESPERA: (*os/exec.Cmd).Run\n"+
				"  y sus hermanas.", construye, motivo)
		}
	}

	// Y las que SÍ esperan tienen que seguir estando. Sin esta mitad, vaciar la tabla entera dejaría
	// la prueba en verde: cero primitivas es cero falsos positivos y también cero medición.
	for _, espera := range []string{"(*os/exec.Cmd).Run", "(*net/http.Client).Do"} {
		if _, esta := primitivasDeSalida[espera]; !esta {
			t.Errorf("%q salió de primitivasDeSalida. Es una espera real con el candado del despacho\n"+
				"  tomado; sin ella el análisis deja de ver el borde y su verde no distingue «no hay\n"+
				"  defecto» de «no estoy mirando».", espera)
		}
	}
}

// salidasDeFleetAlDia corre el análisis y devuelve las salidas exportadas de internal/fleet.
//
// Aborta si el conjunto viene vacío: con cero salidas, cualquier aserción de la forma «X no está»
// pasa por vacuidad, que es el verde que no distingue «está bien» de «no miré».
func salidasDeFleetAlDia(t *testing.T) map[string]bool {
	t.Helper()
	a := analizarCandado(t)
	enElBorde := map[string]bool{}
	for _, s := range a.salidasFleet {
		enElBorde[s] = true
	}
	if len(enElBorde) == 0 {
		t.Fatal("el análisis no derivó NI UNA salida de internal/fleet: está ciego, y con cero " +
			"salidas las aserciones de «no está» pasarían por vacuidad")
	}
	return enElBorde
}

// clavesOrdenadas devuelve las claves en orden, para que el mensaje de un fallo no cambie de una
// corrida a otra por el recorrido aleatorio de un mapa.
func clavesOrdenadas(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// primitivasDeSalida son las funciones de la stdlib que HACEN ESPERAR al que las llama. Es un HECHO
// DEL MUNDO y va clavado: derivarlo de las tools que ya declaran `lockSelf` volvería espejo a la guarda.
//
// ANTES DECÍAN «sacan una llamada del proceso» Y LISTABAN `os/exec.Command`. Era la pregunta
// equivocada, y se vio recién cuando quedó una sola tool en el trinquete:
//
//	`exec.Command` y `exec.CommandContext` NO salen a ningún lado — CONSTRUYEN un *exec.Cmd. Lo que
//	sale, y sobre todo lo que ESPERA, es `Run`, `Wait`, `Output` o `CombinedOutput`.
//
// La diferencia no es académica: `AbrirShellPorSSH` (internal/fleet/shell_ssh.go) hace `cmd.Start()`
// y vuelve — su `cmd.Wait()` corre en una goroutine—, así que abrir una shell tarda MILISEGUNDOS y
// nunca sostuvo el candado esperando nada. Medido: 0,19 s con un `ssh` falso que duerme 30 s. Con la
// tabla vieja, `musubi_fleet_shell` figuraba en el trinquete por «cruzar la red» cuando no espera a
// nadie, y `EjecutarPorSSH` —que hace `cmd.Run()` y puede tardar diez minutos— entraba por la MISMA
// puerta y con el mismo peso. La guarda no podía distinguirlas porque no estaba preguntando por la
// espera, sino por la construcción.
//
// Lo que importa para el candado del despacho es CUÁNTO SE ESPERA con él tomado. Una llamada que
// arranca un proceso y vuelve no congela a nadie; una que espera su salida, sí.
var primitivasDeSalida = map[string]string{
	"(*net/http.Client).Do":         "HTTP (espera la respuesta)",
	"(*os/exec.Cmd).Run":            "un proceso externo (lo espera)",
	"(*os/exec.Cmd).Wait":           "un proceso externo (lo espera)",
	"(*os/exec.Cmd).Output":         "un proceso externo (lo espera)",
	"(*os/exec.Cmd).CombinedOutput": "un proceso externo (lo espera)",
}

// bordesDeInterfaz son los métodos de red de las interfaces propias. Se reconocen por nombre porque
// la implementación concreta no se conoce en tiempo de compilación.
var bordesDeInterfaz = map[string]string{
	"Embed":      "embedding.Provider.Embed",
	"EmbedBatch": "embedding.BatchProvider.EmbedBatch / embedding.EmbedBatch",
	"Ask":        "cognition.Provider.Ask",
}

// stdlibDePrimitivas son los ÚNICOS paquetes que el chequeo de tipos carga de verdad.
var stdlibDePrimitivas = map[string]bool{"net/http": true, "os/exec": true, "sync": true, "context": true}

type importadorIndulgente struct {
	gc    types.Importer
	cache map[string]*types.Package
}

func (i *importadorIndulgente) Import(path string) (*types.Package, error) {
	if p, ok := i.cache[path]; ok {
		return p, nil
	}
	var p *types.Package
	if stdlibDePrimitivas[path] {
		var err error
		if p, err = i.gc.Import(path); err != nil {
			return nil, err
		}
	} else {
		nombre := path
		if j := strings.LastIndex(path, "/"); j >= 0 {
			nombre = path[j+1:]
		}
		p = types.NewPackage(path, nombre)
		p.MarkComplete()
	}
	i.cache[path] = p
	return p, nil
}

type entradaDeCandado struct {
	handlers []string // la cadena entera: el envoltorio Y el handler envuelto
	lockSelf bool
	pos      string
}

type analisisDeCandado struct {
	archivos          int
	sinHandlerLegible int
	tools             map[string]entradaDeCandado
	alcanza           map[string]bool
	enElBorde         map[string]string
	primitivasVistas  int
	salidasFleet      []string
}

// grafoDeLlamadas parsea y chequea tipos de un paquete y devuelve quién llama a quién (por nombre
// simple), qué funciones tocan el borde y cuántas llamadas a primitivas se resolvieron.
func grafoDeLlamadas(t *testing.T, fset *token.FileSet, imp types.Importer, pkgPath string,
	salidasFleet map[string]bool) (map[string]map[string]bool, map[string]string, int, []*ast.File) {
	t.Helper()
	out, err := guiones.Herramienta(t, "go", "list", "-json", pkgPath).Output()
	if err != nil {
		t.Fatalf("go list %s: %v — no medí nada", pkgPath, err)
	}
	var info struct {
		Dir     string
		GoFiles []string
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("no pude leer la salida de go list %s: %v", pkgPath, err)
	}
	var files []*ast.File
	for _, n := range info.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(info.Dir, n), nil, 0)
		if err != nil {
			t.Fatalf("%s: no pude parsear: %v", n, err)
		}
		files = append(files, f)
	}
	tipos := &types.Info{Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: imp, Error: func(error) {}}
	_, _ = conf.Check(pkgPath, fset, files, tipos)

	llama := map[string]map[string]bool{}
	borde := map[string]string{}
	primitivas := 0
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			yo := fn.Name.Name
			if llama[yo] == nil {
				llama[yo] = map[string]bool{}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				// ── LO QUE CORRE EN UNA GOROUTINE NO HACE ESPERAR A QUIEN LA LANZÓ ───────────
				//
				// `go func() { cmd.Wait() }()` devuelve el control en el acto: el llamador sigue y
				// suelta el candado, y el Wait bloquea a OTRA goroutine que no lo tiene. Sin este
				// corte, esa espera se le imputaba igual al llamador.
				//
				// Es la mitad que faltaba para que la tabla de primitivas signifique algo. Con las
				// primitivas corregidas pero sin esto, `AbrirShellPorSSH` seguiría figurando en el
				// borde por su `cmd.Wait()` de la goroutine de limpieza — o sea que el arreglo
				// habría quedado verde y falso, midiendo una espera que nadie espera.
				//
				// MEDIDO antes de escribirlo: `ast.Inspect` SÍ desciende en el cuerpo de un GoStmt
				// (ve `adentroDeGoroutine` y `directa`); cortando acá ve sólo `directa`.
				if _, esGoroutine := n.(*ast.GoStmt); esGoroutine {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var nombre, motivo string
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					nombre = fun.Name
				case *ast.SelectorExpr:
					if fun.Sel == nil {
						return true
					}
					nombre = fun.Sel.Name
					var obj types.Object
					if s, hay := tipos.Selections[fun]; hay {
						obj = s.Obj()
					} else if o, hay := tipos.Uses[fun.Sel]; hay {
						obj = o
					}
					if f, ok := obj.(*types.Func); ok {
						if tipo, es := primitivasDeSalida[f.FullName()]; es {
							primitivas++
							motivo = tipo + " (" + f.FullName() + ")"
						}
					}
					if motivo == "" && salidasFleet != nil {
						if x, ok := fun.X.(*ast.Ident); ok {
							if pn, ok := tipos.Uses[x].(*types.PkgName); ok &&
								pn.Imported().Path() == "musubi/internal/fleet" && salidasFleet[fun.Sel.Name] {
								motivo = "fleet." + fun.Sel.Name
							}
						}
					}
				default:
					return true
				}
				llama[yo][nombre] = true
				if motivo == "" {
					if d, es := bordesDeInterfaz[nombre]; es {
						motivo = d
					}
				}
				if motivo != "" {
					if _, ya := borde[yo]; !ya {
						borde[yo] = motivo + " @ " + fset.Position(call.Pos()).String()
					}
				}
				return true
			})
		}
	}
	return llama, borde, primitivas, files
}

func propagarAlcance(llama map[string]map[string]bool, borde map[string]string) map[string]bool {
	alcanza := map[string]bool{}
	for fn := range borde {
		alcanza[fn] = true
	}
	for cambio := true; cambio; {
		cambio = false
		for quien, llamadas := range llama {
			if alcanza[quien] {
				continue
			}
			for a := range llamadas {
				if alcanza[a] {
					alcanza[quien] = true
					cambio = true
					break
				}
			}
		}
	}
	return alcanza
}

// analizarCandado corre el análisis entero una vez. Las dos pruebas de arriba lo comparten.
func analizarCandado(t *testing.T) analisisDeCandado {
	t.Helper()
	fset := token.NewFileSet()
	lookup := func(path string) (io.ReadCloser, error) {
		out, err := guiones.Herramienta(t, "go", "list", "-export", "-f", "{{.Export}}", path).Output()
		if err != nil {
			return nil, err
		}
		return os.Open(strings.TrimSpace(string(out)))
	}
	imp := &importadorIndulgente{gc: importer.ForCompiler(fset, "gc", lookup), cache: map[string]*types.Package{}}

	// ── 1) internal/fleet: qué funciones EXPORTADAS llegan a una primitiva.
	llamaF, bordeF, _, _ := grafoDeLlamadas(t, fset, imp, "musubi/internal/fleet", nil)
	alcanzaF := propagarAlcance(llamaF, bordeF)
	salidas := map[string]bool{}
	var a analisisDeCandado
	for fn := range alcanzaF {
		if fn != "" && fn[0] >= 'A' && fn[0] <= 'Z' {
			salidas[fn] = true
			a.salidasFleet = append(a.salidasFleet, fn)
		}
	}
	sort.Strings(a.salidasFleet)
	if len(salidas) == 0 {
		t.Fatal("no derivé NI UNA función exportada de internal/fleet que salga a la red. Ese paquete " +
			"abre SSH y HTTP hacia las máquinas: si el análisis no lo ve, está ciego y su verde no vale")
	}

	// ── 2) internal/mcp, con el borde completo.
	llama, borde, primitivas, files := grafoDeLlamadas(t, fset, imp, "musubi/internal/mcp", salidas)
	a.archivos = len(files)
	a.enElBorde = borde
	a.primitivasVistas = primitivas
	a.alcanza = propagarAlcance(llama, borde)
	a.tools = map[string]entradaDeCandado{}

	// ── 3) EL REGISTRO. `Name:`, `handler:` y `lock:` viven en el MISMO literal, así que se leen
	// juntos y no hay que aparear por cercanía de líneas.
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var nombre, pos string
			var handlers []string
			var teniaCampoHandler, lockSelf bool
			ast.Inspect(lit, func(m ast.Node) bool {
				kv, ok := m.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				k, ok := kv.Key.(*ast.Ident)
				if !ok {
					return true
				}
				switch k.Name {
				case "Name":
					if bl, ok := kv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
						if v, err := strconv.Unquote(bl.Value); err == nil && strings.HasPrefix(v, "musubi_") {
							nombre = v
							pos = fset.Position(bl.Pos()).String()
						}
					}
				case "handler":
					// EL HANDLER PUEDE VENIR ENVUELTO, y ahí se cayó la primera versión de esta
					// guarda: `handler: s.countingSaveCtx(s.toolSaveObservation)` es un CallExpr,
					// no un selector. Se recogen TODOS los nombres del árbol del valor —el envoltorio
					// y lo envuelto—: cualquiera de los dos que alcance el borde compromete el candado.
					teniaCampoHandler = true
					vistos := map[string]bool{}
					ast.Inspect(kv.Value, func(h ast.Node) bool {
						switch v := h.(type) {
						case *ast.SelectorExpr:
							if v.Sel != nil && !vistos[v.Sel.Name] {
								vistos[v.Sel.Name] = true
								handlers = append(handlers, v.Sel.Name)
							}
							return false
						case *ast.Ident:
							if !vistos[v.Name] {
								vistos[v.Name] = true
								handlers = append(handlers, v.Name)
							}
						}
						return true
					})
				case "lock":
					if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "lockSelf" {
						lockSelf = true
					}
				}
				return true
			})
			switch {
			case nombre != "" && len(handlers) > 0:
				a.tools[nombre] = entradaDeCandado{handlers: handlers, lockSelf: lockSelf, pos: pos}
			case nombre != "" && teniaCampoHandler:
				a.sinHandlerLegible++
				t.Errorf("%s: la tool `%s` declara `handler:` en una forma que esta guarda no sabe "+
					"leer, así que quedó FUERA del análisis. Arreglá el extractor: saltearla "+
					"convierte esta guarda en un verde que no cubre esa tool", pos, nombre)
			}
			return true
		})
	}

	// ════════════════════════════════════════════════════════════════════════════════════════
	// CONTROLES DE «MIRÉ ALGO». Todos fatales: un cero significa «no pude medir», no «está bien».
	// ════════════════════════════════════════════════════════════════════════════════════════
	if a.archivos == 0 {
		t.Fatal("no parseé NI UN archivo de internal/mcp. El barrido no llegó: esta guarda no midió nada")
	}
	if len(a.tools) == 0 {
		t.Fatal("no encontré NI UNA tool con `Name:` y `handler:` en el mismo literal. O el registro " +
			"cambió de forma, o esta guarda quedó apuntando al aire — en los dos casos dejó de cubrir")
	}
	if len(borde) == 0 {
		t.Fatal("no vi NI UNA llamada al borde en todo internal/mcp: esta guarda quedó CIEGA")
	}
	// EL CONTROL QUE SÓLO EXISTE PORQUE EL CHEQUEO DE TIPOS PUEDE FALLAR CALLADO. Si el importador no
	// consigue cargar net/http, `c.http.Do` queda sin resolver, el chequeo indulgente no se queja, y
	// las salidas por HTTP desaparecen del análisis sin un error. En este paquete hay llamadas a
	// primitivas: cero resueltas es ceguera, no ausencia.
	if primitivas == 0 {
		t.Fatal("no resolví NI UNA llamada a una primitiva de salida en internal/mcp, y el paquete sale " +
			"por HTTP al central. El chequeo de tipos no cargó la stdlib: el borde por tipo está ciego")
	}
	return a
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LA MITAD DE COMPORTAMIENTO: la guarda de arriba mira la FORMA, ésta mide el EFECTO.
//
// Una guarda estructural sola no alcanza. Puede estar verde con `lock: lockSelf` declarado y el
// embed adentro del `withWriteLock`, que es exactamente el defecto con la marca puesta. Así que
// acá se cuelga el embebedor de verdad, se deja a `musubi_save_observation` atrapada dentro de la
// llamada de red, y se exige que OTRO escritor entre igual.
//
// POR QUÉ ESTA MEDICIÓN NO EXISTÍA. La sonda de G5 —`exigeQueUnEscritorSinEmbedderResponda`— fue
// elegida para NO pasar por el embebedor, y su comentario lo dice: «guardar una observación
// también pasa por el embedder y la sonda quedaría atrapada en el mismo cuelgue que se está
// midiendo». El arnés esquivaba el caso defectuoso a propósito, y el `embedderBloqueante` que hacía
// falta para medirlo estaba escrito, con su interruptor `activo` y su comentario explicando para
// qué servía, y NADIE lo usaba. Construido y nunca prendido, en el propio arnés de prueba.
//
// LA SONDA TIENE QUE SER ESCRITORA, y no es un detalle: una de sólo lectura pasaría igual con el
// defecto puesto si el tramo tomara RLock, porque dos lectores conviven. El escritor es el único
// que se bloquea con CUALQUIER candado tomado, compartido o exclusivo.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_save_observation` en el registro.
// Corrido: la sonda concurrente no vuelve y la prueba declara el bloqueo.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\t// del despacho esta tool congelaba el servidor entero 8,006 s.\n\t\t\tlock: lockSelf,"
// arnes: a="\t\t\t// del despacho esta tool congelaba el servidor entero 8,006 s."
// arnes: prueba="TestGuardarUnaObservacionNoCongelaElServidor"
func TestGuardarUnaObservacionNoCongelaElServidor(t *testing.T) {
	emb := nuevoEmbedderBloqueante()
	s := newTestServer(t, emb)

	// El interruptor se prende DESPUÉS de construir el servidor: si el embebedor colgara desde el
	// arranque, frenaría cualquier siembra y la prueba moriría antes de medir.
	emb.activo.Store(true)

	guardado := make(chan *RpcError, 1)
	go func() {
		guardado <- llamarSinT(s, "musubi_save_observation", map[string]interface{}{
			"topic_key": "candado/medicion",
			"content":   "esta observación se queda colgada en el embebedor a propósito, para medir si el servidor sigue atendiendo",
		})
	}()

	// SE ESPERA A ESTAR ADENTRO DEL Embed. Sin esto la sonda podría correr antes de que la tool
	// llegue a la llamada de red, y la prueba pasaría con el defecto puesto — un verde que mide
	// el momento equivocado.
	select {
	case <-emb.entro:
	case <-time.After(esperaArranque):
		t.Fatal("musubi_save_observation no llegó al embebedor: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_save_observation está colgada dentro del embebedor")

	close(emb.soltar)
	select {
	case rpcErr := <-guardado:
		if rpcErr != nil {
			t.Fatalf("al soltar el embebedor, el guardado falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el embebedor, el guardado igual no volvió: el candado quedó tomado por otra cosa")
	}
}
