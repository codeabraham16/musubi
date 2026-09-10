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
type Politica struct {
	Timeout          time.Duration // RACE_TIMEOUT: lo que se le pasa a `go test -timeout`.
	MargenMinimo     float64       // MARGEN_MINIMO: TECHO/MÁS_LENTO mínimo aceptable.
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
	TimeoutTecho = 2 * time.Hour
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

// NombreArchivoPolitica es el archivo de política, en la raíz del repo.
const NombreArchivoPolitica = "presupuesto-de-pruebas.env"

// CargarPolitica lee la política del archivo indicado.
func CargarPolitica(ruta string) (Politica, error) {
	b, err := os.ReadFile(ruta)
	if err != nil {
		return Politica{}, fmt.Errorf("no se pudo leer la política de presupuesto en %q: %w", ruta, err)
	}
	var p Politica
	var vistoTimeout, vistoMargen, vistoUmbral bool
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
	return p, nil
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
	if v.Rojo() {
		fmt.Fprintf(&b, "ERROR: el margen se comió el umbral. %s tarda %.1fs y con un techo de %s "+
			"un runner %.2f× más lento ya no termina. Arreglo: abaratar el paquete (medir POR QUÉ "+
			"tardó, como hizo A45 con las migraciones) o subir RACE_TIMEOUT en %s con el motivo escrito.\n",
			v.MasLento.Nombre, v.MasLento.Duracion.Seconds(), v.Politica.Timeout,
			v.Politica.MargenMinimo, NombreArchivoPolitica)
	} else {
		fmt.Fprint(&b, "OK: el presupuesto tiene margen.\n")
	}
	return b.String()
}
