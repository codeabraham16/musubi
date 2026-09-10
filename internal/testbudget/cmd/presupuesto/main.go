// Command presupuesto juzga el presupuesto de tiempo de una corrida de `go test` y sale != 0
// cuando el margen medido cae por debajo del mínimo de la política.
//
// Se invoca desde .github/workflows/ci.yml, DESPUÉS de la corrida verde, sobre el log que esa
// corrida ya escribió:
//
//	go run ./internal/testbudget/cmd/presupuesto -salida race.txt
//
// También imprime el techo para que el ci.yml no tenga que tipearlo:
//
//	go run ./internal/testbudget/cmd/presupuesto -imprimir-timeout
//
// EL CÓDIGO DE SALIDA ES LO ÚNICO QUE CI MIRA, y por eso está separado en ejecutar(): main()
// no hace nada más que traducirlo a os.Exit. Antes toda la lógica vivía adentro de main() con
// os.Exit desparramado, este paquete no tenía NI UN test («no test files») y por lo tanto que
// Veredicto.Rojo() —probadísimo— se convirtiera en un exit 1 no lo sostenía nada.
//
//	0 — se pudo medir y hay margen.
//	1 — se midió y el margen se comió el umbral, O no se pudo medir (que es rojo, no verde).
//	2 — error de uso o de entorno: falta -salida, no se puede leer el archivo o la política.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"musubi/internal/testbudget"
)

func main() {
	os.Exit(ejecutar(os.Args[1:], os.Stdout, os.Stderr))
}

func ejecutar(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("presupuesto", flag.ContinueOnError)
	fs.SetOutput(stderr)
	salida := fs.String("salida", "", "archivo con la salida de `go test` a juzgar")
	imprimirTimeout := fs.Bool("imprimir-timeout", false, "imprime RACE_TIMEOUT y termina")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	pol, err := testbudget.CargarPoliticaDelRepo(".")
	if err != nil {
		fmt.Fprintln(stderr, "ERROR:", err)
		return 2
	}

	if *imprimirTimeout {
		fmt.Fprintln(stdout, pol.Timeout)
		return 0
	}

	if *salida == "" {
		fmt.Fprintln(stderr, "ERROR: falta -salida (el archivo con la salida de `go test`)")
		return 2
	}
	b, err := os.ReadFile(*salida)
	if err != nil {
		fmt.Fprintln(stderr, "ERROR: no se pudo leer", *salida, "—", err)
		return 2
	}

	v, err := testbudget.Analizar(string(b), pol)
	if err != nil {
		// NO PUDE MEDIR ES ROJO, NO VERDE. Si el guard se callara acá, el presupuesto volvería
		// a estar sin vigilar y nadie se enteraría — que es exactamente el defecto que cierra.
		fmt.Fprintln(stderr, "ERROR:", err)
		return 1
	}
	fmt.Fprint(stdout, v.Informe())
	if v.Rojo() {
		return 1
	}
	return 0
}
