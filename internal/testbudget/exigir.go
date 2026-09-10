package testbudget

import (
	"errors"
	"flag"
	"fmt"
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
// `-race` con un `-timeout` MENOR que el techo que declara la política del repo.
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
// Observacion, para que ErrorSiElGuardNoCorrio pueda exigir después que esto haya pasado.
func ExigirTimeoutSuficiente(dir string) string {
	d, leido, motivo := leerTimeout(flag.Lookup, os.Args)
	obs := Observacion{Timeout: d, Leido: leido, Motivo: motivo, BajoDetector: BajoDetector}
	pol, err := CargarPoliticaDelRepo(dir)
	if err != nil {
		obs.Politica = err.Error()
	}
	guardarObservacion(obs)
	return decidirTimeout(BajoDetector, d, leido, motivo, func() (Politica, error) {
		return pol, err
	})
}

// decidirTimeout es la DECISIÓN, separada de dónde salen los datos.
//
// Está separada a propósito y no por gusto: `BajoDetector` es una constante de build tag, así
// que una prueba corriendo sin `-race` no podría ejercitar NUNCA la rama que importa —quedaría
// verde sin haber mirado nada, que es la clase de guarda que este repo ya juntó siete veces—.
// Con el detector como PARÁMETRO, los dos lados se prueban en cualquier corrida de Linux.
func decidirTimeout(bajoDetector bool, timeout time.Duration, timeoutLeido bool, motivo string, cargar func() (Politica, error)) string {
	if !bajoDetector {
		// Sin el detector estos paquetes entran holgados en el default de Go; exigir el techo
		// grande acá volvería rojas corridas que hoy pasan sanas (el job `test-cross` y el paso
		// de treesitter corren sin -race).
		return ""
	}
	pol, err := cargar()
	if err != nil {
		return fmt.Sprintf("no se pudo leer la política de presupuesto: %v", err)
	}
	if !timeoutLeido {
		return "no se pudo leer el flag -test.timeout de esta corrida: el guard del presupuesto " +
			"no puede decir si el techo alcanza, y callarse sería darlo por bueno — " + motivo
	}
	if timeout == 0 {
		return "" // -timeout 0 = sin límite. Alcanza por definición.
	}
	if timeout >= pol.Timeout {
		return ""
	}
	return fmt.Sprintf(
		"este paquete corre bajo -race con -timeout %s, y la política del repo (%s) pide %s.\n"+
			"    El default de Go son 10m POR PAQUETE y este paquete ya no entra ahí: la corrida\n"+
			"    terminaría en «panic: test timed out» diez minutos después, culpando al test que\n"+
			"    justo estuviera corriendo. Corré:\n"+
			"        go test -race -timeout %s ./...\n"+
			"    (o -timeout 0 para sin límite).",
		timeout, NombreArchivoPolitica, pol.Timeout, pol.Timeout)
}

// guardarObservacionNil borra la observación. Existe para que las pruebas puedan ejercitar el
// caso «nadie llamó al guard», que es uno de los dos sabotajes que esto cierra.
func guardarObservacionNil() {
	muObservacion.Lock()
	defer muObservacion.Unlock()
	ultimaObs = nil
}
