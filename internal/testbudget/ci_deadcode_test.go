package testbudget

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// ANTI-BORRADO DEL TRINQUETE DE CÓDIGO MUERTO (A2, R8).
//
// El trinquete (internal/deadcode) sólo sirve si el CI lo CORRE. El sabotaje a vigilar es el mismo
// que ya mordió a otras compuertas: el código queda entero en el repo, con sus tests en verde, y el
// step del workflow se borra, se desactiva (`if: false` / `continue-on-error: true`) o se afloja
// (despinnear el analizador, sacarle el baseline). Cualquiera de esas deja una función definida y
// sin conectar sin poner rojo el CI, y nada lo avisaría.
//
// Esta guarda NO se ancla al NOMBRE del step —un nombre se cambia sin cambiar nada— ni al TEXTO del
// `run` —un `echo` que mencione el paquete alcanzaría para engañar a un strings.Contains—, sino al
// COMANDO real: un `go run ./…/ratchet` hallado POR ARGV con comandosGoRun (un eco no es un comando
// `go run`, así que no cuenta) que compara contra el baseline commiteado (`-baseline` con su valor),
// en el job de análisis estático, con el analizador PINNEADO, sin estar desactivado y con su rojo
// llegando al job. La detección de «desactivado» reusa erroresDePasoApagado y la de «el rojo se lo
// tragaron» (`|| true`, un `&` al fondo, una tubería sin pipefail) reusa erroresDeCodigoDeSalida:
// los mismos jueces que protegen al guard del margen.
//
// Hace falta esta guarda aparte de TestCadaCompuertaDeCINombraUnTestQueExiste porque aquélla sólo
// mira las compuertas con forma `go test -run/-bench`, y ésta corre con `go run`.

const (
	jobDelTrinquete      = "lint" // hermano de golangci-lint y los .syso: análisis estático
	paqueteDelTrinquete  = "internal/deadcode/cmd/ratchet"
	baselineDelTrinquete = ".deadcode-baseline.txt"
	moduloAnalizador     = "golang.org/x/tools/cmd/deadcode"
)

// instalaAnalizador captura la versión con la que se instala deadcode: `...deadcode@v0.30.0` → v0.30.0.
var instalaAnalizador = regexp.MustCompile(regexp.QuoteMeta(moduloAnalizador) + `@(\S+)`)

// ErroresDelTrinqueteDeCodigoMuerto exige que el CI CORRA el trinquete y pueda poner rojo el job.
func ErroresDelTrinqueteDeCodigoMuerto(f *FlujoCI) []error {
	// El comando REAL, por argv: un `echo "…ratchet…"` no es un `go run`, así que no engaña al finder.
	corridas := comandosGoRun(f, "./"+paqueteDelTrinquete)

	// De todas las corridas del helper, la que DECIDE el gate es la de comparación: lleva `-baseline`.
	// La de `--update` regenera y no falla por deuda nueva, así que no alcanza para cumplir la guarda.
	var juez *GoRun
	for i := range corridas {
		if v, ok := valorDeFlag(corridas[i].Argv, "baseline"); ok && v == baselineDelTrinquete {
			juez = &corridas[i]
			break
		}
	}
	if juez == nil {
		if len(corridas) > 0 {
			return []error{fmt.Errorf("%s: el helper del trinquete corre pero NINGUNA invocación pasa "+
				"`-baseline %s`; sin baseline es un listado que nunca falla, no un trinquete",
				f.Ruta, baselineDelTrinquete)}
		}
		return []error{fmt.Errorf("%s: ningún `go run ./%s … -baseline %s`: el trinquete de código "+
			"muerto está entero en el repo y no lo ejecuta nadie, así que una función definida y sin "+
			"conectar NO pone rojo el CI", f.Ruta, paqueteDelTrinquete, baselineDelTrinquete)}
	}

	var errs []error
	paso := juez.Paso
	if paso.Job != jobDelTrinquete {
		errs = append(errs, fmt.Errorf("%s · paso %q: el trinquete corre en el job %q, no en %q "+
			"(donde viven los otros chequeos de análisis estático y donde lo ancla esta guarda)",
			f.Ruta, paso.Nombre, paso.Job, jobDelTrinquete))
	}

	// El analizador va PINNEADO: con @latest, un cambio suyo se leería como deuda nueva (rojo espurio).
	switch m := instalaAnalizador.FindStringSubmatch(paso.Run); {
	case m == nil:
		errs = append(errs, fmt.Errorf("%s · paso %q: no instala %s@<versión>; sin el analizador el "+
			"gate no corre", f.Ruta, paso.Nombre, moduloAnalizador))
	case m[1] == "latest":
		errs = append(errs, fmt.Errorf("%s · paso %q: instala el analizador con @latest; va PINNEADO "+
			"a una versión exacta, o un cambio del analizador da un rojo espurio", f.Ruta, paso.Nombre))
	}

	// No puede estar desactivado —ni el paso ni el job—, y su rojo tiene que LLEGAR al job: un
	// `|| true`, un `&` o una tubería sin pipefail se lo tragan y el comando seguiría escrito igual.
	errs = append(errs, erroresDePasoApagado(f, paso, "el paso del trinquete de código muerto")...)
	errs = append(errs, erroresDeCodigoDeSalida(f, *juez)...)
	return errs
}

// Contra el ci.yml REAL: el trinquete corre, pinneado, contra el baseline, y sin estar apagado.
func TestElCiEjecutaElTrinqueteDeCodigoMuerto(t *testing.T) {
	f, _, _ := flujoDelRepo(t)
	for _, err := range ErroresDelTrinqueteDeCodigoMuerto(f) {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------------------------
// LOS SABOTAJES DEL TRINQUETE, UNO POR UNO, sobre un workflow sintético con la misma forma que el real.

const ciBaseTrinquete = `
name: CI
on:
  push:
    branches: [main]
  pull_request:
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: golangci-lint
        run: golangci-lint run ./...
      - name: Trinquete de código muerto
        run: |
          go install golang.org/x/tools/cmd/deadcode@v0.30.0
          GOOS=linux GOARCH=amd64 "$(go env GOPATH)/bin/deadcode" -test ./... > deadcode-raw.txt
          go run ./internal/deadcode/cmd/ratchet -salida deadcode-raw.txt -baseline .deadcode-baseline.txt
`

func juzgarTrinquete(t *testing.T, yaml string) []error {
	t.Helper()
	f, err := ParsearCI("ci-sintetico.yml", yaml)
	if err != nil {
		return []error{err} // un YAML que no parsea también es «no pude mirar»
	}
	return ErroresDelTrinqueteDeCodigoMuerto(f)
}

// El sintético sano no puede dar errores: si diera, la tabla de abajo mediría ruido.
func TestElTrinqueteSinteticoSanoPasa(t *testing.T) {
	if errs := juzgarTrinquete(t, ciBaseTrinquete); len(errs) != 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func TestCadaSabotajeDelTrinqueteCae(t *testing.T) {
	casos := []struct {
		nombre string
		viejo  string
		nuevo  string
	}{
		{
			// (1) Borrar la invocación del helper: el código queda en el repo y nadie lo corre.
			nombre: "se saca el `go run` del helper",
			viejo:  "go run ./internal/deadcode/cmd/ratchet -salida deadcode-raw.txt -baseline .deadcode-baseline.txt",
			nuevo:  "true",
		},
		{
			// (2) Desactivar el paso con un `if:` que no se cumple.
			nombre: "if: false en el paso",
			viejo:  "      - name: Trinquete de código muerto\n        run: |",
			nuevo:  "      - name: Trinquete de código muerto\n        if: false\n        run: |",
		},
		{
			// (3) Aflojar el paso con continue-on-error: su rojo ya no falla el job.
			nombre: "continue-on-error en el paso",
			viejo:  "      - name: Trinquete de código muerto\n        run: |",
			nuevo:  "      - name: Trinquete de código muerto\n        continue-on-error: true\n        run: |",
		},
		{
			// (4) Desactivar el JOB entero con `if: false`.
			nombre: "if: false en el job",
			viejo:  "  lint:\n    runs-on: ubuntu-latest",
			nuevo:  "  lint:\n    if: false\n    runs-on: ubuntu-latest",
		},
		{
			// (5) Despinnear el analizador: @latest vuelve el baseline un blanco móvil.
			nombre: "analizador con @latest",
			viejo:  "deadcode@v0.30.0",
			nuevo:  "deadcode@latest",
		},
		{
			// (6) Sacarle el baseline: sin baseline no es un trinquete, es un listado que no falla.
			nombre: "sin -baseline",
			viejo:  " -baseline .deadcode-baseline.txt",
			nuevo:  "",
		},
		{
			// (7) Mover el gate a un job que no es el de análisis estático.
			nombre: "en el job equivocado",
			viejo:  "  lint:",
			nuevo:  "  otracosa:",
		},
		{
			// (8) Tragarse el rojo con `|| true`: el `go run` sigue escrito, pero su salida ya no falla.
			nombre: "el rojo del helper se lo traga un `|| true`",
			viejo:  "go run ./internal/deadcode/cmd/ratchet -salida deadcode-raw.txt -baseline .deadcode-baseline.txt",
			nuevo:  "go run ./internal/deadcode/cmd/ratchet -salida deadcode-raw.txt -baseline .deadcode-baseline.txt || true",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if !strings.Contains(ciBaseTrinquete, c.viejo) {
				t.Fatalf("el sintético no contiene %q: el sabotaje no aplica y no probaría nada", c.viejo)
			}
			roto := strings.Replace(ciBaseTrinquete, c.viejo, c.nuevo, 1)
			if roto == ciBaseTrinquete {
				t.Fatalf("el reemplazo no cambió nada")
			}
			if errs := juzgarTrinquete(t, roto); len(errs) == 0 {
				t.Errorf("el sabotaje %q quedó VERDE: la guarda no lo atrapa", c.nombre)
			}
		})
	}
}
