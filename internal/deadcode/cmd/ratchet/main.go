// Command ratchet es el trinquete de código muerto (A2): compara la salida de
// golang.org/x/tools/cmd/deadcode contra un baseline commiteado y falla si aparecen funciones
// inalcanzables NUEVAS. El núcleo (normalizar, comparar, validar) vive en internal/deadcode; esto
// es sólo el I/O y los códigos de salida.
//
// USO (en el job `lint` del CI, con el analizador ya corrido a un archivo):
//
//	ratchet -salida deadcode-raw.txt -baseline .deadcode-baseline.txt   # compara; 1 si hay altas
//	ratchet -salida deadcode-raw.txt -baseline .deadcode-baseline.txt --update   # regenera el baseline
//
// CÓDIGOS DE SALIDA, scriptables como `musubi cerebro --alcance`:
//
//	0  ok: 0 altas (puede haber avisos de bajas)
//	1  hay funciones inalcanzables nuevas (violación del trinquete)
//	2  no se pudo correr el chequeo (falta -salida, baseline ausente/deforme, salida no parseable)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"musubi/internal/deadcode"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errw io.Writer) int {
	fs := flag.NewFlagSet("ratchet", flag.ContinueOnError)
	fs.SetOutput(errw)
	salida := fs.String("salida", "", "archivo con la salida cruda de `deadcode` (requerido)")
	baseline := fs.String("baseline", ".deadcode-baseline.txt", "archivo baseline a comparar o regenerar")
	update := fs.Bool("update", false, "reescribe el baseline desde la salida en vez de comparar")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *salida == "" {
		fmt.Fprintln(errw, "ratchet: falta -salida <archivo con la salida de deadcode>")
		return 2
	}

	crudo, err := os.ReadFile(*salida)
	if err != nil {
		fmt.Fprintf(errw, "ratchet: no se pudo leer -salida %q: %v\n", *salida, err)
		return 2
	}
	actual, err := deadcode.Normalize(string(crudo))
	if err != nil {
		fmt.Fprintf(errw, "ratchet: %v\n", err)
		return 2
	}

	if *update {
		if err := os.WriteFile(*baseline, []byte(deadcode.Serializar(actual)), 0o644); err != nil {
			fmt.Fprintf(errw, "ratchet: no se pudo escribir el baseline %q: %v\n", *baseline, err)
			return 2
		}
		fmt.Fprintf(out, "baseline regenerado: %s (%d entradas)\n", *baseline, len(actual))
		return 0
	}

	crudoBase, err := os.ReadFile(*baseline)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(errw, "ratchet: no existe el baseline %q; generalo deliberadamente con --update\n", *baseline)
			return 2
		}
		fmt.Fprintf(errw, "ratchet: no se pudo leer el baseline %q: %v\n", *baseline, err)
		return 2
	}
	base := deadcode.Lineas(string(crudoBase))
	if err := deadcode.SanityBaseline(base); err != nil {
		fmt.Fprintf(errw, "ratchet: %v\n", err)
		return 2
	}

	altas, bajas := deadcode.Compare(base, actual)

	if len(bajas) > 0 {
		for _, b := range bajas {
			fmt.Fprintf(out, "aviso: ya no está inalcanzable (se conectó o se borró): %s\n", legible(b))
		}
		fmt.Fprintf(out, "aviso: la deuda bajó en %d; regenerá el baseline con --update para fijarlo\n", len(bajas))
	}

	if len(altas) > 0 {
		fmt.Fprintf(errw, "\nratchet: %d función(es) nueva(s) quedaron inalcanzables (definidas y sin conectar):\n", len(altas))
		for _, a := range altas {
			fmt.Fprintf(errw, "  ✗ %s\n", legible(a))
		}
		fmt.Fprintln(errw, "\nConectalas (dales un caller real) o, si son inalcanzables a propósito, regenerá el baseline")
		fmt.Fprintln(errw, "con --update y explicá por qué en el commit.")
		return 1
	}

	fmt.Fprintf(out, "ratchet: ok — %d inalcanzable(s) conocida(s), 0 nueva(s)\n", len(actual))
	return 0
}

// legible cambia el TAB de la identidad por una flecha, para imprimirla a una persona.
func legible(id string) string {
	return strings.ReplaceAll(id, "\t", " → ")
}
