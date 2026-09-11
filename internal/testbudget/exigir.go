package testbudget

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// nombreFlagTimeout es el flag que `go test` le pasa al binario de test.
const nombreFlagTimeout = "test.timeout"

// Observacion es lo que el guard vio cuando corrió, guardado para que se pueda AFIRMAR sobre
// ello después, desde un test del propio paquete.
//
// POR QUÉ EXISTE. El guard vive en TestMain, o sea que corre ANTES de que haya un *testing.T:
// si nadie lo llama, o si corre y no puede leer nada, no queda rastro y la suite pasa igual.
// Dos sabotajes de una línea vivían exactamente ahí —borrar la llamada, y borrar el flag.Parse()
// que la hace posible— y los dos quedaban en verde. Con la observación guardada, un test común
// del mismo paquete (ver ErrorSiElGuardNoCorrio) puede exigir que el guard haya corrido Y que
// haya podido medir.
type Observacion struct {
	Timeout      time.Duration // el -test.timeout que se leyó
	Leido        bool          // si se pudo leer de verdad
	Motivo       string        // por qué no se pudo, cuando Leido es false
	BajoDetector bool          // si el binario está instrumentado con -race
	Politica     string        // error de la política, si no se pudo cargar
	Aviso        string        // el aviso NO fatal que se imprimió, si hubo (ver decidirTimeout)
}

var (
	muObservacion sync.Mutex
	ultimaObs     *Observacion
)

func guardarObservacion(o Observacion) {
	muObservacion.Lock()
	defer muObservacion.Unlock()
	ultimaObs = &o
}

// UltimaObservacion devuelve lo que vio la última llamada a ExigirTimeoutSuficiente en este
// proceso. El segundo retorno es false cuando NADIE la llamó — que no es lo mismo que haberla
// llamado y no haber podido medir.
func UltimaObservacion() (Observacion, bool) {
	muObservacion.Lock()
	defer muObservacion.Unlock()
	if ultimaObs == nil {
		return Observacion{}, false
	}
	return *ultimaObs, true
}

// TimeoutDeEstaCorrida devuelve el `-timeout` con el que arrancó este binario de test, y si se
// pudo leer.
//
// NO DEVUELVE 0 CUANDO NO SABE. `-timeout 0` es un valor LEGÍTIMO —significa «sin límite»— así
// que un cero no puede además significar «no encontré el flag»: son dos cosas opuestas (la más
// permisiva posible y la ignorancia total). Por eso el segundo retorno.
func TimeoutDeEstaCorrida() (time.Duration, bool) {
	d, ok, _ := leerTimeout(flag.Lookup, os.Args)
	return d, ok
}

// leerTimeout es la lectura, separada de sus fuentes para poder probar SUS TRES RAMAS DE
// FRACASO. Antes las tres devolvían «no pude leer» y ninguna estaba probada: el lector estaba
// probado de un solo lado.
//
// Y hace algo más que leer el flag: lo CONTRASTA con la línea de comando. `go test` siempre le
// pasa `-test.timeout=…` al binario, pero el valor no aparece en el flag hasta que alguien
// llama a flag.Parse(); quien lo llama de verdad es m.Run(), que corre DESPUÉS del TestMain.
// Sin ese contraste, un TestMain al que le falta el flag.Parse() lee 0 —«sin límite»— y el
// guard entero queda MUDO sin que nada lo note. El contraste convierte ese silencio en un
// «no pude leer», que es rojo.
func leerTimeout(buscar func(string) *flag.Flag, argv []string) (time.Duration, bool, string) {
	enArgv, hayEnArgv := timeoutEnArgv(argv)

	f := buscar(nombreFlagTimeout)
	if f == nil {
		return 0, false, fmt.Sprintf("el binario no registró el flag -%s: esto no parece un "+
			"binario de test de Go (¿testing.Init() no corrió?)", nombreFlagTimeout)
	}
	g, ok := f.Value.(flag.Getter)
	if !ok {
		return 0, false, fmt.Sprintf("el flag -%s no expone su valor (no es un flag.Getter): "+
			"cambió la implementación del paquete testing", nombreFlagTimeout)
	}
	d, ok := g.Get().(time.Duration)
	if !ok {
		return 0, false, fmt.Sprintf("el flag -%s no es una duración sino %T: cambió la "+
			"implementación del paquete testing", nombreFlagTimeout, g.Get())
	}
	if hayEnArgv && d != enArgv {
		return 0, false, fmt.Sprintf("la línea de comando dice -%s=%s y el flag dice %s: los "+
			"flags todavía no se parsearon, así que el valor leído no es el de esta corrida. "+
			"Un TestMain tiene que llamar a flag.Parse() ANTES de mirar el presupuesto (quien "+
			"parsea normalmente es m.Run(), y para entonces ya es tarde para avisar)",
			nombreFlagTimeout, enArgv, d)
	}
	return d, true, ""
}

// timeoutEnArgv busca -test.timeout en la línea de comando, en cualquiera de sus formas
// (`-x=v`, `--x=v`, `-x v`, `--x v`). Es la fuente que NO depende de que alguien haya parseado.
func timeoutEnArgv(argv []string) (time.Duration, bool) {
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		nombre := strings.TrimLeft(a, "-")
		valor, conIgual := "", false
		if k, v, ok := strings.Cut(nombre, "="); ok {
			nombre, valor, conIgual = k, v, true
		}
		if nombre != nombreFlagTimeout {
			continue
		}
		if !conIgual {
			if i+1 >= len(argv) {
				return 0, false
			}
			valor = argv[i+1]
		}
		d, err := time.ParseDuration(valor)
		if err != nil {
			return 0, false
		}
		return d, true
	}
	return 0, false
}

// ErrorSiElGuardNoCorrio es lo que llama un test común del paquete caro para exigir que el guard
// del presupuesto haya corrido de verdad en ESTE proceso.
//
// Cierra los dos sabotajes de una línea que dejaban todo el aparato mudo:
//   - borrar la llamada a ExigirTimeoutSuficiente del TestMain → no hay observación;
//   - borrar el flag.Parse() del TestMain → hay observación, pero no pudo leer el flag.
//
// Y no depende de -race: el guard sólo EXIGE el techo cuando corre bajo el detector, pero MIRA
// siempre, así que esta verificación es igual de dura en una corrida normal.
func ErrorSiElGuardNoCorrio(argv []string) error {
	obs, corrio := UltimaObservacion()
	if !corrio {
		return errors.New("el guard del presupuesto NO corrió en este paquete: el TestMain tiene " +
			"que llamar a testbudget.ExigirTimeoutSuficiente(\".\") antes de m.Run(). Sin eso, " +
			"`go test -race ./...` a secas vuelve a morir con «panic: test timed out» diez " +
			"minutos después, culpando al test que justo estuviera corriendo")
	}
	if !obs.Leido {
		return fmt.Errorf("el guard del presupuesto corrió pero no pudo leer el techo de esta "+
			"corrida, así que quedó mudo: %s", obs.Motivo)
	}
	if obs.Politica != "" {
		return fmt.Errorf("el guard del presupuesto no pudo leer la política: %s", obs.Politica)
	}
	if enArgv, hay := timeoutEnArgv(argv); hay && enArgv != obs.Timeout {
		return fmt.Errorf("el guard leyó -%s=%s y la línea de comando de esta corrida dice %s: "+
			"está juzgando un techo que no es el que rige", nombreFlagTimeout, obs.Timeout, enArgv)
	}
	return nil
}

// ExigirTimeoutSuficiente devuelve un motivo (no vacío) cuando este paquete está corriendo bajo
// `-race` con un `-timeout` MENOR que RACE_TIMEOUT, el presupuesto con el que corre CI. Y cuando
// el `-timeout` alcanza para CI pero no para el piso que la política pide en una máquina de
// desarrollo (TIMEOUT_LOCAL), no devuelve motivo: IMPRIME un aviso por stderr y deja correr. El
// porqué de esa decisión está en decidirTimeout.
//
// PARA QUÉ. `go test -race ./...` a secas usa el default de Go —10 min POR PAQUETE— y los
// paquetes caros de este repo ya están arriba de la mitad de eso. Cuando no alcanza, lo que se
// ve es «panic: test timed out after 10m0s» con el stack del test que le tocó estar corriendo:
// un mensaje que acusa a un test inocente, diez minutos después, y que no nombra el flag. Media
// docena de personas leyeron eso como un deadlock antes de que alguien midiera.
//
// Con esto, la misma corrida falla en el primer segundo diciendo QUÉ flag falta. Es la única
// forma que hay en Go de que «el que corre reciba el timeout solo»: `go test` no lee ninguna
// configuración del módulo, así que no se puede fijar un default; lo que sí se puede es que el
// error lo diga en vez de dejarlo adivinar.
//
// dir es desde dónde buscar la raíz del repo (normalmente "."). Si la política no se puede leer
// eso TAMBIÉN es un motivo: un guard que se calla cuando no puede mirar es el guard que este
// repo ya pagó dos veces.
//
// SIEMPRE DEJA RASTRO. Aunque devuelva "" —sin detector, o con techo de sobra— guarda una
// Observacion, para que ErrorSiElGuardNoCorrio pueda exigir después que esto haya pasado. El
// aviso, si hubo, queda ahí también (Observacion.Aviso), para que se pueda afirmar sobre él.
func ExigirTimeoutSuficiente(dir string) string {
	d, leido, motivo := leerTimeout(flag.Lookup, os.Args)
	return exigir(BajoDetector, dir, d, leido, motivo)
}

// exigir es TODO el cableado —cargar la política, decidir, guardar la observación e IMPRIMIR el
// aviso— con el detector y la lectura del flag como parámetros.
//
// Es el mismo motivo por el que decidirTimeout está separada, una capa más arriba: sin este
// seam, la única prueba posible del aviso sería llamar a la función pura y volver a imprimir el
// resultado desde el test, que es la forma de dejar el cable sin nadie en el medio —este repo ya
// se comió dos veces en un día una prueba que fijaba la forma de la salida sin ejercitar a quien
// la produce—.
func exigir(bajoDetector bool, dir string, timeout time.Duration, leido bool, motivo string) string {
	obs := Observacion{Timeout: timeout, Leido: leido, Motivo: motivo, BajoDetector: bajoDetector}
	pol, err := CargarPoliticaDelRepo(dir)
	if err != nil {
		obs.Politica = err.Error()
	}
	fatal, aviso := decidirTimeout(bajoDetector, timeout, leido, motivo, func() (Politica, error) {
		return pol, err
	})
	obs.Aviso = aviso
	guardarObservacion(obs)
	// EL AVISO SE IMPRIME ACÁ Y NO SE DEVUELVE. Los TestMain que llaman a esto tratan el valor de
	// retorno como fatal (`os.Exit(1)`), así que devolverlo sería convertir el aviso en rojo. Y
	// un aviso que se calcula y no se muestra es la otra mitad del mismo defecto: la guarda que
	// decide y no actúa.
	if aviso != "" {
		fmt.Fprintln(salidaAvisos, "presupuesto de pruebas:", aviso)
	}
	return fatal
}

// salidaAvisos es dónde va el aviso NO fatal. Es una variable para que la prueba pueda AFIRMAR
// que se imprimió de verdad; en producción es siempre os.Stderr.
var salidaAvisos io.Writer = os.Stderr

// decidirTimeout es la DECISIÓN, separada de dónde salen los datos. Devuelve (fatal, aviso):
// el primero es lo que hace `os.Exit(1)` en el TestMain, el segundo es lo que se imprime sin
// romper nada. Como mucho uno de los dos es no vacío.
//
// Está separada a propósito y no por gusto: `BajoDetector` es una constante de build tag, así
// que una prueba corriendo sin `-race` no podría ejercitar NUNCA la rama que importa —quedaría
// verde sin haber mirado nada, que es la clase de guarda que este repo ya juntó siete veces—.
// Con el detector como PARÁMETRO, los dos lados se prueban en cualquier corrida de Linux.
//
// LAS TRES RAMAS, Y POR QUÉ EL DEL MEDIO NO ES ROJO. La política tiene dos techos porque son dos
// entornos (ver el bloque «DOS TECHOS» de presupuesto-de-pruebas.env): RACE_TIMEOUT es el
// presupuesto con el que corre CI y TIMEOUT_LOCAL es el piso que necesita una máquina de
// desarrollo, que es ~3× más lenta. Esta función no puede saber en cuál de los dos está —no hay
// forma de medir la máquina en el primer segundo, y olfatear variables de entorno de CI sería
// hacer que la guarda se comporte distinto según dónde corre, que es justo lo que la vuelve poco
// confiable—. Entonces:
//
//	-timeout < RACE_TIMEOUT ............ FATAL. Es el piso que vale en todos los entornos: CI
//	                                     pasa exactamente RACE_TIMEOUT, así que esta rama no
//	                                     puede poner rojo a CI, y sigue atrapando el default de
//	                                     Go de 10m, que es el defecto que abrió este cabo.
//	RACE_TIMEOUT ≤ -timeout < LOCAL .... AVISO, con los números. Romperle la corrida acá a quien
//	                                     pasó el número de CI sería una guarda que la gente
//	                                     termina apagando; darle verde y callarse sería mandarlo
//	                                     a adivinar de vuelta cuando la corrida muera a los 20m.
//	-timeout ≥ TIMEOUT_LOCAL ........... silencio.
//
// El aviso se ve donde hace falta y no molesta donde no: medido, `go test` sin -v DESCARTA la
// salida de un paquete que pasa y la MUESTRA cuando falla o se cuelga, así que el aviso aparece
// pegado arriba del «panic: test timed out».
func decidirTimeout(bajoDetector bool, timeout time.Duration, timeoutLeido bool, motivo string, cargar func() (Politica, error)) (string, string) {
	if !bajoDetector {
		// Sin el detector estos paquetes entran holgados en el default de Go; exigir el techo
		// grande acá volvería rojas corridas que hoy pasan sanas (el job `test-cross` y el paso
		// de treesitter corren sin -race).
		return "", ""
	}
	pol, err := cargar()
	if err != nil {
		return fmt.Sprintf("no se pudo leer la política de presupuesto: %v", err), ""
	}
	if !timeoutLeido {
		return "no se pudo leer el flag -test.timeout de esta corrida: el guard del presupuesto " +
			"no puede decir si el techo alcanza, y callarse sería darlo por bueno — " + motivo, ""
	}
	if timeout == 0 {
		return "", "" // -timeout 0 = sin límite. Alcanza por definición.
	}
	if timeout < pol.Timeout {
		return fmt.Sprintf(
			"este paquete corre bajo -race con -timeout %s, y la política del repo (%s) pide al\n"+
				"    menos %s.\n"+
				"    El default de Go son 10m POR PAQUETE y este paquete ya no entra ahí: la corrida\n"+
				"    terminaría en «panic: test timed out» diez minutos después, culpando al test que\n"+
				"    justo estuviera corriendo. Corré:\n"+
				"        go test -race -timeout %s ./...\n"+
				"    (%s es el presupuesto con el que corre CI, o sea el mínimo; %s es lo que la\n"+
				"    política pide para una máquina de desarrollo, que es más lenta. -timeout 0 es\n"+
				"    sin límite.)",
			timeout, NombreArchivoPolitica, pol.Timeout, pol.TimeoutLocal, pol.Timeout,
			pol.TimeoutLocal), ""
	}
	if timeout < pol.TimeoutLocal {
		return "", fmt.Sprintf(
			"corrés bajo -race con -timeout %s, que alcanza el presupuesto de CI (%s) pero no el "+
				"piso que %s pide para una máquina de desarrollo (%s). En la peor máquina fechada "+
				"el paquete más lento midió %.1fs, o sea %.2f× contra los %.2f× que exige la "+
				"política: si tu máquina es más lenta que el runner, esta corrida puede morir con "+
				"«panic: test timed out after %s». Si eso pasa, no es un deadlock: corré "+
				"`go test -race -timeout %s ./...` (o -timeout 0).",
			timeout, pol.Timeout, NombreArchivoPolitica, pol.TimeoutLocal,
			pol.MedicionLocal.Seconds(), float64(timeout)/float64(pol.MedicionLocal),
			pol.MargenMinimo, timeout, pol.TimeoutLocal)
	}
	return "", ""
}

// guardarObservacionNil borra la observación. Existe para que las pruebas puedan ejercitar el
// caso «nadie llamó al guard», que es uno de los dos sabotajes que esto cierra.
func guardarObservacionNil() {
	muObservacion.Lock()
	defer muObservacion.Unlock()
	ultimaObs = nil
}
