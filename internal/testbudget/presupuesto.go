// Package testbudget juzga el presupuesto de tiempo de la suite CONTRA LA MEDICIÓN REAL de la
// corrida, en vez de contra un número tipeado a mano en un comentario.
//
// EL DEFECTO QUE CIERRA. El `-timeout` de CI estaba justificado por un comentario con los
// segundos de cada paquete y la fecha en que se midieron. Ese comentario envejeció tres veces
// —246,7 s → 379 s → más de 560 s en el paquete más caro— y las tres veces siguió afirmando el
// margen viejo, porque un número en prosa no tiene quién lo contradiga. El techo dejó de
// significar lo que decía y `go test -race ./...` con el default de Go pasó a caerse, sin que
// CI lo viera (CI pasa un `-timeout` explícito).
//
// CÓMO SE MIDE SOLO. `go test` ya imprime los segundos de cada paquete («ok  musubi/x  12.3s»).
// El guard lee ESA salida —la de la corrida que acaba de pasar, no una de agosto—, saca el
// paquete más lento, y compara TECHO/MÁS_LENTO contra el margen mínimo de la política. Costo
// cero: el dato ya estaba en el log.
//
// UN CERO NO PUEDE SIGNIFICAR «NO PUDE MEDIR». Si la salida no trae NI UNA línea con segundos
// —formato cambiado, corrida abortada, todo servido del caché de `go test`— Analizar devuelve
// error, nunca un margen infinito y un verde. Ese es el modo de falla que este repo ya pagó:
// un emisor que contesta 0 incondicional vuelve indistinguibles «medí y está bien» de «no sé».
package testbudget

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Politica es lo que se le exige al runner. Sale del archivo presupuesto-de-pruebas.env, que es
// la ÚNICA fuente del número: el ci.yml lo lee para armar el `-timeout` y este paquete lo lee
// para juzgar el margen, así que no hay forma de que las dos mitades se desincronicen.
//
// DOS TECHOS PORQUE SON DOS ENTORNOS. Timeout es un PRESUPUESTO (el de CI: tiene que quedar
// corto, si no el guard del margen no se puede poner rojo nunca) y TimeoutLocal es un PISO (el
// que se le exige a quien corre en su máquina: tiene que quedar largo, si no la corrida muere
// con un panic ilegible). Un solo número no podía ser los dos: con el runner midiendo 333,3 s y
// la peor máquina fechada 1059 s, el presupuesto tope da ~33m y el piso da ~35m. Ver el bloque
// «DOS TECHOS» de presupuesto-de-pruebas.env.
type Politica struct {
	Timeout          time.Duration // RACE_TIMEOUT: lo que se le pasa a `go test -timeout` EN CI.
	TimeoutLocal     time.Duration // TIMEOUT_LOCAL: el -timeout que se le pide a quien corre -race en su máquina.
	MedicionLocal    time.Duration // MEDICION_LOCAL_SEGUNDOS: el paquete más lento en la peor máquina fechada.
	MargenMinimo     float64       // MARGEN_MINIMO: TECHO/MÁS_LENTO mínimo aceptable, para los dos techos.
	UmbralLineasTest int           // UMBRAL_GUARDA_LINEAS_TEST: desde cuántas líneas de test un paquete tiene que llevar el guard.
}

// UNA POLÍTICA TIENE QUE PODER PONERSE ROJA. Los rangos de acá abajo no son paranoia de
// validación: son el agujero medido. Un MARGEN_MINIMO de 1,01 o un RACE_TIMEOUT de 500h apagan
// el guard PARA SIEMPRE y pasan verde, y quedan escritos en el archivo que se supone que es la
// política — o sea que el aparato entero sigue ahí, corriendo, sin poder decir que no.
//
// Una política que no puede fallar nunca no es una política: es un adorno con costo de CI.
const (
	// MargenMinimoPiso — por debajo de 1,5 el guard deja de significar algo. 1,5 es lo mínimo
	// honesto que se le puede pedir a un runner compartido: que un runner un 50 % más lento que
	// el de hoy todavía termine. (Hoy se exige 2,0.)
	MargenMinimoPiso = 1.5
	// MargenMinimoTecho — por arriba de 10 ninguna suite real entra, así que CI quedaría rojo
	// permanentemente y lo primero que se hace con un rojo permanente es apagarlo.
	MargenMinimoTecho = 10.0
	// TimeoutPiso — el techo tiene que ser MAYOR que el default de Go (10m por paquete). Un
	// RACE_TIMEOUT igual o menor no compra nada: es el default con más pasos.
	TimeoutPiso = 10 * time.Minute
	// TimeoutTecho — 2h. Un cuelgue de verdad quema el techo entero de un runner; más de dos
	// horas ya no es «un margen generoso», es no tener techo (y el job de GitHub muere a las 6h
	// de todas formas, sin decir por qué).
	//
	// OJO: ESTE RANGO SOLO NO ALCANZA, Y ERA UN AGUJERO MEDIDO. Un rango es un número tipeado
	// contra otro número tipeado: no sabe cuánto tarda la suite. Con RACE_TIMEOUT=2h —legal acá
	// dentro— e internal/mcp en 1059 s, el margen sale 6,8× contra el 2,0× que exige la política,
	// o sea que el guard del margen queda VERDE POR AMPLITUD DEL RANGO y no por estar bien: la
	// suite podría triplicarse sin que nada dijera nada. El tope de verdad es FactorTechoMaximo,
	// más abajo, que se mide contra la corrida en vez de tipearse.
	//
	// Y DESDE QUE HAY DOS TECHOS, ESTE RANGO APRIETA MÁS QUE LO QUE DICE. CargarPolitica exige
	// RACE_TIMEOUT ≤ TIMEOUT_LOCAL ≤ FactorTechoMaximo × MARGEN_MINIMO × MEDICION_LOCAL_SEGUNDOS,
	// o sea que hoy el tope efectivo de RACE_TIMEOUT no es 2h sino 6,00 × 1059 s ≈ 105m — atado a
	// una medición, no a un número tipeado. `RACE_TIMEOUT=2h` ya no entra.
	TimeoutTecho = 2 * time.Hour
	// FactorTechoMaximo — cuántas veces el margen MEDIDO puede pasar al margen EXIGIDO antes de
	// que el techo deje de ser un techo.
	//
	// El guard del margen mira una sola dirección: que el techo no se quede corto. La otra
	// dirección también apaga el aparato, sólo que en silencio — un techo enorme hace que
	// TECHO/MÁS_LENTO sea siempre cómodo y el rojo se vuelve inalcanzable. Con 3 el techo puede
	// sobrar hasta el triple de lo que la política pide (hoy: hasta 6,0× cuando se exige 2,0×,
	// y los medidos son 3,60× en CI y 2,27× en la peor máquina fechada), que es holgura de sobra
	// para el ruido de un runner compartido, y deja de tapar el caso en que alguien «arregla» un
	// rojo subiendo el número.
	//
	// RIGE PARA LOS DOS TECHOS. Contra la corrida, para el presupuesto de CI; y contra
	// MEDICION_LOCAL_SEGUNDOS, para el piso local — ahí lo comprueba CargarPolitica, porque no
	// hay ninguna corrida que pueda medir la laptop de nadie.
	FactorTechoMaximo = 3.0
	// UmbralLineasPiso / UmbralLineasTecho — el umbral que decide QUÉ paquetes tienen que llevar
	// el guard. Subirlo es la forma barata de sacar paquetes de la lista sin tocarlos: con
	// 15.000 líneas de tope, cmd/musubi (13.092 líneas, 118,2 s bajo -race) no se puede dejar
	// afuera moviendo un número.
	UmbralLineasPiso  = 1000
	UmbralLineasTecho = 15000
)

// Paquete es un paquete que REPORTÓ segundos. Un paquete sin tests o servido del caché no
// entra acá: no midió nada, y contarlo como 0 s sería inventar un verde.
type Paquete struct {
	Nombre   string
	Duracion time.Duration
}

// Veredicto es el resultado de juzgar una corrida.
type Veredicto struct {
	Medidos  []Paquete
	MasLento Paquete
	Margen   float64 // Timeout / MasLento.Duracion
	Politica Politica
}

// Rojo dice si el margen medido se comió el umbral de la política.
func (v Veredicto) Rojo() bool { return v.Margen < v.Politica.MargenMinimo }

// TechoDeMas es la MISMA pregunta del otro lado: si el techo sobra tanto contra lo que la
// corrida acaba de medir, el guard del margen no se puede poner rojo nunca y el aparato entero
// pasa a ser decoración con costo de CI.
//
// Se mide contra la corrida y no contra un rango tipeado a propósito: [10m, 2h] es un par de
// números que no sabe cuánto tarda la suite, y adentro de ese rango entra un techo que da 6,8×
// de margen contra el 2,0× exigido.
func (v Veredicto) TechoDeMas() bool {
	return v.Margen > v.Politica.MargenMinimo*FactorTechoMaximo
}

// NombreArchivoPolitica es el archivo de política, en la raíz del repo.
const NombreArchivoPolitica = "presupuesto-de-pruebas.env"

// CargarPolitica lee la política del archivo indicado.
func CargarPolitica(ruta string) (Politica, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return Politica{}, fmt.Errorf("no se pudo leer la política de presupuesto en %q: %w", ruta, err)
	}
	var p Politica
	var vistoTimeout, vistoLocal, vistoMedicion, vistoMargen, vistoUmbral bool
	for _, linea := range strings.Split(string(b), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		clave, valor, ok := strings.Cut(linea, "=")
		if !ok {
			continue
		}
		clave = strings.TrimSpace(clave)
		valor = strings.TrimSpace(valor)
		switch clave {
		case "RACE_TIMEOUT":
			d, err := time.ParseDuration(valor)
			if err != nil {
				return Politica{}, fmt.Errorf("%s: RACE_TIMEOUT=%q no es una duración de Go: %w", ruta, valor, err)
			}
			if d < TimeoutPiso || d > TimeoutTecho {
				return Politica{}, fmt.Errorf("%s: RACE_TIMEOUT=%v está fuera del rango honesto "+
					"[%v, %v]. Por debajo del default de Go (10m por paquete) el techo no compra "+
					"nada; por arriba de %v deja de ser un techo —un cuelgue quemaría el runner "+
					"entero— y además apaga el guard del margen, que pasaría a estar verde con "+
					"cualquier medición", ruta, d, TimeoutPiso, TimeoutTecho, TimeoutTecho)
			}
			p.Timeout, vistoTimeout = d, true
		case "TIMEOUT_LOCAL":
			d, err := time.ParseDuration(valor)
			if err != nil {
				return Politica{}, fmt.Errorf("%s: TIMEOUT_LOCAL=%q no es una duración de Go: %w", ruta, valor, err)
			}
			if d < TimeoutPiso || d > TimeoutTecho {
				return Politica{}, fmt.Errorf("%s: TIMEOUT_LOCAL=%v está fuera del rango honesto "+
					"[%v, %v]. Es el -timeout que se le pide a quien corre -race en su máquina: "+
					"por debajo del default de Go no pide nada, y por arriba de %v ya no es un "+
					"piso sino no tener techo", ruta, d, TimeoutPiso, TimeoutTecho, TimeoutTecho)
			}
			p.TimeoutLocal, vistoLocal = d, true
		case "MEDICION_LOCAL_SEGUNDOS":
			f, err := strconv.ParseFloat(valor, 64)
			if err != nil {
				return Politica{}, fmt.Errorf("%s: MEDICION_LOCAL_SEGUNDOS=%q no es un número: %w", ruta, valor, err)
			}
			if f <= 0 {
				return Politica{}, fmt.Errorf("%s: MEDICION_LOCAL_SEGUNDOS=%v no es una medición: "+
					"un cero o un negativo acá significan «no medí», y «no medí» no puede "+
					"presupuestar nada", ruta, f)
			}
			p.MedicionLocal = time.Duration(f * float64(time.Second))
			vistoMedicion = true
		case "MARGEN_MINIMO":
			f, err := strconv.ParseFloat(valor, 64)
			if err != nil {
				return Politica{}, fmt.Errorf("%s: MARGEN_MINIMO=%q no es un número: %w", ruta, valor, err)
			}
			if f < MargenMinimoPiso || f > MargenMinimoTecho {
				return Politica{}, fmt.Errorf("%s: MARGEN_MINIMO=%v está fuera del rango honesto "+
					"[%v, %v]. Un margen apenas mayor que 1 es no tener margen: el guard queda "+
					"verde para siempre y el aparato entero pasa a ser decoración; uno mayor que "+
					"%v no lo pasa ninguna suite real y un rojo permanente termina apagado",
					ruta, f, MargenMinimoPiso, MargenMinimoTecho, MargenMinimoTecho)
			}
			p.MargenMinimo, vistoMargen = f, true
		case "UMBRAL_GUARDA_LINEAS_TEST":
			n, err := strconv.Atoi(valor)
			if err != nil {
				return Politica{}, fmt.Errorf("%s: UMBRAL_GUARDA_LINEAS_TEST=%q no es un entero: %w", ruta, valor, err)
			}
			if n < UmbralLineasPiso || n > UmbralLineasTecho {
				return Politica{}, fmt.Errorf("%s: UMBRAL_GUARDA_LINEAS_TEST=%d está fuera del "+
					"rango honesto [%d, %d]: subirlo es la forma barata de sacar un paquete caro "+
					"de la enumeración sin tocarlo", ruta, n, UmbralLineasPiso, UmbralLineasTecho)
			}
			p.UmbralLineasTest, vistoUmbral = n, true
		}
	}
	// UNA CLAVE QUE FALTA ES UN ERROR, NO UN CERO. Si RACE_TIMEOUT desapareciera del archivo,
	// devolver la Politica cero dejaría a todo el guard midiendo contra un techo de 0 s.
	if !vistoTimeout {
		return Politica{}, fmt.Errorf("%s: falta RACE_TIMEOUT", ruta)
	}
	if !vistoMargen {
		return Politica{}, fmt.Errorf("%s: falta MARGEN_MINIMO", ruta)
	}
	if !vistoUmbral {
		return Politica{}, fmt.Errorf("%s: falta UMBRAL_GUARDA_LINEAS_TEST", ruta)
	}
	if !vistoLocal {
		return Politica{}, fmt.Errorf("%s: falta TIMEOUT_LOCAL", ruta)
	}
	if !vistoMedicion {
		return Politica{}, fmt.Errorf("%s: falta MEDICION_LOCAL_SEGUNDOS", ruta)
	}
	if err := verificarCoherencia(ruta, p); err != nil {
		return Politica{}, err
	}
	return p, nil
}

// verificarCoherencia es lo que impide que los dos techos vuelvan a pisarse.
//
// LA HISTORIA, EN UNA LÍNEA: había UN número sirviendo a dos consumidores con presiones
// opuestas —el presupuesto de CI, que tiene que quedar corto, y el piso del que corre en su
// máquina, que tiene que quedar largo— y llegó un día en que ningún valor satisfacía a los dos.
// Las dos guardas tenían razón; lo que estaba mal era que juzgaran la misma variable.
//
// Ahora son dos, y lo que las ata es esto:
//
//   - TIMEOUT_LOCAL ≥ RACE_TIMEOUT. El piso local nunca puede quedar por DEBAJO del presupuesto
//     con el que corre CI. Si pudiera, la rama roja del guard (la que atrapa el default de Go de
//     10 min) quedaría gobernada por el número más chico y el tramo de aviso sería código muerto.
//     Es también lo que acota el daño de una MEDICION_LOCAL_SEGUNDOS mentida a la baja.
//
//   - TIMEOUT_LOCAL/MEDICION_LOCAL_SEGUNDOS dentro de la MISMA banda de dos lados que CI le
//     aplica a su propio techo: ≥ MARGEN_MINIMO (si no, el piso local no alcanza para la peor
//     máquina fechada y el guard estaría recomendando el panic ilegible que vino a cerrar) y
//     ≤ MARGEN_MINIMO × FactorTechoMaximo (si no, el piso local es tan grande que no pide nada).
//
// VA ACÁ Y NO EN UN TEST a propósito: así lo comprueban TODOS los que leen la política —el
// binario del presupuesto en CI, y el ExigirTimeoutSuficiente de cada paquete caro—, no sólo el
// que corra `go test ./internal/testbudget/...`. Un archivo de política incoherente no llega a
// gobernar nada.
func verificarCoherencia(ruta string, p Politica) error {
	if p.TimeoutLocal < p.Timeout {
		return fmt.Errorf("%s: TIMEOUT_LOCAL=%v es menor que RACE_TIMEOUT=%v. El piso que se le "+
			"pide a quien corre -race en su máquina no puede quedar por debajo del presupuesto "+
			"con el que corre CI: el runner es el entorno RÁPIDO, no el lento",
			ruta, p.TimeoutLocal, p.Timeout)
	}
	margen := float64(p.TimeoutLocal) / float64(p.MedicionLocal)
	if margen < p.MargenMinimo {
		return fmt.Errorf("%s: TIMEOUT_LOCAL=%v da %.2f× sobre la peor medición fechada "+
			"(MEDICION_LOCAL_SEGUNDOS=%.1fs) y la política exige %.2f×. Así, el guard le "+
			"recomendaría a quien corre en esa máquina un -timeout que NO alcanza, y la corrida "+
			"volvería a morir con «panic: test timed out» culpando a un test inocente — que es "+
			"el defecto que este aparato cierra. Arreglo: subir TIMEOUT_LOCAL, o actualizar la "+
			"medición si el paquete se abarató (con fecha y máquina, en el comentario)",
			ruta, p.TimeoutLocal, margen, p.MedicionLocal.Seconds(), p.MargenMinimo)
	}
	if tope := p.MargenMinimo * FactorTechoMaximo; margen > tope {
		return fmt.Errorf("%s: TIMEOUT_LOCAL=%v da %.2f× sobre la peor medición fechada "+
			"(MEDICION_LOCAL_SEGUNDOS=%.1fs) y el tope es %.2f×: un piso que sobra tanto no pide "+
			"nada, y de paso legaliza un RACE_TIMEOUT igual de grande. Arreglo: bajar "+
			"TIMEOUT_LOCAL a la medición × %.2f (≈%v)",
			ruta, p.TimeoutLocal, margen, p.MedicionLocal.Seconds(), tope, p.MargenMinimo,
			time.Duration(float64(p.MedicionLocal)*p.MargenMinimo).Round(time.Minute))
	}
	return nil
}

// RaizDelRepo sube desde dir hasta encontrar el go.mod del módulo. Sirve para que un test
// encuentre el archivo de política sin tipear una ruta relativa que se rompe al mover el paquete.
func RaizDelRepo(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			return "", errors.New("no se encontró go.mod subiendo desde " + dir)
		}
		dir = padre
	}
}

// CargarPoliticaDelRepo busca la raíz desde dir y carga la política de ahí.
func CargarPoliticaDelRepo(dir string) (Politica, error) {
	raiz, err := RaizDelRepo(dir)
	if err != nil {
		return Politica{}, err
	}
	return CargarPolitica(filepath.Join(raiz, NombreArchivoPolitica))
}

// Las tres formas en que `go test` reporta un paquete. El ancla es el CAMPO, no un substring:
// «ok» tiene que ser la primera palabra de la línea, si no un mensaje de test que empiece con
// «ok » se contaría como un paquete.
var (
	reOK     = regexp.MustCompile(`^ok\s+(\S+)\s+([0-9]+(?:\.[0-9]+)?)s\b`)
	reFAIL   = regexp.MustCompile(`^FAIL\s+(\S+)\s+([0-9]+(?:\.[0-9]+)?)s\b`)
	reCached = regexp.MustCompile(`^ok\s+(\S+)\s+\(cached\)`)
)

// ErrSinMedicion es el error cuando la salida no trae NI UN paquete con segundos.
var ErrSinMedicion = errors.New("la salida de `go test` no trae ni un paquete con segundos: no se pudo medir el presupuesto")

// Analizar saca de la salida de `go test` el paquete más lento y juzga el margen contra la
// política.
//
// Falla —no devuelve un verde— cuando:
//   - no hay ni una línea con segundos (ErrSinMedicion);
//   - algún paquete vino «(cached)»: un resultado cacheado NO trae su tiempo, así que el más
//     lento podría estar escondido detrás de él y el margen que calculáramos sería una mentira
//     optimista. Correr con -count=1 lo evita.
func Analizar(salida string, p Politica) (Veredicto, error) {
	if p.Timeout <= 0 {
		return Veredicto{}, errors.New("política inválida: RACE_TIMEOUT no positivo")
	}
	if p.MargenMinimo <= 1 {
		return Veredicto{}, errors.New("política inválida: MARGEN_MINIMO tiene que ser > 1")
	}

	var medidos []Paquete
	var cacheados []string
	sc := bufio.NewScanner(strings.NewReader(salida))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		linea := strings.TrimRight(sc.Text(), "\r")
		if m := reCached.FindStringSubmatch(linea); m != nil {
			cacheados = append(cacheados, m[1])
			continue
		}
		m := reOK.FindStringSubmatch(linea)
		if m == nil {
			m = reFAIL.FindStringSubmatch(linea)
		}
		if m == nil {
			continue
		}
		seg, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		medidos = append(medidos, Paquete{
			Nombre:   m[1],
			Duracion: time.Duration(seg * float64(time.Second)),
		})
	}
	if err := sc.Err(); err != nil {
		return Veredicto{}, fmt.Errorf("no se pudo leer la salida de `go test`: %w", err)
	}
	if len(cacheados) > 0 {
		return Veredicto{}, fmt.Errorf(
			"%d paquete(s) vinieron del caché de `go test` (%s): un resultado cacheado NO trae "+
				"sus segundos, así que el paquete más lento puede estar escondido ahí y el margen "+
				"sería falsamente optimista. Corré con -count=1",
			len(cacheados), strings.Join(cacheados, ", "))
	}
	if len(medidos) == 0 {
		return Veredicto{}, ErrSinMedicion
	}

	v := Veredicto{Medidos: medidos, Politica: p}
	for _, pk := range medidos {
		if pk.Duracion > v.MasLento.Duracion {
			v.MasLento = pk
		}
	}
	// Un paquete que reporta 0.0s es real (paquete trivial), pero si el MÁS LENTO reporta 0 s
	// entonces la corrida entera midió cero y el margen sería infinito: eso es «no pude medir»
	// disfrazado de verde.
	if v.MasLento.Duracion <= 0 {
		return Veredicto{}, fmt.Errorf("%w (el paquete más lento reportó 0 s)", ErrSinMedicion)
	}
	v.Margen = float64(p.Timeout) / float64(v.MasLento.Duracion)
	return v, nil
}

// Informe es el texto que va al log de CI, con o sin rojo.
func (v Veredicto) Informe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "presupuesto: techo %s · %d paquetes medidos\n", v.Politica.Timeout, len(v.Medidos))
	fmt.Fprintf(&b, "  más lento: %s  %.1fs  (%.0f %% del techo)\n",
		v.MasLento.Nombre, v.MasLento.Duracion.Seconds(),
		100*v.MasLento.Duracion.Seconds()/v.Politica.Timeout.Seconds())
	fmt.Fprintf(&b, "  margen medido: %.2f× (mínimo exigido %.2f×)\n", v.Margen, v.Politica.MargenMinimo)
	switch {
	case v.Rojo():
		fmt.Fprintf(&b, "ERROR: el margen se comió el umbral. %s tarda %.1fs y con un techo de %s "+
			"un runner %.2f× más lento ya no termina. Arreglo: abaratar el paquete (medir POR QUÉ "+
			"tardó, como hizo A45 con las migraciones) o subir RACE_TIMEOUT en %s con el motivo escrito.\n",
			v.MasLento.Nombre, v.MasLento.Duracion.Seconds(), v.Politica.Timeout,
			v.Politica.MargenMinimo, NombreArchivoPolitica)
	case v.TechoDeMas():
		fmt.Fprintf(&b, "ERROR: el techo dejó de ser un techo. RACE_TIMEOUT=%s da %.2f× de margen "+
			"sobre el paquete más lento medido (%s, %.1fs) y la política exige %.2f×: sobra más de "+
			"%.0f veces lo pedido, así que este guard no se puede poner rojo NUNCA y sólo cuesta "+
			"minutos de CI. Arreglo: bajar RACE_TIMEOUT en %s a la medición × %.2f (≈%s).\n",
			v.Politica.Timeout, v.Margen, v.MasLento.Nombre, v.MasLento.Duracion.Seconds(),
			v.Politica.MargenMinimo, FactorTechoMaximo, NombreArchivoPolitica,
			v.Politica.MargenMinimo,
			(time.Duration(float64(v.MasLento.Duracion) * v.Politica.MargenMinimo)).Round(time.Minute))
	default:
		fmt.Fprint(&b, "OK: el presupuesto tiene margen.\n")
	}
	return b.String()
}
