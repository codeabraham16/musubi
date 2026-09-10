package main

// EL EXIT CODE ES LO ÚNICO QUE CI MIRA, Y NO LO SOSTENÍA NADA.
//
// Veredicto.Rojo() estaba probado en trece subtests. Este paquete —el que convierte ese bool en
// el `exit 1` que hace fallar el step— decía «no test files». O sea que todo el aparato colgaba
// de una traducción que nadie ejercitaba: cambiar un `os.Exit(1)` por un `return`, o mandar el
// informe a stderr en vez de salir != 0, dejaba CI verde con el presupuesto reventado.
//
// Acá se prueban las dos capas, a propósito:
//   - ejecutar(), que es la decisión, contra su tabla de casos;
//   - y el PROCESO de verdad, relanzando este mismo binario de test como subproceso, porque el
//     código de salida de un proceso no se puede afirmar desde adentro del proceso.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/testbudget"
)

const varSubproceso = "MUSUBI_PRESUPUESTO_SUBPROCESO"

func TestMain(m *testing.M) {
	if os.Getenv(varSubproceso) == "1" {
		main() // termina con os.Exit(ejecutar(...))
		return
	}
	os.Exit(m.Run())
}

// politica del repo, para DERIVAR los segundos de cada caso en vez de tipear un número que
// envejezca con el archivo — el defecto original de este cabo, en miniatura.
func politica(t *testing.T) testbudget.Politica {
	t.Helper()
	p, err := testbudget.CargarPoliticaDelRepo(".")
	if err != nil {
		t.Fatalf("no se pudo cargar la política del repo: %v", err)
	}
	return p
}

func salidaConPaquete(segundos float64) string {
	return fmt.Sprintf("ok  \tmusubi/internal/algo\t%.1fs\n", segundos)
}

func archivoCon(t *testing.T, contenido string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "race.txt")
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func TestExitCode(t *testing.T) {
	p := politica(t)
	techo := p.Timeout.Seconds()
	// Cómodo: la mitad de lo que la política tolera. Rojo: apenas por encima de lo que tolera.
	comodo := techo / (p.MargenMinimo * 2)
	apretado := techo/p.MargenMinimo + 1

	casos := []struct {
		nombre   string
		args     []string
		codigo   int
		enStdout string
		enStderr string
	}{
		{
			nombre:   "margen de sobra sale 0",
			args:     []string{"-salida", archivoCon(t, salidaConPaquete(comodo))},
			codigo:   0,
			enStdout: "OK: el presupuesto tiene margen",
		},
		{
			nombre:   "margen comido sale 1",
			args:     []string{"-salida", archivoCon(t, salidaConPaquete(apretado))},
			codigo:   1,
			enStdout: "ERROR: el margen se comió el umbral",
		},
		{
			nombre:   "sin medición sale 1, no 0",
			args:     []string{"-salida", archivoCon(t, "=== RUN algo\n--- PASS: algo\nPASS\n")},
			codigo:   1,
			enStderr: "no se pudo medir",
		},
		{
			nombre:   "todo cacheado sale 1",
			args:     []string{"-salida", archivoCon(t, "ok  \tmusubi/internal/algo\t(cached)\n")},
			codigo:   1,
			enStderr: "caché",
		},
		{
			nombre:   "el más lento en 0 s sale 1",
			args:     []string{"-salida", archivoCon(t, salidaConPaquete(0))},
			codigo:   1,
			enStderr: "no se pudo medir",
		},
		{
			nombre:   "sin -salida sale 2",
			args:     []string{},
			codigo:   2,
			enStderr: "falta -salida",
		},
		{
			nombre:   "archivo que no existe sale 2",
			args:     []string{"-salida", filepath.Join(t.TempDir(), "no-existe.txt")},
			codigo:   2,
			enStderr: "no se pudo leer",
		},
		{
			nombre:   "flag desconocido sale 2",
			args:     []string{"-inventado"},
			codigo:   2,
			enStderr: "not defined",
		},
		{
			nombre:   "imprimir-timeout sale 0 y escribe el techo",
			args:     []string{"-imprimir-timeout"},
			codigo:   0,
			enStdout: p.Timeout.String(),
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			// 1) La decisión, en proceso.
			var out, errb bytes.Buffer
			if got := ejecutar(c.args, &out, &errb); got != c.codigo {
				t.Errorf("ejecutar(%v) = %d, quiero %d\nstdout: %s\nstderr: %s",
					c.args, got, c.codigo, out.String(), errb.String())
			}
			if c.enStdout != "" && !strings.Contains(out.String(), c.enStdout) {
				t.Errorf("stdout no menciona %q:\n%s", c.enStdout, out.String())
			}
			if c.enStderr != "" && !strings.Contains(errb.String(), c.enStderr) {
				t.Errorf("stderr no menciona %q:\n%s", c.enStderr, errb.String())
			}

			// 2) El PROCESO. Es lo que lee CI, y es lo que ninguna prueba miraba.
			codigo, sout, serr := correrComoProceso(t, c.args)
			if codigo != c.codigo {
				t.Errorf("el proceso salió %d, quiero %d\nstdout: %s\nstderr: %s",
					codigo, c.codigo, sout, serr)
			}
		})
	}
}

// correrComoProceso relanza este binario de test en modo comando y devuelve su código real.
func correrComoProceso(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), varSubproceso+"=1")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if err == nil {
		return 0, out.String(), errb.String()
	}
	var salida *exec.ExitError
	if ok := asExitError(err, &salida); !ok {
		t.Fatalf("el subproceso no terminó con un código de salida: %v", err)
	}
	return salida.ExitCode(), out.String(), errb.String()
}

func asExitError(err error, dst **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*dst = e
	}
	return ok
}
