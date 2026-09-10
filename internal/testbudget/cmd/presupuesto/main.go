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
package main

import (
	"flag"
	"fmt"
	"os"

	"musubi/internal/testbudget"
)

func main() {
	salida := flag.String("salida", "", "archivo con la salida de `go test` a juzgar")
	imprimirTimeout := flag.Bool("imprimir-timeout", false, "imprime RACE_TIMEOUT y termina")
	flag.Parse()

	pol, err := testbudget.CargarPoliticaDelRepo(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(2)
	}

	if *imprimirTimeout {
		fmt.Println(pol.Timeout)
		return
	}

	if *salida == "" {
		fmt.Fprintln(os.Stderr, "ERROR: falta -salida (el archivo con la salida de `go test`)")
		os.Exit(2)
	}
	b, err := os.ReadFile(*salida)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: no se pudo leer", *salida, "—", err)
		os.Exit(2)
	}

	v, err := testbudget.Analizar(string(b), pol)
	if err != nil {
		// NO PUDE MEDIR ES ROJO, NO VERDE. Si el guard se callara acá, el presupuesto volvería
		// a estar sin vigilar y nadie se enteraría — que es exactamente el defecto que cierra.
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Print(v.Informe())
	if v.Rojo() {
		os.Exit(1)
	}
}
